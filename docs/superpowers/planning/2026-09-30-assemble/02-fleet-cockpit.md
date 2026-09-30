# 02 — The agent-fleet cockpit and git lanes: adopt, integrate, buy or keep

**Date:** 2026-09-30. **Status:** first run 09:43; **revised for the owner's second direction**
(§0 is the answer, §R says what changed and why; §3a–§8 are revised in place). Research only:
Orca's source was read from a clone, nothing was installed, built or run (§8).
**Question:** which existing product can stand in for the multi-agent cockpit (parallel sessions,
one worktree each, live terminals, fork and resume, an attention queue) and for landing the work
back (review, merge queue, PR), instead of us building it?
**Owner direction (2026-09-30):** "focus on using third party solutions ... I need this tool ASAP,
not in weeks."

**Owner's second direction (2026-09-30):** Orca (`stablyai/orca`, MIT) is the owner's pick for the
cockpit direction — verify it and plan how to use it, do not re-argue it. Panacea is the strong
default; the best fully-fledged solution wins ("if we want to use Coolify plus Supabase, and enhance
with snippets from Panacea, that is fine as well"). Snippets of Panacea source are now allowed, with
attribution in the file header.

---

## 0. The answer (revised for the owner's second direction)

**Adopt Orca as the cockpit, unforked. Each lane runs in our Kata sandbox through one Panacea
recipe; one Panacea plugin carries the brief and the lane's PR; landing goes through the customer's
git host; the running app is reviewed in a Panacea PR preview. 7.5 integration days, 4.5 to first
ship**, beside 01's 8 days for the sandbox itself.

1. **Orca fits as the cockpit without a fork** (R2, R3). It is MIT, 81,599★, released daily, and its
   source has the two seams we need: *per-workspace environment recipes* (a `create` script returns
   a pairing code or SSH target; a plugin can ship the recipe, so customer repos stay untouched)
   and a *plugin API* (panels, commands, `agent.status.changed`). Its attention queue, inbox, diff
   notes to the agent and hosted review are far ahead of our phase 2. **Correction:** it does create
   Azure DevOps PRs.
2. **Our phase 2 backend survives; its Develop views retire** (R4). The control plane, the
   `Backend` interface, `katabe`, the reaper, the transcript store (resume and fork, which Orca lacks
   for remote work) and the gateway token carry on. Under Orca, `orca serve` owns the PTY and
   `sandboxd` does not wrap it.
3. **The sandbox still never holds a git credential.** Orca's Push lands in a lane-scoped git
   endpoint on our control plane; the control plane pushes with the user's credential, opens the
   PR, and sets GitHub's merge queue or ADO auto-complete (§4). The price: Orca's own GitHub PR
   features stay dark inside lanes in v1 (owner call 5).
4. **Panacea wins after the PR opens** (R5): its PR previews, preview access and comments,
   `panacea-mcp` and the Forge session-id pattern. Coolify and Supabase are not used here.
5. **Phase 3 loses 28.5 of its 71.5 plan-hours from first ship** (R6); team mode's tasks 49 and 61
   become host policy and the host queue.

**Owner calls** (§7): ship Orca under its own name; the Defender prerequisite on Windows; land on
the host page; mobile off; keep the no-credential sandbox. Money: GitHub Enterprise Cloud for the
queue; AI review per PR.

---

## R. Revised for the owner's second direction

### R1. What changed and why

The first run (stopped 09:43) read Orca from its docs and the GitHub API only. This relaunch read
its source (R2), which both confirms the owner's pick and corrects one first-run claim.

| # | What | First run | Now | Why |
|---|---|---|---|---|
| 1 | The cockpit | Keep our Tauri cockpit; Orca **reference** | **Adopt Orca, unforked**; our control plane and Kata sandboxes become its backend through one recipe and one plugin (R3) | The owner's pick; the recipe seam and plugin API reach every v1 need |
| 2 | Orca's gaps | "no sandbox per session, no Azure DevOps, no Entra" | A sandbox per lane through the recipe; **ADO PR creation exists** (a correction); Entra stays on our side (`panacea login`, the `wss://` route) | Source, not docs (R2) |
| 3 | Attention queue | Build it from Claude Code's HTTP hooks, 2.5 days | **Orca's status store** runs the cockpit's queue; our hook relay moves to *later*, for the Hub and team mode (1.5 days) | Orca already ships it, with subagents, an inbox and notifications |
| 4 | Landing | Host PRs, merge queue, ADO auto-complete | Unchanged, but driven by the control plane with the user's credential, plus a **Panacea PR preview** per lane | Orca's hosted review runs inside the sandbox, which holds no credential (R3) |
| 5 | Phase 3 | ~21 of 71.5 plan-hours leave first ship | **28.5 of 71.5** (Orca takes the six desktop tasks); 09 and 24 reshaped | R6 |
| 6 | Panacea | Not considered here | PR previews, `panacea-mcp`, the Forge session-id pattern (R5) | The second direction |
| 7 | Days | 4.5 | **7.5**, 4.5 to first ship, beside 01's 8 | Switching costs 3 days more than keeping ours and buys a far more mature cockpit |

**Unchanged:** every other candidate verdict in §3; landing through the host (§4); the owner calls
on the host page, GitHub Enterprise Cloud and AI review (now §7, items 3, 6 and 7).

### R2. Orca, verified: source, licence, architecture, extension points

Read from source: a blobless clone of `stablyai/orca` at `59b746ff` (2026-09-29); `package.json`
1.4.214, Electron 43.7.5; `LICENSE` is **MIT**, "Copyright (c) 2026 Lovecast Inc." 30,735 files
(`src/main` 10,279 · `src/renderer` 10,560 · `src/shared` 2,494 · `src/relay` 390 · `src/cli` 305 ·
`mobile` 3,434 · `cloud` 527). Nothing was built or run.

| Layer | What it is | Source |
|---|---|---|
| Desktop | Electron main + React renderer; a terminal daemon owns PTYs; hook server, git, hosted review, plugins | `src/main`, `src/renderer` |
| Headless runtime | `orca serve` (same main, no window) or `orcad`; **serves a browser web client** (`/web-index.html`) plus the runtime RPC; each paired client gets a revocable token | `src/main/runtime/rpc/static-web-client-handler.ts`, `src/main/orcad` |
| SSH relay | Installed on the remote on first connect (needs `node-pty`); owns remote PTYs as leases that survive the desktop closing; git and file providers run there | `src/relay`, docs `ssh.mdx` |
| Mobile relay + push | Relay cells and a director on GCP (default `us-central1`, database `us-central1` only); push through APNs/FCM; its API and auth services are in a private repo (`stablyai/orca-cloud`); Relay pairing needs an Orca sign-in | `cloud/README.md`, `docs/reference/relay-regional-placement.md` |

**Where agents run** (`ways-to-run.mdx`): local · SSH target · Remote Orca Server (beta) · **Cloud VM
per workspace** (Settings → Experimental). A recipe's `create` script runs on the Orca host and
prints one JSON object: *Orca-server mode* (`pairingCode`, `projectRoot`) or *SSH mode*
(`connection.type: "ssh"`, a `target` with `host`, `port`, `username`, `identityFile`, `jumpHost`
**or** `proxyCommand`, `portForwards`). Schema v2 `checkoutMode: provisioned-root` has the machine
clone the workspace itself from `ORCA_REPO_URL` / `ORCA_REPO_REF_HEAD` / `ORCA_REPO_BRANCH`.
`suspend`, `resume` and `destroy` read the lifecycle JSON on stdin.

**Extension points.**
1. **Plugin API v1** (`orca-plugin.json`; "EXPERIMENTAL: no compatibility promises until pluginApi
   v1 freezes"). Contributes `panels` (sandboxed HTML frames in the right sidebar), `commands`
   (global or per worktree), `events` (`worktree.created`, `worktree.removed`,
   `agent.status.changed`), `vmRecipes`, `agents`, `keybindings`, `languagePacks`. Capabilities are
   a closed set — `workspace:read`, `terminal:send`, `notifications:show`, `storage`, `secrets`,
   `events:subscribe`, `settings:own`; scoped `net:fetch` and `process:exec` "arrive in later
   phases". Workers are out-of-process Node; install trust, consent, kill list, audit log and a
   private marketplace exist in `src/main/plugins`.
2. **Recipes** in the repo's `orca.yaml` (`environmentRecipes`), or shipped by a plugin.
3. **The `orca` CLI and skills**: `worktree create`, `worktree ps`, orchestration, automations.
4. Hosted-review providers and the status store are built in, not pluggable.

**Attention.** One status store per execution host, fed by HTTP posts from hooks Orca installs in
the agent's own config, by **OSC title events** (for Claude Code "a status-line hook that emits OSC
title events Orca uses for state dots"), by structured-session journals (its native chat), and by a
cancel inferred from Ctrl+C. Rows carry working / waiting / blocked / done, subagents as children;
an inbox with unread, an agent dashboard and mobile push sit on top. This is far ahead of ours.

**Landing.** GitHub is deep: inline checks and comments, **Enable auto-merge**, **Merge when ready**
into the merge queue, stacked PRs. GitLab and Bitbucket create reviews. **Azure DevOps PR creation
exists** (PAT or bearer token from `ORCA_AZURE_DEVOPS_TOKEN` / `ORCA_AZURE_DEVOPS_ACCESS_TOKEN`,
drafts supported) and ADO PR state shows in the sidebar and Checks — *this corrects §3a, which said
"no Azure DevOps"*. No ADO auto-complete was found. "Fix with AI" and "Resolve with AI" hand
failures to an agent through per-repo action recipes. Line attribution (AI vs human) is local only.

**Resume and fork.** Session History resumes from on-disk transcripts (`claude --resume <id>`) **only
from local workspaces**; a remote one gets "Copy resume command". No conversation fork was found.

**Enterprise edges.** Telemetry goes to PostHog Cloud, US; `ORCA_TELEMETRY_DISABLED=1` turns it off.
Account hot-swap and usage read personal subscriptions from `~/.claude` (unused under `OD-19`). No
SSO or RBAC; the team-auth issue #10687 is open. **Orca's own `docs/reference/windows-edr-posture.md`
reports six Microsoft Defender for Endpoint incidents in eight days on an enterprise Windows 11 /
Intune tenant** (1.4.192): path exclusions do not stop them, an MDE alert-suppression rule and an
ASR allowlist do, and each daily update resets file prevalence. Computer use reads as a
Collection-tactic incident; leave it off.

### R3. How we use Orca, concretely

**Ruling: Orca is the cockpit itself, unforked, with our control plane and Kata sandboxes as its
execution backend, joined by one recipe and one plugin.** Not a fork, and not our Tauri cockpit
borrowing its parts:

- **Not a fork (for v1).** 24,200 source files and a release a day. The plugin API and recipes reach
  every seam v1 needs. A fork means our own Windows code signing (Orca's builds are signed through
  SignPath), a fresh Defender prevalence reset on every build we ship, and a daily rebase. Fork only
  for a must-have that upstream refuses — first candidate: 05's Azure Boards task source, offered
  upstream as a pull request first.
- **Not borrowing parts.** Orca's value is the integrated whole (status store with subagents, inbox,
  diff notes sent to the agent, hosted review, session history). Lifting pieces out of an
  Electron/IPC codebase into Tauri is the 12–18-day path §3a priced, for less.

**The flow, end to end** (sandbox side as in 01 §1a; the recipe seam verified in R2):

| # | Step | Who | How |
|---|---|---|---|
| 1 | Sign in, once | `panacea login` (our Go CLI) | Auth-code + PKCE through the Keycloak realm brokering Entra (`OD-30`); token in Windows Credential Manager (`OD-6`); the recipe scripts reuse it silently |
| 2 | Offer "Run on: Panacea sandbox" | the **Panacea plugin** | Contributes the recipe through `vmRecipes`; Orca merges plugin recipes with the repo's own (`listRecipes`), so **no `orca.yaml` lands in customer repos** |
| 3 | Create a lane | recipe `create` (Windows script) | Calls the control plane's create-lane route (phase 3 task 04) with `ORCA_REPO_URL`, `ORCA_REPO_REF_HEAD`, the branch and the brief key. The control plane clones the lane onto the sandbox disk (task 03), claims a warm Kata sandbox (01), mints the per-session gateway token (`OD-19`) and points `origin` at the lane git endpoint; the sandbox starts `orca serve --project-root … --pairing-address wss://…/lanes/<id> --recipe-json`; the script prints `{pairingCode, projectRoot, userData.resourceId}` |
| 4 | Work | Orca, upstream | Its runtime in the sandbox owns the terminals, end-to-end encrypted over our authenticated `wss://` route; it launches unmodified Claude Code (`ANTHROPIC_BASE_URL` = LiteLLM, AU or Global per project) and installs its hooks there, so the desktop shows working / waiting / done, the inbox and notifications. Diff notes sent to the agent work as upstream |
| 5 | Hand in | Orca's Commit and Push | Push goes to the control plane's **lane git endpoint** (session-token auth, lane branch only); the sandbox never holds a git-host credential (phase 3 decisions 9–10) |
| 6 | Land | plugin command + control plane | "Open PR" pushes the lane branch to GitHub or Azure DevOps **with the user's credential**, opens the PR with the brief trailer, and enables GitHub auto-merge / the merge queue or ADO auto-complete (`mergeCommitMessage` carries the trailer). Panacea deploys a PR preview (R5). Review and merge on the host page |
| 7 | Suspend, resume, destroy | recipe scripts | agent-sandbox pause, resume and delete (01) |

**Resume and fork stay ours.** Orca resumes only local workspaces and has no fork (R2). Phase 2's
transcript store and fork (`--resume A --fork-session` into a new worktree) become two plugin
commands — "Resume in a fresh sandbox" and "Fork into a new lane" — each a recipe create with a
source session.

**Gaps that remain, each with its fix:**
- **Orca's own GitHub PR features stay dark inside a sandbox lane** (inline checks, "Fix broken
  checks", "Merge when ready"): hosted review runs on the execution host
  (`src/main/source-control/hosted-review-execution-host.ts`), and that host holds no GitHub
  credential. The plugin's lane strip and the host page cover v1; lighting them up needs a
  repo-scoped, one-hour GitHub App token in the sandbox (owner call, §7).
- **Recipes are "Experimental" and run only from the desktop** ("Ephemeral VM recipes run on the
  local desktop host in v1", `src/main/ipc/ephemeral-vm-recipe-context.ts`), and a project anchors on
  a local clone on the laptop. `panacea login` can make that clone for non-developers.
- **The plugin API is experimental**: panels are `srcdoc` frames with navigation blocked, and there is
  no `net:fetch` capability yet. If the worker cannot reach the Hub, the lane strip is a Hub page in
  Orca's built-in browser tab instead (spike on day 1).
- **Orca reads OSC title events for its state dots.** Our code never consumes them; anything we build
  on status (Hub, team mode) takes Claude Code's typed hooks through `sandboxd`. Treated as a
  display detail inside the adopted cockpit; if `OD-8` must bind Orca too, the fix is a small
  upstream setting that prefers hooks for hook-capable agents (0.5 day).
- **Windows Defender for Endpoint** needs an alert-suppression rule and an ASR allowlist per tenant
  (R2; owner call, §7). **Mobile** stays off: Orca Relay is US-hosted and needs an Orca sign-in.

**Later, not first ship:** a browser-only cockpit. `orca serve` already serves a web client that
expects a reverse-proxy path prefix (`vite.web.config.ts`: `base: './'`), which would remove the
desktop install and the Defender work. It waits on recipes running from a remote runtime.

### R4. What of phase 2 survives

**The backend survives; the Develop views of the Tauri desktop retire.**

| Phase 2 piece (built) | Under Orca |
|---|---|
| Control plane: sessions API, `backend.Backend` (`dockerbe`, `katabe`), reaper, restart recovery | **Survives** — it becomes the lane and sandbox API the recipe calls |
| Transcript store, resume, fork | **Survives** — two plugin commands; Orca has no remote resume and no fork |
| Per-session gateway token wiring (`OD-19`) | **Survives** unchanged |
| `sandboxd` | **Survives, smaller.** Ruling for 01 §1a: under an Orca lane, `orca serve` owns the PTY and `sandboxd` does **not** wrap it (two PTY owners would double-buffer for nothing). `sandboxd` keeps health, the Claude Code hook receiver for the Hub and team mode, and every session that is not Orca's (team mode's managed sessions, API runs) |
| Attach relay `GET /v1/sessions/{id}/attach` | Kept for non-Orca sessions; Orca lanes stream over Orca's own encrypted channel through the `wss://` route |
| Worktree catalog (read-only) | Superseded by the lane index (phase 3 task 03) |
| Tauri Develop views: rail, sub-rail, Columns and Canvas layouts, session panes, split create, palette, keyboard map, heartbeat clock | **Retired** — Orca's sidebar, splits, quick open, agent dashboard and inbox replace them |
| Status: four observed facts; `BLOCKED-ON-YOU` never built | **Superseded** by Orca's status store (R2) |
| Tauri shell for the Define, Design and Deploy lenses | Not this study's ruling. Direction that keeps one app: those lenses become Hub web pages, opened in Orca's browser tabs from the plugin |

**Net:** of phase 2's work, everything under `control-plane/` and `sandboxd/` carries on;
`desktop/src/components/{develop,rail,subrail,palette}` stops being the product's Develop surface.

### R5. Panacea, Coolify and Supabase for this capability, on merit

Panacea was grepped read-only (`Projects/Panacea`, nothing run); `00-panacea.md` (in progress
today) covers the full inventory.

| Need in this capability | Best complete answer | Why, in one line |
|---|---|---|
| The cockpit | **Orca** (R3) | Panacea has no cockpit; Coolify and Supabase have none |
| A sandbox per lane | **01's adopt** (AKS Pod Sandboxing + `agent-sandbox`) | Forge's preview pods are shared-kernel and preview-shaped (01 §1) |
| See the running app for each lane | **Panacea PR previews** — `webhook-receiver` routes `pull_request_opened` to the preview pipeline and `pull_request_closed` to cleanup (`pipeline/trigger.go:1466-1481`), gated per project (`:453-462`) behind a fail-closed input check for fork PRs (`:402-421`); preview access by SSO, IP list or share link and a preview-comments widget (00 §3) | Already on Azure behind Keycloak with Trivy and Falco. Coolify's previews centre on its GitHub App (its docs cover no Azure DevOps), run on Docker hosts rather than AKS, and bring a second identity |
| Land: review, order, merge, revert | **GitHub merge queue / ADO auto-complete** (§4) | Panacea has no pull-request or queue routes (`routes.go`: none) |
| An agent deploys from inside its lane | **`panacea-mcp`** with a scoped `pnc_` token (00 §4) | Works with an unmodified agent today |
| Session → deploy provenance | **Panacea's Forge pattern, copied with attribution**: every ship stamps `X-Panacea-Session-Id` (`webhook-receiver/models/event.go:16`), which routes to the agent pipeline and reports back (`trigger.go:1504-1525`) | Our lane and brief key ride the same way instead of a new mechanism |
| Attention across devices and teammates | **Orca** for my own agents; **the Hub** for teammates (05) | Supabase Realtime could fan events out, but Orca already mirrors status to its clients and team state is the Hub's call |

**The trade-off in one line:** Panacea is the better complete answer for everything after a lane
becomes a pull request (preview, scan, deploy, promote, evidence); it has nothing for the cockpit or
the lane mechanics, which Orca and our control plane cover. **Coolify** (Apache-2.0) and
**Supabase** (Apache-2.0) are not used here.

**One Panacea gap to close before promising it on Azure DevOps:** `panacea-api` subscribes ADO
`git.push` and `git.pullrequest.created` only (`handlers/webhook_ado.go:42`), and the receiver's
PR-event mapping was found only for GitHub (`webhook-receiver/providers/github.go:168-173`). So ADO
PR previews, and their cleanup when a PR completes, are unverified.

### R6. The revised recommendation

**Adopt Orca as the cockpit, unforked. Run every lane in our Kata sandbox through one Panacea
recipe, add one Panacea plugin, land through the customer's git host, and review the running app
in a Panacea PR preview. Integration: 7.5 days, 4.5 of them to first ship** — on top of 01's 8 days
for the sandbox, the recipe's JSON, the `wss://` route and the clone before bind (not counted twice).

**What we still build (glue):**

| When | Item | Days |
|---|---|---|
| First ship | **Orca rollout package**: one pinned version; Intune or winget install; `ORCA_TELEMETRY_DISABLED=1`; computer use and mobile off; the plugin preinstalled; a one-page Defender note for IT | 0.5 |
| First ship | **`panacea login`**: PKCE through Keycloak (`OD-30`), Windows Credential Manager (`OD-6`), the local anchor clone for non-developers; the recipe scripts call it | 0.5 |
| First ship | **Lane git endpoint** in the control plane: `git http-backend` behind session-token auth, a pre-receive hook that allows the lane branch only, stored under `refs/panacea/lanes/…` (`OD-2`) | 1.5 |
| First ship | **Landing**: push the lane branch with the user's credential; open the PR with the brief trailer; GitHub auto-merge or enqueue; ADO auto-complete with `mergeCommitMessage`; poll PR state; "send back to the agent" as a typed prompt (`terminal:send`), never output parsing | 1.5 |
| First ship | **Panacea PR preview and provenance**: previews on per project; the lane and brief key stamped the Forge way (R5); the preview URL on the lane | 0.5 |
| Week 2 | **Panacea plugin**: a worktree panel (brief card; lane → PR → checks → preview → release strip) and commands (Open PR, Open preview in Orca's browser, Resume in a fresh sandbox, Fork into a new lane); falls back to a Hub page in a browser tab if the worker cannot fetch | 2 |
| Week 2 | **Resume and fork** wired from phase 2's transcript store through the recipe | 0.5 |
| Week 2 | **AI first-pass review** template, opt-in per project (§3c) | 0.5 |
| Later | Claude Code HTTP hooks → `sandboxd` → Hub, for team mode and the Orchestrators (the first run's §6 item 1, minus the cockpit UI Orca now supplies) | 1.5 |

**What it replaces:**

| Plan | Disposition |
|---|---|
| **Phase 3** (24 tasks, 71.5 plan-hours) | **Replaced by Orca:** 06, 18, 19, 20, 21, 22 (6 tasks, 17 h) — the desktop wire types, merge lens, files lens and lifecycle-verb UI. **Replaced by the host:** 08, 10 (2 tasks, 5.5 h) — forecast, queue order, landed history and undo. **Deferred:** 16, 17 (6 h). **Reshaped:** 09 becomes the landing glue; 24's bundle RPC becomes the lane git endpoint (12 h of plan, now glue). **Kept as planned:** 01, 02, 03, 04, 05, 07, 11, 12, 13, 14, 15, 23 (12 tasks, 31 h) — clones, the trailer hook, credentials, `githost`, evidence, the proof. **28.5 of 71.5 plan-hours (40%) leave the first ship** |
| **Phase 2** (built) | The Tauri Develop views retire (R4); the `BLOCKED-ON-YOU` gap closes through Orca's status store |
| **Phase 8** | Tasks 22 (`remotebe`) and 26 (the cloud-machine view), 6 plan-hours, become the recipe (01 §1) |
| **Team mode** (69 tasks) | 49 → host branch policy (required reviewers or CODEOWNERS); 61 → the host queue; 63 → a PR list filtered by author |

**Risks.** Orca's recipes ("Experimental") and plugin API ("no compatibility promises") move
daily: pin one Orca version in the image and on the desktop, and canary each upgrade — the guard
`OD-8` already asks for. Every lane sandbox carries an Electron runtime and Xvfb (01 sizes it). The
laptop needs a local anchor clone. ADO PR previews in Panacea are unverified (R5). This category
loses products fast (Crystal, vibe-kanban and Terragon all stopped in 2026); MIT lets us fork Orca
if Stably AI changes course.

---

## The first run's study (09:43), revised in place where §R changes it

The first run answered "keep our cockpit, land through the host, take 'needs you' from Claude
Code's hooks, 4.5 days". §0 replaces that answer. What still stands from it: the enterprise
platforms in §3b each fail a hard rule (`OD-8`, `OD-19`'s gateway and zero retention, AU residency)
or are an environment rather than a cockpit; landing through GitHub's merge queue and ADO
auto-complete (§4); and the reasoning of §1–§2. Sections §3a, §3c and §4–§8 below are revised in
place, each change marked **Revised**.

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

These all solve C1, C3 and C4 on one person's machine. None ships C2 (a sandbox per untrusted
session), C9 (Entra, gateway, our cloud) or team mode. **Revised:** Orca reaches C2 through its
per-workspace recipe seam and does open Azure DevOps PRs (R2); the others still do neither.

| Candidate | What it does | Licence | Deploy | Enterprise fit | Maturity | Gaps against us | Days | Verdict |
|---|---|---|---|---|---|---|---|---|
| **Orca** `stablyai/orca` | "The ADE for working with a fleet of parallel agents": a worktree per agent, WebGL terminals whose scrollback survives restarts, inline diff comments sent back to the agent, GitHub and Linear in-app, SSH remote worktrees, an always-on `orca serve` host, web and mobile clients that pair to it, per-worktree cloud VMs via user "recipes" | **MIT** | Electron desktop (macOS, Windows, Linux); `orca serve` on a headless Linux box needs Xvfb; clients pair by URL (docs name Tailscale); "Orca does **not** sell managed VPS hosting" | Weak today: no documented SSO or RBAC; the self-host issue asking for team auth and a standalone web client (#10687, opened 2026-07-26) is open with no maintainer reply; enterprise page is "talk to us" | **81,599★**, created 2026-03-17, v1.4.217 on 2026-09-29 (daily releases), Stably AI, YC | **Revised (R2):** single-user, no SSO or RBAC; a sandbox per lane only through a recipe ("Experimental", desktop-run in v1); hosted review runs inside the lane, so it needs a credential there; resume only for local workspaces, no fork; mobile relay US-hosted; Windows Defender incidents on enterprise tenants. Status mixes HTTP hook posts with OSC title events (display only for us, R3). **Azure DevOps PR creation and status do exist** — the first run's "no Azure DevOps" was wrong | **7.5** as the cockpit through one recipe and one plugin (R6); 12–18 only if forked and re-seated | **adopt** (owner's pick; R3) |
| **Emdash** `generalaction/emdash` | Parallel agents, a worktree each; issues from Linear, GitHub, Jira, GitLab and others; diff review, PR create and merge; SSH/SFTP remote projects. Status via the harness's **own lifecycle hooks** ("installs marker-tagged entries"); whether its hooks report out-of-band or through the terminal was not inspected | **Apache-2.0** | Electron desktop, all three OSes | None stated (no SSO, team or audit) | 5,881★, pushed 2026-09-29, YC W26 | Same class gaps as Orca; Azure DevOps not listed | — | **reference** — a worked example of hooks-driven status for our attention queue |
| **Sculptor** `imbue-ai/sculptor` | Desktop app: isolated workspaces, Claude Code and Pi built in, review and merge back, GitHub PRs; an **experimental** Docker/remote container backend | **MIT** | Desktop, macOS (Apple Silicon) and Linux only | None | 233★, pushed 2026-09-29; "experimental research preview", Imbue | No Windows build (our users are on Windows); containers experimental | — | **reference** |
| **Conductor** (Melty Labs) | Mac app running Claude Code, Codex and others in worktrees, now with "Conductor Cloud" sandboxes and multiplayer workspaces | **Closed**, commercial | macOS desktop; cloud sandboxes are the vendor's (region not published) | Pro US$50/month, Teams US$60/user/month, Enterprise custom (third-party review sites, 2026; not verified on the vendor site) | Series A US$22M, March 2026 | **Mac-only**: KPMG's fleet is Windows. Vendor cloud, not ours; no Azure DevOps | — | **reject** (platform) |
| **vibe-kanban** `BloopAI/vibe-kanban` | Kanban of tasks; each gets "a branch, a terminal, and a dev server"; inline diff comments; PRs "review on GitHub, and merge"; runs as a local server with a web UI, Docker self-host behind a reverse proxy | **Apache-2.0** | `npx` or Docker | None | 28,222★, pushed 2026-09-19; **"Vibe Kanban is sunsetting"**, Bloop shut down 2026-04 | Unmaintained; GitHub only | — | **reference** only (prior study agrees) |
| **Crystal** `stravu/crystal` → Nimbalyst | Many Claude Code / Codex sessions in worktrees | MIT (Crystal) | Desktop | None | 3,122★, last push 2026-02-26; "Crystal is now Nimbalyst", deprecated | Dead upstream | — | **reject** |
| **claude-squad** `smtg-ai/claude-squad` | Terminal manager for many agents with worktrees, built on tmux | **AGPL-3.0** — would force us to publish our source for any modified version we let users reach over a network | Local TUI | None | 8,548★, pushed 2026-08-20 | Licence; and, per the 2026-09-28 study (its source was not opened, by licence rule), it infers agent state from the rendered screen, which `OD-8` forbids | — | **reject** |
| **Superset** `superset-sh/superset` | "Agentic IDE to orchestrate 100+ coding agents in parallel" | **Elastic License 2.0** (GitHub shows `NOASSERTION`; `LICENSE.md` read): "may not provide the software to third parties as a hosted or managed service" — exactly our SaaS model | Desktop | None | 14,743★, pushed 2026-09-29 | Licence | — | **reject** |
| **Terragon** `terragon-labs/terragon-oss` | Was a cloud "background agent orchestrator for running Claude Code, Codex and other coding CLIs" | Apache-2.0 snapshot | Web app + sandbox provider | — | **Shut down 2026-02-09**; snapshot "as-is, with no guarantees of maintenance" | Dead | — | **reference** (read its sandbox-provider seam at most) |
| **Omnara** `omnara-ai/omnara` | Pivoted: now "an open source platform for running managed agents" — its own harness over the Messages / Responses APIs, pluggable sandboxes (Daytona, Modal, Blaxel, Unikraft) | Apache-2.0 | Docker Compose + Postgres | Org and project roles, RBAC | 2,878★, pushed 2026-09-29 | Does not run the unmodified Claude Code CLI (fails C1 and `OD-8`'s "unmodified" rule) | — | **reject** for the cockpit; not the mobile command centre it used to be |

**Reading of 3a (revised).** Orca is the most capable product in the class, permissively
licensed, and — read at source — extensible exactly where we need it: a recipe puts each lane in our
sandbox, so the isolation model is kept, and a plugin carries the brief and the lane's PR. **Adopt
Orca as the cockpit (R3); keep Emdash, Sculptor and vibe-kanban as reference.** The first run's
"keep ours" rested on Orca lacking sandboxes and Azure DevOps; the source shows the seam for the
first and the feature for the second.

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
| **Claude Code's own hooks** (`type: "http"`) | The missing "needs you" signal (C5), from the harness itself: `Notification` with `notification_type` = `permission_prompt`, `idle_prompt`, `agent_needs_input`, `elicitation_dialog`, `agent_completed`; `PermissionRequest`; `Stop`; `SessionStart` (`source` = `startup`/`resume`/`fork`); `SessionEnd`. HTTP hooks POST JSON to a URL with `headers` and `allowedEnvVars`; hooks can ship in **managed settings**, which user and project settings cannot disable | Part of the CLI we already run; no new vendor | Out-of-band, typed, documented; no PTY bytes read, so it keeps `OD-8`. Other harnesses degrade to today's four statuses | 1.5, later | **Revised: integrate later** — Orca's status store now runs the cockpit's queue; these hooks feed the Hub and team mode through `sandboxd` (R6) |
| **GitHub pull requests + merge queue** | Review, ordering, CI-validated landing, revert (§4) | GitHub Enterprise Cloud for private repositories | Entra via EMU; audit log; GHE.com Australia | 1.5 (shared with ADO) | **adopt** |
| **Azure DevOps pull requests + auto-complete + build validation** | The same on Azure Repos, serial rather than queued | Azure DevOps Basic (first five users free, then per user per month) | Entra-native; Australian organisation geography | (in the 1.5) | **adopt** |
| **`anthropics/claude-code-action`** | An AI first-pass review on each lane PR, on our runner, through our gateway | **MIT** | Custom base URL, so LiteLLM and the AU path apply; GitHub only | 0.5 | **adopt** (opt-in per project) |
| **Tauri notification plugin** (`tauri-apps/plugins-workspace`) | OS notification when a session starts waiting on you | MIT / Apache-2.0 | Desktop-local | — | **Revised: not needed** — Orca's notifications and inbox replace it |
| **Panacea PR previews** *(added)* | A live, SSO-gated preview per lane PR, with preview comments; cleanup on close (GitHub) | Ours (Panacea) | On Azure behind Keycloak; Trivy and Falco in the pipeline; ADO path unverified (R5) | 0.5 | **adopt** (call it) |
| **`panacea-mcp`** *(added)* | The agent deploys, reads logs and diagnoses from inside its lane | Ours (Panacea) | Scoped `pnc_` token; RBAC in the API (00 §4) | (in 01) | **adopt** |
| **Coolify** *(added)* | PR preview deployments | Apache-2.0 | Previews centre on its GitHub App; no Azure DevOps in its docs; Docker hosts, not AKS; a second identity | — | **reject** here (06 weighs it for deploy) |
| **Supabase** *(added)* | Realtime fan-out of status | Apache-2.0 | Would duplicate Orca's client mirroring; team state is the Hub's call | — | **not used** here |

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
