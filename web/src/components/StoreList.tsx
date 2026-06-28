import type { StoreAvailability } from "@/lib/api-client";
import type { Messages } from "@/i18n";
import { formatDistanceKm, formatPrice } from "@/lib/format";
import { StockBadge } from "./StockBadge";

export interface StoreListProps {
  stores: StoreAvailability[];
  messages: Messages;
  /** Whether a PLZ center is active (controls the empty-state copy). */
  hasCenter: boolean;
}

export function StoreList({ stores, messages, hasCenter }: StoreListProps) {
  const t = messages.stores;

  // Defensive: ensure nearest-first ordering even if the server didn't sort.
  const sorted = [...stores].sort((a, b) => {
    if (a.distanceKm === null) return b.distanceKm === null ? 0 : 1;
    if (b.distanceKm === null) return -1;
    return a.distanceKm - b.distanceKm;
  });

  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <h2 className="mb-3 text-base font-semibold text-slate-900">{t.heading}</h2>

      {sorted.length === 0 ? (
        <p className="text-sm text-slate-500">{hasCenter ? t.none : t.noCenter}</p>
      ) : (
        <ul className="space-y-3">
          {sorted.map((entry) => {
            const distance = formatDistanceKm(entry.distanceKm);
            return (
              <li
                key={entry.store.id}
                className="rounded-md border border-slate-100 p-3"
              >
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <p className="font-medium text-slate-900">{entry.store.name}</p>
                    <p className="text-xs text-slate-500">
                      {entry.store.street ? `${entry.store.street}, ` : ""}
                      {entry.store.plz} {entry.store.city}
                    </p>
                  </div>
                  {distance && (
                    <span className="whitespace-nowrap text-sm font-semibold text-slate-700">
                      {distance}
                    </span>
                  )}
                </div>

                {entry.listings.length > 0 && (
                  <ul className="mt-2 space-y-2">
                    {entry.listings.map((listing, idx) => {
                      const price = formatPrice(listing.priceCents, listing.currency);
                      return (
                        <li
                          key={`${listing.storeId ?? "x"}-${listing.productId}-${idx}`}
                          className="flex flex-wrap items-center justify-between gap-2 text-sm"
                        >
                          <span className="font-medium text-slate-900">
                            {price ?? messages.online.priceUnknown}
                          </span>
                          <StockBadge
                            inStock={listing.inStock}
                            inStockLabel={t.inStock}
                            outOfStockLabel={t.outOfStock}
                          />
                          <a
                            href={listing.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-sky-700 underline hover:text-sky-900"
                          >
                            {t.viewListing}
                          </a>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

export default StoreList;
