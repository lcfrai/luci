# 09 — The incremental baton: what was built

**Date 2026-09-28.** Practice item 8 of `00-summary.md` §3. Documentation and convention only: no
product code changed, no test suite run, no container started.

## What landed

| # | Artifact | Committed? |
|---|---|---|
| 1 | `docs/superpowers/AGENT-HANDOFF.md` — the convention, 122 lines including a skeleton | `4fa28137` |
| 2 | `.superpowers/workflows/build-stream-continue.js` — **rule 13** added to the COMMON block | **no — see below** |
| 3 | `docs/superpowers/RESUME.md` §0d — a pointer subsection, 11 lines | `27536b0e` |

**`.superpowers/workflows/` is git-ignored** (`.git/info/exclude:8`), so the script edit exists on
disk and is live for the six stopped streams, but it is not and cannot be committed here. Verified
with `git check-ignore -v`. If that directory should be tracked, that is a separate decision and a
separate change.

## The shape adopted, and the one thing added to it

From the `handoff` method (`03-build-skills.md` §4, aihero.dev, read 2026-09-28 — method only, none
of their text): destination-shaped, references not copies, secrets redacted, uncommitted scratch.
**Their trigger is rejected**: they write it on request at the end, and the agent we need it for
never reaches the end.

Two things this programme added that the source method does not have:

1. **A DONE-and-committed section, with SHAs**, because the failure observed on 2026-09-28 is a
   successor re-applying a fix that already landed and reporting it as new work. References alone do
   not prevent that; a list of what is already in the branch does.
2. **An explicit write schedule** — first ten tool calls, then every commit, every unit boundary,
   every hazard, and before any command over two minutes. The rule that makes it incremental is the
   schedule, not the content.

## Rule 13, verbatim as it now reads in the COMMON block

> Keep a baton at `${SDDW}/baton.md` --- the handoff whoever resumes this stream reads when you are
> killed mid-unit, and the reason it is written EARLY is that an agent killed by the watchdog never
> reaches the end of its task. Write it within your first ten tool calls, overwrite it at every
> commit and every unit boundary, and never commit it. Six sections, each a few lines: the
> destination (who picks this up and for what, written first), what is DONE and committed with its
> SHAs, the one unit in flight and how far it got, what is uncommitted and where, hazards not to
> retry, and ONE next action. References with line ranges, never pasted copies; no secrets; no diary.
> `${REPO}/docs/superpowers/AGENT-HANDOFF.md` is the convention and carries a skeleton.

**It is a new rule 13 appended after rule 12, not an insertion.** Nothing was renumbered, so no other
document or prompt that cites a rule number by position is disturbed. `SDDW` and `REPO` were already
in scope (`build-stream-continue.js:47` and `:21`; `SDDW` is used by the ledger sentence at line 53,
`REPO` by rule 7). No other behaviour in the file changed: the resume agent, the finish agent, the reviewer block,
the output schemas and the loop are untouched. `node --check` on the edited file exits 0, which is
the check that catches an unterminated template literal — the one way an edit inside a backtick
string breaks all six streams at once.

## Why the baton is not committed

The stream workspace is git-ignored (`.superpowers/sdd/.gitignore` is `*`). Three reasons, in order:
a committed baton invites edits and becomes another artifact to reconcile against the ledger; the
committed record already exists as `progress.md` plus the git log; and a baton committed per write
would add tens of commits per stream to a branch whose log is read by humans. The cost is that a
baton dies with the machine, which is acceptable while every stream runs on this one host, and is
stated in the RESUME.md pointer so nobody is surprised by it.

## What this does not do

- **It does not make anyone read the baton.** Rule 13 makes every build agent *write* one, and
  `RESUME.md` §0d tells a restarting orchestrator to look for one. The resume step of
  `build-stream-continue.js` still reads `stream-survey.json`, the ledger and the review verdicts,
  and was deliberately left alone — the task's instruction was one rule in COMMON and no other
  behaviour change. The resume agent does inherit COMMON, so it is told the path, but it is not
  instructed to open it. See the open question.
- **It reaches build streams only.** The three currently-running workflows use different scripts
  (`phase2-finish-*.js`, `decisions-and-work-plan-*.js`, `team-planfix-continue-*.js`) and none of
  them gained rule 13. Editing a script while its run is live is a hazard this chain did not take.
- **No baton was written.** No stream ran during this chain; the first batons appear when the six
  stopped streams resume.
- **Nothing was verified by running a stream.** The rule's effect on agent behaviour is unmeasured.

## Open question for the owner

> **Should rule 13 be copied into the other workflow scripts, and should the resume step be changed
> to read the baton before the survey?**

The two halves stand or fall together, and both are judgement, not fact:

1. **Coverage.** Build streams are six of the nine workflows that have died this week. The plan-fix
   and gate chains die the same way and leave nothing behind. Copying the rule is a five-minute edit
   per script, but each is a *live run's* script, and editing one mid-run is exactly the class of
   change that voids a run.
2. **Consumption.** A baton that is written and never read is pure cost. Making the resume agent
   open `baton.md` first would close the loop, but it changes the behaviour of the one script all six
   stopped streams are about to resume on — which is the change this chain was told not to make, for
   good reason.

The conservative order, if the owner wants one: let the six streams resume on the current edit, read
one real baton, and only then decide whether it earns a place in the resume step and in the other
scripts.

## Citations resolved today

`00-summary.md:195` (practice item 8) · `03-build-skills.md:232-302` (the method and the rejected
trigger) · `06-sceptic.md` (does not touch this item; it rejected the product-side changes and left
the practice items) · `build-stream-continue.js:53` (stream workspace path) ·
`build-stream-continue.js:71` (new rule 13, before `STREAM LENS` at `:72`) · `RESUME.md:185-194`
(the new subsection).

## Fix round 1 — the read half (2026-09-28)

Closes both Importants and both Minors of `09-baton-review.md`. The reviewer's recommendation is
taken as given: **take the read half now, defer the copy into the live scripts.** Nothing was copied
into `phase2-finish-*`, `decisions-and-work-plan-*` or `team-planfix-continue-*`.

### What changed in `.superpowers/workflows/build-stream-continue.js`, line by line

The file is git-ignored (`.git/info/exclude:8`), so `git diff` cannot show this and the ledger below
is the record. Before: 171 lines, 21,191 bytes, md5 `adc707a49eb018fdf25572f9e65f9100`. After: 174
lines, 22,918 bytes. `diff` against a byte copy taken before the first edit reports exactly four
hunks — `71c71`, `97a98,99`, `106c108,109`, `124c127` — four lines out, five lines in, nothing else
moved.

1. **`:71`, rule 13 — Minor 1 and Minor 2.** One sentence replaced. Was *"Write it within your first
   ten tool calls, overwrite it at every commit and every unit boundary, and never commit it."* Now
   the trigger is an ordinal — *"Your FIRST Write in this task is the baton, before any other
   file"* — and the schedule regains the two dropped moments: *"the first time anything goes wrong,
   and BEFORE starting any command you expect to run over about two minutes (a kill mid-suite must
   not cost you the baton)."* An ordinal trigger also satisfies rule 3's *first file write within
   about 10 tool calls* by construction. The rest of rule 13, including the six sections and the
   pointer to this convention, is unchanged.
2. **`:98-99`, new paragraph in the resume step — Important 1, read half.** Inserted between the
   survey paragraph (`:96`) and `VERIFY, on disk, in the worktree:` (now `:100`). It tells the
   resume agent to read `${SDDW}/baton.md` whole *before* the survey if it exists, because it is the
   only source written from inside the run that died; to treat its SHAs, in-flight unit and
   uncommitted paths as **leads to verify, not facts**; that **the ledger wins** on conflict (which
   matches `RESUME.md` §0d and `AGENT-HANDOFF.md:40-41`); and to say so in `notes` when no baton
   exists. No schema change — `RESUME_OUT` is byte-identical, so a resumed run returns the same
   shape.
3. **`:109`, new final line of the resume prompt — Important 2, the contradiction.** The read-only
   sentence at `:94` now has a second carve-out beside the ledger's: the step writes the baton from
   what it just established, *"These two files are the ONLY writes this step may make --- both are
   git-ignored, so neither is a commit, and 'change no file' above stands for everything else."* It
   says to write it **before returning**, and names the cost it avoids: the reconstruction this step
   produces is what a whole survey run rebuilt by hand on 2026-09-28.
4. **`:127` (was `:124`), the finish step's do-not-re-apply sentence — Important 1, read half.** The
   baton is now the first thing read, ahead of the task report, the ledger tail and `git log`, with
   what each of its sections answers (SHAs already in the branch, salvage on disk, what not to
   retry), followed by *"Verify the baton's claims against git log before relying on one; the ledger
   and the log win where they disagree."* The original re-apply warning that follows is untouched.

Renumbering, for anyone holding the review's citations: rule 13 is still `:71` and the read-only
sentence is still `:94`; the old `:106` is now `:108`, and the old `:124` is now `:127`.

### How it was verified without a diff

- `node --check .superpowers/workflows/build-stream-continue.js` exits **0**.
- Byte copy taken before the first edit, then `diff` — four hunks, listed above, and no hunk outside
  rule 13, the resume prompt and the finish prompt.
- Read of the whole rules block after the edit: rules 1–13 each appear once, in order, none
  renumbered; rule 7 still sits inside the `S.docker` ternary at `:64`; `RESUME_OUT`, `IMPL`,
  `REVIEW`, `REVIEWER` and the build loop are untouched.
- The template literal that the resume prompt lives in closes on its original backtick at `:109` —
  checked by reading the line, because an escaped backtick would still pass `node --check`.

### Convention change

`docs/superpowers/AGENT-HANDOFF.md:83`, first row of the schedule: *"Within the first ten tool
calls"* → *"Your FIRST Write of the task, before any other file"*. This one **is** in git. The other
four rows, including *"Before any command over about two minutes"* at `:87`, were already correct;
the drift was rule 13's, and it is now gone.

### Still open for the owner

**Should rule 13 be copied into `phase2-finish-*`, `decisions-and-work-plan-*` and
`team-planfix-continue-*`?** Deferred, not decided: two of those runs are executing now, and editing
a live run's script can void it. The cost of waiting is that those chains still die leaving nothing
behind. Revisit when they finish.

**Minor 3 stands unfixed** and is not this chain's to fix: the workflow scripts are git-ignored, so
rule 13 survives a restart of this host but not a re-clone, and no review will ever see it in a diff.
