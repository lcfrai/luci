import { StrictMode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { isCockpitChord, useKeyboardMap } from "./useKeyboardMap";
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
//
// `shape.retryRow` reproduces the one way task 22's real list changes shape under a session:
// sessionVerbs.ts puts "retry-attach" ahead of the tail verbs while an attach has failed, so
// Kill's POSITION moves by one. Any fixed index fails one of the two shapes; the id does not.
const { killMock, retryMock, createSessionMock, shape } = vi.hoisted(() => ({
  killMock: vi.fn(),
  retryMock: vi.fn(),
  createSessionMock: vi.fn(),
  shape: { retryRow: false },
}));
vi.mock("../components/develop/sessionVerbs", () => ({
  sessionVerbs: (sessionId: string) => [
    ...(shape.retryRow ? [{ id: "retry-attach", label: "Retry attach", run: () => retryMock(sessionId) }] : []),
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

/** A stand-in for xterm's helper textarea, focused, so an event can pass THROUGH the element
 *  that has focus on its way to the window listener, as a real keystroke does. It has no key
 *  handler of its own; what real xterm does on that element is useKeyboardMap.xterm.test.tsx's. */
function focusedTerminal(): HTMLTextAreaElement {
  const term = document.createElement("textarea");
  term.className = "xterm-helper-textarea";
  document.body.appendChild(term);
  term.focus();
  return term;
}

function pressOn(target: EventTarget, key: string, init: KeyboardEventInit = {}): KeyboardEvent {
  const event = new KeyboardEvent("keydown", { key, bubbles: true, cancelable: true, ...init });
  target.dispatchEvent(event);
  return event;
}

let mounted: { unmount: () => void };

beforeEach(() => {
  resetCockpit();
  __resetOverlays();
  shape.retryRow = false;
  killMock.mockReset().mockResolvedValue(undefined);
  retryMock.mockReset();
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
  mounted = renderHook(() => useKeyboardMap());
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
    const term = focusedTerminal();
    expect(document.activeElement).toBe(term);
    expect(hasOpenOverlay()).toBe(false);
    const event = pressOn(term, "Escape");
    expect(event.defaultPrevented).toBe(false);
    expect(useCockpit.getState().paletteOpen).toBe(false);
    // The terminal's half: the map's predicate is what xterm is told to let past, so Escape
    // must be outside it or xterm would skip it and \x1b would never reach the agent.
    expect(isCockpitChord(event)).toBe(false);
  });

  it("/ is not intercepted, because slash commands belong to the agent CLI", () => {
    expect(press("/").defaultPrevented).toBe(false);
  });

  it("a bare letter, digit or Enter typed into the agent is never a chord", () => {
    // Every key in the table, unmodified. With a session focused, a map that answered any of
    // these would kill, refocus or create while the user is typing a prompt.
    useCockpit.getState().focusSession("s-2");
    for (const key of ["w", "k", "b", "M", "1", "\\", "Enter"]) {
      expect(press(key).defaultPrevented).toBe(false);
    }
    expect(killMock).not.toHaveBeenCalled();
    expect(createSessionMock).not.toHaveBeenCalled();
    expect(useCockpit.getState().selection.sessionId).toBe("s-2");
    expect(useCockpit.getState().paletteOpen).toBe(false);
    expect(useCockpit.getState().railCollapsed).toBe(false);
    expect(useCockpit.getState().machinesOpen).toBe(false);
  });

  it("AltGr composing a backslash is typing, not ⌘\\", () => {
    // German and Swiss layouts compose "\" with AltGr, which Windows reports as Ctrl+Alt.
    useCockpit.getState().focusSession("s-1");
    expect(press("\\", { ctrlKey: true, altKey: true }).defaultPrevented).toBe(false);
    expect(useCockpit.getState().selection.sessionId).toBe("s-1");
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

  it("⌘W still kills, and retries nothing, when a retry-attach row moves Kill down the list", () => {
    shape.retryRow = true;
    useCockpit.getState().focusSession("s-2");
    press("w", { ctrlKey: true });
    expect(killMock).toHaveBeenCalledWith("s-2");
    expect(retryMock).not.toHaveBeenCalled();
  });

  it("⌘W swallows the key and kills nothing while an overlay is open", () => {
    useCockpit.getState().focusSession("s-2");
    renderHook(() => useDismissable<HTMLDivElement>(true, vi.fn()));
    expect(hasOpenOverlay()).toBe(true);
    expect(press("w", { ctrlKey: true }).defaultPrevented).toBe(true);
    expect(killMock).not.toHaveBeenCalled();
  });

  it("mounted under StrictMode, one ⌘W is one kill", () => {
    // The effect mounts, unmounts and mounts again under StrictMode, which main.tsx uses. A
    // cleanup that did not remove the listener would leave two, and the kill would run twice
    // with the same id: the call count is the only thing that differs.
    mounted.unmount();
    renderHook(() => useKeyboardMap(), { wrapper: StrictMode });
    useCockpit.getState().focusSession("s-2");
    press("w", { ctrlKey: true });
    expect(killMock).toHaveBeenCalledTimes(1);
  });

  it("⌘K toggles the palette, including while a terminal has focus", () => {
    // Through the focused element, not straight at the window: dispatched at the window, this
    // name was true of a listener that no focused terminal ever let a chord reach.
    const term = focusedTerminal();
    expect(pressOn(term, "k", { ctrlKey: true }).defaultPrevented).toBe(true);
    expect(useCockpit.getState().paletteOpen).toBe(true);
    pressOn(term, "k", { ctrlKey: true });
    expect(useCockpit.getState().paletteOpen).toBe(false);
  });

  it("⌘K over an open overlay closes that overlay once and opens the palette", () => {
    // Once, not "until the stack is empty": closeTopOverlay() does not pop the stack — the
    // overlay's unmount does, after this handler returns — so a draining loop would call the
    // same overlay's close() again and again. This stub never unmounts, so a loop shows here.
    const close = vi.fn();
    renderHook(() => useDismissable<HTMLDivElement>(true, close));
    press("k", { ctrlKey: true });
    expect(close).toHaveBeenCalledTimes(1);
    expect(useCockpit.getState().paletteOpen).toBe(true);
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

  it("⌘⏎ creates nothing when no branch is selected", () => {
    useCockpit.setState({ selection: { projectId: null, branchId: null, worktreeId: null, sessionId: null } });
    press("Enter", { ctrlKey: true });
    expect(createSessionMock).not.toHaveBeenCalled();
  });

  it("⌘⇧⏎ is not in the table, so it creates nothing and reaches the terminal", () => {
    expect(press("Enter", { ctrlKey: true, shiftKey: true }).defaultPrevented).toBe(false);
    expect(createSessionMock).not.toHaveBeenCalled();
  });

  // The review's four (task-23-review.md, Importants 2 and 3), bodies as written.
  it("⌘⏎, ⌘1 and ⌘\\ act on nothing while an overlay is open", () => {
    useCockpit.getState().focusSession("s-2");
    renderHook(() => useDismissable<HTMLDivElement>(true, vi.fn()));
    expect(hasOpenOverlay()).toBe(true);
    press("Enter", { ctrlKey: true });
    press("1", { ctrlKey: true });
    press("\\", { ctrlKey: true });
    expect(createSessionMock).not.toHaveBeenCalled();
    expect(useCockpit.getState().selection.sessionId).toBe("s-2");
  });

  it("⌘⇧W is not in the table, so it kills nothing", () => {
    useCockpit.getState().focusSession("s-2");
    press("W", { ctrlKey: true, shiftKey: true });
    expect(killMock).not.toHaveBeenCalled();
  });

  it("⌘⇧K is not in the table, so the palette stays shut", () => {
    press("K", { ctrlKey: true, shiftKey: true });
    expect(useCockpit.getState().paletteOpen).toBe(false);
  });

  it("⌘⇧B is not in the table, so the rail stays put", () => {
    press("B", { ctrlKey: true, shiftKey: true });
    expect(useCockpit.getState().railCollapsed).toBe(false);
  });
});

/* The predicate is read twice: by the map, and by xterm through Terminal.tsx's key handler.
 * A key it claims is a key the agent never receives, so both directions are pinned here, where
 * the predicate is the thing that decides. The terminal's end is Terminal.test.tsx (the handler
 * is installed and inverts this) and useKeyboardMap.xterm.test.tsx (real xterm). */
const CHORDS: Array<[string, KeyboardEventInit]> = [
  ["⌘W", { key: "w", ctrlKey: true }],
  ["⌘W on the Meta key", { key: "w", metaKey: true }],
  ["⌘K", { key: "k", ctrlKey: true }],
  ["⌘B", { key: "b", ctrlKey: true }],
  ["⌘⇧M", { key: "M", ctrlKey: true, shiftKey: true }],
  ["⌘⏎", { key: "Enter", ctrlKey: true }],
  ["⌘\\", { key: "\\", ctrlKey: true }],
  ...Array.from({ length: 9 }, (_, i): [string, KeyboardEventInit] => [`⌘${i + 1}`, { key: `${i + 1}`, ctrlKey: true }]),
];
const NOT_CHORDS: Array<[string, KeyboardEventInit]> = [
  ["Escape", { key: "Escape" }],
  ["shift+Tab", { key: "Tab", shiftKey: true }],
  ["/", { key: "/" }],
  ["a bare w", { key: "w" }],
  ["AltGr composing a backslash (Ctrl+Alt+\\)", { key: "\\", ctrlKey: true, altKey: true }],
  ["⌘⇧W", { key: "W", ctrlKey: true, shiftKey: true }],
  ["⌘⇧K", { key: "K", ctrlKey: true, shiftKey: true }],
  ["⌘⇧B", { key: "B", ctrlKey: true, shiftKey: true }],
  ["an unshifted ⌘M", { key: "m", ctrlKey: true }],
  ["⌘0, past the table", { key: "0", ctrlKey: true }],
  ["Ctrl+C, the agent's own interrupt", { key: "c", ctrlKey: true }],
];

describe("isCockpitChord — the one list the map and the terminal both read", () => {
  it.each(CHORDS)("claims %s", (_name, init) => {
    expect(isCockpitChord(new KeyboardEvent("keydown", init))).toBe(true);
  });

  it.each(NOT_CHORDS)("leaves %s to the agent", (_name, init) => {
    expect(isCockpitChord(new KeyboardEvent("keydown", init))).toBe(false);
  });

  it("the map intercepts exactly what it claims, and nothing it does not", () => {
    // Agreement, not a second list. If the map's early return ever grows a copy of its own,
    // this is what fails the day the two drift. No overlay is open, so every chord branch
    // reaches its preventDefault.
    for (const [name, init] of [...CHORDS, ...NOT_CHORDS]) {
      const event = press(init.key ?? "", init);
      expect([name, event.defaultPrevented]).toEqual([name, isCockpitChord(event)]);
    }
  });
});
