### Task 64: "Move my lanes to the cloud"

**Slice TM2. Depends on: 21, P3-24. About 2.5 hours. Seam: none new. Not costed in
`05-design.md` §12; this plan places it in TM2 and says so.**

When a project is shared, every new session launches in Fleet, and a session still running on
the laptop keeps working, labelled *outside the team* (`05-design.md:1044-1045`, task 21). Work
the owner already has in laptop lanes stays there. §10.1 item 4 (`:1046-1048`): *"Unpushed laptop
lanes come along on request. 'Move my 2 lanes to the cloud' exports each as a bundle (phase 3
ruling R4) into the owner's cloud machine, where Fleet takes them over. Nothing moves without
that click."* This task is that request.

Three properties, each pinned by a test.

1. **Nothing moves without the call.** No timer, share event, sign-in or launch moves a lane. The
   only code path that exports a lane to the cloud is `POST …/team/lanes-move`, and the desktop
   calls it only from the button.
2. **A lane already on the machine is not duplicated.** The machine's lane index is read first;
   a lane with the same worktree id is updated fast-forward only, or skipped, never imported a
   second time.
3. **Lane work crosses only as a verified bundle** (phase 3 ruling R4). The laptop's host git
   reads the lane from the host-private project repository, never from the lane clone; the
   machine verifies the bundle before it creates anything. No host path, config key or URL
   crosses (`04-enterprise-constraints.md:460-463`).

Five decisions the scope left open, made here.

**(a) What "unpushed" means.** A laptop lane is a candidate when its synced tip (phase 3 task
07's `SyncLane`, which already brings the lane into the project repository as a bundle through
task 24's runner) has commits the product branch's upstream does not reach. A lane with nothing
ahead is not offered: its work is already on the remote, and the cloud machine fetches it with the
project's read-only credential (task 48).

**(b) Only committed work moves.** A bundle carries commits. A lane with uncommitted changes is
listed with the count (*"3 files not committed stay on this laptop"*) and moves only if the owner
ticks *Move the committed work* for it (`leaveUncommitted: true`); otherwise it is skipped with
`lane_uncommitted`. Nothing on the laptop is discarded either way.

**(c) A lane with a live local session is not moved.** Two agents on two copies of one lane
would diverge at once. The candidate list says *"a session is running in it"* and the move
skips it with `lane_busy`; the owner stops the session and asks again.

**(d) The laptop copy is kept, and frozen.** After a lane moves, phase 3's lane index marks it
`movedAt` and `movedTo` (the machine id), and a new local session in that lane is refused with
`lane_moved_to_cloud` and a link to the cloud lane. The laptop copy is not deleted: deleting an
owner's work is a destructive action this task does not take on her behalf. She may remove the
worktree later with phase 3 task 11's remove, which already asks.

**(e) "Fleet takes it over" is the next launch.** The moved lane becomes a lane on the owner's
cloud machine like any other: `machined`'s sweep sees it (task 34), its first commit there is
exported to the mirror (task 60), and the desktop offers **Continue in the cloud** per moved
lane, which is task 21's team launch with that `worktreeId`, creating the agent record (task 06)
whose `laneRef` names it. Until that launch there is no agent on it, so no lease and no
presence; the lane appears in teammates' views only once an agent is running on it (task 63).
