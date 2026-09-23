# Task 27 — Desktop data: the design client, the event stream, the slice, the transport adapter

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 20a, 23 · **External:** **Hub C3 — Hub plan tasks 37–38, a hard dependency** (task 38's `LocalKeySource` and `client.ts` header; every design route answers a keyless call 401 `local_key_required`, task 20a, review C1); phase 4 task 16 (`keySources`, the loader, the provenance slice) · **Hours:** 2.5

## Why

The editor package talks only to `CanvasTransport` (task 23). This task implements it for the
cockpit: over REST for writes, and over **one SSE stream per window** for events and render
requests (P5-B, G10: a `fetch`, not an interval). It adds the design slice (not persisted, G3), the
registry of document stores (class instances, kept outside zustand), and one key source so phase
4's single loader resolves the approved brief keys the Design lens shows (seam S4-P5.1, ruling R6).

## Files

- `desktop/src/api/design.ts`, `desktop/src/api/events.ts` + tests (new)
- `desktop/src/api/client.ts` — the `ProblemCode` union gains task 20a's thirteen codes
  (`handoff_snapshot_invalid` is the twelfth, review C2; `design_unsafe` the thirteenth,
  re-review RR-M5) and `upstream_unavailable`, `signed_out`
  if the Hub's C1/C2 have not added them (`local_key_required` is Hub task 38's, already there). **Declared amendment**
  of UI contract §4.2 (task 30 records it). `problems.ts`' `classify` is unchanged: every new code is
  `server-said` (the server wrote the sentence), and `signed_out` is the Hub's C4's to map.
- `desktop/src/design/designTypes.ts`, `designSlice.ts`, `docstores.ts`, `transportAdapter.ts`,
  `selectors.ts` + tests (new)
- `desktop/src/store/store.ts` — `CockpitState` gains `DesignSlice`; `partialize` **unchanged**
- `desktop/src/store/selectors.ts` — `keySources` gains `selectDesignBriefKeys` (phase 4's array)

## Contracts

```ts
// desktop/src/api/design.ts — one method per task 20a, 21 and 22 route; every error is an ApiError
export interface DesignClient {
  canvasForBranch(projectId: string, branchId: string): Promise<Summary | null>;          // route 1
  createCanvas(projectId: string, branchId: string): Promise<{ summary: Summary; created: boolean }>; // 2
  summary(canvasId: string): Promise<Summary>;                                              // 3
  document(canvasId: string, at?: number): Promise<Document>;                               // 4
  submit(canvasId: string, b: { ops: Op[]; label: string; ifRevision?: number }): Promise<ApplyResult>; // 5
  undo(canvasId: string): Promise<UndoResult>;                                              // 6
  redo(canvasId: string): Promise<UndoResult>;                                              // 7
  revert(canvasId: string, sessionId: string, sinceRevision?: number): Promise<RevertResult>; // 8
  breakClaim(canvasId: string, artboardId: string): Promise<void>;                          // 9
  activity(canvasId: string, after: number, limit?: number): Promise<ActivityRow[]>;        // 10
  putAsset(canvasId: string, bytes: Blob): Promise<{ asset: Asset; revision: number }>;     // 11 (sha-256 computed with crypto.subtle for the path)
  assetBytes(canvasId: string, ref: string): Promise<Blob>;                                 // 12
  createThread(canvasId: string, nodeId: string, text: string): Promise<Thread>;            // 13
  reply(canvasId: string, threadId: string, text: string): Promise<Message>;                // 14
  setThreadStatus(canvasId: string, threadId: string, status: "open" | "resolved"): Promise<Thread>; // 15
  closeReview(canvasId: string, reviewId: string): Promise<ReviewRequest>;                  // 16
  setSelection(canvasId: string, nodeIds: string[]): Promise<void>;                         // 17
  exportCode(canvasId: string, nodeId: string, format: "html" | "jsx-inline" | "jsx-tailwind"): Promise<string>; // 18
  renderResult(requestId: string, r: RenderResult): Promise<void>;                          // 19
  approve(canvasId: string, revision: number): Promise<DesignApproval>;                     // 21; sent with timeoutMs: 0 (review I5: client.ts's 10 s DEFAULT_TIMEOUT_MS, client.ts:33, is shorter than one approval capture; the server bounds each artboard with ApprovalRenderTimeout and the toast shows progress)
  approvals(canvasId: string): Promise<ApprovalState>;                                      // 22
  reviewMark(canvasId: string): Promise<{ revision: number; briefKey: string }>;            // 23
  handoff(canvasId: string, approvalId: string, worktreeId: string): Promise<HandoffResult>; // 24
  linkSession(canvasId: string, approvalId: string, sessionId: string): Promise<SessionLink>; // 25
  sessionLinks(canvasId: string): Promise<SessionLink[]>;                                   // 26
  paste(canvasId: string, req: PasteRequest): Promise<PasteResult>;                         // task 29
}
// same options, same ApiError, same send() as client.ts — but localKey is REQUIRED here: a design
// client with no key source throws at construction ("the design client needs the local caller key,
// Hub plan task 38"), because every design route refuses a keyless call (task 20a, review C1).
export function createDesignClient(opts: ClientOptions & { localKey: LocalKeySource }): DesignClient;
```

`LocalKeySource` is Hub plan task 38's (`desktop/src/api/localKey.ts`); the design client and the
stream share the one instance the app builds at start, so a restarted control plane's new key is
re-read once for both.

`designTypes.ts` declares `Summary`, `ApprovalState`, `DesignApproval`, `SessionLink`,
`HandoffResult`, `ActivityRow`, `RevertResult` — field for field the Go JSON of tasks 11, 12, 21,
22 and the Hub's `hubapi` (contract §7.7, §7.8) — and re-exports the editor package's wire types.

```ts
// desktop/src/api/events.ts — the window stream (task 20a's format), read with fetch
export type StreamState = "connecting" | "open" | "reconnecting" | "closed";
export interface EventStreamOptions {
  baseUrl: string; fetch: typeof globalThis.fetch; windowId: string;
  localKey: LocalKeySource;  // REQUIRED (Hub plan task 38): get() before every connect; a 401
                             // local_key_required → refresh() and reconnect ONCE; a second
                             // CONSECUTIVE 401 → "closed". The allowance resets to one on every
                             // connect that answers 200, so a window outlives any number of
                             // control-plane restarts (re-review M-1)
  onCanvas(e: CanvasEvent): void;
  onRender(r: RenderRequest): void;
  onGone(g: { topic: string; code: string }): void;
  onState(s: StreamState): void;
}
export interface EventStream {
  setTopics(topics: string[]): void; // replaces the stream when the SET changes (open new, then abort old)
  close(): void;
}
export function openEventStream(o: EventStreamOptions): EventStream;
// Parser: the text/event-stream rules — "event:", "id:", "data:" lines (several data lines
// joined by "\n"), a blank line dispatches, a line starting ":" is ignored, a partial line waits
// for the next chunk. Reconnect after a drop with setTimeout back-off 0.5 s, 1 s, 2 s … 10 s
// (never an interval, G10), resending each canvas topic with the last revision it saw.
```

```ts
// desktop/src/design/designSlice.ts — NOT persisted (G3; ruling R3: phase 5 claims no version)
export interface DesignSlice {
  design: {
    windowId: string;                                  // "wn-" + 12 hex, minted at store creation
    byBranch: Record<string, { state: "unknown" | "loading" | "none" | "ready" | "error"; canvasId?: string; error?: string }>;
    summaries: Record<string, Summary>;                // by canvas id
    approvals: Record<string, ApprovalState>;          // by canvas id; refreshed on content events and after approve
    pendingApproval: { canvasId: string; revision: number; timer: number } | null; // P5-I
    // Set by the render responder (task 24) from a RenderRequest's progress (task 13) while the
    // approval POST is in flight; cleared when it settles. The toast reads it (task 28, review I5).
    approvalProgress: { canvasId: string; index: number; total: number; name: string } | null;
    // The canvas under approval, from designApprove until the POST settles (or Undo). While set,
    // the window keeps a hidden renderer for it, so leaving the lens mid-capture hands the next
    // re-sent artboard to that renderer instead of failing the approval (task 21 step 4,
    // re-review RR-I5-1). Set at the Undo window's start, so the stream swap it causes lands
    // before any capture request.
    approving: string | null;
    stream: StreamState;
    lastUndo: { canvasId: string; message: string } | null;
  };
  designLoadBranch(projectId: string, branchId: string): Promise<void>;
  designCreateCanvas(projectId: string, branchId: string): Promise<void>;
  designApprove(canvasId: string, revision: number): void;   // starts the 5 s Undo window; the POST is sent when it lapses
  designCancelApproval(): void;                               // the toast's Undo
  designApprovalProgress(p: { canvasId: string; index: number; total: number; name: string } | null): void; // the responder's call (review I5)
  designReReview(canvasId: string): Promise<void>;
  designStartDevelop(canvasId: string, approvalId: string, worktreeId: string, harness: string): Promise<{ sessionId: string; linked: boolean }>; // P5-N's three calls
  designRetryLink(canvasId: string, approvalId: string, sessionId: string): Promise<void>;
}

// desktop/src/design/docstores.ts — one DocStore per open canvas, outside zustand (they are
// mutable class instances with their own subscribe); created on first use, disposed when no
// view and no hidden renderer holds them.
export function docStore(canvasId: string): DocStore | undefined;
export function acquireDocStore(canvasId: string, transport: CanvasTransport): Promise<{ store: DocStore; release: () => void }>;

// desktop/src/design/transportAdapter.ts
export function createCockpitTransport(client: DesignClient, stream: EventStream): CanvasTransport;
// subscribe → adds "canvas:<id>@<after>" to the stream's topic set; a "gap" from the DocStore →
// document() reload; onRenderRequest → adds "renderer:<id>:visible|hidden", runs the handler,
// POSTs route 19; assetUrl → assetBytes → URL.createObjectURL, cached per ref, revoked on release.

// desktop/src/design/selectors.ts
export const selectDesignBriefKeys: KeySource;   // every loaded approval's briefKey (S4-P5.1)
// selectHiddenRendererCanvases (P5-Q): the canvas ids of branches that have at least one
// running or idle claude-code session (from the poller's sessions) and a loaded canvas —
// excluding the canvas the Design lens is showing (it has a visible renderer) — plus
// design.approving, INCLUDING when the lens shows it: that hidden renderer is the approval's
// fallback when the lens closes mid-capture (re-review RR-I5-1). Render still prefers visible.
export function selectHiddenRendererCanvases(s: CockpitState): string[];
// selectHandoffWorktrees (re-review RR-I1-5): the branch's worktrees route 24 accepts — kind
// "clone" first, then "primary" — never "linked" (route 24 answers 409 handoff_worktree_linked).
// kind is phase 3 task 06's Worktree.kind (it amends types.ts and UI contract §2; this plan does
// not). Until task 06 lands, handoffKind(w) — the one seam, in this file — returns
// w.isPrimary ? "primary" : "linked", which is exact before phase 3 (every non-primary worktree
// is then a `git worktree add` checkout). If task 06 is already in the tree when this task is
// built, handoffKind is `return w.kind` from the start; otherwise the merge that brings task 06
// into this branch changes it to that, in the same commit (seam S5-P3.3).
export function selectHandoffWorktrees(s: CockpitState, branchId: string): Worktree[];
```

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `events.test.ts › parser` | chunks split mid-line, mid-`data:` and across two events decode to the right events; `: ping` is ignored | an SSE parser that assumes one event per chunk |
| `events.test.ts › reconnect` (fake timers) | a drop reconnects after 0.5 s, then 1 s …; the second request's URL carries `@<last revision>` | reconnect replaying from zero, or never |
| `events.test.ts › noInterval` | the module never calls `setInterval` (spy) | a second clock (G10) |
| `events.test.ts › topicsReplace` | changing the topic set opens one new request and aborts the old | two streams per window |
| `events.test.ts › authHeader` | the first connect and every reconnect carry `Authorization: Bearer <localKey.get()>`; a 401 `local_key_required` → `refresh()` once and a reconnect with the new key; a second consecutive 401 → state `closed` and `onGone` with code `local_key_required`, no further request; and the sequence 401 → `refresh()` → 200, the connection dropped, 401 → `refresh()` → 200 again ends `open` with `refresh()` called twice (the allowance reset by the 200, re-review M-1) | a stream the local key refuses; a restart that leaves the window deaf, including the second restart in one window's life; an infinite retry |
| `design.test.ts › localKeyRequired` | `createDesignClient` without `localKey` throws; with it, every method's request carries the header (one assertion per method, the same table as the row below) | a design client that silently calls keyless and gets 401 on every route (review C1) |
| `design.test.ts` (one row per method) | method, path and body; an error body becomes `ApiError` with its code | a route typo found only in the lens |
| `designSlice.test.ts › undoWindow` (fake timers) | `designApprove` sends nothing for 5 s; `designCancelApproval` in that window sends nothing ever; after 5 s exactly one POST | an approval sent before Undo can stop it (P5-I) |
| `designSlice.test.ts › notPersisted` | `partialize(state)` has no `design` key | design state leaking into localStorage |
| `designSlice.test.ts › approvalProgress` | `designApprovalProgress({…, index: 3, total: 12, name: "Billing"})` sets it; the approve POST settling (success or `ApiError`) clears it; `approve` is called with `timeoutMs: 0` (an injected `fetch` sees no abort after 10 s, fake timers) | a 12-artboard approval aborted by the client's 10 s default, or a toast stuck on *"3 of 12"* (review I5) |
| `designSlice.test.ts › startDevelop` | three calls in order; a failing third leaves `linked: false` and the retry path works | an unlinked session nobody can fix |
| `transportAdapter.test.ts › gapReload` | a DocStore `gap` triggers exactly one `document()` | a canvas stuck behind after a drop |
| `transportAdapter.test.ts › renderRoundTrip` | a render event runs the handler and POSTs its result to route 19 | render requests that time out server-side |
| `selectors.test.ts` | `selectDesignBriefKeys` lists approved keys; `selectHiddenRendererCanvases` includes a branch with a running claude-code session and excludes the lens's canvas; with `approving` set to the lens's own canvas it includes that canvas, and with `approving` null it drops it again | hidden renderers for every branch, or none; an approval whose only renderer closes with the lens (re-review RR-I5-1) |
| `selectors.test.ts › selectHandoffWorktrees` (re-review RR-I1-5) | a branch with a primary, a linked and a clone worktree → `[clone, primary]`; a branch whose only worktree is linked → `[]`; before phase 3 (`handoffKind` on `isPrimary`), a non-primary worktree → excluded | a handoff offered into a linked worktree, answered 409 after the click |
| `designSlice.test.ts › approving` | `designApprove(c, 5)` sets `approving: c` at once (inside the Undo window); `designCancelApproval` clears it; the approve POST settling, with success or an `ApiError`, clears it | a hidden renderer kept for ever, or one attached only after capture has started |

## Steps

1. Tests with the injected `fetch`. 2. Code. 3. Add the key source to phase 4's `keySources` array
if it exists; otherwise export `selectDesignBriefKeys` and note in the commit that phase 4's task 16
must append it (its seam S4-P5.1 already says so). 4. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5/desktop && npx vitest run src/api src/design src/store && npx tsc --noEmit && git diff --stat -- src/store/types.ts
```
Expected: tests pass; typecheck clean; **no diff** for `types.ts` (G2).

## Not claimed

- No UI: the lens is task 28.
- The local key is Hub C3's (Hub plan tasks 37–38), and this task does not start until both have
  landed: the loopback Host guard and the Origin allowlist stop a browser, not a sandboxed process,
  so there is no "guarded as every other route" fallback (review C1). What the key does not stop is
  another process of the same OS user, which can read the key file (Hub contract §9.4).
