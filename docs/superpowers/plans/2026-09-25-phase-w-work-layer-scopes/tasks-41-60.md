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

