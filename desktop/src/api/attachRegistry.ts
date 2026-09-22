// Sockets and xterm instances, keyed by session id, ref-counted, at module scope — NOT inside
// a React effect (contract §1.7). A dropped WebSocket destroys nothing: destruction is an
// explicit kill, the idle TTL, or the reaper collecting an orphan.

import { useCallback, useSyncExternalStore } from "react";
import { Kind, decode, encode, encodeResize } from "../lib/wire";
import { ApiError } from "./client";
import type { CockpitClient, ProblemCode } from "./client";
import { api } from "./index";
import type { Session, SessionId } from "../store/types";

/** WebSocket.OPEN, fixed at 1 by the HTML spec. Read from a constant so the registry never
 *  touches a global the test's fake socket does not own — decision (f). */
const WS_OPEN = 1;

/** §7.2. Flush at ~10 Hz: one term.write per flush, however much arrived. */
export const FLUSH_INTERVAL_MS = 100;
/** §1.7. How long a zero-ref entry survives, so StrictMode's mount/unmount/mount and a pane
 *  moving between layouts do not drop the PTY. */
export const GRACE_MS = 5_000;
/** §4.6. The delay BEFORE attempt n+1, capped at the last value. */
export const RETRY_DELAYS_MS = [250, 500, 1_000, 2_000] as const;
/** §4.6. A cold image pull is the outer bound; a shorter budget turns slow into false. */
export const RETRY_BUDGET_MS = 60_000;

/** The slice of xterm the registry uses. Structural, so the real Terminal and task 14's
 *  FakeTerm both satisfy it and this module imports no xterm — decision (a). */
export interface TermLike {
  readonly cols: number;
  readonly rows: number;
  buffer: { active: { viewportY: number; baseY: number } };
  open(parent: HTMLElement): void;
  focus(): void;
  write(data: string | Uint8Array, callback?: () => void): void;
  scrollToBottom(): void;
  onData(cb: (data: string) => void): { dispose(): void };
  onResize(cb: (size: { cols: number; rows: number }) => void): { dispose(): void };
  dispose(): void;
}

export type AttachPhase =
  | "detached"      // no entry: nothing is holding this session
  | "connecting"    // first handshake in flight
  | "attached"      // open
  | "reconnecting"  // the ladder is running after a failure or a close
  | "ended"         // the session exited. Not an error: the transcript stands
  | "failed";       // we could not attach and the session is NOT exited

export interface AttachState {
  phase: AttachPhase;
  /** Handshakes tried in the current ladder; back to 0 once one succeeds. */
  attempts: number;
  /** Rendered verbatim by the pane. Never a code, never a stack. */
  reason: string;
}

export interface AcquireOptions {
  /** Called at most once per session id. */
  createTerm: () => TermLike;
  /** session.status === "exited" (§6.6): the scrollback stays, onData is not forwarded. */
  readOnly: boolean;
}

export interface AttachHandle {
  /** Registry-owned. The component appends and removes it; it is destroyed only here. */
  element: HTMLDivElement;
  term: TermLike;
  /** Idempotent, and called AFTER the element is in the document — decision (c). */
  mount(): void;
  focus(): void;
}

export interface RegistryDeps {
  client: Pick<CockpitClient, "openAttach" | "getSession">;
}

interface Entry {
  id: SessionId;
  session: Session;
  term: TermLike;
  element: HTMLDivElement;
  opened: boolean;
  refs: number;
  readOnly: boolean;
  ws: WebSocket | null;
  state: AttachState;
  pending: Uint8Array[];
  pendingBytes: number;
  flushTimer: ReturnType<typeof setTimeout> | null;
  graceTimer: ReturnType<typeof setTimeout> | null;
  retryTimer: ReturnType<typeof setTimeout> | null;
  attempts: number;
  /** 0 = no ladder is running. Stamped when one starts, cleared when a handshake succeeds. */
  ladderStartedAt: number;
  /** A socket for this entry has been open at least once. The next connect is therefore a
   *  RE-connect, whatever `attempts` reads after the success reset it to 0. */
  everAttached: boolean;
  disposables: { dispose(): void }[];
  destroyed: boolean;
}

/** One frozen instance, never re-allocated — decision (d). */
const DETACHED: AttachState = Object.freeze({ phase: "detached", attempts: 0, reason: "" });
const ENCODER = new TextEncoder();
const BANNER_ENDED = ENCODER.encode("\r\n\x1b[2m[session ended]\x1b[0m\r\n");

const entries = new Map<SessionId, Entry>();
const listeners = new Map<SessionId, Set<(state: AttachState) => void>>();
let deps: RegistryDeps = { client: api };

export function configureAttachRegistry(next: Partial<RegistryDeps>): void {
  deps = { ...deps, ...next };
}

/* ── the public surface ──────────────────────────────────────────────────────────────── */

export function acquireAttach(session: Session, opts: AcquireOptions): AttachHandle {
  let entry = entries.get(session.id);
  if (!entry) {
    const element = document.createElement("div");
    element.dataset.sessionId = session.id;
    element.style.width = "100%";
    element.style.height = "100%";
    entry = {
      id: session.id,
      session,
      term: opts.createTerm(),
      element,
      opened: false,
      refs: 0,
      readOnly: opts.readOnly,
      ws: null,
      state: DETACHED,
      pending: [],
      pendingBytes: 0,
      flushTimer: null,
      graceTimer: null,
      retryTimer: null,
      attempts: 0,
      ladderStartedAt: 0,
      everAttached: false,
      disposables: [],
      destroyed: false,
    };
    entries.set(session.id, entry);
    wireTerm(entry);
    openIfLive(entry);
  } else {
    entry.session = session;
  }
  entry.readOnly = opts.readOnly;
  entry.refs += 1;
  if (entry.graceTimer !== null) {
    clearTimeout(entry.graceTimer);
    entry.graceTimer = null;
  }
  const held = entry;
  return {
    element: held.element,
    term: held.term,
    mount: () => {
      if (held.opened) return;
      held.term.open(held.element);
      held.opened = true;
    },
    focus: () => held.term.focus(),
  };
}

export function releaseAttach(sessionId: SessionId): void {
  const entry = entries.get(sessionId);
  if (!entry) return;
  entry.refs = Math.max(0, entry.refs - 1);
  if (entry.refs > 0) return;
  if (entry.graceTimer !== null) clearTimeout(entry.graceTimer);
  entry.graceTimer = setTimeout(() => destroy(entry), GRACE_MS);
}

/** The poller's view of the session. The registry cannot derive `status` or `attachPath` and
 *  must not run a fetch loop of its own to learn them. */
export function updateAttachSession(session: Session): void {
  const entry = entries.get(session.id);
  if (!entry) return;
  const wasExited = entry.session.status === "exited";
  entry.session = session;
  if (session.status === "exited" && !wasExited) {
    clearRetry(entry);
    if (!entry.ws) {
      publish(entry, { phase: "ended", attempts: entry.attempts, reason: exitedReason(session) });
    }
  }
}

export function setAttachReadOnly(sessionId: SessionId, readOnly: boolean): void {
  const entry = entries.get(sessionId);
  if (entry) entry.readOnly = readOnly;
}

export function getAttachState(sessionId: SessionId): AttachState {
  return entries.get(sessionId)?.state ?? DETACHED;
}

export function subscribeAttach(
  sessionId: SessionId,
  listener: (state: AttachState) => void,
): () => void {
  let set = listeners.get(sessionId);
  if (!set) {
    set = new Set();
    listeners.set(sessionId, set);
  }
  set.add(listener);
  return () => {
    const current = listeners.get(sessionId);
    if (!current) return;
    current.delete(listener);
    if (current.size === 0) listeners.delete(sessionId);
  };
}

/** The footer's attach word (§6.5) reads this, not the store: attach state is transport state,
 *  and putting it in the store would turn every socket event into a store write and every
 *  store write into a re-render of the rail. */
export function useAttachState(sessionId: SessionId): AttachState {
  const subscribe = useCallback(
    (onChange: () => void) => subscribeAttach(sessionId, onChange),
    [sessionId],
  );
  const snapshot = useCallback(() => getAttachState(sessionId), [sessionId]);
  return useSyncExternalStore(subscribe, snapshot);
}

/** Test seam: drop every entry and restore the default client. */
export function __resetAttachRegistry(): void {
  for (const entry of [...entries.values()]) destroy(entry);
  entries.clear();
  listeners.clear();
  deps = { client: api };
}

/* ── internals ───────────────────────────────────────────────────────────────────────── */

function publish(entry: Entry, state: AttachState): void {
  entry.state = state;
  notify(entry.id, state);
}

function notify(id: SessionId, state: AttachState): void {
  const set = listeners.get(id);
  if (!set) return;
  for (const listener of [...set]) listener(state);
}

/** "Rendered verbatim" (§2, Session.exitReason). The fallback is only for an empty string. */
function exitedReason(session: Session): string {
  return session.exitReason.trim() !== "" ? session.exitReason : "the session has exited";
}

function wireTerm(entry: Entry): void {
  entry.disposables.push(
    entry.term.onData((data) => {
      if (entry.readOnly) return;   // exited: a keystroke does nothing, by design (§6.6)
      const ws = entry.ws;
      if (!ws || ws.readyState !== WS_OPEN) return;
      ws.send(encode(Kind.Stdin, ENCODER.encode(data)));
    }),
  );
  entry.disposables.push(
    entry.term.onResize(({ cols, rows }) => {
      const ws = entry.ws;
      if (!ws || ws.readyState !== WS_OPEN) return;
      ws.send(encodeResize(cols, rows));
    }),
  );
}

/** An exited session has no sandbox to attach to, so the registry does not ask — 409 is the
 *  only answer the attach route can give. The pane is still not blank: the reason is written
 *  into the terminal directly, NOT through the flush buffer, because there is no stream for it
 *  to be ordered against. */
function openIfLive(entry: Entry): void {
  if (entry.session.status === "exited") {
    const reason = exitedReason(entry.session);
    entry.term.write(`\r\n\x1b[2m[${reason}]\x1b[0m\r\n`);
    publish(entry, { phase: "ended", attempts: 0, reason });
    return;
  }
  connect(entry);
}

function connect(entry: Entry): void {
  if (entry.destroyed || entry.ws) return;
  // Stamped on `ladderStartedAt === 0`, not on `attempts === 0`: a successful handshake resets
  // attempts to 0, so keying off it would re-stamp mid-ladder and the 60 s budget would never
  // expire. `onopen` clears the stamp, which is what makes a drop after an hour attached start
  // a FRESH 60 s budget rather than exhaust one that began an hour ago.
  if (entry.ladderStartedAt === 0) entry.ladderStartedAt = Date.now();
  entry.attempts += 1;
  publish(entry, {
    // "connecting" is only ever the FIRST handshake of this entry's life. Once a socket has
    // opened, every later one is a reconnect — the footer must not read "connecting" at the
    // user while a live session is being picked back up.
    phase: !entry.everAttached && entry.attempts === 1 ? "connecting" : "reconnecting",
    attempts: entry.attempts,
    reason: "",
  });

  let ws: WebSocket;
  try {
    // No Authorization header, here or anywhere: there is no bearer token on this hop before
    // Phase 8 (§4.5). The browser attaches Origin to the handshake by itself.
    ws = deps.client.openAttach(entry.session);
  } catch {
    scheduleRetry(entry);   // openAttach throws only on a URL it cannot construct
    return;
  }
  entry.ws = ws;

  ws.onopen = () => {
    if (entry.ws !== ws || entry.destroyed) return;
    // A successful handshake retires the whole ladder: the count AND the budget clock. Leaving
    // the clock set is the bug where a session attached for over a minute gives up instantly
    // the first time its socket drops.
    entry.attempts = 0;
    entry.ladderStartedAt = 0;
    entry.everAttached = true;
    publish(entry, { phase: "attached", attempts: 0, reason: "" });
    ws.send(encodeResize(entry.term.cols, entry.term.rows));
  };

  ws.onmessage = (ev: MessageEvent) => {
    if (entry.ws !== ws || entry.destroyed) return;
    if (!(ev.data instanceof ArrayBuffer)) return;   // the attach protocol is binary-only
    const { kind, payload } = decode(new Uint8Array(ev.data));
    if (kind === Kind.Stdout) writeOut(entry, payload);
    else if (kind === Kind.Exit) writeOut(entry, BANNER_ENDED);
  };

  ws.onclose = () => {
    if (entry.ws !== ws) return;
    entry.ws = null;
    if (entry.destroyed) return;
    scheduleRetry(entry);
  };

  // A refused handshake surfaces as error and THEN close. The ladder lives in onclose so it
  // runs exactly once per socket, whichever way that socket died.
  ws.onerror = () => {};
}

/** §7.2. Payloads accumulate and flush on a 100 ms timer as ONE write, so a build log at
 *  4 000 lines/second costs ten writes a second and not four thousand. The timer is armed only
 *  when something is buffered, so an idle session costs nothing.
 *
 *  There is deliberately no byte cap on the buffer: 100 ms of output is bounded by the
 *  socket's own backpressure, and xterm's scrollback (raised to 10 000 lines in Terminal.tsx)
 *  is the real bound on what is kept. A cap here would silently drop transcript. */
function writeOut(entry: Entry, bytes: Uint8Array): void {
  if (bytes.length === 0) return;
  entry.pending.push(bytes);
  entry.pendingBytes += bytes.length;
  if (entry.flushTimer !== null) return;
  entry.flushTimer = setTimeout(() => flush(entry), FLUSH_INTERVAL_MS);
}

function flush(entry: Entry): void {
  entry.flushTimer = null;
  if (entry.destroyed || entry.pending.length === 0) return;
  const chunk = concatChunks(entry.pending, entry.pendingBytes);
  entry.pending = [];
  entry.pendingBytes = 0;

  // Measured BEFORE the write, because the write itself moves baseY (§7.2, known defect 5).
  // Scrolled from the write callback rather than inline, because term.write is asynchronous:
  // xterm parses on its own schedule, and scrolling before the parse lands leaves the
  // viewport one chunk short.
  const buffer = entry.term.buffer.active;
  const pinned = buffer.viewportY >= buffer.baseY;
  entry.term.write(chunk, () => {
    if (pinned) entry.term.scrollToBottom();
  });
}

/** One allocation, byte-exact. Concatenating BYTES rather than decoding each payload is what
 *  keeps a multibyte character split across two frames intact: a per-chunk TextDecoder without
 *  { stream: true } turns "→" arriving as e2 86 | 92 into two replacement characters. */
function concatChunks(chunks: Uint8Array[], total: number): Uint8Array {
  if (chunks.length === 1) return chunks[0];
  const out = new Uint8Array(total);
  let at = 0;
  for (const chunk of chunks) {
    out.set(chunk, at);
    at += chunk.length;
  }
  return out;
}

/** §4.6's ladder. Called once per socket that died, whichever way it died. */
function scheduleRetry(entry: Entry): void {
  if (entry.destroyed) return;

  // Amends §4.6: the ladder runs for starting, running AND idle, and stops only on exited.
  // An idle session is a live sandbox that has been quiet; stranding it detached is a pane
  // that can never come back, because the output that would revive it is the output we are
  // no longer attached to receive.
  if (entry.session.status === "exited") {
    publish(entry, {
      phase: "ended",
      attempts: entry.attempts,
      reason: exitedReason(entry.session),
    });
    return;
  }

  // A socket that opened cleared the clock, so this close begins a new ladder and gets the
  // full budget. Stamped here rather than only in connect(), because the budget test below
  // runs before the first connect of the new ladder.
  if (entry.ladderStartedAt === 0) entry.ladderStartedAt = Date.now();

  if (Date.now() - entry.ladderStartedAt >= RETRY_BUDGET_MS) {
    void giveUp(entry);
    return;
  }

  // Rung = attempts - 1, FLOORED AT ZERO. `attempts` is 0 here whenever the socket had opened
  // (onopen reset it), and `RETRY_DELAYS_MS[-1]` is `undefined`, which setTimeout reads as 0 —
  // a live session that drops would reconnect in a tight loop instead of after §4.6's 250 ms.
  const rung = Math.min(Math.max(entry.attempts - 1, 0), RETRY_DELAYS_MS.length - 1);
  const delay = RETRY_DELAYS_MS[rung];
  publish(entry, { phase: "reconnecting", attempts: entry.attempts, reason: "" });
  clearRetry(entry);
  entry.retryTimer = setTimeout(() => {
    entry.retryTimer = null;
    connect(entry);
  }, delay);
}

/** The ladder is spent. One read of the record, so the sentence the pane renders is the truth
 *  rather than the likeliest-looking code — see the note above this step. */
async function giveUp(entry: Entry): Promise<void> {
  publish(entry, {
    phase: "failed",
    attempts: entry.attempts,
    reason: "the sandbox did not accept a connection within 60 seconds",
  });

  let fresh: Session;
  try {
    fresh = await deps.client.getSession(entry.id);
  } catch (error) {
    if (entry.destroyed) return;
    publish(entry, { phase: "failed", attempts: entry.attempts, reason: reasonForError(error) });
    return;
  }
  if (entry.destroyed) return;

  entry.session = fresh;
  if (fresh.status === "exited") {
    publish(entry, { phase: "ended", attempts: entry.attempts, reason: exitedReason(fresh) });
    return;
  }
  publish(entry, {
    phase: "failed",
    attempts: entry.attempts,
    reason: `the sandbox is ${fresh.status} but refused the attach connection`,
  });
}

function reasonForError(error: unknown): string {
  if (!(error instanceof ApiError)) return "the control plane could not be reached";
  return reasonForCode(error.code, error.message);
}

/** NOT an exhaustive switch, deliberately. Eight of the eighteen `ProblemCode` members
 *  (§4.2's fifteen server codes, the non_loopback_publish Group C's ruling 7 added, and the
 *  two the client raises itself) are named below, in seven sentences — forbidden_origin and
 *  forbidden_host share one — because the server's own
 *  message for those is a protocol fact rather than something to show a user. The other ten
 *  codes fall through to `default`, which renders the server's message
 *  verbatim — those messages are already written for a human, and paraphrasing them here would
 *  be a second copy to drift.
 *
 *  Adding a case is safe; removing `default` is not. An exhaustive switch over this union
 *  would break every time the control plane adds a code, in a UI file, for no gain. */
function reasonForCode(code: ProblemCode, message: string): string {
  switch (code) {
    case "network_unavailable":
      return "the control plane is not running";
    case "timeout":
      return "the control plane did not answer in time";
    case "backend_unavailable":
      return "Docker Desktop is not running";
    case "session_not_found":
      return "the control plane no longer has a record of this session";
    case "forbidden_origin":
    case "forbidden_host":
      return "the control plane refused this window";
    case "sandbox_not_running":
      return "the sandbox is not running";
    case "non_loopback_publish":
      // The sixteenth code. The sandbox IS running, so this must never read "not running".
      return "the sandbox is running but published its port off loopback, so attaching is refused";
    default:
      return message;
  }
}

function clearRetry(entry: Entry): void {
  if (entry.retryTimer === null) return;
  clearTimeout(entry.retryTimer);
  entry.retryTimer = null;
}

function destroy(entry: Entry): void {
  if (entry.destroyed) return;
  entry.destroyed = true;
  clearRetry(entry);
  if (entry.flushTimer !== null) { clearTimeout(entry.flushTimer); entry.flushTimer = null; }
  if (entry.graceTimer !== null) { clearTimeout(entry.graceTimer); entry.graceTimer = null; }
  for (const disposable of entry.disposables) disposable.dispose();
  entry.disposables = [];
  const ws = entry.ws;
  entry.ws = null;
  // Closing the socket detaches. It does NOT destroy the session: the record and its
  // transcript survive, and only a kill, the idle TTL or the reaper remove them.
  if (ws) ws.close(1000, "pane closed");
  entry.term.dispose();
  entry.element.remove();
  entries.delete(entry.id);
  notify(entry.id, DETACHED);
}
