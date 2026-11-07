-- Subscription token flows, written with the statements of
-- internal/store/subscriptions.go (Create, ConfirmByToken, UnsubByToken,
-- ListConfirmed).
BEGIN;
SELECT plan(18);

\i test/pgtap/seeds/seed.sql

-- Create: radius and product come from the caller; '' product means any (B7).
SELECT lives_ok(
    $$INSERT INTO subscriptions (email, plz, product_id, confirm_token, unsub_token)
      VALUES ('new@example.com', '73035', NULLIF('', '')::uuid, 'tok-c-new', 'tok-u-new')$$,
    'subscription without radius and product is created'
);
SELECT is(
    (SELECT radius_km FROM subscriptions WHERE confirm_token = 'tok-c-new'),
    50,
    'omitted radius defaults to 50 km (B7)'
);
SELECT ok(
    (SELECT product_id IS NULL FROM subscriptions WHERE confirm_token = 'tok-c-new'),
    'empty product id is stored as NULL = any product (B7)'
);
SELECT ok(
    (SELECT confirmed_at IS NULL AND unsubbed_at IS NULL FROM subscriptions WHERE confirm_token = 'tok-c-new'),
    'a new subscription is pending: not confirmed, not unsubscribed'
);
SELECT throws_ok(
    $$INSERT INTO subscriptions (email, plz, product_id, confirm_token, unsub_token)
      VALUES ('x@example.com', '73035', '00000000-0000-0000-0000-0000000000ff', 'c', 'u')$$,
    '23503', NULL,
    'subscription for an unknown product is rejected'
);
SELECT lives_ok(
    $$INSERT INTO subscriptions (email, plz, confirm_token, unsub_token)
      VALUES ('new@example.com', '73035', 'tok-c-new2', 'tok-u-new2')$$,
    're-subscribing the same email creates another pending row (no uniqueness on email, B8)'
);

-- ConfirmByToken (B9, B10)
SELECT results_eq(
    $$UPDATE subscriptions SET confirmed_at = COALESCE(confirmed_at, now())
      WHERE confirm_token = 'seed-confirm-2' RETURNING email$$,
    $$VALUES ('pending@example.com'::text)$$,
    'confirming a valid token returns the email (B9)'
);
SELECT ok(
    (SELECT confirmed_at IS NOT NULL FROM subscriptions WHERE confirm_token = 'seed-confirm-2'),
    'confirmed_at is set after confirm (B9)'
);
SELECT results_eq(
    $$UPDATE subscriptions SET confirmed_at = COALESCE(confirmed_at, now())
      WHERE confirm_token = 'seed-confirm-1' RETURNING email$$,
    $$VALUES ('confirmed@example.com'::text)$$,
    'confirming an already confirmed token still returns the email (idempotent, B9)'
);
SELECT is(
    (SELECT confirmed_at FROM subscriptions WHERE confirm_token = 'seed-confirm-1'),
    '2026-01-15 10:05:00+00'::timestamptz,
    'repeat confirm keeps the original confirmed_at (set once, B9)'
);
SELECT is_empty(
    $$UPDATE subscriptions SET confirmed_at = COALESCE(confirmed_at, now())
      WHERE confirm_token = 'no-such-token' RETURNING email$$,
    'unknown confirm token matches no row (B10)'
);
SELECT is_empty(
    $$UPDATE subscriptions SET confirmed_at = COALESCE(confirmed_at, now())
      WHERE confirm_token = 'seed-unsub-2' RETURNING email$$,
    'an unsub token is not accepted as confirm token'
);

-- UnsubByToken (B11)
SELECT isnt_empty(
    $$UPDATE subscriptions SET unsubbed_at = COALESCE(unsubbed_at, now())
      WHERE unsub_token = 'seed-unsub-1' RETURNING id$$,
    'unsubscribing a valid token matches its row (B11)'
);
SELECT is(
    (SELECT unsubbed_at FROM subscriptions WHERE unsub_token = 'seed-unsub-3'),
    '2026-01-15 11:00:00+00'::timestamptz,
    'seed: gone@example.com was unsubscribed at a fixed time'
);
SELECT lives_ok(
    $$UPDATE subscriptions SET unsubbed_at = COALESCE(unsubbed_at, now())
      WHERE unsub_token = 'seed-unsub-3'$$,
    'repeat unsubscribe succeeds'
);
SELECT is(
    (SELECT unsubbed_at FROM subscriptions WHERE unsub_token = 'seed-unsub-3'),
    '2026-01-15 11:00:00+00'::timestamptz,
    'repeat unsubscribe keeps the original unsubbed_at'
);
SELECT is_empty(
    $$UPDATE subscriptions SET unsubbed_at = COALESCE(unsubbed_at, now())
      WHERE unsub_token = 'no-such-token' RETURNING id$$,
    'unknown unsub token matches no row (B11)'
);

-- ListConfirmed (B11, B12): confirmed and not unsubscribed, oldest first.
-- seed-confirm-1 was unsubscribed above; seed-confirm-2 was confirmed above.
SELECT results_eq(
    $$SELECT email FROM subscriptions
      WHERE confirmed_at IS NOT NULL AND unsubbed_at IS NULL
      ORDER BY created_at, email$$,
    $$VALUES ('pending@example.com'::text)$$,
    'only confirmed, not unsubscribed subscriptions are listed (B11, B12)'
);

SELECT * FROM finish();
ROLLBACK;
