-- name: ListProducts :many
SELECT id, name, description, price_cents, created_at
FROM products
ORDER BY name;

-- name: GetProduct :one
SELECT id, name, description, price_cents, created_at
FROM products
WHERE id = $1;
