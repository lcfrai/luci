### Task 03: `lanes` — the lane index, clone creation with the user's identity, and the composite catalog

OD-2 makes a cockpit worktree a local clone. This task builds the three things that make a clone
a worktree the rest of the product can see: a small persisted **lane index** (which clone belongs
to which project and product branch — a fact git cannot hold, because a clone does not know it is
a lane), **clone creation**, and a **catalog** that lists clones beside phase 2's
`git worktree list` so the rail, `PATCH …/worktrees/{id}` and session create all see them through
the seams they already use (`api.Enumerator`, `worktree.Provider`, seam S3.2). No route yet: task
04 wires it.

**One fact from task 01 shapes clone creation.** A clone does not inherit its source's
`.git/config`, nor an `includeIf "gitdir:…"` section that matched the project's path, so
`git var GIT_AUTHOR_IDENT` in a clone can answer differently from the project
(`TestBehaviourGitVarInACloneMissesTheSourcesLocalIdentity`). The sandbox fix resolves identity
per worktree path. So creation asks `git var` **in the project** and writes the answer into the
clone's local config; the fix's resolver then gives the clone the answer the project would.

**Files:**
- `control-plane/internal/statepath/atomic.go` + `atomic_test.go` — **only if** `statepath` has no
  `WriteAtomic` yet: orchestrator ruling R2's lift of `registry.writeAtomic`
  (`registry/filestore.go:460`) with its tests (`registry/writeatomic_test.go`) moved, not copied;
  `registry` then calls `statepath.WriteAtomic` (p2, mechanical)
- `control-plane/internal/gitx/ident.go` + test — `ParseIdent` (new)
- `control-plane/internal/lanes/store.go` — `Lane`, `Queue`, `Rename`, `Store`, `FileStore` (new)
- `control-plane/internal/lanes/clone.go` — `Creator`, `CreateRequest`, errors, `RemoveTree` (new)
- `control-plane/internal/lanes/catalog.go` — `Source`, `Catalog` (new)
- `control-plane/internal/lanes/*_test.go` (new)
- `control-plane/internal/project/project.go` — `Worktree.Kind`, set in `parseWorktrees` (p2; no git)
- `control-plane/internal/project/project_test.go` — one test for `Kind` (p2, appended)

#### Interfaces — complete

```go
package gitx

// Ident is one person's commit identity.
type Ident struct{ Name, Email string }

// ParseIdent reads `git var GIT_AUTHOR_IDENT`'s "Name <email> 1790156728 +1000". The first
// '<' and the first '>' after it delimit the email; git strips angle brackets from both
// parts. An empty name or email is not an identity (ok == false). This is the one parser
// of that line: task 04 points cmd/api's gitidentity.go at it and deletes its copy.
func ParseIdent(line string) (id Ident, ok bool)
```

```go
// Package lanes owns the cockpit's worktrees: local clones of a project repository, one per
// worktree (OD-2), and the small index that says which project and product branch each
// belongs to. git is the truth for every git fact; this index holds only what git cannot.
package lanes

// Lane is one cockpit worktree.
type Lane struct {
	ID        string    `json:"id"`        // project.WorktreeID(Path): "w-" + 8 hex
	ProjectID string    `json:"projectId"` // the project it was cloned from
	Branch    string    `json:"branch"`    // the product branch's short ref name, e.g. "evidence-upload"
	Name      string    `json:"name"`      // the clone directory's base name, unique per project
	Path      string    `json:"path"`      // absolute: <CloneRoot>/<ProjectID>/<Name>
	MachineID string    `json:"machineId"` // gitx.Machine.ID(); "local" in phase 3
	CreatedAt time.Time `json:"createdAt"`
}

// Queue is the human's landing order for one product branch (plan decision 1). Order holds
// worktree ids; an id that is not a ready lane of that branch is ignored when read.
type Queue struct {
	ProjectID string   `json:"projectId"`
	Branch    string   `json:"branch"`
	Order     []string `json:"order"`
}

// Rename is one branch rename the cockpit performed (brief A8). Task 11 writes these.
type Rename struct {
	ProjectID    string    `json:"projectId"`
	FromBranchID string    `json:"fromBranchId"`
	ToBranchID   string    `json:"toBranchId"`
	From         string    `json:"from"`
	To           string    `json:"to"`
	At           time.Time `json:"at"`
}

var (
	ErrExists  = errors.New("lanes: a lane with that id or name already exists")
	ErrUnknown = errors.New("lanes: no lane with that id")
)

// Store is the lane index. Every method is safe for concurrent use.
type Store interface {
	List() []Lane
	ByID(id string) (Lane, bool)
	ByProject(projectID string) []Lane                 // CreatedAt order, then Name
	Add(l Lane) error                                  // ErrExists on a duplicate ID, or ProjectID+Name
	Remove(id string) error                            // ErrUnknown; also drops id from every Queue
	Queue(projectID, branch string) []string           // nil when never set
	SetQueue(projectID, branch string, order []string) error
	Renames(projectID string) []Rename                 // oldest first
	AddRename(r Rename) error
	// Retarget moves every lane and queue of (projectID, from) to branch to. Task 11's rename
	// refuses while lanes exist (open question Q4), so in v1 it only ever moves a queue.
	Retarget(projectID, from, to string) error
}

// FileStore keeps the index in one JSON document, <dir>/lanes.json, schemaVersion 1,
// written through statepath.WriteAtomic on every mutation and rolled back in memory when the
// write fails (registry.FileStore's discipline, filestore.go:422-440).
type FileStore struct{ /* unexported */ }

// OpenFileStore loads dir/lanes.json, creating dir (0o700) and an empty index when absent.
// An unreadable or unparseable file is an error naming the path — never an empty index,
// which would orphan every clone on disk without a word.
func OpenFileStore(dir string) (*FileStore, error)
```

```go
package lanes

var (
	ErrUnknownBranch = errors.New("lanes: the project has no such branch")
	ErrNameTaken     = errors.New("lanes: that worktree name is taken, or its directory exists")
	ErrBadName       = errors.New("lanes: a worktree name is 1-40 of a-z 0-9 . _ - and starts with a letter or digit")
)

// CreateRequest is one worktree to create. Project comes from the enumerator, so its Path is
// the scanner's own answer, never a path a client supplied.
type CreateRequest struct {
	Project project.Project
	Branch  string // must resolve as refs/heads/<Branch> in Project.Path
	Name    string // "" derives "<last segment of Branch, slugged, ≤ 24>-<n>", lowest free n ≥ 1
}

// Creator makes lane clones on one machine.
type Creator struct {
	Machine gitx.Machine
	Store   Store
	Now     func() time.Time // nil means time.Now
}

// Create, in this order, and the order is the point:
//  1. validate Name (ErrBadName); derive it when empty;
//  2. `rev-parse --verify --quiet refs/heads/<Branch>^{commit}` in the project (Read) —
//     ErrUnknownBranch, and nothing on disk;
//  3. Path = <CloneRoot>/<ProjectID>/<Name>; ErrNameTaken when the index has the name OR
//     anything exists at Path — an existing directory is never removed or reused;
//  4. `clone --local --branch <Branch> --origin origin -- <Project.Path> <Path>` (Local);
//  5. `var GIT_AUTHOR_IDENT` in the PROJECT (Read); when ParseIdent accepts it,
//     `config user.name` and `config user.email` in the clone (Local). No identity is not an
//     error: the sandbox fix's resolver logs it at session create;
//  6. Store.Add.
// A failure in 4-6 removes Path with RemoveTree before returning, so a failed create leaves
// no directory behind; a failure in 1-3 has touched nothing.
// Scopes, per task-02 §HGH H3: steps 2, 4 and 5's `var` run with Dir the project and Scope
// Project (step 4's clone checks its source that way); step 5's `config` writes run in the
// clone with Scope Clone.
func (c *Creator) Create(ctx context.Context, req CreateRequest) (Lane, error)

// RemoveTree deletes a directory tree that git wrote. On Windows git leaves object and pack
// files read-only, and os.RemoveAll fails on them (task 01's TestBehaviourRemoveAllOnAClone
// records whether it does on this machine); RemoveTree clears the read-only attribute during
// the walk and then removes. It refuses a path that is not under root.
func RemoveTree(root, path string) error
```

```go
package lanes

// Source is the phase 2 enumeration the catalog composes: *project.Scanner satisfies it.
type Source interface {
	Projects(ctx context.Context) ([]project.Project, error)
	Branches(ctx context.Context, projectID string) ([]project.Branch, error)
	Worktrees(ctx context.Context, projectID string) ([]project.Worktree, error)
	Path(ctx context.Context, worktreeID string) (string, error)
}

// Catalog is what the API serves as both api.Enumerator and worktree.Provider: the scanner's
// primary and linked worktrees, then this project's lane clones, each with Kind "clone" and
// BranchID = project.BranchID(projectID, lane.Branch).
type Catalog struct {
	Source Source
	Store  Store
}

var (
	_ worktree.Provider = (*Catalog)(nil)
)

func (c *Catalog) Projects(ctx context.Context) ([]project.Project, error)
func (c *Catalog) Branches(ctx context.Context, projectID string) ([]project.Branch, error)
func (c *Catalog) Worktrees(ctx context.Context, projectID string) ([]project.Worktree, error)

// Path resolves a lane first (worktree.Verify on its Path, so a clone deleted outside the
// cockpit is worktree.ErrNotADir, the 404 session create already maps), then the Source.
func (c *Catalog) Path(ctx context.Context, worktreeID string) (string, error)
```

```go
// project.go — the one field this task adds, and the only change to that package.
type Worktree struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	BranchID  string `json:"branchId"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsPrimary bool   `json:"isPrimary"`
	// Kind is "primary" for the repository's main worktree, "linked" for one made with
	// `git worktree add`, and "clone" for a cockpit worktree (set by lanes.Catalog, never
	// here). Git works inside a sandbox for "primary" and "clone"; not for "linked", whose
	// .git is a file naming a host path (the sandbox git fix's finding).
	Kind string `json:"kind"`
}
```

#### The tests to write first

Real git through `gitxtest` wherever the claim is about git; a fake `Source` for the catalog.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestParseIdent` | `"Ada <ada@x.io> 1 +0000"` → Ada/ada@x.io; `"<a@b> 1 +0"` → not ok; `"Ada 1 +0"` → not ok | accepting half an identity |
| `TestFileStoreRoundTripsAcrossReopen` | Add two lanes, SetQueue, AddRename; reopen → identical | an index that does not survive a restart |
| `TestFileStoreRefusesDuplicateIDOrName` | same ID → ErrExists; same ProjectID+Name, other ID → ErrExists; same Name, other project → ok | two clones in one directory |
| `TestFileStoreRemoveDropsTheIDFromQueues` | Remove(w) → `Queue(p,b)` no longer holds w | a queue naming a worktree that is gone |
| `TestFileStoreFailedWriteLeavesMemoryUnchanged` | a read-only dir makes Add fail → List unchanged | memory and disk disagreeing after a full disk |
| `TestOpenFileStoreRefusesACorruptFile` | `{not json` → error containing the path | an empty index orphaning every clone |
| `TestCreateClonesOnTheBranch` | clone's `rev-parse HEAD` equals the project's `refs/heads/feat` and `symbolic-ref HEAD` is `refs/heads/feat`; `Lane.ID == project.WorktreeID(Path)`; the lane is in the store | a clone on the default branch |
| `TestCreateCopiesTheProjectsIdentity` | project has local `user.name=Ada`; the clone's `git var GIT_AUTHOR_IDENT` starts `Ada <` | agents committing as the machine-wide identity in a repo that sets its own |
| `TestCreateWithoutAnIdentityStillCreates` | no identity anywhere (isolated config, `user.useConfigOnly`) → lane created, clone config has no `user.name` | failing a create for a commit-time problem |
| `TestCreateUnknownBranchTouchesNothing` | `Branch: "nope"` → ErrUnknownBranch, `<root>/<p>` has no entry | a half-made clone |
| `TestCreateNeverReusesAnExistingDirectory` | a file at Path → ErrNameTaken and the file's bytes are unchanged | RemoveTree on a directory we did not make |
| `TestCreateCleansUpAfterAFailedClone` | a Runner that fails the clone after creating Path → Path absent | leaving a broken clone for the catalog to list |
| `TestCreateRefusesBadNames` | `""`→derived; `"../x"`, `"A"`, `"-x"`, 41 chars → ErrBadName | a name that escapes CloneRoot |
| `TestCreateDerivesTheLowestFreeName` | branch `feat/upload`, lanes `upload-1` and `upload-3` exist → `upload-2` | a derived name colliding |
| `TestRemoveTreeDeletesReadOnlyObjects` | a real clone → RemoveTree succeeds and the path is gone | `os.RemoveAll` failing on Windows' read-only packs |
| `TestRemoveTreeRefusesOutsideRoot` | `RemoveTree(root, filepath.Dir(root))` → error, nothing deleted | a lane path that escaped the root deleting the user's disk |
| `TestCatalogListsClonesAfterTheScannersWorktrees` | Source gives primary+linked; store has 2 lanes of p and 1 of q → p's list is primary, linked, then 2 clones with `Kind:"clone"`, `IsPrimary:false`, the right BranchID | clones missing from the rail |
| `TestCatalogPathResolvesALaneFirst` | a lane id resolves to its path without calling Source.Path | a clone unreachable by session create |
| `TestCatalogPathOfAVanishedClone` | lane dir deleted → `errors.Is(err, worktree.ErrNotADir)` | a mount of a missing directory |
| `TestParseWorktreesSetsKind` (project_test.go) | first record `Kind:"primary"`, the rest `"linked"` | a linked worktree reported as primary |

#### Steps

- [ ] **Step 1: R2's lift, if needed.** `grep -n "func WriteAtomic" control-plane/internal/statepath/*.go`.
  One hit: phase 4 did it; use it. None: move `writeAtomic` and `writeatomic_test.go` into
  `statepath` as `WriteAtomic`, change `registry`'s two call sites, run
  `go test ./control-plane/internal/registry/ ./control-plane/internal/statepath/ -count=1`, and
  commit that move **alone**, before anything else, so the lift is one reviewable commit.
- [ ] **Step 2:** write the tests; `gitxtest.NewRepo` for the git ones.
- [ ] **Step 3:** implement `ident.go`, `store.go`, `clone.go`, `catalog.go`, and the `Kind` field.
- [ ] **Step 4:** run and commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/lanes/ ./control-plane/internal/gitx/ -race -count=1   → PASS
go test ./control-plane/internal/project/ -count=1                                      → PASS, TestEveryGitCommandIsReadOnly unchanged
go test ./control-plane/internal/registry/ ./control-plane/internal/statepath/ -count=1  → PASS
```

#### What this task does not claim

- Nothing serves the catalog yet (task 04), so the rail does not show clones yet.
- A lane whose product branch was deleted outside the cockpit still lists, under a branch id no
  branch has; the rail shows it nowhere. Task 11's remove clears it; nothing detects it here.
- Clone creation on a second volume copies objects instead of hard-linking them; task 01's
  measurement says whether that matters on the owner's laptop.
