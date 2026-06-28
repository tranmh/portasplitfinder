import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  AvailabilityResponseSchema,
  SubscriptionResponseSchema,
  ApiError,
  getAvailability,
  createSubscription,
} from "./api-client";
import { availabilityFixture } from "@/test/fixtures";

describe("Zod schemas", () => {
  it("parses a valid availability payload", () => {
    const parsed = AvailabilityResponseSchema.parse(availabilityFixture);
    expect(parsed.center?.plz).toBe("10115");
    expect(parsed.stores).toHaveLength(2);
    expect(parsed.online).toHaveLength(2);
    expect(parsed.online[1].listings[0].priceCents).toBeNull();
  });

  it("rejects an invalid availability payload (wrong types)", () => {
    const bad = {
      ...availabilityFixture,
      radiusKm: "fifty", // should be number
    };
    const result = AvailabilityResponseSchema.safeParse(bad);
    expect(result.success).toBe(false);
  });

  it("rejects an availability payload missing required fields", () => {
    const result = AvailabilityResponseSchema.safeParse({ center: null });
    expect(result.success).toBe(false);
  });

  it("accepts null price and null distance", () => {
    const result = AvailabilityResponseSchema.safeParse({
      center: null,
      radiusKm: 50,
      stores: [
        {
          store: {
            id: "x",
            sourceKey: "obi",
            name: "n",
            street: null,
            plz: "10115",
            city: "Berlin",
            lat: 1,
            lon: 2,
          },
          distanceKm: null,
          listings: [],
        },
      ],
      online: [],
      generatedAt: "2026-06-28T10:00:00Z",
    });
    expect(result.success).toBe(true);
  });

  it("validates the subscription response shape", () => {
    expect(
      SubscriptionResponseSchema.safeParse({ status: "pending_confirmation" })
        .success,
    ).toBe(true);
    expect(
      SubscriptionResponseSchema.safeParse({ status: "nope" }).success,
    ).toBe(false);
  });
});

describe("fetchers", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.restoreAllMocks();
  });
  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("getAvailability builds query params and parses the response", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(availabilityFixture), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    global.fetch = fetchMock as unknown as typeof fetch;

    const res = await getAvailability({ plz: "10115", radius: 25 });
    expect(res.stores).toHaveLength(2);

    const calledUrl = String(fetchMock.mock.calls[0][0]);
    expect(calledUrl).toContain("/api/availability");
    expect(calledUrl).toContain("plz=10115");
    expect(calledUrl).toContain("radius=25");
  });

  it("throws ApiError with field on a 400 validation error", async () => {
    global.fetch = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          error: "validation_error",
          message: "bad plz",
          field: "plz",
        }),
        { status: 400, headers: { "Content-Type": "application/json" } },
      ),
    ) as unknown as typeof fetch;

    await expect(getAvailability({ plz: "1" })).rejects.toMatchObject({
      name: "ApiError",
      status: 400,
      code: "validation_error",
      field: "plz",
    });
  });

  it("createSubscription posts a JSON body and parses 202", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ status: "pending_confirmation" }), {
        status: 202,
        headers: { "Content-Type": "application/json" },
      }),
    );
    global.fetch = fetchMock as unknown as typeof fetch;

    const res = await createSubscription({ email: "a@b.de", plz: "10115" });
    expect(res.status).toBe("pending_confirmation");

    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(init.method).toBe("POST");
    const body = JSON.parse(String(init.body));
    expect(body).toMatchObject({
      email: "a@b.de",
      plz: "10115",
      radiusKm: 50,
      productId: null,
    });
  });

  it("ApiError carries code and message", () => {
    const err = new ApiError(404, { error: "not_found", message: "nope" });
    expect(err.status).toBe(404);
    expect(err.code).toBe("not_found");
    expect(err.message).toBe("nope");
  });
});
