-- Seed catalog. Fixed UUIDs so the inventory service (Phase 5) can seed stock
-- against known product ids. Runs in every environment (local and cloud).
INSERT INTO products (id, name, description, price_cents) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Aeron Chair',         'Ergonomic office chair with lumbar support', 129900),
    ('22222222-2222-2222-2222-222222222222', 'Mechanical Keyboard', 'Hot-swappable, tactile brown switches',      8900),
    ('33333333-3333-3333-3333-333333333333', '4K Monitor',          '27-inch IPS display, 60Hz',                  32900),
    ('44444444-4444-4444-4444-444444444444', 'USB-C Hub',           '7-in-1 aluminum hub',                         4500)
ON CONFLICT (id) DO NOTHING;
