import { test, expect, type Page, type Response } from "@playwright/test";
import { API_BASE_URL } from "../../playwright.config";

const API_URL = process.env.AQAA_APP_API_URL ?? API_BASE_URL;
const MUNICH = "OBI Markt München-Zentrum";

/** True for the browser's GET /api/availability?plz=<plz>&radius=<radius>. */
function isAvailability(res: Response, plz: string, radius: number): boolean {
  const u = new URL(res.url());
  return (
    res.request().method() === "GET" &&
    u.pathname === "/api/availability" &&
    u.searchParams.get("plz") === plz &&
    u.searchParams.get("radius") === String(radius)
  );
}

function storeListOf(page: Page) {
  return page
    .locator("section")
    .filter({ has: page.getByRole("heading", { name: "Märkte in der Nähe" }) });
}

/** The top-level store entries (one per store) rendered in the list. */
function storeItemsOf(page: Page) {
  return storeListOf(page).locator(":scope > ul > li");
}

test.describe("PLZ filter defaults and validation", () => {
  test("defaults to PLZ 73035 / 50 km with the in-stock-only toggle off and loads that availability", async ({
    page,
  }) => {
    const initial = page.waitForResponse((r) => isAvailability(r, "73035", 50));
    await page.goto("/");

    await expect(page.locator("#plz-filter-input")).toHaveValue("73035");
    await expect(page.locator("#radius-filter-input")).toHaveValue("50");
    await expect(page.getByText("50 km", { exact: true })).toBeVisible();
    await expect(
      page.getByRole("checkbox", { name: "Nur Märkte mit Verfügbarkeit anzeigen" }),
    ).not.toBeChecked();

    // The page fetched availability for the default filter.
    const res = await initial;
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.center).toMatchObject({ plz: "73035" });
    expect(body.radiusKm).toBe(50);

    // The list renders exactly the stores the API returned.
    await expect(storeItemsOf(page)).toHaveCount(body.stores.length);
    if (body.stores.length === 0) {
      // A center is active, so the "none in radius" copy (not "enter a PLZ").
      await expect(
        storeListOf(page).getByText("Keine Märkte im gewählten Umkreis gefunden."),
      ).toBeVisible();
    }
  });

  test("an invalid PLZ shows an error and does not query, a valid one updates the results", async ({
    page,
  }) => {
    await page.clock.install();
    const requested: string[] = [];
    page.on("request", (req) => {
      const u = new URL(req.url());
      if (u.pathname === "/api/availability") {
        requested.push(u.searchParams.get("plz") ?? "");
      }
    });

    const initial = page.waitForResponse((r) => isAvailability(r, "73035", 50));
    await page.goto("/");
    await initial;

    const plzInput = page.locator("#plz-filter-input");
    await plzInput.fill("123");

    const error = page.locator("#plz-filter-error");
    await expect(error).toBeVisible();
    await expect(error).toHaveText("Bitte eine gültige 5-stellige PLZ eingeben.");
    await expect(plzInput).toHaveAttribute("aria-invalid", "true");

    // Let the debounce elapse with the invalid value in the field.
    await page.clock.runFor(2_000);

    // Non-digits are stripped from the input. Typed key by key: a single fill
    // of "80a331" would be cut to five characters by maxlength first.
    await plzInput.fill("");
    await plzInput.pressSequentially("80a331");
    await expect(plzInput).toHaveValue("80331");
    await expect(error).toHaveCount(0);
    await expect(plzInput).toHaveAttribute("aria-invalid", "false");

    const updated = page.waitForResponse((r) => isAvailability(r, "80331", 50));
    await page.clock.runFor(2_000);
    const res = await updated;
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.center).toMatchObject({ plz: "80331" });

    // The invalid PLZ never reached the API; the valid one came after it.
    expect(requested).not.toContain("123");
    expect(requested).toContain("80331");
    await expect(storeItemsOf(page)).toHaveCount(body.stores.length);
  });

  test("the in-stock-only toggle hides stores without an in-stock listing", async ({
    page,
  }) => {
    const initial = page.waitForResponse((r) => isAvailability(r, "73035", 50));
    await page.goto("/");
    const body = await (await initial).json();

    const all: string[] = body.stores.map((s: any) => s.store.name);
    const inStock: string[] = body.stores
      .filter((s: any) => s.listings.some((l: any) => l.inStock))
      .map((s: any) => s.store.name);

    const items = storeItemsOf(page);
    await expect(items).toHaveCount(all.length);

    const toggle = page.getByRole("checkbox", {
      name: "Nur Märkte mit Verfügbarkeit anzeigen",
    });
    await toggle.check();
    await expect(toggle).toBeChecked();
    await expect(items).toHaveCount(inStock.length);
    for (const name of inStock) {
      await expect(items.filter({ hasText: name })).toHaveCount(1);
    }
    if (inStock.length === 0) {
      await expect(
        storeListOf(page).getByText("Keine Märkte im gewählten Umkreis gefunden."),
      ).toBeVisible();
    }

    // Unchecking restores the full result set without a new request.
    await toggle.uncheck();
    await expect(items).toHaveCount(all.length);
  });
});

test.describe("PLZ filter", () => {
  test("entering PLZ 80331 updates the store list to show the München store", async ({
    page,
    request,
  }) => {
    // Needs the mock source's snapshots, which only exist once a poller ran.
    const probe = await request.get(
      `${API_URL}/api/availability?plz=80331&radius=50`,
    );
    const probeBody = await probe.json();
    test.skip(
      !probeBody.stores?.some((s: any) => s.store.name === MUNICH),
      "environment: no mock availability snapshots (poller has not run)",
    );

    await page.goto("/");

    const storeList = storeListOf(page);
    await expect(storeList).toBeVisible({ timeout: 15_000 });

    // Enter the München PLZ (input is debounced by the frontend).
    const plzInput = page.locator("#plz-filter-input");
    await expect(plzInput).toBeVisible();
    await plzInput.fill("80331");

    // The München store appears...
    const munich = storeList.getByText(MUNICH, {
      exact: false,
    });
    await expect(munich).toBeVisible({ timeout: 15_000 });

    // ...with a distance now shown (radius filtering + distance ordering active).
    await expect(storeList.getByText(/\d+\s*km/).first()).toBeVisible();

    // Radius filtering: a far store (e.g. Hamburg, >50km) is filtered out.
    await expect(
      storeList.getByText("OBI Markt Hamburg-Altstadt", { exact: false }),
    ).toHaveCount(0);
  });
});
