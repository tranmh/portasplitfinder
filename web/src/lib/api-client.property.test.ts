import { afterEach, expect, test, vi } from "vitest";
import fc from "fast-check";
import { ApiError, getAvailability, searchStores } from "./api-client";

const originalFetch = global.fetch;

afterEach(() => {
  global.fetch = originalFetch;
  vi.unstubAllEnvs();
  vi.resetModules();
});

const emptySearch = { stores: [], total: 0, page: 1, pageSize: 20 };

function jsonResponse(body: unknown, status = 200, statusText = ""): Response {
  return new Response(JSON.stringify(body), {
    status,
    statusText,
    headers: { "Content-Type": "application/json" },
  });
}

function captureFetch(response: () => Response): { urls: string[] } {
  const calls = { urls: [] as string[] };
  global.fetch = vi.fn(async (input: RequestInfo | URL) => {
    calls.urls.push(String(input));
    return response();
  }) as unknown as typeof fetch;
  return calls;
}

test("searchStores query params round trip through the URL", async () => {
  await fc.assert(
    fc.asyncProperty(
      fc.option(fc.fullUnicodeString(), { nil: undefined }),
      fc.option(fc.string(), { nil: undefined }),
      fc.option(fc.integer(), { nil: undefined }),
      fc.option(fc.integer(), { nil: undefined }),
      async (q, plz, page, pageSize) => {
        const calls = captureFetch(() => jsonResponse(emptySearch));
        await searchStores({ q, plz, page, pageSize });
        expect(calls.urls).toHaveLength(1);
        const url = new URL(calls.urls[0]);
        expect(url.pathname.endsWith("/api/stores")).toBe(true);
        expect(url.searchParams.get("q")).toBe(q === undefined || q === "" ? null : q);
        expect(url.searchParams.get("plz")).toBe(plz === undefined || plz === "" ? null : plz);
        expect(url.searchParams.get("page")).toBe(page === undefined ? null : String(page));
        expect(url.searchParams.get("pageSize")).toBe(
          pageSize === undefined ? null : String(pageSize),
        );
        expect([...url.searchParams.keys()].every((k) => ["q", "plz", "page", "pageSize"].includes(k))).toBe(true);
      },
    ),
  );
});

test("getAvailability query params round trip through the URL", async () => {
  const availability = { center: null, radiusKm: 50, stores: [], online: [], generatedAt: "x" };
  await fc.assert(
    fc.asyncProperty(
      fc.option(fc.fullUnicodeString(), { nil: undefined }),
      fc.option(fc.double({ noNaN: true }), { nil: undefined }),
      fc.option(fc.fullUnicodeString(), { nil: undefined }),
      async (plz, radius, productId) => {
        const calls = captureFetch(() => jsonResponse(availability));
        await getAvailability({ plz, radius, productId });
        const url = new URL(calls.urls[0]);
        expect(url.pathname.endsWith("/api/availability")).toBe(true);
        expect(url.searchParams.get("plz")).toBe(plz === undefined || plz === "" ? null : plz);
        expect(url.searchParams.get("radius")).toBe(radius === undefined ? null : String(radius));
        expect(url.searchParams.get("productId")).toBe(
          productId === undefined || productId === "" ? null : productId,
        );
      },
    ),
  );
});

test("searchStores maps any JSON error body to an ApiError", async () => {
  await fc.assert(
    fc.asyncProperty(
      fc.integer({ min: 400, max: 599 }),
      fc.string(),
      fc.option(fc.string(), { nil: undefined }),
      fc.option(fc.option(fc.string(), { nil: null }), { nil: undefined }),
      async (status, error, message, field) => {
        captureFetch(() => jsonResponse({ error, message, field }, status));
        const err = await searchStores({ q: "x" }).then(
          () => {
            throw new Error("expected a rejection");
          },
          (e: unknown) => e,
        );
        expect(err).toBeInstanceOf(ApiError);
        const apiErr = err as ApiError;
        expect(apiErr.name).toBe("ApiError");
        expect(apiErr.status).toBe(status);
        expect(apiErr.code).toBe(error);
        expect(apiErr.message).toBe(message ?? error);
        expect(apiErr.field).toBe(field ?? null);
      },
    ),
  );
});

test("searchStores maps a non JSON error body to an internal ApiError", async () => {
  await fc.assert(
    fc.asyncProperty(
      fc.integer({ min: 400, max: 599 }),
      fc.stringMatching(/^[A-Za-z0-9 ]{0,20}$/),
      fc.string(),
      async (status, statusText, body) => {
        let parses = true;
        try {
          JSON.parse(body);
        } catch {
          parses = false;
        }
        fc.pre(!parses);
        captureFetch(() => new Response(body, { status, statusText }));
        const err = await searchStores({ q: "x" }).then(
          () => {
            throw new Error("expected a rejection");
          },
          (e: unknown) => e,
        );
        expect(err).toBeInstanceOf(ApiError);
        const apiErr = err as ApiError;
        expect(apiErr.status).toBe(status);
        expect(apiErr.code).toBe("internal");
        expect(apiErr.message).toBe(statusText);
        expect(apiErr.field).toBeNull();
      },
    ),
  );
});

test("searchStores joins the base URL and the path with one slash", async () => {
  const segment = fc.stringMatching(/^[a-z0-9-]{1,8}$/);
  await fc.assert(
    fc.asyncProperty(
      fc.array(segment, { maxLength: 3 }),
      fc.boolean(),
      async (segments, trailingSlash) => {
        const prefix = segments.length ? `/${segments.join("/")}` : "";
        const base = `http://api.example.test${prefix}${trailingSlash ? "/" : ""}`;
        vi.resetModules();
        vi.stubEnv("NEXT_PUBLIC_API_BASE_URL", base);
        const mod = await import("./api-client");
        expect(mod.API_BASE_URL).toBe(base);
        const calls = captureFetch(() => jsonResponse(emptySearch));
        await mod.searchStores({});
        expect(calls.urls[0]).toBe(`http://api.example.test${prefix}/api/stores`);
      },
    ),
  );
});
