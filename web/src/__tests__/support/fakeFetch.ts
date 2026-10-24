import { vi } from "vitest";

/**
 * Fake for the global `fetch` boundary used by src/lib/api-client.ts.
 * Routes are matched on method + pathname; unmatched requests answer 404.
 * Every call is recorded with its parsed URL and JSON body.
 *
 *   const api = installFakeFetch();
 *   api.on("GET", "/api/availability", () => jsonResponse(buildAvailabilityResponse()));
 *   ...
 *   api.restore();
 */

export interface RecordedCall {
  method: string;
  url: URL;
  body: unknown;
  signal: AbortSignal | null | undefined;
}

type Handler = (call: RecordedCall) => Response | Promise<Response>;

export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

export function apiErrorResponse(
  status: number,
  error: string,
  message?: string,
  field?: string | null,
): Response {
  return jsonResponse({ error, message, field }, status);
}

/** A promise you resolve by hand; use it to hold a request "in flight". */
export function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

export function installFakeFetch() {
  const routes = new Map<string, Handler[]>();
  const calls: RecordedCall[] = [];
  const original = globalThis.fetch;

  const mock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(typeof input === "string" ? input : input instanceof URL ? input.href : input.url);
    const method = (init?.method ?? "GET").toUpperCase();
    const body = typeof init?.body === "string" ? JSON.parse(init.body) : undefined;
    const call: RecordedCall = { method, url, body, signal: init?.signal };
    calls.push(call);
    if (init?.signal?.aborted) {
      throw new DOMException("The operation was aborted.", "AbortError");
    }
    const queue = routes.get(`${method} ${url.pathname}`);
    if (!queue || queue.length === 0) {
      return apiErrorResponse(404, "not_found", `no fake route for ${method} ${url.pathname}`);
    }
    // The last handler sticks; earlier ones are used once each (like mockResolvedValueOnce).
    const handler = queue.length > 1 ? queue.shift()! : queue[0];
    return handler(call);
  });

  globalThis.fetch = mock as unknown as typeof fetch;

  const api = {
    mock,
    calls,
    on(method: string, pathname: string, handler: Handler) {
      const key = `${method.toUpperCase()} ${pathname}`;
      routes.set(key, [...(routes.get(key) ?? []), handler]);
      return api;
    },
    callsTo(pathname: string) {
      return calls.filter((c) => c.url.pathname === pathname);
    },
    restore() {
      globalThis.fetch = original;
    },
  };
  return api;
}

export type FakeFetch = ReturnType<typeof installFakeFetch>;
