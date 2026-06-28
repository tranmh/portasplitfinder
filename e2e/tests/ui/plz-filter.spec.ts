import { test, expect } from "@playwright/test";

test.describe("PLZ filter", () => {
  test("entering PLZ 80331 updates the store list to show the München store", async ({
    page,
  }) => {
    await page.goto("/");

    const storeList = page
      .locator("section")
      .filter({ has: page.getByRole("heading", { name: "Märkte in der Nähe" }) });
    await expect(storeList).toBeVisible({ timeout: 15_000 });

    // Without a PLZ, no distance is shown for any store (distanceKm is null).
    await expect(storeList.getByText(/\d+\s*km/)).toHaveCount(0);

    // Enter the München PLZ (input is debounced by the frontend).
    const plzInput = page.locator("#plz-filter-input");
    await expect(plzInput).toBeVisible();
    await plzInput.fill("80331");

    // The München store appears...
    const munich = storeList.getByText("OBI Markt München-Zentrum", {
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
