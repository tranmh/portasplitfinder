import { describe, it, expect } from "vitest";
import { DEFAULT_LOCALE, getMessages, type Locale } from "./index";
import { de } from "./de";
import { en } from "./en";

/** Dotted paths of every leaf in a nested dictionary, sorted. */
function leafPaths(obj: Record<string, unknown>, prefix = ""): string[] {
  return Object.entries(obj)
    .flatMap(([key, value]) => {
      const path = prefix ? `${prefix}.${key}` : key;
      return value !== null && typeof value === "object"
        ? leafPaths(value as Record<string, unknown>, path)
        : [path];
    })
    .sort();
}

function leafValues(obj: Record<string, unknown>): unknown[] {
  return Object.values(obj).flatMap((v) =>
    v !== null && typeof v === "object" ? leafValues(v as Record<string, unknown>) : [v],
  );
}

describe("i18n dictionaries", () => {
  it("de and en have exactly the same keys", () => {
    expect(leafPaths(en)).toEqual(leafPaths(de));
  });

  it("de and en have the same nesting (no key is a string in one and a group in the other)", () => {
    const groups = (o: Record<string, unknown>) =>
      Object.entries(o)
        .filter(([, v]) => typeof v === "object")
        .map(([k]) => k)
        .sort();
    expect(groups(en)).toEqual(groups(de));
  });

  it.each([
    ["de", de],
    ["en", en],
  ])("every %s message is a non-empty string", (_name, dict) => {
    for (const value of leafValues(dict)) {
      expect(typeof value).toBe("string");
      expect((value as string).trim()).not.toBe("");
    }
  });

  it("labels online retailers as 'deutschlandweit verfügbar' in German", () => {
    expect(de.online.nationwide).toBe("deutschlandweit verfügbar");
    expect(de.online.priceUnknown).toBe("Preis unbekannt");
  });

  it("actually translates the English dictionary", () => {
    expect(en.online.nationwide).not.toBe(de.online.nationwide);
    expect(en.online.priceUnknown).not.toBe(de.online.priceUnknown);
  });
});

describe("getMessages", () => {
  it("defaults to German", () => {
    expect(DEFAULT_LOCALE).toBe("de");
    expect(getMessages()).toBe(de);
  });

  it("returns the dictionary for each supported locale", () => {
    expect(getMessages("de")).toBe(de);
    expect(getMessages("en")).toBe(en);
  });

  it("falls back to the default locale for an unknown locale", () => {
    expect(getMessages("fr" as Locale)).toBe(de);
  });
});
