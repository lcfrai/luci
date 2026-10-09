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
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AppShell } from "./AppShell";
import { LENSES } from "./subrail/SubRail";
import { resetCockpit, useCockpit } from "../store/store";
import {
  TERMINAL_HOST_ATTRIBUTE,
  isInsideTerminal,
} from "../hooks/useKeyboardMap";
import {
  aBranch,
  aHarness,
  aMachine,
  aProject,
  aSession,
  aWorktree,
} from "../test/fixtures";

const { createSession } = vi.hoisted(() => ({ createSession: vi.fn() }));
vi.mock("../api", () => ({ api: { createSession } }));

// ADDED BY TASK 20 with the DevelopLens mount. `populate()` gives the branch a session, so
// the develop arm now reaches <Terminal/> and, unmocked, a real xterm plus a live attach
// ladder (250 ms rungs for 60 s, surviving the 5 s grace after unmount) would run inside a
// suite about composition. §1.5: a component test never renders a glyph, and
// group-d-real-browser-carries §8 puts the real terminal in task 24's window, not in jsdom.
//
// EDITED BY OD-9's FIX ROUND 1: the stand-in now carries what the keyboard map and the pane
// read from a real one, and nothing else: the terminal-host marker, and xterm's helper
// textarea, which is the element a keystroke lands on. The literal is checked against
// TERMINAL_HOST_ATTRIBUTE in the test that relies on it (a vi.mock factory cannot import).
vi.mock("./Terminal", () => ({
  Terminal: ({ sessionId }: { sessionId: string }) => (
    <div data-testid="terminal" data-session={sessionId} data-terminal-host="">
      <textarea
        className="xterm-helper-textarea"
        aria-label={`terminal ${sessionId}`}
      />
    </div>
  ),
}));

function populate(): void {
  useCockpit.setState({
    machines: [aMachine()],
    harnesses: [aHarness()],
    projects: [aProject({ id: "p-1", name: "portal" })],
    branchesByProject: {
      "p-1": [aBranch({ id: "b-1", projectId: "p-1", name: "evidence" })],
    },
    worktreesByBranch: {
      "b-1": [aWorktree({ id: "w-1", branchId: "b-1", name: "wt-one" })],
    },
    sessions: { "s-1": aSession({ id: "s-1", projectId: "p-1", branchId: "b-1", worktreeId: "w-1" }) },
    sessionIdsByWorktree: { "w-1": ["s-1"] },
    selection: {
      projectId: "p-1",
      branchId: "b-1",
      worktreeId: "w-1",
      sessionId: null,
    },
  });
}

describe("AppShell", () => {
  beforeEach(() => {
    createSession.mockReset();
    resetCockpit();
  });

  it("starts at Projects home with direct creation and import actions", () => {
    render(<AppShell />);
    expect(screen.getByRole("region", { name: "Projects" })).not.toBeNull();
    expect(screen.queryByRole("tree")).toBeNull();
    expect(
      screen.getByRole("heading", { name: "Your projects" }),
    ).not.toBeNull();
    expect(screen.getByRole("button", { name: "New project" })).not.toBeNull();
    expect(
      screen.getByRole("button", { name: "Import repository" }),
    ).not.toBeNull();
    expect(
      screen.getByRole("button", { name: "Restore backup" }),
    ).not.toBeNull();
    expect(screen.queryByRole("button", { name: /new session/i })).toBeNull();
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
    expect(
      screen.getByRole("complementary", { name: "Phases" }),
    ).not.toBeNull();
    // And with a branch, the same slot is the sub-rail rather than the placeholder.
    expect(middleSlot()!.tagName).toBe("ASIDE");
    // It names the branch it is about. Added in task 20's fix round 1, which pointed this
    // derivation at `selectSelectedBranch` (shared-shape §4) — before it, blanking the name
    // outright passed the whole suite, so the sub-rail's one piece of context was unasserted.
    expect(container.querySelector(".sub-branch")?.textContent).toBe(
      "evidence",
    );
  });

  // §6.3 and AGENTIC-UX §4 rule 12: entry into any lens is legal from any state. Not one of
  // the six may be disabled, numbered or marked complete.
  it("offers all four journey sections and keeps Files and Git inside Develop", async () => {
    populate();
    render(<AppShell />);
    for (const lens of LENSES) {
      const row = screen.getByRole("button", { name: lens.label });
      expect(row.hasAttribute("disabled")).toBe(false);
    }
    expect(LENSES).toHaveLength(4);

    // And every one routes to something honest rather than a dead panel.
    await userEvent.click(screen.getByRole("button", { name: "Git" }));
    expect(useCockpit.getState().lens).toBe("merge");
    expect(
      screen.getByRole("heading", { name: "Review the work." }),
    ).not.toBeNull();
  });

  // §6.4: these two are in FUNCTIONAL-SPEC's top bar, so deleting them would be a silent scope
  // cut and rendering them live would be a lie. Disabled with the phase that makes each real.
  it("offers real Develop tools without dead controls", () => {
    populate();
    render(<AppShell />);
    expect(
      screen.getByRole("navigation", { name: "Develop tools" }),
    ).not.toBeNull();
    expect(screen.queryByRole("button", { name: "Authority" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Run locally" })).toBeNull();
  });

  it("keeps a projectless account sign-in terminal reachable without a fabricated project", () => {
    useCockpit.setState({
      sessions: {
        auth: aSession({
          id: "auth",
          harness: "codex",
          authOnly: true,
          projectId: "",
          branchId: "",
          worktreeId: "setup-codex",
        }),
      },
    });
    useCockpit.getState().focusSession("auth");
    render(<AppShell />);
    expect(
      screen.getByRole("heading", { name: "Connect your Codex account" }),
    ).not.toBeNull();
    expect(
      screen.getByRole("button", { name: "Return to agent connection" }),
    ).not.toBeNull();
    expect(screen.getByLabelText("terminal auth")).not.toBeNull();
    expect(screen.queryByRole("heading", { name: "Your projects" })).toBeNull();
  });

  it("mounts exactly one toast stack, and shows an Undo when one is offered", () => {
    render(<AppShell />);
    expect(document.querySelectorAll(".toast-stack")).toHaveLength(1);
    // act(), because pushToast is a store write from outside React and the stack has to
    // re-render before the assertion can see it.
    act(() => useCockpit.getState().pushToast("killed the session", () => {}));
    expect(screen.getByRole("button", { name: "Undo" })).not.toBeNull();
    expect(screen.getByRole("status").textContent).toContain(
      "killed the session",
    );
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

  /* OD-9's fix round 1. The toast that turns "Send shortcut keys to the terminal" on names ONE
   * way back to the palette, and this walks it. user-event's press moves focus the way a
   * browser's does: to the closest focusable ancestor, or to <body>. The pane-header row is why
   * the toast no longer says "click outside the terminal": that click IS outside the terminal,
   * and the pane hands focus straight back to it (§6.8's second mover), where ⌘K is the agent's.
   * So a surface that starts moving focus into a pane on a click turns the top-bar row red.
   * The click lands on `header.top` itself, not on `.top-spacer`: the spacer is an empty span
   * whose auto margin is the gap, so the empty part a user clicks is the header's own box. */
  it.each([
    {
      route: "an empty part of the top bar",
      selector: "header.top",
      opens: true,
    },
    {
      route: "the focused pane's own header",
      selector: ".pane-head",
      opens: false,
    },
  ])(
    "OD-9 on, a terminal focused: a click on $route, then ⌘K, opens the palette: $opens",
    async ({ selector, opens }) => {
      expect(TERMINAL_HOST_ATTRIBUTE).toBe("data-terminal-host");
      populate();
      useCockpit.getState().focusSession("s-1");
      useCockpit.getState().setSendChordsToTerminal(true);
      render(<AppShell />);
      const textarea = screen.getByLabelText("terminal s-1");
      textarea.focus();
      expect(isInsideTerminal(document.activeElement)).toBe(true);
      const surface = document.querySelector(selector);
      expect(surface).not.toBeNull();
      await userEvent.click(surface as Element);
      expect(isInsideTerminal(document.activeElement)).toBe(!opens);
      await userEvent.keyboard("{Control>}k{/Control}");
      expect(useCockpit.getState().paletteOpen).toBe(opens);
    },
  );

  it("collapses the rail and deepens for a branch through class state alone", () => {
    const { container, rerender } = render(<AppShell />);
    const shell = container.querySelector(".shell");
    expect(shell).not.toBeNull();
    expect(shell!.className).not.toContain("shell-deep");
    expect(shell!.className).not.toContain("shell-rail-collapsed");
    expect(shell!.className).not.toContain("grain");

    populate();
    useCockpit.getState().toggleRail();
    rerender(<AppShell />);
    expect(shell!.className).toContain("shell-deep");
    expect(shell!.className).toContain("shell-rail-collapsed");
  });

  it("opens compact project navigation, isolates the workspace, traps focus and returns it on Escape", async () => {
    const media = vi
      .spyOn(window, "matchMedia")
      .mockImplementation((query) => ({
        matches: query === "(max-width: 760px)",
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: () => false,
      }));
    try {
      populate();
      const { container } = render(<AppShell />);
      const trigger = screen.getByRole("button", {
        name: "Projects",
      });
      expect(container.querySelector(".rail")?.hasAttribute("inert")).toBe(
        true,
      );
      await userEvent.click(trigger);
      expect(trigger.getAttribute("aria-expanded")).toBe("true");
      expect(
        container.querySelector(".shell-main")?.hasAttribute("inert"),
      ).toBe(true);
      expect(document.activeElement).toBe(
        screen.getByRole("button", { name: "Close projects" }),
      );
      const last = screen.getByRole("button", {
        name: /^Commands /,
      });
      last.focus();
      await userEvent.tab();
      expect(document.activeElement).toBe(
        screen.getByRole("button", { name: "All projects" }),
      );
      await userEvent.tab({ shift: true });
      expect(document.activeElement).toBe(last);
      await userEvent.keyboard("{Escape}");
      expect(trigger.getAttribute("aria-expanded")).toBe("false");
      expect(document.activeElement).toBe(trigger);
      expect(
        container.querySelector(".shell-main")?.hasAttribute("inert"),
      ).toBe(false);
    } finally {
      media.mockRestore();
    }
  });

  it("closes compact navigation after selecting a different project", async () => {
    const media = vi
      .spyOn(window, "matchMedia")
      .mockImplementation((query) => ({
        matches: query === "(max-width: 760px)",
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: () => false,
      }));
    try {
      populate();
      useCockpit.setState((state) => ({
        projects: [...state.projects, aProject({ id: "p-2", name: "garden" })],
      }));
      render(<AppShell />);
      const trigger = screen.getByRole("button", {
        name: "Projects",
      });
      await userEvent.click(trigger);
      await userEvent.click(screen.getByRole("treeitem", { name: /garden/ }));
      expect(trigger.getAttribute("aria-expanded")).toBe("false");
      expect(useCockpit.getState().selection.projectId).toBe("p-2");
    } finally {
      media.mockRestore();
    }
  });
  it.each([
    [true, "Keep editing"], [true, "Escape"],
    [false, "Keep editing"], [false, "Escape"],
  ] as const)("returns guarded navigation focus to a usable control (compact=%s, close=%s)", async (compact, close) => {
    const media = vi.spyOn(window, "matchMedia").mockImplementation(query => ({
      matches: compact && query === "(max-width: 760px)", media: query,
      onchange: null, addEventListener: vi.fn(), removeEventListener: vi.fn(),
      addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: () => false,
    }));
    try {
      populate();
      useCockpit.getState().setUnsavedProjectDraft("retained-definition", {
        projectId: "p-1", label: "Definition", lens: "discover", retained: true,
      });
      render(<AppShell />);
      const projects = screen.getByRole("button", { name: "Projects" });
      if (compact) await userEvent.click(projects);
      const allProjects = screen.getByRole("button", { name: "All projects" });
      await userEvent.click(allProjects);
      const dialog = await screen.findByRole("dialog", { name: "Keep your changes?" });
      await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true));
      expect(useCockpit.getState().selection.projectId).toBe("p-1");
      if (close === "Escape") await userEvent.keyboard("{Escape}");
      else await userEvent.click(screen.getByRole("button", { name: "Keep editing" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
      await waitFor(() => expect(document.activeElement).toBe(compact ? projects : allProjects));
      expect(document.activeElement?.closest("[inert]")).toBeNull();
      expect(useCockpit.getState().selection.projectId).toBe("p-1");
      expect(useCockpit.getState().pendingNavigation).toBeNull();
      expect(useCockpit.getState().unsavedProjectDrafts["retained-definition"]).toEqual({
        projectId: "p-1", label: "Definition", lens: "discover", retained: true,
      });
    } finally { media.mockRestore(); }
  });

  it.each(["disabled", "inert", "hidden"])("refuses an unusable compact focus fallback (%s)", async unavailable => {
    const media = vi.spyOn(window, "matchMedia").mockImplementation(query => ({
      matches: query === "(max-width: 760px)", media: query,
      onchange: null, addEventListener: vi.fn(), removeEventListener: vi.fn(),
      addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: () => false,
    }));
    try {
      populate();
      useCockpit.getState().setUnsavedProjectDraft("retained-definition", {
        projectId: "p-1", label: "Definition", lens: "discover", retained: true,
      });
      render(<AppShell />);
      const projects = screen.getByRole("button", { name: "Projects" });
      await userEvent.click(projects);
      await userEvent.click(screen.getByRole("button", { name: "All projects" }));
      const dialog = await screen.findByRole("dialog", { name: "Keep your changes?" });
      await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true));
      projects.setAttribute(unavailable, "");
      const fallbackFocus = vi.spyOn(projects, "focus");
      await userEvent.click(screen.getByRole("button", { name: "Keep editing" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
      // Radix close autofocus is deferred until its focus trap is removed.
      await new Promise(resolve => setTimeout(resolve, 30));
      expect(fallbackFocus).not.toHaveBeenCalled();
      expect(useCockpit.getState().selection.projectId).toBe("p-1");
      expect(useCockpit.getState().unsavedProjectDrafts["retained-definition"]?.retained).toBe(true);
    } finally { media.mockRestore(); }
  });
});
