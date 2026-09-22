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
    const { rerender } = render(<Terminal sessionId="s-1" readOnly={false} />);
    rerender(<Terminal sessionId="s-1" readOnly={false} />);
    expect(FakeWebSocket.instances.length).toBe(1);
  });

  it("constructs exactly one terminal under StrictMode's double mount", () => {
    seed(session());
    render(
      <StrictMode>
        <Terminal sessionId="s-1" readOnly={false} />
      </StrictMode>,
    );
    expect(FakeWebSocket.instances.length).toBe(1);
    expect((grab().term as unknown as FakeTerm).disposed).toBe(false);
  });

  it("puts the registry's element in the pane, opens into it and takes DOM focus", () => {
    seed(session());
    const { container } = render(<Terminal sessionId="s-1" readOnly={false} />);
    const held = grab();
    const term = held.term as unknown as FakeTerm;
    expect(container.querySelector('[data-session-id="s-1"]')).toBe(held.element);
    expect(term.openedIn).toBe(held.element);
    expect(term.focused).toBeGreaterThan(0);
  });

  it("a store update does not dispose or re-create the terminal (defect 1)", () => {
    seed(session());
    render(<Terminal sessionId="s-1" readOnly={false} />);
    const term = grab().term as unknown as FakeTerm;
    useCockpit.setState({
      sessions: { "s-1": session({ status: "idle", updatedAt: "2026-09-18T05:00:00Z" }) },
    });
    expect(term.disposed).toBe(false);
    expect(FakeWebSocket.instances.length).toBe(1);
  });

  it("opens no socket for an exited session and reports why", () => {
    seed(session({ status: "exited", exitReason: "process exited with status 0" }));
    render(<Terminal sessionId="s-1" readOnly />);
    expect(FakeWebSocket.instances.length).toBe(0);
    expect(getAttachState("s-1").phase).toBe("ended");
    expect(getAttachState("s-1").reason).toBe("process exited with status 0");
  });

  it("releases on unmount, and the terminal survives the grace window", () => {
    seed(session());
    const { unmount } = render(<Terminal sessionId="s-1" readOnly={false} />);
    unmount();
    expect((grab().term as unknown as FakeTerm).disposed).toBe(false);
  });

  it("unmount detaches the element and tears the entry down once the grace window passes", () => {
    vi.useFakeTimers();
    try {
      seed(session());
      const { unmount } = render(<Terminal sessionId="s-1" readOnly={false} />);
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

  it("opens the terminal exactly once, and only after its element is in the document", () => {
    seed(session());
    render(
      <StrictMode>
        <Terminal sessionId="s-1" readOnly={false} />
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
    render(<Terminal sessionId="s-1" readOnly={false} />);
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
    render(<Terminal sessionId="s-1" readOnly={false} />);
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
    const { rerender } = render(<Terminal sessionId="s-1" readOnly={false} />);
    const term = grab().term as unknown as FakeTerm;
    const ws = FakeWebSocket.last()!;
    ws.acceptOpen();

    rerender(<Terminal sessionId="s-1" readOnly />);
    const before = ws.sent.length;
    term.type("rm -rf /\r");
    expect(ws.sent.length).toBe(before);
  });
});
