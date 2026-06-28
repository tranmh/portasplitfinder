# PortaSplitFinder — API Contract

Authoritative spec for the HTTP API exposed by the Go backend (`cmd/api`). The
frontend `web/src/lib/api-client.ts` mirrors these shapes with Zod schemas.

- Base path: `/api`
- Content type: `application/json; charset=utf-8`
- All timestamps are RFC 3339 / ISO 8601 UTC strings (e.g. `2026-06-28T10:00:00Z`).
- Money is an integer number of **euro cents** (`priceCents`) plus a `currency`
  string (always `EUR` for now). `null` price means "price unknown".
- Errors use a single shape (see below).

## Shared types

```jsonc
// Product — a tracked Portasplit variant
Product {
  id: string,            // uuid
  name: string,          // "Midea Portasplit MAW-..."
  model: string,         // manufacturer model code
  imageUrl: string|null
}

// Source — a retailer
Source {
  key: string,           // "obi" | "ebay" | "amazon" | "guenstiger" | "idealo" | "mock"
  name: string,          // display name
  type: "online" | "store",
  baseUrl: string,
  lastCheckedAt: string|null,
  lastStatus: "ok" | "error" | "never",
  lastError: string|null
}

// Store — a physical branch (only for type:"store" sources, e.g. OBI)
Store {
  id: string,            // uuid
  sourceKey: string,     // "obi"
  name: string,          // "OBI Markt Berlin-Spandau"
  street: string|null,
  plz: string,           // 5-digit German postal code
  city: string,
  lat: number,
  lon: number
}

// Listing — availability of one product at one source (and store, if physical)
Listing {
  sourceKey: string,
  productId: string,
  storeId: string|null,  // set for physical stores, null for online
  inStock: boolean,
  quantity: number|null, // units known to be in stock, if reported
  priceCents: number|null,
  currency: string,      // "EUR"
  url: string,           // deep link to the product/listing
  checkedAt: string
}
```

## Endpoints

### GET /api/health
Liveness + readiness. Used by the Docker healthcheck. Pings the DB.
- `200` → `{ "status": "ok", "db": "ok", "time": "<iso>" }`
- `503` → `{ "status": "degraded", "db": "error", "time": "<iso>" }`

### GET /api/products
List tracked products.
- `200` → `{ "products": Product[] }`

### GET /api/sources
List sources and their last scrape status.
- `200` → `{ "sources": Source[] }`

### GET /api/availability
Current availability, optionally centered on a PLZ with a radius.

Query params:
- `plz` (optional, 5 digits) — center point for store distance + ordering.
- `radius` (optional, integer km, default `50`, max `500`) — only applied when `plz` is given; filters physical stores to those within the radius.
- `productId` (optional) — restrict to one product; default all tracked products.

Response `200`:
```jsonc
{
  "center": { "plz": "10115", "lat": 52.53, "lon": 13.38 } | null,
  "radiusKm": 50,
  "stores": [                      // physical-store availability, nearest first
    {
      "store": Store,
      "distanceKm": number|null,   // null when no plz given
      "listings": Listing[]        // listings at this store (in-stock first)
    }
  ],
  "online": [                      // online retailers — "nationwide"
    {
      "source": Source,
      "listings": Listing[]
    }
  ],
  "generatedAt": "<iso>"
}
```
- `400` if `plz` is malformed or `radius` out of range.

### POST /api/subscriptions
Create a notification subscription. Sends a **double opt-in** confirmation email.

Body:
```jsonc
{ "email": "a@b.de", "plz": "10115", "radiusKm": 50, "productId": string|null }
```
- `radiusKm` optional, default `50`, range `1..500`.
- `productId` optional (null = any tracked product).

Responses:
- `202` → `{ "status": "pending_confirmation" }` (always 202 for valid input,
  even if email already subscribed, to avoid enumeration; a fresh confirm mail
  is sent).
- `400` → validation error (bad email / plz / radius).

### GET /api/subscriptions/confirm?token=...
Confirm (double opt-in). Token from the confirmation email.
- `200` → `{ "status": "confirmed", "email": "a@b.de" }`
- `400`/`404` → `{ "error": "invalid_token", ... }`

### GET /api/subscriptions/unsubscribe?token=...
Unsubscribe link (in every notification email).
- `200` → `{ "status": "unsubscribed" }`
- `400`/`404` → invalid token.

## Error shape

All non-2xx JSON responses use:
```jsonc
{ "error": "machine_code", "message": "human readable", "field": string|null }
```
Common codes: `validation_error`, `invalid_token`, `not_found`, `rate_limited`,
`internal`.

## Notes

**Frontend**
- Poll `/api/availability` on load and when the PLZ/radius changes; debounce PLZ input.
- Treat `priceCents: null` as "Preis unbekannt".
- `online` entries are shown in a side panel as "deutschlandweit verfügbar".

**Sources**
- `obi` is a real adapter but currently reports **online webshop availability**
  (obi.de ships nationwide), so its listings have `storeId: null` and appear under
  `online`, not as map pins. Per-market OBI store stock is a future enhancement.

**Backend**
- Server is authoritative: validate `plz`/`radius`/`email` server-side regardless of client.
- `cmd/poller` writes `availability_snapshots`; `/api/availability` reads the latest snapshot per (source, store, product).
- Notifications are sent by the poller (not the API) when a previously-unavailable listing becomes in-stock and a confirmed subscription's radius covers it; de-duplicated via the `notifications` table.
