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
