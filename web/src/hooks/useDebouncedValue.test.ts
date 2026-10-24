import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useDebouncedValue } from "./useDebouncedValue";

describe("useDebouncedValue", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns the initial value immediately", () => {
    const { result } = renderHook(() => useDebouncedValue("73035", 400));
    expect(result.current).toBe("73035");
  });

  it("updates only after the delay has elapsed", () => {
    const { result, rerender } = renderHook(({ v }) => useDebouncedValue(v, 400), {
      initialProps: { v: "7" },
    });

    rerender({ v: "73" });
    act(() => {
      vi.advanceTimersByTime(399);
    });
    expect(result.current).toBe("7");

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe("73");
  });

  it("uses 400 ms as the default delay", () => {
    const { result, rerender } = renderHook(({ v }) => useDebouncedValue(v), {
      initialProps: { v: 1 },
    });
    rerender({ v: 2 });
    act(() => {
      vi.advanceTimersByTime(399);
    });
    expect(result.current).toBe(1);
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe(2);
  });

  it("restarts the timer on every change and emits only the last value", () => {
    const { result, rerender } = renderHook(({ v }) => useDebouncedValue(v, 300), {
      initialProps: { v: "" },
    });

    for (const v of ["7", "73", "730", "7303", "73035"]) {
      rerender({ v });
      act(() => {
        vi.advanceTimersByTime(200);
      });
      expect(result.current).toBe("");
    }

    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(result.current).toBe("73035");
  });

  it("restarts the timer when the delay changes", () => {
    const { result, rerender } = renderHook(({ v, d }) => useDebouncedValue(v, d), {
      initialProps: { v: "a", d: 100 },
    });
    rerender({ v: "b", d: 100 });
    act(() => {
      vi.advanceTimersByTime(50);
    });
    rerender({ v: "b", d: 500 });
    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(result.current).toBe("a");
    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(result.current).toBe("b");
  });

  it("with a zero delay updates on the next tick", () => {
    const { result, rerender } = renderHook(({ v }) => useDebouncedValue(v, 0), {
      initialProps: { v: 1 },
    });
    rerender({ v: 2 });
    expect(result.current).toBe(1);
    act(() => {
      vi.advanceTimersByTime(0);
    });
    expect(result.current).toBe(2);
  });

  it("does not fire a pending update after unmount", () => {
    const { rerender, unmount } = renderHook(({ v }) => useDebouncedValue(v, 400), {
      initialProps: { v: 1 },
    });
    rerender({ v: 2 });
    expect(vi.getTimerCount()).toBe(1);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
