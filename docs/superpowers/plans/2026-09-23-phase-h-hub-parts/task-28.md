### Task 28: `credstore` — the machine's secret store: Windows Credential Manager, memory, chunking, one target grammar (seam S-CRED)

**Lane C. Depends on: 06 merged (go.mod). About 3 hours. Contract task C1.**

`OD-6`: tokens are kept in **Windows Credential Manager** (DPAPI-protected, per user), and **the
same store** holds the Azure DevOps and GitHub tokens phase 3 needs (`OD-4`) and the Panacea token
phase 7 needs (`OD-13`). This task is that store's lowest layer — **seam S-CRED**: one `Secrets`
interface, two implementations, one chunking rule for tokens larger than a credential blob, and
**one** place every target name is spelled. Task 29 builds the typed accessors on it (GitHub,
Panacea); task 31 stores the realm's offline refresh token through it (OD-30: Keycloak brokering
Entra, main `docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md` §11 `:581`); task 39 is the only write path for a
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
// (OD-6): the realm's refresh token (OD-30), a GitHub token, a Panacea token. Windows
// Credential Manager in production, memory in tests and off Windows. There is no
// file-backed store: a file is the thing Credential Manager exists to avoid
// (contract §9.1). Nothing in this package logs; no error carries a blob.
//
// Phase 3's credstore.go (Host, Credential, Store, None, Chain — phase 3 task 13)
// lives in this same package; if absent, Hub task 29 writes it, and phase 3's file wins.
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

// The sign-in targets key on the ISSUER (OD-30 ruling §11 :581; Hub contract §17 §9.1), so
// a token from one realm is never offered to another. issuer is the validated
// PANACEA_APP_OIDC_ISSUER (task 32's TestTheIssuerIsARealmURL): <scheme>://<host>/realms/<realm>.
// <issuer host> is the URL's host, port included (127.0.0.1:8751 locally); <realm>
// is the path segment after /realms/. These replace EntraAccountTarget and
// EntraRefresh*(tenantID, clientID), which keyed on an Entra tenant and client.

// OIDCAccountTarget: the account record, written LAST so it marks a complete
// write (contract §9.1):  PanaceaApp/oidc/<issuer host>/<realm>/account
func OIDCAccountTarget(issuer string) string

// OIDCRefreshTarget: chunk i of one generation of the refresh token:
//   PanaceaApp/oidc/<issuer host>/<realm>/refresh/<gen>/<i>
// Every write of a refresh token is a new generation (NewGeneration, chunk.go), so
// a write never overwrites a chunk the account record still names (review I1).
func OIDCRefreshTarget(issuer, gen string, i int) string

// OIDCRefreshGenPrefix is the prefix of one generation's chunks: …/refresh/<gen>/
func OIDCRefreshGenPrefix(issuer, gen string) string

// OIDCRefreshPrefix is the prefix of every generation's chunks: …/refresh/
func OIDCRefreshPrefix(issuer string) string

// GitHubTarget: a GitHub token for owner, or for every owner when owner is "":
// PanaceaApp/git/github/<owner or "*">
func GitHubTarget(owner string) string

// AzureDevOpsTarget: an Azure DevOps personal access token for one organisation
// (review I10): PanaceaApp/git/azure-devops/<org>. No wildcard: a PAT is minted
// per organisation, so org "" is refused like any other empty part.
func AzureDevOpsTarget(org string) string

// PanaceaTarget: phase 7's pnc_ token for one API host:  PanaceaApp/panacea/<api host>
func PanaceaTarget(apiHost string) string

// An issuer that is not <scheme>://<host>/realms/<realm> is a bad part. A part (an
// issuer host, a realm, an owner, an organisation, a host) may not be empty (except
// GitHubTarget's owner), contain "/", "*", a space or a control character, or
// exceed 128 bytes; a gen is exactly 16 lowercase hex characters. The builders
// panic on a bad one, because every caller passes a validated value and a bad one
// is a programming error. A gen has two sources: NewGeneration, and the account
// record read back from the store. The second is NOT validated configuration (a
// newer format read after a downgrade, a hand-edited or corrupted entry), so task
// 31 passes a stored gen to ValidGeneration before any builder sees it, and a gen
// that fails is handled as a missing one, never a panic (plan re-review b03 N1).

// ValidGeneration reports whether gen is exactly 16 lowercase hex characters: the
// rule the builders enforce, exported so a parser can check a stored value first.
func ValidGeneration(gen string) bool
```

```go
// control-plane/internal/credstore/chunk.go

// Chunks are written by GENERATION (review I1). A value that spans several blobs
// cannot be replaced in place atomically: overwriting chunk 0 and then failing on
// chunk 1 leaves new-0 + old-1 behind a valid marker, which is neither value. So a
// write never touches the chunks a reader may be using. The sequence, which task 31
// follows for the realm's offline refresh token (OD-30):
//   1. gen := NewGeneration(); PutChunked under that generation's prefix.
//   2. Write the "written last" marker (the OIDC account record), naming gen and
//      the chunk count. Until this Put succeeds the old marker names the old
//      generation, whose chunks are untouched.
//   3. Only then DeleteChunked the old generation's prefix. A failure or a crash
//      here leaves an unnamed generation, which SweepGenerations removes at start-up.
// A failure in step 1 or 2 returns the error; the caller best-effort deletes the
// new generation's prefix, and start-up's sweep catches what it misses.

// NewGeneration returns 16 lowercase hex characters from crypto/rand. Random, not
// old+1, so two writers (a retry after a crash, a second process) never share a
// chunk target.
func NewGeneration() (string, error)

// PutChunked writes data as ceil(len/MaxBlob) blobs at prefix+"0", prefix+"1", …
// prefix is one fresh generation's (OIDCRefreshGenPrefix). It never deletes and
// never writes outside prefix. It returns the chunk count, which the caller records
// with the generation in its marker. An empty data is 0 chunks. On an error the
// chunks written so far stay; see the sequence above.
func PutChunked(s Secrets, prefix, userName string, data []byte) (chunks int, err error)

// GetChunked reads n chunks and joins them. A missing chunk is ErrNotFound: the
// marker said n and the store disagrees, so the value is not trusted.
func GetChunked(s Secrets, prefix string, n int) ([]byte, error)

// DeleteChunked deletes every chunk under prefix (by List): one generation's
// prefix after a rotation, or OIDCRefreshPrefix, every generation, at sign-out.
func DeleteChunked(s Secrets, prefix string) error

// SweepGenerations deletes every chunk under prefix (OIDCRefreshPrefix) whose
// generation segment is not keep; keep "" deletes every generation. Start-up calls
// it with the generation the marker names, before any write.
func SweepGenerations(s Secrets, prefix, keep string) error
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
| `TestAWriteNeverTouchesAnotherGeneration` | 6,000 bytes under generation A (3 chunks), then 100 bytes under generation B (1 chunk); A's `GetChunked(3)` still returns the 6,000 bytes whole, B's `GetChunked(1)` the 100; two `NewGeneration` values differ and each is 16 lowercase hex | the in-place overwrite (review I1): a rotation that tears the token the marker still names |
| `TestSweepKeepsOnlyTheNamedGeneration` | generations A (3 chunks), B (1) and a stray C (2, no marker); `SweepGenerations(prefix, B)` → `List(prefix)` is exactly B's chunk 0; `SweepGenerations(prefix, "")` → empty; a chunk under a **different account's** prefix is untouched | a sweep that deletes the named generation; a stale generation left for ever; a sweep that crosses accounts |
| `TestAMissingChunkIsNotFound` | delete chunk 1 of 3; `GetChunked(3)` → `ErrNotFound` | a corrupted token sent to the issuer |
| `TestTargetsAreExact` | each builder's output byte for byte, e.g. `OIDCRefreshTarget("https://auth.dev.panacea.kpmg/realms/panacea","0123456789abcdef",2)` = `PanaceaApp/oidc/auth.dev.panacea.kpmg/panacea/refresh/0123456789abcdef/2`, `OIDCRefreshGenPrefix("http://127.0.0.1:8751/realms/panacea-app-dev","0123456789abcdef")` = `PanaceaApp/oidc/127.0.0.1:8751/panacea-app-dev/refresh/0123456789abcdef/`, `OIDCAccountTarget` of two realms on one host → two targets, and an issuer with no `/realms/` segment panics, `GitHubTarget("")` = `PanaceaApp/git/github/*`, `AzureDevOpsTarget("kpmg-au")` = `PanaceaApp/git/azure-devops/kpmg-au` (review I10) | two components spelling one target differently |
| `TestTargetPartsAreValidated` | a part with `/`, `*`, a space, a newline, or 129 bytes → panic; a gen of 15 or 17 characters, or with an uppercase or non-hex character → panic, and `ValidGeneration` of each of those → false (of `"0123456789abcdef"` → true) | a caller-supplied owner that escapes its namespace; a `ValidGeneration` that accepts what a builder panics on |
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
go test ./control-plane/internal/credstore/... -race -count=1 -v 2>&1 | grep -c "^--- PASS"   # 8 on Linux/macOS; 9 on Windows
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
- Any typed accessor: task 29 (GitHub, Panacea) and task 31 (the realm's refresh token).
