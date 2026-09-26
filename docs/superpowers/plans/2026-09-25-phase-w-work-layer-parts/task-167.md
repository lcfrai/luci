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
