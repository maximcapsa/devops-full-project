-- name: GetStock :one
SELECT product_id, available, reserved
FROM stock
WHERE product_id = $1;

-- name: GetStockForUpdate :one
SELECT product_id, available, reserved
FROM stock
WHERE product_id = $1
FOR UPDATE;

-- name: ReserveStock :exec
UPDATE stock
SET available = available - sqlc.arg(qty),
    reserved  = reserved + sqlc.arg(qty)
WHERE product_id = sqlc.arg(product_id);

-- name: GetProcessedOrder :one
SELECT result, reason
FROM processed_orders
WHERE order_id = $1;

-- name: InsertProcessedOrder :exec
INSERT INTO processed_orders (order_id, result, reason)
VALUES ($1, $2, $3);

-- name: InsertReservation :exec
INSERT INTO reservations (order_id, product_id, quantity)
VALUES ($1, $2, $3);

-- name: ListReservations :many
SELECT product_id, quantity
FROM reservations
WHERE order_id = $1;

-- name: ReleaseStock :exec
UPDATE stock
SET available = available + sqlc.arg(qty),
    reserved  = reserved - sqlc.arg(qty)
WHERE product_id = sqlc.arg(product_id);

-- name: MarkOrderReleased :execrows
UPDATE processed_orders
SET result = 'RELEASED'
WHERE order_id = $1 AND result = 'RESERVED';
