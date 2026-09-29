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

Verdicts: **adopt** = take whole · **integrate** = put it beside ours behind a seam · **buy** =
commercial product · **reference** = read the UX or design, take no dependency · **reject**.
"Days" is the integration effort to make it do C1–C9 for us, not the vendor's install time.
Maturity figures are GitHub API values read 2026-09-30 unless marked.

### 3a. Desktop "agent development environments" (the class phase 2 already is)

These all solve C1, C3 and C4 on one person's machine. None solves C2 (a sandbox per untrusted
session), C9 (Entra, gateway, our cloud) or team mode, and none lands work into Azure DevOps.

| Candidate | What it does | Licence | Deploy | Enterprise fit | Maturity | Gaps against us | Days | Verdict |
|---|---|---|---|---|---|---|---|---|
| **Orca** `stablyai/orca` | "The ADE for working with a fleet of parallel agents": a worktree per agent, WebGL terminals whose scrollback survives restarts, inline diff comments sent back to the agent, GitHub and Linear in-app, SSH remote worktrees, an always-on `orca serve` host, web and mobile clients that pair to it, per-worktree cloud VMs via user "recipes" | **MIT** | Electron desktop (macOS, Windows, Linux); `orca serve` on a headless Linux box needs Xvfb; clients pair by URL (docs name Tailscale); "Orca does **not** sell managed VPS hosting" | Weak today: no documented SSO or RBAC; the self-host issue asking for team auth and a standalone web client (#10687, opened 2026-07-26) is open with no maintainer reply; enterprise page is "talk to us" | **81,599★**, created 2026-03-17, v1.4.217 on 2026-09-29 (daily releases), Stably AI, YC | Single-user; worktrees share one host, no sandbox per session; no Azure DevOps; no gateway or Entra; release notes and docs do not show a hooks-based status signal (unverified either way, see §8) | 12–18 to fork, rebrand and re-seat on our control plane; we would lose the Define/Design/Deploy lenses already in our shell | **reference** |
| **Emdash** `generalaction/emdash` | Parallel agents, a worktree each; issues from Linear, GitHub, Jira, GitLab and others; diff review, PR create and merge; SSH/SFTP remote projects. Status via the harness's **own lifecycle hooks** ("installs marker-tagged entries") — the `OD-8`-compatible way | **Apache-2.0** | Electron desktop, all three OSes | None stated (no SSO, team or audit) | 5,881★, pushed 2026-09-29, YC W26 | Same class gaps as Orca; Azure DevOps not listed | — | **reference** — the best worked example of hooks-driven status for our attention queue |
| **Sculptor** `imbue-ai/sculptor` | Desktop app: isolated workspaces, Claude Code and Pi built in, review and merge back, GitHub PRs; an **experimental** Docker/remote container backend | **MIT** | Desktop, macOS (Apple Silicon) and Linux only | None | 233★, pushed 2026-09-29; "experimental research preview", Imbue | No Windows build (our users are on Windows); containers experimental | — | **reference** |
| **Conductor** (Melty Labs) | Mac app running Claude Code, Codex and others in worktrees, now with "Conductor Cloud" sandboxes and multiplayer workspaces | **Closed**, commercial | macOS desktop; cloud sandboxes are the vendor's (region not published) | Pro US$50/month, Teams US$60/user/month, Enterprise custom (third-party review sites, 2026; not verified on the vendor site) | Series A US$22M, March 2026 | **Mac-only**: KPMG's fleet is Windows. Vendor cloud, not ours; no Azure DevOps | — | **reject** (platform) |
| **vibe-kanban** `BloopAI/vibe-kanban` | Kanban of tasks; each gets "a branch, a terminal, and a dev server"; inline diff comments; PRs "review on GitHub, and merge"; runs as a local server with a web UI, Docker self-host behind a reverse proxy | **Apache-2.0** | `npx` or Docker | None | 28,222★, pushed 2026-09-19; **"Vibe Kanban is sunsetting"**, Bloop shut down 2026-04 | Unmaintained; GitHub only | — | **reference** only (prior study agrees) |
| **Crystal** `stravu/crystal` → Nimbalyst | Many Claude Code / Codex sessions in worktrees | MIT (Crystal) | Desktop | None | 3,122★, last push 2026-02-26; "Crystal is now Nimbalyst", deprecated | Dead upstream | — | **reject** |
| **claude-squad** `smtg-ai/claude-squad` | Terminal manager for many agents with worktrees, built on tmux | **AGPL-3.0** — would force us to publish our source for any modified version we let users reach over a network | Local TUI | None | 8,548★, pushed 2026-08-20 | Licence, and it reads the tmux screen to infer state, which `OD-8` forbids | — | **reject** |
| **Superset** `superset-sh/superset` | "Agentic IDE to orchestrate 100+ coding agents in parallel" | **Elastic License 2.0** (GitHub shows `NOASSERTION`; `LICENSE.md` read): "may not provide the software to third parties as a hosted or managed service" — exactly our SaaS model | Desktop | None | 14,743★, pushed 2026-09-29 | Licence | — | **reject** |
| **Terragon** `terragon-labs/terragon-oss` | Was a cloud "background agent orchestrator for running Claude Code, Codex and other coding CLIs" | Apache-2.0 snapshot | Web app + sandbox provider | — | **Shut down 2026-02-09**; snapshot "as-is, with no guarantees of maintenance" | Dead | — | **reference** (read its sandbox-provider seam at most) |
| **Omnara** `omnara-ai/omnara` | Pivoted: now "an open source platform for running managed agents" — its own harness over the Messages / Responses APIs, pluggable sandboxes (Daytona, Modal, Blaxel, Unikraft) | Apache-2.0 | Docker Compose + Postgres | Org and project roles, RBAC | 2,878★, pushed 2026-09-29 | Does not run the unmodified Claude Code CLI (fails C1 and `OD-8`'s "unmodified" rule) | — | **reject** for the cockpit; not the mobile command centre it used to be |

**Reading of 3a.** Orca is the most capable product in the class and it is permissively licensed,
but it is the same thing we have already built, minus sandboxes, minus Azure DevOps and minus the
enterprise edges. Switching now buys a better terminal polish and costs the isolation model and
the four-phase shell. **Keep ours; take Orca's and Emdash's UX ideas (inline diff comments sent to
the agent, hooks-driven "waiting on you", mobile follow-ups) as reference.**

## 4. Landing the work: pull requests and merge queues

_GitHub merge queue, Azure DevOps PRs and auto-complete, and what phase 3 still owns._

## 5. Recommendation

## 6. What we still build (the glue)

## 7. Calls only the owner can make

## 8. What this study could not verify

## Sources
