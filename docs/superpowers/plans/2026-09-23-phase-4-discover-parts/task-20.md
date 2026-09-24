### Task 20: Desktop — `4-D1` on every session start (six paths)

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
- `desktop/src/discover/startAttempt.ts` + `startAttempt.test.ts` (new) — `routeBriefRefusal`,
  `sendAttempt` and `replayStart`, the one place a refusal is recorded and the one place it is
  re-sent; `anchorKey` and `useRefusalHost`, which say where it renders
- `desktop/src/hooks/useCreateSession.ts` — call site 1 (modify)
- `desktop/src/components/topbar/TopBar.tsx` — the `new-session` home and the fallback for every
  other home that is not mounted (modify; re-review b07-1 R-I1)
- `desktop/src/components/develop/EmptyDevelop.tsx` — call site 2; hosts `empty:<target.id>` and
  mounts the refusal beside its two buttons (modify)
- `desktop/src/components/develop/sessionVerbs.ts` — call sites 3, 5, 6; `runImmediate` gains the
  optional `refusal` (modify)
- `desktop/src/components/develop/SessionPane.tsx` — call site 4; hosts `pane:<sessionId>` and
  mounts the refusal for the session-bound attempts (modify)
- `desktop/src/store/slices/discoverSlice.ts` — the pending refusal (modify, task 16's)
- Tests, one per call site: `hooks/useCreateSession.test.ts` (new),
  `components/develop/EmptyDevelop.test.tsx` (new), `components/develop/sessionVerbs.test.ts`
  and `components/develop/SessionPane.test.tsx` (both exist; modify). The fallback's test goes in
  `components/AppShell.test.tsx` (exists; modify), because only the whole shell mounts the rail,
  the lens and `TopBar` together.

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
  /** How many mounted homes hold each anchor key (see "Where the refusal renders"). Not
   *  persisted: `partialize` is unchanged (plan §5), so a reload starts every count at zero. */
  refusalHosts: Record<string, number>;
  hostRefusal(key: string): void;    // +1
  releaseRefusal(key: string): void; // -1; the key is deleted at zero
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

/** The one per-kind send. LOAD-BEARING SHAPE: a `switch (pending.kind)` with no `default:` and a
 *  declared return type that excludes `undefined`, so a fifth kind fails to compile with TS2366.
 *  (`replayStart` returns `Promise<void>`, where an unhandled arm would fall off the end
 *  silently, so the switch lives here and not there. Probed with the desktop's TypeScript 6.0.3,
 *  `--strict`: an unhandled fifth kind gives TS2366; all four handled compiles clean. Re-review
 *  b07-1 R-M1.) `opts` is `withoutBrief ? { withoutBrief: true } : undefined`, so a retry of a
 *  first refusal sends exactly the first request, with no `withoutBrief: false` key (R-M2):
 *    create  → api.createSession(withoutBrief ? { ...request, withoutBrief: true } : request)
 *    restart → api.restartSession(sessionId, opts)
 *    resume  → api.resumeSession(source, harness, opts)
 *    fork    → api.forkSession(source, targetWorktreeId, harness, opts)
 *  `source` is `selectSession(sourceId)(useCockpit.getState())` (the selector is curried,
 *  `selectors.ts:52-54`), read at the moment of the replay. When it is gone (purged), nothing is
 *  sent and this returns `null`. */
export function sendAttempt(pending: PendingStart, withoutBrief: boolean): Promise<Session | null>;

/** Retry calls `replayStart(pending, pending.withoutBrief)`; the override calls
 *  `replayStart(pending, true)`. It awaits `sendAttempt`. On `null` (the source is gone), the
 *  refusal clears and one toast says "The session this started from is no longer in the
 *  cockpit."
 *  On success it clears `pendingStart` and upserts the session. For every kind except restart it
 *  also selects the new session's worktree and focuses the session. A restart keeps its id, so
 *  focus is not moved. A second `brief_not_written` replaces the refusal through
 *  `routeBriefRefusal`. Any other error clears the refusal and toasts through this module's own
 *  `sentenceForReplay`: a `switch (classify(error))` with no `default:`. The prose is local
 *  because its tail names this act (`problems.ts:3-8`). */
export function replayStart(pending: PendingStart, withoutBrief: boolean): Promise<void>;

/** The anchor key of the attempt's home, or `null` when the home is `TopBar` itself (re-review
 *  b07-1 R-I1). A `switch (attempt.kind)` with no `default:` and a `string | null` return, so
 *  TS2366 applies:
 *    create  → origin === "new-session" ? null : `empty:${worktreeId}`
 *    restart → `pane:${sessionId}`
 *    resume, fork → `pane:${sourceId}` */
export function anchorKey(attempt: StartAttempt): string | null;

/** Called by each home while it is mounted: `hostRefusal(key)` in a `useEffect`,
 *  `releaseRefusal(key)` in its cleanup. `SessionPane` passes `pane:<sessionId>`;
 *  `EmptyDevelop` passes `empty:<target.id>`. */
export function useRefusalHost(key: string): void;
```

**Where the refusal renders.** At every moment each refusal renders in exactly one place: its
**home** when the home is mounted, otherwise the **top-bar fallback** (re-review b07-1 R-I1). The
homes, anchored to the control that started the session:
- `create`, `origin: "new-session"`: in `TopBar`, directly below `NewSessionButton`
  (`topbar/TopBar.tsx:41`). All the hook's callers share it, including the rail's `WorktreeRow`
  and the `ContextMenu`, because none of them has a surface of its own that outlives the click.
  `TopBar` is mounted in every lens and on every branch (`AppShell.tsx:92`), so this home is
  always mounted and never falls back.
- `create`, `origin: "empty-develop"`: in `EmptyDevelop`, beside its two buttons.
- `restart`: in the `SessionPane` of `sessionId`. `resume` and `fork`: in the `SessionPane` of
  `sourceId`. Above its exit bar or header. The session menu closes before its verb runs (`act`,
  `sessionVerbs.ts:303-306`), so the menu itself has nothing left to anchor to.

**Why a fallback: a home is often not mounted.** The session verbs are reachable where no pane
exists. `Rail` and `ContextMenu` are mounted in every lens (`AppShell.tsx:89`, `:97`).
`SessionRow` opens a `{kind: "session"}` menu (`rail/SessionRow.tsx:49`) for any expanded worktree
on any branch (`rail/WorktreeRow.tsx:86-89`; expansion is independent of selection), and
`ContextMenu` hands it `sessionVerbs` (`menu/ContextMenu.tsx:157`). But a pane is mounted only
in the Develop lens (`AppShell.tsx:17-27`), only for the selected branch (`selectPaneSessionIds`,
`store/selectors.ts:72-83`), and in Canvas only for pinned tiles (`CanvasLayout.tsx:394`). So a
Restart, Resume or Fork from the Discover lens, from another branch, or on an unpinned Canvas
session would otherwise be recorded and rendered nowhere: the silent path `4-D1` exists to
prevent. `EmptyDevelop` has the same gap when the person changes lens or branch before the 409
lands.

**The fallback.** `TopBar` renders the refusal directly below `NewSessionButton`, with one line
above the reason that names what was refused: *Restart of `<session name>`*, *Resume of `<session
name>`*, *Fork of `<session name>` into `<worktree name>`*, or *New session in `<worktree name>`*.
The names are read from the store at render time. Retry and the override behave exactly as at the
home.

Option (b) of R-I1 was considered and rejected: moving the lens, branch and focus to bring the
pane on screen navigates the person away from where they acted. The AGENTIC-UX principle held
above ("each decision is answerable in place, without navigating") rules that out.

**How "mounted" is known: a host registry, not a copy of the layout rules.** Re-deriving
mountedness from the lens, `selectPaneSessionIds`, the pin list and `DevelopLens`'s empty-state
and Canvas-seeding rules (`DevelopLens.tsx:24-44`) would be a second copy of those rules, and a
copy drifts. So each home says it is mounted:
- `anchorKey(attempt)` gives the home's key:
  - `create`, `origin: "new-session"` → `null` (always mounted);
  - `create`, `origin: "empty-develop"` → `empty:<worktreeId>`;
  - `restart` → `pane:<sessionId>`;
  - `resume` and `fork` → `pane:<sourceId>`.
- `useRefusalHost(key)` is called by `SessionPane` with `pane:<sessionId>`, and by `EmptyDevelop`
  with `empty:<target.id>` (the worktree its two buttons pass to `startSessionOnWorktree`). In a
  `useEffect`, it increments `refusalHosts[key]` and decrements it on cleanup. It is a count, not a
  flag, so StrictMode's mount, cleanup and re-mount cannot leave a stale zero or a stale one. At
  most one pane per session is mounted: Columns and Canvas are exclusive (`DevelopLens.tsx:40-44`),
  and each lists a session once.
- A home renders `pendingStart` when `anchorKey(pendingStart)` is its own key. `TopBar` renders it
  when the key is `null` or `(refusalHosts[key] ?? 0) === 0`. The two conditions are disjoint, so
  the refusal is never shown twice and never shown nowhere.

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
| `retry re-sends the identical request` | the second `createSession` body equals the first, and neither has a `withoutBrief` key (re-review b07-1 R-M2); a retried `restartSession` gets `undefined` options | a retry that drops the person's harness or prompt; a retry that adds `withoutBrief: false` |
| `start without the brief sends withoutBrief` | the body adds `withoutBrief: true` and nothing else changes | an override that also drops options |
| `the override is not the default focus` | after render, `document.activeElement` is Retry | a going-on-without made the easy path |
| `restart routes the same way` | a 409 from `restartSession` sets `kind: "restart"`; the override calls `restartSession(id, {withoutBrief: true})` | restart silently starting without the brief |
| **Call site 1** (`useCreateSession.test.ts`): `the new-session hook routes the refusal` | a 409 `brief_not_written` → `pendingStart` is `{kind: "create", origin: "new-session", request}` with `request` equal to the body sent; `pushToast` and `setError` are not called | call site 1 still toasting (review I9) |
| **Call site 2** (`EmptyDevelop.test.tsx`): `the empty state's start routes the refusal` | `startSessionOnWorktree` answered by the 409 → `pendingStart` is `{kind: "create", origin: "empty-develop", request}` with the body sent; no toast; the refusal renders in `EmptyDevelop` and not at the new-session control | the second `api.createSession` call site left on `sentenceForCreate` (review I9) |
| **Call site 3** (`sessionVerbs.test.ts`): `the menu's restart rolls back and routes the refusal` | the Restart verb answered by the 409 → the optimistic `starting` is rolled back to the recorded session; `pendingStart` is `{kind: "restart", sessionId}`; `pushToast` is not called | `runImmediate`'s catch reporting before the refusal is seen |
| **Call site 4** (`SessionPane.test.tsx`): `the exit bar's restart routes the refusal` | the exit bar's Restart answered by the 409 → `pendingStart` is `{kind: "restart", sessionId}`; `sentenceForRestart` does not toast; the refusal renders in that pane | the exit bar's own `restartSession` still toasting (review I9) |
| **Call site 5** (`sessionVerbs.test.ts`): `resume routes the refusal and replays the verb` | Resume answered by the 409 → `pendingStart` is `{kind: "resume", sourceId, harness}`; Retry calls `resumeSession(source, harness, undefined)` and the override calls it with `{withoutBrief: true}`, `source` read from the store | resume refused with no way to re-send, because the desktop never held its request (review I9) |
| **Call site 6** (`sessionVerbs.test.ts`): `fork routes the refusal and replays the verb` | Fork answered by the 409 → `pendingStart` is `{kind: "fork", sourceId, targetWorktreeId, harness}` with `worktreeId` equal to the target; Retry calls `forkSession(source, targetWorktreeId, harness, undefined)` and the override calls it with `{withoutBrief: true}` | fork refused into a toast, or replayed into the source's worktree |
| `a replay whose source is gone sends nothing` (`startAttempt.test.ts`) | a `resume` refusal whose `sourceId` was purged → Retry sends no request, clears `pendingStart` and toasts the one sentence | a replay that throws on `undefined`, or sends a body built from nothing |
| `each reason shows its extra` — table | the six rows above, `block_broken` in both variants | one generic sentence for six different fixes |
| `an unknown project offers the override` | a `project_unknown` 409 on a create and on a restart → the `project_unknown` sentence, **Retry** focused, **Start without the brief** present; the override re-sends with `withoutBrief: true` (review I2) | the new reason falling through to a toast or a generic sentence; the one recovery that works when the projects cannot be read left off |
| `a refused override offers no second override` | a `block_broken` 409 answering a request that carried `withoutBrief: true` (create and restart) → `pendingStart.withoutBrief` is true; **Start without the brief** is absent; the sentence is the `withoutBrief: true` variant and does not contain "start without the brief"; Retry re-sends `withoutBrief: true` | a recovery offered that re-sends the request just refused (re-review I2) |
| `other errors keep phase 2's behaviour` | a `backend_unavailable` still toasts through `sentenceFor` | a phase 2 regression |
| `a rail restart refused while no pane is mounted still renders the refusal` (`AppShell.test.tsx`) — three rows: the Discover lens; the Develop lens with another branch selected; Canvas with the session unpinned | the full `AppShell` renders. The rail's `SessionRow` → right-click → Restart, answered by the 409. Then: exactly one refusal is in the document, inside `TopBar`, and its first line names the session (*Restart of `<name>`*); no `SessionPane` is mounted for it; `pushToast` is not called. Then the person selects that session's branch in Develop (Columns): the refusal is inside that session's pane, and `TopBar` holds none (re-review b07-1 R-I1) | a session-bound refusal recorded and rendered nowhere; a fallback that also renders while the home is mounted, so the refusal shows twice |
| `the host count survives a remount` (`startAttempt.test.ts`) | `useRefusalHost("pane:s1")` mounted, unmounted and re-mounted under `StrictMode` → `refusalHosts["pane:s1"]` is 1; after the final unmount the key is absent | a flag that a StrictMode cleanup clears after the re-mount, sending the refusal to `TopBar` while its pane is on screen |

**Steps:**

- [ ] **Step 1:** Re-locate the six call sites of the table above by symbol, then run
  `grep -rn "api\.\(createSession\|restartSession\|resumeSession\|forkSession\)" desktop/src --include=*.ts --include=*.tsx`
  and confirm that every **code** hit outside `api/` and outside tests is one of the six. Comment
  lines also match (at `974ed4d`: `EmptyDevelop.tsx:13`, `:24`, `sessionVerbs.ts:428`); they are
  not call sites and do not count. A seventh code hit is a finding: stop and record it in the task
  report before going on.
- [ ] **Step 2:** Write the tests; confirm they fail.
- [ ] **Step 3:** The slice additions, `startAttempt.ts`, the component. Then the six routing
  edits: each `catch` calls `routeBriefRefusal(error, attempt, false)` first and returns when it
  answers true. In `runImmediate`, the optional `refusal?: StartAttempt` is checked after
  `rollback()` and before `report`. Then the three components "Where the refusal renders" names:
  `SessionPane` and `EmptyDevelop` call `useRefusalHost` and render `pendingStart` only when
  `anchorKey(pendingStart)` is their own key; `TopBar` renders it when the key is `null` or no
  home holds it.
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
