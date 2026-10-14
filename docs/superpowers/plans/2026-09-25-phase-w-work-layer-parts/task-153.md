### Task 153: tracker sync, part 4: Jira one way in; then two way for comments only, on all three

W5, stream WH, about 3 hours (an estimate, not a measurement). Depends on task 151
(`worksync/boards.go`, `text.go`, `fetch.go`, `task-151.md`) and task 152 (`worksync/github.go`,
`GitHubAppToken`, `task-152.md`), and through them on task 150's engine and seam (**scope only, no
part on this tree**, `…-scopes/tasks-141-160.md:418-458`, names **UNVERIFIED** against its part).
Reads task 03's `Comment` (`task-03.md:357-372`), task 12's `CommentRow` and append-only rule
(`task-12.md:290-316`), task 14's comment erase (`CommentEraseResponse`, `task-14.md:165`), task
141's `edit_proposal` kind (`task-141.md:97`), task 99's coordination kinds (`note`,
`evidence_proposal`, `negotiation`, as `task-141.md:22-23` names them), task 169's Jira target
`{site, projectKey}` (`task-169.md:164`) and task 21's `auOnly` (`task-21.md:203-212`).

Closes **L-215** (05 §1: *"Two-way with Jira, as L-202, so adoption needs no big bang"*) and, with
tasks 151, 152 and 171, **L-202** and the W5 half of **L-220** (the Jira, Azure Boards and GitHub
Issues importers; *others later* keeps L-220 PARTIAL). 05 §5.4 W5: *"then **two way for comments
only** (2 d)"*; 05 §3.4: comments of the coordination class *"are never rendered"* outside their
readers, so they never leave.

**What this task must not get wrong:** only a person's plain comment ever leaves the product; no
comment arrives twice or comes back as an echo; and "two way" never means states. A test kills each.

**Decisions made here, so nobody re-derives them:**

1. **Jira search is pinned in one constant**, `jiraSearchPath = "/rest/api/3/search/jql"`, the
   token-paged search (`nextPageToken`, `isLast`). Atlassian has been replacing the offset-paged
   `/rest/api/3/search`; which one Jira Cloud serves on the day is **UNVERIFIED**, and the part's
   fixtures are the `search/jql` shape. Changing endpoints changes this constant and the page type.
2. **OAuth 2.0 only, through `api.atlassian.com`.** Requests go to
   `https://api.atlassian.com/ex/jira/{cloudId}` with the bearer token; the `cloudId` is resolved
   once per pull from `GET https://api.atlassian.com/oauth/token/accessible-resources` by matching
   the target's `site`. Task 151's `fetcher` sends only `Bearer`, so an API token (basic auth) is
   **not** supported here (a gap below). The token is a `secret_ref` (task 145's proposal).
3. **The JQL is minute-precise, so the last minute is re-read.** JQL compares `updated` at minute
   precision in the calling user's time zone, so the pull reads `GET /rest/api/3/myself`'s
   `timeZone` once and writes the cursor's time floored to the minute in that zone:
   `project = "ABC" AND updated >= "2026/09/25 21:02" ORDER BY updated ASC, key ASC`. SW.6's
   idempotency absorbs the overlap. The cursor is `j1|<updated RFC 3339>` in UTC. Past 2,000 items
   in one minute the pull stops with task 152's `tracker_stalled`.
4. **The external id is `<site>#<numeric id>`, not the key.** A Jira key changes when an issue moves
   project (`ABC-12` becomes `XYZ-3`); the numeric id does not. The key lives in the URL
   (`https://<site>/browse/ABC-12`) and in the sync log. This refines the scope's *"the issue key →
   `externalId`"*, as task 151's decision 9 refined Azure Boards'.
5. **Atlassian Document Format becomes plain text by one small converter** in `jira.go`:
   `paragraph` and `heading` end a line; `bulletList` items start `- `, `orderedList` items `1. `,
   `2. `…; `codeBlock` is fenced with three backticks; a `text` node with a `link` mark is
   `text (url)`, an `https` url only; `hardBreak` is a line break; every other node, a `table`
   included, becomes its children's text, a block per line. No node's JSON survives.
6. **The priority table**, fixed in `jira.go`: `Highest` → `urgent`, `High` → `high`, `Medium` →
   `medium`, `Low` and `Lowest` → `low`; any other name, a site's own included, → `none`. A missing
   priority is `none`. An unknown name never fails a run.
7. **Two way, comments only, in mode `in_and_comments`, for all three sources.** *In*: an external
   comment on a linked item becomes one `ic-` comment of kind `comment`, `text·H` (its author is a
   person, outside), created by the connection's integration principal, with `external {source,
   externalCommentId}` and the external author's name as display text. *Out*: a comment of kind
   `comment`, `text·H`, written by a person, top-level, on an issue linked through this connection,
   with no `external`, is posted as *"Dylan (PanaceaApp): …"* and its external id stored. `note`,
   `evidence_proposal`, `negotiation`, `edit_proposal`, every `text·A` comment and every reply
   **never leave**. States, assignees, labels and text never sync either way.
8. **Loops are cut twice.** First by the stored ids: an external comment whose id is already on one
   of our comments is skipped, and a comment with `external` set is never posted. Second by
   authorship: each adapter drops, on the way in, a comment written by its own identity (Azure
   DevOps: the Hub's identity id; GitHub: `performed_via_github_app.id` equals the app id; Jira: the
   token's `accountId`). The second layer is what makes a crash between the tracker's reply and the
   store's commit harmless on the way in; on the way out that crash can post a second copy on the
   next run, which is recorded, not hidden.
9. **An AU data only project posts nothing out.** 05's Q8 rule for branch names (task 21) applies to
   comments too: once text reaches a customer's tracker an erase cannot reach it. The push step
   skips such a project and says so once per run (`au_data_only`); task 150's connect route should
   refuse `in_and_comments` for it in the first place (a gap below).
10. **Erase reaches the posted copy where it can.** Task 14's erase of a comment we posted calls the
    adapter's `Delete`; the outcome (`erased_upstream`, or `erase_not_propagated` with the tracker's
    status) is recorded in the run. An imported comment's erase touches only our copy.

**Files:**

- `control-plane/internal/hub/worksync/jira.go` — the Jira adapter, the ADF converter, the priority
  table, and its `Commenter` half (new)
- `control-plane/internal/hub/worksync/jira_test.go`, `testdata/jira/*.json` — ours (new)
- `control-plane/internal/hub/worksync/comments.go` — `Commenter`, the pure out/in rules (new)
- `control-plane/internal/hub/worksync/comments_test.go` — the rules' tests (new)
- `control-plane/internal/hub/worksync/boards.go` ⚑ (task 151) — `PostComment`, `DeleteComment`,
  and comments in `Pull` when the mode asks (modified)
- `control-plane/internal/hub/worksync/github.go` ⚑ (task 152) — the same (modified)
- `control-plane/internal/hub/worksync/engine.go` ⚑ (task 150) — the in-comment write and the push
  step (modified; **UNVERIFIED** shape until 150's part exists)
- `control-plane/internal/hub/hubapi/work_intake.go` ⚑ (task 143) — **only if** its part lacks
  `ExternalComment` and `CommentExternal` below (modified, conditionally)

**Interfaces:**

- Produces: `worksync.NewJira`, `(*Jira).Source() == "jira"`, `(*Jira).Pull`; `worksync.ADFText`,
  `jiraPriority`; `worksync.Commenter`; `worksync.ShouldPost`, `worksync.OutboundText`,
  `worksync.NewInbound`; `Code*` for the run: `au_data_only`, `erased_upstream`,
  `erase_not_propagated`
- Consumes: tasks 150-152 as above; task 143's `ExternalComment`, `CommentExternal`,
  `TrackerModeInAndComments`, `ImportSourceJira`; task 03's `Comment`, `CommentKindComment`;
  task 02's `AuthorHuman`
- Consumed by: task 150's engine; task 168 (end to end), task 171 (the sync log shows the counts)

#### The wire — for task 143's `hubapi/work_intake.go` (complete; applied here only if absent)

```go
// TrackerModeIn and TrackerModeInAndComments are TrackerConnection.Mode's two
// words (task 143's Validate refuses any other, 05 §5.4 W5).
const (
	TrackerModeIn            = "in"
	TrackerModeInAndComments = "in_and_comments"
	ImportSourceJira         = "jira"
)

// ExternalComment is one comment read from a tracker item. ExternalID is the
// item's (SW.6's id); ExternalCommentID is the tracker's own comment id as a
// string. Body is plain text already (the adapter's sanitiser or ADF
// converter); AuthorName is display text, erasable, never a user id.
type ExternalComment struct {
	ExternalID        string    `json:"externalId"`
	ExternalCommentID string    `json:"externalCommentId"`
	AuthorName        string    `json:"authorName"`
	Body              string    `json:"body"`
	At                time.Time `json:"at"`
}

// CommentExternal marks a comment that came from, or went to, a tracker. It is
// set once, on import or after a post, and never cleared (not even by erase).
type CommentExternal struct {
	Source            string `json:"source"`
	ExternalCommentID string `json:"externalCommentId"`
	AuthorName        string `json:"authorName"` // "" for one we posted
}

// Additive field on task 03's Comment (shown as the diff):
//   External *CommentExternal `json:"external"` // null for a comment that never crossed
```

#### The columns — for task 145's migration (complete SQL; **not** applied by this task)

Comments cross only if `issue_comments` can hold the mark and refuse a second one. Task 145's
table list (`tasks-141-160.md:237-250`) names no such columns, so this is a gap for that owner:

```sql
ALTER TABLE issue_comments
  ADD COLUMN external_source      text,
  ADD COLUMN external_comment_id  text,
  ADD COLUMN external_author_name text,
  ADD CONSTRAINT issue_comments_external_pair
    CHECK ((external_source IS NULL) = (external_comment_id IS NULL)),
  ADD CONSTRAINT issue_comments_external_id_len
    CHECK (external_comment_id IS NULL OR char_length(external_comment_id) BETWEEN 1 AND 200);

-- one of our comments per tracker comment, per org: the first loop cut (decision 8)
CREATE UNIQUE INDEX issue_comments_external_once
  ON issue_comments (org_id, external_source, external_comment_id)
  WHERE external_source IS NOT NULL;

-- set once: NULL → value, never value → other value (an erase keeps it, as task 15's ids are kept)
CREATE TRIGGER issue_comments_external_frozen
  BEFORE UPDATE OF external_source, external_comment_id ON issue_comments
  FOR EACH ROW WHEN (OLD.external_source IS NOT NULL)
  EXECUTE FUNCTION hub.forbid_change();
```

`hub.forbid_change()` is the Hub's own freeze function as task 145's scope names it for requests
(`tasks-141-160.md:237`); its exact name on `feat/hub` is **UNVERIFIED**. `external_author_name`
joins task 14's erase list (it is a person's name). The org id's column name follows task 06.

#### Interfaces — complete: `worksync/comments.go`

```go
package worksync

import (
	"context"
	"errors"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// Commenter is the comment half every adapter adds (decision 7). Comments
// *in* arrive through Pull's Pulled.Comments; these are the two writes *out*.
type Commenter interface {
	// PostComment writes text on the item externalID and returns the new
	// comment's id as the tracker names it.
	PostComment(ctx context.Context, conn hubapi.TrackerConnection, externalID, text string) (string, error)
	// DeleteComment removes a comment this product posted (decision 10).
	DeleteComment(ctx context.Context, conn hubapi.TrackerConnection, externalID, externalCommentID string) error
}

const (
	CodeAUDataOnly         = "au_data_only"         // decision 9, once per run
	CodeErasedUpstream     = "erased_upstream"      // decision 10, the copy is gone
	CodeEraseNotPropagated = "erase_not_propagated" // decision 10, the tracker refused
	postSuffix             = " (PanaceaApp): "
)

// ErrNotPosted is ShouldPost's reason when a comment stays in the product.
var ErrNotPosted = errors.New("worksync: this comment never leaves the product")

// PushCandidate is what the push step knows about one of our comments.
type PushCandidate struct {
	Comment      hubapi.Comment
	WrittenBy    string // hubapi.ActorKind of the writer (task 143): only "person" posts
	IssueLinked  bool   // an ExternalLink through this connection exists
	ExternalID   string // that link's externalId
	ProjectAU    bool   // the project is AU data only (task 21's auOnly)
	DisplayName  string // the writer's display name
}

// ShouldPost is decision 7's out rule and decision 9's AU rule, in one place.
// It returns nil only for a person's top-level plain text·H comment, not yet
// crossed, on a linked issue of a comments-mode connection, in a project that
// is not AU data only.
func ShouldPost(conn hubapi.TrackerConnection, c PushCandidate) error {
	switch {
	case conn.Mode != hubapi.TrackerModeInAndComments,
		!c.IssueLinked,
		c.Comment.Kind != hubapi.CommentKindComment, // note, evidence_proposal, negotiation, edit_proposal
		c.Comment.Body.Author != hubapi.AuthorHuman, // every text·A
		c.Comment.ParentID != nil,                   // replies stay a conversation of ours
		c.Comment.External != nil,                   // imported, or already posted (decision 8)
		c.Comment.ErasedAt != nil,
		c.WrittenBy != "person":
		return ErrNotPosted
	case c.ProjectAU:
		return &PullError{Code: CodeAUDataOnly, Err: ErrNotPosted}
	}
	return nil
}

// OutboundText is "Dylan (PanaceaApp): …", cut to limit bytes with the cut
// shown, so the tracker's own comment limit never fails the post.
func OutboundText(displayName, body string, limit int) string {
	name := strings.TrimSpace(CutRunes(displayName, 80))
	if name == "" {
		name = "Someone"
	}
	return CutBytes(name+postSuffix+body, limit)
}

// NewInbound filters the comments a pull read (decision 8's first cut) and
// orders none: the engine writes them in the tracker's order. known holds the
// externalCommentIds already on our comments for this connection's source.
func NewInbound(in []hubapi.ExternalComment, known map[string]bool) []hubapi.ExternalComment {
	out := make([]hubapi.ExternalComment, 0, len(in))
	for _, c := range in {
		if known[c.ExternalCommentID] || strings.TrimSpace(c.Body) == "" {
			continue
		}
		known[c.ExternalCommentID] = true // twice in one pull is still once
		out = append(out, c)
	}
	return out
}
```

`ShouldPost` reads `Comment.ParentID` as the one-level threading of task 12: a reply on our side
answers our own conversation, and a tracker has no place for it. The tracker limits `OutboundText`
is called with are `jiraCommentMax` (32 KiB), `githubCommentMax` (65,536 characters, used as bytes,
the stricter reading) and `boardsCommentMax` (the same 32 KiB); each is our choice below the
tracker's documented ceiling as we remember it, **UNVERIFIED** on the day.

#### Interfaces — complete: `worksync/jira.go`, the pull

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
	jiraSearchPath   = "/rest/api/3/search/jql" // decision 1: the one place the endpoint lives
	jiraGatewayURL   = "https://api.atlassian.com"
	jiraCursorTag    = "j1"
	jiraPageSize     = 100
	jiraCommentMax   = 32 << 10
	jiraCommentsPull = 2000 // comments read per pull, all items together
)

// JiraTarget is TrackerConnection.Target for source jira (task-169.md:164).
type JiraTarget struct {
	Site       string `json:"site"`       // "acme.atlassian.net", host only
	ProjectKey string `json:"projectKey"` // "ABC"
}

type JiraConfig struct {
	Client  *http.Client
	Token   TokenFunc // the OAuth 2.0 access token (decision 2)
	BaseURL string    // jiraGatewayURL in production, httptest in tests
	Sleep   func(context.Context, time.Duration) error
	Now     func() time.Time
}

// Jira is task 150's Adapter and this task's Commenter for Jira Cloud.
type Jira struct {
	f    fetcher
	base string
}

func NewJira(c JiraConfig) *Jira {
	if c.BaseURL == "" {
		c.BaseURL = jiraGatewayURL
	}
	if c.Sleep == nil {
		c.Sleep = sleepCtx
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return &Jira{f: fetcher{client: c.Client, token: c.Token, sleep: c.Sleep, now: c.Now},
		base: strings.TrimRight(c.BaseURL, "/")}
}

func (j *Jira) Source() string { return hubapi.ImportSourceJira }

type jiraIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"` // ADF, or null
		Priority    *struct{ Name string } `json:"priority"`
		Labels      []string        `json:"labels"`
		Reporter    *struct{ DisplayName string } `json:"reporter"`
		Created     string          `json:"created"`
		Updated     string          `json:"updated"`
		Status      *struct{ Name string } `json:"status"`
		StatusDate  string          `json:"statuscategorychangedate"`
	} `json:"fields"`
}

// jiraTime reads Jira's "2026-09-25T11:02:00.000+1000" (no colon in the zone).
func jiraTime(s string) time.Time {
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339Nano, s)
	}
	return t
}

// Pull is decisions 1-4: resolve the site, read the caller's zone and id,
// page the JQL search, and in comments mode read each item's new comments.
func (j *Jira) Pull(ctx context.Context, conn hubapi.TrackerConnection, cur Cursor) (Pulled, error) {
	var t JiraTarget
	if err := json.Unmarshal(conn.Target, &t); err != nil || !jiraSite.MatchString(t.Site) || !jiraKey.MatchString(t.ProjectKey) {
		return Pulled{}, &PullError{Code: CodeTrackerRefused, Err: fmt.Errorf("jira target: %v", err)}
	}
	since, err := parseJiraCursor(cur)
	if err != nil {
		return Pulled{}, &PullError{Code: CodeTrackerShape, Err: err}
	}
	api, me, err := j.session(ctx, t.Site)
	if err != nil {
		return Pulled{}, err
	}
	jql := `project = "` + t.ProjectKey + `"`
	if !since.IsZero() {
		at := since.UTC().Add(-14 * time.Hour) // no zone: 14 h earlier than any zone reads it
		if me.zone != nil {
			at = since.In(me.zone)
		}
		jql += ` AND updated >= "` + at.Truncate(time.Minute).Format("2006/01/02 15:04") + `"`
	}
	jql += " ORDER BY updated ASC, key ASC"
	out := Pulled{Items: []hubapi.ImportItem{}, Upstream: []UpstreamChange{}, Comments: []hubapi.ExternalComment{}, Next: cur}
	var ids []string
	token, last, seen := "", since, 0
	for {
		if seen >= hubapi.MaxImportItems && last.After(since) {
			break
		}
		if seen >= githubTieMax {
			return Pulled{}, &PullError{Code: CodeTrackerStalled, Err: fmt.Errorf("over %d issues in the minute of %s", githubTieMax, since)}
		}
		body := map[string]any{"jql": jql, "maxResults": jiraPageSize, "fields": []string{"summary", "description",
			"priority", "labels", "reporter", "created", "updated", "status", "statuscategorychangedate"}}
		if token != "" {
			body["nextPageToken"] = token
		}
		var page struct {
			Issues        []jiraIssue `json:"issues"`
			NextPageToken string      `json:"nextPageToken"`
			IsLast        bool        `json:"isLast"`
		}
		if err := j.f.do(ctx, http.MethodPost, api+jiraSearchPath, body, &page, nil); err != nil {
			return Pulled{}, err
		}
		for _, is := range page.Issues {
			seen++
			if up := jiraTime(is.Fields.Updated); up.After(last) {
				last = up
			}
			item := is.toImport(t.Site)
			out.Items, ids = append(out.Items, item), append(ids, is.ID)
			if sc := jiraTime(is.Fields.StatusDate); !since.IsZero() && sc.After(since) {
				out.Upstream = append(out.Upstream, UpstreamChange{ExternalID: item.ExternalID, Field: "state", At: sc})
			}
		}
		if page.IsLast || page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	if conn.Mode == hubapi.TrackerModeInAndComments {
		if out.Comments, err = j.comments(ctx, api, t.Site, ids, since, me.accountID); err != nil {
			return Pulled{}, err
		}
	}
	if !last.IsZero() {
		out.Next = Cursor(jiraCursorTag + "|" + last.UTC().Format(time.RFC3339))
	}
	return out, nil
}

var (
	jiraSite = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}\.atlassian\.net$`)
	jiraKey  = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,9}$`)
)

type jiraMe struct {
	zone      *time.Location
	accountID string
}

// session resolves the site's cloudId (decision 2), then the caller's zone and
// account id (decisions 3 and 8), and returns the API root for the site.
func (j *Jira) session(ctx context.Context, site string) (string, jiraMe, error) {
	var res []struct{ ID, URL string }
	if err := j.f.do(ctx, http.MethodGet, j.base+"/oauth/token/accessible-resources", nil, &res, nil); err != nil {
		return "", jiraMe{}, err
	}
	api := ""
	for _, r := range res {
		if r.URL == "https://"+site {
			api = j.base + "/ex/jira/" + url.PathEscape(r.ID)
		}
	}
	if api == "" {
		return "", jiraMe{}, &PullError{Code: CodeTrackerRefused, Err: fmt.Errorf("token cannot reach %s", site)}
	}
	var me struct{ AccountID, TimeZone string }
	if err := j.f.do(ctx, http.MethodGet, api+"/rest/api/3/myself", nil, &me, nil); err != nil {
		return "", jiraMe{}, err
	}
	zone, err := time.LoadLocation(me.TimeZone)
	if err != nil {
		zone = nil // decision 3: an unknown zone widens the window, see Pull
	}
	return api, jiraMe{zone: zone, accountID: me.AccountID}, nil
}
```

`time.LoadLocation("")` returns UTC without an error, so the implementer treats an empty
`timeZone` as unknown too (`me.TimeZone == ""` joins the `err != nil` branch); the test
`TestJiraUnknownZoneWidens` pins both.

#### Interfaces — complete: `worksync/jira.go`, the mapping, ADF and the table

```go
// toImport maps one issue to SW.6 (decisions 4-6).
func (is jiraIssue) toImport(site string) hubapi.ImportItem {
	f := is.Fields
	prio, author, created := hubapi.PriorityNone, "", ""
	if f.Priority != nil {
		prio = jiraPriority(f.Priority.Name)
	}
	if f.Reporter != nil {
		author = CutRunes(f.Reporter.DisplayName, hubapi.MaxImportAuthorNameChars)
	}
	if c := jiraTime(f.Created); !c.IsZero() {
		created = c.UTC().Format(time.RFC3339)
	}
	labels := []string{}
	for _, l := range f.Labels {
		if l = strings.TrimSpace(CutRunes(l, hubapi.MaxLabelNameChars)); l != "" && len(labels) < hubapi.MaxImportLabels {
			labels = append(labels, l)
		}
	}
	return hubapi.ImportItem{
		ExternalID:  site + "#" + is.ID,
		URL:         "https://" + site + "/browse/" + url.PathEscape(is.Key),
		Title:       CutRunes(strings.TrimSpace(f.Summary), hubapi.MaxIssueTitleChars),
		Description: CutBytes(ADFText(f.Description), hubapi.MaxIssueDescriptionBytes),
		Priority:    prio, Labels: labels, AuthorName: author, CreatedAt: created,
	}
}

// jiraPriority is decision 6's fixed table; case-sensitive, as Jira's names are.
func jiraPriority(name string) hubapi.Priority {
	switch name {
	case "Highest":
		return hubapi.PriorityUrgent
	case "High":
		return hubapi.PriorityHigh
	case "Medium":
		return hubapi.PriorityMedium
	case "Low", "Lowest":
		return hubapi.PriorityLow
	}
	return hubapi.PriorityNone
}

type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
	Marks   []struct {
		Type  string `json:"type"`
		Attrs struct{ Href string } `json:"attrs"`
	} `json:"marks"`
}

// ADFText is decision 5. Invalid or null input is "".
func ADFText(raw json.RawMessage) string {
	var doc adfNode
	if len(raw) == 0 || json.Unmarshal(raw, &doc) != nil {
		return ""
	}
	var b strings.Builder
	adfWrite(&b, doc, "")
	return tidyLines(b.String())
}

func adfWrite(b *strings.Builder, n adfNode, prefix string) {
	switch n.Type {
	case "text":
		b.WriteString(n.Text)
		for _, m := range n.Marks {
			if m.Type == "link" && strings.HasPrefix(m.Attrs.Href, "https://") && m.Attrs.Href != n.Text {
				b.WriteString(" (" + m.Attrs.Href + ")")
			}
		}
		return
	case "hardBreak":
		b.WriteByte('\n')
		return
	case "codeBlock":
		b.WriteString("\n```\n")
		for _, c := range n.Content {
			adfWrite(b, c, "")
		}
		b.WriteString("\n```\n")
		return
	case "bulletList", "orderedList":
		for i, c := range n.Content {
			p := "- "
			if n.Type == "orderedList" {
				p = strconv.Itoa(i+1) + ". "
			}
			adfWrite(b, c, prefix+p)
		}
		return
	}
	if prefix != "" { // a listItem: its first block carries the marker
		b.WriteString(prefix)
	}
	for _, c := range n.Content {
		adfWrite(b, c, "")
	}
	switch n.Type {
	case "paragraph", "heading", "listItem", "tableRow", "blockquote", "panel", "tableCell", "tableHeader":
		b.WriteByte('\n')
	}
}
```

A list nested in a list item loses its indent (the marker is written, the depth is not). That is
accepted: the text stays readable and nothing is markup. The table test pins that each cell's text
arrives on its own line and no `{`, `"type"` or `table` word does.

#### Interfaces — complete: `worksync/jira.go`, comments both ways and the cursor

```go
// comments reads each pulled item's comments created after since, drops our
// own (decision 8's second cut), and stops at jiraCommentsPull.
func (j *Jira) comments(ctx context.Context, api, site string, ids []string, since time.Time, self string) ([]hubapi.ExternalComment, error) {
	out := []hubapi.ExternalComment{}
	for _, id := range ids {
		for start := 0; ; {
			var page struct {
				Comments []struct {
					ID      string          `json:"id"`
					Author  struct{ AccountID, DisplayName string } `json:"author"`
					Body    json.RawMessage `json:"body"`
					Created string          `json:"created"`
				} `json:"comments"`
				Total int `json:"total"`
			}
			u := fmt.Sprintf("%s/rest/api/3/issue/%s/comment?orderBy=created&startAt=%d&maxResults=100", api, url.PathEscape(id), start)
			if err := j.f.do(ctx, http.MethodGet, u, nil, &page, nil); err != nil {
				return nil, err
			}
			for _, c := range page.Comments {
				at := jiraTime(c.Created)
				if c.Author.AccountID == self || (!since.IsZero() && !at.After(since)) {
					continue
				}
				if len(out) == jiraCommentsPull {
					return nil, &PullError{Code: CodeTrackerStalled, Err: fmt.Errorf("over %d comments in one pull", jiraCommentsPull)}
				}
				out = append(out, hubapi.ExternalComment{ExternalID: site + "#" + id, ExternalCommentID: c.ID,
					AuthorName: CutRunes(c.Author.DisplayName, hubapi.MaxImportAuthorNameChars),
					Body:       CutBytes(ADFText(c.Body), hubapi.MaxIssueDescriptionBytes), At: at})
			}
			if start += len(page.Comments); len(page.Comments) == 0 || start >= page.Total {
				break
			}
		}
	}
	return out, nil
}

// PostComment writes text as one ADF paragraph per line (decision 7).
func (j *Jira) PostComment(ctx context.Context, conn hubapi.TrackerConnection, externalID, text string) (string, error) {
	api, id, err := j.itemAPI(ctx, conn, externalID)
	if err != nil {
		return "", err
	}
	paras := []map[string]any{}
	for _, line := range strings.Split(text, "\n") {
		p := map[string]any{"type": "paragraph", "content": []map[string]any{}}
		if line != "" {
			p["content"] = []map[string]any{{"type": "text", "text": line}}
		}
		paras = append(paras, p)
	}
	var res struct{ ID string }
	body := map[string]any{"body": map[string]any{"type": "doc", "version": 1, "content": paras}}
	if err := j.f.do(ctx, http.MethodPost, api+"/rest/api/3/issue/"+url.PathEscape(id)+"/comment", body, &res, nil); err != nil {
		return "", err
	}
	return res.ID, nil
}

// DeleteComment answers 204 with no body, so it reads the status itself
// rather than through decodeReply (task 151's fetcher decodes JSON only).
func (j *Jira) DeleteComment(ctx context.Context, conn hubapi.TrackerConnection, externalID, commentID string) error {
	api, id, err := j.itemAPI(ctx, conn, externalID)
	if err != nil {
		return err
	}
	res, err := j.f.once(ctx, http.MethodDelete, api+"/rest/api/3/issue/"+url.PathEscape(id)+"/comment/"+url.PathEscape(commentID), nil)
	if err != nil {
		return &PullError{Code: CodeTrackerUnreachable, Err: err}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusNotFound { // gone already is gone
		return &PullError{Code: CodeEraseNotPropagated, Err: fmt.Errorf("status %d", res.StatusCode)}
	}
	return nil
}

func (j *Jira) itemAPI(ctx context.Context, conn hubapi.TrackerConnection, externalID string) (string, string, error) {
	var t JiraTarget
	site, id, ok := strings.Cut(externalID, "#")
	if json.Unmarshal(conn.Target, &t) != nil || !ok || site != t.Site {
		return "", "", &PullError{Code: CodeTrackerShape, Err: fmt.Errorf("not an item of this connection: %q", externalID)}
	}
	api, _, err := j.session(ctx, site)
	return api, id, err
}

func parseJiraCursor(c Cursor) (time.Time, error) {
	if c == "" {
		return time.Time{}, nil
	}
	tag, ts, ok := strings.Cut(string(c), "|")
	if !ok || tag != jiraCursorTag {
		return time.Time{}, fmt.Errorf("not a jira cursor: %q", c)
	}
	return time.Parse(time.RFC3339, ts)
}

var (
	_ Adapter   = (*Jira)(nil)
	_ Commenter = (*Jira)(nil)
)
```

#### Interfaces — complete: the comment halves of `boards.go` ⚑ and `github.go` ⚑

Each adapter's `Pull` gains one call at its end, `if conn.Mode == hubapi.TrackerModeInAndComments
{ out.Comments, err = x.comments(…) }`, beside the item read, and the file gains the three methods
below. Azure Boards' comments API is a preview version, pinned in its own constant.

```go
// ── boards.go, appended ──────────────────────────────────────────────────────

const (
	boardsCommentsAPIVersion = "7.1-preview.4" // UNVERIFIED on the day; the one place it lives
	boardsCommentMax         = 32 << 10
)

// boardsSelf is the Hub identity's id in this organisation (decision 8), read
// from connectionData's authenticatedUser once per pull.
func (b *Boards) boardsSelf(ctx context.Context, org string) (string, error) {
	var cd struct {
		AuthenticatedUser struct{ ID string } `json:"authenticatedUser"`
	}
	err := b.f.do(ctx, http.MethodGet, b.base+"/"+url.PathEscape(org)+"/_apis/connectionData", nil, &cd, nil)
	return cd.AuthenticatedUser.ID, err
}

// comments reads each pulled item's comments after since, ours dropped.
func (b *Boards) comments(ctx context.Context, t BoardsTarget, items []hubapi.ImportItem, since time.Time) ([]hubapi.ExternalComment, error) {
	self, err := b.boardsSelf(ctx, t.Organisation)
	if err != nil {
		return nil, err
	}
	out := []hubapi.ExternalComment{}
	for _, it := range items {
		_, id, _ := strings.Cut(it.ExternalID, "#")
		for tok := ""; ; {
			var page struct {
				Comments []struct {
					ID          int    `json:"id"`
					Text        string `json:"text"` // HTML
					CreatedDate string `json:"createdDate"`
					CreatedBy   struct{ ID, DisplayName string } `json:"createdBy"`
				} `json:"comments"`
				ContinuationToken string `json:"continuationToken"`
			}
			u := fmt.Sprintf("%s/%s/%s/_apis/wit/workItems/%s/comments?api-version=%s&$top=200&continuationToken=%s",
				b.base, url.PathEscape(t.Organisation), url.PathEscape(t.Project), id, boardsCommentsAPIVersion, url.QueryEscape(tok))
			if err := b.f.do(ctx, http.MethodGet, u, nil, &page, nil); err != nil {
				return nil, err
			}
			for _, c := range page.Comments {
				at, _ := time.Parse(time.RFC3339Nano, c.CreatedDate)
				if c.CreatedBy.ID == self || (!since.IsZero() && !at.After(since)) {
					continue
				}
				if len(out) == jiraCommentsPull {
					return nil, &PullError{Code: CodeTrackerStalled, Err: fmt.Errorf("over %d comments in one pull", jiraCommentsPull)}
				}
				out = append(out, hubapi.ExternalComment{ExternalID: it.ExternalID, ExternalCommentID: strconv.Itoa(c.ID),
					AuthorName: CutRunes(c.CreatedBy.DisplayName, hubapi.MaxImportAuthorNameChars),
					Body:       CutBytes(PlainTextFromHTML(c.Text), hubapi.MaxIssueDescriptionBytes), At: at})
			}
			if tok = page.ContinuationToken; tok == "" {
				break
			}
		}
	}
	return out, nil
}

// PostComment posts text as escaped HTML with <br> for line breaks, so a
// person's "<b>" is shown as written, never rendered.
func (b *Boards) PostComment(ctx context.Context, conn hubapi.TrackerConnection, externalID, text string) (string, error) {
	t, id, err := boardsItemOf(conn, externalID)
	if err != nil {
		return "", err
	}
	u := fmt.Sprintf("%s/%s/%s/_apis/wit/workItems/%s/comments?api-version=%s", b.base,
		url.PathEscape(t.Organisation), url.PathEscape(t.Project), id, boardsCommentsAPIVersion)
	var res struct{ ID int }
	body := map[string]string{"text": strings.ReplaceAll(html.EscapeString(text), "\n", "<br>")}
	if err := b.f.do(ctx, http.MethodPost, u, body, &res, nil); err != nil {
		return "", err
	}
	return strconv.Itoa(res.ID), nil
}
```

```go
// DeleteComment answers 204; 404 is already gone (decision 10).
func (b *Boards) DeleteComment(ctx context.Context, conn hubapi.TrackerConnection, externalID, commentID string) error {
	t, id, err := boardsItemOf(conn, externalID)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/%s/%s/_apis/wit/workItems/%s/comments/%s?api-version=%s", b.base, url.PathEscape(t.Organisation),
		url.PathEscape(t.Project), id, url.PathEscape(commentID), boardsCommentsAPIVersion)
	return deleteStatus(b.f.once(ctx, http.MethodDelete, u, nil))
}

func boardsItemOf(conn hubapi.TrackerConnection, externalID string) (BoardsTarget, string, error) {
	var t BoardsTarget
	org, id, ok := strings.Cut(externalID, "#")
	if json.Unmarshal(conn.Target, &t) != nil || !ok || org != t.Organisation {
		return t, "", &PullError{Code: CodeTrackerShape, Err: fmt.Errorf("not an item of this connection: %q", externalID)}
	}
	return t, id, nil
}

// deleteStatus is shared by the three DeleteComments (Jira's inlines it above
// for clarity; the implementer may call this one there too).
func deleteStatus(res *http.Response, err error) error {
	if err != nil {
		return &PullError{Code: CodeTrackerUnreachable, Err: err}
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotFound {
		return &PullError{Code: CodeEraseNotPropagated, Err: fmt.Errorf("status %d", res.StatusCode)}
	}
	return nil
}

// ── github.go, appended ──────────────────────────────────────────────────────
// GitHubConfig gains AppID int64 (the same app as GitHubAppToken's), the key
// to decision 8's second cut.

const githubCommentMax = 65536

// comments reads the repository's issue comments since the cursor in one
// pager; comments on unlinked numbers (pull requests among them) are dropped
// by the engine, which keeps only linked externalIds.
func (g *GitHub) comments(ctx context.Context, t GitHubTarget, repo string, since time.Time) ([]hubapi.ExternalComment, error) {
	q := url.Values{"sort": {"created"}, "direction": {"asc"}, "per_page": {"100"}}
	if !since.IsZero() {
		q.Set("since", since.UTC().Format(time.RFC3339))
	}
	next := fmt.Sprintf("%s/repos/%s/%s/issues/comments?%s", g.base, url.PathEscape(t.Owner), url.PathEscape(t.Repository), q.Encode())
	out := []hubapi.ExternalComment{}
	for next != "" {
		var page []struct {
			ID        int64     `json:"id"`
			Body      string    `json:"body"`
			IssueURL  string    `json:"issue_url"`
			CreatedAt time.Time `json:"created_at"`
			User      struct{ Login string } `json:"user"`
			Via       *struct{ ID int64 } `json:"performed_via_github_app"`
		}
		var hdr http.Header
		if err := g.f.do(ctx, http.MethodGet, next, nil, &page, &hdr); err != nil {
			return nil, githubLimit(err, hdr)
		}
		for _, c := range page {
			if (c.Via != nil && c.Via.ID == g.appID) || (!since.IsZero() && !c.CreatedAt.After(since)) {
				continue // ours, or an edit of an older comment (since filters on updated)
			}
			if len(out) == jiraCommentsPull {
				return nil, &PullError{Code: CodeTrackerStalled, Err: fmt.Errorf("over %d comments in one pull", jiraCommentsPull)}
			}
			n := c.IssueURL[strings.LastIndexByte(c.IssueURL, '/')+1:]
			out = append(out, hubapi.ExternalComment{ExternalID: repo + "#" + n, ExternalCommentID: strconv.FormatInt(c.ID, 10),
				AuthorName: CutRunes(c.User.Login, hubapi.MaxImportAuthorNameChars),
				Body:       CutBytes(PlainTextFromHTML(c.Body), hubapi.MaxIssueDescriptionBytes), At: c.CreatedAt})
		}
		var err error
		if next, err = g.nextLink(hdr.Get("Link")); err != nil {
			return nil, err
		}
	}
	return out, nil
}
```

```go
// PostComment posts Markdown as written, except that every "@" is followed by
// a zero-width space: a person's "@alice" in our product must not notify a
// GitHub user who never joined it.
func (g *GitHub) PostComment(ctx context.Context, conn hubapi.TrackerConnection, externalID, text string) (string, error) {
	t, n, err := githubItemOf(conn, externalID)
	if err != nil {
		return "", err
	}
	u := fmt.Sprintf("%s/repos/%s/%s/issues/%s/comments", g.base, url.PathEscape(t.Owner), url.PathEscape(t.Repository), n)
	var res struct{ ID int64 }
	var hdr http.Header
	if err := g.f.do(ctx, http.MethodPost, u, map[string]string{"body": strings.ReplaceAll(text, "@", "@​")}, &res, &hdr); err != nil {
		return "", githubLimit(err, hdr)
	}
	return strconv.FormatInt(res.ID, 10), nil
}

func (g *GitHub) DeleteComment(ctx context.Context, conn hubapi.TrackerConnection, externalID, commentID string) error {
	t, _, err := githubItemOf(conn, externalID)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("%s/repos/%s/%s/issues/comments/%s", g.base, url.PathEscape(t.Owner), url.PathEscape(t.Repository), url.PathEscape(commentID))
	return deleteStatus(g.f.once(ctx, http.MethodDelete, u, nil))
}

func githubItemOf(conn hubapi.TrackerConnection, externalID string) (GitHubTarget, string, error) {
	var t GitHubTarget
	repo, n, ok := strings.Cut(externalID, "#")
	if json.Unmarshal(conn.Target, &t) != nil || !ok || repo != t.Owner+"/"+t.Repository {
		return t, "", &PullError{Code: CodeTrackerShape, Err: fmt.Errorf("not an item of this connection: %q", externalID)}
	}
	return t, n, nil
}

var (
	_ Commenter = (*Boards)(nil)
	_ Commenter = (*GitHub)(nil)
)
```

`boards.go` gains the standard library's `html` import (for `EscapeString`); `text.go`'s
`golang.org/x/net/html` is a different file's import and does not clash.

#### Interfaces — the engine's two steps: `worksync/engine.go` ⚑ (task 150; shape **UNVERIFIED**)

Written against task 150's scope: `Run` pulls, writes one store transaction per page, and appends a
`SyncRun`. Where 150's part names things differently, its names win (rule H1) and this logic moves
over unchanged.

```go
// commentsIn runs inside the page's transaction, after its items, in mode
// in_and_comments only. A comment on an externalId with no link is dropped:
// that is how pull-request comments and unlinked items never enter.
func (e *Engine) commentsIn(ctx context.Context, tx store.Tx, conn hubapi.TrackerConnection, in []hubapi.ExternalComment, run *hubapi.SyncRun) error {
	links, err := store.LinksByExternal(ctx, tx, conn.ProjectID, conn.Source, externalIDs(in))
	if err != nil {
		return err
	}
	known, err := store.KnownExternalComments(ctx, tx, conn.Source, commentIDs(in)) // decision 8, first cut
	if err != nil {
		return err
	}
	for _, c := range NewInbound(in, known) {
		link, ok := links[c.ExternalID]
		if !ok {
			continue
		}
		id := ids.Wide("ic-")
		if err := store.InsertExternalComment(ctx, tx, store.ExternalCommentRow{ID: id, IssueID: link.IssueID,
			Body: c.Body, Source: conn.Source, ExternalCommentID: c.ExternalCommentID, AuthorName: c.AuthorName,
			CreatedBy: conn.PrincipalID}); err != nil { // AuthorClass "H", Kind "comment", ParentID ""
			return err
		}
		// task 13's comment.created, actor null, data.via the tc- id (task 150's integration principal)
		if err := e.emit(ctx, tx, conn, "comment.created", map[string]any{"commentId": id, "issueId": link.IssueID}); err != nil {
			return err
		}
		run.CommentsIn++
	}
	return nil
}
```

```go
// push runs after the last page commits, in mode in_and_comments only, and
// only when the adapter is a Commenter. Candidates are our comments on issues
// linked through this connection, created after their link, not yet crossed:
// a person's history from before the connection is never posted.
func (e *Engine) push(ctx context.Context, conn hubapi.TrackerConnection, a Adapter, run *hubapi.SyncRun) error {
	cm, ok := a.(Commenter)
	if !ok || conn.Mode != hubapi.TrackerModeInAndComments {
		return nil
	}
	cands, err := store.PushCandidates(ctx, e.db, conn.ID, 200) // oldest first, at most 200 a run
	if err != nil {
		return err
	}
	auNoted := false
	for _, c := range cands {
		switch err := ShouldPost(conn, c); {
		case errors.Is(err, ErrNotPosted) && !auNoted && isCode(err, CodeAUDataOnly):
			run.Errors, auNoted = append(run.Errors, hubapi.SyncError{Code: CodeAUDataOnly}), true
			continue
		case err != nil:
			continue
		}
		text := OutboundText(c.DisplayName, c.Comment.Body.Text, commentMax(conn.Source))
		extID, err := cm.PostComment(ctx, conn, c.ExternalID, text)
		if err != nil {
			run.Errors = append(run.Errors, hubapi.SyncError{ExternalID: c.ExternalID, Code: codeOf(err)})
			continue // retried next run; the stored-id cut makes a retry safe on the way in
		}
		// one transaction: set issue_comments.external_* and append the metadata-only
		// event comment.crossed {commentId, source, direction: "out"} (a gap for task 143)
		if err := store.MarkPosted(ctx, e.db, c.Comment.ID, conn.Source, extID); err != nil {
			return err // the one window where a second copy can be posted (decision 8)
		}
		run.CommentsOut++
	}
	return e.eraseUpstream(ctx, conn, cm, run)
}

// eraseUpstream is decision 10: for each comment we posted that task 14 has
// since erased and whose outcome is still unset, delete the posted copy and
// record erased_upstream or erase_not_propagated on the row and in the run.
func (e *Engine) eraseUpstream(ctx context.Context, conn hubapi.TrackerConnection, cm Commenter, run *hubapi.SyncRun) error {
	rows, err := store.ErasedPosted(ctx, e.db, conn.ID, 200)
	if err != nil {
		return err
	}
	for _, r := range rows {
		outcome := CodeErasedUpstream
		if err := cm.DeleteComment(ctx, conn, r.ExternalID, r.ExternalCommentID); err != nil {
			outcome = CodeEraseNotPropagated
		}
		run.Errors = append(run.Errors, hubapi.SyncError{ExternalID: r.ExternalID, Code: outcome})
		if err := store.SetEraseOutcome(ctx, e.db, r.CommentID, outcome); err != nil {
			return err
		}
	}
	return nil
}
```

`isCode`, `codeOf` (a `*PullError`'s code, else `tracker_unreachable`) and `commentMax` (the
three `*CommentMax` constants by source) are three-line helpers in `comments.go`. The store calls
(`LinksByExternal`, `KnownExternalComments`, `InsertExternalComment`, `PushCandidates`, `MarkPosted`,
`ErasedPosted`, `SetEraseOutcome`) go in task 150's `store/work_trackers.go`; each is one statement
over the columns proposed above plus one more, `issue_comments.external_erase_outcome text`
(`NULL` until decision 10 has run), which joins the gap for task 145. `erased_upstream` is recorded
in `SyncRun.errors` although it is not a failure, because `SyncRun` has no other list; a gap below.

#### Tests to write first — `worksync/comments_test.go` (pure, complete)

```go
package worksync

import (
	"errors"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

var commentsConn = hubapi.TrackerConnection{ID: "tc-3", Source: hubapi.ImportSourceJira, Mode: hubapi.TrackerModeInAndComments}

func plain() PushCandidate {
	return PushCandidate{Comment: hubapi.Comment{ID: "ic-1", Kind: hubapi.CommentKindComment,
		Body: hubapi.AuthoredText{Text: "looks right", Author: hubapi.AuthorHuman}},
		WrittenBy: "person", IssueLinked: true, ExternalID: "acme.atlassian.net#10001", DisplayName: "Dylan"}
}

// Kills "agent text leaves the product" and every other out-rule removal.
func TestShouldPostOnlyAPersonsPlainComment(t *testing.T) {
	if err := ShouldPost(commentsConn, plain()); err != nil {
		t.Fatalf("a person's plain comment was held: %v", err)
	}
	parent := "ic-0"
	ext := &hubapi.CommentExternal{Source: "jira", ExternalCommentID: "9"}
	for name, mut := range map[string]func(*PushCandidate){
		"note":              func(c *PushCandidate) { c.Comment.Kind = "note" },
		"evidence_proposal": func(c *PushCandidate) { c.Comment.Kind = "evidence_proposal" },
		"negotiation":       func(c *PushCandidate) { c.Comment.Kind = "negotiation" },
		"edit_proposal":     func(c *PushCandidate) { c.Comment.Kind = "edit_proposal" },
		"text·A":            func(c *PushCandidate) { c.Comment.Body.Author = hubapi.AuthorAgent },
		"reply":             func(c *PushCandidate) { c.Comment.ParentID = &parent },
		"already crossed":   func(c *PushCandidate) { c.Comment.External = ext }, // kills "echo loop" out
		"by the principal":  func(c *PushCandidate) { c.WrittenBy = "integration" },
		"unlinked":          func(c *PushCandidate) { c.IssueLinked = false },
	} {
		c := plain()
		mut(&c)
		if err := ShouldPost(commentsConn, c); !errors.Is(err, ErrNotPosted) {
			t.Errorf("%s: posted (err %v)", name, err)
		}
	}
}

// Kills "mode ignored" on the way out.
func TestShouldPostHoldsEverythingInModeIn(t *testing.T) {
	in := commentsConn
	in.Mode = hubapi.TrackerModeIn
	if err := ShouldPost(in, plain()); !errors.Is(err, ErrNotPosted) {
		t.Fatalf("mode in posted a comment: %v", err)
	}
}

// Kills "AU text leaves the product" (decision 9).
func TestShouldPostHoldsAUDataOnly(t *testing.T) {
	c := plain()
	c.ProjectAU = true
	var pe *PullError
	if err := ShouldPost(commentsConn, c); !errors.As(err, &pe) || pe.Code != CodeAUDataOnly {
		t.Fatalf("want au_data_only, got %v", err)
	}
}

// Kills "comments duplicated": the second run's known set holds the first
// run's ids, so the same pull writes nothing more.
func TestNewInboundArrivesOnce(t *testing.T) {
	pulled := []hubapi.ExternalComment{{ExternalID: "x#1", ExternalCommentID: "7", Body: "hi"},
		{ExternalID: "x#1", ExternalCommentID: "7", Body: "hi"}, {ExternalID: "x#1", ExternalCommentID: "8", Body: " "}}
	known := map[string]bool{}
	if got := NewInbound(pulled, known); len(got) != 1 {
		t.Fatalf("run 1 wrote %d, want 1 (a duplicate in one pull, and an empty body)", len(got))
	}
	if got := NewInbound(pulled, known); len(got) != 0 {
		t.Fatalf("run 2 wrote %d, want 0", len(got))
	}
}

func TestOutboundTextNamesThePersonAndFits(t *testing.T) {
	got := OutboundText("Dylan", strings.Repeat("a", 40000), jiraCommentMax)
	if !strings.HasPrefix(got, "Dylan (PanaceaApp): ") || len(got) > jiraCommentMax || !strings.HasSuffix(got, cutMark) {
		t.Fatalf("prefix or cut wrong: %q…, %d bytes", got[:24], len(got))
	}
}
```

#### Tests to write first — `worksync/jira_test.go` (complete)

```go
package worksync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// fakeJira serves accessible-resources, myself, one search page from
// testdata/jira/search-page.json, and one comment page; it counts comment reads.
func fakeJira(t *testing.T, zone string, commentReads *int, lastJQL *string) *Jira {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch p := r.URL.Path; {
		case p == "/oauth/token/accessible-resources":
			w.Write([]byte(`[{"id":"c-1","url":"https://acme.atlassian.net"}]`))
		case p == "/ex/jira/c-1/rest/api/3/myself":
			w.Write([]byte(`{"accountId":"self-1","timeZone":"` + zone + `"}`))
		case p == "/ex/jira/c-1"+jiraSearchPath:
			var q struct{ JQL string }
			json.NewDecoder(r.Body).Decode(&q)
			*lastJQL = q.JQL
			w.Write(readTestdata(t, "jira/search-page.json"))
		case strings.HasSuffix(p, "/comment"):
			*commentReads++
			w.Write(readTestdata(t, "jira/comments-page.json"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return NewJira(JiraConfig{Client: srv.Client(), BaseURL: srv.URL,
		Token: func(context.Context) (string, error) { return "oauth", nil }})
}

func jiraConn(mode string) hubapi.TrackerConnection {
	return hubapi.TrackerConnection{ID: "tc-4", Source: hubapi.ImportSourceJira, Mode: mode,
		Target: json.RawMessage(`{"site":"acme.atlassian.net","projectKey":"ABC"}`)}
}

// Kills "ADF dropped" and "ADF markup imported".
func TestADFTextKeepsCellsAndNoMarkup(t *testing.T) {
	raw := readTestdata(t, "jira/adf-table.json")
	got := ADFText(raw)
	for _, want := range []string{"Steps", "- one", "cell A", "cell B", "see docs (https://acme.example/d)", "```\nx := 1\n```"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.ContainsAny(got, "{}") || strings.Contains(got, `"type"`) || strings.Contains(got, "tableCell") {
		t.Fatalf("markup survived: %q", got)
	}
}

// Kills "unknown priority fails the run" and the table's shift.
func TestJiraPriorityTable(t *testing.T) {
	for in, want := range map[string]hubapi.Priority{"Highest": hubapi.PriorityUrgent, "High": hubapi.PriorityHigh,
		"Medium": hubapi.PriorityMedium, "Low": hubapi.PriorityLow, "Lowest": hubapi.PriorityLow,
		"Blocker": hubapi.PriorityNone, "": hubapi.PriorityNone} {
		if got := jiraPriority(in); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}

// Kills "two way means states": a transition to Done is an UpstreamChange;
// the item has no state; the id is the numeric one (decision 4).
func TestJiraDoneIsUpstreamOnly(t *testing.T) {
	var reads int
	var jql string
	got, err := fakeJira(t, "Australia/Sydney", &reads, &jql).Pull(context.Background(), jiraConn(hubapi.TrackerModeIn), "j1|2026-09-25T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Upstream) != 1 || got.Upstream[0].ExternalID != "acme.atlassian.net#10001" || got.Upstream[0].Field != "state" {
		t.Fatalf("upstream %+v", got.Upstream)
	}
	raw, _ := json.Marshal(got.Items[0])
	if strings.Contains(string(raw), "Done") || got.Items[0].URL != "https://acme.atlassian.net/browse/ABC-12" {
		t.Fatalf("item %s", raw)
	}
	if !strings.Contains(jql, `updated >= "2026/09/25 20:00"`) { // 10:00Z in Sydney (AEST, +10) on that date
		t.Fatalf("jql %q", jql)
	}
	if reads != 0 || len(got.Comments) != 0 { // kills "mode ignored" on the way in
		t.Fatalf("mode in read %d comment pages", reads)
	}
}
```

```go
// Kills "echo loop" on the way in: the comment written by our own account is
// dropped; the person's arrives with the author's name as display text.
func TestJiraCommentsDropOurOwn(t *testing.T) {
	var reads int
	var jql string
	got, err := fakeJira(t, "UTC", &reads, &jql).Pull(context.Background(), jiraConn(hubapi.TrackerModeInAndComments), "j1|2026-09-25T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if reads != 1 || len(got.Comments) != 1 {
		t.Fatalf("%d reads, comments %+v", reads, got.Comments)
	}
	c := got.Comments[0]
	if c.ExternalCommentID != "501" || c.AuthorName != "Ana Outside" || c.Body != "Seen on Safari 17" {
		t.Fatalf("comment %+v", c)
	}
}

// Kills "unknown zone narrows the window": an empty or unknown timeZone reads
// 14 hours earlier in UTC wall-clock terms, never later.
func TestJiraUnknownZoneWidens(t *testing.T) {
	for _, zone := range []string{"", "Mars/Olympus"} {
		var reads int
		var jql string
		if _, err := fakeJira(t, zone, &reads, &jql).Pull(context.Background(), jiraConn(hubapi.TrackerModeIn), "j1|2026-09-25T10:00:00Z"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(jql, `updated >= "2026/09/24 20:00"`) {
			t.Errorf("zone %q: jql %q", zone, jql)
		}
	}
}

var _ = time.Second // the file's time import is used by the helpers above in the full file
```

#### Testdata — `worksync/testdata/jira/` (complete, ours)

`search-page.json`:

```json
{"issues": [{"id": "10001", "key": "ABC-12", "fields": {"summary": "Resume upload fails",
  "description": {"type": "doc", "version": 1, "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Steps"}]}]},
  "priority": {"name": "Highest"}, "labels": ["web"], "reporter": {"displayName": "Ana Outside"},
  "created": "2026-09-20T09:00:00.000+1000", "updated": "2026-09-25T21:03:00.000+1000",
  "status": {"name": "Done"}, "statuscategorychangedate": "2026-09-25T21:02:00.000+1000"}}],
 "isLast": true}
```

`comments-page.json`:

```json
{"startAt": 0, "total": 2, "comments": [
  {"id": "500", "author": {"accountId": "self-1", "displayName": "PanaceaApp"},
   "body": {"type": "doc", "version": 1, "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Dylan (PanaceaApp): looks right"}]}]},
   "created": "2026-09-25T21:04:00.000+1000"},
  {"id": "501", "author": {"accountId": "acc-9", "displayName": "Ana Outside"},
   "body": {"type": "doc", "version": 1, "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Seen on Safari 17"}]}]},
   "created": "2026-09-25T21:05:00.000+1000"}]}
```

`adf-table.json`: a `doc` holding, in order, a paragraph `Steps`; a `bulletList` with one item
`one`; a `table` of one `tableRow` with two `tableCell`s holding paragraphs `cell A` and `cell B`;
a paragraph whose text `see docs` carries a `link` mark to `https://acme.example/d`; and a
`codeBlock` with the text `x := 1`. The implementer writes it as JSON in that order; the test above
is its specification.

#### Integration rows — `server/work_trackers_integration_test.go` ⚑ (task 150's; `-tags integration`)

Against task 150's fake adapter, extended to implement `Commenter` and record its posts:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestExternalCommentArrivesOnceAcrossRuns` | two runs over the same pulled comment write one `ic-` row with `external_comment_id`, `author_class = 'H'`, `created_by` the principal, and one `comment.created` event with `actor: null` | "comments duplicated"; "imported as a person" |
| `TestPostedCommentIsNotReimported` | a person's comment is posted once (one fake post), `external_*` set; the next pull returns the tracker's copy under the posted id and no second row is written | "echo loop" |
| `TestNoteOnALinkedIssueNeverLeaves` | an Orchestrator's `note` and a `text·A` comment on a linked issue: zero fake posts after two runs | "agent text leaves the product" |
| `TestModeInWritesNoCommentEitherWay` | in mode `in`, a pulled comment writes no row and a person's comment makes no post | "mode ignored" |
| `TestUpstreamDoneLeavesOurStateAlone` | a pull whose `Upstream` holds a `state` change leaves the issue in `triage` and its revision count at 1 | "two way means states" |
| `TestErasedPostedCommentIsDeletedUpstream` | task 14's erase of a posted comment, then a run: one fake delete, `external_erase_outcome = 'erased_upstream'`; a fake delete returning 403 gives `erase_not_propagated` | "erase stays upstream" |
| `TestHistoryBeforeTheLinkIsNotPosted` | a person's comment written before the link exists: zero posts | "a person's history posted" |

#### Mutants for the pure and adapter tests, applied by hand

| Mutant | Test that fails |
|---|---|
| `adfWrite`'s default branch skips children (a table's cells lost) | `TestADFTextKeepsCellsAndNoMarkup` |
| `ADFText` returns `string(raw)` | `TestADFTextKeepsCellsAndNoMarkup` |
| `"Highest"` maps to `PriorityHigh`, or an unknown name returns an error | `TestJiraPriorityTable` |
| `toImport` adds `Fields.Status.Name` to `Labels` | `TestJiraDoneIsUpstreamOnly` |
| `ExternalID` is `is.Key` | `TestJiraDoneIsUpstreamOnly` |
| `Pull` reads comments whatever the mode | `TestJiraDoneIsUpstreamOnly` |
| `comments` keeps `AccountID == self` | `TestJiraCommentsDropOurOwn` |
| an unknown zone falls back to UTC with no margin | `TestJiraUnknownZoneWidens` |
| any one `ShouldPost` case removed | `TestShouldPostOnlyAPersonsPlainComment` (its row) |
| the mode check removed from `ShouldPost` | `TestShouldPostHoldsEverythingInModeIn` |
| the `ProjectAU` case removed | `TestShouldPostHoldsAUDataOnly` |
| `NewInbound` does not add to `known` | `TestNewInboundArrivesOnce` |

#### Steps

1. Confirm tasks 150, 151, 152 and 143 are on the sync base; check whether task 143's part carries
   `ExternalComment`, `CommentExternal`, `TrackerModeInAndComments` and `ImportSourceJira`. Add
   only what is absent, from the wire section above, and say so in the commit (rule H1).
2. Confirm task 145's migration carries the `issue_comments` columns and `external_erase_outcome`.
   If not, **stop** and raise the gap to the orchestrator: this task does not write a migration.
3. Write `comments_test.go`, `jira_test.go` and the three testdata files; the package fails to
   compile. Write `comments.go`, then `jira.go`; run green.
4. Add the comment halves to `boards.go` and `github.go` (`GitHubConfig.AppID` and the `appID`
   field included); extend task 151's and 152's fakes with a comments path; run green.
5. Add `commentsIn`, `push` and `eraseUpstream` to `engine.go`, the store calls to
   `store/work_trackers.go`, and the integration rows; run them with `-tags integration`.
6. Apply each mutant by hand, see its test fail, revert it. Commit only the files listed above.

#### Acceptance

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/hub/worksync/... -race -count=1 -v -run "ADF|Jira|ShouldPost|NewInbound|OutboundText"'
# expect: TestADFTextKeepsCellsAndNoMarkup, TestJiraPriorityTable, TestJiraDoneIsUpstreamOnly,
#         TestJiraCommentsDropOurOwn, TestJiraUnknownZoneWidens, TestShouldPostOnlyAPersonsPlainComment,
#         TestShouldPostHoldsEverythingInModeIn, TestShouldPostHoldsAUDataOnly, TestNewInboundArrivesOnce,
#         TestOutboundTextNamesThePersonAndFits: PASS; ok
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/hub/worksync/... -race -count=1'
# expect: ok (tasks 151 and 152's tests still pass)
~/.claude/tools/claudezsh/claudezsh 'go test -tags integration ./control-plane/internal/hub/server/... -race -count=1 -run "ExternalComment|PostedComment|NoteOnALinked|ModeInWrites|UpstreamDone|ErasedPosted|HistoryBefore"'
# expect: the seven rows PASS against control-plane/cmd/hub/compose.yaml; ok
```

#### What this task does NOT claim

- That any tracker's API answers as the fixtures do: they are **our** shapes. Which Jira search
  endpoint is live, and Azure Boards' comments preview version, are **UNVERIFIED** on the day.
- That erased comments can be deleted in every tracker, or by our identity: a refusal is recorded
  as `erase_not_propagated`, never hidden.
- That a crash between a tracker's reply and our commit cannot post a second copy (decision 8
  says it can; the way in is safe, the way out is at-least-once).
- Jira with an API token, Jira Data Center, or Azure DevOps Server: none is supported here.
- That the Hub reaches `api.atlassian.com` (**UNVERIFIED**), or that anyone has granted the OAuth
  app to a Jira site (a site admin's act).
- That upstream edits of a comment are shown: an edit of an already imported comment is not read.

#### Gaps found while writing this part

1. **Task 145: comment columns.** `issue_comments` needs `external_source`, `external_comment_id`,
   `external_author_name` and `external_erase_outcome`, the unique index and the freeze trigger
   (complete SQL above). Without them nothing in this task can store a crossing.
2. **Task 143: types and one event kind.** `ExternalComment`, `CommentExternal` (with
   `Comment.External`), `TrackerModeIn`/`TrackerModeInAndComments`, and the event kind
   `comment.crossed` for a post's metadata-only event (task 13 lists the comment kinds).
3. **Task 143 / 150: `SyncRun` has no outcome list.** `erased_upstream` and `au_data_only` are not
   failures but can only go in `SyncRun.errors`. Proposed: `notes []SyncError` beside `errors`.
4. **Task 150: the connect route should refuse `in_and_comments` for an AU data only project**
   (decision 9); the push step's check is the second layer, not the only one.
5. **Task 151's fetcher reads JSON only.** Each `DeleteComment` reads the status itself
   (`deleteStatus`), and Jira's API-token (basic auth) credential cannot be sent. A
   `fetcher.header` hook in `fetch.go` would admit both; it is task 151's file, so it is left.
6. **Decision 4 refines the scope's key-as-id** to `<site>#<numeric id>`; task 171's sync log should
   show the key from the URL, not the id.
