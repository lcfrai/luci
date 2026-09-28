# 11a — Fact sheet: phase 4 (Discover) as built

**What this is.** Facts about phase 4 as it stands in the phase-4 worktree today, for a designer.
It designs nothing and recommends nothing. Every fact carries a `file:line` in
`.claude/worktrees/phase-4/` (branch `feat/phase-4-discover`), abbreviated below as `p4/`.
Written 2026-09-29.

**Tree read.** `p4/` at `ca737d92` (*docs(plan): task 09 fix round 1, the part follows the code*).

## What could not be verified

- **No test was run for this sheet.** "Approved", "green" and every test count are what the review
  files, the parts and the ledger say, not a run made here.
- **The ledger is not a committed record.** `p4/.superpowers/sdd/p4/progress.md` is local and
  git-ignored (`:5-6`); it was read by grep only, as instructed.
- **Task 09's mutation round** is "running" in the ledger's last line (`progress.md:562`); its
  result lives in a scratchpad outside the tree and was not read. No re-review of 09 exists.
- _(more added per section)_

---

## 1. The plan and what is built

**The plan.** `p4/docs/superpowers/plans/2026-09-23-phase-4-discover.md` (600 lines) plus 23 parts,
`p4/docs/superpowers/plans/2026-09-23-phase-4-discover-parts/task-01.md` … `task-23.md`. Its goal
(`:46-62`): five questions answered one at a time become the brief, stored in the Hub; when a
session starts, the control plane writes the brief's latest complete revision into the worktree's
`CLAUDE.md` before the agent's process starts and stamps the session with the key of what it wrote;
that key, `brief:bf-…@n`, is the start of the thread; the model "helps and never gates".

**The task table** (`:390-414`). Hours are the plan's estimates, not time spent. State is from the
review files in `p4/.superpowers/sdd/p4/` (their last `Task quality:` line) and the git log.

| # | Task (plan wording, shortened) | Depends | Est. | State today |
|---|---|---|---|---|
| 01 | Lift `writeAtomic` into `statepath` | — | 1 h | Approved (`task-t01-review.md`) |
| 02 | `modelclient` | — | 1.5 h | Approved (`task-t02-review-1.md`) |
| 03 | `discover`: content model and ops (`4-D2`, R6 ids, per-question skip) | — | 3 h | Approved (`task-t03-review-1.md`) |
| 04 | `discover`: the model follow-up, degrading to nothing | 02, 03 | 1.5 h | Approved (`task-t04-review-1.md`) |
| 05 | `discover.Resolve` (`4-D3`); `claudemd` Render, Merge, Strip, Inspect | 03 | 2.5 h | Approved (`task-t05-review-2.md`) |
| 06 | `provenance`: key grammar, `Ref`, `Build`, criterion digests | 03, 05 | 2.5 h | Approved (`task-t06-review-2.md`) |
| 07 | `discoverhub`: binding and reads | 03, 05, 06, Hub H0; HA4-2 for the real Hub | 3 h | Approved (`task-t07-review-1.md`) |
| 08 | `discoverhub`: writes, `4-D3` fan-out, archive, rebind | 07; HA4-2, HA4-9 | 3 h | Approved (`task-t08-review.md`) |
| 09 | `contextwrite`: the Materialiser | 01, 05, 07, 10 | 3 h | **In fix** (see below) |
| 10 | `api`: the `Materialiser` seam, `brief_not_written`, `RouteMounter`, interim | — | 3.5 h | Approved (`task-t10-review.md`) |
| 11 | `discoverapi`: brief routes, relay rule, route audit | 07, 08, 10 | 3 h | Unbuilt |
| 12 | `discoverapi`: refs, branch provenance, materialisations | 09, 11 | 2 h | Unbuilt |
| 13 | `repoanalysis` and the drafted brief | 02, 03, 11 | 2.5 h | Unbuilt |
| 14 | Wiring and configuration in `cmd/api` | 09–13; Hub C2; phase 3 task 13 | 1.5 h | Unbuilt |
| 15 | Desktop: Discover client, wire types, problem codes | 11, 12 | 2 h | Unbuilt |
| 16 | Desktop: `discoverSlice`, `provenanceSlice`, the one loader | 15 | 3 h | Unbuilt |
| 17 | Desktop: `ContextChip`, `DefinitionOfDone`, `ShippedManifest` | 16 | 3 h | Unbuilt |
| 18 | Desktop: the Discover lens — binding, asking, the brief born, follow-up | 16, 17 | 3 h | Unbuilt |
| 19 | Desktop: moved on, project changed, undo, past revision, missing, erased, hand-edited | 18 | 3 h | Unbuilt |
| 20 | Desktop: `4-D1` on every session start (six paths) | 15, 16 | 4 h | Unbuilt |
| 21 | Desktop: import a repository — drafted from the code | 13, 18 | 2 h | Unbuilt |
| 22 | Contract amendments (Go, UI, provenance) | 10, 12, 16 | 1.5 h | Unbuilt |
| 23 | End-to-end verification; the owner's batched steps | all | 2 h | Unbuilt |

**Counts.** 9 of 23 approved (01–08, 10): 21.5 of the plan's 57 estimated hours. One in fix (09,
3 h). Thirteen unbuilt (11–23, 32.5 h), every desktop task among them. The ledger says the same:
"9 of 23 tasks APPROVED" (`p4/.superpowers/sdd/p4/progress.md:550`) and the remaining order is 09's
fix and re-review, then 11 to 23 (`:554`).

**Task 09 in detail.** Built and committed (`5fb80e09` code and tests, ledger `:540`); its review
`task-t09-review.md` ends *Needs fixes*, 0 Critical / 2 Important / 8 Minor, both Importants
security (host git ran a sandbox-written `core.fsmonitor`; the Materialiser followed planted
symlinks) (ledger `:551`). Fix round 1 is committed as `6a354f69` (code) and `ca737d92` (part 09)
(`:560`), green on `contextwrite` and `statepath` (`:559`). The ledger's last line says a mutation
round over 18 mutants was **running** at `ca737d92` (`:562`). No `task-t09-review-1.md` exists.

**What "unbuilt" looks like on disk.** No `discoverapi` or `repoanalysis` package exists
(`ls p4/control-plane/internal/`). The only Discover file in the desktop is phase 2's placeholder,
`p4/desktop/src/components/lens/DiscoverLens.tsx:8-16`: it renders `LensPlaceholder` with the line
*"Turn a conversation into a spec this branch can be measured against."* (`:13`) — note it says
"spec", a word `CONTEXT.md` rules out. The binary still runs `materialiserUntilTask14`
(`p4/control-plane/cmd/api/materialiser_interim.go:18`), task 10's stand-in until task 14 wires
the real one.

**Order and the UI gate.** Three lanes: pure Go (01→06), seams (10 alone), desktop (15→21)
(`plan:416-424`). "UI branches (17–21) stay unmerged until the owner has looked" (`OD-1`,
`plan:427-428`). Critical path: Hub H0 → 07 → 08 → 09 → 11 → 12 → 14 → 23 (`plan:430`).

## 2. The brief content model (task 03)

_(pending)_

## 3. The provenance key grammar and `Ref` (task 06)

_(pending)_

## 4. `claudemd` (task 05) and the Materialiser (task 09)

_(pending)_

## 5. The interview as shipped

_(pending)_

## 6. What would have to change

_(pending)_

---

## Closing

_(pending)_
