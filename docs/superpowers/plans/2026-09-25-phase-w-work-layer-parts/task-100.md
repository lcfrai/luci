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

-- ── one row per (issue, owner) who delegated it; "live" means the owner is still assignee ──
CREATE TABLE issue_delegations (
  org_id        text        NOT NULL REFERENCES orgs (id),
  issue_id      text        NOT NULL,
  owner_user_id text        NOT NULL,
  delegation_id text        NOT NULL REFERENCES delegations (id),        -- TM-04's dl- row
  delegated_at  timestamptz NOT NULL DEFAULT now(),
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
