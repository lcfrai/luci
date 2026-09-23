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
command (`control-plane/cmd/hub/devstack.sh up`: Postgres in Docker, the Hub and a local OIDC issuer
that speaks Entra's shape on the host), so no Azure resource is needed to build or test any of it.

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
   calls the Hub (ADR-H1 §5.2, §6.2, §6.5). The renderer holds only the local caller key (tasks 37–38).
2. **No request body carries an actor.** Every Hub write records the principal of the token that made
   it (ADR-H1 §5.3; contract §5.3).
3. **Nothing Hub-derived is persisted on the user's machine.** The control plane may cache immutable
   revisions in memory; neither it nor the renderer writes Hub data to disk (ADR-H1 §5.4, assertion
   A5 widened).

**Tech stack.** Go 1.23 (the toolchain floor does not move; see the `go` directive constraint),
`github.com/jackc/pgx/v5` **v5.7.5** (MIT, the only new direct requirement; contract §2),
`golang.org/x/time/rate` and `golang.org/x/sys/windows` promoted from indirect to direct. Postgres 16
(`postgres:16-alpine` locally, no extensions). The desktop adds no npm dependency and no Rust
crate: sign-in opens the browser through `@tauri-apps/plugin-opener`, already a dependency
(`desktop/package.json`, `desktop/src-tauri/Cargo.toml`) and already used
(`desktop/src/components/menu/ContextMenu.tsx:44`).

**Spec:** ADR-H1 (the decision), the Hub contract (the exact routes, JSON, DDL and variables),
`docs/superpowers/planning/2026-09-23-owner-decisions.md` (`OD-1`…`OD-13`), `docs/PROGRAM.md`,
`docs/DESIGN-SYSTEM.md` (tasks 41 and 42).

---

## The decisions this plan rests on

| Id | Decision | Where it bites in this plan |
|---|---|---|
| `OD-3` | briefs, approvals and the ledger are shared from day one | the Hub exists at all; tasks 13–27 |
| `OD-4` | Azure DevOps **and** GitHub | `project_remotes.host` has both values (task 17); the credential store serves both (tasks 28, 29, 31, 39) |
| `OD-5` | our own server side, in this monorepo | the Hub is `control-plane/cmd/hub`; the environment is phase 8's |
| `OD-6` | Entra, auth-code + PKCE, tokens in Windows Credential Manager, same store for ADO and GitHub | tasks 28–32; the credential-store seam (§"Seams", S-CRED) |
| `OD-8` | harness transcript rule | not touched: the Hub never sees a terminal byte or a transcript (ADR-H1 §1.3) |
| `OD-11` | permissive licences only | `pgx/v5` and its three `jackc` requirements are MIT; nothing else is added |
| `OD-12` | owner hands-on steps batched to the end | tasks 41 and 42's visual acceptance and task 43's real-tenant sign-in stay **unverified** until then |
| `OD-13` | Panacea stays a separate API | the Panacea token is phase 7's, kept in this phase's store (S-CRED); the Hub stores no Panacea credential |
| ADR-H1 §4 | Design A + amendment (Hub in the control-plane module) + grafts G1–G6 | the file map; G1 = task 23, G2 = task 21, G3 = tasks 14 and 26, G4 = task 07, G5 = tasks 14 and 27, G6 = task 27 |
| ADR-H1 §5.2 | direct Entra, no broker; the control plane holds tokens | tasks 30–32 |
| ADR-H1 §5.4 | local caller key on the control-plane hop | tasks 37–38 |
| ADR-H1 §10 Q1–Q7 | open owner questions, each with a recommendation | this plan builds every recommendation (§"Open questions") |

---

## Global constraints

Every task restates the ones it touches. None of them moves.

- **Module and paths.** Go module `github.com/kpmg-au-emu/panacea-app/control-plane`
  (`control-plane/go.mod:1`). `go.work` lists `./control-plane` and `./sandboxd` and **gains
  nothing**: the Hub is a second `main` package in the existing module (ADR-H1 §4.1).
- **Test commands.** `go test ./...` is invalid at the workspace root (phase 2 plan, global
  constraints). Always:
  - unit: `go test ./sandboxd/... ./control-plane/... -race -count=1`
  - Hub integration (needs the local Postgres of task 09 on `127.0.0.1:5433`):
    `go test -tags integration ./control-plane/internal/hub/... -race -count=1`
    (`hubclient` lives at `control-plane/internal/hub/hubclient`, contract §2, so this covers it)
  - desktop: `cd desktop && npm test && npm run typecheck`
  An integration test file starts `//go:build integration`, so CI's untagged run
  (`azure-pipelines.yml:14`) never needs a database. Every integration test **skips, never fails**,
  when `PANACEA_APP_HUB_TEST_DATABASE_URL` is unset — and task 06's harness prints why.
- **The `go` directive.** It is `go 1.23` today (`control-plane/go.mod:3`, `go.work:1`) and CI pins
  1.23 (`azure-pipelines.yml:11`). Task 06 adds `github.com/jackc/pgx/v5` **pinned at v5.7.5**,
  whose own directive is `go 1.23.0` (read from the module cache, 2026-09-23); `go` will rewrite
  the module's line to `go 1.23.0`. That is the same toolchain floor and is **accepted**; anything
  above `1.23.0` is not. pgx `v5.10.0` (also cached) declares `go 1.25.0` and **must not be used**.
  Check `git diff control-plane/go.mod` before every commit that touches it.
- **No Go build inside Docker.** The corporate proxy breaks `go mod download` inside an image build
  (`images/sandbox/Dockerfile:3-6`) and the control-plane module is not vendored. Binaries are built
  on the host; that is why the local stack (task 09) runs only Postgres in a container.
- **Shell.** Every shell command goes through `~/.claude/tools/claudezsh/claudezsh` (the user's
  `CLAUDE.md` §1). Docker commands go through PowerShell or through the wrapper, never bare Git Bash
  path arguments (phase 2 constraint: Git Bash mangles container paths).
- **Commit trailer.** Every commit message ends with exactly
  `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`. Commit by pathspec
  (`git commit -F <file> -- <paths>`; `git add <path>` first for a new file). Never
  `git add -A/-u/.`, stash, reset, checkout, switch, rebase, merge, amend or push.
- **What never changes.**
  - Agent CLIs run unmodified and unwrapped; **no feature derives meaning from PTY bytes**
    (`OD-8`). The Hub never sees a terminal byte, a transcript or a session's output.
  - **Never pool** a subscription or a credential across users. Every token in this phase is the
    signed-in user's own, on that user's machine. The Hub server stores **no** git, Panacea or
    Claude credential (ADR-H1 §5.2; contract §11.2).
  - **No secret in source.** Configuration is `PANACEA_APP_*` variables, read in one block per
    binary (`control-plane/cmd/api/config.go:18-51` is the pattern; contract §12.2). The one
    exception is the four `AZURE_*` names AKS's workload-identity webhook injects (contract §12.2).
    `devissuer`'s committed `dev-tenant.json` holds fictional users and no secret.
  - **The control plane listens on loopback only** (`127.0.0.1:8730`, `config.go:54`). The Hub
    listens on loopback locally (`127.0.0.1:8750`) and on `0.0.0.0:8080` only when
    `PANACEA_APP_HUB_ENV` is `dev` or `prod` (contract §12.2). `devissuer` binds loopback only and
    refuses anything else. The sign-in listener binds `127.0.0.1:0`.
  - **No token in a log, an error, a URL, the renderer, or a sandbox.** Access tokens live in the
    control plane's memory; the refresh token lives in Windows Credential Manager.
  - `httpx` imports only the standard library (`control-plane/internal/httpx/problem.go:60-62`).
  - **No `GET` on the control plane changes state** (`control-plane/internal/api/api.go:15-21`).
    Every new `GET` is added to `catalogReadOnlyRoutes`
    (`control-plane/internal/api/routes_audit_test.go:32`).
- **UI.** Tasks 41 and 42 read `docs/DESIGN-SYSTEM.md` first and hold to its non-negotiables. The
  **owner's eyes are the only visual acceptance**, batched to the end (`OD-12`); every UI claim in
  a report says *unverified by the owner*.

---

## Where this plan corrects the Hub contract — the "Hub contract asks"

The contract says that where it and the ADR disagree, the ADR wins (contract `:4-5`). Reading the
built tree and the sibling plans found **ten** places where the contract, as written, would fail
when built. Each is a correction this plan builds, with the exact shape; none adds a Hub route.
They are asks against the contract's text, to be folded into it by whoever next edits it.

| # | Contract says | Why it fails | This plan builds | Task |
|---|---|---|---|---|
| **A1** | `repoKey` for Azure DevOps matches `^[a-z0-9-]{1,50}\.visualstudio\.com/[^/]{1,64}/[^/]{1,64}$` (§7.4) | phase 3's `githost.RepoKey()` returns `azure-devops/<org>/<project>/<repo>`, lowercased (phase 3 task 13 `:136-139`), and phase 3's own review ruled that form the right one because `dev.azure.com/<org>` and `<org>.visualstudio.com` are one organisation (phase 3 review I10, `:350-356`). The contract's regex would refuse every Azure DevOps key phase 3 sends | `github`: `^github\.com/[a-z0-9-]{1,39}/[a-z0-9._-]{1,100}$`; `azure-devops`: `^azure-devops/[a-z0-9-]{1,50}/[^/]{1,64}/[^/]{1,64}$`; **and** `repoKey == strings.ToLower(repoKey)` for both — the Hub refuses a non-canonical key rather than storing two spellings of one repository | 01, 17 |
| **A2** | `compose.yaml` runs `db`, `devissuer`, `migrate` and `hub` (§12.1; ADR-H1 §5.7) | three independent faults. (1) Building a Go binary inside `docker build` fails behind the corporate proxy (`images/sandbox/Dockerfile:3-6`). (2) `entra.Config.AllowInsecureAuthority` permits **only a loopback** `http` authority (contract §6.3; phase 8 task 16 `TestAnInsecureAuthorityIsRefused`), so a Hub in a container cannot validate against `http://devissuer:8740`. (3) The token's `iss` must be the same string for the browser, the control plane and the Hub; inside a bridge network the Hub cannot see the host's `127.0.0.1:8740` | `compose.yaml` runs **Postgres only**, on `127.0.0.1:5433`. `devissuer`, `hub migrate` and `hub serve` run as host processes, built and started by `control-plane/cmd/hub/devstack.sh up`, stopped by `down` (by PID, never by name). Every variable and port in contract §12.1 is unchanged | 09 |
| **A3** | "every route except `/healthz`" is behind the local key (§9.4, §13 C3) | the control plane's health route is `GET /v1/healthz` and `GET /api/v1/healthz`, mounted outside the guard (`control-plane/internal/api/api.go:303-306`); there is no `/healthz` | the exemptions are exactly those two patterns and `GET /v1/sessions/{id}/attach` (`attach.go:42`) | 37 |
| **A4** | the desktop sends `Authorization: Bearer <local key>` (§9.4) | the CORS preflight answers `Access-Control-Allow-Headers: Content-Type` only (`control-plane/internal/httpx/origin.go:168-171`). With an `Authorization` header every request, `GET` included, is preflighted, and the preflight refuses it: **no call would succeed** | `corsHeaders = "Content-Type, Authorization"`; the local-key middleware sits **inside** the origin middleware, so an allowed-origin `OPTIONS` is still answered 204 before any key check (`origin.go:231-237`) | 37 |
| **A5** | `devissuer` is `control-plane/cmd/devissuer`, standard library only (§2) | a `main` package cannot be imported, so task 30's tests could not run the real issuer in-process and would need a second, hand-written fake OIDC server | the issuer is a package, `control-plane/internal/devissuer` (standard library only), and `cmd/devissuer/main.go` is a ~60-line `main` over it. Test code may import it; the boundary test forbids `cmd/api` and `cmd/hub` from linking it | 11, 12 |
| **A6** | the control plane validates the `id_token`'s signature against `jwks_uri` (§6.1 step 9) | it needs a second RS256-over-JWKS implementation in the control plane beside `internal/entra`, which validates **access** tokens for a resource API and cannot validate an `id_token` (no `scp`, a different `aud`). Two copies of key-set code drift | the `id_token` arrives on the back channel, from the token endpoint the control plane itself called, so OIDC Core 1.0 §3.1.3.7 item 6 permits TLS server validation in place of the signature check. `entraauth` checks `iss`, `aud`, `nonce`, `exp`, `tid` and `oid` **without** a signature, and then cross-checks: `GET /v1/me` (step 11), whose access token the Hub validates in full, must answer an org whose `tenantId` equals the `id_token`'s `tid`, or the sign-in fails `exchange_failed` and nothing is stored | 30 |
| **A7** | the Hub maps `entra` rejections to 401 and says nothing of a key-set outage (§4.2, §6.3) | phase 8's `entra` adds `ErrKeySetUnavailable` (phase 8 task 16 `:102-106`); its `Middleware` answers that 503 `backend_unavailable`, which is a control-plane code meaning Docker (`problem.go:115`) | the Hub calls `Validator.Validate` itself in `internal/hub/server/auth.go` and maps `ErrKeySetUnavailable` to 503 **`unavailable`**; `/readyz` calls `JWKSValidator.Ready` | 13 |
| **A8** | the credential store has target names for a GitHub and a Panacea token (§9.1) but **no way to put one there** | nothing in the contract writes `PanaceaApp/git/github/…` or `PanaceaApp/panacea/…`; phase 3 would read an always-empty store and phase 7 would build its own write path into the same store | three **control-plane** routes (not Hub routes), write-only for the secret: `POST /v1/credentials/{kind}`, `DELETE /v1/credentials/{kind}?scope=`, `GET /v1/credentials` (metadata only) — `kind` ∈ `github`, `panacea`. Phase 7 uses these instead of writing its own (seam S-CRED) | 39 |
| **A9** | `hubapi` imports the standard library only; `hubclient` imports `hubapi` and the standard library (§2) | the same contract has `hubapi/codes.go` **import** the shared codes from `httpx` (§4.2), `hubclient.Error` carry an `httpx.Problem`, and `hubclient` mint idempotency keys with `ids.UUID()` (§9.2). Built as written, the boundary test fails the contract's own code | the boundary allows `hubapi` → `internal/httpx`; `hubclient` → `hubapi`, `internal/httpx`, `internal/ids`. Both `httpx` and `ids` import only the standard library, so the Hub still links nothing of the control plane's | 01, 09 |
| **A10** | the audit envelope covers `v, kind, orgId, seq, at, actorUserId, event, projectId, subject, data` (§3.6) | `audit_events.team_id` is not in it, yet the change feed decides who sees an event by `team_id` or `project_id` (§7.11). An owner-role edit of `team_id` would move an event into another team's feed **without breaking the chain** | the envelope adds `"teamId"` (or `""`). `hub verify` also checks each row's `feed` flag against the kind table of §8, since `feed` is derived from `kind`. Golden vectors 3 and 4 (task 02) are computed with it | 02, 14, 27 |

**Additions, not corrections** — shapes the contract did not name, which this plan adds and task 44
folds in. None is a Hub route.

| Addition | Exact shape | Why | Task |
|---|---|---|---|
| `Panacea-Auth-State` response header on the control plane, exposed to the webview beside `Panacea-Hub-Cursor` | `signed_out` \| `signing_in` \| `signed_in` | the desktop learns a sign-in finished, or a refresh died, from the poll it already makes — phase 2's one-request-per-tick rule (`poller.ts:1-3`) | 36, 40 |
| `hubclient` gains `Teams`, `Projects.Project`/`AddRemote`, `Designs.Approval`/`SessionLinks`, `Releases.Release`, `ErrSignedOut`, `ErrNotConfigured`, `ErrTokenUnavailable`, `Invalidator`, `WriteError`, `VerifyRevision`, `FormatRecorder` | task 04, 31, 33, 34's contracts | each wraps a route §7 already defines, or names a rule §4.3/§9.2 already states | 04, 31, 33, 34 |
| `entraauth.Config.AfterSignIn` returns `(hubTenantID string, err error)` | was `error` only | correction A6's cross-check needs the Hub org's tenant | 30 |
| Refresh redemptions serialised across **all** scopes | one lock; single-flight per scope under it | one refresh token, rotated on use, redeemed twice at once would sign the user out | 31 |
| An overdue `reserved` release **reads** `expired` before the janitor marks it | rendered state only | the fake and the Hub agree through the interface; no caller confirms a dead reservation | 23 |
| Control-plane `hub-link` and `hub/teams` routes | task 35's table | the share flow (Q8): without it no local project becomes a Hub project | 35, 42 |
| `server.Patterns()` | `func (s *Server) Patterns() []string` | the matrix meta-test and the conformance harness read the real route table, never a list | 08, 16, 34 |

### Files that belong to a sibling phase — "whichever lands first"

Three packages this plan needs are **owned by sibling plans** that may land before or after it.
Each task that needs one runs a preflight: if the file exists, it is used as it is; if not, the task
writes it **verbatim from the owning part**, so the later merge is a no-op or a whitespace conflict,
never two designs.

| Package | Owner | This plan's task | The owning part |
|---|---|---|---|
| `control-plane/internal/entra` | phase 8 task 16 | 10 | sibling `feat/env`, `…/2026-09-23-phase-8-own-environment-parts/task-16.md` |
| `control-plane/internal/credstore/credstore.go` | phase 3 task 13 | 29 | sibling `feat/phase-3-git`, `…/2026-09-23-phase-3-git-convergence-parts/task-13.md` |
| `control-plane/internal/githost/remote.go` | phase 3 task 13 | 35 | the same part |

Two further facts, verified here, **close** items the contract and ADR left open:

- **`opener:default` permits `http://` and `https://` URLs** — its generated description reads
  *"This permission set allows opening `mailto:`, `tel:`, `https://` and `http://` urls using their
  default application"*, with `allow-open-url` and `allow-default-urls` included (main tree,
  `desktop/src-tauri/gen/schemas/desktop-schema.json`, read 2026-09-23; this worktree has no
  generated schema). So sign-in needs **no capability change** (closes ADR-H1 §11, contract §15
  last bullet).
- **`golang.org/x/sys/windows` has no Credential Manager wrappers** — `grep -l "CredWrite\|CredRead"`
  over `x/sys@v0.24.0/windows/*.go` in the module cache finds nothing. Task 28 declares
  `CREDENTIALW` itself and calls `advapi32.dll` through `windows.NewLazySystemDLL`, which is what the
  ADR's *"via `golang.org/x/sys/windows`"* can only mean.

And one the contract's threat model states wrongly, **not** fixed here because it is phase 2's:
T7 cites *"the webview's CSP (`tauri.conf.json`)"*, but `desktop/src-tauri/tauri.conf.json` sets
`"security": {"csp": null}` — there is no CSP. Recorded as seam S-CSP.

---

## File map

Every file the phase creates or modifies, with the task that owns it. Paths are relative to the
repository root. **Shared files** — touched by more than one task in this plan, or by a sibling
phase's plan tonight — are marked ⚑; the tasks that touch them run in the order given, never in
parallel worktrees (see §"Order and parallel lanes").

### The Hub server — `control-plane/cmd/hub`, `control-plane/internal/hub/...`

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `control-plane/internal/ids/ids.go` ⚑ | `Wide(prefix)`: 16-hex ids for the Hub (contract §3.1) | modified | 01 |
| `control-plane/internal/hub/hubapi/codes.go` | the Hub's error codes (contract §4.2), shared ones re-exported from `httpx` | new | 01 |
| `control-plane/internal/hub/hubapi/keys.go` | key grammar, digests, branch refs, repo keys, id shapes — parse and format | new | 01 |
| `control-plane/internal/hub/hubapi/canonical.go` | `Canonical` — RFC 8785 restricted (contract §3.4) | new | 02 |
| `control-plane/internal/hub/hubapi/envelope.go` | the brief-revision and audit-event envelopes; `ValidateContent` | new | 02 |
| `control-plane/internal/hub/hubapi/types.go` | one struct per JSON object and request body of contract §7 | new | 03 |
| `control-plane/internal/hub/hubclient/hubclient.go` | the consumer interfaces, `Error`, `ErrUnavailable`, `TokenSource` | new | 04 |
| `control-plane/internal/hub/hubclient/fake*.go` | `NewFake()` — every observable rule of §7, in memory | new | 04, 05 |
| `control-plane/internal/hub/hubclient/conformance/conformance.go` | one table-driven suite for the fake and the real Hub; rows grow per task | new | 05; rows 15–27, 34 |
| `control-plane/internal/hub/hubclient/{client,transport,relay}.go` | the HTTP client, its retry/idempotency/refresh rules, the relay rule (contract §4.3) | new | 33 |
| `control-plane/internal/hub/hubclient/methods_*.go` | every interface method over HTTP, with the digest re-check | new | 34 |
| `control-plane/internal/hub/migrate/{migrate.go,roles.sql,roles_local.sql}` | the runner under an advisory lock; the per-environment roles | new | 06 |
| `control-plane/internal/hub/hubtest/hubtest.go` | integration harness: a scratch database per test, migrated, torn down | new | 06 |
| `control-plane/internal/hub/migrate/sql/0001_init.sql` | the complete first schema, guards, RLS, owner functions, grants | new | 07 |
| `control-plane/cmd/hub/{main,config,serve,verify}.go` | `hub serve`, `hub migrate`, `hub verify`; one config block | new | 08; `verify.go` extended 20, 27 |
| `control-plane/internal/hub/server/server.go` | mux, middleware chain, `/healthz`, `/readyz`, 404/405, headers, request logging | new | 08 |
| `control-plane/internal/hub/boundary_test.go` | the import boundary (ADR-H1 §4.1, contract §2, A9) | new | 09 |
| `control-plane/cmd/hub/{compose.yaml,devstack.sh,README.md}`, `.gitignore` | the local stack: Postgres in Docker, the two binaries on the host (correction A2); `.out/` ignored | new, modified | 09 |
| `control-plane/internal/entra/*` | Entra access-token validation — **phase 8 task 16's files**; written here only if that task has not landed | new | 10 |
| `control-plane/internal/devissuer/*` | the local OIDC issuer, as a package (correction A5) | new | 11, 12 |
| `control-plane/cmd/devissuer/{main.go,dev-tenant.json}` | its `main`, and the fictional tenant | new | 11 |
| `control-plane/internal/hub/server/{auth,ratelimit}.go` | Origin refusal, rate limits, token validation, principal resolution (contract §5.1) | new | 13 |
| `control-plane/internal/hub/store/{store,dbauth}.go` | the pool; Entra-only Postgres over the standard library | new | 08 |
| `control-plane/internal/hub/store/orgs.go` | org enrolment at start; principal resolution | new | 13 |
| `control-plane/internal/hub/server/tx.go`, `store/{audit,idempotency}.go` | the one write-transaction shape (contract §10.6): audit lock, chain, idempotency | new | 14 |
| `control-plane/internal/hub/server/{me,teams}.go`, `store/teams.go` | `/v1/me`, teams, members | new | 15 |
| `control-plane/internal/hub/server/invites.go`, `store/invites.go` | invites and their binding | new | 16 |
| `control-plane/internal/hub/server/matrix_integration_test.go` | the authorisation matrix (contract §5.3), every route × every principal | new | 16; rows 17–27 |
| `control-plane/internal/hub/server/projects.go`, `store/projects.go` | projects and remotes | new | 17 |
| `control-plane/internal/hub/server/briefs.go`, `store/briefs.go` | brief heads | new | 18 |
| `control-plane/internal/hub/server/revisions.go`, `store/revisions.go` | the batch revision write and history | new | 19 |
| `control-plane/internal/hub/server/refs.go`, `store/refs.go` | `/v1/refs` | new | 20; design half 22 |
| `control-plane/internal/hub/blobstore/blobstore.go`, `server/blobs.go`, `store/blobs.go` | content-addressed blobs in Postgres behind an interface (G2) | new | 21 |
| `control-plane/internal/hub/server/{approvals,sessionlinks}.go`, `store/{approvals,sessionlinks}.go` | design approvals and session links | new | 22 |
| `control-plane/internal/hub/server/releases.go`, `store/releases.go` | reserve, confirm, abandon, the deployment join (G1) | new | 23, 24 |
| `control-plane/internal/hub/server/janitor.go` | expiry, blob collection, idempotency cleanup, under an advisory lock | new | 24 |
| `control-plane/internal/hub/server/evidence.go`, `store/evidence.go` | phase 3's evidence (H-2) | new | 25 |
| `control-plane/internal/hub/server/changes.go`, `store/changes.go` | the change feed (G3) | new | 26 |
| `control-plane/internal/hub/server/{audit,erase}.go`, `store/erase.go` | the audit route, erasure (G6) | new | 27 |
| `control-plane/go.mod`, `go.sum` ⚑ | `pgx/v5 v5.7.5`; `x/time`, `x/sys` direct | modified | 06 (pgx), 13 (`x/time`), 28 (`x/sys`) |

### The control plane — `control-plane/cmd/api`, `control-plane/internal/...`

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `control-plane/internal/credstore/{secrets,memory,chunk,targets}.go` | `Secrets`, the in-memory store, chunking, the target-name grammar | new | 28 |
| `control-plane/internal/credstore/wincred_windows.go`, `wincred_other.go` | Windows Credential Manager over `advapi32.dll`; the non-Windows stub | new | 28 |
| `control-plane/internal/credstore/credstore.go` ⚑ | **phase 3 task 13's file** (`Host`, `Credential`, `Store`, `None`, `Chain`); written here verbatim only if absent | new | 29 |
| `control-plane/internal/credstore/{tokens,local}.go` | `GitHubTokens`, `PanaceaTokens`, `Inventory`; `Local` (phase 3's `Store` for GitHub) | new | 29 |
| `control-plane/internal/credstore/credstoretest/credstoretest.go` | the behaviour suite every `Secrets` passes | new | 28 |
| `azure-pipelines.yml` ⚑ | one step: `GOOS=windows go vet ./control-plane/...` | modified | 28 |
| `control-plane/internal/githost/remote.go` ⚑ | **phase 3 task 13's file** (`Remote`, `ParseRemote`, `RepoKey`); written here verbatim only if absent | new | 35 |
| `control-plane/internal/entraauth/{entraauth,oidc,pkce,listener,signin,idtoken}.go` | discovery, PKCE, the one-shot listener, the code exchange, A6's `id_token` checks | new | 30 |
| `control-plane/internal/entraauth/{tokens,account,devops}.go` | `TokenSource`, refresh, rotation, sign-out, start-up; the Azure DevOps adapter | new | 31 |
| `control-plane/internal/httpx/problem.go` ⚑ | four codes: `signed_out`, `hub_not_configured`, `local_key_required`, and `upstream_unavailable` if R4 has not landed | modified | 32 |
| `control-plane/internal/api/auth.go` | `/v1/auth/*` | new | 32 |
| `control-plane/internal/api/api.go` ⚑ | `Deps` fields; registrations; the local-key wrap | modified | 32, 35, 36, 37, 39 |
| `control-plane/internal/api/routes_audit_test.go` ⚑ | new `GET`s in `catalogReadOnlyRoutes` | modified | 32, 35, 36, 39 |
| `control-plane/cmd/api/{config,main}.go` ⚑ | the new variables; wiring | modified | 32, 35, 36, 37 |
| `control-plane/internal/hublink/hublink.go`, `internal/api/hublink.go` | local project → Hub project; the share routes | new | 35 |
| `control-plane/internal/hubfeed/poller.go`, `internal/api/hubchanges.go` | the change-feed poll; the cursor and auth-state headers | new | 36 |
| `control-plane/internal/httpx/origin.go` ⚑ | expose `Panacea-Hub-Cursor` and `Panacea-Auth-State`; allow the `Authorization` header (A4) | modified | 36, 37 |
| `control-plane/internal/hub/hubclient/hubclient.go` | the optional `Invalidator` | modified | 31 |
| `control-plane/internal/localkey/localkey.go` | the local caller key: mint, write, compare | new | 37 |
| `control-plane/internal/api/credentials.go` | the write-only credential routes (A8) | new | 39 |

### The desktop — `desktop/`

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `desktop/src-tauri/src/lib.rs` ⚑ | `local_key` command | modified | 38 |
| `desktop/src/api/localKey.ts`, `desktop/src/api/index.ts` | fetch the key once, hold it in memory; build `api` with it | new, modified | 38 |
| `desktop/src/api/client.ts` ⚑ | the key on every call; auth, hub-link, credential and change functions; new codes | modified | 38, 40 |
| `desktop/src/api/hubFeed.ts` | the one feed reader and its subscriber registry | new | 40 |
| `desktop/src/api/poller.ts` ⚑, `desktop/src/App.tsx` ⚑ | the header hook after a good tick; one line wiring it | modified | 40 |
| `desktop/src/store/slices/authSlice.ts`, `store/store.ts` ⚑, `store/types.ts` ⚑ | sign-in and share state, never persisted | new, modified | 40, 42 |
| `desktop/src/components/rail/{RailFooter,AccountControl}.tsx`, `rail.css` ⚑ | the sign-in surface | modified, new | 41 |
| `desktop/src/components/share/{SharePopover.tsx,share.css}` | share a project with a team | new | 42 |
| `desktop/src/components/menu/ContextMenu.tsx` ⚑, `desktop/src/components/AppShell.tsx` ⚑ | the *Share with a team…* item; the popover's one mount | modified | 42 |

### Proof and documents

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `control-plane/internal/hub/e2e_integration_test.go` | sign-in through `devissuer`, then every consumer interface against the real Hub | new | 43 |
| `docs/verification/2026-09-23-phase-h-results.md` | what was run, what was observed, what stays unverified | new | 43 |
| `docs/PROGRAM.md`, `docs/superpowers/contracts/2026-09-23-hub-contract.md`, `…/2026-09-18-phase-2-ui-contract.md`, `…/planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md` | the amendments ADR-H1 §7.2 names, and A1–A10 plus the additions folded into the contract | modified | 44 |

---

## Tasks

Each task has its part file, `docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-NN.md`, and
each is sized for one implementer in **1–3 hours**. Hours assume the implementer has read ADR-H1,
the contract, and the part; they are the plan's estimate, not a measurement. Contract task ids
(H0–H7, C1–C4) are given so the two documents can be read side by side.

### The minimum Hub first — and where each sibling phase can start

The minimum is what phases 4, 5 and 7 need to **integrate against a real Hub**, and nothing else.
It is tasks **01–24** on the server and **28, 30–35, 37, 38** on the client — 33 of the 44 tasks,
35 with the credential pair (29, 39) phase 7 needs. Four start lines fall inside it:

| Start line | After | Who can start | What they can do |
|---|---|---|---|
| **S0 — code against the fake** | 01–05 (≈ 13 h) | phases **3, 4, 5, 7** | build every Hub-backed handler against `hubclient`'s interfaces and `hubclient.NewFake()`, and run `hubclient/conformance` against the fake. Nothing below S0 changes an interface they code to |
| **S4 — phase 4 integrates** | 06–20 (server), 28, 30–34 partial, 35 (client) | phase **4** | point its `discoverstore` Hub implementation at the local stack: create briefs, write revisions, resolve keys, with a real sign-in through `devissuer` |
| **S5 — phase 5 integrates** | S4 + 21–22 | phase **5** | upload snapshots, approve designs, write session links, resolve `design:` keys |
| **S7 — phase 7 integrates** | S5 + 23–24, 29, 39 | phase **7** | reserve → deploy → confirm/abandon, rollback, the deployment join; its `pnc_` token in the credential store |
| S3 — phase 3's shared evidence | S4 + 25, 29, 31, 39 | phase **3** | `evidence.Store`'s Hub implementation (`Shared() == true`); push and PR with a token from the store (GitHub pasted; Azure DevOps from the Entra sign-in if Q5 is yes) |

### The task list

| # | Task | Contract id | Depends on | Hours | Min? |
|---|---|---|---|---|---|
| 01 | `ids.Wide`; `hubapi` codes, keys, digests, branch refs, repo keys | H0 | — | 2 | ✓ |
| 02 | `hubapi.Canonical`, the envelopes, `ValidateContent`, the golden vectors | H0 | 01 | 2.5 | ✓ |
| 03 | `hubapi` wire types for every object and request of §7 | H0 | 01 | 2.5 | ✓ |
| 04 | `hubclient` interfaces, `Error`, `ErrUnavailable`; the fake, part 1 (projects, teams, briefs, refs) | H0 | 02, 03 | 3 | ✓ |
| 05 | the fake, part 2 (designs, releases, evidence, feed); the conformance suite — **S0** | H0 | 04 | 3 | ✓ |
| 06 | the migrate runner, `roles.sql`, the integration harness; `pgx` pinned | H0 | — | 3 | ✓ |
| 07 | `sql/0001_init.sql` complete, with a database test for every guard | H0 | 06 | 3 | ✓ |
| 08 | `cmd/hub`: config, `serve`, `migrate`, `verify` shell; the server chassis; the pool | H0 | 01, 06, 07 | 3 | ✓ |
| 09 | the local stack: Postgres-only compose, `devstack.sh`, the README (A2); the import boundary test | H0 | 07, 08, 12 | 2 | ✓ |
| 10 | `internal/entra` — phase 8 task 16, verbatim, **only if it has not landed** | H1 | — | 3 (0 if landed) | ✓ |
| 11 | `devissuer`, part 1: the package, config, discovery, JWKS, token minting; `cmd/devissuer` | H1 | — | 2.5 | ✓ |
| 12 | `devissuer`, part 2: `/authorize`, `/token` (code + PKCE S256, rotating refresh) | H1 | 11 | 3 | ✓ |
| 13 | the Hub's auth pipeline: Origin, rate limits, validation, principal, org enrolment (A7) | H1/H2 | 07, 08, 10 | 3 | ✓ |
| 14 | the write transaction: audit lock, hash chain, idempotency (contract §10.6) | H2/H7 | 02, 13 | 3 | ✓ |
| 15 | `/v1/me`, teams, members | H2 | 03, 14 | 3 | ✓ |
| 16 | invites and binding; the authorisation matrix harness | H2 | 15 | 3 | ✓ |
| 17 | projects and remotes (A1) | H3 | 16 | 2.5 | ✓ |
| 18 | brief heads: create, list, for-branch, get, rebind, archive | H3 | 17 | 3 | ✓ |
| 19 | the batch revision write and history | H3 | 18 | 3 | ✓ |
| 20 | `/v1/refs`, brief half; `hub verify` re-checks revisions — **S4 (server side)** | H3 | 19 | 2.5 | ✓ |
| 21 | blobs, behind `blobstore` (G2) | H4 | 17 | 3 | ✓ |
| 22 | design approvals, session links, `/v1/refs` design half — **S5 (server side)** | H4 | 20, 21 | 3 | ✓ |
| 23 | releases: reserve and the pin (G1); list and get | H5 | 22 | 3 | ✓ |
| 24 | confirm, abandon, the deployment join; the janitor — **S7 (server side)** | H5 | 23 | 3 | ✓ |
| 25 | evidence (H-2) | H6 | 20 | 1.5 | |
| 26 | the change feed (G3) | H7 | 16 | 2 | |
| 27 | the audit route, erasure (G6), the full `hub verify` | H7 | 26, 19 | 2.5 | |
| 28 | `credstore`: `Secrets`, `Memory`, `WinCred` with chunking, target names | C1 | — | 3 | ✓ |
| 29 | `credstore`: `Local` (GitHub), `PanaceaTokens`; phase 3's `credstore.go` if absent | C1 | 28 | 2 | S7 |
| 30 | `entraauth`: discovery, PKCE, the one-shot listener, the exchange (A6) | C1 | 12, 28 | 3 | ✓ |
| 31 | `entraauth`: `TokenSource`, refresh single-flight, rotation, sign-out, start-up; `DevOps` | C1 | 30 | 3 | ✓ |
| 32 | the control plane's `/v1/auth/*`, its variables, its codes, the wiring | C1 | 31 | 2.5 | ✓ |
| 33 | `hubclient` over HTTP: transport, retries, idempotency, refresh, the relay rule | C2 | 04 | 3 | ✓ |
| 34 | `hubclient` methods, the digest re-check, conformance against the real Hub | C2 | 33, 20 (rows grow with 22–27) | 3 | ✓ |
| 35 | `hublink`: local project → Hub project; the share routes | C2 | 32, 34; phase 3 task 13's `githost` (or its copy) | 3 | ✓ |
| 36 | the change-feed poller, `/v1/hub/changes`, the cursor header | C2 | 26, 35 | 2 | |
| 37 | the local caller key, control-plane side (A3, A4) | C3 | 32 | 2 | ✓ |
| 38 | the local caller key, desktop side | C3 | 37 | 2 | ✓ |
| 39 | the credential routes: GitHub and Panacea tokens (A8) | C1 | 29, 37 | 2 | S7 |
| 40 | the desktop's auth, share and feed client; `authSlice`; `hubFeed` | C4 | 36, 38, 39 | 3 | |
| 41 | the sign-in surface in the rail footer — **UI** | C4 | 40 | 3 | |
| 42 | share a project with a team — **UI** | C4 | 40, 41 | 3 | |
| 43 | end-to-end proof through `devissuer`; the real-tenant checklist; the results record | all | 01–42 | 3 | |
| 44 | the document amendments; A1–A10 folded into the contract; the seams handed over | — | 43 | 1.5 | |

Totals: server 01–27 ≈ 73.5 h; client and desktop 28–42 ≈ 39.5 h; proof and documents 4.5 h;
**≈ 117.5 engineer-hours**, against ADR-H1's 16 engineer-days (≈ 128 h). The difference is the
ADR's day including review; this table is implementation time. The minimum (S7 reached) is
tasks 01–24, 28–35, 37–39 ≈ 96 h: about **six working days for two engineers** — one on the server
lane, one on identity then the client lane.

### Order and parallel lanes

Four lanes, each its own git worktree branched from `feat/hub`, so they can run at once. **An
implementer never merges**; the orchestrator merges a lane's finished tasks into `feat/hub`, in the
order below, and a lane that needs another lane's task waits for that merge.

| Lane | Worktree branch | Tasks, in order | Waits for |
|---|---|---|---|
| **A — contract in code, then server** | `feat/hub-server` | 01 → 02 → 03 → 04 → 05 → 06 → 07 → 08 → 13 → 14 → 15 → 16 → 17 → 18 → 19 → 20 → 21 → 22 → 23 → 24 → 25 → 26 → 27 | 13 waits for 10 (lane B) |
| **B — identity** | `feat/hub-identity` | 10 → 11 → 12 → 09 | 09 waits for 07 and 08 (lane A) |
| **C — control plane** | `feat/hub-client` | 28 → 29 → 30 → 31 → 32 → 37 → 33 → 34 → 35 → 39 → 36 | 28 waits for 06 (go.mod); 30 for 12; 33 for 04; 34's rows for 20, 22, 24–27; 36 for 26 |
| **D — desktop** | `feat/hub-desktop` | 38 → 40 → 41 → 42 | 38 for 37; 40 for 36 and 39 |
| — | `feat/hub` | 43 → 44 | everything |

**Why the lanes are cut this way — the shared files.** Five tasks edit the control plane's wiring
(`internal/api/api.go`, `cmd/api/config.go`, `cmd/api/main.go`, `routes_audit_test.go`): 32, 35,
36, 37, 39. Two edit `httpx/origin.go`: 36, 37. All are in lane C, one after another, so no two
worktrees ever edit those files at once. Four desktop tasks edit `client.ts` or the store: all in
lane D. `control-plane/go.mod` is edited by 06 (pgx), 13 (`x/time` direct) and 28 (`x/sys` direct);
28 waits for 06's merge, and 13 is in 06's own lane.

**Sibling phases edit the same wiring files tonight** — phases 3, 4, 5 and 7 each register routes
in `api.go`, add fields to `Deps`, variables to `config.go` and codes to `httpx`. Every task here
follows the rule that keeps those merges mechanical: a handler lives in **its own new file**; the
edit to `api.go` is **one registration line and one `Deps` field**; a new code is **appended at the
end** of `httpx`'s constant block with a comment naming its task; a new variable is **appended at
the end** of `config.go`'s block. A merge conflict in those files is then two adjacent additions,
resolved by keeping both.

---

## Seams — what later phases and sibling phases plug into

| Id | Seam | Shape | Who consumes it |
|---|---|---|---|
| **S-CRED** | **The credential store.** One machine store, Windows Credential Manager (`credstore.Secrets`, `WinCred()`; `Memory()` off Windows and in tests), **one target grammar** (`credstore/targets.go`), and one typed accessor per credential: the Entra refresh token (`entraauth`, task 31), a **GitHub** token (`credstore.Local`, phase 3's `Store`), an **Azure DevOps** bearer from the Entra sign-in (`entraauth.DevOps`, phase 3's `Store`, if Q5 is yes), and the **Panacea** `pnc_` token (`credstore.PanaceaTokens`, phase 7). The only write path for a pasted token is task 39's `POST /v1/credentials/{kind}`; nothing returns a secret | tasks 28, 29, 31, 39 | **phase 3** (`OD-4`: push and PR on both hosts), **phase 7** (`OD-13`: the Panacea API), **phase 6** (a future `kind` for the user's own Claude credentials — never pooled), **phase 8** (nothing: the broker reads no store) |
| S-TOKEN | `entraauth.TokenSource` — `Token(ctx, scope)`, `ErrSignedOut` | task 31 | `hubclient` (task 33); phase 8's broker client `remotebe` (HA-2) |
| S-HUB | `hubclient`'s consumer interfaces, `NewFake()`, the conformance suite, and `hubclient.WriteError` (the relay rule) | tasks 04, 05, 33, 34 | phases 3 (`Evidence`), 4 (`Briefs`), 5 (`Designs`), 7 (`Releases`) |
| S-LINK | `hublink.Resolver.Project(ctx, localProjectID) (hubapi.Project, hublink.State, error)` — how a handler finds the Hub project behind a local one | task 35 | phases 3, 4, 5, 7: every Hub-backed handler starts here |
| S-FEED | the control plane's `hubfeed.Poller.Subscribe(kindPrefix, fn)`; the desktop's `hubFeed.subscribe(kindPrefix, fn)` | tasks 36, 40 | phase 4's `provenanceSlice` (`brief.*`), phase 5's approvals (`design.*`), phase 7's releases (`release.*`) |
| S-ENTRA | `internal/entra` — one validator for the Hub and the broker (HA-1) | task 10 / phase 8 task 16 | phase 8's `cmd/broker` |
| S-CHART | `hub serve` / `hub migrate` / `hub verify`; `:8080`; `/healthz`, `/readyz`; `roles.sql`; `PANACEA_APP_HUB_*` | tasks 06–08 | phase 8 tasks 27 (image), 29 (chart, principals SQL), 30 (pipeline) |
| S-CI | an integration job with a Postgres service, running `go test -tags integration ./control-plane/internal/hub/...` | not built here: `azure-pipelines.yml` runs no database today | phase 8's pipelines (ask) |
| S-CANVAS | a `/v1/canvas/…` WebSocket route on the same server, identity from the same validator (ADR-H1 §5.5) | reserved; nothing built | phase 5 v2 |
| S-SSE | `GET /v1/changes/stream`, the same events, `Last-Event-ID` as the cursor | reserved | when the 5 s poll is noticed |
| S-BYO | multi-tenant registrations, a second `PANACEA_APP_HUB_TENANTS` entry, an enrolment route | reserved (ADR-H1 §5.9 item 3) | the customer tier |
| S-CSP | the webview has **no CSP** (`tauri.conf.json` `"csp": null`) | not ours; phase 2 or 9 | phase 5 renders design frames in that webview |

---

## Open questions for the owner — each with the recommendation this plan builds

ADR-H1 §10's seven stand, and this plan builds each recommendation: **Q1** only a complete revision
is approved against or pinned; **Q2** erasure keeps key, digest, author and time and resolves
`erased`; **Q3** a designer may approve their own design in v1; **Q4** teammates join by an email
invite that binds at first sign-in; **Q5** Azure DevOps uses the same Entra sign-in; **Q6** writes
never queue while the Hub is down; **Q7** the Hub holds no canvas documents in v1. Three more come
from this plan:

| # | Question | Recommendation (what this plan builds) |
|---|---|---|
| **Q8** | How does a local project join the Hub? | **Explicitly**: a *Share with a team* action on the project row (task 42) creates the Hub project from the local project's remotes, in a team the user picks or creates in the same popover. A project whose remote is already some Hub project's links **automatically** on first sight (task 35), because that is the same repository. The alternative — auto-create a personal team for every local project — makes teams nobody chose |
| **Q9** | Where does a GitHub token come from in v1? | **Pasted once** into the app (a fine-grained personal access token), stored by task 39, with phase 3's machine-credential-helper fallback behind it. A GitHub OAuth app would need an app registration under `kpmg-au-emu` and `lcfrai` — later |
| **Q10** | Who runs the Hub's database tests in CI? | **Phase 8's pipeline** adds a Postgres service job (S-CI). Until then they run on a developer laptop against task 09's stack, and every report says so |

---

## What this plan did not verify

- **Nothing was compiled or run.** No Go was built, no container started, no SQL applied, no
  token issued.
- **Entra behaviour** carried from ADR-H1 §11: loopback redirect matching, v2 claims with
  `profile`, refresh rotation, one refresh token for three resources, KPMG token sizes. Task 43's
  real-tenant checklist checks them once the registrations exist, and stays unverified until the
  owner's batched session (`OD-12`).
- **Postgres facts from knowledge, pinned by tests rather than trusted:** the default name
  `users_org_id_entra_oid_key` that `resolve_principal`'s `ON CONFLICT ON CONSTRAINT` relies on
  (task 07 asserts it exists); that a transaction-local `set_config` leaves `''`, not `NULL`, on a
  pooled connection after commit (task 13's test runs a second transaction on the same connection
  and asserts it reads nothing); `SECURITY DEFINER` running as the owner for `guard_revision`'s
  `current_user` test (task 27).
- **Win32 facts from documentation recall:** `CRED_MAX_CREDENTIAL_BLOB_SIZE` = 2,560 bytes; the
  `CREDENTIALW` layout. Task 28's Windows-only test round-trips a 6,000-byte blob to prove the
  chunking, on the owner's laptop, since CI is Linux (`azure-pipelines.yml:6`).
- **Sibling plans were read as they stood at about 21:30 on 2026-09-23.** Phase 3 (23 parts and a
  review) and phase 8 (33 parts) were read by absolute path; phases 4, 5 and 7 had only 6, 13 and
  34 lines of plan, so **their asks of the Hub are not known**. This plan satisfies the contract's
  §0 table for them and gives each an S0 start line; any ask they raise later is additive.
- **Phase 8 task 16** (`internal/entra`) and **phase 3 task 13** (`credstore.go`) are plans, not
  code. Tasks 10 and 29 write them from those parts verbatim only if they have not landed.
- **Docker Desktop's reachability** of a host service bound to `127.0.0.1` from a container was not
  tested; the local caller key (task 37) makes the answer not matter, as ADR-H1 §5.4 intends.
- **Tauri 2 app-defined commands need no capability entry** (task 38): inferred from phase 2's
  `greet`, which has none; checked only when task 38 runs `npm run tauri dev`.
- **Token shapes**: GitHub's documented prefixes (`ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_`,
  `github_pat_`) from knowledge; Panacea's `pnc_` from the phase 7 brief (`:119`), its length bound
  a guess (task 39).
- **`docker compose up --wait`** on the owner's Docker Desktop version (task 09 falls back to
  polling the health status).
- **The Azure DevOps resource id** `499b84ac-1321-427f-aa17-267ca6975798` is the contract's
  (§6.2, §15), not checked; `PANACEA_APP_ENTRA_DEVOPS` defaults to `false` until IT grants consent.
- **Two golden vectors of the contract were re-computed** for this plan (Python `json.dumps` with
  `sort_keys`, `separators=(",",":")`, `ensure_ascii=False`): both lengths (209, 256) and both
  digests agree with contract §3.5. Vectors 3 and 4 (task 02) were computed the same way. That is
  the only code this plan ran.

