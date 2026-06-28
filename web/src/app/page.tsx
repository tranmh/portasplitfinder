"use client";

import { useCallback, useState } from "react";
import { getMessages, DEFAULT_LOCALE } from "@/i18n";
import { useAvailability } from "@/hooks/useAvailability";
import { PlzFilter, type PlzFilterValue } from "@/components/PlzFilter";
import { MapPanel } from "@/components/MapPanel";
import { OnlinePanel } from "@/components/OnlinePanel";
import { StoreList } from "@/components/StoreList";
import { SubscribeForm } from "@/components/SubscribeForm";
import { formatDateTime } from "@/lib/format";

const messages = getMessages(DEFAULT_LOCALE);

export default function HomePage() {
  const [plz, setPlz] = useState("");
  const [radius, setRadius] = useState(50);

  const handleFilterChange = useCallback((value: PlzFilterValue) => {
    setPlz(value.plz);
    setRadius(value.radius);
  }, []);

  const { data, loading, error, refetch } = useAvailability(plz, radius);

  const center = data?.center ?? null;
  const stores = data?.stores ?? [];
  const online = data?.online ?? [];
  const radiusKm = data?.radiusKm ?? radius;
  const generatedAt = formatDateTime(data?.generatedAt ?? null);

  return (
    <main className="mx-auto max-w-7xl px-4 py-6">
      <header className="mb-6">
        <h1 className="text-2xl font-bold text-slate-900">{messages.appTitle}</h1>
        <p className="text-sm text-slate-500">{messages.appSubtitle}</p>
      </header>

      <div className="grid gap-6 lg:grid-cols-[360px_1fr]">
        {/* Left column: filter + lists + subscribe */}
        <div className="space-y-6">
          <PlzFilter messages={messages} onChange={handleFilterChange} />

          {error ? (
            <div className="rounded-lg border border-red-200 bg-red-50 p-4">
              <p className="text-sm text-red-800">{messages.state.error}</p>
              <button
                type="button"
                onClick={refetch}
                className="mt-2 rounded-md bg-red-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-red-700"
              >
                {messages.state.retry}
              </button>
            </div>
          ) : loading && !data ? (
            <div className="rounded-lg border border-slate-200 bg-white p-4 text-sm text-slate-500">
              {messages.state.loading}
            </div>
          ) : (
            <>
              <StoreList
                stores={stores}
                messages={messages}
                hasCenter={Boolean(center)}
              />
              <OnlinePanel online={online} messages={messages} />
            </>
          )}

          <SubscribeForm
            messages={messages}
            initialPlz={plz}
            initialRadius={radius}
          />
        </div>

        {/* Right column: map */}
        <div className="lg:sticky lg:top-6 lg:h-[calc(100vh-6rem)]">
          <MapPanel
            stores={stores}
            center={center}
            radiusKm={radiusKm}
            messages={messages}
          />
          {generatedAt && (
            <p className="mt-2 text-right text-xs text-slate-400">
              {messages.state.generatedAt}: {generatedAt}
            </p>
          )}
        </div>
      </div>
    </main>
  );
}
