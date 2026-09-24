# Phase W: The Work Layer (Linear parity-plus)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Each task's full text is in `2026-09-25-phase-w-work-layer-parts/task-NN.md`; this file is the skeleton those parts are written from.

**Goal:** A team moves its backlog into PanaceaApp and never goes back. Issues are **Hub objects on
the provenance thread**: each one points at the brief criteria it serves, so the chain reads
brief → criterion → design frame → issue → session → land → evidence → release, in one place.
Everything Linear does that we price as MATCH, EXCEED or PARTIAL is built (W0 to W5, `OD-26`), in
the design-language uplift (`02`, `05` §4.4) with Commit Mono and Segoe UI Variable (`OD-28`). An
agent takes any row with `Shift+R`, shows a first sign of work within 10 seconds, and the issue
turns `done` by itself only when the platform **observed** its own evidence at its own landed
commit. A hand-set done reads *done, not proven*, and sync can never write it. Progress is proof
per criterion and plain counts per group; no percentage, bar, ETA or "N of M" anywhere (`OD-27`).
From W3, per-user Orchestrators negotiate on issues across owners, and nobody steers another
person's agent (`OD-14` to `OD-17`).

**Delivery shape.** Phase W runs **alongside** team mode on its own streams (`OD-25`). W1 is the
first adoptable slice and its demo runs in **local mode** first (`OD-20`), then moves to the cloud
when team mode's gateway stream (phase 6 stream A) and environment stream (phase 8 stream C) land.
Team mode's critical path is untouched: no Phase W task edits a team-mode task, and Phase W's Hub
deltas are new Hub tasks numbered after Hub task 43, with the same sync-merge rule team mode uses
(Global constraints, rule H1).

**The demo W1 must pass (05 §5.3).** Import a Linear backlog or a CSV; it lands in Triage as *not
tied · imported*. Press `T` on EP-142 and attach the criterion it serves from a text-match
suggestion. Press `Shift+R`; within 10 seconds a local-mode agent is working and its one live
line is on the row. Teammates comment. It lands; the row flashes `landed` in place and does not
move. The platform watched the resume test pass at the landed commit, so EP-142 turns `done` with
no click. The last frame is the Desk: *"Nothing needs you. EP-142 landed and proven at 14:02."*
W1b adds *"EP-142 shipped to production at 14:02."*

**Tech Stack:** Go 1.23 (the Hub service in the `feat/hub` contract; `control-plane` and the
phase 3 and phase 4 packages), React 19 + TypeScript 6 + Vite 8, `zustand` 5, Vitest 5 +
`@testing-library/react` 16 + jsdom 30, Tauri 2. No new runtime dependency without a licence line
in the task (MIT, Apache-2.0, BSD, ISC or OFL only; `OD-11`).

**Spec:** `docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (revision 2) and
`00-summary.md`; the parity matrix `01-linear-feature-inventory.md` (257 items); the design craft
`02-linear-design-language.md`; the critiques `06` and `07`; `docs/DESIGN-SYSTEM.md`;
`docs/AGENTIC-UX.md` with the four `Q13` amendments (task 08); `prototype/work-layer.html`.

---

## Architecture

No new service. The work layer is **a new Hub aggregate** (`Work`), **a relay and a binding** in
the local control plane, **small deltas to phases 3 and 4** that put the issue on the thread, and
**a new cockpit lens plus the Desk frame** on the desktop. Fleet (Phase T) joins in W3 only.

```
desktop (the portal)                 control plane (loopback, per user)       the Hub (Phase H + Work)
┌───────────────────────────┐ REST  ┌───────────────────────────────────┐ REST ┌────────────────────────┐
│ Work lens · Plan · issue  │ ────► │ /v1/hub/work/* relay (contract    │ ───► │ issues is-, revisions  │
│ view · Desk frame         │       │   §4.3 relay rule, unchanged)     │      │ comments ic-, features │
│ replica: in memory only,  │ ◄──── │ Panacea-Hub-Cursor on /v1/sessions│ ◄─── │ cycles, views, … lp-   │
│   cursor on /v1/changes   │       │ session ⇄ issue binding (IssueRef)│      │ one event per write    │
│ optimistic write + Undo   │       │ Materialiser: brief key + issue   │      │ authz rows, erase      │
└───────────────────────────┘       │ phase 3: Panacea-Issue trailer,   │      │ janitor: cycles, loops │
                                    │   land → landed, done evaluator   │      └───────────┬────────────┘
                                    └───────────────┬───────────────────┘                  │ W3: Fleet reads
                                                    │ local mode (OD-20) now;              ▼ issues, writes
                                                    ▼ cloud when phase 6 A + phase 8 C land   notes as owner
                                              sandbox · agent CLI (never parsed, OD-8)
```

Seven rules shape every task. A task that breaks one is wrong however good it is alone.

1. **The issue is on the thread.** An issue head `is-` is mutable; an issue revision
   `issue:is-…@r` is immutable and digested, and its `serves [{briefKey, criterionId}]` is what
   joins it to Define. `EP-142` is a display alias from `project.key` and a number, **never a
   key**, because an admin can change the project key (05 §3.6).
2. **Done means proven.** `started`, `in_review`, `landed` and `done` are **facts**, written as the
   person whose act caused them. `done` needs the issue's **own** evidence, at or after its **own**
   land commit, for each criterion it serves; observed evidence (a watched test run's exit code at
   the landed commit) closes it with no click. A hand-set `done` needs a one-line reason and reads
   *done, not proven*. **No sync or integration token may write a fact state or `done`**, and an
   authorisation row plus a test pins it (05 §3.3, §3.6).
3. **Three homes, never mixed** (05 §3.1). A decision or accountability fact is a Hub record; a fact
   about now (who is on an issue, which files, the live line) is Fleet or control-plane state,
   derived and never stored on the issue; a preference is the desktop's. **The desktop keeps no Hub
   data on disk** (`ADR-H1:467-474`), so the replica is in memory (Q5).
4. **Rows never move under the reader.** Arrivals and moves are held (`arrivals.hold` on `N`,
   `moves.hold`), a landed row flashes in place, and a row carries **one** agent line, the reader's
   own; a teammate shows as a person and the files they touch, never as dots (05 §4.4).
5. **Proof per criterion, no predictions** (`OD-27`). Surfaces show which criteria are proven and
   plain counts per group. No percentage, progress bar, ETA or "N of M" completion figure appears
   anywhere; every surface task's tests call the shared `expectNoPrediction` helper (task 20).
6. **Agent CLIs are never modified, wrapped or parsed** (`OD-8`). The live line and every status
   come from mechanically observed facts (session status, lane events, observed files, exit codes),
   never from PTY bytes. Issue text enters an agent's managed block only as spec-class text its
   owner adopted (Start or Delegate), after the Hub's schema and character checks; comments and
   requests are never rendered into it (05 §3.4, `Q3`).
7. **Nobody steers another person's agent** (`OD-16`). An edit to someone's in-flight issue reaches
   its owner as a Desk *inform*, never their agents; a reassign is a *decide* for the owner; a
   cross-owner relation is a proposal from the editor's own Orchestrator (W3) (05 §3.7).

### Streams, and where each slice can start

Phase W runs on its own streams beside Phase T (`OD-25`, Q11 option B). The Hub stream reviews with
team mode's stream B; the desktop streams sit beside its stream F and **share one Desk frame**
(05 §5.6): team mode adds rows to the frame task 62 builds, never a second Desk.

| Stream | Tasks | Starts after | Lands in |
|---|---|---|---|
| **WH, Hub deltas** | 02-17 (W1), then each slice's Hub tasks | Hub task 43 merged into `feat/hub`, synced here (rule H1) | `feat/work-layer`, merged into `feat/hub` by the orchestrator |
| **WP, phases 3 and 4** | 18-19, 21-25 | phase 4 tasks 06, 09-10, 16-17; phase 3 tasks 05, 07-09 | `feat/work-layer`, each file's owner phase wins (rule H2) |
| **WD1, look and Work lens** | 20, 26-45 | task 01; the look needs no Hub | `feat/work-layer` |
| **WD2, issue view, keys, replica, Desk** | 46-70 | tasks 16 and 20 (the fake is enough to start) | `feat/work-layer` |
| **W2-W5** | 72 onward | the slice before, per the table | `feat/work-layer` |

---

## The decisions this plan rests on

Owner decisions are in `docs/superpowers/planning/2026-09-23-owner-decisions.md`; the newer win.

| Id | What it fixes for Phase W | Where it bites |
|---|---|---|
| `OD-25` | Built **alongside** team mode on its own streams; W1 demos in local mode first; team mode's critical path is untouched | the streams table; no task edits a Phase T task |
| `OD-26` | **Everything, W1 to W5**: every MATCH, EXCEED and PARTIAL item of the 257 is built; the 27 SKIPs stand (Q6) | the coverage map; the SKIP list |
| `OD-27` | **Proof per criterion and plain counts; no percentage, bar, ETA or "N of M"**; `AGENTIC-UX.md` gets Q13's four one-line amendments | rule 5; task 20's `expectNoPrediction`; task 08 writes the amendments |
| `OD-28` | **Commit Mono** (OFL) for every id, path and time; **Segoe UI Variable** for the sans | tasks 27-28 (faces bundled as files, licences recorded) |
| `OD-1` | The review gate is waived for phases 3, 4, 5 and 7 **only**; Phase W keeps it: the in-app prototype (task 38) is shown before the Work surfaces are sized, and `feat/work-layer` is not merged onward until the owner has looked. Q3, Q4, Q5, Q7, Q8, Q9, Q11 and Q12 take their recommended answers under it | tasks 38-39; merge policy |
| `OD-3`, `OD-5` | Shared state lives in **the Hub**, our own server; issues are Hub records from day one | stream WH |
| `OD-6` | The desktop signs in with Entra PKCE; every Hub write is as a person | authorisation rows, tasks 06-07 |
| `OD-2`, `OD-4` | Worktrees are local clones; **Azure DevOps and GitHub both**, so outside-PR linking, webhooks and sync cover both hosts | tasks in W5 |
| `OD-8` | Agent CLIs are never parsed; a harness transcript may be read only through a pinned adapter, and nothing here needs one | rule 6 |
| `OD-11` | Clean room: no Linear source, CSS, icon or copy is copied; our glyphs are drawn; every dependency is MIT, Apache-2.0, BSD, ISC or OFL | tasks 28-29 licence checks |
| `OD-13` | No Panacea source is copied; phase 7 stays behind its interface, so W1b and W4's release rows read the manifest through phase 7 | W1b, W4 |
| `OD-14`-`OD-17` | Team mode: per-user Orchestrators are the only cross-user channel; **nobody steers another's agent** | rule 7; W3 |
| `OD-19`, `OD-21` | Inference only through the LiteLLM gateway, Global or AU; W4's AI assists need phase 6 | W4 assists gate on phase 6 |
| `OD-20` | Local mode stays; W1 runs there first | task 70's demo |
| `OD-24` | A project's data classification is set at creation; AU-only projects route AU and branch names carry no title slug (Q8) | tasks 21, 24 |

The design's own questions, as answered: **Q1** → `OD-27`; **Q2** → `OD-26`; **Q3** issue text
enters the agent's context with guards (rule 6); **Q4** the Desk frame is in W1 (task 62);
**Q5** the replica is in memory (tasks 55-58); **Q6** the skips stand; **Q7** Linear's project is
our *feature*; **Q8** no title slug in AU-only branch names; **Q9** build our own, sync one way
in (W5); **Q10** → `OD-28`; **Q11** option B (`OD-25`); **Q12** cross-team in W4; **Q13** the
four amendments (task 08); **Q14** Segoe UI Variable (`OD-28`).

---

## Global constraints

**Merging and sibling files**

- **H1, the sync-merge rule for Hub deltas** (the rule team mode's Hub deltas follow; Phase T's
  skeleton at `afc5b5d` does not yet spell it out, so its wording there is **UNVERIFIED**). The
  Hub route table says *"Nothing else is routed"* (`hub-contract.md:609`), so the work layer arrives
  as **new Hub tasks after Hub task 43, never edits to a Hub task in flight** (05 §5.5). Before a
  WH task starts, the orchestrator **sync-merges** the current `feat/hub` into `feat/work-layer`;
  the implementer never merges. A WH task is **additive only**: new files, a new `Work` interface
  embedded in `hubclient.Hub` (`hub-contract.md:1329`), new rows appended to the route table and
  the authorisation matrix, new wire types, and migrations taking the **next free number at sync
  time** (renumbered on a clash; they are append-only and applied nowhere shared until merged).
  On an add/add or edit conflict **the Hub plan's file wins whole**; this plan's change is re-applied
  on top of it and its tests re-run against it, and a failure is a finding against our change.
  The orchestrator merges finished WH tasks into `feat/hub` in task order.
- **H2, deltas to phases 3 and 4** (tasks 18-19, 21-25). Each extends a file its owner phase
  builds (`provenance/key.go`, `Session`, the Materialiser, the commit hook, land, the evaluator).
  If the owner's file has not landed on the sync base, the task **waits** (its `dependsOn` names the
  owner task); it never writes a second copy. The owner phase wins every conflict.
- **W0's two Hub-contract edits go as a proposed edit** to the `feat/hub` owner, not a direct change
  (05 §5.2), because that contract is under live edit. Task 01 writes the proposal.
- **UI branches stay unmerged until the owner has looked** (`OD-1`, `OD-12`); visual acceptance is
  the owner's eyes, never a screenshot or a self-report (`CLAUDE.md`).

**Go**

- Module path `github.com/kpmg-au-emu/panacea-app/control-plane`; the `go` directive stays
  `go 1.23` in `go.work` and every `go.mod`. Check `git diff` on them before every commit.
- **`go test ./...` is invalid at the workspace root.** Run
  `go test ./control-plane/... -race -count=1`; the Hub's integration rows run with
  `-tags integration` against `control-plane/cmd/hub/compose.yaml`.
- The Hub's boundary test (`hub-contract.md:78-82`) holds: the control plane imports only
  `hubapi` and `hubclient`, never `internal/hub/server` or `internal/hub/store`.
- **Every Hub write appends one metadata-only event in the same transaction**
  (`hub-contract.md:1131`); every new route gets its authorisation-matrix rows in the same task;
  every new object's text gets the brief's erase (org admin only; keys, digests and revision
  numbers kept, resolving `erased`).
- New ids use `ids.Wide(prefix)` (`hub-contract.md:93-100`): `is-`, `ic-`, `ft-`, `ml-`, `cy-`,
  `in-`, `rq-`, `lb-`, `tp-`, `vw-`, `lp-`, `df-` (05 §3.6; checked against the Hub's and TM-05's
  prefixes). `nt-` is TM-05's and is reused for status updates.

**Desktop**

- Read `docs/DESIGN-SYSTEM.md`, `docs/AGENTIC-UX.md` and `prototype/work-layer.html` before any
  UI task. No indigo or violet, no gradient hero, no blanket glass; elevation is a 1px ring plus
  layered shadow; radius by element size; motion only on `transform`/`opacity`, never
  `transition: all`; **one heartbeat** (`desktop/src/hooks/heartbeat.ts`), never a spinner per row;
  live numbers `tabular-nums`.
- **Faces** (`OD-28`): Commit Mono for every id, key, path, time, cost and duration, at `--t-fact`
  12px (11px mono only for `kbd` hints); Segoe UI Variable for the sans. `--ink-3` (3.43:1,
  `tokens.css:47`) never carries a fact. `--mono` must stay a real monospace for xterm.js
  (`tokens.css:149-152`).
- **Keys** (05 §4.3): single keys and `G`/`O`/`M` sequences fire only with focus in a list, by
  extending `isCockpitChord` (`desktop/src/hooks/useKeyboardMap.ts`); agent keys test
  `event.code` (`KeyR` with `shiftKey`), never `event.key`; Linear's `Shift+D`, `Shift+E`,
  `Shift+P`, `Shift+M` and `Shift+S` do nothing; `OD-9`'s terminal toggle is respected.
- **No completion prediction** (`OD-27`): every surface task's tests call `expectNoPrediction`
  (task 20), which fails on a `%`, a `progressbar` role, an ETA or an "N of M" string.
- The desktop persists **no Hub data**: no `localStorage`, IndexedDB or file write of any Hub
  record; a test in task 55 greps for it.
- Tests: `npm --prefix desktop test` (`vitest run`) and `npm --prefix desktop run typecheck`.

**Everywhere**

- Clean room (`OD-11`): no Linear source, CSS, icon, screenshot or copy is copied; Linear facts come
  only from `01` and `02`. Every new dependency names its licence (MIT, Apache-2.0, BSD, ISC, OFL).
- No secrets in source; configuration is `PANACEA_APP_*`; the control plane stays on loopback.
- State what is unverified. Every size in this plan is an estimate; none is measured.
- Every shell command goes through `~/.claude/tools/claudezsh/claudezsh` (device rule).
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.

---

## File map

Every file the phase creates or modifies, with the task that owns it. Paths are relative to the
repository root. **Shared files**, touched by more than one task here or owned by a sibling plan,
are marked ⚑; the tasks that touch them run in task order, never in parallel worktrees. Hub server
files (`server/`, `store/`, `migrate/`) are **planned by the Hub plan and not yet on `feat/hub`**
at this writing (only `hubapi/` and `hubclient/` exist there); their paths follow the Hub plan's
file map (`2026-09-23-phase-h-hub.md:260-362`) and a WH task that finds them moved follows the
move (rule H1). Where a slice adds to a W1 file, the later task is listed in the same row.

### The Hub: `control-plane/internal/hub/...` (stream WH)

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `hubapi/keys.go` ⚑ | the `issue:` case in the key grammar the Hub mints and accepts (Hub task 01's file; W0's proposal) | modified | 02 |
| `hubapi/workids.go` | the Work id prefixes (`is-` … `df-`); `DisplayAlias(projectKey, n)`, which is never parsed as a key | new | 02 |
| `hubapi/workcanonical.go`, `hubapi/testdata/issue-revision/*.json` | the `issue-revision` canonical kind; golden vectors | new | 02 |
| `hubapi/work.go` | W1 wire types: issue head and revision, `serves`, states, thread state, relations, labels, policies, comments, subscriptions, imports | new | 03 |
| `hubapi/work_plan.go`, `work_roadmap.go`, `work_intake.go` | W2, W4 and W5 wire types | new | 72, 104, 133 |
| `hubclient/work.go` | the `Work` interface, one sub-interface per slice | new | 04; 72, 104, 133 |
| `hubclient/hubclient.go` ⚑ | one line: `Hub` embeds `Work` (`hub-contract.md:1329`) | modified | 04 |
| `hubclient/fake_work*.go` | the fake's Work rules, in memory | new | 04, 05; 73, 105, 134 |
| `hubclient/conformance/work*.go` | Work conformance rows, run against the fake and the real Hub | new | 05; 73, 105, 134 |
| `hubclient/methods_work*.go` | the Work methods over HTTP, with the digest re-check | new | 16; 80, 112, 141 |
| `migrate/sql/NNNN_work_*.sql` | one migration per slice, **next free number at sync time**: tables, RLS, grants, guards | new | 06, 07; 74, 106, 135 |
| `store/work_*.go`, `server/work_*.go` | the store and routes, one pair per aggregate | new | 08-15; W2-W5 Hub tasks |
| `server/matrix_integration_test.go` ⚑ | authorisation-matrix rows for every Work route | modified | 07 and every WH route task |
| `server/changes.go` ⚑ | the change feed carries Work kinds, filtered per caller | modified | 13 |
| `server/erase.go` ⚑ | erase for issues, comments, requests and notes | modified | 14; 136 |
| `server/janitor.go` ⚑ | cycle rollover and closure proposals (W2); recurring issues and reminders (W4); loop triggers (W5) | modified | 77; 110; 146 |
| `work_e2e_integration_test.go` | W1's Hub end to end through `devissuer` | new | 17 |

---
