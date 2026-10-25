import { test, expect, type Response } from "@playwright/test";

test.describe("map page", () => {
  test("renders the Leaflet map and the online retailers panel", async ({
    page,
  }) => {
    const initial = page.waitForResponse((r) => isDefaultAvailability(r));
    await page.goto("/");

    // App header confirms the page hydrated.
    await expect(
      page.getByRole("heading", { name: "PortaSplitFinder", level: 1 }),
    ).toBeVisible();

    // Leaflet map container is rendered (client-side react-leaflet).
    const map = page.locator(".leaflet-container");
    await expect(map).toBeVisible({ timeout: 15_000 });
    // The tile layer requests OSM tiles. Whether they load depends on outbound
    // network, so only assert the tile elements exist.
    await expect(page.locator("img.leaflet-tile").first()).toBeAttached({
      timeout: 15_000,
    });

    // Online panel shows nationwide retailers.
    const onlinePanel = page
      .locator("section")
      .filter({ has: page.getByRole("heading", { name: "Online-Händler" }) });
    await expect(onlinePanel).toBeVisible();
    await expect(onlinePanel.getByText("deutschlandweit verfügbar").first()).toBeVisible();

    // The panel lists exactly the online sources the API returned.
    const body = await (await initial).json();
    const onlineItems = onlinePanel.locator(":scope > ul > li");
    await expect(onlineItems).toHaveCount(body.online.length);
    for (const o of body.online) {
      await expect(
        onlineItems.getByText(o.source.name, { exact: true }),
      ).toBeVisible();
    }
    if (body.online.length === 0) {
      await expect(
        onlinePanel.getByText("Derzeit keine Online-Angebote."),
      ).toBeVisible();
    }
  });
});

/** True for the page's initial GET /api/availability?plz=73035&radius=50. */
function isDefaultAvailability(res: Response): boolean {
  const u = new URL(res.url());
  return (
    res.request().method() === "GET" &&
    u.pathname === "/api/availability" &&
    u.searchParams.get("plz") === "73035" &&
    u.searchParams.get("radius") === "50"
  );
}

test.describe("map and list view", () => {
  test("markers, radius circle and the store list match the API availability response", async ({
    page,
  }) => {
    const initial = page.waitForResponse((r) => isDefaultAvailability(r));
    await page.goto("/");
    const res = await initial;
    expect(res.status()).toBe(200);
    const body = await res.json();

    await expect(page.locator(".leaflet-container")).toBeVisible({
      timeout: 15_000,
    });

    // One red center marker for the PLZ and one circle for the radius.
    await expect(page.locator(".leaflet-marker-icon.psf-center-marker")).toHaveCount(1);
    await expect(page.locator("path.leaflet-interactive")).toHaveCount(1);

    // One store marker per returned store with coordinates.
    const located = body.stores.filter(
      (s: any) => Number.isFinite(s.store.lat) && Number.isFinite(s.store.lon),
    );
    await expect(
      page.locator(".leaflet-marker-icon:not(.psf-center-marker)"),
    ).toHaveCount(located.length);

    // The list shows the same stores, nearest first (API order).
    const names = page
      .locator("section")
      .filter({ has: page.getByRole("heading", { name: "Märkte in der Nähe" }) })
      .locator(":scope > ul > li > div > div > p:first-child");
    await expect(names).toHaveText(body.stores.map((s: any) => s.store.name));

    // A marker popup shows its store name and stock state.
    if (located.length > 0) {
      const first = located[0];
      const markers = page.locator(".leaflet-marker-icon:not(.psf-center-marker)");
      const count = await markers.count();
      let opened = false;
      for (let i = 0; i < count && !opened; i++) {
        await markers.nth(i).click({ force: true });
        const popup = page.locator(".leaflet-popup-content");
        await expect(popup).toBeVisible();
        opened = (await popup.textContent())?.includes(first.store.name) ?? false;
      }
      expect(opened, `no marker popup for ${first.store.name}`).toBe(true);
      const anyInStock = first.listings.some((l: any) => l.inStock);
      await expect(page.locator(".leaflet-popup-content")).toContainText(
        anyInStock ? "Auf Lager" : "Nicht verfügbar",
      );
    }
  });

  test("changing the PLZ recenters the map on the new center", async ({ page }) => {
    const initial = page.waitForResponse((r) => isDefaultAvailability(r));
    await page.goto("/");
    await initial;
    await expect(page.locator(".leaflet-marker-icon.psf-center-marker")).toHaveCount(1);

    const updated = page.waitForResponse((r) => {
      const u = new URL(r.url());
      return u.pathname === "/api/availability" && u.searchParams.get("plz") === "10115";
    });
    await page.locator("#plz-filter-input").fill("10115");
    const body = await (await updated).json();
    expect(body.center).toMatchObject({ plz: "10115" });

    // Still exactly one center marker and circle, and the marker count follows
    // the new response.
    await expect(page.locator(".leaflet-marker-icon.psf-center-marker")).toHaveCount(1);
    await expect(page.locator("path.leaflet-interactive")).toHaveCount(1);
    await expect(
      page.locator(".leaflet-marker-icon:not(.psf-center-marker)"),
    ).toHaveCount(body.stores.length);

    // The center marker's popup marks the user's location.
    await page.locator(".leaflet-marker-icon.psf-center-marker").click({ force: true });
    await expect(page.locator(".leaflet-popup-content")).toHaveText("Ihr Standort");
  });
});
