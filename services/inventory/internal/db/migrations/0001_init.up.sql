-- Inventory schema (search_path pinned to "inventory").
CREATE TABLE stock (
    product_id uuid PRIMARY KEY,
    available  int NOT NULL DEFAULT 0 CHECK (available >= 0),
    reserved   int NOT NULL DEFAULT 0 CHECK (reserved >= 0)
);

-- Dedupe table making OrderPlaced consumption idempotent. The stored result
-- lets us re-emit the same StockReserved/StockRejected on redelivery without
-- touching stock again.
CREATE TABLE processed_orders (
    order_id   uuid PRIMARY KEY,
    result     text NOT NULL,           -- RESERVED | REJECTED
    reason     text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
