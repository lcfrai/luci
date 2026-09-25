### Task 08: Issue heads, numbers and `project.key`: routes and store, one event per write

Stream **WH**, seam **SW.1** (the server side of the head), about 3 hours (an estimate, not a
measurement). Depends on task 07 (the guards and the matrix rows) and Hub tasks **H14** (the write
transaction: `server.Write`, `(*Tx).Emit`, idempotency, hub-contract §10.6) and **H17** (projects,
`ProjectRole`), sync-merged into `feat/work-layer` first (rule H1).

This is the first Work route task: after it, a person can create an issue, read it by id or by
number, list a project's issues, patch its priority or assignee, and give the project the key that
makes `EP-142` readable. It owns the **head**. Revisions after the first, restore and `/v1/refs`
are task 09's; state moves are task 10's.

**Eight decisions.**

1. **Numbers come from a counter row, under its row lock, inside the write transaction.** `INSERT
   … ON CONFLICT (project_id) DO UPDATE SET last_number = last_number + 1 RETURNING last_number`
   seeds the row on a project's first issue and bumps it after. A Postgres sequence is never used: a
   rolled-back create would burn its number, and `EP-7` would be missing forever. Because every Hub
   write takes the org's audit-head lock first (§10.6, `:2043-2045`), two creates in one org are
   already serialised; the counter's row lock is the second layer, and task 07's
   `guard_issue_counter` the third.
2. **A create is one record and one event.** It inserts the head at `head_revision` 0, revision 1
   with its `issue_serves` rows, and moves the head to 1, then emits **one** `issue.created`.
   hub-contract §8 says *exactly one event per changed record* (`:1133`); this task treats a new
   issue's head and first revision as one record. That reading is **UNVERIFIED** with the Hub owner
   and is asked in the WH proposal. Task 13 adds the creator's `subscription.set` in the same
   transaction; that is a second record (the subscription), so it is a second event by the same rule.
3. **The alias is computed on read, never stored or accepted.** `IssueHead.alias` is
   `hubapi.DisplayAlias(key, number)` from `project_keys` at read time, and `""` when the project
   has no key yet (the desktop then shows the number alone). No request field, query parameter or
   path segment accepts an alias; `by-number/{n}` takes the integer.
4. **The project key is a team admin's, unique in the team.** `PUT /v1/projects/{projectId}/key`
   needs `admin` (§5.3's *rename a project* row, `:350`), validates `hubapi.ValidProjectKey`, and a
   key already used by another project of the same team is 409 `project_key_taken`. Setting the
   same key again is 200 with **no** event. Changing it changes every alias and no key.
5. **The assignee is one person in the team.** `assigneeId` must be a `u-` id (`hubapi.ValidID`)
   of a user whose `team_members` row in the issue's team has role `member` or `admin`. A viewer
   cannot act on an issue (§5.3: 403 on every write), so assigning one is refused like an agent
   (`ag-…`, `OD-16`): 400 `bad_request`, `details.field = "assigneeId"`.
6. **A read of an unreadable issue is 404 `issue_not_found`, never 403 or `project_not_found`.**
   The issue is loaded under row-level security, then `ProjectRole` on its project; `ok = false`
   (another team, another org, no such issue) answers the same 404, so the route leaks neither the
   issue's existence nor its project (hub-contract §5.3's 404 columns). A member of the team who is
   refused a write gets 403, as every Hub route.
7. **A create's reply is stored for replay without its text.** `server.Write` keeps the whole
   reply in `idempotency_keys` (H14 `:111`), and an issue's reply carries its title and
   description, which erase (task 14) must be able to remove. So the create uses
   `server.WriteReplayable`, as the Hub's route 26 does for brief text (H14 `:128-134`): `Stored`
   is the reply with `revision.content` null, and the rebuild reads the issue as it is now. This is
   a second user of `WriteReplayable`; H14's comment says *"Only route 26 uses it"*, which becomes
   stale and is named in the WH proposal.
8. **Labels on create and patch wait for task 11.** `labels` must be `[]` on create and absent on a
   patch in this task (400 `bad_request`, `details.field = "labels"` otherwise). Task 11 replaces
   that one check with its label rule (`label_unavailable`) in the same two paths; nothing here
   writes `issue_labels`.

**Two rules the create needs that the scope gives to tasks 09 and 10, built here.** A create
writes revision 1 and answers a head, so it cannot wait for them:

- **`serves` is checked at the key level.** Each `briefKey` must parse (`hubapi.ParseBriefKey`),
  name a brief of the **same project**, and name a revision that exists and is not erased; each
  `criterionId` is 1-128 characters, as `evidence` (`0001_init.sql`, contract `:1744`). A
  failure is 422 `key_unresolvable` naming `content.serves[i]`. `checkServes` is declared here and
  task 09 calls the same function for every later revision. **Whether the criterion id is a
  criterion of that brief revision is not checked by the Hub**: brief content is opaque to it
  (`BriefRevision.Content json.RawMessage`, `types.go:176`), and criterion ids are positional and
  derived by phase 4's parser (provenance contract draft `:209-213`). The scope asks for that check;
  it needs a Hub-side reader of phase 4's schema that no Hub task plans. It is listed as a skeleton
  gap, and task 09's part carries the decision.
- **`threadState` is computed on every head read**, from the head revision's `issue_serves` rows
  and the briefs they name: `on_thread` when any served brief has `scope = 'branch'`,
  `project_level` when every one is `scope = 'project'`, otherwise `not_tied` with the stored
  `not_tied_reason`. A create with empty `serves` and no `notTiedReason` is 422 (05 §2.3 point 4:
  *no silent open end*), `details.field = "notTiedReason"`. `threadStateOf` is declared here; task
  10 reuses it and adds `proof` and `landCommit` beside it.

**Files:**
- `control-plane/internal/hub/store/work_issues.go` — the counter, the head, revision 1, the key,
  the reads and the list (new)
- `control-plane/internal/hub/server/work_issues.go` — the six routes, `readableIssue`,
  `issueHead`, `checkServes`, `threadStateOf` (new)
- `control-plane/internal/hub/server/work_issues_integration_test.go` — the tests below (new)
- `control-plane/internal/hub/server/server.go` ⚑ — one line, `s.registerWorkIssues()`, beside
  H17's `s.registerProjects()` (H17 `:17`) (modify)
- `control-plane/internal/hub/hubapi/work.go` ⚑ — the two codes; `IssueWithRevision`,
  `IssueQuery` and `ProjectKey` if task 03 or 04 has not declared them (modify, additive)
- `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` — `"08"` appended to
  `workMatrixLive` (task 07's file; modify, one token)

**Interfaces:**
- Produces:
  - the routes `PUT /v1/projects/{projectId}/key`, `POST` and `GET /v1/projects/{projectId}/issues`,
    `GET /v1/projects/{projectId}/issues/by-number/{n}`, `GET` and `PATCH /v1/issues/{issueId}`
  - `hubapi.CodeIssueNotFound` (`issue_not_found`, 404) and `hubapi.CodeProjectKeyTaken`
    (`project_key_taken`, 409). Task 11's part declares `CodeIssueNotFound` "if task 03 did not";
    with this task first, task 11 drops its line
  - `store.IssueRow`, `store.RevisionRow`, `store.NextIssueNumber`, `store.InsertIssue`,
    `store.InsertIssueRevision`, `store.MoveIssueHead`, `store.LockIssue`, `store.IssueByID`,
    `store.IssueByNumber`, `store.ListIssues`, `store.IssueRevisionAt`, `store.ProjectKeyOf`,
    `store.SetProjectKey`, `store.PatchIssue`, `store.Assignable` — tasks 09-15 reuse them
  - in `server`: `readableIssue`, `issueHead`, `issueWithRevision`, `checkServes`,
    `threadStateOf` — the readable-issue lookup task 11 and task 12 name, and the head builder
  - the events `issue.created`, `issue.updated`, `project.key_set`, in task 13's names (SW.5)
- Consumes:
  - task 02: `hubapi.ValidID`, `PrefixIssue`, `ValidProjectKey`, `DisplayAlias`, `ParseBriefKey`,
    `IssueRevisionEnvelope`, `IssueRevisionContent`, `IssueRevisionDigest`,
    `ValidateIssueRevisionContent`
  - task 03: `IssueHead`, `IssueRevision`, `ThreadState`, `CreateIssueRequest`,
    `PatchIssueRequest`, `SetProjectKeyRequest`, `ValidInitialState`, the enums
  - tasks 06-07: the tables, constraint names and guards; `workMatrixLive`
  - H14: `Write`, `WriteReplayable`, `Read`, `Tx`, `Emit`, `Refuse`, `Forbidden`, `Decode`
  - H17: `ProjectRole` (H17 `:48`); H15: `Require` (H15 `:48`); H08: `ids.Wide`
- Requires of later tasks:
  - **Task 09** calls `checkServes` and `store.InsertIssueRevision` for every revision after the
    first; it does not write a second copy of either.
  - **Task 10** calls `threadStateOf` and fills `proof` and `landCommit` in `issueHead`.
  - **Task 11** replaces decision 8's check; **task 13** adds the subscription writes to the
    create and patch paths, in the same transaction.
