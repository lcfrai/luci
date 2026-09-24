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

## 3. Consent, privacy and residency

### 3.1 Live-streaming another person's terminal

The owner wants teammates to *"see the work I am doing on it, the agents I have running, and what
their focus is"*. That splits cleanly in two, and the split is the whole privacy design:

- **The roster** (who, which agent, which lane, stated focus, status, last activity) is
  **work metadata**. Showing it to the team is the point of team mode and needs no per-view
  consent. Joining a team is the consent, and the invite screen should say so in one sentence.
- **The terminal** is **content**. It shows whatever the agent printed: file contents, a `.env`
  it `cat`ed, a token in an error message, a colleague's name in a test fixture. Phase 2 cannot
  redact it: redaction means scanning PTY bytes for meaning, which OD-8 forbids
  (`owner-decisions.md:73`), and pattern-based redaction is unreliable anyway.

So a watch is governed like screen-sharing, not like a dashboard:

| Control | Rule |
|---|---|
| **Default off** | no session is watchable until its owner shares it |
| **Per session, per person, time-boxed** | the Z3 grant (§2.4): one session, named teammates or "my team", an expiry (suggest 60 minutes, renewable) |
| **Visible while live** | the owner's row shows who is watching now, from the relay's own connection count, not from the viewer's claim |
| **Revocable at once** | revoking closes the relay connection, not just the next one |
| **Never stored** | the relay holds no scrollback and writes no bytes to disk or logs. The Hub's logging rule already bans bodies (`hub-contract.md:2115-2121`); the relay inherits it |
| **Replay is the owner's call** | on connect, a viewer gets live output only. The 256 KiB scrollback replay frame (`phase-2-go-contract.md:186`) is sent only if the owner ticked "include recent output" |
| **Audited** | `watch.granted`, `watch.started`, `watch.ended`, `watch.revoked`, each with both principals (§8) |

**Workplace-surveillance law — UNVERIFIED, needs KPMG legal.** Watching an employee's terminal
may be "computer surveillance" under state law (for example the *Workplace Surveillance Act 2005*
(NSW), which is understood to require notice to the employee). This was not read for this
document. The design above (owner-initiated, visible, per-session) is chosen so the owner is
always the one sharing, which is the posture least likely to engage it. It is not legal advice.

**Privacy Act 1988 (Cth), APP 11 — UNVERIFIED.** Terminals and coordination text can carry
personal information (client names in test data, colleagues' emails). APP 11 requires reasonable
steps to protect it. This was not read for this document; the controls that address it are §3.1
(no storage) and §3.3 (residency).

### 3.2 Transcripts and secrets: Forget must stay local and total

OD-10's Forget deletes the session's `<id>.jsonl` files on the owner's machine
(`owner-decisions.md:21`). Team mode must not create a copy Forget cannot reach. That rules out:

- **Transcript upload** to the Hub, for any feature (§2.3).
- **Transcript text in coordination records.** A control plane writing an intent must not
  summarise the transcript into it. The intent is what the **human** typed as the session's
  opening prompt or focus (`openingPrompt`, `title`, `phase-2-go-contract.md:2027`, `:2018`), or a
  one-line status the human edited. Anything auto-derived is from git (paths, commits), per OD-8's
  fallback.
- **Secrets in coordination records.** The Hub should run a secret-pattern check on every
  coordination write and refuse a match (`409 looks_like_secret`, suggested). This is a server-side
  check on text a person submitted, not a PTY scan, so it does not collide with OD-8.

What Forget **cannot** undo, and the design must say so on screen: anything a teammate already
**watched** live, and anything a teammate's agent already **read** from a coordination record. So
the second is kept free of secrets by construction (above), and the first is gated by consent
(§3.1).

**Erasure of coordination records.** The Hub's erasure is a tombstone that keeps key and digest
(`ADR-H1…:255`), audit rows hold no content (`ADR-H1…:520-523`), and only an org admin erases. For
coordination text, which the author typed, the author should also be able to tombstone their own
record. That is a new route, and it needs the same `SECURITY DEFINER` path as brief erasure.

### 3.3 Data residency

| Data | Where it lives | Residency |
|---|---|---|
| Roster, coordination records, watch grants, audit | the Hub | **Australia East**, geo-backup only to Australia Southeast (`ADR-H1…:524-526`) |
| Live terminal bytes while watched | the relay, in transit only | must run in the Hub's region; **never** a third-party relay (for example a hosted TURN or tunnelling service) |
| Transcripts, code, the lane clone | the owner's machine or sandbox | unchanged by team mode |
| Prompts, including a teammate's coordination text an agent reads | **Anthropic**, under the *reading* agent's owner's account | **not in Australia via the direct Claude service** — see below |

The last row is the new flow team mode creates: **text I wrote reaches Anthropic under my
teammate's account and settings.** Two consequences:

1. **Training settings differ per account.** The Consumer Terms §4 say Anthropic may use Materials
   for training *"unless you opt out of training through your account settings"*
   (https://www.anthropic.com/legal/consumer-terms, read 2026-09-24). A teammate on a consumer Pro
   or Max plan with training on would send my coordination text into training. **Control: a
   project policy that team-mode members run agents only on commercial accounts (Team, Enterprise,
   API, or a cloud provider), or on consumer accounts with training off.** The product cannot read a
   consumer account's training setting, so this is a policy and an attestation, not a check.
2. **Inference geography.** Third-party pages say the direct Claude API processes in the US or EU
   and that Australian residency comes through AWS Bedrock in Sydney
   (https://www.padiso.co/blog/deploying-claude-australia-data-residency-compliance-latency/ and
   https://www.futureability.com/claude-data-residency-australia, both found by search 2026-09-24,
   not read in full). Anthropic's own page, https://claude.com/regional-compliance, was **blocked by
   the corporate network** on 2026-09-24. **UNVERIFIED.** Team mode does not change this for a
   person's own work, but it widens it: my text now travels under whichever route my teammate
   chose. A project whose data must stay onshore needs a **project-level inference policy**
   ("Bedrock Sydney only") that every member's control plane enforces before launching an agent.

**The relay and the Hub are different services.** ADR-H1 states *"The Hub never sees a terminal
byte"* (`ADR-H1…:88`). A watch relay through the Hub's process would break that and put terminal
content next to the audit database. The relay should be its own stateless service in the same
region, authorising each connection by asking the Hub for the Z3 grant. If `05-design` routes
watch bytes through the Hub instead, ADR-H1 §1.3 must be amended first.

## 4. Subscription terms: "spin up an agent in my project" means on *your* account

### 4.1 What the terms say (read 2026-09-24)

| Source | Text |
|---|---|
| Consumer Terms §2 (https://www.anthropic.com/legal/consumer-terms, effective 8 Oct 2025) | *"You may not share your Account login information, Anthropic API key, or Account credentials with anyone else."* *"You also may not make your Account available to anyone else."* |
| Claude Code legal page (https://code.claude.com/docs/en/legal-and-compliance) | Hosted Claude Code needs the Commercial Terms; *"Customers may not pay for, resell, or intermediate Claude usage on their end users' behalf. Each end user must authenticate with their own Anthropic API key, Claude subscription plan credentials, or 3P inference provider credential"* |
| same page | developers may not *"route requests through Free, Pro, or Max plan credentials on behalf of their users"*, nor *"collect, store, or intermediate Claude.ai credentials or session tokens — sign-in to a Claude account must complete through Anthropic's own flow"* |
| same page | it does not *"prevent an end user from signing in to the unmodified Claude Code binary with their own Claude subscription, including where a platform hosts Claude Code"* |
| same page | *"Advertised usage limits for Pro and Max plans assume ordinary, individual usage of Claude Code and the Agent SDK."* |
| same page | Claude Code is under the Commercial Terms for Team, Enterprise and API users, the Consumer Terms for Free, Pro and Max |

The product's rule (`CLAUDE.md`, "never pool subscriptions across users") is consistent with all of
these. Team mode is where it is most likely to be broken by accident.

### 4.2 What it means for team mode

1. **An agent's account is its launcher's, fixed at launch.** "Alice spins up an agent in Dylan's
   project" runs on **Alice's** machine or sandbox, signed in to **Alice's** Claude account, in
   **Alice's** lane clone. The Hub records the launcher principal on the presence row (Z1), and that
   principal can never change for the life of the session.
2. **No "run this on the project's capacity".** A project has no subscription. Any feature shaped
   as "the team's agents", "the project's quota" or "borrow a teammate's spare capacity" is the
   pooling the terms forbid. Per-user rotation (`CLAUDE.md`, "AI capacity") rotates across one
   person's **own** accounts only.
3. **No hand-off of a running agent.** "Give my agent to Alice while I am on leave" is making my
   account available to her. The supported move is: Alice starts **her own** agent on the same lane,
   reading the lane's coordination records and commits. The lane is shared; the session is not.
4. **No credential crosses the Hub.** The Hub stores no git or Panacea credential (`ADR-H1…:356-358`)
   and must never store a Claude one either: sign-in completes in Anthropic's own flow, inside the
   unmodified CLI, in the launcher's sandbox (`owner-decisions.md:23`, OD-12's `/login`).
5. **Server-side enforcement exists only for what the Hub can see.** The Hub can refuse a presence
   row whose launcher is not the caller, and can refuse any API that would start a session for
   someone else. It **cannot** see which Claude account a CLI inside a sandbox signed in to. So a
   user who signs a sandbox in to a colleague's Claude account is out of the product's reach. The
   control is the terms, the sign-in screen's wording, and audit, not code. **This gap should be
   stated to the owner as residual, not claimed closed.**
6. **Managed policy follows the account.** A KPMG Claude account fetches KPMG's managed policy,
   which strips hooks and MCP on the laptop (phase 3 risk R4, `scope-and-risk-phase-3-git-convergence.md:389`,
   unverified inside a sandbox). Two teammates on differently-managed accounts get different agent
   behaviour on the same repo. Coordination must therefore work with **no hooks and no MCP** (§5.3).

## 5. Isolation, and prompt injection across teammates' agents

### 5.1 A teammate's agent must never reach my machine or sandbox

| Path | Rule | Where it is already enforced, or not |
|---|---|---|
| teammate → my laptop | **no inbound connection to any laptop, ever.** Every control plane dials out; nothing dials in | phase 2's control plane is loopback-only with a `Host` guard (`phase-2-go-contract.md:2234`, `hub-contract.md:2074`). Team mode must add no listener |
| teammate's sandbox → my sandbox (cloud) | denied | phase 8's `default-deny` plus sandbox policy: *"No rule allows a sandbox to reach another pod, so sandbox↔sandbox and sandbox→Hub/broker traffic is denied"* (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment-parts/task-11.md:155-159`). Cilium's `ipBlock` behaviour is **UNVERIFIED** there (`:162-164`) |
| teammate's sandbox → my sandbox (local Docker) | not applicable: the two sandboxes are on two different laptops | — |
| any sandbox → the Hub | denied (same line). **This is a feature:** no agent can write a coordination record directly | phase 8 task 11; locally, the Hub token never enters a sandbox (`ADR-H1…:551-552`) |
| teammate's code → my host | a teammate's lane arrives through the **remote**, and my host fetches it into the project under phase 3 ruling R4's hardening. Their commits are **data** until something executes them, and execution happens **only in a sandbox** | ruling R4 rules 1, 2 and 4 (`orchestrator-rulings.md:14-36`) |
| teammate's code → my sandbox | **accepted risk.** Running tests on a merged teammate lane runs their code, in my sandbox, with my Claude session. Same as pulling a colleague's branch today | the Kata boundary (cloud) or phase 1's container (local). No git credential is in the sandbox (A4, `scope-and-risk-phase-3-git-convergence.md:186`) |

Two consequences for `05-design`:

1. **The Hub is the only rendezvous.** Presence, coordination and watch grants go laptop → Hub
   and Hub → laptop on the laptop's own outbound poll (the change feed, `ADR-H1…:252`). No design
   may add peer-to-peer between laptops or sandboxes.
2. **An agent cannot publish.** Because sandboxes cannot reach the Hub and hold no Hub token, every
   coordination record is written by a **control plane**, from a **human's** input or from
   **deterministic** facts (git, session status). That single rule removes the worst injection
   path in §5.2, and it costs nothing: it is already true of the network.

### 5.2 Coordination data is untrusted input: the injection chain

The owner wants every agent to *"understand what each other are doing"*. Mechanically, that means
text written on behalf of one person is placed where another person's agent will read it. That is
**indirect prompt injection** as OWASP defines it: *"when an LLM accepts input from external
sources, such as websites or files"* (https://genai.owasp.org/llmrisk/llm01-prompt-injection/,
read 2026-09-24).

**The chain to design against.** A web page or a dependency README injects Alice's agent → Alice's
agent writes something that becomes a coordination record → Dylan's agent reads it → Dylan's agent
writes code or runs a command under Dylan's session in Dylan's sandbox → Dylan lands and pushes.
Every agent is both a reader and, indirectly, a writer, so an unfenced design lets one poisoned
page **propagate across the whole team's agents**. The attacker gains Dylan's session, Dylan's
egress and a chance of a commit under Dylan's name.

Anthropic's own guidance lists, for untrusted content, *"Avoid piping untrusted content directly
to Claude"* and notes that *"no system is completely immune to all attacks"*
(https://code.claude.com/docs/en/security, read 2026-09-24). So the fences below reduce the risk;
none of them closes it alone, and the ranking in §9 says so.
