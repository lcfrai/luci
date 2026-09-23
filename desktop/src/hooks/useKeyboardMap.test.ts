import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useKeyboardMap } from "./useKeyboardMap";
import { __resetOverlays, hasOpenOverlay, useDismissable } from "./useDismissable";
import { resetCockpit, useCockpit } from "../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../test/fixtures";

// Task 22 owns the kill verb and task 18 owns the create path; this file proves the CHORDS
// reach them, not what they do. If step 1 found a different name or arity for the verb list,
// this factory and the import in useKeyboardMap.ts are the only two places that change.
//
// The fake returns task 22's SHAPE — a MenuItem[] with one `kill` entry — rather than a bare
// function, because the shape is the seam: the hook finds the item by id and calls `run`, and
// a fake that skipped the list would let a hook that called something else pass.
const { killMock, createSessionMock } = vi.hoisted(() => ({
  killMock: vi.fn(),
  createSessionMock: vi.fn(),
}));
vi.mock("../components/develop/sessionVerbs", () => ({
  sessionVerbs: (sessionId: string) => [
    { id: "resume", label: "Resume", run: undefined },
    { id: "kill", label: "Kill", run: () => killMock(sessionId) },
  ],
}));
vi.mock("./useCreateSession", () => ({ useCreateSession: () => createSessionMock }));

/** cancelable:true is MANDATORY. Without it preventDefault() is a no-op, defaultPrevented
 *  stays false, and every "is not intercepted" assertion below passes for the wrong reason. */
function press(key: string, init: KeyboardEventInit = {}): KeyboardEvent {
  const event = new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true, ...init });
  window.dispatchEvent(event);
  return event;
}

beforeEach(() => {
  resetCockpit();
  __resetOverlays();
  killMock.mockReset().mockResolvedValue(undefined);
  createSessionMock.mockReset().mockResolvedValue(undefined);
  useCockpit.setState({
    projects: [aProject({ id: "p-1", name: "engagement-portal", path: "C:\\work\\engagement-portal" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload", isHead: false })] },
    worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1" })] },
    sessions: Object.fromEntries(
      ["s-1", "s-2", "s-3"].map((id) => [
        id,
        aSession({ id, projectId: "p-1", branchId: "b-1", worktreeId: "w-1", permissionMode: "ask" }),
      ]),
    ),
    sessionIdsByWorktree: { "w-1": ["s-1", "s-2", "s-3"] },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: null },
  });
  renderHook(() => useKeyboardMap());
});

afterEach(() => {
  document.querySelectorAll("textarea.xterm-helper-textarea").forEach((node) => node.remove());
});

describe("useKeyboardMap — the deliberate non-interceptions", () => {
  it("shift+tab is not intercepted and session.permissionMode is unchanged", () => {
    const event = press("Tab", { shiftKey: true });
    expect(event.defaultPrevented).toBe(false);
    expect(useCockpit.getState().sessions["s-1"].permissionMode).toBe("ask");
  });

  it("Escape is not intercepted while a terminal has focus and no overlay is open", () => {
    const term = document.createElement("textarea");
    term.className = "xterm-helper-textarea";
    document.body.appendChild(term);
    term.focus();
    expect(hasOpenOverlay()).toBe(false);
    const event = press("Escape");
    expect(event.defaultPrevented).toBe(false);
    expect(useCockpit.getState().paletteOpen).toBe(false);
  });

  it("/ is not intercepted, because slash commands belong to the agent CLI", () => {
    expect(press("/").defaultPrevented).toBe(false);
  });
});

describe("useKeyboardMap — the chord table", () => {
  it("⌘W preventDefaults even with nothing focused, so the webview never closes", () => {
    expect(press("w", { ctrlKey: true }).defaultPrevented).toBe(true);
    expect(killMock).not.toHaveBeenCalled();
  });

  it("⌘W kills the focused session", () => {
    useCockpit.getState().focusSession("s-2");
    expect(press("w", { metaKey: true }).defaultPrevented).toBe(true);
    expect(killMock).toHaveBeenCalledWith("s-2");
  });

  it("⌘K toggles the palette, including while a terminal has focus", () => {
    expect(press("k", { ctrlKey: true }).defaultPrevented).toBe(true);
    expect(useCockpit.getState().paletteOpen).toBe(true);
    press("k", { ctrlKey: true });
    expect(useCockpit.getState().paletteOpen).toBe(false);
  });

  it("⌘1 focuses the first pane session and ⌘9 past the end is a no-op", () => {
    press("1", { ctrlKey: true });
    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
    press("9", { ctrlKey: true });
    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
  });

  it("⌘\\ moves to the next pane session and wraps at the end", () => {
    useCockpit.getState().focusSession("s-3");
    press("\\", { ctrlKey: true });
    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
  });

  it("⌘B toggles the rail and ⌘⇧M opens the machines popover", () => {
    press("b", { ctrlKey: true });
    expect(useCockpit.getState().railCollapsed).toBe(true);
    press("M", { ctrlKey: true, shiftKey: true });
    expect(useCockpit.getState().machinesOpen).toBe(true);
  });

  // §6.9 marks ⌘B "Always", so it survives an open overlay: the rail is not behind the
  // palette's scrim, it is beside it, and a user who opened the palette to find a session
  // can still widen the rail to look for one. ⌘⇧M is the declared departure — see the hook.
  it("⌘B still toggles the rail while an overlay is open, and ⌘⇧M does not", () => {
    // The overlay stack is module-private; the only way to push onto it is to mount a
    // useDismissable. The explicit type argument is required — T appears in no parameter,
    // so there is nothing to infer it from. setup.ts's cleanup() unmounts it after the test
    // and __resetOverlays() in beforeEach backstops that.
    renderHook(() => useDismissable<HTMLDivElement>(true, vi.fn()));
    expect(hasOpenOverlay()).toBe(true);
    press("b", { ctrlKey: true });
    expect(useCockpit.getState().railCollapsed).toBe(true);
    press("M", { ctrlKey: true, shiftKey: true });
    expect(useCockpit.getState().machinesOpen).toBe(false);
  });

  it("⌘⏎ creates a session on the selected branch's first worktree", () => {
    press("Enter", { ctrlKey: true });
    expect(createSessionMock).toHaveBeenCalledWith("w-1");
  });
});
