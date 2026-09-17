# Phase 4: Discover Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A person answers five questions about a branch, one at a time, and a `CLAUDE.md` appears in that branch's worktree that every agent session started there genuinely reads. The same brief is then visible as a context chip in Develop, as the definition of done in Merge, and as what shipped in Deploy — one artifact, one thread, three surfaces.

**Architecture:** Four Go packages in `control-plane` with no shared state — `discover` (the question flow and inheritance, pure logic), `claudemd` (rendering and merging the managed block), `contextwrite` (the atomic write into a worktree), `repoanalysis` (mechanical repo scan) — plus `modelclient` (one HTTP call to the Messages API), `discoverstore` (JSON-file persistence) and `discoverapi` (HTTP). The desktop renders the flow and the three provenance surfaces. The model is an *improvement* to the flow, never a gate on it: every model-assisted path degrades to a static one.

**Tech Stack:** Go 1.23 (standard library only — no new Go dependencies), React 19 + TypeScript + Vite 8, Vitest 5, Testing Library 16, Tauri 2.

**Spec:** `docs/PROGRAM.md` (Phase 4), `CLAUDE.md`, `docs/FUNCTIONAL-SPEC.md`, `docs/DESIGN-SYSTEM.md`, `docs/AGENTIC-UX.md`, `prototype/app.html` (the approved interaction).

---

## Global Constraints

- Go module path prefix: `github.com/kpmg-au-emu/panacea-app`.
- **`go test ./...` is invalid at this workspace root.** Always `go test ./sandboxd/... ./control-plane/... -race -count=1`.
- **The `go` directive stays `1.23`** in every `go.mod` and in `go.work`. CI pins 1.23; a higher directive makes a network-restricted agent attempt a toolchain download and fail.
- **This phase adds zero Go dependencies.** See Task 1 for why the official Anthropic Go SDK is deliberately not used — it is a verified hard blocker, not a preference.
- Agent CLIs are **never** parsed, wrapped or reimplemented. Discover writes a file the CLI reads; it does not read the CLI.
- **No progress bars, percentages, ETAs, or "N of M met"** anywhere (`docs/AGENTIC-UX.md` anti-pattern 2). The one permitted denominator is `QUESTION n OF m` over the closed base question set, and Task 2 explains why that one is defensible.
- **AI capacity compliance** (`CLAUDE.md`): the model key is the signed-in user's **own** API key or a corporate gateway key. Pooled Claude subscriptions are forbidden and blocked server-side. Phase 6 replaces the credential source; the `modelclient.Completer` interface does not change.
- Shell is Windows. Use PowerShell for anything touching Docker or paths. Beware the pipe exit-code trap: `cmd | head` reports `head`'s status.
- Every commit message ends with:
  ```
  Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
  ```

---

## Phase seams — read before Task 1

Three contracts cross a phase boundary. Each is restated inside the relevant task's **Interfaces** block and in a doc comment in the code, because Phase 1's only Critical defect was invisible inside either task alone.

| Seam | Phase 4 owns | The other phase owns | Where it is stated |
|---|---|---|---|
| **Worktrees** | The content of `CLAUDE.md` and the act of writing it | Phase 3: creating, listing and removing worktrees | Task 5, `contextwrite.WorktreeResolver` |
| **Persistence** | The `discoverstore.Store` interface and a file-backed implementation | Phase 2: the app database | Task 7, `discoverstore.Store` |
| **Names and shell** | The `discoverapi.Naming` interface and the Discover/provenance React components | Phase 2: the project model, the rail, the sub-rail, where components mount | Tasks 9, 11, 12 |

**SEAM HAZARD, the one that will bite:** `CLAUDE.md` must be on disk **before** the session's PTY starts. Claude Code reads it at session start. Write it afterwards and the session inherits nothing, silently, with no error anywhere — the feature looks built and is not. Phase 3's session-start path calls `Materialise` before `ptysession.Start`, never after. This sentence is repeated in `contextwrite`'s package comment and must be repeated in the Phase 3 plan.

---

## File Structure

| Path | Responsibility |
|---|---|
| `control-plane/internal/modelclient/client.go` | One HTTP call to `POST /v1/messages`. No SDK. |
| `control-plane/internal/discover/flow.go` | Questions, answers, edit-in-place, honest step |
| `control-plane/internal/discover/assist.go` | Model-tailored options and at most one follow-up |
| `control-plane/internal/discover/inherit.go` | project → branch resolution into a `Document` |
| `control-plane/internal/claudemd/render.go` | Deterministic managed block; merge preserving user text |
| `control-plane/internal/contextwrite/writer.go` | Atomic write into a worktree. **Phase 3 seam.** |
| `control-plane/internal/repoanalysis/scan.go` | Mechanical repo scan |
| `control-plane/internal/repoanalysis/draft.go` | Model drafts a brief from the scan |
| `control-plane/internal/discoverstore/store.go` | One JSON file per brief. **Phase 2 seam.** |
| `control-plane/internal/provenance/provenance.go` | The one type Develop, Merge and Deploy share |
| `control-plane/internal/discoverapi/api.go` | HTTP routes under `/api/v1/discover` |
| `desktop/src/lib/discoverApi.ts` | TypeScript mirror of the JSON above |
| `desktop/src/discover/DiscoverView.tsx` | The question flow surface |
| `desktop/src/discover/ImportRepo.tsx` | Analyse → draft → correct → choose where to start |
| `desktop/src/provenance/*.tsx` | Context chip, definition of done, what shipped |

---

### Task 1: The model client

Discover is the first phase that genuinely needs a model. This package is the only place in the product that talks to one.

**Why not `github.com/anthropics/anthropic-sdk-go` — verified, not assumed.** Its `go.mod` at `v1.73.0` declares `go 1.24` and `toolchain go1.25.8`. Every module here is pinned to `go 1.23` to match the CI toolchain. A 1.23 build agent asked to compile a 1.24 module either fails outright or tries to download a newer toolchain, which the corporate proxy blocks — the exact failure Phase 1 hit. The subset needed here is one endpoint and three headers, so it is written directly. Revisit when CI moves past 1.24.

**Files:**
- Create: `control-plane/internal/modelclient/client.go`
- Test: `control-plane/internal/modelclient/client_test.go`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `modelclient.Config{BaseURL, APIKey, Model string; MaxTokens int; Timeout time.Duration}`
  - `modelclient.ConfigFromEnv() (Config, error)`
  - `modelclient.New(cfg Config) (*Client, error)`
  - `modelclient.Request{System, User, Effort string; JSONSchema json.RawMessage}`
  - `modelclient.Response{Text, Model string; InputTokens, OutputTokens int}`
  - `modelclient.Completer` interface with `Complete(ctx context.Context, req Request) (Response, error)` — **this is the seam Phase 6's rotation engine replaces**
  - `modelclient.ExtractJSON(text string) ([]byte, error)`
  - `modelclient.ErrNoCredentials`, `ErrRefused`, `ErrNoText`, `ErrNoJSON`
  - `modelclient.DefaultBaseURL = "https://api.anthropic.com"`, `DefaultModel = "claude-opus-5"`

- [ ] **Step 1: Write the failing tests**

Create `control-plane/internal/modelclient/client_test.go`:

```go
package modelclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestCompleteSurfacesHTTPErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid x-api-key"}}`))
	}))
	defer srv.Close()

	c, _ := modelclient.New(modelclient.Config{BaseURL: srv.URL, APIKey: "bad"})
	_, err := c.Complete(context.Background(), modelclient.Request{User: "x"})
	if err == nil {
		t.Fatal("expected an error for 401")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Errorf("error = %q, want the status and the body", err)
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

func TestNewRejectsAMissingKey(t *testing.T) {
	if _, err := modelclient.New(modelclient.Config{BaseURL: "http://x"}); !errors.Is(err, modelclient.ErrNoCredentials) {
		t.Fatalf("err = %v, want ErrNoCredentials", err)
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/modelclient/...`
Expected: FAIL — package `modelclient` does not exist.

- [ ] **Step 3: Write the implementation**

Create `control-plane/internal/modelclient/client.go`:

```go
// Package modelclient calls the Anthropic Messages API over plain HTTP.
//
// COMPLIANCE (CLAUDE.md, "AI capacity"): the key this client sends must be the
// signed-in user's OWN API key, or a corporate gateway key issued to them.
// A pooled Claude subscription OAuth token is forbidden — it breaches
// Anthropic's Consumer Terms and is blocked server-side. Phase 6 replaces
// ConfigFromEnv with the rotation engine but keeps the Completer interface.
//
// WHY RAW HTTP AND NOT github.com/anthropics/anthropic-sdk-go: that module's
// go.mod (v1.73.0) declares `go 1.24` and `toolchain go1.25.8`. Every module
// here is pinned to `go 1.23` to match the CI toolchain, and a 1.23 build agent
// asked to compile a 1.24 module either fails outright or tries to download a
// newer toolchain — which the corporate proxy blocks. Revisit when CI moves
// past 1.24. The subset used here is one endpoint and three headers.
package modelclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultBaseURL is Anthropic's first-party API. A gateway (for example the
// KPMG Kelios gateway) is configured by overriding it; the path /v1/messages
// is appended, so the value must be an origin, not a full endpoint.
const DefaultBaseURL = "https://api.anthropic.com"

// DefaultModel is the model every Discover call uses unless overridden.
const DefaultModel = "claude-opus-5"

// apiVersion is the required anthropic-version header value.
const apiVersion = "2023-06-01"

// Errors returned by this package.
var (
	ErrNoCredentials = errors.New("modelclient: PANACEA_APP_MODEL_API_KEY is not set")
	ErrRefused       = errors.New("modelclient: the model declined the request")
	ErrNoText        = errors.New("modelclient: response contained no text block")
	ErrNoJSON        = errors.New("modelclient: no JSON object found in response text")
)

// Config describes how to reach a model.
type Config struct {
	BaseURL   string
	APIKey    string
	Model     string
	MaxTokens int
	Timeout   time.Duration
}

// ConfigFromEnv reads PANACEA_APP_MODEL_BASE_URL, PANACEA_APP_MODEL_API_KEY
// and PANACEA_APP_MODEL_ID. Only the key is mandatory.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		BaseURL:   strings.TrimSuffix(getenv("PANACEA_APP_MODEL_BASE_URL", DefaultBaseURL), "/"),
		APIKey:    os.Getenv("PANACEA_APP_MODEL_API_KEY"),
		Model:     getenv("PANACEA_APP_MODEL_ID", DefaultModel),
		MaxTokens: 8192,
		Timeout:   120 * time.Second,
	}
	if cfg.APIKey == "" {
		return Config{}, ErrNoCredentials
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Request is one non-streaming completion.
type Request struct {
	// System is the system prompt. Sent as a single text block.
	System string
	// User is the single user turn.
	User string
	// Effort maps to output_config.effort: low, medium, high, xhigh or max.
	// Empty means the server default (high).
	Effort string
	// JSONSchema, when non-nil, is sent as output_config.format so the reply
	// is constrained to that schema. The raw text is still returned.
	JSONSchema json.RawMessage
}

// Response is the useful part of one reply.
type Response struct {
	Text         string
	Model        string
	InputTokens  int
	OutputTokens int
}

// Completer is the seam every Discover feature depends on. Tests substitute a
// deterministic fake; Phase 6 substitutes the rotation engine.
type Completer interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

// Client is the HTTP implementation of Completer.
type Client struct {
	cfg  Config
	http *http.Client
}

// New returns a Client. It does not contact the network.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, ErrNoCredentials
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	cfg.BaseURL = strings.TrimSuffix(cfg.BaseURL, "/")
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 8192
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}, nil
}

type wireSystem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type wireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type wireFormat struct {
	Type   string          `json:"type"`
	Schema json.RawMessage `json:"schema"`
}

type wireOutputConfig struct {
	Effort string      `json:"effort,omitempty"`
	Format *wireFormat `json:"format,omitempty"`
}

type wireRequest struct {
	Model        string            `json:"model"`
	MaxTokens    int               `json:"max_tokens"`
	System       []wireSystem      `json:"system,omitempty"`
	Messages     []wireMessage     `json:"messages"`
	OutputConfig *wireOutputConfig `json:"output_config,omitempty"`
}

type wireContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type wireUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type wireResponse struct {
	Model      string             `json:"model"`
	StopReason string             `json:"stop_reason"`
	Content    []wireContentBlock `json:"content"`
	Usage      wireUsage          `json:"usage"`
}

// Complete performs one non-streaming POST /v1/messages.
func (c *Client) Complete(ctx context.Context, req Request) (Response, error) {
	body := wireRequest{
		Model:     c.cfg.Model,
		MaxTokens: c.cfg.MaxTokens,
		Messages:  []wireMessage{{Role: "user", Content: req.User}},
	}
	if req.System != "" {
		body.System = []wireSystem{{Type: "text", Text: req.System}}
	}
	if req.Effort != "" || req.JSONSchema != nil {
		oc := &wireOutputConfig{Effort: req.Effort}
		if req.JSONSchema != nil {
			oc.Format = &wireFormat{Type: "json_schema", Schema: req.JSONSchema}
		}
		body.OutputConfig = oc
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return Response{}, fmt.Errorf("modelclient: encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.BaseURL+"/v1/messages", bytes.NewReader(encoded))
	if err != nil {
		return Response{}, fmt.Errorf("modelclient: build request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", c.cfg.APIKey)
	httpReq.Header.Set("anthropic-version", apiVersion)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("modelclient: call model: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return Response{}, fmt.Errorf("modelclient: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("modelclient: model returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var decoded wireResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return Response{}, fmt.Errorf("modelclient: decode response: %w", err)
	}
	if decoded.StopReason == "refusal" {
		return Response{}, ErrRefused
	}

	var text strings.Builder
	for _, block := range decoded.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	if text.Len() == 0 {
		return Response{}, ErrNoText
	}
	return Response{
		Text:         text.String(),
		Model:        decoded.Model,
		InputTokens:  decoded.Usage.InputTokens,
		OutputTokens: decoded.Usage.OutputTokens,
	}, nil
}

// ExtractJSON returns the first balanced top-level JSON object in text.
// output_config.format constrains first-party Anthropic replies, but a gateway
// may ignore it and wrap the object in prose or a fenced code block, so every
// JSON-shaped call goes through here rather than straight into json.Unmarshal.
func ExtractJSON(text string) ([]byte, error) {
	depth, start, inString, escaped := 0, -1, false, false
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth > 0 {
				depth--
				if depth == 0 && start >= 0 {
					return []byte(text[start : i+1]), nil
				}
			}
		}
	}
	return nil, ErrNoJSON
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./control-plane/internal/modelclient/... -v -race -count=1`
Expected: PASS — 6 top-level tests, 5 sub-tests under `TestExtractJSON`.

- [ ] **Step 5: Confirm no dependency crept in**

Run: `git diff --stat control-plane/go.mod control-plane/go.sum`
Expected: no output. If either file changed, a dependency was added — remove it and use the standard library.

- [ ] **Step 6: Commit**

```bash
git add control-plane/internal/modelclient
git commit -m "feat(modelclient): messages api over plain http, no sdk

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: The question flow

Pure logic, no I/O, no model. This is where the prototype's worst bug is killed.

**On the one permitted denominator.** `docs/AGENTIC-UX.md` anti-pattern 2 bans progress bars, percentages, ETAs and "N of M met". `QUESTION 2 OF 5` survives that ban for one reason: the base question set is a **closed list that cannot grow mid-flow**, so 5 is a denominator the system can defend. "2 of 4 criteria met" is banned because the system cannot defend "met". Model-proposed follow-ups are appended *without* a denominator — `Step.Total` is zero for them — precisely so adding one never makes the "OF 5" a lie. A bar or a percentage is still forbidden; the only legal rendering is the literal count.

**Files:**
- Create: `control-plane/internal/discover/flow.go`
- Test: `control-plane/internal/discover/flow_test.go`

**Interfaces:**
- Consumes: nothing
- Produces:
  - `discover.Scope` with `ScopeProject`, `ScopeBranch`
  - `discover.Key` with `KeyPurpose`, `KeyAudience`, `KeyConstraints`, `KeyDone`, `KeyOutOfScope`
  - `discover.Question{Key, ProjectPrompt, BranchPrompt, Help string; Options []string; FollowUp bool}` and `(Question).Prompt(Scope) string`
  - `discover.BaseQuestions() []Question`
  - `discover.Source` with `SourcePicked`, `SourceTyped`, `SourceDrafted`
  - `discover.Answer{Key, Prompt, Text, Source, AnsweredAt}`
  - `discover.Status` with `StatusOpen`, `StatusComplete`, `StatusSkipped`
  - `discover.Brief` and `discover.NewBrief(id string, scope Scope, projectID, branchID string, now time.Time) *Brief`
  - `discover.Step{Kind, Ordinal, Total, Question, Prompt}` and `StepQuestion`, `StepFollowUp`, `StepComplete`
  - `(*Brief).Step() Step`, `.Record(Key, string, Source, time.Time) error`, `.Edit(Key, string, time.Time) error`, `.Skip(time.Time)`, `.Reopen(time.Time)`, `.AddFollowUp(Question, time.Time) error`, `.AnswerFor(Key) (Answer, bool)`, `.Answered(Key) bool`
  - `discover.SplitCriteria(text string) []string`
  - `discover.ErrUnknownKey`, `ErrEmptyAnswer`, `ErrNotAnswered`

- [ ] **Step 1: Write the failing tests**

Create `control-plane/internal/discover/flow_test.go`:

```go
package discover_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

var clock = time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)

func newBranchBrief(t *testing.T) *discover.Brief {
	t.Helper()
	return discover.NewBrief("b-1", discover.ScopeBranch, "p-1", "br-1", clock)
}

func TestStepWalksTheClosedQuestionSetInOrder(t *testing.T) {
	b := newBranchBrief(t)

	step := b.Step()
	if step.Kind != discover.StepQuestion {
		t.Fatalf("kind = %q, want question", step.Kind)
	}
	if step.Ordinal != 1 || step.Total != 5 {
		t.Errorf("ordinal/total = %d/%d, want 1/5", step.Ordinal, step.Total)
	}
	if step.Question.Key != discover.KeyPurpose {
		t.Errorf("first key = %q, want purpose", step.Question.Key)
	}
	if step.Prompt != "What is this branch for?" {
		t.Errorf("prompt = %q, want the branch wording", step.Prompt)
	}

	if err := b.Record(discover.KeyPurpose, "Fix something broken", discover.SourcePicked, clock); err != nil {
		t.Fatalf("Record: %v", err)
	}
	step = b.Step()
	if step.Ordinal != 2 || step.Question.Key != discover.KeyAudience {
		t.Errorf("second step = %d/%q, want 2/audience", step.Ordinal, step.Question.Key)
	}
}

func TestProjectScopeUsesTheProjectWording(t *testing.T) {
	b := discover.NewBrief("p-brief", discover.ScopeProject, "p-1", "", clock)
	if got := b.Step().Prompt; got != "What is this project for?" {
		t.Errorf("prompt = %q, want the project wording", got)
	}
}

func TestBriefCompletesAfterTheLastBaseQuestion(t *testing.T) {
	b := newBranchBrief(t)
	for _, q := range discover.BaseQuestions() {
		if err := b.Record(q.Key, "answer for "+string(q.Key), discover.SourceTyped, clock); err != nil {
			t.Fatalf("Record %s: %v", q.Key, err)
		}
	}
	if b.Status != discover.StatusComplete {
		t.Errorf("status = %q, want complete", b.Status)
	}
	if b.Step().Kind != discover.StepComplete {
		t.Errorf("step kind = %q, want complete", b.Step().Kind)
	}
	if b.Step().Total != 0 {
		t.Errorf("complete step total = %d, want 0 — there is no denominator to show", b.Step().Total)
	}
}

// This is the regression test for docs/FUNCTIONAL-SPEC.md known defect 7:
// prototype/app.html's editAnswer spliced the answer array and destroyed every
// answer after the one being edited.
func TestEditChangesOneAnswerAndKeepsTheRest(t *testing.T) {
	b := newBranchBrief(t)
	for _, q := range discover.BaseQuestions() {
		if err := b.Record(q.Key, "answer for "+string(q.Key), discover.SourceTyped, clock); err != nil {
			t.Fatalf("Record %s: %v", q.Key, err)
		}
	}

	later := clock.Add(time.Minute)
	if err := b.Edit(discover.KeyAudience, "Client users only", later); err != nil {
		t.Fatalf("Edit: %v", err)
	}

	if len(b.Answers) != 5 {
		t.Fatalf("answers = %d, want 5 — editing must not drop later answers", len(b.Answers))
	}
	for i, want := range []discover.Key{
		discover.KeyPurpose, discover.KeyAudience, discover.KeyConstraints,
		discover.KeyDone, discover.KeyOutOfScope,
	} {
		if b.Answers[i].Key != want {
			t.Errorf("answers[%d].Key = %q, want %q — order must be stable", i, b.Answers[i].Key, want)
		}
	}
	edited, _ := b.AnswerFor(discover.KeyAudience)
	if edited.Text != "Client users only" {
		t.Errorf("edited text = %q", edited.Text)
	}
	if last, _ := b.AnswerFor(discover.KeyOutOfScope); last.Text != "answer for out_of_scope" {
		t.Errorf("last answer = %q, want it untouched", last.Text)
	}
	if b.Status != discover.StatusComplete {
		t.Errorf("status = %q, want the brief to stay complete after an edit", b.Status)
	}
}

func TestEditRejectsAnUnansweredQuestion(t *testing.T) {
	b := newBranchBrief(t)
	if err := b.Edit(discover.KeyDone, "x", clock); !errors.Is(err, discover.ErrNotAnswered) {
		t.Fatalf("err = %v, want ErrNotAnswered", err)
	}
}

func TestRecordRejectsEmptyAnswersAndUnknownKeys(t *testing.T) {
	b := newBranchBrief(t)
	if err := b.Record(discover.KeyPurpose, "   ", discover.SourceTyped, clock); !errors.Is(err, discover.ErrEmptyAnswer) {
		t.Fatalf("err = %v, want ErrEmptyAnswer", err)
	}
	if err := b.Record(discover.Key("nope"), "x", discover.SourceTyped, clock); !errors.Is(err, discover.ErrUnknownKey) {
		t.Fatalf("err = %v, want ErrUnknownKey", err)
	}
}

func TestFollowUpsCarryNoDenominator(t *testing.T) {
	b := newBranchBrief(t)
	for _, q := range discover.BaseQuestions() {
		_ = b.Record(q.Key, "answer", discover.SourceTyped, clock)
	}
	err := b.AddFollowUp(discover.Question{
		Key:          discover.Key("rollout"),
		BranchPrompt: "How does this reach users?",
		Help:         "You said this changes a published API.",
		Options:      []string{"Behind a flag", "All at once"},
	}, clock)
	if err != nil {
		t.Fatalf("AddFollowUp: %v", err)
	}

	step := b.Step()
	if step.Kind != discover.StepFollowUp {
		t.Fatalf("kind = %q, want followUp", step.Kind)
	}
	if step.Total != 0 {
		t.Errorf("total = %d, want 0 — a follow-up has no honest denominator", step.Total)
	}
	if b.Status != discover.StatusOpen {
		t.Errorf("status = %q, want the brief reopened", b.Status)
	}
}

func TestAddFollowUpRejectsADuplicateKey(t *testing.T) {
	b := newBranchBrief(t)
	if err := b.AddFollowUp(discover.Question{Key: discover.KeyDone, BranchPrompt: "x"}, clock); err == nil {
		t.Fatal("expected an error for a duplicate key")
	}
}

func TestSkipClosesTheBriefWithoutDiscardingAnswers(t *testing.T) {
	b := newBranchBrief(t)
	_ = b.Record(discover.KeyPurpose, "Fix the uploader", discover.SourcePicked, clock)
	b.Skip(clock)

	if b.Status != discover.StatusSkipped {
		t.Errorf("status = %q, want skipped", b.Status)
	}
	if b.Step().Kind != discover.StepComplete {
		t.Errorf("step = %q, want complete", b.Step().Kind)
	}
	if len(b.Answers) != 1 {
		t.Errorf("answers = %d, want the one already given to survive", len(b.Answers))
	}
}

func TestSplitCriteria(t *testing.T) {
	got := discover.SplitCriteria("- Tests pass\n- No API break;  Someone reviews it\n\n")
	want := []string{"Tests pass", "No API break", "Someone reviews it"}
	if len(got) != len(want) {
		t.Fatalf("got %d items %q, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("item %d = %q, want %q", i, got[i], want[i])
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/discover/...`
Expected: FAIL — package `discover` does not exist.

- [ ] **Step 3: Write the types and the question set**

Create `control-plane/internal/discover/flow.go` with this first half:

```go
// Package discover turns intent into durable context. Its output is a
// CLAUDE.md written into a worktree, which Claude Code reads at session start —
// that file is the inheritance mechanism, not a metaphor for one.
package discover

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Scope is the level a brief belongs to. A branch brief inherits the project
// brief and overrides it key by key.
type Scope string

const (
	ScopeProject Scope = "project"
	ScopeBranch  Scope = "branch"
)

// Key identifies one question. Keys are stable: they are the join between a
// project answer and the branch answer that overrides it, and they are the
// section order in the generated CLAUDE.md.
type Key string

const (
	KeyPurpose     Key = "purpose"
	KeyAudience    Key = "audience"
	KeyConstraints Key = "constraints"
	KeyDone        Key = "done"
	KeyOutOfScope  Key = "out_of_scope"
)

// Question is one step of the flow. Options are pickable answers; the free-text
// escape is always available and is not modelled here because it accepts
// anything.
type Question struct {
	Key           Key      `json:"key"`
	ProjectPrompt string   `json:"projectPrompt"`
	BranchPrompt  string   `json:"branchPrompt"`
	Help          string   `json:"help"`
	Options       []string `json:"options"`
	// FollowUp marks a question the model added on top of the base set. A
	// follow-up has no defensible denominator, so Step.Total is zero for it.
	FollowUp bool `json:"followUp"`
}

// Prompt returns the wording for a scope.
func (q Question) Prompt(s Scope) string {
	if s == ScopeProject && q.ProjectPrompt != "" {
		return q.ProjectPrompt
	}
	return q.BranchPrompt
}

// BaseQuestions is the closed set every brief starts from. It is closed on
// purpose: "QUESTION 2 OF 5" is only honest because this list cannot grow
// mid-flow. Model-proposed follow-ups are appended and shown without a
// denominator (docs/AGENTIC-UX.md anti-pattern 2).
func BaseQuestions() []Question {
	return []Question{
		{
			Key:           KeyPurpose,
			ProjectPrompt: "What is this project for?",
			BranchPrompt:  "What is this branch for?",
			Help:          "One sentence. The first thing every agent here reads.",
			Options: []string{
				"Add a capability we do not have yet",
				"Fix something broken",
				"Change how something works",
				"Try an idea we might throw away",
			},
		},
		{
			Key:           KeyAudience,
			ProjectPrompt: "Who is this project for?",
			BranchPrompt:  "Who is this branch for?",
			Help:          "Agents make different choices for an internal tool than a client-facing one.",
			Options: []string{
				"Our own team",
				"KPMG staff broadly",
				"Client users",
				"Nobody yet — a spike",
			},
		},
		{
			Key:           KeyConstraints,
			ProjectPrompt: "What must this project never break?",
			BranchPrompt:  "What must this branch not break?",
			Help:          "Agents will refuse work that violates this instead of guessing.",
			Options: []string{
				"The published API",
				"Existing data and migrations",
				"The deployed environment",
				"Nothing — it is isolated",
			},
		},
		{
			Key:           KeyDone,
			ProjectPrompt: "What has to be true before work here is finished?",
			BranchPrompt:  "What has to be true before this branch can merge?",
			Help:          "This becomes the definition of done, shown again at merge.",
			Options: []string{
				"It has tests that pass",
				"It does not break the published API",
				"It works on mobile",
				"Someone else has reviewed it",
			},
		},
		{
			Key:           KeyOutOfScope,
			ProjectPrompt: "What is explicitly out of scope for this project?",
			BranchPrompt:  "What is explicitly out of scope for this branch?",
			Help:          "Naming this is how you stop an agent widening the job while you are away.",
			Options: []string{
				"Performance work",
				"Visual design",
				"Migrations and data changes",
				"Nothing is off the table",
			},
		},
	}
}

// Source records where an answer came from. Drafted answers are the model's
// reading of an imported repo and are shown as corrections-pending until the
// user touches them.
type Source string

const (
	SourcePicked  Source = "picked"
	SourceTyped   Source = "typed"
	SourceDrafted Source = "drafted"
)

// Answer is one settled response.
type Answer struct {
	Key        Key       `json:"key"`
	Prompt     string    `json:"prompt"`
	Text       string    `json:"text"`
	Source     Source    `json:"source"`
	AnsweredAt time.Time `json:"answeredAt"`
}

// Status is where a brief has got to.
type Status string

const (
	StatusOpen     Status = "open"
	StatusComplete Status = "complete"
	StatusSkipped  Status = "skipped"
)

// Brief is the durable artifact. Questions is stored on the brief, not derived,
// so a brief opened tomorrow asks the same questions it asked today even if
// BaseQuestions changes.
type Brief struct {
	ID        string     `json:"id"`
	Scope     Scope      `json:"scope"`
	ProjectID string     `json:"projectId"`
	BranchID  string     `json:"branchId,omitempty"`
	Questions []Question `json:"questions"`
	Answers   []Answer   `json:"answers"`
	Status    Status     `json:"status"`
	Revision  int        `json:"revision"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// NewBrief starts a brief over the base question set.
func NewBrief(id string, scope Scope, projectID, branchID string, now time.Time) *Brief {
	return &Brief{
		ID:        id,
		Scope:     scope,
		ProjectID: projectID,
		BranchID:  branchID,
		Questions: BaseQuestions(),
		Answers:   []Answer{},
		Status:    StatusOpen,
		Revision:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
```

- [ ] **Step 4: Write the flow behaviour**

Append to `control-plane/internal/discover/flow.go`:

```go
// StepKind distinguishes the three things the surface can be showing.
type StepKind string

const (
	StepQuestion StepKind = "question"
	StepFollowUp StepKind = "followUp"
	StepComplete StepKind = "complete"
)

// Step is what Discover should render next.
//
// Total is the denominator and is zero for follow-ups and for the complete
// state. A UI must never render a bar, a percentage or an ETA from it — the
// only permitted rendering is the literal "QUESTION n OF m" over the closed
// base set (docs/DESIGN-SYSTEM.md, docs/AGENTIC-UX.md anti-pattern 2).
type Step struct {
	Kind     StepKind `json:"kind"`
	Ordinal  int      `json:"ordinal"`
	Total    int      `json:"total"`
	Question Question `json:"question"`
	Prompt   string   `json:"prompt"`
}

// Errors returned by this package.
var (
	ErrUnknownKey  = errors.New("discover: no question with that key")
	ErrEmptyAnswer = errors.New("discover: an answer cannot be empty")
	ErrNotAnswered = errors.New("discover: that question has not been answered yet")
)

// Answered reports whether a key already has an answer.
func (b *Brief) Answered(key Key) bool {
	for _, a := range b.Answers {
		if a.Key == key {
			return true
		}
	}
	return false
}

// question looks up a question by key.
func (b *Brief) question(key Key) (Question, int, bool) {
	for i, q := range b.Questions {
		if q.Key == key {
			return q, i, true
		}
	}
	return Question{}, -1, false
}

// baseCount is how many of the brief's questions belong to the closed set.
func (b *Brief) baseCount() int {
	n := 0
	for _, q := range b.Questions {
		if !q.FollowUp {
			n++
		}
	}
	return n
}

// Step returns the next thing to render.
func (b *Brief) Step() Step {
	if b.Status == StatusSkipped || b.Status == StatusComplete {
		return Step{Kind: StepComplete}
	}
	base := b.baseCount()
	ordinal := 0
	for _, q := range b.Questions {
		if !q.FollowUp {
			ordinal++
		}
		if b.Answered(q.Key) {
			continue
		}
		if q.FollowUp {
			return Step{Kind: StepFollowUp, Question: q, Prompt: q.Prompt(b.Scope)}
		}
		return Step{Kind: StepQuestion, Ordinal: ordinal, Total: base, Question: q, Prompt: q.Prompt(b.Scope)}
	}
	return Step{Kind: StepComplete}
}

// Record settles one answer. It appends when the key is new and replaces in
// place when it is not, so re-answering never disturbs the order of the list
// or any other answer.
func (b *Brief) Record(key Key, text string, source Source, now time.Time) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return ErrEmptyAnswer
	}
	q, _, ok := b.question(key)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}
	answer := Answer{
		Key:        key,
		Prompt:     q.Prompt(b.Scope),
		Text:       text,
		Source:     source,
		AnsweredAt: now,
	}
	replaced := false
	for i := range b.Answers {
		if b.Answers[i].Key == key {
			b.Answers[i] = answer
			replaced = true
			break
		}
	}
	if !replaced {
		b.Answers = append(b.Answers, answer)
	}
	b.Revision++
	b.UpdatedAt = now
	if b.Status == StatusOpen && b.Step().Kind == StepComplete {
		b.Status = StatusComplete
	}
	return nil
}

// Edit changes a settled answer IN PLACE.
//
// The prototype in prototype/app.html spliced the answer array at the edited
// index and lost every later answer (docs/FUNCTIONAL-SPEC.md, known defect 7).
// Edit exists so that bug has one place it can be prevented and one test that
// proves it is.
func (b *Brief) Edit(key Key, text string, now time.Time) error {
	if !b.Answered(key) {
		return fmt.Errorf("%w: %q", ErrNotAnswered, key)
	}
	return b.Record(key, text, SourceTyped, now)
}

// Skip closes the brief without answering the rest. The answers already given
// still generate a CLAUDE.md; a skipped brief is not an empty one.
func (b *Brief) Skip(now time.Time) {
	b.Status = StatusSkipped
	b.Revision++
	b.UpdatedAt = now
}

// Reopen moves a settled brief back to open so the next unanswered question is
// asked again. Editing an answer does not reopen a brief.
func (b *Brief) Reopen(now time.Time) {
	b.Status = StatusOpen
	b.Revision++
	b.UpdatedAt = now
	if b.Step().Kind == StepComplete {
		b.Status = StatusComplete
	}
}

// AddFollowUp appends a model-proposed question. Follow-ups carry no
// denominator, so adding one never changes "OF 5".
func (b *Brief) AddFollowUp(q Question, now time.Time) error {
	if q.Key == "" {
		return fmt.Errorf("%w: empty", ErrUnknownKey)
	}
	if _, _, exists := b.question(q.Key); exists {
		return fmt.Errorf("discover: question %q already present", q.Key)
	}
	q.FollowUp = true
	b.Questions = append(b.Questions, q)
	b.Revision++
	b.UpdatedAt = now
	if b.Status == StatusComplete {
		b.Status = StatusOpen
	}
	return nil
}

// AnswerFor returns the settled answer for a key.
func (b *Brief) AnswerFor(key Key) (Answer, bool) {
	for _, a := range b.Answers {
		if a.Key == key {
			return a, true
		}
	}
	return Answer{}, false
}

// SplitCriteria turns a definition-of-done answer into discrete items.
// Deterministic: split on newlines and semicolons, strip bullet markers, drop
// empties. No model call — the merge checklist must not drift between reads.
func SplitCriteria(text string) []string {
	rough := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == ';' })
	items := make([]string, 0, len(rough))
	for _, part := range rough {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "- ")
		part = strings.TrimPrefix(part, "* ")
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./control-plane/internal/discover/... -v -race -count=1`
Expected: PASS — 9 tests, including `TestEditChangesOneAnswerAndKeepsTheRest`.

- [ ] **Step 6: Commit**

```bash
git add control-plane/internal/discover
git commit -m "feat(discover): question flow with edit-in-place and an honest step

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Model-tailored options and at most one follow-up

This is the genuinely AI-dependent part of the flow. Both functions **degrade to the static behaviour on any failure** — no credentials, gateway down, nonsense reply. Discover must work on a laptop with nothing configured.

**Files:**
- Create: `control-plane/internal/discover/assist.go`
- Test: `control-plane/internal/discover/assist_test.go`

**Interfaces:**
- Consumes: `modelclient.Completer`, `modelclient.Request`, `modelclient.Response`, `modelclient.ExtractJSON`
- Produces:
  - `discover.Context{ProjectName, BranchName, ProjectHint, RepoSummary string}`
  - `discover.TailorOptions(ctx, c modelclient.Completer, b *Brief, q Question, hint Context) Question`
  - `discover.ProposeFollowUp(ctx, c modelclient.Completer, b *Brief, hint Context, now time.Time) bool`

- [ ] **Step 1: Write the failing tests**

Create `control-plane/internal/discover/assist_test.go`:

```go
package discover_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
)

// fakeCompleter returns canned replies and records the last request.
type fakeCompleter struct {
	reply modelclient.Response
	err   error
	last  modelclient.Request
	calls int
}

func (f *fakeCompleter) Complete(_ context.Context, req modelclient.Request) (modelclient.Response, error) {
	f.last = req
	f.calls++
	return f.reply, f.err
}

func TestTailorOptionsReplacesTheOptions(t *testing.T) {
	fake := &fakeCompleter{reply: modelclient.Response{
		Text: `{"options":["Unblock evidence upload","Stop the 40MB timeout","Ship the mobile path"]}`,
	}}
	b := discover.NewBrief("b", discover.ScopeBranch, "p", "br", clock)

	got := discover.TailorOptions(context.Background(), fake, b, discover.BaseQuestions()[0],
		discover.Context{ProjectName: "Engagement Portal", BranchName: "evidence-upload"})

	if len(got.Options) != 3 || got.Options[0] != "Unblock evidence upload" {
		t.Errorf("options = %#v", got.Options)
	}
	if got.Key != discover.KeyPurpose {
		t.Errorf("key changed to %q; tailoring must only touch the options", got.Key)
	}
	if fake.last.Effort != "low" {
		t.Errorf("effort = %q, want low — the user is waiting on this call", fake.last.Effort)
	}
}

func TestTailorOptionsDegradesToTheStaticOptions(t *testing.T) {
	question := discover.BaseQuestions()[0]
	b := discover.NewBrief("b", discover.ScopeBranch, "p", "br", clock)

	for name, fake := range map[string]*fakeCompleter{
		"transport error": {err: errors.New("gateway down")},
		"not json":        {reply: modelclient.Response{Text: "I'm afraid I can't do that"}},
		"too few options": {reply: modelclient.Response{Text: `{"options":["one","two"]}`}},
	} {
		t.Run(name, func(t *testing.T) {
			got := discover.TailorOptions(context.Background(), fake, b, question, discover.Context{})
			if len(got.Options) != len(question.Options) || got.Options[0] != question.Options[0] {
				t.Errorf("options = %#v, want the static set", got.Options)
			}
		})
	}
}

func TestTailorOptionsWithoutAModelIsANoOp(t *testing.T) {
	question := discover.BaseQuestions()[1]
	b := discover.NewBrief("b", discover.ScopeBranch, "p", "br", clock)
	got := discover.TailorOptions(context.Background(), nil, b, question, discover.Context{})
	if len(got.Options) != len(question.Options) {
		t.Errorf("options = %#v, want the static set", got.Options)
	}
}

func TestProposeFollowUpAddsAtMostOneQuestion(t *testing.T) {
	fake := &fakeCompleter{reply: modelclient.Response{Text: `{
		"needed": true, "key": "rollout", "prompt": "How does this reach users?",
		"help": "You said this changes a published API.",
		"options": ["Behind a flag", "All at once", "Internal only first"]}`}}

	b := discover.NewBrief("b", discover.ScopeBranch, "p", "br", clock)
	_ = b.Record(discover.KeyPurpose, "Change the upload API", discover.SourceTyped, clock)

	if !discover.ProposeFollowUp(context.Background(), fake, b, discover.Context{}, clock) {
		t.Fatal("ProposeFollowUp returned false, want true")
	}
	if len(b.Questions) != 6 {
		t.Errorf("questions = %d, want 6", len(b.Questions))
	}
}

func TestProposeFollowUpDeclinesWhenNoneIsNeeded(t *testing.T) {
	fake := &fakeCompleter{reply: modelclient.Response{Text: `{"needed": false}`}}
	b := discover.NewBrief("b", discover.ScopeBranch, "p", "br", clock)
	_ = b.Record(discover.KeyPurpose, "Fix the uploader", discover.SourceTyped, clock)

	if discover.ProposeFollowUp(context.Background(), fake, b, discover.Context{}, clock) {
		t.Error("ProposeFollowUp returned true for needed=false")
	}
	if len(b.Questions) != 5 {
		t.Errorf("questions = %d, want the base set untouched", len(b.Questions))
	}
}

func TestProposeFollowUpOnAnUntouchedBriefDoesNothing(t *testing.T) {
	fake := &fakeCompleter{reply: modelclient.Response{Text: `{"needed": true, "key":"x","prompt":"y","options":["a","b","c"]}`}}
	b := discover.NewBrief("b", discover.ScopeBranch, "p", "br", clock)
	if discover.ProposeFollowUp(context.Background(), fake, b, discover.Context{}, clock) {
		t.Error("proposed a follow-up before any question was answered")
	}
	if fake.calls != 0 {
		t.Errorf("calls = %d, want 0 — do not spend a model call on an empty brief", fake.calls)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/discover/...`
Expected: FAIL — `undefined: discover.TailorOptions`.

- [ ] **Step 3: Write the implementation**

Create `control-plane/internal/discover/assist.go`:

```go
package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
)

// optionsSystem rewrites a question's pickable answers for the project at hand.
// The question itself never changes — only the options — so the key, the
// ordering and the denominator all stay fixed.
const optionsSystem = `You are tailoring the pickable answers for one question in a short branch-setup interview.

Return between three and five options. Each is at most nine words, written as the user would say it about
their own work, concrete rather than generic. Do not offer an "other" or "something else" option: a free-text
box is always on screen beside your options. Do not repeat an answer the user has already given.`

var optionsSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["options"],
  "properties": {
    "options": {
      "type": "array",
      "minItems": 3,
      "maxItems": 5,
      "items": {"type": "string"}
    }
  }
}`)

type optionsReply struct {
	Options []string `json:"options"`
}

// Context is what the model is told about the work, so options are about this
// branch rather than about software in general.
type Context struct {
	ProjectName string
	BranchName  string
	ProjectHint string // the project brief's purpose, or the idea typed at creation
	RepoSummary string // repoanalysis Findings.Summary(), empty for a new project
}

// TailorOptions replaces q.Options with options written for ctx.
//
// It DEGRADES rather than fails: on any error — no credentials, gateway down,
// nonsense reply — it returns the question unchanged, and the user sees the
// static options. Discover must work on a laptop with no model configured, so
// the model is an improvement to the flow, never a gate on it.
func TailorOptions(ctx context.Context, c modelclient.Completer, b *Brief, q Question, hint Context) Question {
	if c == nil {
		return q
	}

	var prompt strings.Builder
	fmt.Fprintf(&prompt, "Project: %s\n", hint.ProjectName)
	if hint.BranchName != "" {
		fmt.Fprintf(&prompt, "Branch: %s\n", hint.BranchName)
	}
	if hint.ProjectHint != "" {
		fmt.Fprintf(&prompt, "What the project is: %s\n", hint.ProjectHint)
	}
	if hint.RepoSummary != "" {
		fmt.Fprintf(&prompt, "\nRepository scan:\n%s\n", hint.RepoSummary)
	}
	if len(b.Answers) > 0 {
		prompt.WriteString("\nAlready answered:\n")
		for _, a := range b.Answers {
			fmt.Fprintf(&prompt, "- %s: %s\n", a.Prompt, a.Text)
		}
	}
	fmt.Fprintf(&prompt, "\nQuestion to write options for: %s\n", q.Prompt(b.Scope))
	fmt.Fprintf(&prompt, "Current generic options: %s\n", strings.Join(q.Options, " | "))

	resp, err := c.Complete(ctx, modelclient.Request{
		System: optionsSystem,
		User:   prompt.String(),
		// A cheap, bounded call: the user is waiting on it between questions.
		Effort:     "low",
		JSONSchema: optionsSchema,
	})
	if err != nil {
		return q
	}
	raw, err := modelclient.ExtractJSON(resp.Text)
	if err != nil {
		return q
	}
	var reply optionsReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return q
	}

	cleaned := make([]string, 0, len(reply.Options))
	for _, o := range reply.Options {
		o = strings.TrimSpace(o)
		if o != "" {
			cleaned = append(cleaned, o)
		}
	}
	if len(cleaned) < 3 {
		return q
	}
	if len(cleaned) > 5 {
		cleaned = cleaned[:5]
	}
	q.Options = cleaned
	return q
}

// followUpSystem asks for at most one extra question, and permits zero.
const followUpSystem = `You are reviewing a completed branch-setup interview.

If one further question would materially change how a coding agent works on this branch, propose exactly one.
Otherwise propose none. Prefer none: every extra question is a cost paid by every user of this product.

A proposed question needs a short snake_case key, a one-sentence prompt ending in a question mark, a one-sentence
reason the user will see, and three or four pickable options of at most nine words each.`

var followUpSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["needed"],
  "properties": {
    "needed": {"type": "boolean"},
    "key": {"type": "string"},
    "prompt": {"type": "string"},
    "help": {"type": "string"},
    "options": {"type": "array", "minItems": 3, "maxItems": 4, "items": {"type": "string"}}
  }
}`)

type followUpReply struct {
	Needed  bool     `json:"needed"`
	Key     string   `json:"key"`
	Prompt  string   `json:"prompt"`
	Help    string   `json:"help"`
	Options []string `json:"options"`
}

// ProposeFollowUp appends at most one model-proposed question to b.
//
// It reports whether a question was added. Like TailorOptions it degrades to a
// no-op on any error, because a brief that is complete is more valuable than a
// brief that is blocked on a gateway.
func ProposeFollowUp(ctx context.Context, c modelclient.Completer, b *Brief, hint Context, now time.Time) bool {
	if c == nil || len(b.Answers) == 0 {
		return false
	}
	var prompt strings.Builder
	fmt.Fprintf(&prompt, "Project: %s\nBranch: %s\n\nAnswers so far:\n", hint.ProjectName, hint.BranchName)
	for _, a := range b.Answers {
		fmt.Fprintf(&prompt, "- %s: %s\n", a.Prompt, a.Text)
	}

	resp, err := c.Complete(ctx, modelclient.Request{
		System:     followUpSystem,
		User:       prompt.String(),
		Effort:     "medium",
		JSONSchema: followUpSchema,
	})
	if err != nil {
		return false
	}
	raw, err := modelclient.ExtractJSON(resp.Text)
	if err != nil {
		return false
	}
	var reply followUpReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return false
	}
	if !reply.Needed || reply.Key == "" || reply.Prompt == "" || len(reply.Options) < 3 {
		return false
	}
	q := Question{
		Key:           Key(reply.Key),
		ProjectPrompt: reply.Prompt,
		BranchPrompt:  reply.Prompt,
		Help:          reply.Help,
		Options:       reply.Options,
	}
	return b.AddFollowUp(q, now) == nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./control-plane/internal/discover/... -v -race -count=1`
Expected: PASS — 15 tests including the three `TestTailorOptionsDegradesToTheStaticOptions` sub-tests.

- [ ] **Step 5: Commit**

```bash
git add control-plane/internal/discover/assist.go control-plane/internal/discover/assist_test.go
git commit -m "feat(discover): model-tailored options and one optional follow-up

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Inheritance and the CLAUDE.md block

Project → branch → session. The branch inherits the project brief; a branch answer for the same key overrides it and the file says so.

**Files:**
- Create: `control-plane/internal/discover/inherit.go`, `control-plane/internal/claudemd/render.go`
- Test: `control-plane/internal/discover/inherit_test.go`, `control-plane/internal/claudemd/render_test.go`

**Interfaces:**
- Consumes: `discover.Brief`, `discover.Key`, `discover.SplitCriteria`
- Produces:
  - `discover.Origin` with `OriginProject`, `OriginBranch`, `OriginOverride`
  - `discover.Section{Key, Heading, Body string; Items []string; Origin Origin; Inherits string}`
  - `discover.Document{ProjectName, BranchName, BriefID string; Revision int; Sections []Section}`
  - `discover.Resolve(projectName, branchName string, project, branch *Brief) Document`
  - `claudemd.Render(doc discover.Document) string` — deterministic, golden-tested
  - `claudemd.Merge(existing, block string) (string, error)`
  - `claudemd.BeginPrefix`, `claudemd.EndMarker`, `claudemd.ErrUnterminatedBlock`

- [ ] **Step 1: Write the failing inheritance tests**

Create `control-plane/internal/discover/inherit_test.go`:

```go
package discover_test

import (
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

func TestResolveInheritsTheProjectBriefWhereTheBranchIsSilent(t *testing.T) {
	project := discover.NewBrief("p-brief", discover.ScopeProject, "p-1", "", clock)
	_ = project.Record(discover.KeyPurpose, "An engagement portal.", discover.SourceTyped, clock)
	_ = project.Record(discover.KeyAudience, "Client users", discover.SourcePicked, clock)

	branch := discover.NewBrief("b-brief", discover.ScopeBranch, "p-1", "br-1", clock)
	_ = branch.Record(discover.KeyPurpose, "Add evidence upload.", discover.SourceTyped, clock)

	doc := discover.Resolve("Engagement Portal", "evidence-upload", project, branch)

	if len(doc.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(doc.Sections))
	}
	if doc.Sections[0].Key != discover.KeyPurpose || doc.Sections[0].Origin != discover.OriginOverride {
		t.Errorf("purpose = %q/%q, want purpose/override", doc.Sections[0].Key, doc.Sections[0].Origin)
	}
	if doc.Sections[0].Body != "Add evidence upload." {
		t.Errorf("purpose body = %q, want the branch answer", doc.Sections[0].Body)
	}
	if doc.Sections[0].Inherits != "An engagement portal." {
		t.Errorf("inherits = %q, want the project answer it replaced", doc.Sections[0].Inherits)
	}
	if doc.Sections[1].Key != discover.KeyAudience || doc.Sections[1].Origin != discover.OriginProject {
		t.Errorf("audience = %q/%q, want audience/project", doc.Sections[1].Key, doc.Sections[1].Origin)
	}
	if doc.BriefID != "b-brief" {
		t.Errorf("BriefID = %q, want the branch brief", doc.BriefID)
	}
}

func TestResolveSectionOrderDoesNotDependOnAnswerOrder(t *testing.T) {
	branch := discover.NewBrief("b", discover.ScopeBranch, "p", "br", clock)
	_ = branch.Record(discover.KeyOutOfScope, "Visual design", discover.SourcePicked, clock)
	_ = branch.Record(discover.KeyPurpose, "Fix the uploader", discover.SourcePicked, clock)

	doc := discover.Resolve("P", "b", nil, branch)
	if doc.Sections[0].Key != discover.KeyPurpose || doc.Sections[1].Key != discover.KeyOutOfScope {
		t.Errorf("order = %q,%q — file order must be fixed, not answer order",
			doc.Sections[0].Key, doc.Sections[1].Key)
	}
}

func TestResolveSplitsTheDefinitionOfDoneIntoItems(t *testing.T) {
	branch := discover.NewBrief("b", discover.ScopeBranch, "p", "br", clock)
	_ = branch.Record(discover.KeyDone, "Tests pass\nNo API break", discover.SourceTyped, clock)

	doc := discover.Resolve("P", "b", nil, branch)
	if len(doc.Sections) != 1 {
		t.Fatalf("sections = %d, want 1", len(doc.Sections))
	}
	if len(doc.Sections[0].Items) != 2 {
		t.Fatalf("items = %#v, want 2", doc.Sections[0].Items)
	}
}

func TestResolveWithNoBriefsProducesAnEmptyDocument(t *testing.T) {
	doc := discover.Resolve("P", "main", nil, nil)
	if len(doc.Sections) != 0 {
		t.Errorf("sections = %d, want 0", len(doc.Sections))
	}
}

func TestResolveCarriesFollowUpsAfterTheBaseSections(t *testing.T) {
	branch := discover.NewBrief("b", discover.ScopeBranch, "p", "br", clock)
	_ = branch.Record(discover.KeyPurpose, "Fix the uploader", discover.SourcePicked, clock)
	_ = branch.AddFollowUp(discover.Question{
		Key: discover.Key("rollout"), BranchPrompt: "How does this reach users?",
	}, clock)
	_ = branch.Record(discover.Key("rollout"), "Behind a flag", discover.SourcePicked, clock)

	doc := discover.Resolve("P", "b", nil, branch)
	if len(doc.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(doc.Sections))
	}
	if doc.Sections[1].Key != discover.Key("rollout") {
		t.Errorf("last section = %q, want rollout", doc.Sections[1].Key)
	}
	if doc.Sections[1].Heading != "How does this reach users" {
		t.Errorf("heading = %q, want the prompt without its question mark", doc.Sections[1].Heading)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/discover/...`
Expected: FAIL — `undefined: discover.Resolve`.

- [ ] **Step 3: Write the inheritance implementation**

Create `control-plane/internal/discover/inherit.go`:

```go
package discover

import "strings"

// Origin says which brief a section came from. It is rendered in CLAUDE.md so
// an agent — and a human reading the file — can see what was inherited and
// what the branch changed.
type Origin string

const (
	OriginProject  Origin = "project"
	OriginBranch   Origin = "branch"
	OriginOverride Origin = "override" // branch answered a key the project also answered
)

// Section is one rendered block of the generated context file.
type Section struct {
	Key      Key      `json:"key"`
	Heading  string   `json:"heading"`
	Body     string   `json:"body"`
	Items    []string `json:"items,omitempty"`
	Origin   Origin   `json:"origin"`
	Inherits string   `json:"inherits,omitempty"` // the project text a branch answer replaced
}

// Document is everything needed to render a CLAUDE.md. It is produced by
// Resolve and consumed by Render; nothing else may build one.
type Document struct {
	ProjectName string    `json:"projectName"`
	BranchName  string    `json:"branchName"`
	BriefID     string    `json:"briefId"`
	Revision    int       `json:"revision"`
	Sections    []Section `json:"sections"`
}

// headings maps a key to its section heading. Order here is the order in the
// file, and it does not depend on the order the questions were answered in.
var headings = []struct {
	Key     Key
	Heading string
}{
	{KeyPurpose, "What this is for"},
	{KeyAudience, "Who it is for"},
	{KeyConstraints, "What must not break"},
	{KeyDone, "Definition of done"},
	{KeyOutOfScope, "Out of scope"},
}

// headingFor returns the heading for a key, falling back to the question prompt
// for model-proposed follow-ups.
func headingFor(key Key, briefs ...*Brief) string {
	for _, h := range headings {
		if h.Key == key {
			return h.Heading
		}
	}
	for _, b := range briefs {
		if b == nil {
			continue
		}
		if q, _, ok := b.question(key); ok {
			return strings.TrimSuffix(q.Prompt(b.Scope), "?")
		}
	}
	return string(key)
}

// orderedKeys returns the base keys in file order, then any follow-up keys in
// the order they were added — project first, then branch.
func orderedKeys(project, branch *Brief) []Key {
	seen := map[Key]bool{}
	out := make([]Key, 0, len(headings)+4)
	for _, h := range headings {
		seen[h.Key] = true
		out = append(out, h.Key)
	}
	for _, b := range []*Brief{project, branch} {
		if b == nil {
			continue
		}
		for _, q := range b.Questions {
			if !seen[q.Key] {
				seen[q.Key] = true
				out = append(out, q.Key)
			}
		}
	}
	return out
}

// Resolve applies inheritance: project → branch. A branch answer replaces the
// project answer for the same key and is marked OriginOverride, carrying the
// project text it replaced so the file can show both. Keys the branch did not
// answer are inherited unchanged. Either brief may be nil.
func Resolve(projectName, branchName string, project, branch *Brief) Document {
	doc := Document{ProjectName: projectName, BranchName: branchName}
	if branch != nil {
		doc.BriefID, doc.Revision = branch.ID, branch.Revision
	} else if project != nil {
		doc.BriefID, doc.Revision = project.ID, project.Revision
	}

	for _, key := range orderedKeys(project, branch) {
		var projectAnswer, branchAnswer Answer
		haveProject, haveBranch := false, false
		if project != nil {
			projectAnswer, haveProject = project.AnswerFor(key)
		}
		if branch != nil {
			branchAnswer, haveBranch = branch.AnswerFor(key)
		}
		if !haveProject && !haveBranch {
			continue
		}

		section := Section{Key: key, Heading: headingFor(key, branch, project)}
		switch {
		case haveBranch && haveProject:
			section.Body = branchAnswer.Text
			section.Origin = OriginOverride
			section.Inherits = projectAnswer.Text
		case haveBranch:
			section.Body = branchAnswer.Text
			section.Origin = OriginBranch
		default:
			section.Body = projectAnswer.Text
			section.Origin = OriginProject
		}
		if key == KeyDone {
			section.Items = SplitCriteria(section.Body)
		}
		doc.Sections = append(doc.Sections, section)
	}
	return doc
}
```

- [ ] **Step 4: Run the inheritance tests to verify they pass**

Run: `go test ./control-plane/internal/discover/... -run Resolve -v`
Expected: PASS — 5 tests.

- [ ] **Step 5: Write the failing render tests**

Create `control-plane/internal/claudemd/render_test.go`:

```go
package claudemd_test

import (
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

func sampleDoc() discover.Document {
	return discover.Document{
		ProjectName: "Engagement Portal",
		BranchName:  "evidence-upload",
		BriefID:     "b-42",
		Revision:    3,
		Sections: []discover.Section{
			{
				Key: discover.KeyPurpose, Heading: "What this is for",
				Body: "Add evidence upload.", Origin: discover.OriginOverride,
				Inherits: "An engagement portal.",
			},
			{
				Key: discover.KeyAudience, Heading: "Who it is for",
				Body: "Client users", Origin: discover.OriginProject,
			},
			{
				Key: discover.KeyDone, Heading: "Definition of done",
				Body:   "Tests pass\nNo API break",
				Items:  []string{"Tests pass", "No API break"},
				Origin: discover.OriginBranch,
			},
		},
	}
}

const wantRendered = `<!-- panacea:discover:begin brief=b-42 rev=3 -->
# Engagement Portal — evidence-upload

Written by Discover in Panacea App. Every session started on this branch reads it first.
Change it in Discover, not here: this block is regenerated and your edits inside it will be lost.

## What this is for
_Set on this branch, replacing the project brief: An engagement portal._

Add evidence upload.

## Who it is for
_Inherited from the project brief._

Client users

## Definition of done

- Tests pass
- No API break

---

Brief b-42, revision 3.
<!-- panacea:discover:end -->
`

func TestRenderMatchesTheGoldenFile(t *testing.T) {
	got := claudemd.Render(sampleDoc())
	if got != wantRendered {
		t.Errorf("Render mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, wantRendered)
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	if claudemd.Render(sampleDoc()) != claudemd.Render(sampleDoc()) {
		t.Error("Render is not deterministic; a timestamp has crept into the managed block")
	}
}

func TestRenderHandlesAnEmptyBrief(t *testing.T) {
	got := claudemd.Render(discover.Document{ProjectName: "P", BranchName: "main", BriefID: "b-0"})
	if !strings.Contains(got, "No brief has been recorded") {
		t.Errorf("empty document render = %q", got)
	}
	if !strings.HasSuffix(got, claudemd.EndMarker+"\n") {
		t.Error("render must always close the managed block")
	}
}

func TestMergeAppendsWhenNoBlockIsPresent(t *testing.T) {
	existing := "# Hand written\n\nSome house rules.\n"
	got, err := claudemd.Merge(existing, claudemd.Render(sampleDoc()))
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if !strings.HasPrefix(got, "# Hand written\n\nSome house rules.\n\n<!-- panacea:discover:begin") {
		t.Errorf("merged = %q, want the user content first then the block", got)
	}
}

func TestMergeReplacesOnlyTheManagedBlock(t *testing.T) {
	first, err := claudemd.Merge("# Rules\n\nAlways run gofmt.\n", claudemd.Render(sampleDoc()))
	if err != nil {
		t.Fatalf("first Merge: %v", err)
	}
	withTail := first + "\n## Notes below the block\n\nKeep me.\n"

	doc := sampleDoc()
	doc.Revision = 4
	second, err := claudemd.Merge(withTail, claudemd.Render(doc))
	if err != nil {
		t.Fatalf("second Merge: %v", err)
	}

	if !strings.Contains(second, "Always run gofmt.") {
		t.Error("content above the block was lost")
	}
	if !strings.Contains(second, "Keep me.") {
		t.Error("content below the block was lost")
	}
	if strings.Contains(second, "rev=3") {
		t.Error("the old block survived")
	}
	if !strings.Contains(second, "rev=4") {
		t.Error("the new block was not written")
	}
	if strings.Count(second, claudemd.BeginPrefix) != 1 {
		t.Errorf("begin markers = %d, want exactly 1", strings.Count(second, claudemd.BeginPrefix))
	}
}

func TestMergeOnAnEmptyFileReturnsJustTheBlock(t *testing.T) {
	block := claudemd.Render(sampleDoc())
	got, err := claudemd.Merge("", block)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got != block {
		t.Errorf("merged = %q, want the block alone", got)
	}
}

func TestMergeFailsClosedOnAnUnterminatedBlock(t *testing.T) {
	broken := "# Rules\n\n<!-- panacea:discover:begin brief=x rev=1 -->\nhalf a block\n"
	if _, err := claudemd.Merge(broken, claudemd.Render(sampleDoc())); err != claudemd.ErrUnterminatedBlock {
		t.Fatalf("err = %v, want ErrUnterminatedBlock", err)
	}
}
```

- [ ] **Step 6: Run the render tests to verify they fail**

Run: `go test ./control-plane/internal/claudemd/...`
Expected: FAIL — package `claudemd` does not exist.

- [ ] **Step 7: Write the renderer**

Create `control-plane/internal/claudemd/render.go`:

```go
// Package claudemd renders a Discover document as the CLAUDE.md that agent
// sessions read at start-up, and merges it into a file a human may also own.
//
// The rendered block is DETERMINISTIC: same document in, byte-identical text
// out. There is deliberately no timestamp inside the managed block — a
// timestamp would rewrite the file on every materialise, which shows up as a
// spurious diff in the worktree and as noise in review.
package claudemd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

// Markers delimit the block this package owns. Everything outside them belongs
// to the user and is never touched.
const (
	BeginPrefix = "<!-- panacea:discover:begin"
	EndMarker   = "<!-- panacea:discover:end -->"
)

// ErrUnterminatedBlock is returned when a begin marker has no matching end.
// Merge fails closed rather than guessing where the block stops, because the
// guess would delete somebody's writing.
var ErrUnterminatedBlock = errors.New("claudemd: begin marker with no end marker")

// Render returns the managed block for a document, including both markers and
// a trailing newline.
func Render(doc discover.Document) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s brief=%s rev=%d -->\n", BeginPrefix, doc.BriefID, doc.Revision)
	title := doc.ProjectName
	if doc.BranchName != "" {
		title = doc.ProjectName + " — " + doc.BranchName
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	b.WriteString("Written by Discover in Panacea App. Every session started on this branch reads it first.\n")
	b.WriteString("Change it in Discover, not here: this block is regenerated and your edits inside it will be lost.\n")

	if len(doc.Sections) == 0 {
		b.WriteString("\nNo brief has been recorded for this branch yet.\n")
		b.WriteString(EndMarker + "\n")
		return b.String()
	}

	for _, s := range doc.Sections {
		fmt.Fprintf(&b, "\n## %s\n", s.Heading)
		switch s.Origin {
		case discover.OriginProject:
			b.WriteString("_Inherited from the project brief._\n")
		case discover.OriginOverride:
			fmt.Fprintf(&b, "_Set on this branch, replacing the project brief: %s_\n", s.Inherits)
		}
		b.WriteString("\n")
		if len(s.Items) > 0 {
			for _, item := range s.Items {
				fmt.Fprintf(&b, "- %s\n", item)
			}
		} else {
			fmt.Fprintf(&b, "%s\n", s.Body)
		}
	}

	fmt.Fprintf(&b, "\n---\n\nBrief %s, revision %d.\n", doc.BriefID, doc.Revision)
	b.WriteString(EndMarker + "\n")
	return b.String()
}

// Merge splices block into existing, replacing any block already there and
// leaving every other byte alone.
//
//   - no begin marker: block is appended after the existing content
//   - begin and end present: the region between them inclusive is replaced
//   - begin present, end missing: ErrUnterminatedBlock, nothing changed
func Merge(existing, block string) (string, error) {
	start := strings.Index(existing, BeginPrefix)
	if start == -1 {
		trimmed := strings.TrimRight(existing, "\n")
		if trimmed == "" {
			return block, nil
		}
		return trimmed + "\n\n" + block, nil
	}
	rest := existing[start:]
	endRel := strings.Index(rest, EndMarker)
	if endRel == -1 {
		return "", ErrUnterminatedBlock
	}
	end := start + endRel + len(EndMarker)
	tail := strings.TrimPrefix(existing[end:], "\n")

	var b strings.Builder
	b.WriteString(existing[:start])
	b.WriteString(block)
	b.WriteString(tail)
	return b.String(), nil
}
```

- [ ] **Step 8: Run both packages' tests**

Run: `go test ./control-plane/internal/discover/... ./control-plane/internal/claudemd/... -v -race -count=1`
Expected: PASS. `TestRenderMatchesTheGoldenFile` passing is the proof the block is byte-exact; if it fails, fix the renderer, never the golden string.

- [ ] **Step 9: Commit**

```bash
git add control-plane/internal/discover/inherit.go control-plane/internal/discover/inherit_test.go control-plane/internal/claudemd
git commit -m "feat(discover): project to branch inheritance rendered as a managed CLAUDE.md block

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Writing CLAUDE.md into a worktree — the Phase 3 seam

**Files:**
- Create: `control-plane/internal/contextwrite/writer.go`
- Test: `control-plane/internal/contextwrite/writer_test.go`

**Interfaces:**
- Consumes: `discover.Document`, `claudemd.Render`, `claudemd.Merge`
- Produces:
  - `contextwrite.WorktreeResolver` interface with `Path(ctx context.Context, worktreeID string) (string, error)`
  - `contextwrite.StaticResolver map[string]string`
  - `contextwrite.NewWriter(resolver WorktreeResolver) *Writer`
  - `(*Writer).Materialise(ctx, worktreeID string, doc discover.Document) (Result, error)`
  - `contextwrite.WriteInto(dir string, doc discover.Document) (Result, error)`
  - `contextwrite.Result{Path string; Bytes int; Created bool}`
  - `contextwrite.FileName = "CLAUDE.md"`, `ErrNotAbsolute`, `ErrNotADir`, `ErrUnknownID`

**PHASE 3 SEAM — this contract is restated in the Phase 3 plan and in the package comment:**
1. `WorktreeResolver.Path` returns an **absolute** path to an **existing directory** that is the root of a checked-out worktree. It never creates, moves or removes anything.
2. **`Materialise` runs BEFORE `ptysession.Start`, never after.** Claude Code reads `CLAUDE.md` at session start. Writing it afterwards means the session inherits nothing, silently, with no error anywhere — the whole phase looks built and is not.
3. Phase 4 never touches git. Phase 3 never renders brief content.

- [ ] **Step 1: Write the failing tests**

Create `control-plane/internal/contextwrite/writer_test.go`:

```go
package contextwrite_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/contextwrite"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

func doc(rev int) discover.Document {
	return discover.Document{
		ProjectName: "Engagement Portal",
		BranchName:  "evidence-upload",
		BriefID:     "b-42",
		Revision:    rev,
		Sections: []discover.Section{{
			Key: discover.KeyPurpose, Heading: "What this is for",
			Body: "Add evidence upload.", Origin: discover.OriginBranch,
		}},
	}
}

func TestMaterialiseWritesClaudeMdIntoTheWorktree(t *testing.T) {
	dir := t.TempDir()
	w := contextwrite.NewWriter(contextwrite.StaticResolver{"wt-1": dir})

	res, err := w.Materialise(context.Background(), "wt-1", doc(1))
	if err != nil {
		t.Fatalf("Materialise: %v", err)
	}
	if !res.Created {
		t.Error("Created = false, want true on the first write")
	}
	if res.Path != filepath.Join(dir, "CLAUDE.md") {
		t.Errorf("path = %q", res.Path)
	}

	raw, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(raw), "Add evidence upload.") {
		t.Errorf("file = %q, want the brief body", raw)
	}
	if res.Bytes != len(raw) {
		t.Errorf("Bytes = %d, want %d", res.Bytes, len(raw))
	}
}

func TestMaterialisePreservesHandWrittenContent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")
	if err := os.WriteFile(target, []byte("# House rules\n\nNever touch infra.\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	w := contextwrite.NewWriter(contextwrite.StaticResolver{"wt-1": dir})
	res, err := w.Materialise(context.Background(), "wt-1", doc(1))
	if err != nil {
		t.Fatalf("Materialise: %v", err)
	}
	if res.Created {
		t.Error("Created = true, want false when the file already existed")
	}

	raw, _ := os.ReadFile(target)
	if !strings.Contains(string(raw), "Never touch infra.") {
		t.Error("hand-written content was destroyed")
	}
}

func TestMaterialiseIsIdempotentAndReplacesTheBlockOnRewrite(t *testing.T) {
	dir := t.TempDir()
	w := contextwrite.NewWriter(contextwrite.StaticResolver{"wt-1": dir})

	if _, err := w.Materialise(context.Background(), "wt-1", doc(1)); err != nil {
		t.Fatalf("first: %v", err)
	}
	first, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))

	if _, err := w.Materialise(context.Background(), "wt-1", doc(1)); err != nil {
		t.Fatalf("second: %v", err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if string(first) != string(second) {
		t.Error("rewriting the same document changed the file; the write is not idempotent")
	}

	if _, err := w.Materialise(context.Background(), "wt-1", doc(2)); err != nil {
		t.Fatalf("third: %v", err)
	}
	third, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if strings.Contains(string(third), "rev=1") {
		t.Error("the old revision block survived a rewrite")
	}
	if strings.Count(string(third), "<!-- panacea:discover:begin") != 1 {
		t.Error("rewriting appended a second managed block")
	}
}

func TestMaterialiseLeavesNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	w := contextwrite.NewWriter(contextwrite.StaticResolver{"wt-1": dir})
	if _, err := w.Materialise(context.Background(), "wt-1", doc(1)); err != nil {
		t.Fatalf("Materialise: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "CLAUDE.md" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory = %v, want only CLAUDE.md", names)
	}
}

func TestMaterialiseRejectsAnUnknownWorktree(t *testing.T) {
	w := contextwrite.NewWriter(contextwrite.StaticResolver{})
	if _, err := w.Materialise(context.Background(), "nope", doc(1)); !errors.Is(err, contextwrite.ErrUnknownID) {
		t.Fatalf("err = %v, want ErrUnknownID", err)
	}
}

func TestWriteIntoRejectsARelativePath(t *testing.T) {
	if _, err := contextwrite.WriteInto("relative/path", doc(1)); !errors.Is(err, contextwrite.ErrNotAbsolute) {
		t.Fatalf("err = %v, want ErrNotAbsolute", err)
	}
}

func TestWriteIntoRejectsAMissingDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-there")
	if _, err := contextwrite.WriteInto(missing, doc(1)); err == nil {
		t.Fatal("expected an error for a missing worktree directory")
	}
}

func TestWriteIntoRejectsAFileWhereADirectoryIsExpected(t *testing.T) {
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := contextwrite.WriteInto(file, doc(1)); !errors.Is(err, contextwrite.ErrNotADir) {
		t.Fatalf("err = %v, want ErrNotADir", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/contextwrite/...`
Expected: FAIL — package `contextwrite` does not exist.

- [ ] **Step 3: Write the implementation**

Create `control-plane/internal/contextwrite/writer.go`:

```go
// Package contextwrite puts the generated CLAUDE.md into a worktree.
//
// PHASE SEAM — read this before changing anything here.
//
// Phase 3 owns git worktrees: creating them, listing them, removing them.
// Phase 4 owns only the content of CLAUDE.md and the act of writing it.
// The two meet at WorktreeResolver, and at one ordering rule:
//
//	CLAUDE.md MUST be on disk before the session's PTY starts.
//
// Claude Code reads CLAUDE.md at session start. Write it after the process is
// up and the session that was supposed to inherit the brief inherits nothing,
// silently, with no error anywhere. Phase 3's session-start path calls
// Materialise before ptysession.Start, not after.
package contextwrite

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

// FileName is the file agents read. Do not make it configurable: the name is
// the contract with Claude Code.
const FileName = "CLAUDE.md"

// Errors returned by this package.
var (
	ErrNotAbsolute = errors.New("contextwrite: worktree path must be absolute")
	ErrNotADir     = errors.New("contextwrite: worktree path is not a directory")
	ErrUnknownID   = errors.New("contextwrite: no worktree with that id")
)

// WorktreeResolver maps a worktree id to its checkout directory.
//
// CONTRACT — Phase 3 implements this, and this paragraph is restated in the
// Phase 3 plan:
//   - Path returns an ABSOLUTE path to an EXISTING directory that is the root
//     of a checked-out worktree.
//   - It is safe to call at any time after the worktree exists on disk.
//   - It never creates, moves or removes anything.
type WorktreeResolver interface {
	Path(ctx context.Context, worktreeID string) (string, error)
}

// StaticResolver is the Phase 4 stand-in: a fixed id-to-path map. It lets
// Discover be built, tested and demonstrated before Phase 3 exists.
type StaticResolver map[string]string

// Path implements WorktreeResolver.
func (s StaticResolver) Path(_ context.Context, worktreeID string) (string, error) {
	path, ok := s[worktreeID]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownID, worktreeID)
	}
	return path, nil
}

// Writer materialises documents into worktrees.
type Writer struct {
	resolver WorktreeResolver
}

// NewWriter returns a Writer backed by resolver.
func NewWriter(resolver WorktreeResolver) *Writer {
	return &Writer{resolver: resolver}
}

// Result describes what was written.
type Result struct {
	Path    string `json:"path"`
	Bytes   int    `json:"bytes"`
	Created bool   `json:"created"`
}

// Materialise renders doc and writes it into the worktree, preserving any
// content the user wrote outside the managed block.
func (w *Writer) Materialise(ctx context.Context, worktreeID string, doc discover.Document) (Result, error) {
	dir, err := w.resolver.Path(ctx, worktreeID)
	if err != nil {
		return Result{}, err
	}
	return WriteInto(dir, doc)
}

// WriteInto writes doc into dir/CLAUDE.md. It is exported so a caller that
// already knows the path — the local machine backend, a test — can skip the
// resolver.
func WriteInto(dir string, doc discover.Document) (Result, error) {
	if !filepath.IsAbs(dir) {
		return Result{}, fmt.Errorf("%w: %q", ErrNotAbsolute, dir)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return Result{}, fmt.Errorf("contextwrite: stat worktree: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("%w: %q", ErrNotADir, dir)
	}

	target := filepath.Join(dir, FileName)
	existing := ""
	created := true
	if raw, err := os.ReadFile(target); err == nil {
		existing = string(raw)
		created = false
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, fmt.Errorf("contextwrite: read %s: %w", FileName, err)
	}

	merged, err := claudemd.Merge(existing, claudemd.Render(doc))
	if err != nil {
		return Result{}, err
	}
	if err := writeAtomic(target, []byte(merged)); err != nil {
		return Result{}, err
	}
	return Result{Path: target, Bytes: len(merged), Created: created}, nil
}

// writeAtomic writes to a temp file in the same directory and renames over the
// target, so a crash mid-write can never leave a half-written CLAUDE.md that an
// agent then reads. os.Rename replaces an existing file on Windows as well as
// POSIX (it calls MoveFileEx with MOVEFILE_REPLACE_EXISTING).
func writeAtomic(target string, data []byte) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".claude-md-*")
	if err != nil {
		return fmt.Errorf("contextwrite: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpName)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("contextwrite: write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("contextwrite: sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("contextwrite: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("contextwrite: replace %s: %w", FileName, err)
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./control-plane/internal/contextwrite/... -v -race -count=1`
Expected: PASS — 8 tests. `TestMaterialiseLeavesNoTemporaryFiles` is the one that catches a botched atomic write on Windows.

- [ ] **Step 5: Commit**

```bash
git add control-plane/internal/contextwrite
git commit -m "feat(contextwrite): atomically write CLAUDE.md into a worktree

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Repository analysis and the drafted brief

Importing a repo: analyse the code, draft a brief, let the user correct it. The scan is mechanical so the facts shown are facts; only the *draft* comes from a model, and every drafted answer is labelled as such.

The model sees `Findings` — counts, manifest names, directory names — and never file contents. That keeps client source out of the prompt, which matters for the same ISM reason recorded against `codex-security`.

**Files:**
- Create: `control-plane/internal/repoanalysis/scan.go`, `control-plane/internal/repoanalysis/draft.go`
- Test: `control-plane/internal/repoanalysis/scan_test.go`

**Interfaces:**
- Consumes: `modelclient.Completer`, `modelclient.ExtractJSON`, `discover.Brief`, `discover.Key`, `discover.SourceDrafted`
- Produces:
  - `repoanalysis.LanguageCount{Language string; Files int}`
  - `repoanalysis.Findings{Root string; FilesSeen int; Truncated bool; Languages []LanguageCount; Manifests, TopLevel []string; TestFiles int; HasCLAUDEMD bool}`
  - `(Findings).Summary() string`
  - `repoanalysis.Scan(root string) (Findings, error)`, `repoanalysis.ErrNotADir`
  - `repoanalysis.DraftBrief(ctx, c modelclient.Completer, findings Findings, brief *discover.Brief, now time.Time) error`

- [ ] **Step 1: Write the failing tests**

Create `control-plane/internal/repoanalysis/scan_test.go`:

```go
package repoanalysis_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/repoanalysis"
)

var clock = time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)

// fixtureRepo builds a small tree that exercises every branch of Scan.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                      "module example.com/portal\n",
		"Dockerfile":                  "FROM scratch\n",
		"README.md":                   "# Portal\n",
		"services/api/routes.go":      "package api\n",
		"services/api/routes_test.go": "package api\n",
		"services/api/upload.go":      "package api\n",
		"web/app.tsx":                 "export {}\n",
		"web/app.test.tsx":            "export {}\n",
		"node_modules/dep/index.js":   "module.exports = {}\n",
		".git/config":                 "[core]\n",
	}
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

func TestScanReportsLanguagesManifestsAndTests(t *testing.T) {
	f, err := repoanalysis.Scan(fixtureRepo(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(f.Languages) == 0 || f.Languages[0].Language != "Go" || f.Languages[0].Files != 3 {
		t.Errorf("languages = %#v, want Go first with 3 files", f.Languages)
	}
	for _, want := range []string{"Dockerfile", "README.md", "go.mod"} {
		if !contains(f.Manifests, want) {
			t.Errorf("manifests = %v, want %q present", f.Manifests, want)
		}
	}
	if f.TestFiles != 2 {
		t.Errorf("TestFiles = %d, want 2", f.TestFiles)
	}
	if f.HasCLAUDEMD {
		t.Error("HasCLAUDEMD = true, but the fixture has none")
	}
}

func TestScanSkipsNodeModulesAndDotGit(t *testing.T) {
	f, err := repoanalysis.Scan(fixtureRepo(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if contains(f.TopLevel, "node_modules") || contains(f.TopLevel, ".git") {
		t.Errorf("topLevel = %v, want node_modules and .git excluded", f.TopLevel)
	}
	for _, l := range f.Languages {
		if l.Language == "JavaScript" {
			t.Error("JavaScript counted; node_modules was walked")
		}
	}
}

func TestScanIsDeterministic(t *testing.T) {
	root := fixtureRepo(t)
	first, _ := repoanalysis.Scan(root)
	second, _ := repoanalysis.Scan(root)
	if first.Summary() != second.Summary() {
		t.Errorf("summary is not stable:\n%s\n---\n%s", first.Summary(), second.Summary())
	}
}

func TestScanRejectsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "afile")
	_ = os.WriteFile(file, []byte("x"), 0o644)
	if _, err := repoanalysis.Scan(file); !errors.Is(err, repoanalysis.ErrNotADir) {
		t.Fatalf("err = %v, want ErrNotADir", err)
	}
}

func TestSummaryNamesTheFactsTheImportDialogShows(t *testing.T) {
	f, _ := repoanalysis.Scan(fixtureRepo(t))
	summary := f.Summary()
	for _, want := range []string{"Languages by file count:", "Manifests present:", "Test files:"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary missing %q:\n%s", want, summary)
		}
	}
}

type fakeCompleter struct {
	reply modelclient.Response
	err   error
	last  modelclient.Request
}

func (f *fakeCompleter) Complete(_ context.Context, req modelclient.Request) (modelclient.Response, error) {
	f.last = req
	return f.reply, f.err
}

func TestDraftBriefRecordsDraftedAnswers(t *testing.T) {
	fake := &fakeCompleter{reply: modelclient.Response{Text: `{"answers":[
		{"key":"purpose","text":"An engagement portal with a Go API and a React web app."},
		{"key":"audience","text":"Client users, confirm."},
		{"key":"nonsense","text":"dropped"}]}`}}

	brief := discover.NewBrief("b", discover.ScopeProject, "p", "", clock)
	findings, _ := repoanalysis.Scan(fixtureRepo(t))

	if err := repoanalysis.DraftBrief(context.Background(), fake, findings, brief, clock); err != nil {
		t.Fatalf("DraftBrief: %v", err)
	}
	if len(brief.Answers) != 2 {
		t.Fatalf("answers = %d, want 2 — the unknown key must be dropped, not fatal", len(brief.Answers))
	}
	for _, a := range brief.Answers {
		if a.Source != discover.SourceDrafted {
			t.Errorf("%s source = %q, want drafted", a.Key, a.Source)
		}
	}
	if fake.last.JSONSchema == nil {
		t.Error("DraftBrief must constrain the reply with a schema")
	}
	if !strings.Contains(fake.last.User, "Languages by file count:") {
		t.Error("the prompt must carry the mechanical findings")
	}
}

func TestDraftBriefFailsWhenNothingUsableComesBack(t *testing.T) {
	fake := &fakeCompleter{reply: modelclient.Response{Text: `{"answers":[{"key":"bogus","text":"x"}]}`}}
	brief := discover.NewBrief("b", discover.ScopeProject, "p", "", clock)
	findings, _ := repoanalysis.Scan(fixtureRepo(t))

	if err := repoanalysis.DraftBrief(context.Background(), fake, findings, brief, clock); err == nil {
		t.Fatal("expected an error when no answer could be recorded")
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/repoanalysis/...`
Expected: FAIL — package `repoanalysis` does not exist.

- [ ] **Step 3: Write the scanner**

Create `control-plane/internal/repoanalysis/scan.go`:

```go
// Package repoanalysis reads an existing repository so Discover can draft a
// brief the user then corrects.
//
// Scan is MECHANICAL ONLY: file counts, manifests, entry points. No model call
// happens here, so the findings shown to the user are facts, not a summary that
// might be wrong. The model sees Findings, never the source tree — and it never
// sees file contents, which keeps tenant source out of the prompt.
package repoanalysis

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// maxFiles bounds the walk so a huge monorepo cannot hang the desktop app.
const maxFiles = 20000

// skipDirs are never descended into.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "target": true,
	"dist": true, "build": true, ".next": true, ".venv": true, "__pycache__": true,
}

// languageByExt maps a file extension to the language name reported.
var languageByExt = map[string]string{
	".go": "Go", ".ts": "TypeScript", ".tsx": "TypeScript", ".js": "JavaScript",
	".jsx": "JavaScript", ".py": "Python", ".rs": "Rust", ".java": "Java",
	".cs": "C#", ".rb": "Ruby", ".php": "PHP", ".sql": "SQL", ".sh": "Shell",
	".tf": "Terraform", ".md": "Markdown", ".yaml": "YAML", ".yml": "YAML",
}

// manifests are the files whose presence says how a repo is built and run.
var manifests = []string{
	"go.mod", "package.json", "pyproject.toml", "requirements.txt", "Cargo.toml",
	"pom.xml", "build.gradle", "Gemfile", "composer.json", "Dockerfile",
	"docker-compose.yml", "docker-compose.yaml", "Makefile", "CLAUDE.md",
	"README.md", "AGENTS.md",
}

// LanguageCount is one language and how many files it owns.
type LanguageCount struct {
	Language string `json:"language"`
	Files    int    `json:"files"`
}

// Findings is everything Scan discovered. Every field is a fact that can be
// pointed at in the tree — there is no derived opinion in here.
type Findings struct {
	Root        string          `json:"root"`
	FilesSeen   int             `json:"filesSeen"`
	Truncated   bool            `json:"truncated"`
	Languages   []LanguageCount `json:"languages"`
	Manifests   []string        `json:"manifests"`
	TopLevel    []string        `json:"topLevel"`
	TestFiles   int             `json:"testFiles"`
	HasCLAUDEMD bool            `json:"hasClaudeMd"`
}

// ErrNotADir is returned when root is not a directory.
var ErrNotADir = errors.New("repoanalysis: root is not a directory")

// Scan walks root and reports what is there.
func Scan(root string) (Findings, error) {
	info, err := os.Stat(root)
	if err != nil {
		return Findings{}, fmt.Errorf("repoanalysis: stat root: %w", err)
	}
	if !info.IsDir() {
		return Findings{}, fmt.Errorf("%w: %q", ErrNotADir, root)
	}

	// Slices are initialised so the JSON carries [] rather than null, which
	// keeps the TypeScript types free of nullable arrays.
	f := Findings{
		Root:      root,
		Languages: []LanguageCount{},
		Manifests: []string{},
		TopLevel:  []string{},
	}
	byLanguage := map[string]int{}
	foundManifests := map[string]bool{}
	wanted := map[string]bool{}
	for _, m := range manifests {
		wanted[m] = true
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return Findings{}, fmt.Errorf("repoanalysis: read root: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() && !skipDirs[e.Name()] && !strings.HasPrefix(e.Name(), ".") {
			f.TopLevel = append(f.TopLevel, e.Name())
		}
	}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable directory is not a reason to fail the whole scan.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if path != root && skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if f.FilesSeen >= maxFiles {
			f.Truncated = true
			return fs.SkipAll
		}
		f.FilesSeen++

		name := d.Name()
		if wanted[name] {
			foundManifests[name] = true
		}
		if name == "CLAUDE.md" && filepath.Dir(path) == root {
			f.HasCLAUDEMD = true
		}
		if lang, ok := languageByExt[strings.ToLower(filepath.Ext(name))]; ok {
			byLanguage[lang]++
		}
		if isTestFile(name) {
			f.TestFiles++
		}
		return nil
	})
	if walkErr != nil {
		return Findings{}, fmt.Errorf("repoanalysis: walk: %w", walkErr)
	}

	for lang, n := range byLanguage {
		f.Languages = append(f.Languages, LanguageCount{Language: lang, Files: n})
	}
	sort.Slice(f.Languages, func(i, j int) bool {
		if f.Languages[i].Files != f.Languages[j].Files {
			return f.Languages[i].Files > f.Languages[j].Files
		}
		return f.Languages[i].Language < f.Languages[j].Language
	})
	for name := range foundManifests {
		f.Manifests = append(f.Manifests, name)
	}
	sort.Strings(f.Manifests)
	sort.Strings(f.TopLevel)
	return f, nil
}

func isTestFile(name string) bool {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, "_test.go"),
		strings.HasSuffix(lower, ".test.ts"), strings.HasSuffix(lower, ".test.tsx"),
		strings.HasSuffix(lower, ".spec.ts"), strings.HasSuffix(lower, ".spec.tsx"),
		strings.HasPrefix(lower, "test_") && strings.HasSuffix(lower, ".py"):
		return true
	}
	return false
}

// Summary renders Findings as the plain-English block shown in the import
// dialog and sent to the model. It is deterministic so the same repo always
// produces the same prompt, which keeps prompt caching useful.
func (f Findings) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Files scanned: %d", f.FilesSeen)
	if f.Truncated {
		fmt.Fprintf(&b, " (stopped at the %d-file limit)", maxFiles)
	}
	b.WriteString("\n")

	if len(f.Languages) > 0 {
		parts := make([]string, 0, len(f.Languages))
		for _, l := range f.Languages {
			parts = append(parts, fmt.Sprintf("%s %d", l.Language, l.Files))
		}
		fmt.Fprintf(&b, "Languages by file count: %s\n", strings.Join(parts, ", "))
	} else {
		b.WriteString("Languages by file count: none recognised\n")
	}

	if len(f.Manifests) > 0 {
		fmt.Fprintf(&b, "Manifests present: %s\n", strings.Join(f.Manifests, ", "))
	} else {
		b.WriteString("Manifests present: none\n")
	}
	if len(f.TopLevel) > 0 {
		fmt.Fprintf(&b, "Top-level directories: %s\n", strings.Join(f.TopLevel, ", "))
	}
	fmt.Fprintf(&b, "Test files: %d\n", f.TestFiles)
	fmt.Fprintf(&b, "Existing CLAUDE.md at the root: %t\n", f.HasCLAUDEMD)
	return b.String()
}
```

- [ ] **Step 4: Write the drafter**

Create `control-plane/internal/repoanalysis/draft.go`:

```go
package repoanalysis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
)

// draftSystem is the system prompt for brief drafting. It says "draft", not
// "decide": the user corrects every line before anything is written.
const draftSystem = `You are drafting a project brief for an engineering team from a mechanical scan of their repository.

You are given only file counts, manifest names and directory names. You never see source code.
Write what the evidence supports and nothing more. Where the evidence is thin, say so in the answer itself
("looks like X, confirm") rather than inventing detail.

Answer every question you are given. Each answer is one or two plain sentences, no markdown, no bullet
characters, no preamble. For the definition-of-done question, put each criterion on its own line.`

// draftSchema constrains the reply to a list of {key, text} pairs.
var draftSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["answers"],
  "properties": {
    "answers": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["key", "text"],
        "properties": {
          "key": {"type": "string"},
          "text": {"type": "string"}
        }
      }
    }
  }
}`)

type draftReply struct {
	Answers []struct {
		Key  string `json:"key"`
		Text string `json:"text"`
	} `json:"answers"`
}

// DraftBrief asks the model to fill a brief from findings. Every answer it
// returns is recorded with SourceDrafted so the surface can show it as a draft
// awaiting correction, which is the difference between a helpful starting point
// and a lie the user never agreed to.
//
// Unknown keys in the reply are dropped rather than failing the call: a
// hallucinated key must not cost the user the four good answers beside it.
func DraftBrief(ctx context.Context, c modelclient.Completer, findings Findings, brief *discover.Brief, now time.Time) error {
	prompt := "Repository scan:\n\n" + findings.Summary() + "\nQuestions, by key:\n"
	for _, q := range brief.Questions {
		prompt += fmt.Sprintf("- %s: %s\n", q.Key, q.Prompt(brief.Scope))
	}

	resp, err := c.Complete(ctx, modelclient.Request{
		System:     draftSystem,
		User:       prompt,
		Effort:     "high",
		JSONSchema: draftSchema,
	})
	if err != nil {
		return fmt.Errorf("repoanalysis: draft brief: %w", err)
	}

	raw, err := modelclient.ExtractJSON(resp.Text)
	if err != nil {
		return fmt.Errorf("repoanalysis: draft brief: %w", err)
	}
	var reply draftReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return fmt.Errorf("repoanalysis: decode draft: %w", err)
	}

	recorded := 0
	for _, a := range reply.Answers {
		if err := brief.Record(discover.Key(a.Key), a.Text, discover.SourceDrafted, now); err != nil {
			continue // unknown or empty: drop this one, keep the rest
		}
		recorded++
	}
	if recorded == 0 {
		return fmt.Errorf("repoanalysis: draft brief: the model returned no usable answers")
	}
	return nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./control-plane/internal/repoanalysis/... -v -race -count=1`
Expected: PASS — 7 tests.

- [ ] **Step 6: Commit**

```bash
git add control-plane/internal/repoanalysis
git commit -m "feat(repoanalysis): mechanical repo scan and a model-drafted brief

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: The brief store — the Phase 2 seam

**Files:**
- Create: `control-plane/internal/discoverstore/store.go`
- Test: `control-plane/internal/discoverstore/store_test.go`

**Interfaces:**
- Consumes: `discover.Brief`, `discover.Scope`
- Produces:
  - `discoverstore.Store` interface: `Save(*discover.Brief) error`, `Get(id string) (*discover.Brief, error)`, `ForProject(projectID string) (*discover.Brief, error)`, `ForBranch(branchID string) (*discover.Brief, error)`
  - `discoverstore.NewFileStore(dir string) (*FileStore, error)`
  - `discoverstore.DefaultDir() string`
  - `discoverstore.ErrNotFound`

**PHASE 2 SEAM — restated in the Phase 2 plan:** Phase 2 owns the app database. Phase 4 depends on the `Store` *interface*, never on the file layout. When Phase 2's database lands, implement `Store` over it and delete `FileStore`; no other Phase 4 file changes. `FileStore` exists so this phase is buildable and demonstrable before Phase 2 ships, and because adding a SQLite driver to a module pinned at `go 1.23` is a dependency risk this phase does not need to take.

- [ ] **Step 1: Write the failing tests**

Create `control-plane/internal/discoverstore/store_test.go`:

```go
package discoverstore_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverstore"
)

var clock = time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)

func newStore(t *testing.T) *discoverstore.FileStore {
	t.Helper()
	s, err := discoverstore.NewFileStore(filepath.Join(t.TempDir(), "briefs"))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return s
}

func TestSaveAndGetRoundTrip(t *testing.T) {
	s := newStore(t)
	b := discover.NewBrief("brief-1", discover.ScopeBranch, "p-1", "br-1", clock)
	if err := b.Record(discover.KeyPurpose, "Add evidence upload.", discover.SourceTyped, clock); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := s.Save(b); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Get("brief-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Revision != b.Revision || len(got.Answers) != 1 {
		t.Errorf("round trip lost data: %+v", got)
	}
	if got.Answers[0].Text != "Add evidence upload." {
		t.Errorf("answer = %q", got.Answers[0].Text)
	}
	if len(got.Questions) != 5 {
		t.Errorf("questions = %d, want the stored set of 5", len(got.Questions))
	}
}

func TestSaveReplacesAnEarlierRevision(t *testing.T) {
	s := newStore(t)
	b := discover.NewBrief("brief-1", discover.ScopeBranch, "p-1", "br-1", clock)
	_ = s.Save(b)
	_ = b.Record(discover.KeyPurpose, "Later", discover.SourceTyped, clock)
	_ = s.Save(b)

	got, err := s.Get("brief-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Answers) != 1 {
		t.Errorf("answers = %d, want the second save to win", len(got.Answers))
	}
}

func TestForBranchAndForProjectSelectByScope(t *testing.T) {
	s := newStore(t)
	_ = s.Save(discover.NewBrief("brief-p", discover.ScopeProject, "p-1", "", clock))
	_ = s.Save(discover.NewBrief("brief-b", discover.ScopeBranch, "p-1", "br-1", clock))

	p, err := s.ForProject("p-1")
	if err != nil {
		t.Fatalf("ForProject: %v", err)
	}
	if p.ID != "brief-p" {
		t.Errorf("ForProject = %q, want brief-p", p.ID)
	}

	b, err := s.ForBranch("br-1")
	if err != nil {
		t.Fatalf("ForBranch: %v", err)
	}
	if b.ID != "brief-b" {
		t.Errorf("ForBranch = %q, want brief-b", b.ID)
	}
}

func TestMissingBriefsAreErrNotFound(t *testing.T) {
	s := newStore(t)
	if _, err := s.Get("nope"); !errors.Is(err, discoverstore.ErrNotFound) {
		t.Errorf("Get = %v, want ErrNotFound", err)
	}
	if _, err := s.ForBranch("nope"); !errors.Is(err, discoverstore.ErrNotFound) {
		t.Errorf("ForBranch = %v, want ErrNotFound", err)
	}
	if _, err := s.ForProject("nope"); !errors.Is(err, discoverstore.ErrNotFound) {
		t.Errorf("ForProject = %v, want ErrNotFound", err)
	}
}

func TestUnsafeIdsAreRejected(t *testing.T) {
	s := newStore(t)
	for _, id := range []string{"", "../escape", "a/b", `a\b`, "dotted.id"} {
		b := discover.NewBrief(id, discover.ScopeBranch, "p", "br", clock)
		if err := s.Save(b); err == nil {
			t.Errorf("Save accepted unsafe id %q", id)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/discoverstore/...`
Expected: FAIL — package `discoverstore` does not exist.

- [ ] **Step 3: Write the implementation**

Create `control-plane/internal/discoverstore/store.go`:

```go
// Package discoverstore persists briefs as one JSON file each.
//
// PHASE SEAM: Phase 2 owns the app's database. This store exists so Phase 4 is
// buildable and demonstrable before that database lands, and so Discover adds
// no dependency to a module pinned at go 1.23. When Phase 2's store arrives,
// implement discoverstore.Store over it and delete FileStore — the interface
// is what the API depends on, not the file layout.
package discoverstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

// ErrNotFound is returned when no brief matches.
var ErrNotFound = errors.New("discoverstore: no such brief")

// Store is the persistence seam the HTTP layer depends on.
type Store interface {
	Save(b *discover.Brief) error
	Get(id string) (*discover.Brief, error)
	ForProject(projectID string) (*discover.Brief, error)
	ForBranch(branchID string) (*discover.Brief, error)
}

// FileStore keeps one JSON file per brief in a directory.
type FileStore struct {
	mu  sync.RWMutex
	dir string
}

// NewFileStore creates dir if needed and returns a store over it.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("discoverstore: create state dir: %w", err)
	}
	return &FileStore{dir: dir}, nil
}

// DefaultDir is where briefs live when PANACEA_APP_STATE_DIR is unset.
// LOCALAPPDATA on Windows, XDG_STATE_HOME or ~/.local/state elsewhere.
func DefaultDir() string {
	if dir := os.Getenv("PANACEA_APP_STATE_DIR"); dir != "" {
		return filepath.Join(dir, "discover")
	}
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		return filepath.Join(dir, "PanaceaApp", "state", "discover")
	}
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "panacea-app", "discover")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "panacea-app", "discover")
	}
	return filepath.Join(home, ".local", "state", "panacea-app", "discover")
}

// safeName rejects ids that would escape the state directory.
func safeName(id string) (string, error) {
	if id == "" || strings.ContainsAny(id, `/\.`) {
		return "", fmt.Errorf("discoverstore: unsafe brief id %q", id)
	}
	return id + ".json", nil
}

// Save writes a brief, replacing any earlier version.
func (s *FileStore) Save(b *discover.Brief) error {
	name, err := safeName(b.ID)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return fmt.Errorf("discoverstore: encode brief: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	target := filepath.Join(s.dir, name)
	tmp, err := os.CreateTemp(s.dir, ".brief-*")
	if err != nil {
		return fmt.Errorf("discoverstore: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("discoverstore: write brief: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("discoverstore: close brief: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("discoverstore: replace brief: %w", err)
	}
	return nil
}

// Get returns one brief by id.
func (s *FileStore) Get(id string) (*discover.Brief, error) {
	name, err := safeName(id)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	raw, err := os.ReadFile(filepath.Join(s.dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if err != nil {
		return nil, fmt.Errorf("discoverstore: read brief: %w", err)
	}
	var b discover.Brief
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("discoverstore: decode brief %s: %w", id, err)
	}
	return &b, nil
}

// ForProject returns the project-scope brief for a project.
func (s *FileStore) ForProject(projectID string) (*discover.Brief, error) {
	return s.find(func(b *discover.Brief) bool {
		return b.Scope == discover.ScopeProject && b.ProjectID == projectID
	}, "project "+projectID)
}

// ForBranch returns the branch-scope brief for a branch.
func (s *FileStore) ForBranch(branchID string) (*discover.Brief, error) {
	return s.find(func(b *discover.Brief) bool {
		return b.Scope == discover.ScopeBranch && b.BranchID == branchID
	}, "branch "+branchID)
}

// find scans the directory. The local corpus is one file per branch, so a scan
// is cheap; when Phase 2's database replaces this, it becomes an indexed query.
func (s *FileStore) find(match func(*discover.Brief) bool, what string) (*discover.Brief, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("discoverstore: list state dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // deterministic when more than one somehow matches

	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(s.dir, name))
		if err != nil {
			continue
		}
		var b discover.Brief
		if err := json.Unmarshal(raw, &b); err != nil {
			continue
		}
		if match(&b) {
			return &b, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNotFound, what)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./control-plane/internal/discoverstore/... -v -race -count=1`
Expected: PASS — 5 tests.

- [ ] **Step 5: Commit**

```bash
git add control-plane/internal/discoverstore
git commit -m "feat(discoverstore): file-backed brief store behind a Store interface

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Provenance — the one type Develop, Merge and Deploy share

`CLAUDE.md` line 13: *"the spec written in Define is visibly referenced in Design, carried into Develop, and attached to the release in Deploy. Same artifact, one thread, shown throughout. Any feature that breaks that thread is wrong regardless of how good it is in isolation."*

The mechanism that makes the thread unbreakable is that all three surfaces call **one endpoint** and render **one type**. If they each derived their own view of the brief, they would drift, and the drift would be invisible.

**Files:**
- Create: `control-plane/internal/provenance/provenance.go`
- Test: `control-plane/internal/provenance/provenance_test.go`

**Interfaces:**
- Consumes: `discover.Document`, `discover.Key*`, `discover.SplitCriteria`
- Produces:
  - `provenance.CriterionState` with `Settled`, `Open`, `Contested`
  - `provenance.Criterion{ID, Text string; State CriterionState; Because string}`
  - `provenance.Ref{BriefID string; Revision int; ProjectID, BranchID, Headline, Audience, Constraints, OutOfScope string; Criteria []Criterion; ContextPath string; Present bool}`
  - `provenance.Build(projectID, branchID string, doc discover.Document, contextPath string) Ref`

**Deliberately absent, and this is a design decision not an omission:** there is no `Met int`, no `Total int`, no percentage and no completion tick anywhere in `Ref`. `docs/AGENTIC-UX.md` deletes "definition of done — 2 of 4 met" as a progress bar wearing a text disguise. Convergence (`settled` / `open` / `contested`) replaces it. A criterion may not carry a non-`Open` state without a `Because` naming the evidence (principle 7).

- [ ] **Step 1: Write the failing tests**

Create `control-plane/internal/provenance/provenance_test.go`:

```go
package provenance_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

var clock = time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)

func resolved(t *testing.T) discover.Document {
	t.Helper()
	b := discover.NewBrief("b-42", discover.ScopeBranch, "p-1", "br-1", clock)
	_ = b.Record(discover.KeyPurpose, "Add evidence upload.", discover.SourceTyped, clock)
	_ = b.Record(discover.KeyAudience, "Client users", discover.SourcePicked, clock)
	_ = b.Record(discover.KeyDone, "Tests pass\nNo API break", discover.SourceTyped, clock)
	_ = b.Record(discover.KeyOutOfScope, "Visual design", discover.SourcePicked, clock)
	return discover.Resolve("Engagement Portal", "evidence-upload", nil, b)
}

func TestBuildCarriesTheHeadlineAndTheCriteria(t *testing.T) {
	ref := provenance.Build("p-1", "br-1", resolved(t), `C:\work\wt\CLAUDE.md`)

	if !ref.Present {
		t.Error("Present = false, want true")
	}
	if ref.Headline != "Add evidence upload." {
		t.Errorf("Headline = %q", ref.Headline)
	}
	if ref.Audience != "Client users" {
		t.Errorf("Audience = %q", ref.Audience)
	}
	if ref.OutOfScope != "Visual design" {
		t.Errorf("OutOfScope = %q", ref.OutOfScope)
	}
	if ref.BriefID != "b-42" {
		t.Errorf("BriefID = %q", ref.BriefID)
	}
	if ref.ContextPath == "" {
		t.Error("ContextPath is empty; Deploy needs the file that shipped")
	}
	if len(ref.Criteria) != 2 {
		t.Fatalf("criteria = %#v, want 2", ref.Criteria)
	}
	if ref.Criteria[0].Text != "Tests pass" || ref.Criteria[1].Text != "No API break" {
		t.Errorf("criteria text = %#v", ref.Criteria)
	}
}

func TestEveryCriterionStartsOpenWithNoEvidence(t *testing.T) {
	ref := provenance.Build("p-1", "br-1", resolved(t), "")
	for _, c := range ref.Criteria {
		if c.State != provenance.Open {
			t.Errorf("%q state = %q, want open", c.Text, c.State)
		}
		if c.Because != "" {
			t.Errorf("%q carries evidence %q while Open", c.Text, c.Because)
		}
	}
}

func TestCriterionIdsAreStableAcrossRevisions(t *testing.T) {
	first := provenance.Build("p-1", "br-1", resolved(t), "")
	second := provenance.Build("p-1", "br-1", resolved(t), "")
	for i := range first.Criteria {
		if first.Criteria[i].ID != second.Criteria[i].ID {
			t.Errorf("criterion %d id changed: %q then %q", i, first.Criteria[i].ID, second.Criteria[i].ID)
		}
	}
	if !strings.HasPrefix(first.Criteria[0].ID, "b-42:done:") {
		t.Errorf("id = %q, want it anchored to the brief", first.Criteria[0].ID)
	}
}

func TestBuildWithNoBriefIsNotPresent(t *testing.T) {
	ref := provenance.Build("p-1", "br-1", discover.Document{}, "")
	if ref.Present {
		t.Error("Present = true for an empty document")
	}
	if ref.Criteria == nil {
		t.Error("Criteria is nil; it must marshal as [] so the UI can map over it")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/provenance/...`
Expected: FAIL — package `provenance` does not exist.

- [ ] **Step 3: Write the implementation**

Create `control-plane/internal/provenance/provenance.go`:

```go
// Package provenance turns a resolved Discover document into the one object
// that Develop, Merge and Deploy all render.
//
// This is the product spine (CLAUDE.md, line 13): the spec written in Discover
// is visibly referenced in Develop as a context chip, in Merge as the definition
// of done, and in Deploy as what shipped. Same artifact, one thread. Those three
// surfaces call ONE endpoint and share ONE type, so the thread cannot drift
// between them — if it could, it would.
package provenance

import (
	"strconv"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
)

// CriterionState is convergence, not progress.
//
// docs/AGENTIC-UX.md deletes "definition of done — 2 of 4 met": a progress bar
// wearing a text disguise. A criterion is settled, open, or contested; there is
// no denominator, no percentage and no tick count anywhere in this type.
type CriterionState string

const (
	// Settled: the criterion has been met and something says so.
	Settled CriterionState = "settled"
	// Open: nothing has claimed it either way. This is the default.
	Open CriterionState = "open"
	// Contested: two sources disagree, or work landed that undoes it.
	Contested CriterionState = "contested"
)

// Criterion is one line of the definition of done.
type Criterion struct {
	ID    string         `json:"id"`
	Text  string         `json:"text"`
	State CriterionState `json:"state"`
	// Because names the evidence for the state. Empty while Open. A UI must not
	// show a non-Open state without it (docs/AGENTIC-UX.md principle 7).
	Because string `json:"because,omitempty"`
}

// Ref is what every provenance surface renders.
type Ref struct {
	BriefID   string `json:"briefId"`
	Revision  int    `json:"revision"`
	ProjectID string `json:"projectId"`
	BranchID  string `json:"branchId"`
	// Headline is the purpose answer, shown on the Develop context chip.
	Headline string `json:"headline"`
	// Audience and Constraints are shown when the chip is expanded.
	Audience    string      `json:"audience,omitempty"`
	Constraints string      `json:"constraints,omitempty"`
	OutOfScope  string      `json:"outOfScope,omitempty"`
	Criteria    []Criterion `json:"criteria"`
	// ContextPath is the CLAUDE.md the sessions on this branch actually read.
	// Empty until the brief has been materialised into a worktree.
	ContextPath string `json:"contextPath,omitempty"`
	// Present is false when no brief exists yet. Surfaces render their own
	// "no brief yet, run Discover" state rather than an empty chip.
	Present bool `json:"present"`
}

// Build assembles a Ref from a resolved document.
func Build(projectID, branchID string, doc discover.Document, contextPath string) Ref {
	ref := Ref{
		BriefID:     doc.BriefID,
		Revision:    doc.Revision,
		ProjectID:   projectID,
		BranchID:    branchID,
		Criteria:    []Criterion{},
		ContextPath: contextPath,
		Present:     len(doc.Sections) > 0,
	}
	for _, s := range doc.Sections {
		switch s.Key {
		case discover.KeyPurpose:
			ref.Headline = s.Body
		case discover.KeyAudience:
			ref.Audience = s.Body
		case discover.KeyConstraints:
			ref.Constraints = s.Body
		case discover.KeyOutOfScope:
			ref.OutOfScope = s.Body
		case discover.KeyDone:
			items := s.Items
			if len(items) == 0 {
				items = discover.SplitCriteria(s.Body)
			}
			for i, text := range items {
				ref.Criteria = append(ref.Criteria, Criterion{
					ID:    criterionID(doc.BriefID, i),
					Text:  text,
					State: Open,
				})
			}
		}
	}
	return ref
}

// criterionID is stable across revisions for the same position, so a state set
// on a criterion survives an unrelated edit elsewhere in the brief.
func criterionID(briefID string, index int) string {
	return briefID + ":done:" + strconv.Itoa(index)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./control-plane/internal/provenance/... -v -race -count=1`
Expected: PASS — 4 tests.

- [ ] **Step 5: Commit**

```bash
git add control-plane/internal/provenance
git commit -m "feat(provenance): one ref shared by develop, merge and deploy

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: The Discover HTTP API

**Files:**
- Create: `control-plane/internal/discoverapi/api.go`
- Test: `control-plane/internal/discoverapi/api_test.go`
- Modify: `control-plane/cmd/api/main.go`

**Interfaces:**
- Consumes: everything above
- Produces:
  - `discoverapi.Naming` interface: `ProjectName(projectID string) string`, `BranchName(branchID string) string`
  - `discoverapi.StaticNaming map[string]string`
  - `discoverapi.Deps{Store discoverstore.Store; Writer *contextwrite.Writer; Model modelclient.Completer; Naming Naming; NewID IDFunc; Now func() time.Time; ContextOf func(branchID string) string}`
  - `discoverapi.New(deps Deps) *API` implementing `http.Handler`
  - Routes, all under `/api/v1/discover`:

| Method | Path | Body | Response |
|---|---|---|---|
| POST | `/briefs` | `{scope, projectId, branchId}` | 201 `{brief, step}` |
| GET | `/briefs/{briefID}` | — | 200 `{brief, step}` |
| GET | `/branches/{branchID}/brief` | — | 200 `{brief, step}` or 404 |
| POST | `/briefs/{briefID}/answers` | `{key, text, source}` | 200 `{brief, step}` |
| PATCH | `/briefs/{briefID}/answers/{key}` | `{text}` | 200 `{brief, step}` |
| POST | `/briefs/{briefID}/skip` | — | 200 `{brief, step}` |
| POST | `/briefs/{briefID}/materialise` | `{worktreeId}` | 200 `{path, bytes, created}` |
| POST | `/analyse` | `{repoPath, projectId}` | 200 `{findings, summary, view, drafted, draftedNote}` |
| GET | `/branches/{branchID}/provenance` | — | 200 `Ref` (never 404) |

**PHASE 2 SEAM, restated:** `Naming` is Phase 2's project model reduced to two strings. Both methods return a human-readable name, or the id itself when the object is unknown, and **never** return an error — a missing display name must not fail a brief. Phase 4 ships `StaticNaming` so it can run alone.

**Contract with the desktop (Task 10):** every JSON field name below is mirrored by `desktop/src/lib/discoverApi.ts`. Changing one without the other is the defect that comment exists to prevent. Route patterns use Go 1.22+ `ServeMux` method-and-wildcard syntax, which is available on the pinned 1.23 toolchain.

- [ ] **Step 1: Write the failing tests**

Create `control-plane/internal/discoverapi/api_test.go`:

```go
package discoverapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/contextwrite"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverstore"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

var clock = time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)

type harness struct {
	api      *discoverapi.API
	store    *discoverstore.FileStore
	worktree string
}

func newHarness(t *testing.T, model modelclient.Completer) *harness {
	t.Helper()
	store, err := discoverstore.NewFileStore(filepath.Join(t.TempDir(), "briefs"))
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	worktree := t.TempDir()
	counter := 0
	api := discoverapi.New(discoverapi.Deps{
		Store:  store,
		Writer: contextwrite.NewWriter(contextwrite.StaticResolver{"wt-1": worktree}),
		Model:  model,
		Naming: discoverapi.StaticNaming{"p-1": "Engagement Portal", "br-1": "evidence-upload"},
		NewID:  func() string { counter++; return fmt.Sprintf("brief-%d", counter) },
		Now:    func() time.Time { return clock },
	})
	return &harness{api: api, store: store, worktree: worktree}
}

func (h *harness) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader([]byte("{}"))
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	h.api.ServeHTTP(rec, req)
	return rec
}

type view struct {
	Brief discover.Brief `json:"brief"`
	Step  discover.Step  `json:"step"`
}

func decodeView(t *testing.T, rec *httptest.ResponseRecorder) view {
	t.Helper()
	var v view
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode view: %v (body %s)", err, rec.Body.String())
	}
	return v
}

func TestCreateAnswerAndCompleteABranchBrief(t *testing.T) {
	h := newHarness(t, nil)

	rec := h.do(t, http.MethodPost, "/api/v1/discover/briefs", map[string]string{
		"scope": "branch", "projectId": "p-1", "branchId": "br-1",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d, body %s", rec.Code, rec.Body)
	}
	v := decodeView(t, rec)
	if v.Step.Kind != discover.StepQuestion || v.Step.Total != 5 {
		t.Errorf("step = %q total %d, want question/5", v.Step.Kind, v.Step.Total)
	}

	id := v.Brief.ID
	for _, q := range discover.BaseQuestions() {
		rec = h.do(t, http.MethodPost, "/api/v1/discover/briefs/"+id+"/answers", map[string]string{
			"key": string(q.Key), "text": "answer for " + string(q.Key), "source": "picked",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("answer %s = %d, body %s", q.Key, rec.Code, rec.Body)
		}
	}
	v = decodeView(t, rec)
	if v.Brief.Status != discover.StatusComplete {
		t.Errorf("status = %q, want complete", v.Brief.Status)
	}
	if v.Step.Total != 0 {
		t.Errorf("complete step total = %d, want 0", v.Step.Total)
	}
}

func TestEditAnswerKeepsTheRest(t *testing.T) {
	h := newHarness(t, nil)
	v := decodeView(t, h.do(t, http.MethodPost, "/api/v1/discover/briefs", map[string]string{
		"scope": "branch", "projectId": "p-1", "branchId": "br-1",
	}))
	id := v.Brief.ID
	for _, q := range discover.BaseQuestions() {
		h.do(t, http.MethodPost, "/api/v1/discover/briefs/"+id+"/answers", map[string]string{
			"key": string(q.Key), "text": "answer for " + string(q.Key),
		})
	}

	rec := h.do(t, http.MethodPatch, "/api/v1/discover/briefs/"+id+"/answers/audience",
		map[string]string{"text": "Client reviewers only"})
	if rec.Code != http.StatusOK {
		t.Fatalf("edit = %d, body %s", rec.Code, rec.Body)
	}
	v = decodeView(t, rec)
	if len(v.Brief.Answers) != 5 {
		t.Fatalf("answers = %d, want 5", len(v.Brief.Answers))
	}
	if v.Brief.Answers[4].Text != "answer for out_of_scope" {
		t.Errorf("last answer = %q, want untouched", v.Brief.Answers[4].Text)
	}
}

func TestEditRejectsAnUnansweredKey(t *testing.T) {
	h := newHarness(t, nil)
	v := decodeView(t, h.do(t, http.MethodPost, "/api/v1/discover/briefs", map[string]string{
		"scope": "branch", "projectId": "p-1", "branchId": "br-1",
	}))
	rec := h.do(t, http.MethodPatch, "/api/v1/discover/briefs/"+v.Brief.ID+"/answers/done",
		map[string]string{"text": "x"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("edit = %d, want 400", rec.Code)
	}
}

func TestMaterialiseWritesTheInheritedClaudeMd(t *testing.T) {
	h := newHarness(t, nil)

	project := decodeView(t, h.do(t, http.MethodPost, "/api/v1/discover/briefs", map[string]string{
		"scope": "project", "projectId": "p-1",
	}))
	h.do(t, http.MethodPost, "/api/v1/discover/briefs/"+project.Brief.ID+"/answers",
		map[string]string{"key": "audience", "text": "Client users"})

	branch := decodeView(t, h.do(t, http.MethodPost, "/api/v1/discover/briefs", map[string]string{
		"scope": "branch", "projectId": "p-1", "branchId": "br-1",
	}))
	h.do(t, http.MethodPost, "/api/v1/discover/briefs/"+branch.Brief.ID+"/answers",
		map[string]string{"key": "purpose", "text": "Add evidence upload."})

	rec := h.do(t, http.MethodPost, "/api/v1/discover/briefs/"+branch.Brief.ID+"/materialise",
		map[string]string{"worktreeId": "wt-1"})
	if rec.Code != http.StatusOK {
		t.Fatalf("materialise = %d, body %s", rec.Code, rec.Body)
	}

	raw, err := os.ReadFile(filepath.Join(h.worktree, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "Add evidence upload.") {
		t.Error("branch answer missing from CLAUDE.md")
	}
	if !strings.Contains(body, "Client users") {
		t.Error("project answer was not inherited into CLAUDE.md")
	}
	if !strings.Contains(body, "Engagement Portal — evidence-upload") {
		t.Errorf("title missing; got:\n%s", body)
	}
}

func TestMaterialiseRejectsAnUnknownWorktree(t *testing.T) {
	h := newHarness(t, nil)
	v := decodeView(t, h.do(t, http.MethodPost, "/api/v1/discover/briefs", map[string]string{
		"scope": "branch", "projectId": "p-1", "branchId": "br-1",
	}))
	rec := h.do(t, http.MethodPost, "/api/v1/discover/briefs/"+v.Brief.ID+"/materialise",
		map[string]string{"worktreeId": "nope"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("materialise = %d, want 400", rec.Code)
	}
}

func TestProvenanceCarriesTheThreadToDevelopMergeAndDeploy(t *testing.T) {
	h := newHarness(t, nil)
	v := decodeView(t, h.do(t, http.MethodPost, "/api/v1/discover/briefs", map[string]string{
		"scope": "branch", "projectId": "p-1", "branchId": "br-1",
	}))
	id := v.Brief.ID
	h.do(t, http.MethodPost, "/api/v1/discover/briefs/"+id+"/answers",
		map[string]string{"key": "purpose", "text": "Add evidence upload."})
	h.do(t, http.MethodPost, "/api/v1/discover/briefs/"+id+"/answers",
		map[string]string{"key": "done", "text": "Tests pass\nNo API break"})

	rec := h.do(t, http.MethodGet, "/api/v1/discover/branches/br-1/provenance", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("provenance = %d, body %s", rec.Code, rec.Body)
	}
	var ref provenance.Ref
	if err := json.Unmarshal(rec.Body.Bytes(), &ref); err != nil {
		t.Fatalf("decode ref: %v", err)
	}
	if !ref.Present {
		t.Error("Present = false, want true")
	}
	if ref.Headline != "Add evidence upload." {
		t.Errorf("headline = %q", ref.Headline)
	}
	if len(ref.Criteria) != 2 {
		t.Fatalf("criteria = %#v, want 2", ref.Criteria)
	}
	for _, c := range ref.Criteria {
		if c.State != provenance.Open {
			t.Errorf("criterion %q state = %q, want open", c.Text, c.State)
		}
	}
	if strings.Contains(rec.Body.String(), "percent") || strings.Contains(rec.Body.String(), "\"met\"") {
		t.Error("provenance must carry convergence, never a progress denominator")
	}
}

func TestProvenanceForABranchWithNoBriefIsAnEmptyRefNotAnError(t *testing.T) {
	h := newHarness(t, nil)
	rec := h.do(t, http.MethodGet, "/api/v1/discover/branches/br-none/provenance", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("provenance = %d, want 200", rec.Code)
	}
	var ref provenance.Ref
	_ = json.Unmarshal(rec.Body.Bytes(), &ref)
	if ref.Present {
		t.Error("Present = true for a branch with no brief")
	}
}

type fakeCompleter struct{ text string }

func (f fakeCompleter) Complete(_ context.Context, _ modelclient.Request) (modelclient.Response, error) {
	return modelclient.Response{Text: f.text}, nil
}

func TestAnalyseDraftsABriefFromARepo(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	h := newHarness(t, fakeCompleter{text: `{"answers":[
		{"key":"purpose","text":"A Go service. Confirm."},
		{"key":"audience","text":"Our own team"}]}`})

	rec := h.do(t, http.MethodPost, "/api/v1/discover/analyse", map[string]string{
		"repoPath": repo, "projectId": "p-1",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("analyse = %d, body %s", rec.Code, rec.Body)
	}
	var resp struct {
		Summary     string `json:"summary"`
		Drafted     bool   `json:"drafted"`
		DraftedNote string `json:"draftedNote"`
		View        view   `json:"view"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Drafted {
		t.Errorf("Drafted = false, note %q", resp.DraftedNote)
	}
	if len(resp.View.Brief.Answers) != 2 {
		t.Fatalf("answers = %d, want 2", len(resp.View.Brief.Answers))
	}
	for _, a := range resp.View.Brief.Answers {
		if a.Source != discover.SourceDrafted {
			t.Errorf("%s source = %q, want drafted", a.Key, a.Source)
		}
	}
	if !strings.Contains(resp.Summary, "Manifests present: go.mod") {
		t.Errorf("summary = %q", resp.Summary)
	}
}

func TestAnalyseWithoutAModelIsHonestAboutIt(t *testing.T) {
	repo := t.TempDir()
	h := newHarness(t, nil)

	rec := h.do(t, http.MethodPost, "/api/v1/discover/analyse", map[string]string{
		"repoPath": repo, "projectId": "p-1",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("analyse = %d, body %s", rec.Code, rec.Body)
	}
	var resp struct {
		Drafted     bool   `json:"drafted"`
		DraftedNote string `json:"draftedNote"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Drafted {
		t.Error("Drafted = true with no model configured")
	}
	if !strings.Contains(resp.DraftedNote, "PANACEA_APP_MODEL_API_KEY") {
		t.Errorf("note = %q, want it to name the missing setting", resp.DraftedNote)
	}
}

func TestUnknownBriefIs404(t *testing.T) {
	h := newHarness(t, nil)
	if rec := h.do(t, http.MethodGet, "/api/v1/discover/briefs/missing", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("get = %d, want 404", rec.Code)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/discoverapi/...`
Expected: FAIL — package `discoverapi` does not exist.

- [ ] **Step 3: Write the API scaffolding**

Create `control-plane/internal/discoverapi/api.go` with this first half:

```go
// Package discoverapi serves the Discover module over HTTP.
//
// Every JSON field name here is mirrored by desktop/src/lib/discoverApi.ts.
// Changing one without the other is the defect this comment exists to prevent.
package discoverapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/contextwrite"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverstore"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/repoanalysis"
)

// Naming resolves display names for a project and a branch. Phase 2 owns the
// project model; Phase 4 needs two strings from it and nothing else.
//
// CONTRACT — restated in the Phase 2 plan: both methods return a
// human-readable name, or the id itself when the object is unknown. They never
// return an error, because a missing display name must not fail a brief.
type Naming interface {
	ProjectName(projectID string) string
	BranchName(branchID string) string
}

// StaticNaming is the Phase 4 stand-in for Phase 2's project model.
type StaticNaming map[string]string

// ProjectName implements Naming.
func (s StaticNaming) ProjectName(projectID string) string { return s.lookup(projectID) }

// BranchName implements Naming.
func (s StaticNaming) BranchName(branchID string) string { return s.lookup(branchID) }

func (s StaticNaming) lookup(id string) string {
	if name, ok := s[id]; ok {
		return name
	}
	return id
}

// IDFunc mints brief ids. Injected so tests are deterministic.
type IDFunc func() string

// Deps is everything the API needs.
type Deps struct {
	Store     discoverstore.Store
	Writer    *contextwrite.Writer
	Model     modelclient.Completer // may be nil: Discover works without a model
	Naming    Naming
	NewID     IDFunc
	Now       func() time.Time
	ContextOf func(branchID string) string // path of the CLAUDE.md last written
}

// API is an http.Handler serving /api/v1/discover/...
type API struct {
	deps Deps
	mux  *http.ServeMux
}

// New wires the routes. Now and NewID default to the real clock and a
// time-ordered id.
func New(deps Deps) *API {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.NewID == nil {
		var counter int
		deps.NewID = func() string {
			counter++
			return fmt.Sprintf("br%d-%d", time.Now().UnixNano(), counter)
		}
	}
	if deps.Naming == nil {
		deps.Naming = StaticNaming{}
	}
	if deps.ContextOf == nil {
		deps.ContextOf = func(string) string { return "" }
	}

	a := &API{deps: deps, mux: http.NewServeMux()}
	a.mux.HandleFunc("POST /api/v1/discover/briefs", a.createBrief)
	a.mux.HandleFunc("GET /api/v1/discover/briefs/{briefID}", a.getBrief)
	a.mux.HandleFunc("GET /api/v1/discover/branches/{branchID}/brief", a.getBranchBrief)
	a.mux.HandleFunc("POST /api/v1/discover/briefs/{briefID}/answers", a.recordAnswer)
	a.mux.HandleFunc("PATCH /api/v1/discover/briefs/{briefID}/answers/{key}", a.editAnswer)
	a.mux.HandleFunc("POST /api/v1/discover/briefs/{briefID}/skip", a.skipBrief)
	a.mux.HandleFunc("POST /api/v1/discover/briefs/{briefID}/materialise", a.materialise)
	a.mux.HandleFunc("POST /api/v1/discover/analyse", a.analyse)
	a.mux.HandleFunc("GET /api/v1/discover/branches/{branchID}/provenance", a.provenanceFor)
	return a
}

// ServeHTTP implements http.Handler.
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) { a.mux.ServeHTTP(w, r) }

// briefView is the wire shape: the brief plus the step to render, so the UI
// never recomputes the flow and the two can never disagree.
type briefView struct {
	Brief *discover.Brief `json:"brief"`
	Step  discover.Step   `json:"step"`
}

func (a *API) view(b *discover.Brief) briefView { return briefView{Brief: b, Step: b.Step()} }

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func decode(r *http.Request, into any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}

func (a *API) load(w http.ResponseWriter, r *http.Request) (*discover.Brief, bool) {
	b, err := a.deps.Store.Get(r.PathValue("briefID"))
	if errors.Is(err, discoverstore.ErrNotFound) {
		writeErr(w, http.StatusNotFound, err)
		return nil, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return nil, false
	}
	return b, true
}
```

- [ ] **Step 4: Write the brief handlers**

Append to `control-plane/internal/discoverapi/api.go`:

```go
type createBriefRequest struct {
	Scope     discover.Scope `json:"scope"`
	ProjectID string         `json:"projectId"`
	BranchID  string         `json:"branchId"`
}

func (a *API) createBrief(w http.ResponseWriter, r *http.Request) {
	var req createBriefRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Scope != discover.ScopeProject && req.Scope != discover.ScopeBranch {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("scope must be project or branch"))
		return
	}
	if req.ProjectID == "" || (req.Scope == discover.ScopeBranch && req.BranchID == "") {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("projectId, and branchId for a branch brief, are required"))
		return
	}

	b := discover.NewBrief(a.deps.NewID(), req.Scope, req.ProjectID, req.BranchID, a.deps.Now())
	if err := a.deps.Store.Save(b); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, a.view(b))
}

func (a *API) getBrief(w http.ResponseWriter, r *http.Request) {
	if b, ok := a.load(w, r); ok {
		writeJSON(w, http.StatusOK, a.view(b))
	}
}

func (a *API) getBranchBrief(w http.ResponseWriter, r *http.Request) {
	b, err := a.deps.Store.ForBranch(r.PathValue("branchID"))
	if errors.Is(err, discoverstore.ErrNotFound) {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, a.view(b))
}

type answerRequest struct {
	Key    discover.Key    `json:"key"`
	Text   string          `json:"text"`
	Source discover.Source `json:"source"`
}

func (a *API) recordAnswer(w http.ResponseWriter, r *http.Request) {
	b, ok := a.load(w, r)
	if !ok {
		return
	}
	var req answerRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Source == "" {
		req.Source = discover.SourceTyped
	}
	if err := b.Record(req.Key, req.Text, req.Source, a.deps.Now()); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// One model call at the end of the base set, not per answer.
	if b.Step().Kind == discover.StepComplete && a.deps.Model != nil {
		discover.ProposeFollowUp(r.Context(), a.deps.Model, b, a.hint(b), a.deps.Now())
	}
	if err := a.deps.Store.Save(b); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, a.view(b))
}

type editRequest struct {
	Text string `json:"text"`
}

func (a *API) editAnswer(w http.ResponseWriter, r *http.Request) {
	b, ok := a.load(w, r)
	if !ok {
		return
	}
	var req editRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// Edit replaces one answer in place. It must never truncate the list.
	if err := b.Edit(discover.Key(r.PathValue("key")), req.Text, a.deps.Now()); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := a.deps.Store.Save(b); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, a.view(b))
}

func (a *API) skipBrief(w http.ResponseWriter, r *http.Request) {
	b, ok := a.load(w, r)
	if !ok {
		return
	}
	b.Skip(a.deps.Now())
	if err := a.deps.Store.Save(b); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, a.view(b))
}
```

- [ ] **Step 5: Write the inheritance, materialise, analyse and provenance handlers**

Append to `control-plane/internal/discoverapi/api.go`:

```go
// resolve builds the inherited document for a brief.
func (a *API) resolve(b *discover.Brief) discover.Document {
	projectName := a.deps.Naming.ProjectName(b.ProjectID)
	if b.Scope == discover.ScopeProject {
		return discover.Resolve(projectName, "", b, nil)
	}
	var project *discover.Brief
	if p, err := a.deps.Store.ForProject(b.ProjectID); err == nil {
		project = p
	}
	return discover.Resolve(projectName, a.deps.Naming.BranchName(b.BranchID), project, b)
}

func (a *API) hint(b *discover.Brief) discover.Context {
	hint := discover.Context{
		ProjectName: a.deps.Naming.ProjectName(b.ProjectID),
	}
	if b.Scope == discover.ScopeBranch {
		hint.BranchName = a.deps.Naming.BranchName(b.BranchID)
		if p, err := a.deps.Store.ForProject(b.ProjectID); err == nil {
			if purpose, ok := p.AnswerFor(discover.KeyPurpose); ok {
				hint.ProjectHint = purpose.Text
			}
		}
	}
	return hint
}

type materialiseRequest struct {
	WorktreeID string `json:"worktreeId"`
}

func (a *API) materialise(w http.ResponseWriter, r *http.Request) {
	b, ok := a.load(w, r)
	if !ok {
		return
	}
	var req materialiseRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.WorktreeID == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("worktreeId is required"))
		return
	}
	res, err := a.deps.Writer.Materialise(r.Context(), req.WorktreeID, a.resolve(b))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type analyseRequest struct {
	RepoPath  string `json:"repoPath"`
	ProjectID string `json:"projectId"`
}

type analyseResponse struct {
	Findings repoanalysis.Findings `json:"findings"`
	Summary  string                `json:"summary"`
	View     briefView             `json:"view"`
	// Drafted is false when no model was configured or the draft failed. The
	// import dialog says so rather than pretending the blank brief is a draft.
	Drafted     bool   `json:"drafted"`
	DraftedNote string `json:"draftedNote,omitempty"`
}

func (a *API) analyse(w http.ResponseWriter, r *http.Request) {
	var req analyseRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.RepoPath == "" || req.ProjectID == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("repoPath and projectId are required"))
		return
	}
	findings, err := repoanalysis.Scan(req.RepoPath)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	b := discover.NewBrief(a.deps.NewID(), discover.ScopeProject, req.ProjectID, "", a.deps.Now())
	resp := analyseResponse{Findings: findings, Summary: findings.Summary()}

	if a.deps.Model == nil {
		resp.DraftedNote = "No model is configured, so the questions are unanswered. Set PANACEA_APP_MODEL_API_KEY to have Discover draft a brief from the scan."
	} else if err := repoanalysis.DraftBrief(r.Context(), a.deps.Model, findings, b, a.deps.Now()); err != nil {
		resp.DraftedNote = "The scan succeeded but drafting a brief failed: " + err.Error()
	} else {
		resp.Drafted = true
		resp.DraftedNote = "Drafted from the scan. Correct anything that is wrong before you start."
	}

	if err := a.deps.Store.Save(b); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	resp.View = a.view(b)
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) provenanceFor(w http.ResponseWriter, r *http.Request) {
	branchID := r.PathValue("branchID")
	b, err := a.deps.Store.ForBranch(branchID)
	if errors.Is(err, discoverstore.ErrNotFound) {
		// Not an error: the branch simply has no brief yet, and every surface
		// renders that state rather than failing.
		writeJSON(w, http.StatusOK, provenance.Ref{BranchID: branchID, Criteria: []provenance.Criterion{}})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK,
		provenance.Build(b.ProjectID, branchID, a.resolve(b), a.deps.ContextOf(branchID)))
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./control-plane/internal/discoverapi/... -v -race -count=1`
Expected: PASS — 10 tests.

- [ ] **Step 7: Serve it from the control plane**

Replace `control-plane/cmd/api/main.go`:

```go
package main

import (
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/contextwrite"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverstore"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "listen address")
	stateDir := flag.String("state", discoverstore.DefaultDir(), "where briefs are stored")
	worktree := flag.String("worktree", "", "local worktree directory registered as worktree id 'local'")
	flag.Parse()

	store, err := discoverstore.NewFileStore(*stateDir)
	if err != nil {
		slog.Error("state directory unusable", "err", err)
		os.Exit(1)
	}

	// Phase 3 replaces StaticResolver with the real worktree registry.
	resolver := contextwrite.StaticResolver{}
	if *worktree != "" {
		resolver["local"] = *worktree
	}

	// A missing key is not fatal: Discover works without a model, with static
	// options and no drafted brief.
	var model modelclient.Completer
	if cfg, cfgErr := modelclient.ConfigFromEnv(); cfgErr == nil {
		client, clientErr := modelclient.New(cfg)
		if clientErr != nil {
			slog.Error("model client not built", "err", clientErr)
		} else {
			model = client
			slog.Info("model configured", "baseURL", cfg.BaseURL, "model", cfg.Model)
		}
	} else if errors.Is(cfgErr, modelclient.ErrNoCredentials) {
		slog.Warn("no model configured; Discover will use static options and cannot draft a brief",
			"set", "PANACEA_APP_MODEL_API_KEY")
	}

	api := discoverapi.New(discoverapi.Deps{
		Store:  store,
		Writer: contextwrite.NewWriter(resolver),
		Model:  model,
	})

	mux := http.NewServeMux()
	mux.Handle("/api/v1/discover/", api)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	slog.Info("panacea-app control plane listening", "addr", *addr, "state", *stateDir)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		slog.Error("server exited", "err", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 8: Verify the whole Go workspace still builds and passes**

```bash
go build ./sandboxd/... ./control-plane/...
go vet ./sandboxd/... ./control-plane/...
go test ./sandboxd/... ./control-plane/... -race -count=1
```
Expected: all three exit 0. (`./...` at this root is invalid — it resolves to no modules and errors.)

- [ ] **Step 9: Smoke the endpoint by hand**

PowerShell, two windows:

```powershell
go run ./control-plane/cmd/api -addr 127.0.0.1:8090 -state $env:TEMP\pa-discover
```

```powershell
$body = '{"scope":"branch","projectId":"p-1","branchId":"br-1"}'
Invoke-RestMethod -Method Post -Uri http://127.0.0.1:8090/api/v1/discover/briefs `
  -ContentType application/json -Body $body | ConvertTo-Json -Depth 5
```
Expected: a JSON object whose `step.kind` is `question`, `step.ordinal` is `1` and `step.total` is `5`.

- [ ] **Step 10: Commit**

```bash
git add control-plane/internal/discoverapi control-plane/cmd/api
git commit -m "feat(discoverapi): http surface for briefs, materialise and provenance

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: Desktop test harness, tokens and the API client

**Files:**
- Modify: `desktop/package.json`, `desktop/tsconfig.json`
- Create: `desktop/vitest.config.ts`, `desktop/src/styles/tokens.css`, `desktop/src/lib/discoverApi.ts`
- Test: `desktop/src/lib/discoverApi.test.ts`

**Interfaces:**
- Consumes: the JSON shapes produced by Task 9
- Produces:
  - `desktop/src/lib/discoverApi.ts` exporting `Scope`, `Key`, `Source`, `Status`, `StepKind`, `CriterionState`, `Question`, `Answer`, `Brief`, `Step`, `BriefView`, `Criterion`, `ProvenanceRef`, `MaterialiseResult`, `Findings`, `AnalyseResult`, the `DiscoverApi` interface, and `HttpDiscoverApi`
  - `npm run test` in `desktop/`

**Version note.** Every version below was read from the registry on 2026-09-17, not recalled. `vitest@5.0.1` declares `vite` peers `^6.4.0 || ^7.0.0 || ^8.0.0` (the repo is on Vite 8) and `engines.node` `^22.12.0 || ^24.0.0 || >=26.0.0`. `@testing-library/react@16.3.3` peers React `^18 || ^19` and requires `@testing-library/dom ^10`, which must be installed explicitly.

- [ ] **Step 1: Install the test tooling**

```powershell
cd desktop
npm install --save-dev vitest@5.0.1 jsdom@30.1.0 `
  @testing-library/react@16.3.3 @testing-library/dom@10.4.2 @testing-library/user-event@14.6.7
```
Expected: `added N packages`, no peer-dependency errors.

- [ ] **Step 2: Add the test script**

In `desktop/package.json`, add to `"scripts"`:

```json
    "test": "vitest run",
    "test:watch": "vitest"
```

- [ ] **Step 3: Configure Vitest**

Create `desktop/vitest.config.ts`:

```ts
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    // No globals: every test imports describe/it/expect explicitly, so
    // tsconfig needs no extra "types" entry and tsc stays clean.
    globals: false,
    restoreMocks: true,
  },
});
```

- [ ] **Step 4: Add the token layer**

**Check first:** `ls desktop/src/styles/tokens.css`. If Phase 2 has already shipped it, **skip this step** and instead confirm every variable named below exists in it. Discover must never define a second token layer.

Create `desktop/src/styles/tokens.css`:

```css
/* Panacea App token layer — docs/DESIGN-SYSTEM.md.
   Warm graphite base, deep teal accent used only for interactive and current
   state, status as a separate vocabulary. Values match prototype/app.html,
   which is the approved direction. Phase 2 may re-derive these in OKLCH; when
   it does, keep the variable NAMES so no component changes. */
:root {
  --page: #0a0908;
  --canvas: #0e0c0b;
  --surface: #141210;
  --inset: #1a1715;
  --hover: #1f1c19;

  --ink: #f2ede7;
  --ink-2: #a79f97;
  --ink-3: #6c645d;
  --ink-4: #453f39;

  --line-soft: rgba(255, 248, 240, 0.04);
  --line: rgba(255, 248, 240, 0.075);
  --line-strong: rgba(255, 248, 240, 0.15);

  --accent: #2fb3bd;
  --accent-hi: #74d9e0;
  --accent-tint: rgba(47, 179, 189, 0.11);

  --run: #43b95e;
  --need: #e2a33c;
  --fail: #ef5f45;
  --drift: #c98ae0;

  --r-chip: 6px;
  --r-ctl: 8px;
  --r-card: 10px;
  --r-window: 14px;

  /* Elevation is a 1px ring PLUS a layered shadow, never a flat border. */
  --e-hair: 0 0 0 1px var(--line);
  --e-card: 0 0 0 1px var(--line), 0 1px 2px rgba(0, 0, 0, 0.4), 0 4px 12px rgba(0, 0, 0, 0.22);

  --ease: cubic-bezier(0.23, 1, 0.32, 1);

  --mono: "Cascadia Mono", ui-monospace, "SF Mono", Menlo, Consolas, monospace;
  --sans: "Segoe UI Variable Text", "Segoe UI", -apple-system, system-ui, sans-serif;
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

- [ ] **Step 5: Write the failing API-client test**

Create `desktop/src/lib/discoverApi.test.ts`:

```ts
import { describe, it, expect, vi, afterEach } from "vitest";
import { HttpDiscoverApi } from "./discoverApi";
import type { BriefView, ProvenanceRef } from "./discoverApi";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

const emptyView: BriefView = {
  brief: {
    id: "b-1",
    scope: "branch",
    projectId: "p-1",
    branchId: "br-1",
    questions: [],
    answers: [],
    status: "open",
    revision: 0,
    createdAt: "2026-09-18T09:00:00Z",
    updatedAt: "2026-09-18T09:00:00Z",
  },
  step: { kind: "question", ordinal: 1, total: 5, prompt: "What is this branch for?", question: {
    key: "purpose", projectPrompt: "", branchPrompt: "What is this branch for?",
    help: "", options: [], followUp: false } },
};

afterEach(() => vi.unstubAllGlobals());

describe("HttpDiscoverApi", () => {
  it("creates a branch brief at the documented path", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(emptyView, 201));
    vi.stubGlobal("fetch", fetchMock);

    const api = new HttpDiscoverApi("http://localhost:8090");
    const view = await api.createBrief({ scope: "branch", projectId: "p-1", branchId: "br-1" });

    expect(view.step.total).toBe(5);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("http://localhost:8090/api/v1/discover/briefs");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ scope: "branch", projectId: "p-1", branchId: "br-1" });
  });

  it("records an answer with its source", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(emptyView));
    vi.stubGlobal("fetch", fetchMock);

    const api = new HttpDiscoverApi("http://localhost:8090");
    await api.answer("b-1", { key: "purpose", text: "Add upload", source: "picked" });

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("http://localhost:8090/api/v1/discover/briefs/b-1/answers");
    expect(JSON.parse(init.body)).toEqual({ key: "purpose", text: "Add upload", source: "picked" });
  });

  it("edits an answer with PATCH on the key", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(emptyView));
    vi.stubGlobal("fetch", fetchMock);

    const api = new HttpDiscoverApi("http://localhost:8090");
    await api.editAnswer("b-1", "audience", "Client reviewers");

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("http://localhost:8090/api/v1/discover/briefs/b-1/answers/audience");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body)).toEqual({ text: "Client reviewers" });
  });

  it("reads provenance for a branch", async () => {
    const ref: ProvenanceRef = {
      briefId: "b-1", revision: 4, projectId: "p-1", branchId: "br-1",
      headline: "Add evidence upload.", criteria: [
        { id: "b-1:done:0", text: "Tests pass", state: "open" },
      ], present: true,
    };
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(ref));
    vi.stubGlobal("fetch", fetchMock);

    const api = new HttpDiscoverApi("http://localhost:8090");
    const got = await api.provenance("br-1");

    expect(fetchMock.mock.calls[0][0])
      .toBe("http://localhost:8090/api/v1/discover/branches/br-1/provenance");
    expect(got.criteria[0].state).toBe("open");
  });

  it("surfaces the server's error message", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
      jsonResponse({ error: "discover: that question has not been answered yet" }, 400)));

    const api = new HttpDiscoverApi("http://localhost:8090");
    await expect(api.editAnswer("b-1", "done", "x"))
      .rejects.toThrow("discover: that question has not been answered yet");
  });

  it("trims a trailing slash from the base url", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(emptyView));
    vi.stubGlobal("fetch", fetchMock);

    await new HttpDiscoverApi("http://localhost:8090/").brief("b-1");
    expect(fetchMock.mock.calls[0][0]).toBe("http://localhost:8090/api/v1/discover/briefs/b-1");
  });
});
```

- [ ] **Step 6: Run the test to verify it fails**

Run: `cd desktop && npm run test`
Expected: FAIL — `Failed to resolve import "./discoverApi"`.

- [ ] **Step 7: Write the client**

Create `desktop/src/lib/discoverApi.ts`:

```ts
// Mirror of control-plane/internal/discoverapi. Every field name below matches
// a Go json tag exactly; changing one side without the other is the defect the
// Go package comment warns about.

export type Scope = "project" | "branch";
export type Key = "purpose" | "audience" | "constraints" | "done" | "out_of_scope" | string;
export type Source = "picked" | "typed" | "drafted";
export type Status = "open" | "complete" | "skipped";
export type StepKind = "question" | "followUp" | "complete";
export type CriterionState = "settled" | "open" | "contested";

export interface Question {
  key: Key;
  projectPrompt: string;
  branchPrompt: string;
  help: string;
  options: string[];
  followUp: boolean;
}

export interface Answer {
  key: Key;
  prompt: string;
  text: string;
  source: Source;
  answeredAt: string;
}

export interface Brief {
  id: string;
  scope: Scope;
  projectId: string;
  branchId?: string;
  questions: Question[];
  answers: Answer[];
  status: Status;
  revision: number;
  createdAt: string;
  updatedAt: string;
}

/**
 * Step.total is a denominator over the CLOSED base question set and is zero for
 * follow-ups and for the complete state. The only permitted rendering is the
 * literal "QUESTION n OF m". No bar, no percentage, no ETA
 * (docs/AGENTIC-UX.md anti-pattern 2).
 */
export interface Step {
  kind: StepKind;
  ordinal: number;
  total: number;
  question: Question;
  prompt: string;
}

export interface BriefView {
  brief: Brief;
  step: Step;
}

export interface Criterion {
  id: string;
  text: string;
  state: CriterionState;
  because?: string;
}

export interface ProvenanceRef {
  briefId: string;
  revision: number;
  projectId: string;
  branchId: string;
  headline: string;
  audience?: string;
  constraints?: string;
  outOfScope?: string;
  criteria: Criterion[];
  contextPath?: string;
  present: boolean;
}

export interface MaterialiseResult {
  path: string;
  bytes: number;
  created: boolean;
}

export interface LanguageCount {
  language: string;
  files: number;
}

export interface Findings {
  root: string;
  filesSeen: number;
  truncated: boolean;
  languages: LanguageCount[];
  manifests: string[];
  topLevel: string[];
  testFiles: number;
  hasClaudeMd: boolean;
}

export interface AnalyseResult {
  findings: Findings;
  summary: string;
  view: BriefView;
  drafted: boolean;
  draftedNote?: string;
}

export interface CreateBriefRequest {
  scope: Scope;
  projectId: string;
  branchId?: string;
}

export interface AnswerRequest {
  key: Key;
  text: string;
  source: Source;
}

export interface DiscoverApi {
  createBrief(req: CreateBriefRequest): Promise<BriefView>;
  brief(briefId: string): Promise<BriefView>;
  branchBrief(branchId: string): Promise<BriefView>;
  answer(briefId: string, req: AnswerRequest): Promise<BriefView>;
  editAnswer(briefId: string, key: Key, text: string): Promise<BriefView>;
  skip(briefId: string): Promise<BriefView>;
  materialise(briefId: string, worktreeId: string): Promise<MaterialiseResult>;
  analyse(repoPath: string, projectId: string): Promise<AnalyseResult>;
  provenance(branchId: string): Promise<ProvenanceRef>;
}

export class HttpDiscoverApi implements DiscoverApi {
  private readonly base: string;

  constructor(baseUrl: string) {
    this.base = baseUrl.replace(/\/+$/, "");
  }

  private async call<T>(path: string, method: string, body?: unknown): Promise<T> {
    const init: RequestInit = { method, headers: { "content-type": "application/json" } };
    if (body !== undefined) init.body = JSON.stringify(body);

    const res = await fetch(`${this.base}/api/v1/discover${path}`, init);
    const text = await res.text();
    let parsed: unknown = undefined;
    if (text.length > 0) {
      try {
        parsed = JSON.parse(text);
      } catch {
        throw new Error(`discover: ${res.status} returned non-JSON: ${text.slice(0, 200)}`);
      }
    }
    if (!res.ok) {
      const message =
        parsed && typeof parsed === "object" && "error" in parsed
          ? String((parsed as { error: unknown }).error)
          : `discover: request failed with ${res.status}`;
      throw new Error(message);
    }
    return parsed as T;
  }

  createBrief(req: CreateBriefRequest): Promise<BriefView> {
    return this.call<BriefView>("/briefs", "POST", req);
  }

  brief(briefId: string): Promise<BriefView> {
    return this.call<BriefView>(`/briefs/${encodeURIComponent(briefId)}`, "GET");
  }

  branchBrief(branchId: string): Promise<BriefView> {
    return this.call<BriefView>(`/branches/${encodeURIComponent(branchId)}/brief`, "GET");
  }

  answer(briefId: string, req: AnswerRequest): Promise<BriefView> {
    return this.call<BriefView>(`/briefs/${encodeURIComponent(briefId)}/answers`, "POST", req);
  }

  editAnswer(briefId: string, key: Key, text: string): Promise<BriefView> {
    return this.call<BriefView>(
      `/briefs/${encodeURIComponent(briefId)}/answers/${encodeURIComponent(key)}`,
      "PATCH",
      { text },
    );
  }

  skip(briefId: string): Promise<BriefView> {
    return this.call<BriefView>(`/briefs/${encodeURIComponent(briefId)}/skip`, "POST", {});
  }

  materialise(briefId: string, worktreeId: string): Promise<MaterialiseResult> {
    return this.call<MaterialiseResult>(
      `/briefs/${encodeURIComponent(briefId)}/materialise`,
      "POST",
      { worktreeId },
    );
  }

  analyse(repoPath: string, projectId: string): Promise<AnalyseResult> {
    return this.call<AnalyseResult>("/analyse", "POST", { repoPath, projectId });
  }

  provenance(branchId: string): Promise<ProvenanceRef> {
    return this.call<ProvenanceRef>(`/branches/${encodeURIComponent(branchId)}/provenance`, "GET");
  }
}
```

- [ ] **Step 8: Run the test to verify it passes**

Run: `cd desktop && npm run test`
Expected: PASS — 6 tests in `src/lib/discoverApi.test.ts`.

- [ ] **Step 9: Verify TypeScript is still clean**

Run: `cd desktop && npx tsc --noEmit`
Expected: exits 0. (`noUnusedLocals` and `noUnusedParameters` are on; unused imports are errors here, not warnings.)

- [ ] **Step 10: Commit**

```bash
git add desktop/package.json desktop/package-lock.json desktop/vitest.config.ts desktop/src/styles desktop/src/lib/discoverApi.ts desktop/src/lib/discoverApi.test.ts
git commit -m "feat(desktop): vitest harness, token layer and the discover api client

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: The Discover surface

`prototype/app.html` lines 964–987 are the specification. One question at a time, numbered pickable options, a free-text escape always on screen, a SETTLED list below with per-answer `edit`, a Skip in the top bar, and a completion state that names the file that was written.

**Files:**
- Create: `desktop/src/discover/DiscoverView.tsx`, `desktop/src/discover/discover.css`
- Test: `desktop/src/discover/DiscoverView.test.tsx`

**Interfaces:**
- Consumes: `DiscoverApi`, `BriefView`, `Answer`, `Key`, `Source` from `desktop/src/lib/discoverApi.ts`
- Produces: `DiscoverView({ api, view, branchName, worktreeId, onChange })` — a controlled component. The parent (Phase 2's branch shell) owns the `BriefView` and receives every update through `onChange`, so the cockpit can render the context chip from the same data without a second fetch.

- [ ] **Step 1: Write the failing tests**

Create `desktop/src/discover/DiscoverView.test.tsx`:

```tsx
import { useState } from "react";
import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DiscoverView } from "./DiscoverView";
import type { Answer, BriefView, DiscoverApi, Key, Question } from "../lib/discoverApi";

/** Holds the view the way Phase 2's branch shell will: state up, changes down. */
function Harness({ api, initial, worktreeId }: { api: DiscoverApi; initial: BriefView; worktreeId?: string }) {
  const [current, setCurrent] = useState(initial);
  return (
    <DiscoverView api={api} view={current} branchName="evidence-upload"
      worktreeId={worktreeId} onChange={setCurrent} />
  );
}

const BASE_KEYS: Key[] = ["purpose", "audience", "constraints", "done", "out_of_scope"];

function question(key: Key, prompt: string, options: string[], followUp = false): Question {
  return { key, projectPrompt: prompt, branchPrompt: prompt, help: "Why we ask.", options, followUp };
}

function answer(key: Key): Answer {
  return {
    key,
    prompt: `Prompt for ${key}`,
    text: `answer for ${key}`,
    source: "typed",
    answeredAt: "2026-09-18T09:00:00Z",
  };
}

function view(overrides: Partial<BriefView> = {}): BriefView {
  const questions = BASE_KEYS.map((k) => question(k, `Prompt for ${k}`, ["One", "Two", "Three"]));
  return {
    brief: {
      id: "b-1",
      scope: "branch",
      projectId: "p-1",
      branchId: "br-1",
      questions,
      answers: [],
      status: "open",
      revision: 0,
      createdAt: "2026-09-18T09:00:00Z",
      updatedAt: "2026-09-18T09:00:00Z",
    },
    step: {
      kind: "question",
      ordinal: 1,
      total: 5,
      prompt: "Prompt for purpose",
      question: questions[0],
    },
    ...overrides,
  };
}

function stubApi(overrides: Partial<DiscoverApi> = {}): DiscoverApi {
  const notCalled = () => {
    throw new Error("unexpected API call");
  };
  return {
    createBrief: notCalled,
    brief: notCalled,
    branchBrief: notCalled,
    answer: notCalled,
    editAnswer: notCalled,
    skip: notCalled,
    materialise: notCalled,
    analyse: notCalled,
    provenance: notCalled,
    ...overrides,
  } as DiscoverApi;
}

describe("DiscoverView", () => {
  it("asks one question at a time with an honest count and no progress bar", () => {
    render(<DiscoverView api={stubApi()} view={view()} branchName="evidence-upload" onChange={() => {}} />);

    expect(screen.getByText("QUESTION 1 OF 5")).toBeDefined();
    expect(screen.getByRole("heading", { name: "Prompt for purpose" })).toBeDefined();
    expect(screen.queryByRole("progressbar")).toBeNull();
    expect(document.body.textContent).not.toMatch(/\d+\s*%/);
  });

  it("offers pickable options and a free-text escape together", () => {
    render(<DiscoverView api={stubApi()} view={view()} branchName="b" onChange={() => {}} />);

    expect(screen.getByRole("button", { name: /One/ })).toBeDefined();
    expect(screen.getByRole("button", { name: /Two/ })).toBeDefined();
    expect(screen.getByRole("textbox", { name: /say it your own way/i })).toBeDefined();
  });

  it("records a picked option as source picked", async () => {
    const next = view({ step: { kind: "complete", ordinal: 0, total: 0, prompt: "", question: view().brief.questions[0] } });
    const answerFn = vi.fn().mockResolvedValue(next);
    const onChange = vi.fn();

    render(<DiscoverView api={stubApi({ answer: answerFn })} view={view()} branchName="b" onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: /Two/ }));

    await waitFor(() => expect(answerFn).toHaveBeenCalledWith("b-1", {
      key: "purpose", text: "Two", source: "picked",
    }));
    expect(onChange).toHaveBeenCalledWith(next);
  });

  it("records typed text as source typed", async () => {
    const answerFn = vi.fn().mockResolvedValue(view());
    render(<DiscoverView api={stubApi({ answer: answerFn })} view={view()} branchName="b" onChange={() => {}} />);

    await userEvent.type(screen.getByRole("textbox", { name: /say it your own way/i }), "Unblock upload");
    await userEvent.click(screen.getByRole("button", { name: /^Answer$/ }));

    await waitFor(() => expect(answerFn).toHaveBeenCalledWith("b-1", {
      key: "purpose", text: "Unblock upload", source: "typed",
    }));
  });

  // The UI half of docs/FUNCTIONAL-SPEC.md known defect 7.
  it("edits one settled answer and leaves the others on screen", async () => {
    const settled = view({
      step: { kind: "complete", ordinal: 0, total: 0, prompt: "", question: view().brief.questions[0] },
    });
    settled.brief.answers = BASE_KEYS.map(answer);
    settled.brief.status = "complete";

    const editFn = vi.fn().mockImplementation(async (_id: string, key: Key, text: string) => {
      const updated = structuredClone(settled);
      const target = updated.brief.answers.find((a) => a.key === key);
      if (target) target.text = text;
      return updated;
    });

    render(<Harness api={stubApi({ editAnswer: editFn })} initial={settled} />);

    expect(screen.getAllByTestId("settled-answer")).toHaveLength(5);

    await userEvent.click(screen.getByRole("button", { name: "Edit answer for audience" }));
    const field = screen.getByRole("textbox", { name: /edit your answer/i });
    await userEvent.clear(field);
    await userEvent.type(field, "Client reviewers only");
    await userEvent.click(screen.getByRole("button", { name: /^Save$/ }));

    await waitFor(() => expect(editFn).toHaveBeenCalledWith("b-1", "audience", "Client reviewers only"));
    await waitFor(() => expect(screen.getAllByTestId("settled-answer")).toHaveLength(5));
    expect(screen.getByText("answer for out_of_scope")).toBeDefined();
  });

  it("shows a follow-up without a denominator", () => {
    const followUp = question("rollout", "How does this reach users?", ["A", "B", "C"], true);
    const v = view({ step: { kind: "followUp", ordinal: 0, total: 0, prompt: followUp.branchPrompt, question: followUp } });

    render(<DiscoverView api={stubApi()} view={v} branchName="b" onChange={() => {}} />);

    expect(screen.getByText("FOLLOW-UP")).toBeDefined();
    expect(screen.queryByText(/ OF /)).toBeNull();
  });

  it("names the file that carries the context when the brief is settled", () => {
    const v = view({ step: { kind: "complete", ordinal: 0, total: 0, prompt: "", question: view().brief.questions[0] } });
    v.brief.status = "complete";
    v.brief.answers = [answer("purpose")];

    render(<Harness api={stubApi()} initial={v} />);

    // CLAUDE.md and the branch name each appear in the header and in the body,
    // so assert on presence rather than uniqueness.
    expect(screen.getAllByText(/CLAUDE\.md/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/evidence-upload/).length).toBeGreaterThan(0);
  });

  it("writes CLAUDE.md into the worktree on request and reports the path", async () => {
    const materialise = vi.fn().mockResolvedValue({
      path: "C:\\work\\wt\\CLAUDE.md", bytes: 512, created: true,
    });
    const v = view({ step: { kind: "complete", ordinal: 0, total: 0, prompt: "", question: view().brief.questions[0] } });
    v.brief.status = "complete";
    v.brief.answers = [answer("purpose")];

    render(<Harness api={stubApi({ materialise })} initial={v} worktreeId="wt-1" />);
    await userEvent.click(screen.getByRole("button", { name: /Write CLAUDE\.md/ }));

    await waitFor(() => expect(materialise).toHaveBeenCalledWith("b-1", "wt-1"));
    await waitFor(() => expect(screen.getByText(/C:\\work\\wt\\CLAUDE\.md/)).toBeDefined());
  });

  it("shows the server's message when an answer is rejected", async () => {
    const answerFn = vi.fn().mockRejectedValue(new Error("discover: an answer cannot be empty"));
    render(<DiscoverView api={stubApi({ answer: answerFn })} view={view()} branchName="b" onChange={() => {}} />);

    await userEvent.click(screen.getByRole("button", { name: /One/ }));
    await waitFor(() => expect(screen.getByRole("alert").textContent)
      .toContain("discover: an answer cannot be empty"));
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd desktop && npm run test`
Expected: FAIL — `Failed to resolve import "./DiscoverView"`.

- [ ] **Step 3: Write the component**

Create `desktop/src/discover/DiscoverView.tsx`:

```tsx
import { useState } from "react";
import type { Answer, BriefView, DiscoverApi, Key, MaterialiseResult } from "../lib/discoverApi";
import "./discover.css";

export interface DiscoverViewProps {
  api: DiscoverApi;
  /** Owned by the parent so Develop can render the context chip from the same data. */
  view: BriefView;
  branchName: string;
  /** Supplied once Phase 3 has a worktree; without it the write button is hidden. */
  worktreeId?: string;
  onChange: (next: BriefView) => void;
}

/**
 * The step label.
 *
 * "QUESTION 2 OF 5" is legal because the base question set is closed. A
 * follow-up has no defensible denominator, so it is labelled without one. There
 * is no bar, no percentage and no ETA here, and there must never be
 * (docs/AGENTIC-UX.md anti-pattern 2).
 */
function stepLabel(kind: string, ordinal: number, total: number): string {
  if (kind === "followUp") return "FOLLOW-UP";
  if (kind === "complete") return "SETTLED";
  return `QUESTION ${ordinal} OF ${total}`;
}

export function DiscoverView({ api, view, branchName, worktreeId, onChange }: DiscoverViewProps) {
  const [draft, setDraft] = useState("");
  const [editingKey, setEditingKey] = useState<Key | null>(null);
  const [editDraft, setEditDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [written, setWritten] = useState<MaterialiseResult | null>(null);

  const { brief, step } = view;

  async function run(work: () => Promise<BriefView>) {
    setBusy(true);
    setError(null);
    try {
      onChange(await work());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  function pick(text: string) {
    void run(() => api.answer(brief.id, { key: step.question.key, text, source: "picked" }));
  }

  function submitTyped() {
    const text = draft.trim();
    if (text === "") return;
    setDraft("");
    void run(() => api.answer(brief.id, { key: step.question.key, text, source: "typed" }));
  }

  function beginEdit(a: Answer) {
    setEditingKey(a.key);
    setEditDraft(a.text);
    setError(null);
  }

  async function saveEdit() {
    const key = editingKey;
    if (key === null) return;
    const text = editDraft.trim();
    if (text === "") return;
    setEditingKey(null);
    await run(() => api.editAnswer(brief.id, key, text));
  }

  async function writeContext() {
    if (!worktreeId) return;
    setBusy(true);
    setError(null);
    try {
      setWritten(await api.materialise(brief.id, worktreeId));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  const settled = brief.answers.length > 0 && (
    <section className="dsc-settled">
      <h3 className="dsc-sech">SETTLED</h3>
      {brief.answers.map((a) => (
        <div className="dsc-answer" data-testid="settled-answer" key={a.key}>
          <span className="dsc-answer-q">{a.prompt}</span>
          {editingKey === a.key ? (
            <span className="dsc-answer-edit">
              <textarea
                aria-label="Edit your answer"
                className="dsc-field"
                rows={2}
                value={editDraft}
                onChange={(e) => setEditDraft(e.target.value)}
              />
              <button className="dsc-btn dsc-btn-accent" onClick={() => void saveEdit()} disabled={busy}>
                Save
              </button>
              <button className="dsc-btn" onClick={() => setEditingKey(null)}>Cancel</button>
            </span>
          ) : (
            <>
              <span className="dsc-answer-a">{a.text}</span>
              {a.source === "drafted" && <span className="dsc-drafted">drafted — check it</span>}
              <button
                className="dsc-edit"
                aria-label={`Edit answer for ${a.key}`}
                onClick={() => beginEdit(a)}
              >
                edit
              </button>
            </>
          )}
        </div>
      ))}
    </section>
  );

  return (
    <div className="dsc">
      <header className="dsc-top">
        <span className="dsc-title">Discover</span>
        <span className="dsc-sub">
          becomes <span className="dsc-mono">CLAUDE.md</span> for every session on {branchName}
        </span>
        <span className="dsc-spacer" />
        {step.kind !== "complete" && (
          <button className="dsc-btn" disabled={busy} onClick={() => void run(() => api.skip(brief.id))}>
            Skip
          </button>
        )}
      </header>

      <div className="dsc-body">
        {error && (
          <p className="dsc-error" role="alert">{error}</p>
        )}

        {step.kind === "complete" ? (
          <section className="dsc-card">
            <p className="dsc-step">{stepLabel(step.kind, step.ordinal, step.total)}</p>
            <h2 className="dsc-h2">This branch has its context.</h2>
            <p className="dsc-help">
              Written to <span className="dsc-mono">CLAUDE.md</span> on {branchName}. Every session
              started here reads it first.
            </p>
            {worktreeId && (
              <button className="dsc-btn dsc-btn-accent" disabled={busy} onClick={() => void writeContext()}>
                Write CLAUDE.md into the worktree
              </button>
            )}
            {written && (
              <p className="dsc-help dsc-mono">
                {written.created ? "Created" : "Updated"} {written.path} ({written.bytes} bytes)
              </p>
            )}
          </section>
        ) : (
          <section className="dsc-card">
            <p className="dsc-step">{stepLabel(step.kind, step.ordinal, step.total)}</p>
            <h2 className="dsc-h2">{step.prompt}</h2>
            <p className="dsc-help">{step.question.help}</p>

            {step.question.options.map((option, i) => (
              <button
                className="dsc-pick"
                key={option}
                disabled={busy}
                onClick={() => pick(option)}
              >
                <span className="dsc-pick-key">{i + 1}</span>
                <span className="dsc-pick-text">{option}</span>
              </button>
            ))}

            <div className="dsc-free">
              <textarea
                aria-label="or say it your own way"
                placeholder="or say it your own way…"
                className="dsc-field"
                rows={2}
                value={draft}
                disabled={busy}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey) {
                    e.preventDefault();
                    submitTyped();
                  }
                }}
              />
              <button className="dsc-btn dsc-btn-accent" disabled={busy} onClick={submitTyped}>
                Answer
              </button>
            </div>
          </section>
        )}

        {settled}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Write the stylesheet**

Create `desktop/src/discover/discover.css`:

```css
/* Discover surface. Every colour is a token; no literal hex below.
   Elevation is a ring plus a shadow; the accent marks interactive state only. */
@import "../styles/tokens.css";

.dsc {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--page);
  color: var(--ink);
  font-family: var(--sans);
}

.dsc-top {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--line);
}
.dsc-title { font-size: 13px; font-weight: 620; }
.dsc-sub { font-size: 11.5px; color: var(--ink-3); }
.dsc-spacer { flex: 1; }

.dsc-body { flex: 1; min-height: 0; overflow: auto; padding: 24px; }
.dsc-card { max-width: 620px; }

.dsc-step {
  font-family: var(--mono);
  font-size: 10px;
  letter-spacing: 0.08em;
  color: var(--ink-3);
  margin: 0 0 9px;
}
.dsc-h2 { font-size: 19px; line-height: 1.3; letter-spacing: -0.02em; font-weight: 650; margin: 0 0 8px; }
.dsc-help { font-size: 12.5px; color: var(--ink-2); line-height: 1.6; margin: 0 0 15px; }
.dsc-mono { font-family: var(--mono); font-size: 11.5px; }

.dsc-error {
  font-size: 12px;
  color: var(--fail);
  background: rgba(239, 95, 69, 0.1);
  border-radius: var(--r-ctl);
  padding: 8px 11px;
  margin: 0 0 14px;
  max-width: 620px;
}

.dsc-pick {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  text-align: left;
  padding: 10px 12px;
  margin-bottom: 7px;
  border: 0;
  border-radius: var(--r-card);
  background: var(--surface);
  box-shadow: var(--e-hair);
  color: var(--ink);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
  transition: box-shadow 160ms var(--ease), background-color 160ms var(--ease);
}
.dsc-pick:hover:not(:disabled) { box-shadow: 0 0 0 1px var(--accent); }
.dsc-pick:focus-visible { outline: none; box-shadow: 0 0 0 2px var(--accent); }
.dsc-pick:disabled { opacity: 0.55; cursor: default; }

.dsc-pick-key {
  width: 17px; height: 17px;
  display: grid; place-items: center;
  border-radius: var(--r-chip);
  background: var(--inset);
  box-shadow: var(--e-hair);
  font-family: var(--mono);
  font-size: 10px;
  color: var(--ink-3);
}
.dsc-pick-text { flex: 1; }

.dsc-free {
  display: flex;
  gap: 8px;
  align-items: flex-end;
  margin-top: 14px;
  background: var(--surface);
  box-shadow: var(--e-hair);
  border-radius: var(--r-card);
  padding: 9px 10px;
}
.dsc-field {
  flex: 1;
  background: none;
  border: 0;
  color: var(--ink);
  resize: none;
  font: inherit;
  font-size: 12.5px;
  outline: none;
}
.dsc-field:focus-visible { outline: none; }

.dsc-btn {
  border: 0;
  border-radius: var(--r-ctl);
  background: var(--inset);
  box-shadow: var(--e-hair);
  color: var(--ink-2);
  font: inherit;
  font-size: 11.5px;
  padding: 6px 11px;
  cursor: pointer;
  transition: background-color 160ms var(--ease), color 160ms var(--ease);
}
.dsc-btn:hover:not(:disabled) { background: var(--hover); color: var(--ink); }
.dsc-btn:focus-visible { outline: none; box-shadow: 0 0 0 2px var(--accent); }
.dsc-btn:disabled { opacity: 0.55; cursor: default; }
.dsc-btn-accent { background: var(--accent); color: #04201f; box-shadow: none; }
.dsc-btn-accent:hover:not(:disabled) { background: var(--accent-hi); color: #04201f; }

.dsc-settled { margin-top: 24px; max-width: 620px; }
.dsc-sech {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.11em;
  color: var(--ink-4);
  margin: 0 0 9px;
}
.dsc-answer {
  display: flex;
  gap: 11px;
  align-items: flex-start;
  padding: 9px 0;
  border-bottom: 1px solid var(--line-soft);
  font-size: 12px;
}
.dsc-answer-q { color: var(--ink-3); width: 210px; flex: 0 0 auto; }
.dsc-answer-a { color: var(--ink); flex: 1; }
.dsc-answer-edit { flex: 1; display: flex; gap: 8px; align-items: flex-end; }
.dsc-drafted {
  font-family: var(--mono);
  font-size: 9.5px;
  color: var(--need);
  background: rgba(226, 163, 60, 0.12);
  border-radius: var(--r-chip);
  padding: 2px 6px;
  white-space: nowrap;
}
.dsc-edit {
  border: 0; background: none; padding: 0;
  color: var(--ink-4);
  font: inherit; font-size: 10.5px;
  cursor: pointer;
}
.dsc-edit:hover { color: var(--accent); }
.dsc-edit:focus-visible { outline: none; color: var(--accent); text-decoration: underline; }
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd desktop && npm run test`
Expected: PASS — 9 tests in `DiscoverView.test.tsx`, plus the 6 API tests.

- [ ] **Step 6: Verify TypeScript**

Run: `cd desktop && npx tsc --noEmit`
Expected: exits 0.

- [ ] **Step 7: Commit**

```bash
git add desktop/src/discover
git commit -m "feat(desktop): discover question flow with edit-in-place

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 12: The provenance thread in Develop, Merge and Deploy

Three components, one `ProvenanceRef`, one endpoint. Phase 2 mounts the chip on each session header; Phase 3 mounts the definition of done above the merge list; Phase 7 mounts the manifest in Deploy. Phase 4 ships all three plus a harness route so the thread is verifiable now.

**Files:**
- Create: `desktop/src/provenance/useProvenance.ts`, `desktop/src/provenance/ContextChip.tsx`, `desktop/src/provenance/DefinitionOfDone.tsx`, `desktop/src/provenance/ShippedManifest.tsx`, `desktop/src/provenance/provenance.css`
- Test: `desktop/src/provenance/provenance.test.tsx`

**Interfaces:**
- Consumes: `DiscoverApi.provenance`, `ProvenanceRef`, `Criterion`
- Produces:
  - `useProvenance(api: DiscoverApi, branchId: string): { ref: ProvenanceRef | null; error: string | null }`
  - `ContextChip({ refValue, onOpenDiscover })` — **mounted by Phase 2** on every session header
  - `DefinitionOfDone({ refValue, onOpenDiscover })` — **mounted by Phase 3** above the merge list
  - `ShippedManifest({ refValue, version })` — **mounted by Phase 7** in Deploy

**CONTRACT for the mounting phases:** each component takes a `ProvenanceRef` and nothing else derived from a brief. No mounting phase may compute its own view of the spec — that is how the thread breaks, and the break is invisible. If a surface needs a field the `Ref` lacks, add it to `provenance.Ref` in Task 8, not locally.

- [ ] **Step 1: Write the failing tests**

Create `desktop/src/provenance/provenance.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ContextChip } from "./ContextChip";
import { DefinitionOfDone } from "./DefinitionOfDone";
import { ShippedManifest } from "./ShippedManifest";
import type { ProvenanceRef } from "../lib/discoverApi";

function ref(overrides: Partial<ProvenanceRef> = {}): ProvenanceRef {
  return {
    briefId: "b-42",
    revision: 6,
    projectId: "p-1",
    branchId: "br-1",
    headline: "Add evidence upload.",
    audience: "Client users",
    constraints: "The published API",
    outOfScope: "Visual design",
    criteria: [
      { id: "b-42:done:0", text: "Tests pass", state: "open" },
      { id: "b-42:done:1", text: "No API break", state: "settled", because: "contract tests green at 09:41" },
      { id: "b-42:done:2", text: "Works on mobile", state: "contested", because: "two worktrees disagree on the viewport fix" },
    ],
    contextPath: "C:\\work\\wt\\CLAUDE.md",
    present: true,
    ...overrides,
  };
}

describe("ContextChip", () => {
  it("shows the branch purpose and opens Discover", async () => {
    const open = vi.fn();
    render(<ContextChip refValue={ref()} onOpenDiscover={open} />);

    const chip = screen.getByRole("button", { name: /Add evidence upload\./ });
    expect(chip).toBeDefined();
    await userEvent.click(chip);
    expect(open).toHaveBeenCalledOnce();
  });

  it("says so when the branch has no brief", () => {
    render(<ContextChip refValue={ref({ present: false, headline: "" })} onOpenDiscover={() => {}} />);
    expect(screen.getByText(/No brief yet/)).toBeDefined();
  });
});

describe("DefinitionOfDone", () => {
  it("renders convergence, never a count", () => {
    render(<DefinitionOfDone refValue={ref()} onOpenDiscover={() => {}} />);

    expect(screen.getByText("Tests pass")).toBeDefined();
    expect(screen.getByText("No API break")).toBeDefined();
    expect(screen.getByText("Works on mobile")).toBeDefined();

    const text = document.body.textContent ?? "";
    expect(text).not.toMatch(/\b\d+\s+of\s+\d+\b/i);
    expect(text).not.toMatch(/\d+\s*%/);
    expect(screen.queryByRole("progressbar")).toBeNull();
  });

  it("never shows a non-open state without its evidence", () => {
    render(<DefinitionOfDone refValue={ref()} onOpenDiscover={() => {}} />);
    expect(screen.getByText(/contract tests green at 09:41/)).toBeDefined();
    expect(screen.getByText(/two worktrees disagree/)).toBeDefined();
  });

  it("points at Discover when there is no definition of done", () => {
    render(<DefinitionOfDone refValue={ref({ criteria: [] })} onOpenDiscover={() => {}} />);
    expect(screen.getByText(/No definition of done/)).toBeDefined();
  });
});

describe("ShippedManifest", () => {
  it("attaches the brief to the release", () => {
    render(<ShippedManifest refValue={ref()} version="v0.4.2" />);

    expect(screen.getByText(/v0\.4\.2/)).toBeDefined();
    expect(screen.getByText(/Add evidence upload\./)).toBeDefined();
    expect(screen.getByText(/brief b-42, revision 6/i)).toBeDefined();
    expect(screen.getByText(/CLAUDE\.md/)).toBeDefined();
  });

  it("is honest when nothing was specified", () => {
    render(<ShippedManifest refValue={ref({ present: false, headline: "", criteria: [] })} version="v0.1.0" />);
    expect(screen.getByText(/shipped without a brief/i)).toBeDefined();
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd desktop && npm run test`
Expected: FAIL — `Failed to resolve import "./ContextChip"`.

- [ ] **Step 3: Write the hook**

Create `desktop/src/provenance/useProvenance.ts`:

```ts
import { useEffect, useState } from "react";
import type { DiscoverApi, ProvenanceRef } from "../lib/discoverApi";

/**
 * One fetch per branch, shared by every provenance surface.
 *
 * The thread only holds if Develop, Merge and Deploy read the SAME ref. Any
 * surface that derives its own view of the brief has broken it, and the break
 * is invisible until the two disagree in front of a client.
 */
export function useProvenance(api: DiscoverApi, branchId: string) {
  const [ref, setRef] = useState<ProvenanceRef | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    setRef(null);
    setError(null);
    api
      .provenance(branchId)
      .then((value) => {
        if (live) setRef(value);
      })
      .catch((e: unknown) => {
        if (live) setError(e instanceof Error ? e.message : String(e));
      });
    return () => {
      live = false;
    };
  }, [api, branchId]);

  return { ref, error };
}
```

- [ ] **Step 4: Write the three surfaces**

Create `desktop/src/provenance/ContextChip.tsx`:

```tsx
import type { ProvenanceRef } from "../lib/discoverApi";
import "./provenance.css";

export interface ContextChipProps {
  refValue: ProvenanceRef;
  onOpenDiscover: () => void;
}

/**
 * Mounted by Phase 2 on every session header. This is the Develop end of the
 * provenance thread: the session shows the spec it is working to, and one click
 * lands on the brief that says it.
 */
export function ContextChip({ refValue, onOpenDiscover }: ContextChipProps) {
  if (!refValue.present) {
    return (
      <button className="pv-chip pv-chip-absent" onClick={onOpenDiscover}>
        <span className="pv-glyph" aria-hidden="true">◇</span>
        No brief yet — run Discover
      </button>
    );
  }
  return (
    <button
      className="pv-chip"
      onClick={onOpenDiscover}
      title={[refValue.audience && `For: ${refValue.audience}`,
              refValue.constraints && `Must not break: ${refValue.constraints}`,
              refValue.outOfScope && `Out of scope: ${refValue.outOfScope}`]
        .filter(Boolean)
        .join("\n")}
    >
      <span className="pv-glyph" aria-hidden="true">◇</span>
      {refValue.headline}
    </button>
  );
}
```

Create `desktop/src/provenance/DefinitionOfDone.tsx`:

```tsx
import type { Criterion, ProvenanceRef } from "../lib/discoverApi";
import "./provenance.css";

export interface DefinitionOfDoneProps {
  refValue: ProvenanceRef;
  onOpenDiscover: () => void;
}

/**
 * Mounted by Phase 3 above the merge list. This is the Merge end of the thread.
 *
 * Convergence, not progress: settled / open / contested, in that fixed order,
 * with no count, no denominator, no percentage and no completion tick.
 * docs/AGENTIC-UX.md deletes "2 of 4 met" specifically, and this component is
 * the place that ban is enforced.
 */
const ORDER: Criterion["state"][] = ["contested", "open", "settled"];

const GLYPH: Record<Criterion["state"], string> = {
  settled: "✓",
  open: "○",
  contested: "≠",
};

export function DefinitionOfDone({ refValue, onOpenDiscover }: DefinitionOfDoneProps) {
  if (refValue.criteria.length === 0) {
    return (
      <section className="pv-dod">
        <h3 className="pv-sech">DEFINITION OF DONE · from Discover</h3>
        <p className="pv-empty">
          No definition of done on this branch.{" "}
          <button className="pv-link" onClick={onOpenDiscover}>Answer it in Discover</button>
        </p>
      </section>
    );
  }

  const ordered = ORDER.flatMap((state) => refValue.criteria.filter((c) => c.state === state));

  return (
    <section className="pv-dod">
      <h3 className="pv-sech">DEFINITION OF DONE · from Discover</h3>
      <ul className="pv-list">
        {ordered.map((c) => (
          <li className={`pv-item pv-${c.state}`} key={c.id}>
            <span className="pv-state-glyph" aria-hidden="true">{GLYPH[c.state]}</span>
            <span className="pv-item-body">
              <span className="pv-item-text">{c.text}</span>
              <span className="pv-item-state">{c.state}</span>
              {/* A non-open state without its evidence is not renderable. */}
              {c.state !== "open" && c.because && (
                <span className="pv-because">{c.because}</span>
              )}
            </span>
          </li>
        ))}
      </ul>
      <p className="pv-source">
        Brief {refValue.briefId}, revision {refValue.revision}.{" "}
        <button className="pv-link" onClick={onOpenDiscover}>Open Discover</button>
      </p>
    </section>
  );
}
```

Create `desktop/src/provenance/ShippedManifest.tsx`:

```tsx
import type { ProvenanceRef } from "../lib/discoverApi";
import "./provenance.css";

export interface ShippedManifestProps {
  refValue: ProvenanceRef;
  version: string;
}

/**
 * Mounted by Phase 7 in Deploy. This is the far end of the provenance thread:
 * the release carries the brief it was written against, so "what shipped" is
 * answered with the spec, not with a commit list.
 */
export function ShippedManifest({ refValue, version }: ShippedManifestProps) {
  if (!refValue.present) {
    return (
      <section className="pv-shipped">
        <h3 className="pv-sech">WHAT SHIPPED</h3>
        <p className="pv-empty">
          <span className="pv-mono">{version}</span> shipped without a brief. Nothing in Discover
          describes what this branch was for.
        </p>
      </section>
    );
  }
  return (
    <section className="pv-shipped">
      <h3 className="pv-sech">WHAT SHIPPED</h3>
      <p className="pv-shipped-line">
        <span className="pv-mono">{version}</span> carries: {refValue.headline}
      </p>
      {refValue.criteria.length > 0 && (
        <ul className="pv-list">
          {refValue.criteria.map((c) => (
            <li className={`pv-item pv-${c.state}`} key={c.id}>
              <span className="pv-item-body">
                <span className="pv-item-text">{c.text}</span>
                <span className="pv-item-state">{c.state}</span>
              </span>
            </li>
          ))}
        </ul>
      )}
      <p className="pv-source">
        From brief {refValue.briefId}, revision {refValue.revision}
        {refValue.contextPath && (
          <>
            {" · agents read "}
            <span className="pv-mono">{refValue.contextPath}</span>
          </>
        )}
      </p>
    </section>
  );
}
```

- [ ] **Step 5: Write the stylesheet**

Create `desktop/src/provenance/provenance.css`:

```css
/* Provenance surfaces. Status colours are the separate status vocabulary from
   docs/DESIGN-SYSTEM.md and never the accent; each state carries a glyph as
   well as a colour so it survives colour blindness. */
@import "../styles/tokens.css";

.pv-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  max-width: 320px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  border: 0;
  border-radius: var(--r-chip);
  background: var(--accent-tint);
  color: var(--accent-hi);
  font-family: var(--mono);
  font-size: 9.5px;
  padding: 3px 7px;
  cursor: pointer;
  transition: background-color 160ms var(--ease);
}
.pv-chip:hover { background: rgba(47, 179, 189, 0.2); }
.pv-chip:focus-visible { outline: none; box-shadow: 0 0 0 2px var(--accent); }
.pv-chip-absent { background: var(--inset); color: var(--ink-3); }
.pv-glyph { font-size: 10px; }

.pv-sech {
  font-size: 9px;
  font-weight: 700;
  letter-spacing: 0.11em;
  color: var(--ink-4);
  margin: 0 0 9px;
  font-family: var(--sans);
}

.pv-dod,
.pv-shipped {
  max-width: 640px;
  font-family: var(--sans);
  color: var(--ink);
}

.pv-list { list-style: none; margin: 0; padding: 0; }
.pv-item {
  display: flex;
  gap: 9px;
  align-items: flex-start;
  padding: 8px 0;
  border-bottom: 1px solid var(--line-soft);
  font-size: 12px;
}
.pv-item-body { display: flex; flex-direction: column; gap: 2px; }
.pv-item-text { color: var(--ink); }
.pv-item-state {
  font-family: var(--mono);
  font-size: 9.5px;
  letter-spacing: 0.06em;
  color: var(--ink-3);
}
.pv-because { font-size: 11px; color: var(--ink-2); line-height: 1.5; }

.pv-state-glyph { width: 12px; font-size: 11px; line-height: 1.5; }
.pv-settled .pv-state-glyph { color: var(--run); }
.pv-open .pv-state-glyph { color: var(--ink-4); }
.pv-contested .pv-state-glyph { color: var(--need); }

.pv-empty { font-size: 12px; color: var(--ink-2); line-height: 1.6; margin: 0; }
.pv-shipped-line { font-size: 12.5px; color: var(--ink-2); line-height: 1.7; margin: 0 0 10px; }
.pv-source { font-size: 11px; color: var(--ink-3); margin: 10px 0 0; }
.pv-mono { font-family: var(--mono); font-size: 11.5px; font-variant-numeric: tabular-nums; }

.pv-link {
  border: 0; background: none; padding: 0;
  color: var(--accent);
  font: inherit;
  cursor: pointer;
  text-decoration: underline;
}
.pv-link:focus-visible { outline: none; box-shadow: 0 0 0 2px var(--accent); border-radius: var(--r-chip); }
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd desktop && npm run test`
Expected: PASS — 7 tests in `provenance.test.tsx`.

- [ ] **Step 7: Commit**

```bash
git add desktop/src/provenance
git commit -m "feat(desktop): provenance thread for develop, merge and deploy

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 13: Importing an existing repo

`prototype/app.html` lines 615–648. Analyse the code, draft a brief, let the user correct it, then offer three places to start: brief only, brief then design, or straight to Develop.

**Files:**
- Create: `desktop/src/discover/ImportRepo.tsx`
- Test: `desktop/src/discover/ImportRepo.test.tsx`

**Interfaces:**
- Consumes: `DiscoverApi.analyse`, `AnalyseResult`, `Findings`, `BriefView`
- Produces:
  - `export type StartMode = "discover" | "design" | "develop"`
  - `ImportRepo({ api, projectId, onBringIn })` where `onBringIn(mode: StartMode, view: BriefView): void`

**SCOPE:** `onBringIn` hands the choice to the caller. Phase 2 decides which surface to open, Phase 5 owns what `"design"` means. Phase 4 must not navigate — it has no router.

- [ ] **Step 1: Write the failing tests**

Create `desktop/src/discover/ImportRepo.test.tsx`:

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ImportRepo } from "./ImportRepo";
import type { AnalyseResult, DiscoverApi } from "../lib/discoverApi";

function analysis(overrides: Partial<AnalyseResult> = {}): AnalyseResult {
  return {
    findings: {
      root: "C:\\work\\engagement-portal",
      filesSeen: 412,
      truncated: false,
      languages: [
        { language: "Go", files: 188 },
        { language: "TypeScript", files: 96 },
      ],
      manifests: ["Dockerfile", "go.mod", "package.json"],
      topLevel: ["services", "web"],
      testFiles: 54,
      hasClaudeMd: false,
    },
    summary: "Files scanned: 412\n",
    drafted: true,
    draftedNote: "Drafted from the scan. Correct anything that is wrong before you start.",
    view: {
      brief: {
        id: "b-9",
        scope: "project",
        projectId: "p-1",
        questions: [],
        answers: [
          {
            key: "purpose",
            prompt: "What is this project for?",
            text: "An engagement portal with a Go API and a React web app.",
            source: "drafted",
            answeredAt: "2026-09-18T09:00:00Z",
          },
        ],
        status: "open",
        revision: 1,
        createdAt: "2026-09-18T09:00:00Z",
        updatedAt: "2026-09-18T09:00:00Z",
      },
      step: {
        kind: "question",
        ordinal: 2,
        total: 5,
        prompt: "Who is this project for?",
        question: {
          key: "audience", projectPrompt: "Who is this project for?",
          branchPrompt: "", help: "", options: [], followUp: false,
        },
      },
    },
    ...overrides,
  };
}

function stubApi(analyse: DiscoverApi["analyse"]): DiscoverApi {
  const notCalled = () => {
    throw new Error("unexpected API call");
  };
  return {
    createBrief: notCalled, brief: notCalled, branchBrief: notCalled,
    answer: notCalled, editAnswer: notCalled, skip: notCalled,
    materialise: notCalled, provenance: notCalled, analyse,
  } as DiscoverApi;
}

describe("ImportRepo", () => {
  it("shows the mechanical findings before any draft", async () => {
    const analyse = vi.fn().mockResolvedValue(analysis());
    render(<ImportRepo api={stubApi(analyse)} projectId="p-1" onBringIn={() => {}} />);

    await userEvent.type(screen.getByRole("textbox", { name: /repository folder/i }), "C:\\work\\portal");
    await userEvent.click(screen.getByRole("button", { name: /^Analyse$/ }));

    await waitFor(() => expect(analyse).toHaveBeenCalledWith("C:\\work\\portal", "p-1"));
    expect(screen.getByText("Go · 188 files")).toBeDefined();
    expect(screen.getByText(/412 files scanned/)).toBeDefined();
    expect(screen.getByText(/go\.mod/)).toBeDefined();
  });

  it("marks every drafted answer as a draft to correct", async () => {
    const analyse = vi.fn().mockResolvedValue(analysis());
    render(<ImportRepo api={stubApi(analyse)} projectId="p-1" onBringIn={() => {}} />);

    await userEvent.type(screen.getByRole("textbox", { name: /repository folder/i }), "C:\\work\\portal");
    await userEvent.click(screen.getByRole("button", { name: /^Analyse$/ }));

    await waitFor(() => expect(screen.getByText(/An engagement portal with a Go API/)).toBeDefined());
    expect(screen.getByText(/Correct anything that is wrong/)).toBeDefined();
    expect(screen.getAllByText("drafted — check it").length).toBe(1);
  });

  it("offers three places to start and reports the choice", async () => {
    const result = analysis();
    const analyse = vi.fn().mockResolvedValue(result);
    const onBringIn = vi.fn();
    render(<ImportRepo api={stubApi(analyse)} projectId="p-1" onBringIn={onBringIn} />);

    await userEvent.type(screen.getByRole("textbox", { name: /repository folder/i }), "C:\\work\\portal");
    await userEvent.click(screen.getByRole("button", { name: /^Analyse$/ }));
    await waitFor(() => expect(screen.getByRole("button", { name: /Confirm the brief first/ })).toBeDefined());

    expect(screen.getByRole("button", { name: /Brief, then design/ })).toBeDefined();
    expect(screen.getByRole("button", { name: /Straight to Develop/ })).toBeDefined();

    await userEvent.click(screen.getByRole("button", { name: /Straight to Develop/ }));
    await userEvent.click(screen.getByRole("button", { name: /^Bring it in$/ }));

    expect(onBringIn).toHaveBeenCalledWith("develop", result.view);
  });

  it("says plainly when no model drafted the brief", async () => {
    const analyse = vi.fn().mockResolvedValue(analysis({
      drafted: false,
      draftedNote: "No model is configured, so the questions are unanswered. Set PANACEA_APP_MODEL_API_KEY to have Discover draft a brief from the scan.",
    }));
    render(<ImportRepo api={stubApi(analyse)} projectId="p-1" onBringIn={() => {}} />);

    await userEvent.type(screen.getByRole("textbox", { name: /repository folder/i }), "C:\\work\\portal");
    await userEvent.click(screen.getByRole("button", { name: /^Analyse$/ }));

    await waitFor(() => expect(screen.getByText(/PANACEA_APP_MODEL_API_KEY/)).toBeDefined());
  });

  it("surfaces a scan failure instead of an empty draft", async () => {
    const analyse = vi.fn().mockRejectedValue(new Error("repoanalysis: root is not a directory"));
    render(<ImportRepo api={stubApi(analyse)} projectId="p-1" onBringIn={() => {}} />);

    await userEvent.type(screen.getByRole("textbox", { name: /repository folder/i }), "C:\\nope");
    await userEvent.click(screen.getByRole("button", { name: /^Analyse$/ }));

    await waitFor(() => expect(screen.getByRole("alert").textContent)
      .toContain("repoanalysis: root is not a directory"));
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd desktop && npm run test`
Expected: FAIL — `Failed to resolve import "./ImportRepo"`.

- [ ] **Step 3: Write the component**

Create `desktop/src/discover/ImportRepo.tsx`:

```tsx
import { useState } from "react";
import type { AnalyseResult, BriefView, DiscoverApi } from "../lib/discoverApi";
import "./discover.css";

export type StartMode = "discover" | "design" | "develop";

export interface ImportRepoProps {
  api: DiscoverApi;
  projectId: string;
  /** Phase 2 decides which surface opens; Phase 4 has no router. */
  onBringIn: (mode: StartMode, view: BriefView) => void;
}

const START_MODES: { mode: StartMode; title: string; detail: string }[] = [
  {
    mode: "discover",
    title: "Confirm the brief first",
    detail: "Finish the questions, then agents inherit the answers.",
  },
  {
    mode: "design",
    title: "Brief, then design",
    detail: "Also draft the current screens so you can change them first.",
  },
  {
    mode: "develop",
    title: "Straight to Develop",
    detail: "Sessions open immediately. Run Discover later.",
  },
];

export function ImportRepo({ api, projectId, onBringIn }: ImportRepoProps) {
  const [path, setPath] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<AnalyseResult | null>(null);
  const [mode, setMode] = useState<StartMode>("discover");

  async function analyse() {
    const trimmed = path.trim();
    if (trimmed === "") return;
    setBusy(true);
    setError(null);
    try {
      setResult(await api.analyse(trimmed, projectId));
    } catch (e) {
      setResult(null);
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="dsc">
      <header className="dsc-top">
        <span className="dsc-title">Bring in a repo</span>
        <span className="dsc-sub">we read the code first, then Discover drafts a brief you correct</span>
      </header>

      <div className="dsc-body">
        {error && <p className="dsc-error" role="alert">{error}</p>}

        <div className="dsc-free">
          <textarea
            aria-label="Repository folder"
            placeholder="C:\work\engagement-portal"
            className="dsc-field"
            rows={1}
            value={path}
            disabled={busy}
            onChange={(e) => setPath(e.target.value)}
          />
          <button className="dsc-btn dsc-btn-accent" disabled={busy} onClick={() => void analyse()}>
            Analyse
          </button>
        </div>

        {result && (
          <>
            <section className="dsc-settled">
              <h3 className="dsc-sech">WHAT WE FOUND</h3>
              <p className="dsc-help">
                {result.findings.filesSeen} files scanned
                {result.findings.truncated && " (stopped at the scan limit)"} ·{" "}
                {result.findings.testFiles} test files
              </p>
              {result.findings.languages.map((l) => (
                <div className="dsc-answer" key={l.language}>
                  <span className="dsc-answer-q">{l.language}</span>
                  <span className="dsc-answer-a">{`${l.language} · ${l.files} files`}</span>
                </div>
              ))}
              <div className="dsc-answer">
                <span className="dsc-answer-q">Manifests</span>
                <span className="dsc-answer-a dsc-mono">
                  {result.findings.manifests.length > 0 ? result.findings.manifests.join(", ") : "none"}
                </span>
              </div>
              <div className="dsc-answer">
                <span className="dsc-answer-q">Existing CLAUDE.md</span>
                <span className="dsc-answer-a">{result.findings.hasClaudeMd ? "yes" : "no"}</span>
              </div>
            </section>

            <section className="dsc-settled">
              <h3 className="dsc-sech">DRAFT BRIEF</h3>
              {result.draftedNote && <p className="dsc-help">{result.draftedNote}</p>}
              {result.view.brief.answers.length === 0 ? (
                <p className="dsc-help">Nothing drafted. The questions are waiting for you in Discover.</p>
              ) : (
                result.view.brief.answers.map((a) => (
                  <div className="dsc-answer" key={a.key}>
                    <span className="dsc-answer-q">{a.prompt}</span>
                    <span className="dsc-answer-a">{a.text}</span>
                    {a.source === "drafted" && <span className="dsc-drafted">drafted — check it</span>}
                  </div>
                ))
              )}
            </section>

            <section className="dsc-settled">
              <h3 className="dsc-sech">WHERE DO YOU WANT TO START</h3>
              {START_MODES.map((m, i) => (
                <button
                  className="dsc-pick"
                  key={m.mode}
                  aria-pressed={mode === m.mode}
                  onClick={() => setMode(m.mode)}
                >
                  <span className="dsc-pick-key">{i + 1}</span>
                  <span className="dsc-pick-text">
                    {m.title}
                    <small className="dsc-pick-detail">{m.detail}</small>
                  </span>
                </button>
              ))}
              <button
                className="dsc-btn dsc-btn-accent"
                onClick={() => onBringIn(mode, result.view)}
              >
                Bring it in
              </button>
            </section>
          </>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Add the one missing style rule**

Append to `desktop/src/discover/discover.css`:

```css
.dsc-pick-detail { display: block; color: var(--ink-3); font-size: 10.5px; margin-top: 2px; }
.dsc-pick[aria-pressed="true"] { box-shadow: 0 0 0 1px var(--accent); background: var(--accent-tint); }
.dsc-pick[aria-pressed="true"] .dsc-pick-key { background: var(--accent); color: #04201f; }
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd desktop && npm run test`
Expected: PASS — 5 tests in `ImportRepo.test.tsx`; 27 tests across the desktop suite.

- [ ] **Step 6: Verify TypeScript and the production build**

```powershell
cd desktop
npx tsc --noEmit
npm run build
```
Expected: both exit 0.

- [ ] **Step 7: Commit**

```bash
git add desktop/src/discover
git commit -m "feat(desktop): import an existing repo into a drafted brief

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

### Task 14: End-to-end verification — a real agent inherits the brief

No new code. This task proves the one claim the phase lives or dies on: **a real, unmodified Claude Code session started in the worktree reads the brief Discover wrote.** A screenshot of the Discover surface is not evidence of that; only the agent answering from the file is.

**Files:**
- Create: `docs/verification/2026-09-18-phase-4-results.md`

- [ ] **Step 1: Make a real worktree**

```powershell
$wt = "$env:TEMP\pa-phase4\evidence-upload"
New-Item -ItemType Directory -Force $wt | Out-Null
git -C $wt init 2>&1 | Out-Null
"hello" | Out-File -Encoding utf8 "$wt\README.md"
```
Expected: the directory exists and contains `README.md`.

- [ ] **Step 2: Start the control plane against it**

```powershell
$env:PANACEA_APP_MODEL_API_KEY = "<your own key, or the gateway key>"
go run ./control-plane/cmd/api -addr 127.0.0.1:8090 -state $env:TEMP\pa-phase4\state -worktree $wt
```
Expected: `panacea-app control plane listening`. If the key is unset you will see the `no model configured` warning instead — that is a valid run, the options are just the static ones.

Record which it was. **Compliance check:** the key must be your own API key or a corporate gateway key. If you are tempted to paste a Claude subscription token here, stop — that is the thing `CLAUDE.md` forbids.

- [ ] **Step 3: Drive the flow from the desktop app**

```powershell
cd desktop
npm run tauri dev
```

Mount `DiscoverView` against `new HttpDiscoverApi("http://127.0.0.1:8090")` with a brief created for `projectId: "p-1"`, `branchId: "br-1"`, and `worktreeId: "local"`.

Answer all five questions. Make the purpose answer something an agent could only know from the file, for example:

> This branch exists to add evidence upload to the engagement portal. The codeword for this branch is ORTHOCLASE.

Then click **Write CLAUDE.md into the worktree**.

Record: the path reported, and whether editing answer 2 afterwards left answers 3, 4 and 5 intact on screen.

- [ ] **Step 4: Confirm the file on disk**

```powershell
Get-Content "$wt\CLAUDE.md"
```
Expected: one `<!-- panacea:discover:begin ... -->` block, the five sections in file order, the codeword present, and the closing `<!-- panacea:discover:end -->`.

- [ ] **Step 5: Confirm a hand-written section survives a rewrite**

```powershell
Add-Content "$wt\CLAUDE.md" "`n## House rules`n`nNever touch infra."
```
Edit any answer in Discover, click **Write CLAUDE.md into the worktree** again, then:

```powershell
Select-String -Path "$wt\CLAUDE.md" -Pattern "Never touch infra","panacea:discover:begin" | Format-Table Line
```
Expected: `Never touch infra.` is still there, and `panacea:discover:begin` appears exactly once.

- [ ] **Step 6: THE ACCEPTANCE TEST — a real agent reads it**

Start a real Claude Code session in that worktree, using the Phase 1 sandbox image:

```powershell
docker run --rm -it -v "${wt}:/home/dev/work" -w /home/dev/work panacea-app/sandbox:dev bash -lc "claude"
```

Authenticate with your own credentials, then ask, without pasting the file:

> What is this branch for, and is there a codeword?

Expected: the agent answers from `CLAUDE.md` and says **ORTHOCLASE**.

**This is the phase's acceptance criterion.** If the agent does not know, the inheritance mechanism does not work and nothing else in Phase 4 matters. Record the exact transcript lines either way — a failure here is information, and hiding it wastes the phase.

- [ ] **Step 7: Confirm the provenance thread**

With the same brief settled, call the provenance endpoint directly:

```powershell
Invoke-RestMethod -Uri http://127.0.0.1:8090/api/v1/discover/branches/br-1/provenance |
  ConvertTo-Json -Depth 5
```
Expected: `present` is `true`, `headline` is the purpose answer, `criteria` is one entry per line of the definition-of-done answer, every `state` is `open`, `contextPath` ends in `CLAUDE.md`, and **no field anywhere in the response is a count, a percentage or a ratio**.

Mount `ContextChip`, `DefinitionOfDone` and `ShippedManifest` against that ref and confirm by eye that all three name the same brief id and revision.

- [ ] **Step 8: Record the results**

Create `docs/verification/2026-09-18-phase-4-results.md` containing: every command run, whether a model was configured, the rendered `CLAUDE.md` in full, the answer the agent gave in Step 6 verbatim, the provenance JSON, and each defect found. State failures plainly.

- [ ] **Step 9: Tear down and commit**

```powershell
Remove-Item -Recurse -Force "$env:TEMP\pa-phase4"
```

```bash
git add docs/verification/2026-09-18-phase-4-results.md
git commit -m "docs: phase 4 discover end-to-end verification results

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

---

## Self-review

**Spec coverage.**

| Requirement | Where it is met |
|---|---|
| One question at a time, pickable answers **plus** a free-text escape | Task 11 — `dsc-pick` buttons and the always-present `dsc-free` textarea; test `offers pickable options and a free-text escape together` |
| Progress shown honestly, no progress bar | Task 2 `Step.Total` (zero for follow-ups and complete) + Task 11 `stepLabel`; tests assert no `progressbar` role and no `%` |
| Per-branch `CLAUDE.md` written into the worktree | Tasks 4 and 5; proved for real in Task 14 Step 6 |
| Inheritance project → branch → session | Task 4 `discover.Resolve`; the session leg is the file itself, which is why Task 14 Step 6 is the acceptance test |
| Editing a settled answer edits in place | Task 2 `Brief.Edit` (append-or-replace, never splice) + Task 9 `PATCH` + Task 11 edit form. Three tests named for `FUNCTIONAL-SPEC` defect 7, one per layer |
| Provenance thread into Develop, Merge, Deploy | Task 8 `provenance.Ref`, one endpoint; Task 12 three components fed only by that `Ref` |
| Importing a repo: analyse → draft → correct → three start modes | Task 6 `Scan` + `DraftBrief`; Task 13 `ImportRepo` with `StartMode` |
| How the app calls a model, compliantly | Task 1. Own key or gateway via `PANACEA_APP_MODEL_*`; pooled subscriptions forbidden in the package comment and re-checked in Task 14 Step 2 |

**Placeholder scan.** No `TBD`, no "add error handling", no "similar to Task N". Every code block is complete and compiles. Every command has an expected output. Every version string (`vitest@5.0.1`, `jsdom@30.1.0`, `@testing-library/react@16.3.3`, `@testing-library/dom@10.4.2`, `@testing-library/user-event@14.6.7`) and every API literal (`/v1/messages`, `anthropic-version: 2023-06-01`, `output_config.format.type = "json_schema"`, `claude-opus-5`) was read from the registry or the API reference on 2026-09-17, not recalled. The `anthropic-sdk-go` exclusion rests on a verified fact: `v1.73.0`'s `go.mod` declares `go 1.24`, above the pinned CI toolchain.

**Type consistency.** `discover.Key` constants are the same five strings in Go and in `discoverApi.ts`. `Step` has identical fields on both sides, `total` included. `provenance.Ref` and `ProvenanceRef` match field for field, `criteria` always an array because `Build` initialises it and the 404 path substitutes an empty one. `Findings` initialises `Languages`, `Manifests` and `TopLevel` to empty slices so the TypeScript arrays are never nullable. `modelclient.Completer` is the only model seam and is consumed identically by `discover.TailorOptions`, `discover.ProposeFollowUp` and `repoanalysis.DraftBrief`.

**Cross-task contracts, stated in both places.**
- `contextwrite.WorktreeResolver` — Task 5 Interfaces, the `contextwrite` package comment, and the seam table at the top. The ordering rule (**materialise before `ptysession.Start`**) is written in all three, because a violation produces no error and no symptom except an agent that quietly knows nothing.
- `discoverstore.Store` — Task 7 Interfaces and the package comment. Phase 2 implements it; nothing else in Phase 4 changes.
- `discoverapi.Naming` — Task 9 Interfaces and the interface's doc comment. Two strings, never an error.
- `ProvenanceRef` — Task 8 and Task 12. No mounting phase may derive its own view of the spec; if a surface needs a new field it goes in `provenance.Ref`, not locally.

**Deliberately out of scope**, each with a home: the cockpit shell, the rail and the sub-rail (Phase 2); git worktree lifecycle and the merge list these components mount into (Phase 3); the design canvas behind `StartMode "design"` (Phase 5); credential rotation and spend attribution behind `modelclient.Completer` (Phase 6); the Deploy surface `ShippedManifest` mounts into (Phase 7); authentication on the control-plane HTTP surface, which binds to `127.0.0.1` in this phase (Phase 9).

**Known risk carried forward.** `Criterion.State` is `Open` for every criterion in this phase — nothing yet computes `Settled` or `Contested`, because the evidence for them is merge and test data that Phase 3 owns. The type, the ordering, the glyphs and the "no state without evidence" rule are all built and tested now so Phase 3 only supplies the facts. That is a deliberate stub of a *state machine*, not of a feature: the definition of done is fully visible, carried from Discover, and honest about knowing nothing yet.

---

Plan complete and saved to `docs/superpowers/plans/2026-09-18-phase-4-discover.md`.








