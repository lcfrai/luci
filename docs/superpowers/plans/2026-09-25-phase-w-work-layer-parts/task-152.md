### Task 152: tracker sync, part 3: GitHub Issues, one way in

W5, stream WH, about 2.5 hours (an estimate, not a measurement). Depends on task 150 (the engine and
its `Adapter` seam; **scope only, no part on this tree**, names read from
`…-scopes/tasks-141-160.md:418-458`, **UNVERIFIED** against its part) and reuses task 151's
`worksync/text.go` (`PlainTextFromHTML`, `CutRunes`, `CutBytes`) and `worksync/fetch.go`
(`fetcher`, `PullError`, `TokenFunc`, the one `429` retry), `task-151.md` at *Interfaces*. Reads task
15's SW.6 rules (`task-15.md:167-292`), task 03's `ImportItem` (`task-03.md:543-552`), task 143's
`TrackerConnection` (scope `tasks-141-160.md:150-152`, no part) and task 169's GitHub target
`{owner, repository}` (`task-169.md:163`).

Closes **L-202**'s GitHub half, with tasks 151 and 171 (05 §1 L-202). No route, no table, no
migration: pure Go behind task 150's seam. Outside pull requests are **not** this task's: they are
tasks 154-155's, through host webhooks.

**What this task must not get wrong:** a pull request is not an issue, a rate limit loses nothing,
and the installation token never leaves `api.github.com`. A test kills each.

**Decisions made here, so nobody re-derives them:**

1. **The list call.** `GET /repos/{owner}/{repo}/issues?state=all&sort=updated&direction=asc
   &per_page=100[&since=<t>]`, following the `Link` header's `rel="next"` until 500 items
   (`hubapi.MaxImportItems`) or no `next`. The API version is pinned by a header, not a query:
   `X-GitHub-Api-Version: 2022-11-28` (`githubAPIVersion`), with `Accept:
   application/vnd.github+json`, both set by a `RoundTripper` so task 151's `fetcher` is not edited.
   Which versions GitHub serves on the day is **UNVERIFIED**.
2. **A `pull_request` key means skip.** GitHub lists pull requests in the issues endpoint; an
   element carrying `pull_request` (any value) is skipped, yet its `updated_at` still moves the
   cursor, so a repository busy with PRs does not re-read them forever.
3. **The cursor is `g1|<updated_at>`**, and `since` is inclusive, so the last second is re-read.
   SW.6's `(source, externalId)` idempotency absorbs that (`task-15.md:32-38`); the run's
   `existing` count shows it. The degenerate case, more than 500 elements updated in the one
   second the cursor names, cannot advance: the pull then keeps following `next` in the same call
   until `updated_at` moves, bounded at 2,000 elements, and hands back more than 500 items, which
   the engine must write in SW.6 pages (a gap for task 150, below). Past 2,000 in one second the
   pull stops with `tracker_stalled` and says so in the run.
4. **Upstream changes come from the repository's issue events**, not from the issue: `closed`,
   `reopened` → `state`; `assigned`, `unassigned` → `assignees`. `GET
   /repos/{owner}/{repo}/issues/events?per_page=100` is newest first with no `since`, so the pull
   reads pages until an event is older than the cursor, at most 5 pages; older changes are then not
   shown, which loses no item (a change is shown, never applied, 05 §5.4 W5). A first run reads no
   events: nothing had been imported to change.
5. **The external id is `owner/repo#number`**, so the same number in two repositories is two
   issues in one Hub project (the scope's rule). A repository rename or transfer changes the id
   and re-imports; the cost is accepted and recorded.
6. **A rate limit stops the pull and keeps nothing.** `403` or `429` with `x-ratelimit-remaining:
   0` is `*PullError{Code: "tracker_throttled", RetryAt: x-ratelimit-reset}`, and the items read on
   earlier pages of the same pull are dropped, not returned: the cursor stays and the next run reads
   them again. A `429` with `Retry-After` is task 151's fetcher's single wait-and-retry.
7. **The token never follows a link off-host.** A `next` URL whose scheme and host differ from the
   configured base is refused (`tracker_shape`) before any request, so a hostile `Link` header
   cannot collect the installation token.
8. **The credential is a GitHub App installation token**, minted from the app's private key held as
   a `secret_ref` (task 145's Key Vault proposal, not yet accepted). `GitHubAppToken` signs an RS256
   JWT with the standard library (no new dependency), exchanges it at `POST
   /app/installations/{id}/access_tokens`, and caches the token until five minutes before its
   `expires_at`. The app is installed on the organisation by its owner; that KPMG's Enterprise
   Managed Users organisation permits it is **UNVERIFIED** (an org-owner act).
9. **Markdown through the HTML sanitiser.** `body` is Markdown; the sanitiser drops raw HTML and
   decodes entities, so an `<img onerror>` is gone. It also drops a tag written inside a code span
   (`` `<div>` ``) and folds runs of blank lines. Both are accepted and recorded: imported text is
   a starting point a person edits, never evidence.

**Files:**

- `control-plane/internal/hub/worksync/github.go` — the adapter and `GitHubAppToken` (new)
- `control-plane/internal/hub/worksync/github_test.go` — its tests against `httptest` (new)
- `control-plane/internal/hub/worksync/testdata/github/*.json` — response shapes **written by us**
  from the fields read here, not copied from GitHub's documentation (new)

**Interfaces:**

- Produces: `worksync.NewGitHub(GitHubConfig) *GitHub`, `(*GitHub).Source() == "github"`,
  `(*GitHub).Pull`; `worksync.GitHubAppToken(GitHubAppConfig) TokenFunc`; `GitHubTarget`;
  `CodeTrackerStalled`
- Consumes: task 150's `Adapter`, `Cursor`, `Pulled`, `UpstreamChange`; task 151's `fetcher`,
  `PullError`, `TokenFunc`, `PlainTextFromHTML`, `CutRunes`, `CutBytes`, `CodeTracker*`; task 143's
  `hubapi.TrackerConnection`, `hubapi.ImportSourceGitHub`; task 03's `ImportItem`, `PriorityNone`
- Consumed by: task 150's engine; task 153 (comments on GitHub); task 155 (`GitHubAppToken` for PR
  linkbacks and autolinks, the same app)

#### Interfaces — complete: `worksync/github.go`

```go
package worksync

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

const (
	githubAPIVersion   = "2022-11-28" // decision 1: the one place the version lives
	githubBaseURL      = "https://api.github.com"
	githubCursorTag    = "g1"
	githubTieMax       = 2000 // decision 3's bound
	githubEventPages   = 5    // decision 4's bound
	CodeTrackerStalled = "tracker_stalled"
)

// GitHubTarget is TrackerConnection.Target for source github (task-169.md:163).
type GitHubTarget struct {
	Owner      string `json:"owner"`
	Repository string `json:"repository"`
}

var githubName = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9._-]{0,99})$`)

type GitHubConfig struct {
	Client  *http.Client
	Token   TokenFunc // GitHubAppToken in production (decision 8)
	BaseURL string
	Sleep   func(context.Context, time.Duration) error
	Now     func() time.Time
}

// GitHub is task 150's Adapter for GitHub Issues, one way in.
type GitHub struct {
	f    fetcher
	base *url.URL
}

// versionHeaders sets decision 1's two headers on every request it carries.
type versionHeaders struct{ next http.RoundTripper }

func (v versionHeaders) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Accept", "application/vnd.github+json")
	r.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	return v.next.RoundTrip(r)
}

func NewGitHub(c GitHubConfig) *GitHub {
	if c.BaseURL == "" {
		c.BaseURL = githubBaseURL
	}
	if c.Sleep == nil {
		c.Sleep = sleepCtx
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	base, err := url.Parse(strings.TrimRight(c.BaseURL, "/"))
	if err != nil {
		panic("worksync: github base url: " + err.Error()) // configuration, not input
	}
	tr := c.Client.Transport
	if tr == nil {
		tr = http.DefaultTransport
	}
	client := *c.Client
	client.Transport = versionHeaders{tr}
	return &GitHub{f: fetcher{client: &client, token: c.Token, sleep: c.Sleep, now: c.Now}, base: base}
}

func (g *GitHub) Source() string { return hubapi.ImportSourceGitHub }

// githubIssue is one element of the list: only what the mapping reads.
type githubIssue struct {
	Number      int              `json:"number"`
	Title       string           `json:"title"`
	Body        *string          `json:"body"`
	HTMLURL     string           `json:"html_url"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	User        struct{ Login string } `json:"user"`
	Labels      []struct{ Name string } `json:"labels"`
	PullRequest json.RawMessage  `json:"pull_request"`
}

// Pull reads issues updated since cur (decisions 1-3), then the events that
// show state and assignee changes (decision 4).
func (g *GitHub) Pull(ctx context.Context, conn hubapi.TrackerConnection, cur Cursor) (Pulled, error) {
	var t GitHubTarget
	if err := json.Unmarshal(conn.Target, &t); err != nil || !githubName.MatchString(t.Owner) ||
		!githubName.MatchString(t.Repository) {
		return Pulled{}, &PullError{Code: CodeTrackerRefused, Err: fmt.Errorf("github target: %v", err)}
	}
	since, err := parseGitHubCursor(cur)
	if err != nil {
		return Pulled{}, &PullError{Code: CodeTrackerShape, Err: err}
	}
	repo := t.Owner + "/" + t.Repository
	q := url.Values{"state": {"all"}, "sort": {"updated"}, "direction": {"asc"}, "per_page": {"100"}}
	if !since.IsZero() {
		q.Set("since", since.UTC().Format(time.RFC3339))
	}
	next := fmt.Sprintf("%s/repos/%s/%s/issues?%s", g.base, url.PathEscape(t.Owner), url.PathEscape(t.Repository), q.Encode())
	out := Pulled{Items: []hubapi.ImportItem{}, Upstream: []UpstreamChange{}, Next: cur}
	seen, last := 0, since
	for next != "" {
		if seen >= hubapi.MaxImportItems && last.After(since) {
			break // one page's worth read and the cursor moves (decision 3)
		}
		if seen >= githubTieMax {
			return Pulled{}, &PullError{Code: CodeTrackerStalled, Err: fmt.Errorf("over %d issues updated at %s", githubTieMax, since)}
		}
		var page []githubIssue
		var hdr http.Header
		if err := g.f.do(ctx, http.MethodGet, next, nil, &page, &hdr); err != nil {
			return Pulled{}, githubLimit(err, hdr) // decision 6: nothing kept
		}
		for _, is := range page {
			seen++
			if is.UpdatedAt.After(last) {
				last = is.UpdatedAt
			}
			if len(is.PullRequest) > 0 && string(is.PullRequest) != "null" {
				continue // decision 2
			}
			out.Items = append(out.Items, is.toImport(repo))
		}
		if next, err = g.nextLink(hdr.Get("Link")); err != nil {
			return Pulled{}, err
		}
	}
	if !last.IsZero() {
		out.Next = Cursor(githubCursorTag + "|" + last.UTC().Format(time.RFC3339))
	}
	if !since.IsZero() {
		if out.Upstream, err = g.events(ctx, t, repo, since); err != nil {
			return Pulled{}, err
		}
	}
	return out, nil
}

// toImport maps one issue to SW.6. GitHub has no priority, so none.
func (is githubIssue) toImport(repo string) hubapi.ImportItem {
	body := ""
	if is.Body != nil {
		body = *is.Body
	}
	labels := []string{}
	for _, l := range is.Labels {
		if n := strings.TrimSpace(CutRunes(strings.TrimSpace(l.Name), hubapi.MaxLabelNameChars)); n != "" &&
			len(labels) < hubapi.MaxImportLabels {
			labels = append(labels, n)
		}
	}
	created := ""
	if !is.CreatedAt.IsZero() {
		created = is.CreatedAt.UTC().Format(time.RFC3339)
	}
	return hubapi.ImportItem{
		ExternalID:  repo + "#" + strconv.Itoa(is.Number), // decision 5
		URL:         is.HTMLURL,                              // verbatim; SW.6 refuses non-https
		Title:       CutRunes(strings.TrimSpace(is.Title), hubapi.MaxIssueTitleChars),
		Description: CutBytes(PlainTextFromHTML(body), hubapi.MaxIssueDescriptionBytes),
		Priority:    hubapi.PriorityNone,
		Labels:      labels,
		AuthorName:  CutRunes(is.User.Login, hubapi.MaxImportAuthorNameChars),
		CreatedAt:   created,
	}
}

// githubLimit turns a primary rate limit into decision 6's error.
func githubLimit(err error, hdr http.Header) error {
	if hdr == nil || hdr.Get("X-Ratelimit-Remaining") != "0" {
		return err
	}
	pe := &PullError{Code: CodeTrackerThrottled, Err: err}
	if sec, e := strconv.ParseInt(hdr.Get("X-Ratelimit-Reset"), 10, 64); e == nil {
		pe.RetryAt = time.Unix(sec, 0).UTC()
	}
	return pe
}

var linkNext = regexp.MustCompile(`<([^>]+)>\s*;\s*rel="next"`)

// nextLink returns the rel="next" URL, "" at the end, or an error if the URL
// leaves the configured scheme and host (decision 7).
func (g *GitHub) nextLink(h string) (string, error) {
	m := linkNext.FindStringSubmatch(h)
	if m == nil {
		return "", nil
	}
	u, err := url.Parse(m[1])
	if err != nil || u.Scheme != g.base.Scheme || u.Host != g.base.Host {
		return "", &PullError{Code: CodeTrackerShape, Err: fmt.Errorf("next link off-host: %q", m[1])}
	}
	return u.String(), nil
}

type githubEvent struct {
	Event     string    `json:"event"`
	CreatedAt time.Time `json:"created_at"`
	Issue     struct {
		Number      int             `json:"number"`
		PullRequest json.RawMessage `json:"pull_request"`
	} `json:"issue"`
}

var githubEventField = map[string]string{"closed": "state", "reopened": "state",
	"assigned": "assignees", "unassigned": "assignees"}

// events reads the repository's issue events newest first until one is older
// than since, at most githubEventPages pages (decision 4).
func (g *GitHub) events(ctx context.Context, t GitHubTarget, repo string, since time.Time) ([]UpstreamChange, error) {
	out := []UpstreamChange{}
	next := fmt.Sprintf("%s/repos/%s/%s/issues/events?per_page=100", g.base, url.PathEscape(t.Owner), url.PathEscape(t.Repository))
	for p := 0; p < githubEventPages && next != ""; p++ {
		var page []githubEvent
		var hdr http.Header
		if err := g.f.do(ctx, http.MethodGet, next, nil, &page, &hdr); err != nil {
			return nil, githubLimit(err, hdr)
		}
		for _, e := range page {
			if !e.CreatedAt.After(since) {
				return out, nil
			}
			f, ok := githubEventField[e.Event]
			if !ok || (len(e.Issue.PullRequest) > 0 && string(e.Issue.PullRequest) != "null") {
				continue
			}
			out = append(out, UpstreamChange{ExternalID: repo + "#" + strconv.Itoa(e.Issue.Number), Field: f, At: e.CreatedAt})
		}
		var err error
		if next, err = g.nextLink(hdr.Get("Link")); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func parseGitHubCursor(c Cursor) (time.Time, error) {
	if c == "" {
		return time.Time{}, nil
	}
	tag, ts, ok := strings.Cut(string(c), "|")
	if !ok || tag != githubCursorTag {
		return time.Time{}, fmt.Errorf("not a github cursor: %q", c)
	}
	return time.Parse(time.RFC3339, ts)
}

var _ Adapter = (*GitHub)(nil)

// GitHubAppConfig names the app and one installation. Key reads the private
// key through the secret_ref seam (task 145's proposal); it is never a field.
type GitHubAppConfig struct {
	Client         *http.Client
	BaseURL        string
	AppID          int64
	InstallationID int64
	Key            func(ctx context.Context) (*rsa.PrivateKey, error)
	Now            func() time.Time
}

// GitHubAppToken is decision 8: an installation token, re-minted five
// minutes before it expires, one mint at a time.
func GitHubAppToken(c GitHubAppConfig) TokenFunc {
	if c.BaseURL == "" {
		c.BaseURL = githubBaseURL
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	var mu sync.Mutex
	var tok string
	var exp time.Time
	return func(ctx context.Context) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if tok != "" && c.Now().Before(exp.Add(-5*time.Minute)) {
			return tok, nil
		}
		key, err := c.Key(ctx)
		if err != nil {
			return "", err
		}
		jwt, err := githubAppJWT(c.AppID, key, c.Now())
		if err != nil {
			return "", err
		}
		f := fetcher{client: c.Client, token: func(context.Context) (string, error) { return jwt, nil },
			sleep: sleepCtx, now: c.Now}
		var r struct {
			Token     string    `json:"token"`
			ExpiresAt time.Time `json:"expires_at"`
		}
		u := fmt.Sprintf("%s/app/installations/%d/access_tokens", strings.TrimRight(c.BaseURL, "/"), c.InstallationID)
		if err := f.do(ctx, http.MethodPost, u, map[string]any{}, &r, nil); err != nil {
			return "", err
		}
		tok, exp = r.Token, r.ExpiresAt
		return tok, nil
	}
}

// githubAppJWT is an RS256 JWT: iat 60 s in the past for clock drift, exp
// nine minutes on (under GitHub's ten-minute ceiling), iss the app id.
func githubAppJWT(appID int64, key *rsa.PrivateKey, now time.Time) (string, error) {
	enc := base64.RawURLEncoding
	head := enc.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]int64{"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(), "iss": appID})
	signing := head + "." + enc.EncodeToString(claims)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signing + "." + enc.EncodeToString(sig), nil
}
```

**Two consequences, stated so nobody "fixes" them.** An issue whose `pull_request` key is
present but `null` is an issue (the key's value decides, not its presence in the raw bytes; the
test sends both forms). And the events read (decision 4) uses the same token and counts against the
same rate limit as the list; a rate limit there discards the whole pull too, since a pull that
returned items without their changes would advance the cursor past changes nobody saw.

#### Wire shapes — the calls the adapter makes (shapes, not copies of any documentation)

```text
GET {base}/repos/acme/web/issues?direction=asc&per_page=100&since=2026-09-25T11%3A00%3A00Z&sort=updated&state=all
  X-GitHub-Api-Version: 2022-11-28 · Accept: application/vnd.github+json · Authorization: Bearer <installation token>
  ← 200 [{"number": 12, "title": "Resume upload fails", "body": "Steps <img src=x onerror=alert(1)>",
          "html_url": "https://github.com/acme/web/issues/12", "created_at": "2026-09-20T09:00:00Z",
          "updated_at": "2026-09-25T11:03:00Z", "user": {"login": "octo-person"},
          "labels": [{"name": "bug"}]},
         {"number": 13, "pull_request": {"url": "…"}, "updated_at": "2026-09-25T11:04:00Z", …}]
    Link: <{base}/repositories/1/issues?page=2&…>; rel="next"

GET {base}/repos/acme/web/issues/events?per_page=100
  ← 200 [{"event": "closed", "created_at": "2026-09-25T11:02:00Z", "issue": {"number": 12}}]

POST {base}/app/installations/42/access_tokens      Authorization: Bearer <app JWT>
  ← 201 {"token": "<installation token>", "expires_at": "2026-09-25T12:02:00Z"}
```

The first element becomes `{"externalId": "acme/web#12", "url":
"https://github.com/acme/web/issues/12", "title": "Resume upload fails", "description": "Steps",
"priority": "none", "labels": ["bug"], "authorName": "octo-person", "createdAt":
"2026-09-20T09:00:00Z"}`, and the event becomes `UpstreamChange{ExternalID: "acme/web#12", Field:
"state", At: 11:02}`.

#### Tests to write first — `worksync/github_test.go` (complete)

```go
package worksync

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// ghServer serves pages[path][page] as JSON, adds a Link to page+1 while one
// exists, and lets hook answer first. It records every request's headers.
type ghServer struct {
	pages map[string][]string
	hook  func(w http.ResponseWriter, r *http.Request) bool
	seen  []http.Header
	srv   *httptest.Server
}

func (s *ghServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.seen = append(s.seen, r.Header.Clone())
	if s.hook != nil && s.hook(w, r) {
		return
	}
	pages, ok := s.pages[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	p := 0
	fmt.Sscan(r.URL.Query().Get("page"), &p)
	if p+1 < len(pages) {
		w.Header().Set("Link", fmt.Sprintf(`<%s%s?page=%d>; rel="next"`, s.srv.URL, r.URL.Path, p+1))
	}
	w.Write([]byte(pages[p]))
}

func newTestGitHub(t *testing.T, s *ghServer) *GitHub {
	t.Helper()
	s.srv = httptest.NewServer(s)
	t.Cleanup(s.srv.Close)
	return NewGitHub(GitHubConfig{Client: s.srv.Client(), BaseURL: s.srv.URL,
		Token: func(context.Context) (string, error) { return "inst", nil },
		Sleep: func(context.Context, time.Duration) error { return nil }})
}

func ghConn(repo string) hubapi.TrackerConnection {
	return hubapi.TrackerConnection{ID: "tc-2", Source: hubapi.ImportSourceGitHub, Mode: "in",
		Target: json.RawMessage(`{"owner":"acme","repository":"` + repo + `"}`)}
}

func ghIssue(n int, extra string) string {
	return fmt.Sprintf(`{"number":%d,"title":"t%d","body":"b","html_url":"https://github.com/acme/web/issues/%d",`+
		`"created_at":"2026-09-20T09:00:00Z","updated_at":"2026-09-25T11:%02d:00Z","user":{"login":"o"},"labels":[]%s}`, n, n, n, n%60, extra)
}

// Kills "PRs imported as issues"; a null pull_request is still an issue.
func TestGitHubSkipsPullRequests(t *testing.T) {
	s := &ghServer{pages: map[string][]string{"/repos/acme/web/issues": {
		"[" + ghIssue(1, "") + "," + ghIssue(3, `,"pull_request":null`) + "," + ghIssue(4, `,"pull_request":{"url":"x"}`) + "]"}}}
	got, err := newTestGitHub(t, s).Pull(context.Background(), ghConn("web"), "")
	if err != nil || len(got.Items) != 2 || got.Items[1].ExternalID != "acme/web#3" {
		t.Fatalf("err %v, items %+v", err, got.Items)
	}
	if got.Next != "g1|2026-09-25T11:04:00Z" { // the last element is the PR: its time still moves the cursor
		t.Fatalf("next %q", got.Next)
	}
	if h := s.seen[0]; h.Get("X-GitHub-Api-Version") != githubAPIVersion || h.Get("Authorization") != "Bearer inst" {
		t.Fatalf("headers %v", h) // kills "version not pinned"
	}
}

// Kills "first page only".
func TestGitHubFollowsEveryLinkPage(t *testing.T) {
	s := &ghServer{pages: map[string][]string{"/repos/acme/web/issues": {
		"[" + ghIssue(1, "") + "]", "[" + ghIssue(2, "") + "]", "[" + ghIssue(3, "") + "]"}}}
	got, err := newTestGitHub(t, s).Pull(context.Background(), ghConn("web"), "")
	if err != nil || len(got.Items) != 3 {
		t.Fatalf("err %v, %d items", err, len(got.Items))
	}
}

// Kills "number collides across repos".
func TestGitHubIDsAreScopedByRepository(t *testing.T) {
	s := &ghServer{pages: map[string][]string{"/repos/acme/web/issues": {"[" + ghIssue(5, "") + "]"},
		"/repos/acme/api/issues": {"[" + ghIssue(5, "") + "]"}}}
	g := newTestGitHub(t, s)
	a, _ := g.Pull(context.Background(), ghConn("web"), "")
	b, _ := g.Pull(context.Background(), ghConn("api"), "")
	if a.Items[0].ExternalID != "acme/web#5" || b.Items[0].ExternalID != "acme/api#5" {
		t.Fatalf("ids %q and %q", a.Items[0].ExternalID, b.Items[0].ExternalID)
	}
}

// Kills "state imported": a close is an UpstreamChange; the item has no state.
func TestGitHubCloseIsUpstreamOnly(t *testing.T) {
	s := &ghServer{pages: map[string][]string{
		"/repos/acme/web/issues": {"[" + ghIssue(12, `,"state":"closed","assignees":[{"login":"z"}]`) + "]"},
		"/repos/acme/web/issues/events": {`[{"event":"closed","created_at":"2026-09-25T11:02:00Z","issue":{"number":12}},` +
			`{"event":"labeled","created_at":"2026-09-25T11:01:00Z","issue":{"number":12}},` +
			`{"event":"assigned","created_at":"2026-09-24T08:00:00Z","issue":{"number":12}}]`}}}
	got, err := newTestGitHub(t, s).Pull(context.Background(), ghConn("web"), "g1|2026-09-25T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	want := UpstreamChange{ExternalID: "acme/web#12", Field: "state", At: time.Date(2026, 9, 25, 11, 2, 0, 0, time.UTC)}
	if len(got.Upstream) != 1 || !got.Upstream[0].At.Equal(want.At) || got.Upstream[0].Field != want.Field {
		t.Fatalf("upstream %+v", got.Upstream) // the older "assigned" is before the cursor
	}
	raw, _ := json.Marshal(got.Items[0])
	if strings.Contains(string(raw), "closed") || strings.Contains(string(raw), `"z"`) {
		t.Fatalf("state or assignee leaked: %s", raw)
	}
}

// Kills "rate limit loses items": page 2 is limited; nothing is returned and
// the reset time is carried for SyncRun.errors.
func TestGitHubRateLimitKeepsNothing(t *testing.T) {
	s := &ghServer{pages: map[string][]string{"/repos/acme/web/issues": {"[" + ghIssue(1, "") + "]", "[]"}}}
	s.hook = func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Query().Get("page") != "1" {
			return false
		}
		w.Header().Set("X-Ratelimit-Remaining", "0")
		w.Header().Set("X-Ratelimit-Reset", "1790000000")
		w.WriteHeader(http.StatusForbidden)
		return true
	}
	got, err := newTestGitHub(t, s).Pull(context.Background(), ghConn("web"), "g1|2026-09-25T10:00:00Z")
	var pe *PullError
	if !errors.As(err, &pe) || pe.Code != CodeTrackerThrottled || !pe.RetryAt.Equal(time.Unix(1790000000, 0)) {
		t.Fatalf("err %v", err)
	}
	if len(got.Items) != 0 || got.Next != "" {
		t.Fatalf("a limited pull returned %d items, next %q", len(got.Items), got.Next)
	}
}

// Kills "HTML imported".
func TestGitHubBodyHoldsNoTag(t *testing.T) {
	s := &ghServer{pages: map[string][]string{"/repos/acme/web/issues": {
		"[" + strings.Replace(ghIssue(1, ""), `"body":"b"`, `"body":"Steps <img src=x onerror=\"alert(1)\">"`, 1) + "]"}}}
	got, err := newTestGitHub(t, s).Pull(context.Background(), ghConn("web"), "")
	if err != nil || got.Items[0].Description != "Steps" {
		t.Fatalf("err %v, description %q", err, got.Items[0].Description)
	}
}

// Kills "token sent to another host" (decision 7).
func TestGitHubRefusesOffHostNextLink(t *testing.T) {
	s := &ghServer{pages: map[string][]string{"/repos/acme/web/issues": {"[" + ghIssue(1, "") + "]"}}}
	s.hook = func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("Link", `<https://collector.example/steal?page=2>; rel="next"`)
		w.Write([]byte("[" + ghIssue(1, "") + "]"))
		return true
	}
	_, err := newTestGitHub(t, s).Pull(context.Background(), ghConn("web"), "")
	var pe *PullError
	if !errors.As(err, &pe) || pe.Code != CodeTrackerShape || len(s.seen) != 1 {
		t.Fatalf("err %v after %d requests", err, len(s.seen))
	}
}

// Kills "a token per request" and "JWT not RS256 by the app": two calls mint
// once; the JWT names the app and verifies under the app's public key.
func TestGitHubAppTokenMintsOnceAndSigns(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	mints := 0
	s := &ghServer{pages: map[string][]string{}}
	s.hook = func(w http.ResponseWriter, r *http.Request) bool {
		mints++
		jwt := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if err := verifyRS256(jwt, &key.PublicKey, 7); err != nil || r.URL.Path != "/app/installations/42/access_tokens" {
			http.Error(w, "bad", http.StatusUnauthorized)
			return true
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"token":"inst-1","expires_at":"2026-09-25T13:00:00Z"}`))
		return true
	}
	s.srv = httptest.NewServer(s)
	defer s.srv.Close()
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	tok := GitHubAppToken(GitHubAppConfig{Client: s.srv.Client(), BaseURL: s.srv.URL, AppID: 7, InstallationID: 42,
		Key: func(context.Context) (*rsa.PrivateKey, error) { return key, nil }, Now: func() time.Time { return now }})
	for i := 0; i < 2; i++ {
		if got, err := tok(context.Background()); err != nil || got != "inst-1" {
			t.Fatalf("call %d: %q, %v", i, got, err)
		}
	}
	if now = now.Add(56 * time.Minute); mints != 1 {
		t.Fatalf("minted %d times for two calls", mints)
	}
	tok(context.Background()) // four minutes from expiry: re-minted
	if mints != 2 {
		t.Fatalf("not re-minted near expiry: %d", mints)
	}
}
```

`verifyRS256(jwt, pub, iss)` is a twelve-line test helper in the same file: split on `.`, check the
header is `{"alg":"RS256","typ":"JWT"}`, `rsa.VerifyPKCS1v15` over `sha256(head.claims)`, decode the
claims and compare `iss`. It is test code only; the product never verifies a JWT here.

#### Testdata — `worksync/testdata/github/` (complete, ours)

`issues-page.json` holds the Wire shapes section's two-element list (the issue and the PR, with the
PR's `…` filled as `"title": "PR", "user": {"login": "o"}, "labels": []`), and `events-page.json`
its one event. `TestGitHubRecordedShape`, beside the tests above, serves both files and asserts the
issue maps to the exact `ImportItem` JSON in that section and the event to the one
`UpstreamChange`, as task 151's `TestBoardsRecordedShape` does.

#### Mutants, applied by hand before the code is final

| Mutant | Test that fails |
|---|---|
| the `pull_request` check removed | `TestGitHubSkipsPullRequests` |
| a `pull_request: null` element skipped too | `TestGitHubSkipsPullRequests` |
| `nextLink` always returns `""` | `TestGitHubFollowsEveryLinkPage` |
| `ExternalID` is `strconv.Itoa(is.Number)` | `TestGitHubIDsAreScopedByRepository` |
| `events` maps `labeled` to `state`, or reads past the cursor | `TestGitHubCloseIsUpstreamOnly` |
| `githubLimit` returns `err` unchanged | `TestGitHubRateLimitKeepsNothing` |
| the pull returns the items read before the limit | `TestGitHubRateLimitKeepsNothing` |
| `toImport` uses `body` raw | `TestGitHubBodyHoldsNoTag` |
| `nextLink` skips the host check | `TestGitHubRefusesOffHostNextLink` |
| `GitHubAppToken` ignores its cache | `TestGitHubAppTokenMintsOnceAndSigns` |

#### Steps

1. Confirm tasks 150, 151 and 143 are on the sync base (`worksync/engine.go`, `text.go`,
   `fetch.go`, `hubapi.ImportSourceGitHub`); follow landed names, or wait (rule H1).
2. Write `github_test.go` and the testdata; the package fails to compile (no `GitHub`).
3. Write `github.go`; run the package green; apply each mutant by hand and revert it.
4. Register the adapter in task 150's engine by `Source()`. The production `Key` func stays unwired
   (a `nil` refuses the connection with `tracker_refused`, named in the run) until task 145's Key
   Vault proposal is accepted.
5. Commit the three paths above and nothing else.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/hub/worksync/... -race -count=1 -run "GitHub" -v'
# expect: TestGitHubSkipsPullRequests, TestGitHubFollowsEveryLinkPage, TestGitHubIDsAreScopedByRepository,
#         TestGitHubCloseIsUpstreamOnly, TestGitHubRateLimitKeepsNothing, TestGitHubBodyHoldsNoTag,
#         TestGitHubRefusesOffHostNextLink, TestGitHubAppTokenMintsOnceAndSigns, TestGitHubRecordedShape: PASS; ok
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/hub/worksync/... -race -count=1'
# expect: ok (task 151's tests still pass beside these)
~/.claude/tools/claudezsh/claudezsh 'git diff --stat -- control-plane/go.mod go.work'
# expect: no output (no new dependency: the JWT is the standard library)
```

#### What this task does NOT claim

- That KPMG's GitHub Enterprise Managed Users organisation permits installing the app, or that
  anyone has (**UNVERIFIED**, an org-owner act); or GitHub Enterprise Server (L-204, phase 3's).
- That `2022-11-28`, the `since` semantics, the events endpoint's order, or the rate-limit headers
  behave as written on the day. The fixtures are **our** shapes; no test reaches GitHub.
- That the Hub's network reaches `api.github.com` (**UNVERIFIED**, task 150's caveat).
- That an upstream edit of title or body is shown: only state and assignee changes are (decision
  4); an edit re-reads the issue, which SW.6 answers as `existing` and does not rewrite.
- Two-way comments (task 153) or outside pull requests (tasks 154-155).

#### Gaps found while writing this part

1. **Task 150: a pull may exceed one SW.6 page** in decision 3's tie case (up to 2,000 items). The
   engine must write `Pulled.Items` in pages of `hubapi.MaxImportItems` and advance the cursor only
   after the last page commits. Task 151's part assumed one page per pull; both hold if the engine
   always pages.
2. **Task 143 / 150: `SyncRun.errors` has no time.** The scope asks the rate-limit reset time be
   recorded in `SyncRun.errors`, whose shape is `{externalId, code}` (`tasks-141-160.md:152-153`).
   Proposed: `retryAt?` on the error entry, filled from `PullError.RetryAt`. Until then the reset
   time is in the Hub's log only.
3. **`tracker_stalled`** joins task 151's four run-level `worksync` codes, none of them in task
   143's appended list.
4. **Task 155 reuses `GitHubAppToken`** for linkbacks and autolinks. The app then needs issues
   read, pull-request write and administration (autolinks) permissions; task 159's manifests are
   people's apps, not this one, so where the Hub's own app manifest is recorded is unassigned.
