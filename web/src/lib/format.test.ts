import { describe, it, expect } from "vitest";
import {
  formatDateTime,
  formatDistanceKm,
  formatPrice,
  isValidEmail,
  isValidPlz,
  isValidRadius,
} from "./format";

// Intl uses (narrow) no-break spaces between amount and unit; normalise them.
const norm = (s: string | null) => (s === null ? null : s.replace(/[  ]/g, " "));

describe("isValidPlz", () => {
  it.each(["73035", "10115", "00000", "99999", " 73035 "])("accepts %j", (plz) => {
    expect(isValidPlz(plz)).toBe(true);
  });

  it.each(["", "     ", "7303", "730355", "7303a", "73 035", "-7303", "７３０３５", "73035\n1"])(
    "rejects %j",
    (plz) => {
      expect(isValidPlz(plz)).toBe(false);
    },
  );
});

describe("isValidEmail", () => {
  it.each(["a@b.de", "max.mustermann@example.com", "  user@mail.example.org  "])("accepts %j", (email) => {
    expect(isValidEmail(email)).toBe(true);
  });

  it.each(["", "plain", "a@b", "@b.de", "a@.de", "a b@c.de", "a@b c.de", "a@@b.de"])("rejects %j", (email) => {
    expect(isValidEmail(email)).toBe(false);
  });
});

describe("isValidRadius", () => {
  it.each([1, 50, 499, 500])("accepts the in-range value %d", (r) => {
    expect(isValidRadius(r)).toBe(true);
  });

  it.each([0, 0.99, -1, 500.01, 501, Number.NaN, Number.POSITIVE_INFINITY, Number.NEGATIVE_INFINITY])(
    "rejects %d",
    (r) => {
      expect(isValidRadius(r)).toBe(false);
    },
  );
});

describe("formatPrice", () => {
  it("formats euro cents as a German currency string", () => {
    expect(norm(formatPrice(4990))).toBe("49,90 €");
  });

  it("formats a price of 0 as 0,00 € rather than treating it as unknown", () => {
    expect(norm(formatPrice(0))).toBe("0,00 €");
  });

  it("formats thousands with a group separator", () => {
    expect(norm(formatPrice(129999))).toBe("1.299,99 €");
  });

  it("formats a single cent", () => {
    expect(norm(formatPrice(1))).toBe("0,01 €");
  });

  it("returns null for an unknown price so callers can show 'Preis unbekannt'", () => {
    expect(formatPrice(null)).toBeNull();
    expect(formatPrice(undefined as unknown as null)).toBeNull();
  });

  it("honours currency and locale arguments", () => {
    expect(norm(formatPrice(4990, "EUR", "en-US"))).toBe("€49.90");
    expect(norm(formatPrice(1000, "USD", "en-US"))).toBe("$10.00");
  });

  it("throws for an invalid currency code", () => {
    expect(() => formatPrice(100, "NOT-A-CURRENCY")).toThrow(RangeError);
  });
});

describe("formatDistanceKm", () => {
  it("formats with at most one fractional digit and a km suffix", () => {
    expect(norm(formatDistanceKm(4.25))).toMatch(/^4,[23] km$/);
    expect(norm(formatDistanceKm(12.34))).toBe("12,3 km");
  });

  it("formats whole and zero distances without decimals", () => {
    expect(formatDistanceKm(5)).toBe("5 km");
    expect(formatDistanceKm(0)).toBe("0 km");
  });

  it("groups thousands in German locale", () => {
    expect(formatDistanceKm(1234.5)).toBe("1.234,5 km");
  });

  it("uses the given locale", () => {
    expect(formatDistanceKm(12.34, "en-US")).toBe("12.3 km");
  });

  it("returns null for an unknown distance", () => {
    expect(formatDistanceKm(null)).toBeNull();
    expect(formatDistanceKm(undefined as unknown as null)).toBeNull();
  });
});

describe("formatDateTime", () => {
  // Midday UTC keeps the calendar date stable for any test-machine time zone.
  const ISO = "2026-06-28T12:00:00Z";

  it("formats an ISO timestamp with German medium date and short time", () => {
    const out = formatDateTime(ISO);
    expect(out).not.toBeNull();
    expect(out).toContain("28.06.2026");
    expect(out).toMatch(/\d{2}:\d{2}$/);
  });

  it("uses the given locale", () => {
    const out = formatDateTime(ISO, "en-US");
    expect(out).toContain("Jun 28, 2026");
  });

  it.each([null, "", "not-a-date", "2026-13-45T99:99:99Z"])("returns null for %j", (iso) => {
    expect(formatDateTime(iso)).toBeNull();
  });
});
