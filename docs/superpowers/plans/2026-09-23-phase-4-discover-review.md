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
