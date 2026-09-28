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
  enforced in code. The write is task 06's route 87 (fix batch b01, X-C1), and this task fills
  its seam: `teamModeGate`'s body becomes `return s.TeamModeAllowed()`. Task 06's dev-only
  `PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED` survives only beside the `fake` sink: with `off` it no
  longer opens the gate, and `loadConfig` still refuses it outside `local` and `dev`.
  Every harness that turns a project team already sets **both**
  `PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED=true` and `PANACEA_APP_HUB_AUDIT_SINK=fake`. The full
  list, and it is closed — a seventh must add itself here: task 06's `teamWorld`, task 08's
  real-Hub conformance `newEnv`, task 43's Z5 harness, task 33's cluster Hub, **task 44's leaver
  world** (`task-44.md:310-321`) and **task 14's `TestFleetCannotRecordAPolicyVersion`**, which
  reuses `teamWorld` (`task-14.md:930-936`, the row itself at `:965`); the last two are fix batch b01 round 3, re-review
  I-A, the rest round 2's I-4. So the swap leaves them green. This task's Step 5 proves it: after
  the swap it runs `go test -tags integration ./internal/hub/...`, which is every one of those Go
  suites, and they pass. Task 33's cluster Hub is not a Go suite; its `05-team-mode.sh` sets the
  same two variables (`task-33.md:79`).

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
- `control-plane/internal/hub/server/projectmode.go` — `teamModeGate`'s body becomes
  `return s.TeamModeAllowed()` (modify: one line; task 06's route 87, fix batch b01)
- `control-plane/cmd/hub/{config.go,serve.go}` — the sink settings; start both loops (modify)
- `control-plane/cmd/hub/verify.go` + `verify_integration_test.go` — the `anchor` check, after
  Hub task 27's `audit` (modify)
- `infra/terraform/modules/observability/hubaudit.tf` + `tests/hubaudit.tftest.hcl` — the DCE,
  two tables, the DCR, Sentinel onboarding, the missing-anchor rule (new; phase 8 task 04's module)
- `infra/terraform/modules/observability/{versions,outputs}.tf` — the `azapi` provider; the DCE,
  DCR and workspace outputs (modify)
- `infra/terraform/stacks/workloads/main.tf` + `tests/workloads.tftest.hcl` — `id-hub`'s
  `role_assignments` gain two entries, and a test pins its set (modify; phase 8 task 10's files)
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
	"delegation.revoked":    {"reason"},                             // task 01: RevokeReason, task-03.md:117-122
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

```go
// control-plane/internal/hub/store/auditstream.go
package store

// StreamCursor reads (inserting at 0 if absent) the session org's cursor.
func StreamCursor(ctx context.Context, tx pgx.Tx, orgID string) (streamed int64, anchoredAt *time.Time, err error)

// AdvanceStream sets streamed_seq = to (forward only; the trigger refuses
// anything else) and updated_at = now.
func AdvanceStream(ctx context.Context, tx pgx.Tx, orgID string, to int64, now time.Time) error

// MarkAnchored sets anchored_at = now.
func MarkAnchored(ctx context.Context, tx pgx.Tx, orgID string, now time.Time) error

// StreamRow is what the streamer reads: never more than the columns FromRow needs.
type StreamRow struct {
	Seq            int64
	Canonical      []byte
	Hash, PrevHash [32]byte
	Feed           bool
}

// EventsAfter reads rows seq > after, ascending, at most limit (≤ 500).
func EventsAfter(ctx context.Context, tx pgx.Tx, after int64, limit int) ([]StreamRow, error)

// HeadOf reads the session org's audit_heads row without locking it.
func HeadOf(ctx context.Context, tx pgx.Tx) (Head, error)

// RowHash reads one row's hash; ok false when no row has that seq.
func RowHash(ctx context.Context, tx pgx.Tx, seq int64) (hash [32]byte, ok bool, err error)
```

#### The loops — complete source

```go
// control-plane/internal/hub/server/auditstream.go
package server

// StreamConfig: Every 15 s; Anchor 1 h; Batch 500.
type StreamConfig struct {
	Every, Anchor time.Duration
	Batch         int
	Replica       string // the pod name
	Now           func() time.Time
	Log           *slog.Logger
}

// RunAuditStream and RunAuditAnchor are Hub task 24's loop shape, one lock
// each, so a slow Azure call in one does not delay the other. Neither emits an
// audit event and neither takes the audit head lock: reading committed rows
// and advancing a delivery cursor changes no record (contract §8's rule is
// one event per changed record).
func RunAuditStream(ctx context.Context, s *Server, sink auditsink.Sink, c StreamConfig) error
func RunAuditAnchor(ctx context.Context, s *Server, sink auditsink.Sink, c StreamConfig) error

// StreamOnce, for each org of hub.org_ids(): in one transaction with
// set_config('hub.org_id', org, true), read the cursor and EventsAfter(cursor,
// Batch); map each row with auditsink.FromRow; COMMIT (the read transaction
// ends before the network call); sink.SendEvents; on nil, a second short
// transaction AdvanceStream(to = the last seq sent). A mapping error is a
// FAULT logged with org and seq, and the org stops streaming until hub verify
// is run, because an unparseable row is itself evidence of an edit. It
// returns the number of records sent this round.
func (s *Server) StreamOnce(ctx context.Context, sink auditsink.Sink, c StreamConfig) (sent int, err error)

// AnchorOnce, for each org: read HeadOf and the cursor in one transaction;
// sink.SendAnchors({Seq, Hash, StreamedSeq, Replica, TimeGenerated: now});
// on nil, MarkAnchored. Orgs are batched into one send per round.
func (s *Server) AnchorOnce(ctx context.Context, sink auditsink.Sink, c StreamConfig) (anchored int, err error)

// TeamModeAllowed is nil when the sink is "logs", or, in local and dev only,
// when task 06's TeamModeUnstreamed flag stands beside the "fake" sink (the
// flag is a start-up error anywhere else). Route 87, the one write that sets a
// project's mode to team, calls it first, through teamModeGate (task 06, b01).
func (s *Server) TeamModeAllowed() error {
	if s.cfg.AuditSink == AuditSinkLogs {
		return nil
	}
	if s.cfg.TeamModeUnstreamed && s.cfg.AuditSink == AuditSinkFake {
		return nil
	}
	return Refuse(http.StatusServiceUnavailable, hubapi.CodeUnavailable,
		"Team projects need the audit to leave the Hub first; it is not configured.",
		map[string]string{"reason": "audit_not_streaming"})
}
```

Each round logs one line per org, `audit stream org=<id> sent=<n> lag_s=<now minus the oldest
unsent event's at, 0 when none>`, and each anchor `audit anchor org=<id> seq=<n>`. The Hub has no
metrics endpoint on its branch (none found in Hub tasks or the contract at `9f2fa31`), so these
lines, collected by phase 8 task 04's Container Insights, are what an alert reads.

#### The `anchor` check

Registered in `cmd/hub/verify.go` after Hub task 27's `audit`, so the in-database chain is known to
link before it is compared with the outside. It needs `auditsink.Anchors` (the `logs` reader in
the cluster, the `Fake` in tests) and `PANACEA_APP_HUB_VERIFY_SINCE` (default 30 days, the
workspace's dev retention, phase 8 `task-04.md:37-44`). For each org of `hub.org_ids()`:

1. **A1:** every anchor's `Hash` equals `RowHash(anchor.Seq)`; an anchor with `Seq` 0 needs no row.
2. **A2:** `HeadOf.Seq` ≥ the largest anchored `Seq` (a truncated chain).
3. **A3:** anchors in time order have non-decreasing `Seq` (a head that went back).
4. **A4:** every streamed event (de-duplicated on `Seq`) with `Seq` ≤ head has `Hash` equal to the
   row's, and `PrevHash` equal to the row's `prev_hash`.
5. **A5:** at least one anchor exists for an org whose head `Seq` > 0 and whose first event is older
   than two anchor periods (a Hub that never anchored).

The first fault prints `FAULT anchor: org <id> seq <n>: <rule>` and exits 1; a healthy run prints
`ok anchor (<a> anchors, <e> events in <m> orgs)`.

#### The phase 8 half — complete source

```hcl
# infra/terraform/modules/observability/hubaudit.tf
# The Hub's audit, outside the Hub. Two custom tables, one endpoint, one rule
# with two streams, Sentinel on the workspace, and the one detection whose
# absence would hide an attack: an org that stopped anchoring.

locals {
  hub_audit_columns = [
    { name = "TimeGenerated", type = "datetime" }, { name = "OrgId", type = "string" },
    { name = "Seq", type = "long" }, { name = "Kind", type = "string" },
    { name = "Feed", type = "boolean" }, { name = "TeamId", type = "string" },
    { name = "ProjectId", type = "string" }, { name = "Subject", type = "string" },
    { name = "ActorUserId", type = "string" }, { name = "Hash", type = "string" },
    { name = "PrevHash", type = "string" }, { name = "DataKeys", type = "dynamic" },
    { name = "DataValues", type = "dynamic" }, { name = "Dropped", type = "int" },
  ]
  hub_anchor_columns = [
    { name = "TimeGenerated", type = "datetime" }, { name = "OrgId", type = "string" },
    { name = "Seq", type = "long" }, { name = "Hash", type = "string" },
    { name = "StreamedSeq", type = "long" }, { name = "Replica", type = "string" },
  ]
  hub_tables = { PanaceaHubAudit_CL = local.hub_audit_columns, PanaceaHubAuditAnchor_CL = local.hub_anchor_columns }
}

# azurerm 4.81.0 manages an existing table's plan and retention, not a custom
# table's schema (from knowledge, UNVERIFIED), so the tables are azapi.
resource "azapi_resource" "hub_table" {
  for_each  = local.hub_tables
  type      = "Microsoft.OperationalInsights/workspaces/tables@2022-10-01"
  name      = each.key
  parent_id = azurerm_log_analytics_workspace.this.id
  body = {
    properties = {
      schema          = { name = each.key, columns = each.value }
      retentionInDays = var.retention_days
    }
  }
}

resource "azurerm_monitor_data_collection_endpoint" "hub_audit" {
  name                          = "dce-hub-audit-panacea-app-${var.environment}"
  resource_group_name           = var.resource_group_name
  location                      = var.location
  public_network_access_enabled = true # the Hub egresses to it; a private link is later (Gap 4)
}

resource "azurerm_monitor_data_collection_rule" "hub_audit" {
  name                        = "dcr-hub-audit-panacea-app-${var.environment}"
  resource_group_name         = var.resource_group_name
  location                    = var.location
  data_collection_endpoint_id = azurerm_monitor_data_collection_endpoint.hub_audit.id
  destinations {
    log_analytics {
      name                  = "la"
      workspace_resource_id = azurerm_log_analytics_workspace.this.id
    }
  }
  dynamic "stream_declaration" {
    for_each = local.hub_tables
    content {
      stream_name = "Custom-${stream_declaration.key}"
      dynamic "column" {
        for_each = stream_declaration.value
        content {
          name = column.value.name
          type = column.value.type
        }
      }
    }
  }
  dynamic "data_flow" {
    for_each = local.hub_tables
    content {
      streams       = ["Custom-${data_flow.key}"]
      destinations  = ["la"]
      output_stream = "Custom-${data_flow.key}"
      transform_kql = "source"
    }
  }
  depends_on = [azapi_resource.hub_table]
}

resource "azurerm_sentinel_log_analytics_workspace_onboarding" "this" {
  workspace_id = azurerm_log_analytics_workspace.this.id
}

# An org that anchored in the last day and not in the last two hours. A Hub
# that stops anchoring is the one change a rewrite of history needs.
resource "azurerm_sentinel_alert_rule_scheduled" "hub_anchor_missing" {
  name                       = "hub-audit-anchor-missing"
  log_analytics_workspace_id = azurerm_sentinel_log_analytics_workspace_onboarding.this.workspace_id
  display_name               = "Panacea Hub: audit chain head not anchored for 2 hours"
  severity                   = "High"
  query_frequency            = "PT1H"
  query_period               = "P1D"
  trigger_operator           = "GreaterThan"
  trigger_threshold          = 0
  query                      = <<-KQL
    PanaceaHubAuditAnchor_CL
    | where TimeGenerated > ago(1d)
    | summarize last = max(TimeGenerated) by OrgId
    | where last < ago(2h)
  KQL
}

output "workspace_customer_id" { value = azurerm_log_analytics_workspace.this.workspace_id }
output "hub_audit_dce_endpoint" { value = azurerm_monitor_data_collection_endpoint.hub_audit.logs_ingestion_endpoint }
output "hub_audit_dcr_id" { value = azurerm_monitor_data_collection_rule.hub_audit.id }
output "hub_audit_dcr_immutable_id" { value = azurerm_monitor_data_collection_rule.hub_audit.immutable_id }
```

`versions.tf` gains `azapi = { source = "Azure/azapi", version = "2.5.0" }` (the version is
**UNVERIFIED**: pin the one `terraform init` resolves on the day, then record it in the lock file).

`stacks/workloads/main.tf` (phase 8 task 10), `module "hub"`'s `role_assignments` gain:

```hcl
    { role = "Monitoring Metrics Publisher", scope = data.terraform_remote_state.foundation.outputs.hub_audit_dcr_id },
    { role = "Log Analytics Reader", scope = data.terraform_remote_state.foundation.outputs.log_analytics_workspace_id },
```

Where `modules/observability` is instantiated (the foundation stack, per phase 8's layout,
**UNVERIFIED until merge**) re-exports the four outputs above, and `log_analytics_workspace_id` if
it does not already.

```hcl
# infra/terraform/modules/observability/tests/hubaudit.tftest.hcl (mock azurerm and azapi)
run "two_tables_two_streams" {
  command = plan
  assert {
    condition     = length(azurerm_monitor_data_collection_rule.hub_audit.data_flow) == 2
    error_message = "the Hub's audit rule needs exactly the event and anchor flows"
  }
}
run "no_text_column" {
  command = plan
  assert {
    condition     = alltrue([for c in local.hub_audit_columns : !contains(["Canonical", "Data", "Body", "Text"], c.name)])
    error_message = "the audit table must not have a column that could hold an event's text"
  }
}
run "sentinel_rule_reads_anchors" {
  command = plan
  assert {
    condition     = strcontains(azurerm_sentinel_alert_rule_scheduled.hub_anchor_missing.query, "PanaceaHubAuditAnchor_CL")
    error_message = "the missing-anchor rule must read the anchor table"
  }
}
```

`stacks/workloads/tests/workloads.tftest.hcl` gains `hub_roles_are_send_and_read`: the set of
`role` values planned for `module.hub` equals exactly `["Monitoring Metrics Publisher", "Log
Analytics Reader"]` plus whatever task 10 already assigns it (with `hub_key_vault_access` false:
none), so `Log Analytics Contributor`, `Data Purger` or `Monitoring Contributor` on the Hub fails
the plan. `role-allowlist.txt` gains the two role names.

#### Tests first

The Hub tests run on Hub task 06's integration harness (`-tags integration`, real Postgres), with
`auditsink.Fake` as sink and reader. The marker is `ZQX-MATTER-4471`, as Hub task 27's erasure test
uses (Hub `task-27.md:72`).

**`auditsink/record_test.go`** (no database):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestStringsLeaveOnlyWhenListed` | `team.created {"name": marker}` → `DataKeys ["name"]`, `DataValues {}`, `Dropped 1` | a mapping that copies `Data` |
| `TestAListedKeyStillNeedsTheGrammar` | `delegation.revoked {"reason": "member removed ZQX"}` → dropped, `Dropped 1` | trusting the key alone, so a contract change leaks text |
| `TestEnumsNumbersAndBooleansLeave` | `delegation.revoked {"reason":"member_removed"}`, `audit.read {"after":3,"count":40}` → all three values present | an allowlist so tight Sentinel can tell nothing |
| `TestTheSameKeyIsTextInAnotherKind` | `release.abandoned {"reason": "client asked"}` and `brief.erased {"reason": …}` → dropped | a key-only allowlist (`reason` is an enum in one kind, text in two) |
| `TestHashesAreTheRowsNotRecomputed` | `FromRow` with a `hash` that does not match `canonical` → the record carries the given hash | recomputing, which hides a row whose stored hash was edited |
| `TestTheRecordHasNoCanonical` | the marshalled record's JSON keys equal exactly the 14 columns of `hub_audit_columns` | a new field added to the record and not to the table, or a `Canonical` field |

**`auditsink/logs_test.go`** (an `httptest` server):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestSendPostsToTheStreamPath` | the path is `/dataCollectionRules/<dcr>/streams/Custom-PanaceaHubAudit_CL`, `api-version=2023-01-01`, the bearer from `Token("https://monitor.azure.com/.default")`, a JSON array | a wrong scope or stream name |
| `TestSendSplitsLargeBatches` | 1,200 records → three calls, none over 1 MB | one call over the API limit, refused whole |
| `TestSendFailsOnAnyNon2xx` | 429, 500, 403 → non-nil | a 429 treated as sent, advancing the cursor over lost events |
| `TestQueryRefusesABadOrgID` | `AnchorsOf(ctx, "org-1' or 1==1", …)` → error, zero requests | KQL injection through the org id |

**`server/auditstream_integration_test.go`:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestEveryEventStreamsOnce` | 30 writes over two orgs; `StreamOnce` twice → the fake holds 30 records, each `(org, seq)` once, and each org's cursor equals its head | a cursor never advanced, or advanced per org wrongly |
| `TestAFailedSendIsRetried` | `FailN = 1` → round 1 sends 0 and the cursor stays; round 2 sends all | the cursor advanced before the 2xx |
| `TestNoTextLeaves` | a team named with the marker, a project likewise, an invite to `marker@example.com`, a release abandoned and a brief erased with the marker in `reason`, a brief revised with the marker in content → after `StreamOnce` and `AnchorOnce`, `bytes.Contains(w, marker)` is false for every `Fake.Wire` batch | any path by which text reaches the stream |
| `TestEverySubjectIsAnIdentifier` | every streamed `Subject` matches `^[a-z]+-[0-9a-f]+$`, a route pattern `^(GET\|POST\|PUT\|PATCH\|DELETE) /v1/`, or `^sha256:[0-9a-f]{64}$` | a kind whose subject is a name |
| `TestTheCursorMovesForwardOnly` | as `hub_app`: `UPDATE … SET streamed_seq = streamed_seq - 1` → `23514`; past the head → `23514` | a cursor wound back or forward by hand |
| `TestStreamingTakesNoHeadLock` | a write holding the org's head lock does not block `StreamOnce` for that org (it completes within 1 s) | the streamer serialising every write in the org |
| `TestAnchorsEveryHourEvenWhenIdle` | two `AnchorOnce` with no writes between → two anchors, same `Seq` and `Hash` | anchors only on change, so silence is not a signal |
| `TestTeamModeNeedsTheSink` | sink `off` → `TeamModeAllowed` is 503 `unavailable`, `reason: audit_not_streaming`; `logs` → nil; `fake` alone → 503; `fake` with `TeamModeUnstreamed` → nil; `off` with `TeamModeUnstreamed` → 503 (b01) | team mode reachable with the audit inside the building; the dev flag opening the gate with no sink at all |
| `TestRouteSetsModeThroughTheSinkGate` (b01, integration) | a Hub with sink `off` and the flag `false`: route 87 → 503, the project still `solo`; the same pool, sink `logs`: → 200 `mode: "team"` | `teamModeGate` left as task 06's flag-only body, so task 45's sink never gates route 87 |

**`cmd/hub/verify_integration_test.go`** (added to Hub task 27's):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAnchorPassesOnAHealthyChain` | 30 events, stream, anchor → exit 0, exactly `ok anchor (2 anchors, 30 events in 2 orgs)` | a check that passes on zero anchors |
| `TestAConsistentRewriteIsCaught` | stream and anchor; then, as the owner, disable the triggers, change row 12's `canonical` and recompute `hash` and `prev_hash` for 12–30 so the chain links → the `audit` check prints `ok audit` **and** the `anchor` check exits 1 with `FAULT anchor: org <id> seq 30: A1` | **the scope's test**: the in-database chain as the only proof |
| `TestARewriteBeforeTheNextAnchorIsCaught` | stream only (no anchor since row 12 was written), rewrite 12–30 as above → exit 1, `seq 12: A4` | anchors alone, leaving the hour between anchors open |
| `TestATruncatedChainIsCaught` | anchor at 30, delete rows 21–30 and set the head to 20 → exit 1, `A2` | a head allowed behind its anchor |
| `TestAHubThatNeverAnchoredIsCaught` | events older than 2 h, zero anchors → exit 1, `A5` | a stopped anchor loop passing verify |

#### The contract lines

Hub contract (`2026-09-23-hub-contract.md`):
- §11, row T5 (`:2073`): the residual *"audit anchoring is later (ADR §5.9)"* becomes *"every event
  is streamed as metadata to Log Analytics within about 15 s and each org's head is anchored there
  hourly; `hub verify`'s `anchor` check compares them (Phase T task 45). An owner rewrite is caught
  unless it lands inside the stream lag and the streamer is also stopped, which the missing-anchor
  rule raises within two hours."*
- §12: `PANACEA_APP_HUB_AUDIT_SINK` (`off` · `fake` · `logs`), `PANACEA_APP_HUB_AUDIT_DCE`,
  `PANACEA_APP_HUB_AUDIT_DCR`, `PANACEA_APP_HUB_AUDIT_WORKSPACE`, `PANACEA_APP_HUB_VERIFY_SINCE`;
  `logs` needs all three of DCE, DCR and workspace, or `hub serve` exits at start-up. And
  §12.2's `PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED` row (task 01's, `task-01.md:245`) and §7.4's
  gate sentence (`task-01.md:522`) gain *"and only beside `PANACEA_APP_HUB_AUDIT_SINK=fake`"*
  (b01 round 2, re-review I-4), so the contract states the rule `TeamModeAllowed` enforces.
- §15: *"That Azure Monitor accepts the stream as declared is verified only by task 57's run."*

#### Steps

- [ ] **Step 1: Read, after the sync-merges of `feat/hub` and `feat/env`.** Hub task 27's `audit`
  check and Hub task 08's verify runner as merged; Hub task 24's loop; phase 8 task 04's module and
  task 10's `module "hub"`. Where a merged name differs, follow it; the tests do not change.
- [ ] **Step 2: Record tests, then `record.go`.** They pass. (30 min)
- [ ] **Step 3: `sink.go`, `logs_test.go`, then `logs.go`.** They pass. (25 min)
- [ ] **Step 4: Migration and store.** `00NN_audit_stream.sql`, `store/auditstream.go`. (20 min)
- [ ] **Step 5: Loops.** `auditstream_integration_test.go` fails; `server/auditstream.go`,
  `config.go`, `serve.go`; it passes. Then the `teamModeGate` swap, and
  `go test -tags integration ./internal/hub/... -count=1` (tasks 06, 08, 43, 44 and 14's suites,
  whose harnesses set the flag beside sink `fake`) stays green (re-review I-4, I-A). (35 min)
- [ ] **Step 6: The `anchor` check.** The five verify tests fail, then pass. (25 min)
- [ ] **Step 7: Terraform.** `hubaudit.tf`, the outputs, `versions.tf`, the workloads entries, both
  tests; `terraform test` in both directories; `bash infra/scripts/role-allowlist.sh`. (20 min)
- [ ] **Step 8: Contract.** The three edits above.
- [ ] **Step 9: Commit.** `git add` the new files; `git commit -F <msg> -- <paths>`.

#### Acceptance

```bash
cd control-plane && go test ./internal/hub/auditsink/ -count=1
# ok  	…/internal/hub/auditsink	…
cd control-plane && go test -tags integration ./internal/hub/server/ -run 'TestEveryEventStreams|TestAFailedSend|TestNoTextLeaves|TestEverySubject|TestTheCursor|TestStreamingTakes|TestAnchorsEvery|TestTeamModeNeeds' -count=1 -v 2>&1 | grep -c '^--- PASS'
# 8
cd control-plane && go test -tags integration ./cmd/hub/ -run 'TestAnchor|TestAConsistent|TestARewrite|TestATruncated|TestAHubThat' -count=1 -v 2>&1 | grep -c '^--- PASS'
# 5
cd control-plane && go test -tags integration ./internal/hub/... -count=1 > /tmp/t45-hub.txt 2>&1; echo rc=$?
# rc=0: tasks 06, 08, 43, 44 and 14's team harnesses still turn P team after the swap (b01 I-4, I-A)
cd infra/terraform/modules/observability && terraform init -backend=false >/dev/null && terraform test
# Success! … passed, 0 failed.
cd infra/terraform/stacks/workloads && terraform init -backend=false >/dev/null && terraform test
# Success! … passed, 0 failed.
bash infra/scripts/role-allowlist.sh
# role-allowlist: <n> roles used, all allowed
```

#### What this task does not claim

- **That Azure accepts any of it.** The Logs Ingestion and query calls, the table schema and the
  DCR are from Microsoft's documentation as known at the knowledge cutoff; nothing here has
  touched a real workspace. Task 57's run on the real environment is the first proof, and the
  skeleton's gate names it.
- **That the stream leaks nothing through its hashes.** `Hash` is SHA-256 over the whole canonical
  envelope, text included. Someone with workspace read and a short list of guesses for one
  `reason` could test each guess only by rebuilding the whole envelope, org, time and ids
  included. That is a real but narrow attack; the workspace's readers are the org's own security
  staff.
- **That an attacker who holds the database owner role and stops the Hub's pods is caught at
  once.** The missing-anchor rule raises it within two hours; `hub verify` catches the rewrite
  whenever it runs next.
- **That Sentinel's retention outlives Postgres's.** The tables inherit the workspace retention
  (30 days in dev). An anchor older than the retention cannot be compared, so the ISM-required
  period (phase 8 `task-04.md:39`, an owner input) is also this check's reach.
- **Any Sentinel content beyond one rule.** Other detections (forbidden-rate spikes, erasures,
  `compliance` route lowering from task 43) are the security team's to write on these tables.

#### Gaps found in the skeleton

1. **The write that sets a project's `mode` to `team`. Resolved by fix batch b01 (review X-C1):**
   task 06's route 87 is the one writer and calls `teamModeGate` before any write; this part
   fills that seam with `TeamModeAllowed`, and `TestRouteSetsModeThroughTheSinkGate` fails if the
   seam is left unfilled. Share (task 56) reaches it through task 21's local route only.
2. **The skeleton lists task 45 as a Hub delta only**, but the stream needs a collection endpoint,
   two custom tables, a collection rule, Sentinel onboarding and two roles, none of which phase 8
   plans. This part carries them as a phase 8 delta on task 04's module and task 10's stack; the
   skeleton's file map (`:261`) should name them.
3. **`azapi` is a new provider** for phase 8, which pins azurerm alone (phase 8 `task-04.md:18`).
   If azurerm 4.81.0 can declare a custom table's columns, use that instead; either way it is a
   phase 8 owner call.
4. **The collection endpoint is public.** Phase 8's private networking for PaaS (a private link
   scope for Azure Monitor) is not planned; the Hub reaches the endpoint over the internet with its
   Entra token. Adding an Azure Monitor Private Link Scope is a later phase 8 task.
5. **The foundation stack's output names** (`log_analytics_workspace_id` and the four new ones)
   are assumed; phase 8's stack layout is **UNVERIFIED until merge**.
