import { expect, test, type APIRequestContext } from "@playwright/test";
import { Client } from "pg";

/**
 * Double opt-in: subscribe in the UI, confirm via the link from the
 * confirmation mail, then unsubscribe. The links come from MailHog when it is
 * reachable (MAILHOG_BASE_URL / AQAA_APP_MAILHOG_URL); otherwise the tokens are
 * read from the API's database (DATABASE_URL), which holds exactly what the
 * mail would carry. Without either the test is skipped.
 */

const API_URL = (process.env.AQAA_APP_API_URL ?? "http://localhost:8080").replace(/\/$/, "");
const MAILHOG_URL = (process.env.MAILHOG_BASE_URL ?? process.env.AQAA_APP_MAILHOG_URL ?? "").replace(
  /\/$/,
  "",
);
const DATABASE_URL = process.env.DATABASE_URL ?? process.env.AQAA_PG_URL ?? "";

interface Links {
  confirmUrl: string;
  unsubUrl: string;
}

async function mailhogAvailable(request: APIRequestContext): Promise<boolean> {
  if (!MAILHOG_URL) return false;
  try {
    const res = await request.get(`${MAILHOG_URL}/api/v2/messages?limit=1`, { timeout: 5_000 });
    return res.ok();
  } catch {
    return false;
  }
}

/** Newest confirmation mail for the address, polled until it arrives. */
async function linksFromMailhog(request: APIRequestContext, email: string): Promise<Links> {
  let links: Links | null = null;
  await expect
    .poll(
      async () => {
        const res = await request.get(
          `${MAILHOG_URL}/api/v2/search?kind=to&query=${encodeURIComponent(email)}`,
        );
        const body = (await res.json()) as { items: { Content: { Body: string } }[] };
        const raw = (body.items[0]?.Content.Body ?? "").replace(/=\r?\n/g, "").replace(/=3D/g, "=");
        const confirm = raw.match(/https?:\/\/[^\s"<>]+\/api\/subscriptions\/confirm\?token=[\w-]+/);
        const unsub = raw.match(/https?:\/\/[^\s"<>]+\/api\/subscriptions\/unsubscribe\?token=[\w-]+/);
        links = confirm && unsub ? { confirmUrl: confirm[0], unsubUrl: unsub[0] } : null;
        return links !== null;
      },
      { timeout: 15_000 },
    )
    .toBe(true);
  return links!;
}

async function withDb<T>(fn: (db: Client) => Promise<T>): Promise<T> {
  const db = new Client({ connectionString: DATABASE_URL });
  await db.connect();
  try {
    return await fn(db);
  } finally {
    await db.end();
  }
}

async function dbAvailable(): Promise<boolean> {
  if (!DATABASE_URL) return false;
  try {
    return await withDb(async (db) => {
      const res = await db.query("SELECT to_regclass('public.subscriptions') AS t");
      return res.rows[0]?.t !== null;
    });
  } catch {
    return false;
  }
}

interface SubscriptionRow {
  confirm_token: string;
  unsub_token: string;
  plz: string;
  radius_km: number;
  confirmed_at: Date | null;
  unsubbed_at: Date | null;
}

async function latestSubscription(email: string): Promise<SubscriptionRow | undefined> {
  return withDb(async (db) => {
    const res = await db.query<SubscriptionRow>(
      `SELECT confirm_token, unsub_token, plz, radius_km, confirmed_at, unsubbed_at
         FROM subscriptions WHERE email = $1 ORDER BY created_at DESC LIMIT 1`,
      [email],
    );
    return res.rows[0];
  });
}

test("subscribe, confirm via the mailed link, unsubscribe", async ({ page, request }, testInfo) => {
  const viaMailhog = await mailhogAvailable(request);
  const viaDb = !viaMailhog && (await dbAvailable());
  test.skip(!viaMailhog && !viaDb, "neither MailHog nor the API database is reachable");

  // One fixed address per browser project; the newest row / mail is this run's.
  const email = `aqaa-e2e-${testInfo.project.name}@example.com`;
  const before = viaDb ? await latestSubscription(email) : undefined;

  const first = page.waitForResponse((r) => new URL(r.url()).pathname.endsWith("/api/availability"));
  await page.goto("/");
  await first;

  const form = page
    .locator("section")
    .filter({ has: page.getByRole("heading", { name: "Benachrichtigung bei Verfügbarkeit" }) });
  await form.getByLabel("E-Mail-Adresse").fill(email);
  await form.getByLabel("PLZ", { exact: true }).fill("80331");
  await form.getByLabel("Umkreis (km)").fill("120");

  const posted = page.waitForResponse(
    (r) => r.request().method() === "POST" && new URL(r.url()).pathname.endsWith("/api/subscriptions"),
  );
  await form.getByRole("button", { name: "Benachrichtigen lassen" }).click();
  const res = await posted;
  expect(res.status()).toBe(202);
  expect(await res.json()).toEqual({ status: "pending_confirmation" });
  expect(JSON.parse(res.request().postData() ?? "{}")).toEqual({
    email,
    plz: "80331",
    radiusKm: 120,
    productId: null,
  });

  await expect(page.getByRole("heading", { name: "Bestätigungs-E-Mail gesendet" })).toBeVisible();
  await expect(
    page.getByText("Bitte bestätigen Sie Ihre Anmeldung über den Link in der E-Mail."),
  ).toBeVisible();

  let links: Links;
  if (viaMailhog) {
    links = await linksFromMailhog(request, email);
  } else {
    const row = await latestSubscription(email);
    expect(row).toBeDefined();
    expect(row!.confirm_token).not.toBe(before?.confirm_token);
    expect(row!.plz).toBe("80331");
    expect(row!.radius_km).toBe(120);
    expect(row!.confirmed_at).toBeNull();
    expect(row!.unsubbed_at).toBeNull();
    links = {
      confirmUrl: `${API_URL}/api/subscriptions/confirm?token=${encodeURIComponent(row!.confirm_token)}`,
      unsubUrl: `${API_URL}/api/subscriptions/unsubscribe?token=${encodeURIComponent(row!.unsub_token)}`,
    };
  }

  const confirmed = await request.get(links.confirmUrl);
  expect(confirmed.status()).toBe(200);
  expect(await confirmed.json()).toEqual({ status: "confirmed", email });

  // Confirming twice is idempotent.
  const again = await request.get(links.confirmUrl);
  expect(again.status()).toBe(200);
  expect(await again.json()).toEqual({ status: "confirmed", email });

  const unsubscribed = await request.get(links.unsubUrl);
  expect(unsubscribed.status()).toBe(200);
  expect(await unsubscribed.json()).toEqual({ status: "unsubscribed" });

  if (viaDb) {
    const row = await latestSubscription(email);
    expect(row!.confirmed_at).not.toBeNull();
    expect(row!.unsubbed_at).not.toBeNull();
  }

  // A token that was never issued is rejected.
  const bogus = await request.get(`${API_URL}/api/subscriptions/confirm?token=aqaa-e2e-unknown-token`);
  expect(bogus.status()).toBe(404);
  expect(await bogus.json()).toMatchObject({ error: "invalid_token", field: "token" });
});
