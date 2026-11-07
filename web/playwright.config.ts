import { defineConfig, devices } from "@playwright/test";

/**
 * E2E specs run against an already running stack (web on :3000, API on :8080).
 * No webServer here: the environment starts the apps.
 */
export default defineConfig({
  testDir: "./e2e",
  testMatch: "**/*.spec.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 15_000 },
  reporter: "list",
  use: {
    baseURL: process.env.AQAA_BASE_URL ?? process.env.AQAA_APP_WEB_URL ?? "http://localhost:3000",
    locale: "de-DE",
    timezoneId: "Europe/Berlin",
    trace: "retain-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
});
