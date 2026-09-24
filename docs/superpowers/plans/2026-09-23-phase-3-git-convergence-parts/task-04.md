### Task 04: Create a worktree over HTTP — the route, `kind` on the wire, configuration, and wiring

After this task a `POST` makes a clone the rail lists under its branch, and a session started on
it gets a working git (the fix's environment plus a `.git` directory). It is the first task that
edits the control plane's shared files, so it also sets up the structure every later task
appends to: **one** `Deps` field (`Git *GitDeps`), a phase 3 file for the rest
(`api/gitdeps.go`), and **one** registration line (`a.registerGit(guarded)`) with a phase 3 route
table (`api/gitroutes.go`). After this task, later tasks edit `gitdeps.go` and `gitroutes.go`,
not `api.go` (dependency-graph §4.2's "marked, contiguous block" rule, made structural).

**Gate G1 holds before this task starts**, with phase 2's line-ending round in (plan "Gate G1";
re-review b03 I-4: the older `func sessionEnv` and `GitIdentity func` greps match phase 3's base
without that round, so they stopped nothing):
`grep -c 'func sessionEnv(id GitIdentity, eol GitLineEndings' control-plane/internal/api/sessions.go`
is 1 and `grep -c '{"core.eol", eol.EOL}' control-plane/internal/api/sessions.go` is 1 (task 05's
Step 1 greps, the same two). If either is 0, stop and report. This task edits
`handleSessionCreate` and `handleSessionRestart` (the mount guard below), which those rounds
also changed, so it must start on them.

**Files:**
- `control-plane/internal/api/api.go` — `Deps.Git *GitDeps`; `a.registerGit(guarded)` (p2, two lines)
- `control-plane/internal/api/gitdeps.go` — `GitDeps`, the phase 3 problem helpers (new)
- `control-plane/internal/api/gitroutes.go` — `gitRoutes()`, `registerGit` (new)
- `control-plane/internal/api/worktrees.go` + `worktrees_test.go` — the create handler (new)
- `control-plane/internal/api/catalog.go` — `worktreeJSON.Kind` (p2, one field, one assignment)
- `control-plane/internal/api/mounts.go` + `mounts_test.go` — `MountGuard`, task 03's `lanes.MountGuard` (new; re-review b03 I-2)
- `control-plane/internal/api/sessions.go` — the Store write that makes a session starting runs inside `MountGuard.Admit`, in create and in restart (p2, two call sites)
- `control-plane/internal/httpx/problem.go` — `CodeGitFailed`, `CodeHostFailed` (p2, two constants)
- `control-plane/cmd/api/config.go` — `PANACEA_APP_WORKTREES_DIR` (p2)
- `control-plane/cmd/api/main.go` — the phase 3 wiring block (p2)
- `control-plane/cmd/api/gitidentity.go` — `parseGitIdent` replaced by `gitx.ParseIdent` (the fix's file)
- `docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md` — §3.1 pointer row, §3.2 codes, §3.7 `kind`, §4 one row

#### Interfaces — complete

```go
// api.go — the only two lines this task adds to that file.
type Deps struct {
	// … every existing field, unchanged …

	// Git is everything phase 3's routes need (api/gitdeps.go). Nil in a Deps a phase 2 test
	// builds; cmd/api always sets it, and TestServeWiresGit proves that.
	Git *GitDeps
}

// in New, beside the other register calls:
	a.registerGit(guarded)
```

```go
// gitdeps.go
package api

// GitDeps grows one contiguous, commented block per phase 3 task. Never interleave.
type GitDeps struct {
	// ── task 04 ──
	Machine gitx.Machine
	Lanes   lanes.Store
	Creator *lanes.Creator
	// VersionErr is gitx.CheckVersion's startup error, or nil. Non-nil makes every phase 3
	// route that runs git answer 409 conflict, reason git_too_old, with its sentence.
	VersionErr error
	// Mounts is the one MountGuard. main.go gives the same value to Creator.Mounts; session
	// create and restart admit through it. Deps.Git == nil admits without a lock: no lane can
	// be created then, so there is no copy to order against.
	Mounts *MountGuard
	// Locks is the one gitx.Locks (task-02; review I7). Task 04 only constructs it; task 09's
	// Lander (and task 10's Undo and Revert), task 11's branch verbs and task 12's Fetcher and
	// Push take this value, never one of their own, so a push and a land on one project wait on
	// the same lock whichever lane merged first.
	Locks *gitx.Locks
}
```

```go
// mounts.go
package api

// MountGuard is task 03's lanes.MountGuard over the session registry (re-review b03 I-2).
// Session create has no lock of its own to share (sessions.go at 8236ba1 serialises only
// restarts, per id, in sessionIDSet), so this is that lock. One mutex orders two things:
//   - Admit(dir, fn) runs fn, the Store write that makes a session on dir StatusStarting
//     before its sandbox starts (create's Store.Create, restart's Store.Update; at 10fa5fd
//     sessions.go:689 and :1316, each before its Backend.Create at :729 and :1340);
//   - WhileUnmounted(ctx, dir, fn) lists the registry (Store.List) and runs fn only when no
//     session whose Status is not registry.StatusExited resolves, through Worktrees.Path, to
//     dir. A List or Path error is logged and counts as mounted: fail closed. err is fn's.
// Paths compare after filepath.Clean, and case-folded on Windows. So a lane's hooks copy out
// of a project's working tree and a sandbox mounting that tree never overlap.
type MountGuard struct{ /* mu sync.Mutex; store registry.Store; paths worktree.Provider; log */ }

func NewMountGuard(store registry.Store, paths worktree.Provider, log *slog.Logger) *MountGuard
func (g *MountGuard) Admit(dir string, fn func() error) error // nil g runs fn unlocked
func (g *MountGuard) WhileUnmounted(ctx context.Context, dir string, fn func() error) (bool, error)

// The closed list of details.reason on a phase 3 409 conflict. The desktop switches on it;
// it never reads the message to decide anything.
const (
	ReasonNameTaken         = "name_taken"          // POST worktrees
	ReasonGitTooOld         = "git_too_old"         // any phase 3 route that runs git
	ReasonBranchCheckedOut  = "branch_checked_out"  // lands, bring-in, undo, revert, rename, delete; details.path
	ReasonBranchMoved       = "branch_moved"        // lands, bring-in, undo, revert; details.tip
	ReasonUpstreamMoved     = "upstream_moved"      // bring-in (task 09; review I5); details.upstream
	ReasonMergeConflict     = "merge_conflict"      // lands, bring-in, revert; details.paths (detailList)
	ReasonNothingToLand     = "nothing_to_land"     // lands, bring-in
	ReasonNotReady          = "not_ready"           // lands
	ReasonBriefUnresolvable = "brief_unresolvable"  // lands, evidence; details.why: no_resolver | missing | digest_mismatch | no_such_criterion
	ReasonNoGitIdentity     = "no_git_identity"     // lands, bring-in, revert
	ReasonUndoNotExact      = "undo_not_exact"      // undo, revert; details.why: pushed | built_upon | not_a_land | not_the_tip
	ReasonLiveSessions      = "live_sessions"       // DELETE worktree; details.sessions — the desktop kills them first
	ReasonConfirmRequired   = "confirm_required"    // DELETE worktree; details.paths, .commits, .confirm
	ReasonConfirmStale      = "confirm_stale"       // DELETE worktree: the state changed since the question; fresh details
	ReasonNotCockpit        = "not_a_cockpit_worktree" // DELETE worktree of kind primary or linked
	ReasonBranchHasLanes    = "branch_has_worktrees"   // PATCH and DELETE branch; details.worktrees
	ReasonBranchExists      = "branch_exists"          // POST branches, PATCH branch
	ReasonNoRemote          = "no_remote"              // fetch, push, PR: the project has no origin; bring-in: the branch has no upstream
	ReasonPushRejected      = "push_rejected"          // push: the remote has commits the branch lacks; details.gitSays
	ReasonUnknownHost       = "unknown_host"           // PR: origin is neither GitHub nor Azure DevOps
	ReasonNoCredential      = "no_credential"          // PR; details.host
	ReasonNotPushed         = "not_pushed"             // PR: the branch has no upstream, or commits the remote lacks; details.why: never_pushed | ahead
	ReasonTranscript        = "transcript_unreadable"  // transcript span; details.why is the adapter's Unreadable.Reason (task 17)
	// task-02 §HGH H3: a *gitx.UntrustedConfig, on any phase 3 route that runs git; details.key, never the value
	ReasonCloneConfigUntrusted   = "clone_config_untrusted"   // a key outside the clone allowlist
	ReasonProjectConfigUntrusted = "project_config_untrusted" // a key on the project refusal list
)

// gitConflict writes 409 conflict with details.reason and any further details.
func gitConflict(w http.ResponseWriter, reason, msg string, details map[string]string)

// detailList encodes a list as ONE details value: the JSON array json.Marshal(xs) makes,
// "[]" for none (review I2). Phase 2's problem envelope keeps details a map[string]string
// (httpx.WriteProblem), so a list cannot be a []string there, and a newline-joined value
// cannot carry a path that holds a newline (task 07's TestPathsWithNewlinesAndSpaces). Every
// details.paths goes through it: merge_conflict (tasks 09, 10; the land and the bring-in) and
// confirm_required (task 11).
// The desktop reads it with JSON.parse (task 18's detailList in api/convergence.ts). The other list-valued details
// keep their line formats: commits and elsewhere hold shas and ref names, and git refuses a
// control character in a ref name (check-ref-format).
func detailList(xs []string) string

// gitFailed writes git_failed: 502 when the call was gitx.Remote, else 500, with
// details.gitSays = (*gitx.Error).Sentence(). msg is the control plane's sentence around it.
// A non-*gitx.Error falls through to a.internal.
func (a *API) gitFailed(w http.ResponseWriter, r *http.Request, op, msg string, err error)

// gitReady reports whether phase 3 is wired and the host git is new enough, writing the
// answer when it is not: 500 internal (logged "phase 3 is not wired") for a nil a.deps.Git,
// 409 git_too_old for a VersionErr.
func (a *API) gitReady(w http.ResponseWriter, r *http.Request) bool
```

```go
// httpx/problem.go — two constants, appended to the existing block.
	CodeGitFailed  = "git_failed"  // 500/502 a git process the control plane ran failed; details.gitSays
	CodeHostFailed = "host_failed" // 502 the GitHub or Azure DevOps API refused; details.hostSays, details.status
```

```go
// gitroutes.go
package api

// gitRoutes is phase 3's route table. Each task appends its rows under its own comment.
func (a *API) gitRoutes() []route {
	return []route{
		// ── task 04 ──
		{"POST /v1/projects/{projectId}/worktrees", http.HandlerFunc(a.handleWorktreeCreate)},
	}
}

func (a *API) registerGit(mux *routes) {
	for _, rt := range a.gitRoutes() {
		mux.Handle(rt.Pattern, rt.Handler)
	}
}
```

**Wire: `POST /v1/projects/{projectId}/worktrees`**

```json
// request
{ "branchId": "b-8c1d0e42-evidence-upload-1a2b3c4d", "name": "upload-api" }
// "name" is optional; absent or "" derives one (task 03)

// 201 Created — exactly one element of GET …/worktrees, so the rail needs no second shape
{
  "id": "w-4a77b1c9",
  "projectId": "p-8c1d0e42",
  "branchId": "b-8c1d0e42-evidence-upload-1a2b3c4d",
  "name": "upload-api",
  "path": "C:\\Users\\…\\PanaceaApp\\state\\worktrees\\p-8c1d0e42\\upload-api",
  "isPrimary": false,
  "kind": "clone",
  "sessionCount": 0,
  "ready": false
}
```

| Refusal | Status, code | details |
|---|---|---|
| body not JSON, unknown field | 400 `invalid_json` | — (httpx.DecodeJSON) |
| `branchId` missing, or names no branch of the project | 400 `bad_request` | — |
| `name` fails `lanes.ErrBadName` | 400 `bad_request` | message is `ErrBadName`'s rule |
| unknown project | 404 `project_not_found` | `projectId` |
| name taken or directory exists | 409 `conflict` | `reason: name_taken` |
| host git older than 2.40 | 409 `conflict` | `reason: git_too_old` |
| the clone failed | 500 `git_failed` | `gitSays` |

```go
// catalog.go — worktreeJSON gains one field, filled from project.Worktree.Kind.
	Kind string `json:"kind"` // "primary" | "linked" | "clone"
```

```go
// cmd/api/config.go — one row. Default statepath.Dir("worktrees").
	envWorktreesDir = "PANACEA_APP_WORKTREES_DIR"
// loadConfig refuses, each with its own sentence: a relative path; a path inside
// PANACEA_APP_PROJECTS_DIR, or one containing it (compared cleaned and, on Windows,
// case-insensitively) — the scanner would list every clone as a project; and a path that is
// an existing file. The two may sit on different volumes: task 03 copies objects on every
// clone (`--no-hardlinks`, plan Q9), so no hard link needs one volume and nothing warns.
```

```go
// cmd/api/main.go — the phase 3 block, after the scanner and before api.New.
	// ── phase 3 task 04 ──
	// task-02 §HGH: an empty hooks directory and the machine's own helpers and proxy.
	hardening, err := gitx.MachineHardening(ctx, statepath.Dir("git"))
	if err != nil {
		return fmt.Errorf("prepare host git hardening: %w", err)
	}
	gitExec, err := gitx.NewExec(hardening)
	if err != nil {
		return fmt.Errorf("prepare host git hardening: %w", err)
	}
	gitVersion, gitVersionErr := gitx.CheckVersion(ctx, gitExec)
	if gitVersionErr != nil {
		log.Warn("phase 3's git features are off", "err", gitVersionErr)
	}
	laneStore, err := lanes.OpenFileStore(statepath.Dir("lanes"))
	if err != nil {
		return fmt.Errorf("open the lane index: %w", err)
	}
	if err := os.MkdirAll(cfg.WorktreesDir, 0o700); err != nil {
		return fmt.Errorf("create the worktrees directory: %w", err)
	}
	machine := gitx.Local{Runner: gitExec, Root: cfg.WorktreesDir}
	if cfg.ProjectsDir != "" {
		catalog := &lanes.Catalog{Source: scanner, Store: laneStore}
		enumerator, worktrees = catalog, catalog // both Deps.Projects and Deps.Worktrees
	}
	// The registry store and logger main.go already passes as Deps.Store and Deps.Log; the
	// catalog above as the path resolver, so both sides resolve a worktree the same way.
	mounts := api.NewMountGuard(store, worktrees, log)
	gitDeps := &api.GitDeps{
		Machine:    machine,
		Lanes:      laneStore,
		Creator:    &lanes.Creator{Machine: machine, Store: laneStore, Mounts: mounts},
		VersionErr: gitVersionErr,
		Mounts:     mounts,
		Locks:      &gitx.Locks{}, // the only one; later blocks read gitDeps.Locks
	}
	// and in the startup log line: "git", gitVersion, "worktrees", cfg.WorktreesDir
```

The handler resolves the project and branch through `a.deps.Projects` (now the catalog), maps
`lanes` errors onto the table above with `errors.Is`, and answers with the new worktree built the
way `decoratedWorktrees` builds one (`sessionCount` 0, `ready` from the flag snapshot, which is
false for a new id).

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestWorktreeCreateReturnsTheListShape` | 201; the body decodes into `worktreeJSON` with `kind:"clone"`; a following `GET …/worktrees` holds an element **deep-equal** to it | a create response the rail cannot merge |
| `TestWorktreeCreateResolvesTheBranchByID` | a `branchId` naming no branch → 400; a real one → `lanes.CreateRequest.Branch` is that branch's `Name` | cloning a name the client made up |
| `TestWorktreeCreateMapsNameTaken` | a Creator returning `lanes.ErrNameTaken` → 409, `details.reason == "name_taken"` | a 500 for a user's naming clash |
| `TestWorktreeCreateSurfacesGitsSentence` | Creator returns a `*gitx.Error` with stderr `fatal: Filename too long` → 500 `git_failed`, `details.gitSays` is that line | hiding the one sentence the user can act on |
| `TestDetailListRoundTripsAnyPath` (review I2) | `detailList(["a b\nc.txt","docs/résumé.md","q\"x"])` decodes through `json.Unmarshal` into the same three strings; `detailList(nil) == "[]"` | a newline-joined `details.paths`, which splits `a b\nc.txt` into two paths |
| `TestWorktreeCreateWithOldGitIsAConflict` | `VersionErr` set → 409 `git_too_old`, Creator never called | cloning on a git that cannot land |
| `TestWorktreeCreateWithoutGitDepsIs500` | `Deps.Git == nil` → 500 `internal`, and the log line names phase 3 wiring | a nil-pointer panic |
| `TestCatalogWorktreeJSONCarriesKind` | GET list over a fake enumerator returning each Kind → the three strings on the wire | the field dropped at the JSON boundary |
| `TestRegisteredRoutesIncludeWorktreeCreate` | `RegisteredPatterns()` contains the POST, and the existing §10a audit still passes | a route registered on the raw mux |
| `TestLoadConfigWorktreesDir` (cmd/api) | default is `statepath.Dir("worktrees")`; relative → error; inside projects dir → error naming both variables; containing it → error | clones scanned as projects |
| `TestServeWiresGit` (cmd/api) | the Deps `serve` builds has a non-nil `Git` whose Creator's Machine root is `cfg.WorktreesDir`, and whose `Creator.Mounts` is non-nil and the same pointer as `Git.Mounts`; `Git.Locks` is non-nil | a production build with phase 3 unwired, or two guards that order nothing |
| `TestMountGuardRefusesALiveSession` (re-review b03 I-2) | a registry session on `w-1`, which the fake Provider resolves to `dir`: `StatusStarting` and running → `WhileUnmounted` returns false and fn never runs; `StatusExited` → true, fn ran; `dir` in other case on Windows → still false | a copy out of a tree a live sandbox can rewrite |
| `TestMountGuardOrdersAdmitAfterTheCopy` | fn blocks on a channel; a concurrent `Admit(dir, …)` has not run its fn when the channel is released, and runs it after (an ordered event log, under `-race`) | two locks, or none, so a sandbox starts mid-copy |
| `TestMountGuardFailsClosed` | `Store.List` errors, or `Path` errors for a live session → false, fn never runs, one log line | a registry glitch read as "nothing mounted" |
| `TestSessionCreateAndRestartAdmitThroughTheGuard` | with a `WhileUnmounted` fn held open on the primary worktree's path, a session create and a restart on it do not reach `Backend.Create` until it is released; with `Deps.Git == nil`, create still works | a session start that bypasses the guard |
| `TestGitIdentityUsesGitxParseIdent` (cmd/api) | the fix's `gitidentity_test.go` cases still pass after the swap | two parsers of one line drifting |

#### Steps

- [ ] **Step 1: Preflight** — G1's two greps (the line-ending round's, above); `grep -c "a.registerGit" control-plane/internal/api/api.go` is 0.
  G1's contract diff (review I12): `git diff 3303fd6 HEAD -- docs/superpowers/contracts/2026-09-18-phase-2-go-contract.md`,
  and read every hunk in §3.1 and §3.2 before appending to them; name any in the task report.
- [ ] **Step 2:** tests first. `worktrees_test.go` builds `New(Deps{…, Git: &GitDeps{…}})` with a
  fake `lanes.Store` and a `Creator` over `gitxtest` repos for the one real-git case.
- [ ] **Step 3:** `gitdeps.go`, `gitroutes.go`, `worktrees.go`, the two `api.go` lines, the
  `catalog.go` field, the two codes; `mounts.go`, and the two `Admit` call sites in
  `sessions.go` (re-locate them with `grep -n "a.deps.Store.Create\|StatusStarting"`, since the
  line numbers above are `10fa5fd`'s).
- [ ] **Step 4:** config and `main.go`; swap `parseGitIdent` for `gitx.ParseIdent` and delete it.
- [ ] **Step 5:** amend the Go contract: §3.1 gains one row, *"Phase 3 routes: see
  `plans/2026-09-23-phase-3-git-convergence.md`, The HTTP surface"*; §3.2 gains `git_failed` and
  `host_failed`; §3.7's worktree object gains `kind` with the three values and the sentence from
  `project.Worktree.Kind`'s comment; §4 gains `PANACEA_APP_WORKTREES_DIR`.
- [ ] **Step 6:** run; commit by pathspec.

#### Acceptance

```
go test ./control-plane/... -race -count=1                                     → PASS
curl -s -X POST http://127.0.0.1:8730/v1/projects/<p>/worktrees -H "Content-Type: application/json" \
     -d "{\"branchId\":\"<b>\"}"                                               → 201, "kind":"clone"
curl -s http://127.0.0.1:8730/v1/projects/<p>/worktrees                        → the same object in the list
```

The two `curl` lines are a manual check on the owner's laptop with a real project; the report
records their output or says they were not run.

#### What this task does not claim

- A session on the new worktree works in git (fix environment plus a `.git` directory) — that is
  **inferred** from task 01's `TestSandboxCloneHasWorkingGit`, and task 05's Docker test is the
  one that runs it through the real session create.
- No UI creates a worktree yet (task 22).
- Nothing removes one yet (task 11).
