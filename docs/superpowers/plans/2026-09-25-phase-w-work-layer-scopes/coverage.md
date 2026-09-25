# Phase W parity coverage map: the 257 rows

The checklist `OD-26` asks for. Every MATCH, EXCEED and PARTIAL row of the parity matrix names
the Phase W tasks that close it, and every SKIP gives its reason. The *Gaps* list at the end
names every non-SKIP row that no Phase W task closes.

**Sources.** *Item* is the numbered master list, §21 of
`docs/superpowers/planning/2026-09-25-linear-parity/01-linear-feature-inventory.md` (items 1-257).
*Verdict*, *05 stop* and the SKIP reason come from the §1 table of `05-design.md` (revision 2).
Its recount is 139 MATCH, 67 EXCEED, 24 PARTIAL and 27 SKIP, and this map counts the same. Task
ids come from the ten scope files beside this one, read on `feat/work-layer` at `c5841bf`.

**Method.** The map is extracted from the scope paragraphs mechanically. It records only what
each paragraph says, with no judgement added.

- A task **closes** a row when its paragraph cites the row after *Closes* or *closes*.
- A task **serves** a row when it cites the row after *Serves*, *Carries* or *Parity rows
  served*. Tasks 01-20 use *served* for a joint close and name the co-closers, who appear in the
  first list.
- These citations are not counted: rows after *none closed* (a precondition), rows that appear
  only in a *does not claim* clause, and rows named only in passing.
- The demo-and-results tasks (70, 97, 113, 142, 174, 187) appear as **proof** and are never
  counted as closers, so a row that is proved but never built still shows as a gap.
- A SKIP row has `—` in its task column. Tasks 49, 55, 146, 165 and the paragraph citing L-86
  mention SKIP rows only to say they stay SKIP (L-128, L-230, L-135, L-232, L-86).

**05 stop legend.** `W1`-`W5` means built in Phase W. `exists` means already built before Phase
W. `TM`, `phase 3`, `phase 5`, `phase 6` and `phase 8` mean the row is owned by that plan.
`later` means 05's deferred list, pulled into Phase W where a task closes the row.

**Counts.** 203 of the 230 non-SKIP rows have at least one closing Phase W task. 6 are served
but never closed, and 21 have no Phase W task at all. All 27 of those gaps have a 05 stop
outside Phase W (`exists`, `TM`, `phase 3/5/6/8`, or `exists / later`), and no row whose stop
is `W1`-`W5` lacks a closer.

**What this map does not claim.** It does not claim that any closing task's acceptance passes,
because no task is built yet. It does not claim that a row cited in prose without one of the
verbs above was caught: such a citation is missed, which errs toward reporting a gap. It also
does not claim that 05's `exists` rows are built, because this map did not re-check them
against the desktop tree (UNVERIFIED).

| L | Item | Verdict | 05 stop | Tasks that close it | SKIP reason |
|---:|---|---|---|---|---|
| 1 | Workspaces; one person in several workspaces | MATCH | exists | **none** |  |
| 2 | Teams as the primary unit, each with its own workflow, cycles and labels | MATCH | W1 | 75 · serves 08, 11 · proof 70 |  |
| 3 | Team identifier in human-readable issue IDs, for example `ENG-123` | MATCH | W1 | 40, 46 · serves 02, 04, 08, 17 · proof 70 |  |
| 4 | Sub-teams with enforced and optional inheritance, up to 5 levels | SKIP | — | — | Hub v1 has no per-project grants (hub-contract:333); nesting needs a grant model first. Revisit with org-level initiatives |
| 5 | Private teams, invisible to non-members | MATCH | exists | **none** |  |
| 6 | Private issue sharing with a non-member | SKIP | — | — | Same reason as L-4: no per-object grants in v1. A guest role is the later answer |
| 7 | Copy team automations and templates to another team | MATCH | later | 175 |  |
| 8 | Six fixed status categories with custom named statuses per team | EXCEED | W1 / W2 | 22, 23, 28, 46, 75, 84 · serves 03, 04, 10, 17 · proof 70 |  |
| 9 | Default status for new issues | MATCH | W1 | 25, 68 · serves 03-05, 11, 17, 42 · proof 70 |  |
| 10 | Reserved Duplicate status | MATCH | W1 | 42, 48 · serves 03, 05, 10, 11 · proof 70 |  |
| 11 | Auto-close stale issues | EXCEED | W2 | 73, 77, 94 |  |
| 12 | Auto-archive closed issues, projects and cycles | MATCH | W2 | 77, 91 |  |
| 13 | Five-level priority, deliberately coarse | MATCH | W1 | 28, 46 · serves 03, 04, 08, 17 · proof 70 |  |
| 14 | Manual drag order shared by the whole workspace | MATCH | W2 | 79, 89 · serves 73 |  |
| 15 | Urgent priority notifies the assignee | EXCEED | W1 | 63 · proof 70 |  |
| 16 | Estimates: four scales, extended scale, explicit zero | MATCH | W2 | 79, 89 · serves 73 |  |
| 17 | Workspace and team labels | MATCH | W1 | 46, 78, 90 · serves 03, 05, 11, 17, 72 · proof 70 |  |
| 18 | Mutually exclusive label groups | MATCH | W2 | 78, 90 · serves 72, 73 |  |
| 19 | `Group/Label` shorthand creates both | MATCH | W2 | 78, 90 · serves 72 |  |
| 20 | Parent and sub-issues with property inheritance | EXCEED | W2 | 78, 88 · serves 73 |  |
| 21 | Parent and sub-issue auto-close automations | EXCEED | W2 | 78, 88 · serves 73 |  |
| 22 | Turn a comment or a checklist into sub-issues | MATCH | W2 | 78, 88 |  |
| 23 | Convert a large issue into a project | EXCEED | W2 | 76, 86 · serves 73 |  |
| 24 | Relations: blocked by, blocking, related, duplicate | EXCEED | W1 | 48, 63 · serves 03, 05, 11, 17 · proof 70 |  |
| 25 | Resolved blocker downgrades to Related automatically | MATCH | W2 | 48, 79, 89 · serves 73 |  |
| 26 | Issue templates, workspace and team scope | MATCH | W2 | 78, 89 |  |
| 27 | Form templates with typed, required fields | MATCH | W5 | 148, 170 · proof 174 |  |
| 28 | Default templates, different for members and non-members | MATCH | W5 | 147 · serves 148 · proof 174 |  |
| 29 | Create from anywhere: `C`, `Alt+C`, create-from-selection | EXCEED | W1 | 59, 68 · proof 70 |  |
| 30 | `linear.new` URL scheme with prefill parameters | MATCH | W2 | 93 |  |
| 31 | Email intake, per team and per template | MATCH | W5 | 149 · proof 174 |  |
| 32 | Recurring issues | MATCH | W4 | 114, 121, 134 · proof 142 |  |
| 33 | Drafts, local and persistent, kept six months | MATCH | W2 | 77, 80, 92 · serves 72, 73 |  |
| 34 | Description auto-save and description history with restore | EXCEED | W1 | 47, 49 · serves 02, 04, 09, 17 · proof 70 |  |
| 35 | Move issue between teams; old ID and URL redirect | MATCH | W2 | 79, 89 · serves 73 |  |
| 36 | Undo | MATCH | W1 | 56, 66 · serves 61 · proof 70 |  |
| 37 | Due dates with colour-coded urgency and due-soon alerts | MATCH | W2 | 79, 89, 94 · serves 81 |  |
| 38 | Exactly one human assignee; `I` assigns to me | MATCH | W1 | 46, 53 · serves 03, 04, 08, 17 · proof 70 |  |
