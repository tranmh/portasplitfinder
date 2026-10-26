import { test, expect } from "vitest";
import fc from "fast-check";
import {
  isValidPlz,
  isValidEmail,
  isValidRadius,
  formatPrice,
  formatDistanceKm,
  formatDateTime,
} from "./format";

const digit = fc.constantFrom(..."0123456789".split(""));
const whitespace = fc
  .array(fc.constantFrom(" ", "\t", "\n", " "), { maxLength: 3 })
  .map((xs) => xs.join(""));

// Parse a de-DE number back: drop grouping dots, spaces and units, decimal comma to dot.
function parseDe(s: string): number {
  return Number(s.replace(/[^\d,\-]/g, "").replace(",", "."));
}

test("isValidPlz accepts exactly five digits with surrounding whitespace", () => {
  fc.assert(
    fc.property(
      fc.array(digit, { maxLength: 8 }).map((xs) => xs.join("")),
      whitespace,
      whitespace,
      (digits, pre, post) => {
        expect(isValidPlz(pre + digits + post)).toBe(digits.length === 5);
      },
    ),
  );
});

test("isValidEmail rejects strings without exactly one at sign", () => {
  fc.assert(
    fc.property(fc.string(), (s) => {
      const ats = s.split("@").length - 1;
      fc.pre(ats !== 1);
      expect(isValidEmail(s)).toBe(false);
    }),
  );
});

test("isValidRadius accepts only finite values from 1 to 500", () => {
  fc.assert(
    fc.property(
      fc.oneof(fc.double(), fc.integer({ min: -10, max: 600 })),
      (r) => {
        expect(isValidRadius(r)).toBe(Number.isFinite(r) && r >= 1 && r <= 500);
      },
    ),
  );
});

test("formatPrice round trips integer cents", () => {
  fc.assert(
    fc.property(fc.integer(), (cents) => {
      const s = formatPrice(cents);
      expect(s).not.toBeNull();
      expect(s).toContain("€");
      expect(Math.round(parseDe(s as string) * 100)).toBe(cents);
    }),
  );
});

test("formatDistanceKm stays within rounding of the input", () => {
  fc.assert(
    fc.property(
      fc.double({ min: -1e6, max: 1e6, noNaN: true }),
      (km) => {
        const s = formatDistanceKm(km);
        expect(s).not.toBeNull();
        expect(s!.endsWith(" km")).toBe(true);
        expect(Math.abs(parseDe(s!) - km)).toBeLessThanOrEqual(0.05 + 1e-9);
      },
    ),
  );
});

test("formatDateTime never throws and formats every valid ISO date", () => {
  fc.assert(
    fc.property(
      fc.oneof(
        fc.string(),
        fc.date({ noInvalidDate: true }).map((d) => d.toISOString()),
      ),
      (iso) => {
        const s = formatDateTime(iso);
        if (s === null) {
          expect(Number.isNaN(new Date(iso).getTime()) || iso === "").toBe(true);
        } else {
          expect(typeof s).toBe("string");
          expect(s.length).toBeGreaterThan(0);
        }
      },
    ),
  );
});
