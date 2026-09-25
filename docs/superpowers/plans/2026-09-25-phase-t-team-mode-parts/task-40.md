### Task 40: Runner rotation and DIVE

**Slice:** TM1. **Focused time:** 2.5 h. **dependsOn:** 27 (the runner, `Argv`, `ChildEnv`,
`Parked`), 39 (Fleet's attach handler, `RouteAgent`); 26 (ST.5, `SessionFor`, the claim) and 31
(`CurrentTurn` names DIVE's origin) through them. **Seam:** ST.5, extended.

Two things keep an Orchestrator's context and cost bounded, and one lets its owner step inside it.

1. **Rotation.** An Orchestrator is one Claude conversation continued turn after turn
   (`task-27.md:13-19`). Left alone it grows for ever: every turn re-reads the whole transcript,
   and the transcript holds everything the Orchestrator ever wrote about peer text. §4.2 and `06`
   I9 require a **new** session each day and at each task boundary, seeded **only** from Fleet's
   typed state, **never** from the old transcript, so no agent-written summary of a teammate's
   words is carried forward. Taint is not per session (task 31 taints every team Orchestrator from
   its first turn), so rotating resets nothing unsafe (§7.2).
2. **DIVE.** While the runner is parked between turns, the owner may attach an **interactive**
   `claude --resume <same id>` through the runner, see the conversation, and talk to it directly.
   Never a fork: the owner is inside the real session, not a copy (§4.2; AGENTIC-UX's `Enter` to
   dive, `docs/AGENTIC-UX.md:147`). Team events queue meanwhile and show as waiting. DIVE's start,
   its end and every directive it issues are `ot-…` records with origin `human`.

Six decisions the scope left open, made here so nobody re-derives them.

**(a) Fleet decides rotation; the runner only obeys `mode`.** Task 26 already made the session id
Fleet state and left `rotateDue` for this task (`task-26.md:22-29`, `:558`). `turns.RotateDue`
is one pure function: due when the session's first turn was on an earlier **Australia/Sydney**
calendar date than now (the product's one region; UTC would rotate at 10:00 local, mid-morning),
or when a task boundary is pending. The runner sees only `mode: "new"` with a fresh uuid, which it
already handles (`task-27.md:149-157`). Fleet embeds `time/tzdata`, so the zone does not depend on
the image's `/usr/share/zoneinfo`.

**(b) A task boundary is a task of this Orchestrator's reaching `done` or `abandoned`.** Task 24's
`task done` and `task abandon` verbs (`task-24.md`) call `turns.MarkBoundary(orch)`, which sets
`rotate_pending`; the next Fleet turn is `new`. A boundary during a DIVE waits for the DIVE to end,
and a DIVE never rotates (decision e).

**(c) Each session gets its own `HOME`, and rotation deletes the old one whole.** The runner cannot
tell Claude Code "forget", and must not reach into its transcript files (OD-8: a transcript may be
**read** through a version-pinned adapter, and nothing may require it; nothing here reads or
names a file inside it). So `rotate.go` gives each session id its own directory,
`/run/panacea/sessions/<uuid>/home`, passes it as the child's `HOME`, and on the first `new`
turn after a rotation removes **every other** session directory with one `os.RemoveAll` per
directory, after the new child has started. The old transcript is then physically gone from the
pod: no `Read` tool call, no `--resume` and no seed can carry a byte of it. The runner never lists,
opens or parses anything under a session's `HOME`; it creates the directory and deletes it.

**(d) The seed is a typed verb, `panacea seed`, not text in the prompt.** Task 27 accepts only
task 26's `Input(n)` as the turn's input (`task-27.md:20-23`), and that stays. A new session
learns its world by calling `panacea seed`, an Orchestrator-only read verb (task 41's instructions
tell it to call `seed` first in any session with no history; the managed `claudeMd` loads in every
session). Its result is Fleet's typed state only: the `panacea team` view (task 22), open threads
by id, kind, state and counterpart (task 28, no message bodies), its human's escalation policy rows
(task 30), open escalations by id, and its **own human's** last five messages (`hm-…`, `text·H`,
from the Hub). No `ms-…` body, no turn reply, nothing an agent wrote. Peer text still reaches it
only through `panacea inbox`, quoted (§7.1).

**(e) DIVE is a claimed turn with origin `human`, so nothing else can run.** `POST
/v1/agents/{ag}/dive` claims a turn through task 26's `ClaimTurn` (the two-replica guard) with
origin `human`, `Mode: resume` on the current uuid (or `new` if there has never been one), and
`Until = now + DiveMax` (2 h). The claim is what makes Fleet's scheduler start nothing while the
owner is inside (`task-26.md:565-566`), so team events queue exactly as they do behind any turn,
and `CurrentTurn` returns the DIVE's `TurnRef` with origin `human`, which is the hook task 31
left for this (`task-31.md:41-44`). Rotation is never due inside a DIVE: the owner dives into the
conversation they can see, not a fresh one.

**(f) The runner hands its own PTY to the interactive child, and reads nothing.** The runner is
the orchestrator pod's PTY command (`task-27.md:3-9`), so its stdin, stdout and stderr **are** the
PTY the owner attaches to through task 39. On `dive.begin` it starts `claude --resume <uuid>`
(interactive: no `-p`, no `--output-format`, never `--fork-session`) with those three descriptors,
the same environment allowlist plus the PTY's `TERM`, the session's `HOME`, and its own process
group; then it waits for the child to exit, and sends `dive.end`. The runner never reads the PTY
while the child owns it: the bytes go from the child to `sandboxd`'s hub to the owner, as in
phase 2.

