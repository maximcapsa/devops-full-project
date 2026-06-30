// Package server implements the ProductService gRPC handlers backed by Postgres.
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

	productv1 "github.com/maximcapsa/devops-full-project/gen/product/v1"
	"github.com/maximcapsa/devops-full-project/services/product/internal/db"
)

// Server implements productv1.ProductServiceServer.
type Server struct {
	productv1.UnimplementedProductServiceServer
	q   *db.Queries
	log *slog.Logger
}

// New builds a Server over the given pool.
func New(pool *pgxpool.Pool, log *slog.Logger) *Server {
	return &Server{q: db.New(pool), log: log}
}

// ListProducts returns the full catalog ordered by name.
func (s *Server) ListProducts(ctx context.Context, _ *productv1.ListProductsRequest) (*productv1.ListProductsResponse, error) {
	rows, err := s.q.ListProducts(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "list products", slog.Any("err", err))
		return nil, status.Error(codes.Internal, "failed to list products")
	}
	out := make([]*productv1.Product, 0, len(rows))
	for _, r := range rows {
		out = append(out, toProto(r))
	}
	return &productv1.ListProductsResponse{Products: out}, nil
}

// GetProduct returns a single product by id.
func (s *Server) GetProduct(ctx context.Context, req *productv1.GetProductRequest) (*productv1.Product, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product id")
	}
	row, err := s.q.GetProduct(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "product not found")
	}
	if err != nil {
		s.log.ErrorContext(ctx, "get product", slog.Any("err", err))
		return nil, status.Error(codes.Internal, "failed to get product")
	}
	return toProto(row), nil
}

// toProto maps a DB row to the wire type. Stock is owned by the inventory
// service; the bff enriches listings with real stock from Phase 5 onward.
func toProto(p db.Product) *productv1.Product {
	return &productv1.Product{
		Id:          uuid.UUID(p.ID.Bytes).String(),
		Name:        p.Name,
		Description: p.Description,
		PriceCents:  p.PriceCents,
		Stock:       0,
	}
}
