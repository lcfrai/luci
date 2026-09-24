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
