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

