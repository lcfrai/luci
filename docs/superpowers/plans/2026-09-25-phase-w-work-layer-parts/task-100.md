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
`/v1/projects/{projectId}/…` (`task-16.md:127-156`). W3's nine routes use the same shapes, so the
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
`orchestrator`), it is 422 `orchestrator_unavailable`: the key is team mode's.

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
| `server/work_agents.go` | new: `registerWorkAgents`, the nine handlers, `agentSwitchGate` |
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

`LinkEvidenceProposal` and `GuidanceVersions` are **not** in task 99's scope's method list
(`tasks-81-100.md:452-455`). The first is task 104's `Linker` (`task-104.md:127-131`); the second
is task 109's history list (decision 7, `task-109.md:49-52`). Both are additions to SW.19 that
task 99's part must carry; if it does not, this part's gap 1 says so. The route names are placed in
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
{ "issueId": "is-…", "ownerId": "u-…", "delegatedAt": "…", "delegationId": "dl-…" }
// 403 forbidden {required:"assignee"} · 422 orchestrator_unavailable {projectId}
// 409 conflict when the issue is erased · event issue.delegated {issueId, ownerId, delegationId}

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
// EvidenceProposalComment = { "comment": Comment, "proposal": EvidenceProposal,
//                             "recordedAs": "ev-…" | "" }

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
