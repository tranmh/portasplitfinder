# Plan: Backend Core (domain, DB, sources, geo)

**Context:** Foundation of the Go backend for PortaSplitFinder. Pluggable data
sources (mock-first), Postgres persistence, PLZ geolocation. TDD-first.
See `../api-contract.md` and `2026-06-28-portasplitfinder-master-plan.md`.

## Deliverables

### `internal/domain` — shared types
`Product`, `Source` (type `online|store`), `Store`, `Listing`, `Subscription`,
`AvailabilitySnapshot`. Mirror the API contract field names. Pure structs + small
enums (`SourceType`). No DB/HTTP imports.

### `internal/config` — env parsing
Parse `DATABASE_URL`, `API_PORT`, `PUBLIC_BASE_URL`, SMTP_*, `SOURCES`,
`POLL_INTERVAL`, `POLLER_HEALTH_PORT`. Single `Load() (Config, error)`.

### `migrations/` — goose SQL
Tables per master plan: `products`, `sources`, `stores`, `availability_snapshots`,
`subscriptions`, `notifications`. Enums as `text + CHECK`. Indexes:
`availability_snapshots (source_key, store_id, product_id, checked_at desc)`,
`subscriptions (confirm_token)`, `(unsub_token)`, `(confirmed_at)`.
Include a `00001_init.sql` up/down.

### `internal/store` — pgx repos (one file per aggregate)
`ProductRepo`, `SourceRepo`, `StoreRepo`, `SnapshotRepo` (write snapshot + read
latest-per-key), `SubscriptionRepo` (create, confirm by token, unsub by token,
list confirmed), `NotificationRepo` (record + exists-check for dedupe).
Use `pgxpool`. Migrations runnable programmatically (goose embed) so `--seed`
and tests can set up schema.

### `internal/source` — pluggable sources
```go
type Source interface {
    Key() string
    Meta() domain.Source
    Fetch(ctx) ([]domain.Listing, []domain.Store, error)
}
```
- `mock` impl: deterministic seed — a few OBI stores (real PLZ/coords) with
  varying stock + online listings for amazon/ebay/guenstiger/idealo. Drives the
  whole app + e2e.
- `obi`, `ebay`, `amazon`, `guenstiger`, `idealo`: stub impls that return
  `ErrNotImplemented` (or empty) behind the same interface, with a `// TODO real`
  marker. A registry builds enabled sources from `SOURCES` env.

### `internal/geo`
Embed `data/plz_centroids.csv` (plz,lat,lon). `Lookup(plz) (lat, lon, ok)`,
`DistanceKm(a,b)` haversine, `WithinRadius`. If a full dataset isn't bundled,
ship a representative subset covering the seed stores + a `// TODO full dataset`.

## Tests (TDD)
- **Unit (no DB):** geo haversine (known distances, e.g. Berlin↔Munich ≈ 504 km),
  PLZ lookup, mock source determinism, config parsing, source registry.
- **Integration (`integration` tag, testcontainers Postgres):** each repo CRUD,
  snapshot latest-per-key query, subscription confirm/unsub token flow,
  notification dedupe.

## Done when
`go test ./...` green; `RUN_INTEGRATION=1 go test -tags=integration ./...` green.
