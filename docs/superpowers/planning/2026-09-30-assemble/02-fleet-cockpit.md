# 02 — The agent-fleet cockpit and git lanes: adopt, integrate, buy or keep

**Date:** 2026-09-30. **Status:** in progress (skeleton; sections fill in as they are verified).
**Question:** which existing product can stand in for the multi-agent cockpit (parallel sessions,
one worktree each, live terminals, fork and resume, an attention queue) and for landing the work
back (review, merge queue, PR), instead of us building it?
**Owner direction (2026-09-30):** "focus on using third party solutions ... I need this tool ASAP,
not in weeks."

---

## 0. The answer

_To be written once the candidates are verified._

## 1. What we already have (phase 2, built)

Read from the tree on `feat/phase-2-cockpit` (549 commits), not from the plan's promises.

| Piece | What is real today | Where |
|---|---|---|
| Sandboxed sessions | Unmodified agent CLIs run in Docker sandboxes behind one `Backend` interface; a `katabe` implementation exists and is dormant until phase 8 | `control-plane/internal/backend/{dockerbe,katabe}` |
| Live terminals | `sandboxd` owns one PTY per session with a ring buffer; `GET /v1/sessions/{id}/attach` relays WebSocket frames; nothing reads PTY bytes for meaning (`OD-8`) | `sandboxd/`, `control-plane/internal/api/attach.go`, `internal/proxy` |
| Session lifecycle | Create, get, patch, delete, restart; detach does not destroy; restart recovery and a reaper | `internal/api/sessions.go`, `internal/reaper` |
| Resume and fork | A per-worktree transcript store outlives the sandbox, so `claude --resume` works in a fresh sandbox; fork = `--resume A --fork-session` into another worktree with a read-only staged copy of the source transcript | `sessions.go`, `sessions_fork.go` |
| Worktrees | Read-only enumeration of projects, branches and worktrees; many sessions per worktree. **Creating a worktree as a real clone is phase 3 task 03–05, not built** | `internal/api/catalog.go`, `internal/worktree` |
| Cockpit | Tauri 2 + React 19 window: rail, sub-rail, Columns and Canvas layouts, session panes, split-button create, command palette, keyboard map, one heartbeat clock | `desktop/src/components/{develop,rail,subrail,palette}` |
| Status | Four observed facts only: `starting`, `running`, `idle`, `exited`. **`BLOCKED-ON-YOU` is not built** — phase 2 refused to invent it from screen output | phase 2 plan, refinement 3 |
| Landing | **Nothing.** `MergeLens.tsx` and `FilesLens.tsx` are 15-line placeholders. Phase 3 (git lanes, 24 tasks, ~72 h in its own table) is 2 of 24 built | `desktop/src/components/lens/` |

**So the honest gap is not the terminal cockpit, which is built. It is three things:** (a) the
attention queue (which session needs me now), (b) turning each session's worktree into a real,
isolated clone whose work is fetched back, and (c) landing: diff review, a merge queue, push and a
pull request on Azure DevOps or GitHub. Team mode (69 tasks, plan only) sits on top of all three.

## 2. The need, as testable capabilities

Every candidate in §3 is scored against these, in this order. C1–C4 are phase 2 (built); C5–C9
are the gap.

| # | Capability | Hard rule behind it |
|---|---|---|
| C1 | Many agent sessions in parallel, one worktree each, unmodified CLIs (Claude Code first) | `CLAUDE.md`, `OD-8` |
| C2 | Each session in an **isolated sandbox** (untrusted agent), later a Kata microVM on AKS | `CLAUDE.md` isolation lock |
| C3 | Live terminal, attach from anywhere, detach does not kill | phase 2 |
| C4 | Fork and resume a conversation, including into a new worktree | phase 2, `OD-10` |
| C5 | **Attention queue**: the human sees which sessions are waiting on them, without screen-scraping | `docs/AGENTIC-UX.md` §3–4, `OD-8` |
| C6 | Worktree = local clone, work fetched back under `refs/panacea/lanes/…`; sandbox never holds a git credential | `OD-2`, phase 3 decisions 9–10 |
| C7 | Review the diff, land in an order the human sets, with undo; conflicts shown, not dumped on the user | phase 3 tasks 07–11, 16, 19–21 |
| C8 | Push and open a PR on **Azure DevOps and GitHub**, with the brief key carried as a trailer (provenance) | `OD-4`, `CLAUDE.md` provenance spine |
| C9 | Runs on Azure in our environment, later the customer's cloud; Entra via Keycloak; AI via the LiteLLM gateway; AU residency possible | `OD-5`, `OD-18`, `OD-19`, `OD-30` |

## 3. Candidates

_One row per serious candidate: what it does, licence, deploy, enterprise fit, maturity, gaps,
integration days, verdict._

## 4. Landing the work: pull requests and merge queues

_GitHub merge queue, Azure DevOps PRs and auto-complete, and what phase 3 still owns._

## 5. Recommendation

## 6. What we still build (the glue)

## 7. Calls only the owner can make

## 8. What this study could not verify

## Sources
