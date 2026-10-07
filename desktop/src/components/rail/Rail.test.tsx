import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { Rail } from "./Rail";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../../test/fixtures";
import type { CockpitState } from "../../store/store";
import { personalPermissions } from "../../api/teams";
import type { useTeamWorkspace } from "../teams/TeamWorkspaceProvider";

const teamWorkspace = vi.hoisted(() => ({ value: null as ReturnType<typeof useTeamWorkspace> }));
const setup = vi.hoisted(() => ({ openProject: vi.fn(), openSettings: vi.fn() }));
vi.mock('../teams/TeamWorkspaceProvider', async importOriginal => ({
  ...await importOriginal<typeof import('../teams/TeamWorkspaceProvider')>(),
  useTeamWorkspace: () => teamWorkspace.value,
}));
vi.mock('../setup/SetupProvider', () => ({ useSetup: () => setup }));
function teamScope(edit: boolean, admin: boolean): NonNullable<ReturnType<typeof useTeamWorkspace>> {
  const selected = {
    team: { id: 'team-aaaaaaaaaaaaaaaaaaaaaa', name: 'Shared team', revision: 1, createdAt: '', updatedAt: '' },
    membership: { userId: 'alice', role: 'editor' as const, grants: { execute: false, secrets: false, deploy: false, admin }, revision: 1 },
    permissions: { ...personalPermissions, edit, admin },
  };
  return { teamId: selected.team.id, managedTeamId: selected.team.id, selected, teams: [selected], revision: 1, refresh: vi.fn().mockResolvedValue(undefined), select: vi.fn() };
}

const project = aProject({
  id: "p-1",
  name: "engagement-portal",
  path: "C:\\work\\portal",
});
const branch = aBranch({
  id: "b-1",
  projectId: "p-1",
  name: "evidence-upload",
  isHead: false,
});
const worktree = aWorktree({
  id: "w-1",
  projectId: "p-1",
  branchId: "b-1",
  name: "wt-evidence",
  path: "C:\\work\\wt\\evidence",
  sessionCount: 2,
});
const base = {
  projectId: "p-1",
  branchId: "b-1",
  worktreeId: "w-1",
  harness: "claude-code",
};
const running = aSession({
  ...base,
  id: "s-run",
  status: "running",
  title: "wire the upload route",
});
const idling = aSession({
  ...base,
  id: "s-idle",
  status: "idle",
  title: "read the spec",
});

function seed(over: Partial<CockpitState> = {}): void {
  resetCockpit();
  useCockpit.setState({
    projects: [project],
    branchesByProject: { "p-1": [branch] },
    worktreesByBranch: { "b-1": [worktree] },
    sessions: { "s-run": running, "s-idle": idling },
    sessionIdsByWorktree: { "w-1": ["s-run", "s-idle"] },
    expandedProjects: { "p-1": true },
    expandedBranches: { "b-1": true },
    expandedWorktrees: { "w-1": true },
    ...over,
  });
}

describe("Rail", () => {
  beforeEach(() => { seed(); teamWorkspace.value = null; setup.openProject.mockClear(); setup.openSettings.mockClear(); });

  it('does not invite a nonadmin team member to create a project from the sidebar', async () => {
    seed({ projects: [] }); teamWorkspace.value = teamScope(true, false);
    render(<Rail />);
    const create = screen.getByRole('button', { name: 'Create project' });
    expect(create).toHaveProperty('disabled', true);
    await userEvent.click(create);
    expect(setup.openProject).not.toHaveBeenCalled();
    expect(within(screen.getByRole('region', { name: 'Projects' })).getByText('No projects shared yet. Ask your team administrator to share a project.')).toBeTruthy();
    expect(screen.queryByText(/Create a project or import a repository from your/)).toBeNull();
    expect(create).toHaveProperty('title', 'Ask your team administrator to create or share a project.');
    const result = await axe.run(document, { runOnly: { type: 'rule', values: ['aria-required-children', 'button-name'] } });
    expect(result.violations.map(rule => rule.id)).toEqual([]);
  });

  it.each([[true, true, true], [true, false, false], [false, true, false], [false, false, false]])('requires current edit=%s and admin=%s for sidebar creation (allowed=%s)', async (edit, admin, allowed) => {
    teamWorkspace.value = teamScope(edit, admin);
    render(<Rail />);
    const create = screen.getByRole('button', { name: 'Create project' });
    expect(create).toHaveProperty('disabled', !allowed);
    await userEvent.click(create);
    expect(setup.openProject).toHaveBeenCalledTimes(allowed ? 1 : 0);
  });

  it('updates permission and empty-state guidance when switching from personal to a team and after access changes', async () => {
    seed({ projects: [] });
    const view = render(<Rail />);
    const create = screen.getByRole('button', { name: 'Create project' });
    expect(create).toHaveProperty('disabled', false);
    await userEvent.click(create);
    expect(setup.openProject).toHaveBeenCalledTimes(1);
    teamWorkspace.value = teamScope(true, false);
    view.rerender(<Rail />);
    expect(create).toHaveProperty('disabled', true);
    expect(within(screen.getByRole('region', { name: 'Projects' })).getByText(/No projects shared yet/)).toBeTruthy();
    await userEvent.click(create);
    expect(setup.openProject).toHaveBeenCalledTimes(1);
    teamWorkspace.value = teamScope(true, true);
    view.rerender(<Rail />);
    expect(create).toHaveProperty('disabled', false);
    expect(within(screen.getByRole('region', { name: 'Projects' })).getByText(/Create a project or import/)).toBeTruthy();
    teamWorkspace.value = { ...teamScope(true, true), selected: null };
    view.rerender(<Rail />);
    expect(create).toHaveProperty('disabled', true);
    teamWorkspace.value = null;
    view.rerender(<Rail />);
    expect(create).toHaveProperty('disabled', false);
  });

  it('keeps keyboard tree navigation available when the creation action is disabled', async () => {
    teamWorkspace.value = teamScope(false, false);
    render(<Rail />);
    screen.getByRole('button', { name: 'All projects' }).focus();
    await userEvent.tab();
    const rows = screen.getAllByRole('treeitem');
    expect(document.activeElement).toBe(rows[0]);
    await userEvent.keyboard('{ArrowDown}');
    expect(document.activeElement).toBe(rows[1]);
    expect(setup.openProject).not.toHaveBeenCalled();
  });

  it("labels an empty project catalogue without exposing a tree missing its required items", async () => {
    seed({ projects: [] });
    render(<Rail />);
    const result = await axe.run(document, { runOnly: { type: "rule", values: ["aria-required-children"] } });
    expect(result.violations.map(rule => rule.id)).toEqual([]);
    expect(screen.queryByRole("tree")).toBeNull();
    const empty = screen.getByRole("region", { name: "Projects" });
    expect(within(empty).getByText(/No projects yet/)).not.toBeNull();
    expect(screen.getByRole("button", { name: "Create project" })).not.toBeNull();
  });

  it("retains tree navigation when projects arrive and returns to a labelled region after the last project disappears", async () => {
    seed({ projects: [] });
    render(<Rail />);
    expect(screen.getByRole("region", { name: "Projects" })).not.toBeNull();
    act(() => useCockpit.setState({ projects: [project] }));
    const tree = screen.getByRole("tree", { name: "Projects" });
    const rows = within(tree).getAllByRole("treeitem");
    expect(rows.filter(row => row.tabIndex === 0)).toEqual([rows[0]]);
    rows[0].focus();
    await userEvent.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(rows[1]);
    act(() => useCockpit.setState({ projects: [] }));
    expect(screen.queryByRole("tree")).toBeNull();
    expect(within(screen.getByRole("region", { name: "Projects" })).getByText(/No projects yet/)).not.toBeNull();
  });

  it("nests project, branch, worktree and session as four levels", () => {
    render(<Rail />);
    const levelOf = (name: RegExp): string | null =>
      screen.getByRole("treeitem", { name }).getAttribute("aria-level");
    expect(levelOf(/engagement-portal/)).toBe("1");
    expect(levelOf(/evidence-upload$/)).toBe("2");
    expect(levelOf(/wt-evidence/)).toBe("3");
    expect(levelOf(/wire the upload route/)).toBe("4");
  });

  it("selects a branch and sets the lens to develop in one click", async () => {
    render(<Rail />);
    await userEvent.click(
      screen.getByRole("treeitem", { name: /evidence-upload$/ }),
    );
    expect(useCockpit.getState().selection.branchId).toBe("b-1");
    expect(useCockpit.getState().lens).toBe("develop");
  });

  it("gives the branch the highest-attention status among its sessions", () => {
    render(<Rail />);
    expect(
      screen.getByRole("img", { name: "running · evidence-upload" }),
    ).toBeDefined();
    expect(
      screen.queryByRole("img", { name: "idle · evidence-upload" }),
    ).toBeNull();
  });

  it("keeps a worktree expandable when only exited sessions remain", () => {
    const done = aSession({
      ...base,
      id: "s-done",
      status: "exited",
      title: "read the spec",
    });
    seed({
      worktreesByBranch: { "b-1": [{ ...worktree, sessionCount: 0 }] },
      sessions: { "s-done": done },
      sessionIdsByWorktree: { "w-1": ["s-done"] },
    });
    render(<Rail />);
    const row = screen.getByRole("treeitem", { name: /wt-evidence/ });
    expect(row.getAttribute("aria-expanded")).toBe("true");
    expect(
      screen.getByRole("treeitem", { name: /read the spec/ }),
    ).toBeDefined();
  });

  // The other arm of the same OR. Step 11's departure is "sessionCount > 0 OR the store
  // already holds session ids", and each arm carries a case the other cannot: the test above
  // covers sessionCount === 0 with ids present, this one covers sessionCount > 0 with no ids
  // fetched yet — the first paint after the catalogue lands and before the session list does.
  // Measured: without it, collapsing hasChildren to `sessionIds.length > 0` survives every
  // other test in this file, which is a worktree that silently loses its chevron until a poll
  // completes. Group D shared-shape §3A.1 rule 3: a two-condition guard pins BOTH collapses.
  it("draws the chevron before the session list has been fetched", () => {
    seed({ sessions: {}, sessionIdsByWorktree: {} });
    render(<Rail />);
    const row = screen.getByRole("treeitem", { name: /wt-evidence/ });
    expect(row.getAttribute("aria-expanded")).toBe("true");
  });

  // The negative case, which is what BOUNDS the departure. Widening when the chevron is drawn
  // is only a departure and not a free-for-all if a worktree with nothing under it still draws
  // none: §6.2 gives aria-expanded to a row that has children, and an expand affordance that
  // reveals an empty group is a control that lies. Measured: without this, `hasChildren = true`
  // survives both collapse mutants above.
  it("draws no chevron for a worktree with nothing under it", () => {
    seed({
      worktreesByBranch: { "b-1": [{ ...worktree, sessionCount: 0 }] },
      sessions: {},
      sessionIdsByWorktree: {},
    });
    render(<Rail />);
    const row = screen.getByRole("treeitem", { name: /wt-evidence/ });
    expect(row.hasAttribute("aria-expanded")).toBe(false);
  });

  it("carries names and no numbers", () => {
    render(<Rail />);
    expect(screen.getByRole("tree").textContent ?? "").not.toMatch(/\d/);
  });

  it("moves with the arrow keys and collapses with ArrowLeft", async () => {
    render(<Rail />);
    const rows = screen.getAllByRole("treeitem");
    rows[0].focus();
    await userEvent.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(rows[1]);
    await userEvent.keyboard("{ArrowLeft}");
    expect(useCockpit.getState().expandedBranches["b-1"]).toBe(false);
  });

  it("offers one keyboard entry point and moves it with arrow navigation", async () => {
    render(<Rail />);
    const rows = screen.getAllByRole("treeitem");
    expect(rows.filter((row) => row.tabIndex === 0)).toEqual([rows[0]]);
    rows[0].focus();
    await userEvent.keyboard("{ArrowDown}");
    expect(document.activeElement).toBe(rows[1]);
    expect(rows.filter((row) => row.tabIndex === 0)).toEqual([rows[1]]);
    await userEvent.tab();
    expect(document.activeElement).not.toBe(rows[0]);
  });

  // ── BranchRow's expansion guard, both collapses ────────────────────────────────────
  // `if (worktrees.length > 0) toggleBranch(id)` has two degenerate forms and each needs its
  // own case: toggling unconditionally (an empty ARIA group under a childless branch) and
  // never toggling (a branch that cannot be opened at all).

  it("does not toggle a childless branch, but still selects it", async () => {
    seed({
      worktreesByBranch: {},
      sessions: {},
      sessionIdsByWorktree: {},
      expandedBranches: {},
    });
    render(<Rail />);
    await userEvent.click(
      screen.getByRole("treeitem", { name: /evidence-upload$/ }),
    );
    expect(useCockpit.getState().selection.branchId).toBe("b-1");
    expect(useCockpit.getState().expandedBranches["b-1"]).toBeUndefined();
  });

  it("does toggle a branch that has worktrees", async () => {
    seed({ expandedBranches: {} });
    render(<Rail />);
    await userEvent.click(
      screen.getByRole("treeitem", { name: /evidence-upload$/ }),
    );
    expect(useCockpit.getState().expandedBranches["b-1"]).toBe(true);
  });

  // ── ArrowRight's three states ──────────────────────────────────────────────────────
  // aria-expanded is "false", "true" or ABSENT, and the third is a leaf. ARIA APG says
  // ArrowRight does nothing on an end node; without the `=== "true"` guard the null case fell
  // through to the else and moved focus down, duplicating ArrowDown.

  it("expands a collapsed row with ArrowRight rather than moving", async () => {
    seed({ expandedBranches: {} });
    render(<Rail />);
    const rows = screen.getAllByRole("treeitem");
    rows[1].focus();
    await userEvent.keyboard("{ArrowRight}");
    expect(useCockpit.getState().expandedBranches["b-1"]).toBe(true);
    expect(document.activeElement).toBe(rows[1]);
  });

  it("moves to the first child with ArrowRight on an already-expanded row", async () => {
    render(<Rail />);
    const rows = screen.getAllByRole("treeitem");
    rows[1].focus();
    await userEvent.keyboard("{ArrowRight}");
    expect(document.activeElement).toBe(rows[2]);
  });

  it("does nothing on ArrowRight at a leaf", async () => {
    render(<Rail />);
    const rows = screen.getAllByRole("treeitem");
    const leaf = rows[rows.length - 1];
    expect(leaf.hasAttribute("aria-expanded")).toBe(false);
    leaf.focus();
    await userEvent.keyboard("{ArrowRight}");
    expect(document.activeElement).toBe(leaf);
  });

  it("dispatches every right-click through the one context menu", () => {
    render(<Rail />);
    fireEvent.contextMenu(
      screen.getByRole("treeitem", { name: /evidence-upload$/ }),
    );
    expect(useCockpit.getState().contextMenu?.kind).toBe("branch");
    expect(useCockpit.getState().contextMenu?.id).toBe("b-1");
  });
});
