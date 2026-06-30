-- Catalog table. search_path is pinned to the "product" schema at connect time,
-- so names are unqualified. gen_random_uuid() is built into Postgres 13+.
CREATE TABLE products (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    price_cents bigint NOT NULL CHECK (price_cents >= 0),
    created_at  timestamptz NOT NULL DEFAULT now()
);
