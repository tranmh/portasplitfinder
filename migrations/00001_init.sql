-- +goose Up
-- +goose StatementBegin
CREATE TABLE products (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    model      text NOT NULL,
    image_url  text,
    created_at timestamptz NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE sources (
    key             text PRIMARY KEY,
    name            text NOT NULL,
    type            text NOT NULL CHECK (type IN ('online', 'store')),
    base_url        text NOT NULL,
    last_checked_at timestamptz,
    last_status     text NOT NULL DEFAULT 'never' CHECK (last_status IN ('ok', 'error', 'never')),
    last_error      text
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE stores (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_key text NOT NULL REFERENCES sources(key) ON DELETE CASCADE,
    name       text NOT NULL,
    street     text,
    plz        text NOT NULL,
    city       text NOT NULL,
    lat        double precision NOT NULL,
    lon        double precision NOT NULL,
    UNIQUE (source_key, name)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE availability_snapshots (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_key  text NOT NULL REFERENCES sources(key) ON DELETE CASCADE,
    store_id    uuid REFERENCES stores(id) ON DELETE CASCADE,
    product_id  uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    in_stock    boolean NOT NULL,
    quantity    integer,
    price_cents integer,
    currency    text NOT NULL DEFAULT 'EUR',
    url         text NOT NULL,
    checked_at  timestamptz NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_snapshots_latest
    ON availability_snapshots (source_key, store_id, product_id, checked_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE subscriptions (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL,
    plz           text NOT NULL,
    radius_km     integer NOT NULL DEFAULT 50,
    product_id    uuid REFERENCES products(id) ON DELETE CASCADE,
    confirm_token text NOT NULL,
    unsub_token   text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    confirmed_at  timestamptz,
    unsubbed_at   timestamptz
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_subscriptions_confirm_token ON subscriptions (confirm_token);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_subscriptions_unsub_token ON subscriptions (unsub_token);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX idx_subscriptions_confirmed_at ON subscriptions (confirmed_at);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE notifications (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id uuid NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    source_key      text NOT NULL,
    store_id        uuid,
    product_id      uuid NOT NULL,
    dedupe_key      text NOT NULL,
    sent_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (subscription_id, dedupe_key)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_notifications_dedupe ON notifications (subscription_id, dedupe_key);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notifications;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS subscriptions;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS availability_snapshots;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS stores;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS sources;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS products;
-- +goose StatementEnd
