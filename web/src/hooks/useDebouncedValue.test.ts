import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useDebouncedValue } from "./useDebouncedValue";

describe("useDebouncedValue", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  function setup<T>(initial: T, delay?: number) {
    return renderHook(
      ({ value, delay }: { value: T; delay?: number }) => useDebouncedValue(value, delay),
      { initialProps: { value: initial, delay } },
    );
  }

  it("returns the initial value immediately", () => {
    const { result } = setup("73035");
    expect(result.current).toBe("73035");
  });

  it("updates only after the default 400 ms", () => {
    const { result, rerender } = setup("73035");
    rerender({ value: "10115", delay: undefined });

    act(() => vi.advanceTimersByTime(399));
    expect(result.current).toBe("73035");

    act(() => vi.advanceTimersByTime(1));
    expect(result.current).toBe("10115");
  });

  it("restarts the delay on every change and emits only the last value", () => {
    const { result, rerender } = setup("", 300);

    rerender({ value: "1", delay: 300 });
    act(() => vi.advanceTimersByTime(250));
    rerender({ value: "10", delay: 300 });
    act(() => vi.advanceTimersByTime(250));
    rerender({ value: "101", delay: 300 });
    act(() => vi.advanceTimersByTime(299));
    expect(result.current).toBe("");

    act(() => vi.advanceTimersByTime(1));
    expect(result.current).toBe("101");
  });

  it("honours a custom delay", () => {
    const { result, rerender } = setup(1, 1000);
    rerender({ value: 2, delay: 1000 });

    act(() => vi.advanceTimersByTime(999));
    expect(result.current).toBe(1);
    act(() => vi.advanceTimersByTime(1));
    expect(result.current).toBe(2);
  });

  it("treats a zero delay as the next tick", () => {
    const { result, rerender } = setup("a", 0);
    rerender({ value: "b", delay: 0 });
    expect(result.current).toBe("a");
    act(() => vi.advanceTimersByTime(0));
    expect(result.current).toBe("b");
  });

  it("does not schedule a visible change when re-rendered with the same value", () => {
    const { result, rerender } = setup("x", 200);
    rerender({ value: "x", delay: 200 });
    act(() => vi.advanceTimersByTime(200));
    expect(result.current).toBe("x");
  });

  it("clears the pending timer on unmount", () => {
    const { rerender, unmount } = setup("a", 400);
    rerender({ value: "b", delay: 400 });
    expect(vi.getTimerCount()).toBeGreaterThan(0);

    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });
});
