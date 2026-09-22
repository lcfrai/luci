import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  FLUSH_INTERVAL_MS,
  GRACE_MS,
  RETRY_BUDGET_MS,
  __resetAttachRegistry,
  acquireAttach,
  configureAttachRegistry,
  getAttachState,
  releaseAttach,
  setAttachReadOnly,
  subscribeAttach,
  updateAttachSession,
} from "./attachRegistry";
import type { TermLike } from "./attachRegistry";
import { ApiError } from "./client";
import { FakeTerm, FakeWebSocket } from "../test/fakes";
import { aSession } from "../test/fixtures";
import { Kind, decode, encode } from "../lib/wire";
import type { Session } from "../store/types";

/** The two client methods the registry uses, and nothing else. */
function stubClient(getSession?: () => Promise<Session>) {
  return {
    openAttach: (session: Session) =>
      new FakeWebSocket(`ws://127.0.0.1:8730${session.attachPath}`) as unknown as WebSocket,
    getSession: getSession ?? (async () => { throw new Error("getSession was not stubbed"); }),
  };
}

let term: FakeTerm;
/** No cast: FakeTerm structurally satisfies TermLike. If tsc rejects this line, step 1's
 *  transcription of FakeTerm drifted — fix the fake, never widen the interface. */
const createTerm = (): TermLike => { term = new FakeTerm(); return term; };

/** The newest socket the stub client handed out. */
const socket = (): FakeWebSocket => FakeWebSocket.last()!;

/** The bytes of one frame the registry sent. Task 14's FakeWebSocket records whatever `send()`
 *  was handed, typed `string | ArrayBufferLike | ArrayBufferView`; the registry always hands it
 *  a Uint8Array, so anything else is a defect and this throws rather than coercing. Negative
 *  indexes count from the end. It lives here, not in the shared fake: shared shape §3.3 keeps
 *  `fakes.ts` task 14's, and this is a handful of lines only one suite needs. */
function sentFrame(ws: FakeWebSocket, index: number): Uint8Array {
  const raw = ws.sent[index < 0 ? ws.sent.length + index : index];
  if (!(raw instanceof Uint8Array)) throw new Error(`sent[${index}] is not a frame: ${String(raw)}`);
  return raw;
}

const live = (over: Partial<Session> = {}): Session =>
  aSession({ id: "s-1", status: "running", attachPath: "/v1/sessions/s-1/attach", ...over });

beforeEach(() => {
  vi.useFakeTimers();
  FakeWebSocket.reset();
  __resetAttachRegistry();
  configureAttachRegistry({ client: stubClient() });
});

afterEach(() => {
  __resetAttachRegistry();
  vi.useRealTimers();
});

describe("attachRegistry — lifecycle", () => {
  it("opens exactly one socket for two acquires of the same session", () => {
    const session = live();
    const a = acquireAttach(session, { createTerm, readOnly: false });
    const b = acquireAttach(session, { createTerm, readOnly: false });
    expect(FakeWebSocket.instances.length).toBe(1);
    expect(a.term).toBe(b.term);
    expect(a.element).toBe(b.element);
  });

  it("keeps the socket open while one holder remains", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    acquireAttach(live(), { createTerm, readOnly: false });
    releaseAttach("s-1");
    vi.advanceTimersByTime(GRACE_MS * 2);
    expect(socket().closedWith).toBeNull();
    expect(term.disposed).toBe(false);
  });

  it("tears down GRACE_MS after the last holder releases, and not one tick before", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    releaseAttach("s-1");
    vi.advanceTimersByTime(GRACE_MS - 1);
    expect(socket().closedWith).toBeNull();
    expect(term.disposed).toBe(false);
    vi.advanceTimersByTime(2);
    expect(socket().closedWith).not.toBeNull();
    expect(term.disposed).toBe(true);
    expect(getAttachState("s-1").phase).toBe("detached");
  });

  it("a re-acquire inside the grace window cancels the teardown (StrictMode's shape)", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    releaseAttach("s-1");
    vi.advanceTimersByTime(GRACE_MS - 10);
    acquireAttach(live(), { createTerm, readOnly: false });
    vi.advanceTimersByTime(GRACE_MS * 2);
    expect(FakeWebSocket.instances.length).toBe(1);
    expect(term.disposed).toBe(false);
  });

  it("a store update does not dispose the terminal or re-open the socket (defect 1)", () => {
    const session = live();
    acquireAttach(session, { createTerm, readOnly: false });
    socket().acceptOpen();
    updateAttachSession({ ...session, status: "idle", updatedAt: "2026-09-18T04:05:06Z" });
    expect(term.disposed).toBe(false);
    expect(FakeWebSocket.instances.length).toBe(1);
    expect(getAttachState("s-1").phase).toBe("attached");
  });
});

describe("attachRegistry — input", () => {
  it("sends the terminal's size as soon as the socket opens", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    const frame = decode(sentFrame(socket(), 0));
    expect(frame.kind).toBe(Kind.Resize);
    const view = new DataView(frame.payload.buffer, frame.payload.byteOffset);
    expect(view.getUint16(0)).toBe(80);
    expect(view.getUint16(2)).toBe(24);
  });

  it("forwards a keystroke while the session is live", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    term.type("ls\r");
    const frame = decode(sentFrame(socket(), -1));
    expect(frame.kind).toBe(Kind.Stdin);
    expect(new TextDecoder().decode(frame.payload)).toBe("ls\r");
  });

  it("forwards a resize when the fit addon resizes the terminal", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    term.resize(120, 40);
    const frame = decode(sentFrame(socket(), -1));
    expect(frame.kind).toBe(Kind.Resize);
    const view = new DataView(frame.payload.buffer, frame.payload.byteOffset);
    expect(view.getUint16(0)).toBe(120);
  });

  it("drops a keystroke while readOnly, and leaves the socket alone", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    const before = socket().sent.length;
    setAttachReadOnly("s-1", true);
    term.type("rm -rf /\r");
    expect(socket().sent.length).toBe(before);
    expect(socket().closedWith).toBeNull();
  });

  it("does not open a socket for an exited session, and writes its reason verbatim", () => {
    acquireAttach(
      live({ status: "exited", exitReason: "process exited with status 130" }),
      { createTerm, readOnly: true },
    );
    expect(FakeWebSocket.instances.length).toBe(0);
    const state = getAttachState("s-1");
    expect(state.phase).toBe("ended");
    expect(state.reason).toBe("process exited with status 130");
    expect(term.written.join("")).toContain("process exited with status 130");
  });

  it("gives an exited session with no reason a sentence anyway", () => {
    acquireAttach(live({ status: "exited", exitReason: "" }), { createTerm, readOnly: true });
    expect(getAttachState("s-1").reason).toBe("the session has exited");
    expect(term.written.join("")).toContain("the session has exited");
  });

  it("hands out one frozen detached state for a session it does not hold", () => {
    // A snapshot getter that allocates re-renders useSyncExternalStore for ever — decision (d).
    const first = getAttachState("s-unknown");
    expect(getAttachState("s-unknown")).toBe(first);
    expect(Object.isFrozen(first)).toBe(true);
  });

  it("drops a keystroke and a resize while the handshake is still in flight", () => {
    acquireAttach(live(), { createTerm, readOnly: false });   // CONNECTING; never opened
    term.type("ls\r");
    term.resize(120, 40);
    expect(socket().sent).toEqual([]);   // FakeWebSocket.send() throws unless OPEN
  });

  it("drops a keystroke and a resize while the socket is down between attempts", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitClose(1006);            // entry.ws is null and a retry is armed
    const before = socket().sent.length;
    term.type("ls\r");
    term.resize(120, 40);
    expect(socket().sent.length).toBe(before);
  });

  it("leaves the pane attached while the socket is open, even after the poll says exited", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    updateAttachSession(live({ status: "exited", exitReason: "killed" }));
    // Output can still be arriving down that socket; "ended" is the close's word, not the poll's.
    expect(getAttachState("s-1").phase).toBe("attached");
  });

  it("an exited session polled twice publishes once", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitClose(1006);
    const seen: string[] = [];
    const off = subscribeAttach("s-1", (state) => seen.push(state.phase));
    updateAttachSession(live({ status: "exited", exitReason: "killed" }));
    updateAttachSession(live({ status: "exited", exitReason: "killed" }));
    off();
    expect(seen).toEqual(["ended"]);   // a re-publish every 2 s is a re-render every 2 s
  });
});

describe("attachRegistry — 10 Hz batching and the pinned rule (§7.2)", () => {
  const stdout = (s: string) => encode(Kind.Stdout, new TextEncoder().encode(s));

  it("three payloads inside one window produce exactly one write", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(stdout("one "));
    socket().emitBinary(stdout("two "));
    socket().emitBinary(stdout("three"));
    expect(term.written).toEqual([]);   // nothing yet: the flush timer has not fired
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.written).toEqual(["one two three"]);
  });

  it("arms no timer while nothing is buffered", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS * 50);
    expect(term.written).toEqual([]);
  });

  it("keeps a multibyte character split across two frames intact", () => {
    const arrow = new TextEncoder().encode("→");   // e2 86 92
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(encode(Kind.Stdout, arrow.subarray(0, 2)));
    socket().emitBinary(encode(Kind.Stdout, arrow.subarray(2)));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.written).toEqual(["→"]);
  });

  it("output does scroll a pane pinned to the bottom", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(stdout("line\n"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.scrolledToBottom).toBe(1);
    expect(term.buffer.active.viewportY).toBe(term.buffer.active.baseY);
  });

  it("output does not scroll a pane the user has scrolled up", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(stdout("a\nb\nc\nd\ne\n"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.scrolledToBottom).toBe(1);

    term.scrollUp(3);
    const parked = term.buffer.active.viewportY;
    socket().emitBinary(stdout("f\ng\n"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);

    expect(term.scrolledToBottom).toBe(1);   // still only the first flush
    expect(term.buffer.active.viewportY).toBe(parked);
  });

  it("writes the end banner after the output that preceded it", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(stdout("done."));
    socket().emitBinary(encode(Kind.Exit, new Uint8Array(0)));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.written.length).toBe(1);
    expect(term.written[0]).toBe("done.\r\n\x1b[2m[session ended]\x1b[0m\r\n");
  });

  it("arms exactly one flush timer however many payloads land in the window", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    expect(vi.getTimerCount()).toBe(0);
    socket().emitBinary(stdout("one "));
    socket().emitBinary(stdout("two "));
    socket().emitBinary(stdout("three"));
    // One, not three: 4 000 lines/second must not be 4 000 timers/second.
    expect(vi.getTimerCount()).toBe(1);
  });

  it("an empty stdout payload writes nothing and arms nothing", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(encode(Kind.Stdout, new Uint8Array(0)));
    expect(vi.getTimerCount()).toBe(0);
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.written).toEqual([]);
  });

  it("ignores a text frame: the attach protocol is binary-only", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    expect(() => socket().emitText("not a frame")).not.toThrow();
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.written).toEqual([]);
  });

  it("ignores a frame from a socket the ladder has already superseded", () => {
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    const stale = FakeWebSocket.instances[0];
    stale.emitClose(1006);
    vi.advanceTimersByTime(250);
    socket().acceptOpen();
    stale.emitBinary(stdout("ghost"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.written).toEqual([]);
  });
});

describe("attachRegistry — the retry ladder (§4.6)", () => {
  /** Drive the ladder past its 60 s budget. Async, so the give-up fetch settles too. */
  async function exhaustLadder() {
    for (let i = 0; i < 40; i += 1) {
      socket().emitClose(1006);
      await vi.advanceTimersByTimeAsync(2_000);
    }
  }

  it("retries a refused handshake on 250, 500, 1000, 2000, then 2000", () => {
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    expect(FakeWebSocket.instances.length).toBe(1);

    socket().emitClose(1006);
    expect(getAttachState("s-1").phase).toBe("reconnecting");
    vi.advanceTimersByTime(249);
    expect(FakeWebSocket.instances.length).toBe(1);
    vi.advanceTimersByTime(1);
    expect(FakeWebSocket.instances.length).toBe(2);

    socket().emitClose(1006);
    vi.advanceTimersByTime(500);
    expect(FakeWebSocket.instances.length).toBe(3);

    socket().emitClose(1006);
    vi.advanceTimersByTime(1_000);
    expect(FakeWebSocket.instances.length).toBe(4);

    socket().emitClose(1006);
    vi.advanceTimersByTime(2_000);
    expect(FakeWebSocket.instances.length).toBe(5);

    socket().emitClose(1006);
    vi.advanceTimersByTime(2_000);
    expect(FakeWebSocket.instances.length).toBe(6);
  });

  it("reconnects a socket that opened and then closed while the session is running", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitClose(1006);
    // Both halves matter, and both are about the ladder RESTARTING rather than resuming:
    // a successful handshake set attempts back to 0, so the rung must floor at 250 ms and not
    // index RETRY_DELAYS_MS[-1], and the phase the footer reads must be reconnecting — the
    // user is not "connecting" to a session they were just attached to.
    expect(getAttachState("s-1").phase).toBe("reconnecting");
    vi.advanceTimersByTime(249);
    expect(FakeWebSocket.instances.length).toBe(1);
    vi.advanceTimersByTime(1);
    expect(FakeWebSocket.instances.length).toBe(2);
    expect(getAttachState("s-1").phase).toBe("reconnecting");
  });

  it("reconnects an idle session — a live sandbox that has simply been quiet", () => {
    acquireAttach(live({ status: "idle" }), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitClose(1006);
    vi.advanceTimersByTime(250);
    expect(FakeWebSocket.instances.length).toBe(2);
  });

  it("does not retry a session that reads exited, and reports its reason", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    updateAttachSession(live({ status: "exited", exitReason: "killed" }));
    socket().emitClose(1006);
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(FakeWebSocket.instances.length).toBe(1);
    expect(getAttachState("s-1").phase).toBe("ended");
    expect(getAttachState("s-1").reason).toBe("killed");
  });

  it("gives up after the budget and calls a RUNNING sandbox running, never 'not running'", async () => {
    configureAttachRegistry({ client: stubClient(async () => live({ status: "running" })) });
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    await exhaustLadder();

    const state = getAttachState("s-1");
    expect(state.phase).toBe("failed");
    expect(state.reason).toContain("running");
    expect(state.reason).not.toContain("not running");
  });

  it("gives up with the exit reason when the session turns out to have exited", async () => {
    const dead = live({
      status: "exited",
      exitReason: "sandbox disappeared while the app was not running",
    });
    configureAttachRegistry({ client: stubClient(async () => dead) });
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    await exhaustLadder();

    expect(getAttachState("s-1").phase).toBe("ended");
    expect(getAttachState("s-1").reason).toBe(
      "sandbox disappeared while the app was not running",
    );
  });

  it("gives up with a plain sentence when the control plane cannot be reached", async () => {
    configureAttachRegistry({
      client: stubClient(async () => {
        throw new ApiError("network_unavailable", "fetch failed", 0);
      }),
    });
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    await exhaustLadder();

    expect(getAttachState("s-1").phase).toBe("failed");
    expect(getAttachState("s-1").reason).toBe("the control plane is not running");
  });

  it("never tells a non_loopback_publish refusal as 'not running' (the sixteenth code)", async () => {
    // Group C's ruling 7. The sandbox IS running; it published its port off loopback. This is
    // the one sentence the failure-reason rendering must never produce for this code.
    configureAttachRegistry({
      client: stubClient(async () => {
        throw new ApiError("non_loopback_publish", "published off loopback", 500);
      }),
    });
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    await exhaustLadder();

    const reason = getAttachState("s-1").reason;
    expect(reason).toContain("running");
    expect(reason).toContain("loopback");
    expect(reason).not.toContain("not running");
  });

  it("keeps 'the sandbox is not running' for the code that actually means it", async () => {
    configureAttachRegistry({
      client: stubClient(async () => {
        throw new ApiError("sandbox_not_running", "no sandbox", 409);
      }),
    });
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    await exhaustLadder();

    expect(getAttachState("s-1").reason).toBe("the sandbox is not running");
  });

  it("the second handshake of the first ladder reads reconnecting, not connecting", () => {
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    expect(getAttachState("s-1").phase).toBe("connecting");   // the first, and only the first
    socket().emitClose(1006);
    vi.advanceTimersByTime(250);
    expect(FakeWebSocket.instances.length).toBe(2);
    expect(getAttachState("s-1").phase).toBe("reconnecting");
  });

  it("ignores an open from a socket the ladder has already superseded", () => {
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    const stale = FakeWebSocket.instances[0];
    stale.emitClose(1006);
    vi.advanceTimersByTime(250);
    expect(FakeWebSocket.instances.length).toBe(2);

    stale.acceptOpen();
    expect(getAttachState("s-1").phase).toBe("reconnecting");
    expect(stale.sent.length).toBe(0);   // no resize down a socket we have moved off
  });

  it("a close from a superseded socket does not start a second ladder", () => {
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    const stale = FakeWebSocket.instances[0];
    stale.emitClose(1006);
    vi.advanceTimersByTime(250);
    expect(FakeWebSocket.instances.length).toBe(2);

    stale.emitClose(1006);               // a browser can deliver close more than once
    vi.advanceTimersByTime(2_000);
    expect(FakeWebSocket.instances.length).toBe(2);
  });

  it("caps the rung at 2 s rather than running off the end of the ladder", () => {
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    for (const delay of [250, 500, 1_000, 2_000]) {
      socket().emitClose(1006);
      vi.advanceTimersByTime(delay);
    }
    expect(FakeWebSocket.instances.length).toBe(5);

    socket().emitClose(1006);            // attempts is now past the end of RETRY_DELAYS_MS
    vi.advanceTimersByTime(1_999);
    expect(FakeWebSocket.instances.length).toBe(5);
    vi.advanceTimersByTime(1);
    expect(FakeWebSocket.instances.length).toBe(6);
  });

  it("a reconnect after a long attach restarts the ladder from the bottom rung", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    vi.advanceTimersByTime(61_000);      // attached for longer than the whole retry budget

    socket().emitClose(1006);
    vi.advanceTimersByTime(250);
    // A fresh budget: the clock onopen cleared must not be the one this drop is measured against.
    expect(FakeWebSocket.instances.length).toBe(2);

    socket().emitClose(1006);
    vi.advanceTimersByTime(250);
    // Still the bottom rung, because onopen reset the attempt count as well as the clock.
    expect(FakeWebSocket.instances.length).toBe(3);
  });

  it("gives up the moment the budget is spent, not a rung later", async () => {
    configureAttachRegistry({ client: stubClient(async () => live({ status: "running" })) });
    acquireAttach(live({ status: "starting" }), { createTerm, readOnly: false });
    for (let i = 0; i < 30; i += 1) {
      socket().emitClose(1006);
      await vi.advanceTimersByTimeAsync(2_000);
    }
    expect(getAttachState("s-1").phase).toBe("reconnecting");   // 58 s of ladder, still trying

    socket().emitClose(1006);            // exactly RETRY_BUDGET_MS since the ladder started
    expect(getAttachState("s-1").phase).toBe("failed");
    await vi.advanceTimersByTimeAsync(0);
  });

  it("an exited poll cancels a pending reconnect", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitClose(1006);
    expect(getAttachState("s-1").phase).toBe("reconnecting");

    updateAttachSession(live({ status: "exited", exitReason: "killed" }));
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(FakeWebSocket.instances.length).toBe(1);   // the armed retry never fired
    expect(getAttachState("s-1").phase).toBe("ended");
    expect(getAttachState("s-1").reason).toBe("killed");
  });

  it("a poll that still reads running does not end a pending reconnect", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitClose(1006);

    updateAttachSession(live({ status: "running" }));
    expect(getAttachState("s-1").phase).toBe("reconnecting");
    vi.advanceTimersByTime(250);
    expect(FakeWebSocket.instances.length).toBe(2);
  });
});
