### Task 184: triage rotation imported from PagerDuty or Opsgenie (L-218)

Stream **WL** (Hub and desktop), about 2 hours (an estimate, not a measurement). Depends on task
**174** (W5's proof) and task **134** (the rotation on the desktop), and builds on task **121**'s
Hub-held rotation and task **114**'s wire (seam **SW.20**). 05 L-218 (*"triage rotation imported
from PagerDuty or Opsgenie"*, slice `later`) and L-50 (*"syncing it from PagerDuty and the like is
L-218"*). Hub server paths (`server/`, `store/`, `migrate/`) are **planned, not on `feat/hub`**,
so every name taken from them is **UNVERIFIED** until they land (rule H1).

A team that already keeps its on-call rota in PagerDuty or Opsgenie should not keep a second one
in Panacea. This task lets the team point task 121's rotation at that schedule, so there is **one
rota, not two**, and Panacea reads who is on call from it once an hour.

**Decisions made here, so nobody re-derives them:**

1. **Read-only, and only "who is on call now".** The pass asks the vendor for the current on-call
   person of one schedule. It never reads layers, overrides or restrictions as structure, never
   writes to the vendor and never pages anyone. The vendor's own scheduled end for that shift is
   stored as `onCallUntil` and shown as *until <t> · PagerDuty*: a fact the vendor stated, never a
   forecast of ours (`OD-27`).
2. **Match by Entra email, exact, case-insensitive, current members only.** The vendor's email
   is lower-cased and compared whole to `lower(users.email)` of a **current** member of the team
   (`team_members` row). No substring, no domain match, no display-name match. `alice@kpmg.com.au`
   never matches `alice@kpmg.com.au.evil`.
3. **The import never adds a member or grants a right.** An on-call person who is not on the team
   leaves duty **empty** with the words *on call in PagerDuty, not on this team*. Membership stays
   with `PUT /v1/teams/{teamId}/members/{userId}` and, from task 185, group links.
4. **An outage never clears duty.** A vendor error (network, 4xx, 5xx, an unparseable body) keeps
   the last match and records `lastError`; the next hourly run retries.
5. **One event per change of the match, none per sync.** A change of the stored match (a member to
   another member, or to or from nobody) writes task 121's `triage.handover` event, whose data is
   `{from, to, skipped, by}` (`task-121.md:112`), with `actor: null`, `skipped: 0`, `by` the
   rotation's author, `from`/`to` a `u-` id or `null`, and one added scalar `source: "pagerduty" |
   "opsgenie"` (05 §3.6). A sync that finds the same match writes nothing but `ext_last_synced_at`,
   a column, not an event. An error writes no event: the match did not change.
6. **No vendor call inside a transaction.** Task 121's passes run inside the per-org write
   transaction (`task-121.md:18-21`). A network call there would hold row locks for as long as a
   vendor takes. So the sync runs in Hub task 24's `Sweep` **after the advisory lock is taken and
   before the per-org loop**: a short read transaction lists the org's due sources, the vendor calls
   run with no transaction open (10 s timeout each), and a short write transaction applies each
   result. The session-level advisory lock still covers the whole run, so two sweeps make one call.
   `passHandovers` skips a rotation with a source; its events come from the sync.
7. **Due means hourly, first sync within a sweep.** A source is due when `ext_last_synced_at` is
   `NULL` or at least one hour old by `cfg.Now()`. Setting a source clears the match columns, so the
   next sweep (task 121 speaks of a sixty-second sweep) syncs it; until then the read says
   *not synced yet*, never a guessed name.
8. **`members` stays stored.** Switching `external` off returns task 121's derived duty from the
   kept list; switching it on does not overwrite the list. Task 121's `putRotation` validation of
   `members` is unchanged (`task-121.md:328-330`).
9. **One credential per org and vendor, never per team.** A PagerDuty or Opsgenie API token is an
   account's key, not a team's, so a team admin never pastes one into a rotation: an org admin sets
   it once, under the fixed names `rotation/pagerduty` and `rotation/opsgenie`, and a team only
   names a `scheduleRef`. A team whose org has no credential for the vendor gets 422
   `rotation_source_unconfigured` on `PUT`. Opsgenie's EU accounts use a different API host, so the
   host is config beside the credential (`PANACEA_APP_HUB_OPSGENIE_BASE`, default the public host).
   The secret's home follows W5's integrations and is referenced by name only.
   Task 145's scope puts W5's secrets in Key Vault under the Hub's workload identity, held in the
   database as a `secret_ref` and version, and calls Key Vault a **proposed Hub-contract addition**
   (`…-scopes/tasks-141-160.md:241-246`); parts 145, 150 and 157 are not written on 2026-09-26, so
   the home is **UNVERIFIED**. The token appears in no event, log line, error string or fixture. If
   the Hub contract has no secret home at sync time, this task stops at step 1 and reports.
10. **The desktop computes nothing.** Task 134 draws `TriageRotationView` as read (its decision 2).
   This task only adds the `external` fields to that read and one choice to the editor.

**Files:**
- `control-plane/internal/hub/hubapi/work_roadmap.go` ⚑ — `RotationSource` and
  `TriageRotation.External` (additive; task 114's file)
- `control-plane/internal/hub/rotation/source.go` — the `Source` interface, `OnCall`, the matcher
  (new)
- `control-plane/internal/hub/rotation/pagerduty.go`, `opsgenie.go` — the two readers (new)
- `control-plane/internal/hub/rotation/{source,pagerduty,opsgenie}_test.go` and
  `rotation/testdata/*.json` — golden payloads written from each vendor's public docs (new)
- `control-plane/internal/hub/migrate/sql/NNNN_work_wl_rotation.sql` — the columns (new; the next
  free number at sync time)
- `control-plane/internal/hub/store/work_schedules.go`, `server/work_schedules.go` ⚑ — store and
  return `external` (task 121's files)
- `control-plane/internal/hub/server/janitor.go` ⚑ (Hub task 24's `Sweep`) — one call,
  `syncRotationSources(ctx, now, &report)`, after `pg_try_advisory_lock(hashtext(
  'panacea-app-hub-janitor'))` succeeds and before the per-org loop (decision 6;
  `task-121.md:18-21`, `:71-74`); one counter, `SweepReport.RotationSynced`
- `control-plane/internal/hub/server/janitor_rotation.go` (new) — `syncRotationSources`
- `control-plane/internal/hub/server/janitor_w4.go` ⚑ — `passHandovers` (`task-121.md:229-235`)
  skips a rotation whose `ext_kind` is set
- `control-plane/internal/hub/server/work_rotation_sync_integration_test.go` — `-tags
  integration` (new)
- `control-plane/internal/hub/hubclient/fake_work*.go` ⚑ — the fake stores `external` and
  returns it; it never calls a vendor
- `desktop/src/api/work.ts` ⚑ — `RotationSource` and the two optional fields (task 125's
  `TriageRotation`/`TriageRotationView`, `task-125.md:152-154`, appended)
- `desktop/src/components/plan/TriageRotationSettings.tsx` ⚑ (+ `.test.tsx`) — task 134's editor
  (`task-134.md:61-62`): one choice, *Hub rota*, *from PagerDuty*, *from Opsgenie*
- `desktop/src/work/recurring.ts` ⚑ — task 134's `dutyLine` gains the vendor clause
  (`task-134.md:68-69`)

#### `hubapi/work_roadmap.go` ⚑, the additions (complete)

Additive on task 114's `TriageRotation` and `TriageRotationView` (`task-114.md:272-286`), after
task 121's G1 (`TZ`, `OnDuty *User`, `HandoverOn`; `task-121.md:101`, `:335-336`), so SW.20 keeps
every field it has.

```go
// RotationSource points a team's triage rotation at a schedule in an incident
// tool the team already keeps. The Hub reads who is on call now, at most once
// an hour; it never writes to the vendor and never pages anyone (task 184).
type RotationSource struct {
	Kind        string `json:"kind"`        // RotationPagerDuty | RotationOpsgenie
	ScheduleRef string `json:"scheduleRef"` // the vendor's schedule id, ^[A-Za-z0-9_-]{1,64}$

	// Read-only: set by the sync, ignored on PUT.
	LastSyncedAt *string `json:"lastSyncedAt,omitempty"` // RFC 3339 UTC; nil = not synced yet
	LastError    *string `json:"lastError,omitempty"`    // one of the RotationErr* codes, never vendor text
	OnCallUntil  *string `json:"onCallUntil,omitempty"`  // the vendor's stated shift end, RFC 3339
	NotOnTeam    bool    `json:"notOnTeam,omitempty"`    // someone is on call who is not a current member
}

const (
	RotationPagerDuty = "pagerduty"
	RotationOpsgenie  = "opsgenie"

	RotationErrUnreachable = "vendor_unreachable" // network error or timeout
	RotationErrRejected    = "vendor_rejected"    // 401 or 403: the org's credential
	RotationErrNoSchedule  = "schedule_not_found" // 404 for scheduleRef
	RotationErrVendor      = "vendor_error"       // any other status, or a body we cannot read
	RotationErrAmbiguous   = "on_call_ambiguous"  // more than one person on call now
)

// TriageRotation gains one optional field (the rest is task 114's, unchanged):
//	External *RotationSource `json:"external,omitempty"`
//
// TriageRotationView is unchanged in shape. With External set, OnDuty is the
// stored match (nil when nobody matched), HandoverOn is "" and
// Rotation.External.OnCallUntil carries the vendor's end instead.

// Validate, appended to task 114's TriageRotation.Validate: External nil is
// task 121's rotation; else Kind in the two constants and ScheduleRef matching
// the pattern, each failure a 422 with field "external.kind" or
// "external.scheduleRef".
```

`hubapi/codes.go` ⚑ gains one code, `rotation_source_unconfigured` (422: the org has no
credential for that vendor, decision 9).

#### `rotation/source.go`, the interface and the matcher (complete)

```go
package rotation

// OnCall is what a vendor says about a schedule right now.
type OnCall struct {
	Emails []string   // every person the vendor lists as on call now, as the vendor spells them
	Until  *time.Time // the vendor's end of the current shift, if it states one
}

// Source reads one schedule. Implementations never retry, never log the
// credential and return a *SourceError for every failure.
type Source interface {
	Kind() string // hubapi.RotationPagerDuty | hubapi.RotationOpsgenie
	OnCallNow(ctx context.Context, scheduleRef string, now time.Time) (OnCall, error)
}

// SourceError carries one hubapi.RotationErr* code and the HTTP status, never
// the response body and never a header.
type SourceError struct {
	Code   string
	Status int // 0 for a network error
}

func (e *SourceError) Error() string { return "rotation source: " + e.Code }

// Member is a current team member as the store reads it.
type Member struct {
	UserID string // u-…
	Email  string // users.email, as Entra gave it at sign-in
}

// Match returns the one current member whose email equals the one on-call
// email, compared whole after strings.ToLower and strings.TrimSpace on both
// sides. ok is false when nobody matched; ambiguous is true when the vendor
// lists more than one distinct email (the sync keeps the last match and
// records RotationErrAmbiguous rather than choosing). No substring, prefix,
// domain or display-name match exists.
func Match(oc OnCall, members []Member) (userID string, ok bool, ambiguous bool)
```

`desktop/src/api/work.ts` ⚑, appended beside task 125's types (`task-125.md:152-154`):

```ts
export type RotationSourceKind = "pagerduty" | "opsgenie";
export type RotationSourceError =
  | "vendor_unreachable" | "vendor_rejected" | "schedule_not_found" | "vendor_error" | "on_call_ambiguous";
/** Read-only fields are the Hub's; the desktop sends kind and scheduleRef only. */
export interface RotationSource {
  kind: RotationSourceKind;
  scheduleRef: string;
  lastSyncedAt?: string;
  lastError?: RotationSourceError;
  onCallUntil?: string;
  notOnTeam?: boolean;
}
// task 125's TriageRotation gains: external?: RotationSource
```

#### The two readers (`rotation/pagerduty.go`, `rotation/opsgenie.go`)

Both are written from each vendor's **public API documentation, not from a live call**; every
path, header and field below is **UNVERIFIED** until a sandbox account answers (step 1 records
the doc pages and the date read). Each reader takes a `*http.Client` and a base URL, so tests
point it at `httptest.Server`, and reads the body through `io.LimitReader(body, 1<<20)`.

| Reader | Request (read-only) | Fields read | `Until` |
|---|---|---|---|
| PagerDuty | `GET {base}/oncalls?schedule_ids[]={ref}&include[]=users&earliest=true`, `Authorization: Token token=<secret>`, `Accept: application/vnd.pagerduty+json;version=2` | `oncalls[].user.email` | the earliest `oncalls[].end` |
| Opsgenie | `GET {base}/v2/schedules/{ref}/on-calls?scheduleIdentifierType=id&flat=true`, `Authorization: GenieKey <secret>` | `data.onCallRecipients[]` (emails when `flat=true`) | `nil`: the call states no end, so none is shown |

Status mapping, the same for both: 200 → parse; 401 and 403 → `vendor_rejected`; 404 →
`schedule_not_found`; any other status or an unparseable body → `vendor_error`; a transport error
or the 10 s timeout → `vendor_unreachable`. The `SourceError` keeps the code and the status only.

#### The migration (`migrate/sql/NNNN_work_wl_rotation.sql`, complete)

```sql
-- Task 184: a triage rotation may read who is on call from PagerDuty or Opsgenie.
-- Additive on task 116's work_triage_rotations; RLS and grants are 116's, unchanged.
ALTER TABLE work_triage_rotations
  ADD COLUMN ext_kind           text        NULL,
  ADD COLUMN ext_schedule_ref   text        NULL,
  ADD COLUMN ext_on_duty        text        NULL REFERENCES users(id),
  ADD COLUMN ext_not_on_team    boolean     NOT NULL DEFAULT false,
  ADD COLUMN ext_on_call_until  timestamptz NULL,
  ADD COLUMN ext_last_synced_at timestamptz NULL,
  ADD COLUMN ext_last_error     text        NULL,
  ADD CONSTRAINT work_triage_rotations_ext_kind
    CHECK (ext_kind IS NULL OR ext_kind IN ('pagerduty', 'opsgenie')),
  ADD CONSTRAINT work_triage_rotations_ext_pair
    CHECK ((ext_kind IS NULL) = (ext_schedule_ref IS NULL)),
  ADD CONSTRAINT work_triage_rotations_ext_ref
    CHECK (ext_schedule_ref IS NULL OR ext_schedule_ref ~ '^[A-Za-z0-9_-]{1,64}$'),
  ADD CONSTRAINT work_triage_rotations_ext_error
    CHECK (ext_last_error IS NULL OR ext_last_error IN ('vendor_unreachable', 'vendor_rejected',
      'schedule_not_found', 'vendor_error', 'on_call_ambiguous'));

-- The sync lists due sources in one indexed read per org.
CREATE INDEX work_triage_rotations_ext_due
  ON work_triage_rotations (org_id, ext_last_synced_at NULLS FIRST)
  WHERE ext_kind IS NOT NULL;
```

`users(id)` and `org_id` are the Hub's and task 116's names (**UNVERIFIED** until both land; a
different spelling is a find-and-replace). No column holds a token or an email.

#### The store (`store/work_schedules.go` ⚑, signatures complete)

```go
// RotationRow (task 121, task-121.md:291-296) gains:
//	ExtKind, ExtScheduleRef *string
//	ExtOnDuty               *string // u-…
//	ExtNotOnTeam            bool
//	ExtOnCallUntil          *time.Time
//	ExtLastSyncedAt         *time.Time
//	ExtLastError            *string

// PutRotation (task 121) writes ExtKind and ExtScheduleRef. When either changes
// it also clears ExtOnDuty, ExtNotOnTeam, ExtOnCallUntil, ExtLastSyncedAt and
// ExtLastError in the same statement, so a new source starts "not synced yet".

// DueSources lists the org's rotations with a source whose last sync is NULL or
// at or before now − 1 h, oldest first, at most 100 (the rest wait a sweep).
func DueSources(ctx context.Context, q Querier, now time.Time) ([]RotationRow, error)

// CurrentMembers is the team's members with their users.email, for Match.
func CurrentMembers(ctx context.Context, q Querier, teamID string) ([]rotation.Member, error)

// ApplySync writes one sync result under SELECT … FOR UPDATE on the row. It
// returns changed=true when ExtOnDuty moved (to or from NULL included), and the
// previous value, so the caller appends exactly one triage.handover.
type SyncResult struct {
	OnDuty    *string    // the match, or nil
	NotOnTeam bool
	Until     *time.Time
	Err       *string    // a RotationErr* code; when set, OnDuty/NotOnTeam/Until are ignored
}
func ApplySync(ctx context.Context, tx pgx.Tx, teamID string, at time.Time, r SyncResult) (prev *string, changed bool, err error)
```

`ApplySync` with `Err` set updates `ext_last_error` and `ext_last_synced_at` only: the match,
`ext_not_on_team` and `ext_on_call_until` keep their values (decision 4). Without `Err` it clears
`ext_last_error`.

#### The sync (`server/janitor_rotation.go`, contract complete)

```go
package server

// RotationSources is how Sweep reaches the vendors; tests pass fakes.
// JanitorConfig (Hub task 24) gains: Sources map[string]rotation.Source // by Kind
//
// syncRotationSources runs once per Sweep, holding the session-level janitor
// lock, before the per-org loop (decision 6). For each org:
//  1. read tx (set_config('hub.org_id', org, true)): store.DueSources(now);
//     commit.
//  2. for each due row, no tx open: src := cfg.Sources[kind]; nil → the org has
//     no credential: Err = "vendor_rejected". oc, err := src.OnCallNow(ctx10s,
//     ref, now). err → SyncResult{Err: code}. Else members :=
//     store.CurrentMembers (a read tx); id, ok, amb := rotation.Match(oc, members);
//     amb → Err "on_call_ambiguous"; ok → OnDuty id; !ok and len(oc.Emails) > 0 →
//     NotOnTeam; no emails at all → OnDuty nil, NotOnTeam false.
//  3. write tx (hub.org_id set, store.LockHead as task 121): prev, changed :=
//     store.ApplySync(...); changed → emitWork("triage.handover", team tm-,
//     Actor nil, data {from: prev, to: OnDuty, skipped: 0, by: row.By,
//     source: kind}); commit.
// r.RotationSynced counts rows written in step 3. An error in one row is that
// row's lastError and never stops the pass for other rows or orgs.
func syncRotationSources(ctx context.Context, now time.Time, r *SweepReport) error
```

The log line per row is `rotation sync team=<tm-> kind=<kind> result=<ok|code>`: no email, no
schedule name, no token, no vendor body.

#### The routes (`server/work_schedules.go` ⚑)

Task 121's two routes (`task-121.md:309-310`, `:328-333`), extended, no new route:

- **`putRotation`**: `external` validated as above; a kind with no org credential → 422
  `rotation_source_unconfigured`; read-only fields in the body are ignored. `triage.rotation_set`'s
  data gains `external: "pagerduty" | "opsgenie" | ""`. Team admin only, as today: the matrix row
  for the route is task 121's and gains one case (a `member` PUT with `external` → 403).
- **`getRotation`**: with a source, `onDuty` is `ext_on_duty` **if that user is still a current
  member**, else `null` with `external.notOnTeam: true`; `handoverOn` is `""`; the read-only
  fields come from the columns. Without a source, task 121's derivation, unchanged.

The fake (`hubclient/fake_work*.go` ⚑) stores `external` and returns it with the read-only fields
empty; it has no `Source` and never calls a vendor.

#### The desktop (`TriageRotationSettings.tsx` ⚑ and `recurring.ts` ⚑)

One segmented choice above task 134's member list: **Hub rota** · **from PagerDuty** · **from
Opsgenie**. A vendor choice shows one text field, *Schedule id*, and keeps the member list, dimmed,
with the words *kept for when you switch back*. The desktop sends `{...rotation, external: {kind,
scheduleRef}}` through task 125's `setTriageRotation`.

`dutyLine` (`task-134.md:68`) gains the vendor clause and reads the Hub's fields only:

| The read | The line |
|---|---|
| matched, `onCallUntil` set | *On triage duty: Priya, until Tue 6 Oct 09:00 · PagerDuty* |
| matched, no `onCallUntil` | *On triage duty: Priya · Opsgenie* |
| `notOnTeam` | *On call in PagerDuty, not on this team* |
| `lastSyncedAt` absent | *PagerDuty rota: not synced yet* |
| `lastError` set, a match kept | the matched line, then *· last read failed* in the muted tone |

The time is the vendor's, formatted in the person's zone with `tabular-nums`. No countdown, no
*in 3 h*, no bar (`OD-27`).

#### Tests to write first, part 1 — `rotation/*_test.go` (pure, `httptest`, no database)

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestMatchIsWholeAndCaseInsensitive` | `ALICE@kpmg.com.au` matches member `alice@kpmg.com.au`; `alice@kpmg.com.au.evil` and `li@kpmg.com.au` match nobody | a substring or case-sensitive match |
| `TestMatchNeverPicksBetweenTwo` | two distinct emails on call → `ambiguous: true`, `ok: false` | a sync that picks the first person |
| `TestPagerDutyGolden` | `testdata/pagerduty_oncalls.json` → one email, `Until` the earliest `end` | a reader that drops the shift end |
| `TestOpsgenieGolden` | `testdata/opsgenie_oncalls_flat.json` → the recipients, `Until` nil | a guessed end for Opsgenie |
| `TestStatusesMapToCodes` | 401, 403, 404, 500, a torn body and a closed server → the five codes, each `Status` kept | an outage read as "nobody on call" |
| `TestTheTokenIsInNoError` | the server echoes the `Authorization` header in a 500 body; `err.Error()` and `%+v` of the error hold neither the token nor the body | a leaked credential |

#### Tests to write first, part 2 — `server/work_rotation_sync_integration_test.go` (`//go:build integration`)

Each runs `RunJanitorOnce` (task 121) with `cfg.Sources` a fake `Source` that counts calls.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAMatchedOnCallIsOnDuty` | on-call email = member Priya's → `GET` reads `onDuty` Priya, `lastSyncedAt` set; one `triage.handover {from: null, to: Priya, source: "pagerduty"}`, `actor` null | a missing match |
| `TestTheSameMatchWritesNoEvent` | a second sync an hour later, same person → zero new events, `lastSyncedAt` moved | an event per sync |
| `TestANonMemberAddsNobody` | on-call email of a user in the org but not the team → `onDuty` null, `notOnTeam: true`; `team_members` rows unchanged, no `team.member_set` event | an import that adds members or grants a right |
| `TestAnOutageKeepsDuty` | after a match, the fake returns 500 → `onDuty` still Priya, `lastError: "vendor_error"`, no event | an outage that clears duty |
| `TestSwitchingOffKeepsTheList` | `PUT` without `external` → `GET` equals task 121's derived duty over the stored `members` | an import that overwrote the list |
| `TestARemovedMemberIsNotOnDuty` | Priya matched, then removed from the team → `GET` reads `onDuty` null, `notOnTeam: true` before the next sync | a stale match that outlives membership |
| `TestTwoSweepsMakeOneCall` | two `RunJanitorOnce` at once on one due source → the fake counts 1 call; one report `Locked: false` | a missing lock |
| `TestNotDueWithinTheHour` | a sync at `t`, then sweeps at `t+59m` → 1 call; at `t+60m` → 2 | a sync on every sweep |
| `TestNoCredentialIs422` | `PUT` with `kind: "opsgenie"` and no org credential → 422 `rotation_source_unconfigured` | a source that fails silently later |
| `TestAMemberCannotSetASource` | a `member`'s `PUT` with `external` → 403 (matrix case) | a missing authorisation row |
| `TestTheTokenIsInNoEventOrLog` | after syncs and a 500 whose body echoes the token: no `events.data`, no captured log line and no `lastError` holds the token or any email | a leaked credential or a personal detail in the log |

#### Tests to write first, part 3 — `TriageRotationSettings.test.tsx` and `recurring.test.ts` ⚑

| Test | Asserts | Mutant it kills |
|---|---|---|
| `offers three sources` | the choice has *Hub rota*, *from PagerDuty*, *from Opsgenie*; picking one sends `external {kind, scheduleRef}` only | read-only fields sent from the desktop |
| `keeps the members` | switching to PagerDuty still sends `members` | a desktop that drops the list |
| `dutyLine reads the vendor's words` | the five rows of the table above, exactly | a guessed name or a missing *not on this team* |
| `no prediction` | `expectNoPrediction(container)` with `onCallUntil` set: no *in 3 h*, no bar | a countdown |

#### Steps

- [ ] **Step 1: confirm the sync base** (rule H1): `feat/hub` sync-merged; tasks 114, 116, 121,
  125 and 134 committed; W5's secret home settled (decision 9; stop and report if not). Record the
  two vendors' doc pages and the date read in the golden files' README.
- [ ] **Step 2: pure tests first.** Write `rotation/*_test.go` and the two golden files; they fail
  to compile. Write `source.go`, `pagerduty.go`, `opsgenie.go`; run until green.
- [ ] **Step 3: the migration and store.** Write the SQL, run `migrate` against the compose stack,
  extend `RotationRow`, `PutRotation`; add `DueSources`, `CurrentMembers`, `ApplySync`.
- [ ] **Step 4: integration tests, then the sync.** Write part 2; it fails. Write
  `janitor_rotation.go`, the one call in `Sweep`, the skip in `passHandovers`, the route changes and
  the matrix case. Run until green.
- [ ] **Step 5: the fake**, storing `external`; one conformance row (a `PUT` with a source reads
  back `kind` and `scheduleRef`, read-only fields empty) against the fake and the Hub.
- [ ] **Step 6: the desktop.** Part 3 first, then the type, the choice and `dutyLine`.
- [ ] **Step 7: acceptance**, then commit by path, each message ending with the `Co-Authored-By`
  line.

#### Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/work
go test ./control-plane/internal/hub/rotation/... -race -count=1
# expect: ok  .../internal/hub/rotation
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run '^Test(AMatchedOnCall|TheSameMatch|ANonMember|AnOutageKeeps|SwitchingOff|ARemovedMember|TwoSweepsMakeOne|NotDueWithin|NoCredentialIs422|AMemberCannotSetASource|TheTokenIsInNoEvent)' \
  2>&1 | grep -c "^--- PASS"
# expect: 11
go test -tags integration ./control-plane/internal/hub/... -race -count=1 -run 'Matrix|Conformance'
# expect: ok for every package
npm --prefix desktop test -- TriageRotationSettings recurring
# expect: every test passes, 4 of them this task's
npm --prefix desktop run typecheck
# expect: exit 0
```

#### What this task does NOT claim

- PagerDuty's layers, overrides or restrictions, or Opsgenie's rotations, mapped one to one: the
  Hub reads only who is on call now.
- Paging, acknowledging or escalating anyone from Panacea; any write to either vendor.
- A sync faster than hourly, or a shift end for Opsgenie.
- A live call to either vendor: every payload is a golden file written from public docs
  (**UNVERIFIED**), and the Hub reaching either vendor through phase 8's egress is **UNVERIFIED**.
- That KPMG uses PagerDuty or Opsgenie.

**Parity.** Closes **L-218** (slice `later`), and with tasks 121 and 134 the *later* clause of
**L-50**; the command and output are recorded by task 187.

#### Gaps found while writing this part

1. **The scope names `server/janitor.go` for the pass.** Task 121's rotation pass is
   `server/janitor_w4.go`'s `passHandovers`, run inside a per-org write transaction; a vendor call
   there holds locks, so the sync is a new step in `Sweep` (decision 6).
2. **The scope's `triage.handover {teamId, from, to}`** is task 121's `{from, to, skipped, by}` on
   subject `tm-` (`task-121.md:112`); this part adds `source`.
3. **The scope's `RotationSource` has no credential** and no non-member flag. Decision 9 keeps the
   credential per org (no field); `notOnTeam` is added so the desktop can say the words.
4. **`handoverOn` has no meaning with a source.** This part returns `""`; task 114's type could
   make it optional instead (task 114 and task 121 already disagree on its name:
   `NextHandoverOn`, `task-114.md:285`, against `handoverOn`, `task-121.md:332`).
5. **The secret home** for W5 and WL is a proposed Hub-contract addition, not settled
   (`…-scopes/tasks-141-160.md:241-246`); parts 145, 150 and 157 are not written.
6. **The File map has no WL rows**: `rotation/`, `server/janitor_rotation.go` and the migration
   should be listed.
