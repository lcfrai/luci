# Provenance contract — DRAFT for product-owner review

**Status: DRAFT. Not approved. Not an implementation plan.** Written 2026-09-23 as part of the
phases 4/5/7 planning pass. `CLAUDE.md`'s gate stands: the owner reviews prototypes before plans,
and plans before code. This document is an input to that gate, not a way round it.

**What it is for.** `CLAUDE.md`: *"The connective tissue is provenance: the spec written in Define
is visibly referenced in Design, carried into Develop, and attached to the release in Deploy. Same
artifact, one thread, shown throughout. Any feature that breaks that thread is wrong regardless of
how good it is in isolation."* Phase 2's two contracts are Develop-only. No contract names the
artifact, owns its fields, or says what breaks the thread. This one does.

---

## 0. Inputs, and what this draft does not verify

### 0.1 Read for this draft

| Input | Used for |
|---|---|
| `survey-phase-2-substrate.md` §2, §3, §5, §6, §8 | what the carrier (`Session.contextRef`), store, client and poller actually are |
| `survey-phase-4-discover.md` §1, §2, §4.1, §4.2, §5 | the brief, the `Ref`, the three surfaces, Materialise |
| `survey-phase-5-design.md` §7, §8 | **present** (written earlier today). Northstar's digest-bound approval, the rival spine, the one-carrier constraint |
| `survey-phase-7-deploy.md` §5.4, §6 | the metadata-less deploy API, `pnc_` tokens, the local-link proposal |
| `dependency-graph-and-order.md` §3.4, §3.5, §4 | slice zero, decision 0c, lanes A/B/C, 7a/7b |
| `completeness-critique.md` §1, §2 | the six items §6 of this draft resolves |

Read directly from the tree, to check the surveys' claims rather than repeat them:

- `desktop/src/store/types.ts` — the `Session` block. `contextRef: string` with the doc comment
  *"Opaque provenance handle from Phase 4. Stored, carried, never interpreted."*
- `desktop/src/store/store.ts:14-58` — `CockpitState`, `PERSIST_VERSION = 1`, `partialize`, `migrate`.
- `desktop/src/api/client.ts:1-8` — no `Authorization` header before phase 8, and why.
- `desktop/src/api/poller.ts:1-3` — *"Phase 2 polls, from exactly one place, with exactly one
  request per tick."*
- `docs/superpowers/plans/2026-09-18-phase-4-discover.md`, **only** these regions: `discover.Brief`
  (`:969-980`), every `Revision++` (`:1117`, `:1142`, `:1150`, `:1168`), `discover.Document`
  (`:1724-1730`), `discoverstore.Store` (`:3019`, `:3177`, `FileStore.Save` at `:3226`),
  `provenance.Ref` (`:3515-3533`), the route table (`:3612-3622`), `useProvenance` (`:5716`),
  `ShippedManifest` (`:5870`).

### 0.2 One fact from that reading that the surveys did not state

**The phase 4 plan keeps no revision history.** `discoverstore.Store` is
`Save / Get / ForProject / ForBranch` — one document per brief id, overwritten on every `Save`.
`Revision` is bumped by `Record`, `Edit`, `Skip`, `Reopen` and `AddFollowUp`, so a five-question
brief reaches revision 5 before it is ever materialised, and every later edit destroys the text of
the revision before it. **Revision 3's text does not exist in the store once revision 4 is
saved.** (A worktree's `CLAUDE.md` may still hold a rendering of it, but that is an output, not a
record — §3.3.) That is why critique §1.1 cannot be fixed at the Deploy end alone: there is nothing
to point back to. §1 below fixes it at the origin.

### 0.3 What this draft does not verify

- **Nothing was run.** No build, no test, no network call. Every claim about code is a reading.
- **Every type below is a proposed shape**, not a built one. Field names are final only when the
  phase 4 plan amendment writes them; this contract fixes identity, ownership and semantics.
- **Phase 3 has no plan and no survey.** Every row below that names phase 3 as a writer is an
  obligation this contract places on a plan that does not exist yet.
- **Whether a worktree's `CLAUDE.md` is committed to git** is not settled anywhere I read. §3.4
  offers a check that depends on it; the check is marked conditional.
- **Panacea's route line numbers** come from `survey-phase-7-deploy.md`, carried through, not
  re-read.

---

## 1. The artifact

### 1.1 Name

**The artifact is the Brief.** Discover writes it; every later phase refers to it. "Spec" in
`CLAUDE.md` and "brief" in the phase 4 plan are the same object. This contract uses **Brief**
everywhere, because it is the word already in the plan, the managed block
(`panacea:discover:begin brief=…`) and the UI.

A Brief has two forms, and the whole of critique §1 comes from treating them as one:

| Form | What it is | Mutable? | Who uses it |
|---|---|---|---|
| **Brief head** | the living document at its latest revision — what Discover edits | yes, by Discover only | Discover, and surfaces about *the work in front of you* |
| **Brief revision** | the Brief exactly as it stood at revision *n* | **never** | everything that makes a claim about the past: a session's inheritance, a design approval, a release |

### 1.2 Identity — the brief key

A brief revision is named by a **brief key**:

```
brief:<briefId>@<revision>          e.g.  brief:b-42@3
```

- `briefId` is the id Discover minted (`discover.Brief.ID`). It never changes for the life of
  the Brief, including across branch rename, worktree deletion and branch deletion.
- `revision` is `discover.Brief.Revision` at the moment the revision was saved. Monotonic per
  Brief, never reused.
- Each revision also carries a **digest**: SHA-256 over a canonical encoding of its questions,
  answers and status. The key names the revision; the digest proves the stored text is the text
  that was named. A link that records `key + digest` can detect a rewritten revision file. This is
  the same discipline as Northstar's digest-bound approval (`survey-phase-5-design.md` §7.1) and it
  makes the two binding mechanisms speak one language.

**The key is the identity that survives all four phases.** Nothing downstream holds a branch id, a
brief id alone, or a copy of the text as its reference to the Brief. It holds a key.

The key grammar is **owned by the `provenance` package** (phase 4). A `kind:` prefix is part of
the grammar so that ledger links can name other subjects in the same form. `design:` is
**reserved** for phase 5 and appears only in ledger links, never in `contextRef` (§3.2). No other
kind is defined by this draft.

### 1.3 Shape of a brief revision

```
BriefRevision {
  key         "brief:b-42@3"
  briefId     "b-42"
  revision    3
  digest      "sha256:…"
  projectId   the project the brief was written for
  branchId    the branch it was written for ("" for a project-scoped brief)
  status      open | complete | skipped      — as discover.Status
  questions   []Question                     — as stored on the brief, so the text is self-describing
  answers     []Answer
  savedAt     RFC 3339
}
```

It is `discover.Brief` frozen, plus `key` and `digest`. Nothing on it is evaluated: **criterion
state is not part of a revision** (§2.3).

### 1.4 The view every surface renders — `provenance.Ref`, amended

`provenance.Ref` stays the one type every surface renders (phase 4's rule, kept). It changes from
*"the branch's brief"* to *"one brief revision, resolved"*:

| Field | Status | Meaning |
|---|---|---|
| `key` | **new** | the brief key this Ref resolves. Empty only when `present` is false |
| `briefId`, `revision` | kept | now always equal to the parts of `key` |
| `digest` | **new** | the revision's digest |
| `headRevision` | **new** | the Brief's current revision when this Ref was served. `revision < headRevision` means *superseded* |
| `projectId`, `branchId`, `headline`, `audience`, `constraints`, `outOfScope` | kept | from the revision's answers, as `provenance.Build` does today |
| `criteria` | kept | text from the revision; `state` is evaluated against a **subject**, see §2.3 |
| `contextPath` | kept, **head `Ref`s only** | the `CLAUDE.md` the head was last materialised into. Empty on a `Ref` resolved by key, because where a past revision was once written is not a property of that revision, and it changes |
| `present` | kept | false when there is no Brief. Never 404 — kept |
| `resolution` | **new** | `resolved` · `missing`. `missing` means a key was supplied and no revision answers it, which is a different fact from "no brief". **Superseded is not a resolution** — it depends on the head, which moves, so the desktop derives it (§4.1, §6.4) and a cached `Ref` never goes stale |

### 1.5 The answer to critique §1.1, in one line

**A release is attached to the Brief as it was when the release was shipped — a pinned brief
revision — never to the Brief as it reads now.** The pinned revision is rendered with its
`headRevision` alongside, so the Deploy lens can say *"shipped against revision 3; the brief has
since moved to revision 6"*. Which revision is pinned, and by whom, is §3.4.

## 2. Where it lives, and who owns each write

### 2.1 Two stores, one of them new

| Store | Location | Holds | Package owner |
|---|---|---|---|
| **Brief store** | `statepath.Dir("discover")` — as the phase 4 plan has it | the Brief head (one document per brief, overwritten), **plus an append-only revision log**: one immutable document per `(briefId, revision)` | phase 4, `discoverstore` |
| **Provenance ledger** | `statepath.Dir("provenance")` — **new** | append-only **link records**: a link says *"this subject was bound to this brief key"*. One record per event, never edited, never deleted | phase 4, `provenance` — slice zero |

The revision log is the one real change to the phase 4 plan's storage. `FileStore.Save` writes the
revision document first and the head second, both through the atomic-write helper (decision 0d
decides whether that is `writeAtomic` lifted into `statepath` or a duplicate). A crash between the
two leaves a revision with no head pointing at it, which is harmless; the reverse order would leave
a head whose revision text does not exist, which is the defect this contract exists to prevent.

The ledger is a separate store, not more fields on the brief, because its records are **written by
three different phases** and a brief document written by one of them is exactly the shared-file
contention `dependency-graph-and-order.md` §2 counts. Phase 4 owns the ledger's schema, storage and
read API. Each link *kind* has exactly one writer (§2.2) and writes through one function per kind.
No phase writes another phase's link kind.

### 2.2 Ownership — one owner per field

| Field or record | Lives in | **Sole writer** | Written when | Read by |
|---|---|---|---|---|
| Brief head (`discover.Brief`) | Brief store | phase 4 — Discover API handlers | each answer, edit, skip, reopen, follow-up | Discover lens |
| Brief revision (§1.3) | Brief store, revision log | phase 4 — inside the same `Save` that bumps `Revision` | every revision bump | the resolver, only |
| Brief key grammar + resolver | `provenance` package | phase 4 | — | every surface, via §3.1's endpoints |
| Managed block in `CLAUDE.md` | the worktree | phase 4 — `contextwrite` (Materialise) | before a harness process starts (§2.4) | the agent, at process start |
| `Session.contextRef` | registry session record — **exists today** | **the server path that materialised**, never the client (§2.4, §6.3) | each process start: create, restart, resume, fork | `ContextChip` via the resolver |
| **Design link** — `(canvasId, designRevision, reviewDigest) → brief key` | provenance ledger | phase 5 | when a human approves the `VISION` gate | Design chrome, Deploy |
| **Merge link** — `(merge commit) → brief key reviewed against` | **phase 3 decides**: ledger or a git trailer | phase 3 | at merge | Merge, Deploy |
| **Release link** — `(projectName, pipelineRun) → brief key + digest (+ design key)` | provenance ledger | phase 7 — the control-plane Ship handler, **in the same operation as `POST …/deploy`** (§3.4, §6.2) | on the 202 from Panacea | Deploy lens |
| Release link — `deploymentId` join | provenance ledger, a second record referencing the first | phase 7 — the deploy poller | first time it observes the `Deployment` for that `pipelineRun` | Deploy lens |
| Criterion **state** | computed, never stored on a revision (§2.3) | phase 3's evaluator; phase 4 returns `open` for every criterion until then | on read | every surface |
| PipelineRun annotation mirror | Panacea, only if the metadata ask (amendment 3.1) is granted | phase 7 Ship handler, write-once, from the same value as the release link | same moment as the release link | **recovery only** — never rendered while the ledger has the record |
| Desktop provenance cache | zustand, `provenanceSlice`, **not persisted** (§6.5) | phase 4 | §6.4's loader | the four surfaces |

### 2.3 Criterion state is evaluated against a subject, not stored

A criterion's text belongs to a revision and is immutable. Whether it is `settled`, `open` or
`contested` is a judgement about **some code**, and the code differs by surface:

| Surface | Subject the state is evaluated against |
|---|---|
| `ContextChip` (Develop) | the session's worktree at its current head |
| `DefinitionOfDone` (Merge) | the branch head being merged |
| `ShippedManifest` (Deploy) | **the commit that shipped** — `Deployment.Commit` |

Evaluating a release's criteria against the branch head is the same defect as critique §1.1 one
field lower: a criterion broken by a commit made after the release would render as broken on the
release. Phase 4 is unaffected — every state is `open` by design. **The obligation falls on phase 3's
evaluator**: it takes `(brief key, commit)` and returns states, it never takes a branch id. Whether
a release additionally freezes the evaluation it had at Ship time is a phase 3/7 question, deferred
to whichever plan builds the evaluator.

**Evidence is keyed by brief key and criterion id together, never by criterion id alone.** The
plan's `criterionID(briefID, index)` is positional — *"stable across revisions for the same
position"* — so inserting a criterion at revision 5 shifts every later id. Evidence recorded
against `b-42:done:2` at revision 3 would otherwise attach itself to a different sentence at
revision 5. The key disambiguates; the evaluator must store it.

### 2.4 What `contextRef` now means

`Session.contextRef` holds **the brief key of the revision that was materialised into the
session's worktree before its harness process started**. Nothing else.

- It is a fact about inheritance, so **only the path that did the materialising may write it**. A
  client-supplied value is at best a guess about what the server wrote.
- A **restart is a new process** and re-reads `CLAUDE.md`. Phase 2's `handleSessionRestart`
  (`api/sessions.go:729`, per the substrate survey) keeps `contextRef` unchanged across restart,
  and the desktop carries it from the source session on resume and fork (`client.ts:359`, `:381`,
  also per the substrate survey).
  **Both are correct for a carrier and wrong for this contract** once a brief can change between
  starts: the value must be restamped on every start. **Inferred, not verified:** that Claude Code
  re-reads `CLAUDE.md` on `--resume`. If it does not, resume keeps the old key and restart restamps.
- `types.ts` does not change. `contextRef` stays `string`, and phase 2's code still stores, carries
  and never interprets it. The grammar is interpreted by the `provenance` package and nowhere else.
- An empty `contextRef` keeps its meaning: no brief was materialised. That is a state to show
  (§4.1), not a reason to hide the chip.

## 3. How each phase references it without copying it

**The rule:** every phase after Discover holds a **brief key**, never the brief's text and never a
branch id standing in for the brief. Text is fetched by key from one resolver. A key names an
immutable revision, so holding it cannot drift. This is the third option critique §1.2 did not
list: not a pointer to a mutable head (drifts), not a frozen copy (a second source of truth), but
**a pointer to an immutable revision**.

### 3.1 The read API — two endpoints, one type

| Endpoint | Returns | Cacheable | Serves |
|---|---|---|---|
| `GET /api/v1/discover/branches/{branchID}/provenance` | the **head** `Ref` for the branch — kept from the phase 4 plan, still never 404 | no — the head moves | Discover, `DefinitionOfDone`, Design chrome *before* approval |
| `GET /api/v1/discover/refs?key=…&key=…` — **new** | one `Ref` per key, plus `heads: { briefId: revision }` for every brief named | **forever**, apart from `headRevision` | `ContextChip`, `ShippedManifest`, Design chrome *after* approval, anything else holding a key |

The batch form is deliberate: a twelve-session canvas resolves twelve keys in one request (§6.4).
An unknown key returns a `Ref` with `resolution: "missing"` in its slot; it does not fail the
batch. `headRevision` is the only time-varying field on a resolved `Ref`, so the desktop caches it
per brief, not per key (§6.4).

Ledger reads are **not** a third provenance endpoint. Each phase serves its own subjects with the
key attached — phase 7's releases list carries each release's pinned key; phase 5's canvas binding
carries its design link's key — and the surface resolves the key through the endpoint above. That
keeps one resolver and one `Ref` type, which is phase 4's original rule, kept.

### 3.2 Per phase — what it holds, what it never holds

| Phase | Holds | Resolves through | Never holds |
|---|---|---|---|
| **4 Discover** | the Brief head; writes every revision | — it is the source | — |
| **2/4 Develop** | `Session.contextRef` = a brief key (§2.4) | `/discover/refs` | the headline or criteria text in session state; a branch id used as "the session's brief" |
| **5 Design** | a **design link**: `design:<canvasId>@<designRevision>` + `reviewDigest` → the brief key the approval was reviewed against | `/discover/refs` | a brief copy in the canvas database that anything renders from |
| **3 Merge** | a **merge link**: merge commit → brief key reviewed against (phase 3 chooses the home) | `/discover/refs` | criterion state evaluated against anything but the merged commit |
| **7 Deploy** | a **release link**: `(projectName, pipelineRun)` → brief key + digest, and the design key if one exists | `/discover/refs` | the brief head; any copy of the brief's text |

**`design:` keys name approved designs, and they appear only in ledger links.** `contextRef` stays
a brief key and nothing else, so its meaning is one sentence long. A session started *from* an
approved design gets its brief key in `contextRef` like any other session, and phase 5 writes a
separate `session → design key` link to the ledger. That settles `survey-phase-5-design.md` §7.4
("one carrier holds one ref") without a second carrier field and without a `types.ts` change.
The shape of that link is reserved here and finalised in phase 5's plan.

### 3.3 Materialisations are renders, not copies

Two consumers cannot call our API: an agent reading `CLAUDE.md` at process start, and a Northstar
canvas whose agents read `GuidelineDoc`s. Writing the brief's text into those places is allowed,
and it is **not** a copy for the purposes of this contract, on four conditions:

1. It carries its brief key in a marker — `<!-- panacea:discover:begin brief=b-42 rev=3 -->`
   already does; the proposed canvas guideline mirror (`survey-phase-5-design.md` §7.2) must too.
2. It is overwritten **from** the Brief store, never edited in place and synced back.
3. **Nothing reads it back as a source.** No surface renders a headline parsed out of `CLAUDE.md`
   or out of a guideline. The only permitted read-back is the drift check: *"does the marker's key
   match what this place should carry?"*
4. A drift check exists for each place that has an editor other than us. `CLAUDE.md` has one in
   `claudemd.Merge`; the canvas guideline, which `set_guidelines` lets any canvas user edit
   (`survey-phase-5-design.md` §7.2), needs one.

The guideline mirror pays for itself in Design. Guidelines are inside Northstar's review digest, so a `VISION`
approval over a canvas carrying the brief mirror binds, by digest, **the exact brief text the
design was reviewed against**. The design link then records the brief key explicitly, so nobody
has to parse the digest input to find it.

### 3.4 The release pin — which revision, set by whom

**Rule: a release is pinned exactly once, at the moment it is shipped, to an immutable brief key.
The pin is never re-pointed.**

| How the release happened | What is pinned | Written by |
|---|---|---|
| **Ship from the cockpit** | the head key, **at the moment of Ship**, of the brief the `provenance` package resolves for the branch being shipped — literally the spec as it stood when the code shipped. Phase 7 asks for it; it never re-derives branch → brief itself | the control-plane Ship handler, in the same operation as `POST …/deploy` |
| **Rollback, promote or redeploy from the cockpit** | the key **of the deployment being re-run** — it ships that deployment's commit and image, so it inherits that deployment's pin, not today's head | the same handler, copying a key from the source release link |
| **Deployed outside the cockpit** — auto-deploy on push, the Panacea portal, another client | **nothing**. The row says *"no brief is recorded for this release"* | — |

Three constraints on this table:

- **Never pin by observation.** When the deploy poller first sees a deployment it did not ship,
  it must not pin the head it can see now. The head at observation can post-date the ship by days;
  pinning it recreates critique §1.1 through the back door.
- **The Ship-time pin and the merge link are two facts, not a choice.** If phase 3 records that a
  merge was reviewed against revision 3 and the brief is at revision 5 when Ship is pressed, the
  release is pinned to 5 and the manifest also shows the merge link's 3. Disagreement is shown,
  not resolved.
- **Conditional cross-check, only if phase 3 commits `CLAUDE.md`.** The managed block in the
  shipped commit's tree names the revision the code was written with. Read through phase 3's git
  API, it gives a *derived* reading for releases shipped outside the cockpit — labelled *"read from
  the shipped code's CLAUDE.md"*, never presented as a pin — and a cross-check for releases shipped
  from it. Unverified whether `CLAUDE.md` is committed; this is open question 4.

**Loss.** The ledger lives in our state directory. If that directory is lost, a pinned release
becomes indistinguishable from an unpinned one — unless Panacea grants the metadata ask, in which
case the Ship handler writes the same key and digest onto the PipelineRun as a write-once mirror
and the ledger can be rebuilt from it. The wording *"no brief is recorded for this release"* is
true in both cases, which is why it is the required wording.

## 4. "Visibly referenced" — the UI obligation per phase

**The test, for the owner's review of any prototype:** from any session, approved design or
release, a person can answer *"which brief, which revision, and is it still current?"* **without
opening anything**, and *"what exactly did it say?"* in **one click**. A surface that fails either
half has not visibly referenced the brief.

### 4.1 One state vocabulary, every surface

Every provenance element distinguishes these six states. They are different facts and a surface
that merges two of them is lying about one.

| State | Condition | What the element must say (pattern, not final copy) | Visual rule |
|---|---|---|---|
| **current** | key resolved, `revision == headRevision` | the headline, and `rev 3` | neutral ink; `rev` in `var(--numeric)` |
| **superseded** | key resolved, `revision < headRevision` | `rev 3 · brief now at rev 6` | `--drift` **plus** a static shape or text marker — colour alone fails rule 6 of the six CSS rules |
| **none** | no key: empty `contextRef`, no Brief, unbound canvas, unpinned release | per surface: *"no brief"*, *"not bound to a brief"*, *"no brief is recorded for this release"* | shown, quietly. **Never hidden** — see prohibition P9 |
| **missing** | a key was supplied and no revision answers it | the key, verbatim, in mono, and *"not found"* | `--fail`. This is data loss in our own store, not a user state |
| **resolving** | key known, `Ref` not yet in the cache | a fixed-width placeholder | **no spinner**. The element does not change width when it resolves |
| **unavailable** | the resolver call failed | the call site's own sentence, through `classify()` — `control-plane-down` / `backend-down` / `server-said` | as the phase 2 problem pattern: exhaustive switch, declared return type, no `default:` |

**Every element in the `current`, `superseded` or `missing` state opens Discover at that exact
revision, read-only.** That is the one-click half of the test, and it is a new obligation on
phase 4: Discover must render a past revision (§8).

### 4.2 Design — phase 5

- **Rendered by our chrome, outside the canvas iframe.** Never inside a frame: frame scripts are
  untrusted, and `survey-phase-5-design.md` §3.1 found a frame can already persist edits to itself
  as the viewing human. A provenance claim drawn by code a frame controls is forgeable.
- **Before approval:** the canvas header carries the branch's head `Ref` as a chip.
  **After approval:** it carries the design link's pinned key, and says
  *"approved against brief rev 3 by <approver>"*.
- **When the brief moves on after approval**, the chrome says so (`superseded`). It does not
  silently re-point the approval to the new revision.
- An unbound canvas says *"not bound to a brief"* — the `none` state, visible.
- **An interaction phase 5 must design deliberately:** if the brief mirror is a `GuidelineDoc`,
  re-writing it after a brief edit bumps `design_revision` and **invalidates the `VISION`
  approval**, because guidelines are inside the digest. That may be correct (the spec changed, so
  re-review) or too aggressive (a typo fix voids an approval). Open question 3.

### 4.3 Develop — phases 2 and 4

- **Every session pane header carries a `ContextChip`**, resolved from that session's
  `contextRef` — not from the branch. Two sessions on one branch that inherited different revisions
  show different chips. That is the point of critique §1.3.
- Collapsed: the headline, truncated, and `rev n`. Expanded: audience, constraints, out of scope,
  criteria, the key in mono, and *"open in Discover at this revision"*.
- **`superseded` matters most here**: it tells the user an agent is working from instructions that
  have since changed. The chip may offer *restart to inherit rev 6*; it must at least say it.
- Twelve panes, twelve chips, **one request** (§6.4), no per-chip loading indicator.

### 4.4 Merge — phase 3

- `DefinitionOfDone` above the merge list renders the **head** `Ref` — the work in front of you
  against the spec as it reads now. That reading is correct here, and critique §1.1 agrees.
- Criterion states are evaluated against the branch head being merged (§2.3), each non-`open`
  state with its `because`.
- Merging writes the merge link (§2.2). Its home is phase 3's call.

### 4.5 Deploy — phase 7

- **Each release row names its pinned revision** — `brief rev 3` — or says *"no brief is recorded
  for this release"*. Rows are never painted with the head.
- **`ShippedManifest` renders the pinned revision's text, never the head's.** Its first line is
  the pin — *"v0.4.2 shipped against brief b-42, revision 3"* — and, when superseded,
  *"the brief has since moved to revision 6"* with a link to each.
- If a merge link or a design link exists for the release, the manifest shows it beside the pin,
  and shows disagreement as disagreement (§3.4).
- Criterion states are evaluated against `Deployment.Commit`. Until phase 3's evaluator exists
  every state is `open`, and the manifest says *"no evidence recorded yet"* — it must not imply the
  criteria were unmet.
- The three absent states stay apart: *no brief recorded* (`none`), *revision not found*
  (`missing`), *could not reach the control plane* (`unavailable`).

## 5. What breaks the thread — prohibitions, with reasons

Each of these is a feature that could look finished and be wrong. `CLAUDE.md`: *"Any feature that
breaks that thread is wrong regardless of how good it is in isolation."*

| # | Prohibited | Why |
|---|---|---|
| **P1** | Rendering a release — its manifest, its row, or its criteria — from the brief **head** | Critique §1.1. The Deploy lens would claim a past release satisfies criteria written after it shipped: confidently wrong, and worse than showing nothing |
| **P2** | Pinning a release **at observation** — writing the head's key when the poller first sees a deployment it did not ship | Same defect by a second route. The head at observation can post-date the ship by days |
| **P3** | **Re-pointing** a pin or link, or **editing or deleting** a brief revision | The key is the identity. A revision that can change is not a revision, and every link that named it is silently rewritten |
| **P4** | **Copying brief text** into any store something renders from — the ledger, the canvas database, `localStorage`, a PipelineRun annotation, component state | A copy drifts, and the drift is invisible until two surfaces disagree in front of a client. Materialisations under §3.3's four conditions are the only exception |
| **P5** | **Parsing a brief key** outside the `provenance` package — in the desktop, in phase 2's Go, in phase 7's handler | One grammar, one owner. `types.ts` promises `contextRef` is *never interpreted*; the desktop keeps that promise by passing keys to the resolver, never splitting them |
| **P6** | A **client-supplied `contextRef`** accepted as fact, or a `contextRef` that differs from what was materialised before the process started | Critique §1.3. The field would then record what a client *intended*, not what an agent *inherited*, and the chip would be decorative |
| **P7** | A **release link written by a second call** — ship first, record later — or by a process other than the one that called `POST …/deploy` | A crash between the two calls loses the link; a separate "record this link" endpoint lets any caller assert any link. Critique §2.4 is resolved on this basis (§6.6) |
| **P8** | **Fetching provenance per component** — a `useEffect` fetch in a chip, a row or a pane | Critique §1.4. It breaks `poller.ts`'s one-place rule and `CLAUDE.md`'s one-heartbeat rule, and twelve panes become twelve requests |
| **P9** | **Hiding** a provenance element because there is no brief | Absence is information. A session with no brief looks identical to one whose chip failed to render, and a missing link becomes undetectable |
| **P10** | **Running Northstar's own `DISCOVERING`, `DEFINING`, `BUILDING`, `VERIFYING`, `DEPLOY_PENDING` or `DEPLOYED` phases** in a Panacea build | `survey-phase-5-design.md` §7.1: a second spine in the canvas's Postgres that will disagree with ours about where the project is. Keep the `VISION` gate and its digest; disable the rest. **Binding only if the owner agrees — A6** |
| **P11** | **Rendering provenance inside a canvas frame** | Frame scripts are untrusted and can already write as the viewer (`survey-phase-5-design.md` §3.1). A claim a frame can draw is a claim a frame can forge |
| **P12** | **Evaluating a release's criteria against the branch head**, or showing any non-`open` state without its `because` | §2.3, and phase 4's own rule from `AGENTIC-UX.md` principle 7 — kept. Also kept: no denominator, no percentage, no "2 of 4 met" |
| **P13** | **Deleting a brief revision when a branch or worktree is deleted** | Critique §4.2. A release outlives its branch; its manifest must still resolve. Branch deletion may archive the head; revisions stay. Retention is built by phase 4 (§8); the erasure policy is open question 5 |

## 6. Resolutions of the completeness critique

Each subsection gives the finding in one line, the ruling, what changes, and who owns the change.
**Resolved** means this contract settles it. **Deferred** means it names why and to whom.

### 6.1 Critique §1.1 — the release is attached to the brief's present tense

**Finding.** `provenance.Ref` has no immutable identity and one branch-keyed endpoint, so
`ShippedManifest` shows revision 6's text beside a release cut at revision 3.

**Ruling — resolved.** The Brief gets an append-only revision log (§2.1) and an immutable key
(§1.2). `Ref` names the revision it resolves (`key`, `digest`, `headRevision`, `resolution` —
§1.4). A release is pinned to a key at Ship (§3.4) and its manifest resolves the pinned key, never
the head (P1). The branch-keyed endpoint stays, and serves only the surfaces for which the head is
the right reading: Discover, `DefinitionOfDone`, and Design before approval.

**What changes.** Phase 4 plan, Task 7 (`discoverstore`: revision log, `Revision(key)` read) and
Task 8 (`provenance`: key grammar, `Ref` fields, digest), and the `ShippedManifest` test at plan
`:5681` — it must assert the pinned revision and the superseded line, not the head.
**Owner:** phase 4, inside slice zero, because amendment 2 already makes slice zero the thing 5
and 7 build against.

**Phase 4's own rule survives, sharpened.** *"If a mounting surface needs a field, it goes into
`provenance.Ref`, never computed locally"* stays. It was preventing Deploy from fixing §1.1 locally
only because `Ref` could not say which revision it was. It now can.

### 6.2 Critique §1.2 — phase 7's fix cannot be built against phase 4's plan

**Finding.** Phase 7 proposes storing `(project, pipelineRun) → provenance.Ref`. With no
fetch-by-revision, that is either a drifting pointer or a frozen copy.

**Ruling — resolved.** Neither. The release link stores **a brief key and its digest** — a pointer
to an immutable revision. It cannot drift, because the revision cannot change (P3). It is not a
second source of truth, because it holds no text (P4). `ShippedManifest` keeps taking the one `Ref`
type — **no snapshot type is introduced** — resolved through `GET /api/v1/discover/refs` (§3.1).
The digest lets the Deploy lens detect a revision file rewritten under it and show `missing`
rather than render altered text.

**Two refinements phase 7's survey did not have.** Rollback, promote and redeploy inherit the pin
of the deployment they re-run, not today's head (§3.4). Deployments made outside the cockpit are
not pinned at all, and never pinned by observation (P2).

**What changes.** Phase 7's release-link design is fixed by §2.2 and §3.4, and it depends on the
by-key endpoint existing — so the endpoint is in phase 4 slice zero, and phase 7a cannot render a
manifest before slice zero merges. That is the edge `dependency-graph-and-order.md` §4.3 already
draws. **Owner:** phase 4 for the endpoint; phase 7 for the ledger writes.

### 6.3 Critique §1.3 — phase 2's `contextRef` carrier is bypassed

**Finding.** `useProvenance` is keyed by branch, so the carrier phase 2 threaded through five layers
is read by nothing, and a session that inherited revision 3 shows the branch's current chip.

**Ruling — resolved, and it narrows decision 0c.**

1. **`ContextChip` resolves the session's `contextRef`**, through the by-key endpoint, not the
   branch (§4.3). Sessions that inherited different revisions show different chips.
2. **`contextRef` means "the revision materialised before this process started"** (§2.4), and only
   the server path that did the materialising writes it (P6). It is restamped on every process
   start — create, restart, resume, fork.
3. That makes decision 0c's option 3 (client-enforced Materialise) **unable to satisfy this
   contract**: a client cannot know what the server's worktree held when the PTY started. Options 1
   and 2 both can. `dependency-graph-and-order.md` §3.5 already recommends option 1. Under
   option 1, the `Materialiser` interface declared in `api` returns the key it materialised, and
   `handleSessionCreate` and `handleSessionRestart` stamp `ContextRef` from that return value.
   **The choice between 1 and 2 stays the orchestrator's**; this contract only removes 3.

**What changes.** `control-plane/internal/api/sessions.go` — create and restart stamp the value;
the create request's `contextRef` is ignored or rejected rather than trusted. On the desktop,
`toCreateRequest` (`store/defaults.ts:110`, `:127`) and resume/fork (`api/client.ts:359`, `:381`;
both line references from substrate §6.1, not re-read) stop sending a value the server will
overwrite; leaving them sending it is harmless but misleading.
**`desktop/src/store/types.ts` does not change** — `Session.contextRef: string` and its comment
stay true. Go contract and UI contract §4 need a one-paragraph amendment each.
**Owner:** whichever phase decision 0c puts in charge of `sessions.go` — phase 4 under both
options 1 and 2.

### 6.4 Critique §1.4 — `useProvenance` breaks the one-poller rule

**Finding.** `useProvenance` is a per-component `useEffect` fetch. Twelve panes, twelve requests,
against `poller.ts:1-3` and `CLAUDE.md`'s one-heartbeat rule.

**Ruling — resolved.** `useProvenance` is deleted from the phase 4 plan. In its place:

- **A `provenanceSlice`** in the zustand store, **not persisted** (§6.5), holding three maps:
  `refsByKey` (immutable once filled — a resolved key is never fetched again), `headRevisionByBrief`
  (the one moving number), and `headRefByBranch` (for the head-reading surfaces).
- **One loader, called from one place — the poller's tick**, after `replaceSessions`. It collects
  the `contextRef`s of sessions in the store that are absent from `refsByKey` and, **only if there
  are any**, issues one `GET /api/v1/discover/refs` for all of them. On a steady tick that is zero
  extra requests; it fires when a session is created, resumed or forked.
- **Head refs are not polled.** They are fetched when the selected branch changes, through the same
  loader, and written through from Discover's own mutation responses. Discover's API is the only
  writer of a brief head and this app is its only caller, so its responses are the only way a head
  moves. **If any other writer appears** — an agent editing a brief over MCP, a second window — head
  refresh joins the poller tick and this ruling is reopened.
- **Selectors, per substrate §3.2's rule:** `selectRefForSession`, `selectHeadRefForBranch` and
  `selectProvenanceState` go into `store/selectors.ts`, each with a doc comment naming its surface,
  a frozen `NO_REF` constant for the absent case, and `useShallow` where a fresh object is built.
  The `superseded` test — `revision < headRevisionByBrief[briefId]` — is a selector, not inline
  component logic.
- The four surfaces stay **presentational** and take a `Ref` prop, as the plan specifies; their
  mount points select. That keeps phase 4's component tests unchanged in shape.

**What changes.** `poller.ts`'s header comment moves from *"exactly one request per tick"* to *"one
`listSessions` per tick, plus one batch provenance resolve only on a tick where new keys appeared"*
— a declared amendment to UI contract §4.7. `store.ts`'s `CockpitState` gains the slice — an
amendment to UI contract §3.1, **not** to §2, so the `types.ts` byte gate is untouched.
**Owner:** phase 4.

### 6.5 Critique §2.3 — the persist-version bump nobody owns

**Finding.** A persisted fourth slice means `PERSIST_VERSION` 2 and a real migration, and phases
4, 5 and 7 could each write their own version 2.

**Ruling — resolved for provenance, deferred for everything else.**

**Provenance persists nothing, so it needs no bump.** `store.ts` says why in its own words: what is
not persisted *"is re-fetched, because the control plane is the only truth"*. The provenance slice
is a cache of control-plane truth. Persisting it would put a copy of brief text in `localStorage`,
which is P4. Phase 4's editing state for Discover (`BriefView` by branch, `survey-phase-4-discover.md`
§4.2) is also server state and also needs no persistence. So **phase 4, as this contract shapes
it, adds root keys and zero persisted keys**, and `partialize` does not change.

**Deferred, to the orchestrator** — because allocating a shared version number across concurrent
lanes is a scheduling decision, and `dependency-graph-and-order.md` §4.3's merge order is the
orchestrator's. A proposed rule to decide on, not asserted here:

1. A phase claims a persist version **in its plan text**, when the plan is written, not at merge.
2. `migrate` becomes an ordered chain of per-version steps (`1→2`, `2→3`), each total over
   `unknown`, because `store.ts:43-53` says it runs before any error boundary exists.
3. A phase that merges second **renumbers its step and chains it**. There is never a second "v2".
4. A new persisted per-branch key ships with its prune, because substrate §8 item 4 records the
   unbounded `pinned` blob as an open carry and copying its shape copies the defect.

### 6.6 Critique §2.4 — two auth models never set side by side

**Finding.** The desktop hop sends no `Authorization` until the auth phase (`client.ts:6-8`); phase
7 has the app holding a `pnc_` bearer for `panacea-api`; nobody says which process holds it.

**Ruling — resolved on which process; deferred on whether the boundary is sufficient.**

**Provenance decides the process.** P7 requires the release link to be written by the process
that calls `POST …/deploy`, in the same operation. The ledger is in the control plane's state
directory. So **the control plane makes every `panacea-api` call** and **holds the token**. This
agrees with where phase 7 already put its client — *"a typed Go client plus a polling loop"*
(`survey-phase-7-deploy.md` verdict) in lane C — and it closes the ambiguity in its §5.4.

- **The renderer never holds the token.** It renders agent output and, from phase 5, design
  frames; it is the most exposed process in the app. The user pastes the token once into the
  renderer, which passes it to a **write-only** control-plane endpoint; nothing ever returns it.
- **Storage is a new dependency.** `control-plane/go.mod` has no keyring or credential-store
  module (checked for this draft: no match for `keyring`, `credential` or `wincred`), and
  `desktop/src-tauri/Cargo.toml` has none either (critique §3.3). Choosing one is phase 7's cost.

**Deferred — to the owner of control-plane authentication, before phase 7a's first task.** The
loopback bind, Host guard and Origin allowlist were designed to stop a *browser* reaching the
control plane. They do not stop **another local user's process** connecting to `127.0.0.1:8730`.
Today that exposure lets a stranger drive sessions; with this ruling it would also let one read
Panacea projects as the user and, from 7b, deploy as them. Whether that is acceptable until the
auth phase, or whether 7a must bring per-user local auth with it, is a security ruling this
contract does not own. It gates **7a**, not only 7b, because reads need the token too.

**Also unowned:** `client.ts:6-8` says auth arrives in **phase 8**; the phase 4 plan's out-of-scope
list (`:6512`) says **phase 9**; `PROGRAM.md:108-112` gives phase 8 to Kata and phase 9 to
hardening without naming control-plane auth in either. Open question 6.

## 7. Open questions for the product owner

### 7.1 Asserted by this draft — overrule any of them and the named sections change

| # | Assertion | Sections that depend on it |
|---|---|---|
| A1 | The Brief keeps every revision, and every cross-phase reference is an immutable `brief:<id>@<rev>` key with a digest | §1, §2.1, §3, §6.1, §6.2 |
| A2 | A release is pinned to the Brief head **at the moment of Ship**; rollback inherits the re-run deployment's pin; nothing is pinned by observation | §3.4, P1, P2 |
| A3 | `contextRef` is stamped by the server path that materialised, on every process start — which rules out decision 0c's option 3 | §2.4, §6.3, P6 |
| A4 | The control plane makes every `panacea-api` call and holds the token; the renderer never does | §6.6, P7 |
| A5 | Nothing provenance-related is persisted in the desktop | §6.5, P4 |
| A6 | Northstar's phases other than the `VISION` gate are disabled in a Panacea build. **This was the phase 5 survey's recommendation to you, not a decision you have made**; P10 turns it into a rule only if you agree | P10, §3.2 |

### 7.2 Questions only the owner can answer

1. **Is the Ship-time head the right pin (A2)?** The alternative is the revision the merge was
   reviewed against. This draft pins Ship-time head and *shows* the merge revision beside it when
   they differ. If you want the manifest to lead with the reviewed revision instead, §3.4 flips.
2. **Releases deployed outside the cockpit** — auto-deploy on push, the Panacea portal. Is *"no
   brief is recorded for this release"* acceptable, or must those releases show the brief read
   from the shipped code's `CLAUDE.md` (which needs question 4 answered *yes*)?
3. **Should editing the brief after a design is approved void the approval?** If the brief is
   mirrored into the canvas as a guideline, Northstar's digest voids it automatically (§4.2). The
   alternative is to leave the approval standing and mark it `superseded`. The first is stricter;
   the second does not punish a typo fix.
4. **Is a worktree's `CLAUDE.md` — with its managed brief block — committed to the repository?**
   Yes gives every release a brief reading derived from the code itself (§3.4). It also puts the
   brief's text into git history, visible to anyone with the repo, which may matter to a regulated
   customer. This is phase 3's to build and yours to decide.
5. **Retention.** A1 and P13 keep every revision a link refers to, for as long as the link exists,
   and this draft keeps unreferenced revisions too. A regulated customer may require deletion of a
   brief. What should happen to a release manifest whose brief a customer has asked us to erase —
   render `missing`, or keep the revision under a retention exception?
6. **Who owns control-plane authentication, and does phase 7a wait for it (§6.6)?** `client.ts`
   says phase 8, the phase 4 plan says phase 9, `PROGRAM.md` names neither. The security ruling on
   holding a `pnc_` token behind a loopback-only boundary gates 7a's first task.

### 7.3 Decisions this draft needs from the orchestrator, not the owner

- **Decision 0c** between options 1 and 2 (option 3 is excluded by A3 — §6.3).
- **Persist-version allocation** across lanes — the proposed rule in §6.5.
- **Whether the Panacea metadata ask is raised now** (`dependency-graph-and-order.md` amendment
  3.1). Without it, a lost state directory makes pinned releases indistinguishable from unpinned
  ones (§3.4). The ask is additive and cheap to make; the answer has someone else's lead time.

## 8. What this contract changes in existing plans — a change list, not tasks

No implementation plan is written here (`CLAUDE.md`'s gate). This is the list a plan author
needs, grouped by the document that would change. Nothing below has been edited; every target
file is read-only in this pass.

### 8.1 Phase 4 plan — `docs/superpowers/plans/2026-09-18-phase-4-discover.md`

| Plan location | Change | Contract section |
|---|---|---|
| Task 7, `discoverstore` | append-only revision log; `Save` writes revision before head; a by-key read; no revision deleted on branch or worktree deletion | §2.1, P3, P13 |
| Task 8, `provenance` | brief-key grammar and parser (the only parser, P5); `Ref` gains `key`, `digest`, `headRevision`, `resolution`; digest over a canonical encoding; ledger schema and one writer function per link kind | §1.2–§1.4, §2.1 |
| Task 9, `discoverapi` route table (`:3612-3622`) | add `GET /refs?key=…` batch resolve; the branch endpoint stays | §3.1 |
| Task 5 / decision 0c | Materialise returns the key it wrote; session create and restart stamp `ContextRef` from it | §2.4, §6.3 |
| Desktop provenance task (`:5589` onward) | delete `useProvenance`; add `provenanceSlice`, the tick-driven loader and three selectors; `ContextChip` mounted from `contextRef`; `ShippedManifest` test asserts the pin and the superseded line | §6.4, §6.1 |
| Discover view | open a past revision read-only — the one-click half of §4's test | §4.1 |
| Survey wording | *"one endpoint, three surfaces"* becomes *two endpoints, one type, four surfaces* (Design joins) | §3.1 |

### 8.2 Phase 2 contracts — amendments, owned by the phase that takes the file

- **Go contract:** `contextRef` semantics (§2.4); restart restamps; the create request's value is
  not trusted.
- **UI contract §3.1:** `CockpitState` gains `provenanceSlice` (not persisted, so §3.5's version
  is untouched).
- **UI contract §4:** create, resume and fork stop sending `contextRef`.
- **UI contract §4.7:** the poller's one-request rule gains the edge-triggered batch resolve.
- **UI contract §2 / `types.ts`: no change.** Verified against the file for this draft:
  `contextRef: string` with its comment already says what this contract needs.

### 8.3 Obligations on plans that do not exist yet

| Phase | Obligation |
|---|---|
| **3** | criterion evaluator takes `(brief key, commit)`, never a branch id (§2.3); writes the merge link and chooses its home; decides whether `CLAUDE.md` is committed (open question 4); branch deletion archives a brief head and never deletes a revision (P13) |
| **5** | writes the design link at `VISION` approval; the guideline mirror carries a marker and a drift check (§3.3); provenance drawn only by our chrome (P11); Northstar's other phases disabled if the owner agrees (A6); `session → design key` link shape finalised |
| **7** | release link in the same operation as `POST …/deploy` (P7); rollback inherits the source pin; no observation pin (P2); token held by the control plane behind a write-only endpoint (§6.6); write-once PipelineRun mirror if the metadata ask is granted |

---

*End of draft. Review order suggested for the owner: §1.5, §4's test, §5, then §7.*
