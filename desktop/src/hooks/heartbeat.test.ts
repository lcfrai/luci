import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
// The stylesheet is read through Vite's `?raw` loader rather than node:fs, which is how task
// 14's src/styles/tokens.test.ts already reads it. Two measured reasons, not a preference:
// `@types/node` is NOT a dependency of this package (vite.config.ts carries a @ts-expect-error
// for `node:process` precisely because of that), so `import { readFileSync } from "node:fs"`
// fails typecheck with TS2591; and `import.meta.url` under vitest's transform is not a file:
// URL here, so fileURLToPath throws "The URL must be of scheme file". vitest.config.ts opts
// `?raw` past the css-disable plugin for exactly this. Same subject, same assertion.
import tokensCss from "../styles/tokens.css?raw";
import { BEAT_MS, EPOCH, beatDelayStyle, useHeartbeat, __resetHeartbeat } from "./heartbeat";

describe("beatDelayStyle", () => {
  it("returns a negative delay so a late mount is already in phase", () => {
    const spy = vi.spyOn(performance, "now").mockReturnValue(EPOCH + 900);
    expect(beatDelayStyle()).toEqual({ animationDelay: "-900ms" });
    spy.mockRestore();
  });

  it("wraps at one beat, so the delay is never longer than the cycle", () => {
    const spy = vi.spyOn(performance, "now").mockReturnValue(EPOCH + BEAT_MS * 3 + 120);
    expect(beatDelayStyle()).toEqual({ animationDelay: "-120ms" });
    spy.mockRestore();
  });

  it("agrees with the --beat token in tokens.css", () => {
    const match = /--beat:\s*(\d+)ms/.exec(tokensCss);
    expect(match).not.toBeNull();
    expect(Number(match![1])).toBe(BEAT_MS);
  });
});

describe("useHeartbeat", () => {
  // ORDER MATTERS, and getting it wrong fails by days rather than by milliseconds.
  // __resetHeartbeat() seeds the module's cached tick from Date.now(), so the fake clock has
  // to already BE at the fixed instant when it runs. Call setSystemTime inside the test
  // instead and the cached tick is a real-clock value while the interval fires on the fake
  // one, so `result.current - first` is the gap between 2026-09-18 and today, not 1000.
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-18T00:00:00.400Z"));
    __resetHeartbeat();
  });
  afterEach(() => {
    __resetHeartbeat();
    vi.useRealTimers();
  });

  it("ticks once a second and reports whole seconds", () => {
    const { result } = renderHook(() => useHeartbeat());
    const first = result.current;
    expect(first % 1000).toBe(0);
    act(() => void vi.advanceTimersByTime(1000));
    expect(result.current - first).toBe(1000);
  });

  it("shares one interval across many subscribers", () => {
    const setInterval = vi.spyOn(globalThis, "setInterval");
    const a = renderHook(() => useHeartbeat());
    const b = renderHook(() => useHeartbeat());
    const c = renderHook(() => useHeartbeat());
    expect(setInterval).toHaveBeenCalledTimes(1);
    expect(a.result.current).toBe(b.result.current);
    expect(b.result.current).toBe(c.result.current);
    setInterval.mockRestore();
  });

  it("stops the interval when the last subscriber unmounts", () => {
    const clearInterval = vi.spyOn(globalThis, "clearInterval");
    const a = renderHook(() => useHeartbeat());
    const b = renderHook(() => useHeartbeat());
    a.unmount();
    expect(clearInterval).not.toHaveBeenCalled();
    b.unmount();
    expect(clearInterval).toHaveBeenCalledTimes(1);
    clearInterval.mockRestore();
  });
});
