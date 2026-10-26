BEGIN;
\i test/pgtap/seeds/seed.psql
SELECT plan(7);

SELECT is((SELECT count(*)::int FROM products), 2, 'seed inserts two products');
SELECT is((SELECT count(*)::int FROM sources), 3, 'seed inserts three sources');
SELECT is((SELECT count(*)::int FROM stores), 3, 'seed inserts three stores');
SELECT is((SELECT count(*)::int FROM availability_snapshots), 5, 'seed inserts five snapshots');
SELECT is((SELECT count(*)::int FROM subscriptions WHERE confirmed_at IS NOT NULL AND unsubbed_at IS NULL), 2,
          'seed has two active confirmed subscriptions');
SELECT is((SELECT last_status FROM sources WHERE key = 'chain-c'), 'never', 'unchecked source keeps the default status');
SELECT throws_ok(
    $$INSERT INTO notifications (subscription_id, source_key, product_id, dedupe_key)
      SELECT subscription_id, source_key, product_id, dedupe_key FROM notifications
      WHERE id = '00000000-0000-0000-0000-000000000501'$$,
    '23505', NULL, 'a duplicate dedupe key for the same subscription is rejected');

SELECT * FROM finish();
ROLLBACK;
