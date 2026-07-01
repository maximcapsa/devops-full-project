// Package server implements the NotificationService gRPC read API.
package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	notificationv1 "github.com/maximcapsa/devops-full-project/gen/notification/v1"
	"github.com/maximcapsa/devops-full-project/services/notification/internal/db"
)

// Server implements notificationv1.NotificationServiceServer.
type Server struct {
	notificationv1.UnimplementedNotificationServiceServer
	q   *db.Queries
	log *slog.Logger
}

// New builds a Server.
func New(pool *pgxpool.Pool, log *slog.Logger) *Server {
	return &Server{q: db.New(pool), log: log}
}

// ListNotifications returns the notifications for an order, oldest first.
func (s *Server) ListNotifications(ctx context.Context, req *notificationv1.ListNotificationsRequest) (*notificationv1.ListNotificationsResponse, error) {
	u, err := uuid.Parse(req.GetOrderId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order id")
	}
	rows, err := s.q.ListNotifications(ctx, pgtype.UUID{Bytes: u, Valid: true})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list notifications")
	}
	out := make([]*notificationv1.Notification, 0, len(rows))
	for _, r := range rows {
		out = append(out, &notificationv1.Notification{
			Id:        uuid.UUID(r.ID.Bytes).String(),
			OrderId:   uuid.UUID(r.OrderID.Bytes).String(),
			Type:      r.Type,
			Message:   r.Message,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}
	return &notificationv1.ListNotificationsResponse{Notifications: out}, nil
}
