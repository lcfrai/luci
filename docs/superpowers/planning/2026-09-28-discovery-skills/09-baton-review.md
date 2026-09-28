# 09 — Review: does the baton survive its own failure mode?

**Date 2026-09-28.** Independent review of the baton chain (`09-baton-report.md`, commits `4fa28137`,
`27536b0e`, `668af891`, plus one uncommittable edit to `.superpowers/workflows/build-stream-continue.js`).
Read-only: this file and scratch are the only things written. No product code, no test suite, no
container.

## The four questions, answered

| # | Question | Answer |
|---|---|---|
| 1 | Useful if the agent dies 90 seconds in? | **Yes on content, soft on trigger** — Minor 1 |
| 2 | Prevents a successor re-applying a landed fix? | **The section exists; nothing reads it** — Important 1 |
| 3 | Does the script edit change anything else? | **No. One added line, byte-exact** — evidence below |
| 4 | Short enough to be followed under the watchdog? | **Yes** — 756 chars, self-contained, no new mandatory read |

## Q3 first, because it was the Critical risk: the script edit is byte-exact

The file is git-ignored, so `git diff` cannot answer this. I reconstructed the predecessor from this
session's own transcript: the full-file `Write` of `build-stream-continue.js` recorded at
`~/.claude/projects/C--Users-au-dev-dmackie1-Projects-PanaceaApp/56a311e8-….jsonl` line 783
(20,422 chars) is the version before the baton chain touched it. Dumped and diffed against the file
on disk now:

```
diff written.js build-stream-continue.js
70a71
> 13. Keep a baton at ${SDDW}/baton.md --- the handoff whoever resumes this stream reads …
```

One hunk, one added line, zero removed. Byte delta is **757 = the 756-character rule plus its
newline** — arithmetic that only closes if nothing else moved. `node --check` exits 0. Rules 1–13
each appear once, in order, none renumbered (rule 7 sits inside the `S.docker` ternary at `:64`, as
before). `SDDW` (`:47`) and `REPO` (`:21`) are both in scope before `COMMON` is built at `:50`, so
no `ReferenceError` at load. The resume prompt, finish prompt, `REVIEWER`, both output schemas and
the build loop are unchanged.

**No Critical.** The six stopped streams resume on the same script plus one rule.

Commits are equally clean: `4fa28137` = +122 lines, one file; `27536b0e` = +11 lines, one file;
`668af891` = +104 lines, one file. All additive, all under `docs/`.

## Q1: an agent killed 90 seconds in

The convention passes the substantive test. Of the six sections in `AGENT-HANDOFF.md:48-62`, four are
fillable before the agent has done any work of its own — destination (`:48`), DONE-and-committed
(`:51`, whose SHAs come from `git log` and the resume step, not from this agent's output), hazards
(`:60`, inherited) and next action (`:62`). A baton written at minute one is already the thing the
2026-09-28 survey run had to reconstruct by hand. The convention does **not** only make sense once
work is done.

What is soft is the trigger — see Minor 1.

## Q2: the re-apply failure — the section exists, the reader does not

The line asked for: **`docs/superpowers/AGENT-HANDOFF.md:51-54`**, section 2 "DONE and committed",
with its own justification at `:53-54` — *"The specific failure here is a successor re-applying a fix
that already landed, then reporting it as new work."* It reaches agents through
`build-stream-continue.js:71` ("what is DONE and committed with its SHAs").

That is the write half. The read half does not exist yet, which is Important 1.

## Issues

### Important 1 — the baton is written by every agent and opened by none

`build-stream-continue.js:96-106` lists what the resume agent must read: the survey JSON, review
verdict lines, the plan's task table, the ledger tail, `git log`, `carried/INDEX.md`. `baton.md` is
not among them. The finish agent at `:124` is told to read "the task's own report … the tail of
progress.md … git log" before changing anything — again not the baton, even though `:124` is the
exact sentence about not re-applying a committed fix, and the baton's section 2 is the exact answer
to it. Rule 13 is in `COMMON` so every agent is told the path exists, but under a 180-second watchdog
agents work the explicit list.

Until one line changes, the convention cannot prevent the failure in Q2 — it can only enable
preventing it. The implementer disclosed this and escalated it rather than editing behaviour it was
told not to edit; that was the right call, and it is the owner's decision below.

### Important 2 — rule 13 contradicts the resume step's own read-only instruction

`build-stream-continue.js:94` tells the resume agent: *"You are READ-ONLY in this step: change no
file, run no test, commit nothing."* `:71` tells the same agent to write `baton.md` within its first
ten tool calls. One carve-out already exists for the ledger (`:106`, "that file is git-ignored, so
this is a note, not a commit") but none for the baton, so the agent must guess which instruction
wins.

This lands where the value is highest. The resume agent holds the reconstruction that previously
cost a whole survey run to produce; if it dies at minute eight having obeyed `:94`, that work is lost
again and the next run re-derives it. Fix is one clause at `:106`, in the same file and the same
class of edit already made: name the baton alongside the ledger as the second thing this step may
write.

### Minor 1 — the trigger is a tool-call count, not a position

"Within your first ten tool calls" (`:71`, and the first row of the schedule at
`AGENT-HANDOFF.md:83`). `COMMON:55` already requires three documents read before anything else, and a
build agent then reads its plan part in ranges, so ten tool calls can be several minutes in — past
the 90-second case. Rule 3 (`:60`) separately spends the same budget: "Make your first file write
within about 10 tool calls." Sharper and shorter: **your first Write in this task is the baton.**
That also satisfies rule 3 by construction.

### Minor 2 — the rule drops the two triggers that cover death mid-command

`AGENT-HANDOFF.md:81-87` lists five write moments. Rule 13 carries three (first ten calls, every
commit, every unit boundary) and omits "anything that goes wrong once" and "before any command over
about two minutes". The second omission is the one that matters: a stream that backgrounds a
974-second suite (`RESUME.md` names lanes ~974 s, gitx ~1110 s) and is killed during it falls back to
a baton last written at the previous commit. Nine words in the rule close it.

### Minor 3 — rule 13 exists on disk only, and no review will ever see it

`.git/info/exclude:8` ignores `.superpowers/workflows/`, confirmed with `git check-ignore -v`. The
rule survives a restart of this host, not a re-clone, and never appears in a diff — which is why this
review had to reconstruct the predecessor from a session transcript to check it. Pre-existing repo
condition, honestly disclosed at `09-baton-report.md:14-17`; recorded here because the cost is now
higher than it was.

## Q4: length

Rule 13 is one paragraph, 756 characters, about 1 percent of the `COMMON` block, and **names all six
sections inline** — an agent never has to open the 122-line convention to comply, and no new
mandatory read was added to `COMMON:55`. Pointing at `${REPO}/docs/…/AGENT-HANDOFF.md` rather than a
worktree path is correct: the doc is committed on `feat/phase-2-cockpit` in the main tree and does
not exist on the stream branches. The skeleton (`:96-122`) caps the artifact at about fifty lines,
which is a couple of Writes per unit. Proportionate.

## Checks that passed, briefly

- Paths agree across all three artifacts: `SDDW` = `<worktree>/.superpowers/sdd/<stream>` (`:47`),
  matching `AGENT-HANDOFF.md:24` and `RESUME.md:185-194`.
- "Never committed" is real, not aspirational: `.claude/worktrees/phase-3/.superpowers/sdd/.gitignore`
  is `*`.
- No baton exists anywhere yet (`ls .claude/worktrees/*/.superpowers/sdd/*/baton.md` is empty), so
  every claim about agent behaviour remains unmeasured — as the report itself states.
- `RESUME.md` §0d ranks the ledger above the baton on conflict; `AGENT-HANDOFF.md:40-41` says the
  same from the other side. Consistent.
- All six stopped streams do use this one script (`RESUME.md:176`), so the coverage claim holds.

## The decision the owner is being asked for

**One decision, two halves: should the resume step be changed to READ the baton, and should rule 13
be copied into the other workflow scripts?**

Today every build agent will write a baton and no agent is instructed to open one. Closing that loop
means editing `build-stream-continue.js`'s resume step — the same file, and the six streams that run
it are stopped, so that half carries the same risk as the edit already made, which is near zero.
Copying the rule into `phase2-finish-*`, `decisions-and-work-plan-*` and `team-planfix-continue-*`
is the riskier half: those runs are live, and editing a live run's script can void it.

My recommendation, which differs from the report's: **split the halves.** Take the read half now —
it is one clause in a stopped script and it is what makes the other three commits worth anything.
Defer the copy half until the running workflows finish. Add Important 2's carve-out in the same edit.

Alternative, if the owner prefers to prove it first: resume the six streams as they stand, read one
real baton, then decide both halves together. The cost of waiting is that the first six resumed
streams write batons nobody reads.

**Task quality: Needs fixes**
