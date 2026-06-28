import { z } from "zod";

/**
 * Typed API client + Zod schemas mirroring docs/api-contract.md.
 * Base URL comes from NEXT_PUBLIC_API_BASE_URL.
 */

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";

// ---------------------------------------------------------------------------
// Shared schemas (mirror "Shared types" in the contract)
// ---------------------------------------------------------------------------

export const ProductSchema = z.object({
  id: z.string(),
  name: z.string(),
  model: z.string(),
  imageUrl: z.string().nullable(),
});
export type Product = z.infer<typeof ProductSchema>;

export const SourceSchema = z.object({
  key: z.string(),
  name: z.string(),
  type: z.enum(["online", "store"]),
  baseUrl: z.string(),
  lastCheckedAt: z.string().nullable(),
  lastStatus: z.enum(["ok", "error", "never"]),
  lastError: z.string().nullable(),
});
export type Source = z.infer<typeof SourceSchema>;

export const StoreSchema = z.object({
  id: z.string(),
  sourceKey: z.string(),
  name: z.string(),
  street: z.string().nullable(),
  plz: z.string(),
  city: z.string(),
  lat: z.number(),
  lon: z.number(),
});
export type Store = z.infer<typeof StoreSchema>;

export const ListingSchema = z.object({
  sourceKey: z.string(),
  productId: z.string(),
  storeId: z.string().nullable(),
  inStock: z.boolean(),
  quantity: z.number().nullable(),
  priceCents: z.number().nullable(),
  currency: z.string(),
  url: z.string(),
  checkedAt: z.string(),
});
export type Listing = z.infer<typeof ListingSchema>;

// ---------------------------------------------------------------------------
// Endpoint response schemas
// ---------------------------------------------------------------------------

export const ProductsResponseSchema = z.object({
  products: z.array(ProductSchema),
});
export type ProductsResponse = z.infer<typeof ProductsResponseSchema>;

export const SourcesResponseSchema = z.object({
  sources: z.array(SourceSchema),
});
export type SourcesResponse = z.infer<typeof SourcesResponseSchema>;

export const CenterSchema = z.object({
  plz: z.string(),
  lat: z.number(),
  lon: z.number(),
});
export type Center = z.infer<typeof CenterSchema>;

export const StoreAvailabilitySchema = z.object({
  store: StoreSchema,
  distanceKm: z.number().nullable(),
  listings: z.array(ListingSchema),
});
export type StoreAvailability = z.infer<typeof StoreAvailabilitySchema>;

export const OnlineAvailabilitySchema = z.object({
  source: SourceSchema,
  listings: z.array(ListingSchema),
});
export type OnlineAvailability = z.infer<typeof OnlineAvailabilitySchema>;

export const AvailabilityResponseSchema = z.object({
  center: CenterSchema.nullable(),
  radiusKm: z.number(),
  stores: z.array(StoreAvailabilitySchema),
  online: z.array(OnlineAvailabilitySchema),
  generatedAt: z.string(),
});
export type AvailabilityResponse = z.infer<typeof AvailabilityResponseSchema>;

export const SubscriptionResponseSchema = z.object({
  status: z.literal("pending_confirmation"),
});
export type SubscriptionResponse = z.infer<typeof SubscriptionResponseSchema>;

export const ApiErrorSchema = z.object({
  error: z.string(),
  message: z.string().optional(),
  field: z.string().nullable().optional(),
});
export type ApiErrorBody = z.infer<typeof ApiErrorSchema>;

// ---------------------------------------------------------------------------
// Error type
// ---------------------------------------------------------------------------

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly field: string | null;

  constructor(status: number, body: ApiErrorBody) {
    super(body.message ?? body.error);
    this.name = "ApiError";
    this.status = status;
    this.code = body.error;
    this.field = body.field ?? null;
  }
}

// ---------------------------------------------------------------------------
// Internal fetch helpers
// ---------------------------------------------------------------------------

function buildUrl(path: string, params?: Record<string, string | number | undefined>): string {
  const url = new URL(path.replace(/^\//, ""), API_BASE_URL.endsWith("/") ? API_BASE_URL : `${API_BASE_URL}/`);
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== null && value !== "") {
        url.searchParams.set(key, String(value));
      }
    }
  }
  return url.toString();
}

async function parseError(res: Response): Promise<never> {
  let body: ApiErrorBody = { error: "internal", message: res.statusText };
  try {
    const json = await res.json();
    const parsed = ApiErrorSchema.safeParse(json);
    if (parsed.success) body = parsed.data;
  } catch {
    // non-JSON error body; keep default
  }
  throw new ApiError(res.status, body);
}

async function request<T>(
  schema: z.ZodType<T>,
  path: string,
  init?: RequestInit & { params?: Record<string, string | number | undefined> },
): Promise<T> {
  const { params, ...rest } = init ?? {};
  const res = await fetch(buildUrl(path, params), {
    ...rest,
    headers: {
      Accept: "application/json",
      ...(rest.body ? { "Content-Type": "application/json" } : {}),
      ...rest.headers,
    },
  });

  if (!res.ok) {
    return parseError(res);
  }

  const json = await res.json();
  return schema.parse(json);
}

// ---------------------------------------------------------------------------
// Public fetchers
// ---------------------------------------------------------------------------

export interface AvailabilityQuery {
  plz?: string;
  radius?: number;
  productId?: string;
}

export function getAvailability(
  query: AvailabilityQuery = {},
  signal?: AbortSignal,
): Promise<AvailabilityResponse> {
  return request(AvailabilityResponseSchema, "/api/availability", {
    method: "GET",
    signal,
    params: {
      plz: query.plz,
      radius: query.radius,
      productId: query.productId,
    },
  });
}

export function getProducts(signal?: AbortSignal): Promise<ProductsResponse> {
  return request(ProductsResponseSchema, "/api/products", { method: "GET", signal });
}

export function getSources(signal?: AbortSignal): Promise<SourcesResponse> {
  return request(SourcesResponseSchema, "/api/sources", { method: "GET", signal });
}

export interface CreateSubscriptionInput {
  email: string;
  plz: string;
  radiusKm?: number;
  productId?: string | null;
}

export function createSubscription(
  input: CreateSubscriptionInput,
  signal?: AbortSignal,
): Promise<SubscriptionResponse> {
  return request(SubscriptionResponseSchema, "/api/subscriptions", {
    method: "POST",
    signal,
    body: JSON.stringify({
      email: input.email,
      plz: input.plz,
      radiusKm: input.radiusKm ?? 50,
      productId: input.productId ?? null,
    }),
  });
}
