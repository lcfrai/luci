import { beforeEach, describe, expect, it, vi } from "vitest";
import { aBranch, aHarness, aProject, aSession } from "../test/fixtures";
import {
  createCockpitStore,
  migrate,
  partialize,
  PERSIST_KEY,
  PERSIST_VERSION,
  resetCockpit,
  useCockpit,
} from "./store";
import { APP_DEFAULTS, CANVAS_MAX_TILES, initialPrefsState } from "./slices/prefsSlice";
import { initialDataState } from "./slices/dataSlice";
import { initialUiState, MAX_TOASTS, TOAST_TTL_PLAIN, TOAST_TTL_UNDO } from "./slices/uiSlice";
import type { Session } from "./types";

/* Three harnesses, so `setHarnesses` has something with a length to assert. Composed at the
 * call site from `aHarness` rather than added to `fixtures.ts` — shared-shape §3.2 sanctions
 * six builders and no seventh. Nothing in this suite reads a field of them. */
const CATALOGUE = [aHarness({ id: "claude-code" }), aHarness({ id: "shell" }), aHarness({ id: "codex" })];

/* Every session this suite builds pins its own `worktreeId`: `aSession()`'s default comes from
 * task 14's `IDS`, not from "w-1", and the assertions below index the worktree map by hand. */
const sessionsOn = (worktreeId: string, ids: string[]): Session[] =>
  ids.map((id) => aSession({ id, worktreeId, attachPath: `/v1/sessions/${id}/attach` }));

beforeEach(() => {
  localStorage.clear();
  resetCockpit();
});

describe("dataSlice", () => {
  it("clears a vanished project only after a successful catalogue result", () => {
    const store = useCockpit.getState();
    store.selectBranch("missing-project", "old-branch");
    store.setLoading("projects", true);
    store.setError({ message: "Connection unavailable", code: "offline" });
    expect(useCockpit.getState().selection.projectId).toBe("missing-project");
    expect(useCockpit.getState().projectsLoaded).toBe(false);
    store.setProjects([aProject({ id: "another-project" })]);
    expect(useCockpit.getState().selection).toEqual({ projectId: null, branchId: null, worktreeId: null, sessionId: null });
    expect(useCockpit.getState().projectsLoaded).toBe(true);
  });

  it("preserves all current navigation when the project remains in the catalogue", () => {
    const store = useCockpit.getState();
    store.selectBranch("p-1", "b-1");
    const selected = useCockpit.getState().selection;
    store.setProjects([aProject({ id: "p-1" })]);
    expect(useCockpit.getState().selection).toBe(selected);
  });
  it("keeps catalogue setters independent of one another", () => {
    const s = useCockpit.getState();
    s.setProjects([aProject({ id: "p-1", name: "luci", path: "C:\\work\\luci" })]);
    s.setBranches("p-1", [aBranch({ id: "b-1", projectId: "p-1", name: "main", isHead: true })]);
    s.setHarnesses(CATALOGUE);
    expect(useCockpit.getState().projects).toHaveLength(1);
    expect(useCockpit.getState().branchesByProject["p-1"]).toHaveLength(1);
    expect(useCockpit.getState().harnesses).toHaveLength(3);
    expect(useCockpit.getState().machines).toEqual([]);
  });

  it("rebuilds the index in server order and empties a worktree that lost its sessions", () => {
    const s = useCockpit.getState();
    s.replaceSessions([...sessionsOn("w-1", ["s-1", "s-2"]), ...sessionsOn("w-2", ["s-3"])],
      ["w-1", "w-2"]);
    expect(useCockpit.getState().sessionIdsByWorktree["w-1"]).toEqual(["s-1", "s-2"]);
    s.replaceSessions(sessionsOn("w-2", ["s-3"]), ["w-1", "w-2"]);
    expect(useCockpit.getState().sessionIdsByWorktree["w-1"]).toEqual([]);
    expect(useCockpit.getState().sessions["s-1"]).toBeUndefined();
    expect(useCockpit.getState().sessions["s-3"]).toBeDefined();
  });

  it("covers a worktree the caller did not list but an incoming row names", () => {
    const s = useCockpit.getState();
    // Every other call in this suite lists every row's worktree, so none of them exercises
    // the `covered.add(row.worktreeId)` line — and without it the index build dereferences a
    // bucket that was never created and throws a TypeError inside the 2 s poller, the one
    // code path that runs continuously.
    s.replaceSessions(sessionsOn("w-3", ["s-4"]), []);
    expect(useCockpit.getState().sessionIdsByWorktree["w-3"]).toEqual(["s-4"]);
    expect(useCockpit.getState().sessions["s-4"]).toBeDefined();
  });

  it("leaves a worktree the fetch did not cover completely alone", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-9", ["s-9"]), ["w-9"]);
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    expect(useCockpit.getState().sessionIdsByWorktree["w-9"]).toEqual(["s-9"]);
    expect(useCockpit.getState().sessions["s-9"]).toBeDefined();
  });

  it("keeps the id array identical when a poll changed nothing", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1", "s-2"]), ["w-1"]);
    const before = useCockpit.getState().sessionIdsByWorktree["w-1"];
    s.replaceSessions(sessionsOn("w-1", ["s-1", "s-2"]), ["w-1"]);
    expect(useCockpit.getState().sessionIdsByWorktree["w-1"]).toBe(before);
  });

  it("keeps the session object identical when every field matches", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    const before = useCockpit.getState().sessions["s-1"];
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    expect(useCockpit.getState().sessions["s-1"]).toBe(before);
  });

  it("preserves an equal implementation snapshot across polls and notices changed binding fields or arrays", () => {
    const context = { projectId: "p-1", briefRevision: 2, planRevision: 3, specificationRevision: 6, designPlanRevision: 8, taskIds: ["task-1"], briefDigest: "brief-2", planDigest: "plan-3", specificationDigest: "spec-6", designPlanDigest: "design-8", worktreeId: "w-1", sourceCommit: "a".repeat(40), sourceDigest: "source-a", sourceDirty: false, taskTitles: ["Build editor"], requirementIds: ["R-EDIT"] };
    const original = aSession({ id: "s-bound", worktreeId: "w-1", implementationContext: context });
    const s = useCockpit.getState();
    s.replaceSessions([original], ["w-1"]);
    s.replaceSessions([structuredClone(original)], ["w-1"]);
    expect(useCockpit.getState().sessions["s-bound"]).toBe(original);
    for (const changed of [{ sourceDigest: "changed-source" }, { taskIds: ["task-2"] }, { taskTitles: ["Revised editor" ] }, { requirementIds: [] }]) {
      s.replaceSessions([original], ["w-1"]);
      const updated = { ...original, implementationContext: { ...context, ...changed } };
      s.replaceSessions([updated], ["w-1"]);
      expect(useCockpit.getState().sessions["s-bound"]).toBe(updated);
    }
    s.replaceSessions([{ ...original, implementationContext: undefined }], ["w-1"]);
    expect(useCockpit.getState().sessions["s-bound"].implementationContext).toBeUndefined();
  });

  it("keeps equal native implementation outcomes stable and publishes a changed phase", () => {
    const original = aSession({ id: "s-outcome", worktreeId: "w-1", implementationOutcome: { state: "quiet", evidence: "session-only" } });
    const s = useCockpit.getState();
    s.replaceSessions([original], ["w-1"]);
    s.replaceSessions([structuredClone(original)], ["w-1"]);
    expect(useCockpit.getState().sessions[original.id]).toBe(original);
    const ended = { ...original, status: "exited" as const, implementationOutcome: { state: "ended" as const, evidence: "session-only" as const } };
    s.replaceSessions([ended], ["w-1"]);
    expect(useCockpit.getState().sessions[original.id]).toBe(ended);
    const observed = { ...original, implementationOutcome: { state: "quiet" as const, evidence: "session-only" as const, nativeTurnId: "msg_67d5b111", nativeToolClass: "reads-only" as const } };
    s.replaceSessions([observed], ["w-1"]);
    expect(useCockpit.getState().sessions[original.id]).toBe(observed);
  });

  it("keeps equal delivery receipts stable and surfaces changed or missing delivery across polls", () => {
    const original = aSession({ id: "s-delivery", worktreeId: "w-1", reviewedContextDelivery: { status: "captured", version: 1, digest: "a".repeat(64) } });
    const s = useCockpit.getState();
    s.replaceSessions([original], ["w-1"]);
    s.replaceSessions([structuredClone(original)], ["w-1"]);
    expect(useCockpit.getState().sessions[original.id]).toBe(original);
    for (const receipt of [{ status: "legacy-live" as const }, { status: "captured" as const, version: 1, digest: "b".repeat(64) }, undefined]) {
      const updated = { ...original, reviewedContextDelivery: receipt };
      s.replaceSessions([updated], ["w-1"]);
      expect(useCockpit.getState().sessions[original.id]).toBe(updated);
    }
  });

  it("replaces the session object when a field actually changed", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    const before = useCockpit.getState().sessions["s-1"];
    s.replaceSessions(
      [aSession({ id: "s-1", worktreeId: "w-1", attachPath: "/v1/sessions/s-1/attach", status: "idle" })],
      ["w-1"],
    );
    expect(useCockpit.getState().sessions["s-1"]).not.toBe(before);
    expect(useCockpit.getState().sessions["s-1"].status).toBe("idle");
  });

  it("upserts a known id without moving it — a poll moves zero pixels", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1", "s-2", "s-3"]), ["w-1"]);
    s.upsertSession(
      aSession({ id: "s-3", worktreeId: "w-1", status: "exited", exitReason: "killed" }),
    );
    expect(useCockpit.getState().sessionIdsByWorktree["w-1"]).toEqual(["s-1", "s-2", "s-3"]);
    expect(useCockpit.getState().sessions["s-3"].exitReason).toBe("killed");
  });

  it("prepends an unknown id, because server order is createdAt descending", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    s.upsertSession(aSession({ id: "s-new", worktreeId: "w-1" }));
    expect(useCockpit.getState().sessionIdsByWorktree["w-1"]).toEqual(["s-new", "s-1"]);
  });

  it("notifies no subscriber when an upsert changed nothing", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    const seen = vi.fn();
    const unsubscribe = useCockpit.subscribe(seen);
    s.upsertSession(aSession({ id: "s-1", worktreeId: "w-1", attachPath: "/v1/sessions/s-1/attach" }));
    unsubscribe();
    expect(seen).not.toHaveBeenCalled();
  });
});

describe("removeSession and lastError", () => {
  it("purges the record, the index entry, the tile, the pin, the focus and the menus", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1", "s-2"]), ["w-1"]);
    s.setTile("s-1", { x: 10, y: 20, w: 400, h: 300 });
    s.pinSession("s-1", ["s-1"]);
    s.focusSession("s-1");
    s.openSessionMenu("s-1");
    s.openContextMenu({ kind: "session", id: "s-1", x: 40, y: 80 });
    s.removeSession("s-1");
    const after = useCockpit.getState();
    expect(after.sessions["s-1"]).toBeUndefined();
    expect(after.sessionIdsByWorktree["w-1"]).toEqual(["s-2"]);
    expect(after.tiles["s-1"]).toBeUndefined();
    expect(after.pinned).toEqual([]);
    expect(after.selection.sessionId).toBeNull();
    // Task 18 renders both from an id; a menu left open over a forgotten session is a
    // surface reading a dangling reference.
    expect(after.sessionMenuFor).toBeNull();
    expect(after.contextMenu).toBeNull();
  });

  it("leaves a focus and a menu that pointed elsewhere alone", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1", "s-2"]), ["w-1"]);
    s.focusSession("s-2");
    s.openSessionMenu("s-2");
    // A BRANCH menu whose id happens to equal the removed session's. Clearing on the id
    // alone would close it, so the kind is what makes the guard correct rather than lucky.
    s.openContextMenu({ kind: "branch", id: "s-1", x: 0, y: 0 });
    s.removeSession("s-1");
    const after = useCockpit.getState();
    expect(after.selection.sessionId).toBe("s-2");
    expect(after.sessionMenuFor).toBe("s-2");
    expect(after.contextMenu).toEqual({ kind: "branch", id: "s-1", x: 0, y: 0 });
  });

  it("leaves a context menu open over a DIFFERENT session alone", () => {
    const s = useCockpit.getState();
    s.replaceSessions(sessionsOn("w-1", ["s-1", "s-2"]), ["w-1"]);
    // The guard on contextMenu is a three-way conjunction, so it has two degenerate
    // collapses and the branch-kind test above pins only one. This is the other: a menu of
    // the RIGHT kind over the WRONG session. Dropping the id check closes it, and nothing
    // in that test can tell, because its menu is a branch.
    s.openContextMenu({ kind: "session", id: "s-2", x: 5, y: 6 });
    s.removeSession("s-1");
    expect(useCockpit.getState().contextMenu).toEqual({
      kind: "session",
      id: "s-2",
      x: 5,
      y: 6,
    });
  });

  it("is clearable only by setError(null), never by a later successful fetch", () => {
    const s = useCockpit.getState();
    s.setError({ code: "unsupported_option", message: "effort is not accepted by shell" });
    s.setMachines([]);
    s.replaceSessions([], []);
    expect(useCockpit.getState().lastError?.code).toBe("unsupported_option");
    s.setError(null);
    expect(useCockpit.getState().lastError).toBeNull();
  });

  it("carries the sixteenth error code without a store change", () => {
    useCockpit.getState().setError({ code: "non_loopback_publish", message: "published off loopback" });
    expect(useCockpit.getState().lastError?.code).toBe("non_loopback_publish");
  });

  it("tracks a per-entity load key", () => {
    const s = useCockpit.getState();
    s.setLoading("worktrees:b-1", true);
    expect(useCockpit.getState().loading["worktrees:b-1"]).toBe(true);
    s.setLoading("worktrees:b-1", false);
    expect(useCockpit.getState().loading["worktrees:b-1"]).toBe(false);
  });
});

describe("uiSlice", () => {
  it("sends selectBranch to develop and clears the deeper selection", () => {
    const s = useCockpit.getState();
    s.setLens("merge");
    s.selectWorktree("w-1");
    s.focusSession("s-1");
    s.selectBranch("p-1", "b-2");
    const after = useCockpit.getState();
    expect(after.lens).toBe("develop");
    expect(after.selection).toEqual({
      projectId: "p-1",
      branchId: "b-2",
      worktreeId: null,
      sessionId: null,
    });
  });

  it("clears the branch, the worktree and the session when a project is selected", () => {
    const s = useCockpit.getState();
    s.selectBranch("p-1", "b-1");
    s.selectWorktree("w-1");
    s.focusSession("s-1");
    s.selectProject("p-2");
    expect(useCockpit.getState().selection).toEqual({
      projectId: "p-2",
      branchId: null,
      worktreeId: null,
      sessionId: null,
    });
  });

  it("keeps the focused session when a worktree is selected — focus is not navigation", () => {
    const s = useCockpit.getState();
    s.selectBranch("p-1", "b-1");
    s.focusSession("s-1");
    s.selectWorktree("w-2");
    const after = useCockpit.getState();
    expect(after.selection.worktreeId).toBe("w-2");
    expect(after.selection.sessionId).toBe("s-1");
  });

  it("focuses a session without changing the branch", () => {
    const s = useCockpit.getState();
    s.selectBranch("p-1", "b-1");
    s.focusSession("s-7");
    expect(useCockpit.getState().selection.branchId).toBe("b-1");
    expect(useCockpit.getState().selection.sessionId).toBe("s-7");
  });

  it("expands an absent key on the first toggle and collapses on the second", () => {
    const s = useCockpit.getState();
    expect(useCockpit.getState().expandedProjects["p-1"]).toBeUndefined();
    s.toggleProject("p-1");
    expect(useCockpit.getState().expandedProjects["p-1"]).toBe(true);
    s.toggleProject("p-1");
    expect(useCockpit.getState().expandedProjects["p-1"]).toBe(false);
  });

  it("closes one overlay without closing the others", () => {
    const s = useCockpit.getState();
    s.openPalette();
    s.openPicker();
    s.openContextMenu({ kind: "session", id: "s-1", x: 40, y: 80 });
    s.openSessionMenu("s-1");
    s.closePicker();
    const after = useCockpit.getState();
    expect(after.paletteOpen).toBe(true);
    expect(after.pickerOpen).toBe(false);
    expect(after.contextMenu?.id).toBe("s-1");
    expect(after.sessionMenuFor).toBe("s-1");
  });
});

describe("toasts", () => {
  it("gives an undoable toast 6000 ms and a plain one 3200 ms", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-18T09:00:00.000Z"));
    const s = useCockpit.getState();
    s.pushToast("Session killed", () => {});
    s.pushToast("Defaults saved");
    const [undoable, plain] = useCockpit.getState().toasts;
    expect(undoable.ttl).toBe(TOAST_TTL_UNDO);
    expect(undoable.createdAt).toBe(Date.parse("2026-09-18T09:00:00.000Z"));
    expect(plain.ttl).toBe(TOAST_TTL_PLAIN);
    expect(plain.undo).toBeUndefined();
    vi.useRealTimers();
  });

  it("dismisses by id and caps the column at MAX_TOASTS, dropping the oldest", () => {
    const s = useCockpit.getState();
    for (let i = 0; i < MAX_TOASTS + 2; i += 1) s.pushToast(`toast ${i}`);
    const toasts = useCockpit.getState().toasts;
    expect(toasts).toHaveLength(MAX_TOASTS);
    expect(toasts[0].message).toBe("toast 2");
    s.dismissToast(toasts[0].id);
    expect(useCockpit.getState().toasts).toHaveLength(MAX_TOASTS - 1);
  });
});

describe("prefsSlice", () => {
  it("starts at the documented APP_DEFAULTS", () => {
    expect(APP_DEFAULTS).toEqual({
      harness: "claude-code",
      model: "",
      effort: "",
      permissionMode: "ask",
    });
    expect(useCockpit.getState().defaults).toEqual(APP_DEFAULTS);
  });

  it("patches app defaults field by field and leaves the rest standing", () => {
    useCockpit.getState().setDefaults({ effort: "high" });
    expect(useCockpit.getState().defaults).toEqual({ ...APP_DEFAULTS, effort: "high" });
    // A patch key present with an `undefined` value must not punch a hole in the bottom of
    // the inheritance chain. The assertion above cannot see the difference — a
    // `{ ...defaults, ...patch }` spread satisfies it — and this one can.
    useCockpit.getState().setDefaults({ effort: undefined });
    expect(useCockpit.getState().defaults.effort).toBe("high");
  });

  it("preserves an omitted connection patch and clears an explicitly undefined app connection", () => {
    const state = useCockpit.getState();
    state.setDefaults({ agentConnectionId: "gateway-one", model: "chosen-model", effort: "high" });
    state.setDefaults({ permissionMode: "plan" });
    const required = { ...APP_DEFAULTS, model: "chosen-model", effort: "high", permissionMode: "plan" };
    expect(useCockpit.getState().defaults).toEqual({ ...required, agentConnectionId: "gateway-one" });
    state.setDefaults({ agentConnectionId: undefined, effort: undefined });
    expect(useCockpit.getState().defaults).toEqual(required);
    expect(useCockpit.getState().defaults).not.toHaveProperty("agentConnectionId");
    expect(createCockpitStore().getState().defaults).not.toHaveProperty("agentConnectionId");
    state.setDefaults({ agentConnectionId: "" });
    state.setDefaults({ model: "chosen-model" });
    expect(createCockpitStore().getState().defaults).toEqual({ ...required, agentConnectionId: "" });
  });

  it("deletes an override key set to undefined, so the level above inherits again", () => {
    const s = useCockpit.getState();
    s.setProjectDefaults("p-1", { harness: "codex", model: "gpt-5-codex" });
    s.setProjectDefaults("p-1", { model: undefined });
    expect(useCockpit.getState().projectDefaults["p-1"]).toEqual({ harness: "codex" });
    // `toEqual` ignores a key whose value is `undefined`, so the line above passes just as
    // happily against a stored hole. The key must be GONE — that is the whole reason for
    // deleting rather than assigning, because partialize would otherwise persist a key with
    // nothing in it.
    expect(Object.keys(useCockpit.getState().projectDefaults["p-1"])).toEqual(["harness"]);
  });

  it("records and clears a tile", () => {
    const s = useCockpit.getState();
    s.setTile("s-1", { x: 1, y: 2, w: 3, h: 4 });
    expect(useCockpit.getState().tiles["s-1"]).toEqual({ x: 1, y: 2, w: 3, h: 4 });
    s.clearTile("s-1");
    expect(useCockpit.getState().tiles["s-1"]).toBeUndefined();
  });

  it("pins in the order the user pinned, and is idempotent", () => {
    const s = useCockpit.getState();
    const scope12 = ["s-1", "s-2"];
    s.pinSession("s-2", scope12);
    s.pinSession("s-1", scope12);
    s.pinSession("s-2", scope12);
    expect(useCockpit.getState().pinned).toEqual(["s-2", "s-1"]);
    expect(useCockpit.getState().toasts).toHaveLength(0);
  });

  it("no-ops past the cap and says so in a toast naming the number", () => {
    const s = useCockpit.getState();
    // The scope carries every id in play, so the cap counts the same set it counted
    // before it became `pinned INTERSECT scopeIds`, and the assertions are unchanged.
    const scope = [...Array(CANVAS_MAX_TILES).keys()]
      .map((i) => `s-${i}`)
      .concat("s-over");
    for (let i = 0; i < CANVAS_MAX_TILES; i += 1) s.pinSession(`s-${i}`, scope);
    s.pinSession("s-over", scope);
    const after = useCockpit.getState();
    expect(after.pinned).toHaveLength(CANVAS_MAX_TILES);
    expect(after.pinned).not.toContain("s-over");
    expect(after.toasts).toHaveLength(1);
    expect(after.toasts[0].message).toContain(String(CANVAS_MAX_TILES));
  });

  it("caps within the scope, so another branch's pins cannot fill this one", () => {
    // Added by task 21's fix round 2. The cap is over what Canvas SHOWS. Ten pins held for
    // other branches used to make a six-session branch unable to seed, with a toast pointing
    // at tiles that were not on screen.
    const s = useCockpit.getState();
    for (let i = 0; i < CANVAS_MAX_TILES; i += 1) s.pinSession(`other-${i}`, [`other-${i}`]);
    const mine = ["m-1", "m-2"];
    s.pinSession("m-1", mine);
    s.pinSession("m-2", mine);

    const after = useCockpit.getState();
    expect(after.pinned).toContain("m-1");
    expect(after.pinned).toContain("m-2");
    expect(after.toasts).toHaveLength(0);
  });

  it("unpins without disturbing the order of the rest", () => {
    const s = useCockpit.getState();
    const scope123 = ["s-1", "s-2", "s-3"];
    s.pinSession("s-1", scope123);
    s.pinSession("s-2", scope123);
    s.pinSession("s-3", scope123);
    s.unpinSession("s-2");
    expect(useCockpit.getState().pinned).toEqual(["s-1", "s-3"]);
  });
});

describe("persistence", () => {
  const PERSISTED_KEYS = [
    "defaults", "projectDefaults", "branchDefaults", "tiles", "pinned",
    "layout", "railCollapsed",
    "expandedProjects", "expandedBranches", "expandedWorktrees",
    "selection", "lens",
    "sendChordsToTerminal",
  ];

  it("partializes exactly the thirteen keys §3.5 names, and nothing else", () => {
    const s = useCockpit.getState();
    s.setHarnesses(CATALOGUE);
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    s.pushToast("hello", () => {});
    s.openPalette();
    s.setError({ code: "bad_request", message: "no" });
    expect(Object.keys(partialize(useCockpit.getState())).sort()).toEqual([...PERSISTED_KEYS].sort());
  });

  it("writes the snapshot under the documented key and version", () => {
    // The literals, pinned. Every other assertion in this suite reads the CONSTANTS, so a
    // rename would be invisible to all of them — and a renamed key silently wipes the tiles,
    // pins and defaults of everyone who upgrades.
    expect(PERSIST_KEY).toBe("luci.cockpit");
    expect(PERSIST_VERSION).toBe(2);

    const s = useCockpit.getState();
    s.setLayout("canvas");
    s.setTile("s-1", { x: 8, y: 9, w: 480, h: 320 });
    s.pinSession("s-1", ["s-1"]);
    const raw = localStorage.getItem(PERSIST_KEY);
    expect(raw).not.toBeNull();
    const parsed = JSON.parse(raw as string);
    expect(parsed.version).toBe(PERSIST_VERSION);
    expect(parsed.state.layout).toBe("canvas");
    // VALUES, not only key names: the key-set assertion above is satisfied by a partialize
    // that writes a hole for any of the thirteen, and nothing else round-trips one.
    expect(parsed.state.tiles).toEqual({ "s-1": { x: 8, y: 9, w: 480, h: 320 } });
    expect(parsed.state.pinned).toEqual(["s-1"]);
    expect("toasts" in parsed.state).toBe(false);
    expect("sessions" in parsed.state).toBe(false);
  });

  // OD-9. The setting is off by default, the toggle flips it both ways, and what it writes is
  // what a restart reads back. A VALUE assertion, for the reason the comment above gives.
  it("OD-9: 'Send shortcut keys to the terminal' is off by default, sets both ways, and survives a restart", () => {
    expect(useCockpit.getState().sendChordsToTerminal).toBe(false);
    useCockpit.getState().setSendChordsToTerminal(true);
    expect(useCockpit.getState().sendChordsToTerminal).toBe(true);
    const parsed = JSON.parse(localStorage.getItem(PERSIST_KEY) as string);
    expect(parsed.state.sendChordsToTerminal).toBe(true);
    expect(createCockpitStore().getState().sendChordsToTerminal).toBe(true);
    // A setter, not a flip: setting the value it already holds leaves it held.
    useCockpit.getState().setSendChordsToTerminal(true);
    expect(useCockpit.getState().sendChordsToTerminal).toBe(true);
    useCockpit.getState().setSendChordsToTerminal(false);
    expect(useCockpit.getState().sendChordsToTerminal).toBe(false);
    expect(createCockpitStore().getState().sendChordsToTerminal).toBe(false);
  });

  // Added after mutant M16 (the setter's no-op guard deleted) survived every other OD-9 test.
  it("OD-9: setting the value the setting already holds notifies no subscriber", () => {
    const seen = vi.fn();
    const unsubscribe = useCockpit.subscribe(seen);
    useCockpit.getState().setSendChordsToTerminal(false);
    expect(seen).not.toHaveBeenCalled();
    useCockpit.getState().setSendChordsToTerminal(true);
    expect(seen).toHaveBeenCalledTimes(1);
    useCockpit.getState().setSendChordsToTerminal(true);
    unsubscribe();
    expect(seen).toHaveBeenCalledTimes(1);
  });

  // No version bump came with the key, so a snapshot written by the build before OD-9 must
  // rehydrate with the setting OFF — the behaviour that snapshot was written under — and must
  // keep everything else it carried.
  it("OD-9: a version 1 snapshot written before the setting existed rehydrates with it off", () => {
    localStorage.setItem(
      PERSIST_KEY,
      JSON.stringify({ state: { layout: "canvas", railCollapsed: true }, version: 1 }),
    );
    const store = createCockpitStore();
    expect(store.getState().sendChordsToTerminal).toBe(false);
    expect(store.getState().layout).toBe("canvas");
    expect(store.getState().railCollapsed).toBe(true);
  });

  it("drops a version 0 snapshot rather than translating a shape nobody documented", () => {
    expect(migrate({ layout: "canvas", pinned: ["s-1"] }, 0)).toEqual({});
  });

  it("passes a current version 2 snapshot through untouched", () => {
    const snapshot = { layout: "canvas" as const, pinned: ["s-1"] };
    expect(migrate(snapshot, 2)).toBe(snapshot);
  });

  it("upgrades every version 1 preference without inventing a connection selection", () => {
    const snapshot: ReturnType<typeof partialize> = {
      defaults: { ...APP_DEFAULTS, effort: "high" as const },
      projectDefaults: { "p-1": { model: "project-model" } },
      branchDefaults: { "b-1": { permissionMode: "plan" as const } },
      tiles: { "s-1": { x: 8, y: 9, w: 480, h: 320 } },
      pinned: ["s-1"], layout: "canvas" as const, railCollapsed: true,
      expandedProjects: { "p-1": true }, expandedBranches: { "b-1": true }, expandedWorktrees: { "w-1": true },
      selection: { projectId: "p-1", branchId: "b-1", worktreeId: "w-1", sessionId: "s-1" },
      lens: "design" as const, sendChordsToTerminal: true,
    };
    localStorage.setItem(PERSIST_KEY, JSON.stringify({ state: snapshot, version: 1 }));
    const state = createCockpitStore().getState();
    expect(partialize(state)).toEqual(snapshot);
    expect(state.defaults).not.toHaveProperty("agentConnectionId");
    expect(state.projectDefaults["p-1"]).not.toHaveProperty("agentConnectionId");
    expect(state.branchDefaults["b-1"]).not.toHaveProperty("agentConnectionId");
    expect(JSON.parse(localStorage.getItem(PERSIST_KEY)!)).toEqual({ state: snapshot, version: 2 });
  });

  it.each([undefined, "", "gateway-one"])("preserves connection choice %s at all three levels across migration and restart", (agentConnectionId) => {
    const connection = agentConnectionId === undefined ? {} : { agentConnectionId };
    const choices = {
      defaults: { ...APP_DEFAULTS, ...connection },
      projectDefaults: { "p-1": { model: "project-model", ...connection } },
      branchDefaults: { "b-1": { effort: "high" as const, ...connection } },
    };
    localStorage.setItem(PERSIST_KEY, JSON.stringify({ state: choices, version: 1 }));
    const migrated = createCockpitStore().getState();
    const restarted = createCockpitStore().getState();
    for (const state of [migrated, restarted]) {
      expect(state.defaults).toEqual(choices.defaults);
      expect(state.projectDefaults).toEqual(choices.projectDefaults);
      expect(state.branchDefaults).toEqual(choices.branchDefaults);
      for (const defaults of [state.defaults, state.projectDefaults["p-1"], state.branchDefaults["b-1"]]) {
        if (agentConnectionId === undefined) expect(defaults).not.toHaveProperty("agentConnectionId");
        else expect(defaults).toHaveProperty("agentConnectionId", agentConnectionId);
      }
    }
    expect(JSON.parse(localStorage.getItem(PERSIST_KEY)!).version).toBe(2);
  });

  it("is total over garbage, because it runs before any error boundary exists", () => {
    expect(migrate(null, 1)).toEqual({});
    expect(migrate("nonsense", 1)).toEqual({});
    expect(migrate(42, 1)).toEqual({});
    expect(migrate(undefined, 1)).toEqual({});
  });

  it("launches clean over a version 0 snapshot", () => {
    localStorage.setItem(
      PERSIST_KEY,
      JSON.stringify({ state: { layout: "canvas", pinned: ["s-gone"] }, version: 0 }),
    );
    const store = createCockpitStore();
    expect(store.getState().layout).toBe("columns");
    expect(store.getState().pinned).toEqual([]);
    expect(store.getState().defaults).toEqual(APP_DEFAULTS);
  });

  it("rehydrates a version 1 snapshot", () => {
    localStorage.setItem(
      PERSIST_KEY,
      JSON.stringify({
        state: {
          layout: "canvas",
          railCollapsed: true,
          tiles: { "s-1": { x: 8, y: 9, w: 480, h: 320 } },
          pinned: ["s-1"],
        },
        version: 1,
      }),
    );
    const store = createCockpitStore();
    expect(store.getState().layout).toBe("canvas");
    expect(store.getState().railCollapsed).toBe(true);
    expect(store.getState().tiles["s-1"]).toEqual({ x: 8, y: 9, w: 480, h: 320 });
  });

  it("keeps the code default for a key the snapshot predates — the flat-shape proof", () => {
    localStorage.setItem(
      PERSIST_KEY,
      JSON.stringify({ state: { layout: "canvas" }, version: 1 }),
    );
    const store = createCockpitStore();
    expect(store.getState().layout).toBe("canvas");
    expect(store.getState().tiles).toEqual({});
    expect(store.getState().pinned).toEqual([]);
    expect(store.getState().expandedProjects).toEqual({});
    expect(typeof store.getState().pinSession).toBe("function");
  });

  it("logs rather than going silent when the snapshot cannot be read", () => {
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});

    // The readable snapshot FIRST. A callback that logged unconditionally would satisfy the
    // assertion below on its own, which would make it prove nothing — shared-shape §3A.1
    // rule 3: a fallback is worth asserting only against the case where it must not fire.
    localStorage.setItem(PERSIST_KEY, JSON.stringify({ state: { layout: "canvas" }, version: 1 }));
    expect(createCockpitStore().getState().layout).toBe("canvas");
    expect(logged).not.toHaveBeenCalled();

    localStorage.setItem(PERSIST_KEY, "{not json");
    const store = createCockpitStore();
    expect(store.getState().layout).toBe("columns");
    expect(logged).toHaveBeenCalled();
    logged.mockRestore();
  });
});

describe("resetCockpit", () => {
  it("restores every field and leaves the actions callable", () => {
    const s = useCockpit.getState();
    s.setHarnesses(CATALOGUE);
    s.replaceSessions(sessionsOn("w-1", ["s-1"]), ["w-1"]);
    s.setLayout("canvas");
    s.pinSession("s-1", ["s-1"]);
    s.pushToast("hello");
    resetCockpit();
    const after = useCockpit.getState();
    expect(after.harnesses).toEqual([]);
    expect(after.sessions).toEqual({});
    expect(after.sessionIdsByWorktree).toEqual({});
    expect(after.layout).toBe("columns");
    expect(after.pinned).toEqual([]);
    expect(after.toasts).toEqual([]);
    expect(localStorage.getItem(PERSIST_KEY)).toBeNull();
    after.setLayout("canvas");
    expect(useCockpit.getState().layout).toBe("canvas");

    // resetCockpit spreads these three, and so does every createCockpitStore(), so their
    // nested containers are shared by every store in the process. Frozen, so the day
    // something mutates one in place is a loud TypeError rather than silent corruption of
    // every other store.
    for (const state of [initialDataState, initialUiState, initialPrefsState]) {
      expect(Object.isFrozen(state)).toBe(true);
      for (const value of Object.values(state)) {
        if (typeof value === "object" && value !== null) expect(Object.isFrozen(value)).toBe(true);
      }
    }
  });
});
