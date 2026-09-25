# Phase W task scopes: tasks 41 to 60

Scope paragraphs for tasks 41-60 of `2026-09-25-phase-w-work-layer.md` (W1: the Work lens's held
changes, Triage, the board, display and bulk, Plan, the issue view, the editor, comments and
attachments, keys, the replica, the palette and peek). A part writer reads one paragraph and the
skeleton's row; where they disagree, **the row's id, dependsOn and seams win** (skeleton, *Task
scopes*).

**Conventions used below.**

- Paths are relative to the repository root; desktop paths are under `desktop/`. A path marked ⚑
  is shared (skeleton *File map*): the tasks that touch it run in task order.
- "05 §x" is `docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (revision 2);
  "L-n" is row n of the parity matrix in `01-linear-feature-inventory.md` as 05 §1 prices it.
  A row a task **closes** is built there; a row it only **serves** is closed by the task named.
- Seams are the skeleton's `SW.n` (*Seams* table). Every surface test calls
  `expectNoPrediction(container)` (SW.8, task 20) and reads its data through the replica
  (SW.16) or, before task 55 lands, through `workFixtures` (SW.8).
- "Mutant" names the one wrong implementation a test exists to kill.
- Every task follows the skeleton's *Global constraints* (Desktop): list-focus keys only, one
  heartbeat, `tabular-nums`, `transform`/`opacity` motion, Commit Mono facts at `--t-fact`, no Hub
  data on disk. Tests run with `npm --prefix desktop test` and `npm --prefix desktop run typecheck`.
- **Tasks 41-54 come before the replica** (55-58) in dependsOn but read and write through SW.16's
  signatures (`useWork(selector)`, `mutate(op) → {undo}`, `onRefused`). Until task 55 lands they
  test against the SW.8 fixtures behind those same signatures; no task here invents a second
  data path.

**41 — `arrivals.hold`, `moves.hold` and departures; `N` applies.** Files: new
`desktop/src/work/hold.ts` and `hold.test.ts` (a pure model; not yet in the skeleton's File map),
`src/components/work/HeldLine.tsx` (File map row, tasks 40-41), and edits to task 40's `WorkLens.tsx`,
`CriterionGroup.tsx`, `IssueRow.tsx` and `work.css`. Adds `type HeldChange = {kind:"arrival",
issueId, groupKey} | {kind:"move", issueId, fromGroup, toGroup, cause} | {kind:"departure", issueId,
groupKey, alias, why}`, `reconcile(prev: HoldState, next: GroupedIssues, focus: {groupKey: string |
null}) → HoldState` and `applyHeld(state, next) → HoldState`, where `HoldState` is the order the
reader sees plus the held list. Behaviour (05 §4.1 D C2): a new issue waits behind one `HeldLine` at
the top of its group; a row whose group or state changes **stays in place**, takes its new
`StateGlyph` (SW.12), flashes once over `--dur-flash` (SW.11) and shows one mono cause line (`→ done
· own evidence at 7c1f0a2 (test run, exit 0) · 14:02`); the group header adds `· 1 moving`; a
departure leaves a one-line ghost (`EP-128 left this view · cancelled`) until `N`; a row relocates
only on `N`, on leaving the view, or when focus is outside its group; more than three changes in
one group at once flash the header once and no row (D M9). `N` is bound on the list element's own
`onKeyDown`, so it fires only with focus in the list; task 53 moves it into `KEYMAP` unchanged. The
row exposes `data-changed="unseen"`, which task 34's reduced-motion layer draws as the static 2px
tick (05 §4.4 C7). Tests first: `reconcile` keeps an arriving issue out of the shown order until
`applyHeld` (kills a lens that inserts arrivals at once, so rows jump); a state change that re-groups
an issue keeps its index and records the cause (kills re-grouping on every replica change); focus
moving outside the group relocates the row without `N` (kills a model that only ever relocates on
`N`); a departure renders its ghost and `N` removes it (kills dropping the row at once); four
changes in one group give one header flash and zero row flashes (kills per-row flashing); the header
reads `3 issues · 1 moving` and `expectNoPrediction` passes (kills an `1 of 3` counter); unmount and
remount applies everything held (kills held state that outlives the view); `N` typed in a text field
inside the lens does nothing (kills a `document` listener). Depends on 40. Design: 05 §4.1 (*Rows
never move under the reader*), §4.4 C7 and work rule 3; 02 C7 `arrivals.hold` (`02:267`; 05's `N`
wins over 02's *R to show*). Parity: closes no row alone; it is Architecture rule 4, which L-101's
list (task 40), the board (43) and the Desk (62) rely on. Does not claim: FLIP re-sort (task 44),
the live-region announcement (66), the reduced-motion tick's styling (34, which is not upstream of
41 in dependsOn).

**42 — Triage: the view, number keys, `T` with a text-match criterion suggestion.** Files:
`src/components/work/TriageView.tsx` (+ test), new `src/work/suggestCriterion.ts` (+ test) and new
`src/components/work/CriterionPicker.tsx` (+ test), which tasks 46 and 47 reuse (neither new file is
in the File map yet); edits `work.css` and the Work lens's `ViewHeader` actions (SW.13) to switch to
Triage. Adds `interface CriterionSuggestion {briefKey; criterionId; text; matched: readonly
string[]}` and `suggestCriteria(issue: {title, description}, criteria, limit = 3) →
CriterionSuggestion[]`: shared normalised terms (lower case, diacritics and stop words removed),
ties in brief order; the picker shows the **matched words** as the why and never a score (AX
Principle 7; anti-pattern 12). Uses SW.1's `serves [{briefKey, criterionId}]`, SW.15's issue and
criterion types, SW.16's `mutate`, and the team policy of task 11. Keys, bound on the Triage list
element only: `1` accept into the team's default state (L-9's policy); `2` duplicate, which opens an
issue picker and writes the `duplicate` state and a `duplicates` relation as **one** undoable act;
`3` decline to `cancelled` with an optional one-line reason; `4` snooze (see *does not claim*); `T`
opens `CriterionPicker` seeded with the suggestions and writes a new revision whose `serves` gains
the criterion, after which the row shows `thread.node` (SW.14) instead of *not tied · imported*.
When the Hub refuses the accept because the team requires a priority (L-48; the code name is task
07/11's, **UNVERIFIED** here), the row returns with the `--fail` flash and the reason, and the
priority picker opens in place. Tests first: EP-142's text ranks *Uploads survive a dropped
connection* first (kills brief-order output); the suggestion row has matched words and no number,
and `expectNoPrediction` passes (kills a similarity percentage); `1` in a text field and `1` in the
Work list do nothing (kills a global number-key binding); `2` sends state and relation in one op with
one Undo (kills a state-only duplicate); a priority refusal rolls the row back and opens the picker
(kills an accept that never rolls back); an imported row reads *not tied · imported* until `T`
(kills hiding the thread state). Depends on 40. Design: 05 §4.3 (`1`-`4`, `T`), §3.3 (`triage`),
§2.3 point 4 (thread state), §5.3's demo. Parity: closes L-46 (the view; Desk rows are task 63),
L-47 (number keys; `Shift+R` accept-and-Start is task 61), L-48 (the refusal shown; the check is task
11), L-10 with task 48. Does not claim: gateway suggestions (L-51, L-52, W4); `4`'s record home,
because defer with a reason is W2 (05:1237) and no W1 task gives snooze a Hub field (**UNVERIFIED**:
if SW.1 carries none, `4` is a finding against task 03, not a desktop stub).

**43 — the board, and drag that offers a fact instead of faking it.** Files:
`src/components/work/BoardView.tsx` (+ test), edits to `WorkLens.tsx` (a List/Board switch in the
`ViewHeader` actions, SW.13, as `prototype/work-layer.html:553`) and `work.css`. Uses task 40's row
parts (`StateGlyph`, alias in mono, the right-hand fact, title in sans, the owner line, `AgentLine`),
task 41's hold model (a card never moves under the reader), SW.4's `StateMove{to, cause, reason}`,
SW.14's `useThreadLit(criterionId)` and SW.16's `mutate`. **Columns are criteria by default**, in
brief order, with *Serves no criterion* last (the prototype, `work-layer.html:645`; 05 §4.4 C13's
structural check fails a work screen whose default grouping is workflow state); a state board is the
`grouping: "state"` display option of task 44, never the default. An empty criterion column says
*No issue serves this criterion. C files one.* (`work-layer.html:661`), which is the uncovered
criterion as a fact. Drag uses pointer events and `transform` only, no new dependency. Drop rules:
into another criterion column writes a new revision whose `serves` is re-pointed from the source
criterion to the target (reversible, `optimistic.undo`); between human-band state columns
(`backlog`, `todo`, a W2 custom state) writes one `setState` as the person; into **a fact state**
writes **nothing**: the card returns and the column offers the fact in place: `started` → *Start a
lane? Shift+R* (the act is task 61's), `in_review` → *set when the worktree is marked ready or
queued*, `landed` → *set when a land commit carries its Panacea-Issue trailer*, `done` → the
reason step task 48 adds for a hand-set done. A person-set state shows beside the fact it is
waiting on (L-102, *a manual move shows both*). The keyboard path is `S` and `T` (tasks 46, 42), so
the board needs no drag to be usable. Tests first: default columns are criteria with *Serves no
criterion* last (kills a state-column default); a drop on `started` sends no op and shows the Start
offer (kills faking the fact); a drop on `landed` offers no override (kills a manual `landed`); a
`backlog`→`todo` drop sends exactly one op as the person (kills a no-op drop); a cross-criterion drop
writes one revision that removes the source criterion (kills appending both); an empty column renders
its sentence (kills hiding uncovered criteria); `work.css` carries no `transition: all` and drag
moves only `transform` (kills a layout-animating drag); `expectNoPrediction` passes on the board
(kills an `N of M` column total). Depends on 40. Design: 05 §4.1 (by criterion), §3.3 (the five
fact states), §4.4 C13 and C7. Parity: closes L-101's board (the list is task 40; the timeline is
W4's) and L-102's W1 part (drag offers the fact). Does not claim: sub-groups and swimlanes (L-102's
W2 part), a shared manual order inside a column (L-14, L-103, W2).

**44 — display options and the bulk bar, with *Start N worktrees* and `confirm.inplace`.** Files:
`src/components/work/DisplayMenu.tsx` and `BulkBar.tsx` (+ tests), new
`src/components/common/ConfirmInPlace.tsx` (+ test; not yet in the File map), edits to `WorkLens.tsx`,
`BoardView.tsx`, `work.css` and the built `src/store/slices/prefsSlice.ts`. The menu is the built
`Popover` (`src/components/common/Popover.tsx`). Adds `interface DisplayOptions {grouping:
"criterion" | "state" | "assignee" | "priority"; ordering: "priority" | "updated" | "created" |
"number"; density: "default" | "compact"; show: {assignee; labels; priority; agentLine; updated}}`,
kept per view as a **preference** in `prefsSlice` (rule 3: a preference is the desktop's; no Hub
record rides with it), default `grouping: "criterion"`; density maps to `--row-h` 32px and
`--row-h-compact` 28px (SW.11; 05 §4.4 C4); an explicit re-sort animates with FLIP, the only place it
may (C7). Adds the selection model `selection: ReadonlySet<IssueId>` in task 37's `workSlice`
(non-persisted): `X` toggles and `Shift+↓`/`Shift+↑` extend, bound on the list element until task 53
folds them into `KEYMAP`; a selected row keeps the ring and gains the check glyph (SW.12), so lit
and selected stay two shapes (05 §4.1 D I1). The bar appears at one selected row and offers state
(human bands), priority, assignee, labels, attach a criterion and cancel, each **one** `mutate` batch
with **one** Undo for the whole batch; and *Start N worktrees* (N ≥ 2), which shows a consequence
line (*Starts N agents on N worktrees · route Global*, the route from the project's data class,
`OD-24`, and no cost figure) and a two-step confirm inside the bar, then calls an injected
`onStart(ids)` that task 61 wires to SW.10. `ConfirmInPlace {consequence, confirmLabel, onConfirm,
onCancel}` never renders a dialog. Tests first: the default grouping is criterion and a chosen
grouping survives a remount (kills a state default and kills view-local state); the persisted prefs
contain display keys and no issue data (kills saving rows with the prefs); hiding labels removes row
chips and not the issue view's (kills a global toggle); bulk priority on three issues is one batch
reverted by one Undo (kills three chips, or an Undo that reverts only the last); the first *Start 3
worktrees* press calls nothing and shows the consequence, the second calls `onStart` once with three
ids, `Esc` cancels (kills an optimistic start); `ConfirmInPlace` has no `dialog` role (kills a
modal); compact rows read their height from the token (kills a hard-coded 28px); `expectNoPrediction`
passes. Depends on 40. Design: 05 §4.3 (`X`, `Shift+↓`), §4.6 (the two optimism classes), §4.4 C4
and C7; 02 C8 (`02:276-278`). Parity: closes L-104 and L-225 (its *Delegate N to my Orchestrator*
is W3's). Does not claim: *Set as default for everyone* (L-105), saved views (L-93), manual ordering
(L-103), all W2.

**45 — Plan (`G P`): Triage, Backlog, criteria with their issues, *My issues*.** Files:
`src/components/work/PlanSurface.tsx` (+ test), edits to `src/components/AppShell.tsx` ⚑ (the Plan
mount; task 40 mounted Work there and task 62 mounts the Desk, in task order) and `work.css`. Plan is
the **project** surface (05 §4.1's table); it mounts the way task 40 mounts Work (a `workSlice`
surface value, not a seventh lens tab; **UNVERIFIED** until task 40's part fixes the mechanism) and
is reached in W1 from a *Plan* action in the Work lens's `ViewHeader` (SW.13); `G P` is bound by task
54 and the palette verb by task 59. W1's Plan holds **exactly four sections, in order**: *Triage*
(task 42's `TriageView`, embedded), *Backlog* (issues in `backlog`, grouped by criterion),
*Criteria* (every criterion of the project's briefs, each with its convergence word `settled`,
`open`, `contested`, `drifting` or `uncovered`, a plain issue count and its issues under
`thread.line`, SW.14) and *My issues* (assignee is the signed-in person, from `/v1/auth/status`'s
account, grouped by criterion). The features board, cycles, initiatives and saved views are **not
drawn at all** until their slices bring them, because 05 §4.1 shows a noun only once a team has one.
A criterion's convergence word is **read** from phase 4's criterion state (the planned
`desktop/src/provenance/loader.ts`, P4 tasks 16-17, **UNVERIFIED until merge**), never computed on the
desktop from issue states, because *done* and *settled* are two facts (05 §3.3). Uses SW.15, SW.16's
`useWork`, SW.12's glyphs and task 41's hold model in every section. Tests first: the four sections
render in order and nothing else (kills empty W2 placeholders); a criterion no issue serves reads
`uncovered` and stays listed (kills hiding it); a criterion whose three issues are all `done` but
whose evidence does not hold at the head reads `open` (kills deriving `settled` from issue states);
*My issues* lists the reader's assigned issues and not issues whose agents the reader owns or
issues the reader subscribes to (kills the wrong filter); every group size is a plain count and
`expectNoPrediction` passes (kills a coverage percentage). Depends on 42. Design: 05 §4.1 (Plan
row), §4.5 (Discover marks an unserved criterion), §3.3 (convergence words), §4.2 (the Desk, not
*My issues*, is the ranking). Parity: closes L-112's view (the ranking is the Desk, tasks 62-65) and
L-46's second home; serves L-101. Does not claim: the features board (L-67 onward), cycles (L-57
onward), initiatives, saved views and favourites (W2, W4).

**46 — the issue view, part 1: head fields and their pickers.** Files:
`src/components/issue/IssueView.tsx` and `IssueHead.tsx` (+ tests), and new
`src/components/issue/pickers/{StatePicker,PriorityPicker,AssigneePicker,LabelPicker}.tsx` (+ tests;
not yet in the File map), which the row keys `S`, `P`, `A`, `I` and `L` open in task 53. The view uses
the one `ViewHeader` (SW.13) with the route badge `Global`/`AU`; the head shows the title in sans, the
alias `EP-142` in mono from `DisplayAlias(projectKey, n)` (SW.1), which is **display only**: every
link, key and write uses the `is-` id (05 §3.6). Facts (alias, times, ids) are Commit Mono at
`--t-fact` in `--ink-2` or stronger, never `--ink-3` (SW.11). *State*: the nine states with their
`StateGlyph` (SW.12); `backlog`, `todo`, `cancelled`, `duplicate` (with a duplicate-of picker, as
task 42's `2`) and `done` on an issue with an empty `serves` are selectable; `started`, `in_review`
and `landed` are **listed with the fact each waits on and cannot be chosen**; `done` on an issue that
serves criteria is marked *needs a reason*, and task 48 adds that step. Writes go as SW.4's
`StateMove{to, cause, reason}` through SW.16's `mutate`. *Priority*: the five levels `none`, `urgent`,
`high`, `medium`, `low`, drawn as the glyph's notch in ink, never a colour, urgent included (05 §4.4
work rule 2). *Assignee*: people only, never an agent or a session (`OD-16`); *Assign to me* first.
*Labels*: the team's labels (task 11). When the issue has **another owner's live agents** (a derived
fact from their sessions' `IssueRef`, task 19), the assignee picker's commit becomes *Ask Dylan to
hand over*, a reassign request through SW.2 (the method name is task 04's, **UNVERIFIED** here), and
the head reads *reassign requested by Alice* until the owner decides on the Desk (task 65; 05 §3.7
rule 2). Tests first: choosing `started` is impossible and sends nothing (kills a picker that fakes a
fact); the urgent notch uses no `--need` or other signal colour (kills colour-coded priority); the
assignee list has no agent or session entries (kills an agent assignee); a reassign on an issue with
someone else's agents sends a request and leaves the assignee unchanged (kills steering another
person's agent); changing the fixture's project key changes the alias and no link or key (kills the
alias used as a key); no fact element carries the `--ink-3` class (kills an unreadable fact);
`expectNoPrediction` passes. Depends on 37, 39. Design: 05 §4.1 (issue view), §3.3, §3.6, §3.7 rules
1-2, §4.4 C3, C5 and work rules 1-2. Parity: closes L-13, L-38, L-17's team labels (org labels and
groups are W2), L-181, and the display half of L-3 (with task 08); L-8's W1 part (fact states shown as
facts). Does not claim: estimate `E` (L-16), due date (L-37), cycle and feature fields (W2).

**47 — the issue view, part 2: revisions, restore, `serves` with `thread.node`, revision-pinned
links.** Files: `src/components/issue/Revisions.tsx` and `Serves.tsx` (+ tests), edits to
`IssueView.tsx`, and new `src/work/links.ts` (+ test; not yet in the File map). Uses SW.1's
immutable, digested revision `issue:is-…@r` and its `serves [{briefKey, criterionId}]`, SW.14's
`<ThreadNode/>`, `<ThreadLine/>` and `useThreadLit(criterionId)`, task 42's `CriterionPicker` and
SW.16's `mutate`. *Revisions*: newest first, each with `@r` in mono, the person, the author class
`text·H`/`text·A` as a mark shape and a word (never colour alone), the time, and what changed in
words (title, description, `serves`); any revision opens read-only with a line diff against the
head; *Restore* writes a **new** revision whose content equals the chosen one and never rewrites
history (reversible, `optimistic.undo`). The view opens **at the revision a chip names** (05 §4.5
rule 1); on a non-head revision it says *You are reading @3; the head is @5 (Alice, 11:02)* with a
link to the head. A revision an agent of the reader was given (the session's `IssueRef`, task 19)
reads *dylan · agent 1 read @3*, which is §3.7 rule 1's basis. `links.ts` adds
`issueLink(id, rev?) → "panacea://issue/is-…@r"` and `parseIssueLink(s) → {id, rev} | null`, which
accepts only the `is-` form and never an alias (05 §3.6); *Copy link* pins the revision being read,
*Copy link to head* does not. *Serves*: each criterion in sans with its convergence word, the brief
key `brief:bf-…@n` in mono and a `thread.node`; hover **or focus** lights the criterion across
Work, Design and Develop and never rings the row (05 §4.1 D I1); `T` adds through the picker and
removal writes a new revision. Tests first: restoring @2 on a head of @3 creates @4 equal to @2 and
keeps @2 and @3 (kills an in-place rewrite); the author class is present as text on every revision
(kills colour-only authorship); *Copy link* on @3 yields `…@3` (kills links that always point at the
head); `parseIssueLink("EP-142")` returns null (kills alias parsing); focusing a `serves` row lights
its criterion and adds no selection ring (kills lit-as-selected); removing a criterion sends a
revision write, and Undo returns it (kills a mutable field patch); opening at @3 shows the head line
(kills silently showing the head); `expectNoPrediction` passes. Depends on 36, 46. Design: 05 §2.3
(the thread), §3.6, §3.7 rule 1, §4.4 C2, §4.5 rules 1 and 4. Parity: closes L-34 and L-98's W1 part
(the revision-pinned `panacea://` link); serves L-191 (task 19 writes the revision into the managed
block; this view shows which one). Does not claim: the https Hub link and read-only page (L-98's W2
part, task 80), registering the `panacea://` scheme with the OS (**UNVERIFIED** whether the desktop
has it), moving an issue between projects (L-35, W2).

**48 — the issue view, part 3: relations, the live agent line, *done, not proven* and its reason.**
Files: `src/components/issue/Relations.tsx` and `IssueAgentLine.tsx` (+ tests), edits to `IssueView.tsx`
and to task 46's `StatePicker.tsx` (the reason step). *Relations*: `blocked by`, `blocking`,
`related`, `duplicate of` (task 11's kinds), each row with the alias in mono, the title, its glyph and
owner; one relation record shows on **both** issues; add and remove through SW.16's `mutate`, opened
from the view here and from `M` then `B` `X` `R` `D` in task 54. A relation to an issue another person
owns is recorded and, in solo mode, reaches that owner as an *inform* (task 65's row), never their
agents (05 §3.7 rule 3's W1 part). A blocker that stalls someone's agent shows the fact and the wait
in mono (*blocking dylan · agent 1 · 42m*); the Desk row with its cost is task 63. *The live agent
line* wraps task 40's `AgentLine`: one line per **own** agent on the issue, `who.label` in mono then
MEANING in sans, one dot each on the one heartbeat (`useHeartbeat`, `desktop/src/hooks/heartbeat.ts`);
a teammate's work is the person and one line of what they touch, with no dot and no count; liveness
coarsens (`live` under 10 s, then `quiet 2m`) only on the beat; *open session* is a link. The line's
source is the control plane's sessions with an `IssueRef` (task 19) and lane events, **never PTY
bytes** (`OD-8`). *Done, not proven*: for an issue that serves criteria, `done` in the picker asks
a one-line reason (required, trimmed non-empty) and sends `StateMove{to: "done", cause, reason}`
(SW.4); the issue then carries `StateGlyph notProven` (SW.12) in every place it appears and the head
shows the reason, its author and time. The done rule's **result words** come from SW.4 and are
rendered, not computed: *done · own evidence at 7c1f0a2 (test run, exit 0) · 14:02*, *proven,
self-attested*, *done · c-91e2 still open (EP-150 not landed)*. Tests first: a reasonless hand-done on
a serving issue cannot be sent (kills a done without reason); with a reason, the row, the board card
and the view all draw `notProven` (kills an ordinary done glyph); observed evidence renders its commit
and exit code and no *not proven* (kills conflating the two); a self-attested result says so (kills
hiding it); a teammate's issue renders no dot and no count (kills fleet dots for others);
`IssueAgentLine` imports nothing from `src/api/attachRegistry.ts` or the terminal (kills PTY-derived
meaning); a relation added on EP-142 shows as `blocking` on EP-150 (kills a one-sided relation); a
cross-owner relation sends no act at the other owner's session (kills steering); no component here
starts a timer of its own (kills a per-row clock); `expectNoPrediction` passes. Depends on 46.
Design: 05 §3.3 (the done rule), §3.7 rule 3, §4.1 (one agent line, D C3, D I10), §4.4 C10. Parity:
closes L-24's W1 part (the kinds and the stalled-agent fact; land order is phase 3's, resolved-blocker
becomes related is L-25, W2), L-10's relation, L-39's W1 part (the delegate is the assignee's own
agent) and, with task 46, L-181. Does not claim: sub-issues (L-20), the W3 negotiation line (§4.1).

**49 — the editor, part 1: one Markdown editor on `prosemirror-*` (MIT), author class kept.**
Files: new `src/components/editor/{Editor.tsx,schema.ts,markdown.ts,authorClass.ts,editor.css}`
(+ tests) and `desktop/package.json` ⚑, which gains `prosemirror-model`, `-state`, `-view`,
`-transform`, `-commands`, `-keymap`, `-history`, `-inputrules` and `-markdown` (with `markdown-it`
under it), each with a licence line (MIT; the part confirms each from `package-lock.json` at
install, versions **UNVERIFIED** here). Adds `interface EditorProps {value: MarkdownDoc; onCommit(next:
MarkdownDoc): void; readOnly?: boolean; ariaLabel: string}`, `type MarkdownDoc = {markdown: string;
authorship: readonly AuthorSpan[]}`, `interface AuthorSpan {from: number; to: number; cls: "text·H" |
"text·A"}` over offsets in the Markdown string, and `parseMarkdown(md) → Node`,
`serializeMarkdown(doc) → string`; `AuthorSpan` maps onto SW.1's authorship field on a revision (the
field name is task 03's, **UNVERIFIED** here). One editor serves the issue description here and
comments in task 51. Markdown shortcuts are input rules (`# `, `- `, `1. `, `> `, a fence, `**`,
`` ` ``, `[ ] `); raw HTML is off (`markdown-it` `html: false`); only `https:`, `http:` and
`panacea:` links are live; nothing embeds (L-128 is a SKIP: links stay text). **Author class is
kept** (TM-05 §3.3 as 05 L-124 cites it): an agent passage stays `text·A`; a person's edit inside it
turns only the edited range `text·H`; pasted text is the person's; an agent passage carries a gutter
mark (`<MarkGlyph kind/>`, SW.12) and a word in its accessible name, never colour alone. The
description commits as **one revision write** on blur, on `Ctrl+Enter`, or after 3 s idle (a
proposed cadence, the owner's to judge), never per keystroke; editor undo is ProseMirror's history,
separate from the list's `U`. Text is sans (`voice.split`, 05 §4.4 C3); code is Commit Mono. Tests
first: the fixture set round-trips byte for byte (kills a serializer that rewrites `*` as `_` or drops
a trailing newline); `<script>` in Markdown renders as text (kills `html: true`); a `javascript:` link
is not live (kills an unsafe href); typing inside an agent passage splits it and marks only the typed
range `text·H` (kills flipping the whole passage, and kills losing the agent mark on any edit); an
agent passage has a non-colour mark and a word (kills colour-only authorship); `C`, `S`, `N` and `1`
typed in the editor insert characters and fire no work verb (kills a `keys.list` leak into text
fields); one blur sends one revision write (kills per-keystroke writes); every added dependency's
licence in the lock file is MIT (kills a copyleft addition). Depends on 46. Design: 05 §4.4 C3, §3.4
(issue text reaches an agent only as adopted spec-class text after the Hub's checks), §1 row I.
Parity: closes L-124 and L-126's Markdown half (the slash menu is task 50); serves L-34 (each commit
is a revision, task 47). Does not claim: multiplayer editing (L-121, team mode's TM2), the Discover
brief editor (phase 4's), drafts (L-33, W2), agent edits shown as proposals (L-130, W4).

**50 — the editor, part 2: the slash menu and tables.** Files: new
`src/components/editor/{slashMenu.ts,SlashMenu.tsx,tables.ts}` (+ tests), edits to task 49's
`schema.ts` and `markdown.ts`, and `desktop/package.json` ⚑ (`prosemirror-tables`, MIT, with its
licence line). Adds `const SLASH_ITEMS: readonly SlashItem[]` with `interface SlashItem {id; label;
shortcut: string; run(view): void}`: headings 1-3, bulleted, numbered and check lists, quote, code
block, divider, table, *Link an issue* (searches the replica by alias and inserts the `is-` link,
shown as its alias; task 47's `issueLink`) and *Link a criterion* (inserts the brief key and
criterion id, shown as the criterion's words). Each item shows its Markdown shortcut as a `kbd` hint
(`--t-micro` mono, 02 C9) read from the **same table** the input rules use, so the menu teaches the
shortcuts and cannot drift from them (L-226's rule, inside the editor). `/` opens the menu only at
the start of a block or after whitespace; typing filters; `Enter` inserts; `Esc` closes and leaves
the literal `/`. The menu is a `listbox` with `aria-activedescendant`, placed at the caret without
scrolling the document. Tables are GFM pipe tables: `markdown-it` parses them and `tables.ts` adds
the serializer; `Tab` and `Shift+Tab` move between cells; add and remove a row or column from the
menu. Tests first: `/` at a block start opens the menu and `/` inside a word types a slash (kills an
always-open trigger); `tab` filters to *Table* and `Enter` inserts a table with a header row (kills a
broken filter); a table serialises to a pipe table that re-parses to an equal document (kills lossy
tables); `Esc` leaves `/` in the text (kills eating the character); every item's hint equals its input
rule's trigger (kills a second, drifting table); *Link an issue* inserts an `is-` link, never the
alias text as the target (kills an alias key); opening the menu does not change the document's scroll
position (kills a scroll jump). Depends on 49. Design: 05 §1 row I (L-126, L-127), §4.4 C3, §3.6.
Parity: closes L-126's slash menu and L-127's table part. Does not claim: callouts (task 92, W2),
Mermaid and collapsibles (L-127's *later* part), embeds (L-128, SKIP).

**51 — comments: the thread, mentions, reactions, anchors, resolve.** Files:
`src/components/issue/Comments.tsx` (+ test; any split-out sibling under `src/components/issue/` is
not yet in the File map) and an edit to `IssueView.tsx`. Uses SW.1's comment `ic-` shape and task
12's rules (append-only; mentions, reactions, anchors and resolve on the server), task 49's `Editor`
for writing (no second editor), SW.16's `mutate` for every write and SW.12's `MarkGlyph` for the
author class. W1 comments have one kind; W3 adds `note`, `evidence_proposal` and `negotiation`
(SW.19). **Anchors**, rendered for all five kinds SW.1 carries (issue text at a revision with its
quote, a brief section, a criterion, a transcript span, a diff hunk; the field names are task 03's,
**UNVERIFIED** here): the issue view creates the issue-text anchor (select text in the description,
then *Comment*) and the criterion anchor (from a `Serves` row); span and hunk anchors are created
from peek (task 60) through the same `addComment({anchor})` call. An anchor whose text changed in a
later revision shows *on @3* with its quote and is never silently re-mapped. **Mentions**: `@` in the
editor opens a people picker of the team and stores the user id, rendered as the name. **Reactions**:
a fixed small emoji set (our choice; custom emoji are *later*, L-41), toggled per person, shown with
the people's names and no number (05 §4.4 work rule 3). **Resolve** folds a thread to *Resolved by
Priya · 11:02* and can be undone. A correction is a reply, or task 12's edit record if SW.1 carries
one (**UNVERIFIED**); nothing rewrites a posted comment. An agent's comment (`text·A`, W3 onward, since
in W1 an agent writes only on its owner's behalf, 05 §3.6) carries the agent mark and a word. A new
comment arriving while the reader is scrolled up never moves the scroll position. Tests first:
posting sends one append whose Markdown and authorship come from `Editor` (kills a second text path);
a text anchor stores the revision and the quote, and after the description changes it shows *on @3*
(kills re-mapping offsets onto a new revision); a mention stores the user id, so a renamed person still
resolves (kills name-text mentions); the same person reacting twice ends with no reaction (kills a
double add); reactions render names and no digit (kills a reaction count); resolve then Undo reopens
the thread (kills a one-way resolve); an agent comment carries its mark and word (kills colour-only
authorship); an arrival while scrolled up leaves `scrollTop` unchanged (kills a scroll jump);
`expectNoPrediction` passes. Depends on 49. Design: 05 §3.6 (who writes comments), §3.4 (comments never
enter an agent's managed block; task 19 tests that), §4.1 (issue view), §1 row B. Parity: closes L-41's
W1 part and L-42. Does not claim: thread summaries (L-43, W4), comment drafts (L-44, W2), custom emoji.

**52 — attachments by blob digest; subscribe and unsubscribe.** Files:
`src/components/issue/Attachments.tsx` (+ test), new `src/work/blob.ts` (+ test; not yet in the File
map), edits to `IssueHead.tsx` (the subscribe control) and `IssueView.tsx`. Uses the Hub's
content-addressed blobs, `PUT` and `GET`/`HEAD /v1/projects/{projectId}/blobs/{sha}`
(`hub-contract.md:590-591`, §7.6), through the control plane (whether the relay table carries the blob
routes is **UNVERIFIED**; the part checks contract §4.3 and a missing row is a finding against task
16), and SW.1's attachment record naming a digest (task 09). `blob.ts` adds `sha256Hex(bytes) →
Promise<string>` on `crypto.subtle.digest` and `uploadAttachment(projectId, file) →
Promise<{digest, name, size, contentType}>`, which hashes **before** the upload and names the blob by
that hash; the Hub re-checks (`digest_mismatch` 422). Refusals show their fact in mono:
`blob_too_large` 413 with `limitBytes` (16 MiB default, `hub-contract.md:261`, `:2172`) and
`quota_exceeded` 409 (1 GiB a project, `:260`). A download is re-hashed and a mismatch is not opened.
PNG, JPEG, GIF and WebP preview from their bytes through an `<img>` with an object URL; every other
type, SVG included, is a file row, so no attachment runs script. Bytes stay in memory; nothing is
cached to disk; only the person's explicit *Save as* writes a file (the part checks this against
`ADR-H1:467-474`). **Subscribe**: the head shows the Hub's reason (*Subscribed · you commented*;
create, assign, mention and comment subscribe on the server, task 13) and one control to unsubscribe
or subscribe, written as the person; subscribe has **no single key**, because Linear's `Shift+S`
does nothing here (05 §4.3 rule 3), and it is a palette verb in task 59. Tests first: the upload's
path sha equals the locally computed hash of the bytes (kills trusting a server-reported hash); a 413
renders `limitBytes` (kills a generic error); a download whose bytes do not hash to its digest is
not opened (kills trusting the bytes); an SVG attachment never enters the DOM as markup (kills script
in an attachment); no `localStorage`, IndexedDB or file-system call happens during upload or view
(kills a blob cache); the subscribe reason is the Hub's field verbatim (kills a reason guessed on the
desktop); unsubscribe sends one op and Undo restores (kills a silent toggle). Depends on 51. Design:
05 §5.3 rows 4 and 12 (both pulled into W1), §3.1 (three homes), §4.3 rule 3. Parity: closes L-45 and
L-117, each moved into W1 by 05 §5.3. Does not claim: the Desk digest for subscriptions (L-116, W2),
subscribing to a view (L-96, W2), per-person notification settings.

**53 — keys, part 1: `keys.list`, `isCockpitChord` extended, `event.code` for agent keys, the dead
Linear chords, the single-key switch.** Files: new `src/work/keys/keymap.ts` (+ test), edits to
`src/hooks/useKeyboardMap.ts` ⚑ (+ its tests), `src/store/slices/prefsSlice.ts` (one preference) and,
for the switch, one toggle row in `src/components/palette/commands.ts` ⚑ (before task 59 in task
order; the File map names only 59 there). Adds `type KeyScope = "list" | "issueRow" | "triage" |
"deskDecision" | "everyRow"`, `interface KeyBinding {verb: VerbId; keys: readonly string[]; code?:
string; shift?: boolean; scope: KeyScope; slice: "W1" | "W2" | "W3"; label: string; linear: "match" |
"depart" | "none"}`, `const KEYMAP: readonly KeyBinding[]` (05 §4.3's table, W1 rows active, W2 and
W3 rows present and inert), `const DEAD_LINEAR_CHORDS` (`Shift+D`, `Shift+E`, `Shift+P`, `Shift+M`,
`Shift+S`, each with our equivalent), `isListKeyTarget(target) → boolean` (inside a
`[data-work-list]` grid or listbox; false inside a terminal host by `isInsideTerminal`,
`useKeyboardMap.ts:90`, and in an input, textarea, select or contenteditable),
`matchListKey(event, scope) → KeyBinding | null`, and `registerVerb(verb, run) → unregister`, the one
registry every row verb goes through. **The chord predicate is extended by a sibling branch, not by
new keys**: `useKeyboardMap` gains a list-focus branch that runs only when no Ctrl, Meta or Alt is
held, and `CHORD_KEYS`, `SHIFTED_CHORD_KEYS` (`:55-56`) and `isCockpitChord`'s pinned source (`:45-80`)
stay unchanged, because a key added to those lists is one xterm skips and the agent stops receiving
(`:63-71`); `OD-9`'s toggle keeps meaning what it means. Agent keys match the physical key:
`event.code === "KeyR" && event.shiftKey`, never `event.key`. **No `Shift`+letter is a record act**:
range select is `Shift+↓`/`Shift+↑`. The list-local bindings of tasks 41, 42 and 44 (`N`, `1`-`4`, `X`,
`Shift+↓`) move into `KEYMAP` unchanged. The single-key switch (`singleKeyShortcuts`, a persisted
preference beside `sendChordsToTerminal`, `prefsSlice.ts:40`, `:69`) turns every single key and
sequence off and leaves chords and the palette (WCAG 2.1.4); 05 places it in *Settings → Keyboard*,
which the desktop does not have yet, so it is a palette toggle row on `OD-9`'s pattern
(`commands.ts:19-28`) until one exists. Tests first: `S` in a work list opens the state picker and `S`
in an input, the editor or a terminal host does nothing (kills a document-level single key); `Ctrl+K`
still opens the palette from a list, and both chord lists equal their pre-task values (kills a list
branch that swallows chords, and kills single keys added to the chord lists); `Shift+R` with code
`KeyR` starts, `R` from Caps Lock without Shift does not, and an AZERTY `event.key` with code `KeyR`
still starts (kills `event.key` matching); each dead chord dispatches no verb (kills a Linear chord
wired to an agent or a subscribe); a structural test over `KEYMAP` finds every Shift+letter bound to
an agent verb (kills a Shift record act); with the switch off `S`, `N` and `1` do nothing and `Ctrl+K`
works (kills a switch that misses sequences or kills chords); `Shift+O` and `Shift+A` are inert in W1
(kills an Orchestrator key before W3). Depends on 40. Design: 05 §4.3 (the four rules and the table);
02 C9 (`02:292`); the skeleton's *Global constraints*, Keys. Parity: closes L-223's single-key half
(sequences are task 54); binds L-47's and L-38's keys. Does not claim: `E`, `Y`, `B`, `D` (W2),
`Shift+O` and `Shift+A` (W3), the verbs' own behaviour (their tasks).

**54 — keys, part 2: `G`, `O` and `M` sequences, the generated `?` sheet, chords in menus, the safe
triangle.** Files: new `src/work/keys/sequences.ts` (+ test), `src/components/work/ShortcutSheet.tsx`
(+ test), new `src/components/common/menuAim.ts` (+ test; not yet in the File map), edits to task
53's `keymap.ts` and to the menus built on `Popover.tsx` (task 44's `DisplayMenu`, task 46's pickers).
Adds `const SEQUENCES` (`G` then `I` Desk, `M` my issues, `T` Triage, `B` backlog, `W` Work, `P`
Plan, and inert until their slice `V` current cycle and `O` my Orchestrator; `O` then `I` open by
number, `P` a project, and inert `C` and `F`; `M` then `B` `X` `R` `D`, which open task 48's relation
picker with `blocked by`, `blocking`, `related` or `duplicate of` chosen), a pending-sequence state
with `SEQ_WINDOW_MS = 1200` (a proposal, recorded not measured) shown as a `G …` `kbd` hint in the
`ViewHeader` (SW.13), cancelled by `Esc` or an unknown key; `chordFor(verb) → string | null`, which
every menu item renders so a menu teaches its keys (02 `:305`); and `useMenuAim(parentRect,
submenuRect)`, the submenu safe triangle as **geometry with no timer** (02 `menu.aim`, `02:265`),
written from the idea and not from anyone's code (`OD-11`). `O I` calls the verb `issue.openByNumber`,
which task 58 registers. The `?` sheet is **generated** from `KEYMAP`, `SEQUENCES` and
`DEAD_LINEAR_CHORDS`: active bindings only, grouped by scope, the five Linear chords that do nothing
here with our equivalent for each, the line *Space peeks*, and the single-key switch's state (05 §4.4
work rule 5). Same list-focus rule as task 53. Tests first: `G` then `P` opens Plan, and the same keys
in a text field do nothing (kills sequences outside list focus); `G`, a wait past the window, then
`P` does nothing (kills an unbounded pending prefix); `G V` before W2 does nothing and the sheet does
not list it (kills listing inert keys); `M B` opens the relation picker on `blocked by` (kills the
wrong kind); `O I` reaches a stub `issue.openByNumber` (kills an unrouted verb); the sheet's rows are
exactly `KEYMAP`'s active rows (kills a hand-written, drifting sheet); a menu item shows the new chord
after its binding changes in `KEYMAP` (kills hard-coded hints); a pointer path across a sibling item
inside the triangle keeps the submenu with no timer advanced (kills a hover-timeout menu), and a path
outside it switches at once (kills a sticky submenu). Depends on 53. Design: 05 §4.3, §4.4 work rule 5;
02 C7 and C9. Parity: closes L-223 (with 53), L-224, L-226's menus (the palette's hints are task 59)
and L-227. Does not claim: open by number itself (task 58), the Desk and Orchestrator targets'
surfaces (tasks 62-65, W3).

**55 — the replica, part 1: in memory, full load, the cursor on `/v1/changes`; the no-persistence
test.** Files: new `src/work/replica/replica.ts` (+ test) and `src/work/replica/noPersistence.test.ts`.
Adds `interface ReplicaDeps {listAll(projectId): Promise<WorkSnapshot>; changes(after: string):
Promise<{events: HubEvent[]; next: string; more: boolean}>; fetchSubjects(ids): Promise<WorkRecord[]>}`
over SW.2's relay (`/v1/hub/work/*`, task 16; the list convention `?limit=&after=` →
`{items, next}`, `hub-contract.md:48-49`) and the relayed feed `GET /v1/hub/changes?after=`
(`:1372`); `createReplica(deps) → Replica` with `status: "empty" | "loading" | "ready" | "resyncing"`,
`cursor`, `get(id)` and `byProject(projectId)`; and `useWork<T>(selector) → T`, the read half of
SW.16. **Order of the first load**: take the head with `after=latest` (`:1101`), load every readable
project's Work records, then read the feed after the cursor taken **before** the load, so a write made
during the load is not lost; apply by `seq`, drop an event at or below the last applied `seq`, and ask
again at once on `more: true` (`:1099-1100`). Feed events are metadata only (SW.5), so applying one
**re-fetches its subject** in a batch; the event's `data` is never used as the record. Afterwards the
replica reads the feed **once per change** of the `Panacea-Hub-Cursor` header on `GET /v1/sessions`
(`:1377-1385`), which phase 4 task 16's loader reads (**UNVERIFIED until merge**, rule H2; the
skeleton's dependsOn for 55 names only task 37). The replica lives outside the persisted store and
beside task 37's non-persisted `workSlice`. **The no-persistence test** has two halves: a static scan of
`src/work/**` and `src/components/{work,issue,editor,desk}/**` for `localStorage`, `sessionStorage`,
`indexedDB`, `caches.open`, `@tauri-apps/plugin-fs`, `@tauri-apps/plugin-store`, `writeFile` and
`writeTextFile`, expecting none; and a runtime spy on `Storage.prototype.setItem` and `indexedDB.open`
through a full load and 100 events, expecting only the preference keys the store already persists,
with the store's `partialize` output checked for Work keys as `store.test.ts:335` does. Tests first: an
event committed during the load is applied (kills loading and then asking `after=latest`); a repeated
event changes nothing (kills double-apply); `more: true` fetches again in the same turn (kills waiting
a poll tick); an unchanged cursor header causes no feed read and a changed one causes exactly one
(kills a feed read per tick); applying `issue.revised` fetches the subject (kills using event metadata
as content); `useWork` re-renders a component only when its selection changes (kills whole-tree
renders); both no-persistence halves pass (kills a disk cache for a fast return). Depends on 37.
Design: 05 §5.3 (*The replica, reconciled with 02*), §4.4 (*One correction to 02*: in memory), §3.1;
`ADR-H1:467-474`; 02 C8 `hub.replica` and `sync.checkpoint` (`02:274-275`); hub-contract §7.11. Parity:
closes L-228 with task 56 and L-229's W1 part with task 57; L-230 is a SKIP and stays one. Does not
claim: `budget.return ≤ 2s` after a restart (05 §4.4 marks it **UNVERIFIED** for large projects;
task 70 records it), writes (task 56), visibility and resync (task 57).

**56 — the replica, part 2: optimistic writes, rollback on refusal, `optimistic.undo`, `U`.** Files:
new `src/work/replica/ops.ts` and `undo.ts` (+ tests), new `src/components/common/UndoChip.tsx` (+
test; not yet in the File map), and edits to `replica.ts`. Adds `type WorkOp` (a closed union:
`setState` with SW.4's `StateMove`, `setPriority`, `setAssignee`, `labels {add, remove}`, `revise
{baseRev, patch}`, `relation {add | remove}`, `comment`, `reaction`, `resolve`, `subscribe`, each
naming its issue), `mutate(op | op[]) → {undo(): void; settled: Promise<"applied" | Refusal>}`,
`onRefused(listener) → unsubscribe` and `interface Refusal {op; code; message; issueId}`: the write
half of SW.16. `mutate` changes the replica in the same frame (`budget.paint ≤ 16 ms`, 05 §4.6,
recorded not asserted), sends through SW.2 with an `Idempotency-Key` that is **reused across retries**
(`hub-contract.md:2256`), and reconciles with the Hub's returned record. On a refusal it reverts
**only the op's own fields**, so a teammate's change that arrived meanwhile survives; the row flashes
`--fail` with the Hub's code and message; `onRefused` fires once (task 66 speaks it). A
`revision_conflict` 409 on `revise` (`:2250`) keeps the person's text in memory and offers it again on
the new head. Irreversible acts are **not** in `WorkOp` (erase, land, push and deploy are
`confirm.inplace`, task 44), which a `@ts-expect-error` test pins. `UndoChip` counts `--undo-window`
(6000 ms, SW.11) in `tabular-nums` seconds and **pauses while its row has focus or hover** (05 §4.6);
a batch is one chip; `U` (registered through task 53's `registerVerb`) undoes the latest op still in
its window by sending the **inverse op as a new write** by the person; after the window the revision
history (task 47) is the way back. Ops queue in memory across a network drop and never survive a
restart. Tests first: the row repaints before the request resolves (kills paint-after-server); a
refusal after a concurrent teammate event reverts only the op's field (kills whole-record snapshot
restore); a refusal shows code and message and fires `onRefused` once (kills a silent rollback); a
retry reuses the key (kills duplicate writes); with fake timers a hovered chip does not lapse at 6 s and
lapses after the hover ends (kills an unpausing countdown); `U` sends the inverse op (kills a local-only
undo); a three-issue batch is one chip that reverts all three (kills per-op chips); a conflict keeps the
text (kills losing a person's words). Depends on 55. Design: 05 §4.6 (*Undo, revision 2*), §5.3 row
14; 02 C8 (`02:276-278`). Parity: closes L-36 and, with task 55, L-228. Does not claim: an offline queue
that survives a restart (L-230, SKIP), the undo announcement (task 66), Start's *Undo Start* (task 61).

**57 — the replica, part 3: only what the caller may read; resync on a gap; erase.** Files: new
`src/work/replica/visibility.ts` (+ test) and edits to `replica.ts`. Uses the Hub's per-caller filter
(the feed carries an event only for a project the caller can read or a team the caller is in,
`hub-contract.md:1103-1104`, matrix row `:357`), the membership events `team.member_set` and
`team.member_removed` (`:1140-1141`), SW.5's Work kinds and task 14's erase. Rules: the replica lists
Work records **only** for projects the caller can read, and a subject fetch answered 403 or 404 drops
that record at once. `team.member_removed` naming the caller drops every record of that team's
projects in the same tick, and an open view of one says *You no longer have access to EP* as a fact.
`team.member_set` that makes a project newly readable triggers a **full load of that project**,
because its earlier events were filtered out of this caller's feed and will never arrive. **A gap
cannot be seen by `seq`**: the org's sequence is gapless but filtered per caller (`:1099`, `:1103`),
so the replica never checks contiguity. It resyncs on: a feed `next` below the last applied `seq`
(a restored Hub: full resync); a relay error or `upstream_unavailable` 503 followed by recovery
(resume from the last applied cursor, no full load); and the membership events above. During a
resync `status` is `"resyncing"`, the current rows stay on screen, one fact line says *Reloading ·
since 11:02* (no spinner, no per-row state), and the result enters through task 41's hold model, so
nothing moves under the reader. **Erase**: an erase event for an issue or a comment (kind names are
task 13's, **UNVERIFIED** here) replaces its text in memory with the `erased` resolution and keeps
the key, digest and number, so chips and links still resolve, as *erased* (05 §3.6). Tests first:
removal from a team drops all its records in one tick (kills a drop at the next load); being added
loads the new project in full (kills waiting for feed events that never come); a 403 subject fetch
drops the record (kills keeping it); against a fake Hub, the replica never requests a project outside
the caller's list (kills listing every org project); a `next` below the last applied `seq` resyncs
while the old rows stay visible (kills ignoring a restore, and kills a blank flash); an outage and
recovery resumes from the cursor with no full load (kills a reload per network blip); after an erase,
a JSON dump of the replica holds the key and digest and none of the erased text (kills text kept in a
revision cache); the resync shows one line and no spinner (kills per-row spinners). Depends on 55.
Design: 05 §5.3 (*loading only what the caller may read*), §3.6 (erase), §3.1 (three homes);
hub-contract §7.11 and §8. Parity: closes L-229's W1 part with task 55. Does not claim: the Hub-side
filter for Work kinds (task 13; 05 §5.3 still marks that filter **UNVERIFIED** for Work), erase's
admin surface (task 14's route has no desktop UI here).

**58 — the replica, part 4: `/` search and `O I` open by number.** Files: new
`src/work/replica/search.ts` (+ test), new `src/components/work/{SearchField,OpenByNumber}.tsx`
(+ tests; not yet in the File map), and one registration through task 53's `registerVerb`. Adds
`search(q, opts?: {projectId?; limit?}) → SearchHit[]`, `interface SearchHit {issueId; alias; title;
matched: "alias" | "title" | "description"; rev}` (the last of SW.16's four signatures) and
`resolveAlias(projectKey, n) → IssueId | null`, which looks the number up in the replica's
`(projectId, number)` index and **never parses the alias into a key** (05 §3.6). The index is derived
from replica records and updated on each change, so an erased record, which carries no text, never
matches. Matching: NFKD, diacritics stripped, lower case, token prefixes; an exact alias first, then
title hits above description hits. `/` in list focus opens `SearchField` over the current view (view
search and jump, L-106); inside the editor `/` stays task 50's slash menu. `O` then `I` (task 54's
sequence) calls the verb `issue.openByNumber`, registered here: a mono number prompt; `Enter` opens
the issue view at the head; an unknown number says *No EP-999 in EP* and goes nowhere. While the
replica is still `"loading"` after a restart, `search` asks the Hub's title search (task 14) once
through the relay and labels those results *from the Hub · titles only*. `budget.palette ≤ 50 ms` is
timed over a 10,000-issue fixture and **recorded, not asserted** (05 §4.6). Tests first: `resume`
ranks EP-142's title hit above a description-only hit (kills unranked results); `EP-142` and `142`
both put EP-142 first (kills alias-as-plain-text); `resumé` finds `resume` (kills a naive
`includes`); an erased issue's old title finds nothing (kills an index that keeps text); `/` in a list
opens search and `/` in the editor does not (kills a global `/`); `O I 142 Enter` opens EP-142 and `O
I 999` navigates nowhere (kills opening a wrong issue); after the fixture's project key changes to
`UP`, `EP-142` does not resolve and `UP-142` does (kills parsing the alias); a search while loading
calls the Hub once and labels the results (kills empty results after a restart). Depends on 53, 55.
Design: 05 §4.3 (`/`, `O` then `I`), §4.6, §3.6; 02 C8 budgets (`02:280-286`). Parity: closes L-106's
W1 part. Does not claim: `@person`, `#label` and `EP-` filter prefixes inside search (L-107, W2),
natural-language filters (L-100, W4), a moved issue's old number redirecting (L-35, W2).

**59 — the palette: every work verb with its chord, the four prefixes, opens beside the row.**
Files: `src/components/palette/commands.ts` ⚑, `CommandPalette.tsx` ⚑ (+ their tests) and
`palette.css`. `buildCommands(state)` (`commands.ts:72`) gains the work verbs **generated from task
53's `KEYMAP`**, one source, each `Command.hint` (`:17-18`) set from task 54's `chordFor(verb)`: every
W1 verb of 05 §4.3's table, *Go to Work*, *Plan* and *Desk*, *Apply held changes*, *Copy link* (task
47), *Subscribe* and *Unsubscribe* (no key, so no hint) and *Start* (`Shift+R`). Adds
`parsePrefix(query) → {scope: "all" | "issues" | "people" | "commands" | "sessions"; needle}`: `#`
issues through task 58's `search`, `@` people of the reader's teams from the replica, `>` commands
only, `~` the built session commands; inside a scope `filterCommands` (`:219`) keeps its label-only
rule, whose comment says why `meta` is not matched. Adds `openPalette({anchor: DOMRect; row:
RowContext})`: invoked from a row (`Ctrl+K` with a row focused, or a row's menu), the palette opens
**beside that row**, not centred, and lists that row's verbs first; from anywhere else it opens as
built, including from a terminal with `OD-9`'s toggle. Selection stays by id, never by index
(`CommandPalette.tsx:4`). `budget.palette ≤ 50 ms` is recorded, not asserted. Tests first: every work
command's hint equals `chordFor` for its verb, across all active `KEYMAP` rows (kills hand-written
hints that drift); `#resume` lists EP-142 and no commands (kills an ignored prefix); `>` lists only
commands, `@al` lists Alice and `~` lists sessions (kills a prefix mapped to the wrong scope); an
unprefixed query still matches labels only and the built palette tests stay green (kills widening to
`meta`); opened from a focused row, the palette sits beside it and its first rows are that row's
verbs (kills a centred palette in global order); *Subscribe* carries no hint (kills a `Shift+S`
hint); *Delegate* and *Ask* are absent in W1 (kills inert W3 verbs); a verb run from an anchored
palette acts on the anchored row even when the list cursor sits elsewhere (kills acting on the wrong
issue). Depends on 54, 58. Design: 05 §4.3 (*The palette*), §4.1 (the Palette row), §4.6. Parity:
closes L-221's W1 part and L-222, and L-226's palette half (task 54 did the menus); serves L-29
(`C` from the palette; create is task 68). Does not claim: an assistant in the palette (L-155, team
mode), W2's *Save as view* (L-95).

**60 — peek on `Space`: `peek.span`, `peek.hunk`, `peek.person`.** Files:
`src/components/work/Peek.tsx` (+ test), new `src/work/peek.ts` (+ test; not yet in the File map),
edits to task 40's `IssueRow.tsx`, task 43's `BoardView.tsx` and `work.css`. Adds `type PeekKind =
"issue" | "span" | "hunk" | "person"` and `peekFor(row) → PeekKind`: an issue row peeks the record
(head, `serves` with its criterion ids such as `c-91e2` in mono, since 05 §4.1 keeps machine ids in
peek and off the row); a row with the reader's own agent line peeks **`peek.span`**, the observed
facts behind that line (the session status change, the lane event, the files observed, the exit code,
each with its time and source, in mono) plus an *open session* link; a landed row peeks
**`peek.hunk`**, the land commit's hunks, each with its span where one exists (phase 3's land and diff
records; paths **UNVERIFIED until merge**, rule H2); a teammate's row peeks **`peek.person`**: that
person's issues in this project, their states and reasons, and files only where an observed fact
exists, **never their transcript or terminal** (`OD-15`). From a span or hunk peek, *Comment on this*
calls task 51's `addComment({anchor})`. Behaviour (02 C9, `02:307-312`): `Space` tap opens and closes,
hold peeks while held; `↑`/`↓` move the peek down the list and never scroll it; peek never marks
anything seen and closes on `Esc`; it paints from the replica first (`budget.peek ≤ 100 ms`, recorded)
and later facts stream in; it enters with `--dur-peek` on `opacity` and `transform` only. Tests first:
tap opens, a second tap closes, and hold-then-release closes (kills tap-only peek); `↓` with peek open
moves it to the next row and the list's `scrollTop` is unchanged (kills moving the list); opening peek
leaves the read cursor where it was (kills marking seen); `Esc` closes; `peek.person` makes no attach
or terminal call and renders no transcript line (kills showing a teammate's session); `Peek` imports
nothing from `src/api/attachRegistry.ts` (kills PTY-derived meaning, `OD-8`); `c-91e2` appears in
peek in mono and not on the row (kills machine ids on the row); `Space` in a text field types a space
(kills a `keys.list` leak); the peek's CSS animates only `opacity` and `transform` (kills layout
motion); `expectNoPrediction` passes. Depends on 40, 55. Design: 05 §4.3 (`Space`), §4.1 (*other
agents are one `Space` away*; `peek.person`), §4.6; 02 C9. Parity: closes L-108's W1 part. Does not
claim: 02 C9's twelve transcript lines around a span, which need a harness transcript read through
`OD-8`'s pinned adapter (L-185, W3; no W1 source satisfies `OD-8`); leases in `peek.person` (W3);
create an issue from a span or hunk (task 68).

