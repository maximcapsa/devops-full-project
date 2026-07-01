// Package consumer records a notification row for every saga event so the UI
// can show live order status.
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
	"github.com/maximcapsa/devops-full-project/services/notification/internal/db"
)

// Handler writes notifications. InsertNotification is ON CONFLICT DO NOTHING on
// (order_id, type), so redelivery is idempotent.
type Handler struct {
	q   *db.Queries
	log *slog.Logger
}

// New builds a Handler.
func New(pool *pgxpool.Pool, log *slog.Logger) *Handler {
	return &Handler{q: db.New(pool), log: log}
}

// Handle dispatches on the record's topic, extracting order id / type / message.
func (h *Handler) Handle(ctx context.Context, rec *kgo.Record) error {
	var orderID, typ, msg string

	switch rec.Topic {
	case events.TopicOrdersPlaced:
		var e eventsv1.OrderPlaced
		if !h.decode(ctx, rec.Value, &e) {
			return nil
		}
		orderID, typ, msg = e.GetOrderId(), "OrderPlaced", "Order placed"
	case events.TopicInventoryReserved:
		var e eventsv1.StockReserved
		if !h.decode(ctx, rec.Value, &e) {
			return nil
		}
		orderID, typ, msg = e.GetOrderId(), "StockReserved", "Stock reserved"
	case events.TopicInventoryRejected:
		var e eventsv1.StockRejected
		if !h.decode(ctx, rec.Value, &e) {
			return nil
		}
		orderID, typ, msg = e.GetOrderId(), "StockRejected", "Stock rejected: "+e.GetReason()
	case events.TopicPaymentsCompleted:
		var e eventsv1.PaymentCompleted
		if !h.decode(ctx, rec.Value, &e) {
			return nil
		}
		orderID, typ, msg = e.GetOrderId(), "PaymentCompleted", "Payment completed"
	case events.TopicPaymentsFailed:
		var e eventsv1.PaymentFailed
		if !h.decode(ctx, rec.Value, &e) {
			return nil
		}
		orderID, typ, msg = e.GetOrderId(), "PaymentFailed", "Payment failed: "+e.GetReason()
	default:
		return nil
	}

	u, err := uuid.Parse(orderID)
	if err != nil {
		h.log.ErrorContext(ctx, "bad order id", slog.String("topic", rec.Topic), slog.String("order_id", orderID))
		return nil
	}
	if err := h.q.InsertNotification(ctx, db.InsertNotificationParams{
		OrderID: pgtype.UUID{Bytes: u, Valid: true}, Type: typ, Message: msg,
	}); err != nil {
		return err // retryable
	}
	h.log.InfoContext(ctx, "notification recorded", slog.String("order_id", orderID), slog.String("type", typ))
	return nil
}

// decode unmarshals a protojson payload; a bad payload is logged and skipped.
func (h *Handler) decode(ctx context.Context, b []byte, m proto.Message) bool {
	if err := protojson.Unmarshal(b, m); err != nil {
		h.log.ErrorContext(ctx, "bad event payload", slog.Any("err", err))
		return false
	}
	return true
}
