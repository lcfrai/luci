// Sockets and xterm instances are keyed by session id and ref-counted at module scope.
// Navigation retains a bounded temporary cache. Releasing a terminal view or
// disconnecting its socket never stops the worker session.

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
/** The grace period must not retain every terminal visited during rapid navigation. */
export const MAX_IDLE_TERMINALS = 8;
/** Bound bytes awaiting parsing, including the one outstanding xterm write. */
export const MAX_OUTPUT_BYTES = 1024 * 1024;
/** Tiny frames also consume array/object overhead even when their byte total is small. */
export const MAX_OUTPUT_CHUNKS = 1024;
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
  onResize(cb: (size: { cols: number; rows: number }) => void): {
    dispose(): void;
  };
  dispose(): void;
}

export type AttachPhase =
  | "detached" // no entry: nothing is holding this session
  | "connecting" // first handshake in flight
  | "attached" // open
  | "reconnecting" // the ladder is running after a failure or a close
  | "ended" // the session exited. Not an error: the transcript stands
  | "failed"; // we could not attach and the session is NOT exited

export interface AttachState {
  phase: AttachPhase;
  /** Handshakes tried in the current ladder; back to 0 once one succeeds. */
  attempts: number;
  /** `Date.now()` when the CURRENT ladder started, and **0 whenever no ladder is running** —
   *  so `ladderStartedAt !== 0` is exactly "this pane is retrying". Contract §7.7 asks the pane
   *  for "the elapsed retry time, not a spinner", and `attempts` cannot supply that: a number
   *  that ticks is the thing that section is asking for.
   *
   *  The RAW STAMP is published, never a computed elapsed. The pane subtracts it from
   *  `useHeartbeat()`'s 1 Hz tick at render time and prints the result in `var(--numeric)`,
   *  which is what keeps the registry timer-free — a registry that published an elapsed would
   *  need a clock of its own, and §7.1 already has the one clock that job belongs to. */
  ladderStartedAt: number;
  /** Rendered verbatim by the pane. Never a code, never a stack. */
  reason: string;
}

export interface AcquireOptions {
  /** Called at most once per session id. */
  createTerm: () => TermLike;
  /** session.status === "exited" (§6.6): the scrollback stays, onData is not forwarded. */
  readOnly: boolean;
  /** Current project execution authority; false keeps cached output without a socket. */
  permitted?: boolean;
  /** Pause new handshakes during a background refresh without dropping an open stream. */
  connectPermitted?: boolean;
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
  /** Last known opaque allocation identity; omission cannot erase a terminal outcome. */
  executionGeneration: string | null;
  terminalExit: boolean;
  permitted: boolean;
  connectPermitted: boolean;
  ws: WebSocket | null;
  opening: boolean;
  connectVersion: number;
  state: AttachState;
  pending: Uint8Array[];
  pendingBytes: number;
  writingBytes: number;
  overloaded: boolean;
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
const DETACHED: AttachState = Object.freeze({
  phase: "detached",
  attempts: 0,
  ladderStartedAt: 0,
  reason: "",
});
const ENCODER = new TextEncoder();
const BANNER_ENDED = ENCODER.encode("\r\n\x1b[2m[session ended]\x1b[0m\r\n");

const entries = new Map<SessionId, Entry>();
// Insertion order is release order; a re-acquired entry leaves this cache immediately.
const idleEntries = new Map<SessionId, Entry>();
const listeners = new Map<SessionId, Set<(state: AttachState) => void>>();
let deps: RegistryDeps = { client: api };

export function configureAttachRegistry(next: Partial<RegistryDeps>): void {
  deps = { ...deps, ...next };
}

/* ── the public surface ──────────────────────────────────────────────────────────────── */

export function acquireAttach(
  session: Session,
  opts: AcquireOptions,
): AttachHandle {
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
      executionGeneration: knownExecutionGeneration(session),
      terminalExit: false,
      permitted: opts.permitted ?? true,
      connectPermitted: opts.connectPermitted ?? opts.permitted ?? true,
      ws: null,
      opening: false,
      connectVersion: 0,
      state: DETACHED,
      pending: [],
      pendingBytes: 0,
      writingBytes: 0,
      overloaded: false,
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
    updateAttachSession(session);
    setAttachPermission(session.id, opts.permitted ?? true, opts.connectPermitted ?? opts.permitted ?? true);
  }
  // LAST ACQUIRER WINS, and this line runs on the shared path too — so read it as the
  // invariant it depends on: **`readOnly` is a property of the SESSION, never of the pane.**
  // It is `session.status === "exited"` and nothing else (§6.6). Two panes on one session
  // therefore always agree, and overwriting is a no-op. The day a surface acquires the same
  // session with a different intent — a read-only preview tile beside a live pane — this line
  // silences the live pane's stdin, and the defect will look like a keyboard bug rather than a
  // ref-counting one. If that surface is ever wanted, readOnly moves out of the shared entry
  // and onto the handle; do not "fix" it by making this conditional.
  entry.readOnly = opts.readOnly;
  entry.refs += 1;
  idleEntries.delete(session.id);
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
  idleEntries.delete(sessionId);
  idleEntries.set(sessionId, entry);
  while (idleEntries.size > MAX_IDLE_TERMINALS) {
    const oldest = idleEntries.values().next().value;
    if (!oldest) break;
    destroy(oldest);
  }
}

/** The poller's view of the session. The registry cannot derive `status` or `attachPath` and
 *  must not run a fetch loop of its own to learn them. */
/** Allocation fingerprints are opaque identities, never ordered timestamps or authority. */
function knownExecutionGeneration(session: Session): string | null {
  return typeof session.executionGeneration === "string" && /^[0-9a-f]{64}$/.test(session.executionGeneration)
    ? session.executionGeneration : null;
}

function sameSessionScope(a: Session, b: Session): boolean {
  return a.id === b.id && a.machineId === b.machineId && a.projectId === b.projectId &&
    a.branchId === b.branchId && a.worktreeId === b.worktreeId && a.attachPath === b.attachPath;
}

function publishTerminalExit(entry: Entry): void {
  clearRetry(entry);
  entry.ladderStartedAt = 0;
  publish(entry, {
    phase: "ended", attempts: entry.attempts, ladderStartedAt: 0,
    reason: entry.session.status === "exited" ? exitedReason(entry.session) : "The terminal process has ended.",
  });
}

export function updateAttachSession(session: Session): void {
  const entry = entries.get(session.id);
  if (!entry || !sameSessionScope(entry.session, session)) return;
  const generation = knownExecutionGeneration(session);
  const replacement = generation !== null && entry.executionGeneration !== null && generation !== entry.executionGeneration;
  const wasExited = entry.session.status === "exited";
  entry.session = session;
  if (replacement) {
    clearRetry(entry);
    entry.connectVersion += 1;
    entry.opening = false;
    const old = entry.ws;
    entry.ws = null;
    old?.close(1000, "session allocation changed");
    entry.executionGeneration = generation;
    entry.terminalExit = false;
    entry.attempts = 0;
    entry.ladderStartedAt = 0;
    openIfLive(entry);
    return;
  }
  if (generation !== null && !(entry.terminalExit && entry.executionGeneration === null)) entry.executionGeneration = generation;
  if (entry.terminalExit && !entry.overloaded) {
    publishTerminalExit(entry);
  } else if (session.status === "exited" && !wasExited) {
    clearRetry(entry);
    entry.ladderStartedAt = 0;
    if (!entry.ws) publishTerminalExit(entry);
  }
}

/** Older servers have no allocation identity. Reconnecting remains an explicit
 * transport action; an exited session must instead be restarted by the user. */
export function canReconnectEndedAttach(sessionId: SessionId): boolean {
  const entry = entries.get(sessionId);
  return !!entry && !entry.destroyed && entry.terminalExit && entry.executionGeneration === null &&
    entry.session.status !== "exited" && entry.permitted && entry.connectPermitted && !entry.ws && !entry.opening;
}

export function setAttachReadOnly(
  sessionId: SessionId,
  readOnly: boolean,
): void {
  const entry = entries.get(sessionId);
  if (entry) entry.readOnly = readOnly;
}

/** Changing access detaches the stream but retains the terminal and its scrollback. */
export function setAttachPermission(sessionId: SessionId, permitted: boolean, connectPermitted = permitted): void {
  const entry = entries.get(sessionId);
  if (!entry || entry.destroyed || (entry.permitted === permitted && entry.connectPermitted === connectPermitted)) return;
  entry.permitted = permitted;
  entry.connectPermitted = permitted && connectPermitted;
  if (entry.connectPermitted) {
    openIfLive(entry);
    return;
  }
  clearRetry(entry);
  entry.connectVersion += 1;
  entry.opening = false;
  entry.attempts = 0;
  entry.ladderStartedAt = 0;
  const ws = entry.ws;
  if (permitted && ws?.readyState === WS_OPEN) return;
  entry.ws = null;
  ws?.close(1000, "project execution access paused");
  if (entry.session.status !== "exited" && !entry.overloaded) publish(entry, {
    phase: "detached", attempts: 0, ladderStartedAt: 0,
    reason: "Terminal access requires current project execution permission.",
  });
}

/** Start a FRESH ladder for a pane whose previous one is spent. Returns false, having done
 *  nothing, when there is nothing to retry.
 *
 *  ADDED BY TASK 22 as a declared departure — this file is task 19's. The gap it closes:
 *  **`failed` is terminal.** Nothing in this surface re-attempts, and the pane's exit bar cannot
 *  offer the way out because it is absent *by definition* — the socket died while the session is
 *  NOT exited, so the Restart that answers `exited` never appears. Release-and-reacquire is not a
 *  retry: after the grace it destroys the terminal and loses the transcript, and inside the grace
 *  it hands back this same exhausted entry. Task 22's ⋯ menu carries the verb that calls this.
 *
 *  **It resets the two counters and then calls `connect()` rather than stamping the clock
 *  itself**, and that is the load-bearing part. `ladderStartedAt !== 0` is the published meaning
 *  of "this pane is retrying" (§7.7), which task 20's footer renders an elapsed from. `connect()`
 *  already owns the stamp with its own `if (entry.ladderStartedAt === 0)`, so zeroing here and
 *  letting it stamp keeps ONE writer of that invariant. Stamping by hand would make two; leaving
 *  the zero in place would render a retrying pane with no elapsed. */
export function retryAttach(sessionId: SessionId): boolean {
  const entry = entries.get(sessionId);
  // `entry.destroyed` is checked for the same reason the four guards named at `destroy()` are,
  // and it is masked in the same way: `destroy()` deletes the entry, so today the first half
  // always catches it first. It stops being masked the moment anything marks an entry destroyed
  // without removing it from `entries` — the same expiry condition already written at destroy().
  if (!entry || entry.destroyed || !entry.permitted || !entry.connectPermitted) return false;
  // An exited session has no sandbox to attach to; 409 is the only answer the attach route can
  // give, which is the rule `openIfLive` states.
  if (entry.session.status === "exited" || (entry.terminalExit && !canReconnectEndedAttach(sessionId))) return false;
  // A socket is open, or a handshake is already in flight: either way an attempt is running, so
  // there is no spent ladder to restart and zeroing the stamp would blank an elapsed the pane is
  // rendering. `connect()` would early-return on the same condition, so without this the call
  // would report success having done nothing. The one mid-ladder moment this DOES accept is an
  // armed rung, where `ws` is null.
  if (entry.ws || entry.opening) return false;
  // Never reset the parser or attach a replay while an earlier parse still owns bytes.
  if (entry.writingBytes > 0) return false;
  if (entry.overloaded) {
    entry.overloaded = false;
    // A disconnected stream can end inside an escape sequence. Cancel that sequence
    // and reset the display before replay; never join two incomplete transcripts.
    writeOut(entry, ENCODER.encode("\x18\x1bc"));
  }
  // NO `clearRetry` HERE, and this comment is what the deleted line left behind. A mutant adding
  // one back SURVIVES every test, because three separate mechanisms already make a rung armed
  // before this call unobservable by the time it could matter:
  //
  //   1. `scheduleRetry` calls `clearRetry` itself before arming the next rung, so any close
  //      after this point cancels the stale timer.
  //   2. With no close, the stale timer fires into a `connect()` that early-returns on the live
  //      `entry.ws` this retry is about to set — and nulls `entry.retryTimer` on its way out.
  //   3. `destroy()` clears it in the teardown path.
  //
  // WHAT WOULD HAVE TO CHANGE for the absence to become a defect: any ONE of those three. If
  // `scheduleRetry` stops clearing before it arms, or `connect()` stops early-returning on a live
  // socket, or `destroy()` stops clearing, then a rung armed before this retry can reach a
  // `connect()` with a null `ws` and open a second socket off the ladder this call replaced —
  // two ladders interleaved, each arming rungs the other satisfies. At that point `retryAttach`
  // needs its own `clearRetry(entry)` back, on this line.
  entry.attempts = 0;
  // DECLARED EQUIVALENT MUTANT, and it is equivalent in value rather than in meaning. Replacing
  // this `0` with `Date.now()` survives the suite, because `connect()` runs SYNCHRONOUSLY on the
  // next line and its own conditional stamp reads the very same millisecond — the published
  // `ladderStartedAt` is identical either way.
  //
  // What would have to change for it to stop being equivalent: anything landing between these
  // two lines — an await, a publish, a listener callback — or `connect()` ceasing to stamp
  // conditionally. Then the two diverge, and the hand-stamped version is a second writer of an
  // invariant `connect()` owns. That condition is mechanically checkable, so it is a guard and
  // not only this comment:
  //     grep -A1 '^  entry.ladderStartedAt = 0;$' attachRegistry.ts | grep -c '^  connect(entry);$'
  entry.ladderStartedAt = 0;
  entry.terminalExit = false; // Only an explicit legacy reconnect reaches this reset.
  entry.executionGeneration = knownExecutionGeneration(entry.session) ?? entry.executionGeneration;
  connect(entry);
  return true;
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
  detachAllTerminals();
  listeners.clear();
  deps = { client: api };
}

/** Detach local terminal views without stopping the sessions on their original machine. */
export function detachAllTerminals(): void {
  for (const entry of [...entries.values()]) destroy(entry);
  entries.clear();
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
  return session.exitReason.trim() !== ""
    ? session.exitReason
    : "the session has exited";
}

function wireTerm(entry: Entry): void {
  entry.disposables.push(
    entry.term.onData((data) => {
      if (entry.readOnly || entry.terminalExit || !entry.permitted) return; // exited or revoked: no input reaches the agent
      const ws = entry.ws;
      if (!ws || ws.readyState !== WS_OPEN) return;
      ws.send(encode(Kind.Stdin, ENCODER.encode(data)));
    }),
  );
  entry.disposables.push(
    entry.term.onResize(({ cols, rows }) => {
      if (!entry.permitted) return;
      const ws = entry.ws;
      if (!ws || ws.readyState !== WS_OPEN) return;
      ws.send(encodeResize(cols, rows));
    }),
  );
}

/** An exited session has no sandbox to attach to. Its reason shares the same
 * bounded parser queue as output, including an earlier write still completing. */
function openIfLive(entry: Entry): void {
  if (entry.terminalExit) {
    if (!entry.overloaded) publishTerminalExit(entry);
    return;
  }
  if (entry.session.status === "exited") {
    const reason = exitedReason(entry.session);
    writeOut(entry, ENCODER.encode(`\r\n\x1b[2m[${reason}]\x1b[0m\r\n`));
    if (entry.writingBytes === 0) {
      if (entry.flushTimer !== null) clearTimeout(entry.flushTimer);
      flush(entry);
    }
    publish(entry, { phase: "ended", attempts: 0, ladderStartedAt: 0, reason });
    return;
  }
  connect(entry);
}

function connect(entry: Entry): void {
  if (entry.destroyed || entry.terminalExit || entry.overloaded || !entry.permitted || !entry.connectPermitted || entry.ws || entry.opening) return;
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
    phase:
      !entry.everAttached && entry.attempts === 1
        ? "connecting"
        : "reconnecting",
    attempts: entry.attempts,
    // Stamped just above if this is the first rung; unchanged on every later rung of the same
    // ladder, so the pane's elapsed keeps growing rather than resetting every 250 ms.
    ladderStartedAt: entry.ladderStartedAt,
    reason: "",
  });

  const version = ++entry.connectVersion;
  entry.opening = true;
  const failed = () => {
    if (entry.destroyed || version !== entry.connectVersion) return;
    entry.opening = false;
    scheduleRetry(entry);
  };
  const ready = (ws: WebSocket) => {
    if (version === entry.connectVersion) entry.opening = false;
    if (
      entry.destroyed ||
      !entry.permitted ||
      !entry.connectPermitted ||
      version !== entry.connectVersion ||
      entry.session.status === "exited"
    ) {
      ws.close(1000, "attachment no longer needed");
      return;
    }
    bindSocket(entry, ws);
  };
  try {
    const socket = deps.client.openAttach(entry.session);
    if ("then" in socket) void socket.then(ready, failed);
    else ready(socket);
  } catch {
    failed();
  }
}

function bindSocket(entry: Entry, ws: WebSocket): void {
  entry.ws = ws;

  ws.onopen = () => {
    if (entry.ws !== ws || entry.destroyed) return;
    // A successful handshake retires the whole ladder: the count AND the budget clock. Leaving
    // the clock set is the bug where a session attached for over a minute gives up instantly
    // the first time its socket drops.
    entry.attempts = 0;
    entry.ladderStartedAt = 0;
    entry.everAttached = true;
    publish(entry, {
      phase: "attached",
      attempts: 0,
      ladderStartedAt: 0,
      reason: "",
    });
    ws.send(encodeResize(entry.term.cols, entry.term.rows));
  };

  ws.onmessage = (ev: MessageEvent) => {
    if (entry.ws !== ws || entry.destroyed) return;
    if (!(ev.data instanceof ArrayBuffer)) return; // the attach protocol is binary-only
    const { kind, payload } = decode(new Uint8Array(ev.data));
    if (kind === Kind.Stdout) writeOut(entry, payload);
    else if (kind === Kind.Exit && !entry.terminalExit) {
      // The typed frame ends this allocation, irrespective of its textual reason.
      // It does not prove success, output verification, cleanup or integration.
      entry.terminalExit = true;
      writeOut(entry, BANNER_ENDED);
      if (!entry.overloaded) publishTerminalExit(entry);
    }
  };

  ws.onclose = (event) => {
    if (entry.ws !== ws) return;
    entry.ws = null;
    if (entry.destroyed) return;
    if (event.code === 1013) {
      pauseOverloaded(entry);
      return;
    }
    scheduleRetry(entry);
  };

  // A refused handshake surfaces as error and THEN close. The ladder lives in onclose so it
  // runs exactly once per socket, whichever way that socket died.
  ws.onerror = () => {};
}

/** Browser WebSockets do not provide receive backpressure. Bound the application queue
 * and xterm's asynchronous parser together; stop this view visibly if it cannot keep up. */
function writeOut(entry: Entry, bytes: Uint8Array): void {
  if (entry.destroyed || entry.overloaded || bytes.length === 0) return;
  if (bytes.length + entry.pendingBytes + entry.writingBytes > MAX_OUTPUT_BYTES || entry.pending.length >= MAX_OUTPUT_CHUNKS) {
    pauseOverloaded(entry);
    return;
  }
  entry.pending.push(bytes);
  entry.pendingBytes += bytes.length;
  scheduleFlush(entry);
}

function scheduleFlush(entry: Entry): void {
  if (entry.destroyed || entry.overloaded || entry.writingBytes > 0 || entry.pending.length === 0 || entry.flushTimer !== null) return;
  entry.flushTimer = setTimeout(() => flush(entry), FLUSH_INTERVAL_MS);
}

function pauseOverloaded(entry: Entry): void {
  entry.overloaded = true;
  clearRetry(entry);
  if (entry.flushTimer !== null) clearTimeout(entry.flushTimer);
  entry.flushTimer = null;
  entry.pending = [];
  entry.pendingBytes = 0;
  entry.connectVersion++;
  entry.opening = false;
  entry.ladderStartedAt = 0;
  const ws = entry.ws;
  entry.ws = null;
  // Browsers only permit application close codes 3000–4999 (or normal closure).
  ws?.close(4008, "terminal output exceeded the display buffer");
  publish(entry, {
    phase: "failed", attempts: entry.attempts, ladderStartedAt: 0,
    reason: entry.terminalExit
      ? "Terminal display paused because output arrived too quickly. The terminal process has ended; earlier output may be missing."
      : "Terminal display paused because output arrived too quickly. The agent continues. Retry attach restores recent output only; earlier output may be missing.",
  });
}

function flush(entry: Entry): void {
  entry.flushTimer = null;
  if (entry.destroyed || entry.overloaded || entry.writingBytes > 0 || entry.pending.length === 0) return;
  const chunk = concatChunks(entry.pending, entry.pendingBytes);
  entry.pending = [];
  entry.pendingBytes = 0;
  entry.writingBytes = chunk.length;

  // Measured BEFORE the write, because the write itself moves baseY (§7.2, known defect 5).
  // Scrolled from the write callback rather than inline, because term.write is asynchronous:
  // xterm parses on its own schedule, and scrolling before the parse lands leaves the
  // viewport one chunk short.
  const buffer = entry.term.buffer.active;
  const pinned = buffer.viewportY >= buffer.baseY;
  entry.term.write(chunk, () => {
    entry.writingBytes = 0;
    if (entry.destroyed) return;
    if (pinned) entry.term.scrollToBottom();
    scheduleFlush(entry);
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
  if (entry.destroyed || entry.overloaded || !entry.permitted) return;
  if (entry.terminalExit) {
    publishTerminalExit(entry);
    return;
  }
  if (!entry.connectPermitted) {
    publish(entry, { phase: "detached", attempts: 0, ladderStartedAt: 0, reason: "Waiting for current project execution permission before reconnecting." });
    return;
  }

  // Amends §4.6: the ladder runs for starting, running AND idle, and stops only on exited.
  // An idle session is a live sandbox that has been quiet; stranding it detached is a pane
  // that can never come back, because the output that would revive it is the output we are
  // no longer attached to receive.
  if (entry.session.status === "exited") {
    entry.ladderStartedAt = 0; // the ladder stops here and the pane's elapsed stops with it
    publish(entry, {
      phase: "ended",
      attempts: entry.attempts,
      ladderStartedAt: 0,
      reason: exitedReason(entry.session),
    });
    return;
  }

  // A socket that opened cleared the clock, so this close begins a new ladder and gets the
  // full budget. Stamped here rather than only in connect(), because the budget test below
  // runs before the first connect of the new ladder.
  if (entry.ladderStartedAt === 0) entry.ladderStartedAt = Date.now();

  if (Date.now() - entry.ladderStartedAt >= RETRY_BUDGET_MS) {
    // `giveUp` is async and nothing awaits it, so a listener that throws inside one of its
    // publishes would land as an unhandled rejection. Widening giveUp's own try is the wrong
    // fix: that catch means "the diagnostic fetch failed" and feeds `reasonForError`, so a
    // consumer's bug would be rendered to the user as an attach failure reason — a lie in the
    // one sentence this whole path exists to get right. Rethrowing on a macrotask instead makes
    // this behave like every SYNCHRONOUS publish in this file, where a throwing listener leaves
    // the event handler as an uncaught error the platform reports. React's useSyncExternalStore
    // callback must not throw; if one does it is a consumer defect and it should be loud.
    void giveUp(entry).catch((error: unknown) => {
      setTimeout(() => {
        throw error;
      });
    });
    return;
  }

  // Rung = attempts - 1, FLOORED AT ZERO. `attempts` is 0 here whenever the socket had opened
  // (onopen reset it), and `RETRY_DELAYS_MS[-1]` is `undefined`, which setTimeout reads as 0 —
  // a live session that drops would reconnect in a tight loop instead of after §4.6's 250 ms.
  const rung = Math.min(
    Math.max(entry.attempts - 1, 0),
    RETRY_DELAYS_MS.length - 1,
  );
  const delay = RETRY_DELAYS_MS[rung];
  publish(entry, {
    phase: "reconnecting",
    attempts: entry.attempts,
    ladderStartedAt: entry.ladderStartedAt,
    reason: "",
  });
  clearRetry(entry);
  entry.retryTimer = setTimeout(() => {
    entry.retryTimer = null;
    connect(entry);
  }, delay);
}

/** The ladder is spent. One read of the record, so the sentence the pane renders is the truth
 *  rather than the likeliest-looking code — see the note above this step. */
async function giveUp(entry: Entry): Promise<void> {
  const version = entry.connectVersion;
  entry.ladderStartedAt = 0; // the ladder is spent: there is no elapsed left to render
  publish(entry, {
    phase: "failed",
    attempts: entry.attempts,
    ladderStartedAt: 0,
    reason: "the sandbox did not accept a connection within 60 seconds",
  });

  let fresh: Session;
  try {
    fresh = await deps.client.getSession(entry.id);
  } catch (error) {
    if (entry.destroyed || !entry.permitted || version !== entry.connectVersion) return;
    publish(entry, {
      phase: "failed",
      attempts: entry.attempts,
      ladderStartedAt: 0,
      reason: reasonForError(error),
    });
    return;
  }
  if (entry.destroyed || !entry.permitted || version !== entry.connectVersion) return;

  entry.session = fresh;
  if (fresh.status === "exited") {
    publish(entry, {
      phase: "ended",
      attempts: entry.attempts,
      ladderStartedAt: 0,
      reason: exitedReason(fresh),
    });
    return;
  }
  publish(entry, {
    phase: "failed",
    attempts: entry.attempts,
    ladderStartedAt: 0,
    reason: `the sandbox is ${fresh.status} but refused the attach connection`,
  });
}

function reasonForError(error: unknown): string {
  if (!(error instanceof ApiError))
    return "the control plane could not be reached";
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
  if (entry.flushTimer !== null) {
    clearTimeout(entry.flushTimer);
    entry.flushTimer = null;
  }
  if (entry.graceTimer !== null) {
    clearTimeout(entry.graceTimer);
    entry.graceTimer = null;
  }
  for (const disposable of entry.disposables) disposable.dispose();
  entry.disposables = [];
  // ── LOAD-BEARING ORDERING. Read this before reordering the two lines below. ──────────
  // `entry.ws` is nulled HERE, before the close and before any later event can arrive. Four
  // guards elsewhere in this file depend on that being true, and they depend on it in a way
  // that does not show up in the test suite:
  //
  //     onopen      `if (entry.ws !== ws || entry.destroyed) return;`
  //     onmessage   `if (entry.ws !== ws || entry.destroyed) return;`
  //     onclose     `if (entry.ws !== ws) return;` … `if (entry.destroyed) return;`
  //     scheduleRetry  `if (entry.destroyed) return;`
  //
  // Because this line runs first, a socket event arriving after teardown always fails the
  // STALE-SOCKET half (`entry.ws` is null, so it can never equal the socket the handler closed
  // over) and returns before the `entry.destroyed` half is ever the deciding condition. The
  // stale-socket halves are tested; the `destroyed` halves are not, and four mutation survivors
  // are declared on exactly this basis — they are masked, not covered.
  //
  // **What would have to change for that declaration to expire:** any path that calls
  // `destroy()` while leaving a live `entry.ws` in place — nulling after the close instead of
  // before, dropping the null, or a second teardown route that skips it. The moment that
  // happens the four `destroyed` checks become live, reachable, UNTESTED branches, and the
  // failure they would then be the only thing preventing is a destroyed entry re-entering the
  // retry ladder: a socket reopened for a pane that is gone, on a terminal already disposed.
  // If you reorder this, write the four negative cases first.
  const ws = entry.ws;
  entry.ws = null;
  entry.opening = false;
  entry.connectVersion += 1;
  // Closing the socket detaches without stopping the session. Its record remains;
  // a future attach can replay recent output, but this view's scrollback is discarded.
  if (ws) ws.close(1000, "pane closed");
  entry.term.dispose();
  entry.pending = [];
  entry.pendingBytes = 0;
  entry.writingBytes = 0;
  entry.element.remove();
  entries.delete(entry.id);
  idleEntries.delete(entry.id);
  notify(entry.id, DETACHED);
}
