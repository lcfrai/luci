# Survey — Phase 4, Discover (product journey: **Define**)

Written 2026-09-23. Read-only pass. No code, no plan, no git write.
Scope: is `docs/superpowers/plans/2026-09-18-phase-4-discover.md` still coherent against what
Phase 2 actually shipped, and what does Discover need from the phases around it.

---

## Verdict, first

**The plan is ~85% still valid and 100% still worth keeping. Two things in it are now wrong, and
one of them is the seam the whole phase is built on.**

| | |
|---|---|
| **Still correct** | Go tasks 1–8 (modelclient, discover, claudemd, contextwrite, repoanalysis, discoverstore, provenance). Every seam the plan declared, Phase 2 delivered under Phase 4's exact names. |
| **Wrong — high** | The plan assigns the CLAUDE.md-before-PTY ordering guarantee to **Phase 3's session-start path**. Phase 3 does not own session start. **Phase 2 does, and it has already shipped it with no hook.** See §4.1. |
| **Wrong — medium** | Task 11 specifies `DiscoverView` as a **prop-controlled component** owned by "Phase 2's branch shell". Phase 2 shipped a zustand store and prop-less lens components. There is no branch shell. See §4.2. |
| **Needs a one-word edit** | Task 10 step 3 creates `desktop/vitest.config.ts`, which now exists with settings Phase 4's version would delete. Already flagged in the UI contract §11 and never actioned. See §4.3. |
| **Not invalidated** | Nothing in the phase-order correction of `6bcc9af` touches Phase 4. The plan already says Phase 3 = worktrees, 5 = design canvas, 6 = capacity, 7 = deploy. It was written against the corrected order before the correction. |

---

## 1. What this phase is, in product terms

A person opens a branch, answers five questions one at a time — what is this for, who for, what
must it not break, what has to be true before it merges, what is out of scope — and a `CLAUDE.md`
appears in that branch's worktree. Every agent session started there reads that file at process
start, so the brief is not advice, it is **inheritance**.

The same brief then reappears without being re-entered: as a context chip on every session header
in Develop, as the definition of done above the merge list in Merge, and as what-shipped attached
to the release in Deploy. One artifact, one endpoint, three surfaces. `CLAUDE.md` line 13 calls
that thread the product spine; the plan's Task 8 is the mechanism that makes it unbreakable — all
three surfaces render **one type** (`provenance.Ref`) fetched from **one endpoint**, so they
cannot drift.

Discover is also the on-ramp for an existing repo: analyse → model drafts a brief → the human
corrects it → pick where to start. Every model-assisted path degrades to a static one; Discover
works on a laptop with no credentials configured.

---

## 2. What Phase 2 actually shipped that Discover consumes — verified by reading the code

All five seams the plan declared were honoured. This is unusually clean and worth stating plainly.

| Seam | Phase 4 plan expects | Phase 2 shipped | Verdict |
|---|---|---|---|
| Worktree resolution | `contextwrite.WorktreeResolver` with `Path(ctx, worktreeID) (string, error)` | `worktree.Provider`, identical signature — `control-plane/internal/worktree/worktree.go:32`; `worktree.Static` at `:38`; `project.Scanner.Path` at `control-plane/internal/project/project.go:339` | **Exact match.** `contextwrite.NewWriter(scanner)` compiles with no adapter. |
| Display names | `discoverapi.Naming` — `ProjectName(id) string`, `BranchName(id) string`, never an error, id as fallback | `project.Scanner.ProjectName` / `.BranchName` — `project.go:172`, `:175`, both delegating to a cache with id fallback | **Exact match.** |
| State directory | `discoverstore.DefaultDir()` walking `PANACEA_APP_STATE_DIR` → `LOCALAPPDATA` → `XDG_STATE_HOME` → `~/.local/state` → temp | `statepath.Dir(sub)`, same ladder character-for-character, and its own package comment names `Dir("discover")` as the Phase 4 caller — `control-plane/internal/statepath/statepath.go:30` | **Exact match.** Phase 4's function becomes a one-line delegation. |
| Route prefix | Every Discover route and every Discover test uses `/api/v1/discover` | `/v1` is canonical; `guarded.Handle("/api/v1/", http.StripPrefix("/api", guarded))` at `control-plane/internal/api/api.go:274`, with a DO-NOT-DELETE comment | **Resolved in Phase 4's favour.** The plan's ~40 test URLs land unchanged. |
| Provenance carrier | Session must remember which brief it inherited | `registry.Session.ContextRef` (`registry/session.go:72`), accepted and returned by `POST /v1/sessions` (`api/sessions.go:65`, `:152`, `:301`), mirrored to `Session.contextRef` in `desktop/src/store/types.ts:163`, and carried optionally through `toCreateRequest` (`desktop/src/store/defaults.ts:110`, `:127`) | **Delivered, opaque.** Phase 2 stores it, never interprets it, never validates it, never fails a create on a missing brief. Correct. |

Two further facts Phase 4 will want:

- **The lens arm is one line.** `desktop/src/components/AppShell.tsx:26–33` routes `discover` to
  `LensPlaceholder phase={4}`. Replacing that arm is the entire mount.
- **The token layer already exists** and the UI contract §5.4 lists all 28 variable names Phase 4
  uses as present with the same meaning, with a Phase 2 test asserting they exist. Task 10 step 4's
  "check first, skip if present" wording is correct as written and needs no change.

---

## 3. Where the plan is still exactly right

Do not rewrite these. They were written against a Phase 2 that did not exist and they landed on it:

- **Task 1, no Anthropic Go SDK.** The reason given is verified and still true: `v1.73.0`'s
  `go.mod` declares `go 1.24`/`toolchain go1.25.8`, every module here is pinned to `go 1.23`, and a
  1.23 agent behind the corporate proxy cannot download a toolchain. The `modelclient.Completer`
  interface is the seam Phase 6's rotation engine replaces.
- **Task 2's one permitted denominator.** `QUESTION n OF 5` survives `AGENTIC-UX.md` anti-pattern 2
  only because the base question set is closed; model follow-ups append with `Step.Total = 0`. The
  reasoning is sound and the test that pins it (`TestFollowUpsCarryNoDenominator`) is the right test.
- **Task 2's `Brief.Edit` append-or-replace.** This is the regression test for `FUNCTIONAL-SPEC.md`
  known defect 7 (`editAnswer` splices all later answers). Three tests, one per layer.
- **Task 8's deliberate absence of `Met`/`Total`/percentage** from `provenance.Ref`, replaced by
  `settled`/`open`/`contested` with a mandatory `Because`. This is the correct reading of
  `AGENTIC-UX.md` and of `DESIGN-SYSTEM.md`.
- **Task 8's carried risk** — every criterion is `Open` in this phase because the evidence for
  `Settled`/`Contested` is merge and test data Phase 3 owns. Stubbing a *state machine* while
  shipping the whole *feature* is the right call and is honestly labelled.

---

## 4. Where the plan is now wrong

### 4.1 BLOCKER — the seam hazard is assigned to the wrong phase, and the right phase has shipped

The plan's seam table (line 42) and Task 5 (line 2139) both say:

> `Materialise` runs BEFORE `ptysession.Start`, never after. … **Phase 3's session-start path calls
> `Materialise` before `ptysession.Start`.**

**Phase 3 does not own session start. Phase 2 does, and it is already on disk.** The one place a
sandbox is created is `handleSessionCreate` in `control-plane/internal/api/sessions.go:205`. Its
order is: resolve the worktree (`:219`) → build the harness argv → mint ids → `Store.Create`
(`:283`) → `Backend.Create` (`:322`). The Phase 2 go contract §5.1 reserved the space in prose —
*"Anything Phase 4 later needs on disk before the agent starts — `CLAUDE.md` — is written between
steps 1 and 5"* — but **no hook was built**. `api.Deps` (`api.go:66`) has no `Materialiser` field,
and `api.New` validates every dependency it has, so adding one is a struct field, a validation
branch, a call site and tests in a file that changed today.

Worse, Phase 4's *own* plan never closes the gap either. Its only trigger for `Materialise` is the
desktop calling `POST /briefs/{id}/materialise` from `DiscoverView` when the brief completes (plan
line 5265, and the endpoint at line 3620). So today the ordering guarantee is **client-enforced
only**. The failure mode is precisely the one the plan warns about in three places: the user
finishes Discover, starts a session before the write lands or on a worktree that was never
materialised, and the agent inherits nothing — silently, with no error, and the feature looks built.

**Three ways out, ranked:**

1. **Server-side, at session create.** Add `Materialiser` to `api.Deps`; `handleSessionCreate`
   resolves the branch's brief and materialises between its step 1 and step 5. Makes the guarantee
   structural. Costs an edit to two Phase 2 files and a new Phase 2→Phase 4 package dependency
   (which inverts the current direction — `api` would import `discover`).
2. **A `contextRef` precondition.** If `POST /v1/sessions` carries a `contextRef` and the
   corresponding `CLAUDE.md` is absent or stale, materialise it first. Narrower blast radius,
   same structural guarantee, and it finally gives `contextRef` a job on the server.
3. **Leave it client-enforced and add an honest signal.** Discover's completion state already names
   the file it wrote; add a warning on the session header when `contextRef` is set but the
   worktree has no managed block. Cheapest, weakest — and it violates `AGENTIC-UX.md`'s "assume the
   user is intermittently present".

**This is the orchestrator's call, and it should be made before any Phase 4 task is written**,
because option 1 or 2 changes who owns `sessions.go` for the duration.

### 4.2 Task 11's component contract does not match Phase 2's shipped architecture

The plan (line 4966) specifies:

> `DiscoverView({ api, view, branchName, worktreeId, onChange })` — a controlled component. **The
> parent (Phase 2's branch shell) owns the `BriefView`** and receives every update through
> `onChange`, so the cockpit can render the context chip from the same data without a second fetch.

There is no branch shell. Phase 2 shipped a zustand store (`desktop/src/store/store.ts`, slices
`dataSlice` / `prefsSlice` / `uiSlice`) and lens components that take **no props at all** —
`LensRouter` renders a bare `<DevelopLens />` (`AppShell.tsx:24`), and `DevelopLens` reads the
store. `SessionPaneHeader.tsx` likewise reads the store directly.

The plan's stated *goal* ("the chip renders from the same data without a second fetch") is right,
and the zustand store is a **better** vehicle for it than prop drilling from a parent that does not
exist. The correction is small but must be made deliberately: Phase 4 adds a `discoverSlice`
holding `BriefView` by branch id and `ProvenanceRef` by branch id, and `DiscoverView` /
`ContextChip` read from it. Keeping the prop-based signature as written would mean threading state
through `AppShell` → `LensRouter` → `DiscoverView` and separately through
`DevelopLens` → `ColumnsLayout` → `SessionPane` → `SessionPaneHeader`, against the grain of
everything Phase 2 built.

### 4.3 Task 10 step 3 would delete Phase 2's vitest setup

Phase 4 Task 10 step 3 creates `desktop/vitest.config.ts` outright. It exists and carries three
settings Phase 4's version lacks:

- `setupFiles: ["./src/test/setup.ts"]` — without it jsdom has no `matchMedia`, no
  `ResizeObserver`, no `scrollIntoView`, and a large part of Phase 2's suite fails.
- `clearMocks: true`.
- `css: { include: [/\.css\?raw$/] }` — without it, `tokens.css?raw` is rewritten to `""` and the
  token assertions Phase 4 relies on for §5.4 compatibility silently pass against an empty string.

This exact conflict was identified in the Phase 2 UI contract §11 and marked *"Flagged for whoever
finalises the Phase 4 plan; Group D does not edit it."* **Nobody has actioned it.** The fix is to
give step 3 the same "check first, and if it exists confirm the settings below are present"
wording step 4 already uses for `tokens.css`. Same for step 1's `npm install` — every package is
already installed at the same versions.

### 4.4 Small: `discoverstore` cannot reach the retry helper the contract told it to use

Go contract §7.2 says Phase 4's store should adopt Phase 2's `writeAtomic`, which retries five
times on the Windows `ERROR_ACCESS_DENIED` a scanner holding the target produces. It is
**unexported**: `control-plane/internal/registry/filestore.go:460`. Phase 4 either duplicates it or
someone lifts it into a shared package (`statepath` is the natural home, and it already exists to
stop exactly this kind of duplication). One line of scope, worth deciding now rather than in a
review comment.

### 4.5 Small: a shipped placeholder still names the pre-correction phase number

`AppShell.tsx:62` tells the user Deploy "arrives in phase 5". Under the order corrected today
(`6bcc9af`) Deploy is **phase 7**. `discover`/`4`, `design`/`5`, `files`/`3`, `merge`/`3` are all
correct. It is a one-token fix in a file Phase 2 is still editing — the orchestrator should hand it
to a Phase 2 task, not to Phase 4.

---

## 5. The provenance thread — what this phase reads and writes

Discover is the **origin** of the thread. Nothing upstream of it writes provenance; everything
downstream reads it.

**Writes:**

- `discover.Brief` — the durable artifact, one JSON document per brief in
  `statepath.Dir("discover")`. Carries its own `Questions` so a brief opened tomorrow asks what it
  asked today even if `BaseQuestions()` changes. Has a monotonic `Revision`.
- The managed block in each worktree's `CLAUDE.md`, delimited
  `<!-- panacea:discover:begin brief=… rev=… -->` … `<!-- panacea:discover:end -->`. Deterministic
  render — no timestamp inside the block, or every materialise rewrites the file and shows up as a
  spurious diff. `claudemd.Merge` preserves user text outside the markers.
- `provenance.Ref` — served from `GET /api/v1/discover/branches/{branchID}/provenance`, **never
  404**; an absent brief returns `Present: false` with empty criteria. This is the single type
  Develop, Merge and Deploy render.
- `Session.contextRef` — set at session create by whoever starts the session, resolved back to a
  `Ref` through Discover's own endpoint.

**Reads:**

- `project.Scanner.ProjectName` / `.BranchName` for headings (Phase 2).
- `worktree.Provider.Path` to know where to write (Phase 2 today, Phase 3 once worktrees are
  created and destroyed for real).
- Nothing else. Phase 4 never touches git and never reads a CLI.

**Owes downstream, as a contract:** each of `ContextChip`, `DefinitionOfDone` and
`ShippedManifest` takes a `ProvenanceRef` **and nothing else derived from a brief**. If a mounting
surface needs a field, it goes into `provenance.Ref` in Task 8 — never computed locally. That rule
is the thread; a surface that derives its own view drifts, and the drift is invisible.

---

## 6. Phase-order observations, since the order is explicitly provisional

I have no reason to move Phase 4. Two observations for the orchestrator:

1. **Phase 4 does not need Phase 3.** Its only worktree dependency is `Path(id) → dir`, which Phase
   2 already satisfies via `project.Scanner`. Discover could run **concurrently with Phase 3**, in
   its own worktree, with no shared files except `api.go`/`sessions.go` (§4.1) and `AppShell.tsx`.
   If the goal is parallelism, 3 and 4 are the pair that separates cleanly.
2. **Phase 7 (Deploy) depends on Phase 4, not the reverse.** `ShippedManifest` is built here and
   mounted there. If Deploy is pulled earlier, it inherits a hole where the manifest goes. Phase 5
   (Design canvas) has a softer dependency — `StartMode "design"` in Task 13 is the only touch
   point, and Discover degrades fine without it.

---

## 7. What I did not verify, stated plainly

- **I did not run anything.** No `go test`, no `npm test`, no build. Every claim about Phase 2 is
  from reading source, not from execution.
- **I did not read the whole 6,527-line plan.** I read its structure, its seam table, the
  Interfaces block of every task, tasks 1/2/4/5/7/8/9/10/11/12 in the regions where they touch
  Phase 2, and the self-review. Claims about task internals I did not open (3, 6, 13, 14) are
  inferred from the Interfaces blocks and the self-review and are labelled as such.
- **The `deploy` placeholder phase number** (§4.5) is the only defect I found in shipped Phase 2
  code, and it is cosmetic. I did not audit Phase 2 for correctness — that is not this task.
- **Phase 2 is 21 of 26 tasks done and moving.** Every file I cite in `desktop/src` and
  `control-plane/internal/api` has a commit from today. Line numbers will drift; the symbol names
  should not.
- **Whether the `/api/v1` alias reaches a separately-mounted `discoverapi` handler** depends on how
  Phase 4 mounts it in `cmd/api/main.go`. The alias is registered on the `guarded` mux, so a
  Discover handler mounted on that same mux inherits it; one mounted on `root` would not. Not
  tested — the handler does not exist.
- **Phase 2 has a route audit** (`control-plane/internal/api/routes_audit_test.go`) with a stated
  rule that any later task registering a GET or HEAD owes it a line. Phase 4 registers three GETs.
  Whether that audit sees a handler outside the `api` package, I did not determine.
