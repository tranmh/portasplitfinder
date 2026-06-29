# Plan: BAUHAUS per-market store source adapter (`bauhaus`)

## Context

PortaSplitFinder already tracks the Midea PortaSplit at OBI markets and shows
per-market reserve & collect availability as real map pins
(`internal/source/obi.go`, `Type=store`). bauhaus.info carries the **same
device** and, like OBI, exposes per-store local availability across all German
BAUHAUS Fachcentren. The user wants the same treatment for BAUHAUS: a new
pluggable `Source` that emits one store-keyed listing per Fachcentrum, producing
real map pins — implemented behind the existing `Source` interface, mirroring
OBI exactly.

**Key difference from OBI (decided):** bauhaus.info is behind **Cloudflare Bot
Management** and returns HTTP 403 to plain Go HTTP clients even with a browser
User-Agent (OBI only needed the browser UA). Per user decision, we implement the
full adapter and **degrade gracefully on 403** — identical to OBI's bot-blocking
handling: `httpx` maps 403 → `ErrBlocked`, the source returns a source-level
error, the poller records `StatusError`, no pins appear, nothing crashes. Live
data flows once a Cloudflare bypass is added later (deferred, not in scope). No
new dependencies; no TLS-impersonation or headless browser.

## Confirmed BAUHAUS internals (reverse-engineered from the site's JS bundle)

- **Product id / SKU** is the `/p/{id}` segment. Known Midea PortaSplit SKUs:
  `31934233` (PortaSplit 12.000 BTU cool+heat) and `33946696` (PortaSplit Cool
  8.000 BTU). Product URL form: `/{category}/{slug}/p/{productId}`.
- **Store directory:** `GET /api/stores-search` → stores with
  `{ storeId, name, geolocation:{ latitude, longitude }, address{…}, country_code }`.
  `storeId` is the zero-padded store number (e.g. `"520"` München). Geolocation
  gives real lat/lon → real pins.
- **Per-store availability:**
  `GET /api/product-availability/availability-detail/{productId}?storeId={storeId}&searchRadius={radius}`
  → an `availabilityMap` keyed by purchase option: `"reservation"` (reserve &
  collect — the OBI-equivalent branch) and `"online-order"` (ship home). Each
  branch carries a stock state (`AVAILABLE`, `BACK_IN_STOCK`, `ready_for_pickup`,
  `PICKUP_TIME_ENABLED`, …). Error envelope: `{status_code, error_code}`.
- Some response field names (exact casing of `stores-search`, whether `storeId`
  query wants the bare number) are inferred from the bundle, not live-verified
  (Cloudflare 403'd every probe). The decoder is written defensively (tolerant
  struct tags + fallbacks) so live shape drift degrades to "skip", not panic.

## Approach

Add a new `bauhausSource` modeled directly on `obiSource`, register it, and wire
it into config/docs. **Type = store**, one store-keyed listing per Fachcentrum
based on the `reservation` branch of `availabilityMap` (the local-pickup
equivalent of OBI reserve & collect). The frontend needs **no changes** —
`sourceKey` is an opaque string and store listings already render as generic map
pins (`web/src/components/MapView`/`OnlinePanel`), confirmed by grep.

Implementation is done by a **subagent in a git worktree** (per repo workflow),
TDD: write `bauhaus_test.go` against an `httptest.Server` first, then the
adapter until green.

### Files to create

- **`internal/source/bauhaus.go`** — the adapter. Mirror `obi.go` structure:
  - `bauhausSource{ client *Client; baseURL string }`, `newBauhausSource()`
    (base `https://www.bauhaus.info`), `base()`, `httpClient()` helpers — copy
    from OBI.
  - `Key() == "bauhaus"`; `Meta()` → `domain.Source{Key:"bauhaus",
    Name:"BAUHAUS", Type:domain.SourceStore, BaseURL:"https://www.bauhaus.info",
    LastStatus:domain.StatusNever}`.
  - A **fresh** `bauhausStoreNamespace = uuid.MustParse(<new fixed UUID>)` (NOT
    OBI's) and `bauhausStoreID(storeNo) = uuid.NewSHA1(ns, []byte("bauhaus:store:"+storeNo))`.
  - `knownBauhausArticles` table mirroring `knownObiArticles`: match Midea +
    portasplit → SKU `31934233` (primary), with a second entry for `33946696`
    handled via the same matcher list. Product page URL built from base + `/p/{sku}`.
    Search fallback (`searchArticle`) is best-effort via `/search?q=` (or
    `/api/suggester`); on block/no-match it returns an error and the product is
    skipped (no source error) — exactly like OBI.
  - `Fetch(ctx, products)`: resolve products → SKUs (skip unmappable), fetch
    directory once via `fetchMarkets`, build placeable `domain.Store`s
    (`geolocation` lat/lon; fall back to `geo.Lookup(plz)` when zero, skip if
    unplaceable — reuse the OBI placement logic), then query per-(product,store)
    availability concurrently with a `bauhausAvailabilityWorkers = 8` semaphore
    + mutex. Same error policy: directory failure or **all** lookups failing →
    source-level error (so a Cloudflare 403 wave surfaces as `StatusError`);
    partial failures are skipped.
  - `fetchMarketListing(...)`: GET the availability-detail URL, decode the
    `availabilityMap`, read the `reservation` branch → `InStock = state ∈
    {AVAILABLE, BACK_IN_STOCK, ready_for_pickup}`. `Quantity` = nil (BAUHAUS
    reports states, not counts — `null` = unknown per repo convention).
    `PriceCents` set only if a price field is present, else nil. `StoreID` =
    pointer to the deterministic store id; `Currency:"EUR"`; `URL` = product page.
  - JSON shape structs: `bauhausStoreLocator`/`bauhausStore` (with
    `geolocation.latitude/longitude`, `storeId`, `name`, nested `address`) and
    `bauhausAvailabilityResp` (the `availabilityMap`). Reuse `firstNonEmpty` /
    `firstNonZero` helpers already in `obi.go`.

- **`internal/source/bauhaus_test.go`** — unit tests mirroring `obi_test.go`
  (no DB): `httptest.Server` serving a small `stores-search` directory + canned
  availability-detail responses. Assert: Meta; per-store placement (geolocation
  + PLZ fallback + skip-unplaceable); in-stock vs out-of-stock from the
  `reservation` state; deterministic store ids; directory-failure → source
  error; all-stores-fail (403) → source error; empty/unmappable products → no
  error; store/listing referential integrity. Use `NewClient(WithBaseBackoff(0),
  WithHTTPClient(srv.Client()))` like `newTestOBI`.

- **`internal/source/bauhaus_live_test.go`** — `//go:build integration`, gated
  on `BAUHAUS_LIVE=1`, mirroring `obi_live_test.go`: best-effort real fetch that
  **soft-skips on `ErrBlocked`** (expected, given Cloudflare) and otherwise
  asserts non-empty stores/listings.

### Files to edit

- **`internal/source/source.go`** — add `"bauhaus": func() Source { return
  newBauhausSource() }` to the `builders` map.
- **`.env.example`** — extend the `SOURCES` comment and value note for `bauhaus`
  (keep default as-is or document `mock,obi,bauhaus`); do not silently change the
  running default beyond what the user expects.
- **`CLAUDE.md`** — under `internal/source`, document the new `bauhaus` adapter
  (Type=store, endpoints, Cloudflare-403 graceful degradation) next to the OBI
  paragraph; move the line out of "Deferred" / into "Done".
- **`CODEMAP.md`** — add the `bauhaus.go` one-liner.
- **`docs/plans/2026-06-29-bauhaus-source-adapter.md`** — commit this plan
  (repo convention: dated plan per feature).
- Optional: a memory note `bauhaus-endpoints-recon.md` capturing the confirmed
  endpoints + Cloudflare caveat (sibling to `obi-endpoints-recon.md`).

## Out of scope / deferred

- **Cloudflare bypass** (uTLS/curl-impersonate or headless browser) — needed for
  live data; tracked as a follow-up. The adapter is correct and tested now and
  starts returning pins the moment a bypass lands.
- The `online-order` (ship-home) branch of `availabilityMap` — could later add a
  nationwide online listing, but this feature mirrors OBI (store pins only).

---

# Phase 2: Web-unblocker transport + 73035/50 km scoping

## Context (Phase 2)

Empirical testing proved bauhaus.info's `/api/*` data routes are blocked at the
IP/network level by Cloudflare (managed JS challenge + per-IP WAF). TLS
impersonation, a real headless Chrome holding `cf_clearance`, an 11-min cooldown,
and 1-request/min pacing **all still got 403** from this environment's IP. There
is no robust *free/self-contained* bypass — robustness means externalizing
Cloudflare's arms race to a vendor whose business is keeping up with it. So:
route the real adapters' fetches through a **commercial web-unblocker API**
(ScrapingBee / ZenRows / Oxylabs / Bright Data) via a provider-agnostic URL
template, and scope BAUHAUS to PLZ **73035 + 50 km** at low cadence to keep cost
to the cheapest tier. Adapter logic is unchanged; only the transport swaps.

## Design

**Provider-agnostic transport (no new dependency — plain `net/http`).** The user
supplies one env var, `UNBLOCKER_URL`, a template containing a `{{url}}`
placeholder, e.g.:
- ScrapingBee: `https://app.scrapingbee.com/api/v1/?api_key=KEY&premium_proxy=true&country_code=de&url={{url}}`
- ZenRows: `https://api.zenrows.com/v1/?apikey=KEY&js_render=true&antibot=true&premium_proxy=true&proxy_country=de&url={{url}}`

When set, the shared `httpx.Client` rewrites every outbound GET to
`strings.Replace(template, "{{url}}", url.QueryEscape(target), 1)` and fetches
that instead, returning the unblocked target's body/status. When unset, behavior
is exactly today's (direct fetch → `ErrBlocked` on 403) — graceful fallback.

### Files to edit

- **`internal/source/httpx.go`**
  - Add `WithUnblocker(template string) ClientOption` storing `unblocker string`
    on `Client`.
  - In `try()` (the single-request path), if `c.unblocker != ""`, fetch
    `applyUnblocker(c.unblocker, target)` instead of `target` (substitute +
    `url.QueryEscape`). Keep all retry/`ErrBlocked` logic identical — a vendor
    that still returns 403 degrades exactly as today.
- **`internal/source/source.go`**
  - Add package var `unblockerTemplate string` + `func SetUnblocker(t string)`.
    Real-source constructors apply it. (Keeps the `Source` interface and
    `Build()` signature untouched — the hard rule holds.)
- **`internal/source/obi.go` & `bauhaus.go`** — in `newOBISource` /
  `newBauhausSource`, build the client with `NewClient(WithUnblocker(unblockerTemplate))`
  so both real adapters route through the unblocker when configured.
- **`internal/source/bauhaus.go`** — BAUHAUS scoping (the part the user asked
  for):
  - Add `const bauhausAnchorPLZ = "73035"` and `const bauhausRadiusKm = 50`.
  - After building placeable stores, **filter** to those within 50 km of
    `geo.Lookup(bauhausAnchorPLZ)` using the existing `geo.WithinRadius(a, b,
    radiusKm)` (`internal/geo/geo.go:97`). If the anchor PLZ can't be resolved,
    skip filtering (defensive).
  - Be gentle: drop `bauhausAvailabilityWorkers` from 8 → **2** (with the unblocker
    rotating IPs, heavy concurrency is unnecessary and impolite; ~10 markets × 1–2
    SKUs stays small).
- **`internal/config/config.go`** — add `UnblockerURL string` parsed from
  `getEnv("UNBLOCKER_URL", "")`.
- **`cmd/poller/main.go`** — call `source.SetUnblocker(cfg.UnblockerURL)` just
  before `source.Build(cfg.Sources)` (line ~57).
- **`.env.example`** — document `UNBLOCKER_URL` with the two example templates
  above and a note that without it BAUHAUS degrades to `ErrBlocked`.
- **`CLAUDE.md`** — document the unblocker transport + the 73035/50 km scoping;
  move the Cloudflare item out of "Deferred" (now addressed via unblocker).

### Tests

- **`internal/source/httpx_test.go`** — add: with `WithUnblocker(template)`
  pointed at an `httptest.Server`, assert the server receives the rewritten URL
  carrying the URL-encoded target, and the target body is returned; with no
  template, the request goes direct (unchanged). A 403 from the unblocker still
  yields `ErrBlocked`.
- **`internal/source/bauhaus_test.go`** — add a radius test: a directory with
  markets near 73035 and far away asserts only the near ones become stores
  (`geo.WithinRadius`). Existing tests adjusted so their fixture stores fall
  within the 50 km window (or set a near-73035 anchor in the test directory).
- **`internal/config/config_test.go`** — assert `UNBLOCKER_URL` parses.

## Verification (Phase 2)

1. **Unit:** `make test` — new httpx/bauhaus/config tests green; existing unaffected.
2. **Lint/build:** `make lint` + `make build`.
3. **Live (you, with a key):** set `UNBLOCKER_URL` (free-trial key from
   ScrapingBee/ZenRows) + `SOURCES=mock,obi,bauhaus`, run `cmd/poller --once`;
   confirm BAUHAUS returns real pins for Fachcentren near 73035. Without a key it
   degrades to `ErrBlocked` (StatusError), app unaffected.
4. A throwaway live check of the unblocker against `/api/stores-search` can
   confirm the provider+params before wiring (done in scratchpad, not committed).

---

## Verification

1. **Unit:** `make test` (or `go test ./internal/source/...`) — new
   `bauhaus_test.go` green; existing OBI/mock tests unaffected.
2. **Lint/build:** `make lint` (`go vet ./...`) and `make build` succeed.
3. **Registry:** confirm `SOURCES=mock,obi,bauhaus` builds without "unknown
   source key" (covered by a `config`/`source.Build` assertion or a quick
   `go test ./internal/config/...`).
4. **Graceful degradation (key behavior):** `make up` with
   `SOURCES=mock,obi,bauhaus`; poller logs show BAUHAUS returning `ErrBlocked`/
   `StatusError` (Cloudflare) **without crashing**, while mock/OBI keep working
   and the map still renders. The `sources` catalogue row for `bauhaus` exists
   (self-registered via `Meta()`).
5. **Live (optional, will likely soft-skip):** `BAUHAUS_LIVE=1 RUN_INTEGRATION=1
   go test -tags=integration -run TestBauhaus_Live ./internal/source/` — skips on
   block; if a bypass exists, asserts real pins.
6. **e2e unaffected:** `make e2e` still passes (mock-driven; BAUHAUS off or
   degraded doesn't change deterministic fixtures).
