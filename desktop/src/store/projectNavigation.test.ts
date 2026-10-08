import { beforeEach, expect, it, vi } from "vitest";
import { startCatalogue } from "../App";
import { createCockpitStore, resetCockpit, useCockpit } from "./store";
import { MAX_NAVIGATION_PROJECTS, PROJECT_NAVIGATION_PREFIX, navigationScope, readProjectNavigation, writeProjectNavigation } from "../preferences/projectNavigation";
import { aBranch, aProject, aSession, aWorktree } from "../test/fixtures";
import type { Lens } from "./types";

const projects = ["A", "B", "C", "D"].map(id => aProject({ id, name: id, path: `/projects/${id}` }));
const scope = navigationScope("local-api", "owner", "", { mode: "local" }, "/projects", "owner")!;
beforeEach(() => { localStorage.clear(); resetCockpit(); vi.restoreAllMocks(); });

it("restores each of four project stages through switching and a recreated app store", () => {
  const store = useCockpit.getState(); store.setProjects(projects); store.bindProjectNavigation(scope);
  const stages: Lens[] = ["design", "files", "develop", "deploy"];
  projects.forEach((project, i) => { store.selectProject(project.id); store.setLens(stages[i]); });
  projects.forEach((project, i) => { store.selectProject(project.id); expect(useCockpit.getState().lens).toBe(stages[i]); });
  const restarted = createCockpitStore(); restarted.getState().bindProjectNavigation(scope);
  expect(restarted.getState().selection.projectId).toBe("D");
  restarted.getState().setProjects(projects);
  projects.forEach((project, i) => { restarted.getState().selectProject(project.id); expect(restarted.getState().lens).toBe(stages[i]); });
  expect(JSON.stringify(readProjectNavigation(scope))).not.toMatch(/prompt|credential|permission|runId/);
});

it("keeps a retained draft and destination together through confirmed project navigation", () => {
  const store = useCockpit.getState(); store.setProjects(projects); store.bindProjectNavigation(scope);
  store.selectProject("B", "files"); store.selectProject("A", "design");
  const discard = vi.fn(); store.setUnsavedProjectDraft("draft-A", { projectId: "A", label: "Design", lens: "design", retained: true, discard });
  store.selectProject("B");
  expect(useCockpit.getState().selection.projectId).toBe("A"); expect(useCockpit.getState().lens).toBe("design");
  expect(readProjectNavigation(scope).selected).toBe("A");
  store.discardDraftsAndNavigate(true);
  expect(useCockpit.getState().selection.projectId).toBe("B"); expect(useCockpit.getState().lens).toBe("files"); expect(discard).not.toHaveBeenCalled();
  store.selectProject("A"); expect(useCockpit.getState().lens).toBe("design");
});

it.each(["account", "team", "root", "host"])("does not transfer position to another %s", kind => {
  const store = useCockpit.getState(); store.setProjects(projects); store.bindProjectNavigation(scope); store.selectProject("A", "deploy");
  const other = navigationScope("local-api", kind === "account" ? "other" : "owner", kind === "team" ? "team-other" : "", { mode: "local", host: kind === "host" ? "remote" : "" }, kind === "root" ? "/new-root" : "/projects", "owner")!;
  store.bindProjectNavigation(other); expect(useCockpit.getState().selection.projectId).toBeNull(); store.selectProject("A"); expect(useCockpit.getState().lens).toBe("discover");
  store.bindProjectNavigation(scope); expect(useCockpit.getState().selection.projectId).toBe("A"); expect(useCockpit.getState().lens).toBe("deploy");
});

it("refuses a removed pending destination while retaining the draft and invalidates relocated source positions", () => {
  const store = useCockpit.getState(); store.setProjects(projects); store.bindProjectNavigation(scope);
  store.selectProject("B", "files"); store.selectProject("A", "design");
  const discard = vi.fn(); store.setUnsavedProjectDraft("draft", { projectId: "A", label: "Design", lens: "design", discard });
  store.selectProject("B"); store.setProjects(projects.filter(project => project.id !== "B")); store.discardDraftsAndNavigate();
  expect(discard).not.toHaveBeenCalled(); expect(useCockpit.getState().selection.projectId).toBe("A"); expect(useCockpit.getState().unsavedProjectDrafts.draft).toBeDefined();
  store.setUnsavedProjectDraft("draft", null); store.setProjects(projects.map(project => project.id === "A" ? { ...project, path: "/replacement/A" } : project));
  store.selectProject("C"); store.selectProject("A"); expect(useCockpit.getState().lens).toBe("discover");
  // A new source starts at its definition; old source navigation is not reused.
  writeProjectNavigation(scope, { selected: "A", projects: { A: { lens: "merge", path: "/old/A", updatedAt: Date.now() } } });
  const fresh = createCockpitStore(); fresh.getState().setProjects(projects); fresh.getState().bindProjectNavigation(scope); fresh.getState().selectProject("A");
  expect(fresh.getState().lens).toBe("discover");
});

it("releases removed project metadata and ignores late catalogue writes without dropping valid startup preferences", () => {
  const store = useCockpit.getState(); store.setProjects(projects);
  store.setBranches("A", [aBranch({ id: "branch-A", projectId: "A" })]);
  store.setWorktrees("branch-A", [aWorktree({ id: "worktree-A", branchId: "branch-A", projectId: "A" })]);
  store.upsertSession(aSession({ id: "session-A", branchId: "branch-A", worktreeId: "worktree-A", projectId: "A" }));
  useCockpit.setState({ expandedBranches: { "branch-A": true, "branch-B": true }, pinned: ["session-A", "not-loaded-B"] });
  store.setProjects(projects.filter(project => project.id !== "A"));
  store.setBranches("A", [aBranch({ id: "branch-A", projectId: "A" })]); store.setWorktrees("branch-A", [aWorktree({ id: "worktree-A" })]);
  const current = useCockpit.getState(); expect(current.branchesByProject.A).toBeUndefined(); expect(current.worktreesByBranch["branch-A"]).toBeUndefined();
  expect(current.sessions["session-A"]).toBeUndefined(); expect(current.pinned).toEqual(["not-loaded-B"]); expect(current.expandedBranches).toEqual({ "branch-B": true });
});

it("bounds retained positions and scopes without removing drafts, and reports a storage failure", () => {
  const many = Array.from({ length: 100 }, (_, i) => aProject({ id: `p-${i}`, path: `/p/${i}` }));
  const store = useCockpit.getState(); store.setProjects(many); store.bindProjectNavigation(scope);
  many.forEach(project => store.selectProject(project.id, "files")); expect(Object.keys(useCockpit.getState().projectPositions)).toHaveLength(MAX_NAVIGATION_PROJECTS);
  localStorage.setItem("luci.project-draft.v1:untouched", "draft");
  for (let i = 0; i < 12; i++) writeProjectNavigation(`scope-${i}`, { selected: null, projects: {} });
  expect(Object.keys(localStorage).filter(key => key.startsWith(PROJECT_NAVIGATION_PREFIX))).toHaveLength(8);
  expect(localStorage.getItem("luci.project-draft.v1:untouched")).toBe("draft");
  const original = Storage.prototype.setItem;
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(function(this: Storage, key, value) { if (key.startsWith(PROJECT_NAVIGATION_PREFIX)) throw new Error("quota"); original.call(this, key, value); });
  store.setLens("design"); expect(useCockpit.getState().lens).toBe("design"); expect(useCockpit.getState().projectNavigationError).toBe(true);
});

it("treats corrupt or oversized preferences as unavailable without restoring execution intent", () => {
  localStorage.setItem(PROJECT_NAVIGATION_PREFIX + scope, '{"version":1,"projects":null}'); expect(readProjectNavigation(scope)).toEqual({ projects: {}, selected: null });
  localStorage.setItem(PROJECT_NAVIGATION_PREFIX + scope, JSON.stringify({ version: 1, selected: "A", projects: { A: { lens: "execute-now", path: "/projects/A", updatedAt: Date.now(), consent: true } } }));
  expect(readProjectNavigation(scope).selected).toBeNull();
  localStorage.setItem(PROJECT_NAVIGATION_PREFIX + scope, "x".repeat(256 * 1024)); expect(readProjectNavigation(scope).projects).toEqual({});
});

it("preserves a returning root's saved stage until its own catalogue arrives", () => {
  const store = useCockpit.getState(); store.setProjects(projects); store.bindProjectNavigation(scope); store.selectProject("A", "files");
  const remoteScope = navigationScope("local-api", "owner", "", { mode: "local" }, "/other-projects", "owner")!;
  const remoteProjects = [aProject({ id: "remote-A", path: "/other-projects/A" })];
  store.bindProjectNavigation(remoteScope); expect(useCockpit.getState().projectsLoaded).toBe(false);
  store.setProjects(remoteProjects); store.selectProject("remote-A", "deploy");
  store.bindProjectNavigation(scope);
  expect(useCockpit.getState().projectsLoaded).toBe(false); expect(useCockpit.getState().selection.projectId).toBe("A"); expect(useCockpit.getState().lens).toBe("files");
  expect(useCockpit.getState().projectPositions.A.lens).toBe("files");
  store.setProjects(projects); expect(useCockpit.getState().lens).toBe("files");
});

it("ignores an old catalogue response after a same-account source switch", async () => {
  const store = createCockpitStore(); store.getState().setProjects(projects); store.getState().bindProjectNavigation(scope); store.getState().selectProject("A", "files");
  let release!: (value: ReturnType<typeof aBranch>[]) => void;
  const heldBranches = new Promise<ReturnType<typeof aBranch>[]>(resolve => { release = resolve; });
  const client = { listProjects: async () => projects, listMachines: async () => [], listHarnesses: async () => [], listBranches: () => heldBranches, listWorktrees: async () => [] } as unknown as import("../api/client").CockpitClient;
  const old = startCatalogue(client, store);
  try {
    const next = navigationScope("local-api", "owner", "", { mode: "local" }, "/new-projects", "owner")!;
    store.getState().bindProjectNavigation(next);
    store.getState().setProjects([aProject({ id: "A", path: "/new-projects/A" })]);
    release([aBranch({ id: "old-branch", projectId: "A" })]);
    for (let i = 0; i < 6; i++) await Promise.resolve();
    expect(store.getState().projects[0].path).toBe("/new-projects/A");
    expect(store.getState().branchesByProject.A).toBeUndefined();
  } finally { old.stop(); }
});
