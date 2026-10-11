import { beforeEach, expect, it, vi } from "vitest";
import { createCockpitStore, partialize } from "./store";
import { aBranch, aProject, aSession, aWorktree } from "../test/fixtures";

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

function sessionWorkspace() {
  const store = workspace();
  for (const projectId of ["A", "B"]) {
    const branch = aBranch({ id: `branch-${projectId}`, projectId });
    const worktree = aWorktree({ id: `worktree-${projectId}`, projectId, branchId: branch.id, path: `/checkouts/${projectId}` });
    store.getState().setBranches(projectId, [branch]);
    store.getState().setWorktrees(branch.id, [worktree]);
    store.getState().upsertSession(aSession({ id: `session-${projectId}`, projectId, branchId: branch.id, worktreeId: worktree.id, executionGeneration: `allocation-${projectId}` }));
  }
  store.getState().selectBranch("A", "branch-A", "worktree-A");
  store.getState().focusSession("session-A");
  store.getState().showLuci();
  return store;
}

it("reveals an existing session in the same Develop editor without discarding its draft or creating work", () => {
  const store = sessionWorkspace(), discard = vi.fn();
  const peer = aSession({ ...store.getState().sessions["session-A"], id: "session-peer" });
  store.getState().upsertSession(peer);
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Plan", lens: "develop", retained: true, discard });
  const before = store.getState();
  store.getState().openSession(peer.id);
  expect(store.getState()).toMatchObject({ workspaceSurface: "tools", lens: "develop", selection: { projectId: "A", branchId: "branch-A", worktreeId: "worktree-A", sessionId: peer.id }, pendingNavigation: null });
  expect(store.getState().unsavedProjectDrafts).toBe(before.unsavedProjectDrafts);
  expect(store.getState().sessions).toBe(before.sessions);
  expect(discard).not.toHaveBeenCalled();
});

it.each(["project", "phase", "checkout"] as const)("guards explicit session opening across a different %s without changing focus on cancel", changed => {
  const store = sessionWorkspace(), discard = vi.fn();
  let target = "session-A";
  if (changed === "project") target = "session-B";
  if (changed === "phase") store.getState().setLens("design");
  if (changed === "checkout") {
    const extra = aWorktree({ id: "other-A", projectId: "A", branchId: "branch-A", path: "/checkouts/other-A" });
    store.getState().setWorktrees("branch-A", [...store.getState().worktreesByBranch["branch-A"], extra]);
    store.getState().upsertSession(aSession({ id: "session-other", projectId: "A", branchId: "branch-A", worktreeId: extra.id }));
    target = "session-other";
  }
  store.getState().showLuci();
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Editor", lens: store.getState().lens, retained: true, discard });
  const before = store.getState();
  store.getState().openSession(target);
  expect(store.getState().pendingNavigation).toMatchObject({ kind: "session", sessionId: target });
  expect(store.getState().selection).toBe(before.selection);
  expect(store.getState().workspaceSurface).toBe("luci");
  store.getState().cancelNavigation();
  expect(store.getState().selection).toBe(before.selection);
  expect(store.getState().unsavedProjectDrafts).toBe(before.unsavedProjectDrafts);
  store.getState().openSession(target);
  store.getState().discardDraftsAndNavigate(true);
  expect(store.getState()).toMatchObject({ workspaceSurface: "tools", lens: "develop", pendingNavigation: null,
    selection: { sessionId: target, projectId: store.getState().sessions[target].projectId, worktreeId: store.getState().sessions[target].worktreeId } });
  expect(discard).not.toHaveBeenCalled();
  expect(store.getState().sessions).toBe(before.sessions);
});

it("requires explicit discard for an unretained draft before opening another session", () => {
  const store = sessionWorkspace(), discard = vi.fn();
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Plan", lens: "develop", discard });
  store.getState().openSession("session-B");
  store.getState().discardDraftsAndNavigate(true);
  expect(store.getState().selection.sessionId).toBe("session-A");
  expect(store.getState().workspaceSurface).toBe("luci");
  expect(discard).not.toHaveBeenCalled();
  store.getState().discardDraftsAndNavigate();
  expect(store.getState().selection.sessionId).toBe("session-B");
  expect(store.getState().unsavedProjectDrafts).toEqual({});
  expect(discard).toHaveBeenCalledOnce();
});

it.each(["missing", "record-id", "project", "branch", "worktree", "index", "auth-only"] as const)("refuses an unavailable or mismatched %s session destination without changing selection or drafts", invalid => {
  const store = sessionWorkspace();
  const session = store.getState().sessions["session-B"];
  if (invalid === "missing") store.setState({ sessions: { "session-A": store.getState().sessions["session-A"] } });
  if (invalid === "record-id") store.setState({ sessions: { ...store.getState().sessions, "session-B": { ...session, id: "different" } } });
  if (invalid === "project") store.setState({ sessions: { ...store.getState().sessions, "session-B": { ...session, projectId: "A" } } });
  if (invalid === "branch") store.getState().setBranches("B", []);
  if (invalid === "worktree") store.getState().setWorktrees("branch-B", []);
  if (invalid === "index") store.setState({ sessionIdsByWorktree: { ...store.getState().sessionIdsByWorktree, "worktree-B": [] } });
  if (invalid === "auth-only") store.setState({ sessions: { ...store.getState().sessions, "session-B": { ...session, authOnly: true } } });
  const before = store.getState();
  store.getState().openSession("session-B");
  expect(store.getState().selection).toBe(before.selection);
  expect(store.getState().unsavedProjectDrafts).toBe(before.unsavedProjectDrafts);
  expect(store.getState().workspaceSurface).toBe("luci");
  expect(store.getState().pendingNavigation).toBeNull();
  expect(store.getState().toasts.slice(-1)[0]?.message).toMatch(/no longer available/);
});

it.each(["missing", "tuple", "created", "allocation", "branch", "worktree", "index", "source", "scope", "project-path", "worktree-path"] as const)("revalidates a pending session after %s changes before discarding any edits", changed => {
  const store = sessionWorkspace(), discard = vi.fn();
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Plan", lens: "develop", retained: true, discard });
  store.getState().openSession("session-B");
  const session = store.getState().sessions["session-B"];
  if (changed === "missing") store.setState({ sessions: { "session-A": store.getState().sessions["session-A"] } });
  if (changed === "tuple") store.setState({ sessions: { ...store.getState().sessions, "session-B": { ...session, worktreeId: "worktree-A" } } });
  if (changed === "created") store.getState().upsertSession({ ...session, createdAt: "2026-10-12T10:00:00Z" });
  if (changed === "allocation") store.getState().upsertSession({ ...session, executionGeneration: "restarted-B" });
  if (changed === "branch") store.getState().setBranches("B", []);
  if (changed === "worktree") store.getState().setWorktrees("branch-B", []);
  if (changed === "index") store.setState({ sessionIdsByWorktree: { ...store.getState().sessionIdsByWorktree, "worktree-B": [] } });
  if (changed === "source") store.setState({ projectSourceRevisions: { B: 1 } });
  if (changed === "scope") store.setState({ projectCatalogueEpoch: store.getState().projectCatalogueEpoch + 1 });
  if (changed === "project-path") store.setState({ projects: store.getState().projects.map(project => project.id === "B" ? { ...project, path: "/replacement-B" } : project) });
  if (changed === "worktree-path") store.getState().setWorktrees("branch-B", [{ ...store.getState().worktreesByBranch["branch-B"][0], path: "/replacement-checkout" }]);
  const before = store.getState();
  store.getState().discardDraftsAndNavigate();
  expect(store.getState().selection).toBe(before.selection);
  expect(store.getState().unsavedProjectDrafts).toBe(before.unsavedProjectDrafts);
  expect(store.getState().workspaceSurface).toBe("luci");
  expect(store.getState().pendingNavigation).toBeNull();
  expect(store.getState().toasts.slice(-1)[0]?.message).toMatch(/edited draft is still here/);
  expect(discard).not.toHaveBeenCalled();
});

it("allows current session output and status observation to progress while navigation is awaiting consent", () => {
  const store = sessionWorkspace(), discard = vi.fn();
  store.getState().setUnsavedProjectDraft("draft", { projectId: "A", label: "Plan", lens: "develop", retained: true, discard });
  store.getState().openSession("session-B");
  store.getState().upsertSession({ ...store.getState().sessions["session-B"], title: "Updated title", status: "exited", updatedAt: "2026-10-12T10:00:00Z" });
  store.getState().discardDraftsAndNavigate(true);
  expect(store.getState()).toMatchObject({ workspaceSurface: "tools", pendingNavigation: null, selection: { sessionId: "session-B" } });
  expect(discard).not.toHaveBeenCalled();
});
