-- Latest snapshot per (source, store, product), written with the statements of
-- internal/store/snapshots.go (Latest, LatestForKey) (B19, B23).
BEGIN;
SELECT plan(11);

\i test/pgtap/seeds/seed.sql

SELECT is(
    (SELECT count(*)::int FROM (
        SELECT DISTINCT ON (source_key, store_id, product_id) id
        FROM availability_snapshots
        ORDER BY source_key, store_id, product_id, checked_at DESC) l),
    3,
    'seed: three keys (two OBI stores, one Amazon online listing)'
);
SELECT results_eq(
    $$SELECT DISTINCT ON (source_key, store_id, product_id) id
      FROM availability_snapshots
      ORDER BY source_key, store_id, product_id, checked_at DESC$$,
    $$VALUES ('44444444-4444-4444-4444-444444444444'::uuid),
             ('44444444-4444-4444-4444-444444444442'::uuid),
             ('44444444-4444-4444-4444-444444444443'::uuid)$$,
    'Latest picks the newest snapshot per key (B19)'
);

-- LatestForKey with a store id
SELECT is(
    (SELECT in_stock FROM availability_snapshots
      WHERE source_key = 'obi'
        AND store_id IS NOT DISTINCT FROM NULLIF('22222222-2222-2222-2222-222222222221', '')::uuid
        AND product_id = '11111111-1111-1111-1111-111111111111'
      ORDER BY checked_at DESC LIMIT 1),
    false,
    'LatestForKey returns the newer out-of-stock snapshot, not the older in-stock one'
);

-- LatestForKey for an online listing: '' store id means NULL.
SELECT is(
    (SELECT id FROM availability_snapshots
      WHERE source_key = 'amazon'
        AND store_id IS NOT DISTINCT FROM NULLIF('', '')::uuid
        AND product_id = '11111111-1111-1111-1111-111111111111'
      ORDER BY checked_at DESC LIMIT 1),
    '44444444-4444-4444-4444-444444444444'::uuid,
    'LatestForKey with empty store id finds the online listing'
);
SELECT is_empty(
    $$SELECT id FROM availability_snapshots
      WHERE source_key = 'obi'
        AND store_id IS NOT DISTINCT FROM NULLIF('', '')::uuid
        AND product_id = '11111111-1111-1111-1111-111111111111'$$,
    'LatestForKey with empty store id does not match store listings'
);
SELECT is_empty(
    $$SELECT id FROM availability_snapshots
      WHERE source_key = 'obi'
        AND store_id IS NOT DISTINCT FROM NULLIF('22222222-2222-2222-2222-222222222221', '')::uuid
        AND product_id = '11111111-1111-1111-1111-111111111112'$$,
    'LatestForKey for a product without snapshots finds nothing'
);

-- A new online snapshot for Amazon becomes the latest; NULL store ids group together.
SELECT ok(
    fx.snapshot('amazon', '11111111-1111-1111-1111-111111111111', NULL, false,
                '2026-01-15 13:00:00+00', NULL, NULL) IS NOT NULL,
    'a later online snapshot with unknown price is inserted'
);
SELECT is(
    (SELECT count(*)::int FROM (
        SELECT DISTINCT ON (source_key, store_id, product_id) id
        FROM availability_snapshots
        ORDER BY source_key, store_id, product_id, checked_at DESC) l),
    3,
    'online snapshots with NULL store id share one key'
);
SELECT results_eq(
    $$SELECT in_stock, price_cents, currency FROM (
        SELECT DISTINCT ON (source_key, store_id, product_id) *
        FROM availability_snapshots
        ORDER BY source_key, store_id, product_id, checked_at DESC) l
      WHERE source_key = 'amazon'$$,
    $$VALUES (false, NULL::integer, 'EUR'::text)$$,
    'latest online snapshot keeps NULL price (unknown) and default EUR currency (B23)'
);

-- Insert with checked_at omitted uses the DB default.
SELECT lives_ok(
    $$INSERT INTO availability_snapshots (source_key, product_id, in_stock, url)
      VALUES ('amazon', '11111111-1111-1111-1111-111111111112', true, 'https://www.amazon.de/dp/9000')$$,
    'snapshot without checked_at, price and currency is accepted'
);
SELECT is(
    (SELECT checked_at FROM availability_snapshots WHERE url = 'https://www.amazon.de/dp/9000'),
    now(),
    'checked_at defaults to the transaction time'
);

SELECT * FROM finish();
ROLLBACK;
