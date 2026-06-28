# Plan: Docker Compose, Healthchecks, E2E

**Context:** Wire the whole stack into Docker Compose with healthchecks for
hands-off bring-up, then Playwright e2e. See master plan + `../api-contract.md`.

## Dockerfiles
- `cmd/api/Dockerfile` — multi-stage: build static Go `api` → `gcr.io/distroless`
  or `alpine`. Binary at `/app/api`. Healthcheck uses `/app/api --healthcheck`.
- `cmd/poller/Dockerfile` — same pattern, `/app/poller`, `--healthcheck`.
- `web/Dockerfile` — Next.js standalone output; needs `wget` for healthcheck
  (alpine base). Exposes 3000; healthcheck GETs `/api/health`.

## docker-compose.yml (already scaffolded)
Services: `db` (pg_isready), `mailhog` (wget :8025), `api` (`--healthcheck`,
depends_on db healthy), `poller` (`--healthcheck`, depends_on db+mailhog healthy),
`web` (wget /api/health, depends_on api healthy). Verify `docker compose up -d
--build --wait` reaches all-healthy unattended. `api` runs migrations + seed on
boot (or a one-shot `seed` step) so the stack has data.

## E2E (`e2e/`, Playwright)
Two projects: `api` (request-context, headless) and `ui` (chromium).
Deterministic via the **mock** source + **Mailhog** (assert mail via its HTTP API
`http://localhost:8025/api/v2/messages`).

Specs:
- `health.spec.ts` — /api/health 200; web reachable.
- `availability.spec.ts` — GET /api/availability returns stores+online; with a
  seed PLZ, radius filtering changes the store set.
- `map.spec.ts` (ui) — page loads, Leaflet map + markers render, online panel shows.
- `plz-filter.spec.ts` (ui) — enter PLZ → map recenters, store list updates.
- `subscribe.spec.ts` (ui+api) — submit subscribe form → 202 → confirmation mail
  in Mailhog → open confirm link → confirmed. Then trigger poller `--once` (or
  toggle stock in seed) → availability mail arrives in Mailhog.

## CI-friendly
`make up` (wait healthy) → `make e2e` → `make down`. Document in README/CLAUDE.md.

## Done when
`make up` brings the stack to healthy unattended; all Playwright specs pass.
