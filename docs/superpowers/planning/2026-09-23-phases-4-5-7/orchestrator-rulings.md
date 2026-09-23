# Orchestrator rulings — phases 3, 4, 5 and 7 planning pass

**Date:** 2026-09-23. **Status:** decided. These are the decisions `completeness-critique-2.md` §7.4
and `provenance-contract-draft.md` §7.3 name as **the orchestrator's, not the product owner's** —
scheduling, internal structure and contract mechanics. Each is recorded as
**ruling — reason — cost if wrong**, so a later plan can cite it and a later reader can overturn it
knowing what it costs.

**What is deliberately NOT here:** anything that changes what the product does for a user. Those are
in `completeness-critique-2.md` §7.1–§7.3 and are the owner's — per person or shared, what a key
names, who owns control-plane authentication, the AGPL question, and the rest.

---

## R1 — Decision 0c: who calls Materialise → **option 1**

**Ruling.** A **`Materialiser` interface declared in `control-plane/internal/api`**, satisfied by
phase 4's `contextwrite`, held in `api.Deps`, and called **between worktree-resolve and
`backend.Create`** in `handleSessionCreate` and `handleSessionRestart`. It **returns the key it
materialised**, and those handlers stamp `Session.ContextRef` from the return value. Phase 4 owns
`sessions.go` for this change.

**Reason.** Option 3 (client-enforced) is already excluded by the contract's A3 — a client cannot
know what the server's worktree held when the PTY started. Between the two structural options, option
1 makes the guarantee **impossible to bypass** and gives A3 exactly what it needs, a key returned by
the code that did the writing. Option 2 (materialise when `contextRef` is absent or stale) makes the
same guarantee through **more state**: a staleness test is a second place the truth lives, and a
wrong staleness answer silently skips materialisation. Declaring the interface in `api` keeps the
package dependency pointing the right way.

**Cost if wrong.** One interface, one `Deps` field, one validation branch in `api.New`.

**Seam note, for task 26.** Phase 2's `resume-fix` is editing `handleSessionCreate` right now to mint
a Claude session UUID (`--session-id`) and resolve it on resume. **The `Materialiser` call lands in the
same function, between the same two steps.** Task 26's `SEAMS.md` should name this as one seam with
two occupants and state their order: materialise first, so the stamped `ContextRef` and the minted
session id both describe the process that actually starts.

## R2 — Decision 0d: `writeAtomic` → **lift it into a shared package; do not duplicate**

**Ruling.** Move `writeAtomic` from `control-plane/internal/registry/filestore.go:460` into a shared
package (the contract calls it `statepath`), **with its tests moving with it**, and have both the
registry and phase 4's revision store call the one implementation.

**Reason.** This function is not generic plumbing. **Task 7's plan carried a Critical** because
`os.Open` on Windows does not request `FILE_SHARE_DELETE`, so an atomic-replace test failed
deterministically on Windows while pointing at correct code; the fix inverted the test on Windows
rather than skipping it. **A duplicate is a second copy of a subtle Windows-specific behaviour that
does not carry that hard-won test.** One implementation, one test, one place to be wrong.

**Cost if wrong.** Phase 4 edits a closed phase-2 file for the lift — a mechanical move with its
tests, done once, as a declared departure.

## R3 — Persist-version allocation → **adopt the contract's §6.5 rule, plus one clause**

**Ruling.** As proposed in `provenance-contract-draft.md` §6.5:

1. A phase **claims a persist version in its plan text** when the plan is written, not at merge.
2. `migrate` becomes an **ordered chain of per-version steps** (`1→2`, `2→3`), each **total over
   `unknown`**, because it runs before any error boundary exists.
3. A phase that merges second **renumbers its step and chains it**. There is never a second "v2".
4. A new persisted per-branch key **ships with its prune** — the unbounded `pinned` blob is an open
   carry, and copying its shape copies the defect.

**Plus: 5. A renumber at merge updates the claiming plan's text in the same commit.** This phase
learned repeatedly that a fact changed in one place and left standing in another is how two sources
come to disagree.

**Reason.** Allocating one shared counter across concurrent lanes is a scheduling problem, and the
rule turns it into a mechanical one. Provenance itself persists nothing (contract A5), so phase 4
claims no version.

**Cost if wrong.** A renumber at merge time — which the rule already expects.

## R4 — The problem-code ruling (critique-2 §3.5) → **one new kind, `upstream-down`, written once**

**Ruling.** A **fourth `ProblemKind`, `upstream-down`**, meaning *a remote service the control plane
depends on — the Panacea API, or the design canvas server — is unreachable*. The server emits a
distinct `ProblemCode`, `upstream_unavailable`, with `details.upstream` naming which one. `classify`
maps the code to the kind. **Both phase 5 and phase 7 use it**; whichever builds first adds it, as a
declared amendment to phase 2's UI contract §4.2 and `desktop/src/api/problems.ts`.

**Reason.** Deploy's brief proposed routing every Panacea failure through `server-said`, so that phase
2 needs no change — but **Deploy's own prototype disables Ship and rollback when offline**
(`phase-7-deploy.html:670-674`), and `server-said` is a sentence rendered verbatim. The desktop cannot
turn Ship off on a sentence without parsing it. It needs a kind.

**And adding a kind is the enforcement working, not a cost of it.** `classify`'s three call sites each
switch over `ProblemKind` with a declared return type and no `default:`, so a fourth kind is a
**compile error at every site** under `strict: true` — each site is forced to decide what to show. That
is precisely why task 20 built it that way.

**Cost if wrong.** One union member and a decision at each call site.

## R5 — The rollback key shape (`scope-and-risk-phase-7-deploy.md:225`) → **a tagged union of two key shapes**

**Ruling.** The release link's key is a **tagged union**, not one shape stretched over two events:

- `{ kind: "run", projectName, pipelineRun }` — a build that ran.
- `{ kind: "rollback", projectName, environment, deploymentId, at }` — a rollback, which creates no
  PipelineRun.

**Never synthesise a `pipelineRun` for a rollback.** A rollback **inherits the pin of the deployment it
restored** (contract §3.4): it copies that deployment's link's brief key and digest when one exists,
and shows *"no brief is recorded for this release"* when none does. Phase 4 owns the ledger schema;
phase 7 raises the change.

**Reason.** A fabricated run name would make a rollback indistinguishable from a build in the ledger,
and the difference is exactly what an auditor asks about.

**⚠️ One premise is unverified.** *"No PipelineRun is created"* is **inferred** from the rollback
handler's return type and a field comment — the phase 7 brief says it did not read
`RollbackDeployment`. **Phase 7's plan must read that function before relying on this ruling.** If a
rollback does create a run, the `run` shape covers it and the union is unnecessary.

**Cost if wrong.** One extra shape in one union.

## R6 — Non-session keys in the one loader (critique-2 §1.2) → **the loader unions key sources**

**Ruling.** The one provenance loader collects keys from **a set of key sources**, not from sessions
alone. Each lens that displays keys contributes **a selector in `store/selectors.ts`** returning the
keys it currently shows — sessions' `contextRef`s, Deploy's release pins, Design's canvas binding. The
loader unions them and issues **one batched request** for the keys it does not yet hold.

The loader runs on **the existing poll tick and on key-set changes — never on a new interval.** There
are exactly two `setInterval` call sites in the non-test tree, the display heartbeat and the data
poller, and a third would violate the one-clock rule a reviewer checked twice in phase 2.

**Reason.** The alternatives are both worse: each lens fetching for itself violates the contract's P8,
and growing `poller.ts`'s own contract each time a lens holds a key moves a phase 2 contract every
phase. A source registered as a selector is **additive** — a new lens adds one selector and changes
nothing else — and shared-shape §4 already says derived reads belong in `selectors.ts`.

**Cost if wrong.** One selector per lens.

## R7 — The Panacea metadata ask (contract §7.3) → **drafted; sending it is the owner's call**

**Ruling.** An additive `metadata map[string]string` on `panacea-api`'s `TriggerDeployOptions` is
worth asking for **now** — without it, a lost state directory makes pinned releases
indistinguishable from unpinned ones, and the answer has another team's lead time.

**But asking is an outward-facing request to another team, so it is not mine to send.** It is on the
owner's list with that note.

---

## Settled by measurement rather than ruling

- **The Panacea dev API base is `https://dev.panacea.kpmg/api`.** `GET …/api/v1/version` returned
  HTTP 200, `application/json`, `{"version":"v1.163.0"}`, unauthenticated.
  `https://api.dev.panacea.kpmg/…` returns **307 to `/oauth2/sign_in`** — it is behind the SSO proxy.
  Recorded in `dependency-graph-and-order.md` item 4.
- **The stale AGPL framing** in three places is corrected — `PROGRAM.md` (`3c54287`, `b9f982a`) and
  `dependency-graph-and-order.md` (`b9f982a`).
- **`claude --resume` and `CLAUDE.md` re-reads** — the experiment is running inside phase 2's resume
  fix and will be recorded in `resume-fix-report.md`.
