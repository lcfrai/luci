### Task 26: the change feed — route 43 (graft G3)

**Lane A. Depends on: 16. About 2 hours. Contract task H7.**

`OD-3`: a teammate who opens the project sees the same brief, the same approval, the same pin.
The Hub pushes nothing in v1; the control plane polls **one ordered feed per user** every 5 s
(task 36), and the desktop invalidates what an event names (task 40). The feed **is** the audit
log, filtered (G3): every write appended its events under the org's lock in commit order (task
14), so a reader can never see event *n+1* before event *n* is committed.

**Files:**
- `control-plane/internal/hub/server/changes.go` + `changes_integration_test.go` (new)
- `control-plane/internal/hub/store/changes.go` (new)
- `control-plane/internal/hub/server/matrix_integration_test.go` — `rowsTask26` (modify)
- `control-plane/internal/hub/server/server.go` — `s.registerChanges()` (modify: one line)

#### Contract — the route, complete

**Route 43 — `GET /v1/changes?after=<seq | latest>&limit=<1–500, default 200>`**, any caller.

- `after`: a non-negative decimal (the org's audit sequence) or `latest`; else 400 `field: after`.
  `limit` outside 1–500 → 400 `field: limit`.
- `after=latest` → `200 {"events":[],"next":"<head seq>","more":false}` — the control plane's first
  call after sign-in.
- Otherwise, in one read transaction: read the org's head `H` (`SELECT seq FROM hub.audit_heads`,
  no lock); select events with `after < seq ≤ H`, `feed = true`, and **visible**, ordered by `seq`,
  at most `limit + 1`. **Visible** means `team_id` is a team the caller is in, or `project_id` is a
  project of such a team (contract §7.11).
- `events`: at most `limit`, each a `hubapi.Event` whose `data` is **parsed from the stored
  canonical bytes** (the table stores no second copy) and whose `actor` is the `User` of
  `actor_user_id`, or `null` (the janitor).
- `more`: `true` when a `limit + 1`-th event exists. `next`: the last returned event's `seq` when
  `more`; otherwise **`H`** — so a caller who can see none of a busy org's events still moves its
  cursor forward and does not rescan the same range every 5 s.
- Audit-only kinds (`auth.user_provisioned`, `auth.forbidden`, `blob.uploaded`, `blob.deleted`,
  `audit.read`; `feed = false`) never appear. `brief.erased` appears **without** its `reason`
  (§8; task 27 writes that event).

**Why reading `H` first is gapless.** An event row and its head advance commit in one transaction
(task 14; the `audit_append` trigger, task 07). Under `READ COMMITTED`, any event with `seq ≤ H`
that the head statement saw committed is visible to the next statement. A writer still in flight
holds a `seq > H`, which the next poll collects.

#### Tests to write first — `changes_integration_test.go`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLatestIsTheHeadWithNoEvents` | after 3 writes, `after=latest` → no events, `next: "3"` (or the head's value) | a first poll that replays history |
| `TestEventsCarryTheirData` | a brief revision → a `brief.revised` event with `data.key`, `data.revision`, `data.digest`, `data.status` and `actor` = the writer | a feed the desktop cannot invalidate from |
| `TestOnlyVisibleEvents` | Grace's team and Out Sider's team both write; each sees only its own team's events | cross-team leakage through the feed (T4) |
| `TestAuditOnlyKindsNeverAppear` | a blob upload and a refused write → no `blob.uploaded`, no `auth.forbidden` in anyone's feed | an audit-only kind, which names routes and emails, reaching teammates |
| `TestPaging` | 5 visible events, `limit=2` → 2 + `more: true`; following `next` three times collects all 5 once each | a cursor that repeats or skips |
| `TestTheCursorPassesInvisibleEvents` | Out Sider polls after 10 of Grace's writes → no events, `next` = the head | a poller rescanning a growing range for ever |
| `TestARemovedMemberStopsSeeing` | Linus removed from T → his next poll shows none of T's later events | a former member watching a team |
| `TestGaplessUnderConcurrentWriters` | 20 goroutines write while a reader polls every 10 ms; the union of what the reader collected equals the 20 events' seqs, and within each poll seqs are strictly increasing | contract §13 H7's *"gapless under 20 concurrent writers"* |
| `TestParameterBounds` | `after=-1`, `after=abc`, `limit=0`, `limit=501` → 400, `field` named | — |

Matrix `rowsTask26` (list rows count events of team T written by the fixture):

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `GET /v1/changes?after=0` | 200 (T's events) | 200 (T's) | 200 (T's) | 200 (none) | 200 (none) | 200 (none) |

#### Steps

- [ ] **Step 1:** Re-read contract §7.11, §8, §10.2 (the feed indexes).
- [ ] **Step 2:** Tests and the row; they fail. Then `store/changes.go`, `changes.go`.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestLatestIsTheHead|TestEventsCarryTheir|TestOnlyVisibleEvents|TestAuditOnlyKinds|TestPaging|TestTheCursorPasses|TestARemovedMember|TestGaplessUnder|TestParameterBounds|TestAuthorisationMatrix|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                  # 11
```

#### What this task does not claim

- A push channel: SSE on `/v1/changes/stream` is later and additive (ADR-H1 §5.5; seam S-SSE).
- Latency: at most one poll interval plus one desktop tick (ADR-H1 §5.5), measured by task 43.
