# Plan: Backend API, Poller Jobs, Notifications

**Context:** HTTP API (`cmd/api`), scheduled poller (`cmd/poller`), and SMTP
email notifications. Builds on backend-core. See `../api-contract.md`.

## `internal/httpapi` (chi)
Handlers implementing every endpoint in the contract:
- `GET /api/health` — pings DB; 200 `{status,db,time}` / 503 degraded.
- `GET /api/products`, `GET /api/sources`.
- `GET /api/availability?plz=&radius=&productId=` — read latest snapshots, join
  stores, compute `distanceKm` via `internal/geo`, filter+sort by radius,
  split `stores` vs `online`. Validate plz (5 digits) / radius (1..500).
- `POST /api/subscriptions` — validate email/plz/radius; create unconfirmed sub
  with `confirm_token` + `unsub_token`; send double-opt-in email; always 202.
- `GET /api/subscriptions/confirm?token=`, `.../unsubscribe?token=`.
Middleware: request logging, recoverer, JSON error helper (contract error shape),
permissive CORS for the web origin.

## `cmd/api`
Flags: `--seed` (seed products/sources/stores via mock source, then exit),
`--healthcheck` (HTTP GET own /api/health, exit 0/1 — used by Docker healthcheck),
`--migrate-only`. Runs goose migrations on boot, then serves.

## `internal/notify`
`Mailer` interface; SMTP impl (`net/smtp` or `go-mail`), config-driven
(`SMTP_TLS` toggles plain vs TLS). `confirmEmail` + `availabilityEmail`
templates (German), both include unsubscribe link from `unsub_token` and
`PUBLIC_BASE_URL`. Provide a `fake` mailer (captures messages) for tests.

## `internal/jobs` + `cmd/poller`
- Poll loop every `POLL_INTERVAL`: for each enabled source, `Fetch`, upsert stores,
  write `availability_snapshots`, update source `last_*`.
- After writing, detect **transitions to in-stock** (compare to previous latest
  snapshot). For each newly-in-stock listing, find confirmed subscriptions where
  (online listing → all subs for that product) or (store listing → subs whose
  radius covers the store via geo). Send `availabilityEmail`, record in
  `notifications` to dedupe (skip if already sent for that sub+listing+day).
- `cmd/poller` flags: `--healthcheck` (checks `POLLER_HEALTH_PORT /healthz`),
  `--once` (single poll, for tests/CI). Expose `GET /healthz` returning 200 while
  alive + last-poll age.

## Tests
- **Unit:** availability assembly (sort/split/distance) with fake repos;
  notification matching (radius hit/miss, online=everyone, dedupe); email template
  rendering; token generation.
- **Integration:** full poller run against testcontainers Postgres with mock
  source + fake mailer → asserts snapshots written and the right emails queued;
  API handler tests hitting real DB (availability filtering by plz/radius;
  subscription create→confirm→unsub).

## Done when
All endpoints match the contract; `make test` + `make test-int` green;
`--healthcheck` works for both binaries.
