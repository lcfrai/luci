### Task 28: `credstore` — the machine's secret store: Windows Credential Manager, memory, chunking, one target grammar (seam S-CRED)

**Lane C. Depends on: 06 merged (go.mod). About 3 hours. Contract task C1.**

`OD-6`: tokens are kept in **Windows Credential Manager** (DPAPI-protected, per user), and **the
same store** holds the Azure DevOps and GitHub tokens phase 3 needs (`OD-4`) and the Panacea token
phase 7 needs (`OD-13`). This task is that store's lowest layer — **seam S-CRED**: one `Secrets`
interface, two implementations, one chunking rule for tokens larger than a credential blob, and
**one** place every target name is spelled. Task 29 builds the typed accessors on it (GitHub,
Panacea); task 31 stores the Entra refresh token through it; task 39 is the only write path for a
pasted token.

**Two rules no implementation may break:** a secret is never logged, never in an error, never
returned over HTTP; and every secret is the signed-in user's own, on that user's machine (never
pooled; the Hub stores none, ADR-H1 §5.2).

**Files:**
- `control-plane/internal/credstore/secrets.go` — `Secrets`, `MaxBlob`, `ErrNotFound`, `ErrUnsupported` (new)
- `control-plane/internal/credstore/memory.go` (new)
- `control-plane/internal/credstore/wincred_windows.go` — over `advapi32.dll` (new)
- `control-plane/internal/credstore/wincred_other.go` — `//go:build !windows` (new)
- `control-plane/internal/credstore/chunk.go`, `targets.go` (new)
- `control-plane/internal/credstore/credstoretest/credstoretest.go` — the behaviour suite (new)
- `control-plane/internal/credstore/{memory_test.go,wincred_windows_test.go,chunk_test.go,targets_test.go}` (new)
- `control-plane/go.mod` — `golang.org/x/sys` becomes direct **at whatever version the graph already holds**, by `go mod tidy`; no version is named (after task 06 that is v0.32.0 or higher, because pgx v5.7.5 → x/crypto v0.37.0 → x/sys v0.32.0) (modify ⚑)
- `azure-pipelines.yml` — one step: `GOOS=windows go vet ./control-plane/...` (modify ⚑; see Step 5)

#### Contract — complete

```go
// control-plane/internal/credstore/secrets.go

// Package credstore is this machine's store of the signed-in user's own secrets
// (OD-6): the Entra refresh token, a GitHub token, a Panacea token. Windows
// Credential Manager in production, memory in tests and off Windows. There is no
// file-backed store: a file is the thing Credential Manager exists to avoid
// (contract §9.1). Nothing in this package logs; no error carries a blob.
//
// Phase 3's credstore.go (Host, Credential, Store, None, Chain — phase 3 task 13)
// lives in this same package; task 29 of the Hub plan writes it verbatim if absent.
package credstore

import "errors"

// Secrets is this machine's secret store.
type Secrets interface {
	Get(target string) ([]byte, error)              // ErrNotFound when absent
	Put(target, userName string, blob []byte) error // len(blob) <= MaxBlob
	Delete(target string) error                     // absent is not an error
	List(prefix string) ([]string, error)           // target names only, sorted
}

// MaxBlob is CRED_MAX_CREDENTIAL_BLOB_SIZE (5 × 512 bytes), from the Win32
// documentation as recalled — unverified (ADR-H1 R1); Memory enforces the same.
const MaxBlob = 2560

var (
	ErrNotFound    = errors.New("credstore: not found")
	ErrTooLarge    = errors.New("credstore: a blob is at most 2,560 bytes; chunk it")
	ErrUnsupported = errors.New("credstore: Windows Credential Manager is not available on this OS")
)

func WinCred() Secrets // wincred_windows.go; off Windows, every method returns ErrUnsupported
func Memory() Secrets  // tests, and every non-Windows build: secrets last for the process only
```

```go
// control-plane/internal/credstore/targets.go — every target name, in one place.

// TargetPrefix begins every name this product writes, so List(TargetPrefix)
// is exactly our entries and nobody else's.
const TargetPrefix = "PanaceaApp/"

// EntraAccountTarget: the account record, written LAST so it marks a complete
// write (contract §9.1):  PanaceaApp/entra/<tenantId>/<clientId>/account
func EntraAccountTarget(tenantID, clientID string) string

// EntraRefreshTarget: chunk i of the refresh token:  PanaceaApp/entra/<tenantId>/<clientId>/refresh/<i>
func EntraRefreshTarget(tenantID, clientID string, i int) string

// EntraRefreshPrefix is the prefix of every chunk: …/refresh/
func EntraRefreshPrefix(tenantID, clientID string) string

// GitHubTarget: a GitHub token for owner, or for every owner when owner is "":
// PanaceaApp/git/github/<owner or "*">
func GitHubTarget(owner string) string

// PanaceaTarget: phase 7's pnc_ token for one API host:  PanaceaApp/panacea/<api host>
func PanaceaTarget(apiHost string) string

// A part (a tenant id, a client id, an owner, a host) may not be empty (except
// GitHubTarget's owner), contain "/", "*", a space or a control character, or
// exceed 128 bytes; the builders panic on one, because every caller passes a
// validated value and a bad one is a programming error.
```

```go
// control-plane/internal/credstore/chunk.go

// PutChunked writes data as ceil(len/MaxBlob) blobs at prefix+"0", prefix+"1", …
// and then deletes any higher-numbered chunk left from a longer previous value.
// It returns the chunk count, which the caller records in its own "written
// last" marker (the Entra account record). An empty data is 0 chunks.
func PutChunked(s Secrets, prefix, userName string, data []byte) (chunks int, err error)

// GetChunked reads n chunks and joins them. A missing chunk is ErrNotFound: the
// marker said n and the store disagrees, so the value is not trusted.
func GetChunked(s Secrets, prefix string, n int) ([]byte, error)

// DeleteChunked deletes every chunk under prefix (by List).
func DeleteChunked(s Secrets, prefix string) error
```

**`wincred_windows.go` — described, with its one declaration verbatim.** `golang.org/x/sys/windows`
has **no** Credential Manager wrappers (checked 2026-09-24: no `CredWrite`/`CredRead` in
`x/sys@{v0.24.0,v0.39.0,v0.47.0}/windows/*.go` in the module cache; v0.32.0, the version task 06's
pgx selects, is not cached, so it is unverified there — it lies between two versions that have none,
and Step 2 greps the version actually built), so the file loads `advapi32.dll` with
`windows.NewLazySystemDLL("advapi32.dll")` and calls `CredWriteW`, `CredReadW`, `CredDeleteW`,
`CredEnumerateW` and `CredFree` through `NewProc(…).Call`, with this struct:

```go
// CREDENTIALW, as wincred.h declares it.
type credentialW struct {
	Flags              uint32
	Type               uint32 // credTypeGeneric
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32 // credPersistLocalMachine
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2 // NEVER 3 (CRED_PERSIST_ENTERPRISE): a roaming profile would
	                            // copy the token to every machine the user touches (ADR-H1 §5.2)
	errorNotFound           = 1168
)
```

`Get` copies the blob out before `CredFree`; `Delete` of an absent target returns nil;
`List(prefix)` enumerates with filter `prefix + "*"` and treats `ERROR_NOT_FOUND` as empty. Blobs
are the secret's UTF-8 bytes. `userName` is written for display in the Credential Manager control
panel (the account's UPN, or `github`, or the Panacea host) — never a secret.

#### Tests to write first

`credstoretest.Run(t *testing.T, newStore func(t *testing.T) (s credstore.Secrets, prefix string))` is
the behaviour suite, run against `Memory()` everywhere and against `WinCred()` on Windows (with a
unique `PanaceaApp/test-<8 hex>/` prefix and a cleanup that deletes everything under it).

| Test | Asserts | Mutant it kills |
|---|---|---|
| `Run/put_get_delete` | put → get the same bytes; delete → `ErrNotFound`; delete again → nil | a delete that errors on absence, breaking sign-out after a partial write |
| `Run/overwrite` | put A then B → B | a store that keeps the first write |
| `Run/max_blob` | 2,560 bytes → ok; 2,561 → `ErrTooLarge`, and the error text holds no byte of the blob | a silent truncation of a refresh token |
| `Run/list_is_by_prefix` | targets `…/a/1`, `…/a/2`, `…/ab/1`; `List("…/a/")` → exactly the first two, sorted | a prefix match that reads another account's chunks |
| `Run/errors_never_hold_blobs` | every error from every method, formatted with `%v` and `%+v`, contains no blob byte sequence | a token in a log (T1, T10) |
| `TestChunkedRoundTrips6000Bytes` | `PutChunked` 6,000 bytes → 3 chunks; `GetChunked(3)` → the same bytes (Memory; and WinCred on Windows — contract §13 C1) | a KPMG-sized refresh token (R1) that cannot be stored |
| `TestChunkedShrinkDeletesStaleChunks` | 6,000 then 100 bytes → 1 chunk; chunks 1 and 2 are gone | a rotated shorter token read back with a stale tail |
| `TestAMissingChunkIsNotFound` | delete chunk 1 of 3; `GetChunked(3)` → `ErrNotFound` | a corrupted token sent to Entra |
| `TestTargetsAreExact` | each builder's output byte for byte, e.g. `EntraRefreshTarget("t","c",2)` = `PanaceaApp/entra/t/c/refresh/2`, `GitHubTarget("")` = `PanaceaApp/git/github/*` | two components spelling one target differently |
| `TestTargetPartsAreValidated` | a part with `/`, `*`, a space, a newline, or 129 bytes → panic | a caller-supplied owner that escapes its namespace |
| `TestWinCredPersistsOnThisMachineOnly` (Windows only) | after `Put`, a raw `CredReadW` shows `Persist == 2` and `Type == 1` | enterprise persistence roaming the token (ADR-H1 §5.2) |
| `TestWinCredUnsupportedOffWindows` (non-Windows only) | every method → `ErrUnsupported` | a Linux build that pretends to have stored a token |

#### Steps

- [ ] **Step 1:** Re-read contract §9.1 and ADR-H1 §5.2's Credential Manager paragraph.
- [ ] **Step 2:** Record the version the graph already holds: `cd control-plane && go list -m
  golang.org/x/sys > $TEMP/xsys.txt` (v0.32.0 or higher once task 06 has landed). **Never `go get`
  x/sys at a named version**: a lower one downgrades x/crypto, and with it pgx below v5.7.5 (review
  I8). Grep that version for wrappers: `grep -l "CredWrite\|CredRead" "$(go env
  GOMODCACHE)/golang.org/x/sys@<version>/windows/"*.go` → nothing. If it finds one, stop and record
  it: the hand-written `NewProc` calls below would duplicate a library wrapper.
- [ ] **Step 3:** The suite and tests; they fail. Then the seven source files. Then `go mod tidy`
  (tidy reads every build tag, so `wincred_windows.go`'s import counts on Linux too). `git diff
  control-plane/go.mod` shows only the x/sys line moving from the indirect block to the direct block,
  **at the version Step 2 recorded**; any other line moving is a defect to undo, not to commit.
- [ ] **Step 4:** On the owner's Windows laptop: the whole package under `-race`, which runs the
  WinCred half. Then `cmdkey /list | grep PanaceaApp/test-` → **empty** (the suite cleaned up).
- [ ] **Step 5:** CI is Linux (`azure-pipelines.yml:6`), so `wincred_windows.go` is never compiled
  there. Append one step after `Vet` — `- script: GOOS=windows go vet ./control-plane/...` with
  `displayName: Vet (windows)` — so a Windows-only compile error cannot merge. This edits phase 2's
  pipeline file; the step is additive, and phase 8's `pipelines/` tree may later absorb it.
- [ ] **Step 6:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/credstore/... -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 7 on Linux/macOS; 8 on Windows
GOOS=windows go vet ./control-plane/...                                                          # no output
go vet ./control-plane/...                                                                       # no output
cmdkey /list | grep -c "PanaceaApp/test-"                                                        # 0   (Windows, after the tests)
```

(`Run` counts as one top-level test with five subtests; on Windows it runs twice, for Memory and
WinCred, as `TestMemoryConforms` and `TestWinCredConforms`.)

#### What this task does not claim

- **The 2,560-byte limit**, and the `CREDENTIALW` layout, are from the Win32 documentation as
  recalled (contract §15; ADR-H1 R1). The Windows test proves a 6,000-byte round trip on the
  owner's machine; KPMG's real refresh-token size is measured at the first real sign-in (task 43).
- That DPAPI protection is what an enterprise auditor requires; it is what `OD-6` chose.
- Any typed accessor: task 29 (GitHub, Panacea) and task 31 (Entra).
