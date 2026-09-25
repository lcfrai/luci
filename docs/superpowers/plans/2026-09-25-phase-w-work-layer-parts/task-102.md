### Task 102: `Shift+O` delegate and `Shift+A` ask; the team strip's line is the issue title

**Slice:** W3. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WD2.
**dependsOn:** 101; TM-53, TM-55. **Seam:** consumes **SW.10** (task 24's Start result and its
*Undo* rule) and SW.19 (task 100's delegate and withdraw routes). Phase T paths are **UNVERIFIED
until merge** (rule H2).

Task 53 reserved two chords as inert *"from W3"* (05 §4.3: *"`Shift+O` and `Shift+A` do nothing
until W3"*). This task makes them live. `Shift+O` gives **my** issue to **my** Orchestrator;
`Shift+A` asks my Orchestrator about the focused row. And the team strip, which names what each
person is doing, names the issue a person wrote instead of the task title an agent wrote.

**Anchors, re-read on 2026-09-25.** 05-design (`linear-parity/05-design.md`): §2.1 *Team:
Delegate* (`:521-523`: *"`Shift+O` sends EP-142 to **your own** Orchestrator … Nobody can delegate
to someone else's agents (OD-16)"*), §3.4's strip row (`:773`: *"the line is the **issue title**
(`text·H`) where one exists, instead of the task title (`text·A`)"*), the key table
(`:1010-1011`), §4.6's reversibility row (`:1105`: *"reversible until the session's first
**file** write … an **Undo chip**"*). Phase W: task 53's `KEYMAP`, `KeyBinding`,
`isListKeyTarget`, `matchListKey`, `registerVerb` and task 59's `buildCommands`, `openPalette`
(scopes only, `tasks-41-60.md`; **no part written yet, UNVERIFIED**); task 61's `StartChip` and
`useStart` (scope only, `tasks-61-80.md`); task 24's `ErrFirstWrite` → 409 `first_write` and
`Written` (`task-24.md:31`, `:197-210`); task 37's `WORK_RELAY`, `WORK_LOCAL`, `relayPath`,
`makeWorkClient` (`task-37.md:264-310`, `:370-410`); task 100's `DelegateIssue`,
`WithdrawDelegation` and decision (f). Phase T at `64ab2c4`: TM-52's `PersonRow.line
{briefSection, taskTitle, agents}` (`task-52.md:852-859`), Go `Line` (`:326-331`), `AskSubject
{kind: "task" | "path" | "agent" | "conflict"; ref}` (`:950`), the slice's `openOrchestrator(
projectId, subject?)` (`:1028`), ST.8 assembled in `control-plane/internal/api/teamassemble.go`
(`:90`); TM-53's rule that the strip *"computes no status, no label and no line of its own"*
(`task-53.md:10-11`) and its row table (`:136-140`); TM-55's palette command
`verb:ask-orchestrator` (`task-55.md:62-64`).

Six decisions the scope left open, made here.

**(a) The keys are task 53's rows, switched on, not new bindings.** Task 53 holds both rows with
`slice: "W3"`, inert. This task adds `"W3"` to the active slices and registers the two verbs with
`registerVerb`. Both test `event.code` (`KeyO`, `KeyA`) with `shiftKey` and no Ctrl, Meta or Alt,
and fire only when `isListKeyTarget` is true, so never in a terminal (OD-9) or a text field.

**(b) What `Shift+O` does, by row.** On **my** issue in a **team** project: delegate. On an
**unassigned** issue: assign me first (task 61's rule for `Shift+R`, one `mutate` write), then
delegate. On **someone else's** issue: nothing is sent, and the row says in place *"Only Alice can
delegate EP-142."* (OD-16). On a **solo** project (the strip's `team: false`): nothing is sent, and
the row says *"Delegate needs team mode. Start (Shift+R) runs it here."* Every refusal is words in
the row, never a toast, and the palette's *Delegate to my Orchestrator* is disabled with the same
words as its hint.

**(c) Undo is held twice.** The chip offers *Undo* while the control plane reports `undoable`; the
control plane's undo route refuses with 409 `first_write` once any of my agents on a task bound to
this issue has an observed file (TM-22's `Files`, through TM-52's reader). The Hub cannot see a
file write (task 100 (f)), so the refusal lives in the control plane, not the Hub. There is **no
timer**, as task 24's *Undo Start* has none.

**(d) `Shift+A` asks about the row, and says which row.** It calls TM-52's
`openOrchestrator(projectId, subject)` with `subject {kind: "issue", ref: "issue:is-…@r"}` for an
issue row, and the row's own kind (task, path, agent, conflict) for Phase T's rows. `"issue"` is an
added member of `AskSubject.kind` (a Phase T delta, gap 1). On a solo project it does nothing and
says *"Ask needs team mode."* It sends nothing by itself: the panel opens with the subject chip and
an empty composer; the person writes the question.

**(e) The strip line is assembled by the control plane, not the row.** TM-53 draws what ST.8 says.
So `Line` gains `issue {key, alias, title}` in TM-52's `teamview.go`, filled in `teamassemble.go`
from the person's newest active task's `issueId` (task 98), read with the viewer's own Hub session.
When the issue title is `text·H`, `taskTitle` is sent as `null` and the row shows the issue title;
when it is `text·A`, both are sent and the row keeps the task title, so an agent's words never
replace a person's and a person's always replace an agent's. The rule is task 101's decision (g),
the same on both surfaces.

**(f) The chip is `StartChip`'s sibling, and says less.** *"Delegated to your Orchestrator ·
Undo"* while undoable; *"Your Orchestrator has it"* after the first file write, with no action;
*"Delegation withdrawn"* for 4 s after an undo, then the row returns to its state. No countdown,
no spinner (one heartbeat, DESIGN-SYSTEM); the chip's words are facts from the control plane, never
a prediction (`expectNoPrediction`, OD-27).

**Files:**

| Path | Change |
|---|---|
| `desktop/src/work/keys/keymap.ts` ⚑ (task 53) | `"W3"` joins the active slices; the two rows' `verb`s registered |
| `desktop/src/components/palette/commands.ts` ⚑ (task 59) | *Delegate to my Orchestrator* and *Ask my Orchestrator about this*, hints from `chordFor` |
| `desktop/src/work/delegate/useDelegate.ts`, `useDelegate.test.ts` | new: the row rule of decision (b), the calls, the chip state |
| `desktop/src/components/work/DelegateChip.tsx`, `DelegateChip.test.tsx` | new: decision (f) |
| `desktop/src/api/work.ts` ⚑ (task 37) | two `WORK_RELAY` rows, two `WORK_LOCAL` rows, four client methods |
| `control-plane/internal/api/workdelegate.go`, `workdelegate_test.go` | new: the delegation state and the guarded undo (decision (c)) |
| `control-plane/internal/api/api.go` ⚑ | one line, `a.registerWorkDelegate(guarded)`, beside `registerWork` |
| `control-plane/internal/api/teamview.go` ⚑, `teamassemble.go` ⚑ (TM-52) | `Line.Issue`; the title rule (decision (e)) |
| `desktop/src/store/types.ts` ⚑ (TM-52) | `line.issue`; `AskSubject.kind` gains `"issue"` |
| `desktop/src/components/team/TeamRow.tsx` ⚑ (TM-53) | the line reads `line.issue.title` when present |

The scope named TM-53's strip for the title change; TM-53 assembles no line of its own
(`task-53.md:10-11`), so the rule goes to TM-52's control-plane assembly and TM-53's row only
renders the added field. `workdelegate.go`, `api.go`, `work.ts` and TM-52's files are added to the
scope's list for decisions (c) and (e).

#### The wire, complete

```ts
// desktop/src/api/work.ts ⚑ — appended to WORK_RELAY (task 100's routes, relayed by Relay())
  ["DelegateIssue", "POST", "/v1/issues/{issueId}/delegate"],
  ["WithdrawDelegation", "DELETE", "/v1/issues/{issueId}/delegate"],
// appended to WORK_LOCAL (this task's control-plane routes; never relayed)
  ["delegationState", "GET", "/v1/work/issues/{issueId}/delegation"],
  ["undoDelegate", "POST", "/v1/work/issues/{issueId}/delegation/undo"],

/** Task 100's IssueDelegation. withdrawnAt is null while live. */
export interface IssueDelegation {
  issueId: string; ownerId: string; delegatedAt: string; delegationId: string;
  withdrawnAt: string | null;
}
/** This task's control-plane answer: a fact, never an estimate. */
export interface DelegationState {
  issueId: string;
  state: "none" | "undoable" | "working" | "withdrawn";
  since: string | null;         // when that state began, as the control plane observed it
  firstWriteAgent: string | null; // ag-… whose observed file closed the window, else null
}
// WorkClient gains:
  delegateIssue(issueId: string): Promise<IssueDelegation>;
  delegationState(issueId: string): Promise<DelegationState>;
  undoDelegate(issueId: string): Promise<IssueDelegation>;   // 409 first_write after a write
```

`WithdrawDelegation` is relayed but the desktop never calls it directly: `undoDelegate` is the one
way, so the first-write check cannot be skipped from the desktop. It stays relayed because task
100's relay mounts every W3 row but `AddAgentComment`; `TestTheDesktopUndoesOnlyThroughTheGuard`
pins that no desktop module calls `WithdrawDelegation`.

```jsonc
// GET /v1/work/issues/{issueId}/delegation → 200 DelegationState
{ "issueId": "is-…", "state": "undoable", "since": "2026-09-25T09:12:04Z", "firstWriteAgent": null }
// POST /v1/work/issues/{issueId}/delegation/undo → 200 IssueDelegation (withdrawnAt set)
// 409 first_write {agentId} · 404 delegation_not_found · 503 team_view_unavailable {since}
// (TM-52's reader down: the control plane cannot prove no write happened, so it refuses)
```

```go
// control-plane/internal/api/teamview.go ⚑ (TM-52) — Line gains one field
type Line struct {
	BriefSection *BriefSection `json:"briefSection"`
	TaskTitle    *TeamText     `json:"taskTitle"` // null when Fleet withholds it, or when Issue's title is text·H
	Agents       int           `json:"agents"`
	Issue        *LineIssue    `json:"issue"`     // Phase W: the newest active task's issue; null when none
}

type LineIssue struct {
	Key   string   `json:"key"`   // issue:is-…@r
	Alias string   `json:"alias"` // EP-142, display only
	Title TeamText `json:"title"` // Author "human" | "agent", from the revision's AuthoredText
}
```

```ts
// desktop/src/store/types.ts ⚑ (TM-52)
  line: { briefSection: BriefSection | null; taskTitle: TeamText | null; agents: number;
          issue: { key: string; alias: string; title: TeamText } | null };
export interface AskSubject { kind: "task" | "path" | "agent" | "conflict" | "issue"; ref: string; }
```

#### `control-plane/internal/api/workdelegate.go`, complete in shape

```go
package api

// DelegationFacts is what the undo guard reads: TM-52's reader, as the viewer. FirstWrite
// answers the first of the caller's own agents, on a task bound to issueID, with an observed
// file (TM-22's WorkerAgent.Files non-empty), and when it was first seen.
type DelegationFacts interface {
	FirstWrite(ctx context.Context, projectID, issueID string) (agentID string, at time.Time, ok bool, err error)
}

// registerWorkDelegate mounts the two local routes of decision (c).
func (a *API) registerWorkDelegate(mux *routes) {
	mux.Handle("GET /v1/work/issues/{issueId}/delegation", a.guard(a.delegationState))
	mux.Handle("POST /v1/work/issues/{issueId}/delegation/undo", a.guard(a.undoDelegate))
}

// delegationState: Hub Issue (the project and assignee), then the viewer's delegation row
// through the relay's WorkAgents read, then FirstWrite. state is "none" with no row,
// "withdrawn" with withdrawnAt set, "working" once FirstWrite is ok, else "undoable".
func (a *API) delegationState(w http.ResponseWriter, r *http.Request)

// undoDelegate re-reads the state in the same request (never trusting the desktop's copy):
// "working" → 409 first_write {agentId}; FirstWrite's error → 503 team_view_unavailable, since
// without the reader nobody can show no write happened; "undoable" → Work.WithdrawDelegation.
func (a *API) undoDelegate(w http.ResponseWriter, r *http.Request)
```

Task 100 has no read route for a delegation, so live-or-withdrawn comes from the change feed:
the newest `issue.delegated` or `issue.undelegated` for this issue with `ownerId` = the viewer,
read through the control plane's feed reader (SW.5, task 13; that the control plane, not only the
desktop replica, holds the feed is **UNVERIFIED**, gap 2). With no such event the answer is `none`
and the chip is absent, never a guess.

#### `desktop/src/work/delegate/useDelegate.ts`, interfaces complete

```ts
import type { DelegationState, IssueDelegation } from "../../api/work";

/** Why Shift+O did nothing, in words the row shows in place (decision (b)). */
export type DelegateRefusal =
  | { kind: "not_yours"; assigneeName: string; alias: string } // "Only Alice can delegate EP-142."
  | { kind: "solo" }                                            // "Delegate needs team mode. …"
  | { kind: "erased" }
  | { kind: "no_orchestrator" }; // the Hub's 422 orchestrator_unavailable, in words

export interface RowTarget {
  issueId: string; alias: string; projectId: string;
  assignee: { id: string; name: string } | null;
}

export interface UseDelegate {
  /** Shift+O and the palette call this; it never throws. */
  delegate(row: RowTarget): Promise<{ ok: true; d: IssueDelegation } | { ok: false; why: DelegateRefusal }>;
  undo(issueId: string): Promise<{ ok: true } | { ok: false; code: "first_write" | "team_view_unavailable" }>;
  /** The chip's state, refreshed on the one heartbeat (no poll of its own), keyed by issue. */
  state(issueId: string): DelegationState | undefined;
}

export function useDelegate(): UseDelegate;

/** The pure rule of decision (b); useDelegate calls it before any request. */
export function delegateRule(row: RowTarget, me: string, team: boolean):
  { action: "delegate" | "assign_then_delegate" } | { action: "refuse"; why: DelegateRefusal };
```

#### `desktop/src/components/work/DelegateChip.tsx`

```tsx
export function DelegateChip(props: { issueId: string; alias: string }): JSX.Element | null;
// undoable → "Delegated to your Orchestrator" · button "Undo" (confirm not needed: reversible)
// working  → "Your Orchestrator has it" · no button
// withdrawn → "Delegation withdrawn", 4 s, then null
// none | undefined → null
// Sans at --t-body for the words; the alias in Commit Mono at --t-fact; elevation is the
// row's own; no spinner, no countdown, no percentage (OD-27).
```

A 409 `first_write` from `undo` turns the chip to *working* at once and says *"Too late to undo:
your Orchestrator's agent changed a file."* in the chip's place for 4 s; nothing is retried.
