import { defineConfig, devices } from "@playwright/test";

/**
 * Base URLs for the running docker-compose stack. Overridable via env so the
 * same suite can run against a remote/CI deployment.
 */
export const API_BASE_URL =
  process.env.API_BASE_URL ?? "http://localhost:8080";
export const WEB_BASE_URL =
  process.env.WEB_BASE_URL ?? "http://localhost:3000";
export const MAILHOG_BASE_URL =
  process.env.MAILHOG_BASE_URL ?? "http://localhost:8025";

export default defineConfig({
  testDir: "./tests",
  // The subscribe spec drives the shared poller + Mailhog, so keep the suite
  // serial to avoid cross-test interference on the single running stack.
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? "list" : [["list"], ["html", { open: "never" }]],
  timeout: 60_000,
  expect: { timeout: 10_000 },
  use: {
    trace: "on-first-retry",
  },
  projects: [
    {
      // Headless API/integration project: no browser, uses the `request`
      // fixture against the API and Mailhog.
      name: "api",
      testMatch: /tests\/(api|e2e)\/.*\.spec\.ts/,
      use: {
        baseURL: API_BASE_URL,
      },
    },
    {
      // Browser project against the Next.js web UI.
      name: "ui",
      testMatch: /tests\/ui\/.*\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: WEB_BASE_URL,
      },
    },
  ],
});
