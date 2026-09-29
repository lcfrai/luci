# 02 — The agent-fleet cockpit and git lanes: adopt, integrate, buy or keep

**Date:** 2026-09-30. **Status:** complete; research only, nothing installed or run (§8).
**Question:** which existing product can stand in for the multi-agent cockpit (parallel sessions,
one worktree each, live terminals, fork and resume, an attention queue) and for landing the work
back (review, merge queue, PR), instead of us building it?
**Owner direction (2026-09-30):** "focus on using third party solutions ... I need this tool ASAP,
not in weeks."

---

## 0. The answer

**Keep our cockpit. Land through the customer's git host. Take "needs you" from Claude Code's own
hooks. 4.5 integration days.**

1. **No product replaces the cockpit without breaking a hard rule.** The best one, Orca (MIT,
   81,599★, daily releases), is the same single-user desktop class as our phase 2 — with no sandbox
   per session, no Azure DevOps, no Entra, and a status signal read off the terminal stream.
   The enterprise platforms either fail `OD-8`, `OD-19`'s gateway and zero retention, or AU
   residency (GitHub Agent HQ, OpenHands, Warp Oz, Claude Managed Agents), or are an environment
   rather than a cockpit (Coder, AGPL core — a buy question for phase 8; Ona, now being bought by
   OpenAI) (§3).
2. **Landing is where third parties pay.** GitHub's merge queue and Azure DevOps' auto-complete
   replace phase 3's own land queue, forecast and undo: **about 21 of phase 3's 71.5 plan-hours**
   leave the first ship (tasks 08, 10, 19 replaced; 09, 14, 20 shrunk; 16, 17, 21 deferred), and
   team mode's tasks 49 and 61 become host policy and the host queue (§4).
3. **The attention queue costs 2.5 days, not a plan.** Claude Code posts typed HTTP hooks
   (`permission_prompt`, `idle_prompt`, `agent_needs_input`, `Stop`) from managed settings; that is
   the runtime signal phase 2 refused to fake (§6).
4. **Three owner calls** (§7): land on the host page for v1, GitHub Enterprise Cloud as the
   requirement for queued landing, and whether AI first-pass review spends tokens on every PR.

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
| **Orca** `stablyai/orca` | "The ADE for working with a fleet of parallel agents": a worktree per agent, WebGL terminals whose scrollback survives restarts, inline diff comments sent back to the agent, GitHub and Linear in-app, SSH remote worktrees, an always-on `orca serve` host, web and mobile clients that pair to it, per-worktree cloud VMs via user "recipes" | **MIT** | Electron desktop (macOS, Windows, Linux); `orca serve` on a headless Linux box needs Xvfb; clients pair by URL (docs name Tailscale); "Orca does **not** sell managed VPS hosting" | Weak today: no documented SSO or RBAC; the self-host issue asking for team auth and a standalone web client (#10687, opened 2026-07-26) is open with no maintainer reply; enterprise page is "talk to us" | **81,599★**, created 2026-03-17, v1.4.217 on 2026-09-29 (daily releases), Stably AI, YC | Single-user; worktrees share one host, no sandbox per session; no Azure DevOps; no gateway or Entra. Status comes from "Orca-managed hooks that report working / waiting / done", but for Claude Code the hook "emits OSC title events" — an in-band terminal escape Orca reads off the PTY stream, which `OD-8` rules out for us | 12–18 to fork, rebrand and re-seat on our control plane; we would lose the Define/Design/Deploy lenses already in our shell | **reference** |
| **Emdash** `generalaction/emdash` | Parallel agents, a worktree each; issues from Linear, GitHub, Jira, GitLab and others; diff review, PR create and merge; SSH/SFTP remote projects. Status via the harness's **own lifecycle hooks** ("installs marker-tagged entries"); whether its hooks report out-of-band or through the terminal was not inspected | **Apache-2.0** | Electron desktop, all three OSes | None stated (no SSO, team or audit) | 5,881★, pushed 2026-09-29, YC W26 | Same class gaps as Orca; Azure DevOps not listed | — | **reference** — a worked example of hooks-driven status for our attention queue |
| **Sculptor** `imbue-ai/sculptor` | Desktop app: isolated workspaces, Claude Code and Pi built in, review and merge back, GitHub PRs; an **experimental** Docker/remote container backend | **MIT** | Desktop, macOS (Apple Silicon) and Linux only | None | 233★, pushed 2026-09-29; "experimental research preview", Imbue | No Windows build (our users are on Windows); containers experimental | — | **reference** |
| **Conductor** (Melty Labs) | Mac app running Claude Code, Codex and others in worktrees, now with "Conductor Cloud" sandboxes and multiplayer workspaces | **Closed**, commercial | macOS desktop; cloud sandboxes are the vendor's (region not published) | Pro US$50/month, Teams US$60/user/month, Enterprise custom (third-party review sites, 2026; not verified on the vendor site) | Series A US$22M, March 2026 | **Mac-only**: KPMG's fleet is Windows. Vendor cloud, not ours; no Azure DevOps | — | **reject** (platform) |
| **vibe-kanban** `BloopAI/vibe-kanban` | Kanban of tasks; each gets "a branch, a terminal, and a dev server"; inline diff comments; PRs "review on GitHub, and merge"; runs as a local server with a web UI, Docker self-host behind a reverse proxy | **Apache-2.0** | `npx` or Docker | None | 28,222★, pushed 2026-09-19; **"Vibe Kanban is sunsetting"**, Bloop shut down 2026-04 | Unmaintained; GitHub only | — | **reference** only (prior study agrees) |
| **Crystal** `stravu/crystal` → Nimbalyst | Many Claude Code / Codex sessions in worktrees | MIT (Crystal) | Desktop | None | 3,122★, last push 2026-02-26; "Crystal is now Nimbalyst", deprecated | Dead upstream | — | **reject** |
| **claude-squad** `smtg-ai/claude-squad` | Terminal manager for many agents with worktrees, built on tmux | **AGPL-3.0** — would force us to publish our source for any modified version we let users reach over a network | Local TUI | None | 8,548★, pushed 2026-08-20 | Licence; and, per the 2026-09-28 study (its source was not opened, by licence rule), it infers agent state from the rendered screen, which `OD-8` forbids | — | **reject** |
| **Superset** `superset-sh/superset` | "Agentic IDE to orchestrate 100+ coding agents in parallel" | **Elastic License 2.0** (GitHub shows `NOASSERTION`; `LICENSE.md` read): "may not provide the software to third parties as a hosted or managed service" — exactly our SaaS model | Desktop | None | 14,743★, pushed 2026-09-29 | Licence | — | **reject** |
| **Terragon** `terragon-labs/terragon-oss` | Was a cloud "background agent orchestrator for running Claude Code, Codex and other coding CLIs" | Apache-2.0 snapshot | Web app + sandbox provider | — | **Shut down 2026-02-09**; snapshot "as-is, with no guarantees of maintenance" | Dead | — | **reference** (read its sandbox-provider seam at most) |
| **Omnara** `omnara-ai/omnara` | Pivoted: now "an open source platform for running managed agents" — its own harness over the Messages / Responses APIs, pluggable sandboxes (Daytona, Modal, Blaxel, Unikraft) | Apache-2.0 | Docker Compose + Postgres | Org and project roles, RBAC | 2,878★, pushed 2026-09-29 | Does not run the unmodified Claude Code CLI (fails C1 and `OD-8`'s "unmodified" rule) | — | **reject** for the cockpit; not the mobile command centre it used to be |

**Reading of 3a.** Orca is the most capable product in the class and it is permissively licensed,
but it is the same thing we have already built, minus sandboxes, minus Azure DevOps and minus the
enterprise edges. Switching now buys a better terminal polish and costs the isolation model and
the four-phase shell. **Keep ours; take Orca's and Emdash's UX ideas (inline diff comments sent to
the agent, hooks-driven "waiting on you", mobile follow-ups) as reference.**

### 3b. Cloud and enterprise agent platforms (the class that could replace more than the cockpit)

These are the products that could, in principle, stand in for the cockpit *and* the cloud
machines. Each fails at least one hard constraint; the table says which.

| Candidate | What it does | Licence | Deploy | Enterprise fit | Maturity | Gaps against us | Days | Verdict |
|---|---|---|---|---|---|---|---|---|
| **Coder** (workspaces, Coder Agents, AI Gateway, Agent Firewall) `coder/coder` | Self-hosted workspaces on Kubernetes from Terraform templates; Claude Code installs "into a workspace template using a registry module"; its own Go agent ("Coder Agents"); Premium adds AI Gateway, AI Governance and Agent Firewall | **AGPL-3.0** core. Running it unmodified is fine; modifying it and serving it to users forces us to publish those modifications, and shipping it into a customer tenancy needs Coder's commercial terms | Self-host on AKS (Helm); a template can set a Kata `runtimeClassName`, so a pod per task is plausible (not tested) | **Strong**: OIDC SSO (Entra directly or via Keycloak), audit logs, workspace proxies, HA. Premium is "annually per user"; AI Premium is "Custom" with "Unlimited" concurrent agents versus a 5-agent cap otherwise | 16,759★, pushed 2026-09-29, Coder Technologies (VC-backed, shipping since 2021) | No lanes, no merge queue, no provenance; its AI Gateway overlaps `OD-21` LiteLLM; its web dashboard is not our four-phase shell; AGPL for customer tenancies | 8–12 as a `coderbe` Backend under our cockpit | **reference** here; a real **buy** candidate for phase 8 (environment), which is another study's call |
| **OpenHands Agent Canvas** `OpenHands/OpenHands` | Launched 2026-06-16: a "self-hosted developer control center" running OpenHands, Claude Code, Codex, Gemini "or any ACP-compatible agent" across local, Docker, VM and cloud backends, plus Slack / GitHub / Linear automations | **MIT** (no paid `enterprise/` directory found — `tree/main/enterprise` is 404, 2026-09-30) | Self-host, Docker; Python server | Multi-user SSO and audit are in the commercial OpenHands Cloud / Enterprise (not verified in detail) | 89,540★, pushed 2026-09-29, All Hands AI | Third-party agents run **through ACP adapters** — a wrapper around Claude Code, and a chat surface rather than the live terminal (`OD-8`); no Azure DevOps shown; no Kata | 10–15 | **reference** (EventStream, automations) |
| **GitHub Agent HQ / mission control** | Assign one task to Claude, Codex, Copilot and others in parallel; "GitHub Actions or self-hosted runners"; an Agent Control Plane with "security policies and audit logging"; work "lands as draft pull requests" with a Copilot first-pass review | Commercial | SaaS (github.com or GHE.com with data residency, Australia GA 2025-02-04) | Entra via EMU/OIDC; audit. Included in Copilot: Business US$19, Enterprise US$39 per user per month, plus GitHub Enterprise Cloud (third-party price survey, 2026) | Announced 2025-10-28, GA reported 2026-02-04 | **Inference runs through GitHub, not our gateway** (`OD-19` attribution and ZDR). **Copilot data residency is "United States [and] European Union" only — Australia is not listed** (docs, 2026-09-30). GitHub repos only (Azure Boards, not Azure Repos). No live terminal | — | **reference** for the fleet; its **PR and merge-queue half is adopted for landing** (§4) |
| **Warp Oz** | "Orchestration platform for cloud coding agents" (2026-02-10): hundreds of agents, schedules, multi-harness; self-hosted agents on Enterprise | Commercial | Agents can run on our infra, but "Oz orchestrator still manages lifecycle and observability" — the control plane is Warp's cloud | Enterprise plan, contact sales | Warp (VC-backed) | Control plane outside our cloud; **docs.warp.dev is blocked by the KPMG proxy** (redirects to the acceptable-use policy) | — | **reject** |
| **Ona** (ex-Gitpod) | Sandboxed environments, agents, guardrails; Enterprise runners in the customer's VPC | Commercial | Runners in AWS or GCP; an Azure-hosted control plane is claimed by one secondary source only | SSO/OIDC, SCIM, audit on Enterprise | **OpenAI announced it is acquiring Ona (2026-06-11)** | Vendor now owned by a model provider; Azure runners unconfirmed | — | **reject** |
| **AQ** (aq.dev) | "Multiplayer coding harness ... shared live terminals, a code editor, and app previews, in your own cloud" | Commercial | Data plane is a VM in your cloud; control plane is aq.dev | SSO, audit and Azure not stated | Early-stage | SaaS control plane; GitHub only mentioned | — | **reference** (shared terminals are team mode's C-surface) |
| **cube.computer** | "An always-on cloud computer for Claude Code & Codex": Mac cockpit, GitHub sign-in, worktrees, mobile follow-ups | Commercial | Vendor cloud, region not published; prepaid usage | No SSO or audit stated | Small | The product we set out to beat for regulated enterprise; not buyable for it | — | **reference** |
| **Claude Managed Agents** (Anthropic) | Beta since 2026-04 (`managed-agents-2026-04-01`): Anthropic's harness with managed or **self-hosted sandboxes**, events over SSE, scheduled runs; US$0.08 per session-hour plus tokens | Commercial API | Anthropic cloud, or sandboxes on our infra | Org API keys | Beta | "**Not currently eligible for Zero Data Retention**" (docs, 2026-09-30) — fails `OD-19`; not the unmodified CLI; no terminal | — | **reject now**; re-read when ZDR-eligible |

**Reading of 3b.** Nobody sells "unmodified Claude Code, a sandbox per session, a live terminal,
on our Azure, through our gateway, landing into Azure DevOps and GitHub". Coder comes closest on
the enterprise edges and is worth a buy decision for the *environment*; GitHub comes closest on
*landing*. Neither is a cockpit we can adopt: Coder's agent surface runs its own agent (and its
earlier Tasks UI sat on `coder/agentapi`, which the 2026-09-28 study rejected for reading the
rendered screen), and GitHub's fleet fails `OD-19` and AU residency.

### 3c. What we adopt instead — the parts of other products that do fit

| Candidate | What it does for us | Licence / cost | Enterprise fit | Days | Verdict |
|---|---|---|---|---|---|
| **Claude Code's own hooks** (`type: "http"`) | The missing "needs you" signal (C5), from the harness itself: `Notification` with `notification_type` = `permission_prompt`, `idle_prompt`, `agent_needs_input`, `elicitation_dialog`, `agent_completed`; `PermissionRequest`; `Stop`; `SessionStart` (`source` = `startup`/`resume`/`fork`); `SessionEnd`. HTTP hooks POST JSON to a URL with `headers` and `allowedEnvVars`; hooks can ship in **managed settings**, which user and project settings cannot disable | Part of the CLI we already run; no new vendor | Out-of-band, typed, documented; no PTY bytes read, so it keeps `OD-8`. Other harnesses degrade to today's four statuses | 2.5 | **adopt** |
| **GitHub pull requests + merge queue** | Review, ordering, CI-validated landing, revert (§4) | GitHub Enterprise Cloud for private repositories | Entra via EMU; audit log; GHE.com Australia | 1.5 (shared with ADO) | **adopt** |
| **Azure DevOps pull requests + auto-complete + build validation** | The same on Azure Repos, serial rather than queued | Azure DevOps Basic (first five users free, then per user per month) | Entra-native; Australian organisation geography | (in the 1.5) | **adopt** |
| **`anthropics/claude-code-action`** | An AI first-pass review on each lane PR, on our runner, through our gateway | **MIT** | Custom base URL, so LiteLLM and the AU path apply; GitHub only | 0.5 | **adopt** (opt-in per project) |
| **Tauri notification plugin** (`tauri-apps/plugins-workspace`) | OS notification when a session starts waiting on you | MIT / Apache-2.0 | Desktop-local | (in the 2.5) | **adopt** |

## 4. Landing the work: pull requests and merge queues

**The move:** a lane leaves the cockpit as a branch and a pull request on the customer's own git
host, and the host's review, policy, CI and merge machinery does the landing. We stop building our
own land queue, chained forecast and undo. Both hosts are already in `OD-4`, so this adds no vendor.

| | **GitHub** (github.com, or GHE.com in Australia) | **Azure DevOps** (Azure Repos) |
|---|---|---|
| Review | PR diff, inline comments, required reviews, CODEOWNERS | PR diff, inline comments, required reviewers, branch policies |
| Ordering | **Merge queue**: FIFO, "temporary branches with a special prefix", a `merge_group` event for CI, build concurrency 1–100, group size 1–100, "Only merge non-failing pull requests". Private repositories need **GitHub Enterprise Cloud** | **No merge queue** — none native and none found in the marketplace (2026-09-30). Equivalent: **auto-complete** (a 2026 repo setting can turn it on for every new PR) plus a build-validation policy whose build expires when the target branch updates. Serial and correct; costs rebuilds |
| Provenance | Brief key in the PR body; the repo's "pull request title and description" default carries it into the merge or squash commit (one test needed, §8) | `completionOptions.mergeCommitMessage` sets the merge commit message exactly, so the brief key rides as a trailer; `workItemRefs` links a Boards item |
| Undo | "Revert" on a merged PR opens a revert PR | "Revert" on a completed PR opens a revert branch and PR |
| API from Go | REST + GraphQL (auto-merge, enqueue) via `go-github` (BSD-3-Clause) | REST 7.1 (`autoCompleteSetBy`, `completionOptions`, `mergeStrategy` = `noFastForward`/`squash`/`rebase`/`rebaseMerge`) via `azure-devops-go-api` (MIT) — both already chosen in the 2026-09-28 study |
| AI review before a human | `anthropics/claude-code-action` (**MIT**) runs on our runner and accepts a custom base URL, so it goes through LiteLLM (`OD-19`) | No official task; `claude -p` in an Azure Pipelines step through the same gateway (glue) |
| Residency | GHE.com Australia is GA (2025-02-04) for code at rest. Copilot's in-region processing is US and EU only, so **do not use Copilot review** on AU projects | An Azure DevOps organisation can sit in Australia (standard ADO geography; not re-verified today) |

**What this does to phase 3 (24 tasks, 71.5 plan-hours in its own table; 01–02 built, 03 in
fix round):**

| Disposition | Tasks | Plan-hours | Why |
|---|---|---|---|
| **Keep as planned** — C6, the security model and provenance, which no product sells | 01, 02, 03, 04, 05, 06, 07, 11, 12, 13, 15, 18, 22, 23, 24 (15) | 43 | Real clones per worktree, the `Panacea-Session` trailer hook, the sandbox git runner and bundles, fetch-back under `refs/panacea/lanes/…`, push with the **user's** credential (the sandbox never holds one), branch verbs, evidence over ancestry, the proof |
| **Replaced by the host** | 08 chained forecast and queue order · 10 landed history and exact undo · 19 merge lens part 1 (queue, forecasts, land, conflicts) (3) | 8.5 | GitHub's merge queue is the chained forecast, run for real by CI; ADO's auto-complete plus expiring build validation is the serial form; host "Revert" is the undo. A conflict goes back to the lane's agent as a task, not to a resolver we build |
| **Shrunk** | 09 land → push lane branch, open PR, enqueue or set auto-complete, refresh clones after merge · 20 merge lens part 2 → a lane-to-PR strip (checks, queue position, merged) plus the evidence dialog · 14 githost grows by auto-merge, enqueue, auto-complete and `mergeCommitMessage` (3) | 11 → 7.5 | The host does the merge; we show its state |
| **Deferred past first ship** | 16 files read model · 17 `OD-8` transcript adapter · 21 files lens (3) | 9 | Review happens in the host's PR diff for v1; the cross-lane files view and span back-links come back after |

**Net: about 21 of 71.5 plan-hours (29%) leave the first ship, and the hardest-to-get-right part
of phase 3 — a correct concurrent land queue with compare-and-swap and exact undo — becomes a host
feature that already has years of production behind it.** The prior study's go-gitdiff and
sergi/go-diff adoptions now apply only to the deferred tasks 16 and 21.

## 5. Recommendation

**Keep and extend our phase 2 cockpit. Switch to nothing. Land through the customer's git host —
GitHub pull requests with the merge queue, Azure DevOps pull requests with auto-complete — and take
the "needs you" signal from Claude Code's own HTTP hooks.** Integration: **4.5 days** (hooks and the
attention queue 2.5, host landing glue 1.5, AI first-pass review 0.5).

| | |
|---|---|
| **Replaces** | **Phase 3:** tasks 08, 10, 19 replaced (8.5 h); 09, 14, 20 shrunk (11 → 7.5 h); 16, 17, 21 deferred past first ship (9 h) — about **21 of 71.5 plan-hours**. **Team mode:** 49 (team land review) becomes host branch policy — path-scoped required reviewers or CODEOWNERS; 61 (team land queue over the mirror) becomes the host queue; 63 shrinks to a PR list filtered by author. **Phase 2:** closes the named `BLOCKED-ON-YOU` gap, which was never a planned task |
| **Why** | (1) The cockpit is built and already does the one thing no ADE does — a sandbox per untrusted session. Orca is the best of its class, but re-seating it on our control plane is 12–18 days and loses sandboxes, the four-phase shell and Azure DevOps. (2) Landing is where a third party removes real risk: a concurrent land queue with compare-and-swap and exact undo is the hardest code in phase 3, and both hosts already run it in production under the customer's own policies, CI and audit. (3) The hooks are the "agent-runtime signal" phase 2 said it needed, typed and out-of-band, so `OD-8` holds |
| **Risks** | Host-side landing changes the owner-approved phase 3 merge lens (§7). ADO has no queue, so many lanes land serially with rebuilds. Hook payloads can drift with CLI releases — pin the CLI in the image and canary it. A customer's managed policy could strip hooks (KPMG's own laptop policy strips settings-file hooks) — ship them in the image's managed settings and canary that too. The brief trailer must survive each host's merge — one test per host before first ship |

## 6. What we still build (the glue)

**Do now — first ship**

1. **Attention queue (2.5 d).** The sandbox image ships `managed-settings.json` with HTTP hooks for
   `Notification`, `PermissionRequest` (observe only, no decision), `Stop`, `SessionStart` and
   `SessionEnd`, posting to a loopback `sandboxd` endpoint with a per-session bearer
   (`allowedEnvVars`). `sandboxd` relays the typed event on its existing channel; the control plane
   adds `attention` to the session (`waiting_on_you` with a reason, `done`, cleared on next input);
   the cockpit gets a "Needs you" queue ordered by wait time, a pane badge and an OS notification,
   all on the one heartbeat clock. Other harnesses keep today's four statuses. Pin the CLI; a canary
   asserts the hook schema.
2. **Host landing glue (1.5 d)**, inside phase 3 task 14's `githost`: GitHub auto-merge and enqueue;
   ADO `autoCompleteSetBy` with `completionOptions` (`mergeCommitMessage` carrying the brief
   trailer, `mergeStrategy`, `deleteSourceBranch`); poll PR state on the existing tick; the
   lane-to-PR strip in the merge lens; "send back to the agent" for a conflict or a failed check,
   as a templated prompt typed into the session (input, never output parsing).
3. **AI first-pass review (0.5 d):** a `claude-code-action` workflow template for GitHub and an
   Azure Pipelines template that runs `claude -p` through the gateway. Opt-in per project.
4. **Phase 3's kept core, as planned (not glue):** tasks 03 (finishing), 04–07, 11–13, 15, 18,
   22–24 — about 36.5 plan-hours still to build. Nothing sells a credential-free sandbox whose work
   is fetched back under `refs/panacea/lanes/…` with provenance, so this stays ours.

**Later**

5. Files read model, transcript adapter and files lens (phase 3 tasks 16, 17, 21).
6. PR review comments sent back to the lane's agent in one click — the best idea in Orca and
   vibe-kanban, and cheap once item 2 exists.

**Why the hooks keep `OD-8`, stated so a reviewer can check it.** `OD-8` protects three things:
the CLI stays unmodified, nothing guesses meaning from the terminal stream, and nothing depends
silently on undocumented internals. HTTP hooks are documented, delivered out-of-band as typed JSON,
and configured rather than patched in. The amended rule's guards still apply by analogy: pin the CLI
version, canary the payload, and let the attention queue **degrade** to `starting / running / idle /
exited` when no hook arrives — no feature may *require* it. When team mode's `OD-23` policy mount
(phase T tasks 13–15) lands, the hook entries move into its non-overridable layer instead of being
baked into the image.

## 7. Calls only the owner can make

1. **Land on the host for v1 (customer-visible).** Review and merge happen on the GitHub or Azure
   DevOps pull-request page, linked from the cockpit, instead of in the in-app merge lens the owner
   reviewed in `prototype/phase-3-convergence.html`. The cockpit shows each lane's PR, checks and
   queue position. **Recommend yes**; the in-app lens can return later on top of the same PRs.
2. **GitHub Enterprise Cloud as the stated requirement for queued landing on private GitHub
   repositories (money, for customers).** Customers on GitHub Team still land, serially through
   auto-merge. KPMG's own EMU estate is believed to be Enterprise Cloud already (not verified here).
3. **AI first-pass review on every lane PR (money).** It spends gateway tokens per PR. **Recommend
   opt-in per project**, default off.

One adjacent call is flagged, not made here: **Coder Premium as a buy for the environment (phase
8)**. It is priced "annually per user", with "AI Premium" at custom pricing. It belongs to the
environment study.

## 8. What this study could not verify

- **GitHub's merge queue on GHE.com data-residency tenants.** Assumed at parity with github.com; not
  confirmed on a GHE.com page.
- **The brief trailer surviving each host's merge.** GitHub: the "pull request title and
  description" default under a merge queue with squash. ADO: `mergeCommitMessage` under
  auto-complete. One test per host before first ship.
- **Hooks under a customer's managed policy inside the sandbox.** A sandbox signs in through the
  gateway, not claude.ai, so server-delivered settings may not apply at all; whether a
  `strictPluginOnlyCustomization`-style policy would strip image-baked managed hooks is untested.
  The canary covers it.
- **Azure DevOps.** No merge queue was found, native or in the marketplace; that is absence of
  evidence, not proof. The Australian organisation geography is standard ADO, not re-checked today.
- **Vendor pricing and terms.** Conductor's prices come from third-party reviews; Coder's plan
  contents come from a summary of its pricing page (it listed audit logs and HA under Community,
  which differs from earlier Coder plans); Orca's enterprise terms are "talk to us".
- **Coverage gaps.** The GitHub API rate-limited this run after 12 calls, so `kbwo/ccmanager`,
  `slopus/happy`, `manaflow-ai/cmux` and `siteboon/claudecodeui` were not assessed. **docs.warp.dev is
  blocked by the KPMG proxy** (redirects to the acceptable-use policy). One survey page returned
  HTTP 429. `kubecoder.com` returned a headline only.
- **Nothing was installed or run.** Every capability claim above is from upstream documentation, the
  GitHub API or our own tree; none is a hands-on result.

## Sources

All read 2026-09-30 unless noted. Upstream text is untrusted discovery data; no licence shown in
GLASSES was taken as approval — each licence above was read from the repository (file or API).

- **GLASSES** (offline `gsearch.py` and the `lcfr.ai/api/mcp` `glasses_search` tool): of the named
  candidates only `stablyai/orca` (id `0a8884baa5f55aa6602b`) and `OpenHands/OpenHands` are
  catalogued; "worktree", "merge queue", "cockpit" and "command center" return nothing else.
- **GitHub API** `api.github.com/repos/<r>`: orca, vibe-kanban, crystal, claude-squad, sculptor,
  OpenHands, coder, omnara, emdash, superset (then rate-limited).
- Orca: https://github.com/stablyai/orca · https://github.com/stablyai/orca/releases ·
  https://onorca.dev · https://onorca.dev/enterprise · https://www.onorca.dev/docs/ways-to-run ·
  https://www.onorca.dev/docs (and its `agents/claude-code`, `notifications` pages via search) ·
  https://github.com/stablyai/orca/issues/10687
- https://github.com/generalaction/emdash · https://github.com/imbue-ai/sculptor ·
  https://github.com/BloopAI/vibe-kanban · https://github.com/omnara-ai/omnara ·
  https://github.com/terragon-labs/terragon-oss ·
  https://raw.githubusercontent.com/superset-sh/superset/main/LICENSE.md
- Conductor pricing and funding: third-party reviews found by search (vibecoding.app, codepick.dev,
  madewithlove.com); not the vendor's own page.
- Coder: https://coder.com/docs/ai-coder · https://coder.com/pricing
- OpenHands: https://raw.githubusercontent.com/OpenHands/OpenHands/main/README.md ·
  https://www.openhands.dev/blog/introducing-agent-canvas (via search)
- GitHub: https://github.blog/news-insights/company-news/welcome-home-agents/ ·
  https://docs.github.com/en/enterprise-cloud@latest/admin/data-residency/github-copilot-with-data-residency ·
  https://github.blog/changelog/2025-02-04-github-enterprise-cloud-data-residency-in-australia-is-generally-available/ ·
  https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue ·
  https://github.com/anthropics/claude-code-action
- Azure DevOps: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/update?view=azure-devops-rest-7.1 ·
  https://learn.microsoft.com/en-us/azure/devops/release-notes/2026/sprint-270-update ·
  https://learn.microsoft.com/en-us/azure/devops/repos/git/branch-policies
- Warp Oz: https://www.warp.dev/newsroom/2026/2/10/warp-launches-oz-the-orchestration-platform-for-cloud-coding-agents (search result; docs blocked)
- Ona: https://ona.com/pricing and acquisition reports found by search (bex.co, 2026-07-28)
- https://aq.dev/ · https://aq.dev/alternatives/ · https://cube.computer/
- Anthropic: https://platform.claude.com/docs/en/managed-agents/overview ·
  https://code.claude.com/docs/en/hooks
- This repo: `docs/superpowers/planning/2026-09-28-oss-adoption/{00-summary,03-agent-fleet}.md`;
  `docs/superpowers/plans/2026-09-18-phase-2-backend-and-cockpit.md`; phase 3 and phase T plans
  under `.claude/worktrees/{phase-3,team}/docs/superpowers/plans/` (read-only);
  `docs/superpowers/planning/2026-09-23-owner-decisions.md` (`OD-8` text).
