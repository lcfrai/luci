### Task 156: outbound webhooks: metadata-only payloads on the change feed, actor and previous values, HMAC-SHA256, retries, the delivery log

W5, stream WH, seam **SW.22** (the payload and its signature header; consumed by 157, 158, 178 and
182). Depends on task 13 (SW.5, the Work kinds and `emitWork`) and task 145 (the W5 migration:
`work_webhooks`, `work_webhook_deliveries`, the principal fixtures). Types come from task 143
(SW.21: `Webhook`, `WebhookDelivery`, `CreatedWebhook`, `Actor`, the code `webhook_url_invalid`).
Hub anchors were read in the `feat/hub` worktree at `3df9fab8`: the contract
`docs/superpowers/contracts/2026-09-23-hub-contract.md` (cited `hub-contract:N`), Hub task 24's
part (the janitor, `H24:N`) and Hub task 26's part (the change feed, `H26:N`). `server/`, `store/`
and `migrate/` are **planned, not on `feat/hub`**; every path in them is **UNVERIFIED until merge**.
Tasks 143 and 145 have no part at this writing; what this part needs from them is stated below as a
requirement on their parts, and their files win.

A webhook is the change feed, delivered out (05 §3.6). It adds no second event model: the janitor
reads the same visible events `GET /v1/changes` would give the webhook's creator (`H26:24-27`,
hub-contract §7.11), and POSTs each one, signed, to a URL a team admin registered. The receiver gets
ids and changed field names, never text; it fetches text by key with its own rights (05 §1 L-251).

**The one thing this task must not get wrong** is that no customer text leaves the Hub. The feed is
already metadata only (task 13's canary), so the rule here is narrower: the payload is built **only**
from the stored event, and the actor is reduced to ids. `hubapi.Event.Actor` on `/v1/changes` is a
`*User` with `displayName` and `email` (`control-plane/internal/hub/hubapi/types.go:543` at
`3df9fab8`); the webhook carries SW.21's `Actor`, ids only, because an email address sent to a third
party's endpoint is personal data leaving the tenant. The canary test writes one marked string into
every text field W1-W5 has and reads every delivered body.

Five decisions the scope left open, made here.

1. **Delivery is at least once, not exactly once.** `Panacea-Delivery` names the delivery (one
   webhook × one event) and stays the same across its attempts, so a receiver de-duplicates on it.
   Order is by `event.seq`; a retried delivery can arrive after a later one, and the README of task
   158 says so.
2. **The janitor queues inside a transaction and sends outside it.** Each sweep, holding the Hub
   janitor's advisory lock (`H24:80-83`), (a) per org and per active webhook, in one transaction,
   appends a queued row for each new visible event and advances the webhook's cursor; then (b), still
   holding the lock but in **no** transaction, POSTs the due attempts, at most 200 per sweep and 8 at
   a time, and records each result in its own short transaction. A 10 s POST never holds a row lock
   or the org's audit head, and a second replica sends nothing because it has no lock.
3. **The delivery log is append-only.** `work_webhook_deliveries` gets one row per state: attempt `0`
   `pending` is *queued*; attempt `n ≥ 1` is `ok` (2xx), `pending` (failed, `nextAttemptAt` set) or
   `failed` (failed, none left). A delivery's state is its highest attempt row. Nothing is updated.
4. **A new webhook starts at the head.** Its cursor is the org's head `seq` at create, so it
   receives what happens next and never a backfill of history. Redeliver is the only way to send an
   old event, and only one that was queued for this webhook.
5. **Rotation overlaps for 24 hours.** `rotate` mints secret version `n+1`; for 24 h after it the
   header carries two `v1=` values (new, then old), and a receiver accepts if any matches. After that
   only the new one is sent. A receiver can switch without a missed delivery.

**Files:**

- `control-plane/internal/hub/webhooks/sign.go`, `sign_test.go` (new): `Sign`, `Verify`,
  `SignatureHeader`; pure
- `control-plane/internal/hub/webhooks/deliver.go`, `deliver_test.go` (new): the payload builder,
  the egress guard, the sender and the retry schedule; no store import
- `control-plane/internal/hub/hubapi/webhookpayload.go` (new): SW.22's wire types, in `hubapi` so
  task 158's generator documents them by reflection. **Not in the File map** (a gap to add)
- `control-plane/internal/hub/store/work_webhooks.go` (new): the webhook rows, the cursor, the log
- `control-plane/internal/hub/server/work_webhooks.go` (new): the six routes
- `control-plane/internal/hub/server/work_webhooks_integration_test.go` (new, `-tags integration`)
- `control-plane/internal/hub/server/janitor.go` ⚑ (Hub task 24's file): step 5, `sweepWebhooks`,
  after Hub task 24's four steps and before W2's and W4's steps (tasks 77, 121). **The File map's
  `janitor.go` row lists 77, 121 and 162 but not 156** (a gap to add)
- `control-plane/internal/hub/hubapi/workevents.go` ⚑ (task 13's): `IssueUpdatedData` gains the
  metadata values and their `…Prev`; three new kinds `webhook.created`, `webhook.disabled`,
  `webhook.rotated`; `WebhookKinds()`
- `control-plane/internal/hub/store/work_issues.go` ⚑ (task 08's): the patch path fills the new
  `IssueUpdatedData` fields from the row it locked (old) and the row it wrote (new)
- `control-plane/internal/hub/server/work_matrix_rows_integration_test.go` ⚑ (tasks 07, 145): the six routes' rows
- `control-plane/cmd/hub/serve.go` ⚑ (Hub task 08's): `Deps.WebhookSecrets`; nil leaves webhooks off

**Interfaces:**

- Produces:
  - **SW.22**: `hubapi.WebhookPayload`, `hubapi.WebhookEvent`; the three headers; `webhooks.Sign`,
    `webhooks.Verify` (the receiver's recipe, which task 158's README prints)
  - the six routes below, their matrix rows, and the three `webhook.*` kinds in SW.5's registry
  - `hubapi.WebhookKinds() []string`: the kinds a webhook may list
- Consumes:
  - task 143: `Webhook`, `WebhookDelivery`, `CreatedWebhook`, `Actor`, `ActorKind`, `Page[T]`,
    `CodeWebhookURLInvalid`
  - task 145: the two tables, which **must** carry the columns in *The store* below (a requirement on
    task 145's part: `cursor_seq`, `consecutive_failures`, `secret_ref`, `secret_version`,
    `prev_secret_version`, `rotated_at` on `work_webhooks`; `delivery_id`, `attempt` on the log)
  - task 13: `emitWork`, `WorkEventKinds`, `IssueUpdatedData`, `SubjectOnlyKinds()`
  - Hub task 24: `Sweep`'s lock and per-org loop (`H24:80-101`); Hub task 26: the visible-events
    query with task 13's clause (`H26:24-27`)
- Consumed by: 157 (the Teams post step runs after this step, and reads the same cursor rule), 158
  (documents SW.22), 178 and 182 (receivers)

#### `hubapi/webhookpayload.go` (complete)

```go
package hubapi

// WebhookPayload is SW.22: the one body every delivery POSTs. It is built from the
// stored audit event and nothing else. Receivers must ignore unknown fields.
type WebhookPayload struct {
	Delivery string       `json:"delivery"` // wd-…, the same on every attempt
	Event    WebhookEvent `json:"event"`
}

// WebhookEvent is the /v1/changes event the webhook's creator could read, with one
// change: Actor is SW.21's Actor (ids only), never the feed's *User, so no display
// name or email address is sent to a third party.
type WebhookEvent struct {
	Seq       int64          `json:"seq"`
	Kind      string         `json:"kind"`
	At        string         `json:"at"` // RFC 3339, UTC
	TeamID    *string        `json:"teamId"`
	ProjectID *string        `json:"projectId"`
	Subject   string         `json:"subject"`
	Actor     Actor          `json:"actor"` // task 143; {"kind":"janitor"} when the feed's actor is null
	Data      map[string]any `json:"data"`  // {} when empty; strings, int64s and bools only
}

// The three headers, and the signature's replay window.
const (
	HeaderWebhookEvent     = "Panacea-Event"     // the event's kind
	HeaderWebhookDelivery  = "Panacea-Delivery"  // wd-…
	HeaderWebhookSignature = "Panacea-Signature" // t=<unix seconds>,v1=<hex>[,v1=<hex>]
	WebhookReplayWindowSec = 300                 // a receiver refuses a t older than this
)
```

#### `hubapi/workevents.go` ⚑ (task 13's file; the added lines, complete)

`issue.updated`'s `data` keeps `fields` (task 13, the comma-joined wire names of the patch) and
gains a value and a previous value for each **metadata** field. Each is a flat string, because
event `data` holds strings, int64s and bools only (hub-contract:1166): an id as itself, a priority
as its word, an estimate in decimal, a date as `YYYY-MM-DD`, label ids sorted and comma-joined, and
an absent value as `""`. Only the fields named in `fields` are meaningful; the rest are `""`.
`title` and `description` are named in `fields` when they change and are **never valued**. The
fields W2 and W4 add (`estimate`, `dueDate`, `cycleId`, `featureId`, `parentId`) are `""` until
those slices' patch paths set them; task 08's W1 patch sets `priority`, `assigneeId`, `labelIds`.

```go
type IssueUpdatedData struct {
	Fields         string `json:"fields"` // comma-joined wire names: data carries no arrays
	Priority       string `json:"priority"`
	PriorityPrev   string `json:"priorityPrev"`
	AssigneeID     string `json:"assigneeId"`
	AssigneeIDPrev string `json:"assigneeIdPrev"`
	Estimate       string `json:"estimate"`
	EstimatePrev   string `json:"estimatePrev"`
	DueDate        string `json:"dueDate"`
	DueDatePrev    string `json:"dueDatePrev"`
	CycleID        string `json:"cycleId"`
	CycleIDPrev    string `json:"cycleIdPrev"`
	FeatureID      string `json:"featureId"`
	FeatureIDPrev  string `json:"featureIdPrev"`
	ParentID       string `json:"parentId"`
	ParentIDPrev   string `json:"parentIdPrev"`
	LabelIDs       string `json:"labelIds"`
	LabelIDsPrev   string `json:"labelIdsPrev"`
}
```

The three kinds, added to task 13's constants and registry (team-scoped, visible to the team's
readers; the URL is never in `data`, because a URL may carry a receiver's token in its query):

```go
const (
	EventWebhookCreated  = "webhook.created"
	EventWebhookDisabled = "webhook.disabled"
	EventWebhookRotated  = "webhook.rotated"
)

type WebhookCreatedData struct {
	KindsCount int64 `json:"kindsCount"`
	HasProject bool  `json:"hasProject"`
}
type WebhookDisabledData struct {
	Reason string `json:"reason"` // "by_admin" | "failures" | "creator_not_admin"
}
type WebhookRotatedData struct {
	SecretVersion int64 `json:"secretVersion"`
}

// appended to WorkEventKinds, task 13's registry:
//	{EventWebhookCreated, WebhookCreatedData{}, ScopeTeam, VisibleToReaders},
//	{EventWebhookDisabled, WebhookDisabledData{}, ScopeTeam, VisibleToReaders},
//	{EventWebhookRotated, WebhookRotatedData{}, ScopeTeam, VisibleToReaders},

// hubReleaseKinds are the Hub's feed kinds a webhook may also list (hub-contract:1155-1159).
// release.abandoned is left out: its data carries "reason", a person's text.
var hubReleaseKinds = []string{"release.reserved", "release.confirmed", "release.expired",
	"release.deployment_joined"}

// WebhookKinds is what Webhook.kinds may name: every SW.5 kind that is
// VisibleToReaders (SubjectOnly kinds such as subscription.set are private to one
// person and never leave the Hub), then hubReleaseKinds. Sorted, no duplicates.
func WebhookKinds() []string {
	out := make([]string, 0, len(WorkEventKinds)+len(hubReleaseKinds))
	for _, k := range WorkEventKinds {
		if k.Visibility == VisibleToReaders {
			out = append(out, k.Kind)
		}
	}
	out = append(out, hubReleaseKinds...)
	sort.Strings(out)
	return out
}
```

`workevents.go` gains the `sort` import. Task 144's fake rule (*"a webhook's `kinds` lists only
SW.5 kinds"*) is widened by the four release kinds; that is a note for task 144's part, and the
fake calls `WebhookKinds()` rather than keeping its own list.

#### `webhooks/sign.go` (complete)

```go
// Package webhooks signs and sends SW.22 deliveries. It imports hubapi and the
// standard library only: the store and the janitor call it, never the reverse.
package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Sign is hex(HMAC-SHA256(secret, t + "." + body)). body is the exact bytes sent:
// a receiver verifies what it read off the wire, never a re-encoding of it.
func Sign(secret []byte, t int64, body []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(strconv.FormatInt(t, 10)))
	m.Write([]byte{'.'})
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// SignatureHeader is "t=<t>,v1=<sig>" with one v1 per secret, newest first; two
// only in the 24 h after a rotation (decision 5).
func SignatureHeader(t int64, body []byte, secrets ...[]byte) string {
	var b strings.Builder
	b.WriteString("t=" + strconv.FormatInt(t, 10))
	for _, s := range secrets {
		b.WriteString(",v1=" + Sign(s, t, body))
	}
	return b.String()
}

var (
	ErrSignatureMalformed = errors.New("webhooks: signature header malformed")
	ErrSignatureStale     = errors.New("webhooks: signature timestamp outside the window")
	ErrSignatureMismatch  = errors.New("webhooks: no v1 signature matches")
)

// Verify is the receiver's check, and the recipe task 158's README prints: parse
// t and every v1; refuse |now - t| > window (hubapi.WebhookReplayWindowSec); accept
// when any v1 equals Sign(secret, t, body), compared with hmac.Equal.
func Verify(header string, body, secret []byte, now time.Time, window time.Duration) error {
	var t int64 = -1
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		switch {
		case !ok:
			return ErrSignatureMalformed
		case k == "t":
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n < 0 {
				return ErrSignatureMalformed
			}
			t = n
		case k == "v1":
			sigs = append(sigs, v)
		} // unknown keys are ignored, so a later v2 is additive
	}
	if t < 0 || len(sigs) == 0 {
		return ErrSignatureMalformed
	}
	if d := now.Sub(time.Unix(t, 0)); d > window || d < -window {
		return ErrSignatureStale
	}
	want := []byte(Sign(secret, t, body))
	for _, s := range sigs {
		if hmac.Equal([]byte(s), want) {
			return nil
		}
	}
	return ErrSignatureMismatch
}
```

#### `webhooks/deliver.go` (complete: the guard, the schedule, the sender)

```go
package webhooks

// Backoff is the wait before attempt n+1 after attempt n failed (n = 1..5). After
// the 6th attempt fails the delivery is "failed". A redeliver is one extra attempt
// with no schedule of its own.
var Backoff = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute,
	2 * time.Hour, 6 * time.Hour}

// DisableAfter consecutive failed attempts (any delivery; a 2xx resets it) disables
// the webhook with reason "failures".
const DisableAfter = 20

// NextAttempt returns when attempt n+1 is due after attempt n failed at 'at', and
// false when n was the last. n ≥ 1.
func NextAttempt(n int, at time.Time) (time.Time, bool) {
	if n < 1 || n > len(Backoff) {
		return time.Time{}, false
	}
	return at.Add(Backoff[n-1]), true
}

var ErrURLRefused = errors.New("webhooks: URL refused")

// blocked are the ranges no delivery may reach: private, loopback, link-local,
// unspecified, multicast, shared (CGNAT), Azure's platform address 168.63.129.16,
// and the cloud metadata addresses. IPv4-mapped IPv6 is unmapped first.
var blocked = mustPrefixes("0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
	"169.254.0.0/16", "172.16.0.0/12", "192.168.0.0/16", "168.63.129.16/32", "224.0.0.0/4",
	"240.0.0.0/4", "::/128", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8", "fd00:ec2::254/128")

// Allowed reports whether ip may be dialled.
func Allowed(ip netip.Addr) bool {
	ip = ip.Unmap()
	for _, p := range blocked {
		if p.Contains(ip) {
			return false
		}
	}
	return ip.IsValid()
}

// CheckURL is the create-time check: https, a host, no user info, no fragment, and
// every address the host resolves to Allowed. The same rule runs again at connect
// time (Dialer), so a name that later resolves inward (DNS rebinding) still fails.
func CheckURL(ctx context.Context, r *net.Resolver, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return ErrURLRefused
	}
	addrs, err := r.LookupNetIP(ctx, "ip", u.Hostname())
	if err != nil || len(addrs) == 0 {
		return ErrURLRefused
	}
	for _, a := range addrs {
		if !Allowed(a) {
			return ErrURLRefused
		}
	}
	return nil
}

// Dialer refuses, at connect, any address that is not Allowed. Control runs after
// resolution and before connect(2), with the literal address being dialled.
func Dialer() *net.Dialer {
	return &net.Dialer{Timeout: 5 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		ap, err := netip.ParseAddrPort(address)
		if err != nil || !Allowed(ap.Addr()) {
			return ErrURLRefused
		}
		return nil
	}}
}

// Client is the one HTTP client deliveries use: the guarded dialer, no proxy (a
// proxy would dial for us and bypass Control), no redirects (a 3xx is a failure,
// never followed), TLS 1.2 minimum, a 10 s total timeout per attempt.
func Client() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{Proxy: nil, DialContext: Dialer().DialContext,
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, MaxIdleConnsPerHost: 2},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
```

The same file, continued: the payload, the secrets seam and one attempt.

```go
func mustPrefixes(ss ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

// Body encodes the payload. encoding/json sorts map keys, so the same event gives
// the same bytes on every attempt; the signature covers these bytes, not a re-encoding.
func Body(p hubapi.WebhookPayload) ([]byte, error) { return json.Marshal(p) }

// Secrets is where HMAC secrets live. Its implementation is Key Vault under the
// Hub's workload identity, which is a PROPOSED Hub-contract addition (task 145;
// hub-contract:2083-2085 grants no Key Vault access in v1). Until it is accepted,
// serve.go passes nil and every webhook route answers 503 unavailable,
// details.reason "secrets_backend"; the tests use MemorySecrets.
type Secrets interface {
	// New mints 32 random bytes under ref at version, and returns them once.
	New(ctx context.Context, ref string, version int64) ([]byte, error)
	Get(ctx context.Context, ref string, version int64) ([]byte, error)
}

// Attempt is one POST's outcome, as the log records it.
type Attempt struct {
	ResponseCode int  // 0 when no response (refused, timeout, TLS)
	OK           bool // 2xx
}

// Send POSTs body once with the three headers. It never reads more than 4 KiB of
// the response, and records no response header or body: a receiver's error page
// is its own text, not ours to keep.
func Send(ctx context.Context, c *http.Client, url, kind, delivery string, body []byte,
	now time.Time, secrets ...[]byte) Attempt {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Attempt{}
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "PanaceaApp-Hub-Webhooks/1")
	req.Header.Set(hubapi.HeaderWebhookEvent, kind)
	req.Header.Set(hubapi.HeaderWebhookDelivery, delivery)
	req.Header.Set(hubapi.HeaderWebhookSignature, SignatureHeader(now.Unix(), body, secrets...))
	resp, err := c.Do(req)
	if err != nil {
		return Attempt{}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	return Attempt{ResponseCode: resp.StatusCode, OK: resp.StatusCode >= 200 && resp.StatusCode < 300}
}
```

`deliver.go` imports `bytes`, `context`, `crypto/tls`, `encoding/json`, `errors`, `io`, `net`,
`net/http`, `net/netip`, `net/url`, `syscall`, `time` and `hubapi`. `MemorySecrets` (a map behind a
mutex, in `deliver.go` so the server's tests can use it) is the only other exported name.

#### The store (`store/work_webhooks.go`, signatures complete)

The columns this part needs from task 145's migration, stated as a requirement on that part (its
file wins on names; the behaviour is this part's):

```sql
-- work_webhooks (task 145): one row per webhook; RLS on org_id; hub_app SELECT, INSERT, UPDATE.
--   id text PK (wh-), org_id, team_id, project_id NULL, url text, kinds text[] NOT NULL,
--   state text CHECK (state IN ('active','disabled')), created_by text, created_at timestamptz,
--   secret_ref text, secret_version bigint, prev_secret_version bigint NULL,
--   rotated_at timestamptz NULL, cursor_seq bigint NOT NULL, consecutive_failures int NOT NULL DEFAULT 0
-- work_webhook_deliveries (task 145): append-only (hub_app SELECT, INSERT; no UPDATE, no DELETE).
--   id text (wd-, the delivery), org_id, webhook_id, event_seq bigint, attempt int,
--   status text CHECK (status IN ('ok','failed','pending')), response_code int,
--   at timestamptz, next_attempt_at timestamptz NULL,
--   PRIMARY KEY (id, attempt), UNIQUE (webhook_id, event_seq, attempt)
```

`UNIQUE (webhook_id, event_seq, attempt)` is what makes attempt `0` (queued) insertable once per
webhook and event; a second queue of the same event is `ON CONFLICT DO NOTHING`.

```go
package store

// CreateWebhook inserts the row with cursor_seq = the org's audit head (decision 4).
func CreateWebhook(ctx context.Context, tx pgx.Tx, w NewWebhook) (hubapi.Webhook, error)
func ListWebhooks(ctx context.Context, tx pgx.Tx, teamID string) ([]hubapi.Webhook, error)
func LockWebhook(ctx context.Context, tx pgx.Tx, id string) (WebhookRow, error) // FOR UPDATE
func DisableWebhook(ctx context.Context, tx pgx.Tx, id string) (changed bool, err error)
func RotateWebhook(ctx context.Context, tx pgx.Tx, id string, at time.Time) (newVersion int64, err error)
// ActiveWebhooks lists the org's active rows for the janitor's queue step.
func ActiveWebhooks(ctx context.Context, tx pgx.Tx) ([]WebhookRow, error)
// QueueDeliveries appends attempt-0 rows for seqs and sets cursor_seq = next, in the
// caller's transaction; returns how many rows were new.
func QueueDeliveries(ctx context.Context, tx pgx.Tx, webhookID string, seqs []int64, next int64, at time.Time) (int, error)
// DueAttempts: deliveries whose highest-attempt row is 'pending' with
// next_attempt_at <= now, oldest first, at most limit, across active webhooks.
func DueAttempts(ctx context.Context, tx pgx.Tx, now time.Time, limit int) ([]DueAttempt, error)
// RecordAttempt appends the attempt row and updates consecutive_failures (reset on
// ok, +1 otherwise); returns the new count.
func RecordAttempt(ctx context.Context, tx pgx.Tx, d DueAttempt, a Attempt) (failures int, err error)
func ListDeliveries(ctx context.Context, tx pgx.Tx, webhookID string, after string, limit int) (hubapi.Page[hubapi.WebhookDelivery], error)
func LockDelivery(ctx context.Context, tx pgx.Tx, deliveryID string) (DueAttempt, string /*status*/, error)

type NewWebhook struct {
	ID, TeamID, ProjectID, URL, CreatedBy, SecretRef string
	Kinds                                            []string
	At                                               time.Time
}
type WebhookRow struct {
	hubapi.Webhook
	SecretRef           string
	PrevSecretVersion   int64 // 0 when none
	RotatedAt           *time.Time
	CursorSeq           int64
	ConsecutiveFailures int
}
type DueAttempt struct {
	DeliveryID, WebhookID string
	EventSeq              int64
	Attempt               int // the attempt about to be made (last row's attempt + 1)
	Redeliver             bool
}
type Attempt struct {
	N            int
	Status       string // ok | pending | failed
	ResponseCode int
	At           time.Time
	NextAt       *time.Time
}
```

#### The routes (`server/work_webhooks.go`) and their wire

```go
func (s *Server) registerWorkWebhooks(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/teams/{teamId}/webhooks", s.createWebhook)             // team admin
	mux.HandleFunc("GET /v1/teams/{teamId}/webhooks", s.listWebhooks)               // team admin
	mux.HandleFunc("POST /v1/webhooks/{webhookId}/disable", s.disableWebhook)       // team admin
	mux.HandleFunc("POST /v1/webhooks/{webhookId}/rotate", s.rotateWebhook)         // team admin
	mux.HandleFunc("GET /v1/webhooks/{webhookId}/deliveries", s.listDeliveries)     // team admin
	mux.HandleFunc("POST /v1/webhook-deliveries/{deliveryId}/redeliver", s.redeliver) // team admin
}
```

```jsonc
// POST /v1/teams/tm-…/webhooks                              team admin only
{ "url": "https://hooks.example.com/panacea", "projectId": "pj-…", "kinds": ["issue.state", "issue.updated"] }
// → 201 CreatedWebhook; "secret" is 32 bytes, base64url, shown once and never again
{ "webhook": { "id": "wh-…", "teamId": "tm-…", "projectId": "pj-…",
    "url": "https://hooks.example.com/panacea", "kinds": ["issue.state", "issue.updated"],
    "state": "active", "createdBy": "u-…", "secretVersion": 1 },
  "secret": "Q2h…" }
// url not https, with user info, or resolving to a refused address → 400 webhook_url_invalid
// kinds empty, over 64, or naming a kind outside hubapi.WebhookKinds() → 400 bad_request, field kinds
// projectId not a project of the team → 404 project_not_found
// secrets backend absent → 503 unavailable, details.reason "secrets_backend"

// GET /v1/teams/tm-…/webhooks → 200 { "items": [ Webhook … ], "next": null }  (no secret, ever)
// POST /v1/webhooks/wh-…/disable {} → 200 Webhook, state "disabled"; already disabled → 200, no event
// POST /v1/webhooks/wh-…/rotate {}  → 200 CreatedWebhook, secretVersion n+1, the new secret once
// GET /v1/webhooks/wh-…/deliveries?after=&limit=
//   → 200 Page[WebhookDelivery], newest first, one item per attempt row:
{ "items": [ { "id": "wd-…", "webhookId": "wh-…", "eventSeq": 4812, "attempt": 2,
    "status": "pending", "responseCode": 500, "at": "2026-09-26T03:10:00Z",
    "nextAttemptAt": "2026-09-26T03:15:00Z" } ], "next": "…" }
// POST /v1/webhook-deliveries/wd-…/redeliver {} → 202 WebhookDelivery (the new queued row);
//   a delivery still pending → 409 conflict; its webhook disabled → 409 conflict
```

**Authorisation.** Every route is team `admin` of the webhook's team (05 §3.6: a webhook sends the
team's events out, which is the admin's call). A member or viewer is 403 `forbidden`, a non-member
404 (`team_not_found`, or `webhook_not_found` for the `wh-` and `wd-` routes: **not in task 143's
code list**, a gap for that part). No app (task 159) or key (task 160) may reach these routes: task
159's scope table names *"keys and other apps"* and task 160's keys are refused team settings, and
this part adds webhooks to both refusal lists as matrix rows. Rows appended to
`work_matrix_rows_integration_test.go`, one per route: ✓ `A`; 403 `V M`; 404 `O N X`; and 403 for the task 145
fixtures *app for M*, *M's personal key*, *team key*, *integration principal*, *host webhook*.

**Events.** `webhook.created {kindsCount, hasProject}`, `webhook.disabled {reason}` (`by_admin`
from the route), `webhook.rotated {secretVersion}`, each through `emitWork` in the route's
transaction. The delivery log appends no event: an attempt is not a change anyone reads in the
feed, and an event per attempt would feed back into every webhook listening for `webhook.*`.

#### The janitor step (`server/janitor.go` ⚑, step 5, complete as a contract)

```go
// sweepWebhooks is step 5 of Sweep, run only by the replica holding the janitor lock.
// (a) Queue, per org, in that org's transaction (the same set_config and LockHead as
// steps 1-4, H24:84-90), for each ActiveWebhooks row:
//   - the creator's current role in the webhook's team is read; not admin (demoted,
//     removed, or gone from the org) → DisableWebhook, emit webhook.disabled
//     {"creator_not_admin"}, skip;
//   - the visible-events query of H26 runs AS THE CREATOR (task 13's user-id argument),
//     after cursor_seq, at most 500, then filtered to the webhook's kinds and, when set,
//     its projectId; QueueDeliveries appends attempt 0 for each and sets cursor_seq to
//     the query's next (H26: the head when nothing is visible, so no cursor stalls).
// (b) Send, in no transaction: DueAttempts(now, 200); 8 workers; for each:
//   - LockWebhook in a short transaction re-reads state: disabled → nothing is sent;
//   - the event is read by seq and reduced to hubapi.WebhookEvent (Actor from the
//     event's actor columns; before task 167, actor_user_id → {"person", userId}, and
//     null → {"janitor"}); Body; secrets are version, plus prev_secret_version while
//     now < rotated_at + 24 h; webhooks.Send with webhooks.Client();
//   - RecordAttempt: 2xx → ok; else NextAttempt → pending with nextAttemptAt, or failed;
//     a redeliver's failure is failed. failures ≥ DisableAfter → DisableWebhook and
//     emit webhook.disabled {"failures"}, in the same transaction as the attempt row.
// SweepReport gains WebhooksQueued, WebhookAttempts, WebhooksDisabled int.
func sweepWebhooks(ctx context.Context, st *store.Store, cfg JanitorConfig, deps WebhookDeps) (queued, attempts, disabled int, err error)

type WebhookDeps struct {
	Secrets webhooks.Secrets // nil: step 5 is skipped entirely
	Client  *http.Client     // webhooks.Client(); a test passes an httptest TLS client with the same Control
}
```

The *inform* the scope asks for is the `webhook.disabled` event with reason `failures`: it is
feed-visible to the team's readers, and the Desk's `failed` section shows it to the creator through
task 62's row-kind registry. **Registering that row kind is a desktop change this Hub task cannot
make** (task 62's `deskRows`); it is a gap for the W5 desktop tasks (169-173) to own.

The advisory lock is held across (b), so a sweep that spends 8 × 10 s on dead endpoints delays the
next sweep by that much, never overlaps it. With 200 attempts, 8 at a time and 10 s each, a
worst-case send phase is 250 s; `Interval` is 60 s (`H24:72`), so a flood of dead endpoints slows
the janitor's other steps. That is bounded, stated, and why `DisableAfter` exists.

**The test seam.** A test receiver is an `httptest.NewTLSServer` on `127.0.0.1`, which `Allowed`
refuses. So the guard takes its rule as a value, and the defaults close over `Allowed`:

```go
func DialerWith(allow func(netip.Addr) bool) *net.Dialer                               // Dialer() = DialerWith(Allowed)
func ClientWith(allow func(netip.Addr) bool, tlsCfg *tls.Config) *http.Client          // Client() = ClientWith(Allowed, nil)
func CheckURLWith(ctx context.Context, r *net.Resolver, raw string, allow func(netip.Addr) bool) error // CheckURL = …(Allowed)
```

`server.Deps.WebhookAllow` is set only by tests; `TestServeNeverSetsWebhookAllow` greps
`cmd/hub/serve.go` for it and fails on a match.

#### Tests to write first, part 1 — `webhooks/sign_test.go`, `deliver_test.go` (pure)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestSignatureIsOverTheRawBody` | a body with keys in a non-sorted order and extra spaces, signed; `Verify` over those bytes → nil; over `json.Marshal(json.Unmarshal(body))` → `ErrSignatureMismatch`; one byte changed → mismatch | signature over re-encoded JSON |
| `TestSignKnownVector` | secret `"whsec-test"`, `t = 1790000000`, body `{"a":1}` → the hex written literally in the test (computed once with `openssl dgst -sha256 -hmac`) | a changed signing string (`t` and body joined without `.`) |
| `TestVerifyWindowAndRotation` | `t` 301 s old → `ErrSignatureStale`; 299 s → nil; a header with two `v1`, only the second matching → nil; no `t` → malformed | no replay window; only the first `v1` checked |
| `TestAllowedRefusesInwardAddresses` | `10.0.0.5`, `172.16.1.1`, `192.168.0.1`, `127.0.0.1`, `169.254.169.254`, `168.63.129.16`, `100.64.0.1`, `::1`, `fe80::1`, `fd00:ec2::254`, `::ffff:10.0.0.5` → false; `20.190.160.1` → true | SSRF through any one range; IPv4-mapped bypass |
| `TestCheckURLAtCreate` | `https://10.0.0.5/h`, `https://169.254.169.254/latest`, `http://hooks.example.com/`, `https://u:p@hooks.example.com/` → `ErrURLRefused` | plain-http delivery; SSRF at create |
| `TestDialerRefusesAtConnect` | `Dialer().DialContext(ctx, "tcp", "10.0.0.5:443")` and `"169.254.169.254:80"` → an error wrapping `ErrURLRefused`, with no packet sent (Control runs before connect) | DNS rebinding: a check at create only |
| `TestClientDoesNotFollowRedirects` | a TLS test server answering 302 to a second server; `Send` → `ResponseCode 302`, `OK false`; the second server saw nothing | a redirect into an inward address |
| `TestNextAttemptSchedule` | n = 1..5 → +1 m, +5 m, +30 m, +2 h, +6 h; n = 6 → false | no retry; retry forever |
| `TestWebhookKindsExcludeSubjectOnlyAndText` | `WebhookKinds()` holds no `SubjectOnlyKinds()` entry and not `release.abandoned`; holds `issue.updated` and `release.confirmed` | private kinds sent out; a text-bearing Hub kind |
| `TestIssueUpdatedDataIsFlat` | task 13's scalar test re-run over the widened struct passes; every `…Prev` has its pair | a non-scalar value; a missing pair |

#### Tests to write first, part 2 — `server/work_webhooks_integration_test.go` (`//go:build integration`)

Each seeds an org, team `tm-a` with admin Ada, member Linus, a project, and a TLS receiver that
records every request's raw body and headers. The janitor's `Sweep` is called directly, with a
fake clock.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestNoTextInAnyDelivery` | a canary written into an issue's title and description, a comment, a request quote (task 146) and a label name, through their routes; a webhook on every kind; sweeps until quiet → no body contains the canary, and no body contains Ada's email or display name | text in the payload; the feed's `User` actor sent out |
| `TestReceiverVerifiesTheSignature` | the receiver runs `webhooks.Verify` with the secret from create → nil for every delivery; `Panacea-Event` equals `event.kind`; `Panacea-Delivery` equals `delivery` | a header that does not match the body |
| `TestPriorityChangeCarriesThePreviousValue` | patch priority `low` → `high` → the delivered `data` has `fields` naming priority, `priority: "high"`, `priorityPrev: "low"`; a title change delivers `fields` naming title and no value of it | no previous values; a title valued |
| `TestRetriesThenFails` | the receiver answers 500; sweeps at +0, +1 m, +6 m, +36 m, +2 h 36 m, +8 h 36 m → six attempt rows, statuses five `pending` then `failed`; a sweep at +1 day sends nothing | no retry; retry forever |
| `TestTwentyFailuresDisableAndInform` | 20 events, receiver 500 → after the 20th failed attempt the webhook is `disabled`, one `webhook.disabled {reason: "failures"}` in Ada's `/v1/changes`; the next sweep sends nothing | a dead endpoint drained forever |
| `TestSSRFRefusedAtCreateAndDelivery` | create with `https://10.0.0.5/` → 400 `webhook_url_invalid`; a row whose `url` is set, as the table owner, to `https://169.254.169.254/` (standing for a name that has since rebound inward; the create check passed earlier) → the attempt records `responseCode 0`, `pending`, and the test's listener on that path saw no connection | SSRF |
| `TestUnreadableProjectNeverDelivered` | a team webhook with no `projectId`; an event in a project of another team Ada is not in → never queued | readability skipped |
| `TestCreatorDemotedDisables` | Ada demoted to member → the next sweep disables with `creator_not_admin`, sends nothing | a webhook outliving its admin |
| `TestTwoReplicasDeliverOnce` | two `Sweep` calls on two pools at once → each event's delivery has exactly one attempt-1 row, and the receiver saw it once | double delivery |
| `TestRotationOverlap` | rotate → the next delivery's header has two `v1`, the old secret verifies; +25 h → one `v1`, the old secret fails | a rotation that breaks receivers, or keeps the old secret for ever |
| `TestWebhookRoutesMatrix` | the six routes' rows, including 403 for every task 145 fixture (app, keys, integration, host) | a missing role row |

#### Steps

- [ ] **Step 1: Check the base.** Hub tasks 24 and 26 merged (`Sweep`'s per-org loop, the visible
  query and its user-id argument from task 13); task 145's two tables carry the columns above, or
  its part is amended first; task 143's `Webhook`, `WebhookDelivery`, `CreatedWebhook` and `Actor`
  as fixed. If a Hub file moved, follow the move (rule H1).
- [ ] **Step 2: The pure tests** (part 1), then `sign.go`, `deliver.go` and the `workevents.go`
  additions, until green. Compute `TestSignKnownVector`'s hex once with
  `printf '%s' '1790000000.{"a":1}' | openssl dgst -sha256 -hmac whsec-test` and paste it.
- [ ] **Step 3: The integration tests** (part 2); they fail. Then the store, the routes, the
  matrix rows, task 08's patch path filling the new data fields, and janitor step 5, until green.
- [ ] **Step 4: Run the neighbours.** Task 13's canary and registry tests, task 08's patch tests,
  Hub task 24's janitor tests and the matrix meta-test: all still pass.
- [ ] **Step 5: Commit by pathspec** the files above, one commit, with the message naming SW.22.

#### Acceptance

```bash
go test ./control-plane/internal/hub/webhooks/ -race -count=1 -v 2>&1 | grep -c '^--- PASS'   # 8
go test ./control-plane/internal/hub/hubapi/ -race -count=1 -run '^Test(WebhookKindsExcludeSubjectOnlyAndText|IssueUpdatedDataIsFlat|EveryWorkDataFieldIsAScalar)$' -v 2>&1 | grep -c '^--- PASS'   # 3
export PANACEA_APP_HUB_TEST_DATABASE_URL='postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable'
go test ./control-plane/internal/hub/server/ -tags integration -race -count=1 -v \
  -run '^Test(NoTextInAnyDelivery|ReceiverVerifiesTheSignature|PriorityChangeCarriesThePreviousValue|RetriesThenFails|TwentyFailuresDisableAndInform|SSRFRefusedAtCreateAndDelivery|UnreadableProjectNeverDelivered|CreatorDemotedDisables|TwoReplicasDeliverOnce|RotationOverlap|WebhookRoutesMatrix|ServeNeverSetsWebhookAllow)$' \
  2>&1 | grep -c '^--- PASS'                                                                   # 12
go test ./control-plane/internal/hub/... -tags integration -race -count=1                     # every package ok
grep -rn "WebhookAllow" control-plane/cmd/                                                     # no output
```

Expected: `8`, `3`, `12`, every package `ok`, and no output from the last `grep` (exit 1).
`TestEveryWorkDataFieldIsAScalar` is task 13's test, re-run here over the widened struct.

#### What this task does NOT claim

- **Not that the Hub may open these connections.** The contract gives the Hub process outbound
  HTTPS to `login.microsoftonline.com` and nothing else (`hub-contract:2083-2085`, `:2211`), and no
  Key Vault. Outbound delivery and the secret store are part of the proposed Hub-contract addition
  task 145 names; until the `feat/hub` owner accepts it, `Deps.WebhookSecrets` is nil in `serve.go`
  and the routes answer 503. Nothing here is live in any environment.
- **Not that the retry schedule suits every receiver**, nor exactly-once delivery (decision 1).
- **Not ordering.** A retried delivery may arrive after a later event's; receivers order by `seq`.
- **Not the Desk row** for a disabled webhook (a desktop change; see the gaps).
- **Not measured throughput.** The 200-attempt, 8-worker bound is a choice; nobody has timed it.
- **Not that egress filtering is complete.** The guard refuses the listed ranges at connect; a
  network policy on the Hub's pod (phase 8's) is the second defence and is **UNVERIFIED**.

**Parity rows:** closes **L-251**; serves **L-148** (release events delivered out) and **L-154**
(task 178's connectors are receivers of SW.22).

#### Gaps found while writing this part

1. The File map has no row for `hubapi/webhookpayload.go`, and its `server/janitor.go` row lists
   77, 121 and 162 but not 156 (step 5) or 157 (the Teams post step).
2. Task 143's code list has no `webhook_not_found`; this part uses it for the `wh-` and `wd-` routes.
3. Task 145 must carry the columns in *The store*; its scope names only `secret_ref` and version.
4. Task 144's fake rule *"kinds lists only SW.5 kinds"* is widened by four `release.*` kinds;
   the fake should call `hubapi.WebhookKinds()`.
5. The Desk row kind for `webhook.disabled` has no owning task; it belongs with 169-173.
6. Hub task 14's handling of a plain `error` from `Write`'s `fn` (500) is **UNVERIFIED**, as task
   13 noted; `emitWork` in these routes relies on it.
