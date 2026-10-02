package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/product"
)

func TestPersonalAgentKeyOnlyEntersClaudeSandboxAndNeverSessionEvidence(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	var logs strings.Builder
	env.api.deps.Log = slog.New(slog.NewTextHandler(&logs, nil))
	p, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: env.dir})
	if err != nil {
		t.Fatal(err)
	}
	env.api.deps.Product = p
	p.Register(env.mux)
	key := "sk-ant-fixture-personal-key-do-not-serialize-123456"
	saveBody, _ := json.Marshal(map[string]string{"apiKey": key})
	if response := env.do(t, http.MethodPost, "/v1/setup/agents/anthropic", string(saveBody)); response.Code != 200 {
		t.Fatalf("save key: %s", response.Body.String())
	}
	createdResponse := env.do(t, http.MethodPost, "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"claude-code"}`)
	created := sessionsDecodeOne(t, createdResponse, http.StatusCreated)
	sessionsAwaitWatch(t, done)
	if env.be.spec(t, 0).Env["ANTHROPIC_API_KEY"] != key {
		t.Fatal("personal key was not passed to Claude")
	}
	record, _ := env.api.deps.Store.Get(created.ID)
	recordJSON, _ := json.Marshal(record)
	listed := env.do(t, http.MethodGet, "/v1/sessions", "")
	providers := env.do(t, http.MethodGet, "/v1/setup/agents", "")
	for _, evidence := range []string{createdResponse.Body.String(), string(recordJSON), listed.Body.String(), providers.Body.String(), logs.String()} {
		if strings.Contains(evidence, key) {
			t.Fatal("personal credential leaked into a response, record or log")
		}
	}
	// Shell sessions must not inherit the key merely because Luci has one.
	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", `{"worktreeId":"w-4a77b1c9","harness":"shell"}`), http.StatusCreated)
	sessionsAwaitWatch(t, done)
	if _, exists := env.be.spec(t, 1).Env["ANTHROPIC_API_KEY"]; exists {
		t.Fatal("personal key leaked into a shell sandbox")
	}
	if response := env.do(t, http.MethodPost, "/v1/setup/agents/anthropic", `{"apiKey":""}`); response.Code != 200 {
		t.Fatalf("clear key: %s", response.Body.String())
	}
	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/"+created.ID+"/restart", ""), http.StatusOK)
	sessionsAwaitWatch(t, done)
	if _, exists := env.be.spec(t, 2).Env["ANTHROPIC_API_KEY"]; exists {
		t.Fatal("restart replayed a cleared key")
	}
	if strings.Contains(logs.String(), key) {
		t.Fatal("restart logged a personal key")
	}
}
