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
to someone else's agents (OD-16)"*), §3.4's strip row (`:771`: *"the line is the **issue title**
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
