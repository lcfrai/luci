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
