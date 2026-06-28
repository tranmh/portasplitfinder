import { de, type Messages } from "./de";
import { en } from "./en";

export type Locale = "de" | "en";

export const DEFAULT_LOCALE: Locale = "de";

const dictionaries: Record<Locale, Messages> = { de, en };

export function getMessages(locale: Locale = DEFAULT_LOCALE): Messages {
  return dictionaries[locale] ?? dictionaries[DEFAULT_LOCALE];
}

export type { Messages };
