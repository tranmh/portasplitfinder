import type {
  AvailabilityResponse,
  Center,
  Listing,
  OnlineAvailability,
  Product,
  Source,
  Store,
  StoreAvailability,
} from "@/lib/api-client";

/**
 * Builders for the API contract types (docs/api-contract.md). Every builder
 * returns a fresh, schema-valid object; pass overrides for what the test cares
 * about. Timestamps are fixed so output never depends on the wall clock.
 */

export const FIXED_ISO = "2026-06-28T10:00:00Z";

/** The default search of the web UI: PLZ 73035 (Göppingen), 50 km. */
export const DEFAULT_CENTER: Center = { plz: "73035", lat: 48.703, lon: 9.652 };
export const DEFAULT_RADIUS_KM = 50;

export function aProduct(overrides: Partial<Product> = {}): Product {
  return {
    id: "p1",
    name: "Portasplit Klimagerät",
    model: "PS-12000",
    imageUrl: null,
    ...overrides,
  };
}

export function aSource(overrides: Partial<Source> = {}): Source {
  return {
    key: "obi",
    name: "OBI",
    type: "store",
    baseUrl: "https://www.obi.de",
    lastCheckedAt: FIXED_ISO,
    lastStatus: "ok",
    lastError: null,
    ...overrides,
  };
}

export function anOnlineSource(overrides: Partial<Source> = {}): Source {
  return aSource({
    key: "idealo",
    name: "Idealo",
    type: "online",
    baseUrl: "https://www.idealo.de",
    ...overrides,
  });
}

export function aStore(overrides: Partial<Store> = {}): Store {
  return {
    id: "s1",
    sourceKey: "bauhaus",
    name: "BAUHAUS Göppingen",
    street: "Stuttgarter Str. 1",
    plz: "73035",
    city: "Göppingen",
    lat: 48.703,
    lon: 9.652,
    ...overrides,
  };
}

export function aListing(overrides: Partial<Listing> = {}): Listing {
  return {
    sourceKey: "bauhaus",
    productId: "p1",
    storeId: "s1",
    inStock: true,
    quantity: 2,
    priceCents: 49900,
    currency: "EUR",
    url: "https://www.bauhaus.info/p1",
    checkedAt: FIXED_ISO,
    ...overrides,
  };
}

/** A listing that is out of stock with an unknown price ("Preis unbekannt"). */
export function anOutOfStockListing(overrides: Partial<Listing> = {}): Listing {
  return aListing({ inStock: false, quantity: null, priceCents: null, ...overrides });
}

/**
 * A store with its listings. Listings default to one in-stock listing whose
 * sourceKey/storeId match the store.
 */
export function aStoreAvailability(
  opts: {
    store?: Partial<Store>;
    distanceKm?: number | null;
    listings?: Partial<Listing>[];
  } = {},
): StoreAvailability {
  const store = aStore(opts.store);
  const listings = (opts.listings ?? [{}]).map((l) =>
    aListing({ sourceKey: store.sourceKey, storeId: store.id, ...l }),
  );
  return {
    store,
    distanceKm: opts.distanceKm === undefined ? 5 : opts.distanceKm,
    listings,
  };
}

/** An online retailer with its listings (storeId is always null online). */
export function anOnlineAvailability(
  opts: { source?: Partial<Source>; listings?: Partial<Listing>[] } = {},
): OnlineAvailability {
  const source = anOnlineSource(opts.source);
  const listings = (opts.listings ?? [{}]).map((l) =>
    aListing({
      sourceKey: source.key,
      storeId: null,
      quantity: null,
      url: `${source.baseUrl}/p1`,
      ...l,
    }),
  );
  return { source, listings };
}

export function anAvailabilityResponse(
  overrides: Partial<AvailabilityResponse> = {},
): AvailabilityResponse {
  return {
    center: DEFAULT_CENTER,
    radiusKm: DEFAULT_RADIUS_KM,
    stores: [],
    online: [],
    generatedAt: FIXED_ISO,
    ...overrides,
  };
}

/**
 * A realistic scenario around the default PLZ 73035: two stores in stock, one
 * out of stock, one beyond 50 km, plus one online retailer in stock and one
 * without a known price. Ordered nearest first like the backend does.
 */
export function aMixedStockScenario(): AvailabilityResponse {
  return anAvailabilityResponse({
    stores: [
      aStoreAvailability({
        store: { id: "bh-gp", name: "BAUHAUS Göppingen" },
        distanceKm: 1.2,
        listings: [{ quantity: 4, priceCents: 47900 }],
      }),
      aStoreAvailability({
        store: {
          id: "bh-es",
          name: "BAUHAUS Esslingen",
          street: "Weilstr. 10",
          plz: "73734",
          city: "Esslingen",
          lat: 48.74,
          lon: 9.31,
        },
        distanceKm: 25.4,
        listings: [{ inStock: false, quantity: 0, priceCents: 49900 }],
      }),
      aStoreAvailability({
        store: {
          id: "obi-s",
          sourceKey: "obi",
          name: "OBI Stuttgart-Vaihingen",
          street: null,
          plz: "70563",
          city: "Stuttgart",
          lat: 48.73,
          lon: 9.11,
        },
        distanceKm: 39.9,
        listings: [{ quantity: null, priceCents: null }],
      }),
      aStoreAvailability({
        store: {
          id: "bh-ul",
          name: "BAUHAUS Ulm",
          street: "Blaubeurer Str. 80",
          plz: "89077",
          city: "Ulm",
          lat: 48.4,
          lon: 9.96,
        },
        distanceKm: 72.5,
        listings: [{ quantity: 1, priceCents: 45900 }],
      }),
    ],
    online: [
      anOnlineAvailability({ listings: [{ priceCents: 46900 }] }),
      anOnlineAvailability({
        source: {
          key: "amazon",
          name: "Amazon",
          baseUrl: "https://www.amazon.de",
          lastCheckedAt: null,
          lastStatus: "never",
        },
        listings: [{ inStock: false, priceCents: null }],
      }),
    ],
  });
}
