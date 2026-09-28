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
{ "name": "Bug", "color": "fail" }
// → { "id": "lb-…", "teamId": "tm-…", "name": "Bug", "color": "fail", "archivedAt": null }
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

#### `hubapi/work.go`, the additions (complete)

```go
// ── Task 11: relations, labels, the team policy, triage ─────────────────────
// Appended to work.go (task 03's file). Additive only: no existing field moves.

// The two read-only relation words. A relation is stored once; the other end
// reads the inverse. A RelationRequest naming one of these is 400 bad_request.
const (
	RelationBlockedBy    RelationKind = "blocked_by"
	RelationDuplicatedBy RelationKind = "duplicated_by"
)

// WritableRelationKind is true for the three kinds a person may write.
func WritableRelationKind(k RelationKind) bool {
	switch k {
	case RelationBlocks, RelationDuplicates, RelationRelates:
		return true
	}
	return false
}

// InverseRelation is the word the other end reads. relates is its own inverse.
func InverseRelation(k RelationKind) RelationKind {
	switch k {
	case RelationBlocks:
		return RelationBlockedBy
	case RelationBlockedBy:
		return RelationBlocks
	case RelationDuplicates:
		return RelationDuplicatedBy
	case RelationDuplicatedBy:
		return RelationDuplicates
	}
	return k
}

// Additive fields (declared on task 03's structs; shown here as the diff):
//   IssueHead:         SnoozedUntil *string `json:"snoozedUntil"`   // RFC 3339; null when not snoozed
//   PatchIssueRequest: SnoozedUntil *string `json:"snoozedUntil,omitempty"` // "" clears
//   Label:             ArchivedAt   *string `json:"archivedAt"`     // null while live

// Policy names, the values of details.policy on 422 policy_refused.
const PolicyPriorityToLeaveTriage = "priorityToLeaveTriage"

// DefaultTeamPolicy is what a team with no row reads (L-9's default status).
func DefaultTeamPolicy(teamID string) TeamPolicy {
	return TeamPolicy{TeamID: teamID, DefaultState: StateBacklog}
}

// TriageExitRefused is L-48: under priorityToLeaveTriage, an issue with no
// priority may leave triage only to be declined (cancelled) or marked a
// duplicate. It is pure so the Hub (task 10's move path) and the fake (task
// 05) cannot disagree.
func TriageExitRefused(from, to IssueState, p Priority, pol TeamPolicy) bool {
	if !pol.PriorityToLeaveTriage || from != StateTriage || to == StateTriage {
		return false
	}
	if to == StateCancelled || to == StateDuplicate {
		return false
	}
	return p == PriorityNone
}

// Work codes task 11 needs. If task 03 declared them, these lines are dropped.
const (
	CodeIssueNotFound    = "issue_not_found"    // 404, details.issueId
	CodeLabelNotFound    = "label_not_found"    // 404, details.labelId
	CodeRelationNotFound = "relation_not_found" // 404, details.kind, details.otherId
	CodeLabelExists      = "label_exists"       // 409, details.labelId (the live label)
	CodeRelationCycle    = "relation_cycle"     // 422, details.kind, details.otherId
	CodeLabelUnavailable = "label_unavailable"  // 422, details.field = "labels[n]"
	CodePolicyRefused    = "policy_refused"     // 422, details.policy, details.issueId
)
```

`TeamPolicy` keeps task 03's field list; `UpdatedAt` and `UpdatedBy` must be pointer types
(`*string`, `*User`) so the defaults encode as `null`. If task 03 declared them as values, this
is the one non-additive edit, and it is announced at WS0 (skeleton, *Start lines*).

#### The migration section (appended to `NNNN_work_w1.sql`, complete)

Task 06 already carries most of what this task needs, read in its part at this writing:
`labels_team_live_name`, a unique index on `(team_id, lower(name)) WHERE archived_at IS NULL`
(`task-06.md:247`), so a name is unique among a team's **live** labels case-insensitively and an
archived `Bug` frees the name; `issue_relations` with `PRIMARY KEY (issue_id, kind,
other_issue_id)`, `issue_relations_kind_known` and `issue_relations_not_self` (`:259-273`); and
`team_work_policies` (`:276-289`). One column is missing: the snooze.

```sql
-- ── task 11: triage snooze (05 §4.2; AX §7.4: deferred items age visibly) ────────
-- A snoozed issue stays in triage and sorts after the rest; it is never hidden.
ALTER TABLE issues ADD COLUMN snoozed_until timestamptz;
GRANT UPDATE (snoozed_until) ON issues TO hub_app;
CREATE INDEX issues_triage_order ON issues (project_id, (snoozed_until IS NOT NULL), created_at DESC)
  WHERE state = 'triage';
```

Task 06's `TestWorkGrantsAreExactlyTheList` gains `issues.snoozed_until` in its expected list in
this task (the test is in task 06's file; the list is data, and adding a granted column without
adding it there is the red that test exists for). One mismatch is recorded and not fixed here:
task 06's `team_work_policies.default_state` has `DEFAULT 'triage'`, while this task's absent-row
default is `backlog` (scope 11, L-9). The store never relies on the column default, because a `PUT`
writes every column, so the mismatch is harmless; it is listed as a gap so task 06's owner can
align it.

#### The store (`store/work_relations.go`, `work_labels.go`, `work_policies.go`, signatures complete)

```go
package store

// Every function runs inside the caller's write or read transaction (Hub task 14's
// server.Tx wraps a pgx.Tx), after set_config('hub.org_id') and, for writes, after
// the org's audit head lock, so a check-then-insert here cannot race another write
// in the same org (hub-contract §10.6, "one lock order").

// RelationRow is one stored row, in the normal form: relates has the smaller id
// first; blocks and duplicates point from the writer's subject to the target.
type RelationRow struct {
	IssueID, OtherIssueID string
	Kind                  string // blocks | duplicates | relates
	CreatedAt             time.Time
	CreatedBy             string
}

// NormaliseRelation turns (subject, kind, other), where kind may be a read word,
// into the stored row's (issue, kind, other). blocked_by B from A is (B blocks A).
func NormaliseRelation(subject string, kind hubapi.RelationKind, other string) (issue, storedKind, otherID string)

// InsertRelation inserts the row; inserted is false when the row already existed
// (ON CONFLICT DO NOTHING), in which case the caller emits no event.
func InsertRelation(ctx context.Context, tx pgx.Tx, org string, r RelationRow) (inserted bool, err error)

// ReverseExists is the two-cycle check: a directed row (other, kind, issue).
func ReverseExists(ctx context.Context, tx pgx.Tx, issue, kind, other string) (bool, error)

// DeleteRelation deletes the normalised row; deleted is false when none existed.
func DeleteRelation(ctx context.Context, tx pgx.Tx, issue, kind, other string) (deleted bool, err error)

// RelationsOf reads both ends: rows where issue_id = id as stored, and rows where
// other_issue_id = id with the inverse word. Ordered by kind, then created_at.
func RelationsOf(ctx context.Context, q Querier, id string) ([]hubapi.Relation, error)

func InsertLabel(ctx context.Context, tx pgx.Tx, org string, l hubapi.Label, by string, at time.Time) error // 23505 on labels_team_live_name → ErrLabelExists
func LiveLabelByName(ctx context.Context, q Querier, teamID, name string) (hubapi.Label, bool, error)
func LabelByID(ctx context.Context, q Querier, id string) (hubapi.Label, bool, error)
func ArchiveLabel(ctx context.Context, tx pgx.Tx, id string, at time.Time) (changed bool, err error)
func ListLabels(ctx context.Context, q Querier, teamID string, withArchived bool, pr hubapi.PageRequest) (hubapi.Page[hubapi.Label], error)

// TeamPolicy answers the stored row or hubapi.DefaultTeamPolicy(teamID).
func TeamPolicy(ctx context.Context, q Querier, teamID string) (hubapi.TeamPolicy, error)
func PutTeamPolicy(ctx context.Context, tx pgx.Tx, org string, p hubapi.TeamPolicy) (changed []string, err error) // the field names that differ

var ErrLabelExists = errors.New("store: a live label of that name exists in the team")
```

`Querier` is whatever Hub task 08's `store.go` names the common interface of `pgx.Tx` and
`*pgxpool.Pool`; the name is **UNVERIFIED** until that file lands, and a different name is a
find-and-replace.

#### The routes (`server/work_relations.go`, `work_labels.go`, `work_policies.go`)

Registered beside Hub task 08's mux entries, with Go 1.22 method patterns, so `server.Patterns()`
lists them and task 07's matrix rows go live:

```go
func (s *Server) registerWorkSocial(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/issues/{issueId}/relations", s.postRelation)
	mux.HandleFunc("DELETE /v1/issues/{issueId}/relations/{kind}/{otherId}", s.deleteRelation)
	mux.HandleFunc("GET /v1/teams/{teamId}/labels", s.listLabels)
	mux.HandleFunc("POST /v1/teams/{teamId}/labels", s.postLabel)
	mux.HandleFunc("POST /v1/labels/{labelId}/archive", s.archiveLabel)
	mux.HandleFunc("GET /v1/teams/{teamId}/work-policy", s.getWorkPolicy)
	mux.HandleFunc("PUT /v1/teams/{teamId}/work-policy", s.putWorkPolicy)
}
```

**`postRelation`**, inside `s.WriteReplayable` (see *Replay storage* below), in this order (the order is the rule; each step's refusal
rolls the transaction back and appends nothing):

1. `server.Decode[hubapi.RelationRequest]`; `!WritableRelationKind(kind)` → 400 `bad_request`,
   `field: "kind"`; a malformed `issueId` → 400, `field: "issueId"`.
2. Load the subject inside the caller's readable set (task 08's lookup) → 404 `issue_not_found`;
   the caller's role on its team below `member` → `server.Forbidden("member")`.
3. Load the other issue **inside the same readable set** → 404 `issue_not_found` naming
   `details.issueId` = the other id; the other issue's `team_id` ≠ the subject's → the same 404
   (decision 3: a relation never tells a reader another team's issue exists).
4. `subject == other` → 400 `bad_request`, `field: "issueId"`, "An issue cannot relate to
   itself." The handler refuses before the `INSERT`, so task 06's `issue_relations_not_self`
   `CHECK` is the second layer and never the answer a person sees.
5. `NormaliseRelation`; for `blocks` and `duplicates`, `ReverseExists` → 422 `relation_cycle`.
6. `InsertRelation`; `inserted == false` → 200 with the head and **no** `Emit`.
7. `Emit(Event{Kind: "issue.related", TeamID, ProjectID, Subject: subjectID, Data: {"kind",
   "otherIssueId", "otherAssigneeId"}})`, where `otherAssigneeId` is the other issue's assignee
   when that assignee is set **and is not the writer**, else `""` (05 §3.7 rule 3: the owner is
   informed, never their agents). 201 with the subject's head.

**Replay storage.** `postRelation` answers an `IssueHead`, which carries `external.url` (an
imported issue's link, which task 14 erases). So it runs under `server.WriteReplayable` with a
`rebuildIssueHead` that reads the head as it is now, storing the reply with `external.url` `""`,
the rule task 08 applies to its create (`task-08.md:45-51`). The label and policy replies hold no
issue text and use `server.Write`; `DELETE` and `PUT` are never stored (Hub task 14 claims keys
for `POST` only, `H14:103`).

`deleteRelation` accepts any of the five words in `{kind}` (the read words included, because the
desktop deletes what it shows), normalises, and deletes; nothing deleted → 404
`relation_not_found`; else one `issue.unrelated` event with the same `data` and 200.

**`postLabel`** (admin): `name` trimmed, 1-40 characters, no control characters → else 400
`field: "name"`; `color` must be one of `LabelColors` → else 400 `field: "color"`; `ErrLabelExists`
→ 409 `label_exists`, `details.labelId` the live label's id. One `label.created` `{"name"}` event
with `TeamID` set and `ProjectID` empty (a label is team metadata; hub-contract §8 already carries
a team's and a project's `name` in `data`, `:1139` and `:1145`, and forbids only brief content,
blob bytes, evidence `because` text and tokens, `:1133-1134`; a label name follows that precedent).
**`archiveLabel`** (admin of the label's team; the label outside the caller's teams → 404
`label_not_found`): sets `archived_at` once; a second call is 200 and no event. Issues keep an
archived label they already carry (history is not rewritten); a new `PATCH` cannot add one.

**`putWorkPolicy`** (admin): all three fields required (a missing one is 400 naming it);
`defaultState` must satisfy `hubapi.ValidInitialState` → else 400 `field: "defaultState"`;
`PutTeamPolicy` returns the changed field names; none changed → 200 and no event; else one
`team.work_policy_set` `{"fields": "defaultState,priorityToLeaveTriage"}` (a comma-joined
string: `data` carries no arrays, hub-contract §8's last paragraph).

**The label palette.** `LabelColors` is the list of `docs/DESIGN-SYSTEM.md` token names a label may
use, declared once in `hubapi/work.go` as `[]string`, and a Go test reads the token names out of
`desktop/src/styles/tokens.css` and fails when one in the list is not a token there. The exact
list is task 26's to widen (the tokens task); until it lands this task ships six names that are
tokens in `desktop/src/styles/tokens.css` today, read at this writing: `ink-2` (`:46`), `accent`
(`:63`), `run`, `need`, `fail` and `drift` (`:74-77`). `thread` (`:57`) is deliberately absent: it
is the provenance signature and a label must never look like the thread.

**The create and patch paths (task 08's file, edited in task order).** `CreateIssueRequest` with an
empty `state` takes `TeamPolicy(team).DefaultState`; a non-empty one must satisfy
`ValidInitialState` (task 03). An import (task 15) and intake (W5) **always** pass `triage`, so the
policy never applies to them. `PatchIssueRequest.labels`: each id must be a live label of the
issue's own team → else 422 `label_unavailable`, `details.field = "labels[n]"`; the list replaces
the issue's labels (`DELETE` then `INSERT` on `issue_labels`, one `issue.updated` event with
`fields: "labels"`). `PatchIssueRequest.snoozedUntil`: an RFC 3339 time in the future, allowed only
while the issue is in `triage` (else 422 `bad_request`, `field: "snoozedUntil"`), and `""` clears
it; a move out of `triage` clears it in the same `UPDATE` (task 10's path).

**The move path (task 10's file).** Before the `UPDATE` of a `hand` move, `TriageExitRefused(from,
to, priority, TeamPolicy(team))` → 422 `policy_refused`, `details.policy =
"priorityToLeaveTriage"`, no event. The check applies to `hand` moves only. A fact move out of
`triage` (Start on an issue nobody accepted) is task 24's to prevent or allow; whether task 24
accepts the issue first is **UNVERIFIED** here, and the conformance row in task 05 is where the two
must agree.

#### Tests to write first, part 1 — `hubapi/work_social_test.go` (pure, no database)

```go
package hubapi

import "testing"

func TestTriageExitRefusedOnlyWhereThePolicySays(t *testing.T) {
	on := TeamPolicy{TeamID: "tm-0000000000000001", DefaultState: StateBacklog, PriorityToLeaveTriage: true}
	off := on
	off.PriorityToLeaveTriage = false
	cases := []struct {
		name     string
		from, to IssueState
		p        Priority
		pol      TeamPolicy
		refused  bool
	}{
		{"accept unprioritised", StateTriage, StateTodo, PriorityNone, on, true},
		{"accept to backlog unprioritised", StateTriage, StateBacklog, PriorityNone, on, true},
		{"accept prioritised", StateTriage, StateTodo, PriorityHigh, on, false},
		{"decline is never blocked", StateTriage, StateCancelled, PriorityNone, on, false},
		{"duplicate is never blocked", StateTriage, StateDuplicate, PriorityNone, on, false},
		{"policy off", StateTriage, StateTodo, PriorityNone, off, false},
		{"not leaving triage", StateBacklog, StateTodo, PriorityNone, on, false},
		{"triage to triage", StateTriage, StateTriage, PriorityNone, on, false},
	}
	for _, c := range cases {
		if got := TriageExitRefused(c.from, c.to, c.p, c.pol); got != c.refused {
			t.Errorf("%s: TriageExitRefused = %v, want %v", c.name, got, c.refused)
		}
	}
}

func TestInverseRelationIsAnInvolution(t *testing.T) {
	for _, k := range []RelationKind{RelationBlocks, RelationBlockedBy, RelationDuplicates,
		RelationDuplicatedBy, RelationRelates} {
		if InverseRelation(InverseRelation(k)) != k {
			t.Errorf("InverseRelation twice on %q is %q", k, InverseRelation(InverseRelation(k)))
		}
	}
	if InverseRelation(RelationRelates) != RelationRelates {
		t.Errorf("relates must read the same from both ends")
	}
	if InverseRelation(RelationBlocks) != RelationBlockedBy {
		t.Errorf("blocks must read blocked_by from the other end")
	}
}

func TestOnlyThreeRelationKindsAreWritable(t *testing.T) {
	for k, want := range map[RelationKind]bool{RelationBlocks: true, RelationDuplicates: true,
		RelationRelates: true, RelationBlockedBy: false, RelationDuplicatedBy: false, "parent": false} {
		if WritableRelationKind(k) != want {
			t.Errorf("WritableRelationKind(%q) = %v, want %v", k, !want, want)
		}
	}
}

func TestTheDefaultPolicyIsBacklogWithEverySwitchOff(t *testing.T) {
	p := DefaultTeamPolicy("tm-0000000000000001")
	if p.DefaultState != StateBacklog || p.PriorityToLeaveTriage || p.StopAtLandedWithoutServes {
		t.Fatalf("DefaultTeamPolicy = %+v", p)
	}
	if p.UpdatedAt != nil || p.UpdatedBy != nil {
		t.Fatalf("the defaults must read updatedAt and updatedBy null, got %+v", p)
	}
}
```

| Test | Mutant it kills |
|---|---|
| `TestTriageExitRefusedOnlyWhereThePolicySays` | a policy that blocks declining (`cancelled`) or marking a duplicate; a policy that fires when switched off; one that blocks moves that are not out of triage |
| `TestInverseRelationIsAnInvolution` | a one-sided read (`blocks` shown as `blocks` from both ends); an inverse table with a typo that maps `duplicated_by` back to `blocks` |
| `TestOnlyThreeRelationKindsAreWritable` | a writer storing `blocked_by`, which would make one pair two rows and the two-cycle check blind |
| `TestTheDefaultPolicyIsBacklogWithEverySwitchOff` | a default of `triage` (every new issue would land in a queue nobody asked for) or a switch on by default |

#### Tests to write first, part 2 — `server/work_social_integration_test.go` (`//go:build integration`)

The cast is the Hub's (Hub task 15's part, `task-15.md:57-60`): tokens minted with
`devissuer.MintAccessToken` against an in-process issuer, a Hub `httptest.Server` over
`hubtest.Migrated`, and `as(name)` returning a client with that user's token. Ada is admin of team
Portal (project `EP`), Grace a member, Vera a viewer, Linus an org member outside Portal, and Xavier
in another enrolled org. Team Billing (project `PAY`) has Linus as admin. Issues are made through
task 08's `POST /v1/projects/{projectId}/issues`, never by SQL, so every row went through the route.
Each test counts events by reading `hub.audit_events` as the owner (`hubtest.OwnerConn`) for the
org, before and after.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRelationToAnotherTeamsIssueIs404` | Grace relates EP-1 to a PAY issue → 404 `issue_not_found`, `details.issueId` the PAY id; Linus (who can read PAY but not EP) relating PAY-1 to EP-1 → 404; no event either way | a relation that leaks another team's issue id |
| `TestTheSameRelationTwiceIsOneRowAndOneEvent` | `EP-1 blocks EP-2` → 201; the same → 200, the same head; `issue_relations` has one row; one `issue.related` event | a double event on a repeat |
| `TestRelatesIsStoredOnceFromEitherEnd` | `EP-1 relates EP-2`, then `EP-2 relates EP-1` → 200, one row, one event | a symmetric kind stored twice |
| `TestTheTwoCycleIsRefused` | `EP-1 blocks EP-2`, then `EP-2 blocks EP-1` → 422 `relation_cycle`; the same for `duplicates` | a two-cycle, which makes both issues wait on each other |
| `TestTheOtherEndReadsTheInverseWord` | after `EP-1 blocks EP-2`, `GET EP-2` has `{kind: blocked_by, issueId: EP-1}`; after `EP-3 duplicates EP-1`, `GET EP-1` has `duplicated_by EP-3` | a one-sided read |
| `TestEitherEndMayDeleteWithEitherWord` | `DELETE /v1/issues/EP-2/relations/blocked_by/EP-1` → 200, the row gone, one `issue.unrelated`; the same again → 404 `relation_not_found` | a delete that only the writer's end can find |
| `TestARelationToAnOwnedIssueInformsItsOwner` | EP-2 assigned to Ada; Grace relates EP-1 to EP-2 → the event's `otherAssigneeId` is Ada's id; Ada relating the pair back → `otherAssigneeId: ""` | an owner never told (05 §3.7 rule 3), or told about their own act |
| `TestARelationToItselfIs400` | `EP-1 relates EP-1` → 400 `field: issueId`, no 500 from the `CHECK` | a database error surfacing as 500 |
| `TestDuplicatesMovesNoState` | `EP-3 duplicates EP-1` → both states unchanged | a relation that moves a state behind the person's back |
| `TestLabelNamesAreUniqueCaseInsensitively` | Ada creates `Bug` → 201; `bug` → 409 `label_exists` naming the first; after archiving `Bug`, `bug` → 201 | case-sensitive uniqueness; an archived name held forever |
| `TestOnlyAdminsManageLabels` | Grace `POST …/labels` → 403 `required: admin`; Vera `GET` → 200; Linus `GET` → 404 `team_not_found` | a member creating labels (07's row) |
| `TestALabelColourIsATokenName` | `"#ff0000"` → 400 `field: color`; `"thread"` → 400; `"fail"` → 201 | a hex colour, or the thread's colour on a label |
| `TestAnIssueCarriesOnlyItsTeamsLiveLabels` | `PATCH EP-1 {labels: [billingLabel]}` → 422 `label_unavailable`, `details.field: labels[0]`; an archived Portal label → 422; a live one → 200 and one `issue.updated` `{fields: labels}` | another team's or an archived label on an issue |
| `TestUnderThePolicyTriageNeedsAPriority` | Ada `PUT` the policy `priorityToLeaveTriage: true`; a triage issue with priority `none`, `hand` → `todo` → 422 `policy_refused`, `details.policy: priorityToLeaveTriage`, no event; → `cancelled` with a reason → 200 | a policy that blocks declining; a refusal that still appends an event |
| `TestCreateWithNoStateTakesThePolicyDefault` | the policy `defaultState: todo`; create with no `state` → `todo`; with no policy row → `backlog`; an explicit `state: triage` → `triage` whatever the default. The scope's "an import lands in `triage`" half is task 15's `TestImportLandsInTriageWhateverTheDefault`, because the import route does not exist until then | a create that ignores the policy, or a policy that overrides an explicit state |
| `TestAMemberCannotPutThePolicy` | Grace `PUT …/work-policy` → 403 `required: admin`; Vera `GET` → 200 with the defaults and `updatedAt: null` | a member changing team settings |
| `TestAnUnchangedPolicyAppendsNoEvent` | the same `PUT` twice → one `team.work_policy_set`, `fields` naming only what changed | an event per `PUT` whatever changed |
| `TestSnoozeKeepsTheIssueInTriageAndLast` | three triage issues; snooze the newest; `GET …/issues?state=triage` lists it last, not absent; `snoozedUntil` in the past → 400; on a `todo` issue → 422 | a snooze that hides the issue, which is what AX §7.4 forbids |
| `TestLeavingTriageClearsTheSnooze` | a snoozed triage issue accepted → `snoozedUntil: null` | a stale snooze on an issue that already left |
| `TestNoSocialEventCarriesText` | a title canary `CANARY-11-TITLE` on the related issues; no event's `data` of this file's kinds contains it | issue text in the feed (hub-contract:1133-1134) |

The matrix rows for these seven routes are task 07's (`work_matrix_rows.go`); they go live here
because `server.Patterns()` now lists the patterns. This task adds no row and edits none.

#### Steps

- [ ] **Step 1:** Confirm the sync base (rule H1): `feat/hub` sync-merged into `feat/work-layer` by
  the orchestrator, tasks 08 and 10 committed. Re-read hub-contract §4.2, §5.2, §5.3, §8 and Hub
  task 14's part at the new base; if a helper this part names has moved, follow the move.
- [ ] **Step 2:** Write `hubapi/work_social_test.go`; run it; it fails to compile (the names do not
  exist). Append the `hubapi/work.go` block above; it passes.
- [ ] **Step 3:** Append the migration section to `NNNN_work_w1.sql` (or a new next-number file if
  task 06 has merged into `feat/hub`); add `issues.snoozed_until` to task 06's grant-list test.
- [ ] **Step 4:** Write `server/work_social_integration_test.go`; run it against the compose stack;
  every row fails (404 `route_not_found`).
- [ ] **Step 5:** Write the three store files, then the three server files and
  `registerWorkSocial`; run until green.
- [ ] **Step 6:** Edit task 08's create and patch paths and task 10's move path as described; rerun
  tasks 08 and 10's integration tests, which must stay green.
- [ ] **Step 7:** Run the fake's conformance rows (task 05) against the fake; a row that now
  disagrees with the Hub is fixed in the fake, never by loosening the row.
- [ ] **Step 8:** `git diff -- go.work control-plane/go.mod` shows nothing; commit by pathspec.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/internal/hub/hubapi/ -race -count=1 -run 'TestTriageExit|TestInverseRelation|TestOnlyThree|TestTheDefaultPolicy' -v 2>&1 | grep -c "^--- PASS"   # 4
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(RelationToAnotherTeams|TheSameRelationTwice|RelatesIsStoredOnce|TheTwoCycle|TheOtherEndReads|EitherEndMayDelete|ARelationToAnOwned|ARelationToItself|DuplicatesMovesNoState|LabelNamesAreUnique|OnlyAdminsManageLabels|ALabelColour|AnIssueCarriesOnlyItsTeams|UnderThePolicyTriage|CreateWithNoStateTakes|AMemberCannotPutThePolicy|AnUnchangedPolicy|SnoozeKeeps|LeavingTriageClears|NoSocialEventCarries)' \
  2>&1 | grep -c "^--- PASS"                                                                    # 20
go test -tags integration ./control-plane/internal/hub/... -race -count=1 -run 'WorkIssues|WorkStates|Matrix'   # ok
go test ./control-plane/... -race -count=1                                                                      # ok
```

The counts are what the tables above name (4 and 20); a different count is a test added or lost,
and either is a finding.

#### What this task does not claim

- Label groups and workspace (org) labels (L-18, L-19, W2), custom states (W2, task 75).
- A cycle check beyond the two-cycle (decision 2).
- The triage UI and its number keys (task 42); the `2` key's relation-plus-move pair is task 42's.
- That resolving a blocker downgrades it to *related* (L-25); nothing here does, and 05 prices it
  in W2.
- The Desk's *inform* for `otherAssigneeId` (task 63) or the W3 Orchestrator proposal (task 99
  onward): this task only carries the id on the event.
- That the event kind names are final: task 13 makes them constants (SW.5) and a rename there is a
  find-and-replace here.
- Any measured latency: every size in this part is an estimate.

**Parity rows served:** **L-2** (a team owns its labels and work settings; with task 08),
**L-9** (the default status; closed with task 45's create surface), **L-10** (the duplicate mark;
with task 10's state), **L-17** (team labels; workspace labels stay W2), **L-24** (blocks, blocked
by, related, duplicate; the Desk cost-of-waiting half closes with task 63), **L-46** and **L-47**
(the server half of the triage inbox and its actions; closed with task 42), **L-48** (priority
before leaving triage).

---

**Appended by fix batch b01 (2026-09-28), review finding X-C1: `implements` is deliberately not a
`RelationKind`.** The finding offered two shapes for the issue-to-design link — a new kind in this
task's set, or a second `serves`-like field on the revision. It is the **field**, and the field
already exists: `IssueRevisionContent.Implements []string` at `task-02.md:264`, in the canonical
bytes and all three golden vectors. This task's set stays exactly three write kinds (`blocks`,
`duplicates`, `relates`) and two read-only inverses, and **no `implements` kind is added here**.

Three reasons, so nobody re-opens it. (1) A relation is **issue to issue**; `implements` points at a
`design:cv-…@<rev>` key, which is not an issue and has no inverse row to write. (2) A relation is
**mutable and undirected in time**; `implements` is part of the revision's **immutable, digested**
content, which is what puts the issue on the provenance thread at all — a relation row would sit
beside the digest and could disagree with it. (3) Decision 3 above scopes relations to one team
through the issue's project; a canvas is reached through its own project's access, so the same rule
cannot be reused unchanged.

The write path, the reverse read (*which issues implement this canvas?*) and the design-approval
close of 04 §7.7 are **task 81b**; the surfaces are **task 96b**.

