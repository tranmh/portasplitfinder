import { bench, describe, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { availabilityFixture } from "@/test/fixtures";
import { useAvailability } from "./useAvailability";

// No network: the api-client resolves from a fixture; every 5th PLZ fails so
// the error branch is exercised too.
vi.mock("@/lib/api-client", () => ({
  getAvailability: (query: { plz?: string }) =>
    query.plz && query.plz.endsWith("5")
      ? Promise.reject(new Error("boom"))
      : Promise.resolve(availabilityFixture),
}));

const SIZES = [10, 100, 500] as const;

let sink: unknown;

describe("useAvailability: param changes trigger refetch", () => {
  for (const n of SIZES) {
    // Valid and invalid PLZs alternating with radius changes.
    const params = Array.from({ length: n }, (_, i) => ({
      plz: i % 3 === 0 ? "123" : String(10000 + i),
      radius: (i % 50) + 1,
    }));

    bench(`rerender with ${n} param changes`, async () => {
      const { result, rerender, unmount } = renderHook(
        ({ plz, radius }) => useAvailability(plz, radius),
        { initialProps: { plz: "73035", radius: 50 } },
      );
      for (const p of params) {
        await act(async () => {
          rerender(p);
        });
      }
      sink = result.current;
      unmount();
    });

    bench(`refetch ${n} times`, async () => {
      const { result, unmount } = renderHook(() => useAvailability("73035", 50));
      for (let i = 0; i < n; i++) {
        await act(async () => {
          result.current.refetch();
        });
      }
      sink = result.current;
      unmount();
    });
  }
});

export { sink };
