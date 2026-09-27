### Task 09: Usage rollup and the team-budget reader

LiteLLM enforces two of the three budget dimensions natively: the person (its user budget) and the project (its team budget, since `team_id` is the project, P6-D6). The **team** dimension has no LiteLLM tier this plan can rely on, so keysmith enforces it at mint from spend it records itself (P6-D8, task 06). This task records that spend: a sweep that reads each recent key's spend from LiteLLM every 60 seconds into `keysmith.key_spend`, and a reader that sums it per person, project and team for the current UTC month. The reader is task 07's `SpentReader` and the source of keysmith's `GET /v1/usage` (task 08).

**The one thing this task must not get wrong is losing spend.** A key LiteLLM has deleted (revoked, superseded, or expired and cleaned up) can no longer be read, and a row that went to zero when its key vanished would hand that spend back to the team's budget. So a recorded spend **never decreases**: the upsert keeps the greater of the stored and the read figure, a read that fails leaves the row alone, and `key_spend` has no foreign key to `bindings`, so task 05's `DeleteExpired` cannot take spend with it. That rule has a second half the first draft missed (review M-3): the sweep runs every 60 seconds, so a key deleted between two sweeps keeps only what the last sweep saw, and keysmith deletes a key on every refresh and every revoke. `RecordKey` closes that window — keysmith calls it immediately before each `/key/delete` (task 07 decision (j)) — and it runs the same `record` the sweep runs, so there is one upsert path and one never-lower rule, not two that can drift.

LiteLLM's own spend reports (tag and team reports) are not used, because those features may be enterprise-only in the pinned image (**UNVERIFIED** until task 18). `GET /key/info`'s `spend` is the one figure this task reads, through task 04's `KeyInfo` (`task-04.md`, `KeyInfo{Spend float64, …}` and `(*Client).KeyInfo(ctx, keyID)`); that it reports a key by the hash task 07 stores is task 18's P12.

**Files:**
- `control-plane/internal/usage/usage.go`: `KeyRef`, `Record`, `Total`, `Store`, `KeySource`, `MemStore`, `PGStore`, `Reader`, `CacheTTL` (new)
- `control-plane/internal/usage/sweep.go`: `KeyInfoer`, `Sweeper`, `record`, `RecordKey`, `DefaultEvery`, `DefaultLookback`, `RecordTimeout` (new)
- `control-plane/internal/usage/usage_test.go`: the store suite on both stores, the reader and sweeper tests (new)
- `control-plane/internal/binding/migrations/0002_key_spend.sql`: `keysmith.key_spend` and its indexes (new; task 05's `Migrate` runs it in order)

**Interfaces:**
- Produces:
  - `usage.Store` = `Upsert(ctx, Record) error` (never lowers a recorded spend) + `Sum(ctx, scope, id string, periodStart time.Time) (Total, error)`
  - `usage.KeySource` = `RecentKeys(ctx, expiringAfter time.Time) ([]KeyRef, error)`; `*usage.PGStore` implements it over `keysmith.bindings`
  - `usage.NewMemStore() *MemStore`, `usage.NewPGStore(*pgxpool.Pool) *PGStore`
  - `usage.NewReader(Store, budget.Policy, now func() time.Time) *Reader`; `(*Reader).Spent(ctx, wire.Owner, teamID, projectID string) (budget.Spent, error)` (task 07's `SpentReader`); `(*Reader).Rows(ctx, scope, id string) ([]wire.UsageRow, error)`, exactly one row, the current period's, with zero spend when nothing was spent (task 21's gap 6)
  - `usage.Sweeper{Keys KeySource; Admin KeyInfoer; Store Store; Clock func() time.Time; Every, Lookback time.Duration; Report func(error); Prices PriceReader; Groups []string}` with `Once(ctx) (int, error)`, `Run(ctx)` and `RecordKey(ctx, wire.Binding) error` (keysmith's `SpendRecorder`, task 07 decision (j)); `usage.PriceReader`, `usage.ErrUnpriced` (decision (g))
- Consumes: `budget.Policy`, `budget.Spent`, `budget.PeriodStart` (task 06, committed); `wire.Owner` and `(Owner).UserID()`, `wire.UsageRow`, `wire.ScopePerson`, `ScopeProject`, `ScopeTeam` (task 03); `litellmadmin.KeyInfo`, `litellmadmin.GroupPrice` and `(*Client).ModelPrices` (task 04; the last two added by review I-5); `keysmith.bindings`' columns `key_hash`, `key_id`, `session_id`, `owner_tid`, `owner_oid`, `team_id`, `project_id`, `created_at`, `expires_at` and `binding.Migrate` (task 05's `0001_keysmith.sql`, read in `task-05.md`); `github.com/jackc/pgx/v5` (task 05 adds it).
- Required by: task 07 (`Deps.Spent`), task 08 (`NewPGStore`, `NewReader`, `Sweeper`, `GET /v1/usage`), task 21 (the rows it shows).

**Decisions where the skeleton is silent:**

**(a) A key's spend belongs to the month it was minted in.** `PeriodStart(CreatedAt)`, UTC. A key minted at 23:30 UTC on 30 September counts wholly against September, including what it spends after midnight. Keys live one hour (`TOKEN_TTL`), so at most one TTL of each session's spend lands in the previous month. The alternative, splitting by LiteLLM's spend logs, needs a feature this task does not rely on.

**(b) The sweep reads keys whose expiry is within `Lookback` (10 minutes) of now, or later.** Live keys, and keys that expired or were revoked recently, whose last minutes of spend LiteLLM may still be writing. A revoked key LiteLLM has deleted answers an error, and its last recorded figure stands.

**(c) The sweep reads `keysmith.bindings` itself**, through `PGStore.RecentKeys`, rather than adding a method to task 05's `Store`. The query is read-only and names only the columns listed above; migration `0002` adds the `expires_at` index it needs. If task 05's table changes shape, this query's test (the PG suite) fails.

**(d) The person's id is `Owner.UserID()`** (`"<tid>:<oid>"`, task 03), stored as `user_id`: the same string LiteLLM's user budget is keyed on (task 07), so our person total and LiteLLM's describe the same person.

**(e) `Rows` returns one row.** The current period's, with `BudgetUSD` from the policy: `Limits(ctx, id, "")` for a team, `Limits(ctx, "", projectID)` for a project, `Limits(ctx, "", "")` for a person. That is exact for task 06's `Static`, which answers every project alike; a Hub policy that varies by team (S6.2) needs the project's team here, which this API does not carry (gap 3).

**(f) `Requests` is recorded as 0.** `KeyInfo` carries no request count (`task-04.md`'s `KeyInfo` has none), so the column exists, as the skeleton lists it, and stays 0 until a source is chosen (gap 4).

**(g) Every sweep checks that LiteLLM prices every group** (review I-5). A group LiteLLM would charge nothing for spends US$0, so all three budgets on it are silently off. The review asked for a flag on a key with `requests > 0` and `spend == 0`, but decision (f) records `Requests` as 0: that check would never fire. So the sweep reads the cause instead of a symptom. `Once` first calls `Prices.ModelPrices` (task 04, `GET /model/info`: what LiteLLM will charge per token, its `model_info` merged with its own table) and reports `ErrUnpriced` for each of `Groups` (task 02's `catalogue.Groups()`) that is missing or has a zero input or output price. A nil `Prices` while `Groups` is set is reported on every sweep too, so a wiring that forgets the reader is loud. The check never stops the sweep. Task 02 decision (f) is the gate at build time; this is the check at run time, after an image bump or a hand-deployed config.

- [ ] **Step 1: Preflight**

```bash
cd control-plane
test -d internal/usage && echo "STOP: internal/usage exists" || echo "ok: no usage package yet"       # expect ok
ls internal/binding/migrations/                                                                   # expect 0001_keysmith.sql only
grep -n 'type KeyInfo struct\|func (c \*Client) KeyInfo' internal/litellmadmin/client.go          # expect both (task 04)
grep -n 'func (o Owner) UserID\|ScopePerson' internal/inference/wire/wire.go                       # expect both (task 03)
```

A `0002_*` file already present means another part took the number; stop and report rather than renumbering task 05's runner.

- [ ] **Step 2: Write the failing tests**

One behaviour suite runs on both stores (the Postgres run needs task 17's compose Postgres through `PANACEA_APP_GATEWAY_TEST_DATABASE_URL`, and skips with that name otherwise, as task 05's does), then the reader and the sweeper on the memory store with fakes for LiteLLM and the key source.

Create `control-plane/internal/usage/usage_test.go`:

```go
package usage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math"
	"net/netip"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/litellmadmin"
)

var (
	ctx   = context.Background()
	sep   = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	oct   = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	alice = wire.Owner{TID: "t-1", OID: "alice"}
)

const testDBVar = "PANACEA_APP_GATEWAY_TEST_DATABASE_URL"

func hashOf(key string) string { return wire.HashToken(key) }

func rec(key, session, user, team, project string, period time.Time, usd float64) Record {
	return Record{KeyHash: hashOf(key), SessionID: session, UserID: user, TeamID: team, ProjectID: project,
		PeriodStart: period, SpendUSD: usd, UpdatedAt: period.Add(time.Hour)}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func sum(t *testing.T, s Store, scope, id string, period time.Time) float64 {
	t.Helper()
	tot, err := s.Sum(ctx, scope, id, period)
	must(t, err)
	return tot.SpendUSD
}

func static(t *testing.T) budget.Policy {
	t.Helper()
	s, err := budget.NewStatic(budget.Proposed)
	must(t, err)
	return s
}
```

```go
// runStoreSuite is the behaviour both stores pass; each case names its mutant.
func runStoreSuite(t *testing.T, newStore func(t *testing.T) Store) {
	t.Run("a lower read never lowers a recorded spend", func(t *testing.T) {
		// Mutant killed: a plain overwrite, which zeroes a key LiteLLM deleted.
		s := newStore(t)
		must(t, s.Upsert(ctx, rec("k1", "s-1", "t-1:alice", "team-1", "p-1", sep, 3)))
		must(t, s.Upsert(ctx, rec("k1", "s-1", "t-1:alice", "team-1", "p-1", sep, 0)))
		if got := sum(t, s, wire.ScopeProject, "p-1", sep); got != 3 {
			t.Errorf("after a lower read: %v, want 3", got)
		}
		must(t, s.Upsert(ctx, rec("k1", "s-1", "t-1:alice", "team-1", "p-1", sep, 5)))
		if got := sum(t, s, wire.ScopeProject, "p-1", sep); got != 5 {
			t.Errorf("after a higher read: %v, want 5", got)
		}
	})
	t.Run("spend sums across a session's generations", func(t *testing.T) {
		// Mutant killed: one row per session, the newest key's, instead of per key.
		s := newStore(t)
		must(t, s.Upsert(ctx, rec("gen1", "s-1", "t-1:alice", "team-1", "p-1", sep, 2)))
		must(t, s.Upsert(ctx, rec("gen2", "s-1", "t-1:alice", "team-1", "p-1", sep, 1.5)))
		if got := sum(t, s, wire.ScopeProject, "p-1", sep); got != 3.5 {
			t.Errorf("two generations: %v, want 3.5", got)
		}
	})
	t.Run("each scope sums its own column, and a team spans its projects", func(t *testing.T) {
		// Mutant killed: a team read that is really a project read.
		s := newStore(t)
		must(t, s.Upsert(ctx, rec("a", "s-1", "t-1:alice", "team-1", "p-1", sep, 1)))
		must(t, s.Upsert(ctx, rec("b", "s-2", "t-1:bob", "team-1", "p-2", sep, 2)))
		must(t, s.Upsert(ctx, rec("c", "s-3", "t-1:alice", "team-2", "p-3", sep, 4)))
		for _, c := range []struct {
			scope, id string
			want      float64
		}{
			{wire.ScopeTeam, "team-1", 3}, {wire.ScopeTeam, "team-2", 4}, {wire.ScopeProject, "p-2", 2},
			{wire.ScopePerson, "t-1:alice", 5}, {wire.ScopePerson, "t-1:carol", 0},
		} {
			if got := sum(t, s, c.scope, c.id, sep); got != c.want {
				t.Errorf("%s %s = %v, want %v", c.scope, c.id, got, c.want)
			}
		}
	})
	t.Run("periods are separate", func(t *testing.T) {
		// Mutant killed: a sum over every period, a budget that never resets.
		s := newStore(t)
		must(t, s.Upsert(ctx, rec("sep", "s-1", "t-1:alice", "team-1", "p-1", sep, 7)))
		must(t, s.Upsert(ctx, rec("oct", "s-2", "t-1:alice", "team-1", "p-1", oct, 1)))
		if got := sum(t, s, wire.ScopeTeam, "team-1", oct); got != 1 {
			t.Errorf("October = %v, want 1", got)
		}
	})
	t.Run("an unknown scope is refused", func(t *testing.T) {
		// Mutant killed: an unknown scope summed over the whole table.
		if _, err := newStore(t).Sum(ctx, "org", "x", sep); !errors.Is(err, ErrScope) {
			t.Errorf("scope org: err = %v, want ErrScope", err)
		}
	})
}

func TestMemStoreSuite(t *testing.T) {
	runStoreSuite(t, func(*testing.T) Store { return NewMemStore() })
}

func TestPGStoreSuite(t *testing.T) {
	if os.Getenv(testDBVar) == "" {
		t.Skip(testDBVar + " is not set")
	}
	runStoreSuite(t, func(t *testing.T) Store { return NewPGStore(freshPool(t)) })
}

// freshPool is a new database on the test server, migrated by task 05's
// runner (so 0002 runs), and dropped when the test ends.
func freshPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv(testDBVar)
	admin, err := pgx.Connect(ctx, url)
	must(t, err)
	var b [6]byte
	_, _ = rand.Read(b[:])
	name := "usage_test_" + hex.EncodeToString(b[:])
	_, err = admin.Exec(ctx, "create database "+name)
	must(t, err)
	cfg, err := pgxpool.ParseConfig(url)
	must(t, err)
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	must(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "drop database if exists "+name+" with (force)")
		_ = admin.Close(context.Background())
	})
	must(t, binding.Migrate(ctx, pool))
	return pool
}
```

```go
// Mutants killed: a key query on created_at, or on live keys only, which never
// records the last minutes of a key that has just expired or been revoked.
func TestPGRecentKeysReadsKeysExpiringAfterTheCut(t *testing.T) {
	if os.Getenv(testDBVar) == "" {
		t.Skip(testDBVar + " is not set")
	}
	pool := freshPool(t)
	now := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	bindings := binding.NewPGStore(pool)
	for i, created := range []time.Time{now.Add(-80 * time.Minute), now.Add(-65 * time.Minute), now.Add(-10 * time.Minute)} {
		key := []string{"expired-20m", "expired-5m", "live"}[i]
		must(t, bindings.Put(ctx, wire.Binding{KeyHash: hashOf(key), KeyID: hashOf(key), SessionID: "s-" + key,
			Generation: 1, Owner: alice, TeamID: "team-1", ProjectID: "p-1", AgentID: "a-1",
			PodAddr: netip.MustParseAddr("10.244.1.7"), RouteAtMint: dataclass.RouteGlobal,
			CreatedAt: created, ExpiresAt: created.Add(time.Hour)}))
	}
	refs, err := NewPGStore(pool).RecentKeys(ctx, now.Add(-DefaultLookback))
	must(t, err)
	got := map[string]KeyRef{}
	for _, r := range refs {
		got[r.KeyHash] = r
	}
	if len(got) != 2 || got[hashOf("expired-20m")].KeyHash != "" {
		t.Fatalf("RecentKeys = %+v, want expired-5m and live only", refs)
	}
	if r := got[hashOf("live")]; r.Owner != alice || r.TeamID != "team-1" || r.ProjectID != "p-1" ||
		!r.CreatedAt.Equal(now.Add(-10*time.Minute)) || r.KeyID != hashOf("live") {
		t.Errorf("live ref = %+v", r)
	}
}

// countingStore counts Sum calls, and fails them when err is set.
type countingStore struct {
	Store
	mu   sync.Mutex
	sums int
	err  error
}

func (c *countingStore) Sum(ctx context.Context, scope, id string, p time.Time) (Total, error) {
	c.mu.Lock()
	c.sums++
	err := c.err
	c.mu.Unlock()
	if err != nil {
		return Total{}, err
	}
	return c.Store.Sum(ctx, scope, id, p)
}

// Mutant killed: the month computed in local time, so a Sydney morning on
// 1 October (still 30 September in UTC) reads September's spend as gone.
func TestSpentIsTheCurrentUTCMonth(t *testing.T) {
	s := NewMemStore()
	must(t, s.Upsert(ctx, rec("sep", "s-1", alice.UserID(), "team-1", "p-1", sep, 7)))
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.FixedZone("AEST", 10*3600))
	r := NewReader(s, static(t), func() time.Time { return now })
	if got, err := r.Spent(ctx, alice, "team-1", "p-1"); err != nil || got != (budget.Spent{PersonUSD: 7, ProjectUSD: 7, TeamUSD: 7}) {
		t.Errorf("at 09:00 AEST on 1 Oct: %+v, %v; want September's 7 in each dimension", got, err)
	}
	now = time.Date(2026, 10, 1, 0, 30, 0, 0, time.UTC).Add(time.Hour) // October in UTC, cache expired
	if got, err := r.Spent(ctx, alice, "team-1", "p-1"); err != nil || got != (budget.Spent{}) {
		t.Errorf("in October UTC: %+v, %v; want zero", got, err)
	}
}

// Mutants killed: no cache (three database reads per mint), a cache that never
// expires (a team could overshoot for ever, not for 60 s), and an error read
// as zero spend, which task 07 would mint a full slice on.
func TestSpentIsCachedSixtySecondsAndFailsClosed(t *testing.T) {
	s := &countingStore{Store: NewMemStore()}
	now := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	r := NewReader(s, static(t), func() time.Time { return now })
	for i := 0; i < 2; i++ {
		if _, err := r.Spent(ctx, alice, "team-1", "p-1"); err != nil {
			t.Fatal(err)
		}
	}
	if s.sums != 3 {
		t.Errorf("two reads made %d sums, want 3 (one per dimension, then cached)", s.sums)
	}
	must(t, s.Upsert(ctx, rec("k", "s-1", alice.UserID(), "team-1", "p-1", sep, 4)))
	now = now.Add(59 * time.Second)
	if got, _ := r.Spent(ctx, alice, "team-1", "p-1"); got.TeamUSD != 0 {
		t.Errorf("at 59 s: team %v, want the cached 0", got.TeamUSD)
	}
	now = now.Add(2 * time.Second)
	if got, _ := r.Spent(ctx, alice, "team-1", "p-1"); got.TeamUSD != 4 || s.sums != 6 {
		t.Errorf("at 61 s: team %v after %d sums, want 4 after 6", got.TeamUSD, s.sums)
	}
	s.err = errors.New("database down")
	now = now.Add(61 * time.Second)
	if _, err := r.Spent(ctx, alice, "team-1", "p-1"); err == nil {
		t.Error("a failed sum answered as spend")
	}
}
```

```go
// Mutants killed: no row when nothing was spent (the cockpit cannot show the
// budget, task 21's gap 6); another dimension's budget; an unknown scope read.
func TestRowsIsTheCurrentPeriodWithItsBudget(t *testing.T) {
	s := NewMemStore()
	must(t, s.Upsert(ctx, rec("k", "s-1", alice.UserID(), "team-1", "p-1", sep, 12.5)))
	now := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	r := NewReader(s, static(t), func() time.Time { return now })
	p := budget.Proposed
	for _, c := range []struct {
		scope, id     string
		spend, budget float64
	}{
		{wire.ScopeProject, "p-1", 12.5, p.ProjectUSD},
		{wire.ScopeTeam, "team-1", 12.5, p.TeamUSD},
		{wire.ScopePerson, alice.UserID(), 12.5, p.PersonUSD},
		{wire.ScopeProject, "p-unspent", 0, p.ProjectUSD},
	} {
		rows, err := r.Rows(ctx, c.scope, c.id)
		want := []wire.UsageRow{{Scope: c.scope, ID: c.id, PeriodStart: sep, SpendUSD: c.spend, BudgetUSD: c.budget}}
		if err != nil || !reflect.DeepEqual(rows, want) {
			t.Errorf("Rows(%s, %s) = %+v, %v; want %+v", c.scope, c.id, rows, err, want)
		}
	}
	if _, err := r.Rows(ctx, "org", "x"); !errors.Is(err, ErrScope) {
		t.Errorf("Rows(org) err = %v, want ErrScope", err)
	}
}

type fakeKeys struct {
	refs  []KeyRef
	since time.Time
}

func (f *fakeKeys) RecentKeys(_ context.Context, after time.Time) ([]KeyRef, error) {
	f.since = after
	return f.refs, nil
}

// fakeInfo answers KeyInfo by key id; an absent id is a key LiteLLM deleted.
type fakeInfo map[string]float64

func (f fakeInfo) KeyInfo(_ context.Context, id string) (litellmadmin.KeyInfo, error) {
	v, ok := f[id]
	if !ok {
		return litellmadmin.KeyInfo{}, errors.New("litellmadmin: key/info: 404")
	}
	return litellmadmin.KeyInfo{Spend: v}, nil
}

func ref(key string, created time.Time) KeyRef {
	return KeyRef{KeyHash: hashOf(key), KeyID: hashOf(key), SessionID: "s-1", Owner: alice,
		TeamID: "team-1", ProjectID: "p-1", CreatedAt: created}
}

// Mutants killed: a failed read written as zero, or a deleted key's row
// dropped; either hands spent money back to the team's budget.
func TestDeletedKeyKeepsItsSpend(t *testing.T) {
	now := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	info := fakeInfo{hashOf("k1"): 3}
	store := NewMemStore()
	reported := 0
	sw := &Sweeper{Keys: &fakeKeys{refs: []KeyRef{ref("k1", now.Add(-30*time.Minute))}}, Admin: info,
		Store: store, Clock: func() time.Time { return now }, Report: func(error) { reported++ }}
	if n, err := sw.Once(ctx); err != nil || n != 1 {
		t.Fatalf("first sweep: %d, %v", n, err)
	}
	delete(info, hashOf("k1")) // LiteLLM deleted the key
	if n, err := sw.Once(ctx); err != nil || n != 0 || reported != 1 {
		t.Errorf("sweep of a deleted key: %d swept, %v, %d reported; want 0, nil, 1", n, err, reported)
	}
	info[hashOf("k1")] = 0 // or LiteLLM answers zero for it
	if _, err := sw.Once(ctx); err != nil {
		t.Fatal(err)
	}
	if got := sum(t, store, wire.ScopeTeam, "team-1", sep); got != 3 {
		t.Errorf("team spend = %v, want the 3 recorded before the delete", got)
	}
}

// Mutants killed: RecordKey that reads but does not file; one that files under
// the call's month instead of the key's; one whose failure is swallowed, which
// would tell keysmith the spend is safe when it is not (review M-3).
func TestRecordKeyFilesTheSpendASweepWouldMiss(t *testing.T) {
	now := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	info := fakeInfo{hashOf("k1"): 7}
	store := NewMemStore()
	sw := &Sweeper{Keys: &fakeKeys{}, Admin: info, Store: store, Clock: func() time.Time { return now }}
	k := ref("k1", now.Add(-30*time.Minute)) // never swept: Keys has no refs
	b := wire.Binding{KeyHash: k.KeyHash, KeyID: k.KeyID, SessionID: k.SessionID, Owner: k.Owner,
		TeamID: k.TeamID, ProjectID: k.ProjectID, CreatedAt: k.CreatedAt}
	if err := sw.RecordKey(ctx, b); err != nil {
		t.Fatal(err)
	}
	if got := sum(t, store, wire.ScopeTeam, "team-1", sep); got != 7 {
		t.Errorf("team spend = %v, want 7 filed under the key's month", got)
	}
	delete(info, hashOf("k1"))
	if err := sw.RecordKey(ctx, b); err == nil {
		t.Error("RecordKey of a key LiteLLM no longer knows = nil; keysmith must be able to report it")
	}
}

// deadlineStore records the deadline of the context its Upsert was given.
type deadlineStore struct {
	*MemStore
	has bool
	at  time.Time
}

func (s *deadlineStore) Upsert(ctx context.Context, r Record) error {
	s.at, s.has = ctx.Deadline()
	return s.MemStore.Upsert(ctx, r)
}

// Mutants killed: a RecordKey that passes its caller's context straight
// through. keysmith calls it from deleteKeys on context.Background() and on
// context.WithoutCancel(ctx) — neither carries a deadline — while holding that
// session's stripe, so an unbounded Upsert parks a goroutine per retry
// (task 07 decision (j), re-review b06 Important 3).
func TestRecordKeyBoundsItsOwnDeadline(t *testing.T) {
	now := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	store := &deadlineStore{MemStore: NewMemStore()}
	sw := &Sweeper{Keys: &fakeKeys{}, Admin: fakeInfo{hashOf("k1"): 1}, Store: store,
		Clock: func() time.Time { return now }}
	k := ref("k1", now.Add(-30*time.Minute))
	b := wire.Binding{KeyHash: k.KeyHash, KeyID: k.KeyID, SessionID: k.SessionID, Owner: k.Owner,
		TeamID: k.TeamID, ProjectID: k.ProjectID, CreatedAt: k.CreatedAt}
	if err := sw.RecordKey(context.Background(), b); err != nil { // no deadline, as deleteKeys gives it
		t.Fatal(err)
	}
	if !store.has {
		t.Fatal("the store saw a context with no deadline; keysmith holds the session's stripe across this call")
	}
	if left := time.Until(store.at); left <= 0 || left > RecordTimeout {
		t.Errorf("deadline in %v, want (0, %v]", left, RecordTimeout)
	}
}

// Mutants killed: a lookback that misses keys that have just expired; spend
// filed under the sweep's month instead of the key's (decision (a)); a NaN or
// negative figure recorded.
func TestSweepLookbackPeriodAndBadFigures(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 20, 0, 0, time.UTC)
	keys := &fakeKeys{refs: []KeyRef{ref("late-sep", time.Date(2026, 9, 30, 23, 30, 0, 0, time.UTC)), ref("nan", now), ref("neg", now)}}
	info := fakeInfo{hashOf("late-sep"): 2, hashOf("nan"): math.NaN(), hashOf("neg"): -1}
	store := NewMemStore()
	reported := 0
	sw := &Sweeper{Keys: keys, Admin: info, Store: store, Clock: func() time.Time { return now }, Report: func(error) { reported++ }}
	if n, err := sw.Once(ctx); err != nil || n != 1 || reported != 2 {
		t.Errorf("sweep: %d swept, %v, %d reported; want 1, nil, 2", n, err, reported)
	}
	if !keys.since.Equal(now.Add(-DefaultLookback)) {
		t.Errorf("asked for keys expiring after %v, want %v", keys.since, now.Add(-DefaultLookback))
	}
	if sepSpend, octSpend := sum(t, store, wire.ScopeProject, "p-1", sep), sum(t, store, wire.ScopeProject, "p-1", oct); sepSpend != 2 || octSpend != 0 {
		t.Errorf("September %v, October %v; want 2 and 0", sepSpend, octSpend)
	}
}

type fakePrices map[string]litellmadmin.GroupPrice

func (f fakePrices) ModelPrices(context.Context) (map[string]litellmadmin.GroupPrice, error) {
	return f, nil
}

// Review I-5, decision (g). Kills: no price check (an unpriced group spends
// US$0 and every budget on it is silently off), a check of input only, and a
// nil reader passing quietly while groups are named. au/unlisted pins that a
// group LiteLLM does not list is reported.
func TestSweepReportsAnUnpricedGroup(t *testing.T) {
	var got []string
	sw := &Sweeper{Keys: &fakeKeys{}, Admin: fakeInfo{}, Store: NewMemStore(),
		Prices: fakePrices{"global/m": {Input: 1e-6, Output: 5e-6}, "au/m": {Input: 1e-6}},
		Groups: []string{"global/m", "au/m", "au/unlisted"},
		Report: func(err error) {
			if errors.Is(err, ErrUnpriced) {
				got = append(got, err.Error())
			}
		}}
	if _, err := sw.Once(ctx); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, "\n")
	if len(got) != 2 || !strings.Contains(joined, "au/m ") || !strings.Contains(joined, "au/unlisted") {
		t.Errorf("unpriced reports = %q, want au/m and au/unlisted only", got)
	}
	got, sw.Prices = nil, nil
	if _, err := sw.Once(ctx); err != nil || len(got) != 1 {
		t.Errorf("a nil price reader: %d unpriced reports, %v; want 1, nil", len(got), err)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

```bash
cd control-plane && go test ./internal/usage/ -count=1 2>&1 | head -5
```

Expected: a build failure naming `undefined: Record`, `undefined: Store` or `undefined: Sweeper`.

- [ ] **Step 4: Write the migration**

Create `control-plane/internal/binding/migrations/0002_key_spend.sql`. Task 05's runner applies it once, in order, and records version 2 in `keysmith.schema_version` (`task-05.md`, decision (c)), so it needs no `if not exists`.

```sql
-- Task 09: each session key's spend, as last read from LiteLLM.
-- No foreign key to keysmith.bindings: a binding removed by DeleteExpired must
-- not take its spend with it, or the team's budget would get the money back.
create table keysmith.key_spend (
    key_hash     text             primary key check (key_hash ~ '^[0-9a-f]{64}$'),
    session_id   text             not null,
    user_id      text             not null,  -- wire.Owner.UserID(), LiteLLM's user_id
    team_id      text             not null,
    project_id   text             not null,
    period_start timestamptz      not null,  -- the UTC month the key was minted in
    spend_usd    double precision not null check (spend_usd >= 0),
    requests     bigint           not null default 0 check (requests >= 0),
    updated_at   timestamptz      not null
);

create index key_spend_person  on keysmith.key_spend (period_start, user_id);
create index key_spend_project on keysmith.key_spend (period_start, project_id);
create index key_spend_team    on keysmith.key_spend (period_start, team_id);

-- The sweep's query (usage.PGStore.RecentKeys) filters on expiry.
create index bindings_expires_at on keysmith.bindings (expires_at);
```

- [ ] **Step 5: Write the implementation**

Create `control-plane/internal/usage/usage.go`:

```go
// Package usage records each session key's spend from LiteLLM and sums it per
// person, project and team for the current UTC month. keysmith enforces the
// team dimension from it at mint (P6-D8).
package usage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// CacheTTL is how long Reader.Spent keeps a sum.
const CacheTTL = 60 * time.Second

var ErrScope = errors.New("usage: the scope is person, project or team, with an id")

// KeyRef is what the sweep needs of one binding.
type KeyRef struct {
	KeyHash, KeyID, SessionID string
	Owner                     wire.Owner
	TeamID, ProjectID         string
	CreatedAt                 time.Time
}

// Record is one key's spend, filed under the UTC month it was minted in.
type Record struct {
	KeyHash     string
	SessionID   string
	UserID      string // wire.Owner.UserID()
	TeamID      string
	ProjectID   string
	PeriodStart time.Time
	SpendUSD    float64
	Requests    int64
	UpdatedAt   time.Time
}

// Total is one scope's sum in one period.
type Total struct {
	SpendUSD float64
	Requests int64
}

// Store keeps Records. Upsert never lowers a recorded spend or request count.
type Store interface {
	Upsert(ctx context.Context, r Record) error
	Sum(ctx context.Context, scope, id string, periodStart time.Time) (Total, error)
}

// KeySource lists the keys whose expiry is after a time: live ones and ones
// that expired or were revoked recently.
type KeySource interface {
	RecentKeys(ctx context.Context, expiringAfter time.Time) ([]KeyRef, error)
}

// column is the key_spend column each scope sums, and the only text that is
// ever spliced into Sum's SQL.
var column = map[string]string{wire.ScopePerson: "user_id", wire.ScopeProject: "project_id", wire.ScopeTeam: "team_id"}

func (r Record) validate() error {
	if r.KeyHash == "" || r.UserID == "" || r.TeamID == "" || r.ProjectID == "" {
		return errors.New("usage: a record needs its key, person, team and project")
	}
	if math.IsNaN(r.SpendUSD) || math.IsInf(r.SpendUSD, 0) || r.SpendUSD < 0 || r.Requests < 0 {
		return fmt.Errorf("usage: spend %v is not a usable figure", r.SpendUSD)
	}
	return nil
}
```

```go
// MemStore is the in-memory Store, for tests.
type MemStore struct {
	mu   sync.Mutex
	rows map[string]Record
}

func NewMemStore() *MemStore { return &MemStore{rows: map[string]Record{}} }

// Upsert keeps a key's first attribution and period, and the greater spend.
func (m *MemStore) Upsert(_ context.Context, r Record) error {
	if err := r.validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.rows[r.KeyHash]; ok {
		old.SpendUSD, old.Requests, old.UpdatedAt = max(old.SpendUSD, r.SpendUSD), max(old.Requests, r.Requests), r.UpdatedAt
		r = old
	}
	m.rows[r.KeyHash] = r
	return nil
}

func (m *MemStore) Sum(_ context.Context, scope, id string, period time.Time) (Total, error) {
	if _, ok := column[scope]; !ok {
		return Total{}, ErrScope
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var t Total
	for _, r := range m.rows {
		v := map[string]string{wire.ScopePerson: r.UserID, wire.ScopeProject: r.ProjectID, wire.ScopeTeam: r.TeamID}[scope]
		if v == id && r.PeriodStart.Equal(period) {
			t.SpendUSD += r.SpendUSD
			t.Requests += r.Requests
		}
	}
	return t, nil
}

// PGStore is the Store on keysmith.key_spend, and the KeySource on
// keysmith.bindings (decision (c)).
type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

const upsertSQL = `insert into keysmith.key_spend
    (key_hash, session_id, user_id, team_id, project_id, period_start, spend_usd, requests, updated_at)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
on conflict (key_hash) do update set
    spend_usd  = greatest(keysmith.key_spend.spend_usd, excluded.spend_usd),
    requests   = greatest(keysmith.key_spend.requests, excluded.requests),
    updated_at = excluded.updated_at`

func (p *PGStore) Upsert(ctx context.Context, r Record) error {
	if err := r.validate(); err != nil {
		return err
	}
	if _, err := p.pool.Exec(ctx, upsertSQL, r.KeyHash, r.SessionID, r.UserID, r.TeamID, r.ProjectID,
		r.PeriodStart.UTC(), r.SpendUSD, r.Requests, r.UpdatedAt.UTC()); err != nil {
		return fmt.Errorf("usage: upsert: %w", err)
	}
	return nil
}

func (p *PGStore) Sum(ctx context.Context, scope, id string, period time.Time) (Total, error) {
	col, ok := column[scope]
	if !ok {
		return Total{}, ErrScope
	}
	var t Total
	q := `select coalesce(sum(spend_usd), 0), coalesce(sum(requests), 0)::bigint
from keysmith.key_spend where period_start = $1 and ` + col + ` = $2`
	if err := p.pool.QueryRow(ctx, q, period.UTC(), id).Scan(&t.SpendUSD, &t.Requests); err != nil {
		return Total{}, fmt.Errorf("usage: sum %s: %w", scope, err)
	}
	return t, nil
}

const recentSQL = `select key_hash, key_id, session_id, owner_tid, owner_oid, team_id, project_id, created_at
from keysmith.bindings where expires_at > $1`

func (p *PGStore) RecentKeys(ctx context.Context, expiringAfter time.Time) ([]KeyRef, error) {
	rows, err := p.pool.Query(ctx, recentSQL, expiringAfter.UTC())
	if err != nil {
		return nil, fmt.Errorf("usage: recent keys: %w", err)
	}
	refs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (KeyRef, error) {
		var k KeyRef
		err := row.Scan(&k.KeyHash, &k.KeyID, &k.SessionID, &k.Owner.TID, &k.Owner.OID, &k.TeamID, &k.ProjectID, &k.CreatedAt)
		return k, err
	})
	if err != nil {
		return nil, fmt.Errorf("usage: recent keys: %w", err)
	}
	return refs, nil
}
```

```go
type spentKey struct {
	owner         wire.Owner
	team, project string
	period        int64 // Unix seconds of the period start
}

type spentEntry struct {
	s     budget.Spent
	until time.Time
}

// Reader sums the current UTC month. It is task 07's SpentReader and the
// source of keysmith's GET /v1/usage.
type Reader struct {
	store  Store
	limits budget.Policy
	now    func() time.Time
	mu     sync.Mutex
	cache  map[spentKey]spentEntry
}

// NewReader: nil now is time.Now.
func NewReader(store Store, limits budget.Policy, now func() time.Time) *Reader {
	if now == nil {
		now = time.Now
	}
	return &Reader{store: store, limits: limits, now: now, cache: map[spentKey]spentEntry{}}
}

// Spent is this UTC month's spend per dimension. A sum that fails is an
// error, never zero (P6-D5); a good answer is kept for CacheTTL.
func (r *Reader) Spent(ctx context.Context, owner wire.Owner, teamID, projectID string) (budget.Spent, error) {
	now := r.now()
	period := budget.PeriodStart(now)
	k := spentKey{owner: owner, team: teamID, project: projectID, period: period.Unix()}
	r.mu.Lock()
	e, ok := r.cache[k]
	r.mu.Unlock()
	if ok && now.Before(e.until) {
		return e.s, nil
	}
	var s budget.Spent
	for _, d := range []struct {
		scope, id string
		dst       *float64
	}{
		{wire.ScopePerson, owner.UserID(), &s.PersonUSD},
		{wire.ScopeProject, projectID, &s.ProjectUSD},
		{wire.ScopeTeam, teamID, &s.TeamUSD},
	} {
		t, err := r.store.Sum(ctx, d.scope, d.id, period)
		if err != nil {
			return budget.Spent{}, fmt.Errorf("usage: %s spend: %w", d.scope, err)
		}
		*d.dst = t.SpendUSD
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.cache) >= 4096 {
		for old, v := range r.cache {
			if !now.Before(v.until) {
				delete(r.cache, old)
			}
		}
	}
	r.cache[k] = spentEntry{s: s, until: now.Add(CacheTTL)}
	return s, nil
}

// Rows is one row: this period's spend for the scope and its budget
// (decision (e)), with zero spend when nothing was spent.
func (r *Reader) Rows(ctx context.Context, scope, id string) ([]wire.UsageRow, error) {
	if _, ok := column[scope]; !ok || id == "" {
		return nil, ErrScope
	}
	period := budget.PeriodStart(r.now())
	t, err := r.store.Sum(ctx, scope, id, period)
	if err != nil {
		return nil, err
	}
	var team, project string
	switch scope {
	case wire.ScopeTeam:
		team = id
	case wire.ScopeProject:
		project = id
	}
	l, err := r.limits.Limits(ctx, team, project)
	if err != nil {
		return nil, err
	}
	b := map[string]float64{wire.ScopePerson: l.PersonUSD, wire.ScopeProject: l.ProjectUSD, wire.ScopeTeam: l.TeamUSD}[scope]
	return []wire.UsageRow{{Scope: scope, ID: id, PeriodStart: period, SpendUSD: t.SpendUSD, BudgetUSD: b, Requests: t.Requests}}, nil
}
```

Create `control-plane/internal/usage/sweep.go`:

```go
package usage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/litellmadmin"
)

// ErrUnpriced is reported for a group LiteLLM would charge nothing for, which
// silently turns off every budget on it (review I-5, decision (g)).
var ErrUnpriced = errors.New("usage: LiteLLM prices a model group at zero")

// PriceReader is task 04's (*litellmadmin.Client).ModelPrices.
type PriceReader interface {
	ModelPrices(ctx context.Context) (map[string]litellmadmin.GroupPrice, error)
}

const (
	DefaultEvery    = 60 * time.Second
	DefaultLookback = 10 * time.Minute

	// RecordTimeout bounds one RecordKey, read and write together. keysmith
	// calls it from deleteKeys on a background or WithoutCancel context, both
	// of which carry no deadline, and it calls it while holding that session's
	// stripe (task 07 decision (j)). Without this bound a stalled database
	// parks one goroutine per retry on the same stripe.
	RecordTimeout = 5 * time.Second
)

// KeyInfoer is task 04's (*litellmadmin.Client).KeyInfo.
type KeyInfoer interface {
	KeyInfo(ctx context.Context, keyID string) (litellmadmin.KeyInfo, error)
}

// Sweeper copies each recent key's spend from LiteLLM into the Store.
type Sweeper struct {
	Keys     KeySource
	Admin    KeyInfoer
	Store    Store
	Clock    func() time.Time // nil: time.Now
	Every    time.Duration    // 0: DefaultEvery
	Lookback time.Duration    // 0: DefaultLookback (decision (b))
	Report   func(error)      // nil: dropped
	Prices   PriceReader      // decision (g); nil while Groups is set is itself reported
	Groups   []string         // task 02's catalogue.Groups(): every group a key may name
}

// checkPrices reports every group LiteLLM would price at zero on input or
// output, or does not list. It never stops the sweep: spend is still copied.
func (s *Sweeper) checkPrices(ctx context.Context) {
	if len(s.Groups) == 0 {
		return
	}
	if s.Prices == nil {
		s.report(fmt.Errorf("%w: no price reader is wired, so %d groups are unchecked", ErrUnpriced, len(s.Groups)))
		return
	}
	prices, err := s.Prices.ModelPrices(ctx)
	if err != nil {
		s.report(fmt.Errorf("usage: reading LiteLLM's model prices: %w", err))
		return
	}
	for _, g := range s.Groups {
		// An unlisted group reads as the zero price, so it is reported too.
		if p := prices[g]; !(p.Input > 0) || !(p.Output > 0) {
			s.report(fmt.Errorf("%w: %s (input %v, output %v per token)", ErrUnpriced, g, p.Input, p.Output))
		}
	}
}

// Once reads every recent key's spend and records it, returning how many it
// recorded. A key that cannot be read, or reads as NaN, infinite or negative,
// is reported and skipped, and its last record stands.
func (s *Sweeper) Once(ctx context.Context) (int, error) {
	s.checkPrices(ctx) // decision (g), review I-5
	now := s.clock()
	lookback := s.Lookback
	if lookback <= 0 {
		lookback = DefaultLookback
	}
	refs, err := s.Keys.RecentKeys(ctx, now.Add(-lookback))
	if err != nil {
		return 0, fmt.Errorf("usage: listing recent keys: %w", err)
	}
	n := 0
	for _, k := range refs {
		if err := s.record(ctx, k, now); err != nil {
			s.report(err)
			continue
		}
		n++
	}
	return n, nil
}

// record reads one key's spend and files it. The sweep reports what it returns
// and goes on to the next key; RecordKey returns it to keysmith.
func (s *Sweeper) record(ctx context.Context, k KeyRef, now time.Time) error {
	info, err := s.Admin.KeyInfo(ctx, k.KeyID)
	if err != nil {
		return fmt.Errorf("usage: key info for session %s: %w", k.SessionID, err)
	}
	if math.IsNaN(info.Spend) || math.IsInf(info.Spend, 0) || info.Spend < 0 {
		return fmt.Errorf("usage: session %s: LiteLLM reported spend %v; not recorded", k.SessionID, info.Spend)
	}
	return s.Store.Upsert(ctx, Record{KeyHash: k.KeyHash, SessionID: k.SessionID, UserID: k.Owner.UserID(),
		TeamID: k.TeamID, ProjectID: k.ProjectID, PeriodStart: budget.PeriodStart(k.CreatedAt),
		SpendUSD: info.Spend, UpdatedAt: now})
}

// RecordKey files one key's spend at once, because its caller is about to
// delete the key at LiteLLM and this is the last moment /key/info can answer
// for it (task 07 decision (j), review M-3). It is keysmith's SpendRecorder.
// The figure goes through the same never-lower upsert as the sweep's, so a
// final read that LiteLLM answers late or low cannot take spend away.
//
// The caller's context carries no deadline: keysmith reaches deleteKeys on
// context.Background() and on context.WithoutCancel(ctx), which drops the
// parent's deadline, and it holds the session's stripe across the call. So
// this bounds itself (RecordTimeout), read and write together. The sweep's own
// calls go to record directly and keep the sweep's context.
func (s *Sweeper) RecordKey(ctx context.Context, b wire.Binding) error {
	ctx, cancel := context.WithTimeout(ctx, RecordTimeout)
	defer cancel()
	return s.record(ctx, KeyRef{KeyHash: b.KeyHash, KeyID: b.KeyID, SessionID: b.SessionID,
		Owner: b.Owner, TeamID: b.TeamID, ProjectID: b.ProjectID, CreatedAt: b.CreatedAt}, s.clock())
}

// Run sweeps at once, then every Every, until ctx ends.
func (s *Sweeper) Run(ctx context.Context) {
	every := s.Every
	if every <= 0 {
		every = DefaultEvery
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		if _, err := s.Once(ctx); err != nil {
			s.report(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Sweeper) clock() time.Time {
	if s.Clock == nil {
		return time.Now()
	}
	return s.Clock()
}

func (s *Sweeper) report(err error) {
	if s.Report != nil {
		s.Report(err)
	}
}
```

- [ ] **Step 6: Run the tests to verify they pass**

```bash
cd control-plane
go test ./internal/usage/ -count=1 -race -v 2>&1 | grep -c '^\s*--- PASS'   # expect 14 without a database
go test ./internal/usage/ -count=1 -v 2>&1 | grep -c '^--- SKIP'            # expect 2, each naming PANACEA_APP_GATEWAY_TEST_DATABASE_URL
```

With task 17's compose Postgres up and `PANACEA_APP_GATEWAY_TEST_DATABASE_URL` set to it, the first line prints `21` and the second `0`: the store suite's five cases run against real SQL, and `RecentKeys` against task 05's table.

- [ ] **Step 7: Prove the mutants die**

Apply each edit alone, run `go test ./internal/usage/ -count=1`, confirm the named test fails, and revert. The two SQL edits need the test database; without it, make the same edit in `MemStore` and the store suite fails there.

| Edit | Test that must fail |
|---|---|
| `MemStore.Upsert`: `max(old.SpendUSD, r.SpendUSD)` → `r.SpendUSD` (and `greatest(…)` → `excluded.spend_usd` in `upsertSQL`) | `TestMemStoreSuite/a_lower_read…`, `TestDeletedKeyKeepsItsSpend` |
| `column`: `wire.ScopeTeam: "team_id"` → `"project_id"` (and the `MemStore.Sum` map likewise) | `TestMemStoreSuite/each_scope…` |
| `Reader.Spent`: `budget.PeriodStart(now)` → `time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)` | `TestSpentIsTheCurrentUTCMonth` |
| `Reader.Spent`: `now.Add(CacheTTL)` → `now.Add(24 * time.Hour)` | `TestSpentIsCachedSixtySecondsAndFailsClosed` |
| `record`: `budget.PeriodStart(k.CreatedAt)` → `budget.PeriodStart(now)` | `TestSweepLookbackPeriodAndBadFigures` |
| `record`: on a `KeyInfo` error, return a `Record` with `SpendUSD: 0` instead of the error | `TestDeletedKeyKeepsItsSpend` (with a plain-overwrite store too) |
| `record`: delete the `Store.Upsert` call and `return nil` | `TestRecordKeyFilesTheSpendASweepWouldMiss` (team spend 0, want 7: a read that files nothing; review M-3) |
| `RecordKey`: `CreatedAt: b.CreatedAt` → `CreatedAt: s.clock()` | `TestRecordKeyFilesTheSpendASweepWouldMiss` (filed under the call's month, not the key's, so the month boundary double-counts) |
| `RecordKey`: `return s.record(…)` → `_ = s.record(…); return nil` | `TestRecordKeyFilesTheSpendASweepWouldMiss` (the second call returns nil, so keysmith would report nothing for a spend it lost) |
| `RecordKey`: delete the `context.WithTimeout(ctx, RecordTimeout)` pair | `TestRecordKeyBoundsItsOwnDeadline` (no deadline on the context the store sees; re-review b06 Important 3) |
| `Sweeper.Once`: delete the `s.checkPrices(ctx)` call (review I-5) | `TestSweepReportsAnUnpricedGroup` |
| `checkPrices`: delete `\|\| !(p.Output > 0)` | `TestSweepReportsAnUnpricedGroup` (`au/m` has no output price) |
| `checkPrices`: the `s.Prices == nil` branch `return`s without reporting | `TestSweepReportsAnUnpricedGroup` (the nil reader) |
| `recentSQL`: `expires_at > $1` → `expires_at > now()` | `TestPGRecentKeysReadsKeysExpiringAfterTheCut` |

- [ ] **Step 8: Vet and format**

```bash
cd control-plane
go vet ./internal/usage/ && gofmt -l internal/usage/   # expect no output
go test ./internal/binding/ -count=1                  # expect ok: task 05's runner accepts 0002 (its embedded-set test)
```

- [ ] **Step 9: Commit**

```bash
git add control-plane/internal/usage/usage.go control-plane/internal/usage/sweep.go \
  control-plane/internal/usage/usage_test.go control-plane/internal/binding/migrations/0002_key_spend.sql
git commit -F <message file> -- control-plane/internal/usage/ control-plane/internal/binding/migrations/0002_key_spend.sql
```

Message: `feat(usage): the per-key spend sweep and the three-dimension reader (phase 6 task 09)`, a body naming decisions (a) to (g), and the attribution line.

**Acceptance:** step 6 prints `14` and `2` without a database, and `21` and `0` with one; step 7's fourteen edits each fail the named test; step 8 prints nothing and then `ok`.

**What this task does not claim:**

- **A reset date.** This reader reports spend and keysmith's UTC month; LiteLLM's own `budget_reset_at` is not read here. Seam **S6.12**'s fallback, if task 18's P16 fails, has this reader fetch it per person and per project (one `/user/info`, one `/team/info`) and fill the `wire.UsageRow` field task 03 adds. This part merges in wave 4 and task 18 runs in wave 8, so that arrives as an amendment (re-review b06, Minor 2).
- **Accuracy finer than the sweep.** keysmith's view of spend is up to `Every` (60 s) plus `CacheTTL` (60 s) old, plus however long LiteLLM takes to write a request's spend onto its key (LiteLLM batches spend writes; the delay is **UNVERIFIED** until task 18's P12). Between sweeps a team can overshoot its budget by what its live keys spend in that window, on top of task 06's bound of K × `SessionCapUSD`. The person and project dimensions also have LiteLLM's own real-time budgets (task 18, P03 and P04); the team has only this.
- **Every cent of a revoked key.** A key LiteLLM deletes between two sweeps loses up to one interval of spend from this rollup (its last recorded figure stands). LiteLLM's user and team spend still count it, so the person and project dimensions are unaffected; the team dimension under-counts by at most that interval per revoked key.
- **That `GET /key/info` reports spend by the hashed id** task 07 stores (P12), or that its `spend` is cumulative over the key's life (training knowledge, **UNVERIFIED**).
- **Request counts.** `Requests` is 0 (decision (f)). So the sweep cannot flag one key with requests and no spend (review I-5's suggestion); decision (g) checks the price of every group instead.
- **That `GET /model/info` reports the price LiteLLM charges.** Its shape and its merging of `model_info` with LiteLLM's own table are training knowledge, **UNVERIFIED** until task 18.
- **Per-team budgets from the Hub.** `Rows`' `BudgetUSD` is exact only while the policy answers every team alike (decision (e)).

**Gaps against the skeleton, for the orchestrator:**

1. **The sweep needs a list of recent keys,** which task 05's `Store` does not offer (`BySession` needs a session id). This part reads `keysmith.bindings` directly through `PGStore.RecentKeys` (decision (c)), so task 05 is unchanged; task 08's wiring passes this `PGStore`, not the binding store, as `Sweeper.Keys`.
2. **Migration `0002` also indexes `keysmith.bindings(expires_at)`,** task 05's table, because the sweep's query is here.
3. **`GET /v1/usage?scope=&id=` carries no team for a project,** so a Hub policy that varies by team (S6.2) cannot give a project row its exact budget. When H-2 lands, the route needs a `teamId` parameter or the Hub's project record.
4. **No request count source.** If `Requests` matters to the cockpit, task 18 should find whether the pinned LiteLLM exposes one per key without an enterprise feature.
5. **`Rows` returns the current period only, one row per call** (task 21's gap 6 asked for the zero-spend row; this delivers it).
6. **The price check needs two hand-offs** (decision (g), review I-5). **Task 04** adds `(*Client).ModelPrices(ctx) (map[string]GroupPrice, error)` over `GET /model/info` (done in its part, fix batch b02). **Task 08** passes `Prices: <the task 04 client>` and `Groups: catalogue.Groups()` into the `Sweeper` it runs; without `Groups` the check is off, so task 08's wiring names both (done in its `run`, fix batch b02; the line has no test of its own).
7. **`RecordKey` is this part's half of task 07 decision (j)** (review M-3). keysmith declares the interface (`keysmith.SpendRecorder`), this part implements it on `*Sweeper`, and **task 08** passes that same sweeper as `Deps.Recorder` where it already passes the reader as `Deps.Spent`. `Recorder` is required, so a wiring that forgets it fails at `keysmith.New` rather than losing spend quietly. Nothing else moves: this part still depends on 04 and 06, and task 07 on 04, 05 and 06.
