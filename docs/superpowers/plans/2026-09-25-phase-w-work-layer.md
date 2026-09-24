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
