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
6. **Written as the person's own agent, through a delegation.** The control plane writes
   through task 100's `AddAgentComment` with the attribution `{ownerId: the person, agentId: the
   session id, model, sessionId}`, class `text·A`. Task 100 **refuses a person's own token** on
   that route (`TestAPersonCannotWriteAnAgentKind`: *"Dylan's desktop token on `agent-comments` →
   403 `required: delegation`"*, `task-100.md:796`), rightly: otherwise a person could forge agent
   text. So solo writes need a **delegation the person grants to their own local control plane**
   (Phase T's delegation record, TM-06, with a holder other than Fleet and `via: local`). That
   holder does not exist in TM-06 or task 100 (gap 2). Until it does, the two write verbs answer
   `unavailable` with *"solo agent notes need a local delegation (not built)"*, `issue.show`
   works, and this part never falls back to writing the comment as the person's own `text·H`.
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

#### Interfaces — complete: `control-plane/internal/workverbs/solo.go` (the `Verbs` for solo mode)

```go
package workverbs

import (
	"context"
	"encoding/json"
)

// Handlers is task 101's verb logic with its Hub writer abstracted: the same functions Fleet
// registers, taking the writer they should use. Step 1 points these at task 101's exports.
type Handlers interface {
	Show(ctx context.Context, owner, issue string, args json.RawMessage) (json.RawMessage, error)
	Note(ctx context.Context, w AgentWriter, args json.RawMessage) (json.RawMessage, error)
	Propose(ctx context.Context, w AgentWriter, args json.RawMessage) (json.RawMessage, error)
}

// AgentWriter writes an agent-kind comment with the attribution fixed by the caller, never by args.
type AgentWriter interface {
	WriteAgentComment(ctx context.Context, issueID string, kind string, body json.RawMessage) (json.RawMessage, error)
}

// LocalDelegation returns a writer acting under the person's delegation to this control plane,
// or ok=false while no such delegation exists (decision 6, gap 2).
type LocalDelegation interface {
	Writer(ctx context.Context, s Stamp, model string) (w AgentWriter, ok bool)
}

type SoloVerbs struct {
	H       Handlers
	Deleg   LocalDelegation
	ModelOf func(sessionID string) string // from the session record; "" when unknown
}

var errNoLocalDelegation = &WireError{Code: "unavailable", Message: "solo agent notes need a local delegation (not built)"}

func (v SoloVerbs) IssueShow(ctx context.Context, s Stamp, args json.RawMessage) (json.RawMessage, error) {
	return v.H.Show(ctx, s.OwnerID, s.IssueID, args)
}

func (v SoloVerbs) IssueNote(ctx context.Context, s Stamp, args json.RawMessage) (json.RawMessage, error) {
	w, ok := v.Deleg.Writer(ctx, s, v.ModelOf(s.SessionID))
	if !ok {
		return nil, errNoLocalDelegation
	}
	return v.H.Note(ctx, w, args)
}

func (v SoloVerbs) EvidencePropose(ctx context.Context, s Stamp, args json.RawMessage) (json.RawMessage, error) {
	w, ok := v.Deleg.Writer(ctx, s, v.ModelOf(s.SessionID))
	if !ok {
		return nil, errNoLocalDelegation
	}
	return v.H.Propose(ctx, w, args)
}
```

`solo.go` is a third file in the package, beyond the scope's two; the file map needs it (gap 5).

#### Tests to write first — `control-plane/internal/workverbs/serve_test.go`

```go
package workverbs

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// fixtures is TM-09's set; task 101 adds the three verbs' files to it.
const fixtures = "../../../testdata/panacea-control/v1"

type recVerbs struct{ got []Stamp }

func (r *recVerbs) reply(s Stamp, name string) (json.RawMessage, error) {
	r.got = append(r.got, s)
	b, err := os.ReadFile(filepath.Join(fixtures, name+".result.json"))
	var resp Response
	_ = json.Unmarshal(b, &resp)
	return resp.Result, err
}
func (r *recVerbs) IssueShow(_ context.Context, s Stamp, _ json.RawMessage) (json.RawMessage, error) { return r.reply(s, "issue.show") }
func (r *recVerbs) IssueNote(_ context.Context, s Stamp, _ json.RawMessage) (json.RawMessage, error) { return r.reply(s, "issue.note") }
func (r *recVerbs) EvidencePropose(_ context.Context, s Stamp, _ json.RawMessage) (json.RawMessage, error) {
	return r.reply(s, "evidence.propose")
}

func argIssue(_ string, a json.RawMessage) (string, error) {
	var x struct{ Issue string `json:"issue"` }
	err := json.Unmarshal(a, &x)
	return x.Issue, err
}

func server(t *testing.T, st Stamp) (*Server, string, *recVerbs) {
	v := &recVerbs{}
	s := &Server{Tokens: NewTokens(), Verbs: v, IssueOf: argIssue}
	tk, err := s.Tokens.Issue(st)
	if err != nil {
		t.Fatal(err)
	}
	return s, tk, v
}

func call(s *Server, tk, remote string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/control", bytes.NewReader(body))
	r.RemoteAddr, r.Header = remote, http.Header{"Authorization": {"Bearer " + tk}}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func compact(t *testing.T, b []byte) string {
	var out bytes.Buffer
	if err := json.Compact(&out, bytes.TrimSpace(b)); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

var dylanOn142 = Stamp{SessionID: "s-1", OwnerID: "u-dylan", IssueID: "is-142", ProjectID: "pj-1"}

func TestEveryFixtureRoundTripsByteForByte(t *testing.T) {
	for _, verb := range []string{"issue.show", "issue.note", "evidence.propose"} {
		req, err := os.ReadFile(filepath.Join(fixtures, verb+".request.json"))
		if err != nil {
			t.Fatal(err)
		}
		var env Envelope
		_ = json.Unmarshal(req, &env)
		st := dylanOn142
		if is, _ := argIssue(verb, env.Args); is != "" {
			st.IssueID = is // the session is Start-bound to the fixture's own issue
		}
		s, tk, _ := server(t, st)
		want, _ := os.ReadFile(filepath.Join(fixtures, verb+".result.json"))
		got := call(s, tk, "127.0.0.1:50000", req)
		if compact(t, got.Body.Bytes()) != compact(t, want) {
			t.Fatalf("%s: solo reply differs from Fleet's fixture\n got %s\nwant %s", verb, got.Body, want)
		}
	}
}

func TestASessionNotBoundByStartIsRefused(t *testing.T) {
	s, tk, v := server(t, Stamp{SessionID: "s-2", OwnerID: "u-dylan"}) // opened by hand: no IssueID
	got := call(s, tk, "127.0.0.1:50000", []byte(`{"v":1,"id":"1","verb":"issue.show","args":{"issue":"is-142"}}`))
	if !bytes.Contains(got.Body.Bytes(), []byte(`"class_forbidden"`)) || len(v.got) != 0 {
		t.Fatalf("got %s, handler calls %d", got.Body, len(v.got))
	}
}

func TestAWriteOnAnotherIssueIsRefused(t *testing.T) {
	s, tk, v := server(t, dylanOn142)
	got := call(s, tk, "127.0.0.1:50000", []byte(`{"v":1,"id":"1","verb":"issue.note","args":{"issue":"is-150","text":"x"}}`))
	if !bytes.Contains(got.Body.Bytes(), []byte(`"class_forbidden"`)) || len(v.got) != 0 {
		t.Fatalf("got %s", got.Body)
	}
}

func TestTheStampWinsOverABodySessionID(t *testing.T) {
	s, tk, v := server(t, dylanOn142)
	call(s, tk, "127.0.0.1:50000", []byte(`{"v":1,"id":"1","verb":"issue.show","args":{"issue":"is-142","session":"s-alice","owner":"u-alice"}}`))
	if len(v.got) != 1 || v.got[0] != dylanOn142 {
		t.Fatalf("handler saw %+v", v.got)
	}
}

func TestANonLoopbackPeerIsRefusedAndCannotBind(t *testing.T) {
	s, tk, v := server(t, dylanOn142)
	if got := call(s, tk, "10.1.2.3:50000", []byte(`{"v":1,"id":"1","verb":"issue.show","args":{}}`)); got.Code != http.StatusForbidden || len(v.got) != 0 {
		t.Fatalf("code %d", got.Code)
	}
	for _, addr := range []string{"0.0.0.0:0", ":0", "192.168.1.5:0", "[::]:0"} {
		if _, err := Open(addr, s); err != ErrNotLoopback {
			t.Fatalf("Open(%q) = %v", addr, err)
		}
	}
}

func TestTeamInSoloSaysSo(t *testing.T) {
	s, tk, _ := server(t, dylanOn142)
	got := call(s, tk, "127.0.0.1:50000", []byte(`{"v":1,"id":"3","verb":"team","args":{}}`))
	want, _ := os.ReadFile(filepath.Join(fixtures, "solo.team.json"))
	if compact(t, got.Body.Bytes()) != compact(t, want) {
		t.Fatalf("got %s", got.Body)
	}
}

func TestARevokedTokenStampsNothing(t *testing.T) {
	s, tk, _ := server(t, dylanOn142)
	s.Tokens.Revoke("s-1")
	if got := call(s, tk, "127.0.0.1:50000", []byte(`{"v":1,"id":"1","verb":"issue.show","args":{}}`)); got.Code != http.StatusUnauthorized {
		t.Fatalf("code %d", got.Code)
	}
}
```

`solo.team.json` is the first of the three solo fixtures above, saved under that name.

#### Tests to write first — `control-plane/internal/workverbs/solo_test.go` and `internal/api/workverbs_test.go`

```go
func TestSoloWritesAreTheAgentsThroughTheDelegationOrNothing(t *testing.T) {
	h := &recHandlers{}
	none := SoloVerbs{H: h, Deleg: noDelegation{}, ModelOf: func(string) string { return "claude-opus-4-8" }}
	_, err := none.IssueNote(context.Background(), dylanOn142, json.RawMessage(`{"issue":"is-142","text":"x"}`))
	var we *WireError
	if !errors.As(err, &we) || we.Code != "unavailable" || h.notes != 0 {
		t.Fatalf("wrote without a delegation: %v, notes %d", err, h.notes)
	}
	w := &recWriter{}
	with := SoloVerbs{H: h, Deleg: fixedDelegation{w}, ModelOf: func(string) string { return "claude-opus-4-8" }}
	if _, err := with.IssueNote(context.Background(), dylanOn142, json.RawMessage(`{"issue":"is-142","text":"x"}`)); err != nil {
		t.Fatal(err)
	}
	if w.stamp != dylanOn142 || w.model != "claude-opus-4-8" || w.personToken {
		t.Fatalf("writer %+v: the note must be the agent's, under the delegation, never the person's token", w)
	}
}
```

```go
// internal/api/workverbs_test.go
func TestAHandOpenedSessionGetsNoChannel(t *testing.T)    // SessionEnv with IssueID "" returns no env lines
func TestTheTokenNeverReachesSessionJSON(t *testing.T)    // create a Start-bound session; sessionJSON has no PANACEA_CONTROL_TOKEN value
func TestExitClosesTheListener(t *testing.T)              // after SessionExited, a dial to the URL is refused
func TestNoControlRouteOnTheAPIMux(t *testing.T)          // routes_audit_test.go row: no /control path
```

The four `api` tests are named with their assertion; their bodies follow `sessions_test.go`'s
fake-backend shape (`control-plane/internal/api/sessions_test.go`, built), which step 1 reads.
`recHandlers`, `recWriter`, `noDelegation` and `fixedDelegation` are 10-line doubles in
`solo_test.go`: `recWriter` records the stamp and model it was built with and whether it was built
from the person's own token.

| Test | Mutant it kills |
|---|---|
| every fixture round-trips | the solo wire drifts from Fleet's |
| a hand-opened session is refused | any session writes notes |
| a write on another issue is refused | the one-voice rule checks binding, not the issue |
| the stamp wins over the body | a caller-supplied identity |
| non-loopback peer refused; no non-loopback bind | the endpoint binds all interfaces |
| `team` says *solo project: no team view* | fake team rows in solo |
| a revoked token stamps nothing | a session's channel outlives the session |
| writes through the delegation or not at all | written as the person, `text·H`, from the person's own token |
| a hand-opened session gets no env | a channel handed to every session |
| the token never in `sessionJSON` | the credential leaks to the desktop or a log |
| exit closes the listener | a listener per dead session accumulates |
| no `/control` on the API mux | the verbs served on the main API, where any local caller could reach them |

#### Steps

- [ ] **Step 1: read on the sync base** TM-09's wire package and fixture names, TM-16's socket
  framing, TM-18's `panaceacli` transport, task 101's three handlers and their fixture files,
  task 24's Start binding and task 19's `Session.IssueRef`, phase 2's `sessions.go` create and
  exit paths and `sessionJSON`, phase 3's `lanegit.Runner` (P3 task 24) and how a local sandboxed
  session is started under it. Rename in this part's files only.
- [ ] **Step 2: the spike (30 minutes, a hard stop).** Start one solo session under phase 3's
  runner in a local sandbox, and from inside it run
  `curl -s -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:<port>/control` against an
  endpoint opened with `Open("127.0.0.1:0", …)` on the host; then, if `sandboxd` carries TM-16's
  `/control` in the local image, open it from the control plane over the attach connection and
  send one `issue.show` envelope. Record each command, its output and the commit in the W3
  section of `docs/verification/2026-09-25-phase-w-results.md`, one paragraph headed *Task 112
  spike*. **If neither path reaches the endpoint, stop**: the task ships host sessions only, the
  record says *no*, and no second runner is written.
- [ ] **Step 3: the Go tests first**, then `stamp.go`, `serve.go` and `solo.go`; the three solo
  fixtures join TM-09's set.
- [ ] **Step 4: `workverbs.go`**, its two calls in the session create and exit paths, `Deps`,
  `config.go` and the audit row.
- [ ] **Step 5: the transport delta** in TM-18's package and a `GOOS=windows` build of
  `sandboxd/cmd/panacea` placed in the desktop bundle's `bin/` (the packaging line is phase 2's
  installer; **UNVERIFIED** that it accepts a second binary).
- [ ] **Step 6: run the acceptance commands.** Commit by path after steps 2, 3, 4 and 5, each
  message ending with the `Co-Authored-By` line.

#### Acceptance

```bash
go test ./control-plane/internal/workverbs/ -race -count=1 -v
# expect: serve 7 and solo 1 tests, all PASS
go test ./control-plane/internal/api/ ./control-plane/cmd/api/ -race -count=1 -run 'WorkVerbs|HandOpened|TokenNever|ExitCloses|NoControlRoute|RoutesAudit|Config'
# expect: ok for both packages
PANACEA_APP_WORK_VERBS_SOCKET=0.0.0.0:7000 go run ./control-plane/cmd/api 2>&1 | head -1
# expect: one line naming PANACEA_APP_WORK_VERBS_SOCKET and "must name a loopback address"; exit non-zero
go test ./control-plane/... -race -count=1
# expect: ok for every package; no FAIL line
grep -c "Task 112 spike" docs/verification/2026-09-25-phase-w-results.md
# expect: 1
```

The `go run` line starts no server when the address is refused, so it leaves nothing running (the
background-orphan rule does not apply). The counts are this part's plan, not a run.

#### What this task does NOT claim

- That a session under phase 3's runner can reach the verbs; step 2's record decides, and it may
  say *no*.
- That solo `issue.note` and `evidence.propose` write anything before a local delegation exists
  (decision 6, gap 2). `issue.show` is the only verb that works on the Hub as it is planned today.
- That any local harness calls the verbs unprompted: SW.7's managed block asks it to, and nothing
  observes whether it does (`OD-8`).
- That the Windows `panacea` build ships in the installer (step 5).
- A second runner, a team view in solo mode, or leases in solo mode.

**Parity.** Closes no new row. It carries **L-156**'s and **L-166**'s W3 halves (`05-design.md:318,
:328`) to solo projects: `issue.show` now, the two writes once gap 2 is closed.

#### Gaps found while writing this part

1. **The spike's answer is unknown** until step 2 runs; a *no* leaves sandboxed solo sessions on
   *"observed facts and human text only"* (04 §4.5).
2. **No delegation holder for the local control plane.** Task 100 refuses a person's token on
   agent-comments (`task-100.md:796`), and TM-06's delegation names Fleet as its holder. Solo agent
   notes need a delegation the person grants to their own control plane, `via: local`, with its
   own matrix rows: a delta for TM-06 and a WH task after task 100.
3. **TM-18's binary dials a Unix socket only** and is built for Linux; a URL transport and a
   Windows build are deltas to TM-18 (rule H2).
4. **Fixture names are assumed** (`<verb>.request.json`, `<verb>.result.json`); TM-09 and task 101
   fix them.
5. **The file map has no row** for `control-plane/internal/workverbs/` (`serve.go`, `stamp.go`,
   `solo.go`), `internal/api/workverbs.go` or the `config.go` variable.
