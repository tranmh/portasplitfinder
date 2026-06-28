import { test, expect } from "@playwright/test";

test.describe("map page", () => {
  test("renders the Leaflet map and the online retailers panel", async ({
    page,
  }) => {
    await page.goto("/");

    // App header confirms the page hydrated.
    await expect(
      page.getByRole("heading", { name: "PortaSplitFinder", level: 1 }),
    ).toBeVisible();

    // Leaflet map container is rendered (client-side react-leaflet).
    const map = page.locator(".leaflet-container");
    await expect(map).toBeVisible({ timeout: 15_000 });
    // Map tiles load -> the tile pane has tile images.
    await expect(page.locator(".leaflet-tile").first()).toBeVisible({
      timeout: 15_000,
    });

    // Online panel shows nationwide retailers.
    const onlinePanel = page
      .locator("section")
      .filter({ has: page.getByRole("heading", { name: "Online-Händler" }) });
    await expect(onlinePanel).toBeVisible();
    await expect(onlinePanel.getByText("deutschlandweit verfügbar").first()).toBeVisible();
    // At least one known online retailer is listed.
    await expect(onlinePanel.getByText("Amazon", { exact: true })).toBeVisible();
  });
});
