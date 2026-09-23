import { StrictMode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { FakeTerm, FakeWebSocket } from "../test/fakes";
import { aSession } from "../test/fixtures";
import {
  GRACE_MS,
  __resetAttachRegistry,
  acquireAttach,
  configureAttachRegistry,
  getAttachState,
  releaseAttach,
  useAttachState,
} from "../api/attachRegistry";
import type { AttachHandle } from "../api/attachRegistry";
import { resetCockpit, useCockpit } from "../store/store";
import type { Session } from "../store/types";
import { Terminal } from "./Terminal";

// The factory takes a DYNAMIC import and does not close over the top-level `FakeTerm` binding
// above. vi.mock is hoisted above this file's imports, so a factory that reads that binding can
// throw "Cannot access '__vi_import_0__' before initialization" depending on which module the
// runner pulls first — task 14 documents this form in fakes.ts's header and it is the only one
// this file may use. The top-level import stays: the tests below still need FakeTerm as a type
// and for `instanceof`-style narrowing.
vi.mock("@xterm/xterm", async () => ({ Terminal: (await import("../test/fakes")).FakeTerm }));
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class { fit() {} activate() {} dispose() {} },
}));
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));

const session = (over: Partial<Session> = {}): Session =>
  aSession({ id: "s-1", status: "running", attachPath: "/v1/sessions/s-1/attach", ...over });

function seed(s: Session): void {
  useCockpit.setState({ sessions: { [s.id]: s } });
}

beforeEach(() => {
  FakeWebSocket.reset();
  __resetAttachRegistry();
  resetCockpit();
  configureAttachRegistry({
    client: {
      openAttach: (s: Session) => new FakeWebSocket(`ws://127.0.0.1:8730${s.attachPath}`) as unknown as WebSocket,
      getSession: async () => session(),
    },
  });
});

afterEach(() => {
  __resetAttachRegistry();
});

/** Reach the entry the component built without re-creating it. The factory throws if it is
 *  ever called, which is itself the assertion that one already exists. */
function grab(): AttachHandle {
  const handle = acquireAttach(session(), {
    createTerm: () => { throw new Error("the component should already have built a terminal"); },
    readOnly: false,
  });
  releaseAttach("s-1");   // put the ref count straight back
  return handle;
}

describe("Terminal", () => {
  it("two renders with the same sessionId construct exactly one Terminal", () => {
    seed(session());
    const { rerender } = render(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
    rerender(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
    expect(FakeWebSocket.instances.length).toBe(1);
  });

  it("constructs exactly one terminal under StrictMode's double mount", () => {
    seed(session());
    render(
      <StrictMode>
        <Terminal sessionId="s-1" readOnly={false} focused={false} />
      </StrictMode>,
    );
    expect(FakeWebSocket.instances.length).toBe(1);
    expect((grab().term as unknown as FakeTerm).disposed).toBe(false);
  });

  it("puts the registry's element in the pane, opens into it and takes DOM focus when focused", () => {
    seed(session());
    const { container } = render(<Terminal sessionId="s-1" readOnly={false} focused />);
    const held = grab();
    const term = held.term as unknown as FakeTerm;
    expect(container.querySelector('[data-session-id="s-1"]')).toBe(held.element);
    expect(term.openedIn).toBe(held.element);
    expect(term.focused).toBeGreaterThan(0);
  });

  it("a store update does not dispose or re-create the terminal (defect 1)", () => {
    seed(session());
    render(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
    const term = grab().term as unknown as FakeTerm;
    useCockpit.setState({
      sessions: { "s-1": session({ status: "idle", updatedAt: "2026-09-18T05:00:00Z" }) },
    });
    expect(term.disposed).toBe(false);
    expect(FakeWebSocket.instances.length).toBe(1);
  });

  it("opens no socket for an exited session and reports why", () => {
    seed(session({ status: "exited", exitReason: "process exited with status 0" }));
    render(<Terminal sessionId="s-1" readOnly focused={false} />);
    expect(FakeWebSocket.instances.length).toBe(0);
    expect(getAttachState("s-1").phase).toBe("ended");
    expect(getAttachState("s-1").reason).toBe("process exited with status 0");
  });

  it("releases on unmount, and the terminal survives the grace window", () => {
    seed(session());
    const { unmount } = render(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
    unmount();
    expect((grab().term as unknown as FakeTerm).disposed).toBe(false);
  });

  it("unmount detaches the element and tears the entry down once the grace window passes", () => {
    vi.useFakeTimers();
    try {
      seed(session());
      const { unmount } = render(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
      const held = grab();
      const term = held.term as unknown as FakeTerm;

      unmount();
      expect(held.element.parentNode).toBeNull();   // the pane gave the element back
      expect(term.disposed).toBe(false);            // and destroyed nothing doing it

      vi.advanceTimersByTime(GRACE_MS + 1);
      expect(term.disposed).toBe(true);             // the release did reach the registry
    } finally {
      vi.useRealTimers();
    }
  });

  it("observes its own host for resize, and disconnects that observer on unmount", () => {
    // Added by task 21's fix round 1, with the source change it pins. Task 14's setup.ts
    // polyfill is a no-op class, so whether a refit actually reflows is a real-browser carry
    // either way and nothing here pretends otherwise. What IS pinnable is the wiring — and the
    // third assertion is why this is a spy rather than a grep: a grep can prove the text
    // `ro.disconnect()` exists, not that it sits on the cleanup path.
    const w = window as unknown as { ResizeObserver: unknown };
    const real = w.ResizeObserver;
    const observed: Element[] = [];
    let disconnects = 0;
    w.ResizeObserver = class {
      observe(el: Element) { observed.push(el); }
      unobserve() {}
      disconnect() { disconnects += 1; }
    };
    try {
      seed(session());
      const { container, unmount } = render(
        <Terminal sessionId="s-1" readOnly={false} focused={false} />,
      );
      const host = container.firstElementChild;

      expect(observed.length).toBe(1);
      // The host, NOT the registry's element: that one outlives this component and is
      // re-parented between layouts, so an observer on it would outlive its owner.
      expect(observed[0]).toBe(host);
      expect(disconnects).toBe(0);

      unmount();
      expect(disconnects).toBe(1);
    } finally {
      w.ResizeObserver = real;
    }
  });

  it("opens the terminal exactly once, and only after its element is in the document", () => {
    seed(session());
    render(
      <StrictMode>
        <Terminal sessionId="s-1" readOnly={false} focused={false} />
      </StrictMode>,
    );
    const term = grab().term as unknown as FakeTerm;
    expect(term.opens).toBe(1);
    // xterm measures its cell size from a RENDERED element; open() on a detached node gives a
    // zero-column terminal, which is why mount() runs after the append.
    expect(term.openedWhileConnected).toBe(true);
  });

  it("a store update does not re-run the mount effect", () => {
    seed(session());
    // `focused` so the count under test is a real one rather than 0 === 0.
    render(<Terminal sessionId="s-1" readOnly={false} focused />);
    const term = grab().term as unknown as FakeTerm;
    const focusedBefore = term.focused;
    // act, so the re-render AND its passive effects actually flush: without it neither the
    // real deps list nor a wrong one would run, and this test would pass for no reason.
    act(() => {
      useCockpit.setState({
        sessions: { "s-1": session({ status: "idle", updatedAt: "2026-09-18T05:00:00Z" }) },
      });
    });
    // A dependency on the session object would re-acquire, re-append and steal focus back
    // every 2 s — known defect 1 wearing a component's clothes.
    expect(term.focused).toBe(focusedBefore);
  });

  it("pushes the store's session into the registry, so a kill stops the reconnect", () => {
    seed(session());
    render(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
    FakeWebSocket.last()!.acceptOpen();
    act(() => {
      useCockpit.setState({
        sessions: { "s-1": session({ status: "exited", exitReason: "killed" }) },
      });
    });
    FakeWebSocket.last()!.emitClose(1006);
    expect(getAttachState("s-1").phase).toBe("ended");
    expect(getAttachState("s-1").reason).toBe("killed");
  });

  it("pushes readOnly into the registry, so an exited pane stops forwarding stdin", () => {
    seed(session());
    const { rerender } = render(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
    const term = grab().term as unknown as FakeTerm;
    const ws = FakeWebSocket.last()!;
    ws.acceptOpen();

    rerender(<Terminal sessionId="s-1" readOnly focused={false} />);
    const before = ws.sent.length;
    term.type("rm -rf /\r");
    expect(ws.sent.length).toBe(before);
  });
});

/** The smallest possible consumer of the hook: task 20's SessionPaneFooter, reduced to the one
 *  job §6.5 gives it. It reads the registry, never the store. */
function AttachProbe({ sessionId }: { sessionId: string }) {
  const state = useAttachState(sessionId);
  return (
    <span data-testid="phase" data-attempts={state.attempts}>
      {state.phase}
    </span>
  );
}

describe("useAttachState", () => {
  it("re-renders its subscriber as the phase moves, and when the entry is torn down", () => {
    vi.useFakeTimers();
    try {
      acquireAttach(session(), { createTerm: () => new FakeTerm(), readOnly: false });
      const { getByTestId } = render(<AttachProbe sessionId="s-1" />);
      expect(getByTestId("phase").textContent).toBe("connecting");

      act(() => {
        FakeWebSocket.last()!.acceptOpen();
      });
      expect(getByTestId("phase").textContent).toBe("attached");

      releaseAttach("s-1");
      act(() => {
        vi.advanceTimersByTime(GRACE_MS + 1);
      });
      // destroy() must TELL its subscribers, not merely delete the entry. A getAttachState
      // assertion passes on `entries.delete` plus the `?? DETACHED` fallback alone; a
      // subscriber never notified keeps rendering "attached" for a session whose socket is
      // closed and whose terminal is disposed.
      expect(getByTestId("phase").textContent).toBe("detached");
    } finally {
      vi.useRealTimers();
    }
  });

  it("hands the hook a stable snapshot, so useSyncExternalStore does not re-render for ever", () => {
    // Decision (d). If the snapshot getter allocated, React would either loop or throw
    // "The result of getSnapshot should be cached"; rendering an unheld session is the case
    // where the getter returns the shared frozen constant on every call.
    const { getByTestId } = render(<AttachProbe sessionId="s-never-held" />);
    expect(getByTestId("phase").textContent).toBe("detached");
    expect(getByTestId("phase").getAttribute("data-attempts")).toBe("0");
  });
});

/* ADDED BY TASK 20's FIX ROUND 1, alongside the `focused` prop in Terminal.tsx.
 *
 * The defect: `handle.focus()` ran unconditionally on mount. React flushes passive effects
 * children-before-parents in mount order, so Columns with nine panes left the NINTH terminal
 * holding DOM focus while the `pane--focused` ring follows `selection.sessionId`, which is
 * null on a cold open. No ring anywhere and keystrokes going to the last pane in server order.
 *
 * These live here rather than in the pane's suite because this is the file whose FakeTerm
 * counts `focus()` calls, and the behaviour under test is this component's. */
describe("Terminal focus is conditional", () => {
  it("takes no DOM focus when it is not the focused pane", () => {
    seed(session());
    render(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
    expect((grab().term as unknown as FakeTerm).focused).toBe(0);
  });

  it("takes DOM focus on the false→true transition, so a rail-row click moves the caret", () => {
    seed(session());
    const { rerender } = render(<Terminal sessionId="s-1" readOnly={false} focused={false} />);
    const term = grab().term as unknown as FakeTerm;
    expect(term.focused).toBe(0);

    rerender(<Terminal sessionId="s-1" readOnly={false} focused />);

    expect(term.focused).toBe(1);
  });

  it("does not re-focus while it stays focused, so a poll cannot yank the caret back", () => {
    seed(session());
    const { rerender } = render(<Terminal sessionId="s-1" readOnly={false} focused />);
    const term = grab().term as unknown as FakeTerm;
    expect(term.focused).toBe(1);

    rerender(<Terminal sessionId="s-1" readOnly={false} focused />);
    act(() => {
      useCockpit.setState({
        sessions: { "s-1": session({ status: "idle", updatedAt: "2026-09-18T05:00:00Z" }) },
      });
    });

    expect(term.focused).toBe(1);
  });

  // The whole point of the prop: mount order stops mattering, because an unfocused pane does
  // nothing regardless of when its effects run.
  it("leaves the caret alone when several panes mount and none is focused", () => {
    useCockpit.setState({
      sessions: {
        "s-1": session({ id: "s-1", attachPath: "/v1/sessions/s-1/attach" }),
        "s-2": session({ id: "s-2", attachPath: "/v1/sessions/s-2/attach" }),
      },
    });
    render(
      <>
        <Terminal sessionId="s-1" readOnly={false} focused={false} />
        <Terminal sessionId="s-2" readOnly={false} focused={false} />
      </>,
    );

    for (const id of ["s-1", "s-2"]) {
      const handle = acquireAttach(session({ id, attachPath: `/v1/sessions/${id}/attach` }), {
        createTerm: () => { throw new Error("the component should already have built a terminal"); },
        readOnly: false,
      });
      releaseAttach(id);
      expect((handle.term as unknown as FakeTerm).focused).toBe(0);
    }
  });
});

/* ADDED BY TASK 23's FIX ROUND 1, alongside the key handler in Terminal.tsx's makeTerm.
 *
 * The defect: xterm handles keydown on its focused textarea and cancels every key it maps to
 * data, so with a terminal focused ⌘3 went to the agent as ESC and ⌘⏎ as CR, and the keyboard
 * map never saw either. The handler is how xterm is told which keys are the cockpit's.
 *
 * Structure only. FakeTerm records the handler and has no key path of its own, so this pins
 * THAT makeTerm installs one and WHICH WAY it points: false tells xterm to skip the event
 * without cancelling it. What real xterm does with that false is useKeyboardMap.xterm.test.tsx's.
 * Both lists are written out rather than read from isCockpitChord, so a predicate that drifted
 * is caught here as well as at the map. */
type KeyCase = [string, KeyboardEventInit];

const SKIPPED_BY_XTERM: KeyCase[] = [
  ["⌘W", { key: "w", ctrlKey: true }],
  ["⌘K", { key: "k", ctrlKey: true }],
  ["⌘B", { key: "b", ctrlKey: true }],
  ["⌘⇧M", { key: "M", ctrlKey: true, shiftKey: true }],
  ["⌘⏎", { key: "Enter", ctrlKey: true }],
  ["⌘\\", { key: "\\", ctrlKey: true }],
  ...Array.from({ length: 9 }, (_, i): KeyCase => [`⌘${i + 1}`, { key: `${i + 1}`, ctrlKey: true }]),
];

/* ADDED BY TASK 23's FIX ROUND 2. §6.9's ⌘ is metaKey || ctrlKey, and every row above presses
 * Ctrl. So a handler narrowed to `!(event.ctrlKey && isCockpitChord(event))` passed the whole
 * suite, and through a real xterm Meta+Enter then went to the agent as CR and created no
 * session. The Meta half of the table is therefore pinned too, row for row, with Ctrl released
 * so that Meta is the only modifier that can decide it. */
const SKIPPED_BY_XTERM_ON_META: KeyCase[] = SKIPPED_BY_XTERM.map(([name, init]): KeyCase => [
  `${name} on the Meta key`,
  { ...init, ctrlKey: false, metaKey: true },
]);

const HANDLED_BY_XTERM: KeyCase[] = [
  ["Escape, the agent's interrupt", { key: "Escape" }],
  ["shift+Tab, the agent's permission cycle", { key: "Tab", shiftKey: true }],
  ["/, the agent's slash commands", { key: "/" }],
  ["a bare w", { key: "w" }],
  ["AltGr composing a backslash (Ctrl+Alt+\\)", { key: "\\", ctrlKey: true, altKey: true }],
  ["Ctrl+C", { key: "c", ctrlKey: true }],
];

describe("Terminal lets the cockpit's chords past xterm", () => {
  function installedHandler(): (event: KeyboardEvent) => boolean {
    seed(session());
    render(<Terminal sessionId="s-1" readOnly={false} focused />);
    const handler = (grab().term as unknown as FakeTerm).customKeyEventHandler;
    expect(handler).not.toBeNull();
    return handler as (event: KeyboardEvent) => boolean;
  }

  it.each([...SKIPPED_BY_XTERM, ...SKIPPED_BY_XTERM_ON_META])("xterm skips %s, so it bubbles to the keyboard map", (_name, init) => {
    expect(installedHandler()(new KeyboardEvent("keydown", init))).toBe(false);
  });

  it.each(HANDLED_BY_XTERM)("xterm still handles %s, so it reaches the agent", (_name, init) => {
    expect(installedHandler()(new KeyboardEvent("keydown", init))).toBe(true);
  });
});
