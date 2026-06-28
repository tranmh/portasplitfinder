import type {
  AvailabilityResponse,
  OnlineAvailability,
  StoreAvailability,
} from "@/lib/api-client";

export const onlineFixture: OnlineAvailability[] = [
  {
    source: {
      key: "idealo",
      name: "Idealo",
      type: "online",
      baseUrl: "https://www.idealo.de",
      lastCheckedAt: "2026-06-28T10:00:00Z",
      lastStatus: "ok",
      lastError: null,
    },
    listings: [
      {
        sourceKey: "idealo",
        productId: "p1",
        storeId: null,
        inStock: true,
        quantity: null,
        priceCents: 49900,
        currency: "EUR",
        url: "https://www.idealo.de/p1",
        checkedAt: "2026-06-28T10:00:00Z",
      },
    ],
  },
  {
    source: {
      key: "amazon",
      name: "Amazon",
      type: "online",
      baseUrl: "https://www.amazon.de",
      lastCheckedAt: null,
      lastStatus: "never",
      lastError: null,
    },
    listings: [
      {
        sourceKey: "amazon",
        productId: "p1",
        storeId: null,
        inStock: false,
        quantity: null,
        priceCents: null, // -> "Preis unbekannt"
        currency: "EUR",
        url: "https://www.amazon.de/p1",
        checkedAt: "2026-06-28T09:00:00Z",
      },
    ],
  },
];

export const storesFixture: StoreAvailability[] = [
  {
    store: {
      id: "s2",
      sourceKey: "obi",
      name: "OBI Markt München",
      street: "Hauptstr. 5",
      plz: "80331",
      city: "München",
      lat: 48.137,
      lon: 11.575,
    },
    distanceKm: 12.3,
    listings: [
      {
        sourceKey: "obi",
        productId: "p1",
        storeId: "s2",
        inStock: true,
        quantity: 3,
        priceCents: 47900,
        currency: "EUR",
        url: "https://www.obi.de/s2/p1",
        checkedAt: "2026-06-28T10:00:00Z",
      },
    ],
  },
  {
    store: {
      id: "s1",
      sourceKey: "obi",
      name: "OBI Markt Berlin-Spandau",
      street: null,
      plz: "13585",
      city: "Berlin",
      lat: 52.535,
      lon: 13.2,
    },
    distanceKm: 3.1,
    listings: [
      {
        sourceKey: "obi",
        productId: "p1",
        storeId: "s1",
        inStock: false,
        quantity: null,
        priceCents: null, // -> "Preis unbekannt"
        currency: "EUR",
        url: "https://www.obi.de/s1/p1",
        checkedAt: "2026-06-28T10:00:00Z",
      },
    ],
  },
];

export const availabilityFixture: AvailabilityResponse = {
  center: { plz: "10115", lat: 52.53, lon: 13.38 },
  radiusKm: 50,
  stores: storesFixture,
  online: onlineFixture,
  generatedAt: "2026-06-28T10:00:00Z",
};
