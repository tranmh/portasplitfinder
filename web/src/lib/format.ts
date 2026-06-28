/** Shared formatting + validation helpers used across components. */

export const PLZ_REGEX = /^\d{5}$/;

export function isValidPlz(plz: string): boolean {
  return PLZ_REGEX.test(plz.trim());
}

export function isValidEmail(email: string): boolean {
  // Pragmatic email check; the server is authoritative.
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim());
}

export function isValidRadius(radius: number): boolean {
  return Number.isFinite(radius) && radius >= 1 && radius <= 500;
}

/**
 * Format a price given in euro cents. Returns null when the price is unknown
 * so callers can render "Preis unbekannt".
 */
export function formatPrice(
  priceCents: number | null,
  currency = "EUR",
  locale = "de-DE",
): string | null {
  if (priceCents === null || priceCents === undefined) return null;
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
  }).format(priceCents / 100);
}

export function formatDistanceKm(
  distanceKm: number | null,
  locale = "de-DE",
): string | null {
  if (distanceKm === null || distanceKm === undefined) return null;
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(distanceKm)} km`;
}

export function formatDateTime(
  iso: string | null,
  locale = "de-DE",
): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(d);
}
