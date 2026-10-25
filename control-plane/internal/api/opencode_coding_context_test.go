package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func codingModelEnvironment() map[string]string {
	return map[string]string{"OPENCODE_CONFIG_CONTENT": `{"model":"luci-local/luci-acceptance-qwen2.5-general-8k:latest","instructions":["LUCI.md"],"provider":{"luci-local":{"options":{"baseURL":"http://host.docker.internal:11434/v1","apiKey":"{env:OPENCODE_LOCAL_API_KEY}"},"models":{"luci-acceptance-qwen2.5-general-8k:latest":{"name":"saved"}}}}}`}
}

func TestNamedLocalOpenCodeSharesReviewedPolicyAndExactContext(t *testing.T) {
	for _, provider := range []string{"luci-local", "luci-gateway"} {
		t.Run(provider, func(t *testing.T) {
			env := codingModelEnvironment()
			env["OPENCODE_CONFIG_CONTENT"] = strings.ReplaceAll(env["OPENCODE_CONFIG_CONTENT"], "luci-local", provider)
			if err := clearReviewedOpenCodeInstructions(env); err != nil {
				t.Fatal(err)
			}
			engineCalls, modelCalls := 0, 0
			err := prepareLocalOpenCodeCodingContextWith(context.Background(), env, func(context.Context) error { engineCalls++; return nil }, func(_ context.Context, host, alias string) (int, error) {
				modelCalls++
				if engineCalls != 1 || host != "127.0.0.1:11434" || alias != "luci-acceptance-qwen2.5-general-8k:latest" {
					t.Fatal("probe changed the selected host/model or preceded engine authority", host, alias)
				}
				return 8192, nil
			})
			if err != nil || engineCalls != 1 || modelCalls != 1 {
				t.Fatal("selected route did not complete exact context preparation", err)
			}
			if env["BUN_OPTIONS"] != "--smol" {
				t.Fatal("verified local route did not select the lower-memory runtime profile")
			}
			var config map[string]any
			if err := json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config); err != nil {
				t.Fatal(err)
			}
			selected, err := selectedLocalOpenCodeOllama(config)
			if err != nil || selected == nil {
				t.Fatal("prepared route lost its selected model", err)
			}
			limit := selected.entry["limit"].(map[string]any)
			build := config["agent"].(map[string]any)["build"].(map[string]any)
			permissions := build["permission"].(map[string]any)
			if limit["context"] != float64(8192) || limit["output"] != float64(2048) || build["prompt"] != reviewedLocalOpenCodePrompt || len(config["instructions"].([]any)) != 0 {
				t.Fatal("named and legacy routes disagree on the reviewed task or model window")
			}
			for _, guidance := range []string{"obeying its captured role and write scope", "For an implementation task with assigned write paths", "create the required file only when that task allows it", "complete the selected task", "Use only tools allowed by the selected task and existing permissions"} {
				if !strings.Contains(reviewedLocalOpenCodePrompt, guidance) {
					t.Fatal("fixed prompt lost the selected task's role or permission boundary", guidance)
				}
			}
			for _, blanket := range []string{"continue its implementation", "Use direct coding and check tools", "in the writable checkout"} {
				if strings.Contains(reviewedLocalOpenCodePrompt, blanket) {
					t.Fatal("fixed prompt biases a captured reviewer toward implementation", blanket)
				}
			}
			for _, tool := range []string{"task", "webfetch", "websearch"} {
				if permissions[tool] != "deny" || config["tools"].(map[string]any)[tool] != false {
					t.Fatal("reviewed local tool policy lost", tool)
				}
			}
			for _, tool := range []string{"edit", "write", "bash"} {
				if _, changed := permissions[tool]; changed {
					t.Fatal("local preparation changed selected coding permissions", tool)
				}
			}
		})
	}
}

func TestLocalOpenCodeLowMemoryProfilePreservesExplicitRuntimeOptions(t *testing.T) {
	for _, provider := range []string{"luci-local", "luci-gateway"} {
		for _, options := range []string{"", "--smol", "--use-system-ca", "--use-system-ca --smol"} {
			t.Run(provider+"/"+options, func(t *testing.T) {
				env := codingModelEnvironment()
				env["OPENCODE_CONFIG_CONTENT"] = strings.ReplaceAll(env["OPENCODE_CONFIG_CONTENT"], "luci-local", provider)
				env["BUN_OPTIONS"] = options
				env["OPENCODE_PERMISSION"] = `{"*":"ask"}`
				if err := prepareLocalOpenCodeCodingContextWith(context.Background(), env, func(context.Context) error { return nil }, func(context.Context, string, string) (int, error) { return 8192, nil }); err != nil {
					t.Fatal(err)
				}
				if actual, present := env["BUN_OPTIONS"]; !present || actual != options || env["OPENCODE_PERMISSION"] != `{"*":"ask"}` {
					t.Fatal("local preparation replaced explicit runtime options or selected permissions")
				}
			})
		}
	}
}

func TestOrdinaryLocalOpenCodeUsesConciseTaskPromptWithoutReviewedAuthority(t *testing.T) {
	for _, provider := range []string{"luci-local", "luci-gateway"} {
		for _, shape := range []string{"no agents", "other agent", "configured build"} {
			t.Run(provider+"/"+shape, func(t *testing.T) {
				env := codingModelEnvironment()
				var config map[string]any
				if err := json.Unmarshal([]byte(strings.ReplaceAll(env["OPENCODE_CONFIG_CONTENT"], "luci-local", provider)), &config); err != nil {
					t.Fatal(err)
				}
				instructions := []any{"LUCI.md", "docs/conventions.md"}
				tools := map[string]any{"edit": true, "webfetch": false}
				permission := map[string]any{"*": "ask", "external_directory": "deny"}
				plan := map[string]any{"prompt": "Keep the selected plan agent", "temperature": 0.2}
				buildFields := map[string]any{}
				config["instructions"], config["tools"], config["permission"] = instructions, tools, permission
				if shape != "no agents" {
					agents := map[string]any{"plan": plan}
					if shape == "configured build" {
						buildFields = map[string]any{"temperature": 0.3, "permission": map[string]any{"edit": "ask"}, "description": "Configured build"}
						agents["build"] = buildFields
					}
					config["agent"] = agents
				}
				raw, err := json.Marshal(config)
				if err != nil {
					t.Fatal(err)
				}
				env["OPENCODE_CONFIG_CONTENT"] = string(raw)
				env["OPENCODE_PERMISSION"] = `{"*":"ask"}`
				// Ordinary create/restart call this helper without clearing LUCI.md
				// or supplying an implementation-context bundle.
				if err := prepareLocalOpenCodeCodingContextWith(context.Background(), env, func(context.Context) error { return nil }, func(context.Context, string, string) (int, error) { return 8192, nil }); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config); err != nil {
					t.Fatal(err)
				}
				agents := config["agent"].(map[string]any)
				build := agents["build"].(map[string]any)
				prompt, _ := build["prompt"].(string)
				if prompt != ordinaryLocalOpenCodePrompt || len(prompt) > 1300 || strings.Contains(prompt, "/home/dev/.luci-reviewed-context") || strings.Contains(prompt, "human-reviewed") {
					t.Fatal("ordinary task retained broad defaults or claimed reviewed authority")
				}
				for _, guidance := range []string{"human's current task", "current worktree", "available tools", "allowed edits", "applicable tests", "dependency and network limits", "existing permissions", "After compaction", "without evidence"} {
					if !strings.Contains(prompt, guidance) {
						t.Fatal("ordinary task prompt lost its scope or evidence boundary", guidance)
					}
				}
				delete(build, "prompt")
				if !reflect.DeepEqual(build, buildFields) || !reflect.DeepEqual(config["instructions"], instructions) || !reflect.DeepEqual(config["tools"], tools) || !reflect.DeepEqual(config["permission"], permission) || shape != "no agents" && !reflect.DeepEqual(agents["plan"], plan) {
					t.Fatal("ordinary prompt replaced unrelated agent fields or project/tool policy")
				}
				selection, err := selectedLocalOpenCodeOllama(config)
				if err != nil || selection == nil {
					t.Fatal("ordinary prompt lost the exact selected model", err)
				}
				limit := selection.entry["limit"].(map[string]any)
				if limit["context"] != float64(8192) || limit["output"] != float64(2048) || env["BUN_OPTIONS"] != "--smol" || env["OPENCODE_PERMISSION"] != `{"*":"ask"}` {
					t.Fatal("ordinary prompt changed model limits, runtime profile or approval mode")
				}
			})
		}
	}
}

func TestOrdinaryLocalOpenCodePreservesExplicitBuildPrompt(t *testing.T) {
	for _, provider := range []string{"luci-local", "luci-gateway"} {
		for _, tc := range []struct{ name, prompt string }{{"empty", ""}, {"custom", "Explicit current task policy"}, {"reviewed", reviewedLocalOpenCodePrompt}} {
			t.Run(provider+"/"+tc.name, func(t *testing.T) {
				env := codingModelEnvironment()
				var config map[string]any
				if err := json.Unmarshal([]byte(strings.ReplaceAll(env["OPENCODE_CONFIG_CONTENT"], "luci-local", provider)), &config); err != nil {
					t.Fatal(err)
				}
				build := map[string]any{"prompt": tc.prompt, "temperature": 0.4}
				plan := map[string]any{"prompt": "Other agent"}
				config["agent"] = map[string]any{"build": build, "plan": plan}
				raw, err := json.Marshal(config)
				if err != nil {
					t.Fatal(err)
				}
				env["OPENCODE_CONFIG_CONTENT"] = string(raw)
				if err := prepareLocalOpenCodeCodingContextWith(context.Background(), env, func(context.Context) error { return nil }, func(context.Context, string, string) (int, error) { return 8192, nil }); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config); err != nil {
					t.Fatal(err)
				}
				agents := config["agent"].(map[string]any)
				if !reflect.DeepEqual(agents["build"], build) || !reflect.DeepEqual(agents["plan"], plan) {
					t.Fatal("local default replaced explicit empty/custom/reviewed prompt or other agent fields")
				}
			})
		}
	}
}

func TestOpenCodeLowMemoryProfileRequiresSuccessfulLocalContext(t *testing.T) {
	base := codingModelEnvironment()["OPENCODE_CONFIG_CONTENT"]
	for _, tc := range []struct {
		name             string
		raw              string
		engineErr        error
		modelErr         error
		window           int
		cancelBefore     bool
		cancelDuringRead bool
		wantErr          bool
		wantEngine       bool
		wantModel        bool
	}{
		{name: "malformed", raw: "{"},
		{name: "remote", raw: strings.ReplaceAll(base, "http://host.docker.internal:11434/v1", "https://model.example.invalid/v1")},
		{name: "unselected provider", raw: strings.Replace(base, `"model":"luci-local/`, `"model":"luci-gateway/`, 1)},
		{name: "missing selected alias", raw: strings.Replace(base, `"model":"luci-local/luci-acceptance-qwen2.5-general-8k:latest"`, `"model":"luci-local/other"`, 1), wantErr: true},
		{name: "engine refused", raw: base, engineErr: errors.New("fixture engine unavailable"), wantErr: true, wantEngine: true},
		{name: "context refused", raw: base, modelErr: errors.New("fixture model unavailable"), wantErr: true, wantEngine: true, wantModel: true},
		{name: "unsupported window", raw: base, window: 2048, wantErr: true, wantEngine: true, wantModel: true},
		{name: "already cancelled", raw: base, cancelBefore: true, wantErr: true},
		{name: "cancelled read returns valid window", raw: base, window: 8192, cancelDuringRead: true, wantErr: true, wantEngine: true, wantModel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelBefore {
				cancel()
			}
			env := map[string]string{"OPENCODE_CONFIG_CONTENT": tc.raw, "OPENCODE_PERMISSION": `{"*":"ask"}`}
			before := map[string]string{}
			for key, value := range env {
				before[key] = value
			}
			engineCalled, modelCalled := false, false
			err := prepareLocalOpenCodeCodingContextWith(ctx, env, func(context.Context) error {
				engineCalled = true
				return tc.engineErr
			}, func(context.Context, string, string) (int, error) {
				modelCalled = true
				if tc.cancelDuringRead {
					cancel()
				}
				return tc.window, tc.modelErr
			})
			if (err != nil) != tc.wantErr || engineCalled != tc.wantEngine || modelCalled != tc.wantModel || !reflect.DeepEqual(env, before) {
				t.Fatalf("unaccepted preparation changed the worker environment: err=%v engine=%t model=%t", err, engineCalled, modelCalled)
			}
		})
	}
}

func TestNamedOpenCodeRecognizesOnlyExactLocalOllamaSelection(t *testing.T) {
	for _, endpoint := range []string{"http://localhost:11434/v1", "http://127.0.0.1:11434/v1", "http://[::1]:11434/v1", "http://host.docker.internal:11434/v1"} {
		t.Run(endpoint, func(t *testing.T) {
			var config map[string]any
			raw := strings.ReplaceAll(codingModelEnvironment()["OPENCODE_CONFIG_CONTENT"], "luci-local", "luci-gateway")
			raw = strings.ReplaceAll(raw, "http://host.docker.internal:11434/v1", endpoint)
			if err := json.Unmarshal([]byte(raw), &config); err != nil {
				t.Fatal(err)
			}
			selected, err := selectedLocalOpenCodeOllama(config)
			if err != nil || selected == nil || selected.alias != "luci-acceptance-qwen2.5-general-8k:latest" {
				t.Fatal("exact supported local route was not recognized", err)
			}
		})
	}
	for _, endpoint := range []string{"http://example.invalid:11434/v1", "https://localhost:11434/v1", "http://localhost:4000/v1", "http://localhost:11434/other", "http://localhost:11434/v1?model=other", "http://localhost:11434/v1?", "http://localhost:11434/v1#other", "http://name@localhost:11434/v1", "http://localhost:11434/%76%31"} {
		t.Run(endpoint, func(t *testing.T) {
			env := codingModelEnvironment()
			env["OPENCODE_CONFIG_CONTENT"] = strings.ReplaceAll(strings.ReplaceAll(env["OPENCODE_CONFIG_CONTENT"], "luci-local", "luci-gateway"), "http://host.docker.internal:11434/v1", endpoint)
			if err := clearReviewedOpenCodeInstructions(env); err != nil {
				t.Fatal(err)
			}
			before := env["OPENCODE_CONFIG_CONTENT"]
			err := prepareLocalOpenCodeCodingContextWith(context.Background(), env, func(context.Context) error { t.Fatal("nonlocal route checked local engine"); return nil }, func(context.Context, string, string) (int, error) {
				t.Fatal("nonlocal route probed host model")
				return 0, nil
			})
			if err != nil || env["OPENCODE_CONFIG_CONTENT"] != before || strings.Contains(before, "automatic reminder") || !strings.Contains(before, `"instructions":[]`) {
				t.Fatal("nonlocal named route acquired local context/policy", err)
			}
		})
	}
	t.Run("selected alias missing", func(t *testing.T) {
		env := codingModelEnvironment()
		var config map[string]any
		if err := json.Unmarshal([]byte(strings.ReplaceAll(env["OPENCODE_CONFIG_CONTENT"], "luci-local", "luci-gateway")), &config); err != nil {
			t.Fatal(err)
		}
		config["model"] = "luci-gateway/different-model"
		data, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		env["OPENCODE_CONFIG_CONTENT"] = string(data)
		before := env["OPENCODE_CONFIG_CONTENT"]
		err = prepareLocalOpenCodeCodingContextWith(context.Background(), env, func(context.Context) error { t.Fatal("missing exact model reached engine"); return nil }, func(context.Context, string, string) (int, error) {
			t.Fatal("missing exact model was probed")
			return 0, nil
		})
		if err == nil || env["OPENCODE_CONFIG_CONTENT"] != before {
			t.Fatal("missing selected alias was accepted or mutated")
		}
	})
	t.Run("provider must match selection", func(t *testing.T) {
		env := codingModelEnvironment()
		env["OPENCODE_CONFIG_CONTENT"] = strings.Replace(env["OPENCODE_CONFIG_CONTENT"], `"model":"luci-local/`, `"model":"luci-gateway/`, 1)
		if err := clearReviewedOpenCodeInstructions(env); err != nil {
			t.Fatal(err)
		}
		before := env["OPENCODE_CONFIG_CONTENT"]
		err := prepareLocalOpenCodeCodingContextWith(context.Background(), env, func(context.Context) error { t.Fatal("unselected provider reached engine"); return nil }, func(context.Context, string, string) (int, error) {
			t.Fatal("unselected provider was probed")
			return 0, nil
		})
		if err != nil || before != env["OPENCODE_CONFIG_CONTENT"] || strings.Contains(before, "automatic reminder") {
			t.Fatal("local provider entry overrode a different selected route", err)
		}
	})
	t.Run("legacy remote prompt remains unchanged", func(t *testing.T) {
		env := codingModelEnvironment()
		env["OPENCODE_CONFIG_CONTENT"] = strings.ReplaceAll(env["OPENCODE_CONFIG_CONTENT"], "http://host.docker.internal:11434/v1", "http://example.invalid:4000/v1")
		if err := clearReviewedOpenCodeInstructions(env); err != nil {
			t.Fatal(err)
		}
		before := env["OPENCODE_CONFIG_CONTENT"]
		err := prepareLocalOpenCodeCodingContextWith(context.Background(), env, func(context.Context) error { t.Fatal("legacy remote route reached engine"); return nil }, func(context.Context, string, string) (int, error) {
			t.Fatal("legacy remote route was probed")
			return 0, nil
		})
		if err != nil || before != env["OPENCODE_CONFIG_CONTENT"] || !strings.Contains(before, "automatic reminder") {
			t.Fatal("legacy local-provider prompt behavior changed", err)
		}
	})
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
	if err := prepareLocalOpenCodeCodingContextWith(ctx, env, func(context.Context) error { return nil }, read); err != nil {
		t.Fatal(err)
	}
	var config struct {
		Instructions []string        `json:"instructions"`
		Tools        map[string]bool `json:"tools"`
		Agent        map[string]struct {
			Prompt     string            `json:"prompt"`
			Permission map[string]string `json:"permission"`
		} `json:"agent"`
		Provider map[string]struct {
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
	build := config.Agent["build"]
	if !strings.Contains(build.Prompt, "automatic reminder") || !strings.Contains(build.Prompt, "/home/dev/.luci-reviewed-context") || !strings.Contains(build.Prompt, "file that does not exist yet") || strings.Contains(build.Prompt, "luci-acceptance-qwen") {
		t.Fatal("reviewed local coding prompt lost stable, source-only task guidance")
	}
	for _, tool := range []string{"task", "webfetch", "websearch"} {
		if config.Tools[tool] || build.Permission[tool] != "deny" {
			t.Fatalf("unneeded reviewed local tool %s remains available", tool)
		}
	}
	for _, tool := range []string{"read", "edit", "write", "bash"} {
		if enabled, present := config.Tools[tool]; present && !enabled {
			t.Fatalf("reviewed local coding tool %s was disabled", tool)
		}
		if build.Permission[tool] != "" {
			t.Fatalf("reviewed local coding tool %s changed Ask authority", tool)
		}
	}
	if !strings.Contains(env["OPENCODE_CONFIG_CONTENT"], "{env:OPENCODE_LOCAL_API_KEY}") {
		t.Fatal("lost the private key placeholder")
	}
	ordinary := codingModelEnvironment()
	if err := prepareLocalOpenCodeCodingContextWith(ctx, ordinary, func(context.Context) error { return nil }, func(context.Context, string, string) (int, error) { return 8192, nil }); err != nil {
		t.Fatalf("ordinary bounded 8k session: %v", err)
	}
	if !strings.Contains(ordinary["OPENCODE_CONFIG_CONTENT"], `"instructions":["LUCI.md"]`) {
		t.Fatal("ordinary session lost its project instructions")
	}
	if strings.Contains(ordinary["OPENCODE_CONFIG_CONTENT"], "automatic reminder") {
		t.Fatal("ordinary coding session received reviewed task guidance")
	}
}

func TestReviewedOpenCodeOnlyNarrowsSelectedLocalBuildAgent(t *testing.T) {
	env := codingModelEnvironment()
	var config map[string]any
	if err := json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config); err != nil {
		t.Fatal(err)
	}
	config["tools"] = map[string]any{"edit": true, "webfetch": true}
	config["agent"] = map[string]any{
		"build": map[string]any{"permission": map[string]any{"edit": "ask", "webfetch": "allow"}},
		"plan":  map[string]any{"prompt": "preserved plan route"},
	}
	encoded, _ := json.Marshal(config)
	env["OPENCODE_CONFIG_CONTENT"] = string(encoded)
	if err := clearReviewedOpenCodeInstructions(env); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config); err != nil {
		t.Fatal(err)
	}
	agents := config["agent"].(map[string]any)
	build := agents["build"].(map[string]any)
	permission := build["permission"].(map[string]any)
	tools := config["tools"].(map[string]any)
	if permission["edit"] != "ask" || tools["edit"] != true || permission["webfetch"] != "deny" || tools["webfetch"] != false || agents["plan"].(map[string]any)["prompt"] != "preserved plan route" {
		t.Fatal("reviewed local policy changed unrelated tool, agent, or Ask settings")
	}

	gateway := map[string]string{"OPENCODE_CONFIG_CONTENT": `{"model":"luci-gateway/selected","instructions":["LUCI.md"]}`}
	if err := clearReviewedOpenCodeInstructions(gateway); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(gateway["OPENCODE_CONFIG_CONTENT"], "automatic reminder") || !strings.Contains(gateway["OPENCODE_CONFIG_CONTENT"], `"instructions":[]`) {
		t.Fatal("nonlocal reviewed route acquired local coding policy")
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

func TestOpenCodeContextRefusesUnknownWindowWithoutChangingRoute(t *testing.T) {
	env := codingModelEnvironment()
	before := env["OPENCODE_CONFIG_CONTENT"]
	if err := prepareLocalOpenCodeCodingContextWith(context.Background(), env, func(context.Context) error { return nil }, func(context.Context, string, string) (int, error) { return 0, nil }); err == nil || !strings.Contains(err.Error(), "explicit supported serving context") {
		t.Fatalf("missing alias window accepted: %v", err)
	}
	if env["OPENCODE_CONFIG_CONTENT"] != before {
		t.Fatal("failed preflight changed the saved model route")
	}
}

func TestOpenCodeContextRefusesUnverifiedSelectedEngineBeforeModelProbe(t *testing.T) {
	env := codingModelEnvironment()
	before := env["OPENCODE_CONFIG_CONTENT"]
	called := false
	err := prepareLocalOpenCodeCodingContextWith(context.Background(), env, func(context.Context) error { return errors.New("unverified selected engine") }, func(context.Context, string, string) (int, error) { called = true; return 8192, nil })
	if err == nil || called || env["OPENCODE_CONFIG_CONTENT"] != before {
		t.Fatalf("unverified remote engine reached host model: err=%v called=%t", err, called)
	}
}

func TestOpenCodeEngineIdentityDeadlineAndCallerCancellationPreventDispatch(t *testing.T) {
	check := func(t *testing.T, ctx context.Context, wantEngine bool) {
		t.Helper()
		env := codingModelEnvironment()
		before := env["OPENCODE_CONFIG_CONTENT"]
		engineCalled, modelCalled := false, false
		start := time.Now()
		err := prepareLocalOpenCodeCodingContextWith(ctx, env, func(ctx context.Context) error {
			engineCalled = true
			deadline, ok := ctx.Deadline()
			if !ok || deadline.Sub(start) > localEngineIdentityDeadline+time.Second {
				t.Fatal("selected engine read has no short deadline")
			}
			<-ctx.Done()
			return ctx.Err()
		}, func(context.Context, string, string) (int, error) { modelCalled = true; return 8192, nil })
		if err == nil || engineCalled != wantEngine || modelCalled || env["OPENCODE_CONFIG_CONTENT"] != before {
			t.Fatalf("deadline/cancellation leaked into model or dispatch: err=%v engine=%t model=%t", err, engineCalled, modelCalled)
		}
		if time.Since(start) > localEngineIdentityDeadline+time.Second {
			t.Fatal("engine preflight exceeded its deadline")
		}
	}
	t.Run("identity deadline", func(t *testing.T) { check(t, context.Background(), true) })
	t.Run("caller cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		defer cancel()
		check(t, ctx, false)
	})
	t.Run("caller cancels during identity", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		env := codingModelEnvironment()
		before := env["OPENCODE_CONFIG_CONTENT"]
		modelCalled := false
		err := prepareLocalOpenCodeCodingContextWith(ctx, env, func(child context.Context) error {
			cancel()
			<-child.Done()
			return child.Err()
		}, func(context.Context, string, string) (int, error) { modelCalled = true; return 8192, nil })
		if err == nil || modelCalled || env["OPENCODE_CONFIG_CONTENT"] != before {
			t.Fatalf("cancelled identity reached model metadata: err=%v model=%t", err, modelCalled)
		}
	})
}
