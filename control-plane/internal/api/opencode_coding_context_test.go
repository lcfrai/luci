package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func codingModelEnvironment() map[string]string {
	return map[string]string{"OPENCODE_CONFIG_CONTENT": `{"model":"luci-local/luci-acceptance-qwen2.5-general-8k:latest","instructions":["LUCI.md"],"provider":{"luci-local":{"options":{"baseURL":"http://host.docker.internal:11434/v1","apiKey":"{env:OPENCODE_LOCAL_API_KEY}"},"models":{"luci-acceptance-qwen2.5-general-8k:latest":{"name":"saved"}}}}}`}
}

func TestReviewedOpenCodeTaskUsesCapturedContextAndVerifiedModelLimit(t *testing.T) {
	ctx := context.Background()
	read := func(_ context.Context, host, alias string) (int, error) {
		if host != "127.0.0.1:11434" || alias != "luci-acceptance-qwen2.5-general-8k:latest" {
			t.Fatalf("wrong probe target %q %q", host, alias)
		}
		return 8192, nil
	}
	env := codingModelEnvironment()
	if err := clearReviewedOpenCodeInstructions(env); err != nil {
		t.Fatal(err)
	}
	if err := prepareLocalOpenCodeCodingContextWith(ctx, env, read); err != nil {
		t.Fatal(err)
	}
	var config struct {
		Instructions []string `json:"instructions"`
		Provider     map[string]struct {
			Models map[string]struct {
				Limit struct {
					Context int `json:"context"`
					Output  int `json:"output"`
				} `json:"limit"`
			} `json:"models"`
		} `json:"provider"`
	}
	if err := json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config); err != nil {
		t.Fatal(err)
	}
	limit := config.Provider["luci-local"].Models["luci-acceptance-qwen2.5-general-8k:latest"].Limit
	if limit.Context != 8192 || limit.Output != 2048 || len(config.Instructions) != 0 {
		t.Fatalf("wrong exact model limits: %+v", limit)
	}
	if !strings.Contains(env["OPENCODE_CONFIG_CONTENT"], "{env:OPENCODE_LOCAL_API_KEY}") {
		t.Fatal("lost the private key placeholder")
	}
	ordinary := codingModelEnvironment()
	if err := prepareLocalOpenCodeCodingContextWith(ctx, ordinary, func(context.Context, string, string) (int, error) { return 8192, nil }); err != nil {
		t.Fatalf("ordinary bounded 8k session: %v", err)
	}
	if !strings.Contains(ordinary["OPENCODE_CONFIG_CONTENT"], `"instructions":["LUCI.md"]`) {
		t.Fatal("ordinary session lost its project instructions")
	}
}

func TestOllamaCodingContextProbeIsBoundedAndExact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/show" {
			t.Fatalf("wrong request %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			Model string `json:"model"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Model != "exact-alias:latest" {
			t.Fatal("wrong model identity")
		}
		w.Write([]byte(`{"parameters":"temperature 0.7\nnum_ctx 12288\n"}`))
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	window, err := readLocalOllamaContext(context.Background(), host, "exact-alias:latest")
	if err != nil || window != 12288 {
		t.Fatalf("window=%d err=%v", window, err)
	}
}
