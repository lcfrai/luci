import { beforeEach, expect, it, vi } from "vitest";
import { createCockpitStore, partialize } from "./store";
import { aBranch, aProject, aWorktree } from "../test/fixtures";

const projects = ["A", "B"].map(id => aProject({ id, path: `/projects/${id}` }));
function workspace() {
  const store = createCockpitStore();
  store.getState().setProjects(projects);
  store.getState().bindProjectNavigation("owner/local/projects");
  store.getState().selectProject("A", "design");
  return store;
}
beforeEach(() => { localStorage.clear(); });

it("defaults to Luci without adding a persisted surface preference", () => {
  const store = createCockpitStore();
  expect(store.getState().workspaceSurface).toBe("luci");
  store.getState().setLens("develop");
  expect(store.getState().workspaceSurface).toBe("tools");
  expect(partialize(store.getState())).not.toHaveProperty("workspaceSurface");
  const restarted = createCockpitStore();
  expect(restarted.getState().workspaceSurface).toBe("luci");
  expect(restarted.getState().lens).toBe("develop");
});

it("opens Luci without discarding edits, changing selection or resolving pending navigation", () => {
  const store = workspace(), discard = vi.fn();
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Design", lens: "design", retained: true, discard });
  store.getState().setLens("deploy");
  const before = store.getState();
  store.getState().showLuci();
  const after = store.getState();
  expect(after.workspaceSurface).toBe("luci");
  expect(after.selection).toBe(before.selection);
  expect(after.lens).toBe("design");
  expect(after.unsavedProjectDrafts).toBe(before.unsavedProjectDrafts);
  expect(after.pendingNavigation).toBe(before.pendingNavigation);
  expect(after.projectPositions).toBe(before.projectPositions);
  expect(discard).not.toHaveBeenCalled();
});

it.each(["discover", "design", "develop", "deploy"] as const)("returns to the same %s editor without asking to discard its retained draft", lens => {
  const store = workspace(), discard = vi.fn();
  store.getState().setLens(lens);
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Editor", lens, retained: true, discard });
  store.getState().showLuci();
  const before = store.getState();
  store.getState().setLens(lens);
  expect(store.getState().workspaceSurface).toBe("tools");
  expect(store.getState().selection).toBe(before.selection);
  expect(store.getState().unsavedProjectDrafts).toBe(before.unsavedProjectDrafts);
  expect(store.getState().pendingNavigation).toBeNull();
  expect(discard).not.toHaveBeenCalled();
});

it("keeps a different phase behind the draft guard until navigation is accepted", () => {
  const store = workspace(), discard = vi.fn();
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Design", lens: "design", retained: true, discard });
  store.getState().showLuci();
  store.getState().setLens("deploy");
  expect(store.getState()).toMatchObject({ workspaceSurface: "luci", lens: "design", pendingNavigation: { kind: "lens", lens: "deploy" } });
  store.getState().cancelNavigation();
  expect(store.getState()).toMatchObject({ workspaceSurface: "luci", lens: "design", pendingNavigation: null });
  expect(store.getState().unsavedProjectDrafts.draft).toBeDefined();
  store.getState().setLens("deploy");
  store.getState().discardDraftsAndNavigate(true);
  expect(store.getState()).toMatchObject({ workspaceSurface: "tools", lens: "deploy", pendingNavigation: null });
  expect(discard).not.toHaveBeenCalled();
});

it("does not keep an unretained draft through a guarded different phase", () => {
  const store = workspace(), discard = vi.fn();
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Design", lens: "design", discard });
  store.getState().showLuci();
  store.getState().setLens("discover");
  store.getState().discardDraftsAndNavigate(true);
  expect(store.getState()).toMatchObject({ workspaceSurface: "luci", lens: "design", pendingNavigation: { kind: "lens", lens: "discover" } });
  store.getState().discardDraftsAndNavigate();
  expect(discard).toHaveBeenCalledOnce();
  expect(store.getState()).toMatchObject({ workspaceSurface: "tools", lens: "discover", pendingNavigation: null });
});

it.each(["luci", "tools"] as const)("preserves the %s surface on plain project selection after its draft guard", surface => {
  const store = workspace(), discard = vi.fn();
  store.getState().selectProject("B", "files");
  store.getState().selectProject("A", "design");
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Design", lens: "design", retained: true, discard });
  if (surface === "luci") store.getState().showLuci();
  store.getState().selectProject("B");
  expect(store.getState()).toMatchObject({ workspaceSurface: surface, selection: { projectId: "A" }, lens: "design", pendingNavigation: { kind: "project", projectId: "B" } });
  store.getState().cancelNavigation();
  expect(store.getState().selection.projectId).toBe("A");
  expect(store.getState().workspaceSurface).toBe(surface);
  store.getState().selectProject("B");
  store.getState().discardDraftsAndNavigate(true);
  expect(store.getState()).toMatchObject({ workspaceSurface: surface, selection: { projectId: "B" }, lens: "files", pendingNavigation: null });
  expect(discard).not.toHaveBeenCalled();
});

it("reveals an explicitly selected project phase, including the unchanged current phase", () => {
  const store = workspace(), discard = vi.fn();
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Design", lens: "design", retained: true, discard });
  store.getState().showLuci();
  store.getState().selectProject("A", "design");
  expect(store.getState()).toMatchObject({ workspaceSurface: "tools", lens: "design", pendingNavigation: null });
  expect(store.getState().unsavedProjectDrafts.draft).toBeDefined();
  store.getState().showLuci();
  store.getState().selectProject("B", "deploy");
  expect(store.getState().workspaceSurface).toBe("luci");
  store.getState().discardDraftsAndNavigate(true);
  expect(store.getState()).toMatchObject({ workspaceSurface: "tools", lens: "deploy", selection: { projectId: "B" } });
  expect(discard).not.toHaveBeenCalled();
});

it("retains Luci and its edited project when a guarded destination loses catalogue membership", () => {
  const store = workspace(), discard = vi.fn();
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Design", lens: "design", retained: true, discard });
  store.getState().showLuci();
  store.getState().selectProject("B", "deploy");
  store.getState().setProjects(projects.filter(project => project.id !== "B"));
  store.getState().discardDraftsAndNavigate();
  expect(store.getState()).toMatchObject({ workspaceSurface: "luci", lens: "design", selection: { projectId: "A" }, pendingNavigation: null });
  expect(store.getState().unsavedProjectDrafts.draft).toBeDefined();
  expect(discard).not.toHaveBeenCalled();
});

it("selects tools for explicit branch navigation and confirmed own-project source changes", () => {
  const store = workspace();
  const main = aBranch({ id: "main-A", projectId: "A" });
  const worktree = aWorktree({ id: "primary-A", projectId: "A", branchId: main.id });
  store.getState().setBranches("A", [main]);
  store.getState().setWorktrees(main.id, [worktree]);
  store.getState().showLuci();
  store.getState().selectBranch("A", main.id, worktree.id);
  expect(store.getState()).toMatchObject({ workspaceSurface: "tools", lens: "develop", selection: { projectId: "A", branchId: main.id, worktreeId: worktree.id } });
  store.getState().showLuci();
  store.getState().acceptBranchChange("B", [aBranch({ id: "main-B", projectId: "B" })], aWorktree({ id: "primary-B", projectId: "B", branchId: "main-B" }));
  expect(store.getState().workspaceSurface).toBe("luci");
  store.getState().acceptBranchChange("A", [main], worktree);
  expect(store.getState().workspaceSurface).toBe("tools");
});

it("does not let focus-only updates pull the selected project away from Luci", () => {
  const store = workspace();
  store.getState().showLuci();
  store.getState().selectWorktree("primary-A");
  store.getState().focusSession("session-A");
  expect(store.getState()).toMatchObject({ workspaceSurface: "luci", lens: "design", selection: { worktreeId: "primary-A", sessionId: "session-A" } });
});

it("resets to Luci for a changed account/workspace scope without changing same-scope presentation", () => {
  const store = workspace();
  expect(store.getState().workspaceSurface).toBe("tools");
  store.getState().bindProjectNavigation("owner/local/projects");
  expect(store.getState().workspaceSurface).toBe("tools");
  store.getState().bindProjectNavigation("other/remote/projects");
  expect(store.getState()).toMatchObject({ workspaceSurface: "luci", selection: { projectId: null }, projectRunInspection: null, projectRunVerification: null, projectRunDeployment: null });
});
