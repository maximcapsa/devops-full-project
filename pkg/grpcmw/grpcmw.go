// Package grpcmw provides gRPC interceptors that propagate a request id across
// service hops and log each call as structured JSON, so a single request can
// be correlated through bff -> product/order -> ... in the logs.
package grpcmw

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const requestIDMD = "x-request-id"

type ctxKey struct{}

// UnaryServerInterceptor injects/propagates a request id and logs each RPC.
func UnaryServerInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		rid := fromIncoming(ctx)
		ctx = context.WithValue(ctx, ctxKey{}, rid)

		start := time.Now()
		resp, err := handler(ctx, req)

		level := slog.LevelInfo
		if err != nil {
			level = slog.LevelError
		}
		log.LogAttrs(ctx, level, "grpc_request",
			slog.String("method", info.FullMethod),
			slog.String("request_id", rid),
			slog.Duration("took", time.Since(start)),
			slog.Any("err", err),
		)
		return resp, err
	}
}

// UnaryClientInterceptor copies the context request id into outgoing metadata.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if rid, ok := ctx.Value(ctxKey{}).(string); ok && rid != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, requestIDMD, rid)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// RequestID returns the request id stored in ctx, or "".
func RequestID(ctx context.Context) string {
	rid, _ := ctx.Value(ctxKey{}).(string)
	return rid
}

// WithRequestID stores rid in ctx (used by REST entrypoints like the bff).
func WithRequestID(ctx context.Context, rid string) context.Context {
	if rid == "" {
		rid = uuid.NewString()
	}
	return context.WithValue(ctx, ctxKey{}, rid)
}

func fromIncoming(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get(requestIDMD); len(v) > 0 && v[0] != "" {
			return v[0]
		}
	}
	return uuid.NewString()
}
