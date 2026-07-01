// Command order is the order-placement gRPC service. PlaceOrder prices the cart
// via the product service, persists the order, and produces OrderPlaced to
// Kafka. It runs its own migrations and exposes health probes.
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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	orderv1 "github.com/maximcapsa/devops-full-project/gen/order/v1"
	productv1 "github.com/maximcapsa/devops-full-project/gen/product/v1"
	"github.com/maximcapsa/devops-full-project/pkg/config"
	"github.com/maximcapsa/devops-full-project/pkg/events"
	"github.com/maximcapsa/devops-full-project/pkg/grpcmw"
	"github.com/maximcapsa/devops-full-project/pkg/health"
	"github.com/maximcapsa/devops-full-project/pkg/kafka"
	applog "github.com/maximcapsa/devops-full-project/pkg/log"
	"github.com/maximcapsa/devops-full-project/pkg/postgres"
	"github.com/maximcapsa/devops-full-project/services/order/internal/db"
	"github.com/maximcapsa/devops-full-project/services/order/internal/server"
)

const schema = "orders"

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
	productAddr := config.String("PRODUCT_GRPC_ADDR", "localhost:50051")

	log.Info("running migrations", slog.String("schema", schema))
	if err := postgres.Migrate(ctx, dbURL, db.MigrationsFS, "migrations", schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	pool, err := postgres.Connect(ctx, dbURL, schema)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Kafka: ensure our topic exists (single broker => replication factor 1).
	if err := kafka.EnsureTopics(ctx, brokers, 1, 1, events.TopicOrdersPlaced); err != nil {
		return fmt.Errorf("ensure topics: %w", err)
	}
	producer, err := kafka.NewProducer(brokers)
	if err != nil {
		return fmt.Errorf("kafka producer: %w", err)
	}
	defer producer.Close()

	// Product gRPC client (for pricing).
	productConn, err := grpc.NewClient(productAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcmw.UnaryClientInterceptor()),
	)
	if err != nil {
		return fmt.Errorf("dial product: %w", err)
	}
	defer func() { _ = productConn.Close() }()
	productClient := productv1.NewProductServiceClient(productConn)

	// HTTP health.
	h := health.New()
	h.AddReadyCheck("postgres", pool.Ping)
	h.AddReadyCheck("kafka", producer.Ping)
	httpAddr := ":" + strconv.Itoa(config.Int("ORDER_HTTP_PORT", 8082))
	httpSrv := &http.Server{Addr: httpAddr, Handler: h.Mux(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("http listening", slog.String("addr", httpAddr))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", slog.Any("err", err))
		}
	}()

	// gRPC.
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcmw.UnaryServerInterceptor(log)))
	orderv1.RegisterOrderServiceServer(grpcServer, server.New(pool, productClient, producer, log))
	reflection.Register(grpcServer)

	grpcAddr := ":" + strconv.Itoa(config.Int("ORDER_GRPC_PORT", 50052))
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", grpcAddr, err)
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("grpc listening", slog.String("addr", grpcAddr))
		errCh <- grpcServer.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received, draining")
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("grpc serve: %w", err)
		}
	}

	grpcServer.GracefulStop()
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shCtx)
	return nil
}
