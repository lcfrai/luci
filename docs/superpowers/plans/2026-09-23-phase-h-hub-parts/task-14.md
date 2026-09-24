### Task 14: the one transaction shape — principal, org, audit lock, idempotency, events (contract §10.6)

**Lane A. Depends on: 02, 13. About 3 hours. Contract tasks H2/H7 (G3 and G5's Go half).**

Every `/v1` handler runs inside one of two helpers. `Read` is §5.1 steps 5–6 and the handler.
`Write` is contract §10.6 exactly: resolve the principal, set the row-level-security key, **lock
the org's audit head first** (one lock order, so writes cannot deadlock, and the change feed is
gapless), claim the idempotency key, run the handler, append one audit event per changed record,
store the response, commit — and only then write the response to the wire, so a commit that fails
is never reported as a success.

**Files:**
- `control-plane/internal/hub/server/tx.go` + `tx_integration_test.go` (new)
- `control-plane/internal/hub/server/apierror.go` + `apierror_test.go` (new)
- `control-plane/internal/hub/store/audit.go` + `audit_test.go` (new)
- `control-plane/internal/hub/store/idempotency.go` (new)

#### Contract — complete

```go
// control-plane/internal/hub/server/apierror.go
package server

// APIError is how a handler refuses: the helpers turn it into the problem body,
// roll the transaction back, and store nothing.
type APIError struct {
	Status  int
	Code    string // a hubapi.Code* constant
	Message string // one human sentence, safe to render (contract §4.1)
	Details map[string]string
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

// Refuse builds one. Handlers write `return 0, nil, server.Refuse(409, hubapi.CodeRevisionConflict, …)`.
func Refuse(status int, code, message string, details map[string]string) *APIError

// Forbidden is 403 forbidden, details.required = required; it also records the
// auth.forbidden audit event (contract §8) in its own short transaction, because
// the refused write's transaction rolls back.
func Forbidden(required string) *APIError
```

```go
// control-plane/internal/hub/server/tx.go
package server

// Principal is the caller after contract §5.1 steps 5–6.
type Principal struct {
	OrgID, UserID      string
	TenantID, ObjectID string
	Name, Email        string
	OrgAdmin           bool
}

// Tx is one request's transaction.
type Tx struct {
	pgx.Tx
	P   Principal
	Now time.Time // the transaction's clock, truncated to the millisecond
	// unexported: the locked audit head (Write only), the pending events
}

// Event is one audit event a write records. Emit appends to the pending list;
// Write inserts them in call order after fn returns, before COMMIT.
type Event struct {
	Kind      string // a contract §8 kind
	TeamID    string // "" when none
	ProjectID string // "" when none
	Subject   string
	Data      map[string]any // strings, int64s and bools only
}

func (tx *Tx) Emit(e Event)

// Read: BEGIN; store.ResolvePrincipal; set_config('hub.org_id', org, true); if the
// principal was created by this call, lock the audit head and append
// auth.user_provisioned; fn; COMMIT. ErrNotEnrolled → 403 tenant_not_enrolled.
func (s *Server) Read(ctx context.Context, fn func(tx *Tx) error) error

// WriteResult is what a write handler answers; Write sends it after COMMIT.
type WriteResult struct {
	Status int
	Body   any    // JSON; nil for 204
	Header http.Header // extra headers, e.g. none today
}

// Write is contract §10.6:
//   BEGIN
//   store.ResolvePrincipal            → ErrNotEnrolled: 403 tenant_not_enrolled
//   set_config('hub.org_id', org, true)
//   store.LockHead(org)               -- SELECT … FROM hub.audit_heads … FOR UPDATE
//   if POST and Idempotency-Key:      -- the key must match ^[A-Za-z0-9_-]{1,64}$, else 400 bad_request
//       store.ClaimKey(…)             → seen with the same method, path and request hash:
//                                        ROLLBACK and answer the stored status and body,
//                                        with Idempotent-Replay: true
//                                     → seen with a different one: 422 idempotency_mismatch
//   fn(tx)                            → *APIError: ROLLBACK, answer the problem
//   store.Append(each pending event)
//   store.StoreResponse(key, status, body)   (2xx only; a refusal is never stored. Route 26's
//                                        body carries brief text; hub.erase_brief deletes every
//                                        stored body naming the brief, A17, task 07)
//   COMMIT                            → failure: 500 internal (nothing was sent yet)
//   write the response
// The request body (≤ httpx.MaxBodyBytes) is read once, before BEGIN, into a
// buffer the handler decodes from, so the request hash covers exactly the bytes
// the handler saw. Blob uploads use WriteStream (task 21), which reads no body here.
func (s *Server) Write(w http.ResponseWriter, r *http.Request, fn func(tx *Tx, body []byte) (WriteResult, error))

// WriteLimit is Write with the body cap named: Write is WriteLimit(…,
// httpx.MaxBodyBytes, …). Only route 26 passes another (hubapi.MaxRevisionBatchBytes,
// HA4-3, task 00). A body over limit is 400 bad_request, field "body", the
// sentence naming limit in bytes — refused before BEGIN, so nothing is locked.
func (s *Server) WriteLimit(w http.ResponseWriter, r *http.Request, limit int64, fn func(tx *Tx, body []byte) (WriteResult, error))

// Decode is httpx.DecodeJSON's rules over the buffered body: unknown fields are
// 400 bad_request, malformed JSON 400 invalid_json.
func Decode[T any](body []byte, dst *T) error // returns an *APIError
```

```go
// control-plane/internal/hub/store/audit.go
package store

type Head struct {
	OrgID string
	Seq   int64
	Hash  [32]byte
}

// LockHead is the org's write lock: the first statement after set_config in every write.
func LockHead(ctx context.Context, tx pgx.Tx, orgID string) (*Head, error)

type AuditRow struct {
	At          time.Time // truncated to the millisecond, so the stored timestamptz and the
	                      // canonical "at" string are the same instant
	Kind        string
	TeamID      string
	ProjectID   string
	Subject     string
	ActorUserID string // "" for the janitor
	Data        map[string]any
}

// Append builds hubapi.AuditEnvelope{Seq: head.Seq+1, …} (with teamId, correction A10),
// computes hubapi.ChainHash(head.Hash, canonical), INSERTs the row with
// feed = InFeed(kind), and advances head in memory. The database trigger
// advances hub.audit_heads and refuses anything out of order.
func Append(ctx context.Context, tx pgx.Tx, head *Head, row AuditRow) error

// InFeed is contract §8's last column: false for auth.user_provisioned,
// auth.forbidden, blob.uploaded, blob.deleted and audit.read; true for every other kind.
func InFeed(kind string) bool
```

```go
// control-plane/internal/hub/store/idempotency.go
package store

type Claim struct {
	Replay         bool   // true: answer Status and Response, run nothing
	Mismatch       bool   // true: 422 idempotency_mismatch
	Status         int
	Response       []byte
}

// ClaimKey inserts (org, user, key, method, path, request_sha256) ON CONFLICT DO
// NOTHING; on conflict it reads the stored row. Because every write in the org
// holds the audit head lock first, a row seen here is always committed: there is
// no "in flight" state to wait on (the status 0 of contract §10.2 is never observed).
func ClaimKey(ctx context.Context, tx pgx.Tx, orgID, userID, key, method, path string, requestSHA [32]byte) (Claim, error)

// StoreResponse fills the claimed row's status and response.
func StoreResponse(ctx context.Context, tx pgx.Tx, userID, key string, status int, response []byte) error
```

The request hash is `SHA-256(method + " " + path-with-query + "\n" + body)`.

#### Tests to write first

Unit (`apierror_test.go`, `audit_test.go`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestAPIErrorBecomesTheProblemBody` | `Refuse(409, "revision_conflict", "…", {"headRevision":"6"})` through `Write` → the exact `httpx.Problem` JSON | a Go error chain in `message` |
| `TestWriteLimitNamesItsCap` | `WriteLimit(…, 4<<20, fn)` accepts a 2 MiB body that `Write` refuses; 4 MiB + 1 → 400 `field: body`, the sentence naming 4194304, `fn` never called and no `BEGIN` recorded | route 26's 64-revision batch refused at 1 MiB, or a cap checked after the org lock is taken (HA4-3) |
| `TestInFeedIsTheContractTable` | every kind of contract §8, row by row, with its column | an audit-only kind (`auth.forbidden`, which names routes) reaching teammates' feeds |
| `TestAppendBuildsVector3` | `Append` with vector 3's inputs (task 02) produces vector 3's canonical bytes and hash, captured through a recording `pgx.Tx` fake | the Go writer and `hub verify` disagreeing about the envelope |

Integration (`tx_integration_test.go`, `//go:build integration`, on `hubtest.Migrated` with one org
enrolled and the test route of `export_test.go` extended to write):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestTheOrgIsSetForTheTransaction` | inside `Read` and `Write`, `current_setting('hub.org_id')` = the principal's org | a handler querying before the RLS key is set, seeing nothing, and answering 404 |
| `TestTwentyConcurrentWritersAreGapless` | 20 goroutines × 1 write each → `seq` 1…20 with no gap, and each row's `hash` = `ChainHash(previous row's hash, canonical)` | `bigserial` cursors that skip rows committing out of order (G3) |
| `TestAFailedWriteLeavesNoEvent` | `fn` emits an event, then returns `Refuse(409,…)` → no row in `audit_events`, head unchanged, the 409 on the wire | an event for a change that never happened |
| `TestAReplayAnswersTheStoredResponse` | a POST with `Idempotency-Key: k1` → 201; the same again → 201, the byte-identical body, `Idempotent-Replay: true`, and **one** event in total | a retry after a lost response double-writing (the reason §1 requires the key) |
| `TestAReusedKeyWithAnotherBodyIs422` | same key, different body → 422 `idempotency_mismatch` | a replay of the wrong request's answer |
| `TestKeysArePerUser` | the same key from two users → two independent writes | one user's key answering another user's request |
| `TestARefusalIsNotStored` | key k2 → 409; the conflict is removed; k2 again → 201 | a transient refusal replayed for 24 hours |
| `TestABadKeyIs400` | `Idempotency-Key: has space` → 400 `bad_request`, `field: Idempotency-Key` | a key the table's CHECK rejects surfacing as a 500 |
| `TestAFirstReadProvisionsAndAudits` | a new principal's first `Read` → a `users` row and one `auth.user_provisioned` event with `feed = false`; its second → nothing new | provisioning that is not audited, or audited twice |
| `TestAnUnenrolledTenantIs403` | a principal whose tenant has no org → 403 `tenant_not_enrolled` from both helpers | a user created without an org |
| `TestNothingIsSentBeforeCommit` | a `fn` that succeeds but whose commit fails — `export_test.go` sets the unexported `beforeCommit` hook to return an error in place of `COMMIT` → 500 `internal`, not 201, and no event persisted | a 201 for a write that rolled back |

#### Steps

- [ ] **Step 1:** Re-read contract §1 (idempotency), §3.6, §8, §10.6; ADR-H1 G3 and G5.
- [ ] **Step 2:** Tests; they fail. Then `apierror.go`, `audit.go`, `idempotency.go`, `tx.go`.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/hub/server/ ./control-plane/internal/hub/store/ -race -count=1                         # ok
go test -tags integration ./control-plane/internal/hub/server/ -run 'Org|Concurrent|Failed|Replay|Reused|PerUser|Refusal|BadKey|FirstRead|Unenrolled|BeforeCommit' -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 11
```

#### What this task does not claim

- Any route of contract §7; the first ones are task 15's.
- Throughput under the org lock (ADR-H1 R10): writes are human-paced; measured later.
