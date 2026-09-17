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
| Identity | EntraID via the existing oauth2-proxy → Keycloak chain |
| Execution | Subagent-driven, orchestrated, with review gates per task |

## Phases

### Phase 1 — One live session, end to end
The walking skeleton. A Tauri window showing a **real Claude Code session** running in a Kata microVM on the dev AKS, typed into from the desktop.

Retires the only genuinely unproven risk in the program: PTY fidelity over a WebSocket into a microVM — ANSI rendering, resize, reconnect, latency, and whether Claude Code's permission prompts survive the round trip. Everything after this assumes it works.

No rail, no auth, no persistence. One hard-coded session.

### Phase 2 — The cockpit
Machines, projects, branches, worktrees, **many sessions per worktree**, the rail, Columns and Canvas layouts, session lifecycle (resume / fork / restart / rename / kill), harness and model selection, plain shells, the command palette, keyboard map. EntraID sign-in. Persistence.

This is the prototype made real. Largest phase; split into its own sub-plans when written.

### Phase 3 — Capacity
Credential vault, per-user subscription registration, quota tracking from `anthropic-ratelimit-unified-*` headers, rotation on exhaustion, metered API/Bedrock backstop, per-session attribution and spend. Never pooled across users.

### Phase 4 — Git, coordination and convergence
Worktree lifecycle against real git, file-claim coordination between concurrent agents, diff review with transcript back-links, merge ordering, conflict resolution, landed history with undo, PR creation.

### Phase 5 — Deploy connector
Go client for `panacea-api`. Branch environments with arbitrary names, access control through EntraID groups, versions, rollback, auto-deploy, build and scan visibility.

### Phase 6 — Discover
The question flow, generation of per-branch `CLAUDE.md`, inheritance project → branch → session, the provenance thread surfacing in Develop, Merge and Deploy.

### Phase 7 — Design
Adapt `Paperdesign` (the doop fork — real-HTML frames, MCP agent editing, multiplayer, already Tauri-shelled). Handoff bundle: tokens, component tree, screens. **AGPL-3.0 must be cleared before any external distribution.**

### Phase 8 — Enterprise hardening and distribution
Audit trail, admin controls over permitted harnesses and models, signed MSI and update channel for the KPMG fleet, ISM evidence, optional customer-hosted tier.

## Cross-cutting constraints

- **The CLI is never reimplemented.** Harnesses run unmodified; the app orchestrates around real PTYs. Any feature requiring us to parse or replace agent UI is wrong.
- **Design system**: `docs/DESIGN-SYSTEM.md` governs all UI. No progress bars, no counts outside the queue, one accent for interactive state only.
- **Agentic UX**: `docs/AGENTIC-UX.md` governs interaction. Assume the user is intermittently present.
- **Never pool subscriptions across users** — breaches Anthropic's Consumer Terms.
- **Kata constraints**: explicit CPU/memory requests are mandatory (the guest VM is sized from them), no host networking, Defender for Containers cannot assess Kata pods — document as a compliance gap.

## Open risks

1. **Kata cold start** is seconds; warm pools are mandatory and their sizing is unproven at our concurrency.
2. **Anthropic OAuth enforcement** — subscription tokens are restricted to Claude Code/Claude.ai server-side. Phase 3 must verify KPMG's plan type works via the supported path before building on it.
3. **Tauri distribution at KPMG** — code signing and update channel need platform-team engagement early, not at Phase 8.
4. **Paperdesign licence** — AGPL-3.0 is fine internally, blocking for sale.
