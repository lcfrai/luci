### Task 61: Start on the desktop: `Shift+R`, the *Undo Start* chip, *Stop and discard*, the 10 s first sign

Stream **WD1**, about 3 hours (an estimate, not a measurement). The desktop half of seam **SW.10**.
Depends on task 24 (the three control-plane routes and their wire, `task-24.md:92-127`) and task 56
(`mutate`, `MutateHandle`, `Refusal`, `onRefused`, `task-56.md:324-356`). Uses task 37's client
members `start`, `undoStart` and `discardStart` and their types (`task-37.md:231-241`, `:353-358`,
`:446-448`), task 37's `workProblem` (`task-37.md:478-485`), task 20's fixtures
(`task-20.md:155-165`, `:217-241`, `:299-330`) and `expectNoPrediction` (`task-20.md:126`), task
44's `onStart` prop (`task-44.md:186`), and the built heartbeat (`desktop/src/hooks/heartbeat.ts:64`,
one interval per window) and `isInsideTerminal` (`desktop/src/hooks/useKeyboardMap.ts:90`). Task
53's `KEYMAP`, `registerVerb`, `isListKeyTarget` and `RowContext`, and task 40's `IssueRow`, have
no part on the tree at this writing: their names are their **scopes'** words (`tasks-41-60.md:390-427`,
`tasks-21-40.md`), **UNVERIFIED**, and they win any disagreement.

After this task a person presses `Shift+R` on an issue of theirs and an agent is working on it in a
lane of its own, with the issue pinned in its context; the row says so at once and offers to take
it back.

**What the reader sees** (05 §4.3 `Shift+R`, `05-design.md:1009`; §4.6 *Undo, revision 2*,
`:1105-1114`; §2.1 point 1, `:517-520`; §5.3's demo, `:1176`):

- **On my issue**, `Shift+R` with the list focused: the row gains a chip in place, *Starting…* as
  words (no spinner), and within the control plane's answer *Started · AU route · Undo Start*. The
  issue's state word becomes `started` when task 22's convergence writes it, through the replica.
- **On an unassigned issue**: the row's assignee becomes me first (one `optimistic.undo` write),
  then it starts. **On a triage issue of mine**: Start accepts it on the way (*accept and Start*,
  L-47), in the same one request (`accept: true`, `task-24.md:97`).
- **On Alice's issue**: nothing is sent, and the row says in place *"EP-163 is Alice's. Only her
  agents work on it."* (OD-16), for 4 s.
- **The first sign.** Ten seconds after the session's `createdAt` with no output, the chip's second
  line reads *no output yet · 10s* in mono, and the number climbs with the one heartbeat. Once the
  session record has output, it reads *last output 3s ago*. Before ten seconds it reads nothing.
- ***Undo Start*** stays for as long as the control plane allows it, **past the six seconds** of
  `--undo-window` and with no countdown. Pressed, it stops the session, removes the lane, puts the
  state back, and shows *Start undone · cost not metered in local mode* for 4 s.
- **After the agent's first file write**, pressing *Undo Start* turns the chip to *Stop and
  discard* at once, with *"Too late to undo: the agent changed a file."* Pressing *Stop and
  discard* shows the consequence line (*"Discards 3 files: upload/resume.go, …"*) and a *Discard*
  button; only that second press deletes anything.
- **Select five rows and press *Start 5 worktrees*** (task 44's bulk bar): five chips, one per row,
  each its own Start.

**Decisions made here, so nobody re-derives them:**

1. **Task 24's wire wins over the scope's working names.** The scope's `StartResult {…, startState,
   createdAt}` is not the wire; task 37 types the real one (`StartResult {issueKey, worktreeId,
   sessionId, branch, route, routeForced, undo}`, `task-37.md:235-238`). So `startState` is the
   **desktop's** state, not a field: `undoable` from the 201, `discard_only` from the first
   `409 first_write` (`task-24.md:109`). `createdAt` is the session record's (`store/types.ts:155`),
   read from the cockpit store once the poller lists the session. No field is added to `work.ts`
   (gap 1).
2. **The revision is pinned by the control plane, not sent.** Task 24 reads the head and its head
   revision in one call and returns `issueKey: "issue:is-…@r"` (`task-24.md:17-20`). The desktop
   sends `issueId`, and the test holds that it sends no revision of its own, so a stale replica can
   never pin an older text than the Hub's head.
3. **One pure rule decides what `Shift+R` does to a row**, `startRule(head, me)`, in the order:
   assigned to someone else → `not_mine`; unassigned → `assign_then_start`; mine in `triage` →
   `start` with `accept: true`; mine in `backlog` or `todo` → `start`; mine in any other state →
   `not_startable`. It mirrors task 24's refusal table (`task-24.md:121`) so no call is made that
   the route would refuse; the route still decides, and its refusal is shown if they disagree.
