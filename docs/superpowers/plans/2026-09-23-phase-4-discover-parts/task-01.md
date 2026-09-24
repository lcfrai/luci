### Task 01: Lift `writeAtomic` into `statepath` (ruling R2)

**Why.** Ruling R2: one implementation of the atomic replace, with its Windows retry and its tests,
shared by the registry and by phase 4's `contextwrite` — never a second copy that lacks the
hard-won test (the `FILE_SHARE_DELETE` finding of phase 2 task 7). Under `OD-3` phase 4 has no
revision store on disk, so its only caller is the `CLAUDE.md` write (task 09). **Phase 3's task 03
step 1 does the same lift "if phase 4 has not done it first"** (phase 3 plan `:138`): whichever
lands first does the move, and the other drops the move. Phase 3's move has no `Mode` and no
parameterised temp pattern, so this task still adds those if phase 3 lands first. Check before
starting (step 1).

**One real change, not only a move.** The registry's version chmods the temp file `0o600` and
creates directories `0o700` (`registry/filestore.go:462` for the `MkdirAll`, `:488` for the
chmod). That is right for the session registry and **wrong for `CLAUDE.md`**: the file is read by the agent inside the sandbox, through a
bind mount, as the container's user, which is not guaranteed to be the host user. So the lifted
function takes a `Mode`. The registry passes `Private`, keeping its behaviour byte for byte.

**Files:**
- `control-plane/internal/statepath/atomic.go` — `WriteAtomic`, `Mode`, `syncDir` (new)
- `control-plane/internal/statepath/atomic_test.go` — the five tests moved from
  `registry/writeatomic_test.go`, their helper `countTempFiles`, plus two new ones (new)
- `control-plane/internal/registry/filestore.go` — `writeAtomic` becomes a one-line call; `syncDir`
  is deleted from this file (modify, **declared departure into a closed phase 2 file**, R2)
- `control-plane/internal/registry/writeatomic_test.go` — deleted (its tests moved)

**Interfaces — complete source:**

```go
// control-plane/internal/statepath/atomic.go
package statepath

import "os"

// Mode is who may read what WriteAtomic writes.
type Mode int

const (
	// Private: file 0o600, created directories 0o700. The session registry.
	Private Mode = iota
	// Shared: file 0o644, created directories 0o755. A file another user on
	// the machine must read - CLAUDE.md, which the agent reads through a bind
	// mount as the sandbox's user, not as ours.
	Shared
)

// WriteAtomic replaces path with data so that a reader sees either the old
// bytes or the new bytes, never a mixture and never an empty file.
//
// It creates missing parent directories with the mode's directory permission,
// writes a temp file in the same directory named "." + stem + "-*" + ext
// (sessions.json -> .sessions-*.json, CLAUDE.md -> .CLAUDE-*.md), syncs it,
// sets the mode's file permission on the TEMP file (never on the published
// one), and renames it over path. The rename is retried five times with
// 20, 40, 60 and 80 ms between attempts and no sleep after the last, because
// on Windows a scanner holding the target open fails MoveFileEx with
// ERROR_ACCESS_DENIED. Errors are wrapped with "statepath: <op> <path>".
// The temp file is removed on every failure path.
func WriteAtomic(path string, data []byte, mode Mode) error

// FilePerm and DirPerm are the permissions a mode writes.
func (m Mode) FilePerm() os.FileMode
func (m Mode) DirPerm() os.FileMode
```

```go
// control-plane/internal/registry/filestore.go — the whole of what remains of writeAtomic
func writeAtomic(path string, data []byte) error {
	return statepath.WriteAtomic(path, data, statepath.Private)
}
```

**Tests first** (`statepath/atomic_test.go`, package `statepath`):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWriteAtomicCreatesTheFileAndItsDirectory` (moved) | a nested path is created with the data | a `WriteAtomic` that drops `MkdirAll` |
| `TestWriteAtomicReplacesAFileThatAlreadyExists` (moved) | the new bytes replace the old | a write that appends |
| `TestWriteAtomicWhileAReaderHoldsTheTargetOpen` (moved, **unchanged including its Windows inversion**) | on Windows the replace fails while a reader holds the file, and the old document is intact; elsewhere it succeeds | a fallback that writes in place |
| `TestWriteAtomicLeavesNoTempFileBehind` (moved) | no `.sessions-*.json` left after three writes | a missing `os.Remove` on success |
| `TestWriteAtomicFailureLeavesThePreviousContentIntact` (moved; its string check becomes `"statepath: replace"`) | an unreplaceable target is an error naming the op and path, and nothing else changed | a retry loop that returns nil on exhaustion |
| `TestSharedModeWritesAWorldReadableFile` (new; skipped on Windows, where POSIX bits are not honoured) | `Shared` publishes `0o644` and creates directories `0o755` | a lift that kept the hard-coded `0o600` — the defect this task exists for |
| `TestTempNameFollowsTheTarget` (new) | while the rename is blocked (reuse the directory-target trick), the temp file matches `.CLAUDE-*.md` for `CLAUDE.md` and `.sessions-*.json` for `sessions.json` | a fixed `.sessions-*` pattern leaking into worktrees |

Added when task 01 was built, to kill the survivors its mutation run found (all in
`statepath/atomic_test.go` unless named):

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestModePermissions` | `Private` is 0o600/0o700, `Shared` 0o644/0o755, and any other value fails closed to 0o600/0o700, on every OS | a swapped constant, which every POSIX-mode test above misses on Windows because it skips; an unknown mode treated as `Shared` |
| `TestWriteAtomicRidesOutATransientHolder` | through an unexported `rename`/`sleep` seam: held four attempts, the fifth publishes; backoff 20/40/60/80 ms; on POSIX the temp already has the mode's permission at every rename | `attempts = 1`; a chmod moved onto the published file |
| `TestWriteAtomicGivesUpAfterFiveAttemptsWithoutAFinalSleep` | five renames, four sleeps, the last error wrapped (`errors.Is`), `statepath: replace <path>`, no temp left | a sleep after the last attempt; `nil` on exhaustion |
| `TestTheDocumentIsWrittenPrivate` (`registry/filemode_test.go`, new; skipped on Windows) | a save through `NewFileStore` publishes 0o600 in 0o700 directories | the registry passing `Shared`: once the moved tests left package `registry`, nothing pinned which mode the registry asks for |

`registry`'s own suite (`filestore_test.go`, `filestore_contract_test.go`) must pass **unchanged**:
it is the proof the registry's behaviour did not move. The new `filemode_test.go` sits beside it
and changes neither file.

**Steps:**

- [ ] **Step 1: Check the lift has not already landed.**
  `grep -n "func WriteAtomic" control-plane/internal/statepath/*.go` — expect no match. If phase 3
  has landed it (its task 03 moves `writeAtomic` as it is: no `Mode`, and a fixed
  `.sessions-*.json` temp pattern), the file names already match, but the behaviour this task
  exists for does not. Do **not** skip to step 5; take this path instead, as its own commit:
  1. Add `TestSharedModeWritesAWorldReadableFile` and `TestTempNameFollowsTheTarget` to
     `statepath/atomic_test.go`, and change the moved tests' calls to `WriteAtomic(p, d, Private)`.
     Run `go test ./control-plane/internal/statepath/...` and expect a compile failure: too many
     arguments to `WriteAtomic`.
  2. Add `Mode`, `FilePerm` and `DirPerm` to `atomic.go`, and parameterise the two permissions and
     the temp pattern as step 3 says.
  3. Give every caller `statepath.Private`. Find them with `grep -rn "statepath.WriteAtomic("
     control-plane/`. The registry writes its file from one place, the `writeAtomic` call at
     `filestore.go:401` at `3303fd6`, so after phase 3's lift it has one caller: either that call
     or the one-line wrapper it goes through. Phase 3's own callers are found by the same grep:
     `lanes/store.go` (phase 3 `task-03.md:24`, `:104-106`) and, if its task 15 has landed,
     `evidence.FileStore` (`task-15.md:76-77`). `Private` keeps each caller's behaviour byte for
     byte.
  4. Go to step 5, and on this path run `go test ./control-plane/... -race -count=1` there, not
     only `statepath` and `registry`: phase 3's callers live in other packages. The step 6 commit
     carries `atomic.go`, `atomic_test.go`, `filestore.go` **and every other file the sub-step 3
     grep found**, and no deletion. A commit that leaves one two-argument caller behind does not
     compile (re-review b08-2 R2-M3).
  If the lift has landed **with** a mode, check that both new tests exist and pass, add whichever
  is missing, then go to step 5.
- [ ] **Step 2: Move the tests.** `git mv control-plane/internal/registry/writeatomic_test.go
  control-plane/internal/statepath/atomic_test.go`, change `package registry` to `package
  statepath`, `writeAtomic(p, d)` to `WriteAtomic(p, d, Private)`, the error string to
  `"statepath: replace"`. Add the two new tests. Run `go test ./control-plane/internal/statepath/...`
  — expect a compile failure (`WriteAtomic` undefined).
- [ ] **Step 3: Write `atomic.go`** by moving the body of `registry.writeAtomic` and `syncDir`,
  parameterising the two permissions and the temp pattern. No other behavioural change.
- [ ] **Step 4: Replace `registry.writeAtomic`'s body** with the one-line call above; delete
  `syncDir` and any import the file no longer needs. At `ad5a8f6` that is `runtime` and
  `path/filepath`; `time` stays, because `quarantine` and `SavedAt` use it (fact checked when
  task 01 was built).
- [ ] **Step 5: Run** `go test ./control-plane/internal/statepath/... ./control-plane/internal/registry/... -race -count=1`
  and `go vet ./control-plane/...`.
- [ ] **Step 6: Commit** `statepath/atomic.go`, `statepath/atomic_test.go`,
  `registry/filestore.go` and the deletion, by pathspec. The message names R2 and the declared
  departure into `registry/filestore.go`.

**Acceptance:**

```
$ go test ./control-plane/internal/statepath/... ./control-plane/internal/registry/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry
$ grep -rn "os.Rename(tmpName\|os.CreateTemp\|func syncDir" control-plane/internal/registry/
(no output — the registry no longer replaces its document itself)
$ grep -c "func WriteAtomic" control-plane/internal/statepath/atomic.go
1
```

The acceptance used to be `grep -rn "os.Rename" control-plane/internal/registry/` with no output.
That was wrong on the facts: `quarantine` (`filestore.go:173` at `ad5a8f6`) still renames an
unreadable document aside with `os.Rename(s.path, aside)`, which is not an atomic replace and is
not part of the lift, and two comments (`:46`, `:158`) name `os.Rename`.

**Does NOT claim:** that the Windows reader test was run on Windows by this task (it runs where CI
runs; phase 2 task 7 established its inversion); any change to the registry's on-disk format.
