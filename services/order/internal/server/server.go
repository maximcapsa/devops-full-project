// Package server implements the OrderService gRPC handlers. PlaceOrder prices
// the order by calling the product service over gRPC, persists it in a
// transaction, and produces an OrderPlaced event to Kafka.
package server

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "github.com/maximcapsa/devops-full-project/gen/events/v1"
	orderv1 "github.com/maximcapsa/devops-full-project/gen/order/v1"
	productv1 "github.com/maximcapsa/devops-full-project/gen/product/v1"
	"github.com/maximcapsa/devops-full-project/pkg/events"
	"github.com/maximcapsa/devops-full-project/pkg/kafka"
	"github.com/maximcapsa/devops-full-project/services/order/internal/db"
)

// Server implements orderv1.OrderServiceServer.
type Server struct {
	orderv1.UnimplementedOrderServiceServer
	pool     *pgxpool.Pool
	q        *db.Queries
	product  productv1.ProductServiceClient
	producer *kafka.Producer
	log      *slog.Logger
}

// New builds a Server.
func New(pool *pgxpool.Pool, product productv1.ProductServiceClient, producer *kafka.Producer, log *slog.Logger) *Server {
	return &Server{pool: pool, q: db.New(pool), product: product, producer: producer, log: log}
}

type line struct {
	id        pgtype.UUID
	productID string
	quantity  int32
}

// PlaceOrder validates and prices the cart, persists the order + items in one
// transaction, then produces OrderPlaced. The order is authoritative once
// committed; a publish failure is logged (a production system would use a
// transactional outbox — noted as a deliberate simplification).
func (s *Server) PlaceOrder(ctx context.Context, req *orderv1.PlaceOrderRequest) (*orderv1.Order, error) {
	if len(req.GetItems()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "order must contain at least one item")
	}

	// Aggregate quantities per product (dedupes and avoids PK conflicts).
	qty := map[string]int32{}
	var seen []string
	for _, it := range req.GetItems() {
		if it.GetQuantity() <= 0 {
			return nil, status.Error(codes.InvalidArgument, "item quantity must be positive")
		}
		if _, ok := qty[it.GetProductId()]; !ok {
			seen = append(seen, it.GetProductId())
		}
		qty[it.GetProductId()] += it.GetQuantity()
	}

	// Price each line by calling the product service (source of truth for price).
	var total int64
	lines := make([]line, 0, len(seen))
	for _, pid := range seen {
		u, err := uuid.Parse(pid)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid product id %q", pid)
		}
		p, err := s.product.GetProduct(ctx, &productv1.GetProductRequest{Id: pid})
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return nil, status.Errorf(codes.InvalidArgument, "product %s does not exist", pid)
			}
			s.log.ErrorContext(ctx, "price product", slog.String("product_id", pid), slog.Any("err", err))
			return nil, status.Error(codes.Internal, "failed to price order")
		}
		total += p.GetPriceCents() * int64(qty[pid])
		lines = append(lines, line{id: pgtype.UUID{Bytes: u, Valid: true}, productID: pid, quantity: qty[pid]})
	}

	// Persist order + items atomically.
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to start transaction")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)
	o, err := qtx.CreateOrder(ctx, db.CreateOrderParams{Status: "PENDING", TotalCents: total})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create order")
	}
	for _, ln := range lines {
		if err := qtx.AddOrderItem(ctx, db.AddOrderItemParams{OrderID: o.ID, ProductID: ln.id, Quantity: ln.quantity}); err != nil {
			return nil, status.Error(codes.Internal, "failed to add order item")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, status.Error(codes.Internal, "failed to commit order")
	}

	orderID := uuid.UUID(o.ID.Bytes).String()
	s.publishOrderPlaced(ctx, orderID, total, o.CreatedAt, lines)

	return buildOrder(orderID, o.Status, o.TotalCents, o.CreatedAt, lines), nil
}

// GetOrder returns an order with its items.
func (s *Server) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.Order, error) {
	u, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order id")
	}
	id := pgtype.UUID{Bytes: u, Valid: true}
	o, err := s.q.GetOrder(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get order")
	}
	items, err := s.q.ListOrderItems(ctx, id)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get order items")
	}

	out := &orderv1.Order{
		Id:         req.GetId(),
		Status:     o.Status,
		TotalCents: o.TotalCents,
		CreatedAt:  o.CreatedAt.Format(time.RFC3339),
	}
	for _, it := range items {
		out.Items = append(out.Items, &orderv1.OrderItem{
			ProductId: uuid.UUID(it.ProductID.Bytes).String(),
			Quantity:  it.Quantity,
		})
	}
	return out, nil
}

func (s *Server) publishOrderPlaced(ctx context.Context, orderID string, total int64, at time.Time, lines []line) {
	evt := &eventsv1.OrderPlaced{
		OrderId:    orderID,
		TotalCents: total,
		OccurredAt: timestamppb.New(at),
	}
	for _, ln := range lines {
		evt.Items = append(evt.Items, &eventsv1.EventItem{ProductId: ln.productID, Quantity: ln.quantity})
	}
	payload, err := protojson.Marshal(evt)
	if err != nil {
		s.log.ErrorContext(ctx, "marshal OrderPlaced", slog.Any("err", err))
		return
	}
	if err := s.producer.Publish(ctx, events.TopicOrdersPlaced, orderID, payload); err != nil {
		s.log.ErrorContext(ctx, "publish OrderPlaced", slog.String("order_id", orderID), slog.Any("err", err))
		return
	}
	s.log.InfoContext(ctx, "OrderPlaced published", slog.String("order_id", orderID), slog.Int64("total_cents", total))
}

func buildOrder(id, status string, total int64, at time.Time, lines []line) *orderv1.Order {
	out := &orderv1.Order{Id: id, Status: status, TotalCents: total, CreatedAt: at.Format(time.RFC3339)}
	for _, ln := range lines {
		out.Items = append(out.Items, &orderv1.OrderItem{ProductId: ln.productID, Quantity: ln.quantity})
	}
	return out
}
