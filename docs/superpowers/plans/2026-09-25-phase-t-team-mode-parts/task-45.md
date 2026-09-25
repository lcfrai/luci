### Task 45: Hub delta: audit to Sentinel; the anchored chain head

**Slice TM1. Depends on: Hub task 27 (and reads Hub tasks 14, 20, 24; phase 8 tasks 04, 10; Phase
T task 44). About 2.5 hours. Seams: none new.**

The Hub's audit is a SHA-256 hash chain in its own Postgres (Hub task 14's `store.Append`, Hub
`task-14.md:140-178`), and `hub verify` walks it (Hub `task-27.md:48-61`). That catches one edited
row. It does not catch an owner who rewrites **every** row after a point, consistently: the chain
still links, and Hub task 27 says so (`task-27.md:115-116`). ADR-H1 promised the fix, "hash-chained
audit streamed to Log Analytics and Sentinel" (`ADR-H1:162-163`), and deferred the anchor to later
(`ADR-H1` §5.9 item 4, `:587-588`: "the chain head exported to Log Analytics hourly … phase 8's
DCR"). The team-mode design brings that later forward: audit leaves the building **before team mode
reaches anyone but the owner** (`05-design.md` §11.1, `:1132`), and the skeleton makes it part of
the gate before the first real teammate (skeleton `:1029`).

This task builds three things:

1. **The stream.** Every audit event, within about 15 seconds of its commit, is sent to the Log
   Analytics workspace of phase 8 task 04 through the Logs Ingestion API, as a metadata record:
   its org, `seq`, time, kind, team, project, subject, actor, `hash` and `prevHash`, and the
   **names** of its data keys, never their string values unless the pair is on a short enum list.
2. **The anchor.** Every hour, for each org, the chain head (`audit_heads.seq`, `hash`) goes to a
   second table. Sentinel is enabled on the workspace, and one scheduled rule fires when an org
   with events has no anchor for two hours, because a Hub that stops anchoring is the attack.
3. **The comparison.** `hub verify` gains an `anchor` check: every anchored head and every streamed
   event hash must equal the database's row at that `seq`, and the head must not be behind the
   newest anchor. A consistent rewrite that passes the `audit` check fails this one.

It is a delta on the Hub branch, built after the sync-merge of `feat/hub`, plus a small delta on
phase 8's `modules/observability` (task 04) and workload identities (task 10), after the
sync-merge of `feat/env`. Every Hub and phase 8 path below is **planned**, so each is **UNVERIFIED
until merge**.

**The one thing this task must not get wrong is that the stream carries no text.** Log Analytics
is a second store of the audit, readable by anyone with workspace read, and its retention is not
the Hub's erasure (Hub task 27, `:118-129`). Contract §8's `data` holds team and project names,
invite emails, branch refs and two free-text `reason`s (`brief.erased`, `release.abandoned`)
(Hub contract `:1137-1167`). So the mapping is an allowlist: a string value leaves only when its
`(kind, key)` pair is in `StreamEnums` **and** the value matches the enum grammar; integers and
booleans leave; everything else leaves as its key's name alone. `canonical` never leaves.

**The second thing is that the Hub cannot rewrite what it has sent.** The Hub's identity holds
`Monitoring Metrics Publisher` on the collection rule (send only) and `Log Analytics Reader` on the
workspace (read only), and nothing that deletes or purges. The phase 8 test asserts that set
exactly, so a later role added to `id-hub` fails the plan.

**Decisions this part makes.**

- **A cursor, not an in-transaction send.** A write must not wait on, or fail with, Azure Monitor;
  the streamer reads committed rows after a per-org cursor and advances it after a 2xx. A crash
  between the send and the cursor update sends a batch twice; records carry `(orgId, seq)`, and
  the check de-duplicates on it. The window an owner could rewrite an unsent row in is the stream
  lag, which the `anchor` check reports.
- **One replica streams and anchors,** by Hub task 24's advisory-lock shape (Hub `task-24.md:80-90`),
  with lock names `panacea-app-hub-audit-stream` and `panacea-app-hub-audit-anchor`.
- **Anchors are sent every hour even when the head has not moved,** so their absence is a signal.
- **Streaming is off unless configured**: `PANACEA_APP_HUB_AUDIT_SINK` is `off` (local, the
  default), `fake` (tests, dev) or `logs`. While it is not `logs`, `Server.TeamModeAllowed` answers
  503 `unavailable`, `details.reason = "audit_not_streaming"`, and the write that sets a project's
  `mode` to `team` calls it first. That is how "part of the gate before the first real teammate" is
  enforced in code. No part owns that write yet (`task-06.md:699-701`); Gap 1 below.

**Files:**
- `control-plane/internal/hub/auditsink/record.go` + `record_test.go` — `EventRecord`,
  `AnchorRecord`, `FromRow`, `StreamEnums` (new)
- `control-plane/internal/hub/auditsink/sink.go` — `Sink`, `Anchors`, `Fake` (new)
- `control-plane/internal/hub/auditsink/logs.go` + `logs_test.go` — the Logs Ingestion sender and
  the Log Analytics query reader, both over `httptest` in the tests (new)
- `control-plane/internal/hub/store/auditstream.go` — `StreamCursor`, `AdvanceStream`,
  `EventsAfter`, `Heads` (new)
- `control-plane/internal/hub/migrate/sql/00NN_audit_stream.sql` — `audit_stream_cursors` (new;
  the number is the next free one after the sync-merge)
- `control-plane/internal/hub/server/auditstream.go` — `RunAuditStream`, `RunAuditAnchor`,
  `StreamOnce`, `AnchorOnce`, `TeamModeAllowed` (new)
- `control-plane/internal/hub/server/auditstream_integration_test.go` (new)
- `control-plane/cmd/hub/{config.go,serve.go}` — the sink settings; start both loops (modify)
- `control-plane/cmd/hub/verify.go` + `verify_integration_test.go` — the `anchor` check, after
  Hub task 27's `audit` (modify)
- `infra/terraform/modules/observability/hubaudit.tf` + `tests/hubaudit.tftest.hcl` — the DCE,
  two tables, the DCR, Sentinel onboarding, the missing-anchor rule (new; phase 8 task 04's module)
- `infra/terraform/modules/observability/{variables,outputs}.tf` — `hub_principal_id`; the DCE and
  DCR outputs (modify)
- `infra/terraform/role-allowlist.txt` — `Monitoring Metrics Publisher`, `Log Analytics Reader`
  (modify; phase 8 task 10's file, `task-10.md:39`)
- `docs/superpowers/contracts/2026-09-23-hub-contract.md` — §11 T5's residual, §12's settings,
  §15's line (modify)

**Interfaces:**
- Consumes: Hub task 14's `store.Head`, `LockHead`, `AuditRow`, `InFeed` and the chain rule
  (Hub `task-14.md:140-178`); Hub task 27's `audit` verify check and its org walk by
  `hub.org_ids()` (Hub `task-27.md:48-61`); Hub task 24's advisory-lock loop; Hub task 08's verify
  runner (`ok <name>` / `FAULT <name>: <what>`, Hub `task-08.md:264-266`); `hubapi.AuditEnvelope`
  and `ParseAuditEnvelope` and `ChainHash` (real: `hubapi/envelope.go:266`, `:337`, `:380` at
  `9f2fa31`); phase 8 task 04's
  workspace and task 10's `id-hub`.
- Produces: `auditsink.Sink`, `auditsink.Anchors`, `auditsink.Fake`, `auditsink.EventRecord`,
  `auditsink.AnchorRecord`, `auditsink.FromRow`; `server.StreamOnce`, `server.AnchorOnce`,
  `server.TeamModeAllowed`; the verify check `anchor`; the Terraform outputs
  `hub_audit_dce_endpoint`, `hub_audit_dcr_immutable_id`.
- Required by: task 57 (the TM1 gate names "audit streams to Sentinel"), task 46 (an erasure
  tombstone is streamed as metadata like any event), task 44 (its `delegation.revoked` reasons
  leave as enums).

#### The records — complete source

```go
// control-plane/internal/hub/auditsink/record.go
package auditsink

// EventRecord is one audit event as it leaves the Hub: the row's columns and
// hashes, the NAMES of its data keys, and only the data values that cannot be
// text. Field names are the custom table's columns (hubaudit.tf).
type EventRecord struct {
	TimeGenerated string           `json:"TimeGenerated"` // the event's at, RFC 3339 ms
	OrgID         string           `json:"OrgId"`
	Seq           int64            `json:"Seq"`
	Kind          string           `json:"Kind"`
	Feed          bool             `json:"Feed"`
	TeamID        string           `json:"TeamId"`
	ProjectID     string           `json:"ProjectId"`
	Subject       string           `json:"Subject"`
	ActorUserID   string           `json:"ActorUserId"`
	Hash          string           `json:"Hash"`     // 64 lowercase hex
	PrevHash      string           `json:"PrevHash"` // 64 lowercase hex
	DataKeys      []string         `json:"DataKeys"` // sorted, every key, values or not
	DataValues    map[string]any   `json:"DataValues"` // int64 | bool | an allowed enum string
	Dropped       int              `json:"Dropped"`  // string values withheld
}

// AnchorRecord is one org's chain head at one moment.
type AnchorRecord struct {
	TimeGenerated string `json:"TimeGenerated"` // when it was read
	OrgID         string `json:"OrgId"`
	Seq           int64  `json:"Seq"`  // audit_heads.seq; 0 for an org with no events
	Hash          string `json:"Hash"` // audit_heads.hash, 64 lowercase hex
	StreamedSeq   int64  `json:"StreamedSeq"` // the stream cursor at the same moment
	Replica       string `json:"Replica"`     // the pod name, for the missing-anchor rule
}

// StreamEnums is every (kind, key) whose string value may leave. Each value is
// an enum or a GUID in the contract (§8, Phase T task 01's rows). Anything not
// here leaves as its key's name only. Adding a pair needs the contract row
// that makes its value an enum, cited in the comment beside it.
var StreamEnums = map[string][]string{
	"team.member_set":       {"role"},                               // §8: {"role"}
	"team.invite_bound":     {"role"},                               // §8: {"userId","role"}
	"brief.revised":         {"status"},                             // §8: {…,"status"}
	"brief.created":         {"scope"},                              // §8: {"scope","branchRef"}
	"release.reserved":      {"kind", "environment", "pinStatus"},   // §8
	"evidence.recorded":     {"verdict"},                            // §8
	"auth.user_provisioned": {"tenantId"},                           // §8: a GUID
	"auth.forbidden":        {"required"},                           // §8: a role name
	"delegation.revoked":    {"reason"},                             // task 01: RevokeReason, task-03.md:110-115
}

// enumValue is the grammar an allowed string must also match: a lowercase
// word or words with "_" or "-", at most 40, or a GUID. A value outside it is
// withheld and counted, so a contract change that puts text in an "enum" key
// leaks nothing and shows up as Dropped > 0.
var enumValue = regexp.MustCompile(`^([a-z][a-z0-9_-]{0,39}|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$`)

// FromRow maps one committed audit row. It parses canonical with
// hubapi.ParseAuditEnvelope only to read Data; the envelope's text never
// reaches the record. hash and prevHash are the row's columns.
func FromRow(canonical []byte, hash, prevHash [32]byte, feed bool) (EventRecord, error) {
	e, err := hubapi.ParseAuditEnvelope(canonical)
	if err != nil {
		return EventRecord{}, err
	}
	r := EventRecord{
		TimeGenerated: e.At, OrgID: e.OrgID, Seq: e.Seq, Kind: e.Event, Feed: feed,
		TeamID: e.TeamID, ProjectID: e.ProjectID, Subject: e.Subject, ActorUserID: e.ActorUserID,
		Hash: hex.EncodeToString(hash[:]), PrevHash: hex.EncodeToString(prevHash[:]),
		DataKeys: slices.Sorted(maps.Keys(e.Data)), DataValues: map[string]any{},
	}
	allowed := StreamEnums[e.Event]
	for k, v := range e.Data {
		switch v := v.(type) {
		case int64, bool:
			r.DataValues[k] = v
		case string:
			if slices.Contains(allowed, k) && enumValue.MatchString(v) {
				r.DataValues[k] = v
			} else {
				r.Dropped++
			}
		default:
			r.Dropped++
		}
	}
	return r, nil
}
```

`Subject` leaves as it is: contract §8 makes it an id, a route pattern or a `sha256:` digest for
every kind (Hub contract `:1137-1163`), never text. `TestEverySubjectIsAnIdentifier` below holds
that against every kind a test run emits.

```go
// control-plane/internal/hub/auditsink/sink.go
package auditsink

// Sink sends records to one stream. It returns nil only on a 2xx for the
// whole batch; the caller does not advance its cursor otherwise.
type Sink interface {
	SendEvents(ctx context.Context, recs []EventRecord) error
	SendAnchors(ctx context.Context, recs []AnchorRecord) error
}

// Anchors reads back what was sent, for hub verify. since bounds the window
// to the workspace's retention.
type Anchors interface {
	AnchorsOf(ctx context.Context, orgID string, since time.Time) ([]AnchorRecord, error)
	EventsOf(ctx context.Context, orgID string, since time.Time) ([]EventRecord, error)
}

// Fake keeps everything in memory, is both a Sink and Anchors, and can fail
// on demand. Sent records are stored as their JSON, so a test can search the
// exact bytes that would have left the Hub.
type Fake struct {
	mu      sync.Mutex
	Events  []EventRecord
	Anchs   []AnchorRecord
	Wire    [][]byte // every batch body as marshalled
	FailN   int      // the next FailN sends return an error
}

func NewFake() *Fake
```

```go
// control-plane/internal/hub/auditsink/logs.go
package auditsink

// Logs is the real Sink and Anchors. It holds no secret: Token is the Hub's
// workload-identity exchange (phase 8 task 10's id-hub, the same exchange the
// database password uses, phase 8 plan :268), asked for two scopes.
//
// Send (the Logs Ingestion API, from Microsoft's documentation as known at
// the knowledge cutoff, UNVERIFIED against the tenant until task 57's run):
//   POST {DCE}/dataCollectionRules/{DCR}/streams/{stream}?api-version=2023-01-01
//   Authorization: Bearer <token for https://monitor.azure.com/.default>
//   Content-Type: application/json; body = a JSON array of records
// A batch is at most 500 records and 1 MB of JSON (the API's per-call limit,
// UNVERIFIED); a larger slice is split. 204 is success; 429 and 5xx are
// errors the loop retries next round; 401/403 are logged at ERROR.
//
// Read (the Log Analytics query API, same status):
//   POST https://api.loganalytics.io/v1/workspaces/{Workspace}/query
//   Authorization: Bearer <token for https://api.loganalytics.io/.default>
//   body {"query": <KQL>, "timespan": "<since>/<now>"}
// with KQL whose only spliced value is the org id, refused unless
// hubapi.ValidID(hubapi.PrefixOrg, orgID) holds (org- and hex, no quote):
//   PanaceaHubAuditAnchor_CL | where OrgId == '<org>' | order by TimeGenerated asc
//   PanaceaHubAudit_CL       | where OrgId == '<org>' | summarize arg_max(TimeGenerated, *) by Seq
type Logs struct {
	HTTP      *http.Client // timeout 10 s
	Token     func(ctx context.Context, scope string) (string, error)
	DCE       string // https://<dce>.australiaeast-1.ingest.monitor.azure.com
	DCR       string // the rule's immutable id, dcr-<32 hex>
	Workspace string // the workspace's customer id (a GUID)
}

const (
	StreamEvents  = "Custom-PanaceaHubAudit_CL"
	StreamAnchors = "Custom-PanaceaHubAuditAnchor_CL"
)

func (l Logs) SendEvents(ctx context.Context, recs []EventRecord) error
func (l Logs) SendAnchors(ctx context.Context, recs []AnchorRecord) error
func (l Logs) AnchorsOf(ctx context.Context, orgID string, since time.Time) ([]AnchorRecord, error)
func (l Logs) EventsOf(ctx context.Context, orgID string, since time.Time) ([]EventRecord, error)
```

#### The cursor — complete source

```sql
-- control-plane/internal/hub/migrate/sql/00NN_audit_stream.sql
-- How far each org's audit has been sent. Not audited itself: it records
-- delivery, not a change anyone made, and advancing it must not move the head.
CREATE TABLE audit_stream_cursors (
  org_id       text        PRIMARY KEY REFERENCES orgs (id),
  streamed_seq bigint      NOT NULL DEFAULT 0 CHECK (streamed_seq >= 0),
  anchored_at  timestamptz,
  updated_at   timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE audit_stream_cursors ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_stream_cursors FORCE ROW LEVEL SECURITY;
CREATE POLICY audit_stream_cursors_org ON audit_stream_cursors
  USING (org_id = current_setting('hub.org_id', true))
  WITH CHECK (org_id = current_setting('hub.org_id', true));
GRANT SELECT, INSERT, UPDATE ON audit_stream_cursors TO hub_app;
-- Forward only: a cursor moved back would re-send, harmless; one moved past
-- the head would skip events, which this refuses.
CREATE FUNCTION hub.audit_stream_forward() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.streamed_seq < OLD.streamed_seq THEN
    RAISE EXCEPTION 'audit stream cursor moves forward only' USING ERRCODE = '23514';
  END IF;
  IF NEW.streamed_seq > (SELECT seq FROM audit_heads WHERE org_id = NEW.org_id) THEN
    RAISE EXCEPTION 'audit stream cursor past the head' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER audit_stream_forward BEFORE UPDATE ON audit_stream_cursors
  FOR EACH ROW EXECUTE FUNCTION hub.audit_stream_forward();
```

The trigger's `SELECT` on `audit_heads` runs as `hub_app`, which holds `SELECT` on it (Hub task
14's `LockHead` reads it through `hub.lock_audit_head()`; that `hub_app` may `SELECT` it directly
is **UNVERIFIED until merge**; if not, the function is `SECURITY DEFINER` owned by the migrate role,
as task 07b's functions are).
