// Package consumer handles OrderPlaced events: it reserves stock idempotently
// and emits StockReserved or StockRejected.
package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "github.com/maximcapsa/devops-full-project/gen/events/v1"
	"github.com/maximcapsa/devops-full-project/pkg/events"
	"github.com/maximcapsa/devops-full-project/pkg/kafka"
	"github.com/maximcapsa/devops-full-project/services/inventory/internal/db"
)

const (
	resultReserved = "RESERVED"
	resultRejected = "REJECTED"
)

// Handler reserves stock in response to OrderPlaced.
type Handler struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	producer *kafka.Producer
	log      *slog.Logger
}

// New builds a Handler.
func New(pool *pgxpool.Pool, producer *kafka.Producer, log *slog.Logger) *Handler {
	return &Handler{pool: pool, q: db.New(pool), producer: producer, log: log}
}

// Handle dispatches records by topic: OrderPlaced reserves stock,
// PaymentFailed compensates by releasing the reservation.
func (h *Handler) Handle(ctx context.Context, rec *kgo.Record) error {
	switch rec.Topic {
	case events.TopicOrdersPlaced:
		return h.handleOrderPlaced(ctx, rec)
	case events.TopicPaymentsFailed:
		return h.handlePaymentFailed(ctx, rec)
	default:
		return nil
	}
}

// handleOrderPlaced is idempotent: an order already in processed_orders is not
// re-reserved, but its result is re-emitted (in case the original emit was
// lost), and downstream consumers dedupe.
func (h *Handler) handleOrderPlaced(ctx context.Context, rec *kgo.Record) error {
	var evt eventsv1.OrderPlaced
	if err := protojson.Unmarshal(rec.Value, &evt); err != nil {
		// Poison message — log and skip rather than block the partition forever.
		h.log.ErrorContext(ctx, "bad OrderPlaced payload", slog.Any("err", err))
		return nil
	}
	orderUUID, err := uuid.Parse(evt.GetOrderId())
	if err != nil {
		h.log.ErrorContext(ctx, "bad order id in OrderPlaced", slog.String("order_id", evt.GetOrderId()))
		return nil
	}

	// Already processed? Re-emit its stored result.
	proc, err := h.q.GetProcessedOrder(ctx, pgUUID(orderUUID))
	if err == nil {
		return h.emit(ctx, evt.GetOrderId(), evt.GetTotalCents(), proc.Result, proc.Reason)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("lookup processed order: %w", err) // retryable
	}

	result, reason, err := h.reserve(ctx, orderUUID, &evt)
	if err != nil {
		return err // retryable
	}
	h.log.InfoContext(ctx, "order processed", slog.String("order_id", evt.GetOrderId()),
		slog.String("result", result), slog.String("reason", reason))
	return h.emit(ctx, evt.GetOrderId(), evt.GetTotalCents(), result, reason)
}

// reserve checks and reserves stock in one transaction, recording the outcome
// in processed_orders. Returns RESERVED or REJECTED (with a reason).
func (h *Handler) reserve(ctx context.Context, orderUUID uuid.UUID, evt *eventsv1.OrderPlaced) (string, string, error) {
	qty := map[string]int32{}
	var seen []string
	for _, it := range evt.GetItems() {
		if _, ok := qty[it.GetProductId()]; !ok {
			seen = append(seen, it.GetProductId())
		}
		qty[it.GetProductId()] += it.GetQuantity()
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := h.q.WithTx(tx)

	result, reason := resultReserved, ""
	for _, pid := range seen {
		u, perr := uuid.Parse(pid)
		if perr != nil {
			result, reason = resultRejected, "invalid product id "+pid
			break
		}
		st, serr := qtx.GetStockForUpdate(ctx, pgUUID(u))
		if errors.Is(serr, pgx.ErrNoRows) {
			result, reason = resultRejected, "unknown product "+pid
			break
		}
		if serr != nil {
			return "", "", serr
		}
		if st.Available < qty[pid] {
			result, reason = resultRejected, "insufficient stock for "+pid
			break
		}
	}

	if result == resultReserved {
		for _, pid := range seen {
			u, _ := uuid.Parse(pid)
			if err := qtx.ReserveStock(ctx, db.ReserveStockParams{Qty: qty[pid], ProductID: pgUUID(u)}); err != nil {
				return "", "", err
			}
			// Record the line so a PaymentFailed compensation can release it.
			if err := qtx.InsertReservation(ctx, db.InsertReservationParams{
				OrderID: pgUUID(orderUUID), ProductID: pgUUID(u), Quantity: qty[pid],
			}); err != nil {
				return "", "", err
			}
		}
	}

	if err := qtx.InsertProcessedOrder(ctx, db.InsertProcessedOrderParams{
		OrderID: pgUUID(orderUUID), Result: result, Reason: reason,
	}); err != nil {
		return "", "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return result, reason, nil
}

func (h *Handler) emit(ctx context.Context, orderID string, total int64, result, reason string) error {
	now := timestamppb.Now()
	switch result {
	case resultReserved:
		payload, _ := protojson.Marshal(&eventsv1.StockReserved{OrderId: orderID, TotalCents: total, OccurredAt: now})
		return h.producer.Publish(ctx, events.TopicInventoryReserved, orderID, payload)
	case resultRejected:
		payload, _ := protojson.Marshal(&eventsv1.StockRejected{OrderId: orderID, Reason: reason, OccurredAt: now})
		return h.producer.Publish(ctx, events.TopicInventoryRejected, orderID, payload)
	default:
		return nil
	}
}

// handlePaymentFailed compensates a failed payment by releasing the order's
// reservation. Idempotent: MarkOrderReleased flips RESERVED -> RELEASED and
// reports affected rows; 0 rows means already released (or never reserved),
// so the stock update is skipped on redelivery.
func (h *Handler) handlePaymentFailed(ctx context.Context, rec *kgo.Record) error {
	var evt eventsv1.PaymentFailed
	if err := protojson.Unmarshal(rec.Value, &evt); err != nil {
		h.log.ErrorContext(ctx, "bad PaymentFailed payload", slog.Any("err", err))
		return nil
	}
	orderUUID, err := uuid.Parse(evt.GetOrderId())
	if err != nil {
		h.log.ErrorContext(ctx, "bad order id in PaymentFailed", slog.String("order_id", evt.GetOrderId()))
		return nil
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := h.q.WithTx(tx)

	affected, err := qtx.MarkOrderReleased(ctx, pgUUID(orderUUID))
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil // already released, or the order was never reserved
	}

	lines, err := qtx.ListReservations(ctx, pgUUID(orderUUID))
	if err != nil {
		return err
	}
	for _, ln := range lines {
		if err := qtx.ReleaseStock(ctx, db.ReleaseStockParams{Qty: ln.Quantity, ProductID: ln.ProductID}); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	h.log.InfoContext(ctx, "reservation released", slog.String("order_id", evt.GetOrderId()),
		slog.Int("lines", len(lines)))
	return nil
}

func pgUUID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }
