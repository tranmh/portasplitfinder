import type { OnlineAvailability } from "@/lib/api-client";
import type { Messages } from "@/i18n";
import { formatDateTime, formatPrice } from "@/lib/format";
import { StockBadge } from "./StockBadge";

export interface OnlinePanelProps {
  online: OnlineAvailability[];
  messages: Messages;
}

export function OnlinePanel({ online, messages }: OnlinePanelProps) {
  const t = messages.online;

  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <h2 className="text-base font-semibold text-slate-900">{t.heading}</h2>
      <p className="mb-3 text-sm text-slate-500">{t.nationwide}</p>

      {online.length === 0 ? (
        <p className="text-sm text-slate-500">{t.none}</p>
      ) : (
        <ul className="space-y-3">
          {online.map((entry) => {
            const lastChecked = formatDateTime(entry.source.lastCheckedAt);
            return (
              <li
                key={entry.source.key}
                className="rounded-md border border-slate-100 p-3"
              >
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <p className="font-medium text-slate-900">{entry.source.name}</p>
                    <p className="text-xs text-slate-500">{t.nationwide}</p>
                  </div>
                </div>

                {entry.listings.length === 0 ? (
                  <p className="mt-2 text-sm text-slate-500">{t.none}</p>
                ) : (
                  <ul className="mt-2 space-y-2">
                    {entry.listings.map((listing, idx) => {
                      const price = formatPrice(listing.priceCents, listing.currency);
                      return (
                        <li
                          key={`${listing.sourceKey}-${listing.productId}-${idx}`}
                          className="flex flex-wrap items-center justify-between gap-2 text-sm"
                        >
                          <span className="font-medium text-slate-900">
                            {price ?? t.priceUnknown}
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

                <p className="mt-2 text-xs text-slate-400">
                  {t.lastChecked}: {lastChecked ?? t.never}
                </p>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

export default OnlinePanel;
