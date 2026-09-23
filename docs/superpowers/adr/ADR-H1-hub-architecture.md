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
