# Phase H: The Hub — shared server, sign-in, and the credential store

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status:** plan, 2026-09-23. Written from `docs/superpowers/adr/ADR-H1-hub-architecture.md` and
`docs/superpowers/contracts/2026-09-23-hub-contract.md` on branch `feat/hub`. Owner-reviewable under
`OD-1`; adversarially reviewed before code. **Nothing in this plan was compiled or run.** Where it
cites a file, line, type or route of the built tree, the citation was opened on 2026-09-23 in this
worktree; §"What this plan did not verify" lists the rest.

**Goal:** One shared server — the Hub — holds what `OD-3` made shared: users, teams, projects and
their git remotes, brief revisions, design approvals and their snapshots, releases with their pins,
phase 3's evidence, and a hash-chained audit log that doubles as the change feed. The desktop's
control plane signs the user in to Entra with auth-code + PKCE, keeps the refresh token in Windows
Credential Manager, and calls the Hub **as that user**. The whole stack runs on a laptop with one
`docker compose up` and a local OIDC issuer that speaks Entra's shape, so no Azure resource is
needed to build or test any of it.

**Architecture:** one new server binary and one new development binary, both in the existing
`control-plane` Go module (ADR-H1 §4.1); three new control-plane packages; one new desktop surface.

```
Tauri window (React)                control plane (cmd/api, 127.0.0.1:8730)          Hub (cmd/hub, 127.0.0.1:8750)
 ┌───────────────────┐  Bearer <local key>  ┌───────────────────────────────┐  Bearer <Entra AT>  ┌──────────────────┐
 │ sign-in surface   │ ──────────────────►  │ /v1/auth/*    (entraauth)     │ ──────────────────► │ /v1/...  server  │
 │ Discover/Design/  │ ◄──────────────────  │ phase 4/5/7/3 handlers        │ ◄────────────────── │ entra.Validator  │
 │ Deploy panes      │  Panacea-Hub-Cursor  │   └─ hubclient (HTTP | fake)  │   httpx.Problem     │ store (pgx)      │
 └─────────┬─────────┘                      │ credstore (WinCred | memory)  │                     └────────┬─────────┘
           │ opener: system browser         │ change-feed poller (5 s)      │                              │ hub_app (DML)
           ▼                                └──────────────┬────────────────┘                     ┌────────▼─────────┐
   Entra / devissuer  ◄── code + PKCE ── one-shot listener 127.0.0.1:0                            │ Postgres 16 (RLS)│
   (127.0.0.1:8740)                                                                               └──────────────────┘
```

Three rules hold throughout, and every task restates the one it touches:

1. **The renderer never holds an Entra token.** The control plane runs the flow, holds the tokens and
   calls the Hub (ADR-H1 §5.2, §6.2, §6.5). The renderer holds only the local caller key (task 30).
2. **No request body carries an actor.** Every Hub write records the principal of the token that made
   it (ADR-H1 §5.3; contract §5.3).
3. **Nothing Hub-derived is persisted on the user's machine.** The control plane may cache immutable
   revisions in memory; neither it nor the renderer writes Hub data to disk (ADR-H1 §5.4, assertion
   A5 widened).

**Tech stack.** Go 1.23 (the module's `go` directive does not move), `github.com/jackc/pgx/v5` (MIT,
the only new module requirement; contract §2), `golang.org/x/time/rate` and `golang.org/x/sys/windows`
promoted from indirect to direct. Postgres 16 (`postgres:16-alpine` locally, no extensions). The
desktop adds no npm dependency and no Rust crate: sign-in opens the browser through
`@tauri-apps/plugin-opener`, which the shell already carries (task 31 verifies).

**Spec:** ADR-H1 (the decision), the Hub contract (the exact routes, JSON, DDL and variables),
`docs/superpowers/planning/2026-09-23-owner-decisions.md` (`OD-1`…`OD-13`), `docs/PROGRAM.md`,
`docs/DESIGN-SYSTEM.md` (task 33 only).

---

## The decisions this plan rests on

| Id | Decision | Where it bites in this plan |
|---|---|---|
| `OD-3` | briefs, approvals and the ledger are shared from day one | the Hub exists at all; tasks 13–23 |
| `OD-4` | Azure DevOps **and** GitHub | `project_remotes.host` has both values (task 13); the credential store serves both (task 24–26) |
| `OD-5` | our own server side, in this monorepo | the Hub is `control-plane/cmd/hub`; the environment is phase 8's |
| `OD-6` | Entra, auth-code + PKCE, tokens in Windows Credential Manager, same store for ADO and GitHub | tasks 24–27; the credential-store seam (§"Seams") |
| `OD-8` | harness transcript rule | not touched: the Hub never sees a terminal byte or a transcript (ADR-H1 §1.3) |
| `OD-11` | permissive licences only | `pgx/v5` is MIT; nothing else is added |
| `OD-12` | owner hands-on steps batched to the end | task 33's visual acceptance and task 34's real-tenant sign-in stay **unverified** until then |
| `OD-13` | Panacea stays a separate API | the Panacea token is phase 7's, kept in this phase's store (§"Seams"); the Hub stores no Panacea credential |
| ADR-H1 §4 | Design A + amendment (Hub in the control-plane module) + grafts G1–G6 | the file map; G1 = task 18, G2 = task 16, G3 = task 21, G4 = task 04, G5 = task 21, G6 = task 22 |
| ADR-H1 §5.2 | direct Entra, no broker; the control plane holds tokens | tasks 25–27 |
| ADR-H1 §5.4 | local caller key on the control-plane hop | tasks 30–31 |
| ADR-H1 §10 Q1–Q7 | open owner questions, each with a recommendation | this plan builds every recommendation (§"Open questions") |

