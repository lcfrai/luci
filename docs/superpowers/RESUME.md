# RESUME — how the orchestrator picks up after a restart

Written 2026-09-24 17:35, just before a planned device shutdown. **Read this first on any restart.**
It is the fastest correct path back. Last time, rebuilding state from transcripts took 30 minutes.

## 0. The owner's command

```
cd C:\Users\au-dev-dmackie1\Projects\PanaceaApp
claude --resume 56a311e8-629e-4437-9e39-3cb2f503917f
```

(The session before it was `2ce0c1a9-63c3-42f2-bf4e-e6ba45d14fab`, which ended at about 18:10 on
2026-09-27. A fresh session **cannot** `resumeFromRunId` another session's run, so every run id
below from that session is dead; section 0b names the live one.)

Then type: **Continue — read docs/superpowers/RESUME.md**.
(If that session id will not resume, run `claude --continue` in the same folder, or start a fresh
`claude` there and say the same thing. Step 3 covers the fresh-session case.)

## 0b. CURRENT at 2026-09-27 21:00 — one run, phase 2 close-out continuation

Everything else is still PAUSED by OD-31 until phase 2 is in the owner's hands.

| Run | scriptPath | args | What it does |
|---|---|---|---|
| `wf_a990237b-582` (task `wozma9i5q`) | the session's `workflows/scripts/phase2-closeout-continuation-wf_a990237b-582.js` | none | three file-disjoint chains in the main tree, then a gate |

Its chains, all on `feat/phase-2-cockpit` in the main tree, partitioned by file so they do not
collide (the partition is in the script's COMMON, rule 5):
- **Harness** — task 25 fix round 1 (review `task-25-review.md`: 0 C, 2 I, 8 M) then review-2;
  then task 26's residual I1 + m1–m5 (`task-26-review-3.md`, left OPEN after 3 rounds) then
  review-4. Owns `desktop/**` except `src/api/client.ts`, `docs/verification/**`, `docs/SEAMS.md`,
  task-25.md and task-26.md.
- **HostGit** — F2 + F4, then security X-3. Owns `control-plane/internal/project/**`.
- **Lifecycle** — F1 + F3. Owns `control-plane/internal/api/**`, `internal/reaper/**`,
  `sandboxd/**`, `desktop/src/api/client.ts`.
- **Gate** — after all three: the full Go suite, the desktop suite, typecheck, and a host-load
  sample, one at a time, nothing else running. Writes `SDD/closeout-gate.md` and rules on whether
  step 19 can be attempted.

**What is left after it**, in order:
1. **Step 19**, task 25's end-to-end run: once, on a quiet host, with Docker, a real control
   plane, Vite and the native window. It takes the machine over (a headed browser and a Tauri
   window), so it wants the owner not using it. The recipe is
   `docs/verification/2026-09-27-phase-2-results.md` § *Reproducing this*.
2. The results document rewritten from that run, then task 25's final review.
3. The **final whole-branch review** (the close-out script's last agent, never reached).
4. Create `main`, merge phase 2, then sync each stream branch from it.
5. Then restart at most 4 streams, per OD-31 (section 3c-00).

**State of the day's work, 2026-09-27.** Task 26: 3 rounds, left OPEN (0 C, 1 I, 5 M). Task 25:
implemented, its end-to-end run stopped at its first check (`setup`) on a host saturated by the
other streams, and its review is 0 C, 2 I, 8 M. Four product defects F1–F4 are recorded in the
results document with owners and were **not** fixed that day; the run above fixes them. The
18 NOT RUN checks are still the bulk of what phase 2 has not proven.

**Damage check done at 20:50, 2026-09-27:** Docker Desktop down, nothing listening on 8810–8815,
no orphan `go`/`api`/`node`/`python`, no task `.output` over 500 MB, C: 30 GB free. The Docker
lock in the old session's scratchpad was **stale and is released** — the first holder's stack
(PIDs 33984/34692, containers `s-6a855488`, `s-f44823fb`, `s-e19633bd`) no longer exists, so the
three records it asks someone to DELETE cannot be deleted and need not be.

## 1. What was running at shutdown (7 workflows)

| Stream | Worktree / branch | Last run id | Script | args |
|---|---|---|---|---|
| Phase 2 fix rounds (**continuation 3**, the OD-10 then fork review loops) | main tree, `feat/phase-2-cockpit` | `wf_07e1a9f6-d45` | the session's `workflows/scripts/phase2-fix-rounds-wf_f13e23da-5aa.js` (resume from that path; the `.superpowers` copy predates continuation 3) | none |
| p3 — git, worktrees, convergence | `.claude/worktrees/phase-3`, `feat/phase-3-git` | `wf_0feefc44-791` | `.superpowers/workflows/build-stream-current.js` | `{"stream":"p3"}` |
| env — phase 8, own environment | `.claude/worktrees/env`, `feat/env` | `wf_5c36d742-153` | `.superpowers/workflows/build-stream-as-launched.js` | `{"stream":"env"}` |
| hub — Phase H | `.claude/worktrees/hub`, `feat/hub` | `wf_b4bd0092-bcc` | `…/build-stream-as-launched.js` | `{"stream":"hub"}` |
| p5 — clean-room canvas | `.claude/worktrees/phase-5`, `feat/phase-5-canvas` | `wf_a97b9908-eab` | `…/build-stream-as-launched.js` | `{"stream":"p5"}` |
| p4 — Discover | `.claude/worktrees/phase-4`, `feat/phase-4-discover` | `wf_23f7be7c-382` | **`…/build-stream-p4.js`** (p4 built tasks 01-02 before the carry-index patch; this exact copy keeps its cache. Never resume p4 from the as-launched copy) | `{"stream":"p4"}` |
| p7 — Deploy connector | `.claude/worktrees/phase-7`, `feat/phase-7-deploy` | `wf_63db912c-42f` | `…/build-stream-as-launched.js` | `{"stream":"p7"}` |

Script paths are relative to the repo root: `C:\Users\au-dev-dmackie1\Projects\PanaceaApp\`.
`.superpowers/` is git-ignored and local, and survives a shutdown. Copies of the session's own
script files (same content) are under `~/.claude/projects/C--Users-au-dev-dmackie1-Projects-PanaceaApp/2ce0c1a9-63c3-42f2-bf4e-e6ba45d14fab/workflows/scripts/`.

**State at 17:35:**
- **Phase 2:**
  - approved: t23-r2, t24-r1, gitfix-r1, od9-toggle, resume-r1;
  - running: the reaper-scope review loop; then od10-forget; then fork-carries;
  - the main tree has uncommitted edits from the od10/fork agent, which will not compile mid-edit.
    That is expected; its successor finishes them.
- **Streams:** p4, env and hub have started building (task 01; env and hub were running
  carry-index first). p3, p5 and p7 were on their last plan-fix batches.

## 2. Resume procedure (same session)

1. **Check for damage first:**
   - `df -h /c`: keep more than 15 GB free.
   - `git -C <each worktree> status --short`: uncommitted files are the in-flight agents' WIP. Leave
     them; the resumed agent continues from them.
   - A stale Docker lock at `C:/Users/AU-DEV~1/AppData/Local/Temp/claude/C--Users-au-dev-dmackie1-Projects-PanaceaApp/2ce0c1a9-63c3-42f2-bf4e-e6ba45d14fab/scratchpad/docker-daemon.lock`
     is safe to remove if nothing listens on its port (`rm owner.txt && rmdir`).
   - Leftover `panacea-app.managed` containers: `docker ps -a --filter label=panacea-app.managed=true`.
2. **Resume every workflow with the Workflow tool**: `scriptPath` = the table's script (absolute
   path), `resumeFromRunId` = the table's run id, `args` = the table's args. Completed agents replay
   from cache, and only the in-flight agent re-runs.
3. **Verify the cache hit** a minute later: in `…/subagents/workflows/<run id>/journal.jsonl`, the
   last `"label"` entries should repeat the in-flight agent, not restart at the gate.
4. **If `resumeFromRunId` is refused** (a fresh session cannot resume another session's run), use
   the journals:
   - each `…/subagents/workflows/<run id>/journal.jsonl` maps each label to its result;
   - each stream's own ledger is `<worktree>/.superpowers/sdd/<stream>/progress.md`, and its batch,
     task report and review files sit beside it.
   Write a continuation that starts each stream at its first unfinished step. The build-stream
   script's gate and triage are cheap to re-run; the triage result lists the batches and tasks.

## 3. Hard-won rules (do not relearn them)

- **Never SendMessage a Workflow agent.** It spawns a duplicate, and TaskStop then kills both.
- **Never use `isolation: 'worktree'`.** It blocks claudezsh. Worktrees are created by hand under
  `.claude/worktrees/`.
- **A workflow with parallel chains cannot be resumed by prefix.** Write a continuation instead.
- **If you edit a script after launch, resume from the as-launched copy**, or the cache misses.
- **Plan re-reviews do not converge.** The script carries residual Important and Minor findings into
  the build. Only a Critical earns round 3.
- **Every agent: no Write or Edit over about 80 lines, first write early, no heredoc through
  claudezsh**, and read only your own `carried/task-<id>.md`.
- **Before every push:** rescan all branches for secrets and blobs over 5 MB, then push with
  explicit refspecs. **The scan must gate the push.** Run it in batches
  (`git rev-list <branches> | xargs -n 150 git grep -I -l -E '<patterns>'`, where exit 123 means
  clean): at about 900 commits, a single `git grep … $(git rev-list …)` fails with "Argument list
  too long", and a chained push then runs unscanned. That happened on 2026-09-24; the batched
  rescan afterwards found 0 hits in 927 commits. Origin is the PRIVATE `github.com/lcfrai/panaceaapp`, and the PAT is read by a
  repo-local helper from `~/.secrets/panaceaapp-github.pat`. Never print it.
- The owner's decisions are OD-1 to OD-13 in `docs/superpowers/planning/2026-09-23-owner-decisions.md`.
  The IT request for Entra and the new environment is in `docs/superpowers/planning/2026-09-24-it-request-entra.md`.
- The phase 2 ledger is `.superpowers/sdd/2026-09-18-phase-2-backend-and-cockpit/progress.md`, over
  6,500 lines, so grep it. Its tail is the latest state.

## 3b. Queued the moment phase 2's continuation 3 finishes (21:35, 2026-09-24): SECURITY

**X-1 primary half, a Critical in merged phase 2 code** (found by phase 3's plan re-review b02-3;
phase 3 ruling R4 rule 6). `sessionGitGlobalFile` (`control-plane/internal/api/sessions.go`, about
479-505, commit `d7e2dea`) mounts, read-only into the sandbox, **any absolute regular host file**
that `core.excludesFile` or `core.attributesFile` names. It reads those keys with host git **in the
worktree**, whose `.git/config` the agent can write. So an agent sets
`core.excludesFile=C:/Users/<u>/.ssh/id_rsa`, and the next create or restart mounts that file into
the sandbox.
- **The fix is the root cause:** the host-side resolvers (global files, line endings, identity)
  read only host-owned config: `--global`/`--system`, run with a working directory outside any
  repository, never `-C <worktree>`.
- **Tests, red first:** a repo-local `core.excludesFile` pointing at a secret is NOT mounted; a
  global one still is; a repo-local `core.autocrlf` does not change what the sandbox gets.
- Amend the Go contract. Then run a review, then fix and re-review rounds.
- Run it in the main tree, AFTER continuation 3, so it does not collide in `sessions.go`.

**Phase 3** is paused for ruling R4 (`.claude/worktrees/phase-3/.superpowers/sdd/p3/orchestrator-rulings.md`),
which workflow `wf_edf4498c-78e` (p3-ruling-r4) applies. When its re-review passes, resume p3's
build stream with `args {"stream":"p3","cleared":["b02"]}`.

**Also running (21:45):**
- **Team-collaboration research**, run `wf_939b83b5-17a`, task `w1k373knn`. It is the session's
  `workflows/scripts/team-collab-research-wf_939b83b5-17a.js`, with no args, and resumes by prefix
  only within the same session. Its output is `docs/superpowers/planning/2026-09-24-team-collaboration/`:
  00-summary, 01-04 research, 05-design, 06-07 critiques. The owner asked for multi-user projects:
  a team sees each member's work, agents and focus, and spins up its own agents that know what
  every other agent is doing.
- **p3-ruling-r4**, run `wf_edf4498c-78e`.
- **Linear parity-plus research, design and prototype**, run `wf_a368c5a8-fc0` (the session's
  `workflows/scripts/linear-parity-plus-wf_a368c5a8-fc0.js`). Its output is
  `docs/superpowers/planning/2026-09-25-linear-parity/` (00-summary, 01-04 research, 05-design,
  06-07 critiques) and `prototype/work-layer.html`. The owner's ask: "everything Linear does and
  more … prettier, better", plus the agent and provenance edge.
- The team-collab research resumed as task `whrp1f8cb` (same run `wf_939b83b5-17a`), with the
  OD-14 to OD-20 DIRECTION in its design, critique and revise prompts.
- Owner decisions now run OD-1 to OD-20. OD-18: Azure. OD-19: org ZDR inference via a gateway
  (Global Anthropic by default, AU Bedrock on trigger). OD-20: local for now, cloud-first.

**Added 08:30, 2026-09-25:**
- **Phase 2 close-out**, run `wf_33aecd88-cd3` (the session's `workflows/scripts/phase2-closeout-wf_33aecd88-cd3.js`). It is sequential, so it resumes by prefix. It does the X-1 security fix, then rebuilds `:dev` and reconciles tasks 25 and 26, then task 26, then task 25's agent-provable steps (the owner checklist is batched), then the final whole-branch review. The phase 2 fix rounds (continuation 3) are COMPLETE: od10 and fork are approved.
- **p3 R4 round 4**, run `wf_d53f7dc5-a3d` (the session's `workflows/scripts/p3-r4-round4-wf_d53f7dc5-a3d.js`). When its review passes, resume p3's build stream (run `wf_0feefc44-791`, `build-stream-current.js`) with args `{"stream":"p3","cleared":["b02"]}`.
- Team-collab research: COMPLETE (`00-summary.md`). Linear parity-plus: COMPLETE (`00-summary.md`, `prototype/work-layer.html`).
- Owner decisions now run OD-1 to OD-24. OD-21: LiteLLM. OD-22: read-only fetch. OD-23: a policy injection mechanism. OD-24: the data class is set at creation and drives inference and deploy scans.

**Added 09:00, 2026-09-25: three new streams** (OD-21 to OD-28). The worktrees are
`.claude/worktrees/{phase-6,team,work}`, on branches `feat/phase-6-gateway`, `feat/team-mode` and
`feat/work-layer`.
- **Plans:** `wave-c-plans`, run `wf_33255dc5-1ac`: skeleton, then parallel part writers, then an
  adversarial review, for each stream. It has **parallel chains, so do not resume it by prefix**
  after an interruption. Read its journal and write a continuation per stream.
- **Build streams**, all on the session's `build-stream-wf_56772bc5-8a7.js` (copy:
  `.superpowers/workflows/build-stream-current.js`). Their gates wait for the plan reviews:
  - p6: run `wf_3628332d-1ab`, args `{"stream":"p6"}`;
  - team: run `wf_e90a5a3f-ad5`, args `{"stream":"team"}`;
  - work: run `wf_d0a82d09-d10`, args `{"stream":"work"}`.

**Added 10:00, 2026-09-25: after the second spend-limit stop (09:24).** Ten runs were resumed with
`resumeFromRunId` and keep their run IDs: close-out, p3 R4 round 4, hub, p4, p5, p7, env and p6's
build stream. `wave-c-plans` (`wf_33255dc5-1ac`) is **retired**. Only p6's skeleton had finished.
The team and work skeleton writers were killed by the watchdog while composing a very large return
(a scope for every task), so the new scripts return task ids only. It is replaced by:
- **p6 plan continuation**, run `wf_7bc1a1b9-064`: `.superpowers/workflows/wave-c-p6-continue.js`,
  no args. It writes the 25 part files, then the review.
- **team plan parts**, run `wf_2b949030-b55`: `.superpowers/workflows/wave-c-plan-parts.js`, args
  `{"stream":"team"}`. It writes the 69 part files in sections of 10. Each section gets its own
  reviewer, then one cross-cutting reviewer, then an assembler writes `…-review.md`.
- **work plan parts**, run `wf_90d7f9c9-ec9`: the same script, args `{"stream":"work"}`. It first
  finishes the skeleton (the W5 rows), then writes the scopes in `…-scopes/`, then the coverage
  map `…-scopes/coverage.md`, then the parts and the reviews as for team.
- Every agent in these two scripts is restart-safe: it finishes a file and never rewrites it. So
  resume them with `resumeFromRunId`. A cache miss re-runs safely.
- The section reviews live in `<worktree>/.superpowers/sdd/<stream>/plan-review-sections/`, which
  is git-ignored. `.superpowers/sdd/.gitignore` (`*`) was added to `team` and `phase-6`, because the
  gate needs a clean `git status`.
- **The team and work build streams are STOPPED** (`wf_e90a5a3f-ad5`, `wf_d0a82d09-d10`). Their gate
  gives up after 5 hours, and their plans take longer (OD-29, all part files now). When
  `…-phase-t-team-mode-review.md` (or `…-phase-w-work-layer-review.md`) is committed, launch that
  build stream **FRESH**: no `resumeFromRunId`, because a cached gate result of `ready=false` would
  replay. Use the session's `build-stream-wf_56772bc5-8a7.js` with `{"stream":"team"}` or
  `{"stream":"work"}`.

**Added 12:50, 2026-09-25:**
- **p6 plan: COMPLETE.** Its review is Needs fixes: 1 Critical (C-1: LiteLLM body fields could send
  AU traffic to Global) and 10 Important. p6's build stream (`wf_3628332d-1ab`) runs the fix rounds
  once its gate sees the committed review.
- **p3 R4: APPROVED** (r4-review-5: 0 Critical, 0 Important, 3 Minor). p3's build stream is resumed
  (`wf_0feefc44-791`, `build-stream-current.js`, `{"stream":"p3","cleared":["b02"]}`). Two side
  agents (not workflows; if they die, dispatch again):
  - one applies M-1 to M-3 to `phase-3 … task-24.md`;
  - one restates the env plan's P8-D7 to match R4 (Q18).
  Q19 is fixed in `orchestrator-rulings.md` R4.6. G5 (X-1 is not on `feat/phase-3-git`) closes at
  the phase 2 sync. `4948320` on phase 3 has another stream's message (a message-file collision).
  It stays unamended.
- **Team plan repair.** At 11:40 a transient "Not logged in" killed the writers of team tasks 01-05
  and 41-45. Tasks 04, 05 and 41-45 are missing, and task 03 is cut off. Stage 1 is run
  `wf_5c66b9e9-0e9` (`.superpowers/workflows/wave-c-team-repair.js`, `{"stream":"team"}`).
  **Stage 2, after the main team run (`wf_2b949030-b55`) finishes:** re-review sections 1 (01-10)
  and 5 (41-50). Their section files will flag the missing parts. Then re-run the cross-cutting
  review and the assembler. Launch the team build stream only after that.
  **Stage 1 is DONE** (all of 01-05 and 41-45 exist). Its writers' 13 gaps are in
  `team/.superpowers/sdd/team/repair-gaps.md`, and stage 2's reviewers must read that file.

**Added 20:05, 2026-09-25: a third spend stop (19:31), then a forced resume.** All 12 runs were
stopped and resumed with `resumeFromRunId`. **Parallel runs resume from cache now** (keys are
`v2:<hash>`; verified on env-r4). There is no need for continuations.
- `a1e2c46` on `feat/env` is a SALVAGE commit. It holds the env-r4 first attempt's work on tasks 19,
  20 and 25, which its retries had refused to touch.
- 17:58: a runaway `python -` REPL (PID 23092) had filled C: with a 38 GB task log. It is killed and
  deleted, and disk is back to 47 GB. The watcher now flags any task `.output` over 500 MB.

**Added 21:50, 2026-09-25: a fourth spend stop (21:17), and a correction.**
- Resuming does NOT work for `wave-c-plan-parts`: its nested parallel steps miss the cache, so each
  resume re-ran finished writers and reviews. The team and work runs (`wf_2b949030-b55`,
  `wf_90d7f9c9-ec9`) are RETIRED.
- They are replaced by `.superpowers/workflows/wave-c-plan-finish.js`, which is idempotent. A
  state agent reads the disk, then only missing parts and unreviewed sections are dispatched,
  followed by the cross-cutting review and the assembler. The assembler includes
  `sec-90-identity.md`, and for team it runs the stage-2 repair re-reviews of sections 1 and 5.
  - team: `wf_1bf44b8c-f6f`, `{"stream":"team"}`.
  - work: `wf_3c2c558c-b5b`, `{"stream":"work"}`.
  - **After any stop, launch it FRESH with the same args. Do not resume it.**
- SALVAGE commits `aab769f` (main) and `dd7cb16` (env) hold OD-30 fix-1's stranded edits.

**Added 22:30, 2026-09-25: cross-stream order fixes.**
- **env R4 re-plan: DONE.** Review 1's 4 Criticals are fixed. Review 2 left 2 Importants, which the
  seams run below fixes.
- **p3 was missing task 24** (the sandboxd git runner, added by R4 after p3's triage ran). Tasks 07,
  08, 09, 11 and 16 depend on it. `build-stream-current.js` now takes `args.insertBefore`, and p3 is
  resumed with `{"stream":"p3","cleared":["b02"],"insertBefore":{"07":[{"id":"24",…}]}}`. **Every p3
  resume must pass those args** (copy them from task `wlt02zfzi`'s launch).
- **Build streams can now hold tasks.** Both `build-stream-as-launched.js` and
  `build-stream-current.js` take `args.hold` (a list of ids). Held tasks are skipped and logged, and
  kept out of `done`, so the cache still holds. They are built later by a fresh run.
- **Run `wf_e83c0311-995`** (`.superpowers/workflows/env-p3-seams.js`):
  - it files phase 8's asks S8-P3.3, S8-P3.4 and S8-P3.5 into the phase 3 plan, task 02 first
    because p3 reaches it soon;
  - it fixes env review-2's I1, I2 and M1 to M5;
  - it returns env's **held set**, recorded in `env/.superpowers/sdd/env/held.md`.
  **Then restart the env stream with `{"stream":"env","hold":[<held ids>]}`**, before env reaches
  task 20 (position 17; env is on task 02).

**Added 23:40, 2026-09-25:**
- **Hub stream STOPPED at task 04**: not approved after 2 fix rounds (0 Critical, 1 Important, a
  test-coverage gap in the fake's org-admin rule). Tasks 01-03 are approved; 41 remain.
  - `build-stream-as-launched.js` now takes `args.clearedTasks`.
  - `.superpowers/workflows/task-round3.js` is an orchestrator round 3 for one task: a fix, then a
    scoped re-review. Its run is `wf_884200d0-8ba`, with
    `{"stream":"hub","dir":"hub","task":"04","review":"…/task-04-review-2.md"}`.
  - **When its review-3 is Approved, resume hub:** `resumeFromRunId: wf_b4bd0092-bcc` with
    `{"stream":"hub","clearedTasks":["04"]}`.
- p7 stopped on a sign-out and is resumed as task `wcbdm4rcg`.
- Owner questions (4 from phase 7, 5 from the Hub) are collected in
  `docs/superpowers/planning/2026-09-25-owner-questions-pending.md` for the next question round.

**Added 00:30, 2026-09-26: a sign-out killed agents in 5 runs. All are resumed.**
- The runs: identity, p4, env, close-out, and team finisher (fresh run `wf_df8d0e15-93e`).
- The work finisher (`wf_3c2c558c-b5b`) lost writers for 149-168. **Launch it FRESH once more after
  it finishes**, and it writes only what is missing.
- **X-1 is APPROVED** (x1-global-files-review-2). Close-out continues with the reconcile, 26, 25
  and the final review.
- **QUEUED, SECURITY, before phase 2 merges to `main`: X-3**, from the X-1 review, with no owner
  yet.
  - **O-1 (Critical-class):** the rail scanner runs `git worktree list --porcelain` in the
    agent-writable primary worktree (`internal/project/project.go:282`). An agent-written
    `.git/commondir` lists another repository's worktrees, and a create mounts one read-write. The
    cheapest guard is to confine `Scanner.Path` results to ProjectsDir after EvalSymlinks.
  - **m-5:** a host-owned `core.excludesFile` can name a file inside a project, which an agent can
    swap for a link to a secret. Under R4's rule, no bind-mount source may be a path a sandbox can
    write.
  - **O-2.**
  - Run as: a ruling, then a fix, then an adversarial review, AFTER close-out's final review; fold
    in its findings too.

**Added 01:30, 2026-09-26:**
- **Team plan: COMPLETE.** It has 69 parts, and its review is committed:
  `…-phase-t-team-mode-review.md`, Needs fixes, 18 C / 153 I / 134 M.
  - At least S01-C1 is stale ("tasks 04 and 05 missing"). The section 1 reviewer appended to an
    interrupted pre-repair pass.
  - **`build-stream-v2.js`** (for team and work, which launch fresh) adds `fixOnly:'critical'`. Only
    Criticals are batched, and the review is carried, so implementers grep it for their own task.
    It also adds `planOnly`, which stops after the plan fix.
  - Team run: `{"stream":"team","fixOnly":"critical","planOnly":true}`. **To build later, resume
    it with the same args minus `planOnly`**, once its sibling code (the Hub, phase 8, phase 3) has
    landed; most team tasks depend on it.
- **OD-30 identity: review-2 is 0 C, 2 I**, and round 3 is running (`wf_c1460e29-6b6`).

**Added 05:40, 2026-09-26:**
- **OD-30 identity is APPROVED** (review-3: 0 C, 0 I, 2 M). M1 is fixed as `3df9fab8` on `feat/hub`
  (ADR-H1:809). M2 is carried to env task 31 (`env/.superpowers/sdd/env/carried/task-31.md`): the
  realm runtime checks, plus a first-time-user sign-in (U-16).
- For the owner, later: the ruling's section 10 checks and the Panacea asks K1-K5 are
  owner-run and batched (OD-12).
- A fifth spend stop ran from 02:15 to 05:20. Every run restarted **on its own** when usage reset,
  so nothing was forced.

**Added 2026-09-26, later: the org's WEEKLY limit (reset 30 Sep) ended every workflow.** The
runtime called that reset "too far out to wait for" and failed all in-flight agents. After the
owner added usage, all 11 were relaunched. The new task ids:
- **env** `w5ac7lsk5`, with
  `{"stream":"env","hold":["21","24","25","26","32","33"],"insertBefore":{"30":[24a],"08":[27a]}}`.
  - The cached triage predates the re-plan, so its "24" names a deleted part (it is now 24a and
    24b), and it has no 27a.
  - `build-stream-as-launched.js` now takes `insertBefore` too.
  - **Every env resume must pass these args** (copy them from `w5ac7lsk5`'s launch).
  - The held set is in `env/.superpowers/sdd/env/held.md`: 21, 21b, 24b, 25, 25b, 26, 32, 33.
  - Re-admit them once phase 3's code is sync-merged into `feat/env`. 21b, 24b and 25b need
    `insertBefore` too.
- **p3** `whgfigd7k` (same insert args). **p4** `w823a4wvg`. **p5** `wq7y2nlcm`. **p6** `w06fg5ozf`.
  **p7** `wo9w3grgt`.
- **close-out** `wevstu56b`: task 26, then 25, then the final review.
- **hub t04 round 3: APPROVED** (review-3: 0 C, 0 I, 2 M). Its Minors are carried to
  `hub/.superpowers/sdd/hub/carried/task-05.md`. The hub stream is resumed as task `wwz370lue`,
  with `{"stream":"hub","clearedTasks":["04"]}`, and is now on task 05. **Every hub resume must
  pass `clearedTasks:["04"]`.**
- **env-p3-seams: APPROVED** (seams-review-2: 0 C, 0 I, 4 M). Phase 3's plan now carries S8-P3.6
  MachineFS (task 02), S8-P3.4 Bundles and the S8-P3.5 pool (task 24). The held set is confirmed.
  The Minors, and a seam gap (`GIT_CONFIG_KEY_<n>` for `transfer.fsckObjects`, which task 21's
  Env allowlist refuses), are carried to env tasks 21, 21b, 24b, 25 and 25b.
- **team v2** `wndrouo9x`: 8 Critical batches.
- **work finisher** `wf_591bc2c2-c47` (fresh): 132 parts, 18 section reviews, then the
  cross-cutting review and assembly.

**Added 13:40, 2026-09-25: OD-30 (sign-in) and the env R4 re-plan.**
- **OD-30:** sign-in goes through a Keycloak broker on Panacea's existing Entra registration, with
  zero clicks. The IT request's Part 1 is no longer needed. Run `wf_579f2bb3-bfb`
  (`.superpowers/workflows/identity-keycloak.js`, no args) does the work in order: facts, the ruling
  `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md`, then edits the hub, env,
  p6, p7 and p4 plans and the main documents, then reviews.
  - The team and work plans get the change as findings, in `sec-90-identity.md` in their review
    sections folder. **Both assemblers must include `sec-90`**: check after the work run finishes,
    and put it in team's stage 2.
  - Urgent: the hub build reaches tasks 10 and 11 (token validation) in about 5 tasks.
  - env task 16 (`internal/entra`) is BUILT, so its rework folds into an unbuilt task.
- **env R4 re-plan:** run `wf_bb6abd47-038` (`.superpowers/workflows/env-r4-replan.js`). A side
  agent gated env tasks 20, 21, 24b and 25, because they were planned on the retired git design, and
  a gated part stops the env stream. This run decides the five open items, re-plans the four parts
  and removes the gates. It must land before the env build reaches task 20, about 12 tasks away.

## 3c-00. OD-31 AMENDED (19:50, 2026-09-26): STREAMLINED, AT MOST 4 STREAMS, NO OWNER TEST PAUSE

- **Running (4):** close-out (`wf30babpy`); hub (`we20zj19k`, args
  `{"stream":"hub","clearedTasks":["04"],"liteFrom":"05"}`); p3 (`wdcqplawr`, its insert args plus
  `"liteFrom":"02"`); p4 (`w1gu0q50d`, `{"stream":"p4","liteFrom":"t10"}`).
- **Queue, in order,** each started when a slot frees:
  1. p7 `{"stream":"p7","liteFrom":"t05"}`
  2. p5 `{"stream":"p5","liteFrom":"t05"}`
  3. env: its hold and insert args plus `"liteFrom":"03"`
  4. p6 `{"stream":"p6","liteFrom":"01"}`
  5. team plan fix, then its build
  6. the work plan (resume its writing only after the others; lighter parts)
  7. After close-out: X-3, then merge phase 2 to `main`.
- **Lite mode** (`args.liteFrom`, in all five build scripts) adds instructions only to the
  implement, review and fix prompts from that task on:
  - mutation tests only for security-sensitive code;
  - only the changed packages' tests;
  - one fix round, a second only for a Critical;
  - residual Importants carried, not re-looped.
  Tasks already run keep their prompts, so the cache holds.
- **Every resume must pass `liteFrom`** as well as the stream's other args.
- **p4 (21:00): 7 of 23 approved.** It stopped because t07 was BLOCKED: it needs the Hub H0 code
  (hub tasks 01-04), which lives only on `feat/hub`.
  - Run `wf_82a8c5f7-370` (`p4-sync-hub`) merges `feat/phase-2-cockpit`, then feat/hub at its last
    pre-task-05 commit, into p4, and resolves the 5 `internal/api` conflicts.
  - **Then resume p4 with**
    `{"stream":"p4","liteFrom":"t10","retryNote":{"t07":"The orchestrator merged phase 2 and the Hub H0 code into this branch; H0 is present now."}}`.
  - `args.retryNote` (in all five scripts) changes only that task's implement prompt, so its
    cached BLOCKED result is not replayed.
- **The same Hub dependency will hit p5, p7, team and work:** before each one reaches a
  Hub-consuming task, sync-merge an approved `feat/hub` commit into it.

## 3c-0. OD-31 (19:30, 2026-09-26): ONLY THE PHASE 2 CLOSE-OUT RUNS (superseded by 3c-00)

Every other stream is **PAUSED** by the owner's decision (OD-31) until phase 2 is merged to `main`
and in the owner's hands. The close-out's remaining work is task 26, task 25, the final review, then
X-3 (section 3b's queue). After that:
- restart at most 4 streams from the table below;
- use a lighter build script: 1 review round, and mutation tests only for security code;
- keep the work plan's writing paused until the owner says otherwise.

Close-out is task `wf30babpy` (run `wf_33aecd88-cd3`).

## 3c. RESUME-ALL SET (current at 19:05, 2026-09-26), used AFTER phase 2 lands (OD-31)

After a spend or usage stop, check the transcripts (`scratchpad/wfstatus6.py`). If they are more
than about 15 minutes old once usage is back, TaskStop every run, wait about 25 s, then call
`Workflow` with each row below. `WS` means `.superpowers/workflows/`, and `SS` means the session's
`workflows/scripts/`.

| Stream | resumeFromRunId | scriptPath | args |
|---|---|---|---|
| close-out | `wf_33aecd88-cd3` | SS `phase2-closeout-wf_33aecd88-cd3.js` | none |
| hub | `wf_b4bd0092-bcc` | WS `build-stream-as-launched.js` | `{"stream":"hub","clearedTasks":["04"]}` |
| env | `wf_5c36d742-153` | WS `build-stream-as-launched.js` | `{"stream":"env","hold":["21","24","25","26","32","33"],"insertBefore":{"30":[24a],"08":[27a]}}`, with the full objects as in section 3b's weekly-limit entry |
| p3 | `wf_0feefc44-791` | WS `build-stream-current.js` | `{"stream":"p3","cleared":["b02"],"insertBefore":{"07":[task 24 object]}}` |
| p4 | `wf_23f7be7c-382` | WS `build-stream-p4.js` | `{"stream":"p4"}` |
| p5 | `wf_a97b9908-eab` | WS `build-stream-as-launched.js` | `{"stream":"p5"}` |
| p6 | `wf_3628332d-1ab` | SS `build-stream-wf_56772bc5-8a7.js` | `{"stream":"p6"}` |
| p7 | `wf_63db912c-42f` | WS `build-stream-as-launched.js` | `{"stream":"p7"}` |
| team (plan fix) | `wf_0404c9cf-002` | WS `build-stream-v2.js` | `{"stream":"team","fixOnly":"critical","planOnly":true}` |
| work (plan) | `wf_591bc2c2-c47`, or fresh | WS `wave-c-plan-finish.js` | `{"stream":"work"}` |

## 4. After phase 2's fix rounds finish

Still to do for phase 2:
1. Rebuild the sandbox image `:dev` under the Docker lock.
2. Apply `task-25-26-reconcile.md` (items 1-12) to the task 25 and 26 plan parts.
3. Dispatch task 26 (the seams) and task 25's agent-provable steps. Its owner-observed steps are
   batched to the end (OD-12).
4. The final whole-branch review.
5. Create `main`, merge phase 2, then sync each stream branch from it.
