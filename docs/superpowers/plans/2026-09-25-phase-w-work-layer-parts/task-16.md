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
- `control-plane/internal/hub/hubclient/methods_briefs.go` ⚑ (H34's) — one line: `Resolve`'s
  per-key re-check also calls `VerifyIssueRevision` for an `issue:` result
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

#### Contract — the query rule, and the re-check

A Work list's filters travel as a query string, and three parties read it: this client writes it,
the relay reads it from the desktop, and the Hub's routes (tasks 08 and 12) read it again. **One
rule, stated once:** the key is the field's `json` tag name, or, for a field with no tag, its Go
name with the first letter lowered (`hubapi.PageRequest`'s `After` and `Limit`, `types.go:202-205`,
become `after` and `limit`); a string is sent as is, an `int` in decimal, a `bool` only when true,
a `[]string` as one key per element; a zero value is omitted. `DecodeQuery` is the inverse and
refuses a key no field names.

```go
// control-plane/internal/hub/hubclient/query.go
package hubclient

// EncodeQuery writes each struct's exported fields as query parameters by the
// rule above. It accepts only structs of string, int, bool and []string
// fields; any other kind panics, so a new filter type fails its first test.
func EncodeQuery(parts ...any) url.Values

// DecodeQuery fills *T from q by the same rule. An unknown key, a repeated key
// for a scalar field, a non-decimal int, or a bool other than "true" and
// "false" is a *FieldError naming the key, which the relay answers 400
// bad_request with details.field. The same key in two parts is a panic at
// EncodeQuery, never a silent overwrite.
func DecodeQuery[T any](q url.Values) (T, error)

// FieldError is a query the rule refuses.
type FieldError struct {
	Field  string
	Reason string
}

func (e *FieldError) Error() string { return "query " + e.Field + ": " + e.Reason }
```

```go
// control-plane/internal/hub/hubclient/verify_work.go
package hubclient

// VerifyIssueRevision rebuilds the issue-revision canonical envelope from r's
// fields and content (hubapi.IssueRevisionDigest, task 02) and compares it
// with r.Digest. A mismatch is ErrDigestMismatch (H34's sentinel, one
// sentinel for both kinds); callers outside the reference resolver return it
// wrapped in ErrUnavailable. An erased revision (content null) is not
// checked: there is nothing left to check, and its resolution says so.
func VerifyIssueRevision(r hubapi.IssueRevision) error {
	if r.Content == nil {
		return nil
	}
	got, _, err := hubapi.IssueRevisionDigest(hubapi.IssueRevisionEnvelope{
		V: 1, Kind: "issue-revision",
		IssueID: r.IssueID, Revision: r.Revision, ProjectID: r.ProjectID,
		Content: *r.Content,
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDigestMismatch, err)
	}
	if got != r.Digest {
		return ErrDigestMismatch
	}
	return nil
}
```

`IssueRevisionDigest(env) (Digest, []byte, error)` and `IssueRevisionEnvelope{v, kind, issueId,
revision, projectId, content}` are task 02's (scope 02). The envelope names `projectId`, so the
re-check needs it on every revision the client receives: task 03's `IssueRevision` must carry
`projectId`, as `BriefRevision` does (`hubapi/types.go:172`). Scope 03's field list omits it; this
part relies on it being added (gap 5 below), and without it a revision read alone
(`IssueRevisionAt`) cannot be re-checked at all. `/v1/refs`
already carries issue revisions after task 09, and `Briefs.Resolve` is H34's: its per-key re-check
turns a failing revision into `missing`. This task adds the `issue:` case to that loop, one line in
H34's `methods_briefs.go` calling `VerifyIssueRevision`, so a tampered issue behind a `/v1/refs`
answer is `missing` too, not rendered.

#### Contract — the relay

`Deps` gains one optional field; `New`'s required-dependency loop (`api.go:295-300`) does **not**
list it, so a control plane with no Hub starts and answers `hub_not_configured` per route.

```go
// control-plane/internal/api/api.go ⚑ — in type Deps, after Log
	// Work is the Hub's Work surface (tasks 04, 16). Nil means no Hub is
	// configured — local mode without one (OD-20) — and every /v1/hub/work
	// route answers 503 hub_not_configured. It is never required by New.
	Work hubclient.Work

// in New, beside a.registerAttach(guarded) at api.go:341
	a.registerWork(guarded)
```

```go
// control-plane/internal/api/work.go
package api

// errDecoded means httpx.DecodeJSON has already written the 400.
var errDecoded = errors.New("api: request body refused and answered")

// workCall is one relay handler: decode, call, return the value to encode.
type workCall func(r *http.Request, w http.ResponseWriter, wk hubclient.Work) (any, error)

// registerWork mounts one relay route per hubclient.WorkRoutes row. A row with
// no handler, or a handler with no row, is a panic at New — so at every test.
func (a *API) registerWork(mux *routes) {
	calls := workCalls()
	for _, rt := range hubclient.WorkRoutes {
		call, ok := calls[rt.Method]
		if !ok {
			panic("api: no relay handler for hubclient.WorkRoutes row " + rt.Method)
		}
		delete(calls, rt.Method)
		mux.Handle(rt.Relay(), a.relayWork(rt, call))
	}
	for name := range calls {
		panic("api: relay handler " + name + " has no hubclient.WorkRoutes row")
	}
}

func (a *API) relayWork(rt hubclient.WorkRoute, call workCall) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := a.deps.Log
		if log == nil {
			log = slog.Default()
		}
		if a.deps.Work == nil {
			hubclient.WriteError(w, log, hubclient.ErrNotConfigured)
			return
		}
		v, err := call(r, w, a.deps.Work)
		var fe *hubclient.FieldError
		switch {
		case errors.Is(err, errDecoded):
			return
		case errors.As(err, &fe):
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
				fe.Error(), map[string]string{"field": fe.Field})
		case err != nil:
			hubclient.WriteError(w, log, err) // the relay rule, §4.3, unchanged
		case rt.Created:
			httpx.WriteJSON(w, http.StatusCreated, v)
		default:
			httpx.WriteJSON(w, http.StatusOK, v)
		}
	})
}

// body decodes a request body strictly (httpx.DecodeJSON: unknown fields and
// trailing content refused), or reports that the 400 is already written.
func body[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var v T
	if !httpx.DecodeJSON(w, r, &v) {
		return v, errDecoded
	}
	return v, nil
}

// pathInt reads {n} or {rev}: decimal, 1 to 999999999, as the key grammar's
// revision (task 02). Anything else is 400 before the Hub is called.
func pathInt(r *http.Request, name string) (int, error) {
	s := r.PathValue(name)
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 999_999_999 || s != strconv.Itoa(n) {
		return 0, &hubclient.FieldError{Field: name, Reason: "must be a whole number from 1"}
	}
	return n, nil
}
```

The handlers, one per row. Ids pass through verbatim; the client escapes them (`workPath`).

```go
// control-plane/internal/api/work.go (continued)
func workCalls() map[string]workCall {
	pv := func(r *http.Request, k string) string { return r.PathValue(k) }
	return map[string]workCall{
		"SetProjectKey": func(r *http.Request, w http.ResponseWriter, wk hubclient.Work) (any, error) {
			req, err := body[hubapi.SetProjectKeyRequest](w, r)
			if err != nil {
				return nil, err
			}
			return wk.SetProjectKey(r.Context(), pv(r, "projectId"), req.Key)
		},
		"CreateIssue": func(r *http.Request, w http.ResponseWriter, wk hubclient.Work) (any, error) {
			req, err := body[hubapi.CreateIssueRequest](w, r)
			if err != nil {
				return nil, err
			}
			return wk.CreateIssue(r.Context(), pv(r, "projectId"), req)
		},
		"Issues": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			q, err := hubclient.DecodeQuery[issuesQuery](r.URL.Query())
			if err != nil {
				return nil, err
			}
			return wk.Issues(r.Context(), pv(r, "projectId"), q.IssueQuery, q.PageRequest)
		},
		"IssueByNumber": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			n, err := pathInt(r, "n")
			if err != nil {
				return nil, err
			}
			return wk.IssueByNumber(r.Context(), pv(r, "projectId"), n)
		},
		"Issue": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			return wk.Issue(r.Context(), pv(r, "issueId"))
		},
		"PatchIssue": func(r *http.Request, w http.ResponseWriter, wk hubclient.Work) (any, error) {
			req, err := body[hubapi.PatchIssueRequest](w, r)
			if err != nil {
				return nil, err
			}
			return wk.PatchIssue(r.Context(), pv(r, "issueId"), req)
		},
		"WriteIssueRevision": func(r *http.Request, w http.ResponseWriter, wk hubclient.Work) (any, error) {
			req, err := body[hubapi.WriteIssueRevisionRequest](w, r)
			if err != nil {
				return nil, err
			}
			return wk.WriteIssueRevision(r.Context(), pv(r, "issueId"), req)
		},
		"IssueRevisions": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			p, err := hubclient.DecodeQuery[hubapi.PageRequest](r.URL.Query())
			if err != nil {
				return nil, err
			}
			return wk.IssueRevisions(r.Context(), pv(r, "issueId"), p)
		},
		"IssueRevisionAt": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			n, err := pathInt(r, "rev")
			if err != nil {
				return nil, err
			}
			return wk.IssueRevisionAt(r.Context(), pv(r, "issueId"), n)
		},
		"RestoreIssueRevision": func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
			n, err := pathInt(r, "rev")
			if err != nil {
				return nil, err
			}
			return wk.RestoreIssueRevision(r.Context(), pv(r, "issueId"), n)
		},
		"MoveIssueState": func(r *http.Request, w http.ResponseWriter, wk hubclient.Work) (any, error) {
			move, err := body[hubapi.StateMove](w, r)
			if err != nil {
				return nil, err
			}
			return wk.MoveIssueState(r.Context(), pv(r, "issueId"), move)
		},
		// …continued below
```

```go
		"AttachBlob": withBody(func(r *http.Request, wk hubclient.Work, req hubapi.AttachRequest) (any, error) {
			return wk.AttachBlob(r.Context(), r.PathValue("issueId"), req)
		}),
		"Attachments": noBody(func(r *http.Request, wk hubclient.Work) (any, error) {
			items, err := wk.Attachments(r.Context(), r.PathValue("issueId"))
			return listBody(items), err
		}),
		"AddRelation": withBody(func(r *http.Request, wk hubclient.Work, req hubapi.RelationRequest) (any, error) {
			return wk.AddRelation(r.Context(), r.PathValue("issueId"), req)
		}),
		"RemoveRelation": noBody(func(r *http.Request, wk hubclient.Work) (any, error) {
			return wk.RemoveRelation(r.Context(), r.PathValue("issueId"),
				hubapi.RelationKind(r.PathValue("kind")), r.PathValue("otherId"))
		}),
		"Labels": noBody(func(r *http.Request, wk hubclient.Work) (any, error) {
			items, err := wk.Labels(r.Context(), r.PathValue("teamId"))
			return listBody(items), err
		}),
		"CreateLabel": withBody(func(r *http.Request, wk hubclient.Work, req hubapi.CreateLabelRequest) (any, error) {
			return wk.CreateLabel(r.Context(), r.PathValue("teamId"), req)
		}),
		"ArchiveLabel": noBody(func(r *http.Request, wk hubclient.Work) (any, error) {
			return wk.ArchiveLabel(r.Context(), r.PathValue("labelId"))
		}),
		"TeamPolicy": noBody(func(r *http.Request, wk hubclient.Work) (any, error) {
			return wk.TeamPolicy(r.Context(), r.PathValue("teamId"))
		}),
		"PutTeamPolicy": withBody(func(r *http.Request, wk hubclient.Work, req hubapi.PutTeamPolicyRequest) (any, error) {
			return wk.PutTeamPolicy(r.Context(), r.PathValue("teamId"), req)
		}),
		"Comments": withQuery(func(r *http.Request, wk hubclient.Work, p hubapi.PageRequest) (any, error) {
			return wk.Comments(r.Context(), r.PathValue("issueId"), p)
		}),
		"CreateComment": withBody(func(r *http.Request, wk hubclient.Work, req hubapi.CreateCommentRequest) (any, error) {
			return wk.CreateComment(r.Context(), r.PathValue("issueId"), req)
		}),
		"React": withBody(func(r *http.Request, wk hubclient.Work, req hubapi.ReactRequest) (any, error) {
			return wk.React(r.Context(), r.PathValue("commentId"), req)
		}),
		"ResolveComment": withBody(func(r *http.Request, wk hubclient.Work, req hubapi.ResolveRequest) (any, error) {
			return wk.ResolveComment(r.Context(), r.PathValue("commentId"), req)
		}),
		"Subscribe": withBody(func(r *http.Request, wk hubclient.Work, req hubapi.SubscribeRequest) (any, error) {
			return wk.Subscribe(r.Context(), r.PathValue("issueId"), req)
		}),
		"Subscriptions": withQuery(func(r *http.Request, wk hubclient.Work, p hubapi.PageRequest) (any, error) {
			return wk.Subscriptions(r.Context(), p)
		}),
		"SearchIssues": withQuery(func(r *http.Request, wk hubclient.Work, q hubapi.IssueSearchQuery) (any, error) {
			items, err := wk.SearchIssues(r.Context(), q)
			return listBody(items), err
		}),
		"ImportBatch": withBody(func(r *http.Request, wk hubclient.Work, b hubapi.ImportBatch) (any, error) {
			return wk.ImportBatch(r.Context(), r.PathValue("projectId"), b)
		}),
		"EraseIssue": noBody(func(r *http.Request, wk hubclient.Work) (any, error) {
			return wk.EraseIssue(r.Context(), r.PathValue("issueId"))
		}),
		"EraseComment": noBody(func(r *http.Request, wk hubclient.Work) (any, error) {
			return wk.EraseComment(r.Context(), r.PathValue("commentId"))
		}),
	}
}

// withBody, withQuery and noBody are the three handler shapes; the first eleven
// rows above are written long-hand so a reader sees one expanded, and task 16's
// implementer may fold them into these three with no change in behaviour.
func withBody[T any](f func(*http.Request, hubclient.Work, T) (any, error)) workCall {
	return func(r *http.Request, w http.ResponseWriter, wk hubclient.Work) (any, error) {
		v, err := body[T](w, r)
		if err != nil {
			return nil, err
		}
		return f(r, wk, v)
	}
}

func withQuery[T any](f func(*http.Request, hubclient.Work, T) (any, error)) workCall {
	return func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) {
		q, err := hubclient.DecodeQuery[T](r.URL.Query())
		if err != nil {
			return nil, err
		}
		return f(r, wk, q)
	}
}

func noBody(f func(*http.Request, hubclient.Work) (any, error)) workCall {
	return func(r *http.Request, _ http.ResponseWriter, wk hubclient.Work) (any, error) { return f(r, wk) }
}

// listBody answers a method that returns a slice in the Hub's own list shape
// (contract §1: {"items":[…],"next":null}), so the relay's body is the Hub's
// body and an empty list is [] rather than null.
func listBody[T any](items []T) hubapi.Page[T] {
	if items == nil {
		items = []T{}
	}
	return hubapi.Page[T]{Items: items}
}

// issuesQuery is GET …/issues' query: the filter and the page, flattened.
// DecodeQuery reads an embedded struct's fields as if they were declared here.
type issuesQuery struct {
	hubapi.IssueQuery
	hubapi.PageRequest
}
```

`EncodeQuery` and `DecodeQuery` flatten an anonymous embedded struct's fields, which is what lets
`issuesQuery` be one query string; a key that two embedded structs both name panics at the first
call, so the collision cannot ship.

**The route audit.** Eleven relay routes are `GET`s. Each gets one line in
`catalogReadOnlyRoutes` (`routes_audit_test.go:32`), and one new test guards the map itself:

```go
// control-plane/internal/api/routes_audit_test.go ⚑ — appended to catalogReadOnlyRoutes
	"GET /v1/hub/work/projects/{projectId}/issues":                 workRelayRead,
	"GET /v1/hub/work/projects/{projectId}/issues/by-number/{n}":    workRelayRead,
	"GET /v1/hub/work/issues/{issueId}":                             workRelayRead,
	"GET /v1/hub/work/issues/{issueId}/revisions":                   workRelayRead,
	"GET /v1/hub/work/issues/{issueId}/revisions/{rev}":             workRelayRead,
	"GET /v1/hub/work/issues/{issueId}/attachments":                 workRelayRead,
	"GET /v1/hub/work/teams/{teamId}/labels":                        workRelayRead,
	"GET /v1/hub/work/teams/{teamId}/work-policy":                   workRelayRead,
	"GET /v1/hub/work/issues/{issueId}/comments":                    workRelayRead,
	"GET /v1/hub/work/me/subscriptions":                             workRelayRead,
	"GET /v1/hub/work/issues/search":                                workRelayRead,

// beside the map
const workRelayRead = "A relay of a Hub GET (task 16): one hubclient call, no local " +
	"state read or written; the Hub's own route is read-only by task 07's matrix."

// TestTheReadOnlyAllowlistHoldsNoWrite: a key of catalogReadOnlyRoutes whose
// method is not GET or HEAD is a write excused from nothing — the audit only
// ever consults the map for GETs — so it is refused here, by name.
func TestTheReadOnlyAllowlistHoldsNoWrite(t *testing.T) {
	for pattern := range catalogReadOnlyRoutes {
		method, _, _ := strings.Cut(pattern, " ")
		if method != http.MethodGet && method != http.MethodHead {
			t.Errorf("%q is in catalogReadOnlyRoutes but is a %s", pattern, method)
		}
	}
}
```

The existing `TestNoRegisteredGetCanChangeState` (`routes_audit_test.go:133`) then fails if a
twelfth `GET` row is added to `hubclient.WorkRoutes` without its line here, which is the point.

**The wiring.** `cmd/api/main.go` builds H33's client once for the Hub-backed handlers (Hub task
C1's wiring, H-plan task 32, *"the control plane's `/v1/auth/*`, its variables, its codes, the
wiring"*, **UNVERIFIED until merge**) and passes the same value:

```go
// control-plane/cmd/api/main.go ⚑ — in the api.Deps literal at main.go:147
		Work: hubWork, // nil when PANACEA_APP_HUB_URL is unset (OD-20); a *hubclient.Client otherwise
```

`hubWork` is declared `var hubWork hubclient.Work` and assigned only when the client is built, so
a nil `*hubclient.Client` never becomes a non-nil interface holding a nil pointer (the classic Go
trap that would make every route panic instead of answering `hub_not_configured`).

#### Tests to write first

`methods_work_test.go`, against an `httptest.Server` playing the Hub that records each request's
method, raw path (`r.URL.RawPath` or `EscapedPath()`), query and body, and answers from a script:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEveryWorkMethodUsesItsRoute` | for each `WorkRoutes` row, calling that method with ids `pj-…`, `is-…`, `ic-…`, `lb-…` records exactly `row.Verb` and the row's pattern filled with those ids | a path typo; a `PUT` where the Hub serves `POST`; two methods sharing one row |
| `TestEveryWorkIssuesMethodHasARow` | reflect over `reflect.TypeOf((*WorkIssues)(nil)).Elem()`: its method names equal the `Method` column as a set, and no two rows share a `Method` | a method added to task 04's interface with no route; a stale row |
| `TestAPathValueIsEscapedNotConcatenated` | `Issue(ctx, "is-1/../teams")` records the raw path `/v1/issues/is-1%2F..%2Fteams` | a `"/v1/issues/" + id` build that lets a desktop value walk the Hub's paths |
| `TestAPostCarriesOneIdempotencyKeyAcrossRetries` | the server answers `503` then `201`; `CreateIssue` records two attempts with the same `Idempotency-Key`, one `issue` created | a method that bypasses H33's transport and mints its own key |
| `TestATamperedRevisionIsAnErrorNotAValue` | the server serves revision 3 of `is-…` with a stored title altered after its digest was taken; `IssueRevisionAt`, `Issue` and `IssueRevisions` each return an error with `errors.Is(err, ErrUnavailable)` and `errors.Is(err, ErrDigestMismatch)` | a skipped re-check; a re-check on one method only |
| `TestAnErasedRevisionIsNotHashed` | `content: null` with its original digest → no error | a re-check that calls every erased issue tampered |
| `TestResolveMarksATamperedIssueMissing` | `/v1/refs` answering an `issue:` key whose revision fails the re-check → that result `missing`, one `error` log line naming the key | the `issue:` case missing from H34's loop |
| `TestTheQueryRuleRoundTrips` (`query_test.go`) | `DecodeQuery[issuesQuery](EncodeQuery(q, page))` equals the input for a table of filters; an unknown key, `limit=ten` and `limit=1&limit=2` are each a `*FieldError` naming the key | a lenient decoder; an encoder that drops a `[]string` element |

`work_test.go`, against the relay with `Deps.Work` set to a scripted stub (a `hubclient.Work` whose
methods return what the test says) or to `hubclient.NewFake()`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheRelayServesEveryRowAtItsDerivedPath` | for each row, the request `row.Relay()` with ids reaches the stub's method named by the row, with the same ids | a hand-typed relay path that drifts from the Hub's |
| `TestAHubPolicyRefusalPassesThroughUnchanged` | the stub returns `*hubclient.Error{422, policy_refused, details.policy: priorityToLeaveTriage}` for `MoveIssueState`; the desktop gets 422 and the same `error`, `message` and `details`, byte for byte | a relay that rewraps problems; a relay that drops `details` |
| `TestAHub500IsUpstreamUnavailable` | `ErrUnavailable` → 503, `error: upstream_unavailable`, `details.upstream: hub` | leaking the Hub's 500 to the desktop |
| `TestAHub401IsSignedOut` | `*Error{401, unauthenticated, reason: audience}` → 401 `signed_out` | a Hub rejection shown as a Work failure |
| `TestNoHubIsHubNotConfiguredAndTheProcessStarts` | `api.New` with `Deps.Work == nil` returns no error; every relay route answers 503 `hub_not_configured` | a startup failure in local mode with no Hub; a nil-pointer panic |
| `TestTheRelayDecodesStrictly` | `PATCH …/issues/{id}` with `{"state":"done"}` → 400 `bad_request` and the stub is never called | a lenient relay that forwards a smuggled state (the Hub also refuses; this is the first line) |
| `TestABadRevisionNumberNeverReachesTheHub` | `…/revisions/0`, `…/revisions/03` and `…/by-number/x` → 400 naming the field; stub not called | a relay that forwards garbage and lets the Hub word the error |
| `TestAWriteAnswers201WhereTheHubDoes` | `CreateIssue` → 201, `PatchIssue` → 200, `ImportBatch` → 200 | a relay that answers 200 to every write |
| `TestAnEmptyListIsAnEmptyArray` | the stub's `Labels` returns `nil` → body `{"items":[],"next":null}` | `null` reaching a desktop that indexes the list |

`routes_audit_test.go` ⚑ gains `TestTheReadOnlyAllowlistHoldsNoWrite` (above); adding
`"POST /v1/hub/work/issues/{issueId}/state"` to the map makes it fail (kills an unaudited write).

`conformance/work_integration_test.go` (`-tags integration`): H34's real-Hub harness, in-process
Hub on a scratch database with `devissuer` tokens, runs **every `W-NN` row of tasks 04-05** through
`hubclient.New` — this task's methods — instead of `NewFake()`. It kills fake-versus-real drift: a
rule the fake enforces and the Hub does not, or the reverse, fails a named row. This is the WS0
promise (skeleton *Start lines*) kept.

**One limit the relay inherits, said so no one rediscovers it.** `httpx.DecodeJSON` caps a body at
`MaxBodyBytes = 1 << 20` (`httpx/problem.go:22`) and sets `DisallowUnknownFields`. A desktop
`ImportBatch` through the relay is therefore at most 1 MiB. Task 25's importer does not use the
relay: it runs in the control plane, calls `Deps.Work.ImportBatch` in-process and splits a large
import into batches, so the cap never meets a real Linear export.

#### Steps

- [ ] **Step 1: confirm the sync base.** `git log --oneline -1` on the sync base, and
  `grep -n "func WriteError\|func Relay" control-plane/internal/hub/hubclient/relay.go` and
  `grep -n "ErrDigestMismatch" control-plane/internal/hub/hubclient/verify.go` each print one line
  (H33 and H34 are merged); `grep -n "type WorkIssues" control-plane/internal/hub/hubclient/work.go`
  prints one line (task 04). If any is empty, stop: this task waits (rule H2).
- [ ] **Step 2: take the merged names.** Read task 04's `WorkIssues` and replace this part's
  `ResolveComment`, `Attachments`, `IssueWithRevision`'s field names and `IssueRevisionDigest`'s
  arguments with the merged ones where they differ. Record each difference in the commit message.
- [ ] **Step 3: write `query_test.go` and `methods_work_test.go`.** Run
  `go test ./control-plane/internal/hub/hubclient/ -run 'Work|Query|Tampered|Erased|Escaped' -count=1`;
  expect a compile failure naming `WorkRoutes`, `EncodeQuery` and `VerifyIssueRevision`.
- [ ] **Step 4: write `query.go`, `verify_work.go`, `methods_work.go`, and the one line in
  `methods_briefs.go`.** Re-run step 3's command; expect `ok`.
- [ ] **Step 5: write `work_test.go` and the audit test.** Run
  `go test ./control-plane/internal/api/ -run 'Relay|Hub|Work|ReadOnly' -count=1`; expect a compile
  failure naming `registerWork` and `Deps.Work`.
- [ ] **Step 6: write `api/work.go`, the `Deps` field, the `registerWork` line and the eleven
  `catalogReadOnlyRoutes` lines.** Re-run step 5's command; expect `ok`. Then, as the check that
  the audit bites, add `"POST /v1/hub/work/issues/{issueId}/state": "x",` to the map, run
  `go test ./control-plane/internal/api/ -run ReadOnlyAllowlist -count=1`, see it fail naming that
  pattern, and delete the line.
- [ ] **Step 7: wire `main.go`.** `go build ./control-plane/...` passes; start the control plane with
  `PANACEA_APP_HUB_URL` unset and `curl -s http://127.0.0.1:8730/v1/hub/work/issues/is-3f2a91c07b1e4d2a`
  prints `{"error":"hub_not_configured",…}` with status 503.
- [ ] **Step 8: the conformance run against the real Hub** (step 9's second command).
- [ ] **Step 9: commit** `hubclient/{query,query_test,verify_work,methods_work,methods_work_test}.go`,
  `hubclient/methods_briefs.go`, `hubclient/conformance/work_integration_test.go`,
  `api/{work,work_test,api,routes_audit_test}.go` and `cmd/api/main.go`, by path.

#### Acceptance

```bash
go test ./control-plane/... -race -count=1
# expect: ok for every package; no FAIL line

go test ./control-plane/internal/hub/hubclient/conformance/ -tags integration -race -count=1 -run 'Work' -v
# expect: one "--- PASS: .../W-NN" line per row of tasks 04-05, then "ok"
```

#### What this task does NOT claim

- The desktop's calls (task 37, `src/api/work.ts`), Start (task 24) or the import adapters (task 25).
- That the relay authorises anything: the Hub does (task 07's matrix). The relay's only defences
  are Phase 1's loopback bind, `Host` guard and `Origin` allowlist, as for every route before
  Phase 8.
- A second change feed: Work events come on `/v1/hub/changes`, unchanged.
- W2-W5's methods (tasks 82, 100, 124, 168 add their rows to `WorkRoutes` the same way).
- Anything measured on a cloud run, or against a real Entra tenant.

Parity rows served: none closed alone. It carries every W1 Hub row to the desktop, and serves
**L-248** (the public API is the app's own API: the relay is the same routes, the same bodies).

#### Gaps found while writing this part (for task 04's owner and the skeleton's)

1. **`Resolve` collides.** Task 04's scope names a `WorkIssues.Resolve`; `Hub` also embeds
   `Briefs.Resolve(ctx, keys)` (`hubclient.go:63`), so `type Hub interface` would not compile. This
   part uses `ResolveComment`.
2. **No method reads `GET /v1/issues/{issueId}/attachments`** (task 07's route list). This part
   adds `Attachments(ctx, issueID) ([]hubapi.Attachment, error)`; task 05 should declare it.
3. **The query-string rule is nowhere fixed.** Tasks 08 and 12 parse list filters on the Hub; this
   part's `EncodeQuery`/`DecodeQuery` rule (the `json` tag name, else the lowered Go name) is the
   one both sides should use, and the real-Hub conformance run is what proves they agree.
4. **The File map omits** `hubclient/query.go`, `hubclient/verify_work.go` and the one-line edit to
   H34's `hubclient/methods_briefs.go`.
5. **`IssueRevision` has no `projectId`** in scope 03's field list, but the `issue-revision`
   envelope (scope 02) hashes it. Task 03 should add `ProjectID string "projectId"`, additively,
   as `BriefRevision` has it; otherwise no client can re-check a revision it reads on its own.

