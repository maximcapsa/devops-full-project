// Package consumer updates order status in response to downstream saga events
// (payment result / stock rejection).
package consumer

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/maximcapsa/devops-full-project/gen/events/v1"
	"github.com/maximcapsa/devops-full-project/pkg/events"
	"github.com/maximcapsa/devops-full-project/services/order/internal/db"
)

// StatusHandler applies terminal status transitions. UpdateOrderStatus is
// idempotent, and each order receives exactly one terminal event.
type StatusHandler struct {
	q   *db.Queries
	log *slog.Logger
}

// NewStatusHandler builds a StatusHandler.
func NewStatusHandler(pool *pgxpool.Pool, log *slog.Logger) *StatusHandler {
	return &StatusHandler{q: db.New(pool), log: log}
}

// Handle maps each event topic to the resulting order status.
func (h *StatusHandler) Handle(ctx context.Context, rec *kgo.Record) error {
	var orderID, newStatus string

	switch rec.Topic {
	case events.TopicPaymentsCompleted:
		var e eventsv1.PaymentCompleted
		if !h.decode(ctx, rec.Value, &e) {
			return nil
		}
		orderID, newStatus = e.GetOrderId(), "PAID"
	case events.TopicPaymentsFailed:
		var e eventsv1.PaymentFailed
		if !h.decode(ctx, rec.Value, &e) {
			return nil
		}
		orderID, newStatus = e.GetOrderId(), "PAYMENT_FAILED"
	case events.TopicInventoryRejected:
		var e eventsv1.StockRejected
		if !h.decode(ctx, rec.Value, &e) {
			return nil
		}
		orderID, newStatus = e.GetOrderId(), "REJECTED"
	default:
		return nil
	}

	u, err := uuid.Parse(orderID)
	if err != nil {
		h.log.ErrorContext(ctx, "bad order id", slog.String("topic", rec.Topic), slog.String("order_id", orderID))
		return nil
	}
	if err := h.q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
		ID: pgtype.UUID{Bytes: u, Valid: true}, Status: newStatus,
	}); err != nil {
		return err // retryable
	}
	h.log.InfoContext(ctx, "order status updated", slog.String("order_id", orderID), slog.String("status", newStatus))
	return nil
}

func (h *StatusHandler) decode(ctx context.Context, b []byte, m proto.Message) bool {
	if err := protojson.Unmarshal(b, m); err != nil {
		h.log.ErrorContext(ctx, "bad event payload", slog.Any("err", err))
		return false
	}
	return true
}
