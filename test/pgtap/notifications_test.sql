-- Notification dedupe, written with the statements of
-- internal/store/notifications.go (Record, Exists) (B24) and the cascades
-- that clean up after deleted rows.
BEGIN;
SELECT plan(12);

\i test/pgtap/seeds/seed.sql

-- Exists
SELECT ok(
    (SELECT EXISTS (SELECT 1 FROM notifications
        WHERE subscription_id = '33333333-3333-3333-3333-333333333331'
          AND dedupe_key = 'amazon::11111111-1111-1111-1111-111111111111')),
    'Exists is true for the seeded notification'
);
SELECT ok(
    NOT (SELECT EXISTS (SELECT 1 FROM notifications
        WHERE subscription_id = '33333333-3333-3333-3333-333333333332'
          AND dedupe_key = 'amazon::11111111-1111-1111-1111-111111111111')),
    'Exists is per subscription: same key for another subscription is false'
);

-- Record: first claim inserts, the repeat claim is a no-op (atomic dedupe).
SELECT isnt_empty(
    $$INSERT INTO notifications (subscription_id, source_key, store_id, product_id, dedupe_key)
      VALUES ('33333333-3333-3333-3333-333333333331', 'obi',
              NULLIF('22222222-2222-2222-2222-222222222221', '')::uuid,
              '11111111-1111-1111-1111-111111111111', 'obi:22222222-2222-2222-2222-222222222221:p1')
      ON CONFLICT (subscription_id, dedupe_key) DO NOTHING RETURNING id$$,
    'Record claims a new (subscription, dedupe_key) (B24)'
);
SELECT is_empty(
    $$INSERT INTO notifications (subscription_id, source_key, store_id, product_id, dedupe_key)
      VALUES ('33333333-3333-3333-3333-333333333331', 'obi',
              NULLIF('22222222-2222-2222-2222-222222222221', '')::uuid,
              '11111111-1111-1111-1111-111111111111', 'obi:22222222-2222-2222-2222-222222222221:p1')
      ON CONFLICT (subscription_id, dedupe_key) DO NOTHING RETURNING id$$,
    'Record of the same (subscription, dedupe_key) inserts nothing (no repeat mail, B24)'
);
SELECT is(
    (SELECT count(*)::int FROM notifications
      WHERE subscription_id = '33333333-3333-3333-3333-333333333331'
        AND dedupe_key = 'obi:22222222-2222-2222-2222-222222222221:p1'),
    1,
    'exactly one notification row per (subscription, dedupe_key)'
);
SELECT isnt_empty(
    $$INSERT INTO notifications (subscription_id, source_key, store_id, product_id, dedupe_key)
      VALUES ('33333333-3333-3333-3333-333333333332', 'obi', NULL,
              '11111111-1111-1111-1111-111111111111', 'obi:22222222-2222-2222-2222-222222222221:p1')
      ON CONFLICT (subscription_id, dedupe_key) DO NOTHING RETURNING id$$,
    'the same dedupe_key may be claimed by another subscription'
);
SELECT isnt_empty(
    $$INSERT INTO notifications (subscription_id, source_key, store_id, product_id, dedupe_key)
      VALUES ('33333333-3333-3333-3333-333333333331', 'amazon', NULLIF('', '')::uuid,
              '11111111-1111-1111-1111-111111111112', 'amazon::11111111-1111-1111-1111-111111111112')
      ON CONFLICT (subscription_id, dedupe_key) DO NOTHING RETURNING store_id$$,
    'Record with empty store id stores an online notification'
);
SELECT ok(
    (SELECT store_id IS NULL FROM notifications
      WHERE dedupe_key = 'amazon::11111111-1111-1111-1111-111111111112'),
    'empty store id is stored as NULL'
);
SELECT throws_ok(
    $$INSERT INTO notifications (subscription_id, source_key, product_id, dedupe_key)
      VALUES ('00000000-0000-0000-0000-0000000000ff', 'obi',
              '11111111-1111-1111-1111-111111111111', 'k')$$,
    '23503', NULL,
    'notification for an unknown subscription is rejected'
);

-- Cascades
DELETE FROM subscriptions WHERE id = '33333333-3333-3333-3333-333333333331';
SELECT is(
    (SELECT count(*)::int FROM notifications
      WHERE subscription_id = '33333333-3333-3333-3333-333333333331'),
    0,
    'deleting a subscription deletes its notifications'
);

DELETE FROM sources WHERE key = 'obi';
SELECT is(
    (SELECT count(*)::int FROM stores WHERE source_key = 'obi')
      + (SELECT count(*)::int FROM availability_snapshots WHERE source_key = 'obi'),
    0,
    'deleting a source deletes its stores and snapshots'
);

DELETE FROM products WHERE id = '11111111-1111-1111-1111-111111111111';
SELECT is(
    (SELECT count(*)::int FROM subscriptions
      WHERE product_id = '11111111-1111-1111-1111-111111111111')
      + (SELECT count(*)::int FROM availability_snapshots
      WHERE product_id = '11111111-1111-1111-1111-111111111111'),
    0,
    'deleting a product deletes its snapshots and product subscriptions'
);

SELECT * FROM finish();
ROLLBACK;
