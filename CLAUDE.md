# CLAUDE.md — PortaSplitFinder

Guidance for working in this repo. Keep it accurate as the code changes.

## What this is

Web app that tracks availability of the **Midea Portasplit** mobile air
conditioner across German retailers and shows it on a **map of Germany**, with a
**postal-code (PLZ) filter** and **email notifications** ("tell me when it's
available within N km of my PLZ", default 50 km).

## Stack

- **Backend:** Go 1.22 — chi router, pgx/v5 (pgxpool), goose migrations
  (embedded), PostgreSQL 16. Two binaries: `cmd/api` (REST) and `cmd/poller`
  (scheduled scraper + notifier).
- **Frontend:** Next.js 15 (App Router) + TypeScript (strict) + React 19,
  react-leaflet 5 + Leaflet with OpenStreetMap tiles (no API key), Tailwind,
  Zod for response validation.
- **Email:** configurable SMTP (env). Mailhog in dev. Double opt-in (GDPR).
- **Infra:** Docker Compose (db, mailhog, api, poller, web) — every service has a
  healthcheck so `make up` brings the stack to ready unattended.

## Commands (Makefile)

- `make up` — build + start the whole stack and **wait until healthy**
  (`docker compose up -d --build --wait`). `make down` to stop.
- `make test` — Go unit tests (no DB).
- `make test-int` — Go integration tests (testcontainers Postgres;
  `RUN_INTEGRATION=1 go test -tags=integration ./...`). Requires Docker.
- `make e2e` — Playwright e2e against a running stack (`make up` first).
- `make lint` — `go vet ./...` + `cd web && npm run lint`.
- `make seed` — seed catalogue (also auto-seeds on api boot when empty).
- `make build` / `make api-dev` / `make web-dev`.

Frontend: `cd web && npm run dev | build | test | lint`.

## Architecture

- `cmd/api` — runs migrations on boot, auto-seeds the catalogue when empty,
  serves the API. Flags: `--healthcheck` (used by Docker), `--seed`,
  `--migrate-only`.
- `cmd/poller` — every `POLL_INTERVAL`: fetch each enabled source → upsert stores
  → write `availability_snapshots` → detect out→in-stock transitions → email
  matching confirmed subscriptions (deduped). Flags: `--once`, `--healthcheck`.
  Exposes `/healthz`. **Depends on api being healthy** (the api seeds the
  catalogue the poller's snapshots FK-reference).
- `internal/source` — the pluggable `Source` interface
  (`Fetch(ctx, products)` — sources are given the tracked products so they can
  search and tag listings with the right `ProductID`). `mock` provides
  deterministic seed data driving the whole app. **`obi` is a real
  per-market store adapter** (`Type=store`): OBI's webshop does not ship the
  tracked products home, so availability is reserve & collect only. It fetches
  the German market directory
  (`/api/disc/store/locator/country/DE` → ~357 markets with lat/lon), then
  queries per-market stock for each tracked article
  (`/api/disc/article-service-proxy/store-specific-articles/v2/DE/de/{storeNumber}?articleNumbers=…`
  → `storeSpecificArticle.stock`/`price`), emitting one store-keyed listing per
  market (real map pins). Store ids are deterministic UUIDv5 from the OBI store
  number. It degrades gracefully when bot-blocked (per-market failures are
  skipped; a source-level error is returned only if the directory or *every*
  market lookup fails). Note: one availability request per market per tracked
  product per poll (~357 at default `POLL_INTERVAL=5m`, bursted with bounded
  concurrency). `ebay/amazon/guenstiger/idealo` are still stubs
  (`ErrNotImplemented`). Enabled set comes from `SOURCES` (e.g. `mock,obi`).
  `internal/source/httpx.go` is the shared HTTP helper (browser UA, backoff,
  typed `ErrBlocked`) for real adapters.
- Each source self-registers its catalogue row: the poller upserts `src.Meta()`
  before writing snapshots, so a real source enabled via `SOURCES` needs no
  separate seed step.
- `internal/geo` — embedded PLZ→lat/lon CSV + haversine radius math.
- `internal/store` — pgx repos, one file per aggregate.
- See `CODEMAP.md` for a one-line-per-directory map and `docs/api-contract.md`
  for the authoritative API spec.

## Hard rules / conventions

- **Mock-first, pluggable sources.** Real scraping/APIs go behind the `Source`
  interface; never break the interface. Keep the `mock` source deterministic.
- **The mock stock transition** (for tests): the München OBI listing is
  out-of-stock by default; flip it in-stock with `MOCK_TRANSITION=1` (or
  `source.NewMockWithTransition(true)`). With a prior out-of-stock snapshot in
  the DB, the next poll fires the availability email. Product/store IDs are fixed
  deterministic UUIDs so seed, poller, and e2e agree.
- **Server is authoritative:** validate PLZ (5 digits) / radius (1..500) / email
  server-side regardless of the client.
- **Postgres enums** as `text + CHECK` (goose-friendly), not native enums.
- **Test at all three levels:** unit (no DB) + integration (testcontainers
  Postgres, `integration` build tag) + e2e (Playwright; deterministic via mock
  source + Mailhog). Prefer TDD.
- **Money** is integer euro cents (`priceCents`) + `currency`; `null` = unknown.
- **Timestamps** are RFC 3339 UTC.

## Workflow

- Implementation is done with **subagents in git worktrees**, parallelized
  (backend vs frontend) after the API contract is frozen, then integrated.
- Plans live in `docs/plans/` as dated markdown (`YYYY-MM-DD-<feature>.md`).

## Config

Copy `.env.example` → `.env`. Key vars: `DATABASE_URL`, `API_PORT`,
`PUBLIC_BASE_URL` (used to build email confirm/unsub links),
`NEXT_PUBLIC_API_BASE_URL`, `SMTP_*`, `SOURCES` (e.g. `mock`), `POLL_INTERVAL`.

## Deferred (not yet built)

Real adapters for eBay (Browse API), idealo, guenstiger, Amazon (next, in that
rough order); full ~8000-row PLZ dataset (a representative subset is bundled);
auth/admin; rate-limiting hardening; CI workflow.

Done: OBI per-market store-level stock (true map pins) — see `internal/source/obi.go`.
