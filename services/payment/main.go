// Command payment consumes StockReserved, simulates payment, and emits
// PaymentCompleted or PaymentFailed. It is stateless (no database).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/maximcapsa/devops-full-project/pkg/config"
	"github.com/maximcapsa/devops-full-project/pkg/events"
	"github.com/maximcapsa/devops-full-project/pkg/health"
	"github.com/maximcapsa/devops-full-project/pkg/kafka"
	applog "github.com/maximcapsa/devops-full-project/pkg/log"
	"github.com/maximcapsa/devops-full-project/services/payment/internal/consumer"
)

func main() {
	log := applog.New(config.String("LOG_LEVEL", "info"))
	if err := run(log); err != nil {
		log.Error("fatal", slog.Any("err", err))
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	brokers := config.Strings("KAFKA_BROKERS", []string{"localhost:9092"})
	limitCents := int64(config.Int("PAYMENT_LIMIT_CENTS", 1_000_000)) // $10,000 default

	if err := kafka.EnsureTopics(ctx, brokers, 1, 1, events.TopicPaymentsCompleted, events.TopicPaymentsFailed); err != nil {
		return fmt.Errorf("ensure topics: %w", err)
	}
	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		return fmt.Errorf("kafka producer: %w", err)
	}
	defer producer.Close()

	cons, err := kafka.NewConsumer(brokers, "payment", events.TopicInventoryReserved)
	if err != nil {
		return fmt.Errorf("kafka consumer: %w", err)
	}
	defer cons.Close()

	// HTTP health.
	h := health.New()
	h.AddReadyCheck("kafka", producer.Ping)
	httpAddr := ":" + strconv.Itoa(config.Int("PAYMENT_HTTP_PORT", 8084))
	httpSrv := &http.Server{Addr: httpAddr, Handler: h.Mux(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("http listening", slog.String("addr", httpAddr))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", slog.Any("err", err))
		}
	}()

	handler := consumer.New(producer, limitCents, log)
	errCh := make(chan error, 1)
	go func() {
		log.Info("consuming", slog.String("topic", events.TopicInventoryReserved), slog.String("group", "payment"),
			slog.Int64("limit_cents", limitCents))
		errCh <- cons.Run(ctx, handler.HandleStockReserved)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received, draining")
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}

	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shCtx)
	return nil
}
