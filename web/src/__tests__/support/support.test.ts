import { afterEach, describe, expect, it } from "vitest";
import { ApiError, AvailabilityResponseSchema, getAvailability } from "@/lib/api-client";
import { buildAvailabilityResponse, buildListing, buildStoreAvailability } from "./builders";
import { apiErrorResponse, installFakeFetch, jsonResponse, type FakeFetch } from "./fakeFetch";

describe("test support", () => {
  let api: FakeFetch | undefined;
  afterEach(() => api?.restore());

  it("builders produce schema-valid payloads", () => {
    const payload = buildAvailabilityResponse({
      stores: [buildStoreAvailability({ listings: [buildListing({ priceCents: null, inStock: false })] })],
    });
    expect(AvailabilityResponseSchema.safeParse(payload).success).toBe(true);
  });

  it("fake fetch serves routes, records calls and returns errors", async () => {
    api = installFakeFetch()
      .on("GET", "/api/availability", () => jsonResponse(buildAvailabilityResponse()))
      .on("GET", "/api/availability", () => apiErrorResponse(400, "invalid_plz", "bad", "plz"));

    const res = await getAvailability({ plz: "73035", radius: 50 });
    expect(res.center?.plz).toBe("73035");
    expect(api.callsTo("/api/availability")[0].url.searchParams.get("plz")).toBe("73035");

    await expect(getAvailability({ plz: "00000" })).rejects.toBeInstanceOf(ApiError);
    expect(api.calls).toHaveLength(2);
  });
});
