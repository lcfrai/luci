### Task 20: Desktop — `4-D1` on session start and restart

When `POST /v1/sessions` or `POST …/restart` answers `409 brief_not_written`, the session did not
start (task 10 guarantees nothing was recorded). The person sees **why**, a **Retry**, and one
deliberate **Start without the brief** (prototype state 10). Today that 409 would become a toast
with the server's sentence (`useCreateSession.ts:80-83` at `3303fd6`), which is the silent path
`4-D1` exists to prevent.

**The six ways a session starts** (review I9). Each one reaches `POST /v1/sessions` or
`POST …/restart`, so each one can be refused, and each one's `catch` must route the refusal. Lines
are at `974ed4d`; re-locate each by its symbol before editing.

| # | Call site (symbol) | The request it sends | Its `catch` today | The attempt it records |
|---|---|---|---|---|
| 1 | `useCreateSession` (`hooks/useCreateSession.ts:69`). Every "new session" control goes through it: `NewSessionButton`, `WorktreeRow`, `ContextMenu`, `SessionPicker`, `useKeyboardMap`, and the menu's shell verb (`sessionVerbs.ts:459`) | `api.createSession` | `pushToast(sentenceFor(…))`, `:80-83` | `create`, `origin: "new-session"` |
| 2 | `startSessionOnWorktree` (`components/develop/EmptyDevelop.tsx:42`), the empty state's two buttons. It stays outside the hook, as its own comment (`:16-25`) requires. | `api.createSession` | `pushToast(sentenceForCreate(…))`, `:46-48` | `create`, `origin: "empty-develop"` |
| 3 | the menu's Restart (`components/develop/sessionVerbs.ts:437`, the `runImmediate` commit) | `api.restartSession` | `rollback(); report(…)`, `:205-207` | `restart` |
| 4 | the exit bar's Restart (`restartSession`, `components/develop/SessionPane.tsx:31-40`) | `api.restartSession` | `pushToast(sentenceForRestart(…))`, `:38` | `restart` |
| 5 | Resume (`sessionVerbs.ts:317`) | `api.resumeSession`, which builds its create request **inside** `client.ts` (`:342-361`) | `report(…)`, `:321` | `resume` |
| 6 | Fork (`sessionVerbs.ts:353`) | `api.forkSession`, which builds its create request inside `client.ts` (`:362-383`) | `report(…)`, `:357` | `fork` |

Resume and fork build their request inside the client, so the desktop never holds a
`CreateSessionRequest` for them. Retry for those two therefore re-calls the verb, with the same
arguments, instead of re-sending a stored body. Task 15 gives both verbs the `{withoutBrief}`
option this needs.

**Read `docs/DESIGN-SYSTEM.md` first.** Held here: the refusal is **answerable in place**
(AGENTIC-UX principle, "each decision is answerable in place, without navigating") — an inline
surface anchored to the control that started it, not a modal and not a toast; **Start without the
brief** is a secondary control, never the default focus, because going on without the brief must
be deliberate; `--need` with its **triangle** shape marks the refusal (fact sheet §10); elevation,
radius and motion as task 17. The owner's eyes are the visual acceptance (`OD-12`).

**Files:**
- `desktop/src/discover/StartWithoutBrief.tsx` + `StartWithoutBrief.test.tsx` (new)
- `desktop/src/discover/startAttempt.ts` + `startAttempt.test.ts` (new) — `routeBriefRefusal`
  and `replayStart`, the one place a refusal is recorded and the one place it is re-sent
- `desktop/src/hooks/useCreateSession.ts` — call site 1 (modify)
- `desktop/src/components/develop/EmptyDevelop.tsx` — call site 2; mounts the refusal beside its
  two buttons (modify)
- `desktop/src/components/develop/sessionVerbs.ts` — call sites 3, 5, 6; `runImmediate` gains the
  optional `refusal` (modify)
- `desktop/src/components/develop/SessionPane.tsx` — call site 4; mounts the refusal for the
  session-bound attempts (modify)
- `desktop/src/store/slices/discoverSlice.ts` — the pending refusal (modify, task 16's)
- Tests, one per call site: `hooks/useCreateSession.test.ts` (new),
  `components/develop/EmptyDevelop.test.tsx` (new), `components/develop/sessionVerbs.test.ts`
  and `components/develop/SessionPane.test.tsx` (both exist; modify)

**Interfaces — complete source:**

```ts
// desktop/src/store/slices/discoverSlice.ts — additions
export type BriefReason =
  | "hub_unavailable" | "signed_out" | "write_failed" | "block_broken" | "tracked_elsewhere"
  | "project_unknown"; // task 10's closed set, in the same order (review I2)

/** What to re-send, one arm per kind of start (review I9). `worktreeId` is the worktree the new
 *  session would run in, which the `write_failed` row names. */
export type StartAttempt =
  /** Call sites 1 and 2. `request` is the body the person's click sent, so Retry re-sends it
   *  unchanged and nothing is re-resolved against today's defaults. `origin` says which surface
   *  renders the refusal, so the empty state and the new-session control never both show it. */
  | { kind: "create"; worktreeId: WorktreeId; origin: "new-session" | "empty-develop";
      request: CreateSessionRequest }
  /** Call sites 3 and 4. The restart keeps the session id, so the id is all a replay needs. */
  | { kind: "restart"; worktreeId: WorktreeId; sessionId: SessionId }
  /** Call site 5. The client builds the create body from the source session's live record. */
  | { kind: "resume"; worktreeId: WorktreeId; sourceId: SessionId; harness: Harness }
  /** Call site 6. `worktreeId` equals `targetWorktreeId`: the fork runs in the target. */
  | { kind: "fork"; worktreeId: WorktreeId; sourceId: SessionId; targetWorktreeId: WorktreeId;
      harness: Harness };

export type PendingStart = StartAttempt & {
  reason: BriefReason;
  message: string; // the server's sentence, verbatim
  /** True when the refused request already carried withoutBrief: true. The override is then
   *  not offered again: it would re-send the request that was just refused (re-review I2). */
  withoutBrief: boolean;
};

// in DiscoverSlice:
  pendingStart: PendingStart | null;
  setPendingStart(p: PendingStart | null): void;
```

```ts
// desktop/src/discover/startAttempt.ts
/** Every one of the six call sites calls this FIRST in its catch. It returns true when `error`
 *  is a 409 `brief_not_written`. It then records the refusal, and the caller shows nothing
 *  else: no toast, no `setError`. It returns false for every other error, and the caller's
 *  phase 2 path runs unchanged. `sentWithoutBrief` is what the refused request carried. It is
 *  false at all six call sites, which never send the override on a first attempt. */
export function routeBriefRefusal(
  error: ApiError, attempt: StartAttempt, sentWithoutBrief: boolean,
): boolean;

/** Retry calls `replayStart(pending, pending.withoutBrief)`; the override calls
 *  `replayStart(pending, true)`. A `switch (pending.kind)` with no `default:`, so a fifth kind
 *  fails with TS2366:
 *    create  → api.createSession({ ...request, withoutBrief })
 *    restart → api.restartSession(sessionId, { withoutBrief })
 *    resume  → api.resumeSession(source, harness, { withoutBrief })
 *    fork    → api.forkSession(source, targetWorktreeId, harness, { withoutBrief })
 *  `source` is `selectSession(sourceId)` at the moment of the replay. When it is gone (purged),
 *  nothing is sent: the refusal clears and one toast says "The session this started from is no
 *  longer in the cockpit."
 *  On success it clears `pendingStart` and upserts the session. For every kind except restart it
 *  also selects the new session's worktree and focuses the session. A restart keeps its id, so
 *  focus is not moved. A second `brief_not_written` replaces the refusal through
 *  `routeBriefRefusal`. Any other error clears the refusal and toasts through this module's own
 *  `sentenceForReplay`: a `switch (classify(error))` with no `default:`. The prose is local
 *  because its tail names this act (`problems.ts:3-8`). */
export function replayStart(pending: PendingStart, withoutBrief: boolean): Promise<void>;
```

**Where the refusal renders.** Each arm renders in exactly one place, anchored to the control that
started it:
- `create`, `origin: "new-session"`: where the new-session control renders.
- `create`, `origin: "empty-develop"`: in `EmptyDevelop`, beside its two buttons.
- `restart`, `resume` and `fork`: in the `SessionPane` of `sessionId` or `sourceId`, above its
  exit bar or header. The session menu closes before its verb runs (`act`, `sessionVerbs.ts:303-306`),
  so the menu itself has nothing left to anchor to.

```ts
// desktop/src/discover/StartWithoutBrief.tsx
/** Rendered by the surface that started the session, anchored to it (see "Where the refusal
 *  renders"). Retry is replayStart(pending, pending.withoutBrief): the identical request, or for
 *  resume and fork the identical verb call. "Start without the brief" is
 *  replayStart(pending, true), and the
 *  session's chip then reads "started without the brief" (P4-C) — the server stamps
 *  none:declined; the desktop never writes it. */
export function StartWithoutBrief(props: { pending: PendingStart }): JSX.Element;
```

**Reason → sentence and extra control** (the server's `message` is shown first, verbatim):

| `details.reason` | Extra |
|---|---|
| `hub_unavailable` | — |
| `signed_out` | *Sign in*, pointing at the rail footer (the Hub's task C4) |
| `write_failed` | the worktree name |
| `block_broken` | `withoutBrief: false`: *The managed block in CLAUDE.md is damaged. Open the file and remove the marker lines, or start without the brief.* `withoutBrief: true`: *The managed block in CLAUDE.md is damaged, so the old brief cannot be removed. Open the file and remove the marker lines, then retry.* |
| `tracked_elsewhere` | *This repository tracks both CLAUDE.md files, so the brief cannot be written without changing a tracked file.* (OQ-2) |
| `project_unknown` | *The projects could not be read, so it is not known which brief this worktree inherits.* (review I2). The server raises it only when the request did **not** carry `withoutBrief: true` (task 10: the override does not need a project), so it has no `withoutBrief: true` variant, and *Start without the brief* stays on offer. |

When `pendingStart.withoutBrief` is true, **Start without the brief** is not rendered for any
reason; only **Retry** remains. Task 09 reports a broken marker as `block_broken` on the override's
strip too, so the person is told the real cause and the real repair.

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `a brief_not_written create does not toast` | the 409 sets `pendingStart`; `pushToast` is not called | 4-D1's refusal reduced to a sentence that fades |
| `retry re-sends the identical request` | the second `createSession` body equals the first | a retry that drops the person's harness or prompt |
| `start without the brief sends withoutBrief` | the body adds `withoutBrief: true` and nothing else changes | an override that also drops options |
| `the override is not the default focus` | after render, `document.activeElement` is Retry | a going-on-without made the easy path |
| `restart routes the same way` | a 409 from `restartSession` sets `kind: "restart"`; the override calls `restartSession(id, {withoutBrief: true})` | restart silently starting without the brief |
| **Call site 1** (`useCreateSession.test.ts`): `the new-session hook routes the refusal` | a 409 `brief_not_written` → `pendingStart` is `{kind: "create", origin: "new-session", request}` with `request` equal to the body sent; `pushToast` and `setError` are not called | call site 1 still toasting (review I9) |
| **Call site 2** (`EmptyDevelop.test.tsx`): `the empty state's start routes the refusal` | `startSessionOnWorktree` answered by the 409 → `pendingStart` is `{kind: "create", origin: "empty-develop", request}` with the body sent; no toast; the refusal renders in `EmptyDevelop` and not at the new-session control | the second `api.createSession` call site left on `sentenceForCreate` (review I9) |
| **Call site 3** (`sessionVerbs.test.ts`): `the menu's restart rolls back and routes the refusal` | the Restart verb answered by the 409 → the optimistic `starting` is rolled back to the recorded session; `pendingStart` is `{kind: "restart", sessionId}`; `pushToast` is not called | `runImmediate`'s catch reporting before the refusal is seen |
| **Call site 4** (`SessionPane.test.tsx`): `the exit bar's restart routes the refusal` | the exit bar's Restart answered by the 409 → `pendingStart` is `{kind: "restart", sessionId}`; `sentenceForRestart` does not toast; the refusal renders in that pane | the exit bar's own `restartSession` still toasting (review I9) |
| **Call site 5** (`sessionVerbs.test.ts`): `resume routes the refusal and replays the verb` | Resume answered by the 409 → `pendingStart` is `{kind: "resume", sourceId, harness}`; Retry calls `resumeSession(source, harness, {withoutBrief: false})` and the override calls it with `{withoutBrief: true}`, `source` read from the store | resume refused with no way to re-send, because the desktop never held its request (review I9) |
| **Call site 6** (`sessionVerbs.test.ts`): `fork routes the refusal and replays the verb` | Fork answered by the 409 → `pendingStart` is `{kind: "fork", sourceId, targetWorktreeId, harness}` with `worktreeId` equal to the target; Retry and the override call `forkSession(source, targetWorktreeId, harness, {withoutBrief})` | fork refused into a toast, or replayed into the source's worktree |
| `a replay whose source is gone sends nothing` (`startAttempt.test.ts`) | a `resume` refusal whose `sourceId` was purged → Retry sends no request, clears `pendingStart` and toasts the one sentence | a replay that throws on `undefined`, or sends a body built from nothing |
| `each reason shows its extra` — table | the six rows above, `block_broken` in both variants | one generic sentence for six different fixes |
| `an unknown project offers the override` | a `project_unknown` 409 on a create and on a restart → the `project_unknown` sentence, **Retry** focused, **Start without the brief** present; the override re-sends with `withoutBrief: true` (review I2) | the new reason falling through to a toast or a generic sentence; the one recovery that works when the projects cannot be read left off |
| `a refused override offers no second override` | a `block_broken` 409 answering a request that carried `withoutBrief: true` (create and restart) → `pendingStart.withoutBrief` is true; **Start without the brief** is absent; the sentence is the `withoutBrief: true` variant and does not contain "start without the brief"; Retry re-sends `withoutBrief: true` | a recovery offered that re-sends the request just refused (re-review I2) |
| `other errors keep phase 2's behaviour` | a `backend_unavailable` still toasts through `sentenceFor` | a phase 2 regression |

**Steps:**

- [ ] **Step 1:** Re-locate the six call sites of the table above by symbol, then run
  `grep -rn "api\.\(createSession\|restartSession\|resumeSession\|forkSession\)" desktop/src --include=*.ts --include=*.tsx`
  and confirm that every hit outside `api/` and outside tests is one of the six. A seventh is a
  finding: stop and record it in the task report before going on.
- [ ] **Step 2:** Write the tests; confirm they fail.
- [ ] **Step 3:** The slice additions, `startAttempt.ts`, the component. Then the six routing
  edits: each `catch` calls `routeBriefRefusal(error, attempt, false)` first and returns when it
  answers true. In `runImmediate`, the optional `refusal?: StartAttempt` is checked after
  `rollback()` and before `report`. Mount `StartWithoutBrief` in the three places "Where the
  refusal renders" names, each filtering `pendingStart` to its own arm.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec. **Do not merge** (`OD-1`).

**Acceptance:**

```
$ cd desktop && npx vitest run && npx tsc --noEmit
(all pass; tsc silent)
```

Owner-observed, **unverified until the batched session** (`OD-12`): the refusal's placement and
wording.

**Does NOT claim:** that `OQ-3` (remember the override per branch) is answered — this task asks
every time, as recommended.
