package api_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/api"
)

// This file is contract §10a, checked by a machine — Group C ruling 1. It is its
// own file because it is the one tasks 11 and 12 must edit, and a name they can
// find beats a block halfway down api_test.go.

// catalogReadOnlyRoutes is the explicit allowlist: every GET and HEAD pattern
// this package registers must appear here, beside the reason it is safe to serve
// with no CSRF defence at all.
//
// Contract §10a is the rule and task 9's origin.go is the reason. That middleware
// allows a request carrying NO Origin header — curl and every non-browser tool
// send none — and a browser sets Host itself, so a hostile page's
// <img src="http://127.0.0.1:8730/…"> satisfies both of the control plane's
// checks without ever seeing the response. Seeing the response is not what a
// side effect needs. There is no token on this hop before Phase 8.
//
// ADDING A READ-ONLY ROUTE MEANS ADDING A LINE HERE. Tasks 11 and 12 extend this
// map; they do not touch catalogAuditRoutes. Adding a side-effecting GET means a
// failing test that names the rule before it can be made to pass.
var catalogReadOnlyRoutes = map[string]string{
	"GET /v1/product/projects/{id}/assist/repository/requests/{requestId}": "Reads the authenticated actor's exact retained request in an owned project. No repair, source sampling, provider resolution, sandbox probes or allocation occurs; a missing receipt does not prove a delayed POST is absent.",
	"GET /v1/product/projects/{id}/assist":                                 "Lists at most twenty bounded private job metadata records for an owned project. No inference, sandbox probes, recovery or project writes occur on GET.",
	"GET /v1/product/projects/{id}/assist/{jobId}":                         "Reads a bounded private job snapshot. No model calls, sandbox probes, recovery or project writes occur on GET.",
	"GET /v1/setup/agents/codex/account":                                   "Reads private connection metadata only; no CLI, credential export, reconciliation or writes.",
	"GET /v1/healthz": "Reads Deps.BackendReady, which task 13 keeps as a CACHED bool. " +
		"This is the one route mounted outside the origin guard, so a hostile page reaches it " +
		"with no checks whatsoever; a healthz that refreshed a probe on request would be a " +
		"side-effecting GET on the least defended route on the surface (ruling 2).",
	"GET /api/v1/healthz":                    "Contract §7.5's alias of the same handler, on the same outer mux.",
	"GET /v1/machines":                       "Reads the backend kind and the cached readiness bool. Nothing is created and nothing is probed.",
	"GET /v1/projects":                       "A git enumeration through three read-only subcommands (project.Scanner).",
	"GET /v1/projects/{projectId}/branches":  "The same enumeration, filtered to one project.",
	"GET /v1/projects/{projectId}/worktrees": "An enumeration plus one registry read and one flag snapshot. The ready flag is only ever WRITTEN by the PATCH beside it.",
	"GET /v1/harnesses":                      "A package-level table (contract §2.9), copied on the way out.",
	"GET /v1/sessions": "One Store.List, filtered and sorted in memory. The verbs that change a " +
		"session are POST, PATCH and DELETE beside it; nothing here creates a sandbox, " +
		"destroys one, or touches the registry.",
	"GET /v1/sessions/{id}": "One Store.Get. An exited session stays readable — the transcript is the " +
		"artifact — and reading it changes nothing about the record or the sandbox.",
	"GET /v1/sessions/{id}/reviewed-context": "Reads the owned session and verifies its bounded immutable bundle, then returns one fixed canonical document. No source refresh, current approval changes, repair, provider resolution, execution or persistence occurs; legacy/missing/corrupt capture is refused without substitution.",
	"GET /v1/sessions/{id}/native-turn": "Explicit read of one owned session and its authenticated sandbox's bounded native turn summary. The route never sends a model request, changes a session or writes a candidate; unavailable/stale evidence returns unknown.",
}

// catalogUpgradeGatedRoutes is the third category: a route that IS a GET, DOES
// write, and is safe anyway because every write sits behind a WebSocket upgrade
// the §10a attacker cannot perform.
//
// THE ARGUMENT IS NOT RESTATED HERE. It is contract §10a.1, which carries the
// ruling, the Fetch-spec and RFC 6455 citations, and the reasoning about what
// §10a's threat model does and does not cover. A reason string is the wrong
// place for a security argument: copied, it drifts from the contract silently.
//
// Two things this file owes on top of that pointer, and they are the reason a
// route lands here rather than in the map above:
//
//   - It is a separate map because catalogReadOnlyRoutes is read to learn what
//     is read-only, and a writing route listed there teaches something false.
//     §10a.1 records that as the deciding argument.
//   - §10a.1's exception is conditional and the condition is ORDERING: the write
//     must follow the gate. That is pinned by
//     TestAttachWritesNothingUntilTheOriginAndUpgradeChecksHavePassed in
//     attach_test.go, which refuses at each gate and asserts nothing was written.
//     An entry here costs that test as well as this line.
var catalogUpgradeGatedRoutes = map[string]string{
	"GET /v1/sessions/{id}/attach": "Writes attachCount and lastOutputAt, so NOT read-only. A GET " +
		"because RFC 6455 makes a WebSocket handshake one; gated because handleAttach writes nothing " +
		"until its origin check and its upgrade check have both passed. Contract §10a.1 is the " +
		"ruling and the full argument; " +
		"TestAttachWritesNothingUntilTheOriginAndUpgradeChecksHavePassed is the proof the ordering " +
		"still holds.",
}

// catalogMethodAgnosticRoutes is the other half of the audit, and the half that is
// easy to forget: a pattern with NO method in it answers every verb, GET included,
// so a check that only inspects patterns labelled GET would wave it straight
// through. Each entry states why the route cannot act on a GET.
var catalogMethodAgnosticRoutes = map[string]string{
	"/api/v1/": "Contract §7.5's alias. It strips /api and re-dispatches into the same mux, " +
		"where a method-bearing pattern decides — so the audit of those patterns covers this one.",
	"/": "The outer mux's catch-all into the origin middleware and the guarded mux. " +
		"Same argument: every route it can reach is audited on its own pattern.",
}

// catalogAuditRoutes applies contract §10a to a list of registered patterns and
// returns one message per violation. It takes the list rather than reading it,
// so the same rule can be run against the real route table and against a pattern
// that does not exist yet.
func catalogAuditRoutes(patterns []string) []string {
	var violations []string
	for _, pattern := range patterns {
		method, _, hasMethod := strings.Cut(pattern, " ")
		switch {
		case !hasMethod:
			if _, allowed := catalogMethodAgnosticRoutes[pattern]; !allowed {
				violations = append(violations, fmt.Sprintf(
					"%q is registered with no method, so it answers GET as well as everything else. "+
						"Contract §10a: no GET may change state, because task 9's guard passes a request "+
						"with no Origin and the browser sets Host itself. Name the method in the pattern, "+
						"or add the pattern to catalogMethodAgnosticRoutes with the reason a GET is safe on it.",
					pattern))
			}
		case method == http.MethodGet, method == http.MethodHead:
			_, readOnly := catalogReadOnlyRoutes[pattern]
			_, upgradeGated := catalogUpgradeGatedRoutes[pattern]
			if !readOnly && !upgradeGated {
				violations = append(violations, fmt.Sprintf(
					"%q is a %s on neither allowlist. Contract §10a: no GET or HEAD "+
						"may change state — a hostile page can emit <img src> at this route and both of "+
						"the control plane's checks pass. If it only reads, add it to "+
						"catalogReadOnlyRoutes with the reason. If every write it makes sits behind a "+
						"WebSocket upgrade, it belongs in catalogUpgradeGatedRoutes (§10a.1) — and owes "+
						"a test that the write really does follow the gate. Otherwise it needs POST, "+
						"PATCH or DELETE — or a Phase 8 token — and not a line in either map.",
					pattern, method))
			}
			if readOnly && upgradeGated {
				violations = append(violations, fmt.Sprintf(
					"%q is on BOTH allowlists, which cannot both be true: one says it never writes and "+
						"the other says its writes are gated. Delete whichever is wrong.",
					pattern))
			}
		}
	}
	return violations
}

func TestNoRegisteredGetCanChangeState(t *testing.T) {
	// Group C ruling 1: the invariant contract §10a states in prose, checked by
	// a machine. It walks what New ACTUALLY registered rather than a list
	// maintained by hand, and it reads the recorders live rather than a snapshot,
	// so tasks 11 and 12's routes are audited the moment they are added —
	// wherever in New the registering line lands, and with no change to this test.
	d, _ := catalogNewDeps(t)
	a, err := api.New(d)
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}

	registered := a.RegisteredPatterns()
	if len(registered) == 0 {
		t.Fatal("New registered no patterns; the audit would pass vacuously")
	}
	for _, violation := range catalogAuditRoutes(registered) {
		t.Error(violation)
	}

	// The reverse direction, so the allowlist cannot rot: an entry for a route
	// that no longer exists is standing permission for whatever is registered
	// under that pattern next.
	live := make(map[string]bool, len(registered))
	for _, pattern := range registered {
		live[pattern] = true
	}
	for name, allowlist := range map[string]map[string]string{
		"catalogReadOnlyRoutes":       catalogReadOnlyRoutes,
		"catalogUpgradeGatedRoutes":   catalogUpgradeGatedRoutes,
		"catalogMethodAgnosticRoutes": catalogMethodAgnosticRoutes,
	} {
		for pattern := range allowlist {
			if !live[pattern] {
				t.Errorf("%s allows %q, which is not registered — delete the line", name, pattern)
			}
		}
	}
}

func TestTheRouteAuditRejectsASideEffectingGet(t *testing.T) {
	// The audit above is only worth having if it fails, so this drives the two
	// shapes a side effect arrives in: a GET that reads like a verb, and a
	// pattern registered with no method at all.
	for name, pattern := range map[string]string{
		"a GET that is really a verb": "GET /v1/sessions/{sessionId}/kill",
		"a pattern with no method":    "/v1/sessions/{sessionId}",
	} {
		t.Run(name, func(t *testing.T) {
			violations := catalogAuditRoutes([]string{pattern})
			if len(violations) != 1 {
				t.Fatalf("audit of %q returned %d violations, want 1: %v", pattern, len(violations), violations)
			}
			if !strings.Contains(violations[0], pattern) {
				t.Errorf("violation = %q, want it to name the offending pattern", violations[0])
			}
			if !strings.Contains(violations[0], "§10a") {
				t.Errorf("violation = %q, want it to point at contract §10a", violations[0])
			}
		})
	}
}

func TestTheRouteAuditRejectsAPatternClaimingToBeBothReadOnlyAndGated(t *testing.T) {
	// The two allowlists make incompatible claims — "it never writes" and "its
	// writes are gated" — so a pattern on both is a mistake someone makes by
	// adding a line without deleting the other one. The maps are test-local, so
	// the cheapest honest way to drive the branch is to put a pattern on both
	// and take it off again.
	const pattern = "GET /v1/test-only/both-allowlists"
	catalogReadOnlyRoutes[pattern] = "a fixture, not a route"
	catalogUpgradeGatedRoutes[pattern] = "a fixture, not a route"
	t.Cleanup(func() {
		delete(catalogReadOnlyRoutes, pattern)
		delete(catalogUpgradeGatedRoutes, pattern)
	})

	violations := catalogAuditRoutes([]string{pattern})
	if len(violations) != 1 {
		t.Fatalf("audit returned %d violations, want 1: %v", len(violations), violations)
	}
	if !strings.Contains(violations[0], "BOTH") {
		t.Errorf("violation = %q, want it to say the pattern is on both allowlists", violations[0])
	}
}

func TestNoFileRegistersOnTheRawMuxBehindTheRecorder(t *testing.T) {
	// The belt to the recorder's braces. Making routes.mux a named unexported
	// field means mux.ServeMux.HandleFunc — the spelling the embedded version
	// promoted into every file here — no longer compiles, but Go has no barrier
	// against a.guarded.mux.Handle(…) INSIDE this package, and a route
	// registered that way is served and never audited.
	//
	// So: no source file but api.go, which declares the type, may select .mux at
	// all. Reaching past the recorder becomes a deliberate act that reddens a
	// suite and names this rule, rather than a nine-character alternative
	// spelling.
	//
	// _test.go files are out of scope, and not as an oversight: the threat is a
	// route the PRODUCT serves and no test walks, and a test file registers
	// nothing in the product binary. Scoping it this way also keeps this file,
	// which has to spell the forbidden selectors out, from matching itself.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package directory: %v", err)
	}
	found := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || name == "api.go" {
			continue
		}
		src, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		found++
		for _, bypass := range []string{".mux.Handle", ".mux.ServeMux", ".mux.HandleFunc"} {
			if strings.Contains(string(src), bypass) {
				t.Errorf("%s contains %q. Register through the *routes recorder — mux.Handle or "+
					"mux.HandleFunc — so contract §10a's audit can see the route. Reaching the "+
					"mux behind it serves a route no test walks.", name, bypass)
			}
		}
	}
	if found == 0 {
		t.Fatal("scanned no source files; the working directory is not the package source " +
			"directory, or every handler file has been renamed and this scan is now a no-op")
	}
}
