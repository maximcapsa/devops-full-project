// Package consumer simulates payment in response to StockReserved and emits
// PaymentCompleted or PaymentFailed. Payment is stateless: idempotency is
// provided by downstream consumers deduping on order_id.
package consumer

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "github.com/maximcapsa/devops-full-project/gen/events/v1"
	"github.com/maximcapsa/devops-full-project/pkg/events"
	"github.com/maximcapsa/devops-full-project/pkg/kafka"
)

// Handler simulates payments. Orders whose total exceeds LimitCents are
// declined, so PaymentFailed is demoable deterministically.
type Handler struct {
	producer   *kafka.Producer
	limitCents int64
	log        *slog.Logger
}

// New builds a Handler.
func New(producer *kafka.Producer, limitCents int64, log *slog.Logger) *Handler {
	return &Handler{producer: producer, limitCents: limitCents, log: log}
}

// HandleStockReserved is a kafka.Handler.
func (h *Handler) HandleStockReserved(ctx context.Context, rec *kgo.Record) error {
	var evt eventsv1.StockReserved
	if err := protojson.Unmarshal(rec.Value, &evt); err != nil {
		h.log.ErrorContext(ctx, "bad StockReserved payload", slog.Any("err", err))
		return nil
	}
	orderID := evt.GetOrderId()
	now := timestamppb.Now()

	if evt.GetTotalCents() > h.limitCents {
		reason := fmt.Sprintf("amount %d cents exceeds limit %d", evt.GetTotalCents(), h.limitCents)
		h.log.InfoContext(ctx, "payment declined", slog.String("order_id", orderID), slog.String("reason", reason))
		payload, _ := protojson.Marshal(&eventsv1.PaymentFailed{OrderId: orderID, Reason: reason, OccurredAt: now})
		return h.producer.Publish(ctx, events.TopicPaymentsFailed, orderID, payload)
	}

	h.log.InfoContext(ctx, "payment completed", slog.String("order_id", orderID), slog.Int64("total_cents", evt.GetTotalCents()))
	payload, _ := protojson.Marshal(&eventsv1.PaymentCompleted{OrderId: orderID, OccurredAt: now})
	return h.producer.Publish(ctx, events.TopicPaymentsCompleted, orderID, payload)
}
