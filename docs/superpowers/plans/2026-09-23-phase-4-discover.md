# Phase 4 — Discover (Define): implementation plan

> **For agentic workers:** execute task by task with `superpowers:subagent-driven-development` or
> `superpowers:executing-plans`. Every task's part file is
> `docs/superpowers/plans/2026-09-23-phase-4-discover-parts/task-NN.md`. Steps use `- [ ]`
> checkboxes. Every task is test-first.

**Status:** written 2026-09-23 on branch `feat/phase-4-discover` (worktree
`.claude/worktrees/phase-4`, based on `3303fd6`). Owner-reviewable under `OD-1`; adversarially
reviewed before code. **Nothing in this plan was compiled or run.** Every citation was opened in
this pass unless it is marked *unverified*; §11 lists what was not verified.

## Supersedes `2026-09-18-phase-4-discover.md`, and what changed

This plan **supersedes** `docs/superpowers/plans/2026-09-18-phase-4-discover.md` (written
2026-09-17, 6,527 lines). That plan was written before the provenance contract
(`planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md`) and before `OD-3`. Its question
flow, its golden `CLAUDE.md` render and its model client are still right, and this plan keeps them
by reference (each part says which old region it keeps and what it changes). Everything below
changed:

| # | The 2026-09-18 plan | This plan | Why |
|---|---|---|---|
| C1 | `discoverstore`: one JSON file per brief in `statepath.Dir("discover")`, **overwritten** on every save (`:3019`, `FileStore.Save` `:3226`) | **No local brief store.** Brief heads and immutable revisions live in the **Hub**, written through `hubclient.Briefs` (Hub contract §7.5, §9.2). The control plane persists no brief data to disk | `OD-3` (shared from day one); ADR-H1 §5.4 widens contract A5 to the whole machine; contract §0.2 (the old store kept no history) |
| C2 | `Revision++` inside `Record`/`Edit`/`Skip`/`Reopen`/`AddFollowUp` (`:1117`–`:1168`) | The **Hub** numbers revisions (`baseRevision + 1`, gapless). The domain produces a new `Content`; the control plane writes it with `baseRevision` = the head it read | Hub contract §7.5; ADR-H1 §7.1 row 4 |
| C3 | Brief ids `b-…`, minted locally | Hub-minted `bf-` + 16 hex; keys `brief:bf-…@n` | Hub contract §3.1–§3.2 |
| C4 | One key names only the branch brief; inherited project text is unpinned (`Document` `:1724-1730`) | A branch revision's content **pins the project revision it inherits** (`inherits: {key, digest}`), and a project edit writes a new revision of every complete inheriting branch brief in **one** Hub batch | `4-D3`, accepted under `OD-1`; critique-2 §1.1 |
| C5 | Answers carry `Source` only | Answers carry an **author** (`person` · `model_drafted` · `model_accepted`); a drafted answer is not settled, so a revision holding one is never complete and is never materialised | `4-D2`, accepted under `OD-1`; critique-2 §2.7 item 3 |
| C6 | Criterion ids positional, `criterionID(briefID, index)` | Criterion ids minted once (`c-` + 8 hex) and kept across edits by exact text; each criterion also carries a **text digest** | brief R6; phase 3 asks P4-1, P4-2 |
| C7 | The desktop calls `POST /briefs/{id}/materialise` when a brief completes (`:3620`, `:5265`); "Phase 3's session-start path calls `Materialise`" (`:42`) | An `api.Materialiser`, called **inside** `handleSessionCreate` and `handleSessionRestart` before `backend.Create`, returns the key it wrote, and the handler stamps `Session.ContextRef` from it. The create request's `contextRef` is ignored. No materialise route exists | ruling **R1**; contract §2.4, §6.3, P6; survey §4.1 |
| C8 | Materialise failure: unspecified | Refused with `409 brief_not_written`; the person may retry or choose **Start without the brief**, which records `contextRef = "none:declined"` | `4-D1`, accepted under `OD-1`; critique-2 §2.7 item 2 |
| C9 | Only complete-or-not was implicit | **Only a complete revision is inherited, pinned or approved.** A session inherits the branch brief's latest complete revision, else the project brief's, else nothing. `superseded` compares with `latestCompleteRevision`, not with the head | ADR-H1 Q1; fact sheet §2 fourth column |
| C10 | `useProvenance(api, branchId)`, one fetch per component (`:5716`) | Deleted. A non-persisted `provenanceSlice`, one loader on the poll tick over **key sources** registered as selectors, one batched `GET /v1/discover/refs` | contract §6.4, P8; ruling **R6** |
| C11 | `DiscoverView({api, view, …, onChange})`, prop-controlled, owned by "Phase 2's branch shell" (`:4966`) | A prop-less `DiscoverLens` in `AppShell.tsx`'s `discover` arm, reading a non-persisted `discoverSlice` | survey §4.2 |
| C12 | Task 10 creates `desktop/vitest.config.ts` and installs test tooling | Neither. The file exists with `setupFiles`, `clearMocks` and the `?raw` CSS include; this plan never writes it | survey §4.3; phase 2 UI contract §11 |
| C13 | `contextwrite` carries its own `writeAtomic` (`:2438`) | `writeAtomic` is **lifted** from `registry/filestore.go:460` into `statepath` with its tests, and `contextwrite` calls it | ruling **R2** |
| C14 | `Ref.contextPath`; `present` | `Ref` gains `key`, `digest`, `resolution` (`resolved` · `missing` · `erased` · `invalid` · `none`), `noneReason`, `latestCompleteRevision`, `inherits`, per-criterion `digest`; `contextPath` is dropped for a per-worktree materialisations route | contract §1.4; Hub contract §7.5 (`erased`); phase 3 P4-2 |
| C15 | `CLAUDE.md` always, never excluded | `CLAUDE.md` when the repository does not track it, with `/CLAUDE.md` added to the clone's `.git/info/exclude`; `.claude/CLAUDE.md` when `CLAUDE.md` is tracked; refused when both are | phase 3 ask P4-6; `3-D6` (the managed block is never committed) |
| C16 | Model-tailored options on every question (Task 3, `TailorOptions`) | **Dropped from v1.** The model paths are the follow-up question and the repo draft (brief S4). Options are the static `BaseQuestions()` set | a model call on every step for options nobody asked for in S1–S10; a GET cannot trigger one, and tailored options would need storing to survive a reload |
| C17 | Task 14's real-agent test run by the implementer | The scripted parts run in task 23; the real `claude` sign-in and the "does the agent know the codeword" step join the **owner's batched session** and stay unverified until then | `OD-12` |

---

## 1. Goal

A person opens a project or a branch in the cockpit and answers five questions, one at a time —
what it is for, who it is for, what it must not break, what has to be true before it merges, what
is out of scope. The answers become **the Brief**, stored in the Hub so a teammate sees the same
one. When any session starts on that branch, the control plane writes the Brief's latest complete
revision into the worktree's `CLAUDE.md` **before the agent's process starts**, and stamps the
session with the key of exactly what it wrote. The session does not get the Brief as advice; it
**inherits** it.

That key — `brief:bf-…@n` — is where the provenance thread starts. It appears as a chip on every
session pane in Develop, as the definition of done above the merge list (phase 3 mounts it), and on
the release (phase 7 mounts it). Every surface resolves a key through one batched endpoint into one
type, `provenance.Ref`, so no surface can drift from another. Any chip opens Discover at the exact
revision it names, read-only.

The model helps and never gates: a follow-up question, and on an existing repository a brief
drafted from a mechanical scan, both degrade to the plain five questions when no model is
configured. A drafted answer is held until a person accepts or changes it.

## 2. Architecture

```
 desktop (renderer)                     control plane (loopback, 127.0.0.1:8730)            Hub (Phase H)
 ────────────────────                   ─────────────────────────────────────────            ─────────────
 DiscoverLens ─┐                        discoverapi  ── routes under /v1/discover ──┐
 ContextChip ──┼─ discoverSlice         (mounted through api's route recorder)      │
 DefinitionOfDone  provenanceSlice      │                                           │
 ShippedManifest   (never persisted)    ├─ discoverhub.Service ── hubclient.Briefs ─┼──► /v1/projects/{id}/briefs
      │                                 │    binding (repo key → Hub project)        │    /v1/projects/{id}/brief-revisions
 poller tick ──► one GET /v1/discover/  │    reads, writes, 4-D3 fan-out, key cache  │    /v1/refs
                 refs for unseen keys   │                                           │
                                        ├─ discover (pure): questions, ops, Content, │
                                        │    Resolve (project → branch)             │
                                        ├─ claudemd (pure): Render, Merge, Strip,   │
                                        │    Inspect                                │
                                        ├─ provenance (pure): key grammar, Ref      │
                                        ├─ contextwrite: the Materialiser ◄── api.handleSessionCreate / Restart
                                        │    target choice, exclude line,              (R1: before backend.Create)
                                        │    statepath.WriteAtomic
                                        ├─ repoanalysis: mechanical scan, draft
                                        └─ modelclient: one Messages call (Completer)
```

**Four rules hold the shape.**

1. **The Hub stores; the control plane decides.** The Hub numbers, digests and resolves revisions
   and reads exactly one field of `content`, `status` (Hub contract §7.5). The questions, the
   inheritance rule, what makes a brief complete and what gets rendered are phase 4's Go, in the
   control plane. The desktop computes none of it and never computes a digest (contract P5).
2. **One writer per fact.** Brief text: only `discoverhub` through `WriteRevisions`.
   `Session.ContextRef`: only the session handlers, from the `Materialiser`'s return value. The
   managed block: only `contextwrite`. Criterion state: phase 3's evaluator; phase 4 serves `open`.
3. **A key names immutable text.** A brief revision never changes (P3). A branch revision pins
   the project revision it inherits (`4-D3`), so the resolved text an agent read is a pure function
   of one key. Every render — the managed block, a chip, the past-revision view — is computed from
   that key and never read back from a file (contract §3.3).
4. **Nothing provenance-related is persisted on the machine** (contract A5, widened by ADR-H1
   §5.4). The control plane holds resolved revisions **in memory** for its lifetime, because they
   are immutable; the desktop holds them in non-persisted slices. `PERSIST_VERSION` stays `1`
   (ruling R3: phase 4 claims no persist version).

**Packages** (all new unless marked): `control-plane/internal/{modelclient, discover, claudemd,
provenance, discoverhub, contextwrite, discoverapi, repoanalysis}`; `statepath` gains
`WriteAtomic` (lifted); `api` gains `Materialiser` and `RouteMounter` (declared amendments to phase
2 files). Desktop: `desktop/src/discover/`, `desktop/src/provenance/`, `desktop/src/api/discover.ts`,
`desktop/src/store/slices/{discoverSlice,provenanceSlice}.ts`.

**Dependency direction.** `api` imports none of phase 4's packages: it declares the two interfaces
and `cmd/api/main.go` wires phase 4's implementations in (R1's reason). `discoverapi` does not
import `api` either — it mounts through a `func(pattern string, h http.Handler)` — so `api`'s own
tests can import `discoverapi` to audit its routes without a cycle. Phase 4 imports only the Hub's
`hubapi` and `hubclient` (Hub contract §2's boundary: the control plane may import nothing else of
`internal/hub/...`).
