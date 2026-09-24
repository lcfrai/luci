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

## 4. After phase 2's fix rounds finish

Still to do for phase 2:
1. Rebuild the sandbox image `:dev` under the Docker lock.
2. Apply `task-25-26-reconcile.md` (items 1-12) to the task 25 and 26 plan parts.
3. Dispatch task 26 (the seams) and task 25's agent-provable steps. Its owner-observed steps are
   batched to the end (OD-12).
4. The final whole-branch review.
5. Create `main`, merge phase 2, then sync each stream branch from it.
