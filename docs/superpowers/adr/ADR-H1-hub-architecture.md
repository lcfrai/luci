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

