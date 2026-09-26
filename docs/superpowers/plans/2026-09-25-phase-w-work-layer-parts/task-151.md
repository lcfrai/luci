### Task 151: tracker sync, part 2: Azure Boards, one way in

W5, stream WH, about 3 hours (an estimate, not a measurement). Depends on task 150 (the engine, the
`Adapter` seam, `Cursor`, `Pulled`, `UpstreamChange`; **scope only, no part on this tree**, so every
name taken from it is read from `…-scopes/tasks-141-160.md:418-458` and is **UNVERIFIED** against
its part). Reads task 15's SW.6 rules (`task-15.md:167-292`: `ValidateImportBatch`, the `MaxImport*`
limits, `validImportURL`), task 03's `ImportItem` and `Priority` (`task-03.md:128-138`, `:543-552`),
task 02's `MaxIssueTitleChars` (200 code points) and `MaxIssueDescriptionBytes` (16 KiB,
`task-02.md:210-211`), task 143's `TrackerConnection` (scope `tasks-141-160.md:150-152`, no part)
and task 169's `TrackerTarget` (`task-169.md:160-164`), which names the Azure Boards target
`{organisation, project, areaPath}`.

Closes **L-202**'s Azure Boards half, with tasks 152 and 171 (05 §1 L-202: *"GitHub Issues **and
Azure Boards**"*). It adds no route, no table and no migration: the adapter is pure Go behind task
150's seam, and every write it causes is the engine's, through SW.6.

**What this task must not get wrong** is that Azure Boards is read, never believed. An item's
state, its assignee and its HTML never become ours: the state is reported as an `UpstreamChange`
and shown (05 §5.4 W5: *"Sync never writes a state, a fact or `done`"*), and the HTML leaves as plain
text through one sanitiser that task 152 reuses. A test kills each.

**Decisions made here, so nobody re-derives them:**

1. **Pinned version.** `boardsAPIVersion = "7.1"`, the one constant every call carries as
   `api-version`. Which versions Azure DevOps Services serves on the day is **UNVERIFIED** (the
   corporate network blocks some documentation, `CLAUDE.md`); the implementer checks the current
   REST reference before the first real call and changes only this constant.
2. **One pull is at most one SW.6 page.** `Pull` returns at most `hubapi.MaxImportItems` (500)
   items, so the engine writes one page per transaction (task 150). The WIQL query asks
   `$top=500`; the batch read takes the ids in slices of `boardsBatchMax` (200), the endpoint's own
   ceiling. 450 changed items are one WIQL call and three batch calls.
3. **The cursor is strict, so ties never loop.** `Cursor` is `b1|<changedDate RFC 3339 nano>|<id>`.
   The WIQL is `[System.ChangedDate] > d OR ([System.ChangedDate] = d AND [System.Id] > id)`,
   ordered by `ChangedDate, Id`, with `timePrecision=true` (without it WIQL compares dates at day
   precision; the parameter's behaviour is **UNVERIFIED** on the day). A bulk edit that stamps 800
   items with one time is read in two pulls, never re-read. `""` is the first run: no date clause.
4. **The web link is read, never built.** The batch read asks `$expand: "Links"` and no `fields`
   list (the service refuses both together, **UNVERIFIED**), and `url` is `_links.html.href`
   verbatim. The adapter never rewrites `http:` to `https:`: SW.6 refuses it
   (`task-15.md:265-274`), so a server that hands out plain links is visible, not papered over.
5. **State is shown, never imported.** `ImportItem` has no state field (`task-03.md:543-552`). An
   item whose `Microsoft.VSTS.Common.StateChangeDate` is after the cursor's time yields one
   `UpstreamChange{Field: "state", At: that date}`; its `System.State` is carried only as the
   display word (the gap below). A first run emits none: nothing had been imported to change.
6. **Throttling waits once.** A `429` sleeps for `Retry-After` (seconds, capped at 60) and retries
   the same request once; a second `429`, or a `Retry-After` over the cap, is `*PullError{Code:
   "tracker_throttled", RetryAt}` and the cursor stays (task 150 advances only on commit).
7. **Tags are cut, never dropped.** `System.Tags` is `;`-separated. Each tag is trimmed, empty ones
   skipped, each cut to 60 code points, at most 50 kept (SW.6's limits). A cut tag cannot match a
   team label, so it lands in `unmatchedLabels` where a person sees it (`task-15.md:39-44`).
8. **The credential is the Hub's identity, not a stored secret.** The adapter takes a
   `TokenFunc`; production wires the Hub's workload identity for the Azure DevOps resource
   `499b84ac-1321-427f-aa17-267ca6975798/.default`. That identity must be added to the Azure DevOps
   organisation by its administrator (a tenant act, **UNVERIFIED** that anyone has). The Hub
   contract grants the process no such identity in v1 (`hub-contract.md:2083-2085`, quoted in
   task 145's scope), so the wiring waits on task 145's proposed addition; the tests use a fake.
9. **The external id is scoped by organisation**: `org#4522`, not `4522`. A work item id is
   unique within one Azure DevOps organisation only, and SW.6's idempotency is per Hub project
   (`task-15.md:32-38`), so one project fed by two organisations would otherwise merge two items.
   This is the same rule task 152 applies to GitHub (`owner/repo#number`). An organisation rename
   changes the id and re-imports; that cost is accepted and recorded, not hidden.

**Files:**

- `control-plane/internal/hub/worksync/boards.go` — the adapter (new)
- `control-plane/internal/hub/worksync/boards_test.go` — its tests against `httptest` (new)
- `control-plane/internal/hub/worksync/text.go` — `PlainTextFromHTML`, `CutRunes`, `CutBytes`, the
  one sanitiser tasks 152 and 153 reuse (new; **not in the File map**, a gap)
- `control-plane/internal/hub/worksync/text_test.go` — the sanitiser's tests (new)
- `control-plane/internal/hub/worksync/fetch.go` — `getJSON`/`postJSON` with the one `429` retry,
  `PullError`, `TokenFunc` (new; **not in the File map**, a gap; tasks 152 and 153 reuse it)
- `control-plane/internal/hub/worksync/testdata/boards/*.json` — response shapes **written by us**
  from the fields this adapter reads, not copied from any Microsoft page (new)
- `control-plane/go.mod` ⚑ — `golang.org/x/net` moves from `// indirect` (`v0.26.0`, line 53 at this
  writing) to a direct requirement for `golang.org/x/net/html` (BSD-3-Clause); no version change,
  and the `go` directive stays `go 1.23`

**Interfaces:**

- Produces: `worksync.NewBoards(BoardsConfig) *Boards`, `(*Boards).Source() == "azure_boards"`,
  `(*Boards).Pull`; `worksync.PlainTextFromHTML`, `CutRunes`, `CutBytes`; `worksync.PullError`,
  `TokenFunc`, `getJSON`, `postJSON`
- Consumes: task 150's `Adapter`, `Cursor`, `Pulled`, `UpstreamChange`; task 143's
  `hubapi.TrackerConnection` and `hubapi.ImportSourceAzureBoards`; task 03's `ImportItem`,
  `Priority*`; task 15's limits
- Consumed by: task 150's engine (registered by `Source()`), task 152 (`text.go`, `fetch.go`), task
  153 (comments on Azure Boards, `text.go`), task 171 (the connection form names the target)

#### Interfaces — complete: `worksync/fetch.go`

```go
package worksync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// TokenFunc returns a bearer token for one call. Azure Boards wires the Hub's
// workload identity (decision 8); GitHub an installation token (task 152).
// It is called per request, so a token that expires mid-pull is re-minted.
type TokenFunc func(ctx context.Context) (string, error)

// PullError is a pull that stopped without a page. Code is a Hub code string
// (task 143's list or the ones below); RetryAt is zero unless the tracker said
// when to come back. The engine records it in SyncRun.errors with externalId
// "" and leaves the cursor where it was (task 150).
type PullError struct {
	Code    string
	RetryAt time.Time
	Err     error
}

func (e *PullError) Error() string {
	if e.RetryAt.IsZero() {
		return fmt.Sprintf("worksync: %s: %v", e.Code, e.Err)
	}
	return fmt.Sprintf("worksync: %s until %s: %v", e.Code, e.RetryAt.UTC().Format(time.RFC3339), e.Err)
}
func (e *PullError) Unwrap() error { return e.Err }

const (
	CodeTrackerThrottled   = "tracker_throttled"   // 429, or a rate limit at zero (task 152)
	CodeTrackerUnreachable = "tracker_unreachable" // transport error or 5xx
	CodeTrackerRefused     = "tracker_refused"     // 401, 403 (not a rate limit), 404
	CodeTrackerShape       = "tracker_shape"       // a body we could not decode
	maxRetryAfter          = 60 * time.Second
	maxTrackerBody         = 8 << 20 // one page of 200 items with descriptions; unmeasured
)

// fetcher is the one HTTP path every adapter uses: bearer token, JSON in and
// out, a bounded body, and exactly one retry after a 429 (decision 6).
type fetcher struct {
	client *http.Client
	token  TokenFunc
	sleep  func(context.Context, time.Duration) error // tests pass a recorder
	now    func() time.Time
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// do sends method url with body (nil for GET) and decodes a 2xx reply into
// out. hdr, if not nil, receives the reply's headers (GitHub's Link).
func (f *fetcher) do(ctx context.Context, method, url string, body any, out any, hdr *http.Header) error {
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return err
		}
	}
	for attempt := 0; ; attempt++ {
		res, err := f.once(ctx, method, url, raw)
		if err != nil {
			return &PullError{Code: CodeTrackerUnreachable, Err: err}
		}
		if res.StatusCode == http.StatusTooManyRequests {
			wait, ok := retryAfter(res.Header.Get("Retry-After"))
			res.Body.Close()
			if attempt == 0 && ok && wait <= maxRetryAfter {
				if err := f.sleep(ctx, wait); err != nil {
					return err
				}
				continue
			}
			return &PullError{Code: CodeTrackerThrottled, RetryAt: f.now().Add(wait), Err: fmt.Errorf("429 from %s", method)}
		}
		defer res.Body.Close()
		if hdr != nil {
			*hdr = res.Header.Clone()
		}
		return decodeReply(res, out)
	}
}

func (f *fetcher) once(ctx context.Context, method, url string, raw []byte) (*http.Response, error) {
	var rd io.Reader
	if raw != nil {
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return nil, err
	}
	tok, err := f.token(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/json")
	if raw != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return f.client.Do(req)
}

// retryAfter reads the delta-seconds form only; an HTTP-date is refused, so a
// tracker that sends one is throttled for this run and tried next time.
func retryAfter(v string) (time.Duration, bool) {
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, false
	}
	return time.Duration(n) * time.Second, true
}

func decodeReply(res *http.Response, out any) error {
	switch {
	case res.StatusCode >= 500:
		return &PullError{Code: CodeTrackerUnreachable, Err: fmt.Errorf("status %d", res.StatusCode)}
	case res.StatusCode >= 400:
		return &PullError{Code: CodeTrackerRefused, Err: fmt.Errorf("status %d", res.StatusCode)}
	case res.StatusCode < 200 || res.StatusCode > 299:
		return &PullError{Code: CodeTrackerShape, Err: fmt.Errorf("status %d", res.StatusCode)}
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxTrackerBody+1))
	if err != nil {
		return &PullError{Code: CodeTrackerUnreachable, Err: err}
	}
	if len(body) > maxTrackerBody {
		return &PullError{Code: CodeTrackerShape, Err: fmt.Errorf("reply over %d bytes", maxTrackerBody)}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return &PullError{Code: CodeTrackerShape, Err: err}
	}
	return nil
}
```

The decoder is **lenient** on purpose (unknown fields ignored), unlike the Hub's strict route
decoder: a tracker adds fields without notice, and a sync that stops on each one is not a sync.
The adapter reads only the fields named below; everything else is dropped unread.

The four codes are run-level, never item-level, and never a Hub HTTP status: they live only in
`SyncRun.errors[].code` with `externalId: ""`. Task 143's appended codes
(`tasks-141-160.md:184-187`) do not list them, so this part keeps them as `worksync` constants
that never reach a problem body (a gap noted for task 143).

#### Interfaces — complete: `worksync/text.go`

```go
package worksync

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// cutMark ends a text this package shortened, so the cut is shown (05 §5.4
// W5; the scope's "the cut shown"). It counts inside the limit.
const cutMark = " …"

// PlainTextFromHTML is the one sanitiser (tasks 151-153). It tokenises, keeps
// text nodes, turns block ends into line breaks, and drops every tag, every
// attribute, and the whole content of script, style, iframe, object, noscript
// and template. Its output is text, never Markdown and never HTML: a leftover
// "<" is a character the issue view escapes like any other. Entities decode.
func PlainTextFromHTML(s string) string {
	z := html.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	skip := 0 // depth inside a dropped element
	for {
		switch z.Next() {
		case html.ErrorToken:
			return tidyLines(b.String())
		case html.StartTagToken:
			name, _ := z.TagName()
			if dropped[string(name)] {
				skip++
			} else if skip == 0 && string(name) == "br" {
				b.WriteByte('\n')
			}
		case html.SelfClosingTagToken:
			if name, _ := z.TagName(); skip == 0 && string(name) == "br" {
				b.WriteByte('\n')
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch {
			case dropped[string(name)]:
				if skip > 0 {
					skip--
				}
			case skip == 0 && blocks[string(name)]:
				b.WriteByte('\n')
			}
		case html.TextToken:
			if skip == 0 {
				b.Write(z.Text()) // Text() has entities decoded
			}
		}
	}
}

var dropped = map[string]bool{"script": true, "style": true, "iframe": true, "object": true,
	"noscript": true, "template": true, "svg": true, "math": true}

var blocks = map[string]bool{"p": true, "div": true, "li": true, "tr": true, "h1": true,
	"h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "pre": true, "blockquote": true,
	"table": true, "ul": true, "ol": true}

// tidyLines trims each line's right edge and folds three or more line breaks
// into two, so a tracker's nested divs do not become a page of blank lines.
func tidyLines(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out, blank := make([]string, 0, len(lines)), 0
	for _, l := range lines {
		l = strings.TrimRight(l, " \t ")
		if l == "" {
			if blank++; blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// CutRunes returns s unchanged if it has at most n code points, else its first
// n-2 code points and cutMark, trimmed: exactly n or fewer, the cut shown.
// Titles use n = hubapi.MaxIssueTitleChars (200, task-02.md:210).
func CutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	keep := n - utf8.RuneCountInString(cutMark)
	i, k := 0, 0
	for i < len(s) && k < keep {
		_, w := utf8.DecodeRuneInString(s[i:])
		i, k = i+w, k+1
	}
	return strings.TrimRight(s[:i], " ") + cutMark
}

// CutBytes is CutRunes for a UTF-8 byte limit, cutting on a code point
// boundary. Descriptions use n = hubapi.MaxIssueDescriptionBytes (16 KiB).
func CutBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	i := n - len(cutMark)
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return strings.TrimRight(s[:i], " ") + cutMark
}
```

`CutRunes` counts code points, as Postgres `char_length` and task 02's rule do; a title of 200
code points passes untouched and one of 201 becomes its first 198 plus the mark, 200 in all (`" …"` is two code
points and four bytes). An empty title after sanitising stays empty and SW.6 refuses it by name
(`task-02.md:251`); the engine reports it in the run.

#### Interfaces — complete: `worksync/boards.go`

```go
package worksync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

const (
	boardsAPIVersion = "7.1" // decision 1: the one place the version lives
	boardsBatchMax   = 200   // the batch read's own ceiling per call
	boardsBaseURL    = "https://dev.azure.com"
	boardsCursorTag  = "b1"
)

// BoardsTarget is TrackerConnection.Target for source azure_boards, the Go
// twin of task 169's TS union arm (task-169.md:162). AreaPath is optional.
type BoardsTarget struct {
	Organisation string  `json:"organisation"`
	Project      string  `json:"project"`
	AreaPath     *string `json:"areaPath"`
}

var boardsOrgName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,49}$`)

// BoardsConfig wires the adapter. BaseURL is boardsBaseURL in production and
// an httptest URL in tests; Sleep and Now default to the real clock.
type BoardsConfig struct {
	Client  *http.Client
	Token   TokenFunc
	BaseURL string
	Sleep   func(context.Context, time.Duration) error
	Now     func() time.Time
}

// Boards is task 150's Adapter for Azure Boards, one way in.
type Boards struct {
	f    fetcher
	base string
}

func NewBoards(c BoardsConfig) *Boards {
	if c.BaseURL == "" {
		c.BaseURL = boardsBaseURL
	}
	if c.Sleep == nil {
		c.Sleep = sleepCtx
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return &Boards{f: fetcher{client: c.Client, token: c.Token, sleep: c.Sleep, now: c.Now},
		base: strings.TrimRight(c.BaseURL, "/")}
}

func (b *Boards) Source() string { return hubapi.ImportSourceAzureBoards }

// Pull reads the items changed after cur, at most one SW.6 page (decision 2).
func (b *Boards) Pull(ctx context.Context, conn hubapi.TrackerConnection, cur Cursor) (Pulled, error) {
	var t BoardsTarget
	if err := json.Unmarshal(conn.Target, &t); err != nil || !boardsOrgName.MatchString(t.Organisation) ||
		t.Project == "" {
		return Pulled{}, &PullError{Code: CodeTrackerRefused, Err: fmt.Errorf("azure boards target: %v", err)}
	}
	since, lastID, err := parseBoardsCursor(cur)
	if err != nil {
		return Pulled{}, &PullError{Code: CodeTrackerShape, Err: err}
	}
	org, proj := url.PathEscape(t.Organisation), url.PathEscape(t.Project)
	wiqlURL := fmt.Sprintf("%s/%s/%s/_apis/wit/wiql?api-version=%s&timePrecision=true&$top=%d",
		b.base, org, proj, boardsAPIVersion, hubapi.MaxImportItems)
	var q struct {
		WorkItems []struct {
			ID int `json:"id"`
		} `json:"workItems"`
	}
	if err := b.f.do(ctx, http.MethodPost, wiqlURL, map[string]string{"query": boardsWIQL(t, since, lastID)}, &q, nil); err != nil {
		return Pulled{}, err
	}
	out := Pulled{Items: []hubapi.ImportItem{}, Upstream: []UpstreamChange{}, Next: cur}
	if len(q.WorkItems) == 0 {
		return out, nil
	}
	ids := make([]int, 0, len(q.WorkItems))
	for _, w := range q.WorkItems {
		ids = append(ids, w.ID)
	}
	got := make(map[int]boardsItem, len(ids))
	batchURL := fmt.Sprintf("%s/%s/_apis/wit/workitemsbatch?api-version=%s", b.base, org, boardsAPIVersion)
	for lo := 0; lo < len(ids); lo += boardsBatchMax {
		hi := min(lo+boardsBatchMax, len(ids))
		var page struct {
			Value []boardsItem `json:"value"`
		}
		body := map[string]any{"ids": ids[lo:hi], "$expand": "Links"} // decision 4: no "fields"
		if err := b.f.do(ctx, http.MethodPost, batchURL, body, &page, nil); err != nil {
			return Pulled{}, err
		}
		for _, it := range page.Value {
			got[it.ID] = it
		}
	}
	for _, id := range ids { // WIQL order is cursor order
		it, ok := got[id]
		if !ok {
			continue // deleted between the two calls: it is simply not seen this run
		}
		item, changed := it.toImport(t.Organisation)
		out.Items = append(out.Items, item)
		if !since.IsZero() && changed.After(since) {
			out.Upstream = append(out.Upstream, UpstreamChange{ExternalID: item.ExternalID, Field: "state", At: changed})
		}
		if cd := it.time("System.ChangedDate"); !cd.IsZero() {
			out.Next = boardsCursor(cd, id)
		}
	}
	return out, nil
}

// boardsItem is one element of the batch read: only what the mapping reads.
type boardsItem struct {
	ID     int                        `json:"id"`
	Fields map[string]json.RawMessage `json:"fields"`
	Links  struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"_links"`
}

func (it boardsItem) str(k string) string {
	var s string
	_ = json.Unmarshal(it.Fields[k], &s) // absent or not a string reads as ""
	return s
}

func (it boardsItem) time(k string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, it.str(k))
	return t
}

// toImport maps one item to SW.6 (the scope's table) and returns the item's
// state-change time for decision 5. It never fixes a value SW.6 refuses.
func (it boardsItem) toImport(org string) (hubapi.ImportItem, time.Time) {
	var by struct {
		DisplayName string `json:"displayName"`
	}
	_ = json.Unmarshal(it.Fields["System.CreatedBy"], &by)
	var prio int
	_ = json.Unmarshal(it.Fields["Microsoft.VSTS.Common.Priority"], &prio)
	created := ""
	if c := it.time("System.CreatedDate"); !c.IsZero() {
		created = c.UTC().Format(time.RFC3339)
	}
	return hubapi.ImportItem{
		ExternalID:  org + "#" + strconv.Itoa(it.ID), // decision 9
		URL:         it.Links.HTML.Href,              // decision 4: verbatim
		Title:       CutRunes(strings.TrimSpace(it.str("System.Title")), hubapi.MaxIssueTitleChars),
		Description: CutBytes(PlainTextFromHTML(it.str("System.Description")), hubapi.MaxIssueDescriptionBytes),
		Priority:    boardsPriority(prio),
		Labels:      boardsTags(it.str("System.Tags")),
		AuthorName:  CutRunes(by.DisplayName, hubapi.MaxImportAuthorNameChars),
		CreatedAt:   created,
	}, it.time("Microsoft.VSTS.Common.StateChangeDate")
}

// boardsPriority: 1-4 in order; anything else, including absent (0), is none.
func boardsPriority(p int) hubapi.Priority {
	switch p {
	case 1:
		return hubapi.PriorityUrgent
	case 2:
		return hubapi.PriorityHigh
	case 3:
		return hubapi.PriorityMedium
	case 4:
		return hubapi.PriorityLow
	}
	return hubapi.PriorityNone
}

// boardsTags applies decision 7.
func boardsTags(s string) []string {
	out := []string{}
	for _, t := range strings.Split(s, ";") {
		if t = strings.TrimSpace(t); t == "" {
			continue
		}
		if out = append(out, strings.TrimSpace(CutRunes(t, hubapi.MaxLabelNameChars))); len(out) == hubapi.MaxImportLabels {
			break
		}
	}
	return out
}

// boardsWIQL is decision 3's query. Literals are WIQL strings, single quotes
// doubled; the project name is @project, never interpolated.
func boardsWIQL(t BoardsTarget, since time.Time, lastID int) string {
	var b strings.Builder
	b.WriteString("SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @project")
	if t.AreaPath != nil && *t.AreaPath != "" {
		b.WriteString(" AND [System.AreaPath] UNDER " + wiqlString(*t.AreaPath))
	}
	if !since.IsZero() {
		d := wiqlString(since.UTC().Format(time.RFC3339Nano))
		fmt.Fprintf(&b, " AND ([System.ChangedDate] > %s OR ([System.ChangedDate] = %s AND [System.Id] > %d))", d, d, lastID)
	}
	b.WriteString(" ORDER BY [System.ChangedDate] ASC, [System.Id] ASC")
	return b.String()
}

func wiqlString(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func boardsCursor(t time.Time, id int) Cursor {
	return Cursor(fmt.Sprintf("%s|%s|%d", boardsCursorTag, t.UTC().Format(time.RFC3339Nano), id))
}

// parseBoardsCursor reads "" as the first run and refuses any other shape, so
// a cursor from another adapter cannot silently restart a connection.
func parseBoardsCursor(c Cursor) (time.Time, int, error) {
	if c == "" {
		return time.Time{}, 0, nil
	}
	p := strings.Split(string(c), "|")
	if len(p) != 3 || p[0] != boardsCursorTag {
		return time.Time{}, 0, fmt.Errorf("not an azure boards cursor: %q", c)
	}
	t, err := time.Parse(time.RFC3339Nano, p[1])
	if err != nil {
		return time.Time{}, 0, err
	}
	id, err := strconv.Atoi(p[2])
	return t, id, err
}

var _ Adapter = (*Boards)(nil)
```

**Two consequences, stated so nobody "fixes" them.** A work item deleted between the WIQL call
and the batch read is skipped without error and the cursor still passes it: a deleted item is not
an item. And an item whose `System.ChangedDate` is absent does not move the cursor, so a service
that stops sending the field re-reads the same page each run; SW.6's idempotency makes that
harmless, and the run's `existing` count makes it visible.

#### Wire shapes — the calls the adapter makes (shapes, not copies of any documentation)

```text
POST {base}/{org}/{project}/_apis/wit/wiql?api-version=7.1&timePrecision=true&$top=500
  {"query": "SELECT [System.Id] FROM WorkItems WHERE [System.TeamProject] = @project
             AND ([System.ChangedDate] > '2026-09-25T11:02:00.123Z' OR ([System.ChangedDate] =
             '2026-09-25T11:02:00.123Z' AND [System.Id] > 4521)) ORDER BY [System.ChangedDate] ASC,
             [System.Id] ASC"}
  ← {"workItems": [{"id": 4522}, {"id": 4530}]}

POST {base}/{org}/_apis/wit/workitemsbatch?api-version=7.1
  {"ids": [4522, 4530], "$expand": "Links"}
  ← {"value": [{"id": 4522, "fields": {"System.Title": "Resume upload fails",
       "System.Description": "<p>Steps</p>", "Microsoft.VSTS.Common.Priority": 1,
       "System.Tags": "web; upload", "System.CreatedBy": {"displayName": "A. Person"},
       "System.CreatedDate": "2026-09-20T09:00:00Z", "System.ChangedDate": "2026-09-25T11:03:00Z",
       "System.State": "Active", "Microsoft.VSTS.Common.StateChangeDate": "2026-09-25T11:03:00Z"},
     "_links": {"html": {"href": "https://dev.azure.com/org/proj/_workitems/edit/4522"}}}]}
```

The `ImportItem` produced from the first element (SW.6's JSON, `task-03.md:543-552`):

```json
{"externalId": "org#4522", "url": "https://dev.azure.com/org/proj/_workitems/edit/4522",
 "title": "Resume upload fails", "description": "Steps", "priority": "urgent",
 "labels": ["web", "upload"], "authorName": "A. Person", "createdAt": "2026-09-20T09:00:00Z"}
```

#### Tests to write first — `worksync/boards_test.go` (complete)

The server is `httptest`; items are built in code (`boardsFields`) except the two recorded shapes
below, which are ours. No test reaches a real tracker.

```go
package worksync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// fakeBoards answers WIQL with ids and the batch read with items built by
// mk; hook, if set, may answer first (429s). It counts calls per path.
type fakeBoards struct {
	mu     sync.Mutex
	ids    []int
	mk     func(id int) map[string]any
	hook   func(w http.ResponseWriter, r *http.Request) bool
	calls  map[string]int
	query  string
	bodies [][]int
}

func (f *fakeBoards) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[r.URL.Path]++
	if f.hook != nil && f.hook(w, r) {
		return
	}
	switch r.URL.Path {
	case "/org/proj/_apis/wit/wiql":
		var q struct{ Query string }
		_ = json.NewDecoder(r.Body).Decode(&q)
		f.query = q.Query
		items := []map[string]int{}
		for _, id := range f.ids {
			items = append(items, map[string]int{"id": id})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"workItems": items})
	case "/org/_apis/wit/workitemsbatch":
		var q struct{ IDs []int }
		_ = json.NewDecoder(r.Body).Decode(&q)
		f.bodies = append(f.bodies, q.IDs)
		out := []map[string]any{}
		for _, id := range q.IDs {
			out = append(out, map[string]any{"id": id, "fields": f.mk(id), "_links": map[string]any{
				"html": map[string]string{"href": fmt.Sprintf("https://dev.azure.com/org/proj/_workitems/edit/%d", id)}}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"value": out})
	default:
		http.NotFound(w, r)
	}
}

func boardsFields(id int) map[string]any {
	return map[string]any{"System.Title": fmt.Sprintf("item %d", id), "System.Description": "<p>d</p>",
		"System.ChangedDate": "2026-09-25T11:00:00Z", "System.CreatedDate": "2026-09-20T09:00:00Z",
		"System.CreatedBy": map[string]string{"displayName": "A. Person"}, "System.State": "New"}
}

func newTestBoards(t *testing.T, f *fakeBoards) (*Boards, *[]time.Duration) {
	t.Helper()
	f.calls = map[string]int{}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	slept := &[]time.Duration{}
	b := NewBoards(BoardsConfig{Client: srv.Client(), BaseURL: srv.URL,
		Token: func(context.Context) (string, error) { return "tok", nil },
		Sleep: func(_ context.Context, d time.Duration) error { *slept = append(*slept, d); return nil },
		Now:   func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) }})
	return b, slept
}

var boardsConn = hubapi.TrackerConnection{ID: "tc-1", Source: hubapi.ImportSourceAzureBoards,
	Target: json.RawMessage(`{"organisation":"org","project":"proj","areaPath":null}`), Mode: "in"}

// Kills "one page only": 450 ids read in 200 + 200 + 50.
func TestBoardsReadsEveryBatch(t *testing.T) {
	f := &fakeBoards{mk: boardsFields}
	for i := 1; i <= 450; i++ {
		f.ids = append(f.ids, i)
	}
	b, _ := newTestBoards(t, f)
	got, err := b.Pull(context.Background(), boardsConn, "")
	if err != nil {
		t.Fatal(err)
	}
	if n := f.calls["/org/_apis/wit/workitemsbatch"]; n != 3 || len(got.Items) != 450 {
		t.Fatalf("batch calls %d, items %d; want 3 and 450", n, len(got.Items))
	}
	if len(f.bodies[2]) != 50 || got.Next != boardsCursor(time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC), 450) {
		t.Fatalf("last batch %d ids, next %q", len(f.bodies[2]), got.Next)
	}
}

// Kills "off-by-one priority".
func TestBoardsPriority(t *testing.T) {
	for in, want := range map[int]hubapi.Priority{0: hubapi.PriorityNone, 1: hubapi.PriorityUrgent,
		2: hubapi.PriorityHigh, 3: hubapi.PriorityMedium, 4: hubapi.PriorityLow, 5: hubapi.PriorityNone} {
		if got := boardsPriority(in); got != want {
			t.Errorf("priority %d → %q, want %q", in, got, want)
		}
	}
}

// Kills "HTML imported".
func TestBoardsDescriptionHoldsNoTag(t *testing.T) {
	f := &fakeBoards{ids: []int{7}, mk: func(id int) map[string]any {
		m := boardsFields(id)
		m["System.Description"] = `<p>Steps</p><script>alert(1)</script><img src=x onerror="alert(2)">then &amp; more`
		return m
	}}
	b, _ := newTestBoards(t, f)
	got, err := b.Pull(context.Background(), boardsConn, "")
	if err != nil {
		t.Fatal(err)
	}
	if d := got.Items[0].Description; d != "Steps\nthen & more" || strings.ContainsAny(d, "<>") {
		t.Fatalf("description %q", d)
	}
}

// Kills "state imported": a close after the cursor is an UpstreamChange only.
func TestBoardsStateIsUpstreamOnly(t *testing.T) {
	f := &fakeBoards{ids: []int{7}, mk: func(id int) map[string]any {
		m := boardsFields(id)
		m["System.State"], m["Microsoft.VSTS.Common.StateChangeDate"] = "Closed", "2026-09-25T11:02:00Z"
		return m
	}}
	b, _ := newTestBoards(t, f)
	cur := boardsCursor(time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC), 3)
	got, err := b.Pull(context.Background(), boardsConn, cur)
	if err != nil {
		t.Fatal(err)
	}
	want := UpstreamChange{ExternalID: "org#7", Field: "state", At: time.Date(2026, 9, 25, 11, 2, 0, 0, time.UTC)}
	if len(got.Upstream) != 1 || got.Upstream[0] != want {
		t.Fatalf("upstream %+v", got.Upstream)
	}
	raw, _ := json.Marshal(got.Items[0])
	if strings.Contains(string(raw), "Closed") || strings.Contains(string(raw), `"state"`) {
		t.Fatalf("the state leaked into the item: %s", raw)
	}
	if !strings.Contains(f.query, "[System.Id] > 3") {
		t.Fatalf("cursor not strict: %s", f.query) // also kills "ties re-read" (decision 3)
	}
}

// Kills "throttling fails the run": one 429 with Retry-After 5 waits 5 s and
// retries; two in a row stop with tracker_throttled and no page.
func TestBoardsThrottleWaitsOnce(t *testing.T) {
	for _, tc := range []struct {
		name    string
		n429    int
		wantErr bool
	}{{"once", 1, false}, {"twice", 2, true}} {
		t.Run(tc.name, func(t *testing.T) {
			left := tc.n429
			f := &fakeBoards{ids: []int{1}, mk: boardsFields, hook: func(w http.ResponseWriter, r *http.Request) bool {
				if left > 0 && strings.HasSuffix(r.URL.Path, "/wiql") {
					left--
					w.Header().Set("Retry-After", "5")
					w.WriteHeader(http.StatusTooManyRequests)
					return true
				}
				return false
			}}
			b, slept := newTestBoards(t, f)
			got, err := b.Pull(context.Background(), boardsConn, "")
			var pe *PullError
			switch {
			case !tc.wantErr && (err != nil || len(got.Items) != 1 || len(*slept) != 1 || (*slept)[0] != 5*time.Second):
				t.Fatalf("err %v, items %d, slept %v", err, len(got.Items), *slept)
			case tc.wantErr && (!errors.As(err, &pe) || pe.Code != CodeTrackerThrottled || pe.RetryAt.IsZero()):
				t.Fatalf("want tracker_throttled with a retry time, got %v", err)
			}
		})
	}
}

// Kills "unsafe link": an http: web link is carried verbatim and SW.6
// refuses it by name, so the engine can name it in SyncRun.errors (task 150).
func TestBoardsPlainHTTPLinkIsRefusedBySW6(t *testing.T) {
	f := &fakeBoards{ids: []int{9}, mk: boardsFields}
	b, _ := newTestBoards(t, f)
	f.hook = func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.HasSuffix(r.URL.Path, "/workitemsbatch") {
			return false
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]any{{"id": 9, "fields": boardsFields(9),
			"_links": map[string]any{"html": map[string]string{"href": "http://tfs.example/_workitems/edit/9"}}}}})
		return true
	}
	got, err := b.Pull(context.Background(), boardsConn, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Items[0].URL != "http://tfs.example/_workitems/edit/9" {
		t.Fatalf("the adapter rewrote the link: %q", got.Items[0].URL)
	}
	err = hubapi.ValidateImportBatch(hubapi.ImportBatch{Source: hubapi.ImportSourceAzureBoards, Items: got.Items})
	var fe *hubapi.FieldError
	if !errors.As(err, &fe) || fe.Field != "items[0].url" {
		t.Fatalf("want items[0].url refused, got %v", err)
	}
}

// The recorded shape decodes to the ImportItem in the part (Wire shapes).
func TestBoardsRecordedShape(t *testing.T) {
	raw := readTestdata(t, "boards/batch-reply.json")
	var page struct{ Value []boardsItem }
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	item, _ := page.Value[0].toImport("org")
	got, _ := json.Marshal(item)
	want := `{"externalId":"org#4522","url":"https://dev.azure.com/org/proj/_workitems/edit/4522",` +
		`"title":"Resume upload fails","description":"Steps","priority":"urgent","labels":["web","upload"],` +
		`"authorName":"A. Person","createdAt":"2026-09-20T09:00:00Z"}`
	if string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}
```

`readTestdata` is the package's one helper (`os.ReadFile(filepath.Join("testdata", name))` with
`t.Fatal` on error), defined in `text_test.go` below so tasks 152 and 153 share it.

#### Tests to write first — `worksync/text_test.go` (complete)

```go
package worksync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Kills "HTML imported" at the sanitiser, for every source that uses it.
func TestPlainTextDropsEveryTag(t *testing.T) {
	for in, want := range map[string]string{
		`<p>a</p><div>b<br>c</div>`:                        "a\nb\nc",
		`x<script>alert(1)</script>y`:                      "xy",
		`<img src=x onerror="alert(1)">z`:                  "z",
		`<style>p{}</style><svg><script>1</script></svg>k`: "k",
		`&lt;b&gt; is text`:                                "<b> is text",
		"<p>one</p>\n\n\n\n<p>two</p>":                     "one\n\ntwo",
	} {
		if got := PlainTextFromHTML(in); got != want {
			t.Errorf("PlainTextFromHTML(%q) = %q, want %q", in, got, want)
		}
	}
}

// Kills "cut hidden" and "cut splits a code point".
func TestCutsShowTheCut(t *testing.T) {
	title := strings.Repeat("é", 201)
	got := CutRunes(title, 200)
	if n := utf8.RuneCountInString(got); n != 200 || !strings.HasSuffix(got, cutMark) {
		t.Fatalf("CutRunes: %d code points, suffix %q", n, got[len(got)-4:])
	}
	if CutRunes(strings.Repeat("a", 200), 200) != strings.Repeat("a", 200) {
		t.Fatal("a title at the limit was cut")
	}
	body := strings.Repeat("é", 9000) // 18000 bytes
	cut := CutBytes(body, 16<<10)
	if len(cut) > 16<<10 || !utf8.ValidString(cut) || !strings.HasSuffix(cut, cutMark) {
		t.Fatalf("CutBytes: %d bytes, valid %v", len(cut), utf8.ValidString(cut))
	}
}
```

The one decoded entity case (`&lt;b&gt;`) is deliberate: the output is text, so a `<` that was an
entity comes back as a character. The issue view renders descriptions as text (task 46), never as
HTML, so that `<` is inert. A test in task 46's part owns that side; this part does not re-prove
it.

#### Testdata — `worksync/testdata/boards/` (complete, ours)

`wiql-reply.json`:

```json
{"workItems": [{"id": 4522}, {"id": 4530}]}
```

`batch-reply.json` (the first element is the one the Wire shapes section maps):

```json
{"value": [
  {"id": 4522, "fields": {"System.Title": "Resume upload fails", "System.Description": "<p>Steps</p>",
    "Microsoft.VSTS.Common.Priority": 1, "System.Tags": "web; upload",
    "System.CreatedBy": {"displayName": "A. Person"}, "System.CreatedDate": "2026-09-20T09:00:00Z",
    "System.ChangedDate": "2026-09-25T11:03:00Z", "System.State": "Active",
    "Microsoft.VSTS.Common.StateChangeDate": "2026-09-25T11:03:00Z"},
   "_links": {"html": {"href": "https://dev.azure.com/org/proj/_workitems/edit/4522"}}},
  {"id": 4530, "fields": {"System.Title": "Second", "System.ChangedDate": "2026-09-25T11:04:00Z"},
   "_links": {"html": {"href": "https://dev.azure.com/org/proj/_workitems/edit/4530"}}}
]}
```

#### Mutants, applied by hand before the code is final

| Mutant | Test that fails |
|---|---|
| the batch loop runs once (`lo < boardsBatchMax`) | `TestBoardsReadsEveryBatch` |
| `case 1:` returns `PriorityHigh` (the scale shifted) | `TestBoardsPriority` |
| `PlainTextFromHTML` returns its input | `TestBoardsDescriptionHoldsNoTag`, `TestPlainTextDropsEveryTag` |
| `toImport` appends `System.State` to `Labels` | `TestBoardsStateIsUpstreamOnly` |
| `attempt == 0` becomes `false` (no retry) | `TestBoardsThrottleWaitsOnce/once` |
| `toImport` rewrites `http://` to `https://` | `TestBoardsPlainHTTPLinkIsRefusedBySW6` |
| the WIQL drops `[System.Id] > id` (ties re-read) | `TestBoardsStateIsUpstreamOnly` |
| `CutRunes` keeps `n` code points before the mark | `TestCutsShowTheCut` |

#### Steps

1. Confirm task 150 has landed on the sync base (`worksync/engine.go` defines `Adapter`, `Cursor`,
   `Pulled`, `UpstreamChange`) and task 143 has (`hubapi.TrackerConnection` with `Target` as
   `json.RawMessage`; `ImportSourceAzureBoards`). If a name differs, follow the landed name and note
   it in the commit; if either has not landed, wait (rule H1).
2. Write `text_test.go`, `boards_test.go` and the two testdata files. Run the package: it fails to
   compile (no `Boards`, no `PlainTextFromHTML`).
3. Move `golang.org/x/net` to a direct requirement in `control-plane/go.mod`, same version; check
   `git diff control-plane/go.mod go.work` shows no `go` directive change.
4. Write `fetch.go`, then `text.go`; run `TestPlainText*` and `TestCuts*` green.
5. Write `boards.go`; run the package green.
6. Apply each mutant in the table by hand, see its test fail, revert it.
7. Register the adapter in task 150's engine by `Source()` (one line where the engine lists its
   adapters; **UNVERIFIED** where, until 150's part exists). Leave the production `TokenFunc`
   unwired behind a `nil` check that refuses the connection with `tracker_refused` and says why in
   the run, until task 145's Key Vault and workload-identity proposal is accepted.
8. Commit the files above and nothing else.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/hub/worksync/... -race -count=1 -run "Boards|PlainText|Cuts" -v'
# expect: TestBoardsReadsEveryBatch, TestBoardsPriority, TestBoardsDescriptionHoldsNoTag,
#         TestBoardsStateIsUpstreamOnly, TestBoardsThrottleWaitsOnce (2 subtests),
#         TestBoardsPlainHTTPLinkIsRefusedBySW6, TestBoardsRecordedShape, TestPlainTextDropsEveryTag,
#         TestCutsShowTheCut: all PASS; ok
~/.claude/tools/claudezsh/claudezsh 'go vet ./control-plane/internal/hub/worksync/...'
# expect: exit 0, no output
~/.claude/tools/claudezsh/claudezsh 'git diff --stat -- control-plane/go.mod go.work'
# expect: control-plane/go.mod only, one line moved out of the indirect block; no "go 1.2x" change
~/.claude/tools/claudezsh/claudezsh 'grep -rn "http://" control-plane/internal/hub/worksync/boards.go'
# expect: no output (the adapter never builds a link)
```

#### What this task does NOT claim

- That the Hub's identity has been added to any Azure DevOps organisation (**UNVERIFIED**; a tenant
  act), or that the Hub's network reaches `dev.azure.com` (**UNVERIFIED**, task 150's own caveat).
- That `api-version=7.1`, `timePrecision`, `$top` on WIQL, or refusing `$expand` with `fields`
  behave as written on the day. The fixtures are **our** shapes of the fields read here; no test
  proves the live service answers them.
- That `Microsoft.VSTS.Common.StateChangeDate` exists in every process template (it is read if
  present; an absent one means no `UpstreamChange` is shown, never a wrong one).
- Two-way comments (task 153), Azure DevOps Server on premises, or any upstream field beyond state.
- Anything about how many items a real project holds or how long a pull takes: no size is measured.

#### Gaps found while writing this part

1. **The File map lacks `worksync/text.go` and `worksync/fetch.go`** (the shared sanitiser and the
   one-retry fetcher). Its `worksync/{engine,links,boards,github,jira}.go` row should add both.
2. **Task 150: per-item refusal.** Its scope says *"a page that fails validation leaves the cursor
   where it was"* (`tasks-141-160.md:448-449`), while this task's scope names an `http:` link in
   the run's `errors`. Both hold only if the engine validates **each item** (a one-item
   `ValidateImportBatch`), moves a refused item into `SyncRun.errors {externalId, code}`, writes the
   rest, and keeps the whole-page refusal for store failures. Task 150's part should say so; this
   part's test proves the adapter half only.
3. **Task 150: pull until empty.** Decision 2 makes one `Pull` one page; the engine must call `Pull`
   again while it returns items and a new cursor, bounded per run. Its scope does not say which.
4. **Task 143 / 150: `UpstreamChange` has no value.** The sync log's *"closed in Azure Boards at
   11:02"* (`tasks-141-160.md:439-440`) needs the tracker's own word (`Closed`). `{externalId,
   field, at}` cannot carry it. Proposed: `To string`, display text only, never mapped to our
   states. Until then the log can say only *"state changed in Azure Boards at 11:02"*.
5. **Task 143: `TrackerConnection.Target`'s Go type** is unnamed in its scope; this part assumes
   `json.RawMessage` decoded per source, matching task 169's TS union (`task-169.md:161-164`).
6. **Task 143: the four run-level codes** (`tracker_throttled`, `tracker_unreachable`,
   `tracker_refused`, `tracker_shape`) are not in its appended codes; they are kept as `worksync`
   constants here and never become an HTTP status.
7. **Decision 9 refines the scope's `System.Id → externalId`** to `org#id`, the same scoping task
   152 applies to GitHub. Task 171's connection form and any report that prints the id should show
   the `#id` part.
