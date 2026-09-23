import { beforeEach, describe, expect, it } from "vitest";
import { aBranch, aHarness, aMachine, aSession, aWorktree } from "../test/fixtures";
import {
  selectBranches,
  selectCanvasSessionIds,
  selectDefaultMachine,
  selectFocusedSessionId,
  selectHarness,
  selectPaneSessionIds,
  selectResolvedDefaults,
  selectSelectedBranch,
  selectSession,
  selectSessionIdsForWorktree,
  selectTargetWorktreeId,
  selectWorktrees,
} from "./selectors";
import { resetCockpit, useCockpit } from "./store";

/* Composed here from task 15's `aHarness`, not added to `fixtures.ts`: shared-shape §3.2
 * sanctions six builders and no seventh. Every field the assertions below read is written out
 * here — `shell`'s name, `codex`'s unavailability, and `claude-code`'s model list and
 * defaultModel, which the clamping test depends on. */
const CATALOGUE = [
  aHarness({
    id: "claude-code",
    name: "Claude Code",
    available: true,
    models: [
      { id: "opus-4.8", name: "Opus 4.8" },
      { id: "sonnet-4.8", name: "Sonnet 4.8" },
    ],
    defaultModel: "sonnet-4.8",
    efforts: ["", "low", "medium", "high"],
    permissionModes: ["ask", "bypass", "plan"],
  }),
  aHarness({
    id: "shell",
    name: "Shell",
    available: true,
    models: [],
    defaultModel: "",
    efforts: [],
    permissionModes: [],
  }),
  aHarness({
    id: "codex",
    name: "Codex",
    available: false,
    models: [{ id: "gpt-5-codex", name: "GPT-5 Codex" }],
    defaultModel: "gpt-5-codex",
  }),
];

/** One branch, two worktrees in server order, two sessions on the first and one on the
 *  second. Everything below reads this shape. Every id is pinned by an override, because
 *  `aSession()` and `aWorktree()` default theirs from task 14's `IDS`. */
function seed(): void {
  const s = useCockpit.getState();
  s.setHarnesses(CATALOGUE);
  s.setBranches("p-1", [aBranch({ id: "b-1", projectId: "p-1", name: "main", isHead: true })]);
  s.setWorktrees("b-1", [
    aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", name: "main", isPrimary: true }),
    aWorktree({ id: "w-2", projectId: "p-1", branchId: "b-1", name: "evidence-upload" }),
  ]);
  s.replaceSessions(
    [
      aSession({ id: "s-1", worktreeId: "w-1" }),
      aSession({ id: "s-2", worktreeId: "w-1" }),
      aSession({ id: "s-3", worktreeId: "w-2" }),
    ],
    ["w-1", "w-2"],
  );
  s.selectBranch("p-1", "b-1");
}

beforeEach(() => {
  localStorage.clear();
  resetCockpit();
});

describe("selectors", () => {
  it("prefers the machine flagged default, then falls back to the first", () => {
    useCockpit.getState().setMachines([
      aMachine({ id: "cloud-1", name: "cloud-1", kind: "cloud", default: false }),
      aMachine({ id: "local", name: "local", kind: "local", default: true }),
    ]);
    expect(selectDefaultMachine(useCockpit.getState())?.id).toBe("local");
    useCockpit.getState().setMachines([
      aMachine({ id: "cloud-1", name: "cloud-1", kind: "cloud", default: false }),
      aMachine({ id: "cloud-2", name: "cloud-2", kind: "cloud", default: false }),
    ]);
    expect(selectDefaultMachine(useCockpit.getState())?.id).toBe("cloud-1");
    useCockpit.getState().setMachines([]);
    expect(selectDefaultMachine(useCockpit.getState())).toBeUndefined();
  });

  it("returns the very same empty array for an unknown or null parent", () => {
    const s = useCockpit.getState();
    const a = selectBranches("p-unknown")(s);
    const b = selectBranches(null)(s);
    const c = selectWorktrees("b-unknown")(s);
    expect(a).toEqual([]);
    expect(a).toBe(b);
    expect(selectSessionIdsForWorktree("w-unknown")(s)).toEqual([]);
    expect(c).toEqual([]);
    // `toEqual` is satisfied by a fresh literal, which is exactly the re-render the shared
    // constants exist to prevent — every collapsed worktree row in the rail reads the id
    // selector, and every unselected project row reads the worktree one.
    expect(selectSessionIdsForWorktree("w-a")(s)).toBe(selectSessionIdsForWorktree("w-b")(s));
    expect(c).toBe(selectWorktrees(null)(s));
  });

  it("reads branches, worktrees, session ids, a session and a harness", () => {
    seed();
    const s = useCockpit.getState();
    expect(selectBranches("p-1")(s).map((b) => b.id)).toEqual(["b-1"]);
    expect(selectWorktrees("b-1")(s).map((w) => w.id)).toEqual(["w-1", "w-2"]);
    expect(selectSessionIdsForWorktree("w-1")(s)).toEqual(["s-1", "s-2"]);
    expect(selectSession("s-3")(s)?.worktreeId).toBe("w-2");
    expect(selectSession("s-nope")(s)).toBeUndefined();
    expect(selectHarness("shell")(s)?.name).toBe("Shell");
    expect(selectHarness("nope")(s)).toBeUndefined();
  });

  it("orders the panes by worktree, then by session, both in server order", () => {
    seed();
    expect(selectPaneSessionIds(useCockpit.getState())).toEqual(["s-1", "s-2", "s-3"]);

    // The seed indexes w-1 before w-2, so the assertion above cannot tell "iterate the
    // worktree list" apart from "iterate the session index" — both give the same answer.
    // Build the index in the OPPOSITE order: the panes must still follow the worktree list.
    resetCockpit();
    const s = useCockpit.getState();
    s.setWorktrees("b-1", [
      aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1", name: "main", isPrimary: true }),
      aWorktree({ id: "w-2", projectId: "p-1", branchId: "b-1", name: "evidence-upload" }),
    ]);
    s.replaceSessions(
      [
        aSession({ id: "s-3", worktreeId: "w-2" }),
        aSession({ id: "s-1", worktreeId: "w-1" }),
        aSession({ id: "s-2", worktreeId: "w-1" }),
      ],
      ["w-2", "w-1"],
    );
    s.selectBranch("p-1", "b-1");
    expect(Object.keys(useCockpit.getState().sessionIdsByWorktree)).toEqual(["w-2", "w-1"]);
    expect(selectPaneSessionIds(useCockpit.getState())).toEqual(["s-1", "s-2", "s-3"]);
  });

  it("keeps an exited session in the pane order, because the transcript is the artifact", () => {
    seed();
    useCockpit.getState().upsertSession(
      aSession({ id: "s-2", worktreeId: "w-1", status: "exited", exitReason: "killed" }),
    );
    expect(selectPaneSessionIds(useCockpit.getState())).toEqual(["s-1", "s-2", "s-3"]);
  });

  it("returns the stable empty constant when no branch is selected", () => {
    const first = selectPaneSessionIds(useCockpit.getState());
    useCockpit.getState().setLayout("canvas");
    expect(first).toBe(selectPaneSessionIds(useCockpit.getState()));
    expect(first).toEqual([]);
  });

  it("filters dead pins out of the canvas and keeps pin order", () => {
    seed();
    const s = useCockpit.getState();
    // Production passes the branch's pane order as the scope; so does this. "s-gone" is
    // not in it, which is the point of the case below — it pins and the selector drops it.
    const scope = selectPaneSessionIds(useCockpit.getState());
    s.pinSession("s-3", scope);
    s.pinSession("s-1", scope);
    s.pinSession("s-gone", scope);
    expect(selectCanvasSessionIds(useCockpit.getState())).toEqual(["s-3", "s-1"]);

    // The EMPTY case, which is the one `NO_IDS` exists for and which nothing else pins: an
    // empty Canvas is the state the app launches in, so a fresh [] here re-renders the
    // canvas on every 2 s poll before the user has pinned anything.
    resetCockpit();
    const empty = selectCanvasSessionIds(useCockpit.getState());
    expect(empty).toEqual([]);
    expect(selectCanvasSessionIds(useCockpit.getState())).toBe(empty);
  });

  it("drops a pin whose session is on another branch, and brings it back on the switch", () => {
    // `pinned` is one flat global list (§3.4) while Canvas renders one branch. Added by task
    // 21's fix round 2: without the intersection, the second branch shows the first's tiles.
    seed();
    const s = useCockpit.getState();
    s.setBranches("p-1", [
      aBranch({ id: "b-1", projectId: "p-1", name: "main", isHead: true }),
      aBranch({ id: "b-2", projectId: "p-1", name: "audit-trail", isHead: false }),
    ]);
    s.setWorktrees("b-2", [
      aWorktree({ id: "w-3", projectId: "p-1", branchId: "b-2", name: "audit-trail" }),
    ]);
    s.upsertSession(aSession({ id: "s-9", projectId: "p-1", branchId: "b-2", worktreeId: "w-3" }));
    s.pinSession("s-1", ["s-1"]);
    s.pinSession("s-9", ["s-9"]);

    expect(selectCanvasSessionIds(useCockpit.getState())).toEqual(["s-1"]);

    useCockpit.getState().selectBranch("p-1", "b-2");
    expect(selectCanvasSessionIds(useCockpit.getState())).toEqual(["s-9"]);
    // Both survive in the store, which is what makes each branch's arrangement come back.
    expect(useCockpit.getState().pinned).toEqual(["s-1", "s-9"]);
  });

  it("threads the selection through resolveDefaults", () => {
    seed();
    const s = useCockpit.getState();
    s.setProjectDefaults("p-1", { effort: "high" });
    s.setBranchDefaults("b-1", { model: "opus-4.8" });
    expect(selectResolvedDefaults(useCockpit.getState())).toEqual({
      harness: "claude-code",
      model: "opus-4.8",
      effort: "high",
      permissionMode: "ask",
    });
  });

  it("clamps a stale branch default against the live catalogue", () => {
    seed();
    useCockpit.getState().setBranchDefaults("b-1", { harness: "codex", model: "gpt-5-codex" });
    const r = selectResolvedDefaults(useCockpit.getState());
    expect(r.harness).toBe("claude-code");
    expect(r.model).toBe("sonnet-4.8");
  });

  it("reports the focused session id", () => {
    expect(selectFocusedSessionId(useCockpit.getState())).toBeNull();
    useCockpit.getState().focusSession("s-2");
    expect(selectFocusedSessionId(useCockpit.getState())).toBe("s-2");
  });

  /* TWO branches, and the selection on the second. With one branch per project — which every
   * other fixture in this file and in ColumnsLayout.test.tsx uses — `.find(b => b.id === …)`
   * and `[0]` are indistinguishable, and the `[0]` mutant survives the whole suite. It is not
   * a harmless mutant: DevelopLens hands this to EmptyDevelop, which derives its worktree list
   * and its create target from `branch.id`, so a user on branch B would be shown branch A's
   * worktree name and the `＋` button would start a session on A's worktree. */
  describe("selectSelectedBranch", () => {
    beforeEach(() => {
      useCockpit.setState({
        branchesByProject: {
          "p-1": [
            aBranch({ id: "b-1", projectId: "p-1", name: "main" }),
            aBranch({ id: "b-2", projectId: "p-1", name: "evidence-upload" }),
          ],
        },
      });
    });

    it("returns the branch named by the selection, not the project's first", () => {
      useCockpit.setState({
        selection: { projectId: "p-1", branchId: "b-2", worktreeId: null, sessionId: null },
      });
      expect(selectSelectedBranch(useCockpit.getState())?.id).toBe("b-2");
      expect(selectSelectedBranch(useCockpit.getState())?.name).toBe("evidence-upload");
    });

    it("returns undefined when no branch is selected", () => {
      useCockpit.setState({
        selection: { projectId: "p-1", branchId: null, worktreeId: null, sessionId: null },
      });
      expect(selectSelectedBranch(useCockpit.getState())).toBeUndefined();
    });

    it("returns undefined when no project is selected", () => {
      useCockpit.setState({
        selection: { projectId: null, branchId: "b-2", worktreeId: null, sessionId: null },
      });
      expect(selectSelectedBranch(useCockpit.getState())).toBeUndefined();
    });

    it("returns undefined when the selected branch is not in the project", () => {
      useCockpit.setState({
        selection: { projectId: "p-1", branchId: "b-gone", worktreeId: null, sessionId: null },
      });
      expect(selectSelectedBranch(useCockpit.getState())).toBeUndefined();
    });
  });

  // Task 23's addition to §3.7: ⌘⏎ and the palette's "New session…" row share this read.
  describe("selectTargetWorktreeId", () => {
    it("prefers the explicitly selected worktree over the branch's first", () => {
      seed();
      useCockpit.getState().selectWorktree("w-2");
      expect(selectTargetWorktreeId(useCockpit.getState())).toBe("w-2");
    });

    it("falls back to the branch's first worktree, in server order", () => {
      seed();
      expect(selectTargetWorktreeId(useCockpit.getState())).toBe("w-1");
    });

    it("is null for a selected branch that has no worktrees, and with no branch at all", () => {
      seed();
      useCockpit.getState().setWorktrees("b-1", []);
      expect(selectTargetWorktreeId(useCockpit.getState())).toBeNull();
      resetCockpit();
      expect(selectTargetWorktreeId(useCockpit.getState())).toBeNull();
    });
  });
});
