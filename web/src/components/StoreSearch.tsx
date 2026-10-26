"use client";

import { useState, type FormEvent } from "react";
import { searchStores, type StoreAvailability } from "@/lib/api-client";
import type { Messages } from "@/i18n";
import { formatDistanceKm, isValidPlz } from "@/lib/format";
import { StockBadge } from "./StockBadge";

export interface StoreSearchProps {
  messages: Messages;
  /** Current PLZ filter; when valid, results are ordered by distance. */
  plz: string;
}

export function StoreSearch({ messages, plz }: StoreSearchProps) {
  const t = messages.storeSearch;
  const [term, setTerm] = useState("");
  const [results, setResults] = useState<StoreAvailability[] | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [error, setError] = useState(false);

  async function load(nextPage: number) {
    setError(false);
    try {
      const res = await searchStores({
        q: term.trim(),
        plz: isValidPlz(plz) ? plz.trim() : undefined,
        page: nextPage,
      });
      setResults((prev) => (nextPage === 1 || !prev ? res.stores : [...prev, ...res.stores]));
      setTotal(res.total);
      setPage(nextPage);
    } catch {
      setError(true);
    }
  }

  function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    void load(1);
  }

  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <h2 className="mb-3 text-base font-semibold text-slate-900">{t.heading}</h2>
      <form onSubmit={handleSubmit} className="flex gap-2">
        <input
          type="search"
          value={term}
          placeholder={t.placeholder}
          onChange={(e) => setTerm(e.target.value)}
          className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500"
        />
        <button
          type="submit"
          className="rounded-md bg-sky-600 px-3 py-2 text-white hover:bg-sky-700"
        >
          <svg viewBox="0 0 20 20" fill="currentColor" className="h-4 w-4">
            <path
              fillRule="evenodd"
              d="M9 3.5a5.5 5.5 0 1 0 3.47 9.77l3.63 3.63a.75.75 0 1 0 1.06-1.06l-3.63-3.63A5.5 5.5 0 0 0 9 3.5ZM5 9a4 4 0 1 1 8 0 4 4 0 0 1-8 0Z"
              clipRule="evenodd"
            />
          </svg>
        </button>
      </form>

      {error && <p className="mt-2 text-sm text-red-600">{t.error}</p>}

      {results && (
        <div className="mt-3">
          <p className="mb-2 text-xs text-slate-500">
            {total} {t.results}
          </p>
          {results.length === 0 ? (
            <p className="text-sm text-slate-500">{t.noResults}</p>
          ) : (
            <ul className="space-y-2">
              {results.map((entry) => {
                const distance = formatDistanceKm(entry.distanceKm);
                const anyInStock = entry.listings.some((l) => l.inStock);
                return (
                  <li
                    key={entry.store.id}
                    className="flex items-center justify-between gap-2 rounded-md border border-slate-100 p-2 text-sm"
                  >
                    <div>
                      <p className="font-medium text-slate-900">{entry.store.name}</p>
                      <p className="text-xs text-slate-500">
                        {entry.store.plz} {entry.store.city}
                        {distance ? ` · ${distance}` : ""}
                      </p>
                    </div>
                    <StockBadge
                      inStock={anyInStock}
                      inStockLabel={messages.stores.inStock}
                      outOfStockLabel={messages.stores.outOfStock}
                    />
                  </li>
                );
              })}
            </ul>
          )}
          {results.length < total && (
            <button
              type="button"
              onClick={() => void load(page + 1)}
              className="mt-3 text-sm font-medium text-sky-700 underline hover:text-sky-900"
            >
              {t.loadMore}
            </button>
          )}
        </div>
      )}
    </section>
  );
}

export default StoreSearch;
