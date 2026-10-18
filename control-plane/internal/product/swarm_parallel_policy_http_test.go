package product

import (
	"reflect"
	"strings"
	"testing"
)

func TestSwarmParallelPolicyHTTPDefaultsExplicitLimitsAndStaleSelection(t *testing.T) {
	for _, row := range []struct{ name, mode string; limit, expected, status int }{
		{"managed-default", "managed", 0, 1, 201},
		{"managed-explicit-two", "managed", 2, 0, 409},
		{"api-default", "api", 0, 2, 201},
		{"api-explicit-three", "api", 3, 3, 201},
		{"unknown-default", "unknown", 0, 1, 201},
		{"unknown-explicit-two", "unknown", 2, 0, 409},
		{"stale-api", "stale", 2, 0, 409},
	} {
		t.Run(row.name, func(t *testing.T) {
			s, mux, p, req, selected, _ := projectRunHTTPFixture(t)
			req.PermissionMode, selected.PermissionMode = "bypass", "bypass"
			selected.Connection.Kind, selected.Connection.Protocol = "compatible-api", "responses"
			selected.Connection.Endpoint = "https://example.invalid/v1"
			if row.mode == "managed" {
				selected.Connection.Kind, selected.Connection.AuthMode = "codex-subscription", "chatgpt"
				selected.Connection.Protocol, selected.Connection.Endpoint = "", ""
			}
			if row.mode == "unknown" { selected.Connection.Kind = "unassessed" }
			selected.AgentConnectionKind, selected.AgentConnectionProtocol, selected.AuthMode = selected.Connection.Kind, selected.Connection.Protocol, selected.Connection.AuthMode
			base := "/v1/product/projects/" + p.ID + "/runs"
			run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201))
			before := auditTree(t, s.stateDir)
			if row.mode == "stale" { selected.Connection.Revision++ }
			response := journeyRequest(t, mux, "POST", base+"/"+run.ID+"/swarm", map[string]int{"expectedRevision": run.Revision, "maxParallel": row.limit}, row.status)
			if row.status == 201 {
				got := projectRunHTTPResponse(t, response)
				if got.Swarm == nil || got.Swarm.MaxParallel != row.expected || len(got.Operations) != 0 || got.Input.Route != run.Input.Route { t.Fatal("preparation changed route or allocated a worker", got) }
			} else {
				if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) { t.Fatal("refused preparation mutated durable state") }
				if row.mode == "managed" && !strings.Contains(response.Body.String(), "share managed ChatGPT credentials") { t.Fatal(response.Body.String()) }
				if row.mode == "unknown" && !strings.Contains(response.Body.String(), "Parallel support is not established") { t.Fatal(response.Body.String()) }
			}
		})
	}
}
