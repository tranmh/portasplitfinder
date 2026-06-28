# Plan: First real source adapter — OBI (obi.de)

## Context

PortaSplitFinder currently runs on a deterministic **mock** source behind a
pluggable `Source` interface (`internal/source/source.go`); `obi`, `ebay`,
`amazon`, `guenstiger`, `idealo` are stubs returning `ErrNotImplemented`
(`internal/source/stubs.go`). The user wants the first **real** adapter
implemented, in the order **obi → ebay → idealo/guenstiger**, tested with
**offline fixtures + an env-gated live test**. This plan implements **OBI**.

### What I verified (grounding)
- OBI has **no public availability API**. Product pages live at
  `https://www.obi.de/p/<articleNo>/<slug>` (our exact product, the Midea
  PortaSplit, is article **8620890**); search is `https://www.obi.de/search/<term>/`.
- OBI product pages carry **schema.org `Product` JSON-LD** (`<script
  type="application/ld+json">`) with an `offers` object → `price`,
  `priceCurrency`, `availability` (`…/InStock` | `…/OutOfStock`). This is the
  stable scraping target.
- **OBI is bot-protected**: server-side fetches (WebFetch) return **404/403** to
  non-browser clients. So live OBI **cannot be relied on in CI** — confirming the
  fixtures-first approach — and the adapter must **degrade gracefully** when blocked.
- The availability endpoint (`internal/httpapi/availability.go`) splits
  `stores` vs `online` **purely by `StoreID` presence**, not by source type. A
  listing with `StoreID=nil` renders in the **online** panel keyed by its source.

### v1 scope decision (please veto if undesired)
v1 reports **obi.de webshop (online) availability** for tracked products: one
listing per product, `StoreID=nil`, shown in the online panel as "deutschlandweit
verfügbar" (accurate — OBI ships nationwide). OBI's `Meta().Type` becomes
`online` to match. **Per-market store-level stock (true OBI map pins) is
deferred** — it needs OBI's market-availability XHR endpoint, a store directory
with geocoding, and a bot bypass; that's a separate, larger effort.

## Design

### Interface change (enables every real source)
Real sources must know *which* products to look up and which `ProductID` to tag.
Change:
```go
Fetch(ctx context.Context, products []domain.Product) ([]domain.Listing, []domain.Store, error)
```
- `internal/source/mock.go`: accept and ignore the arg (keeps its own catalogue).
- `internal/source/stubs.go`: update the 4 remaining stub signatures.
- `internal/jobs/poller.go`: add `ListProducts` to the `jobs.Store` interface
  (already implemented on `*DB` in `internal/store/adapter.go:16`), load products
  once per cycle in `PollOnce`, and pass them to each `Fetch`.
- Update every `Fetch(...)` caller in tests: `source_test.go`,
  `jobs/poller_test.go`, `jobs/poller_integration_test.go`.

### New: shared HTTP helper — `internal/source/httpx.go`
Reusable by eBay/idealo later. Browser-like `User-Agent`, `Accept-Language:
de-DE`, configurable timeout (~15s), retry with exponential backoff (≤3 tries) on
5xx/timeouts, and a typed **`ErrBlocked`** for 403/404/429. The poller already
records per-source error status and continues, so a block is non-fatal.

### New: `internal/source/obi.go` (replaces the stub)
`obiSource` holds an HTTP client (via httpx) and resolves each tracked product to
an OBI product URL:
1. **Known mapping** first (e.g. product whose Name/Model matches the Midea
   PortaSplit → `/p/8620890/...`). Small default table, documented.
2. **Search fallback**: GET `/search/<name>/`, parse the first product link.

Then GET the product page and parse JSON-LD:
- Extract all `application/ld+json` blocks (handle `@graph` arrays and
  `AggregateOffer`), find the `Product`, read `offers.price`,
  `offers.priceCurrency`, `offers.availability`.
- Emit **one `domain.Listing`** per product: `SourceKey:"obi"`, `StoreID:nil`,
  `InStock` from availability, `PriceCents` from price×100, `Currency`, `URL`.
- On `ErrBlocked`/parse failure for a product: skip it (and surface a source-level
  error if *all* fail), never panic.

Use **goquery** (`github.com/PuerkitoBio/goquery`) for HTML parsing + `encoding/json`
for the JSON-LD. `Meta()` → `Type: online`, name "OBI", baseURL `https://www.obi.de`.

### Config / wiring
- No new required env. Keep default `SOURCES=mock`; document `SOURCES=mock,obi`
  to enable OBI. No docker-compose change (SOURCES already passed; containers have
  outbound network — OBI will likely be **blocked** there, logging a source error
  while the app keeps working on mock data).
- Optional `OBI_LIVE=1` gates the live test.

## Tests (all three levels; TDD)
- **Unit (offline, deterministic):** parser vs `internal/source/testdata/obi_*.html`
  fixtures — InStock, OutOfStock, `@graph`, `AggregateOffer`, and a 404/blocked
  body; httpx retry/backoff + `ErrBlocked`; product→URL mapping + search fallback.
  Fixtures are representative schema.org Product pages (captured live if the Go
  client's browser UA isn't blocked; otherwise hand-built and clearly marked,
  since OBI blocks automated capture).
- **Integration:** existing testcontainers suite stays green (update Fetch
  callers). Add an **env-gated live test** (`//go:build integration` + `OBI_LIVE=1`)
  that fetches the real PortaSplit URL and **soft-skips on block**.
- **E2E (Playwright):** unchanged — mock still drives it, so the 8 specs stay green.
- **Frontend (Vitest):** unaffected.

## Critical files
- Change: `internal/source/source.go` (interface), `internal/source/mock.go`,
  `internal/source/stubs.go`, `internal/jobs/poller.go`.
- New: `internal/source/obi.go`, `internal/source/httpx.go`,
  `internal/source/obi_test.go`, `internal/source/httpx_test.go`,
  `internal/source/testdata/obi_*.html`.
- Touch tests: `internal/source/source_test.go`, `internal/jobs/poller_test.go`,
  `internal/jobs/poller_integration_test.go`.
- Docs: new `docs/plans/2026-06-28-obi-source-adapter.md` (this plan), a note in
  `docs/api-contract.md` (OBI = online webshop availability for now) and `CLAUDE.md`
  (real-sources progress). `go.mod`: add goquery.

## Execution (agents + git worktrees, per project workflow)
Implement in a dedicated worktree `wt-obi-source` via one agent (TDD), then merge
to `main`. Save the plan to `docs/plans/` first.

## Verification (end-to-end)
1. `go test ./...` — unit incl. OBI parser + httpx — green.
2. `RUN_INTEGRATION=1 go test -tags=integration ./...` — green (testcontainers).
3. `SOURCES=mock,obi go run ./cmd/poller --once` — if OBI isn't blocked, an `obi`
   entry appears in `/api/availability` (online panel); if blocked, `/api/sources`
   shows `obi` `lastStatus:error` and the app still works on mock data.
4. `OBI_LIVE=1 go test -tags=integration ./internal/source/ -run Live` — best-effort
   real fetch (soft-skips on block).
5. `make up && make e2e` (8 specs) and `cd web && npm run test` (28) still green.
