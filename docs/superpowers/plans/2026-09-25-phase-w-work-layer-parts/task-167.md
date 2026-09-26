### Task 167: audit: each event says person or agent, and whose agent; streaming to Microsoft Sentinel

W5, stream WH, about 2.5 hours (an estimate, not a measurement). Depends on task **145** (the
actor settings, `task-145.md:62-67`, `:125-131`) and Hub task **H27** (the audit route and `hub
verify`'s chain walk, hub tree `task-27.md:22-61`). It reads task 143's `ActorKind` and `Actor`
(`task-143.md:152-176`), TM-03's v2 audit envelope (`via`, `delegationId`, team tree
`task-03.md:1380-1400`), and the `data.via` values tasks 150, 153 and 154 already write
(`task-153.md:935`, `task-154.md:720`).

Parity: closes **L-239**'s Sentinel half (the audit itself exists, hub-contract:1133-1134).
Design: 05 §1 P (L-239, `05-design.md:421`), §3.6 (the janitor names the policy's author). OD-16
(an agent acts as its owner and says so), OD-27 (proof, not prediction).

**What the reader does.** Ada, org admin of *KPMG AU*, sets `PANACEA_APP_HUB_SENTINEL` for her
tenant's data collection rule. Within a minute her Sentinel workspace holds a `PanaceaAudit_CL`
row per audit event, audit-only kinds such as `auth.forbidden` included. A row reads `ActorKind
agent · ActorId u-priya · Via fleet · Delegation dl-…`: Priya's agent commented, not Priya. A
janitor rollover reads `ActorKind janitor · ActorId u-dylan`: the policy Dylan wrote did it. When
someone edits an audit row by hand, the export stops and says where; it never skips the row.

**Decisions made here, so nobody re-derives them:**

1. **Who acted is derived, never stored twice.** `hubapi.ActorOf(env AuditEnvelope)` is pure, over
   what the audit row already holds: `actorUserId`, TM-03's `via`/`delegationId`, and
   `data.via`/`data.author`. No column, no envelope field (rule H1), no second source to disagree.
2. **W5 principals say so in `data.via`, not in the envelope.** TM-03's check refuses any
   envelope `via` but `fleet` (team tree `task-03.md:1393-1395`), so an app writes `data.via
   "app:oa-…"` and a key `"key:ak-…"`, as a host already writes `"host:github"` (task 154) and a
   sync write the `tc-` id (task 153). One place stamps it: `Tx.Emit`, from the settings tasks 159
   and 160 set (`hub.work_actor_app`, `hub.work_actor_key`); it never overwrites a `via` a handler set.
3. **Teams is a channel, not a kind.** The scope lists `teams`; task 143's closed set has seven
   kinds and no `teams` (`task-143.md:166`). A Teams ask is the person's own act, so it is
   `person` with `Via "teams"`. 143 wins.
4. **The exporter reads the store, not `/v1/audit`.** Route 44 appends `audit.read` per page
   (hub tree `task-27.md:26-29`), which the exporter would then export, forever. It reads
   `audit_events` in-process, per org, on a janitor transaction, and appends no event per batch:
   its cursor row is its record.
5. **The cursor moves only after a 2xx, and the chain is checked before a post.** Each row's
   `prevHash` must equal the cursor's `hash` (then the previous row's) and `hash` must be
   `ChainHash(prevHash, canonical)`. A break stops that org's export with an error; it never
   skips, and never posts a row after the break.
6. **At most 1 MB per post; a 429 waits `Retry-After`.** Batches fill to `MaxPostBytes` and no
   further. A 5xx backs off 1 s doubling to 5 min. A 400, 403 or 413 stops the org's loop: the
   rule or the role is wrong and a retry cannot fix it. A 401 refreshes the token once.
7. **One stream per org.** Each entry of `PANACEA_APP_HUB_SENTINEL` names a tenant; the exporter
   finds that tenant's enrolled org and runs one loop for it under `hub.org_id`. There is no
   global stream and no entry without a tenant.
8. **The export shows metadata only.** The record is fifteen named fields. `Data` is the event's
   data as stored (hub-contract §8 keeps content out of events) **minus `reason`**, which only
   `brief.erased` holds and the feed hides too (hub tree `task-27.md:76`); an admin reads it at
   `/v1/audit`.
9. **The token is the Hub's workload identity**, for `https://monitor.azure.com//.default`, from
   the entry's tenant. No secret is configured. It reuses Hub task 08's exchange with the scope
   as a parameter (gap 3).

**Files:**
- `control-plane/internal/hub/hubapi/work_actor.go` (+ `work_actor_test.go`) — `ActorOf` (new)
- `control-plane/internal/hub/store/tx.go` ⚑ (Hub task 14) — `Emit` stamps `data.via` (decision 2)
- `control-plane/internal/hub/store/sentinel.go` (+ `_integration_test.go`) — rows after a cursor,
  the cursor (new)
- `control-plane/internal/hub/sentinel/{ingest.go,export.go}` (+ `_test.go` each) — the client and
  the loop (new; the File map has **no row** for this package, gap 1)
- `control-plane/cmd/hub/config.go` ⚑ — `PANACEA_APP_HUB_SENTINEL`; `cmd/hub/serve.go` ⚑ — start
  one exporter per entry (the file name is Hub task 08's, **UNVERIFIED**)
- the table `work_sentinel_cursors`, required of task 145 below (gap 2)

#### Interfaces — complete: `control-plane/internal/hub/hubapi/work_actor.go`

```go
package hubapi

import "strings"

// Attribution is who an audit event says acted (05 L-239). ActorID is always
// the person the act is attributed to: the agent's owner for an agent, the
// policy's author for the janitor, the mapped person for a host. It is ""
// only for an integration or a janitor act with no author.
type Attribution struct {
	Kind       ActorKind
	ActorID    string // u-…
	Via        string // "fleet", "app:oa-…", "key:ak-…", "host:github", "tc-…", "teams", or ""
	Agent      string // the agent's id when the event's data names one (data.agentId), else ""
	Delegation string // dl-… for an agent act (TM-03 v2), else ""
}

// ActorOf reads the audit row only (decision 1). The order matters: an
// envelope via beats data, and data.via beats a null actor.
func ActorOf(env AuditEnvelope) Attribution {
	agent, _ := env.Data["agentId"].(string)
	if env.Via == ViaFleet { // TM-03: actorUserId is the owner, always
		return Attribution{Kind: ActorAgent, ActorID: env.ActorUserID, Via: env.Via,
			Agent: agent, Delegation: env.DelegationID}
	}
	via, _ := env.Data["via"].(string)
	a := Attribution{ActorID: env.ActorUserID, Via: via}
	switch {
	case strings.HasPrefix(via, "app:"):
		a.Kind = ActorApp
	case strings.HasPrefix(via, "key:"):
		a.Kind = ActorKey
	case strings.HasPrefix(via, "host:"):
		a.Kind = ActorHost
	case strings.HasPrefix(via, PrefixTrackerConnection+"-"):
		a.Kind = ActorIntegration
	case env.ActorUserID == "":
		a.Kind = ActorJanitor
		a.ActorID, _ = env.Data["by"].(string) // §3.6: the policy's author (task 121's key)
		if a.ActorID == "" {
			a.ActorID, _ = env.Data["author"].(string) // the scope's name, if a kind uses it
		}
	default:
		a.Kind = ActorPerson // "teams" included (decision 3)
	}
	return a
}
```

`ViaFleet` and `AuditEnvelope.Via`/`DelegationID` are TM-03's; `PrefixTrackerConnection` is task
143's `tc` prefix constant (**UNVERIFIED** name; the literal is `"tc"`, `task-150.md`). A sync
write's `actor` is null (`task-150.md:35-37`), so the `tc-` case is tested before the null case,
and a sync event is `integration`, never `janitor`. Every `ActorKind` but `agent` is reachable
from a v1 row, so `ActorOf` works on this branch before Phase T merges, with `Via` never `fleet`.
The janitor's author rides on the companion event, `template.schedule_ran {by}` (`task-121.md:357`)
and `tracker.page {by}` (`task-150.md:40-42`), not on the `issue.created` beside it, so that row
reads `janitor` with `ActorID ""`: true, and the companion row, one `Seq` later, names the author.

#### The `data.via` stamp — `store/tx.go` ⚑ (Hub task 14), the added lines

```go
// In Emit, before the event joins the pending list (decision 2):
	if v := tx.principalVia(); v != "" {
		if e.Data == nil {
			e.Data = map[string]any{}
		}
		if _, set := e.Data["via"]; !set { // a handler's own via wins (154's host:…)
			e.Data["via"] = v
		}
	}

// principalVia is "app:"+AppID or "key:"+KeyID for an app or key transaction,
// read from the Claims Write's prologue already copies into the settings
// hub.work_actor_app and hub.work_actor_key (tasks 159, 160); "" otherwise.
func (tx *Tx) principalVia() string
```

This changes the bytes of no existing event: a person's, a Fleet and a janitor transaction have no
app or key, so `principalVia` is `""` and `Data` is untouched. `Data` values stay strings.

#### The table — `work_sentinel_cursors`, as this task requires it of task 145 (complete)

```sql
-- task 167: one row per org per stream; the exporter's only write (decision 4).
CREATE TABLE work_sentinel_cursors (
  org_id      text        NOT NULL REFERENCES orgs (id),
  stream      text        NOT NULL CHECK (stream ~ '^Custom-[A-Za-z0-9_]{1,60}$'),
  seq         bigint      NOT NULL CHECK (seq >= 0),               -- the last seq acknowledged
  hash        bytea       NOT NULL CHECK (octet_length(hash) = 32), -- that row's hash; 32 zero bytes at 0
  updated_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (org_id, stream)
);
ALTER TABLE work_sentinel_cursors ENABLE ROW LEVEL SECURITY;
ALTER TABLE work_sentinel_cursors FORCE ROW LEVEL SECURITY;
CREATE POLICY work_sentinel_cursors_org ON work_sentinel_cursors
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT, UPDATE ON work_sentinel_cursors TO hub_app; -- no DELETE: a reset is an owner act
```

`Custom-` is the prefix Azure Monitor gives a data collection rule's custom input stream
(**UNVERIFIED** against the live service). The cursor names a stream as well as an org, so a
second stream for the same org starts at 0 and replays the whole chain once, by design.

#### The store — `store/sentinel.go` (signatures complete)

```go
package store

// SentinelRow is one audit_events row as the exporter needs it.
type SentinelRow struct {
	Env       hubapi.AuditEnvelope // parsed from canonical (hubapi.ParseAuditEnvelope, H02 D3)
	Canonical []byte
	Hash      [32]byte
	PrevHash  [32]byte
}

// SentinelCursor reads the org's cursor for stream, or {0, 32 zero bytes}.
func (tx *Tx) SentinelCursor(ctx context.Context, stream string) (seq int64, hash [32]byte, err error)

// AuditAfter reads up to limit rows with seq > after, in seq order, from
// audit_events under the transaction's hub.org_id: every kind, feed or not.
func (tx *Tx) AuditAfter(ctx context.Context, after int64, limit int) ([]SentinelRow, error)

// AdvanceSentinel upserts the cursor to (seq, hash) only if seq is greater
// than the stored one; a lower or equal seq is a no-op, never a rewind.
func (tx *Tx) AdvanceSentinel(ctx context.Context, stream string, seq int64, hash [32]byte) error
```

Each call runs in its own short janitor transaction for one org (Hub task 24's pattern: the
Hub's process, `hub.org_id` set, no principal), so no transaction is held open across a post.

#### Configuration — `cmd/hub/config.go` ⚑ (appended to Hub task 08's table)

| Variable | Default | Meaning |
|---|---|---|
| `PANACEA_APP_HUB_SENTINEL` | unset | A JSON list `[{"tenantId","endpoint","dcr","stream"}]`. Unset or `[]`: no exporter runs. `tenantId` is an enrolled tenant's GUID (lowercase), `endpoint` an `https://` data collection endpoint with no path, query or user info, `dcr` the rule's immutable id (`dcr-` then 32 hex), `stream` a `Custom-…` name. One entry per tenant; a second for the same tenant, an unknown tenant, or any other key refuses to start. No secret: decision 9 |

```go
// In Config (H08:82):
	Sentinel []SentinelTarget

type SentinelTarget struct {
	TenantID string `json:"tenantId"`
	Endpoint string `json:"endpoint"`
	DCR      string `json:"dcr"`
	Stream   string `json:"stream"`
}

// parseSentinel decodes strictly (DisallowUnknownFields) and checks every
// field; an error names the entry's index and field, never the value.
func parseSentinel(raw string, enrolled []Tenant) ([]SentinelTarget, error)
```

#### Interfaces — complete: `control-plane/internal/hub/sentinel/ingest.go`

```go
// Package sentinel streams each enrolled org's audit chain to that org's own
// Azure Monitor data collection rule, for Microsoft Sentinel (05 L-239).
// Standard library only.
package sentinel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// APIVersion is the Logs Ingestion API version the part pins. UNVERIFIED
// against the live service (no workspace exists, phase 8); step 1 checks it.
const APIVersion = "2023-01-01"

// MonitorScope is the resource the token is for. The double slash is Azure
// Monitor's documented form; UNVERIFIED like APIVersion.
const MonitorScope = "https://monitor.azure.com//.default"

// MaxPostBytes is the Logs Ingestion API's per-call limit (decision 6).
const MaxPostBytes = 1 << 20

// Token returns a bearer for MonitorScope in one tenant (decision 9); fresh
// skips its cache, which the client asks for once after a 401.
type Token func(ctx context.Context, fresh bool) (string, error)

// Target is one PANACEA_APP_HUB_SENTINEL entry. cmd/hub/config.go declares
// `type SentinelTarget = sentinel.Target`, so the two are one type.
type Target struct {
	TenantID string `json:"tenantId"`
	Endpoint string `json:"endpoint"`
	DCR      string `json:"dcr"`
	Stream   string `json:"stream"`
}

// Poster posts one batch. Client is the real one; tests use a recorder.
type Poster interface {
	Post(ctx context.Context, body []byte) error
}

// Client posts to {endpoint}/dataCollectionRules/{dcr}/streams/{stream}.
type Client struct {
	URL   string // built by NewClient; never logged with a token
	Token Token
	HTTP  *http.Client
	fresh bool // set after one refresh on a 401, cleared on a 2xx
}

func NewClient(t Target, tok Token, hc *http.Client) *Client {
	return &Client{Token: tok, HTTP: hc, URL: fmt.Sprintf(
		"%s/dataCollectionRules/%s/streams/%s?api-version=%s", t.Endpoint, t.DCR, t.Stream, APIVersion)}
}

// RetryAfter is a 429 or 5xx: wait Wait, then post the same bytes again.
type RetryAfter struct{ Wait time.Duration; Status int }

func (r *RetryAfter) Error() string { return fmt.Sprintf("sentinel: %d, retry in %s", r.Status, r.Wait) }

// ErrRefused is a status no retry can fix (400, 403, 413, a second 401).
var ErrRefused = errors.New("sentinel: the ingestion endpoint refused the batch")

func (c *Client) Post(ctx context.Context, body []byte) error {
	tok, err := c.Token(ctx, c.fresh)
	if err != nil {
		return &RetryAfter{Wait: time.Second, Status: 0} // the token endpoint is down: back off
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return &RetryAfter{Wait: time.Second}
	}
	defer res.Body.Close()
	switch s := res.StatusCode; {
	case s >= 200 && s < 300:
		c.fresh = false
		return nil
	case s == http.StatusTooManyRequests:
		return &RetryAfter{Wait: retryAfter(res.Header.Get("Retry-After")), Status: s}
	case s == http.StatusUnauthorized && !c.fresh:
		c.fresh = true // the next call asks the token source to skip its cache
		return &RetryAfter{Wait: 0, Status: s}
	case s >= 500:
		return &RetryAfter{Wait: time.Second, Status: s} // the loop doubles it (decision 6)
	default:
		return fmt.Errorf("%w: %d", ErrRefused, s)
	}
}

// retryAfter reads delta-seconds (1..300); anything else is 30 s. An
// HTTP-date is not honoured: the service documents seconds (UNVERIFIED).
func retryAfter(h string) time.Duration {
	if n, err := strconv.Atoi(h); err == nil && n >= 1 && n <= 300 {
		return time.Duration(n) * time.Second
	}
	return 30 * time.Second
}
```

#### Interfaces — complete: `control-plane/internal/hub/sentinel/export.go`, the record

```go
package sentinel

// Record is one row of the custom table: these fifteen keys and no other
// (decision 8). The DCR's stream declaration must list the same columns.
type Record struct {
	TimeGenerated string          `json:"TimeGenerated"` // the event's at, RFC 3339 UTC
	OrgId         string          `json:"OrgId"`
	Seq           int64           `json:"Seq"`
	Kind          string          `json:"Kind"`
	TeamId        string          `json:"TeamId"`
	ProjectId     string          `json:"ProjectId"`
	Subject       string          `json:"Subject"`
	ActorId       string          `json:"ActorId"`
	ActorKind     string          `json:"ActorKind"`
	Via           string          `json:"Via"`
	Agent         string          `json:"Agent"`
	Delegation    string          `json:"Delegation"`
	Data          json.RawMessage `json:"Data"` // the event's data minus "reason", as JSON
	Hash          string          `json:"Hash"`     // 64 lowercase hex
	PrevHash      string          `json:"PrevHash"` // 64 lowercase hex
}

func recordOf(r store.SentinelRow) (Record, error) {
	data := make(map[string]any, len(r.Env.Data))
	for k, v := range r.Env.Data {
		if k != "reason" { // decision 8
			data[k] = v
		}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return Record{}, err
	}
	a := hubapi.ActorOf(r.Env)
	return Record{TimeGenerated: r.Env.At, OrgId: r.Env.OrgID, Seq: r.Env.Seq, Kind: r.Env.Event,
		TeamId: r.Env.TeamID, ProjectId: r.Env.ProjectID, Subject: r.Env.Subject,
		ActorId: a.ActorID, ActorKind: string(a.Kind), Via: a.Via, Agent: a.Agent,
		Delegation: a.Delegation, Data: raw,
		Hash: hex.EncodeToString(r.Hash[:]), PrevHash: hex.EncodeToString(r.PrevHash[:])}, nil
}

// ErrChainBroken stops an org's export (decision 5). Its message names the
// org and seq and which rule, as hub verify's FAULT line does.
var ErrChainBroken = errors.New("sentinel: the audit chain is broken")

// ErrRecordTooLarge is one event over MaxPostBytes: never truncated, never skipped.
var ErrRecordTooLarge = errors.New("sentinel: one audit event is larger than a post")

// verify checks rows against the cursor (seq, hash), in order.
func verify(org string, seq int64, prev [32]byte, rows []store.SentinelRow) error {
	for _, r := range rows {
		switch {
		case r.Env.Seq != seq+1:
			return fmt.Errorf("%w: org %s seq %d: a gap after %d", ErrChainBroken, org, r.Env.Seq, seq)
		case r.PrevHash != prev:
			return fmt.Errorf("%w: org %s seq %d: prevHash", ErrChainBroken, org, r.Env.Seq)
		case hubapi.ChainHash(r.PrevHash, r.Canonical) != r.Hash:
			return fmt.Errorf("%w: org %s seq %d: hash", ErrChainBroken, org, r.Env.Seq)
		}
		seq, prev = r.Env.Seq, r.Hash
	}
	return nil
}
```

`verify` is `hub verify`'s rules 1-3 (hub tree `task-27.md:52-54`) over a window, so a row edited
after `hub verify` last ran is caught by the next export, not by the next manual walk. It does not
recheck rule 4 (the columns against the canonical): `SentinelRow.Env` **is** the parsed canonical,
so the record cannot say what the columns say and the hash does not.

#### Interfaces — complete: `export.go`, the loop

```go
// Source is one org's slice of the store; storeSource wraps store.Store with
// a janitor transaction per call under hub.org_id = OrgID (decision 7).
type Source interface {
	Cursor(ctx context.Context) (seq int64, hash [32]byte, err error)
	After(ctx context.Context, seq int64, limit int) ([]store.SentinelRow, error)
	Advance(ctx context.Context, seq int64, hash [32]byte) error
}

// Exporter is one org's loop. There is no shared state between two.
type Exporter struct {
	OrgID     string
	Src       Source
	Post      Poster
	Sleep     func(ctx context.Context, d time.Duration) error // time.After in main; a recorder in tests
	Log       *slog.Logger
	PollEvery time.Duration // 30 s in main
}

const readLimit = 500

// Run exports until ctx ends or a stop error (chain, refusal, oversize), which
// it logs at error level and returns: the org's export stops, the Hub serves on.
func (e *Exporter) Run(ctx context.Context) error {
	backoff := time.Second
	for ctx.Err() == nil {
		n, err := e.once(ctx)
		var ra *RetryAfter
		switch {
		case err == nil && n == 0:
			err = e.Sleep(ctx, e.PollEvery)
		case err == nil:
			backoff = time.Second
			continue
		case errors.As(err, &ra):
			wait := ra.Wait
			if ra.Status != http.StatusTooManyRequests && ra.Status != http.StatusUnauthorized {
				wait, backoff = backoff, min(backoff*2, 5*time.Minute)
			}
			err = e.Sleep(ctx, wait)
		default:
			e.Log.Error("sentinel export stopped", "org", e.OrgID, "err", err)
			return err
		}
		if err != nil {
			return err // ctx cancelled while sleeping
		}
	}
	return ctx.Err()
}

// once posts at most one batch and returns how many events it acknowledged.
func (e *Exporter) once(ctx context.Context) (int, error) {
	seq, prev, err := e.Src.Cursor(ctx)
	if err != nil {
		return 0, &RetryAfter{Wait: time.Second}
	}
	rows, err := e.Src.After(ctx, seq, readLimit)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	if err := verify(e.OrgID, seq, prev, rows); err != nil {
		return 0, err // decision 5: nothing after a break is posted
	}
	body, n, err := batch(rows)
	if err != nil {
		return 0, err
	}
	if err := e.Post.Post(ctx, body); err != nil {
		return 0, err // the cursor has not moved (decision 5)
	}
	last := rows[n-1]
	return n, e.Src.Advance(ctx, last.Env.Seq, last.Hash)
}

// batch is a JSON array of the leading rows that fit in MaxPostBytes, and how many.
func batch(rows []store.SentinelRow) ([]byte, int, error) {
	buf := []byte{'['}
	for i, r := range rows {
		rec, err := recordOf(r)
		if err != nil {
			return nil, 0, err
		}
		b, err := json.Marshal(rec)
		if err != nil {
			return nil, 0, err
		}
		if len(buf)+len(b)+2 > MaxPostBytes { // a comma and the closing bracket
			if i == 0 {
				return nil, 0, fmt.Errorf("%w: org %s seq %d", ErrRecordTooLarge, r.Env.OrgID, r.Env.Seq)
			}
			return append(buf, ']'), i, nil
		}
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, b...)
	}
	return append(buf, ']'), len(rows), nil
}
```

#### Tests to write first — `ActorOf` and the stamp

`hubapi/work_actor_test.go` (unit, table over `AuditEnvelope` literals):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAFleetCommentIsTheOwnersAgent` | `Via: fleet, DelegationID: dl-…, ActorUserID: u-priya, Data{agentId: ag-…}` → `{agent, u-priya, fleet, ag-…, dl-…}` | an agent's act shown as the person's own |
| `TestAnAppAndAKeyAreNamed` | `data.via "app:oa-1"` → `app`; `"key:ak-1"` → `key`; `ActorID` the person both times | an app or key acting invisibly |
| `TestAHostWriteIsTheMappedPerson` | `data.via "host:github"`, actor `u-alice` → `{host, u-alice}` | a host merge shown as Alice by hand |
| `TestASyncWriteIsIntegrationNotJanitor` | actor `""`, `data.via "tc-…"` → `integration` | the null-actor case tested first, so a sync reads as the janitor |
| `TestTheJanitorNamesThePolicysAuthor` | `template.schedule_ran`, actor `""`, `data.by u-dylan` → `{janitor, u-dylan}`; the same without `by` → `{janitor, ""}` | an unattributed act, or an author invented |
| `TestATeamsAskIsThePersons` | `data.via "teams"`, actor `u-priya` → `{person, u-priya, teams}` | a closed set of seven widened to eight (decision 3) |

`store/tx_via_integration_test.go` (`//go:build integration`): an app's comment (task 159's
fixture) and a key's (task 160's) each append one event whose `data.via` is `app:oa-…` and
`key:ak-…`; a person's comment's data has no `via` key and its canonical bytes equal a fixture
recorded before this task (kills a stamp that moves every existing digest); task 154's host
merge keeps `host:github` (kills the stamp overwriting a handler's own `via`).

#### Tests to write first — the client and the loop

`sentinel/ingest_test.go`, against an `httptest.Server`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestThePostIsTheDocumentedCall` | path `/dataCollectionRules/dcr-…/streams/Custom-PanaceaAudit_CL`, query `api-version=2023-01-01`, `Authorization: Bearer t1`, `Content-Type: application/json` | a URL built from the wrong fields |
| `TestA429ReadsRetryAfter` | `429` with `Retry-After: 7` → `*RetryAfter{7s, 429}`; `Retry-After: soon` → 30 s | a tight retry loop |
| `TestA401RefreshesOnce` | two 401s: the first is a `RetryAfter` and the next call asks `Token(ctx, true)`; the second is `ErrRefused` | a token refresh storm, or a stale token forever |
| `TestRefusalsStop` | 400, 403 and 413 → `errors.Is(err, ErrRefused)` | a bad rule retried forever |
| `TestTheTokenIsNeverLogged` | a `slog` buffer across every case holds no `t1` | a bearer in the Hub's log |

`sentinel/export_test.go`, over `memSource` (a slice of chained rows built with `ChainHash`, a
cursor, and a record of `Advance` calls), `recPoster` (scripted answers, the bodies kept) and
`recSleep` (the durations asked, returns at once):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheCursorMovesOnlyAfterA2xx` | answers `503`, then `200`: after the first `once`, no `Advance`; after the second, one `Advance` to the last seq and its hash | advanced before the ack, losing a batch |
| `TestA429WaitsItsRetryAfter` | answers `429 Retry-After: 7`, then `200`: `recSleep` holds exactly `[7s]` | a tight retry loop |
| `TestA5xxBacksOffDoubling` | four `503`s then `200`: `[1s 2s 4s 8s]` | a fixed or no backoff |
| `TestThreeMegabytesIsThreePosts` | 30 rows, each record about 97 KB (a `Data` string): three posts of ten, each ≤ `MaxPostBytes`, cursor at 30 | one oversized post |
| `TestAnOversizeEventStops` | one record over 1 MB: `ErrRecordTooLarge`, no post, cursor unchanged | an event truncated or skipped |
| `TestABrokenPrevHashStops` | row 12's `PrevHash` flipped: rows 1-11 posted, then `Run` returns `ErrChainBroken` naming `seq 12: prevHash`, the log has one error line, no post holds seq 12 or later | a forged chain exported, or a skip |
| `TestAnEditedRowStops` | row 5's canonical altered after hashing: `ErrChainBroken`, `seq 5: hash` | an edit that keeps the links but changes a byte |
| `TestTheRecordHasFifteenKeys` | for every Work event fixture of tasks 143-166 (`hubapi` test data, one per kind), the posted object's keys are exactly the fifteen and `Data` has no `reason`, `title`, `body`, `quote`, `email` or `text` key | a text field leaks |

`recSleep` records each duration it is asked for, except `PollEvery`: the first poll sleep means
the backlog is drained, so it cancels the run's context and records nothing. The lists above are
therefore exact, and `Run` returns `context.Canceled` in every test that does not stop on an error.

#### Tests to write first — the Hub half and the configuration

`store/sentinel_integration_test.go` and `sentinel/export_integration_test.go`
(`//go:build integration`, the Hub's test database, `storeSource` over it, `recPoster` per org):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestOrgAsEventsNeverReachOrgBsStream` | orgs A and B, 20 events each, two exporters: A's poster holds only `OrgId` A and seqs 1-20, B's only B's | one global stream, or a reader missing `hub.org_id` |
| `TestAuditOnlyKindsAreExported` | an `auth.forbidden` (a refused write) and a `blob.uploaded`: both posted | the feed read instead of the audit (decision 4) |
| `TestExportingAppendsNoEvent` | a full run over 20 events: `audit_events` still holds 20 | an `audit.read` per batch, exported forever |
| `TestTheCursorNeverRewinds` | `AdvanceSentinel` to 12, then to 9: the row reads 12 | a late ack rewinding the cursor, re-exporting a range |
| `TestCursorsAreTheOrgsOwn` | as org B, `SentinelCursor` for A's stream reads `0`; `UPDATE work_sentinel_cursors` across orgs as `hub_app` → 0 rows | RLS missing on the new table |
| `TestAnOwnerEditStopsTheExport` | as the owner, with the triggers disabled as H27's verify test does (hub tree `task-27.md:84`), alter one row's `canonical`: the next run stops with `ErrChainBroken` at that seq | a forged chain exported |

`cmd/hub/config_test.go` ⚑, `parseSentinel` rows: unset and `[]` → no targets; one valid entry
→ one target; an unknown key, a second entry for one tenant, a tenant not enrolled, `http://`, an
endpoint with a path, `dcr` without `dcr-`, a `stream` without `Custom-` → each an error naming
index and field and **not** the value (kills a config echoing an endpoint into a log).

#### Wiring — `cmd/hub/serve.go` ⚑ (the added lines)

```go
// After the store opens and before the listener: one goroutine per target.
// A stopped exporter is logged and stays stopped until restart (decision 5).
for _, t := range cfg.Sentinel {
	org, _ := st.OrgForTenant(ctx, t.TenantID) // enrolled: checked by parseSentinel
	tok := store.EntraToken(cfg.Workload, t.TenantID, sentinel.MonitorScope, httpClient, time.Now)
	ex := &sentinel.Exporter{OrgID: org, Src: sentinel.NewStoreSource(st, org, t.Stream),
		Post: sentinel.NewClient(t, tok, httpClient), Sleep: sleepCtx, Log: log, PollEvery: 30 * time.Second}
	go func() { _ = ex.Run(ctx) }()
}
```

`store.EntraToken` is Hub task 08's `EntraPassword` with the tenant and scope as parameters
(`EntraPassword(w, …)` becomes `EntraToken(w, w.TenantID, DatabaseScope, …)`, byte-identical in
behaviour, gap 3); its `fresh` argument skips the cache. `OrgForTenant` and `sleepCtx` are the
two small helpers this wiring needs (**UNVERIFIED** whether H08 or H24 already has either).

The stamp's integration test is `TestAppAndKeyWritesCarryVia`.

#### Steps

- [ ] **Step 1:** Confirm the sync base (rule H1): H27's `store/auditread.go` and `hub verify`
  rules; H14's `Tx.Emit`; H08's `EntraPassword` and `Config`; task 143's `ActorKind`; tasks
  159-160's settings. Check `APIVersion`, `MonitorScope` and the `Custom-` prefix against
  Microsoft's current Logs Ingestion API page and record the date read in the commit; a
  difference changes the constants, not the design. If task 145 lacks `work_sentinel_cursors`,
  raise it against 145 (gap 2); this task waits.
- [ ] **Step 2:** `work_actor_test.go`; it fails to compile. `work_actor.go` until its six rows
  pass. TM-03's `Via`/`DelegationID` are absent until Phase T merges: until then the Fleet row is
  skipped with `t.Skip("TM-03 unmerged")` and the task is not closed.
- [ ] **Step 3:** `ingest_test.go` and `export_test.go`; they fail. `ingest.go` and `export.go`
  until the thirteen rows pass.
- [ ] **Step 4:** The stamp in `tx.go` and `TestAppAndKeyWritesCarryVia`. Record the person's
  comment fixture's canonical bytes **before** the change, so the test compares against bytes
  this task did not write.
- [ ] **Step 5:** `store/sentinel.go`, `storeSource`, the six integration rows; then
  `parseSentinel`, its rows, `EntraToken` and the wiring. Rerun `hub verify`'s own tests unchanged.
- [ ] **Step 6:** `git diff -- go.work control-plane/go.mod` shows nothing (standard library
  only); commit by pathspec.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/internal/hub/hubapi/ -race -count=1 -v \
  -run '^Test(AFleetCommentIsTheOwnersAgent|AnAppAndAKeyAreNamed|AHostWriteIsTheMappedPerson|ASyncWriteIsIntegrationNotJanitor|TheJanitorNamesThePolicysAuthor|ATeamsAskIsThePersons)$' \
  2>&1 | grep -c "^--- PASS"
# expect: 6 (5 and one SKIP before Phase T merges)
go test ./control-plane/internal/hub/sentinel/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"
# expect: 13
go test -tags integration ./control-plane/internal/hub/store/ ./control-plane/internal/hub/sentinel/ -race -count=1 -v \
  -run 'OrgAsEvents|AuditOnlyKinds|ExportingAppendsNoEvent|CursorNeverRewinds|CursorsAreTheOrgs|OwnerEditStops|AppAndKeyWritesCarryVia' \
  2>&1 | grep -c "^--- PASS"
# expect: 7
go test ./control-plane/cmd/hub/ -race -count=1 -run Sentinel
# expect: ok
go test -tags integration ./control-plane/cmd/hub/ -race -count=1 -run Verify
# expect: ok (H27's chain tests, unchanged)
git diff --stat -- go.work control-plane/go.mod
# expect: no output
```

#### What this task does NOT claim

- **That a Sentinel workspace, data collection endpoint or rule, or analytic rule exists
  anywhere** (phase 8), or that any stream has run against Azure. Every post above went to an
  `httptest` server.
- **That the Hub can get a token in a customer's tenant.** That needs the Hub's app registration
  to be multi-tenant, consented there, and granted *Monitoring Metrics Publisher* on the rule
  (**UNVERIFIED**; gap 3).
- **Exactly-once delivery.** If `Advance` fails after a 2xx, the batch is posted again; each
  record carries `OrgId` and `Seq`, so a Sentinel query can drop the duplicate. It is
  at-least-once and in order.
- **An alert.** A stop is one error-level log line naming org, seq and rule; the rule that pages
  someone on it is phase 8's.
- **Any desktop surface** that shows *Priya's agent* on an activity line; that reads `data.via`
  and the envelope in the tasks that render the feed.

**Parity rows served:** **L-239**'s Sentinel half closed; its audit half exists.
