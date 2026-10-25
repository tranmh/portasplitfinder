import type {
  AvailabilityResponse,
  Listing,
  OnlineAvailability,
  Product,
  Source,
  Store,
  StoreAvailability,
} from "@/lib/api-client";

/**
 * Builders for the API domain types. Every builder returns a valid,
 * schema-conforming object; pass overrides for the fields a test cares about.
 * Timestamps are fixed so tests stay deterministic.
 */

export const FIXED_NOW = "2026-06-28T10:00:00Z";

export function buildProduct(overrides: Partial<Product> = {}): Product {
  return { id: "p1", name: "Akku-Bohrschrauber", model: "X-100", imageUrl: null, ...overrides };
}

export function buildSource(overrides: Partial<Source> = {}): Source {
  return {
    key: "idealo",
    name: "Idealo",
    type: "online",
    baseUrl: "https://www.idealo.de",
    lastCheckedAt: FIXED_NOW,
    lastStatus: "ok",
    lastError: null,
    ...overrides,
  };
}

export function buildStore(overrides: Partial<Store> = {}): Store {
  return {
    id: "s1",
    sourceKey: "obi",
    name: "OBI Markt Esslingen",
    street: "Hauptstr. 1",
    plz: "73035",
    city: "Göppingen",
    lat: 48.703,
    lon: 9.652,
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
    priceCents: 4990,
    currency: "EUR",
    url: "https://www.obi.de/s1/p1",
    checkedAt: FIXED_NOW,
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
    listings: [buildListing({ sourceKey: store.sourceKey, storeId: store.id })],
    ...overrides,
  };
}

export function buildOnlineAvailability(
  overrides: Partial<OnlineAvailability> = {},
): OnlineAvailability {
  const source = overrides.source ?? buildSource();
  return {
    source,
    listings: [
      buildListing({
        sourceKey: source.key,
        storeId: null,
        quantity: null,
        url: `${source.baseUrl}/p1`,
      }),
    ],
    ...overrides,
  };
}

export function buildAvailabilityResponse(
  overrides: Partial<AvailabilityResponse> = {},
): AvailabilityResponse {
  return {
    center: { plz: "73035", lat: 48.703, lon: 9.652 },
    radiusKm: 50,
    stores: [buildStoreAvailability()],
    online: [buildOnlineAvailability()],
    generatedAt: FIXED_NOW,
    ...overrides,
  };
}
