// Package server implements the InventoryService gRPC stock queries.
package server

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	inventoryv1 "github.com/maximcapsa/devops-full-project/gen/inventory/v1"
	"github.com/maximcapsa/devops-full-project/services/inventory/internal/db"
)

// Server implements inventoryv1.InventoryServiceServer.
type Server struct {
	inventoryv1.UnimplementedInventoryServiceServer
	q   *db.Queries
	log *slog.Logger
}

// New builds a Server.
func New(pool *pgxpool.Pool, log *slog.Logger) *Server {
	return &Server{q: db.New(pool), log: log}
}

// GetStock returns current stock for one product.
func (s *Server) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.StockItem, error) {
	u, err := uuid.Parse(req.GetProductId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product id")
	}
	st, err := s.q.GetStock(ctx, pgUUID(u))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "no stock for product")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get stock")
	}
	return &inventoryv1.StockItem{ProductId: req.GetProductId(), Available: st.Available, Reserved: st.Reserved}, nil
}

// CheckStock reports whether every requested quantity is currently available.
func (s *Server) CheckStock(ctx context.Context, req *inventoryv1.CheckStockRequest) (*inventoryv1.CheckStockResponse, error) {
	all := true
	items := make([]*inventoryv1.StockItem, 0, len(req.GetItems()))
	for _, it := range req.GetItems() {
		u, err := uuid.Parse(it.GetProductId())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid product id "+it.GetProductId())
		}
		st, err := s.q.GetStock(ctx, pgUUID(u))
		if errors.Is(err, pgx.ErrNoRows) {
			all = false
			items = append(items, &inventoryv1.StockItem{ProductId: it.GetProductId()})
			continue
		}
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to check stock")
		}
		if st.Available < it.GetQuantity() {
			all = false
		}
		items = append(items, &inventoryv1.StockItem{ProductId: it.GetProductId(), Available: st.Available, Reserved: st.Reserved})
	}
	return &inventoryv1.CheckStockResponse{AllAvailable: all, Items: items}, nil
}

func pgUUID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }
