# ADR-H1 — The Hub's architecture

**Status:** decided by the orchestrator's judge pass, 2026-09-23. Owner-reviewable, as every plan is
under `OD-1`; nothing here is built. **Branch:** `feat/hub`, based on `3303fd6`.
**Contract built from this ADR:** `docs/superpowers/contracts/2026-09-23-hub-contract.md`.
**Decisions it rests on:** `docs/superpowers/planning/2026-09-23-owner-decisions.md` — `OD-3`
(shared from day one), `OD-4` (both git hosts), `OD-5` (our own server side, in this monorepo),
`OD-6` (Entra, auth-code + PKCE, tokens in Windows Credential Manager), `OD-11` (clean-room canvas,
permissive licences only), `OD-13` (Panacea stays a separate API).

---

## 0. How this ADR was made, and one fact the reader must have first

**The three candidate designs this judge pass was asked to score did not exist.** The task named
`docs/superpowers/planning/hub/design-a.md`, `design-b.md` and `design-c.md`, with their summaries
passed as an empty list. Checked on 2026-09-23 in this worktree, in the main tree and in every
sibling worktree under `.claude/worktrees/` (`env`, `phase-3`, `phase-4`, `phase-5`, `phase-7`,
`sandbox-git-fix`): **no `planning/hub/` directory and no file named `design-*` exists anywhere**.
The three design agents most likely died before their first write (a known failure mode of this
program: spend caps, harness restarts and the 180-second watchdog).

So the judge pass could not judge their text. It did the next most useful thing: it **wrote each
candidate out from the stance the task assigned it** — A *smallest Go-native*, B *regulated-enterprise
first*, C *provenance and collaboration first* — at the same level of detail, against the same
inputs, and scored those (§3). **The scores are therefore one author's judgement of three
reconstructions, not an independent panel.** The adversarial review `OD-1` requires before code is
the check on that, and it should be told this.

What was read for this ADR, directly, in this worktree unless a sibling is named:

| Input | Used for |
|---|---|
| `planning/2026-09-23-owner-decisions.md` (all 122 lines) | the constraints every candidate must meet |
| `planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md` (all 675 lines) | every store the Hub must hold, the key grammar, P1–P13 |
| `planning/2026-09-23-phases-4-5-7/orchestrator-rulings.md` (all) | R1 (Materialiser), R2 (lift, do not duplicate), R4 (`upstream-down`), R5 (rollback key union) |
| `docs/PROGRAM.md` (all) | phase table, local-first rule, cross-cutting constraints |
| `control-plane/internal/httpx/problem.go` (all), `control-plane/internal/ids/ids.go` (all) | the problem model and id rules the Hub reuses |
| `control-plane/go.mod`, `sandboxd/go.mod`, `go.work` | module layout, what is already a dependency |
| `desktop/src/api/client.ts:1-30`, `desktop/src-tauri/Cargo.toml` | the desktop hop has no auth today; the shell has no keyring crate |
| sibling `env` worktree, `planning/env/survey-own-environment.md` §1.7–§1.9, §2, §3.5, §4 | phase 8's recommended estate, so the Hub fits it |

Further inputs read later in this pass are listed where they are used. §11 lists what was **not**
verified.

---

## 1. Context

### 1.1 What forces a server

Until 2026-09-23 every provenance store lived in the control plane's state directory on the user's
laptop (`provenance-contract-draft.md` §2.1: the brief store at `statepath.Dir("discover")`, the
ledger at `statepath.Dir("provenance")`). `OD-3` made briefs, approvals and the ledger **shared from
day one**, and `OD-5` put them on **our own server side, in this repo**. A teammate who opens the
project must see the same brief, the same approval and the same release pin. A laptop cannot be that
source of truth.

### 1.2 What the Hub must hold — the contract's stores, moved

`provenance-contract-draft.md` §2.2 names every record and its sole writer. `OD-3`'s exception
(owner decisions `:29-30`) keeps each recommendation's **shape** and moves its **location** to the
Hub. So the Hub holds:

| Record | Contract section | Writer phase |
|---|---|---|
| Brief head, one per brief, mutable by Discover only | §1.1, §2.2 | 4 |
| Brief revision, immutable, `brief:<briefId>@<rev>` + SHA-256 digest | §1.2, §1.3 | 4 |
| Design link: `design:<canvasId>@<designRevision>` + `reviewDigest` → brief key | §2.2, §3.2 | 5 |
| Session → design key link (shape reserved) | §3.2 | 5 |
| Merge link: merge commit → brief key reviewed against (**phase 3 chooses its home**) | §2.2 | 3 |
| Release link, tagged union `run` / `rollback` (R5), pinned once at Ship | §3.4, R5 | 7 |
| Release → `deploymentId` join, a second record referencing the first | §2.2 | 7 |

Plus what `OD-3`/`OD-5` add and no earlier document owned: **users, teams, membership, projects and
their git remotes on both hosts** (`OD-4`), and an **audit log**.

### 1.3 What stays where it is

- **`Session.contextRef`** stays on the control plane's registry session record
  (`desktop/src/store/types.ts`, contract §2.4). It is a fact about one process on one machine. It
  holds a brief key; the key resolves at the Hub.
- **Sessions, machines, worktrees and PTYs** stay local. The Hub never sees a terminal byte.
- **Criterion state** stays computed, never stored (§2.3). Phase 3's evaluator is local.
- **The managed block in `CLAUDE.md`** stays a render written by the control plane's Materialiser
  (R1). It now renders a revision fetched from the Hub.

### 1.4 The forces, ranked

1. **Provenance integrity.** A revision that can change is not a revision (P3). A link a caller can
   assert is not a pin (P7). Server-side, these become database invariants, not code discipline.
2. **Unblock phases 4, 5 and 7 tonight.** Their planners build against the contract this ADR
   produces. The minimum Hub must be small enough to build first and complete enough not to move.
3. **Local-first** (`PROGRAM.md` "Local-first"). The Hub runs on a laptop with Docker Postgres and
   the identical sign-in flow, before any Azure spend.
4. **Regulated enterprise.** KPMG and its customers: Australian residency, audit, erasure, least
   privilege, and later a customer's own Entra tenant.
5. **Simplicity.** Every moving part is one the phase 8 environment must host and one the owner
   must approve. Panacea's own SSO chain (Keycloak, oauth2-proxy) is a recurring outage class
   (`env` survey §1.7, §2.2).

### 1.5 What the sibling plans already ask of the Hub

Three sibling planners wrote while this ADR did not yet exist, and each defined the minimum it needs
as an *ask*. Read on 2026-09-23 by absolute path, never written to. The contract satisfies each ask
or names the difference (contract §12).

| Ask | From | Source, read directly |
|---|---|---|
| **H-1** credential store, **H-2** evidence store, **H-3** repository identity (`githost.RepoKey`, never the local `p-…`), **H-4** the signed-in principal as recorder | phase 3 | `worktrees/phase-3/docs/superpowers/plans/2026-09-23-phase-3-git-convergence.md:289-301` |
| **HA-1** one shared Entra validator for the Hub and the broker, **HA-2** a control-plane `TokenSource`, **HA-3** the app registrations, **HA-4** the deployable, **HA-5** the database, **HA-6** who calls the Hub, **HA-7** audit and object storage | phase 8 | `worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:220-238` (uncommitted edits in that tree at the time of reading) |
| Approvals, design links and **approved snapshots** in the Hub from v1; live shared documents in the Hub from **v2**, as the same Go package hosted server-side | phase 5 | `worktrees/phase-5/docs/superpowers/planning/phase-5/paper-design-cleanroom-spec.md:372-380`, `:461-490` |

Two facts from reading the tree that shape every section below:

- **Local ids cannot be shared ids.** `project.ProjectID` is `"p-"` + the first 8 hex of the SHA-256
  of the project's **absolute path** (`control-plane/internal/project/project.go:101-102`), and
  `BranchID` embeds that hash (`:118-121`). Two teammates' ids for the same repository never
  match. So the Hub mints its **own** project ids, keys a project to its git remotes (`OD-4`,
  phase 3's H-3), and names a branch by its **git ref name**, which *is* shared through the remote.
- **`httpx` and `ids` are `internal` to the control-plane module** (`control-plane/internal/httpx`,
  `control-plane/internal/ids`), and `go.work` holds only `./control-plane` and `./sandboxd`. A new
  module cannot import them. Sharing them means moving them — the same call R2 made for
  `writeAtomic`.

---

## 2. The candidates

Each is written at the same depth, from the stance the task assigned it (§0). All three share what
the owner decisions fix and are not re-argued: Go, Postgres, Entra with auth-code + PKCE from the
desktop, tokens in Windows Credential Manager, loopback-only control plane, this monorepo.

### 2.1 Design A — smallest Go-native

- **One new Go module, `hub/`**, one binary, REST + JSON under `/api/v1`, the control plane's
  problem body. `pgx` for Postgres, SQL migrations embedded in the binary and run by a ~80-line
  runner. Nothing else in the process.
- **Identity:** the **control plane** runs PKCE against Entra directly and holds the tokens. The Hub
  validates Entra access tokens itself against JWKS. **No broker.** A ~400-line development issuer
  in Go emits Entra-shaped tokens so local development runs the identical flow.
- **Data:** orgs, users, teams, projects with remotes, brief heads plus immutable revisions, one
  append-only link table, releases, an audit table. Immutability by trigger and by revoked grants.
- **Realtime:** none. The control plane polls a `changes` cursor.
- **Blobs:** none — nothing in the first cut needs them.
- **Authorisation:** team roles, checked in Go on every query.
- **Cost:** about 3 engineer-days to the first route phase 4 can call; about 8 to the whole of it.

### 2.2 Design B — regulated-enterprise first

- **Keycloak in front of Entra** as the identity broker: the desktop does PKCE against Keycloak,
  Keycloak federates Entra, and a customer's own Entra is one more Keycloak identity provider. The
  Hub trusts only Keycloak's issuer.
- **The Tauri shell** runs the flow in Rust and keeps tokens with a keyring crate, handing the
  control plane a short-lived token per call.
- **Postgres row-level security on every table**, per team, with the user id set per transaction.
  **Envelope encryption** of brief text with a per-org key in Key Vault, so erasure is
  crypto-shredding. **Hash-chained audit** streamed to Log Analytics and Sentinel. API Management
  in front of the Hub for rate limits and the WAF.
- **Blobs:** Azure Blob per org, customer-managed keys.
- **Cost:** about 15 engineer-days before phase 4's first call, dominated by Keycloak and APIM
  configuration that has no local equivalent.

### 2.3 Design C — provenance and collaboration first

- **Event-sourced.** Every write is an event in one append-only log; brief heads, releases and
  link views are projections rebuilt from it. **Revisions chain**: each revision's digest covers
  its parent's, so history cannot be rewritten without breaking every later digest.
- **The Hub signs each ledger entry** with a server key, so a release manifest verifies offline.
- **Two-phase Ship.** The Hub takes the release pin itself in a *reservation* before the deploy
  call, and the control plane only confirms the run afterwards — the caller never asserts a pin.
- **Realtime from day one**: a WebSocket fan-out with presence, and the Hub hosts the canvas's
  multiplayer relay (a Yjs-protocol relay) from v1.
- **Blobs:** content-addressed (`sha256`) from v1, for design assets and snapshots.
- **Cost:** about 12 engineer-days before phase 4's first call; projections and the relay are on
  the critical path.

---

## 3. Scores

1 is worst, 5 is best. **Delivery risk scores high when the risk is low.** One author, three
reconstructions (§0).

| Criterion | A — smallest | B — regulated | C — provenance | Why the spread |
|---|:-:|:-:|:-:|---|
| Fit to `OD-1`…`OD-13` | **4** | 2 | 4 | B's Keycloak is a broker `OD-6` did not ask for and phase 8's OQ-4 rules out; B's Rust-held tokens contradict *"the control plane calls it as the user"* (`OD-5` consequence text, owner decisions `:48`). A's polling only half-meets `OD-3`'s *a teammate sees the same brief*: seconds late, not wrong |
| Simplicity | **5** | 1 | 2 | A adds one binary and one database. B adds Keycloak, APIM, Key Vault key hierarchy, a Rust auth stack. C adds projections, a signing key and a relay |
| Security | 3 | **5** | 3 | B's RLS, envelope keys and WAF are real defence in depth. A's authorisation lives in Go only — one missed `WHERE` is a cross-tenant read. C's signatures add integrity, not confidentiality |
| Provenance integrity | 4 | 3 | **5** | A makes P3 a database invariant. C adds the parent chain and, decisively, the **reservation**: the Hub, not the caller, takes the Ship pin. B has no provenance mechanism beyond audit |
| Local-first | **5** | 2 | 4 | A runs as `docker compose up` with Postgres and the dev issuer. B's Keycloak realm and APIM policies have no faithful local twin |
| Unblocks 4, 5 and 7 | **5** | 2 | 3 | A's first routes are the ones phase 4 needs. C holds them behind projections; B behind identity plumbing |
| Delivery risk | **5** | 1 | 2 | A has no component this team has not built before. B's Keycloak is a named outage class. C's event store is a rewrite-sized bet |
| **Total /35** | **31** | 16 | 23 | |

---

## 4. Decision

**Design A wins, with one structural amendment and six grafts.** A is the only candidate whose
first route phase 4 can call exists in days, whose local twin is the real thing, and whose moving
parts are ones this team already runs. Its two real weaknesses — authorisation enforced only in Go,
and a release pin the caller asserts — are exactly what the runners-up are good at, and each fix is
small enough to take.

### 4.1 The structural amendment — the Hub lives in the control-plane module

A, as written in §2.1, is a new `hub/` module in `go.work`. **The judge pass overrules that.** The
Hub is a second binary in the existing module:

```
control-plane/cmd/hub/          main package: the Hub server            (new)
control-plane/cmd/devissuer/    main package: the local OIDC issuer      (new)
control-plane/internal/hub/...  the Hub's packages                       (new)
control-plane/internal/entra/   Entra access-token validation            (phase 8 task 16; HA-1)
control-plane/internal/httpx/   reused as-is: Problem, WriteJSON, DecodeJSON
control-plane/internal/ids/     reused, plus one new function (contract §3.1)
```

Why, in order of weight:

1. **`httpx` and `ids` are `internal`** (§1.5). A new module can import them only after they are
   moved to a shared module, which rewrites the import block of every control-plane handler.
   Phases 3, 4, 5 and 7 are editing those handlers tonight in four worktrees. A move is a
   four-way merge collision for no behaviour change.
2. **Phase 8 already made this call for the broker.** `control-plane/cmd/broker` imports
   `control-plane/internal/entra` (phase 8 plan, file map `:306`, task 16). With the Hub in the same
   module, HA-1 — *"the broker and the Hub must accept and reject the same tokens"* — is satisfied by
   construction: it is one package, not two to reconcile.
3. **Go links only what a `main` package imports.** The Hub binary does not carry `client-go` or the
   Docker client because they share a `go.mod`. `pgx` joins the module's requirements; the
   control-plane binary does not link it.

The cost is a boundary the compiler does not draw. It is drawn by a test (contract §13, task H0.3):
`go list -deps ./cmd/hub` may name only the standard library, `internal/hub/...`, `internal/httpx`,
`internal/ids`, `internal/entra`, `pgx` and `golang.org/x/time/rate`; and no package outside
`internal/hub/...` may import `internal/hub/server/...`. The control plane reaches the Hub only
through `internal/hub/hubclient` and the wire types in `internal/hub/hubapi`.

### 4.2 The grafts

| # | Graft | From | What it fixes in A | Cost | Taken? |
|---|---|---|---|---|---|
| G1 | **Ship reservation.** `POST …/releases` makes the **Hub** resolve and fix the pin *before* the deploy call; the control plane then confirms the run or abandons it | C | A's release link is asserted by the caller after the fact — P7 (*"a release link written by a second call"*) is broken the moment the store is on another machine | ~1 day | **yes** |
| G2 | **Content-addressed blobs** (`sha256`), **stored in Postgres** in v1 behind a `blobstore` interface | C (moved into Postgres) | A has no blobs, but phase 5 captures the approved snapshot into the Hub at approval (`paper-design-cleanroom-spec.md:474`) | ~1 day | **yes**, without C's object store |
| G3 | **An ordered change feed.** Every write appends one row to the audit log under a lock; the feed is that log, filtered. The control plane polls it; SSE later serves the same rows | C, reduced | A's "changes cursor" was unspecified, and a `bigserial` cursor skips rows that commit out of order | ~½ day | **yes**, polling only |
| G4 | **Postgres row-level security on `org_id`**, plus a DDL identity and a DML identity, plus immutability by grants **and** triggers | B, reduced | A's single line of tenancy defence (§3, security 3) | ~1 day | **yes**, org level only |
| G5 | **Hash-chained audit log** | B and C | tampering with the audit is silent in A | ~½ day | **yes**; anchoring the head outside the database is later |
| G6 | **An org-admin app role**, and **erasure as a tombstone** that keeps key and digest | B | A has no erasure and no role above team admin | ~½ day | **yes** |

**Taken whole from neither.** Rejected grafts, each with its reason:

- **C's event sourcing** — projections rebuilt from a log make every read path a second system to
  test, and the property it buys (no lost history) G4's immutability already gives the only records
  that need it.
- **C's server-signed ledger entries** — worth it only when a manifest leaves our system and must
  verify offline. Nothing does in v1. It is a key-management programme; later (§7).
- **C's WebSocket presence and Yjs relay in v1** — phase 5 keeps live canvas documents in the control
  plane in v1 (`paper-design-cleanroom-spec.md:735`, OQ-2) and names its v2 model as
  server-authoritative per-property last-writer-wins, not Yjs (`:647`). The Hub will host *that Go
  package* in v2, over the WebSocket library already in `go.mod`. A relay built tonight would be for
  a protocol phase 5 is not using.
- **B's Keycloak broker** — §6.1.
- **B's API Management in front of the Hub** — rate limits live in the process (contract §11.3);
  the WAF question is phase 8's edge, not the Hub's.
- **B's envelope encryption and crypto-shredding** — needs a per-org key hierarchy in Key Vault with
  no local twin. Azure Postgres encrypts at rest; erasure is a tombstone written by a dedicated
  function (G6). Revisit when a customer contract requires customer-managed keys.
- **B's Rust-held tokens** — §6.2.

**Revised cost.** H-min (§5.9) is about **11 engineer-days**; the first route phase 4 can call
(`POST …/brief-revisions`) is reachable on **day 3**. Estimates assume one engineer who has read
this ADR and the contract, and that the phase 8 `internal/entra` package lands first or is written
here to its interface.

---

## 5. The decision in detail — the nine questions

Each answer is the decision; the contract holds the exact routes, JSON, DDL and variables.

### 5.1 Service shape

- **Where:** §4.1. One binary, `hub`, built from `control-plane/cmd/hub`. Stateless: two replicas
  behind phase 8's gateway are safe, because every piece of shared state is in Postgres and the only
  in-process state is the JWKS cache and the rate-limit buckets.
- **API style:** REST + JSON under **`/v1`** (no `/api` prefix — the Hub serves nothing but its API,
  and phase 8's `HTTPRoute` mounts it at the root of its own host). Versioning is the path segment;
  a breaking change is `/v2` served beside `/v1` for one release. Additive fields are not breaking,
  and every decoder on both sides tolerates them **except** the Hub's own request decoding, which
  keeps `httpx.DecodeJSON`'s unknown-field rejection.
- **Error model:** `httpx.Problem` — `{"error","message","details"}` — **byte-for-byte the control
  plane's**, written through `httpx.WriteProblem`. The Hub's codes are its own constant block in
  `internal/hub/hubapi`, disjoint from `httpx`'s sixteen except where the meaning is identical
  (`invalid_json`, `bad_request`, `forbidden_origin`, `conflict`, `internal`). Contract §4.
- **How a Hub error reaches the desktop:** the control plane relays a Hub 4xx problem **unchanged**
  (same status, same code), so a teammate's `revision_conflict` reads the same in both places.
  A Hub that cannot be reached, or answers 5xx, becomes the control plane's **503
  `upstream_unavailable`, `details.upstream = "hub"`** — ruling R4's code and kind, with a third
  upstream name. No new desktop `ProblemKind` is needed.

### 5.2 Identity

**The control plane runs the sign-in and holds the tokens. The renderer never sees one.**

1. The renderer calls the control plane's `POST /api/v1/auth/sign-in`. The control plane makes a
   PKCE verifier (32 random bytes), its S256 challenge, a `state` and a `nonce`, and opens a
   **one-shot listener on `127.0.0.1:0`** (RFC 8252 §7.3, an ephemeral port).
2. It answers `{ "authorizeUrl": … }`. The renderer opens that URL in the **system browser** through
   `tauri-plugin-opener`, which `desktop/src-tauri/Cargo.toml` already depends on and
   `capabilities/default.json` already grants (`opener:default`). No embedded webview sees the
   password, which is what Entra's conditional access expects of a desktop app.
3. Entra redirects to `http://127.0.0.1:<port>/?code=…&state=…`. The listener checks `state`,
   answers a static "you can close this tab" page, closes itself, and exchanges the code plus
   verifier at the token endpoint.
4. The **refresh token** goes to **Windows Credential Manager**; **access tokens stay in memory**.
   The renderer learns the outcome by calling `GET /api/v1/auth/status` on its poll tick.

The one-shot listener is deliberate. The main control-plane listener's Origin policy rests on an
invariant written into `httpx/origin.go`'s `Allow` doc comment: *"no route behind this middleware
may ever have a side-effecting GET"*. An OAuth callback is a side-effecting GET. Putting it on a
separate listener that exists for one sign-in keeps that invariant true instead of adding its
first exception.

**Why the control plane and not the Tauri shell.** `OD-5`'s consequence says *"the desktop's
control plane calls it as the user"*. Phase 8's HA-2 asks for a `TokenSource` **in the control
plane**. Phase 3's H-1 asks for `credstore.Store` **in the control plane**. The provenance contract's
§6.6 puts the Panacea token in the process that calls Panacea. If the shell held the tokens, every
Hub, broker and Panacea call would need the token handed across a process boundary, and the shell
would need a keyring crate it does not have. Rejected in §6.2.

**Refresh.** Entra access tokens live 60–90 minutes. `entraauth.TokenSource.Token(ctx, scope)`
serves a cached token until 5 minutes before `exp`, then redeems the refresh token, **single-flight**
per scope so twelve concurrent callers make one request. Entra rotates the refresh token on
redemption; the new one overwrites the old in Credential Manager before the access token is
returned. `invalid_grant` or `interaction_required` (a conditional-access sign-in frequency, for
example) clears the store and reports `signed_out` with reason `reauth_required`. **One refresh
token serves every resource** the desktop client is consented for: the Hub (`Hub.Access`), the
broker (`Sandboxes.Access`, phase 8) and, if the owner wants it, Azure DevOps. So there is one
sign-in, not three.

**Credential Manager.** Generic credentials, `CRED_PERSIST_LOCAL_MACHINE` (never enterprise-roaming:
a roaming profile would copy the token to every machine the user touches), written through
`advapi32!CredWriteW` via `golang.org/x/sys/windows` — already in the module graph as an indirect
requirement, so no new module. A credential blob is capped at 2,560 bytes
(`CRED_MAX_CREDENTIAL_BLOB_SIZE`); a larger refresh token is split across numbered targets.
**Unverified:** the size of a KPMG refresh token. The same store is phase 3's H-1 (`credstore`, for
GitHub and Azure DevOps) and phase 7's Panacea token. The **Hub server stores no git or Panacea
credential**: each user's credentials stay on that user's machine, which is `CLAUDE.md`'s never-pool
rule applied to git as well as to Claude.

**How the Hub validates a token** — `internal/entra`, the same package the broker uses:

| Check | Rule |
|---|---|
| Format | JWS compact, `alg` = `RS256` only, `kid` present. `none` and HMAC are rejected before parsing claims |
| Signature | against the tenant's JWKS, `https://login.microsoftonline.com/<tid>/discovery/v2.0/keys`, cached 24 h, refetched on an unknown `kid` at most once per 5 minutes |
| `ver` | `2.0`. The Hub API's manifest sets `requestedAccessTokenVersion: 2` |
| `iss` | exactly `https://login.microsoftonline.com/<tid>/v2.0`, where `<tid>` is the token's own `tid` **and** an enrolled org's tenant |
| `aud` | the **Hub API's client id** (a v2 token's `aud` is the GUID, not the `api://` URI) |
| `scp` | contains `Hub.Access`. A token with no `scp` (app-only, `idtyp=app`) is rejected: the Hub is delegated-only |
| `azp` | in `PANACEA_APP_HUB_ALLOWED_CLIENTS` — the desktop's client id. A token some other app obtained for our API is refused |
| `exp`, `nbf` | 120 s of clock skew |
| Principal | **`(tid, oid)`**. Never `sub` (pairwise per app), never `preferred_username` (mutable) |

**Direct Entra, no broker.** §6.1.

**The BYO-customer-Entra tier, later.** The data model is keyed by `(tid, oid)` and an org *is* an
Entra tenant from day one, so the later tier needs no migration. It needs the two registrations
switched to `AzureADMultipleOrgs`, the customer's admin consent, and the customer's `tid` added as an
org row (contract §5.1). A token from a tenant with no org row answers **403
`tenant_not_enrolled`**. The customer-hosted tier is the same binary with the customer's own
registrations in its configuration.

**Local development — `devissuer`.** A KPMG app registration is an IT request with its own lead
time. `control-plane/cmd/devissuer` is a loopback-only OIDC issuer (~500 lines of Go, standard
library only) that serves the **same five things the control plane uses from Entra**: the discovery
document, `/authorize`, `/token` (authorization code with PKCE S256 enforced; refresh with rotation),
the JWKS, and Entra-v2-shaped access tokens (`iss`, `aud`, `azp`, `scp`, `tid`, `oid`, `name`,
`preferred_username`, `roles`, `ver`). `/authorize` shows a page listing the users in its
configuration file; clicking one signs in as that user, which is how a developer tests two teammates
on one laptop. The control plane is configured with an **authority URL** and discovers every
endpoint from it, so the code path is identical. What `devissuer` cannot reproduce — conditional
access, MFA, consent prompts, KPMG's real token sizes and group overage — is listed in contract §6.4
and verified at the first sign-in against the real tenant.

The Hub refuses to start with a non-`https` authority unless `PANACEA_APP_HUB_DEV_ISSUER=true`, and
refuses that flag when `PANACEA_APP_HUB_ENV` is anything but `local`.

### 5.3 Data model and authorisation

| Entity | Key | Notes |
|---|---|---|
| **org** | `org-…` | one per Entra tenant (`entra_tenant_id` unique). v1 has one: KPMG's |
| **user** | `u-…`; unique `(tid, oid)` | created on first authenticated call. `display_name` and `email` refreshed from the token on each sign-in; never used as a key |
| **team** | `tm-…` | inside one org. The creator becomes its first `admin` |
| **membership** | `(team, user)` → `admin` · `member` · `viewer` | the only grant of project access |
| **invite** | `iv-…` | a team admin invites **an email address in the same org**; it binds to the first user of that tenant whose token carries a matching `preferred_username` or `email`, case-insensitive. No Microsoft Graph call, so no Graph consent |
| **project** | `pj-…` | owned by one team |
| **project remote** | `(project, repo_key)` | `repo_key` is phase 3's `githost.RepoKey` (H-3): `github.com/<owner>/<repo>` or `<org>.visualstudio.com/<project>/<repo>`; host `github` · `azure-devops` (`OD-4`). **Unique per org**, so a repository resolves to one project |
| **brief** (head) | `bf-…` | project- or branch-scoped. A branch is named by its **git ref**, `refs/heads/…`, the one name teammates share through the remote |
| **brief revision** | `brief:bf-…@n` | immutable. Content is stored as its **canonical bytes**; the digest is SHA-256 of exactly those bytes |
| **design approval** | `ap-…`; unique `design:<canvasId>@<rev>` | approval, design link and snapshot in one row, written in one transaction |
| **session → design link** | `ln-…` | shape reserved by contract §3.2, fixed here |
| **release** | `rl-…` | tagged union `run` · `rollback` (R5); the pin is taken by the Hub at reservation (G1) |
| **release → deployment join** | `(release, deployment_id)` | a second record referencing the first (contract §2.2) |
| **evidence** | `ev-…` | phase 3's `evidence.Record` (H-2), with `recordedBy` = the principal (H-4) |
| **blob** | `sha256:…` per org | bytes in Postgres (G2) |
| **audit event** | `seq` | append-only, hash-chained (G5); also the change feed (G3) |

**Local ids cannot be shared ids** (§1.5), so the Hub mints its own, with prefixes disjoint from
the control plane's (`s-`, `p-`, `b-`, `w-`, `m-`) and 16 hex characters rather than 8: a server-wide
id space for every user is not a laptop's.

**Who may do what.** Team role decides everything inside a project. Contract §5.3 has the full
matrix; the shape of it:

- **viewer** reads the project and everything under it.
- **member** also writes brief revisions, approves designs, reserves and confirms releases, records
  evidence, uploads blobs and writes session links.
- **admin** also manages members, invites and project remotes.
- **org admin** — the Entra app role `Hub.OrgAdmin`, assigned by IT — reads the org's audit log and
  performs erasure. It grants **no read of project content**; erasure addresses a key, not a text.

A caller with no membership gets **404** for the project and everything under it, never 403, so
project ids cannot be probed. Every write records its principal from the token; **no request body
carries an actor, an approver or a recorder field**.

