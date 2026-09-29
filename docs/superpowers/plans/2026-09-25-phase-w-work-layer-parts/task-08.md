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
  failure is 422 `key_unresolvable` naming `content.serves[i]` (`serves[i]` on a create, whose
  body has no `content`). `checkServes` is declared here and
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

#### Wire shapes

```jsonc
// PUT /v1/projects/pj-…/key                     admin; 200 (same key again: 200, no event)
{ "key": "EP" }
// → hubapi.ProjectKey
{ "projectId": "pj-…", "key": "EP", "setAt": "2026-09-25T10:02:11.000Z", "setBy": { "id": "u-…", "displayName": "Grace", "email": "…" } }
// 409 when another project of the team holds it
{ "error": "project_key_taken", "message": "Another project in this team already uses EP.",
  "details": { "key": "EP", "projectId": "pj-…" } }

// POST /v1/projects/pj-…/issues                 member; 201; Idempotency-Key honoured
{ "title": { "text": "Resume a dropped upload", "author": "H" },
  "description": { "text": "", "author": "H" },
  "serves": [ { "briefKey": "brief:bf-3f2a91c07b1e4d2a@6", "criterionId": "done:2" } ],
  "state": "todo", "priority": "high", "assigneeId": "u-…", "labels": [], "notTiedReason": "" }
// → hubapi.IssueWithRevision
{ "head": { "id": "is-…", "projectId": "pj-…", "number": 142, "alias": "EP-142", "state": "todo",
            "stateReason": "", "assignee": { "id": "u-…", "displayName": "Linus", "email": "…" },
            "priority": "high", "labels": [], "relations": [], "headRevision": 1,
            "threadState": { "kind": "project_level", "reason": "" }, "source": "person",
            "external": null, "createdAt": "…", "createdBy": { … }, "updatedAt": "…" },
  "revision": { "issueId": "is-…", "revision": 1, "key": "issue:is-…@1", "digest": "sha256:…",
                "content": { "title": {…}, "description": {…}, "serves": [ … ], "implements": [] },
                "writtenAt": "…", "writtenBy": { … }, "restoredFrom": null } }

// GET /v1/projects/pj-…/issues?state=todo&state=started&assignee=u-…&label=lb-…&after=&limit=50
// viewer; Page[IssueHead], number descending; assignee=none selects unassigned
// GET /v1/projects/pj-…/issues/by-number/142     viewer; IssueWithRevision (the head revision)
// GET /v1/issues/is-…                            viewer; IssueWithRevision; 404 issue_not_found

// PATCH /v1/issues/is-…                          member; 200 IssueHead; no revision is written
{ "priority": "urgent", "assigneeId": null }      // null clears the assignee; an absent field is kept
// a body naming "state" → 400 bad_request (PatchIssueRequest has no state field: task 03)
```

`PatchIssueRequest.assigneeId` needs three meanings: absent (keep), `null` (clear), a `u-` id
(set). A Go `*string` cannot tell absent from `null`, so the handler decodes the body twice: once
into `PatchIssueRequest` (strict, which refuses `state`) and once into a `map[string]json.RawMessage`
to learn which keys were present. This is the one place in W1 that needs it; if task 03 declares a
tri-state type instead, the second decode is dropped.

#### `hubapi/work.go`, the additions (complete)

```go
// ── Task 08: heads, numbers and the project key ──────────────────────────────
// Appended to work.go (task 03's file). Additive only. A declaration task 03 or 04
// already made is dropped here, and its spelling wins.

const (
	CodeIssueNotFound   = "issue_not_found"   // 404, details.issueId
	CodeProjectKeyTaken = "project_key_taken" // 409, details.key, details.projectId
)

// IssueWithRevision is a head with its head revision, content included, so a
// client can re-hash it (hub-contract §3.4). Task 16 reads it as {Head, Revision}.
type IssueWithRevision struct {
	Head     IssueHead     `json:"head"`
	Revision IssueRevision `json:"revision"`
}

// IssueQuery is GET …/issues' filter. A zero field does not filter.
type IssueQuery struct {
	States   []IssueState // ?state=, repeatable; each must be a known state
	Assignee string       // ?assignee=u-… or "none"
	Label    string       // ?label=lb-…
}

// ProjectKey is the project's display key and who set it last.
type ProjectKey struct {
	ProjectID string `json:"projectId"`
	Key       string `json:"key"`
	SetAt     string `json:"setAt"`
	SetBy     User   `json:"setBy"`
}

// Work event kinds this task emits (task 13 moves them to workevents.go, SW.5).
const (
	EventIssueCreated  = "issue.created"   // {number, key, digest, state, source}
	EventIssueUpdated  = "issue.updated"   // {fields}: comma-joined field names
	EventProjectKeySet = "project.key_set" // {key, previousKey}
)
```

#### `store/work_issues.go` (the contract; bodies are one statement each unless noted)

```go
package store

// IssueRow is one issues row. Empty strings stand for NULL text columns.
type IssueRow struct {
	ID, OrgID, ProjectID, TeamID string
	Number                        int
	State                         hubapi.IssueState
	StateReason                   string
	AssigneeUserID                string
	Priority                      hubapi.Priority
	HeadRevision                  int
	NotTiedReason                 hubapi.NotTiedReason
	Source                        hubapi.IssueSource
	ExternalSource, ExternalID    string
	ExternalURL                   string
	CreatedAt, UpdatedAt          time.Time
	CreatedBy                     string
	ErasedAt                      *time.Time
}

// RevisionRow is one issue_revisions row with its serves (read from issue_serves).
type RevisionRow struct {
	OrgID, IssueID, ProjectID string
	Revision                  int
	Canonical                 []byte // nil when erased
	Digest                    hubapi.Digest
	WrittenAt                 time.Time
	WrittenBy                 string
	RestoredFrom              *int
	ErasedAt                  *time.Time
	Serves                    []hubapi.Serve
}

// IssueFilter is hubapi.IssueQuery after validation.
type IssueFilter struct {
	States     []hubapi.IssueState
	AssigneeID string // "" = any
	Unassigned bool
	LabelID    string
}

// NextIssueNumber:
//   INSERT INTO project_issue_counters (org_id, project_id, last_number) VALUES ($1, $2, 1)
//   ON CONFLICT (project_id) DO UPDATE SET last_number = project_issue_counters.last_number + 1
//   RETURNING last_number
func NextIssueNumber(ctx context.Context, tx pgx.Tx, org, projectID string) (int, error)

// InsertIssue writes the head at head_revision 0.
func InsertIssue(ctx context.Context, tx pgx.Tx, r IssueRow) error

// InsertIssueRevision writes the revision, then one issue_serves row per r.Serves
// entry (duplicates in Serves are refused by the caller, not collapsed here).
func InsertIssueRevision(ctx context.Context, tx pgx.Tx, r RevisionRow) error

// MoveIssueHead: UPDATE issues SET head_revision = $2, updated_at = $3 WHERE id = $1.
func MoveIssueHead(ctx context.Context, tx pgx.Tx, issueID string, to int, at time.Time) error

// LockIssue: SELECT … FROM issues WHERE id = $1 FOR UPDATE (hub_app holds column
// UPDATE on issues, which FOR UPDATE needs). found=false for an id row-level
// security hides; the caller cannot tell that from absent, by design.
func LockIssue(ctx context.Context, tx pgx.Tx, id string) (r IssueRow, found bool, err error)
func IssueByID(ctx context.Context, q Querier, id string) (IssueRow, bool, error)
func IssueByNumber(ctx context.Context, q Querier, projectID string, n int) (IssueRow, bool, error)

// ListIssues is keyset by number descending; the cursor is the last number as a
// decimal string, opaque to callers. Limit 1-200, default 50 (the Hub's PageRequest).
func ListIssues(ctx context.Context, q Querier, projectID string, f IssueFilter, pr hubapi.PageRequest) (rows []IssueRow, next *string, err error)

// IssueRevisionAt reads one revision with its serves.
func IssueRevisionAt(ctx context.Context, q Querier, issueID string, rev int) (RevisionRow, bool, error)

// ProjectKeyOf: found=false when the project has no key yet.
func ProjectKeyOf(ctx context.Context, q Querier, projectID string) (key string, setAt time.Time, setBy string, found bool, err error)

// SetProjectKey upserts the key. It returns the previous key ("" for none) and
// changed=false when the key was already this one. 23505 on
// project_keys_team_key_key is ErrProjectKeyTaken with the holder's project id.
func SetProjectKey(ctx context.Context, tx pgx.Tx, org, teamID, projectID, key, by string, at time.Time) (previous string, changed bool, err error)

var ErrProjectKeyTaken = errors.New("store: project key taken in this team")

// IssuePatch holds only the fields a PATCH named.
type IssuePatch struct {
	Priority      *hubapi.Priority
	SetAssignee   bool   // the body named assigneeId
	AssigneeID    string // "" clears
	NotTiedReason *hubapi.NotTiedReason
}

// PatchIssue writes the named fields and updated_at, and returns the names of the
// fields whose value changed (camelCase wire names), empty for a no-op.
func PatchIssue(ctx context.Context, tx pgx.Tx, id string, p IssuePatch, at time.Time) (changed []string, err error)

// Assignable: a team_members row for (teamID, userID) with role member or admin.
func Assignable(ctx context.Context, q Querier, teamID, userID string) (bool, error)
```

`Querier` is whatever Hub task 08's `store.go` names the common interface of `pgx.Tx` and
`*pgxpool.Pool`, as task 11's part says; the name is **UNVERIFIED** until the Hub's store lands.

#### `server/work_issues.go` (the routes; the create written out whole)

```go
package server

// registerWorkIssues is called once from server.go, beside registerProjects. It
// registers through the chassis's recorder (H08), so (*Server).Patterns() lists
// every route and task 07's meta-tests see them.
func (s *Server) registerWorkIssues() {
	s.route("PUT /v1/projects/{projectId}/key", s.putProjectKey)
	s.route("POST /v1/projects/{projectId}/issues", s.createIssue)
	s.route("GET /v1/projects/{projectId}/issues", s.listIssues)
	s.route("GET /v1/projects/{projectId}/issues/by-number/{n}", s.issueByNumber)
	s.route("GET /v1/issues/{issueId}", s.getIssue)
	s.route("PATCH /v1/issues/{issueId}", s.patchIssue)
}

func (s *Server) createIssue(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	s.WriteReplayable(w, r, httpx.MaxBodyBytes, s.rebuildIssue, func(tx *Tx, body []byte) (WriteResult, error) {
		ctx := r.Context()
		var req hubapi.CreateIssueRequest
		if err := Decode(body, &req); err != nil {
			return WriteResult{}, err
		}
		role, teamID, ok, err := ProjectRole(ctx, tx, projectID)
		if err != nil {
			return WriteResult{}, err
		}
		if err := Require(role, ok, hubapi.RoleMember, notFoundProject(projectID)); err != nil {
			return WriteResult{}, err
		}
		content, state, err := s.checkCreate(ctx, tx, teamID, projectID, &req) // decisions 5, 8; serves; thread
		if err != nil {
			return WriteResult{}, err
		}
		n, err := store.NextIssueNumber(ctx, tx, tx.P.OrgID, projectID)
		if err != nil {
			return WriteResult{}, err
		}
		id := ids.Wide(hubapi.PrefixIssue)
		head := store.IssueRow{ID: id, OrgID: tx.P.OrgID, ProjectID: projectID, TeamID: teamID, Number: n,
			State: state, AssigneeUserID: deref(req.AssigneeID), Priority: req.Priority,
			NotTiedReason: req.NotTiedReason, Source: hubapi.SourcePerson,
			CreatedAt: tx.Now, UpdatedAt: tx.Now, CreatedBy: tx.P.UserID}
		if err := store.InsertIssue(ctx, tx, head); err != nil {
			return WriteResult{}, err
		}
		env := hubapi.IssueRevisionEnvelope{V: 1, Kind: "issue-revision", IssueID: id, Revision: 1,
			ProjectID: projectID, Content: content}
		digest, canonical, err := hubapi.IssueRevisionDigest(env)
		if err != nil {
			return WriteResult{}, err
		}
		if err := store.InsertIssueRevision(ctx, tx, store.RevisionRow{OrgID: tx.P.OrgID, IssueID: id,
			ProjectID: projectID, Revision: 1, Canonical: canonical, Digest: digest, WrittenAt: tx.Now,
			WrittenBy: tx.P.UserID, Serves: content.Serves}); err != nil {
			return WriteResult{}, err
		}
		if err := store.MoveIssueHead(ctx, tx, id, 1, tx.Now); err != nil {
			return WriteResult{}, err
		}
		tx.Emit(Event{Kind: hubapi.EventIssueCreated, TeamID: teamID, ProjectID: projectID, Subject: id,
			Data: map[string]any{"number": int64(n), "key": hubapi.IssueKey{IssueID: id, Revision: 1}.String(),
				"digest": digest.String(), "state": string(state), "source": string(hubapi.SourcePerson)}})
		out, err := s.issueWithRevision(ctx, tx, id, 1)
		if err != nil {
			return WriteResult{}, err
		}
		stored := out
		stored.Revision.Content = nil // decision 7: no issue text in idempotency_keys
		return WriteResult{Status: http.StatusCreated, Body: out, Stored: stored}, nil
	})
}

// rebuildIssue answers a replay from what the org holds now: the stored reply's
// issue id and revision, re-read, so an erased issue replays with content null.
func (s *Server) rebuildIssue(tx *Tx, stored []byte) (any, error)

func (s *Server) putProjectKey(w http.ResponseWriter, r *http.Request)   // admin; decision 4
func (s *Server) listIssues(w http.ResponseWriter, r *http.Request)      // s.Read; viewer
func (s *Server) issueByNumber(w http.ResponseWriter, r *http.Request)   // s.Read; viewer; n ≥ 1 else 400
func (s *Server) getIssue(w http.ResponseWriter, r *http.Request)        // s.Read; readableIssue(viewer)
func (s *Server) patchIssue(w http.ResponseWriter, r *http.Request)      // s.Write; readableIssue(member)
```

The helpers later Work tasks call, in the same file:

```go
// readableIssue loads an issue under row-level security, then ProjectRole on its
// project, and Requires want. Unreadable, absent and another org's are one 404
// issue_not_found (decision 6). lock=true reads it FOR UPDATE (every write path).
func readableIssue(ctx context.Context, tx *Tx, id string, want hubapi.Role, lock bool) (store.IssueRow, hubapi.Role, error)

// issueHead assembles the wire head: alias from project_keys, the assignee and
// creator as hubapi.User, labels (issue_labels ids, [] until task 11 writes them),
// relations ([] until task 11's store.RelationsOf), threadState (threadStateOf),
// external (nil unless external_source is set). Task 10 adds proof and landCommit.
func (s *Server) issueHead(ctx context.Context, q store.Querier, r store.IssueRow) (hubapi.IssueHead, error)

// issueWithRevision is issueHead plus revision rev decoded from its canonical bytes.
func (s *Server) issueWithRevision(ctx context.Context, q store.Querier, id string, rev int) (hubapi.IssueWithRevision, error)

// checkServes is the key-level rule above. field is the JSON path prefix used in
// the 422, "content.serves" here and in task 09, "serves" on create.
func checkServes(ctx context.Context, q store.Querier, projectID string, serves []hubapi.Serve, field string) error

// threadStateOf computes the head's thread state from its serves and their briefs.
func threadStateOf(ctx context.Context, q store.Querier, r store.IssueRow, serves []hubapi.Serve) (hubapi.ThreadState, error)
```

`s.route` stands for the recorder H17's `registerProjects` uses; its real name is **UNVERIFIED**
until the Hub's chassis (H08) is on the sync base, and the implementer uses that name. `deref`,
`notFoundProject` and `hubapi.SourcePerson` are one-liners, or task 03's spellings.

#### Tests to write first — `server/work_issues_integration_test.go` (`//go:build integration`)

Against the compose stack, through `devissuer` persons (Grace admin, Linus member, Vera viewer, as
H16b's fixture). Every event count reads `audit_events` for the org.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTwentyConcurrentCreatesAreGapless` | 20 goroutines create in one project → numbers exactly 1..20, no repeat | a counter read outside the transaction |
| `TestAFailedCreateBurnsNoNumber` | create 1; a create whose `serves` names another project's brief → 422 `key_unresolvable`, `details.field = "serves[0]"`; the next create is number 2 | a Postgres sequence; a counter bumped in its own transaction |
| `TestARetriedCreateIsOneIssue` | the same `Idempotency-Key` twice → same `head.id`, `Idempotent-Replay: true` on the second, one `issue.created` | a double create on retry |
| `TestAReplayHoldsNoText` | after a create with a canary title, `idempotency_keys.response` for that key does not contain the canary; the replay's body does (rebuilt) | the whole reply stored, which erase could not reach (decision 7) |
| `TestOneEventPerWriteAndNoText` | create, patch, key set: exactly one `issue.created`, one `issue.updated` (`fields` = `"priority"`), one `project.key_set`; no event `data` contains the title or description canary | head and revision as two events; content in the feed (hub-contract `:1133-1134`) |
| `TestAnAssigneeIsAPersonInTheTeam` | `assigneeId` = an `ag-…` id, Out Sider's id, Vera's id → 400, `details.field = "assigneeId"`; Linus's → 201 with `assignee.id` his | an agent as assignee (`OD-16`); a viewer who cannot act |
| `TestTheAliasFollowsTheKey` | key `EP`; issue 142 reads `EP-142`; key moved to `PAY` → `by-number/142` and `GET /v1/issues/{id}` read `PAY-142`, `revision.key` unchanged; no key set → `alias: ""` | a stored alias |
| `TestAKeyIsUniqueInTheTeam` | two projects of T: `EP` on the first, `EP` on the second → 409 `project_key_taken` with `details.projectId` the first; `EP` in another team → 200; `ep`, `E`, `EPICS1` → 400 | a key unique per project only; a loosened grammar |
| `TestSettingTheSameKeyIsNoEvent` | `EP` twice → 200, 200, one `project.key_set` | an event for a no-op |
| `TestAPatchCannotMoveState` | `PATCH` with `"state":"done"` → 400 `bad_request`; state unchanged | a state move that skips task 10's rule |
| `TestAPatchWritesNoRevision` | 50 priority patches → `headRevision` still 1, `issue_revisions` has one row | a revision per head write (an agent's context invalidated by triage) |
| `TestAbsentNullAndSetAreThreeThings` | `{}` keeps the assignee; `{"assigneeId":null}` clears it; `{"assigneeId":"u-…"}` sets it | `*string` read as absent = clear |
| `TestAnUnreadableIssueIs404` | Out Sider and Xavier `GET /v1/issues/{id}` → 404 `issue_not_found`, no project id in the body | a 403 or `project_not_found` that leaks the project |
| `TestNotTiedNeedsAReason` | create with `serves: []`, `notTiedReason: ""` → 422, `details.field = "notTiedReason"`; with `chore` → 201, `threadState` `{not_tied, chore}` | a silent open end (05 §2.3) |
| `TestThreadStateReadsTheBriefScope` | serves a project-scope brief → `project_level`; a branch-scope brief → `on_thread` | scope ignored |
| `TestTheListIsKeysetByNumber` | 120 issues, `limit=50` → 120..71, then 70..21, then 20..1 and `next: null`; `state=todo&assignee=none` filters | offset paging that skips or repeats under inserts |
| `TestLabelsWaitForTask11` | `labels: ["lb-…"]` on create → 400, `details.field = "labels"` | labels written with no team check |

Task 04's and 05's conformance rows that touch these routes run against the real Hub once task
16's client exists (task 16's acceptance, not this one's).

#### Steps

- [ ] **Step 1: Check the base.** On the sync base, read H14's `tx.go`, H17's `projects.go` and
  `authz.go`, and the chassis's recorder (H08). Note the recorder's name and `Querier`'s. Read
  task 03's `work.go` and task 04's additions, and drop any declaration above they already made.
- [ ] **Step 2: The tests first.** Write the seventeen tests. Run them: they fail to compile
  (no routes, no store). Add `"08"` to `workMatrixLive`; H16b's matrix and task 07's two
  meta-tests now fail too (rows live, routes absent).
- [ ] **Step 3: The store**, then **the routes**, then the one line in `server.go`. Run this task's
  tests, the matrix, and task 07's meta-tests until all pass.
- [ ] **Step 4: The whole Hub suite**, unit and integration, to see no Hub test changed colour.
- [ ] **Step 5: Commit by pathspec**, the six files, naming in the message any declaration dropped
  because task 03 or 04 made it.

#### Acceptance

```bash
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./control-plane/internal/hub/... -tags integration -race -count=1 -run 'WorkIssues|Matrix|WorkRows|LiveWorkRows'
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(TwentyConcurrentCreatesAreGapless|AFailedCreateBurnsNoNumber|ARetriedCreateIsOneIssue|AReplayHoldsNoText|OneEventPerWriteAndNoText|AnAssigneeIsAPersonInTheTeam|TheAliasFollowsTheKey|AKeyIsUniqueInTheTeam|SettingTheSameKeyIsNoEvent|APatchCannotMoveState|APatchWritesNoRevision|AbsentNullAndSetAreThreeThings|AnUnreadableIssueIs404|NotTiedNeedsAReason|ThreadStateReadsTheBriefScope|TheListIsKeysetByNumber|LabelsWaitForTask11)$' \
  2>&1 | grep -c '^--- PASS'                                                    # 17
go test ./control-plane/... -race -count=1                                     # unit: every package ok
```

Expected: the first ends `ok` for each package it matches (the scope's own acceptance line); the
second prints `17`; the third prints no `FAIL`. With the database variable unset, the integration
tests are `--- SKIP`, which is not a pass.

#### What this task does not claim

- **No later revision, restore or `/v1/refs`.** Only revision 1 is written, by the create; task 09
  writes the rest and makes `issue:` keys resolve.
- **No state move.** `state` is set at create among `triage`, `backlog`, `todo` and moves only
  through task 10's route; a `PATCH` naming it is refused.
- **Not that the criterion exists in the brief.** `serves` is checked at the key level only; the
  criterion-membership check needs a Hub reader of phase 4's brief schema (the gap above).
- **Not the one-record reading of §8.** A create is one event by this task's reading; the Hub
  owner's confirmation is **UNVERIFIED**.
- **No subscription, label or relation write.** Tasks 13 and 11 add them to these paths.
- **No desktop surface, and no measured latency.** The list's index (`issues_project_state`, task
  06) was chosen by reading the filter, not by `EXPLAIN` on data.

**Parity rows served:** **L-2** (the team gains Work settings, with task 11), **L-3** (the key and
the alias; closed with tasks 40 and 46), **L-13** and **L-38** (priority and the one human
assignee; closed with the issue view and the lens), **L-239** (every write's attributed event; the
Hub's audit route already exists).

#### Addendum (fix batch b02, S03-C1): the create route applies SW.3's check rule

Appended so no line above moves. After `s.checkCreate` returns `content` (`:340-343`), and before
`store.NextIssueNumber` so a refused create takes no number (W-01): `if err :=
hubapi.ValidateCheckChange(hubapi.IssueRevisionContent{}, content, true); err != nil { return
WriteResult{}, Refuse(422, hubapi.CodeBadRequest, err.Error(), fieldOf(err, "")) }`. `byPerson` is
`true` because every W1 principal is a person (task 07's addendum says who passes `false`). The
rule is task 07's; the function and its tests are task 02's. Size unchanged: one call.
