# Plan: Frontend — Map, PLZ Filter, Subscriptions

**Context:** Next.js (App Router) + TS + React UI showing Portasplit availability
on a map of Germany, with PLZ filter and email subscription. Consumes the API in
`../api-contract.md`. See master plan for the hybrid-map decision.

## Stack
- Next.js 15 App Router, TypeScript strict, React 19.
- **react-leaflet + leaflet** with OpenStreetMap tiles (free, no API key). Map
  component must be dynamically imported (`ssr:false`) since Leaflet needs `window`.
- Tailwind CSS for layout. Zod for API response validation.
- i18n: simple message dictionaries `de` (default) + `en` in `src/i18n/`.

## Structure (`web/src`)
- `lib/api-client.ts` — typed fetchers (`getAvailability`, `getProducts`,
  `getSources`, `createSubscription`) with Zod schemas mirroring the contract.
  Base URL from `NEXT_PUBLIC_API_BASE_URL`.
- `app/page.tsx` — main page: map + side panel + filter + subscribe.
- `app/api/health/route.ts` — returns 200 (for the web container healthcheck).
- `components/MapView.tsx` — Germany map; OBI store markers with stock badge in
  popup; a radius circle + center marker when a PLZ is set.
- `components/PlzFilter.tsx` — PLZ input (5 digits) + radius slider (default 50,
  1..500). Debounced; updates availability query.
- `components/OnlinePanel.tsx` — side panel listing online sources
  ("deutschlandweit verfügbar"), price, in-stock badge, deep link, last-checked.
- `components/StoreList.tsx` — nearby stores ranked by distance.
- `components/SubscribeForm.tsx` — email + PLZ + radius → POST /subscriptions;
  shows "Bestätigungs-E-Mail gesendet" on 202; client + server validation.
- `hooks/useAvailability.ts` — fetch + refetch on plz/radius change.

## Behaviour
- On load: fetch availability (no center) → show all stores + online panel.
- Enter PLZ → recenter map, draw radius circle, refetch with plz+radius, stores
  filtered + sorted by distance, distance shown per store.
- Price null → "Preis unbekannt". Empty state when nothing in stock.

## Tests (Vitest + React Testing Library)
- `api-client` Zod parsing (valid + invalid payloads).
- `PlzFilter` validation + debounce callback.
- `SubscribeForm` validation + success/error states (mocked fetch).
- `OnlinePanel`/`StoreList` rendering from fixture data.
- MapView is integration-tested via Playwright (Leaflet needs a real DOM).

## Done when
`npm run lint` + `npm run test` green; `npm run build` succeeds; page renders the
map + panels against a running API.
