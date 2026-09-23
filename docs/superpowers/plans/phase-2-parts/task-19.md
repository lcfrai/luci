### Task 19: The terminal binding and the attach registry

This is the last task of Group D's foundation and the only one that owns a live socket. It exists because of one sentence in contract §6.6: `Terminal` takes **a session id, not a URL**. A URL prop makes the socket a render-time concern — every re-render that changes the string re-opens it, every unmount closes it, and a pane cannot move from Columns to Canvas without the PTY dying in transit. A registry keyed by session id, ref-counted, living at module scope outside React, is what makes "disconnect does not destroy", "a store update does not dispose the terminal I am typing into" (known defect 1) and "move a pane between layouts" all true at once. The registry is ref-counted rather than boolean-owned because two layouts can legitimately show one session at the same moment, and because `React.StrictMode` mounts every effect twice in dev (§1.7): without ref-counting and a grace window the control plane's `attachCount` reads 2 for one pane on screen, and a naive teardown closes the socket that the second mount is about to need.

The two rules of contract §7.2 are hard and they both live here, in the message handler, not in a component — a component that re-renders on every frame of agent output is the defect they exist to prevent. **Batch to ~10 Hz:** stdout payloads accumulate and flush on a 100 ms timer, concatenated into exactly one `term.write` per flush, so a build log at 4 000 lines/second costs 10 writes a second and not 4 000; the timer is armed only when the buffer is non-empty, so an idle session costs nothing. **Never scroll-jump:** pinned-ness is measured *before* the write, because the write itself moves `baseY`, and `scrollToBottom()` is called only from the write callback and only when the pane was pinned. A pane the user scrolled up to read stays exactly where it is, forever, until the user scrolls back down. Known defect 5 is this and nothing else.

`readOnly` is `session.status === "exited"` and it means precisely one thing: `onData` is not forwarded. The scrollback stays rendered, scrollable and selectable, because **the transcript is the artifact** — a finished session is still worth reading, and a dropped WebSocket destroys nothing. Destruction happens on an explicit kill, on the idle TTL, or when the reaper collects an orphan; never because a pane closed and never because a socket dropped. That asymmetry is the whole reason the grace timer and the retry ladder exist.

Three things the UI contract predates, which override it here. **`GET /v1/harnesses` returns `[]`, never `null`** (Group B task 8 pinned it with a marshalled-bytes test), so nothing in this task writes a defensive `?? []`. **The control plane emits sixteen error codes, not the fifteen §4.2 lists**: Group C ruling 7 added `non_loopback_publish`, emitted by task 12's attach handler when the daemon reports a sandbox published off loopback, and that sandbox is *running* — so the failure reason this task renders must never say "not running" for it. **Session status is four values**, `starting · running · idle · exited`; `docs/AGENTIC-UX.md` §4 rule 8 lists seven and five of them need PTY-reading this phase forbids. There is no bearer token on this hop and there must not be one before Phase 8 (§4.5): the boundary is loopback bind + `Host` guard + `Origin` allowlist, all server-side, and the browser attaches `Origin` to the handshake by itself.

**A note on the expected-output blocks in this file, read it before you judge one.** `desktop/node_modules` contains **no vitest** at the time of writing — task 14 installs `vitest@5.0.1` and `jsdom@30.1.0` — so no *test* command in this plan has been run and **no reporter transcript here is transcribed from a real run**. Rather than invent one, every verification step states *what you must see* (the failing test's name, the assertion, the reason) and marks the surrounding reporter chrome as approximate. If the chrome differs from what is printed below, the chrome is wrong and the named assertion is what matters; do not "fix" a test to match a transcript in a document. **The one exception is the `tsc` block in step 11**, which is compiler output and not a reporter transcript: `typescript@6.0.3` *is* installed today, so that one was produced by running the compiler, and step 11 says so where it prints it.

**Files:**
- `desktop/src/test/fakes.ts` — **task 14's file.** Group D shared shape §3.3 names task 19 as the heavy user of `FakeTerm` and `FakeWebSocket` and says that if either must grow a capability, task 19 grows *task 14's copy* and says so. Step 1 does exactly that, and only to `FakeTerm`: a write callback and a moving `baseY` (without which the pinned-scroll ordering cannot be proven at all), recorded resize handlers, a focus counter, and the `openedIn` / `addons` records. **`FakeWebSocket` is not touched** — every member this task drives already exists in task 14's class, and replacing it would orphan that file's non-exported `Listener` and `toArrayBuffer` and break its own typecheck (modify)
- `desktop/src/api/attachRegistry.ts` — ref-counted sockets and xterm instances keyed by session id, the 100 ms flush, the pinned-scroll rule, the retry ladder, the attach state and its React binding (new)
- `desktop/src/api/attachRegistry.test.ts` — lifecycle, ref-counting, StrictMode's double mount, batching, both halves of defect 5, stdin gating, the ladder, and the give-up reason (new)
- `desktop/src/api/poller.ts` — the one interval for the whole app: `startPolling`, `POLL_INTERVAL_MS`, `coveredWorktreeIds` (new)
- `desktop/src/api/poller.test.ts` — one request per tick, the visibility pause, the in-flight guard, error reporting (new)
- `desktop/src/components/Terminal.tsx` — rewritten against the new signature: constructs the `XTerm`, owns the fit addon and the xterm import, hands the instance to the registry, and owns DOM focus inside the pane (modify)
- `desktop/src/components/Terminal.test.tsx` — the wiring, under `<StrictMode>`, with `vi.mock("@xterm/xterm")` (new)
- `desktop/src/App.tsx` — **not in §9's file list for this task, and the departure is argued in step 12.** Phase 1's `App.tsx` renders `<Terminal url={url}/>`; the contract-mandated signature change makes that line a type error, and this task is required to leave `npm run typecheck` green. The edit is the smallest one that compiles and it reserves the insertion point task 24 fills (modify)

**Interfaces:**
- Produces:
  - `attachRegistry.acquireAttach(session, { createTerm, readOnly }) → AttachHandle` — first caller constructs, every later caller shares; clears any pending teardown
  - `attachRegistry.releaseAttach(sessionId)` — decrements; teardown only at zero refs *and* after `GRACE_MS`
  - `attachRegistry.updateAttachSession(session)` and `setAttachReadOnly(sessionId, readOnly)` — the two facts the registry cannot derive
  - `attachRegistry.getAttachState(sessionId) → AttachState`, `subscribeAttach(sessionId, listener) → () => void`, `useAttachState(sessionId) → AttachState` — **the footer's "attach state" (§6.5) reads this**, not the store
  - `attachRegistry.TermLike`, `AttachState`, `AttachPhase`, `AttachHandle`, `FLUSH_INTERVAL_MS`, `GRACE_MS`, `RETRY_DELAYS_MS`, `RETRY_BUDGET_MS`, `configureAttachRegistry`, `__resetAttachRegistry`
  - `poller.startPolling(client, store) → stop()`, `poller.POLL_INTERVAL_MS`, `poller.coveredWorktreeIds(indexed, sessions)` — *amended 2026-09-23 by task 24: `startPolling(client, store, onGoodTick?)`, and a good tick clears only the error the poller raised (UI contract §4.7, "Two amendments")*
  - `components/Terminal.Terminal` — `{ sessionId, readOnly }`, `memo`'d on those two props
- Consumes: task 15's `Session`/`SessionId` (`store/types.ts`) and its `aSession` fixture builder from `src/test/fixtures.ts` — **task 15's, per shared shape §3.1**, which moved the six typed builders out of task 17 because task 16 runs first and needs them; task 16's `CockpitClient.openAttach`, `CockpitClient.getSession`, `ApiError`, `ProblemCode`, and the `api` singleton; task 17's `useCockpit` and `selectSession`; the pre-existing `lib/wire.ts` (`Kind`, `decode`, `encode`, `encodeResize`), unchanged; task 14's `fakes.ts` — `FakeWebSocket` as shipped, `FakeTerm` grown in step 1. This task declares **no** fixture builder of its own; every variant is an override object passed to `aSession`.
- Required by: task 20 (`SessionPane` renders `<Terminal sessionId readOnly={status === "exited"}/>` and `SessionPaneFooter` renders `useAttachState`), task 21 (a tile carries the same session across a layout change and must not drop the PTY), task 24 (`App.tsx` mounts `AppShell` and calls `startPolling`).
- Amends contract §4.6, twice. (1) `idle` joins `starting` and `running` as a status the ladder retries — argued in step 6. (2) The give-up reason is read from a diagnostic `GET /v1/sessions/{id}` rather than assumed to be `sandbox_not_running`: a refused WebSocket upgrade exposes neither status nor body to JavaScript, and the plausible guess is the one sentence Group C's ruling 7 forbids for `non_loopback_publish` — argued in step 7.
- Adds to contract §6.6: `Terminal` also passes a `createTerm` factory to the registry, so the xterm import stays in the component (§1.5) while the *instance* lives in the registry (§1.7). Neither section describes the seam; step 11 does.

- [ ] **Step 1: Preflight — confirm the facts this task is built on, then grow task 14's FakeTerm**

Everything below must already be true before a line of this task is written, and all of it is cheap to check. If any of it is false, **stop and report it to the orchestrator rather than patching another task's file** — tasks 14, 15 and 16 own those files and a silent local patch is how Group C produced nine Criticals.

Every guard below is **anchored** (shared-shape §3A). An unanchored `grep -c` for a name that the
file also mentions in a doc comment passes against code that does not have the thing — which is
the failure mode §3A was written to stop, and (a) is a measured instance of it.

```bash
cd desktop
# (a) Task 16's ProblemCode must carry the sixteenth code. Group C ruling 7 added
#     non_loopback_publish; the registry switches on it in step 7 and a missing member is a
#     compile error, not a runtime surprise.
#     ANCHORED, and this is not pedantry: the doc comment task 16 writes above the union names
#     this code twice in prose, so the unanchored form reads 3 today and still reads 2 with the
#     member deleted — it passes against a union that does not have it. Anchor on the `|` that
#     begins a union line, because a comment line begins with `*` and cannot match; but let the
#     member sit ANYWHERE on that line, because task 16 packs several members onto one line and
#     a pattern demanding `| "non_loopback_publish"` at the START of its own line reads 1 today
#     and 0 the moment that union is reflowed. Measured against task 16's text: 1 as written, 1
#     reflowed with the member packed beside a neighbour, 0 with the member deleted.
grep -cE '^[[:space:]]*\|.*"non_loopback_publish"' src/api/client.ts   # expect 1

# (b) Task 14's fakes must exist, and FakeTerm must be the contract §1.5 class, not a stub.
grep -cE '^export class FakeTerm' src/test/fakes.ts        # expect 1
grep -cE '^export class FakeWebSocket' src/test/fakes.ts   # expect 1

# (c) Task 15's typed fixture builders must exist. Shared shape §3.1: task 14 creates
#     fixtures.ts with the untyped constants (FIXED_NOW, IDS) and reserves an insertion point;
#     TASK 15 fills it with the six builders, because it is the first task that can type them
#     and the last one that can do it before task 16 needs them. `aSession` is the only one
#     this task uses. Anchored, so task 14's reserving comment — which names the builders in
#     prose — cannot satisfy the guard.
grep -cE '^export function aSession' src/test/fixtures.ts   # expect 1
```

Now grow **task 14's** `desktop/src/test/fakes.ts`. Shared-shape §3.3 is explicit that `FakeTerm` and `FakeWebSocket` are task 14's, that neither is redefined anywhere, and that task 19 — their heavy user — **grows** task 14's copy rather than replacing it. A replacement is not a stylistic difference: task 14's file holds the non-exported `type Listener` and `function toArrayBuffer`, both used only by its `FakeWebSocket`, so swapping that class out orphans them and the file stops compiling under the repo's `noUnusedLocals` with `TS6196`/`TS6133`.

**`FakeWebSocket` needs nothing from this task.** Every member this task drives already exists in task 14's class, and the two suites below use task 14's names. The mapping, so no one re-invents one:

| This task needs | Task 14's member | Note |
|---|---|---|
| open the socket | `acceptOpen()` | sets `readyState = OPEN`, then fires `onopen` |
| deliver one wire frame | `emitBinary(bytes)` | hands `onmessage` a standalone `ArrayBuffer`, which is what `binaryType = "arraybuffer"` yields |
| an abnormal close | `emitClose(1006)` | the code is passed explicitly: task 14's default is 1000, and a refused handshake is 1006 |
| the newest socket | `FakeWebSocket.last()` | |
| did the registry close it | `closedWith` | `null` until `close()` is called; a **recording** close that does not fire `onclose`, which is what the browser does |
| the bytes of a sent frame | `sent[i]` | typed `string \| ArrayBufferLike \| ArrayBufferView`; step 2 adds a local narrowing helper rather than widening the shared fake |

One behaviour of task 14's class to keep in mind while reading the registry: its `send()` **throws** unless `readyState === OPEN`. The registry guards every send on exactly that, so the throw is a tripwire for a regression rather than something to work around.

So only `FakeTerm` grows. Replace task 14's `FakeTerm` with the class below — a superset of its **members**, every original one kept and every addition marked — and leave the rest of the file, `FakeWebSocket` included, untouched. It is deliberately **not** a superset of its signatures: task 14 declares `loadAddon()`, `open()` and `onResize()` with no parameters, and each one here gains the argument it has to record. Nothing in tasks 14 to 24 calls those three bare — the component passes the fit addon, the registry passes the host element and the resize callback — so no call site that compiled before stops compiling.

```ts
/* ─────────────────────────────────────────────────────────────────────────────────────
 * FakeTerm — grown by task 19 (shared-shape §3.3). The additions are marked; nothing in
 * contract §1.5's original class was removed, and FakeWebSocket below is not touched.
 * ───────────────────────────────────────────────────────────────────────────────────── */

/** Records what the component and the registry do to a terminal without loading the real one. */
export class FakeTerm {
  cols = 80;
  rows = 24;
  written: string[] = [];
  disposed = false;
  scrolledToBottom = 0;
  /** task 19: Terminal.tsx must hand DOM focus to the pane, and that has to be assertable. */
  focused = 0;
  addons: unknown[] = [];
  openedIn: HTMLElement | null = null;
  buffer = { active: { viewportY: 0, baseY: 0 } };
  private dataHandlers: ((d: string) => void)[] = [];
  private resizeHandlers: ((size: { cols: number; rows: number }) => void)[] = [];

  loadAddon(addon: unknown) { this.addons.push(addon); }
  open(parent: HTMLElement) { this.openedIn = parent; }
  focus() { this.focused += 1; }

  /** task 19 grew this, and both changes are load-bearing:
   *  (a) the callback is INVOKED — the registry's pinned-scroll rule lives inside it, so a
   *      fake that swallows the callback makes known defect 5 untestable;
   *  (b) baseY advances by the number of newlines written while viewportY does not, which is
   *      what a real write does. It is the only way a test can tell "pinned was measured
   *      BEFORE the write" from "measured after": measured after, a pane that was pinned
   *      reads viewportY < baseY and never scrolls. */
  write(data: string | Uint8Array, callback?: () => void) {
    const text = typeof data === "string" ? data : new TextDecoder().decode(data);
    this.written.push(text);
    this.buffer.active.baseY += text.split("\n").length - 1;
    callback?.();
  }

  /** Re-pins the viewport, as xterm does. */
  scrollToBottom() {
    this.scrolledToBottom += 1;
    this.buffer.active.viewportY = this.buffer.active.baseY;
  }

  onData(cb: (d: string) => void) {
    this.dataHandlers.push(cb);
    return { dispose: () => { this.dataHandlers = this.dataHandlers.filter((h) => h !== cb); } };
  }
  /** task 19: the registry forwards a resize frame from here, so the handler must be recorded. */
  onResize(cb: (size: { cols: number; rows: number }) => void) {
    this.resizeHandlers.push(cb);
    return { dispose: () => { this.resizeHandlers = this.resizeHandlers.filter((h) => h !== cb); } };
  }
  dispose() { this.disposed = true; }

  /** test helper: simulate the user typing */
  type(s: string) { this.dataHandlers.forEach((h) => h(s)); }
  /** test helper: simulate the fit addon resizing the terminal (task 19) */
  resize(cols: number, rows: number) {
    this.cols = cols;
    this.rows = rows;
    this.resizeHandlers.forEach((h) => h({ cols, rows }));
  }
  /** test helper: simulate the user scrolling up N lines to read (task 19) */
  scrollUp(lines: number) {
    this.buffer.active.viewportY = Math.max(0, this.buffer.active.baseY - lines);
  }
}
```

Then confirm the tree still compiles — nothing consumes the new members yet, so this must be green on its own:

```bash
cd desktop
npm run typecheck
```

You must see it exit 0 with no diagnostics. If `tsc` complains about `this.dataHandlers` in the `dispose` closure, the class body was transcribed with `readonly` on the handler arrays; they are reassigned by `dispose`, so they are plain fields. If it reports `TS6196: 'Listener' is declared but never used` or `TS6133: 'toArrayBuffer' is declared but never used`, `FakeWebSocket` was replaced rather than left alone — those two are its private helpers, and this step must not have touched it.

- [ ] **Step 2: Write the failing tests for the registry's lifecycle, ref-counting and input path**

These are the tests that pin the registry's *reason for existing*: one socket per session however many panes show it, a teardown that survives StrictMode's double mount, a store update that does not dispose the terminal (known defect 1), and stdin that is wired for a live session and silent for an exited one (§6.6).

The file injects a two-method client stub rather than using a shared fake. That stub lives here and not in task 14's `fakes.ts` deliberately: only this task needs `{ openAttach, getSession }`, and a fake in the shared file is a fake every later task carries. The test relies on **task 15's** `aSession()` builder accepting an override object — shared shape §3.2's "a task needing a variant passes an override object; it does not add a near-duplicate builder". `aSession` is one of the six sanctioned names and this task declares none of its own; step 1's guard (c) is what makes the import's existence a checked fact rather than an assumption.

Timers are vitest's fake timers rather than an injected clock. One fewer seam, and the code under test then calls the same `setTimeout` production calls; vitest's fake timers also fake `Date.now`, which the retry budget reads.

Create `desktop/src/api/attachRegistry.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  GRACE_MS,
  __resetAttachRegistry,
  acquireAttach,
  configureAttachRegistry,
  getAttachState,
  releaseAttach,
  setAttachReadOnly,
  updateAttachSession,
} from "./attachRegistry";
import type { TermLike } from "./attachRegistry";
import { FakeTerm, FakeWebSocket } from "../test/fakes";
import { aSession } from "../test/fixtures";
import { Kind, decode } from "../lib/wire";
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
```

```ts
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
});
```

Run them. The module does not exist yet, so this is a collection failure, not an assertion failure:

```bash
cd desktop
npm test -- src/api/attachRegistry.test.ts
```

**What you must see:** the run fails at import time, naming `src/api/attachRegistry.test.ts` and the unresolved import `./attachRegistry`, with a message of the form `Failed to load url ./attachRegistry … Does the file exist?`, and **zero tests collected**. The reporter chrome around that line is approximate here — vitest 5 is not installed in this tree yet, so nothing in this block was transcribed from a run. What is not negotiable is that the failure is the missing module and not a passing suite.

- [ ] **Step 3: Implement the registry's lifecycle, transport and input path**

This is the module's first half: the entry table, ref-counting, the grace window, the socket wiring and the state feed. Batching (step 5) and the retry ladder (step 7) are added on top of it, and the two `TODO(step N)` markers below say exactly where. Step 7 also adds two imports at the top of the file; beyond those, and the marker each step replaces, neither one touches this module.

Decisions the contract does not make, each with its reason, because a later reader will otherwise re-derive them wrongly:

**(a) The registry never imports xterm.** `TermLike` is a structural interface, and the instance arrives through the `createTerm` factory the component passes. Contract §1.5 puts the xterm import in `Terminal.tsx` and §1.7 puts the instance in the registry; a factory satisfies both, and it is what lets `attachRegistry.test.ts` run with no `vi.mock` at all.

**(b) The registry owns a detached `<div>` per session and the component appends it.** The alternative — re-calling `term.open()` against each new host node — is not supported by xterm across parents, and re-parenting one element is what actually preserves the rendered scrollback when a pane moves from Columns to Canvas. The element carries `data-session-id` and inline sizing rather than a class: `develop.css`'s prefixes are owned by tasks 20 and 21 (shared-shape §2) and a mount point is not a themeable surface.

**(c) `mount()` is separate from `acquireAttach()`.** xterm measures its cell size from a *rendered* element; `open()` on a detached node yields a zero-column terminal. So the component appends first and calls `mount()` second, and `mount()` is idempotent.

**(d) `getAttachState` returns one frozen `DETACHED` constant for an unknown id.** `useAttachState` is a `useSyncExternalStore` and a snapshot getter that allocates a fresh object on every call re-renders forever. This is the single most likely way to hang the cockpit and it costs one `Object.freeze` to prevent.

**(e) A transient reconnect writes nothing into the terminal.** Phase 1's component wrote `[disconnected]` on every close; on a ladder that reconnects, that text accumulates inside the artifact. The honest channel for "reconnecting" is the attach state the footer renders, so the only synthetic writes are the end-of-session ones, which are transcript-worthy.

**(f) `WS_OPEN` is a local constant, not `WebSocket.OPEN`.** The registry must not touch a global the fake does not own; the value is fixed at 1 by the HTML spec.

Create `desktop/src/api/attachRegistry.ts`:

```ts
// Sockets and xterm instances, keyed by session id, ref-counted, at module scope — NOT inside
// a React effect (contract §1.7). A dropped WebSocket destroys nothing: destruction is an
// explicit kill, the idle TTL, or the reaper collecting an orphan.

import { useCallback, useSyncExternalStore } from "react";
import { Kind, decode, encode, encodeResize } from "../lib/wire";
import type { CockpitClient } from "./client";
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
```

```ts
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
```

```ts
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

/** TODO(step 5): batching and the pinned-scroll rule replace this straight-through write. */
function writeOut(entry: Entry, bytes: Uint8Array): void {
  entry.term.write(bytes);
}

/** TODO(step 7): the retry ladder. Until step 7, a closed socket stays closed. */
function scheduleRetry(entry: Entry): void {
  publish(entry, { phase: "failed", attempts: entry.attempts, reason: "the attach socket closed" });
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
```

Run the suite and the typechecker:

```bash
cd desktop
npm test -- src/api/attachRegistry.test.ts
npm run typecheck
```

**What you must see:** all ten tests in the two `describe` blocks pass, and `tsc` exits 0. If `tears down GRACE_MS after the last holder releases` fails on its first assertion — `expect(socket().closedWith).toBeNull()`, printing roughly `expected { code: 1000, reason: 'pane closed' } to be null` — the teardown ran on `releaseAttach` instead of on the timer. If `a store update does not dispose the terminal` fails, `updateAttachSession` is re-entering `connect`.

- [ ] **Step 4: Write the failing tests for 10 Hz batching and the pinned-scroll rule**

Known defect 5 and contract §7.2. Two of these are the regression tests §8 names by title, and the wording is taken from there so a grep for the defect finds them: *"output does not scroll a pane the user has scrolled up"* and *"output does scroll a pane pinned to the bottom"*.

Extend the import lists at the top of `desktop/src/api/attachRegistry.test.ts` — `FLUSH_INTERVAL_MS` from `./attachRegistry`, and `encode` from `../lib/wire` — then append:

```ts
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
});
```

```bash
cd desktop
npm test -- src/api/attachRegistry.test.ts
```

**What you must see:** five of the six new tests fail, and the ten from step 3 still pass. The exception is *arms no timer while nothing is buffered*: it emits nothing at all, so step 3's straight-through `writeOut` leaves `term.written` empty and its assertion already holds — that test is green here and must still be green after step 5. The other five are assertion failures, not import errors. Step 3's `writeOut` writes straight through and never consults the buffer, so the first failing assertion in *three payloads inside one window* is the pre-flush one — `AssertionError: expected [ 'one ', 'two ', 'three' ] to deeply equal []` — and both scroll tests fail on `expected 0 to be 1` for `scrolledToBottom`, because nothing calls `scrollToBottom` yet. Reporter chrome around those lines is approximate; the assertion text is the part that matters.

- [ ] **Step 5: Implement the flush buffer and the pinned-scroll rule**

Replace step 3's `TODO(step 5)` `writeOut` in `desktop/src/api/attachRegistry.ts` with these three functions. Nothing else in the file changes.

```ts
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
```

```bash
cd desktop
npm test -- src/api/attachRegistry.test.ts
npm run typecheck
```

**What you must see:** sixteen passing tests and a clean `tsc`. If *keeps a multibyte character split across two frames intact* fails with `expected [ '��' ] to deeply equal [ '→' ]`, the implementation decoded each payload to a string before buffering instead of concatenating bytes.

- [ ] **Step 6: Write the failing tests for the retry ladder and the give-up reason**

Contract §4.6 fixes the ladder: 250 ms, 500 ms, 1 s, 2 s, 2 s … capped at 2 s, give up after 60 s total, do not retry a session that reads `exited`, and **do** retry a socket that opened and then closed while the session is still live — that is a control-plane restart, and the user should watch the pane reconnect rather than stare at a dead terminal.

**One departure from §4.6, and it is deliberate.** That section says "retry only while the session's status is `starting` or `running`". Taken literally it strands `idle`: an idle session is a *live sandbox that has simply been quiet*, its status is derived from a lastOutput timestamp, and refusing to reconnect it would leave a perfectly healthy agent unreachable until something else moved it back to `running` — which nothing can, because the only thing that would is output the cockpit is no longer attached to receive. The rule's intent is its complement, stated in the same paragraph: *"A session that reads `exited` is not retried."* So the ladder runs for `starting`, `running` and `idle`, and stops on `exited`. This is recorded in the Interfaces block above as an amendment to §4.6.

Add `RETRY_BUDGET_MS` to the `./attachRegistry` import list and `ApiError` from `./client`, then append:

```ts
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
});
```

```bash
cd desktop
npm test -- src/api/attachRegistry.test.ts
```

**What you must see:** the sixteen earlier tests pass and all seven of these fail, because step 3's placeholder `scheduleRetry` publishes `failed` and schedules nothing. In *retries a refused handshake* and in *reconnects a socket that opened and then closed* the first failing assertion is the same phase check — `AssertionError: expected 'failed' to be 'reconnecting'`; in *reconnects an idle session* it is `expected 1 to be 2`; and the rest fail on the phase or on a `reason` that still reads `"the attach socket closed"`.

- [ ] **Step 7: Implement the ladder, and make the give-up reason honest**

**The thing to understand before writing this, because it is the override the task text calls out.** When a handshake is refused, JavaScript learns *nothing* about why: the WebSocket API exposes neither the HTTP status nor the body, and a rejected upgrade arrives as `onerror` followed by `onclose` with code 1006. Task 12's attach handler answers `409 sandbox_not_running` or `500 non_loopback_publish` with a perfectly good JSON body, and the browser throws it away. So the registry must not *guess* a code — and guessing is exactly what would produce the forbidden sentence, because `sandbox_not_running` is the plausible guess and `non_loopback_publish` means the sandbox **is** running.

What it does instead: when the 60 s budget is spent, it makes one diagnostic `GET /v1/sessions/{id}` — an ordinary §4.4 method, no new client surface — and phrases the reason from the record. `exited` gives the `exitReason` verbatim; anything else says the sandbox *is* in that status and refused the connection, which is true for the `non_loopback_publish` case and never says "not running" about a running sandbox. `non_loopback_publish` still gets its own branch in `reasonForCode` below, with the sentence Group C's ruling 7 requires, so that the day a later phase gives the client a way to read the refusal directly the wording is already right. That switch is deliberately **not** exhaustive over the eighteen-member union — the ten codes it does not name render the server's own message, which is written for a human — and its doc comment says so.

Add `ApiError` (a value import) and `ProblemCode` (a type import) from `./client` at the top of `desktop/src/api/attachRegistry.ts`, then replace the `TODO(step 7)` `scheduleRetry` with:

```ts
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
```

```bash
cd desktop
npm test -- src/api/attachRegistry.test.ts
npm run typecheck
```

**What you must see:** twenty-three passing tests and a clean `tsc`. A `tsc` error of the form `Type '"non_loopback_publish"' is not comparable to type 'ProblemCode'` means step 1's preflight (a) was skipped and task 16's union is still missing the sixteenth server code — report it, do not edit `client.ts`.

- [ ] **Step 8: Write the failing tests for the poller**

Contract §4.7: one interval for the whole app, one request per tick (`GET /v1/sessions` unfiltered — never one request per worktree), paused while the document is hidden, and **the poll must move zero pixels**. The store side of "zero pixels" is task 17's `replaceSessions` preserving index position; the poller's side is issuing the request that feeds it and writing nothing else.

The client stub is a cast rather than a full `CockpitClient` implementation: the poller uses exactly one method, and an object literal spelling out §4.4's whole surface is a second copy of task 16's interface, free to drift from it without anyone noticing.

Create `desktop/src/api/poller.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { POLL_INTERVAL_MS, coveredWorktreeIds, startPolling } from "./poller";
import { ApiError } from "./client";
import type { CockpitClient } from "./client";
import { resetCockpit, useCockpit } from "../store/store";
import { aSession } from "../test/fixtures";
import type { Session } from "../store/types";

/** The poller calls exactly one method; the cast keeps this stub from drifting from §4.4. */
const stubClient = (listSessions: () => Promise<Session[]>): CockpitClient =>
  ({ listSessions }) as unknown as CockpitClient;

function setVisibility(value: "visible" | "hidden"): void {
  Object.defineProperty(document, "visibilityState", { configurable: true, get: () => value });
  document.dispatchEvent(new Event("visibilitychange"));
}

beforeEach(() => {
  vi.useFakeTimers();
  resetCockpit();
  setVisibility("visible");
});

afterEach(() => {
  setVisibility("visible");
  vi.useRealTimers();
});

describe("startPolling", () => {
  it("fetches once immediately and then on every interval", async () => {
    let calls = 0;
    const stop = startPolling(stubClient(async () => { calls += 1; return []; }), useCockpit);
    expect(calls).toBe(1);                       // the first render does not wait 2 s
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    expect(calls).toBe(2);
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 3);
    expect(calls).toBe(5);
    stop();
  });

  it("writes the fetched sessions into the store", async () => {
    const fresh = aSession({ id: "s-1", worktreeId: "w-1" });
    const stop = startPolling(stubClient(async () => [fresh]), useCockpit);
    await vi.advanceTimersByTimeAsync(0);
    const state = useCockpit.getState();
    expect(state.sessions["s-1"]).toBeDefined();
    expect(state.sessionIdsByWorktree["w-1"]).toEqual(["s-1"]);
    stop();
  });

  it("covers a worktree the store knows about but the response no longer mentions", async () => {
    useCockpit.setState({
      sessions: { "s-old": aSession({ id: "s-old", worktreeId: "w-old" }) },
      sessionIdsByWorktree: { "w-old": ["s-old"] },
    });
    const stop = startPolling(stubClient(async () => []), useCockpit);
    await vi.advanceTimersByTimeAsync(0);
    expect(useCockpit.getState().sessions["s-old"]).toBeUndefined();
    stop();
  });

  it("pauses while the document is hidden and catches up when it is shown", async () => {
    let calls = 0;
    const stop = startPolling(stubClient(async () => { calls += 1; return []; }), useCockpit);
    expect(calls).toBe(1);
    setVisibility("hidden");
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 5);
    expect(calls).toBe(1);                       // a lid closed for eight hours queues nothing
    setVisibility("visible");
    await vi.advanceTimersByTimeAsync(0);
    expect(calls).toBe(2);
    stop();
  });

  it("does not stack requests when one tick outlives the interval", async () => {
    let calls = 0;
    let release: (sessions: Session[]) => void = () => {};
    const stop = startPolling(
      stubClient(() => { calls += 1; return new Promise<Session[]>((r) => { release = r; }); }),
      useCockpit,
    );
    expect(calls).toBe(1);
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 3);
    expect(calls).toBe(1);
    release([]);
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    expect(calls).toBe(2);
    stop();
  });

  it("records a failure and clears it on the next success", async () => {
    let failing = true;
    const stop = startPolling(
      stubClient(async () => {
        if (failing) throw new ApiError("network_unavailable", "fetch failed", 0);
        return [];
      }),
      useCockpit,
    );
    await vi.advanceTimersByTimeAsync(0);
    expect(useCockpit.getState().lastError).toEqual({
      code: "network_unavailable",
      message: "fetch failed",
    });
    failing = false;
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
    expect(useCockpit.getState().lastError).toBeNull();
    stop();
  });

  it("stops fetching after stop()", async () => {
    let calls = 0;
    const stop = startPolling(stubClient(async () => { calls += 1; return []; }), useCockpit);
    stop();
    await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 5);
    expect(calls).toBe(1);
  });
});

describe("coveredWorktreeIds", () => {
  it("unions what the response saw with what the store already indexes", () => {
    const ids = coveredWorktreeIds(["w-1", "w-2"], [aSession({ id: "s-1", worktreeId: "w-3" })]);
    expect(ids).toEqual(["w-1", "w-2", "w-3"]);
  });

  it("never repeats an id", () => {
    const ids = coveredWorktreeIds(["w-1"], [aSession({ id: "s-1", worktreeId: "w-1" })]);
    expect(ids).toEqual(["w-1"]);
  });
});
```

```bash
cd desktop
npm test -- src/api/poller.test.ts
```

**What you must see:** a collection failure naming `./poller` as unresolvable, and zero tests collected — the same shape as step 2.

- [ ] **Step 9: Implement the poller**

> **Amended 2026-09-23 by task 24, a declared departure into this file (UI contract §4.7, "Two
> amendments").** The block below is what task 19 shipped, and two of its lines no longer hold.
> `startPolling` takes a third, optional `onGoodTick`, called after every tick whose fetch
> succeeded. And `if (store.getState().lastError !== null) store.getState().setError(null);` is now
> `if (raised !== null && store.getState().lastError === raised) store.getState().setError(null);`,
> where `raised` is the error object this poller last wrote: a good tick clears the poller's own
> report and nothing else. The test "records a failure and clears it on the next success" still
> holds, because that failure is the poller's own. `desktop/src/api/poller.ts` is the reference.

Create `desktop/src/api/poller.ts`:

```ts
// The one interval for the whole app (§4.7). Started when AppShell mounts (task 24), stopped
// when it unmounts. There is no event stream in the Go contract — §10.5 asks for one — so
// Phase 2 polls, from exactly one place, with exactly one request per tick.

import { ApiError } from "./client";
import type { CockpitClient } from "./client";
import type { useCockpit } from "../store/store";
import type { Session, WorktreeId } from "../store/types";

export const POLL_INTERVAL_MS = 2_000;

/** The worktrees this unfiltered fetch is authoritative for: every one the response mentions,
 *  plus every one the store already indexes. The second half is what makes a worktree whose
 *  last session was purged go EMPTY rather than keep a stale row. */
export function coveredWorktreeIds(
  indexed: readonly WorktreeId[],
  sessions: readonly Session[],
): WorktreeId[] {
  const seen = new Set<WorktreeId>(indexed);
  for (const session of sessions) seen.add(session.worktreeId);
  return [...seen];
}

export function startPolling(client: CockpitClient, store: typeof useCockpit): () => void {
  let stopped = false;
  let inFlight = false;

  const tick = async (): Promise<void> => {
    // Paused while hidden: a laptop lid closed for eight hours must not wake to 14 000 queued
    // requests. In-flight guard: a slow tick delays the next one, it never stacks another.
    if (stopped || inFlight || document.visibilityState === "hidden") return;
    inFlight = true;
    try {
      const sessions = await client.listSessions();
      if (stopped) return;
      const state = store.getState();
      const covered = coveredWorktreeIds(Object.keys(state.sessionIdsByWorktree), sessions);
      state.replaceSessions(sessions, covered);
      // Only written when it actually changes: a store write every 2 s is a re-render every
      // 2 s, which is the layout shift AGENTIC-UX §10 rule 7 bans.
      if (store.getState().lastError !== null) store.getState().setError(null);
    } catch (error) {
      if (stopped) return;
      store.getState().setError(
        error instanceof ApiError
          ? { code: error.code, message: error.message }
          : { code: "internal", message: String(error) },
      );
    } finally {
      inFlight = false;
    }
  };

  const onVisibility = (): void => {
    if (document.visibilityState === "visible") void tick();
  };

  void tick();   // the first render does not wait POLL_INTERVAL_MS for its data
  const handle = setInterval(() => void tick(), POLL_INTERVAL_MS);
  document.addEventListener("visibilitychange", onVisibility);

  return () => {
    stopped = true;
    clearInterval(handle);
    document.removeEventListener("visibilitychange", onVisibility);
  };
}
```

```bash
cd desktop
npm test -- src/api/poller.test.ts
npm run typecheck
```

**What you must see:** nine passing tests and a clean `tsc`. If *covers a worktree the store knows about* fails, `coveredWorktreeIds` was given only the response's ids; if *does not stack requests* fails with `expected 4 to be 1`, the `inFlight` guard is missing or is reset before the await resolves.

- [ ] **Step 10: Write the failing test for `Terminal.tsx`'s new signature**

Contract §6.6 changes the prop from a URL to a session id, and §1.5 fixes how this component is tested: `vi.mock("@xterm/xterm")` against task 14's `FakeTerm`, asserting the wiring and never rendering a glyph. The two things this file must prove are the two the registry cannot prove on its own: **one `XTerm` per session id even under `StrictMode`** (defect 1, §1.7 — RTL does not enable StrictMode, so the test wraps it by hand), and that the component hands DOM focus into the pane (§6.8's ownership table).

Create `desktop/src/components/Terminal.test.tsx`:

```tsx
import { StrictMode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import { FakeTerm, FakeWebSocket } from "../test/fakes";
import { aSession } from "../test/fixtures";
import {
  __resetAttachRegistry,
  configureAttachRegistry,
  getAttachState,
} from "../api/attachRegistry";
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
```

Add `acquireAttach` and `releaseAttach` to the `../api/attachRegistry` import (and `AttachHandle` as a type import), then append the tests:

```tsx
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
});
```

```bash
cd desktop
npm run typecheck
npm test -- src/components/Terminal.test.tsx
```

**What you must see:** `tsc` rejects the new props against the Phase 1 signature, with a diagnostic of this shape —

```
src/components/Terminal.test.tsx(NN,NN): error TS2322: Type '{ sessionId: string; readOnly: boolean; }' is not assignable to type 'IntrinsicAttributes & { url: string; }'.
  Property 'sessionId' does not exist on type 'IntrinsicAttributes & { url: string; }'.
```

— and the test run fails too, because the Phase 1 component passes `undefined` to `new WebSocket(...)`. The position is printed as `NN`: this file does not exist yet, so both coordinates depend on how the two blocks above were transcribed, and the count of repeats will differ as well. The file name, the `TS2322`, and `sessionId` on the left of the assignability message are the parts that must be there.

- [ ] **Step 11: Rewrite `Terminal.tsx` against the session id**

The component keeps the jobs the contract leaves it and gives up the rest. It **owns** the xterm import and its options, the fit addon, and DOM focus inside the pane (§6.8). It **no longer owns** the socket, the instance's lifetime, or anything that happens to output — all three moved to the registry, which is the whole point of §6.6.

Details that are easy to get wrong:

**The effect's dependency list is `[sessionId, hasSession]` and nothing else.** Not `session`, not `readOnly`. A dep on the session object re-runs the effect on every 2 s poll, which releases and re-acquires the entry — the exact defect 1 shape, in the component instead of the registry. `readOnly` and the session snapshot are pushed into the registry by two separate one-line effects that acquire nothing. `hasSession` is there because the store may not have loaded the record on first render, and without it the effect would never run again once it had returned early.

**`memo` is on the component**, per §8's row 1, so a parent re-render with unchanged props does not even reach the effects.

**`scrollback: 10_000`.** §7.2: xterm's default is 1 000 lines, which makes scrolling up barely worth doing; the pinned rule only pays for itself if there is something up there to read.

**And the theme.** Phase 1's component hard-codes `#0a0908` / `#f2ede7`. Task 24's design-system audit greps the tree for literal hex and shared shape §3B bans a colour function outside the token layer, so the two colours xterm needs are resolved from the token layer instead — xterm cannot read a CSS variable itself. The names are `--page` (the window ground, `#0a0908`, the exact colour Phase 1 hard-codes) and `--ink` (`#f2ede7`, likewise); there is no `--surface-2` and no `--text` in task 14's file. Confirm both before transcribing, anchored on the declaration so a `var(--page)` reference elsewhere in the file cannot satisfy the guard:

```bash
cd desktop
grep -cE '^[[:space:]]*--page:' src/styles/tokens.css   # expect 1
grep -cE '^[[:space:]]*--ink:'  src/styles/tokens.css   # expect 1
```

If either reads 0, task 14 renamed a token: read `src/styles/tokens.css`, take the ground and
the primary ink from there, and **do not** fall back to a hex literal — that is the one thing
task 24's audit is built to catch.

Replace `desktop/src/components/Terminal.tsx` in full:

```tsx
import { memo, useEffect, useRef } from "react";
import { Terminal as XTerm } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import {
  acquireAttach,
  releaseAttach,
  setAttachReadOnly,
  updateAttachSession,
} from "../api/attachRegistry";
import type { TermLike } from "../api/attachRegistry";
import { selectSession } from "../store/selectors";
import { useCockpit } from "../store/store";
import type { SessionId } from "../store/types";

/** Checked against src/styles/tokens.css by the greps above; task 14 owns the names. */
const TERM_BG_TOKEN = "--page";
const TERM_FG_TOKEN = "--ink";

/** The fit addon belongs to the terminal, and the terminal outlives this component, so the
 *  pairing cannot live in a ref. Weak, so a disposed terminal takes its addon with it. */
const fitAddons = new WeakMap<TermLike, FitAddon>();

/** No literal hex in this file. If the tokens do not resolve — jsdom, where nothing is
 *  rendered anyway — no theme is passed and xterm keeps its own defaults. */
function themeFromTokens(): { background: string; foreground: string } | undefined {
  const styles = getComputedStyle(document.documentElement);
  const background = styles.getPropertyValue(TERM_BG_TOKEN).trim();
  const foreground = styles.getPropertyValue(TERM_FG_TOKEN).trim();
  if (background === "" || foreground === "") return undefined;
  return { background, foreground };
}

/** Called by the registry at most once per session id. */
function makeTerm(): TermLike {
  const term = new XTerm({
    fontFamily: '"Cascadia Mono", ui-monospace, Menlo, Consolas, monospace',
    fontSize: 12,
    theme: themeFromTokens(),
    cursorBlink: true,
    // §7.2. xterm's default is 1 000 lines, which makes scrolling up not worth doing — and
    // the pinned-scroll rule only pays for itself if there is something up there to read.
    scrollback: 10_000,
    // Let the agent CLI own the screen; we never reinterpret its output.
    convertEol: false,
  });
  const fit = new FitAddon();
  term.loadAddon(fit);
  fitAddons.set(term, fit);
  return term;
}

function TerminalImpl({ sessionId, readOnly }: { sessionId: SessionId; readOnly: boolean }) {
  const host = useRef<HTMLDivElement>(null);
  const session = useCockpit(selectSession(sessionId));
  const hasSession = session !== undefined;

  // Two pushes into the registry. Neither acquires, so neither can re-create anything.
  useEffect(() => {
    if (session) updateAttachSession(session);
  }, [session]);
  useEffect(() => {
    setAttachReadOnly(sessionId, readOnly);
  }, [sessionId, readOnly]);

  useEffect(() => {
    const node = host.current;
    // Read imperatively: a dependency on the session object would re-run this effect on every
    // 2 s poll, which is known defect 1 wearing a component's clothes.
    const current = useCockpit.getState().sessions[sessionId];
    if (!node || !current) return;

    const handle = acquireAttach(current, { createTerm: makeTerm, readOnly });
    node.appendChild(handle.element);
    handle.mount();                              // AFTER the append: xterm measures a
    fitAddons.get(handle.term)?.fit();           // rendered element, never a detached one
    handle.focus();

    const onWindowResize = () => fitAddons.get(handle.term)?.fit();
    window.addEventListener("resize", onWindowResize);

    return () => {
      window.removeEventListener("resize", onWindowResize);
      if (handle.element.parentNode === node) node.removeChild(handle.element);
      releaseAttach(sessionId);
    };
  }, [sessionId, hasSession]);

  return <div ref={host} style={{ width: "100%", height: "100%" }} />;
}

/** memo'd per §8 row 1: a parent re-render with unchanged props never reaches the effects. */
export const Terminal = memo(TerminalImpl);
```

The wrapper carries no class name on purpose. `develop.css`'s prefixes belong to tasks 20 and 21 (shared-shape §2), and a mount point that only ever fills its parent does not need one; the pane around it is what gets styled.

```bash
cd desktop
npm test -- src/components/Terminal.test.tsx
npm run typecheck
```

**What you must see:** six passing tests. `tsc` is expected to **still fail here, in one place only** — `src/App.tsx`, which is step 12's subject:

```
src/App.tsx(23,32): error TS2322: Type '{ url: string; }' is not assignable to type 'IntrinsicAttributes & { sessionId: string; readOnly: boolean; }'.
  Property 'url' does not exist on type 'IntrinsicAttributes & { sessionId: string; readOnly: boolean; }'.
```

Both lines are transcribed from a real compile, not reasoned about: the repository's own
`desktop/node_modules/.bin/tsc` (TypeScript 6.0.3) under `desktop/tsconfig.json`'s exact
options, against the current `App.tsx` and a `memo`'d `Terminal` with the new props. `23,32` is
where `url` sits on Phase 1's `{connected ? <Terminal url={url} /> : …}` line. **The position
will move if `App.tsx` has been touched since** — step 10 prints its own position as `NN` for
exactly that reason, because the file it names does not exist yet. This one does, so the
position is measured rather than guessed; if it has moved, the `TS2322` on `url` is still the
part that must be there. One error, in one file, and it is the file step 12 rewrites.

If instead `makeTerm` fails to compile with `Type 'Terminal' is not assignable to type 'TermLike'`, read the mismatch rather than casting it away: the interface is deliberately the smallest slice of xterm this app uses, and a genuine signature difference in `@xterm/xterm@6` belongs in `TermLike`, not under an `as unknown`.

- [ ] **Step 12: Make `App.tsx` compile again, and run the whole suite**

**This file is not in §9's list for task 19, and taking it is a deliberate departure.** §9 assigns `desktop/src/App.tsx` to task 24. But Phase 1's `App.tsx` renders `<Terminal url={url}/>` and it is the *only* caller of the component whose signature §6.6 requires this task to change, so after step 11 the repository does not typecheck — and "every task runs `npm run typecheck`" is not satisfiable by declaring the failure someone else's. The alternatives are worse: passing a fabricated `sessionId` would leave a dev tool that silently does nothing, which is a lie in code; and leaving the tree red hands task 20 a broken baseline it cannot tell apart from its own mistakes.

**The cost, stated plainly:** the Phase 1 "type a ws:// URL and get a terminal" harness stops working here and does not come back until task 24 mounts the real cockpit. That harness cannot survive §6.6 in any form — there is no session id to type into a box, and attach is deliberately no longer addressable from the UI by URL. Phase 1's proof of the PTY path is recorded in `docs/verification/2026-09-17-phase-1-results.md`; nothing is being claimed as working that is not.

The edit is the smallest one that compiles, and it applies shared-shape rule 1 — the modifying task reserves the insertion point in a comment that names its consumer. Replace `desktop/src/App.tsx` in full:

```tsx
export default function App() {
  return (
    <div style={{ display: "flex", height: "100vh", alignItems: "center", justifyContent: "center" }}>
      {/* Task 24 replaces this component entirely: <AppShell/> mounts here, startPolling()
          runs alongside it, and app.css is imported from main.tsx. Nothing else in this file
          is meant to survive. The Phase 1 ws:// harness was retired in task 19, when
          <Terminal/> stopped taking a URL (contract §6.6). */}
      <p style={{ opacity: 0.6, fontSize: 13 }}>The cockpit mounts here in task 24.</p>
    </div>
  );
}
```

Now the whole suite, the typechecker, and the guards below. (The Go guard — "no Group D task
edits Go" — moved to step 13, where it can be scoped to the staged set. Run from here it reports
every uncommitted `.go` file in the repository, including the `sandboxd/*.go` edits another
workstream already has in the tree, and a guard that fires on somebody else's work is a guard
the next implementer learns to ignore.)

```bash
cd desktop
npm test
npm run typecheck

# §6.8: the registry is the only thing that scrolls a pane. Anchored so the sentence about
# scrolling inside a comment cannot satisfy the guard.
grep -rnE "^[^/*]*\bscrollToBottom\(" src --include=*.ts --include=*.tsx \
  | grep -vE "src/(api/attachRegistry\.ts|test/fakes\.ts|api/attachRegistry\.test\.ts)"   # expect no output

# The URL prop is gone from the tree.
grep -rn "<Terminal url" src                                                              # expect no output

# No literal hex in anything this task wrote (task 24's audit will grep for it).
grep -nE "#[0-9a-fA-F]{3,8}\b" src/api/attachRegistry.ts src/api/poller.ts \
  src/components/Terminal.tsx src/App.tsx                                                 # expect no output
```

**What you must see:** every test in `desktop/` green — the thirty-eight this task adds (23 registry, 9 poller, 6 Terminal) plus everything tasks 14 to 18 left behind — `tsc` clean, and every guard above silent. Vitest's own summary line format is not transcribed here for the reason given at the top of this file; what matters is zero failures and zero unhandled errors, and in particular no `Unhandled Rejection` from a give-up fetch that outlived its test.

If a test in another file starts failing only when the full suite runs, suspect the registry's module-level state: `__resetAttachRegistry()` in `beforeEach` **and** `afterEach` is what keeps an entry from leaking into the next file, because vitest's `restoreMocks`/`clearMocks` do not reset module state (§1.6).

- [ ] **Step 13: Commit**

Stage by name — never `git add -A`, `-u` or `.` (shared-shape §6). Eight files: the **six** in §9's row for this task (`attachRegistry.ts`, `poller.ts`, `Terminal.tsx`, and the three test files), plus task 14's `fakes.ts`, grown in step 1 under shared shape §3.3, plus `App.tsx`, whose inclusion step 12 argues. Both additions are declared departures and neither is in §9's row.

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp
git add \
  desktop/src/test/fakes.ts \
  desktop/src/api/attachRegistry.ts \
  desktop/src/api/attachRegistry.test.ts \
  desktop/src/api/poller.ts \
  desktop/src/api/poller.test.ts \
  desktop/src/components/Terminal.tsx \
  desktop/src/components/Terminal.test.tsx \
  desktop/src/App.tsx

# The staged set, and nothing else. This is the "no Group D task edits Go" guard in the only
# form that is honest about who did what: `git status -- '*.go'` reports every uncommitted Go
# file in the repository, and another workstream's sandboxd/*.go edits may well be sitting in
# this tree. The staged set is what THIS task is about to commit.
git diff --cached --name-only             # expect exactly the eight lines above, in any order
git diff --cached --name-only -- '*.go'   # expect no output
```

The message is written to a file first. A multi-line heredoc can terminate early when the command crosses an extra `bash -c` boundary, and the symptom — `here-document delimited by end-of-file`, then the message body executing as shell commands — is worth one extra line to avoid.

```bash
printf '%s\n' \
'feat(cockpit): attach registry, poller and the session-id Terminal' \
'' \
'<Terminal/> takes a sessionId, not a URL (contract §6.6). The socket and the' \
'xterm instance move into a module-level, ref-counted registry keyed by session' \
'id, so a store write never disposes the terminal a user is typing into (defect' \
'1), a pane can move between Columns and Canvas without dropping the PTY, and' \
"StrictMode's double mount opens one socket rather than two." \
'' \
'Output batches on a 100 ms timer into one term.write per flush (~10 Hz), and' \
'pinned-ness is measured BEFORE that write, so a pane the user has scrolled up' \
'is never scroll-jumped by agent output (defect 5, §7.2).' \
'' \
'The attach ladder retries at 250/500/1000/2000 ms for 60 s while the session is' \
'starting, running or idle, and stops on exited. A refused handshake exposes' \
'neither status nor body to JavaScript, so on give-up the registry reads the' \
'session record once and phrases the reason from it: a running sandbox is never' \
'described as "not running", which is what non_loopback_publish (the sixteenth' \
'error code) would otherwise be mistold as.' \
'' \
'poller.ts adds the one 2 s interval: one unfiltered GET /v1/sessions per tick,' \
'paused while the document is hidden, never stacked.' \
'' \
'App.tsx is reduced to a placeholder that reserves task 24 its mount point. It' \
'was the only caller of the old URL prop and the tree must typecheck.' \
'' \
'Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>' \
> /tmp/task-19-commit.txt
git commit -F /tmp/task-19-commit.txt
```

Verify the trailer survived, exactly:

```bash
git log -1 --pretty=%B | tail -2
```

**What you must see:** the last non-empty line is `Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>`, with nothing after it.

---

### What task 20 inherits, in one place

- `<Terminal sessionId={id} readOnly={session.status === "exited"} />` — nothing else. No URL, no client, no socket.
- `useAttachState(sessionId)` for the footer's attach word. The phases are `detached`, `connecting`, `attached`, `reconnecting`, `ended`, `failed`. `reason` is a sentence written for a user and is rendered verbatim; `failed` is the one that wants a Restart action beside it (§4.6).
- The pane must render `<Terminal/>` for **all four statuses** (defect 3): the composer always exists, and `readOnly` narrows to "stdin is not forwarded", never to "the scrollback is gone".
- Do not call `scrollToBottom`, do not open a socket, and do not put attach state in the store. §6.8's ownership table is enforced by the anchored grep in step 12, which task 20 should keep running.
