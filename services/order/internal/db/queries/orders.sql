-- name: CreateOrder :one
INSERT INTO orders (status, total_cents)
VALUES ($1, $2)
RETURNING id, status, total_cents, created_at;

-- name: AddOrderItem :exec
INSERT INTO order_items (order_id, product_id, quantity)
VALUES ($1, $2, $3);

-- name: GetOrder :one
SELECT id, status, total_cents, created_at
FROM orders
WHERE id = $1;

-- name: ListOrderItems :many
SELECT product_id, quantity
FROM order_items
WHERE order_id = $1
ORDER BY product_id;

-- name: UpdateOrderStatus :exec
UPDATE orders
SET status = $2
WHERE id = $1;
