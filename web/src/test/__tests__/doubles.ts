import { vi } from "vitest";
import type {
  Listing,
  Store,
  StoreAvailability,
  StoreSearchResponse,
} from "@/lib/api-client";

/** Shared builders and a fake fetch for StoreSearch / searchStores tests. */

export function buildStore(overrides: Partial<Store> = {}): Store {
  return {
    id: "s1",
    sourceKey: "obi",
    name: "OBI Markt Esslingen",
    street: "Fabrikstr. 1",
    plz: "73035",
    city: "Göppingen",
    lat: 48.7,
    lon: 9.65,
    ...overrides,
  };
}

export function buildListing(overrides: Partial<Listing> = {}): Listing {
  return {
    sourceKey: "obi",
    productId: "p1",
    storeId: "s1",
    inStock: true,
    quantity: 2,
    priceCents: 47900,
    currency: "EUR",
    url: "https://www.obi.de/s1/p1",
    checkedAt: "2026-06-28T10:00:00Z",
    ...overrides,
  };
}

export function buildStoreAvailability(
  overrides: Partial<StoreAvailability> = {},
): StoreAvailability {
  const store = overrides.store ?? buildStore();
  return {
    store,
    distanceKm: 4.2,
    listings: [buildListing({ storeId: store.id, sourceKey: store.sourceKey })],
    ...overrides,
  };
}

export function buildStoreSearchResponse(
  stores: StoreAvailability[] = [buildStoreAvailability()],
  overrides: Partial<StoreSearchResponse> = {},
): StoreSearchResponse {
  return { stores, total: stores.length, page: 1, pageSize: 20, ...overrides };
}

export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/**
 * Installs a fetch stub answering with the given responses in order (the last
 * one repeats). Call `vi.unstubAllGlobals()` in afterEach.
 */
export function stubFetch(...responses: Array<Response | (() => Response) | Error>) {
  let i = 0;
  const fn = vi.fn(async (_input: RequestInfo | URL, _init?: RequestInit) => {
    const next = responses[Math.min(i++, responses.length - 1)];
    if (next instanceof Error) throw next;
    return typeof next === "function" ? next() : next.clone();
  });
  vi.stubGlobal("fetch", fn);
  return {
    fn,
    /** URL of the n-th call as a URL object. */
    url(n = 0): URL {
      return new URL(String(fn.mock.calls[n][0]));
    },
  };
}
