// Command bff is the backend-for-frontend / API gateway. It speaks REST/JSON to
// the browser (via grpc-gateway generated from the same protos) and gRPC to the
// internal services. New upstreams are added by dialing them and registering
// their gateway handler on the shared mux.
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

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	orderv1 "github.com/maximcapsa/devops-full-project/gen/order/v1"
	productv1 "github.com/maximcapsa/devops-full-project/gen/product/v1"
	"github.com/maximcapsa/devops-full-project/pkg/config"
	"github.com/maximcapsa/devops-full-project/pkg/grpcmw"
	"github.com/maximcapsa/devops-full-project/pkg/health"
	applog "github.com/maximcapsa/devops-full-project/pkg/log"
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

	productAddr := config.String("PRODUCT_GRPC_ADDR", "localhost:50051")
	orderAddr := config.String("ORDER_GRPC_ADDR", "localhost:50052")
	httpPort := config.Int("BFF_HTTP_PORT", 8080)
	corsOrigin := config.String("CORS_ALLOW_ORIGIN", "*")

	// Dial upstreams. grpc.NewClient is lazy, so this doesn't block on startup.
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcmw.UnaryClientInterceptor()),
	}
	productConn, err := grpc.NewClient(productAddr, dialOpts...)
	if err != nil {
		return fmt.Errorf("dial product: %w", err)
	}
	defer func() { _ = productConn.Close() }()

	orderConn, err := grpc.NewClient(orderAddr, dialOpts...)
	if err != nil {
		return fmt.Errorf("dial order: %w", err)
	}
	defer func() { _ = orderConn.Close() }()

	// REST<->gRPC gateway.
	gwmux := runtime.NewServeMux()
	if err := productv1.RegisterProductServiceHandler(ctx, gwmux, productConn); err != nil {
		return fmt.Errorf("register product handler: %w", err)
	}
	if err := orderv1.RegisterOrderServiceHandler(ctx, gwmux, orderConn); err != nil {
		return fmt.Errorf("register order handler: %w", err)
	}

	// Health: readiness reflects upstream channel state.
	h := health.New()
	h.AddReadyCheck("product", connReady("product", productConn))
	h.AddReadyCheck("order", connReady("order", orderConn))

	root := http.NewServeMux()
	root.Handle("/healthz", h.Mux())
	root.Handle("/readyz", h.Mux())
	root.Handle("/", gwmux)

	handler := withCORS(corsOrigin, withRequestID(root))
	addr := ":" + strconv.Itoa(httpPort)
	srv := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}

	errCh := make(chan error, 1)
	go func() {
		log.Info("bff listening", slog.String("addr", addr), slog.String("product_addr", productAddr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received, draining")
	case err := <-errCh:
		return fmt.Errorf("http serve: %w", err)
	}

	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shCtx)
}

// connReady returns a readiness check that passes unless the channel is in a
// failed/shutdown state (Idle/Connecting are fine — the client is lazy).
func connReady(name string, conn *grpc.ClientConn) health.CheckFunc {
	return func(context.Context) error {
		switch s := conn.GetState(); s {
		case connectivity.TransientFailure, connectivity.Shutdown:
			return fmt.Errorf("%s channel %s", name, s)
		default:
			return nil
		}
	}
}

// withRequestID ensures every request carries an id (from the X-Request-Id
// header or generated), stored in context so the gRPC client interceptor
// propagates it downstream, and echoed back on the response.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := grpcmw.WithRequestID(r.Context(), r.Header.Get("X-Request-Id"))
		w.Header().Set("X-Request-Id", grpcmw.RequestID(ctx))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// withCORS allows the browser storefront (a different origin) to call the API.
func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
