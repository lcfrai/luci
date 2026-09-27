### Task 07: keysmith core: mint, refresh, revoke

A sandbox's only credential is a one-hour LiteLLM virtual key that keysmith mints for one session generation (P6-D6), and whose **binding** (hash, pod address, project, route at mint, expiry) keysmith records first (P6-D7). This task is the logic of that, with no transport: the order of the checks, the idempotency of a retried generation, the refresh that does not cut a stream in flight, and revocation. Task 08 puts HTTP and caller identity in front of it.

**The one thing this task must not get wrong is a key nobody holds.** Panacea's lesson is that a mint can time out *after* LiteLLM has made the key (`~/Projects/Panacea/services/panacea-api/handlers/llm_env_keys.go:120`, the `/key/generate` call, verified 2026-09-25). The caller then retries. If the retry simply mints again, the first key is live, budgeted and held by no one. So a retried generation **revokes whatever that generation already has live** before it mints, and a mint that fails half-way revokes its own binding before it returns. After any sequence of retries, a generation has at most one live binding, and it is the one whose token the caller last received.

**Files:**
- `control-plane/internal/keysmith/keysmith.go`: `AdminAPI`, `SpentReader`, `SpendRecorder`, `Deps`, `Service`, `New`, `Mint`, `Refresh`, `RevokeSession`, `ByKeyHash`, the error sentinels, `BudgetDuration`, `EnsureMemo` (new)
- `control-plane/internal/keysmith/keysmith_test.go`: nineteen tests, at least one mutant each (new)

**Interfaces:**
- Produces:
  - `keysmith.New(Deps) (*Service, error)`; `(*Service).Mint(ctx, wire.MintRequest) (wire.MintResponse, error)`; `(*Service).Refresh(ctx, wire.RefreshRequest) (wire.MintResponse, error)`; `(*Service).RevokeSession(ctx, wire.RevokeRequest) (int, error)`
  - `(*Service).ByKeyHash(ctx, keyHash string) (wire.Binding, error)`: `*Service` satisfies `wire.BindingReader` (a compile-time assertion pins it), and retires a raised Global-minted key as it reads it (decision (h)); task 08's `Deps.Bindings` is this, not the store
  - `keysmith.AdminAPI` (the four `*litellmadmin.Client` methods keysmith calls) and `keysmith.SpentReader` (task 09's `*usage.Reader` satisfies it)
  - Errors task 08 maps to problem codes: `ErrInvalid` (400 `bad_request`; `ErrTeamMismatch` wraps it, decision (i)), `wire.ErrRouteUnknown` wrapped (503 `route_unknown`), `*BudgetExhaustedError{Dimension}` (402 `budget_exhausted`, the status is task 03's), `ErrBudgetUnknown` (503 `budget_unknown`, see gap 2), `ErrNoSession` (404 `session_not_found`), `ErrBusy` and `ErrSuperseded` (409 `conflict`), `ErrAdmin` and `ErrStore` (503 `backend_unavailable`)
- Consumes:
  - task 03: `wire.MintRequest` (with `Validate`), `wire.MintResponse`, `wire.RefreshRequest`, `wire.RevokeRequest`, `wire.Binding`, `wire.BindingReader`, `wire.Owner`, `wire.ProjectInference`, `wire.RouteSource`, `wire.ErrRouteUnknown`, `wire.HashToken`
  - task 04: `litellmadmin.GenerateKeyRequest`, `litellmadmin.GenerateKeyResponse{Key string}`, and the method signatures in `AdminAPI` below (the identifiers are this part's assumption; gap 4)
  - task 05: `binding.Store`, `binding.ErrExists`, `binding.NewMemStore()` for the tests (gap 1 is a change to its uniqueness rule)
  - task 06: `budget.Policy`, `budget.Spent`, `budget.Slice`, `budget.Dimension`, `budget.None`, `budget.NewStatic`, `budget.Proposed` (read in `task-06.md`, committed)
  - task 01: `dataclass.Decide`, `dataclass.Tighter`, `dataclass.Route`, `RouteGlobal`, `RouteAU`, `Class`, `LevelInternal`, `ResidencyAny`, `ResidencyAUOnly` (read in `task-01.md`, committed)
- Required by: task 08 (the HTTP API wraps `Service`, and its binding route reads through `ByKeyHash`); task 15 (the retry contract: the same generation, twice, is safe; the refresh on a raise, gap 7)

**Decisions where the skeleton is silent:**

**(a) A retry is detected by generation, and it kills the earlier key first.** `Mint(g)` and `Refresh(g)` both begin by revoking every **live** binding of generation `g` and deleting its LiteLLM key. The caller is asking for generation `g` again, which it only does when it never received the first reply. This runs before the route and budget checks, so a retry that is then refused leaves nothing live that the caller does not hold. On a first mint there is nothing to revoke and no LiteLLM call is made.

**(b) A per-session lock inside one process, and a re-check across processes.** Mints of one session are serialised by one of 64 striped mutexes (bounded memory, no map that grows). Two keysmith replicas (task 19 runs two) are covered by the store: `Put` refuses a second **live** binding of a `(session, generation)` (gap 1), which this task reports as `ErrBusy`, and after `GenerateKey` returns, the mint re-reads its own binding; if a retry on the other replica revoked it meanwhile, the mint deletes its own key and returns `ErrSuperseded`. Either way the caller retries and converges on one live key.

**(c) Refresh schedules revocation as data, not as a timer.** `Refresh(g)` mints `g` from the live binding of `g-1`, then calls `Store.Revoke(g-1, now+Grace)`: a revocation dated in the future. The gatekeeper (task 10) treats a binding as revoked from `RevokedAt` onward, so the old key keeps working for `Grace` and stops at `Grace`, across a keysmith restart, with no timer to lose. The LiteLLM-side delete of the old key is a best-effort `After(Grace, …)`; if it is lost, the key is still refused by the gatekeeper and expires at its `TTL`. Only the call that dates the revocation schedules that delete, so a retried refresh, or a refresh after decision (h) has retired the key, schedules no second `/key/delete` of a key already gone (re-review R-3; what LiteLLM answers to that is **UNVERIFIED**, and nothing now asks). `RefreshRequest.Generation` is the **new** generation, so a retried refresh is exactly a retried mint (decision (a)).

**(d) A key minted on AU can name only `au/*` groups.** The skeleton gives every key both groups. A key minted on Global must keep both, because a raise to AU takes effect mid-key (P6-D4). A key minted on AU never needs a `global/*` group, since the gatekeeper keeps it on AU until the next mint (`Tighter(live, RouteAtMint)`). Denying it at LiteLLM too means a gatekeeper bug cannot send an AU project's request to Global. Defence in depth for OD-24's "never loosen", at no cost.

**(e) An admin error's text is scrubbed of the token before it is wrapped.** Task 04 promises its errors carry no response body. keysmith does not rely on that: every wrapped error has the session's token replaced by `[redacted]`. Panacea's own mint puts LiteLLM's response body in its error (`llm_env_keys.go:133`, verified 2026-09-25), which is the failure this closes twice.

**(f) `KeyID` is `wire.HashToken(token)`.** It is what keysmith passes to `DeleteKeys` and what task 09 passes to `KeyInfo`, so keysmith never stores a raw token. That LiteLLM names a key by the hex SHA-256 of its value, and accepts that name on `/key/delete` and `/key/info`, is training knowledge, **UNVERIFIED** until task 18's P11 and P12.

**(g) LiteLLM user and team budgets use `budget_duration: "30d"`**, the value Panacea runs in its pinned LiteLLM (`services/panacea-api/handlers/llm_env_keys.go:270`, verified 2026-09-25). Task 06 already records that LiteLLM's window is independent of our UTC month. **That is two clocks on one sum, and this constant is provisional** (review M-7, seam S6.12). Task 18's **P16** asks whether the pinned image accepts a calendar-month duration; a PASS makes that the value here and there is one clock. Task 18's **P15** asks what `ensure` does to the window: this service re-sends the same figure every `EnsureMemo` (10 minutes), and if `/user/update` moves `budget_reset_at`, LiteLLM's window never elapses, its `spend` never returns to zero, and a person refused once is refused for good while the cockpit reads US$0.00. If P15 fails, `memoKey` gains the period (`budget.PeriodStart(now)`) so the ensure happens once per person per month, not once per ten minutes — a one-field change kept here rather than in a gap, because the failure it prevents is silent.

**(h) A raise retires every Global-minted key within one refresh's grace** (review C-1). Decision (d) narrows an AU-minted key to `au/*`, but a key minted while its project was Global names every `global/*` group. Until now it kept them until its scheduled refresh, up to about 50 minutes after a raise. A body fallback that got past task 11, or a gatekeeper bug, could then take an AU project's request out of Australia on that key.

So `*Service` is itself a `wire.BindingReader`. Task 08 serves `GET /v1/bindings/{keyHash}` from it instead of from the store (task 08's gap 8), and every gatekeeper lookup reaches it through the gatekeeper's binding cache (≤ 10 s, P6-D7). `ByKeyHash` acts only on a binding that is live and was minted on Global, and whose project's route (`s.route`, the same check `mint` makes, which is task 03's `ProjectInference.LiveRoute`) is now AU. Task 15's refresher reads a raise by the same function, so a raise keysmith retires on is a raise the refresher re-mints on (re-review R-6). It dates that binding's revocation `Grace` from now and deletes its LiteLLM key after `Grace`: exactly what `Refresh` does to the generation it replaces, so a stream in flight is not cut. It never moves an earlier date later.

The bound, after the first request following a raise: at most the route cache (≤ 30 s) plus the binding cache (≤ 10 s) plus `Grace` (5 min). A Global-minted key that is not used after the raise carries nothing, because only the gatekeeper and keysmith can reach LiteLLM (P6-D1). If the route cannot be read, `ByKeyHash` changes nothing and reports it. The gatekeeper reads the route itself and fails closed (task 10), so a Hub blip must not turn into a refusal of every binding lookup.

**(i) The team is the project's, not the caller's** (review I-7). `MintRequest.TeamID` is what the caller says, and the team budget (`Budgets.Limits`, `Spent`), the binding's `TeamID` and the key's `team` metadata, which task 09's ledger attributes by, all key on it. `mint` reads the project once, through `s.project`, which returns the live route and `ProjectInference.TeamID` (task 03 decision (h)). A non-empty source team that differs from the caller's refuses the mint with `ErrTeamMismatch`, an `ErrInvalid` (400 `bad_request`), before the budget read and before any LiteLLM call. A source with no teams gives `""`, and then the caller's team stands: that is task 13's `File` source only, local mode's `projects.json`. The Hub source refuses a record without a valid `teamId` (task 13 decision (f)), so in the cloud `""` never arrives. `Refresh` re-mints through `mint` with the binding's team, so a project whose team no longer matches stops refreshing: fail closed. Whether the owner is a member of that team is the caller's check (ask P8-1), because keysmith holds no user token.

**(j) Every delete of a key records its spend first** (review M-3). Task 09's sweep reads each key's spend every 60 seconds and keeps the last figure a key ever showed, so a key deleted between two sweeps loses up to one sweep of spend — on **every** refresh and every revoke, which is every session, several times an hour. The team dimension is keysmith's own (P6-D8), so that loss is a budget that undercounts for the rest of the month. The fix is one read: `deleteKeys` takes bindings, calls `Deps.Recorder.RecordKey` for each, and only then calls `Admin.DeleteKeys`. One rule, no exceptions, because an exception is what an implementer gets wrong: the mint-rollback path (`generateThenBind`) records a key whose token was never returned, whose spend is therefore zero, and task 09's upsert never lowers a recorded figure, so a zero row changes no sum. A `RecordKey` that fails is reported and the delete proceeds — a key that cannot be read is still a key that must not stay usable. The read costs one `/key/info` call on task 04's own deadline, on a background context, never on the caller's.

**`RecordKey` is bounded by task 09, not by this part** (re-review b06 round 1, Important 3). All six `deleteKeys` call sites reach it on `context.Background()` (`:994`, `:1058`) or `context.WithoutCancel(ctx)` (`:1023`, `:1073`, `:1212`, `:1229`), and `WithoutCancel` drops the parent's deadline too, so the context that arrives here has none. The `/key/info` read is bounded anyway — task 04 gives every admin call its own deadline — but `RecordKey`'s `Store.Upsert` is a database write, and a deadline-free write is held inside `s.lock(session)` (`:1291`, and the bound this costs other sessions is stated at `:1416`). A stalled Postgres would then park one goroutine per retry on the same stripe while keysmith answers `504 mint_timeout` and the caller retries. So `RecordKey` puts its own `context.WithTimeout(ctx, usage.RecordTimeout)` (5 s) around the whole read-and-file, in task 09 where the write lives: one place, and the sweep's own calls keep the sweep's context. Nothing in this part passes a deadline of its own.

**Why keysmith retires rather than re-mints**, as the review words it: keysmith never holds a token after the mint reply (decision (f)) and cannot hand one to a sandbox. A key it minted on its own would be a key nobody holds, the one thing this task must not get wrong. **The re-mint belongs to the session's refresher.** Task 15's `Launcher` already holds `Routes`, and the cloud broker is ask P8-1. The refresher refreshes as soon as its project's route rises, and `Refresh` then mints on AU with `au/*` only (decision (d)). A refresher that does not do this leaves its session refused (`401 token_revoked`) from the end of `Grace` until its next refresh. That fails closed, never open. Gap 7 hands this to task 15 and to P8-1.

- [ ] **Step 1: Preflight**

```bash
cd control-plane
test -d internal/keysmith && echo "STOP: internal/keysmith exists" || echo "ok: no keysmith package yet"   # expect ok
for p in inference/wire litellmadmin binding budget dataclass; do test -d internal/$p && echo "ok $p" || echo "MISSING $p"; done
grep -n 'func NewMemStore\|ErrExists' internal/binding/*.go | head -3                               # expect both names
grep -n 'type GenerateKeyResponse\|func (c \*Client) GenerateKey' internal/litellmadmin/client.go  # expect both
grep -n 'func (pi ProjectInference) LiveRoute' internal/inference/wire/wire.go                     # expect one line (task 03 decision (g))
```

Every `MISSING` line is a dependency not merged yet (tasks 01, 03, 04, 05, 06); stop and report. If the last two greps show different identifiers from the ones this part assumes (gap 4), rename in this part's code to match the merged ones; do not change theirs.

- [ ] **Step 2: Write the failing tests**

Nineteen tests (seventeen, plus the two decision (j) adds). The store is task 05's real memory store, not a fake, so the idempotency tests exercise the uniqueness rule the Postgres store also passes (`bindingtest`). LiteLLM, the route source, the spend reader, the spend recorder and the clock are fakes. `After` is captured, so the grace-period delete runs when the test says, not on a timer.

Create `control-plane/internal/keysmith/keysmith_test.go`:

```go
package keysmith

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/litellmadmin"
)

type fakeAdmin struct {
	mu        sync.Mutex
	generated []litellmadmin.GenerateKeyRequest
	deleted   [][]string
	log       []string // "record:<keyID>" then "delete:<keyID>", in order (decision (j))
	users     int
	teams     int
	genErr    error                                   // GenerateKey fails with this
	echo      bool                                    // GenerateKey fails with an error that quotes the key
	replyKey  string                                  // the key LiteLLM "chose" when the request has none
	during    func(r litellmadmin.GenerateKeyRequest) // runs inside GenerateKey
}

func (f *fakeAdmin) GenerateKey(_ context.Context, r litellmadmin.GenerateKeyRequest) (litellmadmin.GenerateKeyResponse, error) {
	f.mu.Lock()
	f.generated = append(f.generated, r)
	during, err, echo, reply := f.during, f.genErr, f.echo, f.replyKey
	f.mu.Unlock()
	if during != nil {
		during(r)
	}
	if echo {
		return litellmadmin.GenerateKeyResponse{}, fmt.Errorf("litellm 500: duplicate key %s", r.Key)
	}
	if err != nil {
		return litellmadmin.GenerateKeyResponse{}, err
	}
	if r.Key != "" {
		reply = r.Key
	}
	return litellmadmin.GenerateKeyResponse{Key: reply}, nil
}

func (f *fakeAdmin) DeleteKeys(_ context.Context, keys []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, append([]string(nil), keys...))
	for _, k := range keys {
		f.log = append(f.log, "delete:"+k)
	}
	return nil
}

func (f *fakeAdmin) EnsureUser(context.Context, string, float64, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users++
	return nil
}

func (f *fakeAdmin) EnsureTeam(context.Context, string, float64, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.teams++
	return nil
}
```

```go
type routes map[string]wire.ProjectInference

func (r routes) Project(_ context.Context, id string) (wire.ProjectInference, error) {
	if p, ok := r[id]; ok {
		return p, nil
	}
	return wire.ProjectInference{}, fmt.Errorf("%w: %s", wire.ErrRouteUnknown, id)
}

type spentFake struct {
	s   budget.Spent
	err error
}

func (f spentFake) Spent(context.Context, wire.Owner, string, string) (budget.Spent, error) {
	return f.s, f.err
}

var (
	t0     = time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	anyRes = dataclass.Class{Level: dataclass.LevelInternal, Residency: dataclass.ResidencyAny}
	auOnly = dataclass.Class{Level: dataclass.LevelInternal, Residency: dataclass.ResidencyAUOnly}
	groups = []string{"global/claude-opus-4-8", "au/claude-opus-4-8"}
)

type rig struct {
	svc   *Service
	admin *fakeAdmin
	rec   *fakeRecorder
	store binding.Store
	now   time.Time
	later []scheduled // After calls, run by the test
}

// fakeRecorder is task 09's *usage.Sweeper in the tests. It appends to the
// fakeAdmin's own call log, so a test can assert that the record of a key's
// final spend happened BEFORE its delete (decision (j)), not merely that both
// happened. err makes a failed record a reported failure, never a lost delete.
type fakeRecorder struct {
	admin *fakeAdmin
	err   error
	keys  []string // key hashes recorded, in order
}

func (f *fakeRecorder) RecordKey(_ context.Context, b wire.Binding) error {
	f.admin.mu.Lock()
	f.keys = append(f.keys, b.KeyHash)
	f.admin.log = append(f.admin.log, "record:"+b.KeyID)
	f.admin.mu.Unlock()
	return f.err
}

// scheduled is one After call. The delay is kept so a test can prove a delete
// waits out Grace (re-review R-4): a rig that drops it lets After(0, …) pass.
type scheduled struct {
	d time.Duration
	f func()
}

func newRig(t *testing.T, mut ...func(*Deps)) *rig {
	t.Helper()
	pol, err := budget.NewStatic(budget.Proposed)
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{admin: &fakeAdmin{}, store: binding.NewMemStore(), now: t0}
	r.rec = &fakeRecorder{admin: r.admin} // shares the fake's log, so order is provable
	d := Deps{
		Admin: r.admin, Store: r.store, Budgets: pol, Spent: spentFake{}, Recorder: r.rec,
		Routes: routes{
			"p-global": {ProjectID: "p-global", TeamID: "team-1", DataClass: anyRes, Route: dataclass.RouteGlobal, Effective: dataclass.RouteGlobal},
			// The record says global; the class forces AU. keysmith must not trust the record.
			"p-au":     {ProjectID: "p-au", TeamID: "team-1", DataClass: auOnly, Route: dataclass.RouteGlobal, Effective: dataclass.RouteAU},
			"p-noclass": {ProjectID: "p-noclass", Route: dataclass.RouteGlobal, Effective: dataclass.RouteGlobal},
			// Decision (i): another team's project, and a source with no teams (local mode).
			"p-team2":  {ProjectID: "p-team2", TeamID: "team-2", DataClass: anyRes, Route: dataclass.RouteGlobal, Effective: dataclass.RouteGlobal},
			"p-noteam": {ProjectID: "p-noteam", DataClass: anyRes, Route: dataclass.RouteGlobal, Effective: dataclass.RouteGlobal},
		},
		Groups:  groups,
		Clock:   func() time.Time { return r.now },
		After:   func(d time.Duration, f func()) { r.later = append(r.later, scheduled{d, f}) },
		BaseURL: "http://gatekeeper:4000",
	}
	for _, m := range mut {
		m(&d)
	}
	if r.svc, err = New(d); err != nil {
		t.Fatal(err)
	}
	return r
}

func mintReq(project string, gen int) wire.MintRequest {
	return wire.MintRequest{SessionID: "s-1", Generation: gen, Owner: wire.Owner{TID: "t-1", OID: "o-1"},
		TeamID: "team-1", ProjectID: project, AgentID: "a-1", SessionClass: "solo", PodAddr: "10.244.1.7"}
}

// liveBindings is every binding of s-1 that liveAt accepts at the given time.
func liveBindings(t *testing.T, s binding.Store, at time.Time) []wire.Binding {
	t.Helper()
	all, err := s.BySession(context.Background(), "s-1")
	if err != nil {
		t.Fatal(err)
	}
	var out []wire.Binding
	for _, b := range all {
		if liveAt(b, at) {
			out = append(out, b)
		}
	}
	return out
}
```

```go
// Mutants killed: EnsureUser, EnsureTeam or GenerateKey called before the route
// is known (check-after-call); a zero data class treated as Global.
func TestMintChecksTheRouteBeforeAnyLiteLLMCall(t *testing.T) {
	for _, project := range []string{"p-missing", "p-noclass"} {
		r := newRig(t)
		_, err := r.svc.Mint(context.Background(), mintReq(project, 1))
		if !errors.Is(err, wire.ErrRouteUnknown) {
			t.Fatalf("%s: err = %v, want wire.ErrRouteUnknown", project, err)
		}
		if n := len(r.admin.generated) + len(r.admin.deleted) + r.admin.users + r.admin.teams; n != 0 {
			t.Errorf("%s: %d LiteLLM calls, want 0", project, n)
		}
		if b := liveBindings(t, r.store, t0); len(b) != 0 {
			t.Errorf("%s: %d live bindings, want 0", project, len(b))
		}
	}
}

// Review I-7, decision (i). Mutants killed: no team check (the caller's claimed
// team is budgeted and attributed); the check moved below the spend read or the
// LiteLLM calls; a source team of "" refused (local mode has no teams).
func TestMintRefusesATeamThatIsNotTheProjects(t *testing.T) {
	// The spend read fails, so a check placed after it reports budget_unknown.
	r := newRig(t, func(d *Deps) { d.Spent = spentFake{err: errors.New("ledger down")} })
	_, err := r.svc.Mint(context.Background(), mintReq("p-team2", 1))
	if !errors.Is(err, ErrTeamMismatch) || !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrTeamMismatch, an ErrInvalid", err)
	}
	if n := len(r.admin.generated) + len(r.admin.deleted) + r.admin.users + r.admin.teams; n != 0 {
		t.Errorf("%d LiteLLM calls, want 0", n)
	}
	if b := liveBindings(t, r.store, t0); len(b) != 0 {
		t.Errorf("%d live bindings, want 0", len(b))
	}
	// A source with no teams (task 13's File) leaves the caller's team standing,
	// and a matching team mints.
	for _, project := range []string{"p-noteam", "p-global"} {
		ok := newRig(t)
		if _, err := ok.svc.Mint(context.Background(), mintReq(project, 1)); err != nil {
			t.Errorf("%s: %v, want a mint", project, err)
		}
	}
}

// Mutants killed: minting without consulting Slice; naming a fixed dimension.
func TestMintNamesTheExhaustedDimensionAndMintsNothing(t *testing.T) {
	p := budget.Proposed
	cases := map[budget.Dimension]budget.Spent{
		budget.Person:  {PersonUSD: p.PersonUSD},
		budget.Project: {ProjectUSD: p.ProjectUSD + 1},
		budget.Team:    {TeamUSD: p.TeamUSD},
	}
	for want, s := range cases {
		r := newRig(t, func(d *Deps) { d.Spent = spentFake{s: s} })
		_, err := r.svc.Mint(context.Background(), mintReq("p-global", 1))
		var be *BudgetExhaustedError
		if !errors.As(err, &be) || be.Dimension != want {
			t.Fatalf("%s: err = %v, want *BudgetExhaustedError{%s}", want, err, want)
		}
		if len(r.admin.generated) != 0 || len(liveBindings(t, r.store, t0)) != 0 {
			t.Errorf("%s: a key or a binding was made", want)
		}
	}
}

// Mutant killed: an unreadable spend treated as nothing spent, which mints a
// full slice against a dimension that may already be exhausted (P6-D5).
func TestMintFailsClosedWhenSpendCannotBeRead(t *testing.T) {
	r := newRig(t, func(d *Deps) { d.Spent = spentFake{err: errors.New("rollup down")} })
	_, err := r.svc.Mint(context.Background(), mintReq("p-global", 1))
	if !errors.Is(err, ErrBudgetUnknown) {
		t.Fatalf("err = %v, want ErrBudgetUnknown", err)
	}
	if len(r.admin.generated) != 0 {
		t.Errorf("GenerateKey called %d times, want 0", len(r.admin.generated))
	}
}

// Mutants killed: GenerateKey before Store.Put (a LiteLLM key no binding can
// find); the raw token stored in the binding; a wrong alias, duration, owner
// id, team id, model list, metadata or slice.
func TestMintBindsBeforeItGenerates(t *testing.T) {
	r := newRig(t)
	r.admin.during = func(g litellmadmin.GenerateKeyRequest) {
		b, err := r.store.ByKeyHash(context.Background(), wire.HashToken(g.Key))
		if err != nil || !liveAt(b, t0) {
			t.Errorf("no live binding while GenerateKey ran: %v", err)
		}
	}
	resp, err := r.svc.Mint(context.Background(), mintReq("p-global", 1))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp.Token, "sk-pa-") || len(resp.Token) != len("sk-pa-")+43 {
		t.Errorf("token has length %d, want sk-pa- plus 43", len(resp.Token))
	}
	want := litellmadmin.GenerateKeyRequest{Key: resp.Token, KeyAlias: "sess/s-1/1", Duration: "1h",
		UserID: "t-1:o-1", TeamID: "p-global", Models: groups, MaxBudget: budget.Proposed.SessionCapUSD,
		Metadata: map[string]string{"tid": "t-1", "oid": "o-1", "team": "team-1", "project": "p-global",
			"agent": "a-1", "session": "s-1", "route": "global"}}
	if got := r.admin.generated[0]; !reflect.DeepEqual(got, want) {
		t.Errorf("GenerateKey got\n%+v\nwant\n%+v", got, want)
	}
	bs := liveBindings(t, r.store, t0)
	if len(bs) != 1 {
		t.Fatalf("%d live bindings, want 1", len(bs))
	}
	b := bs[0]
	if b.KeyHash != wire.HashToken(resp.Token) || b.KeyID != b.KeyHash || b.Generation != 1 ||
		b.PodAddr != netip.MustParseAddr("10.244.1.7") || b.RouteAtMint != dataclass.RouteGlobal ||
		!b.ExpiresAt.Equal(t0.Add(time.Hour)) || b.RevokedAt != nil {
		t.Errorf("binding = %+v", b)
	}
	if strings.Contains(fmt.Sprintf("%+v", b), resp.Token) {
		t.Error("the binding holds the raw token")
	}
	if resp.KeyID != b.KeyID || resp.Route != dataclass.RouteGlobal || resp.BaseURL != "http://gatekeeper:4000" ||
		!resp.ExpiresAt.Equal(b.ExpiresAt) || resp.MaxBudgetUSD != budget.Proposed.SessionCapUSD {
		t.Errorf("response = %+v", resp)
	}
}
```

```go
// Mutants killed: every key given both groups (decision (d)); the stored route
// ("global") trusted over the class's floor.
func TestAUKeyNamesOnlyAUGroups(t *testing.T) {
	r := newRig(t)
	resp, err := r.svc.Mint(context.Background(), mintReq("p-au", 1))
	if err != nil {
		t.Fatal(err)
	}
	g := r.admin.generated[0]
	if resp.Route != dataclass.RouteAU || g.Metadata["route"] != "au" ||
		!slices.Equal(g.Models, []string{"au/claude-opus-4-8"}) {
		t.Errorf("route %q, metadata route %q, models %v: want au and au/* only", resp.Route, g.Metadata["route"], g.Models)
	}
	if b := liveBindings(t, r.store, t0); len(b) != 1 || b[0].RouteAtMint != dataclass.RouteAU {
		t.Errorf("binding route = %+v, want au", b)
	}
}

// Mutant killed: a retried generation minted again without revoking the first
// key, which leaves a live, budgeted key that nobody holds (the Panacea chain).
func TestRetriedGenerationLeavesOneLiveKey(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	first, err := r.svc.Mint(ctx, mintReq("p-global", 1))
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.svc.Mint(ctx, mintReq("p-global", 1))
	if err != nil {
		t.Fatal(err)
	}
	if bs := liveBindings(t, r.store, t0); len(bs) != 1 || bs[0].KeyHash != wire.HashToken(second.Token) {
		t.Fatalf("live = %+v, want only the second key", bs)
	}
	if want := [][]string{{wire.HashToken(first.Token)}}; !reflect.DeepEqual(r.admin.deleted, want) {
		t.Errorf("deleted = %v, want exactly the first key", r.admin.deleted)
	}
}

// Mutant killed: returning the error with the binding still live. The delete
// matters too: a GenerateKey that timed out may have made the key anyway.
func TestFailedGenerateKeyLeavesNoLiveBinding(t *testing.T) {
	r := newRig(t)
	r.admin.genErr = errors.New("litellm: 503")
	_, err := r.svc.Mint(context.Background(), mintReq("p-global", 1))
	if !errors.Is(err, ErrAdmin) {
		t.Fatalf("err = %v, want ErrAdmin", err)
	}
	if b := liveBindings(t, r.store, t0); len(b) != 0 {
		t.Errorf("%d live bindings after a failed GenerateKey, want 0", len(b))
	}
	if want := [][]string{{wire.HashToken(r.admin.generated[0].Key)}}; !reflect.DeepEqual(r.admin.deleted, want) {
		t.Errorf("deleted = %v, want the attempted key", r.admin.deleted)
	}
}

// Mutant killed: LiteLLM's error text wrapped verbatim (decision (e)).
func TestErrorsNeverCarryTheToken(t *testing.T) {
	r := newRig(t)
	r.admin.echo = true
	_, err := r.svc.Mint(context.Background(), mintReq("p-global", 1))
	key := r.admin.generated[0].Key
	if err == nil || strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("err = %v: want an error, scrubbed of the key and marked [redacted]", err)
	}
}
```

```go
// Mutants killed: revoking the old generation at once (cuts a stream in
// flight), never revoking it (two live keys for one session for an hour), and
// deleting its LiteLLM key before Grace (cuts the stream at LiteLLM instead).
func TestRefreshKeepsTheOldKeyForGraceOnly(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	old, err := r.svc.Mint(ctx, mintReq("p-global", 1))
	if err != nil {
		t.Fatal(err)
	}
	r.now = t0.Add(45 * time.Minute)
	at := r.now
	fresh, err := r.svc.Refresh(ctx, wire.RefreshRequest{SessionID: "s-1", Generation: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !fresh.ExpiresAt.Equal(at.Add(time.Hour)) {
		t.Errorf("fresh key expires %v, want %v", fresh.ExpiresAt, at.Add(time.Hour))
	}
	if n := len(liveBindings(t, r.store, at.Add(5*time.Minute-time.Nanosecond))); n != 2 {
		t.Errorf("%d live just inside the grace, want 2", n)
	}
	live := liveBindings(t, r.store, at.Add(5*time.Minute))
	if len(live) != 1 || live[0].KeyHash != wire.HashToken(fresh.Token) || live[0].Generation != 2 {
		t.Errorf("live at the end of the grace = %+v, want only generation 2", live)
	}
	if len(r.admin.deleted) != 0 || len(r.later) != 1 {
		t.Fatalf("deleted %v and %d scheduled, want nothing deleted yet and one scheduled", r.admin.deleted, len(r.later))
	}
	if r.later[0].d != 5*time.Minute {
		t.Errorf("the old key's delete waits %v, want Grace (5m)", r.later[0].d)
	}
	r.later[0].f()
	if want := [][]string{{wire.HashToken(old.Token)}}; !reflect.DeepEqual(r.admin.deleted, want) {
		t.Errorf("after the grace, deleted = %v, want the old key", r.admin.deleted)
	}
}

// Mutant killed: a refresh minted without a live previous generation, which
// lets a caller jump generations or revive a revoked session.
func TestRefreshNeedsTheLiveGenerationBefore(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	refresh := func(gen int) error {
		_, err := r.svc.Refresh(ctx, wire.RefreshRequest{SessionID: "s-1", Generation: gen})
		return err
	}
	if err := refresh(2); !errors.Is(err, ErrNoSession) {
		t.Errorf("refresh with no session: %v, want ErrNoSession", err)
	}
	if _, err := r.svc.Mint(ctx, mintReq("p-global", 1)); err != nil {
		t.Fatal(err)
	}
	if err := refresh(3); !errors.Is(err, ErrNoSession) {
		t.Errorf("refresh skipping generation 2: %v, want ErrNoSession", err)
	}
	if _, err := r.svc.RevokeSession(ctx, wire.RevokeRequest{SessionID: "s-1"}); err != nil {
		t.Fatal(err)
	}
	if err := refresh(2); !errors.Is(err, ErrNoSession) {
		t.Errorf("refresh after revoke: %v, want ErrNoSession", err)
	}
	if n := len(r.admin.generated); n != 1 {
		t.Errorf("GenerateKey called %d times, want 1 (the mint)", n)
	}
}

// Mutant killed: no re-check after GenerateKey, so a mint that another replica
// superseded while the call ran returns a token whose binding is revoked, and
// leaves its LiteLLM key behind (decision (b)).
func TestSupersededWhileGeneratingDeletesItsOwnKey(t *testing.T) {
	r := newRig(t)
	r.admin.during = func(g litellmadmin.GenerateKeyRequest) {
		if err := r.store.Revoke(context.Background(), wire.HashToken(g.Key), t0); err != nil {
			t.Error(err)
		}
	}
	_, err := r.svc.Mint(context.Background(), mintReq("p-global", 1))
	if !errors.Is(err, ErrSuperseded) {
		t.Fatalf("err = %v, want ErrSuperseded", err)
	}
	if want := [][]string{{wire.HashToken(r.admin.generated[0].Key)}}; !reflect.DeepEqual(r.admin.deleted, want) {
		t.Errorf("deleted = %v, want its own key", r.admin.deleted)
	}
}
```

```go
// Mutant killed: revoking only the newest generation, which leaves the
// generation still inside its grace working after the session is deleted.
func TestRevokeSessionRevokesEveryGeneration(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	one, err := r.svc.Mint(ctx, mintReq("p-global", 1))
	if err != nil {
		t.Fatal(err)
	}
	two, err := r.svc.Refresh(ctx, wire.RefreshRequest{SessionID: "s-1", Generation: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.RevokeSession(ctx, wire.RevokeRequest{SessionID: "s-1"}); err != nil {
		t.Fatal(err)
	}
	if n := len(liveBindings(t, r.store, t0)); n != 0 {
		t.Errorf("%d live bindings after RevokeSession, want 0", n)
	}
	var deleted []string
	for _, d := range r.admin.deleted {
		deleted = append(deleted, d...)
	}
	for _, tok := range []string{one.Token, two.Token} {
		if !slices.Contains(deleted, wire.HashToken(tok)) {
			t.Errorf("a generation's key was not deleted at LiteLLM; deleted = %v", deleted)
		}
	}
}

// Mutants killed: no final read at all (task 09's sweep then loses up to 60 s
// of every revoked key's spend); a read AFTER the delete, which LiteLLM answers
// 404 to, so the figure is lost just as surely.
func TestARevokedKeysSpendIsRecordedBeforeItsKeyIsDeleted(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	one, err := r.svc.Mint(ctx, mintReq("p-global", 1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.RevokeSession(ctx, wire.RevokeRequest{SessionID: "s-1"}); err != nil {
		t.Fatal(err)
	}
	id := wire.HashToken(one.Token)
	if want := []string{"record:" + id, "delete:" + id}; !slices.Equal(r.admin.log, want) {
		t.Errorf("call order = %v, want %v: the spend must be read while the key still exists", r.admin.log, want)
	}
	if !slices.Contains(r.rec.keys, id) {
		t.Errorf("recorded %v, want the revoked key %s", r.rec.keys, id)
	}
}

// Mutants killed: a delete that a failed record skips (the key would stay
// usable at LiteLLM until its TTL); a record failure returned instead of
// reported (RevokeSession would fail for a bookkeeping error).
func TestAFailedSpendRecordStillDeletesTheKey(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	if _, err := r.svc.Mint(ctx, mintReq("p-global", 1)); err != nil {
		t.Fatal(err)
	}
	r.rec.err = errors.New("key/info: 503")
	if _, err := r.svc.RevokeSession(ctx, wire.RevokeRequest{SessionID: "s-1"}); err != nil {
		t.Fatalf("RevokeSession = %v, want nil: a bookkeeping failure is reported, not returned", err)
	}
	if len(r.admin.deleted) == 0 {
		t.Error("the key was not deleted after the record failed")
	}
}

// Mutants killed, in the fallback for task 18's P01 failing: a caller-chosen
// key still sent; the binding made for a different key than LiteLLM's.
func TestFallbackBindsTheKeyLiteLLMChose(t *testing.T) {
	r := newRig(t, func(d *Deps) { d.LiteLLMChoosesKey = true })
	r.admin.replyKey = "sk-litellm-chose-this"
	resp, err := r.svc.Mint(context.Background(), mintReq("p-global", 1))
	if err != nil {
		t.Fatal(err)
	}
	if r.admin.generated[0].Key != "" {
		t.Error("the fallback sent a caller-chosen key")
	}
	if resp.Token != "sk-litellm-chose-this" {
		t.Errorf("token = %q, want LiteLLM's", resp.Token)
	}
	if bs := liveBindings(t, r.store, t0); len(bs) != 1 || bs[0].KeyHash != wire.HashToken(resp.Token) {
		t.Errorf("bindings = %+v, want one for LiteLLM's key", bs)
	}
}

// Mutant killed: EnsureUser and EnsureTeam remembered forever, so a LiteLLM
// user or team deleted by an operator is never re-created with its budget.
func TestEnsureIsRememberedForTenMinutes(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	for i, at := range []time.Duration{0, 9 * time.Minute, 11 * time.Minute} {
		r.now = t0.Add(at)
		req := mintReq("p-global", 1)
		req.SessionID = fmt.Sprintf("s-%d", i+1)
		if _, err := r.svc.Mint(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	if r.admin.users != 2 || r.admin.teams != 2 {
		t.Errorf("EnsureUser %d, EnsureTeam %d: want 2 each (t0, then t0+11m)", r.admin.users, r.admin.teams)
	}
}
```

The two tests of decision (h). `routes` is a map, so a test that captures it through `mut` raises a project mid-test and the Service sees it on its next read.

```go
// Review C-1, decision (h). Mutants killed: a lookup that never retires (the
// raise branch gone), a retirement dated now instead of Grace on, a second
// lookup that schedules a second delete, a delete that does not wait Grace
// (re-review R-4), and a refresh after the retire that schedules another
// (re-review R-3).
func TestARaiseRetiresAGlobalMintedKeyAfterGrace(t *testing.T) {
	var rs routes
	r := newRig(t, func(d *Deps) { rs = d.Routes.(routes) })
	ctx := context.Background()
	m, err := r.svc.Mint(ctx, mintReq("p-global", 1))
	if err != nil {
		t.Fatal(err)
	}
	h := wire.HashToken(m.Token)
	if b, err := r.svc.ByKeyHash(ctx, h); err != nil || b.RevokedAt != nil || len(r.later) != 0 {
		t.Fatalf("before the raise: RevokedAt %v, err %v, %d scheduled; want untouched", b.RevokedAt, err, len(r.later))
	}
	rs["p-global"] = wire.ProjectInference{ProjectID: "p-global", DataClass: anyRes,
		Route: dataclass.RouteAU, Effective: dataclass.RouteAU}
	want := t0.Add(5 * time.Minute)
	b, err := r.svc.ByKeyHash(ctx, h)
	if err != nil || b.RevokedAt == nil || !b.RevokedAt.Equal(want) {
		t.Fatalf("after the raise: RevokedAt %v, err %v; want %v", b.RevokedAt, err, want)
	}
	if live := liveBindings(t, r.store, want); len(live) != 0 {
		t.Errorf("%d bindings still live at the end of the grace, want 0", len(live))
	}
	if again, err := r.svc.ByKeyHash(ctx, h); err != nil || !again.RevokedAt.Equal(want) || len(r.later) != 1 {
		t.Fatalf("second lookup: RevokedAt %v, err %v, %d deletes scheduled; want %v and 1", again.RevokedAt, err, len(r.later), want)
	}
	if r.later[0].d != 5*time.Minute {
		t.Errorf("the retired key's delete waits %v, want Grace (5m): sooner cuts a stream in flight", r.later[0].d)
	}
	// The refresher follows a raise within RaiseCheck (task 15). The retire has
	// already dated this revocation and scheduled its delete, so the refresh
	// must not schedule a second (re-review R-3).
	if _, err := r.svc.Refresh(ctx, wire.RefreshRequest{SessionID: "s-1", Generation: 2}); err != nil {
		t.Fatal(err)
	}
	if len(r.later) != 1 {
		t.Fatalf("%d deletes scheduled after the refresh, want 1: the retire already scheduled it", len(r.later))
	}
	r.later[0].f()
	if w := [][]string{{h}}; !reflect.DeepEqual(r.admin.deleted, w) {
		t.Errorf("after the grace, deleted = %v, want the Global-minted key", r.admin.deleted)
	}
}

// Decision (h)'s other half. Mutants killed: retiring without a raise (the
// route check gone), retiring an AU-minted key (the RouteAtMint check gone),
// and an unreadable route returned as an error, which task 08 would turn into
// a 503 on every lookup of that key.
func TestALookupRetiresOnlyARaisedGlobalKey(t *testing.T) {
	var rs routes
	reported := 0
	r := newRig(t, func(d *Deps) { rs = d.Routes.(routes); d.Report = func(error) { reported++ } })
	ctx := context.Background()
	g, err := r.svc.Mint(ctx, mintReq("p-global", 1))
	if err != nil {
		t.Fatal(err)
	}
	req := mintReq("p-au", 1)
	req.SessionID = "s-2"
	a, err := r.svc.Mint(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	check := func(name, token string, wantReported int) {
		t.Helper()
		b, err := r.svc.ByKeyHash(ctx, wire.HashToken(token))
		if err != nil || b.RevokedAt != nil || len(r.later) != 0 || reported != wantReported {
			t.Errorf("%s: RevokedAt %v, err %v, %d scheduled, %d reported; want untouched, %d reported",
				name, b.RevokedAt, err, len(r.later), reported, wantReported)
		}
	}
	check("Global project, Global key", g.Token, 0)
	check("AU project, AU-minted key", a.Token, 0)
	delete(rs, "p-global")
	check("route unreadable", g.Token, 1)
}
```

- [ ] **Step 3: Run the tests to verify they fail**

```bash
cd control-plane && go test ./internal/keysmith/ -count=1 2>&1 | head -5
```

Expected: a build failure naming `undefined: Service`, `undefined: Deps` or `undefined: liveAt`. Any other failure means a dependency's identifiers differ from this part's (gap 4); fix the names here first.

- [ ] **Step 4: Write the implementation**

Create `control-plane/internal/keysmith/keysmith.go`:

```go
// Package keysmith mints, refreshes and revokes the per-session LiteLLM
// virtual keys (P6-D6), recording each key's binding before the key exists
// (P6-D7). It has no transport; cmd/keysmith (task 08) serves it over HTTP.
package keysmith

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/litellmadmin"
)

// BudgetDuration is LiteLLM's reset window for the user and team budgets
// keysmith ensures: the value Panacea runs (decision (g)). It is NOT our UTC
// calendar month, so two clocks bound one sum until task 18's P16 says the
// pinned image accepts a monthly one (seam S6.12, review M-7).
const BudgetDuration = "30d"

// EnsureMemo is how long a successful EnsureUser or EnsureTeam is remembered.
const EnsureMemo = 10 * time.Minute

const tokenPrefix = "sk-pa-"

// AdminAPI is the part of task 04's *litellmadmin.Client that keysmith calls.
type AdminAPI interface {
	GenerateKey(ctx context.Context, r litellmadmin.GenerateKeyRequest) (litellmadmin.GenerateKeyResponse, error)
	DeleteKeys(ctx context.Context, keys []string) error
	EnsureUser(ctx context.Context, id string, maxBudget float64, budgetDuration string) error
	EnsureTeam(ctx context.Context, id string, maxBudget float64, budgetDuration string) error
}

// SpentReader is what has been spent this period; task 09's *usage.Reader.
type SpentReader interface {
	Spent(ctx context.Context, owner wire.Owner, teamID, projectID string) (budget.Spent, error)
}

// SpendRecorder files one key's spend before its LiteLLM key is deleted, so the
// 60-second sweep's blind spot is not lost (decision (j)); task 09's
// *usage.Sweeper. It reads /key/info itself: keysmith holds no spend figure.
type SpendRecorder interface {
	RecordKey(ctx context.Context, b wire.Binding) error
}

// Deps is everything a Service needs. Clock, Rand, After, Report, TTL and
// Grace have defaults; the rest are required.
type Deps struct {
	Admin    AdminAPI
	Store    binding.Store
	Budgets  budget.Policy
	Spent    SpentReader
	Recorder SpendRecorder // required: decision (j); nil would lose spend silently
	Routes   wire.RouteSource
	Groups  []string // every LiteLLM model group: "global/<id>" and "au/<id>" (task 02)
	Clock   func() time.Time
	Rand    io.Reader
	After   func(time.Duration, func()) // nil: time.AfterFunc
	Report  func(error)                 // best-effort failures; nil: dropped
	TTL     time.Duration               // 0: one hour
	Grace   time.Duration               // 0: five minutes
	BaseURL string                      // what a sandbox sets as ANTHROPIC_BASE_URL
	// LiteLLMChoosesKey is the fallback if task 18's P01 shows LiteLLM refuses
	// a caller-chosen key: generate first, then bind the key the reply names.
	LiteLLMChoosesKey bool
}

var (
	ErrInvalid       = errors.New("keysmith: invalid request")
	ErrBudgetUnknown = errors.New("keysmith: the budget or the spend could not be read")
	ErrNoSession     = errors.New("keysmith: no live binding of the previous generation")
	ErrBusy          = errors.New("keysmith: another mint of this generation holds the binding")
	ErrSuperseded    = errors.New("keysmith: superseded by a retry of the same generation")
	ErrAdmin         = errors.New("keysmith: a LiteLLM admin call failed")
	ErrStore         = errors.New("keysmith: the binding store failed")
	// ErrTeamMismatch is an ErrInvalid, so task 08 maps it to 400 bad_request
	// with no new code (decision (i), review I-7).
	ErrTeamMismatch = fmt.Errorf("%w: the caller's team is not the project's", ErrInvalid)
)

// BudgetExhaustedError names the dimension with nothing left.
type BudgetExhaustedError struct{ Dimension budget.Dimension }

func (e *BudgetExhaustedError) Error() string {
	return "keysmith: budget exhausted: " + string(e.Dimension)
}
```

```go
type memoKey struct {
	kind, id string
	usd      float64
}

// Service is safe for concurrent use. Calls for one session are serialised by
// one of 64 striped locks; across replicas, the store decides (decision (b)).
type Service struct {
	d       Deps
	locks   [64]sync.Mutex
	mu      sync.Mutex
	ensured map[memoKey]time.Time
}

func New(d Deps) (*Service, error) {
	if d.Admin == nil || d.Store == nil || d.Budgets == nil || d.Spent == nil ||
		d.Recorder == nil || d.Routes == nil {
		return nil, errors.New("keysmith: Admin, Store, Budgets, Spent, Recorder and Routes are all required")
	}
	var global, au bool
	for _, g := range d.Groups {
		switch {
		case len(g) > len("global/") && strings.HasPrefix(g, "global/"):
			global = true
		case len(g) > len("au/") && strings.HasPrefix(g, "au/"):
			au = true
		default:
			return nil, fmt.Errorf("keysmith: model group %q is neither global/<id> nor au/<id>", g)
		}
	}
	if !global || !au {
		return nil, errors.New("keysmith: Groups needs at least one global/ and one au/ group")
	}
	u, err := url.Parse(d.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" {
		return nil, fmt.Errorf("keysmith: BaseURL %q must be http(s)://host[:port] with no path", d.BaseURL)
	}
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Rand == nil {
		d.Rand = rand.Reader
	}
	if d.After == nil {
		d.After = func(t time.Duration, f func()) { time.AfterFunc(t, f) }
	}
	if d.Report == nil {
		d.Report = func(error) {}
	}
	if d.TTL == 0 {
		d.TTL = time.Hour
	}
	if d.Grace == 0 {
		d.Grace = 5 * time.Minute
	}
	if d.Grace < 0 || d.TTL <= d.Grace {
		return nil, fmt.Errorf("keysmith: TTL %v must exceed Grace %v", d.TTL, d.Grace)
	}
	return &Service{d: d, ensured: map[memoKey]time.Time{}}, nil
}

// subject is who and where a key is for. A refresh copies it from the
// previous generation's binding, so a refresh cannot move a session.
type subject struct {
	session, team, project, agent string
	owner                         wire.Owner
	pod                           netip.Addr
}

// Mint mints generation req.Generation of a session. A retry of a generation
// first revokes whatever that generation already has live (decision (a)).
func (s *Service) Mint(ctx context.Context, req wire.MintRequest) (wire.MintResponse, error) {
	if err := req.Validate(); err != nil {
		return wire.MintResponse{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	pod, err := netip.ParseAddr(req.PodAddr)
	if err != nil {
		return wire.MintResponse{}, fmt.Errorf("%w: pod address: %v", ErrInvalid, err)
	}
	defer s.lock(req.SessionID)()
	all, err := s.d.Store.BySession(ctx, req.SessionID)
	if err != nil {
		return wire.MintResponse{}, fmt.Errorf("%w: %v", ErrStore, err)
	}
	if err := s.supersede(ctx, all, req.Generation); err != nil {
		return wire.MintResponse{}, err
	}
	return s.mint(ctx, subject{session: req.SessionID, team: req.TeamID, project: req.ProjectID,
		agent: req.AgentID, owner: req.Owner, pod: pod.Unmap()}, req.Generation)
}
```

```go
// Refresh mints generation req.Generation from the live binding of the one
// before it, then dates that binding's revocation Grace from now, so a stream
// in flight on the old key is not cut (decision (c)).
func (s *Service) Refresh(ctx context.Context, req wire.RefreshRequest) (wire.MintResponse, error) {
	if req.SessionID == "" || req.Generation < 2 {
		return wire.MintResponse{}, fmt.Errorf("%w: refresh needs a session and a generation of 2 or more", ErrInvalid)
	}
	defer s.lock(req.SessionID)()
	all, err := s.d.Store.BySession(ctx, req.SessionID)
	if err != nil {
		return wire.MintResponse{}, fmt.Errorf("%w: %v", ErrStore, err)
	}
	now := s.d.Clock()
	var prev *wire.Binding
	for i := range all {
		if all[i].Generation == req.Generation-1 && liveAt(all[i], now) {
			prev = &all[i]
		}
	}
	if prev == nil {
		return wire.MintResponse{}, fmt.Errorf("%w: session %s generation %d", ErrNoSession, req.SessionID, req.Generation-1)
	}
	if err := s.supersede(ctx, all, req.Generation); err != nil {
		return wire.MintResponse{}, err
	}
	resp, err := s.mint(ctx, subject{session: prev.SessionID, team: prev.TeamID, project: prev.ProjectID,
		agent: prev.AgentID, owner: prev.Owner, pod: prev.PodAddr}, req.Generation)
	if err != nil {
		return wire.MintResponse{}, err
	}
	// Never move a revocation later: a retried refresh keeps the first date.
	// Whoever dates the revocation schedules the delete, and only they: a
	// retried refresh, or a refresh after a raise's retire (decision (h)),
	// finds the date set and schedules nothing (re-review R-3).
	retire := s.d.Clock().Add(s.d.Grace)
	if prev.RevokedAt == nil || prev.RevokedAt.After(retire) {
		if err := s.d.Store.Revoke(context.WithoutCancel(ctx), prev.KeyHash, retire); err != nil {
			s.d.Report(fmt.Errorf("keysmith: dating the revocation of session %s generation %d (it still expires at %s): %w",
				prev.SessionID, prev.Generation, prev.ExpiresAt.Format(time.RFC3339), err))
		}
		old := prev // the whole binding: the delete records its spend first (decision (j))
		s.d.After(s.d.Grace, func() { s.deleteKeys(context.Background(), old) })
	}
	return resp, nil
}

// RevokeSession revokes every generation of a session now and deletes their
// LiteLLM keys. The store's revocation is what the gatekeeper obeys, within
// its binding cache (P6-D7); the LiteLLM delete stops spend at once.
func (s *Service) RevokeSession(ctx context.Context, req wire.RevokeRequest) (int, error) {
	if req.SessionID == "" {
		return 0, fmt.Errorf("%w: no session", ErrInvalid)
	}
	defer s.lock(req.SessionID)()
	all, err := s.d.Store.BySession(ctx, req.SessionID)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrStore, err)
	}
	now := s.d.Clock()
	n, err := s.d.Store.RevokeSession(ctx, req.SessionID, now)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrStore, err)
	}
	var live []wire.Binding
	for _, b := range all {
		if now.Before(b.ExpiresAt) {
			live = append(live, b)
		}
	}
	if len(live) > 0 {
		s.deleteKeys(context.WithoutCancel(ctx), live...)
	}
	return n, nil
}

var _ wire.BindingReader = (*Service)(nil)

// ByKeyHash is the gatekeeper's binding lookup (task 08 serves it). A live key
// minted on Global whose project is now AU has its revocation dated Grace from
// now, as Refresh dates the generation it replaces, and its LiteLLM key is
// deleted then (decision (h)). An unreadable route changes nothing here: the
// gatekeeper reads the route itself and fails closed (task 10).
func (s *Service) ByKeyHash(ctx context.Context, keyHash string) (wire.Binding, error) {
	b, err := s.d.Store.ByKeyHash(ctx, keyHash)
	if err != nil || b.RouteAtMint != dataclass.RouteGlobal || !liveAt(b, s.d.Clock()) {
		return b, err
	}
	route, err := s.route(ctx, b.ProjectID)
	if err != nil {
		s.d.Report(fmt.Errorf("keysmith: the route of project %s, for a Global-minted key: %w", b.ProjectID, err))
		return b, nil
	}
	if route != dataclass.RouteAU {
		return b, nil
	}
	retire := s.d.Clock().Add(s.d.Grace)
	if b.RevokedAt != nil && !b.RevokedAt.After(retire) {
		return b, nil // already retiring no later than a raise would
	}
	if err := s.d.Store.Revoke(context.WithoutCancel(ctx), b.KeyHash, retire); err != nil {
		s.d.Report(fmt.Errorf("keysmith: retiring session %s generation %d after a raise: %w", b.SessionID, b.Generation, err))
		return b, nil // the next lookup tries again; the gatekeeper still sends it on AU
	}
	b.RevokedAt = &retire
	retired := b // the whole binding: the delete records its spend first (decision (j))
	s.d.After(s.d.Grace, func() { s.deleteKeys(context.Background(), retired) })
	return b, nil
}

// supersede revokes every live binding of generation gen and deletes its key.
// The caller asking for gen again never received it (decision (a)).
func (s *Service) supersede(ctx context.Context, all []wire.Binding, gen int) error {
	now := s.d.Clock()
	for _, b := range all {
		if b.Generation != gen || !liveAt(b, now) {
			continue
		}
		if err := s.d.Store.Revoke(ctx, b.KeyHash, now); err != nil {
			return fmt.Errorf("%w: revoking the superseded key: %v", ErrStore, err)
		}
		s.deleteKeys(context.WithoutCancel(ctx), b)
	}
	return nil
}
```

```go
// mint runs the checks in the skeleton's order: route, then budget, then the
// LiteLLM user and team, then the binding, then the key. Nothing reaches
// LiteLLM until the route and the budget both allow it.
func (s *Service) mint(ctx context.Context, sub subject, gen int) (wire.MintResponse, error) {
	route, team, err := s.project(ctx, sub.project)
	if err != nil {
		return wire.MintResponse{}, err
	}
	// Decision (i), review I-7: attribution and the team budget key on the
	// project's team, never on the caller's claim. "" is a source with no teams.
	if team != "" && team != sub.team {
		return wire.MintResponse{}, fmt.Errorf("%w: project %s is team %s's, not %s",
			ErrTeamMismatch, sub.project, team, sub.team)
	}
	limits, err := s.d.Budgets.Limits(ctx, sub.team, sub.project)
	if err != nil {
		return wire.MintResponse{}, fmt.Errorf("%w: limits: %v", ErrBudgetUnknown, err)
	}
	spent, err := s.d.Spent.Spent(ctx, sub.owner, sub.team, sub.project)
	if err != nil {
		return wire.MintResponse{}, fmt.Errorf("%w: spent: %v", ErrBudgetUnknown, err)
	}
	maxUSD, dim, ok := budget.Slice(limits, spent)
	if !ok {
		if dim == budget.None {
			return wire.MintResponse{}, fmt.Errorf("%w: the session cap is unusable", ErrBudgetUnknown)
		}
		return wire.MintResponse{}, &BudgetExhaustedError{Dimension: dim}
	}
	userID := sub.owner.TID + ":" + sub.owner.OID
	if err := s.ensure(ctx, "user", userID, limits.PersonUSD); err != nil {
		return wire.MintResponse{}, err
	}
	if err := s.ensure(ctx, "team", sub.project, limits.ProjectUSD); err != nil {
		return wire.MintResponse{}, err
	}
	now := s.d.Clock()
	b := wire.Binding{SessionID: sub.session, Generation: gen, Owner: sub.owner, TeamID: sub.team,
		ProjectID: sub.project, AgentID: sub.agent, PodAddr: sub.pod, RouteAtMint: route,
		CreatedAt: now, ExpiresAt: now.Add(s.d.TTL)}
	req := litellmadmin.GenerateKeyRequest{
		KeyAlias: fmt.Sprintf("sess/%s/%d", sub.session, gen), Duration: duration(s.d.TTL),
		UserID: userID, TeamID: sub.project, Models: s.groupsFor(route), MaxBudget: maxUSD,
		Metadata: map[string]string{"tid": sub.owner.TID, "oid": sub.owner.OID, "team": sub.team,
			"project": sub.project, "agent": sub.agent, "session": sub.session, "route": string(route)},
	}
	issue := s.bindThenGenerate
	if s.d.LiteLLMChoosesKey {
		issue = s.generateThenBind
	}
	token, err := issue(ctx, req, &b)
	if err != nil {
		return wire.MintResponse{}, err
	}
	return wire.MintResponse{Token: token, KeyID: b.KeyID, ExpiresAt: b.ExpiresAt, Route: route,
		BaseURL: s.d.BaseURL, MaxBudgetUSD: maxUSD}, nil
}

// route is the project's route now, by wire's one rule (task 03 decision (g)),
// which the session refresher also uses to see a raise (re-review R-6).
// Anything unparsed is route_unknown, never Global (P6-D5).
func (s *Service) route(ctx context.Context, project string) (dataclass.Route, error) {
	r, _, err := s.project(ctx, project)
	return r, err
}

// project is route plus the project's team as the source read it (task 03
// decision (h)): one read, so the route and the team cannot come from two
// different versions of the record.
func (s *Service) project(ctx context.Context, project string) (dataclass.Route, string, error) {
	pi, err := s.d.Routes.Project(ctx, project)
	if err != nil {
		return "", "", fmt.Errorf("%w: project %s: %v", wire.ErrRouteUnknown, project, err)
	}
	r, err := pi.LiveRoute()
	if err != nil {
		return "", "", fmt.Errorf("%w: project %s: %v", wire.ErrRouteUnknown, project, err)
	}
	return r, pi.TeamID, nil
}
```

```go
// bindThenGenerate is the primary path: the binding exists before the key, so
// no LiteLLM key ever lacks a record that can find and delete it.
func (s *Service) bindThenGenerate(ctx context.Context, req litellmadmin.GenerateKeyRequest, b *wire.Binding) (string, error) {
	token, err := s.newToken()
	if err != nil {
		return "", err
	}
	b.KeyHash = wire.HashToken(token)
	b.KeyID = b.KeyHash // decision (f)
	if err := s.d.Store.Put(ctx, *b); err != nil {
		if errors.Is(err, binding.ErrExists) {
			return "", ErrBusy
		}
		return "", fmt.Errorf("%w: %v", ErrStore, err)
	}
	req.Key = token
	if _, err := s.d.Admin.GenerateKey(ctx, req); err != nil {
		s.kill(ctx, *b) // a generate that timed out may still have made the key
		return "", scrub(ErrAdmin, "generate key", err, token)
	}
	cur, err := s.d.Store.ByKeyHash(ctx, b.KeyHash)
	if err != nil {
		s.kill(ctx, *b)
		return "", fmt.Errorf("%w: re-reading the binding: %v", ErrStore, err)
	}
	if !liveAt(cur, s.d.Clock()) {
		s.kill(ctx, *b)
		return "", ErrSuperseded
	}
	return token, nil
}

// generateThenBind is the fallback for task 18's P01 failing: LiteLLM chooses
// the key and the binding follows. A generate that fails after LiteLLM made a
// key leaves that key with no binding; the gatekeeper refuses it, and it
// expires at TTL. That window is the price of the fallback.
func (s *Service) generateThenBind(ctx context.Context, req litellmadmin.GenerateKeyRequest, b *wire.Binding) (string, error) {
	req.Key = ""
	resp, err := s.d.Admin.GenerateKey(ctx, req)
	if err != nil {
		return "", scrub(ErrAdmin, "generate key", err)
	}
	token := resp.Key
	if !strings.HasPrefix(token, "sk-") {
		return "", fmt.Errorf("%w: generate key: the reply names no usable key", ErrAdmin)
	}
	b.KeyHash = wire.HashToken(token)
	b.KeyID = b.KeyHash
	if err := s.d.Store.Put(ctx, *b); err != nil {
		s.deleteKeys(context.WithoutCancel(ctx), *b) // spend is zero here; decision (j) keeps the one rule
		if errors.Is(err, binding.ErrExists) {
			return "", ErrBusy
		}
		return "", scrub(ErrStore, "put binding", err, token)
	}
	return token, nil
}

// kill revokes a binding now and deletes its key, on a context the caller
// cannot cancel. Failures are reported, not returned: the caller already has
// an error to return, and a revoked binding is refused by the gatekeeper.
func (s *Service) kill(ctx context.Context, b wire.Binding) {
	bg := context.WithoutCancel(ctx)
	if err := s.d.Store.Revoke(bg, b.KeyHash, s.d.Clock()); err != nil {
		s.d.Report(fmt.Errorf("keysmith: revoking session %s generation %d: %w", b.SessionID, b.Generation, err))
	}
	s.deleteKeys(bg, b)
}

// deleteKeys records each key's final spend, then deletes them. The record
// comes first and has no exception (decision (j)): task 09's sweep runs every
// 60 seconds and keeps only the last figure a key showed, so a delete without a
// final read loses up to a minute of every refreshed and every revoked key's
// spend, and the team dimension is keysmith's own to enforce. A record that
// fails is reported and the delete goes ahead: a key that cannot be read is
// still a key that must not stay usable.
func (s *Service) deleteKeys(ctx context.Context, bs ...wire.Binding) {
	keys := make([]string, 0, len(bs))
	for _, b := range bs {
		if err := s.d.Recorder.RecordKey(ctx, b); err != nil {
			s.d.Report(fmt.Errorf("keysmith: recording the final spend of session %s generation %d before deleting its key: %w",
				b.SessionID, b.Generation, err))
		}
		keys = append(keys, b.KeyID)
	}
	if len(keys) == 0 {
		return
	}
	if err := s.d.Admin.DeleteKeys(ctx, keys); err != nil {
		s.d.Report(scrub(ErrAdmin, "delete keys", err))
	}
}
```

```go
// ensure creates or updates the LiteLLM user or team with its budget, at most
// once per EnsureMemo for the same id and figure.
func (s *Service) ensure(ctx context.Context, kind, id string, usd float64) error {
	k := memoKey{kind, id, usd}
	now := s.d.Clock()
	s.mu.Lock()
	until, ok := s.ensured[k]
	s.mu.Unlock()
	if ok && now.Before(until) {
		return nil
	}
	var err error
	if kind == "user" {
		err = s.d.Admin.EnsureUser(ctx, id, usd, BudgetDuration)
	} else {
		err = s.d.Admin.EnsureTeam(ctx, id, usd, BudgetDuration)
	}
	if err != nil {
		return scrub(ErrAdmin, "ensure "+kind, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ensured) >= 4096 {
		for old, u := range s.ensured {
			if !now.Before(u) {
				delete(s.ensured, old)
			}
		}
	}
	s.ensured[k] = now.Add(EnsureMemo)
	return nil
}

// lock takes the session's stripe and returns its unlock.
func (s *Service) lock(session string) func() {
	h := fnv.New32a()
	h.Write([]byte(session))
	m := &s.locks[h.Sum32()%uint32(len(s.locks))]
	m.Lock()
	return m.Unlock
}

// groupsFor is the model groups a key minted on r may name (decision (d)).
func (s *Service) groupsFor(r dataclass.Route) []string {
	if r != dataclass.RouteAU {
		return append([]string(nil), s.d.Groups...)
	}
	var out []string
	for _, g := range s.d.Groups {
		if strings.HasPrefix(g, "au/") {
			out = append(out, g)
		}
	}
	return out
}

func (s *Service) newToken() (string, error) {
	var raw [32]byte
	if _, err := io.ReadFull(s.d.Rand, raw[:]); err != nil {
		return "", fmt.Errorf("keysmith: token randomness: %w", err)
	}
	return tokenPrefix + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

// liveAt is task 03's (wire.Binding).LiveAt: live before its expiry and before
// its revocation time, so a revocation dated in the future is a scheduled one
// (decision (c)). The gatekeeper (task 10) calls the same function.
func liveAt(b wire.Binding, now time.Time) bool { return b.LiveAt(now) }

// duration renders a TTL in LiteLLM's duration syntax ("1h", "45m", "90s").
func duration(d time.Duration) string {
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	}
	return fmt.Sprintf("%ds", (d+time.Second-1)/time.Second)
}

// scrub wraps cause under kind with every secret replaced (decision (e)).
func scrub(kind error, what string, cause error, secrets ...string) error {
	msg := cause.Error()
	for _, sec := range secrets {
		if sec != "" {
			msg = strings.ReplaceAll(msg, sec, "[redacted]")
		}
	}
	return fmt.Errorf("%w: %s: %s", kind, what, msg)
}
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
cd control-plane && go test ./internal/keysmith/ -count=1 -race -v 2>&1 | grep -c '^--- PASS'
```

Expected: `19`, and no `--- FAIL` line. Seventeen, plus decision (j)'s two.

- [ ] **Step 6: Prove the mutants die**

Apply each edit alone, run `go test ./internal/keysmith/ -count=1`, confirm the named test fails, and revert the edit.

| Edit in `keysmith.go` | Test that must fail |
|---|---|
| in `mint`, move the two `s.ensure` calls above `s.route` | `TestMintChecksTheRouteBeforeAnyLiteLLMCall` |
| in `Mint`, delete the `s.supersede` call | `TestRetriedGenerationLeavesOneLiveKey` |
| in `bindThenGenerate`, replace the first `s.kill(ctx, *b)` with nothing | `TestFailedGenerateKeyLeavesNoLiveBinding` |
| in `bindThenGenerate`, `scrub(ErrAdmin, "generate key", err, token)` → `fmt.Errorf("%w: %v", ErrAdmin, err)` | `TestErrorsNeverCarryTheToken` |
| in `Refresh`, `retire := s.d.Clock().Add(s.d.Grace)` → `retire := s.d.Clock()` | `TestRefreshKeepsTheOldKeyForGraceOnly` |
| in `groupsFor`, `if r != dataclass.RouteAU` → `if true` | `TestAUKeyNamesOnlyAUGroups` |
| in `bindThenGenerate`, delete the `if !liveAt(cur, …)` block | `TestSupersededWhileGeneratingDeletesItsOwnKey` |
| in `mint`, `return … fmt.Errorf("%w: spent: %v", ErrBudgetUnknown, err)` → `spent = budget.Spent{}` and carry on | `TestMintFailsClosedWhenSpendCannotBeRead` |
| in `ByKeyHash`, delete the `if route != dataclass.RouteAU { return b, nil }` block | `TestALookupRetiresOnlyARaisedGlobalKey` (the Global project) |
| in `ByKeyHash`, delete `b.RouteAtMint != dataclass.RouteGlobal \|\|` | `TestALookupRetiresOnlyARaisedGlobalKey` (the AU-minted key) |
| in `ByKeyHash`, `retire := s.d.Clock().Add(s.d.Grace)` → `retire := s.d.Clock()` | `TestARaiseRetiresAGlobalMintedKeyAfterGrace` |
| in `ByKeyHash`, delete the `if b.RevokedAt != nil && … { return b, nil }` block | `TestARaiseRetiresAGlobalMintedKeyAfterGrace` (the second lookup schedules a second delete) |
| in `ByKeyHash`, after the route read, `return b, nil` → `return b, err` | `TestALookupRetiresOnlyARaisedGlobalKey` (route unreadable) |
| in `ByKeyHash`, `s.d.After(s.d.Grace, …)` → `s.d.After(0, …)` | `TestARaiseRetiresAGlobalMintedKeyAfterGrace` (the delete waits `0s`, want 5m; re-review R-4) |
| in `Refresh`, `s.d.After(s.d.Grace, …)` → `s.d.After(0, …)` | `TestRefreshKeepsTheOldKeyForGraceOnly` (the delete waits `0s`, want 5m; re-review R-4) |
| in `Refresh`, move `keyID := prev.KeyID` and the `s.d.After` call back below the `if` | `TestARaiseRetiresAGlobalMintedKeyAfterGrace` (2 deletes scheduled after the refresh, want 1; re-review R-3) |
| in `mint`, delete the `if team != "" && team != sub.team { … }` block | `TestMintRefusesATeamThatIsNotTheProjects` (`budget_unknown`, want `ErrTeamMismatch`; review I-7) |
| in `mint`, move that block below the `s.d.Spent.Spent` read | `TestMintRefusesATeamThatIsNotTheProjects` (`budget_unknown`, want `ErrTeamMismatch`) |
| in `mint`, `team != "" && team != sub.team` → `team != sub.team` | `TestMintRefusesATeamThatIsNotTheProjects` (`p-noteam` refused, want a mint) |
| in `deleteKeys`, delete the `s.d.Recorder.RecordKey` call and keep the `keys = append(...)` | `TestARevokedKeysSpendIsRecordedBeforeItsKeyIsDeleted` (log is `[delete:…]`, want `record:` first; review M-3) |
| in `deleteKeys`, move the `RecordKey` call into a second loop **after** `s.d.Admin.DeleteKeys` | `TestARevokedKeysSpendIsRecordedBeforeItsKeyIsDeleted` (order `delete:`,`record:`, want the reverse: LiteLLM answers `404` once the key is gone) |
| in `deleteKeys`, on a `RecordKey` error `return` instead of `s.d.Report(…)` | `TestAFailedSpendRecordStillDeletesTheKey` (no `delete:` entry: a key that cannot be read stays usable until its `TTL`) |

- [ ] **Step 7: Vet and format**

```bash
cd control-plane
go vet ./internal/keysmith/ && gofmt -l internal/keysmith/     # expect no output
grep -n 'log\.\|slog\.\|fmt.Print' internal/keysmith/keysmith.go  # expect no output: the core never logs
```

The last line is global constraint 3 at its source: nothing in the core writes a log line, so nothing in it can log a token. Task 08 owns `Report`.

- [ ] **Step 8: Commit**

```bash
git add control-plane/internal/keysmith/keysmith.go control-plane/internal/keysmith/keysmith_test.go
git commit -F <message file> -- control-plane/internal/keysmith/keysmith.go control-plane/internal/keysmith/keysmith_test.go
```

Message: `feat(keysmith): mint, refresh and revoke, one live key per generation (phase 6 task 07)`, a body naming decisions (a) to (i), and the attribution line.

**Acceptance:** step 5 prints `19` under `-race`; step 6's twenty-two edits each fail the named test, the last three of them decision (j)'s; step 7 prints nothing.

**What this task does not claim:**

- **Transport or caller identity.** Who may call `Mint` is task 08's `callerauth`. This package trusts its caller for the owner and the session, but not for the team: decision (i) checks `TeamID` against the project's (review I-7). Whether the owner belongs to that team is the caller's check (ask P8-1; task 15's launcher runs as the person, in local mode).
- **That LiteLLM accepts a caller-chosen `key`, honours `duration`, `max_budget`, `models` and `metadata` on `/key/generate`, or names a key by the SHA-256 of its value.** All from LiteLLM's documented admin API and training knowledge, **UNVERIFIED** against the pinned image until task 18 (P01, P02, P05, P11, P12). P01 failing switches `LiteLLMChoosesKey` on; the code for that path is written and tested here.
- **That LiteLLM enforces unique `key_alias` values.** Nothing here relies on it. If it does, a retry whose delete of the superseded key failed gets `ErrAdmin` from `GenerateKey`, and the caller's next retry converges.
- **Exactly one LiteLLM key per generation at every instant.** Between a timed-out `GenerateKey` and the `kill` that follows it, a key may exist whose binding is already revoked. The gatekeeper refuses it (task 10), and nothing but the gatekeeper and keysmith can reach LiteLLM (P6-D1). In the fallback path, a key whose reply was lost has no binding at all until it expires at `TTL`.
- **That a raise retires a Global-minted key nobody uses.** Decision (h) acts on a lookup, and only the gatekeeper looks a key up. A key that makes no request after the raise keeps its `global/*` groups until its refresh or its `TTL`. It has nothing to send them with, since only the gatekeeper and keysmith reach LiteLLM (P6-D1). Whether LiteLLM then refuses a body fallback outside an AU-minted key's `au/*` is task 18's P14.
- **A lock across replicas.** Two replicas racing one generation converge through `ErrBusy`, `ErrSuperseded` and a retry, not through mutual exclusion.
- **Freedom from head-of-line waiting.** A session's stripe is held across LiteLLM calls (up to task 04's 20-second deadline) and across decision (j)'s record (up to `usage.RecordTimeout`, 5 s), so up to 1 in 64 other sessions' mints can wait behind a slow one. Every one of those waits is bounded; none is deadline-free.

**Gaps against the skeleton, for the orchestrator:**

1. **Task 05's uniqueness rule must count live rows only.** *Closed by fix batch b06:* the skeleton's `ErrExists` on "a duplicate `(session, generation)`" would have refused the re-mint that decision (a) requires, because the superseded row still exists; the skeleton now carries the partial index, the `least(revoked_at, at)` rule and the pitfall row that used to claim a mint is idempotent on `key_alias` (`2026-09-25-phase-6-gateway.md:102`, `:230`). Task 05 needs `unique (session_id, generation) where revoked_at is null` (a partial index), the memory store the same rule, and `bindingtest` a case for it. Task 05 also needs `Revoke` and `RevokeSession` to accept a future `at` and never move an existing `revoked_at` later (`least(revoked_at, at)`); decision (c) depends on both.
2. **`budget_unknown` (503)** is not in task 03's code list. Task 06 proposed it (its gap 1); this part uses it for `ErrBudgetUnknown`.
3. **`liveAt` belongs in `wire`**, as `(Binding) LiveAt(now time.Time) bool`, so keysmith and the gatekeeper (task 10) cannot drift. Task 03 has adopted it (its decision (b)), with `budget_unknown` (its decision (c)); `liveAt` here is a one-line call to it.
4. **Task 04's identifiers are assumed:** `GenerateKeyRequest{Key, KeyAlias, Duration, UserID, TeamID, Models []string, MaxBudget float64, Metadata map[string]string}`, `GenerateKeyResponse{Key string}`, `DeleteKeys(ctx, []string) error`, `EnsureUser`/`EnsureTeam(ctx, id string, maxBudget float64, budgetDuration string) error`; and task 05's `binding.NewMemStore()`. Step 1 checks them.
5. **The skeleton gives every key both groups;** decision (d) narrows an AU-minted key to `au/*`. No other part is affected: the gatekeeper already keeps such a key on AU.
6. **The skeleton's file map names a `Revoke`** besides `RevokeSession`. With `wire.RevokeRequest{SessionID}` the only revoke on the wire, a single-key revoke is internal (`kill`, `supersede`) and not exported.
7. **The re-mint on a raise is the refresher's (decision (h), review C-1).** keysmith retires a raised Global-minted key `Grace` after its first lookup, but only a caller can deliver the new key. Three hand-offs follow; fix batch b01 made the first two in their parts. **Task 15:** the `Launcher`'s refresh loop re-reads its project's route every `RaiseCheck` (one minute) while the key was minted on Global, and refreshes at once when the route is AU (done: `sleepUnlessRaised`, `TestARaiseRefreshesAtOnce`, its gap 7). **Task 08:** its `apiDeps.Bindings` is wired to the `*keysmith.Service`, not to the store (done in its `run`, its gap 8; the one line has no test of its own). **Ask P8-1:** the cloud broker refreshes on a raise the same way. Without these, a raise leaves the session refused from the end of `Grace` to its next scheduled refresh. That fails closed, but it is a usability failure the owner would see.
8. **The team comes from the project (decision (i), review I-7).** Task 03 adds `ProjectInference.TeamID` (its decision (h)), task 13's `Hub` source fills it from the record's `teamId` and refuses a record without one (its decision (f)), and `mint` refuses a caller whose `TeamID` differs. **Ask P8-1** adds that the broker checks the owner's membership of the project's team in the Hub before it mints, since keysmith holds no user token. The skeleton's P6-D6 mint had no such check.
9. **`Deps.Recorder` is new and required (decision (j), review M-3).** Two parts must match it. **Task 09** implements `RecordKey(ctx, wire.Binding) error` on `*usage.Sweeper`: one `KeyInfo` read, one `Upsert`, the period from `budget.PeriodStart(b.CreatedAt)`, and the same never-lower rule the sweep uses. **Task 08** passes that sweeper as `Recorder` where it already passes the reader as `Spent`; keysmith declares the interface, so nothing imports `usage` that did not already. Neither task's dependency order changes: 07 still depends on 04, 05 and 06, and 09 on 04 and 06.
