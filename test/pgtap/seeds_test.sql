-- Smoke test for the shared fixtures in test/pgtap/seeds/seed.sql.
BEGIN;
SELECT plan(9);

\i test/pgtap/seeds/seed.sql

SELECT is((SELECT count(*)::int FROM products), 2, 'seed inserts two products');
SELECT is((SELECT count(*)::int FROM sources), 4, 'seed inserts four sources');
SELECT is((SELECT count(*)::int FROM availability_snapshots), 4, 'seed inserts four snapshots');
SELECT is(
    (SELECT in_stock FROM availability_snapshots
      WHERE store_id = '22222222-2222-2222-2222-222222222221'
      ORDER BY checked_at DESC LIMIT 1),
    false,
    'latest snapshot of OBI Reutlingen is out of stock'
);
SELECT is(
    (SELECT count(*)::int FROM subscriptions WHERE confirmed_at IS NOT NULL AND unsubbed_at IS NULL),
    1,
    'exactly one active confirmed subscription'
);

-- Builders compose and return keys usable as foreign keys.
SELECT is(fx.source('testsrc', 'store'), 'testsrc', 'fx.source returns its key');
SELECT ok(
    fx.snapshot('testsrc', fx.product('Builder Product'), fx.store('testsrc', 'Builder Store')) IS NOT NULL,
    'fx.snapshot builds over fx.product and fx.store'
);
SELECT ok(
    fx.notification(fx.subscription('builder@example.com'), 'testsrc::x') IS NOT NULL,
    'fx.notification builds over fx.subscription'
);
SELECT throws_ok(
    $$SELECT fx.notification('33333333-3333-3333-3333-333333333331',
                             'amazon::11111111-1111-1111-1111-111111111111')$$,
    '23505',
    NULL,
    'seeded notification dedupe key is unique per subscription'
);

SELECT * FROM finish();
ROLLBACK;
