# 01 — Internal inventory: what the repo plans toward team collaboration, and where it stops

**Date read:** 2026-09-24. **Author:** research agent, team-collaboration run (`RESUME.md:109-114`).
**Scope:** read-only survey of the main tree (`feat/phase-2-cockpit`) and the worktrees `hub`,
`phase-3`, `phase-4`, `phase-5`, `phase-7`, `env`. Paths under `.claude/worktrees/<name>/` are
written `wt/<name>/…`. Line numbers are as read on 2026-09-24; several of those trees are being
edited live, so a cited line can drift.

**The owner's four capabilities**, as this document numbers them:

| # | Capability |
|---|---|
| C1 | Share a project with a team |
| C2 | Everyone **sees** each member's work, running agents and each agent's focus, live |
| C3 | Each member spins up **their own** agents on the same repo, from their own machine |
| C4 | Every agent, whoever owns it, knows what the other agents are doing, so they coordinate |

## 0. The verdict in five lines

1. **"The research recently" is the Hub: `wt/hub/docs/superpowers/adr/ADR-H1-hub-architecture.md`**
   plus its contract and plan (§1). It was the answer to `OD-3` "shared from day one".
2. **C1 is planned in full and not built.** Teams, membership, invites and a Share popover are in
   the Hub plan (tasks 15, 35, 42). Only `hubapi`'s grammar exists in code (§2.1).
3. **C2 and C4 are explicitly out of scope everywhere.** The Hub ADR keeps *"sessions, machines,
   worktrees and PTYs local. The Hub never sees a terminal byte"* (ADR-H1 `:88`). No document
   plans a shared session list, presence, an agent's focus, or an agent-to-agent channel (§2.2, §2.4).
4. **C3 works by accident, not by design.** Each teammate can already run agents on their own
   clone of the same remote, but nothing joins the two fleets: lanes, the queue and the forecast
   exist only in one control plane (§2.3).
5. **The blocking assumption is one sentence repeated in four plans:** one person's control plane
   is the only observer of that person's agents, and it tells nobody (§3).

That gap between the plans and the owner's ask is why it *"hasn't been implemented properly yet"*:
the research answered "shared **records**", and the owner is asking for shared **activity**.

## 1. Which document is "the piece of research recently"

**Answer: the Hub research of 2026-09-23 — `wt/hub/docs/superpowers/adr/ADR-H1-hub-architecture.md`**,
with `wt/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md` and
`wt/hub/docs/superpowers/plans/2026-09-23-phase-h-hub.md` (43 task parts). Grounds:

- It is the only document whose subject is more than one user. Its trigger chain is explicit:
  critique-2 §7.1 #2 asked *"Per person, or shared? … The contract never considers a second user"*
  (`docs/superpowers/planning/2026-09-23-phases-4-5-7/completeness-critique-2.md:522`); the owner
  answered **"Shared from day one"** (`docs/superpowers/planning/2026-09-23-owner-decisions.md:14`,
  `OD-3`); `OD-5` (`:16`) gave the orchestrator the architecture; the owner decisions then created
  "Phase H — the Hub" (`owner-decisions.md`, section "The new phase: the Hub").
- ADR-H1 is dated 2026-09-23 (`ADR-H1:3`) and says *"nothing here is built"* (`:3-4`). It scored
  three designs, one of which — **C, "provenance and collaboration first"** — carried realtime
  WebSocket presence (`:177-178`). That graft was **rejected for v1** (`:264-268`).
- Its plan ends in `task-43.md`, *"two teammates … share one repository's project, and see each
  other's brief revision, design approval, release pin and evidence"* (`wt/hub/…/phase-h-hub-parts/task-43.md:8-11`).
  **Sessions and agents are not on that list.** That is the precise sense in which the research
  "hasn't been implemented properly": it is unbuilt, and even when built it shares records, not
  running work.
- A search of every tree on 2026-09-24 for `*collab*`, `*team*`, `*presence*`, `*fleet*` found no
  earlier collaboration document; the only hit is this run's own directory. `git log --all
  --grep=team --grep=collab` shows the Hub plan's commits (`29049c8` task 42 "share a project with
  a team", `b28ef4c` task 43 "two-teammate end-to-end proof", `fd93074` task 15 teams/members).

**Supporting research, each a partial answer to the ask:**

| Document | What it contributes | Cite |
|---|---|---|
| Critique-2 §7.1 #2 | Posed "per person or shared" for the stores | `completeness-critique-2.md:522`, `:341-342` |
| Phase 7 scope-and-risk, D1 | *"A release a teammate ships from their cockpit has no release link in my ledger"* | `docs/superpowers/planning/2026-09-23-phases-4-5-7/scope-and-risk-phase-7-deploy.md:20`, `:232` |
| Phase 3 scope-and-risk | Lands are trailers in git, so *"visible to every teammate who fetches"*; **no declared file claims** (A9) | `scope-and-risk-phase-3-git-convergence.md:96`, `:175`, `:191`, `:287` |
| Phase 5 survey | *"Multi-human collaboration is optional in the product sense"*; the canvas in a user's VM needs *"a route into the VM"* for multi-human | `survey-phase-5-design.md:288`, `:440` |
| Entra IT request | Q4: teammates join by email invite in the same tenant | `docs/superpowers/planning/2026-09-24-it-request-entra.md:103` |

## 2. The four capabilities, one at a time

Status words: **built** (code on a branch, tested), **planned** (a plan task with a contract),
**named** (a sentence in a survey or ADR, no task), **absent** (nothing found).

### 2.1 C1 — share a project with a team: **planned in full, not built**

| Piece | Status | Cite |
|---|---|---|
| Orgs, users, teams, membership `admin`/`member`/`viewer`, invites by email in the same tenant | planned (Hub task H2 / plan task 15) | ADR-H1 `:402-406`; contract routes 3-12 `wt/hub/…/2026-09-23-hub-contract.md:565-574` |
| A Hub project owned by one team, keyed to its git remotes (`githost.RepoKey`), unique per org | planned (H3) | ADR-H1 `:407-408`; contract routes 13-18 `:575-580` |
| Local project → Hub project resolver, and the share routes (seam S-LINK) | planned (plan task 35) | `wt/hub/…/phase-h-hub-parts/task-35.md:1-12` |
| Share popover on the project's menu, "a new team named after the project" by default | planned (plan task 42), UI drawn first in `prototype/phase-h-hub.html` | `task-42.md:1-40`; that prototype **does not exist yet** in `wt/hub/prototype/` (listed 2026-09-24) |
| Non-members get 404, never 403; no request body carries an actor | planned | ADR-H1 `:433-435` |
| Code that exists today | **built: only the wire grammar** — `control-plane/internal/hub/hubapi` (ids, codes, keys, canonical encoding) | `wt/hub` `git log` head `94d6625` (tasks 01-02); no `internal/hub/server` or `store` directory; no `cmd/hub` |

**What is missing for the owner's C1:** nothing in shape. Sharing a project gives teammates the
project's **records** (briefs, approvals, releases, evidence) and nothing about its activity; the
missing parts are C2-C4. **Why local ids forced this design:** `ProjectID` is a hash of the absolute
path on one laptop (`control-plane/internal/project/project.go:101-102`, cited ADR-H1 `:121-125`), so
a shared project must be keyed by its remote, not by anything the control plane mints.

### 2.2 C2 — everyone sees each member's work, agents and focus, live: **absent**

**Every plan keeps sessions local, deliberately.**

| Fact | Cite |
|---|---|
| *"Sessions, machines, worktrees and PTYs stay local. The Hub never sees a terminal byte."* | ADR-H1 `:88` |
| When the Hub is down, *"Local sessions, terminals, worktrees — unaffected. They never touch the Hub"* | ADR-H1 `:465` |
| The session registry is one JSON file in the user's state directory | `control-plane/internal/registry/registry.go:56-62` (`statepath.Dir("sessions")/sessions.json`) |
| A session record has **no owner or user field**; its only "focus" data is `Title` and `OpeningPrompt`, plus `Status`, `LastOutputAt`, `ContextRef` | `control-plane/internal/registry/session.go:42-54`, `:88-104` |
| The control plane refuses a non-loopback bind: *"a sandbox attach socket on a non-loopback address is a shell on the LAN"* | `control-plane/cmd/api/config.go:73`, `:354` |
| The one Hub record that names a session is the session → design link; its `sessionId` *"means something only on the creator's machine; teammates see it with `createdBy`"* | contract `:941-942` (§7.8) |
| The change feed carries Hub **writes** only (brief, approval, release, evidence events), polled every 5 s | ADR-H1 `:478-483`; contract route 43 `:605` |
| Presence and cursors are deferred to "Phase 5 v2", and only for the canvas | ADR-H1 `:264-268`, `:584` |
| The prototypes draw one user: every avatar is "DM"; the only "presence" markers are **agent** cursors on the canvas | `prototype/index.html:350`, `:403`, `:494`, `:539`, `:656` |

**What is missing:** (a) a shared record of a running session — who owns it, which project, branch
and worktree, its status, its focus; (b) a feed teammates' cockpits read; (c) a way to see a
teammate's agent's **work** (diff, commits) before it is pushed; (d) a definition of "focus" that
is not scraped from PTY bytes (`OD-8` forbids that). **Nearest pieces that exist:** the Hub's
ordered change feed (G3, ADR-H1 `:252`) is the right transport shape and nothing uses it for
sessions; phase 3's lane sync computes *what each worktree is changing, from git alone*
(`wt/phase-3/…/phase-3-git-convergence-parts/task-07.md:1`), but only on the owner's machine.

### 2.3 C3 — each member runs their own agents on the same repo: **possible in isolation, never joined**

Nothing stops two teammates each cloning the same remote and running agents today. What exists
makes each fleet **correct alone** and **invisible to the other**.

| Piece | Status | Cite |
|---|---|---|
| Each user's agents run on **their own** subscription; nothing is pooled | locked | `CLAUDE.md` "AI capacity"; `OD-6` (`owner-decisions.md:17`) |
| Each user's control plane runs on their laptop, loopback only, one per user | built | `control-plane/cmd/api/config.go:73` |
| A worktree is a lane clone; its work is fetched into **that user's** project repo as `refs/panacea/lanes/<worktree-id>` | planned (phase 3 tasks 03-07; paused for R4) | `wt/phase-3/…/2026-09-23-phase-3-git-convergence.md:28-29`, `:101-104` |
| The lane index and the land **queue order** are persisted in the local control plane | planned | same plan `:118-120`, `:183-186` |
| Lands are local `update-ref` with compare-and-swap under a per-project lock keyed by the **local** project id | planned | same plan `:110-115` |
| A teammate's work arrives **only after they push**: the project's `<branch>@{upstream}` is copied into each clone as `refs/remotes/upstream/<branch>`; "Bring in" merges it | planned (tasks 08-09) | same plan `:205-207`; `task-08.md:262` ("a teammate's push") |
| A push the remote refuses is `push_rejected`: "Fetch, then land or merge them in a worktree" | planned (task 20) | `task-04.md:118`; review `:274` |
| Cloud machines: the broker labels every object with owner = `tid`+`oid` and **filters every read by it**; one namespace, NetworkPolicy so **no sandbox reaches another** | planned (phase 8) | `wt/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:133` (P8-D4), `:134` (P8-D5), P8-D12 in the same table |
| Both teammates' local projects resolve to **one** Hub project through the remote's `RepoKey` | planned (task 35) | ADR-H1 `:408`; `task-35.md:5-12` |

**What is missing:**
1. **No shared lane namespace.** A lane is `refs/panacea/lanes/<worktree-id>` in one person's
   project `.git`; `w-…` ids are local (ADR-H1 `:419-421` lists `w-` among the control plane's
   local prefixes). A teammate's unpushed lane is invisible to me by construction.
2. **The forecast and the queue are per machine.** `merge-tree` runs only over lanes in my repo
   (plan `:101-104`), so two teammates' agents editing `limits.ts` on the same branch are forecast
   clean on both machines until one pushes and the other gets `push_rejected`.
3. **The product branch has two landers.** Each control plane lands with CAS against **its own**
   ref; the remote is the only arbiter, after the fact. No plan names who lands for a shared branch.
4. **The broker, correctly, cannot help.** P8-D4 filters every read by owner, so any cross-user
   view of sessions must come from a record the owner *publishes*, not from the broker listing
   another user's pods.

### 2.4 C4 — agents know what the other agents are doing: **absent; one reusable channel is planned for the canvas**

`PROGRAM.md` promised phase 3 *"file-claim coordination between concurrent agents"*
(`docs/PROGRAM.md:92`). Phase 3's scoping **cut it**: *"No file claim is declared or enforced in
v1. Claims are observed from git and forecast with `merge-tree`"* (A9,
`docs/superpowers/planning/2026-09-23-phases-4-5-7/scope-and-risk-phase-3-git-convergence.md:191`),
deferred to *"a later phase, if an agent channel exists"* (`:287`), because *"no file claim can be
enforced … without wrapping the CLI, which phase 2 forbids"* (`:43`).

| Piece that exists or is planned | What an agent can learn from it | Status | Cite |
|---|---|---|---|
| Observed claims: each worktree's changed paths, from git | nothing — it is a **cockpit** view for the human | planned (phase 3 task 07) | `task-07.md:1` |
| Chained `merge-tree` forecast; uncommitted overlap labelled "both edit — not forecast" | nothing directly; the human sees it | planned (tasks 08, 16) | `task-16.md:247-250`; scope-and-risk R9 `:399` |
| Shared-worktree warning 3-D2: two agents in one worktree allowed, warned, one click to a new worktree | nothing; it warns the human | planned (tasks 06, 19, 22) | plan `:167`; scope-and-risk `:38-43` |
| `Panacea-Session: <id>` trailer on every agent commit | after a fetch, `git log` names which **session** wrote a commit, not what it is doing now | planned (task 05) | plan `:26-27`, `:171` (3-D6) |
| The managed block in `CLAUDE.md`, rendered by phase 4's Materialiser **before the PTY starts** | the brief, at session start, once | planned (phase 4) | `wt/phase-4/docs/superpowers/plans/2026-09-18-phase-4-discover.md:42`, `:54`; ADR-H1 `:90-91` |
| A loopback MCP listener **inside each sandbox**, run by `sandboxd`, forwarded over a channel the control plane opens; the control plane stamps the session id; *"the agent needs no token: the channel is the credential"* | anything the control plane chooses to serve — **the one planned agent-facing channel** | planned for the canvas only (phase 5 OQ-4) | `wt/phase-5/docs/superpowers/planning/phase-5/paper-design-cleanroom-spec.md:492-512` |
| Canvas **claims**: an agent's first write into an artboard leases it; another agent's write fails with `claimed` naming the holder; humans never blocked | the one **declared, enforced** claim design in the repo, scoped to artboards | planned (phase 5) | same spec `:426-432` |
| Canvas MCP `set_status`, `get_comments`, `get_feedback` | an agent broadcasts a status line; reads human notes | named (phase 5 survey) | `docs/superpowers/planning/2026-09-23-phases-4-5-7/survey-phase-5-design.md:139` |
| Transcript adapter (`OD-8`), pinned, read-only, canary-tested | the control plane can derive "what the agent last did" from the transcript file, never from PTY bytes | planned (phase 3 task 17, 3-D7 b) | plan `:163` |
| The ranked queue gives a *"bonus for agents blocking other agents"*; the v3 coordination banner becomes a *"standing collision forecast"* | a UX intent for the human, no mechanism | named | `docs/AGENTIC-UX.md:71`, `:227` |

**What is missing:** (a) any tool or file through which an agent **asks** "who else is working on
this repo, on what"; (b) any record of an agent's **intent** (a declared focus or path claim) as
opposed to its **effects** (git changes); (c) anything that crosses users — every piece above lives
in one control plane; (d) any refresh after start — the `CLAUDE.md` block is rendered once, before
the PTY (`phase-4-discover.md:42`), so an agent started at 09:00 knows nothing of an agent started
at 09:05. **No agent-to-agent network path may exist:** P8-D12 denies every sandbox's egress to
*"every other sandbox"* (`wt/env/…/phase-8-own-environment.md`, P8-D12 row), and the phase 5
channel is host-reaches-in only (`paper-design-cleanroom-spec.md:502-506`). Coordination therefore
has to be **mediated** by the control plane and, across users, by the Hub.

## 3. The assumptions in the current plans that block the ask

Each is a correct choice for a single user. Each has to be named, because a design that keeps it
silently cannot deliver C2-C4.

| # | Assumption | Where it is written | Blocks |
|---|---|---|---|
| A1 | **Sessions are one machine's facts.** The registry is `sessions.json` in the user's state dir; the Hub never sees sessions, machines, worktrees or PTYs | `registry.go:56-62`; ADR-H1 `:88`, `:465` | C2, C4 |
| A2 | **A session has no owner.** `registry.Session` carries no user; on one laptop the user is implied | `session.go:42-104` | C2 (whose agent is this?) |
| A3 | **"Focus" is a title and an opening prompt.** Nothing records what an agent is working on now; the only live signal is `LastOutputAt` | `session.go:44`, `:54`, `:99` | C2, C4 |
| A4 | **The control plane is per user, loopback-only, and is the only Hub caller.** Nothing reaches another user's control plane, by design | `config.go:73`, `:354`; ADR-H1 `:439-443` | C2 (no teammate can ask my control plane anything) |
| A5 | **Lanes, the queue and the forecast live in one user's project `.git`.** Ids are local (`w-`, `p-` path hash); the forecast runs over my lanes only | phase 3 plan `:28-29`, `:101-104`, `:118-120`, `:183-186`; ADR-H1 `:121-125`, `:419-421` | C3, C4 |
| A6 | **A teammate exists only after a push.** Phase 3's only teammate paths are `refs/remotes/upstream/<branch>`, Bring in, and `push_rejected` | phase 3 plan `:205-207`; `task-04.md:118`; `task-08.md:262` | C3 (collisions found at push time) |
| A7 | **The change feed carries records, not activity**, and the Hub persists nothing about running work | ADR-H1 `:478-483`; contract route 43 `:605` | C2 |
| A8 | **Agent context is set once, at start.** The `CLAUDE.md` managed block renders before the PTY starts; nothing refreshes it | `phase-4-discover.md:42`; ADR-H1 `:90-91` | C4 |
| A9 | **Claims are observed, never declared.** No agent channel was assumed to exist | scope-and-risk phase 3 `:191`, `:287` | C4 |
| A10 | **The only agent-facing channel is scoped to the canvas.** Phase 5's in-sandbox MCP listener is designed for canvas tools, and *"there is no MCP wiring in the harness package today"* | `paper-design-cleanroom-spec.md:492-512` | C4 |
| A11 | **Nothing Hub-derived may touch the user's disk.** A5 widened: *"neither the renderer nor the control plane persists Hub data to disk"*, for erasure (G6) and P4 | ADR-H1 `:467-474` | C4, if a teammate's activity is written into a file in my clone for my agent to read |

**A11 is the subtle one.** The obvious way to tell an agent about teammates' agents is a file in
its clone (a `CLAUDE.md` section, a `.panacea/team.md`). That file is on the user's disk and in a
sandbox-writable directory, and it would hold Hub data (a teammate's name and focus). It collides
with A5's widening and with erasure. A pull channel (an MCP tool the agent calls, answered from the
control plane's memory) does not.

## 4. Constraints any collaboration design must respect

### 4.1 Phase 3 ruling R4 — the host never runs git where a sandbox can write

Source: `wt/phase-3/.superpowers/sdd/p3/orchestrator-rulings.md:3-65` (added 21:30, 2026-09-24).

- **No sandbox mounts the project repository**; every session runs in a lane clone (`:14-17`).
- **Host git never runs with an agent-writable directory** as repo, `--git-dir`, `-C`, work tree or
  alternate (`:18-21`). `Scope: Clone` is deleted.
- **Lane git runs inside the sandbox through `sandboxd`**, a git-runner RPC on its authenticated
  channel, returning `status --porcelain=v2 -z`, `rev-parse`/`for-each-ref`, and `git bundle create`
  (`:22-28`). Lane work crosses to the host **as a bundle, which is data** (`:29-36`); updates into a
  lane go the other way as a host-written bundle (`:40-42`).
- It names phase 8's `machined` (P8-D7) as the same seam (`:27-28`).

**What R4 means for collaboration:**
1. Anything that tells a teammate what my agent is changing comes from the `sandboxd` runner's
   structured results, or from bundles fetched into my host-private project — never from the host
   reading a lane.
2. A teammate's work reaching **my** agent's lane must arrive as a bundle written by my host from
   my project repository. The trust root is each user's host-private project `.git`.
3. `sandboxd` is already the place that answers "what is this lane doing" in structured form. It is
   the natural producer of an agent's **observed** activity, and phase 5's in-sandbox MCP listener
   (`paper-design-cleanroom-spec.md:502`) is the natural consumer channel for **declared** activity.
   Both are `sandboxd`, not the CLI, so `OD-8` and phase 2's no-wrapping rule are not touched.

### 4.2 Phase 8 — the broker (P8-D4) and the machine agent (P8-D7)

Source: `wt/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md`, decision table
`:128-145`.

| Decision | Constraint on collaboration | Cite |
|---|---|---|
| **P8-D4** broker: the only holder of Kubernetes credentials; validates an Entra token (`Sandboxes.Access`); owner = `tid`+`oid`; **labels every object and filters every read by owner** | a teammate can never list, attach to or read my sandboxes through the broker. Cross-user visibility must be a **published record**, not a broker read. Do not propose loosening this | `:133`; OQ-1 `:119` |
| **P8-D5** one namespace, users separated by owner labels, NetworkPolicy and per-owner limits | same | `:134` |
| **P8-D6** a cloud machine is one disk plus one machine pod, per user | a "shared machine" for a team does not exist; each member brings their own | `:135` |
| **P8-D7** `machined` serves the catalog and the git runner inside the machine pod, behind a per-machine bearer token; only the broker reaches it; no `pods/exec` | the cloud twin of R4's `sandboxd` runner. Lane facts for cloud agents come from `machined`, via the broker, to **the owner's** control plane only | `:136` |
| **P8-D12** sandbox egress denied to the pod and service CIDRs, the VNet and **every other sandbox**; ingress only from the broker | agents cannot talk to each other or, over the cluster network, to the Hub. Whether a sandbox could reach the Hub's public ingress via the NAT is **UNVERIFIED**, and moot: a sandbox holds no user token | `:141` |
| **S8.2** the laptop never holds a cluster credential | the desktop control plane stays the owner's single point of contact with their fleet | `:658` |

### 4.3 Owner decisions and program rules

| Rule | Constraint | Cite |
|---|---|---|
| `OD-2` | one clone per worktree; work fetched back under `refs/panacea/lanes/…` | `owner-decisions.md:13` |
| `OD-3` / `OD-5` | shared state lives server-side, in our own environment, and the orchestrator chooses its shape. The Hub is the only shared server planned | `:14`, `:16`; ADR-H1 `:50-57` |
| `OD-6` | Entra PKCE; tokens in Windows Credential Manager; *"Users are always themselves; nothing is pooled"* | `:17` |
| `OD-8` | a transcript may be read through a pinned, canary-tested adapter; nothing may require it; nothing reads PTY bytes | `:19`; `CLAUDE.md` "Harness records" |
| No pooling | each teammate's agents run on **that teammate's** subscriptions; a shared project must never let my cockpit start an agent on your capacity | `CLAUDE.md` "AI capacity" |
| Kata | per-pod microVM; an agent's blast radius is its own clone | `CLAUDE.md` "Isolation"; `OD-2` consequence `:13` |
| Hub: principal from the token | *"no request body carries an actor, an approver or a recorder field"*; non-members get 404 | ADR-H1 `:433-435` |
| Hub: the control plane is the only caller | the renderer and sandboxes never call the Hub; requests with an `Origin` header are refused | ADR-H1 `:439-443` |
| Hub data never on the user's disk | see A11 | ADR-H1 `:467-474` |
| PTY bytes stay local | *"The Hub never sees a terminal byte"*; attach tokens are never on the REST surface | ADR-H1 `:88`; `session.go:35`, `:95` |
| Sandboxes hold no git credential; push runs on the host with the user's own credential | a shared lane cannot be "pushed by the agent" | phase 3 plan `:316-318` |

## 5. Reuse map for the design (05-design)

What a collaboration design can stand on, and what it has to add. "Reuse" means an existing
planned seam, extended; nothing here is built except where marked.

| Need | Reuse | Add |
|---|---|---|
| Who is on the project (C1) | Hub teams, membership, invites, project ↔ `RepoKey`, share popover (tasks 15, 35, 42) | nothing |
| Transport to teammates' cockpits (C2) | Hub change feed G3, polled every 5 s; SSE is "v1.1 and additive" (ADR-H1 `:478-488`) | new event kinds for session activity |
| A shared record of a running agent (C2) | the principal-from-token rule; `session_link` shows the precedent of a session id visible to teammates with `createdBy` (contract `:941-942`) | a Hub **session/activity record** published by the owner's control plane: owner, project, branch **ref**, status, declared focus, changed paths. This reverses ADR-H1 `:88` for metadata only; PTY bytes and attach tokens stay local |
| What each agent is changing (C2, C4) | phase 3 observed claims (task 07) via R4's `sandboxd` runner / P8-D7 `machined` | publish the path set, not the diff, to the Hub |
| A teammate's unpushed work (C2, C3) | lanes as bundles (R4 `:29-36`) | either publish lane refs to the shared remote under a `refs/panacea/…` namespace, or accept "visible at push" — a decision, not an inventory fact |
| Declared intent and claims (C4) | phase 5's artboard lease model (`paper-design-cleanroom-spec.md:426-432`) | the same lease shape for **paths**, advisory for agents, never blocking a human |
| An agent asking "who else is here" (C4) | phase 5's in-sandbox MCP listener, host-reaches-in, session stamped by the control plane (`:492-512`) | non-canvas tools on the same listener, answered from the control plane's in-memory copy of the Hub feed (keeps A11) |
| Refreshing an agent mid-run (C4) | none — the `CLAUDE.md` block is start-only | pull via the MCP tool; no PTY injection (`OD-8`) |
| Cross-user forecast (C3, C4) | phase 3's chained `merge-tree` (tasks 08, 16) | run it against published teammate lane refs, once they are fetchable |

## 6. What is unverified, and what this inventory did not cover

- **Build state, as of 2026-09-24:** the Hub has only `internal/hub/hubapi` (tasks 01-02, `wt/hub`
  head `94d6625`). Phase 3 has **no** `lanes`, `converge` or `gitx` package in `wt/phase-3`
  (last code commit `4f6d698`) and is paused for R4 (`docs/superpowers/RESUME.md`, "Phase 3 is
  paused"). Phase 5 is at its canvas spike, task 01 (`wt/phase-5` head `ec303f0`); no in-sandbox MCP
  listener exists. Phase 8 is at sandbox pod hardening, task 14 (`wt/env` head `692df20`).
- **Line numbers in live trees drift.** `wt/phase-3`, `wt/hub` and `wt/env` were being edited while
  this was read.
- **UNVERIFIED:** whether a sandbox on AKS could reach the Hub's public ingress through the NAT
  (P8-D12 allows internet egress). The design should not depend on either answer.
- **UNVERIFIED:** whether Claude Code reloads an MCP server's tool list or `CLAUDE.md` mid-session;
  §5's "pull via an MCP tool" assumes only that a tool call returns fresh data.
- **Not read:** phase 7's plan beyond its scope-and-risk brief; phase 4's plan beyond the
  Materialiser lines; `prototype/phase-3-convergence.html` in detail (the v3 coordination banner is
  cited through `docs/AGENTIC-UX.md:227`). External prior art is 02 and 03 in this directory.
