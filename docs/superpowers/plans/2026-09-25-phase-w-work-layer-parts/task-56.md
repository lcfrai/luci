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
