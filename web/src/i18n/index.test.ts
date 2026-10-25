import { describe, expect, it } from "vitest";
import { DEFAULT_LOCALE, getMessages, type Locale } from "./index";
import { de } from "./de";
import { en } from "./en";

type Tree = { [key: string]: string | Tree };

function leafPaths(obj: Tree, prefix = ""): string[] {
  return Object.entries(obj).flatMap(([k, v]) => {
    const path = prefix ? `${prefix}.${k}` : k;
    return typeof v === "string" ? [path] : leafPaths(v, path);
  });
}

function leaves(obj: Tree, prefix = ""): [string, string][] {
  return Object.entries(obj).flatMap(([k, v]) => {
    const path = prefix ? `${prefix}.${k}` : k;
    return typeof v === "string" ? [[path, v] as [string, string]] : leaves(v, path);
  });
}

describe("i18n dictionaries", () => {
  it("de and en have exactly the same keys", () => {
    const deKeys = leafPaths(de as Tree).sort();
    const enKeys = leafPaths(en as unknown as Tree).sort();
    expect(enKeys).toEqual(deKeys);
  });

  it.each([
    ["de", de],
    ["en", en],
  ] as const)("%s has no empty strings", (_name, dict) => {
    for (const [path, value] of leaves(dict as unknown as Tree)) {
      expect(value.trim(), path).not.toBe("");
    }
  });

  it("contains the contract copy for online entries and unknown prices", () => {
    expect(de.online.nationwide).toBe("deutschlandweit verfügbar");
    expect(de.online.priceUnknown).toBe("Preis unbekannt");
  });
});

describe("getMessages", () => {
  it("defaults to German", () => {
    expect(DEFAULT_LOCALE).toBe("de");
    expect(getMessages()).toBe(de);
  });

  it("returns the requested dictionary", () => {
    expect(getMessages("de")).toBe(de);
    expect(getMessages("en")).toBe(en);
  });

  it("falls back to the default locale for an unknown locale", () => {
    expect(getMessages("fr" as Locale)).toBe(de);
  });
});
