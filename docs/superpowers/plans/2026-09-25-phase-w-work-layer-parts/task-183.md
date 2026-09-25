### Task 183: Alert intake into `triage`: Azure Monitor first, then Sentry (L-209)

Stream **WL**, Hub, about 2.5 hours (an estimate, not a measurement). Depends on task 146
(requests `rq-`: the requester, the verbatim quote, accept or link in `triage`, erase) and task
174; task 143's SW.21 wire and task 147's intake rules are under it. Neither part is written at
this writing, so every name taken from them is from their **scope paragraphs**
(`tasks-141-160.md:139-149`, `:278-300`, `:322-335`) and is **UNVERIFIED** until the part exists;
where a part names a thing differently, the part wins and this one follows it.

05 row L-209: *"Azure Monitor first (OD-18), Sentry next"*. **An alert is a request from an
outside system, not a new kind of issue.** It enters through task 146's create, passes task 147's
`Route` (first match wins, so an admin can route `Sev0` to the on-call project), and waits in a
team's `triage` for a person to accept it into an issue or link it to one (05 §3.5 steps 1-3).
Nothing about an alert moves an issue: not the fire, not the re-fire, not the resolve.

**What a re-fire is.** An alert rule fires the same alert again and again. `external_id` is
Azure Monitor's `alertId` or Sentry's issue id, and `alert_links` is unique on
`(source_id, external_id)`, so a re-fire bumps `fired_count` on the **same** request. The count
is a measured fact, and task 170's Triage row shows it as *fired 7 times*, never a rate or a
forecast (`OD-27`).

**What a resolve is.** `Resolved` sets `resolved_at` on the link and changes **no issue state**
(SW.3): the issue moves only by its own facts. The Triage row and the issue's requester list say
*resolved in Azure Monitor at 14:02*: a fact about the source, shown as the source's. A later
`Fired` for the same id clears `resolved_at` and bumps the count.

**The quote, and why it includes the severity.** The quote is the source's words, `text·H` with
the source as requester (`kind: "alert"`), never an agent's; an agent reads it only as data (05
§3.5). It is built as one string, `"<severity> · <rule name>\n<description>"`, and the same
string is `Route`'s `Incoming.text`. So a rule an admin writes, `textContains: "Sev0"`, matches
exactly what the reader later sees. Past **4 KiB** the quote is cut at a UTF-8 boundary and the
link's `quoteTruncated` is set; the quote itself is never given a marker it did not send, so it
stays verbatim as far as it goes.

**Two corrections to the scope, made here so nobody re-derives them.**
1. **A source needs configuration the scope's type did not carry.** Azure Monitor's secure
   webhook presents an Entra bearer whose audience is an app the customer registers; the Hub must
   know that audience per source. Sentry signs with its integration's client secret, which Sentry
   generates and the admin copies in. So `AlertSource` gains `audience` (Azure Monitor) and
   `secretRef` (Sentry; task 144's Key Vault `secret_ref` pattern, a proposed Hub-contract
   addition since `hub-contract.md:2083-2085` gives the Hub no Key Vault in v1). The Sentry
   secret is write-only: accepted on create, never returned.
2. **"Marked *truncated*" is a flag, not text in the quote** (above), so `AlertLink` gains
   `quoteTruncated`.

**The `als-` prefix** is free against 05 §3.6's list (`05-design.md:816-821`, verified in this
worktree at `0901bab9`) and against task 143's eleven W5 additions (`tasks-141-160.md:131-138`: `ir- fm- tc- xl- hl- wh- wd-
oa- ak- mc- lr-`); the part
re-checks both at sync time, as the scope asks.

**Files** (the File map has no row; proposals in its `hub/` layout):
- `control-plane/internal/hub/alerts/azuremonitor.go` (+ test) — `ParseAzureMonitor` (new)
- `control-plane/internal/hub/alerts/sentry.go` (+ test) — `VerifySentry`, `ParseSentry` (new)
- `control-plane/internal/hub/alerts/intake.go` (+ test) — `Alert`, `QuoteOf`, `Ingest` (new)
- `control-plane/internal/hub/alerts/testdata/{azure-fired,azure-resolved,azure-unknown-schema,sentry-issue-created,sentry-issue-resolved}.json`
  — golden payloads written from each vendor's public documentation, marked **UNVERIFIED** in
  a sibling `README` (new)
- `control-plane/internal/hub/server/work_alerts.go` (+ `_integration_test.go`) — the ingest
  route and the source CRUD (new)
- `control-plane/internal/hub/store/work_alerts.go` — sources, links, the bump (new)
- `control-plane/internal/hub/migrate/sql/NNNN_work_wl_alerts.sql` — the next free number at sync
  time (new)
- `control-plane/internal/hub/hubapi/work_intake.go` ⚑ (task 143, SW.21) — `AlertSource`,
  `AlertLink`, `CreateAlertSourceRequest`, `ChannelAlert`, `RequesterKindAlert` (modified)
- `control-plane/internal/hub/hubclient/work.go` ⚑ — `CreateAlertSource`, `AlertSources`,
  `DeleteAlertSource` on `WorkIntake` (modified)
- `control-plane/internal/hub/hubclient/fake_work*.go` ⚑, `conformance/work*.go` ⚑ — the fake's
  rules and the conformance rows (modified)
- `control-plane/internal/hub/server/matrix_integration_test.go` ⚑ — the routes' rows and the
  alert principal's 403 (modified)

#### Interfaces — complete

```go
// hubapi/work_intake.go (task 143, SW.21) — additions. The two enum values are additive; the
// desktop's Triage row (task 170) renders an unknown channel by its word.
const (
	ChannelAlert       = "alert" // Request.Channel
	RequesterKindAlert = "alert" // Request.Requester.Kind: the source; displayName is its name
	PrefixAlertSource  = "als"   // als-…, checked free at sync time (05 §3.6, task 143's list)

	AlertKindAzureMonitor = "azure_monitor"
	AlertKindSentry       = "sentry"

	MaxAlertQuoteBytes = 4 << 10   // 4 KiB; past it the quote is cut and QuoteTruncated set
	MaxAlertBodyBytes  = 256 << 10 // a larger inbound body is 413 and nothing is read
)

type AlertSource struct {
	ID        string `json:"id"`     // als-…
	TeamID    string `json:"teamId"` // tm-…; the team whose triage receives it
	Kind      string `json:"kind"`   // AlertKindAzureMonitor | AlertKindSentry
	Name      string `json:"name"`   // 1-80; the requester's displayName, e.g. "api-prod monitor"
	Audience  string `json:"audience"` // azure_monitor: the Entra app id URI its bearer must name; "" for sentry
	CreatedBy User   `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
	// No secret field: a Sentry source's secret is held by reference (secret_ref) and never
	// returned. A reflection test over AlertSource fails on any field named like secret.
}

type CreateAlertSourceRequest struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Audience     string `json:"audience,omitempty"`     // required for azure_monitor, refused for sentry
	SentrySecret string `json:"sentrySecret,omitempty"` // required for sentry, refused for azure_monitor; write-only
}

type AlertLink struct {
	SourceID       string  `json:"sourceId"`
	ExternalID     string  `json:"externalId"` // Azure Monitor alertId | Sentry issue id
	RequestID      string  `json:"requestId"`  // rq-…
	FiredCount     int     `json:"firedCount"` // measured: one per Fired received
	LastFiredAt    string  `json:"lastFiredAt"`
	ResolvedAt     *string `json:"resolvedAt"` // the source's resolve time; null while firing
	QuoteTruncated bool    `json:"quoteTruncated"`
}

// Request (task 143) gains one read-only field, filled when Channel is "alert":
//	Alert *AlertLink `json:"alert,omitempty"`
```

```go
// hubclient/work.go — WorkIntake gains three methods; the fake and the HTTP client implement both.
	CreateAlertSource(ctx context.Context, teamID string, req hubapi.CreateAlertSourceRequest) (hubapi.AlertSource, error)
	AlertSources(ctx context.Context, teamID string) ([]hubapi.AlertSource, error) // [] never nil
	DeleteAlertSource(ctx context.Context, teamID, sourceID string) error
```

```go
// alerts/intake.go — pure, apart from Ingest's store call.
package alerts

// Alert is either vendor's payload, normalised. Nothing else in the payload is kept.
type Alert struct {
	ExternalID string    // the dedup key
	Condition  string    // "fired" | "resolved"
	Severity   string    // "Sev0".."Sev4" (Azure Monitor) | Sentry's level ("fatal", "error", …)
	RuleName   string    // Azure Monitor essentials.alertRule | Sentry issue title
	Description string   // essentials.description | Sentry issue culprit
	At         time.Time // firedDateTime / resolvedDateTime | the event's timestamp
}

// QuoteOf builds "<severity> · <rule name>\n<description>", then cuts it to MaxAlertQuoteBytes
// at the last UTF-8 boundary at or below the limit. It is also Route's Incoming.text.
func QuoteOf(a Alert) (quote string, truncated bool)

// Ingest applies one alert for one source, in one transaction:
//   fired, no link          → task 146's create (channel alert, requester {kind alert,
//                             displayName source.Name}, quote), routed by task 147's Route with
//                             Incoming{channel: "alert", text: quote}; then the link, count 1
//   fired, link exists      → fired_count+1, last_fired_at, resolved_at = NULL; no new request,
//                             and the request's quote is NOT rewritten (the first fire's words stay)
//   resolved, link exists   → resolved_at; nothing else
//   resolved, no link       → nothing written; answered 202 (a resolve for an alert we never saw)
// It never calls a state move, an assignment or a priority change, and no event it appends is an
// issue event: request.created (task 146) on the first fire, request.alert_fired {firedCount} and
// request.alert_resolved {} after, metadata only, drafts that task 13 makes constants (SW.5).
func Ingest(ctx context.Context, tx pgx.Tx, src hubapi.AlertSource, a Alert, now time.Time) (hubapi.AlertLink, error)
```

```go
// alerts/azuremonitor.go — the common alert schema only, from Azure Monitor's public
// documentation (UNVERIFIED here). Any other schemaId is ErrUnknownSchema: no guessing parser.
const AzureCommonSchema = "azureMonitorCommonAlertSchema"

// ParseAzureMonitor reads schemaId and data.essentials.{alertId, alertRule, severity,
// monitorCondition ("Fired" | "Resolved"), firedDateTime, resolvedDateTime, description}.
// A missing alertId, or a monitorCondition outside the two, is ErrUnknownSchema.
func ParseAzureMonitor(body []byte) (Alert, error)

// alerts/sentry.go — from Sentry's integration-platform documentation (UNVERIFIED here).
// VerifySentry checks Sentry-Hook-Signature == hex(HMAC-SHA256(secret, raw body)) with
// hmac.Equal, before any decode.
func VerifySentry(secret []byte, signature string, body []byte) error // ErrBadSignature

// ParseSentry accepts Sentry-Hook-Resource "issue" with action "created" (fired) or "resolved";
// ExternalID data.issue.id, RuleName data.issue.title, Description data.issue.culprit, Severity
// data.issue.level. Any other resource or action is ErrNotAnAlert (answered 202, nothing written:
// Sentry sends assignment and comment events we deliberately do not take).
func ParseSentry(resource string, body []byte) (Alert, error)
```

**Authentication, per source kind** (the source is looked up by the path's `sourceId` first; an
unknown or deleted id is 401, not 404, so the route does not confirm which sources exist):
- **Azure Monitor** is configured as a **secure webhook** on an action group: it presents an
  Entra bearer. The Hub validates it with the same validator it uses for users (hub-contract's
  Entra token check, reused, never a second one): signature against Entra's keys, the org's
  tenant, not expired, and `aud` equal to the source's `Audience`. No valid bearer → 401, nothing
  read. Which app id Azure Monitor's token names as its caller is **UNVERIFIED here**; the part
  checks audience and tenant, and the step 1 preflight records the caller claim seen in a real
  test fire before any caller check is added.
- **Sentry**: `VerifySentry` over the raw body with the source's secret, read by `secret_ref`.
  Wrong or absent → 401, nothing written.

**The alert principal.** An authenticated ingest call acts as the source, `actor {kind: "app",
appId: <als-…>}` (task 143's `Actor`; task 167 fills it on events). Its only capability is
`Ingest` for its own team. The matrix row proves a state move, an assign or a priority change
under that principal is 403.

**Routes** (`server/work_alerts.go`):

```go
// POST   /v1/integrations/alerts/{sourceId}            the source (per kind, above); 202 {}
//        bodies over MaxAlertBodyBytes → 413; ErrUnknownSchema → 422 alert_schema_unknown;
//        ErrNotAnAlert → 202 and nothing written
// POST   /v1/teams/{teamId}/alert-sources              team admin; CreateAlertSourceRequest → AlertSource
// GET    /v1/teams/{teamId}/alert-sources              team admin; → []AlertSource
// DELETE /v1/teams/{teamId}/alert-sources/{sourceId}   team admin; the links and requests stay
//        (they are the team's history); only new fires stop, as 401
```

```sql
-- migrate/sql/NNNN_work_wl_alerts.sql
CREATE TABLE alert_sources (
  id          text PRIMARY KEY,            -- als-…
  team_id     text NOT NULL,
  kind        text NOT NULL CHECK (kind IN ('azure_monitor', 'sentry')),
  name        text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
  audience    text NOT NULL DEFAULT '',
  secret_ref  text NOT NULL DEFAULT '',   -- sentry only; a Key Vault reference, never the secret
  created_by  text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,
  CHECK ((kind = 'azure_monitor' AND audience <> '' AND secret_ref = '')
      OR (kind = 'sentry' AND secret_ref <> '' AND audience = ''))
);
CREATE TABLE alert_links (
  source_id       text NOT NULL REFERENCES alert_sources(id),
  external_id     text NOT NULL,
  request_id      text NOT NULL,           -- rq-…
  fired_count     integer NOT NULL CHECK (fired_count >= 1),
  last_fired_at   timestamptz NOT NULL,
  resolved_at     timestamptz,
  quote_truncated boolean NOT NULL DEFAULT false,
  UNIQUE (source_id, external_id)
);
-- Row-level security follows task 06's pattern for team-owned tables (its policy text wins).
-- No text column: the words live only on the request, so task 146's erase removes them and
-- these rows keep ids and counts, as 05 §3.6's erase rule keeps keys.
```

**A re-fire on a declined request** bumps the count and leaves the request declined: a person's
decision is not undone by a machine. The Triage row still shows the count, so a person can see
the alert kept firing after they declined it.

#### Tests first — each with the mutant it kills

Pure rows run without tags; route, matrix and conformance rows under `-tags integration`, and
the conformance rows also against the fake.

| Test (file) | What it asserts | Mutant it kills |
|---|---|---|
| `TestFiredMakesOneTriageRequest` (`work_alerts_integration_test.go`) | the golden `azure-fired.json` with a valid bearer → one `rq-`, `channel` `alert`, requester `{kind: "alert", displayName: <source name>}`, `state` `open`, in the source team's triage (no rule matched); no issue exists | an alert that skips intake, or lands in backlog as an issue |
| `TestRuleRoutesBySeverity` | an intake rule `{channel: "alert", textContains: "Sev0"}` routing to project P; a `Sev0` fire is routed to P and `request.created` carries the rule id; a `Sev3` fire is not | severity invisible to `Route` |
| `TestRefireDedups` | the same `alertId` fired twice → one request, `firedCount: 2`, `lastFiredAt` the second's; the request's quote equals the first fire's | a missing dedup, or a quote rewritten by a later fire |
| `TestResolvedMovesNothing` | a request accepted into issue I; then `azure-resolved.json` → the link's `resolvedAt` set; I's state unchanged; the store's `MoveIssueState` spy recorded zero calls | an alert that resolves work |
| `TestRefireAfterResolveClears` | fired, resolved, fired → `resolvedAt` null, `firedCount: 2` | a stale *resolved* on a firing alert |
| `TestDeclinedStaysDeclined` | a declined request re-fired → still `declined`, `firedCount` bumped | a machine undoing a person's decision |
| `TestSentryWrongSignature401` | a flipped digit in `Sentry-Hook-Signature` → 401 and no row; an absent header → 401 | unauthenticated intake |
| `TestAzureWithoutBearer401` | no `Authorization`, an expired token, and a token for another audience → 401 each, and no row | unauthenticated intake; any-audience tokens |
| `TestUnknownSchema422` | `azure-unknown-schema.json` (`schemaId` something else) → 422 `alert_schema_unknown`, no row | a guessing parser |
| `TestQuoteCappedAt4KiB` (`intake_test.go`) | a 64 KiB description → a quote of at most `MaxAlertQuoteBytes` bytes that is valid UTF-8, `truncated` true; a 3-byte rune straddling the limit is dropped whole | an unbounded quote, or a cut mid-rune |
| `TestBodyCap413` | a 300 KiB body → 413 before the parser runs (a parser spy is not called) | memory spent on hostile input |
| `TestSentryIgnoresNonAlerts` | resource `comment` or action `assigned` → 202, no row | assignment sync pretending to be intake |
| `TestAlertPrincipalCannotMove` (`matrix_integration_test.go`) | under the alert principal: `POST …/state`, an assignee PATCH and a priority PATCH → 403 each | a fact written by an integration |
| `TestSourceCRUDAdminOnly` | a team `member` create, list or delete → 403; an `admin` succeeds; a create response and a list body have no field holding the Sentry secret (reflection over `AlertSource` and a raw-body search for the planted secret) | a secret returned, or a member adding sources |
| `TestUnknownSource401` | a well-formed fire at a random `als-` id, and at a deleted source → 401, not 404 | a route that enumerates sources |
| `TestEraseKeepsLinkIds` | task 146's erase on an alert request → the quote gone (*erased*); the `alert_links` row keeps `externalId`, `requestId` and `firedCount` | an erase that misses alert text, or that loses the dedup key so the next fire makes a second request |
| conformance (`conformance/work*.go`) | the three `WorkIntake` methods behave the same on the fake and over HTTP, including `[]` for no sources and the write-only secret | a fake that drifts from the Hub |

#### Steps

- [ ] **Step 1: Preflight.** Tasks 143, 146 and 147 merged on the sync base, or wait. Re-read
  their names (the create function, `Route`, `Incoming`, `Actor`, the erase, the event kinds);
  theirs win. Re-check `als-` and the migration number. Record in the commit body whether a real
  Azure Monitor test fire was available; if one was, record its token's caller claim.
- [ ] **Step 2: The parsers, tests first.** Write the golden files and `azuremonitor_test.go`,
  `sentry_test.go`, `intake_test.go`; `go test ./control-plane/internal/hub/alerts/... -count=1`.
  Expected: FAIL, `undefined: alerts.ParseAzureMonitor`. Implement; re-run. Expected: `ok`.
- [ ] **Step 3: The wire, the fake and conformance, tests first.** Add the SW.21 types and the
  three methods; conformance rows red on the fake, then green.
- [ ] **Step 4: The store, the migration and the routes, tests first.** The integration rows red
  (404), then `Ingest`, `store/work_alerts.go`, `server/work_alerts.go`; green with
  `-tags integration`.
- [ ] **Step 5: The matrix rows**, then the whole acceptance, then commit (`git add` the new
  files, then `git commit -F <msgfile> -- <paths>`).

#### Acceptance

```bash
go test ./control-plane/internal/hub/alerts/... -race -count=1
# expected: "ok"
go test -tags integration ./control-plane/internal/hub/server/... ./control-plane/internal/hub/conformance/... -run 'Alert|Matrix|Conformance' -count=1
# expected: "ok" per package
go test ./control-plane/internal/hub/hubclient/... -count=1
# expected: "ok": the fake passes the same conformance rows
```

#### What this task does not claim

- **Not resolving the alert back** in Azure Monitor or Sentry when the issue is done, and not
  Sentry's assignee sync. Linear does both (01 L-209); ours stops at intake and the inbound
  *resolved* fact.
- **Not that an action group or Sentry can reach the Hub** through phase 8's edge allowlist
  (**UNVERIFIED**), and **not a live alert** from either vendor: every payload here is a golden
  file written from public documentation, **UNVERIFIED**.
- **Not a caller check on Azure Monitor's token** beyond audience and tenant, until a real fire
  shows the claim (step 1).
- **Not other Azure Monitor schemas** (the legacy metric, log and activity-log shapes) nor Sentry
  metric alerts; each is a 422 or a 202 as stated.
- **Not paging or on-call.** Who answers the triage row is task 121's rotation, and task 184's
  import from PagerDuty or Opsgenie.
- Design: 05 §1 row L-209 (*"Azure Monitor first (OD-18), Sentry next"*), §3.5 (requests), §3.6
  (no integration writes a fact; id prefixes). Parity: closes **L-209**.

**Interfaces:**
- Produces:
  - SW.21 additions: `ChannelAlert`, `RequesterKindAlert`, `PrefixAlertSource`, the two kinds,
    `MaxAlertQuoteBytes`, `MaxAlertBodyBytes`, `AlertSource`, `CreateAlertSourceRequest`,
    `AlertLink`, `Request.Alert`
  - `hubclient.WorkIntake.CreateAlertSource`, `.AlertSources`, `.DeleteAlertSource`, with fake
    rules and conformance rows
  - `alerts.Alert`, `QuoteOf`, `Ingest`, `ParseAzureMonitor`, `ParseSentry`, `VerifySentry`
  - the four routes; `alert_sources`, `alert_links`; the event drafts
    `request.alert_fired {firedCount}` and `request.alert_resolved {}` (task 13 makes them
    constants, SW.5)
- Consumes:
  - task 146 — the request create function, `Request`, the erase; task 147 — `Route`,
    `Incoming`, `Decision`; task 143 — SW.21 and `Actor`; task 144 — the `secret_ref` pattern;
    task 170 — the Triage row that shows *fired N times* and *resolved*; the Hub's Entra validator
- Amends: SW.21 gains two enum values, three types and one read-only `Request` field (additive).
