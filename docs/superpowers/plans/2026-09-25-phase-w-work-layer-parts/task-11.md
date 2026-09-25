### Task 11: relations, team labels, `triage` and team policies

W1, stream WH; no seam of its own. Depends on task 08 (issue heads, the create path, `PATCH`) and,
in task order, touches task 10's move path in this plan's own files (the triage exit). Hub anchors
were read in the `feat/hub` worktree at `9f2fa31`: the contract
`docs/superpowers/contracts/2026-09-23-hub-contract.md` (cited `hub-contract:N`) and the Hub plan's
task 14 part `docs/superpowers/plans/2026-09-23-phase-h-hub-parts/task-14.md` (cited `H14:N`), which
fixes the write helper every handler here uses. `server/` and `store/` are **planned, not on
`feat/hub`** at `9f2fa31` (only `hubapi/` and `hubclient/` exist there), so every path under them
is **UNVERIFIED until Hub tasks 08-14 land** (rule H1).

This task gives a team its working vocabulary. Three small aggregates, each one pair of files:
**relations** between two issues (`blocks`, `duplicates`, `relates`; L-24), **labels** that a team
owns (`lb-`; L-17), and **the team's work policy**, the three switches 05 §3.3 and L-9/L-48 name.
It also puts the server half of **triage** in place: where an issue lands when nobody names a
state, the one policy that can hold an issue in `triage`, and *snooze*, which defers without hiding.

Four decisions the scope left open, made here so no reader re-derives them.

1. **A relation is stored once, in a normal form, and read from both ends.** `relates` is
   symmetric, so its row always has the smaller id first. `blocks` and `duplicates` are directed:
   the row is `(issue_id = the writer's subject, other_issue_id = the target)`. On read, the other
   end sees the inverse word: `blocked_by` for `blocks`, `duplicated_by` for `duplicates`. The two
   inverse words are **read-only** members of `RelationKind`; a `RelationRequest` naming one is 400.
   Task 03 declares the three write kinds only, so this task adds the two read kinds additively
   (listed as a skeleton gap in the return).
2. **The cycle rule is the two-cycle, and only that.** `B blocks A` after `A blocks B` is 422
   `relation_cycle`, and so is `B duplicates A` after `A duplicates B`. A longer cycle (A→B→C→A) is
   **allowed** in W1: finding it needs a graph walk under the org lock, and 05 §3.7 asks for none.
   The part says so rather than implying a full check.
3. **Relations are same-team only.** A project's access is exactly its team's membership
   (hub-contract:333), so two issues of one team are both readable by anyone who can read either.
   A cross-team relation would show a reader an issue id of a project they cannot read. The other
   issue is looked up **inside the caller's readable set**, so another team's issue answers 404
   `issue_not_found`, never 403 (hub-contract:273-276's reason, verified at `9f2fa31`).
4. **A duplicate mark is a relation, not a state move.** `duplicates` writes the row and one event
   and leaves both states alone. Triage's `2` key (task 42) does the relation **and** a `hand` move
   to `duplicate`, as two writes, each with its own event and its own Undo.

**The one thing this task must not get wrong** is the triage exit under `priorityToLeaveTriage`.
The policy exists so an issue does not leave triage unprioritised; it must never stop a person
**declining** one. So the refusal applies to a move out of `triage` to any state except
`cancelled` and `duplicate`, only when the issue's priority is `none`, and it is 422
`policy_refused` with `details.policy = "priorityToLeaveTriage"`, appending **no** event. Task 16's
relay passes that problem through untouched (`task-16.md:25-32`), which is how the desktop tells a
person *why* EP-142 stayed in triage.

**Files:**

- `control-plane/internal/hub/hubapi/work.go` ⚑ (task 03's file, modified additively): the two read
  kinds `RelationBlockedBy`, `RelationDuplicatedBy`; `IssueHead.SnoozedUntil`,
  `PatchIssueRequest.SnoozedUntil`; `TriageExitRefused`, the pure policy check both the Hub and the
  fake call; the Work codes this task needs, if task 03 has not declared them
- `control-plane/internal/hub/hubapi/work_social_test.go` (new): the pure rules, no database
- `control-plane/internal/hub/migrate/sql/NNNN_work_w1.sql` ⚑ (task 06's file, a section appended
  before it is applied anywhere shared; the next free number instead if 06 has merged, rule H1)
- `control-plane/internal/hub/store/work_relations.go`, `server/work_relations.go` (new)
- `control-plane/internal/hub/store/work_labels.go`, `server/work_labels.go` (new)
- `control-plane/internal/hub/store/work_policies.go`, `server/work_policies.go` (new)
- `control-plane/internal/hub/store/work_issues.go` ⚑ (task 08's file): the create reads the
  policy; the patch validates `labels` and `snoozedUntil`
- `control-plane/internal/hub/store/work_states.go` ⚑ (task 10's file): the triage exit check
- `control-plane/internal/hub/server/work_social_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/hubclient/fake_work*.go` ⚑ (task 05's file): only if a rule here
  differs from what 05 wrote; the conformance rows (task 05) are the arbiter

**Interfaces:**

- Produces:
  - the routes, in 07's list and 07's matrix rows: `POST /v1/issues/{issueId}/relations`,
    `DELETE /v1/issues/{issueId}/relations/{kind}/{otherId}`, `GET` and `POST
    /v1/teams/{teamId}/labels`, `POST /v1/labels/{labelId}/archive`, `GET` and `PUT
    /v1/teams/{teamId}/work-policy`
  - `hubapi.RelationBlockedBy`, `hubapi.RelationDuplicatedBy` (read-only kinds),
    `hubapi.WritableRelationKind(k) bool`, `hubapi.InverseRelation(k) RelationKind`
  - `hubapi.DefaultTeamPolicy(teamID) TeamPolicy`, `hubapi.TriageExitRefused(from, to, priority,
    policy) bool`, the one rule the Hub and the fake both call
  - `IssueHead.snoozedUntil` and `PatchIssueRequest.snoozedUntil` (additive)
  - the events `issue.related`, `issue.unrelated`, `label.created`, `label.archived`,
    `team.work_policy_set`, as drafts that task 13 makes constants (SW.5)
- Consumes:
  - task 03: `RelationKind`, `Relation`, `RelationRequest`, `Label`, `CreateLabelRequest`,
    `TeamPolicy`, `PutTeamPolicyRequest`, `IssueState`, `Priority`, `PatchIssueRequest`
  - task 06: `labels`, `issue_labels`, `issue_relations`, `team_work_policies`, the named constraint
    `labels_team_live_name`, `issue_relations_kind_known`
  - task 07: the matrix rows for these seven routes; `hub.work_cause` (the triage exit is a `hand`
    move and sets nothing new)
  - task 08: `store.LockIssue`, the readable-issue lookup, the create and patch paths
  - task 10: the `hand` move path, where the triage exit check runs before the `UPDATE`
  - Hub task 14: `server.Write`, `server.Read`, `(*Tx).Emit`, `server.Refuse`, `server.Forbidden`,
    `server.Decode` (`H14:27-138`); Hub task 15: the caller's team role
- Consumed by: task 13 (the kinds), task 15 (the label match and the policy it must override),
  task 42 (the triage keys: accept, duplicate, decline, snooze), task 44 (labels in the display
  menu), task 63 (the Desk's *inform* for a relation on someone else's issue)

The names of task 03's identifiers (`hubapi.StateTriage`, `hubapi.PriorityNone`,
`hubapi.RelationBlocks` and the rest) are written here as task 03's part spells them; where it
spells one differently, task 03 wins and this part is a find-and-replace.

#### Wire shapes

```jsonc
// POST /v1/issues/is-0a1b…/relations            member; 201 new, 200 existing (no event)
{ "kind": "blocks", "issueId": "is-9f8e…" }
// → 201 | 200: the subject's head, relations read from its own end
{ "id": "is-0a1b…", "relations": [ { "kind": "blocks", "issueId": "is-9f8e…" } ], … IssueHead … }

// GET /v1/issues/is-9f8e…   the other end reads the inverse word
{ "id": "is-9f8e…", "relations": [ { "kind": "blocked_by", "issueId": "is-0a1b…" } ], … }

// DELETE /v1/issues/is-9f8e…/relations/blocked_by/is-0a1b…  → 200 head; 404 relation_not_found
// Either end may delete, with either word: the pair is normalised before the lookup.

// POST /v1/teams/tm-…/labels                      admin; 201
{ "name": "Bug", "color": "danger" }
// → { "id": "lb-…", "teamId": "tm-…", "name": "Bug", "color": "danger", "archivedAt": null }
// GET /v1/teams/tm-…/labels?archived=true         viewer; Page[Label], name order, live first
// POST /v1/labels/lb-…/archive                    admin; 200 Label; a second call 200, no event

// GET /v1/teams/tm-…/work-policy                  viewer; the defaults when no row exists
{ "teamId": "tm-…", "defaultState": "backlog", "priorityToLeaveTriage": false,
  "stopAtLandedWithoutServes": false, "updatedAt": null, "updatedBy": null }
// PUT /v1/teams/tm-…/work-policy                  admin; every field present (a PUT, not a PATCH)
{ "defaultState": "todo", "priorityToLeaveTriage": true, "stopAtLandedWithoutServes": false }

// POST /v1/issues/is-…/state  {"to":"todo","cause":"hand"} on an un-prioritised triage issue
// → 422
{ "error": "policy_refused", "message": "This team needs a priority before an issue leaves triage.",
  "details": { "policy": "priorityToLeaveTriage", "issueId": "is-…" } }
```

`Label.archivedAt` is one field this task needs that task 03's `Label{id, teamId, name, color}`
does not list; it is added additively (a skeleton gap: an archived label must read as archived).
`TeamPolicy.updatedAt` and `updatedBy` are `null` for a team that never set one, so a client can
tell "the defaults" from "an admin chose these values".

