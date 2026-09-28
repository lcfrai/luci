# 09 — Review round 2: is the baton a closed loop?

**Date 2026-09-29.** Independent re-review of the baton fix round (commit `b5468547`, plus four
uncommittable hunks in `.superpowers/workflows/build-stream-continue.js`). Read-only: this file and
scratch are the only things written. No product code, no test suite, no container.

## The four questions, answered

| # | Question | Answer |
|---|---|---|
| 1 | Does a successor actually READ the baton before deciding what is done? | **Yes, twice.** Lines quoted below. No Critical |
| 2 | Is the write-vs-read-only contradiction resolved, and narrowly? | **Yes** — two named git-ignored paths, product files still barred |
| 3 | Do the script edits verify? | **Yes** — `node --check` 0, rules 1–13 intact, four hunks, nothing else moved |
| 4 | Was rule 13 copied into the live workflows' scripts? | **No.** Only `build-stream-continue.js` changed |

## Q1: the loop closes — the reader exists at both ends

The successor's first agent, `build-stream-continue.js:98`, before it touches the survey:

> `FIRST, before the survey: if ${SDDW}/baton.md exists, read it whole. It is the handoff the killed
> agent wrote about THIS stream (rule 13; convention at ${REPO}/docs/superpowers/AGENT-HANDOFF.md),
> and it is the only source written from inside the run that died. Use its "DONE and committed"
> SHAs, its in-flight unit and its "Uncommitted" paths as the leads you verify below --- leads, not
> facts … Where the baton and the ledger disagree, the LEDGER wins. If there is no baton, say so in
> notes.`

The agent that decides what is already done, `:127`, first sentence of the finish step:

> `BEFORE YOU CHANGE ANYTHING: read ${SDDW}/baton.md first if it exists --- the resume step wrote it
> and its "DONE and committed" section names the SHAs already in the branch … Verify the baton's
> claims against git log before relying on one; the ledger and the log win where they disagree.`

Both are ordinal ("FIRST", "BEFORE YOU CHANGE ANYTHING"), both rank the baton as leads below the
ledger and the log, and `:127` is the exact sentence that carries the re-apply warning. Review 09's
Important 1 is closed. The "if it exists" guard is not a hole: `:109` makes the resume step write one,
and if the resume step returns nothing the run stops at `:111` before the finish agent exists.

The per-task build agents (`:146`) and fix-round agents (`:163`) are still not told to read it. That
is correct: they start units the dead agent never entered, and the in-flight unit — the only place
the re-apply failure lives — is handled at `:127`.

## Q2: the carve-out is narrow

`:94` still says *"You are READ-ONLY in this step: change no file, run no test, commit nothing."*
`:109` resolves it by naming the exception rather than weakening `:94`: *"These two files are the
ONLY writes this step may make --- both are git-ignored, so neither is a commit, and 'change no
file' above stands for everything else."* Two absolute paths, both under `${SDDW}`
(`<worktree>/.superpowers/sdd/<stream>`, whose `.gitignore` is `*`). `control-plane/**`,
`desktop/**`, `sandboxd/**`, plans and contracts stay unwritable, and "run no test, commit nothing"
is untouched. Same shape as the ledger carve-out it sits beside. One edge in Minor 1.

## Q3: the edits verify, and nothing outside the prompts moved

I reconstructed the pre-baton file from this session's transcript — the full-file `Write` at
`~/.claude/projects/C--Users-au-dev-dmackie1-Projects-PanaceaApp/56a311e8-….jsonl` line 783, 170
lines — and diffed it against disk. **Four hunks, four lines out, five in:** `70a71` (rule 13),
`96a98,99` (the read paragraph plus its blank line), `105c108,109` (the ledger line keeps its text
and hands the template-literal close to the new baton line), `123c127` (the finish step's read list).
`RESUME_OUT`, `IMPL`, `REVIEW`, `REVIEWER`, `STREAMS`, the build loop and every `log` line are
byte-identical. Cross-checks: `node --check` exits 0; `:109` closes on a real backtick with
`{ label: KEY + ':resume-plan', phase: 'Resume', schema: RESUME_OUT }` intact; rules 1–13 each appear
once and in order (`:58`–`:71`, rule 7 still inside the `S.docker` ternary at `:64`), no gap, no
duplicate, none renumbered; 174 lines, 22,918 bytes, md5 `c1ec4931629b91d87a8617cc8fc114bb`. The old
finish sentence survives intact inside `:127` — the re-apply warning was extended, not replaced.

Commit `b5468547` is two files, +74/−1, both under `docs/`. Nothing else in the tree changed: the
only dirty paths are two `control-plane/internal/api/*_test.go` files that were already dirty at the
start of this round and belong to another workflow.

## Q4: the live scripts are untouched

Of the fifteen scripts in `.superpowers/workflows/`, `build-stream-continue.js` is the only one with
an mtime after 2026-09-27 (`find -newermt`), and it is the only one whose text contains "baton"
(`grep -l` over `*.js`). There is no `.js` anywhere else in the repo above depth 3. The deferral is
real, and it is still the owner's open question.

## Issues

### Important 1 — rule 13's ordinal and the resume step order the same write two different ways

`:71` now says *"Your FIRST Write in this task is the baton, before any other file"*, and rule 3
(`:60`) independently pushes a first write inside about ten tool calls. But `:108` tells the same
agent to append to `progress.md` first and `:109` says *"Then write ${SDDW}/baton.md, per rule 13,
from what you have just established … Write the baton BEFORE you return"*. "Per rule 13" points back
at the rule it inverts, so a literal reading is unsatisfiable and nothing says which wins.

What fails: the resume agent obeys the emphatic ordinal, writes `baton.md` at tool call six from what
it has so far, and overwrites the dead agent's baton — the file `:98` itself calls *"the only source
written from inside the run that died"* — with a stub. If it is then killed at minute eight, which is
the case `:109` names in its own justification, the successor reads the stub and the inside account
is gone. Fix is one clause at `:109`: rule 13's ordinal does not apply to this step, and the dead
agent's baton is not overwritten until there is something better to put in it.

### Important 2 — the round changed a fact and left one of its two restatements stale

`RESUME.md:188` still describes the baton as *"written in its first ten tool calls"* — the trigger
review 09 called too soft and this round replaced. `AGENT-HANDOFF.md:83` was updated in the same
commit; `RESUME.md` was not. `RESUME.md:180-183` compounds it: it lists what the resume step's first
agent reads (*"`stream-survey.json` plus the stream's own ledger and review verdicts"*) and does not
mention the baton, which is this round's entire change and now the step's first read.

What fails: the two convention documents disagree about when a baton is written, and the document
the orchestrator opens to launch a stream (`RESUME.md` §0d, the "read its baton" section) describes
the loop as still open. Two lines close it.

### Minor 1 — "the ONLY writes this step may make" collides with rule 2

`:109`'s phrasing is absolute, and rule 2 (`:59`) requires the opposite: *"Exit codes: redirect to a
file and read the real status. Never read a status through a pipe."* A redirect is a write, and the
resume step runs `git log` and `git status`. An agent that takes `:109` literally reads its exit
codes through a pipe — the named failure of rule 2 — or burns turns on the conflict. Bound the
sentence to the worktree and the repo, for example "the only files in the repository this step may
write"; scratch redirects outside it were never in scope.

### Minor 2 — the baton's read half is still unexercised

`ls .claude/worktrees/*/.superpowers/sdd/*/baton.md` is still empty, as review 09 found. Every claim
in Q1 is a reading of the prompt text, not of behaviour. The first evidence arrives when one stopped
stream resumes and its finish agent acts on a real baton; until then the loop is closed on paper.
Recorded, not charged against this round — no baton can exist before a run starts.

## The decision still open for the owner

Unchanged and correctly deferred: whether rule 13 and the two read clauses are copied into
`phase2-finish-*`, `decisions-and-work-plan-*` and `team-planfix-continue-*`. Two of those runs are
executing, and editing a live run's script can void it. The cost of waiting is that a death in those
chains still leaves nothing behind.

**Task quality: Needs fixes**
