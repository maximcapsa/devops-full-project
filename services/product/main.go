// Command product is the product-catalog gRPC service. It runs its own
// migrations on startup, serves ProductService over gRPC, and exposes
// /healthz + /readyz over HTTP for Kubernetes probes.
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

	productv1 "github.com/maximcapsa/devops-full-project/gen/product/v1"
	"github.com/maximcapsa/devops-full-project/pkg/config"
	"github.com/maximcapsa/devops-full-project/pkg/grpcmw"
	"github.com/maximcapsa/devops-full-project/pkg/health"
	applog "github.com/maximcapsa/devops-full-project/pkg/log"
	"github.com/maximcapsa/devops-full-project/pkg/postgres"
	"github.com/maximcapsa/devops-full-project/services/product/internal/db"
	"github.com/maximcapsa/devops-full-project/services/product/internal/server"
)

const schema = "product"

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

	log.Info("running migrations", slog.String("schema", schema))
	if err := postgres.Migrate(ctx, dbURL, db.MigrationsFS, "migrations", schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	pool, err := postgres.Connect(ctx, dbURL, schema)
	if err != nil {
		return err
	}
	defer pool.Close()

	// HTTP: health probes.
	h := health.New()
	h.AddReadyCheck("postgres", pool.Ping)
	httpAddr := ":" + strconv.Itoa(config.Int("PRODUCT_HTTP_PORT", 8081))
	httpSrv := &http.Server{Addr: httpAddr, Handler: h.Mux(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("http listening", slog.String("addr", httpAddr))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", slog.Any("err", err))
		}
	}()

	// gRPC: ProductService.
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(grpcmw.UnaryServerInterceptor(log)))
	productv1.RegisterProductServiceServer(grpcServer, server.New(pool, log))
	reflection.Register(grpcServer) // lets grpcurl introspect without protos

	grpcAddr := ":" + strconv.Itoa(config.Int("PRODUCT_GRPC_PORT", 50051))
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", grpcAddr, err)
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("grpc listening", slog.String("addr", grpcAddr))
		errCh <- grpcServer.Serve(lis)
	}()

	// Wait for shutdown signal or a fatal serve error.
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
