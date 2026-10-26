// @playwright/test is installed by the e2e runner only, not in web/package.json.
// @ts-ignore TS2307 when the package is absent
import { test, expect, type APIRequestContext, type Page, type Request, type Response, type Route } from "@playwright/test";

type PageFixtures = { page: Page };

/**
 * E2E: the "Markt suchen" (StoreSearch) flow on the home page, against the
 * running `next start` front end. Data-dependent assertions stub GET
 * /api/stores in the browser so they do not depend on the database content;
 * the empty-state and contract checks go to the real Go API.
 */

const BASE_URL = process.env.AQAA_BASE_URL ?? "http://localhost:3000";
const API_URL = process.env.AQAA_APP_API_URL ?? "http://localhost:8080";

const DEFAULT_PLZ = "73035";

const t = {
  heading: "Markt suchen",
  placeholder: "Marktname oder Stadt",
  noResults: "Keine passenden Märkte gefunden.",
  results: "Treffer",
  loadMore: "Weitere Märkte laden",
  error: "Suche fehlgeschlagen. Bitte erneut versuchen.",
  inStock: "Auf Lager",
  outOfStock: "Nicht verfügbar",
};

// ---------------------------------------------------------------------------
// Fixtures mirroring the StoreSearchResponse schema of the API contract
// ---------------------------------------------------------------------------

interface StoreFixture {
  id: string;
  name: string;
  plz: string;
  city: string;
  distanceKm: number | null;
  inStock: boolean[];
}

function storeAvailability(s: StoreFixture) {
  return {
    store: {
      id: s.id,
      sourceKey: "obi",
      name: s.name,
      street: null,
      plz: s.plz,
      city: s.city,
      lat: 48.7,
      lon: 9.6,
    },
    distanceKm: s.distanceKm,
    listings: s.inStock.map((inStock, i) => ({
      sourceKey: "obi",
      productId: `p-${i}`,
      storeId: s.id,
      inStock,
      quantity: inStock ? 3 : 0,
      priceCents: 49900,
      currency: "EUR",
      url: `https://example.test/${s.id}/${i}`,
      checkedAt: "2026-10-01T10:00:00Z",
    })),
  };
}

function searchBody(stores: StoreFixture[], total: number, page = 1) {
  return { stores: stores.map(storeAvailability), total, page, pageSize: 20 };
}

const CORS = { "access-control-allow-origin": "*" };

/** Stubs GET /api/stores; `respond` gets the query of each request. */
async function stubStoreSearch(
  page: Page,
  respond: (query: URLSearchParams) => { status?: number; body: unknown },
): Promise<URLSearchParams[]> {
  const seen: URLSearchParams[] = [];
  await page.route(
    (url: URL) => url.pathname === "/api/stores",
    async (route: Route) => {
      const query = new URL(route.request().url()).searchParams;
      seen.push(query);
      const { status = 200, body } = respond(query);
      await route.fulfill({ status, headers: CORS, contentType: "application/json", body: JSON.stringify(body) });
    },
  );
  return seen;
}

function isAvailabilityRequest(req: Request): boolean {
  return new URL(req.url()).pathname === "/api/availability";
}

/** Opens the home page and waits until it is hydrated (first availability fetch fired). */
async function openHome(page: Page) {
  const hydrated = page.waitForRequest(isAvailabilityRequest);
  await page.goto(BASE_URL);
  await hydrated;
}

function searchSection(page: Page) {
  return page.locator("section").filter({ has: page.getByRole("heading", { name: t.heading }) });
}

async function submitSearch(page: Page, term: string) {
  const section = searchSection(page);
  await section.getByPlaceholder(t.placeholder).fill(term);
  await section.locator('button[type="submit"]').click();
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe("store search by name or city", () => {
  test("submits the trimmed term with the default PLZ and shows rows with stock badges", async ({ page }: PageFixtures) => {
    const seen = await stubStoreSearch(page, () => ({
      body: searchBody(
        [
          { id: "s-1", name: "OBI Göppingen", plz: "73033", city: "Göppingen", distanceKm: 2.345, inStock: [false, true] },
          { id: "s-2", name: "OBI Stuttgart", plz: "70173", city: "Stuttgart", distanceKm: null, inStock: [false] },
        ],
        2,
      ),
    }));
    await openHome(page);

    await submitSearch(page, "  OBI  ");

    const section = searchSection(page);
    await expect(section.getByText(`2 ${t.results}`)).toBeVisible();
    const rows = section.getByRole("listitem");
    await expect(rows).toHaveCount(2);

    // B22: name, "plz city" plus distance when known, badge in stock iff any listing is.
    await expect(rows.nth(0)).toContainText("OBI Göppingen");
    await expect(rows.nth(0)).toContainText("73033 Göppingen · 2,3 km");
    await expect(rows.nth(0)).toContainText(t.inStock);
    await expect(rows.nth(1)).toContainText("OBI Stuttgart");
    await expect(rows.nth(1).getByText("70173 Stuttgart", { exact: true })).toBeVisible();
    await expect(rows.nth(1)).toContainText(t.outOfStock);
    await expect(rows.nth(1)).not.toContainText("km");

    // B19/B20: trimmed term, default PLZ filter, first page.
    expect(seen).toHaveLength(1);
    expect(seen[0].get("q")).toBe("OBI");
    expect(seen[0].get("plz")).toBe(DEFAULT_PLZ);
    expect(seen[0].get("page")).toBe("1");
    await expect(section.getByRole("button", { name: t.loadMore })).toHaveCount(0);
  });

  test("omits plz once the PLZ filter is cleared", async ({ page }: PageFixtures) => {
    const seen = await stubStoreSearch(page, () => ({ body: searchBody([], 0) }));
    await openHome(page);

    const cleared = page.waitForRequest(
      (req: Request) => isAvailabilityRequest(req) && !new URL(req.url()).searchParams.has("plz"),
    );
    await page.getByLabel("Postleitzahl (PLZ)").fill("");
    await cleared;

    await submitSearch(page, "Ulm");
    await expect(searchSection(page).getByText(t.noResults)).toBeVisible();

    expect(seen).toHaveLength(1);
    expect(seen[0].get("q")).toBe("Ulm");
    expect(seen[0].has("plz")).toBe(false);
  });

  test("loads more pages and a new search replaces the results", async ({ page }: PageFixtures) => {
    const seen = await stubStoreSearch(page, (query) => {
      if (query.get("q") === "Ulm") {
        return { body: searchBody([{ id: "u-1", name: "Bauhaus Ulm", plz: "89073", city: "Ulm", distanceKm: 40, inStock: [true] }], 1) };
      }
      return query.get("page") === "2"
        ? { body: searchBody([{ id: "m-3", name: "Markt C", plz: "73037", city: "Göppingen", distanceKm: 5, inStock: [] }], 3, 2) }
        : {
            body: searchBody(
              [
                { id: "m-1", name: "Markt A", plz: "73033", city: "Göppingen", distanceKm: 1, inStock: [true] },
                { id: "m-2", name: "Markt B", plz: "73035", city: "Göppingen", distanceKm: 3, inStock: [false] },
              ],
              3,
            ),
          };
    });
    await openHome(page);

    await submitSearch(page, "Markt");
    const section = searchSection(page);
    const rows = section.getByRole("listitem");
    await expect(section.getByText(`3 ${t.results}`)).toBeVisible();
    await expect(rows).toHaveCount(2);

    const loadMore = section.getByRole("button", { name: t.loadMore });
    await loadMore.click();
    await expect(rows).toHaveCount(3);
    await expect(rows.nth(0)).toContainText("Markt A");
    await expect(rows.nth(2)).toContainText("Markt C");
    // No listings at all means not in stock.
    await expect(rows.nth(2)).toContainText(t.outOfStock);
    await expect(loadMore).toHaveCount(0);
    expect(seen[1].get("page")).toBe("2");
    expect(seen[1].get("q")).toBe("Markt");

    await submitSearch(page, "Ulm");
    await expect(section.getByText(`1 ${t.results}`)).toBeVisible();
    await expect(rows).toHaveCount(1);
    await expect(rows.nth(0)).toContainText("Bauhaus Ulm");
    expect(seen[2].get("page")).toBe("1");
  });

  test("shows an error message when the search request fails", async ({ page }: PageFixtures) => {
    await stubStoreSearch(page, () => ({ status: 500, body: { error: "internal", message: "boom" } }));
    await openHome(page);

    await submitSearch(page, "OBI");

    const section = searchSection(page);
    await expect(section.getByText(t.error)).toBeVisible();
    await expect(section.getByRole("listitem")).toHaveCount(0);
  });

  test("shows the empty state for a term without matches (real API)", async ({ page }: PageFixtures) => {
    await openHome(page);

    const response = page.waitForResponse((res: Response) => new URL(res.url()).pathname === "/api/stores");
    await submitSearch(page, "zzqx-aqaa-kein-markt");
    expect((await response).status()).toBe(200);

    const section = searchSection(page);
    await expect(section.getByText(`0 ${t.results}`)).toBeVisible();
    await expect(section.getByText(t.noResults)).toBeVisible();
    await expect(section.getByRole("listitem")).toHaveCount(0);
  });

  test("GET /api/stores returns a full page when enough stores match (real API)", async ({ request }: { request: APIRequestContext }) => {
    const res = await request.get(`${API_URL}/api/stores?pageSize=1&page=1`);
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.page).toBe(1);
    expect(body.pageSize).toBe(1);
    expect(body.stores).toHaveLength(Math.min(1, body.total));
  });
});
