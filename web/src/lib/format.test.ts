import { describe, it, expect } from "vitest";
import {
  PLZ_REGEX,
  formatDateTime,
  formatDistanceKm,
  formatPrice,
  isValidEmail,
  isValidPlz,
  isValidRadius,
} from "./format";

// Intl separates the number and the unit with (narrow) no-break spaces;
// normalise them so the assertions read like the rendered text.
const plain = (s: string | null) => (s === null ? null : s.replace(/\s/g, " "));

describe("isValidPlz", () => {
  it.each(["73035", "10115", "00000", "99999"])("accepts the 5-digit PLZ %s", (plz) => {
    expect(isValidPlz(plz)).toBe(true);
  });

  it("trims surrounding whitespace before checking", () => {
    expect(isValidPlz("  73035 ")).toBe(true);
    expect(isValidPlz("\t10115\n")).toBe(true);
  });

  it.each([
    ["empty", ""],
    ["blank", "   "],
    ["4 digits", "7303"],
    ["6 digits", "730351"],
    ["letter", "7303a"],
    ["inner space", "73 035"],
    ["sign", "-7303"],
    ["non-ASCII digits", "٧٣٠٣٥"],
  ])("rejects %s", (_name, plz) => {
    expect(isValidPlz(plz)).toBe(false);
  });

  it("PLZ_REGEX itself does not trim", () => {
    expect(PLZ_REGEX.test(" 73035")).toBe(false);
    expect(PLZ_REGEX.test("73035")).toBe(true);
  });
});

describe("isValidEmail", () => {
  it.each(["a@b.de", "first.last+tag@sub.example.com", "  user@example.org  "])(
    "accepts %s",
    (email) => {
      expect(isValidEmail(email)).toBe(true);
    },
  );

  it.each([
    ["empty", ""],
    ["no at", "user.example.de"],
    ["no domain dot", "user@example"],
    ["no local part", "@example.de"],
    ["empty domain label", "user@.de"],
    ["trailing dot only", "user@example."],
    ["double at", "user@@example.de"],
    ["inner space", "us er@example.de"],
  ])("rejects %s", (_name, email) => {
    expect(isValidEmail(email)).toBe(false);
  });
});

describe("isValidRadius", () => {
  it.each([1, 50, 499.5, 500])("accepts %s", (r) => {
    expect(isValidRadius(r)).toBe(true);
  });

  it.each([0, 0.999, -1, 500.001, 501, Number.NaN, Number.POSITIVE_INFINITY, Number.NEGATIVE_INFINITY])(
    "rejects %s",
    (r) => {
      expect(isValidRadius(r)).toBe(false);
    },
  );
});

describe("formatPrice", () => {
  it("returns null for an unknown price so callers can render 'Preis unbekannt'", () => {
    expect(formatPrice(null)).toBeNull();
    expect(formatPrice(undefined as unknown as null)).toBeNull();
  });

  it("formats euro cents as de-DE currency by default", () => {
    expect(plain(formatPrice(49900))).toBe("499,00 €");
    expect(plain(formatPrice(1999))).toBe("19,99 €");
    expect(plain(formatPrice(5))).toBe("0,05 €");
  });

  it("formats a price of zero instead of treating it as unknown", () => {
    expect(plain(formatPrice(0))).toBe("0,00 €");
  });

  it("uses de-DE thousands separators", () => {
    expect(plain(formatPrice(123456789))).toBe("1.234.567,89 €");
  });

  it("honours currency and locale arguments", () => {
    expect(formatPrice(1999, "USD", "en-US")).toBe("$19.99");
    expect(plain(formatPrice(1999, "CHF", "de-DE"))).toBe("19,99 CHF");
  });
});

describe("formatDistanceKm", () => {
  it("returns null for an unknown distance", () => {
    expect(formatDistanceKm(null)).toBeNull();
    expect(formatDistanceKm(undefined as unknown as null)).toBeNull();
  });

  it("formats with at most one decimal and a ' km' suffix in de-DE", () => {
    expect(formatDistanceKm(1.2)).toBe("1,2 km");
    expect(formatDistanceKm(25.44)).toBe("25,4 km");
    expect(formatDistanceKm(39.96)).toBe("40 km");
    expect(formatDistanceKm(12)).toBe("12 km");
  });

  it("formats zero distance", () => {
    expect(formatDistanceKm(0)).toBe("0 km");
  });

  it("uses de-DE grouping for large distances", () => {
    expect(formatDistanceKm(1234.5)).toBe("1.234,5 km");
  });

  it("honours the locale argument", () => {
    expect(formatDistanceKm(1234.5, "en-US")).toBe("1,234.5 km");
  });
});

describe("formatDateTime", () => {
  it.each([
    ["null", null],
    ["empty", ""],
    ["garbage", "not-a-date"],
    ["impossible month", "2026-13-01T00:00:00Z"],
  ])("returns null for %s input", (_name, iso) => {
    expect(formatDateTime(iso)).toBeNull();
  });

  it("formats a valid ISO timestamp as medium date + short time in de-DE", () => {
    // Without an offset the timestamp is local time, so the output is the same in every TZ.
    expect(plain(formatDateTime("2026-06-28T14:05:00"))).toBe("28.06.2026, 14:05");
  });

  it("converts an offset timestamp into the local zone", () => {
    const out = plain(formatDateTime("2026-06-28T12:00:00Z"));
    expect(out).toMatch(/^(27|28|29)\.06\.2026, \d{2}:\d{2}$/);
  });

  it("honours the locale argument", () => {
    expect(plain(formatDateTime("2026-06-28T14:05:00", "en-US"))).toBe("Jun 28, 2026, 2:05 PM");
  });
});
