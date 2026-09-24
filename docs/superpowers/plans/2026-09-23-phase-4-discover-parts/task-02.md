### Task 02: `modelclient` — kept from the 2026-09-18 plan's task 1

**Kept, by reference.** The old plan's task 1 (`2026-09-18-phase-4-discover.md:68-580`) is
still right, and the survey says so (§3): one `POST /v1/messages` over plain HTTP, no Anthropic
Go SDK because `v1.73.0` declares `go 1.24` / `toolchain go1.25.8` and every module here is pinned
to `go 1.23` (G1). Its test file (`:95-258`, seven tests) and implementation (`:268-560`) are
**copied as written**, with the five amendments below. Read that region; this part does not
repeat it.

**Amendments:**

1. **Verify the wire field names before copying.** The old plan sends `output_config.effort` and
   `output_config.format` (`:355-360`, `:420-450`). They are the old plan's, and this pass did not
   check them against the current Messages API reference. Step 1 checks them; a field the API does
   not accept is dropped from the request, not guessed at. `DefaultModel = "claude-opus-5"` is
   likewise the old plan's value and is only a default: `PANACEA_APP_MODEL_ID` overrides it.
2. **Add `var _ Completer = (*Client)(nil)`** at the foot of `client.go`, so the seam phase 6
   replaces is checked by the compiler, not by a caller.
3. **"No model" is a nil `Completer`.** `ConfigFromEnv` returning `ErrNoCredentials` or
   `ErrNoBaseURL` (amendment 4) is not an error at start-up: `cmd/api` (task 14) logs one line and passes a nil `Completer` to
   `discover.ProposeFollowUp` and `repoanalysis.DraftAnswers` (task 13), both of which treat nil as "no model"
   and degrade. Nothing else in the process holds a key.
4. **No default endpoint (review I13).** The old plan's `DefaultBaseURL = "https://api.anthropic.com"`
   (`:299-302`) and both fallbacks to it (`ConfigFromEnv`'s `getenv(..., DefaultBaseURL)` at `:331`
   and `New`'s `if cfg.BaseURL == ""` at `:389-391`) are **deleted, not copied**. `ConfigFromEnv`'s
   doc comment (`:327-328`, "Only the key is mandatory.") is **rewritten**, not copied: "the key and
   the base URL are both mandatory; neither is defaulted". A brief's answers
   are engagement descriptions (client names, scope, constraints), and a key alone must never send
   them to a public API. The model's destination is always a value someone chose:
   - `ConfigFromEnv` reads the key first. Key unset: `ErrNoCredentials`, as before. Key set and
     `PANACEA_APP_MODEL_BASE_URL` unset or blank: the new `ErrNoBaseURL`. Both mean "model off"
     (amendment 3). Neither is a start-up failure.
   - `New(Config{APIKey: "k"})` with an empty `BaseURL` returns `ErrNoBaseURL`. It does not guess.
   - The old test at `:223` (`New(Config{BaseURL: "http://x"})` is `ErrNoCredentials`) is kept: the
     key check still runs first.
5. **The HTTP ceiling is below the longest desktop budget (analyse, 60 s) (review I4).** The old
   plan's `Timeout: 120 * time.Second` (`:335`, `:399-400`) becomes `DefaultTimeout = 45 * time.Second`.
   It is **not** below the 30 s follow-up budget or the 10 s default, and it does not need to be:
   the per-call budgets keep each call under its own desktop timeout. Without one of them, the
   45 s ceiling would not keep that call inside its desktop timeout. It is a ceiling only; every caller also passes its own context: task 04's
   `FollowUpBudget` (20 s) and task 13's `DraftBudget` (35 s), each inside a route deadline
   (task 11's `Budgets`: 28 s and 55 s). The desktop waits 30 s for `apply` with
   `propose_follow_up` and 60 s for `analyse` (task 15), so a slow model reaches the desktop as the
   server's typed degrade, never as the desktop's `timeout`. The ordering rule is: model budget <
   route deadline < desktop `timeoutMs`, for every Discover call that reaches a model.

**Files:**
- `control-plane/internal/modelclient/client.go` (new)
- `control-plane/internal/modelclient/client_test.go` (new)
- `control-plane/internal/modelclient/timeout_internal_test.go` (new; `package modelclient`, so
  it can read the built `http.Client`'s `Timeout`; the test table below always named it)

**Interfaces — complete source** (the old plan's declarations, restated so this part stands alone;
the bodies are the old plan's):

```go
package modelclient

// No DefaultBaseURL: the endpoint is never defaulted (amendment 4, review I13).
const DefaultModel = "claude-opus-5" // default only; PANACEA_APP_MODEL_ID overrides
const DefaultTimeout = 45 * time.Second // HTTP ceiling; below the longest desktop budget (analyse, 60 s); the per-call budgets keep each call under its own (amendment 5)

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
| `TestCompleteSendsTheRequiredHeadersAndBody` | `POST`, path `/v1/messages`, `content-type: application/json`, `x-api-key`, `anthropic-version: 2023-06-01`; model, the one user turn and the system text block in the body; `Response.Model` and usage read back | a client that drops the version header |
| `TestCompleteSendsJSONSchemaWhenSupplied` | the schema reaches the body | a schema silently ignored |
| `TestCompleteSurfacesHTTPErrors` | a 401 row **and a 500 row**, each an error carrying the status and the body (the old plan's test had the 401 only, which cannot kill the mutant in the next column) | a client that returns empty text on 500 |
| `TestCompleteReportsRefusal` | `stop_reason: refusal` is `ErrRefused` | a refusal read as an empty answer |
| `TestNewRejectsAMissingKey` | `New(Config{BaseURL: "http://x"})` (the old row), `New(Config{})` and a blank key are each `ErrNoCredentials`: the key check runs first | a client that sends no key |
| `TestExtractJSON` | the object inside prose and fences is found; **plus a "first of two objects" row** (`{"a":1} and then {"b":2}` gives `{"a":1}`): each of the old five rows holds one outermost brace pair, so the greedy mutant in the next column passed all five | a greedy first-`{`-to-last-`}` extractor |
| `TestExtractJSONFailsWhenAbsent` | no object is `ErrNoJSON` | a nil-byte success |

Add these:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestConfigFromEnvReadsOnlyPanaceaAppVariables` | with `ANTHROPIC_API_KEY` (and `ANTHROPIC_AUTH_TOKEN`) set and `PANACEA_APP_MODEL_API_KEY` unset, then blank, `ConfigFromEnv` is `ErrNoCredentials` | a fallback to a variable a developer's shell may hold for a different account (G4) |
| `TestAKeyWithoutABaseURLIsModelOff` | with `PANACEA_APP_MODEL_API_KEY` set, `ANTHROPIC_BASE_URL` set, and `PANACEA_APP_MODEL_BASE_URL` unset, then set to `"  "`, `ConfigFromEnv` is `ErrNoBaseURL`; `New(Config{APIKey: "k"})` and a blank `BaseURL` are `ErrNoBaseURL` | the old `getenv(..., "https://api.anthropic.com")` fallback, in either `ConfigFromEnv` or `New` (I13); a fallback to `ANTHROPIC_BASE_URL` |
| `TestTheDefaultTimeoutIsBelowTheLongestDesktopBudget` | `DefaultTimeout < 60*time.Second`; `New(Config{APIKey: "k", BaseURL: "http://x"})` (no `Timeout`) builds an `http.Client` whose `Timeout` is `DefaultTimeout` (an internal `package modelclient` test file, `timeout_internal_test.go`, reads the unexported field, so no test waits 45 s) | a default left at the old 120 s, which outlasts task 15's 60 s `analyse` budget (I4) |

**As built (task 02, commits d6867dc and 59879db):** the tests above, and five more. One pins the
happy path. The other four were written after the mutation run at d6867dc, where both predicted
survivors (S1, S2) survived:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestConfigFromEnvReadsTheThreeVariables` | key, base URL (spaces and a trailing slash trimmed), `PANACEA_APP_MODEL_ID` and the `MaxTokens`/`Timeout` defaults; no model id gives `DefaultModel`; `DefaultTimeout` is 45 s | `PANACEA_APP_MODEL_ID` ignored; the trailing slash kept |
| `TestNewFillsTheDefaults` | `New` with only a key and a base URL ending in `/` posts to `/v1/messages` with `DefaultModel`, `max_tokens` 8192 and no `output_config` | `New`'s defaults dropped |
| `TestCompleteReportsNoTextBlock` | a thinking-only reply at `max_tokens`, and empty content, are `ErrNoText` | S1: a 200 with no text block read as a success with empty text |
| `TestCompleteBoundsTheReplyItReads` | a body of exactly 8 MiB is read whole; one byte more is an error (both sides of the bound, fix round 1) | the read limit removed; a shift typo (`8<<10`) or an off-by-one limit (R2) |
| `TestACallersTimeoutIsKept` (internal) | a caller's `Timeout: 5s` reaches the `http.Client` | S2: `New` always using `DefaultTimeout` |

**As built, fix round 1 (review `task-t02-review.md`):** after the reviewer's mutation run, where
all six of its mutants (R1 to R6) survived:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCompleteHonoursTheCallersContext` | against a handler that holds the call open, a 50 ms deadline is `context.DeadlineExceeded` and a cancel once the call is in flight is `context.Canceled`, each inside 10 s (the client's own ceiling is 45 s). Only this package can pin it: tasks 04 and 13 use a fake `Completer` | R1: `Complete` ignoring the caller's `ctx`, which amendment 5's ordering rests on (review I1) |
| `TestCompleteReportsRefusal` (+ row) | a refusal that carries partial text is `ErrRefused`, with no `Text` | R3: the refusal checked only when there is no text (M2) |
| `TestCompleteSendsJSONSchemaWhenSupplied` (+ effort) | `Effort: "low"` beside a schema reaches `output_config.effort` | R6: the effort dropped when a schema is sent (M6) |
| `TestNewTrimsTheModelID` | `New` sends a padded model id trimmed, and a blank one as `DefaultModel` | `New` not trimming the model id (M5) |
| `TestCompleteJoinsEveryTextBlock` | two text blocks, after a thinking block, give `"ab"` | R4: only the first text block kept |
| `TestExtractJSON` (+ row) | `}{"a":{"b":1}}` gives the whole object | R5: the `depth > 0` guard removed (returns the inner `{"b":1}`) |
| `TestCompleteCapsTheBodyItQuotesInAnError` | a 64 KiB error body is quoted to exactly its first 2 KiB, with its full length named | the whole body, up to 8 MiB, quoted into an error that lands in logs |
| `TestTheQuotedBodyIsCutOnARuneBoundary` | the 2 KiB cut backs up to the start of a character it would split; the error is valid UTF-8 | a byte cut that leaves half a character in a log line |

A blank key is "model off" (`ErrNoCredentials`), in `ConfigFromEnv` and in `New`, the same way a
blank base URL is `ErrNoBaseURL`; the key and the model id are trimmed, in both. This is a
departure from "copied as written", declared in the task 02 report. So is the capped error
body: the old plan quoted the whole body.

**Steps:**

- [x] **Step 1:** Check `output_config` against the Messages API reference; record the result in
  the commit message. If a field is not accepted, drop it and its test assertion.
  *Result (d6867dc):* both accepted, per the bundled claude-api skill reference (cached
  2026-06-24; no live docs fetched): `output_config.effort` is GA (low to max; default `high` on
  `claude-opus-5`, `medium` on `claude-opus-5-5`), and `output_config.format {type: json_schema,
  schema}` is the current field (top-level `output_format` is the deprecated one). Nothing dropped.
- [x] **Step 2:** Write the tests; run `go test ./control-plane/internal/modelclient/...` — expect
  a compile failure.
- [x] **Step 3:** Copy the implementation with the five amendments.
- [x] **Step 4:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/modelclient/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/modelclient
$ git diff --stat HEAD -- control-plane/go.mod go.work
(no output — no dependency added, G1)
```

**Does NOT claim:** that any real model was called (every test uses `httptest`); that the gateway
at `PANACEA_APP_MODEL_BASE_URL` accepts the same body (unverified until task 23 runs with a key).
