import { expect, test, type Page, type Request } from "@playwright/test";

/**
 * Validation feedback in the PLZ filter and the subscribe form: client-side
 * checks block bad input before any request, and server-side validation
 * errors are surfaced on the matching field.
 */

const API_URL = (process.env.AQAA_APP_API_URL ?? "http://localhost:8080").replace(/\/$/, "");

const PLZ_INVALID = "Bitte eine gültige 5-stellige PLZ eingeben.";

async function openHome(page: Page) {
  const first = page.waitForResponse(
    (r) => new URL(r.url()).pathname.endsWith("/api/availability") && r.request().method() === "GET",
  );
  await page.goto("/");
  await first;
}

function section(page: Page, heading: string) {
  return page.locator("section").filter({ has: page.getByRole("heading", { name: heading }) });
}

function isSubscribePost(req: Request): boolean {
  return req.method() === "POST" && new URL(req.url()).pathname.endsWith("/api/subscriptions");
}

test.describe("validation messages", () => {
  test("PLZ filter flags an incomplete PLZ and strips non-digits", async ({ page }) => {
    await openHome(page);

    const requested: string[] = [];
    page.on("request", (req) => {
      const url = new URL(req.url());
      if (url.pathname.endsWith("/api/availability")) requested.push(url.searchParams.get("plz") ?? "");
    });

    const filter = section(page, "Standort & Umkreis");
    const plz = page.getByLabel("Postleitzahl (PLZ)", { exact: true });

    await plz.fill("123");
    await expect(filter.getByRole("alert")).toHaveText(PLZ_INVALID);
    await expect(plz).toHaveAttribute("aria-invalid", "true");

    await plz.fill("12ab");
    await expect(plz).toHaveValue("12");
    await expect(filter.getByRole("alert")).toHaveText(PLZ_INVALID);

    const valid = page.waitForResponse(
      (r) => r.url().includes("/api/availability") && new URL(r.url()).searchParams.get("plz") === "80331",
    );
    await plz.fill("80331");
    await valid;
    await expect(filter.getByRole("alert")).toHaveCount(0);
    await expect(plz).toHaveAttribute("aria-invalid", "false");

    // Incomplete PLZs never reach the API.
    expect(requested).not.toContain("123");
    expect(requested).not.toContain("12");
  });

  test("PLZ unknown to the server shows the error state with retry", async ({ page, request }) => {
    const api = await request.get(`${API_URL}/api/availability?plz=00000&radius=50`);
    expect(api.status()).toBe(400);
    expect(await api.json()).toMatchObject({ error: "validation_error", field: "plz" });

    await openHome(page);
    const failed = page.waitForResponse(
      (r) => r.url().includes("/api/availability") && new URL(r.url()).searchParams.get("plz") === "00000",
    );
    await page.getByLabel("Postleitzahl (PLZ)", { exact: true }).fill("00000");
    expect((await failed).status()).toBe(400);
    await expect(page.getByText("Daten konnten nicht geladen werden.")).toBeVisible();
    await expect(page.getByRole("button", { name: "Erneut versuchen" })).toBeVisible();

    const recovered = page.waitForResponse(
      (r) => r.url().includes("/api/availability") && new URL(r.url()).searchParams.get("plz") === "80331",
    );
    await page.getByLabel("Postleitzahl (PLZ)", { exact: true }).fill("80331");
    expect((await recovered).status()).toBe(200);
    await expect(page.getByText("Daten konnten nicht geladen werden.")).toHaveCount(0);
    await expect(page.getByRole("heading", { name: "Märkte in der Nähe" })).toBeVisible();
  });

  test("subscribe form shows client-side errors and sends nothing", async ({ page }) => {
    await openHome(page);

    let posts = 0;
    page.on("request", (req) => {
      if (isSubscribePost(req)) posts++;
    });

    const form = section(page, "Benachrichtigung bei Verfügbarkeit");
    const email = form.getByLabel("E-Mail-Adresse");
    const plz = form.getByLabel("PLZ", { exact: true });
    const radius = form.getByLabel("Umkreis (km)");

    // The form is prefilled from the active filter.
    await expect(plz).toHaveValue("73035");
    await expect(radius).toHaveValue("50");

    await email.fill("not-an-email");
    await plz.fill("123");
    await radius.fill("0");
    await form.getByRole("button", { name: "Benachrichtigen lassen" }).click();

    await expect(form.locator("#subscribe-email-error")).toHaveText(
      "Bitte eine gültige E-Mail-Adresse eingeben.",
    );
    await expect(form.locator("#subscribe-plz-error")).toHaveText(PLZ_INVALID);
    await expect(form.locator("#subscribe-radius-error")).toHaveText(
      "Umkreis muss zwischen 1 und 500 km liegen.",
    );
    await expect(email).toHaveAttribute("aria-invalid", "true");
    await expect(plz).toHaveAttribute("aria-invalid", "true");
    await expect(radius).toHaveAttribute("aria-invalid", "true");

    // Fixing the radius clears only that error on the next submit.
    await radius.fill("501");
    await form.getByRole("button", { name: "Benachrichtigen lassen" }).click();
    await expect(form.locator("#subscribe-radius-error")).toBeVisible();
    await radius.fill("500");
    await form.getByRole("button", { name: "Benachrichtigen lassen" }).click();
    await expect(form.locator("#subscribe-radius-error")).toHaveCount(0);
    await expect(form.locator("#subscribe-email-error")).toBeVisible();
    await expect(form.locator("#subscribe-plz-error")).toBeVisible();

    expect(posts).toBe(0);
  });

  test("subscribe form surfaces the server's email validation", async ({ page }) => {
    await openHome(page);

    const form = section(page, "Benachrichtigung bei Verfügbarkeit");
    // Passes the pragmatic client check (x@y.z) but not the server's
    // RFC 5322 single-address parser.
    await form.getByLabel("E-Mail-Adresse").fill("aqaa-e2e@example.com>");
    await form.getByLabel("PLZ", { exact: true }).fill("73035");
    await form.getByLabel("Umkreis (km)").fill("50");

    const posted = page.waitForResponse((r) => isSubscribePost(r.request()));
    await form.getByRole("button", { name: "Benachrichtigen lassen" }).click();
    const res = await posted;
    expect(res.status()).toBe(400);
    expect(await res.json()).toMatchObject({ error: "validation_error", field: "email" });

    await expect(form.locator("#subscribe-email-error")).toHaveText("invalid email");
    await expect(form.getByLabel("E-Mail-Adresse")).toHaveAttribute("aria-invalid", "true");
    await expect(page.getByText("Bestätigungs-E-Mail gesendet")).toHaveCount(0);
  });
});
