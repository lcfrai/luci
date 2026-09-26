### Task 64: the Desk: FAILED with *Restart from checkpoint* and *Abandon*; LANDED; RUNNING's fold

Stream **WD2**, about 3 hours (an estimate, not a measurement). Depends on task 62 (SW.17:
`registerDeskRow`, `registerDeskSource`, `DeskItem`, `DeskRank`, `DeskSnapshot` with
`lastVisitAt` and `sessions`, `task-62.md:126-157`, `:198-208`), and through their seams on task
19 (`Session.issueRef`, `task-19.md:300-303`), task 24 (SW.10's discard, `task-24.md:112-114`,
`:287-291`) and task 10 (SW.4's `IssueHead.proof`, `task-10.md:47-54`, `task-37.md:102`). Uses
task 63's block in `deskRows.tsx` (its `hhmm` and `stood`, `task-63.md:330-335`), task 61's
`firstSign` (`task-61.md:185-194`), task 40's `whoLabel` (`task-40.md:432-434`), task 37's
`WorkClient.discardStart` (`task-37.md:357-358`), task 29's `MarkGlyph`, task 56's `mutate`, the
built `api.restartSession` (`desktop/src/api/client.ts:153`, `:338`), `classify`
(`desktop/src/api/problems`), `formatElapsed` (`desktop/src/lib/time.ts:27`) and the heartbeat
(`hooks/heartbeat.ts:64`).

After this task the Desk's FAILED, LANDED and RUNNING sections hold rows: a failed agent on the
person's issue with the two answers that matter, what of theirs landed and how it was proven, and
their agents, with the ones that changed since they last looked drawn as three-line cards and the
rest folded into one line.

**What the reader sees** (05 §4.2's FAILED, LANDED and RUNNING rows and *RUNNING's shape*,
`05-design.md:938-948`; §3.3, `:736-753`; AX §5, `docs/AGENTIC-UX.md:148`;
`prototype/work-layer.html:767-785`):

- **A FAILED row**: `dylan · agent 1` and *EP-142*, its title, then *"exit at 02:14 · process
  exited"* in mono, the reason as the control plane wrote it. Under it *"ranked first: the agent on
  EP-142 stopped and nothing is working on it"* and *"if you do nothing: the worktree stays as it
  is and nothing resumes"*. Two buttons: *Restart from checkpoint* and *Abandon*.
- ***Restart from checkpoint*** answers in place: *"Restarting in the same worktree"*, then
  *"Restarted · running"* or the control plane's own sentence. The Desk does not move to Develop.
- ***Abandon*** asks first: the first press shows *"Abandon deletes the worktree and its
  uncommitted changes: upload/resume.go, upload/chunk.go"* and an *Abandon* button in `--need`;
  only the second press discards. The session record stays listed in Develop.
- **A LANDED row**: the `done` glyph, *EP-142*, and the word SW.4's result gives, never another:
  *"landed and proven at 14:02"*, *"proven, self-attested at 14:02"*, *"done, not proven at
  14:02"* in the *done, not proven* glyph, *"done, serves no criterion at 14:02"*, or, still
  `landed`, *"landed at 14:02, evidence waiting"*. When a sibling serving the same criterion is not
  landed, a second clause: *"done · c-91e2 still open (EP-150 not landed)"* (05 §3.3).
  A *Mark seen* button, with *Undo* for the undo window (6 s).
- **RUNNING**: one card per agent that **changed since the last visit**, exactly three lines:
  `dylan · agent 2 · idle · EP-160` with *"running 41m · last output 11m ago"*; the issue's title
  in `text·H`; and the next fact it waits on, as a rule: *"Waits for you: the agent is idle"*.
  Under the cards, one line: *"23 steady, all live"* (or *"21 steady, 2 quiet"*) with *Expand*;
  pressing it lists each steady agent in place, one line each.
- No row shows `~`, a `%`, a bar or a countdown (`OD-27`): every number is an observation.

**Decisions made here, so nobody re-derives them:**

1. **Five kinds, one pure file.** `work/desk/agents.ts` holds the selectors over a `DeskSnapshot`
   and returns `DeskItem`s; no React, no store, no clock (`snap.takenAt` is the now). The kinds:
   `lane.failed` (FAILED), `issue.landed` and `issue.done` (LANDED), `agent.changed` and
   `agent.steady` (RUNNING). `deskRows.tsx` registers each as a row and a source (task 62's
   decision 2), in a block after task 63's. Task 71 adds `issue.shipped` to LANDED beside these.
2. **An agent is on my issue when its `issueRef` starts `issue:<id>@` and the head is mine.** The
   prefix test is task 40's and task 60's (`task-40.md:426`, `task-60.md:106`): the desktop builds
   the prefix from a head's id and compares; it never splits the key (`task-19.md:301-302`). *Mine*
   is `head.assignee.id === snap.me`, as task 63's decision 2. A session with `issueRef === ""` is
   not on the Desk: Develop shows it.
3. **Failed means exited with a reason that is not my own stop.** `status === "exited"`,
   `exitReason !== ""` and `exitReason !== "killed"`. `"killed"` is the one reason the kill route
   writes (`control-plane/internal/api/sessions.go:1723-1726`), which is the person's own stop;
   the reaper's `"process exited"`, `"idle timeout"` and `"sandbox disappeared"`
   (`reaper/reaper.go:45-48`) are failures, and so is any other non-empty reason, shown verbatim.
   The exit time is the record's `updatedAt`: the registry stamps it on every write
   (`registry/filestore.go:321`) and `markExited` refuses to touch an exited record
   (`reaper.go:632-634`), so it is the exit's write unless a later rename touched it (gap 3).
   A failed session whose issue has a newer live session is not FAILED: the person already
   restarted or started again, and RUNNING shows that agent.
