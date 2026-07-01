-- Per-order reservation lines, recorded at reserve time so a PaymentFailed
-- compensation knows exactly what to release. The release flips the matching
-- processed_orders row RESERVED -> RELEASED, which is the idempotency guard.
CREATE TABLE reservations (
    order_id   uuid NOT NULL,
    product_id uuid NOT NULL,
    quantity   int  NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (order_id, product_id)
);
