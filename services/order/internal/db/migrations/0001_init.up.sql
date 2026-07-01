-- Order schema. The schema is named "orders" (not "order") because ORDER is a
-- reserved SQL keyword and would break search_path resolution. search_path is
-- pinned to "orders" at connect time, so names below are unqualified.
CREATE TABLE orders (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    status      text NOT NULL DEFAULT 'PENDING',
    total_cents bigint NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE order_items (
    order_id   uuid NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    product_id uuid NOT NULL,
    quantity   int  NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (order_id, product_id)
);
