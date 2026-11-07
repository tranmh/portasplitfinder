import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useAvailability } from "./useAvailability";
import { ApiError } from "@/lib/api-client";
import { FakeApi } from "@/__tests__/support/fake-api";

describe("useAvailability", () => {
  let api: FakeApi;

  beforeEach(() => {
    api = new FakeApi();
    api.install();
  });
  afterEach(() => {
    api.uninstall();
  });

  type Props = { plz: string; radius: number; productId?: string };

  function setup(initialProps: Props) {
    return renderHook(({ plz, radius, productId }: Props) => useAvailability(plz, radius, productId), {
      initialProps,
    });
  }

  const availabilityRequests = () => api.requestsTo("/api/availability");
  const lastQuery = () => availabilityRequests().at(-1)!.url.searchParams;

  it("starts loading, then exposes the data for a valid PLZ and radius", async () => {
    const { result } = setup({ plz: "73035", radius: 50 });

    expect(result.current.loading).toBe(true);
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeNull();
    expect(result.current.data?.center?.plz).toBe("73035");
    expect(result.current.data?.radiusKm).toBe(50);
    // BAUHAUS Ulm at 72.5 km is outside 50 km.
    expect(result.current.data?.stores.map((s) => s.store.id)).toEqual(["bh-gp", "bh-es", "obi-s"]);

    expect(availabilityRequests()).toHaveLength(1);
    expect(lastQuery().get("plz")).toBe("73035");
    expect(lastQuery().get("radius")).toBe("50");
    expect(lastQuery().has("productId")).toBe(false);
  });

  it("sends a trimmed PLZ", async () => {
    const { result } = setup({ plz: " 73035 ", radius: 25 });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(lastQuery().get("plz")).toBe("73035");
    expect(lastQuery().get("radius")).toBe("25");
  });

  it.each(["", "7303", "730351", "abcde"])(
    "omits plz and radius for the invalid PLZ %j and loads the unfiltered list",
    async (plz) => {
      const { result } = setup({ plz, radius: 50 });
      await waitFor(() => expect(result.current.loading).toBe(false));

      expect(lastQuery().has("plz")).toBe(false);
      expect(lastQuery().has("radius")).toBe(false);
      expect(result.current.error).toBeNull();
      expect(result.current.data?.center).toBeNull();
      expect(result.current.data?.stores).toHaveLength(4);
    },
  );

  it("passes the productId when given", async () => {
    const { result } = setup({ plz: "73035", radius: 50, productId: "p1" });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(lastQuery().get("productId")).toBe("p1");
  });

  it("exposes an ApiError and stops loading when the backend fails", async () => {
    api.failNext("GET", "/api/availability", 500, "internal");
    const { result } = setup({ plz: "73035", radius: 50 });

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeInstanceOf(ApiError);
    expect(result.current.error).toMatchObject({ status: 500, code: "internal" });
  });

  it("surfaces a network failure as an Error", async () => {
    const held = api.hold("GET", "/api/availability");
    const { result } = setup({ plz: "73035", radius: 50 });
    await held.arrived;

    act(() => held.fail(new TypeError("Failed to fetch")));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeInstanceOf(TypeError);
    expect(result.current.error?.message).toBe("Failed to fetch");
  });

  it("wraps a non-Error rejection into an Error", async () => {
    api.respondOnce("GET", "/api/availability", () => {
      throw "boom";
    });
    const { result } = setup({ plz: "73035", radius: 50 });

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe("boom");
  });

  it("surfaces a response that does not match the contract as an error", async () => {
    api.respondOnce(
      "GET",
      "/api/availability",
      new Response(JSON.stringify({ center: null }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const { result } = setup({ plz: "73035", radius: 50 });

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeInstanceOf(Error);
  });

  it("refetch() requests again and clears a previous error", async () => {
    api.failNext("GET", "/api/availability", 503, "unavailable");
    const { result } = setup({ plz: "73035", radius: 50 });
    await waitFor(() => expect(result.current.error).not.toBeNull());

    act(() => result.current.refetch());
    expect(result.current.loading).toBe(true);
    expect(result.current.error).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeNull();
    expect(result.current.data?.center?.plz).toBe("73035");
    expect(availabilityRequests()).toHaveLength(2);
  });

  it("keeps refetch stable across renders", async () => {
    const { result, rerender } = setup({ plz: "73035", radius: 50 });
    const first = result.current.refetch;
    await waitFor(() => expect(result.current.loading).toBe(false));
    rerender({ plz: "10115", radius: 50 });
    expect(result.current.refetch).toBe(first);
  });

  it("re-fetches when the PLZ changes", async () => {
    const { result, rerender } = setup({ plz: "73035", radius: 50 });
    await waitFor(() => expect(result.current.loading).toBe(false));

    rerender({ plz: "10115", radius: 50 });
    expect(result.current.loading).toBe(true);

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(availabilityRequests()).toHaveLength(2);
    expect(lastQuery().get("plz")).toBe("10115");
    expect(result.current.data?.center?.plz).toBe("10115");
  });

  it("re-fetches when the radius changes", async () => {
    const { result, rerender } = setup({ plz: "73035", radius: 50 });
    await waitFor(() => expect(result.current.loading).toBe(false));

    rerender({ plz: "73035", radius: 100 });
    await waitFor(() => expect(result.current.data?.radiusKm).toBe(100));
    expect(lastQuery().get("radius")).toBe("100");
    // At 100 km BAUHAUS Ulm (72.5 km) is included.
    expect(result.current.data?.stores).toHaveLength(4);
  });

  it("re-fetches when the productId changes", async () => {
    const { result, rerender } = setup({ plz: "73035", radius: 50 });
    await waitFor(() => expect(result.current.loading).toBe(false));

    rerender({ plz: "73035", radius: 50, productId: "p2" });
    await waitFor(() => expect(availabilityRequests()).toHaveLength(2));
    expect(lastQuery().get("productId")).toBe("p2");
  });

  it("does not re-fetch on a re-render with the same parameters", async () => {
    const { result, rerender } = setup({ plz: "73035", radius: 50 });
    await waitFor(() => expect(result.current.loading).toBe(false));

    rerender({ plz: "73035", radius: 50 });
    expect(result.current.loading).toBe(false);
    expect(availabilityRequests()).toHaveLength(1);
  });

  it("aborts the stale request and ignores its late answer", async () => {
    const stale = api.hold("GET", "/api/availability");
    const { result, rerender } = setup({ plz: "73035", radius: 50 });
    await stale.arrived;

    rerender({ plz: "10115", radius: 50 });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data?.center?.plz).toBe("10115");

    act(() => stale.release());
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.data?.center?.plz).toBe("10115");
    expect(result.current.error).toBeNull();
  });

  it("aborts the in-flight request on unmount", async () => {
    let signal: AbortSignal | undefined;
    const original = globalThis.fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      signal = init?.signal ?? undefined;
      return original(input, init);
    }) as typeof fetch;
    const held = api.hold("GET", "/api/availability");

    const { unmount } = setup({ plz: "73035", radius: 50 });
    await held.arrived;
    expect(signal?.aborted).toBe(false);

    unmount();
    expect(signal?.aborted).toBe(true);
    held.release();
  });
});
