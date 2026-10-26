import { describe, it, expect, afterEach, vi } from "vitest";
import { searchStores, StoreSearchResponseSchema } from "@/lib/api-client";
import {
  buildStoreAvailability,
  buildStoreSearchResponse,
  buildStore,
  jsonResponse,
  stubFetch,
} from "./__tests__/doubles";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("test doubles", () => {
  it("builders produce payloads the API schema accepts", () => {
    const res = buildStoreSearchResponse([
      buildStoreAvailability(),
      buildStoreAvailability({ store: buildStore({ id: "s2" }), distanceKm: null }),
    ]);
    expect(StoreSearchResponseSchema.parse(res).total).toBe(2);
  });

  it("stubFetch serves searchStores and records the request URL", async () => {
    const fake = stubFetch(jsonResponse(buildStoreSearchResponse()));
    const res = await searchStores({ q: "Göppingen", plz: "73035", page: 1 });
    expect(res.stores[0].store.plz).toBe("73035");
    expect(fake.url().pathname).toBe("/api/stores");
    expect(fake.url().searchParams.get("q")).toBe("Göppingen");
  });
});
