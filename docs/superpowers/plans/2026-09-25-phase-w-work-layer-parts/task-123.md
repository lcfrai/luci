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

**Files:**

- `control-plane/internal/hub/store/work_cursors.go`, `server/work_cursors.go` (new): three routes
- `control-plane/internal/hub/store/work_scope.go` (new): the subjects under a scope (decision 4)
- `control-plane/internal/hub/server/changes.go` ⚑ (Hub task 26's route; tasks 13 and 117 before
  this): the `scope` parameter, and nothing else
- `control-plane/internal/hub/server/work_cursors_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑: the `"123"` rows

**Interfaces:**

- Produces: `GET /v1/me/read-cursors?scope={kind}:{id}`, `PUT /v1/me/read-cursors`, `POST
  /v1/me/read-cursors/mark-seen` (T114's route table), and `scope` on `GET /v1/changes` (task 124's
  `ScopedChanges` row); `store.ScopeSubjects`, `store.AdvanceCursor`
- Consumes: T114's `ScopeRef`, `ReadCursor`, `MarkSeenRequest`; task 116's `work_read_cursors` and
  its trigger; task 13's feed filter (`SubjectOnlyKinds`, the readable-project join) and its kinds
  (SW.5); tasks 76, 12, 119 and 120 for the rows decision 4 joins; TM task 07's `RequireDesktop`;
  the Hub's `ChangesPage` (`hubapi/types.go:547-551`) and `PageRequest` (`:201-205`)
- Consumed by: task 124 (`GetReadCursor`, `AdvanceReadCursor`, `MarkSeen`, `ScopedChanges` over
  HTTP), task 133 (the four delta classes, drawn from the scoped read after the cursor), and TM
  task 54 through 133

**G1, task 114.** T114 carries `ScopeRef {kind, id}` but not its text form or the scoped page.
Task 124 needs both and names them `ReadScope`, `ParseReadScope` and `ScopedChangesPage`
(`task-124.md`, *Gaps* 2); this part keeps T114's `ScopeRef` name and writes the rest as 114 must
carry it. One name wins at sync; the wire is the same.

```go
// control-plane/internal/hub/hubapi/work_roadmap.go (task 114's file) — G1, additive

const (
	ScopeKindProject = "project"
	ScopeKindFeature = "feature"
	ScopeKindIssue   = "issue"
)

// String is the query form, "feature:ft-…".
func (s ScopeRef) String() string { return s.Kind + ":" + s.ID }

// ParseScopeRef reads "kind:id"; the id must carry the kind's prefix
// (pj-, ft-, is-; task 02's ValidID), so "feature:is-…" is refused.
func ParseScopeRef(v string) (ScopeRef, error) {
	kind, id, ok := strings.Cut(v, ":")
	prefix := map[string]string{ScopeKindProject: PrefixProject, ScopeKindFeature: PrefixFeature, ScopeKindIssue: PrefixIssue}[kind]
	if !ok || prefix == "" || !ValidID(prefix, id) {
		return ScopeRef{}, &FieldError{"scope", ErrScope}
	}
	return ScopeRef{Kind: kind, ID: id}, nil
}

var ErrScope = errors.New("scope must be project:pj-…, feature:ft-… or issue:is-…")

// ScopedChangesPage is ChangesPage plus Truncated: true only on a first look
// (no after) when older events under the scope were left out (decision 5).
type ScopedChangesPage struct {
	Events    []Event `json:"events"`
	Next      string  `json:"next"`
	More      bool    `json:"more"`
	Truncated bool    `json:"truncated"`
}

const FirstLookLimit = 200
```

`PrefixFeature` and `PrefixIssue` are task 02's names for `ft-` and `is-` (`hubapi/workids.go`), and
`PrefixProject` the Hub's (`envelope.go`'s `ValidID(PrefixProject, …)`, built). `ScopedChangesPage`
repeats `ChangesPage`'s three fields rather than embedding it, so its JSON is flat and a client
decoding either gets the same keys.

#### The store (signatures complete)

```go
package store

// store/work_scope.go

// ScopeSubjects is decision 4: the subject ids whose events lie under the
// scope, and the scope's project. found is false when the scope's object does
// not exist in the org; the handler then applies the object's read rule.
//   project: projectID only (the feed filters on projectId, not on subjects)
//   feature: ft- ∪ its bf- ∪ is- WHERE feature_id = ft ∪ ic- of those ∪ ml- of ft
//            ∪ nt- of work_update_meta WHERE subject = ft ∪ briefKey prefix for evidence
//   issue:   is- ∪ ic- WHERE issue_id = is
func ScopeSubjects(ctx context.Context, q Querier, s hubapi.ScopeRef) (projectID string, subjects []string, briefKeyPrefix string, found bool, err error)

// store/work_cursors.go

// Cursor reads the principal's cursor on a scope; 0 when never set.
func Cursor(ctx context.Context, q Querier, userID string, s hubapi.ScopeRef) (int64, error)

// AdvanceCursor is decision 2 in one statement:
//   INSERT INTO work_read_cursors (org_id, user_id, scope_kind, scope_id, seq, updated_at)
//   VALUES (…) ON CONFLICT (user_id, scope_kind, scope_id)   -- task 116's primary key
//   DO UPDATE SET seq = GREATEST(work_read_cursors.seq, EXCLUDED.seq),
//                 updated_at = CASE WHEN EXCLUDED.seq > work_read_cursors.seq
//                                   THEN EXCLUDED.updated_at ELSE work_read_cursors.updated_at END
//   RETURNING seq
// GREATEST keeps task 116's "seq only increases" trigger from ever firing on
// this path; the trigger is the guard for any other writer.
func AdvanceCursor(ctx context.Context, tx pgx.Tx, org, userID string, s hubapi.ScopeRef, seq int64, at time.Time) (int64, error)

// HeadSeq is the org's current audit head, for the 422 of decision 2.
func HeadSeq(ctx context.Context, q Querier) (int64, error)
```

The column names are task 116's scope's (`work_read_cursors`, *one row per person and scope*) and
are renamed at build time if its part chose others. A feature with hundreds of issues gives
`ScopeSubjects` a long list; the build may turn it into a subquery inside the feed read instead of
an array parameter. Either is the same rule; which is faster is **unmeasured**.

#### The routes

```go
// server/work_cursors.go
mux.HandleFunc("GET /v1/me/read-cursors", s.getReadCursor)
mux.HandleFunc("PUT /v1/me/read-cursors", s.advanceReadCursor)
mux.HandleFunc("POST /v1/me/read-cursors/mark-seen", s.markSeen)
// server/changes.go ⚑: no new pattern; Hub task 26's GET /v1/changes gains `scope`.
```

**The read rule, shared.** `readScope(tx, principal, ScopeRef)`: `ParseScopeRef` (400 `field:
"scope"`); `ScopeSubjects`; not found, or the object not readable by the principal under task 13's
readable-project rule → 404 with the object's own code: `project_not_found`, `feature_not_found`
or `issue_not_found`, exactly what `GET` of that object answers, so a cursor never tells more than
the object's own route (`hub-contract:242`, `:273`). A task 117 grant makes a granted project's
scopes readable, as it makes the objects readable.

- **`getReadCursor`**, in `s.Read`: `RequireDesktop` (403 `required: desktop`); `readScope`;
  `Cursor`; 200 `ReadCursor`. Any query parameter but `scope` is ignored (decision 6).
- **`advanceReadCursor`**, in a write transaction **that appends no event** (decision 3):
  `RequireDesktop`; `Decode[hubapi.ReadCursor]`; `readScope`; `seq` from 0 to `HeadSeq` (else 422
  `field: "seq"`); `AdvanceCursor`; 200 `ReadCursor` with the stored `seq`, lower or not.
- **`markSeen`**, the same in **one** transaction over `MarkSeenRequest.Scopes` (1-200, T114): any
  scope failing `readScope` fails the whole request with that scope's 404 and `details.index`, and
  none moves; 200 `[]ReadCursor` in request order.
- **`GET /v1/changes?scope=…`** (⚑ Hub task 26's handler): when `scope` is absent, the handler is
  byte-for-byte what it was. When present: `readScope`; then the Hub's own feed query **with its
  caller filter first** (task 13's readable-project join and `SubjectOnlyKinds`), narrowed by
  `projectId = <scope's project>` and, for a feature or an issue, `subject = ANY(<subjects>) OR
  data->>'briefKey' LIKE <prefix>`; `after` and `limit` as the Hub's (limit 1-500, default 200);
  no `after` → the newest `FirstLookLimit` events in seq order, `truncated` when a 201st exists;
  `after=latest` → no events and the head, as the Hub's rule (`hub-contract:1101-1102`). The answer
  is `ScopedChangesPage`.

How Hub task 26 builds its feed query (one SQL string or composed clauses) is **UNVERIFIED** until
its code lands; this task adds its narrowing **inside** that query, after the caller filter, never
as a post-filter over a page, so a page is never short and `more` stays true to the rows.

#### Tests to write first — `server/work_cursors_integration_test.go` (`//go:build integration`)

The cast is task 11's (Grace, a Portal member; Linus, Billing only; Xavier, another org), Grace's
desktop and her Orchestrator's Fleet principal. Portal has features FT-1 and FT-2; EP-142 is under
FT-1 and EP-143 under FT-2.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestACursorOnlyAdvances` | PUT `seq` 7 → 7; PUT 5 → 200 and 7; GET → 7; PUT above the head → 422 `field: seq` | cursor goes back; the future marked seen |
| `TestACursorIsTheCallersOwn` | Grace sets 9 on `project:<Portal>`; Linus, granted nothing, gets 404 on the same scope; `GET …?scope=…&userId=<Grace>` as Ada answers Ada's own `0` | a cursor readable by others |
| `TestOnlyADesktopMovesACursor` | the Fleet principal under Grace's delegation → 403 `required: desktop` on all three routes | an Orchestrator marking a person's work seen |
| `TestAnUnreadableScopeIs404` | Linus on `project:<Portal>` → 404 `project_not_found`; on `feature:<FT-1>` → 404 `feature_not_found`; on an id that exists nowhere → the same answers | existence leak (`hub-contract:273`) |
| `TestTheScopeFilterFollowsTheTree` | a comment on EP-142 and a milestone on FT-1 appear under `feature:<FT-1>` and `project:<Portal>`, never under `feature:<FT-2>`; EP-143's state move only under FT-2 and Portal | scope ignored |
| `TestTheScopeFilterNeverWidens` | a `subject` Grace cannot read (a Billing issue id placed in a Portal feature's list by a fixture) never appears; `SubjectOnly` kinds for Ada never reach Grace | a filter applied before the caller filter |
| `TestAFirstLookIsBounded` | 250 events under FT-1, no `after` → 200 events, the newest, in seq order, `truncated: true`; with 150 events → 150 and `truncated: false` | an unbounded first read |
| `TestNoScopeIsTheHubsFeed` | `GET /v1/changes` without `scope` → the same bytes as Hub task 26's own test expects | a changed Hub route |
| `TestMarkSeenIsAllOrNothing` | three scopes → three cursors in one transaction; one unreadable scope among them → 404 `details.index: 2`, and none of the three moved | a partial mark |
| `TestACursorMoveAppendsNoEvent` | the org's head `seq` is the same before and after twenty advances and a mark-seen | cursor moves flood the feed |

#### The matrix rows (`workMatrixRows["123"]`, appended)

Columns V M A O N X (`hub-contract:341`), each principal on a scope in Portal. The routes are
*self*: every role that can read the scope moves its **own** cursor.

| Route | V | M | A | O | N | X | Granted member |
|---|---|---|---|---|---|---|---|
| `GET /v1/me/read-cursors?scope=project:<Portal>` | 200 | 200 | 200 | 404 | 404 | 404 | 200 |
| `PUT /v1/me/read-cursors` | 200 | 200 | 200 | 404 | 404 | 404 | 200 |
| `POST /v1/me/read-cursors/mark-seen` | 200 | 200 | 200 | 404 | 404 | 404 | 200 |
| `GET /v1/changes?scope=project:<Portal>` | 200 | 200 | 200 | 404 | 404 | 404 | 200 |

A viewer moves a cursor: reading is what a viewer does, and a cursor records only that. The Hub's
own `GET /v1/changes` row (`hub-contract:357`) is unchanged; these are additional rows for the
`scope` form.

#### Steps

- [ ] **Step 1:** Confirm the sync base (rule H1): Hub task 26's feed, tasks 13, 114, 116, 117,
  119 and 120 committed; TM tasks 52 and 54 on the sync base or waited for (rule H2). Re-read
  `hub-contract:1088-1110` and task 13's feed filter at the base.
- [ ] **Step 2:** Check T114 for G1; if missing, stop and raise it against 114 (and name 124's
  `ReadScope` spelling in the same note).
- [ ] **Step 3:** Write the integration test and the matrix rows; every row fails.
- [ ] **Step 4:** `store/work_scope.go` and `store/work_cursors.go`; then the three cursor routes;
  their rows go green.
- [ ] **Step 5:** The `scope` branch of Hub task 26's handler; `TestNoScopeIsTheHubsFeed` and Hub
  task 26's own feed tests stay green; then the scope rows.
- [ ] **Step 6:** Rerun task 13's feed tests and task 115's cursor conformance rows against the
  fake; they agree on 7 then 5 reading 7.
- [ ] **Step 7:** `git diff -- go.work control-plane/go.mod` shows nothing; commit by pathspec.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(ACursorOnlyAdvances|ACursorIsTheCallersOwn|OnlyADesktopMovesACursor|AnUnreadableScopeIs404|TheScopeFilterFollowsTheTree|TheScopeFilterNeverWidens|AFirstLookIsBounded|NoScopeIsTheHubsFeed|MarkSeenIsAllOrNothing|ACursorMoveAppendsNoEvent)$' \
  2>&1 | grep -c "^--- PASS"                                                                    # 10
go test -tags integration ./control-plane/internal/hub/... -race -count=1                      # ok
go test ./control-plane/... -race -count=1                                                      # ok
```

#### What this task does not claim

- Session or branch cursors (AX §6.1 names both; they are the cockpit's, not the Work layer's).
- The four delta classes, or any surface: task 133 derives and draws them from this read.
- That Phase T's `teamSeenAt` and this cursor are one record: they are two until the sync with
  Phase T decides (the proposal above), and task 62's control moves both.
- That a feature's scope holds events of issues that **left** it: membership is read now
  (decision 4).
- Any read speed for a feature with many issues (unmeasured).

**Parity rows served:** none alone. It carries **L-118** and **L-119** for task 133, which closes
them.

**Gaps for the skeleton and siblings:** G1 (`ScopeRef.String`, `ParseScopeRef`,
`ScopedChangesPage`, `FirstLookLimit`) belongs to task 114, and 124 spells them `ReadScope`; the
global constraint *"every Hub write appends one event"* needs decision 3's named exception for
read cursors; Phase T's `teamSeenAt` (TM tasks 52 and 54) is a second cursor for the sync to fold
into this one.
