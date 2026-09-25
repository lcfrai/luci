### Task 25: Control plane: the Linear public-API and CSV import adapters

Stream **WP**, consumes seam **SW.6** (task 15's normalised batch, idempotent per project by
`(source, externalId)`), about 3 hours (an estimate, not a measurement). Depends on task 15 (the
Hub's import route) and task 16 (`hubclient.Work.ImportBatch` over HTTP and `Deps.Work`).

The demo's first step is *"Import a Linear backlog or a CSV; it lands in Triage as not tied ·
imported"* (05 §5.3). Task 15 made the Hub's half: a batch of normalised items goes in, each new one
lands in `triage` with thread state `not_tied · imported`, and a re-run writes nothing. This task
makes the two things that produce that batch on the person's machine: a **pager** over Linear's
public GraphQL API, and a **CSV parser**. Both are pure adapters. Neither decides a state, a
thread state, an author or a number; the Hub does all four.

**Nothing imported sets a state.** Linear's workflow state is not read into the batch at all, and
SW.6's `ImportItem` has no field that could carry one. An issue that was *Done* in Linear arrives
in `triage` here, because in PanaceaApp `done` means **proven** (the skeleton's rule 2) and an
import proves nothing. The person triages it. 05 §2.3 point 4 is the rule; `TestLinearStateIsNotCarried`
pins it.

**The Linear API key lives for one request.** It arrives in the request body, is held in a local
variable of the handler, goes into exactly one header on requests to exactly one host, and is
dropped when the handler returns. It is never logged (the handler's log lines carry counts and
Linear's error codes only), never written under `statepath` or anywhere else, never echoed in a
problem body, and never sent anywhere but `https://api.linear.app/graphql`. The HTTP client
refuses every redirect, so a redirect cannot carry the header to another host. Whether that host is
reachable from the corporate network is **UNVERIFIED**; the CSV path is the fallback, and it needs
no network at all.

**Clean room** (`OD-11`). The query and the fixtures under `workimport/testdata/` are written by
hand from Linear's public API documentation (the `issues` connection, `nodes`, `pageInfo
{hasNextPage endCursor}`, `priority` 0 to 4). No response is recorded from the live API and
copied in, and no Linear code is read. The field names below are the public schema's as documented;
they are **UNVERIFIED against the live API** from this network, and the pager's first live run is
the check.

**Refusals are per line, and counts are plain.** A row or an issue the adapter cannot turn into a
valid item is **refused locally** with the CSV line number or the Linear identifier and one reason,
and the rest go on. That matters because task 15's batch is **all or nothing**: one invalid item
fails its whole call. So the adapter checks every rule task 15 checks (title 1-200 characters,
description at most 16 KiB, `url` `https` or empty, `externalId` 1-200 characters) before sending,
and sends chunks of at most **100** items (task 15 allows 500; 100 keeps one refused chunk small).
The answer is `{created, unchanged, refused, unmatchedLabels}`: two counts, a list and a list of
names, never a percentage (`OD-27`).

**Three corrections to the scope, made against SW.6's owner (task 15).**

1. **The Linear identifier is not kept as an `externalKey`.** The scope said "the source identifier
   kept as `externalKey`"; SW.6's `ImportItem` has no such field (`externalId`, `url`, `title`,
   `description`, `priority`, `labels`, `authorName`, `createdAt`). `externalId` is Linear's issue
   `id`, which is stable, and not its `identifier` (`ENG-123`), which changes when a team key does:
   the same reason our own `EP-142` is never a key. The identifier survives in the item's `url`
   and in `refused[].at`. A field for it is a gap, raised in the return, and additive if wanted.
2. **The answer gains `unmatchedLabels`.** Task 15 returns the label names no team label matched,
   and never creates them (a label is an admin write). The person needs to see them, so the wire
   carries them.
3. **Preview cannot say *created* versus *unchanged*.** SW.6 has no dry run, and the Hub is the only
   place that knows which `(source, externalId)` pairs exist. So preview answers `ready` (the items
   that would be sent) and `refused`, and `created` and `unchanged` are `null` until the import
   runs. A dry-run flag on task 15's route is a gap, raised in the return.

**One limit the built decoder imposes.** `httpx.DecodeJSON` caps a body at `MaxBodyBytes = 1 << 20`
(`httpx/problem.go:22`, `:126`, read at `c5841bf`). A 5,000-row CSV inside a JSON string can exceed
1 MiB, so the import routes decode with their own `http.MaxBytesReader` of **8 MiB** and the same
strict rules (`DisallowUnknownFields`, one value, the same problem body on failure).

**Files:**
- `control-plane/internal/workimport/batch.go` — `Item`, `Refusal`, `Plan`, `Validate`, `Chunks`,
  `Send` (new)
- `control-plane/internal/workimport/linear.go` — `LinearPager`, the query, the priority map (new)
- `control-plane/internal/workimport/csv.go` — `ParseCSV`, the digest (new)
- `control-plane/internal/workimport/{batch,linear,csv}_test.go` (new)
- `control-plane/internal/workimport/testdata/linear-page-{1,2}.json`, `linear-ratelimited.json`,
  `csv-{basic,crlf,bom,no-title,bad-priority}.csv` — hand-written fixtures (new)
- `control-plane/internal/api/workimport.go` + `workimport_test.go` — the two routes (new)
- `control-plane/internal/api/api.go` ⚑ — `Deps.WorkImport *workimport.Importer` (optional) and
  `a.registerWorkImport(guarded)` (modified)
- `control-plane/internal/api/routes_audit_test.go` ⚑ — the two patterns are POSTs (modified)
- `control-plane/cmd/api/main.go` ⚑ — build the `Importer` when `Deps.Work` exists. **No new
  variable**: the Linear endpoint is a constant, and only a test sets `LinearPager.Endpoint`, so no
  configuration can point the key at another host (modified)

**Interfaces:**
- Produces: the wire below; `workimport.Importer` with `Preview` and `Import`; `workimport.Item`,
  `Refusal`, `ParseCSV`, `LinearPager`. Consumed by task 69 (the import dialog) and W5's sync
  engine (task 150 reuses `Validate` and `Chunks`)
- Consumes:
  - task 03/15 — `hubapi.ImportBatch{Source, Items}`, `hubapi.ImportItem{ExternalID, URL, Title,
    Description, Priority, Labels, AuthorName, CreatedAt}`, `hubapi.ImportResult{Created,
    Existing, UnmatchedLabels}`, `hubapi.Priority` and its five values (assumed
    `hubapi.PriorityNone` … `PriorityLow`, the spelling task 11's part uses at `task-11.md:96`)
  - task 16 — `hubclient.Work.ImportBatch(ctx, projectID, batch) (hubapi.ImportResult, error)`
    (task 16 part `:403`), `hubclient.WriteError`, `Deps.Work`
  - built — `httpx.WriteProblem`, `httpx.MaxBodyBytes`
- Amends nothing in the Hub.

#### Contract — the wire

```jsonc
// POST /v1/work/import/preview   and   POST /v1/work/import   (the same body)
{ "projectId": "p-…",
  "source": "linear",                                   // "linear" | "csv"
  "linear": { "apiKey": "lin_api_…", "teamKey": "ENG" }, // with source linear; teamKey optional
  "csv":    { "content": "title,description\n…" } }     // with source csv; at most 8 MiB, 5,000 rows

// 200 from /import
{ "created": 41, "unchanged": 3,
  "refused": [ { "at": "line 17", "reason": "title is empty" },
               { "at": "ENG-212", "reason": "description is over 16 KiB" } ],
  "unmatchedLabels": ["Infra"] }

// 200 from /preview: no Hub write, and no Hub read of existing pairs (SW.6 has no dry run)
{ "ready": 44, "created": null, "unchanged": null, "refused": [ … ], "unmatchedLabels": null }
```

| Refusal | Status, code | `details` |
|---|---|---|
| unknown field, missing `projectId`, `source` not `linear`/`csv`, the wrong sub-object for the source, both sub-objects | 400 `bad_request` | `field` |
| body over 8 MiB, or more than 5,000 data rows | 413 `too_large` / 400 `bad_request` | `limit` |
| no Hub | 503 `hub_not_configured` | — |
| Linear answers 401 or 403 | 422 `import_source_refused` | `source: "linear"`, `status` |
| Linear unreachable, or still rate-limited after five waits | 503 `import_source_unavailable` | `source: "linear"` |
| the Hub refuses a chunk | the relay rule (hub-contract §4.3), and the counts of chunks already sent are in `details.created`, `details.unchanged` | as the Hub's |

`refused[].at` is `"line N"` (1-based, counting the header as line 1, as a spreadsheet shows it) or
Linear's `identifier`. A Hub refusal of a chunk after earlier chunks succeeded is not rolled back:
those issues exist, and a re-run finds them `unchanged`, which is what idempotency is for.

#### Interfaces — complete

```go
// workimport/batch.go
package workimport

// Item is one external issue, normalised, before it becomes an hubapi.ImportItem. At names it
// for a refusal: "line N" or a Linear identifier.
type Item struct {
	At   string
	Item hubapi.ImportItem
}

// Refusal is one item the adapter did not send, and why, in one plain sentence.
type Refusal struct {
	At     string `json:"at"`
	Reason string `json:"reason"`
}

// Limits, task 15's, checked here so no chunk is refused whole for one bad item.
const (
	MaxTitle       = 200
	MaxDescription = 16 << 10
	MaxExternalID  = 200
	ChunkSize      = 100
	MaxRows        = 5000
)

// Validate returns "" for an item task 15 would accept, else the reason. Title is trimmed
// first; an empty title is refused, never filled in. Labels are trimmed, empty ones dropped,
// and deduplicated case-insensitively, keeping the first spelling.
func Validate(it *hubapi.ImportItem) string {
	it.Title = strings.TrimSpace(it.Title)
	switch {
	case it.Title == "":
		return "title is empty"
	case utf8.RuneCountInString(it.Title) > MaxTitle:
		return "title is over 200 characters"
	case len(it.Description) > MaxDescription:
		return "description is over 16 KiB"
	case it.ExternalID == "" || utf8.RuneCountInString(it.ExternalID) > MaxExternalID:
		return "the source id is empty or over 200 characters"
	case it.URL != "" && !strings.HasPrefix(it.URL, "https://"):
		return "the link is not https"
	}
	it.Labels = cleanLabels(it.Labels)
	return ""
}

// Plan splits items into those to send and those refused, in input order. A second item with
// an ExternalID already planned is refused "duplicate of <At>": task 15's unique index would
// otherwise fail the chunk that holds both.
func Plan(items []Item) (send []hubapi.ImportItem, refused []Refusal)

// Chunks cuts send into slices of at most ChunkSize, in order. Never an empty chunk.
func Chunks(send []hubapi.ImportItem) [][]hubapi.ImportItem

// Batcher is the one hubclient.Work method import needs; Deps.Work satisfies it.
type Batcher interface {
	ImportBatch(ctx context.Context, projectID string, batch hubapi.ImportBatch) (hubapi.ImportResult, error)
}

// Outcome is the /import answer.
type Outcome struct {
	Created         int       `json:"created"`
	Unchanged       int       `json:"unchanged"`
	Refused         []Refusal `json:"refused"`         // [] when none
	UnmatchedLabels []string  `json:"unmatchedLabels"` // [] when none; sorted, deduplicated
}

// Send posts each chunk in order and sums the answers: Created += len(res.Created), Unchanged
// += len(res.Existing), UnmatchedLabels ∪= res.UnmatchedLabels. The first error stops it and is
// returned with the Outcome so far, which the handler puts in the problem's details.
func Send(ctx context.Context, b Batcher, projectID, source string, chunks [][]hubapi.ImportItem) (Outcome, error)
```

```go
// workimport/linear.go
package workimport

// LinearEndpoint is the only host the key is ever sent to.
const LinearEndpoint = "https://api.linear.app/graphql"

// linearQuery is written from Linear's public GraphQL documentation (the issues connection and
// its pageInfo). It asks for nothing the batch does not carry: no state, no assignee, no
// comments. The team filter is added only when teamKey is set.
const linearQuery = `query Issues($first: Int!, $after: String, $filter: IssueFilter) {
  issues(first: $first, after: $after, filter: $filter) {
    nodes { id identifier url title description priority createdAt
            creator { name } labels { nodes { name } } }
    pageInfo { hasNextPage endCursor }
  }
}`

// LinearPager walks the issues connection by cursor, 50 per page, to the end.
type LinearPager struct {
	// Endpoint is LinearEndpoint in production; a test sets an httptest URL. Nothing else does.
	Endpoint string
	Client   *http.Client // CheckRedirect refuses every redirect (NewLinearClient)
	// Sleep waits between rate-limited attempts; tests record it instead of sleeping.
	Sleep func(ctx context.Context, d time.Duration) error
}

const (
	linearPageSize   = 50
	linearMaxWaits   = 5
	linearMaxWait    = 60 * time.Second
	linearMaxIssues  = MaxRows // the same 5,000 ceiling as a CSV
)

// NewLinearClient is an http.Client with a 30-second timeout that follows no redirect, so a
// redirect can never carry the Authorization header to another host.
func NewLinearClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

// ErrLinearRefused is a 401 or 403 from Linear (a wrong or revoked key); ErrLinearUnavailable is
// a transport failure, a 5xx, a redirect, or a rate limit that outlasted linearMaxWaits.
var (
	ErrLinearRefused     = errors.New("workimport: linear refused the key")
	ErrLinearUnavailable = errors.New("workimport: linear is unavailable")
)

// All returns every issue as an Item, in the API's order, plus refusals for issues that cannot
// be items. apiKey is sent as the Authorization header value to Endpoint and nowhere else, and
// it is never part of an error, a log line or a return value.
//
// Per page: POST {query, variables: {first: 50, after: cursor, filter}}. On HTTP 429, wait
// Retry-After (seconds, capped at 60; absent means 5), at most five times. A GraphQL error whose
// extensions.code is "RATELIMITED" (how the public docs describe a limit) is treated the same,
// waiting 5 s. hasNextPage false, or more than 5,000 issues, ends the walk; past 5,000 the rest
// are one Refusal{At: "linear", Reason: "more than 5,000 issues; import by team"}.
func (p *LinearPager) All(ctx context.Context, apiKey, teamKey string) ([]Item, []Refusal, error)

// linearPriority maps the documented 0-4 to SW.6's words: 0 no priority, 1 urgent, 2 high,
// 3 normal (our medium), 4 low. Anything else is refused, never guessed.
func linearPriority(n int) (hubapi.Priority, bool) {
	switch n {
	case 0:
		return hubapi.PriorityNone, true
	case 1:
		return hubapi.PriorityUrgent, true
	case 2:
		return hubapi.PriorityHigh, true
	case 3:
		return hubapi.PriorityMedium, true
	case 4:
		return hubapi.PriorityLow, true
	}
	return "", false
}

// linearItem is one node → Item: ExternalID node.id, URL node.url, Title, Description (""
// when null), Priority, Labels (names), AuthorName creator.name ("" when null), CreatedAt
// node.createdAt verbatim. At is node.identifier. Then Validate.
```

The variable `filter` is `{"team": {"key": {"eq": teamKey}}}` when `teamKey` is set, else absent.
`teamKey` is checked as `^[A-Za-z0-9]{1,10}$` before it is sent, so a body cannot inject into the
filter object. The header is `Authorization: <apiKey>` for a personal API key, as the public docs
describe (an OAuth token would take `Bearer`; W1 takes personal keys only). All of this is written
from documentation and is **UNVERIFIED against the live API**.

```go
// workimport/csv.go
package workimport

// ParseCSV reads content with encoding/csv. A leading UTF-8 BOM is stripped. The first record
// is the header: names are trimmed and matched case-insensitively; "title" is required;
// "description", "priority", "labels" and "id" are optional; any other column is ignored and
// named once in a Refusal{At: "line 1", Reason: "column <name> is not imported"}. A header
// without "title" is an error (ErrNoTitleColumn), and nothing is imported.
//
// Per data row (line N = its 1-based record number, the header being line 1):
//   title        required; empty → Refusal "title is empty" (a row is never dropped silently)
//   description  verbatim after line endings are normalised
//   priority     "", none, urgent, high, medium, low (case-insensitive) → that Priority;
//                anything else → Refusal "priority <v> is not none, urgent, high, medium or low"
//   labels       split on ";", trimmed, empty parts dropped
//   id           when present and non-empty: ExternalID = the trimmed value
//                when absent or empty:       ExternalID = digestID(the normalised row)
// More than MaxRows data rows → ErrTooManyRows, and nothing is imported.
func ParseCSV(content string) ([]Item, []Refusal, error)

var (
	ErrNoTitleColumn = errors.New("workimport: the header row has no title column")
	ErrTooManyRows   = errors.New("workimport: more than 5,000 rows")
)

// normalise is the one text rule for the digest and the item alike: "\r\n" and a lone "\r"
// become "\n", and surrounding spaces and tabs are trimmed. encoding/csv already folds "\r\n"
// inside a quoted field; a lone "\r" it keeps, so this does it.
func normalise(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Trim(s, " \t")
}

// digestID is "csv-" + the first 32 hex of SHA-256 over the normalised title, description,
// priority word and labels (lower-cased, sorted, deduplicated), each preceded by its byte
// length and a colon so no two rows can collide by moving text between fields. The same file
// imported twice gives the same ids, so the second import creates nothing (task 15's index).
func digestID(title, description string, p hubapi.Priority, labels []string) string {
	h := sha256.New()
	ls := make([]string, 0, len(labels))
	for _, l := range labels {
		ls = append(ls, strings.ToLower(l))
	}
	slices.Sort(ls)
	for _, f := range append([]string{title, description, string(p)}, slices.Compact(ls)...) {
		fmt.Fprintf(h, "%d:%s", len(f), f)
	}
	return "csv-" + hex.EncodeToString(h.Sum(nil))[:32]
}
```

The csv `Reader` runs with `FieldsPerRecord = -1` (a short row is read, its missing columns are
empty), `LazyQuotes = false` and `ReuseRecord = false`. A stray quote is a `*csv.ParseError`;
`encoding/csv` does not promise to resume after one, so parsing **stops** there: the rows before
it are items, and one Refusal `{At: "line N", Reason: "the file cannot be read past this line"}`
names where it stopped. Two rows with the same digest describe the same issue; `Plan` refuses the
second as `duplicate of line M`.

```go
// workimport/batch.go, continued — the Importer.

// Importer is Deps.WorkImport: both sources, one flow. The key never outlives Import.
type Importer struct {
	Hub    Batcher
	Linear *LinearPager
	Log    *slog.Logger // counts and codes only
}

// Source is the decoded body's source part; exactly one of Linear and CSV is set.
type Source struct {
	Kind   string // "linear" | "csv"
	APIKey string // linear only; zeroed by the handler after Import returns
	Team   string
	CSV    string
}

// Preview parses and plans, and writes nothing: {ready, refused}.
func (im *Importer) Preview(ctx context.Context, src Source) (ready int, refused []Refusal, err error)

// Import parses, plans, sends in chunks and sums.
func (im *Importer) Import(ctx context.Context, projectID string, src Source) (Outcome, error)
```

The handler (`api/workimport.go`) decodes the body into a local struct with an 8 MiB
`MaxBytesReader`, builds `Source`, calls `Preview` or `Import`, and on return sets
`src.APIKey = ""` and the decoded body's key to `""` before anything else, so a later panic
dump or a heap read in a test cannot find it in a live variable. Its log lines are
`"work import", "source", …, "created", n, "unchanged", n, "refused", n`.

- [ ] **Step 1: Write the fixtures and the failing adapter tests**

Hand-write the fixtures: `linear-page-1.json` (two nodes, `hasNextPage: true`, `endCursor:
"c1"`; one node *Done* in a `state` field the query never asks for, put there to prove it is
ignored), `linear-page-2.json` (one node, `hasNextPage: false`), `linear-ratelimited.json` (a
GraphQL error with `extensions.code: "RATELIMITED"`); the CSVs as their names say. A test server
answers page 1 unless the request's `variables.after` is `"c1"`.

```go
func TestLinearPagerFollowsTheCursorToTheEnd(t *testing.T) {
	srv, seen := linearServer(t, "linear-page-1.json", "linear-page-2.json")
	p := &workimport.LinearPager{Endpoint: srv.URL, Client: workimport.NewLinearClient(), Sleep: noSleep}
	items, refused, err := p.All(ctx, "lin_api_TESTKEY", "")
	if err != nil || len(refused) != 0 {
		t.Fatalf("All: %v, refused %v", err, refused)
	}
	if len(items) != 3 || len(*seen) != 2 || (*seen)[1].After != "c1" || (*seen)[0].First != 50 {
		t.Fatalf("items %d, requests %+v; want 3 items over two pages of 50, the second after c1", len(items), *seen)
	}
}
```

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLinearPagerFollowsTheCursorToTheEnd` | three items over two requests; the second sends `after: "c1"`; `first: 50` | stopping at page one; the wrong page size |
| `TestLinearPagerHonoursRetryAfter` | the server answers 429 `Retry-After: 7` once, then page 1 → `Sleep` recorded exactly `7s` before the retry; `Retry-After: 900` → `60s`; six 429s → `ErrLinearUnavailable` | retrying at once; an unbounded wait or loop |
| `TestLinearRateLimitErrorWaits` | the `RATELIMITED` fixture once → a `5s` sleep, then success | a GraphQL-level limit read as a failure |
| `TestLinearKeyNeverLeaves` | the logger's buffer, every problem body, every file under a temp `statepath` root (walked after the import) and a second test server's received headers hold no `lin_api_TESTKEY`; a 302 from the endpoint to the second server → `ErrLinearUnavailable`, and the second server saw no request | a key in a log line, a file or a redirect |
| `TestLinearPriorityMapping` | 0→none, 1→urgent, 2→high, 3→medium, 4→low; 5 and −1 → refused naming the identifier | an off-by-one between `none` and `urgent` |
| `TestLinearStateIsNotCarried` | the *Done* node's `hubapi.ImportItem` re-encoded to JSON has no `state` key, and after `Import` on the fake the issue is in `triage`, `not_tied · imported` | an import that arrives `done` |
| `TestCSVRequiresTitle` | no `title` column → `ErrNoTitleColumn`; a row with an empty title → a Refusal at its line, the other rows kept | a silent drop |
| `TestCSVReimportIsIdempotent` | the same file imported twice on the fake → `created: 3, unchanged: 0`, then `created: 0, unchanged: 3` | duplicates on re-import |
| `TestCSVDigestIgnoresLineEndings` | `csv-crlf.csv` and the same text with LF, and a description holding a lone `\r` versus `\n`, give identical `ExternalID`s | a digest over raw bytes |
| `TestCSVStripsTheBOM` | `csv-bom.csv`'s first header is `title`, not `﻿title` | a BOM that hides the title column |
| `TestCSVDigestSeparatesFields` | title `ab`, description `c` versus title `a`, description `bc` → different ids | a digest over concatenated fields |
| `TestBatchChunksAtOneHundred` | 250 valid items → a recording `Batcher` sees three calls of 100, 100 and 50, in order | one unbounded request |
| `TestPlanRefusesWhatTheHubWould` | a 201-character title, a 16 KiB + 1 description, an `http://` URL, a duplicate id → four Refusals with their `At`s, and none of them in `send` | a chunk refused whole for one bad item |

Run: `go test ./control-plane/internal/workimport/... -race -count=1`
Expected: FAIL, `no Go files in …/workimport`.

- [ ] **Step 2: Implement `batch.go`, `linear.go` and `csv.go`**

Write the three files from the blocks above. Re-run step 1's command. Expected: `ok`.

- [ ] **Step 3: The routes, tests first**

```go
// api/workimport.go — registration, beside task 24's.
func (a *API) registerWorkImport(mux *routes) {
	mux.Handle("POST /v1/work/import/preview", http.HandlerFunc(a.handleWorkImportPreview))
	mux.Handle("POST /v1/work/import", http.HandlerFunc(a.handleWorkImport))
}

// importMaxBody replaces httpx.MaxBodyBytes (1 MiB) for these two routes only.
const importMaxBody = 8 << 20

type workImportBody struct {
	ProjectID string `json:"projectId"`
	Source    string `json:"source"`
	Linear    *struct {
		APIKey  string `json:"apiKey"`
		TeamKey string `json:"teamKey"`
	} `json:"linear"`
	CSV *struct {
		Content string `json:"content"`
	} `json:"csv"`
}

// in type Deps, after task 24's WorkStart:
//	WorkImport *workimport.Importer // nil when Work is nil: both routes answer hub_not_configured
// in New, after a.registerWorkStart(guarded):
//	a.registerWorkImport(guarded)
```

`control-plane/internal/api/workimport_test.go`, on the phase 2 API rig with
`Deps.WorkImport = &workimport.Importer{Hub: hubclient.NewFake().As(dev), Linear: <test pager>}`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWorkImportWire` | a CSV body → 200 with exactly `created`, `unchanged`, `refused`, `unmatchedLabels`; `refused` and `unmatchedLabels` are `[]` when empty | a `null` list the dialog indexes; a percentage or a ratio in the answer |
| `TestWorkImportPreviewWritesNothing` | preview of a 3-row CSV → `{"ready":3,"created":null,"unchanged":null,…}`; the fake's `Calls()` holds no `ImportBatch` | a preview that imports |
| `TestWorkImportBodyShape` | `source: "jira"`, both sub-objects, the wrong one, an unknown field → 400 each naming it | a lenient decoder |
| `TestWorkImportTakesALargeCSV` | a 1.5 MiB, 5,000-row CSV → 200; 5,001 rows → 400 naming `limit`; a 9 MiB body → 413 | the 1 MiB `DecodeJSON` cap cutting a legal file; no ceiling at all |
| `TestWorkImportLinearRefusedKey` | the test pager's server answers 401 → 422 `import_source_refused`, `details.status == 401`, and no `apiKey` in the body | a Linear failure reported as the Hub's, or with the key |
| `TestWorkImportUnmatchedLabels` | a row labelled `Infra`, which the fake's team lacks → `unmatchedLabels: ["Infra"]` and no label created (task 15's rule, re-proved through the adapter) | a member creating labels through import |
| `TestWorkImportWithoutHub` | `Deps.Work` and `WorkImport` nil → 503 `hub_not_configured` | a nil dereference with no Hub (`OD-20`) |

And the two patterns join task 24's `TestWorkStartRoutesArePosts` list in `routes_audit_test.go`
(rename it `TestWorkRoutesArePosts` if task 24 has landed; one test, five patterns).

Run: `go test ./control-plane/internal/api/... -race -count=1 -run 'WorkImport|WorkRoutes'`
Expected: FAIL, `undefined: workimport`. Write `api/workimport.go`, the `Deps` field, the
registration and the `cmd/api` wiring (`&workimport.Importer{Hub: deps.Work, Linear:
&workimport.LinearPager{Endpoint: workimport.LinearEndpoint, Client: workimport.NewLinearClient(),
Sleep: sleepCtx}, Log: log}`); re-run. Expected: `ok`.

- [ ] **Step 4: Acceptance, then commit**

```bash
~/.claude/tools/claudezsh/claudezsh 'go test ./control-plane/internal/workimport/... ./control-plane/internal/api/... ./control-plane/cmd/api/... -race -count=1'
```

Expected: one `ok` per package, no `FAIL`. `git diff -- go.work control-plane/go.mod` is empty (no
new dependency: `encoding/csv`, `crypto/sha256`, `net/http` are the standard library).
Commit with `git commit -F <msgfile> -- <the paths above>`.

#### What this task does not claim

- **Not that `api.linear.app` is reachable** from the corporate network, nor that the query's field
  names match the live schema: both are **UNVERIFIED**, written from public documentation. The CSV
  path needs no network and is the fallback.
- **Not the import dialog** (task 69), and not Jira, Azure Boards or GitHub Issues (W5, tasks
  150-153).
- **Not comments, attachments, relations, assignees or states.** None is imported in W1; **L-220**
  stays PARTIAL, as task 15 says.
- **Not a preview of *created* versus *unchanged*** (no dry run in SW.6; gap raised).
- **Not the Linear identifier as a field** (no `externalKey` in SW.6; gap raised). It is in the
  item's `url`.
- **Not OAuth.** W1 takes a personal API key per request; an OAuth app for Linear is not built.
- Design: 05 §5.3 (row 5 and the demo's first step), §2.3 point 4, §3.6. Parity: closes the W1
  part of **L-220** (Linear and CSV importers), and with task 15 **L-9**'s *intake always lands in
  `triage`*.