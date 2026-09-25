### Task 123: read cursors for Work objects: the input to *since you last looked*

W4, stream WH; seam **SW.5** (it reads the Work feed kinds task 13 fixed and filters the feed by
scope). Depends on task 116 (`work_read_cursors`, whose `seq` only increases) and Phase T task 54
(the Desk's SINCE YOU LAST LOOKED). Hub anchors were read in the `feat/hub` worktree at `1271ddb`
(contract cited `hub-contract:N`; built `hubapi/types.go`). Phase T anchors were read on
`feat/team-mode` at `64ab2c4`: TM task 54's part and TM task 52's part (cited `TM54:N`, `TM52:N`).
The wire is task 114's part (in progress, cited `T114` by type name). Every `server/` and `store/`
path is **UNVERIFIED until merge** (rule H1).

AGENTIC-UX asks for *"A read cursor per user, per scope"*: *"Everything renders as a delta against
it. 'Mark all seen' moves it; entering a session moves it for that session only. Without this,
nothing can be marked new"* (`docs/AGENTIC-UX.md:157`). This task is the Hub half for Work objects:
a person's cursor per project, feature or issue, and a read of the change feed narrowed to one of
those scopes, so task 133 can say *what changed since you last looked* and name the issues.

Six decisions the scope left open, made here.

1. **The cursor is a feed `seq`, not a time.** AX §6.1 writes `last_seen_at`; the Hub's feed cursor
   is the org's audit sequence, *"gapless, in commit order"* (`hub-contract:1099`), so an event
   committed a moment late with an earlier clock reading is still after the cursor. A time would
   miss it. `ReadCursor.Seq` is T114's, `0` meaning *never looked*.
2. **Only forward, and never an error for trying.** `PUT` with a lower `seq` answers 200 with the
   stored cursor, as task 115's fake row says; task 116's trigger is the second guard. A `seq`
   above the org's current head is 422 `field: "seq"`: marking events that do not exist yet as seen
   would hide them when they arrive.
3. **A cursor move appends no event.** The global rule is one event per Hub write
   (`hub-contract:1133`); a cursor is the one named exception, with the Hub's own precedent: the
   janitor deletes idempotency keys with *"no event: not a record change anyone reads"*
   (`H24:97-98`, Hub task 24's part). A cursor is read by one person and says what they read; an
   event would broadcast reading habits to the team and flood the feed every time anyone looked.
   The exception is listed as a skeleton gap so the global constraint names it.
4. **Scope membership is read now.** An issue is *under* a feature when its head names that feature
   **now** (task 76's `feature_id`), not when each event was written; the Hub keeps no history of
   membership in the feed. Under a **project**: every event whose `projectId` is it. Under a
   **feature**: events whose subject is the feature, its branch brief, one of its issues, a comment
   on one of them, one of its milestones or a status update on it (task 119's `work_update_meta`),
   and evidence whose `briefKey` names its brief. Under an **issue**: the issue and its comments.
   The per-caller filter of task 13 then applies unchanged (`SubjectOnly` kinds, readable projects).
5. **A first look is bounded.** `GET /v1/changes?scope=…` with no `after` reads the scope's newest
   200 events in seq order, and `truncated: true` when older ones exist; with `after`, it pages
   forward as the Hub's feed does (limit 1-500, default 200, `more`). The route **requires**
   `scope` on this path; without it, the Hub's own feed answers as before (task 124: *"this row
   requires `scope`, so it can never be used as a second, unfiltered feed"*).
6. **The person's own, from their desktop.** No route takes a user id; a `userId` in the query is
   ignored and the cursor is always the principal's. The routes take TM task 07's `RequireDesktop`:
   an Orchestrator keeps its own read position in TM task 28's inbox, and never moves a person's.

**Phase T has a second cursor, and the two must become one.** The scope says a grep of the Phase T
**plan** finds no cursor record. Its **parts** do: TM task 52 persists `teamSeenAt`, an RFC 3339
time, in the desktop store beside the other preferences and sends it as `?since=` (`TM52:72`,
`:102`), and TM task 54's *mark all seen* calls `markTeamSeen(generatedAt)` beside W-62's control
(`TM54:57-60`); TM-54 does not claim it follows a person between devices (`TM54:222`). So there
are two: a desktop-local time for the team band, and this Hub-side `seq` per scope. At sync (rule
H2) the Phase T file wins; the proposal this part makes is that `teamSeenAt` becomes this task's
cursor on the **project** scope, so one *mark all seen* moves one record, on every device. Until
then, task 62's control calls both, and neither reads the other.

**The one thing this task must not get wrong** is showing someone a scope they cannot read by way
of its cursor or its delta. A cursor on an unreadable project that answers `seq: 0` instead of 404
confirms the project exists (`hub-contract:273`); a scope filter applied before the caller filter
leaks another team's events. `TestAnUnreadableScopeIs404` and `TestTheScopeFilterNeverWidens` hold
both.
