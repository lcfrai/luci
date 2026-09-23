// The two commit shapes, the gates, and contract §8's two named regression tests.
//
// vi.useFakeTimers() is what makes the Undo window testable at all: the deferred helper's timer
// and the toast's own timer both read TOAST_TTL_UNDO, so advancing by exactly that value is the
// moment the request is sent, and one millisecond less is the last moment Undo still works.
// Both are asserted.
//
// No jest-dom (shared-shape §3C). Nothing here renders, so there is no DOM to assert against.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { MenuItem } from "../menu/ContextMenu";
import { IDS, aHarness, aSession, aWorktree } from "../../test/fixtures";
import { TOAST_TTL_UNDO } from "../../store/slices/uiSlice";
import { resetCockpit, useCockpit } from "../../store/store";
import type { Session, Toast } from "../../store/types";
import { ApiError } from "../../api/client";

const { killMock, purgeMock, restartMock, renameMock, resumeMock, forkMock, readyMock } = vi.hoisted(
  () => ({
    killMock: vi.fn(),
    purgeMock: vi.fn(),
    restartMock: vi.fn(),
    renameMock: vi.fn(),
    resumeMock: vi.fn(),
    forkMock: vi.fn(),
    readyMock: vi.fn(),
  }),
);
// Only `../../api` is mocked. `../../api/client` is a different specifier and stays real, so
// `ApiError` above is the genuine class and `instanceof` still holds inside the module.
vi.mock("../../api", () => ({
  api: {
    killSession: killMock,
    purgeSession: purgeMock,
    restartSession: restartMock,
    renameSession: renameMock,
    resumeSession: resumeMock,
    forkSession: forkMock,
    setWorktreeReady: readyMock,
  },
}));

// Only the two members sessionVerbs.ts imports. The registry function itself is proven in
// `src/api/attachRegistry.test.ts` against task 19's FakeWebSocket harness — mocking it there
// too would test the verb's wiring and leave the registry proven by nothing.
const { attachPhaseMock, retryAttachMock } = vi.hoisted(() => ({
  attachPhaseMock: vi.fn(() => "detached"),
  retryAttachMock: vi.fn(() => true),
}));
vi.mock("../../api/attachRegistry", () => ({
  getAttachState: (id: string) => ({
    phase: attachPhaseMock(),
    attempts: 0,
    ladderStartedAt: 0,
    reason: "",
    id,
  }),
  retryAttach: retryAttachMock,
}));

import {
  __resetSessionVerbs,
  isSessionPending,
  markWorktreeReady,
  renameSessionTo,
  sessionVerbs,
  worktreeReadyItems,
} from "./sessionVerbs";

const DEPS = { createSession: vi.fn(), close: vi.fn(), beginRename: vi.fn() };
/** The fork target. IDS has one worktree and this suite needs two on the branch. */
const SIBLING = "w-2";
/** The third session in the rename test. IDS carries `session` and `sessionB` only. */
const THIRD = "s-3";
const find = (items: MenuItem[], id: string): MenuItem | undefined => items.find((i) => i.id === id);

/** The newest toast. NOT `toasts.at(-1)`: `Array.prototype.at` is ES2022 and this project's lib
 *  is ES2020 (`tsconfig.json:5`), so that spelling fails `tsc` with TS2550 while passing vitest —
 *  green tests over a tree that does not compile. */
const lastToast = (): Toast | undefined => {
  const { toasts } = useCockpit.getState();
  return toasts[toasts.length - 1];
};

function seed(overrides: Partial<Session> = {}): void {
  // aHarness() IS the shell descriptor (task 15), so only the claude-code entry needs overrides.
  const claude = aHarness({ id: "claude-code", name: "Claude Code", supportsResume: true });
  useCockpit.getState().setHarnesses([claude, aHarness()]);
  useCockpit
    .getState()
    .setWorktrees(IDS.branch, [aWorktree(), aWorktree({ id: SIBLING, name: "retention-rules" })]);
  useCockpit.getState().upsertSession(aSession(overrides));
}

beforeEach(() => {
  vi.useFakeTimers();
  resetCockpit();
  __resetSessionVerbs();
  for (const mock of [killMock, purgeMock, restartMock, renameMock, resumeMock, forkMock, readyMock]) {
    mock.mockReset();
  }
  attachPhaseMock.mockReturnValue("detached");
  retryAttachMock.mockReturnValue(true);
});

afterEach(() => {
  vi.useRealTimers();
});

describe("sessionVerbs — the gates", () => {
  it("offers Forget only on an exited session, and says why when it does not", () => {
    seed({ status: "running" });
    const running = find(sessionVerbs(IDS.session, DEPS), "forget");
    expect(running?.run).toBeUndefined();
    expect(running?.disabledReason).toContain("Kill it first");

    useCockpit.getState().upsertSession(aSession({ status: "exited", exitReason: "killed" }));
    expect(find(sessionVerbs(IDS.session, DEPS), "forget")?.run).toBeTypeOf("function");
  });

  it("offers Kill only while the session is alive", () => {
    seed({ status: "exited" });
    expect(find(sessionVerbs(IDS.session, DEPS), "kill")?.run).toBeUndefined();
    useCockpit.getState().upsertSession(aSession({ status: "idle" }));
    expect(find(sessionVerbs(IDS.session, DEPS), "kill")?.run).toBeTypeOf("function");
  });

  it("offers Resume only when the harness supports it", () => {
    seed({ harness: "claude-code" });
    expect(find(sessionVerbs(IDS.session, DEPS), "resume")?.run).toBeTypeOf("function");
    useCockpit.getState().upsertSession(aSession({ harness: "shell" }));
    const shellResume = find(sessionVerbs(IDS.session, DEPS), "resume");
    expect(shellResume?.run).toBeUndefined();
    expect(shellResume?.disabledReason).toContain("Shell");
  });

  it("renders one Fork row per other worktree on the branch, and disables the verb when there is none", () => {
    seed();
    expect(sessionVerbs(IDS.session, DEPS).map((i) => i.id)).toContain(`fork-${SIBLING}`);
    expect(sessionVerbs(IDS.session, DEPS).map((i) => i.id)).not.toContain(`fork-${IDS.worktree}`);

    useCockpit.getState().setWorktrees(IDS.branch, [aWorktree()]);
    const only = find(sessionVerbs(IDS.session, DEPS), "fork");
    expect(only?.run).toBeUndefined();
    expect(only?.disabledReason).toContain("§10.3");
  });

  // Every other verb closes the host menu on its way out. Rename is the one that must NOT,
  // because in SessionMenu the host is the menu that is about to render the rename input —
  // closing it first unmounts the surface the verb exists to show, and `closeSessionMenu`
  // nulls `sessionMenuFor` so the input never appears. A spy `onClose` cannot see that, which
  // is why the assertion is here on the verb rather than only in the component's suite.
  it("Rename… hands off without closing the host menu, unlike every other verb", () => {
    seed();
    const deps = { createSession: vi.fn(), close: vi.fn(), beginRename: vi.fn() };

    find(sessionVerbs(IDS.session, deps), "rename")?.run?.();
    expect(deps.beginRename).toHaveBeenCalledWith(IDS.session);
    expect(deps.close).not.toHaveBeenCalled();

    find(sessionVerbs(IDS.session, deps), "copy-session-id")?.run?.();
    expect(deps.close).toHaveBeenCalledTimes(1);
  });

  // I3: these three verbs were declared, reset and never asserted — their run bodies had never
  // been entered by any test, so `api.resumeSession(session, harness)` could have been called
  // with the arguments in either order and nothing would have noticed.
  it("Resume creates a continuing session from this one and focuses it", async () => {
    seed({ harness: "claude-code" });
    const resumed = aSession({ id: "s-resumed", title: "Evidence upload" });
    resumeMock.mockResolvedValue(resumed);

    find(sessionVerbs(IDS.session, DEPS), "resume")?.run?.();
    await vi.runOnlyPendingTimersAsync();

    const [source, harness] = resumeMock.mock.calls[0] as [Session, { id: string }];
    expect(source.id).toBe(IDS.session);
    expect(harness.id).toBe("claude-code");
    expect(useCockpit.getState().sessions["s-resumed"]?.id).toBe("s-resumed");
    expect(useCockpit.getState().selection.sessionId).toBe("s-resumed");
  });

  it("Fork creates into the OTHER worktree and focuses the result", async () => {
    seed({ harness: "claude-code" });
    const forked = aSession({ id: "s-forked", worktreeId: SIBLING });
    forkMock.mockResolvedValue(forked);

    find(sessionVerbs(IDS.session, DEPS), `fork-${SIBLING}`)?.run?.();
    await vi.runOnlyPendingTimersAsync();

    const [source, target] = forkMock.mock.calls[0] as [Session, string];
    expect(source.id).toBe(IDS.session);
    // The sibling, never the session's own worktree — the whole point of the fork gate.
    expect(target).toBe(SIBLING);
    expect(useCockpit.getState().selection.sessionId).toBe("s-forked");
  });

  it("Open a shell here goes through the ONE create path, forcing the shell harness", () => {
    seed();
    const deps = { createSession: vi.fn().mockResolvedValue(undefined), close: vi.fn(), beginRename: vi.fn() };

    find(sessionVerbs(IDS.session, deps), "shell")?.run?.();

    // deps.createSession is task 18's useCreateSession, which re-resolves defaults and clamps.
    // This module adds no second create path, so the assertion is on the delegation.
    expect(deps.createSession).toHaveBeenCalledWith(IDS.worktree, { harness: "shell" });
  });

  it("a create verb that the server refuses reports the sentence and focuses nothing", async () => {
    seed({ harness: "claude-code" });
    resumeMock.mockRejectedValue(new ApiError("harness_unavailable", "Claude Code is not installed.", 409));

    find(sessionVerbs(IDS.session, DEPS), "resume")?.run?.();
    await vi.runOnlyPendingTimersAsync();

    expect(lastToast()?.message).toBe("Claude Code is not installed.");
    // No Undo on a create: there is nothing to take back, because nothing was created.
    expect(lastToast()?.undo).toBeUndefined();
    expect(useCockpit.getState().selection.sessionId).toBeNull();
  });

  it("returns a single honest row when the store does not hold the session", () => {
    seed();
    const items = sessionVerbs("s-nope", DEPS);
    expect(items).toHaveLength(1);
    expect(items[0]?.run).toBeUndefined();
  });
});

// The carry this task took as a declared departure: a pane whose attach has failed had no way
// forward from the pane itself, because `failed` is terminal and the exit bar is absent by
// definition — the socket died while the session is NOT exited.
describe("sessionVerbs — Retry attach", () => {
  it("appears only while the attach ladder is spent", () => {
    seed({ status: "running" });
    expect(find(sessionVerbs(IDS.session, DEPS), "retry-attach")).toBeUndefined();

    attachPhaseMock.mockReturnValue("reconnecting");
    expect(find(sessionVerbs(IDS.session, DEPS), "retry-attach")).toBeUndefined();

    attachPhaseMock.mockReturnValue("failed");
    const row = find(sessionVerbs(IDS.session, DEPS), "retry-attach");
    expect(row?.label).toBe("Retry attach");
    expect(row?.run).toBeTypeOf("function");
  });

  it("closes the menu and restarts the ladder, silently when it works", () => {
    seed({ status: "running" });
    attachPhaseMock.mockReturnValue("failed");
    const deps = { createSession: vi.fn(), close: vi.fn(), beginRename: vi.fn() };

    find(sessionVerbs(IDS.session, deps), "retry-attach")?.run?.();

    expect(retryAttachMock).toHaveBeenCalledWith(IDS.session);
    expect(deps.close).toHaveBeenCalledTimes(1);
    expect(useCockpit.getState().toasts).toHaveLength(0);
  });

  it("says so when the registry refuses, rather than no-opping under the click", () => {
    seed({ status: "running" });
    attachPhaseMock.mockReturnValue("failed");
    retryAttachMock.mockReturnValue(false);

    find(sessionVerbs(IDS.session, DEPS), "retry-attach")?.run?.();

    expect(lastToast()?.message).toBe(
      "That pane is no longer attached, so there was nothing to retry.",
    );
    expect(lastToast()?.undo).toBeUndefined();
  });
});

describe("sessionVerbs — the deferred commit and its window", () => {
  it("sends nothing until the window closes, and marks the row pending meanwhile", async () => {
    seed({ status: "running" });
    killMock.mockResolvedValue(aSession({ status: "exited", exitReason: "killed" }));

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("exited");
    expect(isSessionPending(IDS.session)).toBe(true);
    expect(killMock).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO - 1);
    expect(killMock).not.toHaveBeenCalled();

    await vi.advanceTimersByTimeAsync(1);
    expect(killMock).toHaveBeenCalledWith(IDS.session);
    expect(isSessionPending(IDS.session)).toBe(false);
  });

  // Minor 1. `runImmediate` argues at length that the server's object matters — it carries
  // `updatedAt` and an `attachPath` the cockpit cannot derive — and Kill discarded it. That read
  // as harmless only because the 2 s poller overwrote it within a tick, which is exactly what
  // this round's suppression now prevents.
  it("adopts the server's own killed row rather than keeping the guess", async () => {
    seed({ status: "running" });
    killMock.mockResolvedValue(
      aSession({
        status: "exited",
        exitReason: "killed",
        updatedAt: "2026-09-18T09:05:00.000Z",
        attachPath: "/v1/sessions/s-3f2a91c0/attach?replay=1",
      }),
    );

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    // The optimistic guess carries the ORIGINAL timestamps, because it is a spread of the
    // snapshot with two fields changed.
    expect(useCockpit.getState().sessions[IDS.session]?.updatedAt).not.toBe(
      "2026-09-18T09:05:00.000Z",
    );

    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO);

    expect(useCockpit.getState().sessions[IDS.session]?.updatedAt).toBe("2026-09-18T09:05:00.000Z");
    expect(useCockpit.getState().sessions[IDS.session]?.attachPath).toBe(
      "/v1/sessions/s-3f2a91c0/attach?replay=1",
    );
  });

  it("Undo inside the window cancels the request and restores the session", async () => {
    seed({ status: "running" });

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    const toast = lastToast();
    expect(toast?.undo).toBeTypeOf("function");
    toast?.undo?.();

    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("running");
    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO * 2);
    expect(killMock).not.toHaveBeenCalled();
    expect(isSessionPending(IDS.session)).toBe(false);
  });

  it("Undo pressed after the request has gone changes nothing", async () => {
    seed({ status: "running" });
    killMock.mockResolvedValue(aSession({ status: "exited", exitReason: "killed" }));

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    const toast = lastToast();
    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO);
    expect(killMock).toHaveBeenCalledTimes(1);

    // The button is already off the screen; this is the millisecond-wide race, not a
    // reachable click. If `undone` could still be set here the rollback would put a running
    // row back over a session the server has already killed.
    toast?.undo?.();
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("exited");
  });

  it("Forget removes the row now and purges when the window closes", async () => {
    seed({ status: "exited" });
    purgeMock.mockResolvedValue(undefined);

    find(sessionVerbs(IDS.session, DEPS), "forget")?.run?.();
    expect(useCockpit.getState().sessions[IDS.session]).toBeUndefined();

    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO);
    expect(purgeMock).toHaveBeenCalledWith(IDS.session);
  });

  it("a failure AFTER the window closes rolls back, in place, and toasts with no Undo", async () => {
    seed({ status: "exited" });
    // A second session, prepended, so the restored row has a neighbour to be wrong about.
    useCockpit.getState().upsertSession(aSession({ id: IDS.sessionB, title: "Retention rules" }));
    purgeMock.mockRejectedValue(new ApiError("conflict", "The session is still attached.", 409));

    find(sessionVerbs(IDS.session, DEPS), "forget")?.run?.();
    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO);

    expect(useCockpit.getState().sessions[IDS.session]?.id).toBe(IDS.session);
    // Back where it was, not at the top: upsertSession PREPENDS an id the store no longer holds.
    expect(useCockpit.getState().sessionIdsByWorktree[IDS.worktree]).toEqual([
      IDS.sessionB,
      IDS.session,
    ]);
    const last = lastToast();
    expect(last?.message).toBe("The session is still attached.");
    expect(last?.undo).toBeUndefined();
    expect(useCockpit.getState().lastError?.code).toBe("conflict");
    // Nothing was focused when the Forget ran, so the rollback must not focus anything. This
    // pins the OTHER half of the two-condition focus guard: without `wasFocused`, a restore
    // would hand the caret to a session the user had never selected.
    expect(useCockpit.getState().selection.sessionId).toBeNull();
  });

  // Not in the plan, and the plan's rollback loses all three of these. `removeSession` drops the
  // hand-placed tile, the pin and the focus alongside the record, so a rollback that restored
  // only the session list would undo a Forget into a session whose Canvas placement had silently
  // vanished — known defect 6 arriving through a side door, on the COMMON path rather than a
  // rare one, because Undo is the button this whole task exists to offer.
  it("Undo of a Forget restores the pin, the tile and the focus, not just the row", () => {
    seed({ status: "exited" });
    const tile = { x: 120, y: 240, w: 360, h: 280 };
    useCockpit.getState().upsertSession(aSession({ id: IDS.sessionB, title: "Retention rules" }));
    useCockpit.getState().pinSession(IDS.sessionB, [IDS.sessionB, IDS.session]);
    useCockpit.getState().pinSession(IDS.session, [IDS.sessionB, IDS.session]);
    useCockpit.getState().setTile(IDS.session, tile);
    useCockpit.getState().focusSession(IDS.session);

    find(sessionVerbs(IDS.session, DEPS), "forget")?.run?.();
    expect(useCockpit.getState().pinned).toEqual([IDS.sessionB]);
    expect(useCockpit.getState().tiles[IDS.session]).toBeUndefined();
    expect(useCockpit.getState().selection.sessionId).toBeNull();

    lastToast()?.undo?.();

    // At its own index in pin order, not appended: pinSession() cannot express this, which is
    // why the rollback writes the two prefs keys rather than replaying forward actions.
    expect(useCockpit.getState().pinned).toEqual([IDS.sessionB, IDS.session]);
    expect(useCockpit.getState().tiles[IDS.session]).toEqual(tile);
    expect(useCockpit.getState().selection.sessionId).toBe(IDS.session);
  });

  // I4: the departure's HEADLINE claim, and round 1 did not pin it. The earlier test pinned the
  // session LAST, where append and insert-at-index produce the same array — so the one part that
  // was actually argued for, the index, was the one part no assertion could see. Here the
  // forgotten session is pinned FIRST, with two pins after it.
  it("Undo puts the pin back at its index, not on the end", () => {
    seed({ status: "exited" });
    useCockpit.getState().upsertSession(aSession({ id: IDS.sessionB, title: "Retention rules" }));
    useCockpit.getState().upsertSession(aSession({ id: THIRD, title: "Access review" }));
    const scope = [IDS.session, IDS.sessionB, THIRD];
    for (const id of scope) useCockpit.getState().pinSession(id, scope);
    expect(useCockpit.getState().pinned).toEqual(scope);

    find(sessionVerbs(IDS.session, DEPS), "forget")?.run?.();
    expect(useCockpit.getState().pinned).toEqual([IDS.sessionB, THIRD]);

    lastToast()?.undo?.();

    // Appending would give [sessionB, THIRD, session] and Canvas would redraw in a different
    // order than the user left it. This is the assertion the departure was argued for.
    expect(useCockpit.getState().pinned).toEqual(scope);
  });

  it("does not yank focus back when the user moved to another session during the window", () => {
    seed({ status: "exited" });
    useCockpit.getState().upsertSession(aSession({ id: IDS.sessionB, title: "Retention rules" }));
    useCockpit.getState().focusSession(IDS.session);

    find(sessionVerbs(IDS.session, DEPS), "forget")?.run?.();
    useCockpit.getState().focusSession(IDS.sessionB);
    lastToast()?.undo?.();

    expect(useCockpit.getState().sessions[IDS.session]?.id).toBe(IDS.session);
    expect(useCockpit.getState().selection.sessionId).toBe(IDS.sessionB);
  });
});

// I2. `session.status` inside sessionVerbs is the OPTIMISTICALLY PATCHED status, so during a
// Kill window the row already reads `exited` — which is exactly the condition that enables
// Forget. The two would stack, and the Kill's rollback would then upsert a row the Forget had
// removed, prepending it to the top of the worktree.
describe("sessionVerbs — one undoable change at a time", () => {
  it("does not offer Forget while a Kill is still undoable", () => {
    seed({ status: "running" });
    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();

    // The patched row reads `exited`, which is what used to enable Forget.
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("exited");
    const forget = find(sessionVerbs(IDS.session, DEPS), "forget");
    expect(forget?.run).toBeUndefined();
    expect(forget?.disabledReason).toBe("Another change to this session has not finished yet.");
    // And Kill does not re-arm either: a second Kill would hold a second request.
    expect(find(sessionVerbs(IDS.session, DEPS), "kill")?.run).toBeUndefined();
  });

  it("the row survives the window with no duplicate and no reorder", async () => {
    seed({ status: "running" });
    useCockpit.getState().upsertSession(aSession({ id: IDS.sessionB, title: "Retention rules" }));
    const order = [IDS.sessionB, IDS.session];
    expect(useCockpit.getState().sessionIdsByWorktree[IDS.worktree]).toEqual(order);
    killMock.mockResolvedValue(aSession({ status: "exited", exitReason: "killed" }));

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    // The Forget that used to be reachable here is now disabled, so nothing stacks.
    find(sessionVerbs(IDS.session, DEPS), "forget")?.run?.();
    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO);

    expect(useCockpit.getState().sessionIdsByWorktree[IDS.worktree]).toEqual(order);
    expect(Object.keys(useCockpit.getState().sessions).sort()).toEqual([...order].sort());
    expect(purgeMock).not.toHaveBeenCalled();
  });

  // The Kill gate's `!inFlight` half needs a session that is ALIVE and pending, which a Kill
  // can never produce — it patches the row to `exited`, so `alive` is already false and the two
  // conditions cannot be told apart. The reachable case is an IMMEDIATE verb: `runImmediate`
  // marks the session pending too, and Restart/Rename leave it alive. Without this test the
  // half is unobservable and a mutant deleting it survives.
  it("does not offer Kill while an immediate verb is still in flight", () => {
    seed({ status: "running" });
    // A request that never settles, so the session stays alive AND pending.
    renameMock.mockReturnValue(new Promise(() => undefined));

    renameSessionTo(IDS.session, "retention rules");
    expect(isSessionPending(IDS.session)).toBe(true);
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("running");

    const kill = find(sessionVerbs(IDS.session, DEPS), "kill");
    expect(kill?.run).toBeUndefined();
    expect(kill?.disabledReason).toBe("Another change to this session has not finished yet.");
  });

  it("re-offers both verbs once the window has closed", async () => {
    seed({ status: "running" });
    killMock.mockResolvedValue(aSession({ status: "exited", exitReason: "killed" }));

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO);

    // The gate is the in-flight window, not a permanent lock — Forget is the whole point of
    // killing something first.
    expect(isSessionPending(IDS.session)).toBe(false);
    expect(find(sessionVerbs(IDS.session, DEPS), "forget")?.run).toBeTypeOf("function");
  });
});

// I1. The poller replaces sessions every 2 s; a deferred verb holds its request for 6 s. Without
// suppression the row returns about two seconds into the window, with the Undo toast still up.
// The suppression lives in `replaceSessions` so every deferred verb gets it for free; these
// tests drive the store action directly, which is what the poller calls.
describe("the 2 s poller cannot undo a deferred verb inside its window", () => {
  /** What the server still reports, because the request has not been sent. */
  const serverRows = (): Session[] => [
    aSession({ id: IDS.sessionB, title: "Retention rules" }),
    aSession({ status: "running" }),
  ];

  it("a killed row keeps its optimistic status across a poll", async () => {
    seed({ status: "running" });
    useCockpit.getState().upsertSession(aSession({ id: IDS.sessionB, title: "Retention rules" }));
    killMock.mockResolvedValue(aSession({ status: "exited", exitReason: "killed" }));

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    useCockpit.getState().replaceSessions(serverRows(), [IDS.worktree]);

    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("exited");
    expect(useCockpit.getState().sessions[IDS.session]?.exitReason).toBe("killed");
    // Its neighbour, which is NOT pending, still takes the server's row.
    expect(useCockpit.getState().sessions[IDS.sessionB]?.title).toBe("Retention rules");
    expect(useCockpit.getState().sessionIdsByWorktree[IDS.worktree]).toEqual([
      IDS.sessionB,
      IDS.session,
    ]);

    // And Undo still restores, because nothing was sent and nothing overwrote the snapshot.
    lastToast()?.undo?.();
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("running");
    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO * 2);
    expect(killMock).not.toHaveBeenCalled();
  });

  it("a forgotten row stays gone across a poll, and out of the index", () => {
    seed({ status: "exited" });
    useCockpit.getState().upsertSession(aSession({ id: IDS.sessionB, title: "Retention rules" }));

    find(sessionVerbs(IDS.session, DEPS), "forget")?.run?.();
    useCockpit.getState().replaceSessions(serverRows(), [IDS.worktree]);

    expect(useCockpit.getState().sessions[IDS.session]).toBeUndefined();
    // The index matters as much as the map: an id here with no record renders a row the session
    // map cannot fill.
    expect(useCockpit.getState().sessionIdsByWorktree[IDS.worktree]).toEqual([IDS.sessionB]);
  });

  it("stops suppressing the moment the window closes", async () => {
    seed({ status: "running" });
    killMock.mockResolvedValue(aSession({ status: "exited", exitReason: "killed" }));

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    await vi.advanceTimersByTimeAsync(TOAST_TTL_UNDO);
    expect(isSessionPending(IDS.session)).toBe(false);

    // The server is authoritative again. A suppression that outlived the window would freeze the
    // row for ever, which is a worse failure than the one it fixes.
    useCockpit.getState().replaceSessions([aSession({ status: "idle" })], [IDS.worktree]);
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("idle");
  });

  it("holds a pending row the server has stopped listing, rather than orphaning it", () => {
    seed({ status: "running" });
    useCockpit.getState().upsertSession(aSession({ id: IDS.sessionB, title: "Retention rules" }));

    find(sessionVerbs(IDS.session, DEPS), "kill")?.run?.();
    // Another client purged it, or the reaper did. The row is held either way: present in the
    // map AND in the index, because present-but-unindexed renders nowhere and Undo cannot
    // repair it — upsertSession leaves an existing id's position alone, and there would be none.
    useCockpit
      .getState()
      .replaceSessions([aSession({ id: IDS.sessionB, title: "Retention rules" })], [IDS.worktree]);

    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("exited");
    expect(useCockpit.getState().sessionIdsByWorktree[IDS.worktree]).toEqual([
      IDS.sessionB,
      IDS.session,
    ]);
  });
});

describe("sessionVerbs — the immediate commit", () => {
  it("Restart patches to starting at once and adopts the server's answer", async () => {
    seed({ status: "exited", exitReason: "process exited" });
    restartMock.mockResolvedValue(aSession({ status: "running", exitReason: "" }));

    find(sessionVerbs(IDS.session, DEPS), "restart")?.run?.();
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("starting");
    expect(useCockpit.getState().sessions[IDS.session]?.exitReason).toBe("");

    await vi.runOnlyPendingTimersAsync();
    expect(restartMock).toHaveBeenCalledWith(IDS.session);
    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("running");
  });

  it("a refused Restart rolls the snapshot back and renders the server's sentence verbatim", async () => {
    seed({ status: "exited", exitReason: "process exited" });
    restartMock.mockRejectedValue(
      new ApiError("non_loopback_publish", "The sandbox published 8080 off loopback.", 500),
    );

    find(sessionVerbs(IDS.session, DEPS), "restart")?.run?.();
    await vi.runOnlyPendingTimersAsync();

    expect(useCockpit.getState().sessions[IDS.session]?.status).toBe("exited");
    expect(useCockpit.getState().sessions[IDS.session]?.exitReason).toBe("process exited");
    expect(lastToast()?.message).toBe(
      "The sandbox published 8080 off loopback.",
    );
  });

  it("renaming the middle session leaves its neighbours unchanged", async () => {
    seed();
    useCockpit.getState().upsertSession(aSession({ id: IDS.sessionB, title: "Retention rules" }));
    useCockpit.getState().upsertSession(aSession({ id: THIRD, title: "Access review" }));
    renameMock.mockResolvedValue(aSession({ id: IDS.sessionB, title: "retention rules" }));

    renameSessionTo(IDS.sessionB, "retention rules");
    await vi.runOnlyPendingTimersAsync();

    expect(renameMock).toHaveBeenCalledWith(IDS.sessionB, "retention rules");
    expect(useCockpit.getState().sessions[IDS.sessionB]?.title).toBe("retention rules");
    expect(useCockpit.getState().sessions[THIRD]?.title).toBe("Access review");
    expect(useCockpit.getState().sessions[IDS.session]?.title).toBe("Evidence upload");
    expect(useCockpit.getState().sessionIdsByWorktree[IDS.worktree]).toEqual([
      THIRD,
      IDS.sessionB,
      IDS.session,
    ]);
  });
});

// §4.2's non-collapse rule, exercised through a verb rather than against the private helper.
// Untested, the two local sentences would be free to decay into `error.message` — which reads
// "Failed to fetch" for a dead control plane and sends the user nowhere.
describe("sessionVerbs — the failure sentence", () => {
  it("names the control plane and Docker separately, and renders every other code verbatim", async () => {
    seed({ status: "exited", exitReason: "process exited" });

    restartMock.mockRejectedValue(new ApiError("network_unavailable", "Failed to fetch", 0));
    find(sessionVerbs(IDS.session, DEPS), "restart")?.run?.();
    await vi.runOnlyPendingTimersAsync();
    expect(lastToast()?.message).toBe(
      "The Panacea control plane is not running, so nothing was changed.",
    );

    restartMock.mockRejectedValue(new ApiError("backend_unavailable", "docker: no such host", 503));
    find(sessionVerbs(IDS.session, DEPS), "restart")?.run?.();
    await vi.runOnlyPendingTimersAsync();
    expect(lastToast()?.message).toBe(
      "Docker is not running on this machine, so nothing was changed.",
    );

    // The sixteenth code. Its sandbox IS running, so nothing may paraphrase it.
    restartMock.mockRejectedValue(
      new ApiError("sandbox_not_running", "The sandbox is not running.", 409),
    );
    find(sessionVerbs(IDS.session, DEPS), "restart")?.run?.();
    await vi.runOnlyPendingTimersAsync();
    expect(lastToast()?.message).toBe("The sandbox is not running.");
  });
});

describe("markWorktreeReady and §10.2's verb", () => {
  it("Mark ready PATCHes the worktree and the flag survives the next list", async () => {
    seed();
    const w = aWorktree({ ready: false });
    useCockpit.getState().setWorktrees(IDS.branch, [w]);
    readyMock.mockResolvedValue({ ...w, ready: true });

    markWorktreeReady(w);
    expect(useCockpit.getState().worktreesByBranch[IDS.branch]?.[0]?.ready).toBe(true);

    await vi.runOnlyPendingTimersAsync();
    expect(readyMock).toHaveBeenCalledWith(IDS.project, IDS.worktree, true);
    // "Survives the next list" is the second half of contract §8 defect 9: what the store
    // holds afterwards is the SERVER's object, so the next poll cannot un-set the flag.
    expect(useCockpit.getState().worktreesByBranch[IDS.branch]?.[0]?.ready).toBe(true);
  });

  it("rolls the flag back when the server refuses", async () => {
    seed();
    const w = aWorktree({ ready: false });
    useCockpit.getState().setWorktrees(IDS.branch, [w]);
    readyMock.mockRejectedValue(new ApiError("worktree_not_found", "No such worktree.", 404));

    markWorktreeReady(w);
    await vi.runOnlyPendingTimersAsync();

    expect(useCockpit.getState().worktreesByBranch[IDS.branch]?.[0]?.ready).toBe(false);
    expect(lastToast()?.message).toBe("No such worktree.");
  });

  // The rollback goes through `replaceById` against the LIVE list, not a snapshot taken before
  // the request. A snapshot restore would put the branch back as it was and discard whatever
  // the 2 s poll added while the PATCH was in flight — this task's plan argued the opposite and
  // had it backwards.
  it("a refusal reverts the flag without discarding a worktree the poll added meanwhile", async () => {
    seed();
    const mine = aWorktree({ ready: false });
    useCockpit.getState().setWorktrees(IDS.branch, [mine]);
    let refuse: (error: unknown) => void = () => undefined;
    readyMock.mockReturnValue(
      new Promise((_resolve, reject) => {
        refuse = reject;
      }),
    );

    markWorktreeReady(mine);
    // The poll lands mid-request and the branch grows a worktree.
    useCockpit
      .getState()
      .setWorktrees(IDS.branch, [
        ...useCockpit.getState().worktreesByBranch[IDS.branch],
        aWorktree({ id: SIBLING, name: "retention-rules", ready: false }),
      ]);

    refuse(new ApiError("worktree_not_found", "No such worktree.", 404));
    await vi.runOnlyPendingTimersAsync();

    const list = useCockpit.getState().worktreesByBranch[IDS.branch];
    expect(list?.map((w) => w.id)).toEqual([IDS.worktree, SIBLING]);
    expect(list?.[0]?.ready).toBe(false);
  });

  it("marks one worktree in a list and keeps every other one", async () => {
    seed();
    const first = aWorktree({ ready: false });
    const second = aWorktree({ id: SIBLING, name: "retention-rules", ready: false });
    useCockpit.getState().setWorktrees(IDS.branch, [first, second]);
    readyMock.mockResolvedValue({ ...second, ready: true });

    markWorktreeReady(second);
    await vi.runOnlyPendingTimersAsync();

    // Contract §8 defect 7's other half, verbatim: "replacing one worktree in a list".
    const list = useCockpit.getState().worktreesByBranch[IDS.branch];
    expect(list?.map((w) => w.id)).toEqual([IDS.worktree, SIBLING]);
    expect(list?.[0]?.ready).toBe(false);
    expect(list?.[1]?.ready).toBe(true);
  });

  // Contract §8 defect 9 as amended 2026-09-23: §10.2 LANDED, `worktreeJSON.Ready` has no
  // `omitempty`, so the flag is ALWAYS on the wire. The test this row used to name — "the verb
  // is absent when the server does not report ready" — is DELETED rather than moved, because it
  // asserted a shape the control plane cannot produce. This is what replaces it.
  it("renders the row unconditionally, and disables it once the worktree is already ready", () => {
    const close = vi.fn();
    const shown = worktreeReadyItems(aWorktree({ ready: false }), close);
    expect(shown).toHaveLength(1);
    expect(shown[0]?.id).toBe("mark-ready");
    expect(shown[0]?.label).toBe("Mark ready");
    expect(shown[0]?.run).toBeTypeOf("function");

    const already = worktreeReadyItems(aWorktree({ ready: true }), close);
    expect(already).toHaveLength(1);
    expect(already[0]?.run).toBeUndefined();
    expect(already[0]?.disabledReason).toContain("Already marked ready");
  });

  // Minor 3. `replaceById` returns a copy for an id it cannot find, and `selectWorktrees`
  // returns the shared empty constant for a branch the store does not hold — so an unguarded
  // write turns a failed lookup into `worktreesByBranch[branchId] = []`, a branch that exists
  // and is empty where there was no key at all. The rail would then draw a branch with no
  // worktrees rather than one that has simply not loaded.
  it("does not invent a branch when the worktree is not in the store", async () => {
    seed();
    const orphan = aWorktree({ id: "w-orphan", branchId: "b-never-loaded", ready: false });
    readyMock.mockResolvedValue({ ...orphan, ready: true });

    markWorktreeReady(orphan);
    await vi.runOnlyPendingTimersAsync();

    expect(useCockpit.getState().worktreesByBranch["b-never-loaded"]).toBeUndefined();
    expect(Object.keys(useCockpit.getState().worktreesByBranch)).toEqual([IDS.branch]);
  });

  it("closes the host menu before it mutates, so the Undo toast is not hidden behind it", () => {
    seed();
    const close = vi.fn();
    useCockpit.getState().setWorktrees(IDS.branch, [aWorktree({ ready: false })]);
    readyMock.mockResolvedValue(aWorktree({ ready: true }));

    worktreeReadyItems(aWorktree({ ready: false }), close)[0]?.run?.();
    expect(close).toHaveBeenCalledTimes(1);
  });
});
