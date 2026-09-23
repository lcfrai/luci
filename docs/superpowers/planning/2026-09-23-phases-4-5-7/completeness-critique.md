# Completeness critique — the phases 4/5/7 planning pass

**Date:** 2026-09-23. **Role:** completeness critic. **Method:** read every file produced by this
pass, then re-verify its load-bearing claims against the tree and against the two sibling repos.
I ran no build, no test and no network call. Where I verified something myself I cite `path:line`;
where I am carrying another document's claim through, I say so.

---

## 0. The headline

**The deliverable does not exist.** The task defined the output as *a prototype plus a
scope-and-risk brief* for each of phases 4, 5 and 7. What exists in
`docs/superpowers/planning/2026-09-23-phases-4-5-7/` is four analysis documents:

| File | Size | What it is |
|---|---|---|
| `survey-phase-2-substrate.md` | 33 KB | substrate survey — good, dense, well-cited |
| `survey-phase-4-discover.md` | 17 KB | phase 4 survey — good |
| `survey-phase-7-deploy.md` | 27 KB | phase 7 survey — good |
| `dependency-graph-and-order.md` | 34 KB | order recommendation — good, honest about its own gap |

**Zero prototypes. Zero scope-and-risk briefs. No phase 5 survey.**
`prototype/` holds seven files — `app`, `index`, `v2`–`v6` — every one last modified **2026-09-17**,
six days before this pass. `prototype/phase-4-discover.html`, `prototype/phase-5-design.html` and
`prototype/phase-7-deploy.html` do not exist; `find` over the whole repo returns nothing matching.

The consequence is not cosmetic. `CLAUDE.md` says *"The product owner reviews prototypes before
implementation plans, and plans before code."* This pass produced neither half of what the gate
needs, so **the gate is still shut and nothing here advances it**. What was produced is genuinely
valuable — it is the best-cited body of work in the programme — but it is *input to* a planning
pass, not the output of one.

---

## 1. Do the three prototypes agree with the provenance contract, and with each other?

They cannot: they do not exist. But the question underneath — *is the artifact born in Discover the
same object Design references and Deploy attaches?* — is answerable from what was produced, and
**the answer is no, for a mechanical reason nobody in this pass caught.**

### 1.1 The release is attached to the brief's *present tense*, not to what shipped

Four facts, each read from the file:

1. `provenance.Ref` (`docs/superpowers/plans/2026-09-18-phase-4-discover.md:3515-3533`) is keyed by
   `BranchID` and carries a monotonic `Revision`. It has **no release, commit or deploy field**.
2. There is exactly **one** endpoint —
   `GET /api/v1/discover/branches/{branchID}/provenance` (`:3622`, registered at `:4071`) — and it
   returns the branch's **current** `Ref`. There is no fetch-by-revision and no fetch-by-release.
3. `useProvenance(api, branchId)` (`:5716`) takes a branch id and nothing else. All three surfaces
   are fed from it.
4. `ShippedManifest({ refValue, version })` (`:5598`) renders the brief beside the release. Its own
   test (`:5681`) asserts it prints *"brief b-42, revision 6"* next to `v0.4.2`.

Put together: **a release cut at revision 3 will render revision 6's headline and criteria after the
brief is edited twice more.** The Deploy lens will state, in the user's own words, that a shipped
release satisfies acceptance criteria that were written after it shipped. That is the spine claim
inverted — not *"same artifact, one thread"* but *"whatever the artifact says today, retro-fitted to
every past release"*. It is worse than no manifest, because it is confidently wrong.

For `ContextChip` (Develop) and `DefinitionOfDone` (Merge), branch-current is the **correct**
reading — those surfaces are about the work in front of you. For `ShippedManifest` it is wrong.
One type is being asked to be two things, and the plan's own rule — *"if a mounting surface needs a
field, it goes into `provenance.Ref`, never computed locally"* (`survey-phase-4-discover.md` §5) —
actively prevents Deploy from fixing it locally.

### 1.2 And Phase 7's recommended fix cannot be built against Phase 4's plan as written

`survey-phase-7-deploy.md` §6 recommends, as option 1: *"Our control plane stores
`(project, pipelineRun) -> provenance.Ref` at the moment we call `POST …/deploy`."*

Store **what**, exactly? There is no way to re-fetch a `Ref` at a pinned revision (fact 2 above), so
this is either:

- store a **pointer** (`briefId` + `branchId`) — which drifts, and reproduces §1.1 exactly; or
- store a **frozen copy of the whole `Ref` value** — which works, but is a different design: it
  means `ShippedManifest` takes a snapshot type, not a live `Ref`, and it means the deploy-time
  copy is now a second source of truth that can disagree with Discover.

Two surveys that agreed in prose are incompatible in shape, and neither noticed, because neither
wrote the type down next to the other's. **This is what a prototype would have caught in ten
minutes and 34 KB of dependency analysis did not.**

### 1.3 Phase 2's plumbed carrier is bypassed entirely

`Session.contextRef` is persisted (`control-plane/internal/registry/session.go:72-74`), typed
(`desktop/src/store/types.ts:162-163`), and propagated through create, resume and fork. Both
surveys celebrate this as "the thread's plumbing is done".

But `useProvenance` is keyed by **branch id**, not by `contextRef`. So the field phase 2 carefully
carried through five layers is **read by nothing in phase 4's plan**. A session whose `contextRef`
points at revision 3 renders the branch's current chip like every other session in the column. The
carrier and the reader are keyed on different things and no document in this pass compares them.

Verified: `grep -rn "contextRef" desktop/src/components` returns **0**.

### 1.4 One more, smaller: `useProvenance` breaks the one-poller rule

`ContextChip` is *"mounted by Phase 2 on every session header"* (`:5596`), and `useProvenance`
(`:5716`) is a per-component `useEffect` + `useState` fetch with no shared cache. A twelve-session
canvas mounts twelve independent fetches. `desktop/src/api/poller.ts:1-3` states the substrate's
rule — *"Phase 2 polls, from exactly one place, with exactly one request per tick"* — and
`CLAUDE.md` states the product rule: *"Many concurrent live processes share one global heartbeat
clock. Never a spinner per row."* Phase 4's hook is a fetch per row. It belongs in the store as a
`discoverSlice` keyed by branch, refreshed by the existing poller.

**Recommendation for §1 as a whole:** before anyone writes a task, `provenance.Ref` needs an
immutable identity (`briefId@revision`) and either a by-revision endpoint or an explicit
snapshot-at-deploy type. This is the single decision that makes the spine real or decorative.

---

## 2. Does anything contradict what phase 2 actually shipped?

### 2.1 No byte-gate breach found — genuinely good news

`desktop/src/store/types.ts` is gated by `diff -u --strip-trailing-cr` against UI contract §2
(`docs/superpowers/plans/phase-2-parts/task-15.md:280`) — **verified, the gate command is real**.
The `Lens` union at `types.ts:173` already carries all six names, so phases 4, 5 and 7 fill arms
rather than extend the model. No document in this pass proposes a `types.ts` field. Clean.

### 2.2 Live defect, flagged twice, still unfixed

`desktop/src/components/AppShell.tsx:62` renders `phase={5}` on the **deploy** arm. `PROGRAM.md:97`
says Deploy is phase 7. Both surveys flag it (at `:59` and `:61` — the line has drifted twice
today, the symbol has not). Correctly, neither survey fixed it; this pass was read-only. It needs an
owner in phase 2 closeout.

### 2.3 Unflagged: the fourth slice is a persist-version bump nobody owns

`survey-phase-4-discover.md` §4.2 correctly rejects the plan's prop-drilled `DiscoverView` and
proposes a `discoverSlice`. What it does not say is the cost. Verified:
`desktop/src/store/store.ts:14` is `CockpitState = DataSlice & UiSlice & PrefsSlice`;
`PERSIST_VERSION = 1` at `:17`; `partialize` at `:29`. Adding a persisted fourth slice is a bump to
version 2 **plus a real migration**, and `store.ts:53`'s migrate must be total over `unknown`
because it runs before any error boundary exists.

`dependency-graph-and-order.md` §2.3 notes the hazard generically and warns that *"two phases doing
that concurrently produce two version-2 migrations"* — then §4.3's merge order does nothing to
prevent it. Phases 4, 5 and 7 could each want persisted per-lens state. **Nobody is assigned the
bump.** That is a concrete, scheduled collision left unowned by the document whose job was
collisions.

### 2.4 Two conflicting auth models, never put side by side

`desktop/src/api/client.ts:6-8` sends **no** `Authorization` header, deliberately, until phase 8,
because the desktop hop's boundary is a loopback bind plus a Host guard plus an Origin allowlist.
`survey-phase-7-deploy.md` §5.4 has the desktop holding a `pnc_` bearer token for `panacea-api`.
Both are correct in isolation. Together they mean the app holds a real credential in phase 7 while
the phase-8 auth decision is still open, and **no document says which process holds it** — the Tauri
renderer, the Rust side, or the local control plane acting as a proxy. If it is the control plane,
the loopback-only threat model at `client.ts:6-8` is now guarding a secret it was not designed to
guard. See also §4.4.

---

## 3. Which claims are asserted rather than verified?

### 3.1 The sibling repos exist — I checked

Both `~/Projects/Panacea` and `~/Projects/Paperdesign` are present and readable. So the phase 7
survey's framing ("sibling repo present and readable") is honest. I did **not** re-open
`services/panacea-api/routes/routes.go`, so every route line number in that survey's §2 tables is
**carried through, not re-verified**. Given the survey's evident care I would spot-check rather than
re-audit, but a plan should not cite those line numbers as though this pass confirmed them.

### 3.2 The AGPL obligation is stated wrongly in two places, and the error is in the risky direction

Verified myself:

- `~/Projects/Paperdesign/package.json:5` — `"license": "AGPL-3.0-only"`.
- `~/Projects/Paperdesign/LICENSE` — GNU AFFERO GPL v3, and **§13 "Remote Network Interaction"** is
  at `LICENSE:540-546`: a modified version must *"prominently offer all users interacting with it
  remotely through a computer network … an opportunity to receive the Corresponding Source"*.
- `~/Projects/Paperdesign/package.json:49` — `"express": "^4.19.2"`. It is a network-served app.

Now the two statements in our own docs:

- `docs/PROGRAM.md:94` — *"AGPL-3.0 must be cleared before any external **distribution**."*
- `dependency-graph-and-order.md:210` — *"a legal gate on **distribution**, not on internal build,
  so it does not block phase 5 starting."*

**Both frame the trigger as distribution. AGPL §13's trigger is network interaction.** `CLAUDE.md`
locks *"we-host SaaS on Panacea's AKS first"*. Under a we-host model there may never be a
"distribution" in the classical sense, and §13 still fires the moment a user's browser interacts
with our modified Paperdesign. The Tauri desktop shell is *also* classic distribution, so both
triggers are in scope, not one.

I am not a lawyer and this is a reading of the licence text, not advice. The point for this pass is
narrower and defensible: **the question the docs tell the owner to ask is the wrong question, and
the wrong one is the easier one.** `dependency-graph-and-order.md` §3.4 amendment 3 asks legal for
"AGPL-3.0 clearance for Paperdesign"; it should ask specifically about §13 under a hosted model.

Two further AGPL items nobody raised at all: the licences of Paperdesign's own dependency tree, and
what `CLA.md` does. I read `CLA.md:1-30` — it grants **the upstream maintainer** sublicensing rights
over contributions. That lets *them* relicense. It does nothing for us as a downstream fork.

### 3.3 "The desktop app stores it in the OS keychain" has no mechanism

`survey-phase-7-deploy.md:310` asserts it. Verified: `desktop/src-tauri/Cargo.toml:22` lists exactly
one plugin, `tauri-plugin-opener`. No `tauri-plugin-stronghold`, no keyring crate, nothing. Secure
credential storage is a new dependency, a new capability entry, and a Windows/macOS behaviour
difference — none of it costed anywhere.

### 3.4 Honestly-labelled unknowns I am re-listing so they are not lost

- **No phase 5 survey exists.** `dependency-graph-and-order.md` §0 says so plainly, which is to its
  credit — then §3.2 declines to move phase 5 *because* the evidence is missing. That reasoning is
  circular: the pass was asked whether the order should change and answered "keep it" on
  one-third of the evidence, for the one phase carrying the largest unretired risk in the envelope.
  "Keep it" and "we could not assess it" are different answers and only one of them was given.
- **The API base URL is unresolved** — `api.dev.panacea.kpmg` vs `dev.panacea.kpmg/api`. One
  unauthenticated `GET /api/v1/version` settles it; nobody made the call.
- **The P3 → P7 ship-path edge is INFERRED**, self-labelled, and gates phase 7b.
- **Nobody ran anything.** No `go test`, no `npm test`, no `tsc`, no `npm run typecheck`, no network
  probe, no render. Every quality claim about the substrate is a reading of a test file's *name*.
- `survey-phase-4-discover.md` §7 admits 4 of 14 tasks in the phase 4 plan were not opened.
- `dependency-graph-and-order.md` §2.1's P3 and P5 columns are self-labelled INFERRED.

### 3.5 One open question I closed

`dependency-graph-and-order.md` §6 Q5 doubts the phase 4 plan's citation of `app.html:964-987`.
Verified: `prototype/app.html:964` is `function viewDiscover(){`, and `:964-987` is the whole
Discover view. **The citation is correct.** Whether `app.html` is the *owner-approved* variant of
seven is a different question and only the owner can answer it.

---

## 4. What did nobody cover?

### 4.1 Offline — zero mentions across all four documents

`grep -ic offline` returns **0** in every file. The phase 4 survey's "works on a laptop with no
credentials configured" is about *model* degradation only. Nothing covers: the control plane down,
the Panacea API unreachable, corporate VPN off, or a laptop on a plane. `desktop/src/api/problems.ts`
gives three kinds — `control-plane-down`, `backend-down`, `server-said` — and **none of them names a
third-party API being unreachable**, which is phase 7's entire read path. Adding a fourth
`ProblemKind` is, by that file's own design, a compile error in every call site. That is a feature,
and it means phase 7 must plan the edit rather than discover it.

### 4.2 Branch deletion — zero mentions

`grep -i` for branch-delete/orphan across the four documents returns **nothing**. A brief lives in
`statepath.Dir("discover")` keyed by branch; the managed block lives in that worktree's `CLAUDE.md`.
When phase 3 deletes a branch or a worktree: is the brief deleted, orphaned or archived? What does a
`ShippedManifest` render for a release whose branch no longer exists? The substrate survey §8 item 4
records that this exact defect **already exists** for `pinned` — nothing prunes pins whose branch was
deleted server-side, and it is unbounded. Phase 4 is about to add a second store with the same
shape and no prune, and phase 7 a third.

### 4.3 Migration — only half covered

Covered: the zustand root-key hazard (§2.3 above), unowned. Not covered at all: Go-side persisted
state. Nothing versions the `discover.Brief` **document**, nothing says what happens to a brief
written by an older schema, and nothing covers migrating anything already in the registry filestore.
`discover.Brief` carrying its own `Questions` is good forward-compat for the question set; it is not
document versioning.

### 4.4 Authentication — split across two models, reconciled by nobody

See §2.4. Add: `POST /api/v1/me/api-keys` is gated behind `RequirePortalGrant()` and rejects a
scoped token, so the app **cannot mint its own token** — the flow is open-browser → copy → paste.
That is honest and it works, but it is an onboarding surface nobody designed, in a lens nobody
prototyped, with a storage mechanism the repo does not have (§3.3).

### 4.5 Files and Merge — no owner in this pass, and a component is already orphaned

`AppShell.tsx:42-57` renders both as `phase={3}`. `PROGRAM.md:86` phase 3's prose covers "diff review
with transcript back-links, merge ordering, conflict resolution", so they are *implicitly* phase 3.
But: `docs/superpowers/plans/` holds phases **1, 2 and 4 only** — there is no phase 3 plan. There was
no phase 3 survey in this pass. And the phase 4 plan builds
`DefinitionOfDone` as *"**mounted by Phase 3** above the merge list"* (`:5597`).

**So phase 4 is specified to build a component whose only mount point lives in an unplanned,
unsurveyed phase.** It will ship tested, rendered nowhere, and stay that way until someone writes
phase 3. That is the same failure the substrate survey found for `contextRef` — built end to end,
wired to nothing — being repeated deliberately one layer up.

### 4.6 Phase 5, entirely

No survey, no prototype, no brief, no risk list, no dependency column that is not inferred. The
`AGPL` gate and the missing event stream are both named, which is the floor, not the ceiling. Not
named anywhere: how a `Paperdesign` express server and a Tauri shell and our control plane share one
window; what the "handoff bundle" (`PROGRAM.md:92`: tokens, component tree, screens) actually
contains; whether Paperdesign's own design language can be made to obey `docs/DESIGN-SYSTEM.md`;
whether multiplayer is in scope at all for a single-user desktop cockpit.

### 4.7 The deliverable shape itself

No scope-and-risk brief exists for any of the three phases, so there is no single document the owner
can read to decide anything. The material is there; it is spread across 111 KB of survey.

---

## 5. Design-system compliance of the three prototypes, checked mechanically

**Cannot be run — the three files do not exist.** No empty state and no failure state can be
assessed, because there is nothing to assess.

I ran the same three greps against the seven prototypes that *do* exist, because phase 4's plan is
specified against one of them:

| file | `gradient` | `transition: all` | lines with a literal hex colour |
|---|---:|---:|---:|
| `app.html` | **3** | 0 | 21 |
| `index.html` | **16** | 0 | 33 |
| `v2.html` | **4** | 0 | 22 |
| `v3.html` | **1** | 0 | 19 |
| `v4.html` | **0** | 0 | 16 |
| `v5.html` | **3** | 0 | 18 |
| `v6.html` | **2** | 0 | 16 |

`transition: all` is clean everywhere — that rule has landed.

**The finding that matters.** `app.html` — the file the phase 4 plan cites as the approved Discover
interaction — carries at `:173` and `:202` a `radial-gradient` dot-grid and at `:179` a
`linear-gradient` resize affordance. Those are **precisely** the two constructs the substrate survey
§7.1 rule 3 records task 24's design-system audit **failing** in task 21's stylesheet
(*"Task 21's `radial-gradient` dot-grid and `linear-gradient` resize affordance both failed"*).

So *"build what the approved prototype shows"* and *"pass the phase 2 design-system audit"* are
already in conflict for the canvas grid, and phase 5 is a canvas. Someone must rule which wins
before phase 5 draws a frame. The substrate survey names the compliant alternatives —
`background-image` from a token, a border, or a repeating box-shadow.

The hex counts are not a violation in a prototype (the rule binds component stylesheets, not
self-contained HTML), but they are the reason a prototype's colours cannot be lifted into the app
verbatim.

---

## 6. What the product owner must decide before any of this becomes a plan

Consolidated across all four documents, deduplicated, ordered by how much each one blocks.

### Decide now — each one blocks written tasks

1. **Snapshot or pointer?** Does Deploy attach a **frozen copy** of the brief as it stood at deploy
   time, or a live pointer to the branch's current brief? (§1.) Blocks: the shape of
   `provenance.Ref`, phase 4 task 8, phase 7's local-link design, and whether `ShippedManifest` can
   be truthful at all. Blocks the most of anything on this list.
2. **Who calls Materialise?** Server-side `Materialiser` in `api.Deps`; or `contextRef` as a
   precondition; or client-enforced plus an honest warning. (§3.5 of the dependency doc; both
   surveys agree it is a blocker.) Decides who owns `control-plane/internal/api/sessions.go`, and
   therefore whether phase 4 can start.
3. **Is a prototype required before these plans, and which of the seven existing files is
   approved?** `CLAUDE.md`'s gate is unopened. This pass produced no prototype, so either the gate
   is waived in writing or the pass is not finished.

### Decide next — external lead time, start the clock now

4. **AGPL-3.0 on Paperdesign, asked correctly** — §13 network-interaction under a we-host SaaS
   model, not distribution alone (§3.2). Ask the legal owner this week; the answer takes calendar
   time and it can kill phase 5's approach outright.
5. **Two cross-repo asks to Panacea:** an additive `metadata map[string]string` on
   `TriggerDeployOptions`, and a public/PKCE Keycloak client in realm `panacea`. Neither blocks
   starting; both have other people's queues attached.
6. **Does the event stream (UI contract §10.5) land before phase 5?** If no, phase 5 ships a second
   transport with a second reconnect ladder and a second failure vocabulary in one window.

### Decide at the gate-zero checkpoint — cheap, but they must be conscious

7. **Phases 3 and 4 concurrently?** The dependency doc's amendment 1 makes a good case and I found
   nothing against it. Contingent on the lens-seam task (0b) landing first.
8. **Who owns `PERSIST_VERSION` 2**, and who owns Files, Merge, and the mount point for
   `DefinitionOfDone`? (§2.3, §4.5.)
9. **One `GET /api/v1/version`** to settle the Panacea base URL. Sixty seconds, and it removes a
   documented contradiction.

---

## 7. What I did not verify

- I opened none of `~/Projects/Panacea`'s Go source. Every `routes.go`, `models.go`, `projects.go`
  and `middleware/` line number in `survey-phase-7-deploy.md` is carried through unchecked.
- I ran no build, no test, no typecheck, no network call, and rendered nothing.
- I read the phase 4 plan only where this critique cites it — the `provenance` package, the
  `Ref` type, the three surfaces, `useProvenance`, and the endpoint table. It is 223 KB; I read
  perhaps 3% of it.
- The AGPL reading in §3.2 is mine, from the licence text at `LICENSE:540-546`. It is not legal
  advice and should not be relied on as any. The recommendation is only that the question put to
  legal be the network-interaction one.
- I did not diff `types.ts` against the contract; I verified the gate command exists at
  `task-15.md:280` and read both files' relevant lines.
- Phase 2 is live in this tree and another agent is editing it. `AppShell.tsx:62` was `:59` and
  `:61` in the two surveys written earlier today. Symbols are stable; line numbers are not.
