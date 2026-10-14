### Task 124: `hubclient`'s W4 methods over HTTP, the relay rows, W4's Hub end to end

W4, stream WH, seam **SW.20** (its HTTP half and its relay rows). Depends on task 115 (the fake's
W4 rules and the conformance rows this task runs against the real Hub), tasks 117 to 123 (every W4
Hub route), task 16 (the `WorkRoute` table, `workPath`, `checked`, the relay's `registerWork`,
`relayWork`, `body` and the audit catalog) and Hub tasks H33-H34 (the transport, `Resolve`'s
per-key digest re-check). Estimate 0.375 (3 h).

This task is to W4 what tasks 16 and 17 were to W1 (and task 82 to W2): the pipe from the desktop
to the Hub for every `WorkRoadmap` call, and the one run that proves the Hub routes of tasks 117 to
123 accept what the wire says. It produces three things.

**First, the HTTP half of `hubclient.WorkRoadmap`**: one method per route of tasks 117 to 123, each
on the exact verb and pattern those tasks fix, carried by H33's transport (idempotency key, one
refresh on `expired`, the 10 s timeout: none written twice). The one W4 record that points at a
revision is the status update: `StatusUpdate.pinnedBrief` is a key, `brief:bf-…@r`, not a
revision body. So the re-check task 16 applies to every revision-bearing response takes one extra
step here: **after every call that returns a `StatusUpdate`, the client resolves each distinct
pinned key through `Briefs.Resolve` (H34), whose per-key re-check turns a revision whose digest
does not match into `missing`**. A pin that comes back `missing` or `invalid` is refused as
`ErrUnavailable` wrapping `ErrPinUnverified`, never returned: the Hub accepted the pin (task 115's
row *the pinned revision exists*), so a pin that no longer resolves is either a tampered revision or
a Hub that disagrees with itself, and neither is put in front of a lead. `erased` passes: an erased
brief revision is a fact the card can say.

**Second, the relay rows.** The desktop talks only to its loopback control plane. Task 16's
`registerWork` mounts one relay route per `WorkRoute` row, the Hub path with `/v1` replaced by
`/v1/hub/work`. This task appends **31 rows** (`WorkRoadmapRoutes`) and their 31 handlers, and
passes every Hub refusal through by the relay rule unchanged (hub-contract §4.3, `:277-295`,
task 16's `relayWork`): `dependency_cycle`, `criterion_not_on_brief`, `update_immutable` and
`grant_exists` (task 114's appended codes) reach the desktop with their status, `code`, `message`
and `details`, so the composer can say *"An update cannot be edited once posted"* and the timeline
can say which dependency closed a loop.

**Third, W4's Hub end to end**: one scenario on `compose.yaml`, through `devissuer`, that walks
Alice's team and Bob's team across every W4 route and asserts the response and the one event per
write, as task 17 did for W1. Task 115's conformance rows run in the same package against the real
Hub. **The one thing this task must not get wrong is that a granted team reads and comments and
does nothing else**: Bob's issue create on Alice's project is `403`, and the feed he reads carries
no body text and no event from a project he was never granted.

**Files:**
- `control-plane/internal/hub/hubclient/methods_work_roadmap.go` — `WorkRoadmapRoutes`, the 31
  `WorkRoadmap` methods over HTTP, `verifyPins` (new)
- `control-plane/internal/hub/hubclient/methods_work_roadmap_test.go` — the route map against a
  recording `httptest` Hub, the pin re-check, path escaping, the `scope` rule (new)
- `control-plane/internal/hub/hubclient/client_work_roadmap_stub.go` — **deleted** in the same
  commit: task 114's `ErrNotConfigured` stubs for `*Client` (`task-114.md:41-47`, decision 2)
- `control-plane/internal/hub/hubclient/methods_work.go` ⚑ (task 16's) — `AllWorkRoutes()`, the one
  list `registerWork` iterates, and `ErrPinUnverified` beside task 16's sentinels
- `control-plane/internal/hub/hubclient/conformance/work_roadmap_integration_test.go` — task 115's
  `W4-NN` rows against the real Hub, `-tags integration` (new)
- `control-plane/internal/api/work.go` ⚑ (task 16's) — `registerWork` iterates `AllWorkRoutes()`;
  `workCalls()` gains the 31 W4 handlers through `roadmapCalls()`
- `control-plane/internal/api/work_roadmap_test.go` — the relay rule for the four W4 codes, the
  header rule, strict decoding, `scope` required (new)
- `control-plane/internal/api/routes_audit_test.go` ⚑ — the ten W4 relay `GET`s in
  `catalogReadOnlyRoutes` (`routes_audit_test.go:32`) and one test that no other new path is
- `control-plane/internal/hub/work_roadmap_e2e_integration_test.go` — `TestWorkRoadmapE2E`,
  `//go:build integration` (new; the file map names no W4 end-to-end file, so the name follows
  tasks 17 and 82)

**What is on the tree today (verified 2026-09-25).** `control-plane/internal/hub/` on `feat/hub`
(`.claude/worktrees/hub` at `1271ddb`) holds only `hubapi` and `hubclient` (`client.go`, `fake.go`,
`fake_briefs.go`, `fake_projects.go`, `hubclient.go` and tests). `hubclient.go:63` declares
`Resolve(ctx, keys []string) (hubapi.RefsResponse, error)`; `hubclient.go:98` declares
`Changes(ctx, after string, limit int) (hubapi.ChangesPage, error)`; `ErrUnavailable` is at
`hubclient.go:135`. `hubapi.RefResult{Key, Kind, Resolution, Revision, Approval}` is at
`hubapi/types.go:322-328`, the `Resolution` words `resolved·missing·erased·invalid` at `:37-43`,
`Event` at `:536-545`, `ChangesPage{Events, Next, More}` at `:547-551`. H34's `ErrDigestMismatch`,
H33's `c.send`, `server`, `store`, `devissuer`, `hubtest` and `compose.yaml` are **planned** and
**UNVERIFIED until merge**. **Task 114's part is on this tree** (`…-parts/task-114.md`): every
`hubapi` W4 type, the 31-method `WorkRoadmap` interface (`task-114.md:657-703`), its route table
(`:715-738`) and its codes (`:633-645`) below are **its** names, read from that part. Tasks 115 to
123 had no part when this was written, so their route bodies and event kinds are their scopes'
(`…-scopes/tasks-101-120.md:389-530`, `tasks-121-140.md:32-134`) and **UNVERIFIED until those
parts land**; where this part needs a name none of them fixes, it says so under *Gaps*.

**Interfaces:**
- Produces: the HTTP `WorkRoadmap`; `WorkRoadmapRoutes`; `AllWorkRoutes()`; `ErrPinUnverified`;
  the 31 relay routes; `TestWorkRoadmapE2E`, the proof task 141 (W4's results) cites.
- Consumes: task 114's `hubapi` W4 types and the `WorkRoadmap` interface; task 115's
  `conformance.WorkRoadmapRows`; task 16's `WorkRoute`, `workPath`, `workRoute`, `c.work`,
  `registerWork`, `relayWork`, `body`, `workRelayRead`; H34's `Resolve`; H43's world (as task 17
  reuses it); task 121's janitor test hook; task 123's scoped `/v1/changes`.
- Required by: task 125 (the desktop's calls, which name these relay paths), task 141.

#### Contract — the route table, complete

One table, as task 16's. The Hub column is the route each of tasks 117 to 123 fixes; the relay
column is derived by task 16's `WorkRoute.Relay()`, never typed.

```go
// control-plane/internal/hub/hubclient/methods_work_roadmap.go
package hubclient

// WorkRoadmapRoutes is every W4 route (tasks 117-123), in task order. The
// relay registers exactly these beside task 16's; the test requires one
// Client method per row and one row per WorkRoadmap method.
var WorkRoadmapRoutes = []WorkRoute{
	// task 117: project grants and an initiative's teams
	{"GrantProject", http.MethodPost, "/v1/projects/{projectId}/grants", true},
	{"ProjectGrants", http.MethodGet, "/v1/projects/{projectId}/grants", false},
	{"RevokeProjectGrant", http.MethodDelete, "/v1/projects/{projectId}/grants/{teamId}", false},
	{"SetInitiativeTeams", http.MethodPut, "/v1/initiatives/{initiativeId}/teams", false},
	// task 118: initiatives
	{"CreateInitiative", http.MethodPost, "/v1/teams/{teamId}/initiatives", true},
	{"Initiatives", http.MethodGet, "/v1/teams/{teamId}/initiatives", false},
	{"Initiative", http.MethodGet, "/v1/initiatives/{initiativeId}", false},
	{"PatchInitiative", http.MethodPatch, "/v1/initiatives/{initiativeId}", false},
	{"SetFeatureInitiative", http.MethodPut, "/v1/features/{featureId}/initiative", false},
	// task 119: status updates, newest first
	{"PostFeatureUpdate", http.MethodPost, "/v1/features/{featureId}/updates", true},
	{"FeatureUpdates", http.MethodGet, "/v1/features/{featureId}/updates", false},
	{"PostInitiativeUpdate", http.MethodPost, "/v1/initiatives/{initiativeId}/updates", true},
	{"InitiativeUpdates", http.MethodGet, "/v1/initiatives/{initiativeId}/updates", false},
	// task 120: milestones, feature dependencies, a feature from a template
	{"CreateMilestone", http.MethodPost, "/v1/features/{featureId}/milestones", true},
	{"Milestones", http.MethodGet, "/v1/features/{featureId}/milestones", false},
	{"PatchMilestone", http.MethodPatch, "/v1/milestones/{milestoneId}", false},
	{"SetMilestoneCriteria", http.MethodPut, "/v1/milestones/{milestoneId}/criteria", false},
	{"AddFeatureDependency", http.MethodPost, "/v1/features/{featureId}/dependencies", false},
	{"FeatureDependencies", http.MethodGet, "/v1/projects/{projectId}/feature-dependencies", false},
	{"RemoveFeatureDependency", http.MethodDelete, "/v1/features/{featureId}/dependencies/{blocks}", false},
	{"CreateFeatureFromTemplate", http.MethodPost, "/v1/projects/{projectId}/features:fromTemplate", true},
	// task 121: schedules, reminders, the triage rotation
	{"SetTemplateSchedule", http.MethodPut, "/v1/templates/{templateId}/schedule", false},
	{"ClearTemplateSchedule", http.MethodDelete, "/v1/templates/{templateId}/schedule", false},
	{"SetReminderPolicy", http.MethodPut, "/v1/reminder-policies", false},
	{"SetTriageRotation", http.MethodPut, "/v1/teams/{teamId}/triage-rotation", false},
	{"TriageRotation", http.MethodGet, "/v1/teams/{teamId}/triage-rotation", false},
	// task 122: insights, a read that takes a body, so POST
	{"QueryInsights", http.MethodPost, "/v1/teams/{teamId}/insights/query", false},
	// task 123: read cursors and the scoped feed
	{"GetReadCursor", http.MethodGet, "/v1/me/read-cursors", false},
	{"AdvanceReadCursor", http.MethodPut, "/v1/me/read-cursors", false},
	{"MarkSeen", http.MethodPost, "/v1/me/read-cursors/mark-seen", false},
	{"ScopedChanges", http.MethodGet, "/v1/changes", false},
}

// briefTemplateRoutes are task 114's three brief-template methods. Their
// paths are task 78's template routes (kind "brief" in the body), which W2's
// table (task 82) already relays, so they are client-only here: workRoute()
// finds them, AllWorkRoutes() does not mount them a second time (ServeMux
// panics on a duplicate pattern).
var briefTemplateRoutes = []WorkRoute{
	{"CreateBriefTemplate", http.MethodPost, "/v1/teams/{teamId}/templates", true},
	{"BriefTemplates", http.MethodGet, "/v1/teams/{teamId}/templates", false},
	{"PatchBriefTemplate", http.MethodPatch, "/v1/templates/{templateId}", false},
}
```

```go
// control-plane/internal/hub/hubclient/methods_work.go ⚑ (task 16's)

// ErrPinUnverified: a StatusUpdate's pinnedBrief did not resolve (task 124).
var ErrPinUnverified = errors.New("hubclient: pinned brief revision did not verify")

// AllWorkRoutes is every Work row the relay mounts: W1 (task 16), W2 (task
// 82, if its part adds a table, appended here at sync) and W4 (task 124).
func AllWorkRoutes() []WorkRoute {
	all := append([]WorkRoute(nil), WorkRoutes...)
	return append(all, WorkRoadmapRoutes...)
}

// workRoute (task 16's) now searches AllWorkRoutes() and then
// briefTemplateRoutes, not WorkRoutes alone.
```

Thirty-one relay rows, ten of them `GET`s, plus three client-only rows. They serve task 114's 31
methods and one more, `ScopedChanges` (*Gaps*, 3): `PostUpdate` and `Updates` each take two rows
(feature and initiative), and the three brief-template methods ride task 78's rows. Four notes:

- `SetMilestoneCriteria` and `FeatureDependencies` are task 114's † paths (`task-114.md:724,727`),
  fixed there because task 120's scope names the methods and not the paths. `FeatureDependencies`
  is per **project** (`GET /v1/projects/{projectId}/feature-dependencies`), which is what the
  replica's full load (task 125) wants: one call per project, both directions, each row a
  `FeatureDependencyView` with the Hub's derived `orderViolated`.
- `ScopedChanges` is task 123's `GET /v1/changes?after=&scope=`. Its relay twin is
  `/v1/hub/work/changes`, which does not collide with H's `/v1/hub/changes` (hub-contract §9.3,
  `:1372`). The whole feed stays H's; this row **requires** `scope`, so it can never be used as a
  second, unfiltered feed.
- `features:fromTemplate` is one literal segment under Go 1.22's `ServeMux` (a wildcard must be a
  whole segment; a colon in a literal is plain text), so it coexists with W2's `features` routes.
- `/v1/initiatives/{initiativeId}` is spelled with `initiativeId` everywhere; task 117's scope
  writes `{id}` for the teams route, and the wildcard name is not part of the wire.

#### Contract — the client methods

Every method is task 16's three lines over `c.work`. The request and response types are task 114's
(`hubapi/work_roadmap.go`); the method set is `hubclient.WorkRoadmap`, which task 114 embeds in
`Work`, so `var _ Work = (*Client)(nil)` (task 16's assertion) stops compiling until every method
below exists. Part 1: grants, initiatives, updates.

```go
// control-plane/internal/hub/hubclient/methods_work_roadmap.go (continued)

var _ WorkRoadmap = (*Client)(nil)

func (c *Client) GrantProject(ctx context.Context, projectID string, req hubapi.GrantProjectRequest) (hubapi.ProjectGrant, error) {
	var out hubapi.ProjectGrant
	return out, c.work(ctx, "GrantProject", nil, req, &out, projectID)
}

func (c *Client) ProjectGrants(ctx context.Context, projectID string) ([]hubapi.ProjectGrant, error) {
	var out struct {
		Items []hubapi.ProjectGrant `json:"items"`
	}
	return out.Items, c.work(ctx, "ProjectGrants", nil, nil, &out, projectID)
}

func (c *Client) RevokeProjectGrant(ctx context.Context, projectID, teamID string) error {
	return c.work(ctx, "RevokeProjectGrant", nil, nil, nil, projectID, teamID)
}

// SetInitiativeTeams replaces the span; TeamIDs[0] must be the owning team.
func (c *Client) SetInitiativeTeams(ctx context.Context, initiativeID string, req hubapi.SetInitiativeTeamsRequest) (hubapi.InitiativeTeams, error) {
	var out hubapi.InitiativeTeams
	return out, c.work(ctx, "SetInitiativeTeams", nil, req, &out, initiativeID)
}

func (c *Client) CreateInitiative(ctx context.Context, teamID string, req hubapi.CreateInitiativeRequest) (hubapi.Initiative, error) {
	var out hubapi.Initiative
	return out, c.work(ctx, "CreateInitiative", nil, req, &out, teamID)
}

func (c *Client) Initiatives(ctx context.Context, teamID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Initiative], error) {
	var out hubapi.Page[hubapi.Initiative]
	return out, c.work(ctx, "Initiatives", EncodeQuery(page), nil, &out, teamID)
}

func (c *Client) Initiative(ctx context.Context, initiativeID string) (hubapi.InitiativeView, error) {
	var out hubapi.InitiativeView
	return out, c.work(ctx, "Initiative", nil, nil, &out, initiativeID)
}

func (c *Client) PatchInitiative(ctx context.Context, initiativeID string, req hubapi.PatchInitiativeRequest) (hubapi.Initiative, error) {
	var out hubapi.Initiative
	return out, c.work(ctx, "PatchInitiative", nil, req, &out, initiativeID)
}

// SetFeatureInitiative points a feature at one initiative, or at none
// (req.InitiativeID nil).
func (c *Client) SetFeatureInitiative(ctx context.Context, featureID string, req hubapi.SetFeatureInitiativeRequest) (hubapi.Feature, error) {
	var out hubapi.Feature
	return out, c.work(ctx, "SetFeatureInitiative", nil, req, &out, featureID)
}

// PostUpdate picks the feature or the initiative route by subject.Kind; the
// Hub, not the client, refuses any other kind (task 114's Validate does too).
func (c *Client) PostUpdate(ctx context.Context, subject hubapi.SubjectRef, req hubapi.PostUpdateRequest) (hubapi.StatusUpdate, error) {
	method := "PostFeatureUpdate"
	if subject.Kind == hubapi.SubjectInitiative {
		method = "PostInitiativeUpdate"
	}
	var out hubapi.StatusUpdate
	if err := c.work(ctx, method, nil, req, &out, subject.ID); err != nil {
		return out, err
	}
	return out, c.verifyPins(ctx, out)
}

func (c *Client) Updates(ctx context.Context, subject hubapi.SubjectRef, page hubapi.PageRequest) (hubapi.Page[hubapi.StatusUpdate], error) {
	method := "FeatureUpdates"
	if subject.Kind == hubapi.SubjectInitiative {
		method = "InitiativeUpdates"
	}
	var out hubapi.Page[hubapi.StatusUpdate]
	if err := c.work(ctx, method, EncodeQuery(page), nil, &out, subject.ID); err != nil {
		return out, err
	}
	return out, c.verifyPins(ctx, out.Items...)
}
```

`ProjectGrants` answers `{items}` with no cursor: a project has as many grants as the org has
teams, and task 117 pages nothing. Two rows each (`PostFeatureUpdate` and `PostInitiativeUpdate`,
`FeatureUpdates` and `InitiativeUpdates`) serve one method, so `TestEveryWorkRoadmapMethodHasARow`
maps method to rows through `roadmapRowsOf` (below), not by name equality.

Part 2: milestones, dependencies, the template create, schedules and the rotation.

```go
// control-plane/internal/hub/hubclient/methods_work_roadmap.go (continued)

func (c *Client) CreateMilestone(ctx context.Context, featureID string, req hubapi.CreateMilestoneRequest) (hubapi.MilestoneView, error) {
	var out hubapi.MilestoneView
	return out, c.work(ctx, "CreateMilestone", nil, req, &out, featureID)
}

// Milestones is task 120's read: each with its criteria's thread-state words;
// reached is derived by the Hub at read and never sent back by the client.
func (c *Client) Milestones(ctx context.Context, featureID string) ([]hubapi.MilestoneView, error) {
	var out struct {
		Items []hubapi.MilestoneView `json:"items"`
	}
	return out.Items, c.work(ctx, "Milestones", nil, nil, &out, featureID)
}

func (c *Client) PatchMilestone(ctx context.Context, milestoneID string, req hubapi.PatchMilestoneRequest) (hubapi.MilestoneView, error) {
	var out hubapi.MilestoneView
	return out, c.work(ctx, "PatchMilestone", nil, req, &out, milestoneID)
}

func (c *Client) SetMilestoneCriteria(ctx context.Context, milestoneID string, req hubapi.SetMilestoneCriteriaRequest) (hubapi.MilestoneView, error) {
	var out hubapi.MilestoneView
	return out, c.work(ctx, "SetMilestoneCriteria", nil, req, &out, milestoneID)
}

func (c *Client) AddFeatureDependency(ctx context.Context, featureID string, req hubapi.FeatureDependencyRequest) (hubapi.FeatureDependencyView, error) {
	var out hubapi.FeatureDependencyView
	return out, c.work(ctx, "AddFeatureDependency", nil, req, &out, featureID)
}

// FeatureDependencies lists every dependency in a project, both directions,
// each with the Hub's derived OrderViolated (task 114's † path).
func (c *Client) FeatureDependencies(ctx context.Context, projectID string) ([]hubapi.FeatureDependencyView, error) {
	var out struct {
		Items []hubapi.FeatureDependencyView `json:"items"`
	}
	return out.Items, c.work(ctx, "FeatureDependencies", nil, nil, &out, projectID)
}

func (c *Client) RemoveFeatureDependency(ctx context.Context, featureID, blocksID string) error {
	return c.work(ctx, "RemoveFeatureDependency", nil, nil, nil, featureID, blocksID)
}

// The three brief-template methods ride task 78's rows (briefTemplateRoutes).
func (c *Client) CreateBriefTemplate(ctx context.Context, teamID string, req hubapi.CreateBriefTemplateRequest) (hubapi.BriefTemplate, error) {
	var out hubapi.BriefTemplate
	body := struct {
		Kind string `json:"kind"`
		hubapi.CreateBriefTemplateRequest
	}{"brief", req}
	return out, c.work(ctx, "CreateBriefTemplate", nil, body, &out, teamID)
}

// BriefTemplates is task 78's list with ?kind=brief, so issue templates never
// decode into a BriefTemplate.
func (c *Client) BriefTemplates(ctx context.Context, teamID string) ([]hubapi.BriefTemplate, error) {
	var out struct {
		Items []hubapi.BriefTemplate `json:"items"`
	}
	return out.Items, c.work(ctx, "BriefTemplates", url.Values{"kind": {"brief"}}, nil, &out, teamID)
}

func (c *Client) PatchBriefTemplate(ctx context.Context, templateID string, req hubapi.PatchBriefTemplateRequest) (hubapi.BriefTemplate, error) {
	var out hubapi.BriefTemplate
	return out, c.work(ctx, "PatchBriefTemplate", nil, req, &out, templateID)
}

// CreateFeatureFromTemplate returns the feature (task 114's signature). Its
// brief at revision 1 and its issues are read afterwards through the methods
// that already re-hash them (H34's brief reads, task 16's issue reads).
func (c *Client) CreateFeatureFromTemplate(ctx context.Context, projectID string, req hubapi.CreateFeatureFromTemplateRequest) (hubapi.Feature, error) {
	var out hubapi.Feature
	return out, c.work(ctx, "CreateFeatureFromTemplate", nil, req, &out, projectID)
}

func (c *Client) SetTemplateSchedule(ctx context.Context, templateID string, s hubapi.TemplateSchedule) (hubapi.TemplateSchedule, error) {
	var out hubapi.TemplateSchedule
	return out, c.work(ctx, "SetTemplateSchedule", nil, s, &out, templateID)
}

func (c *Client) ClearTemplateSchedule(ctx context.Context, templateID string) error {
	return c.work(ctx, "ClearTemplateSchedule", nil, nil, nil, templateID)
}

func (c *Client) SetReminderPolicy(ctx context.Context, p hubapi.ReminderPolicy) (hubapi.ReminderPolicy, error) {
	var out hubapi.ReminderPolicy
	return out, c.work(ctx, "SetReminderPolicy", nil, p, &out)
}

func (c *Client) SetTriageRotation(ctx context.Context, teamID string, r hubapi.TriageRotation) (hubapi.TriageRotationView, error) {
	var out hubapi.TriageRotationView
	return out, c.work(ctx, "SetTriageRotation", nil, r, &out, teamID)
}

// TriageRotation adds who is on duty and the stored rule's next handover
// date (task 121): a date the policy fixes, not a forecast.
func (c *Client) TriageRotation(ctx context.Context, teamID string) (hubapi.TriageRotationView, error) {
	var out hubapi.TriageRotationView
	return out, c.work(ctx, "TriageRotation", nil, nil, &out, teamID)
}
```

Part 3: insights, read cursors, the scoped feed, and the pin re-check.

```go
// control-plane/internal/hub/hubclient/methods_work_roadmap.go (continued)

// QueryInsights: the team is q.TeamID (task 114's signature carries no
// separate team), and it fills the path's {teamId}.
func (c *Client) QueryInsights(ctx context.Context, q hubapi.InsightsQuery) (hubapi.InsightsResult, error) {
	var out hubapi.InsightsResult
	return out, c.work(ctx, "QueryInsights", nil, q, &out, q.TeamID)
}

// scopeValue writes task 123's ?scope={kind}:{id}.
func scopeValue(s hubapi.ScopeRef) string { return s.Kind + ":" + s.ID }

// GetReadCursor reads the caller's own cursor. There is no user parameter:
// a cursor is the person's, and the Hub reads the person from the token.
func (c *Client) GetReadCursor(ctx context.Context, scope hubapi.ScopeRef) (hubapi.ReadCursor, error) {
	var out hubapi.ReadCursor
	return out, c.work(ctx, "GetReadCursor", url.Values{"scope": {scopeValue(scope)}}, nil, &out)
}

// AdvanceReadCursor only advances; a lower Seq returns the stored cursor.
func (c *Client) AdvanceReadCursor(ctx context.Context, cur hubapi.ReadCursor) (hubapi.ReadCursor, error) {
	var out hubapi.ReadCursor
	return out, c.work(ctx, "AdvanceReadCursor", nil, cur, &out)
}

func (c *Client) MarkSeen(ctx context.Context, req hubapi.MarkSeenRequest) ([]hubapi.ReadCursor, error) {
	var out struct {
		Items []hubapi.ReadCursor `json:"items"`
	}
	return out.Items, c.work(ctx, "MarkSeen", nil, req, &out)
}

// ScopedChanges is task 123's filtered read of /v1/changes; it is not in task
// 114's WorkRoadmap (Gaps, 3). A zero scope is a programming error, not a
// request for the whole feed: that is Feed.Changes.
func (c *Client) ScopedChanges(ctx context.Context, scope hubapi.ScopeRef, after string, limit int) (hubapi.ScopedChangesPage, error) {
	if scope.Kind == "" || scope.ID == "" {
		panic("hubclient: ScopedChanges needs a scope; the whole feed is Feed.Changes")
	}
	q := url.Values{"scope": {scopeValue(scope)}}
	if after != "" {
		q.Set("after", after)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out hubapi.ScopedChangesPage
	return out, c.work(ctx, "ScopedChanges", q, nil, &out)
}

// verifyPins resolves every distinct pinnedBrief of the updates in one
// Resolve call (H34 batches by 100 and re-hashes each revision it returns;
// a revision whose digest fails comes back "missing"). resolved and erased
// pass; missing and invalid are ErrPinUnverified inside ErrUnavailable, and
// the caller receives no update at all, not the ones that passed.
func (c *Client) verifyPins(ctx context.Context, updates ...hubapi.StatusUpdate) error {
	seen := make(map[string]bool, len(updates))
	keys := make([]string, 0, len(updates))
	for _, u := range updates {
		if !seen[u.PinnedBrief] {
			seen[u.PinnedBrief] = true
			keys = append(keys, u.PinnedBrief)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	refs, err := c.Resolve(ctx, keys)
	if err != nil {
		return err
	}
	for _, r := range refs.Results {
		switch r.Resolution {
		case hubapi.Resolved, hubapi.Erased:
		default:
			return fmt.Errorf("%w: %w (%s is %s)", ErrUnavailable, ErrPinUnverified, r.Key, r.Resolution)
		}
	}
	return nil
}
```

`scopeValue` writes `project:pj-…`, `feature:ft-…` or `issue:is-…`, the form task 123's `?scope=`
reads; task 114's `ScopeRef` has `Validate()` (`task-114.md:624`) and no string form, so this part
writes the two helpers (`scopeValue` here, `parseScope` in the relay). `verifyPins` costs one
extra round trip per update call, one `Resolve` over the distinct pins of the page. **Not
measured**: no number is claimed for its latency.

#### Contract — the relay rows

`registerWork` (task 16, `work.go`) changes in two lines: it iterates `hubclient.AllWorkRoutes()`
instead of `hubclient.WorkRoutes`, and `workCalls()` merges `roadmapCalls()` in, panicking on a
name both define. Its missing-row and missing-handler panics then cover the W4 rows unchanged.
`relayWork` is untouched: the relay rule, the `hub_not_configured` answer for a nil `Deps.Work`,
and the `201` for `Created` rows are task 16's. **The relay never forwards the desktop's
`Authorization` header**: a handler receives `*http.Request` only to read path values, the query
and the body, and calls `hubclient`, whose transport sets the person's Hub token itself (H33).
`work_roadmap_test.go` proves it for every row.

```go
// control-plane/internal/api/work.go ⚑ — in registerWork
	calls := workCalls()
	for name, call := range roadmapCalls() {
		if _, dup := calls[name]; dup {
			panic("api: relay handler " + name + " defined twice")
		}
		calls[name] = call
	}
	for _, rt := range hubclient.AllWorkRoutes() { // was hubclient.WorkRoutes
```

```go
// control-plane/internal/api/work.go (continued) — the W4 handlers

// emptyAnswer is the empty answer of a write with no body to return (a DELETE).
var emptyAnswer = struct{}{}

// scopeParam reads ?scope={kind}:{id}; absent or malformed is 400 before the
// Hub is called, which is what keeps /v1/hub/work/changes a scoped read only.
func scopeParam(r *http.Request) (hubapi.ScopeRef, error) {
	kind, id, ok := strings.Cut(r.URL.Query().Get("scope"), ":")
	s := hubapi.ScopeRef{Kind: kind, ID: id}
	if !ok || s.Validate() != nil { // task 114's ScopeRef.Validate: the three kinds, a prefixed id
		return s, &hubclient.FieldError{Field: "scope", Reason: "must be project:, feature: or issue: and an id"}
	}
	return s, nil
}

// in1 is a handler that decodes one body of type T and makes one call.
func in1[T any](call func(r *http.Request, wk hubclient.Work, req T) (any, error)) workCall {
	return func(r *http.Request, w http.ResponseWriter, wk hubclient.Work) (any, error) {
		req, err := body[T](w, r)
		if err != nil {
			return nil, err
		}
		return call(r, wk, req)
	}
}

func roadmapCalls() map[string]workCall {
	pv := func(r *http.Request, k string) string { return r.PathValue(k) }
	ctx := func(r *http.Request) context.Context { return r.Context() }
	feature := func(r *http.Request) hubapi.SubjectRef {
		return hubapi.SubjectRef{Kind: hubapi.SubjectFeature, ID: pv(r, "featureId")}
	}
	initiative := func(r *http.Request) hubapi.SubjectRef {
		return hubapi.SubjectRef{Kind: hubapi.SubjectInitiative, ID: pv(r, "initiativeId")}
	}
	return map[string]workCall{
		"GrantProject": in1(func(r *http.Request, wk hubclient.Work, q hubapi.GrantProjectRequest) (any, error) {
			return wk.GrantProject(ctx(r), pv(r, "projectId"), q)
		}),
		"ProjectGrants": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			items, err := wk.ProjectGrants(ctx(r), pv(r, "projectId"))
			return map[string]any{"items": items}, err
		},
		"RevokeProjectGrant": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			return emptyAnswer, wk.RevokeProjectGrant(ctx(r), pv(r, "projectId"), pv(r, "teamId"))
		},
		"SetInitiativeTeams": in1(func(r *http.Request, wk hubclient.Work, q hubapi.SetInitiativeTeamsRequest) (any, error) {
			return wk.SetInitiativeTeams(ctx(r), pv(r, "initiativeId"), q)
		}),
		"CreateInitiative": in1(func(r *http.Request, wk hubclient.Work, q hubapi.CreateInitiativeRequest) (any, error) {
			return wk.CreateInitiative(ctx(r), pv(r, "teamId"), q)
		}),
		"Initiatives": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			page, err := hubclient.DecodeQuery[hubapi.PageRequest](r.URL.Query())
			if err != nil {
				return nil, err
			}
			return wk.Initiatives(ctx(r), pv(r, "teamId"), page)
		},
		"Initiative": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			return wk.Initiative(ctx(r), pv(r, "initiativeId"))
		},
		"PatchInitiative": in1(func(r *http.Request, wk hubclient.Work, q hubapi.PatchInitiativeRequest) (any, error) {
			return wk.PatchInitiative(ctx(r), pv(r, "initiativeId"), q)
		}),
		"SetFeatureInitiative": in1(func(r *http.Request, wk hubclient.Work, q hubapi.SetFeatureInitiativeRequest) (any, error) {
			return wk.SetFeatureInitiative(ctx(r), pv(r, "featureId"), q)
		}),
```

```go
		"PostFeatureUpdate": in1(func(r *http.Request, wk hubclient.Work, q hubapi.PostUpdateRequest) (any, error) {
			return wk.PostUpdate(ctx(r), feature(r), q)
		}),
		"FeatureUpdates": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			page, err := hubclient.DecodeQuery[hubapi.PageRequest](r.URL.Query())
			if err != nil {
				return nil, err
			}
			return wk.Updates(ctx(r), feature(r), page)
		},
		"PostInitiativeUpdate": in1(func(r *http.Request, wk hubclient.Work, q hubapi.PostUpdateRequest) (any, error) {
			return wk.PostUpdate(ctx(r), initiative(r), q)
		}),
		"InitiativeUpdates": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			page, err := hubclient.DecodeQuery[hubapi.PageRequest](r.URL.Query())
			if err != nil {
				return nil, err
			}
			return wk.Updates(ctx(r), initiative(r), page)
		},
		"CreateMilestone": in1(func(r *http.Request, wk hubclient.Work, q hubapi.CreateMilestoneRequest) (any, error) {
			return wk.CreateMilestone(ctx(r), pv(r, "featureId"), q)
		}),
		"Milestones": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			items, err := wk.Milestones(ctx(r), pv(r, "featureId"))
			return map[string]any{"items": items}, err
		},
		"PatchMilestone": in1(func(r *http.Request, wk hubclient.Work, q hubapi.PatchMilestoneRequest) (any, error) {
			return wk.PatchMilestone(ctx(r), pv(r, "milestoneId"), q)
		}),
		"SetMilestoneCriteria": in1(func(r *http.Request, wk hubclient.Work, q hubapi.SetMilestoneCriteriaRequest) (any, error) {
			return wk.SetMilestoneCriteria(ctx(r), pv(r, "milestoneId"), q)
		}),
		"AddFeatureDependency": in1(func(r *http.Request, wk hubclient.Work, q hubapi.FeatureDependencyRequest) (any, error) {
			return wk.AddFeatureDependency(ctx(r), pv(r, "featureId"), q)
		}),
		"FeatureDependencies": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			items, err := wk.FeatureDependencies(ctx(r), pv(r, "projectId"))
			return map[string]any{"items": items}, err
		},
		"RemoveFeatureDependency": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			return emptyAnswer, wk.RemoveFeatureDependency(ctx(r), pv(r, "featureId"), pv(r, "blocks"))
		},
		"CreateFeatureFromTemplate": in1(func(r *http.Request, wk hubclient.Work, q hubapi.CreateFeatureFromTemplateRequest) (any, error) {
			return wk.CreateFeatureFromTemplate(ctx(r), pv(r, "projectId"), q)
		}),
		"SetTemplateSchedule": in1(func(r *http.Request, wk hubclient.Work, q hubapi.TemplateSchedule) (any, error) {
			return wk.SetTemplateSchedule(ctx(r), pv(r, "templateId"), q)
		}),
		"ClearTemplateSchedule": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			return emptyAnswer, wk.ClearTemplateSchedule(ctx(r), pv(r, "templateId"))
		},
		"SetReminderPolicy": in1(func(r *http.Request, wk hubclient.Work, q hubapi.ReminderPolicy) (any, error) {
			return wk.SetReminderPolicy(ctx(r), q)
		}),
		"SetTriageRotation": in1(func(r *http.Request, wk hubclient.Work, q hubapi.TriageRotation) (any, error) {
			return wk.SetTriageRotation(ctx(r), pv(r, "teamId"), q)
		}),
		"TriageRotation": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			return wk.TriageRotation(ctx(r), pv(r, "teamId"))
		},
		"QueryInsights": in1(func(r *http.Request, wk hubclient.Work, q hubapi.InsightsQuery) (any, error) {
			if q.TeamID != pv(r, "teamId") { // the body's team and the path's are one team
				return nil, &hubclient.FieldError{Field: "teamId", Reason: "must match the path"}
			}
			return wk.QueryInsights(ctx(r), q)
		}),
		"GetReadCursor": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			s, err := scopeParam(r)
			if err != nil {
				return nil, err
			}
			return wk.GetReadCursor(ctx(r), s) // any ?userId= is never read
		},
		"AdvanceReadCursor": in1(func(r *http.Request, wk hubclient.Work, q hubapi.ReadCursor) (any, error) {
			return wk.AdvanceReadCursor(ctx(r), q)
		}),
		"MarkSeen": in1(func(r *http.Request, wk hubclient.Work, q hubapi.MarkSeenRequest) (any, error) {
			items, err := wk.MarkSeen(ctx(r), q)
			return map[string]any{"items": items}, err
		}),
		"ScopedChanges": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			s, err := scopeParam(r)
			if err != nil {
				return nil, err
			}
			q := r.URL.Query()
			limit, err := queryLimit(q.Get("limit"))
			if err != nil {
				return nil, err
			}
			return wk.ScopedChanges(ctx(r), s, q.Get("after"), limit)
		},
	}
}
```

`queryLimit` is `""` → 0 (the Hub's default), else a decimal from 1 to 500 or a `*FieldError` on
`limit`, so a malformed limit is a 400 here and not a silent default. `scopeParam` splits at the
first `:` and leans on task 114's `ScopeRef.Validate()` (`task-114.md:624`) for the kinds and the
id prefix, so the relay and the Hub refuse the same scopes.

**The brief-template methods are not relayed by these rows.** `CreateBriefTemplate`,
`BriefTemplates` and `PatchBriefTemplate` share task 78's paths, whose relay rows are task 82's
(W2). The desktop reaches them through task 82's relay with `kind: "brief"` in the body; that
relay handler must pass the `kind` and the brief fields through (a gap for task 82, *Gaps*, 6).

**The route audit.** Ten W4 relay routes are `GET`s; each gets one line in
`catalogReadOnlyRoutes`, under task 16's `workRelayRead` reason, and one new test pins the whole
set of relay patterns so that a path added outside the tables fails by name.

```go
// control-plane/internal/api/routes_audit_test.go ⚑ — appended to catalogReadOnlyRoutes
	"GET /v1/hub/work/projects/{projectId}/grants":          workRelayRead,
	"GET /v1/hub/work/teams/{teamId}/initiatives":           workRelayRead,
	"GET /v1/hub/work/initiatives/{initiativeId}":           workRelayRead,
	"GET /v1/hub/work/features/{featureId}/updates":         workRelayRead,
	"GET /v1/hub/work/initiatives/{initiativeId}/updates":   workRelayRead,
	"GET /v1/hub/work/features/{featureId}/milestones":      workRelayRead,
	"GET /v1/hub/work/features/{featureId}/dependencies":    workRelayRead,
	"GET /v1/hub/work/teams/{teamId}/triage-rotation":       workRelayRead,
	"GET /v1/hub/work/me/read-cursors":                      workRelayRead,
	"GET /v1/hub/work/changes":                              workRelayRead,

// TestW4RelayPathsAreExactlyTheTable: every pattern the API mux holds under
// /v1/hub/work/ is a Relay() of some AllWorkRoutes() row, and the W4 rows add
// exactly 31 patterns. A handler mounted by hand beside the table, or a row
// dropped from it, fails here by pattern name.
func TestW4RelayPathsAreExactlyTheTable(t *testing.T) {
	want := map[string]bool{}
	for _, rt := range hubclient.AllWorkRoutes() {
		want[rt.Relay()] = true
	}
	got := registeredPatterns(t) // routes_audit_test.go's own walk of the mux
	for p := range got {
		if strings.Contains(p, " /v1/hub/work/") && !want[p] {
			t.Errorf("%q is mounted under /v1/hub/work but is in no WorkRoute table", p)
		}
	}
	for p := range want {
		if !got[p] {
			t.Errorf("%q is in a WorkRoute table but not mounted", p)
		}
	}
	if n := len(hubclient.WorkRoadmapRoutes); n != 31 {
		t.Errorf("WorkRoadmapRoutes has %d rows; tasks 117-123 fix 31", n)
	}
}
```

`registeredPatterns` is the name this part gives the walk `TestNoRegisteredGetCanChangeState`
already does (`routes_audit_test.go:133`); if that test inlines its walk, this task lifts it into
the helper and changes nothing else about it.

#### Contract — W4's Hub end to end

**The world** is task 17's, reused by name: H43's `hubtest.Migrated`, `devissuer` in-process with
`Auto`, the Hub's real handler on an `httptest.Server`, one frozen clock `clk` (so the janitor's
passes are driven by the test, never by wall time). **The people**: `alice` (admin of team
*Checkout*, owner of project `EP`), `bob` (admin of team *Payments*), `cara` (member of
*Checkout*, not an admin). Every call goes through a `*hubclient.Client` signed in as that person,
never through `store`, so the Hub's route table, matrix and RLS are all in the path.

```go
// control-plane/internal/hub/work_roadmap_e2e_integration_test.go
//go:build integration

package hub_test

// roadmapPeople extends task 17's devissuer tenant; OIDs are fixed test values.
var roadmapPeople = []devissuer.User{
	{OID: "5b0c1c9e-0000-4000-8000-0000000a11ce", Name: "Alice Example",
		Username: "alice@example.test", Email: "alice@example.test"},
	{OID: "5b0c1c9e-0000-4000-8000-000000000b0b", Name: "Bob Example",
		Username: "bob@example.test", Email: "bob@example.test"},
	{OID: "5b0c1c9e-0000-4000-8000-00000000ca7a", Name: "Cara Example",
		Username: "cara@example.test", Email: "cara@example.test"},
}

// roadmapStep is one t.Run: a call, the answer it must give, and the kinds of
// the events it must append (in order; empty for a read or a refusal).
type roadmapStep struct {
	name   string
	run    func(t *testing.T, w *roadmapWorld) error
	status int      // 0 means success; else the refusal's HTTP status
	code   string   // the refusal's hubapi code, when status != 0
	kinds  []string // the feed kinds appended, SW.5 names
}
```

**The story**, one `t.Run` per step, in order. The kinds in the last column are task 115's fake
kinds, which task 125 copies by name; a step whose kinds differ from the Hub's fails with both
lists printed. `bf-ep` is `EP`'s branch brief for the feature, written by the setup at revisions 1
to 3; `c1` and `c2` are two of its criteria; `ev-1` is one evidence record on `c1`, recorded by
Alice in the setup through H34's `Evidence`.

| # | step | caller | expect | kinds appended |
|---|---|---|---|---|
| 0 | setup: teams, `EP`, feature `ft-ep` with `bf-ep@3`, issue `EP-1` serving `c1`, `ev-1` | alice | ok | (task 17's) |
| 1 | `GrantProject(EP, Payments)` | alice | 201 | `grant.created` |
| 2 | `GrantProject(EP, Payments)` again | alice | 409 `grant_exists` | — |
| 3 | `GrantProject(EP, Checkout2)` | cara | 403 `forbidden` | — |
| 4 | `CreateInitiative(Checkout, "Faster checkout")` | alice | 201 | `initiative.created` |
| 5 | `SetInitiativeTeams(in, [Checkout, Payments])` | alice | ok | `initiative.teams_set` |
| 6 | `SetFeatureInitiative(ft-ep, in)` | alice | ok | `feature.updated` |
| 7 | `CreateMilestone(ft-ep, "Beta", [c1, c2])` | alice | 201 | `milestone.created` |
| 8 | `CreateMilestone(ft-ep, "Wrong", [c-other])` | alice | 422 `criterion_not_on_brief` | — |
| 9 | `AddFeatureDependency(ft-ep, blocks ft-pay)` | alice | ok | `feature_dep.added` |
| 10 | `AddFeatureDependency(ft-pay, blocks ft-ep)` | alice | 422 `dependency_cycle` | — |
| 11 | `PostUpdate(feature ft-ep, at_risk, pin bf-ep@3, cites [ev-1])` | alice | 201 | `work.update_posted` |
| 12 | `PATCH /v1/updates/{nt-…}` with `{health: on_track}` (task 119's refusing route) | alice | 409 `update_immutable` | — |
| 13 | `Initiative(in)` read | bob | ok, `ft-ep` with `latestUpdate {at_risk, at}` | — |
| 14 | `CreateComment(EP-1, …)` | bob | 201 | `comment.created` |
| 15 | `CreateIssue(EP, …)` | bob | 403 `forbidden` | — |
| 16 | `SetReminderPolicy(ft-ep, everyDays 7)`; `SetTemplateSchedule(tp-weekly, weekly 09:00 Australia/Sydney)` | alice | ok | `reminder_policy.set`, `template_schedule.set` |
| 17 | `clk` +9 days; the janitor's test hook runs once | — | ok | `issue.created` (actor null), `update.reminder` |
| 18 | `QueryInsights(Checkout, count, closed in range)` | alice | one group, `EP-1` its one point | — |
| 19 | `AdvanceReadCursor(feature ft-ep, seq)`; then `ScopedChanges(feature ft-ep, after 0)` | bob | cursor at `seq`; only `ft-ep`'s events | — |
| 20 | `RevokeProjectGrant(EP, Payments)`; then `Initiative(in)` | alice, bob | ok; bob's view lists no `ft-ep` | `grant.revoked` |

Step 11 runs through `verifyPins`, so the real Hub's `/v1/refs` answer for `bf-ep@3` is re-hashed
on the client in the run; the step fails if the pin does not verify. Step 12 is the one place the
test reaches past the client: `hubclient` has no update-edit method (task 114 declares none), so
the test sends `PATCH /v1/updates/{updateId}` (registered by task 119 only to refuse,
`task-119.md:54`, `:260`) as a raw request through `w.raw(alice, "PATCH", …)`, the same signed
transport, to prove the Hub, not only the client, refuses it. That path has no relay row: the
desktop is never offered an edit. Step 11's kind `work.update_posted` is task 119's
(`task-119.md:181`). Step 17's hook is task 121's
`server.RunJanitorOnce(ctx, clk.Now())`, exported for tests only (**UNVERIFIED until task 121's
part names it**). Step 13 is Bob's read of Alice's initiative after step 5 spans it to his team;
the view lists only features in projects Bob may read (task 117's rule): `ft-ep` is readable through the grant; a
feature `ft-ck` in Checkout's ungranted project `CK`, joined to the initiative in the setup, is absent.

#### Tests to write first

In `methods_work_roadmap_test.go`, against a recording `httptest` Hub (task 16's `recordingHub`):

1. `TestEveryWorkRoadmapMethodHasARow` — every method of `WorkRoadmap` (by reflection) has at
   least one row through `roadmapRowsOf`, and every row is reached by one method call with
   placeholder ids. **Kills** "a method with no route" and "a row no method uses".
2. `TestRoadmapRoutesHitTheirHubPattern` — each call lands on its row's verb and pattern, with ids
   escaped (`ft-1/../x` stays one segment). **Kills** "a path hand-typed wrong" and "unescaped id".
3. `TestPostUpdateRefusesAPinWhoseDigestFails` — the fake Hub answers `POST …/updates` with a
   `StatusUpdate` pinned to `brief:bf-1@2`, and `/v1/refs` with that revision's content altered
   and its old digest; `PostUpdate` returns an error that `errors.Is` both `ErrUnavailable` and
   `ErrPinUnverified`, and a zero-value update is never used by the caller. **Kills** "digest
   re-check skipped".
4. `TestUpdatesRefusesTheWholePageOnOneBadPin` — a page of three with one bad pin is an error, not
   two updates. **Kills** "partial page shown".
5. `TestScopedChangesNeedsAScope` — a zero `ScopeRef` panics; a set one sends `scope=feature:ft-1`.
   **Kills** "the scoped row used as a whole feed".
5a. `TestBriefTemplateRowsAreClientOnly` — `CreateBriefTemplate` lands on `POST
   /v1/teams/{teamId}/templates` with `"kind":"brief"` in the body; `BriefTemplates` sends
   `?kind=brief`; none of the three rows is in `AllWorkRoutes()`. **Kills** "a duplicate relay
   pattern" (a `ServeMux` panic at start) and "issue templates decoded as briefs".

In `control-plane/internal/api/work_roadmap_test.go`, against a fake `hubclient.Work`:

6. `TestRoadmapRefusalsPassThrough` — for each of task 114's codes (`task-114.md:633-645`):
   `dependency_cycle` (422), `criterion_not_on_brief` (422), `update_immutable` (409),
   `grant_exists` (409), `initiative_not_found`, `milestone_not_found` and `grant_not_found` (404),
   a Hub problem with `details` is re-sent with the same status, `code`, `message` and `details`.
   **Kills** "a W4 code rewrapped as 502".
7. `TestRelayNeverForwardsAuthorization` — for every W4 row, a request carrying
   `Authorization: Bearer desktop-token` and `X-Hub-Token: x` reaches the fake with neither header
   visible (the fake records the context's outgoing headers from H33's transport hook). **Kills**
   "header passthrough".
8. `TestRoadmapBodiesAreStrict` — an unknown field in each W4 body is 400 `bad_request` and the Hub
   is not called. **Kills** "lenient decode".
9. `TestScopedChangesRelayNeedsScope` — `GET /v1/hub/work/changes` with no `scope` is 400 with
   `details.field = "scope"`; `?userId=u-bob` on `GET /v1/hub/work/me/read-cursors` never reaches
   the fake. **Kills** "a second unscoped feed" and "a cursor readable by others".
10. `TestW4RelayPathsAreExactlyTheTable` (above). **Kills** "an unaudited route".

In `conformance/work_roadmap_integration_test.go`: `TestWorkRoadmapConformance` runs
`conformance.WorkRoadmapRows` (task 115) against a `*hubclient.Client` on the real Hub, one
`t.Run` per `W4-NN` row. **Kills** "the fake and the Hub disagree".

In `work_roadmap_e2e_integration_test.go`, riding on the story:

11. `feed_is_metadata_only` — every event appended in steps 1-20 has a `data` whose keys are in
    SW.5's metadata list; no value is a string longer than 200 bytes, and the update body, the
    comment body and the initiative name never appear anywhere in the marshalled feed. **Kills**
    "text in the feed".
12. `granted_team_writes_nothing` — after step 1, every W1-W3 write as Bob on `EP` other than a
    comment is 403 and appends no event. **Kills** "grant is write".
13. `revocation_is_immediate` — after step 20, Bob's `ScopedChanges(project EP)` is 404
    `project_not_found`, not an empty page. **Kills** "revocation ignored" and "existence leak".
14. `janitor_actor_is_null` — step 17's `issue.created` has `actor: null` and the schedule's author
    in `data`. **Kills** "a system user as actor".

#### Steps

1. Write tests 1-5 and run them: they fail to compile (no `WorkRoadmapRoutes`).
2. Write the route table, `AllWorkRoutes`, `ErrPinUnverified` and the client methods; tests 1-5
   pass. Run `go vet ./control-plane/internal/hub/...`.
3. Write tests 6-10; they fail (no W4 handlers). Write `roadmapCalls`, `scopeParam`, `queryLimit`,
   `in1` and the two `registerWork` lines; add the audit rows; tests 6-10 pass.
4. Write the conformance runner and the end-to-end file with the story and checks 11-14.
5. `docker compose -f control-plane/cmd/hub/compose.yaml up -d`, then run the integration tests
   (Acceptance). A failing step prints the step, the caller and both kind lists.
6. `git add` the new files; commit with `git commit -F <msgfile> -- <paths>`.

#### Acceptance

```bash
go test ./control-plane/internal/hub/hubclient/... ./control-plane/internal/api/... -race -count=1
# expect: ok for each package; no FAIL line
go test -tags integration ./control-plane/internal/hub/hubclient/conformance/ -run TestWorkRoadmapConformance -race -count=1
# expect: one "--- PASS: TestWorkRoadmapConformance/W4-NN" per task 115 row, then "ok"
go test -tags integration ./control-plane/internal/hub/ -run TestWorkRoadmapE2E -race -count=1 -v
# expect, in order: one "--- PASS: TestWorkRoadmapE2E/NN_…" per step 0-20, then
# feed_is_metadata_only, granted_team_writes_nothing, revocation_is_immediate,
# janitor_actor_is_null, then "--- PASS: TestWorkRoadmapE2E" and "ok"
```

#### What this task does NOT claim

- A desktop surface: tasks 125 to 141 draw what these routes serve.
- Task 81's digest mail: step 17 proves the `update.reminder` event exists, not that a lead reads it.
- Any latency for `verifyPins` or the scoped feed (unmeasured).
- That a granted team can read the brief: task 117's Hub-contract proposal decides that.
- W2's relay rows: `AllWorkRoutes` makes room for task 82's table, which that part adds.

#### Gaps found while writing this part

1. **Task 120** must register task 114's two † paths, `PUT /v1/milestones/{milestoneId}/criteria`
   and `GET /v1/projects/{projectId}/feature-dependencies` (`task-114.md:724,727`, its gap 2),
   with matrix rows; this table relays exactly those.
2. **Task 114** has no `ScopedChangesPage{Events, Next, More, Truncated}`: task 123's scope
   answers `truncated: true` on a first look, which H's `ChangesPage` (`types.go:547-551`) lacks.
   This part uses that name; task 114 must add it additively, or task 123 must drop `truncated`.
3. **Task 114's** `WorkRoadmap` has no method for task 123's scoped `/v1/changes`. This part adds
   `ScopedChanges(ctx, ScopeRef, after, limit)` to `*Client` and calls it through `hubclient.Work`;
   task 114 must add it to the interface (a 32nd method) and task 115 to the fake, or the relay
   handler cannot compile against `Deps.Work`.
4. **Task 121** must export the janitor's one-run test hook; its scope names *"the test hook"* but
   no function.
5. **Task 115** names no event kinds; the kinds in the story table are this part's and must be
   copied from task 115's fake when it lands (task 125 copies them by name too).
6. **Task 82** (W2's relay) must pass `kind: "brief"` and the brief fields of task 78's template
   routes through to the Hub, and its `GET …/templates` must forward `?kind=`; this part's three
   brief-template methods are client-only and depend on it. If task 82 appends to `WorkRoutes`
   itself rather than adding a table, `AllWorkRoutes` is `WorkRoutes ++ WorkRoadmapRoutes` and
   nothing breaks.
7. **Task 117** has no caller-scoped list of the projects granted to the caller's teams (task
   125's gap 1). When task 117 adds one, this table gains a 32nd row and an eleventh
   audit line; `TestW4RelayPathsAreExactlyTheTable`'s count moves with it.
