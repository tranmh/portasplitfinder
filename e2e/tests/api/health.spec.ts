import { test, expect, request } from "@playwright/test";
import { WEB_BASE_URL } from "../../playwright.config";

test.describe("health", () => {
  test("GET /api/health returns 200 with status ok", async ({ request }) => {
    const res = await request.get("/api/health");
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.status).toBe("ok");
    expect(body.db).toBe("ok");
  });

  test("web root is reachable (200)", async () => {
    const ctx = await request.newContext({ baseURL: WEB_BASE_URL });
    try {
      const res = await ctx.get("/");
      expect(res.status()).toBe(200);
    } finally {
      await ctx.dispose();
    }
  });
});
