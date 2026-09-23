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
    `go test -tags integration ./control-plane/internal/hub/... ./control-plane/internal/hubclient/... -race -count=1`
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
built tree and the sibling plans found **eight** places where the contract, as written, would fail
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

