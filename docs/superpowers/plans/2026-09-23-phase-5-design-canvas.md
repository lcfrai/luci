# Phase 5 — the Design canvas: implementation plan

**Status: DRAFT, written 2026-09-23 on `feat/phase-5-canvas` (based on `3303fd6`).** Built under
`OD-1` (the owner waived the prototype-before-plan gate for this phase); still adversarially
reviewed before any code, and every UI branch stays unmerged until the owner has looked at it.
**Nothing in this plan was compiled or run.**

**Primary input:** `docs/superpowers/planning/phase-5/paper-design-cleanroom-spec.md` ("the spec"
below; 861 lines, read in full). **Binding:** `OD-11`'s clean-room rules
(`docs/superpowers/planning/2026-09-23-owner-decisions.md:84-100`). The author of this plan did not
open, read, search or copy `~/Projects/Paperdesign`, `kgoedecke/doop` or any AGPL/GPL canvas
source, and no search in this plan's making touched `~/Projects/Paperdesign`.

**Builds against the Hub**, designed tonight in a sibling worktree and read there by absolute path,
never written to:
- `.claude/worktrees/hub/docs/superpowers/adr/ADR-H1-hub-architecture.md` ("ADR-H1"), 776 lines,
  read in full at commit `4eefb51`;
- `.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md` ("the Hub
  contract"), 2289 lines; the sections this plan builds on are cited by section and line.

_Sections follow as they are written._
