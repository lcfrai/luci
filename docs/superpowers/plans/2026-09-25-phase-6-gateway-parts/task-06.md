### Task 06: Budgets: three dimensions, defaults on, the per-key slice

A session key carries one number that decides how much it may spend: LiteLLM's `max_budget`. This task computes that number. It is the least of what the person, the project and the team each have left this month, and of a per-session cap, so the tightest of the three dimensions always wins (P6-D8). LiteLLM enforces the person (its user budget) and the project (its team budget) natively; nothing in LiteLLM's open-source build that this plan can rely on enforces a third tier, so the team is enforced here, at mint, from task 09's rollup.

**The one thing this task must not get wrong is what "no number" means.** 05-design §2.5 puts it in one line: *"an unset quota is the org pool by another name"* (`docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md:232-236`, verified 2026-09-25). OD-24 turns that into a rule: budgets are on by default. So in this package a zero, a negative, a `NaN` or an infinity is **never** unlimited. A zero limit is an exhausted dimension. A zero from the Hub's policy falls back to the configured default. A default that is zero refuses to be constructed. And an unconstructed `Static` refuses to answer at all.

This package is pure: standard library only, no I/O, no clock. The Hub-backed `Policy` is ask H-2 and seam S6.2, and it is not built or stubbed here; `Layered` is the shape it plugs into.

**Files:**
- `control-plane/internal/budget/budget.go`: `Dimension`, `Period`, `Limits`, `Spent`, `Policy`, `Static`, `Layered`, `Slice`, `PeriodStart`, `PeriodEnd`, `Proposed` (new)
- `control-plane/internal/budget/budget_test.go`: twelve tests, one mutant each (new)

**Interfaces:**
- Produces:
  - `budget.Dimension` with `Person` (`"person"`), `Project` (`"project"`), `Team` (`"team"`) and `None` (`""`). The three strings are the same as task 03's `UsageRow.Scope` values, and a golden test here pins them so a rename in either package is a red test, not a silent mismatch
  - `budget.Limits`, `budget.Spent`, the `budget.Policy` interface, `budget.NewStatic`, `budget.Layered`, `budget.Slice`, `budget.PeriodStart`, `budget.PeriodEnd`, `budget.Proposed`, `budget.ErrNotConfigured`, `budget.ErrInvalidLimits`, `budget.ErrPolicyUnavailable`
- Consumes: nothing outside the standard library
- Required by: task 07 (`Deps.Budgets` is a `budget.Policy`; `Slice` decides the key's `MaxBudget` and the `budget_exhausted` dimension; `Limits.PersonUSD` and `ProjectUSD` are the budgets passed to `EnsureUser` and `EnsureTeam`); task 08 (config builds `NewStatic` from `PANACEA_APP_GATEWAY_SESSION_CAP_USD` and `DEFAULT_{USER,TEAM,PROJECT}_BUDGET_USD`, defaulting to `Proposed`); task 09 (`Spent` is its reader's return type; `PeriodStart` is its month boundary); task 21 (the usage rows' `BudgetUSD`)

**On the skeleton's "depends on 01, 03".** Nothing here imports `dataclass` or `wire`. The dependency is on task 03's scope strings only, and it is honoured by a golden test on the literal values rather than an import, so this package stays a leaf that task 09 and task 07 can both import without a cycle.

**Decisions where the skeleton is silent:**

**(a) `Static` keeps its limits unexported.** The skeleton writes `Static{Default Limits}`. An exported field lets `budget.Static{}` be written anywhere and bypass the constructor that refuses zero, which is the very hole this task closes. So the field is unexported, `NewStatic` is the only way to fill it, and the zero `Static` answers `ErrNotConfigured`. `Layered.Fallback` is still a `Static`, as the skeleton says.

**(b) A primary that errors fails closed; it does not fall back.** A zero from the primary is "unset", and unset means the default (the skeleton's rule). An *error* from the primary means "we could not tell", and P6-D5 forbids turning that into anything permissive. The default can be looser than an admin's tighter Hub value, so falling back on error would loosen a budget while the Hub is down, and OD-24 says tighten, never loosen. `Layered` returns `ErrPolicyUnavailable` and task 07 refuses the mint with a 503. Task 03's code list has no code for this case; see the gap at the end.

**(c) Every field falls back on its own.** A primary that sets a project budget and leaves the team unset gets the team default and keeps its project value. `Period` is always `monthly`: a primary naming any other period is replaced, because task 09's rollup only sums calendar months.

**(d) Spent below zero counts as zero.** A negative spend is a bug upstream, and subtracting it would raise the remainder above the limit.

**(e) The period is the UTC calendar month.** `PeriodStart(t)` is 00:00 UTC on the first of `t`'s month in UTC, whatever `t`'s location. A Sydney user at 09:00 on 1 October local time is still in September's budget; the cockpit's *"until 1 Oct"* sentence (task 22) reads `PeriodEnd`, and says UTC where it matters.

- [ ] **Step 1: Preflight**

```bash
cd control-plane
test -d internal/budget && echo "STOP: internal/budget exists" || echo "ok: no budget package yet"   # expect ok
grep -c '^go 1.23$' go.mod                                                                          # expect 1
```

A hit on the first line means another agent has been here; stop and report rather than overwriting. Nothing else is checked, because this package imports nothing of ours.

- [ ] **Step 2: Write the failing tests**

Twelve tests. Each names, in its comment, the mutant it kills: the plausible wrong implementation that would otherwise pass. The ones that matter most are the zero cases, because every one of them is a way for "no number" to become "no limit".

Create `control-plane/internal/budget/budget_test.go`:

```go
package budget

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

// full is a valid Limits; tests perturb one field at a time.
func full() Limits {
	return Limits{PersonUSD: 200, ProjectUSD: 1000, TeamUSD: 2000, SessionCapUSD: 25, Period: Monthly}
}

func mustStatic(t *testing.T) Static {
	t.Helper()
	s, err := NewStatic(full())
	if err != nil {
		t.Fatalf("NewStatic(full()): %v", err)
	}
	return s
}

type fakePolicy struct {
	l             Limits
	err           error
	team, project string
}

func (f *fakePolicy) Limits(_ context.Context, teamID, projectID string) (Limits, error) {
	f.team, f.project = teamID, projectID
	return f.l, f.err
}

// Mutant: Slice reports one fixed dimension whenever anything is exhausted.
func TestSliceNamesTheExhaustedDimension(t *testing.T) {
	cases := []struct {
		name  string
		spent Spent
		want  Dimension
	}{
		{"person", Spent{PersonUSD: 200}, Person},
		{"project", Spent{ProjectUSD: 1000.01}, Project},
		{"team", Spent{TeamUSD: 2500}, Team},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, dim, ok := Slice(full(), c.spent)
			if ok || dim != c.want || got != 0 {
				t.Fatalf("Slice = (%v, %q, %v), want (0, %q, false)", got, dim, ok, c.want)
			}
		})
	}
}

// Mutant: max instead of min, or the session cap ignored.
func TestSliceIsTheLeastRemainder(t *testing.T) {
	cases := []struct {
		name  string
		spent Spent
		want  float64
	}{
		{"cap is least", Spent{}, 25},
		{"person is least", Spent{PersonUSD: 190}, 10},
		{"project is least", Spent{ProjectUSD: 996}, 4},
		{"team is least", Spent{TeamUSD: 1999.5}, 0.5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, dim, ok := Slice(full(), c.spent)
			if !ok || dim != None || got != c.want {
				t.Fatalf("Slice = (%v, %q, %v), want (%v, \"\", true)", got, dim, ok, c.want)
			}
		})
	}
}

// Mutant: an unset (zero) or unbounded (infinite) limit read as "no limit".
func TestSliceZeroOrInfiniteLimitIsExhausted(t *testing.T) {
	for name, set := range map[string]func(*Limits){
		"team zero":       func(l *Limits) { l.TeamUSD = 0 },
		"person infinite": func(l *Limits) { l.PersonUSD = math.Inf(1) },
		"project NaN":     func(l *Limits) { l.ProjectUSD = math.NaN() },
	} {
		t.Run(name, func(t *testing.T) {
			l := full()
			set(&l)
			if got, dim, ok := Slice(l, Spent{}); ok || got != 0 || dim == None {
				t.Fatalf("Slice = (%v, %q, %v), want an exhausted dimension", got, dim, ok)
			}
		})
	}
	l := full()
	l.SessionCapUSD = 0
	if got, dim, ok := Slice(l, Spent{}); ok || got != 0 || dim != None {
		t.Fatalf("zero cap: Slice = (%v, %q, %v), want (0, \"\", false)", got, dim, ok)
	}
}

// Mutant: a negative spend subtracted (raising the remainder past the limit),
// or a NaN spend treated as zero (spending we could not read, counted as none).
func TestSliceSpendBelowZeroIsZeroAndNaNIsExhausted(t *testing.T) {
	l := full()
	l.SessionCapUSD = 1e9
	if got, _, ok := Slice(l, Spent{PersonUSD: -50}); !ok || got != 200 {
		t.Fatalf("negative spend: got (%v, %v), want (200, true)", got, ok)
	}
	if _, dim, ok := Slice(full(), Spent{TeamUSD: math.NaN()}); ok || dim != Team {
		t.Fatalf("NaN spend: got (%q, %v), want (team, false)", dim, ok)
	}
}

// Mutant: dimensions checked in map order, so a tie names a different one per run.
func TestSliceReportsTiesInFixedOrder(t *testing.T) {
	for i := 0; i < 50; i++ {
		if _, dim, _ := Slice(full(), Spent{PersonUSD: 999, ProjectUSD: 9999, TeamUSD: 9999}); dim != Person {
			t.Fatalf("run %d: tie named %q, want person", i, dim)
		}
		if _, dim, _ := Slice(full(), Spent{ProjectUSD: 9999, TeamUSD: 9999}); dim != Project {
			t.Fatalf("run %d: tie named %q, want project", i, dim)
		}
	}
}

// Mutant: budgets off by default, a constructor that accepts an unset field.
func TestNewStaticRefusesAnyUnsetField(t *testing.T) {
	for name, set := range map[string]func(*Limits){
		"person zero":      func(l *Limits) { l.PersonUSD = 0 },
		"project negative": func(l *Limits) { l.ProjectUSD = -1 },
		"team NaN":         func(l *Limits) { l.TeamUSD = math.NaN() },
		"cap infinite":     func(l *Limits) { l.SessionCapUSD = math.Inf(1) },
		"period weekly":    func(l *Limits) { l.Period = "weekly" },
		"period empty":     func(l *Limits) { l.Period = "" },
	} {
		t.Run(name, func(t *testing.T) {
			l := full()
			set(&l)
			if _, err := NewStatic(l); !errors.Is(err, ErrInvalidLimits) {
				t.Fatalf("NewStatic: err = %v, want ErrInvalidLimits", err)
			}
		})
	}
	if _, err := NewStatic(Proposed); err != nil {
		t.Fatalf("NewStatic(Proposed): %v", err)
	}
}

// Mutant: the zero Static answers zero limits instead of refusing.
func TestZeroStaticRefuses(t *testing.T) {
	var s Static
	if _, err := s.Limits(context.Background(), "team-1", "proj-1"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("zero Static: err = %v, want ErrNotConfigured", err)
	}
}

// Mutant: "an unset quota is the org pool by another name" (05-design.md:233-234),
// a zero, negative or infinite field from the primary passed through as-is.
func TestLayeredUnsetFromPrimaryIsTheDefault(t *testing.T) {
	p := &fakePolicy{l: Limits{PersonUSD: -5, ProjectUSD: 300, TeamUSD: math.Inf(1), Period: "weekly"}}
	got, err := Layered{Primary: p, Fallback: mustStatic(t)}.Limits(context.Background(), "team-1", "proj-1")
	want := full()
	want.ProjectUSD = 300
	if err != nil || got != want {
		t.Fatalf("Layered = (%+v, %v), want (%+v, nil)", got, err, want)
	}
	if p.team != "team-1" || p.project != "proj-1" {
		t.Fatalf("primary asked for (%q, %q), want (team-1, proj-1)", p.team, p.project)
	}
}

// Mutant: the default served when the primary errors, which loosens a tighter
// Hub budget for as long as the Hub is down (P6-D5, OD-24).
func TestLayeredPrimaryErrorFailsClosed(t *testing.T) {
	p := &fakePolicy{err: errors.New("hub: 503")}
	got, err := Layered{Primary: p, Fallback: mustStatic(t)}.Limits(context.Background(), "team-1", "proj-1")
	if !errors.Is(err, ErrPolicyUnavailable) || got != (Limits{}) {
		t.Fatalf("Layered = (%+v, %v), want (zero, ErrPolicyUnavailable)", got, err)
	}
}

// Mutant: a nil primary dereferenced, or an unconfigured fallback skipped
// because the primary happened to set every field.
func TestLayeredNeedsItsFallbackAndNotItsPrimary(t *testing.T) {
	got, err := Layered{Fallback: mustStatic(t)}.Limits(context.Background(), "t", "p")
	if err != nil || got != full() {
		t.Fatalf("no primary: (%+v, %v), want the default", got, err)
	}
	_, err = Layered{Primary: &fakePolicy{l: full()}}.Limits(context.Background(), "t", "p")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("zero fallback: err = %v, want ErrNotConfigured", err)
	}
}

// Mutant: the period computed in the value's own location, which moves the
// month boundary by ten hours for a Sydney clock.
func TestPeriodIsTheUTCCalendarMonth(t *testing.T) {
	syd := time.FixedZone("AEST", 10*3600)
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, syd) // 2026-09-30T23:00:00Z
	if got, want := PeriodStart(at), time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("PeriodStart = %v, want %v in UTC", got, want)
	}
	if got, want := PeriodEnd(at), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("PeriodEnd = %v, want %v", got, want)
	}
	dec := time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	if got, want := PeriodEnd(dec), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("PeriodEnd(December) = %v, want %v", got, want)
	}
}

// Mutant: a dimension renamed here while task 03's UsageRow.Scope keeps the old string.
func TestDimensionStringsMatchTheWireScopes(t *testing.T) {
	for d, want := range map[Dimension]string{Person: "person", Project: "project", Team: "team", None: ""} {
		if string(d) != want {
			t.Fatalf("Dimension %q, want %q", d, want)
		}
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

```bash
cd control-plane && go test ./internal/budget/
```

Expected: a build failure, `undefined: Limits` (and the rest of the package's names). That is the failing state; nothing here can fail for a second reason.

- [ ] **Step 4: Write the package**

Create `control-plane/internal/budget/budget.go`:

```go
// Package budget is the gateway's spending rule: three dimensions, each on by
// default (OD-24), and the per-key slice keysmith writes into a key's
// max_budget (P6-D8). It is pure: no I/O and no clock.
//
// No number here is ever "unlimited". A zero, negative, NaN or infinite limit
// is an exhausted dimension; an unset field from the primary Policy is the
// configured default; a default that is unset is refused at construction.
package budget

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

// Dimension names what a budget is attached to. The strings are task 03's
// wire.UsageRow.Scope values; a golden test pins them.
type Dimension string

const (
	None    Dimension = ""
	Person  Dimension = "person"
	Project Dimension = "project"
	Team    Dimension = "team"
)

// Period is how often a budget resets. Only the UTC calendar month exists,
// because task 09's rollup sums calendar months and nothing else.
type Period string

const Monthly Period = "monthly"

// Limits are the budgets that bind one project in one team, in US dollars as
// LiteLLM records spend.
type Limits struct {
	PersonUSD     float64 `json:"personUsd"`
	ProjectUSD    float64 `json:"projectUsd"`
	TeamUSD       float64 `json:"teamUsd"`
	SessionCapUSD float64 `json:"sessionCapUsd"`
	Period        Period  `json:"period"`
}

// Spent is what has been spent this period, per dimension. Task 09's Reader
// fills it.
type Spent struct {
	PersonUSD  float64 `json:"personUsd"`
	ProjectUSD float64 `json:"projectUsd"`
	TeamUSD    float64 `json:"teamUsd"`
}

// Policy answers the limits for a project in a team. The Hub-backed Policy is
// ask H-2, seam S6.2, and is not built in phase 6.
type Policy interface {
	Limits(ctx context.Context, teamID, projectID string) (Limits, error)
}

var (
	ErrInvalidLimits     = errors.New("budget: a limit is zero, negative or not finite, or the period is not monthly")
	ErrNotConfigured     = errors.New("budget: no default limits configured")
	ErrPolicyUnavailable = errors.New("budget: the budget policy could not be read")
)

// Proposed is the default until the owner answers Q2: US$200 a person,
// US$1,000 a project, US$2,000 a team a month, and US$25 a session.
// PROPOSED, not decided.
var Proposed = Limits{PersonUSD: 200, ProjectUSD: 1000, TeamUSD: 2000, SessionCapUSD: 25, Period: Monthly}

// usable reports whether v can be a limit: finite and above zero. NaN fails
// the comparison, so it is unusable too.
func usable(v float64) bool { return v > 0 && !math.IsInf(v, 1) }

// Validate is nil only when every field is usable and the period is monthly.
func (l Limits) Validate() error {
	fields := [...]struct {
		name string
		v    float64
	}{{"person", l.PersonUSD}, {"project", l.ProjectUSD}, {"team", l.TeamUSD}, {"session cap", l.SessionCapUSD}}
	for _, f := range fields {
		if !usable(f.v) {
			return fmt.Errorf("%w: %s is %v", ErrInvalidLimits, f.name, f.v)
		}
	}
	if l.Period != Monthly {
		return fmt.Errorf("%w: period is %q", ErrInvalidLimits, l.Period)
	}
	return nil
}

// Static answers the same limits for every project. Its field is unexported,
// so NewStatic is the only way to fill it and the zero Static refuses.
type Static struct{ def *Limits }

// NewStatic refuses any unset field: budgets are on by default (OD-24).
func NewStatic(def Limits) (Static, error) {
	if err := def.Validate(); err != nil {
		return Static{}, err
	}
	return Static{def: &def}, nil
}

func (s Static) Limits(context.Context, string, string) (Limits, error) {
	if s.def == nil {
		return Limits{}, ErrNotConfigured
	}
	return *s.def, nil
}

// Layered asks Primary and fills each unset field from Fallback. An error
// from Primary fails closed: "we could not tell" never becomes the default,
// which may be looser than what an admin set.
type Layered struct {
	Primary  Policy // nil: Fallback alone
	Fallback Static
}

func (l Layered) Limits(ctx context.Context, teamID, projectID string) (Limits, error) {
	def, err := l.Fallback.Limits(ctx, teamID, projectID)
	if err != nil {
		return Limits{}, err
	}
	if l.Primary == nil {
		return def, nil
	}
	p, err := l.Primary.Limits(ctx, teamID, projectID)
	if err != nil {
		return Limits{}, fmt.Errorf("%w: %w", ErrPolicyUnavailable, err)
	}
	return Limits{
		PersonUSD:     pick(p.PersonUSD, def.PersonUSD),
		ProjectUSD:    pick(p.ProjectUSD, def.ProjectUSD),
		TeamUSD:       pick(p.TeamUSD, def.TeamUSD),
		SessionCapUSD: pick(p.SessionCapUSD, def.SessionCapUSD),
		Period:        Monthly,
	}, nil
}

func pick(primary, def float64) float64 {
	if usable(primary) {
		return primary
	}
	return def
}

// Slice is the most one new key may spend: the least of each dimension's
// remainder and the session cap. When a dimension has nothing left, ok is
// false and exhausted names the first such dimension in the fixed order
// person, project, team. An unusable session cap is (0, None, false): the
// caller treats that as a misconfiguration, never as a key without a cap.
func Slice(l Limits, s Spent) (maxUSD float64, exhausted Dimension, ok bool) {
	dims := [...]struct {
		d    Dimension
		left float64
	}{
		{Person, remaining(l.PersonUSD, s.PersonUSD)},
		{Project, remaining(l.ProjectUSD, s.ProjectUSD)},
		{Team, remaining(l.TeamUSD, s.TeamUSD)},
	}
	for _, x := range dims {
		if !(x.left > 0) {
			return 0, x.d, false
		}
	}
	if !usable(l.SessionCapUSD) {
		return 0, None, false
	}
	maxUSD = l.SessionCapUSD
	for _, x := range dims {
		maxUSD = min(maxUSD, x.left)
	}
	return maxUSD, None, true
}

// remaining is what is left of one dimension. An unusable limit leaves
// nothing; a NaN spend (spend we could not read) leaves nothing; a negative
// spend counts as zero.
func remaining(limit, spent float64) float64 {
	if !usable(limit) || math.IsNaN(spent) {
		return 0
	}
	return limit - max(spent, 0)
}

// PeriodStart is 00:00 UTC on the first of t's month in UTC.
func PeriodStart(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// PeriodEnd is the next period's start.
func PeriodEnd(t time.Time) time.Time { return PeriodStart(t).AddDate(0, 1, 0) }
```

- [ ] **Step 5: Run the tests to verify they pass**

```bash
cd control-plane && go test ./internal/budget/ -count=1 -v 2>&1 | grep -c '^--- PASS'
```

Expected: `12`. Subtests are indented and not counted.

- [ ] **Step 6: Kill three mutants by hand**

Each edit below is made, the package run, and the edit reverted. Each must turn exactly the named test red.

| Edit to `budget.go` | Test that must fail |
|---|---|
| `maxUSD = min(maxUSD, x.left)` → `maxUSD = max(maxUSD, x.left)` | `TestSliceIsTheLeastRemainder` |
| in `Layered.Limits`, `return Limits{}, fmt.Errorf(...)` → `return def, nil` | `TestLayeredPrimaryErrorFailsClosed` |
| `func usable(v float64) bool { return v > 0 && ... }` → `return v >= 0 && ...` | `TestNewStaticRefusesAnyUnsetField`, `TestSliceZeroOrInfiniteLimitIsExhausted` |

- [ ] **Step 7: Vet, format, and confirm the package is a leaf**

```bash
cd control-plane
go vet ./internal/budget/ && gofmt -l internal/budget/          # expect no output
go list -deps ./internal/budget/ | grep -c 'kpmg-au-emu'          # expect 1 (the package itself)
```

The last line is the promise that tasks 07 and 09 can both import this package without a cycle.

- [ ] **Step 8: Commit**

```bash
git add control-plane/internal/budget/budget.go control-plane/internal/budget/budget_test.go
git commit -F <message file> -- control-plane/internal/budget/budget.go control-plane/internal/budget/budget_test.go
```

Message: `feat(budget): three dimensions on by default and the per-key slice (phase 6 task 06)`, a body naming decisions (a) to (e), and the attribution line.

**Acceptance:** step 5 prints `12`; step 6's three edits each fail the named test; step 7 prints nothing and then `1`.

**What this task does not claim:**

- **A hard ceiling.** Each key is capped at its slice, and every live key was minted from a `Spent` up to 60 seconds old (task 09's sweep). So for any dimension, spend in a month can exceed its limit by at most **K × `SessionCapUSD`**, where K is the number of keys drawing on that dimension that are live at once. K counts a refreshing session twice for up to `Grace` (5 minutes, task 07), because the old generation and the new one are both live. With the proposed defaults, ten concurrent sessions on one project can overshoot a nearly exhausted project budget by up to US$250. LiteLLM's native user and team budgets are a second, real-time bound on the person and the project if they work in the pinned image (**UNVERIFIED** until task 18's P03 and P04); the team dimension has no second bound.
- **That `SessionCapUSD` caps a session's lifetime.** It caps **each key**. A session refreshes every 45 minutes (`REFRESH_AT`), and each refresh mints a fresh slice, so an eight-hour session can spend up to about eleven caps, bounded by the three dimensions. The gap list below names the change that would make it a lifetime cap.
- **That LiteLLM's resets line up with the UTC month.** keysmith computes every remainder from task 09's calendar-month rollup. LiteLLM's own `budget_duration` resets on its own schedule (relative to creation, from training knowledge, **UNVERIFIED**), so its native budgets are an independent bound whose window may differ from ours.
- **That the proposed figures are right.** `Proposed` is owner question Q2.
- **A Hub-backed `Policy`.** That is ask H-2, seam S6.2; `Layered.Primary` is where it plugs in. Note for whoever builds it (review M-2): `Limits` takes **no person**, so the person figure is one per-team limit applied to each member and no id crosses this boundary. A per-person figure would change this signature, and would then have to name a person the way keysmith does — `<tid>:<oid>` (`wire.Owner.UserID()`, task 03 `:384`), never the Hub's `User.id` (`u-…`, `hub-contract.md:618`). The Hub holds that mapping already (`hub.users.entra_oid` with `orgs.entra_tenant_id`, `hub-contract.md:1953-1973`); H-2 in the skeleton carries the ask.

**Gaps against the skeleton, for the orchestrator:**

1. Task 03's problem codes have none for "the budget policy could not be read". `Layered` fails closed with `ErrPolicyUnavailable`, and task 07 needs a 503 code for it; this part proposes `budget_unknown` (503) be added to task 03's list. Until a Hub primary exists (S6.2), the path is unreachable, because a constructed `Static` never errors.
2. The skeleton's `Static{Default Limits}` has an exported field, which lets the zero-refusing constructor be bypassed. This part unexports it (decision (a)).
3. "Per-session cap" is per key (above). A lifetime cap needs `Spent.SessionUSD` and a session argument on task 07's `SpentReader`, which task 09 can serve from `key_spend` summed by session. Owner call; not done here.

