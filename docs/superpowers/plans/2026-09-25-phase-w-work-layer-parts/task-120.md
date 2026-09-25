### Task 120: milestones `ml-` as criterion subsets; feature dependencies; feature and brief templates

W4, stream WH; no seam. Depends on task 116 (the W4 tables and their guards). It also reads task
76's features and task 78's templates, both W2 and both before 116 in task order; neither part is
written at this writing, so every name taken from them is their **scope's** (`tasks-61-80.md:527`,
`:598`) and **UNVERIFIED** until those parts land. Hub anchors were read in the `feat/hub` worktree
at `1271ddb` (contract cited `hub-contract:N`; built `hubapi/envelope.go`). The wire is task 114's
part (`task-114.md`, in progress at this writing, cited `T114` by type name, since its line
numbers are still moving). Every `server/` and `store/`
path is **UNVERIFIED until merge** (rule H1).

Three planning records that each hang off a feature. A **milestone** is a named subset of the
feature's brief criteria with a date: *reached* when those criteria are settled, and never a bar
(L-72, PARTIAL on purpose, `OD-27`). A **feature dependency** says one feature blocks another, and
the timeline (task 129) colours a violated order. A **brief template** is a team's starting spec:
criteria plus the issue templates that serve them; *new feature from template* makes the feature,
its brief and its issues in one step, already on the thread (L-75, L-122).

Six decisions the scope left open, made here.

1. **The Hub refuses what it can know is wrong, and no more.** The Hub reads exactly one field of
   brief content, `status` (`hub-contract:744-745`), and criterion ids are positional, made by
   phase 4's parser. Task 09 decided that `serves` is therefore checked at the key level by the
   Hub, with membership checked by the writer (`task-09.md:28-39`). Task 114 comments that
   membership is "the Hub's check" (its `validCriterionList` comment); the two disagree. This task follows task 09:
   `criterion_not_on_brief` is answered when the Hub **holds** the brief's criterion list, which is
   true for a brief made from a template (this task records the list, decision 5), and a
   hand-written brief's criteria are checked by the writer (task 130's picker offers only parsed
   criteria). A Hub-wide check needs phase 4's writer to publish each revision's criterion ids, a
   Hub-contract edit that task 09's gap already names; it is not applied here (rule H1).
2. **The word per criterion is the Hub's own record, and says less than the design.** 04 §2.5 says
   *reached when those criteria are settled at the branch head*. The Hub has no git, so it cannot
   tell which evidence is at the head. `MilestoneView` takes each criterion's **latest recorded
   verdict** (`evidence.recorded_at`, the Hub's clock) under any revision of the feature's branch
   brief: `settled` or `contested`; with none, `open` when a live issue's head revision serves it
   and `uncovered` when none does. `drifting` (05 §3.3) needs criterion text, which only the
   desktop has; the Hub never answers it, and task 130 draws it from the replica. These four words
   are the Hub's subset of task 45's five (`task-45.md:81`); 114's `CriterionWord.Word` is a
   string (T114, `CriterionWord`), and this task fixes its values.
3. ***Reached* is every listed criterion `settled` at read**, never stored (T114 decision 6). The
   view prints the words and the flag, never "2 of 3" (`OD-27`).
4. **Dependencies stay inside one team.** Both features must be readable by the caller as a
   member of their team, and in the **same** team; a task 117 grant is read-and-comment, so a
   granted member cannot add one (403). A feature cannot block itself (422). Adding an existing
   pair answers 200 with it and appends no event (no record changed). A cycle is 422
   `dependency_cycle` from task 116's trigger, mapped by the handler, so the route and the database
   agree. `orderViolated` is `blocks.targetDate < from.targetDate` with both set, derived at read.
5. **A brief template carries the content the Hub cannot write.** The Hub never authors phase 4
   content, so *new feature from template* writes revision 1 from **content the template already
   holds**: when a person saves a brief template, the control plane (which has phase 4's schema)
   builds the content object with the template's criteria and the positional id of each, and sends
   both. The Hub validates the content as it validates any revision (`hubapi.ValidateContent`,
   `envelope.go:71`, status `open`) and stores it opaque. **Gap G1, fixed in 114:**
   `BriefTemplate` and its create and patch requests gain `content json.RawMessage`,
   `criterionIds []string` (parallel to `criteria`) and `issueCriteria []string` (parallel to
   `issueTemplateIds`: the criterion id each issue serves, or `""`); and
   `CreateBriefTemplateRequest` gains `kind` (always `"brief"`), because task 78's route decides
   the template kind from the body (T114's route table: *"`kind: "brief"`; task 78's route"*).
6. **The create is one transaction, in task 76's order.** Task 76's feature create (which reserves
   `branchRef` by creating the branch brief at head revision 0, `tasks-61-80.md:533-537`); the
   brief's revision 1 from the template's content, through the Hub's own revision write (Hub task
   19's store path for route 26, `hub-contract:588`; name **UNVERIFIED**); the criterion list
   (decision 1); then one issue per issue template through task 08's create, each `serves
   [{briefKey: brief:<bf>@1, criterionId}]` from `issueCriteria`, an empty entry leaving `serves`
   empty. Each write appends its own event (`hub-contract:1133`). Any failure rolls all of it back,
   the branch reservation included.

**The one thing this task must not get wrong** is a milestone that looks reached when it is not. A
default word (`settled` for a criterion nobody has proven), a count that stands in for the words,
or a criterion from another brief silently accepted would each tell a lead a checkpoint is met.
Decision 2's words start at `uncovered` or `open`, and only a recorded `settled` verdict moves one.

**Files:**

- `control-plane/internal/hub/migrate/sql/NNNN_work_roadmap.sql` ⚑ (task 116's file; a section
  appended, or the next free number after sync, rule H1): `work_brief_criteria` and three template
  columns
- `control-plane/internal/hub/store/work_milestones.go`, `server/work_milestones.go` (new)
- `control-plane/internal/hub/store/work_featuredeps.go`, `server/work_featuredeps.go` (new)
- `control-plane/internal/hub/store/work_templates.go`, `server/work_templates.go` ⚑ (task 78's
  pair; paths as task 78 fixes them): the `kind: "brief"` branch, and `features:fromTemplate`
- `control-plane/internal/hub/store/work_criteria.go` (new): the criterion words of decision 2 and
  the known list of decision 1, shared by the milestone view and task 118's `InitiativeView`
- `control-plane/internal/hub/server/work_roadmap120_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑: the `"120"` rows

**Interfaces:**

- Produces:
  - routes (SW.20, paths as T114's route table fixes them): `POST` and `GET
    /v1/features/{featureId}/milestones`; `PATCH /v1/milestones/{milestoneId}`; `PUT
    /v1/milestones/{milestoneId}/criteria`; `POST /v1/features/{featureId}/dependencies`; `DELETE
    /v1/features/{featureId}/dependencies/{blocks}`; `GET
    /v1/projects/{projectId}/feature-dependencies`; the `kind: "brief"` branch of task 78's `POST`
    and `GET /v1/teams/{teamId}/templates` and `PATCH /v1/templates/{templateId}`; `POST
    /v1/projects/{projectId}/features:fromTemplate`
  - `store.CriterionWords`, `store.KnownCriteria` (task 118 calls the first for its
    `InitiativeFeature.Criteria`)
  - the kinds `milestone.created`, `milestone.updated`, `milestone.criteria_set`,
    `feature.dependency_added`, `feature.dependency_removed`, `template.created`,
    `template.updated` with `kind: "brief"` (gap G2)
- Consumes:
  - T114: `Milestone`, `MilestoneView`, `CriterionWord`, `FeatureDependency`,
    `FeatureDependencyView`, `BriefTemplate`, the requests, `CodeDependencyCycle`,
    `CodeCriterionNotOnBrief`, `CodeMilestoneNotFound`; task 116: `work_milestones`,
    `work_milestone_criteria`, `work_feature_deps` and its cycle trigger, `work_brief_templates`
  - task 76: the feature row (`team_id`, `project_id`, `brief_id`, `branch_ref`, `target_date`) and
    its create path; task 78: the template route and the issue template row; task 08: the issue
    create path and `checkServes`; task 13: `emitWork`
  - the Hub: `hubapi.ValidateContent` (`envelope.go:71`, built), the revision write of route 26
    (Hub task 19), the `evidence` table (`hub-contract:1058-1082`)
- Consumed by: task 118 (criterion words), task 124 (the methods over HTTP), task 125 (the replica
  loads milestones and the project's dependency list), tasks 129 and 130 (the timeline and the
  picker)

**Where this part and task 124 disagree.** Task 124 registers the dependency list as `GET
/v1/features/{featureId}/dependencies` (its route table and *Gaps* 1). T114's route table fixes
`GET /v1/projects/{projectId}/feature-dependencies` (a `†` path, 114's own), and 114 owns SW.20,
so this part follows 114: the timeline draws a project's dependencies in one read. Task 124 renames
its row at sync; a gap for the orchestrator, not a change here.

**Gap G2:** T114 declares no W4 event kinds (task 119 names the same gap). The seven kinds above,
with scalar-only data, are 114's to append to task 13's registry: `milestone.*` `{featureId,
criteria int64}`, `feature.dependency_*` `{from, blocks}`, `template.*` `{kind}` only (task 13's
Work kinds carry no titles, `IssueCreatedData` has none, and this follows it, though the Hub's own
`team.created` carries a name). All `ScopeProject`, but `template.*`, which is `ScopeTeam`.

#### The migration section (appended to `NNNN_work_roadmap.sql`, complete)

```sql
-- ── task 120: a brief template holds the content the Hub cannot write (decision 5) ──
SET search_path = hub, pg_temp;

ALTER TABLE work_brief_templates
  ADD COLUMN content        bytea  NOT NULL,                      -- canonical, ValidateContent's bytes
  ADD COLUMN criterion_ids  text[] NOT NULL,                      -- parallel to criteria
  ADD COLUMN issue_criteria text[] NOT NULL DEFAULT '{}',          -- parallel to issue_template_ids
  ADD CONSTRAINT brief_template_lists CHECK (
        cardinality(criterion_ids) BETWEEN 1 AND 50
    AND cardinality(issue_criteria) = cardinality(issue_template_ids)
    AND octet_length(content) <= 65536);

-- The criterion list the Hub holds for a brief (decision 1): written only by
-- features:fromTemplate, one row per criterion of revision 1, never changed.
CREATE TABLE work_brief_criteria (
  org_id       text NOT NULL,
  brief_id     text NOT NULL REFERENCES briefs (id),
  revision     int  NOT NULL CHECK (revision >= 1),
  ord          int  NOT NULL CHECK (ord >= 0 AND ord < 50),
  criterion_id text NOT NULL CHECK (char_length(criterion_id) BETWEEN 1 AND 128),
  template_id  text NOT NULL,
  PRIMARY KEY (org_id, brief_id, revision, ord),
  UNIQUE (org_id, brief_id, revision, criterion_id)
);
ALTER TABLE work_brief_criteria ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_brief_criteria FORCE ROW LEVEL SECURITY;
CREATE POLICY org_isolation ON work_brief_criteria
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT ON work_brief_criteria TO hub_app;
```

`ADD COLUMN … NOT NULL` with no default is valid only while `work_brief_templates` is empty, which
holds because 116 and 120 ship in one migration file applied nowhere shared until merged (rule H1);
if 116 has merged first, the section takes the next free number and the two `NOT NULL` columns
gain `DEFAULT` values and a backfill note, not a silent change. `briefs (id)` is the Hub's table
(`0001_init.sql`, **UNVERIFIED** name until the Hub's server tree lands). A brief **erase** (Hub
task 27) leaves these rows: they hold positional ids, not text, as `evidence.criterion_id` does.

#### The wire: T114's types, and G1

`Milestone`, `MilestoneView`, `CriterionWord`, `FeatureDependency`, `FeatureDependencyView`,
`BriefTemplate`, `CreateMilestoneRequest`, `PatchMilestoneRequest`, `SetMilestoneCriteriaRequest`,
`FeatureDependencyRequest`, `CreateBriefTemplateRequest`, `PatchBriefTemplateRequest` and
`CreateFeatureFromTemplateRequest` are used as T114 declares them. G1 adds, additively:

```go
// control-plane/internal/hub/hubapi/work_roadmap.go (task 114's file) — G1

// The Hub's criterion words (decision 2): a subset of task 45's five.
const (
	CriterionSettled   = "settled"   // the latest recorded verdict is settled
	CriterionContested = "contested" // the latest recorded verdict is contested
	CriterionOpen      = "open"      // no verdict; a live issue's head revision serves it
	CriterionUncovered = "uncovered" // no verdict; no live issue serves it
)

// On BriefTemplate, CreateBriefTemplateRequest and PatchBriefTemplateRequest
// (pointer-typed on the patch, as its other fields):
//	Content       json.RawMessage `json:"content"`       // phase 4's content object, opaque here
//	CriterionIDs  []string        `json:"criterionIds"`  // parallel to Criteria; 1-128 chars each
//	IssueCriteria []string        `json:"issueCriteria"` // parallel to IssueTemplateIDs; "" = serves nothing
// On CreateBriefTemplateRequest only:
//	Kind string `json:"kind"` // always "brief"; task 78's route dispatches on it
//
// Validate: len(CriterionIDs) == len(Criteria); every IssueCriteria entry is ""
// or one of CriterionIDs; ValidateContent(Content) succeeds with status "open".
```

A template's `Content` is written by the control plane, which holds phase 4's schema; the Hub
checks only what `ValidateContent` checks (a JSON object, `status`, integers, depth, 64 KiB,
`hub-contract:747-752`). That the ids in `CriterionIDs` are the ids phase 4's parser gives the
criteria in `Content` is the writer's claim, as `serves` is (task 09 decision 4).

#### The store (signatures complete)

```go
package store

// store/work_criteria.go

// KnownCriteria is the criterion list the Hub holds for a brief (decision 1):
// the work_brief_criteria rows of its revision 1, or ok == false when the brief
// was not made from a template. Never a guess.
func KnownCriteria(ctx context.Context, q Querier, briefID string) (ids []string, ok bool, err error)

// CriterionWords is decision 2, one word per id, in ids order:
//   latest verdict  SELECT DISTINCT ON (criterion_id) criterion_id, verdict FROM evidence
//                    WHERE brief_key LIKE 'brief:' || $1 || '@%' AND criterion_id = ANY($2)
//                    ORDER BY criterion_id, recorded_at DESC, id DESC
//   else served     EXISTS a live issue whose head revision's issue_serves row names
//                    (a revision of $1, criterion_id)  → "open", otherwise "uncovered".
// "Live" is not erased and not cancelled or duplicate (task 10's closed words).
func CriterionWords(ctx context.Context, q Querier, briefID string, ids []string) ([]hubapi.CriterionWord, error)

// store/work_milestones.go
func InsertMilestone(ctx context.Context, tx pgx.Tx, m MilestoneRow) error
func PatchMilestone(ctx context.Context, tx pgx.Tx, id string, name *string, date *string, clearDate bool, at time.Time) (MilestoneRow, error)
func SetMilestoneCriteria(ctx context.Context, tx pgx.Tx, id string, criterionIDs []string, at time.Time) error
func MilestonesOf(ctx context.Context, q Querier, featureID string) ([]MilestoneRow, error) // by date, nulls last, then created_at
func MilestoneByID(ctx context.Context, q Querier, id string) (MilestoneRow, bool, error)
type MilestoneRow struct {
	ID, FeatureID, Name  string
	Date                 *string
	CriterionIDs         []string // work_milestone_criteria, in ord order
	CreatedAt, UpdatedAt time.Time
}

// store/work_featuredeps.go
// AddFeatureDependency inserts (from, blocks); added is false when the pair
// existed. Task 116's trigger raises SQLSTATE 'P0001' with the message prefix
// 'hub: dependency_cycle', which this function returns as ErrDependencyCycle.
func AddFeatureDependency(ctx context.Context, tx pgx.Tx, org, from, blocks string, at time.Time) (added bool, err error)
func RemoveFeatureDependency(ctx context.Context, tx pgx.Tx, from, blocks string) (removed bool, err error)
// DependenciesOfProject lists every pair with either end in the project, with
// both ends' target dates for orderViolated; the handler drops pairs whose
// other end the caller cannot read.
func DependenciesOfProject(ctx context.Context, q Querier, projectID string) ([]DependencyRow, error)
type DependencyRow struct {
	From, Blocks                   string
	FromTeam, BlocksTeam           string
	FromTarget, BlocksTarget       *string // YYYY-MM-DD
}
var ErrDependencyCycle = errors.New("store: the dependency closes a cycle")

// store/work_templates.go ⚑ (task 78's file), the brief half
func InsertBriefTemplate(ctx context.Context, tx pgx.Tx, t BriefTemplateRow) error
func PatchBriefTemplate(ctx context.Context, tx pgx.Tx, id string, p hubapi.PatchBriefTemplateRequest, canonical []byte, at time.Time) (BriefTemplateRow, error)
func BriefTemplatesOf(ctx context.Context, q Querier, teamID string) ([]BriefTemplateRow, error)
func BriefTemplateByID(ctx context.Context, q Querier, id string) (BriefTemplateRow, bool, error)
func InsertBriefCriteria(ctx context.Context, tx pgx.Tx, org, briefID string, revision int, ids []string, templateID string) error
```

The trigger's SQLSTATE and message prefix are task 116's to fix; if its part chooses others, the
mapping follows it. `DependencyRow` carries both teams so the handler can apply decision 4 and the
read rule without a second query.

#### The routes

```go
mux.HandleFunc("POST /v1/features/{featureId}/milestones", s.createMilestone)
mux.HandleFunc("GET /v1/features/{featureId}/milestones", s.listMilestones)
mux.HandleFunc("PATCH /v1/milestones/{milestoneId}", s.patchMilestone)
mux.HandleFunc("PUT /v1/milestones/{milestoneId}/criteria", s.setMilestoneCriteria)
mux.HandleFunc("POST /v1/features/{featureId}/dependencies", s.addFeatureDependency)
mux.HandleFunc("DELETE /v1/features/{featureId}/dependencies/{blocks}", s.removeFeatureDependency)
mux.HandleFunc("GET /v1/projects/{projectId}/feature-dependencies", s.listFeatureDependencies)
mux.HandleFunc("POST /v1/projects/{projectId}/features:fromTemplate", s.createFeatureFromTemplate)
// task 78's POST/GET /v1/teams/{teamId}/templates and PATCH /v1/templates/{templateId}
// gain the brief branch in their own handlers; no new pattern.
```

The `evidence` columns (`brief_key`, `criterion_id`, `verdict`, `recorded_at`) are named from the
contract's `EvidenceRecord` (`hub-contract:1076-1082`); the Hub's table names are **UNVERIFIED**
until its server tree lands. The `LIKE 'brief:' || $1 || '@%'` match is exact because a `bf-` id is
fixed-width hex and cannot contain `%`, `_` or `@`.

**Milestones.** `createMilestone`, inside `s.Write`: the feature readable as a member or admin of
its team (else 404 `feature_not_found`; a viewer or a granted member 403 `required: member`);
`Decode` and `Validate`; if `KnownCriteria` of the feature's brief is `ok`, every id must be in it,
else 422 `criterion_not_on_brief`, `details.field: "criterionIds[n]"`; `InsertMilestone`; one
`milestone.created`; 201 `MilestoneView` built as the read builds it. `setMilestoneCriteria` is the
same check on the new list, one `milestone.criteria_set`, 200. `patchMilestone` changes `name` and
`date` only (`null` clears the date), one `milestone.updated`, 200; unknown or unreadable id → 404
`milestone_not_found`. `listMilestones`, inside `s.Read`, readable also by a viewer and a granted
member: `MilestonesOf`, then **one** `CriterionWords` call over the union of their ids, so N
milestones cost one evidence query; each view's `reached` is `every word == "settled"`, and a
milestone with no criteria cannot exist (T114's `validCriterionList` needs 1-100).

**Dependencies.** `addFeatureDependency`: `{featureId}` is the blocker (`from`), the body's
`blocks` the blocked. Both readable as a member (404 `feature_not_found` naming the one that is
not); a granted member 403; `from == blocks` → 422 `field: "blocks"`; different teams → 422
`field: "blocks"`, `reason: "other_team"` (decision 4); `AddFeatureDependency`; a cycle → 422
`dependency_cycle` with `details.from`, `details.blocks`; added → one `feature.dependency_added`,
201; existed → 200, no event. The answer is `FeatureDependencyView` with `orderViolated` computed.
`removeFeatureDependency`: removed → one `feature.dependency_removed`, 204; absent → 204, no event.
`listFeatureDependencies`: the project readable (404 `project_not_found`), `DependenciesOfProject`,
pairs whose other end the caller cannot read dropped, `200 []FeatureDependencyView`.

**Brief templates.** Task 78's `POST /v1/teams/{teamId}/templates` decodes `kind` first
(`struct{ Kind string }`, lenient) and hands `"brief"` to `createBriefTemplate`, which decodes
strictly into G1's `CreateBriefTemplateRequest`: team admin only (403 `required: admin`), G1's
`Validate`, `ValidateContent(content)` → its canonical bytes (a `FieldError` is 422 under
`content.`); status other than `open` → 422 `field: "content.status"`; every `issueTemplateIds`
entry a live issue template of the team (422 `field: "issueTemplateIds[n]"`); one
`template.created` `{kind: "brief"}`; 201 `BriefTemplate`. `GET …/templates?kind=brief` lists the
team's brief templates; `PATCH /v1/templates/{templateId}` on a brief template runs the same checks
on what it changes and appends `template.updated`. A template edit changes no feature already
made from it (L-26's rule, task 78): nothing points back at the template but
`work_brief_criteria.template_id`, which is history.

**`createFeatureFromTemplate`**, inside **one** `s.Write`, in decision 6's order:

1. The project readable as a member or admin (404 `project_not_found`; viewer or granted 403);
   `Decode` `CreateFeatureFromTemplateRequest`; the template a live brief template of the
   project's team (404 `template_not_found`, task 78's code, **UNVERIFIED** name).
2. Task 76's feature create with `{name, slug: <branchRef without refs/heads/>}`, which creates
   the branch brief at head revision 0 and appends `feature.created` and `brief.created`; a taken
   ref is task 76's 409 `brief_exists`, and nothing is written.
3. The brief's revision 1: the template's canonical `content` through the revision write of route
   26 (the Hub's envelope, digest and `brief.revised` event), status `open`.
4. `InsertBriefCriteria(brief, 1, template.criterionIds, template.id)`, with no event of its own:
   the list is part of revision 1's write, as `issue_serves` rows are part of an issue revision's.
5. For each issue template, in order: task 08's create with the template's pre-fill (task 78's
   rule: a copy at creation, never a link), `source: "template"` if task 08's `Source` enum has it
   (else `"desktop"`, **UNVERIFIED**), and `serves` from `issueCriteria[i]`; each appends its
   `issue.created`.
6. 201 `Feature`, T114's `CreateFeatureFromTemplate` answer. Task 124's *Gaps* 2 asks for a richer
   `FeatureFromTemplate{Feature, Brief, Issues}`; if 114 adopts it, step 6 answers that instead.

A keyed request (`Idempotency-Key`, task 124's row marks it idempotent) stores its reply with the
feature's `name` blanked, task 14's pattern, since a later feature erase is W2's and must find no
copy. **Any error at any step returns before commit**, so the database and the feed hold none of
it; `brief_exists` from a stale earlier attempt is not special-cased.

#### Tests to write first — `server/work_roadmap120_integration_test.go` (`//go:build integration`)

The cast is task 11's (Ada, Portal's admin; Grace, a member; Linus, Billing only; Xavier, another
org), a Portal viewer and task 117's granted member (named as 117's fixture names them). Fixture:
brief template **TP-1** on Portal's team with criteria `done:1`, `done:2`, `done:3` and four issue
templates mapped to `done:1`, `done:2`, `""`, `done:3`; feature FT-1 made from TP-1; a
hand-written feature FT-2 whose brief has revision 2 written by phase 4's fixture writer; evidence
posted through `POST /v1/evidence` as the tests need it.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestACriterionOffTheBriefIsRefused` | on FT-1, `criterionIds: ["done:1", "done:9"]` → 422 `criterion_not_on_brief`, `field: criterionIds[1]`, nothing written; on FT-2, `done:9` → 201 (the writer's check, decision 1) | any criterion accepted where the Hub knows the list |
| `TestReachedFlipsOnlyOnTheLast` | milestone M on FT-1 over `done:1`, `done:2`: `reached: false`; `settled` on `done:1` → still false; `settled` on `done:2` → true; a later `contested` on `done:1` → false | reached on any; reached stored and stale |
| `TestWordsStartUncoveredOrOpen` | before any evidence, `done:1` (served by an issue) reads `open` and a criterion no issue serves reads `uncovered`; never `settled` | a default optimistic word |
| `TestTheViewCountsNothing` | the JSON of `GET …/milestones` has no key matching `count`, `total`, `of`, `percent`, `progress` or `remaining`, and no number but dates | "N of M" or a bar (OD-27) |
| `TestACycleIsRefused` | FT-1 blocks FT-2 → 201; FT-2 blocks FT-1 → 422 `dependency_cycle`; the same insert as `hub_app` straight to SQL → refused by the trigger | cycle accepted (route or trigger alone) |
| `TestOrderViolatedIsDerived` | FT-1 (target 2026-11-01) blocks FT-2 (target 2026-10-15) → `orderViolated: true`; PATCH FT-2's target to 2026-11-20 → the list reads false with no dependency write | stored and stale |
| `TestDependenciesStayInOneTeam` | a Portal feature blocking a Billing feature → 422 `reason: other_team`; self → 422; the granted member → 403; an existing pair → 200 and no event | cross-team writes; a grant that writes; a duplicate event |
| `TestATemplateMakesAThreadedFeature` | `features:fromTemplate` on TP-1 → one feature, its brief at revision 1 with TP-1's content bytes, four issues whose head `serves` are `done:1`, `done:2`, `[]`, `done:3` under `brief:<bf>@1`; the issues read `threadState` `on_thread`, `on_thread`, `not_tied`, `on_thread` | serves dropped; the Hub inventing content |
| `TestAFailedIssueRollsTheFeatureBack` | the fourth issue template pre-fills an archived label, so its create is 422: no feature, no brief (the ref is free: a second attempt without that template → 201), no issues, and no event after the request's first seq | half a feature |
| `TestATemplateEditChangesNoFeature` | PATCH TP-1's criteria after FT-1 was made → FT-1's brief, its criterion list and its issues are unchanged | a live link to the template |
| `TestOnlyAnAdminWritesBriefTemplates` | Grace's `POST …/templates {kind: brief}` → 403 `required: admin`; Ada's with content status `complete` → 422 `content.status`; `issueCriteria` naming `done:7` → 422 | a member edits the team's spec template; an unvalidated template |
| `TestEachWriteAppendsItsEvents` | a milestone create, a criteria set, a dependency add and remove → one event each; `fromTemplate` with four issues → exactly `feature.created`, `brief.created`, `brief.revised` and four `issue.created`, in that order | a write without its event; an event per request instead of per record |
