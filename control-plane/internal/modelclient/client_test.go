package modelclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
)

func TestCompleteSendsTheRequiredHeadersAndBody(t *testing.T) {
	var gotPath, gotKey, gotVersion string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"model":"claude-opus-5","stop_reason":"end_turn",
			"content":[{"type":"text","text":"hello"}],
			"usage":{"input_tokens":11,"output_tokens":3}}`))
	}))
	defer srv.Close()

	c, err := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "k-test", Model: "claude-opus-5"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	resp, err := c.Complete(context.Background(), modelclient.Request{
		System: "you are terse",
		User:   "say hello",
		Effort: "medium",
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if gotPath != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", gotPath)
	}
	if gotKey != "k-test" {
		t.Errorf("x-api-key = %q, want k-test", gotKey)
	}
	if gotVersion != "2023-06-01" {
		t.Errorf("anthropic-version = %q, want 2023-06-01", gotVersion)
	}
	if gotBody["model"] != "claude-opus-5" {
		t.Errorf("model = %#v, want claude-opus-5", gotBody["model"])
	}
	msgs, _ := gotBody["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("messages = %#v, want exactly one user turn", gotBody["messages"])
	}
	turn, _ := msgs[0].(map[string]any)
	if turn["role"] != "user" || turn["content"] != "say hello" {
		t.Errorf("messages[0] = %#v, want the user turn \"say hello\"", msgs[0])
	}
	sys, _ := gotBody["system"].([]any)
	if len(sys) != 1 {
		t.Fatalf("system = %#v, want one text block", gotBody["system"])
	}
	if block, _ := sys[0].(map[string]any); block["type"] != "text" || block["text"] != "you are terse" {
		t.Errorf("system[0] = %#v, want the text block \"you are terse\"", sys[0])
	}
	oc, ok := gotBody["output_config"].(map[string]any)
	if !ok || oc["effort"] != "medium" {
		t.Errorf("output_config = %#v, want effort medium", gotBody["output_config"])
	}
	if _, present := oc["format"]; present {
		t.Error("output_config.format must be absent when no schema is supplied")
	}
	if resp.Text != "hello" {
		t.Errorf("Text = %q, want hello", resp.Text)
	}
	if resp.InputTokens != 11 || resp.OutputTokens != 3 {
		t.Errorf("usage = %d/%d, want 11/3", resp.InputTokens, resp.OutputTokens)
	}
}

func TestCompleteSendsJSONSchemaWhenSupplied(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"stop_reason":"end_turn","content":[{"type":"text","text":"{}"}]}`))
	}))
	defer srv.Close()

	c, _ := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "k"})
	if _, err := c.Complete(context.Background(), modelclient.Request{
		User:       "x",
		JSONSchema: json.RawMessage(`{"type":"object"}`),
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	oc := gotBody["output_config"].(map[string]any)
	format, ok := oc["format"].(map[string]any)
	if !ok {
		t.Fatalf("output_config.format missing: %#v", oc)
	}
	if format["type"] != "json_schema" {
		t.Errorf("format.type = %v, want json_schema", format["type"])
	}
	if _, ok := format["schema"].(map[string]any); !ok {
		t.Errorf("format.schema = %#v, want the schema object", format["schema"])
	}
}

// TestCompleteSurfacesHTTPErrors has a 4xx row and a 5xx row: the old plan's
// test had the 401 only, which cannot kill "a client that returns empty text
// on 500", the mutant the part names for it.
func TestCompleteSurfacesHTTPErrors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"401", http.StatusUnauthorized, `{"error":{"message":"invalid x-api-key"}}`, "invalid x-api-key"},
		{"500", http.StatusInternalServerError, `{"error":{"message":"overloaded"}}`, "overloaded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c, _ := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "bad"})
			_, err := c.Complete(context.Background(), modelclient.Request{User: "x"})
			if err == nil {
				t.Fatalf("expected an error for %d", tc.status)
			}
			if !strings.Contains(err.Error(), tc.name) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want the status and the body", err)
			}
		})
	}
}

func TestCompleteReportsRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"stop_reason":"refusal","content":[],"usage":{}}`))
	}))
	defer srv.Close()

	c, _ := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "k"})
	if _, err := c.Complete(context.Background(), modelclient.Request{User: "x"}); !errors.Is(err, modelclient.ErrRefused) {
		t.Fatalf("err = %v, want ErrRefused", err)
	}
}

// TestNewRejectsAMissingKey keeps the old plan's row (a base URL but no key)
// and adds New(Config{}): with both missing, the key check runs first.
func TestNewRejectsAMissingKey(t *testing.T) {
	cases := []struct {
		name string
		cfg  modelclient.Config
	}{
		{"no key, a base URL", modelclient.Config{BaseURL: "http://x"}},
		{"nothing at all", modelclient.Config{}},
		{"a blank key", modelclient.Config{APIKey: "  ", BaseURL: "http://x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := modelclient.New(tc.cfg); !errors.Is(err, modelclient.ErrNoCredentials) {
				t.Fatalf("err = %v, want ErrNoCredentials", err)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare object", `{"a":1}`, `{"a":1}`},
		{"fenced", "here you go:\n```json\n{\"a\":1}\n```\nthanks", `{"a":1}`},
		{"nested", `prose {"a":{"b":2}} tail`, `{"a":{"b":2}}`},
		{"brace inside string", `{"a":"}"}`, `{"a":"}"}`},
		{"escaped quote in string", `{"a":"\"}"}`, `{"a":"\"}"}`},
		// The five rows above each hold exactly one outermost brace pair, so a
		// greedy first-{-to-last-} extractor returns the same bytes for every
		// one of them. This row is the one that tells the two apart.
		{"first of two objects", `{"a":1} and then {"b":2}`, `{"a":1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := modelclient.ExtractJSON(tc.in)
			if err != nil {
				t.Fatalf("ExtractJSON: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestExtractJSONFailsWhenAbsent(t *testing.T) {
	if _, err := modelclient.ExtractJSON("no object here"); !errors.Is(err, modelclient.ErrNoJSON) {
		t.Fatalf("err = %v, want ErrNoJSON", err)
	}
}

// unsetenv removes key for the rest of the test and restores it afterwards.
// t.Setenv registers the restore; the Unsetenv makes the variable absent
// rather than empty, which is what a shell that never exported it gives.
func unsetenv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetenv %s: %v", key, err)
	}
}

// TestConfigFromEnvReadsOnlyPanaceaAppVariables: a developer's shell may hold
// ANTHROPIC_API_KEY for a different account (G4). The key this process sends
// must be one somebody set for Panacea App, never one it happened to find.
func TestConfigFromEnvReadsOnlyPanaceaAppVariables(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-someone-elses")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "tok-someone-elses")
	t.Setenv("PANACEA_APP_MODEL_BASE_URL", "http://gateway.test")
	unsetenv(t, "PANACEA_APP_MODEL_API_KEY")

	if _, err := modelclient.ConfigFromEnv(); !errors.Is(err, modelclient.ErrNoCredentials) {
		t.Fatalf("unset key: err = %v, want ErrNoCredentials", err)
	}

	t.Setenv("PANACEA_APP_MODEL_API_KEY", "   ")
	if _, err := modelclient.ConfigFromEnv(); !errors.Is(err, modelclient.ErrNoCredentials) {
		t.Fatalf("blank key: err = %v, want ErrNoCredentials", err)
	}
}

// TestAKeyWithoutABaseURLIsModelOff: a key alone never sends a brief's answers
// anywhere (I13). There is no default endpoint, in ConfigFromEnv or in New.
func TestAKeyWithoutABaseURLIsModelOff(t *testing.T) {
	t.Setenv("PANACEA_APP_MODEL_API_KEY", "k-mine")
	t.Setenv("ANTHROPIC_BASE_URL", "https://api.anthropic.com")

	unsetenv(t, "PANACEA_APP_MODEL_BASE_URL")
	if _, err := modelclient.ConfigFromEnv(); !errors.Is(err, modelclient.ErrNoBaseURL) {
		t.Errorf("unset base URL: err = %v, want ErrNoBaseURL", err)
	}

	t.Setenv("PANACEA_APP_MODEL_BASE_URL", "  ")
	if _, err := modelclient.ConfigFromEnv(); !errors.Is(err, modelclient.ErrNoBaseURL) {
		t.Errorf("blank base URL: err = %v, want ErrNoBaseURL", err)
	}

	if _, err := modelclient.New(modelclient.Config{APIKey: "k"}); !errors.Is(err, modelclient.ErrNoBaseURL) {
		t.Errorf("New with no base URL: err = %v, want ErrNoBaseURL", err)
	}
	if _, err := modelclient.New(modelclient.Config{APIKey: "k", BaseURL: " \t"}); !errors.Is(err, modelclient.ErrNoBaseURL) {
		t.Errorf("New with a blank base URL: err = %v, want ErrNoBaseURL", err)
	}
}

// TestConfigFromEnvReadsTheThreeVariables: the happy path. The base URL is
// trimmed of spaces and a trailing slash; PANACEA_APP_MODEL_ID overrides the
// default model, and its absence gives DefaultModel.
func TestConfigFromEnvReadsTheThreeVariables(t *testing.T) {
	t.Setenv("PANACEA_APP_MODEL_API_KEY", "k-mine")
	t.Setenv("PANACEA_APP_MODEL_BASE_URL", " https://gateway.test/anthropic/ ")
	t.Setenv("PANACEA_APP_MODEL_ID", "claude-sonnet-5")

	cfg, err := modelclient.ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	want := modelclient.Config{
		BaseURL:   "https://gateway.test/anthropic",
		APIKey:    "k-mine",
		Model:     "claude-sonnet-5",
		MaxTokens: 8192,
		Timeout:   modelclient.DefaultTimeout,
	}
	if cfg != want {
		t.Errorf("cfg = %+v, want %+v", cfg, want)
	}

	unsetenv(t, "PANACEA_APP_MODEL_ID")
	cfg, err = modelclient.ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv without a model id: %v", err)
	}
	if cfg.Model != modelclient.DefaultModel {
		t.Errorf("Model = %q, want DefaultModel %q", cfg.Model, modelclient.DefaultModel)
	}
	if modelclient.DefaultTimeout != 45*time.Second {
		t.Errorf("DefaultTimeout = %v, want 45s (task-02 amendment 5)", modelclient.DefaultTimeout)
	}
}
