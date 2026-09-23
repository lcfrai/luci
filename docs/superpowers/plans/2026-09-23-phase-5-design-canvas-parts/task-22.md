# Task 22 — Handoff: the bundle into the worktree, and the session link

**Plan:** `../2026-09-23-phase-5-design-canvas.md` · **Depends on:** 21 · **External:** **Hub C3 — Hub plan tasks 37–38 (`localkey`), a hard dependency** (route 24 writes into a worktree and route 25 writes to the Hub as the signed-in user; without the key a sandbox can call both, review C1); **phase 3 tasks 02–03, a hard dependency for route 24** (task 02's hardened `gitx.Exec`, `task-02 §HGH`, and task 03's `lanes.Catalog` with `project.Worktree.Kind`: the worktree is an agent-writable clone, and bare host git in it is code execution on the Windows host, review I1; without phase 3 route 24 refuses, it never falls back to bare git); Hub H4 (fake until then); phase 3's 3-D6 exclude rule · **Hours:** 3

## Why

The thread carried into Develop (spec §7.6, §9.1 item 11). The approved snapshot is read **from the
Hub**, not from the canvas — so a handoff works while the canvas has moved on, and what the Develop
agent reads is byte-for-byte what was approved (each file checked against the manifest's SHA-256).
It lands in `<worktree>/.panacea/design/<canvasId>@<rev>/` (P5-G), excluded from git (phase 3's
3-D6: the handoff bundle is never committed). A session started from the design is then linked
`session → design:` in the Hub (contract §7.8), while its brief is inherited like any other
session's (phase 4's P4-A, seam S4-P5.2).

**Why writing Hub data into the worktree is allowed here.** ADR-H1 §5.4 forbids *persisting Hub
data on the machine* as a cache. The bundle is not a cache: it is an explicit render into the
worktree, at the person's request, for an agent to read — the same category as phase 4's managed
`CLAUDE.md` block (ADR-H1 §1.3). The control plane **never reads it back** and never renders from it
(P4), and it holds no brief text (there is no brief mirror), so a brief erasure is unaffected.

## Files

- `canvas/handoff/bundle.go` + test (new) — the write, pure over an `FS`
- `canvas/handoff/nofollow.go`, `nofollow_windows.go`, `nofollow_other.go` + test (new) — the
  `FS` that refuses a symlink or reparse point at any component of any path it touches (review I1)
- `control-plane/internal/designapi/handoff.go`, `links.go`, `gitexclude.go`, `handoffgit.go` + tests
  (new); `routes.go` gains three routes; `mount.go`'s `Config` gains `HandoffGit` (this task adds
  the field, as task 21 adds `Hub`)
- `control-plane/internal/designapi/handoffgit_phase3.go` + test (new) — adapts phase 3's
  `lanes.Catalog` and `gitx.Machine` to `HandoffGit`; `control-plane/cmd/api/design.go` — the one
  line that passes it (nil while phase 3 is not in the tree, and route 24 then refuses)

## Contracts

```go
// canvas/handoff/bundle.go

// BundleDir is the bundle's directory relative to the worktree root (P5-G).
func BundleDir(canvasID string, revision int) string // ".panacea/design/<canvasID>@<revision>"

// ApprovalRecord is approval.json in the bundle: the Hub's record, as the
// Develop agent needs it. Written by Materialise, never read back.
type ApprovalRecord struct {
	ApprovalID   string `json:"approvalId"`
	ApprovedAt   string `json:"approvedAt"`
	ApprovedBy   string `json:"approvedBy"` // display name
	BriefKey     string `json:"briefKey"`
	DesignKey    string `json:"designKey"`
	ReviewDigest string `json:"reviewDigest"`
}

// Fetch reads one blob; the control plane passes hubclient.Designs.GetBlob.
type Fetch func(digest string) ([]byte, error)

// Materialise writes the bundle. The Hub is shared across a team (OD-3) and
// checks nothing inside a manifest (Hub contract §7.7, hub-contract.md:903), so a manifest
// is hostile input until every check below has passed (review C2). In order:
//
//  1. Fetch the manifest by its digest; ≤ MaxFileBytes; its SHA-256 is the
//     digest; it decodes strictly (unknown fields refused) as task 21's
//     Manifest (over MaxFileBytes → ErrTooLarge; a digest or decode failure
//     → ErrTampered).
//  2–4. handoff.CheckManifest(m, int64(len(manifestBytes)), ManifestWant{
//     CanvasID: canvasID, DesignKey: rec.DesignKey, BriefKey: rec.BriefKey,
//     ReviewDigest: rec.ReviewDigest}) — task 21's paths.go, the SAME function
//     BuildSnapshot runs on its own output, so the rule the approval was built
//     under and the rule the handoff enforces cannot drift (re-review I-1).
//     It checks, in order: Kind "design-handoff" and V 1; that the manifest
//     names THIS approval — rec.DesignKey goes through doc.ParseDesignKey
//     (task 05, task-05.md:241-242), a key that fails to parse → ErrTampered,
//     and its canvas id must equal canvasID (the route's {canvasId}), which
//     ties BundleDir to the local canvas; the manifest's designKey, canvasId,
//     designRevision, reviewDigest and briefKey equal rec's and the parsed
//     key's (→ ErrTampered); every path SafePath, none twice, the artboard
//     table inside Files, content types, asset hexes (→ ErrUnsafePath); and
//     len(Files)+1 ≤ MaxFiles (500), every Size ≤ MaxFileBytes, the sum with
//     the manifest ≤ MaxSnapshotBytes (512 MiB) (→ ErrTooLarge).
//     BundleDir is built from the parsed rec.DesignKey (the Hub's record),
//     never from the manifest.
//     Steps 1–4 run BEFORE any file is fetched or anything is created on disk.
//  4a. The worktree is agent-writable (review I1), so before any blob is
//     fetched: Lstat ".panacea", ".panacea/design" and BundleDir under
//     worktree, in that order. Each may be absent (it is then created, through
//     fs); one that exists must be a real directory — a symlink, a reparse
//     point (a Windows junction included) or a file → ErrWorktreeUnsafe naming
//     that component, and nothing is fetched, created or written.
//  5. Fetch every file; its length is its declared Size and its SHA-256 its
//     declared digest, else ErrTampered.
//  6. Write each into a temporary sibling directory tmp at
//     filepath.Join(tmp, filepath.FromSlash(p)), after re-checking that
//     filepath.Rel(tmp, that) neither is absolute nor starts with ".." (the
//     same check SafePath makes, repeated at the write site); write
//     manifest.json and approval.json; then rename tmp into BundleDir —
//     replacing an existing bundle only if its manifest differs. Every
//     MkdirAll, WriteFile, ReadFile, Rename and RemoveAll of steps 4a–6 goes
//     through NoFollow(worktree, fs), so a symlink the agent plants inside tmp
//     while it is being filled is refused too (→ ErrWorktreeUnsafe).
//
// Every error writes nothing: tmp is removed and the worktree is as before.
// The route always passes NoFollow(worktreePath, OS{}); a test may pass a bare
// in-memory FS only for the rules that do not concern links.
func Materialise(fs FS, worktree, canvasID, manifestDigest string, fetch Fetch, rec ApprovalRecord) (Result, error)

// ErrWorktreeUnsafe is a path under the worktree that is not what the bundle
// write may touch: a symlink, a reparse point, or a file where a directory
// must be. It is this task's (nofollow.go), not task 21's: the snapshot is
// sound, the worktree is not. The route answers 409 conflict with
// details.reason "handoff_worktree_unsafe" and details.path (relative to the
// worktree), and writes nothing.
var ErrWorktreeUnsafe = errors.New("handoff: worktree path is a link or not a directory")

// NoFollow wraps inner so that every call first Lstats each component of its
// path from root down (root itself is the control plane's own record of the
// worktree, and is trusted). A component that exists must be a real directory
// (or, for the last component of WriteFile, ReadFile and Rename, a regular
// file); ModeSymlink or ModeIrregular (Go 1.23 reports a Windows mount point,
// a junction, as ModeIrregular: go1.23 release notes, os package — unverified
// on this host until TestNoFollow_windowsJunction runs), and on Windows a
// FILE_ATTRIBUTE_REPARSE_POINT in the Win32 attributes (nofollow_windows.go),
// → ErrWorktreeUnsafe naming the component, and inner is not called. A path
// outside root → ErrWorktreeUnsafe. Rename checks both ends. RemoveAll removes
// a link it meets, never its target (os.RemoveAll does not follow links).
//
// Residual, declared: the check and the syscall are two steps, so an agent
// racing the write between them is not excluded. Go 1.24's os.Root closes that
// window; the module stays on go 1.23 (program rule), so NoFollow is the FS
// seam an os.Root-backed FS replaces (owner question, fix batch b03).
func NoFollow(root string, inner FS) FS

// OS is the production FS: package os, nothing added.
type OS struct{}

type Result struct {
	Dir     string   `json:"dir"`     // BundleDir, "/"-separated
	Files   []string `json:"files"`   // relative to Dir, sorted, approval.json and manifest.json included
	Created bool     `json:"created"` // false when an identical bundle was already there
}

type FS interface { // os in production; a temp dir in tests; phase 8 supplies one for a pod (seam S4-P8.1's shape)
	MkdirAll(path string, perm fs.FileMode) error
	WriteFile(path string, data []byte, perm fs.FileMode) error
	ReadFile(path string) ([]byte, error)
	Rename(from, to string) error
	RemoveAll(path string) error
}

// ErrTampered, ErrUnsafePath and ErrTooLarge are task 21's (paths.go,
// snapshot.go; ErrTampered sits in paths.go because CheckManifest returns it).
// Materialise returns them wrapped, naming the offending path or limit. The route answers all
// three as 422 handoff_snapshot_invalid with details.reason
// ("tampered" | "unsafe_path" | "too_large") and writes nothing.
```

### Routes (added to task 20a's table)

| # | Method | Path | Request | `2xx` response |
|---|---|---|---|---|
| 24 | POST | `/v1/design/canvases/{canvasId}/handoffs` | `{"approvalId":"ap-…","worktreeId":"w-…"}` | `201 handoff.Result`, or `200` when unchanged |
| 25 | POST | `/v1/design/canvases/{canvasId}/session-links` | `{"approvalId":"ap-…","sessionId":"s-…"}` | `201 hubapi.SessionLink`, or `200` for a repeat (the Hub's rule, contract §7.8) |
| 26 | GET | `/v1/design/canvases/{canvasId}/session-links` | — | `200 {"items":[hubapi.SessionLink]}` (route 26 is read-only: `catalogReadOnlyRoutes`) |

**The worktree is hostile input too (review I1).** A clone's `.git` and every file in it, and the
project's `.git` whenever a primary-worktree session runs, sit inside the sandbox's read-write
mount; the agent writes them. Phase 3 found that bare host git there is *"code execution on the
Windows host, as the user, outside the microVM"* (`task-02.md:342-349`, phase 3 worktree,
`docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/`). So route 24 runs **no** git
of its own: its one git call goes through phase 3's hardened `gitx.Exec` (`task-02 §HGH`, H1–H5),
it asks git for no path, and every file it writes goes through `NoFollow`.

```go
// control-plane/internal/designapi/handoffgit.go — route 24's view of phase 3, in neutral types,
// so designapi compiles before phase 3 lands (task 20's BranchLifecycle pattern).
// handoffgit_phase3.go adapts phase 3's *lanes.Catalog (task-03.md:279-285) and gitx.Machine
// (task-02.md:200-210) to it. Config.HandoffGit nil → route 24 answers 500 internal, logged
// "phase 3 is not wired" (phase 3 task-04's gitReady sentence), and runs nothing: there is no
// bare-git fallback.
type HandoffGit interface {
	// Worktree is Catalog.Worktrees(ctx, projectID) filtered to worktreeID (its BranchID and
	// Kind), with Path from Catalog.Path (worktree.Verify'd). Unknown → worktree.ErrUnknownID.
	Worktree(ctx context.Context, projectID, worktreeID string) (HandoffWorktree, error)
	// Tracked runs `ls-files -z -- <rel>` through gitx.Machine.Git() — Class Read; Scope Clone
	// for Kind "clone", Project for Kind "primary" (task-02.md:82-90) — and returns the NUL-split
	// paths. A *gitx.UntrustedConfig comes back as *UntrustedGitConfig (key only, never the value).
	Tracked(ctx context.Context, wt HandoffWorktree, rel string) ([]string, error)
}

type HandoffWorktree struct{ ID, Path, BranchID, Kind string } // Kind: "primary" | "linked" | "clone"

type UntrustedGitConfig struct{ Key, Scope string } // Scope "clone" | "project"
func (u *UntrustedGitConfig) Error() string
```

**Handoff, in order:** the approval by id from the Hub, and its `canvasId` must be this canvas
(else `404 approval_not_found` — the Hub's code); `Config.HandoffGit` is set (else `500 internal`,
above); `HandoffGit.Worktree` resolves the worktree (`worktree_not_found`), which is **on the
canvas's branch** (else `409 conflict`: *"that worktree is on another branch"*) and of kind `clone`
or `primary` (a `linked` worktree → `409 conflict`, `details.reason: "handoff_worktree_linked"`:
*"a design can be handed to a cockpit worktree or the project's main checkout, not a linked git
worktree"* — its `.git` is a file naming a directory outside it, so there is no exclude file this
route can pin); `Lstat(<path>/.git)` is a **real directory** — not a gitfile (`gitdir: /elsewhere`,
which the agent can plant), a symlink or a reparse point (else `409 conflict`, `details.reason:
"handoff_worktree_unsafe"`, `details.path: ".git"`, and no git runs); `<path>/.git/info` and
`<path>/.git/info/exclude`, where they exist, are a real directory and a regular file (checked
through `NoFollow`, same answer); `HandoffGit.Tracked(ctx, wt, ".panacea")` is empty (else `409
handoff_path_tracked`) — under §HGH no repository hook, `core.fsmonitor`, filter or pager runs, and
H5 pins `GIT_DIR` to `<path>/.git` (`task-02.md:447-467`); an `*UntrustedGitConfig` is `409
conflict` with `details.reason` `clone_config_untrusted` or `project_config_untrusted` and
`details.key` (phase 3 task-04's reasons, `task-04.md:83-85`), nothing written; then
`Materialise(handoff.NoFollow(path, handoff.OS{}), …)`, whose `Fetch` reads each blob through `io.LimitReader(body, MaxFileBytes+1)` so a Hub
answer longer than any legal blob is refused unread (`ErrTooLarge`), whose `canvasID` is the route's
`{canvasId}`, and whose `rec` is the Hub's approval record just fetched — never a field of the
request body; any of its three snapshot errors is
`422 handoff_snapshot_invalid` and `ErrWorktreeUnsafe` is `409 conflict` (`handoff_worktree_unsafe`,
`details.path`), and nothing is written; then, if `/.panacea/` is not already a line of
`<path>/.git/info/exclude` — a path **computed**, never asked of git: `rev-parse --git-path` follows
a gitfile or a `.git/commondir` the agent can plant (review I1; phase 3's H5 layout pin,
`task-02.md:455-463`) — it is appended through the same `NoFollow` FS (a `ReadFile`, then the
old bytes plus the line written to a sibling `exclude.panacea-tmp` and `Rename`d over `exclude`, so a
hard link the agent made is replaced rather than written through; `.git/info` is made if absent). If that write is
refused, a bundle this call created is removed (`RemoveAll` of `BundleDir`, through `NoFollow`) and
the answer is `409 handoff_worktree_unsafe`, so no error leaves an unexcluded bundle. A Hub failure
mid-way writes nothing (the temporary directory is removed).

**Session link, in order:** the approval; the session from the registry, on the canvas's branch;
its `ContextRef` — the key phase 4 stamped (P6) — must be a brief key, not `""` or `none:declined`
(else `409 design_no_brief`: *"this session started without the brief, so it cannot be linked to
an approved design"*); then `LinkSession{designKey, briefKey: ContextRef, sessionId, branchRef}`.
The link records **both** keys; they may differ, and that difference is the point
(`prototype-fact-sheet.md:92`).

### The desktop's three calls (P5-N), for task 28

1. `POST …/handoffs {approvalId, worktreeId}`
2. `POST /v1/sessions {worktreeId, harness: "claude-code", openingPrompt}` — the existing route; no
   `contextRef` is sent (phase 4's P4-A ignores one)
3. `POST …/session-links {approvalId, sessionId}` — retried from the lens's *Link* action if it
   fails after 2 succeeded

## Tests first

| Test | Asserts | Kills the mutant |
|---|---|---|
| `TestMaterialise_verifies` | a fake fetch returning one altered file → `ErrTampered`, nothing under the worktree | a tampered snapshot handed to an agent |
| `TestMaterialise_hostilePaths` (table; each row a manifest with **correct** digests for every file, one `Files[].Path` replaced) | `../x`, `/x`, `C:\x`, `a\..\..\x`, `assets/../../x`, `CON.html`, `con.html`, `x.html:s`, `../../../../AppData/Roaming/Microsoft/Windows/Start Menu/Programs/Startup/x.cmd` → `errors.Is(err, ErrUnsafePath)`; **nothing written anywhere**: the worktree's parent directory (a `t.TempDir()` holding the worktree) lists exactly what it did before, no tmp sibling survives, and the fake `Fetch` was called for the manifest only | a teammate's manifest writing outside the worktree — code run at the victim's next sign-in (review C2) |
| `TestMaterialise_manifestLies` | a duplicate path; an artboard `HTML` path not in `Files`; `tokens.css` labelled `text/html`; an `assets/<hex>.png` whose bytes hash to another hex → `ErrUnsafePath`, nothing written | a second file overwriting the first; a path smuggled through the artboard table |
| `TestMaterialise_otherDesign` | a valid manifest whose `canvasId`, `designRevision`, `designKey`, `reviewDigest` or `briefKey` differs from the approval record → `ErrTampered`, nothing written; a `rec.DesignKey` that `doc.ParseDesignKey` refuses → `ErrTampered`; a manifest and record that agree but whose canvas id ≠ the `canvasID` argument → `ErrTampered` (re-review M-4) | one approval materialising another design's files; a bundle directory named for another canvas |
| `TestMaterialise_sharesCheckManifest` | a manifest `handoff.CheckManifest` refuses (each of its three sentinels, one row each, reusing task 21's `TestCheckManifest` fixtures) → Materialise returns the same sentinel, and the fake `Fetch` was called for the manifest only | Materialise keeping its own copy of the manifest rule (re-review I-1) |
| `TestMount_wrapsEveryRoute` (task 20a's test, extended) | the count of mounted design routes is now **26** (routes 24–26 added); each answers 401 `local_key_required` with no key | a new route mounted around the guard (re-review M-3) |
| `TestMaterialise_limits` | 500 files + the manifest → `ErrTooLarge`; sizes summing to `MaxSnapshotBytes` + 1 → `ErrTooLarge`, **before** any file fetch; a fetched blob longer than its declared `Size` → `ErrTampered` | a manifest that fills the disk |
| `TestHandoff_invalidSnapshot` | the route over a Hub fake holding the `../x` manifest → 422 `handoff_snapshot_invalid`, `details.reason: "unsafe_path"`; `info/exclude` untouched | the error flattened to 500, or a half-done handoff |
| `TestHandoff_localKeyRequired` | routes 24 and 25 with no `Authorization`, no `Origin` and `Host: 127.0.0.1:8730` → 401 `local_key_required`; no `Fetch`, no file, no `LinkSession` call (task 20a's harness) | a sandbox materialising files into a worktree or linking a session as the human (review C1) |
| `TestMaterialise_idempotent` | twice with one manifest → the second `Created: false`, files untouched (mtime) | a rewrite on every click |
| `TestMaterialise_replacesDifferent` | a directory at `BundleDir` whose `manifest.json` differs (a hand edit, a crash leftover) is replaced whole by the rename; no stray file from it survives | half-old, half-new bundles |
| `TestMaterialise_approvalJSON` | `approval.json` holds the record's six fields | the Develop agent unable to name what it implements |
| `TestHandoff_wrongBranch` | a worktree of another branch → 409 | a design handed to the wrong branch |
| `TestHandoff_tracked` | a fixture repo with `.panacea/x` committed → 409 `handoff_path_tracked` | the bundle overwriting tracked files |
| `TestHandoff_excludeOnce` | two handoffs → `/.panacea/` appears once in `<path>/.git/info/exclude`; `git status --porcelain` shows nothing under `.panacea` | the bundle committed (3-D6) |
| `TestNoFollow_everyComponent` (table: each op × a symlink at the first, a middle and the last component, and a file where a directory must be) | `ErrWorktreeUnsafe` naming the component; the recording inner `FS` saw no call; `Rename` refused for a link at either end; a path outside `root` refused | a check of the last component only, or of the first; a `Rename` that checks its source alone (review I1) |
| `TestNoFollow_windowsJunction` (Windows only; `cmd /c mklink /J` to a `t.TempDir()` outside the root) | `.panacea` a junction → `ErrWorktreeUnsafe`, the outside directory still empty. *Unverified until run:* that Go 1.23's `Lstat` marks a junction `ModeIrregular`; the `FILE_ATTRIBUTE_REPARSE_POINT` check is there if it does not | a guard that tests `ModeSymlink` only, which a junction does not carry |
| `TestMaterialise_symlinkedPanacea` (table: `.panacea`, `.panacea/design`, then `BundleDir` a symlink — a junction on Windows — to a `t.TempDir()` outside the worktree) | `ErrWorktreeUnsafe` naming it; the outside directory lists exactly what it did before; the fake `Fetch` was called for the manifest only (step 4a runs before step 5) | `Materialise`'s writes redirected outside the worktree (review I1) |
| `TestMaterialise_linkPlantedMidWrite` | a `Fetch` fake that, on its second blob, replaces `tmp/assets` with a symlink to a `t.TempDir()` outside → `ErrWorktreeUnsafe`; nothing in the outside directory; tmp removed | step 6's writes made through the bare `fs`, not `NoFollow` |
| `TestHandoff_gitfile` | a clone whose `.git` is a **file** `gitdir: <outside>` (a copy of a real `.git`, `t.TempDir()`) → `409 conflict`, `reason: "handoff_worktree_unsafe"`, `details.path: ".git"`; a recording `HandoffGit` saw no `Tracked` call; `<outside>/info/exclude` byte-for-byte unchanged; no `.panacea` in the worktree | the exclude path asked of git, which follows the gitfile (review I1) |
| `TestHandoff_excludeLinked` (table: `.git/info` a symlink; `.git/info/exclude` a symlink to a file outside) | `409 handoff_worktree_unsafe` naming it; the outside file unchanged; no bundle left, including when the link is planted after `Materialise` (a `Fetch` fake plants it on the last blob, so the created bundle is removed) | an append that follows a link; a bundle left unexcluded |
| `TestHandoff_linkedWorktree` | `Kind: "linked"` → `409 conflict`, `reason: "handoff_worktree_linked"`; no `Tracked` call, nothing written | a gitfile worktree handed a bundle whose exclude cannot be pinned |
| `TestHandoff_fsmonitorNeverRuns` (real `gitx.Exec` from phase 3's `NewExec(MachineHardening(t.TempDir()))`, behind `handoffgit_phase3.go`) | a clone whose `.git/config` sets `core.fsmonitor=<marker script>` → `409 conflict`, `reason: "clone_config_untrusted"`, `details.key: "core.fsmonitor"`, no marker, nothing written; a `primary` worktree with the same key (Scope `Project` admits it; §HGH H1 forces `core.fsmonitor=false`) → `201`, no marker. Shown **red** first against an adapter over a bare `exec.Command("git", "-C", dir, …)` runner. *Unverified:* that this git's `ls-files` runs `core.fsmonitor` at all; if the `primary` row stays green against the bare runner it is recorded as a pin, not a mutant killer (phase 3's `TestHGHUploadPackSourceRunsNothing` convention), and the `clone` row is the killer | host git in the agent's clone run unhardened (review I1) |
| `TestHandoff_phase3Unwired` | `Config.HandoffGit` nil → `500 internal`, the log line *"phase 3 is not wired"*; no git process started (a `PATH` whose `git` writes a marker), no `Fetch`, nothing written | a fallback to bare host git when phase 3 is absent |
| `TestHandoff_hubDown` | fake `GetBlob` → `ErrUnavailable` mid-way → 503 `upstream_unavailable`, no directory left | a partial bundle |
| `TestLink_usesStampedKey` | the Hub fake receives the session's `ContextRef`, not the approval's `briefKey` | the link recording the wrong brief |
| `TestLink_declined` | `ContextRef: "none:declined"` → 409 `design_no_brief`, no Hub call | a link with an unresolvable key (a 422 from the Hub) |
| `TestLink_repeat` | the same link twice → 201 then 200 | duplicate links |

## Steps

1. `bundle.go` tests with an in-memory `FS`, then code. 2. `nofollow.go` tests over a real
`t.TempDir()`, red first against a pass-through `FS`, then code. On Windows a directory link in a
test is a junction (`cmd /c mklink /J`, no privilege needed); a row that needs a **file** symlink
runs when `os.Symlink` succeeds (Developer Mode) and otherwise `t.Skip`s naming the privilege — the
acceptance run lists each skip, and the junction rows cover the directory case on every host.
3. `designapi` tests with a real temp git repository (the `project` package's tests show the
pattern), the Hub fake, and — for `TestHandoff_fsmonitorNeverRuns` and `TestHandoff_gitfile` — phase
3's real `gitx.Exec`. 4. Mutants, on a `$TEMP` copy: `NoFollow` checking the last component only;
`Rename` checking its source only; step 4a skipped; step 6 writing through the bare `fs`; the exclude
path from `rev-parse --git-path` through `Exec`; `Tracked` run with `Scope: Project` for a clone;
the `.git` `Lstat` skipped; `Config.HandoffGit` nil falling back to `exec.Command("git", …)`. Each
must turn a named test red. 5. Acceptance; commit.

## Acceptance

```bash
cd /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/phase-5 && go test ./canvas/handoff/... ./control-plane/internal/designapi/... -race -count=1
```
Expected: two `ok` lines.

## Not claimed

- Worktree creation from a design is phase 3's (S3-P5.1, seam S5-P3.1); this task writes into a
  worktree that exists.
- Whether a Develop agent builds the screen well from the bundle is task 30's observation.
- That the bundle write cannot be raced. `NoFollow` checks, then calls; an agent that swaps a
  directory for a link between the two is not excluded (declared in `NoFollow`'s contract). Go
  1.24's `os.Root` closes it; the go directive is 1.23 (program rule), so this is fix batch b03's
  owner question, and `NoFollow` is the seam the replacement plugs into.
- That a container-made symlink resolves on the Windows host at all (review I1's *unverified*).
  The guard refuses it either way.
- A handoff into a `linked` worktree. It is refused, not built: its git directory is outside the
  worktree, and phase 3's H5 does not pin it (`task-02.md:465-467`).
