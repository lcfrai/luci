# Phase W task scopes: tasks 61 to 80

Scope paragraphs for tasks 61-80 of `2026-09-25-phase-w-work-layer.md` (W1: Start on the desktop,
the Desk frame and its rows, the accessibility floor, issue chips, create and import, the
local-mode demo; W1b: *shipped*; W2: the planning wire, the fake, the migration and the Hub's
planning aggregates). A part writer reads one paragraph and the skeleton's row; where they
disagree, **the row's id, dependsOn and seams win** (skeleton, *Task scopes*).

**Conventions used below.**

- Paths are relative to the repository root; desktop paths are under `desktop/`, Hub paths under
  `control-plane/internal/hub/`. A path marked ⚑ is shared (skeleton *File map*): the tasks that
  touch it run in task order, never in parallel worktrees.
- "05 §x" is `docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (revision 2);
  "L-n" is row n of the parity matrix (`01-linear-feature-inventory.md`, priced in 05 §1). A row a
  task **closes** is built there; a row it only **serves** is closed by the task named.
- Seams are the skeleton's `SW.n` (*Seams* table). Every desktop surface test calls
  `expectNoPrediction(container)` (SW.8, task 20) and reads Work data through the replica
  (`useWork`, `mutate`, `onRefused`, SW.16) or, before task 55 lands, `workFixtures` (SW.8).
- "Mutant" names the one wrong implementation a test exists to kill.
- Hub server paths (`server/`, `store/`, `migrate/`) follow the Hub plan's file map and are not
  on `feat/hub` yet (skeleton *File map*, rule H1): a part cites them as planned. A sibling path
  named only by its owner's plan is **UNVERIFIED until merge** (rule H2).
- Desktop tasks follow the skeleton's *Global constraints*: list-focus keys only, one heartbeat,
  `tabular-nums`, `transform`/`opacity` motion, facts in Commit Mono at `--t-fact` (`OD-28`), no
  Hub data on disk, and no predicted number anywhere (`OD-27`). Tests run with
  `npm --prefix desktop test` and `npm --prefix desktop run typecheck`; Go with
  `go test ./control-plane/...` (Hub integration tests with `-tags integration`).

