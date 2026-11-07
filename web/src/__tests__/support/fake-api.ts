import { vi, type Mock } from "vitest";
import type {
  ApiErrorBody,
  AvailabilityResponse,
  Product,
  Source,
} from "@/lib/api-client";
import { aMixedStockScenario, aProduct, aSource, anOnlineSource } from "./builders";

/**
 * In-memory fake of the Go backend behind `fetch` (docs/api-contract.md).
 * It answers by path, so it works whatever NEXT_PUBLIC_API_BASE_URL is, and
 * it honours AbortSignal like the real fetch does.
 *
 *   const api = new FakeApi();
 *   const fetchMock = api.install();       // in beforeEach
 *   api.uninstall();                       // in afterEach
 *   api.failNext("GET", "/api/availability", 500);
 *   const held = api.hold("GET", "/api/availability");  // loading states
 *   held.release();
 */

export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

export function errorResponse(
  status: number,
  error: string,
  message?: string,
  field: string | null = null,
): Response {
  const body: ApiErrorBody = { error, message: message ?? error, field };
  return jsonResponse(body, status);
}

export interface RecordedRequest {
  method: string;
  url: URL;
  path: string;
  body: unknown;
}

export interface FakeSubscription {
  token: string;
  email: string;
  plz: string;
  radiusKm: number;
  productId: string | null;
  status: "pending_confirmation" | "confirmed" | "unsubscribed";
}

type Handler = (req: RecordedRequest) => Response | Promise<Response>;

interface Override {
  method: string;
  path: string;
  handler: Handler;
  once: boolean;
}

export interface HeldRequest {
  /** Resolves once the request has arrived at the fake. */
  arrived: Promise<RecordedRequest>;
  /** Lets the request through to the normal handler (or the given response). */
  release(response?: Response): void;
  /** Fails the request like a network error. */
  fail(error?: Error): void;
}

const PLZ = /^\d{5}$/;
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function abortError(): DOMException {
  return new DOMException("The operation was aborted.", "AbortError");
}

export class FakeApi {
  availability: AvailabilityResponse;
  products: Product[];
  sources: Source[];
  readonly subscriptions: FakeSubscription[] = [];
  readonly requests: RecordedRequest[] = [];

  private overrides: Override[] = [];
  private originalFetch: typeof fetch | undefined;
  private tokenSeq = 0;

  constructor(seed: { availability?: AvailabilityResponse; products?: Product[]; sources?: Source[] } = {}) {
    this.availability = seed.availability ?? aMixedStockScenario();
    this.products = seed.products ?? [aProduct()];
    this.sources = seed.sources ?? [aSource(), aSource({ key: "bauhaus", name: "BAUHAUS" }), anOnlineSource()];
  }

  /** Replaces globalThis.fetch with a vi.fn backed by this fake. */
  install(): Mock<typeof fetch> {
    this.originalFetch = globalThis.fetch;
    const fn = vi.fn<typeof fetch>((input, init) => this.fetch(input, init));
    globalThis.fetch = fn;
    return fn;
  }

  uninstall(): void {
    if (this.originalFetch) globalThis.fetch = this.originalFetch;
    this.originalFetch = undefined;
  }

  /** Answers every matching request with the handler (until reset). */
  respondWith(method: string, path: string, handler: Handler | Response): void {
    this.overrides.push({ method, path, handler: toHandler(handler), once: false });
  }

  /** Answers the next matching request with the handler, then falls back. */
  respondOnce(method: string, path: string, handler: Handler | Response): void {
    this.overrides.push({ method, path, handler: toHandler(handler), once: true });
  }

  /** The next matching request gets an error body in the contract's shape. */
  failNext(method: string, path: string, status = 500, error = "internal", field: string | null = null): void {
    this.respondOnce(method, path, () => errorResponse(status, error, undefined, field));
  }

  /** Holds the next matching request until release()/fail() is called. */
  hold(method: string, path: string): HeldRequest {
    let arrive!: (r: RecordedRequest) => void;
    let settle!: (r: Response | Error | undefined) => void;
    const arrived = new Promise<RecordedRequest>((r) => (arrive = r));
    const settled = new Promise<Response | Error | undefined>((r) => (settle = r));
    this.respondOnce(method, path, async (req) => {
      arrive(req);
      const outcome = await settled;
      if (outcome instanceof Error) throw outcome;
      return outcome ?? this.route(req);
    });
    return {
      arrived,
      release: (response) => settle(response),
      fail: (error) => settle(error ?? new TypeError("Failed to fetch")),
    };
  }

  reset(): void {
    this.overrides = [];
    this.requests.length = 0;
  }

  requestsTo(path: string): RecordedRequest[] {
    return this.requests.filter((r) => r.path === path);
  }

  async fetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    const signal = init?.signal ?? (input instanceof Request ? input.signal : undefined);
    if (signal?.aborted) throw abortError();

    const url = new URL(input instanceof Request ? input.url : String(input), "http://localhost");
    const method = (init?.method ?? (input instanceof Request ? input.method : "GET")).toUpperCase();
    const rawBody = init?.body;
    const req: RecordedRequest = {
      method,
      url,
      path: url.pathname,
      body: typeof rawBody === "string" ? safeJson(rawBody) : undefined,
    };
    this.requests.push(req);

    const idx = this.overrides.findIndex((o) => o.method === method && o.path === req.path);
    const handler: Handler = idx >= 0 ? this.overrides[idx].handler : (r) => this.route(r);
    if (idx >= 0 && this.overrides[idx].once) this.overrides.splice(idx, 1);

    const response = Promise.resolve().then(() => handler(req));
    if (!signal) return response;
    return new Promise<Response>((resolve, reject) => {
      const onAbort = () => reject(abortError());
      signal.addEventListener("abort", onAbort, { once: true });
      response.then(
        (r) => {
          signal.removeEventListener("abort", onAbort);
          if (signal.aborted) reject(abortError());
          else resolve(r);
        },
        (e) => {
          signal.removeEventListener("abort", onAbort);
          reject(e);
        },
      );
    });
  }

  /** The default behaviour of the backend for each contract endpoint. */
  route(req: RecordedRequest): Response {
    const q = req.url.searchParams;
    switch (`${req.method} ${req.path}`) {
      case "GET /api/health":
        return jsonResponse({ status: "ok", db: "ok", time: this.availability.generatedAt });
      case "GET /api/products":
        return jsonResponse({ products: this.products });
      case "GET /api/sources":
        return jsonResponse({ sources: this.sources });
      case "GET /api/availability":
        return this.getAvailability(q.get("plz"), q.get("radius"), q.get("productId"));
      case "POST /api/subscriptions":
        return this.subscribe(req.body);
      case "GET /api/subscriptions/confirm":
        return this.setStatus(q.get("token"), "confirmed");
      case "GET /api/subscriptions/unsubscribe":
        return this.setStatus(q.get("token"), "unsubscribed");
      default:
        return errorResponse(404, "not_found", `no route ${req.method} ${req.path}`);
    }
  }

  private getAvailability(plz: string | null, radiusRaw: string | null, productId: string | null): Response {
    if (plz !== null && !PLZ.test(plz)) {
      return errorResponse(400, "validation_error", "plz must be 5 digits", "plz");
    }
    const radius = radiusRaw === null ? 50 : Number(radiusRaw);
    if (!Number.isInteger(radius) || radius < 1 || radius > 500) {
      return errorResponse(400, "validation_error", "radius must be 1..500", "radius");
    }
    const base = this.availability;
    const byProduct = <T extends { listings: { productId: string }[] }>(e: T): T =>
      productId ? { ...e, listings: e.listings.filter((l) => l.productId === productId) } : e;

    if (plz === null) {
      return jsonResponse({
        ...base,
        center: null,
        stores: base.stores.map((s) => byProduct({ ...s, distanceKm: null })),
        online: base.online.map(byProduct),
      });
    }
    return jsonResponse({
      ...base,
      center: base.center ? { ...base.center, plz } : { plz, lat: 0, lon: 0 },
      radiusKm: radius,
      stores: base.stores
        .filter((s) => s.distanceKm === null || s.distanceKm <= radius)
        .map(byProduct),
      online: base.online.map(byProduct),
    });
  }

  private subscribe(body: unknown): Response {
    const b = (body ?? {}) as Record<string, unknown>;
    const email = typeof b.email === "string" ? b.email : "";
    const plz = typeof b.plz === "string" ? b.plz : "";
    const radiusKm = b.radiusKm === undefined ? 50 : b.radiusKm;
    if (!EMAIL.test(email)) return errorResponse(400, "validation_error", "invalid email", "email");
    if (!PLZ.test(plz)) return errorResponse(400, "validation_error", "invalid plz", "plz");
    if (typeof radiusKm !== "number" || !Number.isInteger(radiusKm) || radiusKm < 1 || radiusKm > 500) {
      return errorResponse(400, "validation_error", "invalid radius", "radiusKm");
    }
    this.subscriptions.push({
      token: `token-${++this.tokenSeq}`,
      email,
      plz,
      radiusKm,
      productId: typeof b.productId === "string" ? b.productId : null,
      status: "pending_confirmation",
    });
    return jsonResponse({ status: "pending_confirmation" }, 202);
  }

  private setStatus(token: string | null, status: "confirmed" | "unsubscribed"): Response {
    const sub = this.subscriptions.find((s) => s.token === token);
    if (!sub) return errorResponse(404, "invalid_token", "unknown token");
    sub.status = status;
    return jsonResponse(status === "confirmed" ? { status, email: sub.email } : { status });
  }
}

function toHandler(h: Handler | Response): Handler {
  // A Response body can be read once; clone so respondWith can serve it repeatedly.
  return typeof h === "function" ? h : () => h.clone();
}

function safeJson(s: string): unknown {
  try {
    return JSON.parse(s);
  } catch {
    return s;
  }
}
