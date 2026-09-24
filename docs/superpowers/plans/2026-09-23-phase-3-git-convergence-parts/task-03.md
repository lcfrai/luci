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
- `control-plane/internal/briefref/briefref.go` — `Revision`, `Criterion`, `Resolver`, `Archiver`;
  interfaces only, moved here from task 09 so tasks 09, 11 and 15 share one owner (review I7) (new)
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

// Lane is one cockpit worktree. Its ID is phase 2's rule, the hash of its Path, and its Path ends
// in a creation nonce, so removing a lane and creating another with the same Name gives a new
// directory and a new ID (review Minor 7). Without the nonce the new lane would inherit the old
// one's id, and with it every record keyed on that id: the Panacea-Land trailers history names
// (task 10's worktreeName), undo's SetWorktreeReady, registry's WorktreeFlags, the ref
// refs/panacea/lanes/<id> and task 11's refs/panacea/archive/lanes/<id>/*. The ID stays WorktreeID(Path), not a hash of path plus a counter, so
// every id in the product is still derived by the one rule phase 2 wrote.
type Lane struct {
	ID        string    `json:"id"`        // project.WorktreeID(Path): "w-" + 8 hex; new per creation
	ProjectID string    `json:"projectId"` // the project it was cloned from
	Branch    string    `json:"branch"`    // the product branch's short ref name, e.g. "evidence-upload"
	Name      string    `json:"name"`      // the worktree's name, unique per project among its lanes
	Path      string    `json:"path"`      // absolute: <CloneRoot>/<ProjectID>/<Name>-<Nonce>
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
	// Nonce gives step 3's directory suffix: nil means 6 lowercase hex from crypto/rand. Tests pin
	// it to make a collision. It is never derived from the name, the time or a counter.
	Nonce func() string
	// Mounts holds sandboxes off the project's working tree while step 6 reads it (re-review
	// b03 I-2). nil copies no hooks directory: fail closed.
	Mounts MountGuard
}

// MountGuard answers "can a sandbox write dir right now" and holds the answer. WhileUnmounted
// runs fn only when no session that is live or starting has dir as its worktree path, and it
// holds every new mount of dir off until fn returns; otherwise it returns false and runs
// nothing. Task 04's api.MountGuard implements it under the lock that session create and
// restart take before they start a sandbox.
type MountGuard interface {
	WhileUnmounted(ctx context.Context, dir string, fn func() error) (ran bool, err error)
}

// Create, in this order, and the order is the point:
//  1. validate Name (ErrBadName); derive it when empty;
//  2. `rev-parse --verify --quiet refs/heads/<Branch>^{commit}` in the project (Read) —
//     ErrUnknownBranch, and nothing on disk;
//  3. ErrNameTaken when the index has Name for this project. Path =
//     <CloneRoot>/<ProjectID>/<Name>-<c.Nonce()>; when anything exists at Path (os.Lstat), draw
//     again, at most 4 draws, then ErrNameTaken. An existing directory is never removed or
//     reused. A removed lane's directory is gone, so a draw can repeat its nonce only by
//     chance: 1 in 16.7 million per draw, the same order as two paths sharing an 8-hex
//     WorktreeID, which phase 2 already accepts (review Minor 7);
//  4. `clone --no-hardlinks --config core.autocrlf=false --config core.eol=lf
//     --config core.checkStat=minimal --config core.trustctime=false
//     --config core.longpaths=true --config core.symlinks=false
//     --branch <Branch> --origin origin -- <Project.Path> <Path>` (Local), with Call.Env
//     holding NoFilterEnv. `--no-hardlinks` copies every object file, so none is the
//     project's file (review I8, plan Q9). Clone's own `--config` (git's `clone -c`) writes
//     all six keys into the clone's local config before the checkout, so the checkout itself
//     runs under them ("Line endings" and "Blob bytes on disk" below);
//  4a. `config remote.origin.url <NoFetchURL>` and `config remote.origin.pushurl <NoPushURL>`
//     in the clone (Local, Scope Clone): the clone's `origin` stops naming a Windows path,
//     which the sandbox's Linux git would read as SSH to a host named `C` (review Minor 14).
//     No host call reads either value (task-02 §HGH H2: the refresh and the lane sync name
//     the project by its literal path from the lane index);
//  5. `var GIT_AUTHOR_IDENT` in the PROJECT (Read); when ParseIdent accepts it,
//     `config user.name` and `config user.email` in the clone (Local). No identity is not an
//     error: the sandbox fix's resolver logs it at session create;
//  6. `config --local --get core.hooksPath` in the PROJECT (Read; exit 1 means unset). When
//     CopyableHooksPath accepts the value, `config core.hooksPath <value>` in the clone
//     (Local), byte for byte. `git clone` copies neither `.git/config` nor `.git/hooks`, so
//     without this a husky repository's lane runs none of its hooks (review I11). Nothing
//     under `.git/hooks` is ever copied: a host hook's `#!` line names a Windows path. A value
//     CopyableHooksPath refuses is not copied and not an error. When the value was copied,
//     CopyGeneratedHooks then copies the directory it names, if the project generated one,
//     inside c.Mounts.WhileUnmounted(ctx, Project.Path, …). ran == false (a primary-worktree
//     session is live or starting, or Mounts is nil) copies no files and is not an error: the
//     setting stays, and task 05's dispatcher line names the missing folder;
//  7. Store.Add.
// A failure in 4-7 removes Path with RemoveTree before returning, so a failed create leaves
// no directory behind; a failure in 1-3 has touched nothing.
// Scopes, per task-02 §HGH H3: steps 2, 4, 5's `var` and 6's `--get` run with Dir the project
// and Scope Project (step 4's clone checks its source that way); steps 4a, 5 and 6's `config`
// writes, and CopyGeneratedHooks' `ls-files`, run in the clone with Scope Clone.
func (c *Creator) Create(ctx context.Context, req CreateRequest) (Lane, error)

// NoFetchURL and NoPushURL are a lane clone's origin (step 4a, review Minor 14). Each is a path
// that exists on neither machine and names who does the job, so the agent's `git pull` or
// `git push` in the sandbox fails with a sentence that tells it why. Task 01's
// TestSandboxCloneCannotFetch records that sentence verbatim for task 23's agent docs.
const (
	NoFetchURL = "/panacea-app/no-remote/the-cockpit-fetches-origin-and-upstream-for-this-worktree"
	NoPushURL  = "/panacea-app/no-remote/the-cockpit-lands-this-worktree-and-pushes-the-branch"
)

// NoFilterEnv is step 4's Call.Env: GIT_CONFIG_COUNT=3 with filter.lfs.smudge="",
// filter.lfs.process="" and filter.lfs.required=false, at command scope, so a system-wide
// Git LFS (Git for Windows' installer can define filter.lfs.*) neither runs nor smudges
// during the checkout (review Minor 13). It is env, not `--config`, because a `filter.*`
// key in the clone's local config is one task-02 §HGH H3's Clone allowlist refuses.
var NoFilterEnv map[string]string

// CopyableHooksPath reports whether a project's local core.hooksPath is copied into a lane
// clone: it is non-empty, relative (no leading `/` or `\`, no `X:` drive, no leading `~`),
// has no `..` segment, and holds no `\`. The first three are task-02 §HGH H3's Clone rule
// for the key, so a copied value never makes the clone's own config check refuse it; the
// `\` refusal is because the sandbox's Linux git reads `\` as part of a file name, so a
// Windows-style value would name no directory there. Husky writes `.husky/_`.
func CopyableHooksPath(v string) bool

// CopyGeneratedHooks copies <projectTop>/<rel> into <cloneTop>/<rel> when the project holds a
// generated hooks directory there, and reports how many files it copied. Husky's `.husky/_` is
// generated by the project's install step and ignored by git, so a clone never has it, and
// the install step cannot make it inside a lane: its `git config core.hooksPath` write fails
// against the read-only `.git/config` (task-02 §HGH H5), and husky 9 stops there (unverified:
// husky's source was not read for this plan; task 05's Docker test settles what a lane needs,
// not what husky does). Copying the setting without the directory runs nothing (review I11).
// It copies only when every one of these holds, and otherwise copies nothing and returns 0:
//   - no segment of <rel> is `.git`, compared case-insensitively after trimming the trailing
//     dots and spaces Windows drops (`.GIT`, `.git.`, `.git ` all refuse): nothing under any
//     `.git`, so `core.hooksPath=.git/hooks`, the common way to undo husky, copies nothing
//     (re-review b03 I-1; `ls-files` answers empty for such a path, so it cannot be the test);
//   - os.Lstat(<cloneTop>/<rel>) reports not-exist. The clone is safe to stat: no sandbox has
//     mounted it yet. A hooks directory tracked on the lane's `<Branch>` is already in the
//     checkout and is never overwritten; the project's own index is the wrong one to ask,
//     since it is the project's current branch, not the lane's (re-review b03 I-1);
//   - every path component from projectTop to <rel>, and <rel> itself, is a real directory by
//     os.Lstat (no symlink, no junction or other reparse point), so the copy reads nothing
//     outside the project's working tree;
//   - it holds only regular files, at most 64 and 1 MiB in total, one level deep.
// Each file is opened, then os.SameFile(its Lstat info, f.Stat()) must hold, and it is read
// through io.LimitReader of the budget that remains, so neither a file swapped between check
// and open nor one grown after the Lstat is read past the 1 MiB (review Minor 9). These guards
// hold a static tree only; a live agent swapping a parent folder for a link between check and
// read defeats any path check, which is why the caller runs this inside Mounts.WhileUnmounted
// (re-review b03 I-2, the A4 class).
// After the copy, `ls-files --others --exclude-standard -- <rel>` in the clone (Scope Clone)
// must print nothing: the folder is ignored there. Otherwise RemoveTree takes the copy back and
// it returns 0, so a user's uncommitted, unignored `.githooks/` never reaches an agent's
// `git add -A` and a land (re-review b03 Minor 3).
// Files keep their bytes. NTFS has no execute bit, so whether the sandbox's `[ -x ]` passes is
// the bind mount's answer, not the file's; task 05's `TestDispatchChainsToHusky` runs through
// the real mount and settles it (unverified until then). The host never runs them: every
// host git call overrides core.hooksPath (task-02 §HGH H1). A copy that fails after it started
// is a Create failure (step 6), so RemoveTree takes the clone with it.
func CopyGeneratedHooks(ctx context.Context, m gitx.Machine, projectTop, cloneTop, rel string) (int, error)

// RemoveTree deletes a directory tree that git wrote. On Windows git leaves object and pack
// files read-only, and os.RemoveAll fails on them (task 01's TestBehaviourRemoveAllOnAClone
// records whether it does on this machine); RemoveTree clears the read-only attribute during
// the walk and then removes. It refuses a path that is not under root.
func RemoveTree(root, path string) error
```

*Line endings (review C2).* Git for Windows' installer writes `core.autocrlf=true` into the
**system** config, and this machine has it. `git clone` copies no `core.autocrlf` into the new
repository, so a plain clone of an LF blob is CRLF on disk. The sandbox's Linux git has no such
setting, re-hashes every file (the stat data Windows wrote never matches through the bind mount),
and sees the whole tree modified; an agent's `git add -A` then rewrites every text file to CRLF.
Step 4's first four keys fix it at the root. `core.autocrlf=false` and `core.eol=lf` make the working
tree hold the blobs' bytes, and the host's git reads them from the clone's local config.
`core.checkStat=minimal` and `core.trustctime=false` stop the two systems' differing stat fields
from forcing a full re-hash on every host `status` (task 07's poll). A repository whose
`.gitattributes` asks for `eol=crlf` keeps CRLF for those paths, because attributes outrank
`core.autocrlf`. All four keys are on task-02 §HGH H3's `Clone` allowlist.

*How the sandbox's git gets the same answer (plan re-review b02 I-1).* Phase 2's `5a4d0d8`
(on `feat/phase-2-cockpit`, which merged `fix/sandbox-git-ca` at `f898f5e`) gives **every**
session, lane sessions included, `core.autocrlf` and `core.eol` as `GIT_CONFIG_KEY_n` pairs in
`sessionEnv` (`internal/api/sessions.go:215-229` at `10fa5fd`, re-read 2026-09-24, review I12).
Command scope outranks the clone's
local config, so in a lane session the sandbox's git reads the env, not these keys. The values
come from `Deps.GitLineEndings`, which `handleSessionCreate` and `handleSessionRestart` ask in the
session's worktree path (`sessions.go:651` and `:1255`, through `sessionGitLineEndings` at
`:250`), and the production resolver (`cmd/api/gitlineendings.go:36`) runs the host's
`git config` there. For a lane that path is the clone, where the local keys answer `false` and
`lf`, so the env says what the clone's config says. **The invariant is where the resolver is
asked: the clone, never `Project.Path`**, whose system `autocrlf=true` would send `true`/`crlf`.
A probe (`scratchpad/p3/rr-b02-probe.out`) showed what that mutant does: with `GIT_CONFIG_*`
holding `core.autocrlf=true` over a clone made by step 4's argv, `status --porcelain` stays empty
and `git checkout -- a.txt` writes `x\r\ny\r\n`. A second probe (`scratchpad/p3/b02r2-probe.out`,
git 2.53.0.windows.2, host only) ran the resolver's two calls (`config --get --type=bool
--default false core.autocrlf`, `config --get --default native core.eol`): `true`/`native` in
the project (the resolver turns `native` into `crlf`), `false`/`lf` in a step-4 clone, and a
checkout under the clone's answer as env writes `x\ny\n`. Task 05's
`TestALaneSessionSendsTheClonesLineEndings` and `TestTheResolverReadsALaneCloneAsLF` pin the
invariant, and task 01's `TestSandboxCloneHasWorkingGit` checks the bytes in the real sandbox.
The local keys still decide for a sandbox session started without that environment. Phase 2 and
phase 3 differ on where the answer comes from, not on the mechanism: a primary checkout is the
user's, so phase 2 reads its effective setting and sends it; a lane clone is ours to configure,
so phase 3 writes `false`/`lf` into it, and the same resolver sends that.
Verified on git 2.53.0.windows.2 on 2026-09-24, with a test system file holding
`core.autocrlf=true`: a plain `clone --local` checked `a.txt` out as `x\r\ny\r\n` and wrote no
local `core.autocrlf`. Step 4's argv checked it out as `x\ny\n`, wrote all four keys locally,
left `status --porcelain` empty, and checked a `*.bat eol=crlf` path out CRLF.

*Blob bytes on disk: symlinks, Git LFS and long paths (review Minors 13 and 15).* C2's rule
generalises: a lane's checkout holds each blob's bytes and nothing the host's config turned them
into, so the sandbox's git, which has none of that config, agrees with the host's.
- **Symlinks.** `core.symlinks=false`, always. What a clone does without it depends on the
  machine. Probed on git 2.53.0.windows.2 on 2026-09-24 (`scratchpad/p3/b08/probe2.out`): this
  machine's system config holds `core.symlinks=true`, and a clone there made a real Windows
  symlink and wrote no local `core.symlinks`. Under a system config without it (the installer's
  default) or a hermetic one, the clone wrote `core.symlinks=false` locally and checked the link
  out as a plain file holding its target's text. So the key is written either way, and a lane
  behaves the same on every machine: a repository's symlink is a small plain file in the lane,
  on the host and in the sandbox, whose content is the link's target; the index keeps mode
  `120000`, `status` is clean on both sides, and an edit to the file commits a new target. A
  tool that follows the link in the lane reads that text, not the file it names. That is the
  price, and it buys two things: the host never makes a link whose target a landed commit chose
  (an agent's link to `C:\Users\…` would otherwise be a real link in a directory the cockpit
  reads, task 16's Minor 9 class), and the answer no longer depends on whether a Windows link
  shows as a link through Docker Desktop's bind mount (unverified, so not relied on).
- **Git LFS.** Git for Windows' installer can define `filter.lfs.*` in the system config, and the
  sandbox image installs `git`, not `git-lfs` (`images/sandbox/Dockerfile:12`). A host checkout
  would then smudge LFS paths to their content while the sandbox's git, with no such filter,
  hashes that content against a pointer blob and lists every LFS path as modified: C2's shape.
  This machine has neither: its system config defines no `filter.lfs.*` and `git lfs version`
  answers "'lfs' is not a git command" (2026-09-24). Step 4's NoFilterEnv makes the answer the
  same everywhere: the checkout writes the pointer files, which are the blobs' bytes. Probed with
  a test system file holding an LFS-shaped `filter.lfs` whose smudge writes a marker
  (`scratchpad/p3/b08/probe.out`): a plain clone smudged and wrote the marker; the clone with
  NoFilterEnv wrote the pointer byte for byte, ran no filter, and a later production-shaped
  `status` was clean (that probe's clean filter was `cat`; real `git-lfs`'s clean of a pointer
  file is unverified here, since this machine has no `git-lfs`). An LFS repository's lane therefore holds pointers, not content, and the
  agent cannot fetch the content: the sandbox has no credential (A4) and no `git-lfs`. Adding
  `git-lfs` to the image is not the fix: its smudge would reach for a server from the sandbox.
  An agent that writes real content to an LFS path commits it as a plain blob, since the sandbox
  has no filter; the host's `status` of that path, which runs the system's clean filter where
  one exists, then disagrees with the sandbox's (a residual, stated, not handled in phase 3).
- **Long paths.** A clone lives at `%LOCALAPPDATA%\PanaceaApp\state\worktrees\<ProjectID>\
  <Name>-<Nonce>\` (`statepath.go:30-44`, the `worktrees` rung task 04 defaults to). On this
  machine that prefix is 77 characters up to the project id's folder, plus up to 48 for a
  40-character name and its nonce: at most 125 before the repository's own paths, well into
  Windows' 260. Without `core.longpaths=true` Git for Windows fails the checkout. Probed on
  2026-09-24 (`scratchpad/p3/b08/probe3.out`, hermetic): a 359-character path made `clone` exit
  128 with "fatal: cannot create directory at '…': Filename too long" and "warning: Clone
  succeeded, but checkout failed."; `--config core.longpaths=true` exited 0 with a clean
  `status` (task 04's `TestWorktreeCreateSurfacesGitsSentence` shows that sentence). Step 4
  writes it, so the checkout and every later host call in the clone run with it; Linux git
  ignores the key. Task 01's Step 5 records the longest real path against 260.

*The clone's origin (review Minor 14).* `git clone` sets `remote.origin.url` to `Project.Path`,
a Windows path. Linux git reads `C:/…` as scp-style SSH to a host named `C`, so an agent's
`git pull` would fail with an SSH sentence about a host that does not exist. Step 4a replaces it
with NoFetchURL and sets NoPushURL, both paths that exist on neither machine, so the sentence
names the path, and the path says who does the job. Probed on the host (`probe.out`): `fetch`
and `push` exit 128 with "fatal: '<path>' does not appear to be a git repository" (Git for
Windows prefixes its install root to a leading `/`; the host never runs either). The sandbox's
sentence is task 01's to record. `branch.<b>.remote` stays `origin`, so `status` in the sandbox
still counts ahead/behind against `refs/remotes/origin/<b>`, which task 09's refresh writes.

*Object files (review I8, plan Q9).* `clone --local` hard-links the project's object files into
the clone. A hard link is one file with two names, so the agent, through the bind mount, could
clear the read-only attribute (as `RemoveTree` does) and rewrite a pack **in the user's
repository**. OD-2's "blast radius is its own clone" is false while that holds. `--no-hardlinks`
copies the files instead. Verified on the same run: after `clone --local`, the clone's pack,
`.idx`, `.rev`, `info/packs` and `info/commit-graph` had the source's file ids, with a link count
of 2. After `--no-hardlinks`, every file id differed. Neither clone had `objects/info/alternates`.
The cost is disk and clone time, which task 01's Step 5 measures on `~/Projects/Panacea`. The
argv is built in one unexported function, `cloneArgv(branch, src, dst)`: that function is the seam
Q9's answer changes, and no configuration switches it back.

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
// primary and linked worktrees, then this project's lane clones, each with Kind "clone",
// BranchID = project.BranchID(projectID, lane.Branch) and Name = lane.Name (never the
// directory's base name, which carries the creation nonce).
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

```go
// Package briefref is what phase 3 needs from phase 4's provenance package (asks P4-1 and
// P4-3). It declares the interfaces; phase 4 satisfies them. Phase 3 never parses a brief key
// (contract P5): it passes the string to Resolve and copies it into git unchanged.
// It lives in task 03, not task 09, because tasks 09 (Resolver), 11 (Archiver) and 15 (Criterion)
// all import it and 11 runs in a lane parallel to 09 (review I7). No code, so no tests.
package briefref

import "context"

// Revision is one brief revision as phase 3 needs it.
type Revision struct {
	Key      string      // e.g. "brief:b-42@6", exactly as Resolve was given it
	Digest   string      // e.g. "sha256:…", the revision's digest (contract §1.2)
	Criteria []Criterion // in the revision's order
}

// Criterion is one criterion's identity. Digest is SHA-256 of the criterion's text as phase 4
// computes it (ask P4-2); evidence (task 15) matches on it, because the id is positional and
// breaks on insertion (risk R14).
type Criterion struct {
	ID     string
	Digest string
}

// Resolver answers a brief key. ok is false when no revision answers it ("missing" in the
// contract's six-state vocabulary); err is for a resolver that could not answer at all.
type Resolver interface {
	Resolve(ctx context.Context, key string) (rev Revision, ok bool, err error)
}

// Archiver archives a product branch's brief head when the branch is deleted (contract P13:
// "may archive the head; revisions stay"). Called by task 11; one way, phase 3 → phase 4.
type Archiver interface {
	ArchiveHead(ctx context.Context, projectID, branchID string) error
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
| `TestCreateWritesLineEndingConfig` | production-shaped (`UseSystemConfig(WindowsInstallerSystem)`), project with LF blobs `a.txt` and `b.bat` and `.gitattributes` `*.bat eol=crlf`: the clone's `config --local` holds `core.autocrlf=false`, `core.eol=lf`, `core.checkStat=minimal` and `core.trustctime=false`; `a.txt`'s bytes on disk equal its blob (no `\r`); `b.bat`'s are CRLF; `status --porcelain` is empty. Red first against a `--local` argv without the four `--config` | a clone that checks out CRLF under the system `autocrlf` (review C2) |
| `TestCreateWritesBlobBytes` (review Minors 13, 15) | production-shaped, the system file holding `core.symlinks=true` and an LFS-shaped `filter.lfs` (smudge = a marker script, `required=true`); project with a symlink entry `link` → `t.txt` (mode `120000`, via `update-index --cacheinfo`) and `a.bin` under `*.bin filter=lfs`: the clone's `config --local` holds `core.symlinks=false` and `core.longpaths=true`; `link` is a regular file by `os.Lstat` holding `t.txt`; `a.bin`'s bytes equal its blob; no marker exists; `status --porcelain` is empty. Red first twice: without `core.symlinks=false` (`link` is a symlink where the machine allows one, which this one does), and without NoFilterEnv (the marker exists) | a lane whose disk holds what the host's config made of the blobs, which the sandbox's git then sees as modified |
| `TestCreateSetsASelfDescribingOrigin` (review Minor 14) | the clone's `config --get remote.origin.url` is NoFetchURL and `remote.origin.pushurl` is NoPushURL; `branch.<b>.remote` is `origin`; `git fetch` in the clone exits non-zero and its stderr contains NoFetchURL | an agent told about SSH to a host named `C` |
| `TestCreateSharesNoObjectFileWithTheProject` | walk the clone's `.git/objects`; for every file, the same relative path in the project is absent or not `os.SameFile`. Red first against `--local` (task 01 pins that it shares them) | an agent rewriting the user's history through a hard-linked pack (review I8) |
| `TestCreateCopiesTheProjectsIdentity` | project has local `user.name=Ada`; the clone's `git var GIT_AUTHOR_IDENT` starts `Ada <` | agents committing as the machine-wide identity in a repo that sets its own |
| `TestCreateWithoutAnIdentityStillCreates` | no identity anywhere (isolated config, `user.useConfigOnly`) → lane created, clone config has no `user.name` | failing a create for a commit-time problem |
| `TestCreateCopiesARelativeHooksPath` (review I11) | project local `core.hooksPath=.husky/_` and a `.git/hooks/pre-commit` file: the clone's `config --local --get core.hooksPath` prints `.husky/_`, and the clone's `.git/hooks` holds only what `git clone` put there (no `pre-commit`). Then a table of project values, each a fresh create: `/abs/hooks`, `C:/hooks`, `C:hooks`, `\hooks`, `~/hooks`, `../x`, `a/../../x`, `.githooks\x` → lane created, clone has no `core.hooksPath`; `.githooks` → copied. Red first against step 6 removed | a husky lane running none of the project's hooks; a host path or an escape copied into a clone |
| `TestCopyableHooksPath` | the same table as a pure function, plus `""` → false | a predicate that disagrees with Create's copy |
| `TestCreateCopiesAGeneratedHooksDir` (review I11) | Creator.Mounts a fake that runs fn (no session live; nil copies nothing); project local `core.hooksPath=.husky/_`, tracked `.husky/pre-commit`, and an ignored `.husky/_/` holding `.gitignore` (`*`), `h` and `pre-commit`: the clone's `.husky/_` holds the same three files byte-identical, `status --porcelain` in the clone prints nothing. Red first against CopyGeneratedHooks not called | a husky lane with the setting but no hooks, which runs nothing |
| `TestCopyGeneratedHooksRefuses` | table, each → 0 files and nothing written: a tracked `.githooks/` (the clone's checked-out copy is byte-unchanged); a `.githooks/` tracked on the lane's `<Branch>` only and untracked in the project's checkout (the clone's copy is byte-unchanged); `rel` = `.git/hooks`, `.GIT/hooks` and `.git./hooks` (the clone's sample hooks are byte-unchanged); `.husky` a junction (Windows) or symlink to a temp directory; a symlink file inside `.husky/_`; a subdirectory inside it; 65 files; 1 MiB + 1 byte; an untracked `.githooks/` that no `.gitignore` covers (the clone has no `.githooks/` afterwards) | a copy that reads outside the project's tree, reads `.git`, overwrites a hook the lane's branch tracks, or leaves a folder the agent's `git add -A` commits |
| `TestCopyGeneratedHooksChecksTheOpenedFile` | through the unexported package variable `openForCopy` (a test-only seam, default `os.Open`): an opener that returns a different file than the path's Lstat named → 0 files, error names the path; one that returns a file larger than its Lstat size → at most 1 MiB read in total, and the copy is refused | a swap between check and open, or an uncapped read of a file an agent wrote |
| `TestCreateCopiesNoHooksWhileTheProjectIsMounted` (re-review b03 I-2) | Mounts a fake that returns ran == false → the clone has `core.hooksPath` and no `.husky/_`, Create succeeds; Mounts nil → the same; a fake that returns true → the copy runs inside its fn (a flag set in fn is seen by the copy seam) | a copy out of a working tree a live agent can rewrite |
| `TestCreateHooksPathPassesTheCloneCheck` | a created clone whose project had `core.hooksPath=.husky/_`: a `status` with `Scope: Clone` through `gitx.Machine` is not refused (task-02 §HGH H3) | a copy the clone's own allowlist then refuses, breaking every host call in the lane |
| `TestCreateUnknownBranchTouchesNothing` | `Branch: "nope"` → ErrUnknownBranch, `<root>/<p>` has no entry | a half-made clone |
| `TestCreateNeverReusesAnExistingDirectory` | Nonce pinned to `"aaaaaa"`: a file at `<root>/<p>/ui-aaaaaa` → ErrNameTaken after 4 draws and the file's bytes are unchanged; a Nonce that answers `"aaaaaa"` then `"bbbbbb"` → the lane is created at `ui-bbbbbb` | RemoveTree on a directory we did not make |
| `TestCreateAgainAfterRemoveGivesANewID` (review Minor 7) | create `ui`, Store.Remove it and RemoveTree its Path, create `ui` again with the default Nonce → the same Name, a different Path and a different ID; `Lane.Name == "ui"` and `filepath.Base(Path)` matches `^ui-[0-9a-f]{6}$`. Red first against a Path of `<Name>` alone | a re-created lane inheriting the old lane's lands, ready flag and lane ref |
| `TestCreateCleansUpAfterAFailedClone` | a Runner that fails the clone after creating Path → Path absent | leaving a broken clone for the catalog to list |
| `TestCreateRefusesBadNames` | `""`→derived; `"../x"`, `"A"`, `"-x"`, 41 chars → ErrBadName | a name that escapes CloneRoot |
| `TestCreateDerivesTheLowestFreeName` | branch `feat/upload`, lanes `upload-1` and `upload-3` exist → `upload-2` | a derived name colliding |
| `TestRemoveTreeDeletesReadOnlyObjects` | a real clone → RemoveTree succeeds and the path is gone | `os.RemoveAll` failing on Windows' read-only packs |
| `TestRemoveTreeRefusesOutsideRoot` | `RemoveTree(root, filepath.Dir(root))` → error, nothing deleted | a lane path that escaped the root deleting the user's disk |
| `TestCatalogListsClonesAfterTheScannersWorktrees` | Source gives primary+linked; store has 2 lanes of p and 1 of q → p's list is primary, linked, then 2 clones with `Kind:"clone"`, `IsPrimary:false`, the right BranchID, and `Name` the lane's Name, not the nonce-suffixed directory name | clones missing from the rail |
| `TestCatalogPathResolvesALaneFirst` | a lane id resolves to its path without calling Source.Path | a clone unreachable by session create |
| `TestCatalogPathOfAVanishedClone` | lane dir deleted → `errors.Is(err, worktree.ErrNotADir)` | a mount of a missing directory |
| `TestParseWorktreesSetsKind` (project_test.go) | first record `Kind:"primary"`, the rest `"linked"` | a linked worktree reported as primary |

#### Steps

- [ ] **Step 1: R2's lift, if needed.** `grep -n "func WriteAtomic" control-plane/internal/statepath/*.go`.
  One hit: phase 4 did it; use it. None: move `writeAtomic` and `writeatomic_test.go` into
  `statepath` as `WriteAtomic`, change `registry`'s one call site (`filestore.go:401`, in
  `saveLocked`; the definition moves from `:460`) and the test file's five calls, run
  `go test ./control-plane/internal/registry/ ./control-plane/internal/statepath/ -count=1`, and
  commit that move **alone**, before anything else, so the lift is one reviewable commit.
- [ ] **Step 2:** write the tests; `gitxtest.NewRepo` for the git ones.
- [ ] **Step 3:** implement `ident.go`, `store.go`, `clone.go`, `catalog.go`, `briefref.go`, and
  the `Kind` field.
- [ ] **Step 4:** run and commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/lanes/ ./control-plane/internal/gitx/ -race -count=1   → PASS
go vet ./control-plane/internal/briefref/                                               → clean (no test files)
go test ./control-plane/internal/project/ -count=1                                      → PASS, TestEveryGitCommandIsReadOnly unchanged
go test ./control-plane/internal/registry/ ./control-plane/internal/statepath/ -count=1  → PASS
```

#### What this task does not claim

- Nothing serves the catalog yet (task 04), so the rail does not show clones yet.
- A lane whose product branch was deleted outside the cockpit still lists, under a branch id no
  branch has; the rail shows it nowhere. Task 11's remove clears it; nothing detects it here.
- Every clone copies the project's objects (`--no-hardlinks`, Q9), so a lane costs the size of
  the project's `.git/objects` on disk plus a copy's time. Task 01's Step 5 measures both on
  `~/Projects/Panacea`; this task does not.
- This task's line-ending tests run on the host only. That the sandbox's Linux git sees the clone
  as clean is task 01's `TestSandboxCloneHasWorkingGit`.
- A repository's symlinks and Git LFS content do not work in a lane: each is its blob's bytes (a
  link's target text, an LFS pointer). No real `git-lfs` is exercised; the LFS tests use an
  LFS-shaped filter. That a path of up to 125 characters of prefix plus the repository's own
  paths bind-mounts and works in the sandbox is not tested here.
- Step 6 copies the setting, and a generated hooks directory as the project holds it **at
  create time**. A lane made on a branch whose husky differs carries what the project had;
  nothing refreshes it later. Task 05's dispatcher prints its `panacea:` line only when the
  clone has a local `core.hooksPath` whose folder is missing: the project's `.husky/_` was
  deleted after install (`git clean -fdx`), CopyGeneratedHooks refused it (over 64 files, a
  link, a subfolder, or a live primary-worktree session), or the agent deleted it inside the
  lane. Three cases are **silent**, and from inside the clone look exactly like a project with
  no hooks: a lane made **before** the project's install step ran (husky's install is what
  writes the local `core.hooksPath`, so step 6 has nothing to copy); a hooks path that is
  absolute; and hooks that live only in `.git/hooks`, or a `core.hooksPath` naming it
  (pre-commit's shim names the host's Python). Re-creating the lane after the install, with no
  primary-worktree session live, repairs a lane made before install, a deleted folder and a
  live-session refusal. A folder refused for its shape, an absolute path and `.git/hooks` reach
  no lane however it is made (re-review b03 I-3).
