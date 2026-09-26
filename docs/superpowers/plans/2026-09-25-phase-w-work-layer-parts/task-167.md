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
