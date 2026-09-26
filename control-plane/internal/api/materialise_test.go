package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)

// Phase 4 task 10, ruling R1: CLAUDE.md is on disk before the harness process
// starts, and Session.ContextRef names exactly what was written. These tests
// pin WHERE the handlers call the Materialiser, relative to everything that
// records or creates, and that the stamp is the Materialiser's answer and never
// the request's.

// materialiseLog is the one ordered log the Materialiser, the backend and the
// store all write to, so a test reads their relative order off one slice.
type materialiseLog struct {
	mu      sync.Mutex
	entries []string
}

func (l *materialiseLog) add(e string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, e)
}

func (l *materialiseLog) get() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.entries...)
}

// materialiseBackend records Create and Destroy and keeps the set of live
// sandboxes, so a test can ask whether the old one survived. Create answers an
// EMPTY Endpoint on purpose (plan re-review b01 R2-M3): the readiness watch then
// writes nothing to the store, so it cannot add an entry to the log at a time
// the test does not control. The other methods panic through the nil embedded
// interface: nothing here may reach them.
type materialiseBackend struct {
	backend.Backend
	log  *materialiseLog
	mu   sync.Mutex
	live map[string]bool
}

func (b *materialiseBackend) Kind() backend.Kind { return backend.KindDocker }

func (b *materialiseBackend) Create(_ context.Context, spec backend.Spec) (backend.Sandbox, error) {
	b.log.add("backend.create")
	b.mu.Lock()
	defer b.mu.Unlock()
	b.live[spec.ID] = true
	return backend.Sandbox{
		ID: spec.ID, Kind: backend.KindDocker, Ref: "ref-" + spec.ID,
		State: backend.StateStarting, CreatedAt: time.Now().UTC(),
	}, nil
}

func (b *materialiseBackend) Destroy(_ context.Context, id string) error {
	b.log.add("backend.destroy")
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.live[id] {
		return backend.ErrNotFound
	}
	delete(b.live, id)
	return nil
}

func (b *materialiseBackend) has(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.live[id]
}

// materialiseStore is the real FileStore with Create and Update logged. Get,
// List and Delete are the FileStore's own.
type materialiseStore struct {
	*registry.FileStore
	log *materialiseLog
}

func (s materialiseStore) Create(sess registry.Session) error {
	s.log.add("store.create")
	return s.FileStore.Create(sess)
}

func (s materialiseStore) Update(id string, mutate func(*registry.Session) error) (registry.Session, error) {
	s.log.add("store.update")
	return s.FileStore.Update(id, mutate)
}

type materialiseEnv struct {
	h             http.Handler
	log           *materialiseLog
	mat           *catalogMaterialiser
	be            *materialiseBackend
	fs            *registry.FileStore // unlogged: seeding and reading back
	wtPath        string
	conversations string
}

// materialiseNewEnv is catalogNewDeps with the recording Materialiser, backend
// and store, and a worktree that resolves to a real directory. mutate edits the
// Deps before New sees them.
func materialiseNewEnv(t *testing.T, mutate func(*api.Deps)) *materialiseEnv {
	t.Helper()
	d, fs := catalogNewDeps(t)
	env := &materialiseEnv{
		log:           &materialiseLog{},
		fs:            fs,
		wtPath:        t.TempDir(),
		conversations: d.ConversationsDir,
	}
	env.mat = &catalogMaterialiser{onCall: func(api.MaterialiseRequest) { env.log.add("materialise") }}
	env.be = &materialiseBackend{log: env.log, live: map[string]bool{}}
	d.Backend = env.be
	d.Store = materialiseStore{FileStore: fs, log: env.log}
	d.Materialiser = env.mat
	d.Worktrees = worktree.Static{catalogTestWorktree: env.wtPath}
	if mutate != nil {
		mutate(&d)
	}
	a, err := api.New(d)
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	env.h = a
	return env
}

// materialiseSeed records a running session the backend knows, stamped @5,
// straight into the FileStore so the seeding is not in the log.
func (e *materialiseEnv) seed(t *testing.T, mutate func(*registry.Session)) registry.Session {
	t.Helper()
	sess := registry.Session{
		ID: "s-0000000b", Title: "t", MachineID: "local",
		ProjectID: catalogTestProject, BranchID: catalogTestBranch, WorktreeID: catalogTestWorktree,
		Harness: "claude-code", Model: "opus", Command: "claude", Args: []string{"--model", "opus"},
		Status: registry.StatusRunning, Backend: "docker",
		SandboxID: "sb-old", SandboxRef: "ref-sb-old", AttachToken: "old-token",
		ContextRef: "brief:bf-3f2a91c0@5",
		CreatedAt:  time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC),
	}
	if mutate != nil {
		mutate(&sess)
	}
	if err := e.fs.Create(sess); err != nil {
		t.Fatalf("seed: %v", err)
	}
	e.be.mu.Lock()
	e.be.live[sess.SandboxID] = true
	e.be.mu.Unlock()
	return sess
}

type materialiseSession struct {
	ID         string `json:"id"`
	ContextRef string `json:"contextRef"`
}

func materialiseDecodeSession(t *testing.T, code int, body []byte, want int) materialiseSession {
	t.Helper()
	if code != want {
		t.Fatalf("status = %d, want %d; body = %s", code, want, body)
	}
	var s materialiseSession
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("decode the session: %v; body = %s", err, body)
	}
	return s
}

func materialiseDecodeProblem(t *testing.T, code int, body []byte, want int) httpx.Problem {
	t.Helper()
	if code != want {
		t.Fatalf("status = %d, want %d; body = %s", code, want, body)
	}
	var p httpx.Problem
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("decode the problem: %v; body = %s", err, body)
	}
	return p
}

const materialiseCreateBody = `{"worktreeId":"` + catalogTestWorktree + `"}`

func TestNewRefusesANilMaterialiser(t *testing.T) {
	d, _ := catalogNewDeps(t)
	d.Materialiser = nil
	if _, err := api.New(d); err == nil || !strings.Contains(err.Error(), "Materialiser") {
		t.Fatalf("api.New with a nil Materialiser = %v; want an error naming Materialiser, because a default no-op one is a session that silently inherits nothing", err)
	}
}

func TestCreateMaterialisesBeforeRecordingOrCreating(t *testing.T) {
	env := materialiseNewEnv(t, nil)
	rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions", materialiseCreateBody, nil)
	materialiseDecodeSession(t, rec.Code, rec.Body.Bytes(), http.StatusCreated)

	if got, want := env.log.get(), []string{"materialise", "store.create", "backend.create", "store.update"}; !slices.Equal(got, want) {
		t.Errorf("call log = %v, want %v: the brief is on disk before anything is recorded or started (R1)", got, want)
	}
	reqs := env.mat.requests()
	if len(reqs) != 1 {
		t.Fatalf("Materialise called %d times, want 1", len(reqs))
	}
	want := api.MaterialiseRequest{
		WorktreeID: catalogTestWorktree, WorktreePath: filepath.Clean(env.wtPath),
		ProjectID: catalogTestProject, BranchID: catalogTestBranch,
	}
	if reqs[0] != want {
		t.Errorf("Materialise got %+v, want %+v", reqs[0], want)
	}
}

func TestCreateStampsTheReturnedKeyNotTheRequested(t *testing.T) {
	env := materialiseNewEnv(t, nil)
	env.mat.result = api.MaterialiseResult{ContextRef: "brief:bf-3f2a91c0@6"}
	body := `{"worktreeId":"` + catalogTestWorktree + `","contextRef":"brief:bf-3f2a91c0@1"}`
	rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions", body, nil)
	got := materialiseDecodeSession(t, rec.Code, rec.Body.Bytes(), http.StatusCreated)

	if got.ContextRef != "brief:bf-3f2a91c0@6" {
		t.Errorf("response contextRef = %q, want the Materialiser's @6: a client's guess is not inheritance (P6)", got.ContextRef)
	}
	stored, err := env.fs.Get(got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.ContextRef != "brief:bf-3f2a91c0@6" {
		t.Errorf("record contextRef = %q, want @6", stored.ContextRef)
	}
}

func TestCreateRefusalWritesNothing(t *testing.T) {
	env := materialiseNewEnv(t, nil)
	env.mat.err = &api.MaterialiseError{
		Reason:  api.BriefReasonHubUnavailable,
		Message: "The brief could not be written because the Hub did not answer.",
	}
	rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions", materialiseCreateBody, nil)
	p := materialiseDecodeProblem(t, rec.Code, rec.Body.Bytes(), http.StatusConflict)

	if p.Error != httpx.CodeBriefNotWritten {
		t.Errorf("error = %q, want %q", p.Error, httpx.CodeBriefNotWritten)
	}
	if p.Message != "The brief could not be written because the Hub did not answer." {
		t.Errorf("message = %q, want the MaterialiseError's sentence", p.Message)
	}
	if p.Details["reason"] != api.BriefReasonHubUnavailable || p.Details["worktreeId"] != catalogTestWorktree {
		t.Errorf("details = %v, want reason %q and worktreeId %q", p.Details, api.BriefReasonHubUnavailable, catalogTestWorktree)
	}
	if got := env.log.get(); !slices.Equal(got, []string{"materialise"}) {
		t.Errorf("call log = %v, want only [materialise]: a refusal records nothing and starts nothing", got)
	}
	if list, err := env.fs.List(); err != nil || len(list) != 0 {
		t.Errorf("store holds %d sessions (err %v), want none: an orphan record for a session that never started", len(list), err)
	}
	if _, err := os.Stat(env.conversations); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Deps.ConversationsDir exists after a refusal (stat err %v): Materialise sits below sessionConversationStore's MkdirAll, so a refusal touched the disk", err)
	}
}

// Every refusal that only checks the request comes before Materialise and costs
// no Hub call (review M2). One row per validation step, in the handler's order.
func TestCreateValidatesBeforeMaterialising(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		code       string
	}{
		{"the worktree", `{"worktreeId":"w-99999999"}`, http.StatusNotFound, httpx.CodeWorktreeNotFound},
		{"harness.Lookup", `{"worktreeId":"` + catalogTestWorktree + `","harness":"nope"}`, http.StatusBadRequest, httpx.CodeUnknownHarness},
		{"sessionResolveResume", `{"worktreeId":"` + catalogTestWorktree + `","resume":"s-99999999"}`, http.StatusNotFound, httpx.CodeSessionNotFound},
		// The shell offers no models, so Build's validateModel refuses one. Not
		// the default harness: Claude Code takes a free-text model.
		{"harness.Build", `{"worktreeId":"` + catalogTestWorktree + `","harness":"shell","model":"nope"}`, http.StatusBadRequest, httpx.CodeUnsupportedOption},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := materialiseNewEnv(t, nil)
			rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions", tc.body, nil)
			if p := materialiseDecodeProblem(t, rec.Code, rec.Body.Bytes(), tc.status); p.Error != tc.code {
				t.Errorf("error = %q, want %q", p.Error, tc.code)
			}
			if got := env.log.get(); len(got) != 0 {
				t.Errorf("call log = %v, want empty: an invalid request asked the Hub and wrote the brief before it was refused", got)
			}
		})
	}
}

func TestCreateWithoutBriefPassesTheFlag(t *testing.T) {
	env := materialiseNewEnv(t, nil)
	env.mat.result = api.MaterialiseResult{ContextRef: "none:declined"}
	body := `{"worktreeId":"` + catalogTestWorktree + `","withoutBrief":true}`
	rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions", body, nil)
	got := materialiseDecodeSession(t, rec.Code, rec.Body.Bytes(), http.StatusCreated)

	if reqs := env.mat.requests(); len(reqs) != 1 || !reqs[0].WithoutBrief {
		t.Errorf("Materialise got %+v, want one call with WithoutBrief: the override was lost between the wire and the materialiser", reqs)
	}
	if got.ContextRef != "none:declined" {
		t.Errorf("contextRef = %q, want none:declined", got.ContextRef)
	}
}

func TestCreateOtherMaterialiseErrorIs500(t *testing.T) {
	env := materialiseNewEnv(t, nil)
	env.mat.err = errors.New(`open C:\Users\someone\secret-project\CLAUDE.md: Access is denied.`)
	rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions", materialiseCreateBody, nil)
	p := materialiseDecodeProblem(t, rec.Code, rec.Body.Bytes(), http.StatusInternalServerError)

	if p.Error != httpx.CodeInternal {
		t.Errorf("error = %q, want %q", p.Error, httpx.CodeInternal)
	}
	if strings.Contains(rec.Body.String(), "secret-project") || strings.Contains(rec.Body.String(), "CLAUDE.md") {
		t.Errorf("the problem body leaks the raw error: %s", rec.Body.String())
	}
	if got := env.log.get(); !slices.Equal(got, []string{"materialise"}) {
		t.Errorf("call log = %v, want only [materialise]", got)
	}
}

func TestRestartRestamps(t *testing.T) {
	env := materialiseNewEnv(t, nil)
	sess := env.seed(t, nil)
	env.mat.result = api.MaterialiseResult{ContextRef: "brief:bf-3f2a91c0@6"}
	rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions/"+sess.ID+"/restart", "", nil)
	got := materialiseDecodeSession(t, rec.Code, rec.Body.Bytes(), http.StatusOK)

	if got.ContextRef != "brief:bf-3f2a91c0@6" {
		t.Errorf("response contextRef = %q, want @6: a restart is a new process and restamps (contract §2.4)", got.ContextRef)
	}
	if stored, err := env.fs.Get(sess.ID); err != nil || stored.ContextRef != "brief:bf-3f2a91c0@6" {
		t.Errorf("record contextRef = %q (err %v), want @6", stored.ContextRef, err)
	}
}

func TestRestartWithoutBriefPassesTheFlag(t *testing.T) {
	env := materialiseNewEnv(t, nil)
	sess := env.seed(t, nil)
	env.mat.result = api.MaterialiseResult{ContextRef: "none:declined"}
	rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions/"+sess.ID+"/restart", `{"withoutBrief":true}`, nil)
	got := materialiseDecodeSession(t, rec.Code, rec.Body.Bytes(), http.StatusOK)

	reqs := env.mat.requests()
	if len(reqs) != 1 {
		t.Fatalf("Materialise called %d times, want 1", len(reqs))
	}
	want := api.MaterialiseRequest{
		WorktreeID: catalogTestWorktree, WorktreePath: filepath.Clean(env.wtPath),
		ProjectID: catalogTestProject, BranchID: catalogTestBranch, WithoutBrief: true,
	}
	if reqs[0] != want {
		t.Errorf("Materialise got %+v, want %+v: the restart override or the path was lost", reqs[0], want)
	}
	if got.ContextRef != "none:declined" {
		t.Errorf("response contextRef = %q, want none:declined", got.ContextRef)
	}
	if stored, err := env.fs.Get(sess.ID); err != nil || stored.ContextRef != "none:declined" {
		t.Errorf("record contextRef = %q (err %v), want none:declined", stored.ContextRef, err)
	}
}

func TestRestartMaterialisesBeforeTheDestroy(t *testing.T) {
	env := materialiseNewEnv(t, nil)
	sess := env.seed(t, nil)
	rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions/"+sess.ID+"/restart", "", nil)
	materialiseDecodeSession(t, rec.Code, rec.Body.Bytes(), http.StatusOK)

	want := []string{"materialise", "backend.destroy", "store.update", "backend.create", "store.update"}
	if got := env.log.get(); !slices.Equal(got, want) {
		t.Errorf("call log = %v, want %v: Materialise comes before the old sandbox's destroy (review C1)", got, want)
	}
}

func TestRestartAcceptsAnEmptyBody(t *testing.T) {
	env := materialiseNewEnv(t, nil)
	sess := env.seed(t, nil)
	rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions/"+sess.ID+"/restart", "", nil)
	materialiseDecodeSession(t, rec.Code, rec.Body.Bytes(), http.StatusOK)
	if reqs := env.mat.requests(); len(reqs) != 1 || reqs[0].WithoutBrief {
		t.Errorf("Materialise got %+v, want one call without WithoutBrief: an empty body is {}", reqs)
	}
}

// The optional body is decoded like every other body: an unknown field is the
// 400 DecodeJSON answers (bad_request, httpx §1B), and nothing is restarted.
func TestRestartRefusesABodyWithAnUnknownField(t *testing.T) {
	env := materialiseNewEnv(t, nil)
	sess := env.seed(t, nil)
	rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions/"+sess.ID+"/restart", `{"withoutBreif":true}`, nil)
	if p := materialiseDecodeProblem(t, rec.Code, rec.Body.Bytes(), http.StatusBadRequest); p.Error != httpx.CodeBadRequest {
		t.Errorf("error = %q, want %q", p.Error, httpx.CodeBadRequest)
	}
	if got := env.log.get(); len(got) != 0 {
		t.Errorf("call log = %v, want empty", got)
	}
}

func TestRestartRefusalLeavesTheSandboxAlive(t *testing.T) {
	env := materialiseNewEnv(t, nil)
	sess := env.seed(t, nil)
	env.mat.err = &api.MaterialiseError{Reason: api.BriefReasonHubUnavailable, Message: "The brief could not be written because the Hub did not answer."}
	rec := catalogDo(t, env.h, http.MethodPost, "/v1/sessions/"+sess.ID+"/restart", "", nil)
	p := materialiseDecodeProblem(t, rec.Code, rec.Body.Bytes(), http.StatusConflict)

	if p.Error != httpx.CodeBriefNotWritten || p.Details["reason"] != api.BriefReasonHubUnavailable || p.Details["worktreeId"] != catalogTestWorktree {
		t.Errorf("problem = %+v, want brief_not_written with reason hub_unavailable and the worktree id", p)
	}
	if got := env.log.get(); slices.Contains(got, "backend.destroy") || !slices.Equal(got, []string{"materialise"}) {
		t.Errorf("call log = %v, want only [materialise]: a Hub outage must leave the running agent running", got)
	}
	if !env.be.has("sb-old") {
		t.Error("the old sandbox is gone")
	}
	stored, err := env.fs.Get(sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.SandboxID != "sb-old" || stored.Status != registry.StatusRunning || stored.ContextRef != "brief:bf-3f2a91c0@5" {
		t.Errorf("record = sandbox %q, status %q, contextRef %q; want it unchanged (sb-old, running, @5)", stored.SandboxID, stored.Status, stored.ContextRef)
	}
}

// materialiseDiscover is a RouteMounter standing in for phase 4's discover
// routes.
type materialiseDiscover struct{}

func (materialiseDiscover) Mount(handle func(pattern string, h http.Handler)) {
	handle("GET /v1/discover/ping", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("pong"))
	}))
}

func TestDiscoverRoutesAreMountedOnTheGuardedMux(t *testing.T) {
	h := catalogNewAPI(t, func(d *api.Deps) { d.Discover = materialiseDiscover{} })
	for _, path := range []string{"/v1/discover/ping", "/api/v1/discover/ping"} {
		if rec := catalogDo(t, h, http.MethodGet, path, "", nil); rec.Code != http.StatusOK || rec.Body.String() != "pong" {
			t.Errorf("GET %s = %d %q, want 200 pong", path, rec.Code, rec.Body.String())
		}
		if rec := catalogDo(t, h, http.MethodGet, path, "", map[string]string{"Origin": "https://evil.example"}); rec.Code != http.StatusForbidden {
			t.Errorf("GET %s with a foreign Origin = %d, want 403: the route escaped the origin guard", path, rec.Code)
		}
	}
}
