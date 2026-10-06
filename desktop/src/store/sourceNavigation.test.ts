import { beforeEach, expect, it } from "vitest";
import { resetCockpit, useCockpit, partialize } from "./store";
import { selectPaneSessionIds, selectCanvasSessionIds, selectBranchAttentionSessionId, selectSessionIdsForWorktree } from "./selectors";
import { aBranch, aSession, aWorktree } from "../test/fixtures";

beforeEach(() => { resetCockpit(); useCockpit.setState({ selection: { projectId: "owned", branchId: "main", worktreeId: "primary", sessionId: null }, lens: "discover" }); });
it("retains dirty context and drops a queued build inspection when navigation is cancelled", () => {
  const state = useCockpit.getState();
  state.setUnsavedProjectDraft("editor", { projectId: "owned", label: "Definition", lens: "discover" });
  useCockpit.setState({ projectRunInspection: { scope: "account-workspace-project", projectId: "owned", runId: "older-active" } });
  state.setLens("develop");
  expect(useCockpit.getState().lens).toBe("discover");
  expect(useCockpit.getState().projectRunInspection?.runId).toBe("older-active");
  expect(partialize(useCockpit.getState())).not.toHaveProperty("projectRunInspection");
  state.cancelNavigation();
  expect(useCockpit.getState().projectRunInspection).toBeNull();
  expect(useCockpit.getState().unsavedProjectDrafts.editor).toBeDefined();
});
it("keeps a dirty editor mounted until navigation is explicitly confirmed", () => {
  const state = useCockpit.getState(); state.setUnsavedProjectDraft("editor", { projectId: "owned", label: "Definition", lens: "discover" });
  state.setLens("design"); expect(useCockpit.getState().lens).toBe("discover"); expect(useCockpit.getState().pendingNavigation).toEqual({ kind: "lens", lens: "design" });
  state.cancelNavigation(); expect(useCockpit.getState().pendingNavigation).toBeNull(); expect(useCockpit.getState().unsavedProjectDrafts.editor).toBeDefined();
  state.selectProject("other"); expect(useCockpit.getState().selection.projectId).toBe("owned"); state.discardDraftsAndNavigate();
  expect(useCockpit.getState().selection.projectId).toBe("other"); expect(useCockpit.getState().unsavedProjectDrafts).toEqual({});
  expect(partialize(useCockpit.getState())).not.toHaveProperty("pendingNavigation"); expect(partialize(useCockpit.getState())).not.toHaveProperty("projectSourceRevisions");
});
it("moves the same primary checkout while retaining exited transcripts on their saved branch", () => {
  const state = useCockpit.getState(); const main = aBranch({ id: "main", projectId: "owned", isHead: true }); const feature = aBranch({ id: "feature", projectId: "owned", isHead: false });
  state.setBranches("owned", [main, feature]); state.setWorktrees("main", [aWorktree({ id: "primary", projectId: "owned", branchId: "main", isPrimary: true, ready: true })]);
  state.upsertSession(aSession({ id: "old", projectId: "owned", branchId: "main", worktreeId: "primary", status: "exited" })); state.pinSession("old", ["old"]);
  state.acceptBranchChange("owned", [{ ...main, isHead: false }, { ...feature, isHead: true }], aWorktree({ id: "primary", projectId: "owned", branchId: "feature", isPrimary: true, ready: false }));
  expect(useCockpit.getState().worktreesByBranch.main).toEqual([]); expect(selectPaneSessionIds(useCockpit.getState())).toEqual([]); expect(selectCanvasSessionIds(useCockpit.getState())).toEqual([]);
  expect(selectSessionIdsForWorktree("primary", "feature")(useCockpit.getState())).toEqual([]); expect(selectBranchAttentionSessionId("feature")(useCockpit.getState())).toBeNull();
  expect(useCockpit.getState().sessions.old.branchId).toBe("main"); expect(useCockpit.getState().pinned).toEqual(["old"]);
  state.acceptBranchChange("owned", [main, feature], aWorktree({ id: "primary", projectId: "owned", branchId: "main", isPrimary: true, ready: false }));
  expect(selectPaneSessionIds(useCockpit.getState())).toEqual(["old"]); expect(selectCanvasSessionIds(useCockpit.getState())).toEqual(["old"]); expect(useCockpit.getState().projectSourceRevisions.owned).toBe(2);
});
