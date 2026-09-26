### Task 21: Control-plane inference API

The cockpit needs two things about a project's inference: what it runs on now, and how much it has spent against each budget. It also needs one action, raising the project to AU. This task gives the local control plane `GET` and `PATCH /v1/projects/{projectId}/inference` for that, and nothing else. The same shape will later be served by the Hub for cloud projects (ask H-1, seam S6.1); the desktop does not change when it is.

**The one thing this task must not get wrong is who may loosen a route.** OD-24 is "tighten, never loosen" (`docs/superpowers/planning/2026-09-23-owner-decisions.md:140-152`), and task 01's `dataclass.CanChange` and `CanReclassify` are that rule. The local control plane has one caller, the person at this PC, and it calls both functions with `dataclass.Actor{Member: true}` and **never** with `Compliance: true`. There is no configuration, flag or header that changes that. So every lowering this API sees is `403 needs_compliance`, and every raise succeeds.

The second thing is **effective, never stored**. The panel shows the route the gatekeeper will actually apply, computed through `dataclass.Decide` (task 01), so an `au-only` class whose stored route says `global` shows `au`.

**Files:**
- `control-plane/internal/api/inference.go`: the two handlers, the wire shape, `ProjectInferenceStore`, `UsageReader`, `GatewayDeps`, `registerInference` (new)
- `control-plane/internal/api/inference_file.go`: `FileInferenceStore`, which writes task 13's `projects.json` atomically under a lock (new; see decision (f))
- `control-plane/internal/api/inference_test.go`: sixteen tests, one mutant each (new)
- `control-plane/internal/api/api.go`: two `Deps` fields, their checks in `New`, and one `a.registerInference(guarded)` line (modified)
- `control-plane/internal/api/routes_audit_test.go`: one line in `catalogReadOnlyRoutes` for the new `GET` (modified)
- `control-plane/cmd/api/config.go`, `config_test.go`: `PANACEA_APP_GATEWAY_ROUTE_SOURCE` read here as well as task 15's variables, two table rows (modified)
- `control-plane/cmd/api/main.go`: the store and `GatewayDeps` set on `Deps` (modified; gap 7)

**Interfaces:**
- Produces:
  - `GET /v1/projects/{projectId}/inference` → `200 inferenceJSON` (below); `404 project_not_found`
  - `PATCH /v1/projects/{projectId}/inference` with exactly one of `{"route": "au"}` or `{"dataClass": {"level": …, "residency": …}}` → `200 inferenceJSON` (the state after the write); `400 bad_request`, `400 invalid_json`, `403 needs_compliance`, `404 project_not_found`, `409 conflict`, `500 internal`
  - Both are also served under `/api/v1/…` by the existing alias (`control-plane/internal/api/api.go:379`), with no extra line
  - `api.ProjectInferenceStore`, `api.UsageReader`, `api.GatewayDeps`, `api.NewFileInferenceStore`
- Consumes:
  - task 01: `dataclass.Class`, `Route`, `RouteGlobal`, `RouteAU`, `ParseRoute`, `Floor`, `Tighter`, `Decide`, `Actor`, `CanChange`, `CanReclassify`, `ErrNeedsCompliance`, `ErrBelowFloor`, `ErrUnknownClass`, `ErrUnknownRoute`, `User`, `ProjectInferenceFields`, `DecodeFields`
  - task 03: `wire.UsageRow`, `wire.Owner`, `wire.CodeNeedsCompliance` (the constant for `"needs_compliance"`, 403)
  - task 06: `budget.PeriodStart` (the UTC calendar month the rows are filtered to)
  - task 15a: the `gatewayclient.Client`, which must satisfy `api.UsageReader` (gap 1); task 15b: the owner and team `cmd/api` mints for (gap 2) and the `cmd/api` wiring this part extends (review I-10 split task 15; `task-15.md` holds both halves)
  - phase 2: `httpx.WriteJSON`, `httpx.WriteProblem`, `httpx.CodeBadRequest`, `httpx.CodeConflict`, `httpx.CodeProjectNotFound` (`control-plane/internal/httpx/problem.go:45-60`, `:83`, `:93`); `Deps.Projects` (`api.go:52-55`, `:98`); `projectNotFound` (`catalog.go:225`); `a.internal` (`api.go:430`); the `routes` recorder (`api.go:247-266`)
- Required by: task 22 (the desktop mirrors `inferenceJSON` field for field); task 23 (step 3 raises a project through this route)

**Decisions where the skeleton is silent:**

**(a) A project with no entry is unclassified, and the first classification is free.** Local projects are git repositories the control plane finds on disk (`project.Scanner`); there is no create flow here to require a class at, which is where OD-24 puts it (and where ask P4-1 puts it for the cockpit). So a local project starts with no entry in `projects.json`. `GET` answers it with `dataClass`, `route` and `effectiveRoute` all `null`, and task 13's `File` source answers `ErrRouteUnknown`, so the gatekeeper refuses its sessions `503 route_unknown` (P6-D5). The first `PATCH {"dataClass": …}` on it is the creation-time choice: any class is accepted, no ratchet applies (there is nothing to loosen from), and the stored route is set to `dataclass.Floor` of the class. A `PATCH {"route": …}` on an unclassified project is `409 conflict`, "Set a data class first."

**(b) A class change ratchets the stored route too.** When a `PATCH {"dataClass": …}` succeeds, the stored route becomes `Tighter(stored route, Floor(new class))`. Without that, raising a class to `au-only` leaves the stored route `global`, `Decide` shows `au` today, and a later lowering of the class by a compliance officer silently puts the project back on Global. Tighten, never loosen, applies to what is written, not only to what is computed.

**(c) A lowering below the floor is `409 conflict`, not `403`.** Task 01's `CanChange` checks the floor before the role, so `ErrBelowFloor` means no role can help. `403 needs_compliance` would send the person to ask for a role that cannot do it. Task 03's code list has no `below_floor`; `conflict` (phase 2's "wrong state for the verb", `problem.go:54`) with `details.floor` is the nearest honest code, and gap 4 proposes a dedicated one.

**(d) Usage is best-effort; the route is not.** A keysmith that is down, slow (3 s budget for all three calls) or refusing makes `usageState: "unavailable"` and `usage: []`, and the rest of the answer is still `200`. The route is the safety fact; the spend figure is information. A store read that fails is `500`, because then the route itself is unknown.

**(e) Only the current month's rows are returned.** keysmith's `GET /v1/usage?scope=&id=` (task 08, task 09) may return more than one period. This API keeps, for each of the three scopes, the row whose `periodStart` equals `budget.PeriodStart(now)`, so the cockpit never has to choose. A scope with no current row is omitted, and task 22 says so in words.

**(f) The file store is its own file.** The skeleton lists `inference.go` only. The atomic writer, its lock and its read-modify-write are a separate concern with their own tests, and `inference_file.go` keeps the handler file readable. Nothing else moves.

**(g) The configuration is the route source task 13 already names.** The control plane reads `PANACEA_APP_GATEWAY_ROUTE_SOURCE`, the same variable keysmith and the gatekeeper read, and accepts only `file:<path>` or unset. `hub` fails start-up with a sentence naming seam S6.1, because the Hub store is phase H's and is not stubbed here. `PANACEA_APP_GATEWAY_KEYSMITH_URL` set with the route source unset also fails start-up: sessions would mint for projects whose route this control plane cannot show or change. The route source set without keysmith is allowed (classify now, gateway later).

**(h) The store renames, so its readers must mount the directory.** `FileInferenceStore` writes `projects.json.tmp`, syncs it and renames it over `projects.json`, so no reader ever sees half a file. A container that bind-mounts the **file** keeps the old inode after a rename and never sees the change (the skeleton's own rule for the token directory, "Shared names"). So keysmith and the gatekeeper must mount the directory holding `projects.json`, and `ROUTE_SOURCE` names the file inside it. That is task 17's to honour (gap 3).

**(i) The local person and team.** `routeChangedBy` is a `dataclass.User` whose `id` is `<tid>:<oid>`, the same string task 07 passes to `EnsureUser`; the usage calls ask for that person, this project, and the team task 15 mints for. Both come from `GatewayDeps`, which `cmd/api` fills from task 15's configuration (gap 2). With the gateway off, `routeChangedBy.id` is `"local"`.

- [ ] **Step 1: Preflight**

```bash
cd control-plane
test -e internal/api/inference.go && echo "STOP: inference.go exists" || echo "ok"            # expect ok
grep -c 'func Decide(c Class, chosen Route)' internal/dataclass/dataclass.go                     # expect 1 (task 01 merged)
grep -c 'type UsageRow struct' internal/inference/wire/wire.go                                   # expect 1 (task 03 merged)
grep -c 'func PeriodStart' internal/budget/budget.go                                             # expect 1 (task 06 merged)
grep -n 'json:"' internal/inference/wire/wire.go | grep -A7 'UsageRow' | head -8                 # read the six keys
grep -c 'func (c \*Client) Usage' internal/gatewayclient/client.go                               # expect 1, else gap 1 first
grep -n 'a.registerAttach(guarded)' internal/api/api.go                                          # expect one line, near :341
```

The fifth line prints task 03's `UsageRow` JSON keys. This part assumes `scope`, `id`, `periodStart`, `spendUsd`, `budgetUsd`, `requests`. If task 03 chose others, this API is unaffected (it embeds `wire.UsageRow` and never renames a key), and task 22's `UsageRow` type follows task 03, not this file. The sixth line is gap 1: if it prints `0`, add task 15's `Usage` method before starting, with its own test, or stop and report.

- [ ] **Step 2: Write the failing tests**

Sixteen tests in `package api_test`, reusing phase 2's helpers in `api_test.go` (`catalogNewAPI`, `catalogDo`, `catalogTestProject`, `catalogBaseURL`, `api_test.go:26-198`). Each comment names the mutant it kills. The fakes come first.

Create `control-plane/internal/api/inference_test.go`:

```go
package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// infUsage is an api.UsageReader that records every (scope, id) it is asked for.
type infUsage struct {
	mu    sync.Mutex
	calls [][2]string
	rows  map[string][]wire.UsageRow // by scope
	err   error
}

func (u *infUsage) Usage(_ context.Context, scope, id string) ([]wire.UsageRow, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls = append(u.calls, [2]string{scope, id})
	if u.err != nil {
		return nil, u.err
	}
	return u.rows[scope], nil
}

var infOwner = wire.Owner{TID: "t-0000", OID: "o-1111"}

const infTeam = "team-audit"

// infNew builds the API over a FileInferenceStore in a temp dir. usage nil means
// the gateway is off. seed, when non-empty, is written as projects.json first.
func infNew(t *testing.T, usage api.UsageReader, seed string) (http.Handler, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projects.json")
	if seed != "" {
		if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	store, err := api.NewFileInferenceStore(path)
	if err != nil {
		t.Fatalf("NewFileInferenceStore: %v", err)
	}
	h := catalogNewAPI(t, func(d *api.Deps) {
		d.InferenceStore = store
		if usage != nil {
			d.Gateway = &api.GatewayDeps{Usage: usage, Owner: infOwner, TeamID: infTeam, Now: infNow}
		}
	})
	return h, path
}

// infNow is 12 September 2026, so the current period starts 1 September UTC.
func infNow() time.Time { return time.Date(2026, 9, 12, 3, 0, 0, 0, time.UTC) }

// infSeed is one project in task 13's projects.json, in the four-key shape.
func infSeed(project, level, residency, route string) string {
	return `{"` + project + `":{"dataClass":{"level":"` + level + `","residency":"` + residency +
		`"},"inferenceRoute":"` + route + `","routeChangedBy":null,"routeChangedAt":null}}`
}

// infBody mirrors inferenceJSON with plain types, so a renamed key fails here.
type infBody struct {
	ProjectID      string            `json:"projectId"`
	DataClass      *dataclass.Class  `json:"dataClass"`
	Route          *string           `json:"route"`
	EffectiveRoute *string           `json:"effectiveRoute"`
	RouteChangedBy *dataclass.User   `json:"routeChangedBy"`
	Gateway        string            `json:"gateway"`
	UsageState     string            `json:"usageState"`
	Usage          []wire.UsageRow   `json:"usage"`
	Raw            map[string]any    `json:"-"`
}

func infDecode(t *testing.T, body string) infBody {
	t.Helper()
	var b infBody
	if err := json.Unmarshal([]byte(body), &b); err != nil {
		t.Fatalf("body is not inferenceJSON: %v\n%s", err, body)
	}
	if err := json.Unmarshal([]byte(body), &b.Raw); err != nil {
		t.Fatal(err)
	}
	return b
}

func infProblem(t *testing.T, rec interface{ Result() *http.Response }, status int, code string) httpx.Problem {
	t.Helper()
	res := rec.Result()
	defer res.Body.Close()
	var p httpx.Problem
	if err := json.NewDecoder(res.Body).Decode(&p); err != nil {
		t.Fatalf("not the problem shape: %v", err)
	}
	if res.StatusCode != status || p.Error != code {
		t.Fatalf("got %d %q, want %d %q (%s)", res.StatusCode, p.Error, status, code, p.Message)
	}
	return p
}

func str(p *string) string {
	if p == nil {
		return "<null>"
	}
	return *p
}

const infPath = "/v1/projects/" + catalogTestProject + "/inference"

// Mutant: echoing the stored route as effectiveRoute instead of dataclass.Decide.
func TestInferenceGetShowsTheEffectiveRouteNotTheStoredOne(t *testing.T) {
	h, _ := infNew(t, nil, infSeed(catalogTestProject, "confidential", "au-only", "global"))
	rec := catalogDo(t, h, "GET", infPath, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d: %s", rec.Code, rec.Body)
	}
	b := infDecode(t, rec.Body.String())
	if str(b.Route) != "global" || str(b.EffectiveRoute) != "au" {
		t.Fatalf("route %s effective %s; an au-only class runs AU whatever the record says", str(b.Route), str(b.EffectiveRoute))
	}
	if b.DataClass == nil || b.DataClass.Residency != dataclass.ResidencyAUOnly {
		t.Fatalf("dataClass = %+v", b.DataClass)
	}
}

// Mutant: a PATCH that answers 200 with the new route and never writes it.
func TestInferenceARaiseIsStoredAndTheNextGetShowsIt(t *testing.T) {
	h, path := infNew(t, nil, infSeed(catalogTestProject, "internal", "any", "global"))
	rec := catalogDo(t, h, "PATCH", infPath, `{"route":"au"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH = %d: %s", rec.Code, rec.Body)
	}
	b := infDecode(t, catalogDo(t, h, "GET", infPath, "", nil).Body.String())
	if str(b.Route) != "au" || str(b.EffectiveRoute) != "au" {
		t.Fatalf("after a raise GET says route %s effective %s", str(b.Route), str(b.EffectiveRoute))
	}
	if b.RouteChangedBy == nil || b.RouteChangedBy.ID != "local" {
		t.Fatalf("routeChangedBy = %+v, want the local person", b.RouteChangedBy)
	}
	raw, _ := os.ReadFile(path)
	var file map[string]json.RawMessage
	_ = json.Unmarshal(raw, &file)
	f, err := dataclass.DecodeFields(file[catalogTestProject])
	if err != nil || f.InferenceRoute != dataclass.RouteAU {
		t.Fatalf("projects.json holds %s (%v); the gatekeeper reads the file, not this API", raw, err)
	}
}

// Mutant: CanChange called with Actor{Compliance: true}, or not called at all.
func TestInferenceARouteLoweringIsNeedsCompliance(t *testing.T) {
	seed := infSeed(catalogTestProject, "internal", "any", "au")
	h, path := infNew(t, nil, seed)
	rec := catalogDo(t, h, "PATCH", infPath, `{"route":"global"}`, nil)
	infProblem(t, rec, http.StatusForbidden, wire.CodeNeedsCompliance)
	if raw, _ := os.ReadFile(path); string(raw) != seed {
		t.Fatalf("a refused lowering rewrote projects.json:\n%s", raw)
	}
}

// Mutant: a class PATCH that skips CanReclassify, so a member lowers the level.
func TestInferenceAClassLoweringIsNeedsCompliance(t *testing.T) {
	h, _ := infNew(t, nil, infSeed(catalogTestProject, "confidential", "any", "global"))
	rec := catalogDo(t, h, "PATCH", infPath, `{"dataClass":{"level":"internal","residency":"any"}}`, nil)
	infProblem(t, rec, http.StatusForbidden, wire.CodeNeedsCompliance)
}

// Mutant: every CanChange error mapped to 403, sending the person to ask for a
// role that cannot help (decision (c)).
func TestInferenceBelowTheFloorIsConflictAndNamesTheFloor(t *testing.T) {
	h, _ := infNew(t, nil, infSeed(catalogTestProject, "restricted", "au-only", "au"))
	rec := catalogDo(t, h, "PATCH", infPath, `{"route":"global"}`, nil)
	p := infProblem(t, rec, http.StatusConflict, httpx.CodeConflict)
	if p.Details["floor"] != "au" {
		t.Fatalf("details = %v, want floor=au", p.Details)
	}
}

// Mutants: an unclassified project shown as Global; a first classification
// refused by the ratchet; the route left empty after it (decision (a)).
func TestInferenceAnUnclassifiedProjectIsNullAndItsFirstClassSetsTheFloor(t *testing.T) {
	h, _ := infNew(t, nil, "")
	b := infDecode(t, catalogDo(t, h, "GET", infPath, "", nil).Body.String())
	for _, k := range []string{"dataClass", "route", "effectiveRoute"} {
		if v, ok := b.Raw[k]; !ok || v != nil {
			t.Fatalf("%s = %v (present %v), want an explicit null", k, v, ok)
		}
	}
	rec := catalogDo(t, h, "PATCH", infPath, `{"route":"au"}`, nil)
	infProblem(t, rec, http.StatusConflict, httpx.CodeConflict)

	rec = catalogDo(t, h, "PATCH", infPath, `{"dataClass":{"level":"restricted","residency":"au-only"}}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("first classification = %d: %s", rec.Code, rec.Body)
	}
	b = infDecode(t, rec.Body.String())
	if str(b.Route) != "au" || str(b.EffectiveRoute) != "au" {
		t.Fatalf("first class au-only stored route %s effective %s, want au and au", str(b.Route), str(b.EffectiveRoute))
	}
}

// Mutant: a class change that leaves the stored route at global, so a later
// compliance lowering of the class silently returns the project to Global.
func TestInferenceAClassRaiseRatchetsTheStoredRoute(t *testing.T) {
	h, _ := infNew(t, nil, infSeed(catalogTestProject, "internal", "any", "global"))
	rec := catalogDo(t, h, "PATCH", infPath, `{"dataClass":{"level":"internal","residency":"au-only"}}`, nil)
	if b := infDecode(t, rec.Body.String()); str(b.Route) != "au" {
		t.Fatalf("stored route after a residency raise = %s, want au (decision (b))", str(b.Route))
	}
}

// Mutant: a PATCH that writes an entry for a project the scanner does not know,
// which task 13's File would then serve to a session.
func TestInferenceAnUnknownProjectIs404(t *testing.T) {
	h, path := infNew(t, nil, "")
	for _, m := range []string{"GET", "PATCH"} {
		rec := catalogDo(t, h, m, "/v1/projects/p-nope/inference", `{"route":"au"}`, nil)
		infProblem(t, rec, http.StatusNotFound, httpx.CodeProjectNotFound)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("projects.json exists after two 404s: %v", err)
	}
}

// Mutant: a handler that writes its own error JSON or http.Error text, or maps
// Route's UnmarshalJSON error to invalid_json for a well-formed body.
func TestInferencePatchBodyErrorsAreTheProblemShape(t *testing.T) {
	h, _ := infNew(t, nil, infSeed(catalogTestProject, "internal", "any", "global"))
	for _, tc := range []struct{ name, body, code string }{
		{"both fields", `{"route":"au","dataClass":{"level":"internal","residency":"any"}}`, httpx.CodeBadRequest},
		{"neither field", `{}`, httpx.CodeBadRequest},
		{"unknown route", `{"route":"eu"}`, httpx.CodeBadRequest},
		{"unknown level", `{"dataClass":{"level":"secret","residency":"any"}}`, httpx.CodeBadRequest},
		{"unknown field", `{"route":"au","by":"compliance"}`, httpx.CodeBadRequest},
		{"not json", `route=au`, httpx.CodeInvalidJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			infProblem(t, catalogDo(t, h, "PATCH", infPath, tc.body, nil), http.StatusBadRequest, tc.code)
		})
	}
}

// Mutant: a corrupt projects.json read as empty, so the next PATCH rewrites the
// file with one project and drops every other project's class.
func TestInferenceAnUnreadableStoreIs500AndIsNotOverwritten(t *testing.T) {
	h, path := infNew(t, nil, `{"p-other": {"dataClass":`)
	infProblem(t, catalogDo(t, h, "GET", infPath, "", nil), http.StatusInternalServerError, httpx.CodeInternal)
	infProblem(t, catalogDo(t, h, "PATCH", infPath, `{"dataClass":{"level":"internal","residency":"any"}}`, nil),
		http.StatusInternalServerError, httpx.CodeInternal)
	if raw, _ := os.ReadFile(path); string(raw) != `{"p-other": {"dataClass":` {
		t.Fatalf("a corrupt store was overwritten:\n%s", raw)
	}
}

// Mutant: gateway reported "on" with no keysmith, or usage encoded as null,
// which the cockpit's .map would throw on.
func TestInferenceGatewayOffSaysSoAndUsageIsAnEmptyArray(t *testing.T) {
	h, _ := infNew(t, nil, infSeed(catalogTestProject, "internal", "any", "global"))
	b := infDecode(t, catalogDo(t, h, "GET", infPath, "", nil).Body.String())
	if b.Gateway != "off" || b.UsageState != "off" {
		t.Fatalf("gateway %q usageState %q, want off and off", b.Gateway, b.UsageState)
	}
	if u, ok := b.Raw["usage"].([]any); !ok || len(u) != 0 {
		t.Fatalf("usage = %#v, want []", b.Raw["usage"])
	}
}

// Mutants: a team read that is really a project read (task 09's own mutant, one
// layer up); a person keyed by OID alone, which collides across tenants.
func TestInferenceUsageAsksForThisPersonThisProjectAndThisTeam(t *testing.T) {
	u := &infUsage{}
	h, _ := infNew(t, u, infSeed(catalogTestProject, "internal", "any", "global"))
	catalogDo(t, h, "GET", infPath, "", nil)
	want := map[[2]string]bool{
		{"person", "t-0000:o-1111"}: true,
		{"project", catalogTestProject}: true,
		{"team", infTeam}: true,
	}
	if len(u.calls) != 3 {
		t.Fatalf("calls = %v, want exactly three", u.calls)
	}
	for _, c := range u.calls {
		if !want[c] {
			t.Fatalf("unexpected usage call %v; want %v", c, want)
		}
	}
}

// Mutant: every period keysmith returns passed through, so the cockpit shows
// August's spend as this month's (decision (e)).
func TestInferenceUsageKeepsOnlyTheCurrentUTCMonth(t *testing.T) {
	sep := budget.PeriodStart(infNow())
	aug := sep.AddDate(0, -1, 0)
	u := &infUsage{rows: map[string][]wire.UsageRow{
		"project": {
			{Scope: "project", ID: catalogTestProject, PeriodStart: aug, SpendUSD: 900, BudgetUSD: 1000},
			{Scope: "project", ID: catalogTestProject, PeriodStart: sep, SpendUSD: 212.4, BudgetUSD: 1000},
		},
	}}
	h, _ := infNew(t, u, infSeed(catalogTestProject, "internal", "any", "global"))
	b := infDecode(t, catalogDo(t, h, "GET", infPath, "", nil).Body.String())
	if b.Gateway != "on" || b.UsageState != "ok" {
		t.Fatalf("gateway %q usageState %q", b.Gateway, b.UsageState)
	}
	if len(b.Usage) != 1 || b.Usage[0].SpendUSD != 212.4 || !b.Usage[0].PeriodStart.Equal(sep) {
		t.Fatalf("usage = %+v, want only September's project row", b.Usage)
	}
}

// Mutant: a keysmith error that fails the whole panel, hiding the route, which is
// the safety fact (decision (d)).
func TestInferenceAUsageFailureStillAnswersTheRoute(t *testing.T) {
	u := &infUsage{err: errors.New("keysmith: 503")}
	h, _ := infNew(t, u, infSeed(catalogTestProject, "internal", "any", "au"))
	rec := catalogDo(t, h, "GET", infPath, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d with keysmith down; the route must still be served", rec.Code)
	}
	b := infDecode(t, rec.Body.String())
	if b.UsageState != "unavailable" || str(b.EffectiveRoute) != "au" || len(b.Usage) != 0 {
		t.Fatalf("usageState %q effective %s usage %v", b.UsageState, str(b.EffectiveRoute), b.Usage)
	}
	if strings.Contains(rec.Body.String(), "keysmith: 503") {
		t.Fatalf("the upstream error text reached the body: %s", rec.Body)
	}
}

// Mutants: a writer that replaces the whole file with one project; a direct
// write a reader can catch half-way; a shape task 13's File cannot decode.
func TestFileInferenceStoreKeepsOtherProjectsWritesAtomicallyInTheFourKeyShape(t *testing.T) {
	other := infSeed("p-other", "restricted", "au-only", "au")
	h, path := infNew(t, nil, other)
	if rec := catalogDo(t, h, "PATCH", infPath, `{"dataClass":{"level":"internal","residency":"any"}}`, nil); rec.Code != 200 {
		t.Fatalf("PATCH = %d: %s", rec.Code, rec.Body)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]json.RawMessage
	if err := json.Unmarshal(raw, &file); err != nil || len(file) != 2 {
		t.Fatalf("projects.json = %s (%v), want two projects", raw, err)
	}
	for id, entry := range file {
		if _, err := dataclass.DecodeFields(entry); err != nil {
			t.Fatalf("entry %s is not the four-key shape task 13 decodes: %v\n%s", id, err, entry)
		}
	}
	if f, _ := dataclass.DecodeFields(file["p-other"]); f.InferenceRoute != dataclass.RouteAU {
		t.Fatalf("p-other lost its route: %s", file["p-other"])
	}
	if m, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp")); len(m) != 0 {
		t.Fatalf("temp files left behind: %v", m)
	}
}

// Mutant: an Update that reads, unlocks, then writes, so of two concurrent
// changes one is lost. Run with -race.
func TestFileInferenceStoreDoesNotLoseAConcurrentUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	if err := os.WriteFile(path, []byte(infSeed(catalogTestProject, "internal", "any", "global")), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := api.NewFileInferenceStore(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		id := "p-" + string(rune('a'+i))
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Update(ctx, id, func(dataclass.ProjectInferenceFields, bool) (dataclass.ProjectInferenceFields, error) {
				return dataclass.ProjectInferenceFields{
					DataClass:      dataclass.Class{Level: dataclass.LevelInternal, Residency: dataclass.ResidencyAny},
					InferenceRoute: dataclass.RouteGlobal,
				}, nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	raw, _ := os.ReadFile(path)
	var file map[string]json.RawMessage
	if err := json.Unmarshal(raw, &file); err != nil || len(file) != 21 {
		t.Fatalf("after 20 concurrent updates the file holds %d projects, want 21", len(file))
	}
}
```

That is sixteen tests. The existing route audit (`routes_audit_test.go`) is a seventeenth check that needs no new code: it fails until step 4's line for the new `GET` is added to `catalogReadOnlyRoutes`, because a `GET` that calls keysmith is still read-only (it changes nothing anywhere) and must be declared so.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/api/ -run 'Inference|FileInferenceStore' -count=1`

Expected: a build failure naming `api.NewFileInferenceStore`, `api.GatewayDeps` and `Deps.InferenceStore` as undefined. That is the red state; nothing in the package compiles until step 4.

- [ ] **Step 4: Write the types, the store interface and the wire shape**

Create `control-plane/internal/api/inference.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/budget"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// ProjectInferenceStore keeps each project's data class and route. ok is false
// for a project with no entry, which is unclassified (decision (a)). Update runs
// fn on the current value under the store's lock and writes what fn returns; an
// error from fn writes nothing and comes back unchanged, so errors.Is works on it.
//
// The file implementation is FileInferenceStore. The Hub implementation is
// phase H's (ask H-1, seam S6.1) and is deliberately not stubbed here.
type ProjectInferenceStore interface {
	Get(ctx context.Context, projectID string) (f dataclass.ProjectInferenceFields, ok bool, err error)
	Update(ctx context.Context, projectID string,
		fn func(cur dataclass.ProjectInferenceFields, ok bool) (dataclass.ProjectInferenceFields, error),
	) (dataclass.ProjectInferenceFields, error)
}

// UsageReader is keysmith's GET /v1/usage?scope=&id= (tasks 08, 09) as the
// control plane reads it. Task 15's gatewayclient.Client implements it.
type UsageReader interface {
	Usage(ctx context.Context, scope, id string) ([]wire.UsageRow, error)
}

// GatewayDeps is present exactly when the gateway is on
// (PANACEA_APP_GATEWAY_KEYSMITH_URL set, task 15). Owner and TeamID are the
// person and team this control plane mints for, so the usage it shows is the
// usage its own sessions draw on.
type GatewayDeps struct {
	Usage  UsageReader
	Owner  wire.Owner
	TeamID string
	Now    func() time.Time // nil means time.Now
}

// usageTimeout bounds the three usage calls together (decision (d)). It hangs
// off the request's context: this is a read, and a cockpit that gave up wants
// nothing more from it, unlike a mint (task 04).
const usageTimeout = 3 * time.Second

// inferenceJSON is the body of both routes, and task 22's ProjectInference type.
// A null is always written, never omitted: an unclassified project says so.
type inferenceJSON struct {
	ProjectID      string           `json:"projectId"`
	DataClass      *dataclass.Class `json:"dataClass"`
	Route          *dataclass.Route `json:"route"`
	EffectiveRoute *dataclass.Route `json:"effectiveRoute"`
	RouteChangedBy *dataclass.User  `json:"routeChangedBy"`
	RouteChangedAt *time.Time       `json:"routeChangedAt"`
	Gateway        string           `json:"gateway"`    // "on" | "off"
	UsageState     string           `json:"usageState"` // "ok" | "off" | "unavailable"
	Usage          []wire.UsageRow  `json:"usage"`      // never null; current UTC month only
}

// inferencePatch is the PATCH body. Both fields are raw so that an unknown route
// or level is bad_request with a sentence, not invalid_json from a custom
// UnmarshalJSON (httpx.DecodeJSON maps only type and unknown-field errors to
// bad_request, problem.go:165-173).
type inferencePatch struct {
	Route     *string          `json:"route"`
	DataClass *json.RawMessage `json:"dataClass"`
}

// errUnclassified is a route PATCH on a project with no data class.
var errUnclassified = errors.New("api: project has no data class")

// member is the only actor this control plane ever acts as. THERE IS NO PATH
// THAT BUILDS A COMPLIANCE ACTOR IN THIS PACKAGE: the local caller is the person
// at this PC, and a lowering is the org compliance role's (ask H-4), which only
// the Hub can establish.
var member = dataclass.Actor{Member: true}

func personID(o wire.Owner) string { return o.TID + ":" + o.OID }
```

- [ ] **Step 5: Write the handlers**

Append to `control-plane/internal/api/inference.go`:

```go
func (a *API) registerInference(mux *routes) {
	mux.HandleFunc("GET /v1/projects/{projectId}/inference", a.handleInferenceGet)
	mux.HandleFunc("PATCH /v1/projects/{projectId}/inference", a.handleInferencePatch)
}

// knownProject writes 404 or 500 and returns false unless the scanner knows id.
// A PATCH for an id it does not know must not create an entry task 13's File
// would then serve to a session.
func (a *API) knownProject(w http.ResponseWriter, r *http.Request, id string) bool {
	ps, err := a.deps.Projects.Projects(r.Context())
	if err != nil {
		a.internal(w, r, "enumerate projects", err)
		return false
	}
	for _, p := range ps {
		if p.ID == id {
			return true
		}
	}
	projectNotFound(w, id)
	return false
}

func (a *API) handleInferenceGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("projectId")
	if !a.knownProject(w, r, id) {
		return
	}
	var f dataclass.ProjectInferenceFields
	var ok bool
	if s := a.deps.InferenceStore; s != nil {
		var err error
		if f, ok, err = s.Get(r.Context(), id); err != nil {
			a.internal(w, r, "read project inference", err)
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, a.inferenceBody(r.Context(), id, f, ok))
}

// inferenceBody builds the answer. The effective route is computed, never
// copied from the record; an entry that fails Decide shows effectiveRoute null,
// which is what the gatekeeper will do with it too (P6-D5).
func (a *API) inferenceBody(ctx context.Context, id string, f dataclass.ProjectInferenceFields, ok bool) inferenceJSON {
	b := inferenceJSON{ProjectID: id, Gateway: "off", UsageState: "off", Usage: []wire.UsageRow{}}
	if ok {
		c, rt := f.DataClass, f.InferenceRoute
		b.DataClass, b.Route = &c, &rt
		b.RouteChangedBy, b.RouteChangedAt = f.RouteChangedBy, f.RouteChangedAt
		if eff, err := f.Effective(); err == nil {
			b.EffectiveRoute = &eff
		}
	}
	if g := a.deps.Gateway; g != nil {
		b.Gateway = "on"
		b.Usage, b.UsageState = a.currentUsage(ctx, g, id)
	}
	return b
}

// currentUsage asks keysmith for this person, this project and this team, and
// keeps each scope's row for the current UTC month (decision (e)). Any failure
// makes the whole figure unavailable rather than partly shown (decision (d));
// the error goes to the log, never to the body.
func (a *API) currentUsage(ctx context.Context, g *GatewayDeps, projectID string) ([]wire.UsageRow, string) {
	ctx, cancel := context.WithTimeout(ctx, usageTimeout)
	defer cancel()
	now := time.Now
	if g.Now != nil {
		now = g.Now
	}
	period := budget.PeriodStart(now())
	out := []wire.UsageRow{}
	for _, q := range [...]struct{ scope, id string }{
		{string(budget.Person), personID(g.Owner)},
		{string(budget.Project), projectID},
		{string(budget.Team), g.TeamID},
	} {
		rows, err := g.Usage.Usage(ctx, q.scope, q.id)
		if err != nil {
			a.deps.Log.Warn("inference usage unavailable", "scope", q.scope, "project", projectID, "err", err)
			return []wire.UsageRow{}, "unavailable"
		}
		for _, row := range rows {
			// string(...) compiles whether task 03 types Scope or not.
			if string(row.Scope) == q.scope && row.ID == q.id && row.PeriodStart.Equal(period) {
				out = append(out, row)
				break
			}
		}
	}
	return out, "ok"
}

func (a *API) localUser() dataclass.User {
	if g := a.deps.Gateway; g != nil {
		return dataclass.User{ID: personID(g.Owner), DisplayName: "This PC"}
	}
	return dataclass.User{ID: "local", DisplayName: "This PC"}
}

func (a *API) handleInferencePatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("projectId")
	var p inferencePatch
	if !httpx.DecodeJSON(w, r, &p) {
		return
	}
	if (p.Route == nil) == (p.DataClass == nil) {
		badRequest(w, `Send exactly one of "route" or "dataClass".`)
		return
	}
	var wantRoute dataclass.Route
	var wantClass dataclass.Class
	if p.Route != nil {
		rt, err := dataclass.ParseRoute(*p.Route)
		if err != nil {
			badRequest(w, `"route" must be "global" or "au".`)
			return
		}
		wantRoute = rt
	} else if err := json.Unmarshal(*p.DataClass, &wantClass); err != nil || wantClass.Validate() != nil {
		badRequest(w, `"dataClass" needs a known "level" and "residency".`)
		return
	}
	if !a.knownProject(w, r, id) {
		return
	}
	s := a.deps.InferenceStore
	if s == nil {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"Inference settings are not configured on this control plane.",
			map[string]string{"variable": "PANACEA_APP_GATEWAY_ROUTE_SOURCE"})
		return
	}
	by, now := a.localUser(), time.Now().UTC()
	var floor dataclass.Route
	f, err := s.Update(r.Context(), id, func(cur dataclass.ProjectInferenceFields, ok bool) (dataclass.ProjectInferenceFields, error) {
		switch {
		case p.Route != nil && !ok:
			return cur, errUnclassified
		case p.Route != nil:
			floor = dataclass.Floor(cur.DataClass)
			if err := dataclass.CanChange(cur.DataClass, cur.InferenceRoute, wantRoute, member); err != nil {
				return cur, err
			}
			cur.InferenceRoute = wantRoute
		case !ok: // the first classification (decision (a))
			cur = dataclass.ProjectInferenceFields{DataClass: wantClass, InferenceRoute: dataclass.Floor(wantClass)}
		default: // a reclassification, and the stored route ratchets with it (decision (b))
			if err := dataclass.CanReclassify(cur.DataClass, wantClass, member); err != nil {
				return cur, err
			}
			cur.DataClass = wantClass
			cur.InferenceRoute = dataclass.Tighter(cur.InferenceRoute, dataclass.Floor(wantClass))
		}
		cur.RouteChangedBy, cur.RouteChangedAt = &by, &now
		return cur, nil
	})
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusOK, a.inferenceBody(r.Context(), id, f, true))
	case errors.Is(err, errUnclassified):
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"Set a data class for this project first.", map[string]string{"projectId": id})
	case errors.Is(err, dataclass.ErrBelowFloor):
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"This project's data class keeps its inference on "+string(floor)+"; no role can move it lower.",
			map[string]string{"floor": string(floor)})
	case errors.Is(err, dataclass.ErrNeedsCompliance):
		httpx.WriteProblem(w, http.StatusForbidden, wire.CodeNeedsCompliance,
			"Only your organisation's compliance role can lower this project's route or data class.", nil)
	default:
		a.internal(w, r, "update project inference", err)
	}
}
```

`routeChangedBy` and `routeChangedAt` are set on every accepted write, a class change included, because a class change can move the effective route. The Hub's audit (ask H-1) is the real record; these two fields are what the cockpit shows. If task 03 declares its problem codes with a named type rather than as untyped string constants, write `string(wire.CodeNeedsCompliance)` at both call sites.

- [ ] **Step 6: Write the file store**

Create `control-plane/internal/api/inference_file.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
)

// FileInferenceStore is task 13's projects.json, written. It is the local
// control plane's ProjectInferenceStore; keysmith and the gatekeeper read the
// same file through routesource.File.
//
// Every write is temp file, fsync, rename, so a reader sees the old file or the
// new one and never half of either. A rename changes the inode, so a container
// must bind-mount the DIRECTORY, not the file (decision (h)).
//
// The lock is in-process. This control plane is the file's only writer; a hand
// edit made while it runs can be lost, and nothing here claims otherwise.
type FileInferenceStore struct {
	path string
	mu   sync.Mutex
}

var _ ProjectInferenceStore = (*FileInferenceStore)(nil)

// NewFileInferenceStore does not read the file: a missing file is an empty
// store, and a corrupt one is an error on first use, never an empty store.
func NewFileInferenceStore(path string) (*FileInferenceStore, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("api: inference store path %q is not absolute", path)
	}
	return &FileInferenceStore{path: path}, nil
}

// load returns every entry, raw. Only a missing file is empty. Anything else it
// cannot parse is an error, so a PATCH cannot rewrite a corrupt file with one
// project and drop the rest.
func (s *FileInferenceStore) load() (map[string]json.RawMessage, error) {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("api: read %s: %w", filepath.Base(s.path), err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, fmt.Errorf("api: %s is not a JSON object of projects", filepath.Base(s.path))
	}
	return m, nil
}

func decodeEntry(m map[string]json.RawMessage, id string) (dataclass.ProjectInferenceFields, bool, error) {
	e, ok := m[id]
	if !ok {
		return dataclass.ProjectInferenceFields{}, false, nil
	}
	f, err := dataclass.DecodeFields(e)
	if err != nil {
		return dataclass.ProjectInferenceFields{}, false, fmt.Errorf("api: project %s in projects.json: %w", id, err)
	}
	return f, true, nil
}

func (s *FileInferenceStore) Get(_ context.Context, id string) (dataclass.ProjectInferenceFields, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.load()
	if err != nil {
		return dataclass.ProjectInferenceFields{}, false, err
	}
	return decodeEntry(m, id)
}

func (s *FileInferenceStore) Update(_ context.Context, id string,
	fn func(cur dataclass.ProjectInferenceFields, ok bool) (dataclass.ProjectInferenceFields, error),
) (dataclass.ProjectInferenceFields, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.load()
	if err != nil {
		return dataclass.ProjectInferenceFields{}, err
	}
	cur, ok, err := decodeEntry(m, id)
	if err != nil {
		return dataclass.ProjectInferenceFields{}, err
	}
	next, err := fn(cur, ok)
	if err != nil {
		return dataclass.ProjectInferenceFields{}, err // unwrapped: the handler switches on it
	}
	if err := next.Validate(); err != nil {
		return dataclass.ProjectInferenceFields{}, fmt.Errorf("api: refusing to store project %s: %w", id, err)
	}
	enc, err := json.Marshal(next)
	if err != nil {
		return dataclass.ProjectInferenceFields{}, err
	}
	m[id] = enc
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return dataclass.ProjectInferenceFields{}, err
	}
	if err := writeFileAtomic(s.path, append(out, '\n')); err != nil {
		return dataclass.ProjectInferenceFields{}, err
	}
	return next, nil
}

// writeFileAtomic writes <path>.<random>.tmp at 0600 (os.CreateTemp's mode),
// syncs it, and renames it over path. On any failure the temp file is removed.
func writeFileAtomic(path string, data []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("api: create temp beside %s: %w", filepath.Base(path), err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("api: write %s: %w", filepath.Base(path), err)
	}
	return os.Rename(tmp.Name(), path)
}
```

`json.MarshalIndent` over a `map` sorts its keys, so the file is stable across writes and a diff shows exactly the project that changed.

- [ ] **Step 7: Wire it into `Deps`, `New` and the route audit**

In `control-plane/internal/api/api.go`, add two fields to `Deps` (`api.go:68`), after `Log` (`:110`):

```go
	// InferenceStore holds each project's data class and route (phase 6 task
	// 21). Nil when PANACEA_APP_GATEWAY_ROUTE_SOURCE is unset: GET answers
	// nulls and PATCH is 409 conflict naming that variable.
	InferenceStore ProjectInferenceStore

	// Gateway is nil when the gateway is off. When it is on, InferenceStore
	// must be set too: sessions would otherwise mint for projects whose route
	// this control plane can neither show nor change (decision (g)).
	Gateway *GatewayDeps
```

In `New`, after the `ConversationsDir` checks (`api.go:318-326`):

```go
	if g := d.Gateway; g != nil {
		switch {
		case d.InferenceStore == nil:
			return nil, errors.New("api: Deps.Gateway is set and Deps.InferenceStore is nil; set PANACEA_APP_GATEWAY_ROUTE_SOURCE=file:<path>")
		case g.Usage == nil:
			return nil, errors.New("api: Deps.Gateway.Usage is nil")
		case g.Owner.TID == "" || g.Owner.OID == "" || g.TeamID == "":
			return nil, errors.New("api: Deps.Gateway needs the owner's tid and oid and the team id it mints for")
		}
	}
```

After `a.registerAttach(guarded)` (`api.go:341`), one line:

```go
	a.registerInference(guarded)
```

In `control-plane/internal/api/routes_audit_test.go`, one entry in `catalogReadOnlyRoutes` (`routes_audit_test.go:32`):

```go
	"GET /v1/projects/{projectId}/inference": "One enumeration, one store read and at most three keysmith usage reads. The route and class are only ever WRITTEN by the PATCH beside it.",
```

- [ ] **Step 8: Read the route source in `cmd/api`**

In `control-plane/cmd/api/config.go`, beside task 15's gateway variables, add `envGatewayRouteSource = "PANACEA_APP_GATEWAY_ROUTE_SOURCE"` to the constant block (`config.go:29-64`) and a `GatewayRouteFile string` field to `Config`. In `loadConfig(getenv)` (`config.go:165`):

```go
	switch rs := strings.TrimSpace(getenv(envGatewayRouteSource)); {
	case rs == "":
		// classification off; allowed only while the gateway is off (below)
	case strings.HasPrefix(rs, "file:") && filepath.IsAbs(strings.TrimPrefix(rs, "file:")):
		c.GatewayRouteFile = strings.TrimPrefix(rs, "file:")
	case rs == "hub":
		return Config{}, fmt.Errorf("%s=hub: the Hub's project store is phase H's (seam S6.1); the local control plane accepts only file:<absolute path>", envGatewayRouteSource)
	default:
		return Config{}, fmt.Errorf("%s=%q: want file:<absolute path> or unset", envGatewayRouteSource, rs)
	}
	if c.GatewayRouteFile == "" && c.GatewayKeysmithURL != "" { // task 15's field; use its real name
		return Config{}, fmt.Errorf("%s is set and %s is not; sessions would mint for projects whose route this control plane cannot show", envGatewayKeysmithURL, envGatewayRouteSource)
	}
```

In `control-plane/cmd/api/main.go`, where task 15 builds its `gatewayclient.Client`, set `deps.InferenceStore` from `api.NewFileInferenceStore(cfg.GatewayRouteFile)` when the field is non-empty, and `deps.Gateway = &api.GatewayDeps{Usage: client, Owner: <task 15's owner>, TeamID: <task 15's team>}` when the client exists. Add two rows to `config_test.go`'s table: `hub` fails naming the variable and S6.1; the keysmith URL without a route source fails naming both variables.

- [ ] **Step 9: Run the tests to verify they pass**

```bash
go test ./control-plane/internal/api/ -run 'Inference|FileInferenceStore' -race -count=1 -v 2>&1 | grep -c '^--- PASS'   # expect 16
go test ./control-plane/internal/api/ -race -count=1                                                       # expect ok: phase 2's suite, the route audit included
go test ./control-plane/cmd/api/ -race -count=1                                                            # expect ok
```

- [ ] **Step 10: Kill four mutants by hand**

Each edit, made alone and reverted, must turn exactly the named test red:

1. `var member = dataclass.Actor{Member: true, Compliance: true}` → `TestInferenceARouteLoweringIsNeedsCompliance` and `TestInferenceAClassLoweringIsNeedsCompliance`.
2. In `inferenceBody`, `b.EffectiveRoute = &rt` in place of the `Effective()` call → `TestInferenceGetShowsTheEffectiveRouteNotTheStoredOne`.
3. In `load`, `return map[string]json.RawMessage{}, nil` on a parse error → `TestInferenceAnUnreadableStoreIs500AndIsNotOverwritten`.
4. Delete `s.mu.Lock()` and its `defer` from `Update` → `TestFileInferenceStoreDoesNotLoseAConcurrentUpdate` (under `-race`, a data race or a short count).

- [ ] **Step 11: Vet, format, commit**

```bash
gofmt -l control-plane/internal/api/ control-plane/cmd/api/          # expect nothing
go vet ./control-plane/internal/api/ ./control-plane/cmd/api/          # expect nothing
git add control-plane/internal/api/inference.go control-plane/internal/api/inference_file.go control-plane/internal/api/inference_test.go
git commit -F <message file> -- control-plane/internal/api/inference.go control-plane/internal/api/inference_file.go \
  control-plane/internal/api/inference_test.go control-plane/internal/api/api.go control-plane/internal/api/routes_audit_test.go \
  control-plane/cmd/api/config.go control-plane/cmd/api/config_test.go control-plane/cmd/api/main.go
```

Message: `feat(api): project inference route and class, raise-only, with usage (phase 6 task 21)`, a body naming decisions (a) to (i), and the attribution line.

**Acceptance:**

1. Step 9 prints `16`, then `ok` twice.
2. Step 10's four edits each fail the named test.
3. By hand, with task 17's stack up and the control plane started with `PANACEA_APP_GATEWAY_ROUTE_SOURCE=file:<repo>/deploy/local/gateway/projects/projects.json`:

```bash
curl -s http://127.0.0.1:8730/v1/projects/<id>/inference
# expect "effectiveRoute":null and "gateway":"on" for a project not yet in projects.json
curl -s -X PATCH -d '{"dataClass":{"level":"internal","residency":"any"}}' http://127.0.0.1:8730/v1/projects/<id>/inference
# expect "route":"global","effectiveRoute":"global"
curl -s -X PATCH -d '{"route":"au"}' http://127.0.0.1:8730/v1/projects/<id>/inference
# expect "effectiveRoute":"au"
curl -s -X PATCH -d '{"route":"global"}' http://127.0.0.1:8730/v1/projects/<id>/inference
# expect {"error":"needs_compliance",...}
```

The control plane's port `8730` is phase 2's (`api_test.go:26`); the `projects/` subdirectory is gap 3.

**What this task does not claim:**

- **Team-project semantics.** In a team project any member's raise is announced to everyone (`05-design.md:219-222`); that is Fleet's, in team mode. Here the only caller is the person at this PC.
- **A Hub store.** `ProjectInferenceStore` has one implementation. The Hub's (ask H-1, seam S6.1) is phase H's, and `ROUTE_SOURCE=hub` fails start-up rather than pretending.
- **Tightening latency.** A raise is on disk when the PATCH returns. The gatekeeper sees it when task 13's `File` re-reads (modification time) and its `Cache` entry expires, so up to `ROUTE_CACHE_TTL` (30 s) later. Task 23 step 3 observes the whole path.
- **That Docker Desktop shows a host rename inside a container on a directory bind mount promptly**, on Windows in particular. **UNVERIFIED**; task 23 step 3 is the observation.
- **Usage accuracy.** The figures are keysmith's, at most 60 s old (task 09), and US dollars as LiteLLM prices them.
- **That anyone but this control plane writes `projects.json` safely.** The lock is in-process.

**Gaps against the skeleton, for the orchestrator:**

1. **Task 15's client has no `Usage` method.** The skeleton says usage comes "from keysmith's `/v1/usage` through the gateway client", and task 15's scope lists `Mint`, `Refresh`, `WriteToken`, `Refresher`, `RevokeSession` only. It needs `Usage(ctx, scope, id string) ([]wire.UsageRow, error)` on `MINT_TIMEOUT` or shorter, which satisfies `api.UsageReader`.
2. **Task 15 must expose the owner and team it mints for.** The local control plane mints as someone (`MintRequest.Owner`, `TeamID`); the skeleton names no variable for either. `GatewayDeps` takes both from wherever task 15 settles them. The usage "person" id must be `<tid>:<oid>`, the string task 07 passes to `EnsureUser`, and task 09 must key its person rollup the same way.
3. **Task 17 must mount the directory holding `projects.json`, not the file** (decision (h)); this part assumes `deploy/local/gateway/projects/projects.json`. Its checked-in seed must be `{}` or entries in the four-key shape, because this store and task 13's `File` both decode each entry with `dataclass.DecodeFields`. Task 13's `File` should also compare size as well as modification time, since two writes inside one timestamp tick are otherwise one.
4. **No code for "below the floor".** Task 03's list has none; this part uses phase 2's `409 conflict` with `details.floor` (decision (c)). A dedicated `below_floor` (409) in task 03's list would let the cockpit say it without parsing details.
5. **keysmith's caller table has no `usage` route.** Task 08 maps broker → mint, refresh, revoke and gatekeeper → bindings. The local control plane's bearer-file line (task 17's `secrets/callers`) needs `mint refresh revoke usage`, and whoever reads usage in the cloud (the Hub, for task 22's cloud projects) needs a caller entry too.
6. **Task 09 should return a zero-spend current-period row.** A scope with no spend this month has no row here (decision (e)), so the cockpit cannot show its budget. If task 09's `Rows` returns the current period with `spendUsd: 0` and the budget, task 22 can say *"US$0.00 of US$1,000 this month"* instead of *"No spend this month"*.
7. **`cmd/api/main.go`** is modified (step 8) and is not in the skeleton's file map.
