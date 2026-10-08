// Contract §6.5, §6.8 and known defect 4: a session created off-screen with no focus. The
// layout chooser and the three zero states are asserted here rather than in two more files —
// DevelopLens is a short block of branching whose only observable behaviour is which of these
// three renders, and a third file would test the same three renders through a different door.
//
// No jest-dom (shared-shape §3C): plain DOM property assertions only.
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ColumnsLayout } from "./ColumnsLayout";
import { DevelopLens } from "./DevelopLens";
import { startSessionOnWorktree } from "./EmptyDevelop";
import { ApiError } from "../../api/client";
import { resetCockpit, useCockpit } from "../../store/store";
import { aBranch, aHarness, aProject, aSession, aWorktree } from "../../test/fixtures";

vi.mock("../Terminal", () => ({
  Terminal: ({ sessionId }: { sessionId: string }) => (
    <div data-testid="terminal" data-session={sessionId} />
  ),
}));

const { createSessionMock } = vi.hoisted(() => ({ createSessionMock: vi.fn() }));
vi.mock("../../api", () => ({ api: { createSession: createSessionMock } }));

const CLAUDE = aHarness({
  id: "claude-code",
  name: "Claude Code",
  command: "claude",
  available: true,
  models: [{ id: "opus", name: "Opus" }],
  defaultModel: "opus",
  efforts: ["", "high"],
  permissionModes: ["ask"],
});

const SHELL = aHarness({
  id: "shell",
  name: "Shell",
  command: "/bin/bash",
  available: true,
  models: [],
  defaultModel: "",
  efforts: [],
  permissionModes: [],
});

/** One project, one branch, and as many worktrees/sessions as the case needs. */
function seedBranch(opts: { worktrees: string[]; sessionIdsByWorktree: Record<string, string[]> }): void {
  const sessions = Object.fromEntries(
    Object.entries(opts.sessionIdsByWorktree).flatMap(([worktreeId, ids]) =>
      ids.map((id) => [id, aSession({ id, projectId: "p-1", branchId: "b-1", worktreeId, status: "running" })]),
    ),
  );
  useCockpit.setState({
    harnesses: [CLAUDE, SHELL],
    projects: [aProject({ id: "p-1", name: "engagement-portal", path: "C:\\work\\engagement-portal" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload" })] },
    worktreesByBranch: {
      "b-1": opts.worktrees.map((id) => aWorktree({ id, projectId: "p-1", branchId: "b-1", name: id })),
    },
    sessions,
    sessionIdsByWorktree: opts.sessionIdsByWorktree,
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
    layout: "columns",
  });
}

describe("ColumnsLayout", () => {
  beforeEach(() => {
    resetCockpit();
    createSessionMock.mockReset();
    // setup.ts defines scrollIntoView on Element.prototype (jsdom implements none); spy on
    // the same object it was defined on, or the spy is installed on a prototype that only
    // inherits the property and never sees the call.
    vi.spyOn(Element.prototype, "scrollIntoView").mockImplementation(() => {});
  });

  it("renders one pane per id, in the order given", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1", "s-2"] } });
    const { container } = render(<ColumnsLayout sessionIds={["s-1", "s-2"]} />);

    const panes = Array.from(container.querySelectorAll("[data-session-id]"));
    expect(panes.map((p) => p.getAttribute("data-session-id"))).toEqual(["s-1", "s-2"]);
  });

  it("the first pass seeds its id set and steals no focus", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1", "s-2"] } });
    render(<ColumnsLayout sessionIds={["s-1", "s-2"]} />);

    expect(useCockpit.getState().selection.sessionId).toBeNull();
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();
  });

  it("a newly created session is focused and scrolled into view", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1"] } });
    const view = render(<ColumnsLayout sessionIds={["s-1"]} />);

    // Server order is createdAt descending, so a new session arrives at the head. The
    // store write lands while the component is mounted, so it is act-wrapped too, or React
    // logs "An update to ColumnsLayout inside a test was not wrapped in act(...)" over an
    // otherwise green run.
    act(() => {
      seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-2", "s-1"] } });
    });
    view.rerender(<ColumnsLayout sessionIds={["s-2", "s-1"]} />);

    expect(useCockpit.getState().selection.sessionId).toBe("s-2");
    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({ inline: "center", block: "nearest" });
  });

  // Two sessions can arrive on one poll. Server order is createdAt descending, so the newest is
  // first and the newest is the one the user just asked for — focusing the last of the batch
  // would drop them on whichever of their new sessions is oldest.
  it("focuses the newest of several sessions that arrive together", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1"] } });
    const view = render(<ColumnsLayout sessionIds={["s-1"]} />);

    act(() => {
      seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-3", "s-2", "s-1"] } });
    });
    view.rerender(<ColumnsLayout sessionIds={["s-3", "s-2", "s-1"]} />);

    expect(useCockpit.getState().selection.sessionId).toBe("s-3");
  });

  // The scroll effect watches sessionIds as well as the focused id, and this is why: a focus
  // that lands before the pane exists finds nothing to scroll, and without the second
  // dependency it would never look again once the pane arrived.
  it("scrolls a focused pane that did not exist yet when the focus landed", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1"] } });
    const view = render(<ColumnsLayout sessionIds={["s-1"]} />);

    act(() => {
      useCockpit.getState().focusSession("s-2");
    });
    expect(Element.prototype.scrollIntoView).not.toHaveBeenCalled();

    // The focused id does NOT change across this step — only the id list does. Re-seeding the
    // branch here would reset selection.sessionId to null and the focus effect would set it
    // back, which moves focusedId and lets the [focusedId]-only dependency array pass. This is
    // the shape that isolates the second dependency; the first draft of this test did not.
    act(() => {
      useCockpit
        .getState()
        .upsertSession(aSession({ id: "s-2", projectId: "p-1", branchId: "b-1", worktreeId: "w-1" }));
    });
    view.rerender(<ColumnsLayout sessionIds={["s-2", "s-1"]} />);

    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({ inline: "center", block: "nearest" });
  });

  it("focusing an existing session scrolls its pane into view without creating anything", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1", "s-2"] } });
    render(<ColumnsLayout sessionIds={["s-1", "s-2"]} />);

    act(() => {
      useCockpit.getState().focusSession("s-2");
    });

    expect(Element.prototype.scrollIntoView).toHaveBeenCalledWith({ inline: "center", block: "nearest" });
    expect(createSessionMock).not.toHaveBeenCalled();
  });
});

describe("DevelopLens", () => {
  beforeEach(() => {
    resetCockpit();
    createSessionMock.mockReset();
    vi.spyOn(Element.prototype, "scrollIntoView").mockImplementation(() => {});
  });

  it("renders Columns for the selected branch's sessions", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1", "s-2"] } });
    const { container } = render(<DevelopLens />);

    expect(container.querySelectorAll("[data-session-id]")).toHaveLength(2);
  });

  it("renders the Canvas plane when the layout is canvas", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ["s-1"] } });
    useCockpit.setState({ layout: "canvas" });
    const { container } = render(<DevelopLens />);

    // Task 20 reserved this expectation for task 21 to replace. It asserted `.cols-scroll`
    // while Canvas still fell through to Columns; the plane is what it falls through to now.
    expect(container.querySelector('[data-testid="cv-plane"]')).not.toBeNull();
    expect(container.querySelector(".cols-scroll")).toBeNull();
  });

  it("renders the zero state when the branch has no sessions", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": [] } });
    const { container } = render(<DevelopLens />);

    expect(container.querySelector("[data-session-id]")).toBeNull();
    expect(screen.getByRole("button", { name: /Claude Code/ })).toBeDefined();
  });
});

describe("EmptyDevelop", () => {
  beforeEach(() => {
    resetCockpit();
    createSessionMock.mockReset();
    vi.spyOn(Element.prototype, "scrollIntoView").mockImplementation(() => {});
  });

  it("names the rail when no branch is selected, and offers no action at all", () => {
    render(<DevelopLens />);

    expect(screen.getByText(/Select a branch in the rail/i)).toBeDefined();
    expect(screen.queryAllByRole("button")).toHaveLength(0);
  });

  it("opens the branch switching flow when this branch has no checkout", async () => {
    seedBranch({ worktrees: [], sessionIdsByWorktree: {} });
    render(<DevelopLens />);

    expect(screen.getByText(/Open evidence-upload in your project/)).toBeDefined();
    expect(screen.queryByRole("button", { name: /Claude Code/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /shell/i })).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Switch to evidence-upload" }));
    expect(useCockpit.getState().branchManager).toEqual({ projectId: "p-1", action: "switch", branchId: "b-1" });
    expect(createSessionMock).not.toHaveBeenCalled();
  });

  it("offers the two named actions on a worktree with no sessions, and creates with resolved defaults", async () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": [] } });
    createSessionMock.mockResolvedValue(
      aSession({ id: "s-9", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", status: "starting" }),
    );
    render(<DevelopLens />);

    await userEvent.click(screen.getByRole("button", { name: /Claude Code/ }));

    // effort is absent: resolveDefaults leaves it "" and toCreateRequest omits an empty one.
    expect(createSessionMock).toHaveBeenCalledWith({
      worktreeId: "w-1",
      harness: "claude-code",
      model: "opus",
      permissionMode: "ask",
    });
  });

  // §6.9's rule, the same one ⌘⏎ uses: the SELECTED worktree, falling back to the branch's
  // first. With one worktree the fallback and the rule are indistinguishable, so this is the
  // case that tells them apart — and two surfaces resolving it differently is how a user ends
  // up with a session on a worktree they were not looking at.
  it("starts the session on the selected worktree, not on the branch's first", async () => {
    seedBranch({ worktrees: ["w-1", "w-2"], sessionIdsByWorktree: { "w-1": [], "w-2": [] } });
    useCockpit.setState((s) => ({ selection: { ...s.selection, worktreeId: "w-2" } }));
    createSessionMock.mockResolvedValue(
      aSession({ id: "s-11", projectId: "p-1", branchId: "b-1", worktreeId: "w-2" }),
    );
    render(<DevelopLens />);

    expect(screen.getByText(/No sessions on w-2\./)).toBeDefined();
    await userEvent.click(screen.getByRole("button", { name: /Claude Code/ }));

    expect(createSessionMock.mock.calls[0][0].worktreeId).toBe("w-2");
  });

  // §6.4: the gate is always the descriptor, never `harness === "claude-code"`, and that applies
  // to the WORDING as much as to the logic. A hard-coded label lies the first time a user
  // changes their default.
  it("labels the primary action from the resolved defaults, not from a constant", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": [] } });
    useCockpit.setState({
      defaults: { harness: "shell", model: "", effort: "", permissionMode: "ask" },
    });
    render(<DevelopLens />);

    expect(screen.getByRole("button", { name: "＋ Shell" })).toBeDefined();
    expect(screen.queryByRole("button", { name: /Claude Code/ })).toBeNull();
  });

  it("offers no shell action on a machine whose catalogue has no shell", () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": [] } });
    useCockpit.setState({ harnesses: [CLAUDE] });
    render(<DevelopLens />);

    expect(screen.getByRole("button", { name: /Claude Code/ })).toBeDefined();
    expect(screen.queryByRole("button", { name: /open a shell/i })).toBeNull();
  });

  it("the shell action sends no model, effort or permission mode", async () => {
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": [] } });
    createSessionMock.mockResolvedValue(
      aSession({ id: "s-10", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", harness: "shell" }),
    );
    render(<DevelopLens />);

    await userEvent.click(screen.getByRole("button", { name: /open a shell/i }));

    expect(createSessionMock).toHaveBeenCalledWith({ worktreeId: "w-1", harness: "shell" });
  });
});

/* MOVED HERE with the function, from SessionPane.test.tsx. `startSessionOnWorktree` lived in
 * SessionPane.tsx while that file's Restart was also a create; Restart is now
 * `api.restartSession`, so this surface's two buttons are its only callers.
 *
 * Its failure branch is the part a green happy path hides: every client method rejects with
 * ApiError and nothing else (§4), and two of the sixteen codes are different SENTENCES that
 * must not be collapsed. */
describe("startSessionOnWorktree", () => {
  const DEFAULTS = {
    harness: "claude-code",
    model: "opus",
    effort: "high",
    permissionMode: "ask",
  } as const;

  beforeEach(() => {
    resetCockpit();
    createSessionMock.mockReset();
    seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": [] } });
  });

  it("says the control plane is down, and does not say Docker is", async () => {
    createSessionMock.mockRejectedValue(new ApiError("network_unavailable", "fetch failed", 0));

    await startSessionOnWorktree("w-1", { ...DEFAULTS });

    expect(useCockpit.getState().toasts.map((t) => t.message)).toEqual([
      "The control plane is not running. Start it, then try again.",
    ]);
  });

  it("says Docker is down, and does not say the control plane is", async () => {
    createSessionMock.mockRejectedValue(
      new ApiError("backend_unavailable", "docker daemon unreachable", 503),
    );

    await startSessionOnWorktree("w-1", { ...DEFAULTS });

    expect(useCockpit.getState().toasts.map((t) => t.message)).toEqual([
      "Docker is not running, so a sandbox cannot be started.",
    ]);
  });

  it("carries the server's own sentence for every other code, the sixteenth included", async () => {
    createSessionMock.mockRejectedValue(
      new ApiError("non_loopback_publish", "The sandbox published a port off loopback.", 502),
    );

    await startSessionOnWorktree("w-1", { ...DEFAULTS });

    const message = useCockpit.getState().toasts[0].message;
    expect(message).toBe("The sandbox published a port off loopback.");
    // That sandbox is RUNNING. Collapsing this into "not running" would be false, which is
    // the whole reason Group C's ruling 7 made it its own code.
    expect(message).not.toContain("not running");
  });

  it("names a harness the catalogue does not have, and creates nothing", async () => {
    await startSessionOnWorktree("w-1", { ...DEFAULTS, harness: "codex" });

    expect(createSessionMock).not.toHaveBeenCalled();
    expect(useCockpit.getState().toasts[0].message).toBe(
      "The codex harness is not in the catalogue.",
    );
  });
});

describe("bounded column output", () => {
  beforeEach(() => { resetCockpit(); });
  it("keeps 50 sessions navigable while mounting at most six terminals, including keyboard focus", () => {
    let callback: IntersectionObserverCallback | undefined;
    const disconnect = vi.fn();
    vi.stubGlobal("IntersectionObserver", class {
      constructor(cb: IntersectionObserverCallback) { callback = cb; }
      observe() {} unobserve() {} disconnect = disconnect;
    });
    try {
      const ids = Array.from({ length: 50 }, (_, i) => `bounded-${i}`);
      seedBranch({ worktrees: ["w-1"], sessionIdsByWorktree: { "w-1": ids } });
      const view = render(<ColumnsLayout sessionIds={ids} />);
      expect(view.container.querySelectorAll("[data-column-for]")).toHaveLength(50);
      expect(screen.getAllByTestId("terminal")).toHaveLength(2);
      const panes = Array.from(view.container.querySelectorAll("[data-column-for]"));
      act(() => callback?.(panes.map((target, i) => ({ target, isIntersecting: i >= 20 && i < 30 })) as IntersectionObserverEntry[], {} as IntersectionObserver));
      expect(screen.getAllByTestId("terminal")).toHaveLength(6);
      expect(view.container.querySelector('[data-session="bounded-0"]')).toBeNull();
      act(() => useCockpit.getState().focusSession("bounded-49"));
      expect(view.container.querySelector('[data-session="bounded-49"]')).not.toBeNull();
      expect(screen.getAllByTestId("terminal")).toHaveLength(6);
      act(() => callback?.(panes.map(target => ({ target, isIntersecting: false })) as IntersectionObserverEntry[], {} as IntersectionObserver));
      expect(screen.getAllByTestId("terminal")).toHaveLength(1);
      view.unmount(); expect(disconnect).toHaveBeenCalledOnce();
    } finally { vi.unstubAllGlobals(); }
  });
});
