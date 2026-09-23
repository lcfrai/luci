# Adversarial review — Phase 4 Discover plan (2026-09-23)

**Reviewed:** `docs/superpowers/plans/2026-09-23-phase-4-discover.md` and its 23 parts in
`2026-09-23-phase-4-discover-parts/`, branch `feat/phase-4-discover`, plan head `0d82e7a`.
The reviewer was read-only except for this file.

**Verdict in one line:** the plan is thorough and mostly right, but it has **2 Critical** defects:
restart can destroy a running agent and then refuse, and brief text can forge the managed-block
markers. It also has 14 Important and 13 Minor issues. It needs fixes before code.

**How it was checked.**
- Every phase 2 citation listed under "Feasibility" was opened in this worktree, at base `3303fd6`.
- The Hub contract was read in `.claude/worktrees/hub` at HEAD `6a6bc31`.
  `git log a6ccd77..HEAD` on the contract and on ADR-H1 is empty, so the plan's Hub citations
  (made at `a6ccd77`) are still current.
- Phase 3's `githost` key form was checked in `phase-3/.../task-13.md:137`. Phase 5's cleanroom
  spec was checked at `:488` and `:553`.
- The reviewer compiled and ran nothing either.
- Claims drawn from the reviewer's own knowledge, not from this session, are marked as such.
- In the file:line references below, `plan.md` means the main plan file and `task-NN.md` means
  that task's part file.

---

## Coverage

| Source | What it asks | In the plan | Gap |
|---|---|---|---|
| OD-1 | Build on the recommendations; review before code; UI unmerged until the owner looks | §3.1; tasks 17–21 each say "do not merge" | — |
| OD-2 | Local clone per worktree | the exclude line goes to the clone's `info/exclude` via `rev-parse --git-path` (task 09) | — |
| OD-3 | Briefs shared from day one | no local store; `discoverhub` over `hubclient` (tasks 07, 08) | — |
| OD-4 | GitHub **and** Azure DevOps | binding by repo key | **Azure DevOps cannot be bound** until HA4-6 is settled (named, §8.2). v1 therefore covers OD-4 for GitHub only |
| OD-6 | Entra via the Hub | consumed, not run (task 14) | — |
| OD-8 | No PTY meaning; the transcript is read only through an adapter | G6. The plan writes a file the CLI reads and reads nothing back from the CLI | — |
| OD-11 | Clean-room canvas | seams S4-P5.1–3 only | — |
| OD-12 | Owner steps batched and unverified | task 23 O1–O5 | — |
| OD-13 | No Panacea source | — | — |
| 4-D1 | Refuse the start and say why; *Start without the brief* | tasks 09, 10, 20 | restart ordering (C1); 4 of 6 start paths bypass the refusal (I9); the refusal can arrive as a `timeout` (I4) |
| 4-D2 | Drafted answers are not inherited; the author is recorded | `Answer.Author`, `Settled`, accept changes the digest (task 03) | — |
| 4-D3 | **Every** inheriting branch brief gets a new revision | fan-out in one batch (task 08) | a branch brief whose head is open is skipped, and a race exists (I8) |
| R1 | Materialiser in `api`, between worktree-resolve and `backend.Create` | task 10 | the restart placement is unspecified relative to `Backend.Destroy` (C1) |
| R2 | Lift `writeAtomic` | task 01 | — |
| R3 | Phase 4 claims no persist version | G3; task 16 test | — |
| R4 | `upstream_unavailable` | consumed | — |
| R6 | One loader over key sources, no new timer | task 16 | the cache never evicts erased or new-brief data (I10) |
| Prototype states 1–15 | — | 1–6 and 14 → task 18; 7 → 21; 8, 9, 11, 12, 13 → 19; 10 → 20; 15 → 17 | all 15 mapped. The plan adds `erased`, `hub_off`, `unbound`, `no_remote` and `signed_out` (OQ-5 flags `erased`) |
| Security | not a section of the plan | — | **missing** — see C2, I13, M9 |

---

## Strengths

- **R1 is built as a guarantee, not a convention.** Required `Deps.Materialiser`; the request's
  `contextRef` is ignored and a test proves it (`TestCreateStampsTheReturnedKeyNotTheRequested`);
  call-order tests on create; a grep acceptance that `req.ContextRef` reaches nothing.
- **The pinned project revision** (`Content.Inherits`) plus `ErrInheritMismatch` makes a branch
  key name immutable resolved text. This is the core of the spine, and it is tested from both
  sides (tasks 05, 07).
- **The wire format is treated as a wire format.** `Content.V`; no `omitempty`, enforced by a grep
  and a golden. Every slice marshals as `[]`. The local digest is pinned to Hub vector 1. The
  reviewer checked the vector: `sha256:a72a06a1…e9fa` in task 08 matches Hub contract §3.5.
- **4-D2 is exact.** A drafted answer is not settled. Accepting it changes the author and so the
  digest. A draft never overwrites a person's answer.
- **P5 is enforced by tests, not review.** One parser for keys (task 06), with source scans in
  Go (task 14) and TypeScript (task 16).
- **Honest dependency handling.** Each of the nine Hub asks has a fallback, and §11 lists what was
  not verified.
- **No new timer.** The loader test uses `vi.getTimerCount()`, and the poller's own contract is
  amended in its comment only.
- **The supersedes table C1–C17** gives a reason for every change from the old plan.

## Feasibility — citations opened, and what they showed

**Exact:**

| Location | What the plan says |
|---|---|
| `sessions.go` | `:277` `handleSessionCreate`; `:291` worktree; `:337` the `--session-id` comment; `:376` `sessionLocate`; `:382` `Store.Create`; `:401` `ContextRef: req.ContextRef`; `:422` `Backend.Create`; `:879` `handleSessionRestart`; `:895` worktree; `:1010` `Backend.Create` |
| `api.go` | `:254` `registerAttach(guarded)`; `:292` the `/api/v1/` alias on `guarded`; the validation table `:213-224` |
| `registry/filestore.go:460` | `writeAtomic` |
| `worktree/worktree.go:32` | `Provider` |
| `types.ts:162-163` | `contextRef` |
| `poller.ts:1-3` | the header |
| `problems.ts:13` | `ProblemKind` |
| `store.ts:17` | `PERSIST_VERSION = 1` |
| `store.ts:29-42` | `partialize`, which has exactly twelve keys, as task 16 asserts |
| `tokens.css:110-113` | the radius scale |
| `routes_audit_test.go` | is `package api_test` |
| module path | `github.com/kpmg-au-emu/panacea-app/control-plane` |
| `project.Runner` | `project.go:56` |

**Wrong or incomplete** (the details are in the issues below):

| Location | What is wrong |
|---|---|
| `useKeyboardMap.ts:61` | the window listener is at `:166` |
| `project.go:172`, `:175` | `ProjectName` is at `:171` and `BranchName` at `:174` |
| `filestore.go:485` | the chmod is at `:488` |
| task 10's file list | omits `attach_test.go:303`, `sessions_test.go:575` (both `package api`, direct `New(Deps{…})`) and `cmd/api/main.go:122` (I1) |
| task 20's call sites | omit `EmptyDevelop.tsx:41` and `SessionPane.tsx:30`, and ignore resume `sessionVerbs.ts:317` and fork `:353` (I9) |
| the ExecRunner assumption | `project/gitrunner.go:87` formats the error with `%v`, not `%w`, so the exit code cannot be matched with `errors.As` (I7) |
| create ordering | "materialise, then mint and record" is false at the planned position: `:339` mints, `:356` runs `MkdirAll`, `:363` mints, and all three precede `:376` (M2) |

---

## Issues

### Critical

**C1. A restart can destroy the running agent and then refuse, leaving a record that names a dead
sandbox.**
- *Where:*
  - `task-10.md:135-139`: "after the worktree resolves (`:895`) and **before the restart's record
    write**, call `Materialise`".
  - The tests `TestRestartMaterialisesBeforeTheSandbox` and `TestRestartRefusalLeavesTheRecordAlone`
    (`task-10.md:156`, `:158`).
- *What is wrong:*
  - In the real handler, `Backend.Destroy` runs at `sessions.go:944-951`, between the worktree
    resolve (`:895`) and the record write (`:986`). "Before the record write" therefore allows the
    call to go after the destroy.
  - The natural spot, beside the record write, is after the destroy.
  - The tests do not catch this:
    - The call log records only `materialise, store.update, backend.create`. It never logs
      `Destroy`.
    - The refusal test asserts that the record is *unchanged*, which is exactly the broken state.
  - The same file says why the order matters for argv (`:919`): "a restart that cannot build its
    argv must fail while the old sandbox is still alive". The plan does not carry that rule over
    to `Materialise`.
- *Why it matters:*
  - The most common cause of 4-D1's refusal is the Hub not answering. With this bug, a Hub
    outage during a restart kills the person's running agent.
  - The 409 then leaves a record whose `SandboxID` names a destroyed container. Phase 2's reaper
    later writes it `exited` with "sandbox disappeared", which is a fabricated status.
  - The person's recovery, *Start without the brief*, then restarts a session that is already
    dead.
- *How to fix:*
  1. State that the call comes after the argv rebuild (`:921-941`) and **before
     `Backend.Destroy` (`:944`)**.
  2. Add `backend.destroy` to the recording fake's log. Assert the order
     `materialise, backend.destroy, store.update, backend.create`.
  3. Change the refusal test to assert that **`Destroy` was never called** and that the old
     sandbox is still in the fake backend.
  4. Add a restart refusal to task 23 step 9. Today it tests create only.
  5. Accept the known side effect: the old process is still running while the new block is
     written. It read its file at start, so its stamp stays true.

**C2. Brief text can forge the managed-block markers. One teammate's answer can block every session
start on a branch, and survive "Start without the brief".** There is no security section at all.
- *Where:*
  - `task-05.md:103-117`: `Render`, `Merge`, `Strip`. There is no escaping rule.
  - `task-03.md:282-306`: the `Apply` rules. There is no content check and no length cap.
  - `task-09.md:61-63`: a strip that fails is `write_failed`.
  - A grep of the plan and all parts for `escape|inject|<!--` finds no rule.
- *What is wrong:* answers are rendered verbatim into `CLAUDE.md`. Two cases:
  1. **A forged begin marker.** An answer containing `<!-- panacea:discover:begin brief=… rev=… -->`
     renders a block with two begin markers. The first write succeeds, because the file has no
     block yet.
     - Every later `Merge` returns `ErrDuplicateBlock` → `block_broken` → 409 on every start in
       every worktree of that branch, for every teammate who inherits it.
     - The override does not escape it. `WithoutBrief` strips both candidates, and `Strip` fails
       closed on the same duplicate, so the result is `write_failed`.
     - The person cannot start any session there without editing the file by hand.
  2. **A forged end marker.** An answer containing `<!-- panacea:discover:end -->` ends the block
     early.
     - `Strip` removes only up to the first end marker. The rest of the brief stays in the file
       as "user text": a `none:declined` session still reads brief text, and so does a session
       with nothing to inherit.
     - Each `Merge` also re-appends the tail, so the file grows on every start.
     - `Inspect` reports `edited` for ever.

  Neither case needs malice: pasting a copied block into an answer is enough. More broadly,
  under OD-3 a brief is **an instruction channel into other people's agents**. A team member's
  text becomes context for every teammate's sandboxed agent, which runs with their own
  credentials. The plan never names this.
- *Why it matters:* an availability failure that crosses users, from one edit, in a product sold
  to regulated enterprise.
- *How to fix:*
  1. In `discover.Apply`, reject any text or question prompt that contains the sequence
     `panacea:discover:`, with a new `ErrReservedText` → 400 `bad_request`, `field: "text"`.
  2. Make `Render` neutralise `<!--` in every user and model string anyway (for example, as
     `<!-\-`), as defence in depth, with a golden test.
  3. Make `Merge`, `Strip` and `Inspect` treat a stray end marker, or an end before the begin, as
     `ErrUnterminatedBlock`.
  4. Add tests: `TestAnswerCannotContainAMarker`, `TestRenderNeutralisesCommentOpeners` and
     `TestStrayEndMarkerIsBroken`.
  5. Add a short **Security** section to the plan with these points:
     - brief text is cross-user agent instructions;
     - write access is the Hub's member role, and the actor is audited in `brief.revised`;
     - the marker rule above;
     - answer-length caps (M6);
     - model egress (I13).

### Important

**I1. Task 10 makes `Deps.Materialiser` required, but not everything that builds `Deps` is
updated. The control plane cannot start from task 10 until task 14.**
- *Where:* `task-10.md:18-25` (the file list) and `:103` (the validation entry).
- *What is wrong:* `Deps` is built directly in three places the plan does not touch:
  - `attach_test.go:303` and `sessions_test.go:575`, both `package api`. They call
    `New(Deps{…})` and cannot see `api_test.go`'s builder, which is in `package api_test`.
    Task 10's own acceptance command then fails.
  - `cmd/api/main.go:122`. It gets `Materialiser` only in task 14, which waits on Hub C1/C2 and
    phase 3 task 13. No test in `cmd/api` reaches `api.New`: `TestRunRefusesANonLoopbackBind`
    exits first. So the break shows only when the binary starts: "api: Deps.Materialiser is nil".
- *Why it matters:* the easy repair is a default no-op materialiser, which the plan itself names
  as the mutant to kill (`TestNewRefusesANilMaterialiser`). The alternative is a phase branch whose
  binary does not start for most of the phase.
- *How to fix:*
  1. Add both test files to task 10's file list, each given a recording fake.
  2. Have task 10 wire `cmd/api/main.go` with an explicit, named interim value,
     `materialiserUntilTask14`. It returns `""`, logs once at start-up that sessions inherit
     nothing until phase 4 task 14, and is deleted by task 14.
  3. Add a test to task 14 that the interim symbol no longer exists.

**I2. A failed scan makes a session inherit nothing, silently.**
- *Where:* `task-10.md:130-134`, which passes `sessionLocate`'s `projectID` and `branchID`.
- *What is wrong:* `sessionLocate` (`sessions.go:467-490`) returns `"", ""` both when the worktree
  is outside every project and when `Projects()` or `Worktrees()` **failed**. The failure is only
  logged at Warn (`:474`, `:480`), and a git call can time out after 10 s (`gitrunner.go:17`).
  Task 09 defines no behaviour for `ProjectID == ""`. `Locator.ProjectPath("")` either becomes a
  plain error (a 500 on every start) or, if mapped to "nothing to inherit", a session with no brief
  and no warning. That is R1's own failure: "the session inherits nothing, silently".
- *How to fix:*
  1. Task 10 adds an error return to `sessionLocate` (it already owns `sessions.go` for this
     change).
  2. An enumeration failure becomes `*MaterialiseError{Reason: "project_unknown"}`, added to the
     closed reason set and to task 20's table.
  3. `""` is kept for a worktree that is truly outside every project.
  4. Test both paths.

**I3. "Strip both candidates" can edit a tracked file, which the plan says never happens.**
- *Where:* `task-09.md:61-70` (step 3: strip "from BOTH candidate files"; step 5: "strip the other
  candidate"); OQ-2's recommendation, "A tracked file is never edited".
- *What is wrong:* the candidate that is not the target is, by construction, often the **tracked**
  one. Suppose `CLAUDE.md` is tracked and contains a block. The block may have been committed
  before this phase, forced in with `git add -f`, or committed by an agent. Then every materialise,
  and every `WithoutBrief`, rewrites the tracked `CLAUDE.md` and leaves the worktree dirty. This
  repository tracks `CLAUDE.md` (P4-D), so the case is real here.
- *How to fix:*
  - Strip only candidates that `Git.Tracked` reports as untracked.
  - A tracked file that contains a block gets a new `Inspect` state, `committed`, shown in task 19.
  - Materialise leaves that file alone and still writes the untracked target.
  - Add `TestNeverStripsATrackedFile`.

**I4. Nothing budgets time. The 4-D1 refusal, the follow-up and the repository draft can all
reach the desktop as a `timeout`.**
- *Where:*
  - `client.ts:33` sets `DEFAULT_TIMEOUT_MS = 10_000`. Task 15 reuses the transport unchanged
    (`task-15.md:19-23`).
  - Hub contract §9.2 (`hub-contract.md:1335-1339`): each `hubclient` attempt times out at 10 s,
    with two retries after 250 ms and 1 s.
  - The follow-up gets a 20 s context (`task-04.md:83`).
  - The model client's default is 120 s (`task-02.md:52`), and `analyse` has no stated budget.
- *What is wrong:*
  - When the Hub is black-holed rather than refusing connections (VPN down, corporate proxy),
    `Materialise` can take about 31 s across two to three Hub reads. The desktop gives up at 10 s
    and shows `timeout`, not `brief_not_written`, so 4-D1's refusal surface never appears.
  - A model follow-up that takes more than 10 s is aborted, and Retry repeats the model call.
  - Task 23 step 9 stops the Hub, which refuses connections quickly, so it passes either way.
- *How to fix:*
  1. `Materialise` runs under a 5 s context. Its expiry is `hub_unavailable`.
  2. The Discover client passes `timeoutMs` per call: 30 s for `apply` with
     `propose_follow_up`, and 60 s for `analyse`.
  3. The server's model contexts are shorter than those client timeouts.
  4. Task 23 adds a step with a non-routable `PANACEA_APP_HUB_URL` (for example `10.255.255.1`).

**I5. The rendered block is not a function of its key. Renames make every worktree report
"edited by hand".**
- *Where:* P4-G (`plan.md:175`); `task-07.md:359-360` (titles use the Hub project name and the
  branch name); `task-05.md:58` (`Document.Title`); architecture rule 3 (`plan.md:97-100`: "Every
  render … is computed from that key"); task 09's `StateEdited` ("the marker names the right key,
  the text is not its render").
- *What is wrong:* the title line comes from the Hub project's current name, which
  `PATCH /v1/projects/{id}` changes, and from the branch's current name, which `Rebind` changes.
  After either rename, a fresh render of the same key differs in its first line. `Inspect` then
  shows `edited`, with a diff, on every worktree. P4-G's "cannot false-positive" holds only while
  nothing is renamed. The agent also reads a title that no digest covers.
- *How to fix:*
  - Take the branch part of the title from the revision's own envelope `branchRef`. It is
    immutable per revision, as §3.4 shows.
  - Either leave the project name out of the block, or have `Inspect` compare the block with its
    title line normalised.
  - State the rule in task 05, and add `TestRenameDoesNotLookLikeAHandEdit`.

**I6. What happens when the Hub fails during a resolve is defined three ways.**
- *Where:*
  - `task-07.md:368-371`, rule 5: every Hub error "reaches the caller **unchanged**". Rule 6: a
    nil Hub gives `missing` for every brief key.
  - `task-12.md:70-72`: slots that miss the cache become `unavailable`, with a 200.
  - `task-06.md:69-72`: `Unavailable` exists.
- *What is wrong:*
  - `Service.Resolve` returns `(refs, heads, err)`. As written it cannot both pass the error
    through and return per-slot `unavailable`.
  - With rule 6, a configured-then-unset Hub, or `hub_off`, shows every chip as `missing`. The
    chip's words for `missing` are *"this is data loss in our store"* (`task-17.md:104`). That is
    a false alarm about data loss.
  - `signed_out` during a resolve is not specified at all.
- *How to fix:* one rule, stated in task 07 and restated in task 12:
  - `Resolve` never returns a Hub error.
  - A slot served from cache stays `resolved`. Any other slot gets `unavailable` on a Hub error,
    `signed_out`, or a nil Hub.
  - `missing` comes only from the Hub's own answer.
  - Add `TestHubOffIsUnavailableNotMissing`.

**I7. Detecting a tracked file through `project.Runner` depends on an exit code that the runner
does not expose.**
- *Where:* `task-09.md:87-96`, `task-14.md:52-56` ("Exit status 1 from ls-files is untracked"),
  and the test `TestGitQuestionsReadsUntrackedAsFalse`.
- *What is wrong:* `ExecRunner.Run` returns `fmt.Errorf("git %s in %s: %v: %s", …)`
  (`gitrunner.go:87`). The `%v` drops the `*exec.ExitError`, so `errors.As` cannot find exit
  status 1.
  - An implementer who tries `errors.As` gets an error for every untracked `CLAUDE.md`. Every
    session start becomes a 500, or every brief is sent to `.claude/`.
  - The fake-runner test passes either way, because a fake can return any error type.
- *How to fix:* use `git ls-files -- <rel>`, without `--error-unmatch`.
  - It exits 0; empty output means untracked. No error parsing is needed.
  - `rev-parse` failing with "not a git repository" is the only string match left. Say so.
  - Add an integration test over `ExecRunner` in `t.TempDir()` with real git, which is already
    used by phase 2's project tests.

**I8. The 4-D3 fan-out misses two kinds of inheriting branch brief. Their sessions then show
`current` over outdated project text.**
- *Where:*
  - `task-08.md:86-88`, step 6: only branch briefs "whose HEAD is complete" are re-revisioned.
  - `TestFanOutSkipsOpenAndArchivedBranchBriefs` (`:124`).
  - C4 (`plan.md:27`).
  - The claim at `task-08.md:6-7` that "no session can start between the two and inherit a branch
    key whose pinned project text is stale".
- *What is wrong:*
  1. **Open heads.** A branch brief whose head is open but has an earlier complete revision is
     still inherited (P4-A: its latest complete revision), and that revision pins the **old**
     project revision.
     - An open head is common: a pending model draft, an unanswered follow-up, or an edit in
       progress.
     - The fan-out skips it. Sessions keep inheriting the old project text, and because
       superseded compares only within the branch brief (P4-B), the chip says `current`.
  2. **A race.** A teammate's branch op computes `Env.Inherits` from the project's latest complete
     revision (`@5`) just before the project batch commits `@6`. The branch write locks only its
     own head. It commits pinned to `@5`, after `List` has already run, and nothing re-pins it.
- *Why it matters:* 4-D3 exists so the chip can never read `current` over outdated inherited
  text. The immutability of each key still holds. What fails is that `current` means current.
- *How to fix:*
  1. Name the open-head case as a narrowing of 4-D3 in C4, and add it to §10 as an owner
     question.
  2. Recommend and build this: the fan-out also writes `reinherit` on an **open** head, so the
     person finishing the edit inherits `@6`.
  3. Add `selectProvenanceState` → `superseded` when the ref's `inherits.revision` is below the
     project brief's `latestCompleteRevision`, with the head read through `heads`.
  4. For the race, compare after the batch commits: re-list, and re-run the fan-out once for any
     complete branch brief whose `inherits.key` is older.
  5. Test both cases.

**I9. Four of the six ways a session starts bypass the 4-D1 refusal.**
- *Where:* `task-20.md:18-20`, `:30-34` (`PendingStart.kind: "create" | "restart"`) and `:77-79`.
- *What is wrong:* a `brief_not_written` 409 from these call sites still becomes a toast, or is not
  handled at all:
  - `EmptyDevelop.tsx:41`, the second `api.createSession` call site, outside `useCreateSession`;
  - `SessionPane.tsx:30-38`, the exit bar's own `restartSession`, which toasts through
    `sentenceForRestart`;
  - resume, `sessionVerbs.ts:317`;
  - fork, `sessionVerbs.ts:353`.

  Resume and fork build their create request **inside** `client.ts` (`:342-381`), so "Retry
  re-sends the identical request" cannot be done with a `CreateSessionRequest` the desktop never
  held.
- *How to fix:*
  1. List all six call sites in task 20.
  2. Make `PendingStart` a tagged union:
     - `create{request}`;
     - `restart{sessionId}`;
     - `resume{sourceId, harness}`;
     - `fork{sourceId, targetWorktreeId, harness}`.
  3. Give `resumeSession` and `forkSession` an options argument `{withoutBrief}`.
  4. Add one test per call site.

**I10. The desktop cache keeps erased text and never learns that a teammate created a brief.**
- *Where:* `task-16.md:29-31` ("Filled once per key, never overwritten") and `:125-126` (feed:
  `markStale` and `markViewStale` "for every view showing that brief"); Hub contract §9.3
  (`:1373-1375`, which invalidates `headRefByBranch` for `brief.*`).
- *What is wrong:*
  1. On `brief.erased`, `refsByKey` keeps the resolved content until the app restarts. The
     control plane expires it after its TTL; the renderer never does. Chips, the manifest and the
     past-revision view go on showing erased text. Task 07 names this exact failure, citing ADR-H1
     §6.8 (`task-07.md:389`); the reviewer did not open the ADR section itself.
  2. On `brief.created` or `brief.rebound` for a branch that had no brief, no view "shows that
     brief" (it shows `brief: null`), so nothing is marked stale. `headRefByTarget` is never
     invalidated either, so `DefinitionOfDone` stays *no brief* until the person navigates away
     and back.
- *How to fix:*
  - On `brief.erased`, delete every `refsByKey` entry whose `briefId` matches, then re-resolve.
  - On any `brief.*` event, mark stale every view and every `headRefByTarget` entry whose
    `binding.hubProjectId` equals the event's `projectId`.
  - Test both.

**I11. HA4-2 blocks tasks 07 **and** 08, and the plan contradicts itself on who adds it.**
- *Where:* `plan.md:434` ("task 08 is blocked"); `task-07.md:10-12` ("add it to the fake in the
  Hub's package as a separate commit") and `:168-172` (the `Hub` interface **embeds** `List`);
  `plan.md:294-295` ("**Never touched:** … anything under `control-plane/internal/hub/`").
- *What is wrong:*
  - Task 07's `Hub` interface requires `List`, so task 07 cannot compile against `NewFake()`
    without it.
  - The plan both forbids and instructs a write into `internal/hub`.
  - A grep of the Hub worktree's contract and plan for `HA4` or `Briefs.List` finds nothing, so no
    ask has reached the Hub planner.
- *How to fix:*
  1. Mark task 07 blocked by HA4-2 too.
  2. Pick one rule. The recommendation: the orchestrator carries HA4-1..9 to the Hub planner now.
     Until then, task 07 declares `List` on phase 4's **own** interface, with a thin phase-4-owned
     wrapper fake that embeds `hubclient.NewFake()`.
  3. Delete the instruction to write into `internal/hub`.

**I12. The number, Enter and `E` keys fire while typing in the free-text field.**
- *Where:* `task-18.md:18-20` ("element-scoped handlers on the question region") and the test at
  `:105`.
- *What is wrong:* the free-text field sits inside the question region, so its keydown events
  bubble to the region's `onKeyDown`. Typing "10 MB" picks option 1. Typing "Evidence" triggers
  edit. `Enter` in a multi-line answer sends it. The test dispatches only on the region and on
  `window`.
- *How to fix:*
  - The region handler returns early when the event target is an `input`, a `textarea` or
    `[contenteditable]`.
  - Inside the field, `Enter` sends only with `Ctrl+Enter`, or state the rule.
  - Add `typing digits and E in the field is text`.

**I13. The model's default endpoint sends client-confidential brief text to a public API.**
- *Where:* `task-02.md:34` (`DefaultBaseURL = "https://api.anthropic.com"`), `task-14.md:20`,
  `task-04.md:18` (the follow-up prompt carries "the person's own answers").
- *What is wrong:*
  - Setting only `PANACEA_APP_MODEL_API_KEY` sends every brief's answers to a public endpoint.
    Those answers are engagement descriptions — client names, scope, constraints. The deployment
    is a regulated one, where the corporate route is a gateway.
  - G4 governs whose key is used, but not where the data goes.
  - The ISM reasoning applied to file contents in task 13 is not applied to answers.
- *How to fix:*
  - Give `PANACEA_APP_MODEL_BASE_URL` no default. A key without a base URL is "model off", with a
    log line naming both variables.
  - Add a sentence to the new Security section (C2).

**I14. Phase 3's criterion states would be cached for ever inside an immutable `Ref`.**
- *Where:* `task-06.md:83-89` (`Criterion.State` inside `Ref`), `:98-101` ("Cacheable for ever
  EXCEPT headRevision and latestCompleteRevision"); seam S4-P3.4; `task-16.md:29-31`.
- *What is wrong:* `State` and `Because` come from phase 3's `Evaluate`, and they change as
  evidence arrives. The desktop caches the whole `Ref` by key for ever, so once phase 3 plugs in, a
  criterion's state never updates on screen.
- *How to fix:*
  - Take criterion state out of the forever-cached `Ref`, into a per-key moving map served beside
    `heads`. Alternatively, declare `criteria[].state` a moving field and give the loader its
    re-read rule.
  - Record the decision in S4-P3.4 so phase 3 builds against it.
