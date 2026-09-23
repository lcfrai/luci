# Task 10 — The store: the op log, snapshots, reconstruction, crash safety

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 06 · **Hours:** 2.5

## Why

The authority must outlive a window (spec §7.2) and a restart. And three features need the
document **as it was at a past revision**: reading the approved design (`at: approved`, spec §8.1),
building a snapshot for an approval of revision *N* while agents have moved on (task 21), and
*changed after approval* (task 21). The op log is also the undo stack, the activity feed and the
audit trail (spec §7.3). One append-only log of `doc.Change`s plus periodic snapshots gives all of
it. The store is an interface so v2's Hub can host a Postgres one (S5-H.1).

**What the store never holds** (ADR-H1 §5.4, A5 widened): approvals, design links, brief text, or
anything else fetched from the Hub. Canvas documents are local data in v1, not Hub data.

## Files

- `canvas/store/store.go`, `filestore.go`, `memstore.go`, `replay.go` (new)
- `canvas/store/storetest/suite.go` (new) — one conformance suite for both implementations
- `canvas/store/filestore_test.go`, `memstore_test.go` (new)

## Contracts

```go
// canvas/store/store.go
package store

var (
	ErrNotFound = errors.New("store: no such canvas")
	ErrExists   = errors.New("store: already exists")
	ErrRevision = errors.New("store: change is not the next revision")
	ErrCorrupt  = errors.New("store: log is corrupt")
	ErrQuota    = errors.New("store: canvas asset quota exceeded")
)

const (
	SnapshotEvery = 500        // a snapshot is written after every 500th revision
	AssetQuota    = 1 << 30    // 1 GiB of assets per canvas, the Hub's project quota (contract §7.6)
)

// Binding is (local project, local branch) → canvas. v1 ids are local and
// path-hashed; v2 re-keys by Hub project and branch ref (S5-H.1, HA5-5).
type Binding struct {
	ProjectID string `json:"projectId"`
	BranchID  string `json:"branchId"`
	CanvasID  string `json:"canvasId"`
	CreatedAt string `json:"createdAt"`
}

// Store is safe for concurrent use across canvases. Calls for ONE canvas
// are serialised by the authority (task 11), and the store may assume so.
type Store interface {
	Bind(b Binding) error // ErrExists if (ProjectID, BranchID) is bound, or CanvasID is used
	Lookup(projectID, branchID string) (Binding, bool, error)
	Bindings() ([]Binding, error)

	Create(d doc.Document) error // Revision must be 0; ErrExists if the canvas exists
	Load(canvasID string) (doc.Document, error) // the head: last snapshot + every later change
	// Append is durable before it returns. c.Revision must be head+1, else ErrRevision.
	Append(canvasID string, c doc.Change) error
	// Changes returns changes with Revision > after, ascending, at most limit (≤ 1,000).
	Changes(canvasID string, after, limit int) ([]doc.Change, error)
	// At reconstructs the document as of revision rev (0 ≤ rev ≤ head).
	At(canvasID string, rev int) (doc.Document, error)

	// PutAsset stores bytes content-addressed and returns the record; the same
	// bytes return the same record. ErrQuota past AssetQuota.
	PutAsset(canvasID, mediaType string, data []byte, width, height int) (doc.Asset, error)
	Asset(canvasID, ref string) (doc.Asset, []byte, error)
}

func NewFile(dir string, log *slog.Logger) (*FileStore, error) // creates dir 0700; dir must be absolute
func NewMem() *MemStore
```

```
// FileStore layout under dir (the control plane passes statepath.Dir("canvas"), task 20b)
bindings.json                               {"v":1,"bindings":[Binding…]}  — written by temp file + rename
<canvasId>/changes.jsonl                    one doc.Change per line; append, then fsync, per change
<canvasId>/snapshot-<rev, 10 digits>.json   a doc.Document at that revision; revision 0 is written by Create
<canvasId>/assets/<64 hex>                  bytes, written by temp file + rename
<canvasId>/assets/<64 hex>.json             the doc.Asset record
```

**Snapshot retention:** revision 0, every multiple of 5,000, and the two most recent. `At` replays
from the nearest kept snapshot at or below `rev`, so a replay is at most 5,000 changes.

```go
// canvas/store/replay.go

// Replay applies one logged change to d. It re-runs doc.Apply with the
// change's own ops, author and time and a Checker that accepts everything —
// the ops were checked when they were accepted — and fails with ErrCorrupt
// unless the result's revision is c.Revision.
func Replay(d *doc.Doc, c doc.Change) error
```

**Crash rules**, each a test:

1. `Append` returns only after the line and its newline are fsynced.
2. On `Load`, a **final** line that is torn (no newline, or invalid JSON) is truncated away with a
   warning: it was never acknowledged. Any **earlier** bad line is `ErrCorrupt` — never skipped.
3. A snapshot is written after its change is durable; a crash between the two leaves a log that
   replays to the same state.
4. `bindings.json` is never half-written: temp file, fsync, rename.

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `storetest.Run` › *bind, lookup, conflict* | second bind of one branch is `ErrExists`; lookup returns it; `Bindings()` sorted by project then branch | two canvases for one branch |
| `storetest.Run` › *append order* | appending revision 3 to a head of 1 is `ErrRevision` | a gap in the log |
| `storetest.Run` › *load equals live* | 1,200 random batches applied live and appended; `Load` equals the live `Document()` byte for byte | replay diverging from live apply |
| `storetest.Run` › *At* | `At(rev)` for 20 random revisions equals the document captured live at that revision | reconstruction from the wrong snapshot |
| `storetest.Run` › *assets* | same bytes twice → one record; 1 GiB + 1 byte total → `ErrQuota`; `Asset` returns the bytes | a non-idempotent put |
| `TestFileStore_tornTail` | a file ending mid-line loads to the last complete change; the torn bytes are gone after `Load` | a torn tail treated as corruption (the canvas would never open again) |
| `TestFileStore_corruptMiddle` | a bad line 3 of 10 → `ErrCorrupt` | a silently skipped change |
| `TestFileStore_snapshotRetention` | kept snapshots after 10,500 revisions: exactly {0, 5,000, 10,000, 10,500}; after 11,500: exactly {0, 5,000, 10,000, 11,000, 11,500} (10,500 is neither a milestone nor one of the two most recent) | unbounded disk use; a milestone deleted |
| `TestFileStore_restart` | a new `FileStore` on the same dir loads every canvas and binding | state kept only in memory |
| `TestFileStore_noHubData` | after a full fixture run, no file under the dir contains the substrings `"approvedBy"`, `"reviewDigest"` or `"briefDigest"` | an approval cached on disk (ADR-H1 §5.4) |
| `TestMemStore` | `storetest.Run` passes | the fake drifting from the real store |

## Steps

1. Write `storetest/suite.go` and the file-store tests. 2. Implement `memstore.go`, then
   `filestore.go` and `replay.go`. 3. Acceptance; commit by pathspec.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/store/... -race -count=1
```
Expected: `ok  	…/canvas/store` (and `[no test files]` for `storetest`).

## Not claimed

- **One process per state directory.** The FileStore takes no OS file lock, like phase 2's
  registry file store; two control planes on one `PANACEA_APP_STATE_DIR` are unsupported.
- No deletion of canvases: nothing in v1 deletes one (spec §8.4 keeps delete out of MCP; the lens
  has no delete). Worktree or branch removal does not remove a canvas (seam S5-P3.1).
- Disk use is bounded by retention, not capped.
