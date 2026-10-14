### Task 82: `hubclient`'s W2 methods over HTTP, the relay rows, W2's Hub end to end

**Slice:** W2. **Size:** 3 focused hours (an estimate, not a measurement). **Streams:** WH and WP.
**Depends on** task 73 (SW.18 frozen: `Work` embeds `WorkPlan`, and the conformance rows `W-26` to
`W-43`) and tasks 75-81 (the W2 routes this proves). **Seam:** **SW.18**, consumed, not changed —
this task adds no method and no field; it gives the 46 methods task 72 declared an HTTP body, the
relay 46 rows, and the whole of W2 one end-to-end run.

Three things, in task 16's shape, one slice later. **First**, `methods_work_plan.go`: one client
method per `WorkPlan` method, on the exact verb and path of task 72's route rows, carried by H33's
transport so the idempotency key, the single refresh on `expired` and the 10 s timeout are not
written a second time. **Second**, the W2 rows of the `/v1/hub/work/*` relay: no new handler shape,
because task 16's `registerWork` loop already walks a route table; this task adds a second table
and a second call map, and the same panic guards hold both. **Third**,
`work_plan_e2e_integration_test.go`: the story of a team that plans, in one `-tags integration`
run against the real Hub, with `devissuer` tokens.

#### Anchors, re-read on 2026-09-27

All are parts on this branch or the Hub branch, none yet code. `task-72.md:872-958` (`WorkPlan`,
46 methods), `:961-995` (the route rows this task's `WorkPlanRoutes` copies, with their roles),
`:373-385` (`WorkPlanCodeStatus`, 19 codes); `task-73.md:106-124` (the files table, including
`client_work_plan_stub.go`, "task 82 deletes it"), `:60-64` (decision (d): `WorkEnv.Janitor`, and
"task 82 decides whether the real Hub gets a trigger or skips them"); `task-16.md:100-166` (the
`WorkRoute` struct, `Relay()`, `WorkRoutes`), `:168-418` (the client method idiom), `:419-493`
(the query rule and the digest re-check), `:494-560` (`Deps.Work`, `registerWork`, `relayWork`,
`workCall`, `errDecoded`); `task-17.md:46-92` (the world and the four people), `:93-135` (the
story, one `t.Run` per step), `:196-221` (the steps, including the skip when no Hub is reachable);
`task-05.md:103-212` (`RunWork`, `WorkEnv`). Hub contract: `§4.3:277-289` (the relay rule and
`hubclient.Error`), `§9.4` (the caller key never leaves the control plane), `§7` (the route list
the W2 rows are appended to at sync time).

**UNVERIFIED until merge:** H33's transport names (`doJSON`, `withIdempotency`, `refreshOnce`)
are read from the Hub branch's part, not from code; the real Hub's janitor trigger (decision (c)
below) needs the Hub owner's answer; `devissuer`'s W2 scopes are task 17's plus the team-admin
scope, which no part yet fixes.

#### Decisions the scope left open, made here

**(a) A second table, not a widened one.** `WorkRoutes` stays task 16's W1 rows. W2 gets
`WorkPlanRoutes`, the same `WorkRoute` type, in task 72's order. `registerWork` walks both, so a
row with no handler and a handler with no row are still a panic at `New`, and W1's table keeps the
line-for-line match with task 07 that `routes_audit_test.go` asserts. Two tables also make the
catalog diff readable: the audit file gains one block of 46 rows, not 46 lines interleaved.

**(b) The digest re-check runs on four W2 responses, not on every one.** Task 16 re-hashes every
response carrying an `IssueRevision` with content. In W2 only `PromoteToFeature`, `MoveIssue`,
`SetSubState` and `PatchIssuePlan` return one. The other 42 methods return records with no content
body, and re-hashing them would hash nothing and pass vacuously — worse than not calling it,
because it would read as proof. `methods_work_plan_test.go` names the four and fails if a fifth
method returns a revision without being added to the list.

**(c) The real Hub gets no janitor trigger; the four janitor rows skip and say so.** Task 73 (d)
left this to this task. A trigger route would be a test-only write path on a production surface,
which `OD-27`'s no-predictions rule does not cover but the Hub contract's route list does: every
row is a product route. So `WorkEnv.Janitor` is nil in the HTTP env, rows `W-33` to `W-36` skip
with `t.Skip("no janitor trigger on the real Hub; see task 82 (c)")`, and the end-to-end story
proves the janitor's *effect* a different way — it sets a cadence with cycles already due and
reads `Cycles` until the Hub's own sweep has run, with a bounded wait. Reported as a gap: if the
Hub gains an admin-only maintenance trigger, the four rows stop skipping.

**(d) The stub file dies in this commit.** `client_work_plan_stub.go` (task 73) is deleted in the
same commit that adds `methods_work_plan.go`, so `var _ Hub = (*Client)(nil)` never fails to
compile between them. Task 16 did the same to `client_work_stub.go`.

**(e) `CyclesICS` and `ExportIssuesCSV` return bytes, not JSON.** Three methods answer a
non-JSON body: `CyclesICS`, `ExportIssuesCSV`, `ExportFeaturesCSV`. They use H33's raw path
(`doRaw`, **UNVERIFIED**) and return `([]byte, string, error)` — body, content type, error — which
is what task 72 declared. The relay copies the content type through unchanged and does not parse
it; a relay that re-encoded a CSV as JSON is the mutant.

**(f) The read-only page and the calendar feed are not relayed.** Task 72 leaves `GET /cal/{token}.ics`
and `GET /p/issues/{issueId}` without a Go caller. They stay without one: both are browser URLs on
the Hub's own origin, and relaying them would put the control plane in the path of an unauthenticated
read. `routes_audit_test.go` records them as *not relayed, by decision*, so a later reader does not
read their absence as an oversight.

#### Files

| Path | Change |
|---|---|
| `control-plane/internal/hub/hubclient/methods_work_plan.go` | new: `WorkPlanRoutes` and the 46 `Client` methods |
| `control-plane/internal/hub/hubclient/methods_work_plan_test.go` | new: the table tests, the digest list, the header test |
| `control-plane/internal/hub/hubclient/client_work_plan_stub.go` | **deleted** (task 73 (d)) |
| `control-plane/internal/hub/hubclient/conformance/work_fake_test.go` ⚑ (tasks 05, 73) | unchanged; the HTTP env is the new file's |
| `control-plane/internal/hub/hubclient/conformance/work_http_test.go` ⚑ (task 16) | the W2 env: `RunWork` also runs `WorkRowsW2`, `Janitor` nil |
| `control-plane/internal/api/work.go` ⚑ (task 16) | `registerWork` walks `WorkPlanRoutes` too; `workPlanCalls()`, 46 entries |
| `control-plane/internal/api/work_plan_relay_test.go` | new: the relay's own tests (codes, headers, CSV passthrough) |
| `control-plane/internal/api/routes_audit_test.go` ⚑ | the 46 W2 catalog rows, plus the two not-relayed notes |
| `control-plane/internal/hub/work_plan_e2e_integration_test.go` | new: the story, `//go:build integration` |

The file map names no W2 end-to-end file; the name follows task 17's
(`work_e2e_integration_test.go`). Recorded under *Gaps*.

#### Contract — `WorkPlanRoutes`, complete

```go
// control-plane/internal/hub/hubclient/methods_work_plan.go
package hubclient

// WorkPlanRoutes is every W2 route, in task 72's order (task-72.md:964-993).
// The relay registers exactly these; methods_work_plan_test.go requires one
// Client method per row and one row per WorkPlan method.
var WorkPlanRoutes = []WorkRoute{
	{"CustomStates", http.MethodGet, "/v1/teams/{teamId}/states", false},
	{"PutCustomStates", http.MethodPut, "/v1/teams/{teamId}/states", false},
	{"SetSubState", http.MethodPut, "/v1/issues/{issueId}/sub-state", false},
	{"CreateFeature", http.MethodPost, "/v1/projects/{projectId}/features", true},
	{"Features", http.MethodGet, "/v1/projects/{projectId}/features", false},
	{"Feature", http.MethodGet, "/v1/features/{featureId}", false},
	{"PatchFeature", http.MethodPatch, "/v1/features/{featureId}", false},
	{"SetFeature", http.MethodPut, "/v1/issues/{issueId}/feature", false},
	{"PromoteToFeature", http.MethodPost, "/v1/issues/{issueId}/promote", true},
	{"Cadence", http.MethodGet, "/v1/teams/{teamId}/cadence", false},
	{"PutCadence", http.MethodPut, "/v1/teams/{teamId}/cadence", false},
	{"Cycles", http.MethodGet, "/v1/teams/{teamId}/cycles", false},
	{"PatchCycle", http.MethodPatch, "/v1/cycles/{cycleId}", false},
	{"SetCycle", http.MethodPut, "/v1/issues/{issueId}/cycle", false},
	{"ScopeLog", http.MethodGet, "/v1/cycles/{cycleId}/scope-log", false},
	{"ClosureProposals", http.MethodGet, "/v1/closure-proposals", false},
	{"AnswerClosure", http.MethodPost, "/v1/closure-proposals/{proposalId}/answer", false},
	{"MintCalendarToken", http.MethodPost, "/v1/me/calendar-token", true},
	{"RevokeCalendarToken", http.MethodDelete, "/v1/me/calendar-token", false},
	{"CyclesICS", http.MethodGet, "/v1/teams/{teamId}/cycles.ics", false},
	{"SetParent", http.MethodPut, "/v1/issues/{issueId}/parent", false},
	{"Templates", http.MethodGet, "/v1/teams/{teamId}/templates", false},
	{"CreateTemplate", http.MethodPost, "/v1/teams/{teamId}/templates", true},
	{"CreateOrgTemplate", http.MethodPost, "/v1/org/templates", true},
	{"PatchTemplate", http.MethodPatch, "/v1/templates/{templateId}", false},
	{"ArchiveTemplate", http.MethodPost, "/v1/templates/{templateId}/archive", false},
	{"LabelGroups", http.MethodGet, "/v1/teams/{teamId}/label-groups", false},
	{"CreateLabelGroup", http.MethodPost, "/v1/teams/{teamId}/label-groups", true},
	{"CreateOrgLabelGroup", http.MethodPost, "/v1/org/label-groups", true},
	{"CreateOrgLabel", http.MethodPost, "/v1/org/labels", true},
	{"PatchIssuePlan", http.MethodPatch, "/v1/issues/{issueId}/plan", false},
	{"RankIssue", http.MethodPost, "/v1/issues/{issueId}/rank", false},
	{"MoveIssue", http.MethodPost, "/v1/issues/{issueId}/move", false},
	{"CreateView", http.MethodPost, "/v1/teams/{teamId}/views", true},
	{"Views", http.MethodGet, "/v1/teams/{teamId}/views", false},
	{"View", http.MethodGet, "/v1/views/{viewId}", false},
	{"PatchView", http.MethodPatch, "/v1/views/{viewId}", false},
	{"ArchiveView", http.MethodPost, "/v1/views/{viewId}/archive", false},
	{"SetTeamDefault", http.MethodPut, "/v1/views/{viewId}/team-default", false},
	{"CreateDraft", http.MethodPost, "/v1/me/drafts", true},
	{"Drafts", http.MethodGet, "/v1/me/drafts", false},
	{"Draft", http.MethodGet, "/v1/me/drafts/{draftId}", false},
	{"PutDraft", http.MethodPut, "/v1/me/drafts/{draftId}", false},
	{"DeleteDraft", http.MethodDelete, "/v1/me/drafts/{draftId}", false},
	{"ExportIssuesCSV", http.MethodGet, "/v1/projects/{projectId}/issues.csv", false},
	{"ExportFeaturesCSV", http.MethodGet, "/v1/projects/{projectId}/features.csv", false},
}
```

Query strings are **not** in the pattern — `?state=` on `Cycles`, `?mine=true` on
`ClosureProposals`, `?viewId=` on both exports are set by the method, task 16's query rule
(`task-16.md:419-450`). `net/http`'s `ServeMux` ignores a query in a pattern, so a row carrying
one would register a route nothing could reach: the mutant, and
`TestWorkPlanRoutesCarryNoQuery` kills it.

#### Contract — the client methods, one of each kind, complete

The 46 are mechanical. Five kinds cover all of them; the rest are the same five with different
types, and the table test proves none was forgotten.

```go
// 1. A plain GET returning one record.
func (c *Client) Feature(ctx context.Context, featureID string) (hubapi.Feature, error) {
	var out hubapi.Feature
	err := c.doJSON(ctx, http.MethodGet, path("/v1/features/{featureId}", featureID), nil, &out)
	return out, err
}

// 2. A GET with a query and a page.
func (c *Client) Cycles(ctx context.Context, teamID string, state hubapi.CycleState,
	page hubapi.PageRequest) (hubapi.CyclePage, error) {
	q := url.Values{}
	if state != "" {
		q.Set("state", string(state))
	}
	page.Into(q) // task 16's query rule: cursor and limit, written once
	var out hubapi.CyclePage
	err := c.doJSON(ctx, http.MethodGet,
		path("/v1/teams/{teamId}/cycles", teamID)+queryOf(q), nil, &out)
	return out, err
}

// 3. A create: an idempotency key, and 201.
func (c *Client) CreateFeature(ctx context.Context, projectID string,
	req hubapi.CreateFeatureRequest) (hubapi.Feature, error) {
	var out hubapi.Feature
	err := c.doJSON(ctx, http.MethodPost, path("/v1/projects/{projectId}/features", projectID),
		withIdempotency(req, req.IdempotencyKey), &out)
	return out, err
}

// 4. A write whose response carries a revision: re-hash it (decision (b)).
func (c *Client) PromoteToFeature(ctx context.Context, issueID string,
	req hubapi.PromoteRequest) (hubapi.PromoteResult, error) {
	var out hubapi.PromoteResult
	if err := c.doJSON(ctx, http.MethodPost, path("/v1/issues/{issueId}/promote", issueID),
		withIdempotency(req, req.IdempotencyKey), &out); err != nil {
		return hubapi.PromoteResult{}, err
	}
	if err := checkRevisionDigest(out.Issue.Revision); err != nil {
		return hubapi.PromoteResult{}, err // ErrDigestMismatch, never the value
	}
	return out, nil
}

// 5. A raw body: bytes and a content type, never decoded (decision (e)).
func (c *Client) ExportIssuesCSV(ctx context.Context, projectID, viewID string) ([]byte, string, error) {
	q := url.Values{}
	if viewID != "" {
		q.Set("viewId", viewID)
	}
	return c.doRaw(ctx, http.MethodGet,
		path("/v1/projects/{projectId}/issues.csv", projectID)+queryOf(q))
}
```

`path`, `queryOf`, `doJSON`, `doRaw`, `withIdempotency` and `checkRevisionDigest` are task 16's
and H33's; none is written again here. `withIdempotency` returning the body unchanged when the
key is empty is H33's behaviour, and the Hub's own `409 idempotency_conflict` is the backstop.

#### Contract — the relay delta, complete

```diff
 func (a *API) registerWork(mux *routes) {
-	calls := workCalls()
-	for _, rt := range hubclient.WorkRoutes {
+	calls := workCalls()
+	for name, call := range workPlanCalls() {
+		if _, dup := calls[name]; dup {
+			panic("api: relay handler " + name + " is in both W1 and W2 call maps")
+		}
+		calls[name] = call
+	}
+	rows := append(append([]hubclient.WorkRoute{}, hubclient.WorkRoutes...),
+		hubclient.WorkPlanRoutes...)
+	for _, rt := range rows {
 		call, ok := calls[rt.Method]
```

Nothing else in `work.go` changes: `relayWork` already maps `hubclient.Error` to the same code,
status and details (hub-contract §4.3), already strips the local caller key before the outbound
request (§9.4), and already answers `503 hub_not_configured` when `Deps.Work` is nil, which now
covers 46 more routes for free. `workPlanCalls()` is 46 entries in `WorkPlanRoutes` order, each
four lines: decode if there is a body, read the path values, call, return the value.

One exception, and it is the only handler shape this task adds: the three raw rows. `workCall`
returns `(any, error)` and `relayWork` encodes it as JSON, so a `[]byte` would be base64. The
three raw rows are registered by `a.relayWorkRaw(rt, rawCall)`, twelve lines, which copies the
content type and the bytes through and shares `relayWork`'s error mapping by calling the same
`writeRelayError`. `TestRelayRawRowsAreExactlyThree` fails if a fourth appears without a decision.

#### Tests to write first

In `methods_work_plan_test.go`, all on `httptest`, no Hub:

| Test | The mutant it kills |
|---|---|
| `TestWorkPlanRoutesMatchWorkPlanInterface` | a method added to `WorkPlan` with no row, or a row with no method |
| `TestWorkPlanRoutesCarryNoQuery` | `"/v1/teams/{teamId}/cycles?state="` in a row: a route nothing reaches |
| `TestWorkPlanRoutesHaveNoW1Collision` | a W2 row whose `Relay()` equals a W1 row's |
| `TestEveryMethodSendsItsVerbAndPath` | `PatchFeature` sent as `PUT`; `SetCycle` to `/cycle/` |
| `TestCreatesSendAnIdempotencyKey` | the key dropped on a **retried** create — the test calls twice with one key and asserts one header value, not merely a non-empty one |
| `TestFourMethodsRecheckTheDigest` | the re-check skipped on the move path; also fails when a fifth method returns a revision |
| `TestTamperedMoveDigestIsRefused` | a `MoveIssue` response whose body was edited after hashing is returned as a value |
| `TestRawMethodsReturnBytesAndType` | a CSV decoded as JSON, or the content type dropped |
| `TestErrorsCarryCodeAndDetails` | a 409 flattened to `"hub error"`, losing `held_by_substate` |

In `work_plan_relay_test.go`, on the fake (`hubclient.NewFake()`), no Hub:

| Test | The mutant it kills |
|---|---|
| `TestRelayRegistersEveryW2Row` | a row with no handler — asserted as a live `New`, so the panic is the failure |
| `TestRelayKeepsTheHubCode` | the relay rewrites `held_by_substate` to `conflict`, or 409 to 500 |
| `TestRelayNeverForwardsTheCallerKey` | the local caller key reaches the Hub (§9.4); asserted on the outbound `http.Request`, not on a log line |
| `TestRelayAnswers503WithNoHub` | `Deps.Work` nil panics instead of `hub_not_configured` |
| `TestRelayRawRowsAreExactlyThree` | a fourth raw row slips in and is base64'd |
| `TestCatalogListsEveryW2Route` (in `routes_audit_test.go` ⚑) | a route shipped with no catalog row |

And the conformance rows, which are the bulk of the proof: `work_http_test.go` runs
`RunWork(t, env)` with `WorkRowsW2` against the HTTP client, `Janitor: nil`, so every one of task
73's rows `W-26` to `W-43` passes over HTTP or the slice does not land. Four skip, loudly
(decision (c)).

#### Contract — the end to end, one `t.Run` per step

`//go:build integration`. The world and the four people are task 17's
(`task-17.md:46-92`): the org admin, the team admin **Ada**, the member **Ben**, the outsider
**Cleo**, each a `devissuer` token. The story is the scope paragraph's, in order, and each step
asserts a fact, never a prediction.

| # | Step | The assertion |
|---|---|---|
| 1 | Ada sets a two-week cadence with a three-day cooldown | `PutCadence` 200; `Cadence` reads back the same three fields |
| 2 | The Hub's sweep creates cycles ahead | `Cycles(state=upcoming)` reaches the cadence's lookahead count within the bounded wait, or the step fails naming the wait |
| 3 | Ben starts an issue | `MoveIssueState` to `started` puts it in the **current** cycle, and the actor on `issue.cycle_set` is Ben, not the system |
| 4 | Ben promotes a big issue | `PromoteToFeature` returns `ft-`, a reserved branch, a brief key; the issue's `serves` is byte-identical to before, and it is `Features`' first child |
| 5 | Ada saves a view and makes it the team default | `CreateView` then `SetTeamDefault`; `Views` shows one default, and Ben's `Views` shows it too |
| 6 | Ben moves an issue to another team's project | `MoveIssue` 200; the **old** key still resolves through `IssueByNumber` and answers the new id |
| 7 | Cleo reads Ben's draft | `Draft` answers **404**, not 403 (L-33) |
| 8 | Ada exports the view | `ExportIssuesCSV(viewId=)` returns `text/csv` and the view's rows, in the view's order |
| 9 | The digest | the digest builder (task 81) names steps 3, 4 and 6, and names no step that did not happen |

Step 2's bounded wait is the one place this test may be slow: it polls `Cycles` every 2 s for at
most 60 s and fails with the elapsed time and the count it saw. It never sleeps a fixed amount and
then asserts, which is the flake the phase 2 run taught (`task-17.md:210-215`).

The whole file skips, with a reason, when `PANACEA_HUB_URL` is unset — task 17's guard, reused so
`go test ./...` on a laptop stays green and honest about what it did not run.

#### Steps

1. Write `methods_work_plan_test.go`'s nine tests against the absent `WorkPlanRoutes`. Run red.
2. Write `WorkPlanRoutes` and the 46 methods. Delete `client_work_plan_stub.go` in the same
   change. Run the nine green, and `go build ./...` green (the interface is still satisfied).
3. Point `work_http_test.go`'s env at `WorkRowsW2` with `Janitor: nil`. Run. Fix the client, never
   the row: a row that fails over HTTP but passes on the fake is this task's bug by default, and
   is only a route task's bug once the request bytes are shown to be right.
4. Write `work_plan_relay_test.go`'s six tests. Run red.
5. Write `workPlanCalls()`, the `registerWork` delta and `relayWorkRaw`. Run green.
6. Append the 46 catalog rows and the two not-relayed notes to `routes_audit_test.go`. Run.
7. Write `work_plan_e2e_integration_test.go`, nine `t.Run`s in order. Run against a real Hub with
   `-tags integration`. Record the run's output in the commit message, not a summary of it.

#### Acceptance

```
go test ./control-plane/internal/hub/hubclient/... -run 'WorkPlan|Digest|Idempotency' -count=1
  ok — and the -v output names 4 skipped conformance rows, all with the janitor reason

go test ./control-plane/internal/api/... -run 'Relay|Catalog' -count=1
  ok — TestRelayRegistersEveryW2Row constructs a live API with 46 more routes and does not panic

go build ./... && go vet ./control-plane/...
  no output; client_work_plan_stub.go is gone and nothing references it

PANACEA_HUB_URL=… go test ./control-plane/internal/hub/ -tags integration \
  -run TestWorkPlanEndToEnd -count=1 -v
  9 t.Run lines, all PASS, step 2 printing the seconds it waited
```

A failing fourth line is a real failure, not a skip: with `PANACEA_HUB_URL` set, the guard does
not fire, so "it skipped" cannot be reported as "it passed".

#### What this task does NOT claim

- **No desktop surface.** Tasks 83-96 own every screen. This task's proof stops at the relay's
  JSON.
- **No route behaviour.** Tasks 75-81 own the rules; a rule this task finds wrong is reported to
  the route task, and this part is not the place it is fixed.
- **Not the janitor, proved.** Four rows skip (decision (c)). Step 2 proves the Hub *does* create
  cycles ahead; it does not prove rollover, stale-proposal expiry or draft expiry over HTTP. Those
  stay proved on the fake only, and task 97 must record that, not launder it.
- **No performance claim.** The bounded wait is a timeout, not a measurement.
- **No parity row closed alone.** It is the proof path for the Hub halves of 75-81.

#### Gaps found while writing this part

1. **The file map names no W2 end-to-end file.** This part invents
   `control-plane/internal/hub/work_plan_e2e_integration_test.go` on task 17's pattern. The
   skeleton's file map should gain the row.
2. **No janitor trigger exists on the real Hub**, so four conformance rows can only skip. If the
   Hub owner adds an admin-only maintenance route, this part's decision (c) is superseded and the
   rows stop skipping. Until then, task 97's coverage note must say *fake only* for those four.
3. **`devissuer`'s W2 scopes are unfixed.** Step 1 needs a team-admin scope no part yet lists.
   Task 17's set plus team-admin is assumed here, **UNVERIFIED**.
4. **`doRaw` is assumed on H33's transport.** If H33 ships without it, decision (e)'s three
   methods need a raw path added there first, which is a Hub-branch change, not this one.
