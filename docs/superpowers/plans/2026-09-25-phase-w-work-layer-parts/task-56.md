### Task 56: the replica, part 2: optimistic writes, rollback on refusal, `optimistic.undo`, `U`

Stream **WD1**, the write half of seam **SW.16** (the replica), about 3 hours (an estimate, not a
measurement). Depends on task 55 (the replica's read half: `createReplica`, `Replica`, `useWork`,
`WorkRecord`). It also calls three contracts it does not wait to be built: task 37's `WorkClient`
(SW.15, `task-37.md:318-361`), task 53's `registerVerb` (the keymap; no part file yet, so its
signature here is the scope's words, **UNVERIFIED** until task 53's part lands) and task 47's
revision history (the way back after the window).

After this task every Work edit on the desktop goes through one call, `replica.mutate(op)`. It
paints in the same frame, sends once with a key that survives retries, and on a refusal takes back
**only what it changed**. Nothing else in `src/` calls a `WorkClient` write method.

**Decisions made here, so nobody re-derives them:**

1. **An op is a field-level change, not a snapshot.** Each `WorkOp` names its issue and the fields
   it touches. The replica records the op's **before** value of those fields only. A refusal writes
   the before values back **only where the field still holds the op's after value**, so a
   teammate's change that arrived in between survives (the scope's first rollback test). Labels are
   per name: a refused `labels {add: ["ui"]}` removes `ui` and leaves a label a teammate added.
2. **The wire decides what an op sends.** `PatchIssueRequest.labels` is a whole list
   (`task-37.md:190-192`), so a `labels` op sends the replica's list with its adds and removes
   applied **at send time**, not at click time. Two quick label ops on one issue send in order,
   one after the other (a per-issue queue), so the second is computed on the first's result.
3. **The key.** The desktop mints one key per op with `makeId("op")` (`lib/ids.ts:13-15`: at most
   ~20 characters of `[a-z0-9-]`, inside the Hub's `1-64 of [A-Za-z0-9_-]`, `hub-contract.md:50`)
   and sends it on **every** attempt of that op as the `Idempotency-Key` header. The Hub keys
   replays by *(principal, key)* for 24 hours and applies to `POST` only (`hub-contract.md:50-55`);
   the `PATCH`, `PUT` and `DELETE` ops (`setPriority`, `setAssignee`, `labels`, `subscribe`,
   `relation remove`) set a value, so a repeat is harmless without it. `WorkSend` has no header
   parameter today (`task-37.md:373`), so this task adds an optional fourth argument (the delta is
   printed below). The control plane's `hubclient` mints its **own** key per call and reuses it
   across its own retries (`hub-contract.md:1345`, pinned by task 16's
   `TestAPostCarriesOneIdempotencyKeyAcrossRetries`, `task-16.md:844`). For the desktop's key to
   reach the Hub, task 16's relay must forward an inbound `Idempotency-Key` in place of the minted
   one. Task 16's part does not print that: **a gap, named in the return**. Until it lands, the
   desktop's key dedupes nothing past the loopback, and a desktop resend after an unknown outcome
   can double a comment. Decision 6 keeps that case narrow.
4. **Refused, unavailable, conflict.** `workProblem` (`task-37.md:466-485`) sorts the failure.
   `refused` and `signed_out` roll back and fire `onRefused` once. `unavailable` does **not** roll
   back: the op stays painted, marked *not sent yet* on its row, and waits in the in-memory queue
   for the replica's recovery signal (task 57's resume). `revision_conflict` on `revise` rolls the
   text back to the new head **and keeps the person's text** as a `DraftAgain` the editor offers
   on the new head (task 49 reads it; nothing is thrown away).
5. **Undo is a new write by the person.** `U` and the chip send `inverseOf(op, before)` as a fresh
   op with its own key, never a local-only revert. The Hub records it like any other write, so the
   revision history (task 47) shows both. A **batch** (`mutate([...])`) is one undo entry whose
   inverse is the batch of inverses.
6. **What has no inverse.** A hand move is legal only **into** `triage`, `backlog`, `todo`,
   `cancelled`, `duplicate` or `done` (`hubapi.MoveAllowed`, `task-10.md` decision 1). So a move
   **out of** `started`, `in_review` or `landed` cannot be undone by hand, and a move out of `done`
   cannot restore the proof the reopen cleared (`ClearCitations`, `task-10.md:419-420`). Those ops
   return `undoable: false`; the chip is not shown and the row's history is the way back. A
   `comment` has no inverse either: comments are append-only (`task-37.md:153`), so the comment
   paints at once as *sending* and gets no chip. A comment whose outcome is unknown (`unavailable`
   after the request left) is **not resent**; it stays as *not confirmed · Retry*, and Retry is
   the person's choice. That is decision 3's narrow case.
7. **Irreversible acts are not ops.** Erase, land, push and deploy are `confirm.inplace` (task 44;
   05 §4.6 row 5). `WorkOp` is a closed union without them, and a `@ts-expect-error` test pins it.
8. **The window pauses.** `--undo-window` is 6000 ms (`task-26.md:85`). The chip reads the token
   once through `getComputedStyle`, counts whole seconds in `tabular-nums`, and **pauses while its
   row has focus or hover** (05 §4.6, `05-design.md:1108-1109`). The count is not a timer per row:
   the ledger keeps `startedAt` and `pausedMs` against an injected clock, and the chip re-reads it on
   the one 1 Hz tick, `useHeartbeat()` (`hooks/heartbeat.ts:64`). A chip therefore lapses at the
   first beat after 6 s of unpaused time, up to one second late; that is recorded, not hidden.

**Files** (the skeleton's File map lists `src/work/replica/{…,ops,undo,…}.ts` for 55-58, `:304`;
it omits `UndoChip.tsx` and the two `api/` deltas, gaps named in the return):
- `desktop/src/work/replica/ops.ts` — `WorkOp`, `FieldPath`, `fieldsOf`, `readField`,
  `writeField`, `inverseOf`, `sendOp` (new)
- `desktop/src/work/replica/ops.test.ts` — the op tests (new)
- `desktop/src/work/replica/undo.ts` — `UndoLedger`, `createUndoLedger`, `useUndoEntry` (new)
- `desktop/src/work/replica/undo.test.ts` — the window tests (new)
- `desktop/src/work/replica/replica.ts` (task 55) — `mutate`, `onRefused`, `pendingOf`,
  `draftAgain`, the per-issue send queue, `ReplicaWriteView` (modified)
- `desktop/src/work/replica/mutate.test.ts` — the rollback, key and conflict tests (new)
- `desktop/src/components/common/UndoChip.tsx` — the chip (new)
- `desktop/src/components/common/UndoChip.test.tsx` — the pause and batch tests (new)
- `desktop/src/components/common/common.css` ⚑ — `.undo-chip` (modified)
- `desktop/src/work/keys/undoVerb.ts` — registers `U` through task 53's `registerVerb` (new)
- `desktop/src/api/work.ts` ⚑ (task 37) — `WorkSend` gains `opts`; `WorkClient` gains `keyed(key)`
  (modified)
- `desktop/src/api/client.ts` ⚑ — `send` passes `Idempotency-Key` when given (modified)

#### Interfaces — complete: the deltas on `api/work.ts` and `api/client.ts`

```ts
// desktop/src/api/work.ts, part 7 (task-37.md:373): WorkSend gains an optional fourth argument.
export interface SendOptions {
  /** 1-64 of [A-Za-z0-9_-] (hub-contract §1). Sent as Idempotency-Key on every attempt. */
  idempotencyKey?: string;
}
export type WorkSend = (verb: Verb, route: string, body?: unknown, opts?: SendOptions) => Promise<unknown>;

// WorkClient (task-37.md:318) gains one member, after importRun:
  /** The same client with every call carrying `key` as its Idempotency-Key. The replica calls
   *  `keyed(op.id)` once per op and reuses that client for each attempt of the op. */
  keyed(key: string): WorkClient;

// makeWorkClient (task-37.md:399) takes the options it forwards; its two helpers pass them on:
export function makeWorkClient(send: WorkSend, opts: SendOptions = {}): WorkClient {
  const relay = <T,>(m: RelayMethod, args: ReadonlyArray<string | number>, body?: unknown, q = ""): Promise<T> => {
    const [, verb, hub] = WORK_RELAY.find((r) => r[0] === m)!;
    return send(verb, relayPath(fill(hub, args)) + q, body, opts) as Promise<T>;
  };
  const local = <T,>(m: LocalMethod, args: ReadonlyArray<string>, body?: unknown): Promise<T> => {
    const [, verb, path] = WORK_LOCAL.find((r) => r[0] === m)!;
    return send(verb, fill(path, args), body, opts) as Promise<T>;
  };
  // … the thirty-five members exactly as task 37 prints them, then:
  //   keyed: (key) => makeWorkClient(send, { ...opts, idempotencyKey: key }),
}
```

```ts
// desktop/src/api/client.ts. `send` (:245) takes the options; the header joins the one it sends.
  async function send(method: Method, route: string, body?: unknown, opts?: SendOptions): Promise<Reply> {
    // …
      const headers: Record<string, string> = {};
      if (body !== undefined) headers["Content-Type"] = "application/json";
      if (opts?.idempotencyKey) headers["Idempotency-Key"] = opts.idempotencyKey;
      const res = await doFetch(url, {
        method,
        signal: controller.signal,
        headers: Object.keys(headers).length === 0 ? undefined : headers,
        body: body === undefined ? undefined : JSON.stringify(body),
      });
// and the `work` line task 37 adds (task-37.md:521) forwards the fourth argument:
  const work = makeWorkClient(async (verb, route, body, o) => parseJson<unknown>(await send(verb, route, body, o)));
```

`headers: undefined` when there is nothing to send keeps the built `client.test.ts` assertions on a
body-less `GET` byte-identical. The webview's origin is not the loopback's, so `Idempotency-Key`
is a non-safelisted header and the browser preflights it. The built control plane allows
`Content-Type` only (`control-plane/internal/httpx/origin.go:170`, `corsHeaders = "Content-Type"`,
pinned by `origin_test.go:172`), so without the Go delta below every keyed write fails its
preflight. Two more files join the list:
- `control-plane/internal/httpx/origin.go` ⚑ — `corsHeaders` becomes `"Content-Type, Idempotency-Key"`
  (modified). If phase H's C3 local-key header lands first, the two names join one list; the
  owner phase's file wins (rule H2).
- `control-plane/internal/httpx/origin_test.go` ⚑ — the expectation at `:172` follows (modified)

#### Interfaces — complete: `desktop/src/work/replica/ops.ts`, part 1: the op and its fields

```ts
// desktop/src/work/replica/ops.ts — SW.16's write half, task 56.
//
// One op is one person's change to one issue, named by the fields it touches. The replica keeps
// the op's BEFORE value of those fields and nothing else, so a refusal takes back only its own
// change. Irreversible acts (erase, land, push, deploy) are not ops: they are confirm.inplace.
import type {
  CreateCommentRequest, IssueComment, IssueHead, IssueRevision, IssueRevisionContent,
  Priority, Relation, StateMove, User, WorkClient,
} from "../../api/work";

export type WorkOp =
  | { kind: "setState"; issueId: string; move: StateMove }
  | { kind: "setPriority"; issueId: string; priority: Priority }
  /** `assignee` is the picked User so the row paints a name; the wire gets its id. */
  | { kind: "setAssignee"; issueId: string; assignee: User | null }
  | { kind: "labels"; issueId: string; add: readonly string[]; remove: readonly string[] }
  | { kind: "revise"; issueId: string; baseRev: number; patch: Partial<IssueRevisionContent> }
  | { kind: "relation"; issueId: string; change: "add" | "remove"; relation: Relation }
  /** `localId` names the painted comment until the Hub returns its cm- id. */
  | { kind: "comment"; issueId: string; localId: string; req: CreateCommentRequest }
  | { kind: "reaction"; issueId: string; commentId: string; emoji: string; on: boolean }
  | { kind: "resolve"; issueId: string; commentId: string; resolved: boolean }
  | { kind: "subscribe"; issueId: string; subscribed: boolean };
export type WorkOpKind = WorkOp["kind"];

/** One field an op can touch. Labels, relations and reactions are per member, so two people
 *  adding two different labels never take each other's back. */
export type FieldPath =
  | "state" | "priority" | "assignee" | "subscribed"
  | `label:${string}` | `relation:${Relation["kind"]}:${string}`
  | "title" | "description" | "serves" | "implements"
  | `comment:${string}` | `reaction:${string}:${string}` | `resolved:${string}`;

/** The replica's records as an op reads them (task 55 holds them; decision 1). */
export interface ReplicaReadView {
  readonly me: User;
  head(issueId: string): IssueHead | undefined;
  revision(issueId: string): IssueRevision | undefined;
  comments(issueId: string): readonly IssueComment[];
  subscribed(issueId: string): boolean;
}
/** The one way an op changes the replica. `transact` notifies useWork once per call. */
export interface ReplicaWriteView extends ReplicaReadView {
  transact(fn: (w: ReplicaWriter) => void): void;
}
export interface ReplicaWriter {
  putHead(h: IssueHead): void;
  putRevision(r: IssueRevision): void;
  /** Adds, or replaces the comment with the same id. */
  putComment(c: IssueComment): void;
  dropComment(issueId: string, commentId: string): void;
  setSubscribed(issueId: string, on: boolean): void;
}

/** A field's value as the rollback compares it: JSON-equal, so arrays compare by content. */
export type FieldValue = string | number | boolean | null | readonly unknown[] | Record<string, unknown>;
export type Snapshot = ReadonlyMap<FieldPath, FieldValue>;

export function fieldsOf(op: WorkOp): FieldPath[];
export function readField(view: ReplicaReadView, issueId: string, path: FieldPath): FieldValue;
export function writeField(w: ReplicaWriter, view: ReplicaReadView, issueId: string, path: FieldPath, v: FieldValue): void;
/** Paints the op; returns the before and after snapshots of fieldsOf(op). */
export function applyLocal(view: ReplicaWriteView, op: WorkOp): { before: Snapshot; after: Snapshot };
/** Writes `before` back only where the field still equals `after` (decision 1). */
export function revertLocal(view: ReplicaWriteView, op: WorkOp, before: Snapshot, after: Snapshot): FieldPath[];
```

`fieldsOf` needs only the op: a `revise` touches only the content keys its `patch` names, and a
`labels` op touches one path per added or removed name. `revertLocal` returns the paths it wrote
back, so the test can say which fields a rollback touched.

#### Interfaces — complete: `ops.ts`, part 2: the inverse and the send

```ts
/** A hand move may go only into these (hubapi.MoveAllowed, task-10.md decision 1). */
const HAND_INTO: ReadonlySet<string> = new Set(["triage", "backlog", "todo", "cancelled", "duplicate", "done"]);
/** A move OUT of these has no hand inverse: fact states, and done, whose proof a reopen clears. */
const NO_HAND_RETURN: ReadonlySet<string> = new Set(["started", "in_review", "landed", "done"]);

/** Whether `mutate` offers an undo for this op, decided when it is painted (decision 6). */
export function undoable(op: WorkOp, before: Snapshot): boolean {
  if (op.kind === "comment") return false;
  if (op.kind === "setState") return !NO_HAND_RETURN.has(String(before.get("state")));
  return true;
}

/** The person's own inverse, built when it reaches the head of its issue's queue, so a revise's
 *  baseRev is the revision the op wrote. null when nothing would change (decision 5). */
export function inverseOf(op: WorkOp, before: Snapshot, view: ReplicaReadView): WorkOp | null {
  switch (op.kind) {
    case "setState": {
      const from = String(before.get("state"));
      if (!HAND_INTO.has(from) || NO_HAND_RETURN.has(from)) return null;
      return { kind: "setState", issueId: op.issueId,
        move: { to: from as StateMove["to"], cause: "hand", reason: "", landCommit: "", evidence: [] } };
    }
    case "setPriority":
      return { kind: "setPriority", issueId: op.issueId, priority: before.get("priority") as Priority };
    case "setAssignee":
      return { kind: "setAssignee", issueId: op.issueId, assignee: before.get("assignee") as User | null };
    case "labels": {
      const add = op.remove.filter((n) => before.get(`label:${n}`) === true);
      const remove = op.add.filter((n) => before.get(`label:${n}`) === false);
      return add.length + remove.length === 0 ? null : { kind: "labels", issueId: op.issueId, add, remove };
    }
    case "revise": {
      const rev = view.revision(op.issueId);
      if (!rev) return null;
      const patch: Partial<IssueRevisionContent> = {};
      for (const k of Object.keys(op.patch) as (keyof IssueRevisionContent)[]) {
        (patch as Record<string, unknown>)[k] = before.get(k);
      }
      return { kind: "revise", issueId: op.issueId, baseRev: rev.revision, patch };
    }
    case "relation": {
      const path = `relation:${op.relation.kind}:${op.relation.issueId}` as const;
      const was = before.get(path) === true;
      if (was === (op.change === "add")) return null;
      return { ...op, change: was ? "add" : "remove" };
    }
    case "comment":
      return null;
    case "reaction": {
      const was = before.get(`reaction:${op.commentId}:${op.emoji}`) === true;
      return was === op.on ? null : { ...op, on: was };
    }
    case "resolve": {
      const was = before.get(`resolved:${op.commentId}`) === true;
      return was === op.resolved ? null : { ...op, resolved: was };
    }
    case "subscribe": {
      const was = before.get("subscribed") === true;
      return was === op.subscribed ? null : { ...op, subscribed: was };
    }
  }
}

/** What the Hub answered, in the replica's terms. */
export type Settled =
  | { head: IssueHead } | { revision: IssueRevision }
  | { comment: IssueComment; replacesLocal?: string } | { subscribed: boolean };

/** One attempt. `client` is already keyed(op id); a retry calls this again with the same client.
 *  A labels op sends the replica's list AT SEND TIME (decision 2). */
export async function sendOp(op: WorkOp, client: WorkClient, view: ReplicaReadView): Promise<Settled> {
  const id = op.issueId;
  switch (op.kind) {
    case "setState": return { head: await client.moveIssueState(id, op.move) };
    case "setPriority": return { head: await client.patchIssue(id, { priority: op.priority }) };
    case "setAssignee": return { head: await client.patchIssue(id, { assigneeId: op.assignee?.id ?? null }) };
    case "labels": return { head: await client.patchIssue(id, { labels: [...(view.head(id)?.labels ?? [])] }) };
    case "revise": {
      const content = view.revision(id)!.content;          // painted: already carries op.patch
      return { revision: await client.writeIssueRevision(id, { baseRevision: op.baseRev, content }) };
    }
    case "relation": return { head: op.change === "add"
      ? await client.addRelation(id, op.relation)
      : await client.removeRelation(id, op.relation.kind, op.relation.issueId) };
    case "comment": return { comment: await client.createComment(id, op.req), replacesLocal: op.localId };
    case "reaction": return { comment: await client.react(op.commentId, op.emoji, op.on) };
    case "resolve": return { comment: await client.resolveComment(op.commentId, op.resolved) };
    case "subscribe": return { subscribed: (await client.subscribe(id, op.subscribed)).subscribed };
  }
}
```

#### Interfaces — complete: the deltas on `replica.ts` (task 55)

```ts
// desktop/src/work/replica/replica.ts — task 56 adds the write half. Task 55's Replica keeps
// every member it declares; these join it.
import { makeId } from "../../lib/ids";
import { workProblem } from "../../api/work";
import type { ApiError } from "../../api/client";
import type { IssueRevisionContent, WorkClient } from "../../api/work";
import { applyLocal, revertLocal, sendOp, undoable, inverseOf, type Snapshot, type WorkOp } from "./ops";
import type { UndoLedger } from "./undo";

/** Why a write did not stand. `code` and `message` are the server's own (workProblem). */
export interface Refusal {
  op: WorkOp; issueId: string; code: string; message: string; details: Record<string, string>;
}
/** A revise the Hub refused as revision_conflict: the person's words, kept for the new head. */
export interface DraftAgain {
  issueId: string; baseRev: number; headRev: number; patch: Partial<IssueRevisionContent>;
}
/** What a row shows about its own write. Absent: nothing pending. */
export type PendingMark = "sending" | "not_sent" | "not_confirmed";

export interface MutateHandle {
  /** The op's id, also its Idempotency-Key; a batch's id is its first op's. */
  readonly id: string;
  /** False for a comment and for a move out of a fact state or done (decision 6). */
  readonly undoable: boolean;
  /** Sends the inverse as a new write by the person; a no-op once lapsed or when !undoable. */
  undo(): void;
  /** "applied" when every op of the batch stood; else the first Refusal. */
  readonly settled: Promise<"applied" | Refusal>;
}

export interface ReplicaWrites {
  mutate(op: WorkOp | readonly WorkOp[]): MutateHandle;
  /** Fires once per refused op; task 66 speaks it. Returns the unsubscribe. */
  onRefused(listener: (r: Refusal) => void): () => void;
  pendingOf(issueId: string): PendingMark | null;
  draftAgain(issueId: string): DraftAgain | null;
  /** The ops painted but not sent because the Hub was unreachable (decision 4). */
  queued(): number;
  /** Task 57 calls this when the relay answers again; the queue drains in op order. */
  resume(): void;
}
// `interface Replica extends ReplicaWrites` — task 55's Replica gains these members.

/** Task 55's ReplicaDeps gains the two things the write half needs. */
export interface ReplicaWriteDeps {
  work: Pick<WorkClient, "keyed">;
  undo: UndoLedger;
}
```

**How `mutate` runs**, in order, inside `createReplica`:

1. For each op: `id = makeId("op")`; `{before, after} = applyLocal(view, op)` in **one**
   `transact`, so `useWork` re-renders once for a batch; `pending[issueId] = "sending"`.
2. `ledger.push({id, ops, before, issueIds})` when every op is `undoable(op, before)`; a batch
   with one non-undoable op gets no chip (it would undo only part of what the person did).
3. The op joins its issue's queue. The queue's head runs `sendOp(op, work.keyed(id), view)`; a
   `unavailable` answer leaves it at the head, marks it `not_sent`, and waits for `resume()`
   (a comment whose request left is `not_confirmed` instead and is not resent, decision 6).
4. On success: `transact` puts the Hub's record (`head`, `revision`, or `comment` replacing the
   local id), then **re-applies the painted fields of the ops still queued on that issue**, so a
   second quick op's paint is not overwritten by the first op's answer.
5. On `refused` or `signed_out`: `revertLocal(view, op, before, after)`; the row flashes
   `--fail` for `--dur-flash` with the code and message; every listener gets the `Refusal` once;
   the ledger entry is dropped; the rest of that issue's queue still runs.
6. On `revision_conflict` for a `revise`: fetch the head (task 55's `fetchSubjects`), put it,
   and store `DraftAgain {baseRev, headRev, patch}`; the settled value is the `Refusal`.

`undo()` pushes `inverseOf` **lazily**: an `{inverseOf: id}` marker joins each issue's queue, and
the inverse is built when the marker reaches the head, from the op's `before` and the replica at
that moment. That is why a revise's inverse carries the revision the op wrote as its `baseRev`.

#### Interfaces — complete: `desktop/src/work/replica/undo.ts`

```ts
// desktop/src/work/replica/undo.ts — optimistic.undo's window (05 §4.6, revision 2).
//
// One entry per mutate() call, so a batch is one entry and one chip. Time is an injected clock
// (performance.now in the app, a fake in tests); nothing here owns a timer.
import { useSyncExternalStore } from "react";
import { useHeartbeat } from "../../hooks/heartbeat";
import type { Snapshot, WorkOp } from "./ops";

export interface UndoEntry {
  readonly id: string;
  readonly ops: readonly WorkOp[];
  readonly before: readonly Snapshot[];      // one per op, same order
  readonly issueIds: readonly string[];      // the rows whose hover or focus pauses it
  readonly startedAt: number;
  pausedMs: number;
  pausedSince: number | null;
}

export interface UndoLedger {
  push(e: Omit<UndoEntry, "startedAt" | "pausedMs" | "pausedSince">): void;
  /** Hover or focus on one of the entry's rows. Counted per source, so leaving hover while the
   *  row keeps focus stays paused. */
  pause(id: string, source: "hover" | "focus", on: boolean): void;
  remainingMs(id: string): number;
  /** The newest entry still inside its window: what `U` undoes. */
  latestLive(): UndoEntry | null;
  /** Removes and returns the entry (undo pressed, or its op refused). */
  take(id: string): UndoEntry | null;
  /** Drops every entry whose window has run out; returns their ids. */
  lapse(): string[];
  subscribe(listener: () => void): () => void;
  entries(): readonly UndoEntry[];
}

/** `windowMs` is --undo-window read once at start (6000 in tokens.css, task-26.md:85). */
export function createUndoLedger(clock: () => number, windowMs: number): UndoLedger;

/** The chip's view of one entry: whole seconds left, re-read on the 1 Hz heartbeat. It calls
 *  ledger.lapse() on each beat, so an entry lapses on the first beat after its window. */
export function useUndoEntry(ledger: UndoLedger, id: string): { secondsLeft: number; live: boolean };
```

`remainingMs` is `windowMs - (clock() - startedAt - pausedMs - (pausedSince ? clock() - pausedSince : 0))`,
floored at 0. A paused entry's remaining time does not fall, which is the whole of revision 2.

#### Interfaces — complete: `UndoChip` and the `U` verb

```tsx
// desktop/src/components/common/UndoChip.tsx
export interface UndoChipProps {
  ledger: UndoLedger;
  entryId: string;
  /** "Moved to Todo", "3 issues to Todo": the act, never "Success". */
  label: string;
  onUndo(): void;
}
/** <span class="undo-chip" role="group" aria-label={label}>
 *    {label} · <button type="button">Undo</button> <kbd>U</kbd>
 *    <span class="undo-chip-left" aria-hidden="true">{secondsLeft}s</span>
 *  </span>
 *  Pauses through a pointerover/focusin listener on document that tests
 *  `target.closest("[data-issue-id]")` against the entry's issueIds: one listener per chip,
 *  and at most a few chips exist. Renders nothing once !live. */
export function UndoChip(props: UndoChipProps): JSX.Element | null;
```

```ts
// desktop/src/work/keys/undoVerb.ts — `U` through task 53's registry (KEYMAP's `optimistic.undo`
// row; its VerbId is task 53's, UNVERIFIED here).
import { registerVerb } from "./keymap";
export function registerUndoVerb(replica: { undoLatest(): boolean }): () => void {
  return registerVerb("optimistic.undo", () => { replica.undoLatest(); });
}
// Replica gains `undoLatest(): boolean` — `ledger.latestLive()`, then that handle's undo();
// false when nothing is live, and then `U` does nothing and says nothing.
```

```css
/* desktop/src/components/common/common.css ⚑ */
.undo-chip { display: inline-flex; align-items: center; gap: 6px; padding: 0 6px;
  border-radius: var(--r-chip); background: var(--inset); color: var(--ink-2);
  font-family: var(--sans); font-size: var(--t-fact); line-height: var(--t-fact-lh); }
.undo-chip button { color: var(--ink); }
.undo-chip-left { font-family: var(--mono); font-variant-numeric: tabular-nums; color: var(--ink-3); }
```

The chip has no enter animation: it appears with the row's own change, which already explains
the cause. (`--t-fact` and `--t-fact-lh` are task 26's pair, `task-26.md:19-22`.)

**The refusal on the row.** `ReplicaWrites` gains `failedOf(issueId): {seq: number; code: string;
message: string} | null`, set by step 5 and cleared by the next op on that issue. Task 40's
`IssueRow.tsx` renders it as `data-fail={seq}` plus one fact line, `refused · policy_refused ·
Priority is required to leave Triage`, in `--fail` mono. The flash reuses task 41's mechanism
(`data-flash={seq}` restarts one `opacity` animation over `--dur-flash`, `task-41.md:49-52`) with
`--fail-tint` (`tokens.css:80`) as its ground. Two more files join the list:
- `desktop/src/components/work/IssueRow.tsx` ⚑ (task 40) — `data-fail` and the refusal line (modified)
- `desktop/src/components/work/work.css` ⚑ — `[data-fail]` flashes `--fail-tint` (modified)

#### Tests to write first, part 1 — the harness, in `desktop/src/work/replica/mutate.test.ts`

Task 55's `ReplicaDeps` (`listAll`, `changes`, `fetchSubjects`) comes from its scope; its part
file did not exist when this was written, so the harness's read half is **UNVERIFIED** against it.
The write half is this task's own. Every write is a gate the test opens or refuses, so "before the
Hub answers" is a state the test can stand in, not a race.

```ts
import { describe, expect, it, vi } from "vitest";
import { ApiError, type ProblemCode } from "../../api/client";
import type { IssueHead, WorkClient } from "../../api/work";
import { ALICE, WORK_IDS, makeIssueHead, makeIssueRevision } from "../../test/workFixtures";
import { createReplica, type Replica } from "./replica";
import { createUndoLedger } from "./undo";

interface Call { method: string; issueId: string; body: unknown; key: string }
interface Gate { resolve(v: unknown): void; reject(e: unknown): void }

/** A WorkClient whose every write waits for the test. `keyed(k)` stamps k on each call. */
function fakeWork() {
  const calls: Call[] = [];
  const gates: Gate[] = [];
  const hold = (key: string, method: string) => (issueId: string, ...rest: unknown[]) =>
    new Promise((resolve, reject) => {
      calls.push({ method, issueId, body: rest.length === 1 ? rest[0] : rest, key });
      gates.push({ resolve, reject });
    });
  const METHODS = ["patchIssue", "moveIssueState", "writeIssueRevision", "addRelation",
    "removeRelation", "createComment", "react", "resolveComment", "subscribe"] as const;
  const keyed = (key: string) =>
    Object.fromEntries(METHODS.map((m) => [m, hold(key, m)])) as unknown as WorkClient;
  return { work: { keyed }, calls, gates };
}

const refusal = (code: string, message: string) => new ApiError(code as ProblemCode, message, 422);
const EP142 = WORK_IDS.ep142;

async function harness(heads: IssueHead[] = [makeIssueHead({ id: EP142, priority: "none" })]) {
  let now = 0;
  const clock = () => now;
  const undo = createUndoLedger(clock, 6000);
  const hub = fakeWork();
  const fetched = vi.fn(async (ids: readonly string[]) => heads.filter((h) => ids.includes(h.id))
    .map((head) => ({ head, revision: makeIssueRevision({ issueId: head.id }), comments: [] })));
  const r: Replica = createReplica({
    listAll: async () => ({ heads, revisions: heads.map((h) => makeIssueRevision({ issueId: h.id })),
      comments: [], cursor: "c-0" }),
    changes: async () => ({ events: [], next: "c-0", more: false }),
    fetchSubjects: fetched,
    work: hub.work,
    undo,
  });
  await vi.waitFor(() => expect(r.status).toBe("ready"));
  return { r, hub, undo, fetched, tick: (ms: number) => { now += ms; } };
}
```

`WorkSnapshot`'s field names (`heads`, `revisions`, `comments`, `cursor`) and the `WorkRecord`
shape `fetchSubjects` returns are task 55's; the names above are a guess to be aligned when its
part lands (**UNVERIFIED**). `makeIssueRevision({issueId})` uses task 20's factory
(`task-20.md:242`) over its `IssueRevision.issueId` (`task-20.md:187-188`).

#### Tests to write first, part 2 — paint, rollback, refusal, key (`mutate.test.ts`)

```ts
describe("mutate paints first and takes back only its own fields", () => {
  it("repaints the row before the request resolves", async () => {
    const { r, hub } = await harness();
    r.mutate({ kind: "setPriority", issueId: EP142, priority: "high" });
    expect(r.get(EP142)?.head.priority).toBe("high");        // same turn, no await
    expect(hub.calls).toHaveLength(1);
    expect(hub.gates).toHaveLength(1);                        // still unanswered
  }); // kills: paint-after-server (the row changes only when the Hub answers)

  it("a refusal after a teammate's change reverts only the op's field", async () => {
    const { r, hub } = await harness();
    const h = r.mutate({ kind: "setPriority", issueId: EP142, priority: "high" });
    r.__applyForTest({ ...r.get(EP142)!.head, priority: "high", assignee: ALICE }); // teammate
    hub.gates[0].reject(refusal("policy_refused", "Priority is required to leave Triage"));
    expect(await h.settled).toMatchObject({ code: "policy_refused" });
    expect(r.get(EP142)?.head.priority).toBe("none");         // our field went back
    expect(r.get(EP142)?.head.assignee).toEqual(ALICE);       // hers stayed
  }); // kills: whole-record snapshot restore

  it("a refused labels op removes only its own label", async () => {
    const { r, hub } = await harness([makeIssueHead({ id: EP142, labels: [] })]);
    r.mutate({ kind: "labels", issueId: EP142, add: ["ui"], remove: [] });
    r.__applyForTest({ ...r.get(EP142)!.head, labels: ["ui", "api"] });
    hub.gates[0].reject(refusal("policy_refused", "no"));
    await vi.waitFor(() => expect(r.get(EP142)?.head.labels).toEqual(["api"]));
  }); // kills: restoring the whole label list

  it("shows the Hub's code and message and fires onRefused exactly once", async () => {
    const { r, hub } = await harness();
    const heard = vi.fn();
    r.onRefused(heard);
    r.mutate({ kind: "setPriority", issueId: EP142, priority: "high" });
    hub.gates[0].reject(refusal("policy_refused", "Priority is required to leave Triage"));
    await vi.waitFor(() => expect(heard).toHaveBeenCalledTimes(1));
    expect(heard.mock.calls[0][0]).toMatchObject({ code: "policy_refused",
      message: "Priority is required to leave Triage", issueId: EP142 });
    expect(r.failedOf(EP142)).toMatchObject({ code: "policy_refused" });
    await new Promise((t) => setTimeout(t, 0));
    expect(heard).toHaveBeenCalledTimes(1);
  }); // kills: a silent rollback; kills: a listener fired per re-render

  it("a retry after unavailable reuses the op's key", async () => {
    const { r, hub } = await harness();
    r.mutate({ kind: "setPriority", issueId: EP142, priority: "high" });
    hub.gates[0].reject(new ApiError("backend_unavailable", "down", 503));
    await vi.waitFor(() => expect(r.pendingOf(EP142)).toBe("not_sent"));
    expect(r.get(EP142)?.head.priority).toBe("high");         // not rolled back
    expect(r.queued()).toBe(1);
    r.resume();
    await vi.waitFor(() => expect(hub.calls).toHaveLength(2));
    expect(hub.calls[1].key).toBe(hub.calls[0].key);
  }); // kills: a fresh key per attempt (duplicate writes); kills: rolling back on an outage

  it("two quick ops on one issue send one after the other", async () => {
    const { r, hub } = await harness([makeIssueHead({ id: EP142, labels: [] })]);
    r.mutate({ kind: "labels", issueId: EP142, add: ["ui"], remove: [] });
    r.mutate({ kind: "labels", issueId: EP142, add: ["api"], remove: [] });
    expect(hub.calls).toHaveLength(1);                        // the second waits its turn
    hub.gates[0].resolve(makeIssueHead({ id: EP142, labels: ["ui"] }));
    await vi.waitFor(() => expect(hub.calls).toHaveLength(2));
    expect(hub.calls[1].body).toEqual({ labels: ["ui", "api"] });
    expect(r.get(EP142)?.head.labels).toEqual(["ui", "api"]); // the answer kept the 2nd paint
  }); // kills: parallel sends that race; kills: the first answer erasing the second paint
});
```

`__applyForTest(head)` stands in for a feed event plus its subject fetch: task 55's apply path,
exposed only under `import.meta.vitest`. If task 55 prints its own seam for this, use that instead;
the name is this part's (**UNVERIFIED** against task 55).

#### Tests to write first, part 3 — undo, batch, conflict, the closed union (`mutate.test.ts`)

```ts
describe("undo is a new write; a batch is one entry; a conflict keeps the words", () => {
  it("U sends the inverse op to the Hub under a new key", async () => {
    const { r, hub } = await harness();
    r.mutate({ kind: "setPriority", issueId: EP142, priority: "high" });
    hub.gates[0].resolve(makeIssueHead({ id: EP142, priority: "high" }));
    await vi.waitFor(() => expect(r.pendingOf(EP142)).toBeNull());
    expect(r.undoLatest()).toBe(true);
    await vi.waitFor(() => expect(hub.calls).toHaveLength(2));
    expect(hub.calls[1]).toMatchObject({ method: "patchIssue", body: { priority: "none" } });
    expect(hub.calls[1].key).not.toBe(hub.calls[0].key);
    expect(r.get(EP142)?.head.priority).toBe("none");         // painted before the answer too
  }); // kills: a local-only undo (the Hub never hears it); kills: reusing the first op's key

  it("a three-issue batch is one ledger entry whose undo reverts all three", async () => {
    const ids = [WORK_IDS.ep150, WORK_IDS.ep160, WORK_IDS.ep161];
    const { r, hub, undo } = await harness(ids.map((id) => makeIssueHead({ id, priority: "none" })));
    const h = r.mutate(ids.map((issueId) => ({ kind: "setPriority" as const, issueId, priority: "low" as const })));
    expect(undo.entries()).toHaveLength(1);
    ids.forEach((id, i) => hub.gates[i].resolve(makeIssueHead({ id, priority: "low" })));
    await h.settled;
    h.undo();
    await vi.waitFor(() => expect(hub.calls).toHaveLength(6));
    expect(hub.calls.slice(3).map((c) => c.body)).toEqual(ids.map(() => ({ priority: "none" })));
  }); // kills: one chip per op; kills: undo reverting only the batch's first op

  it("after the window, undo sends nothing", async () => {
    const { r, hub, tick, undo } = await harness();
    const h = r.mutate({ kind: "setPriority", issueId: EP142, priority: "high" });
    hub.gates[0].resolve(makeIssueHead({ id: EP142, priority: "high" }));
    await h.settled;
    tick(6001); undo.lapse();
    h.undo();
    expect(r.undoLatest()).toBe(false);
    expect(hub.calls).toHaveLength(1);
  }); // kills: an undo that never expires (the revision history is the way back, task 47)

  it("a move out of a fact state and a comment get no undo entry", async () => {
    const { r, undo } = await harness([makeIssueHead({ id: EP142, state: "in_review" })]);
    const move = r.mutate({ kind: "setState", issueId: EP142,
      move: { to: "todo", cause: "hand", reason: "", landCommit: "", evidence: [] } });
    const say = r.mutate({ kind: "comment", issueId: EP142, localId: "lc-1",
      req: { body: { text: "Looks right", author: "H" }, parentId: null, anchor: null, mentions: [] } });
    expect([move.undoable, say.undoable]).toEqual([false, false]);
    expect(undo.entries()).toHaveLength(0);
  }); // kills: offering an undo the Hub's MoveAllowed would refuse (decision 6)

  it("a revision_conflict keeps the person's text for the new head", async () => {
    const { r, hub, fetched } = await harness();
    const patch = { title: { text: "Resume an upload after a drop", author: "H" as const } };
    const h = r.mutate({ kind: "revise", issueId: EP142, baseRev: 1, patch });
    hub.gates[0].reject(new ApiError("revision_conflict" as ProblemCode, "Revision 2 is newer", 409));
    expect(await h.settled).toMatchObject({ code: "revision_conflict" });
    expect(fetched).toHaveBeenCalledWith([EP142]);
    expect(r.draftAgain(EP142)).toMatchObject({ issueId: EP142, baseRev: 1, patch });
  }); // kills: losing a person's words on a conflict

  it("irreversible acts are not ops", () => {
    // @ts-expect-error erase is confirm.inplace (task 44), never a WorkOp
    const bad: import("./ops").WorkOp = { kind: "erase", issueId: EP142 };
    // @ts-expect-error land is a fact the Hub records, never a hand op
    const land: import("./ops").WorkOp = { kind: "land", issueId: EP142 };
    expect([bad, land]).toHaveLength(2);
  }); // kills: widening WorkOp with an act that has no inverse
});
```

The `StateMove` and `CreateCommentRequest` literals follow task 37's types (`task-37.md:197-199`,
`:205-207`), and the hand move copies that part's own `MOVE` fixture (`task-37.md:712`).

#### Tests to write first, part 4 — the window (`undo.test.ts`, `UndoChip.test.tsx`)

```ts
// desktop/src/work/replica/undo.test.ts
import { describe, expect, it } from "vitest";
import { createUndoLedger } from "./undo";

describe("the undo ledger counts unpaused time only", () => {
  const entry = { id: "op-1", ops: [], before: [], issueIds: ["is-1"] };
  it("hover and focus pause it; it lapses only after 6 s unpaused", () => {
    let now = 0;
    const l = createUndoLedger(() => now, 6000);
    l.push(entry);
    now = 2000; l.pause("op-1", "hover", true);
    now = 20000; expect(l.lapse()).toEqual([]);              // 18 s hovered: still live
    l.pause("op-1", "focus", true); l.pause("op-1", "hover", false);
    now = 30000; expect(l.remainingMs("op-1")).toBe(4000);   // focus alone still pauses
    l.pause("op-1", "focus", false);
    now = 33999; expect(l.lapse()).toEqual([]);
    now = 34000; expect(l.lapse()).toEqual(["op-1"]);
  }); // kills: an unpausing countdown; kills: hover-off resuming while focus still holds
  it("latestLive is the newest live entry, and take removes it", () => {
    let now = 0;
    const l = createUndoLedger(() => now, 6000);
    l.push(entry); now = 10; l.push({ ...entry, id: "op-2" });
    expect(l.latestLive()?.id).toBe("op-2");
    expect(l.take("op-2")?.id).toBe("op-2");
    expect(l.latestLive()?.id).toBe("op-1");
  }); // kills: U undoing the oldest op instead of the latest
});
```

```tsx
// desktop/src/components/common/UndoChip.test.tsx
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { createUndoLedger } from "../../work/replica/undo";
import { expectNoPrediction } from "../../test/expectNoPrediction";
import { UndoChip } from "./UndoChip";

beforeEach(() => { vi.useFakeTimers(); vi.setSystemTime(0); });
afterEach(() => { vi.useRealTimers(); });

function mount(issueIds: string[]) {
  const ledger = createUndoLedger(() => Date.now(), 6000);
  ledger.push({ id: "op-1", ops: [], before: [], issueIds });
  const onUndo = vi.fn();
  const view = render(<>
    {issueIds.map((id) => <div key={id} data-issue-id={id} tabIndex={0}>{id}</div>)}
    <UndoChip ledger={ledger} entryId="op-1" label="Priority High" onUndo={onUndo} />
  </>);
  return { ledger, onUndo, view };
}

it("a hovered row's chip does not lapse at 6 s, and lapses after the hover ends", () => {
  const { view } = mount(["is-1"]);
  act(() => { vi.advanceTimersByTime(1000); });
  fireEvent.pointerOver(screen.getByText("is-1"));
  act(() => { vi.advanceTimersByTime(9000); });              // 10 s wall, 1 s unpaused
  expect(screen.getByRole("button", { name: "Undo" })).toBeTruthy();
  fireEvent.pointerOut(screen.getByText("is-1"));
  act(() => { vi.advanceTimersByTime(5000); });              // 6 s unpaused: the beat lapses it
  act(() => { vi.advanceTimersByTime(1000); });
  expect(screen.queryByRole("button", { name: "Undo" })).toBeNull();
  expectNoPrediction(view.container);
}); // kills: an unpausing countdown

it("a three-issue batch is one chip, and its Undo calls onUndo once", () => {
  const { onUndo } = mount(["is-1", "is-2", "is-3"]);
  expect(screen.getAllByRole("button", { name: "Undo" })).toHaveLength(1);
  fireEvent.click(screen.getByRole("button", { name: "Undo" }));
  expect(onUndo).toHaveBeenCalledTimes(1);
}); // kills: per-op chips

it("the seconds are tabular and whole", () => {
  mount(["is-1"]);
  act(() => { vi.advanceTimersByTime(2000); });
  const left = document.querySelector(".undo-chip-left")!;
  expect(left.textContent).toBe("4s");
}); // kills: a fractional or ms countdown
```

The seconds count is a remaining time, not a completion prediction: it is a fixed window the person
controls, which is why `expectNoPrediction` (rules `percentage`, `n-of-m`, `eta`,
`task-20.md:60`) passes on it. If task 20's `eta` rule matches `4s`, **that is a conflict to raise,
not to route around**: say so in the task's return.

#### Steps

- [ ] Make the `api/work.ts` and `api/client.ts` deltas; task 37's tests still pass, and one new
      case in `client.test.ts` shows `send(..., {idempotencyKey: "op-1"})` sets the header.
- [ ] Write `ops.test.ts` for `fieldsOf`, `undoable` and `inverseOf` (one case per `WorkOp`
      kind, plus the per-label and per-relation members); write `ops.ts` until it passes.
- [ ] Write `mutate.test.ts` (parts 1-3); it fails to find `mutate`. Add the write half to
      `replica.ts` in the six numbered steps above. Mutate by hand and watch the named test go red,
      then revert: paint inside the `.then` (test 1); `revertLocal` writing every field of the
      record (test 2); restore the whole label list (test 3); call listeners from a `useEffect`
      (test 4); `makeId("op")` inside `sendOp` (test 5); send both label ops at once (test 6);
      apply the inverse locally without sending (undo test 1).
- [ ] Write `undo.test.ts` and `undo.ts`; then `UndoChip.test.tsx`, `UndoChip.tsx` and the CSS.
      Mutant: drop the `pausedSince` branch in `remainingMs`; both window tests go red.
- [ ] Write `undoVerb.ts` against task 53's `registerVerb`; if task 53 has not landed, keep the file
      and skip its registration test with a `test.todo` naming task 53 (never a fake keymap).
- [ ] Add `data-fail` and the refusal line to `IssueRow.tsx`; append the `[data-fail]` rule.
- [ ] Commit each file as it goes green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test -- src/work/replica src/components/common/UndoChip.test.tsx'
# expected: ops, mutate, undo and UndoChip suites passing; 13 named cases in mutate and undo
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop test'
# expected: every suite passing, tasks 37, 40 and 55 included
~/.claude/tools/claudezsh/claudezsh 'npm --prefix desktop run typecheck'
# expected: exit 0; the two @ts-expect-error lines are used (an unused one is an error)
~/.claude/tools/claudezsh/claudezsh 'grep -rnE "\.(patchIssue|moveIssueState|writeIssueRevision|createComment|resolveComment|addRelation|removeRelation)\(" desktop/src --include=*.ts --include=*.tsx'
# expected: matches only in src/work/replica/ops.ts and src/api/ (one write path)
```

#### What this task does NOT claim

- **An offline queue that survives a restart** (L-230, a SKIP): the queue is in memory.
- **The undo announcement** in the live region: task 66 listens to `onRefused` and the ledger.
- **Start's *Undo Start*** chip: task 61, on `undoStart` (`task-37.md`), not on this ledger.
- **`budget.paint ≤ 16 ms`**: recorded by task 70's timing run, not asserted here (05 §4.6).
- **End-to-end dedupe of a desktop retry** at the Hub: it needs task 16's relay to forward the
  inbound key (decision 3); until then the key stops at the loopback.
- **The editor's offer of `draftAgain`** on the new head: task 49 reads it.

#### Gaps found while writing this part

1. The skeleton's file map omits `UndoChip.tsx`, `undoVerb.ts`, `mutate.test.ts` and the
   `api/work.ts` + `api/client.ts` deltas for the `Idempotency-Key` option.
2. Task 16's relay mints its own key per call (`task-16.md:844`) and does not forward an inbound
   one; decision 3 needs it to.
3. Task 55's part was not on the tree: `ReplicaDeps`' snapshot names and the `__applyForTest` seam
   are proposals, and task 55 wins.
4. Task 53's `registerVerb` signature and the `optimistic.undo` verb id are the scope's words.
