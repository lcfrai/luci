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

**Revised cost.** H-min (§5.9) is about **16 engineer-days**: 11 on the Hub server and 5 on the
control-plane and desktop side (sign-in, credential store, Hub client, local caller key). Split
across two engineers it is about eight working days. The first route phase 4 can call
(`POST …/brief-revisions`) is reachable on **day 3** of the server track. Estimates assume engineers
who have read this ADR and the contract, and that phase 8's `internal/entra` lands first or is
written here to its interface. The grafts account for about 4½ of the 16 days.

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

### 5.4 Desktop integration

**Who calls the Hub.** The control plane, and nothing else (answers phase 8's HA-6). It calls with
the user's own access token for `Hub.Access`, through `internal/hub/hubclient`. The renderer calls
the control plane exactly as today; the control plane's Discover, Design and Deploy handlers call
the Hub behind it. The Hub rejects any request that carries an `Origin` header, so a webview that
somehow held a token still could not use it (contract §11.4).

**What changes on the control-plane hop.** Once the Hub exists, a process that reaches
`127.0.0.1:8730` can act on shared org data as the signed-in user — not only drive local sessions.
The provenance contract's §6.6 left that exposure to *"the owner of control-plane authentication"*,
and the owner decisions give that ownership to the Hub phase (`:35-36`). So H-min includes a
**local caller key** (contract §9.4): the control plane writes a random 256-bit key to a file only
the user can read, the Tauri shell reads it through one command and hands it to the renderer, and
every control-plane route except the WebSocket attach requires `Authorization: Bearer <key>`. It
stops other OS users and **a sandboxed agent that can reach the host's loopback** (whether Docker
Desktop forwards a container to a host service bound on `127.0.0.1` is **unverified**, and the key
makes the answer not matter). It does not stop another process of the same user; nothing short of
an OS-level peer check does, and that is recorded as risk R4 (§8).

**When the Hub is unreachable.**

| Operation | Behaviour |
|---|---|
| Resolve a key the control plane already holds | served from its in-memory cache — revisions are immutable, so the copy cannot be stale |
| Resolve a new key, or read a head | `unavailable` (contract §4.1's sixth state), via 503 `upstream_unavailable` |
| Any write — a brief answer, an approval, a Ship | refused with 503 `upstream_unavailable`; nothing is queued. A queued approval or pin replayed later would record the wrong moment |
| Start a session | Materialise needs the head; phase 4's `4-D1` recommendation decides (refuse with an override) |
| Local sessions, terminals, worktrees | unaffected. They never touch the Hub |

**What the desktop may cache — contract assertion A5 kept, and widened.** A5 says nothing
provenance-related is persisted in the desktop. It stays, and this ADR widens it to **the user's
machine**: neither the renderer nor the control plane persists Hub data to disk. The control plane
may hold immutable revisions **in memory** for its lifetime. Two reasons, either sufficient:
**erasure** (G6) — a disk copy on a laptop outlives an erase and is out of our reach — and **P4**,
since a disk cache something renders from is a copy by the contract's own definition. The cost is
that Discover cannot be edited offline. `PROGRAM.md`'s *local-first* means the whole stack runs on a
laptop, not that it runs without a network, and the Hub runs on the laptop too (§5.7).

### 5.5 Realtime

**v1 polls a change feed; it pushes nothing.** Every Hub write appends one audit event under one
lock, so events are numbered in commit order (G3). `GET /v1/changes?after=<seq>` returns the events
for projects the caller can read. The control plane polls it every 5 seconds while signed in — one
request per user, whatever the number of projects — and passes the cursor to the desktop in a
response header on the call the poller already makes (contract §9.3). A teammate's approval
appears in at most one poll interval plus one desktop tick.

Why not SSE or WebSocket now: the desktop polls the control plane (`poller.ts`), so a push from the
Hub would end at the control plane anyway, and a poll of an ordered feed is correct under every
failure without reconnect logic. **SSE is v1.1 and additive**: `GET /v1/changes/stream` serves the
same event objects with the same cursor as `Last-Event-ID`.

The provenance contract's §6.4 said head refs are not polled *"unless another writer appears"*.
Teammates are that writer, so its ruling is reopened and closed here: heads are invalidated by the
change feed, not polled per brief.

**The multiplayer canvas relay is not in v1.** Phase 5 keeps live documents in the control plane in
v1 and plans to host **the same Go package** in the Hub for v2 (`paper-design-cleanroom-spec.md:380`,
`:735`). The Hub is shaped so that is additive: a `/v1/canvas/…` WebSocket route on the existing
server, identity from the same validator, authorisation from the same membership. Yjs is not
chosen, because phase 5's model is server-authoritative last-writer-wins per property (`:647`).

### 5.6 Storage

- **Postgres 16** — phase 8 provisions 16 (HA-5, task 08); locally `postgres:16-alpine`. **No
  extensions**: `sha256()`, `gen_random_uuid()`, row-level security and JSONB are all core.
- **Migrations**: plain SQL files embedded in the binary, applied in order by a ~120-line runner in
  `internal/hub/migrate` under `pg_advisory_lock`, recorded in `schema_migrations` with a SHA-256 of
  each file so an edited, already-applied migration is refused. Run as **`hub migrate`**, a
  subcommand, by phase 8's pre-upgrade Job under the DDL identity. `hub serve` never migrates; it
  refuses readiness until the schema is at the version it was built for. Rejected: `golang-migrate`,
  `goose`, `atlas` — each is a dependency, and some a CLI, for a hundred lines we can read.
- **Blobs**: a `blobs` table (`bytea`, content-addressed by SHA-256, per org), behind a `blobstore`
  interface. Caps: 16 MiB a blob, 1 GiB a project. Phase 8 provisions no storage account for the Hub
  (HA-7's default), and this keeps it that way. When design assets outgrow Postgres the interface
  gets an Azure Blob implementation; the routes do not change.
- **Audit**: `audit_events`, append-only by grant and trigger, each row carrying the SHA-256 of the
  previous row's hash and its own canonical fields. It records every write and every security event
  (denials, sign-in of a new user, erasure). It never records brief text or blob bytes.
- **Erasure**: an org admin erases a brief. Its revisions keep **key, digest, author and time**; their
  content is replaced by nothing, `erased_at` is set, and the key then resolves as **`erased`** — a
  seventh provenance state, distinct from `missing` (data loss) and from `none`. It is done by one
  `SECURITY DEFINER` function owned by the DDL identity, the only code path allowed to touch a
  revision. Blobs referenced only by erased records are deleted the same way. Audit rows are not
  erased; they hold no content. Open question Q2 (§10) asks the owner to confirm the rule.
- **Australian data residency**: the Hub, its database and its backups are in **Australia East**
  (phase 8's region, `P8` owner-inputs table `:259`), with geo-backup, if enabled, only to the paired
  **Australia Southeast**. The Hub makes no outbound call carrying user content. Entra itself is a
  global service outside this design's control — noted for the owner, not solved here.

### 5.7 Operations

- **Local-first.** `control-plane/cmd/hub/compose.yaml` (contract §12.1) — beside the binary, not
  under the repo-root `deploy/` tree phase 8 owns — starts `postgres:16-alpine`, `devissuer`, a one-shot `hub migrate` and `hub serve` on
  `127.0.0.1:8750`. The control plane is pointed at it by two variables. No Azure resource is
  needed to develop or test any Hub route.
- **Health.** `GET /healthz`: the process is up. `GET /readyz`: Postgres answers within 1 s, the
  schema version matches the build, and the JWKS for every enrolled tenant (or `devissuer`) has been
  fetched once. Both unauthenticated, both answer no detail beyond `{"status":…}`.
- **Configuration**: every variable is `PANACEA_APP_HUB_*` (contract §12.2), read in one `config.go`
  block exactly as `control-plane/cmd/api/config.go` does. No secret is a variable in any deployed
  environment: the database is reached with workload identity and an Entra token (phase 8's
  `P8-D10`, Entra-only Postgres), fetched by a ~70-line standard-library token exchange, so the
  Azure SDK is not a dependency.
- **Deploy (phase 8).** The chart phase 8 is writing (`deploy/charts/hub`, task 29) holds: listen on
  `:8080`, `/healthz`, `/readyz`, stateless, two replicas, a pre-upgrade migrate Job, 20 s graceful
  shutdown. Contract §12.3 answers HA-4 and HA-5 field by field.

### 5.8 Security

The top-ten threat model is contract §11.1. The decisions that carry it: tenancy by `org_id` in
Go **and** in row-level security (G4); project isolation by membership in Go, with every route
tested as a non-member (contract §13); token audience separation (Hub, broker, Azure DevOps and
Panacea tokens are four audiences, and none is ever forwarded to another service or into a
sandbox); no CORS and no `Origin` accepted; per-user and per-org rate limits in process; a DML
identity that cannot `UPDATE` or `DELETE` an immutable table; and a DDL identity used only by the
migrate Job.

### 5.9 Decomposition — the minimum Hub, and what comes after

**H-min is what phases 3, 4, 5 and 7 need, and nothing else.** Task ids are the contract's §13.

| Track | Task | Unblocks | Days |
|---|---|---|---|
| server | **H0** skeleton: `cmd/hub` (`serve`, `migrate`), config, health, migrate runner, migration `0001`, compose, import-boundary test | everything | 1.5 |
| server | **H1** `devissuer`, and `internal/entra` to phase 8's interface if task 16 has not landed | every authenticated route | 2 |
| server | **H2** users, orgs, teams, members, invites, `/v1/me` | membership, so authorisation | 1.5 |
| server | **H3** projects, remotes, briefs, the revision batch write, the canonical digest, `/v1/refs`, the branch head | **phase 4** | 2 |
| server | **H4** blobs, design approvals, session → design links | **phase 5** | 1.5 |
| server | **H5** releases: reserve, confirm, abandon, rollback, the deployment join | **phase 7** | 1.5 |
| server | **H6** evidence | **phase 3** (H-2) | 0.5 |
| server | **H7** change feed, audit chain, erasure | teammates see changes; Q2 | 1 |
| client | **C1** `entraauth` + `credstore` + `/api/v1/auth/*` on the control plane | sign-in; phase 8's HA-2; phase 3's H-1 | 2 |
| client | **C2** `hubclient`, the relay rule and `upstream_unavailable` for `hub` | every phase's Hub calls | 1 |
| client | **C3** the local caller key, control plane and desktop | the §6.6 exposure | 1 |
| client | **C4** the desktop's sign-in surface: signed-out, signing-in, signed-in, re-auth | the owner can sign in | 1 |

**Order.** H0 → H1 → (H2 ∥ C1) → H3 → (H4 ∥ H5 ∥ H6 ∥ C2) → H7, C3, C4. Phases 4, 5 and 7 build
against the contract in parallel with the Hub and integrate against `compose.yaml` when H3, H4 and
H5 land. Until then, each phase codes to `hubclient`'s interface with its in-memory fake
(contract §9.2), which H0 ships first.

**After H-min, in the order the program is likely to need them:**

1. **SSE** on the change feed (`/v1/changes/stream`) — when the one-poll latency is noticed.
2. **Phase 5 v2** — the canvas package hosted in the Hub over WebSocket; presence; cursors.
3. **The BYO-customer-Entra tier** — multi-tenant registrations, per-customer admin consent,
   an enrolment route for org admins.
4. **Audit anchoring** — the chain head exported to Log Analytics hourly, so a rewritten chain is
   caught outside the database (phase 8's DCR).
5. **Azure Blob** behind `blobstore`, when a project nears its 1 GiB cap.
6. **Server-signed release manifests** — when a manifest must verify outside our system.
7. **A people picker** through Microsoft Graph, replacing invite-by-email — needs Graph consent.
8. **Separation of duties** — an org policy that an approver may not approve their own design.

---

## 6. Rejected alternatives

### 6.1 A broker (Keycloak) in front of Entra

Rejected. **`OD-6`** asks for Entra SSO with auth-code + PKCE, and a broker adds a second token
issuer the Hub must trust and the owner must operate. Panacea's Keycloak is a recurring outage class:
one replica, in-memory sessions, a five-minute rebuild (`env` worktree,
`planning/env/survey-own-environment.md:316`, and its §1.7). Phase 8 already recommends **no broker
in v1** for itself and leaves the Hub's choice to this ADR (phase 8 plan `:119`, OQ-4). The
only thing a broker buys that we need is customer identity federation, and Entra's own multi-tenant
applications do that for Entra customers (§5.2). A customer on a non-Entra IdP is the one case that
reopens this; none is on the roadmap.

### 6.2 The Tauri shell runs the flow and holds the tokens

Rejected. The consumers of the token are all in Go — the Hub client, phase 8's broker backend,
phase 7's Panacea client and phase 3's git host clients. A shell-held token crosses a process
boundary on every call, the shell gains a keyring crate, and `OD-5`'s *"the control plane calls it
as the user"* stops being literally true. The one argument for the shell — that it can open the
browser — is met by the renderer calling `tauri-plugin-opener` with a URL the control plane built.

### 6.3 A new `hub/` Go module, with `httpx`, `ids` and `entra` lifted into a shared module

Rejected for now (§4.1). It is the cleaner end state and costs a sweep of import paths across the
control plane. **Revisit** when no phase branch is open against `control-plane/internal/api`. Nothing
in this design depends on the module the code lives in, so the move is mechanical.

### 6.4 An event-sourced store (Design C)

Rejected (§4.2). It turns every read into a projection to keep correct. The Hub's only records that
must never change — revisions, approvals, pins, evidence — are made immutable directly.

### 6.5 The renderer calls the Hub directly

Rejected. It needs the token in the renderer (the most exposed process, which renders agent output
and, from phase 5, design frames), CORS at the Hub's edge, and a second client. It also splits
provenance writes across two processes, which the provenance contract's P7 forbids.

### 6.6 Membership from Entra group claims

Rejected for v1. It needs IT to create a group per team, and KPMG users are likely to hit **group
overage** — past 200 groups the token carries a Graph pointer instead of the list, and resolving it
needs Graph consent. Team membership lives in the Hub, managed by team admins. **Unverified:** KPMG
users' group counts.

### 6.7 An object store for blobs in v1

Rejected (G2). Phase 5's v1 snapshots are a handful of HTML files, PNGs and token files per approval.
Postgres holds them transactionally with the approval that references them, backs them up with
everything else, and erases them in the same transaction as their records.

### 6.8 A disk cache or an offline write queue on the laptop

Rejected (§5.4). A cache outlives an erase; a queue replays an approval or a pin at the wrong
moment.

### 6.9 A per-user store synchronised between machines

Rejected by `OD-3` itself: shared from day one means one source of truth, not reconciled copies.

### 6.10 gRPC or GraphQL

Rejected. The control plane already speaks REST + JSON with one problem shape to the desktop; a
second style doubles the error model for no caller that needs streaming or graph queries.

---

## 7. Consequences

### 7.1 For each phase

| Phase | What changes for it |
|---|---|
| **4 Discover** | `discoverstore.Store` gains a Hub-backed implementation; the file store becomes a test double. Brief ids are Hub-minted, `bf-` + 16 hex, so the `provenance` grammar must accept them (contract §3.2). A branch is named by its git ref. Every write goes through one batch route, so a project edit that re-revisions each inheriting branch brief (critique-2 §7.1 #3) is one transaction. `GET /api/v1/discover/refs` becomes a control-plane handler over the Hub's `GET /v1/refs`. `Ref.resolution` gains **`erased`** |
| **5 Design** | Approval is one Hub call that writes the approval, the design link and the snapshot's manifest together, after the snapshot's blobs are uploaded. The approver is the token's principal, never a field. Canvas ids should be minted with 16 hex (contract §3.2) so they stay unique when v2 moves canvases into the Hub |
| **7 Deploy** | Ship is **reserve → deploy → confirm** (G1). The Hub, not phase 7, resolves which revision is pinned. Rollback is R5's `rollback` shape and copies its pin from the release it restores. Ship and rollback are disabled while the Hub is unreachable — which the prototype already does for Panacea (`phase-7-deploy.html:670-674`, per ruling R4) |
| **3 Git** | Evidence (H-2) is a Hub route; `Shared()` becomes true. H-1's `credstore` is the control plane's, built here in C1. H-3's `RepoKey` is the Hub's project-remote key. H-4's recorder is the token's principal |
| **8 Environment** | The chart's provisional defaults hold (contract §12.3). Three registrations, as task 09 already guessed, with the details in §9. No storage account. The DB identities `id-hub` and `id-hub-migrate` are the DML and DDL roles |
| **Desktop** | New: a sign-in surface; the local caller key on every control-plane call; one response header read by the poller; `upstream_unavailable` with `upstream: "hub"` |

### 7.2 Documents this ADR amends — each amendment is made by the phase that next edits the file

- **`PROGRAM.md`**, identity line: *"EntraID via … Keycloak"* → Entra direct, auth-code + PKCE run by
  the control plane, validated by the Hub (`OD-6`, this ADR §5.2).
- **`desktop/src/api/client.ts:6-8`** and **UI contract §4.5**: *"no Authorization header before
  Phase 8"* → the local caller key from task C3.
- **Provenance contract**: §2.1's store locations move to the Hub (`OD-3`); §1.4 gains `erased`;
  §6.4's *"head refs are not polled"* is replaced by the change feed; §6.6's open exposure is closed
  by C3; A5 is widened to the whole machine (§5.4).
- **Phase 8 plan**, HA-1…HA-7: answered in contract §12.3 and §14.

### 7.3 What gets harder

- **Discover needs a network.** A laptop with no route to the Hub can run sessions but cannot edit
  a brief, approve a design or ship. This is `OD-3`'s price, stated plainly.
- **Two processes must agree on a digest.** The Hub writes it and the control plane re-checks it,
  both from the same Go package, so they cannot drift — but a TypeScript reimplementation would.
  The desktop never computes one (P5).
- **A second binary to run locally.** One `docker compose up`, documented in contract §12.1.

---

## 8. Open risks

| # | Risk | Likelihood · impact | Mitigation, and what remains |
|---|---|---|---|
| R1 | A KPMG refresh token exceeds Credential Manager's 2,560-byte blob | medium · sign-in cannot persist | chunked targets (contract §9.1). Measured at the first real sign-in |
| R2 | KPMG conditional access blocks, or adds a device-compliance prompt to, a public client with a loopback redirect | medium · nobody signs in | ask IT before the first real sign-in (§9, item 6); `devissuer` keeps all development moving meanwhile |
| R3 | A crash **after** Panacea accepts a deploy and **before** confirm leaves a reserved release that expires as `abandoned` although a run happened | low · one release shows *"reserved, never confirmed"* | confirm is idempotent and retried; if the Panacea metadata ask (R7) is granted, the deployment poller confirms by the `releaseId` it finds on the run. It never pins by observation (P2) |
| R4 | Another process **of the same OS user** drives the control plane, and so the Hub, as that user | low · acts as the user | C3 stops other users and sandboxes, not this. An OS peer-credential check is later work |
| R5 | An agent causes an approval. The Hub cannot tell a human's call from the control plane's | low · a false approval | phase 5's rule that no MCP tool approves; the approve handler is reachable only from our chrome; every approval is audited with its session context |
| R6 | The Hub's packages start importing control-plane internals because the compiler allows it | medium · the Hub drags in local-only code | the import-boundary test in H0 (§4.1) |
| R7 | A later writer encodes the canonical form differently and every digest changes | low · every link reads `missing` | the encoding is specified to the byte (contract §3.4), with golden vectors checked into tests |
| R8 | Entra-only Postgres: a pooled connection outlives the token it was opened with | medium in cloud · connection errors | `MaxConnLifetime` 45 minutes, below the token's life; a token is fetched per new connection |
| R9 | `devissuer` diverges from Entra in a way the code relies on | medium · the first real sign-in fails | contract §6.4 lists every known difference; H1's acceptance test is one real-tenant sign-in |
| R10 | One lock serialises every write (G3, G5) | low · write latency under load | writes are human-paced; the lock is held for one insert. Measure before optimising |
| R11 | These scores are one author's, over reconstructions (§0) | — | `OD-1`'s adversarial review of this ADR and the contract, told so |

---

## 9. What the owner and KPMG IT must provide

Nothing below blocks development: `devissuer` runs the whole flow locally. Items 1–6 block the
**first real sign-in**. Item 9 is phase 8's and is listed so the whole ask goes to IT once.

| # | For | Exactly what | Who |
|---|---|---|---|
| 1 | Tenant | The **KPMG Australia tenant id** (a GUID) the registrations live in | IT |
| 2 | **Desktop registration** — display name *PanaceaApp Desktop* | Single tenant (*"Accounts in this organizational directory only"*). Platform **Mobile and desktop applications**, redirect URIs **`http://localhost`** and **`http://127.0.0.1`** (the second may need the manifest editor: `replyUrlsWithType`, type `InstalledClient`). **No client secret, no certificate.** *Allow public client flows*: **No** — auth-code + PKCE does not need it. Delegated permissions: `openid`, `profile`, `offline_access`, `email`, `api://<hub-id>/Hub.Access`, and phase 8's `api://<broker-id>/Sandboxes.Access`. Optional: Azure DevOps `user_impersonation` if Q5 is yes. **Tenant-wide admin consent** for all of them, so no user sees a consent prompt | IT |
| 3 | **Hub API registration** — *PanaceaApp Hub API* | Single tenant. Application ID URI **`api://<its-client-id>`** (the default; a custom URI needs a verified domain). Exposed delegated scope **`Hub.Access`** (*"Use PanaceaApp's shared workspace as you"*). **Authorised client application**: the desktop's client id, for `Hub.Access`. Manifest **`requestedAccessTokenVersion: 2`**. App role **`Hub.OrgAdmin`** (allowed member types: users and groups). Optional claim **`email`** on the access token. **No secret, no certificate** — the Hub calls nothing as itself in v1 | IT |
| 4 | Enterprise application for the Hub API | **Assignment required: Yes**, assigned to group *PanaceaApp Users* — so only assigned people can obtain a Hub token at all | IT |
| 5 | Groups | **PanaceaApp Users** (security group; the owner and the pilot team). **PanaceaApp Org Admins** (assigned the `Hub.OrgAdmin` app role; the owner). Phase 8's Postgres administrator group (`P8-D10`) if not already requested | IT, owner names members |
| 6 | Conditional access | Confirmation that KPMG's policies allow a public desktop client with a loopback redirect, and what they require of the device (compliant, hybrid-joined). If a sign-in-frequency policy applies, its interval | IT |
| 7 | Decisions | Q1–Q7 in §10 — each has a recommendation this design already plans around | owner |
| 8 | Nothing else | No Graph permission, no client secret, no certificate, no Keycloak, no API Management, no storage account | — |
| 9 | Phase 8's broker registration | *PanaceaApp Sandbox Broker*, scope `Sandboxes.Access`, desktop pre-authorised — as phase 8's task 09 writes it | IT |

---

## 10. Open questions for the owner — each with the recommendation planned around

| # | Question | Recommendation (what the contract builds) |
|---|---|---|
| Q1 | May only a **complete** revision be pinned or approved against? | **Yes.** The Hub refuses an approval against an incomplete revision (`brief_incomplete`). At Ship it pins the **latest complete** revision, and records *"no complete revision"* if there is none. This is the rule all three prototypes draw (`prototype-fact-sheet.md:56-60`) and critique-2 §2.2 asks for |
| Q2 | What does **erasure** of a brief do to a release that pinned it? (provenance contract Q5) | Content removed; key, digest, author and time kept; the pin renders **"erased on <date>"** — a state distinct from `missing` (§5.6) |
| Q3 | May a designer approve their own design? | **Yes in v1**, as the prototype draws; separation of duties is an org policy later (§5.9 item 8) |
| Q4 | How do teammates join? | A team admin **invites an email address in the same tenant**; it binds at that person's first sign-in. No Graph people picker in v1 |
| Q5 | Should Azure DevOps use the **same Entra sign-in**? | **Yes** — one refresh token serves ADO, so no ADO PAT is needed (`OD-4`, `OD-6`). Needs item 2's optional permission. GitHub keeps its own token in the same store |
| Q6 | When the Hub is down, may writes queue and replay? | **No.** A queued approval or pin would record the wrong moment. Writes are refused and say why |
| Q7 | Does the Hub, in its first cut, hold canvas documents? | **No** — phase 5's v1 keeps them in the control plane; the Hub hosts that package in v2 (§5.5) |

---

## 11. What this ADR did not verify

- **Nothing was built or run.** No Go was compiled, no container started, no token issued.
- **Entra behaviour stated from knowledge, not tested here:** that a registered `http://localhost`
  or `http://127.0.0.1` redirect matches any runtime port and the root path; that v2 access tokens
  carry `preferred_username`, `name` and `azp` with the `profile` scope; token lifetimes; refresh
  token rotation on redemption; that one refresh token can be redeemed for the Hub, the broker and
  Azure DevOps. Each is checked by H1's real-tenant acceptance test.
- **Windows:** the 2,560-byte credential blob cap is from the Win32 documentation's constant as
  recalled; KPMG token sizes are unknown (R1).
- **`tauri-plugin-opener`:** present in `Cargo.toml` and granted as `opener:default` — read. Whether
  `opener:default` permits an `https://login.microsoftonline.com/…` URL without a scope entry was not
  checked.
- **Docker Desktop reachability** of a host service bound to `127.0.0.1` from inside a container:
  not tested (§5.4). C3 is designed so the answer does not matter.
- **Sibling plans were read as they stood on 2026-09-23**, several uncommitted. Phase 8's task 16
  part, which will hold the `entra.Validator` source, **did not exist yet** (its parts directory
  held tasks 01–05); the interface this contract assumes is written in contract §6.3 and must be
  reconciled with task 16 when it lands.
- **Phase 4's resolution rule** — a branch brief if one exists, else the project brief — is read
  from the phase 4 plan's `API.resolve` (`:4250-4261`) and `discoverstore.Store` (`:3019`), not
  from built code; no phase 4 code exists.
- **Azure Postgres Flexible Server** supporting major version 16 and the core functions used
  (`sha256`, `gen_random_uuid`, row-level security) is from knowledge and phase 8's choice, not
  from a provisioned server.

*End of ADR-H1. The contract is `docs/superpowers/contracts/2026-09-23-hub-contract.md`.*

