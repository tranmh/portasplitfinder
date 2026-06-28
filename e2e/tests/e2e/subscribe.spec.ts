import { test, expect } from "@playwright/test";
import {
  clearMailhog,
  waitForMessage,
  extractToken,
  subjectOf,
} from "../helpers/mailhog";
import { resetStockBaseline, pollTransition } from "../helpers/poll";

// This is the full notification flow and drives the shared poller, so give it
// extra room.
test.describe("subscribe -> confirm -> availability email", () => {
  test.setTimeout(120_000);

  test("double opt-in then in-stock transition delivers an availability mail", async ({
    request,
  }) => {
    // Unique email per run keeps Mailhog assertions and notification de-dup
    // deterministic.
    const email = `e2e-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.de`;

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
