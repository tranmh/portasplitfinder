import { bench, describe } from "vitest";
import { searchStores, type StoreSearchResponse } from "./api-client";

// fetch is stubbed with a pre-built response object so the real searchStores
// (URL building + zod parsing) runs without network.

function makeResponse(n: number, listingsPerStore = 2): StoreSearchResponse {
  const stores = [];
  for (let i = 0; i < n; i++) {
    const id = `store-${i}`;
    const listings = [];
    for (let j = 0; j < listingsPerStore; j++) {
      listings.push({
        sourceKey: "obi",
        productId: `product-${j}`,
        storeId: id,
        inStock: (i + j) % 2 === 0,
        quantity: j % 3 === 0 ? null : j,
        priceCents: j % 4 === 0 ? null : 47900 + j,
        currency: "EUR",
        url: `https://example.test/p/${j}?store=${i}`,
        checkedAt: "2026-10-26T12:00:00Z",
      });
    }
    stores.push({
      store: {
        id,
        sourceKey: "obi",
        name: `OBI Markt ${i}`,
        street: i % 5 === 0 ? null : `Hauptstraße ${i}`,
        plz: String(10000 + (i % 89999)).padStart(5, "0"),
        city: `Stadt ${i}`,
        lat: 48 + (i % 100) / 100,
        lon: 9 + (i % 100) / 100,
      },
      distanceKm: i % 7 === 0 ? null : i * 0.5,
      listings,
    });
  }
  return { stores, total: n, page: 1, pageSize: n };
}

function stubFetch(body: unknown, ok = true, status = 200): void {
  const res = {
    ok,
    status,
    statusText: ok ? "OK" : "Internal Server Error",
    json: () => Promise.resolve(body),
  };
  globalThis.fetch = (() => Promise.resolve(res)) as unknown as typeof fetch;
}

let sink: unknown;

describe("searchStores: response parsing by number of stores", () => {
  const small = makeResponse(10);
  const medium = makeResponse(100);
  const large = makeResponse(1000);

  bench("10 stores", async () => {
    stubFetch(small);
    sink = await searchStores({ q: "obi", plz: "73035", page: 1 });
  });
  bench("100 stores", async () => {
    stubFetch(medium);
    sink = await searchStores({ q: "obi", plz: "73035", page: 1 });
  });
  bench("1000 stores", async () => {
    stubFetch(large);
    sink = await searchStores({ q: "obi", plz: "73035", page: 1 });
  });
});

describe("searchStores: query string building by length of q", () => {
  const empty = makeResponse(0);
  const special = "Bauhaus & Söhne / Göppingen?#=+%";
  const qSmall = special;
  const qMedium = special.repeat(10);
  const qLarge = special.repeat(100);
  let lastUrl = "";

  function stubCapture(): void {
    const res = { ok: true, status: 200, statusText: "OK", json: () => Promise.resolve(empty) };
    globalThis.fetch = ((url: string) => {
      lastUrl = url;
      return Promise.resolve(res);
    }) as unknown as typeof fetch;
  }

  bench("no params", async () => {
    stubCapture();
    sink = await searchStores();
    sink = lastUrl;
  });
  bench("q 32 chars + plz + page", async () => {
    stubCapture();
    sink = await searchStores({ q: qSmall, plz: "73035", page: 2 });
    sink = lastUrl;
  });
  bench("q 320 chars + plz + page", async () => {
    stubCapture();
    sink = await searchStores({ q: qMedium, plz: "73035", page: 2 });
    sink = lastUrl;
  });
  bench("q 3200 chars + plz + page", async () => {
    stubCapture();
    sink = await searchStores({ q: qLarge, plz: "73035", page: 2 });
    sink = lastUrl;
  });
});

describe("searchStores: non-OK response throws", () => {
  const errorBody = { error: "internal", message: "boom", field: null };

  bench("500 with JSON error body", async () => {
    stubFetch(errorBody, false, 500);
    try {
      await searchStores({ q: "obi" });
      throw new Error("expected searchStores to throw");
    } catch (e) {
      sink = e;
    }
  });
});

export { sink };
