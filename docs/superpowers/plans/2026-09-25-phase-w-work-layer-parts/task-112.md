### Task 112: the same verbs in solo mode through phase 3's runner (UNVERIFIED that it can host them)

Stream **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on
task 101 (the three work verbs, their dispatch rows and their ST.1 fixtures). Uses TM-09's control
wire v1 and its fixture set (`testdata/panacea-control/v1/*.json`, Phase T plan `:506-514`, read at
`64ab2c4`), TM-16's stamping rule (*"every request on it is stamped with the pod's session id and
class before it is forwarded"*, `:584-589`), TM-18's `panacea` binary (`:605-614`), SW.10's Start
binding (task 24) and `Session.IssueRef` (task 19); all Phase T paths are **UNVERIFIED until
merge**. Phase 3's runner is `lanegit.Runner` and `lanegit.Lanes` (P3 task 24, phase 3 plan
`:120-123` and `:598`, read at `8fce56a`): the `sandboxd` git runner's `/git/v1/` RPC on the
attach token.

04 §4.5 says why this task exists: *"Solo mode has no `panacea` channel yet … Until the control
plane serves the same verbs through phase 3's `sandboxd` runner, solo issues get observed facts and
human text only."* This task serves them, or records that it cannot.

**Decisions made here, so nobody re-derives them:**

1. **A spike comes first and can stop the task.** A solo session runs either on the host (a PTY
   the control plane starts on the person's machine) or in a local sandbox under phase 3's runner.
   A loopback endpoint on the host is reachable from a host session; from inside a container,
   `127.0.0.1` is the container's own loopback, so the same endpoint is **not** reachable unless
   the sandbox forwards it. The spike answers one question: *can a session under phase 3's runner
   reach the control plane's verb endpoint, through `sandboxd`'s `/control` (TM-16) dialled by the
   control plane, with no second runner?* It records **yes** or **no** with the command and its
   output. On **no**, the task serves host sessions only, says so in the results, and writes no
   second runner.
2. **Three verbs and nothing else.** `issue.show`, `issue.note` and `evidence.propose`, byte for
   byte as task 101 serves them in Fleet, tested against the **same** fixture files. `team` is
   answered with `solo_project`, whose message is *"solo project: no team view"*. Every other verb
   in TM-09's table is answered `unknown_verb`: solo mode has no leases, peers or workers.
3. **The credential is the channel** (TM-16's rule). Each host session gets its own listener on
   `127.0.0.1` with an ephemeral port and a 32-byte random bearer token, passed in the session's
   environment only (`PANACEA_CONTROL_URL`, `PANACEA_CONTROL_TOKEN`). The stamp is the session
   whose token authenticated; any session id in a request body is ignored. The token is never
   written to disk, logged or returned by `sessionJSON` (phase 2's rule).
4. **Loopback, twice.** The listener binds a loopback address only, and every connection whose
   remote address is not loopback is closed before a byte is read. `PANACEA_APP_WORK_VERBS_SOCKET`
   names the bind address (default `127.0.0.1:0`); a non-loopback value fails startup.
5. **The Start-bound session is the issue's one voice.** With no Orchestrator in solo mode, a
   session created by Start (SW.10) with `Session.IssueRef` naming issue X may `note` and `propose`
   on X only. Any other session, including a plain session the person opened by hand, gets
   `class_forbidden`. `issue.show` is allowed from any Start-bound session, for any issue the
   person may read.
6. **Written as the person's own agent.** The control plane writes through task 99's
   `AddAgentComment` with the **person's** Hub token and the attribution `{ownerId: the person,
   agentId: the session id, model, sessionId}`, class `text·A`, `via: "local"`. Task 100's guard
   requires a non-null `via` on an agent-kind comment and was written for Fleet; whether it
   accepts `via: "local"` from a person's token is **UNVERIFIED** (gap 2), and this part never
   writes the comment as the person's own `text·H`.
7. **The binary is TM-18's, unchanged in meaning.** The control plane puts the directory holding
   `panacea` first on the session's `PATH`. TM-18's binary dials a Unix socket
   (`/run/panacea/control.sock`, TM-16); on a host session it must dial `PANACEA_CONTROL_URL`
   instead, one added transport in `sandboxd/internal/panaceacli` (a delta, gap 3), and a Windows
   build, because TM-18 builds a static Linux binary (`CGO_ENABLED=0`, P8-D8).
8. **No CLI is parsed or modified** (`OD-8`). The harness calls `panacea` because SW.7's managed
   block asks it to; nothing reads the harness's terminal to find out whether it did.

**Files:**
- `control-plane/internal/workverbs/serve.go` (+ `_test.go`) — the listener per session and the
  three-verb dispatcher (new; not in the file map)
- `control-plane/internal/workverbs/stamp.go` (+ `_test.go`) — tokens, the stamp, the loopback
  check (new; not in the file map)
- `control-plane/internal/api/workverbs.go` (+ `_test.go`) — the session hook: allocate on a
  Start-bound session, inject the environment, release on exit (new; not in the file map)
- `control-plane/internal/api/api.go` ⚑, `routes_audit_test.go` ⚑ — `Deps.WorkVerbs`; one audit
  row (modified)
- `control-plane/cmd/api/config.go` ⚑ — `PANACEA_APP_WORK_VERBS_SOCKET` (modified)
- `sandboxd/internal/panaceacli/transport.go` ⚑ (TM-18's package) — the URL transport (modified;
  **UNVERIFIED until merge**)
- `docs/verification/2026-09-25-phase-w-results.md` ⚑ — the spike's record, under W3 (modified;
  task 113 owns the section, this task writes its one paragraph)

#### Wire — complete: what is new in solo mode

The request and result bytes of the three verbs are **task 101's fixtures, reused unchanged**
(`issue.show.*.json`, `issue.note.*.json`, `evidence.propose.*.json`); this part adds no field to
them. Framing on the loopback endpoint: one HTTP `POST /control` per request, body one TM-09
envelope, response one TM-09 response, header `Authorization: Bearer <session token>`. The three
new response fixtures, added to TM-09's set as `solo.*.json`:

```json
{ "id": "3", "ok": false, "error": { "code": "solo_project", "message": "solo project: no team view" } }
{ "id": "4", "ok": false, "error": { "code": "class_forbidden", "message": "only the session Start bound to this issue may write on it" } }
{ "id": "5", "ok": false, "error": { "code": "unknown_verb", "message": "solo mode serves issue.show, issue.note and evidence.propose" } }
```

Session environment, set by the control plane on a Start-bound host session only:

```text
PANACEA_CONTROL_URL=http://127.0.0.1:49731/control
PANACEA_CONTROL_TOKEN=<43 characters, base64url of 32 random bytes>
PATH=<the directory holding panacea><os.PathListSeparator><the session's PATH>
```

#### Interfaces — complete: `control-plane/internal/workverbs/stamp.go`

```go
// Package workverbs serves Phase T's three work verbs to solo sessions on the local control
// plane, byte-compatible with Fleet's (Phase W task 112).
package workverbs

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync"
)

// Stamp is who is calling: fixed by the token, never by the body (decision 3).
type Stamp struct {
	SessionID string // phase 2's session id
	OwnerID   string // u-…, the signed-in person
	IssueID   string // is-… from Session.IssueRef, set only on a Start-bound session
	ProjectID string // pj-…
}

type Tokens struct {
	mu   sync.Mutex
	byTk map[string]Stamp
}

func NewTokens() *Tokens { return &Tokens{byTk: map[string]Stamp{}} }

// Issue mints a token for one session. The token lives in memory and in the session env only.
func (t *Tokens) Issue(s Stamp) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tk := base64.RawURLEncoding.EncodeToString(b)
	t.mu.Lock()
	t.byTk[tk] = s
	t.mu.Unlock()
	return tk, nil
}

// Revoke is called when the session exits; its token then stamps nothing.
func (t *Tokens) Revoke(sessionID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, s := range t.byTk {
		if s.SessionID == sessionID {
			delete(t.byTk, k)
		}
	}
}

var ErrNoStamp = errors.New("unauthenticated")

// StampFor resolves an Authorization header in constant time per candidate.
func (t *Tokens) StampFor(authz string) (Stamp, error) {
	tk, ok := strings.CutPrefix(authz, "Bearer ")
	if !ok || tk == "" {
		return Stamp{}, ErrNoStamp
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, s := range t.byTk {
		if subtle.ConstantTimeCompare([]byte(k), []byte(tk)) == 1 {
			return s, nil
		}
	}
	return Stamp{}, ErrNoStamp
}

// Loopback reports whether a host:port names a loopback address (decision 4).
func Loopback(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return false
	}
	a, err := netip.ParseAddr(host)
	return err == nil && a.IsLoopback()
}
```

#### Interfaces — complete: `control-plane/internal/workverbs/serve.go`

```go
package workverbs

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"
)

// Envelope and Response are TM-09's wire v1; step 1 replaces them with an import of
// control-plane/internal/fleet/wire if Phase T exports it, so the bytes cannot drift.
type Envelope struct {
	V    int             `json:"v"`
	ID   string          `json:"id"`
	Verb string          `json:"verb"`
	Args json.RawMessage `json:"args"`
}

type WireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Since   string `json:"since,omitempty"`
}

type Response struct {
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *WireError      `json:"error,omitempty"`
}

// Verbs is the three handlers, the same code task 101 runs in Fleet, behind an interface that
// takes this package's Stamp. Each returns TM-09's result JSON or a *WireError.
type Verbs interface {
	IssueShow(ctx context.Context, s Stamp, args json.RawMessage) (json.RawMessage, error)
	IssueNote(ctx context.Context, s Stamp, args json.RawMessage) (json.RawMessage, error)
	EvidencePropose(ctx context.Context, s Stamp, args json.RawMessage) (json.RawMessage, error)
}

// IssueOf reads the `issue` arg of a write verb (task 101's args name it `issue`; for
// evidence.propose, the issue is the session's own).
type IssueOf func(verb string, args json.RawMessage) (string, error)

type Server struct {
	Tokens  *Tokens
	Verbs   Verbs
	IssueOf IssueOf
}

const maxBody = 64 << 10 // TM-19's oversized-args rule, in bytes; step 1 matches TM-09's number

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !Loopback(r.RemoteAddr) || r.Method != http.MethodPost || r.URL.Path != "/control" {
		http.Error(w, "", http.StatusForbidden)
		return
	}
	st, err := s.Tokens.StampFor(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	var env Envelope
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil || env.V != 1 {
		writeResp(w, Response{ID: env.ID, Error: &WireError{Code: "invalid_request", Message: "not a v1 envelope"}})
		return
	}
	writeResp(w, s.dispatch(r.Context(), st, env))
}

func (s *Server) dispatch(ctx context.Context, st Stamp, env Envelope) Response {
	fail := func(code, msg string) Response {
		return Response{ID: env.ID, Error: &WireError{Code: code, Message: msg}}
	}
	if st.IssueID == "" {
		return fail("class_forbidden", "only a session started from an issue may use these verbs")
	}
	var h func(context.Context, Stamp, json.RawMessage) (json.RawMessage, error)
	switch env.Verb {
	case "issue.show":
		h = s.Verbs.IssueShow
	case "issue.note", "evidence.propose":
		issue, err := s.IssueOf(env.Verb, env.Args)
		if err != nil {
			return fail("invalid_args", err.Error())
		}
		if issue != st.IssueID {
			return fail("class_forbidden", "only the session Start bound to this issue may write on it")
		}
		if env.Verb == "issue.note" {
			h = s.Verbs.IssueNote
		} else {
			h = s.Verbs.EvidencePropose
		}
	case "team":
		return fail("solo_project", "solo project: no team view")
	default:
		return fail("unknown_verb", "solo mode serves issue.show, issue.note and evidence.propose")
	}
	res, err := h(ctx, st, env.Args)
	var we *WireError
	switch {
	case errors.As(err, &we):
		return Response{ID: env.ID, Error: we}
	case err != nil:
		return fail("unavailable", "the Hub could not be reached")
	}
	return Response{ID: env.ID, OK: true, Result: res}
}

func (e *WireError) Error() string { return e.Code }

func writeResp(w http.ResponseWriter, r Response) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(r)
}
```

The rest of `serve.go` (one file with the block above; `net` and `time` are used here):

```go
var ErrNotLoopback = errors.New("PANACEA_APP_WORK_VERBS_SOCKET must name a loopback address")

// Endpoint is one session's listener. Close it when the session exits.
type Endpoint struct {
	URL   string // http://127.0.0.1:<port>/control
	srv   *http.Server
	ln    net.Listener
}

// Open binds one listener for one session on addr (default 127.0.0.1:0) and serves s on it.
func Open(addr string, s *Server) (*Endpoint, error) {
	if !Loopback(addr) {
		return nil, ErrNotLoopback
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Handler: s, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return &Endpoint{URL: "http://" + ln.Addr().String() + "/control", srv: srv, ln: ln}, nil
}

func (e *Endpoint) Close(ctx context.Context) error { return e.srv.Shutdown(ctx) }
```

One listener per session, not one shared listener, so a leaked token names one session's port and
nothing else; the count of listeners equals the count of live Start-bound host sessions.

#### Interfaces — complete: `control-plane/internal/api/workverbs.go`

```go
package api

import (
	"context"
	"os"
	"path/filepath"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/workverbs"
)

// WorkVerbs is Deps.WorkVerbs: set when PANACEA_APP_WORK_VERBS_SOCKET is set (config.go).
type WorkVerbs struct {
	Addr      string // the bind address, validated loopback at startup
	BinDir    string // the directory holding the panacea binary for this OS
	Server    *workverbs.Server
	endpoints map[string]*workverbs.Endpoint // by session id; guarded by the sessions lock
}

// SessionEnv is called by the session create path (P4 task 10's sessions.go, after task 19's
// IssueRef is set) for a Start-bound host session. It returns the env lines to add.
func (v *WorkVerbs) SessionEnv(ctx context.Context, st workverbs.Stamp, basePath string) ([]string, error) {
	if st.IssueID == "" {
		return nil, nil // a session the person opened by hand gets no channel at all
	}
	tk, err := v.Server.Tokens.Issue(st)
	if err != nil {
		return nil, err
	}
	ep, err := workverbs.Open(v.Addr, v.Server)
	if err != nil {
		v.Server.Tokens.Revoke(st.SessionID)
		return nil, err
	}
	if v.endpoints == nil {
		v.endpoints = map[string]*workverbs.Endpoint{}
	}
	v.endpoints[st.SessionID] = ep
	return []string{
		"PANACEA_CONTROL_URL=" + ep.URL,
		"PANACEA_CONTROL_TOKEN=" + tk,
		"PATH=" + filepath.Clean(v.BinDir) + string(os.PathListSeparator) + basePath,
	}, nil
}

// SessionExited revokes the token and closes the listener; called from the reaper's exit path.
func (v *WorkVerbs) SessionExited(ctx context.Context, sessionID string) {
	v.Server.Tokens.Revoke(sessionID)
	if ep := v.endpoints[sessionID]; ep != nil {
		_ = ep.Close(ctx)
		delete(v.endpoints, sessionID)
	}
}
```

The two calls are one line each in the session create and exit paths (`sessions.go` ⚑, P4 task 10
and task 19's `IssueRef`; `control-plane/internal/reaper` for exit). `sessionJSON` gains no field:
the env lines are never serialised (a test below). `api.go` adds `WorkVerbs *WorkVerbs` to `Deps`;
it adds **no route** to the main mux, and `routes_audit_test.go` gains the row *no `/control` path
is registered on the API mux*.

#### Interfaces — complete: the addition to `control-plane/cmd/api/config.go`

```go
// WorkVerbsAddr is PANACEA_APP_WORK_VERBS_SOCKET: the bind address for solo sessions' work-verb
// endpoints. Empty turns the channel off. A non-loopback value is a startup error (decision 4).
WorkVerbsAddr string

// in the loader, beside the other PANACEA_APP_* reads:
cfg.WorkVerbsAddr = os.Getenv("PANACEA_APP_WORK_VERBS_SOCKET")
if cfg.WorkVerbsAddr == "default" {
	cfg.WorkVerbsAddr = "127.0.0.1:0"
}
if cfg.WorkVerbsAddr != "" && !workverbs.Loopback(cfg.WorkVerbsAddr) {
	return Config{}, fmt.Errorf("PANACEA_APP_WORK_VERBS_SOCKET=%q: %w", cfg.WorkVerbsAddr, workverbs.ErrNotLoopback)
}
```

Off by default until the spike passes and the owner turns it on; the desktop's launcher sets
`PANACEA_APP_WORK_VERBS_SOCKET=default` for solo projects once step 2 records **yes**.

#### Interfaces — complete: the transport delta in TM-18's `sandboxd/internal/panaceacli/transport.go`

```go
// Dial picks the channel: the in-pod socket (TM-16) when it exists, else the solo control plane's
// loopback URL (task 112). Both carry the same TM-09 envelope; the verbs and their output do not
// change.
func Dial() (Transport, error) {
	if _, err := os.Stat("/run/panacea/control.sock"); err == nil {
		return unixTransport("/run/panacea/control.sock"), nil
	}
	if u, tk := os.Getenv("PANACEA_CONTROL_URL"), os.Getenv("PANACEA_CONTROL_TOKEN"); u != "" && tk != "" {
		return httpTransport(u, tk), nil
	}
	return nil, errUnavailable // prints TM-18's "team view unavailable since <t>" answer
}
```

`Transport`, `unixTransport` and `errUnavailable` are TM-18's names as this part assumes them;
step 1 reads TM-18's package and fits `httpTransport` beside its socket client. `httpTransport`
refuses a URL whose host is not loopback before dialling.
