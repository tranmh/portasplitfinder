// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { callWeb, clearFaults, hasFaults, injectFault } from "./fault-helper";

// The web server has no server-side database access, so its health check and
// page must not fail while postgres is down.
const FAULT_SECONDS = 10;

describe.skipIf(!hasFaults)("resilience: web with postgres down", () => {
  beforeEach(clearFaults);
  afterEach(clearFaults);

  it(
    "web keeps serving /api/health and the page while postgres is down",
    async () => {
      await injectFault("down", "postgres", FAULT_SECONDS);

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
