### Task 3: Wire contracts, problem codes, `MessagesURL`, `HashToken`

Slice A, 2 focused hours, depends on task 01. Seams S6.3 (the broker mints through these shapes) and S6.5 (Fleet will too).

This task fixes **every byte that crosses between the gateway's pieces**: what the launcher sends keysmith to mint, what it gets back, what keysmith tells the gatekeeper about a token, what the cockpit reads as usage, and which problem code means what. It is pure Go over the standard library plus task 01, with no I/O, so eleven later parts compile against it and no two of them can disagree on a JSON key. Parallel writers have already written against these names (tasks 06, 07, 11, 13, 17, 21); this part adopts what they assumed wherever the skeleton allowed it, and says where it had to choose.

Three rules are enforced here, not described:

1. **One spelling per field, pinned by a golden.** Every type's JSON is compared to a literal string. JSON keys are camelCase, the Hub contract's convention (`.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md:614-615`) and the one task 17's acceptance script already posts (`task-17.md`, step 7: `sessionId`, `generation`, `owner.tid`, `teamId`, `podAddr`, …). A renamed field is a red test here, not a silent `null` in another process.
2. **A token never appears anywhere but the one field that carries it.** `HashToken` is a one-way hash, `Binding` holds only the hash, and no type's `String` or error text includes a token.
3. **The base URL has no path.** `ANTHROPIC_BASE_URL` is scheme and host only (skeleton, "What Claude Code needs from a gateway"), and every URL our own code builds to `/v1/messages` goes through `MessagesURL`, which refuses a base that already ends in `/v1/messages` (double-pathing) or has any other path, query or credentials. That is the Panacea 405 turned into a function (skeleton, "What Panacea's LiteLLM taught").

Eight decisions the skeleton leaves open, made here so no later part re-derives them:

- **(a) `RefreshRequest.Generation` is the new generation**, so it is at least 2. Task 07 decided this (`task-07.md`, decision (c)): a retried refresh is then exactly a retried mint of the same generation. `Validate` enforces `≥ 2`.
- **(b) `(Binding) LiveAt(now)` lives here**, as task 07's gap 3 asks: live means `now` is before `ExpiresAt` and either `RevokedAt` is nil or `now` is before `*RevokedAt`. A revocation dated in the future is a scheduled one (task 07's refresh grace). keysmith and the gatekeeper (task 10) call this one function and cannot drift.
- **(c) `budget_unknown` (503) joins the code list**, for a budget policy that cannot be read (task 06's gap 1, task 07's gap 2). Failing to read a budget must not look like having none.
- **(d) Addresses are normalised once.** `NormalAddr` unmaps an IPv4-mapped IPv6 address (`::ffff:10.244.1.7` → `10.244.1.7`) and drops any zone. `MintRequest.Validate` refuses a zone, the unspecified address and a mapped form, so a stored `Binding.PodAddr` is always normal, and task 10 compares `NormalAddr(source) == b.PodAddr`. Without this, a dual-stack listener that reports the mapped form would fail every request with `binding_mismatch`.
- **(e) Ids are refused, not escaped.** `SessionID`, `TeamID`, `ProjectID`, `AgentID`, `Owner.TID` and `Owner.OID` must be 1–128 characters of `[A-Za-z0-9._-]`. `SessionID` becomes part of a LiteLLM `key_alias` (`sess/<session>/<generation>`, P6-D6) and `<tid>:<oid>` a LiteLLM `user_id`, so a `/`, a `:` or a space in either would be ambiguous there. `Owner.OID` and `TID` are Entra GUIDs in the cloud and short strings locally (`local`, `dev` in task 17's script); both fit.
- **(f) Codes and statuses live together.** `Status(code)` returns the HTTP status for each of this package's codes, and a test pins the whole table. No wire code equals one of `httpx`'s sixteen (`control-plane/internal/httpx/problem.go:45-60`), so the cockpit's single `switch` on `Problem.Error` never sees one string with two meanings.
- **(g) `(ProjectInference) LiveRoute()` lives here** (re-review R-6). The live route is `Tighter(Decide(DataClass, Route), Effective)`, an error for anything unparsed. keysmith (task 07's `route`, used by task 07 decision (h)'s retire), the gatekeeper (task 10's step 7) and the session refresher (task 15's raise check) each computed it, and the refresher read `Effective` alone. With an AU-only class and a stale Global `Effective`, keysmith retired the key but the refresher saw no raise, which left the session refused for up to 55 minutes. One function, like decision (b), means they cannot drift.
- **(h) `ProjectInference.TeamID` is the project's team, read from the Hub** (review I-7). `MintRequest.TeamID` is the caller's claim, and attribution and the team budget both key on it (task 07's `Budgets.Limits` and `Spent`, task 09's ledger). The Hub's `Project` carries `teamId` (`hub-contract.md:637-639`, never null), and task 13's `Hub` source already reads that record, so it fills `TeamID` and refuses a record without a valid one. keysmith refuses a mint whose `TeamID` differs (task 07 decision (i)). Only a source with no teams leaves it `""`: task 13's `File`, local mode's `projects.json`, where the caller's team stands.

**Files:**
- `control-plane/internal/inference/wire/wire.go` — every type, interface, sentinel, code and helper below (new)
- `control-plane/internal/inference/wire/wire_test.go` — golden JSON, `Validate`, `LiveAt`, `LiveRoute`, `MessagesURL`, `HashToken`, the code table (new)

**Interfaces:**
- Produces:
  - `wire.Owner{TID, OID string}` (JSON `tid`, `oid`) with `(Owner) UserID() string` = `"<tid>:<oid>"` (LiteLLM's `user_id`, task 07)
  - `wire.SessionClass` with `ClassSolo` (`"solo"`), `ClassWorker` (`"worker"`), `ClassOrchestrator` (`"orchestrator"`)
  - `wire.MintRequest{SessionID string; Generation int; Owner Owner; TeamID, ProjectID, AgentID string; SessionClass SessionClass; PodAddr string}` with `Validate() error` and `Addr() (netip.Addr, error)`
  - `wire.MintResponse{Token, KeyID string; ExpiresAt time.Time; Route dataclass.Route; BaseURL string; MaxBudgetUSD float64}`
  - `wire.RefreshRequest{SessionID string; Generation int}` with `Validate()`; `wire.RevokeRequest{SessionID string}` with `Validate()`
  - `wire.Binding{KeyHash, KeyID, SessionID string; Generation int; Owner Owner; TeamID, ProjectID, AgentID string; PodAddr netip.Addr; RouteAtMint dataclass.Route; CreatedAt, ExpiresAt time.Time; RevokedAt *time.Time}` with `LiveAt(time.Time) bool`
  - `wire.UsageRow{Scope, ID string; PeriodStart time.Time; SpendUSD, BudgetUSD float64; Requests int64}` and `ScopePerson`, `ScopeProject`, `ScopeTeam`
  - `wire.ProjectInference{ProjectID, TeamID string; DataClass dataclass.Class; Route, Effective dataclass.Route; Version string}` with `LiveRoute() (dataclass.Route, error)` (decision (g)); `TeamID` is the project's team as the source read it (decision (h))
  - `wire.BindingReader`, `wire.RouteSource`; sentinels `wire.ErrNotFound`, `wire.ErrRouteUnknown`
  - the thirteen `wire.Code…` constants, `wire.Status(code) (int, bool)`, `wire.Codes() []string`
  - `wire.HashToken(string) string`, `wire.CheckBaseURL(string) error`, `wire.MessagesURL(string) (string, error)`, `wire.NormalAddr(netip.Addr) netip.Addr`
- Consumes: `dataclass.Class`, `dataclass.Route`, `RouteGlobal`, `RouteAU` (task 01); the standard library.
- Required by: tasks 05, 06, 07, 08, 09, 10, 11, 12, 13, 15, 21, and task 22's TypeScript mirror of `UsageRow` and `ProjectInference`.

- [ ] **Step 1: Write the failing tests**

Every test names, in its comment, the mutant it kills. The file is `package wire` (internal), so the first failure is a bare `undefined: MintRequest`.

Create `control-plane/internal/inference/wire/wire_test.go`:

```go
package wire

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

var (
	t0      = time.Date(2026, 9, 25, 1, 0, 0, 0, time.UTC)
	t1      = t0.Add(time.Hour)
	revoked = t0.Add(30 * time.Minute)
	own     = Owner{TID: "t-1", OID: "o-1"}
	okMint  = MintRequest{SessionID: "s-1", Generation: 1, Owner: own, TeamID: "team-1",
		ProjectID: "p-1", AgentID: "a-1", SessionClass: ClassSolo, PodAddr: "10.244.1.7"}
)

// Kills: a renamed or re-cased JSON key, which a parallel part (or the broker,
// in another process) would read as a missing field; and a decoder that
// accepts the golden but loses a field on the way back.
func TestJSONKeysAreGolden(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want string
	}{
		{"MintRequest", okMint,
			`{"sessionId":"s-1","generation":1,"owner":{"tid":"t-1","oid":"o-1"},"teamId":"team-1","projectId":"p-1","agentId":"a-1","sessionClass":"solo","podAddr":"10.244.1.7"}`},
		{"MintResponse", MintResponse{Token: "sk-pa-x", KeyID: "k-1", ExpiresAt: t1, Route: dataclass.RouteAU, BaseURL: "http://gatekeeper:4000", MaxBudgetUSD: 25},
			`{"token":"sk-pa-x","keyId":"k-1","expiresAt":"2026-09-25T02:00:00Z","route":"au","baseUrl":"http://gatekeeper:4000","maxBudgetUsd":25}`},
		{"RefreshRequest", RefreshRequest{SessionID: "s-1", Generation: 2}, `{"sessionId":"s-1","generation":2}`},
		{"RevokeRequest", RevokeRequest{SessionID: "s-1"}, `{"sessionId":"s-1"}`},
		{"Binding", Binding{KeyHash: "h-1", KeyID: "h-1", SessionID: "s-1", Generation: 1, Owner: own, TeamID: "team-1",
			ProjectID: "p-1", AgentID: "a-1", PodAddr: netip.MustParseAddr("fd00::7"), RouteAtMint: dataclass.RouteGlobal,
			CreatedAt: t0, ExpiresAt: t1, RevokedAt: &revoked},
			`{"keyHash":"h-1","keyId":"h-1","sessionId":"s-1","generation":1,"owner":{"tid":"t-1","oid":"o-1"},"teamId":"team-1","projectId":"p-1","agentId":"a-1","podAddr":"fd00::7","routeAtMint":"global","createdAt":"2026-09-25T01:00:00Z","expiresAt":"2026-09-25T02:00:00Z","revokedAt":"2026-09-25T01:30:00Z"}`},
		{"UsageRow", UsageRow{Scope: ScopeTeam, ID: "team-1", PeriodStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), SpendUSD: 212.4, BudgetUSD: 2000, Requests: 31},
			`{"scope":"team","id":"team-1","periodStart":"2026-09-01T00:00:00Z","spendUsd":212.4,"budgetUsd":2000,"requests":31}`},
		{"ProjectInference", ProjectInference{ProjectID: "p-1", TeamID: "team-1", DataClass: dataclass.Class{Level: dataclass.LevelConfidential, Residency: dataclass.ResidencyAUOnly},
			Route: dataclass.RouteGlobal, Effective: dataclass.RouteAU, Version: "0123456789abcdef"},
			`{"projectId":"p-1","teamId":"team-1","dataClass":{"level":"confidential","residency":"au-only"},"route":"global","effective":"au","version":"0123456789abcdef"}`},
	}
	for _, tc := range cases {
		got, err := json.Marshal(tc.v)
		if err != nil || string(got) != tc.want {
			t.Errorf("%s:\n got %s (%v)\nwant %s", tc.name, got, err, tc.want)
			continue
		}
		back := reflect.New(reflect.TypeOf(tc.v))
		dec := json.NewDecoder(strings.NewReader(tc.want))
		dec.DisallowUnknownFields()
		if err := dec.Decode(back.Interface()); err != nil {
			t.Errorf("%s: golden does not decode strictly: %v", tc.name, err)
			continue
		}
		if again, _ := json.Marshal(back.Elem().Interface()); !bytes.Equal(again, got) {
			t.Errorf("%s: round trip changed it:\n%s\n%s", tc.name, again, got)
		}
	}
}

// Kills: omitempty on RevokedAt. The Hub's rule is null, never omitted, and
// the gatekeeper must be able to tell "not revoked" from "field missing".
func TestAnUnrevokedBindingSaysNull(t *testing.T) {
	b, _ := json.Marshal(Binding{PodAddr: netip.MustParseAddr("10.0.0.1")})
	if !bytes.Contains(b, []byte(`"revokedAt":null`)) {
		t.Fatalf("got %s", b)
	}
}
```

Then the validation, liveness and address rules, in the same file:

```go
// Kills: accepting generation 0 (the idempotency backbone starts at 1), an
// empty project (a key nobody's budget covers), an unparsable or non-normal
// pod address (a binding no request could ever match), and an id that would
// be ambiguous inside a LiteLLM key_alias or user_id.
func TestMintRequestValidate(t *testing.T) {
	if err := okMint.Validate(); err != nil {
		t.Fatalf("the good request: %v", err)
	}
	v6 := okMint
	v6.PodAddr = "fd00::7"
	if err := v6.Validate(); err != nil {
		t.Fatalf("an IPv6 pod: %v", err)
	}
	bad := map[string]func(*MintRequest){
		"generation 0":     func(r *MintRequest) { r.Generation = 0 },
		"generation -1":    func(r *MintRequest) { r.Generation = -1 },
		"no project":       func(r *MintRequest) { r.ProjectID = "" },
		"no session":       func(r *MintRequest) { r.SessionID = "" },
		"no team":          func(r *MintRequest) { r.TeamID = "" },
		"no agent":         func(r *MintRequest) { r.AgentID = "" },
		"no oid":           func(r *MintRequest) { r.Owner.OID = "" },
		"slash in session": func(r *MintRequest) { r.SessionID = "s/1" },
		"space in agent":   func(r *MintRequest) { r.AgentID = "a 1" },
		"129-char tid":     func(r *MintRequest) { r.Owner.TID = strings.Repeat("t", 129) },
		"unknown class":    func(r *MintRequest) { r.SessionClass = "admin" },
		"hostname":         func(r *MintRequest) { r.PodAddr = "pod-7" },
		"with port":        func(r *MintRequest) { r.PodAddr = "10.244.1.7:4000" },
		"unspecified v4":   func(r *MintRequest) { r.PodAddr = "0.0.0.0" },
		"unspecified v6":   func(r *MintRequest) { r.PodAddr = "::" },
		"v4-mapped":        func(r *MintRequest) { r.PodAddr = "::ffff:10.244.1.7" },
		"zoned":            func(r *MintRequest) { r.PodAddr = "fe80::1%eth0" },
		"empty addr":       func(r *MintRequest) { r.PodAddr = "" },
	}
	for name, edit := range bad {
		r := okMint
		edit(&r)
		if err := r.Validate(); err == nil {
			t.Errorf("%s: Validate accepted it", name)
		}
	}
}

// Kills: a refresh that re-mints generation 1 (it is always the new
// generation, task 07), and an empty revoke that would match every session.
func TestRefreshAndRevokeValidate(t *testing.T) {
	if (RefreshRequest{SessionID: "s-1", Generation: 1}).Validate() == nil {
		t.Error("refresh to generation 1 accepted")
	}
	if err := (RefreshRequest{SessionID: "s-1", Generation: 2}).Validate(); err != nil {
		t.Errorf("refresh to generation 2: %v", err)
	}
	if (RevokeRequest{}).Validate() == nil {
		t.Error("an empty revoke accepted")
	}
}

// Kills: `<=` at expiry, treating any RevokedAt as dead (which would cut every
// stream in flight at refresh, task 07's grace), and ignoring RevokedAt.
func TestLiveAt(t *testing.T) {
	past, future := t0.Add(-time.Minute), t0.Add(5*time.Minute)
	cases := []struct {
		name    string
		revoked *time.Time
		at      time.Time
		want    bool
	}{
		{"fresh", nil, t0, true},
		{"at expiry", nil, t1, false},
		{"after expiry", nil, t1.Add(time.Second), false},
		{"revoked earlier", &past, t0, false},
		{"revocation scheduled", &future, t0, true},
		{"at the scheduled time", &future, future, false},
	}
	for _, tc := range cases {
		b := Binding{ExpiresAt: t1, RevokedAt: tc.revoked}
		if got := b.LiveAt(tc.at); got != tc.want {
			t.Errorf("%s: LiveAt = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Decision (g), re-review R-6. Kills reading Effective alone (the stale row),
// dropping the Effective tightening (the source row), and an unparsed
// class or route read as Global.
func TestLiveRoute(t *testing.T) {
	anyRes := dataclass.Class{Level: dataclass.LevelInternal, Residency: dataclass.ResidencyAny}
	auOnly := dataclass.Class{Level: dataclass.LevelInternal, Residency: dataclass.ResidencyAUOnly}
	g, au := dataclass.RouteGlobal, dataclass.RouteAU
	cases := []struct {
		name string
		pi   ProjectInference
		want dataclass.Route // "" means an error
	}{
		{"global", ProjectInference{DataClass: anyRes, Route: g, Effective: g}, g},
		{"chosen au", ProjectInference{DataClass: anyRes, Route: au, Effective: au}, au},
		{"stale effective", ProjectInference{DataClass: auOnly, Route: g, Effective: g}, au},
		{"source tightened", ProjectInference{DataClass: anyRes, Route: g, Effective: au}, au},
		{"no class", ProjectInference{Route: g, Effective: g}, ""},
		{"unknown effective", ProjectInference{DataClass: anyRes, Route: g, Effective: "mars"}, ""},
	}
	for _, tc := range cases {
		got, err := tc.pi.LiveRoute()
		if got != tc.want || (err == nil) != (tc.want != "") {
			t.Errorf("%s: LiveRoute = %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
}

// Kills: comparing a mapped or zoned source address to a stored normal one.
func TestNormalAddrAndAddr(t *testing.T) {
	for in, want := range map[string]string{"::ffff:10.244.1.7": "10.244.1.7", "fe80::1%eth0": "fe80::1", "fd00::7": "fd00::7"} {
		if got := NormalAddr(netip.MustParseAddr(in)).String(); got != want {
			t.Errorf("NormalAddr(%s) = %s, want %s", in, got, want)
		}
	}
	if a, err := okMint.Addr(); err != nil || a != netip.MustParseAddr("10.244.1.7") {
		t.Errorf("Addr = %v, %v", a, err)
	}
	if own.UserID() != "t-1:o-1" {
		t.Errorf("UserID = %q", own.UserID())
	}
}
```

And the URL, hash and code rules:

```go
// Kills: double-pathing (a base that already ends in /v1/messages), the
// Panacea bare-concatenation habit (a base with its own path), and a base
// carrying a query or credentials that would then ride on every request.
func TestMessagesURL(t *testing.T) {
	ok := map[string]string{
		"http://gatekeeper:4000":      "http://gatekeeper:4000/v1/messages",
		"http://gatekeeper:4000/":     "http://gatekeeper:4000/v1/messages",
		"https://gw.example.internal": "https://gw.example.internal/v1/messages",
		"http://[fd00::7]:4000":       "http://[fd00::7]:4000/v1/messages",
	}
	for in, want := range ok {
		if got, err := MessagesURL(in); err != nil || got != want {
			t.Errorf("MessagesURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{
		"http://gatekeeper:4000/v1/messages", "http://gatekeeper:4000/v1/messages/",
		"http://gatekeeper:4000/v1", "http://gatekeeper:4000/api",
		"http://gatekeeper:4000?beta=true", "http://gatekeeper:4000#x",
		"http://user:pass@gatekeeper:4000", "gatekeeper:4000", "ftp://gatekeeper",
		"http://", "",
	} {
		if got, err := MessagesURL(in); err == nil {
			t.Errorf("MessagesURL(%q) = %q, want an error", in, got)
		}
	}
}

// Kills: an identity or reversible "hash", a different algorithm (LiteLLM names
// a key by the SHA-256 of its value, task 07 decision (f)), and upper case.
func TestHashToken(t *testing.T) {
	const tok = "sk-pa-test"
	h := HashToken(tok)
	if h != "03a76851a40cfb5230a883b0ddc58205db8c9dff2fd77e38d87b0406cec826be" {
		t.Fatalf("HashToken = %s", h)
	}
	if strings.Contains(h, tok) || strings.Contains(h, "test") || HashToken(tok+"x") == h {
		t.Fatal("the hash carries or ignores its input")
	}
}

// Kills: a code with the wrong status (a 401 answered as 403 sends the CLI's
// helper to the wrong recovery), a code added without a status, and a wire code
// that collides with an httpx code of another meaning.
func TestCodesAndStatuses(t *testing.T) {
	want := map[string]int{
		CodeUnauthenticated: 401, CodeCredentialConflict: 401, CodeTokenExpired: 401, CodeTokenRevoked: 401,
		CodeBindingMismatch: 403, CodeCallerForbidden: 403, CodeNeedsCompliance: 403,
		CodeModelNotOffered: 400, CodePathNotAllowed: 404, CodeBudgetExhausted: 402,
		CodeRouteUnknown: 503, CodeBudgetUnknown: 503, CodeMintTimeout: 504,
	}
	if got := Codes(); len(got) != len(want) {
		t.Fatalf("Codes() has %d, want %d: %v", len(got), len(want), got)
	}
	for _, c := range Codes() {
		if s, ok := Status(c); !ok || s != want[c] {
			t.Errorf("Status(%q) = %d, %v; want %d", c, s, ok, want[c])
		}
	}
	if _, ok := Status("nope"); ok {
		t.Error("an unknown code has a status")
	}
	for _, h := range []string{httpx.CodeInvalidJSON, httpx.CodeBadRequest, httpx.CodeUnknownHarness,
		httpx.CodeUnsupportedOption, httpx.CodeForbiddenOrigin, httpx.CodeForbiddenHost, httpx.CodeProjectNotFound,
		httpx.CodeWorktreeNotFound, httpx.CodeSessionNotFound, httpx.CodeConflict, httpx.CodeSandboxNotRunning,
		httpx.CodeHarnessUnavailable, httpx.CodeUpgradeRequired, httpx.CodeInternal, httpx.CodeNonLoopbackPublish,
		httpx.CodeBackendUnavailable} {
		if _, clash := want[h]; clash {
			t.Errorf("wire code %q is already an httpx code", h)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/inference/wire/... -count=1`

Expected: the build fails, because nothing in the package exists yet:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire [github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire.test]
control-plane/internal/inference/wire/wire_test.go:20:12: undefined: Owner
control-plane/internal/inference/wire/wire_test.go:21:12: undefined: MintRequest
...
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire [build failed]
```

If the first error names `dataclass.LevelConfidential` or `dataclass.RouteAU`, task 01 is not merged: stop. The line numbers are indicative.

- [ ] **Step 3: Write the package**

Create `control-plane/internal/inference/wire/wire.go`. It is written here in three fenced blocks, one file.

```go
// Package wire is every shape that crosses between the gateway's pieces:
// launcher → keysmith (mint, refresh, revoke), keysmith → gatekeeper (the
// binding of a token), keysmith → cockpit (usage), and the project route every
// piece reads. JSON keys are camelCase and pinned by golden tests; a token
// appears only in MintResponse.Token and never in an error, a log or a hash.
package wire

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
)

// Owner is the person a session acts for: an Entra tenant and object id in
// the cloud, short local strings in local mode.
type Owner struct {
	TID string `json:"tid"`
	OID string `json:"oid"`
}

// UserID is the owner's LiteLLM user_id (the person's budget, P6-D8).
func (o Owner) UserID() string { return o.TID + ":" + o.OID }

// SessionClass is the kind of session a key is minted for (05-design §2.5).
type SessionClass string

const (
	ClassSolo         SessionClass = "solo"
	ClassWorker       SessionClass = "worker"
	ClassOrchestrator SessionClass = "orchestrator"
)

const maxIDLen = 128

// checkID admits 1-128 characters of [A-Za-z0-9._-]. A "/" would be ambiguous
// in a key_alias ("sess/<session>/<generation>") and a ":" in a user_id.
func checkID(field, v string) error {
	if v == "" || len(v) > maxIDLen {
		return fmt.Errorf("%s must be 1-%d characters", field, maxIDLen)
	}
	for i := 0; i < len(v); i++ {
		switch c := v[i]; {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '.', c == '_', c == '-':
		default:
			return fmt.Errorf("%s holds %q; only [A-Za-z0-9._-] is allowed", field, c)
		}
	}
	return nil
}

// MintRequest asks keysmith for one session generation's token (POST /v1/mint).
type MintRequest struct {
	SessionID    string       `json:"sessionId"`
	Generation   int          `json:"generation"` // 1 on create; a retry repeats it
	Owner        Owner        `json:"owner"`
	TeamID       string       `json:"teamId"`
	ProjectID    string       `json:"projectId"`
	AgentID      string       `json:"agentId"`
	SessionClass SessionClass `json:"sessionClass"`
	PodAddr      string       `json:"podAddr"` // the address the gatekeeper will see
}

// Validate names the first field that is missing or unusable.
func (r MintRequest) Validate() error {
	for _, f := range []struct{ name, v string }{
		{"sessionId", r.SessionID}, {"owner.tid", r.Owner.TID}, {"owner.oid", r.Owner.OID},
		{"teamId", r.TeamID}, {"projectId", r.ProjectID}, {"agentId", r.AgentID},
	} {
		if err := checkID(f.name, f.v); err != nil {
			return err
		}
	}
	if r.Generation < 1 {
		return errors.New("generation must be 1 or more")
	}
	switch r.SessionClass {
	case ClassSolo, ClassWorker, ClassOrchestrator:
	default:
		return fmt.Errorf("sessionClass %q is not solo, worker or orchestrator", r.SessionClass)
	}
	_, err := r.Addr()
	return err
}

// Addr parses PodAddr and accepts only a normal address (no zone, not
// IPv4-mapped, not unspecified), so a stored binding compares with ==.
func (r MintRequest) Addr() (netip.Addr, error) {
	a, err := netip.ParseAddr(r.PodAddr)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("podAddr %q is not an IP address", r.PodAddr)
	}
	if a != NormalAddr(a) || a.IsUnspecified() {
		return netip.Addr{}, fmt.Errorf("podAddr %q must be a plain address: no zone, not IPv4-mapped, not unspecified", r.PodAddr)
	}
	return a, nil
}

// NormalAddr is the one form addresses are compared in: unmapped, unzoned.
func NormalAddr(a netip.Addr) netip.Addr { return a.Unmap().WithZone("") }
```

```go
// MintResponse is keysmith's 201 to a mint or a refresh. Token is the only
// field anywhere in this package that carries a secret.
type MintResponse struct {
	Token        string          `json:"token"`
	KeyID        string          `json:"keyId"` // HashToken(Token), task 07 decision (f)
	ExpiresAt    time.Time       `json:"expiresAt"`
	Route        dataclass.Route `json:"route"` // the route at mint
	BaseURL      string          `json:"baseUrl"`
	MaxBudgetUSD float64         `json:"maxBudgetUsd"` // the key's slice (P6-D8)
}

// RefreshRequest mints the NEW generation (≥ 2) of a session that holds the
// one before it, so a retried refresh is a retried mint (task 07).
type RefreshRequest struct {
	SessionID  string `json:"sessionId"`
	Generation int    `json:"generation"`
}

func (r RefreshRequest) Validate() error {
	if err := checkID("sessionId", r.SessionID); err != nil {
		return err
	}
	if r.Generation < 2 {
		return errors.New("generation is the new generation and must be 2 or more")
	}
	return nil
}

// RevokeRequest revokes every generation of a session.
type RevokeRequest struct {
	SessionID string `json:"sessionId"`
}

func (r RevokeRequest) Validate() error { return checkID("sessionId", r.SessionID) }

// Binding is what keysmith records about one token (P6-D7), and what the
// gatekeeper reads through GET /v1/bindings/{keyHash}. It never holds the token.
type Binding struct {
	KeyHash     string          `json:"keyHash"`
	KeyID       string          `json:"keyId"`
	SessionID   string          `json:"sessionId"`
	Generation  int             `json:"generation"`
	Owner       Owner           `json:"owner"`
	TeamID      string          `json:"teamId"`
	ProjectID   string          `json:"projectId"`
	AgentID     string          `json:"agentId"`
	PodAddr     netip.Addr      `json:"podAddr"` // always NormalAddr form
	RouteAtMint dataclass.Route `json:"routeAtMint"`
	CreatedAt   time.Time       `json:"createdAt"`
	ExpiresAt   time.Time       `json:"expiresAt"`
	RevokedAt   *time.Time      `json:"revokedAt"` // null, never omitted; may be in the future
}

// LiveAt reports whether the binding admits a request at now: before expiry,
// and before its revocation time if it has one. A future RevokedAt is a
// scheduled revocation (a refresh's grace), so the old key works until then.
func (b Binding) LiveAt(now time.Time) bool {
	return now.Before(b.ExpiresAt) && (b.RevokedAt == nil || now.Before(*b.RevokedAt))
}

// The three usage scopes; task 06's budget.Dimension strings equal these.
const (
	ScopePerson  = "person"
	ScopeProject = "project"
	ScopeTeam    = "team"
)

// UsageRow is one scope's spend in one UTC calendar month (GET /v1/usage).
// US dollars, as LiteLLM records them.
type UsageRow struct {
	Scope       string    `json:"scope"`
	ID          string    `json:"id"` // "<tid>:<oid>", the project id, or the team id
	PeriodStart time.Time `json:"periodStart"`
	SpendUSD    float64   `json:"spendUsd"`
	BudgetUSD   float64   `json:"budgetUsd"`
	Requests    int64     `json:"requests"`
}

// ProjectInference is one project's route as a RouteSource reports it.
// Effective is always dataclass.Decide(DataClass, Route), computed by the
// source (task 13), never read from a record.
type ProjectInference struct {
	ProjectID string `json:"projectId"`
	// TeamID is the project's team from the Hub record, never the caller's
	// claim; "" only from a source with no teams (local mode). Decision (h).
	TeamID    string          `json:"teamId"`
	DataClass dataclass.Class `json:"dataClass"`
	Route     dataclass.Route `json:"route"`
	Effective dataclass.Route `json:"effective"`
	Version   string          `json:"version"` // changes when any field above changes
}

// LiveRoute is the route the project runs on now: the class's floor and the
// chosen route through dataclass.Decide, tightened by the source's Effective,
// so a stale Effective can never loosen an AU-only class (decision (g)).
// keysmith, the gatekeeper and the session refresher all call it. Anything
// unparsed is an error, never Global (P6-D5).
func (pi ProjectInference) LiveRoute() (dataclass.Route, error) {
	r, err := dataclass.Decide(pi.DataClass, pi.Route)
	if err != nil {
		return "", err
	}
	if r = dataclass.Tighter(r, pi.Effective); !r.Valid() {
		return "", fmt.Errorf("effective route %q", pi.Effective)
	}
	return r, nil
}

// BindingReader finds a token's binding by HashToken. A miss is ErrNotFound.
type BindingReader interface {
	ByKeyHash(ctx context.Context, keyHash string) (Binding, error)
}

// RouteSource reports a project's live route. Every failure, including an
// unknown project, satisfies errors.Is(err, ErrRouteUnknown) (P6-D5).
type RouteSource interface {
	Project(ctx context.Context, projectID string) (ProjectInference, error)
}

var (
	ErrNotFound     = errors.New("wire: not found")
	ErrRouteUnknown = errors.New("wire: project route unknown")
)
```

```go
// The gateway's problem codes, written with httpx.WriteProblem (the body is
// httpx.Problem, control-plane/internal/httpx/problem.go:93). None equals an
// httpx code; Status gives each one's HTTP status.
const (
	CodeUnauthenticated    = "unauthenticated"     // 401 no credential, or an unknown one
	CodeCredentialConflict = "credential_conflict" // 401 Authorization and x-api-key differ
	CodeTokenExpired       = "token_expired"       // 401 past ExpiresAt
	CodeTokenRevoked       = "token_revoked"       // 401 at or past RevokedAt
	CodeBindingMismatch    = "binding_mismatch"    // 403 source address is not the bound pod
	CodeCallerForbidden    = "caller_forbidden"    // 403 keysmith caller not allowed this route
	CodeNeedsCompliance    = "needs_compliance"    // 403 a loosening by neither compliance nor a solo owner
	CodeModelNotOffered    = "model_not_offered"   // 400 model is not a public catalogue id
	CodePathNotAllowed     = "path_not_allowed"    // 404 not /v1/messages or /v1/messages/count_tokens
	CodeBudgetExhausted    = "budget_exhausted"    // 402 a dimension has nothing left (details: dimension)
	CodeRouteUnknown       = "route_unknown"       // 503 the project's route could not be established
	CodeBudgetUnknown      = "budget_unknown"      // 503 the budget policy could not be read
	CodeMintTimeout        = "mint_timeout"        // 504 the mint outran MINT_TIMEOUT; it still completes
)

var statuses = map[string]int{
	CodeUnauthenticated: http.StatusUnauthorized, CodeCredentialConflict: http.StatusUnauthorized,
	CodeTokenExpired: http.StatusUnauthorized, CodeTokenRevoked: http.StatusUnauthorized,
	CodeBindingMismatch: http.StatusForbidden, CodeCallerForbidden: http.StatusForbidden,
	CodeNeedsCompliance: http.StatusForbidden, CodeModelNotOffered: http.StatusBadRequest,
	CodePathNotAllowed: http.StatusNotFound, CodeBudgetExhausted: http.StatusPaymentRequired,
	CodeRouteUnknown: http.StatusServiceUnavailable, CodeBudgetUnknown: http.StatusServiceUnavailable,
	CodeMintTimeout: http.StatusGatewayTimeout,
}

// Status is the HTTP status for a wire code; ok is false for any other string.
func Status(code string) (status int, ok bool) {
	status, ok = statuses[code]
	return status, ok
}

// Codes lists every wire code, sorted.
func Codes() []string {
	out := make([]string, 0, len(statuses))
	for c := range statuses {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// HashToken is the lowercase hex SHA-256 of a token: the binding's key and
// the KeyID LiteLLM is addressed by. Tokens are 32 random bytes (task 07), so
// an unsalted hash leaves nothing to guess.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

const messagesPath = "/v1/messages"

// CheckBaseURL accepts what ANTHROPIC_BASE_URL may be: http(s), a host, an
// optional port and at most a trailing "/"; no path, query, fragment or
// credentials. A base that already ends in /v1/messages is named as such.
func CheckBaseURL(base string) error {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("base URL %q is not http(s)://host[:port]", base)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery {
		return fmt.Errorf("base URL %q carries credentials, a query or a fragment", base)
	}
	switch p := u.Path; {
	case p == "" || p == "/":
		return nil
	case p == messagesPath || p == messagesPath+"/":
		return fmt.Errorf("base URL %q already ends in %s; give the host only", base, messagesPath)
	default:
		return fmt.Errorf("base URL %q has a path; give the host only", base)
	}
}

// MessagesURL is the one way our code builds a Messages API URL.
func MessagesURL(base string) (string, error) {
	if err := CheckBaseURL(base); err != nil {
		return "", err
	}
	u, _ := url.Parse(base)
	u.Path = messagesPath
	return u.String(), nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
go test ./control-plane/internal/inference/wire/... -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Expected:

```
--- PASS: TestJSONKeysAreGolden (0.00s)
--- PASS: TestAnUnrevokedBindingSaysNull (0.00s)
--- PASS: TestMintRequestValidate (0.00s)
--- PASS: TestRefreshAndRevokeValidate (0.00s)
--- PASS: TestLiveAt (0.00s)
--- PASS: TestLiveRoute (0.00s)
--- PASS: TestNormalAddrAndAddr (0.00s)
--- PASS: TestMessagesURL (0.00s)
--- PASS: TestHashToken (0.00s)
--- PASS: TestCodesAndStatuses (0.00s)
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire
```

- [ ] **Step 5: Mutation spot-check, by hand, then revert**

| Edit | Test that must fail |
|---|---|
| `Binding.PodAddr` tag `podAddr` → `podAddress` | `TestJSONKeysAreGolden` |
| `RevokedAt` tag gains `,omitempty` | `TestAnUnrevokedBindingSaysNull` |
| `LiveAt`: `now.Before(*b.RevokedAt)` → `false` | `TestLiveAt` (`revocation scheduled`) |
| `LiveRoute`: its first line → `return pi.Effective, nil` | `TestLiveRoute` (`stale effective`, `no class`) |
| `LiveRoute`: `dataclass.Tighter(r, pi.Effective)` → `r` | `TestLiveRoute` (`source tightened`, `unknown effective`) |
| `Addr`: delete the `a != NormalAddr(a)` condition | `TestMintRequestValidate` (`v4-mapped`, `zoned`) |
| `CheckBaseURL`: the `default` case returns `nil` | `TestMessagesURL` (`/v1`, `/api`) |
| `statuses`: `CodeTokenExpired` → `http.StatusForbidden` | `TestCodesAndStatuses` |
| `HashToken`: `sha256.Sum256` → `sha256.Sum224` (with `sum[:]`) | `TestHashToken` |

- [ ] **Step 6: Format, vet and confirm the import set**

```bash
cd control-plane && gofmt -l internal/inference/wire && go vet ./internal/inference/wire/...
go list -deps ./internal/inference/wire | grep kpmg-au-emu
```

Expected: nothing from `gofmt` and `go vet`, then exactly two lines (the test's `httpx` import is test-only and not listed):

```
github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass
github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire
```

- [ ] **Step 7: Commit**

```bash
git add control-plane/internal/inference/wire/wire.go control-plane/internal/inference/wire/wire_test.go
git commit -F /tmp/task-03-msg.txt -- control-plane/internal/inference/wire
```

with `/tmp/task-03-msg.txt`:

```
feat(inference): the gateway's wire contracts

Every shape between launcher, keysmith, gatekeeper and cockpit, with
camelCase JSON pinned by goldens: mint, refresh (the new generation),
revoke, the token's binding (with LiveAt, which treats a future
revocation as scheduled), usage rows and the project route with its
team as the Hub records it. Pod
addresses are normalised once so a binding compares with ==.

Thirteen problem codes with their statuses, none shared with httpx.
HashToken is hex SHA-256; MessagesURL refuses a base that already ends
in /v1/messages or has any path, query or credentials.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

**Acceptance:** step 4's eleven lines; each row of step 5 turns its test red and reverting turns it green; step 6 prints nothing and then the two lines.

**What this task does not claim:**

- **Transport or authentication.** Who may call `POST /v1/mint`, and how, is task 08; how the gatekeeper reaches keysmith's `GET /v1/bindings/{keyHash}` is task 12. These are the bodies only.
- **That LiteLLM addresses a key by the SHA-256 of its value.** `KeyID = HashToken(token)` is task 07's decision (f), on training knowledge; task 18's P11 and P12 measure it.
- **That Claude Code treats `402 budget_exhausted` as terminal rather than retrying.** The mint path's `402` reaches a launcher, not the CLI; a budget exhausted mid-key is LiteLLM's own error, relayed by task 11. How the CLI shows either is task 23's item (5).
- **That the broker (ask P8-1) or Fleet (seam S6.5) speak these shapes.** They are fixed here so those callers are written against them; neither exists yet.
- **That the id character set fits every Hub id.** The Hub contract's id formats were not re-read for this part; a Hub id outside `[A-Za-z0-9._-]` would fail `Validate` loudly, not silently.

**Gaps against the skeleton, for the orchestrator:**

1. **`budget_unknown` (503) is added** to the skeleton's twelve codes (decision (c)), as tasks 06 and 07 ask. The skeleton's list has twelve; this package has thirteen.
2. **`(Binding) LiveAt` is adopted** from task 07's gap 3. Task 07's local `liveAt` and task 10 should call `b.LiveAt(now)`; the rule is identical.
3. **`NormalAddr`, `CheckBaseURL`, `Owner.UserID`, `MintRequest.Addr`, `Status`, `Codes` and `SessionClass` constants** are additions. Task 10 compares `wire.NormalAddr(source) == b.PodAddr`; task 07's `BaseURL` check can become `wire.CheckBaseURL`; task 07's `EnsureUser` id is `Owner.UserID()`.
4. **`RefreshRequest.Generation` is the new generation** (task 07's reading); the skeleton's `RefreshRequest{SessionID, Generation}` did not say which.
5. **`budget_exhausted` is `402`**, the status task 07 already assumed; the skeleton gave no status.
6. **`(ProjectInference) LiveRoute` is an addition** (decision (g), re-review R-6). Task 07's `route`, task 10's `route` and task 15's `Prepare` and `sleepUnlessRaised` call it; each part says so.
7. **`ProjectInference.TeamID` is an addition** (decision (h), review I-7). Task 13's `Hub` source fills it from the record's `teamId` and its `File` source leaves it `""`; task 07's `mint` refuses a `MintRequest.TeamID` that differs (decision (i) there). The skeleton's `ProjectInference` had no team.
