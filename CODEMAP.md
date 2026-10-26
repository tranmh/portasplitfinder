# CODEMAP.md

One line per directory. See `CLAUDE.md` for conventions and `docs/api-contract.md`
for the API spec.

## Backend (Go)

- `cmd/api/` — REST server entrypoint; runs migrations + auto-seed on boot; flags `--healthcheck`/`--seed`/`--migrate-only`. Has its `Dockerfile`.
- `cmd/poller/` — scheduled scrape + notify loop; `/healthz`; flags `--once`/`--healthcheck`. Has its `Dockerfile`.
- `internal/domain/` — shared structs + enums mirroring the API contract.
- `internal/config/` — env parsing (`Load()`).
- `internal/source/` — pluggable `Source` interface; `mock` (deterministic seed) + real per-market store adapters `obi` and `bauhaus` (true map pins; `bauhaus` is scoped to 50 km around PLZ 73035 and fetches via an optional web-unblocker — `httpx.WithUnblocker`/`UNBLOCKER_URL` — degrading gracefully on Cloudflare 403) + `ebay/amazon/guenstiger/idealo` stubs; registry built from `SOURCES`.
- `internal/geo/` — embedded PLZ-centroid CSV (`data/plz_centroids.csv`), haversine `DistanceKm`, `Lookup`, `WithinRadius`.
- `internal/store/` — pgx/pgxpool repositories (one file per aggregate), embedded-migration runner, seed.
- `internal/jobs/` — the poller: snapshots, transition detection, notification matching + dedupe; `LastPoll` for `/healthz`.
- `internal/notify/` — `Mailer` interface, SMTP impl, fake mailer, German email templates, token generation.
- `internal/httpapi/` — chi router, handlers, request validation, contract error shape.
- `migrations/` — goose SQL migrations (`00001_init.sql`) + `go:embed`.

## Frontend (`web/`, Next.js)

- `web/src/app/` — App Router pages (`page.tsx`, `layout.tsx`) + `api/health` route for the container healthcheck.
- `web/src/components/` — `MapView`/`MapPanel` (Leaflet, dynamic ssr:false), `PlzFilter`, `StoreList`, `OnlinePanel`, `StoreSearch`, `SubscribeForm`, `StockBadge` (+ co-located `*.test.tsx`).
- `web/src/hooks/` — `useAvailability` (refetch on plz/radius), `useDebouncedValue`.
- `web/src/lib/` — `api-client.ts` (typed fetchers + Zod schemas mirroring the contract), `format.ts`.
- `web/src/i18n/` — `de` (default) + `en` message dictionaries.
- `web/src/test/` — shared test fixtures.

## Tests & docs

- `e2e/` — Playwright suite (`api` + `ui` projects); deterministic via mock source + Mailhog.
- `docs/api-contract.md` — authoritative HTTP API spec.
- `docs/plans/` — dated implementation plans (master + per-phase).
- Go tests are co-located (`*_test.go`; integration behind `//go:build integration`).
