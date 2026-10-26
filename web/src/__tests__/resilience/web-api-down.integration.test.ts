// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { callWeb, clearFaults, hasFaults, injectFault } from "./fault-helper";

// The web server renders the page and /api/health without calling the api
// (the browser calls it directly), so it must keep answering while the api is
// down and still answer normally once it is back.
const FAULT_SECONDS = 10;

describe.skipIf(!hasFaults)("resilience: web with the api down", () => {
  beforeEach(clearFaults);
  afterEach(clearFaults);

  it(
    "web keeps serving /api/health and the page while app-api is down",
    async () => {
      await injectFault("down", "app-api", FAULT_SECONDS);

      const health = await callWeb("/api/health", FAULT_SECONDS * 1000);
      expect(health.error).toBeNull();
      expect(health.status).toBe(200);
      expect(JSON.parse(health.body)).toMatchObject({ status: "ok" });

      const page = await callWeb("/", FAULT_SECONDS * 1000);
      expect(page.error).toBeNull();
      expect(page.status).toBe(200);

      await clearFaults();
      const after = await callWeb("/api/health", FAULT_SECONDS * 1000);
      expect(after.status).toBe(200);
    },
    FAULT_SECONDS * 4 * 1000,
  );
});
