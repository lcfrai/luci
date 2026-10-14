### Task 76: features `ft-`: lead, dates, status, branch reservation, *promote to feature*

**Slice:** W2. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 74 (the `features` table and `issue_heads.feature_id`), and P4-18 (phase 4's task
18, where a brief's text is first drafted), after a sync-merge of `feat/hub` (rule H1).
**Seams:** consumes **SW.18** (task 72's `Feature`, `FeatureState`, `FeatureBranchRef`,
`PromoteResult`, `CodeFeatureNotFound`, `CodeSlugInvalid`) and the Hub's briefs (contract §7,
`2026-09-23-hub-contract.md:755-761`); adds four event kinds to **SW.5**.

Linear calls a planned body of work a *project*. We already have projects (the repository), so
ours is a **feature**: a name, a lead, a target date, a status a person sets, and **its own
branch** (04 §2.5). The branch is what ties a feature to the provenance thread: a feature owns a
branch brief, the brief's criteria are what its issues serve, and landing on that branch is what
turns them `landed`. So creating a feature **reserves the branch** by creating its branch brief in
the same transaction, and a second feature or brief on that ref is refused. *Promote to feature*
turns a large issue into a feature with the issue as its first child, without copying anything.

**Anchors, re-read on 2026-09-26.** In the Hub worktree at `6463e568` (read, not merged here):
brief creation and its `409 brief_exists` (`hub-contract.md:755-761`, `:253`); the ref rules
(`:131-137`); `BriefHead` has no title and a new brief has `headRevision: 0` (`:641-644`); the
`brief.created` event (`:1148`); the rebind route `PATCH /v1/briefs/{briefId}` (Hub plan
`task-18.md:26`); roles (`:324-333`). In this branch's parts (not yet code, **UNVERIFIED until
merge**): task 72's `Feature` (`task-72.md:505-522`), `FeatureState` (`:110-122`),
`FeatureBranchRef` and its slug rule (`:250-251`, `:303-309`), `PromoteResult` (`:659-664`), codes
(`:352`, `:370`); task 24's Start order (`task-24.md:253-270`) and its AU fail-safe
(`:22-27`, `:396-402`); task 11's labels (`task-11.md:280-284`). **Task 74's part is not written
at this writing**; its `features` columns are the scope's (`tasks-61-80.md:475`), **UNVERIFIED**.

#### Decisions the scope left open, made here

**(a) The brief is created exactly as `POST /v1/projects/{projectId}/briefs` creates it.** The
Hub's brief record has no title (`hub-contract.md:641-644`) and a new one has no revision. So the
feature route calls the Hub's own brief insert (Hub task 18's store function, in the same
transaction), writes the same `brief.created` event, and writes **no brief content**. The scope's
"the brief record with the issue's title" cannot be stored on a `BriefHead`; the Hub never writes
brief text on a person's behalf, because every brief answer is authored (`text·H` or `text·A`).
The title reaches the brief when the desktop's promote flow (task 86) opens phase 4's Discover
lens on the new brief with the issue's title as its first question's context (P4-18's path).

**(b) The pointer to the issue is a key, in the event.** `issue.promoted` carries `issueKey`
(`issue:is-…@rev`, 05 §3.6), the revision the feature came from. With the issue's `featureId`
set, that is the whole link: nothing is copied (L-23). The issue's `serves` stays on its own
revision, unchanged.

**(c) A feature's `branchRef` is its brief's, read by a join.** A brief can be rebound (Hub
`task-18.md:26`). If the feature kept its own copy, a rebind would leave the two disagreeing. So
`Feature.branchRef` is answered from `briefs.branch_ref` through `features.brief_id`, and the
reservation's uniqueness is the brief's own index (one live branch brief per `(project,
branchRef)`, `hub-contract.md:760`). Task 74's `features` unique `(project_id, branch_ref)` is
then a second copy this task never reads; see *Corrections to the scope*.

**(d) The slug rule on an AU-only project.** Q8 (`05-design.md:1374`, from 04 §5.2
`04-internal-fit.md:432-433`) keeps title words out of an AU-only project's branch names, because
an erase cannot reach a branch name on a customer's remote. A **word** is a run of 3 or more `[a-z0-9]` in the
lower-cased feature name (and, for *promote*, the issue's title). On an AU-only project a slug any
of whose `-`-separated parts equals a word is 422 `slug_invalid`, `details.slug`, message *"AU
projects keep names out of branches. Try feature-12."* The project's class is read through
`store.ProjectAUOnly`; **a class the Hub cannot read is AU** (task 24's fail-safe,
`task-24.md:22-27`), so until the class has a home (Phase T task 43, **UNVERIFIED**) every
project takes the AU rule. That is visible and safe, and ends when the class lands.

**(e) The lead is a member or admin of the project's team.** A viewer cannot lead work they cannot
write. Anyone else is 422 `bad_request`, `details.field: "leadId"`. The lead defaults to the
creator.

**(f) Status moves freely among the five, by a person only.** `planned`, `active`, `paused`,
`done`, `cancelled` are a person's words about intent; nothing is derived from them and nothing
derives them. What the lanes did (live, landed, released where) is **derived at read time** by the
desktop from the feature's child issues (task 85), and no such field exists on the wire (task 72's
`Feature` has none), so none can be stored.

**Files:**

| Path | Change |
|---|---|
| `control-plane/internal/hub/store/work_features.go` | new: insert, read, list, patch, the child link, the class read |
| `control-plane/internal/hub/server/work_features.go` | new: the six routes, the reservation and *promote* |
| `control-plane/internal/hub/server/work_features_integration_test.go` | new: the tests below |
| `control-plane/internal/hub/hubapi/work_events.go` ⚑ (task 13's) | four kinds and their data structs |
| `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ (task 07's) | `workMatrixRows["76"]`; `"76"` in `workMatrixLive` |
| `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ (H16b's) | the fixture gains `FeatureID`, one feature made in the fixture build |
| `control-plane/internal/hub/server/server.go` ⚑ | one line: `s.registerWorkFeatures(mux)` |
| `control-plane/internal/workstart/start.go` ⚑ (task 24's) | the one-line delta: a lane name equal to a reserved feature slug is refused |

#### The wire (routes and bodies; the records are task 72's)

```text
POST  /v1/projects/{projectId}/features   member  201 Feature           CreateFeatureRequest
GET   /v1/projects/{projectId}/features   viewer  200 Page[Feature]     ?state=&includeCancelled=false&cursor=
GET   /v1/features/{featureId}            viewer  200 Feature
PATCH /v1/features/{featureId}            member  200 Feature           PatchFeatureRequest
PUT   /v1/issues/{issueId}/feature        member  200 IssueHead         SetFeatureRequest
POST  /v1/issues/{issueId}/promote        member  201 PromoteResult     PromoteToFeatureRequest
```

```go
// Task 72's block 6 declares these ("one create/patch body per record", its scope). They are
// shown so this part stands alone; where task 72 spells a field differently, task 72 wins.
type CreateFeatureRequest struct {
	Name       string   `json:"name"`                 // 1-80 characters, no control characters
	Slug       string   `json:"slug"`                 // FeatureBranchRef(slug) must be ok
	LeadID     string   `json:"leadId,omitempty"`     // u-…; "" is the creator
	TargetDate *string  `json:"targetDate,omitempty"` // ValidDate
	Priority   Priority `json:"priority,omitempty"`   // "" is none
	Labels     []string `json:"labels,omitempty"`     // live labels of the team
}

// PatchFeatureRequest: tri-state fields (task 72 decision (d)). Absent keeps; null clears
// where a field may be empty (targetDate only); a value sets. Name, lead and state cannot clear.
type PatchFeatureRequest struct {
	Name       json.RawMessage `json:"name,omitempty"`
	LeadID     json.RawMessage `json:"leadId,omitempty"`
	TargetDate json.RawMessage `json:"targetDate,omitempty"`
	State      json.RawMessage `json:"state,omitempty"`
	Priority   json.RawMessage `json:"priority,omitempty"`
	Labels     json.RawMessage `json:"labels,omitempty"` // the whole list
}

type SetFeatureRequest struct {
	FeatureID *string `json:"featureId"` // ft-… of the issue's project; null clears
}

type PromoteToFeatureRequest struct {
	Name string `json:"name"` // "" takes the issue's title, cut to 80 characters
	Slug string `json:"slug"`
}
```

A worked example, EP-142 promoted in a Global project:

```json
{"name": "Resumable uploads", "slug": "resumable-uploads"}
```

answers 201 with `feature.branchRef: "refs/heads/resumable-uploads"`, `feature.briefId` a new
`bf-…` at `headRevision: 0`, `feature.lead` the promoter, `feature.state: "planned"`, and
`issue.featureId` the new `ft-…`, with `issue.revision` unchanged.

Refusals, in the order checked:

| Condition | Answer |
|---|---|
| a slug `FeatureBranchRef` refuses (upper case, a `/`, over 63 characters) | 422 `slug_invalid`, `details.slug` |
| an AU-only project (or a class unreadable) and a slug sharing a word with the name or title | 422 `slug_invalid`, `details.slug` (decision (d)) |
| a live brief already on `refs/heads/<slug>` | 409 `brief_exists`, `details.briefId` the live one (the Hub's code) |
| a lead not a member or admin of the team | 422 `bad_request`, `details.field: "leadId"` |
| a `targetDate` with a time, or not a day | 422 `date_invalid`, `details.field: "targetDate"` |
| a label not live in the team | 422 `label_unavailable`, `details.field: "labels[n]"` (task 11's code) |
| a `PATCH` naming `lanesLive`, `landed`, `released` or any other key | 400 `bad_request`, `details.field` that key (strict decode) |
| `SetFeatureRequest` naming another project's feature, or one `cancelled` | 404 `feature_not_found` / 422 `bad_request` `field: "featureId"` |
| *promote* on an issue that already has a `featureId` | 422 `bad_request`, `details.field: "featureId"`, *"EP-142 is already in Resumable uploads."* |
| *promote* on an issue that is `done`, `cancelled` or `duplicate` | 422 `bad_request`, `details.field: "state"` (a closed issue starts nothing) |

The two *promote* refusals are decisions: a second promote would make an issue the first child of
two features, and a closed issue has no work left to plan. Neither has its own code in task 72,
so both use `bad_request` with the field, the Hub's idiom for a body a person can correct.

#### The four event kinds (complete, appended to task 13's registry)

A branch ref is already event data in the Hub (`brief.created {"scope","branchRef"}`,
`hub-contract.md:1148`), so `feature.created` may carry it. A feature's **name** is not carried:
it is text a person typed, and the reader can fetch it.

```go
const (
	EventFeatureCreated  = "feature.created"
	EventFeatureUpdated  = "feature.updated"
	EventIssueFeatureSet = "issue.feature_set"
	EventIssuePromoted   = "issue.promoted"
)

type FeatureCreatedData struct {
	FeatureID string `json:"featureId"`
	BriefID   string `json:"briefId"`
	BranchRef string `json:"branchRef"`
}

// FeatureUpdatedData names the changed fields, comma-joined (no arrays in data).
type FeatureUpdatedData struct {
	Fields string `json:"fields"` // e.g. "state,targetDate"
	State  string `json:"state"`  // the state after the write, so a feed reader needs no fetch
}

type IssueFeatureSetData struct {
	From string `json:"from"` // ft-… or ""
	To   string `json:"to"`
}

// IssuePromotedData: the issue's own key at the revision it was promoted from (decision (b)).
type IssuePromotedData struct {
	FeatureID string `json:"featureId"`
	BriefID   string `json:"briefId"`
	IssueKey  string `json:"issueKey"` // issue:is-…@rev
}
```

```go
{EventFeatureCreated, FeatureCreatedData{}, ScopeProject, VisibleToReaders},
{EventFeatureUpdated, FeatureUpdatedData{}, ScopeProject, VisibleToReaders},
{EventIssueFeatureSet, IssueFeatureSetData{}, ScopeProject, VisibleToReaders},
{EventIssuePromoted, IssuePromotedData{}, ScopeProject, VisibleToReaders},
```

**Events per write.** The scope's "every write has one event" is read as **one event per record
written**, because the Hub's brief is its own record and every brief creation writes
`brief.created` (`hub-contract.md:761`), which replicas and Orchestrators follow. So:

| Write | Events, in order |
|---|---|
| create | `brief.created`, `feature.created` |
| patch (something changed) | `feature.updated` |
| set feature (changed) | `issue.feature_set` |
| promote | `brief.created`, `feature.created`, `issue.promoted` |

A patch or set that changes nothing writes nothing and no event.

#### `store/work_features.go` (signatures complete)

```go
package store

// Inside the caller's transaction, after hub.org_id and the org's audit head lock (task 11).

// FeatureRow is the stored row; BranchRef is read from the brief by join (decision (c)).
type FeatureRow struct {
	ID, ProjectID, Name, BriefID, BranchRef, LeadID, CreatedBy string
	TargetDate                                                 *string
	State                                                      hubapi.FeatureState
	Priority                                                   hubapi.Priority
	Labels                                                     []string
	CreatedAt, UpdatedAt                                       time.Time
}

// InsertFeature writes the feature after the caller has inserted its brief in the same tx.
func InsertFeature(ctx context.Context, tx pgx.Tx, org string, f FeatureRow) error

// FeatureByID reads one feature under row-level security; ok is false for none or another
// org's. The caller checks the caller's team role on f.ProjectID.
func FeatureByID(ctx context.Context, q Querier, id string) (f FeatureRow, ok bool, err error)

// ListFeatures: the project's features by created_at, then id; cancelled ones only when asked.
func ListFeatures(ctx context.Context, q Querier, projectID string, state *hubapi.FeatureState, includeCancelled bool, pr hubapi.PageRequest) (hubapi.Page[FeatureRow], error)

// PatchFeature writes the named columns and updated_at; changed lists the JSON names that
// differ from the stored row, sorted (none: no write).
func PatchFeature(ctx context.Context, tx pgx.Tx, id string, next FeatureRow, at time.Time) (changed []string, err error)

// SetIssueFeature: UPDATE issue_heads SET feature_id = $2, updated_at = $3 WHERE issue_id = $1.
// "" writes NULL. Task 74's foreign key refuses an unknown ft-.
func SetIssueFeature(ctx context.Context, tx pgx.Tx, issueID, featureID string, at time.Time) error

// TeamRoleOf answers the user's role in the team, "" when not a member (the Hub's team_members).
func TeamRoleOf(ctx context.Context, q Querier, teamID, userID string) (hubapi.Role, error)

// ProjectAUOnly reads the project's data class. known is false when the project record has no
// class yet (its home is UNVERIFIED until Phase T's task lands); the caller treats that as AU.
func ProjectAUOnly(ctx context.Context, q Querier, projectID string) (auOnly, known bool, err error)
```

The brief insert is the **Hub's own** store function for `POST /v1/projects/{projectId}/briefs`
(Hub task 18, `store/briefs.go`; its name **UNVERIFIED** until merge), called in this task's
transaction, and its unique-violation mapping to `brief_exists` is reused unchanged. If Hub task 18
has no store function callable with a caller's transaction (it could open its own), this task adds
a `tx`-taking variant beside it and the old one calls it; that is a Hub file edit, so it is flagged
in *Corrections*.

#### The routes (`server/work_features.go`), in order

```go
func (s *Server) registerWorkFeatures(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/projects/{projectId}/features", s.postFeature)
	mux.HandleFunc("GET /v1/projects/{projectId}/features", s.listFeatures)
	mux.HandleFunc("GET /v1/features/{featureId}", s.getFeature)
	mux.HandleFunc("PATCH /v1/features/{featureId}", s.patchFeature)
	mux.HandleFunc("PUT /v1/issues/{issueId}/feature", s.putIssueFeature)
	mux.HandleFunc("POST /v1/issues/{issueId}/promote", s.promoteIssue)
}
```

The two `POST`s answer records with no issue text except `PromoteResult.issue`, which is an
`IssueHead` (its `external.url` is an imported issue's link). So `postFeature` runs under
`server.Write` and `promoteIssue` under `server.WriteReplayable` with task 08's
`rebuildIssueHead` for its `issue` half (`task-11.md:334-339`, the same rule). Each refusal rolls
the transaction back and appends nothing.

**`postFeature`** (member of the project's team):

1. `Decode[hubapi.CreateFeatureRequest]`; the project outside the caller's readable set → 404
   `project_not_found`; role below member → `server.Forbidden("member")`.
2. `name` trimmed, 1-80 characters, no control character → else 400 `field: "name"`.
3. `ref, ok := hubapi.FeatureBranchRef(slug)`; `!ok` → 422 `slug_invalid`.
4. `ProjectAUOnly`; `!known || auOnly` → the word check of decision (d) against `name`.
5. `leadId` `""` → the caller; else `TeamRoleOf(team, leadId)` must be `member` or `admin` → else
   422 `bad_request`, `field: "leadId"`.
6. `targetDate` → `hubapi.ValidDate` → else 422 `date_invalid`, `field: "targetDate"`. `priority`
   `""` → `none`, else task 03's `ValidPriority`. Each label → task 11's `LabelByID`, live, of this
   team (or an org label, task 78) → else 422 `label_unavailable`, `field: "labels[n]"`.
7. The Hub's brief insert with `scope: branch`, `branchRef: ref`; its unique violation → 409
   `brief_exists`, `details.briefId` the live brief (the Hub's existing mapping). Then
   `InsertFeature` with a new `ft-` (`ids.Wide(hubapi.PrefixFeature)`, `task-02.md:149-150`),
   state `planned`.
8. Events `brief.created {scope: "branch", branchRef}` then `feature.created`. 201 `Feature`.

**`listFeatures`** and **`getFeature`** (viewer): `getFeature` reads by id and answers 404
`feature_not_found` when the feature is absent **or** its project is outside the caller's
readable set (no existence leak, the Hub's rule for every `*_not_found`). `Lead` and `CreatedBy`
are rendered as `User` by the Hub's user lookup. `InitiativeID` is always `null` in W2.

**`patchFeature`** (member): decode strictly (unknown keys 400, so no derived fact can be sent);
read each tri-state field through `hubapi.ChangeOf`; `name`, `leadId`, `state` and `priority`
refuse `null` (400 naming it); `state` must satisfy `ValidFeatureState`; `leadId`, `targetDate`
and `labels` take the create checks. `PatchFeature`; nothing changed → 200, no event; else
`feature.updated {fields, state}`. A `done` or `cancelled` feature may be patched (a person
reopening it sets `state` back).

**`putIssueFeature`** (member of the issue's team): `readableIssue(…, RoleMember, lock=true)`;
`featureId` `null` clears; else `FeatureByID` in the **issue's project** → else 404
`feature_not_found`; a `cancelled` feature → 422 `bad_request`, `field: "featureId"`. Equal to the
head's → 200, no event. `SetIssueFeature`; `issue.feature_set {from, to}`. 200 `IssueHead`.

**`promoteIssue`** (member):

1. `Decode[hubapi.PromoteToFeatureRequest]`; `readableIssue(…, RoleMember, lock=true)`; the
   head's `featureId` set, or its state closed → 422 (the refusals table).
2. `name` `""` → the head revision's title text, cut to 80 characters at a rune boundary. The
   title is `text·H` of its author; the feature's name is then the **promoter's** field, and the
   feature records `CreatedBy` the promoter (a name is a label, not an authored answer).
3. Steps 3-7 of `postFeature`, with the AU word check run against the name **and** the head
   revision's title (decision (d)); the lead is the promoter.
4. `SetIssueFeature(issue, ft)`. **No issue revision is written**, and `issue_serves` is not
   touched (L-23): the issue keeps serving what it served, on its own revision.
5. Events `brief.created`, `feature.created`, `issue.promoted {featureId, briefId, issueKey}`,
   where `issueKey` is `hubapi.IssueKey{IssueID, head.Revision}.String()` (task 02). 201
   `PromoteResult`.

#### The delta to task 24's Start (`workstart/start.go` ⚑)

The Hub cannot see git (`hub-contract.md:131-137` validates a ref's format only), so the control
plane keeps a reserved name from being cut for anything else. One line in Start's step 4, before
`Lanes.Create` (`task-24.md:264`):

```go
//  4a. reserved, err = Features.Reserved(ctx, ProjectID, name); reserved && issue's featureId is not
//      that feature → ErrNameReserved (409 brief_exists, the Hub's code, details.branchRef).
```

`Features` is a one-method interface beside task 24's `DataClass` (`task-24.md:161-165`),
satisfied by task 82's HTTP client through `ListFeatures` (and by task 73's fake):

```go
// Features answers whether a branch name is a reserved feature slug of the project.
type Features interface {
	Reserved(ctx context.Context, projectID, branchName string) (bool, error)
}
```

An error reading it refuses the Start (a reservation the control plane cannot check is treated as
taken, the same fail-safe as the data class). Cutting the feature's branch on the first Start of a
child issue is task 61's choice of `Branch` (the feature's `branchRef` as the lane's base); whether
P3-04's lane creation cuts a missing base branch is **UNVERIFIED** and reported.

#### The matrix rows (`server/work_matrix_rows_integration_test.go` ⚑), complete

H16b's fixture gains `FeatureID` (built once per cell: a feature `resumable-uploads` in P, made by
Grace through the route, so the rows read and patch a real record). The two creating rows use
fixed slugs, which is safe because H16b builds a fresh world per cell (`task-16b.md:38-41`).

```go
	"76": {
		wwrite("POST /v1/projects/{projectId}/features", "POST", "/v1/projects/{ProjectID}/features", 201, "project_not_found",
			func(*fixture) any { return map[string]any{"name": "Evidence export", "slug": "feature-2"} }),
		wread("GET /v1/projects/{projectId}/features", "/v1/projects/{ProjectID}/features", "project_not_found"),
		wread("GET /v1/features/{featureId}", "/v1/features/{FeatureID}", "feature_not_found"),
		wwrite("PATCH /v1/features/{featureId}", "PATCH", "/v1/features/{FeatureID}", 200, "feature_not_found",
			func(*fixture) any { return map[string]any{"state": "active"} }),
		wwrite("PUT /v1/issues/{issueId}/feature", "PUT", "/v1/issues/{IssueID}/feature", 200, "issue_not_found",
			func(f *fixture) any { return map[string]any{"featureId": f.FeatureID} }),
		wwrite("POST /v1/issues/{issueId}/promote", "POST", "/v1/issues/{IssueID}/promote", 201, "issue_not_found",
			func(*fixture) any { return map[string]any{"name": "Resume uploads", "slug": "feature-3"} }),
	},
```

The slugs `feature-2` and `feature-3` hold no word of their names, so the rows pass under the AU
fail-safe (decision (d)), which is every project until the class has a home. **The sync
principal's** column is task 150's, as in task 75; its database half is task 74's guard on
`issue_heads.feature_id` (`TestTheSyncActorPromotesNothing` below).

#### Tests to write first, each with the mutant it kills

`server/work_features_integration_test.go` (`//go:build integration`). Team T with Grace (admin),
Linus (member), Vera (viewer); project P; EP-142 serving two criteria of P's project brief. Each
is seen failing before the code it covers.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestASlugIsReservedOnce` | a feature on `feature-9` → 201; a second feature on `feature-9` → 409 `brief_exists` naming the first brief, and **no** second `features` row, **no** second brief, **no** event; a plain `POST /v1/projects/{p}/briefs` on `refs/heads/feature-9` → 409 too | the reservation outside the transaction (a feature row left behind the refused brief) |
| `TestADerivedFactCannotBeSent` | `PATCH {"released": true}` → 400 naming `released`; `{"lanesLive": 2}` → the same; the row unchanged | stored facts |
| `TestPromoteLinksAndCopiesNothing` | promote EP-142 → one `ft-`, one brief at `headRevision: 0`, `issue.featureId` set; EP-142's `revision`, digest and `serves` byte-equal before and after; `issue.promoted.issueKey` is `issue:<id>@<rev>` | a new issue revision; `serves` moved to the brief; a copied title |
| `TestAUProjectsKeepTitlesOutOfSlugs` | with the class unreadable (W2's state): promote *Resume uploads* with slug `resume-uploads` → 422 `slug_invalid`; slug `feature-4` → 201; with a store fake answering `known, !auOnly`: `resume-uploads` → 201 | a title slug on an AU branch; the fail-safe inverted (unknown treated as Global) |
| `TestTheLeadIsATeamMember` | `leadId` Vera (viewer) → 422 `field: leadId`; Out Sider → the same; Linus → 201 with `lead` Linus; absent → the creator | any user as lead |
| `TestAFeatureIsItsProjectsOnly` | `PUT /v1/issues/{EP-142}/feature` naming another project's feature → 404 `feature_not_found`; a cancelled feature → 422 | cross-project children |
| `TestTheRebindMovesTheFeatureRef` | rebind the feature's brief to `refs/heads/feature-10` (Hub route) → `GET` the feature answers the new `branchRef` | a stored copy of the ref drifting from the brief |
| `TestEventsPerRecordWritten` | create → exactly `brief.created`, `feature.created`; promote → exactly those two and `issue.promoted`; a no-op patch or set → none; `data` holds only the declared keys and never the name | a missing `brief.created`; two feature events; a silent write; the name in `data` |
| `TestTheSyncActorPromotesNothing` | as `hub_app` with `hub.work_actor_kind = 'integration'`, `UPDATE issue_heads SET feature_id` → `42501` | the database half of the sync rule |

`workstart/start_test.go` (task 24's file, unit, on fakes):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestStartRefusesAReservedName` | `Features.Reserved` true for `ep-142` and the issue not in that feature → `ErrNameReserved`, no lane made, no session; the issue **in** that feature → Start proceeds; `Reserved` errors → refused | a reserved branch cut for another lane; the check failing open |

#### Steps

- [ ] **Step 1: Check the base.** Task 74's `features` table and `issue_heads.feature_id` as
  merged; Hub task 18's brief insert and whether it takes the caller's transaction (see the store
  section); task 72's request bodies as merged. Where the project's data class lives (if Phase T
  has landed it, `ProjectAUOnly` reads it; if not, it answers `known: false`).
- [ ] **Step 2: The tests.** The nine integration tests, the `"76"` rows with the fixture's
  `FeatureID`, and task 24's `TestStartRefusesAReservedName`. Run them: they fail.
- [ ] **Step 3: The store, then the routes** in the order above, then the four event kinds, then
  the one-line Start delta and its `Features` interface (task 73's fake and task 82's client
  satisfy it; until task 82 lands, `cmd/api` wires the fake's method or leaves `Features` nil,
  which **refuses** every Start of a project with features, the fail-safe).
- [ ] **Step 4: Green.** This task's suites, the Hub's brief tests (Hub `TestOneLiveBriefPerScope`
  must still pass: the insert is shared), task 24's suite and the matrix.
- [ ] **Step 5: Commit by pathspec**, the eight files of *Files*.

#### Acceptance

```bash
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(ASlugIsReservedOnce|ADerivedFactCannotBeSent|PromoteLinksAndCopiesNothing|AUProjectsKeepTitlesOutOfSlugs|TheLeadIsATeamMember|AFeatureIsItsProjectsOnly|TheRebindMovesTheFeatureRef|EventsPerRecordWritten|TheSyncActorPromotesNothing)$' \
  2>&1 | grep -c '^--- PASS'                                                    # 9
go test ./control-plane/internal/workstart/ -race -count=1 -run '^TestStartRefusesAReservedName$' -v 2>&1 | grep -c '^--- PASS'   # 1
go test ./control-plane/internal/hub/... -tags integration -race -count=1       # every package ok
grep -rn "branch_ref" control-plane/internal/hub/store/work_features.go | grep -c "briefs"   # >= 1: the ref is read from the brief
```

Expected: `9`, `1`, every package `ok`, and at least one line joining `briefs` for the ref. None
has been run; the code does not exist yet (OD-27).

#### What this task does not claim

- **Not multi-team features** (W4, L-67's second half), **milestones** (W4, L-72), or
  **initiatives** (W4; `initiativeId` is always `null` here).
- **Not the brief's text.** The brief is created empty; phase 4 drafts it (`model_drafted`, 05
  §5.5) after P4-18, from the desktop's promote flow (task 86).
- **Not the derived facts.** Lanes live, landed and released where are the desktop's reading of
  the child issues (task 85); nothing here stores or computes them.
- **Not that the feature's branch exists in git.** It is cut on the first Start of a child issue
  (task 24, task 61); P3-04 cutting a missing base branch is **UNVERIFIED**.
- **Not a correct AU rule per project in W2.** Every project takes the AU slug rule until the data
  class has a home on the project record (the fail-safe of task 24).

#### Corrections to the scope

1. **The brief carries no title** (decision (a)): the Hub's `BriefHead` has no such field and a
   new brief has no revision (`hub-contract.md:641-644`, `:759`). The pointer to the issue is the
   `issue.promoted` event's `issueKey` and the issue's `featureId` (decision (b)).
2. **"One event per write" is one per record written** (the events table), because the brief is a
   Hub record with its own `brief.created`.
3. **A derived fact in a `PATCH` is 400, not 422**: the strict decoder refuses an unnamed key
   (task 72 block 6), which kills the same mutant.
4. **Task 74's `features` unique `(project_id, branch_ref)`** is a copy this task does not read
   (decision (c)). Task 74 may keep `brief_id` unique instead; reported as a gap.
5. **Hub task 18's brief insert may need a `tx`-taking variant** (a Hub file edit, stream WH's
   rule H1 applies); reported.

**Parity rows served:** the Hub halves of **L-23** (promote to feature, nothing copied), **L-67**'s
W2 half (one team's feature, its lead and target date), **L-69** (the overview is the branch
brief, reserved here), **L-70** (status set by a person, facts beside it) and **L-71** (priority
and labels). Tasks 85 and 86 close them with the desktop.
