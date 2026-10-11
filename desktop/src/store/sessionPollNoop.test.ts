import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createCockpitStore } from "./store";
import { aSession } from "../test/fixtures";
import { __resetPendingSessions, setSessionPending } from "../lib/pendingSessions";
import type { Session } from "./types";

beforeEach(() => { localStorage.clear(); __resetPendingSessions(); });
afterEach(() => { __resetPendingSessions(); vi.restoreAllMocks(); });

function observe(store: ReturnType<typeof createCockpitStore>) {
  const changed = vi.fn(), unsubscribe = store.subscribe(changed);
  const write = vi.spyOn(Storage.prototype, "setItem");
  return {
    quiet() { expect(changed).not.toHaveBeenCalled(); expect(write).not.toHaveBeenCalled(); },
    oneChange() { expect(changed).toHaveBeenCalledTimes(1); expect(write).toHaveBeenCalledTimes(1); },
    reset() { changed.mockClear(); write.mockClear(); },
    close() { unsubscribe(); write.mockRestore(); },
  };
}

describe("session reconciliation publishes only actual changes", () => {
  it.each([false, true])("keeps an unchanged empty response quiet (known empty index: %s)", known => {
    const store = createCockpitStore();
    if (known) store.getState().replaceSessions([], ["empty"]);
    const before = store.getState(), observation = observe(store);
    try {
      store.getState().replaceSessions([], known ? ["empty"] : []);
      store.getState().replaceSessions([], known ? ["empty"] : []);
      expect(store.getState()).toBe(before);
      observation.quiet();
    } finally { observation.close(); }
  });

  it("keeps freshly decoded identical rows and ordered indexes quiet", () => {
    const store = createCockpitStore();
    const rows = [aSession({ id: "A", worktreeId: "W", executionGeneration: "a".repeat(64) }),
      aSession({ id: "B", worktreeId: "W", reviewedContextDelivery: { status: "captured", version: 1, digest: "d".repeat(64) } })];
    store.getState().replaceSessions(rows, ["W"]);
    const before = store.getState(), observation = observe(store);
    try {
      store.getState().replaceSessions(structuredClone(rows), ["W"]);
      expect(store.getState()).toBe(before);
      expect(store.getState().sessions.A).toBe(rows[0]);
      expect(store.getState().sessionIdsByWorktree.W).toBe(before.sessionIdsByWorktree.W);
      observation.quiet();
    } finally { observation.close(); }
  });

  it("does not publish an uncovered worktree or dictionary insertion-order-only change", () => {
    const store = createCockpitStore();
    const A = aSession({ id: "A", worktreeId: "W-A" }), B = aSession({ id: "B", worktreeId: "W-B" });
    store.getState().replaceSessions([A, B], ["W-A", "W-B"]);
    const before = store.getState(), observation = observe(store);
    try {
      store.getState().replaceSessions([], []);
      store.getState().replaceSessions([structuredClone(B), structuredClone(A)], ["W-B", "W-A"]);
      expect(store.getState()).toBe(before);
      expect(store.getState().sessionIdsByWorktree).toEqual({ "W-A": ["A"], "W-B": ["B"] });
      observation.quiet();
    } finally { observation.close(); }
  });

  const changes: [string, Partial<Session>][] = [
    ["status and exit reason", { status: "exited", exitReason: "process exited" }],
    ["new allocation generation", { executionGeneration: "b".repeat(64) }],
    ["cleared allocation generation", { executionGeneration: undefined }],
    ["created timestamp", { createdAt: "2026-09-18T09:00:01.000Z" }],
    ["updated timestamp", { updatedAt: "2026-09-18T09:00:01.000Z" }],
    ["last output timestamp", { lastOutputAt: "2026-09-18T09:00:01.000Z" }],
    ["attachment count", { attachCount: 3 }],
    ["recorded start refusal", { startFailure: { stage: "admission", reasonCode: "memory_pressure", message: "Retain the recorded refusal." } }],
  ];
  it.each(changes)("publishes a real %s change", (_label, change) => {
    const store = createCockpitStore();
    const initial = aSession({ id: "A", worktreeId: "W", executionGeneration: "a".repeat(64) });
    store.getState().replaceSessions([initial], ["W"]);
    const updated = { ...initial, ...change }, before = store.getState(), observation = observe(store);
    try {
      store.getState().replaceSessions([updated], ["W"]);
      expect(store.getState()).not.toBe(before);
      expect(store.getState().sessions.A).toBe(updated);
      expect(store.getState().sessionIdsByWorktree.W).toEqual(["A"]);
      observation.oneChange();
    } finally { observation.close(); }
  });

  it("publishes additions, covered removals and server order changes while preserving uncovered rows", () => {
    const store = createCockpitStore();
    const A = aSession({ id: "A", worktreeId: "W" }), B = aSession({ id: "B", worktreeId: "W" });
    const foreign = aSession({ id: "foreign", worktreeId: "other" });
    store.getState().replaceSessions([A, foreign], ["W", "other"]);
    const otherIndex = store.getState().sessionIdsByWorktree.other, observation = observe(store);
    try {
      store.getState().replaceSessions([A, B], ["W"]);
      expect(store.getState().sessionIdsByWorktree.W).toEqual(["A", "B"]);
      expect(store.getState().sessions.B).toBe(B); observation.oneChange(); observation.reset();
      store.getState().replaceSessions([B, A], ["W"]);
      expect(store.getState().sessionIdsByWorktree.W).toEqual(["B", "A"]);
      expect(store.getState().sessions.A).toBe(A); observation.oneChange(); observation.reset();
      store.getState().replaceSessions([B], ["W"]);
      expect(store.getState().sessions.A).toBeUndefined();
      expect(store.getState().sessionIdsByWorktree.W).toEqual(["B"]); observation.oneChange(); observation.reset();
      store.getState().replaceSessions([], ["W"]);
      expect(store.getState().sessions.B).toBeUndefined();
      expect(store.getState().sessionIdsByWorktree.W).toEqual([]); observation.oneChange();
      expect(store.getState().sessions.foreign).toBe(foreign);
      expect(store.getState().sessionIdsByWorktree.other).toBe(otherIndex);
    } finally { observation.close(); }
  });

  it("publishes the first authoritative empty index, then leaves repeated emptiness quiet", () => {
    const store = createCockpitStore(), observation = observe(store);
    try {
      store.getState().replaceSessions([], ["newly-covered"]);
      expect(store.getState().sessionIdsByWorktree["newly-covered"]).toEqual([]);
      observation.oneChange(); observation.reset();
      const before = store.getState();
      store.getState().replaceSessions([], ["newly-covered"]);
      expect(store.getState()).toBe(before); observation.quiet();
    } finally { observation.close(); }
  });

  it("preserves a pending Kill against stale running data without publishing it again", () => {
    const store = createCockpitStore();
    const running = aSession({ id: "A", worktreeId: "W" });
    const held = { ...running, status: "exited" as const, exitReason: "killed" };
    store.getState().replaceSessions([held], ["W"]); setSessionPending("A", true);
    const before = store.getState(), observation = observe(store);
    try {
      store.getState().replaceSessions([running], ["W"]);
      expect(store.getState()).toBe(before); expect(store.getState().sessions.A).toBe(held);
      observation.quiet();
      setSessionPending("A", false);
      store.getState().replaceSessions([running], ["W"]);
      expect(store.getState().sessions.A).toBe(running); observation.oneChange();
    } finally { observation.close(); }
  });

  it("does not resurrect a pending Forget and still accepts later fresh server visibility", () => {
    const store = createCockpitStore(), old = aSession({ id: "A", worktreeId: "W" });
    store.getState().replaceSessions([old], ["W"]); store.getState().removeSession("A"); setSessionPending("A", true);
    const before = store.getState(), observation = observe(store);
    try {
      store.getState().replaceSessions([old], ["W"]);
      expect(store.getState()).toBe(before); expect(store.getState().sessions.A).toBeUndefined();
      expect(store.getState().sessionIdsByWorktree.W).toEqual([]); observation.quiet();
      setSessionPending("A", false);
      store.getState().replaceSessions([old], ["W"]);
      expect(store.getState().sessions.A).toBe(old); observation.oneChange();
    } finally { observation.close(); }
  });

  it("retains a pending row's old position when omitted, then honors removal after settlement", () => {
    const store = createCockpitStore();
    const A = aSession({ id: "A", worktreeId: "W" }), held = aSession({ id: "held", worktreeId: "W" });
    const B = aSession({ id: "B", worktreeId: "W" });
    store.getState().replaceSessions([A, held, B], ["W"]); setSessionPending("held", true);
    const before = store.getState(), observation = observe(store);
    try {
      store.getState().replaceSessions([A, B], ["W"]);
      expect(store.getState()).toBe(before); expect(store.getState().sessionIdsByWorktree.W).toEqual(["A", "held", "B"]);
      observation.quiet();
      setSessionPending("held", false);
      store.getState().replaceSessions([A, B], ["W"]);
      expect(store.getState().sessions.held).toBeUndefined();
      expect(store.getState().sessionIdsByWorktree.W).toEqual(["A", "B"]); observation.oneChange();
    } finally { observation.close(); }
  });
});
