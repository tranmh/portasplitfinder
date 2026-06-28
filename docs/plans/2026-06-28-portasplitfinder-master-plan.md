# PortaSplitFinder — Implementation Plan

## Context

It's a hot summer in Germany and demand for the **Midea Portasplit** mobile air-conditioner is high. The user wants a web app that tracks where this device is available across German online stores (OBI, eBay, Amazon, guenstiger.de, and similar) and shows availability on a **map of Germany**, with a **postal-code (PLZ) filter** and an **email-notification** feature ("tell me when it's available within 50 km of my PLZ").

Stack is fixed by the user: **Go backend, Next.js/TypeScript/React frontend, Docker Compose, tests at unit/integration/e2e levels.** Work is to be done with **agents + git worktrees**, plans saved to `docs/plans/`, and a `CLAUDE.md` produced at the end.

### Decisions confirmed with the user
1. **Data acquisition: mock-first, pluggable scrapers.** A clean `Source` interface plus realistic seed/mock data so the whole system works end-to-end now. Real scrapers/API adapters (eBay/Amazon APIs, OBI/guenstiger HTML) slot in behind the interface later. This sidesteps ToS/anti-bot risk for the MVP.
2. **Map = hybrid.** OBI physical branches are map pins with per-store stock; online-only retailers (Amazon/eBay/guenstiger) appear in a side panel as "available nationwide". The PLZ + radius drives nearby-store ranking and notifications.
3. **Scope = full vertical slice.** Working app: Go backend (Source interface + mock data), Next.js map UI, PLZ filter, email subscription, Docker Compose, and unit/integration/e2e tests. Real scrapers stay stubbed behind the interface.
4. **Email = configurable SMTP (env).** Mailhog in dev via Docker Compose; any SMTP server in prod. Double opt-in confirmation (GDPR-appropriate for Germany).

### Reference projects (concepts reused)
- **`~/work/pgnize`** (Go + Next.js): pluggable backend interface (`Recognizer` → our `Source`) with a `fake` impl for tests; DB-backed job queue with `SELECT … FOR UPDATE SKIP LOCKED`; chi router; pgx/v5; goose migrations; testcontainers-go for integration; Playwright (api + ui projects); multi-profile `docker-compose.yml`; `Makefile` targets; `docs/api-contract.md`; dated plans in `docs/plans/`; worktree + TDD workflow; `CLAUDE.md`/`CODEMAP.md`. This project is essentially the pgnize template re-pointed at retail availability.
- **`~/work/swiss-manager`**: matrix-style QA docs (`docs/qa/`), Docker Compose profiles (dev default / `prod`), `.env.example` sectioning, task-checkbox plan format.

---

## Architecture

```
portasplitfinder/
├── cmd/
│   ├── api/                # REST server entrypoint
│   └── poller/             # scheduled availability checker + notifier
├── internal/
│   ├── config/             # env parsing
│   ├── httpapi/            # chi router, handlers, middleware
│   ├── domain/             # Product, Source, Listing, Store, Subscription, Snapshot
│   ├── source/             # Source interface + impls: mock (seed), obi, ebay, amazon, guenstiger, idealo (stubs)
│   ├── store/              # Postgres repos (pgx) — one file per aggregate
│   ├── geo/                # PLZ→lat/lon lookup + haversine radius
│   ├── jobs/               # poll sources, write snapshots, queue notifications
│   └── notify/             # SMTP sender (configurable) + double-opt-in tokens
├── migrations/             # goose .sql
├── data/plz_centroids.csv  # bundled German PLZ → lat/lon (no runtime API dependency)
├── web/                    # Next.js (App Router) + TS + React
│   └── src/{app,components,lib,hooks,i18n}/
├── e2e/                    # Playwright (api + ui projects)
├── docs/{api-contract.md,plans/,qa/}
├── docker-compose.yml      # postgres, api, web, mailhog (+ poller)
├── Makefile
└── CLAUDE.md, CODEMAP.md
```

### Data model (goose migrations; enums as `text + CHECK` per pgnize convention)
- `products` — the Portasplit variants we track (name, model, image).
- `sources` — retailers (key, display name, type `online|store`, base URL), last-scrape status.
- `stores` — physical branches (OBI): source_id, name, plz, lat, lon.
- `listings` / `availability_snapshots` — per (source[, store], product) result: price, in_stock, quantity (if known), url, checked_at.
- `subscriptions` — email, plz, lat, lon, radius_km (default 50), confirm_token, confirmed_at, unsub_token, created_at.
- `notifications` — sent log (subscription_id, product_id, source/store, sent_at) for dedupe.

### Geolocation
Bundle a German PLZ-centroid CSV in `data/` (~8k rows: plz, lat, lon; source e.g. OpenPLZ / public zip-centroid dataset, license noted). `internal/geo` loads it into memory; `DistanceKm(lat1,lon1,lat2,lon2)` haversine; radius filter on stores and for notification matching. (OpenPLZ API noted as a future live alternative.)

### Map (frontend)
**react-leaflet + OpenStreetMap tiles** — free, no API key. OBI store pins with stock badges; PLZ input recenters the map and draws the radius circle; online retailers in a side panel ("nationwide", price, link, last-checked). i18n de/en.

### API contract (`docs/api-contract.md`, pgnize format)
- `GET /api/health`
- `GET /api/products`
- `GET /api/availability?plz=&radius=` → `{ stores:[{store,lat,lon,distanceKm,listings[]}], online:[{source,price,url,inStock,checkedAt}] }`
- `POST /api/subscriptions` `{email,plz,radiusKm}` → creates + sends double-opt-in email
- `GET /api/subscriptions/confirm?token=` ; `GET /api/subscriptions/unsubscribe?token=`
- `GET /api/sources` (last scrape status)

### Docker health checks (automated bring-up)
Every service in `docker-compose.yml` gets a `healthcheck` so the stack starts in the right order with **zero manual steps**, and `depends_on` uses `condition: service_healthy`:
- **postgres** — `pg_isready -U $POSTGRES_USER`.
- **api** (Go) — container `HEALTHCHECK` curling its own `GET /api/health` (which also pings the DB); compose waits on this.
- **poller** — exposes a tiny `GET /healthz` (or writes a heartbeat file checked by `healthcheck`) so it reports ready/alive.
- **web** (Next.js) — `GET /` (or a `/api/health` route) returns 200.
- **mailhog** — TCP/HTTP check on `:8025`.
`api` waits for `postgres` healthy; `web` and `poller` wait for `api` healthy. Result: `docker compose up --build` reaches a fully-ready stack on its own (suitable for CI/e2e and the verification steps below). `make up` wraps `docker compose up -d --wait` to block until all healthchecks pass.

### Notifications flow
`cmd/poller` runs sources on a schedule → writes snapshots → for each newly-in-stock listing, finds confirmed subscriptions whose radius covers it (or online = everyone) → sends SMTP email (Mailhog in dev) → logs to `notifications` to avoid re-sending.

---

## Execution strategy (agents + git worktrees)

This is the workflow the user mandated; it will also be recorded to memory (see below).

1. **Scaffold + contract first (main worktree):** repo skeleton, `go.mod`, `web/` init, `docs/api-contract.md`, `docs/plans/` entries, `.env.example`, `docker-compose.yml` skeleton, `Makefile`. Freezing the API contract first lets backend and frontend proceed in parallel.
2. **Parallel build in separate git worktrees, one agent each:**
   - **Backend agent** (worktree `wt-backend`): domain, migrations, repos, `Source` interface + mock source + stubs, geo, jobs, notify, httpapi — TDD (unit no-DB, integration via testcontainers).
   - **Frontend agent** (worktree `wt-frontend`): Next.js map UI, PLZ filter, subscription form, api-client mirroring the contract, Vitest component tests.
3. **Integration worktree:** merge, wire Docker Compose (postgres + api + web + mailhog + poller), seed data, then **Playwright e2e** (map loads, PLZ filter narrows results, subscribe → confirm via Mailhog → availability email).
4. **Finalize:** `CLAUDE.md` + `CODEMAP.md`, README, `make` targets verified.

### Plans saved to `docs/plans/` (dated, pgnize/swiss-manager style, TDD-first)
- `YYYY-MM-DD-backend-core.md` (domain, migrations, repos, Source interface, geo)
- `YYYY-MM-DD-backend-api-jobs-notify.md`
- `YYYY-MM-DD-frontend-map-and-subscriptions.md`
- `YYYY-MM-DD-docker-and-e2e.md`

### Memory writes (user asked three times to "save to memory")
At the start of execution, write to `~/.claude/.../memory/`:
- **feedback**: "Test everything at unit + integration + e2e levels" — why + how to apply.
- **feedback**: "Save all implementation plans to `docs/plans/` (dated markdown)" — why + how.
- **feedback**: "Do work with agents + git worktrees" — why + how.
- **project**: PortaSplitFinder overview (stack, mock-first sources, hybrid map, SMTP notify).
Each with an entry line in `MEMORY.md`.

---

## Testing

- **Unit (Go, no DB):** Source parsers against fixture HTML/JSON in `testdata/`; haversine + PLZ lookup; notification-matching (radius + dedupe); double-opt-in token logic.
- **Integration (Go, testcontainers Postgres, `RUN_INTEGRATION=1`/build tag):** repos, poller job worker (SKIP LOCKED claim), API handlers end-to-end against real DB.
- **Frontend unit (Vitest):** MapView, PLZ filter, subscription form, api-client zod validation.
- **E2E (Playwright, api + ui projects):** deterministic via the **mock source** + **Mailhog**; flows: availability map renders with pins, PLZ filter changes results, subscribe → confirm → receive availability email.

---

## Verification (end-to-end)

1. `cp .env.example .env` then `make up` (`docker compose up -d --build --wait`) → blocks until postgres, api, web, mailhog (+ poller) all report **healthy** via their healthchecks; no manual sequencing.
2. Open `http://localhost:3000` → map of Germany with OBI pins + online side panel.
3. Enter a PLZ → map recenters, radius circle drawn, nearby stores ranked, results filtered.
4. Subscribe with email + PLZ + 50 km → confirmation email visible in Mailhog (`:8025`) → click confirm.
5. Trigger poller (or wait for schedule) → availability email arrives in Mailhog.
6. `make test` (Go unit), `make test-int` (integration), `make e2e` (Playwright) all green.

## Open / deferred (not in this slice)
- Real live scrapers + eBay/Amazon API adapters (need keys/approval) — plug in behind `Source` later.
- Live geocoding API (OpenPLZ) instead of bundled CSV.
- Auth/admin dashboard, rate-limiting hardening, CI workflow.
