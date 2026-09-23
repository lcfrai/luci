// The composition root. One mount here smokes every surface this task ships that has no test
// of its own — SubRail, TopBar, LayoutToggle, LensPlaceholder, Toasts, Popover and the machine
// chip's populated branch — which is precisely what the throwaway suite did before it was
// deleted. This is the permanent version of it.
//
// AppShell is also the file tasks 20 and 23 both edit, and the one task 24 renders, so a break
// here is a break three later tasks inherit.
//
// No jest-dom (shared-shape §3C): plain DOM property assertions only.
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AppShell } from "./AppShell";
import { LENSES } from "./subrail/SubRail";
import { resetCockpit, useCockpit } from "../store/store";
import { aBranch, aHarness, aMachine, aProject, aSession, aWorktree } from "../test/fixtures";

const { createSession } = vi.hoisted(() => ({ createSession: vi.fn() }));
vi.mock("../api", () => ({ api: { createSession } }));

// ADDED BY TASK 20 with the DevelopLens mount. `populate()` gives the branch a session, so
// the develop arm now reaches <Terminal/> and, unmocked, a real xterm plus a live attach
// ladder (250 ms rungs for 60 s, surviving the 5 s grace after unmount) would run inside a
// suite about composition. §1.5: a component test never renders a glyph, and
// group-d-real-browser-carries §8 puts the real terminal in task 24's window, not in jsdom.
vi.mock("./Terminal", () => ({
  Terminal: ({ sessionId }: { sessionId: string }) => (
    <div data-testid="terminal" data-session={sessionId} />
  ),
}));

function populate(): void {
  useCockpit.setState({
    machines: [aMachine()],
    harnesses: [aHarness()],
    projects: [aProject({ id: "p-1", name: "portal" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1", name: "evidence" })] },
    worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", branchId: "b-1", name: "wt-one" })] },
    sessions: { "s-1": aSession({ id: "s-1", worktreeId: "w-1" }) },
    sessionIdsByWorktree: { "w-1": ["s-1"] },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: "w-1", sessionId: null },
  });
}

describe("AppShell", () => {
  beforeEach(() => {
    createSession.mockReset();
    resetCockpit();
  });

  it("mounts with an empty store and renders the rail, the bar and a lens", () => {
    render(<AppShell />);
    expect(screen.getByRole("tree")).not.toBeNull();
    expect(screen.getByRole("group", { name: "Layout" })).not.toBeNull();
    expect(screen.getByRole("button", { name: /new session/i })).not.toBeNull();
    // Develop is the default lens, and task 20 replaced its placeholder with the real
    // DevelopLens. With an empty store there is no branch, so the zero state names the rail
    // as the next action — which is the assertion that fails if the arm ever goes blank.
    expect(screen.getByText(/Select a branch in the rail/i)).not.toBeNull();
  });

  // §6.1: the grid always has three columns and the middle one animates from 0. With no branch
  // the sub-rail must NOT be mounted — it would render a back-link to nothing — but the column
  // must still exist, or the grid jumps instead of sliding.
  it("holds the sub-rail column empty until a branch is selected", () => {
    const { container, rerender } = render(<AppShell />);
    const shell = container.querySelector(".shell");
    expect(shell).not.toBeNull();
    const middleSlot = (): Element | null =>
      shell!.querySelector(".rail")?.nextElementSibling ?? null;

    // The SubRail itself must NOT be mounted — it would render a back-link to nothing.
    expect(screen.queryByRole("complementary", { name: "Phases" })).toBeNull();

    // But the COLUMN must still be occupied, which is the half this test named in its comment
    // and did not assert. The grid is three columns and the middle one animates 0px → 182px;
    // unmount the slot entirely and the grid jumps instead of sliding. Queried as "the element
    // after the rail" rather than by child index, so task 23 appending the palette cannot
    // break it — and it fails the moment the placeholder becomes null, because then the rail's
    // next sibling is <main>.
    const empty = middleSlot();
    expect(empty).not.toBeNull();
    expect(empty!.tagName).not.toBe("MAIN");
    expect(empty!.getAttribute("aria-hidden")).toBe("true");

    populate();
    rerender(<AppShell />);
    expect(screen.getByRole("complementary", { name: "Phases" })).not.toBeNull();
    // And with a branch, the same slot is the sub-rail rather than the placeholder.
    expect(middleSlot()!.tagName).toBe("ASIDE");
    // It names the branch it is about. Added in task 20's fix round 1, which pointed this
    // derivation at `selectSelectedBranch` (shared-shape §4) — before it, blanking the name
    // outright passed the whole suite, so the sub-rail's one piece of context was unasserted.
    expect(container.querySelector(".sub-branch")?.textContent).toBe("evidence");
  });

  // §6.3 and AGENTIC-UX §4 rule 12: entry into any lens is legal from any state. Not one of
  // the six may be disabled, numbered or marked complete.
  it("offers all six lenses and disables none of them", async () => {
    populate();
    render(<AppShell />);
    for (const lens of LENSES) {
      const row = screen.getByRole("button", { name: lens.label });
      expect(row.hasAttribute("disabled")).toBe(false);
    }
    expect(LENSES).toHaveLength(6);

    // And every one routes to something honest rather than a dead panel.
    await userEvent.click(screen.getByRole("button", { name: "Merge" }));
    expect(useCockpit.getState().lens).toBe("merge");
    expect(screen.getByRole("heading", { name: "merge" })).not.toBeNull();
    expect(screen.getByText(/Arrives in phase 3\./)).not.toBeNull();
  });

  // §6.4: these two are in FUNCTIONAL-SPEC's top bar, so deleting them would be a silent scope
  // cut and rendering them live would be a lie. Disabled with the phase that makes each real.
  it("is honest about the two controls it cannot implement yet", () => {
    render(<AppShell />);
    for (const [name, phase] of [["Authority", /Phase 3/], ["Run locally", /Phase 5/]] as const) {
      const button = screen.getByRole("button", { name });
      expect(button.hasAttribute("disabled")).toBe(true);
      expect(button.getAttribute("title") ?? "").toMatch(phase);
    }
  });

  it("mounts exactly one toast stack, and shows an Undo when one is offered", () => {
    render(<AppShell />);
    expect(document.querySelectorAll(".toast-stack")).toHaveLength(1);
    // act(), because pushToast is a store write from outside React and the stack has to
    // re-render before the assertion can see it.
    act(() => useCockpit.getState().pushToast("killed the session", () => {}));
    expect(screen.getByRole("button", { name: "Undo" })).not.toBeNull();
    expect(screen.getByRole("status").textContent).toContain("killed the session");
  });

  // Every row kind dispatches through the ONE <ContextMenu/> AppShell mounts; no row renders
  // its own. This is the composition that proves it, rather than ContextMenu in isolation.
  it("renders the one context menu for a right-click anywhere in the rail", async () => {
    populate();
    render(<AppShell />);
    expect(screen.queryByRole("menu")).toBeNull();
    await userEvent.pointer({
      keys: "[MouseRight]",
      target: screen.getByRole("treeitem", { name: /portal/ }),
    });
    expect(useCockpit.getState().contextMenu?.kind).toBe("project");
    expect(screen.getAllByRole("menu")).toHaveLength(1);
  });

  it("collapses the rail and deepens for a branch through class state alone", () => {
    const { container, rerender } = render(<AppShell />);
    const shell = container.querySelector(".shell");
    expect(shell).not.toBeNull();
    expect(shell!.className).not.toContain("shell-deep");
    expect(shell!.className).not.toContain("shell-rail-collapsed");
    // .grain is applied ONCE, on the shell, not per component (tokens.css).
    expect(shell!.className).toContain("grain");

    populate();
    useCockpit.getState().toggleRail();
    rerender(<AppShell />);
    expect(shell!.className).toContain("shell-deep");
    expect(shell!.className).toContain("shell-rail-collapsed");
  });
});
