BEGIN;
\i test/pgtap/support/schema.psql
SELECT plan(38);

-- tables
SELECT has_table('products', 'products table exists');
SELECT has_table('sources', 'sources table exists');
SELECT has_table('stores', 'stores table exists');
SELECT has_table('availability_snapshots', 'availability_snapshots table exists');
SELECT has_table('subscriptions', 'subscriptions table exists');
SELECT has_table('notifications', 'notifications table exists');

-- primary keys
SELECT col_is_pk('products', 'id', 'products.id is the primary key');
SELECT col_is_pk('sources', 'key', 'sources.key is the primary key');
SELECT col_is_pk('stores', 'id', 'stores.id is the primary key');
SELECT col_is_pk('availability_snapshots', 'id', 'availability_snapshots.id is the primary key');
SELECT col_is_pk('subscriptions', 'id', 'subscriptions.id is the primary key');
SELECT col_is_pk('notifications', 'id', 'notifications.id is the primary key');

-- columns the store search relies on
SELECT col_not_null('stores', 'name', 'stores.name is required');
SELECT col_not_null('stores', 'plz', 'stores.plz is required');
SELECT col_not_null('stores', 'city', 'stores.city is required');
SELECT col_not_null('stores', 'lat', 'stores.lat is required');
SELECT col_not_null('stores', 'lon', 'stores.lon is required');
SELECT col_is_null('stores', 'street', 'stores.street is optional');
SELECT col_type_is('stores', 'lat', 'double precision', 'stores.lat is double precision');
SELECT col_type_is('stores', 'lon', 'double precision', 'stores.lon is double precision');
SELECT col_is_unique('stores', ARRAY['source_key', 'name'], 'a store name is unique per source');

-- snapshots
SELECT col_is_null('availability_snapshots', 'store_id', 'store_id is nullable for online listings');
SELECT col_not_null('availability_snapshots', 'in_stock', 'in_stock is required');
SELECT col_is_null('availability_snapshots', 'price_cents', 'price_cents is optional');
SELECT col_default_is('availability_snapshots', 'currency', 'EUR', 'currency defaults to EUR');
SELECT col_type_is('availability_snapshots', 'checked_at', 'timestamp with time zone', 'checked_at is timestamptz');
SELECT fk_ok('availability_snapshots', 'store_id', 'stores', 'id', 'snapshot store_id references stores');
SELECT fk_ok('availability_snapshots', 'product_id', 'products', 'id', 'snapshot product_id references products');
SELECT fk_ok('availability_snapshots', 'source_key', 'sources', 'key', 'snapshot source_key references sources');
SELECT fk_ok('stores', 'source_key', 'sources', 'key', 'store source_key references sources');

-- defaults
SELECT col_default_is('sources', 'last_status', 'never', 'sources.last_status defaults to never');
SELECT col_default_is('subscriptions', 'radius_km', 50, 'subscriptions.radius_km defaults to 50');

-- indexes
SELECT has_index('availability_snapshots', 'idx_snapshots_latest',
                 ARRAY['source_key', 'store_id', 'product_id', 'checked_at'],
                 'latest-snapshot lookup index covers source, store, product, checked_at');
SELECT has_index('subscriptions', 'idx_subscriptions_confirm_token', ARRAY['confirm_token'],
                 'confirm tokens are indexed');
SELECT has_index('subscriptions', 'idx_subscriptions_unsub_token', ARRAY['unsub_token'],
                 'unsubscribe tokens are indexed');
SELECT has_index('notifications', 'idx_notifications_dedupe', ARRAY['subscription_id', 'dedupe_key'],
                 'notification dedupe lookups are indexed');
SELECT col_is_unique('notifications', ARRAY['subscription_id', 'dedupe_key'],
                     'a dedupe key is unique per subscription');
SELECT fk_ok('notifications', 'subscription_id', 'subscriptions', 'id',
             'notification subscription_id references subscriptions');

SELECT * FROM finish();
ROLLBACK;
