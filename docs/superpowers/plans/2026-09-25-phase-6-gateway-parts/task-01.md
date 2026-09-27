### Task 1: Data class, route and the ratchet

Slice A, 2 focused hours, no dependencies. Seams S6.1, S6.7, S6.8.

This task fixes the one decision every later task makes: **for this project, may a request leave Australia?** It is pure Go over the standard library, with no I/O, so every other part can import it without a cycle and without a fake. Everything the gateway does with a route (task 07 at mint, task 10 on every request, task 13 when it reads the Hub, task 21 when a person presses *Keep inference in Australia*) goes through the four functions here and never re-derives the rule.

Three rules come from the owner and are enforced here, not described:

1. **The data class is set at creation, can be updated, and drives AU vs Global** (OD-24, `docs/superpowers/planning/2026-09-23-owner-decisions.md:152`). The route a project runs on is the **tighter** of the class's floor and the route someone chose. A class with `au-only` residency cannot run on Global, whatever the stored route says.
2. **Tighten, never loosen** (OD-24). Any member may raise a project to AU. Lowering needs an org compliance role (ask H-4), and nobody may lower a project below its class's floor. One exception, from the source design and put to the owner as Q5: the owner of a **solo** project may lower its route to the floor (decision (e)).
3. **Fail closed** (P6-D5, global constraint 6). The zero `Class` is invalid, never "public". An unknown level, residency or route is an error, never Global.

**The level list is a proposal** (owner question Q1). `public`, `internal`, `confidential`, `restricted` are written as **one table** (`levels`), and nothing outside this file switches on a level's name: the only thing a level decides today is its rank, used by `CanReclassify`. Which levels force AU is also Q1's; until the owner answers, only `Residency` drives the floor. When KPMG's scheme arrives, the table changes and no other file does.

Five decisions the skeleton leaves open, made here so no later part re-derives them:

- **(a) An unknown route is tighter than every known one inside `Tighter`.** `Tighter` returns a `Route`, not an error, because task 10 calls it on two values it has already validated. If a caller passes an unparsed value anyway, `Tighter` returns that value rather than `global`, so it reaches the next check (the catalogue's `Resolve`, task 02) and is refused there. Returning `global` would be the one path in the program that turns "unknown" into Global.
- **(b) `Decide` refuses an empty chosen route.** The Hub always stores one (ask H-1 sets it to `Floor` at creation) and local mode's `projects.json` always carries one (task 13). A record with none is broken, and a broken record is refused (P6-D5), not defaulted.
- **(c) A class change has its own check, `CanReclassify`.** The skeleton's `CanChange` covers the route only, but task 21 and ask H-1 also accept a `dataClass` change, and moving a project from `au-only` to `any` lowers its floor. `CanReclassify` applies the same ratchet: tightening (a higher level, or `any` → `au-only`) needs a member; loosening either needs compliance. This is additive; `CanChange` keeps the skeleton's signature. It is listed as a skeleton gap in the return.
- **(d) The Hub wire shape follows the Hub's null rule.** The Hub contract says *"Fields are always present; a field that has no value is `null` … never omitted"* (`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md:614-615`), and its principal is `User` = `{id, displayName, email}` (`:618-619`). So `routeChangedBy` is a `*User` and `routeChangedAt` a `*time.Time`, both without `omitempty`, and both `null` until the first change.
- **(e) A solo project's owner may lower the route to the floor (review I-2, owner question Q5).** `05-design.md` §2.5 (`:219-222`): *"A solo project keeps the user's choice. On a team project the route is project-wide … only an org-level compliance role may lower it."* A universal ratchet would leave a solo developer who pressed *Keep inference in Australia* unable to go back, since local mode has no compliance role (ask H-4). So `Actor` gains `Solo bool`, meaning *this member owns a solo project*. In `CanChange`, `Solo` passes the role check on a lowering, but only after the floor check, so an `au-only` project stays on AU for everyone; `Solo` without `Member` is `ErrNotMember`. `CanReclassify` ignores `Solo`: the class is a statement about the data, and loosening it lowers the floor, so it still needs compliance. Who sets `Solo` is the caller's: task 21 sets it for every local project (all of them are solo, `05-design.md` §2.3), and ask H-1's Hub never sets it (a Hub project is a team project). If the owner answers Q5 with the universal ratchet, task 21's constant changes and this file does not.

**`DisallowUnknownFields`, and where it applies.** The skeleton asks for strict decoding of `ProjectInferenceFields`. That is right for the object on its own (a local `projects.json` entry, task 13's `File`) and wrong for the Hub's whole project record (route 15, `hub-contract.md:577`), which carries `id`, `teamId`, `name`, `remotes` and more (`:638-639`). So strictness lives at two levels: `Class.UnmarshalJSON` refuses an unknown key **inside** the `dataClass` object and validates both values; `DecodeFields` refuses an unknown key in a standalone fields object; and `FieldsFromRecord` reads the four fields out of a larger record, ignoring the record's other keys but still refusing an unknown or missing class or route. Task 13's `Hub` source uses `FieldsFromRecord`; its `File` source uses `DecodeFields`.

**Files:**
- `control-plane/internal/dataclass/dataclass.go` — `Level`, `Residency`, `Class`, `Route`, `Actor`, `User`, `ProjectInferenceFields`; `Validate`, `Floor`, `Decide`, `Tighter`, `CanChange`, `CanReclassify`, `ParseRoute`, `Levels`, `DecodeFields`, `FieldsFromRecord` (new)
- `control-plane/internal/dataclass/dataclass_test.go` — the table tests below, each naming the mutant it kills (new)

**Interfaces:**
- Produces:
  - `dataclass.Level` with `LevelPublic`, `LevelInternal`, `LevelConfidential`, `LevelRestricted`; `dataclass.Levels() []Level` (a copy of the table, in rank order, for task 22's class picker and task 21's validation messages)
  - `dataclass.Residency` with `ResidencyAny` (`"any"`), `ResidencyAUOnly` (`"au-only"`)
  - `dataclass.Class{Level; Residency}`, JSON `{"level": …, "residency": …}`; `(Class) Validate() error`; `(*Class) UnmarshalJSON`
  - `dataclass.Route` with `RouteGlobal` (`"global"`), `RouteAU` (`"au"`); `ParseRoute(string) (Route, error)`; `(Route) Valid() bool`; `(*Route) UnmarshalJSON`
  - `dataclass.Floor(Class) Route`, `dataclass.Decide(Class, Route) (Route, error)`, `dataclass.Tighter(a, b Route) Route`
  - `dataclass.Actor{Member, Compliance, Solo bool}` (`Solo`, decision (e)); `dataclass.CanChange(c Class, from, to Route, a Actor) error`; `dataclass.CanReclassify(from, to Class, a Actor) error`
  - sentinels `ErrUnknownClass`, `ErrUnknownRoute`, `ErrNeedsCompliance`, `ErrBelowFloor`, `ErrNotMember`
  - `dataclass.User{ID, DisplayName, Email}` (the Hub's `User`), `dataclass.ProjectInferenceFields{DataClass; InferenceRoute; RouteChangedBy *User; RouteChangedAt *time.Time}`; `DecodeFields([]byte)`, `FieldsFromRecord([]byte)`
- Consumes: the standard library only (`encoding/json`, `errors`, `fmt`, `bytes`, `time`).
- Required by: task 02 (`Route` in `Resolve`), task 03 (`Route` and `Class` in the wire shapes), task 06, task 07 (`Decide` at mint), task 10 (`Tighter`), task 13 (`Decide`, `DecodeFields`, `FieldsFromRecord`), task 21 (`CanChange`, `CanReclassify`), task 22 (mirrors the JSON), and asks H-1 and P7-1, which are written against this JSON exactly.

- [ ] **Step 1: Write the failing tests**

Every test names, in its comment, the mutant it kills: the one plausible wrong implementation that would pass every other test. The file is `package dataclass` (internal), so the compiler's first complaint is a bare `undefined: Class`. `Floor` of an invalid class is `au`, not `global`: the skeleton's wording is "`au-only` → `au`, else `global`", and "else" would include a residency nobody could parse, which is the fail-open path P6-D5 forbids. Only exactly `any` floors at Global.

Create `control-plane/internal/dataclass/dataclass_test.go`:

```go
package dataclass

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

var (
	auOnly     = Class{Level: LevelConfidential, Residency: ResidencyAUOnly}
	anywhere   = Class{Level: LevelInternal, Residency: ResidencyAny}
	member     = Actor{Member: true}
	compliance = Actor{Member: true, Compliance: true}
	solo       = Actor{Member: true, Solo: true} // decision (e)
)

// Kills: Decide returning `chosen` unchanged, which lets an au-only project run on Global.
func TestDecideTakesTheTighterOfFloorAndChoice(t *testing.T) {
	cases := []struct {
		name   string
		c      Class
		chosen Route
		want   Route
	}{
		{"au-only floor beats a global choice", auOnly, RouteGlobal, RouteAU},
		{"au-only with au", auOnly, RouteAU, RouteAU},
		{"any with global stays global", anywhere, RouteGlobal, RouteGlobal},
		{"any raised to au", anywhere, RouteAU, RouteAU},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decide(tc.c, tc.chosen)
			if err != nil {
				t.Fatalf("Decide: %v", err)
			}
			if got != tc.want {
				t.Fatalf("Decide(%+v, %q) = %q, want %q", tc.c, tc.chosen, got, tc.want)
			}
		})
	}
}

// Kills: a `default: return RouteGlobal` for anything unparsed (P6-D5).
func TestDecideRefusesWhatItCannotParse(t *testing.T) {
	cases := []struct {
		name   string
		c      Class
		chosen Route
		want   error
	}{
		{"unknown level", Class{Level: "secret", Residency: ResidencyAny}, RouteGlobal, ErrUnknownClass},
		{"unknown residency", Class{Level: LevelPublic, Residency: "eu-only"}, RouteGlobal, ErrUnknownClass},
		{"zero class", Class{}, RouteGlobal, ErrUnknownClass},
		{"unknown route", anywhere, "apac", ErrUnknownRoute},
		{"empty route", anywhere, "", ErrUnknownRoute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decide(tc.c, tc.chosen)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Decide error = %v, want %v", err, tc.want)
			}
			if got != "" {
				t.Fatalf("Decide returned route %q beside an error; a caller that drops the error must hold no route", got)
			}
		})
	}
}
```

Continue the same file with the zero value, the floor and `Tighter`:

```go
// Kills: treating the empty class as "public", the quiet default a missing field produces.
func TestTheZeroClassIsInvalid(t *testing.T) {
	for _, c := range []Class{{}, {Level: LevelPublic}, {Residency: ResidencyAny}} {
		if err := c.Validate(); !errors.Is(err, ErrUnknownClass) {
			t.Fatalf("Validate(%+v) = %v, want ErrUnknownClass", c, err)
		}
	}
	for _, l := range Levels() {
		if err := (Class{Level: l, Residency: ResidencyAny}).Validate(); err != nil {
			t.Fatalf("Validate with level %q: %v", l, err)
		}
	}
}

// Kills: `if au-only { au } else { global }`, which floors an unparsed residency at Global.
func TestFloor(t *testing.T) {
	cases := map[Residency]Route{ResidencyAny: RouteGlobal, ResidencyAUOnly: RouteAU, "": RouteAU, "eu-only": RouteAU}
	for res, want := range cases {
		if got := Floor(Class{Level: LevelInternal, Residency: res}); got != want {
			t.Fatalf("Floor(residency %q) = %q, want %q", res, got, want)
		}
	}
}

// Kills: Tighter returning its first argument, or the looser one, or "global" for an unknown.
func TestTighter(t *testing.T) {
	cases := []struct{ a, b, want Route }{
		{RouteGlobal, RouteGlobal, RouteGlobal},
		{RouteGlobal, RouteAU, RouteAU},
		{RouteAU, RouteGlobal, RouteAU},
		{RouteAU, RouteAU, RouteAU},
		{RouteGlobal, "apac", "apac"}, // decision (a): the unknown propagates to the next check
		{"apac", RouteAU, "apac"},
	}
	for _, tc := range cases {
		if got := Tighter(tc.a, tc.b); got != tc.want {
			t.Fatalf("Tighter(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestParseRoute(t *testing.T) {
	for _, s := range []string{"global", "au"} {
		if r, err := ParseRoute(s); err != nil || string(r) != s {
			t.Fatalf("ParseRoute(%q) = %q, %v", s, r, err)
		}
	}
	for _, s := range []string{"", "Global", "AU", "apac", "bedrock"} {
		if _, err := ParseRoute(s); !errors.Is(err, ErrUnknownRoute) {
			t.Fatalf("ParseRoute(%q) error = %v, want ErrUnknownRoute", s, err)
		}
	}
}
```

Continue the same file with the ratchet. `CanChange` checks the floor **before** the role, so a member trying to lower an `au-only` project is told the truth: nobody can, not "ask compliance".

```go
func TestCanChangeIsTheRatchet(t *testing.T) {
	cases := []struct {
		name     string
		c        Class
		from, to Route
		a        Actor
		want     error
	}{
		// Raising needs only Member. Kills a ratchet that demands compliance for every change.
		{"member raises to au", anywhere, RouteGlobal, RouteAU, member, nil},
		{"member keeps the route", anywhere, RouteAU, RouteAU, member, nil},
		// Kills a dropped role check: any member could quietly send a project's traffic abroad.
		{"member lowers", anywhere, RouteAU, RouteGlobal, member, ErrNeedsCompliance},
		{"compliance lowers an any project", anywhere, RouteAU, RouteGlobal, compliance, nil},
		// Kills a missing floor check: even compliance cannot run an au-only project on Global.
		{"compliance below the floor", auOnly, RouteAU, RouteGlobal, compliance, ErrBelowFloor},
		{"member below the floor is told the floor", auOnly, RouteAU, RouteGlobal, member, ErrBelowFloor},
		// Decision (e). Kills a Solo that is ignored: a solo developer could never undo a raise.
		{"solo owner lowers an any project", anywhere, RouteAU, RouteGlobal, solo, nil},
		// Kills a Solo checked before the floor: an au-only project would reach Global.
		{"solo owner below the floor", auOnly, RouteAU, RouteGlobal, solo, ErrBelowFloor},
		// Kills a membership check that counts Solo as membership.
		{"solo without membership", anywhere, RouteAU, RouteGlobal, Actor{Solo: true}, ErrNotMember},
		// Kills an actor check that only runs on the lowering branch.
		{"nobody raises", anywhere, RouteGlobal, RouteAU, Actor{}, ErrNotMember},
		{"unknown target", anywhere, RouteGlobal, "apac", member, ErrUnknownRoute},
		{"unknown current", anywhere, "", RouteAU, member, ErrUnknownRoute},
		{"unknown class", Class{}, RouteGlobal, RouteAU, member, ErrUnknownClass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CanChange(tc.c, tc.from, tc.to, tc.a)
			if tc.want == nil && err != nil {
				t.Fatalf("CanChange: %v, want nil", err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("CanChange error = %v, want %v", err, tc.want)
			}
		})
	}
}

// Decision (c). Kills a reclassify check that looks at one axis only.
func TestCanReclassifyTightensFreelyAndLoosensOnlyWithCompliance(t *testing.T) {
	confAny := Class{Level: LevelConfidential, Residency: ResidencyAny}
	intAU := Class{Level: LevelInternal, Residency: ResidencyAUOnly}
	cases := []struct {
		name     string
		from, to Class
		a        Actor
		want     error
	}{
		{"member raises the level", anywhere, confAny, member, nil},
		{"member tightens residency", anywhere, intAU, member, nil},
		{"member lowers the level", confAny, anywhere, member, ErrNeedsCompliance},
		{"member loosens residency", intAU, anywhere, member, ErrNeedsCompliance},
		{"level up, residency down", intAU, confAny, member, ErrNeedsCompliance},
		{"compliance loosens residency", intAU, anywhere, compliance, nil},
		// Decision (e). Kills Solo read as Compliance here: a solo owner would lower the floor itself.
		{"solo owner loosens residency", intAU, anywhere, solo, ErrNeedsCompliance},
		{"solo owner lowers the level", confAny, anywhere, solo, ErrNeedsCompliance},
		{"solo owner tightens residency", anywhere, intAU, solo, nil},
		{"to the zero class", anywhere, Class{}, compliance, ErrUnknownClass},
		{"nobody", anywhere, confAny, Actor{}, ErrNotMember},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CanReclassify(tc.from, tc.to, tc.a)
			if (tc.want == nil) != (err == nil) || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("CanReclassify error = %v, want %v", err, tc.want)
			}
		})
	}
}
```

Continue the same file with the Hub wire shape. The goldens compare **bytes**, because ask H-1's server and task 22's TypeScript are written against these keys and a Go round-trip passes even when a key is renamed.

```go
// Kills: a renamed key, or `omitempty` on the change fields (the Hub never omits, it sends null).
func TestProjectInferenceFieldsWireShape(t *testing.T) {
	at := time.Date(2026, 9, 25, 4, 5, 6, 0, time.UTC)
	changed := ProjectInferenceFields{
		DataClass: auOnly, InferenceRoute: RouteAU, RouteChangedAt: &at,
		RouteChangedBy: &User{ID: "u-3f2a91c07b1e4d2a", DisplayName: "A Person", Email: "a.person@example.com"},
	}
	fresh := ProjectInferenceFields{DataClass: anywhere, InferenceRoute: RouteGlobal}
	golden := map[string]struct {
		v    ProjectInferenceFields
		want string
	}{
		"changed": {changed, `{"dataClass":{"level":"confidential","residency":"au-only"},"inferenceRoute":"au",` +
			`"routeChangedBy":{"id":"u-3f2a91c07b1e4d2a","displayName":"A Person","email":"a.person@example.com"},` +
			`"routeChangedAt":"2026-09-25T04:05:06Z"}`},
		"fresh": {fresh, `{"dataClass":{"level":"internal","residency":"any"},"inferenceRoute":"global",` +
			`"routeChangedBy":null,"routeChangedAt":null}`},
	}
	for name, g := range golden {
		got, err := json.Marshal(g.v)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		if string(got) != g.want {
			t.Fatalf("%s:\n got %s\nwant %s", name, got, g.want)
		}
		back, err := DecodeFields(got)
		if err != nil {
			t.Fatalf("%s: DecodeFields of our own bytes: %v", name, err)
		}
		if back.InferenceRoute != g.v.InferenceRoute || back.DataClass != g.v.DataClass {
			t.Fatalf("%s: round trip = %+v", name, back)
		}
	}
}

// Kills: a lenient decoder, and a missing field decoded as the zero (public, global) value.
func TestDecodeFieldsIsStrict(t *testing.T) {
	const ok = `{"dataClass":{"level":"internal","residency":"any"},"inferenceRoute":"global"`
	cases := []struct {
		name, body string
		want       error // nil means "any error"
	}{
		{"unknown top-level key", ok + `,"extra":1}`, nil},
		{"unknown key inside the class", `{"dataClass":{"level":"internal","residency":"any","colour":"red"},"inferenceRoute":"global"}`, ErrUnknownClass},
		{"unknown level", `{"dataClass":{"level":"secret","residency":"any"},"inferenceRoute":"global"}`, ErrUnknownClass},
		{"unknown route", `{"dataClass":{"level":"internal","residency":"any"},"inferenceRoute":"apac"}`, ErrUnknownRoute},
		{"missing class", `{"inferenceRoute":"global"}`, ErrUnknownClass},
		{"null class", `{"dataClass":null,"inferenceRoute":"global"}`, ErrUnknownClass},
		{"missing route", `{"dataClass":{"level":"internal","residency":"any"}}`, ErrUnknownRoute},
		{"trailing value", ok + `}{}`, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := DecodeFields([]byte(tc.body))
			if err == nil {
				t.Fatalf("DecodeFields accepted %s as %+v", tc.body, f)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("DecodeFields error = %v, want %v", err, tc.want)
			}
			if f != (ProjectInferenceFields{}) {
				t.Fatalf("DecodeFields returned %+v beside an error", f)
			}
		})
	}
	if _, err := DecodeFields([]byte(ok + `}`)); err != nil {
		t.Fatalf("DecodeFields(valid): %v", err)
	}
}

// The Hub's project record (route 15) carries more than these four fields, so the
// record's other keys are ignored and the four are still held to the strict rules.
// Kills: task 13 decoding the whole record strictly (every read fails) or the class loosely.
func TestFieldsFromRecordIgnoresTheRecordButNotTheClass(t *testing.T) {
	const record = `{"id":"pj-1","teamId":"tm-1","name":"Engagement Portal","remotes":[],` +
		`"createdAt":"2026-09-25T00:00:00Z","createdBy":{"id":"u-1","displayName":"A","email":"a@example.com"},` +
		`"dataClass":{"level":"restricted","residency":"au-only"},"inferenceRoute":"global",` +
		`"routeChangedBy":null,"routeChangedAt":null}`
	f, err := FieldsFromRecord([]byte(record))
	if err != nil {
		t.Fatalf("FieldsFromRecord: %v", err)
	}
	eff, err := f.Effective()
	if err != nil || eff != RouteAU {
		t.Fatalf("Effective = %q, %v; an au-only record whose route says global must run AU", eff, err)
	}
	for name, bad := range map[string]string{
		"no class":          `{"id":"pj-1","inferenceRoute":"global"}`,
		"class with extras": `{"id":"pj-1","dataClass":{"level":"internal","residency":"any","x":1},"inferenceRoute":"global"}`,
		"no route":          `{"id":"pj-1","dataClass":{"level":"internal","residency":"any"}}`,
	} {
		if _, err := FieldsFromRecord([]byte(bad)); err == nil {
			t.Fatalf("%s: FieldsFromRecord accepted %s", name, bad)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/dataclass/... -count=1`

Expected: the build fails, because nothing in the package exists yet. The first lines (the compiler stops at ten errors):

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass [github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass.test]
control-plane/internal/dataclass/dataclass_test.go:11:15: undefined: Class
control-plane/internal/dataclass/dataclass_test.go:11:28: undefined: LevelConfidential
...
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass [build failed]
```

The column numbers are indicative; the pass condition is `[build failed]` with `undefined:` lines, not a panic and not a different package.

- [ ] **Step 3: Write the package**

Create `control-plane/internal/dataclass/dataclass.go`. It is written here in four fenced blocks, one file.

```go
// Package dataclass decides where a project's inference may run.
//
// A project carries a data class (set at creation, updatable, OD-24) and a
// chosen inference route. The route it actually runs on is the tighter of the
// class's floor and the choice. Raising is open to any member; lowering needs
// the org compliance role and never goes below the floor. Anything this
// package cannot parse is an error, never Global (P6-D5).
//
// Standard library only: every gateway part imports this, and nothing here
// may import them back.
package dataclass

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

var (
	ErrUnknownClass    = errors.New("dataclass: unknown data class")
	ErrUnknownRoute    = errors.New("dataclass: unknown inference route")
	ErrNeedsCompliance = errors.New("dataclass: lowering needs the org compliance role")
	ErrBelowFloor      = errors.New("dataclass: the route would be below the data class's floor")
	ErrNotMember       = errors.New("dataclass: only a member of the project's team may change it")
)
```

The class, its one table of levels, and the routes:

```go
// Level is how sensitive a project's material is. PROPOSED (owner question Q1):
// the names are placeholders for KPMG's scheme, and only their rank matters.
type Level string

const (
	LevelPublic       Level = "public"
	LevelInternal     Level = "internal"
	LevelConfidential Level = "confidential"
	LevelRestricted   Level = "restricted"
)

// levels is the whole scheme, least sensitive first. It is the only place a
// level is named; nothing outside this file switches on one.
var levels = [...]Level{LevelPublic, LevelInternal, LevelConfidential, LevelRestricted}

// Levels returns the scheme in rank order, as a copy.
func Levels() []Level { return append([]Level(nil), levels[:]...) }

func levelRank(l Level) (int, bool) {
	for i, x := range levels {
		if l == x {
			return i, true
		}
	}
	return 0, false
}

// Residency says whether the material may leave Australia.
type Residency string

const (
	ResidencyAny    Residency = "any"
	ResidencyAUOnly Residency = "au-only"
)

// Class is a project's data class. The zero value is invalid.
type Class struct {
	Level     Level     `json:"level"`
	Residency Residency `json:"residency"`
}

// Validate refuses anything outside the table, including the zero value.
func (c Class) Validate() error {
	if _, ok := levelRank(c.Level); !ok {
		return fmt.Errorf("%w: level %q is not one of %q", ErrUnknownClass, c.Level, Levels())
	}
	if c.Residency != ResidencyAny && c.Residency != ResidencyAUOnly {
		return fmt.Errorf("%w: residency %q is not %q or %q", ErrUnknownClass, c.Residency, ResidencyAny, ResidencyAUOnly)
	}
	return nil
}

// UnmarshalJSON refuses an unknown key inside the class object and an unknown
// value. A JSON null leaves the zero value, which the caller's Validate refuses.
func (c *Class) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	type plain Class // no methods, so no recursion
	var p plain
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return fmt.Errorf("%w: %v", ErrUnknownClass, err)
	}
	if err := Class(p).Validate(); err != nil {
		return err
	}
	*c = Class(p)
	return nil
}
```

The route, the floor and the ratchet:

```go
// Route is where a project's inference runs. AU is the tighter.
type Route string

const (
	RouteGlobal Route = "global" // the direct Anthropic API (OD-19's default)
	RouteAU     Route = "au"     // Amazon Bedrock, ap-southeast-2
)

func routeRank(r Route) (int, bool) {
	switch r {
	case RouteGlobal:
		return 0, true
	case RouteAU:
		return 1, true
	}
	return 0, false
}

// Valid reports whether r is one of the two routes.
func (r Route) Valid() bool {
	_, ok := routeRank(r)
	return ok
}

// ParseRoute is exact: "Global" and "AU" are refused, so a case slip is loud.
func ParseRoute(s string) (Route, error) {
	if r := Route(s); r.Valid() {
		return r, nil
	}
	return "", fmt.Errorf("%w: %q is not %q or %q", ErrUnknownRoute, s, RouteGlobal, RouteAU)
}

// UnmarshalJSON refuses an unknown route. A JSON null leaves "", which the
// caller's validation refuses.
func (r *Route) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("%w: %v", ErrUnknownRoute, err)
	}
	p, err := ParseRoute(s)
	if err != nil {
		return err
	}
	*r = p
	return nil
}

// Floor is the loosest route a class permits. Only residency "any" floors at
// Global; everything else, including a residency nobody could parse, floors at AU.
func Floor(c Class) Route {
	if c.Residency == ResidencyAny {
		return RouteGlobal
	}
	return RouteAU
}

// Tighter returns the tighter of two routes. An unknown route counts as tighter
// than both known ones, so it reaches the next check and is refused there
// instead of becoming Global (decision (a)).
func Tighter(a, b Route) Route {
	ra, ok := routeRank(a)
	if !ok {
		return a
	}
	rb, ok := routeRank(b)
	if !ok || rb > ra {
		return b
	}
	return a
}

// Decide is the route a project actually runs on: the tighter of its class's
// floor and the route chosen for it. On error the route is "".
func Decide(c Class, chosen Route) (Route, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	if !chosen.Valid() {
		return "", fmt.Errorf("%w: chosen route %q", ErrUnknownRoute, chosen)
	}
	return Tighter(Floor(c), chosen), nil
}
```

Who may change what, and the Hub wire shape:

```go
// Actor is who is asking. Compliance is the org role of ask H-4. Solo means
// this member owns a solo project (decision (e), owner question Q5): it may
// lower the route to the floor, and it counts for nothing without Member and
// for nothing in CanReclassify.
type Actor struct {
	Member     bool
	Compliance bool
	Solo       bool
}

// CanChange applies OD-24's ratchet to a route change. The floor is checked
// before the role, so nobody is told to ask for a role that cannot help.
func CanChange(c Class, from, to Route, a Actor) error {
	if err := c.Validate(); err != nil {
		return err
	}
	rf, ok := routeRank(from)
	if !ok {
		return fmt.Errorf("%w: current route %q", ErrUnknownRoute, from)
	}
	rt, ok := routeRank(to)
	if !ok {
		return fmt.Errorf("%w: requested route %q", ErrUnknownRoute, to)
	}
	if !a.Member && !a.Compliance {
		return ErrNotMember
	}
	if rt >= rf {
		return nil // a raise, or no change
	}
	if floor, _ := routeRank(Floor(c)); rt < floor {
		return fmt.Errorf("%w: class %s/%s runs on %q at least", ErrBelowFloor, c.Level, c.Residency, Floor(c))
	}
	// Only after the floor: Solo lowers a choice, never a floor. A Solo actor
	// reaching here is a Member, since the check above refused one that is not.
	if !a.Compliance && !a.Solo {
		return ErrNeedsCompliance
	}
	return nil
}

// CanReclassify applies the same ratchet to a class change (decision (c)): a
// higher level or a stricter residency needs a member; lowering either needs
// compliance.
func CanReclassify(from, to Class, a Actor) error {
	if err := from.Validate(); err != nil {
		return fmt.Errorf("current class: %w", err)
	}
	if err := to.Validate(); err != nil {
		return fmt.Errorf("requested class: %w", err)
	}
	if !a.Member && !a.Compliance {
		return ErrNotMember
	}
	lf, _ := levelRank(from.Level)
	lt, _ := levelRank(to.Level)
	ff, _ := routeRank(Floor(from))
	ft, _ := routeRank(Floor(to))
	if (lt < lf || ft < ff) && !a.Compliance {
		return ErrNeedsCompliance
	}
	return nil
}

// User is the Hub's principal as others see it (hub-contract.md:618-619).
type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

// ProjectInferenceFields is ask H-1's addition to the Hub's project record,
// exactly. The change fields are null until the first change, never omitted.
type ProjectInferenceFields struct {
	DataClass      Class      `json:"dataClass"`
	InferenceRoute Route      `json:"inferenceRoute"`
	RouteChangedBy *User      `json:"routeChangedBy"`
	RouteChangedAt *time.Time `json:"routeChangedAt"`
}
```

And the two decoders:

```go
// Validate checks the class and the stored route. It does not apply the floor;
// Effective does.
func (f ProjectInferenceFields) Validate() error {
	if err := f.DataClass.Validate(); err != nil {
		return err
	}
	if !f.InferenceRoute.Valid() {
		return fmt.Errorf("%w: inferenceRoute %q", ErrUnknownRoute, f.InferenceRoute)
	}
	return nil
}

// Effective is the route the project runs on, never the stored route trusted.
func (f ProjectInferenceFields) Effective() (Route, error) {
	return Decide(f.DataClass, f.InferenceRoute)
}

// DecodeFields decodes a standalone fields object strictly: an unknown key at
// either level, a trailing value, or a missing class or route is an error.
func DecodeFields(data []byte) (ProjectInferenceFields, error) {
	var f ProjectInferenceFields
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return ProjectInferenceFields{}, fmt.Errorf("dataclass: decode fields: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return ProjectInferenceFields{}, errors.New("dataclass: decode fields: trailing content after the object")
	}
	if err := f.Validate(); err != nil {
		return ProjectInferenceFields{}, err
	}
	return f, nil
}

// FieldsFromRecord reads the four fields out of a larger record (the Hub's
// project, route 15), ignoring the record's other keys. The class object itself
// is still strict, and a missing class or route is still an error.
func FieldsFromRecord(data []byte) (ProjectInferenceFields, error) {
	var f ProjectInferenceFields
	if err := json.Unmarshal(data, &f); err != nil {
		return ProjectInferenceFields{}, fmt.Errorf("dataclass: decode record: %w", err)
	}
	if err := f.Validate(); err != nil {
		return ProjectInferenceFields{}, err
	}
	return f, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./control-plane/internal/dataclass/... -race -count=1 -v`

Expected, eleven top-level `--- PASS` lines and one `ok` (subtest lines omitted here; durations vary):

```
--- PASS: TestDecideTakesTheTighterOfFloorAndChoice (0.00s)
--- PASS: TestDecideRefusesWhatItCannotParse (0.00s)
--- PASS: TestTheZeroClassIsInvalid (0.00s)
--- PASS: TestFloor (0.00s)
--- PASS: TestTighter (0.00s)
--- PASS: TestParseRoute (0.00s)
--- PASS: TestCanChangeIsTheRatchet (0.00s)
--- PASS: TestCanReclassifyTightensFreelyAndLoosensOnlyWithCompliance (0.00s)
--- PASS: TestProjectInferenceFieldsWireShape (0.00s)
--- PASS: TestDecodeFieldsIsStrict (0.00s)
--- PASS: TestFieldsFromRecordIgnoresTheRecordButNotTheClass (0.00s)
PASS
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass	1.2s
```

Three failures are worth predicting. `TestDecodeFieldsIsStrict/null_class` passing but `missing_class` failing means `Validate` runs only inside `UnmarshalJSON`, which is never called for an absent key; the post-decode `f.Validate()` is what catches it. `TestProjectInferenceFieldsWireShape/fresh` printing no `routeChangedBy` means someone added `omitempty`, which the Hub's null rule forbids. `TestFloor` failing on `""` means `Floor` was written as "`au-only` → AU, else Global".

- [ ] **Step 5: Mutation spot-check, by hand, then revert**

Each of these edits must turn at least one test red; if one does not, the test is missing its mutant. Make it, run Step 4, confirm the named failure, and revert with your editor (never `git checkout`):

| Edit in `dataclass.go` | Test that must fail |
|---|---|
| `Decide` returns `chosen, nil` after validation | `TestDecideTakesTheTighterOfFloorAndChoice/au-only_floor_beats_a_global_choice` |
| `Validate` gains `if c == (Class{}) { return nil }` | `TestTheZeroClassIsInvalid` |
| `Tighter`'s first `return a` becomes `return RouteGlobal` | `TestTighter` |
| delete the `ErrBelowFloor` branch in `CanChange` | `TestCanChangeIsTheRatchet/compliance_below_the_floor` |
| delete `if !a.Compliance && !a.Solo { return ErrNeedsCompliance }` in `CanChange` | `TestCanChangeIsTheRatchet/member_lowers` |
| `!a.Compliance && !a.Solo` in `CanChange` becomes `!a.Compliance` (decision (e) ignored) | `TestCanChangeIsTheRatchet/solo_owner_lowers_an_any_project` |
| `CanChange` gains `if a.Solo && rt < rf { return nil }` before the floor check | `TestCanChangeIsTheRatchet/solo_owner_below_the_floor` |
| the membership check becomes `!a.Member && !a.Compliance && !a.Solo` | `TestCanChangeIsTheRatchet/solo_without_membership` |
| drop the residency half (`ft < ff`) of the condition in `CanReclassify` | `TestCanReclassifyTightensFreelyAndLoosensOnlyWithCompliance/member_loosens_residency` |
| `!a.Compliance` in `CanReclassify` becomes `!a.Compliance && !a.Solo` | `TestCanReclassifyTightensFreelyAndLoosensOnlyWithCompliance/solo_owner_loosens_residency` |

- [ ] **Step 6: Format, vet and run both modules**

From the repository root:

```bash
gofmt -l control-plane/internal/dataclass
go vet ./control-plane/internal/dataclass/...
go test ./sandboxd/... ./control-plane/... -race -count=1
```

`gofmt -l` and `go vet` print nothing on success. The suite passes when every package reports `ok` or `[no test files]` and no line says `FAIL`; this task is responsible for one line:

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass	1.2s
```

Then prove the package kept its promise of no dependency outside the standard library:

```bash
go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./control-plane/internal/dataclass
```

Expected: exactly one line, the package itself. Any second line is a dependency outside the standard library, and a failure:

```
github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass
```

- [ ] **Step 7: Commit**

Write the message to a file (the shell rule: no heredoc), then commit only this task's paths:

```bash
git add control-plane/internal/dataclass/dataclass.go control-plane/internal/dataclass/dataclass_test.go
git commit -F /tmp/task-01-msg.txt -- control-plane/internal/dataclass
```

with `/tmp/task-01-msg.txt`:

```
feat(dataclass): the data class, the route and the ratchet

A project runs on the tighter of its data class's floor and the route
chosen for it. Only residency "any" floors at Global, so a class nobody
can parse floors at AU. Raising is open to any member; lowering needs
the org compliance role and never goes below the floor, and the floor is
checked first so nobody is told to ask for a role that cannot help.
A solo project's owner (Actor.Solo) may lower the route to the floor,
as 05-design.md 2.5 says; that is owner question Q5. The same ratchet
applies to a class change, where Solo counts for nothing.

The zero class is invalid and every unparsed value is an error, never
Global (P6-D5). The Hub's project fields are fixed here, with the Hub's
null-not-omitted rule, and decoded strictly on their own and leniently
inside the Hub's larger project record.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

**What this task does not claim:**

- That the four levels are KPMG's scheme, or that no level forces AU. Both are owner question Q1; the table and `Floor` are the two places that change.
- How the Hub authorises a class or route change, audits it, or announces it. That is ask H-1 (and H-4 for the compliance role); this package only says whether a change is permitted to an actor already identified.
- Which scans a class requires on the deploy side. That mapping is phase 7's (ask P7-1, seam S6.7); phase 6 owns only the enum.
- That phase 4's create flow requires a class (ask P4-1, seam S6.8).
- That a solo project may go back to Global. `Solo` builds the source design's rule (decision (e)); whether it stands is owner question Q5, and the answer changes task 21's constant, not this package.
- That the Hub's `User` shape is final: it is read from the Hub contract at `hub-contract.md:618-619` on 2026-09-25, in another tree.
