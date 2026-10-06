package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const sessionGatewayKey = "fixture-selected-gateway-key-no-live-auth-12345"

func sessionSaveConnection(t *testing.T, env *sessionsEnv, revision int, id, provider string, enabled bool) product.AgentConnection {
	t.Helper()
	models := product.AgentConnectionModels{}
	endpoints := product.AgentConnectionEndpoints{}
	switch provider {
	case "codex":
		models.Codex = "gateway-codex"
		endpoints.Responses = "http://127.0.0.1:4000/v1"
	case "claude-code":
		models.ClaudeCode = "gateway-claude"
		endpoints.AnthropicMessages = "http://localhost:4000"
	case "opencode":
		models.OpenCode = "gateway-open"
		endpoints.ChatCompletions = "http://[::1]:4000/v1"
	}
	body, _ := json.Marshal(map[string]any{"action": "save", "expectedRevision": revision, "id": id, "name": "Selected test gateway", "kind": "litellm", "enabled": enabled, "models": models, "endpoints": endpoints, "apiKey": sessionGatewayKey})
	w := env.do(t, http.MethodPost, "/v1/setup/agent-connections", string(body))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var catalogue struct {
		Connections []product.AgentConnection `json:"connections"`
	}
	if json.Unmarshal(w.Body.Bytes(), &catalogue) != nil {
		t.Fatal("invalid connection response")
	}
	for _, c := range catalogue.Connections {
		if id == "" || c.ID == id {
			return c
		}
	}
	t.Fatal("saved connection missing")
	return product.AgentConnection{}
}

func TestSavedAgentRoutesBindRealSessionSpecsWithoutSecretEvidence(t *testing.T) {
	for _, provider := range []string{"codex", "claude-code", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			env, _, wt, _ := sessionsPrimaryBranchFixture(t)
			sessionsStubReady(t, nil)
			done := sessionsWatchDone(t)
			c := sessionSaveConnection(t, env, 0, "", provider, true)
			selected, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": provider, "agentConnectionId": c.ID, "model": "default"})
			w := env.do(t, http.MethodPost, "/v1/sessions", string(selected))
			created := sessionsDecodeOne(t, w, 201)
			sessionsAwaitWatch(t, done)
			spec := env.be.spec(t, 0)
			if err := backend.ValidateSpec(spec); err != nil {
				t.Fatal("selected route produced a spec rejected by the actual execution boundary:", err)
			}
			record, err := env.st.Get(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if record.AgentConnectionID != c.ID || record.AgentConnectionRevision != c.Revision || record.AgentConnectionKind != "litellm" || record.AgentConnectionProtocol == "" {
				t.Fatal("session did not retain immutable public route binding")
			}
			if record.CodexConnectionID != "" {
				t.Fatal("gateway session selected subscription account")
			}
			joined := strings.Join(spec.Args, " ")
			if provider == "codex" {
				if spec.Env["OPENAI_API_KEY"] != sessionGatewayKey || !strings.Contains(joined, `base_url="http://host.docker.internal:4000/v1"`) || !slices.Contains(spec.Args, "gateway-codex") || !slices.Contains(spec.Args, "danger-full-access") || !slices.Contains(spec.Args, "on-request") || slices.Contains(spec.Args, "--dangerously-bypass-approvals-and-sandbox") {
					t.Fatal("Docker Codex API/Ask route not applied")
				}
				if record.AuthMode != "api-key" {
					t.Fatal("gateway was recorded as subscription")
				}
			}
			if provider == "claude-code" && (spec.Env["ANTHROPIC_AUTH_TOKEN"] != sessionGatewayKey || spec.Env["ANTHROPIC_BASE_URL"] != "http://host.docker.internal:4000" || !slices.Contains(spec.Args, "gateway-claude")) {
				t.Fatal("Claude token gateway not applied")
			}
			if provider == "opencode" && (spec.Env["OPENCODE_LOCAL_API_KEY"] != sessionGatewayKey || !strings.Contains(spec.Env["OPENCODE_CONFIG_CONTENT"], "http://host.docker.internal:4000/v1") || !slices.Contains(spec.Args, "luci-gateway/gateway-open")) {
				t.Fatal("OpenCode exact provider alias not applied")
			}
			raw, _ := json.Marshal(record)
			for _, v := range []string{w.Body.String(), string(raw), joined, env.do(t, http.MethodGet, "/v1/sessions", "").Body.String(), env.do(t, http.MethodGet, "/v1/setup/agent-connections", "").Body.String()} {
				if strings.Contains(v, sessionGatewayKey) {
					t.Fatal("private connection key entered public/registry/argv evidence")
				}
			}
		})
	}
}

func TestSavedAgentRevisionChangesRefuseRestartBeforeDestroy(t *testing.T) {
	env, _, wt, _ := sessionsPrimaryBranchFixture(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	c := sessionSaveConnection(t, env, 0, "", "claude-code", true)
	body, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": "claude-code", "agentConnectionId": c.ID})
	created := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	before, _ := env.st.Get(created.ID)
	sessionSaveConnection(t, env, 1, c.ID, "claude-code", true)
	w := env.do(t, "POST", "/v1/sessions/"+created.ID+"/restart", "")
	sessionsProblemCode(t, w, 409)
	if !strings.Contains(w.Body.String(), "Start a new session") || env.be.destroyCount() != 0 || len(env.be.specs) != 1 {
		t.Fatal("changed route destroyed or restarted old session")
	}
	after, _ := env.st.Get(created.ID)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("refused restart modified retained session")
	}
	sessionSaveConnection(t, env, 2, c.ID, "claude-code", false)
	sessionsProblemCode(t, env.do(t, "POST", "/v1/sessions/"+created.ID+"/restart", ""), 409)
	if env.be.destroyCount() != 0 {
		t.Fatal("disabled route destroyed old session")
	}
	deleted, _ := json.Marshal(map[string]any{"action": "delete", "expectedRevision": 3, "id": c.ID})
	if w := env.do(t, "POST", "/v1/setup/agent-connections", string(deleted)); w.Code != 200 {
		t.Fatal(w.Code)
	}
	sessionsProblemCode(t, env.do(t, "POST", "/v1/sessions/"+created.ID+"/restart", ""), 409)
	if env.be.destroyCount() != 0 {
		t.Fatal("deleted route destroyed old session")
	}
}

func TestSessionActiveDefaultUsesSavedModelButExistingLegacyNeverSwitches(t *testing.T) {
	env, _, wt, _ := sessionsPrimaryBranchFixture(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	legacy := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", `{"worktreeId":"`+wt.ID+`","harness":"claude-code"}`), 201)
	sessionsAwaitWatch(t, done)
	c := sessionSaveConnection(t, env, 0, "", "claude-code", true)
	selected, _ := json.Marshal(map[string]any{"action": "select", "expectedRevision": 1, "id": c.ID})
	if env.do(t, "POST", "/v1/setup/agent-connections", string(selected)).Code != 200 {
		t.Fatal("select failed")
	}
	active := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", `{"worktreeId":"`+wt.ID+`","harness":"claude-code","model":"opus"}`), 201)
	sessionsAwaitWatch(t, done)
	if active.Model != "gateway-claude" {
		t.Fatal("quick create ignored selected model")
	}
	explicit, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": "claude-code", "model": "another-model", "agentConnectionId": c.ID})
	sessionsProblemCode(t, env.do(t, "POST", "/v1/sessions", string(explicit)), 409)
	sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions/"+legacy.ID+"/restart", ""), 200)
	sessionsAwaitWatch(t, done)
	if spec := env.be.spec(t, 2); spec.Env["ANTHROPIC_AUTH_TOKEN"] != "" || !slices.Contains(spec.Args, "opus") {
		t.Fatal("existing legacy session silently switched to active gateway")
	}
	sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", `{"worktreeId":"`+wt.ID+`","harness":"shell"}`), 201)
	sessionsAwaitWatch(t, done)
	for key := range env.be.spec(t, 3).Env {
		if strings.Contains(key, "ANTHROPIC") || strings.Contains(key, "OPENAI") || strings.Contains(key, "GATEWAY") || strings.Contains(key, "OPENCODE") {
			t.Fatal("shell received model route environment")
		}
	}
}

func TestExplicitSubscriptionCannotFallBackToConfiguredAPI(t *testing.T) {
	env, _, wt, _ := sessionsPrimaryBranchFixture(t)
	key, _ := json.Marshal(map[string]string{"apiKey": sessionGatewayKey})
	if env.do(t, "POST", "/v1/setup/agents/openai", string(key)).Code != 200 {
		t.Fatal("legacy API save failed")
	}
	body, _ := json.Marshal(map[string]any{"action": "save", "expectedRevision": 0, "name": "Subscription only", "kind": "codex-subscription", "enabled": true})
	w := env.do(t, "POST", "/v1/setup/agent-connections", string(body))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var got struct{ Connections []product.AgentConnection }
	json.Unmarshal(w.Body.Bytes(), &got)
	body, _ = json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": "codex", "agentConnectionId": got.Connections[0].ID})
	sessionsProblemCode(t, env.do(t, "POST", "/v1/sessions", string(body)), 409)
	if len(env.be.specs) != 0 {
		t.Fatal("subscription-only request started with legacy API credential")
	}
}

func TestSessionAgentBindingResumeAndRestartResolverRejectsRouteSwitches(t *testing.T) {
	env, _, _, _ := sessionsPrimaryBranchFixture(t)
	c := sessionSaveConnection(t, env, 0, "", "codex", true)
	source := registry.Session{ID: "s-owned", AgentConnectionID: c.ID, AgentConnectionRevision: c.Revision, AgentConnectionKind: c.Kind, AgentConnectionProtocol: "responses"}
	receipt, _, model, err := env.api.resolveSessionAgent("codex", nil, source, "gateway-codex")
	if err != nil || receipt.ID != c.ID || model != "gateway-codex" {
		t.Fatal("source route not retained")
	}
	empty := ""
	if _, _, _, err := env.api.resolveSessionAgent("codex", &empty, source, "default"); err == nil {
		t.Fatal("resume silently switched billing route")
	}
	legacy := registry.Session{ID: "s-old"}
	if receipt, _, _, err := env.api.resolveSessionAgent("codex", nil, legacy, "default"); err != nil || receipt.ID != "" {
		t.Fatal("old session route changed")
	}
}

func TestDockerEndpointsAreRewrittenBeforeStaticCodexConfiguration(t *testing.T) {
	env := map[string]string{"OPENAI_API_KEY": sessionGatewayKey, "LUCI_CODEX_BASE_URL": "http://[::1]:4000/gateway/v1", "ANTHROPIC_BASE_URL": "https://api.example.test/path", "OPENCODE_CONFIG_CONTENT": `{"provider":{"luci-gateway":{"options":{"baseURL":"http://localhost:4000/v1"}},"unrelated":{"options":{"baseURL":"http://localhost:5000/v1"}}}}`}
	nativeDockerEndpoint(env)
	args := nativeArgs("codex", []string{"--model", "gateway-codex"}, env, false)
	if env["LUCI_CODEX_BASE_URL"] != "http://host.docker.internal:4000/gateway/v1" || env["ANTHROPIC_BASE_URL"] != "https://api.example.test/path" || !strings.Contains(strings.Join(args, " "), `base_url="http://host.docker.internal:4000/gateway/v1"`) {
		t.Fatal("static Codex provider captured container localhost")
	}
	if !strings.Contains(env["OPENCODE_CONFIG_CONTENT"], "http://localhost:5000/v1") || strings.Contains(strings.Join(args, " "), sessionGatewayKey) {
		t.Fatal("unrelated provider changed or key entered argv")
	}
	nativeArgs("codex", nil, env, true)
	if env["OPENAI_API_KEY"] != "" || env["LUCI_CODEX_BASE_URL"] != "" {
		t.Fatal("device sign-in inherited gateway API settings")
	}
}

func TestClientCannotChooseDockerBoundaryAndOtherBackendKeepsInnerSandbox(t *testing.T) {
	env, _, wt, _ := sessionsPrimaryBranchFixture(t)
	env.be.kind = backend.KindKata
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	// This is the actual HTTP decoder, not an exported harness Request.
	w := env.do(t, "POST", "/v1/sessions", `{"worktreeId":"`+wt.ID+`","harness":"codex","dockerBoundary":true}`)
	sessionsProblemCode(t, w, 400)
	if len(env.be.specs) != 0 {
		t.Fatal("caller-chosen execution boundary started a sandbox")
	}
	sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", `{"worktreeId":"`+wt.ID+`","harness":"codex"}`), 201)
	sessionsAwaitWatch(t, done)
	args := env.be.spec(t, 0).Args
	if !slices.Contains(args, "workspace-write") || slices.Contains(args, "danger-full-access") || !slices.Contains(args, "on-request") {
		t.Fatal("non-Docker backend silently lost its inner sandbox")
	}
}

func TestSandboxEnvironmentConsumesOnlyRoutingHintsAndPreservesSecretsOffArgv(t *testing.T) {
	env := map[string]string{"OPENAI_API_KEY": sessionGatewayKey, "LUCI_CODEX_BASE_URL": "https://gateway.example.test/v1", "LUCI_CODEX_MODEL": "saved-alias", "LUCI_GATEWAY_API_KEY": sessionGatewayKey, "LUCI_OPENCODE_MODEL": "luci-gateway/saved-alias", "OPENCODE_CONFIG_CONTENT": `{"provider":{"luci-gateway":{"options":{"apiKey":"{env:LUCI_GATEWAY_API_KEY}"}}}}`, "UNRELATED": "unchanged"}
	args := nativeArgs("codex", []string{"--model", "saved-alias"}, env, false)
	nativeSandboxEnvironment(env)
	if env["OPENAI_BASE_URL"] != "https://gateway.example.test/v1" || env["OPENCODE_LOCAL_API_KEY"] != sessionGatewayKey || env["UNRELATED"] != "unchanged" || !strings.Contains(env["OPENCODE_CONFIG_CONTENT"], "{env:OPENCODE_LOCAL_API_KEY}") {
		t.Fatal("provider environment lost its explicit route")
	}
	for k := range env {
		if strings.HasPrefix(k, "LUCI_") {
			t.Fatal("internal hint escaped into backend environment")
		}
	}
	if strings.Contains(strings.Join(args, " "), sessionGatewayKey) || strings.Contains(env["OPENCODE_CONFIG_CONTENT"], sessionGatewayKey) {
		t.Fatal("private key was serialized into configuration/argv")
	}
	// Unknown reserved names remain a loud backend error, rather than being
	// stripped or made admissible by the routing seam.
	env["LUCI_ATTACH_TOKEN"] = "must-remain-rejected"
	nativeSandboxEnvironment(env)
	if env["LUCI_ATTACH_TOKEN"] == "" {
		t.Fatal("routing seam hid a reserved-key attack")
	}
}
