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
2026-09-17, about 6,500 lines and 230 KB). That plan was written before the provenance contract
(`planning/2026-09-23-phases-4-5-7/provenance-contract-draft.md`) and before `OD-3`. Its question
flow, its golden `CLAUDE.md` render and its model client are still right, and this plan keeps them
by reference (each part says which old region it keeps and what it changes). Everything below
changed:

| # | The 2026-09-18 plan | This plan | Why |
|---|---|---|---|
| C1 | `discoverstore`: one JSON file per brief in `statepath.Dir("discover")`, **overwritten** on every save (`:3019`, `FileStore.Save` `:3226`) | **No local brief store.** Brief heads and immutable revisions live in the **Hub**, written through `hubclient.Briefs` (Hub contract §7.5, §9.2). The control plane persists no brief data to disk | `OD-3` (shared from day one); ADR-H1 §5.4 widens contract A5 to the whole machine; contract §0.2 (the old store kept no history) |
| C2 | `Revision++` inside `Record`/`Edit`/`Skip`/`Reopen`/`AddFollowUp` (`:1117`–`:1168`) | The **Hub** numbers revisions (`baseRevision + 1`, gapless). The domain produces a new `Content`; the control plane writes it with `baseRevision` = the head it read | Hub contract §7.5; ADR-H1 §7.1 row 4 |
| C3 | Brief ids `b-…`, minted locally | Hub-minted `bf-` + 16 hex; keys `brief:bf-…@n` | Hub contract §3.1–§3.2 |
| C4 | One key names only the branch brief; inherited project text is unpinned (`Document` `:1724-1730`) | A branch revision's content **pins the project revision it inherits** (`inherits: {key, digest}`), and a project edit writes a new revision of every live inheriting branch brief, **complete or open**, in **one** Hub batch; two post-commit checks re-pin a branch write that raced the batch (task 08 steps 8-9). **Narrowing of `4-D3` (OQ-8):** a branch whose head is open gets `reinherit` at that open head, but its latest complete revision, which P4-A has sessions inherit until the edit completes, is immutable and keeps the old pin. The chip reads `superseded` there (task 16, `selectProvenanceState` rule 4), never `current` | `4-D3`, accepted under `OD-1`; critique-2 §1.1; review I8 |
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
   that key **alone** and never read back from a file (contract §3.3). "Alone" includes the
   block's title: it comes from the revision envelope's own `branchRef`, never from a mutable
   project or branch name (P4-G, review I5).
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
and `cmd/api/main.go` wires phase 4's implementations in (R1's reason). `contextwrite` imports
`api` for the `Materialiser`'s request, result and error types, and `discoverapi` mounts through a
plain `func(pattern string, h http.Handler)`. `api`'s route audit (`routes_audit_test.go`) is the
external test package `api_test`, so it can import `discoverapi` to audit the real Discover routes
without a cycle. Phase 4 imports only the Hub's
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
| **4-D3** | A project edit gives **every inheriting branch brief a new revision** | `Content.Inherits`; the fan-out batch in `discoverhub.Service.Apply`, over complete **and open** branch heads, with two post-commit re-pin checks (task 08); `superseded` when a ref's inherited project revision is below the project's latest complete one (task 16). **Narrowed** for an open head's latest complete revision, which keeps its old pin (C4, **OQ-8**) |

### 3.3 Orchestrator rulings (`orchestrator-rulings.md`)

| Id | Ruling | Where |
|---|---|---|
| **R1** | `Materialiser` declared in `api`, called between worktree-resolve and `backend.Create` in create and restart (in restart, also before the old sandbox's `Backend.Destroy`, so a refusal leaves the running agent alive), returning the key it wrote; phase 4 owns `sessions.go` for this change | task 10 |
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
| **P4-D** | Materialise target: `CLAUDE.md` if the repository does not track it (and `/CLAUDE.md` is appended to `<clone>/.git/info/exclude`); `.claude/CLAUDE.md` if `CLAUDE.md` is tracked; refuse (`tracked_elsewhere`) if both are. A tracked candidate is never written, not even to strip a block; a block already committed in one is left alone and listed in the worktree inspection's `committed`, beside the target's own state and never instead of it; a start there is not refused (OQ-7) (tasks 09, 15, 19; review I3, re-review R-M1). Tracked-ness is read from `git --icase-pathspecs ls-files -- <rel>` output, never an exit code, so a tracked `claude.md` counts as a tracked `CLAUDE.md` on a case-insensitive disk (task 14; review I7, re-review R-I1) | phase 3 ask P4-6 and `3-D6`: the managed block is never committed. **Unverified** that the pinned Claude Code reads `.claude/CLAUDE.md`; task 23 carries the canary | a tracked-`CLAUDE.md` repository — this repository is one — falls back to a location the canary may disprove |
| **P4-E** | A local project is bound to a Hub project **by its remotes' repo keys only**. A repository with no GitHub or Azure DevOps remote shows *"this repository has no shared remote"*; nothing is bound locally | ADR-H1 §5.4: nothing Hub-derived persists on the machine; a brief nobody else can reach is not shared (`OD-3`) | open question OQ-1 |
| **P4-F** | Model-tailored options are dropped from v1 (change C16) | cost per step, and the options would need storing | the owner can ask for them back as one op |
| **P4-G** | **The rendered block is a function of its key, title included** (amended by review I5). A branch key's title is `Branch brief — ` + its own revision envelope's `branchRef` minus `refs/heads/`; a project key's title is `Project brief`. No project name and no current branch name reach the block (task 05's `Document.Title`, task 07 rule 2). *Was:* the Hub project's name and the current branch's name, so a project key rendered a different first line per branch | the project name changes under `PATCH /v1/projects/{id}` and the branch name under a rebind, while the key does not; a title read from either made `Inspect` report `edited` on every worktree after a rename, and gave the agent a line no digest covers. The envelope's `branchRef` is fixed per revision (Hub contract §3.4; a rebind writes no revision, §7.5) and the digest covers it. `TestRenameDoesNotLookLikeAHandEdit` (task 07) | the block no longer says which project it belongs to; the agent's repository and the cockpit's pane header already do. After a rebind, a block still names the ref its revision was written on until the next revision |

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
  across users (`CLAUDE.md`). Where the model's text goes is §4 Security S5's rule, not this one:
  `PANACEA_APP_MODEL_BASE_URL` has no default.
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

**Security** (added for review C2). Every task that touches brief text holds these.

- **S1. Brief text is cross-user agent instructions.** Under `OD-3` a brief lives in the Hub and is
  materialised into the `CLAUDE.md` of every worktree on its branch, for every teammate. One
  person's answer becomes context for another person's sandboxed agent, which runs with *that*
  person's own credentials and subscription. Treat every answer, prompt and criterion as untrusted
  input to someone else's agent: never write it into any file except inside the managed block, and
  never let it change the block's structure (S3). (Model prompts do carry it, in tasks 04 and 13;
  what the model says back is filtered before it reaches `Apply`, S3.)
- **S2. Who may write it, and the record of who did.** The only write of brief text is
  `POST /v1/projects/{projectId}/brief-revisions`, which needs the Hub's `member` role (Hub contract
  route 26, `hub-contract.md:588` at hub `2f84662`: minimum role `M`, per §5.3; a `viewer` gets
  403 `forbidden`, `:238`). Each
  revision emits one `brief.revised` event (`:804`) whose `actor` names the writer (`:1087-1088`).
  Phase 4 adds no second write path and never writes as anyone but the signed-in person.
- **S3. The marker rule.** No question prompt, answer text or criterion text that an op adds or
  changes may contain the reserved sequence `panacea:discover:`. `discover.Apply` checks those
  strings in the result of every op (a string the op left as it was is not re-checked) and refuses
  with `*ReservedTextError` (`errors.Is` → `ErrReservedText`) → 400 `bad_request`,
  `field: "text"`, `key` naming the question (task 03 rule 12, task 11); the desktop shows the
  message at that answer (task 18). The model's text is filtered before it reaches `Apply` (task
  04's follow-up degrades, task 13 drops the draft), so through phase 4's own routes the 400
  answers only text a person typed or restored. The Hub stores `content` without checking it
  (`hub-contract.md:740-743` at hub `2f84662`), so a head written straight to route 26 can hold
  the sequence; the unchanged-string rule keeps that head editable, and the next guarantee keeps
  it harmless.
  **The guarantee:** `claudemd.Render` rewrites every `<!--` in an interpolated string as `<!-\-`,
  so a rendered block holds exactly its own two markers (task 05), whatever the Hub stored.
  `Merge`, `Strip` and `Inspect` count markers and accept only none, or one begin followed by one
  end; a stray, early or second end is `ErrUnterminatedBlock` (task 05). The Materialiser reports
  a broken marker as `block_broken` on every path, including the *Start without the brief* strip,
  and keeps `write_failed` for a real read or write error (task 09, re-review I2). The desktop's
  `block_broken` sentence offers *Start without the brief* only when the refused start did not
  already carry it (task 20). Without this, one pasted block in an answer blocks every session
  start on the branch for every teammate, and the override cannot escape it.
- **S4. Answer-length caps.** Reserved for review M6 (a cap per answer and per prompt, and the
  fan-out batch checked against the Hub's body cap). That fix lands in its own batch; until then,
  the Hub's own 64 KiB envelope limit (`hub-contract.md:742`) is the only bound.
- **S5. Model egress (review I13).** Brief answers are client-confidential engagement text: client
  names, scope, constraints. The follow-up prompt carries the person's own answers (task 04), and
  the repository draft carries file names and counts, never contents (task 13). `G4` decides whose
  key is used. It does not decide where the text goes, and this rule does: **the model endpoint is
  never defaulted.** `PANACEA_APP_MODEL_BASE_URL` has no default (task 02 amendment 4 deletes the
  old `DefaultBaseURL = "https://api.anthropic.com"`). A key without a base URL is "model off":
  Discover degrades exactly as with no key, and one start-up line names both variables, never the
  key's value (task 14). Brief text therefore leaves the machine only for an origin that the
  deployment's owner set by hand, in a regulated deployment the corporate gateway. No code path
  guesses one.

---

## 5. File map

**New — Go** (`control-plane/internal/…`):

| Path | Responsibility | Task |
|---|---|---|
| `modelclient/client.go` + test | one `POST /v1/messages`, no SDK; `Completer`, the seam phase 6 replaces | 02 |
| `discover/content.go` + test | `Content`, `Question`, `Answer`, `Criterion`, `Author`, `BaseQuestions()`, `Step`, `Recompute` | 03 |
| `discover/ops.go` + test | `Op` and `Apply`: record, edit, skip, accept, add/dismiss follow-up, restore, reinherit — the only way content changes; the marker rule (`ErrReservedText`, S3) | 03 |
| `discover/assist.go` + test | `ProposeFollowUp` over a `Completer`, degrading to nothing | 04 |
| `discover/inherit.go` + test | `Resolve(project, branch *Revision) Document` — project → branch, with origins; `Title(branch *Revision)`, the block title's one rule | 05 |
| `claudemd/render.go` + test | `Render`, `Merge`, `Strip`, `Inspect`; the golden blocks; the `<!--` neutraliser and the marker count (S3) | 05 |
| `provenance/key.go` + test | the key grammar — the only parser (P5): `brief:`, `design:`, `none:` | 06 |
| `provenance/ref.go` + test | `Ref`, `Build`, criterion digests, `Resolution` | 06 |
| `discoverhub/binding.go` + test | local project → Hub project by repo key; local branch → `refs/heads/…` | 07 |
| `discoverhub/read.go` + test | `View`, `History`, `Resolve(keys)`, `HeadRef`, `Inheritable`; the in-memory revision cache | 07 |
| `discoverhub/service.go` | `Config`, `Service`, the in-memory revision cache | 07 |
| `discoverhub/fakehub_test.go` | phase 4's wrapper fake: `hubclient.NewFake()` embedded, plus `List` until HA4-2 lands (review I11) — in `discoverhub`, never in the Hub's package | 07 |
| `discoverhub/write.go`, `digest.go`, `lifecycle.go` + tests | `Apply`: one domain op → one Hub batch, with the `4-D3` fan-out and its conflict retry; the local digest until HA4-9; `ArchiveHead`, `Rebind` | 08 |
| `contextwrite/materialise.go`, `inspect.go`, `git.go` + tests | the `api.Materialiser`: target choice, exclude line, write, strip; `Inspect` for drift; the two read-only git questions | 09 |
| `discoverapi/api.go`, `briefs.go`, `provenance.go`, `analyse.go` + tests | the `/v1/discover` routes; the relay rule; `Mount` | 11, 12, 13 |
| `repoanalysis/scan.go`, `draft.go` + test | the mechanical scan; the drafted answers | 13 |
| `control-plane/cmd/api/discover.go` + test | `wireDiscover` and its three adapters | 14 |

**New — desktop** (`desktop/src/…`):

| Path | Responsibility | Task |
|---|---|---|
| `api/discover.ts` + test | the TypeScript mirror of every `/v1/discover` wire shape; `DiscoverClient` | 15 |
| `store/slices/discoverSlice.ts`, `store/slices/provenanceSlice.ts` + tests | non-persisted: brief views by scope; refs by key, heads by brief, head refs by branch | 16 |
| `provenance/loader.ts` + test; `api/hubFeed.ts` | the one loader over key sources, called from the poller's good tick; the Hub feed's control-plane route (Hub C2's) | 16 |
| `provenance/ContextChip.tsx`, `SessionContextChip.tsx`, `DefinitionOfDone.tsx`, `ShippedManifest.tsx`, `ProvenanceDetail.tsx`, `provenance.css` + tests | the three presentational surfaces, each taking a `ProvenanceRef`; the one store-reading mount | 17 |
| `discover/DiscoverLens.tsx`, `QuestionFlow.tsx`, `BriefPanel.tsx`, `BindingState.tsx`, `useBriefView.ts`, `discover.css` + tests | the lens: binding states, asking, the brief born, inheritance, follow-up, model off, unavailable | 18 |
| `discover/BriefHistory.tsx`, `PastRevision.tsx`, `SessionsOnBrief.tsx`, `Materialisations.tsx`, `SectionChanges.tsx` + tests | moved on, project changed, undo, past revision read-only, missing, erased, hand-edited block | 19 |
| `discover/StartWithoutBrief.tsx` + test | the `4-D1` refusal and override on session start and restart | 20 |
| `discover/ImportRepo.tsx` + test | drafted from the code: accept or change each answer | 21 |

**Modified — phase 2 files, each a declared departure named in its task:**

| Path | Change | Task |
|---|---|---|
| `control-plane/internal/statepath/statepath.go` (+ `writeatomic_test.go` moved in) | gains `WriteAtomic`, lifted from `registry/filestore.go:460` | 01 |
| `control-plane/internal/registry/filestore.go`, `registry/writeatomic_test.go` | calls `statepath.WriteAtomic`; the test moves out | 01 |
| `control-plane/internal/api/api.go` | `Deps.Materialiser` (required), `Deps.Discover` (optional `RouteMounter`), one validation branch, one mount line | 10 |
| `control-plane/internal/api/sessions.go` | create and restart call `Materialise` before `Store.Create` / the restart's `Backend.Destroy`; stamp `ContextRef`; ignore the request's `contextRef`; accept `withoutBrief` on both and pass it to `Materialise`; `sessionLocate` returns an error, and an enumeration failure is `409 brief_not_written` `project_unknown` unless `withoutBrief` (review I2) | 10 |
| `control-plane/internal/api/materialiser.go` (new file in `api`) | the interface, request, result and `*MaterialiseError` | 10 |
| `control-plane/internal/httpx/problem.go` | one code, `CodeBriefNotWritten = "brief_not_written"` (409) | 10 |
| `control-plane/internal/api/api_test.go`, `routes_audit_test.go` | the test `Deps` gain a fake `Materialiser`; the audit sees the Discover GETs | 10, 11 |
| `control-plane/internal/api/attach_test.go`, `sessions_test.go` (`package api`) | each builds `Deps` itself and gains its own recording fake; `sessions_test.go` holds the `sessionLocate` tests (review I1, I2) | 10 |
| `control-plane/cmd/api/materialiser_interim.go` (+ test) | `materialiserUntilTask14`: writes nothing, logs once at start-up; created by 10, **deleted by 14** (review I1) | 10, 14 |
| `control-plane/cmd/api/main.go`, `config.go` | `Materialiser: newMaterialiserUntilTask14(log)` (10), then wiring and configuration (14) | 10, 14 |
| `desktop/src/api/client.ts` | `ProblemCode` gains phase 4's and the Hub's relayed codes; `createSession`, `resumeSession`, `forkSession` stop sending `contextRef`; `restartSession` takes an optional `{withoutBrief}` | 15 |
| `desktop/src/store/defaults.ts` | `toCreateRequest` stops setting `contextRef` | 15 |
| `desktop/src/store/store.ts` | `CockpitState` gains the two slices; `partialize` unchanged | 16 |
| `desktop/src/store/selectors.ts` | the key-source selectors and the provenance selectors | 16 |
| `desktop/src/api/poller.ts` | **header comment only**, amended (UI contract §4.7) | 16 |
| `desktop/src/App.tsx` | the poller's good-tick callback also runs the loader | 16 |
| `desktop/src/components/develop/sessionVerbs.ts` | restart routes `brief_not_written` to the refusal | 20 |
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

Each task is 1–3 hours for one implementer who has read this file and the task's part. Task 10
is the exception, at 3.5 h, after review I1 and I2. It stays off the critical path only if it
starts early: it has no dependencies, so it can start before Hub H0, while 09 waits for 07 (3 h
after H0).

| # | Task | Depends on | Est. |
|---|---|---|---|
| 01 | Lift `writeAtomic` into `statepath` (R2) | — | 1 h |
| 02 | `modelclient` — kept from the old plan's task 1 | — | 1.5 h |
| 03 | `discover`: the content model and its ops (`4-D2`, R6 criterion ids, per-question skip) | — | 3 h |
| 04 | `discover`: the model follow-up, degrading to nothing | 02, 03 | 1.5 h |
| 05 | `discover.Resolve` with the pinned project revision (`4-D3`); `claudemd` Render, Merge, Strip, Inspect | 03 | 2.5 h |
| 06 | `provenance`: the key grammar, `Ref`, `Build`, criterion digests | 03, 05 | 2.5 h |
| 07 | `discoverhub`: binding and reads, against `hubclient.NewFake()` through a phase-4-owned wrapper that adds `List` | 03, 05, 06, **Hub H0**; **blocked by HA4-2** for the real Hub (until it lands, phase 4's own `Hub` interface declares `List` and the wrapper fake supplies it; nothing is written under `internal/hub/` — review I11) | 3 h |
| 08 | `discoverhub`: writes, the `4-D3` fan-out, archive and rebind | 07; **blocked by HA4-2** (the fan-out's `List`), HA4-9 | 3 h |
| 09 | `contextwrite`: the Materialiser | 01, 05, 07, 10 (it imports `api`'s request, result and error types) | 3 h |
| 10 | `api`: the `Materialiser` seam in create and restart (R1), `brief_not_written` (with `project_unknown`), `RouteMounter`; the binary's interim `materialiserUntilTask14` | — (declares its own interfaces; tests use fakes) | 3.5 h |
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

Then 07 → 08 → 09 (after lane B's 10) → 11 → 12 → 13 → 14 as the Hub's H0 lands (`hubclient`
interfaces and fake; ADR §5.9 ships them first), then 22 and 23 last. **UI branches (17–21) stay
unmerged until the owner has looked** (`OD-1`).

**Critical path:** Hub H0 → 07 → 08 → 09 → 11 → 12 → 14 → 23, about 18 engineer-hours after H0,
provided 10 has merged before 09 starts.

---

## 7. The contracts, and where each is written in full

A contract is written **once**, as complete source, in the part that produces it. Other parts cite
it. This table is the index.

| Contract | Complete source in | Consumed by |
|---|---|---|
| `statepath.WriteAtomic` | task 01 | 09, registry |
| `modelclient.Completer`, `Request`, `Response`, `Config` | task 02 | 04, 13, phase 6 |
| `discover.Content` (the Hub `content` object, **version 1**), `Question`, `Answer`, `Author`, `Criterion`, `Step`, `Op`, `Apply` | task 03 | everything |
| `discover.Document`, `Section`, `Origin`, `Resolve`; `claudemd.Render`/`Merge`/`Strip`/`Inspect`, the markers, the golden block | task 05 | 06, 09, 12 |
| `provenance` key grammar, `Key`, `Parse`, `Ref`, `Resolution`, `Criterion`, `Build` | task 06 | 07, 11, 12, phase 3 |
| `discoverhub.Service`, `Binding`, `View`, `RepoIdentity` | tasks 07, 08 | 09, 11, 12, 13, phase 3 (`briefref`) |
| `api.Materialiser`, `MaterialiseRequest`, `MaterialiseResult`, `*MaterialiseError`, `api.RouteMounter`; `brief_not_written` | task 10 | 09, 11, 14, 20 |
| `/v1/discover/…` routes and every JSON body | tasks 11, 12 | 15 |
| `desktop/src/api/discover.ts` (the TypeScript mirror) | task 15 | 16–21 |
| `discoverSlice`, `provenanceSlice`, the key-source rule, the provenance selectors | task 16 | 17–21, phases 3, 5, 7 |
| `ContextChip`, `DefinitionOfDone`, `ShippedManifest` props | task 17 | phases 3, 7 |

**The content object is versioned** (`Content.V = 1`). A revision is immutable and kept for ever,
so no migration may rewrite one: every reader handles every version ever written, and a new version
is additive (critique-2 §1.3's "Go-side document versioning", sharpened by P3). The canonical
encoding is the Hub's and carries its own `v` in the envelope (Hub contract §3.4).

---

## 8. Seams

### 8.1 Against phase 2 (shipped; this branch is based on `3303fd6`)

| Seam | State, read in this pass | Handling |
|---|---|---|
| `worktree.Provider.Path` (`worktree/worktree.go:32`) | exact match | the Materialiser receives the resolved path from the handler; it never resolves one itself |
| `project.Scanner.ProjectName`/`BranchName` (`project/project.go:172`, `:175`) | exact match | names in the title of the managed block |
| `handleSessionCreate` (`api/sessions.go:277`): worktree `:291` → … → `sessionLocate` `:376` → `Store.Create` `:382` → `Backend.Create` `:422` | no hook; `sessionLocate` (at `2d6d95b`: `:544-567`, called `:452`) answers `"", ""` alike for "outside every project" and a failed enumeration | task 10 calls `Materialise` after `sessionLocate` and before `Store.Create`, so a refusal writes nothing; `sessionLocate` gains an error, and a failed enumeration is `409 project_unknown` unless `withoutBrief` (review I2) |
| `handleSessionRestart` (`:879`): worktree `:895` → argv rebuild → `Backend.Destroy` `:944` → record `:986` → `Backend.Create` `:1010` (at `2d6d95b`: `:956`, `:972`, `:998-1018`, `:1026`, `:1067`, `:1091`) | no hook; no request body | task 10 calls `Materialise` after the argv rebuild and **before `Backend.Destroy`**, so a refusal leaves the running sandbox alive (review C1), and stamps `ContextRef` in the restart's record write; an optional body carries `withoutBrief`, which reaches `Materialise` beside the restart's own `Worktrees.Path` result (re-review I1); a record whose `ProjectID` is `""` is re-located first, under the create's rule: a failed enumeration is `409 project_unknown` unless `withoutBrief` (review I2; re-review b02 I-1) |
| The route recorder `routes` (`api/api.go` — unexported) and the §10a audit (`routes_audit_test.go`) | a package outside `api` cannot take `*routes` | `api.RouteMounter` hands `guarded.Handle` to `discoverapi.Mount`, so every Discover pattern is recorded and audited |
| The `/api/v1/` alias (`api.go`, on `guarded`) | lives on the guarded mux | Discover mounts on the same mux, so the alias reaches it |
| `writeAtomic` (`registry/filestore.go:460`, unexported) | — | lifted (R2, task 01) |
| `AppShell.tsx` `discover` arm → `LensPlaceholder` | read | task 18 replaces the arm |
| `poller.ts` — "exactly one request per tick" (`:1-3`) | read | task 16's declared amendment |
| `problems.ts` `ProblemKind` (three kinds, `:13`) | read | no new kind: Hub failures arrive as `upstream_unavailable`, which R4's owner adds as a kind; `brief_not_written` is `server-said` plus a code check at one call site |
| `types.ts` `Session.contextRef` (`:162-163`) | read | unchanged (G2) |
| `useKeyboardMap` — the one window listener (`hooks/useKeyboardMap.ts:61`) | read | Discover's keys are element-scoped (G9) |

### 8.2 To phase 3 (plan read at `.claude/worktrees/phase-3`, commit `a9e4289`)

| Id | What phase 4 provides | Phase 3's ask |
|---|---|---|
| S4-P3.1 | `discoverhub.Service` satisfies `briefref.Resolver` — digest and each criterion's id and text digest | P4-1, P4-2 |
| S4-P3.2 | `discoverhub.Service` satisfies `briefref.Archiver.ArchiveHead(ctx, projectID, branchID)` — archives the Hub head; revisions untouched | P4-3 |
| S4-P3.3 | `DefinitionOfDone` takes an optional `onRecordEvidence?(criterionId)` | P4-5 |
| S4-P3.4 | `provenance.Build` takes an optional `Evaluate func(Criterion) (State, Because)`; nil means every criterion `open` | P4-4 |
| S4-P3.5 | the Materialiser appends `/CLAUDE.md` to the clone's `.git/info/exclude` on first write | P4-6 |
| S4-P3.6 | **New ask of phase 3:** when task 11's `RenameBranch` succeeds, call `discoverhub.Service.Rebind(ctx, projectID, oldBranchName, newBranchName)` (names, because the old branch id may no longer resolve after the rename), so the brief follows the branch (Hub `PATCH /v1/briefs/{id}`). Without it a renamed branch shows *no brief* and its old brief sits on a ref nobody uses | — (not in phase 3's list) |
| S4-P3.7 | `sessions.go`: phase 4's call is in the handlers' step order; phase 3 task 05's is inside `sessionEnv`. They do not contend | P4-7 |

**A conflict between siblings, named, not phase 4's to settle:** phase 3's `githost.RepoKey` for
Azure DevOps is `"azure-devops/<org>/<project>/<repo>"` (phase 3 task 13), and the Hub's
`repoKey` pattern is `<org>.visualstudio.com/<project>/<repo>` (Hub contract §7.4). Phase 4 passes
whatever `githost` produces. Until the two agree, an Azure DevOps repository cannot be bound. Hub
contract ask HA4-6.

### 8.3 To phases 5, 6, 7 and 8

| Id | Seam | The later phase | Must not |
|---|---|---|---|
| S4-P5.1 | a key-source selector in `store/selectors.ts` for Design's bound and approved keys (R6) | phase 5 adds one selector | fetch provenance itself (P8) |
| S4-P5.2 | a session started from an approved design inherits the branch's latest complete revision (P4-A); phase 5 records `session → design:` through the Hub's session links | phase 5 | send `contextRef` and expect it kept |
| S4-P5.3 | `get_brief` (cleanroom spec `:553`) resolves through `discoverhub.Service.HeadRef` / `Resolve` | phase 5 | copy brief text into the canvas |
| S4-P6.1 | `modelclient.Completer` | phase 6's rotation replaces the implementation | change the interface |
| S4-P7.1 | `ShippedManifest({refValue, version, mergeRef?, designRef?})` renders the **pinned** `Ref` | phase 7 | pass a head `Ref` (P1) |
| S4-P7.2 | a key-source selector for release pins (R6) | phase 7 | resolve keys outside the loader |
| S4-P8.1 | `MaterialiseRequest.WorktreePath` is a host path. A cloud machine's worktree is on the pod's volume; phase 8 supplies a `contextwrite.FS` for it | phase 8 | write a brief by attaching to the PTY |

---

## 9. Hub contract asks

Each is a shape phase 4 needs that the Hub contract (read at `.claude/worktrees/hub`, commit
`a6ccd77`) does not provide. **No Hub route is invented here.** Each has what phase 4 does until
it is answered.

**Who carries them.** Phase 4 does not write the Hub's contract, plan or code. The
**orchestrator carries HA4-1..9 to the Hub planner** (review I11: a grep of the Hub worktree's
contract and plan for `HA4` or `Briefs.List` found nothing, so none had reached it). HA4-10 was
added later, for review I4, and goes the same way. Until an ask is in the Hub contract, the
"Until answered" column is what phase 4 builds.

*Status read 2026-09-24* (`.claude/worktrees/hub`, `feat/hub` at `d83ff43`): the Hub's **plan**
now answers HA4-1..9 (`2026-09-23-phase-h-hub.md:502-512`; HA4-2 as built with
`BriefQuery.Page` in place of flat `After`/`Limit`). Its contract does not yet (a grep of
`2026-09-23-hub-contract.md` for `HA4` or `BriefQuery` finds nothing), and none of it is built,
so the blocks in §6 stand until H0 lands with them. HA4-10 is not in that table, and a grep of
the Hub's plans for `HA4-10` finds nothing; whether the Hub plan has met it under another name
is **unverified** here.

| Id | Ask | Exact shape | Until answered |
|---|---|---|---|
| **HA4-1** | Publish the `hubapi` Go structs. §9.2 lists their names and says the JSON tags follow §7; the fields are not written anywhere | the struct set in part 07, §"Assumed `hubapi` shapes", derived field-for-field from §7.1 and §7.5 | phase 4 codes against part 07's assumed shapes; a rename is a mechanical fix |
| **HA4-2** | `hubclient.Briefs` has no list method, though route 19 exists | `List(ctx context.Context, projectID string, q hubapi.BriefQuery) (hubapi.Page[hubapi.BriefHead], error)` with `type BriefQuery struct { Scope string; BranchRef string; IncludeArchived bool; After string; Limit int }` | **tasks 07 and 08 are both blocked on it** for the real Hub (review I11): task 07's `Hub` interface needs `List`, and the `4-D3` fan-out cannot find the inheriting branch briefs without it. Until it lands, `List` is declared on phase 4's **own** `discoverhub.Hub` interface, and tests run through a phase-4-owned wrapper fake (`discoverhub/fakehub_test.go`) that embeds `hubclient.NewFake()` and adds `List`. Phase 4 writes nothing under `control-plane/internal/hub/` (§5, "Never touched") |
| **HA4-3** | The revision batch holds 1–16 items (§7.5). A project with more than 15 live branch briefs, complete or open (review I8), cannot re-revision them all in one transaction, and `4-D3` needs one | raise the limit to **64** for `POST …/brief-revisions`, with that route's body cap at **4 MiB**, and export it as `hubapi.MaxRevisionBatch` | the control plane refuses such an edit with `409 fanout_too_large` and a sentence naming the count (task 08) |
| **HA4-4** | Materialise at session start needs the **latest complete** revision's content; `for-branch` returns only the head revision | add `"latestComplete": BriefRevision \| null` beside `"revision"` in each of `branchBrief` and `projectBrief` | one extra `/v1/refs` call per session start (task 07) |
| **HA4-5** | The relay rule (§4.3) is "implemented once in `hubclient`", but no function maps an error to a response | `func Relay(err error) (status int, p httpx.Problem, ok bool)` in `hubclient`, `ok` false for a non-Hub error | `discoverapi.relay` implements §4.3's table locally, one function, marked for deletion when `Relay` lands (task 11) |
| **HA4-6** | One Azure DevOps repo-key form (§8.2 above) | either the Hub accepts `azure-devops/<org>/<project>/<repo>` or phase 3 emits `<org>.visualstudio.com/<project>/<repo>` | Azure DevOps repositories show *"this repository cannot be shared yet"* |
| **HA4-7** | The fake's completeness rule must follow `content.status` exactly and expose `latestCompleteRevision` on heads, and `/v1/refs`'s `heads` must include `latestCompleteRevision` for every brief it names | as §7.1's `BriefHead` and §7.5's `heads` already show | — (a confirmation, not a change) |
| **HA4-8** | The control plane polls the Hub's change feed (§9.3, task C2) but exposes no hook to other control-plane packages. `discoverhub` holds resolved revisions in memory and must drop one when a `brief.erased` event arrives | a subscription on C2's poller: `Subscribe(func(hubapi.Event)) (cancel func())`, called in commit order | the cache's TTL (default 5 minutes, task 07): an erasure reaches this process within the TTL, not at once |
| **HA4-9** | A branch revision pins the digest of the project revision written **in the same batch**, so the digest must be computable before the Hub writes it | `func BriefRevisionDigest(briefID string, revision int, projectID, scope, branchRef string, content json.RawMessage) (string, error)` in `hubapi` | `discoverhub/digest.go` builds §3.4's envelope and calls `hubapi.Canonical`, pinned to §3.5's vector 1 by a test, and the write checks the Hub's returned digest against it (task 08) |
| **HA4-10** | A change to the Hub's transport design, not a gap in it. Hub task 33 at `2f84662` (`phase-h-hub-parts/task-33.md`) specifies `Options.Sleep func(d time.Duration)`, defaulting to `time.Sleep` (`:29-31`), which no `ctx` can interrupt. It retries "a transport error, a timeout" (`:52-53`), which includes the caller's own `context.DeadlineExceeded`. Nothing in it says an attempt's deadline derives from the caller's `ctx`. Its only timing test, `TestEachAttemptTimesOut` (`:121`), never cancels the caller. The same text is on `feat/hub` at `33839b7` (`task-33.md:29-31`, `:56-57`). Discover's route deadlines and `Materialise`'s 5 s Hub budget need **(a)** (review I4) | three changes to task 33, most important first. **(a)** Each attempt's context is `context.WithTimeout(callerCtx, AttemptTimeout)`, never one built from `context.Background()`; test: with the default `AttemptTimeout` and `time.Sleep`, a caller `ctx` with a 100 ms deadline against a server that never answers returns in under 1.5 s (100 ms + 250 ms + 1 s if (b) and (c) are not taken), not about 31 s. **(b)** An attempt that failed because the caller's `ctx` is done (`callerCtx.Err() != nil`) is not retried; it returns at once, wrapping `ctx.Err()`. **(c)** `Sleep` becomes `func(ctx context.Context, d time.Duration) error` and returns `ctx.Err()` when `ctx` ends first; test: a `ctx` cancelled during back-off returns within 50 ms | only **(a)** is load-bearing. Without (b) and (c), a deadline that expires during an attempt costs up to 1.25 s more of back-off (250 ms + 1 s, each retry failing at once), and every phase 4 budget is sized to absorb that (task 11, "Back-off that cannot be interrupted"). Without (a), a black-holed Hub costs the full ~31 s whatever the budget, and task 23 step 10 fails at about 31 s. Declared fallback, built only if the Hub stream refuses (a): `discoverhub` runs each Hub call in a goroutine and returns on `ctx.Done()`, leaving the abandoned call to finish in the background (31 s at most) |

---

## 10. Open questions for the owner — each with the recommendation this plan builds on

| # | Question | Recommendation (built) |
|---|---|---|
| **OQ-1** | A repository with no GitHub or Azure DevOps remote — a fresh `git init` — cannot be bound to a Hub project. May it keep a brief anyway? | **No.** Discover shows *"this repository has no shared remote, so its brief could not be shared"* with the one command that adds one. A local-only brief would be a second store, and `OD-3` asked for one |
| **OQ-2** | `.claude/CLAUDE.md` as the fallback when a repository tracks `CLAUDE.md` (P4-D). If task 23's canary shows the CLI does not read it, what then? | Refuse the start with `tracked_elsewhere` and offer *Start without the brief* — never edit a tracked file |
| **OQ-3** | Should *Start without the brief* be remembered for a branch? | **No.** It is asked at each start, because `4-D1` makes going without the brief deliberate |
| **OQ-4** | Can a person archive a brief by hand? | **Not in v1.** Phase 3 archives the head when the branch is deleted (P13) |
| **OQ-5** | No prototype draws **erased**. Task 17 proposes a hollow square in `--ink-3` with *brief erased* | Accept the proposal at the batched visual pass, or name a shape |
| **OQ-6** | The prototype's "what changed" is a `CLAUDE.md` file diff; task 19 draws a section-level before and after, computed from two resolved refs | The section view; if the owner wants the file diff, task 12 gains `GET /v1/discover/diff?from=&to=`, rendered server-side |
| **OQ-7** | A repository has a Panacea brief block **committed** in a tracked `CLAUDE.md` or `.claude/CLAUDE.md` (P4-D; review I3). Panacea never writes a tracked file, so it cannot remove it. Should a session start there be refused? The cost of not refusing is provenance: every session there reads that block, and **no `contextRef` records it**. A session stamped `none:declined` (chip *declined*) or `""` (chip *none*) still reads a committed brief. A `brief:X@n` session (chip *current*) also reads a second, unstamped block, which may be older. Only task 19's panel shows it, as the worktree inspection's `committed` list. Task 09's WithoutBrief invariant is narrowed to match: a session labelled "started without the brief" finds no old brief **in a file Panacea writes** (`task-09.md` step 3) | **Do not refuse; show it.** The block is repository text under `3-D6`, and a person removes it in a commit. The seam: Inspect already reads each tracked candidate's markers for `committed`; if the owner rules "refuse", Materialise's step 2 makes the same read and returns one new reason, added to api's closed set (task 10) and task 20's sentence table. The chip rules are unchanged either way |
| **OQ-8** | `4-D3` says a project edit gives every inheriting branch brief a new revision. A branch brief whose head is **open** (a pending model draft, an unanswered follow-up, an edit in progress) is still inherited through its latest **complete** revision (P4-A), and that revision is immutable and pins the **old** project revision (review I8). Until the person completes the edit, sessions on that branch inherit the old project text. Is that narrowing of `4-D3` acceptable? | **Accept the narrowing; make it visible.** The fan-out writes `reinherit` at the open head too, so the edit in progress carries the new project text and the revision that completes it pins it (task 08 step 6). The chip reads `superseded`, not `current`, for any ref whose inherited project revision is below the project's latest complete revision (task 16, `selectProvenanceState` rule 4). The cost: the person editing that head meets `revision_conflict` on their next op and re-reads, as on a complete head. The seam, if the owner rules "no narrowing": Materialise refuses a start whose inherited key's pin is below the project's latest complete revision with one new reason, added to api's closed set (task 10) and task 20's sentence table, and the person completes the edit or chooses *Start without the brief* (`4-D1`). The chip rules are unchanged either way |

Carried and **answered elsewhere**: contract Q4 (is `CLAUDE.md` committed?) — no, by phase 3's
`3-D6`; contract Q5 (erasure) — ADR-H1 Q2, the `erased` state; contract Q6 (who owns control-plane
authentication) — the Hub (owner decisions `:35`).

---

## 11. What this plan did not verify

- **Nothing was compiled, run or rendered.** Every claim about code is a reading at `3303fd6`.
  Phase 2 is still committing on `feat/phase-2-cockpit`; line numbers in `sessions.go`, `api.go`
  and `desktop/src` will drift. Each part names the symbol as well as the line.
- **The Hub does not exist.** `hubclient`, `hubapi` and the fake are the contract's words; part 07's
  struct shapes are this plan's reading of §7 (HA4-1).
- **Claude Code's reading of `CLAUDE.md`**: that it reads `.claude/CLAUDE.md` (P4-D), and whether it
  re-reads either on `--resume` or mid-session (brief R7). The resume fix's experiment has no report
  in the tree yet. Task 23 carries both as canaries; the answer changes the restamp-on-resume rule
  and OQ-2.
- **Phase 3's `githost`, `briefref` and `RenameBranch`** are plan text (phase 3 tasks 09, 11, 13),
  not code.
- **Phase 5's cleanroom spec and phase 7's plan** were read only where they name phase 4.
- **The prototype** (`prototype/phase-4-discover.html`, 698 lines) was read for its states and its
  data model; it was not opened in a browser. The owner's eyes remain the visual acceptance.
- **Whether `hubclient.NewFake()` models `revision_conflict` under concurrent batches** is the
  contract's promise (§9.2); task 08's tests depend on it.
- **`entraauth.ErrSignedOut` is the Hub's task C1's.** Tasks 09 and 11 match it with `errors.Is`,
  so they cannot compile before C1 lands unless `hubclient` re-exports the sentinel. If neither is
  true when they start, they match a relayed 401 `signed_out` `*hubclient.Error` only and name the
  gap in the commit message; the `signed_out` refusal then reaches the person one layer later, never
  silently.
