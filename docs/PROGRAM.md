# Panacea App — program decomposition

Full scope is agreed: Discover, Design, Develop, Merge, Deploy, on cloud microVMs, as a native desktop app. There is no time pressure, so phases are sequenced by **risk retired per unit of effort**, not by what demos soonest.

Each phase produces working, testable software and gets its own plan in `docs/superpowers/plans/`.

## Decisions locked

| Decision | Value |
|---|---|
| Repo | New — `PanaceaApp`, own ADO pipeline, consumes Panacea's API as a client |
| Desktop shell | **Tauri** (Rust) + React/TypeScript webview |
| Control plane | **Go**, reusing `panacea-api` patterns for auth and Kubernetes |
| Sandboxes | Azure Kata Containers (AKS pod sandboxing), one microVM per session |
| Terminals | Real PTYs streaming over WebSocket, rendered with xterm.js |
| Agents | Unmodified Claude Code / Codex / OpenCode / Gemini CLI, plus plain shells |
| AI capacity | Per-user rotation across that user's own subscriptions, metered API backstop |
| Identity | **EntraID, OAuth 2.0 auth-code + PKCE from the desktop**; tokens in Windows Credential Manager (`OD-6`). Was "the existing oauth2-proxy → Keycloak chain"; the Hub ADR decides any server-side broker |
| Shared state | **Shared from day one** — briefs, approvals, ledger, releases live in the Hub, our own server, in this monorepo (`OD-3`, `OD-5`) |
| Environment | **Our own**, modelled on Panacea and written from scratch here; Panacea is the deploy target and the model, not the host (`OD-5`, `OD-13`) |
| Execution | Subagent-driven, orchestrated, with review gates per task |

**Owner decisions of 2026-09-23 are recorded in `docs/superpowers/planning/2026-09-23-owner-decisions.md`
(`OD-1`…`OD-13`). Where this file and that one disagree, that one wins.**

## Local-first: Azure is a deployment target, not a dependency

**Decided 2026-09-17, after Phase 1 proved the pipe works against plain Docker.**

Everything that can be built and proven on a laptop is built and proven on a laptop before any Azure spend. This is not only about cost — it is a much faster development loop, and Phase 1 demonstrated the point: a real Claude Code session was driven end to end through Docker with no cluster involved.

**The architectural consequence, and it is the important one: the sandbox backend becomes an interface with two implementations.**

```
type Backend interface {
    Create(ctx, Spec) (Sandbox, error)
    Attach(ctx, id) (io.ReadWriteCloser, error)
    Destroy(ctx, id) error
    List(ctx) ([]Sandbox, error)
}
```

- **`docker`** — local development and the product's genuine "local machine" feature
- **`kata`** — Azure AKS pod sandboxing, what Phase 1 already built

This is not scaffolding to throw away. The product ships a **local machine** alongside cloud machines; the Docker backend *is* that feature. Building it first means the whole product runs on a laptop, and Azure work becomes a swap of one implementation rather than a prerequisite for anything.

It also means every later phase can be developed, demonstrated and reviewed with zero cloud cost.

## Revised phase order

Phases are now ordered by **how much can be proven without spending money**, while keeping risk-first ordering within that constraint.

| # | Phase | Needs Azure? |
|---|---|---|
| 1 | Live session — walking skeleton | ✅ **DONE** (proven on Docker) |
| 2 | Sandbox backend interface + the cockpit | No |
| 3 | Git, worktrees, coordination and convergence | No |
| **H** | **The Hub — identity, shared stores, teams** (added 2026-09-23, `OD-3`/`OD-5`; lettered so no number moves) | No — local-first on Docker Postgres + a real Entra app registration |
| 4 | Discover | No |
| 5 | Design canvas — **clean-room, paper.design-class, with MCP** (`OD-11`) | No |
| 6 | Capacity and credential rotation | Partly — engine is local, real limits need accounts |
| 7 | Deploy connector | Needs the Panacea API, not Azure directly |
| 8 | **Our own environment**: Kata backend + Terraform + warm pools + the Hub's hosting (`OD-5`) | **Yes** — the first real spend; written and planned before the owner provides the environment, never applied until then |
| 9 | Enterprise hardening and distribution | Yes |

The Kata work from Phase 1 (Terraform, the pod manager, the read-only-root volume fix) is already written and unit-tested. It sits dormant behind the backend interface until Phase 8 turns it on.

## Phases

### Phase 1 — One live session, end to end
The walking skeleton. A Tauri window showing a **real Claude Code session** running in a Kata microVM on the dev AKS, typed into from the desktop.

Retires the only genuinely unproven risk in the program: PTY fidelity over a WebSocket into a microVM — ANSI rendering, resize, reconnect, latency, and whether Claude Code's permission prompts survive the round trip. Everything after this assumes it works.

No rail, no auth, no persistence. One hard-coded session.

### Phase 2 — The cockpit
Machines, projects, branches, worktrees, **many sessions per worktree**, the rail, Columns and Canvas layouts, session lifecycle (resume / fork / restart / rename / kill), harness and model selection, plain shells, the command palette, keyboard map. EntraID sign-in. Persistence.

This is the prototype made real. Largest phase; split into its own sub-plans when written.

> **Renumbered 2026-09-23 to match the revised table above, which it had drifted out of.** The table
> was revised and this section was not, so the two disagreed on every phase from 3 onward and on the
> total (nine against eight — the table splits Kata out as its own phase and this section had no
> block for it). The table wins: it is the one marked *revised*, it carries the
> needs-Azure column the reordering was built around, and `plans/2026-09-18-phase-4-discover.md`
> already follows it. **The order is provisional** — the phases 4, 5 and 7 planning pass may surface
> dependencies that change it, and if it does, both the table and this section move together.

### Phase 3 — Git, worktrees, coordination and convergence
Worktree lifecycle against real git, file-claim coordination between concurrent agents, diff review with transcript back-links, merge ordering, conflict resolution, landed history with undo, PR creation.

### Phase H — The Hub
Added 2026-09-23 (`OD-3`, `OD-5`). The shared server the owner asked for: Entra identity (the desktop signs in with PKCE, `OD-6`), the brief store, approvals, the provenance ledger, releases and teams — every store `provenance-contract-draft.md` §2.1 names, living server-side so a teammate sees the same thread. Built local-first (Docker Postgres, a real Entra app registration); hosted in our own environment once phase 8 stands it up. The desktop's control plane calls it **as the user**. Its decomposition is its plan's ADR.

### Phase 4 — Discover
The question flow, generation of per-branch `CLAUDE.md`, inheritance project → branch → session, the provenance thread surfacing in Develop, Merge and Deploy.

### Phase 5 — Design canvas
**Re-scoped 2026-09-23 (`OD-11`): a clean-room, paper.design-class canvas with a first-class MCP server** — real-HTML frames, agent editing through MCP, multiplayer — built only on MIT/Apache/BSD/ISC parts, with a licence gate in CI. **No Paperdesign or doop code is read or reused**; the clean-room rules are in `planning/2026-09-23-owner-decisions.md`. Handoff bundle: tokens, component tree, screens.

*The paragraphs below are kept as the record of why Paperdesign was dropped.* It read "Adapt `Paperdesign` (the doop fork — real-HTML frames, MCP agent editing, multiplayer, already Tauri-shelled)".

**AGPL-3.0, and the trigger is NOT what this line said until 2026-09-23.** It read *"must be cleared before any external distribution"*. **That is the wrong question, and the wrong one is the easier one.** Verified against the licence text: `~/Projects/Paperdesign/package.json:5` is `"license": "AGPL-3.0-only"`, and **§13 "Remote Network Interaction"** at `LICENSE:540-546` requires a modified version to *"prominently offer all users interacting with it remotely through a computer network … an opportunity to receive the Corresponding Source"*. Paperdesign is a network-served app — `package.json:49` carries `express`.

So **two triggers are in scope, not one.** `CLAUDE.md` locks *we-host SaaS on Panacea's AKS first*, under which there may never be a "distribution" in the classical sense — **and §13 fires the moment a user's browser interacts with our modified copy.** The Tauri desktop shell is *also* classic distribution. **Ask legal about §13 under a hosted model**, not about distribution clearance.

Two further items nobody had raised: the licences of Paperdesign's own dependency tree, and what its `CLA.md` does — it grants **the upstream maintainer** sublicensing rights over contributions, which lets *them* relicense and does nothing for us as a downstream fork.

*Not legal advice; a reading of the licence text, recorded so the question asked is the right one.*

### Phase 6 — Capacity and credential rotation
Credential vault, per-user subscription registration, quota tracking from `anthropic-ratelimit-unified-*` headers, rotation on exhaustion, metered API/Bedrock backstop, per-session attribution and spend. Never pooled across users.

### Phase 7 — Deploy connector
Go client for `panacea-api`. Branch environments with arbitrary names, access control through EntraID groups, versions, rollback, auto-deploy, build and scan visibility.

### Phase 8 — Our own environment: Kata backend, Terraform, warm pools, the Hub's hosting
**Widened 2026-09-23 (`OD-5`)**: the owner is providing *"a whole new environment to run your own stuff"*, so this phase stands up the product's own environment — AKS with Kata, the Hub's hosting and database, identity configuration, networking — modelled on Panacea's live `deploy/main.tf` and written from scratch in this repo. Written and planned ahead; **never applied until the owner provides the environment**.

The first real spend. Turns on the Kata work already written and unit-tested in Phase 1 — Terraform, the pod manager, the read-only-root volume fix — which sits dormant behind the backend interface until this phase flips it.

### Phase 9 — Enterprise hardening and distribution
Audit trail, admin controls over permitted harnesses and models, signed MSI and update channel for the KPMG fleet, ISM evidence, optional customer-hosted tier.

## Cross-cutting constraints

- **The CLI is never reimplemented.** Harnesses run unmodified and unwrapped; the app orchestrates around real PTYs, and **no feature derives meaning from PTY bytes**. *Amended 2026-09-23 (`OD-8`):* a harness's own on-disk records — for Claude Code, its JSON-lines transcript — **may be read, read-only**, through a per-harness adapter pinned to the CLI version in the sandbox image, covered by a canary test that fails when the format moves; every consumer **degrades to commit- and session-level facts** when the adapter cannot read, and no feature may *require* it. Deleting such a file by the id the control plane minted is allowed; rewriting it is not. The old wording — "Any feature requiring us to parse or replace agent UI is wrong" — read differently from phase 2's "never parsed" for a transcript file; this is the one rule both now mean.
- **Design system**: `docs/DESIGN-SYSTEM.md` governs all UI. No progress bars, no counts outside the queue, one accent for interactive state only.
- **Agentic UX**: `docs/AGENTIC-UX.md` governs interaction. Assume the user is intermittently present.
- **Never pool subscriptions across users** — breaches Anthropic's Consumer Terms.
- **Kata constraints**: explicit CPU/memory requests are mandatory (the guest VM is sized from them), no host networking, Defender for Containers cannot assess Kata pods — document as a compliance gap.

## Open risks

1. **Kata cold start** is seconds; warm pools are mandatory and their sizing is unproven at our concurrency.
2. **Anthropic OAuth enforcement** — subscription tokens are restricted to Claude Code/Claude.ai server-side. Phase 6 (Capacity and credential rotation) must verify KPMG's plan type works via the supported path before building on it.
3. **Tauri distribution at KPMG** — code signing and update channel need platform-team engagement early, not at Phase 9.
4. **CLOSED 2026-09-23 by `OD-11`** — the owner chose to avoid AGPL entirely; phase 5 is a clean-room build. The record: **Paperdesign licence** — AGPL-3.0, and **not** "fine internally, blocking for sale" as this line said until 2026-09-23. Under the locked we-host model, **§13 Remote Network Interaction** can fire the moment a user's browser interacts with our modified copy, with no sale or distribution involved; the Tauri shell is classic distribution as well. See Phase 5 above for the verified licence text and the question to put to legal.

*The three items above were corrected on 2026-09-23. Items 2 and 3 still carried the pre-renumber phase numbers — Capacity was phase 3 and hardening phase 8 in the old prose — because the renumber in `6bcc9af` swept the phase blocks and not this section below them.*
