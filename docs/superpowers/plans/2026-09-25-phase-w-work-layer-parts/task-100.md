### Task 100: the W3 migration, routes, matrix rows and HTTP methods

**Slice:** W3. **Size:** 3 focused hours (an estimate, not a measurement). **Stream:** WH.
**dependsOn:** 99; TM-05. **Seam:** **SW.19** on the real Hub, as tasks 06-16 did SW.1 and SW.2
for W1, under rule H1 (additive only; the migration takes the next free number at sync time).

Task 99 fixed the W3 wire, the fake and the conformance rows. This task makes the real Hub pass
those rows: one migration, one store file, one route file, the authorisation rows, the HTTP
client methods, and the control plane's relay rows. Every rule is held twice (hub-contract §10.7,
`:2047`): once in Go, once in the database, so a handler bug cannot store what the rule forbids.

**Anchors, re-read on 2026-09-25.** Hub contract on `feat/hub` at `63e7232`: §4.3 the relay rule
(`:277`), §5.3 the matrix (`:335-352`), §7.10 evidence (`:1060-1088`: *"`recordedBy` is the
principal"*, append-only, event `evidence.recorded`), §7.11 the change feed (`:1089`), §8 events
(`:1131`), §10.7 (`:2047`). Phase T on `feat/team` at `64ab2c4`: TM-05's scope (plan `:466-477`:
*"the principal becomes the delegation's owner with `via` set"*); its part (`task-05.md:50-66`:
`Delegable(name, handler)`, `Principal.Via = "fleet"`, `DelegationID`, `OrgAdmin` always false
under a delegation; `:305-320`: `DelegationView`, `Delegable`); TM-03's `hubapi.ViaFleet`,
`RouteName`, `AgentRoleOrchestrator` (`task-03.md:146-153`, `:355-371`); TM-04's `agents` table,
whose `UNIQUE (org_id, owner_user_id, id)` is *"the target of every acts key"*
(`task-04.md:325-345`), and `delegations` (`:289`). Phase W on `feat/work-layer`: task 06's
`issue_comments` (`task-06.md:291-316`, `kind` checked `IN ('comment')`, *"W3 widens (task 99)"*)
and `issues.source` (`:181`, `'orchestrator'` already allowed); task 08's `registerWorkIssues` and
`WriteReplayable` (`task-08.md:313-335`); task 07's matrix helpers (`task-07.md:355-395`); task
16's `WorkRoute` and `Relay()` (`task-16.md:106-160`); task 104's `Linker` (`task-104.md:127-131`);
task 109's `SwitchReader` and `ErrAgentsDisabled` (`task-109.md:94-131`).

**Not on any branch, so UNVERIFIED until merge:** task 99's part (not written when this part was;
every SW.19 Go name below is task 99's scope's, `tasks-81-100.md:439-470`, and Step 1 re-reads
them); TM-05's `server/auth.go` code; Hub task 13's `resolveCaller`; Hub task 14's `Tx`.

Eight decisions the scope left open, made here.

**(a) Paths follow task 16's table, not a `/v1/work/…` prefix.** The scope says *"task 08's
`/v1/work/…` convention"*; task 08 and task 16 register `/v1/issues/…`, `/v1/comments/…` and
`/v1/projects/{projectId}/…` (`task-16.md:127-156`). W3's ten routes use the same shapes, so the
relay derives them with `Relay()` unchanged.

**(b) Attribution is the principal's, never the body's.** `attribution.ownerId` in a body must
equal the principal's `UserID` (TM-05: the delegation's owner), else 403 `agent_not_owner`
(TM-03's code). The stored `agent_owner_id` is written from `tx.P`, and the database pins
`agent_owner_id = created_by` and `(org_id, agent_owner_id, agent_id)` to TM-04's `agents` key, so
Fleet holding Dylan's delegation cannot store a note that names Alice even if Go forgets.

**(c) One voice per owner, at the Hub.** An agent kind needs `via = 'fleet'`, an agent whose
`role` is `orchestrator`, and an issue whose assignee is the owner. A worker's `ag-…` is 403
`agent_not_owner`'s sibling, `forbidden` with `required: "orchestrator"`. Workers report to their
Orchestrator in Fleet (task 103); the Hub never sees five streams.

**(d) The kill switch is checked where the delegated principal is built.** TM-05's
`resolveCaller` gains one call, `s.agentSwitchGate(r, p)`: while the org's `orchestrators` is
false, every non-`GET` request with `p.Via == "fleet"` is 403 `agents_disabled`, and one
metadata-only event `org.agent_write_refused` `{route, delegationId}` is appended in its own
transaction. Reads pass, so Fleet can read the switch it is refused by. A person's write never
reaches the gate. The database half: an insert of an agent-kind comment, or of an issue with
`source = 'orchestrator'`, while the switch is off raises `agents_disabled` (the trigger below).

**(e) An Orchestrator files into `triage`.** W1's `POST /v1/projects/{projectId}/issues` is
registered through `Delegable(RouteWorkIssueCreate, …)`; under `p.Via == "fleet"` the handler
overwrites `state` with `triage` and `source` with `orchestrator` before validation (05 §1 L-193:
*"it lands in `triage` for a person"*). The database refuses an `orchestrator` insert in any other
state. `threadState` stays the request's; a `not_tied` reason is still required.

**(f) Only the assignee delegates; one live delegation per issue.** `POST
/v1/issues/{issueId}/delegate` is the assignee's own (not `Delegable`); anyone else is 403
`forbidden`, `required: "assignee"`. A repeat while live is 200 with the same record and no event.
With no live Orchestrator delegation for the owner in that project (TM-04's `delegations`, class
`orchestrator`), it is 422 `orchestrator_unavailable`: the key is team mode's. `DELETE` on the
same path withdraws it (task 102's *Undo*, 05 §4.6: *"reversible until the session's first file
write"*): the assignee's own, sets `withdrawn_at`, appends `issue.undelegated`; a repeat, or a
withdrawal with nothing live, is 200 with no event. The Hub cannot see a file write, so it does not
refuse a late withdrawal; the desktop offers *Undo* only while the control plane has observed none
(task 102), and Fleet stops what the delegation started when it reads the event (task 98).

**(g) A proposal never writes evidence; a link is its own write.** `evidence_proposal` rows live
in `issue_comments`. `POST /v1/comments/{commentId}/recorded-as` `{evidenceId}` (task 104's
`LinkEvidenceProposal`) is a person's route: the record must exist in the issue's project, carry
the proposal's `criterionId` and `commit`, and have been recorded by the caller. It sets
`recorded_as` once (a repeat with the same id is 200, no event; another id is 409 `conflict`) and
appends `comment.proposal_linked`. The Hub's `POST /v1/evidence` is unchanged.

**(h) Every write appends one metadata-only event, and no text reaches `data`.** Kinds, ids,
versions and booleans only; guidance text, comment text, `because` and the switch's `reason` never
(hub-contract §8). The events are task 99's spellings (UNVERIFIED), listed with the routes.

**Files** (under `control-plane/internal/hub/` unless the path says otherwise):

| Path | Change |
|---|---|
| `migrate/sql/NNNN_work_agents.sql` | new: the widened comment kinds, attribution, delegations, guidance, the switch, the guards |
| `store/work_agents.go` | new: the W3 store functions, each one statement set inside the caller's `Tx` |
| `server/work_agents.go` | new: `registerWorkAgents`, the ten handlers, `agentSwitchGate` |
| `server/server.go` ⚑ | one line, `s.registerWorkAgents()`, beside `registerWorkIssues` |
| `server/work_issues.go` ⚑ | `createIssue` registered through `Delegable`; the triage override (decision (e)) |
| `server/auth.go` ⚑ (TM-05's) | one call, `s.agentSwitchGate(r, p)`, after the delegated principal is built (decision (d)) |
| `server/work_matrix_rows_integration_test.go` ⚑ (task 07's) | `workMatrixRows["100"]`; `"100"` in `workMatrixLive` |
| `server/work_agents_integration_test.go` | new: the Fleet and sync columns, the guards, the events |
| `hubclient/methods_work_agents.go`, `methods_work_agents_test.go` | new: the W3 methods over HTTP |
| `hubclient/work_agents_integration_test.go` | new: task 99's `RunWorkAgents` against the real Hub |
| `control-plane/internal/api/work.go` ⚑, `routes_audit_test.go` ⚑ | the relay registers `WorkAgentRoutes`; nine catalog rows |

The scope named `server/matrix_integration_test.go`; W1's Work rows live in task 07's
`work_matrix_rows_integration_test.go` (`task-07.md:317`), so W3's join them there. `server.go`,
`work_issues.go` and `auth.go` are added to the scope's list: decisions (d) and (e) cannot be held
anywhere else. `server/changes.go` needs no change: task 13 filters the feed by project and team,
not by kind (`task-13.md:122-139`, UNVERIFIED that its final code keeps no kind list; Step 1 greps).

#### The route table, complete: `hubclient/methods_work_agents.go`, the head

```go
// control-plane/internal/hub/hubclient/methods_work_agents.go
package hubclient

import (
	"context"
	"net/http"
	"net/url"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// WorkAgentRoutes is every W3 route, in this order. It reuses task 16's WorkRoute, so the relay
// derives its pattern with Relay() and methods_work_agents_test.go holds one Client method per
// row and one row per WorkAgents method (task 99's interface).
var WorkAgentRoutes = []WorkRoute{
	{"DelegateIssue", http.MethodPost, "/v1/issues/{issueId}/delegate", false},
	{"WithdrawDelegation", http.MethodDelete, "/v1/issues/{issueId}/delegate", false},
	{"AddAgentComment", http.MethodPost, "/v1/issues/{issueId}/agent-comments", true},
	{"ListEvidenceProposals", http.MethodGet, "/v1/projects/{projectId}/evidence-proposals", false},
	{"LinkEvidenceProposal", http.MethodPost, "/v1/comments/{commentId}/recorded-as", false},
	{"PutGuidance", http.MethodPost, "/v1/guidance/{scope}/{scopeId}/versions", true},
	{"GuidanceVersions", http.MethodGet, "/v1/guidance/{scope}/{scopeId}/versions", false},
	{"ResolveGuidance", http.MethodGet, "/v1/guidance/resolve", false},
	{"GetAgentSwitch", http.MethodGet, "/v1/orgs/{orgId}/agent-switch", false},
	{"SetAgentSwitch", http.MethodPut, "/v1/orgs/{orgId}/agent-switch", false},
}

// Route names a delegation may list (TM-03's RouteName). Four W3 routes, and W1's create, are
// Delegable; the rest are a person's own, and Fleet calling them is 403 forbidden, required
// "route" (TM-05 decision (d)).
const (
	RouteWorkIssueCreate      hubapi.RouteName = "work.issues.create"
	RouteWorkAgentComment     hubapi.RouteName = "work.agent_comments.create"
	RouteWorkProposalsRead    hubapi.RouteName = "work.evidence_proposals.read"
	RouteWorkGuidanceResolve  hubapi.RouteName = "work.guidance.resolve"
	RouteWorkAgentSwitchRead  hubapi.RouteName = "work.agent_switch.read"
)
```

`LinkEvidenceProposal`, `GuidanceVersions` and `WithdrawDelegation` are **not** in task 99's scope's method list
(`tasks-81-100.md:452-455`). The first is task 104's `Linker` (`task-104.md:127-131`); the second
is task 109's history list (decision 7, `task-109.md:49-52`); the third is task 102's *Undo*
(decision (f)). All three are additions to SW.19 that task 99's part must carry; if it does not, this part's gap 1 says so. The route names are placed in
`hubclient` only for this listing; Step 3 moves them to `hubapi/work_agents.go` beside task 99's
types if task 99 has not declared them (UNVERIFIED which).

#### `hubclient/methods_work_agents.go`, the methods, complete

The signatures are task 99's `WorkAgents` interface, restated so this part is complete; the two
additions are marked. `agentWork` is task 16's `work` (`task-16.md:216-223`) over this table, so
task 16's file is not edited.

```go
func agentRoute(method string) WorkRoute {
	for _, r := range WorkAgentRoutes {
		if r.Method == method {
			return r
		}
	}
	panic("hubclient: no W3 route for " + method) // methods_work_agents_test.go holds the pairing
}

func (c *Client) agentWork(ctx context.Context, method string, q url.Values, body, out any, args ...string) error {
	rt := agentRoute(method)
	path := workPath(rt.Hub, args...)
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return c.send(ctx, rt.Verb, rt.Hub, path, body, out)
}

// WithdrawDelegation is an addition to task 99's list: task 102's Undo.
func (c *Client) WithdrawDelegation(ctx context.Context, issueID string) (hubapi.IssueDelegation, error) {
	var out hubapi.IssueDelegation
	return out, c.agentWork(ctx, "WithdrawDelegation", nil, nil, &out, issueID)
}

func (c *Client) DelegateIssue(ctx context.Context, issueID string) (hubapi.IssueDelegation, error) {
	var out hubapi.IssueDelegation
	return out, c.agentWork(ctx, "DelegateIssue", nil, struct{}{}, &out, issueID)
}

func (c *Client) AddAgentComment(ctx context.Context, issueID string, req hubapi.AgentCommentRequest) (hubapi.Comment, error) {
	var out hubapi.Comment
	return out, c.agentWork(ctx, "AddAgentComment", nil, req, &out, issueID)
}

func (c *Client) ListEvidenceProposals(ctx context.Context, projectID string, q hubapi.ProposalQuery) ([]hubapi.EvidenceProposalComment, error) {
	v := url.Values{}
	if q.IssueID != "" {
		v.Set("issue", q.IssueID)
	}
	if q.BriefKey != "" {
		v.Set("brief", q.BriefKey)
	}
	var out struct {
		Items []hubapi.EvidenceProposalComment `json:"items"`
	}
	return out.Items, c.agentWork(ctx, "ListEvidenceProposals", v, nil, &out, projectID)
}

// LinkEvidenceProposal is an addition to task 99's list: task 104's Linker.
func (c *Client) LinkEvidenceProposal(ctx context.Context, proposalID, evidenceID string) (hubapi.EvidenceProposalComment, error) {
	var out hubapi.EvidenceProposalComment
	return out, c.agentWork(ctx, "LinkEvidenceProposal", nil, map[string]string{"evidenceId": evidenceID}, &out, proposalID)
}

func (c *Client) PutGuidance(ctx context.Context, scope hubapi.GuidanceScope, scopeID string, req hubapi.PutGuidanceRequest) (hubapi.Guidance, error) {
	var out hubapi.Guidance
	return out, c.agentWork(ctx, "PutGuidance", nil, req, &out, string(scope), scopeID)
}

// GuidanceVersions is an addition to task 99's list: task 109's history, newest first.
func (c *Client) GuidanceVersions(ctx context.Context, scope hubapi.GuidanceScope, scopeID string) ([]hubapi.Guidance, error) {
	var out struct {
		Items []hubapi.Guidance `json:"items"`
	}
	return out.Items, c.agentWork(ctx, "GuidanceVersions", nil, nil, &out, string(scope), scopeID)
}

func (c *Client) ResolveGuidance(ctx context.Context, teamID string) (hubapi.GuidanceView, error) {
	var out hubapi.GuidanceView
	return out, c.agentWork(ctx, "ResolveGuidance", url.Values{"team": {teamID}}, nil, &out)
}

func (c *Client) GetAgentSwitch(ctx context.Context, orgID string) (hubapi.AgentSwitch, error) {
	var out hubapi.AgentSwitch
	return out, c.agentWork(ctx, "GetAgentSwitch", nil, nil, &out, orgID)
}

func (c *Client) SetAgentSwitch(ctx context.Context, orgID string, req hubapi.SetAgentSwitchRequest) (hubapi.AgentSwitch, error) {
	var out hubapi.AgentSwitch
	return out, c.agentWork(ctx, "SetAgentSwitch", nil, req, &out, orgID)
}
```

#### The wire, complete: each route's body, answer, refusals and event

Bodies and answers are task 99's JSON names (UNVERIFIED until its part; Step 1). `User` is the
Hub's (`hub-contract.md`, §7). Every refusal is the Hub's problem shape with `code` and `details`.

```jsonc
// POST /v1/issues/{issueId}/delegate            (the assignee's own; not Delegable)
{}                                                   // body: nothing to choose (decision (f))
// 200 — IssueDelegation
{ "issueId": "is-…", "ownerId": "u-…", "delegatedAt": "…", "delegationId": "dl-…",
  "withdrawnAt": null }
// 403 forbidden {required:"assignee"} · 422 orchestrator_unavailable {projectId}
// 409 conflict when the issue is erased · event issue.delegated {issueId, revision, ownerId}
// (task 98's scope's data, the head revision at the moment of delegation); a delegation
// withdrawn earlier is made live again by the same POST, with a new delegatedAt and the event

// DELETE /v1/issues/{issueId}/delegate          (the assignee's own; not Delegable)
// 200 — IssueDelegation with withdrawnAt set · 404 delegation_not_found when never delegated
// 403 forbidden {required:"assignee"} · a repeat: 200, no event
// event issue.undelegated {issueId, ownerId}

// POST /v1/issues/{issueId}/agent-comments      (Delegable: work.agent_comments.create)
{ "kind": "note" | "evidence_proposal" | "negotiation",
  "body": { "text": "assumed chunk size 8 MB; the brief does not say", "author": "A" },
  "attribution": { "ownerId": "u-…", "agentId": "ag-…", "model": "claude-opus-4-8",
                   "sessionId": "…" },
  "parentId": null,
  "proposal":    { "briefKey": "brief:bf-…@6", "criterionId": "c-40aa…", "issueId": "is-…",
                   "commit": "7c1f0a2…40 hex", "artefact": { "kind": "test", "ref": "…" },
                   "because": "…" },                 // only for evidence_proposal
  "negotiation": { "threadId": "th-…", "counterpartIssueId": "is-…" } }   // only for negotiation
// 201 — Comment, task 03's shape, with kind, attribution and proposal or negotiation filled
// 403 forbidden {required:"delegation"} for a person principal · 403 agent_not_owner
// 403 forbidden {required:"orchestrator"} for a worker · 403 forbidden {required:"assignee"}
// 403 agents_disabled · 400 invalid_request {field} for a missing model, sessionId, proposal
// field or a body.author other than "A" · 422 key_unresolvable {field:"proposal.briefKey"}
// events comment.created {commentId, issueId, kind, ownerId, agentId} and, for each mention,
// comment.mentioned (task 13's kind; W3 bodies carry no mentions, so none in practice)

// GET /v1/projects/{projectId}/evidence-proposals?issue=is-…&brief=brief:bf-…@6
// 200 { "items": [ EvidenceProposalComment ] }  oldest first; a project read (V ✓)
// EvidenceProposalComment, flat, as task 104's TypeScript reads it (task-104.md:358-370):
// { "id": "ic-…", "issueId": "is-…", "kind": "evidence_proposal", "proposal": EvidenceProposal,
//   "attribution": AgentAttribution, "recordedAs": "ev-…" | null, "createdAt": "…",
//   "erasedAt": null }

// POST /v1/comments/{commentId}/recorded-as     (a person's own; not Delegable)
{ "evidenceId": "ev-…" }
// 200 — EvidenceProposalComment with recordedAs set; a repeat with the same id: 200, no event
// 404 evidence_not_found · 409 conflict {recordedAs} for a second id · 422 proposal_mismatch
// {field:"criterionId"|"commit"} · 403 forbidden {required:"recorder"} when the record's
// recordedBy is not the caller · event comment.proposal_linked {commentId, evidenceId}

// POST /v1/guidance/{scope}/{scopeId}/versions  scope org | team | person
{ "text": "…", "baseVersion": 3 }                    // 0 for the first version
// 201 — Guidance { "id": "gd-…", "scope": "team", "scopeId": "tm-…", "text": "…",
//                  "version": 4, "author": User, "at": "…" }
// 409 revision_conflict {headVersion} · 400 invalid_request {field:"text"} (1-8000 characters)
// 403 forbidden {required:"org_admin"|"team_admin"|"self"} · event guidance.versioned
// {guidanceId, scope, scopeId, version}   (never the text)

// GET /v1/guidance/{scope}/{scopeId}/versions   → 200 { "items": [ Guidance ] } newest first
// GET /v1/guidance/resolve?team=tm-…           (Delegable: work.guidance.resolve)
// 200 — GuidanceView { "layers": [ Guidance(person), Guidance(team), Guidance(org) ] },
// most local first; an absent layer is omitted, never an empty Guidance

// GET /v1/orgs/{orgId}/agent-switch             (Delegable: work.agent_switch.read)
// 200 — AgentSwitch { "orchestrators": true, "by": User | null, "at": "…" | null,
//                     "reason": "" }             // never set: on, by nobody
// PUT /v1/orgs/{orgId}/agent-switch             (org admin only)
{ "orchestrators": false, "reason": "incident 4411: agents paused while we investigate" }
// 200 — AgentSwitch · 400 invalid_request {field:"reason"} when turning off without 1-500
// characters · 403 forbidden {required:"org_admin"} · a repeat of the same value: 200, no event
// event org.agent_switch_set {orchestrators}   (never the reason)
```

`agents_disabled` is 403, not 503: the Hub is up, and a retry will not succeed until a person acts.
The client's retries are Hub tasks H33-H34's (`task-16.md:5`); that they never retry a 403 is
**UNVERIFIED**, and `TestAgentsDisabledIsNotRetried` below holds it for this route set.

#### The migration, complete: `migrate/sql/NNNN_work_agents.sql`, part 1 of 2

Applied by `hub migrate` as `hub_owner`, in one transaction, after task 74's W2 migration and
TM-04's `00NN_team.sql` (it references `agents` and `delegations`). Additive to every Hub table;
it widens task 06's `issue_comments`, a Work table, by dropping one `CHECK` and adding columns.

```sql
-- NNNN_work_agents.sql — Phase W, W3: agents on the work. Requires NNNN_work_w1.sql and
-- 00NN_team.sql. Nothing here is read by an agent's context; all text stays text·H or text·A.
SET LOCAL search_path = hub;

-- ── the kill switch, one row per org; no row reads as "on" (decision (d)) ──────────────────
CREATE TABLE org_agent_switches (
  org_id         text        PRIMARY KEY REFERENCES orgs (id),
  orchestrators  boolean     NOT NULL,
  reason         text        NOT NULL DEFAULT '' CHECK (char_length(reason) <= 500),
  set_at         timestamptz NOT NULL DEFAULT now(),
  set_by         text        NOT NULL,
  CONSTRAINT org_agent_switches_off_has_reason
    CHECK (orchestrators OR char_length(btrim(reason)) BETWEEN 1 AND 500),
  FOREIGN KEY (org_id, set_by) REFERENCES users (org_id, id)
);

CREATE FUNCTION agents_disabled(p_org text) RETURNS boolean
  LANGUAGE sql STABLE AS $$
    SELECT COALESCE((SELECT NOT orchestrators FROM hub.org_agent_switches WHERE org_id = p_org),
                    false) $$;

-- ── comment kinds and attribution (SW.19; task 06 left kind IN ('comment')) ──────────────────
ALTER TABLE issue_comments DROP CONSTRAINT issue_comments_kind_known;
ALTER TABLE issue_comments
  ADD COLUMN agent_owner_id   text,
  ADD COLUMN agent_id         text,
  ADD COLUMN agent_model      text CHECK (agent_model ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{0,63}$'),
  ADD COLUMN agent_session_id text CHECK (char_length(agent_session_id) BETWEEN 1 AND 128),
  ADD COLUMN via              text CHECK (via IS NULL OR via = 'fleet'),
  ADD COLUMN proposal         jsonb CHECK (proposal IS NULL OR jsonb_typeof(proposal) = 'object'),
  ADD COLUMN negotiation      jsonb CHECK (negotiation IS NULL
                                           OR jsonb_typeof(negotiation) = 'object'),
  ADD COLUMN recorded_as      text REFERENCES evidence (id),
  ADD CONSTRAINT issue_comments_kind_known
    CHECK (kind IN ('comment', 'note', 'evidence_proposal', 'negotiation')),
  -- an agent kind is attributed in full, via Fleet, as text·A; a person's comment is not
  ADD CONSTRAINT issue_comments_agent_attributed CHECK (
    (kind = 'comment' AND agent_id IS NULL AND agent_owner_id IS NULL AND agent_model IS NULL
       AND agent_session_id IS NULL AND via IS NULL)
    OR (kind <> 'comment' AND agent_id IS NOT NULL AND agent_owner_id IS NOT NULL
       AND agent_model IS NOT NULL AND agent_session_id IS NOT NULL AND via IS NOT NULL
       AND author_class = 'A')),
  -- decision (b): the owner is the writer, and the agent is the owner's (TM-04's acts key)
  ADD CONSTRAINT issue_comments_owner_is_writer
    CHECK (agent_owner_id IS NULL OR agent_owner_id = created_by),
  ADD CONSTRAINT issue_comments_agent_is_owners
    FOREIGN KEY (org_id, agent_owner_id, agent_id) REFERENCES agents (org_id, owner_user_id, id),
  ADD CONSTRAINT issue_comments_proposal_kind
    CHECK ((kind = 'evidence_proposal') = (proposal IS NOT NULL)),
  ADD CONSTRAINT issue_comments_negotiation_kind
    CHECK ((kind = 'negotiation') = (negotiation IS NOT NULL)),
  ADD CONSTRAINT issue_comments_recorded_is_a_proposal
    CHECK (recorded_as IS NULL OR kind = 'evidence_proposal'),
  -- task 14's erase blanks the proposal's because too; ids, commit and criterion stay
  ADD CONSTRAINT issue_comments_erased_proposal_is_blank
    CHECK (erased_at IS NULL OR proposal IS NULL OR proposal->>'because' = '');
CREATE INDEX issue_comments_proposals ON issue_comments (org_id, issue_id, created_at)
  WHERE kind = 'evidence_proposal';
```

The proposal's shape inside `jsonb` is checked in Go (task 99's `EvidenceProposal.Valid`) and by
two further `CHECK`s in part 2; the database holds the keys every rule reads (`criterionId`,
`commit`), not the prose. A `negotiation`'s text is never here: it stays in Phase T's `messages`
(task 99's scope), so its `body` is `''` (part 2) and the row carries the thread id and the
counterpart issue only.

#### The migration, part 2 of 2: delegations, guidance, the database guards, RLS, grants

```sql
ALTER TABLE issue_comments
  ADD CONSTRAINT issue_comments_negotiation_has_no_text
    CHECK (kind <> 'negotiation' OR body = ''),
  ADD CONSTRAINT issue_comments_proposal_keys CHECK (proposal IS NULL OR (
    proposal->>'commit' ~ '^[0-9a-f]{40}$'
    AND char_length(proposal->>'criterionId') BETWEEN 1 AND 128
    AND proposal->>'issueId' = issue_id));

-- ── one row per (issue, owner) who delegated it; "live" means not withdrawn and the owner
--    is still the assignee. The audit of each delegate and withdraw is the event, not the row ──
CREATE TABLE issue_delegations (
  org_id        text        NOT NULL REFERENCES orgs (id),
  issue_id      text        NOT NULL,
  owner_user_id text        NOT NULL,
  delegation_id text        NOT NULL REFERENCES delegations (id),        -- TM-04's dl- row
  delegated_at  timestamptz NOT NULL DEFAULT now(),
  withdrawn_at  timestamptz,
  CONSTRAINT issue_delegations_withdrawn_after CHECK (withdrawn_at IS NULL
                                                      OR withdrawn_at >= delegated_at),
  PRIMARY KEY (org_id, issue_id, owner_user_id),
  FOREIGN KEY (org_id, issue_id)      REFERENCES issues (org_id, id),
  FOREIGN KEY (org_id, owner_user_id) REFERENCES users (org_id, id)
);

-- ── guidance: append-only versions per (scope, scopeId), text·H only (L-183) ──────────────
CREATE TABLE guidance_versions (
  id         text        PRIMARY KEY CHECK (id ~ '^gd-[0-9a-f]{16}$'),
  org_id     text        NOT NULL REFERENCES orgs (id),
  scope      text        NOT NULL CHECK (scope IN ('org', 'team', 'person')),
  scope_id   text        NOT NULL,
  version    integer     NOT NULL CHECK (version >= 1),
  author_class text      NOT NULL DEFAULT 'H' CHECK (author_class = 'H'),
  body       text        NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by text        NOT NULL,
  erased_at  timestamptz,
  CONSTRAINT guidance_body_bounds
    CHECK (erased_at IS NOT NULL OR char_length(btrim(body)) BETWEEN 1 AND 8000),
  CONSTRAINT guidance_erased_is_blank CHECK (erased_at IS NULL OR body = ''),
  CONSTRAINT guidance_org_scope_is_the_org CHECK (scope <> 'org' OR scope_id = org_id),
  CONSTRAINT guidance_scope_id_grammar CHECK (
    (scope = 'team' AND scope_id ~ '^tm-') OR (scope = 'person' AND scope_id ~ '^u-')
    OR scope = 'org'),
  UNIQUE (org_id, scope, scope_id, version),               -- two racing saves: one 409
  FOREIGN KEY (org_id, created_by) REFERENCES users (org_id, id)
);

-- ── the database halves of decisions (b)-(e) ──────────────────────────────────────────────
CREATE FUNCTION work_agent_comment_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.kind = 'comment' THEN RETURN NEW; END IF;
  IF hub.agents_disabled(NEW.org_id) THEN
    RAISE EXCEPTION 'agents_disabled' USING ERRCODE = 'check_violation'; END IF;
  IF NOT EXISTS (SELECT 1 FROM hub.agents a WHERE a.org_id = NEW.org_id AND a.id = NEW.agent_id
                   AND a.role = 'orchestrator' AND a.ended_at IS NULL) THEN
    RAISE EXCEPTION 'agent_not_orchestrator' USING ERRCODE = 'check_violation'; END IF;
  IF NOT EXISTS (SELECT 1 FROM hub.issues i WHERE i.org_id = NEW.org_id AND i.id = NEW.issue_id
                   AND i.assignee_user_id = NEW.agent_owner_id AND i.erased_at IS NULL) THEN
    RAISE EXCEPTION 'owner_not_assignee' USING ERRCODE = 'check_violation'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issue_comments_agent_guard BEFORE INSERT ON issue_comments
  FOR EACH ROW EXECUTE FUNCTION work_agent_comment_guard();

CREATE FUNCTION work_recorded_once() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.recorded_as IS NOT NULL AND NEW.recorded_as IS DISTINCT FROM OLD.recorded_as THEN
    RAISE EXCEPTION 'recorded_as_set' USING ERRCODE = 'check_violation'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issue_comments_recorded_once BEFORE UPDATE OF recorded_as ON issue_comments
  FOR EACH ROW EXECUTE FUNCTION work_recorded_once();

CREATE FUNCTION work_orchestrator_files_triage() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.source <> 'orchestrator' THEN RETURN NEW; END IF;
  IF NEW.state <> 'triage' THEN
    RAISE EXCEPTION 'orchestrator_not_triage' USING ERRCODE = 'check_violation'; END IF;
  IF hub.agents_disabled(NEW.org_id) THEN
    RAISE EXCEPTION 'agents_disabled' USING ERRCODE = 'check_violation'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER issues_orchestrator_triage BEFORE INSERT ON issues
  FOR EACH ROW EXECUTE FUNCTION work_orchestrator_files_triage();
```

```sql
-- ── row-level security: task 06's loop, over exactly the three new tables ──────────────────
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['org_agent_switches', 'issue_delegations', 'guidance_versions'] LOOP
    EXECUTE format('ALTER TABLE hub.%I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY org_isolation ON hub.%I TO hub_app
                    USING (org_id = current_setting(''hub.org_id'', true))
                    WITH CHECK (org_id = current_setting(''hub.org_id'', true))', t);
  END LOOP;
END $$;

-- ── grants: explicit, as task 06's are ────────────────────────────────────────────────────
GRANT SELECT, INSERT ON org_agent_switches, issue_delegations, guidance_versions TO hub_app;
GRANT UPDATE (orchestrators, reason, set_at, set_by) ON org_agent_switches TO hub_app;
GRANT UPDATE (recorded_as)                           ON issue_comments     TO hub_app;
GRANT UPDATE (delegation_id, delegated_at, withdrawn_at) ON issue_delegations TO hub_app;
GRANT EXECUTE ON FUNCTION agents_disabled(text) TO hub_app;
-- No DELETE on issue_delegations, and no UPDATE or DELETE on guidance_versions: a new
-- version is a new row; a withdrawn delegation keeps its row. Erasing guidance text is not
-- granted here (gap 3).
-- No UPDATE of issue_comments' kind, agent_*, via, proposal or negotiation, ever.
```

The trigger functions run as the invoker, so their reads of `agents` and `issues` pass through
the same `org_isolation` policy as the insert they guard: a row of another org is invisible to
them and the guard refuses, never leaks. The three messages are **database codes**, not problem
codes: the store never returns them to a caller, because Go refuses first (decision (b)-(e)); a
test that sees one on the wire has found a Go check that is missing.

#### The store, `store/work_agents.go`, signatures complete

```go
// control-plane/internal/hub/store/work_agents.go
package store

// Each function runs inside the caller's Tx (Hub task 14): one statement set, the event
// appended by the caller's WriteResult. None opens its own transaction; none reads text into
// an event.

// AgentSwitch reads the org's row; no row answers on, by nobody (decision (d)).
func AgentSwitch(ctx context.Context, tx *Tx, orgID string) (hubapi.AgentSwitch, error)

// SetAgentSwitch upserts the row and reports whether the value changed; unchanged writes
// nothing, so the handler appends no event.
func SetAgentSwitch(ctx context.Context, tx *Tx, orgID string, on bool, reason, by string) (hubapi.AgentSwitch, bool, error)

// Delegate inserts the (issue, owner) row, or makes a withdrawn one live again with a new
// delegated_at. It answers the existing row and false on a repeat while live.
func Delegate(ctx context.Context, tx *Tx, issueID, ownerID, delegationID string) (hubapi.IssueDelegation, bool, error)

// Withdraw sets withdrawn_at on a live row. changed is false when it was already withdrawn;
// ErrNoDelegation when no row exists (the handler's 404 delegation_not_found).
func Withdraw(ctx context.Context, tx *Tx, issueID, ownerID string) (hubapi.IssueDelegation, bool, error)

// LiveOrchestratorDelegation finds the owner's unexpired, unrevoked orchestrator-class
// delegation for the project (TM-04's delegations), or "" when none.
func LiveOrchestratorDelegation(ctx context.Context, tx *Tx, projectID, ownerID string) (string, error)

// AgentRole answers the agent's role and owner, and whether it has ended (TM-04's agents).
func AgentRole(ctx context.Context, tx *Tx, agentID string) (role hubapi.AgentRole, ownerID string, ended bool, err error)

// InsertAgentComment writes one agent-kind comment; ownerID is the principal's (decision (b)).
func InsertAgentComment(ctx context.Context, tx *Tx, issueID, ownerID string, req hubapi.AgentCommentRequest) (hubapi.Comment, error)

// Proposals lists evidence_proposal comments of the project, oldest first, filtered by issue
// or brief key; erased rows are listed with blank text, never dropped.
func Proposals(ctx context.Context, tx *Tx, projectID string, q hubapi.ProposalQuery) ([]hubapi.EvidenceProposalComment, error)

// LinkProposal sets recorded_as once. changed is false on a repeat of the same id; a second
// id is ErrRecordedAs, which the handler answers as 409 conflict.
func LinkProposal(ctx context.Context, tx *Tx, commentID, evidenceID string) (hubapi.EvidenceProposalComment, bool, error)

// EvidenceForLink reads the one evidence row the link checks: project, criterion, commit, and
// who recorded it (hub-contract §7.10's columns).
func EvidenceForLink(ctx context.Context, tx *Tx, evidenceID string) (projectID, criterionID, commit, recordedBy string, err error)

// AppendGuidance inserts version base+1; a UNIQUE violation is ErrGuidanceConflict (409).
func AppendGuidance(ctx context.Context, tx *Tx, scope hubapi.GuidanceScope, scopeID string, base int, text, by string) (hubapi.Guidance, error)

// GuidanceVersions lists one scope's versions, newest first; GuidanceHeads reads the newest
// version of each of up to three scopes, for ResolveGuidance.
func GuidanceVersions(ctx context.Context, tx *Tx, scope hubapi.GuidanceScope, scopeID string) ([]hubapi.Guidance, error)
func GuidanceHeads(ctx context.Context, tx *Tx, orgID, teamID, userID string) ([]hubapi.Guidance, error)

var (
	ErrRecordedAs       = errors.New("store: recorded_as is already another evidence id")
	ErrGuidanceConflict = errors.New("store: a guidance version at base+1 exists")
	ErrNoDelegation     = errors.New("store: this owner never delegated this issue")
)
```

#### The routes, `server/work_agents.go`: registration, the gate, and the agent-comment order

The chassis calls are task 08's (`s.route`, `WriteReplayable`, `ProjectRole`, `Require`,
`tx.Emit`; `task-08.md:313-372`) and task 12's `Forbidden(required)` (`task-12.md:356`). The
other seven handlers follow `postAgentComment`'s shape: decode, role, the rule, the store call,
one `tx.Emit`, the answer. Their rule lists are the wire section above, in that order.

```go
// control-plane/internal/hub/server/work_agents.go
package server

// registerWorkAgents is called once from server.go, beside registerWorkIssues.
func (s *Server) registerWorkAgents() {
	d := func(name hubapi.RouteName, h http.HandlerFunc) http.HandlerFunc {
		return Delegable(name, h).ServeHTTP // TM-05: a route without a name is Fleet-forbidden
	}
	s.route("POST /v1/issues/{issueId}/delegate", s.postDelegate)
	s.route("DELETE /v1/issues/{issueId}/delegate", s.deleteDelegate)
	s.route("POST /v1/issues/{issueId}/agent-comments", d(hubclient.RouteWorkAgentComment, s.postAgentComment))
	s.route("GET /v1/projects/{projectId}/evidence-proposals", d(hubclient.RouteWorkProposalsRead, s.listProposals))
	s.route("POST /v1/comments/{commentId}/recorded-as", s.postRecordedAs)
	s.route("POST /v1/guidance/{scope}/{scopeId}/versions", s.postGuidance)
	s.route("GET /v1/guidance/{scope}/{scopeId}/versions", s.listGuidance)
	s.route("GET /v1/guidance/resolve", d(hubclient.RouteWorkGuidanceResolve, s.resolveGuidance))
	s.route("GET /v1/orgs/{orgId}/agent-switch", d(hubclient.RouteWorkAgentSwitchRead, s.getAgentSwitch))
	s.route("PUT /v1/orgs/{orgId}/agent-switch", s.putAgentSwitch)
}

// agentSwitchGate is the one call TM-05's resolveCaller makes after it builds a delegated
// principal (decision (d)). Reads pass. A refused write appends org.agent_write_refused in its
// own transaction, so the refusal is audited even though the write never ran.
func (s *Server) agentSwitchGate(r *http.Request, p Principal) error {
	if p.Via != hubapi.ViaFleet || r.Method == http.MethodGet {
		return nil
	}
	off, err := s.orgAgentsDisabled(r.Context(), p.OrgID) // store.AgentSwitch in a read Tx
	if err != nil || !off {
		return err
	}
	s.appendOwnEvent(r.Context(), p, Event{Kind: "org.agent_write_refused", Subject: p.OrgID,
		Data: map[string]any{"route": string(RouteFrom(r.Context())), "delegationId": p.DelegationID}})
	return Refuse(http.StatusForbidden, "agents_disabled", nil)
}

func (s *Server) postAgentComment(w http.ResponseWriter, r *http.Request) {
	issueID := r.PathValue("issueId")
	s.WriteReplayable(w, r, httpx.MaxBodyBytes, s.rebuildComment, func(tx *Tx, body []byte) (WriteResult, error) {
		ctx := r.Context()
		if tx.P.Via != hubapi.ViaFleet { // 1. a person never writes an agent kind
			return WriteResult{}, Forbidden("delegation")
		}
		var req hubapi.AgentCommentRequest
		if err := Decode(body, &req); err != nil {
			return WriteResult{}, err
		}
		if err := req.Valid(); err != nil { // 2. task 99: kind, model, sessionId, author "A", payload
			return WriteResult{}, err
		}
		if req.Attribution.OwnerID != tx.P.UserID { // 3. decision (b): never the body's owner
			return WriteResult{}, Refuse(http.StatusForbidden, "agent_not_owner", nil)
		}
		role, owner, ended, err := store.AgentRole(ctx, tx, req.Attribution.AgentID)
		if err != nil || owner != tx.P.UserID || ended {
			return WriteResult{}, orErr(err, Refuse(http.StatusForbidden, "agent_not_owner", nil))
		}
		if role != hubapi.AgentRoleOrchestrator { // 4. decision (c): one voice per owner
			return WriteResult{}, Forbidden("orchestrator")
		}
		head, err := s.readableIssue(ctx, tx, issueID, hubapi.RoleMember) // 5. 404 before 403
		if err != nil {
			return WriteResult{}, err
		}
		if head.Assignee == nil || head.Assignee.ID != tx.P.UserID { // 6. the owner's issue only
			return WriteResult{}, Forbidden("assignee")
		}
		if err := s.checkAgentPayload(ctx, tx, head, req); err != nil { // 7. brief key, thread
			return WriteResult{}, err
		}
		c, err := store.InsertAgentComment(ctx, tx, issueID, tx.P.UserID, req)
		if err != nil {
			return WriteResult{}, err
		}
		tx.Emit(Event{Kind: "comment.created", TeamID: head.TeamID, ProjectID: head.ProjectID,
			Subject: c.ID, Data: map[string]any{"issueId": issueID, "kind": string(req.Kind),
				"ownerId": tx.P.UserID, "agentId": req.Attribution.AgentID}})
		return WriteResult{Status: http.StatusCreated, Body: c}, nil
	})
}
```

New helpers in this file: `orgAgentsDisabled` (a read `Tx` over `store.AgentSwitch`),
`appendOwnEvent` (one event in its own transaction, the refusal's audit), `checkAgentPayload`
(a proposal's `briefKey` resolves in the issue's project, else 422 `key_unresolvable`; a
negotiation's `threadId` is a TM-04 thread of the project whose participants include the agent,
else 422 `thread_unresolvable`) and `orErr`. **Assumed from the sync base, UNVERIFIED:** `Refuse`
(a problem with a code and details), `readableIssue` (task 08's read-then-role order, 404 before
403), `rebuildComment` (task 12's replay rebuild) and `WriteResult{Status, Body}`. Step 1 maps each
to its real name; none is a new rule.

**The rule order of the other handlers**, each a refusal before any write:

- `postDelegate`: a person (`Via == ""`, since the route is not `Delegable`); the issue readable
  as member (404, then 403 `required: member`); not erased (409 `conflict`); `assignee == caller`
  (403 `required: assignee`); `store.LiveOrchestratorDelegation` non-empty (422
  `orchestrator_unavailable`); `store.Delegate`; `issue.delegated` only when the row is new or
  was withdrawn.
- `deleteDelegate`: the same first four checks; `store.Withdraw` (404 `delegation_not_found` on
  `ErrNoDelegation`); `issue.undelegated` only when changed. No switch check: withdrawing is a
  person stopping work, and must work while Orchestrators are off.
- `postRecordedAs`: the comment exists and is readable (404 `comment_not_found`); kind
  `evidence_proposal` (422 `proposal_mismatch`, `field: kind`); `store.EvidenceForLink` (404
  `evidence_not_found`, also for another project's record, never a 403 that confirms it exists);
  criterion and commit equal the proposal's (422 `proposal_mismatch`); `recordedBy == caller`
  (403 `required: recorder`); `store.LinkProposal`; `comment.proposal_linked` only when changed.
- `postGuidance`: scope `org` needs `tx.P.OrgAdmin`, `team` the team's admin role, `person` that
  `scopeId == tx.P.UserID`; text bounds; `store.AppendGuidance`; `guidance.versioned`.
- `putAgentSwitch`: `{orgId} == tx.P.OrgID` (404 `org_not_found` otherwise); `tx.P.OrgAdmin`
  (403 `required: org_admin`); turning off needs a reason; `store.SetAgentSwitch`;
  `org.agent_switch_set` only when changed. Under a delegation `OrgAdmin` is always false
  (TM-05 decision (e)), and the route is not `Delegable`, so Fleet can never turn itself on.
- The three reads: role `viewer` on the project or team, the caller's own person layer only.

**`server/work_issues.go` ⚑, `createIssue`, the three changed lines** (decision (e)):

```go
		source := hubapi.SourcePerson
		if tx.P.Via == hubapi.ViaFleet { // an Orchestrator files for a person, into triage
			req.State, source = ptr(hubapi.StateTriage), hubapi.SourceOrchestrator
		}
		content, state, err := s.checkCreate(ctx, tx, teamID, projectID, &req)
		// … unchanged, except Source: source in store.IssueRow and "source": string(source)
```

and its registration becomes `s.route("POST /v1/projects/{projectId}/issues",
Delegable(hubclient.RouteWorkIssueCreate, http.HandlerFunc(s.createIssue)).ServeHTTP)`.
`req.State`'s type is task 03's `*IssueState` (UNVERIFIED; if it is a value, drop `ptr`).

#### The relay: `control-plane/internal/api/work.go` ⚑

```go
// registerWorkAgents mounts the W3 relay rows, in task 16's pattern. AddAgentComment has no
// relay: the desktop holds a person's session, and the Hub refuses a person that route.
func (a *API) registerWorkAgents(mux *routes) {
	calls := workAgentCalls()
	for _, rt := range hubclient.WorkAgentRoutes {
		if rt.Method == "AddAgentComment" {
			continue
		}
		call, ok := calls[rt.Method]
		if !ok {
			panic("api: no relay handler for hubclient.WorkAgentRoutes row " + rt.Method)
		}
		delete(calls, rt.Method)
		mux.Handle(rt.Relay(), a.relayWork(rt, call))
	}
	for name := range calls {
		panic("api: relay handler " + name + " has no hubclient.WorkAgentRoutes row")
	}
}
```

`registerWork` calls `a.registerWorkAgents(mux)` as its last line. `workAgentCalls()` holds nine
`workCall`s, each decoding its body and calling the matching `wk.` method, as `workCalls()` does.
`routes_audit_test.go` gains the nine relay patterns, each `guarded`, in the catalog's order.

#### The matrix rows: `server/work_matrix_rows_integration_test.go` ⚑, key `"100"`

§5.3's six columns, `V M A O N X` (`hub-contract.md:337-339`), with task 07's helpers
(`task-07.md:355-395`). The fixture gains one thing: `IssueID` is assigned to the `M` column's
user, so exactly one person can pass the assignee rule. The Fleet columns are below, in their own
file, because `row.want` is task 07's `[6]int` and a seventh column would change every W1 row.

```go
	"100": {
		{"POST /v1/issues/{issueId}/delegate", row{name: "delegate", method: "POST",
			path: "/v1/issues/{IssueID}/delegate", body: empty,
			want:     [6]int{403, 422, 403, 404, 404, 404}, // M passes auth; no Orchestrator here
			wantCode: map[column]string{colM: "orchestrator_unavailable", colO: "issue_not_found",
				colN: "issue_not_found", colX: "issue_not_found"}}},
		{"DELETE /v1/issues/{issueId}/delegate", row{name: "withdraw", method: "DELETE",
			path: "/v1/issues/{IssueID}/delegate",
			want:     [6]int{403, 404, 403, 404, 404, 404}, // M passes auth; never delegated here
			wantCode: map[column]string{colM: "delegation_not_found", colO: "issue_not_found",
				colN: "issue_not_found", colX: "issue_not_found"}}},
		{"POST /v1/issues/{issueId}/agent-comments", row{name: "agent-comment", method: "POST",
			path: "/v1/issues/{IssueID}/agent-comments", body: agentNote,
			want: [6]int{403, 403, 403, 403, 403, 403}}}, // every person: required "delegation"
		wread("GET /v1/projects/{projectId}/evidence-proposals",
			"/v1/projects/{ProjectID}/evidence-proposals", "project_not_found"),
		{"POST /v1/comments/{commentId}/recorded-as", row{name: "recorded-as", method: "POST",
			path: "/v1/comments/{ProposalID}/recorded-as", body: missingEvidence,
			want:     [6]int{403, 404, 404, 404, 404, 404},
			wantCode: map[column]string{colM: "evidence_not_found", colA: "evidence_not_found",
				colO: "comment_not_found", colN: "comment_not_found", colX: "comment_not_found"}}},
		wadmin("POST /v1/guidance/{scope}/{scopeId}/versions", "POST",
			"/v1/guidance/team/{TeamID}/versions", 201, "team_not_found", guidanceV1),
		wread("GET /v1/guidance/{scope}/{scopeId}/versions",
			"/v1/guidance/team/{TeamID}/versions", "team_not_found"),
		wread("GET /v1/guidance/resolve", "/v1/guidance/resolve?team={TeamID}", "team_not_found"),
		{"GET /v1/orgs/{orgId}/agent-switch", row{name: "switch-read", method: "GET",
			path: "/v1/orgs/{OrgID}/agent-switch", want: [6]int{200, 200, 200, 200, 200, 404},
			wantCode: map[column]string{colX: "org_not_found"}}},
		{"PUT /v1/orgs/{orgId}/agent-switch", row{name: "switch-set", method: "PUT",
			path: "/v1/orgs/{OrgID}/agent-switch", body: switchOff,
			want:     [6]int{403, 403, 403, 200, 403, 404}, // only the org admin, member or not
			wantCode: map[column]string{colX: "org_not_found"}}},
	},
```

```go
func agentNote(f *fixture) any {
	return map[string]any{"kind": "note", "body": map[string]any{"text": "n", "author": "A"},
		"attribution": map[string]any{"ownerId": f.MemberID, "agentId": f.AgentID,
			"model": "claude-opus-4-8", "sessionId": "s-1"}}
}
func missingEvidence(*fixture) any { return map[string]any{"evidenceId": "ev-0000000000000000"} }
func guidanceV1(*fixture) any      { return map[string]any{"text": "prefer small PRs", "baseVersion": 0} }
func switchOff(*fixture) any {
	return map[string]any{"orchestrators": false, "reason": "matrix run"}
}
```

`workMatrixLive` gains `"100"` in the commit that registers the routes. The fixture's `ProposalID`
is one `evidence_proposal` written by the fixture's Fleet principal before the columns run;
`AgentID` is one TM-04 `orchestrator` agent of the `M` user; `OrgID` is the Hub fixture's own.
`PUT … agent-switch` runs last in the file and the fixture turns Orchestrators back on after it,
so no later row runs with the switch off. The `O` column is 200 on the switch and 404 on every
project and team route, which is §5.3's reading: an org admin who is not a member acts on the
org, not inside a team.

#### The Fleet columns: `server/work_agents_integration_test.go` (`//go:build integration`)

Three principals §5.3 has no column for, each minted by TM-05's `devissuer` client-credentials
grant: **F+** Fleet with the `M` user's orchestrator delegation, which lists the four W3
`Delegable` names and `work.issues.create`; **F0** Fleet with no `X-Panacea-Delegation`; **F−**
Fleet with a delegation that lists none of them.

| Route | F+ | F0 | F− |
|---|---|---|---|
| `POST …/delegate`, `POST …/recorded-as`, `POST …/guidance/…/versions`, `PUT …/agent-switch` | 403 `forbidden` `required: route` | 401 `delegation_required` | 403 `required: route` |
| `POST …/agent-comments` | 201 | 401 | 403 `required: work.agent_comments.create` |
| `POST /v1/projects/{p}/issues` | 201, stored `triage`, `source: orchestrator` | 401 | 403 `required: work.issues.create` |
| the three reads | 200 | 401 | 403 `required: <its name>` |

**The sync principal** (SW.3) does not exist until W5 (tasks 145, 150; `task-07.md:29-31`). Its
half here is the database's: `TestTheSyncActorWritesNoAgentKind` inserts an agent-kind row with
`hub.work_actor_kind = 'integration'` and `via` null, and `issue_comments_agent_attributed`
refuses it. Task 150 adds the HTTP column (gap 4).

#### Tests to write first, each with the mutant it kills

All in `server/work_agents_integration_test.go` unless named. Each is written and seen failing
before the code it covers.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestFleetCannotAttributeANoteToAnotherOwner` | F+ holding Dylan's delegation posts `ownerId: Alice` → 403 `agent_not_owner`; a direct insert with `agent_owner_id` ≠ `created_by` fails `issue_comments_owner_is_writer` | attribution taken from the body |
| `TestAWorkerCannotWriteOnTheIssue` | an `ag-…` of role `worker` → 403 `required: orchestrator`; the trigger refuses the same insert | five voices per owner |
| `TestAPersonCannotWriteAnAgentKind` | Dylan's desktop token on `agent-comments` → 403 `required: delegation` | a person forging `text·A` attribution |
| `TestTheSwitchStopsFleetAndNotPeople` | admin sets off; F+ note → 403 `agents_disabled` and one `org.agent_write_refused`; Dylan's `CreateComment` → 201; F+ reads the switch → 200 | every write blocked; the switch a flag only the UI reads |
| `TestTheDatabaseHonoursTheSwitch` | with the row off, a direct `hub_app` insert of a note raises `agents_disabled` | a switch held in Go only |
| `TestOnlyAnOrgAdminSetsTheSwitch` | V, M, A → 403 `required: org_admin`; off without a reason → 400 `field: reason`; a repeat → 200, no event | a team admin who can stop the org |
| `TestAViewerCannotWriteGuidance` | V on team guidance → 403; M → 403; A → 201 version 1; a stale base → 409 `revision_conflict` with `headVersion: 1` | a viewer or member writing policy text |
| `TestGuidanceVersionsNeverRewrite` | versions 1 and 2, then version 1's body and `at` are byte-equal to their first read | `PutGuidance` overwrites |
| `TestAnOrchestratorIssueLandsInTriage` | F+ creates with `state: todo` → stored `triage`, `source: orchestrator`; a direct insert in `todo` fails the trigger | the body's state kept |
| `TestAProposalIsNeverEvidence` | a proposal appends `comment.created` and **no** `evidence.recorded`; Dylan's `POST /v1/evidence` appends exactly one, actor Dylan; the link appends `comment.proposal_linked` | the proposal inserted as evidence |
| `TestALinkChecksTheRecord` | another criterion → 422 `field: criterionId`; another commit → `field: commit`; Alice's record → 403 `required: recorder`; a second id → 409 | a link that marks any record |
| `TestANonAssigneeCannotDelegate` | A (not assignee) → 403 `required: assignee`; M with a live delegation → 200 and `issue.delegated`; again → 200, no event | anyone delegating anyone's issue |
| `TestAWithdrawalIsTheAssigneesAndOnce` | A's `DELETE` → 403; M's → 200, `withdrawnAt` set, one `issue.undelegated`; again → 200, no event; M's `POST` then → live again with one more `issue.delegated`; with Orchestrators off, M's `DELETE` still → 200 | a withdrawal anyone can make; a withdrawal the switch blocks |
| `TestNoTextReachesAnEvent` | every event of this file: `data` holds no value equal to any body, `because`, guidance text or reason sent | text in the event (hub-contract §8) |
| `TestTheSyncActorWritesNoAgentKind` | the insert above fails `issue_comments_agent_attributed` | a sync write taken for an agent's |

**The client's tests** (`hubclient/methods_work_agents_test.go`, against an `httptest` server):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEveryWorkAgentsMethodHasOneRoute` | reflection over task 99's `WorkAgents` plus the two additions: one `WorkAgentRoutes` row per method, one method per row | a method with no route; a stale row |
| `TestAgentCommentKeepsItsDelegationAcrossRetries` | the server answers `503` then `201`; both attempts carry the same `X-Panacea-Delegation` and `Idempotency-Key`, and one comment exists | a method that drops `via` on retry |
| `TestAgentsDisabledIsNotRetried` | a `403 agents_disabled` is one attempt and an `*Error` with that code | a client spinning on a refusal |
| `TestIdsAreEscapedInTheW3Paths` | a `scopeId` with `/` reaches the server as one segment | a path built by concatenation |

`hubclient/work_agents_integration_test.go` runs task 99's `conformance.RunWorkAgents` against the
real Hub on `control-plane/cmd/hub/compose.yaml`, with F+ as the agent principal. A row that passes
on the fake and fails here is a rule the two disagree on, and this task is not done until it is
green (task 05's promise, for W3).

#### Steps

- [ ] **Step 1: Check the sync base** (rule H1). Task 99's part is on it: re-read every name this
  part marks as task 99's (types, `WorkAgents`, codes, event kinds) and change this part's
  spelling to match, noting each change here. TM-04's `agents`, `delegations` and TM-05's
  `Delegable`, `Principal.Via`, `RouteFrom` are merged (`grep -n "func Delegable"
  control-plane/internal/hub/server/*.go` prints one line). Map the four assumed chassis names.
  `grep -n "kind" control-plane/internal/hub/server/changes.go`: if task 13's filter lists kinds,
  append the six new W3 kinds there (`issue.delegated`, `issue.undelegated`,
  `comment.proposal_linked`, `guidance.versioned`, `org.agent_switch_set`,
  `org.agent_write_refused`) and add `changes.go` ⚑ to *Files*.
- [ ] **Step 2: Write the tests first**, all of the tables above; run them and see each fail for
  the reason its row names (404 route, or 501 from the stub `Client`).
- [ ] **Step 3: Write the migration**, take the next free number, run `hub migrate` against the
  compose database, and see `TestTheDatabaseHonoursTheSwitch`, `TestTheSyncActorWritesNoAgentKind`
  and the direct-insert halves pass while the HTTP halves still fail.
- [ ] **Step 4: Write the store, the routes, the gate, the triage override and the client**, then
  the relay. Run to green.
- [ ] **Step 5: Prove the tests bite.** Apply each table's mutant by hand, see its test fail,
  revert. None is committed.
- [ ] **Step 6: Commit** with `git add` for the new files, then `git commit -F <msgfile> --` the
  paths of *Files*. Message file under `.superpowers/sdd/work/msg/`; last line
  `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.

#### Acceptance

```bash
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go build ./control-plane/... && go vet ./control-plane/internal/hub/... ./control-plane/internal/api/
# (no output)
go test ./control-plane/internal/hub/hubclient/ -race -count=1 -run 'TestEveryWorkAgentsMethodHasOneRoute|TestAgentCommentKeepsItsDelegationAcrossRetries|TestAgentsDisabledIsNotRetried|TestIdsAreEscapedInTheW3Paths' -v 2>&1 | grep -c '^--- PASS'
# 4
go test -tags integration ./control-plane/internal/hub/server/ -count=1 -run 'TestFleet|TestAWorker|TestAPerson|TestTheSwitch|TestTheDatabase|TestOnlyAnOrgAdmin|TestAViewer|TestGuidance|TestAnOrchestrator|TestAProposal|TestALink|TestANonAssignee|TestNoTextReachesAnEvent|TestTheSyncActor' -v 2>&1 | grep -c '^--- PASS'
# 14
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -run 'TestAuthorisationMatrix|TestLiveWorkRows|TestEveryRoute'
# ok   (task 07's meta-tests, `task-07.md:561-562`: the ten W3 patterns are live and each has a row)
go test -tags integration ./control-plane/internal/hub/hubclient/ -count=1 -run TestWorkAgentsRowsOnTheHub
# ok
go test ./control-plane/internal/api/ -race -count=1 -run 'TestRoutesAudit|TestWorkRelay'
# ok   (nine W3 relay rows; no relay for AddAgentComment)
```

With the variable unset every integration test is `--- SKIP`, and that is reported as a skip, not a
pass.

#### What this task does not claim

- Phase T's own Z5 matrix rows (TM-43), Fleet's use of these routes (tasks 101 onward), or any
  desktop surface (102-104, 109, 111).
- That guidance text may enter an agent's context. It is stored and resolved; whether it joins
  TM-13's policy layers is task 99's open question to the Phase T owner (UNVERIFIED).
- That the switch stops a running session. It refuses the next Fleet write; task 109's gate stops
  the next turn; a live worker is stopped by *pause all*, which is not here.
- That TM's own tables hold the switch in the database. Their writes pass through the Go gate in
  `resolveCaller` only; the database half covers W3's two tables (comments and issues).
- That a withdrawal stops an Orchestrator's work. The Hub records it and appends
  `issue.undelegated`; Fleet acting on that event is task 98's and Phase T's.
- Any parity row alone. It carries the Hub halves of **L-39, L-156, L-158, L-166, L-173, L-183,
  L-193 and L-246** (verified against 05-design's matrix, `05-design.md:148-428`), which tasks
  102-104, 109 and 111 close on the desktop and in Fleet, and task 113 records.

**Corrections to the scope paragraph** (the skeleton row wins; this part records them): paths are
task 16's shapes, not `/v1/work/…` (decision (a)); the matrix file is task 07's
`work_matrix_rows_integration_test.go`; `server.go`, `work_issues.go` and TM-05's `auth.go` are
added to *Files*; the sync principal's column is the database's until W5 (the Fleet columns
section); the relay carries nine rows, not ten, because `AddAgentComment` is Fleet's alone; a tenth
route, the withdrawal, is added for task 102's *Undo* (decision (f)).

**Gaps found while writing this part** (for the skeleton's owner and the named tasks):

1. **Task 99's part did not exist** when this was written. Its method list lacks
   `LinkEvidenceProposal` (task 104 needs it), `GuidanceVersions` (task 109's history) and
   `WithdrawDelegation` (task 102's Undo); its codes lack `orchestrator_unavailable`,
   `proposal_mismatch`, `evidence_not_found`, `delegation_not_found`, `thread_unresolvable`; its
   events are unnamed (`issue.delegated`, `issue.undelegated`, `comment.proposal_linked`,
   `guidance.versioned`, `org.agent_switch_set`, `org.agent_write_refused` are proposed here).
   Task 99 must carry each, or this part changes to its names at Step 1.
2. **Task 109 reads the switch "as Fleet's own workload principal"** (`task-109.md:94-96`), but
   TM-05 refuses an app-only Fleet token with no delegation (401 `delegation_required`). This part
   makes the read `Delegable` (`work.agent_switch.read`); task 109's gate must send the turn's
   owner's delegation, and TM-06's orchestrator class must list the name.
3. **Guidance has no erase owner.** The skeleton's `server/erase.go` ⚑ row names issues,
   comments, requests and notes (tasks 14, 146). This part adds `erased_at` and the blank `CHECK`
   so no migration is needed later; the route and its grant are unowned.
4. **The sync principal's HTTP column** belongs to task 150, which should add it beside task 07's
   `AnIntegrationNeverWritesAFact`.
5. **The withdraw route is this part's addition.** 05's *"Start or Delegate: reversible until the
   session's first file write"* (`05-design.md:1105`) needs it for task 102's Undo, and task 99's
   scope has none. Task 98 must map `issue.undelegated` so Fleet stops what the delegation started.
6. **The skeleton's file map** omits `server/auth.go` ⚑ and `server/work_issues.go` ⚑ for task 100,
   and has no row for `server/work_agents_integration_test.go`.
