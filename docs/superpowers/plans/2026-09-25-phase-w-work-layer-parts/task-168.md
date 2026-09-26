### Task 168: `hubclient`'s W5 methods over HTTP, the relay rows, W5's Hub end to end

W5, stream WH, about 3 hours (an estimate, not a measurement). The SW.21 methods (task 143's
`WorkIntake`, `task-143.md:815-882`) over HTTP against the real Hub, as tasks 16 and 124 do for
W1 and W4. Depends on **144** (the fake and `conformance.WorkRowsW5`, `task-144.md:551-596`) and
**146-167** (each route part fixes its own paths). Uses task 16's `WorkRoute`, `workPath`,
`c.work`, `registerWork` and `checked` (`task-16.md:105-240`) and task 124's `AllWorkRoutes()`
(`task-124.md:159-167`).

Parity: closes **no row alone**. It is the proof path for the Hub halves of 146-167, which task 174
records. Design: 05 §5.4 W5; hub-contract §4.3 (the relay rule), §9.4 (the local caller key).

**What it proves.** Everything the fake answered in task 144, the real Hub answers the same over
HTTP; the desktop reaches exactly the W5 routes its surfaces call (170-173) and no Hub-facing
ingress; a secret crosses the relay once and is never in its log.

**Decisions made here, so nobody re-derives them:**

1. **One Go table, and the route parts win.** `WorkIntakeRoutes` is the relay's list and task
   169's route test reads it by path (`task-169.md:362-364`). Where task 143's *proposed* row
   (`task-143.md:937-952`) and the route part differ, the part is right: `DecideApp` is
   `…/decision` (159), `DisconnectTracker` is `DELETE /v1/trackers/{connectionId}` (150),
   `CreateHostHook` is `PUT /v1/projects/{projectId}/hooks/{host}` and `ListOutsidePRLinks` is
   per project (154), keys are `…/api-keys` (160), loops are under `/v1/…/work/loops` (162-163),
   the agent policy is `…/work/agent-policy` (166).
2. **Two lists.** `WorkIntakeRoutes` is mounted by the relay: only what a 170-173 surface calls.
   `workIntakeHubOnly` is client-only (the control plane or a test calls it; the relay never
   mounts it), as task 124's `briefTemplateRoutes` (`task-124.md:141-150`). `workRoute` searches
   both; `AllWorkRoutes()` appends only the first.
3. **Not relayed, by construction.** Host webhook ingress (`/v1/hooks/…`, 154-155), the Teams bot
   endpoint (`POST /teams/messages`, outside `/v1`, `task-157.md:203`, `:286`), the public form
   (148), `/mcp` (161) and the exporter (167) have no row in either list, so the relay has no
   pattern for them and `/v1/hub/work/hooks/…` is 404.
4. **Two methods have no HTTP route.** `ApplySyncBatch` runs in the Hub's own process on a janitor
   transaction (`task-150.md:35-37`), and `RequestMessages` has no route in task 157. Over HTTP
   both answer `ErrNoRoute`, a new sentinel, not `ErrNotConfigured`: the caller can tell *this
   Hub has no such route* from *no Hub*. Conformance rows W5-08 to W5-11 run against an
   in-process binding (decision 8).
5. **No W5 response carries an issue revision.** `AcceptRequestResponse` holds an `IssueRef`
   (`task-146.md:289-292`; `IssueRef` is id, number and external id, `task-03.md:399-403`), so
   there is nothing to re-hash here. The accepted issue's revision 1 is checked when it is read,
   by W1's `Issue` (task 16's `checked`, `task-16.md:226-240`).
6. **The relay logs no body.** A `201 CreatedAPIKey` or `CreatedWebhook` holds its secret once
   (`task-143.md:353-356`, `:444-447`). Task 16's `relayWork` re-encodes the typed answer with
   `httpx.WriteJSON` and logs only through `hubclient.WriteError` on a failure
   (`task-16.md:537-562`); nothing here adds a log line with a value in it. `hubclient.Error`
   passes through with the Hub's status, code and details unchanged (hub-contract §4.3).
7. **The local caller key never leaves the machine** (hub-contract §9.4). The relay's Hub call is
   `hubclient`'s, with the person's Entra token (H33); no header of the incoming request is
   copied. The one exception is the desktop's `Idempotency-Key`, carried into H33's transport
   through the context (`hubclient.WithIdempotencyKey`, a name **UNVERIFIED** in H33, gap 6), so a
   desktop retry of one op is the Hub's replay and not a second write.
8. **The end to end uses the fakes, never a real host.** `devissuer` for every token, and a mail
   fake, a host fake, a Teams fake and an ingestion fake (167's `httptest` endpoint). The sync
   principal has no token (decision 4), so `WorkEnv.Integration` binds an in-process client over
   the Hub's store for that one principal (`hubtest.SyncAs`, a name this task adds).
9. **It also builds task 166's `AgentPolicySource`** (166's gap 9): the local project's Hub
   project through `hublink` (hub tree `task-35.md:73-77`), its team, then `GetTeamAgentPolicy`.

**Files:**
- `control-plane/internal/hub/hubclient/methods_work_intake.go` (+ `_test.go`) — the table, the
  51 methods, `ErrNoRoute` (new); `client_work_intake_stub.go` — **deleted** in the same commit
  (`task-143.md:906-916`)
- `control-plane/internal/hub/hubclient/methods_work.go` ⚑ (task 16's, as 124 left it) —
  `AllWorkRoutes()` appends `WorkIntakeRoutes`; `workRoute` also searches `workIntakeHubOnly`
- `control-plane/internal/api/work.go` ⚑ — `workCalls()` gains `intakeCalls()`
- `control-plane/internal/api/work_intake_test.go` — the relay rules (new)
- `control-plane/internal/api/routes_audit_test.go` ⚑ — the W5 relay `GET`s in `catalogReadOnlyRoutes`
- `control-plane/internal/api/agentpolicy.go` (+ `_test.go`) — `hubPolicySource` (new; decision 9)
- `control-plane/cmd/api/main.go` ⚑ — `Deps.AgentPolicy` wired when `PANACEA_APP_HUB_URL` is set
- `control-plane/internal/hub/hubclient/conformance/work_intake_integration_test.go` —
  `WorkRowsW5` over HTTP, `-tags integration` (new)
- `control-plane/internal/hub/work_intake_e2e_integration_test.go` — `TestWorkIntakeE2E` (new;
  the File map names no W5 end-to-end file, so the name follows tasks 17, 82 and 124)

#### Contract — the route table, complete: `hubclient/methods_work_intake.go`, the head

```go
// control-plane/internal/hub/hubclient/methods_work_intake.go
package hubclient

// ErrNoRoute: the Hub has no HTTP route for this method (decision 4).
var ErrNoRoute = errors.New("hubclient: the Hub has no HTTP route for this method")

// WorkIntakeRoutes is every W5 route the relay mounts: the routes a 170-173
// surface calls, in task order (decision 2). A row's name is its method's; a
// method over two routes has the two names task 169 uses.
var WorkIntakeRoutes = []WorkRoute{
	// 146: requests (Triage, task 170)
	{"ListRequests", http.MethodGet, "/v1/teams/{teamId}/requests", false},
	{"Request", http.MethodGet, "/v1/requests/{requestId}", false},
	{"AcceptRequest", http.MethodPost, "/v1/requests/{requestId}/accept", false},
	{"LinkRequest", http.MethodPost, "/v1/requests/{requestId}/link", false},
	{"DeclineRequest", http.MethodPost, "/v1/requests/{requestId}/decline", false},
	{"MarkImportant", http.MethodPut, "/v1/requests/{requestId}/important", false},
	{"IssueRequesters", http.MethodGet, "/v1/issues/{issueId}/requests", false},
	// 147-148: rules and forms (170)
	{"ListIntakeRules", http.MethodGet, "/v1/teams/{teamId}/intake-rules", false},
	{"PutIntakeRules", http.MethodPut, "/v1/teams/{teamId}/intake-rules", false},
	{"CreateForm", http.MethodPost, "/v1/teams/{teamId}/forms", true},
	{"ListForms", http.MethodGet, "/v1/teams/{teamId}/forms", false},
	{"GetForm", http.MethodGet, "/v1/forms/{formId}", false},
	{"PatchForm", http.MethodPatch, "/v1/forms/{formId}", false},
	// 150-153: trackers (171)
	{"ConnectTracker", http.MethodPost, "/v1/projects/{projectId}/trackers", true},
	{"ListTrackers", http.MethodGet, "/v1/teams/{teamId}/trackers", false},
	{"DisconnectTracker", http.MethodDelete, "/v1/trackers/{connectionId}", false},
	{"ListSyncRuns", http.MethodGet, "/v1/trackers/{connectionId}/runs", false},
	{"ListTrackerLinks", http.MethodGet, "/v1/trackers/{connectionId}/links", false},
	// 162-163: loops (172)
	{"CreateLoop", http.MethodPost, "/v1/projects/{projectId}/work/loops", true},
	{"ProjectLoops", http.MethodGet, "/v1/projects/{projectId}/work/loops", false},
	{"GetLoop", http.MethodGet, "/v1/work/loops/{loopId}", false},
	{"SetLoopState", http.MethodPatch, "/v1/work/loops/{loopId}", false},
	{"PutLoopVersion", http.MethodPost, "/v1/work/loops/{loopId}/versions", true},
	{"ListLoopVersions", http.MethodGet, "/v1/work/loops/{loopId}/versions", false},
	{"ListLoopRuns", http.MethodGet, "/v1/work/loops/{loopId}/runs", false},
	{"RunLoopNow", http.MethodPost, "/v1/work/loops/{loopId}/runs", true},
	// 159-161, 166: admin (173)
	{"ListApps", http.MethodGet, "/v1/apps", false},
	{"DecideApp", http.MethodPost, "/v1/apps/{appId}/decision", false},
	{"CreatePersonalKey", http.MethodPost, "/v1/me/api-keys", true},
	{"CreateTeamKey", http.MethodPost, "/v1/teams/{teamId}/api-keys", true},
	{"MyKeys", http.MethodGet, "/v1/me/api-keys", false},
	{"TeamKeys", http.MethodGet, "/v1/teams/{teamId}/api-keys", false},
	{"RevokeAPIKey", http.MethodPost, "/v1/api-keys/{keyId}/revoke", false},
	{"CreateWebhook", http.MethodPost, "/v1/teams/{teamId}/webhooks", true},
	{"ListWebhooks", http.MethodGet, "/v1/teams/{teamId}/webhooks", false},
	{"DisableWebhook", http.MethodPost, "/v1/webhooks/{webhookId}/disable", false},
	{"RotateWebhookSecret", http.MethodPost, "/v1/webhooks/{webhookId}/rotate", false},
	{"ListDeliveries", http.MethodGet, "/v1/webhooks/{webhookId}/deliveries", false},
	{"Redeliver", http.MethodPost, "/v1/webhook-deliveries/{deliveryId}/redeliver", false},
	{"ListMCPConnectors", http.MethodGet, "/v1/work/mcp-connectors", false},
	{"PutMCPConnector", http.MethodPut, "/v1/work/mcp-connectors", false},
	{"DeleteMCPConnector", http.MethodDelete, "/v1/work/mcp-connectors", false},
	{"GetTeamAgentPolicy", http.MethodGet, "/v1/teams/{teamId}/work/agent-policy", false},
	{"PutTeamAgentPolicy", http.MethodPut, "/v1/teams/{teamId}/work/agent-policy", false},
}

// workIntakeHubOnly: methods the control plane or a test calls; the relay
// never mounts these (decision 2).
var workIntakeHubOnly = []WorkRoute{
	{"CreateRequest", http.MethodPost, "/v1/teams/{teamId}/requests", true},
	{"EraseRequest", http.MethodPost, "/v1/requests/{requestId}/erase", false},
	{"IntakeSettings", http.MethodGet, "/v1/teams/{teamId}/intake-address", false},
	{"PutIntakeSettings", http.MethodPut, "/v1/teams/{teamId}/intake-address", false},
	{"RecordReleaseIssues", http.MethodPut, "/v1/releases/{releaseId}/issues", false},
	{"ListOutsidePRLinks", http.MethodGet, "/v1/projects/{projectId}/outside-links", false},
	{"CreateHostHook", http.MethodPut, "/v1/projects/{projectId}/hooks/{host}", false},
	{"BindTeams", http.MethodPut, "/v1/teams/{teamId}/teams-bindings", false},
	{"ListTeamsBindings", http.MethodGet, "/v1/teams/{teamId}/teams-bindings", false},
	{"UnbindTeams", http.MethodPost, "/v1/teams/{teamId}/teams-bindings/remove", false},
	{"RegisterApp", http.MethodPost, "/v1/apps", true},
}
```

**Counted in the block:** 44 relayed rows (20 of them `GET`s) and 11 hub-only rows serve 53 of
the 55 methods below; `ApplySyncBatch` and `RequestMessages` have no row (decision 4). Five notes:

- **Four methods are added to `WorkIntake`**, because a 171-172 surface calls a route task 143's
  interface lacks (169's G1): `ListTrackerLinks` (150), `ProjectLoops`, `GetLoop` and
  `SetLoopState` (162). They are appended to 143's `hubclient/work_intake.go` ⚑ and to task 144's
  fake ⚑ (each with the part's own rule), in this task's commit; 143's count becomes 55.
- **`CreateAPIKey` and `ListAPIKeys` pick their row by `teamID`**: `nil` is `CreatePersonalKey`
  and `MyKeys`, a `tm-` id `CreateTeamKey` and `TeamKeys` (`task-143.md:885-886`).
- **`CreateLoop` passes its id as `projectId`.** Task 143 names the parameter `teamID`
  (`task-143.md:874`); task 162's route is per project (`task-162.md:438`). The route wins; the
  parameter is renamed `projectID` in 143's interface ⚑ (gap 2).
- **`ListOutsidePRLinks` passes its id as `projectId`** (154's route, `task-154.md:325`); 143's
  parameter is `issueID` (`task-143.md:849`), renamed the same way (gap 2).
- **`/v1/work/loops/…` relays as `/v1/hub/work/work/loops/…`**: task 16's `Relay()` is mechanical
  (`task-16.md:118-122`), and a special case would be the one row that could drift (169's G7).

```go
// control-plane/internal/hub/hubclient/methods_work.go ⚑ (as task 124 left it)

// AllWorkRoutes: W1 (16), W4 (124), then W5 (168). W2's and W3's tables
// (82, 100) are appended between them at sync, if their parts add one.
func AllWorkRoutes() []WorkRoute {
	all := append([]WorkRoute(nil), WorkRoutes...)
	all = append(all, WorkRoadmapRoutes...)
	return append(all, WorkIntakeRoutes...)
}

// workRoute searches AllWorkRoutes(), then briefTemplateRoutes (124), then
// workIntakeHubOnly (168). A name in two lists panics in the table test.
```

```go
// hubclient/work_intake.go ⚑ (task 143's interface), appended:
	// added by task 168 for 171-172 (169's G1)
	ListTrackerLinks(ctx context.Context, connectionID string, page hubapi.PageRequest) (hubapi.Page[hubapi.ExternalLink], error)
	ProjectLoops(ctx context.Context, projectID string) ([]hubapi.Loop, error)
	GetLoop(ctx context.Context, loopID string) (hubapi.Loop, error)
	SetLoopState(ctx context.Context, loopID string, req hubapi.SetLoopStateRequest) (hubapi.Loop, error)
```

`hubapi.ExternalLink` is task 150's record (`task-150.md:960`); `hubapi.SetLoopStateRequest`
(`{state: "on"|"paused"}`) is task 162's body (`task-162.md:443-444`), its Go name **UNVERIFIED**
until 162's block is read at sync.

#### Contract — the client methods, part 1: requests, intake, trackers

Every method is task 16's shape over `c.work` (`task-16.md:216-223`); a paged list passes
`EncodeQuery(page)` (`task-16.md:776-778`). The types are task 143's and the route parts'.

```go
// control-plane/internal/hub/hubclient/methods_work_intake.go (continued)

var _ WorkIntake = (*Client)(nil)

func (c *Client) CreateRequest(ctx context.Context, teamID string, req hubapi.CreateRequestRequest) (hubapi.Request, error) {
	var out hubapi.Request
	return out, c.work(ctx, "CreateRequest", nil, req, &out, teamID)
}
func (c *Client) ListRequests(ctx context.Context, teamID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Request], error) {
	var out hubapi.Page[hubapi.Request]
	return out, c.work(ctx, "ListRequests", EncodeQuery(page), nil, &out, teamID)
}
func (c *Client) Request(ctx context.Context, requestID string) (hubapi.Request, error) {
	var out hubapi.Request
	return out, c.work(ctx, "Request", nil, nil, &out, requestID)
}
func (c *Client) AcceptRequest(ctx context.Context, requestID string, req hubapi.AcceptRequestRequest) (hubapi.AcceptRequestResponse, error) {
	var out hubapi.AcceptRequestResponse // an IssueRef, no revision to re-hash (decision 5)
	return out, c.work(ctx, "AcceptRequest", nil, req, &out, requestID)
}
func (c *Client) LinkRequest(ctx context.Context, requestID string, req hubapi.LinkRequestRequest) (hubapi.Request, error) {
	var out hubapi.Request
	return out, c.work(ctx, "LinkRequest", nil, req, &out, requestID)
}
func (c *Client) DeclineRequest(ctx context.Context, requestID string, req hubapi.DeclineRequestRequest) (hubapi.Request, error) {
	var out hubapi.Request
	return out, c.work(ctx, "DeclineRequest", nil, req, &out, requestID)
}
func (c *Client) MarkImportant(ctx context.Context, requestID string, req hubapi.SetImportantRequest) (hubapi.Request, error) {
	var out hubapi.Request
	return out, c.work(ctx, "MarkImportant", nil, req, &out, requestID)
}
func (c *Client) EraseRequest(ctx context.Context, requestID string, req hubapi.EraseRequest) (hubapi.RequestEraseResponse, error) {
	var out hubapi.RequestEraseResponse
	return out, c.work(ctx, "EraseRequest", nil, req, &out, requestID)
}
func (c *Client) IssueRequesters(ctx context.Context, issueID string) ([]hubapi.IssueRequester, error) {
	var out struct{ Items []hubapi.IssueRequester `json:"items"` }
	return out.Items, c.work(ctx, "IssueRequesters", nil, nil, &out, issueID)
}
func (c *Client) RequestMessages(context.Context, string) ([]hubapi.RequestMessage, error) {
	return nil, ErrNoRoute // task 157 registers no route for the thread (decision 4)
}
func (c *Client) ListIntakeRules(ctx context.Context, teamID string) (hubapi.IntakeRuleSet, error) {
	var out hubapi.IntakeRuleSet
	return out, c.work(ctx, "ListIntakeRules", nil, nil, &out, teamID)
}
func (c *Client) PutIntakeRules(ctx context.Context, teamID string, req hubapi.PutIntakeRulesRequest) (hubapi.IntakeRuleSet, error) {
	var out hubapi.IntakeRuleSet
	return out, c.work(ctx, "PutIntakeRules", nil, req, &out, teamID)
}
func (c *Client) CreateForm(ctx context.Context, teamID string, req hubapi.CreateFormRequest) (hubapi.IntakeForm, error) {
	var out hubapi.IntakeForm
	return out, c.work(ctx, "CreateForm", nil, req, &out, teamID)
}
func (c *Client) ListForms(ctx context.Context, teamID string) ([]hubapi.IntakeForm, error) {
	var out struct{ Items []hubapi.IntakeForm `json:"items"` }
	return out.Items, c.work(ctx, "ListForms", nil, nil, &out, teamID)
}
func (c *Client) GetForm(ctx context.Context, formID string) (hubapi.IntakeForm, error) {
	var out hubapi.IntakeForm
	return out, c.work(ctx, "GetForm", nil, nil, &out, formID)
}
func (c *Client) PatchForm(ctx context.Context, formID string, req hubapi.PatchFormRequest) (hubapi.IntakeForm, error) {
	var out hubapi.IntakeForm
	return out, c.work(ctx, "PatchForm", nil, req, &out, formID)
}
```

#### Contract — the client methods, part 2: address, trackers, hosts, webhooks, Teams

```go
func (c *Client) IntakeSettings(ctx context.Context, teamID string) (hubapi.IntakeSettings, error) {
	var out hubapi.IntakeSettings
	return out, c.work(ctx, "IntakeSettings", nil, nil, &out, teamID)
}
func (c *Client) PutIntakeSettings(ctx context.Context, teamID string, req hubapi.PutIntakeSettingsRequest) (hubapi.IntakeSettings, error) {
	var out hubapi.IntakeSettings
	return out, c.work(ctx, "PutIntakeSettings", nil, req, &out, teamID)
}
func (c *Client) RecordReleaseIssues(ctx context.Context, releaseID string, req hubapi.RecordReleaseIssuesRequest) error {
	return c.work(ctx, "RecordReleaseIssues", nil, req, nil, releaseID)
}
func (c *Client) ConnectTracker(ctx context.Context, projectID string, req hubapi.ConnectTrackerRequest) (hubapi.TrackerConnection, error) {
	var out hubapi.TrackerConnection
	return out, c.work(ctx, "ConnectTracker", nil, req, &out, projectID)
}
func (c *Client) ListTrackers(ctx context.Context, teamID string) ([]hubapi.TrackerConnection, error) {
	var out struct{ Items []hubapi.TrackerConnection `json:"items"` }
	return out.Items, c.work(ctx, "ListTrackers", nil, nil, &out, teamID)
}
func (c *Client) DisconnectTracker(ctx context.Context, connectionID string) (hubapi.TrackerConnection, error) {
	var out hubapi.TrackerConnection
	return out, c.work(ctx, "DisconnectTracker", nil, nil, &out, connectionID)
}
func (c *Client) ApplySyncBatch(context.Context, string, hubapi.SyncBatch) (hubapi.SyncRun, error) {
	return hubapi.SyncRun{}, ErrNoRoute // the Hub's own process applies batches (decision 4)
}
func (c *Client) ListSyncRuns(ctx context.Context, connectionID string, page hubapi.PageRequest) (hubapi.Page[hubapi.SyncRun], error) {
	var out hubapi.Page[hubapi.SyncRun]
	return out, c.work(ctx, "ListSyncRuns", EncodeQuery(page), nil, &out, connectionID)
}
func (c *Client) ListTrackerLinks(ctx context.Context, connectionID string, page hubapi.PageRequest) (hubapi.Page[hubapi.ExternalLink], error) {
	var out hubapi.Page[hubapi.ExternalLink]
	return out, c.work(ctx, "ListTrackerLinks", EncodeQuery(page), nil, &out, connectionID)
}
func (c *Client) ListOutsidePRLinks(ctx context.Context, projectID string) ([]hubapi.OutsidePRLink, error) {
	var out struct{ Items []hubapi.OutsidePRLink `json:"items"` }
	return out.Items, c.work(ctx, "ListOutsidePRLinks", nil, nil, &out, projectID)
}
func (c *Client) CreateHostHook(ctx context.Context, projectID string, host hubapi.HostKind) (hubapi.CreatedHostHook, error) {
	var out hubapi.CreatedHostHook
	return out, c.work(ctx, "CreateHostHook", nil, struct{}{}, &out, projectID, string(host))
}
func (c *Client) CreateWebhook(ctx context.Context, teamID string, req hubapi.CreateWebhookRequest) (hubapi.CreatedWebhook, error) {
	var out hubapi.CreatedWebhook
	return out, c.work(ctx, "CreateWebhook", nil, req, &out, teamID)
}
func (c *Client) ListWebhooks(ctx context.Context, teamID string) ([]hubapi.Webhook, error) {
	var out struct{ Items []hubapi.Webhook `json:"items"` }
	return out.Items, c.work(ctx, "ListWebhooks", nil, nil, &out, teamID)
}
func (c *Client) DisableWebhook(ctx context.Context, webhookID string) (hubapi.Webhook, error) {
	var out hubapi.Webhook
	return out, c.work(ctx, "DisableWebhook", nil, struct{}{}, &out, webhookID)
}
func (c *Client) RotateWebhookSecret(ctx context.Context, webhookID string) (hubapi.CreatedWebhook, error) {
	var out hubapi.CreatedWebhook
	return out, c.work(ctx, "RotateWebhookSecret", nil, struct{}{}, &out, webhookID)
}
func (c *Client) ListDeliveries(ctx context.Context, webhookID string, page hubapi.PageRequest) (hubapi.Page[hubapi.WebhookDelivery], error) {
	var out hubapi.Page[hubapi.WebhookDelivery]
	return out, c.work(ctx, "ListDeliveries", EncodeQuery(page), nil, &out, webhookID)
}
func (c *Client) Redeliver(ctx context.Context, deliveryID string) (hubapi.WebhookDelivery, error) {
	var out hubapi.WebhookDelivery
	return out, c.work(ctx, "Redeliver", nil, struct{}{}, &out, deliveryID)
}
func (c *Client) BindTeams(ctx context.Context, teamID string, b hubapi.TeamsBinding) (hubapi.TeamsBinding, error) {
	var out hubapi.TeamsBinding
	return out, c.work(ctx, "BindTeams", nil, b, &out, teamID)
}
func (c *Client) ListTeamsBindings(ctx context.Context, teamID string) ([]hubapi.TeamsBinding, error) {
	var out struct{ Items []hubapi.TeamsBinding `json:"items"` }
	return out.Items, c.work(ctx, "ListTeamsBindings", nil, nil, &out, teamID)
}
func (c *Client) UnbindTeams(ctx context.Context, teamID string, subject hubapi.TeamsSubject) error {
	return c.work(ctx, "UnbindTeams", nil, subject, nil, teamID)
}
```

A body-less `POST` sends `{}` (`struct{}{}`), because the Hub decodes every write strictly
(contract §1) and an empty body is a 400 there. A list answers `{"items": [...]}`, the Hub's list
shape (hub tree `task-35.md:101`); a route part that answers a bare array is read at step 1 and
this method follows it.

#### Contract — the client methods, part 3: apps, keys, connectors, loops, the agent policy

```go
func (c *Client) RegisterApp(ctx context.Context, m hubapi.AppManifest) (hubapi.App, error) {
	var out hubapi.App
	return out, c.work(ctx, "RegisterApp", nil, m, &out)
}
func (c *Client) ListApps(ctx context.Context) ([]hubapi.App, error) {
	var out struct{ Items []hubapi.App `json:"items"` }
	return out.Items, c.work(ctx, "ListApps", nil, nil, &out)
}
func (c *Client) DecideApp(ctx context.Context, appID string, req hubapi.DecideAppRequest) (hubapi.App, error) {
	var out hubapi.App
	return out, c.work(ctx, "DecideApp", nil, req, &out, appID)
}
func (c *Client) CreateAPIKey(ctx context.Context, teamID *string, req hubapi.CreateAPIKeyRequest) (hubapi.CreatedAPIKey, error) {
	var out hubapi.CreatedAPIKey
	if teamID == nil {
		return out, c.work(ctx, "CreatePersonalKey", nil, req, &out)
	}
	return out, c.work(ctx, "CreateTeamKey", nil, req, &out, *teamID)
}
func (c *Client) ListAPIKeys(ctx context.Context, teamID *string) ([]hubapi.APIKey, error) {
	var out hubapi.Page[hubapi.APIKey] // task 160 pages its lists (task-160.md:387-388)
	if teamID == nil {
		return out.Items, c.work(ctx, "MyKeys", nil, nil, &out)
	}
	return out.Items, c.work(ctx, "TeamKeys", nil, nil, &out, *teamID)
}
func (c *Client) RevokeAPIKey(ctx context.Context, keyID string) (hubapi.APIKey, error) {
	var out hubapi.APIKey
	return out, c.work(ctx, "RevokeAPIKey", nil, struct{}{}, &out, keyID)
}
func (c *Client) ListMCPConnectors(ctx context.Context) ([]hubapi.MCPConnector, error) {
	var out struct{ Items []hubapi.MCPConnector `json:"items"` }
	return out.Items, c.work(ctx, "ListMCPConnectors", nil, nil, &out)
}
func (c *Client) PutMCPConnector(ctx context.Context, req hubapi.PutConnectorRequest) (hubapi.MCPConnector, error) {
	var out hubapi.MCPConnector
	return out, c.work(ctx, "PutMCPConnector", nil, req, &out)
}
func (c *Client) DeleteMCPConnector(ctx context.Context, u string) error {
	return c.work(ctx, "DeleteMCPConnector", url.Values{"url": {u}}, nil, nil) // 161's ?url=
}
func (c *Client) CreateLoop(ctx context.Context, projectID string, req hubapi.CreateLoopRequest) (hubapi.Loop, error) {
	var out hubapi.Loop
	return out, c.work(ctx, "CreateLoop", nil, req, &out, projectID)
}
func (c *Client) ProjectLoops(ctx context.Context, projectID string) ([]hubapi.Loop, error) {
	var out struct{ Items []hubapi.Loop `json:"items"` }
	return out.Items, c.work(ctx, "ProjectLoops", nil, nil, &out, projectID)
}
func (c *Client) GetLoop(ctx context.Context, loopID string) (hubapi.Loop, error) {
	var out hubapi.Loop
	return out, c.work(ctx, "GetLoop", nil, nil, &out, loopID)
}
func (c *Client) SetLoopState(ctx context.Context, loopID string, req hubapi.SetLoopStateRequest) (hubapi.Loop, error) {
	var out hubapi.Loop
	return out, c.work(ctx, "SetLoopState", nil, req, &out, loopID)
}
func (c *Client) PutLoopVersion(ctx context.Context, loopID string, body hubapi.LoopVersionBody) (hubapi.LoopVersion, error) {
	var out hubapi.LoopVersion
	return out, c.work(ctx, "PutLoopVersion", nil, body, &out, loopID)
}
func (c *Client) ListLoopVersions(ctx context.Context, loopID string) ([]hubapi.LoopVersion, error) {
	var out struct{ Items []hubapi.LoopVersion `json:"items"` }
	return out.Items, c.work(ctx, "ListLoopVersions", nil, nil, &out, loopID)
}
func (c *Client) ListLoopRuns(ctx context.Context, loopID string, page hubapi.PageRequest) (hubapi.Page[hubapi.LoopRun], error) {
	var out hubapi.Page[hubapi.LoopRun]
	return out, c.work(ctx, "ListLoopRuns", EncodeQuery(page), nil, &out, loopID)
}
func (c *Client) RunLoopNow(ctx context.Context, loopID string) (hubapi.LoopRun, error) {
	var out hubapi.LoopRun // the request id is the Idempotency-Key (task-169.md:355-356)
	body := hubapi.RunLoopNowRequest{RequestID: IdempotencyKeyOf(ctx)} // task 163 requires it (task-163.md:124-126)
	return out, c.work(ctx, "RunLoopNow", nil, body, &out, loopID)
}
func (c *Client) GetTeamAgentPolicy(ctx context.Context, teamID string) (hubapi.TeamAgentPolicy, error) {
	var out hubapi.TeamAgentPolicy
	return out, c.work(ctx, "GetTeamAgentPolicy", nil, nil, &out, teamID)
}
func (c *Client) PutTeamAgentPolicy(ctx context.Context, teamID string, p hubapi.TeamAgentPolicy) (hubapi.TeamAgentPolicy, error) {
	var out hubapi.TeamAgentPolicy // the Hub stamps the rest; strict decoding refuses them (task 166)
	body := hubapi.PutTeamAgentPolicyRequest{Harnesses: p.Harnesses, Skills: p.Skills}
	return out, c.work(ctx, "PutTeamAgentPolicy", nil, body, &out, teamID)
}
```

`ListAPIKeys` returns the first page only, because task 143's signature takes no page. A person
has at most 10 live keys and a team 25 (`task-160.md:381`, `:385`), but revoked and expired keys
are listed too (`:391`), and task 160's default `limit` is not stated: whether one page holds them
all is **UNVERIFIED** (gap 5). `RunLoopNow`'s body is task 163's `{requestId}`: the value is the
context's idempotency key (`IdempotencyKeyOf`, beside `WithIdempotencyKey`; H33's own fresh key
when none is set, whose fit to 163's 8-64 `[A-Za-z0-9._-]` is **UNVERIFIED**, gap 6), so the desktop's one op id is both the header and the body (169), and a
relay retry is 163's `200` with the first run.

#### Contract — the relay: `api/work.go` ⚑, `intakeCalls()` (complete)

`registerWork` already iterates `AllWorkRoutes()` (task 124), so the 44 rows mount once
`workCalls()` merges `intakeCalls()`; a row with no handler, or a handler with no row, panics at
`New` (`task-16.md:520-535`). The hub-only rows have no handler and are never mounted.

```go
// control-plane/internal/api/work.go ⚑ (continued)

type (
	cx = context.Context
	pv = func(string) string
)

// in0: path values and the query, no body. inb: a strict body of T first
// (not in1: task 124's in1, task-124.md:518-527, is in this package already).
// paged: the query decoded as a page (hubclient.DecodeQuery, task 16).
func in0(f func(c cx, w hubclient.Work, p pv, q url.Values) (any, error)) workCall {
	return func(r *http.Request, _ http.ResponseWriter, w hubclient.Work) (any, error) {
		return f(r.Context(), w, r.PathValue, r.URL.Query())
	}
}

func inb[T any](f func(c cx, w hubclient.Work, p pv, b T) (any, error)) workCall {
	return func(r *http.Request, rw http.ResponseWriter, w hubclient.Work) (any, error) {
		b, err := body[T](rw, r)
		if err != nil {
			return nil, err
		}
		return f(hubclient.WithIdempotencyKey(r.Context(), r.Header.Get("Idempotency-Key")), w, r.PathValue, b)
	}
}

func paged(f func(c cx, w hubclient.Work, p pv, pg hubapi.PageRequest) (any, error)) workCall {
	return in0(func(c cx, w hubclient.Work, p pv, q url.Values) (any, error) {
		pg, err := hubclient.DecodeQuery[hubapi.PageRequest](q)
		if err != nil {
			return nil, err // a *FieldError: relayWork answers 400
		}
		return f(c, w, p, pg)
	})
}

// empty is the body of a write that takes none: the relay still reads it
// strictly, so `{"x":1}` is 400 here and never reaches the Hub.
type empty struct{}

func ptr(s string) *string { return &s }
```

`hubclient.WithIdempotencyKey(ctx, "")` returns `ctx` unchanged, so a write with no header gets
H33's own key, as every W1 write does today (decision 7).

**A slice answer is `{"items": […]}`**, as task 124's `ProjectGrants` (`task-124.md:542-545`);
a page is `hubapi.Page[T]` as the Hub sent it. `items` wraps a `nil` slice as `[]`, so the desktop
never branches on `null`.

```go
// control-plane/internal/api/work.go ⚑ (continued) — the W5 handlers, part 1

func items[T any](xs []T, err error) (any, error) {
	if xs == nil {
		xs = []T{}
	}
	return map[string]any{"items": xs}, err
}

// intakeCalls is the handler of every WorkIntakeRoutes row, by the row's name.
func intakeCalls() map[string]workCall {
	return map[string]workCall{
		// 146: requests
		"ListRequests": paged(func(c cx, w hubclient.Work, p pv, pg hubapi.PageRequest) (any, error) {
			return w.ListRequests(c, p("teamId"), pg)
		}),
		"Request": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return w.Request(c, p("requestId"))
		}),
		"AcceptRequest": inb(func(c cx, w hubclient.Work, p pv, b hubapi.AcceptRequestRequest) (any, error) {
			return w.AcceptRequest(c, p("requestId"), b)
		}),
		"LinkRequest": inb(func(c cx, w hubclient.Work, p pv, b hubapi.LinkRequestRequest) (any, error) {
			return w.LinkRequest(c, p("requestId"), b)
		}),
		"DeclineRequest": inb(func(c cx, w hubclient.Work, p pv, b hubapi.DeclineRequestRequest) (any, error) {
			return w.DeclineRequest(c, p("requestId"), b)
		}),
		"MarkImportant": inb(func(c cx, w hubclient.Work, p pv, b hubapi.SetImportantRequest) (any, error) {
			return w.MarkImportant(c, p("requestId"), b)
		}),
		"IssueRequesters": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return items(w.IssueRequesters(c, p("issueId")))
		}),
		// 147-148: rules and forms
		"ListIntakeRules": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return w.ListIntakeRules(c, p("teamId"))
		}),
		"PutIntakeRules": inb(func(c cx, w hubclient.Work, p pv, b hubapi.PutIntakeRulesRequest) (any, error) {
			return w.PutIntakeRules(c, p("teamId"), b)
		}),
		"CreateForm": inb(func(c cx, w hubclient.Work, p pv, b hubapi.CreateFormRequest) (any, error) {
			return w.CreateForm(c, p("teamId"), b)
		}),
		"ListForms": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return items(w.ListForms(c, p("teamId")))
		}),
		"GetForm": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return w.GetForm(c, p("formId"))
		}),
		"PatchForm": inb(func(c cx, w hubclient.Work, p pv, b hubapi.PatchFormRequest) (any, error) {
			return w.PatchForm(c, p("formId"), b)
		}),
		// 150-153: trackers
		"ConnectTracker": inb(func(c cx, w hubclient.Work, p pv, b hubapi.ConnectTrackerRequest) (any, error) {
			return w.ConnectTracker(c, p("projectId"), b)
		}),
		"ListTrackers": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return items(w.ListTrackers(c, p("teamId")))
		}),
		"DisconnectTracker": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return w.DisconnectTracker(c, p("connectionId")) // a DELETE: no body is read
		}),
		"ListSyncRuns": paged(func(c cx, w hubclient.Work, p pv, pg hubapi.PageRequest) (any, error) {
			return w.ListSyncRuns(c, p("connectionId"), pg)
		}),
		"ListTrackerLinks": paged(func(c cx, w hubclient.Work, p pv, pg hubapi.PageRequest) (any, error) {
			return w.ListTrackerLinks(c, p("connectionId"), pg)
		}),
```
