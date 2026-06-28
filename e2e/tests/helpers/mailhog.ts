import { APIRequestContext, expect, request } from "@playwright/test";
import { MAILHOG_BASE_URL } from "../../playwright.config";

export interface MailhogMessage {
  Content: {
    Headers: Record<string, string[]>;
    Body: string;
  };
}

interface MailhogList {
  total: number;
  count: number;
  items: MailhogMessage[];
}

/** Standalone request context so helpers work from any project (ui or api). */
async function mhContext(): Promise<APIRequestContext> {
  return request.newContext({ baseURL: MAILHOG_BASE_URL });
}

/** Delete every message in Mailhog (use in test setup for determinism). */
export async function clearMailhog(): Promise<void> {
  const ctx = await mhContext();
  try {
    const res = await ctx.delete("/api/v1/messages");
    expect(res.ok(), `clear mailhog failed: ${res.status()}`).toBeTruthy();
  } finally {
    await ctx.dispose();
  }
}

async function listMessages(): Promise<MailhogMessage[]> {
  const ctx = await mhContext();
  try {
    const res = await ctx.get("/api/v2/messages");
    expect(res.ok(), `list mailhog failed: ${res.status()}`).toBeTruthy();
    const body = (await res.json()) as MailhogList;
    return body.items ?? [];
  } finally {
    await ctx.dispose();
  }
}

function recipients(msg: MailhogMessage): string[] {
  const to = msg.Content.Headers["To"] ?? [];
  // "To" headers may bundle several addresses comma-separated.
  return to.flatMap((h) => h.split(",").map((a) => a.trim().toLowerCase()));
}

/** Decode an RFC 2047 ("=?UTF-8?B?...?=") encoded header into plain text. */
export function decodeSubject(raw: string | undefined): string {
  if (!raw) return "";
  const m = raw.match(/=\?UTF-8\?B\?(.*?)\?=/i);
  if (m) {
    return Buffer.from(m[1], "base64").toString("utf-8");
  }
  return raw;
}

export function subjectOf(msg: MailhogMessage): string {
  return decodeSubject(msg.Content.Headers["Subject"]?.[0]);
}

/**
 * Poll Mailhog until a message addressed to `email` (optionally whose decoded
 * subject contains `subjectIncludes`) appears. Throws on timeout.
 */
export async function waitForMessage(
  email: string,
  opts: { subjectIncludes?: string; timeoutMs?: number; intervalMs?: number } = {},
): Promise<MailhogMessage> {
  const target = email.toLowerCase();
  const timeoutMs = opts.timeoutMs ?? 20_000;
  const intervalMs = opts.intervalMs ?? 500;
  const deadline = Date.now() + timeoutMs;

  let lastSeen = 0;
  while (Date.now() < deadline) {
    const items = await listMessages();
    lastSeen = items.length;
    const match = items.find((m) => {
      if (!recipients(m).includes(target)) return false;
      if (opts.subjectIncludes) {
        return subjectOf(m).includes(opts.subjectIncludes);
      }
      return true;
    });
    if (match) return match;
    await new Promise((r) => setTimeout(r, intervalMs));
  }
  throw new Error(
    `Timed out after ${timeoutMs}ms waiting for mail to ${email}` +
      (opts.subjectIncludes ? ` with subject ~ "${opts.subjectIncludes}"` : "") +
      ` (Mailhog held ${lastSeen} message(s))`,
  );
}

/** Extract the first `token=<value>` found in the message body. */
export function extractToken(msg: MailhogMessage): string {
  const body = msg.Content.Body ?? "";
  const m = body.match(/token=([A-Za-z0-9\-_=]+)/);
  if (!m) {
    throw new Error("No token= found in mail body");
  }
  // Strip any trailing '=' padding that might be a URL artifact only if it is
  // clearly padding; tokens here are URL-safe base64 without '=' separators.
  return m[1];
}
