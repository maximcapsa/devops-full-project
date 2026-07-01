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
