### Task 20: Desktop — `4-D1` on session start and restart

When `POST /v1/sessions` or `POST …/restart` answers `409 brief_not_written`, the session did not
start (task 10 guarantees nothing was recorded). The person sees **why**, a **Retry**, and one
deliberate **Start without the brief** (prototype state 10). Today that 409 would become a toast
with the server's sentence (`useCreateSession.ts:80-83` at `3303fd6`), which is the silent path
`4-D1` exists to prevent.

**Read `docs/DESIGN-SYSTEM.md` first.** Held here: the refusal is **answerable in place**
(AGENTIC-UX principle, "each decision is answerable in place, without navigating") — an inline
surface anchored to the control that started it, not a modal and not a toast; **Start without the
brief** is a secondary control, never the default focus, because going on without the brief must
be deliberate; `--need` with its **triangle** shape marks the refusal (fact sheet §10); elevation,
radius and motion as task 17. The owner's eyes are the visual acceptance (`OD-12`).

**Files:**
- `desktop/src/discover/StartWithoutBrief.tsx` + `StartWithoutBrief.test.tsx` (new)
- `desktop/src/hooks/useCreateSession.ts` — `brief_not_written` routed to the refusal (modify)
- `desktop/src/components/develop/sessionVerbs.ts` — restart routes the same 409 (modify)
- `desktop/src/store/slices/discoverSlice.ts` — the pending refusal (modify, task 16's)

**Interfaces — complete source:**

```ts
// desktop/src/store/slices/discoverSlice.ts — additions
export type BriefReason =
  | "hub_unavailable" | "signed_out" | "write_failed" | "block_broken" | "tracked_elsewhere";

export interface PendingStart {
  /** What to re-send. For a create, the request the person made; for a restart, the session. */
  kind: "create" | "restart";
  worktreeId: WorktreeId;
  request?: CreateSessionRequest;
  sessionId?: SessionId;
  reason: BriefReason;
  message: string; // the server's sentence, verbatim
  /** True when the refused request already carried withoutBrief: true. The override is then
   *  not offered again: it would re-send the request that was just refused (re-review I2). */
  withoutBrief: boolean;
}

// in DiscoverSlice:
  pendingStart: PendingStart | null;
  setPendingStart(p: PendingStart | null): void;
```

```ts
// desktop/src/discover/StartWithoutBrief.tsx
/** Rendered by the surface that started the session, anchored to it. Retry re-sends the
 *  identical request; "Start without the brief" re-sends it with withoutBrief: true, and the
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
| `each reason shows its extra` — table | the five rows above, `block_broken` in both variants | one generic sentence for five different fixes |
| `a refused override offers no second override` | a `block_broken` 409 answering a request that carried `withoutBrief: true` (create and restart) → `pendingStart.withoutBrief` is true; **Start without the brief** is absent; the sentence is the `withoutBrief: true` variant and does not contain "start without the brief"; Retry re-sends `withoutBrief: true` | a recovery offered that re-sends the request just refused (re-review I2) |
| `other errors keep phase 2's behaviour` | a `backend_unavailable` still toasts through `sentenceFor` | a phase 2 regression |

**Steps:**

- [ ] **Step 1:** Read `useCreateSession.ts` and `sessionVerbs.ts`; find every call site that
  starts or restarts a session (`grep -rn "createSession\|restartSession" desktop/src --include=*.ts*`)
  and confirm each goes through one of these two.
- [ ] **Step 2:** Write the tests; confirm they fail.
- [ ] **Step 3:** The slice additions, the component, the two routing edits; mount
  `StartWithoutBrief` where the new-session control and the session menu render.
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
