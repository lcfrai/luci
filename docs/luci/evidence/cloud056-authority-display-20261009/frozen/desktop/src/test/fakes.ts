/**
 * desktop/src/test/fakes.ts
 *
 * Injectable stand-ins shared by every Group D suite. Created by task 14; per the Group D
 * shared shape §3.3 these are NOT redefined in another file — a task that needs a capability
 * GROWS this copy and says so in both plans. TASK 19 is the known one that does: it adds
 * `scrollUp`, `resize`, `openedIn` and `focused` to FakeTerm, here. Replacing the file rather
 * than growing it orphans the non-exported helpers below and breaks this file's own typecheck
 * under `noUnusedLocals`. Task 19 also added `addons`, a write callback with a moving `baseY`,
 * recorded resize handlers, and `opens` / `openedWhileConnected` — the last two because the
 * mutants "mount is not idempotent" and "mount before append" both survived without them.
 * TASK 23's fix round 1 adds `attachCustomKeyEventHandler` and the handler it records, because
 * makeTerm now calls it and a fake without it throws on construction.
 *
 * This module imports nothing on purpose. It is loaded from inside `vi.mock` factories, and
 * the safe form of that is a dynamic import inside the factory, because `vi.mock` is hoisted
 * above the test file's own imports:
 *
 *     vi.mock("@xterm/xterm", async () => {
 *       const { FakeTerm } = await import("../test/fakes");
 *       return { Terminal: FakeTerm };
 *     });
 *
 * A factory that closes over a top-level `import { FakeTerm }` binding can throw "Cannot
 * access '__vi_import_0__' before initialization" depending on which module is pulled first.
 */

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
  /** task 19: how many times open() was called. StrictMode mounts every effect twice, and a
   *  second open() against a live terminal is the defect the idempotent mount() prevents. */
  opens = 0;
  /** task 19: xterm measures its cell size from a RENDERED element, so open() on a detached
   *  node yields a zero-column terminal. Records whether the host was in the document. */
  openedWhileConnected: boolean | null = null;
  buffer = { active: { viewportY: 0, baseY: 0 } };
  /** task 23 fix round 1: makeTerm hands xterm the cockpit's chord filter, and a test must be
   *  able to call it. Recorded, not run — this fake has no key path of its own. */
  customKeyEventHandler: ((event: KeyboardEvent) => boolean) | null = null;
  private dataHandlers: ((d: string) => void)[] = [];
  private resizeHandlers: ((size: { cols: number; rows: number }) => void)[] = [];

  loadAddon(addon: unknown) { this.addons.push(addon); }
  attachCustomKeyEventHandler(handler: (event: KeyboardEvent) => boolean) {
    this.customKeyEventHandler = handler;
  }
  open(parent: HTMLElement) {
    this.openedIn = parent;
    this.opens += 1;
    this.openedWhileConnected = parent.isConnected;
  }
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

export interface FakeMessageEvent {
  data: string | ArrayBuffer;
}

export interface FakeCloseEvent {
  code: number;
  reason: string;
  wasClean: boolean;
}

type Listener = (ev: unknown) => void;

function toArrayBuffer(bytes: Uint8Array): ArrayBuffer {
  const out = new ArrayBuffer(bytes.byteLength);
  new Uint8Array(out).set(bytes);
  return out;
}

/**
 * A structural stand-in for the browser WebSocket, injected wherever the code takes a socket
 * constructor. Inject it as `FakeWebSocket as unknown as typeof WebSocket`: it implements what
 * the cockpit uses, not the whole DOM interface.
 *
 * `instances` is module state. `restoreMocks` and `clearMocks` do NOT clear it, so every suite
 * that touches it calls `FakeWebSocket.reset()` in `beforeEach`.
 */
export class FakeWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;

  /** Every socket constructed since the last reset(), in construction order. */
  static instances: FakeWebSocket[] = [];

  static reset(): void {
    FakeWebSocket.instances = [];
  }

  /** The most recently constructed socket, or undefined if none. */
  static last(): FakeWebSocket | undefined {
    return FakeWebSocket.instances[FakeWebSocket.instances.length - 1];
  }

  readonly CONNECTING = 0;
  readonly OPEN = 1;
  readonly CLOSING = 2;
  readonly CLOSED = 3;

  readonly url: string;
  readonly protocol: string;
  binaryType: "blob" | "arraybuffer" = "blob";
  readyState: number = FakeWebSocket.CONNECTING;

  /** Everything send() was handed, in order. */
  sent: (string | ArrayBufferLike | ArrayBufferView)[] = [];
  /** Set by close(); null while the socket has never been closed by the client. */
  closedWith: { code?: number; reason?: string } | null = null;

  onopen: ((ev: unknown) => void) | null = null;
  onmessage: ((ev: FakeMessageEvent) => void) | null = null;
  onclose: ((ev: FakeCloseEvent) => void) | null = null;
  onerror: ((ev: unknown) => void) | null = null;

  private listeners: Record<string, Listener[]> = {};

  constructor(url: string | URL, protocols?: string | string[]) {
    this.url = String(url);
    this.protocol = Array.isArray(protocols) ? protocols[0] ?? "" : protocols ?? "";
    FakeWebSocket.instances.push(this);
  }

  addEventListener(type: string, cb: Listener): void {
    const list = this.listeners[type];
    if (list) list.push(cb);
    else this.listeners[type] = [cb];
  }

  removeEventListener(type: string, cb: Listener): void {
    this.listeners[type] = (this.listeners[type] || []).filter((l) => l !== cb);
  }

  send(data: string | ArrayBufferLike | ArrayBufferView): void {
    if (this.readyState !== FakeWebSocket.OPEN) {
      throw new Error(`FakeWebSocket: send() while readyState=${this.readyState}, expected OPEN`);
    }
    this.sent.push(data);
  }

  /** Client-side close. Records; does NOT dispatch — the browser's is asynchronous. */
  close(code?: number, reason?: string): void {
    if (this.readyState === FakeWebSocket.CLOSED) return;
    this.readyState = FakeWebSocket.CLOSED;
    this.closedWith = { code, reason };
  }

  // ── test drivers ──────────────────────────────────────────────────────────────────
  acceptOpen(): void {
    this.readyState = FakeWebSocket.OPEN;
    if (this.onopen) this.onopen({});
    this.fire("open", {});
  }

  emitText(data: string): void {
    const ev: FakeMessageEvent = { data };
    if (this.onmessage) this.onmessage(ev);
    this.fire("message", ev);
  }

  emitBinary(bytes: Uint8Array): void {
    const ev: FakeMessageEvent = { data: toArrayBuffer(bytes) };
    if (this.onmessage) this.onmessage(ev);
    this.fire("message", ev);
  }

  emitClose(code = 1000, reason = ""): void {
    this.readyState = FakeWebSocket.CLOSED;
    const ev: FakeCloseEvent = { code, reason, wasClean: code === 1000 };
    if (this.onclose) this.onclose(ev);
    this.fire("close", ev);
  }

  emitError(): void {
    if (this.onerror) this.onerror({});
    this.fire("error", {});
  }

  private fire(type: string, ev: unknown): void {
    for (const l of this.listeners[type] || []) l(ev);
  }
}

// ── CSS audit helpers ───────────────────────────────────────────────────────────────
// Not fakes, and they live here because contract §9's row for task 14 names nine files and no
// cssAudit.ts, and that row is authoritative for the file list. The
// alternative — a second hex regex inside task 24's designsystem.test.ts — is exactly how two
// audits of the same rule come to disagree. Task 24 imports these.

const CSS_BLOCK_COMMENT = /\/\*[\s\S]*?\*\//g;
const HEX_LITERAL = /#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{3,4})\b/g;
const CUSTOM_PROPERTY_DECL = /^[ \t]*(--[a-z0-9-]+)[ \t]*:/gm;

/**
 * Remove CSS block comments. tokens.css documents its nineteen source hexes in comments on
 * purpose, so a hex audit that does not strip first finds its own documentation.
 */
export function stripCssComments(css: string): string {
  return css.replace(CSS_BLOCK_COMMENT, "");
}

/** Every literal hex colour in the text, in source order, lowercased. */
export function findHexLiterals(css: string): string[] {
  return (css.match(HEX_LITERAL) || []).map((h) => h.toLowerCase());
}

/** Every custom property DECLARED — not merely referenced through var() — in the text. */
export function declaredCustomProperties(css: string): string[] {
  const out: string[] = [];
  CUSTOM_PROPERTY_DECL.lastIndex = 0;
  let m: RegExpExecArray | null = CUSTOM_PROPERTY_DECL.exec(css);
  while (m !== null) {
    out.push(m[1]);
    m = CUSTOM_PROPERTY_DECL.exec(css);
  }
  return out;
}
