// Package modelclient calls the Anthropic Messages API over plain HTTP.
//
// COMPLIANCE (CLAUDE.md, "AI capacity"): the key this client sends must be the
// signed-in user's OWN API key, or a corporate gateway key issued to them.
// A pooled Claude subscription OAuth token is forbidden — it breaches
// Anthropic's Consumer Terms and is blocked server-side. Phase 6 replaces
// ConfigFromEnv with the rotation engine but keeps the Completer interface.
//
// NO DEFAULT ENDPOINT (task 02 amendment 4, review I13): a brief's answers are
// engagement descriptions (client names, scope, constraints), and a key alone
// must never send them to a public API. The destination is always a value
// someone chose in PANACEA_APP_MODEL_BASE_URL. A key without one is "model
// off" (ErrNoBaseURL), which callers treat as a nil Completer and degrade.
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

// DefaultModel is the model every Discover call uses unless overridden. It is
// only a default: PANACEA_APP_MODEL_ID overrides it.
const DefaultModel = "claude-opus-5"

// DefaultTimeout is the HTTP ceiling on one call. It sits below the longest
// desktop budget (analyse, 60 s). It is a ceiling only: every caller also
// passes its own context (task 04's FollowUpBudget, task 13's DraftBudget),
// each inside a route deadline, so that the order model budget < route
// deadline < desktop timeoutMs holds for every Discover call that reaches a
// model. Without one of those budgets, the 45 s ceiling would not keep that
// call inside its desktop timeout (task 02 amendment 5, review I4).
const DefaultTimeout = 45 * time.Second

// defaultMaxTokens is the reply ceiling when Config.MaxTokens is not set.
const defaultMaxTokens = 8192

// apiVersion is the required anthropic-version header value.
const apiVersion = "2023-06-01"

// Errors returned by this package. ErrNoCredentials and ErrNoBaseURL both mean
// "model off": neither is a start-up failure (task 02 amendment 3).
var (
	ErrNoCredentials = errors.New("modelclient: PANACEA_APP_MODEL_API_KEY is not set")
	ErrNoBaseURL     = errors.New("modelclient: PANACEA_APP_MODEL_API_KEY is set but PANACEA_APP_MODEL_BASE_URL is not")
	ErrRefused       = errors.New("modelclient: the model declined the request")
	ErrNoText        = errors.New("modelclient: response contained no text block")
	ErrNoJSON        = errors.New("modelclient: no JSON object found in response text")
)

// Config describes how to reach a model. The key is the person's OWN API key
// or a corporate gateway key issued to them - never a Claude subscription
// token, never shared between people (CLAUDE.md, "AI capacity").
type Config struct {
	BaseURL   string        // PANACEA_APP_MODEL_BASE_URL, an origin; /v1/messages is appended; mandatory, no default
	APIKey    string        // PANACEA_APP_MODEL_API_KEY, mandatory
	Model     string        // PANACEA_APP_MODEL_ID
	MaxTokens int           // default 8192
	Timeout   time.Duration // default DefaultTimeout (45 s)
}

// ConfigFromEnv reads PANACEA_APP_MODEL_API_KEY, PANACEA_APP_MODEL_BASE_URL
// and PANACEA_APP_MODEL_ID. The key and the base URL are both mandatory;
// neither is defaulted. The key is read first: unset or blank is
// ErrNoCredentials. A key with the base URL unset or blank is ErrNoBaseURL.
// No other variable is read, ANTHROPIC_API_KEY included (G4).
func ConfigFromEnv() (Config, error) {
	key := strings.TrimSpace(os.Getenv("PANACEA_APP_MODEL_API_KEY"))
	if key == "" {
		return Config{}, ErrNoCredentials
	}
	base := strings.TrimSuffix(strings.TrimSpace(os.Getenv("PANACEA_APP_MODEL_BASE_URL")), "/")
	if base == "" {
		return Config{}, ErrNoBaseURL
	}
	model := strings.TrimSpace(os.Getenv("PANACEA_APP_MODEL_ID"))
	if model == "" {
		model = DefaultModel
	}
	return Config{
		BaseURL:   base,
		APIKey:    key,
		Model:     model,
		MaxTokens: defaultMaxTokens,
		Timeout:   DefaultTimeout,
	}, nil
}

// Request is one non-streaming completion.
type Request struct {
	// System is the system prompt. Sent as a single text block.
	System string
	// User is the single user turn.
	User string
	// Effort maps to output_config.effort: low, medium, high, xhigh or max.
	// Empty means the model's own default (high on claude-opus-5).
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

// Completer is THE SEAM PHASE 6 REPLACES with its per-user rotation engine.
// The interface does not change when it does. Tests substitute a
// deterministic fake. A nil Completer means "no model": every Discover caller
// treats it so and degrades.
type Completer interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

// Client is the HTTP implementation of Completer.
type Client struct {
	cfg  Config
	http *http.Client
}

// New returns a Client. It does not contact the network. The key is checked
// first (ErrNoCredentials), then the base URL (ErrNoBaseURL); neither is
// guessed.
func New(cfg Config) (*Client, error) {
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	if cfg.APIKey == "" {
		return nil, ErrNoCredentials
	}
	cfg.BaseURL = strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		return nil, ErrNoBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = defaultMaxTokens
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
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

// wireOutputConfig is output_config. Both fields were checked against the
// Messages API reference on 2026-09-24 (task 02 step 1): effort is GA inside
// output_config, and format {type: json_schema, schema} is the current
// structured-output field (the top-level output_format is the deprecated one).
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
//
// It balances braces and skips braces inside double-quoted strings; it does
// not validate what it returns. Callers unmarshal and schema-check the result
// and degrade on any failure (task 04), so a wrong guess costs a degrade, not
// wrong data.
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

// The seam phase 6 replaces is checked by the compiler, not by a caller
// (task 02 amendment 2).
var _ Completer = (*Client)(nil)
