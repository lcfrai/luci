### Task 4: LiteLLM admin client on its own deadlines

Slice B, 2.5 focused hours, depends on task 03. No seams.

keysmith is the only process besides LiteLLM that holds the master key (P6-D1), and this package is the only code in keysmith that uses it. It speaks LiteLLM's admin API, typed: generate a key, delete keys, read a key's spend, make sure a person's and a project's budgets exist, and ask whether LiteLLM is ready. Tasks 07 and 09 call it; nothing else does.

**The one thing this task must not get wrong is the Panacea deadline chain.** In Panacea, `/key/generate` ran on the caller's context (`~/Projects/Panacea/services/panacea-api/handlers/llm_env_keys.go:120`, `http.NewRequestWithContext(ctx, …)`), inside a request's shared 15-second budget (`handlers/projects.go:1891`, `:2622`, each a `context.WithTimeout(r.Context(), 15*time.Second)`, verified 2026-09-25), so a slow mint drained the time of the PipelineRun create that followed it. Here every call runs on `context.WithTimeout(context.WithoutCancel(ctx), Timeout)`: its own deadline, never the caller's, and not cancelled when the caller gives up. A caller that stops waiting (task 08's `504 mint_timeout`) leaves the mint to finish, and task 07's retry of the same generation then finds it.

Three more rules are enforced here, not described:

1. **No key without a master key.** `New` reads the key file once, trims it, and refuses an empty file or one that does not start with `sk-` (LiteLLM's documented master-key prefix, **UNVERIFIED** against the pinned image until task 18). In Panacea the key came from a `secretKeyRef` with `optional: true` (`~/Projects/Panacea/deploy/portal/deploy-portal.yaml:779-784`, verified 2026-09-25), so a missing Secret made an empty key and a pod that still reported ready.
2. **An error never carries a response body.** Panacea's error was `fmt.Errorf("LiteLLM key/generate (env) returned %d: %s", resp.StatusCode, string(respBody))` (`llm_env_keys.go:133`, verified 2026-09-25), and a LiteLLM error body can echo the request, which holds the key being minted. Here an error carries the operation, the status and LiteLLM's `error.type` if it is a short lowercase word, and nothing else.
3. **No call that spends.** `Health` asks `GET /health/readiness`, never `GET /health`, which in LiteLLM calls every configured upstream model (training knowledge, **UNVERIFIED**; if true, a readiness probe every few seconds would be real Anthropic and Bedrock spend). Task 17's compose health check already uses `/health/readiness`.

Four decisions the skeleton leaves open, made here so no later part re-derives them:

- **(a) The identifiers task 07 already wrote against are adopted as given** (`task-07.md`, gap 4): `GenerateKeyRequest{Key, KeyAlias, Duration, UserID, TeamID, Models []string, MaxBudget float64, Metadata map[string]string}`, `GenerateKeyResponse{Key string}`, `DeleteKeys(ctx, []string) error`, `EnsureUser` and `EnsureTeam(ctx, id string, maxBudget float64, budgetDuration string) error`. `GenerateKey` returns `(GenerateKeyResponse, error)`, `KeyInfo(ctx, keyID)` returns `(KeyInfo, error)` and `Health(ctx)` returns `error`.
- **(b) `Ensure` means create, then update.** `EnsureUser` posts `/user/new`; on any non-2xx it posts `/user/update` with the same body, and returns the first call's error only if the second also fails. This converges whether LiteLLM answers an existing user with `400`, `409` or `500` (which one is **UNVERIFIED**), and it applies a changed budget (a new Hub policy, S6.2) to an existing user. `EnsureTeam` does the same with `/team/new` and `/team/update`.
- **(c) Keys are addressed by id, never by value, after the mint.** `KeyInfo` and `DeleteKeys` take task 07's `KeyID` (`wire.HashToken(token)`, task 07 decision (f)), so the raw token is sent to LiteLLM exactly once, in the `/key/generate` body, and never in a URL, where an access log would keep it. That LiteLLM accepts the hash on `/key/info` and `/key/delete` is task 18's P11 and P12.
- **(d) Redirects are errors.** An admin call that answers `3xx` is refused, not followed, so the master key is never sent to a second URL.

**Files:**
- `control-plane/internal/litellmadmin/client.go` — `Options`, `Client`, `New`, `ErrNoMasterKey`, `Error`, the request and response types, the six methods (new)
- `control-plane/internal/litellmadmin/client_test.go` — against an `httptest` fake of LiteLLM (new)

**Interfaces:**
- Produces:
  - `litellmadmin.New(baseURL, masterKeyFile string, o Options) (*Client, error)`; `Options{Timeout time.Duration; HTTP *http.Client}`, zero `Timeout` = `DefaultTimeout` (20 s, the skeleton's `MINT_TIMEOUT`), nil `HTTP` = a client that refuses redirects
  - `litellmadmin.ErrNoMasterKey`; `*litellmadmin.Error{Op string; Status int; Type string; Timeout bool}` with `Unwrap`
  - `GenerateKeyRequest`, `GenerateKeyResponse{Key}`, `KeyInfo{Spend float64; MaxBudget *float64; KeyAlias, UserID, TeamID, Expires string; Metadata map[string]any}`
  - `(*Client) GenerateKey`, `DeleteKeys`, `KeyInfo`, `EnsureUser`, `EnsureTeam`, `Health`
- Consumes: `wire.CheckBaseURL` (task 03) for the base URL; the standard library.
- Required by: task 07 (`keysmith.AdminAPI` is four of these methods), task 08 (`New` at start-up; `Health` in `/readyz`), task 09 (`KeyInfo` in the sweep).

**LiteLLM's admin API as this client calls it.** Paths and field names are LiteLLM's documented proxy API as the skeleton and task 18 give them (`task-18.md`, "LiteLLM's admin API as this script calls it"), **not re-read for this part**, and so **UNVERIFIED** until task 18 runs against the pinned digest:

| Method | Call | Body sent | Read back |
|---|---|---|---|
| `GenerateKey` | `POST /key/generate` | `key`, `key_alias`, `duration`, `user_id`, `team_id`, `models`, `max_budget`, `metadata` | `key` |
| `DeleteKeys` | `POST /key/delete` | `keys` | status |
| `KeyInfo` | `GET /key/info?key=<keyID>` | none | `info.spend`, `info.max_budget`, `info.key_alias`, `info.user_id`, `info.team_id`, `info.expires`, `info.metadata` |
| `EnsureUser` | `POST /user/new`, then `POST /user/update` | `user_id`, `max_budget`, `budget_duration` | status |
| `EnsureTeam` | `POST /team/new`, then `POST /team/update` | `team_id`, `max_budget`, `budget_duration` | status |
| `Health` | `GET /health/readiness` | none | status |

- [ ] **Step 1: Write the failing tests**

Every test runs the client against `fakeLiteLLM`, an `httptest.Server` that records each request's method, path, query, `Authorization` and body, and answers from a per-path table. Each test names the mutant it kills.

Create `control-plane/internal/litellmadmin/client_test.go`:

```go
package litellmadmin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const master = "sk-master-test"

type seen struct{ method, path, query, auth, body string }

type reply struct {
	status int
	body   string
	delay  time.Duration
}

type fakeLiteLLM struct {
	*httptest.Server
	mu      sync.Mutex
	got     []seen
	replies map[string]reply // by path; absent = 200 {}
}

func newFake(t *testing.T, replies map[string]reply) *fakeLiteLLM {
	t.Helper()
	f := &fakeLiteLLM{replies: replies}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.got = append(f.got, seen{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), string(b)})
		rep, ok := f.replies[r.URL.Path]
		f.mu.Unlock()
		if !ok {
			rep = reply{status: 200, body: `{}`}
		}
		select {
		case <-time.After(rep.delay):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(rep.status)
		io.WriteString(w, rep.body)
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeLiteLLM) calls() []seen {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]seen(nil), f.got...)
}

func keyFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "master_key")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func client(t *testing.T, f *fakeLiteLLM, o Options) *Client {
	t.Helper()
	c, err := New(f.URL, keyFile(t, master+"\n"), o)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// Kills: Panacea's optional:true behaviour (an empty key that still starts),
// and an error that quotes the file's content.
func TestNewRefusesAnUnusableMasterKey(t *testing.T) {
	for name, content := range map[string]string{"empty": "", "blank": " \n\t", "not an sk- key": "hunter2-not-a-key", "prefix only": "sk-"} {
		_, err := New("http://litellm:4001", keyFile(t, content), Options{})
		if !errors.Is(err, ErrNoMasterKey) {
			t.Errorf("%s: err = %v, want ErrNoMasterKey", name, err)
		}
		if err != nil && strings.Contains(err.Error(), "hunter2") {
			t.Errorf("%s: the error quotes the file: %v", name, err)
		}
	}
	if _, err := New("http://litellm:4001", filepath.Join(t.TempDir(), "absent"), Options{}); err == nil {
		t.Error("a missing key file was accepted")
	}
	if _, err := New("http://litellm:4001/v1", keyFile(t, master), Options{}); err == nil {
		t.Error("a base URL with a path was accepted")
	}
}
```

Then what each call sends, in the same file:

```go
var genReq = GenerateKeyRequest{Key: "sk-pa-tok", KeyAlias: "sess/s-1/1", Duration: "1h", UserID: "t-1:o-1",
	TeamID: "p-1", Models: []string{"au/claude-opus-4-8", "global/claude-opus-4-8"}, MaxBudget: 25,
	Metadata: map[string]string{"session": "s-1", "route": "global"}}

// Kills: a renamed LiteLLM field (the call "succeeds" and sets nothing), a
// method on the wrong path, a missing or wrong master key on any call, and the
// raw token in a URL (decision (c): KeyInfo sends the id).
func TestEveryCallSendsTheMasterKeyAndItsGoldenBody(t *testing.T) {
	f := newFake(t, map[string]reply{"/key/generate": {status: 200, body: `{"key":"sk-pa-tok"}`}})
	c := client(t, f, Options{})
	ctx := context.Background()
	if _, err := c.GenerateKey(ctx, genReq); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(c.DeleteKeys(ctx, []string{"h-1", "h-2"}))
	_, err := c.KeyInfo(ctx, "h-1")
	must(err)
	must(c.EnsureUser(ctx, "t-1:o-1", 200, "1mo"))
	must(c.EnsureTeam(ctx, "p-1", 1000, "1mo"))
	must(c.Health(ctx))
	want := []seen{
		{"POST", "/key/generate", "", "", `{"key":"sk-pa-tok","key_alias":"sess/s-1/1","duration":"1h","user_id":"t-1:o-1","team_id":"p-1","models":["au/claude-opus-4-8","global/claude-opus-4-8"],"max_budget":25,"metadata":{"route":"global","session":"s-1"}}`},
		{"POST", "/key/delete", "", "", `{"keys":["h-1","h-2"]}`},
		{"GET", "/key/info", "key=h-1", "", ``},
		{"POST", "/user/new", "", "", `{"user_id":"t-1:o-1","max_budget":200,"budget_duration":"1mo"}`},
		{"POST", "/team/new", "", "", `{"team_id":"p-1","max_budget":1000,"budget_duration":"1mo"}`},
		{"GET", "/health/readiness", "", "", ``},
	}
	got := f.calls()
	if len(got) != len(want) {
		t.Fatalf("%d calls, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		if g.auth != "Bearer "+master {
			t.Errorf("%s %s: Authorization %q", g.method, g.path, g.auth)
		}
		if g.method != w.method || g.path != w.path || g.query != w.query || g.body != w.body {
			t.Errorf("call %d:\n got %s %s?%s %s\nwant %s %s?%s %s", i, g.method, g.path, g.query, g.body, w.method, w.path, w.query, w.body)
		}
		if g.body != "" && !json.Valid([]byte(g.body)) {
			t.Errorf("%s: body is not JSON", g.path)
		}
	}
}

// Kills: Ensure that stops at /user/new when the user already exists (the
// budget from a new policy would never be applied), and one that always calls
// update (a first mint would find no user).
func TestEnsureCreatesThenUpdates(t *testing.T) {
	f := newFake(t, map[string]reply{"/user/new": {status: 400, body: `{"error":{"type":"bad_request_error"}}`}})
	c := client(t, f, Options{})
	if err := c.EnsureUser(context.Background(), "t-1:o-1", 200, "1mo"); err != nil {
		t.Fatalf("EnsureUser: %v", err)
	}
	if got := f.calls(); len(got) != 2 || got[0].path != "/user/new" || got[1].path != "/user/update" || got[1].body != got[0].body {
		t.Fatalf("calls %+v, want /user/new then /user/update with the same body", got)
	}
	g := newFake(t, nil)
	if err := client(t, g, Options{}).EnsureTeam(context.Background(), "p-1", 1000, "1mo"); err != nil || len(g.calls()) != 1 {
		t.Fatalf("a created team made %d calls (%v), want 1", len(g.calls()), err)
	}
	h := newFake(t, map[string]reply{"/team/new": {status: 500}, "/team/update": {status: 404}})
	var e *Error
	if err := client(t, h, Options{}).EnsureTeam(context.Background(), "p-1", 1000, "1mo"); !errors.As(err, &e) || e.Op != "team/new" || e.Status != 500 {
		t.Fatalf("both failing: err = %v, want the team/new 500", err)
	}
}

// Kills: a DeleteKeys of nothing that posts an empty list (LiteLLM's reading
// of an empty list is not ours to guess).
func TestDeleteNothingMakesNoCall(t *testing.T) {
	f := newFake(t, nil)
	if err := client(t, f, Options{}).DeleteKeys(context.Background(), nil); err != nil || len(f.calls()) != 0 {
		t.Fatalf("err %v, %d calls", err, len(f.calls()))
	}
}
```

And the deadlines, the errors and the replies:

```go
// Kills: running the call on the caller's context. The caller gives up after
// 1 ms; LiteLLM answers at 50 ms; the mint must still land (task 08's 504 path).
func TestACallerThatGivesUpDoesNotAbortTheCall(t *testing.T) {
	f := newFake(t, map[string]reply{"/key/generate": {status: 200, body: `{"key":"sk-pa-tok"}`, delay: 50 * time.Millisecond}})
	c := client(t, f, Options{Timeout: 5 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	resp, err := c.GenerateKey(ctx, genReq)
	if err != nil || resp.Key != "sk-pa-tok" {
		t.Fatalf("GenerateKey = %+v, %v; want the key despite the caller's 1 ms", resp, err)
	}
}

// Kills: no deadline at all (a hung LiteLLM would hang keysmith with it).
func TestAHungServerFailsAfterTheTimeout(t *testing.T) {
	f := newFake(t, map[string]reply{"/key/generate": {status: 200, body: `{}`, delay: time.Hour}})
	c := client(t, f, Options{Timeout: 100 * time.Millisecond})
	start := time.Now()
	_, err := c.GenerateKey(context.Background(), genReq)
	var e *Error
	if !errors.As(err, &e) || !e.Timeout || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want a timeout *Error wrapping context.DeadlineExceeded", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("took %v with a 100 ms timeout", d)
	}
}

// Kills: Panacea's `returned %d: %s` (llm_env_keys.go:133). The body echoes
// both the minted key and the master key; the error may hold neither.
func TestAnErrorCarriesNeitherTheBodyNorAKey(t *testing.T) {
	echo := `{"error":{"message":"bad key sk-pa-tok, master sk-master-test","type":"internal_error"}}`
	f := newFake(t, map[string]reply{"/key/generate": {status: 500, body: echo}})
	_, err := client(t, f, Options{}).GenerateKey(context.Background(), genReq)
	var e *Error
	if !errors.As(err, &e) || e.Status != 500 || e.Type != "internal_error" || e.Op != "key/generate" {
		t.Fatalf("err = %#v", err)
	}
	for _, leak := range []string{"sk-pa-tok", master, "bad key"} {
		if strings.Contains(err.Error(), leak) {
			t.Errorf("error %q contains %q", err.Error(), leak)
		}
	}
	g := newFake(t, map[string]reply{"/key/generate": {status: 500, body: `{"error":{"type":"sk-pa-tok leaked here"}}`}})
	_, err = client(t, g, Options{}).GenerateKey(context.Background(), genReq)
	if errors.As(err, &e); strings.Contains(err.Error(), "sk-pa-tok") || e.Type != "" {
		t.Errorf("an error.type that is not a short word was kept: %q", err.Error())
	}
}

// Kills: following a redirect, which would send the master key to a second URL.
func TestARedirectIsAnErrorAndNotFollowed(t *testing.T) {
	var elsewhere int
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { elsewhere++ }))
	defer other.Close()
	red := httptest.NewServer(http.RedirectHandler(other.URL+"/key/generate", http.StatusTemporaryRedirect))
	defer red.Close()
	c, err := New(red.URL, keyFile(t, master), Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.GenerateKey(context.Background(), genReq)
	var e *Error
	if !errors.As(err, &e) || e.Status != http.StatusTemporaryRedirect || elsewhere != 0 {
		t.Fatalf("err = %v, elsewhere hit %d times", err, elsewhere)
	}
}

// Kills: a 200 with no key treated as a mint (keysmith would bind an empty
// token), and a KeyInfo that fails on LiteLLM's extra fields.
func TestReplies(t *testing.T) {
	f := newFake(t, map[string]reply{
		"/key/generate": {status: 200, body: `{"token_id":"x"}`},
		"/key/info": {status: 200, body: `{"key":"h-1","info":{"spend":1.25,"max_budget":25,"key_alias":"sess/s-1/1",` +
			`"user_id":"t-1:o-1","team_id":"p-1","expires":"2026-09-25T02:00:00","metadata":{"session":"s-1","n":3},"extra":true}}`},
	})
	c := client(t, f, Options{})
	if _, err := c.GenerateKey(context.Background(), genReq); err == nil {
		t.Error("a reply without a key was accepted")
	}
	info, err := c.KeyInfo(context.Background(), "h-1")
	if err != nil || info.Spend != 1.25 || info.MaxBudget == nil || *info.MaxBudget != 25 ||
		info.KeyAlias != "sess/s-1/1" || info.TeamID != "p-1" || info.Metadata["session"] != "s-1" {
		t.Fatalf("KeyInfo = %+v, %v", info, err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/litellmadmin/... -count=1`

Expected: `[build failed]` with `undefined: GenerateKeyRequest`, `undefined: Options`, `undefined: Client`, `undefined: New` and `undefined: ErrNoMasterKey`.

- [ ] **Step 3: Write the client**

Create `control-plane/internal/litellmadmin/client.go`. It is written here in three fenced blocks, one file.

```go
// Package litellmadmin is LiteLLM's admin API, typed, for keysmith only.
//
// Every call runs on its own deadline, detached from the caller's
// cancellation: a caller that stops waiting does not abort a mint half-way,
// and no caller's deadline is ever spent on LiteLLM (the Panacea lesson). An
// error carries the operation, the status and LiteLLM's error type, never a
// response body, which can echo the key being minted.
package litellmadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire"
)

// DefaultTimeout is each call's own deadline (PANACEA_APP_GATEWAY_MINT_TIMEOUT).
const DefaultTimeout = 20 * time.Second

const maxReply = 1 << 20 // a larger reply is an error, not a key

// ErrNoMasterKey: the key file is empty, blank, or not an "sk-" key.
var ErrNoMasterKey = errors.New("litellmadmin: the master key file is empty or does not hold an sk- key")

// Options tunes a Client. A caller-supplied HTTP client is copied and made to
// refuse redirects, so the master key is never sent to a second URL.
type Options struct {
	Timeout time.Duration
	HTTP    *http.Client
}

// Client holds the master key. It is safe for concurrent use.
type Client struct {
	base    url.URL
	auth    string // "Bearer <master key>"; never printed, never in an error
	timeout time.Duration
	http    *http.Client
}

func refuseRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// New reads the master key file once. It fails on an unusable base URL, an
// unreadable file, or ErrNoMasterKey; the error names the file, never its content.
func New(baseURL, masterKeyFile string, o Options) (*Client, error) {
	if err := wire.CheckBaseURL(baseURL); err != nil {
		return nil, fmt.Errorf("litellmadmin: %w", err)
	}
	u, _ := url.Parse(baseURL)
	u.Path = ""
	raw, err := os.ReadFile(masterKeyFile)
	if err != nil {
		return nil, fmt.Errorf("litellmadmin: master key file: %w", err)
	}
	key := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(key, "sk-") || key == "sk-" || strings.ContainsAny(key, " \t\r\n") {
		return nil, fmt.Errorf("%w (%s)", ErrNoMasterKey, masterKeyFile)
	}
	if o.Timeout <= 0 {
		o.Timeout = DefaultTimeout
	}
	h := &http.Client{}
	if o.HTTP != nil {
		copied := *o.HTTP
		h = &copied
	}
	h.CheckRedirect = refuseRedirect
	return &Client{base: *u, auth: "Bearer " + key, timeout: o.Timeout, http: h}, nil
}

// Error is every failure this package returns. Type is LiteLLM's error.type
// when it is a short lowercase word, or one of this package's own two
// ("reply_unreadable", "reply_without_key"); otherwise empty.
type Error struct {
	Op      string // "key/generate", "user/new", …
	Status  int    // 0 when no reply arrived
	Type    string
	Timeout bool // the call's own deadline passed
	cause   error
}

func (e *Error) Error() string {
	switch {
	case e.Status != 0 && e.Type != "":
		return fmt.Sprintf("litellm %s: status %d (%s)", e.Op, e.Status, e.Type)
	case e.Status != 0:
		return fmt.Sprintf("litellm %s: status %d", e.Op, e.Status)
	case e.Timeout:
		return fmt.Sprintf("litellm %s: no reply within the deadline", e.Op)
	}
	return fmt.Sprintf("litellm %s: transport failure", e.Op)
}

// Unwrap exposes the transport cause (for errors.Is(err, context.DeadlineExceeded));
// Error never prints it.
func (e *Error) Unwrap() error { return e.cause }
```

```go
var errorTypeRe = regexp.MustCompile(`^[a-z_]{1,64}$`)

// errorType keeps LiteLLM's error.type only if it is a short lowercase word,
// so a type field cannot smuggle an echoed key into an error string.
func errorType(raw []byte) string {
	var e struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal(raw, &e) != nil || !errorTypeRe.MatchString(e.Error.Type) {
		return ""
	}
	return e.Error.Type
}

// do is the one place a request is made. It owns the deadline: the caller's
// context contributes its values, never its cancellation or its deadline.
func (c *Client) do(ctx context.Context, op, method string, query url.Values, in, out any) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.timeout)
	defer cancel()
	u := c.base
	u.Path = "/" + op
	u.RawQuery = query.Encode()
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return &Error{Op: op, cause: err}
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return &Error{Op: op, cause: err}
	}
	req.Header.Set("Authorization", c.auth)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return &Error{Op: op, Timeout: errors.Is(err, context.DeadlineExceeded), cause: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxReply+1))
	if err != nil || len(raw) > maxReply {
		return &Error{Op: op, Status: resp.StatusCode, Type: "reply_unreadable",
			Timeout: errors.Is(err, context.DeadlineExceeded), cause: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &Error{Op: op, Status: resp.StatusCode, Type: errorType(raw)}
	}
	if out != nil && json.Unmarshal(raw, out) != nil {
		return &Error{Op: op, Status: resp.StatusCode, Type: "reply_unreadable"}
	}
	return nil
}
```

```go
// GenerateKeyRequest is POST /key/generate. Key is keysmith's own token; if
// the pinned LiteLLM refuses a caller-chosen key (task 18, P01), task 07's
// fallback leaves it empty and reads the generated one from the reply.
type GenerateKeyRequest struct {
	Key       string            `json:"key,omitempty"`
	KeyAlias  string            `json:"key_alias"`
	Duration  string            `json:"duration"` // "1h", LiteLLM's duration syntax
	UserID    string            `json:"user_id"`  // wire.Owner.UserID()
	TeamID    string            `json:"team_id"`  // the PROJECT (P6-D6)
	Models    []string          `json:"models"`
	MaxBudget float64           `json:"max_budget"`
	Metadata  map[string]string `json:"metadata"`
}

// GenerateKeyResponse keeps only the key; LiteLLM's other fields are ignored.
type GenerateKeyResponse struct {
	Key string `json:"key"`
}

// KeyInfo is the "info" object of GET /key/info. Expires stays text: LiteLLM's
// timestamp format (with or without a zone) is UNVERIFIED.
type KeyInfo struct {
	Spend     float64        `json:"spend"`
	MaxBudget *float64       `json:"max_budget"`
	KeyAlias  string         `json:"key_alias"`
	UserID    string         `json:"user_id"`
	TeamID    string         `json:"team_id"`
	Expires   string         `json:"expires"`
	Metadata  map[string]any `json:"metadata"`
}

type budgetBody struct {
	UserID         string  `json:"user_id,omitempty"`
	TeamID         string  `json:"team_id,omitempty"`
	MaxBudget      float64 `json:"max_budget"`
	BudgetDuration string  `json:"budget_duration"`
}

// GenerateKey mints one virtual key. A 2xx without a key is an error.
func (c *Client) GenerateKey(ctx context.Context, r GenerateKeyRequest) (GenerateKeyResponse, error) {
	var out GenerateKeyResponse
	if err := c.do(ctx, "key/generate", http.MethodPost, nil, r, &out); err != nil {
		return GenerateKeyResponse{}, err
	}
	if out.Key == "" {
		return GenerateKeyResponse{}, &Error{Op: "key/generate", Status: http.StatusOK, Type: "reply_without_key"}
	}
	return out, nil
}

// DeleteKeys deletes keys by id (wire.HashToken of each token). None: no call.
func (c *Client) DeleteKeys(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.do(ctx, "key/delete", http.MethodPost, nil, struct {
		Keys []string `json:"keys"`
	}{keys}, nil)
}

// KeyInfo reads one key's spend and limits, by id, never by value.
func (c *Client) KeyInfo(ctx context.Context, keyID string) (KeyInfo, error) {
	var env struct {
		Info KeyInfo `json:"info"`
	}
	if err := c.do(ctx, "key/info", http.MethodGet, url.Values{"key": {keyID}}, nil, &env); err != nil {
		return KeyInfo{}, err
	}
	return env.Info, nil
}

// EnsureUser makes the person's LiteLLM user exist with this budget.
func (c *Client) EnsureUser(ctx context.Context, id string, maxBudget float64, budgetDuration string) error {
	return c.ensure(ctx, "user", budgetBody{UserID: id, MaxBudget: maxBudget, BudgetDuration: budgetDuration})
}

// EnsureTeam makes the project's LiteLLM team exist with this budget.
func (c *Client) EnsureTeam(ctx context.Context, id string, maxBudget float64, budgetDuration string) error {
	return c.ensure(ctx, "team", budgetBody{TeamID: id, MaxBudget: maxBudget, BudgetDuration: budgetDuration})
}

// ensure is create-then-update (decision (b)). A transport failure is returned
// as it is, because whether the create landed is unknown and the caller retries.
// Each of the two calls has its own deadline, so ensure can take twice Timeout.
func (c *Client) ensure(ctx context.Context, kind string, body budgetBody) error {
	err := c.do(ctx, kind+"/new", http.MethodPost, nil, body, nil)
	var e *Error
	if err == nil || !errors.As(err, &e) || e.Status == 0 {
		return err
	}
	if c.do(ctx, kind+"/update", http.MethodPost, nil, body, nil) == nil {
		return nil
	}
	return err
}

// Health is LiteLLM's readiness (its database), never GET /health, which
// calls every upstream model (decision 3 above).
func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, "health/readiness", http.MethodGet, nil, nil, nil)
}
```

- [ ] **Step 4: Run the tests to verify they pass, under the race detector**

```bash
cd control-plane && go test -race ./internal/litellmadmin/... -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Expected:

```
--- PASS: TestNewRefusesAnUnusableMasterKey (0.00s)
--- PASS: TestEveryCallSendsTheMasterKeyAndItsGoldenBody (0.01s)
--- PASS: TestEnsureCreatesThenUpdates (0.01s)
--- PASS: TestDeleteNothingMakesNoCall (0.00s)
--- PASS: TestACallerThatGivesUpDoesNotAbortTheCall (0.05s)
--- PASS: TestAHungServerFailsAfterTheTimeout (0.10s)
--- PASS: TestAnErrorCarriesNeitherTheBodyNorAKey (0.00s)
--- PASS: TestARedirectIsAnErrorAndNotFollowed (0.00s)
--- PASS: TestReplies (0.00s)
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/litellmadmin
```

The timings are indicative; `TestAHungServerFailsAfterTheTimeout` must stay well under its 2-second bound.

- [ ] **Step 5: Mutation spot-check, by hand, then revert**

| Edit | Test that must fail |
|---|---|
| `do`: `context.WithTimeout(context.WithoutCancel(ctx), …)` → `context.WithTimeout(ctx, …)` | `TestACallerThatGivesUpDoesNotAbortTheCall` |
| `do`: replace the `WithTimeout` line with `ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))` | `TestAHungServerFailsAfterTheTimeout` (it hangs until `go test`'s own timeout: run it with `-timeout 30s`) |
| `do`: `Type: errorType(raw)` → `Type: string(raw)` | `TestAnErrorCarriesNeitherTheBodyNorAKey` |
| `New`: delete the `HasPrefix(key, "sk-")` condition | `TestNewRefusesAnUnusableMasterKey` |
| `New`: delete `h.CheckRedirect = refuseRedirect` | `TestARedirectIsAnErrorAndNotFollowed` |
| `ensure`: `return err` after a failed create, without the update | `TestEnsureCreatesThenUpdates` |
| `KeyInfo`: `url.Values{"key": {keyID}}` → the path `key/info/`+keyID | `TestEveryCallSendsTheMasterKeyAndItsGoldenBody` |

- [ ] **Step 6: Format, vet and confirm the import set**

```bash
cd control-plane && gofmt -l internal/litellmadmin && go vet ./internal/litellmadmin/...
go list -deps ./internal/litellmadmin | grep kpmg-au-emu
```

Expected: nothing from `gofmt` and `go vet`, then exactly:

```
github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass
github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/wire
github.com/kpmg-au-emu/panacea-app/control-plane/internal/litellmadmin
```

(`dataclass` arrives through `wire`; it is not imported here.)

- [ ] **Step 7: Commit**

```bash
git add control-plane/internal/litellmadmin/client.go control-plane/internal/litellmadmin/client_test.go
git commit -F /tmp/task-04-msg.txt -- control-plane/internal/litellmadmin
```

with `/tmp/task-04-msg.txt`:

```
feat(litellmadmin): LiteLLM's admin API on its own deadlines

Every call runs on context.WithTimeout(context.WithoutCancel(ctx), ...),
so a caller that gives up never aborts a mint half-way and no caller's
deadline is spent on LiteLLM: Panacea's shared 15-second chain, closed.

The master key is read once from a file and refused when empty or not
an sk- key. Errors carry the operation, the status and a short error
type, never a body, which can echo the key. Redirects are refused, keys
are addressed by id after the mint, Ensure is create-then-update, and
Health asks /health/readiness, never the model-calling /health.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

**Acceptance:** step 4's ten lines under `-race`; each row of step 5 turns its test red and reverting turns it green; step 6 prints nothing and then the three lines.

**What this task does not claim:**

- **That LiteLLM's paths, field names and replies are as written.** The table above is LiteLLM's documented admin API as remembered, not read for this plan; task 18 runs every call against the pinned digest, and a mismatch is fixed here, in one place.
- **That LiteLLM accepts a caller-chosen `key`** (task 18's P01; task 07's fallback), **or a key id on `/key/info` and `/key/delete`** (P11, P12).
- **That `GET /health` calls every model, or that `/health/readiness` checks the database.** Both are training knowledge; the choice of readiness is safe either way.
- **Idempotency.** A retried `GenerateKey` after a timeout may create a second key; task 07 owns that (one live key per session generation, alias `sess/<session>/<generation>`).
- **A request count.** `KeyInfo` returns spend, not the number of requests; LiteLLM's documented `/key/info` reply, as remembered, has no such field. Task 09's `requests` column needs another source (the spend logs) or must be dropped; that is in the gap list.

**Gaps against the skeleton, for the orchestrator:**

1. **Task 07's identifiers are adopted exactly** (its gap 4), plus `GenerateKey` returning `GenerateKeyResponse`, `KeyInfo(ctx, keyID) (KeyInfo, error)` and `Health(ctx) error`.
2. **`Ensure` is create-then-update** (decision (b)); the skeleton says only "Ensure". Each call has its own deadline, so an `Ensure` can take twice `Timeout`; task 07 should budget for that.
3. **`KeyInfo` carries no request count.** Task 09's `key_spend.requests` and `wire.UsageRow.Requests` have no source in this client.
4. **`/health/readiness`, not `/health`**, for `Health` (and task 08's `/readyz`); task 17 already uses readiness.
