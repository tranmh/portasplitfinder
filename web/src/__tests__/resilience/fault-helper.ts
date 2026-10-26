/**
 * Shared helpers for the resilience tests: the fault switch (AQAA_FAULT_URL)
 * and the web service's URL (AQAA_APP_WEB_URL). This file defines no test.
 */

export const FAULT_URL = (process.env.AQAA_FAULT_URL ?? "").replace(/\/+$/, "");
export const WEB_URL = (process.env.AQAA_APP_WEB_URL ?? "http://localhost:3000").replace(/\/+$/, "");

/** True when the fault proxy runs; the tests skip otherwise. */
export const hasFaults = FAULT_URL !== "";

/** Turns every fault off. */
export async function clearFaults(): Promise<void> {
  if (!hasFaults) return;
  await fetch(`${FAULT_URL}/faults`, { method: "DELETE" });
}

/** Turns one fault on for the given seconds. */
export async function injectFault(scenario: string, proxy: string, seconds: number): Promise<void> {
  const res = await fetch(
    `${FAULT_URL}/faults/${scenario}?proxy=${encodeURIComponent(proxy)}&seconds=${seconds}`,
    { method: "POST" },
  );
  const body = (await res.json().catch(() => ({}))) as { error?: unknown };
  if (!res.ok || (body.error !== undefined && body.error !== null && body.error !== "")) {
    throw new Error(`inject ${scenario} on ${proxy}: status ${res.status}, error ${String(body.error)}`);
  }
}

export interface Result {
  status: number;
  body: string;
  error: string | null;
}

/**
 * Sends a request to the web service, limited to `limitMs` (the fault's own
 * duration): an answer that has not come when the fault ends has hung for the
 * whole fault.
 */
export async function callWeb(path: string, limitMs: number): Promise<Result> {
  try {
    const res = await fetch(`${WEB_URL}${path}`, { signal: AbortSignal.timeout(limitMs) });
    return { status: res.status, body: await res.text(), error: null };
  } catch (err) {
    return { status: 0, body: "", error: String(err) };
  }
}
