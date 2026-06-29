"use client";

import { useEffect, useState } from "react";
import { useDebouncedValue } from "@/hooks/useDebouncedValue";
import { isValidPlz } from "@/lib/format";
import type { Messages } from "@/i18n";

export interface PlzFilterValue {
  plz: string;
  radius: number;
}

export interface PlzFilterProps {
  messages: Messages;
  defaultPlz?: string;
  defaultRadius?: number;
  debounceMs?: number;
  /** Fires (debounced) with a valid 5-digit PLZ + radius, or empty PLZ to clear. */
  onChange: (value: PlzFilterValue) => void;
}

export function PlzFilter({
  messages,
  defaultPlz = "",
  defaultRadius = 50,
  debounceMs = 400,
  onChange,
}: PlzFilterProps) {
  const t = messages.filter;
  const [plz, setPlz] = useState(defaultPlz);
  const [radius, setRadius] = useState(defaultRadius);

  const debouncedPlz = useDebouncedValue(plz, debounceMs);
  const debouncedRadius = useDebouncedValue(radius, debounceMs);

  const trimmed = plz.trim();
  const showError = trimmed.length > 0 && !isValidPlz(trimmed);

  useEffect(() => {
    const value = debouncedPlz.trim();
    // Emit when PLZ is valid, or when cleared (so the parent can reset).
    if (value.length === 0 || isValidPlz(value)) {
      onChange({ plz: value, radius: debouncedRadius });
    }
    // onChange is expected to be stable (memoized by caller).
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedPlz, debouncedRadius]);

  return (
    <section className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
      <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-slate-500">
        {t.heading}
      </h2>

      <div className="space-y-4">
        <div>
          <label
            htmlFor="plz-filter-input"
            className="mb-1 block text-sm font-medium text-slate-700"
          >
            {t.plzLabel}
          </label>
          <input
            id="plz-filter-input"
            name="plz"
            type="text"
            inputMode="numeric"
            autoComplete="postal-code"
            maxLength={5}
            placeholder={t.plzPlaceholder}
            value={plz}
            aria-invalid={showError}
            aria-describedby={showError ? "plz-filter-error" : undefined}
            onChange={(e) => setPlz(e.target.value.replace(/[^\d]/g, ""))}
            className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-sky-500 focus:outline-none focus:ring-1 focus:ring-sky-500"
          />
          {showError && (
            <p id="plz-filter-error" role="alert" className="mt-1 text-sm text-red-600">
              {t.plzInvalid}
            </p>
          )}
        </div>

        <div>
          <div className="mb-1 flex items-center justify-between">
            <label
              htmlFor="radius-filter-input"
              className="text-sm font-medium text-slate-700"
            >
              {t.radiusLabel}
            </label>
            <span className="text-sm font-semibold text-slate-900">
              {radius} {t.radiusUnit}
            </span>
          </div>
          <input
            id="radius-filter-input"
            name="radius"
            type="range"
            min={1}
            max={500}
            step={1}
            value={radius}
            aria-label={t.radiusLabel}
            onChange={(e) => setRadius(Number(e.target.value))}
            className="w-full accent-sky-600"
          />
        </div>
      </div>
    </section>
  );
}

export default PlzFilter;
