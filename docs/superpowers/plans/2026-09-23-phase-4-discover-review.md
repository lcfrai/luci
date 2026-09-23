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
| the ExecRunner assumption | `project/gitrunner.go:74` formats the error with `%v`, not `%w`, so the exit code cannot be matched with `errors.As` (I7) |
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
  (`gitrunner.go:74`). The `%v` drops the `*exec.ExitError`, so `errors.As` cannot find exit
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

### Minor

**M1. Line numbers that have drifted.** Each part also names the symbol, so an implementer can
find it. Correct them anyway:
- `useKeyboardMap.ts:61` → `:166` (`plan.md:391`);
- `project.go:172`/`:175` → `:171`/`:174` (`plan.md:381`);
- `filestore.go:485` → `:488` for the chmod (`task-01.md:11`).

**M2. "Materialise, then mint and record" does not match where the call is placed.**
- *Where:* `task-10.md:14-16`.
- The planned position is after `sessionLocate` (`:376`). By then `ids.UUID()` (`:339`),
  `sessionConversationStore`'s `MkdirAll` (`:356`) and `ids.New("s")` (`:363`) have all run.
- The result is harmless: minting is in memory, and the directory is reused. But the sentence
  is false.
- *Fix:* either move the call to just after `Worktrees.Path` (`:300`), which also keeps a refusal
  from touching the disk, or reword the sentence to match the code.

**M3. Two different file names for the lifted helper.** `plan.md:269` says `statepath.go` gains
`WriteAtomic`, with `writeatomic_test.go` moved in. `task-01.md:17-19` creates `statepath/atomic.go`
and `atomic_test.go`. Keep one.

**M4. The `Evaluate` signature differs between the seam table and task 06.** S4-P3.4
(`plan.md:400`) gives `Evaluate func(Criterion) (State, Because)`. `task-06.md:138` gives
`func(briefKey string, c discover.Criterion) (CriterionState, string)`. Phase 3 will code against
one of them. Use task 06's in both places.

**M5. Change C14 lists five resolutions, but tasks 06 and 15 have six.** C14 (`plan.md:37`) omits
`unavailable`. The Hub contract calls `erased` the seventh provenance state and task 16 says eight
UI states. Add one sentence that reconciles the counts.

**M6. No length cap on answers.**
- *Where:* `task-03.md` rule 1.
- The Hub refuses a canonical envelope over 64 KiB (`hub-contract.md:742`) with a 400 whose
  message does not name the answer.
- A 4-D3 batch of many full branch briefs can also pass the Hub's 1 MiB body cap. HA4-3 asks for
  4 MiB for that route; until then, the limit stays 1 MiB.
- *Fix:*
  - Cap each answer at 4,000 characters and each question prompt at 300, with `ErrTooLong` →
    400 `field: "text"`.
  - Have `fanOut` check the batch's total encoded size against the cap before writing.

**M7. CRLF files on Windows.**
- This product runs on Windows first, and `Merge`, `Strip` and `Inspect` are specified in LF
  terms only.
- An editor that re-saves a person's `CLAUDE.md` with CRLF endings would:
  - turn the block into CRLF, so `Inspect` reports `edited` over every line;
  - leave a `\r` behind on `Strip`'s "one newline after it".
- *Fix:* match markers with `\r?\n`, compare the block with line endings normalised, and add a
  CRLF case to `TestStripRemovesTheBlockAndNothingElse`.

**M8. `Bind` claims every remote the clone has.**
- *Where:* `task-07.md:180-186`, `:255-258`.
- A fork's clone often has `origin` = the person's fork and `upstream` = the team's repository.
  `Bind` would register the team's key under the new project. If the team's project already
  exists, the lookup binds the fork to it.
- *Fix:* bind and register `origin` only. Look up by `origin` first and show other matches as a
  choice. Otherwise, state the rule as an owner question.

**M9. The exclude line hides the person's own untracked `CLAUDE.md` from git.**
- The case: an untracked `CLAUDE.md` that holds the person's own notes, or the output of
  Claude Code's `/init`.
- The effect: after the first materialise, `/CLAUDE.md` in `info/exclude` means a plain
  `git add` never stages the person's own file again.
- *Fix:* either state this in task 19's "where it is written" view, or, when the untracked file
  already has content outside a block, target `.claude/CLAUDE.md` instead.

**M10. `signed_out` refuses every start, which is at odds with Hub contract §9.5.**
- *Where:* `task-09.md:64-67`; Hub contract §9.5, `:1406-1407`: "none blocks the Develop lens".
- A person with the Hub configured who has not yet signed in is refused on every start, and
  OQ-3 means the refusal is never remembered.
- The CLAUDE.md "wall of configuration" bar is at stake.
- *Fix:* add this as an owner question with a recommendation. Keep the refusal, but put *Sign in*
  first in the surface.

**M11. An ambiguous task reference.** `task-17.md:37-38` says "declared departure into task 20's
file". In this plan, task 20 is `StartWithoutBrief`. The reference is to phase 2's task 20. Say
"phase 2 task 20's file".

**M12. The `.claude/CLAUDE.md` risk is probably lower than P4-D says.**
- *Reviewer's knowledge, not verified in this session*: the network tools were unavailable.
  Claude Code's public memory documentation lists `./CLAUDE.md` **or** `./.claude/CLAUDE.md` as
  project memory locations.
- Cite the docs page in P4-D, and keep canary O2: the pinned version is what counts.

**M13. The block is written before the exclude line.**
- *Where:* `task-09.md:72-77`, steps 6 then 7.
- If appending to `info/exclude` fails after the block is written, an untracked, un-excluded
  `CLAUDE.md` holding the brief sits in the clone. An agent's `git add -A` would commit it
  (3-D6).
- *Fix:* write the exclude line first; a failure there is `write_failed` before any brief text
  lands. Add `TestExcludeFailureWritesNoBlock`.

---

## Test strategy, sizing, order and design system

- **Red before green** is explicit in every part's Step 1 ("confirm they fail"), and every test
  names the mutant it kills. This is the house style and it is followed.
- **The mutants that survive** are the ones listed above:
  - materialise after destroy (C1);
  - an answer carrying a marker (C2);
  - a tracked candidate stripped (I3);
  - a black-holed Hub (I4);
  - a rename seen as a hand edit (I5);
  - a real `ExecRunner` exit code (I7);
  - an open-head fan-out (I8);
  - start paths that skip the refusal (I9);
  - keys typed in a field (I12).

  Each fix above names the test that kills it.
- **Sizing.** Parts are 1–3 h and mostly plausible.
  - Task 07 at 3 h is light. It has fifteen tests, the heap cache, the binding memo and the HA4-1
    reconcile. Expect 4–5 h, or split binding off into its own task.
  - Task 10 grows by I1 and I2, to about 3.5 h.
- **Order.**
  - Lane B's rule — start task 10 only after phase 2 task 26 — is right.
  - I1 changes what task 10 must leave behind: it must leave a binary that starts.
  - I11 adds task 07 to what HA4-2 blocks.
  - The critical path is otherwise right.
- **Design system.**
  - Every UI part (17–21) cites `DESIGN-SYSTEM.md` and holds its non-negotiables: no gradient, the
    radius scale, `transform`/`opacity` only, one heartbeat and no spinner, `tabular-nums`,
    colour plus shape, and no `--ink-4` as text.
  - Stylesheet greps enforce this, and the owner's eyes are named as the only visual acceptance.
  - No violations found in the plan text.
  - One caution: `--drift` is a violet hue (`tokens.css:77`, `oklch(… 316.7)`). `--drift-tint`
    must stay a flat tint, never a gradient. Task 17's grep already covers that.

---

## The two defects a first implementer is most likely to hit

1. **Materialise placed next to the restart's record write, after `Backend.Destroy`** (C1).
   - The plan's wording, "before the restart's record write", invites it.
   - Its call-order test does not log `Destroy`, and its refusal test asserts the broken state.
   - **The plan does not prevent it.** The fix in C1 does.
2. **Detecting "untracked" from `git ls-files --error-unmatch`'s exit code with `errors.As`**
   (I7).
   - `ExecRunner` flattens the error with `%v` (`gitrunner.go:74`).
   - The fake-runner test passes, and production then fails every session start, or sends every
     brief to `.claude/`.
   - **The plan does not prevent it.** The fix in I7 removes the need to parse the error.

A third defect is near-certain but loud: task 10's required field breaks `attach_test.go`,
`sessions_test.go` and the binary's start-up (I1). The tests will catch the first two. The
start-up failure appears only when someone runs the binary.

---

## Assessment

The spine is sound, and in places better than the documents it builds on:
- immutable `brief:bf-…@n` keys;
- a pinned `{key, digest}` for the inherited project revision;
- a server-stamped `contextRef` from a Materialiser inside session create and restart;
- one parser for keys;
- one loader;
- nothing persisted on the machine;
- the managed block kept out of every commit through `info/exclude`.

Its use of the Hub contract is careful and current. The contract is unchanged since the commit the
plan read, and vector 1 matches. Its honesty about what is unverified is good.

It is not ready for code:
- **C1** turns a Hub outage during a restart into a killed agent and a fabricated status.
- **C2** lets one line of brief text block every session start on a branch for a whole team, and
  the plan has no security section to catch that class.
- **I1–I3** and **I6–I9** are correctness defects that the plan's own tests would not catch.

All the fixes are local to the parts named. None changes an owner decision. The fix-up is about
half a day of plan edits. Nine areas need their tests added or rewritten:
- tasks 03, 05, 07, 08, 09, 10, 16, 18 and 20;
- task 23's step 9.

**Plan quality: Needs fixes**

---

## Fix batch b01 (2026-09-24): C1, C2

Commits `f7b75dc` (C1) and `d307ec4` (C2) on `feat/phase-4-discover`. Line numbers are as of
`d307ec4`. `sessions.go` was checked at `2d6d95b`, its last commit on this branch, and at `3303fd6`,
the plan's base. The Hub contract was checked at hub `2f84662`.

**C1: restart materialises before the destroy.** Fixed as the review said. The review's
`sessions.go` lines are right at `3303fd6` (destroy `:944-945`, record `:986`), but they have
moved on this branch. The plan now locates the call by symbol and cites both commits.

| How to fix | Where it changed |
|---|---|
| 1. After the argv rebuild, before `Backend.Destroy` | `task-10.md:136-148` (the handler bullet: at `2d6d95b` the argv rebuild is `:998-1018`, `sessionEnv` `:1022`, the destroy `:1025-1032`, the record `:1067`); `task-10.md:13-16` (the order rule); `task-10.md:175-177` (the Step 1 preflight also greps the restart's step order) |
| 2. `backend.destroy` in the fake's log, and the four-step order | `task-10.md:153-156` (the fake logs `backend.destroy`); `task-10.md:167` `TestRestartMaterialisesBeforeTheDestroy` |
| 3. The refusal leaves the old sandbox alive and never calls `Destroy` | `task-10.md:169` `TestRestartRefusalLeavesTheSandboxAlive` (replaces `TestRestartRefusalLeavesTheRecordAlone`) |
| 4. A live restart refusal | `task-23.md:56-62` (step 9: Hub stopped → 409; the container with the `panacea-app.sandbox-id` label is still running; the record is unchanged; the pane attaches) |
| 5. Accept the side effect | `task-10.md:147-148` |
| Other places that said the same thing | `2026-09-23-phase-4-discover.md:148` (R1), `:305` (file map), `:416` (seam 8.1) |

**C2: brief text cannot forge the markers.** Fixed as the review said, with two additions:

- The check runs on `Apply`'s whole *result*, not only the op's input text. That way a `draft`, a
  follow-up prompt and a `restore` are covered too. (Round 2 narrows it to the strings the op
  added or changed, still read from the result; see M4 below.)
- The two model paths filter the sequence out before `Apply`. Otherwise a person would get a 400
  for text the model wrote.

| How to fix | Where it changed |
|---|---|
| 1. `ErrReservedText` → 400, `field: "text"` | `task-03.md:273` (the var), `:279` (`ReservedMarker`), `:314-323` (rule 12); `task-11.md:93` (the mapping), `:131` `TestOpsRefusesReservedText` |
| 2. `Render` neutralises `<!--` | `task-05.md:109-124` (`<!--` becomes `<!-\-` in every interpolated string, including prompts and names); `:18` and `:249` (the third golden file); `:269` (an acceptance line: exactly 2 `<!--`) |
| 3. A stray or early end is `ErrUnterminatedBlock` in `Merge`/`Strip`/`Inspect` | `task-05.md:101-104` (the error), `:126-139` (one marker count for all three), `:153-155` (`Inspect`), `:255-258` (step 3 replaces the old first-end scan, checked at `2026-09-18-phase-4-discover.md:2079-2093`), `:9-12` (the Changed list); `task-10.md:79` (the `block_broken` comment) |
| 4. The three tests | `task-03.md:348` `TestAnswerCannotContainAMarker`; `task-05.md:241` `TestRenderNeutralisesCommentOpeners`; `task-05.md:242` `TestStrayEndMarkerIsBroken` |
| 5. A Security section | `2026-09-23-phase-4-discover.md:227-258`, inside §4 so no section number moves. S1: cross-user instruction channel. S2: the Hub `member` role (`hub-contract.md:588`, `:238`) and the `brief.revised` actor (`:804`, `:1087-1088`). S3: the marker rule. S4: reserved for M6. S5: reserved for I13. File-map rows at `:270` and `:273` |
| Task 09's strip-fail path | `task-09.md:64-69` (the note); `task-09.md:153` `TestWithoutBriefOverAStrayEndMarkerIsWriteFailed` (round 2 renames it `…IsBlockBroken` and makes the reason `block_broken`; see I2 below) |
| Model text never becomes a person's 400 (added) | `task-03.md:319-321`; `task-04.md:77` `TestAReplyHoldingTheReservedSequenceDegrades`; `task-13.md:94` `TestADraftHoldingTheReservedSequenceIsDropped` |

**Deferred or not verified:**

- S4 (M6) and S5 (I13) are placeholders only. Each fix lands in its own batch.
- No desktop task says where a 400 `bad_request` with `field: "text"` from `POST …/brief/ops`
  appears, for example inline at the answer. This belongs to the desktop error handling (tasks
  16 and 17), which this batch did not read. Left open.
- `<!-\-` is a guess at how Claude Code reads the text. It is not verified that the CLI shows it to
  the model as literal text. Task 23's canary covers the block as a whole, not this escape.

### Fix batch b01, round 2 (2026-09-24)

Answers `.superpowers/sdd/p4/plan-rereview-b01-1.md` (I1, I2, M1-M5). Commit `071d663` on
`feat/phase-4-discover`; line numbers are as of that commit. `sessions.go` re-checked at `2d6d95b`:
restart `Worktrees.Path` `:972`, `if sess.SandboxID != ""` `:1025`, the two restart store writes
`:1067` and `:1112`, `Backend.Create` `:1091`, the 200 `:1128`. The Hub contract's unchecked
`content` re-checked at hub `2f84662`, `hub-contract.md:738-744`.

| Re-review issue | Where it changed |
|---|---|
| I1.1 The restart names all four request fields | `task-10.md:138-143` (`WorktreeID`, `WorktreePath: path`, `ProjectID`/`BranchID` from the session record, `WithoutBrief: body.WithoutBrief`); `2026-09-23-phase-4-discover.md:314` (file map), `:425` (seam 8.1) |
| I1.2 `TestRestartWithoutBriefPassesTheFlag` | `task-10.md:173` (the flag, a non-empty path equal to `Worktrees.Path`, `none:declined` stamped) |
| I1.3 The live restart override | `task-23.md:62-67` (step 9: `{"withoutBrief": true}` → 200, `none:declined`, a new `sandboxId`, the old container gone, no block left) |
| I2 Recommended option: broken marker is `block_broken`, a real I/O error is `write_failed` | `task-09.md:62-73` (step 3's reason follows the cause); `task-09.md:157` `TestWithoutBriefOverAStrayEndMarkerIsBlockBroken` (renamed from `…IsWriteFailed`), `:158` `TestWithoutBriefWriteErrorIsWriteFailed` (new: `CLAUDE.md` is a directory); `2026-09-23-phase-4-discover.md:253-261` (S3); `task-20.md:37-39` (`PendingStart.withoutBrief`), `:63` (the two `block_broken` sentences), `:66-68` (no second override), `:80` `a refused override offers no second override` |
| M1 The ordering-rule comments name the destroy | `task-10.md:39-43`; `task-09.md:28-32` |
| M2 The restart order test's full log | `task-10.md:174` (a seeded `SandboxID` the fake knows; the exact log `materialise, backend.destroy, store.update, backend.create, store.update`) |
| M3 The desktop shows the 400 | `task-18.md:93` (a state row: the message at the answer `details.key` names, text kept, no Retry), `:110` `reserved text is refused at the field, without Retry` |
| M4 Option 1: check only what the op added or changed, and name the key | `task-03.md:271-284` (`ErrReservedText` doc, `ReservedTextError{Key}` with `Unwrap`), `:324-338` (rule 12), `:367` (`TestAnswerCannotContainAMarker` asserts the key), `:368` `TestUnchangedReservedTextDoesNotBlockOtherOps` (new); `task-11.md:93` (`details.key`), `:131`; `2026-09-23-phase-4-discover.md:242-252` (S3) |
| M5.1 S1's wording | `2026-09-23-phase-4-discover.md:233-235` ("never write it into any file except inside the managed block"; model prompts named) |
| M5.2 The model filters in prose | `task-04.md:31` (the `AssistFailed` comment), `:65-67` (the causes list); `task-13.md:59-64` (`DraftAnswers`' doc) |
| Restatements | `review.md:650-652` and `:663` annotated for the round-2 changes. The round-1 "Deferred" item on the desktop 400 is closed by M3 |

**Why M4 took option 1.** The neutraliser (task 05) is the guarantee that the file cannot be forged,
whatever the Hub stored. Rule 12 is the early refusal that can be explained. A whole-result check
lets one head written straight to route 26 refuse every op on that brief, including ops with no
text, so it turns defence in depth into a denial of service. Checking only what changed keeps every
op that adds text covered: `draft`, `add_follow_up` and `restore` are all read from the result.

**New, found while fixing, not fixed in this batch (N1).** This is the same dead end as I2, on the
`tracked_elsewhere` path. Task 09 step 2 (`task-09.md:58-61`) refuses `tracked_elsewhere` before
step 3's `WithoutBrief` strip runs. So *Start without the brief*, which is OQ-2's recommended
recovery (`2026-09-23-phase-4-discover.md:492`), always fails again when both files are tracked.
Round 2's `task-20.md:66-68` stops the loop at the second refusal, but it does not make the first
offer work. Step 3 also says "strip BOTH candidates", and it does not say that it skips a tracked
candidate. Stripping a tracked `CLAUDE.md` would edit a tracked file (3-D6). Suggested fix:
- under `WithoutBrief`, strip only untracked candidates;
- when both are tracked, return `none:declined` if neither tracked file holds a managed block, else
  `tracked_elsewhere`;
- task 20 then offers the override on `tracked_elsewhere`.
This is left for its own batch because it changes task 09's step order and OQ-2 is open for the
owner.

**Still unverified:** that Claude Code shows `<!-\-` as literal text (carried over), and whether
`discoverhub`'s revision cache lets `Materialise` succeed while the Hub is down. The cache question
is not read in this batch. If the cache does that, task 23 step 9's expected 409s would pass or
fail for the wrong reason.

## Fix batch b02 (2026-09-24): I1, I2

Commits `f1a9b6f` (task 10) and `ac2e515` (tasks 09, 14, 20, 22 and the plan) on
`feat/phase-4-discover`. Line numbers are as of `ac2e515`. The commit that adds this section also
rewrites `task-10.md:63-67` (the `ProjectID` doc, now also closing re-review R2-M2 item 4) and
rewraps `:344-345`, and moves no line. The source was checked at `2d6d95b`,
the last commit on this branch to touch `sessions.go`, `sessions_test.go` and `cmd/api/main.go`.
The review's I1 and I2 lines are right at `3303fd6` but have moved at `2d6d95b`:
- `sessionLocate` is `sessions.go:544-567` (was `:467-490`), called at `:452`;
- the `package api` literal in `sessions_test.go` is `:584` (the review's `:575` is inside the
  `sessionsLocator` literal just above);
- `main.go`'s `api.New(api.Deps{` is `:132` (was `:122`).
`git grep -n "New(Deps{\|:= Deps{\|api.Deps{" -- control-plane` finds exactly four direct
constructions: `api_test.go:142`, `attach_test.go:303`, `sessions_test.go:584` and `main.go:132`.
No `cmd/api` test reaches `api.New`: `main_test.go:387` and `config_sandbox_test.go:172` both exit
in config.

**I1: every `Deps` builder gets a Materialiser, and the binary starts.** Fixed as the review said.

| How to fix | Where it changed |
|---|---|
| 1. Both test files, each with a recording fake | `task-10.md:28-33` (file list); `:277-283` (attach's fake asserts it is never called; sessions' fake records); `:306-313` (Step 1 lists all four constructions, Step 2 adds the fakes) |
| 2. `main.go` wired with the named interim | `task-10.md:34-38` (files); `:155-193` (`materialiserUntilTask14`, complete source: logs one Warn at construction, `""`, or `none:declined` under `WithoutBrief`; the `main.go` line; why only a grep and a build can catch a missing line); `:297` `TestMaterialiserUntilTask14InheritsNothingAndSaysSoOnce`; `:332-345` (acceptance: the `cmd/api` suite, `grep "Materialiser:"`, `go build`) |
| 3. Task 14 deletes it and tests it is gone | `task-14.md:12-17` (files: `main.go` swaps the value, `git rm` the two interim files); `:81` `TestTheInterimMaterialiserIsGone`; `:90-95` (Steps 3 and 4); `:106-107` (acceptance `git grep`) |
| Restatements | `2026-09-23-phase-4-discover.md:318-320` (file map), `:345-348` (task 10 at 3.5 h, the one task over 3 h), `:361` (task table); `task-10.md:318-320` (Step 5: eleven files), `:348-349` (Does NOT claim) |

**One departure from the review's wording.** The interim returns `"none:declined"` when
`WithoutBrief` is set, not `""` in every case. `MaterialiseRequest.WithoutBrief`'s own doc
(`task-10.md:53-56` before this batch) says a Materialiser returns `none:declined`. An interim
that broke the interface would stamp a declined start as "no brief exists". No code at `2d6d95b`
writes a managed block, so there is nothing for it to strip.

**I2: a failed enumeration is `project_unknown`, and a clean miss stays `""`.** Fixed as the
review said, with two additions.

| How to fix | Where it changed |
|---|---|
| 1. `sessionLocate` gains an error return | `task-10.md:136-153` (signature, the three answers, `errProjectUnknown`; a worktree found in B despite A's `Worktrees()` failing is found); `:23-24` (files) |
| 2. `*MaterialiseError{project_unknown}`, the closed set, task 20's table | `task-10.md:100-104` (`BriefReasonProjectUnknown`, raised by the handler only); `:219-228` (create: the 409, `Materialise` not called); `task-20.md:26-28` (`BriefReason`), `:66` (the table row), `:81` (six rows), `:82` `an unknown project offers the override`; `task-22.md:24` (§3.2 names the set of six) |
| 3. `""` kept for a worktree outside every project | `task-10.md:63-67` (`ProjectID`'s doc); `task-09.md:74-80` (step 4: `ProjectID ""` inherits nothing, and `Source` is never asked); `task-09.md:163` `TestNoProjectInheritsNothingWithoutAskingTheHub` |
| 4. Test both paths | `task-10.md:285-293` (`sessions_test.go`: three `sessionLocate` tests, then the create refusal, the create override, the clean miss, and the restart re-locate); `:282-283` (`sessionsLocator.worktreesErr`) |
| Restatements | `2026-09-23-phase-4-discover.md:314` (file map), `:429-430` (seams: create and restart) |

The two additions:
- **The override does not refuse.** Under `withoutBrief: true` the create does not refuse an
  enumeration failure (`task-10.md:223-228`). Task 09 step 3 never asks the Hub, so no project is
  needed. Refusing would make *Start without the brief* a dead end, as N1 is on
  `tracked_elsewhere`. The price is a record with empty ids, which is what phase 2 records today
  for every enumeration failure. For the same reason task 20's `project_unknown` has no
  `withoutBrief: true` variant.
- **Restart re-locates an empty record.** `handleSessionRestart` uses the record's `ProjectID`,
  not `sessionLocate`. A `""` there is ambiguous: phase 2 recorded failures as `""`, and so does
  the override path above. So restart re-runs `sessionLocate` when the record's `ProjectID` is
  `""`, before the destroy, with the create's error rule (`task-10.md:246-252`,
  `TestRestartRelocatesARecordWithNoProject` `:293`). Without this, I2's silent non-inheritance
  would come back on every restart of such a session.

**A correction to the review.** Its fix 2 puts `project_unknown` in "the closed reason set
(task 09)". The closed set is declared in task 10's `materialiser.go` (`task-10.md:93-105`), so the
constant is there. Task 09 only maps causes onto that set, and it never returns
`project_unknown`: it now says so (`task-09.md:78-80`).

**Still unverified:**
- The handler-level tests assume the `sessions_test.go` env can create with the locator failing
  while `env.wt` still resolves the path. `sessionsLocator.err` also fails `Branches`. Whether
  anything else in create calls `Branches` was not read.
- The interim's Warn wording is a plan choice. No test pins more than "task 14".

### Fix batch b02, round 2 (2026-09-24): re-review I-1, M-1, M-2, M-3

Fixes the open issues of `.superpowers/sdd/p4/plan-rereview-b02-1.md` (round 1: I1 resolved, I2
resolved in design and text). Line numbers are as of the commit that adds this subsection. Round 1's
tables above cite `ac2e515`. In `task-10.md` everything after `:232` has moved: +1 line from `:233`,
+4 from `:252`, +5 from `:298`. So round 1's `:246-252` is now `:247-256`, `:285-293` is
`:289-297`, `:297` is `:302`, `:318-320` is `:323-325`, `:332-345` is `:337-350`, and `:347-349` is
`:352-354`. Round 1's `:219-228` and `:223-228` are unchanged.

| Re-review issue | Where it changed |
|---|---|
| **I-1** (Important): restart with `withoutBrief` and a failed enumeration was untested | `task-10.md:250-256` (the restart re-locate bullet now says in full that `errProjectUnknown` **with** `body.WithoutBrief` does not refuse: `Materialise` gets `ProjectID: ""` and `WithoutBrief: true`); `:298` new row `TestRestartWithoutBriefStartsDespiteAnUnknownProject` (seeded `ProjectID: ""`, `sessionsLocator.err`, `{"withoutBrief": true}` → 200, the fake gets `ProjectID: ""` and `WithoutBrief: true`, `none:declined` on the record; mutant: re-locate refuses whatever the override); `:297` `TestRestartRelocatesARecordWithNoProject` gains the optional clean-miss case (200, ids stay `""`, nothing refuses) |
| I-1 restatement | `2026-09-23-phase-4-discover.md:430` (restart seam: the re-locate runs under the create's rule, so it is `409 project_unknown` unless `withoutBrief`) |
| **M-1**: "three new ones" | `task-10.md:323` now "four new ones" (`materialiser.go`, `materialise_test.go`, `materialiser_interim.go`, `materialiser_interim_test.go`) |
| **M-2**: `config_sandbox_test.go:189` does not exist | `task-10.md:191` and this review's b02 section `:736` both now `:172`. Checked with `git show 2d6d95b:control-plane/cmd/api/config_sandbox_test.go`: 181 lines, `TestRunRefusesAMissingCABundleWithExitCode2` at `:172`, its `run(` at `:175` |
| **M-3**: the restart request list read `ProjectID: sess.ProjectID` unconditionally | `task-10.md:233-234` adds "(or the re-located ids, next bullet, when the record's `ProjectID` is `""`)" |

**Restatements checked.** `task-20.md:66` already says `project_unknown` is raised only without
`withoutBrief: true`, for create and restart alike, so it now matches the restart text. `task-20.md:82`
covers both verbs against a mocked client and needs no change. `task-22.md:24` and `task-09.md:79`
restate only the reason set, which this round does not change. A grep over the plan and its parts
finds no other copy of the three changed facts.

**Round 1's "still unverified" item 1 is closed.** The re-review verified it: create reaches
`Deps.Projects` only through `sessionLocate` (`sessions.go:545` at `2d6d95b`), and restart makes no
locator call today. So `sessionsLocator.err` failing all three methods is safe for the new restart
row too.

**Still unverified:** no Go was run, because this round changes plan documents only. Where the
re-locate lands in the restart body relative to the argv and `sessionEnv` rebuilds was not read. The
plan requires only that it comes before `Materialise`, and so before the destroy.

## Fix batch b03 (2026-09-24): I3, I7

Commit `0b7a436` on `feat/phase-4-discover` (tasks 09, 14, 15, 19 and the plan). Line numbers are
as of `0b7a436`; the commit that adds this section touches only this file. The review's cited lines
had moved before this batch: I3's `task-09.md:61-70` was `:62-88` and I7's `task-09.md:87-96` was
`:104-113`. The source citation was checked in this worktree: `ExecRunner.Run` is
`control-plane/internal/project/gitrunner.go:46-78`, and its failure return is
`fmt.Errorf("git %s in %s: %v: %s", …)` at `:74-75`. Phase 2's real-git helpers are
`requireGit` and `makeGitRepo` at `gitrunner_test.go:16-53`.

**I3: a tracked file is never stripped.** Fixed as the review said.

| How to fix | Where it changed |
|---|---|
| 1. Strip only candidates `Git.Tracked` reports untracked | `task-09.md:34-39` (the commit rule: a tracked file is never opened for writing, not even to strip); `:61-70` (step 2 asks `Tracked` of both candidates once and keeps the answers as the *strippable* set; any other `Git` error is a 500, never a guess at "untracked"); `:71-78` (step 3, `WithoutBrief`, strips only strippable candidates); `:99-106` (step 5, both branches) |
| 2. New `Inspect` state `committed` | `task-09.md:156` (`StateCommitted`); `:171-181` (`Inspect` doc: a tracked candidate holding a block, or a broken marker of one, gives `committed` with `Path` the tracked file; it wins over the target's own state); `task-15.md:75-77` (the desktop union gains `"committed"`) |
| 3. Shown in task 19 | `task-19.md:55-59` (`Materialisations` doc); `:80` (the state row: the path in mono, *Remove it in a commit*, no action control); `:102` `committed block names the tracked file and offers nothing` |
| 4. Materialise leaves it alone and still writes the untracked target | `task-09.md:99-106` (step 5: the target is written and the key returned; the tracked file is not opened) |
| 5. `TestNeverStripsATrackedFile` | `task-09.md:197` (four rows: inherit, nothing to inherit, `WithoutBrief`, and a tracked file whose markers are broken; each checks the tracked file is byte-identical and that `Inspect` says `committed`) |
| Restatements | `task-09.md:184-185` (the fake `Git` answers per path); `:196` `TestWithoutBriefStripsAndDeclines` (both candidates now stated untracked); `:208` `TestInspectStates` (eight states, and `committed` wins over `current`); `2026-09-23-phase-4-discover.md:172` (P4-D) |

Two choices the review did not spell out:
- **A committed block does not refuse a start.** Under `WithoutBrief`, and with something to
  inherit, the start goes ahead and the tracked file is left as it is (`task-09.md:71-78`,
  `:99-106`). The block is repository text that no Panacea write may remove (3-D6). Refusing
  would block every start in that repository, with or without the brief, until someone commits a
  removal. The cost is that the session reads both blocks. `committed` in task 19 says so. This is
  recorded for the owner.
- **A tracked file's broken markers are not `block_broken`.** The file is never opened, so only
  `Inspect` reads its markers, and it reports them as `committed` with `MarkerKey: ""`. Row 4 of
  `TestNeverStripsATrackedFile` pins this.

**Left for review N1.** Step 2 still refuses `tracked_elsewhere` before step 3 runs
(`task-09.md:61-70` says so). N1's first bullet, "under `WithoutBrief`, strip only untracked
candidates", is now true. Its remaining bullets, on step order and the both-tracked case, stay with
N1's own batch, because OQ-2 is open for the owner.

**I7: tracked-ness is read from output, not from an exit code.** Fixed as the review said.

| How to fix | Where it changed |
|---|---|
| 1. `git ls-files -- <rel>` without `--error-unmatch`: exit 0, empty output means untracked | `task-09.md:122-133` (the `Git` interface doc: no exit code is read, and why, citing `gitrunner.go:74`); `task-14.md:56-65` (`gitQuestions`: empty stdout is untracked, any output is tracked; any other error is returned) |
| 2. The `rev-parse` "not a git repository" string match is the only one left, and the plan says so | `task-09.md:122-133`; `task-14.md:56-65`. Both commands can fail that way outside a repository, so the one string is matched on either |
| 3. A real-git `ExecRunner` integration test in `t.TempDir()` | `task-14.md:84` `TestGitQuestionsOverARealRepository` (skipped without git, as `requireGit` is; config neutralised as `makeGitRepo` does; tracked, untracked, `.claude/CLAUDE.md`, `ExcludePath`, and a plain directory under `GIT_CEILING_DIRECTORIES` → `ErrNotARepo`) |
| 4. Task 14's "exit status 1" wording and `TestGitQuestionsReadsUntrackedAsFalse` | `task-14.md:56-65` (the wording is gone); `:83` (empty output → `(false, nil)`, a path → `(true, nil)`, the exact args with no `--error-unmatch`, the not-a-repo error, and any other error returned rather than read as untracked) |
| Guard | `task-14.md:114-115` (an acceptance `git grep` for `error-unmatch` and `ExitCode()` in non-test code under `cmd/api` and `contextwrite`) |
| Restatement | `2026-09-23-phase-4-discover.md:172` (P4-D) |

**Restatements checked.** A grep of the plan and its parts for "both candidate", "strip the
other", "error-unmatch", "exit status 1", "exit 1", "at most one managed" and "tracked file" finds
only the lines above. The last is OQ-2 (`2026-09-23-phase-4-discover.md:497`, "never edit a tracked
file"), which now matches. `task-12.md:40,76` passes `contextwrite.Inspection` through unchanged,
so it gains the state without an edit. `task-20.md:65` (`tracked_elsewhere`) is N1's.

**Still unverified:**
- No Go was run, because this batch changes plan documents only.
- `git ls-files -- CLAUDE.md` on a case-insensitive checkout where the index holds `claude.md` was
  not tried.
- Whether Claude Code reads both a tracked `CLAUDE.md` and `.claude/CLAUDE.md` in one session is
  OQ-2's canary (task 23). `committed`'s copy assumes that it does.

## Fix batch b03, round 2 (re-review `plan-rereview-b03-1.md`: R-I1, R-I2, R-M1, R-M2)

Plan edits are commit `4578faf`. Lines are at that commit. Round 1's rows above for "New
`Inspect` state `committed`" and "`TestInspectStates` (eight states)" are **superseded** by R-M1
below: there is no `StateCommitted` any more.

**Probes run** (git 2.53.0.windows.2, `GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM` = `/dev/null`,
scratch repositories under `$TEMP/.../p4/b03r2/`):

| # | Probe | Result |
|---|---|---|
| 1 | index holds `claude.md`, `core.ignorecase=true`: `ls-files -- CLAUDE.md` | exit 0, empty (the re-review's probe 3, reproduced) |
| 2 | same: `--icase-pathspecs ls-files -- CLAUDE.md` | exit 0, `claude.md` |
| 3 | same, with `GIT_LITERAL_PATHSPECS=1` | exit 128, `fatal: global 'literal' pathspec setting is incompatible with all other global pathspec settings` |
| 4 | same repository: `--icase-pathspecs ls-files -- .claude/CLAUDE.md` | exit 0, empty |
| 5 | untracked `CLAUDE.md`: `--icase-pathspecs ls-files -- CLAUDE.md` | exit 0, empty |
| 6 | the new guard over the two prescribed comment lines | exit 1, 0 lines |
| 7 | the new guard over `[]string{"ls-files", "--error-unmatch", rel}` and `ee.ExitCode() == 1` | exit 0, 2 lines |

**R-I1: a tracked `claude.md` now reads as tracked.** Fixed as the re-review said.

| How to fix | Where it changed |
|---|---|
| Ask `git --icase-pathspecs ls-files -- <rel>`, not `:(icase)`, and say why | `task-09.md:124-139` (the `Git` interface doc: the command, core.ignorecase, NTFS/APFS, `GIT_LITERAL_PATHSPECS`, the safe way on Linux); `task-14.md:56-70` (`gitQuestions`) |
| The exact-args assertion | `task-14.md:88` (args exactly `--icase-pathspecs ls-files -- CLAUDE.md`; no `--error-unmatch`, no `:(icase)`) |
| `TestGitQuestionsNeverWrites` reads the subcommand as the first non-option argument | `task-14.md:90` |
| A real-git row: the index holds `claude.md` → `Tracked(CLAUDE.md)` is `(true, nil)` | `task-14.md:89` (a third repository, `core.ignorecase` as `git init` sets it) |
| Restatement | `2026-09-23-phase-4-discover.md:172` (P4-D names the command and the case) |

One correction the re-review did not ask for: both parts wrote the commands as `git -C <dir> …`.
`ExecRunner.Run` passes no `-C`; it sets `cmd.Dir = dir` (`gitrunner.go:55`). `task-09.md:125-126`
now says so, `:127` and `:139` drop the `-C`, and `task-14.md:88` states the args carry no `-C`.

**R-I2: the guard no longer fails on the plan's own comments.** Fixed as the re-review said:
`task-14.md:119-123` matches `'"--error-unmatch"'` (the Go literal with its quotes) and
`'\.ExitCode()'`, and says why. Probes 6 and 7.

**R-M1: `committed` never hides the target's own state.** Fixed by the re-review's second option,
because one `State` for two files is the root cause.

| How to fix | Where it changed |
|---|---|
| `Inspection` keeps the target's own state; a separate list carries the committed block | `task-09.md:161-169` (seven states; `StateCommitted` removed); `:171-178` (`CommittedBlock{Path, MarkerKey}`, and why it is not a state); `:180-188` (`Inspection.Committed`, `[]` when none, `FileName` first); `:189-192` (at most one entry today, and why it is a list) |
| `Inspect` doc | `task-09.md:197-211` (the target is inspected for its own state always; every tracked candidate holding a block or marker is listed; both-tracked is `tracked_elsewhere` whatever the files hold) |
| Which file is `Path` when both are tracked and both hold blocks | `task-09.md:180-188`, `:207-211`. `Path` is always the target. With both tracked there is no target, so `Inspect` answers `tracked_elsewhere`; `Committed`'s order is pinned `FileName` first for N1's batch |
| Tests | `task-09.md:228` `TestNeverStripsATrackedFile` (`Inspect` gives the target's own state and one `Committed` entry); `:239` `TestInspectStates` (seven states, `Committed` `[]`); `:240` `TestInspectListsCommittedBesideTheTargetsState` (`current`, `edited`, `broken`, `superseded` beside one entry; a broken tracked marker → `MarkerKey ""`; both tracked → `tracked_elsewhere`) |
| Desktop type (in `api/discover.ts`, not the gated `store/types.ts`) | `task-15.md:75-78` (the union loses `"committed"`); `:146-152` (`CommittedBlock`, `WorktreeInspection.committed`) |
| Task 19 shows both | `task-19.md:54-62` (`Materialisations` doc); `:83` (the row sits below the target's own state); `:105` (the test now starts from a `current` inspection); `:106` `committed never hides the target's own state` (`edited` keeps its diff and warning; `broken` and `superseded` render as they do alone, compared by render because task 19 names no sentence for them) |
| Restatements | `task-09.md:39`, `:80`, `:107` (step 3, step 5 and the commit rule say "listed in `Committed`"); `2026-09-23-phase-4-discover.md:172` (P4-D) |

One behaviour changed that the re-review did not name. Round 1 answered `committed` for a
both-tracked repository whose files held a block. Now that case is `tracked_elsewhere` whatever
the files hold (`task-09.md:207-211`). No Panacea session can start there (N1), and
`tracked_elsewhere` names the cause a person acts on. N1's batch revisits it.

**R-M2: OQ-7 is in the owner's table.** `2026-09-23-phase-4-discover.md:502` asks whether a start
over a committed block is refused. It names the chip consequence (*declined* and *none* sessions
still read a committed brief; a *current* session reads a second, unstamped block), that no
`contextRef` records the block, the narrowed step 3 invariant, the recommendation (do not refuse;
show it) and the seam (one new reason from the marker read `Inspect` already makes). Pointers:
`task-09.md:77-80` (step 3) and P4-D (`2026-09-23-phase-4-discover.md:172`).

**Restatements checked.** A `git grep` of the plan and all parts for `StateCommitted`, "shown as
`committed`", "eight states", "committed wins", "seven states", `ls-files`, `git -C`, `icase`,
`OQ-6`, "open questions" and "narrowed" finds only the lines above. The "eight states" in
`task-16.md:99,181`, `task-17.md:21,145` and `task-22.md:50` are the provenance chip's, not the
worktree inspection's. `task-12.md:40` passes `contextwrite.Inspection` through, so it gains
`committed` without an edit.

**Still unverified:**
- No Go or TypeScript was compiled or run. This batch changes plan documents only.
- macOS was not probed. The APFS claim rests on APFS being case-insensitive by default.
- `--icase-pathspecs` was probed on git 2.53.0 only. The git versions on the sandbox image and on
  other developers' machines were not checked. It is an old option, but the version that added it
  was not looked up.

## Fix batch b04 (2026-09-24): I4, I13

Commits: `2282e4d` (the parts and the plan), then this section and task 12's rule 5.

**I4: every Discover call now has a budget, and each server budget sits below the desktop's.**
The ordering rule, stated in `task-02.md:40-41` and `task-11.md:105`: model budget < route
deadline < desktop `timeoutMs`.

| Call | Model budget | Server deadline | Desktop `timeoutMs` |
|---|---|---|---|
| Session start (`Materialise`'s Hub read) | — | `HubBudget` 5 s | 10 s (`DEFAULT_TIMEOUT_MS`, `client.ts:33`) |
| Every GET, and ops other than `propose_follow_up` | — | `Budgets.Default` 8 s | 10 s |
| `apply` with `propose_follow_up` | `FollowUpBudget` 20 s | `Budgets.FollowUp` 28 s | 30 s |
| `analyse` | `DraftBudget` 35 s | `Budgets.Analyse` 55 s | 60 s |
| modelclient HTTP ceiling | `DefaultTimeout` 45 s (was 120 s) | — | below 60 s |

| Fix | Where |
|---|---|
| 1. `Materialise` Hub read under 5 s; its expiry is `hub_unavailable` | `task-09.md:56-64` (`HubBudget`, `WithHubBudget`); `:107-115` (step 4); tests `:248` `TestABlackHoledHubIsARefusalWithinTheBudget`, `:249` `TestTheHubBudgetIsBelowTheDesktopSessionTimeout` |
| 2. The client passes `timeoutMs` per call | `task-15.md:24-28` (the transport's `send` gains `call?.timeoutMs`); `:186-194` (`DISCOVER_FOLLOW_UP_TIMEOUT_MS = 30_000`, `DISCOVER_ANALYSE_TIMEOUT_MS = 60_000`); tests `:205-206` |
| 3. Server model contexts shorter than the client's | `task-04.md:42-51` (`FollowUpBudget`), `:98` (step 2), test `:91`; `task-13.md:65-73` (`DraftBudget`), test `:115`; `task-02.md:34-41` (amendment 5), `:55` (`DefaultTimeout`), test `:125` |
| 4. A non-routable `PANACEA_APP_HUB_URL` step | `task-23.md:68-94` (new step 10: black-hole pre-check, restart by PID, four calls each timed against the desktop's `--max-time`); old steps 10 and 11 are now `:95` and `:97` |
| Beyond the review: route deadlines | `task-11.md:96` (error row), `:99-124` (`Budgets`), tests `:163-164`; `task-13.md:91-96` (analyse runs under `Budgets.Analyse`), test `:116`; `task-12.md:78-82` (rule 5), test `:104` |
| Hub ask | `2026-09-23-phase-4-discover.md:497` (**HA4-10**) |

**Where the review's fix was wrong on the facts, and what replaced it.**
- *Fix 3 alone does not keep the follow-up or the draft off `timeout`.* `Apply` reads the Hub
  before it calls the model (`task-08.md:76-83`, steps 2 and 3). A black-holed Hub holds that
  read for about 31 s (§9.2), longer than the 30 s client budget, whatever the model's context
  is. The root cause is that no route had a deadline. So every Discover route now runs under
  `Budgets`, and an expired route deadline during a Hub call is 503 `upstream_unavailable`,
  `upstream: "hub"`. The route classifies by its own `ctx.Err()`, not by the error type
  `hubclient` returns. `task-12.md` rule 5 keeps `refs` at 200 with `unavailable` slots, as its
  rule 1 already promised.
- *"`Materialise` runs under a 5 s context" is narrowed to its Hub read (step 4).* Steps 1-3 and
  5-7 are local git and disk. An expiry there would be misnamed `hub_unavailable`, which tells the
  person to fix the wrong thing. The caller's own cancellation stays a plain error.
- *`hubclient`'s `ctx` handling is not stated in §9.2* (`hub-contract.md:1337-1339` at hub
  `2f84662`, read with `git show`). If `hubclient` ignores `ctx`, no caller budget can cut a
  black-holed read short. That is Hub ask HA4-10, and task 23 step 10 is the live check.

**I13: the model endpoint is never defaulted.**

| Fix | Where |
|---|---|
| `DefaultBaseURL` deleted; `ErrNoBaseURL` | `task-02.md:23-33` (amendment 4, naming the old plan's `:299-302`, `:331`, `:389-391`); `:53` and `:59` (interface block); `:69` (`BaseURL` mandatory); test `:124` `TestAKeyWithoutABaseURLIsModelOff`; `:7`, `:133` ("five amendments"); `:19-20` (amendment 3 names `ErrNoBaseURL`) |
| A key without a base URL is model off, with a log line naming both variables | `task-14.md:24` (no default); `:81` (the fallback row and its exact `Warn` sentence, the key never logged); test `:87` |
| Security section sentence | `2026-09-23-phase-4-discover.md:266-275` (S5, which b01 reserved); `:207` (G4 points to S5) |
| Restatements | `task-04.md:30` (`AssistModelOff`); `task-13.md:86` (the `note` names both variables), `:114`; `task-18.md:90` (state 6's sentence names both variables) |

**Restatements checked.** A grep of the plan and all parts for `api.anthropic`, `DefaultBaseURL`,
`120 s`, `120 *`, "three amendments", `MODEL_API_KEY`, `MODEL_BASE_URL`, `20 s`, `10 s`,
`DEFAULT_TIMEOUT`, `timeoutMs`, `"timeout"`, "Step 10", "Step 11" and "task 23 step" finds only
the lines above. The remaining `api.anthropic.com` mentions are the deletion notes themselves.
Nothing outside task 23 cited its old step numbers 10 or 11.

**Declared, not the owner's.** The budget values (5, 8, 28, 55, 20, 35 and 45 s) are this batch's
choice, one per call. Each sits below the next layer and leaves room for the Hub work around a
model call. No owner question is raised for them.

**Still unverified:**
- No Go or TypeScript was compiled or run. This batch changes plan documents only.
- That `hubclient` honours `ctx` (HA4-10). That `10.255.255.1` black-holes on the corporate
  network: task 23 step 10 sub-step 1 checks it and falls back to `192.0.2.1`.
- That Phase 2's session start fits its non-Hub work into the 5 s `HubBudget` leaves inside the
  desktop's 10 s. Sandbox creation time was not measured in this batch.
- The follow-up budget assumes `Apply`'s Hub reads before the model take under 8 s on a healthy
  Hub. No latency figure for the Hub exists yet.

## Fix batch b04, round 2 (re-review `plan-rereview-b04-1.md`: R-I1, M1-M4, three nits)

Commits `1139097` (R-I1) and `b53297c` (M1-M4 and the nits), on `feat/phase-4-discover`. Parts
dir: `docs/superpowers/plans/2026-09-23-phase-4-discover-parts/`.

### Important

| Finding | Change | Where |
|---|---|---|
| R-I1 (1) HA4-10 rewritten against Hub task 33 | Framed as a change to a specified design, citing hub `2f84662` `task-33.md:29-31` (`Sleep func(d time.Duration)`, `time.Sleep`), `:52-53` (retries "a timeout", so the caller's own expiry), `:121` (the only timing test never cancels the caller), and `feat/hub` `33839b7` `:29-31`, `:56-57`. Three changes, most important first: (a) `context.WithTimeout(callerCtx, AttemptTimeout)`, with a test (100 ms caller deadline, a server that never answers, under 1.5 s); (b) no retry once `callerCtx.Err() != nil`; (c) `Sleep func(ctx, d) error`, with the 50 ms test | plan `:497` |
| R-I1 (2) budgets sized for the uninterruptible back-off | New paragraph "Back-off that cannot be interrupted": up to 1.25 s past a deadline; every budget at least 2 s under its desktop `timeoutMs` (8/10, 28/30, 55/60, `Materialise` 5 inside session start's 10), so at least 0.75 s remains; only (a) must land for I4 to hold | task-11 `:122-135` |
| R-I1 (2) restated at the Materialise step | step 4's comment: task 33 retries the caller's expiry and sleeps without `ctx`, about 6.25 s in all, inside 10 s | task-09 `:112-114` |
| R-I1 (3) each failure time has its own cause | Table gains a Budget column, and each Expected time is a range: 5 to 6.25 s, 8 to 9.25 s, 29.25 s at most, 56.25 s at most. New sub-step 4: at the budget means (a), (b) and (c) all hold. Up to 1.25 s past it means (a) holds and (b) or (c) is missing, which is a pass, recorded. Curl exit 28, or more than 1.25 s past, is a fail: rerun with `--max-time 70`, and about 31 s means (a) is missing. The old "within 1.5 s of `--max-time`" line is gone | task-23 `:84-104` |
| R-I1 (4) fallback declared | If the Hub stream refuses (a), `discoverhub` runs each Hub call in a goroutine and returns on `ctx.Done()`; the abandoned call finishes in the background (31 s at most). Built only on refusal | plan `:497` (last column); task-11 `:132-135` |

### Minor

| Finding | Change | Where |
|---|---|---|
| M1 the ceiling is not below every desktop budget | Amendment 5's heading now reads "below the longest desktop budget (analyse, 60 s)", and it states the ceiling is **not** below 30 s or 10 s. The const comment says the same. Test renamed `TestTheDefaultTimeoutIsBelowTheLongestDesktopBudget` (it still asserts `< 60 s`) | task-02 `:36-40`, `:59`, `:129` |
| M2 `ConfigFromEnv`'s doc comment | Added to amendment 4: old plan `:327-328` ("Only the key is mandatory.", verified) is rewritten to "the key and the base URL are both mandatory; neither is defaulted" | task-02 `:26-27` |
| M3 no fixture for the model-off wiring test | Asserted at a seam instead: `discoverWiring` gains an unexported `model modelclient.Completer`, an untyped nil when off. The test checks `w.model == nil` (a typed-nil `*Client` fails it), then sets the base URL to an `httptest` origin and sends one `w.model.Complete` there, carrying the key in `x-api-key` (old plan `:479`, verified). The route path is left to task 04's `TestNilCompleterIsModelOff` (`task-04.md:82`, verified) | task-14 `:39-45`, `:94` |
| M4 the follow-up accounting leaves out the write | `FollowUpBudget`'s comment: the 8 s covers Apply's reads before the model **and** its write after it (one `WriteRevisions`, at most 3 rebuilds, task-08 `:89-92`, verified). The write-then-503 ambiguity is named (a fresh `Idempotency-Key` on Retry meets `revision_conflict`), and so is the fact that the desktop giving up already had it | task-04 `:46-57` |

### Nits

| Nit | Change | Where |
|---|---|---|
| task-02 `:124` clause that cannot go red | "an `httptest` server counting requests sees none" dropped; the `ErrNoBaseURL` assertions kill the mutant | task-02 `:128` |
| task-15 per-call vs 0 | Ruled: a client-wide 0 wins (`client.ts:127` documents "0 disables"; `:256` arms only when `> 0`, both verified). `limit = timeoutMs > 0 ? (call?.timeoutMs ?? timeoutMs) : 0`. New test `a client built with timeoutMs 0 never times out a Discover call` | task-15 `:26-32`, `:211` |
| task-12 rule 5 omits `materialisations` | Stated: it takes task 11's route-deadline row, by `ctx.Err()`. The possible misnaming of an expiry during `Inspect`'s local reads is accepted, with the reason: those reads take milliseconds, so the budget can only run out in one after the `Source` reads have spent it | task-12 `:82-88` |

**Restatements checked.** Grep of the plan and every part for `HA4-10`, `honour`, `ignores \`ctx\``,
`back-off`, `about 5 s`/`8 s`/`28 s`/`55 s`, `1.5 s`, `31 s`, `BelowEveryDesktopBudget`,
`below every desktop`, `shorter than every`, `Only the key is mandatory`, `counting requests sees
none`, `Hub reads the other`, `call?.timeoutMs` and `timeoutMs: 0` finds only the lines above. The
older rows of this review (`:256`, `:987`, `:997`, `:1021`) are history and stay as written.
Task 13's `DraftBudget` comment already counts "Apply's Hub read and write" (`task-13.md:70-71`).

**Corrected unverified line (M4).** The follow-up budget assumes that `Apply`'s Hub reads before
the model and its write after it take under 8 s together on a healthy Hub.

**Still unverified:**
- No Go or TypeScript was compiled or run. This round changes plan documents only.
- R-I1 (a): whether the Hub stream accepts caller-derived attempts. Until then, the declared
  `discoverhub` fallback is the only thing that holds I4 against a hubclient built as task 33
  specifies.
- That `Inspect` calls `Source` for a project worktree. The plan infers it from task-09 `:254`
  ("`Inspect` on the same request never calls `Source` either" for `ProjectID ""`). The nit-3
  reasoning depends on it.
- Everything the round-1 list carries: `10.255.255.1` on the corporate network, the non-Hub
  session-start time, and Hub latency.

No owner question is raised.
