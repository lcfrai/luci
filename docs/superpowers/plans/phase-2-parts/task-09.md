### Task 9: The error body and the origin guard

This is the first task in Group C and the only one with no dependencies, because everything else in the group imports it. It produces two things that belong to no single handler. The first is **one error body**: contract §3.2 fixes a fifteen-code enum and a three-field JSON object — sixteen codes as built, because Group C's shared-package ruling 7 adds `non_loopback_publish` and makes this task the place it is declared — and the desktop's API client is written against exactly that shape, so a handler that invents `{"err": "..."}` produces a cockpit error branch that silently never matches. The second is **the entire security boundary of a control plane that has no authentication at all**. The plan's cross-group note says the boundary is "loopback bind + `Host` guard + `Origin` allowlist"; task 13 owns the bind, and this task owns the other two. There is no bearer token on the desktop→control-plane hop and there is not meant to be one before Phase 8, so if this middleware is wrong, any web page the user happens to have open can create sandboxes on their laptop.

**The one thing this task must not get wrong is that the middleware GRANTS as well as rejects.** Contract §2.12 specifies an `OriginPolicy` that rejects a bad origin and says nothing about what a good one receives — and those are different mechanisms. The plan's "Cross-group decisions, adjudicated" section resolves it, decision 1, and that resolution is binding here: a Tauri webview *is* a browser, and every build is cross-origin against `http://127.0.0.1:8730` (`http://localhost:1420` in dev differs in host and port; `http://tauri.localhost` packaged on Windows differs in host; `tauri://localhost` on macOS and Linux differs in scheme). Without a granted `Access-Control-Allow-Origin` header, `GET /v1/machines` fails at the response with "No 'Access-Control-Allow-Origin' header is present on the requested resource", and every `POST`, `PATCH` and `DELETE` carrying `Content-Type: application/json` fails one step earlier at a preflight `OPTIONS` that the mux does not route and would answer 405. **WebSockets are exempt** — no preflight, no ACAO requirement — which is precisely why Phase 1's attach relay worked with none of this and why the omission would not surface until task 24 makes the cockpit's first REST call. So: on every request whose `Origin` is present and allowed, set `Access-Control-Allow-Origin` to that origin **echoed verbatim, never `*`**, plus `Vary: Origin`; on `OPTIONS` with an allowed origin answer **204 before the mux** with `Access-Control-Allow-Methods: GET, POST, PATCH, DELETE, OPTIONS`, `Access-Control-Allow-Headers: Content-Type` and `Access-Control-Max-Age: 600`; and never `Access-Control-Allow-Credentials`, because no cookies are used and sending it with an echoed origin would turn the allowlist into the only thing standing between a rejected page and an authenticated one.

Contract §2.12 explains at length why `coder/websocket`'s built-in `AcceptOptions.OriginPatterns` is not reused, and the reason applies to the REST surface too: it matches on the Origin's **host only**, ignoring the scheme, and returns nil for a **missing** Origin. Both are sane defaults for a public web server and both are wrong for a loopback control plane whose threat model is a page the user has open in another tab. `Allow` therefore compares the **full origin string**. A missing Origin is still allowed, and that is not an inconsistency: `curl`, the Go test client and every non-browser tool send none, while the attacker in this model is a browser, and a browser always sends one.

The `Host` guard is the DNS-rebinding defence and it is the one that actually matters, because it is the only check that survives an attack carrying no `Origin` at all. A page at `http://evil.example` can point that name at `127.0.0.1`; its `fetch` then carries `Origin: http://evil.example` and is caught by the allowlist, but an `<img>` or `<script src>` it emits carries **no** `Origin` and is caught by nothing else. Requiring `Host` to name loopback — `127.0.0.0/8`, `::1` or the literal `localhost`, with or without a port — closes it. Both checks ship or neither is worth having.

Six decisions where the contract is silent, made here so no later task re-derives them. **(a) The sixteen `error` codes become exported constants.** §3.2 gives fifteen of them as a table of strings and Group C's ruling 7 adds the sixteenth, `non_loopback_publish`; tasks 10 to 13 would otherwise write sixteen string literals across four files and the desktop would never match the one that was mistyped. **(b) `WriteJSON` marshals into a buffer before touching the `ResponseWriter`**, so a value that cannot be encoded still produces a truthful 500 with a constant body rather than a 200 whose body stops mid-object — the failure mode where the status line has already been sent is unrecoverable and this is the only way to avoid it. **(c) `DecodeJSON` rejects trailing content** after the first JSON value, because `{"name":"a"}{"name":"b"}` currently decodes as the first object and silently discards the second. **(d) `DecodeJSON` does not require `Content-Type: application/json`.** Requiring it would force a preflight on cross-origin posts and so add a second CSRF defence, but the origin allowlist already rejects every cross-origin caller, and requiring it breaks `curl` for no gain. **(e) A rejected `Host` still receives the CORS grant if its `Origin` was allowed**, so a developer whose `PANACEA_APP_ADDR` is misconfigured reads the actual `forbidden_host` JSON in the browser console instead of an opaque CORS failure. **(f) An explicitly configured allowlist that reduces to nothing stays empty** rather than falling back to the defaults: `NewOriginPolicy(nil, …)` means defaults, but `NewOriginPolicy([]string{" "}, …)` means the operator asked for something and got nothing, and silently restoring five origins they did not ask for is worse than a warning and a browser that cannot connect.

**One trap that will bite tasks 10 to 12 if it is not said here.** `httptest.NewRequest(http.MethodGet, "/v1/machines", nil)` sets `Host` to `example.com`, so every handler test that routes through this middleware with a path-only target gets a 403 `forbidden_host` and looks like a routing bug. Always pass an absolute target — `"http://127.0.0.1:8730/v1/machines"` — or set `req.Host` explicitly. Both test files in this task do it the first way.

Both test files are in `package httpx`, not `package httpx_test`. That is deliberate and the step-6 transcript depends on it: an internal test file makes the compiler print a bare `undefined: NewOriginPolicy`, while an external one prints `undefined: httpx.NewOriginPolicy`, and nothing in this package needs the API-surface discipline an external test buys.

**Files:**
- `control-plane/internal/httpx/problem.go` — the `Problem` body, the sixteen error codes, `WriteJSON`, `WriteProblem`, `DecodeJSON` (new)
- `control-plane/internal/httpx/problem_test.go` — exact bytes on the wire, and every rejection `DecodeJSON` owns (new)
- `control-plane/internal/httpx/origin.go` — `OriginPolicy`, `DefaultOrigins`, `Allow`, `LoopbackHost`, `Middleware` with the CORS grant (new)
- `control-plane/internal/httpx/origin_test.go` — grant, preflight, both rejections, and the log line each rejection must leave (new)

**Interfaces:**
- Produces:
  - `httpx.Problem`, `httpx.MaxBodyBytes`, and `httpx.Code*` — the fifteen error-code constants of contract §3.2, plus `CodeNonLoopbackPublish`, the sixteenth that Group C's ruling 7 adds
  - `httpx.WriteJSON(w, status, v)`, `httpx.WriteProblem(w, status, code, msg, details)`, `httpx.DecodeJSON[T](w, r, dst) bool`
  - `httpx.OriginPolicy`, `httpx.NewOriginPolicy(allowed []string, log *slog.Logger) *OriginPolicy`, `httpx.DefaultOrigins() []string`
  - `(*OriginPolicy).Allow(origin string) bool` — called directly by task 12's attach handler **before** `websocket.Accept`, which is what makes a rejected upgrade a readable 403 with a JSON body instead of a close frame the browser reports as "connection failed"
  - `(*OriginPolicy).Allowed() []string` — the effective list, so task 13's startup log can print it
  - `(*OriginPolicy).Middleware(next http.Handler) http.Handler` — wrapped around the routed mux by task 10's `api.New`; `GET /v1/healthz` sits outside it on an outer mux, because contract §3.1 marks it "Unauthenticated, no origin check, for a supervisor" and task 10's decision (a) mounts it accordingly
  - `httpx.LoopbackHost(hostHeader string) bool` — the same predicate task 13's bind refusal needs; it parses a `Host` header, not an address, so task 13 splits its own `PANACEA_APP_ADDR` before calling it
- Consumes: nothing. This package imports only the standard library, and must keep doing so — tasks 10 to 13 all import it, so any edge pointing back would be an import cycle the first time a handler needed a helper.
- Required by: task 10 (`api.New` mounts the middleware and every catalog handler writes through `WriteJSON`), task 11 (`DecodeJSON` on create and patch, `CodeSessionNotFound`, `CodeConflict`, `CodeBadRequest`), task 12 (`Allow` pre-upgrade, `CodeUpgradeRequired`, `CodeSandboxNotRunning`, `CodeNonLoopbackPublish`), task 13 (`PANACEA_APP_ALLOWED_ORIGINS` parsed into `NewOriginPolicy`, and `LoopbackHost` for the bind refusal).
- Amends contract §2.12: `Middleware` grants CORS as well as rejecting, per the plan's adjudicated decision 1. Also adds three symbols §2.12 does not list — the `Code*` constants, `LoopbackHost` and `Allowed()` — and one field §2.12's `OriginPolicy` struct does not have: the unexported `order []string`, which holds the same set in configured order so `Allowed()` and the rejection log line are deterministic rather than map-ordered. All four are additive; none changes a signature the contract fixes.
- Amends contract §3.2: adds a sixteenth code, `non_loopback_publish` (500), per Group C's shared-package ruling 7. Task 12 emits it when the daemon reports a sandbox published off loopback, and that sandbox is *running*, so `sandbox_not_running` would tell the user something false; the cockpit has to be able to say why the attach refused.

- [ ] **Step 1: Write the failing tests for the error body**

The error body is a wire format with a second implementation in TypeScript, so these tests compare the **exact bytes** rather than round-tripping through `json.Unmarshal`. A round-trip passes even when the field is named `err`, because it is the desktop's parser and not Go's that has to match. Two details of that wire format are easy to get wrong and both are pinned below: `json.Encoder.Encode` appends a trailing `\n`, so every expected body ends with one; and `details` must **vanish** when empty rather than appearing as `null`, which is what the `omitempty` tag on a nil map buys.

Create `control-plane/internal/httpx/problem_test.go`:

```go
package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteProblemWritesTheExactBodyStatusAndHeaders(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteProblem(rec, http.StatusNotFound, CodeSessionNotFound,
		"No session with id s-3f2a91c0.", map[string]string{"id": "s-3f2a91c0"})

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	// nosniff, because the Message is rendered from data the caller supplied and
	// a browser that content-sniffs a JSON error body into HTML is an XSS.
	if got, want := rec.Header().Get("X-Content-Type-Options"), "nosniff"; got != want {
		t.Errorf("X-Content-Type-Options = %q, want %q", got, want)
	}

	want := `{"error":"session_not_found","message":"No session with id s-3f2a91c0.","details":{"id":"s-3f2a91c0"}}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n\t%q\nwant\n\t%q", got, want)
	}
}

func TestWriteProblemOmitsEmptyDetails(t *testing.T) {
	// Contract 3.2: "details is omitted when empty." Not null, omitted: the
	// desktop's model types it as optional and null would be a third state.
	for name, details := range map[string]map[string]string{
		"nil":   nil,
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteProblem(rec, http.StatusForbidden, CodeForbiddenHost,
				"This control plane only answers requests addressed to localhost.", details)

			want := `{"error":"forbidden_host","message":"This control plane only answers requests addressed to localhost."}` + "\n"
			if got := rec.Body.String(); got != want {
				t.Errorf("body =\n\t%q\nwant\n\t%q", got, want)
			}
		})
	}
}

func TestWriteJSONReportsItsOwnEncodingFailureAsA500(t *testing.T) {
	rec := httptest.NewRecorder()

	// A channel cannot be marshalled. The point of the test is the STATUS: the
	// encode must fail before anything reaches the ResponseWriter, because once
	// a 200 status line is on the wire no truthful correction is possible.
	WriteJSON(rec, http.StatusOK, map[string]any{"ch": make(chan int)})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); !strings.Contains(got, `"error":"internal"`) {
		t.Errorf("body = %q, want an internal Problem", got)
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("the fallback body is not valid JSON: %v", err)
	}
}

// decodeTarget is the shape DecodeJSON is exercised against. It has one string
// and one int so a type error can name a field.
type decodeTarget struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func decodeBody(t *testing.T, body string) (decodeTarget, *httptest.ResponseRecorder, bool) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost,
		"http://127.0.0.1:8730/v1/sessions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	var dst decodeTarget
	ok := DecodeJSON(rec, req, &dst)
	return dst, rec, ok
}

// problemFrom reads back the Problem DecodeJSON wrote, failing the test if what
// it wrote was not one.
func problemFrom(t *testing.T, rec *httptest.ResponseRecorder) Problem {
	t.Helper()
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body %q is not a Problem: %v", rec.Body.String(), err)
	}
	if p.Error != CodeInvalidJSON {
		t.Errorf("error = %q, want %q", p.Error, CodeInvalidJSON)
	}
	if p.Message == "" {
		t.Error("message is empty; a 400 with no sentence is a bug report with no content")
	}
	return p
}
```

Still `control-plane/internal/httpx/problem_test.go`, continuing in the same file below `problemFrom` — do not repeat the `package` clause or the imports:

```go
func TestDecodeJSONAcceptsAWellFormedBody(t *testing.T) {
	dst, rec, ok := decodeBody(t, `{"name":"api","count":3}`)

	if !ok {
		t.Fatalf("DecodeJSON rejected a valid body: %s", rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("DecodeJSON wrote %q on success; the handler owns the response", rec.Body.String())
	}
	if dst.Name != "api" || dst.Count != 3 {
		t.Errorf("dst = %+v, want {Name:api Count:3}", dst)
	}
}

func TestDecodeJSONRejectsAnUnknownField(t *testing.T) {
	// The reason this is a 400 and not a shrug: "worktreedId" silently ignored
	// is a session created against the wrong worktree, which the user then has
	// to notice themselves.
	_, rec, ok := decodeBody(t, `{"name":"api","colour":"blue"}`)

	if ok {
		t.Fatal("DecodeJSON accepted an unknown field")
	}
	p := problemFrom(t, rec)
	if !strings.Contains(p.Message, "colour") {
		t.Errorf("message = %q, want it to name the offending field", p.Message)
	}
}

func TestDecodeJSONNamesTheFieldWithTheWrongType(t *testing.T) {
	_, rec, ok := decodeBody(t, `{"count":"three"}`)

	if ok {
		t.Fatal("DecodeJSON accepted a string where an int was required")
	}
	p := problemFrom(t, rec)
	if !strings.Contains(p.Message, "count") {
		t.Errorf("message = %q, want it to name the field", p.Message)
	}
}

func TestDecodeJSONRejectsAnEmptyBody(t *testing.T) {
	_, rec, ok := decodeBody(t, "")

	if ok {
		t.Fatal("DecodeJSON accepted an empty body")
	}
	problemFrom(t, rec)
}

func TestDecodeJSONRejectsTrailingContent(t *testing.T) {
	// Without the second Decode this passes silently, having read the first
	// object and dropped the rest.
	_, rec, ok := decodeBody(t, `{"name":"a"}{"name":"b"}`)

	if ok {
		t.Fatal("DecodeJSON accepted two JSON values in one body")
	}
	problemFrom(t, rec)
}

func TestDecodeJSONRejectsABodyOverTheLimit(t *testing.T) {
	// One byte of padding past the limit is enough; the reader errors as soon
	// as the limit is crossed, so nothing here allocates a second megabyte.
	_, rec, ok := decodeBody(t, `{"name":"`+strings.Repeat("a", MaxBodyBytes)+`"}`)

	if ok {
		t.Fatal("DecodeJSON accepted a body over MaxBodyBytes")
	}
	p := problemFrom(t, rec)
	if !strings.Contains(p.Message, "1048576") {
		t.Errorf("message = %q, want it to state the byte limit", p.Message)
	}
}

func TestDecodeJSONFailureIsItselfAProblemResponse(t *testing.T) {
	// DecodeJSON writes the response itself and returns false; a handler that
	// then writes its own 400 would send two status lines. This pins the
	// headers so the caller can see it has already answered.
	_, rec, _ := decodeBody(t, `{`)

	if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	problemFrom(t, rec)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/httpx/... -race -count=1`

Expected: a build failure, not a test failure — a list of `undefined:` lines, one per **use site**, truncated by `gc` at ten. The directory holds one test file and no implementation, so the type checker resolves none of the names it references. They print bare rather than package-qualified because line 15 puts that file in `package httpx` rather than `package httpx_test`:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx [github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx.test]
control-plane\internal\httpx\problem_test.go:14:2: undefined: WriteProblem
control-plane\internal\httpx\problem_test.go:14:41: undefined: CodeSessionNotFound
control-plane\internal\httpx\problem_test.go:44:4: undefined: WriteProblem
control-plane\internal\httpx\problem_test.go:44:44: undefined: CodeForbiddenHost
control-plane\internal\httpx\problem_test.go:61:2: undefined: WriteJSON
control-plane\internal\httpx\problem_test.go:69:8: undefined: Problem
control-plane\internal\httpx\problem_test.go:88:8: undefined: DecodeJSON
control-plane\internal\httpx\problem_test.go:94:64: undefined: Problem
control-plane\internal\httpx\problem_test.go:99:8: undefined: Problem
control-plane\internal\httpx\problem_test.go:103:16: undefined: CodeInvalidJSON
control-plane\internal\httpx\problem_test.go:103:16: too many errors
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx [build failed]
FAIL
```

That is the right failure: it proves the new directory resolves through the workspace under its real import path, and the names it cannot find are exactly the ones step 3 is about to declare. Two properties of that list are worth knowing before you read it as a symptom. `WriteProblem` and `Problem` each appear more than once because `undefined: X` is emitted per use site, not per symbol. And the run stops at ten with `too many errors`: `gc` truncates there, so the eleventh and twelfth — a second `CodeInvalidJSON` at `104:44` and `MaxBodyBytes` at `176:62` — are real and simply never printed. Do not read their absence as a sign they resolved; `go test ./control-plane/internal/httpx/... -count=1 -gcflags=-e` lifts the limit and prints all twelve. The names and their order are the assertion, not the line and column numbers, which move the moment a comment reflows.

What you will **not** see is `no non-test Go files in ...`. That is a `go build` diagnostic and this is `go test`, which builds the *test* package: `go build ./control-plane/internal/httpx/...` on this same state prints nothing at all. If instead you see `no required module provides package github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx`, the directory was created outside `control-plane/` — the module prefix is `github.com/kpmg-au-emu/panacea-app/control-plane`, with `/control-plane` in it. Paths print with `\` on Windows and `/` elsewhere.

- [ ] **Step 3: Write the error body**

This is contract §2.12's first block and §3.2's code table, plus decisions (a), (b), (c) and (d) from the header. Nothing here reaches outside the standard library.

Create `control-plane/internal/httpx/problem.go`:

```go
// Package httpx holds the two things every control-plane handler needs and that
// belong to no single handler: the one error body the whole API answers with,
// and the guard deciding whether a browser context may talk to this process.
//
// It imports only the standard library, and must keep doing so. Tasks 10 to 13
// all import it; an edge pointing the other way becomes an import cycle the
// first time a handler needs a helper.
package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// MaxBodyBytes bounds a request body. The REST surface carries an opening
// prompt, not a file.
const MaxBodyBytes = 1 << 20

// Problem is the error body of every non-2xx JSON response on this API. One
// shape everywhere means the desktop's client has one error branch.
type Problem struct {
	// Error is a stable machine-readable code. The desktop switches on it.
	Error string `json:"error"`
	// Message is one human sentence, safe to render. It never contains a token,
	// a file path outside the user's own projects, or a Go error chain.
	Message string `json:"message"`
	// Details carries structured context, e.g. {"id": "s-3f2a91c0"}.
	Details map[string]string `json:"details,omitempty"`
}

// The complete set of error codes: contract 3.2's fifteen, plus the
// non_loopback_publish that Group C's shared-package ruling 7 adds. That one is
// task 12's, and it is a sixteenth code rather than a reuse of
// sandbox_not_running because a sandbox the daemon reports published off
// loopback IS running - saying otherwise would be a lie the cockpit then shows
// the user. They are constants and not string literals at the call sites
// because the desktop switches on them: a typo in one handler is a code the
// cockpit can never match and that no compiler would have flagged.
const (
	CodeInvalidJSON        = "invalid_json"         // 400 not JSON, unknown field, or over 1 MiB
	CodeBadRequest         = "bad_request"          // 400 a field is missing or unusable
	CodeUnknownHarness     = "unknown_harness"      // 400 harness is not in the catalogue
	CodeUnsupportedOption  = "unsupported_option"   // 400 the harness does not accept that option
	CodeForbiddenOrigin    = "forbidden_origin"     // 403 Origin present and not allowlisted
	CodeForbiddenHost      = "forbidden_host"       // 403 Host is not loopback
	CodeProjectNotFound    = "project_not_found"    // 404
	CodeWorktreeNotFound   = "worktree_not_found"   // 404
	CodeSessionNotFound    = "session_not_found"    // 404
	CodeConflict           = "conflict"             // 409 wrong state for the verb
	CodeSandboxNotRunning  = "sandbox_not_running"  // 409 attach or restart, sandbox gone
	CodeHarnessUnavailable = "harness_unavailable"  // 409 in the catalogue, not in the image
	CodeUpgradeRequired    = "upgrade_required"     // 426 /attach without WebSocket headers
	CodeInternal           = "internal"             // 500 message is generic, detail goes to the log
	CodeNonLoopbackPublish = "non_loopback_publish" // 500 sandbox port bound off loopback (task 12)
	CodeBackendUnavailable = "backend_unavailable"  // 503 no Docker daemon, no kubeconfig
)

// internalBody is the one response this package can always send. It is a
// constant so the encode-failure path cannot itself fail to encode.
const internalBody = `{"error":"internal","message":"The control plane failed to encode its own response."}` + "\n"

func writeBytes(w http.ResponseWriter, status int, body []byte) {
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	// The Message can carry text the caller supplied. A browser that sniffs a
	// JSON error body into HTML would make that an XSS.
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// WriteJSON writes v as JSON with the given status.
//
// It marshals into a buffer BEFORE touching w. Encoding straight to the
// ResponseWriter would send the status line first, so a value that fails
// halfway through produces a 200 with a truncated body and no way back. Buffered,
// the failure is still a truthful 500.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		writeBytes(w, http.StatusInternalServerError, []byte(internalBody))
		return
	}
	writeBytes(w, status, buf.Bytes())
}

// WriteProblem writes a Problem.
func WriteProblem(w http.ResponseWriter, status int, code, msg string, details map[string]string) {
	WriteJSON(w, status, Problem{Error: code, Message: msg, Details: details})
}
```

Still `control-plane/internal/httpx/problem.go`, continuing below `WriteProblem`. `errors`, `fmt`, `io` and `strings` are in the import block above and are used only here, so the file does not compile until this part is transcribed too:

```go
// DecodeJSON reads a JSON body into dst. It rejects unknown fields and bodies
// over 1 MiB, writes a 400 invalid_json Problem itself on failure, and reports
// whether the handler should continue.
//
// It deliberately does NOT require Content-Type: application/json. Requiring it
// would force a preflight on cross-origin posts and so add a second CSRF
// defence, but OriginPolicy already rejects every cross-origin caller, and the
// requirement breaks curl for nothing.
func DecodeJSON[T any](w http.ResponseWriter, r *http.Request, dst *T) bool {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		WriteProblem(w, http.StatusBadRequest, CodeInvalidJSON, decodeMessage(err), nil)
		return false
	}
	// A second value in the body is not documented anywhere on this API, and
	// the silent reading is the dangerous one: `{"a":1}{"b":2}` otherwise
	// decodes as the first object with the second dropped and no complaint.
	if err := dec.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		WriteProblem(w, http.StatusBadRequest, CodeInvalidJSON,
			"Body must contain exactly one JSON object.", nil)
		return false
	}
	return true
}

// decodeMessage turns a decoder error into one human sentence.
//
// The json package's own text for a syntax, type or unknown-field error is
// reused, because it names the offending field or byte offset and carries no
// secret and no filesystem path. It does reveal Go field names on a type error;
// on a loopback developer tool that is worth the diagnosability. Anything this
// does not recognise gets a generic sentence rather than err.Error(), because
// that is the branch an unexpected wrapped error takes and contract 2.12
// forbids a Go error chain in Message.
func decodeMessage(err error) string {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) {
		return fmt.Sprintf("Request body exceeds the %d byte limit.", MaxBodyBytes)
	}
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		return fmt.Sprintf("Body is not valid JSON: %s (at byte %d).", syntax, syntax.Offset)
	}
	var wrongType *json.UnmarshalTypeError
	if errors.As(err, &wrongType) {
		if wrongType.Field != "" {
			return fmt.Sprintf("Field %q must be %s, not %s.", wrongType.Field, wrongType.Type, wrongType.Value)
		}
		return fmt.Sprintf("Body must be %s, not %s.", wrongType.Type, wrongType.Value)
	}
	switch {
	case errors.Is(err, io.EOF):
		return "Body is empty; a JSON object is required."
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "Body ended in the middle of a JSON value."
	}
	if field, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		return "Body has an unknown field: " + field + "."
	}
	return "Body could not be read as JSON."
}
```

- [ ] **Step 4: Run the error-body tests to verify they pass**

Run: `go test ./control-plane/internal/httpx/... -race -count=1`

Expected one `ok` line naming the package, and no `FAIL`:

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx	8.283s
```

The duration is fixed per-package overhead plus the 1 MiB body allocation and is not worth matching: the 8.283s above is one Windows laptop's first `-race` run of this package with a cold build cache and an on-access virus scanner, a warm re-run on the same machine prints under two seconds, and neither number is the assertion — the `ok` is. Two failures are worth naming in advance because both look like a broken implementation and are actually a broken expectation. If `TestWriteProblemWritesTheExactBodyStatusAndHeaders` reports a diff whose only difference is a trailing `\n`, the implementation used `json.Marshal` instead of `json.NewEncoder(...).Encode` — keep `Encode` and keep the newline, since the desktop parses with `response.json()`, which does not care, and a buffered encode is what makes step 3's 500 path possible. If `TestDecodeJSONRejectsABodyOverTheLimit` fails on the message rather than the status, `MaxBodyBytes` was formatted with a separator; the sentence must contain the bare digits `1048576`.

- [ ] **Step 5: Write the failing tests for the origin guard**

Every test here routes through `Middleware` with an **absolute** request target, `http://127.0.0.1:8730/...`, because `httptest.NewRequest` with a path-only target sets `Host` to `example.com` and the `Host` guard then answers 403 before anything else runs. That is the trap named in the header and it is the reason it is named there.

The rejection tests assert on the **log** as well as the response. Contract §2.12 is explicit that a rejection must name what was rejected and what is allowed, because "a silent 403 on a laptop is a bug report that costs an afternoon", and a log line nobody asserts on is a log line that quietly disappears in a later refactor.

Create `control-plane/internal/httpx/origin_test.go`:

```go
package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// policy builds a policy whose log the test can read back. Passing no origins
// exercises the "empty means DefaultOrigins" rule.
func policy(t *testing.T, allowed ...string) (*OriginPolicy, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return NewOriginPolicy(allowed, log), &logs
}

// guard wraps a handler that records whether it was reached. A middleware bug
// that lets a rejected request through is the failure that matters most here,
// and it is invisible unless something downstream says "I ran".
func guard(t *testing.T, p *OriginPolicy, reached *bool) http.Handler {
	t.Helper()
	return p.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		WriteJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
	}))
}

func request(method, origin string) *http.Request {
	req := httptest.NewRequest(method, "http://127.0.0.1:8730/v1/machines", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func TestDefaultOriginsCoverEveryTauriBuild(t *testing.T) {
	// Contract 2.12 derives these five from tauri's own source. If one is
	// dropped, exactly one platform's packaged build stops working and no
	// other test in this repository notices.
	want := []string{
		"http://tauri.localhost",
		"https://tauri.localhost",
		"tauri://localhost",
		"http://localhost:1420",
		"http://127.0.0.1:1420",
	}
	got := DefaultOrigins()
	if len(got) != len(want) {
		t.Fatalf("got %d origins %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("origin %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAllowMatchesTheWholeOriginNotJustTheHost(t *testing.T) {
	p, _ := policy(t)
	cases := []struct {
		origin string
		want   bool
		why    string
	}{
		{"http://tauri.localhost", true, "packaged on Windows"},
		{"https://tauri.localhost", true, "packaged on Windows with useHttpsScheme"},
		{"tauri://localhost", true, "packaged on macOS and Linux"},
		{"http://localhost:1420", true, "the vite dev server"},
		{"http://127.0.0.1:1420", true, "the same dev server by address"},
		{"", true, "curl, the Go test client, anything that is not a browser"},
		{"https://localhost:1420", false, "right host and port, wrong scheme"},
		{"http://localhost:5173", false, "right host, wrong port"},
		{"http://tauri.localhost.evil.example", false, "suffix, not a match"},
		{"null", false, "a sandboxed iframe or a file:// page"},
		{"http://evil.example", false, "the whole point"},
	}
	for _, c := range cases {
		if got := p.Allow(c.origin); got != c.want {
			t.Errorf("Allow(%q) = %v, want %v (%s)", c.origin, got, c.want, c.why)
		}
	}
}

func TestAnExplicitListReplacesTheDefaultsAndIsNormalised(t *testing.T) {
	// Contract section 4: PANACEA_APP_ALLOWED_ORIGINS replaces the defaults, it
	// does not extend them, because "extend" makes lock-down impossible. The
	// surrounding space, trailing slash and capitals are what an operator
	// actually pastes out of a browser address bar.
	p, _ := policy(t, "  HTTP://Tauri.Localhost/  ")

	if !p.Allow("http://tauri.localhost") {
		t.Error("a configured origin was rejected after normalisation")
	}
	if p.Allow("http://localhost:1420") {
		t.Error("an explicit list extended the defaults instead of replacing them")
	}
}

func TestAnAllowlistThatReducesToNothingStaysEmpty(t *testing.T) {
	// Decision (f). An operator who configured something and got nothing back
	// gets a warning and a locked-down server, not five origins they did not
	// ask for silently restored.
	p, logs := policy(t, "   ")

	if got := p.Allowed(); len(got) != 0 {
		t.Errorf("Allowed() = %q, want an empty list", got)
	}
	if p.Allow("http://tauri.localhost") {
		t.Error("a default origin was allowed by a policy configured with none")
	}
	if !p.Allow("") {
		t.Error("a non-browser caller must still be allowed; it sends no Origin to check")
	}
	if !strings.Contains(logs.String(), "empty") {
		t.Errorf("log = %q, want a warning that the allowlist is empty", logs.String())
	}
}
```

Still `control-plane/internal/httpx/origin_test.go`, continuing in the same file:

```go
func TestMiddlewareGrantsAnAllowedOrigin(t *testing.T) {
	p, _ := policy(t)
	var reached bool
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, request(http.MethodGet, "http://localhost:1420"))

	if !reached {
		t.Fatalf("the handler was never reached: %d %s", rec.Code, rec.Body.String())
	}
	if got, want := rec.Header().Get("Access-Control-Allow-Origin"), "http://localhost:1420"; got != want {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q echoed verbatim", got, want)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
		t.Error("the allowlist is the single decision point; a wildcard discards it")
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, want it to contain Origin so no cache serves one origin's response to another", got)
	}
	// No cookies are used anywhere on this API, and this header with an echoed
	// origin would make the allowlist the only thing between a rejected page
	// and an authenticated one.
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want it absent", got)
	}
}

func TestMiddlewareAnswersPreflightBeforeTheMux(t *testing.T) {
	// Without this the mux answers 405 and every POST, PATCH and DELETE from
	// the cockpit fails before its real request is ever sent.
	p, _ := policy(t)
	var reached bool
	req := request(http.MethodOptions, "http://tauri.localhost")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, req)

	if reached {
		t.Error("the preflight reached the mux; it must be answered by the middleware")
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	for header, want := range map[string]string{
		"Access-Control-Allow-Origin":  "http://tauri.localhost",
		"Access-Control-Allow-Methods": "GET, POST, PATCH, DELETE, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type",
		"Access-Control-Max-Age":       "600",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carried %d bytes of body", rec.Body.Len())
	}
}

func TestMiddlewareRejectsAnUnknownOriginAndSaysWhatIsAllowed(t *testing.T) {
	p, logs := policy(t)
	var reached bool
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, request(http.MethodGet, "http://evil.example"))

	if reached {
		t.Fatal("a rejected origin reached the mux")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("a rejected origin was granted Access-Control-Allow-Origin %q", got)
	}
	var problem Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("body %q is not a Problem: %v", rec.Body.String(), err)
	}
	if problem.Error != CodeForbiddenOrigin {
		t.Errorf("error = %q, want %q", problem.Error, CodeForbiddenOrigin)
	}
	if problem.Details["origin"] != "http://evil.example" {
		t.Errorf("details = %v, want the rejected origin", problem.Details)
	}
	// The allowlist goes to the log, not to the body: the body is read by the
	// page that was just rejected.
	if strings.Contains(rec.Body.String(), "tauri.localhost") {
		t.Error("the response body disclosed the allowlist to the rejected caller")
	}
	line := logs.String()
	for _, want := range []string{"http://evil.example", "http://tauri.localhost", "http://localhost:1420"} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q does not name %q", line, want)
		}
	}
}

func TestMiddlewareRejectsANonLoopbackHostButLetsTheBrowserReadWhy(t *testing.T) {
	// DNS rebinding: evil.example resolves to 127.0.0.1, so the connection is
	// local and only the Host header gives it away.
	p, logs := policy(t)
	var reached bool
	req := request(http.MethodGet, "http://tauri.localhost")
	req.Host = "panacea.evil.example"
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, req)

	if reached {
		t.Fatal("a rebound Host reached the mux")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var problem Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("body %q is not a Problem: %v", rec.Body.String(), err)
	}
	if problem.Error != CodeForbiddenHost {
		t.Errorf("error = %q, want %q", problem.Error, CodeForbiddenHost)
	}
	// Decision (e): the origin was fine, so the grant stands and a misconfigured
	// PANACEA_APP_ADDR shows up in the console as this JSON rather than as an
	// opaque CORS failure.
	if got, want := rec.Header().Get("Access-Control-Allow-Origin"), "http://tauri.localhost"; got != want {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, want)
	}
	if !strings.Contains(logs.String(), "panacea.evil.example") {
		t.Errorf("log %q does not name the rejected Host", logs.String())
	}
}

func TestMiddlewareGrantsNothingWhenThereIsNoOrigin(t *testing.T) {
	p, _ := policy(t)
	var reached bool
	rec := httptest.NewRecorder()

	guard(t, p, &reached).ServeHTTP(rec, request(http.MethodGet, ""))

	if !reached {
		t.Fatalf("curl was rejected: %d %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want it absent when there is no origin to echo", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, want Origin even here, or a cache serves this response to an origin", got)
	}
}

func TestLoopbackHost(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8730":              true,
		"127.0.0.1":                   true, // no port, HTTP/1.0 or a hand-built request
		"127.0.0.53:8730":             true, // all of 127.0.0.0/8 is loopback
		"localhost:8730":              true,
		"LocalHost:8730":              true, // Host is case-insensitive
		"localhost.:8730":             true, // the fully-qualified form a resolver may produce
		"[::1]:8730":                  true,
		"[::1]":                       true,
		"::1":                         true,
		"":                            false, // no browser omits it; nothing is lost by refusing
		"evil.example":                false,
		"evil.example:8730":           false,
		"localhost.evil.example:8730": false, // prefix, not a match
		"0.0.0.0:8730":                false,
		"192.168.1.10:8730":           false,
	}
	for host, want := range cases {
		if got := LoopbackHost(host); got != want {
			t.Errorf("LoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
```

- [ ] **Step 6: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/httpx/... -race -count=1`

Expected: a build failure again, but a different one — `problem.go` exists now, so the package has a non-test file and the compiler gets as far as type-checking:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx [github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx.test]
control-plane\internal\httpx\origin_test.go:15:48: undefined: OriginPolicy
control-plane\internal\httpx\origin_test.go:19:9: undefined: NewOriginPolicy
control-plane\internal\httpx\origin_test.go:25:29: undefined: OriginPolicy
control-plane\internal\httpx\origin_test.go:52:9: undefined: DefaultOrigins
control-plane\internal\httpx\origin_test.go:295:13: undefined: LoopbackHost
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx [build failed]
FAIL
```

**Five lines, and every detail of that list is a property of the compiler worth checking.** `OriginPolicy` appears **twice** because `undefined: X` is emitted once per *use site*, not once per symbol, and two helper signatures name the type. There is **no** `too many errors` line, because `gc` truncates at ten and this is five. And there is **no** error for `Allow`, `Middleware` or `Allowed`, even though the tests call all three: bare `undefined: X` is only ever emitted for a package-level identifier, and once a receiver's type is invalid the compiler suppresses the whole cascade of method errors below it. Seeing a bare `undefined: Allow` would mean the test file called it as a function rather than a method.

The line numbers come from one transcription of step 5's two blocks concatenated **with a single blank line between them**, so put one there when you continue the file at `TestMiddlewareGrantsAnAllowedOrigin`. `gofmt` will neither add it for you nor complain if you leave it out: it inserts a blank line before a top-level declaration only when that declaration carries a doc comment, and this one opens with a bare `func`. (`problem.go` and `origin.go` each open their second block with a doc comment, which is why the question does not arise there.) Join with no blank line and the file is still `gofmt`-clean, but every position after the join drops by one and the last line reads `294:13` instead. **Both numbers are correct, for two files that differ by one blank line** — what would be wrong is reading `294:13` as a symbol that failed to resolve. Reflow a comment and they move again. The five names and their order are the assertion, not the numbers. If the package **compiles** at this point, `origin.go` has already been written — delete it and start from the tests, because a test written after the code it checks tests the code that exists rather than the rule that was wanted.

- [ ] **Step 7: Write the origin guard**

This is contract §2.12's second block, plus the plan's adjudicated decision 1 for the CORS grant and decisions (e) and (f) from the header.

Create `control-plane/internal/httpx/origin.go`:

```go
package httpx

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
)

// OriginPolicy decides whether a browser context may talk to this control plane.
//
// WHY NOT coder/websocket's built-in check: AcceptOptions.OriginPatterns matches
// on the Origin's HOST ONLY, ignoring the scheme, and returns nil for a MISSING
// Origin. Both are reasonable defaults for a web server and both are wrong here:
// a loopback control plane's whole threat model is a web page the user happens
// to have open. This policy compares the full origin string and treats an
// unknown origin as a hard 403 with a log line naming what was rejected and what
// is allowed - a silent 403 on a laptop is a bug report that costs an afternoon.
type OriginPolicy struct {
	allowed map[string]struct{}
	order   []string // the same set, in configured order, for the log line
	log     *slog.Logger
}

// DefaultOrigins is what a Tauri webview actually sends.
//
// HOW THIS WAS DETERMINED, because it is the field most likely to be wrong:
// tauri v2 computes the webview's origin in crates/tauri/src/manager/webview.rs.
// For a custom-scheme URL on Windows and Android it produces
// "{http|https}://{scheme}.localhost"; on macOS and Linux it leaves the custom
// scheme intact, giving "tauri://localhost". Which of http and https is chosen
// is WindowConfig.use_https_scheme, which defaults to false, and
// desktop/src-tauri/tauri.conf.json does not set it - so the Windows build sends
// http://tauri.localhost. In development the window loads build.devUrl directly,
// so the origin is http://localhost:1420.
//
// All five are allowed because the same binary serves a Windows dev run, a
// Windows packaged run and a macOS or Linux developer's machine, and an
// allowlist that is right on one of them and silently wrong on the others is
// worse than one that is explicit.
func DefaultOrigins() []string {
	return []string{
		"http://tauri.localhost",  // Tauri 2 packaged, Windows (this repo's default)
		"https://tauri.localhost", // Tauri 2 packaged, Windows with useHttpsScheme
		"tauri://localhost",       // Tauri 2 packaged, macOS and Linux
		"http://localhost:1420",   // vite dev server, tauri.conf.json build.devUrl
		"http://127.0.0.1:1420",   // the same dev server reached by address
	}
}

// NewOriginPolicy returns a policy allowing exactly these origins. An empty
// slice means DefaultOrigins.
//
// A non-empty slice whose entries all normalise away leaves the allowlist EMPTY
// and logs a warning. Falling back to the defaults there would silently restore
// five origins the operator did not ask for, and PANACEA_APP_ALLOWED_ORIGINS
// exists to lock the surface down.
func NewOriginPolicy(allowed []string, log *slog.Logger) *OriginPolicy {
	if log == nil {
		log = slog.Default()
	}
	in := allowed
	if len(in) == 0 {
		in = DefaultOrigins()
	}
	p := &OriginPolicy{allowed: make(map[string]struct{}, len(in)), log: log}
	for _, raw := range in {
		origin := normaliseOrigin(raw)
		if origin == "" {
			continue
		}
		if _, dup := p.allowed[origin]; dup {
			continue
		}
		p.allowed[origin] = struct{}{}
		p.order = append(p.order, origin)
	}
	if len(p.order) == 0 {
		log.Warn("origin allowlist is empty; every browser request will be rejected",
			"configured", len(allowed))
	}
	return p
}

// normaliseOrigin makes comparison total without making it loose. Scheme and
// host are case-insensitive per RFC 6454 and an origin has no path, so lowering
// the whole string cannot change what it means; the trailing slash and the
// surrounding space are what an operator pastes out of an address bar.
func normaliseOrigin(s string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), "/"))
}

// Allow reports whether an Origin header value is acceptable. An EMPTY origin is
// allowed: curl, the Go test client and every non-browser tool send none, and
// the attack this defends against is a browser, which always sends one.
func (p *OriginPolicy) Allow(origin string) bool {
	if origin == "" {
		return true
	}
	_, ok := p.allowed[normaliseOrigin(origin)]
	return ok
}

// Allowed returns the effective allowlist, so task 13 can print it at startup.
func (p *OriginPolicy) Allowed() []string {
	out := make([]string, len(p.order))
	copy(out, p.order)
	return out
}
```

Still `control-plane/internal/httpx/origin.go`, continuing in the same file:

```go
// LoopbackHost reports whether an HTTP Host header names this machine's
// loopback interface.
//
// This is the DNS-rebinding defence and it is the check that survives an attack
// carrying no Origin at all: a page at http://evil.example can point that name
// at 127.0.0.1, and the <img> and <script src> requests it emits then reach this
// process with no Origin header and Host: evil.example. Nothing else sees them.
//
// It takes a Host HEADER, which may carry a port and may be a bracketed IPv6
// literal. Task 13's bind refusal splits PANACEA_APP_ADDR itself before calling
// this.
func LoopbackHost(hostHeader string) bool {
	h := strings.TrimSpace(hostHeader)
	if h == "" {
		// No browser omits Host, so refusing costs nothing legitimate, and
		// "unknown" is not a value this guard may read as loopback.
		return false
	}
	if host, _, err := net.SplitHostPort(h); err == nil {
		h = host
	} else {
		// No port: a bare name or address, or a bracketed IPv6 literal.
		h = strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")
	}
	h = strings.TrimSuffix(h, ".") // the fully-qualified form, "localhost."
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// The preflight answer. Methods is every verb contract 3.1 defines; Headers is
// Content-Type alone, because the desktop sends no custom header and no
// Authorization on this hop; 600 is ten minutes of not asking again, which is
// the only thing a cached preflight is for.
const (
	corsMethods = "GET, POST, PATCH, DELETE, OPTIONS"
	corsHeaders = "Content-Type"
	corsMaxAge  = "600"
)

// Middleware rejects a request whose Host is not loopback with 403
// forbidden_host, and one whose Origin is present and not allowed with 403
// forbidden_origin - and GRANTS CORS to everything else, which contract 2.12
// omits and the plan's adjudicated decision 1 adds. Without the grant the
// cockpit cannot make one REST call: every Tauri build is cross-origin against
// http://127.0.0.1:8730, so a GET fails at the response with no
// Access-Control-Allow-Origin header, and every JSON POST fails one step earlier
// at a preflight the mux does not route. WebSockets are exempt from all of it,
// which is exactly why Phase 1 worked with none of this.
func (p *OriginPolicy) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := p.Allow(origin)

		// Vary unconditionally. The response differs by Origin even when this
		// particular request carried none, and a cache that does not know that
		// will serve one origin's response to another.
		w.Header().Add("Vary", "Origin")
		if origin != "" && allowed {
			// Echoed verbatim, never "*", so the allowlist stays the single
			// decision point. No Access-Control-Allow-Credentials: no cookies
			// are used, and that header beside an echoed origin is what turns
			// an allowlist slip into an authenticated cross-origin call.
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		if !LoopbackHost(r.Host) {
			p.log.Warn("rejected a request whose Host is not loopback",
				"host", r.Host, "method", r.Method, "path", r.URL.Path,
				"remote", r.RemoteAddr)
			WriteProblem(w, http.StatusForbidden, CodeForbiddenHost,
				"This control plane only answers requests addressed to localhost.",
				map[string]string{"host": r.Host})
			return
		}

		if !allowed {
			// The allowlist goes in the log, not in the body: the body is read
			// by the page that was just rejected.
			p.log.Warn("rejected a request from an origin that is not allowed",
				"origin", origin, "allowed", strings.Join(p.order, ", "),
				"method", r.Method, "path", r.URL.Path)
			WriteProblem(w, http.StatusForbidden, CodeForbiddenOrigin,
				"This origin is not allowed to call the Panacea App control plane. "+
					"The control plane log names the origins that are.",
				map[string]string{"origin": origin})
			return
		}

		if r.Method == http.MethodOptions && origin != "" {
			w.Header().Set("Access-Control-Allow-Methods", corsMethods)
			w.Header().Set("Access-Control-Allow-Headers", corsHeaders)
			w.Header().Set("Access-Control-Max-Age", corsMaxAge)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
```

Three things about that order, since all three are decisions and none is in the contract. **The grant is computed first and the `Host` check second**, so decision (e) works: a request from an allowed origin to a rebound `Host` still carries `Access-Control-Allow-Origin` and the browser can therefore read the `forbidden_host` JSON instead of reporting an opaque CORS failure. **The `Host` check runs before the origin check**, because it is the one that does not depend on a header the attacker chooses, and a rebinding attempt should be diagnosed as `forbidden_host` rather than as whatever `Origin` the attacking page happened to send. **An `OPTIONS` with no `Origin` falls through to the mux** rather than being answered 204: it is not a preflight — a real preflight always carries `Origin` — and answering it here would hide a genuine routing question behind a middleware. An `OPTIONS` that *does* carry an allowed origin is answered even without `Access-Control-Request-Method`, because the response is identical either way and the extra branch buys nothing.

One note for task 12, which mounts `/v1/sessions/{id}/attach` behind this same middleware: the `Vary` and `Access-Control-Allow-Origin` headers set here are still on the `ResponseWriter` when `websocket.Accept` writes its `101`, so the upgrade response carries two headers a WebSocket handshake does not define. Browsers ignore them and the handshake is unaffected. Removing them would mean special-casing a path inside this middleware, which is worse.

- [ ] **Step 8: Run the whole package to verify every test passes**

Run: `go test ./control-plane/internal/httpx/... -race -count=1`

Expected one `ok` line and no `FAIL`. This is the first run in which both files' tests are green together:

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx	5.459s
```

Then run the origin tests alone with verbose output once, because three of them assert on a log the package writes — `TestAnAllowlistThatReducesToNothingStaysEmpty`, `TestMiddlewareRejectsAnUnknownOriginAndSaysWhatIsAllowed` and `TestMiddlewareRejectsANonLoopbackHostButLetsTheBrowserReadWhy` — and a silent pass is exactly the thing this package exists to prevent:

```bash
go test ./control-plane/internal/httpx/... -race -count=1 -run 'Origin|Middleware|Loopback|Allow|Explicit' -v
```

`Explicit` is in that filter for exactly one test. `TestAnExplicitListReplacesTheDefaultsAndIsNormalised` contains none of `Origin`, `Middleware`, `Loopback` or `Allow`, so without it nine of the ten origin tests run and the tenth is skipped in silence — which is the same class of miss as the log line nobody asserts on. The ten `--- PASS` lines are the assertion; the durations are not:

```
=== RUN   TestDefaultOriginsCoverEveryTauriBuild
--- PASS: TestDefaultOriginsCoverEveryTauriBuild (0.00s)
=== RUN   TestAllowMatchesTheWholeOriginNotJustTheHost
--- PASS: TestAllowMatchesTheWholeOriginNotJustTheHost (0.00s)
=== RUN   TestAnExplicitListReplacesTheDefaultsAndIsNormalised
--- PASS: TestAnExplicitListReplacesTheDefaultsAndIsNormalised (0.00s)
=== RUN   TestAnAllowlistThatReducesToNothingStaysEmpty
--- PASS: TestAnAllowlistThatReducesToNothingStaysEmpty (0.00s)
=== RUN   TestMiddlewareGrantsAnAllowedOrigin
--- PASS: TestMiddlewareGrantsAnAllowedOrigin (0.00s)
=== RUN   TestMiddlewareAnswersPreflightBeforeTheMux
--- PASS: TestMiddlewareAnswersPreflightBeforeTheMux (0.00s)
=== RUN   TestMiddlewareRejectsAnUnknownOriginAndSaysWhatIsAllowed
--- PASS: TestMiddlewareRejectsAnUnknownOriginAndSaysWhatIsAllowed (0.00s)
=== RUN   TestMiddlewareRejectsANonLoopbackHostButLetsTheBrowserReadWhy
--- PASS: TestMiddlewareRejectsANonLoopbackHostButLetsTheBrowserReadWhy (0.00s)
=== RUN   TestMiddlewareGrantsNothingWhenThereIsNoOrigin
--- PASS: TestMiddlewareGrantsNothingWhenThereIsNoOrigin (0.00s)
=== RUN   TestLoopbackHost
--- PASS: TestLoopbackHost (0.00s)
PASS
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx	1.733s
```

Note what is **not** there: no `slog` text. Every policy these tests build logs into a `bytes.Buffer`, so a rejection's log line only ever surfaces inside an assertion failure message. Read one by temporarily adding `t.Log(logs.String())` if you want to see its shape; do not leave it in.

Three failures are worth predicting because each means the implementation drifted in a specific way rather than being broken generally. `TestMiddlewareAnswersPreflightBeforeTheMux` failing with status 200 and `reached` true means the `OPTIONS` branch was placed after `next.ServeHTTP` or guarded on `Access-Control-Request-Method`. `TestMiddlewareRejectsANonLoopbackHostButLetsTheBrowserReadWhy` failing on the `Access-Control-Allow-Origin` assertion means the `Host` check was hoisted above the grant — that is decision (e) and the test is the only thing holding it. `TestLoopbackHost` failing on `"::1"` alone means the bracket trim was skipped: `net.SplitHostPort("::1")` errors with `too many colons in address`, so an implementation that trusts `SplitHostPort` and gives up on error never parses the unbracketed IPv6 form at all.

- [ ] **Step 9: Vet, format and run the full Go suite**

There is no end-to-end check available in this task and it is worth being explicit about that rather than implying one: nothing in the repository serves HTTP over this middleware until task 10 mounts it in `api.New`, so no `curl` and no browser can exercise the grant yet. The first real proof that CORS works is task 24's first REST call from the cockpit. The tests above are the whole of this task's evidence.

Run all three, from the repository root:

```bash
gofmt -l control-plane/internal/httpx
go vet ./control-plane/internal/httpx/...
go test ./sandboxd/... ./control-plane/... -race -count=1
```

`gofmt -l` prints the name of every file it would reformat, so **no output is the pass**. One likely hit: the `TestLoopbackHost` map literal in `origin_test.go` has keys of differing width and `gofmt` aligns the values into a column — if it names the file, run `gofmt -w control-plane/internal/httpx` and move on; the alignment is the only thing that changes. `go vet` also prints nothing on success, and the one construct it would reasonably complain about is the `fmt.Sprintf` chain in `decodeMessage`, where `%s` is applied to a `*json.SyntaxError` and to a `reflect.Type`; both satisfy `Stringer` or `error`, so both are correct and vet is silent.

For the suite, the acceptance criterion is that every package reports `ok` or `[no test files]` and no line says `FAIL` — not the list below, which by task 9 covers everything tasks 1 to 8 added as well, printed in import-path order. This is the one line this task is responsible for:

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx	5.459s
```

`go test ./...` at the workspace root is invalid and fails with `directory prefix . does not contain modules listed in go.work or their selected dependencies`. Use the two explicit module paths.

- [ ] **Step 10: Commit**

```bash
git add control-plane/internal/httpx/problem.go \
        control-plane/internal/httpx/problem_test.go \
        control-plane/internal/httpx/origin.go \
        control-plane/internal/httpx/origin_test.go
git commit -m "feat(httpx): one error body, and an origin guard that grants as well as rejects

The API answers every failure with the same three-field object and a code
from a closed set, because the desktop has one error branch and a handler
that invents its own shape produces a branch the cockpit silently never
matches. The codes are constants for the same reason: a mistyped literal is
not a compile error anywhere. There are sixteen, not the contract's fifteen:
non_loopback_publish is added here per Group C's shared-package ruling 7,
because a sandbox published off loopback is running and sandbox_not_running
would be a lie.

The origin guard is the whole security boundary of a control plane with no
authentication. It compares the full origin string rather than the host, so
https://localhost:1420 is not http://localhost:1420, and it requires Host to
name loopback, which is the only check that catches a DNS-rebinding request
carrying no Origin at all. Every rejection logs what was rejected and what is
allowed, because a silent 403 on a laptop costs an afternoon.

It also grants. A Tauri webview is a browser and every build is cross-origin
against the loopback port, so without an echoed Access-Control-Allow-Origin
and a preflight answered before the mux, the cockpit cannot make a single
REST call. The origin is echoed verbatim and never wildcarded, so the
allowlist stays the one decision point, and no credentials header is sent
because no cookies are used.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

