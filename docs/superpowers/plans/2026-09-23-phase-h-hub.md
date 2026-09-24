# Phase H: The Hub — shared server, sign-in, and the credential store

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status:** plan, 2026-09-23. Written from `docs/superpowers/adr/ADR-H1-hub-architecture.md` and
`docs/superpowers/contracts/2026-09-23-hub-contract.md` on branch `feat/hub`. Owner-reviewable;
adversarially reviewed before code. `OD-1`'s prototype waiver names phases 3, 4, 5 and 7, not
phase H, so the two UI surfaces get a prototype first (owner question Q13, review M14). **Nothing in this plan was compiled or run.** Where it
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
`golang.org/x/time/rate` and `golang.org/x/sys/windows` promoted from indirect to direct. pgx brings
five `golang.org/x` modules with it, **three bumped and two added** (all indirect): x/text v0.24.0,
x/sys v0.32.0 and x/term v0.31.0 are bumped (from `control-plane/go.mod:54-56`'s v0.16.0, v0.24.0 and
v0.21.0); x/crypto v0.37.0 and x/sync v0.13.0 are new lines (pgx requires x/crypto, x/text and
x/sync; x/crypto v0.37.0 requires x/sys and x/term). x/sys v0.32.0 and x/term v0.31.0 are not in the
module cache, so task 06's `go get` may need the network. x/sys and x/time become direct (tasks 28 and
13) **at whatever version the graph then holds**, by an import and `go mod tidy`; no task names an
x/ version, because a lower one would downgrade x/crypto and pgx (review I8, plan re-review b03 m2).
Postgres 16
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
| `OD-1` | phases 3, 4, 5 and 7 build without the prototype gate; every plan is reviewed; UI branches stay unmerged until the owner has looked (`owner-decisions.md:12`) | the waiver does **not** name phase H, and tasks 41 and 42 had no prototype: task 41's Step 0 writes `prototype/phase-h-hub.html` first (owner question Q13, review M14). **`feat/hub` is not merged onward until the owner has looked** at both surfaces |
| `OD-3` | briefs, approvals and the ledger are shared from day one | the Hub exists at all; tasks 13–27 |
| `OD-4` | Azure DevOps **and** GitHub | `project_remotes.host` has both values (task 17); the credential store serves both (tasks 28, 29, 31, 39) |
| `OD-5` | our own server side, in this monorepo | the Hub is `control-plane/cmd/hub`; the environment is phase 8's |
| `OD-6` | Entra, auth-code + PKCE, tokens in Windows Credential Manager, same store for ADO and GitHub | tasks 28–32; the credential-store seam (§"Seams", S-CRED) |
| `OD-8` | harness transcript rule | not touched: the Hub never sees a terminal byte or a transcript (ADR-H1 §1.3) |
| `OD-11` | permissive licences only | `pgx/v5` and its three `jackc` requirements are MIT; `golang.org/x/crypto` and `x/sync` arrive indirectly with pgx and are BSD-3-Clause, like the x/ modules already in `control-plane/go.mod`; nothing else is added |
| `OD-12` | owner hands-on steps batched to the end | tasks 41 and 42's visual acceptance and task 43's real-tenant sign-in stay **unverified** until then |
| `OD-13` | Panacea stays a separate API | the Panacea token is phase 7's, kept in this phase's store (S-CRED); the Hub stores no Panacea credential |
| ADR-H1 §4 | Design A + amendment (Hub in the control-plane module) + grafts G1–G6 | the file map; G1 = task 23, G2 = task 21, G3 = tasks 14 and 26, G4 = tasks 07a (the grants) and 07b (the guards), G5 = tasks 14 and 27, G6 = task 27 |
| ADR-H1 §5.2 | direct Entra, no broker; the control plane holds tokens | tasks 30–32 |
| ADR-H1 §5.4 | local caller key on the control-plane hop. **A declared departure** (review I3): §5.4 exempts the WebSocket attach (`ADR-H1-hub-architecture.md:451`); this plan does not, and carries the key as the attach's second subprotocol. Only the two health routes go without it | tasks 37–38 |
| ADR-H1 §5.3 invite row, §9 item 3 | an invite binds by `preferred_username` or `email` (`ADR-H1-hub-architecture.md:406`); the Hub API's optional claim is `email` (`:725`). **Amended in place** (A15, review I12; plan re-review b05 N1, N3): binding by `bindEmail`, a member's or a verified domain's `email` claim only, and optional claims `email`, `acct`, `xms_edov`. The ADR's decision (Q4) stands; owner question Q12 | task 44a (the edit); tasks 13, 16a (the rule); task 43 R-4 (the real claims) |
| ADR-H1 §10 Q1–Q7 | open owner questions, each with a recommendation | this plan builds every recommendation (§"Open questions") |

---

## Global constraints

Every task restates the ones it touches. None of them moves.

- **Module and paths.** Go module `github.com/kpmg-au-emu/panacea-app/control-plane`
  (`control-plane/go.mod:1`). `go.work` lists `./control-plane` and `./sandboxd` and **gains no
  `use` line**: the Hub is a second `main` package in the existing module (ADR-H1 §4.1). Its `go`
  line follows control-plane's to `go 1.23.0` in task 06, and task 06 commits it (see "The `go`
  directive"; plan re-review b03 N2).
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
  above `1.23.0` is not. **`go.work:1` must follow it to `go 1.23.0`** in the same commit: since Go
  1.21 a workspace whose `go` line is below a member's refuses to load, and `go 1.23` sorts below
  `1.23.0` (probed 2026-09-24 on a scratch workspace, go1.26.2, `GOTOOLCHAIN=local`: *"module a
  listed in go.work file requires go >= 1.23.0, but go.work lists go 1.23"*, exit 1; `go work edit
  -go=1.23.0` → exit 0). Left uncommitted, the implementer's tree works and every other checkout and
  CI fails at the first root `go` command (plan re-review b03 N2). `sandboxd/go.mod:3` stays `go 1.23`. pgx `v5.10.0` (also cached) declares `go 1.25.0` and **must not be used**.
  The x/ modules pgx pulls up (see "Tech stack") move too: x/crypto v0.37.0, x/text v0.24.0 and
  x/sync v0.13.0 also declare `go 1.23.0` (read from the cache, 2026-09-24); x/sys v0.32.0's and
  x/term v0.31.0's directives are **unverified**, because neither is cached, and task 06's diff check
  is what catches one above `1.23.0`. No task pins an x/ module below what the graph selects: that
  would downgrade pgx.
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
  - `httpx` imports only the standard library (`control-plane/internal/httpx/problem.go:5`; its imports are `:10-18`).
  - **No `GET` on the control plane changes state** (`control-plane/internal/api/api.go:15-21`).
    Every new `GET` is added to `catalogReadOnlyRoutes`
    (`control-plane/internal/api/routes_audit_test.go:32`).
- **UI.** Tasks 41 and 42 read `docs/DESIGN-SYSTEM.md` first and hold to its non-negotiables. The
  **owner's eyes are the only visual acceptance**, batched to the end (`OD-12`); every UI claim in
  a report says *unverified by the owner*.

---

## Where this plan corrects the Hub contract — the "Hub contract asks"

The contract says that where it and the ADR disagree, the ADR wins (contract `:4-5`). Reading the
built tree and the sibling plans, and then this plan's review, found **nineteen** places where the
contract, as written, would fail when built (A18 and A19 are the review's M10 and M11). Each is a correction this plan builds, with the exact shape; none adds a Hub route.
They are folded into the contract's text by **task 44a, which runs first** — on `feat/hub`, before
task 01 and before any lane branches — so no sibling builds against the text this table calls
wrong (review finding I11). The sibling phases' own asks of the Hub, and this plan's answer to
each, are task 00's; 44a folds those in too.

| # | Contract says | Why it fails | This plan builds | Task |
|---|---|---|---|---|
| **A1** | `repoKey` for Azure DevOps matches `^[a-z0-9-]{1,50}\.visualstudio\.com/[^/]{1,64}/[^/]{1,64}$` (§7.4) | phase 3's `githost.RepoKey()` returns `azure-devops/<org>/<project>/<repo>`, lowercased (phase 3 task 13 `:140-143` on `feat/phase-3-git`), and phase 3's own review ruled that form the right one because `dev.azure.com/<org>` and `<org>.visualstudio.com` are one organisation (phase 3 review I10, `:350-356`). The contract's regex would refuse every Azure DevOps key phase 3 sends | `github`: `^github\.com/[a-z0-9-]{1,39}/[a-z0-9._-]{1,100}$`; `azure-devops`: `^azure-devops/[a-z0-9-]{1,50}/[^/\x00-\x1f\x7f]{1,64}/[^/\x00-\x1f\x7f]{1,64}$` (task 01: RE2's bare `[^/]` matches a newline, so `…/repo\n` was a second spelling); **and** `repoKey == strings.ToLower(repoKey)` for both — the Hub refuses a non-canonical key rather than storing two spellings of one repository | 01, 17 |
| **A2** | `compose.yaml` runs `db`, `devissuer`, `migrate` and `hub` (§12.1; ADR-H1 §5.7) | three independent faults. (1) Building a Go binary inside `docker build` fails behind the corporate proxy (`images/sandbox/Dockerfile:3-6`). (2) `entra.Config.AllowInsecureAuthority` permits **only a loopback** `http` authority (contract §6.3; phase 8 task 16 `TestAnInsecureAuthorityIsRefused`), so a Hub in a container cannot validate against `http://devissuer:8740`. (3) The token's `iss` must be the same string for the browser, the control plane and the Hub; inside a bridge network the Hub cannot see the host's `127.0.0.1:8740` | `compose.yaml` runs **Postgres only**, on `127.0.0.1:5433`. `devissuer`, `hub migrate` and `hub serve` run as host processes, built and started by `control-plane/cmd/hub/devstack.sh up`, stopped by `down` (by PID, never by name). Every variable and port in contract §12.1 is unchanged | 09 |
| **A3** | "every route except `/healthz`" is behind the local key (§9.4, §13 C3) | the control plane's health route is `GET /v1/healthz` and `GET /api/v1/healthz`, mounted outside the guard (`control-plane/internal/api/api.go:358-361` on `feat/hub`; `:303-306` before the phase-2 merge); there is no `/healthz` | the only routes without the key are those two patterns, exempt by where they are mounted; `GET /v1/sessions/{id}/attach` (`attach.go:42`) is **not** exempt: it carries the key as its second WebSocket subprotocol (review I3) | 37 |
| **A4** | the desktop sends `Authorization: Bearer <local key>` (§9.4) | the CORS preflight answers `Access-Control-Allow-Headers: Content-Type` only (`control-plane/internal/httpx/origin.go:168-171`). With an `Authorization` header every request, `GET` included, is preflighted, and the preflight refuses it: **no call would succeed** | `corsHeaders = "Content-Type, Authorization"`; the local-key middleware sits **inside** the origin middleware, so an allowed-origin `OPTIONS` is still answered 204 before any key check (`origin.go:231-237`) | 37 |
| **A5** | `devissuer` is `control-plane/cmd/devissuer`, standard library only (§2) | a `main` package cannot be imported, so task 30's tests could not run the real issuer in-process and would need a second, hand-written fake OIDC server | the issuer is a package, `control-plane/internal/devissuer` (standard library only), and `cmd/devissuer/main.go` is a ~60-line `main` over it. Test code may import it; the boundary test forbids `cmd/api` and `cmd/hub` from linking it | 11, 12 |
| **A6** | the control plane validates the `id_token`'s signature against `jwks_uri` (§6.1 step 9) | it needs a second RS256-over-JWKS implementation in the control plane beside `internal/entra`, which validates **access** tokens for a resource API and cannot validate an `id_token` (no `scp`, a different `aud`). Two copies of key-set code drift | the `id_token` arrives on the back channel, from the token endpoint the control plane itself called, so OIDC Core 1.0 §3.1.3.7 item 6 permits TLS server validation in place of the signature check. `entraauth` checks `iss`, `aud`, `nonce`, `exp`, `tid` and `oid` **without** a signature, and then cross-checks: `GET /v1/me` (step 11), whose access token the Hub validates in full, must answer an org whose `tenantId` equals the `id_token`'s `tid`, or the sign-in fails `exchange_failed` and nothing is stored | 30 |
| **A7** | the Hub maps `entra` rejections to 401 and says nothing of a key-set outage (§4.2, §6.3) | phase 8's `entra` adds `ErrKeySetUnavailable` (phase 8 task 16 `:102-106`); its `Middleware` answers that 503 `backend_unavailable`, which is a control-plane code meaning Docker (`problem.go:60`) | the Hub calls `Validator.Validate` itself in `internal/hub/server/auth.go` and maps `ErrKeySetUnavailable` to 503 **`unavailable`**; `/readyz` calls `JWKSValidator.Ready` | 13 |
| **A8** | the credential store has target names for a GitHub and a Panacea token (§9.1) but **no way to put one there** | nothing in the contract writes `PanaceaApp/git/github/…` or `PanaceaApp/panacea/…`; phase 3 would read an always-empty store and phase 7 would build its own write path into the same store | three **control-plane** routes (not Hub routes), write-only for the secret: `POST /v1/credentials/{kind}`, `DELETE /v1/credentials/{kind}?scope=`, `GET /v1/credentials` (metadata only) — `kind` ∈ `github`, `azure-devops`, `panacea`. `azure-devops` (review I10) is a PAT per organisation at `PanaceaApp/git/azure-devops/<org>`, read by `credstore.LocalDevOps` with `Scheme: BasicPAT`, so OD-4's ADO path has a credential in the one store (OD-6) before ADR-H1 Q5 is answered. Phase 7 uses these instead of writing its own (seam S-CRED) | 28, 29, 39 |
| **A9** | `hubapi` imports the standard library only; `hubclient` imports `hubapi` and the standard library (§2) | the same contract has `hubapi/codes.go` **import** the shared codes from `httpx` (§4.2), `hubclient.Error` carry an `httpx.Problem`, and `hubclient` mint idempotency keys with `ids.UUID()` (§9.2). Built as written, the boundary test fails the contract's own code | the boundary allows `hubapi` → `internal/httpx`; `hubclient` → `hubapi`, `internal/httpx`, `internal/ids`. Both `httpx` and `ids` import only the standard library, so the Hub still links nothing of the control plane's | 01, 09 |
| **A10** | the audit envelope covers `v, kind, orgId, seq, at, actorUserId, event, projectId, subject, data` (§3.6) | `audit_events.team_id` is not in it, yet the change feed decides who sees an event by `team_id` or `project_id` (§7.11). An owner-role edit of `team_id` would move an event into another team's feed **without breaking the chain** | the envelope adds `"teamId"` (or `""`). `hub verify` also checks each row's `feed` flag against the kind table of §8, since `feed` is derived from `kind`. Golden vectors 3 and 4 (task 02) are computed with it | 02, 14, 27 |
| **A11** | the refresh token lives at `…/refresh/0` … `/refresh/<n−1>`; the account record carries `refreshChunks` and is *"written **last**, so it marks a complete write"* (§9.1, `hub-contract.md:1172-1173`) | the record marks a complete write only at the **first** sign-in. On rotation the record already exists: chunk 0 is overwritten in place, and if chunk 1's `Put` fails, or the process dies, the store holds new chunk 0 + old chunk 1 behind a valid record — neither token. §6.2's *"a crash in between leaves the old one"* breaks exactly for a multi-chunk (enterprise-sized, R1) token (review I1) | every write is a **generation**: chunks at `PanaceaApp/entra/<tenantId>/<clientId>/refresh/<gen>/<i>`, `gen` 16 lowercase hex from `crypto/rand`; the record adds `"refreshGen"` beside `"refreshChunks"` and is written last, naming the new generation; only then is the old generation deleted; start-up deletes every generation the record does not name; sign-out deletes the record first. Nothing the record names is ever overwritten. Start-up validates the stored `refreshGen` (16 lowercase hex) and `refreshChunks` (1 to 16) before any target is built from them, and treats a bad record as a missing one; a read error that is not `ErrNotFound` deletes nothing (plan re-review b03 N1, m3) | 28, 30, 31 |
| **A12** | `invalid_grant` or `interaction_required` → both Credential Manager entries deleted, `signed_out` with `reason: "reauth_required"` (§6.2, `hub-contract.md:408-409`) | one refresh token redeems the Hub, the broker and, if Q5 is yes, Azure DevOps (§6.2 `:404-407`). A refusal for **one** resource is not a dead refresh token: an unconsented resource answers `invalid_grant` with `suberror: consent_required` (AADSTS65001), and a conditional-access policy on one resource answers `interaction_required` (AADSTS50076/50079) for it alone (from knowledge, unverified). The scope-blind rule deletes the credential the Hub still redeems, so a policy on Azure DevOps signs the user out at every push, and the next sign-in, asking only for `Hub.Access`, sets up the same loop (review I2) | a refusal is read by the scope redeemed. For `HubScope`: the contract's sign-out, unchanged. For any other scope: consent → `ErrScopeNeedsConsent`, interaction (or `invalid_grant` with 50076/50079) → `ErrScopeRefused`, each wrapping a `*ScopeRefusal`; nothing is deleted, only that scope's cached token is dropped, `Status` is unchanged. A plain `invalid_grant` on another scope is settled by one `HubScope` redemption under the same lock: the Hub refuses too → sign-out; it redeems → `ErrScopeRefused`; it answers 5xx → `ErrEntraUnavailable`, nothing deleted. `invalid_scope` (AADSTS70011) on another scope → `ErrScopeNeedsConsent`. **Any other 4xx, on any scope** (`invalid_client`, `unauthorized_client`, `invalid_request`, `invalid_scope` on `HubScope`, an unknown code) → an error naming the code, **nothing deleted**, `Status` unchanged: only `invalid_grant` and `interaction_required` for `HubScope` prove the refresh token dead, so a misconfigured client never costs a sign-in. A `*ScopeRefusal` is remembered per scope until `refreshGen` changes, `Invalidate(scope)`, or 15 min on `Config.Now` (re-review b04 m2, m4). `entraauth.DevOps` returns both refusals as stopping errors, never `ErrNoCredential`, by phase 3's `Chain` rule (a refusal must not silently become another credential) | 31, 43 |
| **A13** | the cached access token is served while `exp − now > 5 min` (§6.2, `hub-contract.md:397-398`) | any token whose lifetime is 5 minutes or less (a short `devissuer` `AccessTTL`; a tenant token-lifetime policy) is never served from the cache: **every** `Token` call redeems and rotates the refresh token. Task 43's end-to-end test, with a 90-second token, could not count "one refresh grant" (review I9) | the skew is `min(5 min, lifetime / 2)`, `lifetime` = `exp` − the `Config.Now()` at which the token response arrived; an hour's token is still refreshed 5 minutes early. Every time comes from the injected `Config.Now`, never `time.Now` | 31, 43 |
| **A14** | the per-IP key is *"the gateway's `X-Forwarded-For` first hop, only when `PANACEA_APP_HUB_TRUST_PROXY=true`"* (§11.3, `hub-contract.md:2091`; §12.2 `:2165`, default `false`) | an ingress **appends** to `X-Forwarded-For`, so the first (left-most) entry is whatever the client sent. A made-up value per request is a fresh bucket, and the per-IP limit — the one that runs **before** token validation to bound the cost of bad tokens — is gone behind phase 8's ingress (review I7) | `PANACEA_APP_HUB_TRUSTED_HOPS`, an integer in [0, 5], default `0` (the header ignored; the peer address is the key — the local-first path, which has no proxy). With N ≥ 1, every `X-Forwarded-For` line is joined in order and the key is the N-th entry from the **right**; fewer than N entries, or one that is not an IP, and the peer address is the key. Phase 8's single ingress sets `1`. A set `TRUST_PROXY` is refused by name, so an old deployment fails loudly. One variable, not `TRUST_PROXY` plus a hop count, so the two cannot disagree. **Sibling ask for phase 8** (plan re-review b05 N2): its chart still sets the retired name — `PANACEA_APP_HUB_TRUST_PROXY=true` (`worktrees/env` `…/phase-8-own-environment-parts/task-29.md:134`), asserted by its test `hub / env is the contract's deployed column` (`:259`, *"`TRUST_PROXY=true`"*). Both become `PANACEA_APP_HUB_TRUSTED_HOPS=1`. Kept, the Hub refuses to start; deleted without the replacement, the key is the ingress pod's address and every client shares one pre-auth bucket | 08, 13 |
| **A15** | *"`email` is the token's `email` claim if present, else `preferred_username`, lowercased. It is used for one thing: binding invites"* (§5.1, `hub-contract.md:317-318`); binding matches it (§7.2 `:655-657`); the Hub API's optional claims are `email` only (§6.5 `:542`) | both claims are mutable and, for a B2B guest, not verified by the tenant; Microsoft says not to authorise by either. An invite is a grant, and team membership is the only authorisation inside an org (T4). Binding stays in the org, which rules out the cross-tenant case but not a guest or a renamed account in the same tenant, nor the multi-tenant tier (S-BYO) (review I12) | binding reads a separate `BindEmail`: the `email` claim alone, lowercased, and only when the token shows `acct` = the JSON number `0` (a member) or `xms_edov` = the JSON boolean `true` (email domain owner verified); otherwise `""` and nothing binds. Never `preferred_username`, never the stored `users.email`; §5.1's `email` stays, as display text only. §6.5's optional claims (access token) become `email`, `acct`, `xms_edov`. `devissuer` emits both claims (members `acct: 0`), so every matrix column still binds locally. The Hub reads the two claims in `auth.go` (`bindingClaims`) from the token `Validate` has just accepted, because `entra.Claims` (phase 8's package) carries neither — the seam. **Sibling ask for phase 8:** add `Member bool` (`acct == 0`) and `EmailVerified bool` (`xms_edov == true`) to `entra.Claims`, both failing closed; `bindingClaims` then reads them. **Second sibling ask for phase 8** (plan re-review b05 N1): `acct` and `xms_edov` are optional claims, which Entra emits only once the registration configures them, and phase 8's Terraform configures `email` alone (`worktrees/env` `…/phase-8-own-environment-parts/task-09.md:44`, `:136-137`). Its Hub registration's `optional_claims.access_token` gains `acct` and `xms_edov` beside `email`, and its test `email_is_an_optional_claim_on_the_hub` (`:170`) asserts all three. Without them every deployed token fails closed, and no invite ever binds with no error — the silent-override class. **ADR-H1, amended in two places by task 44a** (owner question Q12): §9 item 3, IT's registration spec (`ADR-H1-hub-architecture.md:725`, *"Optional claim **`email`**"*), names all three claims; §5.3's invite row (`:406`, *"a matching `preferred_username` or `email`"*) names `bindEmail`. The contract says the ADR wins where the two disagree (`hub-contract.md:4-5`), so leaving `:406` unedited would make the fallback I12 removes the governing text | 11, 13, 16a, 43, 44a |
| **A16** | every link is a single-column foreign key: `team_id … REFERENCES teams (id)` (§10.2, `hub-contract.md:1501`, `:1513`, `:1532`, and every other link); the digest target is `brief_revisions (brief_id, revision, digest)` (§10.7 `:2044`); tenancy is *"row-level security"* (§10.7 `:2049`) | row-level security checks only a row's **own** `org_id` (§10.3 `:1907-1909`), and Postgres checks a foreign key without row-level security. So a `hub_app` insert whose `org_id` is the caller's org but whose `team_id`, `brief_id` or `pin_brief_id` names another org's row succeeds. The contract's tenancy line holds for reads, not for links, and the `X` column's *"Go check disabled"* (§11.1 T3) is undefined, so it could hide the hole (review I5) | every parent table gains `UNIQUE (org_id, id)` (`project_blobs (org_id, project_id, digest)`, `design_approvals (org_id, project_id, design_key)`, `brief_revisions (org_id, brief_id, revision)` and `(org_id, brief_id, revision, digest)` for the composite targets), and every foreign key except the one to `orgs` carries `org_id`: `FOREIGN KEY (org_id, team_id) REFERENCES teams (org_id, id)`, and the same for users, projects, briefs, revisions, blobs, approvals and releases. The database then refuses a cross-org link (`23503`) with every Go check off. Two task 07a tests pin it: the catalog says every key pairs `org_id` with `org_id`, and one `INSERT` per key naming the other org's parent is refused. `disableGoAuthz` is defined: authorisation predicates answer "allowed"; lookups still run under row-level security | 07a, 16b, 17 |
| **A17** | `erase_brief` removes the revisions' content and archives the head; *"approvals, releases, evidence and session links are **not** changed"* (§7.12 `:1113`, §10.4 `:1969`) | each route-26 response is stored whole in `idempotency_keys.response` for 24 hours (§10.6 `:2030`, task 24), and `WriteRevisionsResponse` carries every revision's `content`. So the text survives erasure for a day, and a backup taken that day keeps it for the backup's whole life (review I6) | Route 26 **stores no text**: `idempotency_keys.response` keeps its reply with every `content` null, and a replay rebuilds each `content` and `erasedAt` from `brief_revisions` (task 14's `WriteReplayable`, task 19's `rebuildRevisions`). A replay is the first answer byte for byte until an erasure; after one, the erased revision replays with `content: null`, as `/v1/refs` answers, and `erase_brief` deletes nothing from `idempotency_keys`. This is the review's second option. **Not a purge**, by path or by brief id: the brief id is in far more stored replies than route 26's (every `briefKey`, and so approvals, evidence, releases, heads, and the erase's own reply), and a batch names several briefs. A purge deleted all those text-free replays, so a retry became a second write (a second, permanent evidence row) or a refusal of a success (`409 brief_archived` for a co-batched brief whose revision was written), and it was an unindexed byte scan under the org's lock (re-review b06 N1, m2). Tasks 07b, 19 and 27 pin that the replays survive; task 27's `TestErasureKeepsKeyAndDigest` scans every `text`, `jsonb` and `bytea` column for a marker from the erased content, before and after | 03, 07b, 14, 19, 24, 27 |
| **A18** | row-level security on every table **but `orgs`** (§10.3, `hub-contract.md:1902-1905`), and `GRANT SELECT ON ALL TABLES IN SCHEMA hub TO hub_app` (§10.5 `:1997`); the janitor and `hub verify` visit "every org" (§12.5 `:2223`, §12.1 `:2123`) | any `hub_app` query path, with or without `hub.org_id`, reads every enrolled tenant's org id and display name (review M10). And once `orgs` has a policy, the two callers that list every org on `hub_app`'s connection **before** a `set_config` see none: the janitor would sweep nothing and `hub verify` would pass on an empty walk | `orgs` gets its own policy, `USING (id = current_setting('hub.org_id', true))`, no `WITH CHECK` (`hub_app` may only `SELECT` it). `ensure_org` and `resolve_principal` run as the table's owner and are unaffected. A new `SECURITY DEFINER` function **`hub.org_ids() RETURNS SETOF text`**, the ids only, is how the janitor and `hub verify` list orgs | 07a, 07b, 20, 24, 27 |
| **A19** | `audit_append` runs as the invoker (§10.2 `:1881`), so `hub_app` holds `UPDATE (seq, hash)` on `audit_heads` (§10.5 `:2014`); the write lock is `SELECT seq, hash FROM hub.audit_heads WHERE org_id = $org FOR UPDATE` (§10.6 `:2026`) | a compromised Hub can move an org's head directly, a gap `hub verify` finds only later (review M11). Revoking the grant alone breaks §10.6: `SELECT … FOR UPDATE` needs `UPDATE` privilege on at least one column (from knowledge; task 07b's `TestTheHeadLockNeedsTheFunction` pins it), so no write could take its lock | `audit_append` is `SECURITY DEFINER` and checks `NEW.org_id` against `hub.org_id` **before** it locks anything (as the owner it bypasses row-level security, so without the check an org-A session could lock org B's head); `hub_app` has **no** `UPDATE` on `audit_heads`; §10.6's lock is `SELECT seq, hash FROM hub.lock_audit_head()`, a `SECURITY DEFINER` function with no parameter that locks the session's org only | 07a, 07b, 14, 24 |

**Additions, not corrections** — shapes the contract did not name, which this plan adds and task
44a folds in first. None is a Hub route. The sibling asks' answers (task 00) are listed there, not
here.

| Addition | Exact shape | Why | Task |
|---|---|---|---|
| `Panacea-Auth-State` response header on the control plane, exposed to the webview beside `Panacea-Hub-Cursor` | `signed_out` \| `signing_in` \| `signed_in` | the desktop learns a sign-in finished, or a refresh died, from the poll it already makes — phase 2's one-request-per-tick rule (`poller.ts:1-3`) | 36, 40 |
| `hubclient` gains `Teams`, `Projects.Project`/`AddRemote`, `Designs.Approval`/`SessionLinks`, `Releases.Release`, `ErrSignedOut`, `ErrNotConfigured`, `ErrTokenUnavailable`, `Invalidator`, `WriteError`, `VerifyRevision`, `FormatRecorder` | task 04, 31, 33, 34's contracts | each wraps a route §7 already defines, or names a rule §4.3/§9.2 already states | 04, 31, 33, 34 |
| `entraauth.Config.AfterSignIn` returns `(hubTenantID string, err error)` | was `error` only | correction A6's cross-check needs the Hub org's tenant | 30 |
| Refresh redemptions serialised across **all** scopes | one lock; single-flight per scope under it | one refresh token, rotated on use, redeemed twice at once would sign the user out | 31 |
| An overdue `reserved` release **reads** `expired` before the janitor marks it | rendered state only | the fake and the Hub agree through the interface; no caller confirms a dead reservation | 23 |
| Control-plane `hub-link` and `hub/teams` routes | task 35's table | the share flow (Q8): without it no local project becomes a Hub project | 35, 42 |
| `server.Patterns()` | `func (s *Server) Patterns() []string` | the matrix meta-test and the conformance harness read the real route table, never a list | 08, 16b, 34 |

### Files that belong to a sibling phase — "whichever lands first", and the owner wins

Three packages this plan needs are **owned by sibling plans** that may land before or after it.
Each task that needs one runs a preflight: if the file exists, it is used as it is; if not, the task
writes it from the owning part's contract, so the two copies share one design. **The later merge is
not a no-op** (review M5). Each owning part fixes the declarations only: phase 8 task 16 and phase 3
task 13 give `func` signatures with no bodies (checked on `feat/env` at `aa12b01` and
`feat/phase-3-git` at `a99d20b`). Two implementers therefore write two bodies at one path, and the
merge is an add/add conflict. **The owner phase wins every such conflict:** the orchestrator keeps
the owner's file whole, drops this plan's copy, and re-runs this plan's tests against the owner's
file. A test of ours that fails on the owner's file is a finding against our copy, not against
theirs.

| Package | Owner (wins a conflict) | This plan's task | The owning part | Where this plan's copy differs from the part |
|---|---|---|---|---|
| `control-plane/internal/entra` | phase 8 task 16 | 10 | sibling `feat/env`, `…/2026-09-23-phase-8-own-environment-parts/task-16.md` | the bodies only |
| `control-plane/internal/credstore/credstore.go` | phase 3 task 13 | 29 | sibling `feat/phase-3-git`, `…/2026-09-23-phase-3-git-convergence-parts/task-13.md` | the bodies; phase 3's package comment is dropped (task 28's `secrets.go` holds the package's one); `Credential.Source`'s comment reads `"credential-manager" \| "entra" \| "git-credential-helper"` where phase 3's reads `"hub" \| "git-credential-helper"` (phase 3 `task-13.md:68`) |
| `control-plane/internal/githost/remote.go` | phase 3 task 13 | 35 | the same part | the bodies only |

**Sibling ask for phase 3** (review M5), one line: *adopt Hub contract §9.1's `Credential.Source`
values — `"credential-manager" | "entra" | "git-credential-helper"`, not `"hub"` — in `credstore.go`'s
comment at `task-13.md:68`, because the Hub server holds no git credential; this plan's stores
report `credential-manager` (task 29) and `entra` (task 31).* Until phase 3 adopts it, its file still wins a conflict,
and the only loss is a stale comment: `Source` is a plain `string`, so no code depends on the list.

Two further facts, verified here, **close** items the contract and ADR left open:

- **`opener:default` permits `http://` and `https://` URLs** — its generated description reads
  *"This permission set allows opening `mailto:`, `tel:`, `https://` and `http://` urls using their
  default application"*, with `allow-open-url` and `allow-default-urls` included (main tree,
  `desktop/src-tauri/gen/schemas/desktop-schema.json`, read 2026-09-23; this worktree has no
  generated schema). So sign-in needs **no capability change** (closes ADR-H1 §11, contract §15
  last bullet).
- **`golang.org/x/sys/windows` has no Credential Manager wrappers** — `grep -l "CredWrite\|CredRead"`
  over `x/sys@{v0.24.0,v0.39.0,v0.47.0}/windows/*.go` in the module cache finds nothing (2026-09-24).
  The version actually built after task 06 is v0.32.0 or higher, which is not cached and so not
  grepped here; task 28 Step 2 greps it before writing the calls. Task 28 declares
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
| `control-plane/internal/hub/hubclient/hubclient.go` | the consumer interfaces, `Error`, `ErrUnavailable`, `ErrSignedOut`, `TokenSource`, the optional `Invalidator` (moved here from task 31, review M15) | new | 04 |
| `control-plane/internal/hub/hubclient/fake*.go` | `NewFake()` — every observable rule of §7, in memory | new | 04, 05 |
| `control-plane/internal/hub/hubclient/conformance/conformance.go` | one table-driven suite for the fake and the real Hub; rows grow per task | new | 05; rows 15–27, 34 |
| `control-plane/internal/hub/hubclient/{client,transport,relay}.go` | the HTTP client, its retry/idempotency/refresh rules, the relay rule (contract §4.3) | new | 33 |
| `control-plane/internal/hub/hubclient/methods_*.go` | every interface method over HTTP, with the digest re-check | new | 34 |
| `control-plane/internal/hub/migrate/{migrate.go,roles.sql,roles_local.sql}` | the runner under an advisory lock; the per-environment roles | new | 06 |
| `control-plane/internal/hub/hubtest/hubtest.go` | integration harness: a scratch database per test, migrated, torn down | new | 06 |
| `control-plane/internal/hub/migrate/sql/0001_init.sql` | the complete first schema, guards, RLS, owner functions, grants | new | 07a (tables, RLS, grants), 07b (guards, owner functions) |
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
| `control-plane/internal/hub/server/invites.go`, `store/invites.go` | invites and their binding | new | 16a |
| `control-plane/internal/hub/server/matrix_integration_test.go` | the authorisation matrix (contract §5.3), every route × every principal | new | 16b; rows 17–27 |
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
| `control-plane/go.mod`, `go.sum` ⚑ | `pgx/v5 v5.7.5`; three x/ lines bumped and x/crypto, x/sync added (indirect); `go 1.23.0`; `x/time`, `x/sys` direct | modified | 06 (pgx), 13 (`x/time`), 28 (`x/sys`) |
| `go.work` (and `go.work.sum` if written) ⚑ | the `go` line to `go 1.23.0`, following control-plane's; no `use` line added | modified | 06 |

### The control plane — `control-plane/cmd/api`, `control-plane/internal/...`

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `control-plane/internal/credstore/{secrets,memory,chunk,targets}.go` | `Secrets`, the in-memory store, chunking, the target-name grammar | new | 28 |
| `control-plane/internal/credstore/wincred_windows.go`, `wincred_other.go` | Windows Credential Manager over `advapi32.dll`; the non-Windows stub | new | 28 |
| `control-plane/internal/credstore/credstore.go` ⚑ | **phase 3 task 13's file** (`Host`, `Credential`, `Store`, `None`, `Chain`); written here from its contract only if absent, and phase 3's wins the merge (review M5) | new | 29 |
| `control-plane/internal/credstore/{tokens,local}.go` | `GitHubTokens`, `DevOpsTokens`, `PanaceaTokens`, `Inventory`; `Local` and `LocalDevOps` (phase 3's `Store` for GitHub, and for an Azure DevOps PAT, review I10) | new | 29 |
| `control-plane/internal/credstore/credstoretest/credstoretest.go` | the behaviour suite every `Secrets` passes | new | 28 |
| `azure-pipelines.yml` ⚑ | one step: `GOOS=windows go vet ./control-plane/...` | modified | 28 |
| `control-plane/internal/githost/remote.go` ⚑ | **phase 3 task 13's file** (`Remote`, `ParseRemote`, `RepoKey`); written here from its contract only if absent, and phase 3's wins the merge (review M5) | new | 35 |
| `control-plane/internal/entraauth/{entraauth,oidc,pkce,listener,signin,idtoken}.go` | discovery, PKCE, the one-shot listener, the code exchange, A6's `id_token` checks | new | 30 |
| `control-plane/internal/entraauth/{tokens,account,devops}.go` | `TokenSource`, refresh, rotation, sign-out, start-up, scope-aware refusals (A12), the lifetime-aware skew (A13); the Azure DevOps adapter | new | 31 |
| `control-plane/internal/httpx/problem.go` ⚑ | four codes: `signed_out`, `hub_not_configured`, `local_key_required`, and `upstream_unavailable` if R4 has not landed | modified | 32 |
| `control-plane/internal/api/auth.go` | `/v1/auth/*` | new | 32 |
| `control-plane/internal/api/api.go` ⚑ | `Deps` fields; registrations; the local-key wrap; `New` refuses a zero `LocalKey` (review C2); the package comment's "no authentication" paragraph rewritten (37, review M13) | modified | 32, 35, 36, 37, 39 |
| `control-plane/internal/api/attach.go` ⚑ | one field: `AcceptOptions.Subprotocols` echoes `panacea.v1` (review I3) | modified | 37 |
| `control-plane/internal/api/{export,api,attach,sessions}_test.go` ⚑ | `WithTestKey`; `localkey.ForTest()` in each `Deps` builder, the helper at each request site through `New`, nothing else (review C2; the full list of eight files is task 37's) | modified | 37 |
| `control-plane/internal/api/routes_audit_test.go` ⚑ | new `GET`s in `catalogReadOnlyRoutes`; one comment sentence (37, review M13) | modified | 32, 35, 36, 37, 39 |
| `control-plane/cmd/api/{config,main}.go` ⚑ | the new variables; wiring | modified | 32, 35, 36, 37 |
| `control-plane/internal/hublink/hublink.go`, `internal/api/hublink.go` | local project → Hub project; the share routes | new | 35 |
| `control-plane/internal/hubfeed/poller.go`, `internal/api/hubchanges.go`, `internal/api/cursors.go` | the change-feed poll; `CursorSource`, the `Panacea-<Name>-Cursor` and auth-state headers and their expose list (HA7-4, task 00) | new | 36 |
| `control-plane/internal/httpx/origin.go` ⚑ | allow the `Authorization` header (A4), and the comment that said none is sent (review M13) | modified | 37 |
| `control-plane/internal/localkey/localkey.go`, `private_windows.go`, `private_other.go` | the local caller key: mint, write, compare; the key file owner-only, by a protected one-entry DACL on Windows (review M9) and 0600/0700 elsewhere | new | 37 |
| `control-plane/internal/api/credentials.go` | the write-only credential routes (A8) | new | 39 |

### The desktop — `desktop/`

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `desktop/src-tauri/src/lib.rs` ⚑ | `local_key` command | modified | 38 |
| `desktop/src/api/localKey.ts`, `desktop/src/api/index.ts` | fetch the key once, hold it in memory; build `api` with it | new, modified | 38 |
| `desktop/src/api/client.ts` ⚑ | the key on every call, and in the attach socket's subprotocol (`openAttach` becomes async, review I3); auth, hub-link, credential and change functions; new codes, and the relayed Hub codes as `HubProblemCode`, which `ApiError.code` admits (review M7) | modified | 38, 40 |
| `desktop/src/api/attachRegistry.ts` ⚑ | `connect` awaits `openAttach` (review I3); a pending key read counts as in flight, by a monotonic handshake generation (re-review N1); `reasonForCode` takes `ApiError["code"]` (review M7) | modified | 38 |
| `desktop/src/api/problems.ts`, `desktop/src/defects.test.tsx` ⚑ | a comment; the union-count test, 18 → 22, and the Hub union's (review M7) | modified | 38 |
| `desktop/src/api/hubFeed.ts` | the one feed reader and its subscriber registry | new | 40 |
| `desktop/src/api/poller.ts` ⚑, `desktop/src/App.tsx` ⚑ | the header hook after a good tick; one line wiring it | modified | 40 |
| `desktop/src/store/slices/authSlice.ts`, `store/store.ts` ⚑, `store/types.ts` ⚑ | sign-in and share state, never persisted | new, modified | 40, 42 |
| `desktop/src/components/rail/{RailFooter,AccountControl}.tsx`, `rail.css` ⚑ | the sign-in surface | modified, new | 41 |
| `desktop/src/components/share/{SharePopover.tsx,share.css}` | share a project with a team | new | 42 |
| `prototype/phase-h-hub.html` | the static prototype of both surfaces, written before their code (owner question Q13, review M14) | new | 41 |
| `desktop/src/components/menu/ContextMenu.tsx` ⚑, `desktop/src/components/AppShell.tsx` ⚑ | the *Share with a team…* item, shown only when `auth.hubConfigured`; the popover's one mount | modified | 42 |

### Proof and documents

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `control-plane/internal/hub/e2e_integration_test.go` | sign-in through `devissuer`, then every consumer interface against the real Hub | new | 43 |
| `docs/verification/2026-09-23-phase-h-results.md` | what was run, what was observed, what stays unverified | new | 43 |
| `docs/PROGRAM.md`, `docs/superpowers/contracts/2026-09-23-hub-contract.md`, `…/2026-09-18-phase-2-ui-contract.md`, `…/planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md`, `docs/superpowers/adr/ADR-H1-hub-architecture.md` | the amendments ADR-H1 §7.2 names, and A1–A19 plus the additions folded into the contract; in ADR-H1, A15's two cells only (§5.3's invite row, §9 item 3; owner question Q12) | modified | 44 |

---

## Tasks

Each task has its part file, `docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-NN.md`, and
each is sized for one implementer in **1–3 hours**, except six: task 31 at 4.5 (A12's
scope-aware refusals, review I2, A13's lifetime-aware skew, review I9, the `DevOps` source, review
I10, and re-review b04 m4's remembered refusal all change the `TokenSource` itself, so they belong
with it); task 07b at 3.5 (the guards and the five owner functions share one set of seeded
rows, review M4, which split task 07 into 07a and 07b); task 43 at 5 (review M4: a 12-step
end-to-end test plus every other task's acceptance block); task 38 at 3.5 (its pending-read guards, re-review b02 N1, are about
30 minutes and belong with the `await` that creates the need); and tasks 14 and 19 at 3.5 each
(the replay that stores no brief text, re-review b06 N1: `WriteReplayable` belongs with the write
shape it extends, and the stored form and its rebuild with the route that writes the text). Hours assume the implementer has read ADR-H1,
the contract, and the part; they are the plan's estimate, not a measurement. Contract task ids
(H0–H7, C1–C4) are given so the two documents can be read side by side.

### The minimum Hub first — and where each sibling phase can start

The minimum is what phases 4, 5 and 7 need to **integrate against a real Hub**, and nothing else.
It is tasks **00 and 44a** (the sibling asks answered, and the contract corrected, before any
code), **01–24** on the server and **28, 30–35, 37, 38** on the client — 37 of the 48 tasks, 39
with the credential pair (29, 39) phase 7 needs. Four start lines fall inside it:

| Start line | After | Who can start | What they can do |
|---|---|---|---|
| **S0 — code against the fake** | 44a, 00, 01–05 (≈ 15 h) | phases **3, 4, 5, 7** | build every Hub-backed handler against `hubclient`'s interfaces and `hubclient.NewFake()`, and run `hubclient/conformance` against the fake. Nothing below S0 changes an interface they code to: task 00 answered every sibling ask (HA4-1…9, HA5-1…5, HA7-1…5) **before** S0 froze, and a later change that is not an added method, field or route reopens task 00 and is announced. An added method is additive only because siblings consume these interfaces and `NewFake()` and never implement them (task 00) |
| **S4 — phase 4 integrates** | 06–20 (server), 28, 30–34 partial, 35 (client) | phase **4** | point its `discoverstore` Hub implementation at the local stack: create briefs, write revisions, resolve keys, with a real sign-in through `devissuer` |
| **S5 — phase 5 integrates** | S4 + 21–22 | phase **5** | upload snapshots, approve designs, write session links, resolve `design:` keys |
| **S7 — phase 7 integrates** | S5 + 23–24, 29, 39 | phase **7** | reserve → deploy → confirm/abandon, rollback, the deployment join; its `pnc_` token in the credential store |
| S3 — phase 3's shared evidence | S4 + 25, 29, 31, 39 | phase **3** | `evidence.Store`'s Hub implementation (`Shared() == true`); push and PR with a token from the store (GitHub pasted; Azure DevOps a pasted PAT, review I10, or the Entra sign-in if Q5 is yes) |

### The task list

| # | Task | Contract id | Depends on | Hours | Min? |
|---|---|---|---|---|---|
| 44a | **first**: A1–A19, the additions and task 00's answers folded into the contract; documents only | — | — | 1.5 | ✓ |
| 00 | S0 absorbs the sibling asks: every HA4/HA5/HA7 ask, its answer, the task that builds it (a planning part; no code of its own) | H0 | — | 0.5 | ✓ |
| 01 | `ids.Wide`; `hubapi` codes, keys, digests, branch refs, repo keys; the contract-vs-code repo-key test | H0 | 44a | 2 | ✓ |
| 02 | `hubapi.Canonical`, the envelopes, `ValidateContent`, the golden vectors | H0 | 01 | 2.5 | ✓ |
| 03 | `hubapi` wire types for every object and request of §7 | H0 | 01 | 2.5 | ✓ |
| 04 | `hubclient` interfaces, `Error`, `ErrUnavailable`; the fake, part 1 (projects, teams, briefs, refs) | H0 | 02, 03 | 3 | ✓ |
| 05 | the fake, part 2 (designs, releases, evidence, feed); the conformance suite — **S0** | H0 | 04 | 3 | ✓ |
| 06 | the migrate runner, `roles.sql`, the integration harness; `pgx` pinned | H0 | — | 3 | ✓ |
| 07a | `sql/0001_init.sql`, part 1: the tables, row-level security (`orgs` too, A18) and the grants, with a database test for each (review M4) | H0 | 06 | 3 | ✓ |
| 07b | `sql/0001_init.sql`, part 2: the guards and the owner functions (`org_ids`, `lock_audit_head`, a `SECURITY DEFINER` `audit_append`: A18, A19), with a database test for each; merged with 07a | H0 | 07a | 3.5 | ✓ |
| 08 | `cmd/hub`: config, `serve`, `migrate`, `verify` shell; the server chassis; the pool | H0 | 01, 06, 07b | 3 | ✓ |
| 09 | the local stack: Postgres-only compose, `devstack.sh`, the README (A2); the import boundary test | H0 | 07b, 08, 12 | 2 | ✓ |
| 10 | `internal/entra` — phase 8 task 16's part, **only if it has not landed**; phase 8's wins the merge | H1 | — | 3 (0 if landed) | ✓ |
| 11 | `devissuer`, part 1: the package, config, discovery, JWKS, token minting; `cmd/devissuer` | H1 | — | 2.5 | ✓ |
| 12 | `devissuer`, part 2: `/authorize`, `/token` (code + PKCE S256, rotating refresh) | H1 | 11 | 3 | ✓ |
| 13 | the Hub's auth pipeline: Origin, rate limits, validation, principal, org enrolment (A7) | H1/H2 | 07b, 08, 10, 11 (review M15) | 3 | ✓ |
| 14 | the write transaction: audit lock, hash chain, idempotency (contract §10.6) | H2/H7 | 02, 13 | 3.5 | ✓ |
| 15 | `/v1/me`, teams, members | H2 | 03, 14 | 3 | ✓ |
| 16a | invites and binding, routes 10–12 (review M4) | H2 | 15 | 2.5 | ✓ |
| 16b | the authorisation matrix harness; its rows for routes 3–12 (review M4) | H2 | 16a | 2.5 | ✓ |
| 17 | projects and remotes (A1) | H3 | 16b | 2.5 | ✓ |
| 18 | brief heads: create, list, for-branch, get, rebind, archive | H3 | 17 | 3 | ✓ |
| 19 | the batch revision write and history; the replay that stores no brief text (A17) | H3 | 18 | 3.5 | ✓ |
| 20 | `/v1/refs`, brief half; `hub verify` re-checks revisions — **S4 (server side)** | H3 | 19 | 2.5 | ✓ |
| 21 | blobs, behind `blobstore` (G2) | H4 | 17 | 3 | ✓ |
| 22 | design approvals, session links, `/v1/refs` design half — **S5 (server side)** | H4 | 20, 21 | 3 | ✓ |
| 23 | releases: reserve and the pin (G1); list and get | H5 | 22 | 3 | ✓ |
| 24 | confirm, abandon, the deployment join; the janitor — **S7 (server side)** | H5 | 23 | 3 | ✓ |
| 25 | evidence (H-2) | H6 | 20 | 1.5 | |
| 26 | the change feed (G3) | H7 | 16b | 2 | |
| 27 | the audit route, erasure (G6), the full `hub verify` | H7 | 26, 19 | 2.5 | |
| 28 | `credstore`: `Secrets`, `Memory`, `WinCred` with chunking, target names | C1 | — | 3 | ✓ |
| 29 | `credstore`: `Local` (GitHub), `LocalDevOps` and `DevOpsTokens` (an Azure DevOps PAT, review I10), `PanaceaTokens`; phase 3's `credstore.go` if absent | C1 | 28 | 2.5 | S7 |
| 30 | `entraauth`: discovery, PKCE, the one-shot listener, the exchange (A6) | C1 | 04 (review M15), 12, 28 | 3 | ✓ |
| 31 | `entraauth`: `TokenSource`, refresh single-flight, rotation, sign-out, start-up; scope-aware refusals (A12), remembered per scope, and the lifetime-aware skew (A13); `DevOps`; implements task 04's `Invalidator` | C1 | 30 (and 04 through it) | 4.5 | ✓ |
| 32 | the control plane's `/v1/auth/*`, its variables, its codes, the wiring | C1 | 31 | 2.5 | ✓ |
| 33 | `hubclient` over HTTP: transport, retries, idempotency, refresh, the relay rule | C2 | 04 | 3 | ✓ |
| 34 | `hubclient` methods, the digest re-check, conformance against the real Hub | C2 | 33, 20 (rows grow with 22–27), 11 (review M15) | 3 | ✓ |
| 35 | `hublink`: local project → Hub project; the share routes | C2 | 32, 34; phase 3 task 13's `githost` (or its copy) | 3 | ✓ |
| 36 | the change-feed poller, `/v1/hub/changes`, the cursor header | C2 | 26, 35 | 2 | |
| 37 | the local caller key, control-plane side (A3, A4); the attach subprotocol (I3); the key file's owner-only DACL on Windows (M9) | C3 | 32 | 3.5 | ✓ |
| 38 | the local caller key, desktop side; the keyed async attach and its pending-read guards (I3, N1) | C3 | 37 | 3.5 | ✓ |
| 39 | the credential routes: GitHub, Azure DevOps and Panacea tokens (A8, review I10) | C1 | 29, 37 | 2 | S7 |
| 40 | the desktop's auth, share and feed client; `authSlice`; `hubFeed` | C4 | 36, 38, 39 | 3 | |
| 41 | the prototype of both surfaces first (Step 0, owner question Q13, review M14); the sign-in surface in the rail footer — **UI** | C4 | 40 | 4 | |
| 42 | share a project with a team — **UI** | C4 | 40, 41 | 3 | |
| 43 | end-to-end proof through `devissuer`; the real-tenant checklist; the results record | all | 01–42 | 5 (review M4) | |
| 44b | last: what the build changed, folded into the contract; the seams handed over | — | 43 | 1 | |

Totals: server 01–27 ≈ 80 h (73.5 before review I5's cross-org key tests added 1 h to task
07, re-review b06 N1's replay that stores no brief text added 0.5 h each to tasks 14 and 19, and
review M4's split of task 07, 4 h, into 07a and 07b, 6.5 h, and of task 16, 3 h, into 16a and
16b, 5 h, added 4.5 h);
client and desktop 28–42 ≈ 45.5 h (39.5 before review I3 and
re-review N1 added 1 h to task 37 and 1.5 h to task 38, review I2, I9 and I10 added 1 h to
task 31 and 0.5 h to task 29, re-review b04 m4's remembered refusal added 0.5 h to task 31,
review M14's prototype added 1 h to task 41, and review M9's Windows DACL added 0.5 h to task 37);
proof and documents (00, 43, 44a, 44b) 8 h (6 before review M4 put task 43 at 5 h, not 3);
**≈ 133.5 engineer-hours**, 5.5 h over ADR-H1's 16 engineer-days (≈ 128 h). The ADR's day includes
review and this table is implementation time only, so the plan now runs past the ADR's budget:
review M4's re-sizing (+6.5 h) is what crossed it. Task 00's
answers add rows to tasks 03–05, 07a, 07b, 14, 18, 19, 23, 24, 33, 34, 36 and 40 inside their existing
hours; that is the plan's estimate, not a measurement. The minimum (S7 reached) is tasks 44a, 00,
01–24 (07a, 07b, 16a and 16b included), 28–35, 37–39 ≈ 109.5 h (104.5 before review M4, 109
before review M9).

**The critical path, not the average, sets the date (review M3).** Two engineers work it: one on
the server lane, one on identity then the client lane. Halving 109.5 h to ≈ 55 h is wrong, because
lane A is one implementer in series. Its path to S7 is tasks 01–08 (07a and 07b included) and
13–24 (16a and 16b included): **≈ 63.5 h**, summed from the table above, after task 44a's 1.5 h
alone, so **≈ 65 h**. At the ADR's 8-hour day that is **a little over eight working days**, and
more once review is counted, because this table is implementation time only. Lane A's one
cross-lane wait costs it nothing: 13 needs 10 and 11 (lane B), which are done by ≈ 7 h on the
clock, long before lane A reaches 13 at ≈ 27 h. The second engineer's share of the minimum is
≈ 44.5 h (00, 09–12, 28–35, 37–39). It waits twice: 28 needs 06's `go.mod` (merged at ≈ 17.5 h on
the clock, after ≈ 7 h idle), and 34 needs 20 (≈ 53 h). Even so, 34, 35 and 39 (8 h) finish by
≈ 61 h, before lane A's 24. The lanes, not the dependencies, make the path this long: 06, 07a and
07b (9.5 h) need nothing from lane A, and moving them to the second engineer would give ≈ 55.5 h
on lane A and ≈ 54 h of work on the other, about seven working days, with no idle wait for
`go.mod`. That is a lane change for the orchestrator; this plan does not make it.
All of these figures were summed by script from the table above.

### Order and parallel lanes

Task 44a runs first, alone, on `feat/hub`, and the orchestrator announces its commit to phases 3,
4, 5 and 7 **with the read path**: `git show <44a-sha>:docs/superpowers/contracts/2026-09-23-hub-contract.md`.
Every worktree shares one object store, so that command works from a sibling's own worktree, where
the file on disk is still the old text (task 44a, step 3). Then four lanes, each its own git worktree branched from `feat/hub` **after 44a**, so
they can run at once. **An
implementer never merges**; the orchestrator merges a lane's finished tasks into `feat/hub`, in the
order below, and a lane that needs another lane's task waits for that merge.

| Lane | Worktree branch | Tasks, in order | Waits for |
|---|---|---|---|
| **A — contract in code, then server** | `feat/hub-server` | 01 → 02 → 03 → 04 → 05 → 06 → 07a → 07b → 08 → 13 → 14 → 15 → 16a → 16b → 17 → 18 → 19 → 20 → 21 → 22 → 23 → 24 → 25 → 26 → 27 | 13 waits for 10 and 11 (lane B) |
| **B — identity** | `feat/hub-identity` | 10 → 11 → 12 → 09 | 09 waits for 07b and 08 (lane A) |
| **C — control plane** | `feat/hub-client` | 28 → 29 → 30 → 31 → 32 → 37 → 33 → 34 → 35 → 39 → 36 | 28 waits for 06 (go.mod); 30 for 04 and 12; 33 for 04; 34 for 11, and its rows for 20, 22, 24–27; 36 for 26 |
| **D — desktop** | `feat/hub-desktop` | 38 → 40 → 41 → 42 | 38 for 37; 40 for 36 and 39 |
| — | `feat/hub` | **44a first** (task 00 is read, not built); at the end 43 → 44b | 44a: nothing; 43: everything |

**`feat/hub` is not merged onward until the owner has looked** (`OD-1`, review M14). Lane D merges
into `feat/hub` like every lane, so `feat/hub` carries two UI surfaces. `OD-1` keeps a UI branch
unmerged until the owner has looked at it, and phase H's surfaces also had no prototype (the waiver
names phases 3, 4, 5 and 7 only). The owner's look at `prototype/phase-h-hub.html` (task 41, Step 0)
and at tasks 41 and 42 is batched with the rest (`OD-12`), so no lane waits for it; only the merge
of `feat/hub` onward does. Owner question Q13.

**Why the lanes are cut this way — the shared files.** Five tasks edit the control plane's wiring
(`internal/api/api.go`, `cmd/api/config.go`, `cmd/api/main.go`, `routes_audit_test.go`): 32, 35,
36, 37, 39. One edits `httpx/origin.go`: 37 (task 36's expose list is set by its own `api` wrap,
task 00 HA7-4). All are in lane C, one after another, so no two
worktrees ever edit those files at once. `hubclient/hubclient.go` is lane A's alone: task 04
declares every name in it, `Invalidator` included, so task 31 in lane C only implements it (review
M15). Four desktop tasks edit `client.ts` or the store: all in
lane D. `control-plane/go.mod` is edited by 06 (pgx), 13 (`x/time` direct) and 28 (`x/sys` direct);
28 waits for 06's merge, and 13 is in 06's own lane. `go.work` is edited only by 06 (its `go` line);
a sibling phase that also moves a member to `go 1.23.0` makes the same one-line change, which merges
clean or conflicts identically.

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
| **S-CRED** | **The credential store.** One machine store, Windows Credential Manager (`credstore.Secrets`, `WinCred()`; `Memory()` off Windows and in tests), **one target grammar** (`credstore/targets.go`), and one typed accessor per credential: the Entra refresh token (`entraauth`, task 31), a **GitHub** token (`credstore.Local`, phase 3's `Store`), an **Azure DevOps** PAT per organisation (`credstore.LocalDevOps`, phase 3's `Store`, `Scheme: BasicPAT`, from v1 — review I10), an **Azure DevOps** bearer from the Entra sign-in (`entraauth.DevOps`, phase 3's `Store`, if Q5 is yes; chain order `Local → LocalDevOps → entraauth.DevOps → GitHelper`), and the **Panacea** `pnc_` token (`credstore.PanaceaTokens`, phase 7). The only write path for a pasted token is task 39's `POST /v1/credentials/{kind}`; nothing returns a secret | tasks 28, 29, 31, 39 | **phase 3** (`OD-4`: push and PR on both hosts), **phase 7** (`OD-13`: the Panacea API), **phase 6** (a future `kind` for the user's own Claude credentials — never pooled), **phase 8** (nothing: the broker reads no store) |
| S-TOKEN | `entraauth.TokenSource` — `Token(ctx, scope)`, `ErrSignedOut` (not signed in, or the Hub scope refused), `ErrEntraUnavailable`, and, for a non-Hub scope, `ErrScopeNeedsConsent` / `ErrScopeRefused`, each wrapping a `*ScopeRefusal{Scope, Code, Suberror}` (A12; the user stays signed in) | task 31 | `hubclient` (task 33); phase 8's broker client `remotebe` (HA-2), which shows a broker refusal as the refusal, never as "sign in again" or "broker unavailable" (sibling ask recorded in the review's fix batch b04) |
| S-HUB | `hubclient`'s consumer interfaces, `NewFake()`, the conformance suite, and `hubclient.WriteError` / `hubclient.Relay` (the relay rule, as a write and as a value) — every sibling ask answered in task 00 before they freeze. Siblings **consume** these interfaces and `NewFake()`; they never implement them, so an added method is additive for every one of them | tasks 00, 04, 05, 33, 34 | phases 3 (`Evidence`), 4 (`Briefs`), 5 (`Designs`), 7 (`Releases`) |
| S-LINK | `hublink.Resolver.Project(ctx, localProjectID) (hubapi.Project, hublink.State, error)` — how a handler finds the Hub project behind a local one | task 35 | phases 3, 4, 5, 7: every Hub-backed handler starts here |
| S-FEED | the control plane's `hubfeed.Poller.Subscribe(kindPrefix, fn)`; the desktop's `hubFeed.subscribe(kindPrefix, fn)`; and the generic cursor headers — `api.CursorSource` in `Deps.CursorSources`, one `Panacea-<Name>-Cursor` each, read by the desktop into `HubHeaders.cursors`, keyed by the lowercased name, through one `onHubHeaders` hook (HA7-4; no `onCursor`, a declared difference) | tasks 36, 40 | phase 4's `provenanceSlice` (`brief.*`), phase 5's approvals (`design.*`), phase 7's releases (`release.*`) and its `Deploy` cursor source |
| S-ENTRA | `internal/entra` — one validator for the Hub and the broker (HA-1) | task 10 / phase 8 task 16 | phase 8's `cmd/broker` |
| S-BIND | `server.bindingClaims(bearer) (member, emailVerified bool)` in `auth.go` — reads `acct` and `xms_edov` from the token `Validate` has just accepted, because `entra.Claims` carries neither (A15, review I12). Fails closed on an absent claim or another JSON type | task 13 | phase 8, by the two sibling asks in A15: when `entra.Claims` gains `Member` and `EmailVerified`, only this function's body changes; and its task 09 registration must configure `acct` and `xms_edov`, or no deployed token carries them. S-BYO's guest-heavy tenants depend on it |
| S-HOPS | `PANACEA_APP_HUB_TRUSTED_HOPS` — how many proxies append to `X-Forwarded-For` in front of the Hub; the per-IP key is the N-th entry from the right (A14, review I7) | tasks 08, 13 | phase 8's chart sets `1` behind its single ingress, once the sibling ask in A14 lands: its task 29 still sets the retired `TRUST_PROXY=true`, which the Hub refuses |
| S-CHART | `hub serve` / `hub migrate` / `hub verify`; `:8080`; `/healthz`, `/readyz`; `roles.sql`; `PANACEA_APP_HUB_*` | tasks 06–08 | phase 8 tasks 27 (image), 29 (chart, principals SQL), 30 (pipeline) |
| S-CI | an integration job with a Postgres service, running `go test -tags integration ./control-plane/internal/hub/...` | not built here: `azure-pipelines.yml` runs no database today | phase 8's pipelines (ask) |
| S-CANVAS | a `/v1/canvas/…` WebSocket route on the same server, identity from the same validator (ADR-H1 §5.5) | reserved; nothing built | phase 5 v2 |
| S-SSE | `GET /v1/changes/stream`, the same events, `Last-Event-ID` as the cursor | reserved | when the 5 s poll is noticed |
| S-BYO | multi-tenant registrations, a second `PANACEA_APP_HUB_TENANTS` entry, an enrolment route | reserved (ADR-H1 §5.9 item 3) | the customer tier |
| S-CSP | the webview has **no CSP** (`tauri.conf.json` `"csp": null`) | not ours; phase 2 or 9 | phase 5 renders design frames in that webview |
| S-DEPLOY-TARGET | a shared deploy target on the project (`Project.deployTarget`, `PUT`/`DELETE …/deploy-target`, `project.deploy_target_set`) — phase 7's HA7-3, additive | reserved (task 00); phase 7's local file stands until it is built | phase 7 |

---

## Open questions for the owner — each with the recommendation this plan builds

ADR-H1 §10's seven stand, and this plan builds each recommendation: **Q1** only a complete revision
is approved against or pinned; **Q2** erasure keeps key, digest, author and time and resolves
`erased`; **Q3** a designer may approve their own design in v1; **Q4** teammates join by an email
invite that binds at first sign-in; **Q5** Azure DevOps uses the same Entra sign-in (behind `PANACEA_APP_ENTRA_DEVOPS`, default
`false` until IT consents; until then a PAT the user pastes per organisation serves Azure DevOps
from the same store, review I10); **Q6** writes
never queue while the Hub is down; **Q7** the Hub holds no canvas documents in v1. Six more come
from this plan (Q11 from a sibling ask, task 00; Q12 from review I12; Q13 from review M14, which
named it Q11 before that number was taken):

| # | Question | Recommendation (what this plan builds) |
|---|---|---|
| **Q8** | How does a local project join the Hub? | **Explicitly**: a *Share with a team* action on the project row (task 42) creates the Hub project from the local project's remotes, in a team the user picks or creates in the same popover. A project whose remote is already some Hub project's links **automatically** on first sight (task 35), because that is the same repository. The alternative — auto-create a personal team for every local project — makes teams nobody chose |
| **Q9** | Where does a GitHub token come from in v1? | **Pasted once** into the app (a fine-grained personal access token), stored by task 39, with phase 3's machine-credential-helper fallback behind it. A GitHub OAuth app would need an app registration under `kpmg-au-emu` and `lcfrai` — later |
| **Q10** | Who runs the Hub's database tests in CI? | **Phase 8's pipeline** adds a Postgres service job (S-CI). Until then they run on a developer laptop against task 09's stack, and every report says so |
| **Q11** | ADR-H1 R5 says *"every approval is audited with its session context"* (phase 5's HA5-4). The Hub can verify only the principal and the time. Delete the phrase, or add a client-asserted `surface` field? | **Delete the phrase.** The Hub records the principal and the time, which it verifies; a client-asserted field would sit in an audit row reading as evidence it is not. The contract states that reading (task 44a); an optional field stays an additive change behind `ApprovalRequest` if the owner wants it |
| **Q12** | Correction A15 (review I12) changes two cells of ADR-H1, whose text is the owner's: §5.3's invite row (`ADR-H1-hub-architecture.md:406`) binds by `bindEmail` — the `email` claim of a member (`acct` 0) or of a guest whose email domain is verified (`xms_edov` true) — never by `preferred_username`; and §9 item 3 (`:725`), IT's Hub API registration, configures the optional claims `email`, `acct` and `xms_edov`. Accept both? And second: a caller who cannot bind (a guest without `xms_edov`, an account with no `email`) has no in-product remedy in v1, because no route lists an org's users and no surface shows a user their Hub id. Is that acceptable for v1? | **Yes to both edits; task 44a makes them in place**, each marked *"(amended by the Phase H plan, A15)"*. They keep ADR-H1's decision (Q4: an email invite binds at first sign-in, no Graph) and change only which claim may bind. Unedited, IT builds the registration from §9 and no deployed invite binds, with no error; and the contract's precedence rule (`hub-contract.md:4-5`) makes `:406`'s `preferred_username` binding the governing text again. A members-only rule (refusing a verified guest) is one line in `bindingClaims`. **Yes to the limitation for v1's single tenant**, where it reaches only an unverified guest or an account with no mail attribute; the fallback is out-of-band (an operator reads the id from `users`), stated in task 16a. The in-product remedy — an admin-only view of org users, or `GET /v1/me` reporting an invite it could not bind — is later and additive |
| **Q13** | `OD-1` waives prototype-before-plan for phases 3, 4, 5 and 7 only (`owner-decisions.md:12`). Phase H has two UI surfaces, the sign-in states in the rail footer (task 41) and the share popover (task 42), and no prototype of either. Do you want a prototype of them, and must `feat/hub` wait for your look before it merges onward? | **A static prototype first, and yes, `feat/hub` waits.** Task 41's Step 0 writes `prototype/phase-h-hub.html` (the four sign-in states, the no-Hub footer, the popover's states) before any component code, and the components follow it (+1 h on task 41). Your look at it and at the built surfaces is batched to the end (`OD-12`), so no lane waits; only `feat/hub`'s merge onward does, as `OD-1` keeps every UI branch. *No prototype* skips Step 0 and returns the hour. Seeing the prototype before any code would pause lane D at task 41 and delay task 43, which needs 01–42 |

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
  (task 07a asserts it exists); that a transaction-local `set_config` leaves `''`, not `NULL`, on a
  pooled connection after commit (task 13's test runs a second transaction on the same connection
  and asserts it reads nothing); `SECURITY DEFINER` running as the owner for `guard_revision`'s
  `current_user` test (task 27). Also (A16, A17): that Postgres checks a foreign key without
  row-level security (the premise of review I5; task 07a's `TestACrossOrgReferenceIsRefused` meets
  it both ways, since a same-org row must insert and a cross-org one must fail `23503`); and that
  `position(bytea IN bytea)` exists as task 27's `markerHits` uses it (its hit on
  `brief_revisions.canonical` before the erasure proves it runs).
- **Postgres facts behind A19** (review M11), from knowledge and pinned by task 07b's tests: that
  `SELECT … FOR UPDATE` needs `UPDATE` privilege on at least one column, which is why the write
  lock is `hub.lock_audit_head()` (`TestTheHeadLockNeedsTheFunction`); that a row lock taken inside
  a `SECURITY DEFINER` function belongs to the calling transaction and is held until it ends (the
  same test's second connection); that `WITH CHECK` runs after `BEFORE` triggers, which is why
  `audit_append` checks the org itself (`TestAuditAppendChecksTheOrgFirst`, which goes red on its
  mutant in either order, and whose mutant run records which order holds); and that
  `lock_timeout` bounds a row-lock wait inside a trigger (the same test's 2 s timeout).
- **Physical erasure** (A17, review M12): `erase_brief`'s `UPDATE` leaves dead tuples, which hold
  the text on disk until `VACUUM` reclaims them; the text stays in the write-ahead log; and a backup
  taken **before** the erasure keeps it for the backup's life. All three were true of
  `brief_revisions.canonical` before this plan; none is closed here. G6's promise is that no live
  row and no replay answer holds the text. **The real erasure horizon is the backup retention
  phase 8 sets**: 7 days in dev, 35 in prod (phase 8 task 08 `:229-232`, on `feat/env`), so an
  erasure is complete only after that window (task 27's "does not claim").
- **Win32 facts from documentation recall:** `CRED_MAX_CREDENTIAL_BLOB_SIZE` = 2,560 bytes; the
  `CREDENTIALW` layout. Task 28's Windows-only test round-trips a 6,000-byte blob to prove the
  chunking, on the owner's laptop, since CI is Linux (`azure-pipelines.yml:6`).
- **Sibling plans.** Phase 3 (23 parts and a review) and phase 8 (33 parts) were read by absolute
  path at about 21:30 on 2026-09-23. Phases 4, 5 and 7 then had only a few lines of plan; their
  full plans, read on 2026-09-24 from the sibling worktrees, name their Hub asks, and **task 00
  answers every one before S0 freezes** (review finding C1). The earlier premise that a later ask
  "is additive" was false for three of them (HA5-1, HA7-1 twice). In brief:

  | Ask | Answer | Built by |
  |---|---|---|
  | HA4-1 `hubapi` structs | task 03's structs, frozen at S0 | 03 |
  | HA4-2 `Briefs.List` | built; `BriefQuery.Page` rather than flat `After`/`Limit` | 03, 04, 05, 34 |
  | HA4-3 batch 64, 4 MiB | built: `hubapi.MaxRevisionBatch`, `MaxRevisionBatchBytes`; reply cap 8 MiB; both rest on the wire rule (canonical `content`, no HTML escaping either way) | 03, 04, 08, 14, 19, 33, 34 |
  | HA4-4 `latestComplete` | built on `BriefWithRevision` | 03, 04, 18 |
  | HA4-5 `Relay` | built; `Error.RetryAfter` added | 04, 33 |
  | HA4-6 ADO repo key | = A1 | 01, 17 |
  | HA4-7, HA5-3 fake completeness | confirmed, no change | 04, 05 |
  | HA4-8 `Subscribe` | this plan's `Subscribe(kindPrefix, fn)` stands; `""` is the ask | 36 |
  | HA4-9 revision digest | met by `BriefEnvelope.Digest()` | 02 |
  | HA5-1 `PutBlob` `io.ReadSeeker` | built; rewound before every attempt | 04, 05, 33, 34 |
  | HA5-2 `Designs`' seven methods | 44a amends §9.2 | 44a |
  | HA5-4 R5's "session context" | principal and time only; the ADR wording is an owner question | 44a |
  | HA5-5 canvases in the Hub | reserved, S-CANVAS | — |
  | HA7-1 nullable `environment` | built, with a kind-based CHECK and a set-once guard (task 00 says why the review's shape was corrected) | 03, 04, 05, 07a, 07b, 23, 24, 34 |
  | HA7-2 rollback confirm `{}` | built: naming its own source is 409 | 05, 24, 44a |
  | HA7-3 shared deploy target | reserved, S-DEPLOY-TARGET | — |
  | HA7-4 generic cursor | control plane built as asked: `CursorSource`. Desktop: declared difference — one `onHubHeaders(h)` hook, not `onCursor`; `h.cursors` keyed by the lowercased name (`deploy`) | 36, 40 |
  | HA7-5 `kind` filter | built | 03, 05, 23, 34 |

  The sibling line numbers in task 00 move as those plans are edited; the ask ids do not.
- **Phase 8 task 16** (`internal/entra`) and **phase 3 task 13** (`credstore.go`) are plans, not
  code. Tasks 10 and 29 write them from those parts only if they have not landed. The parts give
  declarations, not bodies, so a copy meets the owner's file as an add/add conflict that the owner
  wins (review M5). That our bodies and the owner's behave alike rests on the owner's tests alone,
  which this plan re-runs after the merge.
- **Docker Desktop's reachability** of a host service bound to `127.0.0.1` from a container was not
  tested; the local caller key (task 37) makes the answer not matter, as ADR-H1 §5.4 intends.
- **Tauri 2 app-defined commands need no capability entry** (task 38): inferred from phase 2's
  `greet`, which has none; checked only when task 38 runs `npm run tauri dev`.
- **Token shapes**: GitHub's documented prefixes (`ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_`,
  `github_pat_`) from knowledge; Panacea's `pnc_` from the phase 7 brief (`:119`), its length bound
  a guess (task 39). The Azure DevOps PAT shape (52 base32 characters classically, longer since)
  is from knowledge, and task 39's bound `^[A-Za-z0-9]{52,256}$` is generous on purpose (review I10).
- **Entra's per-resource refusal bodies** — `invalid_grant` with `suberror: consent_required`
  (AADSTS65001), `interaction_required` (AADSTS50076/50079) and `invalid_scope` (AADSTS70011) for
  one resource while the refresh token still redeems another — are from knowledge, not observed
  (review I2); task 31's tests forge them, and task 43's real-tenant sign-in (R-8) is the first real
  look. So is whether a fresh Hub-only sign-in clears an Azure DevOps 50076 (re-review b04 N1): v1
  has no per-resource step-up, and R-8 records the answer for the owner.
- **The lifetime-aware refresh skew** (A13) assumes Entra's `expires_in` is the token's real
  lifetime; an hour's token behaves exactly as the contract's five minutes, and only a
  tenant token-lifetime policy under ten minutes meets the new rule.
- **Chromium applying `form-action` to the redirect after a form POST** (review I4) is from
  knowledge, not observed. Task 12's page names the listener's exact origin either way, and its
  step 4 — in Edge, clicking Grace lands on the listener URL with no CSP violation — is the first
  real look.
- **`acct` and `xms_edov`** (A15): that both optional claims appear in a v2 **access** token once
  configured, that `acct` is a JSON number (0 member, 1 guest) and `xms_edov` a JSON boolean, are
  from Microsoft's documentation as remembered, not observed. The Hub fails closed on either being
  absent or of another type, so a wrong assumption shows as "no invite binds", never as a grant.
  Task 43's R-4 records the real types.
- **The `X-Forwarded-For` shape behind phase 8's gateway** (A14): that it appends exactly one
  entry, so `TRUSTED_HOPS=1` is right, is assumed from how ingress controllers usually behave;
  phase 8's chart sets the value and its own deployment is where it is first seen.
- **`docker compose up --wait`** on the owner's Docker Desktop version (task 09 falls back to
  polling the health status).
- **The Azure DevOps resource id** `499b84ac-1321-427f-aa17-267ca6975798` is the contract's
  (§6.2, §15), not checked; `PANACEA_APP_ENTRA_DEVOPS` defaults to `false` until IT grants consent,
  and meanwhile a pasted PAT (`credstore.LocalDevOps`, task 39) serves Azure DevOps.
- **Two golden vectors of the contract were re-computed** for this plan (Python `json.dumps` with
  `sort_keys`, `separators=(",",":")`, `ensure_ascii=False`): both lengths (209, 256) and both
  digests agree with contract §3.5. Vectors 3 and 4 (task 02) were computed the same way. That is
  the only code this plan ran.

