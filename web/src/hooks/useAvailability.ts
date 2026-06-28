import { useCallback, useEffect, useRef, useState } from "react";
import {
  getAvailability,
  type AvailabilityResponse,
  type AvailabilityQuery,
} from "@/lib/api-client";
import { isValidPlz } from "@/lib/format";

export interface UseAvailabilityState {
  data: AvailabilityResponse | null;
  loading: boolean;
  error: Error | null;
  refetch: () => void;
}

/**
 * Fetches /api/availability on mount and whenever plz/radius change.
 * Only sends `plz`/`radius` when a valid 5-digit PLZ is provided.
 */
export function useAvailability(
  plz: string,
  radius: number,
  productId?: string,
): UseAvailabilityState {
  const [data, setData] = useState<AvailabilityResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [nonce, setNonce] = useState(0);
  const abortRef = useRef<AbortController | null>(null);

  const refetch = useCallback(() => setNonce((n) => n + 1), []);

  const validPlz = isValidPlz(plz);

  useEffect(() => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    const query: AvailabilityQuery = { productId };
    if (validPlz) {
      query.plz = plz.trim();
      query.radius = radius;
    }

    setLoading(true);
    setError(null);

    getAvailability(query, controller.signal)
      .then((res) => {
        if (!controller.signal.aborted) {
          setData(res);
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return;
        if (err instanceof DOMException && err.name === "AbortError") return;
        setError(err instanceof Error ? err : new Error(String(err)));
        setLoading(false);
      });

    return () => controller.abort();
  }, [plz, radius, productId, validPlz, nonce]);

  return { data, loading, error, refetch };
}
