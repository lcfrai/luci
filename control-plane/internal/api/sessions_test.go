package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/registry/registrytest"
	"github.com/lcfrai/luci/control-plane/internal/worktree"
)

// sessionsToken and sessionsEndpoint are deliberately unmistakable strings. The leak test
// searches the marshalled bytes for them, so they must not be substrings of
// anything legitimately on the wire.
const (
	sessionsToken    = "SECRET-ATTACH-TOKEN-DO-NOT-SERIALISE"
	sessionsEndpoint = "127.0.0.1:65432"
	sessionsRef      = "CONTAINER-REF-DO-NOT-SERIALISE"

	// A conversation id is not a secret, but it is a harness implementation
	// detail the desktop must never be tempted to send back as `resume`: the
	// server resolves it from the record. Unmistakable, and UUID-shaped
	// because a record would never carry anything else.
	sessionsConversation = "c0ffee00-0000-4000-8000-5ec2e7c0ffee"
)

// sessionsFullRecord is a Session with every field set to something distinguishable,
// including all six the contract forbids on the wire. A fixture with zero
// values in the forbidden fields would pass the leak test by accident.
func sessionsFullRecord() registry.Session {
	return registry.Session{
		ID:                     "s-3f2a91c0",
		Title:                  "Claude Code · evidence-upload",
		MachineID:              "local",
		ProjectID:              "p-8c1d0e42",
		BranchID:               "b-8c1d0e42-evidence-upload",
		WorktreeID:             "w-4a77b1c9",
		Harness:                "claude-code",
		Model:                  "opus",
		Effort:                 "high",
		PermissionMode:         "ask",
		OpeningPrompt:          "add tests, then refactor",
		Command:                "claude",
		Args:                   []string{"--model", "opus", "--session-id", sessionsConversation, "add tests, then refactor"},
		ConversationID:         sessionsConversation,
		EarlierConversationIDs: []string{sessionsEarlierConversation},
		CarriedConversationID:  sessionsCarriedConversation,
		Status:                 registry.StatusRunning,
		ExitReason:             "",
		Backend:                "docker",
		SandboxID:              "s-3f2a91c0",
		SandboxRef:             sessionsRef,
		Endpoint:               sessionsEndpoint,
		AttachToken:            sessionsToken,
		CreatedAt:              time.Date(2026, 9, 18, 9, 14, 3, 0, time.UTC),
		UpdatedAt:              time.Date(2026, 9, 18, 9, 14, 7, 0, time.UTC),
		LastOutputAt:           time.Date(2026, 9, 18, 9, 14, 6, 0, time.UTC),
		AttachCount:            1,
		ContextRef:             "",
	}
}

// sessionsEarlierConversation is a conversation a restart left, for the same
// reason and in the same shape as sessionsConversation.
const sessionsEarlierConversation = "ea71e700-0000-4000-8000-5ec2e7ea71e7"

// sessionsCarriedConversation is the conversation a fork was forked from, in
// another worktree: persisted for Forget, and no more a wire value than the
// fork's own.
const sessionsCarriedConversation = "ca771ed0-0000-4000-8000-5ec2e7ca771e"

// This is the test contract §3.10 asks for and the reason api.sessionJSON
// exists. It searches the BYTES. Asserting that a field you remembered is empty
// proves nothing about the field the next person adds.
func TestSessionJSONLeaksNeitherTheTokenNorTheEndpoint(t *testing.T) {
	raw, err := json.Marshal(toJSON(sessionsFullRecord(), sessionBasePath))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for name, secret := range map[string]string{
		"attach token":         sessionsToken,
		"conversation":         sessionsConversation,
		"earlier conversation": sessionsEarlierConversation,
		"carried conversation": sessionsCarriedConversation,
		"endpoint":             sessionsEndpoint,
		"sandbox ref":          sessionsRef,
	} {
		if strings.Contains(body, secret) {
			t.Errorf("the %s reached the wire: %s", name, body)
		}
	}
	// command and args are forbidden for a different reason: they would put the
	// opening prompt in two places and invite the desktop to rebuild argv.
	for _, fragment := range []string{`"command"`, `"args"`, `"sandboxId"`, `"sandboxRef"`, `"endpoint"`, `"attachToken"`, `"conversationId"`, `"earlierConversationIds"`, `"carriedConversationId"`} {
		if strings.Contains(body, fragment) {
			t.Errorf("forbidden key %s is present: %s", fragment, body)
		}
	}
}

// The key set, not just the values. A field added to registry.Session and
// copied into sessionJSON without thought fails here, which is the point.
func TestSessionJSONCarriesExactlyTheContractsTwentyOneKeys(t *testing.T) {
	raw, err := json.Marshal(toJSON(sessionsFullRecord(), sessionBasePath))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := []string{
		"id", "title", "machineId", "projectId", "branchId", "worktreeId",
		"harness", "model", "effort", "permissionMode", "openingPrompt",
		"status", "exitReason", "backend", "isolation",
		"createdAt", "updatedAt", "lastOutputAt", "attachCount", "attachPath",
		"contextRef",
	}
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sorted := append([]string(nil), want...)
	sort.Strings(sorted)
	if strings.Join(keys, ",") != strings.Join(sorted, ",") {
		t.Fatalf("keys =\n\t%v\nwant\n\t%v", keys, sorted)
	}
}

func TestSessionJSONBuildsTheAttachPathAndTheIsolation(t *testing.T) {
	got := toJSON(sessionsFullRecord(), sessionBasePath)
	if want := "/v1/sessions/s-3f2a91c0/attach"; got.AttachPath != want {
		t.Errorf("attachPath = %q, want %q", got.AttachPath, want)
	}
	// toJSON delegates to task 10's isolationFor, which is the ONLY mapping of
	// contract §3.4's three-value enum in this package — group-C ruling 6. The
	// values are therefore task 10's: "container" for docker, "microvm" for
	// kata, and "none" for anything else. A second mapping here that answered
	// "unknown" would invent a fourth value and make the session surface and
	// the machine surface disagree about the same backend.
	for kind, want := range map[string]string{"docker": "container", "kata": "microvm", "": "none", "podman": "none"} {
		s := sessionsFullRecord()
		s.Backend = kind
		if got := toJSON(s, sessionBasePath).Isolation; got != want {
			t.Errorf("isolation for backend %q = %q, want %q", kind, got, want)
		}
	}
}

// A zero lastOutputAt must be "" and never "0001-01-01T00:00:00Z", which every
// date formatter in the cockpit would render as a day in the year 1.
func TestSessionJSONFormatsTimesAndBlanksAZeroLastOutput(t *testing.T) {
	s := sessionsFullRecord()
	s.LastOutputAt = time.Time{}
	got := toJSON(s, sessionBasePath)
	if got.LastOutputAt != "" {
		t.Errorf("lastOutputAt = %q, want the empty string", got.LastOutputAt)
	}
	if got.CreatedAt != "2026-09-18T09:14:03Z" {
		t.Errorf("createdAt = %q, want 2026-09-18T09:14:03Z", got.CreatedAt)
	}
	// Non-UTC in, UTC out: the registry stamps UTC but a restored file or a
	// hand-built record can carry anything.
	s.UpdatedAt = time.Date(2026, 9, 18, 19, 14, 7, 0, time.FixedZone("AEST", 10*3600))
	if got := toJSON(s, sessionBasePath).UpdatedAt; got != "2026-09-18T09:14:07Z" {
		t.Errorf("updatedAt = %q, want the UTC rendering 2026-09-18T09:14:07Z", got)
	}
}

// sessionsStore is an in-memory registry.Store for this package's tests. It is held
// to registrytest.RunStoreContract below, so it cannot drift from FileStore by
// being quietly more forgiving.
type sessionsStore struct{ state *sessionsStoreState }

type sessionsStoreState struct {
	mu    sync.Mutex
	byID  map[string]registry.Session
	order []string
	flags map[string]registry.WorktreeFlags

	// createErr, when set, is what Create returns instead of doing anything.
	// It exists for one test: the handler must answer 400 to
	// registry.ErrInvalidID and 409 to registry.ErrExists, and neither is
	// reachable through the real path because the id comes from ids.New over
	// crypto/rand. It is nil for every other test and for the whole
	// registrytest suite, so the double still faces the contract unchanged.
	createErr error
}

// sessionsStores makes "open the same path twice" mean "the same state", which
// registrytest.NewStoreFunc requires: several of its tests prove a write
// persisted by reopening. Paths come from t.TempDir, so no two tests collide.
var sessionsStores = struct {
	mu     sync.Mutex
	byPath map[string]*sessionsStoreState
}{byPath: map[string]*sessionsStoreState{}}

func sessionsNewStore(path string) (registry.Store, error) {
	sessionsStores.mu.Lock()
	defer sessionsStores.mu.Unlock()
	st, ok := sessionsStores.byPath[path]
	if !ok {
		st = &sessionsStoreState{byID: map[string]registry.Session{}}
		sessionsStores.byPath[path] = st
	}
	return &sessionsStore{state: st}, nil
}

func (m *sessionsStore) Create(s registry.Session) error {
	if s.ID == "" {
		return registry.ErrInvalidID
	}
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	if m.state.createErr != nil {
		return m.state.createErr
	}
	if _, exists := m.state.byID[s.ID]; exists {
		return registry.ErrExists
	}
	now := time.Now().UTC()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	s.CreatedAt = s.CreatedAt.UTC()
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = s.CreatedAt
	}
	s.UpdatedAt = s.UpdatedAt.UTC()
	m.state.byID[s.ID] = s.Clone()
	m.state.order = append(m.state.order, s.ID)
	return nil
}

func (m *sessionsStore) Get(id string) (registry.Session, error) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	s, ok := m.state.byID[id]
	if !ok {
		return registry.Session{}, registry.ErrNotFound
	}
	return s.Clone(), nil
}

func (m *sessionsStore) List() ([]registry.Session, error) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	out := make([]registry.Session, 0, len(m.state.byID))
	for _, id := range m.state.order {
		if s, ok := m.state.byID[id]; ok {
			out = append(out, s.Clone())
		}
	}
	// Ascending by CreatedAt with the id as the tie-break, which is the order
	// registrytest asserts and the order FileStore keeps.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (m *sessionsStore) Update(id string, mutate func(*registry.Session) error) (registry.Session, error) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	cur, ok := m.state.byID[id]
	if !ok {
		return registry.Session{}, registry.ErrNotFound
	}
	// First clone: mutate never touches the stored value, so a mutate that
	// errors leaves nothing behind.
	work := cur.Clone()
	if err := mutate(&work); err != nil {
		return registry.Session{}, err
	}
	// The id is the map key, so committing a rename would leave the index keyed
	// by one id and holding a record carrying another. FileStore refuses it and
	// registrytest's UpdateStampsInUTCAndRefusesAnIDChange holds both
	// implementations to that; a double that quietly allowed it would be the
	// drift the suite exists to catch.
	if work.ID != cur.ID {
		return registry.Session{}, errors.New("api: Update " + id + ": mutate changed the session id to " + work.ID)
	}
	// Both stamps belong to the store, exactly as in FileStore: CreatedAt is a
	// fact about the past and UpdatedAt is what the cockpit sorts and ages by.
	work.CreatedAt = cur.CreatedAt
	work.UpdatedAt = time.Now().UTC()
	// Second clone: the caller's pointer to work outlives this call and
	// registrytest writes through it afterwards. Storing work itself would let
	// that write land, and storing a shallow copy would still share Args.
	m.state.byID[id] = work.Clone()
	return work.Clone(), nil
}

func (m *sessionsStore) Delete(id string) error {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	if _, ok := m.state.byID[id]; !ok {
		return registry.ErrNotFound
	}
	delete(m.state.byID, id)
	for i, existing := range m.state.order {
		if existing == id {
			m.state.order = append(m.state.order[:i], m.state.order[i+1:]...)
			break
		}
	}
	return nil
}

// The worktree ready flag, so one double can serve both Deps.Store and
// Deps.Flags. Contract §2.8 keeps these off Store on purpose and task 10's
// New rejects a nil Flags, so without them every test here would need a
// second, real FileStore just to get past validation.
func (m *sessionsStore) WorktreeFlagsSnapshot() map[string]registry.WorktreeFlags {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	out := make(map[string]registry.WorktreeFlags, len(m.state.flags))
	for id, f := range m.state.flags {
		out[id] = f
	}
	return out
}

func (m *sessionsStore) SetWorktreeReady(worktreeID string, ready bool) (registry.WorktreeFlags, error) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	if m.state.flags == nil {
		m.state.flags = map[string]registry.WorktreeFlags{}
	}
	f := registry.WorktreeFlags{Ready: ready}
	m.state.flags[worktreeID] = f
	return f, nil
}

var (
	_ registry.Store             = (*sessionsStore)(nil)
	_ registry.WorktreeFlagStore = (*sessionsStore)(nil)
)

// The double is held to the same suite FileStore is. Without this, the two
// implementations are two sets of tests that drifted.
func TestSessionsStoreMeetsTheStoreContract(t *testing.T) {
	registrytest.RunStoreContract(t, sessionsNewStore)
}

// sessionsMustStore is the fixture the handler tests build on. It returns the concrete
// type because Deps.Flags needs registry.WorktreeFlagStore, which registry.Store
// does not imply.
func sessionsMustStore(t *testing.T) *sessionsStore {
	t.Helper()
	st, err := sessionsNewStore(registrytest.StorePath(t))
	if err != nil {
		t.Fatalf("open the in-memory store: %v", err)
	}
	return st.(*sessionsStore)
}

// sessionsSeed puts a session straight into the store, for the verbs that need one to
// already exist. errors is imported here and used by the later steps' tests.
func sessionsSeed(t *testing.T, st registry.Store, s registry.Session) registry.Session {
	t.Helper()
	if err := st.Create(s); err != nil && !errors.Is(err, registry.ErrExists) {
		t.Fatalf("seed %s: %v", s.ID, err)
	}
	got, err := st.Get(s.ID)
	if err != nil {
		t.Fatalf("read back %s: %v", s.ID, err)
	}
	return got
}

// sessionsBackend records what was asked of it and answers with whatever the test
// configured. Counting destroys is the point: several tests below assert zero.
type sessionsBackend struct {
	mu sync.Mutex

	kind       backend.Kind
	createErr  error
	destroyErr error
	endpoint   string

	// nonLoopback is what Create reports on backend.Sandbox.NonLoopback. Task
	// 5 sets that field only in List today, but the rule this phase mandates
	// is about what the API does with it wherever it appears, and a rule
	// asserted against a fake that can never set the field is pinned by
	// nothing. Endpoint is blank whenever it is true, which is the invariant
	// backend.Sandbox documents.
	nonLoopback bool

	// onCreate runs at the top of Create, before anything is recorded. It is
	// how a test reads the registry from INSIDE the window between a record
	// being written and its container existing — the only place the ordering
	// of those two is observable, and the window task 13's reaper sweeps.
	onCreate func(spec backend.Spec)

	// onDestroy runs at the top of Destroy, for the same reason: what a
	// sandbox does in its last moments — its CLI writing a transcript's first
	// line — is observable only from inside the call that ends it.
	onDestroy func(id string)

	specs    []backend.Spec
	destroys []string
}

func sessionsNewBackend() *sessionsBackend {
	return &sessionsBackend{kind: backend.KindDocker, endpoint: "127.0.0.1:49200"}
}

func (f *sessionsBackend) Kind() backend.Kind { return f.kind }

func (f *sessionsBackend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
	// The hook runs before the lock is taken and before anything is recorded,
	// so it sees what a sweep landing mid-create would see and a hook that
	// calls back into this fake cannot deadlock.
	f.mu.Lock()
	hook := f.onCreate
	f.mu.Unlock()
	if hook != nil {
		hook(spec)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.specs = append(f.specs, spec)
	if f.createErr != nil {
		return backend.Sandbox{}, f.createErr
	}
	endpoint := f.endpoint
	if f.nonLoopback {
		endpoint = ""
	}
	return backend.Sandbox{
		ID:          spec.ID,
		Kind:        f.kind,
		Ref:         "ref-" + spec.ID,
		Endpoint:    endpoint,
		NonLoopback: f.nonLoopback,
		State:       backend.StateStarting,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func (f *sessionsBackend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	return backend.Sandbox{ID: id, Kind: f.kind, Endpoint: f.endpoint, State: backend.StateRunning}, nil
}

func (f *sessionsBackend) List(ctx context.Context) ([]backend.Sandbox, error) { return nil, nil }

// Attach is here only to satisfy backend.Backend. Nothing in sessions.go
// attaches — task 12 owns the relay — so reaching this is a defect in this
// task, and a panic says so louder than a nil return.
func (f *sessionsBackend) Attach(ctx context.Context, id, token string) (*websocket.Conn, error) {
	panic("sessions.go must never attach; task 12 owns the relay")
}

func (f *sessionsBackend) Destroy(ctx context.Context, id string) error {
	f.mu.Lock()
	hook := f.onDestroy
	f.mu.Unlock()
	if hook != nil {
		hook(id)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.destroys = append(f.destroys, id)
	return f.destroyErr
}

func (f *sessionsBackend) Close() error { return nil }

func (f *sessionsBackend) spec(t *testing.T, i int) backend.Spec {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if i >= len(f.specs) {
		t.Fatalf("backend.Create was called %d times, wanted at least %d", len(f.specs), i+1)
	}
	return f.specs[i]
}

func (f *sessionsBackend) destroyCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.destroys)
}

// sessionsLocator stands in for *project.Scanner and satisfies task 10's
// api.Enumerator. Branches is part of that interface and unused here; it is
// implemented rather than panicking so a future catalog test can share this.
type sessionsLocator struct {
	projects  []project.Project
	branches  map[string][]project.Branch   // keyed by project id
	worktrees map[string][]project.Worktree // keyed by project id
	err       error
}

func (f *sessionsLocator) Projects(ctx context.Context) ([]project.Project, error) {
	return f.projects, f.err
}

func (f *sessionsLocator) Branches(ctx context.Context, projectID string) ([]project.Branch, error) {
	return f.branches[projectID], f.err
}

func (f *sessionsLocator) Worktrees(ctx context.Context, projectID string) ([]project.Worktree, error) {
	return f.worktrees[projectID], f.err
}

var _ Enumerator = (*sessionsLocator)(nil)

// sessionsEnv is everything a test needs, kept together so a test can reach
// past the handler when it has to.
type sessionsEnv struct {
	api *API
	mux *routes
	be  *sessionsBackend
	st  *sessionsStore // nil when the env was built over another store
	wt  worktree.Static
	dir string
	loc *sessionsLocator

	// conversations is Deps.ConversationsDir: the root, not one worktree's
	// store.
	conversations string
}

// sessionsNewEnv is the env almost every test wants: a fresh directory and the
// in-memory store double.
func sessionsNewEnv(t *testing.T) *sessionsEnv {
	t.Helper()
	st := sessionsMustStore(t)
	env := sessionsNewEnvOver(t, t.TempDir(), st, st)
	env.st = st
	return env
}

// sessionsNewEnvOver is the ONE place that constructs an API. Deps carries
// eight fields New refuses to start without — Backend, BackendReady, Store,
// Flags, Projects, Worktrees, Origin and ConversationsDir — so all eight are
// here even though two, BackendReady and Flags, belong to task 10's surface and
// go unexercised.
//
// dir and the store are parameters for one kind of test: a control-plane
// restart, which is a second API over the same state directory and the same
// registry file.
func sessionsNewEnvOver(t *testing.T, dir string, st registry.Store, flags registry.WorktreeFlagStore) *sessionsEnv {
	t.Helper()
	return sessionsNewEnvOverWith(t, dir, st, flags, nil)
}

// sessionsLineEndings is a Deps.GitLineEndings that always gives the same
// answer, as the host's own config does.
func sessionsLineEndings(eol GitLineEndings) func(context.Context) (GitLineEndings, error) {
	return func(context.Context) (GitLineEndings, error) { return eol, nil }
}

// sessionsGlobalFiles is a Deps.GitGlobalFiles that always gives the same
// answer. The zero answer is a host whose git reads no global file.
func sessionsGlobalFiles(files GitGlobalFiles) func(context.Context) (GitGlobalFiles, error) {
	return func(context.Context) (GitGlobalFiles, error) { return files, nil }
}

// sessionsNewEnvOverWith is sessionsNewEnvOver with a hook over the Deps before
// New sees them, for the optional fields — GitIdentity, CABundle, Log — that
// most tests leave at their zero value. Still the one construction site: the
// hook edits these Deps, it does not build its own.
func sessionsNewEnvOverWith(t *testing.T, dir string, st registry.Store, flags registry.WorktreeFlagStore, tweak func(*Deps)) *sessionsEnv {
	t.Helper()
	wtDir := filepath.Join(dir, "evidence-upload")
	if err := os.MkdirAll(wtDir, 0o700); err != nil {
		t.Fatalf("make the worktree dir: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	env := &sessionsEnv{
		be:  sessionsNewBackend(),
		wt:  worktree.Static{"w-4a77b1c9": wtDir},
		dir: wtDir,
		// A sibling of the projects directory, never inside the worktree: a
		// store inside the worktree would be committed with the code.
		conversations: filepath.Join(dir, "conversations"),
		loc: &sessionsLocator{
			projects: []project.Project{{ID: "p-8c1d0e42", Name: "luci", Path: dir}},
			branches: map[string][]project.Branch{
				"p-8c1d0e42": {{ID: "b-8c1d0e42-evidence-upload", ProjectID: "p-8c1d0e42", Name: "evidence-upload"}},
			},
			worktrees: map[string][]project.Worktree{
				"p-8c1d0e42": {{
					ID: "w-4a77b1c9", ProjectID: "p-8c1d0e42",
					BranchID: "b-8c1d0e42-evidence-upload",
					Name:     "evidence-upload", Path: wtDir,
				}},
			},
		},
	}
	deps := Deps{
		Version:          "test",
		Backend:          env.be,
		BackendReady:     func() bool { return true },
		Store:            st,
		Flags:            flags,
		Projects:         env.loc,
		Worktrees:        env.wt,
		Origin:           httpx.NewOriginPolicy(nil, log),
		Log:              log,
		Image:            "luci/sandbox:dev",
		Resources:        backend.DefaultResources,
		ReadyTimeout:     50 * time.Millisecond,
		ConversationsDir: env.conversations,
		GitLineEndings:   sessionsLineEndings(GitLineEndings{AutoCRLF: "false", EOL: "lf"}),
		GitGlobalFiles:   sessionsGlobalFiles(GitGlobalFiles{}),
	}
	if tweak != nil {
		tweak(&deps)
	}
	a, err := New(deps)
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	env.api = a
	// A bare recorder, not the handler New returns: that one is wrapped in task
	// 9's loopback Host guard and its origin check, which are tasks 9's and
	// 10's to test. It is a *routes and not an *http.ServeMux because group-C
	// §1A fixes that signature — the recorder is how contract §10a's audit sees
	// a route at all.
	env.mux = newRoutes()
	a.registerSessions(env.mux)
	return env
}

// sessionsTarget makes every request absolute. httptest.NewRequest fills in
// Host: example.com for a path-only target, which task 9's loopback guard
// correctly answers 403 forbidden_host to — a failure that reads like a routing
// bug in whichever test forgot.
func sessionsTarget(path string) string { return "http://127.0.0.1:8730" + path }

// do drives the recorder this task registered, not task 10's. Origin and Host
// middleware belong to tasks 9 and 10 and are tested there.
func (e *sessionsEnv) do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, sessionsTarget(target), nil)
	} else {
		r = httptest.NewRequest(method, sessionsTarget(target), bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, r)
	return rec
}

// sessionsDecodeOne reads one session object, failing the test with the body when
// the status is not what the caller expected.
func sessionsDecodeOne(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) sessionJSON {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	var got sessionJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode the session object: %v; body = %s", err, rec.Body.String())
	}
	return got
}

// sessionsProblemCode reads the error code out of a failure response.
func sessionsProblemCode(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) string {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, wantStatus, rec.Body.String())
	}
	var p struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode the problem body: %v; body = %s", err, rec.Body.String())
	}
	return p.Error
}

// sessionsStubReady replaces the readiness call for one test. Tests using it must not
// call t.Parallel.
func sessionsStubReady(t *testing.T, err error) {
	t.Helper()
	prev := sessionWaitReady
	sessionWaitReady = func(ctx context.Context, endpoint string, timeout time.Duration) error { return err }
	t.Cleanup(func() { sessionWaitReady = prev })
}

// sessionsStubReadyCalls is sessionsStubReady with a counter, for the tests that
// must prove the readiness wait was never ENTERED rather than that it returned
// something harmless. Stubbing the wait to nil and asserting the happy outcome
// cannot tell "the branch was skipped" from "the branch ran and did nothing",
// which is how the first ruling-10 test was blind to the case it was named for.
func sessionsStubReadyCalls(t *testing.T, err error) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	prev := sessionWaitReady
	sessionWaitReady = func(ctx context.Context, endpoint string, timeout time.Duration) error {
		calls.Add(1)
		return err
	}
	t.Cleanup(func() { sessionWaitReady = prev })
	return &calls
}

// sessionsWatchDone reports each finished readiness watch. Drain one value per watch
// before the test returns, or Cleanup's write races the goroutine's read.
func sessionsWatchDone(t *testing.T) <-chan string {
	t.Helper()
	ch := make(chan string, 8)
	sessionReadyDone = func(id string) { ch <- id }
	t.Cleanup(func() { sessionReadyDone = nil })
	return ch
}

func sessionsAwaitWatch(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case id := <-ch:
		return id
	case <-time.After(2 * time.Second):
		t.Fatal("the readiness watch never finished")
		return ""
	}
}

const sessionsCreateBody = `{"worktreeId":"w-4a77b1c9","harness":"claude-code","model":"opus","effort":"high","openingPrompt":"add tests, then refactor"}`

func TestCreateReturns201StartingWithALocationAndNoSecrets(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)

	rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
	got := sessionsDecodeOne(t, rec, http.StatusCreated)

	// Contract §3.9: 201 as soon as the record exists and the sandbox has been
	// created, NOT after it is ready. A create that blocks for ReadyTimeout is
	// a cockpit that looks frozen.
	if got.Status != string(registry.StatusStarting) {
		t.Errorf("status = %q, want %q", got.Status, registry.StatusStarting)
	}
	if want := sessionBasePath + "/sessions/" + got.ID; rec.Header().Get("Location") != want {
		t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), want)
	}
	if got.AttachPath != sessionBasePath+"/sessions/"+got.ID+"/attach" {
		t.Errorf("attachPath = %q", got.AttachPath)
	}
	if got.MachineID != "local" || got.Isolation != "container" || got.Backend != "docker" {
		t.Errorf("machine/isolation/backend = %q/%q/%q", got.MachineID, got.Isolation, got.Backend)
	}
	// Derived, per §3.9: the harness's display name, a middle dot, the worktree
	// directory's base name.
	if want := "Claude Code · evidence-upload"; got.Title != want {
		t.Errorf("title = %q, want %q", got.Title, want)
	}
	if got.ProjectID != "p-8c1d0e42" || got.BranchID != "b-8c1d0e42-evidence-upload" {
		t.Errorf("projectId/branchId = %q/%q; the locator was not consulted", got.ProjectID, got.BranchID)
	}

	stored, err := env.st.Get(got.ID)
	if err != nil {
		t.Fatalf("the record does not exist after a 201: %v", err)
	}
	if stored.AttachToken == "" {
		t.Error("no attach token was minted; sandboxd refuses to start without one")
	}
	if strings.Contains(rec.Body.String(), stored.AttachToken) {
		t.Errorf("the minted token reached the response body: %s", rec.Body.String())
	}
	sessionsAwaitWatch(t, done)
}

func TestCreateBuildsASpecThatMountsTheWorktreeAndCarriesTheArgv(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)

	got := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	spec := env.be.spec(t, 0)

	if spec.Image != "luci/sandbox:dev" {
		t.Errorf("Image = %q", spec.Image)
	}
	if spec.SessionID != got.ID {
		t.Errorf("Spec.SessionID = %q, want the session id %q; the label is how the reaper names an orphan", spec.SessionID, got.ID)
	}
	if spec.WorkDir != sessionWorkDir {
		t.Errorf("WorkDir = %q, want %q", spec.WorkDir, sessionWorkDir)
	}
	// Two mounts for a harness with a conversation: the worktree, and that
	// worktree's conversation store. sessions_resume_test.go pins the second.
	if len(spec.Mounts) != 2 || spec.Mounts[0].HostPath != env.dir || spec.Mounts[0].ContainerPath != sessionWorkDir {
		t.Fatalf("Mounts = %#v, want the worktree %q at %q and then its conversation store", spec.Mounts, env.dir, sessionWorkDir)
	}
	if spec.Mounts[0].ReadOnly {
		t.Error("the worktree is mounted read-only; the agent cannot write the code it was asked to write")
	}
	if spec.Command != "claude" {
		t.Errorf("Command = %q, want claude", spec.Command)
	}
	stored, err := env.st.Get(got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if strings.Join(spec.Args, "\x00") != strings.Join([]string{"--append-system-prompt", harness.ProjectBriefPrompt, "--model", "opus", "--effort", "high", "--session-id", stored.ConversationID, "add tests, then refactor"}, "\x00") {
		t.Errorf("Args = %#v", spec.Args)
	}
	if spec.AttachToken == "" {
		t.Error("Spec.AttachToken is empty; the sandbox would exit 1 at startup")
	}
	// AMENDED by the sandbox git fix: with no identity and no CA bundle the only
	// extra environment is git's own configuration, whose values
	// sessions_sandboxenv_test.go pins. A LUCI_* key here would be
	// rejected by EnvEntries anyway.
	for k := range spec.Env {
		if !strings.HasPrefix(k, "GIT_CONFIG_") {
			t.Errorf("Env carries %q = %q; with no identity and no CA bundle only git's configuration is set", k, spec.Env[k])
		}
	}
	sessionsAwaitWatch(t, done)
}

// Contract §5.1 step 1: the worktree is resolved FIRST, so a bad id costs
// nothing. Nothing is created and nothing is recorded.
func TestCreateResolvesTheWorktreeBeforeAnythingIsCreated(t *testing.T) {
	env := sessionsNewEnv(t)

	rec := env.do(t, http.MethodPost, "/v1/sessions", `{"worktreeId":"w-nope"}`)
	if code := sessionsProblemCode(t, rec, http.StatusNotFound); code != "worktree_not_found" {
		t.Errorf("error = %q, want worktree_not_found", code)
	}
	if n := len(env.be.specs); n != 0 {
		t.Errorf("backend.Create was called %d times for an unknown worktree", n)
	}
	list, err := env.st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("a record was written for an unknown worktree: %#v", list)
	}
	if env.be.destroyCount() != 0 {
		t.Errorf("Destroy was called %d times", env.be.destroyCount())
	}
}

// Contract §5.1 step 4: the record exists before the sandbox does. When
// backend.Create fails, that record is the only evidence of what happened.
func TestCreateRecordsTheSessionBeforeTheSandboxAndKeepsItOnFailure(t *testing.T) {
	env := sessionsNewEnv(t)
	env.be.createErr = backend.ErrUnavailable

	rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
	if code := sessionsProblemCode(t, rec, http.StatusServiceUnavailable); code != "backend_unavailable" {
		t.Errorf("error = %q, want backend_unavailable", code)
	}
	list, err := env.st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d sessions, want 1 — without the record the container that may exist is an orphan nobody can name", len(list))
	}
	if list[0].Status != registry.StatusExited {
		t.Errorf("status = %q, want exited", list[0].Status)
	}
	if list[0].ExitReason == "" {
		t.Error("exitReason is empty; the user is owed a reason")
	}
	// SandboxID written BEFORE backend.Create is the entire reason for the
	// ordering, and group-C ruling 9 is written against it: without it a
	// container the daemon did create before failing has no record naming it,
	// and task 13's reaper cannot tell it from someone else's container.
	if list[0].SandboxID == "" {
		t.Error("sandboxId is empty; the record cannot name the container that may exist, which is what the record-first ordering is for")
	}
}

// 201 is written before the sandbox is ready, not after — contract §3.9. The
// response carries "starting" either way, so status alone cannot tell the two
// apart: what distinguishes them is that the handler RETURNS while the wait is
// still in flight. The stub releases on its own after two seconds so a
// synchronous create fails this test rather than hanging it.
func TestCreateReturnsBeforeTheSandboxIsReady(t *testing.T) {
	env := sessionsNewEnv(t)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	prev := sessionWaitReady
	sessionWaitReady = func(ctx context.Context, endpoint string, timeout time.Duration) error {
		entered <- struct{}{}
		select {
		case <-release:
		case <-time.After(2 * time.Second):
		}
		return nil
	}
	t.Cleanup(func() { sessionWaitReady = prev })
	done := sessionsWatchDone(t)

	start := time.Now()
	got := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Errorf("create took %s; it waited for readiness, and a cockpit that blocks for ReadyTimeout looks frozen", elapsed)
	}
	if got.Status != string(registry.StatusStarting) {
		t.Errorf("status = %q, want starting", got.Status)
	}
	// And the watch really is running — otherwise the timing above would be
	// explained by there being nothing to wait for.
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the readiness watch never started, so the timing above proves nothing")
	}
	close(release)
	sessionsAwaitWatch(t, done)
}

func TestCreateRejectsAMissingWorktreeID(t *testing.T) {
	env := sessionsNewEnv(t)
	if code := sessionsProblemCode(t, env.do(t, http.MethodPost, "/v1/sessions", `{}`), http.StatusBadRequest); code != "bad_request" {
		t.Errorf("error = %q, want bad_request", code)
	}
	// An unknown field is the decoder's rejection, not this handler's — and
	// since group-C §1B it is bad_request rather than invalid_json, because
	// well-formed JSON asking for something that does not exist is not a parse
	// failure. The plan text predates that ruling; the ruling wins.
	rec := env.do(t, http.MethodPost, "/v1/sessions", `{"worktreeId":"w-4a77b1c9","colour":"red"}`)
	if code := sessionsProblemCode(t, rec, http.StatusBadRequest); code != "bad_request" {
		t.Errorf("error = %q, want bad_request", code)
	}
	if !strings.Contains(rec.Body.String(), "colour") {
		t.Errorf("body = %s, want it to name the offending field", rec.Body.String())
	}
	// The other half of §1B on this surface: a body that genuinely does not
	// parse is still invalid_json, so the two codes keep meaning two things.
	if code := sessionsProblemCode(t, env.do(t, http.MethodPost, "/v1/sessions", `{"worktreeId":`), http.StatusBadRequest); code != "invalid_json" {
		t.Errorf("error = %q, want invalid_json", code)
	}
}

// Each harness failure is a different status, and the difference matters: 400
// means the request was wrong, 409 means the request was reasonable and the
// image is not ready for it.
func TestCreateMapsEveryHarnessFailureToItsOwnStatus(t *testing.T) {
	for name, tc := range map[string]struct {
		body       string
		wantStatus int
		wantCode   string
	}{
		"unknown harness": {
			body:       `{"worktreeId":"w-4a77b1c9","harness":"emacs"}`,
			wantStatus: http.StatusBadRequest, wantCode: "unknown_harness",
		},
		"not in the image": {
			body:       `{"worktreeId":"w-4a77b1c9","harness":"gemini-cli"}`,
			wantStatus: http.StatusConflict, wantCode: "harness_unavailable",
		},
		"option the harness cannot take": {
			body:       `{"worktreeId":"w-4a77b1c9","harness":"shell","openingPrompt":"rm -rf /"}`,
			wantStatus: http.StatusBadRequest, wantCode: "unsupported_option",
		},
	} {
		t.Run(name, func(t *testing.T) {
			env := sessionsNewEnv(t)
			rec := env.do(t, http.MethodPost, "/v1/sessions", tc.body)
			if code := sessionsProblemCode(t, rec, tc.wantStatus); code != tc.wantCode {
				t.Errorf("error = %q, want %q", code, tc.wantCode)
			}
			if n := len(env.be.specs); n != 0 {
				t.Errorf("backend.Create was called %d times; the harness is resolved before anything is created", n)
			}
		})
	}
}

// sessionsStarting is the fixture the watch tests drive.
func sessionsStarting(id string) registry.Session {
	// Model agrees with Args, as it does on every record create writes: a
	// restart rebuilds argv from the configuration fields, so a fixture whose
	// fields contradict its argv would test a record production cannot make.
	return registry.Session{
		ID: id, Title: "t", MachineID: "local", WorktreeID: "w-4a77b1c9",
		Harness: "claude-code", Model: "opus", Command: "claude", Args: []string{"--model", "opus"},
		Status: registry.StatusStarting, Backend: "docker",
		SandboxID: id, SandboxRef: "ref-" + id, AttachToken: sessionsToken,
		CreatedAt: time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC),
	}
}

func TestWatchSessionReadyFlipsStartingToRunning(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	sessionsSeed(t, env.st, sessionsStarting("s-00000001"))

	env.api.watchSessionReady("s-00000001", "s-00000001", "127.0.0.1:49200")

	got, err := env.st.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != registry.StatusRunning {
		t.Errorf("status = %q, want running", got.Status)
	}
	if got.Endpoint != "127.0.0.1:49200" {
		t.Errorf("endpoint = %q; the watch records it so a restart of the control plane can re-read it", got.Endpoint)
	}
	if env.be.destroyCount() != 0 {
		t.Errorf("Destroy was called %d times on the happy path", env.be.destroyCount())
	}
}

func TestWatchSessionReadyDestroysAndMarksExitedWhenTheSandboxNeverAnswers(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, context.DeadlineExceeded)
	sessionsSeed(t, env.st, sessionsStarting("s-00000001"))

	env.api.watchSessionReady("s-00000001", "s-00000001", "127.0.0.1:49200")

	got, err := env.st.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != registry.StatusExited {
		t.Errorf("status = %q, want exited", got.Status)
	}
	if !strings.Contains(got.ExitReason, "ready") {
		t.Errorf("exitReason = %q, want it to say the sandbox never became ready", got.ExitReason)
	}
	if env.be.destroyCount() != 1 {
		t.Fatalf("Destroy called %d times, want 1 — a sandbox that never answered has no session and would otherwise be an orphan", env.be.destroyCount())
	}
}

// The dangerous case. A restart replaces the sandbox while an old watch is
// still in flight; that watch must touch neither the record nor the NEW
// sandbox. It destroys only the sandbox it was given.
func TestWatchSessionReadyNeverTouchesASandboxItDoesNotOwn(t *testing.T) {
	for name, readyErr := range map[string]error{"ready": nil, "never ready": context.DeadlineExceeded} {
		t.Run(name, func(t *testing.T) {
			env := sessionsNewEnv(t)
			sessionsStubReady(t, readyErr)
			s := sessionsStarting("s-00000001")
			s.SandboxID = "s-newsandbox"
			s.SandboxRef = "ref-s-newsandbox"
			sessionsSeed(t, env.st, s)

			env.api.watchSessionReady("s-00000001", "s-oldsandbox", "127.0.0.1:49200")

			got, err := env.st.Get("s-00000001")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.Status != registry.StatusStarting {
				t.Errorf("status = %q, want starting — a stale watch moved a restarted session", got.Status)
			}
			if got.SandboxID != "s-newsandbox" {
				t.Errorf("sandboxId = %q, want s-newsandbox", got.SandboxID)
			}
			for _, id := range env.be.destroys {
				if id == "s-newsandbox" {
					t.Fatal("the stale watch destroyed the sandbox the restart had just created")
				}
			}
		})
	}
}

// Task 5's amendment to contract §2.4 keeps a LAN-published container visible
// so task 13's reaper can destroy it. It is NOT a session status: the process
// is doing whatever it is doing until the reaper acts, and reporting "exited"
// for a running container would be a fabricated status. Group-C ruling 10
// makes the reaper the only actor that resolves it, and this test is what
// stops this task resolving it instead. The fake SETS the field — a test that
// left it false would assert the rule and pin nothing.
//
// THE WAIT IS STUBBED TO FAIL, WHICH IS THE POINT. The first version of this
// test stubbed it to nil, so it proved only that CREATE does not fabricate a
// status and was structurally blind to the watch — the one thing that can.
// With DeadlineExceeded, a watch that ran would destroy the sandbox and write
// exited, and the two assertions below would catch it.
func TestCreateDoesNotTranslateNonLoopbackIntoAStatus(t *testing.T) {
	env := sessionsNewEnv(t)
	env.be.nonLoopback = true
	calls := sessionsStubReadyCalls(t, context.DeadlineExceeded)
	done := sessionsWatchDone(t)

	got := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	if got.Status != string(registry.StatusStarting) {
		t.Errorf("status = %q, want starting", got.Status)
	}
	if got.ExitReason != "" {
		t.Errorf("exitReason = %q; a LAN-published sandbox is not an exit", got.ExitReason)
	}
	if spec := env.be.spec(t, 0); spec.ID != got.ID {
		t.Fatalf("the sandbox was created as %q, want %q", spec.ID, got.ID)
	}
	sessionsAwaitWatch(t, done)

	if env.be.destroyCount() != 0 {
		t.Errorf("Destroy was called %d times; ruling 10 gives that job to task 13's reaper, which logs it distinctly", env.be.destroyCount())
	}
	// There is no endpoint to wait on, so waiting on it is not a slow path, it
	// is a wrong one: WaitReady would build http:///healthz and fail every
	// attempt until the timeout, and the watch would read that as "never became
	// ready" and destroy a container that is running.
	if n := calls.Load(); n != 0 {
		t.Errorf("the readiness wait ran %d times against an empty endpoint", n)
	}

	// The record is what the reaper reads, so it matters that the endpoint is
	// blank there too rather than carrying an address nothing can reach.
	stored, err := env.st.Get(got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Endpoint != "" {
		t.Errorf("endpoint = %q, want it empty — backend.Sandbox blanks it whenever NonLoopback is set", stored.Endpoint)
	}
	if stored.Status != registry.StatusStarting {
		t.Errorf("status = %q, want starting; the record was moved for a container that is running", stored.Status)
	}
	if stored.ExitReason != "" {
		t.Errorf("exitReason = %q on a record that has not exited", stored.ExitReason)
	}
}

// The watch's own version of the rule above, driven directly rather than
// through create, because create is not the only caller — restart launches the
// same watch, and a backend that returns an endpoint-less sandbox there gets
// the same treatment.
//
// "No endpoint" is not "not ready yet". There is nothing to poll and no later
// poll that could succeed, so treating it as a readiness timeout invents both a
// status the session never reached and a destruction ruling 10 reserves for the
// reaper, which logs it distinctly so "why did my sandbox vanish" has an answer.
func TestWatchSessionReadyLeavesAnEndpointLessSandboxToTheReaper(t *testing.T) {
	env := sessionsNewEnv(t)
	calls := sessionsStubReadyCalls(t, context.DeadlineExceeded)
	sessionsSeed(t, env.st, sessionsStarting("s-00000001"))

	env.api.watchSessionReady("s-00000001", "s-00000001", "")

	if n := calls.Load(); n != 0 {
		t.Errorf("the readiness wait ran %d times with no endpoint to wait on", n)
	}
	if env.be.destroyCount() != 0 {
		t.Fatalf("Destroy was called %d times; the sandbox is running and the reaper owns it", env.be.destroyCount())
	}
	got, err := env.st.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != registry.StatusStarting {
		t.Errorf("status = %q, want it left at starting for the reaper", got.Status)
	}
	if got.ExitReason != "" {
		t.Errorf("exitReason = %q; the session never exited and the real cause is not a readiness timeout", got.ExitReason)
	}
	if got.AttachToken == "" {
		t.Error("the attach token was cleared; the record is not terminal and task 13 re-reads it on recovery")
	}
}

// registry.ErrInvalidID is 400 and registry.ErrExists is 409, which task 7's
// decision 3 fixes: "task 11 must answer 400 to it rather than 409". Neither is
// reachable through the real path — the id comes from ids.New over crypto/rand
// — so the double is told to fail. A defensive mapping nothing exercises is a
// mapping that is wrong the first time it fires.
func TestCreateMapsTheStoresOwnSentinelsRatherThanAnswering500(t *testing.T) {
	for name, tc := range map[string]struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		"invalid id": {registry.ErrInvalidID, http.StatusBadRequest, "bad_request"},
		"duplicate":  {registry.ErrExists, http.StatusConflict, "conflict"},
	} {
		t.Run(name, func(t *testing.T) {
			env := sessionsNewEnv(t)
			env.st.state.createErr = tc.err

			rec := env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody)
			if code := sessionsProblemCode(t, rec, tc.wantStatus); code != tc.wantCode {
				t.Errorf("error = %q, want %q", code, tc.wantCode)
			}
			if n := len(env.be.specs); n != 0 {
				t.Errorf("backend.Create ran %d times after the record failed to write; the record exists before the sandbox does", n)
			}
		})
	}
}

func sessionsDecodeList(t *testing.T, rec *httptest.ResponseRecorder) []sessionJSON {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Sessions []sessionJSON `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode the list: %v; body = %s", err, rec.Body.String())
	}
	return got.Sessions
}

// throughNew drives the handler api.New actually returns — origin middleware,
// loopback Host guard, /api/v1 alias and all — rather than the bare recorder
// the rest of this file uses. The target is absolute so the Host is a loopback
// address; a path-only one would be example.com and task 9's guard would
// correctly answer 403 forbidden_host.
func (e *sessionsEnv) throughNew(t *testing.T, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, sessionsTarget(target), nil)
	rec := httptest.NewRecorder()
	e.api.ServeHTTP(rec, r)
	return rec
}

// The line this pins is a.registerSessions(guarded) in New. Delete it and
// every other test in this file still passes, because they drive a recorder
// the fixture registered onto itself; only this one goes through the handler
// the process serves. It checks the /api/v1 alias too, because contract §7.5
// makes that Phase 4's entry point and nothing else in Phase 2 would notice it
// breaking.
func TestNewActuallyRoutesTheSessionSurface(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeedThree(t, env)

	for _, target := range []string{"/v1/sessions", "/api/v1/sessions"} {
		rec := env.throughNew(t, http.MethodGet, target)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s through New = %d, want 200; body = %s — New never registered the session routes", target, rec.Code, rec.Body.String())
		}
		if got := sessionsIDsOf(sessionsDecodeList(t, rec)); len(got) != 3 {
			t.Errorf("GET %s returned %d sessions, want 3", target, len(got))
		}
	}

	// And the guard is still in front of it: this is the same handler, so a
	// non-loopback Host must not reach the session surface at all. The target
	// is path-only on purpose — that is what makes httptest fill in
	// Host: example.com.
	r := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	rec := httptest.NewRecorder()
	env.api.ServeHTTP(rec, r)
	if code := sessionsProblemCode(t, rec, http.StatusForbidden); code != "forbidden_host" {
		t.Errorf("error = %q, want forbidden_host", code)
	}
}

// sessionsIDsOf, and not ids: sessions.go imports internal/ids, and Go forbids
// a package-level identifier that collides with an import name used anywhere
// in the package's files.
func sessionsIDsOf(list []sessionJSON) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		out = append(out, s.ID)
	}
	return out
}

// sessionsSeedThree puts one exited, one running and one starting session in the store,
// created an hour apart, oldest first.
func sessionsSeedThree(t *testing.T, env *sessionsEnv) {
	t.Helper()
	base := time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		id       string
		status   registry.Status
		worktree string
		created  time.Time
	}{
		{"s-0000000a", registry.StatusExited, "w-4a77b1c9", base},
		{"s-0000000b", registry.StatusRunning, "w-4a77b1c9", base.Add(time.Hour)},
		{"s-0000000c", registry.StatusStarting, "w-other", base.Add(2 * time.Hour)},
	} {
		s := sessionsStarting(tc.id)
		s.Status = tc.status
		s.WorktreeID = tc.worktree
		s.CreatedAt = tc.created
		sessionsSeed(t, env.st, s)
	}
}

// Contract §3.11: descending, and exited sessions are included. The transcript
// is the artifact — a rail that hides finished sessions has thrown away the
// thing docs/FUNCTIONAL-SPEC.md says the user came back for.
func TestListIsDescendingByCreatedAtAndKeepsExitedSessions(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeedThree(t, env)

	got := sessionsIDsOf(sessionsDecodeList(t, env.do(t, http.MethodGet, "/v1/sessions", "")))
	want := []string{"s-0000000c", "s-0000000b", "s-0000000a"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestListFiltersByWorktreeAndByRepeatedStatus(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeedThree(t, env)

	got := sessionsIDsOf(sessionsDecodeList(t, env.do(t, http.MethodGet, "/v1/sessions?worktreeId=w-4a77b1c9", "")))
	if strings.Join(got, ",") != "s-0000000b,s-0000000a" {
		t.Errorf("worktree filter = %v, want [s-0000000b s-0000000a]", got)
	}

	// §3.11: status may be repeated, and the repeats are a union.
	got = sessionsIDsOf(sessionsDecodeList(t, env.do(t, http.MethodGet, "/v1/sessions?status=running&status=starting", "")))
	if strings.Join(got, ",") != "s-0000000c,s-0000000b" {
		t.Errorf("status filter = %v, want [s-0000000c s-0000000b]", got)
	}

	got = sessionsIDsOf(sessionsDecodeList(t, env.do(t, http.MethodGet, "/v1/sessions?worktreeId=w-4a77b1c9&status=exited", "")))
	if strings.Join(got, ",") != "s-0000000a" {
		t.Errorf("both filters = %v, want [s-0000000a]", got)
	}
}

// An unknown status is 400, not an empty list: an empty list is
// indistinguishable from "nothing matched", so a cockpit that mistyped the
// filter would show an empty rail forever and never learn why.
func TestListRejectsAnUnknownStatus(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeedThree(t, env)
	rec := env.do(t, http.MethodGet, "/v1/sessions?status=planning", "")
	if code := sessionsProblemCode(t, rec, http.StatusBadRequest); code != "bad_request" {
		t.Errorf("error = %q, want bad_request", code)
	}
}

// null and [] are different values to every JSON client. An empty rail must
// decode as an empty array.
func TestListReturnsAnEmptyArrayAndNeverNull(t *testing.T) {
	env := sessionsNewEnv(t)
	body := env.do(t, http.MethodGet, "/v1/sessions", "").Body.String()
	if !strings.Contains(body, `"sessions":[]`) {
		t.Errorf("body = %s, want an empty sessions array", body)
	}
}

func TestGetReturnsTheSessionOr404(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeedThree(t, env)

	got := sessionsDecodeOne(t, env.do(t, http.MethodGet, "/v1/sessions/s-0000000b", ""), http.StatusOK)
	if got.ID != "s-0000000b" || got.Status != "running" {
		t.Errorf("got %q/%q, want s-0000000b/running", got.ID, got.Status)
	}
	// An exited session is still readable. This is the product requirement, not
	// a nicety: claude --resume reads its transcript from inside the sandbox and
	// the record is how the user finds it again.
	if got := sessionsDecodeOne(t, env.do(t, http.MethodGet, "/v1/sessions/s-0000000a", ""), http.StatusOK); got.Status != "exited" {
		t.Errorf("status = %q, want exited", got.Status)
	}
	if code := sessionsProblemCode(t, env.do(t, http.MethodGet, "/v1/sessions/s-nope", ""), http.StatusNotFound); code != "session_not_found" {
		t.Errorf("error = %q, want session_not_found", code)
	}
}

func TestPatchRenamesAnySessionIncludingAnExitedOne(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeedThree(t, env)

	rec := env.do(t, http.MethodPatch, "/v1/sessions/s-0000000a", `{"title":"  evidence upload — second attempt  "}`)
	got := sessionsDecodeOne(t, rec, http.StatusOK)
	// Trimmed, because a title with trailing spaces is a rail that looks broken.
	if got.Title != "evidence upload — second attempt" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Status != "exited" {
		t.Errorf("patch changed the status to %q; title is the only patchable field", got.Status)
	}
	stored, err := env.st.Get("s-0000000a")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Title != "evidence upload — second attempt" {
		t.Errorf("the rename did not reach the store: %q", stored.Title)
	}
}

func TestPatchRejectsAnEmptyTitleAnAbsentTitleAndAnUnknownField(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeedThree(t, env)

	// Every one of these is bad_request, including the last two: since group-C
	// §1B the decoder answers bad_request for well-formed JSON that names a
	// field this endpoint does not have OR gives a field it does have the wrong
	// type, so one user mistake has one code across the whole surface. The plan
	// text expected invalid_json for the unknown field and the ruling
	// supersedes it.
	for name, tc := range map[string]struct {
		body     string
		wantCode string
	}{
		"empty":                 {`{"title":""}`, "bad_request"},
		"whitespace only":       {`{"title":"   "}`, "bad_request"},
		"absent":                {`{}`, "bad_request"},
		"another field":         {`{"status":"running"}`, "bad_request"},
		"title is not a string": {`{"title":5}`, "bad_request"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := env.do(t, http.MethodPatch, "/v1/sessions/s-0000000b", tc.body)
			if code := sessionsProblemCode(t, rec, http.StatusBadRequest); code != tc.wantCode {
				t.Errorf("error = %q, want %q", code, tc.wantCode)
			}
		})
	}
	// And a body that does not parse is still invalid_json, so the two codes
	// keep meaning two different things on this endpoint too.
	if code := sessionsProblemCode(t, env.do(t, http.MethodPatch, "/v1/sessions/s-0000000b", `{"title":`), http.StatusBadRequest); code != "invalid_json" {
		t.Errorf("error = %q, want invalid_json", code)
	}
	if code := sessionsProblemCode(t, env.do(t, http.MethodPatch, "/v1/sessions/s-nope", `{"title":"x"}`), http.StatusNotFound); code != "session_not_found" {
		t.Errorf("patching an unknown id: error = %q, want session_not_found", code)
	}
}

func TestRestartKeepsTheIDAndMintsANewSandboxAndANewToken(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	before := sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))

	got := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", ""), http.StatusOK)

	if got.ID != "s-0000000b" {
		t.Errorf("id = %q; restart keeps the session id — a new id is what POST /v1/sessions with resume is for", got.ID)
	}
	if got.Status != string(registry.StatusStarting) {
		t.Errorf("status = %q, want starting", got.Status)
	}
	if got.ExitReason != "" {
		t.Errorf("exitReason = %q, want it cleared", got.ExitReason)
	}
	after, err := env.st.Get("s-0000000b")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if after.AttachToken == before.AttachToken || after.AttachToken == "" {
		t.Errorf("the attach token was not replaced (%q); the old one was handed to a sandbox that no longer exists", after.AttachToken)
	}
	if after.SandboxID == before.SandboxID {
		t.Errorf("sandboxId is still %q; a fresh id is what stops Create racing a Destroy that has not finished, and it makes the old container an orphan the reaper can see", after.SandboxID)
	}
	// Same config, and one thing new: the conversation. The stored argv named
	// no conversation, and replaying one that does gets "Session ID … is
	// already in use" from the CLI — sessions_resume_test.go pins both halves.
	if after.ConversationID == "" || after.ConversationID == before.ConversationID {
		t.Errorf("conversationId = %q (was %q); a restart is a clean context and needs a conversation of its own", after.ConversationID, before.ConversationID)
	}
	if want := []string{"--append-system-prompt", harness.ProjectBriefPrompt, "--model", "opus", "--session-id", after.ConversationID}; after.Command != before.Command || strings.Join(after.Args, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("restart ran %q %v, want %q %v", after.Command, after.Args, before.Command, want)
	}
	if len(env.be.destroys) != 1 || env.be.destroys[0] != before.SandboxID {
		t.Errorf("destroys = %v, want exactly the old sandbox %q", env.be.destroys, before.SandboxID)
	}
	if spec := env.be.spec(t, 0); spec.ID != after.SandboxID || spec.AttachToken != after.AttachToken {
		t.Errorf("the new sandbox was created with %q/%q, want %q/<the new token>", spec.ID, spec.AttachToken, after.SandboxID)
	}
	if strings.Contains(env.do(t, http.MethodGet, "/v1/sessions/s-0000000b", "").Body.String(), after.AttachToken) {
		t.Error("the new token is visible on the wire")
	}
	sessionsAwaitWatch(t, done)
}

// Restart writes the record BEFORE it creates, exactly as create does, and for
// the same reason: task 13's reaper matches live containers against SandboxID,
// so the record is the only thing that names a container.
//
// Two sweeps land in the window this closes, and they break a restarting
// session in two different ways.
//
//  1. A record that does not yet name the container being created makes that
//     container an orphan the reaper destroys — it kills a restarting session's
//     brand-new sandbox seconds after it was asked for.
//  2. A record still naming the sandbox the restart has already destroyed makes
//     rule 2 see a session whose container disappeared, and write it terminal:
//     exited, "sandbox disappeared", onto a session that is restarting normally.
//     That is the fabricated status this phase has now ruled on seven times.
//
// Both are read from inside the window, through the backend's create hook,
// because that is the only place the ordering is observable. Asserting on the
// state after the request tells you nothing: both orderings end identically.
func TestRestartNamesTheNewSandboxBeforeItIsCreated(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	before := sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))

	var midCreate registry.Session
	var midErr error
	env.be.onCreate = func(spec backend.Spec) {
		midCreate, midErr = env.st.Get("s-0000000b")
	}

	sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", ""), http.StatusOK)
	sessionsAwaitWatch(t, done)

	if midErr != nil {
		t.Fatalf("read the record mid-create: %v", midErr)
	}
	spec := env.be.spec(t, 0)
	if midCreate.SandboxID != spec.ID {
		t.Errorf("mid-create the record named %q while the container being created was %q; a sweep here destroys a restarting session's new sandbox as an orphan",
			midCreate.SandboxID, spec.ID)
	}
	if midCreate.SandboxID == before.SandboxID {
		t.Errorf("mid-create the record still named %q, the sandbox this restart had already destroyed; a sweep here writes the session exited with \"sandbox disappeared\"",
			before.SandboxID)
	}
	if midCreate.Status != registry.StatusStarting {
		t.Errorf("status mid-create = %q, want starting", midCreate.Status)
	}
	if midCreate.ExitReason != "" {
		t.Errorf("exitReason mid-create = %q, want it already cleared", midCreate.ExitReason)
	}
	// The token is minted with the id and written with it, so the record never
	// names a sandbox while still holding the token of a different one.
	if midCreate.AttachToken != spec.AttachToken {
		t.Errorf("mid-create the record held token %q while the sandbox was being created with %q",
			midCreate.AttachToken, spec.AttachToken)
	}
	// Likewise the conversation: a crash inside this window must leave a record
	// that names the conversation the new sandbox runs, or a later resume
	// continues the one the restart abandoned.
	if i := slices.Index(spec.Args, "--session-id"); i < 0 || midCreate.ConversationID != spec.Args[i+1] {
		t.Errorf("mid-create the record named conversation %q while the sandbox was being created with %#v",
			midCreate.ConversationID, spec.Args)
	}
	// And the endpoint is empty until there is a sandbox to have one: writing
	// the old sandbox's address onto the new id would point an attach at a
	// container that no longer exists.
	if midCreate.Endpoint != "" {
		t.Errorf("endpoint mid-create = %q, want it empty until the new sandbox reports one", midCreate.Endpoint)
	}
}

// §3.14: 409 when a restart is already in flight. The test holds the guard
// directly rather than racing two requests, which is what the internal test
// package is for.
func TestRestartIsRejectedWhileAnotherIsInFlight(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))

	if !sessionRestarts.acquire("s-0000000b") {
		t.Fatal("the guard was already held; a previous test leaked it")
	}
	defer sessionRestarts.release("s-0000000b")

	rec := env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", "")
	if code := sessionsProblemCode(t, rec, http.StatusConflict); code != "conflict" {
		t.Errorf("error = %q, want conflict", code)
	}
	if env.be.destroyCount() != 0 {
		t.Errorf("the rejected restart destroyed %d sandboxes", env.be.destroyCount())
	}
}

// A sandbox the daemon has already forgotten is not an error: the reaper and an
// explicit restart race by design.
func TestRestartToleratesASandboxThatIsAlreadyGone(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))
	env.be.destroyErr = backend.ErrNotFound

	got := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", ""), http.StatusOK)
	if got.Status != string(registry.StatusStarting) {
		t.Errorf("status = %q, want starting", got.Status)
	}
	sessionsAwaitWatch(t, done)
}

// The other side of writing first: when the create fails, the record names the
// sandbox that create was asked for rather than the one the restart had already
// destroyed. Before the reorder a failed restart left the record pointing at a
// container known to be gone, which is the same "record names a dead sandbox"
// the reorder exists to remove — it simply persisted instead of closing.
func TestRestartThatCannotCreateLeavesAnHonestRecord(t *testing.T) {
	env := sessionsNewEnv(t)
	before := sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))
	env.be.createErr = backend.ErrUnavailable

	rec := env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", "")
	if code := sessionsProblemCode(t, rec, http.StatusServiceUnavailable); code != "backend_unavailable" {
		t.Errorf("error = %q, want backend_unavailable", code)
	}
	after, err := env.st.Get("s-0000000b")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if after.SandboxID == before.SandboxID {
		t.Errorf("sandboxId = %q, the sandbox this restart destroyed; the record names a container that is certainly gone", after.SandboxID)
	}
	if after.SandboxID != env.be.spec(t, 0).ID {
		t.Errorf("sandboxId = %q, want the id create was asked for, %q", after.SandboxID, env.be.spec(t, 0).ID)
	}
	if after.Status != registry.StatusExited || after.ExitReason == "" {
		t.Errorf("status/reason = %q/%q, want exited with a reason", after.Status, after.ExitReason)
	}
	if after.Endpoint != "" || after.AttachToken != "" {
		t.Errorf("endpoint/token survived a failed restart: %q/%q", after.Endpoint, after.AttachToken)
	}
}

func TestRestartOnAnUnknownIDIs404(t *testing.T) {
	env := sessionsNewEnv(t)
	rec := env.do(t, http.MethodPost, "/v1/sessions/s-nope/restart", "")
	if code := sessionsProblemCode(t, rec, http.StatusNotFound); code != "session_not_found" {
		t.Errorf("error = %q, want session_not_found", code)
	}
}

func TestDeleteKillsTheSandboxKeepsTheRecordAndIsIdempotent(t *testing.T) {
	env := sessionsNewEnv(t)
	before := sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))

	got := sessionsDecodeOne(t, env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b", ""), http.StatusOK)
	if got.Status != string(registry.StatusExited) || got.ExitReason != "killed" {
		t.Errorf("status/reason = %q/%q, want exited/killed", got.Status, got.ExitReason)
	}
	if len(env.be.destroys) != 1 || env.be.destroys[0] != before.SandboxID {
		t.Fatalf("destroys = %v, want exactly %q", env.be.destroys, before.SandboxID)
	}
	stored, err := env.st.Get("s-0000000b")
	if err != nil {
		t.Fatalf("the record was removed without purge: %v", err)
	}
	if stored.Endpoint != "" || stored.AttachToken != "" {
		t.Errorf("endpoint/token survived the kill: %q/%q — the port will be reused by another container and the token is for a sandbox that is gone", stored.Endpoint, stored.AttachToken)
	}

	// §3.15: killing an already-exited session is 200, not an error. The
	// reaper and a user clicking twice race by design.
	again := sessionsDecodeOne(t, env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b", ""), http.StatusOK)
	if again.ExitReason != "killed" {
		t.Errorf("the second delete rewrote the reason to %q", again.ExitReason)
	}
	if env.be.destroyCount() != 1 {
		t.Errorf("destroys = %d, want 1 — the second delete re-issued a Destroy for a sandbox the first one already removed", env.be.destroyCount())
	}
}

// The consequence of that, and the reason it matters rather than being tidy:
// the record deliberately keeps SandboxID after a kill, so a purge that
// re-issued the Destroy would need a live daemon to remove a record whose
// sandbox is already gone. A user cleaning up their rail after Docker Desktop
// has been closed would find they could not.
func TestPurgingAnAlreadyKilledSessionDoesNotNeedTheBackend(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))
	sessionsDecodeOne(t, env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b", ""), http.StatusOK)

	// Now the daemon goes away.
	env.be.destroyErr = backend.ErrUnavailable

	rec := env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b?purge=true", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", rec.Code, rec.Body.String())
	}
	if _, err := env.st.Get("s-0000000b"); !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("the record survived the purge: err = %v", err)
	}
	if env.be.destroyCount() != 1 {
		t.Errorf("destroys = %d, want 1 — only the kill destroys; the purge has nothing left to ask the backend for", env.be.destroyCount())
	}
}

// The other half, so the skip above cannot widen into "purge never destroys":
// a session that has NOT been killed still destroys first, and a destroy that
// fails still refuses the purge. Removing the record there would orphan a
// container that may still be running, with nothing left naming it.
func TestPurgingALiveSessionStillRefusesWhenTheDestroyFails(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))
	env.be.destroyErr = backend.ErrUnavailable

	rec := env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b?purge=true", "")
	if code := sessionsProblemCode(t, rec, http.StatusServiceUnavailable); code != "backend_unavailable" {
		t.Errorf("error = %q, want backend_unavailable", code)
	}
	if _, err := env.st.Get("s-0000000b"); err != nil {
		t.Errorf("the record was purged while its container may still be running: %v", err)
	}
}

func TestDeleteWithPurgeRemovesTheRecordAndReturns204(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))

	rec := env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b?purge=true", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carried a body: %s", rec.Body.String())
	}
	if _, err := env.st.Get("s-0000000b"); !errors.Is(err, registry.ErrNotFound) {
		t.Errorf("the record survived purge: err = %v", err)
	}
	if env.be.destroyCount() != 1 {
		t.Errorf("purge destroyed %d sandboxes, want 1", env.be.destroyCount())
	}
	if code := sessionsProblemCode(t, env.do(t, http.MethodDelete, "/v1/sessions/s-nope", ""), http.StatusNotFound); code != "session_not_found" {
		t.Errorf("error = %q, want session_not_found", code)
	}
}

// A Destroy that genuinely failed must not produce status "exited". The
// container is probably still running, and a fabricated status is what this
// phase rules out.
func TestDeleteLeavesTheRecordAloneWhenDestroyFails(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))
	env.be.destroyErr = backend.ErrUnavailable

	rec := env.do(t, http.MethodDelete, "/v1/sessions/s-0000000b", "")
	if code := sessionsProblemCode(t, rec, http.StatusServiceUnavailable); code != "backend_unavailable" {
		t.Errorf("error = %q, want backend_unavailable", code)
	}
	stored, err := env.st.Get("s-0000000b")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Status != registry.StatusStarting {
		t.Errorf("status = %q, want it unchanged at starting", stored.Status)
	}
}

// Rule one, contract §5.2. Destroy has three causes and a dropped WebSocket is
// not one of them: finished sessions stay listed and readable because
// claude --resume reads its state from ~/.claude INSIDE the sandbox, so
// destroying on disconnect deletes the artifact the product is built around.
// Task 12 owns the relay; what this task owes is that nothing here destroys.
func TestNothingDestroysBecauseAClientWentAway(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)

	created := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions", sessionsCreateBody), http.StatusCreated)
	sessionsAwaitWatch(t, done)

	// Exactly what task 12's relay defer does when the socket drops.
	if _, err := env.st.Update(created.ID, func(sess *registry.Session) error {
		sess.AttachCount++
		return nil
	}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if _, err := env.st.Update(created.ID, func(sess *registry.Session) error {
		sess.AttachCount--
		sess.LastOutputAt = time.Now().UTC()
		return nil
	}); err != nil {
		t.Fatalf("detach: %v", err)
	}

	if env.be.destroyCount() != 0 {
		t.Fatalf("Destroy was called %d times for a client that merely went away", env.be.destroyCount())
	}
	got := sessionsDecodeOne(t, env.do(t, http.MethodGet, "/v1/sessions/"+created.ID, ""), http.StatusOK)
	if got.Status != string(registry.StatusRunning) {
		t.Errorf("status = %q, want running — a detach is not an exit", got.Status)
	}
	if got.AttachCount != 0 {
		t.Errorf("attachCount = %d, want 0", got.AttachCount)
	}
	if n := len(sessionsDecodeList(t, env.do(t, http.MethodGet, "/v1/sessions", ""))); n != 1 {
		t.Errorf("List returned %d sessions, want 1 — the session must stay listed", n)
	}
}
