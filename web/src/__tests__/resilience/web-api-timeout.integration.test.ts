// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { callWeb, clearFaults, hasFaults, injectFault } from "./fault-helper";

// A stalled api connection must not make the web server hang: it serves the
// page and /api/health without calling the api, so both answer before the
// fault ends.
const FAULT_SECONDS = 10;

describe.skipIf(!hasFaults)("resilience: web with a stalled api", () => {
  beforeEach(clearFaults);
  afterEach(clearFaults);

  it(
    "web answers /api/health and the page before an app-api timeout fault ends",
    async () => {
      await injectFault("timeout", "app-api", FAULT_SECONDS);

      const health = await callWeb("/api/health", FAULT_SECONDS * 1000);
      expect(health.error).toBeNull();
      expect(health.status).toBe(200);

      const page = await callWeb("/", FAULT_SECONDS * 1000);
      expect(page.error).toBeNull();
      expect(page.status).toBe(200);

      await clearFaults();
      const after = await callWeb("/", FAULT_SECONDS * 1000);
      expect(after.status).toBe(200);
    },
    FAULT_SECONDS * 4 * 1000,
  );
});
