import { test, expect } from "@playwright/test";

const MUNICH = "OBI Markt München-Zentrum";

function storeNames(body: any): string[] {
  return (body.stores ?? []).map((s: any) => s.store.name);
}

test.describe("availability", () => {
  test("plz 80331 / radius 50 returns stores and online sources", async ({
    request,
  }) => {
    const res = await request.get("/api/availability?plz=80331&radius=50");
    expect(res.status()).toBe(200);
    const body = await res.json();

    expect(body.center).toMatchObject({ plz: "80331" });
    expect(body.radiusKm).toBe(50);

    // The München store is within 50km of PLZ 80331.
    expect(storeNames(body)).toContain(MUNICH);

    // Online retailers are returned (amazon/ebay/guenstiger/idealo).
    expect(Array.isArray(body.online)).toBe(true);
    expect(body.online.length).toBeGreaterThanOrEqual(1);
    const onlineKeys = body.online.map((o: any) => o.source.key);
    expect(onlineKeys).toEqual(expect.arrayContaining(["amazon"]));
    for (const o of body.online) {
      expect(o.source.type).toBe("online");
    }

    // distanceKm is present (a number) for every store when a plz is given.
    for (const s of body.stores) {
      expect(typeof s.distanceKm).toBe("number");
    }
    const munich = body.stores.find((s: any) => s.store.name === MUNICH);
    expect(munich.distanceKm).toBe(0);
  });

  test("radius filtering: München is excluded for a far PLZ (20095 Hamburg)", async ({
    request,
  }) => {
    const near = await request.get("/api/availability?plz=80331&radius=50");
    const far = await request.get("/api/availability?plz=20095&radius=50");
    expect(near.status()).toBe(200);
    expect(far.status()).toBe(200);

    const nearBody = await near.json();
    const farBody = await far.json();

    // Proven radius filtering: München is in range for 80331 but filtered out
    // for the far Hamburg PLZ (>50km away).
    expect(storeNames(nearBody)).toContain(MUNICH);
    expect(storeNames(farBody)).not.toContain(MUNICH);
  });

  test("without a plz, distanceKm is null", async ({ request }) => {
    const res = await request.get("/api/availability");
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.center).toBeNull();
    for (const s of body.stores) {
      expect(s.distanceKm).toBeNull();
    }
  });
});
