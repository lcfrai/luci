# RESUME — how the orchestrator picks up after a restart

Written 2026-09-24 17:35, just before a planned device shutdown. **Read this first on any restart.**
It is the fastest correct path back. Last time, rebuilding state from transcripts took 30 minutes.

## 0. The owner's command

```
cd C:\Users\au-dev-dmackie1\Projects\PanaceaApp
claude --resume 2ce0c1a9-63c3-42f2-bf4e-e6ba45d14fab
```

Then type: **Continue — read docs/superpowers/RESUME.md**.
(If that session id will not resume, run `claude --continue` in the same folder, or start a fresh
`claude` there and say the same thing. Step 3 covers the fresh-session case.)

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

## 4. After phase 2's fix rounds finish

Still to do for phase 2:
1. Rebuild the sandbox image `:dev` under the Docker lock.
2. Apply `task-25-26-reconcile.md` (items 1-12) to the task 25 and 26 plan parts.
3. Dispatch task 26 (the seams) and task 25's agent-provable steps. Its owner-observed steps are
   batched to the end (OD-12).
4. The final whole-branch review.
5. Create `main`, merge phase 2, then sync each stream branch from it.
