import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { ApiError } from "@/lib/api-client";
import { useAvailability } from "./useAvailability";
import { buildAvailabilityResponse } from "@/__tests__/support/builders";
import {
  apiErrorResponse,
  deferred,
  installFakeFetch,
  jsonResponse,
  type FakeFetch,
} from "@/__tests__/support/fakeFetch";

const PATH = "/api/availability";

describe("useAvailability", () => {
  let api: FakeFetch;

  beforeEach(() => {
    api = installFakeFetch();
  });
  afterEach(() => {
    api.restore();
  });

  it("starts loading, then exposes the parsed response", async () => {
    const payload = buildAvailabilityResponse();
    api.on("GET", PATH, () => jsonResponse(payload));

    const { result } = renderHook(() => useAvailability("73035", 50));

    expect(result.current.loading).toBe(true);
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).toEqual(payload);
    expect(result.current.error).toBeNull();
  });

  it("sends trimmed plz and radius for a valid PLZ", async () => {
    api.on("GET", PATH, () => jsonResponse(buildAvailabilityResponse()));

    const { result } = renderHook(() => useAvailability(" 73035 ", 25, "p1"));
    await waitFor(() => expect(result.current.loading).toBe(false));

    const calls = api.callsTo(PATH);
    expect(calls).toHaveLength(1);
    expect(calls[0].url.searchParams.get("plz")).toBe("73035");
    expect(calls[0].url.searchParams.get("radius")).toBe("25");
    expect(calls[0].url.searchParams.get("productId")).toBe("p1");
  });

  it.each(["", "7303", "abcde", "730355"])(
    "omits plz and radius for the invalid PLZ %j",
    async (plz) => {
      api.on("GET", PATH, () => jsonResponse(buildAvailabilityResponse({ center: null })));

      const { result } = renderHook(() => useAvailability(plz, 50));
      await waitFor(() => expect(result.current.loading).toBe(false));

      const url = api.callsTo(PATH)[0].url;
      expect(url.searchParams.has("plz")).toBe(false);
      expect(url.searchParams.has("radius")).toBe(false);
      expect(result.current.data?.center).toBeNull();
    },
  );

  it("exposes an ApiError and stops loading when the request fails", async () => {
    api.on("GET", PATH, () => apiErrorResponse(400, "invalid_plz", "PLZ unknown", "plz"));

    const { result } = renderHook(() => useAvailability("00000", 50));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBeInstanceOf(ApiError);
    expect(result.current.error).toMatchObject({ status: 400, code: "invalid_plz", field: "plz" });
    expect(result.current.data).toBeNull();
  });

  it("surfaces a network failure as an Error", async () => {
    api.on("GET", PATH, () => {
      throw new TypeError("Failed to fetch");
    });

    const { result } = renderHook(() => useAvailability("73035", 50));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe("Failed to fetch");
  });

  it("treats a schema-invalid payload as an error", async () => {
    api.on("GET", PATH, () => jsonResponse({ center: null, radiusKm: "fifty" }));

    const { result } = renderHook(() => useAvailability("73035", 50));
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.data).toBeNull();
  });

  it("refetches when the PLZ changes", async () => {
    api
      .on("GET", PATH, () => jsonResponse(buildAvailabilityResponse()))
      .on("GET", PATH, () =>
        jsonResponse(buildAvailabilityResponse({ center: { plz: "10115", lat: 52.53, lon: 13.38 } })),
      );

    const { result, rerender } = renderHook(({ plz, r }) => useAvailability(plz, r), {
      initialProps: { plz: "73035", r: 50 },
    });
    await waitFor(() => expect(result.current.data?.center?.plz).toBe("73035"));

    rerender({ plz: "10115", r: 50 });
    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.data?.center?.plz).toBe("10115"));

    const calls = api.callsTo(PATH);
    expect(calls).toHaveLength(2);
    expect(calls[1].url.searchParams.get("plz")).toBe("10115");
  });

  it("refetches when the radius changes", async () => {
    api
      .on("GET", PATH, () => jsonResponse(buildAvailabilityResponse({ radiusKm: 50 })))
      .on("GET", PATH, () => jsonResponse(buildAvailabilityResponse({ radiusKm: 100 })));

    const { result, rerender } = renderHook(({ plz, r }) => useAvailability(plz, r), {
      initialProps: { plz: "73035", r: 50 },
    });
    await waitFor(() => expect(result.current.data?.radiusKm).toBe(50));

    rerender({ plz: "73035", r: 100 });
    await waitFor(() => expect(result.current.data?.radiusKm).toBe(100));

    const calls = api.callsTo(PATH);
    expect(calls).toHaveLength(2);
    expect(calls[1].url.searchParams.get("radius")).toBe("100");
  });

  it("does not refetch on a rerender with unchanged params", async () => {
    api.on("GET", PATH, () => jsonResponse(buildAvailabilityResponse()));

    const { result, rerender } = renderHook(({ plz, r }) => useAvailability(plz, r), {
      initialProps: { plz: "73035", r: 50 },
    });
    await waitFor(() => expect(result.current.loading).toBe(false));

    rerender({ plz: "73035", r: 50 });
    expect(result.current.loading).toBe(false);
    expect(api.callsTo(PATH)).toHaveLength(1);
  });

  it("refetch() issues a new request and clears a previous error", async () => {
    api
      .on("GET", PATH, () => apiErrorResponse(503, "unavailable", "down"))
      .on("GET", PATH, () => jsonResponse(buildAvailabilityResponse()));

    const { result } = renderHook(() => useAvailability("73035", 50));
    await waitFor(() => expect(result.current.error).not.toBeNull());

    act(() => {
      result.current.refetch();
    });
    expect(result.current.loading).toBe(true);
    expect(result.current.error).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBeNull();
    expect(result.current.data?.center?.plz).toBe("73035");
    expect(api.callsTo(PATH)).toHaveLength(2);
  });

  it("keeps a stable refetch function across renders", async () => {
    api.on("GET", PATH, () => jsonResponse(buildAvailabilityResponse()));

    const { result, rerender } = renderHook(({ plz }) => useAvailability(plz, 50), {
      initialProps: { plz: "73035" },
    });
    const first = result.current.refetch;
    rerender({ plz: "10115" });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.refetch).toBe(first);
  });

  it("aborts the previous request and ignores its late response", async () => {
    const slow = deferred<Response>();
    api
      .on("GET", PATH, () => slow.promise)
      .on("GET", PATH, () =>
        jsonResponse(buildAvailabilityResponse({ center: { plz: "10115", lat: 52.53, lon: 13.38 } })),
      );

    const { result, rerender } = renderHook(({ plz }) => useAvailability(plz, 50), {
      initialProps: { plz: "73035" },
    });
    await waitFor(() => expect(api.callsTo(PATH)).toHaveLength(1));

    rerender({ plz: "10115" });
    await waitFor(() => expect(result.current.data?.center?.plz).toBe("10115"));

    expect(api.callsTo(PATH)[0].signal?.aborted).toBe(true);

    await act(async () => {
      slow.resolve(jsonResponse(buildAvailabilityResponse()));
      await slow.promise;
    });
    expect(result.current.data?.center?.plz).toBe("10115");
    expect(result.current.error).toBeNull();
    expect(result.current.loading).toBe(false);
  });

  it("aborts the in-flight request on unmount", async () => {
    const slow = deferred<Response>();
    api.on("GET", PATH, () => slow.promise);

    const { unmount } = renderHook(() => useAvailability("73035", 50));
    await waitFor(() => expect(api.callsTo(PATH)).toHaveLength(1));

    unmount();
    expect(api.callsTo(PATH)[0].signal?.aborted).toBe(true);
    slow.resolve(jsonResponse(buildAvailabilityResponse()));
  });
});
