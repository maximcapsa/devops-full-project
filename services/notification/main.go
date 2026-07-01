// Command notification consumes all saga events and records a notification per
// (order, event type). It serves NotificationService so the UI can poll order
// status.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	notificationv1 "github.com/maximcapsa/devops-full-project/gen/notification/v1"
	"github.com/maximcapsa/devops-full-project/pkg/config"
	"github.com/maximcapsa/devops-full-project/pkg/events"
	"github.com/maximcapsa/devops-full-project/pkg/grpcmw"
	"github.com/maximcapsa/devops-full-project/pkg/health"
	"github.com/maximcapsa/devops-full-project/pkg/kafka"
	applog "github.com/maximcapsa/devops-full-project/pkg/log"
	"github.com/maximcapsa/devops-full-project/pkg/postgres"
	"github.com/maximcapsa/devops-full-project/services/notification/internal/consumer"
	"github.com/maximcapsa/devops-full-project/services/notification/internal/db"
	"github.com/maximcapsa/devops-full-project/services/notification/internal/server"
)

const schema = "notification"

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

	dbURL, err := config.MustString("DATABASE_URL")
	if err != nil {
		return err
	}
	brokers := config.Strings("KAFKA_BROKERS", []string{"localhost:9092"})

	log.Info("running migrations", slog.String("schema", schema))
	if err := postgres.Migrate(ctx, dbURL, db.MigrationsFS, "migrations", schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	pool, err := postgres.Connect(ctx, dbURL, schema)
	if err != nil {
		return err
	}
	defer pool.Close()

	topics := []string{
		events.TopicOrdersPlaced,
		events.TopicInventoryReserved,
		events.TopicInventoryRejected,
		events.TopicPaymentsCompleted,
		events.TopicPaymentsFailed,
	}
	cons, err := kafka.NewConsumer(brokers, "notification", topics...)
	if err != nil {
		return fmt.Errorf("kafka consumer: %w", err)
	}
	defer cons.Close()

	// HTTP health.
	h := health.New()
	h.AddReadyCheck("postgres", pool.Ping)
	httpAddr := ":" + strconv.Itoa(config.Int("NOTIFICATION_HTTP_PORT", 8085))
	httpSrv := &http.Server{Addr: httpAddr, Handler: h.Mux(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("http listening", slog.String("addr", httpAddr))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", slog.Any("err", err))
		}
	}()

	// gRPC.
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcmw.UnaryServerInterceptor(log)))
	notificationv1.RegisterNotificationServiceServer(grpcServer, server.New(pool, log))
	reflection.Register(grpcServer)
	grpcAddr := ":" + strconv.Itoa(config.Int("NOTIFICATION_GRPC_PORT", 50055))
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", grpcAddr, err)
	}

	errCh := make(chan error, 2)
	go func() {
		log.Info("grpc listening", slog.String("addr", grpcAddr))
		errCh <- grpcServer.Serve(lis)
	}()
	handler := consumer.New(pool, log)
	go func() {
		log.Info("consuming", slog.Any("topics", topics), slog.String("group", "notification"))
		errCh <- cons.Run(ctx, handler.Handle)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received, draining")
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}

	grpcServer.GracefulStop()
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shCtx)
	return nil
}
