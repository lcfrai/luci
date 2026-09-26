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
6. **The relay copies bytes and logs no body.** A `201 CreatedAPIKey` or `CreatedWebhook` holds
   its secret once (`task-143.md:353-356`, `:444-447`). The relay writes the Hub's bytes through
   and logs method, pattern, status and duration only. `hubclient.Error` passes through with the
   Hub's status, code and details unchanged (hub-contract §4.3).
7. **The local caller key never leaves the machine** (hub-contract §9.4). The relay's Hub call is
   `hubclient`'s, with the person's Entra token (H33); no header of the incoming request is
   copied except `Idempotency-Key` on a `Created` row.
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
