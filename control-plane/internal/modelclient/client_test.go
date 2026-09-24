package modelclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
)

func TestCompleteSendsTheRequiredHeadersAndBody(t *testing.T) {
	var gotMethod, gotPath, gotContentType, gotKey, gotVersion string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("content-type")
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

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type = %q, want application/json", gotContentType)
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
	if resp.Model != "claude-opus-5" {
		t.Errorf("Model = %q, want the model the reply names", resp.Model)
	}
	if resp.InputTokens != 11 || resp.OutputTokens != 3 {
		t.Errorf("usage = %d/%d, want 11/3", resp.InputTokens, resp.OutputTokens)
	}
}

// TestNewFillsTheDefaults: a Config with only a key and a base URL (with a
// trailing slash) still posts to /v1/messages, names DefaultModel and asks
// for the default 8192 tokens.
func TestNewFillsTheDefaults(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"stop_reason":"end_turn","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	c, err := modelclient.New(modelclient.Config{BaseURL: srv.URL + "/", APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.Complete(context.Background(), modelclient.Request{User: "x"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if gotPath != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages (the trailing slash trimmed)", gotPath)
	}
	if gotBody["model"] != modelclient.DefaultModel {
		t.Errorf("model = %#v, want DefaultModel %q", gotBody["model"], modelclient.DefaultModel)
	}
	if gotBody["max_tokens"] != float64(8192) {
		t.Errorf("max_tokens = %#v, want 8192", gotBody["max_tokens"])
	}
	if _, present := gotBody["output_config"]; present {
		t.Errorf("output_config = %#v, want absent with no effort and no schema", gotBody["output_config"])
	}
}

// TestNewTrimsTheModelID (review M5): New trims the model id as ConfigFromEnv
// does, so a Config built in code (phase 6's rotation engine builds its own)
// cannot send "  " as the model and fail every call as a 400. Blank gives
// DefaultModel.
func TestNewTrimsTheModelID(t *testing.T) {
	cases := []struct {
		name, model, want string
	}{
		{"blank", " \t", modelclient.DefaultModel},
		{"padded", " claude-sonnet-5 ", "claude-sonnet-5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotBody map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewDecoder(r.Body).Decode(&gotBody)
				_, _ = w.Write([]byte(`{"stop_reason":"end_turn","content":[{"type":"text","text":"ok"}]}`))
			}))
			defer srv.Close()

			c, err := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "k", Model: tc.model})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if _, err := c.Complete(context.Background(), modelclient.Request{User: "x"}); err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if gotBody["model"] != tc.want {
				t.Errorf("model = %#v, want %q", gotBody["model"], tc.want)
			}
		})
	}
}

// TestCompleteReportsNoTextBlock: a 200 whose content holds no text block (a
// reply that spent its tokens thinking, or an empty one) is ErrNoText, never
// a success with empty text.
func TestCompleteReportsNoTextBlock(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"thinking only", `{"stop_reason":"max_tokens","content":[{"type":"thinking","thinking":"","signature":"sig"}]}`},
		{"empty content", `{"stop_reason":"end_turn","content":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c, _ := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "k"})
			resp, err := c.Complete(context.Background(), modelclient.Request{User: "x"})
			if !errors.Is(err, modelclient.ErrNoText) {
				t.Fatalf("err = %v (resp %+v), want ErrNoText", err, resp)
			}
		})
	}
}

// TestCompleteBoundsTheReplyItReads: the reply is read through an 8 MiB
// limit, so a runaway body is a decode error, not an unbounded allocation.
// Both sides of the bound are pinned to the byte (review M1): a body of
// exactly 8 MiB is read whole, and one byte more is an error. The upper row
// alone let a shift typo (8<<10) cap every reply at 8 KiB.
func TestCompleteBoundsTheReplyItReads(t *testing.T) {
	const limit = 8 << 20
	const head, tail = `{"stop_reason":"end_turn","content":[{"type":"text","text":"`, `"}]}`
	cases := []struct {
		name   string
		size   int // the whole body, in bytes
		accept bool
	}{
		{"a body of exactly 8 MiB is read whole", limit, true},
		{"a body one byte over 8 MiB is an error", limit + 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Repeat("a", tc.size-len(head)-len(tail))
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(head + text + tail))
			}))
			defer srv.Close()

			c, _ := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "k"})
			resp, err := c.Complete(context.Background(), modelclient.Request{User: "x"})
			switch {
			case tc.accept && err != nil:
				t.Fatalf("a %d-byte body was refused: %v", tc.size, err)
			case tc.accept && len(resp.Text) != len(text):
				t.Fatalf("Text is %d bytes, want the whole %d", len(resp.Text), len(text))
			case !tc.accept && err == nil:
				t.Fatalf("a %d-byte body was accepted (%d bytes of text)", tc.size, len(resp.Text))
			}
		})
	}
}

// TestCompleteHonoursTheCallersContext: the caller's context is the budget
// that keeps each Discover call under its own desktop timeout (task 02
// amendment 5, review I4, review I1). DefaultTimeout (45 s) is only a
// ceiling, longer than task 04's FollowUpBudget (20 s) and than the desktop's
// 30 s wait, so a client that dropped ctx would turn a slow model into the
// desktop's timeout. Tasks 04 and 13 test with a fake Completer; only this
// package can pin that the real Client obeys ctx. The handler holds the call
// open until the client goes away, so only the caller's context can end it
// inside the bound.
func TestCompleteHonoursTheCallersContext(t *testing.T) {
	cases := []struct {
		name string
		want error
		ctx  func(started <-chan struct{}) (context.Context, context.CancelFunc)
	}{
		{"a deadline", context.DeadlineExceeded, func(<-chan struct{}) (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), 50*time.Millisecond)
		}},
		{"a cancel once the call is in flight", context.Canceled, func(started <-chan struct{}) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				select {
				case <-started:
					cancel()
				case <-ctx.Done():
				}
			}()
			return ctx, cancel
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				once.Do(func() { close(started) })
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer srv.Close()
			// Deferred after Close, so it runs first: a handler still held
			// open (the failing case) is released and cannot hang Close.
			defer close(release)

			c, err := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "k"})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			ctx, cancel := tc.ctx(started)
			defer cancel()

			done := make(chan error, 1)
			go func() {
				_, err := c.Complete(ctx, modelclient.Request{User: "x"})
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, tc.want) {
					t.Fatalf("err = %v, want one that wraps %v", err, tc.want)
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("Complete still running 10 s after the caller's context ended: only the client's own %v ceiling would end it", modelclient.DefaultTimeout)
			}
		})
	}
}

// TestCompleteSendsJSONSchemaWhenSupplied sends an effort with the schema
// (review M6): both Discover callers send a schema, and an effort set beside
// it must reach the body too, or it is silently ignored and the call runs at
// the model's default inside a 20 s budget.
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
		Effort:     "low",
		JSONSchema: json.RawMessage(`{"type":"object"}`),
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	oc := gotBody["output_config"].(map[string]any)
	if oc["effort"] != "low" {
		t.Errorf("output_config.effort = %#v, want low beside the schema", oc["effort"])
	}
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

// TestCompleteReportsRefusal has a row with partial text (review M2): a
// refusal can fire after some output, and that output is discarded, never
// returned as a complete answer. The refusal check must run before the text
// is read, not only when there is none.
func TestCompleteReportsRefusal(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"no content", `{"stop_reason":"refusal","content":[],"usage":{}}`},
		{"partial text", `{"stop_reason":"refusal","content":[{"type":"text","text":"partial"}],"usage":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			c, _ := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "k"})
			resp, err := c.Complete(context.Background(), modelclient.Request{User: "x"})
			if !errors.Is(err, modelclient.ErrRefused) {
				t.Fatalf("err = %v (Text %q), want ErrRefused", err, resp.Text)
			}
			if resp.Text != "" {
				t.Errorf("Text = %q, want nothing from a refused reply", resp.Text)
			}
		})
	}
}

// TestCompleteJoinsEveryTextBlock: the reply's text is every text block, in
// order, with the thinking block skipped. The API splits text into several
// blocks (with citations, for one), and a client that kept only the first
// block would return part of the answer as the whole.
func TestCompleteJoinsEveryTextBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"stop_reason":"end_turn","content":[
			{"type":"thinking","thinking":"hmm","signature":"sig"},
			{"type":"text","text":"a"},{"type":"text","text":"b"}]}`))
	}))
	defer srv.Close()

	c, _ := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "k"})
	resp, err := c.Complete(context.Background(), modelclient.Request{User: "x"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Text != "ab" {
		t.Errorf("Text = %q, want \"ab\", both text blocks in order", resp.Text)
	}
}

// TestCompleteCapsTheBodyItQuotesInAnError: a failed reply's body is quoted
// in the error, and an error lands in logs. The body can be a gateway's HTML
// page or an echo of the request, which carries a brief's answers, so at most
// 2 KiB of it is quoted, not the 8 MiB the read allows.
func TestCompleteCapsTheBodyItQuotesInAnError(t *testing.T) {
	const quoted = 2 << 10
	body := strings.Repeat("a", 64<<10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c, _ := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "k"})
	_, err := c.Complete(context.Background(), modelclient.Request{User: "x"})
	if err == nil {
		t.Fatal("expected an error for 502")
	}
	msg := err.Error()
	if !strings.Contains(msg, "502") {
		t.Errorf("error = %.80q..., want the status", msg)
	}
	if !strings.Contains(msg, body[:quoted]) || strings.Contains(msg, body[:quoted+1]) {
		t.Errorf("error quotes %d bytes of the body, want exactly the first %d", strings.Count(msg, "a"), quoted)
	}
	if !strings.Contains(msg, "65536 bytes") {
		t.Errorf("error does not name the body's full length, 65536 bytes (it is %d bytes long)", len(msg))
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
		// A stray close brace before the object must not unbalance the count:
		// without the depth > 0 guard, this returns the INNER {"b":1}.
		{"a stray close brace first", `}{"a":{"b":1}}`, `{"a":{"b":1}}`},
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
