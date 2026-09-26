### Task 78: sub-issues and the parent rule; templates `tp-`; label groups and org labels

**Slice:** W2. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 74 (`issue_heads.parent_id` and its loop trigger, `templates`, `label_groups`,
`labels.group_id`, the nullable `labels.team_id`, the exclusive trigger), after a sync-merge of
`feat/hub` (rule H1). **Seams:** consumes **SW.18** (task 72's `Template`, `LabelGroup`,
`LabelPlan`, `LabelPlanRequest`, `CodeParentLoop`, `CodeParentOtherProject`, `CodeChildrenOpen`,
`CodeExclusiveLabelGroup`, `CodeTemplateNotFound`) and **SW.4** (task 10's done rule); adds one
cause to SW.4, one `Scope` to SW.5, and six event kinds, all additive.

Three small things a team expects on day one of planning. **Sub-issues**: a parent issue holds
children, and Linear closes the parent when the last child closes. Ours closes it only when the
children are closed **and** every criterion the parent serves is settled (L-21, `05-design.md:130`),
so a parent cannot read *done* while the brief it answers is still open. **Templates**: a team or
the org keeps issue shapes that may pre-fill `serves`, so a new issue starts on the thread.
**Label groups**: a group of labels, optionally exclusive (one *Type* per issue), and labels that
belong to the whole org.

**Anchors, re-read on 2026-09-26.** In the Hub worktree at `6463e568` (read, not merged here):
evidence and its `verdict` `settled` | `contested` (`hub-contract.md:1060-1083`); roles, and the
org admin who reads no project content (`:331`); the matrix columns (Hub `task-16b.md:30-35`). In
this branch's parts (**UNVERIFIED until merge**): task 72's `Template` (`task-72.md:581-598`),
`LabelGroup` (`:600-607`), `LabelPlan` and decision (c) on `Label.TeamID` (`:44-47`, `:432-440`),
codes (`:354`, `:360`, `:362`, `:364-365`); task 10's route, proof words and `DecideProof`
(`task-10.md:47-55`, `:402-435`); task 11's label store and the `labels` list path
(`task-11.md:280-284`, `:369-377`); task 13's `Scope` (`task-13.md:234-240`) and registry
(`:250-270`); task 07's `werase` cells (`task-07.md:388-390`). **Task 74's part is not written at
this writing**; its columns and trigger names are the scope's (`tasks-61-80.md:477-483`),
**UNVERIFIED**.

#### Decisions the scope left open, made here

**(a) Children are one level of people's issues, in the same project.** A parent and its child
share `project_id` (else 422 `parent_other_project`); an issue may be a parent and a child at once
(nesting is allowed, since Linear allows it), but no chain may loop (422 `parent_loop`, checked by a
walk up the parent chain in Go and by task 74's trigger). The walk is capped at 32 steps; a longer
chain is refused as a loop (a chain that deep is a data error). Agent tasks below a person's issue
are team mode's (`tk-`, TM-05 §3.3) and never appear in this list (05 L-20).

**(b) "Settled", as the Hub can read it.** A criterion `(briefKey, criterionId)` is **settled for
the Hub** when the **latest** evidence record for it in the project (by `recorded_at`, then id) has
`verdict: settled`. 04 §5.3's full reading ("all of its evidence holds at the head") needs git,
which the Hub never reads; a later `contested` record un-settles it here too. The two readings may
differ when a settled record's commit is no longer at the head; that difference is phase 4's to
show (task 87's convergence words), and is stated in *does not claim*.

**(c) Two ways a parent reaches `done`, and one hold.**
- **A parent with its own work** (it `landed` through its own lane): task 10's evidence rule
  decides, and **open children hold it**, exactly as task 75's `after_landed` sub-state does: the
  evidence move is accepted, citations and proof stored, the state left `landed`.
- **A container parent** (it never landed; any state before `landed`): when the **last open child
  closes**, the server runs the **roll-up**: every child closed, at least one child `done` (a parent
  whose children were all cancelled was abandoned, and a person decides), and every criterion the
  parent serves settled (decision (b)). Then the parent moves to `done` with the new cause
  `children_closed`, **written as the person whose act closed the last child** (skeleton rule 2).
- **A hand-set `done`** on a parent with an open child is 409 `children_open`, `details.open` the
  count, nothing written.

**(d) The roll-up's proof word is the weakest of the done children's.** The order is `not_proven`
< `proven_self_attested` < `proven`; `no_criteria` children are neutral; all neutral gives
`no_criteria`. So a parent reads *proven* only if every child that served a criterion was proven,
and a parent of hand-closed children reads *done, not proven*, as they did. This is the
"inheritance" of L-20, and it keeps 05 §3.3's glyphs honest at the parent.

**(e) `children_closed` is a new fact cause, and it bends rule 2 on purpose.** Skeleton rule 2
says `done` needs the issue's own evidence at its own land commit. A container parent has no land
commit; its work is its children's, each proven under rule 2 on its own. 05 L-21 asks for the
roll-up, so the cause exists, the database guard checks it (no open child, at the time of the
write), and the proof word never claims more than the children proved (decision (d)). This is
**reported to the orchestrator** as a rule-2 amendment for the owner, not assumed.

**(f) A template is a record, not a link** (L-26). Creating an issue from one is a client prefill
(task 88): the desktop reads the template and sends an ordinary `CreateIssueRequest`, so no server
merge exists and a template edit changes no issue. The new issue's text is the **creator's**
`text·H` once they send it; the template's author is not carried onto the issue.

**(g) An org template pre-fills no `serves` and no estimate.** An org admin reads no project
content (`hub-contract.md:331`), so they cannot name a project brief's criterion, and an org has no
estimate scale (the scale is a team's, task 79). An org template with a non-empty `serves` or an
`estimate` is 422 `bad_request` naming the field. A team template's `serves` must each resolve to
a live brief revision's criterion in one of the team's projects (task 09's serves check, reused),
and its `estimate` must satisfy `hubapi.ValidEstimate` for the team's policy.

**(h) Org labels and org groups are org records with a `null` team.** They are written by an org
admin only, visible to every team of the org, and joinable only to org groups (a team label may
join a team group or an org group; an org label, an org group only). Task 11's `GET
/v1/teams/{teamId}/labels` answers the team's labels **and** the org's, org first. Label names stay
unique per team for team labels (task 11's index) and per org for org labels; a team label may
share a name with an org label, and the picker shows the org one with its group prefix (task 90).

**(i) Org-scoped events need a third `Scope`.** Task 13's `Scope` has `ScopeProject` and
`ScopeTeam` (`task-13.md:234-240`); an org label has neither. `ScopeOrg` (both ids null) is added,
visible to every signed-in member of the org, and task 13's feed query (`store/changes.go`) gains
one `OR scope = 'org'` clause. That is a delta to task 13's file, stated in *Files*.

#### Files

| Path | Change |
|---|---|
| `control-plane/internal/hub/store/work_parents.go`, `server/work_parents.go` | new: the parent route, the loop walk, the roll-up |
| `control-plane/internal/hub/store/work_templates.go`, `server/work_templates.go` | new: team and org templates |
| `control-plane/internal/hub/store/work_labels_plan.go`, `server/work_labels_plan.go` | new: groups, org labels, the exclusive check |
| `control-plane/internal/hub/server/work_parents_integration_test.go`, `work_templates_integration_test.go`, `work_labels_plan_integration_test.go` | new: the tests below |
| `control-plane/internal/hub/server/work_states.go` ⚑ (task 10's) | the children hold, the `children_open` refusal, the roll-up call after any close |
| `control-plane/internal/hub/server/work_labels.go` ⚑ (task 11's) | the label create takes `groupId`; the list adds org labels |
| `control-plane/internal/hub/server/work_issues.go` ⚑ (task 08's) | the create and `PATCH` label paths call the exclusive check |
| `control-plane/internal/hub/hubapi/work.go` ⚑ | `CauseChildrenClosed`, its `MoveAllowed` rows, `RollupProof` |
| `control-plane/internal/hub/hubapi/work_events.go` ⚑ (task 13's) | `ScopeOrg`, six kinds |
| `control-plane/internal/hub/store/changes.go` ⚑ (task 13's) | the feed reads `ScopeOrg` events for every member of the org |
| `control-plane/internal/hub/migrate/sql/NNNN_work_plan.sql` ⚑ (task 74's) | appended: the `children_closed` clause of the head guard |
| `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ | `workMatrixRows["78"]`, the `worg` helper; `"78"` in `workMatrixLive` |
| `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ (H16b's) | the fixture gains `ChildIssueID`, `TemplateID`, `GroupID` |
| `control-plane/internal/hub/server/server.go` ⚑ | `s.registerWorkParents(mux)`, `s.registerWorkTemplates(mux)`, `s.registerWorkLabelsPlan(mux)` |

#### The wire

```text
PUT   /v1/issues/{issueId}/parent               member     200 IssueHead          SetParentRequest
GET   /v1/issues/{issueId}/children             viewer     200 Page[IssueHead]    by rank, then number
POST  /v1/teams/{teamId}/templates              admin      201 Template           CreateTemplateRequest
POST  /v1/org/templates                         org admin  201 Template           CreateTemplateRequest (serves [], no estimate)
GET   /v1/teams/{teamId}/templates              viewer     200 Page[Template]     the team's, then the org's; ?includeArchived=false
PATCH /v1/templates/{templateId}                admin / org admin  200 Template   PatchTemplateRequest
POST  /v1/templates/{templateId}/archive        admin / org admin  200 Template
POST  /v1/teams/{teamId}/label-groups           admin      201 LabelGroup         CreateLabelGroupRequest
GET   /v1/teams/{teamId}/label-groups           viewer     200 Page[LabelGroup]   the team's, then the org's
POST  /v1/org/label-groups                      org admin  201 LabelGroup         CreateLabelGroupRequest
POST  /v1/org/labels                            org admin  201 Label              CreateLabelRequest (task 11's, with groupId)
```

`GET /v1/issues/{issueId}/children` and `GET /v1/teams/{teamId}/label-groups` are not in the
scope's list; a parent's page and a label picker cannot draw what they cannot read, so both are
added. Task 72's block 6 bodies, shown so the part stands alone (task 72 wins on any spelling):

```go
type SetParentRequest struct {
	ParentID *string `json:"parentId"` // is-… in the same project; null clears
}

type CreateTemplateRequest struct {
	Name        string       `json:"name"`                  // 1-80 characters
	Title       string       `json:"title"`                 // 0-240; the prefill, not an authored answer
	Description AuthoredText `json:"description"`           // text·H of the template's author
	Serves      []Serve      `json:"serves,omitempty"`      // team templates only
	Labels      []string     `json:"labels,omitempty"`      // exclusive-checked
	Priority    Priority     `json:"priority,omitempty"`    // "" is none
	Estimate    *int         `json:"estimate,omitempty"`    // team templates only; ValidEstimate
}

// PatchTemplateRequest: tri-state (task 72 decision (d)); estimate may clear.
type PatchTemplateRequest struct {
	Name        json.RawMessage `json:"name,omitempty"`
	Title       json.RawMessage `json:"title,omitempty"`
	Description json.RawMessage `json:"description,omitempty"`
	Serves      json.RawMessage `json:"serves,omitempty"`
	Labels      json.RawMessage `json:"labels,omitempty"`
	Priority    json.RawMessage `json:"priority,omitempty"`
	Estimate    json.RawMessage `json:"estimate,omitempty"`
}

type CreateLabelGroupRequest struct {
	Name      string `json:"name"` // 1-40; unique per team, or per org for an org group
	Exclusive bool   `json:"exclusive"`
}
```

#### The `hubapi` additions (complete)

```go
// hubapi/work.go ⚑, beside task 10's causes (SW.4, additive).
// CauseChildrenClosed is the roll-up of a container parent (decision (c)). No caller sends it:
// the state route refuses it from a body (400 naming cause); the server writes it itself.
const CauseChildrenClosed StateCause = "children_closed"

// RollupProof is decision (d): the weakest word of the done children, no_criteria neutral.
// It answers "" for no done child, which the caller reads as "do not roll up".
func RollupProof(doneChildren []ProofWord) ProofWord {
	rank := map[ProofWord]int{ProofNotProven: 0, ProofSelfAttested: 1, ProofProven: 2}
	out, seen := ProofNoCriteria, false
	for _, w := range doneChildren {
		seen = true
		r, counts := rank[w]
		if !counts {
			continue // no_criteria, or a word a done issue never holds
		}
		if cur, ok := rank[out]; !ok || r < cur {
			out = w
		}
	}
	if !seen {
		return ""
	}
	return out
}
```

The proof-word constant names (`ProofNotProven`, `ProofSelfAttested`, `ProofProven`,
`ProofNoCriteria`) are task 10's six (`task-10.md:47-55`); their Go spellings are **UNVERIFIED**
until task 10's `work.go` block lands, and a different spelling is a rename here only.
`MoveAllowed` gains the rows `(triage|backlog|todo|started|in_review, done, children_closed)`:
five rows, and task 10's table test gains them. `landed` is absent: a landed parent closes by its
own evidence (decision (c)'s first bullet).

```go
// hubapi/work_events.go ⚑, task 13's Scope gains one value (SW.5, additive).
const ScopeOrg Scope = 2 // after ScopeProject (0) and ScopeTeam (1); both ids null; every org member reads it

const (
	EventIssueParentSet       = "issue.parent_set"
	EventTemplateCreated      = "template.created"
	EventTemplateUpdated      = "template.updated"
	EventTemplateArchived     = "template.archived"
	EventLabelGroupCreated    = "label_group.created"
	EventIssueRolledUp        = "issue.rolled_up"
)

type IssueParentSetData struct {
	From string `json:"from"` // is-… or ""
	To   string `json:"to"`
}
type TemplateData struct {
	TemplateID string `json:"templateId"`
	Org        bool   `json:"org"`    // an org template
	Fields     string `json:"fields"` // updated only: comma-joined names
}
type LabelGroupCreatedData struct {
	GroupID   string `json:"groupId"`
	Org       bool   `json:"org"`
	Exclusive bool   `json:"exclusive"`
}
// IssueRolledUpData follows the parent's issue.state event: which child's close did it.
type IssueRolledUpData struct {
	ChildID  string `json:"childId"`
	Children int64  `json:"children"` // the parent's children, all closed
}
```

Registry rows: `issue.parent_set` and `issue.rolled_up` are `ScopeProject, VisibleToReaders`;
the three `template.*` and `label_group.created` are `ScopeTeam` for a team record and `ScopeOrg`
for an org record. A kind is registered once, so each registers `ScopeTeam`, and the emitter sets
`ScopeOrg` when the record's team is null; task 13's registry test (by the name task 13 gives it)
gains that one exception. Task 11's `label.created` follows the same rule for an org label. A template's **name is not in `data`**.

#### The migration clause, appended to task 74's file (complete)

```sql
-- Task 78: a children_closed move to done needs every child closed (decision (e)).
-- Added inside task 74's issue_heads guard function (its name is task 74's, UNVERIFIED).
  IF NEW.state = 'done' AND OLD.state IS DISTINCT FROM 'done'
     AND current_setting('hub.work_cause', true) = 'children_closed' THEN
    IF NOT EXISTS (SELECT 1 FROM hub.issue_heads c WHERE c.parent_id = NEW.issue_id) THEN
      RAISE EXCEPTION 'hub: work: children_closed on an issue with no children' USING ERRCODE = '42501';
    END IF;
    IF EXISTS (SELECT 1 FROM hub.issue_heads c WHERE c.parent_id = NEW.issue_id
               AND c.state NOT IN ('done', 'cancelled', 'duplicate')) THEN
      RAISE EXCEPTION 'hub: work: children_closed with an open child' USING ERRCODE = '42501';
    END IF;
  END IF;
```

The settled check (decision (b)) stays in Go: it reads evidence, which a row trigger on
`issue_heads` should not. So the database guards *children closed*, and Go guards *criteria
settled*; the test `TestTheRollupNeedsSettledCriteria` pins the Go half.

#### The store (signatures complete)

```go
package store

// Inside the caller's transaction, after hub.org_id and the org's audit head lock (task 11).

// work_parents.go
// ParentChain answers the ids from id's parent upward, at most max+1 entries; the caller reads
// a chain longer than max as a loop (decision (a)).
func ParentChain(ctx context.Context, q Querier, id string, max int) ([]string, error)
// SetParent: UPDATE issue_heads SET parent_id = $2, updated_at = $3 WHERE issue_id = $1; ""
// writes NULL. Task 74's loop trigger answers a 23514 starting "hub: work: parent", ErrParentLoop.
func SetParent(ctx context.Context, tx pgx.Tx, issueID, parentID string, at time.Time) error
// ChildStates answers each child's id, state and proof word.
func ChildStates(ctx context.Context, q Querier, parentID string) ([]ChildState, error)
type ChildState struct {
	ID    string
	State hubapi.IssueState
	Proof hubapi.ProofWord
}
func Children(ctx context.Context, q Querier, parentID string, pr hubapi.PageRequest) (hubapi.Page[string], error) // ids, by rank then number
// LatestVerdicts answers, per (briefKey, criterionId), the verdict of the latest evidence record
// in the project (decision (b)); a pair with no record is absent.
func LatestVerdicts(ctx context.Context, q Querier, projectID string, pairs []hubapi.Serve) (map[hubapi.Serve]hubapi.Verdict, error)
var ErrParentLoop = errors.New("store: the parent trigger refused a loop")

// work_templates.go
func InsertTemplate(ctx context.Context, tx pgx.Tx, org string, t hubapi.Template) error
func TemplateByID(ctx context.Context, q Querier, id string) (t hubapi.Template, ok bool, err error)
func ListTemplates(ctx context.Context, q Querier, teamID string, withArchived bool, pr hubapi.PageRequest) (hubapi.Page[hubapi.Template], error) // team's, then org's
func PatchTemplate(ctx context.Context, tx pgx.Tx, next hubapi.Template, at time.Time) (changed []string, err error)
func ArchiveTemplate(ctx context.Context, tx pgx.Tx, id string, at time.Time) (changed bool, err error)

// work_labels_plan.go
func InsertLabelGroup(ctx context.Context, tx pgx.Tx, org string, g hubapi.LabelGroup) error // 23505 → ErrGroupExists
func LabelGroupByID(ctx context.Context, q Querier, id string) (g hubapi.LabelGroup, ok bool, err error)
func ListLabelGroups(ctx context.Context, q Querier, teamID string, pr hubapi.PageRequest) (hubapi.Page[hubapi.LabelGroup], error)
// ExclusiveClash answers the first exclusive group holding two or more of labelIDs, and those
// labels in list order; ok false when none clashes. Pure over GroupsOf's answer.
func ExclusiveClash(labelIDs []string, groupOf map[string]GroupRef) (groupID string, labels []string, ok bool)
type GroupRef struct {
	GroupID   string
	Exclusive bool
}
func GroupsOf(ctx context.Context, q Querier, labelIDs []string) (map[string]GroupRef, error)
var ErrGroupExists = errors.New("store: a label group of that name exists")
```

Task 11's `InsertLabel` takes the label's `GroupID` (the embedded `LabelPlan`), and its
`ListLabels` gains the org's labels (`team_id IS NULL`), org first; both are edits to task 11's
store file, listed under task 11's route file in *Files* because they ship together.

#### The routes, in order

Each refusal rolls the transaction back and appends nothing.

**`putParent`** (`PUT /v1/issues/{issueId}/parent`, member): `readableIssue(…, RoleMember,
lock=true)`; `parentId` `null` clears; else the parent through the same readable lookup → 404
`issue_not_found` naming `details.issueId`; another project → 422 `parent_other_project`; the
parent is the issue itself, or `ParentChain(parent, 32)` contains the issue or is longer than 32 →
422 `parent_loop`, `details.parentId`. Equal → 200, no event. `SetParent`; `ErrParentLoop` → the
same 422 (the second guard, reached only by a race the audit lock prevents). Event
`issue.parent_set {from, to}`. Then, when the **old** parent is now childless or its remaining
children are all closed, the roll-up of the old parent runs (a child moved away may be the last
open one). 200 `IssueHead`.

**The roll-up**, one function, `s.rollUp(ctx, tx, parentID, closerID, childID)`, called after any
write that closes a child or moves it away:

1. Lock the parent's head (task 08's `LockIssue`). Its state `done`, `cancelled` or `duplicate`
   → stop. Its state `landed` → the **held** path: its proof word (`ProofOf`) is `proven` or
   `proven_self_attested` and no child is open → move to `done` with cause `evidence_recorded`
   if any live citation was recorded by hand, else `evidence_observed`, as the closer (task 75's
   release, the same code path). Stop.
2. `ChildStates`; any open → stop. `RollupProof` over the `done` children → `""` → stop (all
   cancelled, decision (c)).
3. The parent's served pairs; `LatestVerdicts`; any pair absent or `contested` → stop (the parent
   waits; the desktop shows which criterion, task 88).
4. `SetWorkCause(tx, children_closed)`; `MoveIssueState(parent, done, "", now)`; `PutProof` with the
   roll-up word. Events: task 10's `issue.state {from, to: done, cause: children_closed, proof}`,
   then `issue.rolled_up {childId, children}`. The actor of both is the **closer**.
5. The parent may itself be a child: `rollUp(grandparent, closerID, parentID)`, up the chain
   (bounded by decision (a)'s 32).

**The four insertions in task 10's route** (`server/work_states.go` ⚑):

- **Step 2:** a body's `cause: children_closed` → 400 `field: "cause"`.
- **Step 4, `hand` into `done`:** an open child → 409 `children_open`, `details.open`, before the
  reason check.
- **Step 4, the evidence causes:** a `proven` word on a parent with an open child → stays
  `landed`, citations and word stored, 200 (the hold of decision (c)).
- **After step 6, when the move closed the issue** (`done`, `cancelled`, `duplicate`) and it has a
  parent: `s.rollUp(parent, caller, issue)`, in the same transaction, so the parent's close is
  written as the person whose act closed the child (skeleton rule 2).

**Templates.** `postTeamTemplate` (admin) and `postOrgTemplate` (org admin, the Hub's org-admin
check used by erase, `task-07.md:388-390`): `name` 1-80; `title` ≤ 240; `description` task 02's
`AuthoredText` check with author `H`; `serves` (team only) through task 09's serves check against
the team's projects → else 422 naming `serves[n]`; `labels` live and exclusive-checked → 422
`exclusive_label_group`; `priority`; `estimate` (team only) `ValidEstimate(TeamPolicy(team))` →
else 422 `estimate_off_scale`. An org body with `serves` or `estimate` → 422 naming it (decision
(g)). New `tp-` from `ids.Wide(hubapi.PrefixTemplate)`; `template.created`. `listTemplates`
(viewer): team then org. `patchTemplate` and `archiveTemplate`: a team template needs the team's
admin, an org template the org admin; anyone else who can read it → 403, anyone who cannot → 404
`template_not_found`. A second archive → 200, no event. **No route reads a template to build an
issue** (decision (f)).

**Label groups and org labels.** `postTeamLabelGroup` (admin), `postOrgLabelGroup` (org admin):
`name` 1-40 → `ErrGroupExists` → 409 `label_exists` (task 11's code, `details.groupId`), new `lb-`
(`task-72.md:602`: groups share the label prefix); `label_group.created`. `postOrgLabel` (org
admin): task 11's label checks, `team_id` NULL, `groupId` an **org** group → else 422
`bad_request` `field: "groupId"`; task 11's `label.created`, org-scoped. Task 11's `postLabel`
gains the same `groupId` check with a team or org group.

**The exclusive check**, one call, `ExclusiveClash(GroupsOf(labels))`, in task 08's create and
`PATCH` label paths (`task-11.md:372-374`) and in the template routes → 422
`exclusive_label_group`, `details.groupId`, `details.labels` comma-joined. Task 74's trigger is the
second guard; its refusal maps to the same answer.
