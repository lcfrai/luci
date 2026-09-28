# The baton — an incremental handoff for workflow agents

**Date 2026-09-28.** Method adapted from the `handoff` skill in the aihero.dev catalogue
(<https://www.aihero.dev/skills-handoff>, read 2026-09-28; study at
`planning/2026-09-28-discovery-skills/03-build-skills.md` §4). **Their trigger is rejected and their
text is not used.** The shape is theirs; the words here and the rule that the file is written from
the first minutes are ours, and the second one is the whole point.

## Why this exists

Agents in this programme die without warning: a 180-second no-progress watchdog, usage stops,
sign-outs, and a host that kills freshly spawned processes under load. On 2026-09-28 seven streams
were each found dead mid-fix-round with uncommitted work on disk, and a survey agent had to be run
to reconstruct where every one of them had stopped.

> **A handoff written at the end of a task is never written, because the agent that dies never
> reaches the end.**

So the baton is written in the first minutes and updated as work lands. The version on disk when the
agent is killed is the one that gets used, and it must already be worth reading.

## Where it lives

`<stream workspace>/baton.md` — one file, beside the ledger, overwritten in place.

- **Never committed.** The stream workspace is git-ignored. A committed baton invites edits and
  becomes another artifact to reconcile; the committed record is the ledger and the git log.
- **One file per stream, not one per task.** The successor should not have to choose.
- **Overwrite, do not append.** The baton is current state, not history.

## Baton and ledger are different documents

| | `progress.md` (ledger) | `baton.md` (baton) |
|---|---|---|
| Shape | dated entries, appended, history | one current state, overwritten |
| Written for | the programme record | the specific agent who picks this up next |
| Answers | what happened, in order | what to do in the next ten minutes |
| Grows | forever | never |

Both exist. Neither replaces the other. If a fact belongs in both, the ledger carries it and the
baton points at the ledger.

## What a baton contains

Six things, in this order, each a few lines. If a section is empty, write `none` — an absent section
reads as "the agent died before it got there".

1. **Destination.** Who picks this up and what they are for: *finishing task 14's fix round*, not
   *phase 6*. This is written **first**, before anything else, because it decides what the rest of
   the baton keeps. A baton for a review round and a baton for a build round are not the same file.
2. **DONE and committed.** The task ids and commit SHAs that are finished and in the branch, with
   the one-line claim each commit makes. **This section is not optional and it is the one this
   programme added.** The specific failure here is a successor re-applying a fix that already
   landed, then reporting it as new work.
3. **In flight.** The one unit being worked on, its plan part path with a line range, and how far it
   got: which test is red, which clause of the part is unmet.
4. **Uncommitted, and where.** Every path with unstaged or untracked work, and what state it is in
   (compiles / does not compile / test written but never run). A successor that does not know this
   will either lose the work or trust it.
5. **Hazards.** What has already gone wrong here and what not to retry: a host kill code, a suite
   that takes 18 minutes, a file another stream owns.
6. **The one next action.** A single command or a single edit, runnable now. Not a plan, not a list.

## What a baton does NOT contain

- **No copies.** A path and a line range, never a pasted file, function or plan clause. A second
  copy of a fact rots: on 2026-09-28, of twenty-two citations resolved by hand in `docs/SEAMS.md`,
  every one pointed at the wrong line because the files had grown underneath them. A pointer is
  re-resolved on read; a copy is believed.
- **No diary.** Not what the agent tried, read, considered or abandoned, unless a successor would
  otherwise repeat it — in which case it is a hazard (section 5), stated as a hazard.
- **No secrets.** No tokens, keys, connection strings, PATs or `PANACEA_APP_*` values. Name the
  variable, never its contents. Redact before the write, not afterwards.
- **No conclusions the agent did not verify.** A baton that guesses is worse than a short one. Mark
  an unverified line `UNVERIFIED` and say how to check it.
- **No instructions to a person.** The reader is the next agent under the same owner. It never
  crosses a user boundary (`OD-15`, `OD-16`), and a cross-user channel is not what this is.

## When to write it

| Moment | What changes |
|---|---|
| Within the first ten tool calls | the whole file, destination first, from the resume step's findings |
| Every commit | section 2 gains the SHA; section 4 loses the paths it covered |
| Every unit started or finished | section 3 |
| Anything that goes wrong once | section 5 |
| Before any command over about two minutes | section 6, so a kill mid-command is survivable |

The cost is one Write of about thirty lines, a few times per unit. The failure it prevents has
already cost this programme a whole survey run.

## The skeleton

Copy the headings, not the example text. Keep the whole file under about fifty lines.

```markdown
# Baton — <stream> — <UTC timestamp of this write>

## Destination
Finishing task 14's fix round: two Important findings from review round 1 remain.

## Done and committed
- t12 `a1b2c3d4` — lane store rejects a duplicate lane id
- t13 `e5f6a7b8` — lane list endpoint, 6 tests
- t14 `c9d0e1f2` — PARTIAL: the fix for finding 1 only. Findings 2 and 3 are NOT fixed.

## In flight
t14 fix round 1. Review: `.superpowers/sdd/<key>/reviews/t14-r1.md:40-58`.
Plan part: `docs/superpowers/plans/.../task-14.md:210-260`.
Red test: `TestLaneRebind/stale_revision` — asserts 409, gets 200.

## Uncommitted
- `control-plane/internal/lane/rebind.go` — compiles, fix for finding 2 half-written
- `control-plane/internal/lane/rebind_test.go` — new test, red, never committed

## Hazards
- `go test ./control-plane/internal/lane/...` takes about 7 min on this host. Background it.
- One `exit status 0xc0000142` at 14:12 — host kill, not a compile error. Re-ran and it passed.

## Next action
`go test ./control-plane/internal/lane/ -run TestLaneRebind -race -count=1 > /tmp/t.txt 2>&1`
```
