import { test, expect, type APIRequestContext } from "@playwright/test";
import {
  clearMailhog,
  waitForMessage,
  extractToken,
  subjectOf,
} from "../helpers/mailhog";
import { resetStockBaseline, pollTransition } from "../helpers/poll";
import { MAILHOG_BASE_URL, WEB_BASE_URL } from "../../playwright.config";

const WEB_URL = process.env.AQAA_APP_WEB_URL ?? WEB_BASE_URL;

let uniq = 0;
/** Unique address per test so Mailhog lookups and de-dup never collide. */
function uniqueEmail(tag: string): string {
  uniq += 1;
  return `e2e-${tag}-${process.pid}-${Date.now()}-${uniq}@example.de`;
}

/** Whether a Mailhog instance answers (absent in some environments). */
async function mailhogAvailable(request: APIRequestContext): Promise<boolean> {
  try {
    const res = await request.get(`${MAILHOG_BASE_URL}/api/v2/messages?limit=1`, {
      timeout: 5_000,
    });
    return res.ok();
  } catch {
    return false;
  }
}

function linkToken(body: string, path: "confirm" | "unsubscribe"): string {
  const m = body.match(
    new RegExp(`/api/subscriptions/${path}\\?token=([A-Za-z0-9\\-_]+)`),
  );
  if (!m) throw new Error(`No ${path} link found in mail body`);
  return m[1];
}

test.describe("subscribe through the web UI", () => {
  test("the form is prefilled from the filter, validates client-side and submits to the API", async ({
    page,
  }) => {
    await page.goto(WEB_URL);

    const form = page
      .locator("section")
      .filter({
        has: page.getByRole("heading", { name: "Benachrichtigung bei Verfügbarkeit" }),
      });
    await expect(form).toBeVisible({ timeout: 15_000 });

    // Prefilled from the default filter (73035 / 50 km).
    await expect(page.locator("#subscribe-plz")).toHaveValue("73035");
    await expect(page.locator("#subscribe-radius")).toHaveValue("50");

    // Invalid email is rejected before any request.
    let posted = 0;
    page.on("request", (req) => {
      if (req.method() === "POST" && new URL(req.url()).pathname === "/api/subscriptions") {
        posted += 1;
      }
    });
    await page.locator("#subscribe-email").fill("not-an-email");
    await form.getByRole("button", { name: "Benachrichtigen lassen" }).click();
    await expect(page.locator("#subscribe-email-error")).toHaveText(
      "Bitte eine gültige E-Mail-Adresse eingeben.",
    );
    await expect(page.locator("#subscribe-email")).toHaveAttribute("aria-invalid", "true");
    expect(posted).toBe(0);

    // A valid submission posts to the API and shows the double opt-in notice.
    const email = uniqueEmail("ui");
    await page.locator("#subscribe-email").fill(email);
    const responsePromise = page.waitForResponse(
      (r) =>
        r.request().method() === "POST" &&
        new URL(r.url()).pathname === "/api/subscriptions",
    );
    await form.getByRole("button", { name: "Benachrichtigen lassen" }).click();
    const res = await responsePromise;
    expect(res.status()).toBe(202);
    expect(await res.json()).toEqual({ status: "pending_confirmation" });
    expect(res.request().postDataJSON()).toEqual({
      email,
      plz: "73035",
      radiusKm: 50,
      productId: null,
    });

    await expect(
      page.getByRole("heading", { name: "Bestätigungs-E-Mail gesendet" }),
    ).toBeVisible();
    await expect(
      page.getByText("Bitte bestätigen Sie Ihre Anmeldung über den Link in der E-Mail."),
    ).toBeVisible();
  });
});

test.describe("subscription API", () => {
  test("validates input server-side and rejects unknown tokens", async ({ request }) => {
    const email = uniqueEmail("api");

    const cases: Array<{ data: Record<string, unknown>; field: string }> = [
      { data: { email: "nope", plz: "80331", radiusKm: 50 }, field: "email" },
      { data: { email, plz: "8033", radiusKm: 50 }, field: "plz" },
      { data: { email, plz: "80331", radiusKm: 0 }, field: "radiusKm" },
      { data: { email, plz: "80331", radiusKm: 501 }, field: "radiusKm" },
    ];
    for (const c of cases) {
      const res = await request.post("/api/subscriptions", { data: c.data });
      expect(res.status(), JSON.stringify(c.data)).toBe(400);
      const body = await res.json();
      expect(body.error).toBe("validation_error");
      expect(body.field).toBe(c.field);
    }

    // radiusKm is optional (defaults to 50).
    const ok = await request.post("/api/subscriptions", {
      data: { email, plz: "80331" },
    });
    expect(ok.status()).toBe(202);
    expect(await ok.json()).toEqual({ status: "pending_confirmation" });

    // Re-subscribing the same address is still 202 (no enumeration).
    const again = await request.post("/api/subscriptions", {
      data: { email, plz: "80331", radiusKm: 50 },
    });
    expect(again.status()).toBe(202);

    for (const path of ["confirm", "unsubscribe"]) {
      const missing = await request.get(`/api/subscriptions/${path}`);
      expect(missing.status()).toBe(400);
      expect((await missing.json()).error).toBe("invalid_token");

      const unknown = await request.get(
        `/api/subscriptions/${path}?token=does-not-exist-${Date.now()}`,
      );
      expect([400, 404]).toContain(unknown.status());
      expect((await unknown.json()).error).toBe("invalid_token");
    }
  });
});

test.describe("subscribe -> confirm -> unsubscribe", () => {
  test("double opt-in mail links confirm and unsubscribe the subscription", async ({
    request,
  }) => {
    test.skip(
      !(await mailhogAvailable(request)),
      `environment: no mail catcher at ${MAILHOG_BASE_URL}`,
    );
    const email = uniqueEmail("unsub");

    const sub = await request.post("/api/subscriptions", {
      data: { email, plz: "80331", radiusKm: 50 },
    });
    expect(sub.status()).toBe(202);

    const mail = await waitForMessage(email);
    const confirmToken = linkToken(mail.Content.Body, "confirm");
    const unsubToken = linkToken(mail.Content.Body, "unsubscribe");

    const confirm = await request.get(
      `/api/subscriptions/confirm?token=${confirmToken}`,
    );
    expect(confirm.status()).toBe(200);
    expect(await confirm.json()).toMatchObject({ status: "confirmed" });

    const unsub = await request.get(
      `/api/subscriptions/unsubscribe?token=${unsubToken}`,
    );
    expect(unsub.status()).toBe(200);
    expect(await unsub.json()).toEqual({ status: "unsubscribed" });
  });
});

// This is the full notification flow and drives the shared poller, so give it
// extra room.
test.describe("subscribe -> confirm -> availability email", () => {
  test.setTimeout(120_000);

  test("double opt-in then in-stock transition delivers an availability mail", async ({
    request,
  }) => {
    // Needs Mailhog and the docker-compose poller (B35: München OBI listing
    // out-of-stock by default, in-stock with MOCK_TRANSITION=1).
    test.skip(
      !(await mailhogAvailable(request)),
      `environment: no mail catcher at ${MAILHOG_BASE_URL}`,
    );

    // Unique email per run keeps Mailhog assertions and notification de-dup
    // deterministic.
    const email = uniqueEmail("notify");

    await clearMailhog();

    // 1) Subscribe -> 202 pending_confirmation.
    const sub = await request.post("/api/subscriptions", {
      data: { email, plz: "80331", radiusKm: 50 },
    });
    expect(sub.status()).toBe(202);
    expect((await sub.json()).status).toBe("pending_confirmation");

    // 2) Confirmation mail arrives (German double opt-in).
    const confirmMail = await waitForMessage(email);
    expect(subjectOf(confirmMail)).toContain(
      "Bitte bestätige deine Anmeldung bei PortaSplitFinder",
    );
    const token = extractToken(confirmMail);

    // 3) Confirm the subscription.
    const confirm = await request.get(
      `/api/subscriptions/confirm?token=${token}`,
    );
    expect(confirm.status()).toBe(200);
    const confirmBody = await confirm.json();
    expect(confirmBody.status).toBe("confirmed");
    expect(confirmBody.email.toLowerCase()).toBe(email.toLowerCase());

    // 4) Make the München listing transition out->in-stock so the poller fires
    //    the availability notification. Reset to out-of-stock first so the
    //    transition is guaranteed, then poll with the transition flag.
    await resetStockBaseline();
    const sent = await pollTransition();
    expect(sent).toBeGreaterThanOrEqual(1);

    // 5) The availability mail arrives for this confirmed subscriber.
    const availMail = await waitForMessage(email, {
      subjectIncludes: "Midea Portasplit MAW-",
      timeoutMs: 30_000,
    });
    expect(subjectOf(availMail)).toContain("Midea Portasplit MAW-");
    expect(subjectOf(availMail)).toContain("PortaSplitFinder");
  });
});
