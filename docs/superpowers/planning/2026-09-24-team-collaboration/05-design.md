# 05 — Design: team mode, centrally managed (OD-14 to OD-17)

**Status:** design, 2026-09-24, **rewritten after OD-14 to OD-17**. The earlier draft of this file
(commits `28a65d1` to `7b094c5`) assumed each teammate's agents ran on that teammate's laptop and
that only facts met in the Hub. OD-14 reverses that: team agents run centrally. That draft is
superseded wholesale. What survives from it and from `01`–`04` (the fences, the conflict tiers, the
Hub's team model) is carried in here and marked. **Nothing here is built.** Repo claims are
`path:line`. `01`–`04` are the research files in this folder, cited by section. Line numbers into the
`hub`, `phase-3` and `env` worktrees may drift, because those trees are being edited live.
**UNVERIFIED** marks every claim not checked against a source or a run. No web source was re-read
for this file; external claims carry the read dates given in `02`–`04`.

**The ask** (owner, verbatim): *"share a project with my team, and everyone can not only see the
work I am doing on it, the agents I have running, and what their focus is, but spin up their own
agents in the same repo and the agents understand what each other are doing and what they are
working on."*

**The owner's direction** (`2026-09-23-owner-decisions.md:116-121`): *"Aren't we managing all of the
agents centrally, rather than them being on each machine? I am thinking like a Word Document on
Sharepoint being edited by multiple people, where each agent can see other agents doing work,
understand what the agent is currently working on and the task they have and the reason they are
updating that document, they are able to communicate and understand each other."*

| Key | Capability | Before OD-14 (`01` §2) | In this design |
|---|---|---|---|
| **C1** | share a project with a team | planned in the Hub, not built | the Hub plan, unchanged, plus a team project hosted by Fleet (§2, §3) |
| **C2** | everyone sees each member's work and each agent's focus | absent by design (`ADR-H1:88`) | agents see every task card; humans see a calm team strip (§5, §9) |
| **C3** | each member runs their own agents on the same repo | per machine; fleets never met | every member's agents run in one managed Fleet, each on its owner's subscription (§4, §10) |
| **C4** | agents understand each other | absent; phase 3 cut claims (A9) | task cards, leases and an orchestrator protocol (§5, §6) |

## 0. The design in ten lines

1. **One managed control plane per org, called Fleet,** runs every team agent as a managed session in
   a Kata sandbox in our environment. The desktop becomes a client of Fleet. Laptop Docker stays as
   the solo, offline and development mode (OD-14).
2. **Every user has one Orchestrator per team project.** It is an unmodified Claude Code session on
   that user's own subscription, in its own sandbox. It plans the work and spawns and directs only
   that user's **workers** (OD-16).
3. **Every agent has a task card**: its task; its reason, which is a link to the brief section the
   task serves (the provenance thread); its status; the files it is touching, as observed by the
   platform; and the leases it holds. Every agent on the project can read every card (OD-15).
4. **Agents reach the platform through one channel that survives the KPMG policy:** a `panacea`
   command inside each sandbox, answered by `sandboxd` from Fleet. It needs no MCP and no hooks. An
   MCP front can be added later for a customer whose admin allowlists it (§5.2).
5. **Orchestrators are the only cross-user speakers.** They talk through a typed protocol on Fleet:
   announce, ask, answer, propose, accept, reject, yield. Fleet detects conflicts itself and opens a
   **conflict thread** between the orchestrators involved (§6).
6. **Each Orchestrator escalates only to its own human,** either to **inform** (a line in the
   RETURN handover) or to **ask for a decision** (an ADJUDICATE item), under a policy that human
   sets in advance. No human steers another human's agent (OD-16, §6.5).
7. **Coordination text is untrusted input.** Facts are observed by the platform. Agent text is typed,
   bounded, attributed and quoted. A teammate's free text reaches only orchestrators, and an
   orchestrator that has read it may give its workers only typed directives without asking its
   human. The sandbox and the verb set enforce this, not the prompt (§7).
8. **Conflicts are predicted centrally.** Paths and hunks come from observed activity. Exact
   `merge-tree` forecasts run over a per-project **lane mirror** that Fleet holds, so no unfinished
   lane has to be pushed to the customer's remote. Leases are soft; the reviewed land is the
   backstop (§8).
9. **Humans get a calm team strip:** who is on the repo, one line each, and the files touched.
   Conflicts appear only when an Orchestrator escalates. There is no teammate terminal by default
   (OD-15, §9).
10. **The minimum lovable slice (TM1)** is about 30 focused days after the current wave. It
    re-scopes the Hub, phase 8 and phase 3 in the exact ways listed in §11 (OD-17, §12).

## 1. Principles, each with its source

| # | Principle | Why it binds | Source |
|---|---|---|---|
| P1 | **Team agents run where the platform can see them**: in Fleet, in Kata sandboxes | the owner's model is one shared document, not N laptops | OD-14 (`owner-decisions.md:125`) |
| P2 | **One principal per human, from the token.** Fleet acts for a user only under a delegation that the user's own token created and the Hub records | a teammate, or Fleet itself, cannot write "Dylan's agent is doing X" | `ADR-H1:372`, `:434-435` (via `04` §2.1) |
| P3 | **Only a user and that user's Orchestrator direct that user's agents** | Consumer Terms §2: *"You also may not make your Account available to anyone else"* | OD-16; `04` §2.2 (read 2026-09-24) |
| P4 | **Every token is paid by the agent's owner.** A reader pays to read a peer's message, so senders are rate-limited | nothing is pooled; teammates cannot drain each other's quota | OD-16; `docs/PROGRAM.md:17`; `03` §7 D2 |
| P5 | **Observed facts beat narration.** Files, commits and status come from git and `sandboxd`; anything an agent says about itself is labelled agent text | agents misreport; structured shared state beats chat | `03` §2 I2–I3; `03` §5.2 (MetaGPT, MAST) |
| P6 | **Agent text is data, never authority.** This replaces `04`'s F1 (*"agents never publish"*), which OD-16 overturns by making orchestrators speak | an agent-to-agent channel is an injection channel | `04` §5.2–5.3; OD-16 |
| P7 | **Awareness is advisory; the land queue decides** | hard pre-write admission serialised 96.7% of runs | `03` §5.2 (arXiv 2608.00947) |
| P8 | **No hook or MCP dependency for delivery** | the managed policy sets `allowedMcpServers: []` and strips settings hooks | `03` §1.1; `04` §5.3 |
| P9 | **Nothing requires the transcript adapter or reads PTY bytes** | OD-8 | `owner-decisions.md:19`, `:58-79` |
| P10 | **The Hub still never sees a terminal byte.** Fleet carries PTY streams only to the session owner's own clients | the Hub's erasure and audit story stays intact | `ADR-H1:88` (via `01` §2.2) |

## 2. Architecture: one managed Fleet, the desktop as a client

### 2.1 The pieces

| Piece | Where | Role in team mode | Status |
|---|---|---|---|
| **Cockpit** (Tauri/React) | the user's desktop | a client. Signs in with Entra PKCE (OD-6). For a team project it talks to **Fleet**; for a solo project, to the local control plane as today | phase 2, nearly done |
| **Fleet** (`control-plane/cmd/fleetd`, new) | our environment (OD-5) | the team project's control plane: sessions, the Orchestrator turn loop, task cards, leases, protocol threads, presence, forecasts. Built from phase 2's `registry`, `harness` and supervisor packages, made multi-user | **new** (§11.4) |
| **Hub** (Phase H) | our environment | the system of record: teams, members, projects, briefs, approvals, the provenance ledger, plus the durable coordination records in §3 | planned; `hubapi` only (`01` gaps) |
| **Broker** (`cmd/broker`, P8-D4) | in-cluster | still the only holder of Kubernetes credentials for sandboxes; every object labelled with its owner | phase 8, in flight |
| **`machined`** (P8-D7) | each user's machine pod | that user's worktree clones (OD-2) and git runner, on that user's machine disk (P8-D6) | phase 8, in flight |
| **`sandboxd`** | each sandbox pod | runs the harness PTY; gains the server end of the `panacea` command and the read-only team file (§5) | phase 1–2, extended |
| **Lane mirror** (new) | one bare repo per team project, on a project disk | holds every member's lane heads for cross-user forecasts; git runs there only through Fleet's hardened runner (§8.2) | **new** |

Fleet is **one deployment per environment**, multi-tenant. Every row and every sandbox is keyed by
org (`tid`) and owner (`tid` + `oid`), exactly as the broker already keys sandboxes
(`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:133-134`).
A project is a row, not a deployment. Sharding by project comes later if load demands it.

### 2.2 Why Fleet is a separate service, not a bigger Hub

1. **The Hub's invariant stays true.** `ADR-H1:88` says the Hub never sees a terminal byte (`01`
   §2.2). Fleet carries PTY streams, so it must not be the Hub. Erasure and audit stay simple.
2. **They scale differently.** Fleet scales with running sessions; the Hub scales with records.
3. **They fail differently.** If Fleet goes down, agents pause and no record is lost. If the Hub goes
   down, Fleet keeps sessions running and queues its writes for up to 10 minutes (proposed), then
   pauses every Orchestrator so no negotiation proceeds unrecorded.
4. **Rejected: the earlier draft's design,** where each laptop ran its own fleet and the Hub joined
   the facts. OD-14 overturned it.

### 2.3 Solo and team projects

A project is either **solo**, on the local control plane (laptop Docker, or phase 8's cloud machine
driven from the laptop), or **team**, hosted by Fleet. **Share** turns a solo project into a team
project (§10.1). After that, every new session on it launches in Fleet. A session left running
locally keeps working, but it is labelled *outside the team*: it holds no leases and appears to
others only once its work is pushed. The label is honest, and nothing is silently missed.

### 2.4 Identity, and working while the human is away

1. **Desktop to Fleet:** an Entra access token with Fleet's API as audience. Fleet derives the owner
   as `tid` + `oid`, the same way the broker does (P8-D4, `phase-8-own-environment.md:133`).
2. **Launch creates a delegation.** While the user's token is present, Fleet uses the Entra
   on-behalf-of flow to call the Hub and create a `delegation` record: owner, project, session
   class (`orchestrator` or `worker`), the Hub routes it may call, and an expiry of at most 7 days,
   renewed at each sign-in. Afterwards Fleet calls the Hub as its own workload identity and names
   the delegation. The Hub checks that the delegation exists, is live and covers the route, then
   records **actor = the owner, via = Fleet and the delegation id**.
3. **Why not keep refresh tokens server-side:** a store of user refresh tokens is a pool of user
   credentials. A delegation is narrow (one project, listed routes), audited and revocable.
4. **Revocation.** Removing a member revokes that member's delegations for the project. Fleet sees
   the revocation on the change feed and, within 60 seconds, stops their sessions there, withdraws
   their leases and marks their cards *ended: access removed*. Freshness is a security property
   (`04` §5.3).

This changes one Hub rule: today the laptop's control plane is the Hub's only caller, always with
the user's own token (`01` §4.3). §11.1 records the change.

### 2.5 Each user's own subscription and own git credential, inside Fleet

- **Claude.** The user signs in with `/login` once, in a sign-in session on their own machine. The
  CLI's credential lives on that user's machine disk (P8-D6) and is mounted only into that user's
  sandboxes. Fleet never reads, copies or holds it. Per-user rotation (`docs/PROGRAM.md:17`) picks
  among that user's own credential slots on the same disk. The metered API backstop bills that
  user's cost centre, never a teammate's. **UNVERIFIED:** whether one CLI credential can be used by
  several of the user's sandboxes at once without its token refresh racing; and whether the
  tenant's managed policy applies inside the sandbox after `/login` (`03` C3; OD-12).
- **Git.** `04`'s A4 still holds: no git credential enters any sandbox. Fetch and push run in the
  owner's `machined` git runner (P8-D7), with the **lane owner's** credential. For Azure DevOps the
  source is an Entra on-behalf-of token for the Azure DevOps resource (**UNVERIFIED** for git over
  HTTPS). For GitHub, the desktop hands over a short-lived token from Windows Credential Manager
  (OD-6) for each push, held in memory only. **Push and PR stay human actions** (`04` F7). An
  Orchestrator may propose one; its human decides.

## 3. The data model

### 3.1 What the Hub already plans, unchanged (C1)

`ADR-H1` already defines `org`, `user` (keyed `(tid, oid)`), `team`, `membership` (`admin` ·
`member` · `viewer`), `invite` (same-org email), `project` (owned by one team), `project remote`
(keyed by phase 3's `githost.RepoKey`) and `brief`
(`.claude/worktrees/hub/docs/superpowers/adr/ADR-H1-hub-architecture.md:402-409`). A project's
access is exactly its team's membership (`hub-contract.md:333`, via `04` §2.1). Team mode uses all
of it as planned and adds **no new role**.

### 3.2 The placement rule

**If losing it would lose a decision or an accountability fact, it is a Hub record. If it is a fact
about now, it is Fleet state.** Hub records are revisioned, write one metadata-only audit event each
(G3, `ADR-H1:252`) and reach clients through the existing change feed. Fleet state is in memory with
a snapshot for restart. It is never audited row by row, and it is served by Fleet's own routes.

### 3.3 What team mode adds

`text·A` marks a field an **agent** wrote; `text·H` a field a **human** typed. Every free-text field
is bounded, hygiene-checked (§7.2) and carries its author class, so every reader knows which it has.

| Record | Id | Lives in | Fields (all bounded) | Written by |
|---|---|---|---|---|
| **delegation** | `dl-…` | Hub | owner; project; class; allowed routes; `expiresAt`; `revokedAt` | the owner's token, at launch (§2.4) |
| **task** | `tk-…` | Hub | owner; orchestrator `ag-…`; **title** ≤ 140 (`text·H` if the user typed it, else `text·A`); **reason**: `briefRef` (a brief section id, deterministic) plus a why-line ≤ 200 `text·A`; intended scope (≤ 20 repo-relative globs); state `planned`·`active`·`blocked`·`done`·`abandoned` | the owner's Orchestrator, through `panacea task` |
| **agent** | `ag-…` | Fleet (status); Hub (start, end, role) | owner; role `orchestrator`·`worker`; parent orchestrator; harness id and pinned version; task `tk-…`; lane ref; status `starting`·`working`·`waiting`·`idle`·`ended`; `waitingOn` `human`·`lease`·`peer`; `exitClass` | Fleet, from `sandboxd` and the supervisor; extends phase 2's four statuses (`control-plane/internal/registry/session.go:14-31`) |
| **activity** | on the agent | Fleet, ephemeral, 45 s TTL | files **read** and **edited** in the last 10 min (from `sandboxd`'s watcher on the worktree); dirty paths; dirty hunks (line ranges on a base blob); last commit sha and changed paths; `lastActiveAt` | `sandboxd`, observed. Nothing here is agent-written |
| **lease** | `ls-…` | Fleet (live table, authoritative); Hub (each state change) | holder `ag-…`; globs (≤ 20); mode `intent`·`hold`; `expiresAt` (30 min, renewed while the holder works); state `requested`·`granted`·`contested`·`yielded`·`released`·`expired` | Fleet, on `panacea lease` calls and observed edits (§6.3) |
| **conflict** | `cf-…` | Hub | agent pair; tier `P`·`H`·`M`·`S`; paths and line ranges; basis shas; state `raised`·`negotiating`·`resolved`·`escalated`·`cleared` | Fleet's forecaster only (§8) |
| **thread** | `th-…` | Hub | participants (orchestrator ids); subject (conflict, lease or free); state `open`·`resolved`·`escalated`·`expired`; round count; deadline | Fleet, when a conflict opens or an Orchestrator starts one |
| **message** | `ms-…` | Hub, append-only | thread; sender `ag-…` and its owner; **kind** (§6.2); typed fields per kind; text ≤ 600 `text·A`; `afterPeerInput` flag (§7.3) | an Orchestrator, through `panacea peer` |
| **escalation** | `es-…` | Hub | owner (a human); from orchestrator; **kind `inform`·`decide`**; thread; question ≤ 300 `text·A`; up to 4 options, each with its consequence; default and deadline; answer `text·H`; `answeredAt` | the owner's Orchestrator; answered only by that human |
| **note** | `nt-…` | Hub, immutable | author (from token); target project, task or person; text ≤ 1,000 `text·H` | any member, as themselves |
| **presence** | per user | Fleet, ephemeral | online in the cockpit; project open | the user's cockpit |
| **team policy** | on the project | Hub | allowed inference routes; required account class (`04` §3.3); escalation defaults (§6.5); message budgets (§6.6) | a team `admin` |

Five rules apply to every row:

1. **The owner comes from the token or the delegation, and only the owner's own agents write the
   owner's rows.** Fleet refuses a `panacea task` call from a sandbox whose owner is not the task's
   owner. The channel is the credential: `sandboxd` stamps the calling session, the pattern phase 5
   planned for its canvas listener
   (`.claude/worktrees/phase-5/docs/superpowers/planning/phase-5/paper-design-cleanroom-spec.md:492-512`,
   via `01` §2.4).
2. **No local session id goes to the Hub.** It means something only on the machine that made it
   (`hub-contract.md:941-942`, via `01` §2.2). Records carry `ag-…` ids.
3. **Nothing names a host path, config key or URL.** Paths are repo-relative globs that the Hub
   validates. This closes the X-1 class one hop out (`04` §8.2).
4. **Activity is observed, never reported.** An agent cannot claim to be editing a file it is not
   editing, and cannot hide one it is. The `sandboxd` watcher runs outside the agent's process tree
   (**UNVERIFIED** that inotify inside a Kata guest sees every write through the sub-path mount).
5. **Nobody writes another person's task, lease, message or escalation**, whatever their role.
