import { vi } from "vitest";
import { FIXED_ISO } from "./builders";

/**
 * Freezes Date.now()/new Date() at a fixed instant and lets the test drive
 * timers (setTimeout/setInterval) by hand, e.g. for useDebouncedValue.
 * Promises and queueMicrotask stay real so async hooks still settle.
 * Call the returned function (or vi.useRealTimers()) in afterEach.
 */
export function useFixedClock(iso: string = FIXED_ISO): () => void {
  vi.useFakeTimers({
    now: new Date(iso),
    toFake: ["setTimeout", "clearTimeout", "setInterval", "clearInterval", "Date"],
  });
  return () => vi.useRealTimers();
}
