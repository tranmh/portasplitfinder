import { expect, test, type Page, type Response } from "@playwright/test";

/**
 * Main user flow: the front page loads availability for the default filter
 * (PLZ 73035, 50 km), re-fetches when the PLZ / radius change, and the
 * in-stock toggle hides stores without stock. The expected stores are taken
 * from the API response the page itself received, so the specs hold for any
 * seeded data set.
 */

interface ListingBody {
  inStock: boolean;
}

interface AvailabilityBody {
  center: { plz: string; lat: number; lon: number } | null;
  radiusKm: number;
  stores: { store: { name: string }; distanceKm: number | null; listings: ListingBody[] }[];
  online: { source: { name: string }; listings: ListingBody[] }[];
}

function isAvailability(res: Response, plz?: string, radius?: number): boolean {
  if (res.request().method() !== "GET") return false;
  const url = new URL(res.url());
  if (!url.pathname.endsWith("/api/availability")) return false;
  if (plz !== undefined && url.searchParams.get("plz") !== plz) return false;
  if (radius !== undefined && url.searchParams.get("radius") !== String(radius)) return false;
  return true;
}

async function openHome(page: Page): Promise<AvailabilityBody> {
  const first = page.waitForResponse((r) => isAvailability(r, "73035", 50));
  await page.goto("/");
  const res = await first;
  expect(res.status()).toBe(200);
  return (await res.json()) as AvailabilityBody;
}

function storeSection(page: Page) {
  return page
    .locator("section")
    .filter({ has: page.getByRole("heading", { name: "Märkte in der Nähe" }) });
}

function filterSection(page: Page) {
  return page
    .locator("section")
    .filter({ has: page.getByRole("heading", { name: "Standort & Umkreis" }) });
}

/** Asserts the store list shows exactly these names, in this order. */
async function expectStoreNames(page: Page, names: string[]) {
  const section = storeSection(page);
  const items = section.locator(":scope > ul > li p.font-medium");
  if (names.length === 0) {
    await expect(section.getByText("Keine Märkte im gewählten Umkreis gefunden.")).toBeVisible();
    await expect(items).toHaveCount(0);
  } else {
    await expect(items).toHaveText(names);
  }
}

function formatKm(km: number): string {
  return `${new Intl.NumberFormat("de-DE", { maximumFractionDigits: 1 }).format(km)} km`;
}

test.describe("search by PLZ and radius", () => {
  test("front page defaults to PLZ 73035, 50 km, in-stock toggle off", async ({ page }) => {
    const body = await openHome(page);

    await expect(page.getByLabel("Postleitzahl (PLZ)", { exact: true })).toHaveValue("73035");
    await expect(page.getByRole("slider", { name: "Umkreis" })).toHaveValue("50");
    await expect(filterSection(page).getByText("50 km", { exact: true })).toBeVisible();
    await expect(
      page.getByRole("checkbox", { name: "Nur Märkte mit Verfügbarkeit anzeigen" }),
    ).not.toBeChecked();

    expect(body.center?.plz).toBe("73035");
    expect(body.radiusKm).toBe(50);
    await expectStoreNames(
      page,
      body.stores.map((s) => s.store.name),
    );

    // Online retailers live in their own nationwide panel.
    const online = page
      .locator("section")
      .filter({ has: page.getByRole("heading", { name: "Online-Händler" }) });
    await expect(online.getByText("deutschlandweit verfügbar").first()).toBeVisible();
    for (const entry of body.online) {
      await expect(online.getByText(entry.source.name, { exact: true })).toBeVisible();
    }
  });

  test("entering a PLZ re-fetches and lists stores within the radius with distance", async ({
    page,
  }) => {
    await openHome(page);

    const requested: string[] = [];
    page.on("request", (req) => {
      const url = new URL(req.url());
      if (url.pathname.endsWith("/api/availability")) requested.push(url.search);
    });

    const next = page.waitForResponse((r) => isAvailability(r, "80331", 50));
    const plzInput = page.getByLabel("Postleitzahl (PLZ)", { exact: true });
    await plzInput.fill("");
    await plzInput.pressSequentially("80331");
    const res = await next;
    expect(res.status()).toBe(200);
    const body = (await res.json()) as AvailabilityBody;

    expect(body.center?.plz).toBe("80331");
    expect(body.radiusKm).toBe(50);
    for (const s of body.stores) {
      expect(s.distanceKm).not.toBeNull();
      expect(s.distanceKm!).toBeLessThanOrEqual(50);
    }
    await expectStoreNames(
      page,
      body.stores.map((s) => s.store.name),
    );
    const items = storeSection(page).locator(":scope > ul > li");
    for (const [i, s] of body.stores.entries()) {
      await expect(items.nth(i)).toContainText(formatKm(s.distanceKm!));
    }

    // Partial PLZs are never sent; the typed PLZ is requested exactly once.
    for (const search of requested) {
      const plz = new URLSearchParams(search).get("plz");
      if (plz !== null) expect(plz).toMatch(/^\d{5}$/);
    }
    expect(requested.filter((s) => new URLSearchParams(s).get("plz") === "80331")).toHaveLength(1);
  });

  test("in-stock toggle hides stores without stock", async ({ page }) => {
    await openHome(page);

    // Widen to the maximum radius so the mock stores are in range.
    const wide = page.waitForResponse((r) => isAvailability(r, "73035", 500));
    const slider = page.getByRole("slider", { name: "Umkreis" });
    await slider.focus();
    await slider.press("End");
    await expect(filterSection(page).getByText("500 km", { exact: true })).toBeVisible();
    const res = await wide;
    expect(res.status()).toBe(200);
    const body = (await res.json()) as AvailabilityBody;

    const all = body.stores.map((s) => s.store.name);
    const withStock = body.stores
      .filter((s) => s.listings.some((l) => l.inStock))
      .map((s) => s.store.name);
    await expectStoreNames(page, all);

    const toggle = page.getByRole("checkbox", { name: "Nur Märkte mit Verfügbarkeit anzeigen" });
    await toggle.check();
    await expectStoreNames(page, withStock);
    const items = storeSection(page).locator(":scope > ul > li");
    for (let i = 0; i < withStock.length; i++) {
      await expect(items.nth(i).getByText("Auf Lager", { exact: true }).first()).toBeVisible();
    }

    await toggle.uncheck();
    await expectStoreNames(page, all);
  });
});
