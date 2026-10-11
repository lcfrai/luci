import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { POLL_INTERVAL_MS, coveredWorktreeIds, startPolling } from "./poller";
import { ApiError } from "./client";
import type { CockpitClient } from "./client";
import { resetCockpit, useCockpit } from "../store/store";
import { aSession } from "../test/fixtures";
import type { Session } from "../store/types";

/** The poller calls exactly one method; the cast keeps this stub from drifting from §4.4. */
const stubClient = (listSessions: () => Promise<Session[]>): CockpitClient =>
  ({ listSessions }) as unknown as CockpitClient;

function setVisibility(value: "visible" | "hidden"): void {
  Object.defineProperty(document, "visibilityState", { configurable: true, get: () => value });
  document.dispatchEvent(new Event("visibilitychange"));
}

beforeEach(() => {
  vi.useFakeTimers();
  resetCockpit();
  setVisibility("visible");
});

afterEach(() => {
  setVisibility("visible");
  vi.useRealTimers();
});

describe("startPolling", () => {
  it("fetches once immediately and then on every interval", async () => {
    let calls = 0;
    const stop = startPolling(stubClient(async () => { calls += 1; return []; }), useCockpit);
    expect(calls).toBe(1);                       // the first render does not wait 2 s
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    expect(calls).toBe(2);
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 3);
    expect(calls).toBe(5);
    stop();
  });

  it("writes the fetched sessions into the store", async () => {
    const fresh = aSession({ id: "s-1", worktreeId: "w-1" });
    const stop = startPolling(stubClient(async () => [fresh]), useCockpit);
    await vi.advanceTimersByTimeAsync(0);
    const state = useCockpit.getState();
    expect(state.sessions["s-1"]).toBeDefined();
    expect(state.sessionIdsByWorktree["w-1"]).toEqual(["s-1"]);
    stop();
  });

  it("covers a worktree the store knows about but the response no longer mentions", async () => {
    useCockpit.setState({
      sessions: { "s-old": aSession({ id: "s-old", worktreeId: "w-old" }) },
      sessionIdsByWorktree: { "w-old": ["s-old"] },
    });
    const stop = startPolling(stubClient(async () => []), useCockpit);
    await vi.advanceTimersByTimeAsync(0);
    expect(useCockpit.getState().sessions["s-old"]).toBeUndefined();
    stop();
  });

  it("pauses while the document is hidden and catches up when it is shown", async () => {
    let calls = 0;
    const stop = startPolling(stubClient(async () => { calls += 1; return []; }), useCockpit);
    expect(calls).toBe(1);
    setVisibility("hidden");
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 5);
    expect(calls).toBe(1);                       // a lid closed for eight hours queues nothing
    setVisibility("visible");
    await vi.advanceTimersByTimeAsync(0);
    expect(calls).toBe(2);
    stop();
  });

  it("does not stack requests when one tick outlives the interval", async () => {
    let calls = 0;
    let release: (sessions: Session[]) => void = () => {};
    const stop = startPolling(
      stubClient(() => { calls += 1; return new Promise<Session[]>((r) => { release = r; }); }),
      useCockpit,
    );
    expect(calls).toBe(1);
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 3);
    expect(calls).toBe(1);
    release([]);
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    expect(calls).toBe(2);
    stop();
  });

  it("records a failure and clears it on the next success", async () => {
    let failing = true;
    const stop = startPolling(
      stubClient(async () => {
        if (failing) throw new ApiError("network_unavailable", "fetch failed", 0);
        return [];
      }),
      useCockpit,
    );
    await vi.advanceTimersByTimeAsync(0);
    expect(useCockpit.getState().lastError).toEqual({
      code: "network_unavailable",
      message: "fetch failed",
    });
    failing = false;
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    expect(useCockpit.getState().lastError).toBeNull();
    stop();
  });

  it("stops fetching after stop()", async () => {
    let calls = 0;
    const stop = startPolling(stubClient(async () => { calls += 1; return []; }), useCockpit);
    stop();
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 5);
    expect(calls).toBe(1);
  });

  it("leaves no timer behind after stop()", () => {
    const stop = startPolling(stubClient(async () => []), useCockpit);
    expect(vi.getTimerCount()).toBe(1);
    stop();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("a tick still in flight when stop() lands does not write the store", async () => {
    let release: (sessions: Session[]) => void = () => {};
    const stop = startPolling(
      stubClient(() => new Promise<Session[]>((r) => { release = r; })),
      useCockpit,
    );
    let writes = 0;
    const unsubscribe = useCockpit.subscribe(() => { writes += 1; });
    stop();
    release([aSession({ id: "s-late", worktreeId: "w-late" })]);
    await vi.advanceTimersByTimeAsync(0);
    unsubscribe();
    // AppShell unmounted; a reply that lands afterwards must not repopulate the store.
    expect(writes).toBe(0);
    expect(useCockpit.getState().sessions["s-late"]).toBeUndefined();
  });

  it("a tick that fails after stop() records no error", async () => {
    let reject: (error: unknown) => void = () => {};
    const stop = startPolling(
      stubClient(() => new Promise<Session[]>((_resolve, r) => { reject = r; })),
      useCockpit,
    );
    stop();
    reject(new ApiError("network_unavailable", "fetch failed", 0));
    await vi.advanceTimersByTimeAsync(0);
    expect(useCockpit.getState().lastError).toBeNull();
  });

  it("an unchanged quiet tick keeps notifications and preference writes unchanged", async () => {
    const listSessions = vi.fn(async () => [] as Session[]), goodTick = vi.fn();
    const stop = startPolling(stubClient(listSessions), useCockpit, goodTick);
    await vi.advanceTimersByTimeAsync(0);
    const before = useCockpit.getState(), changed = vi.fn();
    const unsubscribe = useCockpit.subscribe(changed);
    const write = vi.spyOn(Storage.prototype, "setItem");
    try {
      await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
      expect(useCockpit.getState()).toBe(before);
      expect(changed).not.toHaveBeenCalled();
      expect(write).not.toHaveBeenCalled();
      // A quiet response is still a successful fresh read; catalogue retry is
      // not dependent on publishing redundant session state.
      expect(listSessions).toHaveBeenCalledTimes(2);
      expect(goodTick).toHaveBeenCalledTimes(2);
    } finally { unsubscribe(); stop(); write.mockRestore(); }
  });
});

describe("coveredWorktreeIds", () => {
  it("unions what the response saw with what the store already indexes", () => {
    const ids = coveredWorktreeIds(["w-1", "w-2"], [aSession({ id: "s-1", worktreeId: "w-3" })]);
    expect(ids).toEqual(["w-1", "w-2", "w-3"]);
  });

  it("never repeats an id", () => {
    const ids = coveredWorktreeIds(["w-1"], [aSession({ id: "s-1", worktreeId: "w-1" })]);
    expect(ids).toEqual(["w-1"]);
  });
});
