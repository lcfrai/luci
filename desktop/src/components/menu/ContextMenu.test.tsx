// The one right-click menu, for all four row kinds. This is the file TASK 22 edits directly,
// so a defect here is a defect task 22 inherits and builds on. It also has the most branching
// in the task — four item sets, the disabled-with-reason row, the act() close-then-run wrapper
// and the branch-with-no-worktree case — and a hook-ordering constraint that breaks silently:
// the four useCockpit reads sit ABOVE the `if (target === null) return null` early return,
// because a hook called after a conditional return throws "Rendered fewer hooks than expected"
// the first time the menu closes.
//
// No jest-dom (shared-shape §3C): plain DOM property assertions only.
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ContextMenu } from "./ContextMenu";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../../test/fixtures";
import type { ContextMenuTarget } from "../../store/types";

const { createSession } = vi.hoisted(() => ({ createSession: vi.fn() }));
vi.mock("../../api", () => ({ api: { createSession } }));

const project = aProject({ id: "p-1", name: "portal", path: "C:\\work\\portal" });
const branch = aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload" });
const worktree = aWorktree({
  id: "w-1",
  projectId: "p-1",
  branchId: "b-1",
  name: "wt-evidence",
  path: "C:\\work\\wt\\evidence",
});

function seed(target: ContextMenuTarget, withWorktree = true): void {
  resetCockpit();
  useCockpit.setState({
    projects: [project],
    branchesByProject: { "p-1": [branch] },
    worktreesByBranch: withWorktree ? { "b-1": [worktree] } : {},
    // branchId matches the seeded branch: a session naming a branch the store does not hold is
    // a fixture the control plane cannot produce, and task 22's Fork reads through it.
    sessions: { "s-1": aSession({ id: "s-1", worktreeId: "w-1", branchId: "b-1" }) },
    sessionIdsByWorktree: { "w-1": ["s-1"] },
    contextMenu: target,
  });
}

const at = { x: 10, y: 10 };
const labels = (): string[] =>
  screen.getAllByRole("menuitem").map((el) => el.textContent ?? "");

describe("ContextMenu", () => {
  beforeEach(() => createSession.mockReset());

  it("renders nothing at all when no row has been right-clicked", () => {
    resetCockpit();
    render(<ContextMenu />);
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("gives each row kind its own verb set", () => {
    seed({ kind: "project", id: "p-1", ...at });
    const { unmount } = render(<ContextMenu />);
    expect(labels()).toEqual(["New branch", "Copy path", "Reveal in Explorer"]);
    unmount();

    seed({ kind: "branch", id: "b-1", ...at });
    const second = render(<ContextMenu />);
    expect(labels()).toEqual([
      "New session here",
      "Copy branch name",
      "Set branch defaults…",
    ]);
    second.unmount();

    // "Mark ready" is UNCONDITIONAL — §10.2 landed, so `ready` is always on the wire and there
    // is no world in which the row is absent (contract §6.2, §8 defect 9 as amended).
    seed({ kind: "worktree", id: "w-1", ...at });
    const third = render(<ContextMenu />);
    expect(labels()).toEqual([
      "New session here",
      "Open a shell here",
      "Copy path",
      "Mark ready",
    ]);
    third.unmount();

    // Task 22's lifecycle verbs, from ../develop/sessionVerbs. Resume is disabled because no
    // harness is seeded here, and Fork because "b-1" has a single worktree — both render with
    // a reason rather than vanishing, which is §6.4.
    seed({ kind: "session", id: "s-1", ...at });
    render(<ContextMenu />);
    expect(labels()).toEqual([
      "Resume",
      "Fork into…",
      "Restart",
      "Rename…",
      "Open a shell here",
      "Kill",
      "Forget",
      "Copy session id",
    ]);
  });

  // The §6.7 amendment this task argues for: <SessionMenu/> is mounted HERE, not in AppShell,
  // because shared-shape §1 gives AppShell's edit row to task 23. Unproven anywhere else, and
  // the ⋯ menu must be able to open while no right-click menu is up — the normal case.
  it("mounts the ⋯ menu from sessionMenuFor, with no right-click menu open", () => {
    seed({ kind: "session", id: "s-1", ...at });
    useCockpit.getState().closeContextMenu();
    useCockpit.getState().openSessionMenu("s-1");
    render(<ContextMenu />);

    expect(useCockpit.getState().contextMenu).toBeNull();
    expect(screen.getByRole("menuitem", { name: "Kill" })).not.toBeNull();
  });

  // §6.4 and §7.7: a control the user cannot operate must say WHY, and §6.2 puts that reason
  // in the tooltip rather than on a second line, which would reflow the menu.
  it("disables New branch and carries its reason in the tooltip", () => {
    seed({ kind: "project", id: "p-1", ...at });
    render(<ContextMenu />);
    const item = screen.getByRole("menuitem", { name: "New branch" });
    expect(item.hasAttribute("disabled")).toBe(true);
    expect(item.getAttribute("title") ?? "").toContain("no create-branch endpoint");

    // And the enabled rows are genuinely enabled, so "disabled" is a decision and not the
    // default that every row happens to get.
    expect(
      screen.getByRole("menuitem", { name: "Copy path" }).hasAttribute("disabled"),
    ).toBe(false);
  });

  it("disables New session here for a branch with no worktrees, and says so", () => {
    seed({ kind: "branch", id: "b-1", ...at }, false);
    render(<ContextMenu />);
    const item = screen.getByRole("menuitem", { name: "New session here" });
    expect(item.hasAttribute("disabled")).toBe(true);
    expect(item.getAttribute("title")).toBe("This branch has no worktrees yet.");
  });

  // The act() wrapper closes the menu BEFORE running the verb. A menu that stayed open behind
  // the thing it launched would be a second surface the user has to dismiss.
  it("closes the menu when a verb runs", async () => {
    seed({ kind: "worktree", id: "w-1", ...at });
    render(<ContextMenu />);
    expect(useCockpit.getState().contextMenu).not.toBeNull();
    await userEvent.click(screen.getByRole("menuitem", { name: "Copy path" }));
    expect(useCockpit.getState().contextMenu).toBeNull();
  });

  // §6.2's "Set branch defaults…" has no surface of its own; it selects the branch and opens
  // the picker, which is the §6.4 amendment this task argued for.
  it("routes Set branch defaults to the picker, with the branch selected", async () => {
    seed({ kind: "branch", id: "b-1", ...at });
    render(<ContextMenu />);
    await userEvent.click(screen.getByRole("menuitem", { name: "Set branch defaults…" }));
    expect(useCockpit.getState().selection.branchId).toBe("b-1");
    expect(useCockpit.getState().pickerOpen).toBe(true);
  });

  // The hook-ordering constraint, exercised rather than asserted: closing the menu re-renders
  // the component with target === null. If the four useCockpit reads were below the early
  // return, React would throw "Rendered fewer hooks than expected" on exactly this transition.
  it("survives being closed, which is where a misplaced hook would throw", () => {
    seed({ kind: "project", id: "p-1", ...at });
    const { rerender } = render(<ContextMenu />);
    expect(screen.queryByRole("menu")).not.toBeNull();
    useCockpit.getState().closeContextMenu();
    rerender(<ContextMenu />);
    expect(screen.queryByRole("menu")).toBeNull();
  });
});
