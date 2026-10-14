### Task 178: warehouse and spreadsheet sync through the public API and webhooks; the first named connector (L-154)

Stream **WL**, about 2.5 hours (an estimate, not a measurement). Depends on task **156** (outbound
webhooks, seam **SW.22**), task **158** (the public API's docs and the generated Go client under
`sdk/go/`), task **174**, and through them on task 159 (approved Entra apps) and task 160 (API keys
`pak_…`, the `RateLimit-*` headers). 05 L-154 (`05-design.md:311`, verified 2026-09-26):
*"Warehouse and spreadsheet sync · MATCH · Through the API and webhooks; named connectors later ·
later"*. Hub anchors are the `feat/hub` worktree at `54b5e746` (`hub-contract:N`). Parts 156, 158,
159 and 160 are **not written** on 2026-09-26: their names below are their scopes'
(`tasks-141-160.md`) and **UNVERIFIED** until they land.

This task adds **no Hub route**. It ships a reference exporter that a data team runs against the
public API with its own key, and a Power Automate custom connector: the Microsoft-native path
05's L-217 note names (Zapier, Google Sheets and Airbyte stay SKIP there).

**Decisions made here, so nobody re-derives them:**

1. **The feed is the source; the webhook only says "read sooner".** `follow` reads `GET
   /v1/changes?after=<cursor>` (`hub-contract:1091-1105`: gapless, in commit order, `next` and
   `more`). A webhook delivery whose `Panacea-Signature` verifies (SW.22: `t=<unix>,v1=<hex
   HMAC-SHA256(secret, t + "." + raw body)>`, a `t` older than 5 minutes refused) wakes the loop at
   once; its body **never** feeds a row. The payload is metadata only (05 L-251), and a nudge that
   fails to arrive costs only latency, because the next poll reads the same events.
2. **Text is fetched by key, with the key's own rights.** For each event the exporter re-reads the
   record it names (an issue head and revision, a comment, a feature, a cycle, a release, a
   request) through the generated client, as the key's person or team. A record the key cannot read
   is 404 and is **absent** from the export; nothing is exported with an admin's reach.
3. **One JSONL file per kind, upsert by key.** `issues.jsonl`, `revisions.jsonl`,
   `comments.jsonl`, `features.jsonl`, `cycles.jsonl`, `releases.jsonl`, `requests.jsonl`. Each
   line is `{"id", "rev", "seq", "erased", "record"}`: `id` the Hub id, `rev` the record's revision
   (or `0` for kinds with none), `seq` the feed event that caused the line, `record` the Hub's own
   JSON for it. A loader upserts on `(id, rev)`; a later line for the same key wins.
4. **An erase is a tombstone.** `issue.erased`, `comment.erased` and `brief.erased`
   (`task-14.md:34-36`) and task 146's request erase write a line with `erased: true` and `record:
   null` for every exported revision of that id, so a loader that upserts drops the text when the
   Hub does. The exporter keeps a local index `index.jsonl` of `(kind, id, rev)` it has written so
   it can tombstone each; the index holds no text.
5. **The cursor is saved after the write.** `follow` appends the lines, `fsync`s each file, then
   writes the new cursor to `--cursor` through a temp file and rename. A crash between the two
   repeats a batch (the upsert makes that harmless); it never skips one.
6. **Credentials from the environment only.** `PANACEA_HUB_URL` and `PANACEA_API_KEY` (task 160's
   `pak_…`); the key is never written to disk, a flag, or a log line. `PANACEA_WEBHOOK_SECRET` is
   read only when `--listen` is set.
7. **The connector names only routes that exist.** Every action's path and method is checked
   against `docs/api/openapi.json` (task 158's generated document), so a connector cannot drift
   from the contract. The trigger subscribes through task 156's create route and its payload schema
   is SW.22's metadata-only payload, with no text field.

**Files:**
- `control-plane/cmd/panacea-export/main.go` — flags and the two commands (new)
- `control-plane/cmd/panacea-export/export.go` — snapshot and follow, over `sdk/go` (new)
- `control-plane/cmd/panacea-export/sink.go` — JSONL files, the index, tombstones, the cursor (new)
- `control-plane/cmd/panacea-export/nudge.go` — the optional webhook listener and its check (new)
- `control-plane/cmd/panacea-export/{export,sink,nudge}_test.go` — the tests (new)
- `control-plane/cmd/panacea-export/connector_test.go` — the connector's drift and schema checks,
  reading the files below by relative path, so they run under the control plane's `go test` (new)
- `integrations/power-automate/apiDefinition.swagger.json` — the connector's actions and trigger (new)
- `integrations/power-automate/apiProperties.json` — auth and connection parameters (new)
- `integrations/power-automate/README.md` — how an org imports it; what it does not do (new)

**The module edge.** `sdk/go` is its own module in `go.work` (task 158). In workspace mode a
package of one workspace module can import another's, but `control-plane/go.mod` may still need a
`require` of the SDK module for `go mod tidy` and for a build outside the workspace; whether it
does is **UNVERIFIED** until task 158's `sdk/go/go.mod` exists. If it does, the `require` line is
this task's one edit to `control-plane/go.mod`, named in its commit, with no other change there.

#### Interfaces — complete: `sink.go`

```go
package main

// Line is one exported row. A loader upserts on (ID, Rev); a later line wins.
type Line struct {
	ID     string          `json:"id"`
	Rev    int             `json:"rev"`
	Seq    int64           `json:"seq"`
	Erased bool            `json:"erased"`
	Record json.RawMessage `json:"record"` // null when Erased
}

// Kinds are the files the exporter writes, one per kind.
var Kinds = []string{"issues", "revisions", "comments", "features", "cycles", "releases", "requests"}

// Sink appends JSONL lines under Dir and keeps index.jsonl of every (kind, id, rev)
// written, so an erase can tombstone each revision. The index holds no text.
type Sink struct {
	Dir   string
	files map[string]*os.File
	index map[string]map[string][]int // kind → id → revs written
}

func OpenSink(dir string) (*Sink, error)            // creates dir; opens each file O_APPEND; loads index.jsonl
func (s *Sink) Write(kind string, l Line) error      // appends one line and its index entry
func (s *Sink) Tombstone(kind, id string, seq int64) error // one erased line per indexed rev of id; no index entry: none written
func (s *Sink) Sync() error                          // fsync every file and the index
func (s *Sink) Close() error

// Cursor is the feed position, stored as the Hub's "next" string (hub-contract:1094).
func LoadCursor(path string) (string, error) // "" when the file is absent: start from a snapshot
// SaveCursor writes path+".tmp", fsyncs it, and renames it over path. Called only
// after Sink.Sync has returned nil for the batch the cursor follows (decision 5).
func SaveCursor(path, next string) error
```

A `Tombstone` for an id the exporter never wrote writes nothing: the warehouse never held it.

#### Interfaces — complete: `nudge.go` (the webhook listener, only with `--listen`)

```go
package main

// VerifyNudge checks SW.22's Panacea-Signature over the raw body. It answers
// false for a missing or malformed header, a t more than 5 minutes from now in
// either direction, or a v1 that does not match in constant time.
func VerifyNudge(header string, body []byte, secret []byte, now time.Time) bool {
	var t int64
	var v1 string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return false
		}
		switch k {
		case "t":
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return false
			}
			t = n
		case "v1":
			v1 = v
		}
	}
	if t == 0 || v1 == "" {
		return false
	}
	if d := now.Sub(time.Unix(t, 0)); d > 5*time.Minute || d < -5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(strconv.FormatInt(t, 10) + "."))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(v1))
}

// Listen serves POST on addr; a verified body sends one value on wake (non-blocking,
// capacity 1) and answers 204; anything else answers 401 and sends nothing. The body
// is read (at most 64 KiB) and discarded: it never becomes a row (decision 1).
func Listen(addr string, secret []byte, wake chan<- struct{}) (*http.Server, error)
```

The webhook's URL must be public `https` (task 156 refuses private, loopback and link-local
addresses at every attempt), so `--listen` only helps where an org exposes the exporter behind its
own TLS endpoint. Without it, `follow` polls every `--every` (default 60 s) and loses nothing.

#### Interfaces — complete: `main.go` and `export.go`

```text
panacea-export snapshot --out <dir> [--team tm-…] [--project pj-…]
panacea-export follow   --out <dir> --cursor <file> [--every 60s] [--listen :8443]
  env: PANACEA_HUB_URL, PANACEA_API_KEY; with --listen also PANACEA_WEBHOOK_SECRET
  exit 0 on SIGINT after the current batch is synced; 2 on a bad flag or a missing env; 1 otherwise
```

```go
package main

// Client is the part of task 158's generated sdk/go client the exporter uses.
// Its method names are the generator's (oapi-codegen); these are the routes.
type Client interface {
	Changes(ctx context.Context, after string, limit int) (events []Event, next string, more bool, err error) // GET /v1/changes
	Get(ctx context.Context, kind, id string) (rec json.RawMessage, rev int, found bool, err error)            // the record's own GET; found false on 404
	List(ctx context.Context, kind, scope string, after string) (recs []json.RawMessage, next string, err error) // snapshot pages, ?after=&limit=200
}

// Event is the /v1/changes event (hub-contract:1094-1096); Data is metadata only.
type Event struct {
	Seq     int64             `json:"seq"`
	Kind    string            `json:"kind"`
	Subject string            `json:"subject"`
	Data    map[string]string `json:"data"`
}

// KindOf maps an event kind to the file it touches and whether it erases:
// "issue.created" → ("issues", false); "issue.revised" → ("revisions", false);
// "issue.erased" → ("issues", true); an unknown kind → ("", false), skipped.
func KindOf(eventKind string) (file string, erase bool)

// Snapshot pages every listed kind the key can read and writes one line per record,
// then sets the cursor to the feed's head read BEFORE the first page, so an event
// that lands during the snapshot is read again by follow (never missed).
func Snapshot(ctx context.Context, c Client, s *Sink, cursorPath string) error

// Follow loops: Changes(after = the stored cursor); for each event, erase → Tombstone,
// else Get and Write (a 404 writes nothing); Sync; SaveCursor(next). It waits
// --every, or less when wake fires, and asks again at once while more is true.
func Follow(ctx context.Context, c Client, s *Sink, cursorPath string, every time.Duration, wake <-chan struct{}) error
```

`Get` reads through the key's rights (decision 2). A 429 honours `Retry-After` and the
`RateLimit-*` headers (task 160); the exporter never retries tighter than the Hub asks.

#### Interfaces — complete: `integrations/power-automate/apiDefinition.swagger.json`

Power Automate custom connectors take an OpenAPI 2.0 definition with Microsoft's `x-ms-*`
extensions. The paths are the Hub's own; the connector adds nothing between the flow and the Hub.

```jsonc
{
  "swagger": "2.0",
  "info": { "title": "PanaceaApp Work", "version": "1.0",
    "description": "Read issues, releases and requests; trigger on Work events. Metadata only in the trigger." },
  "host": "hub.example.invalid",   // replaced per org at import: its Hub host
  "basePath": "/", "schemes": ["https"],
  "consumes": ["application/json"], "produces": ["application/json"],
  "securityDefinitions": { "entra": { "type": "oauth2", "flow": "accessCode",
    "authorizationUrl": "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
    "tokenUrl": "https://login.microsoftonline.com/common/oauth2/v2.0/token",
    "scopes": { "api://panacea-hub/Hub.Access": "Act as you on the Hub" } } },
  "security": [ { "entra": ["api://panacea-hub/Hub.Access"] } ],
  "paths": {
    "/v1/projects/{projectId}/issues": { "get": { "operationId": "ListIssues", "summary": "List issues",
      "parameters": [ { "name": "projectId", "in": "path", "required": true, "type": "string" },
        { "name": "after", "in": "query", "type": "string" }, { "name": "limit", "in": "query", "type": "integer" } ],
      "responses": { "200": { "description": "A page of issue heads" } } } },
    "/v1/issues/{issueId}": { "get": { "operationId": "GetIssue", "summary": "Get an issue",
      "parameters": [ { "name": "issueId", "in": "path", "required": true, "type": "string" } ],
      "responses": { "200": { "description": "The issue head" } } } },
    "/v1/projects/{projectId}/releases": { "get": { "operationId": "ListReleases", "summary": "List releases",
      "parameters": [ { "name": "projectId", "in": "path", "required": true, "type": "string" } ],
      "responses": { "200": { "description": "A page of releases" } } } },
    "/v1/teams/{teamId}/requests": { "get": { "operationId": "ListRequests", "summary": "List requests",
      "parameters": [ { "name": "teamId", "in": "path", "required": true, "type": "string" } ],
      "responses": { "200": { "description": "A page of requests" } } } },
    "/v1/teams/{teamId}/webhooks": { "post": { "operationId": "OnWorkEvent", "summary": "When a Work event happens",
      "x-ms-trigger": "single",
      "parameters": [ { "name": "teamId", "in": "path", "required": true, "type": "string" },
        { "name": "body", "in": "body", "required": true, "schema": { "type": "object", "properties": {
          "url": { "type": "string", "x-ms-notification-url": true, "x-ms-visibility": "internal" },
          "kinds": { "type": "array", "items": { "type": "string" } },
          "projectId": { "type": "string" } }, "required": ["url", "kinds"] } } ],
      "responses": { "201": { "description": "Subscribed",
        "headers": { "Location": { "type": "string" } } } },
      "x-ms-notification-content": { "description": "A Work event, metadata only (SW.22)",
        "schema": { "$ref": "#/definitions/Delivery" } } } }
  },
  "definitions": {
    "Delivery": { "type": "object", "properties": {
      "delivery": { "type": "string" },
      "event": { "type": "object", "properties": {
        "seq": { "type": "integer" }, "kind": { "type": "string" }, "at": { "type": "string" },
        "teamId": { "type": "string" }, "projectId": { "type": "string" }, "subject": { "type": "string" },
        "actor": { "type": "object", "properties": { "kind": { "type": "string" }, "userId": { "type": "string" },
          "ownerId": { "type": "string" }, "agentId": { "type": "string" }, "appId": { "type": "string" } } },
        "data": { "type": "object", "additionalProperties": { "type": "string" } } } } } }
  }
}
```

The `// replaced per org` comment is for this plan only; the shipped file is plain JSON with the
placeholder host. The scope name `api://panacea-hub/Hub.Access` stands for the Hub's `Hub.Access`
scope (`hub-contract.md:36`, as task 158's scope cites it); its application ID URI is each org's
and is set at import (**UNVERIFIED**: the Hub's app registration is Phase H's). Unsubscribing is
`POST /v1/webhooks/{webhookId}/disable` (task 156); a Power Automate trigger deletes through the
`Location` it was given, so the README records that the connector needs task 156 to answer
`Location: /v1/webhooks/{webhookId}` and to accept `DELETE` there, or the flow's clean-up fails
(gap 2).

#### Interfaces — complete: `integrations/power-automate/apiProperties.json`

```json
{
  "properties": {
    "connectionParameters": {
      "token": {
        "type": "oauthSetting",
        "oAuthSettings": {
          "identityProvider": "aad",
          "clientId": "ORG_CLIENT_ID",
          "scopes": ["api://panacea-hub/Hub.Access"],
          "redirectMode": "Global",
          "redirectUrl": "https://global.consent.azure-apim.net/redirect",
          "properties": { "IsFirstParty": "False", "AzureActiveDirectoryResourceId": "api://panacea-hub" },
          "customParameters": {
            "loginUri": { "value": "https://login.microsoftonline.com" },
            "tenantId": { "value": "ORG_TENANT_ID" },
            "resourceUri": { "value": "api://panacea-hub" }
          }
        }
      }
    },
    "iconBrandColor": "#2B3A40",
    "capabilities": [],
    "publisher": "PanaceaApp",
    "stackOwner": "PanaceaApp"
  }
}
```

`ORG_CLIENT_ID` and `ORG_TENANT_ID` are placeholders the README tells an org's admin to replace
with the app registration task 159 approved; the file holds no secret (the client secret is
entered in the Power Automate portal, never committed). The key names follow Microsoft's
custom-connector `apiProperties.json` schema as the writer knows it; it could not be read from
Microsoft's documentation here (no network), so step 1 checks every key against the current
schema with `paconn validate` and corrects this block (**UNVERIFIED** until then).

#### Tests to write first

All run under `go test ./control-plane/cmd/panacea-export/ -race -count=1`, against a fake `Client`
(an in-memory feed and record store) and a temp directory; nothing reaches a network.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestFollowResumesWithNoDuplicateAndNoGap` | a feed of 30 events; `Follow` is stopped after batch 1 by a sink whose `Sync` fails; a second run from the stored cursor writes events 11-30 once each and every `seq` 1-30 appears exactly once across the files | the cursor saved before the write |
| `TestACrashAfterSyncRepeatsABatchHarmlessly` | the cursor write fails after `Sync`; the next run rewrites batch 1; a loader that upserts on `(id, rev)` ends with the same rows as a clean run | a non-idempotent line shape |
| `TestAnEraseTombstonesEveryRevision` | an issue with revisions 1-3 exported; `issue.erased` → three `revisions` lines and one `issues` line with `erased: true`, `record: null`; the canary title `CANARY-178` appears in no line written after the erase | stale customer text left in the warehouse |
| `TestAnUnreadableRecordIsAbsent` | the fake `Get` answers 404 for one issue the key's person cannot read → no line for it, no error, the cursor moves | an admin-wide export, or a stuck exporter |
| `TestTheWebhookBodyNeverBecomesARow` | a verified nudge whose body carries a `title` field → no line holds it; the line comes from `Get` | a payload fed into the warehouse |
| `TestAnUnsignedOrStaleNudgeIsIgnored` | no header, a wrong `v1`, a `t` six minutes old, and a valid one: only the last wakes the loop; the first three answer 401 | any POST triggering a read |
| `TestTheSnapshotReadsTheHeadFirst` | an event lands between the head read and the last page → `follow` from the snapshot's cursor reads it | a snapshot that skips what landed during it |
| `TestTheKeyIsNeverWritten` | after a run, no file under `--out`, the cursor file or captured logs contains the `PANACEA_API_KEY` value | a key in a log line |
| `TestEveryConnectorPathExistsInTheContract` | each `paths` entry and method in `apiDefinition.swagger.json` is in task 158's `docs/api/openapi.json` | drift from the contract |
| `TestTheTriggerPayloadIsMetadataOnly` | the `Delivery` schema's property names equal SW.22's payload keys; no `title`, `description`, `body`, `text` or `quote` at any depth | text fields in the schema |

#### Steps

- [ ] **Step 1: confirm the sync base.** Tasks 156, 158, 159 and 160 committed; read their parts;
  map `Client` onto the generated `sdk/go` methods; check the payload keys and the signature header
  against task 156's code; run `paconn validate` on the two connector files if the tool is
  available, else mark the connector's schema **UNVERIFIED** in the README.
- [ ] **Step 2: the tests first**, with the fake `Client`; every test fails to compile.
- [ ] **Step 3:** `sink.go`, then `nudge.go`, then `export.go` and `main.go`; run until green.
- [ ] **Step 4:** the connector files and `connector_test.go`; the README (import steps, the
  placeholders, what the connector does not do).
- [ ] **Step 5: run the acceptance commands**; `git diff -- go.work control-plane/go.mod` shows at
  most the one `require` line of *The module edge*; commit by path, each message ending with the
  `Co-Authored-By` line.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/cmd/panacea-export/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"
# expect: 10
go vet ./control-plane/cmd/panacea-export/
# expect: no output
go build -o /dev/null ./control-plane/cmd/panacea-export/ && echo built
# expect: built
PANACEA_HUB_URL= PANACEA_API_KEY= go run ./control-plane/cmd/panacea-export follow --out x --cursor y; echo "exit $?"
# expect: a line naming PANACEA_HUB_URL, then "exit 2"
grep -cE '"(title|description|body|text|quote)"' integrations/power-automate/apiDefinition.swagger.json
# expect: 0
```

#### What this task does NOT claim

- A certified or published connector, or one imported into any Power Automate environment.
- A run against a real warehouse, Fabric, Synapse, Excel workbook or Hub.
- That customer text may leave the product: that is each org's call, made by issuing the key. The
  exporter moves what the key can read and nothing more.
- Zapier, Google Sheets or Airbyte connectors (05 L-217 keeps them SKIP).
- That a loader exists: the files are shaped for an upsert; writing one is the org's.

**Parity.** Closes **L-154** (`05-design.md:311`, MATCH, slice `later`), with task 156's webhooks
(SW.22) and task 158's API; task 187 records the command and output.

#### Gaps found while writing this part

1. **The skeleton's dependsOn for 178 omits 159 and 160**, which it uses (approved apps for the
   connector's sign-in, keys for the exporter); both are before 174, so the order holds.
2. **Task 156 names no delete route and no `Location` header** for a created webhook; a Power
   Automate trigger unsubscribes through `Location`. Either 156 adds both, or the connector's flow
   leaves webhooks behind until an admin disables them.
3. **Webhooks refuse private addresses** (task 156), so the exporter's `--listen` nudge only works
   behind an org's public TLS endpoint; polling is the default and loses nothing.
4. **The `control-plane` to `sdk/go` module edge** may need a `require` line (*The module edge*);
   task 158's part should say how outside-the-SDK tools import it.
5. **The file map has no rows** for `cmd/panacea-export/` or `integrations/power-automate/`.

