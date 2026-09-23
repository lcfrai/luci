### Task 23: Proof — the end-to-end run on real git and Docker, the owner's batched checklist, and the seams anchored

Every earlier task proves its part in isolation. This task runs the whole of phase 3 through the
**real control plane binary**, real git on the host, and real sandboxes in Docker, exactly as a
user would drive it, and writes down what happened. It then assembles the owner's one hands-on
session (OD-12: batched at the very end; until then every owner-observed item is **unverified**),
and anchors each seam phase 3 leaves to a file and line that now exist.

**How the run acts as an agent without being one.** The run starts **shell** sessions (no sign-in
needed) on each worktree and makes its commits with `docker exec -u dev <sandbox> git …` inside
those sandboxes. `docker exec` inherits the container's environment, so every commit runs through
the same `GIT_CONFIG_*`, identity and `PANACEA_APP_SESSION_ID` an agent's would, and the hook
stamps it. The run never reads PTY output to decide anything; it asserts on the HTTP API and on
git.

**Files:**
- `control-plane/e2e/phase3_test.go` — `//go:build e2e` (new)
- `docs/verification/2026-09-2x-phase-3-results.md` — the run's record, the checklist, the seams (new)
- `docs/PROGRAM.md` — phase 3's status line (p2, one line)

#### The run — `TestPhase3EndToEnd`

It builds `cmd/api` into a temp directory, sets `PANACEA_APP_STATE_DIR`, `PANACEA_APP_PROJECTS_DIR`
and `PANACEA_APP_WORKTREES_DIR` to temp directories (the worktrees one **outside** the projects
one), a fixed `PANACEA_APP_GIT_NAME`/`EMAIL`, `PANACEA_APP_LANE_SYNC_INTERVAL=1s`,
`PANACEA_APP_FETCH_INTERVAL=0`, starts it on a free loopback port, and waits for `/v1/healthz`.
It creates a project repository with a bare repository as its `origin`, in husky 9's shape
(review I11; re-review b03 Minor 2): the project's **local** config has `core.hooksPath=.husky/_`;
the branch tracks `.husky/commit-msg` (appends `X-Husky: ran`) and `.husky/reference-transaction`;
and the working tree has an ignored `.husky/_/` holding `.gitignore` (`*`), a stub per name
(`commit-msg`, `reference-transaction`: the one line `. "${0%/*}/h"`) and task 05's stand-in `h`.
The fixture adds the `reference-transaction` stub itself; whether husky 9 generates that name was
not checked. The tracked `.husky/reference-transaction` is
`d=<marker dir>; [ -d "$d" ] && echo ran >> "$d/host-hook-ran"; exit 0`, with `<marker dir>` an
absolute host temp directory outside the projects and worktrees directories, written into the
script when the fixture is made. In a Linux sandbox that path names no directory, so the agent's
commits skip it; only a **host** git that runs the project's hooks writes the marker
(re-review b03 Minor 1). Control, before step 1: the test runs its own plain
`git update-ref refs/e2e/probe HEAD` in the project, with no §HGH prefix, and asserts the marker
now exists (the hook fires on this host, so step 6's absence means something); it then deletes
the ref and, after that (the delete fires the hook too), the marker. The control is the
test's last git call in the project; from then on only the control plane runs git there. The run never writes a clone's config or hooks.
Then, each step asserting the status code and the named facts, and recording both in the
results document:

| # | Step | Asserts |
|---|---|---|
| 1 | `POST …/worktrees` ×2 on `evidence-upload` | 201, `kind:"clone"`; both in `GET …/worktrees` under the branch |
| 2 | a shell session on each; wait for `running` | the sandbox's `git status` exits 0 (`docker exec`) — **the linked-worktree failure is gone for cockpit worktrees** |
| 3 | commit in A (`upload-ui`) and a conflicting commit in B (`upload-api`) | each commit's `Panacea-Session` equals its session's id, and each carries `X-Husky: ran`: the project's hook ran in a clone the route made (task 03 step 6, task 05) |
| 4 | mark both ready; `PUT …/queue` [A, B] | 200 |
| 5 | `GET …/convergence` | A `clean`; B `conflicts`, `conflictsWith:[A]`, the path; both lanes' `commits[].sessions` |
| 6 | `POST …/lands` A with `expectedTip = tip` | 201; a two-parent merge with `Panacea-Land: <A>`; `<marker dir>/host-hook-ran` does **not** exist, although the land's `update-ref` and every lane fetch-back fire `reference-transaction` in the project (the host ran no repository hook: task-02 §HGH H1; the merge itself is `commit-tree`, which runs no hook either way, so no trailer can show this); A's ready flag false; B's clone `origin/evidence-upload` == the merge |
| 7 | `POST …/lands` B | 409 `merge_conflict` with the path; the branch tip unchanged |
| 8 | resolve in B: `git merge origin/evidence-upload`, fix, commit (`docker exec`) | the resolving commit carries B's session |
| 9 | `POST …/lands` B | 201 |
| 10 | `POST …/lands/{B's merge}/undo` | 200; tip back to A's merge; B ready again; `refs/panacea/undone/<sha>` exists |
| 11 | land B again, then `POST …/push` | 200, `setUpstream:true`; the bare remote's branch equals the tip |
| 12 | `GET …/lands` | both lands `pushed:true`, `undo:"revert"`, `undoWhy:"pushed"` |
| 13 | `POST …/lands/{B}/undo` | 409 `undo_not_exact`/`pushed` |
| 14 | `GET …/changes`, `…/changes/diff?path=<p>` | the path; a committed hunk with `tier:1` naming A's session and `sessionKinds` `shell`; `reason` for tier 2 "transcript links exist for Claude Code only" |
| 15 | a race: move the branch with `git update-ref` on the host, then land with the old tip | 409 `branch_moved` with the new tip |
| 16 | check out `evidence-upload` in the project's primary; `GET …/convergence` and land | `checkedOutAt` is the primary's path; 409 `branch_checked_out`; check it back out |
| 17 | `DELETE …/worktrees/{A}` while its session runs | 409 `live_sessions`; kill it; DELETE with nothing to lose → 200; clone dir, lane ref, conversation store gone |
| 18 | an uncommitted file in B; `DELETE` → `confirm_required`; commit in B (state changes); DELETE with the old token → `confirm_stale`; DELETE with the fresh token → 200; `refs/panacea/archive/lanes/<B>/…` names B's unlanded tip; a commit S made in B after `switch -c side` (then `switch` back) is in the question's `details.elsewhere` and named by `refs/panacea/archive/offbranch/<B>/…/heads/side` (review I6) |
| 19 | `POST …/branches` `throwaway`, `PATCH` rename, `GET …/renames`, `DELETE` | 201, 200 with `renamedFrom`, the map entry, 200 with `archivedAs` |
| 20 | `git fsck --no-dangling` on the project | exit 0 — no corruption from any of it |

The run skips with a sentence when Docker or `panacea-app/sandbox:dev` is absent. It writes the
image id, host git version, and wall-clock per step into the results document.

#### The owner's batched checklist (OD-12)

Written into the results document as a numbered list the owner works through in one sitting, each
item with the exact clicks or command, what to look for, and a blank for the answer. **Each is
unverified until then, and every report says so.**

1. **Look.** Open the cockpit; for Merge and Files, reach each of the prototype's fourteen states
   (`prototype/phase-3-convergence.html` `STATES`, lines 387-402) — the document gives the recipe
   for each against the e2e project. Visual acceptance is the owner's eyes; nothing here is a
   screenshot claim.
2. **A real agent.** `/login` in one sandbox; start Claude Code on a cockpit worktree; ask it for a
   small edit and a commit. Confirm the commit carries `Panacea-Session`, and that Files shows a
   **tier 2** link for it.
3. **The transcript canary.** From that session's conversation store, copy one Edit and one Write
   exchange into `control-plane/internal/transcript/claudecode/testdata/captured-2.1.274.jsonl`,
   redacting user text, reviewed for secrets; `TestCapturedFixtureParses` then runs instead of
   skipping (task 17).
4. **Push and PRs.** Push a throwaway branch to `github.com/lcfrai/panaceaapp` through the
   machine's credential manager; open one **draft** PR there, and one on an Azure DevOps
   repository the owner names; close both by hand (task 14's manual check — they create objects
   other people can see, so no agent runs them unasked).
5. **A husky repository.** Clone a project that uses husky into the projects directory and run
   its install step on the host (`npm install`), so `.husky/_` exists; then make a cockpit
   worktree and commit from a sandbox: its hooks ran and the trailer is present (task 05). The
   e2e run's step 3 proves the same on a synthetic layout; this item proves it on a real husky
   version. Record any `panacea: core.hooksPath names` line the commit printed.
6. **The open questions** Q1–Q5 (the plan's table): confirm or overrule each.
7. **Risk R4**, for the record: whether the managed Claude policy reaches the sandbox after
   `/login`. Phase 3 depends on neither answer (it uses no Claude hook or MCP).

#### The seams, anchored

For each row of the plan's *Seams phase 3 leaves for later phases*, and each Hub and phase 4 ask,
the results document records the file and line where the seam now lives in the tree (e.g.
`internal/gitx/machine.go:<n>` for S3-P8.1), found with `git grep`, not recalled. A seam whose
anchor cannot be found is a finding, reported, not papered over.

#### Steps

- [ ] **Step 1: Preflight.** Every Go task merged; `go test ./sandboxd/... ./control-plane/... -race -count=1` green;
  desktop `npx vitest run` green.
- [ ] **Step 2:** write `phase3_test.go`; run it with `-tags e2e` (PowerShell tool for Docker).
- [ ] **Step 3:** write the results document: the run's table with real statuses and times, any
  step that failed and why, the checklist, the anchored seams.
- [ ] **Step 4:** `PROGRAM.md`'s phase 3 row gains "built; owner checks batched (OD-12)" — not
  "DONE" until the owner has looked.
- [ ] **Step 5:** commit by pathspec.

#### Acceptance

```
go test ./control-plane/e2e/ -tags e2e -run TestPhase3EndToEnd -count=1 -v -timeout 20m   → PASS, 20 steps (or SKIP naming Docker)
git grep -n "S3-P8.1\|S3-P7.1\|H-1\|P4-1" -- docs/verification/                            → every seam and ask has a row with a file:line
```

#### What this task does not claim

- It is one run on one laptop. It proves the pieces fit, not that they scale to many lanes.
- Nothing about how the lenses look, and nothing a real agent does, is claimed until the owner's
  checklist is done.
