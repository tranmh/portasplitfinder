import { bench, describe, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useDebouncedValue } from "./useDebouncedValue";

const SIZES = [10, 100, 1_000] as const;

let sink: unknown;

describe("useDebouncedValue: burst of changes then settle", () => {
  for (const n of SIZES) {
    const values = Array.from({ length: n }, (_, i) => String(10000 + i));

    bench(`${n} keystrokes within the delay`, () => {
      // Fake timers only inside the call: no real waiting happens, and the
      // bench runner keeps its real timers between iterations.
      vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
      try {
        run(values);
      } finally {
        vi.useRealTimers();
      }
    });
  }
});

function run(values: string[]) {
  {
    {
      const { result, rerender, unmount } = renderHook(
        ({ value }) => useDebouncedValue(value, 400),
        { initialProps: { value: "" } },
      );
      for (const value of values) {
        act(() => {
          rerender({ value });
          vi.advanceTimersByTime(50);
        });
      }
      act(() => {
        vi.advanceTimersByTime(400);
      });
      sink = result.current;
      unmount();
    }
  }
}

export { sink };
