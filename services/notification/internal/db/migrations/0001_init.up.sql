-- Notification schema (search_path pinned to "notification"). One row per
-- (order_id, type); the unique constraint makes consumption idempotent.
CREATE TABLE notifications (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id   uuid NOT NULL,
    type       text NOT NULL,
    message    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (order_id, type)
);

CREATE INDEX notifications_order_id_idx ON notifications (order_id);
