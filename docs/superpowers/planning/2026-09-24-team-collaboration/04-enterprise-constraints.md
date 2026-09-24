# 04 — Enterprise and security constraints on team mode

**Research, 2026-09-24. Author: research agent 04 of the team-collaboration run (`RESUME.md:108-112`).**
Scope: what a regulated Australian enterprise (KPMG; ISM/IRAP posture) requires of "share a
project with my team, see each other's agents, run my own agents in the same repo, and have the
agents understand each other". This document constrains the design (`05-design`); it does not
choose one.

**The answer in five lines.**
1. Team mode is safe to build **only as a metadata-sharing system**: what is shared between people
   is *intent, claim, lane head, status and focus*, written by the control plane. Raw terminals and
   transcripts stay on the owner's machine by default, and are shown to a teammate only live, only
   on the owner's explicit per-session consent.
2. **No teammate's agent ever runs on, or reads from, another person's machine, sandbox or
   subscription.** "Spin up an agent in the shared project" means *on my machine, on my
   subscription, in my own lane clone of the shared remote*. The Hub brokers facts, never compute.
3. Coordination data is **untrusted input to every other agent**. It is structured, length-capped,
   schema-checked by the Hub, attributed to a human, and delivered to agents as quoted data in a
   file they read, never as instructions spliced into a prompt.
4. The Hub's existing model (team roles, 404-not-403, principal from token, hash-chained audit,
   Australia East) carries most of this. It lacks four things: a **steer** permission distinct
   from write, a **session-visibility** grant, **coordination records** as a new entity, and
   **audit events for viewing** a live stream.
5. The top risk is not a network attack. It is **cross-teammate prompt injection through shared
   coordination text** (§6, rank 1), followed by **secrets in a streamed terminal** (rank 2).

---

## 0. Sources, and how to read the citations

Repo claims cite `path:line`. Paths under `.claude/worktrees/<name>/` are the in-flight worktrees,
read at these heads on 2026-09-24:

| Tree | Head read | What it contributes |
|---|---|---|
| main (`feat/phase-2-cockpit`) | `ba65934` | owner decisions, `RESUME.md`, phase 2 control plane |
| `hub` | `94d6625` | `ADR-H1-hub-architecture.md`, `2026-09-23-hub-contract.md` |
| `phase-3` | `6811f0c` | scope-and-risk brief, orchestrator ruling R4 |

External claims cite a URL and the date it was read. Anything not checked is marked
**UNVERIFIED**. Nothing in this document was run against a live system: every control named here
is a design control, and its effectiveness is **unverified until built and tested**.

**Two names collide, so they are spelled out once.** "Phase 3 **ruling** R4" is the structural git
rule (`.claude/worktrees/phase-3/.superpowers/sdd/p3/orchestrator-rulings.md:3`). "Phase 3
**risk** R4" is the managed-policy-in-the-sandbox risk
(`.claude/worktrees/phase-3/docs/superpowers/planning/2026-09-23-phases-4-5-7/scope-and-risk-phase-3-git-convergence.md:389`).
Both bind team mode, differently (§7).

## 1. The threat actors team mode adds

Single-user phase 2 has one human and their own agents. Team mode adds four actors, and every
control below is aimed at one of them:

| Actor | What is new | Trusted for |
|---|---|---|
| **A1. A teammate** (same org, same team) | a second human who can read and write project state | their own work; not for my machine, my subscription or my secrets |
| **A2. A teammate's agent** | an LLM process, on the teammate's machine, that reads coordination data I wrote and writes coordination data I read | nothing. An agent is a confused deputy by default |
| **A3. A viewer** (read-only member, or an auditor) | a person who sees live work but must not change it | reading what the project shares; not steering |
| **A4. A departed or compromised member** | a membership that should have ended, or a token on a lost laptop | nothing after revocation |

The single-user product already treats **my own agent** as untrusted to the host (phase 3 ruling
R4's premise, `orchestrator-rulings.md:5-10`). Team mode's new fact is that one person's agent
becomes an **author of input to another person's agent**. That crosses a trust boundary the
single-user product never had.

## 2. Identity and authorisation

### 2.1 What the Hub already gives team mode

- **One principal per human, from the token.** `(tid, oid)`, never `sub` or
  `preferred_username` (`.claude/worktrees/hub/docs/superpowers/adr/ADR-H1-hub-architecture.md:372`).
  No request body carries an actor (`ADR-H1…:434-435`). Team mode inherits this unchanged: every
  intent, claim and status is attributed by the server, so a teammate cannot write "Dylan's agent
  is doing X".
- **Three team roles plus org admin**: `viewer`, `member`, `admin`, and `Hub.OrgAdmin`, which reads
  audit and erases but **reads no project content** (`hub-contract.md:326-333`).
- **"A project's access is exactly its team's membership. There are no per-project grants in v1"**
  (`hub-contract.md:333`). Non-members get 404, never 403 (`ADR-H1…:433-434`).
- **Membership is the Hub's, not Entra groups'** (`ADR-H1…:405`, rejected alternative §6.6 at
  `ADR-H1…:634`). Revocation is one row, effective on the next call.

### 2.2 What it lacks: steering is not writing

The matrix (`hub-contract.md:341-359`) has one axis, read versus write, over **shared objects**
(briefs, approvals, releases). Team mode adds **live sessions**, and a session has three
distinct powers the matrix cannot express:

| Power | What it is technically | Who should hold it |
|---|---|---|
| **See the roster** — who is running which agent, on which lane, with what stated focus, in what state | Hub rows written by each control plane (new entity, §2.4) | every team role, `viewer` up |
| **Watch the terminal** — live output of one session | a read-only relay of `0x00` stdout frames (`2026-09-18-phase-2-go-contract.md:2227-2229`) off the owner's machine | a named teammate, **only while the owner consents for that session** (§3.1) |
| **Steer** — type into, interrupt, restart, fork or Forget the agent | `0x01` stdin frames, `restart`, `DELETE` (`phase-2-go-contract.md:2080`, `:2127`, `:2229`) | **the session's owner only.** Not delegable in v1 |

**Why steering is owner-only, and why that is a terms question and not only a security one.** An
agent's session is authenticated to its owner's Claude account. A teammate who types into it is
using that account. Anthropic's Consumer Terms §2: *"You also may not make your Account available
to anyone else"* (https://www.anthropic.com/legal/consumer-terms, effective 8 October 2025, read
2026-09-24). Remote steering by a teammate is exactly that, whatever our authorisation says.
Team/Enterprise plans are under the Commercial Terms instead
(https://code.claude.com/docs/en/legal-and-compliance, read 2026-09-24); the fetch of
https://www.anthropic.com/legal/commercial-terms (read 2026-09-24) found **no** explicit
account-sharing clause, so whether a Team seat may be steered by a colleague is **UNVERIFIED**, and
the design should not depend on it. The safe rule holds under both: **a person steers only agents
running on their own subscription.** "Help a teammate's agent" becomes "post a note to its lane"
(§5), which the owner's agent may read and the owner decides on.

**Consequence for the phase 2 attach path.** `/attach` is one bidirectional WebSocket
(`phase-2-go-contract.md:2227-2229`), guarded by Origin and a loopback `Host` check
(`:2233-2234`), and it is **exempt from the local caller key** (`hub-contract.md:2257`). None of
that survives being exposed off the machine. A watch relay must be a **new, separate, outbound-only
path** from the owner's control plane, which **drops `0x01` and `0x02` frames at the source**, so a
viewer's client is never trusted to be read-only. Exposing `/attach` itself to a teammate is ruled
out.

### 2.3 Can a teammate's agent read my agent's transcript?

**No, in v1.** Reasons, each binding:

1. The transcript is a harness's own file, read only through the pinned local adapter, and
   *"no feature may require the adapter to function"* (`docs/superpowers/planning/2026-09-23-owner-decisions.md:73-79`).
   A cross-user feature built on transcripts would require it.
2. Transcripts carry whatever the agent saw: file contents, `.env` values it printed, tokens it
   echoed. Forget exists to delete them (`owner-decisions.md:21`). If a copy lived on the Hub or on
   a teammate's machine, Forget would have to reach every copy, and could not reach a teammate's
   agent's context window at all.
3. The Hub today *"makes no outbound call carrying user content"* and holds no git or Panacea
   credential (`ADR-H1…:356-358`, `:526`). A transcript store would make it the most sensitive
   system in the product.

What the other agents get instead is **derived, bounded, attributed facts** (§5.2): the stated
intent, the claimed paths, the lane head and its commits, and the session status. Those degrade
to commit- and session-level facts, which is exactly OD-8's required fallback
(`owner-decisions.md:75-77`).

### 2.4 The authorisation additions team mode needs

| # | Addition | Why the current model cannot carry it |
|---|---|---|
| Z1 | **A `session presence` entity** in the Hub: `(project, lane ref, owner principal, harness, status, focus, updatedAt)`, written only by the owner's control plane with the owner's token | there is no session row on the Hub; `sessionId` in a session link *"means something only on the creator's machine"* (`hub-contract.md:941-942`) |
| Z2 | **Coordination records** (intent, claim, note) as immutable, append-only, attributed rows | the matrix has no row for them (`hub-contract.md:341-359`) |
| Z3 | **A watch grant**: `(session, viewer principal, expiresAt)`, created by the owner, revocable, and audited both ways | no per-session or per-person grant exists (`hub-contract.md:333`) |
| Z4 | **Viewer = read roster and coordination; member = also write own coordination and run own agents** | fits §5.2's roles without a new role |
| Z5 | **A row in the matrix test for every new route, run as V, M, A, O, N, X** | the H2 test is the tenancy control for T4, which is Go-only (`hub-contract.md:2072`) |

Z1 to Z5 add no role. The one new *kind* of permission is Z3, and it is granted by a person to a
person for one session, never by role.
