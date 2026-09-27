### Task 5: Binding store: memory, Postgres, behaviour suite

Slice B, 3 focused hours, depends on task 03. Ask P8-4 (the cloud database's login).

A **binding** is what keysmith knows about one token: its hash, the session and generation it was minted for, the person, team and project it bills, the pod address it is bound to, the route at mint, and when it expires or was revoked (P6-D7). The gatekeeper reads it on every request through keysmith (task 12's cached `GET /v1/bindings/{keyHash}`), and keysmith writes it at every mint, refresh and revoke (task 07). This task is the store: one interface, an in-memory implementation for tests and single-process use, a Postgres implementation in the gateway database's own `keysmith` schema, the ordered migration runner task 09 extends, and one behaviour suite both implementations must pass.

**The one thing this task must not get wrong is the uniqueness rule, because it is the idempotency backbone.** Task 07 guarantees *at most one live key per session generation*: a retried mint of a generation first revokes the binding it made before (the caller never received that token), then mints again. So the store must refuse a second **unrevoked** binding of the same `(session, generation)`, and must accept one once the first is revoked. That is a partial unique index, `unique (session_id, generation) where revoked_at is null`, as task 07's gap 1 asks; the skeleton's plain `unique (session_id, generation)` would refuse the re-mint and break every retry after a timeout.

Four more rules are enforced here, not described:

1. **A revocation is never moved later.** `Revoke` and `RevokeSession` accept a time in the future (task 07's refresh schedules the old generation's end at `now + Grace`), and a second revocation only ever brings the time **earlier**: `revoked_at = least(revoked_at, at)`. A late, slow refresh can never extend a key someone else has already revoked at once.
2. **`RevokeSession` revokes every generation**, not only the newest, and a revocation is in the database, so it survives a keysmith restart and is seen by the other replica.
3. **Both stores return the same values.** Times come back in UTC, truncated to the microsecond (Postgres `timestamptz` precision), and addresses in `wire.NormalAddr` form, so a test that passes on memory passes on Postgres.
4. **A malformed binding is refused before it is stored**, by one `Validate` both stores call, with Postgres `check` constraints behind it as a backstop: a key hash that is not 64 lowercase hex characters (task 03's `HashToken`), a generation below 1, a route that is not `global` or `au`, a pod address that is zero or not normal, an expiry not after creation.

Five decisions the skeleton leaves open, made here so no later part re-derives them:

- **(a) `bindingtest` has two entry points.** `Run(t, newStore)` is the skeleton's suite, every case on a fresh empty store. `RunDurable(t, open)` adds the cases that need **a second store over the same data** (a revocation survives a restart); `open` returns a function that opens a new `Store` over one fresh backing database. Memory runs `Run` only; Postgres runs both. The shape is phase 2's `registrytest` (`control-plane/internal/registry/registrytest/registrytest.go:47`, `RunStoreContract`), a normal package importing `testing` so both test files can import it (`docs/superpowers/plans/2026-09-18-phase-2-backend-and-cockpit.md:159`).
- **(b) Each Postgres test case gets its own database.** Task 17 gives this suite the database `keysmith_test` (`task-17.md`, step 8), but `go test ./...` runs packages in parallel and task 09's tests use the same server, so a shared schema would race. The helper connects with `PANACEA_APP_GATEWAY_TEST_DATABASE_URL`, creates `kst_<16 hex>`, migrates it, and drops it `with (force)` in `t.Cleanup`. Task 17's `gateway` user can create databases (its step 8 runs `createdb -U gateway`). Without the variable the Postgres tests **skip, naming it**.
- **(c) The runner owns `keysmith.schema_version`.** `Migrate` creates the schema and the version table itself (`if not exists`) before it can ask which migrations have run, so `0001_keysmith.sql` creates only the bindings table and its indexes. Each migration runs in its own transaction under `pg_advisory_xact_lock`, so two keysmith replicas starting together apply each migration once. File names are `NNNN_name.sql`; a gap or a repeated number is a start-up error, not a skipped migration. Task 09 adds `0002_key_spend.sql` and nothing else.
- **(d) The pod address is written and read as text.** It is stored in an `inet` column (`$n::inet` on insert) and read back with `host(pod_addr)`, then parsed with `netip.ParseAddr`, so the code does not depend on which Go types the pinned pgx maps `inet` to.
- **(e) `pgx/v5` is pinned at `v5.7.1`.** Its `go` directive is `1.21` (training knowledge, **UNVERIFIED** until `go mod tidy` runs; if it asks for a newer `go` line, the directive in `control-plane/go.mod` must not move, global constraint 1, so pick the newest `v5.7.x` that fits and record it). Minimal version selection may raise `golang.org/x/text` and `golang.org/x/crypto`; step 3 records the `go.sum` diff rather than hiding it. Fetching the module needs a reachable Go proxy from the corporate network, **UNVERIFIED** here.

**Files:**
- `control-plane/internal/binding/binding.go` — `Store`, `ErrExists`, `ErrInvalid`, `Validate`, `normalize` (new)
- `control-plane/internal/binding/memstore.go` — `MemStore`, `NewMemStore` (new)
- `control-plane/internal/binding/pgstore.go` — `PGStore`, `NewPGStore` (new)
- `control-plane/internal/binding/migrate.go` — `Migrate`, `parseMigrations`, the embedded `migrations/*.sql` (new)
- `control-plane/internal/binding/migrations/0001_keysmith.sql` (new)
- `control-plane/internal/binding/bindingtest/suite.go` — `Run`, `RunDurable`, `Seed` (new)
- `control-plane/internal/binding/memstore_test.go`, `pgstore_test.go`, `migrate_test.go` (new)
- `control-plane/go.mod`, `control-plane/go.sum` — `github.com/jackc/pgx/v5 v5.7.1` (modified)

**Interfaces:**
- Produces:
  - `binding.Store` = `wire.BindingReader` + `Put(ctx, wire.Binding) error`, `BySession(ctx, sessionID) ([]wire.Binding, error)` (ordered by generation, then creation), `Revoke(ctx, keyHash string, at time.Time) error`, `RevokeSession(ctx, sessionID string, at time.Time) (int, error)`, `DeleteExpired(ctx, before time.Time) (int, error)`
  - `binding.ErrExists` (a duplicate key hash, or a second unrevoked `(session, generation)`), `binding.ErrInvalid`, `binding.Validate(wire.Binding) error`; a missing hash is `wire.ErrNotFound`
  - `binding.NewMemStore() *MemStore`, `binding.NewPGStore(*pgxpool.Pool) *PGStore`, `binding.Migrate(ctx, *pgxpool.Pool) error`
  - `bindingtest.Run(t, func(*testing.T) binding.Store)`, `bindingtest.RunDurable(t, func(*testing.T) func() binding.Store)`, `bindingtest.Seed(session string, gen int, at time.Time) wire.Binding`
- Consumes: `wire.Binding`, `wire.BindingReader`, `wire.ErrNotFound`, `wire.HashToken`, `wire.NormalAddr`, `(wire.Binding) LiveAt` (task 03); `dataclass.Route` (task 01); `github.com/jackc/pgx/v5` (`pgxpool`, `pgconn`).
- Required by: task 07 (`Store`, `ErrExists`, `NewMemStore` in its tests), task 08 (`NewPGStore`, `Migrate` at start-up, `ByKeyHash` behind `GET /v1/bindings/{keyHash}`), task 09 (`Migrate`'s ordered list, `BySession`), task 10 (the suite's memory store as its fake), task 17 (the test database).

- [ ] **Step 1: Write the behaviour suite (the failing tests)**

The suite is the specification: every case runs on a fresh store from the factory, and each names the mutant it kills. It lives in a normal package so both `memstore_test.go` and `pgstore_test.go` can import it (decision (a)).

Create `control-plane/internal/binding/bindingtest/suite.go`, in three fenced blocks, one file:

```go
// Package bindingtest is the behaviour suite every binding.Store must pass.
// It is a normal package importing testing, not a _test.go file, so that the
// memory and Postgres stores' tests (and task 10's fakes) run the same cases.
package bindingtest

import (
	"context"
	"errors"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// T0 is the suite's clock origin: UTC, whole microseconds.
var T0 = time.Date(2026, 9, 25, 1, 0, 0, 0, time.UTC)

// Seed is a valid binding of session generation gen, created at `at` and
// expiring an hour later. Its key hash is derived from session and gen.
func Seed(session string, gen int, at time.Time) wire.Binding {
	tok := "sk-pa-" + session + "-" + strconv.Itoa(gen) + "-" + at.Format(time.RFC3339Nano)
	h := wire.HashToken(tok)
	return wire.Binding{KeyHash: h, KeyID: h, SessionID: session, Generation: gen,
		Owner: wire.Owner{TID: "t-1", OID: "o-1"}, TeamID: "team-1", ProjectID: "p-1", AgentID: "a-1",
		PodAddr: netip.MustParseAddr("10.244.1.7"), RouteAtMint: dataclass.RouteGlobal,
		CreatedAt: at, ExpiresAt: at.Add(time.Hour)}
}

func put(t *testing.T, s binding.Store, b wire.Binding) {
	t.Helper()
	if err := s.Put(context.Background(), b); err != nil {
		t.Fatalf("Put(%s/%d): %v", b.SessionID, b.Generation, err)
	}
}

func get(t *testing.T, s binding.Store, hash string) wire.Binding {
	t.Helper()
	b, err := s.ByKeyHash(context.Background(), hash)
	if err != nil {
		t.Fatalf("ByKeyHash: %v", err)
	}
	return b
}

// same compares two bindings field by field, times with Equal.
func same(a, b wire.Binding) bool {
	ra, rb := a.RevokedAt, b.RevokedAt
	if (ra == nil) != (rb == nil) || (ra != nil && !ra.Equal(*rb)) {
		return false
	}
	a.RevokedAt, b.RevokedAt = nil, nil
	ta := [2]time.Time{a.CreatedAt, a.ExpiresAt}
	tb := [2]time.Time{b.CreatedAt, b.ExpiresAt}
	a.CreatedAt, a.ExpiresAt, b.CreatedAt, b.ExpiresAt = time.Time{}, time.Time{}, time.Time{}, time.Time{}
	return a == b && ta[0].Equal(tb[0]) && ta[1].Equal(tb[1])
}

// Run is the suite. newStore must return an empty store.
func Run(t *testing.T, newStore func(*testing.T) binding.Store) {
	cases := map[string]func(*testing.T, binding.Store){
		"PutThenGetRoundTrips":             testRoundTrip,
		"TimesComeBackUTCToTheMicrosecond": testTimes,
		"AMissingHashIsNotFound":           testMissing,
		"ADuplicateHashIsRefused":          testDuplicateHash,
		"ASecondLiveGenerationIsRefused":   testDuplicateLiveGeneration,
		"ARevokedGenerationMayBeReminted":  testRemint,
		"ConcurrentPutsOfOneGeneration":    testConcurrentPut,
		"RevokeSessionRevokesEveryGen":     testRevokeSession,
		"ARevocationNeverMovesLater":       testNeverLater,
		"RevokingAMissingHashIsNotFound":   testRevokeMissing,
		"DeleteExpiredDeletesOnlyExpired":  testDeleteExpired,
		"PutRefusesAnInvalidBinding":       testInvalid,
		"ReturnedBindingsAreCopies":        testCopies,
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) { tc(t, newStore(t)) })
	}
}
```

```go
// Kills: a column dropped or swapped on the way in or out, and an IPv6 pod
// address that does not survive the inet column.
func testRoundTrip(t *testing.T, s binding.Store) {
	for _, addr := range []string{"10.244.1.7", "fd00:10:244::7"} {
		b := Seed("s-"+addr, 1, T0)
		b.PodAddr = netip.MustParseAddr(addr)
		b.RouteAtMint, b.Owner, b.TeamID = dataclass.RouteAU, wire.Owner{TID: "t-9", OID: "o-9"}, "team-9"
		put(t, s, b)
		if got := get(t, s, b.KeyHash); !same(got, b) {
			t.Errorf("round trip:\n got %+v\nwant %+v", got, b)
		}
	}
}

// Kills: a memory store that keeps nanoseconds and a local zone while
// Postgres returns microseconds in UTC (the suites would disagree).
func testTimes(t *testing.T, s binding.Store) {
	syd := time.FixedZone("AEST", 10*3600)
	b := Seed("s-1", 1, T0.In(syd).Add(123456789*time.Nanosecond))
	put(t, s, b)
	got := get(t, s, b.KeyHash)
	if got.CreatedAt.Location() != time.UTC || got.CreatedAt.Nanosecond()%1000 != 0 ||
		!got.CreatedAt.Equal(b.CreatedAt.Truncate(time.Microsecond)) {
		t.Fatalf("CreatedAt came back %v", got.CreatedAt)
	}
}

// Kills: a miss that returns a zero binding and nil (the gatekeeper would
// treat an unknown token as a binding to 0.0.0.0).
func testMissing(t *testing.T, s binding.Store) {
	if _, err := s.ByKeyHash(context.Background(), wire.HashToken("never-minted")); !errors.Is(err, wire.ErrNotFound) {
		t.Fatalf("err = %v, want wire.ErrNotFound", err)
	}
}

func testDuplicateHash(t *testing.T, s binding.Store) {
	a := Seed("s-1", 1, T0)
	put(t, s, a)
	b := Seed("s-2", 1, T0)
	b.KeyHash, b.KeyID = a.KeyHash, a.KeyID
	if err := s.Put(context.Background(), b); !errors.Is(err, binding.ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
}

// Kills: losing the idempotency backbone (two live keys for one generation).
func testDuplicateLiveGeneration(t *testing.T, s binding.Store) {
	put(t, s, Seed("s-1", 1, T0))
	if err := s.Put(context.Background(), Seed("s-1", 1, T0.Add(time.Second))); !errors.Is(err, binding.ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
}

// Kills: a plain unique (session, generation), which refuses task 07's
// re-mint after it revoked the first attempt.
func testRemint(t *testing.T, s binding.Store) {
	first := Seed("s-1", 1, T0)
	put(t, s, first)
	if err := s.Revoke(context.Background(), first.KeyHash, T0); err != nil {
		t.Fatal(err)
	}
	put(t, s, Seed("s-1", 1, T0.Add(time.Second)))
	if all, _ := s.BySession(context.Background(), "s-1"); len(all) != 2 {
		t.Fatalf("BySession has %d, want 2 (one revoked, one live)", len(all))
	}
}

// Kills: check-then-insert without a lock (both concurrent Puts succeed).
func testConcurrentPut(t *testing.T, s binding.Store) {
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = s.Put(context.Background(), Seed("s-1", 1, T0.Add(time.Duration(i)*time.Second)))
		}(i)
	}
	wg.Wait()
	won := 0
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case !errors.Is(err, binding.ErrExists):
			t.Errorf("a loser got %v, want ErrExists", err)
		}
	}
	if won != 1 {
		t.Fatalf("%d concurrent Puts of one generation succeeded, want 1", won)
	}
}

// Kills: revoking only the newest generation (a refreshed session's older
// key, still in its grace, would outlive the session's deletion).
func testRevokeSession(t *testing.T, s binding.Store) {
	put(t, s, Seed("s-1", 1, T0))
	put(t, s, Seed("s-1", 2, T0.Add(45*time.Minute)))
	put(t, s, Seed("s-2", 1, T0))
	n, err := s.RevokeSession(context.Background(), "s-1", T0.Add(50*time.Minute))
	if err != nil || n != 2 {
		t.Fatalf("RevokeSession = %d, %v; want 2", n, err)
	}
	all, _ := s.BySession(context.Background(), "s-1")
	for _, b := range all {
		if b.LiveAt(T0.Add(51 * time.Minute)) {
			t.Errorf("generation %d is still live", b.Generation)
		}
	}
	if other, _ := s.BySession(context.Background(), "s-2"); len(other) != 1 || other[0].RevokedAt != nil {
		t.Error("another session was revoked too")
	}
	if n, _ := s.RevokeSession(context.Background(), "s-1", T0.Add(55*time.Minute)); n != 0 {
		t.Errorf("a second, later RevokeSession changed %d rows, want 0", n)
	}
}
```

```go
// Kills: a plain overwrite of revoked_at. A slow refresh that schedules the
// old generation's end at +5 min must not undo an immediate revoke.
func testNeverLater(t *testing.T, s binding.Store) {
	ctx := context.Background()
	b := Seed("s-1", 1, T0)
	put(t, s, b)
	for _, at := range []time.Duration{5 * time.Minute, time.Minute, 10 * time.Minute} {
		if err := s.Revoke(ctx, b.KeyHash, T0.Add(at)); err != nil {
			t.Fatal(err)
		}
	}
	if got := get(t, s, b.KeyHash); got.RevokedAt == nil || !got.RevokedAt.Equal(T0.Add(time.Minute)) {
		t.Fatalf("RevokedAt = %v, want T0+1m (the earliest)", got.RevokedAt)
	}
	if _, err := s.RevokeSession(ctx, "s-1", T0.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := get(t, s, b.KeyHash); !got.RevokedAt.Equal(T0.Add(time.Minute)) {
		t.Fatalf("RevokeSession moved RevokedAt to %v", got.RevokedAt)
	}
}

func testRevokeMissing(t *testing.T, s binding.Store) {
	if err := s.Revoke(context.Background(), wire.HashToken("never-minted"), T0); !errors.Is(err, wire.ErrNotFound) {
		t.Fatalf("err = %v, want wire.ErrNotFound", err)
	}
}

// Kills: deleting by created_at, or deleting live rows.
func testDeleteExpired(t *testing.T, s binding.Store) {
	old, live := Seed("s-1", 1, T0), Seed("s-2", 1, T0.Add(50*time.Minute))
	put(t, s, old)
	put(t, s, live)
	n, err := s.DeleteExpired(context.Background(), T0.Add(61*time.Minute))
	if err != nil || n != 1 {
		t.Fatalf("DeleteExpired = %d, %v; want 1", n, err)
	}
	if _, err := s.ByKeyHash(context.Background(), old.KeyHash); !errors.Is(err, wire.ErrNotFound) {
		t.Error("the expired binding is still there")
	}
	get(t, s, live.KeyHash)
}

// Kills: a store that trusts its caller (task 07) to have validated.
func testInvalid(t *testing.T, s binding.Store) {
	bad := map[string]func(*wire.Binding){
		"short hash":       func(b *wire.Binding) { b.KeyHash = "abc" },
		"upper-case hash":  func(b *wire.Binding) { b.KeyHash = "A" + b.KeyHash[1:] },
		"generation 0":     func(b *wire.Binding) { b.Generation = 0 },
		"no session":       func(b *wire.Binding) { b.SessionID = "" },
		"no project":       func(b *wire.Binding) { b.ProjectID = "" },
		"unknown route":    func(b *wire.Binding) { b.RouteAtMint = "eu" },
		"zero address":     func(b *wire.Binding) { b.PodAddr = netip.Addr{} },
		"mapped address":   func(b *wire.Binding) { b.PodAddr = netip.MustParseAddr("::ffff:10.244.1.7") },
		"expires at birth": func(b *wire.Binding) { b.ExpiresAt = b.CreatedAt },
	}
	for name, edit := range bad {
		b := Seed("s-1", 1, T0) // valid but for the one edit, so each case fails for its own reason
		edit(&b)
		if err := s.Put(context.Background(), b); !errors.Is(err, binding.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

// Kills: a memory store that hands out its own RevokedAt pointer.
func testCopies(t *testing.T, s binding.Store) {
	b := Seed("s-1", 1, T0)
	put(t, s, b)
	_ = s.Revoke(context.Background(), b.KeyHash, T0.Add(time.Minute))
	got := get(t, s, b.KeyHash)
	*got.RevokedAt = T0.Add(time.Hour)
	if again := get(t, s, b.KeyHash); !again.RevokedAt.Equal(T0.Add(time.Minute)) {
		t.Fatal("a caller's write reached the store")
	}
}

// RunDurable is for stores that persist. open is called once per case and
// returns a function that opens a new Store over the same, initially empty, data.
func RunDurable(t *testing.T, open func(*testing.T) func() binding.Store) {
	// Kills: a revocation held only in the process that made it (the other
	// keysmith replica, or this one after a restart, would serve the key).
	t.Run("ARevocationSurvivesANewStore", func(t *testing.T) {
		reopen := open(t)
		first := reopen()
		b := Seed("s-1", 1, T0)
		put(t, first, b)
		if _, err := first.RevokeSession(context.Background(), "s-1", T0.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if got := get(t, reopen(), b.KeyHash); got.LiveAt(T0.Add(2 * time.Minute)) {
			t.Fatal("a new store over the same data serves the revoked key")
		}
	})
}
```

Then the three test files that run it. Create `control-plane/internal/binding/memstore_test.go`:

```go
package binding_test

import (
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding/bindingtest"
)

func TestMemStoreContract(t *testing.T) {
	bindingtest.Run(t, func(*testing.T) binding.Store { return binding.NewMemStore() })
}
```

Create `control-plane/internal/binding/pgstore_test.go`. Each case gets a database of its own (decision (b)):

```go
package binding_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding/bindingtest"
)

const testDBVar = "PANACEA_APP_GATEWAY_TEST_DATABASE_URL"

// freshDB creates an empty, migrated database and returns a function that
// opens a new pool over it. The database is dropped when the test ends.
func freshDB(t *testing.T) func() *pgxpool.Pool {
	t.Helper()
	admin := os.Getenv(testDBVar)
	if admin == "" {
		t.Skipf("%s is not set; task 17 step 8 gives this suite a database", testDBVar)
	}
	ctx := context.Background()
	var r [8]byte
	_, _ = rand.Read(r[:])
	name := "kst_" + hex.EncodeToString(r[:])
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("connect with %s: %v", testDBVar, err)
	}
	if _, err := conn.Exec(ctx, "create database "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "drop database if exists "+name+" with (force)")
		conn.Close(context.Background())
	})
	u, _ := url.Parse(admin)
	u.Path = "/" + name
	open := func() *pgxpool.Pool {
		pool, err := pgxpool.New(ctx, u.String())
		if err != nil {
			t.Fatalf("pool: %v", err)
		}
		t.Cleanup(pool.Close)
		return pool
	}
	if err := binding.Migrate(ctx, open()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return open
}

func TestPGStoreContract(t *testing.T) {
	bindingtest.Run(t, func(t *testing.T) binding.Store { return binding.NewPGStore(freshDB(t)()) })
}

func TestPGStoreDurable(t *testing.T) {
	bindingtest.RunDurable(t, func(t *testing.T) func() binding.Store {
		open := freshDB(t)
		return func() binding.Store { return binding.NewPGStore(open()) }
	})
}

// Kills: a runner that re-applies a migration on a second start (0001's
// create table would fail), and one that records nothing.
func TestMigrateIsIdempotent(t *testing.T) {
	open := freshDB(t)
	pool := open()
	if err := binding.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	var versions []int32
	rows, err := pool.Query(context.Background(), "select version from keysmith.schema_version order by version")
	if err == nil {
		versions, err = pgx.CollectRows(rows, pgx.RowTo[int32])
	}
	if err != nil || len(versions) == 0 || versions[0] != 1 {
		t.Fatalf("schema_version = %v, %v; want it to start at 1", versions, err)
	}
}
```

The drop runs on the admin connection after the pools close: `t.Cleanup` runs in reverse order, so each pool's `Close` (registered later) runs first, and `with (force)` (Postgres 13 and later) ends any connection left over.

Create `control-plane/internal/binding/migrate_test.go`, which needs no database:

```go
package binding

import (
	"testing"
	"testing/fstest"
)

// Kills: a runner that trusts directory order, skips a gap, applies two
// files with one number, or silently ignores a misnamed .sql file.
func TestParseMigrations(t *testing.T) {
	file := func(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }
	good := fstest.MapFS{
		"migrations/0002_key_spend.sql": file("select 2;"),
		"migrations/0001_keysmith.sql":  file("select 1;"),
		"migrations/README.md":          file("not a migration"),
	}
	ms, err := parseMigrations(good)
	if err != nil || len(ms) != 2 || ms[0].version != 1 || ms[1].version != 2 || ms[1].sql != "select 2;" {
		t.Fatalf("parseMigrations = %+v, %v", ms, err)
	}
	for name, fsys := range map[string]fstest.MapFS{
		"a gap":         {"migrations/0001_a.sql": file("x"), "migrations/0003_c.sql": file("x")},
		"a repeat":      {"migrations/0001_a.sql": file("x"), "migrations/0001_b.sql": file("x")},
		"not from 1":    {"migrations/0002_a.sql": file("x")},
		"a bad name":    {"migrations/0001_a.sql": file("x"), "migrations/2_b.sql": file("x")},
		"an empty file": {"migrations/0001_a.sql": file("  \n")},
		"none at all":   {"migrations/README.md": file("x")},
	} {
		if _, err := parseMigrations(fsys); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Kills: an embedded set that does not parse (the binary would fail at start).
func TestTheEmbeddedMigrationsParse(t *testing.T) {
	ms, err := parseMigrations(migrationFS)
	if err != nil || len(ms) == 0 || ms[0].version != 1 {
		t.Fatalf("embedded migrations: %+v, %v", ms, err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/binding/... -count=1`

Expected: the build fails, because nothing in the package exists yet. The first lines:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding/bindingtest
control-plane/internal/binding/bindingtest/suite.go:15:2: no required module provides package github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding; ...
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding [setup failed]
```

The exact wording varies (`no Go files`, `undefined: parseMigrations`, or `no required module provides package github.com/jackc/pgx/v5`); the pass condition is a build or setup failure naming this package or `pgx`, not a test failure.

- [ ] **Step 3: Add `pgx/v5`**

```bash
cd control-plane && go get github.com/jackc/pgx/v5@v5.7.1 && git diff --stat go.mod go.sum && grep -n '^go ' go.mod
```

Expected: `go.mod` gains `github.com/jackc/pgx/v5 v5.7.1` (move it into the first `require` block, sorted, beside `sigs.k8s.io/yaml` if task 02 has landed) and a few `// indirect` lines (`github.com/jackc/pgpassfile`, `github.com/jackc/pgservicefile`, `github.com/jackc/puddle/v2`, and possibly higher `golang.org/x/crypto`, `golang.org/x/text`, `golang.org/x/sync`); the last command prints `3:go 1.23`, unchanged. If `go get` tries to change the `go` line or add a `toolchain` line, stop and pick an older `v5.7.x` (decision (e)). If the proxy is unreachable from the corporate network, stop and report; do not vendor by hand.

- [ ] **Step 4: Write the interface and the memory store**

Create `control-plane/internal/binding/binding.go`:

```go
// Package binding stores what keysmith knows about each token (P6-D7): the
// key hash, the session generation, who and what it bills, the pod it is
// bound to, the route at mint, expiry and revocation. Two implementations,
// memory and Postgres, pass one behaviour suite (bindingtest).
package binding

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// Store is the binding store. A missing key hash is wire.ErrNotFound.
type Store interface {
	wire.BindingReader
	// Put stores a new binding. ErrExists: the key hash is taken, or the
	// session generation already has an unrevoked binding. ErrInvalid: Validate.
	Put(ctx context.Context, b wire.Binding) error
	// BySession is every binding of a session, by generation then creation.
	BySession(ctx context.Context, sessionID string) ([]wire.Binding, error)
	// Revoke sets RevokedAt to the earlier of its current value and at.
	Revoke(ctx context.Context, keyHash string, at time.Time) error
	// RevokeSession does the same for every generation of a session and
	// returns how many bindings it changed (0 for an unknown session).
	RevokeSession(ctx context.Context, sessionID string, at time.Time) (int, error)
	// DeleteExpired removes bindings whose ExpiresAt is before `before`.
	DeleteExpired(ctx context.Context, before time.Time) (int, error)
}

var (
	ErrExists  = errors.New("binding: already exists")
	ErrInvalid = errors.New("binding: invalid")
)

func isLowerHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Validate is the rule both stores apply before they store anything.
func Validate(b wire.Binding) error {
	switch {
	case !isLowerHex64(b.KeyHash):
		return fmt.Errorf("%w: keyHash is not 64 lowercase hex characters", ErrInvalid)
	case b.KeyID == "" || b.SessionID == "" || b.TeamID == "" || b.ProjectID == "" || b.AgentID == "":
		return fmt.Errorf("%w: keyId, sessionId, teamId, projectId and agentId are required", ErrInvalid)
	case b.Owner.TID == "" || b.Owner.OID == "":
		return fmt.Errorf("%w: owner.tid and owner.oid are required", ErrInvalid)
	case b.Generation < 1:
		return fmt.Errorf("%w: generation must be 1 or more", ErrInvalid)
	case !b.RouteAtMint.Valid():
		return fmt.Errorf("%w: routeAtMint %q is not global or au", ErrInvalid, b.RouteAtMint)
	case !b.PodAddr.IsValid() || b.PodAddr != wire.NormalAddr(b.PodAddr) || b.PodAddr.IsUnspecified():
		return fmt.Errorf("%w: podAddr must be a plain address", ErrInvalid)
	case !b.ExpiresAt.After(b.CreatedAt):
		return fmt.Errorf("%w: expiresAt must be after createdAt", ErrInvalid)
	}
	return nil
}

// micro is the one time form both stores return (Postgres precision, UTC).
func micro(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }

// normalize returns a copy in the stored form, sharing no pointer with b.
func normalize(b wire.Binding) wire.Binding {
	b.CreatedAt, b.ExpiresAt = micro(b.CreatedAt), micro(b.ExpiresAt)
	if b.RevokedAt != nil {
		r := micro(*b.RevokedAt)
		b.RevokedAt = &r
	}
	return b
}
```

Create `control-plane/internal/binding/memstore.go`:

```go
package binding

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// MemStore is the in-memory Store: tests, and a single keysmith process
// with nothing to survive. One mutex; the check and the insert are atomic.
type MemStore struct {
	mu     sync.Mutex
	byHash map[string]wire.Binding
}

func NewMemStore() *MemStore { return &MemStore{byHash: make(map[string]wire.Binding)} }

var _ Store = (*MemStore)(nil)

func (m *MemStore) Put(_ context.Context, b wire.Binding) error {
	if err := Validate(b); err != nil {
		return err
	}
	b = normalize(b)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, taken := m.byHash[b.KeyHash]; taken {
		return ErrExists
	}
	for _, o := range m.byHash { // the partial unique index, by hand
		if o.SessionID == b.SessionID && o.Generation == b.Generation && o.RevokedAt == nil && b.RevokedAt == nil {
			return ErrExists
		}
	}
	m.byHash[b.KeyHash] = b
	return nil
}

func (m *MemStore) ByKeyHash(_ context.Context, keyHash string) (wire.Binding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.byHash[keyHash]
	if !ok {
		return wire.Binding{}, wire.ErrNotFound
	}
	return normalize(b), nil // a copy: RevokedAt is re-allocated
}

func (m *MemStore) BySession(_ context.Context, sessionID string) ([]wire.Binding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []wire.Binding
	for _, b := range m.byHash {
		if b.SessionID == sessionID {
			out = append(out, normalize(b))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Generation != out[j].Generation {
			return out[i].Generation < out[j].Generation
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// revokeLocked applies least(revoked_at, at) and reports whether it changed.
func revokeLocked(b *wire.Binding, at time.Time) bool {
	at = micro(at)
	if b.RevokedAt != nil && !at.Before(*b.RevokedAt) {
		return false
	}
	b.RevokedAt = &at
	return true
}

func (m *MemStore) Revoke(_ context.Context, keyHash string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.byHash[keyHash]
	if !ok {
		return wire.ErrNotFound
	}
	revokeLocked(&b, at)
	m.byHash[keyHash] = b
	return nil
}

func (m *MemStore) RevokeSession(_ context.Context, sessionID string, at time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for h, b := range m.byHash {
		if b.SessionID == sessionID && revokeLocked(&b, at) {
			m.byHash[h] = b
			n++
		}
	}
	return n, nil
}

func (m *MemStore) DeleteExpired(_ context.Context, before time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for h, b := range m.byHash {
		if b.ExpiresAt.Before(before) {
			delete(m.byHash, h)
			n++
		}
	}
	return n, nil
}
```

The linear scan in `Put` is deliberate: a memory store holds one process's live sessions (hundreds), and a second index is a second thing to keep consistent.

- [ ] **Step 5: Write the migration and its runner**

Create `control-plane/internal/binding/migrations/0001_keysmith.sql`. The runner has already created the schema and `keysmith.schema_version` (decision (c)):

```sql
-- 0001: the binding of each session token (P6-D7). Owned by keysmith;
-- LiteLLM owns schema public of the same database.
create table keysmith.bindings (
    key_hash      text        primary key check (key_hash ~ '^[0-9a-f]{64}$'),
    key_id        text        not null,
    session_id    text        not null,
    generation    integer     not null check (generation >= 1),
    owner_tid     text        not null,
    owner_oid     text        not null,
    team_id       text        not null,
    project_id    text        not null,
    agent_id      text        not null,
    pod_addr      inet        not null,
    route_at_mint text        not null check (route_at_mint in ('global', 'au')),
    created_at    timestamptz not null,
    expires_at    timestamptz not null,
    revoked_at    timestamptz,
    check (expires_at > created_at)
);

-- At most one unrevoked binding per session generation: the idempotency
-- backbone. A revoked row leaves the index, so task 07's re-mint can land.
create unique index bindings_live_generation
    on keysmith.bindings (session_id, generation) where revoked_at is null;

create index bindings_by_session on keysmith.bindings (session_id);
create index bindings_by_expiry  on keysmith.bindings (expires_at);
```

Create `control-plane/internal/binding/migrate.go`:

```go
package binding

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrationLock is the pg_advisory_xact_lock key: two replicas starting
// together apply each migration once. An arbitrary constant, "keysmith".
const migrationLock = 0x6b657973

var migrationName = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)

type migration struct {
	version int
	name    string
	sql     string
}

// parseMigrations reads migrations/*.sql and returns them in version order.
// Versions must run 1, 2, 3… with no gap and no repeat; any other .sql name
// is an error, and a file that is only whitespace is an error.
func parseMigrations(fsys fs.FS) ([]migration, error) {
	names, err := fs.Glob(fsys, "migrations/*.sql")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, n := range names {
		m := migrationName.FindStringSubmatch(path.Base(n))
		if m == nil {
			return nil, fmt.Errorf("binding: migration %q is not named NNNN_name.sql", n)
		}
		body, err := fs.ReadFile(fsys, n)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(string(body)) == "" {
			return nil, fmt.Errorf("binding: migration %q is empty", n)
		}
		v, _ := strconv.Atoi(m[1])
		out = append(out, migration{version: v, name: path.Base(n), sql: string(body)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	if len(out) == 0 {
		return nil, fmt.Errorf("binding: no migrations")
	}
	for i, m := range out {
		if m.version != i+1 {
			return nil, fmt.Errorf("binding: migration %s: want version %d (no gap, no repeat)", m.name, i+1)
		}
	}
	return out, nil
}

const bootstrap = `create schema if not exists keysmith;
create table if not exists keysmith.schema_version (
    version    integer primary key,
    name       text not null,
    applied_at timestamptz not null default now()
)`

// Migrate applies every migration not yet recorded, each in its own
// transaction under the advisory lock. Running it again changes nothing.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	ms, err := parseMigrations(migrationFS)
	if err != nil {
		return err
	}
	for _, m := range ms {
		if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "select pg_advisory_xact_lock($1)", int64(migrationLock)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, bootstrap); err != nil {
				return err
			}
			var done bool
			if err := tx.QueryRow(ctx, "select exists(select 1 from keysmith.schema_version where version = $1)", m.version).Scan(&done); err != nil || done {
				return err
			}
			if _, err := tx.Exec(ctx, m.sql); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, "insert into keysmith.schema_version (version, name) values ($1, $2)", m.version, m.name)
			return err
		}); err != nil {
			return fmt.Errorf("binding: migration %s: %w", m.name, err)
		}
	}
	return nil
}
```

`tx.Exec` with no arguments sends the text on the simple query protocol, so a file of several statements runs as one (pgx v5's documented behaviour for argument-free `Exec`, **UNVERIFIED** against `v5.7.1` until step 7 runs the Postgres suite; `TestMigrateIsIdempotent` fails loudly if not).

- [ ] **Step 6: Write the Postgres store**

Create `control-plane/internal/binding/pgstore.go`:

```go
package binding

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// PGStore keeps bindings in keysmith.bindings. Migrate must have run.
type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

var _ Store = (*PGStore)(nil)

const columns = `key_hash, key_id, session_id, generation, owner_tid, owner_oid, team_id,
	project_id, agent_id, host(pod_addr), route_at_mint, created_at, expires_at, revoked_at`

func (s *PGStore) Put(ctx context.Context, b wire.Binding) error {
	if err := Validate(b); err != nil {
		return err
	}
	b = normalize(b)
	_, err := s.pool.Exec(ctx, `insert into keysmith.bindings (key_hash, key_id, session_id, generation,
		owner_tid, owner_oid, team_id, project_id, agent_id, pod_addr, route_at_mint, created_at, expires_at, revoked_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::inet, $11, $12, $13, $14)`,
		b.KeyHash, b.KeyID, b.SessionID, b.Generation, b.Owner.TID, b.Owner.OID, b.TeamID, b.ProjectID,
		b.AgentID, b.PodAddr.String(), string(b.RouteAtMint), b.CreatedAt, b.ExpiresAt, b.RevokedAt)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" { // unique_violation: the hash or the live generation
		return ErrExists
	}
	if err != nil {
		return fmt.Errorf("binding: put: %w", err)
	}
	return nil
}

func scan(row pgx.Row) (wire.Binding, error) {
	var b wire.Binding
	var addr, route string
	err := row.Scan(&b.KeyHash, &b.KeyID, &b.SessionID, &b.Generation, &b.Owner.TID, &b.Owner.OID, &b.TeamID,
		&b.ProjectID, &b.AgentID, &addr, &route, &b.CreatedAt, &b.ExpiresAt, &b.RevokedAt)
	if err != nil {
		return wire.Binding{}, err
	}
	if b.PodAddr, err = netip.ParseAddr(addr); err != nil {
		return wire.Binding{}, fmt.Errorf("binding: stored pod_addr %q: %w", addr, err)
	}
	b.RouteAtMint = dataclass.Route(route) // the check constraint admits only global and au
	return normalize(b), nil                // pgx returns local time; the suite wants UTC
}

func (s *PGStore) ByKeyHash(ctx context.Context, keyHash string) (wire.Binding, error) {
	b, err := scan(s.pool.QueryRow(ctx, `select `+columns+` from keysmith.bindings where key_hash = $1`, keyHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return wire.Binding{}, wire.ErrNotFound
	}
	if err != nil {
		return wire.Binding{}, fmt.Errorf("binding: by key hash: %w", err)
	}
	return b, nil
}

func (s *PGStore) BySession(ctx context.Context, sessionID string) ([]wire.Binding, error) {
	rows, err := s.pool.Query(ctx, `select `+columns+` from keysmith.bindings
		where session_id = $1 order by generation, created_at`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("binding: by session: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (wire.Binding, error) { return scan(r) })
	if err != nil {
		return nil, fmt.Errorf("binding: by session: %w", err)
	}
	return out, nil
}

// Revoke: least() ignores a NULL, so a first revocation sets `at` and a later
// one can only bring it earlier.
func (s *PGStore) Revoke(ctx context.Context, keyHash string, at time.Time) error {
	tag, err := s.pool.Exec(ctx, `update keysmith.bindings set revoked_at = least(revoked_at, $2)
		where key_hash = $1`, keyHash, micro(at))
	if err != nil {
		return fmt.Errorf("binding: revoke: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return wire.ErrNotFound
	}
	return nil
}

func (s *PGStore) RevokeSession(ctx context.Context, sessionID string, at time.Time) (int, error) {
	tag, err := s.pool.Exec(ctx, `update keysmith.bindings set revoked_at = $2
		where session_id = $1 and (revoked_at is null or revoked_at > $2)`, sessionID, micro(at))
	if err != nil {
		return 0, fmt.Errorf("binding: revoke session: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (s *PGStore) DeleteExpired(ctx context.Context, before time.Time) (int, error) {
	tag, err := s.pool.Exec(ctx, `delete from keysmith.bindings where expires_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("binding: delete expired: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
```

No error string here quotes a binding's values except a stored address; none holds a token (the table has only hashes).

- [ ] **Step 7: Run the tests to verify they pass, both stores**

First without a database: the memory suite and the migration parser run, the Postgres tests skip and say why.

```bash
cd control-plane && go test -race ./internal/binding/... -count=1 -v 2>&1 | grep -E '^(--- (PASS|FAIL|SKIP): Test|ok|FAIL)|is not set'
```

Expected:

```
--- PASS: TestMemStoreContract (0.01s)
--- PASS: TestParseMigrations (0.00s)
--- PASS: TestTheEmbeddedMigrationsParse (0.00s)
    pgstore_test.go:25: PANACEA_APP_GATEWAY_TEST_DATABASE_URL is not set; task 17 step 8 gives this suite a database
--- SKIP: TestPGStoreContract (0.00s)
--- SKIP: TestPGStoreDurable (0.00s)
--- SKIP: TestMigrateIsIdempotent (0.00s)
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding
```

(`TestPGStoreContract` reports its skip once per subtest; the line numbers are indicative. The skip lines may print in a different order.)

Then with task 17's Postgres (its step 8; the password is the local `secrets/postgres_password`, never pasted into a file):

```bash
cd control-plane && PANACEA_APP_GATEWAY_TEST_DATABASE_URL="postgresql://gateway:$(cat ../deploy/local/gateway/secrets/postgres_password)@127.0.0.1:55432/keysmith_test?sslmode=disable" \
  go test -race ./internal/binding/... -count=1 -run 'PG|Migrate' -v 2>&1 | grep -E '^(--- (PASS|FAIL|SKIP): Test|ok|FAIL)'
```

Expected:

```
--- PASS: TestPGStoreContract (1.20s)
--- PASS: TestPGStoreDurable (0.15s)
--- PASS: TestMigrateIsIdempotent (0.12s)
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding
```

If task 17 has not landed, record the Postgres run as **not run** in the commit body; the memory run is still required. The secret file name and the published port `55432` are task 17's (its step 8, `compose.testdb.yaml`); follow task 17 if it names them otherwise.

- [ ] **Step 8: Mutation spot-check, by hand, then revert**

| Edit | Test that must fail |
|---|---|
| `0001_keysmith.sql`: drop `where revoked_at is null` from the unique index | `ARevokedGenerationMayBeReminted` (Postgres) |
| `MemStore.Put`: delete the `(session, generation)` loop | `ASecondLiveGenerationIsRefused`, `ConcurrentPutsOfOneGeneration` (memory) |
| `PGStore.Revoke`: `least(revoked_at, $2)` → `$2` | `ARevocationNeverMovesLater` (Postgres) |
| `revokeLocked`: delete the early `return false` | `ARevocationNeverMovesLater` (memory) |
| `PGStore.RevokeSession`: add `and generation = (select max(generation) … )` | `RevokeSessionRevokesEveryGen` (Postgres) |
| `MemStore.ByKeyHash`: `return normalize(b), nil` → `return b, nil` | `ReturnedBindingsAreCopies` (memory) |
| `scan`: `return normalize(b), nil` → `return b, nil` | `TimesComeBackUTCToTheMicrosecond` (Postgres) |
| `parseMigrations`: delete the `m.version != i+1` loop | `TestParseMigrations` (`a gap`, `a repeat`, `not from 1`) |

- [ ] **Step 9: Format, vet and confirm the import set**

```bash
cd control-plane && gofmt -l internal/binding && go vet ./internal/binding/...
go list -deps ./internal/binding | grep -E 'kpmg-au-emu|jackc' | sort
```

Expected: nothing from `gofmt` and `go vet`, then the three `kpmg-au-emu` packages (`dataclass`, `inference/wire`, `binding`) and `github.com/jackc/pgx/v5` with its own subpackages (`pgconn`, `pgxpool`, `pgtype`, `internal/…`), and `github.com/jackc/pgpassfile`, `pgservicefile`, `puddle/v2`. Any other `kpmg-au-emu` line is a dependency this task did not plan.

- [ ] **Step 10: Commit**

```bash
git add control-plane/internal/binding control-plane/go.mod control-plane/go.sum
git commit -F /tmp/task-05-msg.txt -- control-plane/internal/binding control-plane/go.mod control-plane/go.sum
```

with `/tmp/task-05-msg.txt`:

```
feat(binding): the binding store, memory and Postgres, one suite

One Store interface, an in-memory implementation and a Postgres one in
the gateway database's keysmith schema, both passing bindingtest. At
most one unrevoked binding per session generation (a partial unique
index), so task 07's re-mint after revoking a lost first attempt lands;
a revocation only ever moves earlier; RevokeSession takes every
generation; times return UTC to the microsecond and addresses normal.

Migrate applies NNNN_name.sql files in order, once each, under an
advisory lock; a gap or a repeat is an error. Adds jackc/pgx/v5 v5.7.1.
Postgres suite: <run against task 17's database | not run: task 17 not merged>.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
```

**Acceptance:** step 7's first block without a database (and its second with one, or recorded as not run); each row of step 8 turns its test red and reverting turns it green; step 9 prints nothing and then the expected packages; `go.mod`'s `go 1.23` line is unchanged.

**What this task does not claim:**

- **The cloud database's login** (ask P8-4). Phase 8 makes Postgres Entra-only (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:452`, "Flexible Server, Entra-only, private endpoint", verified 2026-09-25), and whether keysmith reaches it with a password kept in Key Vault or an Entra token is phase 8's decision. This store takes a `*pgxpool.Pool` and does not care; task 08 builds the pool from `PANACEA_APP_GATEWAY_DATABASE_URL_FILE`.
- **That one live binding per generation means one live key.** A scheduled revocation (a refresh's grace) takes a row out of the unique index while it still works, so for up to `Grace` a session has two usable keys: generation `g-1` in its grace and `g`. They are different generations; task 07 relies on exactly that.
- **Deletion of LiteLLM keys.** Revoking a binding makes the gatekeeper refuse the token (task 10); deleting the key inside LiteLLM is task 07's, best effort.
- **That `DeleteExpired` keeps spend.** Spend lives in task 09's `keysmith.key_spend`, which is not keyed to this table by a foreign key, so deleting a binding loses nothing task 09 needs. Who calls `DeleteExpired`, and how often, is task 08's.
- **That pgx `v5.7.1` fits `go 1.23` without a toolchain line**, or that the Go proxy is reachable from the corporate network (decision (e)); step 3 finds out.

**Gaps against the skeleton, for the orchestrator:**

1. **The unique rule is `unique (session_id, generation) where revoked_at is null`** (task 07's gap 1), not the skeleton's plain `unique (session, generation)`, and `ErrExists` means "a second **unrevoked** binding of the generation".
2. **`Revoke` and `RevokeSession` take a future time and never move a revocation later** (`least(revoked_at, at)`), as task 07's decision (c) needs; the skeleton's signatures are unchanged.
3. **`keysmith.schema_version` is created by the runner, not by `0001`** (decision (c)), and it gains a `name` column. Task 09's `0002_key_spend.sql` must not create either.
4. **`bindingtest.RunDurable`** is added beside the skeleton's `Run` (decision (a)), and `ErrInvalid` and `Validate` are additions.
5. **Each Postgres case creates its own database** inside task 17's `keysmith_test` server (decision (b)); task 09's Postgres tests should do the same to avoid racing on the shared schema.
