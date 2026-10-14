### Task 98: Phase T delta — `task.issueId`; the `issue.delegated` and `blocker.landed` events

**Slice:** W3. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 70; TM-03, TM-06, TM-10. **Seam:** **SW.5** (one new Work feed kind appended; the
route is task 100's).

This is a **Phase T delta**: every file it touches belongs to Phase T, so under rule H2 the owner
phase's file wins and nothing here is applied until `feat/team-mode` is sync-merged into
`feat/work-layer`. `OD-25` forbids a Phase W task editing a Phase T *task*; this extends Phase T's
**merged files**, additively, and it is written as a diff against them, not as a rewrite. Until
that merge every Go name below is **UNVERIFIED**, and Step 1 is *re-read the merged file first*.

What it buys: 05 §3.4's sentence — a task's reason now reads **task → issue → criterion →
brief** — and the two Orchestrator events that sentence implies: *an issue was delegated to you*,
and *a blocker you were waiting on landed*.

**Anchors, checked on 2026-09-27.** Phase T plan on `feat/team-mode` at `f4b6a7d5`:
`docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:445-453` (TM-03: `hubapi/team.go` beside
`types.go`, prefixes `dl ag tk ls cf th ms es dv ot hm nt` through `ids.Wide`, new codes in
`codes.go` including `agent_not_owner`, golden JSON per type, *"Does not add routes"*) and
`:477-486` (TM-06: `POST /v1/projects/{p}/tasks`, `PATCH …/tasks/{tk}` — *"state, scope, reason;
owner only"* — each write *"one metadata-only audit event and a feed row"*, tests per route for
owner, other member, viewer, and Fleet with and without a delegation). Phase W on
`feat/work-layer`: task 13's `EventIssueState = "issue.state"` (`task-13.md:125`), its
`IssueStateData` (`:159-162`) and its registry row
`{EventIssueState, IssueStateData{}, ScopeProject, VisibleToReaders}` (`:255`); task 13's rule
that a kind's `data` is exactly its registered type (`:329`); task 105's note that
`control-plane/internal/fleet/delegation/feed.go` is TM-11's and *"task 98 maps its kinds here"*
(`task-105.md:62`); task 111's note that an added field on `hubapi/team.go` is *"as task 98 adds
`issueId`"* (`task-111.md:338`).
**UNVERIFIED, because they are not on this branch:** every path under
`control-plane/internal/hub/hubapi/team.go`, `control-plane/internal/fleet/model/`,
`control-plane/internal/fleet/store/` and `control-plane/internal/fleet/delegation/feed.go`;
Phase T's migration numbering; TM-05's coalescing window. Step 1 checks each and corrects this
part's names in the commit that applies it.

#### Five decisions the scope left open, made here

**(a) `issueId` is optional for ever, not "optional for now".** A task with no issue is the
normal case in a solo project and in Phase T's own fixtures. The field is `*string`, omitted when
nil, and TM1's golden JSON for a task without one must not change by a byte. Anything stronger
breaks Phase T's merged tests, which is exactly what rule H2 forbids.

**(b) Same project, checked in Go and again in the database.** A task's `project_id` and its
issue's `project_id` must match. Go returns 422 `issue_not_assignee`'s sibling
`issue_wrong_project`; the database holds a composite foreign key so a handler bug cannot store
the pair. This is hub-contract §10.7's rule (*every rule twice*), and it is why the migration
carries `(project_id, issue_id)` rather than `issue_id` alone.

**(c) Only the issue's assignee may bind a task to it.** `OD-16` and L-39 (*"Nobody can delegate
to another person's agent"*) make the check: the task's owner must be the issue's `assigneeId` at
the moment of the write. Not a team member, not the issue's creator, not an org admin. The code is
`issue_not_assignee`, added to `codes.go` beside TM-03's six. A later reassignment does **not**
retroactively unbind a task: the Hub records what was true at the write, and the desktop shows the
task's issue with the assignee's name as of then (task 102's surface, not this task's).

**(d) `issue.delegated` is a new Work feed kind; `blocker.landed` is not a Hub kind at all.**
`issue.delegated` is appended to SW.5's registry (task 13) with `data {issueId, revision,
ownerId}` — ids only, no title (skeleton rule 6; hub-contract §8). `blocker.landed` is **derived
in Fleet** from task 13's existing `issue.state` kind: a row whose `data.to` is `landed` on an
issue that `blocks` an issue bound to one of the reader's tasks. No Hub change, no second source
of truth for "landed", and no risk of the two disagreeing. Fleet's model and store gain **two**
kinds because Fleet's own event model is what an Orchestrator reads; only one of them is a Hub
kind.

**(e) Both are non-thread events, so both coalesce.** TM-05 §4.2's window (up to 60 s) applies:
five blockers landing in the same minute reach the Orchestrator as one event with a list of ids,
not five wake-ups. A thread event would not coalesce; neither of these is one. If the merged
TM-05 names a different window, the merged number wins and this part's `60 * time.Second` becomes
a reference to its constant.

#### The files this task touches

| File | Owner | What this task adds |
|---|---|---|
| `control-plane/internal/hub/hubapi/team.go` ⚑ | Phase T (TM-03) | `IssueID *string` on `Task`; nothing else |
| `control-plane/internal/hub/hubapi/codes.go` ⚑ | Phase T (TM-03) | `CodeIssueNotAssignee`, `CodeIssueWrongProject` |
| `control-plane/internal/hub/hubapi/events.go` ⚑ | Phase W (task 13) | `EventIssueDelegated`, `IssueDelegatedData`, one registry row |
| `control-plane/internal/hub/server/team_tasks.go` ⚑ | Phase T (TM-06) | the two checks on create and patch |
| `control-plane/internal/hub/store/team_tasks.go` ⚑ | Phase T (TM-06) | `issue_id` written and read |
| `migrate/sql/NNNN_work_team_link.sql` | this task | the column, its composite FK, its index |
| `control-plane/internal/fleet/model/events.go` ⚑ | Phase T (TM-09) | two Fleet event kinds and their payloads |
| `control-plane/internal/fleet/store/events.go` ⚑ | Phase T (TM-10) | persistence and the per-owner query |
| `control-plane/internal/fleet/delegation/feed.go` ⚑ | Phase T (TM-11) | the mapping, including the derivation |

Tests sit beside each: `hubapi/team_golden_test.go` (TM-03's, extended), a new
`hubapi/events_issue_delegated_test.go`, `server/team_tasks_test.go` (TM-06's, extended), and a
new `fleet/delegation/feed_work_test.go`.

#### Interfaces — complete: the Hub wire delta

```go
// control-plane/internal/hub/hubapi/team.go — TM-03's Task, one field added.
// The task's reason reads task -> issue -> criterion -> brief (05 §3.4). Nil is normal.
type Task struct {
	// … TM-03's fields, unchanged, in their order …
	IssueID *string `json:"issueId,omitempty"` // W3 (task 98): the issue this task serves
}

// control-plane/internal/hub/hubapi/codes.go — beside TM-03's six.
const (
	CodeIssueNotAssignee  = "issue_not_assignee"  // 403: the task's owner is not the issue's assignee
	CodeIssueWrongProject = "issue_wrong_project" // 422: the issue belongs to another project
)
```

```go
// control-plane/internal/hub/hubapi/events.go — SW.5, W3's first kind (task 13's registry).
const EventIssueDelegated = "issue.delegated"

// IssueDelegatedData is metadata only: ids and a revision, never a title or a description
// (skeleton rule 6; hub-contract §8). The desktop and Fleet read the issue itself.
type IssueDelegatedData struct {
	IssueID  string `json:"issueId"`
	Revision int    `json:"revision"` // the issue's revision at delegation
	OwnerID  string `json:"ownerId"`  // the assignee whose Orchestrator is delegated to
}

// Appended to WorkEventKinds beside task 13's W1 rows:
//   {EventIssueDelegated, IssueDelegatedData{}, ScopeProject, VisibleToReaders},
```

#### Interfaces — complete: `migrate/sql/NNNN_work_team_link.sql`

`NNNN` is the next free number **at sync time**, after Phase T's `00NN_team.sql`; the file is
renamed in the commit that applies it (rule H1: additive only). Written to be re-runnable so a
re-ordered merge cannot break it.

```sql
-- NNNN_work_team_link.sql — W3 (task 98): a team task may serve one Work issue.
-- Phase T owns `hub.tasks`; Phase W owns `hub.issues`. This is the only edge between them.
BEGIN;

ALTER TABLE hub.tasks
  ADD COLUMN IF NOT EXISTS issue_id text;

-- Decision (b): same project, held in the database as well as in Go. `hub.issues` carries a
-- UNIQUE (project_id, id) for exactly this (task 06's migration); if it does not, add it here.
ALTER TABLE hub.tasks
  ADD CONSTRAINT tasks_issue_same_project
  FOREIGN KEY (project_id, issue_id)
  REFERENCES hub.issues (project_id, id)
  ON DELETE SET NULL
  DEFERRABLE INITIALLY IMMEDIATE;

-- The read Fleet makes on every turn: "the tasks of this owner bound to an issue".
CREATE INDEX IF NOT EXISTS tasks_issue_idx
  ON hub.tasks (issue_id)
  WHERE issue_id IS NOT NULL;

COMMIT;
```

`ON DELETE SET NULL` and not `CASCADE`: an erased issue (task 14) must not delete a team's task
record. The task stays, its `issueId` becomes null, and the Desk shows it unbound.

#### Interfaces — complete: the Fleet side

```go
// control-plane/internal/fleet/model/events.go — TM-09's kinds, two added.
const (
	EventIssueDelegated = "issue.delegated" // from the Hub feed, one row
	EventBlockerLanded  = "blocker.landed"  // derived here; the Hub has no such kind
)

// IssueDelegatedEvent mirrors the Hub's data. No title: the Orchestrator reads the issue.
type IssueDelegatedEvent struct {
	IssueID  string `json:"issueId"`
	Revision int    `json:"revision"`
	OwnerID  string `json:"ownerId"`
}

// BlockerLandedEvent is derived from one or more `issue.state` rows with To == "landed".
// Coalesced (decision (e)), so it carries a list, and names the waiting task.
type BlockerLandedEvent struct {
	TaskID    string   `json:"taskId"`    // the reader's task that was waiting
	IssueID   string   `json:"issueId"`   // the task's issue
	BlockerIDs []string `json:"blockerIds"` // the issues that landed, ids only
}
```

```go
// control-plane/internal/fleet/delegation/feed.go — TM-11's reader, the Work mapping.
// It runs per org, with the reader's own delegation, so the Hub's read filter has already
// removed anything this owner cannot see. It never widens that filter.
func (r *Reader) mapWork(row hubapi.FeedRow, owner string, bound map[string]string) []model.Event {
	switch row.Kind {
	case hubapi.EventIssueDelegated:
		var d hubapi.IssueDelegatedData
		if err := json.Unmarshal(row.Data, &d); err != nil || d.OwnerID != owner {
			return nil // not mine: one delegation, one Orchestrator (OD-16, L-39)
		}
		return []model.Event{{Kind: model.EventIssueDelegated, Data: model.IssueDelegatedEvent(d)}}

	case hubapi.EventIssueState:
		var d hubapi.IssueStateData
		if err := json.Unmarshal(row.Data, &d); err != nil || d.To != "landed" {
			return nil
		}
		// Derivation (decision (d)): did this landing unblock an issue one of my tasks serves?
		taskID, issueID, ok := bound[d.IssueID] // built from `blocks` edges + this owner's tasks
		if !ok {
			return nil
		}
		return []model.Event{{Kind: model.EventBlockerLanded, Coalesce: true,
			Data: model.BlockerLandedEvent{TaskID: taskID, IssueID: issueID,
				BlockerIDs: []string{d.IssueID}}}}
	}
	return nil
}
```

`bound` is built once per read from the owner's tasks with an `issueId` and task 11's `blocks`
edges on those issues. It is ids only, and it is rebuilt each read rather than cached, so a
relation removed between reads cannot resurrect an event.

#### Tests to write first, each with the mutant it kills

| # | Test | Mutant it kills |
|---|---|---|
| 1 | `TestTaskIssueWrongProject` — create a task whose `issueId` names another project's issue → 422 `issue_wrong_project`; the row is absent | a bare foreign key on `issue_id` with no project check |
| 2 | `TestTaskIssueNotAssignee` — Dylan's Orchestrator binds its task to Alice's issue → 403 `issue_not_assignee` | any member's issue accepted, breaking `OD-16` and L-39 |
| 3 | `TestBlockerLandedOnce` — one landing, three Orchestrators on the team, one waiting task → exactly one event, to that owner | broadcast to every Orchestrator on the team |
| 4 | `TestFeedNeverLeaksUnreadable` — an `issue.delegated` on a project the reader cannot read → no event | the read filter skipped in `mapWork` |
| 5 | `TestEventPayloadsAreIdsOnly` — reflect over both payload types: every field is an id, a revision or a list of ids | a title added to `data` |
| 6 | `TestTaskGoldenUnchanged` — TM1's task fixture without `issueId` round-trips byte for byte | a required field that breaks Phase T's fixtures |
| 7 | `TestBlockerLandedCoalesces` — five blockers land in 10 s → one event with five `blockerIds` | one wake-up per landing (TM-05 §4.2) |
| 8 | `TestIssueDelegatedRegistered` — task 13's registry check sees the new kind with its data type | a kind appended without its row, so `data` is unchecked |

Test 5 is written with reflection, not a hand-written list, so a field added later fails it:
`for each field: name in {issueId, ownerId, taskId, revision, blockerIds}` or the test fails.

#### Steps

1. **Only after the sync-merge.** `feat/team-mode` is merged into `feat/work-layer` first (the
   phase's sync-merge rule). Re-read the merged `hubapi/team.go`, `codes.go`, TM-06's task routes,
   `fleet/model`, `fleet/store`, `fleet/delegation/feed.go` and TM-05's coalescing constant, and
   correct every name in this part. If the merged shape differs, the merged shape wins.
2. **Tests 6 and 1 first** — the golden round-trip (which must pass before any change) and the
   wrong-project refusal (which must fail). That order proves the harness before it proves the
   feature.
3. **The field, then the code constants.** `IssueID *string` on `Task`; the two codes. Run TM-03's
   golden tests: unchanged bytes for a task without an issue.
4. **The migration**, numbered at merge, applied to the dev database, then rolled back and applied
   again to prove it is re-runnable.
5. **The two checks in TM-06's handlers**, on create and on patch, in that order, with tests 1 and
   2 going green one at a time.
6. **The Hub kind**: `EventIssueDelegated`, its data type, its registry row, test 8.
7. **Fleet**: the two model kinds, the store rows, then `mapWork` with tests 3, 4, 5 and 7.
8. **Record the deltas** in the phase's Hub-delta list so task 100 and task 113 can cite them.

#### Acceptance

```bash
# 1. Phase T's fixtures still pass, byte for byte.
go test ./control-plane/internal/hub/hubapi/... -run 'Golden|Task' -count=1
# expect: ok, and "TestTaskGoldenUnchanged" among the run tests

# 2. The two refusals, with their codes.
go test ./control-plane/internal/hub/server/... -run 'TaskIssue' -race -count=1
# expect: ok; the test asserts 422 issue_wrong_project and 403 issue_not_assignee

# 3. The database holds the rule too: Go bypassed, the insert still fails.
psql "$HUB_DSN" -c "insert into hub.tasks (id, project_id, issue_id, owner_user_id) \
  values ('tk-x','pr-a','is-in-project-b','us-1');"
# expect: ERROR: insert or update on table "tasks" violates foreign key constraint
#         "tasks_issue_same_project"

# 4. Fleet: one landing, one event, one owner.
go test ./control-plane/internal/fleet/... -run 'Blocker|Delegated|IdsOnly' -race -count=1
# expect: ok; 5 tests

# 5. No title, no description, anywhere in either payload.
go doc ./control-plane/internal/fleet/model BlockerLandedEvent
go doc ./control-plane/internal/hub/hubapi IssueDelegatedData
# expect: no field named Title, Name, Summary, Description or Text in either
```

#### What this task does NOT claim

- **What an Orchestrator does with either event.** The turn loop, the instructions and the
  decision to act are Phase T's (TM-26, TM-41). This task delivers two events; it does not read
  them.
- **The delegate key or its desktop surface.** `POST /v1/issues/{issueId}/delegate`, the route
  that appends `issue.delegated`, is **task 100's**; the *Delegate* control and its Undo are task
  102's. Without them this task's kind is registered and never appended.
- **That an issue's `blocks` edges are complete.** `bound` is built from task 11's relations; a
  blocker nobody recorded produces no event, and that is correct, not a bug.
- **Any Phase T task's content.** `OD-25`: no Phase W task edits a Phase T task. These are file
  deltas against Phase T's merged files, listed as such, and the Phase T owner may refuse any of
  them at merge.
- **That the routes accept `issueId` today.** TM-06's handlers gain the checks here; until the
  sync-merge the field exists in this part only, and the whole part is **UNVERIFIED**.
- **The 60-second window as a measurement.** It is TM-05's number, cited, not observed here.

#### Gaps found while writing this part

1. **The scope names a code `issue_not_assignee` and no code for the project check.** This part
   adds `issue_wrong_project`; task 100's problem-code list and task 113's W3 rows must carry it,
   or the refusal has no name on the wire.
2. **`hub.issues` may have no `UNIQUE (project_id, id)`.** Task 06's migration is cited from
   `task-06.md:291-316` for comments, not for that key. If it is absent, this migration adds it,
   and that is a change to a Phase W-owned table by a Phase T-delta task: the skeleton should say
   which task owns it. Recorded **UNVERIFIED**.
3. **Fleet's event model has no coalescing flag in any part read here.** `Coalesce: true` in
   `mapWork` is this part's proposal; TM-05 §4.2 describes the behaviour, TM-09's struct may not
   have the field. Step 1 checks, and if it is absent the field is added there — another Phase T
   delta, which this part's row in the delta list must name.
4. **No task owns the phase's Hub-delta list.** Step 8 writes to it; the skeleton's *Hub deltas*
   tasks (99, 100) each carry their own. A single list, cited by 100 and 113, would stop three
   parts re-deriving the same set.
5. **`issue.delegated` needs a matching `issue.undelegated`.** Task 100's part (decision (f))
   already adds a withdraw route and that kind. This part must map it in `mapWork` too, so Fleet
   stops what the delegation started. It is listed here rather than silently left to task 100:
   **this part's mapping is incomplete until task 100's withdraw kind is named**.
6. **The skeleton's dependency row for task 99 says "97, 98".** Task 97 is W2's results file and
   has no code task 99 needs; the real dependency is 98 alone, with 97 only as an ordering
   constraint. Worth correcting so W3's wire is not blocked behind a fortnight-long run.
