import { describe, it, expect, afterEach, vi } from "vitest";
import { ApiError, searchStores } from "./api-client";
import {
  buildStoreAvailability,
  buildStore,
  buildStoreSearchResponse,
  jsonResponse,
  stubFetch,
} from "@/test/__tests__/doubles";

describe("searchStores", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("calls GET /api/stores with q, plz, page and pageSize", async () => {
    const fetch = stubFetch(jsonResponse(buildStoreSearchResponse()));

    await searchStores({ q: "obi", plz: "10115", page: 2, pageSize: 10 });

    const url = fetch.url();
    expect(url.pathname).toBe("/api/stores");
    expect(url.searchParams.get("q")).toBe("obi");
    expect(url.searchParams.get("plz")).toBe("10115");
    expect(url.searchParams.get("page")).toBe("2");
    expect(url.searchParams.get("pageSize")).toBe("10");
    const init = fetch.fn.mock.calls[0][1] as RequestInit;
    expect(init.method).toBe("GET");
    expect((init.headers as Record<string, string>).Accept).toBe("application/json");
  });

  it("encodes special characters in the query", async () => {
    const fetch = stubFetch(jsonResponse(buildStoreSearchResponse()));

    await searchStores({ q: "Köln & Bonn ?#/" });

    expect(fetch.url().searchParams.get("q")).toBe("Köln & Bonn ?#/");
    expect(fetch.url().searchParams.getAll("q")).toHaveLength(1);
    expect(fetch.url().hash).toBe("");
  });

  it("omits undefined and empty parameters", async () => {
    const fetch = stubFetch(jsonResponse(buildStoreSearchResponse()));

    await searchStores({ q: "", plz: undefined });

    expect([...fetch.url().searchParams.keys()]).toEqual([]);
  });

  it("works without any query argument", async () => {
    const fetch = stubFetch(jsonResponse(buildStoreSearchResponse([])));

    const res = await searchStores();

    expect(fetch.url().pathname).toBe("/api/stores");
    expect(res.stores).toEqual([]);
  });

  it("keeps page 0 as a parameter (only undefined and empty are dropped)", async () => {
    const fetch = stubFetch(jsonResponse(buildStoreSearchResponse()));

    await searchStores({ page: 0 });

    expect(fetch.url().searchParams.get("page")).toBe("0");
  });

  it("passes the abort signal to fetch", async () => {
    const fetch = stubFetch(jsonResponse(buildStoreSearchResponse()));
    const controller = new AbortController();

    await searchStores({ q: "x" }, controller.signal);

    expect((fetch.fn.mock.calls[0][1] as RequestInit).signal).toBe(controller.signal);
  });

  it("returns the parsed response shape", async () => {
    const body = buildStoreSearchResponse(
      [
        buildStoreAvailability(),
        buildStoreAvailability({ store: buildStore({ id: "s2", name: "Bauhaus" }), distanceKm: null }),
      ],
      { total: 42, page: 3, pageSize: 2 },
    );
    stubFetch(jsonResponse(body));

    const res = await searchStores({ q: "x" });

    expect(res).toEqual(body);
    expect(res.stores[1].distanceKm).toBeNull();
  });

  it("rejects a response that does not match the schema", async () => {
    stubFetch(jsonResponse({ stores: [], total: "many", page: 1, pageSize: 20 }));

    await expect(searchStores({ q: "x" })).rejects.toThrow();
    await expect(searchStores({ q: "x" })).rejects.not.toBeInstanceOf(ApiError);
  });

  it("rejects a response missing the paging fields", async () => {
    stubFetch(jsonResponse({ stores: [] }));

    await expect(searchStores()).rejects.toThrow();
  });

  it("maps a JSON error body to ApiError", async () => {
    stubFetch(
      jsonResponse({ error: "validation_error", message: "bad plz", field: "plz" }, 400),
    );

    const err = await searchStores({ plz: "1" }).catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({
      status: 400,
      code: "validation_error",
      field: "plz",
      message: "bad plz",
    });
  });

  it("maps a non-JSON error body to an internal ApiError with the status text", async () => {
    stubFetch(
      () => new Response("<html>oops</html>", { status: 502, statusText: "Bad Gateway" }),
    );

    const err = await searchStores({ q: "x" }).catch((e: unknown) => e);

    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 502, code: "internal", field: null, message: "Bad Gateway" });
  });

  it("falls back to the default error when the JSON error body has the wrong shape", async () => {
    stubFetch(
      () =>
        new Response(JSON.stringify({ unexpected: true }), {
          status: 500,
          statusText: "Internal Server Error",
        }),
    );

    await expect(searchStores()).rejects.toMatchObject({
      status: 500,
      code: "internal",
      message: "Internal Server Error",
    });
  });

  it("uses the error code as message when no message is given", async () => {
    stubFetch(jsonResponse({ error: "not_found" }, 404));

    await expect(searchStores()).rejects.toMatchObject({
      status: 404,
      code: "not_found",
      message: "not_found",
      field: null,
    });
  });

  it("propagates network failures", async () => {
    stubFetch(new TypeError("Failed to fetch"));

    await expect(searchStores({ q: "x" })).rejects.toThrow("Failed to fetch");
  });
});
