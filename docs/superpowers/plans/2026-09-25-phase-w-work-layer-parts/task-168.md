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

```go
// control-plane/internal/api/work.go ⚑ (continued) — part 2, inside intakeCalls' map
		// 162-163: loops
		"CreateLoop": inb(func(c cx, w hubclient.Work, p pv, b hubapi.CreateLoopRequest) (any, error) {
			return w.CreateLoop(c, p("projectId"), b)
		}),
		"ProjectLoops": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return items(w.ProjectLoops(c, p("projectId")))
		}),
		"GetLoop": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return w.GetLoop(c, p("loopId"))
		}),
		"SetLoopState": inb(func(c cx, w hubclient.Work, p pv, b hubapi.SetLoopStateRequest) (any, error) {
			return w.SetLoopState(c, p("loopId"), b)
		}),
		"PutLoopVersion": inb(func(c cx, w hubclient.Work, p pv, b hubapi.LoopVersionBody) (any, error) {
			return w.PutLoopVersion(c, p("loopId"), b)
		}),
		"ListLoopVersions": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return items(w.ListLoopVersions(c, p("loopId")))
		}),
		"ListLoopRuns": paged(func(c cx, w hubclient.Work, p pv, pg hubapi.PageRequest) (any, error) {
			return w.ListLoopRuns(c, p("loopId"), pg)
		}),
		// The body's requestId is the key; the desktop sends the header with the same value (169).
		"RunLoopNow": inb(func(c cx, w hubclient.Work, p pv, b hubapi.RunLoopNowRequest) (any, error) {
			return w.RunLoopNow(hubclient.WithIdempotencyKey(c, b.RequestID), p("loopId"))
		}),
		// 159-160: apps and keys
		"ListApps": in0(func(c cx, w hubclient.Work, _ pv, _ url.Values) (any, error) {
			return items(w.ListApps(c))
		}),
		"DecideApp": inb(func(c cx, w hubclient.Work, p pv, b hubapi.DecideAppRequest) (any, error) {
			return w.DecideApp(c, p("appId"), b)
		}),
		"CreatePersonalKey": inb(func(c cx, w hubclient.Work, _ pv, b hubapi.CreateAPIKeyRequest) (any, error) {
			return w.CreateAPIKey(c, nil, b) // 201 CreatedAPIKey: the secret, once (decision 6)
		}),
		"CreateTeamKey": inb(func(c cx, w hubclient.Work, p pv, b hubapi.CreateAPIKeyRequest) (any, error) {
			return w.CreateAPIKey(c, ptr(p("teamId")), b)
		}),
		"MyKeys": in0(func(c cx, w hubclient.Work, _ pv, _ url.Values) (any, error) {
			return items(w.ListAPIKeys(c, nil))
		}),
		"TeamKeys": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return items(w.ListAPIKeys(c, ptr(p("teamId"))))
		}),
		"RevokeAPIKey": inb(func(c cx, w hubclient.Work, p pv, _ empty) (any, error) {
			return w.RevokeAPIKey(c, p("keyId"))
		}),
```

```go
// control-plane/internal/api/work.go ⚑ (continued) — part 3, the rest of the map
		// 156: webhooks
		"CreateWebhook": inb(func(c cx, w hubclient.Work, p pv, b hubapi.CreateWebhookRequest) (any, error) {
			return w.CreateWebhook(c, p("teamId"), b) // 201 CreatedWebhook: the secret, once
		}),
		"ListWebhooks": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return items(w.ListWebhooks(c, p("teamId")))
		}),
		"DisableWebhook": inb(func(c cx, w hubclient.Work, p pv, _ empty) (any, error) {
			return w.DisableWebhook(c, p("webhookId"))
		}),
		"RotateWebhookSecret": inb(func(c cx, w hubclient.Work, p pv, _ empty) (any, error) {
			return w.RotateWebhookSecret(c, p("webhookId"))
		}),
		"ListDeliveries": paged(func(c cx, w hubclient.Work, p pv, pg hubapi.PageRequest) (any, error) {
			return w.ListDeliveries(c, p("webhookId"), pg)
		}),
		"Redeliver": inb(func(c cx, w hubclient.Work, p pv, _ empty) (any, error) {
			return w.Redeliver(c, p("deliveryId"))
		}),
		// 161: MCP connectors; the DELETE names its connector by ?url=
		"ListMCPConnectors": in0(func(c cx, w hubclient.Work, _ pv, _ url.Values) (any, error) {
			return items(w.ListMCPConnectors(c))
		}),
		"PutMCPConnector": inb(func(c cx, w hubclient.Work, _ pv, b hubapi.PutConnectorRequest) (any, error) {
			return w.PutMCPConnector(c, b)
		}),
		"DeleteMCPConnector": in0(func(c cx, w hubclient.Work, _ pv, q url.Values) (any, error) {
			u := q.Get("url")
			if u == "" || len(q) != 1 {
				return nil, &hubclient.FieldError{Field: "url", Reason: "exactly one url is required"}
			}
			return emptyAnswer, w.DeleteMCPConnector(c, u) // task 124's emptyAnswer
		}),
		// 166: the team's agent policy
		"GetTeamAgentPolicy": in0(func(c cx, w hubclient.Work, p pv, _ url.Values) (any, error) {
			return w.GetTeamAgentPolicy(c, p("teamId"))
		}),
		"PutTeamAgentPolicy": inb(func(c cx, w hubclient.Work, p pv, b hubapi.PutTeamAgentPolicyRequest) (any, error) {
			return w.PutTeamAgentPolicy(c, p("teamId"), hubapi.TeamAgentPolicy{Harnesses: b.Harnesses, Skills: b.Skills})
		}),
	}
}
```

```go
// control-plane/internal/api/work.go ⚑ — in registerWork, after task 124's merge loop
	for name, call := range intakeCalls() {
		if _, dup := calls[name]; dup {
			panic("api: relay handler " + name + " defined twice")
		}
		calls[name] = call
	}
```

**Counted:** the map holds 44 entries, one per `WorkIntakeRoutes` row; `registerWork`'s existing
panics (task 16) catch a 43 or a 45. `DeleteMCPConnector` refuses `?url=a&url=b` and any second
parameter before the Hub is called, so one relay call removes at most one connector. The strict
`inb` on the four no-body `POST`s (`RevokeAPIKey`, `DisableWebhook`, `RotateWebhookSecret`,
`Redeliver`) means `{}` is accepted and `{"x":1}` is 400. **No body at all is also 400**: task
16's `body` (`task-16.md:567`) is `httpx.DecodeJSON`, whose first `Decode` returns `io.EOF` on an
empty body and answers it (`httpx/problem.go:130-133`, read 2026-09-26). The desktop's `postJSON`
for these four must send `{}` (task 169's, gap 8).

#### Contract — the route audit (complete)

Twenty W5 relay routes are `GET`s. Each gets one line in `catalogReadOnlyRoutes`
(`routes_audit_test.go:32`) under task 16's `workRelayRead` reason (`task-16.md:789`), and task
124's `TestW4RelayPathsAreExactlyTheTable` (`task-124.md:702-721`) covers the W5 rows unchanged,
because it walks `AllWorkRoutes()`. One W5 test adds the counts and the not-relayed paths.

```go
// control-plane/internal/api/routes_audit_test.go ⚑ — appended to catalogReadOnlyRoutes
	"GET /v1/hub/work/teams/{teamId}/requests":             workRelayRead,
	"GET /v1/hub/work/requests/{requestId}":                workRelayRead,
	"GET /v1/hub/work/issues/{issueId}/requests":           workRelayRead,
	"GET /v1/hub/work/teams/{teamId}/intake-rules":         workRelayRead,
	"GET /v1/hub/work/teams/{teamId}/forms":                workRelayRead,
	"GET /v1/hub/work/forms/{formId}":                      workRelayRead,
	"GET /v1/hub/work/teams/{teamId}/trackers":             workRelayRead,
	"GET /v1/hub/work/trackers/{connectionId}/runs":        workRelayRead,
	"GET /v1/hub/work/trackers/{connectionId}/links":       workRelayRead,
	"GET /v1/hub/work/projects/{projectId}/work/loops":     workRelayRead,
	"GET /v1/hub/work/work/loops/{loopId}":                 workRelayRead,
	"GET /v1/hub/work/work/loops/{loopId}/versions":        workRelayRead,
	"GET /v1/hub/work/work/loops/{loopId}/runs":            workRelayRead,
	"GET /v1/hub/work/apps":                                workRelayRead,
	"GET /v1/hub/work/me/api-keys":                         workRelayRead,
	"GET /v1/hub/work/teams/{teamId}/api-keys":             workRelayRead,
	"GET /v1/hub/work/teams/{teamId}/webhooks":             workRelayRead,
	"GET /v1/hub/work/webhooks/{webhookId}/deliveries":     workRelayRead,
	"GET /v1/hub/work/work/mcp-connectors":                 workRelayRead,
	"GET /v1/hub/work/teams/{teamId}/work/agent-policy":    workRelayRead,

// TestW5RelayCountsAndExclusions: 44 relayed rows (20 GETs), 11 hub-only rows
// never mounted, and no pattern for a Hub-facing ingress (decision 3).
func TestW5RelayCountsAndExclusions(t *testing.T) {
	gets := 0
	for _, rt := range hubclient.WorkIntakeRoutes {
		if rt.Verb == http.MethodGet { // WorkRoute: Method is the name, Verb the verb (task-16.md:111-116)
			gets++
		}
	}
	if n := len(hubclient.WorkIntakeRoutes); n != 44 || gets != 20 {
		t.Errorf("WorkIntakeRoutes: %d rows, %d GETs; 146-166 fix 44 and 20", n, gets)
	}
	got := registeredPatterns(t) // task 124's helper
	for _, rt := range hubclient.WorkIntakeHubOnlyForTest() {
		if got[rt.Relay()] {
			t.Errorf("hub-only %s is mounted as %q", rt.Method, rt.Relay())
		}
	}
	for p := range got {
		for _, bad := range []string{"/v1/hub/work/hooks/", "/v1/hub/work/teams/messages", "/mcp",
			"/v1/hub/work/forms/{formId}/submit", "/sentinel"} {
			if strings.Contains(p, bad) {
				t.Errorf("%q relays a Hub-facing ingress (%s)", p, bad)
			}
		}
	}
}
```

`WorkIntakeHubOnlyForTest` is `hubclient`'s `export_test`-style accessor for the unexported
list; since `api`'s test is another package, it is an exported function in a non-test file,
named so no product caller mistakes it for a relay list (gap 9 offers the alternative). The
Sentinel exporter is outbound only (167) and has no Hub route at all, so the `/sentinel` guard
is belt and braces. **Mutants killed:** a hub-only row appended to `WorkIntakeRoutes` (a
`CreateRequest` relay); a hand-mounted `/v1/hub/work/hooks/{host}`; a 45th row with no surface.

#### Interfaces — complete: `control-plane/internal/api/agentpolicy.go` (decision 9)

```go
// control-plane/internal/api/agentpolicy.go
package api

import (
	"context"
	"fmt"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hublink"
)

// projectLinker is the one hublink.Resolver method the source uses
// (hub tree task-35.md:73-77); a test double is four lines.
type projectLinker interface {
	Project(ctx context.Context, localProjectID string) (hublink.Link, error)
}

// policyReader is the one hubclient method it uses (this task's, above).
type policyReader interface {
	GetTeamAgentPolicy(ctx context.Context, teamID string) (hubapi.TeamAgentPolicy, error)
}

// HubPolicySource is task 166's AgentPolicySource over the Hub: the local
// project's Hub project, that project's team, the team's policy.
type HubPolicySource struct {
	Links  projectLinker
	Policy policyReader
}

var _ AgentPolicySource = HubPolicySource{}

// PolicyFor fails closed: a link or policy error is returned, and task 166's
// gate answers 503 agent_policy_unavailable and starts no session. Only a
// project that is not Linked (Unlinked, NoRemote) is ungated.
func (s HubPolicySource) PolicyFor(ctx context.Context, projectID string) (*hubapi.TeamAgentPolicy, bool, error) {
	link, err := s.Links.Project(ctx, projectID)
	if err != nil {
		return nil, false, fmt.Errorf("agent policy: link %s: %w", projectID, err)
	}
	if link.State != hublink.Linked || link.Project == nil {
		return nil, false, nil // bound to no Hub team (task 166, decision 5)
	}
	team := link.Project.TeamID // hubapi.Project.TeamID, hub tree task-03.md:156
	p, err := s.Policy.GetTeamAgentPolicy(ctx, team)
	if err != nil {
		return nil, true, fmt.Errorf("agent policy: team %s: %w", team, err)
	}
	return &p, true, nil
}
```

```go
// control-plane/cmd/api/main.go ⚑ — after the Hub client and H35's resolver are built
	var agentPolicy api.AgentPolicySource // nil: no gate (OD-20, a machine with no Hub)
	if hubClient != nil && hubLinks != nil {
		agentPolicy = api.HubPolicySource{Links: hubLinks, Policy: hubClient}
	}
// … and in the api.Deps literal (main.go:132 on 2026-09-26; task 16 cites :147):
		AgentPolicy: agentPolicy,
```

`hubClient` and `hubLinks` are the names this part gives H32's client and H35's resolver in
`main.go`; both are **UNVERIFIED** until the H-plan merges (gap 10). The interface variable is
assigned only when both exist, for task 16's reason (a nil pointer in a non-nil interface).
The resolver's five-minute cache (H35) means a policy is read at most once per session start
per team, and the policy itself is never cached here: an admin's `PUT` takes effect at the next
start. **Fail closed is a choice** this part makes for 166: a person signed out of the Hub, on a
machine with `PANACEA_APP_HUB_URL` set, cannot start a session on a project with a GitHub or
Azure DevOps remote until they sign in (gap 11 asks the owner).

#### Tests to write first, part 1 — `hubclient/methods_work_intake_test.go` (the table, no server)

```go
// control-plane/internal/hub/hubclient/methods_work_intake_test.go
// 1. Every route in both lists is well formed and unique.
//    for each r in append(WorkIntakeRoutes, workIntakeHubOnly...):
//      r.Method is one of GET PUT POST PATCH DELETE; r.Pattern begins "/v1/"; no duplicate
//      (method, pattern) pair; every "{name}" segment is lower camel and non-empty.
//    Mutant it kills: a copied row left with the pattern of the row above it.
// 2. AllWorkRoutes() holds every W1, W2, W4 and W5 relayed pattern and NO hub-only one.
//    want := len(task16Routes) + len(task82Routes) + len(task124Routes) + len(WorkIntakeRoutes)
//    got  := len(AllWorkRoutes()); require.Equal(want, got)
//    require.NotContains(patterns(AllWorkRoutes()), "/v1/work/sync-batches")  // decision 2
//    Mutant: workIntakeHubOnly appended too, so the relay mounts ApplySyncBatch.
// 3. The route parts win over task 143's proposed table (decision 1). One row per correction:
//    routeOf("DecideApp").Pattern == "/v1/work/apps/{appId}/decision"
//    routeOf("DisconnectTracker") == {DELETE, "/v1/trackers/{connectionId}"}
//    routeOf("CreateHostHook") == {PUT, "/v1/projects/{projectId}/hooks/{host}"}
//    routeOf("ListOutsidePRLinks").Pattern == "/v1/projects/{projectId}/outside-prs"
//    routeOf("CreateAPIKey").Pattern == "/v1/work/apps/{appId}/api-keys"
//    routeOf("GetTeamAgentPolicy").Pattern == "/v1/teams/{teamId}/work/agent-policy"
//    Mutant: task 143's proposed path copied, which 404s against the built Hub.
// 4. Host ingress, the Teams endpoint, the public form, /mcp and the exporter have no row
//    in either list (decision 3): for each prefix, workRoute finds nothing.
//    Mutant: /v1/hooks/{host} added "for symmetry", exposing Hub-facing ingress on loopback.
// 5. ApplySyncBatch and RequestMessages over HTTP return ErrNoRoute, and ErrNoRoute is not
//    ErrNotConfigured: errors.Is(err, ErrNoRoute) && !errors.Is(err, ErrNotConfigured).
//    Mutant: ErrNotConfigured returned, so a caller reads "no Hub" and hides the surface.
// 6. client_work_intake_stub.go is gone: a build of the package with the stub still present
//    fails on a duplicate method set, so the test is the compile itself; the step list makes
//    the delete part of the same commit (task-143.md:906-916).
```

#### Tests to write first, part 2 — the conformance rows over HTTP

```go
// control-plane/internal/hub/hubclient/conformance/work_intake_integration_test.go
//go:build integration

// TestWorkRowsW5_HTTP runs task 144's conformance.WorkRowsW5 against the HTTP client, the
// same rows the fake passes (task-144.md:551-596). The harness is task 124's
// (task-124.md:? — the W4 integration file), with three additions:
//   env := hubtest.Start(t, hubtest.WithFakes(mail, host, teams, ingest))
//   cl  := hubclient.New(env.URL, devissuer.Token(t, "u-dylan"), hubclient.WithHTTP(env.HTTP))
//   sync := hubtest.SyncAs(env)        // decision 8: in-process, the sync principal has no token
//   conformance.WorkRowsW5(t, conformance.Env{Client: cl, InProcess: sync, Clock: env.Clock})
//
// Rows that must pass here and are the reason the file exists:
//   W5-02 create-request-twice with one Idempotency-Key returns the same requestId and the
//         Hub writes one row (mutant: the key dropped on the retry, two requests filed).
//   W5-05 accept returns an IssueRef whose number is the team's next (decision 5: no digest).
//   W5-17 a 409 from the Hub arrives as hubclient.Error{Status:409, Code:"conflict"} with the
//         Hub's details map intact (mutant: the client rewrites it to a bare error).
//   W5-22 a second CreateAPIKey with the same name is 409, and the first response's secret is
//         the only place the secret ever appears.
//   W5-31 a loop run row is written once per fire, with its trigger and its result word.
```

Every row is task 144's, unchanged: the point of the file is that one body of rows passes twice,
once against `NewFake()` and once against the Hub. A row that needs a fake host, mail or Teams
takes it from `conformance.Env`, which task 144 already carries; no row here reaches the network.

#### Tests to write first, part 3 — `api/work_intake_test.go` (the relay rules)

```go
// control-plane/internal/api/work_intake_test.go — httptest over the built router, a stub
// hubclient behind it (task 16's pattern, task-16.md:592-640).
// 1. Every WorkIntakeRoutes row is mounted under /v1/hub/work/… and reaches its method once.
//    For each row: call the relay path, assert the stub saw exactly that method name and that
//    the path parameters arrived in order. Mutant: two rows wired to the same method, so
//    *important* marks a request accepted.
// 2. Hub-facing ingress is 404 on the relay, all five:
//    /v1/hub/work/hooks/github, /v1/hub/work/teams/messages, /v1/hub/work/forms/{slug}/submit,
//    /v1/hub/work/mcp/read, /v1/hub/work/sentinel/export → 404, and the stub saw no call.
//    Mutant: a catch-all proxy under /v1/hub/work/, which would publish the Hub's ingress.
// 3. A Hub 409 passes through unchanged: the stub returns hubclient.Error{409, "conflict",
//    details}; the relay's body is that code and those details and the status is 409.
//    Mutant: the relay maps every hubclient.Error to 502.
// 4. A created key's secret crosses once and is never logged: the stub answers
//    CreatedAPIKey{Secret: "pnk_live_TESTSECRET"}; the response body holds it; the captured
//    log (a slog handler into a buffer) does not, under any key, and neither does the access
//    line. Mutant: the relay logs the body on 201 "for support".
// 5. No header of the incoming request is copied to the Hub except the idempotency key
//    (decision 7): the caller sends Authorization: Bearer local-caller-key, X-Panacea-Debug
//    and Idempotency-Key: op-7; the stub's captured outbound headers hold op-7 and neither of
//    the other two. Mutant: a header allow-list that passes Authorization through, which is
//    hub-contract §9.4's local caller key leaving the machine.
// 6. The four bodyless PUTs send {} and not an empty body (task 169's gap 8): MarkImportant,
//    ClearImportant, EnableLoop, DisableLoop. Mutant: nil body, and the Hub's decoder 400s.
// 7. A relay path whose row is hub-only is not mounted: /v1/hub/work/sync-batches → 404.
```

#### Tests to write first, part 4 — the agent policy and the end to end

```go
// control-plane/internal/api/agentpolicy_test.go
// 1. A project with no Hub link: PolicyFor returns (nil, false, nil) and no Hub call.
//    Mutant: an error, which would block every session on an unlinked project.
// 2. A linked project: the team id passed to GetTeamAgentPolicy is link.Project.TeamID.
//    Mutant: the project id passed as the team id, which 404s on every start.
// 3. A Hub error returns (nil, true, err): found is true, so the caller fails closed
//    (decision 9's choice, gap 11). Mutant: (nil, false, nil), which fails open.
//
// control-plane/internal/hub/work_intake_e2e_integration_test.go  //go:build integration
// TestWorkIntakeE2E, one ordered story over one Hub, the fakes and devissuer. Each step
// asserts a fact, and the step after it depends on that fact:
//  1. A non-member posts the public form (148): the request exists, state "new".
//  2. The first matching rule (149) routes it to triage; the event's author is the requester.
//     Mutant: the rule engine writes the sync principal as the author.
//  3. Dylan accepts it into an issue that serves c-91e2 (146): AcceptRequestResponse's
//     IssueRef names the new issue and the request is "accepted", linked to it.
//  4. The issue lands; a release is reserved and confirmed (W4's path); the mail fake holds one
//     message to the requester whose body names the release. Mutant: mailed on land, not
//     on ship, so the requester hears about work that is not out.
//  5. A GitHub Issues item syncs in (150): it lands in triage as "not tied · imported", and a
//     state move attempted as the sync principal is 403. Mutant: the sync principal may move
//     state, so an outside tracker drives the Hub's facts.
//  6. An outside PR whose body says "Fixes EP-142" links (154): the link exists; its merge,
//     delivered by the host fake, writes landed with the mapped person as the author, and an
//     unmapped author writes nothing. Mutant: landed written as the app.
//  7. An outbound webhook fires (156): the delivery's HMAC verifies against the secret, and
//     the payload holds ids, kinds and times and no title, description or comment text.
//     Mutant: the issue's title in the payload, which is text leaving the tenant.
//  8. An approved app reads through /mcp/read (161) and a write attempt is 403.
//  9. A loop bound to accept fires once and logs one run with its result (162-163).
// 10. A key over its limit is 429 with the rate headers set (160).
// 11. The exporter posts the chain to the ingestion fake (167) and the fake's last body's
//     digest chain verifies head to tail.
```

#### Steps

- [ ] Write `methods_work_intake_test.go` tests 1-5, then the table and the 51 methods in
      `methods_work_intake.go` until they pass; delete `client_work_intake_stub.go` in the same
      commit. Mutants, one at a time, then revert: copy the row above (test 1); append
      `workIntakeHubOnly` in `AllWorkRoutes` (test 2); restore task 143's proposed `DecideApp`
      path (test 3); add a `/v1/hooks/{host}` row (test 4); return `ErrNotConfigured` (test 5).
- [ ] Edit `methods_work.go`: `workRoute` searches both lists, `AllWorkRoutes()` appends only
      `WorkIntakeRoutes`. Tasks 16, 82 and 124's suites still pass unchanged.
- [ ] Write `api/work_intake_test.go`, then `intakeCalls()` in `work.go` until the seven pass.
      Mutants: two rows on one method (test 1); a catch-all under `/v1/hub/work/` (test 2);
      map every `hubclient.Error` to 502 (test 3); log the 201 body (test 4); copy
      `Authorization` outbound (test 5); send a nil body on the four PUTs (test 6).
- [ ] Add the W5 relay `GET`s to `catalogReadOnlyRoutes` in `routes_audit_test.go`; the audit
      is green with no other edit.
- [ ] Write `agentpolicy_test.go`, then `agentpolicy.go`, then the two lines in `main.go`.
- [ ] Write `conformance/work_intake_integration_test.go`. Run it against a Hub started by
      `hubtest.Start`. Every W5 row that passed against the fake passes here.
- [ ] Write `work_intake_e2e_integration_test.go`, step by step, each step's assert before the
      next step's call. Commit after each step's assert is green.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'cd control-plane && go test ./internal/hub/hubclient/... ./internal/api/...'
# expected: ok for every package; no build error from the deleted stub
~/.claude/tools/claudezsh/claudezsh 'cd control-plane && go test -tags integration ./internal/hub/... ./internal/hub/hubclient/conformance/...'
# expected: ok, TestWorkRowsW5_HTTP and TestWorkIntakeE2E both PASS
~/.claude/tools/claudezsh/claudezsh 'cd control-plane && go vet ./... && gofmt -l internal/hub internal/api'
# expected: no output
~/.claude/tools/claudezsh/claudezsh 'grep -rn "client_work_intake_stub" control-plane/'
# expected: no output (the stub is deleted, decision in task 143)
~/.claude/tools/claudezsh/claudezsh 'grep -nE "/v1/hooks/|teams/messages|/mcp|sentinel" control-plane/internal/hub/hubclient/methods_work_intake.go'
# expected: no output (decision 3: nothing Hub-facing has a row)
~/.claude/tools/claudezsh/claudezsh 'grep -nE "slog|log\\.|Printf" control-plane/internal/api/work.go'
# expected: no line inside intakeCalls (decision 6: the relay logs no body)
```

#### What this task does NOT claim

- **Any desktop surface.** 169-173 own the wire types, the panels and the admin screens; this
  part stops at the relay's edge.
- **A real host, tenant or workspace.** Every outside system in the end to end is a fake in the
  test binary: GitHub, Azure DevOps, Teams, mail and the Sentinel ingestion endpoint.
- **That the Hub's own routes are correct.** 146-167 each prove their own route; this part
  proves the client reaches them and the relay passes them through.
- **A measured latency.** The end to end records no timing; W5's numbers are task 174's.
- **Rate limiting on the relay.** 429 comes from the Hub's key limiter (160); the control plane
  adds no limiter of its own.
- **That a running session re-reads its managed block after an agent policy changes** (OD-8:
  nothing is typed into a CLI). The policy is read at session start.
- **Parity:** closes no row alone; it is the proof path for the Hub halves of 146-167, recorded
  by task 174.

#### Gaps found while writing this part

1. The skeleton's File map names no W5 end-to-end file and no `agentpolicy.go`; both names here
   follow tasks 17, 82 and 124 and should be added to the map beside them.
2. Task 143's `WorkIntake` interface names some parameters `projectId` and some `issueId` in its
   prose while its Go signatures use `projectID` and `issueID`; this part uses the Go spelling
   throughout. One of the two should be corrected in 143.
3. Task 144's `conformance.WorkRowsW5` takes an `Env` whose `InProcess` member does not exist on
   the tree (no `task-144.md` conformance `Env` printed with it); `hubtest.SyncAs` is this
   part's name for the binding decision 4 needs, and is **UNVERIFIED** until 144 merges.
4. `hubtest.Start` with fakes (`WithFakes`) is this part's assumption about the W5 test harness;
   tasks 154-157 and 167 each build their own fake, and no part owns the shared harness. It
   should be one file, owned by 146.
5. Whether task 163's `RunLoopNow` accepts an empty `{requestId}` at all is **UNVERIFIED**: 163
   prints the field but not its optionality.
6. The idempotency key's carriage into H33's transport (`hubclient.WithIdempotencyKey`) and the
   value's fit to 163's `8-64 [A-Za-z0-9._-]` are both **UNVERIFIED** until the H-plan merges.
   If H33 names it differently, one line in `intakeCalls()` changes.
7. No part states whether a W5 relay `GET` belongs in `catalogReadOnlyRoutes` or in the audit's
   new-route list; this part puts the `GET`s in the read-only catalog, as task 124 did for W4.
8. Task 169's four bodyless PUTs (`{}` not nil) are asserted here and there; the rule belongs in
   the hub contract, §4.3, as one sentence.
9. `AllWorkRoutes()` is named for the relay but now omits a list, which reads wrong. The
   alternative is `RelayedWorkRoutes()` plus `AllWorkRoutes()` meaning all; renaming touches
   tasks 16, 82, 124 and 169, so it is left as a note.
10. `hubClient` and `hubLinks` in `cmd/api/main.go` are this part's names for H32's client and
    H35's resolver; both are **UNVERIFIED** until the H-plan merges.
11. **For the owner:** decision 9 fails closed, so a person signed out of the Hub on a machine
    with `PANACEA_APP_HUB_URL` set cannot start a session on a project with a GitHub or Azure
    DevOps remote. Fail open (start, and warn) is the other choice, and OD-20's local mode is
    the reason to consider it.
