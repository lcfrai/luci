# Phase W scopes: tasks 01 to 20 (W0, W1's Hub, the phase 4 deltas, the test kit)

One paragraph per task. A part writer reads its paragraph and the skeleton's row
(`2026-09-25-phase-w-work-layer.md`, *Task list*); where the two disagree, **the row's id,
dependsOn and seams win** and this paragraph is corrected. `§` means a section of
`docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (revision 2); `L-NN` is a row of
`01-linear-feature-inventory.md`, as 05 §1 prices it. A row listed as *closed* here is closed only
when the task's acceptance commands pass; a row closed jointly names the other tasks.

Paths are repository-relative. Hub server paths (`server/`, `store/`, `migrate/` under
`control-plane/internal/hub/`) are planned by the Hub plan and **UNVERIFIED until they land on
`feat/hub`**; sibling-plan paths are **UNVERIFIED until merge** (rules H1, H2). Every Go test runs
as `go test ./control-plane/... -race -count=1`, Hub integration rows with `-tags integration`
against `control-plane/cmd/hub/compose.yaml`; every desktop test as `npm --prefix desktop test`.

---

