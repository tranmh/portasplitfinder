import { bench, describe } from "vitest";
import { getMessages, type Locale } from "./index";

// Flattened key paths of a dictionary, the basis of the de/en parity check.
function keyPaths(obj: unknown, prefix = "", out: string[] = []): string[] {
  if (obj && typeof obj === "object") {
    for (const [k, v] of Object.entries(obj as Record<string, unknown>)) {
      keyPaths(v, prefix ? `${prefix}.${k}` : k, out);
    }
  } else {
    out.push(prefix);
  }
  return out;
}

function missingKeys(a: Locale, b: Locale): string[] {
  const bKeys = new Set(keyPaths(getMessages(b)));
  return keyPaths(getMessages(a)).filter((k) => !bKeys.has(k));
}

const SIZES = [10, 100, 1_000] as const;

let sink: unknown;

describe("i18n de/en key parity", () => {
  for (const n of SIZES) {
    bench(`parity check x${n}`, () => {
      let missing = 0;
      for (let i = 0; i < n; i++) {
        missing += missingKeys("de", "en").length + missingKeys("en", "de").length;
      }
      sink = missing;
    });
  }
});

describe("i18n getMessages lookup", () => {
  for (const n of SIZES) {
    const locales: Locale[] = Array.from({ length: n }, (_, i) => (i % 2 === 0 ? "de" : "en"));
    bench(`getMessages x${n}`, () => {
      let c = 0;
      for (const l of locales) c += getMessages(l).appTitle.length;
      sink = c;
    });
  }
});

export { sink };
