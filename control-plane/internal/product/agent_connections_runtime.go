package product

import (
	"encoding/json"
	"errors"
)

// ResolveAgentConnection returns a public route receipt and a fresh private
// environment map. nil chooses the saved active route (or existing legacy
// behaviour); explicitly empty opts out of that route; a named route is exact.
// Shells never receive provider configuration or credentials.
func (s *Server) ResolveAgentConnection(harness string, selection *string) (AgentConnectionReceipt, map[string]string, error) {
	legacy := AgentConnectionReceipt{Kind: "legacy"}
	if harness == "shell" || harness != "codex" && harness != "claude-code" && harness != "opencode" {
		if selection != nil && *selection != "" {
			return legacy, nil, errors.New("This harness cannot use a saved agent connection.")
		}
		return legacy, nil, nil
	}
	if _, err := s.agentConnectionPath(); err != nil {
		return legacy, nil, errors.New("Agent connections require private Luci state outside the projects folder.")
	}
	s.mu.RLock()
	id := s.agentConnections.ActiveConnectionID
	if selection != nil {
		id = *selection
	}
	var selected privateAgentConnection
	found := false
	for _, c := range s.agentConnections.Connections {
		if c.ID == id {
			selected, found = c, true
			break
		}
	}
	s.mu.RUnlock()
	if id == "" {
		return legacy, s.legacyAgentEnvironment(harness), nil
	}
	if !found || !selected.Enabled || !agentConnectionConfigured(selected) {
		return legacy, nil, errors.New("The selected agent connection is missing, disabled or incomplete. Refresh your connections and choose an enabled route.")
	}
	receipt := AgentConnectionReceipt{ID: selected.ID, Kind: selected.Kind, Revision: selected.Revision}
	if selected.Kind == "codex-subscription" {
		if harness != "codex" {
			return receipt, nil, errors.New("This connection uses a Codex subscription. Choose Codex or select a compatible API connection.")
		}
		receipt.AuthMode, receipt.Model = "chatgpt", selected.Models.Codex
		return receipt, nil, nil
	}
	receipt.AuthMode = "api-key"
	switch harness {
	case "codex":
		if selected.Endpoints.Responses == "" || selected.Models.Codex == "" || selected.APIKey == "" {
			return receipt, nil, errors.New("This connection requires an explicit Responses API base URL, Codex model and private API key.")
		}
		receipt.Protocol, receipt.Endpoint, receipt.Model = "responses", selected.Endpoints.Responses, selected.Models.Codex
		return receipt, map[string]string{"OPENAI_API_KEY": selected.APIKey, "LUCI_CODEX_BASE_URL": receipt.Endpoint, "LUCI_CODEX_MODEL": receipt.Model}, nil
	case "claude-code":
		if selected.Endpoints.AnthropicMessages == "" || selected.Models.ClaudeCode == "" || selected.APIKey == "" {
			return receipt, nil, errors.New("This connection requires an explicit Anthropic Messages base URL, Claude model and private API key.")
		}
		receipt.Protocol, receipt.Endpoint, receipt.Model = "anthropic-messages", selected.Endpoints.AnthropicMessages, selected.Models.ClaudeCode
		env := map[string]string{"ANTHROPIC_BASE_URL": receipt.Endpoint, "ANTHROPIC_MODEL": receipt.Model, "ANTHROPIC_DEFAULT_OPUS_MODEL": receipt.Model, "ANTHROPIC_DEFAULT_SONNET_MODEL": receipt.Model, "ANTHROPIC_DEFAULT_HAIKU_MODEL": receipt.Model}
		if selected.Kind == "direct-api" {
			env["ANTHROPIC_API_KEY"] = selected.APIKey
		} else {
			env["ANTHROPIC_AUTH_TOKEN"] = selected.APIKey
		}
		return receipt, env, nil
	case "opencode":
		if selected.Endpoints.ChatCompletions == "" || selected.Models.OpenCode == "" {
			return receipt, nil, errors.New("This connection requires an explicit ChatCompletions base URL and OpenCode model.")
		}
		receipt.Protocol, receipt.Endpoint, receipt.Model = "chat-completions", selected.Endpoints.ChatCompletions, selected.Models.OpenCode
		config := map[string]any{"model": "luci-gateway/" + receipt.Model, "small_model": "luci-gateway/" + receipt.Model, "enabled_providers": []string{"luci-gateway"}, "autoupdate": false, "share": "disabled", "instructions": []string{"LUCI.md"}, "provider": map[string]any{"luci-gateway": map[string]any{"npm": "@ai-sdk/openai-compatible", "name": "Luci selected connection", "options": map[string]any{"baseURL": receipt.Endpoint, "apiKey": "{env:LUCI_GATEWAY_API_KEY}"}, "models": map[string]any{receipt.Model: map[string]any{"name": receipt.Model}}}}}
		data, err := json.Marshal(config)
		if err != nil {
			return receipt, nil, errors.New("Could not prepare the selected OpenCode model configuration.")
		}
		key := selected.APIKey
		if key == "" {
			key = "local-no-key"
		}
		return receipt, map[string]string{"OPENCODE_CONFIG_CONTENT": string(data), "LUCI_GATEWAY_API_KEY": key, "LUCI_OPENCODE_MODEL": "luci-gateway/" + receipt.Model}, nil
	}
	return receipt, nil, errors.New("This harness cannot use a saved agent connection.")
}
