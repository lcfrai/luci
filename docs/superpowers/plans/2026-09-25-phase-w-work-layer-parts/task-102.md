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
| `desktop/src/work/delegate/register.ts` | new: registers the two verbs with task 53's `registerVerb` |
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

#### The deltas on task 53, task 59, TM-52 and TM-53

```ts
// desktop/src/work/keys/keymap.ts ⚑ (task 53). The two rows exist, inert, as task 53 wrote
// them: { verb: "delegate", keys: ["Shift+O"], code: "KeyO", shift: true, scope: "issueRow",
// slice: "W3", … } and { verb: "ask", keys: ["Shift+A"], code: "KeyA", shift: true,
// scope: "everyRow", slice: "W3", … }. This task changes one constant:
export const ACTIVE_SLICES: ReadonlySet<KeyBinding["slice"]> = new Set(["W1", "W2", "W3"]);
// (task 53's name for the set is UNVERIFIED; Step 1 uses its real one)

// desktop/src/work/delegate/register.ts — new, called once from the Work view's mount
export function registerDelegateVerbs(d: UseDelegate, openOrchestrator: OpenOrchestrator): () => void {
  const offO = registerVerb("delegate", (row) => void d.delegate(row));
  const offA = registerVerb("ask", (row) => openOrchestrator(row.projectId, askSubjectOf(row)));
  return () => { offO(); offA(); };
}

/** An issue row asks about the issue; Phase T's rows keep their own kinds. */
export function askSubjectOf(row: RowContext): AskSubject | undefined;
```

`RowContext` is task 59's type;
`OpenOrchestrator` is TM-52's slice action (`task-52.md:1028`).

**Palette** (`commands.ts` ⚑): two commands, generated from the two `KEYMAP` rows like every
other work verb (task 59), with `hint` from `chordFor`. *Delegate to my Orchestrator* is present on
issue rows of team projects only; on a solo project it is present and disabled, its hint replaced
by *"needs team mode"*. *Ask my Orchestrator about this* is TM-55's `verb:ask-orchestrator` with
the row's subject; this task does not add a second ask command, it passes the subject to TM-55's.

**Assembly** (`teamassemble.go` ⚑, TM-52): for each person row, the newest active task's
`issueId` (task 98); one `Work.Issue` per distinct issue per assembly, as the viewer; `Issue` is
filled when the read succeeds, and a failed read leaves `issue: null` and the line as TM-52 built
it. When `Issue.Title.Author == "human"`, `TaskTitle` is set to `nil`.

**Row** (`TeamRow.tsx` ⚑, TM-53): the line is `briefSection.title`, then ` · `, then **`line.issue`
as `EP-142` in Commit Mono and its title as `AuthoredText compact`** when present, else TM-53's
`taskTitle`, then ` · N agents`. No other change: the dot, the files and the waiting phrase are
TM-53's.

#### Tests to write first, each with the mutant it kills

Desktop in Vitest with Testing Library; the control plane in Go. Every desktop surface test ends
with `expectNoPrediction(container)` (task 20).

| Test | Asserts | Mutant it kills |
|---|---|---|
| `useDelegate › solo does nothing and says why` | `team: false` → no request; `why.kind === "solo"`; the palette command is disabled with *"needs team mode"* | delegate without an Orchestrator |
| `useDelegate › a teammate's issue is refused` | assignee Alice, me Dylan → no request; the row reads *"Only Alice can delegate EP-142."* | delegate to another person's agent (OD-16) |
| `useDelegate › unassigned assigns me first` | one `mutate` assign write, then one `delegateIssue`, in that order | delegating an issue nobody owns |
| `keys › Shift+O matches the code, not the key` | `{code: "KeyO", key: "O", shiftKey: true}` in a list → delegate; `{key: "O", code: "KeyP"}` (another layout) → nothing; `{code: "KeyO", shiftKey: false}` → nothing | `event.key` matched |
| `keys › never in a terminal or a field` | the same event inside the xterm host, an `input` and a `contenteditable` → nothing | a chord that steals a terminal key |
| `DelegateChip › Undo before the first write` | state `undoable` → *Undo*; click → `undoDelegate` once; then *Delegation withdrawn* | undo window ignored |
| `DelegateChip › gone after the first write` | state `working` → no button; a 409 `first_write` from a stale click → *"Too late to undo …"* | an undo offered after work began |
| `TestUndoAfterAFileWriteIsRefused` (Go) | `FirstWrite` ok → 409 `first_write` naming the agent; no `WithdrawDelegation` call | a guard only the UI holds |
| `TestUndoWithoutTheReaderIsRefused` (Go) | `FirstWrite` errors → 503 `team_view_unavailable`; no Hub call | an undo that assumes no write |
| `TestTheDesktopUndoesOnlyThroughTheGuard` | a source scan of `desktop/src` finds `WithdrawDelegation` only in `api/work.ts`'s table | a surface that skips the guard |
| `Shift+A › asks about the issue` | on an issue row → `openOrchestrator(pid, {kind: "issue", ref: "issue:is-…@3"})`; the composer is empty; nothing is sent | an ask sent without words |
| `TestTheLineIsTheIssueTitle` (Go, `teamassemble`) | a task with `issueId` and an `H` title → `issue.title` set, `taskTitle: null`; an `A` title → both set | task title kept |
| `TeamRow › the line reads the issue` | the row's text is `Sessions survive restart · EP-142 Resume a session after a restart · 2 agents` | a row that computes its own line |

#### Steps

- [ ] **Step 1: Check the sync base.** Tasks 53, 59 and 61 are on it: re-read `KEYMAP`'s two W3
  rows, the active-slice set's name, `registerVerb`, `RowContext` and `chordFor`, and rename here.
  TM-52, TM-53 and TM-55 are merged (`grep -n "openOrchestrator" desktop/src/store/slices/*.ts`
  prints the action). Task 100's two routes are relayed (`grep -n WithdrawDelegation
  control-plane/internal/hub/hubclient/methods_work_agents.go`). Find the control plane's feed
  reader for decision (c)'s state; if there is none, stop and raise gap 2 before writing code.
- [ ] **Step 2: Write the tests first**; run them and see each fail for its reason (no verb
  registered, no route, no field).
- [ ] **Step 3: Write the control-plane half** (`workdelegate.go`, the `api.go` line, TM-52's
  `Line.Issue` and assembly) and run the Go tests to green.
- [ ] **Step 4: Write the desktop half** (`work.ts` rows, `useDelegate`, `register.ts`,
  `DelegateChip`, the keymap constant, the palette commands, TM-52's types, TM-53's row) and run
  the Vitest suites to green.
- [ ] **Step 5: Prove the tests bite.** Apply each mutant by hand, see its test fail, revert.
- [ ] **Step 6: Commit** with `git add` for the new files, then `git commit -F <msgfile> --` the
  paths of *Files*. Message file under `.superpowers/sdd/work/msg/`; last line
  `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.
- [ ] **Step 7: Owner's eyes.** The chip's three states and the strip line, in the running app on
  a team project, reviewed by the owner (CLAUDE.md: visual acceptance is the owner's eyes, not a
  self-report). Record the words the owner used in `docs/verification/2026-09-25-phase-w-results.md`
  under W3 (task 113).

#### Acceptance

```bash
cd desktop && npx vitest run src/work/delegate src/components/work/DelegateChip.test.tsx src/work/keys src/components/palette src/components/team/TeamRow.test.tsx 2>&1 | tail -3
# Test Files  N passed (N)   — every file named above; no skipped test
cd .. && go test ./control-plane/internal/api/ -race -count=1 -v -run 'TestUndoAfterAFileWriteIsRefused|TestUndoWithoutTheReaderIsRefused|TestTheLineIsTheIssueTitle' 2>&1 | grep -c '^--- PASS'
# 3
go test ./control-plane/internal/api/ -race -count=1
# ok   (TM-52's own team tests and fixtures unchanged but for the added null "issue" key)
cd desktop && npx tsc --noEmit && npx eslint src/work/delegate src/components/work
# (no output)
```

`N` is the count Vitest prints for the named files; it is not predicted here. The TM-52 fixtures
under `control-plane/internal/api/testdata/team-view/v1/` gain `"issue": null` on every line; the
diff is that key and nothing else (`git diff START -- control-plane/internal/api/testdata/team-view`
shows only added `"issue": null` lines).

#### What this task does not claim

- That a delegated issue reaches a running Orchestrator in the cloud (phase 6 and 8, TM1), or
  that Fleet stops work on `issue.undelegated` (task 98).
- That the chip looks right. Step 7 is the owner's review; no screenshot is evidence.
- Bulk delegate (*"Delegate 4 to my Orchestrator"*, L-225, `05-design.md:402`): task 44's
  `BulkBar` would take the verb; no task in the skeleton adds it, so it is unowned (gap 5). This
  task registers the single-row verb only.
- That TM-55's panel answers an `issue` subject well. It opens with the chip; what the
  Orchestrator does with it is TM-41's and task 110's.

**Parity rows:** closes the W3 half of **L-39** (with task 61: Start in solo mode); makes live the
`Shift+O` chord that **L-223** names, a W1 row task 53 closes with the chord reserved. Rows
checked against `05-design.md` on 2026-09-25 (L-39 at `:148`, L-223 at `:400`).

**Corrections to the scope paragraph:** the strip's title rule is assembled in TM-52's control
plane, not in TM-53's row (decision (e)); Undo is guarded in a new control-plane route because
the Hub cannot see a file write (decision (c)); the withdraw route is task 100's addition, which
the scope's *"the delegate route task 100 adds"* did not include.

**Gaps found while writing this part** (for the skeleton's owner and the named tasks):

1. **`AskSubject.kind` has no `"issue"`** (`task-52.md:950`). This part adds it; Phase T's
   TM-39 attach handler and TM-41's instructions must accept and read it, or `Shift+A` on an
   issue sends a subject Fleet refuses (UNVERIFIED which task in Phase T owns the decode).
2. **No delegation read route and no confirmed control-plane feed reader.** Decision (c) reads
   the delegation's state from the change feed; if the control plane holds no feed, task 100 needs
   `GET /v1/issues/{issueId}/delegate` (an added route) and this part's Step 1 stops for it.
3. **Tasks 53, 59 and 61 have no parts yet**; every name taken from their scopes is re-read at
   Step 1.
4. **The file map** has no rows for `work/delegate/`, `DelegateChip.tsx` or `api/workdelegate.go`.
5. **Bulk delegate is unowned.** L-225's row promises *Delegate 4 to my Orchestrator* in the bulk
   bar; task 44 builds the bar in W1 before any Orchestrator exists, and no W3 task adds the verb.
