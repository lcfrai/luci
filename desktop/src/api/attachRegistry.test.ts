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
  retryAttach,
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
      new FakeWebSocket(
        `ws://127.0.0.1:8730${session.attachPath}`,
      ) as unknown as WebSocket,
    getSession:
      getSession ??
      (async () => {
        throw new Error("getSession was not stubbed");
      }),
  };
}

let term: FakeTerm;
/** No cast: FakeTerm structurally satisfies TermLike. If tsc rejects this line, step 1's
 *  transcription of FakeTerm drifted — fix the fake, never widen the interface. */
const createTerm = (): TermLike => {
  term = new FakeTerm();
  return term;
};

/** A terminal that parses ASYNCHRONOUSLY, as real xterm does: `write` returns immediately and
 *  the data reaches the buffer — moving baseY — only when the deferred parse runs, which is
 *  also when the callback fires. FakeTerm does both synchronously inside `write`, which is
 *  right for every other test here and wrong for exactly one property: that the pinned scroll
 *  happens in the callback and not inline. It lives here, not in the shared fake, for the same
 *  reason `sentFrame` does — shared shape §3.3 keeps `fakes.ts` task 14's. */
class AsyncParseTerm extends FakeTerm {
  override write(data: string | Uint8Array, callback?: () => void) {
    const text =
      typeof data === "string" ? data : new TextDecoder().decode(data);
    this.written.push(text);
    setTimeout(() => {
      this.buffer.active.baseY += text.split("\n").length - 1;
      callback?.();
    }, 0);
  }
}

/** The newest socket the stub client handed out. */
const socket = (): FakeWebSocket => FakeWebSocket.last()!;

/** The bytes of one frame the registry sent. Task 14's FakeWebSocket records whatever `send()`
 *  was handed, typed `string | ArrayBufferLike | ArrayBufferView`; the registry always hands it
 *  a Uint8Array, so anything else is a defect and this throws rather than coercing. Negative
 *  indexes count from the end. It lives here, not in the shared fake: shared shape §3.3 keeps
 *  `fakes.ts` task 14's, and this is a handful of lines only one suite needs. */
function sentFrame(ws: FakeWebSocket, index: number): Uint8Array {
  const raw = ws.sent[index < 0 ? ws.sent.length + index : index];
  if (!(raw instanceof Uint8Array))
    throw new Error(`sent[${index}] is not a frame: ${String(raw)}`);
  return raw;
}

const live = (over: Partial<Session> = {}): Session =>
  aSession({
    id: "s-1",
    status: "running",
    attachPath: "/v1/sessions/s-1/attach",
    ...over,
  });

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
  it("closes a late authenticated socket after its pane was released and prevents duplicate pending tickets", async () => {
    let finish!: (socket: WebSocket) => void;
    const pending = new Promise<WebSocket>((resolve) => {
      finish = resolve;
    });
    const openAttach = vi.fn(() => pending);
    configureAttachRegistry({ client: { ...stubClient(), openAttach } });
    acquireAttach(live(), { createTerm, readOnly: false });
    expect(retryAttach("s-1")).toBe(false);
    expect(openAttach).toHaveBeenCalledTimes(1);
    releaseAttach("s-1");
    vi.advanceTimersByTime(GRACE_MS + 1);
    const late = new FakeWebSocket("ws://127.0.0.1/attach?ticket=one-use");
    finish(late as unknown as WebSocket);
    await Promise.resolve();
    expect(late.closedWith).not.toBeNull();
    expect(getAttachState("s-1").phase).toBe("detached");
  });
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

  it("a torn-down pane reports no running ladder", () => {
    // `ladderStartedAt !== 0` is exactly "this pane is retrying", and a closed pane is not.
    // This is the DETACHED singleton's literal, reached the way every closed pane reaches it —
    // and it is a literal, so nothing recomputes it if the invariant moves. A non-zero here
    // has task 20 render a ticking elapsed for a pane that is doing nothing at all.
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    releaseAttach("s-1");
    vi.advanceTimersByTime(GRACE_MS + 1);

    const state = getAttachState("s-1");
    expect(state.phase).toBe("detached");
    expect(state.ladderStartedAt).toBe(0);
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

  it("an extra release cannot drive the ref count negative and orphan a live pane", () => {
    acquireAttach(live(), { createTerm, readOnly: false }); // refs 1
    releaseAttach("s-1"); // refs 0
    releaseAttach("s-1"); // one release too many
    acquireAttach(live(), { createTerm, readOnly: false }); // pane A
    acquireAttach(live(), { createTerm, readOnly: false }); // pane B
    releaseAttach("s-1"); // pane A closes
    vi.advanceTimersByTime(GRACE_MS * 2);
    // Unclamped, the count would read 0 here and tear down the entry pane B is still showing.
    expect(term.disposed).toBe(false);
  });

  it("an extra release replaces the grace timer rather than stacking another", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    expect(vi.getTimerCount()).toBe(0);
    releaseAttach("s-1");
    expect(vi.getTimerCount()).toBe(1);
    releaseAttach("s-1");
    releaseAttach("s-1");
    // One pending teardown, not three. This is the file whose whole job is not leaking timers.
    expect(vi.getTimerCount()).toBe(1);
  });

  it("a re-acquire inside the grace window leaves no stray timer behind", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    releaseAttach("s-1");
    expect(vi.getTimerCount()).toBe(1);
    acquireAttach(live(), { createTerm, readOnly: false });
    expect(vi.getTimerCount()).toBe(0);
  });

  it("teardown detaches the registry's element even when no pane removed it", () => {
    // The component normally hands the element back in its cleanup. This is the path where
    // destroy() is the only thing detaching it — a pane torn down without cleanup running.
    const handle = acquireAttach(live(), { createTerm, readOnly: false });
    document.body.appendChild(handle.element);
    expect(handle.element.parentNode).toBe(document.body);
    releaseAttach("s-1");
    vi.advanceTimersByTime(GRACE_MS + 1);
    expect(handle.element.parentNode).toBeNull();
  });

  it("a store update does not dispose the terminal or re-open the socket (defect 1)", () => {
    const session = live();
    acquireAttach(session, { createTerm, readOnly: false });
    socket().acceptOpen();
    updateAttachSession({
      ...session,
      status: "idle",
      updatedAt: "2026-09-18T04:05:06Z",
    });
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

  it("an already-exited session reports no running ladder on first open", () => {
    // The other mainline path that publishes a literal 0: an exited session never starts a
    // ladder at all, so a non-zero here would claim a pane is retrying an attach it will never
    // attempt. Same invariant as the torn-down case, different literal, neither derived.
    acquireAttach(
      live({ status: "exited", exitReason: "process exited with status 0" }),
      { createTerm, readOnly: true },
    );
    const state = getAttachState("s-1");
    expect(state.phase).toBe("ended");
    expect(state.ladderStartedAt).toBe(0);
  });

  it("gives an exited session with no reason a sentence anyway", () => {
    acquireAttach(live({ status: "exited", exitReason: "" }), {
      createTerm,
      readOnly: true,
    });
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
    acquireAttach(live(), { createTerm, readOnly: false }); // CONNECTING; never opened
    term.type("ls\r");
    term.resize(120, 40);
    expect(socket().sent).toEqual([]); // FakeWebSocket.send() throws unless OPEN
  });

  it("drops a keystroke and a resize while the socket is down between attempts", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitClose(1006); // entry.ws is null and a retry is armed
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
    expect(seen).toEqual(["ended"]); // a re-publish every 2 s is a re-render every 2 s
  });
});

describe("attachRegistry — 10 Hz batching and the pinned rule (§7.2)", () => {
  const stdout = (s: string) =>
    encode(Kind.Stdout, new TextEncoder().encode(s));

  it("three payloads inside one window produce exactly one write", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(stdout("one "));
    socket().emitBinary(stdout("two "));
    socket().emitBinary(stdout("three"));
    expect(term.written).toEqual([]); // nothing yet: the flush timer has not fired
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
    const arrow = new TextEncoder().encode("→"); // e2 86 92
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

    expect(term.scrolledToBottom).toBe(1); // still only the first flush
    expect(term.buffer.active.viewportY).toBe(parked);
  });

  /** F5 fix round 1, mutant M1. The rule is `viewportY >= baseY`, and the test above parks the
   *  viewport THREE rows up, so a one-row tolerance (`>= baseY - 1`) is invisible to it and
   *  survives the suite. One row up is scrolled up: a user reading the line above the newest
   *  must not be yanked back to the bottom by the next write. That is §7.2's whole promise at
   *  its boundary, and the e2e check that would have caught it (`c4.pinned`) is red for an
   *  unrelated harness defect, so the boundary has to be pinned here. */
  it("output does not scroll a pane the user has scrolled up by exactly one row", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(stdout("a\nb\nc\nd\ne\n"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.scrolledToBottom).toBe(1);

    term.scrollUp(1);
    const parked = term.buffer.active.viewportY;
    expect(parked).toBe(term.buffer.active.baseY - 1); // exactly one row, not three
    socket().emitBinary(stdout("f\n"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);

    expect(term.scrolledToBottom).toBe(1); // still only the first flush
    expect(term.buffer.active.viewportY).toBe(parked);
  });

  /** F5 fix round 1, mutant M2. `flush` scrolls from the write CALLBACK, not inline after
   *  `write` returns, because real xterm parses on its own schedule: the data is not in the
   *  buffer when `write` returns, so an inline `scrollToBottom()` scrolls to the OLD bottom and
   *  the pane sits one chunk short. FakeTerm advances baseY and calls back synchronously, so it
   *  cannot tell the two apart — both land after baseY moved — and the inline mutant survives.
   *  AsyncParseTerm defers both to a timer, which is the ordering real xterm has. */
  it("scrolls a pinned pane from the write callback, not before the write has landed", () => {
    let slow!: AsyncParseTerm;
    const createSlowTerm = (): TermLike => {
      slow = new AsyncParseTerm();
      return slow;
    };
    acquireAttach(live(), { createTerm: createSlowTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(stdout("a\nb\nc\n"));

    vi.advanceTimersByTime(FLUSH_INTERVAL_MS); // the registry flushes: term.write(chunk, cb)
    expect(slow.written).toEqual(["a\nb\nc\n"]);
    expect(slow.buffer.active.baseY).toBe(0); // not parsed yet, exactly as xterm would be

    vi.advanceTimersByTime(1); // the parse lands: baseY moves, then cb fires
    expect(slow.buffer.active.baseY).toBe(3);
    expect(slow.scrolledToBottom).toBe(1);
    expect(slow.buffer.active.viewportY).toBe(slow.buffer.active.baseY);
  });

  /** F5 fix round 2, the mutant the round-1 review NAMED and the suite still did not kill:
   *  latch `pinned` to false on the first scroll-up ("don't fight the user") and every test
   *  above still passes, because not one of them ever scrolls back down. The rule is
   *  `viewportY >= baseY` measured per write, so the transition false -> true is a branch of
   *  its own, and it is the branch a real user reaches: scroll up to read, scroll back down,
   *  expect the pane to resume following. Without this test that carry has no pin at all —
   *  `c4.pinned` is red for an unrelated harness defect (F5). */
  it("output scrolls again once the user has scrolled back down", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(stdout("a\nb\nc\nd\ne\n"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.scrolledToBottom).toBe(1);

    term.scrollUp(3);
    socket().emitBinary(stdout("f\n"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.scrolledToBottom).toBe(1); // frozen, as the test above pins

    // The user scrolls back down by hand: xterm re-pins the viewport. `scrollUp(0)` and not
    // `scrollToBottom()`, because the user's own gesture must not touch the counter the
    // assertion below reads — that counter is the registry's writes, not the user's.
    term.scrollUp(0);
    expect(term.buffer.active.viewportY).toBe(term.buffer.active.baseY);

    socket().emitBinary(stdout("g\n"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.scrolledToBottom).toBe(2); // pinned again: this write follows
    expect(term.buffer.active.viewportY).toBe(term.buffer.active.baseY);
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

  /** The 10 Hz bound itself, in LITERAL milliseconds. Every other test in this block advances
   *  the clock BY `FLUSH_INTERVAL_MS`, so it passes at any value of that constant and at any
   *  shape of timer — which is why two mutants survived the suite (review round 3, Minor 2):
   *  the throttle turned into a debounce, and the constant moved 100 → 1000. This test holds
   *  the wall clock fixed instead: one second of continuous output must reach the screen as
   *  ten writes, spread through the second.
   *
   *  A DEBOUNCE renders nothing at all here — every chunk re-arms the timer and the burst never
   *  stops — which is exactly the failure docs/DESIGN-SYSTEM.md's "log panes throttle to ~10Hz"
   *  forbids: a build log that shows the user an empty pane until it finishes. */
  it("renders ten times in a second of unbroken output, and does not wait for it to stop", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    for (let elapsed = 0; elapsed < 1_000; elapsed += 10) {
      socket().emitBinary(stdout(`t${elapsed} `));
      vi.advanceTimersByTime(10);
    }
    expect(term.written.length).toBe(10);
    // …and batching is not dropping: every chunk of the second is on the screen, in order.
    expect(term.written.join("")).toBe(
      Array.from({ length: 100 }, (_, i) => `t${i * 10} `).join(""),
    );
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

  it("does not pad the next flush with the previous flush's byte count", () => {
    // Two CONSECUTIVE MULTI-CHUNK flushes is the shape nothing else exercises: concatChunks'
    // single-chunk fast path never reads `total`, so a stale pendingBytes is invisible until a
    // window carries more than one payload twice. Without the reset the second flush allocates
    // for 8 + 4 bytes and writes NUL padding into the transcript.
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(stdout("aaaa"));
    socket().emitBinary(stdout("bbbb"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    socket().emitBinary(stdout("cc"));
    socket().emitBinary(stdout("dd"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.written).toEqual(["aaaabbbb", "ccdd"]);
  });

  /** Review round 4, Minor 1, mutant MA: `flush()` re-arming unconditionally
   *  (`entry.flushTimer = entry.destroyed ? null : setTimeout(…)`) survived all 67 tests. Every
   *  timer assertion above is made either before the first flush ("arms no timer while nothing
   *  is buffered", ":454") or inside the window, so none of them can see a pane that keeps a
   *  100 ms timer for the rest of its life. That is the property `attachRegistry.ts:448-450`
   *  states in prose — "the timer is armed only when something is buffered, so an idle session
   *  costs nothing" — and the one `docs/DESIGN-SYSTEM.md` writes as ONE global heartbeat clock,
   *  never a timer per row. Asserted AFTER the flush, which is the only place it is visible. */
  it("holds no timer once the buffered output has gone out, however long the pane then idles", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(stdout("one line\n"));
    expect(vi.getTimerCount()).toBe(1);

    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.written).toEqual(["one line\n"]);
    expect(vi.getTimerCount()).toBe(0); // the flush disarmed, it did not re-arm

    vi.advanceTimersByTime(FLUSH_INTERVAL_MS * 100); // ten seconds of an idle pane
    expect(vi.getTimerCount()).toBe(0);
    expect(term.written).toEqual(["one line\n"]); // and it wrote nothing more
  });

  /** Review round 4, Minor 1, mutant MB: caching the buffer the pinned decision reads
   *  (`attachRegistry.ts:474`) survived all 67 tests, because every test above lives on ONE
   *  buffer. Real xterm swaps `buffer.active` for a different object on the alternate screen —
   *  `vim`, `htop`, `less`, an agent's own full-screen UI — and the alternate screen has no
   *  scrollback, so it is pinned by construction. A cached normal buffer that the user had
   *  parked mid-scroll then freezes a full-screen app's pane for as long as it runs. The
   *  decision has to be read from the buffer that is active AT THE FLUSH. */
  it("decides pinned from the buffer active at the flush, not the one active at the first", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitBinary(stdout("a\nb\nc\nd\ne\n"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.scrolledToBottom).toBe(1);

    term.scrollUp(3); // the user parks the NORMAL buffer mid-scrollback
    socket().emitBinary(stdout("f\n"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.scrolledToBottom).toBe(1);

    // The session enters the alternate screen: a different buffer object, its own viewport, no
    // scrollback to be parked in (viewportY === baseY === 0).
    term.buffer.active = { viewportY: 0, baseY: 0 };
    socket().emitBinary(stdout("full-screen frame\n"));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(term.scrolledToBottom).toBe(2); // pinned, read from the buffer now on screen
  });

  it("ignores a frame from a socket the ladder has already superseded", () => {
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
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
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
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
    configureAttachRegistry({
      client: stubClient(async () => live({ status: "running" })),
    });
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
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
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
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
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    await exhaustLadder();

    expect(getAttachState("s-1").phase).toBe("failed");
    expect(getAttachState("s-1").reason).toBe(
      "the control plane is not running",
    );
  });

  it("never tells a non_loopback_publish refusal as 'not running' (the sixteenth code)", async () => {
    // Group C's ruling 7. The sandbox IS running; it published its port off loopback. This is
    // the one sentence the failure-reason rendering must never produce for this code.
    configureAttachRegistry({
      client: stubClient(async () => {
        throw new ApiError(
          "non_loopback_publish",
          "published off loopback",
          500,
        );
      }),
    });
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
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
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    await exhaustLadder();

    expect(getAttachState("s-1").reason).toBe("the sandbox is not running");
  });

  it("the second handshake of the first ladder reads reconnecting, not connecting", () => {
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    expect(getAttachState("s-1").phase).toBe("connecting"); // the first, and only the first
    socket().emitClose(1006);
    vi.advanceTimersByTime(250);
    expect(FakeWebSocket.instances.length).toBe(2);
    expect(getAttachState("s-1").phase).toBe("reconnecting");
  });

  it("ignores an open from a socket the ladder has already superseded", () => {
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    const stale = FakeWebSocket.instances[0];
    stale.emitClose(1006);
    vi.advanceTimersByTime(250);
    expect(FakeWebSocket.instances.length).toBe(2);

    stale.acceptOpen();
    expect(getAttachState("s-1").phase).toBe("reconnecting");
    expect(stale.sent.length).toBe(0); // no resize down a socket we have moved off
  });

  it("a close from a superseded socket does not start a second ladder", () => {
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    const stale = FakeWebSocket.instances[0];
    stale.emitClose(1006);
    vi.advanceTimersByTime(250);
    expect(FakeWebSocket.instances.length).toBe(2);

    stale.emitClose(1006); // a browser can deliver close more than once
    vi.advanceTimersByTime(2_000);
    expect(FakeWebSocket.instances.length).toBe(2);
  });

  it("caps the rung at 2 s rather than running off the end of the ladder", () => {
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    for (const delay of [250, 500, 1_000, 2_000]) {
      socket().emitClose(1006);
      vi.advanceTimersByTime(delay);
    }
    expect(FakeWebSocket.instances.length).toBe(5);

    socket().emitClose(1006); // attempts is now past the end of RETRY_DELAYS_MS
    vi.advanceTimersByTime(1_999);
    expect(FakeWebSocket.instances.length).toBe(5);
    vi.advanceTimersByTime(1);
    expect(FakeWebSocket.instances.length).toBe(6);
  });

  it("a reconnect after a long attach restarts the ladder from the bottom rung", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    vi.advanceTimersByTime(61_000); // attached for longer than the whole retry budget

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
    configureAttachRegistry({
      client: stubClient(async () => live({ status: "running" })),
    });
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    for (let i = 0; i < 30; i += 1) {
      socket().emitClose(1006);
      await vi.advanceTimersByTimeAsync(2_000);
    }
    expect(getAttachState("s-1").phase).toBe("reconnecting"); // 58 s of ladder, still trying

    socket().emitClose(1006); // exactly RETRY_BUDGET_MS since the ladder started
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
    expect(FakeWebSocket.instances.length).toBe(1); // the armed retry never fired
    expect(getAttachState("s-1").phase).toBe("ended");
    expect(getAttachState("s-1").reason).toBe("killed");
  });

  it("publishes the ladder's start stamp, so the pane can render an elapsed (§7.7)", () => {
    const startedAt = Date.now();
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    expect(getAttachState("s-1").ladderStartedAt).toBe(startedAt);

    socket().emitClose(1006);
    vi.advanceTimersByTime(250);
    // Same ladder, later rung: the stamp must NOT move, or the pane's elapsed resets every
    // 250 ms and stops meaning "how long this has been failing".
    expect(getAttachState("s-1").ladderStartedAt).toBe(startedAt);

    socket().acceptOpen();
    expect(getAttachState("s-1").phase).toBe("attached");
    expect(getAttachState("s-1").ladderStartedAt).toBe(0); // no ladder: nothing to count
  });

  it("stamps a fresh ladder when a long-attached socket drops", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    vi.advanceTimersByTime(61_000);
    const droppedAt = Date.now();
    socket().emitClose(1006);
    expect(getAttachState("s-1").ladderStartedAt).toBe(droppedAt);
  });

  it("clears the stamp when the ladder stops", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();
    socket().emitClose(1006);
    expect(getAttachState("s-1").ladderStartedAt).not.toBe(0);

    updateAttachSession(live({ status: "exited", exitReason: "killed" }));
    expect(getAttachState("s-1").phase).toBe("ended");
    expect(getAttachState("s-1").ladderStartedAt).toBe(0);
  });

  it("a listener that throws during give-up surfaces instead of vanishing into a floating promise", async () => {
    configureAttachRegistry({
      client: stubClient(async () => live({ status: "running" })),
    });
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    for (let i = 0; i < 30; i += 1) {
      socket().emitClose(1006);
      await vi.advanceTimersByTimeAsync(2_000);
    }
    const off = subscribeAttach("s-1", (state) => {
      if (state.phase === "failed") throw new Error("listener blew up");
    });

    socket().emitClose(1006); // budget spent: giveUp publishes "failed" synchronously
    off();
    await Promise.resolve(); // let the call site's .catch() run
    // Rethrown on a macrotask rather than swallowed: an unhandled rejection here would be
    // invisible, and giveUp's own catch must keep meaning "the diagnostic fetch failed".
    expect(() => vi.advanceTimersByTime(0)).toThrow("listener blew up");
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

// ADDED BY TASK 22, as a declared departure into this file — task 19 owns it and had closed it.
// The carry it answers: `failed` is TERMINAL. Nothing re-attempts on its own, and the pane's exit
// bar cannot offer the way out because it is absent by definition — the socket died while the
// session is NOT exited. Release-and-reacquire is not a retry either: after the grace it destroys
// the terminal and loses the transcript, and inside the grace it hands back the same spent entry.
// So the surface needed a tenth export, and task 22's ⋯ menu is where the verb that calls it went.
//
// Every assertion below is on the ACT — socket count, the stamp, the boolean — and not on the
// resulting state (§3A.1 clause 7). A retry's defect signature is "the same value, again": a
// double-connect, a no-op that reports success, or a ladder that restarts without its clock all
// leave `phase` looking exactly right.
describe("attachRegistry — retryAttach", () => {
  /** Drive the ladder to `failed` with a client that confirms the sandbox is still running. */
  async function spendTheLadder(): Promise<void> {
    configureAttachRegistry({
      client: stubClient(async () => live({ status: "running" })),
    });
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    for (let i = 0; i < 40; i += 1) {
      socket().emitClose(1006);
      await vi.advanceTimersByTimeAsync(2_000);
    }
    expect(getAttachState("s-1").phase).toBe("failed");
  }

  it("opens exactly one new socket for a ladder nothing else could restart", async () => {
    await spendTheLadder();
    const spent = FakeWebSocket.instances.length;

    expect(retryAttach("s-1")).toBe(true);

    // One, not two: `connect()` early-returns on a live `entry.ws`, and a retry that both
    // connected and re-armed the ladder would leave `phase` correct and two sockets open.
    expect(FakeWebSocket.instances.length).toBe(spent + 1);
    expect(getAttachState("s-1").phase).not.toBe("failed");
  });

  it("stamps a FRESH ladder, so the pane's elapsed starts counting again (§7.7)", async () => {
    await spendTheLadder();
    // giveUp zeroes the stamp: the spent ladder has no elapsed left to render.
    expect(getAttachState("s-1").ladderStartedAt).toBe(0);

    vi.advanceTimersByTime(5_000);
    const retriedAt = Date.now();
    retryAttach("s-1");

    // `ladderStartedAt !== 0` is the published meaning of "this pane is retrying", and it is
    // what task 20's footer renders an elapsed from. Left at 0 the pane shows a retrying session
    // with no elapsed; stamped by hand here it would be a second writer of an invariant that
    // connect()'s own `if (ladderStartedAt === 0)` already owns.
    expect(getAttachState("s-1").ladderStartedAt).toBe(retriedAt);
  });

  it("restarts the budget rather than resuming the spent one", async () => {
    await spendTheLadder();
    retryAttach("s-1");
    const sockets = FakeWebSocket.instances.length;

    // The bottom rung, from the bottom: attempts was reset, so this is 250 ms and not a
    // continuation of a ladder that had already run past the end of RETRY_DELAYS_MS.
    socket().emitClose(1006);
    vi.advanceTimersByTime(249);
    expect(FakeWebSocket.instances.length).toBe(sockets);
    vi.advanceTimersByTime(1);
    expect(FakeWebSocket.instances.length).toBe(sockets + 1);
  });

  // The two tests above run on the `failed` path, where giveUp has ALREADY zeroed the stamp — so
  // neither of them can tell `entry.ladderStartedAt = 0` from a no-op. These two run the export
  // on the other path it accepts, a ladder mid-flight, which is where that line and the
  // clearRetry beside it actually do something.
  it("restarts the budget clock even when a ladder is already running", () => {
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    const firstLadder = getAttachState("s-1").ladderStartedAt;
    expect(firstLadder).not.toBe(0);

    // Half the budget spent, then stop INSIDE an armed rung: `entry.ws` is null there, which is
    // the only mid-ladder moment a retry is accepted. A handshake already in flight is refused.
    for (let i = 0; i < 15; i += 1) {
      socket().emitClose(1006);
      vi.advanceTimersByTime(2_000);
    }
    socket().emitClose(1006);
    vi.advanceTimersByTime(100);
    const retriedAt = Date.now();

    expect(retryAttach("s-1")).toBe(true);
    // Not the old stamp: a user-initiated retry gets a whole budget, not the remainder of one.
    // This is the assertion that makes `entry.ladderStartedAt = 0` more than a no-op — on the
    // `failed` path giveUp has already zeroed it, so only this path can tell the two apart.
    expect(getAttachState("s-1").ladderStartedAt).toBe(retriedAt);
    expect(getAttachState("s-1").ladderStartedAt).not.toBe(firstLadder);
  });

  // Pins the TIMING of the ladder a retry starts, which is the part a state assertion cannot see
  // (§3A.1 clause 7). It does NOT prove that the rung armed before the retry was cancelled, and
  // an earlier version of this test claimed it did: a mutant adding `clearRetry` back into
  // `retryAttach` survives, because the stale rung is unobservable either way. See the note at
  // that line — `scheduleRetry` clears before arming, and `connect()` early-returns on a live
  // socket, so nothing the stale timer does can reach a socket count.
  it("runs the retry's own ladder from the bottom rung, timed from the retry's close", () => {
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    socket().emitClose(1006); // t=0: a 250 ms rung is armed
    expect(FakeWebSocket.instances.length).toBe(1);

    vi.advanceTimersByTime(100); // t=100
    retryAttach("s-1");
    expect(FakeWebSocket.instances.length).toBe(2); // the retry connected immediately
    socket().emitClose(1006); // t=100: the retry's rung is armed for t=350

    vi.advanceTimersByTime(150); // t=250 — where the pre-retry ladder's rung would have
    expect(FakeWebSocket.instances.length).toBe(2); // landed. Nothing opens here.

    vi.advanceTimersByTime(99); // t=349
    expect(FakeWebSocket.instances.length).toBe(2);
    vi.advanceTimersByTime(1); // t=350: 250 ms after the retry's own close, not 250
    expect(FakeWebSocket.instances.length).toBe(3); // after the ladder it replaced
  });

  it("refuses, and opens nothing, when no pane holds the session", () => {
    expect(retryAttach("s-nope")).toBe(false);
    expect(FakeWebSocket.instances.length).toBe(0);
  });

  it("refuses an exited session, because there is no sandbox to attach to", async () => {
    await spendTheLadder();
    updateAttachSession(live({ status: "exited", exitReason: "killed" }));
    const spent = FakeWebSocket.instances.length;

    expect(retryAttach("s-1")).toBe(false);
    expect(FakeWebSocket.instances.length).toBe(spent);
  });

  it("refuses once the pane is gone and the grace has run out", () => {
    acquireAttach(live({ status: "starting" }), {
      createTerm,
      readOnly: false,
    });
    releaseAttach("s-1");
    vi.advanceTimersByTime(GRACE_MS);
    const before = FakeWebSocket.instances.length;

    expect(retryAttach("s-1")).toBe(false);
    expect(FakeWebSocket.instances.length).toBe(before);
  });

  it("refuses while a socket is live, so it cannot blank an elapsed or double-connect", () => {
    acquireAttach(live(), { createTerm, readOnly: false });
    socket().acceptOpen();

    expect(retryAttach("s-1")).toBe(false);
    expect(FakeWebSocket.instances.length).toBe(1);
    expect(getAttachState("s-1").phase).toBe("attached");
  });
});
