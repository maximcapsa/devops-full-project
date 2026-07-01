-- name: InsertNotification :exec
INSERT INTO notifications (order_id, type, message)
VALUES ($1, $2, $3)
ON CONFLICT (order_id, type) DO NOTHING;

-- name: ListNotifications :many
SELECT id, order_id, type, message, created_at
FROM notifications
WHERE order_id = $1
ORDER BY created_at;
