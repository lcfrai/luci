# 05 — Design: team mode, four capabilities end to end

**Status:** design, 2026-09-24. Nothing here is built. It synthesises the four research files in
this folder: `01-internal-inventory.md`, `02-product-prior-art.md`, `03-techniques.md` and
`04-enterprise-constraints.md`. Repo claims are `path:line`; line numbers into the `hub`,
`phase-3` and `env` worktrees are at the heads those files record and may drift. **UNVERIFIED**
marks every claim not checked against a source or a run. No web source was re-read for this file;
external claims are carried from 02–04 with their original read dates.

**The ask** (owner, verbatim): *"share a project with my team, and everyone can not only see the
work I am doing on it, the agents I have running, and what their focus is, but spin up their own
agents in the same repo and the agents understand what each other are doing and what they are
working on."*

Four capabilities, used as section keys throughout:

| Key | Capability | Today (from `01` §2) |
|---|---|---|
| **C1** | share a project with a team | planned in full in the Hub, not built |
| **C2** | everyone sees each member's work, agents and each agent's focus, live | absent, by design (`ADR-H1:88`) |
| **C3** | each member runs their own agents on the same repo, from their own machine | works per machine; fleets never joined |
| **C4** | every agent knows what the other agents are doing | absent; phase 3 cut file claims (A9) |

## 0. The design in eight lines

1. **Share the facts, not the machine.** The Hub gains three record kinds: an **agent card**, a
   **lane summary** and **coordination records** (focus, claim, note). It also gains one ephemeral
   channel, **presence**. It never holds a transcript, a terminal byte, source code or a credential.
2. **Every record is written by a control plane, as its user, from a human or from git.** Agents
   never publish. The network already enforces this: no sandbox can reach the Hub (`04` §5.1).
3. **A teammate's agents run on the teammate's machine, subscription and git credential.** They
   use their own clone and their own lanes, pushed to the shared remote under a per-user prefix.
4. **Agents learn about each other through a quoted, read-only team file,** written by `sandboxd`
   outside the worktree. `CLAUDE.md` holds one fixed pointer sentence. A templated push through
   Claude Code's inbox socket arrives later, behind a canary. It carries facts and no teammate text.
5. **Conflicts are forecast across machines in tiers**: paths and hunks at the Hub from metadata,
   exact `merge-tree` at each control plane from lanes on the remote. Claims are soft leases.
   Phase 3's reviewed land stays the backstop.
6. **Only the launcher steers.** A teammate may watch a terminal only with the owner's consent,
   given per session and time-boxed, through an outbound-only relay. That comes last.
7. **The minimum lovable slice (TM1)** is share, the team board, agent cards with focus, lane
   summaries, path-tier forecasts and the team file. Estimate: about 9–11 focused days after the
   Hub's H-min and phase 3 task 07 land (§11).
8. **Eight owner questions** are in §12, each with a recommendation.

## 1. Principles, each with its source

| # | Principle | Why it binds | Source |
|---|---|---|---|
| P1 | The Hub brokers **facts**, never compute, credentials, transcripts or terminal bytes | Forget must stay total; Hub makes no outbound call with user content | `04` §9.1 #1; `ADR-H1:88`, `:356-358` (via `04` §2.3) |
| P2 | Coordination records come from **humans or deterministic code**, never from an agent | cuts the agent→record edge of the injection chain | `04` §5.3 F1 |
| P3 | **One principal per human, from the token**; no body carries an actor | a teammate cannot write "Dylan's agent is doing X" | `ADR-H1:372`, `:434-435` (via `04` §2.1) |
| P4 | **Only the launcher steers** a session | Consumer Terms §2, *"You also may not make your Account available to anyone else"* | `04` §2.2 (anthropic.com/legal/consumer-terms, read 2026-09-24) |
| P5 | **No inbound path** to any laptop or sandbox; the Hub is the only rendezvous | stdin frames are code execution in my sandbox | `04` §5.1, §8.1 rank 4 |
| P6 | **Awareness is advisory**; the land queue decides | hard admission serialised 96.7% of runs | `03` §0 #5, §5 (arXiv 2608.00947) |
| P7 | **Nothing requires the transcript adapter** or reads PTY bytes | OD-8 | `owner-decisions.md:73-79` (via `03` K1–K2) |
| P8 | **No hook or MCP dependency** for delivery | the KPMG managed policy strips both | `03` §1.1; `04` §5.3 |

## 2. The data model (C1–C4)

### 2.1 What the Hub already plans, unchanged (C1)

`ADR-H1` already defines `org`, `user` (keyed `(tid, oid)`), `team`, `membership`
(`admin` · `member` · `viewer`), `invite` (same-org email), `project` (owned by one team),
`project remote` (keyed by phase 3's `githost.RepoKey`) and `brief`
(`.claude/worktrees/hub/docs/superpowers/adr/ADR-H1-hub-architecture.md:402-409`). *"A project's
access is exactly its team's membership"* (`hub-contract.md:333`, via `04` §2.1). Team mode uses
all of it as planned and adds **no new role**.

### 2.2 What team mode adds

Two storage classes, following `03` §4.1. **Durable** records are revisioned rows. Each write
appends one metadata-only audit event (G3, `ADR-H1:252`), so they reach teammates through the
existing change feed. **Ephemeral** state lives in memory with a TTL. It is never audited or
persisted, and it is served by its own route.

| Record | Id | Class | Fields (all bounded; F2) | Written by |
|---|---|---|---|---|
| **agent card** (`04` Z1) | `ag-…` | durable | project; **owner** (from token); `machineKind` `local`·`cloud`; harness id; **title** (human-typed at launch, ≤80 chars); brief key (`ContextRef`, `session.go:102-104`); branch ref; **lane ref**; status `starting`·`running`·`idle`·`exited` (`session.go:16-30`); `exitClass` enum; `startedAt`, `endedAt`; `promptShared` flag plus opening prompt **only if the owner opts in** (§12 Q1) | the owner's control plane |
| **lane head** | on the card | durable | merge base sha, head sha, ahead count, **committed paths** (≤500, else truncated with a count), last-commit time. The last commit **subject** is agent-written: it is kept for humans and flagged `agentText` | the owner's control plane, from `sandboxd`'s runner (R4 rule 3) |
| **focus** | `fx-…` | durable, revisioned | agent id; text ≤140; optional scope globs; `origin` `human`·`derived`. Derived default = title + brief section | the owner's control plane; text **typed or confirmed by the owner** (F1) |
| **claim** (soft lease) | `cl-…` | durable | agent id; repo-relative globs (≤20); `expiresAt` (default 2 h, renewed while the agent runs); `origin` | the owner's control plane, on the owner's action |
| **note** | `nt-…` | durable, immutable | author (from token); target: project, lane, or **person**; text ≤1,000; `kind` `note`·`ask` | any member, as themselves |
| **forecast** | `fc-…` | durable | lane pair; tier `P`·`H`·`M`; paths plus line ranges; basis shas; `raised`·`cleared` | the Hub (P, H); the owner's control plane for its own lanes (M) |
| **presence** | per user and machine | **ephemeral**, 45 s TTL | online; per agent: status, `lastActiveAt` (from `LastOutputAt`, a byte count, `session.go:22-23`), **dirty paths**, and later dirty hunks | the owner's control plane, `PUT` every ~15 s |
| **watch grant** (`04` Z3) | `wg-…` | durable | session owner; viewer; `expiresAt` ≤ 4 h | the session's owner only. **TM4** |
| **project team policy** | on the project | durable | allowed inference routes; required account class (commercial, or training off, attested); defaults for Q1 and Q2 | team `admin` |

Four rules apply to every row:

1. **The owner field comes from the token, and only its owner may write the row.** The Hub
   refuses a card, focus or claim write whose token principal is not the card's owner. That is
   `P3` applied to live work.
2. **The local session id never goes to the Hub.** It *"means something only on the creator's
   machine"* (`hub-contract.md:941-942`, via `01`). The card has its own `ag-…` id, and the
   control plane keeps the mapping in its registry (§8.1).
3. **Nothing names a host path, config key or URL.** Paths are repo-relative globs, validated by
   the Hub (F2). This closes the X-1 class one hop out (`04` §8.2 rank 9).
4. **Dirty paths are ephemeral; committed paths are durable.** Uncommitted work changes every
   few seconds. It would flood a hash-chained audit log with facts nobody needs to keep
   (`03` §4.1), so it rides presence. A commit is a fact worth a feed event.

### 2.3 Roles (Z4)

| Role | Reads | Writes |
|---|---|---|
| `viewer` | board, cards, focus, claims, notes, forecasts | nothing |
| `member` | the same | **own** cards, focus, claims, presence; notes as themselves; runs own agents on the project |
| `admin` | the same | plus membership, invites and the project team policy |
| `Hub.OrgAdmin` | audit only, no project content (`hub-contract.md:326-333`, via `04` §2.1) | erasure |

Nobody writes another person's card, focus or claim, whatever their role. Steering is not in
this table because it never touches the Hub (§5.3).
