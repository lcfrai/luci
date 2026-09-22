### Task 11: Session CRUD

This is the task the whole phase exists to reach: the six verbs that turn a worktree id and a harness choice into a running agent, and that keep the record of it afterwards. Everything before it is a seam; this is the first file a user can feel.

Two rules govern it and both are pinned by tests rather than by review.

**Rule one — a disconnect destroys nothing.** Contract §5.2 names exactly three causes of destruction: an explicit `DELETE /v1/sessions/{id}`, the reaper's idle TTL, and the reaper's orphan sweep. A dropped WebSocket is not one of them. `docs/FUNCTIONAL-SPEC.md` requires that finished sessions stay listed and readable, and Resume works by running `claude --resume`, which reads its state from `~/.claude` **inside the sandbox** — so a control plane that destroys the sandbox when the socket drops has deleted the artifact the product is built around, and it has done it silently, at the moment the user's laptop slept. Inside this task the rule has a narrow, checkable form: `sessions.go` calls `Backend.Destroy` from exactly three places — delete, restart, and the ready-watch's own failure path — and from nowhere else. Step 8 asserts a fake backend recorded zero destroys across a create, an attach-count decrement and a get.

**Rule two — `sessionJSON` exists so a secret cannot leak, and the test must prove that, not assert it.** Contract §3.10 forbids six fields on the wire: `attachToken`, `endpoint`, `sandboxRef`, `sandboxId`, `command`, `args`. A test that checks `got.AttachToken == ""` proves only that the field you remembered is empty. The test in step 2 marshals the real struct and searches the **bytes** for the token and the endpoint, then asserts the decoded key set is exactly the twenty-one names §3.10 lists — so a future field added to `registry.Session` and blindly copied into `sessionJSON` fails here rather than in a support ticket. §3.10 is also explicit that `registry.Session` must **not** grow `json:"-"` tags: those fields have to serialise into the registry file.

Four places where the contract is silent, decided here so nobody re-derives them. **(a) `ProjectID` and `BranchID` on create.** `POST /v1/sessions` carries only `worktreeId`, `worktree.Provider` has only `Path`, and §3.10 wants both other ids on the object. The resolution is task 10's `api.Enumerator`, which `*project.Scanner` already satisfies: create scans it once for the worktree and reads the two ids off `project.Worktree`. When the enumerator finds nothing the ids stay `""`, which is honest; guessing them from a path would be a fabricated relationship. The cost is O(projects) git invocations per create, which contract §8.8 already accepts as the shape of this problem, and create is rare. **(b) An unknown `?status=` value is `400 bad_request`**, not an empty list: an empty list is indistinguishable from "no sessions match", and a cockpit that mistypes the filter would show an empty rail forever. **(c) A kill clears `Endpoint` and `AttachToken`.** The port is about to be handed back to the OS and may belong to another container within seconds, and a token for a sandbox that no longer exists is only a liability; restart mints both afresh. **(d) A `Destroy` that genuinely fails does not produce `status: "exited"`.** Contract §3.15 describes the happy path; if the daemon is down the container is probably still running, and reporting it exited is exactly the fabricated status this phase rules out. The verb returns the mapped backend error and leaves the record alone.

Two notes on the state of the tree. `backend.Sandbox.NonLoopback` (task 5) is **not** mapped to a session status here: the contract's amended `List` keeps a LAN-published container in the slice with an empty `Endpoint` so task 13's reaper can destroy it, and until the reaper acts the session's status stays whatever the process is actually doing. And `dockerbe.Create` already waits for `/healthz` (task 5 decision (a)), so for the Docker backend the async watch of §5.1 step 7 usually completes on its first poll; it is kept because `katabe` does not wait, because it is what flips `starting` → `running`, and because deleting it would make the Kata path silently stick at `starting`.

**Files:**
- `control-plane/internal/api/sessions.go` — the six verbs, `sessionJSON`/`toJSON`, the ready watch and the restart guard (new)
- `control-plane/internal/api/sessions_test.go` — the leak test, the in-memory `registry.Store` double run against `registrytest.RunStoreContract`, and one test per rule above (new)
- `control-plane/internal/api/api.go` — **modified in two places, and in two different steps.** Step 1 adds the three session-specific `Deps` fields task 10 does not carry; **step 5 adds the single line `a.registerSessions(guarded)` to `New`, in the same step that defines `registerSessions`.** The split is the group-C contract's ruling 2 and it is not cosmetic: a `New` that calls a method no file has written yet stops `package api` compiling for every step in between, which is exactly the defect the first draft of this task shipped. The contract's task table gives task 11 one created file; a handler cannot reach a dependency the struct does not carry and an unregistered route is not served, so both edits are declared, additive departures from that row, and nothing else in `api.go` is touched.

**Interfaces:**
- **The naming rule, which is the group-C contract's ruling 5 and is mechanical.** Every package-level identifier this task declares — in `sessions.go` and in `sessions_test.go` alike — **begins with** `session` or `sessions`, the prefix of the surface that declares it, exactly as task 10 uses `catalog*` and task 12 uses `attach*`. Begins with, not merely contains: `createSessionRequest`, `patchSessionRequest` and `errSessionMoved` are the shape that reads as compliant and is not, which is why they are spelled `sessionCreateRequest`, `sessionPatchRequest` and `sessionErrMoved` below. It needs no cross-file coordination, which is the point: the first draft of this task declared a bare `fakeBackend` that task 12 declares verbatim in the same package, and the package stops compiling the moment both land. **Two exceptions, and they are the only two.** The `TestXxx` functions, whose names Go fixes to begin with `Test`, stay behaviour-describing and were checked one by one against task 12's committed plan (`TestAttach*`, `TestRelay*`, `TestADroppedSocketDestroysNothing`, `TestOutputTrackerThrottlesRegistryWrites`) and against task 10's, which is `package api_test` and therefore cannot collide at all. And `toJSON`, which contract §3.10 fixes by name — renaming a locked signature to satisfy a convention is the wrong trade, so the collision it would otherwise invite is closed by this sentence instead: no other task writing into `package api` may declare `toJSON`. Methods are not package-level identifiers and are not covered: `(e *sessionsEnv) do` and `(e *sessionsEnv) throughNew` are scoped to their receiver and cannot collide with anything tasks 10 or 12 write.
- Produces, all unexported:
  - `sessionJSON` and `toJSON(registry.Session, string) sessionJSON` — the names contract §3.10 fixes
  - `(*API).registerSessions(*http.ServeMux)` — the name task 10 reserved for this task in its own Interfaces block — and the six handlers `handleSessionCreate` / `List` / `Get` / `Patch` / `Restart` / `Delete`
  - `(*API).watchSessionReady(sessionID, sandboxID, endpoint string)` — §5.1 step 7
  - `sessionBasePath`, `sessionWorkDir`, `sessionTime`, `sessionRestarts`, `sessionWaitReady`, `sessionReadyDone`, `sessionCreateRequest`, `sessionPatchRequest`, `sessionListResponse`, `sessionStatuses`
  - test-only, in `sessions_test.go`: `sessionsBackend`, `sessionsLocator`, `sessionsStore`, `sessionsEnv` and the rest of the `sessions*` fixtures. There is **no** `sessionIsolation`: ruling 6 makes task 10's `isolationFor` the only implementation of contract §3.4's enum in this package.
- Consumes:
  - task 4 — `backend.Backend`, `Spec`, `Mount`, `Resources`, `DefaultResources`, `Kind`/`KindDocker`/`KindKata`, `ReadyTimeout`, `WaitReady`, and the sentinels `ErrNotFound`, `ErrNotRunning`, `ErrAlreadyExists`, `ErrUnavailable`, `ErrUnsupported`, `ErrNonLoopbackPublish`; `ids.New`, `ids.Token`
  - task 7 — `registry.Store`, `Session`, `(Session).Clone`, the four `Status` constants, `ErrNotFound`, `ErrExists`, `ErrInvalidID`, and `registrytest.RunStoreContract` in the test
  - task 8 — `harness.Lookup`, `harness.Build`, `harness.Request`, `harness.ID`/`Effort`/`PermissionMode`, `ErrUnknownHarness`/`ErrUnavailable`/`ErrUnsupportedOption`/`ErrUnknownModel`; `worktree.Provider`, `worktree.ErrUnknownID`, `worktree.ErrNotADir`; `project.Project`, `project.Worktree`
  - task 9 — `httpx.WriteJSON`, `httpx.WriteProblem`, `httpx.DecodeJSON`, and the codes `CodeBadRequest`, `CodeInvalidJSON`, `CodeUnknownHarness`, `CodeUnsupportedOption`, `CodeWorktreeNotFound`, `CodeSessionNotFound`, `CodeConflict`, `CodeSandboxNotRunning`, `CodeHarnessUnavailable`, `CodeBackendUnavailable`. **Not `CodeInternal`:** `sessions.go` never writes a 500 itself, it calls task 10's `a.internal`, which owns the code, the body and the log line
  - task 10 — `api.API` (receiver `a`, field `a.deps`), `api.Deps`, `api.New(Deps) (*API, error)`, `api.Enumerator`, `api.LocalMachineID`, `(*API).internal(w, r, op, err)` for every 500 on this surface, and `isolationFor(backend.Kind) string` for contract §3.4's isolation enum. Task 10 landed after the contract was locked and its names win: the handlers hang off `*API`, not `*Server`, and the routing hook is `registerSessions`, which task 10's own plan already reserves for this task. Three `Deps` fields do not exist yet and step 1 adds them: `Image string`, `Resources backend.Resources`, `ReadyTimeout time.Duration` — `Image`, not `SandboxImage`, which is the group-C contract's rulings 1 and 3; task 13 renames at its one call site
- Required by: task 12 (attaches to the session this task records, and decrements the `attachCount` this task serialises), task 13 (the reaper reads `SandboxID`, `Status` and `LastOutputAt`; restart recovery re-reads `Endpoint`)
- Amends: nothing. It applies contract §2.4's `List` amendment (task 5) by refusing to translate `NonLoopback` into a status.

- [ ] **Step 1: Add the three `Deps` fields task 10 does not have**

Task 10 is written and its names are settled, so this is a small, known edit rather than an investigation. Confirm the three facts first, because a plan reading a file it has not seen is how a task gets transcribed against the wrong receiver:

```bash
grep -n "type API struct" -A 6 control-plane/internal/api/api.go
grep -n "type Deps struct" -A 45 control-plane/internal/api/api.go
grep -n "registerSessions\|guarded" control-plane/internal/api/api.go
```

What you should see, and what every later step is written against: `type API struct { deps Deps; handler http.Handler }`, so the receiver is `a *API` and dependencies are reached as `a.deps.X`; `func New(d Deps) (*API, error)`, which validates that `Backend`, `BackendReady`, `Store`, `Flags`, `Projects`, `Worktrees` and `Origin` are all non-nil and returns an error naming the missing field; `Deps.Projects` typed as `api.Enumerator`, which is `Projects` + `Branches` + `Worktrees` and is what create uses to fill `projectId` and `branchId`; and `api.LocalMachineID`, the constant `"local"`, which this task uses rather than inventing a second default.

**Add three fields to `Deps`**, at the end of the struct, and nothing else in the file:

```go
	// Image is the sandbox image every session runs, e.g.
	// "panacea-app/sandbox:dev". Empty means that default.
	Image string

	// Resources sizes every sandbox. The zero value means
	// backend.DefaultResources.
	Resources backend.Resources

	// ReadyTimeout bounds the post-create readiness watch of contract §5.1
	// step 7. Zero means backend.ReadyTimeout. Task 13 feeds it from
	// PANACEA_APP_READY_TIMEOUT.
	ReadyTimeout time.Duration
```

`time` is already imported by `api.go`; `backend` is already imported for `Deps.Backend`. None of the three is added to `New`'s nil check — all three have a working default, and a validator that rejects a zero value with a sane default turns a convenience into a required argument.

**`New` is not touched in this step.** Task 10's comment reserves the line after `a.registerCatalog(guarded)` for `a.registerSessions(guarded)`, and that line is added in **step 5**, the step that writes the method. Adding it here would leave `api.go` calling a method no file declares, so `package api` would not compile for steps 1 to 4 and step 3's green suite would be impossible — the group-C contract's ruling 2. Adding fields to a struct, by contrast, breaks nothing: an unused field is not an error in Go, so this step's edit leaves the workspace exactly as green as it was.

There is no `MachineID` field and this task does not add one: `api.LocalMachineID` already exists and contract §3.4 fixes the local machine's id as the literal `"local"`.

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected — **no change at all**, and that is the point of doing the `api.go` edit in two halves. Three unused struct fields are not an error in Go, so the output of this run is identical to the run before this step: task 10's `internal/api` tests still pass and no package reports a build failure. If `internal/api` fails here, something other than this step's three added fields changed. (Verified against a tree assembled out of this plan: `go build ./...` exits 0 with the three fields added and no `sessions.go` yet.)

Almost every test below drives a bare `http.ServeMux` the fixture builds and calls `registerSessions` on, rather than the handler `New` returns, so this task's tests fail on this task's code rather than on task 10's origin middleware or its loopback `Host` guard — `httptest.NewRequest` sets `Host: example.com`, which that guard correctly rejects with 403. **Exactly one test is the exception, and it is the one that makes the rest safe:** step 6's `TestNewActuallyRoutesTheSessionSurface` sends `GET /v1/sessions` through `a.ServeHTTP` with a loopback `Host`, so the `New` line added in step 5 is pinned by something. Without it, deleting that line ships unreachable session routes with a fully green suite — which is the first recovery an implementer reaches for when a build error surprises them.

- [ ] **Step 2: Write the failing wire-shape tests and the in-memory `Store` double**

The test file is `package api`, not `package api_test`. Three of its subjects are unexported by design — `toJSON`, the restart guard and the ready watch — and an external test package could reach none of them. It is also what lets the restart test hold the in-flight guard directly instead of racing two goroutines and hoping.

The `Store` double is here rather than in `registry` because it is a test fixture for this package and nothing else needs it. It is keyed by path in a package-level map, because `registrytest.NewStoreFunc` is documented as "calling it twice with one path must reopen the same state" — several tests in the suite prove a property by reloading, and a double that forgets on reopen would pass a weaker suite than `FileStore` faces. That is the whole reason `registrytest` is an importable package rather than a `_test.go` file, and the reason this step calls `RunStoreContract`: an interface with one tested implementation is a type declaration, not a contract.

Create `control-plane/internal/api/sessions_test.go`:

```go
package api

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest"
)

// sessionsToken and sessionsEndpoint are deliberately unmistakable strings. The leak test
// searches the marshalled bytes for them, so they must not be substrings of
// anything legitimately on the wire.
const (
	sessionsToken    = "SECRET-ATTACH-TOKEN-DO-NOT-SERIALISE"
	sessionsEndpoint = "127.0.0.1:65432"
	sessionsRef      = "CONTAINER-REF-DO-NOT-SERIALISE"
)

// sessionsFullRecord is a Session with every field set to something distinguishable,
// including all six the contract forbids on the wire. A fixture with zero
// values in the forbidden fields would pass the leak test by accident.
func sessionsFullRecord() registry.Session {
	return registry.Session{
		ID:             "s-3f2a91c0",
		Title:          "Claude Code · evidence-upload",
		MachineID:      "local",
		ProjectID:      "p-8c1d0e42",
		BranchID:       "b-8c1d0e42-evidence-upload",
		WorktreeID:     "w-4a77b1c9",
		Harness:        "claude-code",
		Model:          "opus",
		Effort:         "high",
		PermissionMode: "ask",
		OpeningPrompt:  "add tests, then refactor",
		Command:        "claude",
		Args:           []string{"--model", "opus", "add tests, then refactor"},
		Status:         registry.StatusRunning,
		ExitReason:     "",
		Backend:        "docker",
		SandboxID:      "s-3f2a91c0",
		SandboxRef:     sessionsRef,
		Endpoint:       sessionsEndpoint,
		AttachToken:    sessionsToken,
		CreatedAt:      time.Date(2026, 9, 18, 9, 14, 3, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 9, 18, 9, 14, 7, 0, time.UTC),
		LastOutputAt:   time.Date(2026, 9, 18, 9, 14, 6, 0, time.UTC),
		AttachCount:    1,
		ContextRef:     "",
	}
}

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
		"attach token": sessionsToken,
		"endpoint":     sessionsEndpoint,
		"sandbox ref":  sessionsRef,
	} {
		if strings.Contains(body, secret) {
			t.Errorf("the %s reached the wire: %s", name, body)
		}
	}
	// command and args are forbidden for a different reason: they would put the
	// opening prompt in two places and invite the desktop to rebuild argv.
	for _, fragment := range []string{`"command"`, `"args"`, `"sandboxId"`, `"sandboxRef"`, `"endpoint"`, `"attachToken"`} {
		if strings.Contains(body, fragment) {
			t.Errorf("forbidden key %s is present: %s", fragment, body)
		}
	}
}
```

Append to the same file:

```go
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
```

Now the double, in the same file. `Update` clones twice on purpose, and the comment says why: the pointer handed to `mutate` escapes in `registrytest`'s seal test and is written through after `Update` returns. Storing that same value would let the late write land, and sharing its `Args` array would let it land even after a copy of the struct.

```go
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
```

```go
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
```

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected — a build failure in `internal/api` only; every other package still passes. `toJSON` and `sessionBasePath` are package-level identifiers, so the compiler says `undefined:` for each, and it says it **once per use site**, not once per symbol. There are **six** uses of each across the four tests above — twelve errors — and `gc` truncates at ten, so ten are printed and then `too many errors`. There is **no** error in `api.go`: step 1 added fields and nothing else, so `New` does not yet mention `registerSessions`. The transcript below is the real one, taken from a tree assembled out of this plan; the exact lines and columns depend on how you transcribed the file, so what to check is the shape, the cap and the absence of an `api.go` line:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/api [github.com/kpmg-au-emu/panacea-app/control-plane/internal/api.test]
internal\api\sessions_test.go:62:27: undefined: toJSON
internal\api\sessions_test.go:62:56: undefined: sessionBasePath
internal\api\sessions_test.go:88:27: undefined: toJSON
internal\api\sessions_test.go:88:56: undefined: sessionBasePath
internal\api\sessions_test.go:116:9: undefined: toJSON
internal\api\sessions_test.go:116:38: undefined: sessionBasePath
internal\api\sessions_test.go:129:13: undefined: toJSON
internal\api\sessions_test.go:129:23: undefined: sessionBasePath
internal\api\sessions_test.go:140:9: undefined: toJSON
internal\api\sessions_test.go:140:19: undefined: sessionBasePath
internal\api\sessions_test.go:140:19: too many errors
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/api [build failed]
FAIL
```

If instead you see `undefined: registrytest` or `undefined: Clone`, task 7 has not landed in this tree and this task cannot proceed; stop and check `control-plane/internal/registry/registrytest/registrytest.go` exists.

- [ ] **Step 3: Implement `sessionJSON`, `toJSON` and the two small helpers**

Every name this file adds is session-prefixed. Three tasks write into package `api` — 10, 11 and 12 — and a bare `isolation()` or `rfc3339()` helper is a merge conflict at best and a silently-shadowed behaviour at worst. Task 12 makes the same commitment for the attach-specific names.

`Isolation` is **not** computed here. Task 10 already declares `isolationFor(backend.Kind) string` in `api.go`, in this same package, and the group-C contract's ruling 6 makes it the only implementation: two functions mapping contract §3.4's three-value enum in one package is a defect even when they agree, and the first draft of this task disagreed with it — reporting `"unknown"` for an unrecognised backend on the session surface while the machine surface reported `"none"`, which invents a fourth value for a closed enum. `registry.Session.Backend` is a plain `string` (task 7 decision 7 keeps `registry` free of a `backend` import), so the call converts: `isolationFor(backend.Kind(s.Backend))`. That conversion is the only reason `sessions.go` imports `backend` in this step.

`sessionJSON` renders the three timestamps as strings rather than as `time.Time`. Go's `time.Time` marshals RFC 3339 with whatever sub-second precision it holds, so the same session serialises as `…:03Z` from the registry and `…:03.412Z` after a round-trip through a stamping `Update` — a difference the desktop would see as a changed field on every poll. Seconds precision, always UTC, and `""` for a zero value.

Create `control-plane/internal/api/sessions.go`:

```go
package api

import (
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// sessionBasePath is the canonical prefix every session URL is built from.
// Contract §3.1 also serves the whole surface under /api/v1, but that alias is
// task 10's rewrite and the paths this package MINTS — Location and attachPath
// — use the canonical form so the desktop has one shape to match.
const sessionBasePath = "/v1"

// sessionWorkDir is where the worktree is mounted inside every sandbox and the
// working directory the harness starts in. Contract §5.1 step 5.
const sessionWorkDir = "/home/dev/work"

// sessionJSON is the session object of contract §3.10, and it is a separate
// struct from registry.Session for one reason: AttachToken, Endpoint,
// SandboxID, SandboxRef, Command and Args must never reach the wire, and they
// cannot be tagged json:"-" on the record because they must reach the registry
// file. A separate struct makes the omission a compile-time fact instead of a
// review question. sessions_test.go proves it by searching the marshalled
// bytes, not by checking a field it remembered.
type sessionJSON struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	MachineID  string `json:"machineId"`
	ProjectID  string `json:"projectId"`
	BranchID   string `json:"branchId"`
	WorktreeID string `json:"worktreeId"`

	Harness        string `json:"harness"`
	Model          string `json:"model"`
	Effort         string `json:"effort"`
	PermissionMode string `json:"permissionMode"`
	OpeningPrompt  string `json:"openingPrompt"`

	Status     string `json:"status"`
	ExitReason string `json:"exitReason"`

	Backend   string `json:"backend"`
	Isolation string `json:"isolation"`

	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	LastOutputAt string `json:"lastOutputAt"`
	AttachCount  int    `json:"attachCount"`
	AttachPath   string `json:"attachPath"`

	ContextRef string `json:"contextRef"`
}

// toJSON renders one session for the wire. base is the URL prefix attachPath is
// built from; production passes sessionBasePath.
func toJSON(s registry.Session, base string) sessionJSON {
	return sessionJSON{
		ID:             s.ID,
		Title:          s.Title,
		MachineID:      s.MachineID,
		ProjectID:      s.ProjectID,
		BranchID:       s.BranchID,
		WorktreeID:     s.WorktreeID,
		Harness:        s.Harness,
		Model:          s.Model,
		Effort:         s.Effort,
		PermissionMode: s.PermissionMode,
		OpeningPrompt:  s.OpeningPrompt,
		Status:         string(s.Status),
		ExitReason:     s.ExitReason,
		Backend:        s.Backend,
		Isolation:      isolationFor(backend.Kind(s.Backend)),
		CreatedAt:      sessionTime(s.CreatedAt),
		UpdatedAt:      sessionTime(s.UpdatedAt),
		LastOutputAt:   sessionTime(s.LastOutputAt),
		AttachCount:    s.AttachCount,
		AttachPath:     base + "/sessions/" + s.ID + "/attach",
		ContextRef:     s.ContextRef,
	}
}

// sessionTime renders a timestamp for the wire: UTC, seconds, and "" for the
// zero value.
func sessionTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
```

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected — `internal/api` builds and its five tests pass, including the store contract suite's seven subtests. Every other package is unchanged.

- [ ] **Step 4: Write the failing tests for create**

These pin contract §5.1's order, which is the part of this task that is genuinely load-bearing: **the worktree is resolved before anything is created, and the record exists before the sandbox does.** Resolving first means a mistyped id costs nothing. Recording first means that when `backend.Create` fails, the record is the only evidence of what happened — and without it, a half-created container is an orphan nobody can name and the reaper has nothing to match it against.

Replace the import block in `control-plane/internal/api/sessions_test.go` with the full set the finished file needs:

```go
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
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)
```

`internal/harness` is deliberately absent: the tests name harnesses by their wire strings (`"claude-code"`, `"codex"`, `"shell"`), because those strings are what the desktop sends and a test written against the Go constants would still pass if a constant's value changed. Go rejects an unused import, so adding it back requires a use.

Append the fakes. The backend fake records every call so a test can assert what was **not** done — zero destroys is how rule one is proved — and `backend.Dial` is never reached because nothing here attaches.

```go
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

	specs    []backend.Spec
	destroys []string
}

func sessionsNewBackend() *sessionsBackend {
	return &sessionsBackend{kind: backend.KindDocker, endpoint: "127.0.0.1:49200"}
}

func (f *sessionsBackend) Kind() backend.Kind { return f.kind }

func (f *sessionsBackend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
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
```

```go
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
	mux *http.ServeMux
	be  *sessionsBackend
	st  *sessionsStore
	wt  worktree.Static
	dir string
	loc *sessionsLocator
}

// sessionsNewEnv is the ONE place that constructs an API. Deps carries seven
// fields New refuses to start without — Backend, BackendReady, Store, Flags,
// Projects, Worktrees and Origin — so all seven are here even though two,
// BackendReady and Flags, belong to task 10's surface and go unexercised.
func sessionsNewEnv(t *testing.T) *sessionsEnv {
	t.Helper()
	dir := t.TempDir()
	wtDir := filepath.Join(dir, "evidence-upload")
	if err := os.MkdirAll(wtDir, 0o700); err != nil {
		t.Fatalf("make the worktree dir: %v", err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st := sessionsMustStore(t)
	env := &sessionsEnv{
		be:  sessionsNewBackend(),
		st:  st,
		wt:  worktree.Static{"w-4a77b1c9": wtDir},
		dir: wtDir,
		loc: &sessionsLocator{
			projects: []project.Project{{ID: "p-8c1d0e42", Name: "panacea", Path: dir}},
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
	a, err := New(Deps{
		Version:      "test",
		Backend:      env.be,
		BackendReady: func() bool { return true },
		Store:        st,
		Flags:        st,
		Projects:     env.loc,
		Worktrees:    env.wt,
		Origin:       httpx.NewOriginPolicy(nil, log),
		Log:          log,
		Image:        "panacea-app/sandbox:dev",
		Resources:    backend.DefaultResources,
		ReadyTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	env.api = a
	// A bare mux, not the handler New returns: that one is wrapped in task 9's
	// loopback Host guard, and httptest.NewRequest sets Host to example.com,
	// which the guard correctly rejects with 403. Origin and Host are task 9's
	// and task 10's to test.
	env.mux = http.NewServeMux()
	a.registerSessions(env.mux)
	return env
}

// do drives the mux this task registered, not task 10's. Origin and Host
// middleware belong to tasks 9 and 10 and are tested there.
func (e *sessionsEnv) do(t *testing.T, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, bytes.NewBufferString(body))
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
```

Two package-level test seams, and the reason they are package-level rather than fields on `API`: `api.go` belongs to task 10 and this task adds nothing to it beyond step 1's three `Deps` fields and one line in `New`, so there is nowhere to hang a `sync.WaitGroup` or an injected clock. The discipline that keeps them race-free is written into the helper below — every test that sets a seam drains one value per watch before it returns, and a channel receive orders the goroutine's read of the variable before `t.Cleanup` writes to it.

```go
// sessionsStubReady replaces the readiness call for one test. Tests using it must not
// call t.Parallel.
func sessionsStubReady(t *testing.T, err error) {
	t.Helper()
	prev := sessionWaitReady
	sessionWaitReady = func(ctx context.Context, endpoint string, timeout time.Duration) error { return err }
	t.Cleanup(func() { sessionWaitReady = prev })
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

	if spec.Image != "panacea-app/sandbox:dev" {
		t.Errorf("Image = %q", spec.Image)
	}
	if spec.SessionID != got.ID {
		t.Errorf("Spec.SessionID = %q, want the session id %q; the label is how the reaper names an orphan", spec.SessionID, got.ID)
	}
	if spec.WorkDir != sessionWorkDir {
		t.Errorf("WorkDir = %q, want %q", spec.WorkDir, sessionWorkDir)
	}
	if len(spec.Mounts) != 1 || spec.Mounts[0].HostPath != env.dir || spec.Mounts[0].ContainerPath != sessionWorkDir {
		t.Fatalf("Mounts = %#v, want one mount of %q at %q", spec.Mounts, env.dir, sessionWorkDir)
	}
	if spec.Mounts[0].ReadOnly {
		t.Error("the worktree is mounted read-only; the agent cannot write the code it was asked to write")
	}
	if spec.Command != "claude" {
		t.Errorf("Command = %q, want claude", spec.Command)
	}
	if strings.Join(spec.Args, "\x00") != strings.Join([]string{"--model", "opus", "--effort", "high", "add tests, then refactor"}, "\x00") {
		t.Errorf("Args = %#v", spec.Args)
	}
	if spec.AttachToken == "" {
		t.Error("Spec.AttachToken is empty; the sandbox would exit 1 at startup")
	}
	if len(spec.Env) != 0 {
		t.Errorf("Env = %#v; this task sets no extra environment, and a PANACEA_APP_* key here would be rejected by EnvEntries anyway", spec.Env)
	}
	sessionsAwaitWatch(t, done)
}
```

```go
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
}

func TestCreateRejectsAMissingWorktreeID(t *testing.T) {
	env := sessionsNewEnv(t)
	if code := sessionsProblemCode(t, env.do(t, http.MethodPost, "/v1/sessions", `{}`), http.StatusBadRequest); code != "bad_request" {
		t.Errorf("error = %q, want bad_request", code)
	}
	// An unknown field is the decoder's rejection, not this handler's.
	rec := env.do(t, http.MethodPost, "/v1/sessions", `{"worktreeId":"w-4a77b1c9","colour":"red"}`)
	if code := sessionsProblemCode(t, rec, http.StatusBadRequest); code != "invalid_json" {
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
			body:       `{"worktreeId":"w-4a77b1c9","harness":"codex"}`,
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
```

```go
// sessionsStarting is the fixture the watch tests drive.
func sessionsStarting(id string) registry.Session {
	return registry.Session{
		ID: id, Title: "t", MachineID: "local", WorktreeID: "w-4a77b1c9",
		Harness: "claude-code", Command: "claude", Args: []string{"--model", "opus"},
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
func TestCreateDoesNotTranslateNonLoopbackIntoAStatus(t *testing.T) {
	env := sessionsNewEnv(t)
	env.be.nonLoopback = true
	sessionsStubReady(t, nil)
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
	if env.be.destroyCount() != 0 {
		t.Errorf("Destroy was called %d times; ruling 10 gives that job to task 13's reaper, which logs it distinctly", env.be.destroyCount())
	}
	sessionsAwaitWatch(t, done)

	// The record is what the reaper reads, so it matters that the endpoint is
	// blank there too rather than carrying an address nothing can reach.
	stored, err := env.st.Get(got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Endpoint != "" {
		t.Errorf("endpoint = %q, want it empty — backend.Sandbox blanks it whenever NonLoopback is set", stored.Endpoint)
	}
	if stored.Status == registry.StatusExited {
		t.Error("the record was marked exited for a container that is running")
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
```

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected — `internal/api` fails to build again. The new references are of two kinds and Go words them differently, which is worth knowing before you read the output. `sessionWaitReady` and `sessionReadyDone` are package-level identifiers, so they produce `undefined: sessionWaitReady`. `registerSessions` and `watchSessionReady` are **methods**, so the compiler says `a.registerSessions undefined (type *API has no field or method registerSessions)` instead. `sessionWorkDir` is **not** in the list: step 3 already declared it. Nine errors, under the ten-error cap, so there is no `too many errors` line, and every one of them is in `sessions_test.go`:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/api [github.com/kpmg-au-emu/panacea-app/control-plane/internal/api.test]
internal\api\sessions_test.go:542:4: a.registerSessions undefined (type *API has no field or method registerSessions)
internal\api\sessions_test.go:595:10: undefined: sessionWaitReady
internal\api\sessions_test.go:596:2: undefined: sessionWaitReady
internal\api\sessions_test.go:597:21: undefined: sessionWaitReady
internal\api\sessions_test.go:605:2: undefined: sessionReadyDone
internal\api\sessions_test.go:606:21: undefined: sessionReadyDone
internal\api\sessions_test.go:818:10: env.api.watchSessionReady undefined (type *API has no field or method watchSessionReady)
internal\api\sessions_test.go:840:10: env.api.watchSessionReady undefined (type *API has no field or method watchSessionReady)
internal\api\sessions_test.go:870:12: env.api.watchSessionReady undefined (type *API has no field or method watchSessionReady)
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/api [build failed]
FAIL
```

Line numbers depend on how you transcribed the file. What must be true: nine errors, every one in `sessions_test.go`, none in `sessions.go` and **none in `api.go`** — `New` does not mention `registerSessions` until step 5 — and every other package still passes.

- [ ] **Step 5: Implement create, the ready watch and the two error mappers**

Everything below reaches its dependencies through `a.deps`, which is the field task 10's `API` actually declares, and every method hangs off `a *API`.

**First, the one line in `api.go`,** now that this step is about to declare the method it names. Task 10's `New` reads, verbatim from `task-10.md:1073-1081`:

```go
	// guarded holds every route the origin middleware protects.
	guarded := http.NewServeMux()
	a.registerCatalog(guarded)
	// Task 11 adds a.registerSessions(guarded) on the next line and task 12
	// adds a.registerAttach(guarded). One line each, and each in the SAME step
	// that defines the method it names: a call added ahead of its definition
	// leaves this file uncompilable for every step in between, so no suite in
	// them can be green. The only other edit either makes here is task 11's
	// three Deps fields: Image, Resources and ReadyTimeout.
```

**"each in the SAME step that defines the method it names" is the load-bearing clause** — it is group-C ruling 2, and it is the whole reason this line is added here and not back in step 1. The closing sentence is the other half of the same bargain: step 1's three `Deps` fields and this one route line are the complete set of edits this task makes to `api.go`, and both are declared in the **Files** entry above rather than smuggled in.

Add the line the comment reserves, immediately below `a.registerCatalog(guarded)`:

```go
	a.registerSessions(guarded)
```

It goes here and not in step 1 because `New` calling a method no file declares stops `package api` compiling for every step in between — group-C ruling 2. **Do not solve a build error by deleting this line instead.** Every test in this task would still pass, because the fixture drives a mux it assembled itself, and `/v1/sessions` would simply not be served by the handler the process actually runs. Step 6 adds the test that catches exactly that.

**Every 500 on this surface goes through task 10's `a.internal(w, r, op, err)`,** which logs the real cause with the method and path and writes contract §3.2's deliberately generic body. Task 10's Interfaces block names it for this task, and writing `httpx.WriteProblem(w, 500, httpx.CodeInternal, …)` by hand instead is how the catalog surface and the session surface end up returning two different 500 bodies. `httpx.WriteProblem` is still used directly for every status that is **not** 500, where the message is part of the contract.

The import block grows in this step and again in steps 7 and 9, because Go rejects an unused import and each pair of steps brings one more. Replace the import block in `control-plane/internal/api/sessions.go` with:

```go
import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)
```

Append to the same file:

```go
// sessionWatchGrace is the slack the readiness watch's own context gets over
// the timeout it passes to WaitReady, so the context is not the thing that
// cancels the wait.
const sessionWatchGrace = 5 * time.Second

// sessionDestroyTimeout bounds a Destroy issued from a background goroutine,
// which has no request context to inherit a deadline from.
const sessionDestroyTimeout = 30 * time.Second

// sessionWaitReady is backend.WaitReady, indirected so a test can drive the
// readiness watch without standing up an HTTP server. Production never
// reassigns it; a test that does must not call t.Parallel.
var sessionWaitReady = backend.WaitReady

// sessionReadyDone is called when a readiness watch has finished and the store
// has been updated. It is nil in production and exists because the watch is a
// goroutine and API is declared in api.go, which this task does not own, so
// there is nowhere to hang a sync.WaitGroup. Task 13 may replace it with one
// when it adds graceful shutdown.
var sessionReadyDone func(sessionID string)

// sessionErrMoved aborts a Store.Update whose precondition no longer holds.
// Returning an error from mutate discards the change, which is exactly the
// behaviour wanted: a watch whose sandbox has been replaced must write nothing.
var sessionErrMoved = errors.New("api: the session moved on while the watch was in flight")

// sessionCreateRequest is the body of POST /v1/sessions, contract §3.9. Every
// field but worktreeId is optional and defaulted below.
type sessionCreateRequest struct {
	MachineID      string `json:"machineId"`
	WorktreeID     string `json:"worktreeId"`
	Harness        string `json:"harness"`
	Model          string `json:"model"`
	Effort         string `json:"effort"`
	PermissionMode string `json:"permissionMode"`
	OpeningPrompt  string `json:"openingPrompt"`
	Title          string `json:"title"`
	Resume         string `json:"resume"`
	ContextRef     string `json:"contextRef"`
}

// registerSessions mounts the six verbs. Go 1.22's ServeMux matches on
// method and path wildcards, so there is no hand-rolled router and no
// switch on r.Method.
func (a *API) registerSessions(mux *http.ServeMux) {
	mux.HandleFunc("POST "+sessionBasePath+"/sessions", a.handleSessionCreate)
	mux.HandleFunc("GET "+sessionBasePath+"/sessions", a.handleSessionList)
	mux.HandleFunc("GET "+sessionBasePath+"/sessions/{id}", a.handleSessionGet)
	mux.HandleFunc("PATCH "+sessionBasePath+"/sessions/{id}", a.handleSessionPatch)
	mux.HandleFunc("DELETE "+sessionBasePath+"/sessions/{id}", a.handleSessionDelete)
	mux.HandleFunc("POST "+sessionBasePath+"/sessions/{id}/restart", a.handleSessionRestart)
}

func (a *API) sessionLog() *slog.Logger {
	if a.deps.Log != nil {
		return a.deps.Log
	}
	return slog.Default()
}

func (a *API) sessionImage() string {
	if a.deps.Image != "" {
		return a.deps.Image
	}
	return "panacea-app/sandbox:dev"
}

// sessionResources refuses a half-filled Resources: Kata sizes a guest VM from
// both numbers and under-requesting either causes a guest OOM.
func (a *API) sessionResources() backend.Resources {
	if a.deps.Resources.CPUMillis > 0 && a.deps.Resources.MemoryMiB > 0 {
		return a.deps.Resources
	}
	return backend.DefaultResources
}

func (a *API) sessionReadyTimeout() time.Duration {
	if a.deps.ReadyTimeout > 0 {
		return a.deps.ReadyTimeout
	}
	return backend.ReadyTimeout
}
```

```go
// handleSessionCreate is contract §5.1, in its order, and the order is the
// point: resolve before creating so a bad id costs nothing, record before the
// sandbox so a failure leaves evidence rather than an unnameable orphan.
func (a *API) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	var req sessionCreateRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()

	req.WorktreeID = strings.TrimSpace(req.WorktreeID)
	if req.WorktreeID == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "worktreeId is required.", nil)
		return
	}

	// 1. The worktree, first.
	path, err := a.deps.Worktrees.Path(ctx, req.WorktreeID)
	if err != nil {
		if errors.Is(err, worktree.ErrUnknownID) || errors.Is(err, worktree.ErrNotADir) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeWorktreeNotFound,
				"No worktree with that id.", map[string]string{"worktreeId": req.WorktreeID})
			return
		}
		a.internal(w, r, "resolve worktree "+req.WorktreeID, err)
		return
	}

	// 2. The harness.
	harnessID := harness.ID(strings.TrimSpace(req.Harness))
	if harnessID == "" {
		harnessID = harness.ClaudeCode
	}
	desc, known := harness.Lookup(harnessID)
	if !known {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeUnknownHarness,
			"No harness with that id.", map[string]string{"harness": string(harnessID)})
		return
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = desc.DefaultModel
	}
	mode := harness.PermissionMode(strings.TrimSpace(req.PermissionMode))
	if mode == "" {
		mode = harness.PermissionAsk
	}
	effort := harness.Effort(strings.TrimSpace(req.Effort))
	command, argv, err := harness.Build(harness.Request{
		Harness:       harnessID,
		Model:         model,
		Effort:        effort,
		Permission:    mode,
		OpeningPrompt: req.OpeningPrompt,
		Resume:        strings.TrimSpace(req.Resume),
	})
	if err != nil {
		a.writeHarnessProblem(w, r, harnessID, desc, err)
		return
	}

	// 3. Ids and the secret.
	sessionID := ids.New("s")
	token := ids.Token()

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = desc.Name + " · " + filepath.Base(filepath.Clean(path))
	}
	// Contract §3.4 and task 10's LocalMachineID: the one machine Phase 2 has
	// is the literal "local". A second default here would be a second answer.
	machineID := strings.TrimSpace(req.MachineID)
	if machineID == "" {
		machineID = LocalMachineID
	}
	projectID, branchID := a.sessionLocate(ctx, req.WorktreeID)

	// 4. The record exists before the sandbox does.
	if err := a.deps.Store.Create(registry.Session{
		ID:             sessionID,
		Title:          title,
		MachineID:      machineID,
		ProjectID:      projectID,
		BranchID:       branchID,
		WorktreeID:     req.WorktreeID,
		Harness:        string(harnessID),
		Model:          model,
		Effort:         string(effort),
		PermissionMode: string(mode),
		OpeningPrompt:  req.OpeningPrompt,
		Command:        command,
		Args:           argv,
		Status:         registry.StatusStarting,
		Backend:        string(a.deps.Backend.Kind()),
		SandboxID:      sessionID,
		AttachToken:    token,
		ContextRef:     req.ContextRef,
	}); err != nil {
		// Task 7's decision 3 fixes these two: an empty id is ErrInvalidID and
		// answers 400, not 409 and not 500. Neither is reachable while
		// sessionID comes from ids.New over crypto/rand, and both are mapped
		// anyway — the first time a defensive branch fires is the worst moment
		// to find out it answered 500 to a client error.
		switch {
		case errors.Is(err, registry.ErrInvalidID):
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
				"The registry rejected the session id.", nil)
		case errors.Is(err, registry.ErrExists):
			httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
				"A session with that id already exists.", map[string]string{"id": sessionID})
		default:
			a.internal(w, r, "record session "+sessionID, err)
		}
		return
	}

	// 5. The sandbox.
	sb, err := a.deps.Backend.Create(ctx, backend.Spec{
		ID:          sessionID,
		SessionID:   sessionID,
		Image:       a.sessionImage(),
		Command:     command,
		Args:        argv,
		AttachToken: token,
		Resources:   a.sessionResources(),
		WorkDir:     sessionWorkDir,
		Mounts:      []backend.Mount{{HostPath: path, ContainerPath: sessionWorkDir}},
	})
	if err != nil {
		// The reason on the record is written for the user; the raw error goes
		// to the log, because it can carry a host path.
		a.sessionLog().Error("create the sandbox", "sessionId", sessionID, "err", err)
		a.markSessionExited(sessionID, "the sandbox could not be created")
		a.writeBackendProblem(w, r, "create the sandbox", err)
		return
	}

	stored, err := a.deps.Store.Update(sessionID, func(sess *registry.Session) error {
		sess.SandboxID = sb.ID
		sess.SandboxRef = sb.Ref
		sess.Endpoint = sb.Endpoint
		return nil
	})
	if err != nil {
		a.internal(w, r, "record the sandbox for session "+sessionID, err)
		return
	}

	// 7. The readiness watch runs on a background context, not r.Context():
	// the request's context is cancelled the moment the 201 is written.
	go a.watchSessionReady(sessionID, sb.ID, sb.Endpoint)

	// 6. Return immediately. The desktop opens the attach socket, which retries
	// while the status is starting.
	w.Header().Set("Location", sessionBasePath+"/sessions/"+sessionID)
	httpx.WriteJSON(w, http.StatusCreated, toJSON(stored, sessionBasePath))
}
```

```go
// sessionLocate finds the project and branch a worktree belongs to, through
// task 10's Enumerator. Both ids are empty when the worktree is not under one
// of the enumerated projects; an empty id is honest, a guessed one is not.
// New rejects a nil Projects, so the nil check is for a hand-built API only.
func (a *API) sessionLocate(ctx context.Context, worktreeID string) (projectID, branchID string) {
	loc := a.deps.Projects
	if loc == nil {
		return "", ""
	}
	projects, err := loc.Projects(ctx)
	if err != nil {
		a.sessionLog().Warn("enumerate projects while creating a session", "err", err)
		return "", ""
	}
	for _, p := range projects {
		worktrees, err := loc.Worktrees(ctx, p.ID)
		if err != nil {
			a.sessionLog().Warn("enumerate worktrees while creating a session", "projectId", p.ID, "err", err)
			continue
		}
		for _, wt := range worktrees {
			if wt.ID == worktreeID {
				return wt.ProjectID, wt.BranchID
			}
		}
	}
	return "", ""
}

// markSessionExited is the one place a session becomes terminal without a
// sandbox to destroy. It clears the endpoint and the token: the port is about
// to be handed back to the OS and may belong to another container within
// seconds, and a token for a sandbox that no longer exists is only a liability.
// An existing reason is kept, because the first reason is the true one.
func (a *API) markSessionExited(sessionID, reason string) {
	if _, err := a.deps.Store.Update(sessionID, func(sess *registry.Session) error {
		sess.Status = registry.StatusExited
		if sess.ExitReason == "" {
			sess.ExitReason = reason
		}
		sess.Endpoint = ""
		sess.AttachToken = ""
		return nil
	}); err != nil && !errors.Is(err, registry.ErrNotFound) {
		a.sessionLog().Error("mark the session exited", "sessionId", sessionID, "err", err)
	}
}

// watchSessionReady is contract §5.1 step 7. It owns exactly one sandbox — the
// one whose id it was given — and it writes to the record only while the record
// still points at that sandbox. Without that guard a slow watch left over from
// before a restart marks the NEW sandbox running with the OLD endpoint, or
// destroys it outright.
func (a *API) watchSessionReady(sessionID, sandboxID, endpoint string) {
	defer func() {
		if done := sessionReadyDone; done != nil {
			done(sessionID)
		}
	}()

	timeout := a.sessionReadyTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout+sessionWatchGrace)
	defer cancel()

	if err := sessionWaitReady(ctx, endpoint, timeout); err != nil {
		a.sessionLog().Warn("the sandbox never became ready",
			"sessionId", sessionID, "sandboxId", sandboxID, "timeout", timeout, "err", err)

		// Destroying is right regardless of what the record now says: a sandbox
		// that never answered /healthz has no user and would be reaped anyway.
		dctx, dcancel := context.WithTimeout(context.Background(), sessionDestroyTimeout)
		if derr := a.deps.Backend.Destroy(dctx, sandboxID); derr != nil && !errors.Is(derr, backend.ErrNotFound) {
			a.sessionLog().Error("destroy a sandbox that never became ready", "sandboxId", sandboxID, "err", derr)
		}
		dcancel()

		if _, err := a.deps.Store.Update(sessionID, func(sess *registry.Session) error {
			if sess.SandboxID != sandboxID {
				return sessionErrMoved
			}
			sess.Status = registry.StatusExited
			// The contract quotes this string in §5.1 step 7. It is the text
			// the cockpit renders, so it is transcribed, not paraphrased.
			sess.ExitReason = "sandbox never became ready"
			sess.Endpoint = ""
			sess.AttachToken = ""
			return nil
		}); err != nil && !errors.Is(err, sessionErrMoved) && !errors.Is(err, registry.ErrNotFound) {
			a.sessionLog().Error("mark the session exited", "sessionId", sessionID, "err", err)
		}
		return
	}

	if _, err := a.deps.Store.Update(sessionID, func(sess *registry.Session) error {
		// starting is the only status this transition is allowed to leave. A
		// session killed while the watch was in flight stays exited.
		if sess.SandboxID != sandboxID || sess.Status != registry.StatusStarting {
			return sessionErrMoved
		}
		sess.Status = registry.StatusRunning
		sess.Endpoint = endpoint
		sess.ExitReason = ""
		return nil
	}); err != nil && !errors.Is(err, sessionErrMoved) && !errors.Is(err, registry.ErrNotFound) {
		a.sessionLog().Error("mark the session running", "sessionId", sessionID, "err", err)
	}
}

// writeHarnessProblem keeps the three harness failures distinct: 400 means the
// request was wrong, 409 means the request was reasonable and the image is not
// ready for it, and the cockpit renders them differently.
func (a *API) writeHarnessProblem(w http.ResponseWriter, r *http.Request, id harness.ID, desc harness.Descriptor, err error) {
	details := map[string]string{"harness": string(id)}
	switch {
	case errors.Is(err, harness.ErrUnknownHarness):
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeUnknownHarness, "No harness with that id.", details)
	case errors.Is(err, harness.ErrUnavailable):
		if desc.Note != "" {
			details["note"] = desc.Note
		}
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeHarnessUnavailable,
			"That harness is not installed in the sandbox image.", details)
	case errors.Is(err, harness.ErrUnsupportedOption), errors.Is(err, harness.ErrUnknownModel):
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeUnsupportedOption, err.Error(), details)
	default:
		a.internal(w, r, "build the argv for harness "+string(id), err)
	}
}

// writeBackendProblem dispatches with errors.Is and never string-matches, which
// is the whole reason task 4 maps every non-101 handshake to a sentinel.
func (a *API) writeBackendProblem(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, backend.ErrUnavailable):
		httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.CodeBackendUnavailable,
			"The sandbox backend is unavailable.", nil)
	case errors.Is(err, backend.ErrUnsupported):
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeUnsupportedOption,
			"The backend cannot honour part of that request.", nil)
	case errors.Is(err, backend.ErrAlreadyExists):
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "That sandbox already exists.", nil)
	case errors.Is(err, backend.ErrNotRunning), errors.Is(err, backend.ErrNotFound):
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeSandboxNotRunning, "The sandbox is not running.", nil)
	case errors.Is(err, backend.ErrNonLoopbackPublish):
		// Loud, and never 200: a published 0.0.0.0 port is a shell on the LAN.
		// The extra line is deliberate duplication — a.internal logs at the
		// same moment, but this one is greppable as SECURITY and names the op.
		a.sessionLog().Error("SECURITY: a sandbox port is not bound to loopback", "op", op, "err", err)
		a.internal(w, r, op, err)
	default:
		a.internal(w, r, op, err)
	}
}
```

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected — `internal/api` still fails to build, and now for a smaller and more specific reason: `registerSessions` mounts six verbs, this step wrote one of them, so it names **five** handlers that do not exist yet. Because they are methods, the wording is the method form, one per reference in `registerSessions`. Note what is **not** here: `api.go` no longer errors, because the `a.registerSessions(guarded)` line added at the top of this step now has a method to call.

```
internal\api\sessions.go:NNN:NN: a.handleSessionList undefined (type *API has no field or method handleSessionList)
internal\api\sessions.go:NNN:NN: a.handleSessionGet undefined (type *API has no field or method handleSessionGet)
internal\api\sessions.go:NNN:NN: a.handleSessionPatch undefined (type *API has no field or method handleSessionPatch)
internal\api\sessions.go:NNN:NN: a.handleSessionDelete undefined (type *API has no field or method handleSessionDelete)
internal\api\sessions.go:NNN:NN: a.handleSessionRestart undefined (type *API has no field or method handleSessionRestart)
```

Five errors, under the ten-error cap, so there is no `too many errors` line this time. If you would rather see the create tests go green before writing the other five handlers, comment out those five `mux.HandleFunc` lines, run, then restore them — but do not commit in that state, and do not extend the trick to the `a.registerSessions(guarded)` line in `New`, which is the one line no test in this file would miss.

- [ ] **Step 6: Write the failing tests for list, get and patch**

The list order is worth a test of its own because it inverts the store's. `registrytest` pins `Store.List` as **ascending** by `createdAt` with the id as the tie-break; contract §3.11 requires the REST surface to be **descending**. Sorting in the handler rather than relying on the store's order and reversing it means the API's contract survives a change to the store's.

**This step also carries the one test that goes through `New`.** Every other test here drives `env.mux`, which the fixture built and registered onto itself, so not one of them would notice if `a.registerSessions(guarded)` were missing from `New` — the routes would exist in the tests and nowhere else, and the suite would be entirely green over a surface the product does not serve. `TestNewActuallyRoutesTheSessionSurface` is the fix, and `GET /v1/sessions` is the right verb for it: it needs no body, no fixture and no backend, so the only thing it can fail on is the wiring. It sets `Host` to a loopback address by hand, because `httptest.NewRequest` sets `example.com` and task 9's guard correctly answers 403 to that.

Append to `control-plane/internal/api/sessions_test.go`:

```go
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
// loopback Host guard, /api/v1 alias and all — rather than the bare mux the
// rest of this file uses. Host is set by hand because httptest.NewRequest
// fills in example.com, which task 9's guard rejects with 403 forbidden_host.
func (e *sessionsEnv) throughNew(t *testing.T, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, target, nil)
	r.Host = "127.0.0.1:8730"
	rec := httptest.NewRecorder()
	e.api.ServeHTTP(rec, r)
	return rec
}

// The line this pins is a.registerSessions(guarded) in New. Delete it and
// every other test in this file still passes, because they drive a mux the
// fixture registered onto itself; only this one goes through the handler the
// process serves. It checks the /api/v1 alias too, because contract §7.5 makes
// that Phase 4's entry point and nothing else in Phase 2 would notice it
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
	// non-loopback Host must not reach the session surface at all.
	r := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil) // Host: example.com
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
```

```go
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

	for name, tc := range map[string]struct {
		body     string
		wantCode string
	}{
		"empty":           {`{"title":""}`, "bad_request"},
		"whitespace only": {`{"title":"   "}`, "bad_request"},
		"absent":          {`{}`, "bad_request"},
		"another field":   {`{"status":"running"}`, "invalid_json"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := env.do(t, http.MethodPatch, "/v1/sessions/s-0000000b", tc.body)
			if code := sessionsProblemCode(t, rec, http.StatusBadRequest); code != tc.wantCode {
				t.Errorf("error = %q, want %q", code, tc.wantCode)
			}
		})
	}
	if code := sessionsProblemCode(t, env.do(t, http.MethodPatch, "/v1/sessions/s-nope", `{"title":"x"}`), http.StatusNotFound); code != "session_not_found" {
		t.Errorf("patching an unknown id: error = %q, want session_not_found", code)
	}
}
```

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected — **byte for byte the same failure as step 5.** The eight new test functions and the four new helpers — `sessionsDecodeList`, `(e *sessionsEnv) throughNew`, `sessionsIDsOf` and `sessionsSeedThree` — add no new undefined symbols of their own: seven of the eight reach the handlers through `env.mux`, and `TestNewActuallyRoutesTheSessionSurface` reaches them through `a.ServeHTTP`, which `api.go` already declares. So the output is the same five method-form lines:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/api [github.com/kpmg-au-emu/panacea-app/control-plane/internal/api.test]
internal\api\sessions.go:149:55: a.handleSessionList undefined (type *API has no field or method handleSessionList)
internal\api\sessions.go:150:60: a.handleSessionGet undefined (type *API has no field or method handleSessionGet)
internal\api\sessions.go:151:62: a.handleSessionPatch undefined (type *API has no field or method handleSessionPatch)
internal\api\sessions.go:152:63: a.handleSessionDelete undefined (type *API has no field or method handleSessionDelete)
internal\api\sessions.go:153:69: a.handleSessionRestart undefined (type *API has no field or method handleSessionRestart)
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/api [build failed]
FAIL
```

If `TestNewActuallyRoutesTheSessionSurface` instead reports `env.api undefined`, the fixture is not keeping the `*API` that `New` returned — step 4's `sessionsNewEnv` stores it in `env.api` precisely so this step can use it.

- [ ] **Step 7: Implement list, get and patch**

Replace the import block in `control-plane/internal/api/sessions.go`, adding `sort`:

```go
import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)
```

Append to the same file:

```go
// sessionPatchRequest is the body of PATCH /v1/sessions/{id}. Title is a
// pointer so an absent field and an empty string are distinguishable: the
// decoder rejects any OTHER field, so a body with no title at all is a request
// that asks for nothing and gets 400 rather than a silent 200.
type sessionPatchRequest struct {
	Title *string `json:"title"`
}

// sessionListResponse is the envelope of contract §3.11. An envelope rather
// than a bare array because a top-level JSON array cannot grow a field later.
type sessionListResponse struct {
	Sessions []sessionJSON `json:"sessions"`
}

// sessionStatuses is the closed set Phase 2 can actually derive. The other
// three in docs/AGENTIC-UX.md §4 rule 8 need agent output, which this phase
// does not read, so a filter naming one of them is a bad request rather than a
// silently empty rail.
var sessionStatuses = map[registry.Status]bool{
	registry.StatusStarting: true,
	registry.StatusRunning:  true,
	registry.StatusIdle:     true,
	registry.StatusExited:   true,
}

func (a *API) handleSessionList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	worktreeID := strings.TrimSpace(query.Get("worktreeId"))

	wanted := map[registry.Status]bool{}
	for _, raw := range query["status"] {
		status := registry.Status(strings.TrimSpace(raw))
		if !sessionStatuses[status] {
			httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
				"status must be one of starting, running, idle or exited.",
				map[string]string{"status": raw})
			return
		}
		wanted[status] = true
	}

	all, err := a.deps.Store.List()
	if err != nil {
		a.internal(w, r, "list sessions", err)
		return
	}

	// Never nil: null and [] are different values to the desktop's decoder.
	out := make([]sessionJSON, 0, len(all))
	for _, sess := range all {
		if worktreeID != "" && sess.WorktreeID != worktreeID {
			continue
		}
		if len(wanted) > 0 && !wanted[sess.Status] {
			continue
		}
		out = append(out, toJSON(sess, sessionBasePath))
	}

	// Descending, per §3.11, sorted here rather than by reversing the store's
	// ascending order: the API's contract must not depend on the store's.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})

	httpx.WriteJSON(w, http.StatusOK, sessionListResponse{Sessions: out})
}

// sessionByID is the 404 every single-session verb shares.
func (a *API) sessionByID(w http.ResponseWriter, r *http.Request) (registry.Session, bool) {
	id := r.PathValue("id")
	sess, err := a.deps.Store.Get(id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"No session with that id.", map[string]string{"id": id})
			return registry.Session{}, false
		}
		a.internal(w, r, "read the session", err)
		return registry.Session{}, false
	}
	return sess, true
}

func (a *API) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.sessionByID(w, r)
	if !ok {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toJSON(sess, sessionBasePath))
}

func (a *API) handleSessionPatch(w http.ResponseWriter, r *http.Request) {
	var req sessionPatchRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.Title == nil {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"title is the only patchable field and it is required.", nil)
		return
	}
	title := strings.TrimSpace(*req.Title)
	if title == "" {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "title cannot be empty.", nil)
		return
	}
	id := r.PathValue("id")
	// Renaming an exited session is allowed, per §3.13: the record outlives the
	// sandbox and the user is naming a transcript.
	updated, err := a.deps.Store.Update(id, func(sess *registry.Session) error {
		sess.Title = title
		return nil
	})
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"No session with that id.", map[string]string{"id": id})
			return
		}
		a.internal(w, r, "rename the session", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toJSON(updated, sessionBasePath))
}
```

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected — still a build failure, now down to the two handlers step 9 writes:

```
internal\api\sessions.go:NNN:NN: a.handleSessionDelete undefined (type *API has no field or method handleSessionDelete)
internal\api\sessions.go:NNN:NN: a.handleSessionRestart undefined (type *API has no field or method handleSessionRestart)
```

- [ ] **Step 8: Write the failing tests for restart, delete and the disconnect rule**

The last test in this step is rule one, in the only form this task can state it: `sessions.go` calls `Backend.Destroy` from three places and a client going away is not one of them. The test drives a create, then does exactly what task 12's relay `defer` does — decrements `attachCount` through `Store.Update` — and asserts the sandbox was never destroyed and the session is still readable.

Append to `control-plane/internal/api/sessions_test.go`:

```go
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
	if after.Command != before.Command || strings.Join(after.Args, "\x00") != strings.Join(before.Args, "\x00") {
		t.Errorf("restart changed the command: %q %v, want %q %v", after.Command, after.Args, before.Command, before.Args)
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

func TestRestartOnAnUnknownIDIs404(t *testing.T) {
	env := sessionsNewEnv(t)
	rec := env.do(t, http.MethodPost, "/v1/sessions/s-nope/restart", "")
	if code := sessionsProblemCode(t, rec, http.StatusNotFound); code != "session_not_found" {
		t.Errorf("error = %q, want session_not_found", code)
	}
}
```

```go
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
```

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected — a build failure with the two method lines from step 7 plus `undefined: sessionRestarts` for each of the two uses in `TestRestartIsRejectedWhileAnotherIsInFlight`. `sessionRestarts` is a package-level variable, so it gets the `undefined:` wording; the handlers are methods, so they get the `has no field or method` wording. Four errors in total, under the cap.

- [ ] **Step 9: Implement restart and delete**

Replace the import block in `control-plane/internal/api/sessions.go` one last time, adding `sync`:

```go
import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/harness"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/worktree"
)
```

Append to the same file:

```go
// sessionIDSet serialises one verb per session id. It is a package-level value
// rather than a field on API because api.go belongs to task 10 and this task
// adds only Deps fields to it. Session ids come from ids.New over crypto/rand,
// so two Servers in one process cannot collide on a real id; two tests that
// hand-write the same id would serialise, which is the correct behaviour
// anyway.
type sessionIDSet struct {
	mu  sync.Mutex
	ids map[string]bool
}

func (s *sessionIDSet) acquire(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	// The init runs BEFORE the read, not after it. Reading a nil map is legal
	// and returns false, so the other order looks harmless — but it is the
	// write on the next line that panics, and the zero value is then only safe
	// by accident of how the package variable below happens to be declared.
	if s.ids == nil {
		s.ids = map[string]bool{}
	}
	if s.ids[id] {
		return false
	}
	s.ids[id] = true
	return true
}

func (s *sessionIDSet) release(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ids, id)
}

// sessionRestarts is §3.14's "409 conflict if a restart is already in flight".
// Two concurrent restarts would otherwise both destroy and both create, and the
// loser's sandbox becomes an orphan holding the session's only live endpoint.
//
// Declared as the zero value, with no map literal: acquire initialises it, and
// a second initialisation here would make that branch dead code that nothing
// ever exercises — which is how a nil-map panic survives a green suite.
var sessionRestarts = &sessionIDSet{}

// handleSessionRestart is §3.14: same config, clean context. It keeps the
// session id, rebuilds the sandbox from the STORED command and args, and mints
// a new attach token. Resume is a different verb — POST /v1/sessions with
// resume set — because resuming continues an old transcript and conflating the
// two makes restart lossy.
func (a *API) handleSessionRestart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !sessionRestarts.acquire(id) {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"A restart is already in flight for that session.", map[string]string{"id": id})
		return
	}
	defer sessionRestarts.release(id)

	sess, ok := a.sessionByID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	// The spec is rebuilt, not replayed, so the mount has to be re-resolved.
	path, err := a.deps.Worktrees.Path(ctx, sess.WorktreeID)
	if err != nil {
		// 409 and not 404: the SESSION exists, it simply cannot be restarted
		// where it was. A 404 here would read as "no such session" and send the
		// user looking for the wrong problem.
		a.sessionLog().Warn("restart: the worktree no longer resolves",
			"sessionId", id, "worktreeId", sess.WorktreeID, "err", err)
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict,
			"The worktree this session was created in no longer resolves.",
			map[string]string{"worktreeId": sess.WorktreeID})
		return
	}

	if sess.SandboxID != "" {
		if err := a.deps.Backend.Destroy(ctx, sess.SandboxID); err != nil && !errors.Is(err, backend.ErrNotFound) {
			a.sessionLog().Error("restart: destroy the old sandbox",
				"sessionId", id, "sandboxId", sess.SandboxID, "err", err)
			a.writeBackendProblem(w, r, "destroy the old sandbox", err)
			return
		}
	}

	// A FRESH sandbox id, not the old one. Reusing it races a Destroy the
	// daemon has not finished and comes back as ErrAlreadyExists; minting a new
	// one also means a container that survived the destroy stops matching any
	// record, so task 13's reaper sees an orphan and removes it.
	sandboxID := ids.New("s")
	token := ids.Token()
	sb, err := a.deps.Backend.Create(ctx, backend.Spec{
		ID:          sandboxID,
		SessionID:   sess.ID,
		Image:       a.sessionImage(),
		Command:     sess.Command,
		Args:        sess.Args,
		AttachToken: token,
		Resources:   a.sessionResources(),
		WorkDir:     sessionWorkDir,
		Mounts:      []backend.Mount{{HostPath: path, ContainerPath: sessionWorkDir}},
	})
	if err != nil {
		a.sessionLog().Error("restart: create the sandbox", "sessionId", id, "err", err)
		a.markSessionExited(id, "the sandbox could not be recreated")
		a.writeBackendProblem(w, r, "create the sandbox", err)
		return
	}

	updated, err := a.deps.Store.Update(id, func(sess *registry.Session) error {
		sess.SandboxID = sb.ID
		sess.SandboxRef = sb.Ref
		sess.Endpoint = sb.Endpoint
		sess.AttachToken = token
		sess.Status = registry.StatusStarting
		sess.ExitReason = ""
		// AttachCount is deliberately untouched. Task 12's relay defer
		// decrements it when the dropped socket notices; zeroing it here would
		// take the count negative.
		return nil
	})
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"No session with that id.", map[string]string{"id": id})
			return
		}
		a.internal(w, r, "record the restart", err)
		return
	}

	go a.watchSessionReady(id, sb.ID, sb.Endpoint)
	httpx.WriteJSON(w, http.StatusOK, toJSON(updated, sessionBasePath))
}
```

```go
// handleSessionDelete is §3.15 and it is one of the three places in this phase
// that is allowed to destroy a sandbox. Without purge the record stays: the
// transcript is the artifact and docs/FUNCTIONAL-SPEC.md requires finished
// sessions stay listed and readable.
func (a *API) handleSessionDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := a.sessionByID(w, r)
	if !ok {
		return
	}
	id := sess.ID
	ctx := r.Context()
	purge := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("purge")), "true")

	if sess.SandboxID != "" {
		// Already gone is success. An explicit kill and the reaper race by
		// design, and a delete that errors on "already gone" makes the cockpit
		// show a failure for work that completed.
		if err := a.deps.Backend.Destroy(ctx, sess.SandboxID); err != nil && !errors.Is(err, backend.ErrNotFound) {
			// The record is NOT marked exited here. The container is probably
			// still running, and reporting it exited would be a fabricated
			// status.
			a.sessionLog().Error("destroy the sandbox", "sessionId", id, "sandboxId", sess.SandboxID, "err", err)
			a.writeBackendProblem(w, r, "destroy the sandbox", err)
			return
		}
	}

	// Marked exited even on the purge path, so a purge that fails half-way
	// leaves an honest record rather than one that still claims to be running.
	updated, err := a.deps.Store.Update(id, func(sess *registry.Session) error {
		sess.Status = registry.StatusExited
		// An existing reason is kept: the first reason is the true one, and
		// killing an already-exited session must not rewrite its history.
		if sess.ExitReason == "" {
			sess.ExitReason = "killed"
		}
		sess.Endpoint = ""
		sess.AttachToken = ""
		// SandboxID is kept. It is how task 13's reaper tells a container that
		// belongs to a known session from an orphan, and it is the only
		// diagnostic left once the endpoint is cleared.
		return nil
	})
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"No session with that id.", map[string]string{"id": id})
			return
		}
		a.internal(w, r, "record the kill", err)
		return
	}

	if !purge {
		httpx.WriteJSON(w, http.StatusOK, toJSON(updated, sessionBasePath))
		return
	}

	if err := a.deps.Store.Delete(id); err != nil && !errors.Is(err, registry.ErrNotFound) {
		// The sandbox is already gone by this point, so the detail an operator
		// needs is "destroyed, not purged" — and it goes in the log, where
		// a.internal puts it, rather than into a 500 body that would then be
		// the only 500 on this surface that reads differently from the
		// catalog's.
		a.internal(w, r, "purge the session record", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected — `internal/api` builds and all **32** top-level tests in `sessions_test.go` pass, alongside task 10's `api_test.go` and `catalog_test.go`. `go test` prints one `ok` line per package with a time:

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/api	6.5s
```

Six and a half seconds is what `-race` costs this package on a Windows laptop; your own timing will differ and only the `ok` matters. If a test fails here, read its message before changing code: each one names the rule it is defending.

- [ ] **Step 10: Verify the whole workspace, vet and gofmt**

Three commands, and all three must be clean before the commit step. `go test ./...` is invalid at this workspace root — it fails with `directory prefix . does not contain modules listed in go.work` — so the package list is always explicit.

```bash
go test ./sandboxd/... ./control-plane/... -race -count=1
go vet ./control-plane/...
gofmt -l control-plane/internal/api
```

`gofmt -l` prints the name of every file it would change and nothing at all when the tree is clean; no output is the pass condition. `go vet` is worth its own run here rather than relying on the one `go test` performs, because this task has several `t.Errorf` calls with more arguments than verbs waiting to happen and vet's `printf` check is what catches them.

Two things to confirm by eye, because no test can:

1. `grep -n "Destroy(" control-plane/internal/api/sessions.go` returns exactly three hits — the ready watch's failure path, restart, and delete. A fourth is rule one broken.
2. `grep -n "AttachToken\|Endpoint" control-plane/internal/api/sessions.go` shows no occurrence inside `toJSON` or `sessionJSON`.

- [ ] **Step 11: Commit**

The commit message goes to a file first. A multi-line heredoc crossing an extra shell boundary can terminate early, and the symptom — `here-document delimited by end-of-file` followed by the message body executing as commands — is worse than the two extra lines this costs.

`api.go` appears in the `git add` only if steps 1 and 5 actually changed it — step 1 adds `Image`, `Resources` and `ReadyTimeout` to `Deps`, step 5 adds `a.registerSessions(guarded)` to `New`. If task 10 as landed already carries all four, drop the path from the list; committing an unchanged path is harmless but a diff that claims a change it did not make is not. Before committing, confirm the line is there:

```bash
grep -cE '^[[:space:]]*a\.registerSessions\(guarded\)' control-plane/internal/api/api.go   # exactly 1
```

**The anchor is the check, not decoration.** Task 10's reserving comment (`task-10.md:1073-1081`) carries the literal string `a.registerSessions(guarded)` in prose, so the unanchored `grep -c "a.registerSessions(guarded)"` prints `1` before step 5 and `2` after it — measured on a file holding that comment — and so can never tell "the line is missing" from "the line is there". `^[[:space:]]*a\.` matches only a line whose first non-space content is the call, which a `//` comment cannot be: it prints `0` before and `1` after. Step 6's `TestNewActuallyRoutesTheSessionSurface` must also be green — that test is the only thing in this task that fails if the line is missing.

```bash
printf '%s\n' \
'feat(api): session CRUD, with the token and endpoint sealed off the wire' \
'' \
'POST/GET/PATCH/DELETE /v1/sessions and POST /v1/sessions/{id}/restart,' \
'in contract 5.1 order: resolve the worktree first so a bad id costs' \
'nothing, record the session before the sandbox so a failed create leaves' \
'evidence rather than an unnameable orphan, 201 immediately, then watch' \
'for readiness on a background context.' \
'' \
'api.sessionJSON is a separate struct from registry.Session because' \
'attachToken, endpoint, sandboxId, sandboxRef, command and args must never' \
'reach the wire and cannot be tagged json:"-" on the record, which has to' \
'persist them. The test searches the marshalled bytes for the token and the' \
'endpoint and pins the key set at the twenty-one names of contract 3.10.' \
'' \
'Destroy is called from three places and a dropped WebSocket is not one of' \
'them (contract 5.2). Finished sessions stay listed and readable because' \
'claude --resume reads its state from ~/.claude inside the sandbox.' \
'' \
'The readiness watch owns one sandbox id and writes to the record only' \
'while the record still points at it, so a watch left over from before a' \
'restart cannot mark the new sandbox running or destroy it.' \
'' \
'backend.Sandbox.NonLoopback is not translated into a session status: the' \
'reaper owns that container, and calling a running one "exited" would be a' \
'fabricated status.' \
'' \
'The in-memory Store double runs registrytest.RunStoreContract, so it' \
'cannot drift from FileStore by being quietly more forgiving.' \
'' \
'Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>' \
> /tmp/task-11-commit.txt

git add control-plane/internal/api/sessions.go \
        control-plane/internal/api/sessions_test.go \
        control-plane/internal/api/api.go
git commit -F /tmp/task-11-commit.txt
```

Then confirm the tree is clean and the commit holds what it claims:

```bash
git status --short
git show --stat HEAD
```

