### Task 16: `hubclient`'s Work methods over HTTP; the control plane's `/v1/hub/work/*` relay

W1, streams WH and WP, seam **SW.2** (its third part: the relay paths). Depends on task 05 (the
frozen `hubclient.Work` interface and the conformance rows), task 15 (the last W1 route, imports)
and Hub tasks H33-H34 (`hubclient` over HTTP: the transport, retries, `Idempotency-Key`, the single
refresh on `expired`, the relay rule, and the brief methods' digest re-check).

This task is the pipe between the desktop and the Hub for every W1 Work call. It produces two
things. The first is **the HTTP half of `hubclient.Work`**: one method per `WorkIssues` method,
each on the exact method and path of task 07's route list, carried by H33's transport so that the
idempotency key, the one refresh on `expired` and the 10 s timeout come for free and are not
written a second time. Every response that carries an `IssueRevision` with content is re-hashed
with `hubapi.IssueRevisionDigest` (task 02), and a mismatch is an error, never a value: that is the
brief methods' re-check (hub-contract §9.2) applied to issues, and it is the only thing that stops
a Hub that altered a stored revision from putting the altered text in front of a person or an agent.

The second is **the relay**. The desktop never talks to the Hub; it talks to its own loopback
control plane, which relays. The relay serves the Hub's Work routes under `/v1/hub/work/…`: the Hub
path with its leading `/v1` replaced by `/v1/hub/work`. `POST /v1/projects/{projectId}/issues`
becomes `POST /v1/hub/work/projects/{projectId}/issues`; `GET /v1/issues/search?q=` becomes
`GET /v1/hub/work/issues/search?q=`. Each handler is a thin call to `Deps.Work`: it decodes
strictly, passes Hub ids verbatim (no local-to-Hub id mapping here; `hublink`, Hub task 35, owns
that), and answers by **the relay rule** unchanged (hub-contract §4.3, `:277-295`).

**The one thing this task must not get wrong is that the relay passes a Hub refusal through
untouched.** A Hub 422 `policy_refused` whose `details.policy` names `priorityToLeaveTriage` is the
only way the desktop can tell a person *why* EP-142 did not leave triage. A relay that rewraps it
as a generic 502, or that drops `details`, turns every policy into "something went wrong". So: a
Hub 4xx problem is re-sent with the same status, `code`, `message` and `details`; a Hub 401 becomes
`signed_out`; a 5xx, a timeout or a refused connection becomes 503 `upstream_unavailable` with
`details.upstream = "hub"`; and a nil `Deps.Work` becomes 503 `hub_not_configured`. The last case
is local mode with no Hub (`OD-20`), and the process must still start.

The desktop's change feed stays `/v1/hub/changes` (hub-contract §9.3, `:1372`). This task adds no
second feed: Work events arrive on the one feed task 13 extended.

**Files:**
- `control-plane/internal/hub/hubclient/methods_work.go` — the W1 `WorkIssues` methods over HTTP,
  one table row per method (new)
- `control-plane/internal/hub/hubclient/query.go` and `query_test.go` — `EncodeQuery` and
  `DecodeQuery`, the one rule for a Work list's query string (new)
- `control-plane/internal/hub/hubclient/verify_work.go` — `VerifyIssueRevision`, the digest re-check
  for issues, beside H34's `verify.go` (new)
- `control-plane/internal/hub/hubclient/methods_work_test.go` — the route map against a recording
  `httptest` Hub, the re-check, path escaping (new)
- `control-plane/internal/hub/hubclient/conformance/work_integration_test.go` — task 05's `W-NN`
  rows against the real Hub, `-tags integration` (new)
- `control-plane/internal/api/work.go` — the `/v1/hub/work/*` relay, one table (new)
- `control-plane/internal/api/work_test.go` — the relay rule, the nil-`Work` case, strict decoding (new)
- `control-plane/internal/api/api.go` ⚑ — `Deps.Work hubclient.Work` (optional) and one line,
  `a.registerWork(guarded)`, beside `a.registerAttach(guarded)` at `api.go:341`
- `control-plane/internal/api/routes_audit_test.go` ⚑ — every relay `GET` in
  `catalogReadOnlyRoutes` (`routes_audit_test.go:32`), and one new test that no key of that map is
  a write
- `control-plane/cmd/api/main.go` ⚑ — hands the Hub client's `Work` to `api.Deps` (the
  `api.New(api.Deps{` literal at `main.go:147`); no new environment variable (`PANACEA_APP_WORK_*`
  begins in tasks 24-25)

**What the tree looks like today (verified 2026-09-25).** On this branch's base the control plane
has no Hub package at all: `control-plane/internal/` holds `api`, `backend`, `harness`, `httpx`,
`ids`, `instance`, `project`, `proxy`, `reaper`, `registry`, `sandbox`, `statepath`, `worktree`.
The Hub's `hubclient` is on the `feat/hub` tree (`.claude/worktrees/hub` at `9f2fa31`), where
`client.go` is still H04's stub (every method returns `ErrNotConfigured`, `client.go:17-134`) and
`hubclient.go:113-141` declares `Error`, `ErrUnavailable`, `ErrSignedOut` and `ErrNotConfigured`.
H33's `relay.go` (`WriteError`, `Relay`) and H34's `verify.go` (`VerifyRevision`,
`ErrDigestMismatch`) are **planned** (H-plan parts `task-33.md:20-120`, `task-34.md:40-56`) and
**UNVERIFIED until merge**. This task starts when both are on the sync base.

**Interfaces:**
- Produces:
  - `(*hubclient.Client)` implements `hubclient.WorkIssues` over HTTP — every method task 04 and
    task 05 declare; `var _ WorkIssues = (*Client)(nil)` makes a missing one a compile error
  - `hubclient.VerifyIssueRevision(r hubapi.IssueRevision) error`
  - `hubclient.WorkRoutes` — the method → Hub route table, exported so the relay and the tests read
    **one** list and cannot drift from each other
  - `api.Deps.Work hubclient.Work`, and every `/v1/hub/work/*` route in the table below
- Consumes: task 04's `hubclient.Work`, `WorkIssues` and `NewFake()`; task 03's `hubapi` W1 types;
  task 02's `hubapi.IssueRevisionDigest`; task 05's conformance rows; H33's transport, `WriteError`
  and `Relay`; H34's `ErrDigestMismatch` and its real-Hub conformance harness
  (`conformance/hub_integration_test.go`, H34 `task-34.md:13`); Phase 1's `routes` recorder
  (`api.go:247-271`) and `httpx.DecodeJSON`.
- Required by: task 19 (the Materialiser reads `IssueRevisionAt` through this client), tasks 21-25
  (the phase 3 deltas, Start and import call `Deps.Work`), task 37 (the desktop's `src/api/work.ts`
  calls these relay paths), tasks 55 and 98 (SW.2's later consumers).
- Amends nothing in hub-contract §4.3. `hub_not_configured` is §9.3's code (C1), written by H33's
  `WriteError(…, ErrNotConfigured)`; this task only makes the nil-`Work` case reach it.

**Two names this part fixes, for task 04's owner (see the gaps list at the end).** `Hub` embeds
both `Briefs` and `Work` (`hubclient.go:101`), and `Briefs` already has
`Resolve(ctx, keys []string) (hubapi.RefsResponse, error)` (`hubclient.go:63`; hub-contract
`:1301`). A `WorkIssues`
method also named `Resolve` with another signature makes `type Hub interface` fail to compile
("duplicate method Resolve"). This part calls the comment method **`ResolveComment`**. And task
07's route list has `GET /v1/issues/{issueId}/attachments`, which no method in task 04's list
reads; this part calls it **`Attachments`**. If task 04 or 05 merged other names, the table below
takes theirs and nothing else changes.

#### Contract — the route table, complete

One table serves the client, the relay and both tests. The Hub column is task 07's W1 route list
(appended to hub-contract §7 as rows 46 onward at sync time); the relay column is derived, never
typed. `Created` is the Hub's `201` rows, which the relay answers `201` too.

```go
// control-plane/internal/hub/hubclient/methods_work.go
package hubclient

// WorkRoute is one W1 WorkIssues method, its Hub route, and its relay twin.
type WorkRoute struct {
	Method  string // the WorkIssues method name, e.g. "MoveIssueState"
	Verb    string // net/http method constant
	Hub     string // the Hub's pattern, contract §7 rows 46+
	Created bool   // the Hub answers 201 Created on success
}

// Relay is the control plane's pattern for the same route: the Hub path with
// its leading "/v1" replaced by "/v1/hub/work". Derived, so it cannot drift.
func (r WorkRoute) Relay() string {
	return r.Verb + " /v1/hub/work" + strings.TrimPrefix(r.Hub, "/v1")
}

// WorkRoutes is every W1 route, in task 07's order. The relay registers
// exactly these; methods_work_test.go requires one Client method per row and
// one row per WorkIssues method.
var WorkRoutes = []WorkRoute{
	{"SetProjectKey", http.MethodPut, "/v1/projects/{projectId}/key", false},
	{"CreateIssue", http.MethodPost, "/v1/projects/{projectId}/issues", true},
	{"Issues", http.MethodGet, "/v1/projects/{projectId}/issues", false},
	{"IssueByNumber", http.MethodGet, "/v1/projects/{projectId}/issues/by-number/{n}", false},
	{"Issue", http.MethodGet, "/v1/issues/{issueId}", false},
	{"PatchIssue", http.MethodPatch, "/v1/issues/{issueId}", false},
	{"WriteIssueRevision", http.MethodPost, "/v1/issues/{issueId}/revisions", true},
	{"IssueRevisions", http.MethodGet, "/v1/issues/{issueId}/revisions", false},
	{"IssueRevisionAt", http.MethodGet, "/v1/issues/{issueId}/revisions/{rev}", false},
	{"RestoreIssueRevision", http.MethodPost, "/v1/issues/{issueId}/revisions/{rev}/restore", true},
	{"MoveIssueState", http.MethodPost, "/v1/issues/{issueId}/state", false},
	{"AttachBlob", http.MethodPost, "/v1/issues/{issueId}/attachments", true},
	{"Attachments", http.MethodGet, "/v1/issues/{issueId}/attachments", false},
	{"AddRelation", http.MethodPost, "/v1/issues/{issueId}/relations", false},
	{"RemoveRelation", http.MethodDelete, "/v1/issues/{issueId}/relations/{kind}/{otherId}", false},
	{"Labels", http.MethodGet, "/v1/teams/{teamId}/labels", false},
	{"CreateLabel", http.MethodPost, "/v1/teams/{teamId}/labels", true},
	{"ArchiveLabel", http.MethodPost, "/v1/labels/{labelId}/archive", false},
	{"TeamPolicy", http.MethodGet, "/v1/teams/{teamId}/work-policy", false},
	{"PutTeamPolicy", http.MethodPut, "/v1/teams/{teamId}/work-policy", false},
	{"Comments", http.MethodGet, "/v1/issues/{issueId}/comments", false},
	{"CreateComment", http.MethodPost, "/v1/issues/{issueId}/comments", true},
	{"React", http.MethodPost, "/v1/comments/{commentId}/reactions", false},
	{"ResolveComment", http.MethodPost, "/v1/comments/{commentId}/resolve", false},
	{"Subscribe", http.MethodPut, "/v1/issues/{issueId}/subscription", false},
	{"Subscriptions", http.MethodGet, "/v1/me/subscriptions", false},
	{"SearchIssues", http.MethodGet, "/v1/issues/search", false},
	{"ImportBatch", http.MethodPost, "/v1/projects/{projectId}/imports", false},
	{"EraseIssue", http.MethodPost, "/v1/issues/{issueId}/erase", false},
	{"EraseComment", http.MethodPost, "/v1/comments/{commentId}/erase", false},
}
```

Thirty rows: task 07's list, with `/v1/refs` (row 27) and `/v1/changes` (row 43) absent because
they are extended, not added, and already have their `Briefs.Resolve` and `Feed.Changes` methods.
`GET /v1/issues/search` and `GET /v1/issues/{issueId}` coexist under Go 1.22's `ServeMux` because a
literal segment is more specific than a wildcard; the relay inherits the same pair.
`ImportBatch` answers `200`, not `201`: a re-run creates nothing (SW.6, task 15), so one status
serves both runs and the body's `created` and `existing` say which.

#### Contract — the client methods

Every method is three lines over one helper. `work` looks the route up by method name (a
missing row panics at the first call, and `TestEveryWorkIssuesMethodHasARow` catches it before
that), fills the path, and hands H33's transport the verb and the **pattern**, so H33's error text
names the pattern and never an id. H33's part does not name its unexported per-call function; this
part calls it `c.send(ctx, verb, pattern, path, body, out)` and takes the merged name at sync
(**UNVERIFIED until merge**).

```go
// control-plane/internal/hub/hubclient/methods_work.go (continued)

var _ WorkIssues = (*Client)(nil)

// workPath fills a pattern's {…} segments in order, each with url.PathEscape.
// A path value arrives from the desktop already unescaped by ServeMux, so
// "is-1%2F..%2Fteams" reaches here as "is-1/../teams"; escaping it again is
// what keeps it one segment on the Hub. Never string concatenation.
func workPath(pattern string, args ...string) string {
	var b strings.Builder
	rest := pattern
	for _, a := range args {
		open := strings.IndexByte(rest, '{')
		shut := strings.IndexByte(rest, '}')
		if open < 0 || shut < open {
			panic("hubclient: more arguments than segments in " + pattern)
		}
		b.WriteString(rest[:open])
		b.WriteString(url.PathEscape(a))
		rest = rest[shut+1:]
	}
	if strings.IndexByte(rest, '{') >= 0 {
		panic("hubclient: fewer arguments than segments in " + pattern)
	}
	b.WriteString(rest)
	return b.String()
}

func workRoute(method string) WorkRoute {
	for _, r := range WorkRoutes {
		if r.Method == method {
			return r
		}
	}
	panic("hubclient: no WorkRoutes row for " + method)
}

// work is one call: the route by name, the path, an optional query, the body.
func (c *Client) work(ctx context.Context, method string, q url.Values, body, out any, args ...string) error {
	rt := workRoute(method)
	path := workPath(rt.Hub, args...)
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return c.send(ctx, rt.Verb, rt.Hub, path, body, out)
}

// checked re-hashes every revision a response carries. An erased revision
// (content null) is not checked, as H34's VerifyRevision does not.
func checked(err error, revs ...*hubapi.IssueRevision) error {
	if err != nil {
		return err
	}
	for _, r := range revs {
		if r == nil {
			continue
		}
		if verr := VerifyIssueRevision(*r); verr != nil {
			return fmt.Errorf("%w: %w (issue %s revision %d)", ErrUnavailable, verr, r.IssueID, r.Revision)
		}
	}
	return nil
}
```

The methods, part 1 (task 04's set). `IssueWithRevision` is read here as `{Head, Revision}`;
task 04 declares it, and its real field names replace these at sync (**UNVERIFIED until merge**).

```go
func (c *Client) SetProjectKey(ctx context.Context, projectID, key string) (hubapi.ProjectKey, error) {
	var out hubapi.ProjectKey
	return out, c.work(ctx, "SetProjectKey", nil, hubapi.SetProjectKeyRequest{Key: key}, &out, projectID)
}

func (c *Client) CreateIssue(ctx context.Context, projectID string, req hubapi.CreateIssueRequest) (hubapi.IssueWithRevision, error) {
	var out hubapi.IssueWithRevision
	err := c.work(ctx, "CreateIssue", nil, req, &out, projectID)
	return out, checked(err, &out.Revision)
}

func (c *Client) Issue(ctx context.Context, issueID string) (hubapi.IssueWithRevision, error) {
	var out hubapi.IssueWithRevision
	err := c.work(ctx, "Issue", nil, nil, &out, issueID)
	return out, checked(err, &out.Revision)
}

func (c *Client) IssueByNumber(ctx context.Context, projectID string, number int) (hubapi.IssueWithRevision, error) {
	var out hubapi.IssueWithRevision
	err := c.work(ctx, "IssueByNumber", nil, nil, &out, projectID, strconv.Itoa(number))
	return out, checked(err, &out.Revision)
}

func (c *Client) Issues(ctx context.Context, projectID string, q hubapi.IssueQuery, page hubapi.PageRequest) (hubapi.Page[hubapi.IssueHead], error) {
	var out hubapi.Page[hubapi.IssueHead]
	return out, c.work(ctx, "Issues", EncodeQuery(q, page), nil, &out, projectID)
}

func (c *Client) PatchIssue(ctx context.Context, issueID string, req hubapi.PatchIssueRequest) (hubapi.IssueHead, error) {
	var out hubapi.IssueHead
	return out, c.work(ctx, "PatchIssue", nil, req, &out, issueID)
}

func (c *Client) WriteIssueRevision(ctx context.Context, issueID string, req hubapi.WriteIssueRevisionRequest) (hubapi.IssueRevision, error) {
	var out hubapi.IssueRevision
	err := c.work(ctx, "WriteIssueRevision", nil, req, &out, issueID)
	return out, checked(err, &out)
}

func (c *Client) IssueRevisions(ctx context.Context, issueID string, page hubapi.PageRequest) (hubapi.Page[hubapi.IssueRevision], error) {
	var out hubapi.Page[hubapi.IssueRevision]
	err := c.work(ctx, "IssueRevisions", EncodeQuery(page), nil, &out, issueID)
	revs := make([]*hubapi.IssueRevision, len(out.Items))
	for i := range out.Items {
		revs[i] = &out.Items[i]
	}
	return out, checked(err, revs...)
}

func (c *Client) IssueRevisionAt(ctx context.Context, issueID string, rev int) (hubapi.IssueRevision, error) {
	var out hubapi.IssueRevision
	err := c.work(ctx, "IssueRevisionAt", nil, nil, &out, issueID, strconv.Itoa(rev))
	return out, checked(err, &out)
}

func (c *Client) RestoreIssueRevision(ctx context.Context, issueID string, rev int) (hubapi.IssueRevision, error) {
	var out hubapi.IssueRevision
	req := hubapi.RestoreRevisionRequest{FromRevision: rev}
	err := c.work(ctx, "RestoreIssueRevision", nil, req, &out, issueID, strconv.Itoa(rev))
	return out, checked(err, &out)
}

func (c *Client) MoveIssueState(ctx context.Context, issueID string, move hubapi.StateMove) (hubapi.IssueHead, error) {
	var out hubapi.IssueHead
	return out, c.work(ctx, "MoveIssueState", nil, move, &out, issueID)
}
```

An error from `c.work` leaves `out` zero, so `checked(err, …)` returns the error before it looks
at a revision; a zero `IssueRevision` is never hashed. `RestoreIssueRevision` sends
`fromRevision` in the body **and** the path, because task 03 declares the body and task 07 the
path; the Hub (task 09) answers 400 when they differ, and this client never sends them different.

The methods, part 2 (task 05's set). None of these returns revision content, so none re-checks.

```go
func (c *Client) AttachBlob(ctx context.Context, issueID string, req hubapi.AttachRequest) (hubapi.Attachment, error) {
	var out hubapi.Attachment
	return out, c.work(ctx, "AttachBlob", nil, req, &out, issueID)
}

func (c *Client) Attachments(ctx context.Context, issueID string) ([]hubapi.Attachment, error) {
	var out hubapi.Page[hubapi.Attachment]
	return out.Items, c.work(ctx, "Attachments", nil, nil, &out, issueID)
}

func (c *Client) AddRelation(ctx context.Context, issueID string, req hubapi.RelationRequest) (hubapi.IssueHead, error) {
	var out hubapi.IssueHead
	return out, c.work(ctx, "AddRelation", nil, req, &out, issueID)
}

func (c *Client) RemoveRelation(ctx context.Context, issueID string, kind hubapi.RelationKind, otherID string) (hubapi.IssueHead, error) {
	var out hubapi.IssueHead
	return out, c.work(ctx, "RemoveRelation", nil, nil, &out, issueID, string(kind), otherID)
}

func (c *Client) Labels(ctx context.Context, teamID string) ([]hubapi.Label, error) {
	var out hubapi.Page[hubapi.Label]
	return out.Items, c.work(ctx, "Labels", nil, nil, &out, teamID)
}

func (c *Client) CreateLabel(ctx context.Context, teamID string, req hubapi.CreateLabelRequest) (hubapi.Label, error) {
	var out hubapi.Label
	return out, c.work(ctx, "CreateLabel", nil, req, &out, teamID)
}

func (c *Client) ArchiveLabel(ctx context.Context, labelID string) (hubapi.Label, error) {
	var out hubapi.Label
	return out, c.work(ctx, "ArchiveLabel", nil, nil, &out, labelID)
}

func (c *Client) TeamPolicy(ctx context.Context, teamID string) (hubapi.TeamPolicy, error) {
	var out hubapi.TeamPolicy
	return out, c.work(ctx, "TeamPolicy", nil, nil, &out, teamID)
}

func (c *Client) PutTeamPolicy(ctx context.Context, teamID string, req hubapi.PutTeamPolicyRequest) (hubapi.TeamPolicy, error) {
	var out hubapi.TeamPolicy
	return out, c.work(ctx, "PutTeamPolicy", nil, req, &out, teamID)
}

func (c *Client) Comments(ctx context.Context, issueID string, page hubapi.PageRequest) (hubapi.Page[hubapi.Comment], error) {
	var out hubapi.Page[hubapi.Comment]
	return out, c.work(ctx, "Comments", EncodeQuery(page), nil, &out, issueID)
}

func (c *Client) CreateComment(ctx context.Context, issueID string, req hubapi.CreateCommentRequest) (hubapi.Comment, error) {
	var out hubapi.Comment
	return out, c.work(ctx, "CreateComment", nil, req, &out, issueID)
}

func (c *Client) React(ctx context.Context, commentID string, req hubapi.ReactRequest) (hubapi.Comment, error) {
	var out hubapi.Comment
	return out, c.work(ctx, "React", nil, req, &out, commentID)
}

func (c *Client) ResolveComment(ctx context.Context, commentID string, req hubapi.ResolveRequest) (hubapi.Comment, error) {
	var out hubapi.Comment
	return out, c.work(ctx, "ResolveComment", nil, req, &out, commentID)
}

func (c *Client) Subscribe(ctx context.Context, issueID string, req hubapi.SubscribeRequest) (hubapi.Subscription, error) {
	var out hubapi.Subscription
	return out, c.work(ctx, "Subscribe", nil, req, &out, issueID)
}

func (c *Client) Subscriptions(ctx context.Context, page hubapi.PageRequest) (hubapi.Page[hubapi.Subscription], error) {
	var out hubapi.Page[hubapi.Subscription]
	return out, c.work(ctx, "Subscriptions", EncodeQuery(page), nil, &out)
}

func (c *Client) SearchIssues(ctx context.Context, q hubapi.IssueSearchQuery) ([]hubapi.IssueHead, error) {
	var out hubapi.Page[hubapi.IssueHead]
	return out.Items, c.work(ctx, "SearchIssues", EncodeQuery(q), nil, &out)
}

func (c *Client) ImportBatch(ctx context.Context, projectID string, batch hubapi.ImportBatch) (hubapi.ImportResult, error) {
	var out hubapi.ImportResult
	return out, c.work(ctx, "ImportBatch", nil, batch, &out, projectID)
}

func (c *Client) EraseIssue(ctx context.Context, issueID string) (hubapi.IssueHead, error) {
	var out hubapi.IssueHead
	return out, c.work(ctx, "EraseIssue", nil, nil, &out, issueID)
}

func (c *Client) EraseComment(ctx context.Context, commentID string) (hubapi.Comment, error) {
	var out hubapi.Comment
	return out, c.work(ctx, "EraseComment", nil, nil, &out, commentID)
}
```

