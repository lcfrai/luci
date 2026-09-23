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

---

## 3. The decisions this plan rests on

### 3.1 Owner decisions (`planning/2026-09-23-owner-decisions.md`)

| Id | What it fixes here | Where |
|---|---|---|
| **OD-1** | Build now on the recommendations on file, which accepts the phase 4 brief's `4-D1`, `4-D2`, `4-D3` as recommended. UI branches stay unmerged until the owner has looked | throughout; tasks 17–21 |
| **OD-3** | Briefs and their revisions are shared, so they live in the Hub. There is no local brief store | tasks 07, 08 |
| **OD-5**, **OD-6** | The Hub is our own server; the control plane calls it as the signed-in user. Phase 4 consumes the Hub's sign-in and never runs one | tasks 07, 14 |
| **OD-8** | Discover reads no transcript. It writes a file the CLI reads; it never reads the CLI | global constraint G6 |
| **OD-12** | Every owner-observed step — the real agent reading the brief, the visual pass — is batched to the end and stays **unverified** | task 23; every UI task's acceptance |
| **OD-13** | No Panacea source; phase 4 touches no Panacea API | — |

### 3.2 The phase 4 brief's decisions (`scope-and-risk-phase-4-discover.md` §0), accepted under OD-1

| Id | Decision | How this plan builds it |
|---|---|---|
| **4-D1** | When the brief cannot be written before a session starts, **refuse the start and say why**; one button, *Start without the brief*, goes on | `409 brief_not_written` with `details.reason` (task 10); the desktop's retry and override (task 20); the override stamps `contextRef = "none:declined"`, which the chip shows as *started without the brief* (tasks 06, 17) |
| **4-D2** | A model-written answer is **not inherited** until a person accepts or changes it; the brief records who wrote each answer | `Answer.Author` (`person` · `model_drafted` · `model_accepted`), inside `content` and so inside the digest; a `model_drafted` answer is not settled (task 03) |
| **4-D3** | A project edit gives **every inheriting branch brief a new revision** | `Content.Inherits`; the fan-out batch in `discoverhub.Service.Apply` (task 08) |

### 3.3 Orchestrator rulings (`orchestrator-rulings.md`)

| Id | Ruling | Where |
|---|---|---|
| **R1** | `Materialiser` declared in `api`, called between worktree-resolve and `backend.Create` in create and restart, returning the key it wrote; phase 4 owns `sessions.go` for this change | task 10 |
| **R2** | Lift `writeAtomic` into `statepath` with its tests; do not duplicate | task 01 |
| **R3** | Persist versions are claimed in plan text. **Phase 4 claims none**: its two slices are not persisted | tasks 15, 16 |
| **R4** | `upstream_unavailable` with `details.upstream`; the Hub adds `"hub"` (ADR-H1 §5.1) | tasks 11, 15 — phase 4 consumes the code the Hub's task C2 adds |
| **R6** | The one loader unions **key sources** registered as selectors; it runs on the poll tick and on key-set changes, never on a new interval | task 16 |

### 3.4 The Hub (ADR-H1, Hub contract), as built against

| Id | What phase 4 takes | Where |
|---|---|---|
| Hub §7.5 | `POST /v1/projects/{id}/briefs`, `GET …/briefs`, `GET …/briefs/for-branch`, `GET /v1/briefs/{id}`, `PATCH` (rebind), `POST …/archive`, `GET …/revisions`, `POST …/brief-revisions` (the only write of brief text, 1–16 items, all or nothing), `GET /v1/refs` | tasks 07, 08 |
| Hub §9.2 | `hubclient.Briefs`, `hubclient.Projects`, `NewFake()`, `hubclient.Error`, `ErrUnavailable`; every Go test in this phase runs against the fake | tasks 07–14 |
| Hub §4.3 | The relay rule: a Hub 4xx passes through unchanged; 5xx and transport become `503 upstream_unavailable` (`upstream: "hub"`); a non-refreshable 401 becomes `401 signed_out` | task 11 |
| ADR Q1 | Only a **complete** revision may be pinned or approved against; phase 4 extends the same rule to what is materialised | tasks 06, 07, 09 |
| Hub §7.5 `erased` | A seventh provenance state | tasks 06, 17 |
| Hub §9.3 | The `Panacea-Hub-Cursor` header on `GET /v1/sessions` and `GET /v1/hub/changes`; **phase 4 owns the UI contract §4.7 amendment** that reads them | task 16 |

### 3.5 Decisions this plan makes itself — the plan author's, recorded so a reviewer can overturn them

| Id | Decision | Reason | Cost if wrong |
|---|---|---|---|
| **P4-A** | A session inherits the branch brief's **latest complete** revision, else the project brief's latest complete revision, else nothing. **Never a key the caller supplies** — a session started from an approved design inherits the head like any other, and phase 5 records the design separately | contract P6; fact sheet §5 (`upload-ui` inherits `b-42@6`, not the design's `@5`); critique-2 §7.2 #6 | phase 5's cleanroom spec `:488` says the design start *"passes the brief key as the session's `contextRef`"*; under this plan that value is ignored. Named as seam S4-P5.2 |
| **P4-B** | `superseded` means `revision < latestCompleteRevision`, not `< headRevision` | an unfinished edit at the head is not inheritable, so a session on the last complete revision is not working from outdated instructions | one comparison in one selector |
| **P4-C** | `contextRef` gains one local form, `none:declined`, owned by the `provenance` grammar and resolved without the Hub | `types.ts` is byte-gated and has no field for *why* a session has no brief (critique-2 §2.7 item 2, §3.2 item 2); the desktop still never parses it (P5) — it hands it to the resolver like any key | one grammar arm |
| **P4-D** | Materialise target: `CLAUDE.md` if the repository does not track it (and `/CLAUDE.md` is appended to `<clone>/.git/info/exclude`); `.claude/CLAUDE.md` if `CLAUDE.md` is tracked; refuse (`tracked_elsewhere`) if both are | phase 3 ask P4-6 and `3-D6`: the managed block is never committed. **Unverified** that the pinned Claude Code reads `.claude/CLAUDE.md`; task 23 carries the canary | a tracked-`CLAUDE.md` repository — this repository is one — falls back to a location the canary may disprove |
| **P4-E** | A local project is bound to a Hub project **by its remotes' repo keys only**. A repository with no GitHub or Azure DevOps remote shows *"this repository has no shared remote"*; nothing is bound locally | ADR-H1 §5.4: nothing Hub-derived persists on the machine; a brief nobody else can reach is not shared (`OD-3`) | open question OQ-1 |
| **P4-F** | Model-tailored options are dropped from v1 (change C16) | cost per step, and the options would need storing | the owner can ask for them back as one op |
| **P4-G** | A project-scoped brief is rendered with the branch's name in its title, so the same project key renders a different first line per branch | the title is where-you-are context, not brief text; the digest and the key still name the brief text only | the hand-edit check renders with the same branch name, so it cannot false-positive |

---

## 4. Global constraints

**Commands.**

- Go: `go test ./control-plane/... -race -count=1` from the repository root, or with a package path.
  `go test ./...` at the workspace root is invalid (`go.work` holds `./control-plane` and
  `./sandboxd`). `go vet ./control-plane/...` must stay clean.
- Desktop: `cd desktop && npx vitest run <path>` for one file, `npx vitest run` for all,
  `npx tsc --noEmit` for types. Nothing is installed: every package this phase uses is already in
  `desktop/package.json`. **Never write `desktop/vitest.config.ts`** (change C12).
- Every shell command in an agent session goes through `~/.claude/tools/claudezsh/claudezsh`.
  Commit by pathspec: `git commit -F <message file> -- <paths>`, after `git add <new paths>`.
  Never `git add -A`, stash, reset, checkout, rebase or amend. Write the message file under a
  **task-unique** name (the scratchpad is shared between agents; this plan's own first commit,
  `0c8454e`, carries another planner's message because two agents wrote one file name).
- Every commit message ends with exactly:
  `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`

**What never changes.**

- **G1.** The `go` directive stays `1.23` in every `go.mod` and in `go.work`. **This phase adds no
  Go module requirement** — `hubclient` and `hubapi` are packages of the same module (ADR-H1 §4.1).
- **G2.** `desktop/src/store/types.ts` does not change, byte for byte (phase 2 task 15's gate).
  `Session.contextRef: string` and its comment already say what this plan needs.
- **G3.** `PERSIST_VERSION` stays `1`; `partialize` does not change (R3; contract §6.5).
- **G4.** The control plane listens on loopback only. Configuration comes from `PANACEA_APP_*`
  variables. No secret is in source; the model key is `PANACEA_APP_MODEL_API_KEY` and is the
  person's **own** key or a corporate gateway key — never a Claude subscription token, never pooled
  across users (`CLAUDE.md`).
- **G5.** No progress bar, percentage, ETA or "N of M met" anywhere. The one denominator is
  `QUESTION n OF 5`, over the closed base set; follow-ups carry none (`AGENTIC-UX.md` principle 10).
- **G6.** Harnesses run unmodified and unwrapped. Discover writes a file the CLI reads at start; it
  reads no PTY byte and no transcript (`OD-8`).
- **G7.** Only the `provenance` Go package parses a key (P5). The desktop passes keys to the
  resolver and compares them for equality; it never splits one. No TypeScript digest exists.
- **G8.** One `setInterval` for data (the poller) and one for display (the heartbeat). Phase 4 adds
  **no** timer (R6).
- **G9.** One window `keydown` listener (`useKeyboardMap`). Discover's `1`–`4`, `Enter` and `E`
  are element-scoped `onKeyDown` handlers on the question region, never a window listener.
- **G10.** Every UI task reads `docs/DESIGN-SYSTEM.md` first and holds its non-negotiables: no
  indigo or violet, no gradient anywhere (phase 2's audit greps for `-gradient(`), elevation is a
  1px ring plus a layered shadow, radius by element size (`--r-chip` 6, `--r-ctl` 8, `--r-card`
  10, `--r-window` 14), `transform`/`opacity` only and never `transition: all`, one heartbeat
  (`hooks/heartbeat.ts`, never a second `requestAnimationFrame` loop — critique-2 §3.4), no
  spinner per row, `tabular-nums` on live numbers, colour plus shape for every state, `--ink-4`
  never as text. **The owner's eyes are the only visual acceptance** (`CLAUDE.md`); a screenshot
  or a passing test is not.

---

## 5. File map

**New — Go** (`control-plane/internal/…`):

| Path | Responsibility | Task |
|---|---|---|
| `modelclient/client.go` + test | one `POST /v1/messages`, no SDK; `Completer`, the seam phase 6 replaces | 02 |
| `discover/content.go` + test | `Content`, `Question`, `Answer`, `Criterion`, `Author`, `BaseQuestions()`, `Step`, `Recompute` | 03 |
| `discover/ops.go` + test | `Op` and `Apply`: record, edit, skip, accept, add/dismiss follow-up, restore, reinherit — the only way content changes | 03 |
| `discover/assist.go` + test | `ProposeFollowUp` over a `Completer`, degrading to nothing | 04 |
| `discover/inherit.go` + test | `Resolve(project, branch *Revision) Document` — project → branch, with origins | 05 |
| `claudemd/render.go` + test | `Render`, `Merge`, `Strip`, `Inspect`; the golden block | 05 |
| `provenance/key.go` + test | the key grammar — the only parser (P5): `brief:`, `design:`, `none:` | 06 |
| `provenance/ref.go` + test | `Ref`, `Build`, criterion digests, `Resolution` | 06 |
| `discoverhub/binding.go` + test | local project → Hub project by repo key; local branch → `refs/heads/…` | 07 |
| `discoverhub/read.go` + test | `View`, `History`, `Resolve(keys)`, `HeadRef`, `Inheritable`; the in-memory revision cache | 07 |
| `discoverhub/write.go` + test | `Apply`: one domain op → one Hub batch, with the `4-D3` fan-out and its conflict retry; `ArchiveHead`, `Rebind` | 08 |
| `contextwrite/materialise.go` + test | the `api.Materialiser`: target choice, exclude line, write, strip; `Inspect` for drift | 09 |
| `discoverapi/api.go`, `briefs.go`, `provenance.go` + tests | the `/v1/discover` routes; the relay rule; `Mount` | 11, 12 |
| `repoanalysis/scan.go`, `draft.go` + test | the mechanical scan; the drafted brief as one revision | 13 |

**New — desktop** (`desktop/src/…`):

| Path | Responsibility | Task |
|---|---|---|
| `api/discover.ts` + test | the TypeScript mirror of every `/v1/discover` wire shape; `DiscoverClient` | 15 |
| `store/slices/discoverSlice.ts`, `store/slices/provenanceSlice.ts` + tests | non-persisted: brief views by scope; refs by key, heads by brief, head refs by branch | 16 |
| `provenance/loader.ts` + test | the one loader over key sources, called from the poller's tick | 16 |
| `provenance/ContextChip.tsx`, `DefinitionOfDone.tsx`, `ShippedManifest.tsx`, `provenance.css` + tests | the three presentational surfaces; each takes a `ProvenanceRef` | 17 |
| `discover/DiscoverLens.tsx`, `QuestionFlow.tsx`, `BriefPanel.tsx`, `discover.css` + tests | the lens: binding states, asking, the brief born, inheritance, follow-up, model off, unavailable | 18 |
| `discover/BriefHistory.tsx`, `PastRevision.tsx`, `SessionsOnBrief.tsx`, `Materialisations.tsx` + tests | moved on, project changed, undo, past revision read-only, missing, erased, hand-edited block | 19 |
| `discover/StartWithoutBrief.tsx` + test | the `4-D1` refusal and override on session start and restart | 20 |
| `discover/ImportRepo.tsx` + test | drafted from the code: accept or change each answer | 21 |

**Modified — phase 2 files, each a declared departure named in its task:**

| Path | Change | Task |
|---|---|---|
| `control-plane/internal/statepath/statepath.go` (+ `writeatomic_test.go` moved in) | gains `WriteAtomic`, lifted from `registry/filestore.go:460` | 01 |
| `control-plane/internal/registry/filestore.go`, `registry/writeatomic_test.go` | calls `statepath.WriteAtomic`; the test moves out | 01 |
| `control-plane/internal/api/api.go` | `Deps.Materialiser` (required), `Deps.Discover` (optional `RouteMounter`), one validation branch, one mount line | 10 |
| `control-plane/internal/api/sessions.go` | create and restart call `Materialise` before `Store.Create` / the restart record; stamp `ContextRef`; ignore the request's `contextRef`; accept `withoutBrief` | 10 |
| `control-plane/internal/api/materialiser.go` (new file in `api`) | the interface, request, result and `*MaterialiseError` | 10 |
| `control-plane/internal/httpx/problem.go` | one code, `CodeBriefNotWritten = "brief_not_written"` (409) | 10 |
| `control-plane/internal/api/api_test.go`, `routes_audit_test.go` | the test `Deps` gain a fake `Materialiser`; the audit sees the Discover GETs | 10, 11 |
| `control-plane/cmd/api/main.go`, `config.go` | wiring and configuration | 14 |
| `desktop/src/api/client.ts` | `ProblemCode` gains phase 4's and the Hub's relayed codes; `createSession`, `resumeSession`, `forkSession` stop sending `contextRef`; `restartSession` takes an optional `{withoutBrief}` | 15 |
| `desktop/src/store/defaults.ts` | `toCreateRequest` stops setting `contextRef` | 15 |
| `desktop/src/store/store.ts` | `CockpitState` gains the two slices; `partialize` unchanged | 16 |
| `desktop/src/store/selectors.ts` | the key-source selectors and the provenance selectors | 16 |
| `desktop/src/api/poller.ts` | after `replaceSessions`, calls the loader; its header comment is amended (UI contract §4.7) | 16 |
| `desktop/src/components/develop/SessionPaneHeader.tsx` | mounts `ContextChip` | 17 |
| `desktop/src/components/AppShell.tsx` | the `discover` arm returns `<DiscoverLens />` | 18 |
| `desktop/src/hooks/useCreateSession.ts` | routes `brief_not_written` to the refusal surface instead of a toast | 20 |
| `desktop/src/styles/tokens.css`, `tokens.test.ts` | `--drift-tint` (the one token phase 4 lacks; phases 5 and 7 need the same one — whoever lands first adds it) | 17 |

**Documents:** `docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md` and
`2026-09-18-phase-2-ui-contract.md` (amendments, task 22);
`planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md` (amendments, task 22);
`docs/verification/2026-09-23-phase-4-results.md` (task 23).

**Never touched:** `desktop/src/store/types.ts`, `desktop/vitest.config.ts`, anything under
`control-plane/internal/hub/`, anything under `~/Projects/Paperdesign` or `~/Projects/Panacea`.

---

## 6. Tasks — dependencies and order

Each task is 1–3 hours for one implementer who has read this file and the task's part.

| # | Task | Depends on | Est. |
|---|---|---|---|
| 01 | Lift `writeAtomic` into `statepath` (R2) | — | 1 h |
| 02 | `modelclient` — kept from the old plan's task 1 | — | 1.5 h |
| 03 | `discover`: the content model and its ops (`4-D2`, R6 criterion ids, per-question skip) | — | 3 h |
| 04 | `discover`: the model follow-up, degrading to nothing | 02, 03 | 1.5 h |
| 05 | `discover.Resolve` with the pinned project revision (`4-D3`); `claudemd` Render, Merge, Strip, Inspect | 03 | 2.5 h |
| 06 | `provenance`: the key grammar, `Ref`, `Build`, criterion digests | 03, 05 | 2.5 h |
| 07 | `discoverhub`: binding and reads, against `hubclient.NewFake()` | 03, 05, 06, **Hub H0** | 3 h |
| 08 | `discoverhub`: writes, the `4-D3` fan-out, archive and rebind | 07 | 3 h |
| 09 | `contextwrite`: the Materialiser | 01, 05, 07 | 3 h |
| 10 | `api`: the `Materialiser` seam in create and restart (R1), `brief_not_written`, `RouteMounter` | — (declares its own interfaces; tests use fakes) | 2.5 h |
| 11 | `discoverapi`: brief routes, the relay rule, the route audit | 07, 08, 10 | 3 h |
| 12 | `discoverapi`: refs, branch provenance, materialisations | 09, 11 | 2 h |
| 13 | `repoanalysis` and the drafted brief | 02, 03, 11 | 2.5 h |
| 14 | Wiring and configuration in `cmd/api` | 09–13; **Hub C2** for the real client, **phase 3 task 13** for `githost` | 1.5 h |
| 15 | Desktop: the Discover client, wire types, problem codes, stop sending `contextRef` | 11, 12 (their contracts, not their code) | 2 h |
| 16 | Desktop: `discoverSlice`, `provenanceSlice`, selectors, the one loader, the poller amendment | 15 | 3 h |
| 17 | Desktop: `ContextChip`, `DefinitionOfDone`, `ShippedManifest`; the chip on every pane header | 16 | 3 h |
| 18 | Desktop: the Discover lens — binding, asking, the brief born, inheritance, follow-up, failures | 16, 17 | 3 h |
| 19 | Desktop: moved on, project changed, undo, past revision, missing, erased, hand-edited | 18 | 3 h |
| 20 | Desktop: `4-D1` on start and restart — retry and *Start without the brief* | 15, 16 | 2 h |
| 21 | Desktop: import a repository — drafted from the code | 13, 18 | 2 h |
| 22 | Contract amendments (Go, UI, provenance) | 10, 12, 16 | 1.5 h |
| 23 | End-to-end verification; the owner's batched steps written down | all | 2 h |

**Order.** Three lanes can run in parallel from the start, each in its own worktree:

- **Lane A (pure Go):** 01 → 02 → 03 → 04, 05 → 06.
- **Lane B (seams):** 10 — alone, because it edits `sessions.go`, which phase 2 (resume fix),
  phase 3 (task 05, `sessionEnv`) and this phase all touch (phase 3 ask P4-7). **Start it only after
  phase 2 task 26 has merged** (brief R5), and rebase-free: take the file as it is on the branch
  you are based on and name the base commit in the commit message.
- **Lane C (desktop):** 15 can start once 11's and 12's wire shapes are fixed in this plan (they
  are: parts 11 and 12), then 16 → 17 → 18 → 19, with 20 and 21 after.

Then 07 → 08 → 09 → 11 → 12 → 13 → 14 as the Hub's H0 lands (`hubclient` interfaces and fake; ADR
§5.9 ships them first), then 22 and 23 last. **UI branches (17–21) stay unmerged until the owner
has looked** (`OD-1`).

**Critical path:** Hub H0 → 07 → 08 → 09 → 11 → 12 → 14 → 23, about 18 engineer-hours after H0.
