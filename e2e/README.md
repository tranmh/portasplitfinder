# PortaSplitFinder e2e (Playwright)

End-to-end tests that run against the **running** docker-compose stack
(API on :8080, web UI on :3000, Mailhog on :8025).

## Prerequisites

Bring the stack up and wait until it is healthy:

```sh
make up        # docker compose up -d --build --wait
```

## Run

From the repo root:

```sh
make e2e       # cd e2e && npm ci && npx playwright test
```

Or directly inside `e2e/`:

```sh
npm install
npx playwright install chromium
npx playwright test            # all projects
npx playwright test --project=api
npx playwright test --project=ui
```

Tear the stack down when finished:

```sh
make down
```

## Projects

- **api** — headless; uses Playwright's `request` fixture against the API and
  Mailhog. Covers `tests/api/*` and the full `tests/e2e/*` notification flow.
- **ui** — chromium against `http://localhost:3000`. Covers `tests/ui/*`.

## Specs

| Spec | What it proves |
| --- | --- |
| `tests/api/health.spec.ts` | `/api/health` is 200 `{status:"ok"}`; web root is 200. |
| `tests/api/availability.spec.ts` | stores + online for PLZ 80331/r50; München filtered out for far PLZ 20095 (radius filtering); `distanceKm` present with a PLZ, null without. |
| `tests/ui/map.spec.ts` | Leaflet map (`.leaflet-container` + tiles) renders; online panel lists nationwide retailers. |
| `tests/ui/plz-filter.spec.ts` | entering PLZ 80331 updates the store list to show the München store + distance. |
| `tests/e2e/subscribe.spec.ts` | subscribe (202) → confirm mail in Mailhog → confirm token (confirmed) → poller transition → availability mail arrives. |

## Config / overrides

Base URLs default to the localhost stack and can be overridden via env:

- `API_BASE_URL` (default `http://localhost:8080`)
- `WEB_BASE_URL` (default `http://localhost:3000`)
- `MAILHOG_BASE_URL` (default `http://localhost:8025`)

## Notes / determinism

- The subscribe spec uses a unique email per run and clears Mailhog in setup.
- The availability notification only fires on an out→in-stock transition for a
  confirmed subscriber whose PLZ+radius covers München. The `poll.ts` helper
  therefore first runs the poller **without** `MOCK_TRANSITION` (baseline:
  out-of-stock), then **with** `MOCK_TRANSITION=1` (fires the mail), via
  `docker compose exec poller /app/poller --once`.
- The suite runs serially (`workers: 1`) because it shares the single running
  poller + Mailhog instance.
