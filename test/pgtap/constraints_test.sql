BEGIN;
\i test/pgtap/support/schema.psql
SELECT plan(24);

-- fixtures (fixed ids and timestamps, rolled back at the end)
INSERT INTO products (id, name, model) VALUES
    ('00000000-0000-0000-0000-00000000a101', 'Board', 'X1');
INSERT INTO sources (key, name, type, base_url) VALUES
    ('t-online', 'T Online', 'online', 'https://online.example.test'),
    ('t-chain',  'T Chain',  'store',  'https://chain.example.test');
INSERT INTO stores (id, source_key, name, street, plz, city, lat, lon) VALUES
    ('00000000-0000-0000-0000-00000000a201', 't-chain', 'T Chain Mitte', NULL, '10115', 'Berlin', 52.53, 13.38),
    ('00000000-0000-0000-0000-00000000a202', 't-chain', 'T Chain Altona', 'Hauptstr. 1', '22765', 'Hamburg', 53.55, 9.93);
INSERT INTO availability_snapshots (id, source_key, store_id, product_id, in_stock, price_cents, url, checked_at) VALUES
    ('00000000-0000-0000-0000-00000000a301', 't-chain', '00000000-0000-0000-0000-00000000a201',
     '00000000-0000-0000-0000-00000000a101', false, 9900, 'https://chain.example.test/1', '2026-01-01T08:00:00Z'),
    ('00000000-0000-0000-0000-00000000a302', 't-chain', '00000000-0000-0000-0000-00000000a201',
     '00000000-0000-0000-0000-00000000a101', true, 9500, 'https://chain.example.test/1', '2026-01-02T08:00:00Z'),
    ('00000000-0000-0000-0000-00000000a303', 't-online', NULL,
     '00000000-0000-0000-0000-00000000a101', true, NULL, 'https://online.example.test/1', '2026-01-02T09:00:00Z');
INSERT INTO subscriptions (id, email, plz, confirm_token, unsub_token) VALUES
    ('00000000-0000-0000-0000-00000000a401', 'a@example.test', '10115', 'ct-1', 'ut-1');

-- sources
SELECT throws_ok(
    $$INSERT INTO sources (key, name, type, base_url) VALUES ('t-bad', 'Bad', 'warehouse', 'https://x.test')$$,
    '23514', NULL, 'a source type other than online or store is rejected');
SELECT throws_ok(
    $$UPDATE sources SET last_status = 'pending' WHERE key = 't-chain'$$,
    '23514', NULL, 'a last_status other than ok, error or never is rejected');
SELECT lives_ok(
    $$UPDATE sources SET last_status = 'error', last_error = 'HTTP 503', last_checked_at = '2026-01-03T00:00:00Z'
      WHERE key = 't-chain'$$,
    'recording a failed check is accepted');
SELECT is((SELECT last_status FROM sources WHERE key = 't-online'), 'never',
          'a new source starts with last_status never');
SELECT ok((SELECT last_checked_at IS NULL AND last_error IS NULL FROM sources WHERE key = 't-online'),
          'a new source has no check time and no error');
SELECT throws_ok(
    $$INSERT INTO sources (key, name, type, base_url) VALUES ('t-chain', 'Dup', 'store', 'https://x.test')$$,
    '23505', NULL, 'source keys are unique');

-- stores
SELECT throws_ok(
    $$INSERT INTO stores (source_key, name, plz, city, lat, lon) VALUES ('t-chain', 'T Chain Mitte', '10117', 'Berlin', 1, 1)$$,
    '23505', NULL, 'a second store with the same name for the same source is rejected');
SELECT lives_ok(
    $$INSERT INTO stores (source_key, name, plz, city, lat, lon) VALUES ('t-online', 'T Chain Mitte', '10117', 'Berlin', 1, 1)$$,
    'the same store name under another source is accepted');
SELECT throws_ok(
    $$INSERT INTO stores (source_key, name, plz, city, lat, lon) VALUES ('t-missing', 'Nowhere', '10115', 'Berlin', 1, 1)$$,
    '23503', NULL, 'a store for an unknown source is rejected');
SELECT throws_ok(
    $$INSERT INTO stores (source_key, name, plz, city, lat) VALUES ('t-chain', 'No Lon', '10115', 'Berlin', 1)$$,
    '23502', NULL, 'a store without coordinates is rejected');
SELECT ok((SELECT id IS NOT NULL FROM stores WHERE source_key = 't-online' AND name = 'T Chain Mitte'),
          'store ids are generated when omitted');

-- snapshots
SELECT is((SELECT currency FROM availability_snapshots WHERE id = '00000000-0000-0000-0000-00000000a301'), 'EUR',
          'snapshot currency defaults to EUR');
SELECT throws_ok(
    $$INSERT INTO availability_snapshots (source_key, product_id, in_stock, url)
      VALUES ('t-online', '00000000-0000-0000-0000-00000000a101', NULL, 'https://x.test')$$,
    '23502', NULL, 'a snapshot without in_stock is rejected');
SELECT throws_ok(
    $$INSERT INTO availability_snapshots (source_key, product_id, in_stock)
      VALUES ('t-online', '00000000-0000-0000-0000-00000000a101', true)$$,
    '23502', NULL, 'a snapshot without url is rejected');
SELECT throws_ok(
    $$INSERT INTO availability_snapshots (source_key, store_id, product_id, in_stock, url)
      VALUES ('t-chain', '00000000-0000-0000-0000-00000000ffff', '00000000-0000-0000-0000-00000000a101', true, 'https://x.test')$$,
    '23503', NULL, 'a snapshot for an unknown store is rejected');
SELECT lives_ok(
    $$INSERT INTO availability_snapshots (source_key, store_id, product_id, in_stock, url)
      VALUES ('t-online', NULL, '00000000-0000-0000-0000-00000000a101', false, 'https://online.example.test/1')$$,
    'an online snapshot without a store is accepted');

-- latest snapshot per (source, product) for one store, as LatestForStore reads it (B12)
SELECT results_eq(
    $$SELECT DISTINCT ON (source_key, product_id) id
        FROM availability_snapshots
       WHERE store_id = '00000000-0000-0000-0000-00000000a201'
       ORDER BY source_key, product_id, checked_at DESC$$,
    $$VALUES ('00000000-0000-0000-0000-00000000a302'::uuid)$$,
    'only the most recent snapshot per source and product is the latest for a store');

-- subscriptions
SELECT is((SELECT radius_km FROM subscriptions WHERE id = '00000000-0000-0000-0000-00000000a401'), 50,
          'subscription radius defaults to 50 km');
SELECT ok((SELECT confirmed_at IS NULL AND unsubbed_at IS NULL AND created_at IS NOT NULL
             FROM subscriptions WHERE id = '00000000-0000-0000-0000-00000000a401'),
          'a new subscription is unconfirmed and active with a creation time');

-- notifications
INSERT INTO notifications (subscription_id, source_key, store_id, product_id, dedupe_key) VALUES
    ('00000000-0000-0000-0000-00000000a401', 't-chain', '00000000-0000-0000-0000-00000000a201',
     '00000000-0000-0000-0000-00000000a101', 'dk-1');
SELECT throws_ok(
    $$INSERT INTO notifications (subscription_id, source_key, product_id, dedupe_key)
      VALUES ('00000000-0000-0000-0000-00000000a401', 't-online', '00000000-0000-0000-0000-00000000a101', 'dk-1')$$,
    '23505', NULL, 'the same dedupe key twice for one subscription is rejected');
SELECT throws_ok(
    $$INSERT INTO notifications (subscription_id, source_key, product_id, dedupe_key)
      VALUES ('00000000-0000-0000-0000-00000000ffff', 't-online', '00000000-0000-0000-0000-00000000a101', 'dk-2')$$,
    '23503', NULL, 'a notification for an unknown subscription is rejected');

-- cascades
DELETE FROM stores WHERE id = '00000000-0000-0000-0000-00000000a201';
SELECT is((SELECT count(*)::int FROM availability_snapshots WHERE store_id = '00000000-0000-0000-0000-00000000a201'), 0,
          'deleting a store removes its snapshots');
DELETE FROM sources WHERE key = 't-chain';
SELECT is((SELECT count(*)::int FROM stores WHERE source_key = 't-chain'), 0,
          'deleting a source removes its stores');
DELETE FROM subscriptions WHERE id = '00000000-0000-0000-0000-00000000a401';
SELECT is((SELECT count(*)::int FROM notifications WHERE subscription_id = '00000000-0000-0000-0000-00000000a401'), 0,
          'deleting a subscription removes its notifications');

SELECT * FROM finish();
ROLLBACK;
