package product

// This describes the saved route's scheduling policy, never current readiness,
// credentials, provider entitlement or permission to allocate a worker.
type swarmParallelPolicy struct {
	Mode    string
	Maximum int
}

const swarmManagedCredentialMessage = "These independent Luci jobs share managed ChatGPT credentials and run one at a time to protect credential refresh. Choose one slot, or explicitly select and review a supported API model connection for bounded parallel jobs. Current access, provider and resource limits still apply."

func projectSwarmParallelPolicy(route ProjectRunRoute) swarmParallelPolicy {
	c := route.Connection
	if c.Kind == "codex-subscription" || route.Harness == "codex" && c.AuthMode == "chatgpt" {
		return swarmParallelPolicy{Mode: "managed-credential-serial", Maximum: 1}
	}
	knownKind := c.Kind == "litellm" || c.Kind == "compatible-api" || c.Kind == "direct-api"
	protocol := map[string]string{"codex": "responses", "claude-code": "anthropic-messages", "opencode": "chat-completions"}[route.Harness]
	expectedModel := c.Model
	if route.Harness == "opencode" { expectedModel = "luci-gateway/" + c.Model }
	if knownKind && c.ID != "" && c.Revision > 0 && c.AuthMode == "api-key" && protocol != "" && c.Protocol == protocol && c.Endpoint != "" && c.Model != "" && expectedModel == route.Model {
		return swarmParallelPolicy{Mode: "bounded-api-route", Maximum: MaxSwarmParallelTasks}
	}
	return swarmParallelPolicy{Mode: "unknown"}
}

func defaultSwarmParallelLimit(route ProjectRunRoute) int {
	if projectSwarmParallelPolicy(route).Mode == "bounded-api-route" {
		return 2
	}
	return 1
}
