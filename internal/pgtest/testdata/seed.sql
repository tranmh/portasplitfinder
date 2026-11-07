-- Deterministic seed matching internal/fixturetest (same IDs and keys).
INSERT INTO products (id, name, model) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Midea PortaSplit 12000 BTU', 'PortaSplit 12000'),
    ('11111111-1111-1111-1111-111111111112', 'Midea PortaSplit 9000 BTU', 'PortaSplit 9000');

INSERT INTO sources (key, name, type, base_url) VALUES
    ('obi', 'OBI', 'store', 'https://www.obi.de'),
    ('amazon', 'Amazon', 'online', 'https://www.amazon.de');

INSERT INTO stores (id, source_key, name, street, plz, city, lat, lon) VALUES
    ('22222222-2222-2222-2222-222222222221', 'obi', 'OBI Reutlingen', 'Hauptstr. 1', '72760', 'Reutlingen', 48.4914, 9.2043),
    ('22222222-2222-2222-2222-222222222222', 'obi', 'OBI Berlin', NULL, '10115', 'Berlin', 52.5321, 13.3849);

-- Two snapshots for the near store: the later one (out of stock) is the latest.
INSERT INTO availability_snapshots (source_key, store_id, product_id, in_stock, quantity, price_cents, url, checked_at) VALUES
    ('obi', '22222222-2222-2222-2222-222222222221', '11111111-1111-1111-1111-111111111111', true, 3, 49900, 'https://www.obi.de/p/portasplit', '2026-01-15 11:00:00+00'),
    ('obi', '22222222-2222-2222-2222-222222222221', '11111111-1111-1111-1111-111111111111', false, 0, 49900, 'https://www.obi.de/p/portasplit', '2026-01-15 12:00:00+00'),
    ('obi', '22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', true, 1, 47900, 'https://www.obi.de/p/portasplit', '2026-01-15 12:00:00+00'),
    ('amazon', NULL, '11111111-1111-1111-1111-111111111111', true, NULL, 52900, 'https://www.amazon.de/dp/portasplit', '2026-01-15 12:00:00+00');

INSERT INTO subscriptions (id, email, plz, radius_km, product_id, confirm_token, unsub_token, created_at, confirmed_at) VALUES
    ('33333333-3333-3333-3333-333333333331', 'confirmed@example.com', '73035', 50, NULL, 'seed-confirm-1', 'seed-unsub-1', '2026-01-15 10:00:00+00', '2026-01-15 10:05:00+00'),
    ('33333333-3333-3333-3333-333333333332', 'pending@example.com', '73035', 50, '11111111-1111-1111-1111-111111111111', 'seed-confirm-2', 'seed-unsub-2', '2026-01-15 10:00:00+00', NULL);
