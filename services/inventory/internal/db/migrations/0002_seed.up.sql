-- Seed stock for the seeded catalog products (same fixed UUIDs).
INSERT INTO stock (product_id, available, reserved) VALUES
    ('11111111-1111-1111-1111-111111111111', 100, 0),
    ('22222222-2222-2222-2222-222222222222', 100, 0),
    ('33333333-3333-3333-3333-333333333333', 100, 0),
    ('44444444-4444-4444-4444-444444444444', 100, 0)
ON CONFLICT (product_id) DO NOTHING;
