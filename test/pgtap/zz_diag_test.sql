-- Shape check for test/pgtap/seeds/schema.sql (the migration's Up section):
-- it must load cleanly and twice (IF NOT EXISTS), with the tables the tests use.
BEGIN;
SELECT plan(8);

\i test/pgtap/seeds/schema.sql
\i test/pgtap/seeds/schema.sql

SELECT has_table('products', 'schema has products');
SELECT has_table('sources', 'schema has sources');
SELECT has_table('stores', 'schema has stores');
SELECT has_table('availability_snapshots', 'schema has availability_snapshots');
SELECT has_table('subscriptions', 'schema has subscriptions');
SELECT has_table('notifications', 'schema has notifications');
SELECT has_index('availability_snapshots', 'idx_snapshots_latest', 'latest-snapshot index exists');
SELECT col_default_is('sources', 'last_status', 'never', 'sources.last_status defaults to never');

SELECT * FROM finish();
ROLLBACK;
