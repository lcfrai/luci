### Task 7: Where state lives, and the session registry

The control plane restarts — a rebuild, a crash, the user quitting the desktop app — and every sandbox it started keeps running without it. Nothing in Phase 2 reconstructs those sandboxes from the Docker daemon alone, because a container label carries an id and nothing else: not the title the user typed, not the worktree it was cut against, not the attach token that makes its PTY reachable. This task is the record that survives the restart. It is two packages. `internal/statepath` answers "where on this machine does state live" exactly once. `internal/registry` holds the session rows, and every task from 9 onward reads or writes them.

`statepath` is a contract rather than a detail because it is shared. Phase 4's `discoverstore.DefaultDir()` already contains this ladder character-for-character (contract §7.1), and the moment one copy gains a case the two disagree about where the user's data is — which does not present as an error, it presents as an empty cockpit and a file nobody can find. `Dir` is parameterised by subdirectory so the registry is `statepath.Dir("sessions")` and Phase 4 becomes `statepath.Dir("discover")`. It creates nothing: the caller knows whether a missing directory is an error and `statepath` does not.

`Update` being the only mutation primitive is the whole design of the store, and it is worth saying plainly what it buys. Three goroutines mutate a session concurrently — the HTTP handler, the attach relay writing `LastOutputAt`, and the reaper — and a `Get`, edit, `Put` sequence loses whichever write finishes first, silently, under load, on the field that matters least often. `Update` runs the caller's mutate function while the store holds its write lock, so read-modify-write is atomic by construction and there is no interleaving to get wrong. The seal has one hole that value semantics do not close on their own: `Session` holds `Args []string`, so a plain struct copy shares its backing array and a caller that appends to a returned `Args` edits the store's copy in place, outside the lock. `Session.Clone` closes it, and the store clones on every boundary crossing — in on `Create`, out of `Get` and `List`, in and out of `Update`. A test pins the seal from the outside: it captures the `*Session` that `mutate` is handed, writes to it *after* `Update` has returned, reloads the document from disk, and asserts the late write did **not** survive. If that test ever goes green in the other direction, the in-memory index and the persisted document can diverge and the cockpit will show state the file does not contain.

The worktree `ready` flag lands here, and the reason is order rather than preference. `docs/FUNCTIONAL-SPEC.md` lists "nothing ever sets `ready`, so user-created worktrees can never merge" among the ten defects this pass fixes, and adjudicated Cross-group decision 2 keeps the verb and makes it work. It persists as one more map in the registry document — `"worktreeFlags": {"w-4a77b1c9": {"ready": true}}` — written under the same write lock and through the same atomic replace, reached by a narrow `WorktreeFlagStore` rather than by widening `Store`. Added now it is an additive change inside `schemaVersion: 1`; added after the registry ships it is a schema bump and a migration for a single boolean. It is the only new persisted state Phase 2 needs. One field on the record is a secret and stays one: `Session.AttachToken` is persisted because a restart must be able to re-attach, and it is **never** serialised onto the REST surface — task 11 owns the `sessionJSON` projection that makes that a compile-time fact rather than a review question.

Seven decisions the contract left open, made here so no reader re-derives them. **(1) `List` returns sessions sorted by `CreatedAt` ascending with `ID` as the tie-break.** The contract says ordering matters to the rail but not what the order is; Go map iteration is randomised, so an unsorted `List` reshuffles the rail on every two-second poll, and an insertion-order slice is a second structure that must stay in sync with `Delete` and with a hand-edited file. A sort has nothing to corrupt. **(2) The store stamps time itself:** `UpdatedAt` after every successful `Update`, and `CreatedAt`/`UpdatedAt` on `Create` when the caller left them zero, always `.UTC()`. Asking every caller to remember is the class of defect `Update`-only exists to prevent, and UTC because the contract's document is `Z`-suffixed and two machines' registries should diff. **(3) A third sentinel, `ErrInvalidID`,** for `Create` with an empty id: that is neither "not found" nor "already exists", and task 11 must answer `400` to it rather than `409`. One sentinel now beats a string-matched error later. **(4) `Session.Clone` is exported,** because task 11's in-memory test double needs the same guarantee as `FileStore` and a private copy helper would be reimplemented there, differently. **(5) The worktree-flag methods sit on their own narrow interface, `WorktreeFlagStore`, and not on `Store`.** The Go contract declares `Store` with five session methods and task 11 is written against that shape; widening it would force every test double in the API package to implement methods it has no opinion about, while task 10 needs exactly two calls — every flag at once to decorate the worktree list, and one flag set to answer the `PATCH`. Reading flags for an unknown worktree returns the zero value and **no error**, because a worktree the registry has never seen is not an error — it is a worktree that is not ready, which is precisely what `ready: false` means. **(6) `Dir("")` returns the state root**, which `filepath.Join(base, "")` already produces; pinning it in a test stops a caller that wants the root from inventing a second ladder. **(7) `registry` imports neither `backend` nor `ids`.** `Session.Backend` holds a `backend.Kind` value as a plain `string` and `Session.Status` is registry's own type, exactly as the contract spells them, so both packages stay standard-library-only and neither can pull the other into a cycle. Test fixtures use literal ids rather than `ids.New`, because a random id cannot be compared against a golden JSON document.

**Files:**
- `control-plane/internal/statepath/statepath.go` — the one OS ladder for state directories, shared verbatim with Phase 4 (new)
- `control-plane/internal/statepath/statepath_test.go` — every rung of the ladder, written with `t.Setenv` so it is the same test on Windows and Linux (new)
- `control-plane/internal/registry/session.go` — `Session`, `Status` and its four constants, `Session.Clone` (new)
- `control-plane/internal/registry/registry.go` — the `Store` interface, the three sentinels, `DefaultPath` (new)
- `control-plane/internal/registry/session_test.go` — the JSON field names the TypeScript side depends on, the four status strings, clone depth, `DefaultPath` (new)
- `control-plane/internal/registry/registrytest/registrytest.go` — `package registrytest`: the behaviour suite every `Store` implementation must pass, including the seal. A normal package rather than a `_test.go` file, because an external test package (`package registry_test`) cannot be imported by anything, and task 11's in-memory double has to run this same suite. It imports `testing` at ordinary build time, exactly as `net/http/httptest` and `testing/fstest` do (new)
- `control-plane/internal/registry/filestore.go` — `FileStore`, `WorktreeFlags` and `WorktreeFlagStore`: the in-memory index, the five `Store` methods (`Create`, `Get`, `List`, `Update`, `Delete`) and the `var _ Store = (*FileStore)(nil)` assertion that proves they are the interface's, one JSON document at `schemaVersion: 1`, the `worktreeFlags` map, the Windows-safe atomic replace, and the quarantine rename for an unparseable file — whose suffix is stamped `2006-01-02T15-04-05Z` rather than the contract's literal RFC3339, because RFC3339 contains colons and `os.Rename` to a name holding a `:` fails on Windows with "The filename, directory name, or volume label syntax is incorrect". The contract text cannot be implemented as written on the platform this ships on; the stamp keeps its sort order and loses only the colons (new, written later in this task)
- `control-plane/internal/registry/filestore_test.go` — `package registry`: `FileStore` against `t.TempDir()`, the document round trip, corrupt-file quarantine, version refusal and the worktree flags (new, written later in this task)
- `control-plane/internal/registry/filestore_contract_test.go` — `package registry_test`: the call into the shared contract suite, plus the properties it cannot reach — the `Create` and `List` halves of the seal, UTC stamping on `Create`, `Update` refusing an id change, `Delete` leaving its siblings alone, and the index roll-back when a save fails. It is a second file rather than more of `filestore_test.go` because it asserts only against the exported surface — every property here is one task 11's double must satisfy too — while `filestore_test.go` is `package registry` and reaches `writeAtomic` and `load` directly, and one file cannot be both packages (new, written later in this task)
- `control-plane/internal/registry/writeatomic_test.go` — `package registry`: the atomic replace on its own, including the Windows case where a reader holds the target open. Go's `os.Open` does not ask for `FILE_SHARE_DELETE`, so on Windows that handle blocks `MoveFileEx` outright: the bounded retry buys out a holder that lets go inside half a second, and a holder that does not makes the write fail closed with the previous document intact. The control plane never does this to itself — `load` reads with `os.ReadFile`, which closes before it returns — so the case only arises from an editor or a scanner outside the process (new, written later in this task)

**Interfaces:**
- Produces:
  - `statepath.EnvStateDir` (`"PANACEA_APP_STATE_DIR"`), `statepath.Dir(sub string) string`
  - `registry.Status` with `StatusStarting` / `StatusRunning` / `StatusIdle` / `StatusExited` — four, and only four
  - `registry.Session`, with the twenty-five JSON field names exactly as the Go contract spells them, and `(Session).Clone() Session`
  - `registry.WorktreeFlags{Ready bool}` — JSON `{"ready": bool}`, declared in `filestore.go` beside the document it persists in
  - `registry.Store` — `Create(Session) error`, `Get(id) (Session, error)`, `List() ([]Session, error)`, `Update(id, func(*Session) error) (Session, error)`, `Delete(id) error`. Five methods, exactly as the Go contract declares them
  - `registry.WorktreeFlagStore` — `WorktreeFlagsSnapshot() map[string]WorktreeFlags`, `SetWorktreeReady(worktreeID string, ready bool) (WorktreeFlags, error)`. The worktree seam is a second interface rather than two more methods on `Store`; task 10 takes this one
  - `registry.ErrNotFound`, `registry.ErrExists`, `registry.ErrInvalidID`, `registry.ErrSchemaTooNew`
  - `registry.DefaultPath() string` — `statepath.Dir("sessions")` + `/sessions.json`
  - `registry.FileStore`, `registry.NewFileStore(path string) (*FileStore, error)`, and `var _ registry.Store = (*registry.FileStore)(nil)`
  - `registrytest.NewStoreFunc` and `registrytest.RunStoreContract(t *testing.T, newStore NewStoreFunc)`, plus the fixtures `registrytest.StorePath`, `registrytest.MustOpen` and `registrytest.Seed` — the shared behaviour suite in `package registrytest`, a normal importable package. `filestore_contract_test.go` calls it and task 11's in-memory double calls the same suite, which is the only way the two implementations are held to one contract rather than to two sets of tests that drifted
- Consumes:
  - Nothing at compile time. Both packages are standard-library only, which is what lets every other package in the phase depend on them.
  - Task 4 by value, not by import: the ids stored here are minted by `ids.New("s")`, `ids.New("w")`, `ids.New("p")`, `ids.New("b")` and the attach token by `ids.Token()`, in tasks 9 and 11; `Session.Backend` carries `backend.KindDocker` or `backend.KindKata` as a string.
- Consumed by: tasks 9–13 (session lifecycle, REST surface, attach relay, reaper) and task 10's worktree `ready` verb. Phase 4's `discoverstore.DefaultDir()` becomes `statepath.Dir("discover")` whenever someone chooses to make the substitution; nothing breaks if nobody does.

- [ ] **Step 1: Write the failing tests for `internal/statepath`**

Every rung of the ladder gets a test, and every test runs on Windows and on Linux with the same assertions. That is possible because `Dir` reads its answer out of the environment and `t.Setenv` lets a test own the environment: `LOCALAPPDATA` is set explicitly rather than assumed absent on Linux, and `HOME` and `USERPROFILE` are both pointed at a `t.TempDir()` so `os.UserHomeDir()` resolves the same way on both platforms. Expected paths are built with `filepath.Join`, never with string concatenation, so a backslash on Windows and a slash on Linux are both correct without a build tag.

What these pin: the precedence order, which is the only thing about the ladder that can silently regress; that `Dir` creates nothing, because a `statepath` that quietly mkdirs would make Phase 4's "is this the first run" check unanswerable; and that `Dir("sessions")` and `Dir("discover")` are siblings, which is the whole of contract §7.1 expressed as an assertion rather than a paragraph.

Create `control-plane/internal/statepath/statepath_test.go`:

```go
package statepath_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath"
)

// clearLadder empties every environment variable Dir reads, so a test starts
// from a known rung instead of from whatever the developer's machine exports.
// t.Setenv restores the previous value when the test ends and panics if the
// test has called t.Parallel, which is why nothing in this file is parallel.
func clearLadder(t *testing.T) {
	t.Helper()
	t.Setenv(statepath.EnvStateDir, "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("XDG_STATE_HOME", "")
}

// homeAt points os.UserHomeDir at dir on every platform. Go reads USERPROFILE
// on Windows and HOME everywhere else, and consults nothing else, so setting
// both makes this one test rather than two behind build tags.
func homeAt(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestEnvStateDirIsTheSharedVariableName(t *testing.T) {
	// Phase 4's discoverstore reads this exact string. Renaming it here without
	// renaming it there splits the user's state across two trees.
	if statepath.EnvStateDir != "PANACEA_APP_STATE_DIR" {
		t.Fatalf("EnvStateDir = %q, want %q", statepath.EnvStateDir, "PANACEA_APP_STATE_DIR")
	}
}

func TestStateDirEnvWinsOverEveryOtherRung(t *testing.T) {
	clearLadder(t)
	homeAt(t, t.TempDir())
	root := t.TempDir()
	t.Setenv(statepath.EnvStateDir, root)
	t.Setenv("LOCALAPPDATA", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	want := filepath.Join(root, "sessions")
	if got := statepath.Dir("sessions"); got != want {
		t.Fatalf("Dir(\"sessions\") = %q, want %q", got, want)
	}
}

func TestLocalAppDataRung(t *testing.T) {
	clearLadder(t)
	homeAt(t, t.TempDir())
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	want := filepath.Join(local, "PanaceaApp", "state", "sessions")
	if got := statepath.Dir("sessions"); got != want {
		t.Fatalf("Dir(\"sessions\") = %q, want %q", got, want)
	}
}

func TestXDGStateHomeRung(t *testing.T) {
	clearLadder(t)
	homeAt(t, t.TempDir())
	xdg := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdg)

	want := filepath.Join(xdg, "panacea-app", "sessions")
	if got := statepath.Dir("sessions"); got != want {
		t.Fatalf("Dir(\"sessions\") = %q, want %q", got, want)
	}
}

func TestHomeRung(t *testing.T) {
	clearLadder(t)
	home := t.TempDir()
	homeAt(t, home)

	want := filepath.Join(home, ".local", "state", "panacea-app", "sessions")
	if got := statepath.Dir("sessions"); got != want {
		t.Fatalf("Dir(\"sessions\") = %q, want %q", got, want)
	}
}

func TestTempFallbackWhenHomeIsUnknowable(t *testing.T) {
	clearLadder(t)
	// An empty HOME and USERPROFILE make os.UserHomeDir return an error —
	// "$HOME is not defined" on Unix, "%userprofile% is not defined" on Windows
	// — which is the only way to reach the last rung without deleting a real
	// user's profile.
	homeAt(t, "")

	want := filepath.Join(os.TempDir(), "panacea-app", "sessions")
	if got := statepath.Dir("sessions"); got != want {
		t.Fatalf("Dir(\"sessions\") = %q, want %q", got, want)
	}
}

func TestDirCreatesNothing(t *testing.T) {
	clearLadder(t)
	homeAt(t, t.TempDir())
	root := filepath.Join(t.TempDir(), "not-yet")
	t.Setenv(statepath.EnvStateDir, root)

	dir := statepath.Dir("sessions")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("stat %s: err = %v, want a not-exist error — Dir must not create anything", dir, err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("stat %s: err = %v, want a not-exist error — Dir must not create its parent either", root, err)
	}
}

func TestDirEmptySubReturnsTheStateRoot(t *testing.T) {
	clearLadder(t)
	homeAt(t, t.TempDir())
	root := t.TempDir()
	t.Setenv(statepath.EnvStateDir, root)

	// filepath.Join drops the empty element and cleans the result, so the root
	// comes back with no trailing separator. Pinned so that a caller wanting the
	// root does not build a second ladder to get it.
	want := filepath.Clean(root)
	if got := statepath.Dir(""); got != want {
		t.Fatalf("Dir(\"\") = %q, want %q", got, want)
	}
}

func TestSessionsAndDiscoverAreSiblings(t *testing.T) {
	// Contract §7.1 as an assertion: Phase 2's registry and Phase 4's discover
	// store live under one parent, so one PANACEA_APP_STATE_DIR moves both.
	clearLadder(t)
	homeAt(t, t.TempDir())
	t.Setenv("LOCALAPPDATA", t.TempDir())

	sessions := statepath.Dir("sessions")
	discover := statepath.Dir("discover")
	if filepath.Dir(sessions) != filepath.Dir(discover) {
		t.Fatalf("not siblings: %q and %q", sessions, discover)
	}
	if sessions == discover {
		t.Fatalf("Dir ignored its argument: both are %q", sessions)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/statepath/... -race -count=1`

Expected: a build failure, not a test failure, because the directory holds a test file and nothing else.

```
github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath: no non-test Go files in C:\Users\au-dev-dmackie1\Projects\PanaceaApp\control-plane\internal\statepath
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath [build failed]
FAIL
```

That is the right failure: it proves the package resolves through the workspace under its real import path and that nothing has been implemented yet. Two ways it goes wrong instead. If you see `no required module provides package github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath`, the directory was created outside `control-plane/` or the import path in the test file does not match `control-plane/go.mod`'s module line — the prefix is `github.com/kpmg-au-emu/panacea-app/control-plane`, with `/control-plane` in it. If `-race` itself errors with a message about cgo or a missing `gcc`, this toolchain predates the pure-Go Windows race runtime; re-run without `-race` and say so in the step's output rather than pretending the detector ran.

- [ ] **Step 3: Write `internal/statepath`**

This is contract §2.7 unchanged. Do not add a `runtime.GOOS` switch: the ladder is selected by environment, not by platform, and that is exactly what lets one test cover both. Do not add a `MkdirAll`.

Create `control-plane/internal/statepath/statepath.go`:

```go
// Package statepath answers one question: where on this machine does the control
// plane keep state. It exists because two packages need the same answer —
// registry (Phase 2) and discoverstore (Phase 4) — and two copies of an OS
// ladder drift the moment one of them gains a case.
package statepath

import (
	"os"
	"path/filepath"
)

// EnvStateDir overrides every other rule.
const EnvStateDir = "PANACEA_APP_STATE_DIR"

// Dir returns the directory for a named kind of state, e.g. Dir("sessions") or
// Dir("discover"). The ladder, in order:
//
//	$PANACEA_APP_STATE_DIR/<sub>
//	%LOCALAPPDATA%\PanaceaApp\state\<sub>        (Windows)
//	$XDG_STATE_HOME/panacea-app/<sub>
//	~/.local/state/panacea-app/<sub>
//	<os.TempDir()>/panacea-app/<sub>             (only if the home directory is unknowable)
//
// The rungs are chosen by environment rather than by runtime.GOOS on purpose:
// %LOCALAPPDATA% is simply unset off Windows, so the same code walks the same
// ladder everywhere and one test covers every platform.
//
// Dir does not create anything. The caller does, because the caller knows
// whether a missing directory is an error.
func Dir(sub string) string {
	if dir := os.Getenv(EnvStateDir); dir != "" {
		return filepath.Join(dir, sub)
	}
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		return filepath.Join(dir, "PanaceaApp", "state", sub)
	}
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "panacea-app", sub)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "panacea-app", sub)
	}
	return filepath.Join(home, ".local", "state", "panacea-app", sub)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./control-plane/internal/statepath/... -race -count=1`

Expected, with the duration varying a lot and meaning less than it looks. The number after `ok <pkg>` is the test binary's **run** time, not the build — the `-race` build is separate and is not printed anywhere. Under `-v` these nine tests sum to about 0.13s, so nearly all of the figure below is fixed per-package overhead: launching a freshly linked `-race` binary past Defender. Measured on one Windows 11 laptop: 1.5s to 2.2s warm, and 4.3s to 5.8s on the first run after a cold file cache. Do not read a number in this range as a signal about the tests.

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath	1.756s
```

Two failures worth recognising on sight. `TestLocalAppDataRung` failing on Linux with the `~/.local/state` path in `got` means the `LOCALAPPDATA` rung was wrapped in a `runtime.GOOS == "windows"` guard; remove the guard rather than the test. `TestTempFallbackWhenHomeIsUnknowable` failing on Windows with a `got` under the user's real profile means `os.UserHomeDir()` was replaced by a hand-rolled `HOMEDRIVE`+`HOMEPATH` lookup, which Go itself does not consult and Phase 4 does not either.

- [ ] **Step 5: Write the failing tests for the session record and the store contract, and watch them fail**

Two files, because they pin two different things. `session_test.go` pins the record: the twenty-five JSON key names that the TypeScript side reads, the four status strings, how far `Clone` copies, and where `DefaultPath` lands. `registrytest/registrytest.go` pins behaviour, as a suite parameterised by a constructor so that every implementation of `Store` — `FileStore` now, task 11's in-memory double later — is held to the same rules by the same code. Writing it as a shared suite rather than as `FileStore` tests is the difference between an interface that is documented and an interface that is enforced.

The suite sits in its own package, `registrytest`, and not in a `_test.go` file, because Go will not let an external test package (`package registry_test`) be imported by anybody — so a suite written there could be called from exactly one file and the "every implementation" promise would be a comment. `registrytest.go` is therefore an ordinary source file that imports `testing` at build time. That is the shape `net/http/httptest` and `testing/fstest` have for the same reason, and it costs a `testing` import in any binary that imports `registrytest` — which is test code only, because nothing else does.

The fixture is the contract's own example document, field for field, and two of its fields are deliberately empty. `exitReason` and `contextRef` being present as `""` in the marshalled JSON is the proof that no field carries `omitempty`; the desktop types them as `string`, not `string | undefined`, and a key that vanishes when a session has not exited yet breaks that.

Create `control-plane/internal/registry/session_test.go`:

```go
package registry_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath"
)

// sampleSession is the contract's example document as a Go value. Ids are
// literals rather than ids.New calls: a random id cannot be compared against a
// golden document, and the prefixes are what this fixture is pinning.
func sampleSession() registry.Session {
	return registry.Session{
		ID:             "s-3f2a91c0",
		Title:          "evidence upload",
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
		Args:           []string{"--model", "opus", "--effort", "high", "add tests, then refactor"},
		Status:         registry.StatusRunning,
		ExitReason:     "",
		Backend:        "docker",
		SandboxID:      "s-3f2a91c0",
		SandboxRef:     "9c1f0b7ad3e2",
		Endpoint:       "127.0.0.1:52431",
		AttachToken:    "Qk9fV0lMTF9CRV9SRVBMQUNFRF9CWV8zMl9SQU5ET01f",
		CreatedAt:      time.Date(2026, 9, 18, 9, 14, 3, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 9, 18, 9, 14, 7, 0, time.UTC),
		LastOutputAt:   time.Date(2026, 9, 18, 9, 14, 6, 0, time.UTC),
		AttachCount:    1,
		ContextRef:     "",
	}
}

func TestSessionJSONKeySet(t *testing.T) {
	raw, err := json.Marshal(sampleSession())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	want := []string{
		"args", "attachCount", "attachToken", "backend", "branchId",
		"command", "contextRef", "createdAt", "effort", "endpoint",
		"exitReason", "harness", "id", "lastOutputAt", "machineId",
		"model", "openingPrompt", "permissionMode", "projectId", "sandboxId",
		"sandboxRef", "status", "title", "updatedAt", "worktreeId",
	}
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("JSON keys =\n  %v\nwant\n  %v", keys, want)
	}

	// Empty in the fixture on purpose: their presence proves no field carries
	// omitempty, which is what lets the desktop type them as string.
	for _, k := range []string{"exitReason", "contextRef"} {
		if string(got[k]) != `""` {
			t.Fatalf("%s = %s, want \"\" — omitempty would drop the key entirely", k, got[k])
		}
	}
}

func TestSessionTimesSerialiseAsRFC3339UTC(t *testing.T) {
	raw, err := json.Marshal(sampleSession())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, want := range []string{
		`"createdAt":"2026-09-18T09:14:03Z"`,
		`"updatedAt":"2026-09-18T09:14:07Z"`,
		`"lastOutputAt":"2026-09-18T09:14:06Z"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("marshalled session does not contain %s\ngot: %s", want, raw)
		}
	}
}

func TestSessionRoundTripsThroughJSON(t *testing.T) {
	want := sampleSession()
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got registry.Session
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	// reflect.DeepEqual is safe here only because every time in the fixture is
	// UTC and carries no monotonic reading. A fixture built from time.Now()
	// would fail this comparison; that is a property of time.Time rather than a
	// bug in Session, and the fix is a UTC fixture, not a looser assertion.
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip changed the record:\n got %+v\nwant %+v", got, want)
	}
}

func TestStatusValues(t *testing.T) {
	// Four, and only four. The other three in docs/AGENTIC-UX.md cannot be
	// derived without reading agent output, which docs/PROGRAM.md forbids.
	for _, tc := range []struct {
		got  registry.Status
		want string
	}{
		{registry.StatusStarting, "starting"},
		{registry.StatusRunning, "running"},
		{registry.StatusIdle, "idle"},
		{registry.StatusExited, "exited"},
	} {
		if string(tc.got) != tc.want {
			t.Errorf("status = %q, want %q", tc.got, tc.want)
		}
	}
}

func TestCloneDeepCopiesArgs(t *testing.T) {
	orig := sampleSession()
	clone := orig.Clone()

	clone.Args[0] = "--model=sonnet"
	if orig.Args[0] != "--model" {
		t.Fatalf("writing clone.Args[0] changed the original to %q — Clone shared the backing array", orig.Args[0])
	}

	clone.Args = append(clone.Args, "--extra")
	if len(orig.Args) != 5 {
		t.Fatalf("appending to clone.Args changed the original length to %d, want 5", len(orig.Args))
	}

	// A nil Args stays nil. Whether the REST surface emits null or [] is task
	// 11's projection to settle; the record does not invent a slice.
	var empty registry.Session
	if empty.Clone().Args != nil {
		t.Fatal("Clone fabricated an empty Args slice from a nil one")
	}
}

func TestCloneCopiesEveryOtherField(t *testing.T) {
	orig := sampleSession()
	if !reflect.DeepEqual(orig.Clone(), orig) {
		t.Fatal("Clone is not value-identical to its source — a field was dropped")
	}
}

func TestDefaultPathIsSessionsJSONUnderTheSessionsStateDir(t *testing.T) {
	root := t.TempDir()
	t.Setenv(statepath.EnvStateDir, root)

	want := filepath.Join(root, "sessions", "sessions.json")
	if got := registry.DefaultPath(); got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestSentinelsAreDistinct(t *testing.T) {
	all := []error{registry.ErrNotFound, registry.ErrExists, registry.ErrInvalidID}
	for i, a := range all {
		if a == nil {
			t.Fatalf("sentinel %d is nil", i)
		}
		for j, b := range all {
			if i != j && errors.Is(a, b) {
				t.Fatalf("sentinels %d and %d are the same error; the API cannot then tell 400 from 404 from 409", i, j)
			}
		}
	}
}
```

Two more silences in the contract are settled by this suite, and the reasons belong here rather than in a code comment. `Delete` of an unknown id returns `ErrNotFound` rather than `nil`: task 13's reaper deletes sessions it believes are gone, and a silent no-op turns "the reaper is deleting the same record every ten seconds" into a condition with no symptom. And when `mutate` returns an error, `Update` discards the change entirely and returns that error unwrapped, so `errors.Is` against the caller's own sentinel works — a half-applied mutation is the one outcome the write lock exists to prevent.

Create `control-plane/internal/registry/registrytest/registrytest.go`:

```go
package registrytest

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// NewStoreFunc opens a Store at path, creating it when it does not exist.
// Calling it twice with one path must reopen the same state: several tests
// below prove a property by reloading, which is the only way to tell an
// in-memory edit from a persisted one.
type NewStoreFunc func(path string) (registry.Store, error)

// RunStoreContract holds any Store implementation to the rules in contract
// §2.8. filestore_contract_test.go calls it with NewFileStore, and task 11's
// in-memory double is expected to call it too: an interface with one tested
// implementation is a type declaration rather than a contract.
//
// This package is not a _test.go file, so it imports testing at normal build
// time. That is deliberate and is what net/http/httptest and testing/fstest do:
// a suite only two packages can reach is a suite an external test package
// cannot import at all. Do not "fix" it by renaming the file.
func RunStoreContract(t *testing.T, newStore NewStoreFunc) {
	t.Helper()
	t.Run("CreateGetListDelete", func(t *testing.T) { testCreateGetListDelete(t, newStore) })
	t.Run("CreateRejectsDuplicateAndEmptyID", func(t *testing.T) { testCreateRejects(t, newStore) })
	t.Run("UpdateAppliesAndStamps", func(t *testing.T) { testUpdateStamps(t, newStore) })
	t.Run("UpdateErrorDiscardsTheChange", func(t *testing.T) { testUpdateErrorDiscardsTheChange(t, newStore) })
	t.Run("TheSealHolds", func(t *testing.T) { testSeal(t, newStore) })
	t.Run("ListIsOrderedByCreatedAt", func(t *testing.T) { testListOrder(t, newStore) })
	t.Run("ConcurrentUpdatesAllLand", func(t *testing.T) { testConcurrentUpdates(t, newStore) })
}

// StorePath puts the document inside the test's own temp directory. t.TempDir
// is removed when the test ends, on Windows as well as Linux, so no test here
// leaves a file behind or collides with a sibling.
func StorePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "sessions.json")
}

// MustOpen opens the store or fails the test. StorePath, MustOpen and Seed are
// exported because filestore_contract_test.go and task 11's double write their
// own tests beside the suite and must build the same fixtures the suite does.
func MustOpen(t *testing.T, newStore NewStoreFunc, path string) registry.Store {
	t.Helper()
	s, err := newStore(path)
	if err != nil {
		t.Fatalf("open store at %s: %v", path, err)
	}
	return s
}

// Seed is the fixture every test here starts from: the fields a session cannot
// be without, and nothing a single test would then have to work around.
func Seed(id string, created time.Time) registry.Session {
	return registry.Session{
		ID:        id,
		Title:     "session " + id,
		MachineID: "local",
		Harness:   "claude-code",
		Command:   "claude",
		Args:      []string{"--model", "opus"},
		Status:    registry.StatusStarting,
		Backend:   "docker",
		CreatedAt: created,
	}
}

func testCreateGetListDelete(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)

	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "session s-00000001" {
		t.Fatalf("Title = %q, want %q", got.Title, "session s-00000001")
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("Create left CreatedAt zero; the store stamps it when the caller has not")
	}
	if got.CreatedAt.Location() != time.UTC {
		t.Fatalf("CreatedAt location = %v, want UTC", got.CreatedAt.Location())
	}
	if _, err := s.Get("s-nope"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("Get unknown id: err = %v, want ErrNotFound", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d sessions, want 1", len(list))
	}

	// Reopening proves the write reached the file rather than only the index.
	reloaded := MustOpen(t, newStore, path)
	if _, err := reloaded.Get("s-00000001"); err != nil {
		t.Fatalf("after reload, Get: %v", err)
	}

	if err := s.Delete("s-00000001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("s-00000001"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("after Delete, Get: err = %v, want ErrNotFound", err)
	}
	if err := s.Delete("s-00000001"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("second Delete: err = %v, want ErrNotFound", err)
	}
	if _, err := MustOpen(t, newStore, path).Get("s-00000001"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("Delete did not reach the file: err = %v, want ErrNotFound", err)
	}
}

func testCreateRejects(t *testing.T, newStore NewStoreFunc) {
	s := MustOpen(t, newStore, StorePath(t))

	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if err := s.Create(Seed("s-00000001", time.Time{})); !errors.Is(err, registry.ErrExists) {
		t.Fatalf("duplicate Create: err = %v, want ErrExists", err)
	}
	if err := s.Create(Seed("", time.Time{})); !errors.Is(err, registry.ErrInvalidID) {
		t.Fatalf("Create with an empty id: err = %v, want ErrInvalidID", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("a rejected Create still landed: List returned %d sessions, want 1", len(list))
	}
}

func testUpdateStamps(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)
	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}
	before, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	got, err := s.Update("s-00000001", func(sess *registry.Session) error {
		sess.Status = registry.StatusRunning
		sess.AttachCount++
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Status != registry.StatusRunning || got.AttachCount != 1 {
		t.Fatalf("Update returned %q/%d, want running/1", got.Status, got.AttachCount)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("Update left UpdatedAt zero; the store stamps it after mutate returns")
	}
	// Not-before rather than strictly-after: the Windows wall clock can tick in
	// steps of roughly 15 ms, so two updates inside one tick share a timestamp
	// and a strict comparison would fail on that machine and nowhere else.
	if got.UpdatedAt.Before(before.UpdatedAt) {
		t.Fatalf("UpdatedAt went backwards: %v then %v", before.UpdatedAt, got.UpdatedAt)
	}
	if !got.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("Update moved CreatedAt from %v to %v", before.CreatedAt, got.CreatedAt)
	}

	if _, err := s.Update("s-nope", func(*registry.Session) error { return nil }); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("Update unknown id: err = %v, want ErrNotFound", err)
	}

	after, err := MustOpen(t, newStore, path).Get("s-00000001")
	if err != nil {
		t.Fatalf("after reload, Get: %v", err)
	}
	if after.Status != registry.StatusRunning || after.AttachCount != 1 {
		t.Fatalf("Update did not reach the file: %q/%d, want running/1", after.Status, after.AttachCount)
	}
}

func testUpdateErrorDiscardsTheChange(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)
	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	boom := errors.New("boom")
	if _, err := s.Update("s-00000001", func(sess *registry.Session) error {
		sess.Status = registry.StatusExited
		return boom
	}); !errors.Is(err, boom) {
		t.Fatalf("Update: err = %v, want the mutate error returned unwrapped", err)
	}

	got, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != registry.StatusStarting {
		t.Fatalf("a failed mutate was committed to the index: status = %q, want starting", got.Status)
	}
	after, err := MustOpen(t, newStore, path).Get("s-00000001")
	if err != nil {
		t.Fatalf("after reload, Get: %v", err)
	}
	if after.Status != registry.StatusStarting {
		t.Fatalf("a failed mutate reached the file: status = %q, want starting", after.Status)
	}
}

func testSeal(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)
	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Capture the pointer mutate is handed, then write through it after Update
	// has returned. If the store hands out a pointer into its own index, this
	// late write lands in memory and never reaches the file, and the cockpit
	// then shows state the document does not contain.
	var escaped *registry.Session
	if _, err := s.Update("s-00000001", func(sess *registry.Session) error {
		escaped = sess
		sess.Title = "committed"
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	escaped.Title = "smuggled"
	escaped.Args[0] = "--smuggled"

	got, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "committed" {
		t.Fatalf("a write through the escaped pointer reached the index: title = %q, want committed", got.Title)
	}
	if got.Args[0] != "--model" {
		t.Fatalf("the escaped pointer still shares the Args array: %q, want --model", got.Args[0])
	}

	after, err := MustOpen(t, newStore, path).Get("s-00000001")
	if err != nil {
		t.Fatalf("after reload, Get: %v", err)
	}
	if after.Title != "committed" || after.Args[0] != "--model" {
		t.Fatalf("the escaped write survived a reload: %q / %q, want committed / --model", after.Title, after.Args[0])
	}

	// The same seal on the read side: a caller editing what Get returned must
	// not be editing the store.
	got.Title = "edited by the caller"
	got.Args[0] = "--edited"
	again, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if again.Title != "committed" || again.Args[0] != "--model" {
		t.Fatalf("Get returned a view into the index: %q / %q, want committed / --model", again.Title, again.Args[0])
	}
}

func testListOrder(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)

	base := time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)
	// Created out of order, and two share a timestamp so the id tie-break is
	// exercised rather than assumed.
	for _, tc := range []struct {
		id      string
		created time.Time
	}{
		{"s-0000000c", base.Add(2 * time.Hour)},
		{"s-0000000a", base},
		{"s-0000000b", base.Add(time.Hour)},
		{"s-0000000d", base.Add(2 * time.Hour)},
	} {
		if err := s.Create(Seed(tc.id, tc.created)); err != nil {
			t.Fatalf("Create %s: %v", tc.id, err)
		}
	}

	want := []string{"s-0000000a", "s-0000000b", "s-0000000c", "s-0000000d"}
	for _, store := range []registry.Store{s, MustOpen(t, newStore, path)} {
		list, err := store.List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got := make([]string, 0, len(list))
		for _, sess := range list {
			got = append(got, sess.ID)
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("List order = %v, want %v — in map order the rail reshuffles on every poll", got, want)
		}
	}
}

func testConcurrentUpdates(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)
	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Update("s-00000001", func(sess *registry.Session) error {
				sess.AttachCount++
				return nil
			}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Update: %v", err)
	}

	got, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AttachCount != n {
		t.Fatalf("AttachCount = %d, want %d — a read-modify-write was lost, which is the whole reason Update exists", got.AttachCount, n)
	}
	after, err := MustOpen(t, newStore, path).Get("s-00000001")
	if err != nil {
		t.Fatalf("after reload, Get: %v", err)
	}
	if after.AttachCount != n {
		t.Fatalf("after reload AttachCount = %d, want %d", after.AttachCount, n)
	}
}
```

Run: `go test ./control-plane/internal/registry/... -race -count=1`

Expected: the same build failure as Step 2, for the same reason — the directory holds test files and nothing else. `registrytest` fails alongside it, because it imports a package that has no non-test files to build; the error is reported once and both packages are marked failed.

```
github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry: no non-test Go files in C:\Users\au-dev-dmackie1\Projects\PanaceaApp\control-plane\internal\registry
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry [build failed]
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest [build failed]
FAIL
```

If instead you see this, `session_test.go` was written with `package registry` rather than `package registry_test`:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry
package github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry
	imports github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry from session_test.go: import cycle not allowed in test
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry [setup failed]
?   	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest	[no test files]
FAIL
```

It is `[setup failed]` rather than `[build failed]` and the file never reaches the type checker, so there is no list of `undefined:` errors to read — the self-import is caught while the package graph is being loaded. (Delete the import as well as the package clause and you do get `undefined: registry`, which is the same mistake one step further along.) `session_test.go` is an external test package on purpose, and `registrytest` reaches the same surface through an ordinary import: both may only use the exported names, because that is the surface every other task in the phase gets.

- [ ] **Step 6: Write the session record and the store declaration, and watch their tests pass**

Two files. `session.go` holds the record and nothing else; `registry.go` holds the interface, the sentinels and `DefaultPath`. Neither imports anything beyond the standard library and `internal/statepath`, which is what lets every other package in the phase depend on this one.

`Clone` is the load-bearing function here. The rule it enforces is that a `Session` crossing the store's boundary is always a copy all the way down, so there is exactly one place — inside `Update`, under the write lock — where the stored record changes.

Create `control-plane/internal/registry/session.go`:

```go
package registry

import "time"

// Status is the session status the user sees.
//
// PHASE 2 SHIPS FOUR, AND ONLY FOUR. docs/AGENTIC-UX.md §4 rule 8 specifies a
// closed set of seven (PLANNING, EXECUTING, BLOCKED-ON-YOU, BLOCKED-ON-MACHINE,
// PAUSED, DRIFTING, DONE). Five of those cannot be derived without reading agent
// output, which docs/PROGRAM.md forbids absolutely. The four below are the facts
// the control plane can actually observe. BLOCKED-ON-YOU needs the agent-runtime
// signal docs/AGENTIC-UX.md §3 calls "a hard runtime dependency, not a UI task".
// It is not invented here and must not be guessed from PTY bytes.
type Status string

const (
	// StatusStarting: the sandbox has been created and has not yet answered
	// /healthz.
	StatusStarting Status = "starting"
	// StatusRunning: the sandbox answers /healthz and its process is alive.
	StatusRunning Status = "running"
	// StatusIdle: running, but no output has crossed the relay for
	// Config.IdleAfter. Derived from a byte COUNT, never from byte content.
	StatusIdle Status = "idle"
	// StatusExited: the process ended, the sandbox was killed, or the sandbox
	// disappeared. Terminal. The record is kept — docs/FUNCTIONAL-SPEC.md:
	// "Finished sessions stay listed and readable — the transcript is the
	// artifact."
	StatusExited Status = "exited"
)

// Session is one row of the registry: everything the control plane must survive
// its own restart with.
//
// AttachToken and Endpoint are persisted and are NEVER serialised onto the REST
// surface. api.sessionJSON exists to make that a compile-time fact rather than a
// review question.
//
// No field carries omitempty, deliberately. The desktop types every one of them
// as present, and a key that disappears while a session has not exited yet is a
// key the TypeScript side would have to defend against for no reason.
type Session struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	MachineID  string `json:"machineId"`
	ProjectID  string `json:"projectId"`
	BranchID   string `json:"branchId"`
	WorktreeID string `json:"worktreeId"`

	Harness        string   `json:"harness"` // harness.ID
	Model          string   `json:"model"`
	Effort         string   `json:"effort"`
	PermissionMode string   `json:"permissionMode"`
	OpeningPrompt  string   `json:"openingPrompt"`
	Command        string   `json:"command"` // resolved argv[0], for restart
	Args           []string `json:"args"`    // resolved argv[1:], for restart

	Status     Status `json:"status"`
	ExitReason string `json:"exitReason"`

	Backend     string `json:"backend"`     // backend.Kind, held as a string so
	SandboxID   string `json:"sandboxId"`   // registry imports no other package
	SandboxRef  string `json:"sandboxRef"`  // container id / pod name
	Endpoint    string `json:"endpoint"`    // host:port, revalidated on every boot
	AttachToken string `json:"attachToken"` // secret; see the type comment

	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	LastOutputAt time.Time `json:"lastOutputAt"`
	AttachCount  int       `json:"attachCount"` // currently-connected clients

	// ContextRef is the Phase 4 brief this session inherited, or "". Phase 2
	// stores and returns it and never interprets it.
	ContextRef string `json:"contextRef"`
}

// Clone returns a copy that shares nothing with s.
//
// A plain struct copy is not enough, and the reason is one field: Args is a
// slice, so two struct "copies" write through the same backing array — outside
// the store's lock, where nothing serialises them. Every Session crossing the
// store's boundary is cloned: in on Create, out of Get and List, in and out of
// Update. A nil Args stays nil; whether the REST surface emits null or [] is
// the projection's decision, not the record's.
func (s Session) Clone() Session {
	out := s
	if s.Args != nil {
		out.Args = make([]string, len(s.Args))
		copy(out.Args, s.Args)
	}
	return out
}
```

Create `control-plane/internal/registry/registry.go`:

```go
// Package registry holds every session this control plane knows about, and is
// the only state in Phase 2 that outlives the process.
//
// CONCURRENCY DISCIPLINE, AND IT IS NOT OPTIONAL: no caller ever performs a
// Get-then-Put. Three goroutines mutate a session concurrently — the HTTP
// handler, the attach relay writing LastOutputAt, and the reaper — and
// Get-then-Put loses whichever write finishes first. Update is the only mutation
// primitive; the mutate function runs while the store holds its write lock, so
// read-modify-write is atomic by construction. Create exists to add a row, and
// takes no mutate function because a row nobody can see yet has nothing to race
// with.
package registry

import (
	"errors"
	"path/filepath"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath"
)

// ErrNotFound is returned for an unknown session id.
var ErrNotFound = errors.New("registry: no such session")

// ErrExists is returned when Create is given an id that is already taken.
var ErrExists = errors.New("registry: session already exists")

// ErrInvalidID is returned when Create is given an empty id. It is separate
// from ErrExists so the REST surface can answer 400 rather than 409: an empty
// id is a caller defect, not a collision.
var ErrInvalidID = errors.New("registry: invalid session id")

// Store holds every session this control plane knows about.
//
// Sessions cross this boundary by value, and deeply: an implementation clones
// on the way in and on the way out, so a caller can never hold a reference into
// the index. The pointer Update hands to mutate is the store's own working copy
// and must not outlive the call — the store commits that copy when mutate
// returns nil and discards it entirely when mutate returns an error.
//
// List returns sessions sorted by CreatedAt ascending with ID as the tie-break.
// Map order would reshuffle the rail on every two-second poll.
//
// Five methods and no more. The worktree ready flag persists in the same
// document and under the same lock, but it is reached through WorktreeFlagStore
// in filestore.go: every consumer of Store cares about sessions, and a widened
// Store would make each of their test doubles implement worktree methods they
// have no opinion about.
type Store interface {
	Create(s Session) error
	Get(id string) (Session, error)
	List() ([]Session, error)
	Update(id string, mutate func(*Session) error) (Session, error)
	Delete(id string) error
}

// DefaultPath is statepath.Dir("sessions") + "/sessions.json".
//
// It resolves at call time rather than in a package variable, because both the
// tests and the desktop set PANACEA_APP_STATE_DIR after this package has been
// initialised, and a cached value would ignore them.
func DefaultPath() string {
	return filepath.Join(statepath.Dir("sessions"), "sessions.json")
}
```

Run: `go test ./control-plane/internal/registry/... -race -count=1`

Expected — the record tests run and pass, and the contract suite compiles without running, because nothing has called `RunStoreContract` yet. `registrytest` holds the suite and no tests of its own, so it reports `[no test files]` here and at every later run:

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry	2.104s
?   	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest	[no test files]
```

Three failures worth recognising on sight. `undefined: registry.FileStore` means the suite reached for the implementation: `registrytest.go` names only the `Store` interface precisely so it compiles ahead of it, so the stray reference is a typo rather than a missing file. A missing key in `TestSessionJSONKeySet` — most often `sandboxId` reported as absent while `sandboxID` is present — means a tag was written in Go casing; the desktop reads these names literally and `json.Marshal` will not correct them. And `TestCloneDeepCopiesArgs` failing on its first assertion — "writing clone.Args[0] changed the original" — means `Clone` copied the slice header and reused the array, which is what `out := s` does on its own when the `Args` block underneath it is missing or was guarded into never running.

- [ ] **Step 7: Write the failing tests for the atomic replace**

The registry document is rewritten in full on every mutation, so the replace is the
single operation that can lose every running session. `os.Rename` is the only
primitive that is atomic on both POSIX and Windows, but on Windows it is
`MoveFileEx` with `MOVEFILE_REPLACE_EXISTING`, and that call fails while another
process holds the target open. Windows Defender and the search indexer open files
briefly and opportunistically, so the symptom is an intermittent
`Access is denied.` on a rename that worked a hundred times before, and it never
reproduces on a Linux CI box. A bounded retry is the fix for that case, and only
for that case: a holder that keeps its handle for longer than the loop's 300 ms
outlives it, and the save then fails with the previous document intact rather
than replaced. Failing closed is the right outcome — writing in place to dodge
the error would trade a lost mutation for a truncated registry.

These tests pin five properties: the first write creates the file, its parent
directory and mode `0o600`, because the document carries attach tokens; the replace
works when the target **already exists** (the case a first-run-only test misses
entirely); it does the right thing on each platform while a reader holds the
target open — the replace goes through on POSIX, and on Windows it fails closed
with the previous document still on disk byte for byte; it leaves no
`.sessions-*.json` litter behind; and a replace that cannot succeed returns an
error instead of a half-written file.

The last of those — "an interrupted write leaves the **previous** document intact
rather than a truncated one" — cannot be tested by killing the process mid-write
inside `go test`. It is tested as the two observable halves of the same invariant:
a failed replace leaves the original bytes untouched (here), and a leftover temp
file from a write that never reached its rename is ignored by the next load
(Step 8, `TestLoadIgnoresALeftoverTempFile`). Together they say what a crash
between `Write` and `Rename` costs: the newest mutation, never the document.

Create `control-plane/internal/registry/writeatomic_test.go`:

```go
package registry

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteAtomicCreatesTheFileAndItsDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "sessions.json")

	if err := writeAtomic(path, []byte("first\n")); err != nil {
		t.Fatalf("writeAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "first\n" {
		t.Fatalf("content = %q, want %q", got, "first\n")
	}

	// The document carries attach tokens, so the mode matters. On Windows the
	// mode bits are advisory and os.Chmod only moves the read-only attribute,
	// so the assertion is skipped there rather than weakened everywhere.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("mode = %v, want -rw-------", perm)
		}
	}
}

func TestWriteAtomicReplacesAFileThatAlreadyExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	if err := os.WriteFile(path, []byte("old and much longer than the new one\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := writeAtomic(path, []byte("new\n")); err != nil {
		t.Fatalf("writeAtomic over an existing file: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "new\n" {
		t.Fatalf("content = %q, want %q", got, "new\n")
	}
}

func TestWriteAtomicWhileAReaderHoldsTheTargetOpen(t *testing.T) {
	// The two platforms genuinely differ here, so the test says so rather than
	// asserting one of them everywhere. Go's os.Open reaches CreateFile through
	// syscall.Open, which hard-codes FILE_SHARE_READ|FILE_SHARE_WRITE and NOT
	// FILE_SHARE_DELETE (src/syscall/syscall_windows.go), so on Windows any open
	// handle — this one, an editor's, a scanner's — makes MoveFileEx return
	// ERROR_ACCESS_DENIED, and a handle held for the whole window outlives the
	// bounded retry. On POSIX a rename onto an open file is ordinary and the
	// reader simply keeps the old inode.
	//
	// The control plane never trips this on itself: load reads with os.ReadFile,
	// which opens, reads and closes before returning, so no reader is alive
	// across its own write. An external holder can still block a save, and the
	// property both branches pin is the one that matters when it does — the
	// target is never left half-written.
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	reader, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer reader.Close()

	err = writeAtomic(path, []byte("new\n"))

	if runtime.GOOS == "windows" {
		// Fail closed, and say which operation failed.
		if err == nil {
			t.Fatal("writeAtomic succeeded with a reader attached; MoveFileEx cannot replace a file another handle holds open, so either the share flags changed or writeAtomic stopped using os.Rename")
		}
		if !strings.Contains(err.Error(), "registry: replace") {
			t.Fatalf("error = %v, want it wrapped as \"registry: replace <path>\"", err)
		}
		// This is the guarantee the registry lives or dies by: the save was
		// lost, the document was not. A writeAtomic that fell back to writing
		// in place to dodge the error would trade this clean failure for a
		// truncated file.
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read back after the failed replace: %v", readErr)
		}
		if string(got) != "old\n" {
			t.Fatalf("content = %q after a failed replace, want the previous content %q", got, "old\n")
		}
		if n := countTempFiles(t, dir); n != 0 {
			t.Fatalf("%d leftover temp files after a failed write, want 0", n)
		}
		return
	}

	if err != nil {
		t.Fatalf("writeAtomic with a reader attached: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "new\n" {
		t.Fatalf("content = %q, want %q", got, "new\n")
	}
	if n := countTempFiles(t, dir); n != 0 {
		t.Fatalf("%d leftover temp files after a successful write, want 0", n)
	}
}

func TestWriteAtomicLeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")

	for i := 0; i < 3; i++ {
		if err := writeAtomic(path, []byte("round\n")); err != nil {
			t.Fatalf("writeAtomic round %d: %v", i, err)
		}
	}

	if n := countTempFiles(t, dir); n != 0 {
		t.Fatalf("%d leftover temp files after three successful writes, want 0", n)
	}
}

func TestWriteAtomicFailureLeavesThePreviousContentIntact(t *testing.T) {
	// A replace that can never succeed: the target path is an existing,
	// non-empty directory. Renaming a file onto it fails on every OS, which is
	// the only portable way to drive writeAtomic's retry loop to exhaustion.
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "occupant"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed occupant: %v", err)
	}

	err := writeAtomic(path, []byte("new\n"))
	if err == nil {
		t.Fatal("writeAtomic onto an unreplaceable target returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "registry: replace") {
		t.Fatalf("error = %v, want it to name the operation and the path", err)
	}

	// Nothing was clobbered and nothing was left behind.
	if _, statErr := os.Stat(filepath.Join(path, "occupant")); statErr != nil {
		t.Fatalf("the pre-existing content was disturbed: %v", statErr)
	}
	if n := countTempFiles(t, dir); n != 0 {
		t.Fatalf("%d leftover temp files after a failed write, want 0", n)
	}
}

func countTempFiles(t *testing.T, dir string) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".sessions-*.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return len(matches)
}
```

- [ ] **Step 8: Write the failing tests for the document, its load rules and the worktree flags**

These tests pin what the document **is** and how a bad one behaves. Four decisions
are load-bearing, and each has a test that fails loudly if a later change reverses
it:

1. **A missing file is first run, not an error.** §5.5 step 1 says so directly.
2. **An unparseable document is moved aside and the store starts empty.** Refusing
   to start leaves the user with a cockpit that will not open and no way to fix it;
   the rename to `<path>.corrupt-<stamp>` means nothing is discarded. Silently
   starting empty *without* the rename would be the data-loss version of the same
   behaviour, so the test asserts the quarantine file exists and holds the original
   bytes.
3. **A `schemaVersion` the binary does not know is a hard error, and the file is
   left exactly where it is.** A newer build has already written fields this build
   would drop on its next save, so starting is the destructive option. The contract
   requires an error naming both versions; the test asserts both numbers appear.
4. **A `schemaVersion` *below* 1 is quarantined, not rejected.** The contract is
   silent here: version 1 is the first, so the only way to see `0` is a document
   that never had the field — a truncated write, something hand-edited, or a file
   from a pre-release build. That is corruption, and it takes the corruption path.
   Refusing to start for it would strand a user behind a file that no migration
   will ever exist for.

The round-trip test also fixes the field-name spelling, because those names are a
contract with the TypeScript side, and it carries `attachToken` deliberately: §5.5
notes the token survives in the file precisely so a restart can re-attach to a
running sandbox. It never reaches the REST surface — task 11's `sessionJSON`
projection is what makes that a compile-time fact — but it must be on disk.

Create `control-plane/internal/registry/filestore_test.go`:

```go
package registry

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sampleSession is deliberately fully populated: every field §5.5 needs to
// re-attach to a live sandbox after a control-plane restart must survive the
// round trip, and a zero value would let a dropped field pass unnoticed.
func sampleSession() Session {
	at := time.Date(2026, 9, 18, 9, 14, 3, 0, time.UTC)
	return Session{
		ID:             "s-3f2a91c0",
		Title:          "evidence upload",
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
		Status:         StatusRunning,
		ExitReason:     "",
		Backend:        "docker",
		SandboxID:      "s-3f2a91c0",
		SandboxRef:     "9c1f0b7ad3e2",
		Endpoint:       "127.0.0.1:52431",
		AttachToken:    "not-a-real-token-but-shaped-like-one",
		CreatedAt:      at,
		UpdatedAt:      at.Add(4 * time.Second),
		LastOutputAt:   at.Add(3 * time.Second),
		AttachCount:    1,
		ContextRef:     "",
	}
}

func newStore(t *testing.T, path string) *FileStore {
	t.Helper()
	st, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore(%s): %v", path, err)
	}
	return st
}

func TestNewFileStoreOnAMissingFileIsAnEmptyStoreNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")

	st := newStore(t, path)

	list, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List returned %d sessions on first run, want 0", len(list))
	}
	// Opening must not create the file: nothing has been mutated yet, and a
	// stray empty document would make "has this app ever run" unanswerable.
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat after open = %v, want the file to still be absent", err)
	}
}

func TestSessionsSurviveAReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	want := sampleSession()

	first := newStore(t, path)
	if err := first.Create(want); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := newStore(t, path).Get(want.ID)
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}

	if got.AttachToken != want.AttachToken {
		t.Fatalf("attachToken = %q, want it persisted so a restart can re-attach", got.AttachToken)
	}
	if got.Endpoint != want.Endpoint || got.SandboxRef != want.SandboxRef || got.Backend != want.Backend {
		t.Fatalf("sandbox identity lost: endpoint=%q ref=%q backend=%q", got.Endpoint, got.SandboxRef, got.Backend)
	}
	if got.Command != want.Command || len(got.Args) != len(want.Args) {
		t.Fatalf("restart argv lost: command=%q args=%v", got.Command, got.Args)
	}
	if got.Status != want.Status || got.AttachCount != want.AttachCount {
		t.Fatalf("status/attachCount = %q/%d, want %q/%d", got.Status, got.AttachCount, want.Status, want.AttachCount)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) || !got.LastOutputAt.Equal(want.LastOutputAt) {
		t.Fatalf("timestamps lost: createdAt=%s lastOutputAt=%s", got.CreatedAt, got.LastOutputAt)
	}
}

func TestDocumentUsesTheContractFieldNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	if err := newStore(t, path).Create(sampleSession()); err != nil {
		t.Fatalf("Create: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read document: %v", err)
	}
	var doc struct {
		SchemaVersion int                        `json:"schemaVersion"`
		SavedAt       string                     `json:"savedAt"`
		Sessions      []map[string]any           `json:"sessions"`
		WorktreeFlags map[string]map[string]bool `json:"worktreeFlags"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("the document this build wrote does not parse: %v\n%s", err, raw)
	}

	if doc.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", doc.SchemaVersion)
	}
	if doc.SavedAt == "" {
		t.Fatal("savedAt is empty; it is diagnostic but it is not optional")
	}
	if len(doc.Sessions) != 1 {
		t.Fatalf("sessions has %d entries, want 1", len(doc.Sessions))
	}
	// These spellings are a contract with the TypeScript side. A rename here is
	// a silent break there, so it fails here first.
	for _, key := range []string{
		"id", "title", "machineId", "projectId", "branchId", "worktreeId",
		"harness", "model", "effort", "permissionMode", "openingPrompt",
		"command", "args", "status", "exitReason", "backend", "sandboxId",
		"sandboxRef", "endpoint", "attachToken", "createdAt", "updatedAt",
		"lastOutputAt", "attachCount", "contextRef",
	} {
		if _, ok := doc.Sessions[0][key]; !ok {
			t.Errorf("session object is missing %q", key)
		}
	}
}

func corruptSiblings(t *testing.T, path string) []string {
	t.Helper()
	matches, err := filepath.Glob(path + ".corrupt-*")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return matches
}

func TestUnparseableDocumentIsQuarantinedAndTheStoreStartsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	garbage := []byte(`{"schemaVersion":1,"sessions":[{"id":"s-3f2a91c0"`) // truncated mid-write
	if err := os.WriteFile(path, garbage, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	st := newStore(t, path)

	list, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List returned %d sessions from an unparseable document, want 0", len(list))
	}
	aside := corruptSiblings(t, path)
	if len(aside) != 1 {
		t.Fatalf("found %d quarantine files, want exactly 1: %v", len(aside), aside)
	}
	kept, err := os.ReadFile(aside[0])
	if err != nil {
		t.Fatalf("read quarantine file: %v", err)
	}
	if string(kept) != string(garbage) {
		t.Fatalf("quarantine file = %q, want the original bytes verbatim", kept)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat %s = %v, want the unreadable document moved, not copied", path, err)
	}
}

func TestSchemaVersionFromTheFutureIsAHardErrorAndTouchesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	future := []byte(`{"schemaVersion":2,"savedAt":"2026-09-18T09:14:07Z","sessions":[]}`)
	if err := os.WriteFile(path, future, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	st, err := NewFileStore(path)
	if err == nil {
		t.Fatalf("NewFileStore on a newer document returned a store (%v), want an error", st)
	}
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("error = %v, want it to wrap ErrSchemaTooNew", err)
	}
	// The contract requires the message to name both versions, because the only
	// useful action is "run the newer build", and the user needs to know which.
	msg := err.Error()
	if !strings.Contains(msg, "2") || !strings.Contains(msg, "1") {
		t.Fatalf("error = %q, want it to name both the document and the binary version", msg)
	}
	// Nothing is moved aside: the file is not corrupt, it is from the future,
	// and a newer build must find it exactly where it left it.
	if aside := corruptSiblings(t, path); len(aside) != 0 {
		t.Fatalf("a newer document was quarantined: %v", aside)
	}
	still, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(still) != string(future) {
		t.Fatalf("document = %q, want it untouched", still)
	}
}

func TestSchemaVersionBelowOneIsQuarantined(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	if err := os.WriteFile(path, []byte(`{"sessions":[]}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	st := newStore(t, path)

	list, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List returned %d sessions, want 0", len(list))
	}
	if aside := corruptSiblings(t, path); len(aside) != 1 {
		t.Fatalf("found %d quarantine files, want exactly 1: %v", len(aside), aside)
	}
}

func TestLoadIgnoresALeftoverTempFile(t *testing.T) {
	// A crash between writeAtomic's Write and its Rename leaves a temp file next
	// to an intact document. The next start must read the document and ignore
	// the debris: the cost of the crash is the newest mutation, never the file.
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	if err := newStore(t, path).Create(sampleSession()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".sessions-4172.json"), []byte(`{"schemaVersion":1,`), 0o600); err != nil {
		t.Fatalf("seed debris: %v", err)
	}

	list, err := newStore(t, path).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d sessions, want 1", len(list))
	}
	if aside := corruptSiblings(t, path); len(aside) != 0 {
		t.Fatalf("the intact document was quarantined because of debris: %v", aside)
	}
}

func TestWorktreeReadyFlagSurvivesAReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")

	got, err := newStore(t, path).SetWorktreeReady("w-4a77b1c9", true)
	if err != nil {
		t.Fatalf("SetWorktreeReady: %v", err)
	}
	if !got.Ready {
		t.Fatal("SetWorktreeReady returned ready=false")
	}

	flags := newStore(t, path).WorktreeFlagsSnapshot()
	if !flags["w-4a77b1c9"].Ready {
		t.Fatalf("after reopen flags = %v, want w-4a77b1c9 ready", flags)
	}
	// Clearing it persists too, or "mark ready" would be a one-way door.
	if _, err := newStore(t, path).SetWorktreeReady("w-4a77b1c9", false); err != nil {
		t.Fatalf("SetWorktreeReady(false): %v", err)
	}
	if newStore(t, path).WorktreeFlagsSnapshot()["w-4a77b1c9"].Ready {
		t.Fatal("ready=false did not persist")
	}
}

func TestUnknownWorktreeIsNotReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")

	// The zero value is the default the API reports as "ready": false for every
	// worktree git knows about and the registry has never been told about.
	if newStore(t, path).WorktreeFlagsSnapshot()["w-never-seen"].Ready {
		t.Fatal("an unknown worktree reported ready")
	}
}

func TestWorktreeFlagsSnapshotIsACopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	st := newStore(t, path)
	if _, err := st.SetWorktreeReady("w-4a77b1c9", true); err != nil {
		t.Fatalf("SetWorktreeReady: %v", err)
	}

	snap := st.WorktreeFlagsSnapshot()
	snap["w-4a77b1c9"] = WorktreeFlags{Ready: false}
	snap["w-injected"] = WorktreeFlags{Ready: true}

	again := st.WorktreeFlagsSnapshot()
	if !again["w-4a77b1c9"].Ready {
		t.Fatal("mutating the snapshot changed the store")
	}
	if _, ok := again["w-injected"]; ok {
		t.Fatal("the snapshot shares its map with the store")
	}
}

func TestSetWorktreeReadyRejectsAnEmptyID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")

	if _, err := newStore(t, path).SetWorktreeReady("", true); err == nil {
		t.Fatal("SetWorktreeReady(\"\") returned nil, want an error")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a rejected call still wrote the document: %v", err)
	}
}

func TestSessionsAndWorktreeFlagsShareOneDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	st := newStore(t, path)
	if err := st.Create(sampleSession()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := st.SetWorktreeReady("w-4a77b1c9", true); err != nil {
		t.Fatalf("SetWorktreeReady: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read document: %v", err)
	}
	var doc struct {
		Sessions      []map[string]any           `json:"sessions"`
		WorktreeFlags map[string]map[string]bool `json:"worktreeFlags"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1: writing a flag dropped the sessions", len(doc.Sessions))
	}
	if !doc.WorktreeFlags["w-4a77b1c9"]["ready"] {
		t.Fatalf("worktreeFlags = %v, want w-4a77b1c9 ready", doc.WorktreeFlags)
	}
}
```

The flag accessors are methods on `*FileStore` and a narrow `WorktreeFlagStore`
interface, **not** new methods on `Store`. `Store` is declared in the Go contract
with five session methods and task 11 depends on that shape; widening it would make
every test double in the API package implement two methods it has no opinion about.
Task 10's worktree handler takes `WorktreeFlagStore` and gets exactly the two calls
it needs. The contract is silent on this — it specifies the persisted shape and the
endpoints, not the Go seam — so this is a decision made here.

`SetWorktreeReady` does not check that the worktree exists. The registry has no
worktree inventory; git does. Task 10's handler resolves the worktree first and
returns 404 `worktree_not_found` before it ever reaches the store, so a second,
weaker existence check here would only be able to disagree with the real one.

- [ ] **Step 9: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/registry/... -race -count=1`

Expected: a build failure, not a test failure. Every symbol the two new files
reach for is still missing, so the compiler stops before a single test runs:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry [github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry.test]
control-plane\internal\registry\filestore_test.go:47:43: undefined: FileStore
control-plane\internal\registry\filestore_test.go:49:13: undefined: NewFileStore
control-plane\internal\registry\filestore_test.go:198:13: undefined: NewFileStore
control-plane\internal\registry\filestore_test.go:202:21: undefined: ErrSchemaTooNew
control-plane\internal\registry\filestore_test.go:312:23: undefined: WorktreeFlags
control-plane\internal\registry\filestore_test.go:313:23: undefined: WorktreeFlags
control-plane\internal\registry\writeatomic_test.go:14:12: undefined: writeAtomic
control-plane\internal\registry\writeatomic_test.go:46:12: undefined: writeAtomic
control-plane\internal\registry\writeatomic_test.go:85:8: undefined: writeAtomic
control-plane\internal\registry\writeatomic_test.go:132:13: undefined: writeAtomic
control-plane\internal\registry\writeatomic_test.go:132:13: too many errors
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry [build failed]
?   	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest	[no test files]
FAIL
```

Eleven lines, not five, and that is the point: `undefined: X` is emitted **once per
use site**, not once per symbol, so `NewFileStore`, `WorktreeFlags` and
`writeAtomic` each appear as many times as they are written. The tenth error trips
`gc`'s cap and the eleventh line is `too many errors` with the tenth error's own
position repeated — which means the list is truncated and there are more sites
below it. That is expected here, not a second defect. (Step 13 hits the same rule
from the other side and stops at nine, just under the cap.) Paths print with `\`
on Windows and `/` elsewhere.

`FileStore` and `NewFileStore` lead because `newStore`, the helper at the top of
`filestore_test.go`, names both, and neither is declared until Step 11 — which is
also why the calls that follow, `st.List()` and
`newStore(t, path).SetWorktreeReady(...)`, produce **no** error of their own: once
the receiver's type is invalid the compiler suppresses the cascade, and it would
print `SetWorktreeReady undefined (type *FileStore has no field or method ...)`
rather than a bare `undefined:` in any case. Bare `undefined: X` is only ever
emitted for a package-level identifier. `registrytest` reports `[no test files]`
because it holds the shared suite and no `_test.go` of its own; it compiles here,
since it needs only `Store`, which Step 6 wrote.

Line numbers will differ; the names are what matters. This is the right failure
because it is unfakeable — the tests name symbols that do not exist, so none of
them can pass by accident on an empty implementation. If instead the package
**compiles** at this point, persistence has already been written somewhere it
does not belong — most often into `registry.go`, which holds the `Store`
interface, the sentinels and `DefaultPath` and nothing else. Remove that copy
before continuing: two implementations writing the same document is the failure
this step exists to catch.

- [ ] **Step 10: Write the persistence implementation**

`Store.Update` runs the caller's mutate function while the write lock is held and
then writes the whole document. That is why `saveLocked` takes no lock of its own
and says so in its name: it is called from inside a critical section, and taking
`s.mu` again would deadlock on Go's non-reentrant mutex.

Create `control-plane/internal/registry/filestore.go`:

```go
package registry

// This file owns persistence: the on-disk document, the rules for reading a bad
// one, and the replace that must not lose a running session. The document holds
// attach tokens, so it is written 0o600. On Windows that mode is advisory —
// os.Chmod moves only the read-only attribute — and the real boundary there is
// that the file lives under the user's own %LOCALAPPDATA%.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// schemaVersion is the version this build writes and the highest it can read.
const schemaVersion = 1

// ErrSchemaTooNew reports a document written by a newer build. It is fatal on
// purpose: see load.
var ErrSchemaTooNew = errors.New("registry: document was written by a newer version of this app")

// WorktreeFlags is the control plane's own state for one worktree — the part git
// cannot tell it. Today that is one flag; it is a struct rather than a bare bool
// so the next one is an additive change inside schemaVersion 1.
type WorktreeFlags struct {
	// Ready is the user's "this worktree is ready to merge" mark. Nothing derives
	// it; only PATCH .../worktrees/{id} sets it.
	Ready bool `json:"ready"`
}

// WorktreeFlagStore is the narrow view the worktree handler depends on. Store
// stays session-only, as the contract declares it.
type WorktreeFlagStore interface {
	WorktreeFlagsSnapshot() map[string]WorktreeFlags
	SetWorktreeReady(worktreeID string, ready bool) (WorktreeFlags, error)
}

// document is the on-disk shape. One document rather than one file per session:
// sessions number in the tens, their order matters to the rail, and a single
// os.Rename is the only atomic primitive available on both Windows and POSIX.
type document struct {
	SchemaVersion int                      `json:"schemaVersion"`
	SavedAt       time.Time                `json:"savedAt"`
	Sessions      []Session                `json:"sessions"`
	WorktreeFlags map[string]WorktreeFlags `json:"worktreeFlags"`
}

// load fills the in-memory index from the document at s.path. It runs once, from
// NewFileStore, before the store is shared with any goroutine, so it takes no
// lock.
//
// Three outcomes, each chosen rather than fallen into:
//
//   - The file is absent. First run. Not an error (§5.5 step 1), and nothing is
//     created — the caller has not mutated anything yet.
//   - The file is unreadable as a schemaVersion-1 document. It is moved aside to
//     <path>.corrupt-<stamp> and the store starts empty. Refusing to start would
//     leave the user with no cockpit and no way to repair the file; starting
//     empty without the rename would destroy it on the next save. The rename is
//     what makes "start empty" safe.
//   - The file is from the future. Hard error, and the file is left exactly where
//     it is. A newer build has written fields this one would silently drop on its
//     next save, so continuing is the destructive option and stopping is the safe
//     one.
func (s *FileStore) load() error {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("registry: read %s: %w", s.path, err)
	}

	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return s.quarantine(fmt.Sprintf("unparseable: %v", err))
	}
	if doc.SchemaVersion > schemaVersion {
		return fmt.Errorf("%w: %s is schemaVersion %d, this build understands %d",
			ErrSchemaTooNew, s.path, doc.SchemaVersion, schemaVersion)
	}
	if doc.SchemaVersion < schemaVersion {
		// Version 1 is the first version, so anything lower is an absent field:
		// a truncated write, a hand edit, or a pre-release file. There is no
		// migration to run and never will be, so it takes the corruption path
		// rather than stranding the user behind a fatal error.
		return s.quarantine(fmt.Sprintf("schemaVersion %d, want %d", doc.SchemaVersion, schemaVersion))
	}

	for _, sess := range doc.Sessions {
		if sess.ID == "" {
			continue
		}
		if _, dup := s.sessions[sess.ID]; dup {
			continue
		}
		s.sessions[sess.ID] = sess
		s.order = append(s.order, sess.ID)
	}
	for id, flags := range doc.WorktreeFlags {
		if id == "" {
			continue
		}
		s.flags[id] = flags
	}
	return nil
}

// quarantine moves an unreadable document aside and reports nil, which tells load
// to continue with an empty store. It reports an error only when the move itself
// fails, because continuing then would overwrite the evidence on the first save.
func (s *FileStore) quarantine(reason string) error {
	// The contract spells this suffix RFC3339, and RFC3339 cannot be a Windows
	// filename: it contains colons, ':' opens an alternate data stream, and
	// os.Rename fails with "The filename, directory name, or volume label syntax
	// is incorrect." The stamp uses '-' for the time separators, which keeps it
	// sortable and loses nothing else.
	stamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	aside := s.path + ".corrupt-" + stamp
	for i := 1; ; i++ {
		if _, err := os.Lstat(aside); errors.Is(err, os.ErrNotExist) {
			break
		}
		if i > 9 {
			return fmt.Errorf("registry: %s is unreadable (%s) and every quarantine name for %s is taken",
				s.path, reason, stamp)
		}
		aside = fmt.Sprintf("%s.corrupt-%s.%d", s.path, stamp, i)
	}
	if err := os.Rename(s.path, aside); err != nil {
		return fmt.Errorf("registry: %s is unreadable (%s) and could not be moved aside: %w", s.path, reason, err)
	}
	return nil
}

// saveLocked writes the whole document. THE CALLER HOLDS s.mu FOR WRITING — every
// mutation ends with it, inside the same critical section that
// changed the index, so what lands on disk is exactly what a reader would see in
// memory. It does not take the lock itself; Go's mutex is not reentrant and doing
// so would deadlock Update.
func (s *FileStore) saveLocked() error {
	doc := document{
		SchemaVersion: schemaVersion,
		SavedAt:       time.Now().UTC(),
		Sessions:      make([]Session, 0, len(s.order)),
		WorktreeFlags: make(map[string]WorktreeFlags, len(s.flags)),
	}
	for _, id := range s.order {
		sess, ok := s.sessions[id]
		if !ok {
			continue
		}
		doc.Sessions = append(doc.Sessions, sess)
	}
	for id, flags := range s.flags {
		doc.WorktreeFlags[id] = flags
	}

	// Indented: tens of sessions make the size irrelevant, and the first thing
	// anyone does with a broken registry is open it in an editor.
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: encode %s: %w", s.path, err)
	}
	return writeAtomic(s.path, append(data, '\n'))
}

// WorktreeFlagsSnapshot returns a copy of every flag the registry holds. A copy,
// because the caller ranges over it while other goroutines are mutating the
// store, and handing out the live map is a data race that -race only catches on
// the unlucky run.
func (s *FileStore) WorktreeFlagsSnapshot() map[string]WorktreeFlags {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]WorktreeFlags, len(s.flags))
	for id, flags := range s.flags {
		out[id] = flags
	}
	return out
}

// SetWorktreeReady records the user's ready mark for one worktree. It does not
// verify that the worktree exists: git owns that inventory and the handler has
// already resolved it.
func (s *FileStore) SetWorktreeReady(worktreeID string, ready bool) (WorktreeFlags, error) {
	if worktreeID == "" {
		return WorktreeFlags{}, fmt.Errorf("registry: SetWorktreeReady: empty worktree id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	before, had := s.flags[worktreeID]
	next := WorktreeFlags{Ready: ready}
	if had && before == next {
		return next, nil // nothing changed; do not rewrite the document
	}

	s.flags[worktreeID] = next
	if err := s.saveLocked(); err != nil {
		// Roll the index back so memory and disk still agree. Without this a
		// full disk leaves the app reporting a flag that no restart will find.
		if had {
			s.flags[worktreeID] = before
		} else {
			delete(s.flags, worktreeID)
		}
		return WorktreeFlags{}, err
	}
	return next, nil
}

// writeAtomic replaces path with data.
//
// os.Rename on Windows is MoveFileEx with MOVEFILE_REPLACE_EXISTING, so it does
// replace an existing file atomically. What it does NOT do is succeed while
// another process has the target open — Windows Defender and search indexers open
// files briefly and opportunistically, and the failure surfaces as
// ERROR_ACCESS_DENIED ("Access is denied.") on a rename that worked a hundred
// times before, on the developer's machine only, never on Linux CI. A bounded
// retry is the whole fix. Writing in place instead would risk a truncated
// registry, which is the one failure that loses running sessions.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("registry: create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".sessions-*.json")
	if err != nil {
		return fmt.Errorf("registry: temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // a no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("registry: write %s: %w", tmpName, err)
	}
	// Sync before the rename, not after: the rename is what publishes the file,
	// and a rename that beats the data to disk is how a power cut produces a
	// zero-length registry.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("registry: sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("registry: close %s: %w", tmpName, err)
	}
	// Chmod the temp file, never the published one: the document must never be
	// world-readable, not even for the instant between rename and chmod.
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("registry: chmod %s: %w", tmpName, err)
	}

	var last error
	for i := 0; i < 5; i++ {
		if last = os.Rename(tmpName, path); last == nil {
			syncDir(dir)
			return nil
		}
		time.Sleep(time.Duration(20*(i+1)) * time.Millisecond)
	}
	return fmt.Errorf("registry: replace %s: %w", path, last)
}

// syncDir flushes the directory entry so the rename survives a power cut, and is
// best effort on purpose. Windows has no directory fsync — opening a directory
// handle and calling Sync fails with "Incorrect function." — and on POSIX a
// failure here means the data is written but the entry may not be durable yet,
// which is not worth failing a save the caller has already applied in memory.
func syncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
```

- [ ] **Step 11: Declare `FileStore` and the constructor that loads it**

Two declarations in `control-plane/internal/registry/filestore.go`, and both are
new: the index the rest of the file is written against, and the constructor that
fills it. Step 10 wrote the document, `load`, `saveLocked` and the flag accessors
against a type that did not exist yet, which is why the package has not compiled
since then. All five field names are the ones those functions already use —
`path`, `mu`, `sessions`, `order`, `flags` — so a struct that spells any of them
differently is renamed to this, not the other way round.

The index:

```go
// FileStore is the Phase 2 implementation: an in-memory index that is written
// through to one JSON document on every mutation.
type FileStore struct {
	path string

	mu       sync.RWMutex
	sessions map[string]Session
	order    []string // session ids in creation order; List preserves it

	// flags is the per-worktree state the control plane owns, keyed by worktree
	// id. It lives beside the sessions in the same document and is written by
	// the same atomic replace, so a crash cannot leave the two disagreeing.
	flags map[string]WorktreeFlags
}
```

The constructor:

```go
// NewFileStore opens the store at path. A missing file is an empty store and no
// error; nothing is written until the first mutation. The contract says "creates
// if absent", which is looser than this: Step 8's missing-file test asserts the
// file is still absent afterwards, because a cockpit that has started no sessions
// should leave nothing behind to explain.
//
// A file that exists but cannot be parsed is renamed to <path>.corrupt-<stamp>
// and an empty store is returned, because refusing to start leaves no cockpit
// and no way to fix it; the rename means nothing is lost. A file from a newer
// schemaVersion is an error — see load.
func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{
		path:     path,
		sessions: make(map[string]Session),
		flags:    make(map[string]WorktreeFlags),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}
```

Failure mode to watch: if any mutation returns before calling
`saveLocked`, its change lives only in memory and vanishes on restart, and no
test in Step 8 catches it unless that mutation has its own round-trip assertion.
`Create`, `Update` and `Delete` must each end with `saveLocked` inside the write
lock, and each must roll its in-memory change back if `saveLocked` fails, the way
`SetWorktreeReady` does. Step 14 writes those three, and
`TestASaveFailureRollsTheIndexBack` in Step 12 is what holds them to it.

- [ ] **Step 12: Write the failing tests for the five `Store` methods**

`registrytest.RunStoreContract` already holds `Create`, `Get`, `List`, `Update`
and `Delete` to every rule the contract states, and nothing calls it yet. This
step is the binding, plus the handful of properties the suite cannot reach.

What the suite already covers, so it is not written twice here: durability across
a reopen for all three mutations (`CreateGetListDelete`, `UpdateAppliesAndStamps`),
`Update`'s failing-mutate path leaving neither the index nor the file changed
(`UpdateErrorDiscardsTheChange`), `List` ordered by `CreatedAt` with two records
sharing a timestamp to exercise the id tie-break (`ListIsOrderedByCreatedAt`),
`Create` on an empty id returning `ErrInvalidID` (`CreateRejectsDuplicateAndEmptyID`),
the escaped-pointer and `Get` halves of the seal (`TheSealHolds`), and fifty
concurrent `Update`s all landing (`ConcurrentUpdatesAllLand`).

What it does not, and what the tests below add: the **`Create` and `List` halves of
the seal** — the suite writes through the pointer `mutate` was handed and through
what `Get` returned, never through the value the caller passed to `Create` nor
through a slice element `List` returned; **`Create` stamping `UpdatedAt` and
normalising both stamps to UTC**, where the suite checks only that `CreatedAt` is
non-zero and UTC; **`Update` refusing a mutate that changes the id**, which would
leave the index keyed by one id and the record carrying another; **`Delete`
removing its own row and no other**, where the suite only ever deletes the single
record it created; and **the roll-back when `saveLocked` fails**, which Step 11
names as a failure mode and no test anywhere exercises — an index that keeps a
change the document rejected reports state that no restart will find.

This file is `package registry_test` — external, so it can only reach what task 11
can reach — and it imports `registrytest` for `StorePath`, `MustOpen` and `Seed`
rather than redeclaring them. A second `Seed` here would be a second fixture that
drifts from the one the suite asserts against, which is the failure the shared
package exists to prevent.

Create `control-plane/internal/registry/filestore_contract_test.go`:

```go
package registry_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest"
)

// newFileStore adapts the constructor to registrytest.NewStoreFunc. It returns
// the interface rather than *FileStore on purpose: every assertion the suite
// makes is one task 11's in-memory double must satisfy too, and a suite written
// against the concrete type could not be reused by it.
func newFileStore(path string) (registry.Store, error) { return registry.NewFileStore(path) }

// TestFileStoreSatisfiesTheStoreContract is the call that makes the shared suite
// mean something. Without it registrytest compiles, reports nothing, and the one
// implementation of Store in the phase is untested.
func TestFileStoreSatisfiesTheStoreContract(t *testing.T) {
	registrytest.RunStoreContract(t, newFileStore)
}

// TestCreateClonesTheSessionItIsGiven closes the half of the seal the suite
// cannot see: the caller still holds the value it passed, and Args is a slice,
// so a Create that stores the struct as given shares its backing array with
// whatever the caller does next — outside the lock, with nothing serialising it.
func TestCreateClonesTheSessionItIsGiven(t *testing.T) {
	st := registrytest.MustOpen(t, newFileStore, registrytest.StorePath(t))

	in := registrytest.Seed("s-00000001", time.Time{})
	if err := st.Create(in); err != nil {
		t.Fatalf("Create: %v", err)
	}
	in.Args[0] = "--smuggled"

	got, err := st.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Args[0] != "--model" {
		t.Fatalf("the stored record shares the caller's Args array: %q, want --model", got.Args[0])
	}
}

// TestListReturnsClones is the same seal on the read side of List. The suite
// pins it for Get only, and a List that ranges the index appending the map
// values hands out live Args arrays to every poll of the rail.
func TestListReturnsClones(t *testing.T) {
	st := registrytest.MustOpen(t, newFileStore, registrytest.StorePath(t))
	if err := st.Create(registrytest.Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	list, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d sessions, want 1", len(list))
	}
	list[0].Args[0] = "--smuggled"

	got, err := st.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Args[0] != "--model" {
		t.Fatalf("List returned a view into the index: %q, want --model", got.Args[0])
	}
}

// TestCreateStampsUpdatedAtAndNormalisesTimesToUTC pins decision 2. The suite
// checks CreatedAt is stamped and UTC when the caller left it zero; this checks
// the other two cases — a caller-supplied time keeps its instant and loses its
// zone, and UpdatedAt is stamped as well.
func TestCreateStampsUpdatedAtAndNormalisesTimesToUTC(t *testing.T) {
	st := registrytest.MustOpen(t, newFileStore, registrytest.StorePath(t))

	// A fixed offset rather than a zoneinfo lookup: Windows ships no tzdata, so
	// time.LoadLocation("Australia/Sydney") fails there and nowhere else.
	sydney := time.FixedZone("AEST", 10*60*60)
	created := time.Date(2026, 9, 18, 19, 14, 3, 0, sydney)
	if err := st.Create(registrytest.Seed("s-00000001", created)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := st.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.CreatedAt.Equal(created) {
		t.Fatalf("CreatedAt = %s, want the instant the caller supplied, %s", got.CreatedAt, created)
	}
	if got.CreatedAt.Location() != time.UTC {
		t.Fatalf("CreatedAt location = %v, want UTC — two machines' documents should diff", got.CreatedAt.Location())
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("Create left UpdatedAt zero; a record nobody has updated reads as updated when it was created")
	}
	if got.UpdatedAt.Location() != time.UTC {
		t.Fatalf("UpdatedAt location = %v, want UTC", got.UpdatedAt.Location())
	}
}

// TestUpdateStampsInUTCAndRefusesAnIDChange. The id is the map key: a mutate
// that changes it would leave the index keyed by one id and holding a record
// carrying another, and the next reload would rename the session silently.
// Refusing is a symptom; restoring the id quietly is not.
func TestUpdateStampsInUTCAndRefusesAnIDChange(t *testing.T) {
	st := registrytest.MustOpen(t, newFileStore, registrytest.StorePath(t))
	if err := st.Create(registrytest.Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := st.Update("s-00000001", func(sess *registry.Session) error {
		sess.Status = registry.StatusIdle
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.UpdatedAt.Location() != time.UTC {
		t.Fatalf("UpdatedAt location = %v, want UTC", got.UpdatedAt.Location())
	}

	if _, err := st.Update("s-00000001", func(sess *registry.Session) error {
		sess.ID = "s-00000002"
		return nil
	}); err == nil {
		t.Fatal("Update accepted a mutate that changed the session id")
	}
	if _, err := st.Get("s-00000002"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("Get s-00000002: err = %v, want ErrNotFound — the rename was committed", err)
	}
	if _, err := st.Get("s-00000001"); err != nil {
		t.Fatalf("Get s-00000001 after the refused rename: %v", err)
	}
}

// TestDeleteRemovesOnlyItsOwnRecord. The suite deletes the only session it
// created, so a Delete that clears the whole index, or one that drops the row
// from the map and leaves its id in the order slice, passes it. This reloads,
// because the survivors have to be in the document and not just in memory.
func TestDeleteRemovesOnlyItsOwnRecord(t *testing.T) {
	path := registrytest.StorePath(t)
	st := registrytest.MustOpen(t, newFileStore, path)

	base := time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)
	for i, id := range []string{"s-0000000a", "s-0000000b", "s-0000000c"} {
		if err := st.Create(registrytest.Seed(id, base.Add(time.Duration(i)*time.Hour))); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}
	if err := st.Delete("s-0000000b"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	list, err := registrytest.MustOpen(t, newFileStore, path).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := make([]string, 0, len(list))
	for _, sess := range list {
		got = append(got, sess.ID)
	}
	want := []string{"s-0000000a", "s-0000000c"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("after Delete the document holds %v, want %v", got, want)
	}
}

// TestASaveFailureRollsTheIndexBack is the failure mode Step 11 names. A
// mutation that applies in memory and fails on disk leaves the cockpit showing
// state no restart will find, which is the one class of bug this whole file
// exists to prevent.
func TestASaveFailureRollsTheIndexBack(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	path := filepath.Join(dir, "sessions.json")
	st := registrytest.MustOpen(t, newFileStore, path)
	if err := st.Create(registrytest.Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Replace the state directory with a regular file. os.MkdirAll then fails
	// with ENOTDIR before writeAtomic reaches its temp file, on Windows exactly
	// as on Linux, which is the portable way to make every save fail — a chmod
	// on the directory would be ignored on Windows.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove %s: %v", dir, err)
	}
	if err := os.WriteFile(dir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write %s: %v", dir, err)
	}

	if err := st.Create(registrytest.Seed("s-00000002", time.Time{})); err == nil {
		t.Fatal("Create reported success while the document could not be written")
	}
	if _, err := st.Get("s-00000002"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("Get after the failed Create: err = %v, want ErrNotFound", err)
	}

	if _, err := st.Update("s-00000001", func(sess *registry.Session) error {
		sess.Status = registry.StatusExited
		return nil
	}); err == nil {
		t.Fatal("Update reported success while the document could not be written")
	}
	got, err := st.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != registry.StatusStarting {
		t.Fatalf("status = %q after the failed Update, want starting — the index was not rolled back", got.Status)
	}

	if err := st.Delete("s-00000001"); err == nil {
		t.Fatal("Delete reported success while the document could not be written")
	}
	if _, err := st.Get("s-00000001"); err != nil {
		t.Fatalf("Get after the failed Delete: %v — the row left memory and never left the file", err)
	}
}
```

- [ ] **Step 13: Run the tests to verify they fail**

Run: `go test ./control-plane/internal/registry/... -race -count=1`

Expected: a build failure naming the five methods that do not exist. `FileStore`
has a document, a lock and a constructor, and no way to put a session into any of
them:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry [github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry.test]
control-plane\internal\registry\filestore_test.go:61:18: st.List undefined (type *FileStore has no field or method List)
control-plane\internal\registry\filestore_test.go:80:18: first.Create undefined (type *FileStore has no field or method Create)
control-plane\internal\registry\filestore_test.go:84:32: newStore(t, path).Get undefined (type *FileStore has no field or method Get)
control-plane\internal\registry\filestore_test.go:108:30: newStore(t, path).Create undefined (type *FileStore has no field or method Create)
control-plane\internal\registry\filestore_test.go:168:18: st.List undefined (type *FileStore has no field or method List)
control-plane\internal\registry\filestore_test.go:233:18: st.List undefined (type *FileStore has no field or method List)
control-plane\internal\registry\filestore_test.go:251:30: newStore(t, path).Create undefined (type *FileStore has no field or method Create)
control-plane\internal\registry\filestore_test.go:258:33: newStore(t, path).List undefined (type *FileStore has no field or method List)
control-plane\internal\registry\filestore_test.go:338:15: st.Create undefined (type *FileStore has no field or method Create)
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry [build failed]
?   	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest	[no test files]
FAIL
```

Nine errors, one per call site, because the compiler reports the missing method
everywhere it is reached rather than once per method — `filestore_test.go` calls
`Create`, `Get` and `List` nine times between them. Nine is under `gc`'s ten-error
cap, so nothing is truncated here; add one more call site and the last line becomes
`too many errors` instead. Line numbers will differ; the method names are what
matters. Paths print with `\` on Windows and `/` elsewhere.

Two things about this output are worth knowing before you read it as incomplete.
The external test package is type-checked only after the internal one builds, so
`filestore_contract_test.go` is not reached yet. Declare `Create`, `Get` and
`List` — the three these nine sites name — and the next run reports the one that
is the real point of the task:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry_test [github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry.test]
control-plane\internal\registry\filestore_contract_test.go:19:65: cannot use registry.NewFileStore(path) (value of type *"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry".FileStore) as "github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry".Store value in return statement: *"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry".FileStore does not implement "github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry".Store (missing method Delete)
```

Two things about that message rather than its content. It spells the package as a
quoted import path instead of `registry.` because the error is raised inside
`registry_test`, where the compiler disambiguates rather than assume the short
name; it is one line however it wraps in a terminal. And it names only the
**alphabetically first** missing method, not the first one the interface
declares — `Delete` here, because `Create` has just been written and `Delete`
sorts ahead of `Update` — so `Update` never appears even though it is missing
too. Write all five, not the one the compiler happens to be complaining about.

If instead the package compiles and the tests merely fail, the methods were
written before their tests — start again from the test file, because a test
written against working code pins whatever that code happens to do rather than
what the contract says.

- [ ] **Step 14: Write the five `Store` methods on `FileStore`**

They go in `filestore.go`, beside the struct and the constructor. The File
structure table reads `registry.go` as "`Store`, the in-memory index, `Update` as
the only mutation primitive", which could be taken as putting them there; it means
the *declarations*, and `registry.go` holds those — the interface, the three
sentinels, `DefaultPath`. `FileStore` is one type, and each of these five touches
both the index and the document in one critical section, so splitting them across
two files by "index" versus "persistence" would cut the type along a seam that does
not exist.

Two rules run through all five. Every mutation ends with `saveLocked()` **inside
the lock it already holds** — `saveLocked` is named for that and takes no lock of
its own, and a second acquisition deadlocks on Go's non-reentrant mutex. And every
mutation rolls its in-memory change back when `saveLocked` fails, the way
`SetWorktreeReady` does, because an index that keeps a change the document rejected
reports state that no restart will find.

First, one import the file does not yet have: `List` sorts, and an unused `sort`
would have failed the Step 13 build, which is the first one `filestore.go` is
compiled in at all, so it arrives with the method that needs it.
Replace the import block in `control-plane/internal/registry/filestore.go`:

```go
import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)
```

Then the methods. Add to `control-plane/internal/registry/filestore.go`:

```go
// The compile-time proof that this type is the Store the rest of the phase
// depends on. It costs nothing at run time and turns a missing or wrongly
// signed method into a build failure here rather than an error in task 11.
var _ Store = (*FileStore)(nil)

// Create adds one session. It is the only way a row enters the store, and it
// takes no mutate function because a row nobody can see yet has nothing to race
// with.
//
// An empty id is ErrInvalidID rather than ErrExists: the map would accept "" as
// a key perfectly well, and task 11 must answer 400 to a caller defect rather
// than 409 to a collision that did not happen.
func (s *FileStore) Create(sess Session) error {
	if sess.ID == "" {
		return ErrInvalidID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, dup := s.sessions[sess.ID]; dup {
		return ErrExists
	}

	// Cloned on the way in: the caller keeps the value it passed, and Args is a
	// slice, so the struct copy alone would leave the caller holding a live
	// pointer into the store's record.
	rec := sess.Clone()

	// The store stamps time, in UTC, because asking every caller to remember is
	// the class of defect Update-only exists to prevent. A caller-supplied
	// CreatedAt is kept — task 13's reaper and the rail both order by it — and
	// only normalised.
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	} else {
		rec.CreatedAt = rec.CreatedAt.UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	} else {
		rec.UpdatedAt = rec.UpdatedAt.UTC()
	}

	// order is what saveLocked writes from, so a Create that updates the map and
	// not the slice persists nothing at all while looking entirely correct in
	// memory until the next restart.
	s.sessions[rec.ID] = rec
	s.order = append(s.order, rec.ID)

	if err := s.saveLocked(); err != nil {
		delete(s.sessions, rec.ID)
		s.order = s.order[:len(s.order)-1]
		return err
	}
	return nil
}

// Get returns one session by id, or ErrNotFound.
//
// The returned value shares nothing with the index: a caller that appends to the
// Args it was handed would otherwise be writing into the store from outside the
// lock, which -race catches only on the unlucky run.
func (s *FileStore) Get(id string) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	return sess.Clone(), nil
}

// List returns every session, sorted by CreatedAt ascending with ID as the
// tie-break.
//
// The sort is not a nicety. Go randomises map iteration deliberately, so a List
// that ranged the index would reshuffle the session rail on every two-second
// poll of the cockpit. Sorting here rather than maintaining a sorted structure
// means there is nothing that can fall out of sync with Delete or with a
// hand-edited document.
func (s *FileStore) List() ([]Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, sess.Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		// Two sessions created inside the same clock tick — which on Windows is
		// about 15 ms wide — would otherwise swap places between polls.
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Update is the only way a session changes, and the reason the Store interface
// has no Put.
//
// mutate runs on a clone of the stored record, under the write lock, and the
// lock is not released between mutate returning and the document being written:
// the whole read-modify-write is one critical section, so the HTTP handler, the
// attach relay writing LastOutputAt and the reaper cannot lose each other's
// writes. mutate must not run against the stored record itself — a mutate that
// fails after changing two fields would leave those two committed.
//
// A failing mutate is returned unwrapped, so errors.Is against the caller's own
// sentinel works, and nothing at all is committed or written.
func (s *FileStore) Update(id string, mutate func(*Session) error) (Session, error) {
	if mutate == nil {
		return Session{}, fmt.Errorf("registry: Update %s: mutate is nil", id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	before, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}

	working := before.Clone()
	if err := mutate(&working); err != nil {
		return Session{}, err
	}
	if working.ID != before.ID {
		// The id is the map key. Committing this would leave the index keyed by
		// one id and holding a record carrying another, and the next reload
		// would rename the session with nothing to show why.
		return Session{}, fmt.Errorf("registry: Update %s: mutate changed the session id to %q", id, working.ID)
	}

	// The store owns both stamps: CreatedAt is a fact about the past, and
	// UpdatedAt is what the cockpit sorts and ages by, so neither is left to the
	// mutate function to remember.
	working.CreatedAt = before.CreatedAt
	working.UpdatedAt = time.Now().UTC()

	// Clone on commit, and clone again on return: the pointer mutate was handed
	// still exists in the caller's closure, and if either the index or the
	// returned value shared its Args array, a write through it after Update has
	// returned would reach the index without ever reaching the document.
	committed := working.Clone()
	s.sessions[id] = committed

	if err := s.saveLocked(); err != nil {
		s.sessions[id] = before
		return Session{}, err
	}
	return committed.Clone(), nil
}

// Delete removes one session, or reports ErrNotFound.
//
// An unknown id is an error rather than a silent no-op because task 13's reaper
// deletes sessions it believes are gone: without the error, "the reaper is
// deleting the same record every ten seconds" is a condition with no symptom.
func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	before, ok := s.sessions[id]
	if !ok {
		return ErrNotFound
	}

	// A fresh slice rather than append(s.order[:i], s.order[i+1:]...): that
	// shifts in place, which would corrupt the copy this function needs to
	// restore if the save fails. Sessions number in the tens.
	next := make([]string, 0, len(s.order))
	for _, existing := range s.order {
		if existing != id {
			next = append(next, existing)
		}
	}
	prevOrder := s.order
	s.order = next
	delete(s.sessions, id)

	if err := s.saveLocked(); err != nil {
		s.sessions[id] = before
		s.order = prevOrder
		return err
	}
	return nil
}
```

- [ ] **Step 15: Run the new tests to verify they pass**

Run, on one line:

```
go test ./control-plane/internal/registry/... -race -count=1 -v -run 'TestFileStoreSatisfiesTheStoreContract|TestCreateClones|TestListReturnsClones|TestCreateStamps|TestUpdateStamps|TestDeleteRemovesOnlyItsOwnRecord|TestASaveFailureRollsTheIndexBack'
```

Expected — the shared suite reporting every subtest, then the six direct tests:

```
=== RUN   TestFileStoreSatisfiesTheStoreContract
=== RUN   TestFileStoreSatisfiesTheStoreContract/CreateGetListDelete
=== RUN   TestFileStoreSatisfiesTheStoreContract/CreateRejectsDuplicateAndEmptyID
=== RUN   TestFileStoreSatisfiesTheStoreContract/UpdateAppliesAndStamps
=== RUN   TestFileStoreSatisfiesTheStoreContract/UpdateErrorDiscardsTheChange
=== RUN   TestFileStoreSatisfiesTheStoreContract/TheSealHolds
=== RUN   TestFileStoreSatisfiesTheStoreContract/ListIsOrderedByCreatedAt
=== RUN   TestFileStoreSatisfiesTheStoreContract/ConcurrentUpdatesAllLand
--- PASS: TestFileStoreSatisfiesTheStoreContract (1.93s)
    --- PASS: TestFileStoreSatisfiesTheStoreContract/CreateGetListDelete (0.07s)
    --- PASS: TestFileStoreSatisfiesTheStoreContract/CreateRejectsDuplicateAndEmptyID (0.04s)
    --- PASS: TestFileStoreSatisfiesTheStoreContract/UpdateAppliesAndStamps (0.05s)
    --- PASS: TestFileStoreSatisfiesTheStoreContract/UpdateErrorDiscardsTheChange (0.05s)
    --- PASS: TestFileStoreSatisfiesTheStoreContract/TheSealHolds (0.05s)
    --- PASS: TestFileStoreSatisfiesTheStoreContract/ListIsOrderedByCreatedAt (0.09s)
    --- PASS: TestFileStoreSatisfiesTheStoreContract/ConcurrentUpdatesAllLand (1.59s)
=== RUN   TestCreateClonesTheSessionItIsGiven
--- PASS: TestCreateClonesTheSessionItIsGiven (0.10s)
=== RUN   TestListReturnsClones
--- PASS: TestListReturnsClones (0.09s)
=== RUN   TestCreateStampsUpdatedAtAndNormalisesTimesToUTC
--- PASS: TestCreateStampsUpdatedAtAndNormalisesTimesToUTC (0.07s)
=== RUN   TestUpdateStampsInUTCAndRefusesAnIDChange
--- PASS: TestUpdateStampsInUTCAndRefusesAnIDChange (0.04s)
=== RUN   TestDeleteRemovesOnlyItsOwnRecord
--- PASS: TestDeleteRemovesOnlyItsOwnRecord (0.14s)
=== RUN   TestASaveFailureRollsTheIndexBack
--- PASS: TestASaveFailureRollsTheIndexBack (0.06s)
PASS
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry	5.763s
?   	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest	[no test files]
```

Under `-v` every test prints its own `=== RUN` before its result, which is why the
six at the bottom are twelve lines rather than six; the contract test's seven are
grouped differently only because they are sequential subtests, so the parent's
`--- PASS` lands after all seven `=== RUN` lines with the children indented under
it. Timings vary widely and are not worth matching — the numbers above are one
Windows run, and `ConcurrentUpdatesAllLand` alone swung between 1.6 s and 2.8 s
across runs on the same machine, because fifty serialised writes under `-race`
are fifty `Sync` calls. The seven subtest names and the six test names do not
vary. Two failures to recognise here rather than debug from scratch.
`TheSealHolds` failing on "a write through the escaped pointer reached the index"
means `Update` committed `working` instead of `working.Clone()`, so the pointer
the caller's closure still holds writes straight into the index. And
`ConcurrentUpdatesAllLand` reporting an
`AttachCount` a little short of 50, intermittently, means `Update` read the record
under `RLock`, released it, and took the write lock to commit — a Get-then-Put in
disguise, and the exact write loss the single critical section exists to prevent.
(The opposite mistake, calling a method that takes `s.mu` again instead of calling
`saveLocked` directly, does not go intermittent: Go's mutex is not reentrant, so it
hangs every time and `go test` kills it after ten minutes with a stack dump.)

- [ ] **Step 16: Run the whole registry package**

Step 15 filtered down to the new tests. This is the unfiltered run, and it is the
first point at which every test in the task is green together — the document,
quarantine and worktree-flag tests from Step 8 call `Create`, `Get` and `List`, so
none of them could pass before Step 14 existed.

Run: `go test ./control-plane/internal/registry/... -race -count=1`

Expected:

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry	6.283s
?   	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest	[no test files]
```

Several seconds, and the exact figure is not worth matching: 4.4s to 6.6s across
runs on one Windows 11 laptop, and 5.8s to 6.4s in a `golang:1.23-bookworm`
container on the same machine. Most of that is the fixed per-package overhead
Step 4 describes. What the number does still rule out is a retry loop that has
been flattened: two tests deliberately drive it to exhaustion and it sleeps
20+40+60+80+100 ms each time —
`TestWriteAtomicFailureLeavesThePreviousContentIntact` on every platform, and
`TestWriteAtomicWhileAReaderHoldsTheTargetOpen` on Windows as well — so those
alone put 0.3s (0.6s on Windows) into the total. If the package reports under
0.3s, the loop is not looping.

- [ ] **Step 17: Run the full Go suite**

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected: every package `ok` or `no test files`, and no `FAIL` line. That — not
the list below — is the acceptance criterion, because by task 7 this command
covers every package tasks 1 to 6 added as well: `sandboxd/cmd/sandboxd`,
`sandboxd/internal/…`, `control-plane/cmd/api` and `control-plane/internal/…`,
printed in import-path order. These three lines are the extract this task is
responsible for:

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry	6.283s
?   	github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest	[no test files]
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath	1.948s
```

`go test ./...` at the workspace root is invalid and will fail with
`directory prefix . does not contain modules listed in go.work or their selected dependencies`.
Use the two explicit module paths.

- [ ] **Step 18: Commit**

```bash
git add control-plane/internal/statepath/statepath.go \
        control-plane/internal/statepath/statepath_test.go \
        control-plane/internal/registry/session.go \
        control-plane/internal/registry/session_test.go \
        control-plane/internal/registry/registry.go \
        control-plane/internal/registry/registrytest/registrytest.go \
        control-plane/internal/registry/filestore.go \
        control-plane/internal/registry/filestore_test.go \
        control-plane/internal/registry/filestore_contract_test.go \
        control-plane/internal/registry/writeatomic_test.go
git commit -m "feat(registry): persist sessions and worktree flags across a restart

The control plane must survive its own restart without losing a running
sandbox, so the endpoint, the sandbox reference and the attach token are on
disk rather than in memory only. One document, replaced atomically, because a
truncated registry is the single failure that loses work; the rename retries
because Windows refuses it while an indexer holds the file open, and that
failure looks like nothing else.

The worktree ready flag ships in the same document now rather than later: as
an extra map it is additive inside schemaVersion 1, while adding it after the
registry ships would be a schema bump and a migration for one boolean.

An unreadable document is moved aside and the app starts empty; a document
from a newer build stops the app instead, because starting would drop the
fields that build wrote.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```
