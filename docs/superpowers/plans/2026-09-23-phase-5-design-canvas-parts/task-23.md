# Task 23 — The editor's data layer: wire types, the document store, the transport port

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 03, 07, 11 · **Hours:** 2.5

## Why

The editor package must run with no cockpit behind it (`OD-11` rule 4). So everything it knows
about the outside world goes through one port, `CanvasTransport`, which the desktop implements over
its API client (task 27) and tests implement in memory. The document it renders is a **store of
node state replaced from events** (P5-V, G8): the package never applies an op — it only builds ops
for the human's edits and sends them. And its style re-check (the renderer's line of defence, G7)
reads the same allowlist Go enforces, copied from task 07's generated file and pinned by a test.

## Files

- `canvas/editor/src/wire.ts`, `ops.ts`, `docstore.ts`, `transport.ts`, `memoryTransport.ts`,
  `allowlist.ts`, `generated/allowlist.json`, `scripts/sync-allowlist.mjs` (new)
- tests beside each; `src/boundary.test.ts` (new)
- `src/index.ts` — exports; `src/probe.tsx` and its test — **deleted**
- `package.json` — `"sync-allowlist": "node scripts/sync-allowlist.mjs"`

## Contracts

```ts
// canvas/editor/src/wire.ts — the Go JSON of tasks 05, 06, 11, 12, 13, field for field.
// A field added in Go is added here in the same change; wire.test.ts round-trips Go fixtures.

export type Kind = "artboard" | "frame" | "text" | "image" | "svg";
export type AuthorKind = "human" | "session";
export interface Author { kind: AuthorKind; userId: string; sessionId: string; handle: string }

export interface Node {
  id: string; kind: Kind; tag: string; name: string; parentId: string; order: string;
  styles: Record<string, string>; text: string; asset: string; alt: string; svg: string;
  attrs: Record<string, string>; x: number; y: number; locked: boolean; hidden: boolean;
  revision: number; editedBy: Author;
}
export interface Page { id: string; name: string; order: string }
export type TokenType = "color" | "radius" | "spacing" | "container" | "breakpoint"
  | "font-family" | "font-weight" | "font-size" | "line-height" | "letter-spacing";
export interface Token { name: string; type: TokenType; value: string }
export interface Asset { ref: string; mediaType: string; size: number; width: number; height: number }
export interface Message { id: string; text: string; at: string; author: Author }
export interface Thread { id: string; nodeId: string; status: "open" | "resolved"; messages: Message[]; createdAt: string; createdBy: Author }
export interface ReviewRequest { id: string; artboardIds: string[]; note: string; revision: number; at: string; author: Author; state: "open" | "closed" }
export interface Document {
  v: 1; canvasId: string; projectId: string; branchId: string; revision: number; createdAt: string;
  pages: Page[]; nodes: Node[]; tokens: Token[]; assets: Asset[]; threads: Thread[]; reviews: ReviewRequest[];
}
export interface Claim { artboardId: string; holders: Author[]; shared: boolean; since: string; expiresAt: string }
export interface Presence { author: Author; artboards: string[]; working: boolean; lastAt: string; lastLabel: string }

export type EventKind = "change" | "reset" | "claims" | "selection";
export interface CanvasEvent {
  canvasId: string; kind: EventKind; revision: number; at: string; author: Author | null; label: string;
  contentChanged: boolean; nodes: Node[]; deleted: string[]; pages: Page[]; deletedPages: string[];
  tokens: Token[]; deletedTokens: string[]; assets: Asset[]; threads: Thread[]; reviews: ReviewRequest[];
  claims: Claim[]; presence: Presence[]; document: Document | null; selection: string[];
}

export type OpKind = "create_page" | "update_page" | "delete_page" | "create_node" | "set_props" | "move"
  | "delete_node" | "set_token" | "delete_token" | "add_asset" | "create_thread" | "add_message"
  | "set_thread_status" | "request_review" | "close_review" | "review_mark";
export interface Op {
  kind: OpKind; pageId?: string; nodeId?: string; page?: Page; node?: Node;
  styles?: Record<string, string | null>; attrs?: Record<string, string | null>;
  name?: string; tag?: string; text?: string; alt?: string; asset?: string; svg?: string;
  locked?: boolean; hidden?: boolean; x?: number; y?: number; parentId?: string; order?: string;
  token?: Token; tokenName?: string;
}
export interface ApplyResult { revision: number; change: { revision: number; label: string; nodes: string[]; deleted: string[] }; notices: unknown[] }
export interface UndoResult { revision: number; undid: number; skipped: string[]; message: string }

export type RenderKind = "screenshot" | "computed" | "layout" | "font";
export interface RenderRequest { id: string; canvasId: string; revision: number; kind: RenderKind; nodeIds: string[]; scale: 1 | 2; family: string; deadline: string }
export interface Box { nodeId: string; x: number; y: number; width: number; height: number }
export interface Png { nodeId: string; width: number; height: number; scale: 1 | 2; data: string /* base64 */ }
export interface RenderResult {
  id: string; error: "" | "not-found" | "not-rendered" | "too-large" | "failed"; message: string;
  pngs: Png[]; computed: { nodeId: string; styles: Record<string, string> }[]; boxes: Box[];
  font: { family: string; available: boolean; weights: number[]; styles: string[]; variable: boolean; axes: string[]; source: "bundled" | "system" | "missing" } | null;
}
```

```ts
// canvas/editor/src/ops.ts — builders for the human's edits. They build; they never apply.
export function setStyles(nodeId: string, styles: Record<string, string | null>): Op;
export function setText(nodeId: string, text: string): Op;
export function rename(nodeId: string, name: string): Op;
export function move(nodeId: string, parentId: string, order: string): Op;
export function moveArtboard(nodeId: string, x: number, y: number): Op;
export function remove(nodeId: string): Op;
export function setFlags(nodeId: string, flags: { locked?: boolean; hidden?: boolean }): Op;
export function setToken(t: Token): Op;
export function deleteToken(name: string): Op;
// between(a, b): the fractional order key between two siblings — a TS port of task 05's
// algorithm, tested against the SAME vectors (the one piece of document logic both sides need,
// because the editor must name a drop position; the authority re-validates it).
export function between(a: string, b: string): string;
```

```ts
// canvas/editor/src/docstore.ts
export type ApplyOutcome = "applied" | "stale" | "gap";

export class DocStore {
  constructor(initial: Document);
  readonly canvasId: string;
  get revision(): number;
  get version(): number;               // bumps on every applied event; the useSyncExternalStore snapshot
  apply(e: CanvasEvent): ApplyOutcome; // change: only revision+1 applies; older → "stale"; newer → "gap" (the caller reloads)
                                        // reset: replaces everything; claims/selection: replace those sets, revision unchanged
  node(id: string): Node | undefined;
  children(parentId: string): Node[];  // by order, then id — the Go rule
  pages(): Page[];
  artboards(pageId: string): Node[];
  tokens(): Token[];
  asset(ref: string): Asset | undefined;
  threads(): Thread[];
  reviews(): ReviewRequest[];
  claims(): Claim[];
  presence(): Presence[];
  selection(): string[];
  subscribe(listener: () => void): () => void;
}
```

```ts
// canvas/editor/src/transport.ts — the package's ONLY window on the world
export interface CanvasTransport {
  document(canvasId: string, at?: number): Promise<Document>;
  subscribe(canvasId: string, after: number, onEvent: (e: CanvasEvent) => void): () => void;
  submit(canvasId: string, batch: { ops: Op[]; label: string; ifRevision?: number }): Promise<ApplyResult>;
  undo(canvasId: string): Promise<UndoResult>;
  redo(canvasId: string): Promise<UndoResult>;
  setSelection(canvasId: string, nodeIds: string[]): Promise<void>;
  assetUrl(canvasId: string, ref: string): Promise<string>; // a blob: URL; the transport owns its lifetime
  uploadAsset(canvasId: string, bytes: Blob): Promise<Asset>;
  exportCode(canvasId: string, nodeId: string, format: "html" | "jsx-inline" | "jsx-tailwind"): Promise<string>;
  createThread(canvasId: string, nodeId: string, text: string): Promise<Thread>;
  reply(canvasId: string, threadId: string, text: string): Promise<Message>;
  setThreadStatus(canvasId: string, threadId: string, status: "open" | "resolved"): Promise<Thread>;
  paste(canvasId: string, req: PasteRequest): Promise<PasteResult>; // task 29
  onRenderRequest(canvasId: string, visible: boolean, handle: (r: RenderRequest) => Promise<RenderResult>): () => void;
}
export interface PasteRequest { targetId: string; mode: "insert-children" | "replace-children"; html: string; index?: number }
export interface PasteResult { revision: number; created: string[]; dropped: { kind: string; what: string; where: string; reason: string }[] }
export class TransportError extends Error { constructor(public code: string, message: string, public details: Record<string, string>) }
```

```ts
// canvas/editor/src/memoryTransport.ts — for this package's tests and a standalone harness:
// an in-memory canvas that accepts batches, bumps revisions and emits events with resulting
// state. It implements the ops the EDITOR sends (set_props, move, delete_node, create_node,
// set_token, delete_token) — enough to test the editor, and deliberately not the authority.
export function createMemoryTransport(seed: Document): CanvasTransport & { events: CanvasEvent[] };
```

```ts
// canvas/editor/src/allowlist.ts — the renderer's re-check (G7), from generated/allowlist.json
export function allowedTag(kind: Kind, tag: string): boolean;
export function allowedStyle(property: string, value: string): boolean;
// allowedStyle: the property is in the list, AND the value contains none of: "\\", "<", ">",
// "{", "}", "@", "expression", "javascript:", "behavior", "-moz-binding", and every url( … )
// target is asset:sha256-<64 hex> (the renderer rewrites those to blob: URLs itself).
```

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `wire.test.ts` | Go-written fixtures (`canvas/authority/testdata/event-*.json`, produced by a Go test in task 11's package and committed) parse into these types with every field present | a TS type missing a field Go sends |
| `ops.test.ts › between vectors` | the seven vectors of task 05 (`("","")→"V"` …) | TS and Go disagreeing on an order key |
| `docstore.test.ts › gap` | revision 5, event 7 → `"gap"`, state unchanged | a store silently skipping a batch |
| `docstore.test.ts › stale` | event 5 at revision 5 → `"stale"` | a replayed event applied twice |
| `docstore.test.ts › reset` | a reset replaces nodes absent from the new document | ghosts after reconnect |
| `docstore.test.ts › childrenOrder` | children sorted by `order` then `id`, as Go | the canvas drawing siblings in another order than export |
| `allowlist.test.ts › copyMatchesGo` | `generated/allowlist.json` equals `../../dialect/allowlist.json` byte for byte | the renderer's list drifting from the sanitiser's |
| `allowlist.test.ts › refuses` | `margin`, `url(https://x)`, `expression(`, `\\75rl(` → false; `color: var(--brand)` → true | a renderer that trusts the server blindly |
| `boundary.test.ts` | no file under `src/` imports a relative path outside `src/`, or a bare module outside `package.json`'s dependencies and peers | the package importing the cockpit (G13) |
| `memoryTransport.test.ts` | a submitted `set_props` yields one change event with the node's full state | tests passing against a transport the desktop's does not match |

## Steps

1. `sync-allowlist.mjs`, run it, commit the copy. 2. Tests, then the files. 3. Delete the probe.
4. Acceptance; commit by pathspec (the Go fixture files come from task 11's package: add a small Go
test there that writes them with `-update`, and commit them with this task).

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5/canvas/editor && npx vitest run && npx tsc --noEmit && echo ok
```
Expected: all tests pass, then `ok`.

## Not claimed

- No UI yet: renderer task 24, editor 25–26.
- The TS `between` is the one piece of document logic in TypeScript, and only because the editor
  must name a drop position; the authority re-validates every key it receives.
