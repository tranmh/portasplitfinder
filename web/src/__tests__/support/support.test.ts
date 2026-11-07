import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import {
  AvailabilityResponseSchema,
  createSubscription,
  getAvailability,
  getProducts,
} from "@/lib/api-client";
import { useAvailability } from "@/hooks/useAvailability";
import { aMixedStockScenario, aStoreAvailability, anAvailabilityResponse } from "./builders";
import { FakeApi } from "./fake-api";
import { useFixedClock } from "./clock";

describe("test support: builders", () => {
  it("produce schema-valid payloads", () => {
    expect(AvailabilityResponseSchema.safeParse(aMixedStockScenario()).success).toBe(true);
    expect(
      AvailabilityResponseSchema.safeParse(
        anAvailabilityResponse({ center: null, stores: [aStoreAvailability({ distanceKm: null })] }),
      ).success,
    ).toBe(true);
  });

  it("return fresh objects each call", () => {
    const a = aMixedStockScenario();
    a.stores[0].store.name = "changed";
    expect(aMixedStockScenario().stores[0].store.name).toBe("BAUHAUS Göppingen");
  });
});

describe("test support: FakeApi", () => {
  let api: FakeApi;

  beforeEach(() => {
    api = new FakeApi();
    api.install();
  });
  afterEach(() => api.uninstall());

  it("filters stores by radius when a PLZ is given", async () => {
    const res = await getAvailability({ plz: "73035", radius: 50 });
    expect(res.center?.plz).toBe("73035");
    expect(res.stores.map((s) => s.store.id)).toEqual(["bh-gp", "bh-es", "obi-s"]);
    expect(api.requestsTo("/api/availability")[0].url.searchParams.get("radius")).toBe("50");
  });

  it("returns a null center and distances without a PLZ", async () => {
    const res = await getAvailability();
    expect(res.center).toBeNull();
    expect(res.stores).toHaveLength(4);
    expect(res.stores.every((s) => s.distanceKm === null)).toBe(true);
  });

  it("answers validation errors in the contract's shape", async () => {
    await expect(getAvailability({ plz: "123" })).rejects.toMatchObject({
      status: 400,
      code: "validation_error",
      field: "plz",
    });
  });

  it("serves failNext once and then falls back", async () => {
    api.failNext("GET", "/api/products", 503, "internal");
    await expect(getProducts()).rejects.toMatchObject({ status: 503 });
    await expect(getProducts()).resolves.toEqual({ products: api.products });
  });

  it("runs the double opt-in flow", async () => {
    await createSubscription({ email: "a@b.de", plz: "73035" });
    expect(api.subscriptions).toMatchObject([{ email: "a@b.de", radiusKm: 50, status: "pending_confirmation" }]);
    const token = api.subscriptions[0].token;

    const confirm = await fetch(`http://api/api/subscriptions/confirm?token=${token}`);
    expect(await confirm.json()).toEqual({ status: "confirmed", email: "a@b.de" });
    const unsub = await fetch(`http://api/api/subscriptions/unsubscribe?token=${token}`);
    expect(await unsub.json()).toEqual({ status: "unsubscribed" });
    expect((await fetch("http://api/api/subscriptions/confirm?token=nope")).status).toBe(404);
  });

  it("rejects with AbortError when the signal aborts a held request", async () => {
    const held = api.hold("GET", "/api/availability");
    const controller = new AbortController();
    const pending = getAvailability({}, controller.signal);
    await held.arrived;
    controller.abort();
    await expect(pending).rejects.toMatchObject({ name: "AbortError" });
    held.release();
  });

  it("drives useAvailability through loading and data", async () => {
    const held = api.hold("GET", "/api/availability");
    const { result } = renderHook(() => useAvailability("73035", 30));
    expect(result.current.loading).toBe(true);

    await held.arrived;
    held.release();
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeNull();
    expect(result.current.data?.radiusKm).toBe(30);
    expect(result.current.data?.stores.map((s) => s.store.id)).toEqual(["bh-gp", "bh-es"]);
  });
});

describe("test support: useFixedClock", () => {
  let restore: () => void;
  beforeEach(() => {
    restore = useFixedClock("2026-06-28T10:00:00Z");
  });
  afterEach(() => restore());

  it("freezes Date and lets the test advance timers", () => {
    expect(new Date().toISOString()).toBe("2026-06-28T10:00:00.000Z");
    let fired = false;
    setTimeout(() => (fired = true), 300);
    vi.advanceTimersByTime(299);
    expect(fired).toBe(false);
    vi.advanceTimersByTime(1);
    expect(fired).toBe(true);
    expect(Date.now()).toBe(Date.parse("2026-06-28T10:00:00.300Z"));
  });
});
