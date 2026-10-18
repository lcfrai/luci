package product

import "testing"

func TestSwarmParallelPolicyManagedCredentialAndOpaqueAPIRoutes(t *testing.T) {
	for _, row := range []struct{ harness, protocol string }{{"codex", "responses"}, {"claude-code", "anthropic-messages"}, {"opencode", "chat-completions"}} {
		t.Run(row.harness, func(t *testing.T) {
			route := ProjectRunRoute{Harness: row.harness, Model: "CaseSensitive/Model", Connection: AgentConnectionReceipt{ID: "saved", Revision: 3, Kind: "compatible-api", AuthMode: "api-key", Protocol: row.protocol, Model: "CaseSensitive/Model", Endpoint: "http://localhost:3210/CaseSensitive/v1"}}
			before := route
			policy := projectSwarmParallelPolicy(route)
			if policy.Mode != "bounded-api-route" || policy.Maximum != MaxSwarmParallelTasks || defaultSwarmParallelLimit(route) != 2 || route != before {
				t.Fatal("API policy changed the receipt or claimed the wrong bound", policy)
			}
		})
	}
	for _, kind := range []string{"codex-subscription", "legacy"} {
		t.Run(kind, func(t *testing.T) {
			route := ProjectRunRoute{Harness: "codex", Connection: AgentConnectionReceipt{Kind: kind, AuthMode: "chatgpt"}}
			if policy := projectSwarmParallelPolicy(route); policy.Mode != "managed-credential-serial" || policy.Maximum != 1 || defaultSwarmParallelLimit(route) != 1 {
				t.Fatal(policy)
			}
		})
	}
}

func TestSwarmParallelPolicyUnknownDoesNotClaimParallelReadiness(t *testing.T) {
	base := ProjectRunRoute{Harness: "codex", Model: "Model", Connection: AgentConnectionReceipt{ID: "saved", Revision: 1, Kind: "direct-api", AuthMode: "api-key", Protocol: "responses", Model: "Model", Endpoint: "https://example.invalid/v1"}}
	for _, name := range []string{"id", "revision", "kind", "auth", "protocol", "model", "endpoint", "harness", "case"} {
		t.Run(name, func(t *testing.T) {
			route := base
			switch name {
			case "id": route.Connection.ID = ""
			case "revision": route.Connection.Revision = 0
			case "kind": route.Connection.Kind = "legacy"
			case "auth": route.Connection.AuthMode = "unknown"
			case "protocol": route.Connection.Protocol = "chat-completions"
			case "model": route.Connection.Model = ""
			case "endpoint": route.Connection.Endpoint = ""
			case "harness": route.Harness = "shell"
			case "case": route.Connection.Model = "model"
			}
			if policy := projectSwarmParallelPolicy(route); policy.Mode != "unknown" || policy.Maximum != 0 || defaultSwarmParallelLimit(route) != 1 {
				t.Fatal("unknown route acquired parallel support", policy)
			}
		})
	}
}
