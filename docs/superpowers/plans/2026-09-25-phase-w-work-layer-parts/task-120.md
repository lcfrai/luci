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
