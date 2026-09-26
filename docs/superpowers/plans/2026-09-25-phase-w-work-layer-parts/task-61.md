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
4. **Assign first, and only then start.** On an unassigned row, `start` calls `mutate({kind:
   "setAssignee", issueId, assignee: me})` and awaits its `settled`. A `Refusal` means the replica
   has already taken the paint back (`task-56.md:378-380`); the row shows the Hub's own message and
   **no Start is sent**. `"applied"` sends the Start. If the Start is then refused, `handle.undo()`
   sends the inverse assign as the person's own write, so the row is not left assigned by a Start
   that never happened. The undo chip task 56 would show for the assign is not shown: the Start chip
   is the one chip on the row (`mutate`'s handle is held, not surfaced).
5. **The chip learns `discard_only` by asking, never by guessing.** SW.10 has no read of "has the
   agent written a file yet" (`task-24.md:92-115`; gap 2). So the chip offers *Undo Start* until a
   press is answered `409 first_write`; then it says why in place and turns to *Stop and discard*.
   Nothing polls and nothing is retried. The chip never claims undo is still possible: it offers
   the act, and the control plane's answer is the fact.
6. ***Stop and discard* is task 24's two-step discard, surfaced as `confirm.inplace`.** The first
   press sends `discardStart(id, "")`, which only asks: the `409 confirm_required` answer carries
   `details.paths` and `details.confirm` (`task-24.md:113`). The chip shows the consequence line
   from those paths and a *Discard* button; the second press sends the token. A stale token is
   answered with a fresh `confirm_required`, which the chip shows again, so the person always
   confirms what is lost **now**. `details` is `map[string]string` on the wire
   (`control-plane/internal/httpx/problem.go:33`), so the desktop reads `paths` as newline-joined
   (gap 4).
7. **The first sign reads the session record and ticks on the one clock.** `firstSign(session,
   nowMs)` is pure: Go's zero time (`0001-01-01T00:00:00Z`) or an empty `lastOutputAt` is *no
   output*; with no output and `nowMs - createdAt ≥ 10 000` it returns `no output yet · <n>s`;
   with output it returns `last output <n>s ago`; otherwise `null`. `nowMs` is `useHeartbeat()`
   (`heartbeat.ts:64`), the window's one 1 Hz tick. The chip starts no timer of its own, and the
   4 s notes (*Start undone*, the refusal words) are one `setTimeout` each, cleared on unmount.
8. **Start state is in memory and per issue.** `createStartStore()` keeps a `StartEntry` per issue
   id in a module store read with `useSyncExternalStore`, like task 24's in-memory record
   (`task-24.md:49-55`). Nothing is written to disk (Global constraints: no Hub data on disk).
   After a desktop restart the chip is gone and the session and worktree verbs in Develop remain;
   task 24 would answer `404 start_not_found` in any case. An entry in `sending` swallows a second
   `Shift+R` on that issue, so a double press is one Start even before the control plane's
   `requestId` rule (`task-24.md:51-52`) is reached. The `requestId` is `makeId("rq")`
   (`desktop/src/lib/ids.ts:12-15`, `rq-<n>-<8 hex>`), inside the wire's 8-64 rule.
9. **The base branch is the project's checked-out branch.** Task 24 needs `branch` (the base).
   The desktop sends the cockpit store's `isHead` branch of the issue's project
   (`store/types.ts:57-63`). Task 24 compares the request's `projectId` with the issue's
   (`task-24.md:256`), so the desktop sends `head.projectId`; that the control plane's project id
   and the Hub's are one value is task 24's assumption (gap 3). No head branch known: nothing is
   sent, and the row says *"No branch is checked out for this project yet."*
10. **Words for the route and the cost, and no number that was not observed.** `route` reads
    *Global route* or *AU route*; `routeForced` adds *· set by the project's data class* (task 24:
    every project is forced AU in W1, `task-24.md:512-515`). `cost: null` reads *cost not metered in
    local mode* (`OD-27`). The chip has no bar, no `%` and no countdown.

**Files** (the skeleton's File map lists `StartChip.tsx` for task 61, `:306`):

- `desktop/src/work/start/useStart.ts` — `startRule`, `firstSign`, `StartEntry`, the store,
  `useStart`, `useStartEntry` (new)
- `desktop/src/work/start/useStart.test.ts` — the rule, the sign and the store (new)
- `desktop/src/work/start/register.ts` — registers `issue.start` with task 53's `registerVerb` (new)
- `desktop/src/components/work/StartChip.tsx` — the chip, its first-sign line, the discard step (new)
- `desktop/src/components/work/StartChip.test.tsx` — keys and surfaces (new)
- `desktop/src/components/work/IssueRow.tsx` (task 40's) — renders the chip and the in-place note
  (modified)
- `desktop/src/components/work/WorkLens.tsx` (task 40's) — registers the verb on mount; passes
  `onStart` to task 44's bulk bar (modified)
- `desktop/src/components/work/work.css` (task 40's) — the `.sc-*` rules, appended (modified)
- `desktop/src/work/keys/keymap.ts` ⚑ (task 53's) — only if task 53 did not print the `Shift+R`
  row: `{verb: "issue.start", keys: ["Shift+R"], code: "KeyR", shift: true, scope: "issueRow",
  slice: "W1", label: "Start", linear: "none"}` (modified, conditional)

`desktop/src/api/work.ts` ⚑ is **not** touched: task 37 already has the three calls.

