// ⌘W against task 22's REAL verb list — the one seam useKeyboardMap.test.ts cannot see.
//
// That file mocks sessionVerbs on purpose: it proves the chords, and a real Kill would drag the
// Undo window into every case. But the hook finds Kill by the id "kill", and with the list mocked
// the only tests that know the id are task 22's own. Rename it there, update those tests to match,
// and the whole suite is green while ⌘W does nothing. This file is what goes red instead.
//
// Only `../api` is mocked, and only `killSession` on it: nothing else here reaches the network.
// No jest-dom (shared-shape §3C); nothing renders.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useKeyboardMap } from "./useKeyboardMap";
import { __resetOverlays } from "./useDismissable";
import { __resetSessionVerbs } from "../components/develop/sessionVerbs";
import { resetCockpit, useCockpit } from "../store/store";
import { aBranch, aProject, aSession, aWorktree } from "../test/fixtures";

const { killSessionMock } = vi.hoisted(() => ({ killSessionMock: vi.fn() }));
vi.mock("../api", () => ({ api: { killSession: killSessionMock } }));

beforeEach(() => {
  // The deferred commit arms a real timer for the Undo window; fake timers keep it from firing
  // into a later file, and __resetSessionVerbs clears it between cases.
  vi.useFakeTimers();
  resetCockpit();
  __resetOverlays();
  __resetSessionVerbs();
  useCockpit.setState({
    projects: [aProject({ id: "p-1", name: "engagement-portal", path: "C:\\work\\engagement-portal" })],
    branchesByProject: { "p-1": [aBranch({ id: "b-1", projectId: "p-1", name: "evidence-upload", isHead: false })] },
    worktreesByBranch: { "b-1": [aWorktree({ id: "w-1", projectId: "p-1", branchId: "b-1" })] },
    sessions: {
      "s-1": aSession({ id: "s-1", title: "orbit-one", projectId: "p-1", branchId: "b-1", worktreeId: "w-1", status: "running" }),
    },
    sessionIdsByWorktree: { "w-1": ["s-1"] },
    selection: { projectId: "p-1", branchId: "b-1", worktreeId: null, sessionId: "s-1" },
  });
  renderHook(() => useKeyboardMap());
});

afterEach(() => {
  __resetSessionVerbs();
  vi.useRealTimers();
});

describe("useKeyboardMap — ⌘W through task 22's own Kill", () => {
  it("marks the focused session exited at once, offers Undo, and sends nothing yet", () => {
    const event = new KeyboardEvent("keydown", { key: "w", ctrlKey: true, bubbles: true, cancelable: true });
    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(true);
    expect(useCockpit.getState().sessions["s-1"].status).toBe("exited");
    const { toasts } = useCockpit.getState();
    expect(toasts[toasts.length - 1]?.undo).toBeTypeOf("function");
    // The request waits out the Undo window; that timing is task 22's and proven there.
    expect(killSessionMock).not.toHaveBeenCalled();
  });
});
