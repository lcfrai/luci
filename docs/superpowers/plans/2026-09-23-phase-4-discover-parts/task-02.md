### Task 02: `modelclient` — kept from the 2026-09-18 plan's task 1

**Kept, by reference.** The old plan's task 1 (`2026-09-18-phase-4-discover.md:68-580`) is
still right, and the survey says so (§3): one `POST /v1/messages` over plain HTTP, no Anthropic
Go SDK because `v1.73.0` declares `go 1.24` / `toolchain go1.25.8` and every module here is pinned
to `go 1.23` (G1). Its test file (`:95-258`, seven tests) and implementation (`:268-560`) are
**copied as written**, with the three amendments below. Read that region; this part does not
repeat it.

**Amendments:**

1. **Verify the wire field names before copying.** The old plan sends `output_config.effort` and
   `output_config.format` (`:355-360`, `:420-450`). They are the old plan's, and this pass did not
   check them against the current Messages API reference. Step 1 checks them; a field the API does
   not accept is dropped from the request, not guessed at. `DefaultModel = "claude-opus-5"` is
   likewise the old plan's value and is only a default: `PANACEA_APP_MODEL_ID` overrides it.
2. **Add `var _ Completer = (*Client)(nil)`** at the foot of `client.go`, so the seam phase 6
   replaces is checked by the compiler, not by a caller.
3. **"No model" is a nil `Completer`.** `ConfigFromEnv` returning `ErrNoCredentials` is not an
   error at start-up: `cmd/api` (task 14) logs one line and passes a nil `Completer` to
   `discover.ProposeFollowUp` and `repoanalysis.DraftBrief`, both of which treat nil as "no model"
   and degrade. Nothing else in the process holds a key.

**Files:**
- `control-plane/internal/modelclient/client.go` (new)
- `control-plane/internal/modelclient/client_test.go` (new)

**Interfaces — complete source** (the old plan's declarations, restated so this part stands alone;
the bodies are the old plan's):

```go
package modelclient

const DefaultBaseURL = "https://api.anthropic.com"
const DefaultModel = "claude-opus-5" // default only; PANACEA_APP_MODEL_ID overrides

var (
	ErrNoCredentials = errors.New("modelclient: PANACEA_APP_MODEL_API_KEY is not set")
	ErrRefused       = errors.New("modelclient: the model declined the request")
	ErrNoText        = errors.New("modelclient: response contained no text block")
	ErrNoJSON        = errors.New("modelclient: no JSON object found in response text")
)

// Config describes how to reach a model. The key is the person's OWN API key
// or a corporate gateway key issued to them - never a Claude subscription
// token, never shared between people (CLAUDE.md, "AI capacity").
type Config struct {
	BaseURL   string        // PANACEA_APP_MODEL_BASE_URL, an origin; /v1/messages is appended
	APIKey    string        // PANACEA_APP_MODEL_API_KEY, mandatory
	Model     string        // PANACEA_APP_MODEL_ID
	MaxTokens int           // default 8192
	Timeout   time.Duration // default 120 s
}

func ConfigFromEnv() (Config, error)

type Request struct {
	System     string
	User       string
	Effort     string          // "" = server default
	JSONSchema json.RawMessage // nil = unconstrained
}

type Response struct {
	Text         string
	Model        string
	InputTokens  int
	OutputTokens int
}

// Completer is THE SEAM PHASE 6 REPLACES with its per-user rotation engine.
// The interface does not change when it does.
type Completer interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

type Client struct{ /* unexported */ }

func New(cfg Config) (*Client, error)
func (c *Client) Complete(ctx context.Context, req Request) (Response, error)

// ExtractJSON returns the first balanced top-level JSON object in text.
func ExtractJSON(text string) ([]byte, error)

var _ Completer = (*Client)(nil)
```

**Tests first** — the old plan's seven (`:110-258`), unchanged in intent:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCompleteSendsTheRequiredHeadersAndBody` | path `/v1/messages`, `x-api-key`, `anthropic-version: 2023-06-01`, model and user turn in the body | a client that drops the version header |
| `TestCompleteSendsJSONSchemaWhenSupplied` | the schema reaches the body | a schema silently ignored |
| `TestCompleteSurfacesHTTPErrors` | a 4xx/5xx is an error carrying the status | a client that returns empty text on 500 |
| `TestCompleteReportsRefusal` | `stop_reason: refusal` is `ErrRefused` | a refusal read as an empty answer |
| `TestNewRejectsAMissingKey` | `New(Config{})` is `ErrNoCredentials` | a client that sends no key |
| `TestExtractJSON` | the object inside prose and fences is found | a greedy first-`{`-to-last-`}` extractor |
| `TestExtractJSONFailsWhenAbsent` | no object is `ErrNoJSON` | a nil-byte success |

Add one:

| `TestConfigFromEnvReadsOnlyPanaceaAppVariables` | with `ANTHROPIC_API_KEY` set and `PANACEA_APP_MODEL_API_KEY` unset, `ConfigFromEnv` is `ErrNoCredentials` | a fallback to a variable a developer's shell may hold for a different account (G4) |

**Steps:**

- [ ] **Step 1:** Check `output_config` against the Messages API reference; record the result in
  the commit message. If a field is not accepted, drop it and its test assertion.
- [ ] **Step 2:** Write the tests; run `go test ./control-plane/internal/modelclient/...` — expect
  a compile failure.
- [ ] **Step 3:** Copy the implementation with the three amendments.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/modelclient/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient
$ git diff --stat HEAD -- control-plane/go.mod go.work
(no output — no dependency added, G1)
```

**Does NOT claim:** that any real model was called (every test uses `httptest`); that the gateway
at `PANACEA_APP_MODEL_BASE_URL` accepts the same body (unverified until task 23 runs with a key).
