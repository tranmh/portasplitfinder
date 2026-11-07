-- Shared pgTAP fixtures: builder functions plus a deterministic seed.
--
-- Load it inside a test's transaction, after BEGIN:
--     \i test/pgtap/seeds/seed.sql
-- Everything here (schema, functions, rows) is rolled back with the test.
-- The rows mirror internal/pgtest/testdata/seed.sql and internal/fixturetest
-- (same IDs and keys), plus a few extra rows for the error/unsubscribed paths.

\i test/pgtap/seeds/schema.sql

CREATE SCHEMA IF NOT EXISTS fx;

-- Builders: every argument has a default, so a test only names what matters.

CREATE OR REPLACE FUNCTION fx.product(
    p_name  text DEFAULT 'Test Product',
    p_model text DEFAULT 'TP-1',
    p_id    uuid DEFAULT gen_random_uuid()
) RETURNS uuid LANGUAGE sql AS $$
    INSERT INTO products (id, name, model, created_at)
    VALUES (p_id, p_name, p_model, '2026-01-15 09:00:00+00')
    RETURNING id;
$$;

CREATE OR REPLACE FUNCTION fx.source(
    p_key  text,
    p_type text DEFAULT 'store',
    p_name text DEFAULT NULL
) RETURNS text LANGUAGE sql AS $$
    INSERT INTO sources (key, name, type, base_url)
    VALUES (p_key, coalesce(p_name, upper(p_key)), p_type, 'https://' || p_key || '.example')
    RETURNING key;
$$;

CREATE OR REPLACE FUNCTION fx.store(
    p_source_key text,
    p_name       text DEFAULT 'Test Store',
    p_plz        text DEFAULT '73035',
    p_city       text DEFAULT 'Göppingen',
    p_lat        double precision DEFAULT 48.7036,
    p_lon        double precision DEFAULT 9.6527,
    p_id         uuid DEFAULT gen_random_uuid()
) RETURNS uuid LANGUAGE sql AS $$
    INSERT INTO stores (id, source_key, name, plz, city, lat, lon)
    VALUES (p_id, p_source_key, p_name, p_plz, p_city, p_lat, p_lon)
    RETURNING id;
$$;

CREATE OR REPLACE FUNCTION fx.snapshot(
    p_source_key text,
    p_product_id uuid,
    p_store_id   uuid DEFAULT NULL,
    p_in_stock   boolean DEFAULT true,
    p_checked_at timestamptz DEFAULT '2026-01-15 12:00:00+00',
    p_quantity   integer DEFAULT NULL,
    p_price      integer DEFAULT 49900
) RETURNS uuid LANGUAGE sql AS $$
    INSERT INTO availability_snapshots
        (source_key, store_id, product_id, in_stock, quantity, price_cents, url, checked_at)
    VALUES
        (p_source_key, p_store_id, p_product_id, p_in_stock, p_quantity, p_price,
         'https://' || p_source_key || '.example/p/' || p_product_id, p_checked_at)
    RETURNING id;
$$;

CREATE OR REPLACE FUNCTION fx.subscription(
    p_email        text,
    p_confirmed_at timestamptz DEFAULT '2026-01-15 10:05:00+00',
    p_product_id   uuid DEFAULT NULL,
    p_plz          text DEFAULT '73035',
    p_radius_km    integer DEFAULT 50,
    p_id           uuid DEFAULT gen_random_uuid()
) RETURNS uuid LANGUAGE sql AS $$
    INSERT INTO subscriptions
        (id, email, plz, radius_km, product_id, confirm_token, unsub_token, created_at, confirmed_at)
    VALUES
        (p_id, p_email, p_plz, p_radius_km, p_product_id,
         'confirm-' || p_email, 'unsub-' || p_email, '2026-01-15 10:00:00+00', p_confirmed_at)
    RETURNING id;
$$;

CREATE OR REPLACE FUNCTION fx.notification(
    p_subscription_id uuid,
    p_dedupe_key      text,
    p_source_key      text DEFAULT 'obi',
    p_product_id      uuid DEFAULT '11111111-1111-1111-1111-111111111111',
    p_store_id        uuid DEFAULT NULL
) RETURNS uuid LANGUAGE sql AS $$
    INSERT INTO notifications (subscription_id, source_key, store_id, product_id, dedupe_key, sent_at)
    VALUES (p_subscription_id, p_source_key, p_store_id, p_product_id, p_dedupe_key, '2026-01-15 12:30:00+00')
    RETURNING id;
$$;

-- Deterministic seed (same IDs as internal/pgtest/testdata/seed.sql).
INSERT INTO products (id, name, model, created_at) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Midea PortaSplit 12000 BTU', 'PortaSplit 12000', '2026-01-15 09:00:00+00'),
    ('11111111-1111-1111-1111-111111111112', 'Midea PortaSplit 9000 BTU', 'PortaSplit 9000', '2026-01-15 09:00:00+00');

INSERT INTO sources (key, name, type, base_url, last_checked_at, last_status, last_error) VALUES
    ('obi', 'OBI', 'store', 'https://www.obi.de', '2026-01-15 12:00:00+00', 'ok', NULL),
    ('amazon', 'Amazon', 'online', 'https://www.amazon.de', '2026-01-15 12:00:00+00', 'ok', NULL),
    ('bauhaus', 'BAUHAUS', 'store', 'https://www.bauhaus.info', '2026-01-15 12:00:00+00', 'error', 'HTTP 403'),
    ('hornbach', 'HORNBACH', 'store', 'https://www.hornbach.de', NULL, 'never', NULL);

INSERT INTO stores (id, source_key, name, street, plz, city, lat, lon) VALUES
    ('22222222-2222-2222-2222-222222222221', 'obi', 'OBI Reutlingen', 'Hauptstr. 1', '72760', 'Reutlingen', 48.4914, 9.2043),
    ('22222222-2222-2222-2222-222222222222', 'obi', 'OBI Berlin', NULL, '10115', 'Berlin', 52.5321, 13.3849);

-- Two snapshots for the near store: the later one (out of stock) is the latest.
INSERT INTO availability_snapshots (id, source_key, store_id, product_id, in_stock, quantity, price_cents, url, checked_at) VALUES
    ('44444444-4444-4444-4444-444444444441', 'obi', '22222222-2222-2222-2222-222222222221', '11111111-1111-1111-1111-111111111111', true, 3, 49900, 'https://www.obi.de/p/portasplit', '2026-01-15 11:00:00+00'),
    ('44444444-4444-4444-4444-444444444442', 'obi', '22222222-2222-2222-2222-222222222221', '11111111-1111-1111-1111-111111111111', false, 0, 49900, 'https://www.obi.de/p/portasplit', '2026-01-15 12:00:00+00'),
    ('44444444-4444-4444-4444-444444444443', 'obi', '22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', true, 1, 47900, 'https://www.obi.de/p/portasplit', '2026-01-15 12:00:00+00'),
    ('44444444-4444-4444-4444-444444444444', 'amazon', NULL, '11111111-1111-1111-1111-111111111111', true, NULL, 52900, 'https://www.amazon.de/dp/portasplit', '2026-01-15 12:00:00+00');

INSERT INTO subscriptions (id, email, plz, radius_km, product_id, confirm_token, unsub_token, created_at, confirmed_at, unsubbed_at) VALUES
    ('33333333-3333-3333-3333-333333333331', 'confirmed@example.com', '73035', 50, NULL, 'seed-confirm-1', 'seed-unsub-1', '2026-01-15 10:00:00+00', '2026-01-15 10:05:00+00', NULL),
    ('33333333-3333-3333-3333-333333333332', 'pending@example.com', '73035', 50, '11111111-1111-1111-1111-111111111111', 'seed-confirm-2', 'seed-unsub-2', '2026-01-15 10:00:00+00', NULL, NULL),
    ('33333333-3333-3333-3333-333333333333', 'gone@example.com', '10115', 25, NULL, 'seed-confirm-3', 'seed-unsub-3', '2026-01-15 10:00:00+00', '2026-01-15 10:05:00+00', '2026-01-15 11:00:00+00');

INSERT INTO notifications (id, subscription_id, source_key, store_id, product_id, dedupe_key, sent_at) VALUES
    ('55555555-5555-5555-5555-555555555551', '33333333-3333-3333-3333-333333333331', 'amazon', NULL, '11111111-1111-1111-1111-111111111111', 'amazon::11111111-1111-1111-1111-111111111111', '2026-01-15 12:05:00+00');
