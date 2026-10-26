import { bench, describe } from "vitest";
import {
  formatDateTime,
  formatDistanceKm,
  formatPrice,
  isValidEmail,
  isValidPlz,
  isValidRadius,
} from "./format";

const SIZES = [100, 1_000, 10_000] as const;

// Deterministic inputs: a mix of valid, invalid and null values.
function plzInputs(n: number): string[] {
  return Array.from({ length: n }, (_, i) =>
    i % 4 === 0 ? ` ${10000 + (i % 89999)} ` : i % 4 === 1 ? "1234" : i % 4 === 2 ? "abcde" : String(10000 + (i % 89999)),
  );
}

function emailInputs(n: number): string[] {
  return Array.from({ length: n }, (_, i) =>
    i % 3 === 0 ? `user${i}@example.de` : i % 3 === 1 ? `user${i}@nodot` : ` spaced ${i}@x.de `,
  );
}

function radiusInputs(n: number): number[] {
  return Array.from({ length: n }, (_, i) =>
    i % 5 === 0 ? Number.NaN : i % 5 === 1 ? 0 : i % 5 === 2 ? 501 : i % 5 === 3 ? Infinity : (i % 500) + 1,
  );
}

function centsInputs(n: number): (number | null)[] {
  return Array.from({ length: n }, (_, i) => (i % 10 === 0 ? null : i * 137));
}

function distanceInputs(n: number): (number | null)[] {
  return Array.from({ length: n }, (_, i) => (i % 10 === 0 ? null : i * 0.37));
}

function isoInputs(n: number): (string | null)[] {
  const base = Date.UTC(2026, 0, 1);
  return Array.from({ length: n }, (_, i) =>
    i % 10 === 0 ? null : i % 10 === 1 ? "not-a-date" : new Date(base + i * 60_000).toISOString(),
  );
}

let sink: unknown;

for (const n of SIZES) {
  const plz = plzInputs(n);
  const email = emailInputs(n);
  const radius = radiusInputs(n);
  const cents = centsInputs(n);
  const dist = distanceInputs(n);
  const iso = isoInputs(n);

  describe(`format validators n=${n}`, () => {
    bench(`isValidPlz n=${n}`, () => {
      let c = 0;
      for (const p of plz) if (isValidPlz(p)) c++;
      sink = c;
    });
    bench(`isValidEmail n=${n}`, () => {
      let c = 0;
      for (const e of email) if (isValidEmail(e)) c++;
      sink = c;
    });
    bench(`isValidRadius n=${n}`, () => {
      let c = 0;
      for (const r of radius) if (isValidRadius(r)) c++;
      sink = c;
    });
  });

  // Each formatter builds a fresh Intl formatter per call; the cost grows
  // linearly with the number of rendered rows.
  describe(`format formatters n=${n}`, () => {
    bench(`formatPrice n=${n}`, () => {
      const out: (string | null)[] = new Array(n);
      for (let i = 0; i < n; i++) out[i] = formatPrice(cents[i]);
      sink = out;
    });
    bench(`formatDistanceKm n=${n}`, () => {
      const out: (string | null)[] = new Array(n);
      for (let i = 0; i < n; i++) out[i] = formatDistanceKm(dist[i]);
      sink = out;
    });
    bench(`formatDateTime n=${n}`, () => {
      const out: (string | null)[] = new Array(n);
      for (let i = 0; i < n; i++) out[i] = formatDateTime(iso[i]);
      sink = out;
    });
  });
}

export { sink };
