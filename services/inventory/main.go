// Command inventory consumes OrderPlaced to reserve stock (emitting
// StockReserved/StockRejected) and serves InventoryService stock queries.
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

	inventoryv1 "github.com/maximcapsa/devops-full-project/gen/inventory/v1"
	"github.com/maximcapsa/devops-full-project/pkg/config"
	"github.com/maximcapsa/devops-full-project/pkg/events"
	"github.com/maximcapsa/devops-full-project/pkg/grpcmw"
	"github.com/maximcapsa/devops-full-project/pkg/health"
	"github.com/maximcapsa/devops-full-project/pkg/kafka"
	applog "github.com/maximcapsa/devops-full-project/pkg/log"
	"github.com/maximcapsa/devops-full-project/pkg/postgres"
	"github.com/maximcapsa/devops-full-project/services/inventory/internal/consumer"
	"github.com/maximcapsa/devops-full-project/services/inventory/internal/db"
	"github.com/maximcapsa/devops-full-project/services/inventory/internal/server"
)

const schema = "inventory"

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

	// Topics this service produces.
	if err := kafka.EnsureTopics(ctx, brokers, 1, 1, events.TopicInventoryReserved, events.TopicInventoryRejected); err != nil {
		return fmt.Errorf("ensure topics: %w", err)
	}
	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		return fmt.Errorf("kafka producer: %w", err)
	}
	defer producer.Close()

	cons, err := kafka.NewConsumer(brokers, "inventory", events.TopicOrdersPlaced)
	if err != nil {
		return fmt.Errorf("kafka consumer: %w", err)
	}
	defer cons.Close()

	// HTTP health.
	h := health.New()
	h.AddReadyCheck("postgres", pool.Ping)
	h.AddReadyCheck("kafka", producer.Ping)
	httpAddr := ":" + strconv.Itoa(config.Int("INVENTORY_HTTP_PORT", 8083))
	httpSrv := &http.Server{Addr: httpAddr, Handler: h.Mux(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("http listening", slog.String("addr", httpAddr))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", slog.Any("err", err))
		}
	}()

	// gRPC.
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcmw.UnaryServerInterceptor(log)))
	inventoryv1.RegisterInventoryServiceServer(grpcServer, server.New(pool, log))
	reflection.Register(grpcServer)
	grpcAddr := ":" + strconv.Itoa(config.Int("INVENTORY_GRPC_PORT", 50053))
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", grpcAddr, err)
	}

	// Run gRPC server and Kafka consumer concurrently; first error wins.
	errCh := make(chan error, 2)
	go func() {
		log.Info("grpc listening", slog.String("addr", grpcAddr))
		errCh <- grpcServer.Serve(lis)
	}()
	handler := consumer.New(pool, producer, log)
	go func() {
		log.Info("consuming", slog.String("topic", events.TopicOrdersPlaced), slog.String("group", "inventory"))
		errCh <- cons.Run(ctx, handler.HandleOrderPlaced)
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
