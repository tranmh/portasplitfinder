# PortaSplitFinder 🌡️❄️

Find where the **Midea Portasplit** mobile air conditioner is available across
German retailers — on a map of Germany, filtered by your postal code, with email
alerts when it comes in stock near you.

It's hot in Germany and everyone wants one. This app tracks availability across
**OBI** (physical stores, shown as map pins) plus online retailers **Amazon**,
**eBay**, **guenstiger.de** and **idealo** (shown as "deutschlandweit verfügbar"),
and emails you when a device becomes available within your chosen radius.

## Quick start

```bash
cp .env.example .env
make up          # builds + starts the whole stack and waits until healthy
```

Then open:

- **App:** http://localhost:3000
- **API:** http://localhost:8080/api/health
- **Mailhog** (dev inbox): http://localhost:8025

`make down` stops everything.

## How it works

- A **Go API** (`cmd/api`) serves availability, products, sources and
  subscriptions, and seeds the catalogue on first boot.
- A **Go poller** (`cmd/poller`) periodically checks each enabled data source,
  records availability snapshots, and emails confirmed subscribers when a device
  transitions to in-stock within their PLZ radius.
- A **Next.js** frontend renders the map (Leaflet + OpenStreetMap), the PLZ
  filter with radius circle, the online-retailer panel, and the subscribe form.
- **Mailhog** captures outgoing email in development.

Data sources are **mock-first**: a deterministic mock source drives the whole
app today; real scrapers / eBay & Amazon API adapters slot in behind the same
`Source` interface later (see `CLAUDE.md` → Deferred).

## Try the notification flow

1. On the site, enter PLZ `80331` (München) → the map recenters and shows the
   nearby OBI store.
2. Subscribe with an email + PLZ `80331` + radius 50.
3. Open Mailhog (http://localhost:8025) → click the confirmation link.
4. Simulate a restock and trigger one poll:
   ```bash
   docker compose exec -e MOCK_TRANSITION=1 poller /app/poller --once
   ```
5. Back in Mailhog you'll see the "available near you" email.

## Tests

```bash
make test       # Go unit tests (no DB)
make test-int   # Go integration tests (testcontainers Postgres; needs Docker)
make up && make e2e   # Playwright end-to-end against the running stack
cd web && npm run test   # frontend unit tests (Vitest)
```

## Documentation

- `CLAUDE.md` — stack, architecture, conventions, commands.
- `CODEMAP.md` — one line per directory.
- `docs/api-contract.md` — the HTTP API spec.
- `docs/plans/` — dated implementation plans.

## Configuration

See `.env.example`. To point at a real SMTP server, set `SMTP_HOST/PORT/
USERNAME/PASSWORD/FROM` and `SMTP_TLS=true`. `SOURCES` selects enabled data
sources (default `mock`).
