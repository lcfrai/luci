### Task 02: `gitx` — the write runner, its timeout classes, git's own sentence, the `Machine` seam

Phase 2's `project.ExecRunner` is *"the only place in the product that starts a git process"*
(`gitrunner.go:19-20`), and it is right for reads behind a 2 s poll: stdout only, headless, ten
seconds, `GIT_OPTIONAL_LOCKS=0`. Two of those are wrong for phase 3 (survey §1.2): a merge,
`update-ref` or clone must take its locks, and ten seconds is wrong for a clone of a large
repository or a push through a corporate proxy. This task builds the runner every phase 3 package
uses, copying `ExecRunner`'s discipline and changing exactly those two things per call class. It
also declares the one seam phase 8 needs (brief A6) and the trailer writer every land uses.

`project.ExecRunner` is **not** edited, and phase 2's `project.Runner` stays the read path for
enumeration. No host git call runs in a lane clone (ruling R4.2). Lane git runs in the sandbox
through task 24's runner, and phase 2's resolvers read host-owned config only (R4.6, the phase-2
stream's fix). After this task the product has two process launchers; the comment on
`ExecRunner` that calls itself "the only place" is amended in one line to name this one too.

**Files:**
- `control-plane/internal/gitx/runner.go` — `Class`, `Call`, `Result`, `Runner`, `Exec` (new)
- `control-plane/internal/gitx/harden.go` — `Scope`, `Hardening`, the §HGH prefix, the H3 config check, the lane-dir guard (`ErrLaneDir`), `UntrustedConfig` (new)
- `control-plane/internal/gitx/error.go` — `*Error`, `Sentence`, `AsError` (new)
- `control-plane/internal/gitx/version.go` — `CheckVersion` (new)
- `control-plane/internal/gitx/machine.go` — `Machine`, `Local` (new)
- `control-plane/internal/gitx/trailers.go` — trailer keys, `Message`, `TrailerFormat` (new)
- `control-plane/internal/gitx/locks.go` — `Locks`, the one per-project lock that land, undo,
  revert, the branch verbs, fetch and push all take, and every clone refresh runs under (review
  I7, I4) (new)
- `control-plane/internal/gitx/internal/spawnseam/seam.go` — `Spawn`, `Set`: the process seam
  only `gitx` and `gitxtest` can import (new)
- `control-plane/internal/gitx/gitxtest/repo.go` — real-git fixtures, a scripted fake and `Spy` (new)
- `control-plane/internal/gitx/*_test.go` — this task's tests (new)
- `control-plane/internal/project/gitrunner.go` — one comment line (p2)

#### Interfaces — complete

```go
// Package gitx runs git for everything phase 3 does that is not phase 2's read-only
// enumeration. project.ExecRunner stays the launcher for that; this is the other one.
package gitx

import (
	"context"
	"errors"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/internal/spawnseam"
)

// Class decides a call's timeout and whether it may take git's optional locks.
type Class int

const (
	// Read never takes an optional lock (GIT_OPTIONAL_LOCKS=0), so a status or a diff run
	// beside an agent's own git never races it for index.lock. ReadTimeout bounds it.
	Read Class = iota
	// Local writes a repository on this machine: clone, fetch between two local
	// repositories, merge-tree, commit-tree, update-ref, config. It takes its locks.
	Local
	// Remote talks to a git host over the network: fetch origin, push.
	Remote
)

const (
	ReadTimeout   = 10 * time.Second
	LocalTimeout  = 2 * time.Minute
	RemoteTimeout = 3 * time.Minute
)

// Call is one git invocation.
type Call struct {
	Dir   string
	Args  []string
	Class Class
	// Stdin, when non-nil, is git's standard input (commit-tree -F -).
	Stdin []byte
	// Env is applied after os.Environ() and after the headless defaults, so a key here wins,
	// except GIT_DIR, GIT_COMMON_DIR and GIT_WORK_TREE: Exec pins those (§HGH H5), and a Call
	// naming one is ErrCallShape.
	// A value may be a secret (task 13 passes an http.extraHeader for push): Env is never
	// logged, never formatted into an Error, and never echoed in a Result.
	Env map[string]string
	// OKExit lists non-zero exit codes that are answers rather than failures. merge-tree
	// --write-tree exits 1 on a conflict (task 01's pin), and a conflict is a forecast.
	OKExit []int
	// Scope picks §HGH H3's config check for Dir. It has no default: the zero value, Unset, is
	// ErrCallShape, so every call site names whose repository it runs in (§HGH "Call-site scopes").
	Scope Scope
	// RemoteURL is the literal URL a Remote call talks to (§HGH H2). Required for Remote.
	RemoteURL string
	// NewLane is the path task 03's lanes.Create is cloning into. Exec admits Dir == NewLane
	// only with Scope None, and only lanes.Create sets it (ruling R4.5); every other call
	// leaves it "". It is the one exception to ErrLaneDir, and it holds for that clone only.
	NewLane string
}

// Scope is whose repository Call.Dir is, which decides what §HGH H3 refuses. Every repository
// a host call may run in is host-private (ruling R4.1); a lane clone is never one (R4.2).
type Scope int

const (
	Unset   Scope = iota // the zero value: Exec refuses it with ErrCallShape and runs nothing
	Project              // the user's repository, Class Read, Local or Remote: refusal list
	None                 // `git version`, a Dir under OwnedRoot, or the clone lanes.Create is making (Call.NewLane): no check
)

// Hardening is what Exec adds to every call (§HGH). cmd/api builds it once at startup.
type Hardening struct {
	HooksDir   string   // an empty directory the control plane owns (H1)
	OwnedRoot  string   // the only root a Scope None call may run under
	Credential []string // "key=value", the machine's credential.*helper entries, system then global (H2)
	Proxy      string   // the machine's proxy; "" means none (H2)
	LaneRoot   string   // Machine.CloneRoot(): Exec refuses any call that runs in it (R4.2)
}

// MachineHardening creates the empty directory <root>/no-hooks, sets OwnedRoot to root, and reads
// H2's helpers and proxy from `git config --system`, then `--global`, run with Dir root and Scope
// None. Task 04 calls it once in main.go, with root statepath.Dir("git"), then sets LaneRoot to
// its Machine's CloneRoot(); MachineHardening leaves LaneRoot "".
func MachineHardening(ctx context.Context, root string) (Hardening, error)

// UntrustedConfig is a repository config key §HGH H3 refuses. It carries the key, never the value.
// The API maps it to conflict + reason project_config_untrusted.
type UntrustedConfig struct {
	Dir   string
	Key   string
	Scope Scope
}

func (u *UntrustedConfig) Error() string

// ErrCallShape is a Call Exec will not run: Args[0] starting with "-", Scope Unset, a Remote call
// without RemoteURL, a Scope None call outside OwnedRoot whose Dir is not its NewLane, or a
// NewLane set on a call that is not Scope None.
var ErrCallShape = errors.New("gitx: call shape refused")

// ErrLaneDir is a call Exec refuses before it starts (ruling R4.2): its Dir, a `-C`,
// `--git-dir` or `--work-tree` argument, or a Call.Env GIT_DIR, GIT_WORK_TREE, GIT_COMMON_DIR,
// GIT_OBJECT_DIRECTORY or GIT_ALTERNATE_OBJECT_DIRECTORIES value lies at or under
// Hardening.LaneRoot (either flag spelling, `=` or next argument; each list entry of the
// alternates value). Each path is made absolute against Dir and cleaned (filepath.Clean) before
// a filepath.Rel compare, which case-folds on Windows. The one exception is Dir == Call.NewLane
// on a Scope None call. Exec checks it before ErrCallShape, so a lane path in a pinned Env key
// is ErrLaneDir. A positional argument (`fetch <path>`) is not parsed here; task 24's
// TestHostNeverRunsGitInALane pins that no call site passes one. A LaneRoot of "" checks
// nothing. Lane git runs in the sandbox, through task 24's runner, instead.
var ErrLaneDir = errors.New("gitx: host git never runs in a lane")

// Result is a call that exited 0 or with a code in OKExit.
type Result struct {
	// Stdout only. git writes warnings to stderr and exits 0; a parser fed both grows a
	// branch named "warning: ignoring broken ref …" (gitrunner.go:32-45).
	Stdout []byte
	// Stderr is kept for the callers that read git's informational output on success.
	Stderr   []byte
	ExitCode int
}

// Runner runs git on one machine.
type Runner interface {
	Run(ctx context.Context, c Call) (Result, error)
}

// Exec is the production Runner.
type Exec struct {
	// Git is the executable; "" means "git" on PATH.
	Git string
	// Timeouts overrides the per-class defaults; a missing or zero entry uses the default.
	Timeouts map[Class]time.Duration
	// Hardening is §HGH's prefix. An Exec whose HooksDir is "" runs nothing (ErrCallShape).
	Hardening Hardening
	// spawn is the process seam: nil in production. When set, Run hands it the child's final
	// argv, Dir and environment just before cmd.Start, for every process it starts (the H3
	// config read included) and for none it refuses. Only gitxtest.Spy sets it, through
	// internal/spawnseam, which no package outside gitx can import (Go's internal rule).
	spawn func(spawnseam.Spawn)
}

var _ Runner = (*Exec)(nil)

// NewExec checks that h.HooksDir is an existing, empty directory, then returns the Exec.
func NewExec(h Hardening) (*Exec, error)

// Run refuses a lane call with ErrLaneDir and a bad shape with ErrCallShape, then applies §HGH
// (the H1 and H2 prefix, then the H3 config check for c.Scope, which refuses with
// *UntrustedConfig before git starts), then starts git headless — GIT_TERMINAL_PROMPT=0,
// GCM_INTERACTIVE=never, GIT_PAGER=cat, GIT_EDITOR=false (a call that wants an editor fails
// rather than hangs), plus GIT_OPTIONAL_LOCKS=0 for Read only — captures stdout and stderr
// separately, and returns an *Error for a start failure, a timeout, or an exit code outside
// {0} ∪ OKExit.
func (x *Exec) Run(ctx context.Context, c Call) (Result, error)

// MaxSentence caps what the cockpit is shown of git's stderr.
const MaxSentence = 2048

// Error is a git call that did not answer.
type Error struct {
	Args     []string // the argv after "git"; Env is never here
	Dir      string
	ExitCode int      // -1 when git did not start or the timeout killed it
	Stderr   string   // trimmed, capped at MaxSentence
	TimedOut bool
	Err      error
}

func (e *Error) Error() string // "git <args> in <dir>: <exit or timeout>: <Sentence()>"
func (e *Error) Unwrap() error { return e.Err }

// Sentence is git's own words: the last stderr line starting "fatal:", "error:" or
// "remote:", else the last non-empty stderr line, else "git exited <code>". Capped at
// MaxSentence bytes on a UTF-8 boundary. The cockpit renders it verbatim (details.gitSays).
func (e *Error) Sentence() string

// AsError is errors.As for *Error.
func AsError(err error) (*Error, bool) {
	var ge *Error
	ok := errors.As(err, &ge)
	return ge, ok
}

// MinVersion is the oldest host git phase 3 runs on: merge-tree --write-tree needs 2.38 and
// merge-tree --merge-base (revert, task 10) needs 2.40.
const MinVersion = "2.40.0"

// CheckVersion runs `git version`, parses "git version 2.53.0.windows.2", and returns the
// version string, or an error whose message is the sentence the user acts on.
func CheckVersion(ctx context.Context, r Runner) (string, error)
```

```go
package gitx

// Machine is where git runs for one machine id (brief A6). Phase 3 has one, Local. Phase 8
// adds a Kata machine behind the same three methods: phase 8's machine serves the runner
// (P8-D7), and lane work crosses to the control plane as a bundle, which is data (ruling
// R4.4), the same model phase 3 builds with task 24's runner.
type Machine interface {
	// ID is the machine id worktrees and sessions carry: api.LocalMachineID ("local") here.
	ID() string
	// Git runs git on this machine, in host-private repositories only (ruling R4.2).
	Git() Runner
	// CloneRoot is the absolute directory this machine's lane clones are created under.
	// Task 04 copies it into Hardening.LaneRoot, so Git() refuses every call in it.
	CloneRoot() string
}

// Local is this machine.
type Local struct {
	Runner Runner
	Root   string // absolute; cmd/api validates it (task 04)
}

func (l Local) ID() string        { return "local" }
func (l Local) Git() Runner       { return l.Runner }
func (l Local) CloneRoot() string { return l.Root }
```

```go
package gitx

// Locks is the ONE lock scope for the project repository's branches (review I7). Every call that
// moves a product branch, or reads one to publish it, holds its project's lock for the whole
// verb: Land and BringIn (task 09), Undo and Revert (task 10), CreateBranch, RenameBranch and
// DeleteBranch (task 11), Fetch and Push (task 12, and task 13's push through it). A push that
// read a tip a land was moving would publish whichever tip it happened to see. The clone
// refresh's other callers take it too (review I4): RefreshBranch, the refresh route (task 09),
// and the lane syncer's tip-change sweep (task 07), which never waits for it: it calls TryLock,
// and a busy project is retried on the next sweep (re-review b06-1 issue 6).
//
// One per process. Task 04 constructs it in cmd/api and hands the same value to every user
// through GitDeps.Locks, so no task builds its own. The key is the project ID, phase 2's
// "p-" + hash of the catalog path (project.go:29, :102). Every caller takes it from the one catalog
// entry, which has one spelling, whereas a path handed in by each caller could be spelled several
// ways on Windows (re-review b03 N-1). The ID does not normalise those spellings itself: hashPath
// only makes the path absolute, cleans it and lowercases it (project.go:86-97), so an 8.3 or
// junction spelling still hashes differently (re-review b04-1 issue 5). The case that remains is
// two catalog entries for one repository, which get two locks; compare-and-swap update-ref still
// refuses the double land, and the loser gets branch_moved (task 09).
//
// Lock order: a Locks lock is taken first. Syncer.SyncLane's per-lane lock (task 07) may be taken
// inside it (Land's step 4, and Undo's sync of every lane of the branch, task 10); nothing takes
// a Locks lock while holding a lane lock. The sweep
// (task 07) takes its Locks lock only after it has synced the lanes and released every lane lock.
type Locks struct {
	// unexported: mu sync.Mutex; m map[string]chan struct{} (capacity 1, one per project ID)
}

// Lock waits for projectID's lock, or for ctx. It returns ctx.Err() when ctx ends first, so a
// request the human abandoned stops waiting. unlock is idempotent. The zero Locks is ready.
func (l *Locks) Lock(ctx context.Context, projectID string) (unlock func(), err error)

// TryLock takes projectID's lock only if it is free now, and never waits: ok is false when
// another holder has it. It is a select with a default case, not Lock with a done ctx, because
// Go's select picks at random between a free lock and a done ctx (re-review b06-1 issue 6).
// The sweep (task 07) is its one caller. unlock is idempotent; it is nil when ok is false.
func (l *Locks) TryLock(projectID string) (unlock func(), ok bool)
```

```go
package gitx

// The trailer keys phase 3 writes. Readers match them exactly as written here.
const (
	TrailerSession     = "Panacea-Session"      // the hook, task 05
	TrailerLand        = "Panacea-Land"         // a land's worktree id, task 09
	TrailerBrief       = "Panacea-Brief"        // the brief key the human saw, task 09
	TrailerBriefDigest = "Panacea-Brief-Digest" // that revision's digest, task 09
	TrailerReverts     = "Panacea-Reverts"      // the land a revert takes back, task 10
	// TrailerBringIn marks task 09's merge-mode bring-in; its value is the upstream tip it
	// merged, a 40-hex sha, so Message's charset always accepts it (a branch name may not).
	TrailerBringIn = "Panacea-Bring-In"
)

// Trailer is one "Key: value" line.
type Trailer struct{ Key, Value string }

// ErrBadMessage is a subject or trailer value Message refuses.
var ErrBadMessage = errors.New("gitx: subject or trailer value is not writable")

// Message builds "subject\n\nbody\n\nKey: value\n…\n". It refuses a subject with a newline
// or over 200 bytes, and a trailer value that is empty, over 256 bytes, or outside
// [A-Za-z0-9._:@/+=-]. That check is a charset, not a grammar: a brief key is copied byte for
// byte and never split (contract P5). Its purpose is that no value can start a second
// trailer line.
func Message(subject, body string, trailers []Trailer) (string, error)

// Log formats use NUL between fields and RS between records, so a subject can hold anything.
const (
	FieldSep  = "\x00"
	RecordSep = "\x1e"
	ValueSep  = "\x1f"
)

// TrailerFormat is a --format fragment printing one trailer key's values joined by ValueSep.
func TrailerFormat(key string) string {
	return "%(trailers:key=" + key + ",valueonly,separator=%x1f)"
}
```

```go
// Package gitxtest builds throwaway repositories for tests that need real git, and a scripted
// Runner for tests that must not.
package gitxtest

// Repo is a repository in t.TempDir() with an isolated config: GIT_CONFIG_NOSYSTEM=1,
// GIT_CONFIG_GLOBAL pointing at an empty file, a fixed identity, init.defaultBranch=main.
// NewRepo is a PROJECT, so its local config also holds keys real projects carry:
// branch.main.vscode-merge-base=origin/main, pull.rebase=true, credential.useHttpPath=true.
// §HGH H3's Project refusal list admits them, so a project check tightened past that list goes
// red in its own task's tests (b01 re-review R1). There is no Scope Clone to run them under
// (ruling R4.2). CloneOf's repo holds only what git clone writes, since clone copies no local
// config from its source; it stands in for a lane clone, which host git never runs in.
// Runner()'s Hardening has LaneRoot "", so a test of ErrLaneDir sets LaneRoot itself.
type Repo struct {
	T   testing.TB
	Dir string
	Env map[string]string
}

func NewRepo(t testing.TB) *Repo
func (r *Repo) Git(args ...string) string                          // fails the test on error; trimmed stdout
func (r *Repo) Commit(msg string, files map[string]string) string  // write, add, commit; returns the sha
func (r *Repo) Runner() gitx.Runner                                 // an Exec whose calls carry r.Env; Hardening.HooksDir is an empty t.TempDir()
func CloneOf(t testing.TB, src *Repo, branch string) *Repo           // git clone --no-hardlinks --branch (task 03's I8 rule)

// UseSystemConfig makes r production-shaped (review C2): it drops GIT_CONFIG_NOSYSTEM from r.Env
// and sets GIT_CONFIG_SYSTEM to a file in t.TempDir() holding body, so the setting that breaks
// production reaches the test. Task 03's line-ending tests run with WindowsInstallerSystem,
// the system config Git for Windows' installer writes (and this machine holds).
func (r *Repo) UseSystemConfig(body string)

const WindowsInstallerSystem = "[core]\n\tautocrlf = true\n"

// HTTPSRemote starts an httptest.NewTLSServer with h and returns its URL. It writes the server's
// certificate to a file and sets http.sslCAInfo to it in r's isolated GIT_CONFIG_GLOBAL file, which
// is not repository config, so §HGH admits it. A Remote call allows https only (§HGH H1), so every
// remote a phase 3 test talks to is one of these, never a plain httptest.NewServer.
func HTTPSRemote(t testing.TB, r *Repo, h http.Handler) string

// Script is a Runner that answers from a table keyed by the joined argv and records calls.
type Script struct {
	Calls []gitx.Call
	// unexported table
}

func (s *Script) On(argv string, res gitx.Result, err error) *Script
func (s *Script) Run(ctx context.Context, c gitx.Call) (gitx.Result, error) // unknown argv fails the test

// Spawn is one process a real Exec started: Argv is cmd.Args (the git executable first, then
// §HGH's prefix, then Call.Args), Dir is cmd.Dir, Env is cmd.Env as "KEY=value".
type Spawn = spawnseam.Spawn

// Spy installs a recorder on e's process seam and returns the slice it appends to, under a
// mutex, one Spawn per process e starts; e still starts it. Read the slice after the calls
// return. t.Cleanup removes the recorder. Env can hold a push token (task 13), so gitxtest
// never logs a Spawn. Task 24's TestHostNeverRunsGitInALane is its first caller.
func Spy(t testing.TB, e *gitx.Exec) *[]Spawn
```

```go
// Package spawnseam is gitx.Exec's process seam. It is internal to gitx, so production code
// outside gitx cannot reach it, and gitxtest can.
package spawnseam

// Spawn is a child's final argv, working directory and environment.
type Spawn struct {
	Argv []string
	Dir  string
	Env  []string
}

// Set is filled by package gitx's init: it installs f as e's spawn observer (nil removes it).
// e is a *gitx.Exec; any other type panics.
var Set func(e any, f func(Spawn))
```

#### The tests to write first

`Exec` is tested against a **helper process**, the standard `os/exec` pattern: the test binary
re-executes itself as a fake `git` (`TestHelperProcess`, gated on an environment variable) that
prints its environment, sleeps, or exits with a chosen code. That is the only way to observe the
child's environment without trusting the code under test.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestExecReturnsStdoutOnlyOnSuccess` | the helper writes `warning: x` to stderr and `ok` to stdout, exits 0 → `Result.Stdout == "ok\n"` | `CombinedOutput` on the success path |
| `TestExecReadClassSetsNoOptionalLocks` | the child sees `GIT_OPTIONAL_LOCKS=0` for `Read` | a write runner that never marks reads |
| `TestExecLocalAndRemoteTakeTheirLocks` | the child does **not** see `GIT_OPTIONAL_LOCKS` for `Local` or `Remote` | copying `ExecRunner`'s environment blindly (survey §1.2) |
| `TestExecIsHeadlessForEveryClass` | `GIT_TERMINAL_PROMPT=0`, `GCM_INTERACTIVE=never`, `GIT_PAGER=cat`, `GIT_EDITOR=false` for all three classes | a class that can pop a credential window |
| `TestExecCallEnvWins` | `Env{"GIT_PAGER":"less"}` reaches the child as `less` | defaults appended after the caller's |
| `TestExecOKExitIsAnAnswer` | exit 1 with `OKExit: []int{1}` → `err == nil`, `ExitCode == 1`; exit 2 → `*Error` | treating merge-tree's conflict as a failure |
| `TestExecTimeoutIsPerClass` | `Timeouts{Read: 50ms}`, helper sleeps 1 s → `TimedOut`, `ExitCode == -1`; the same call as `Local` succeeds | one timeout for every class |
| `TestExecPassesStdin` | helper echoes stdin → stdout equals `Call.Stdin` | `commit-tree -F -` with no message |
| `TestErrorNeverContainsEnv` | `Env{"X":"s3cr3t"}` on a failing call → `"s3cr3t"` in neither `Error()` nor `Sentence()` | formatting the call into the error |
| `TestSentencePrefersTheLastFatalLine` | stderr `hint: a\nfatal: b\nhint: c` → `"fatal: b"` | showing git's hints as the cause |
| `TestSentenceIsCappedOnARuneBoundary` | 3000 bytes of `é` → ≤ 2048 bytes, valid UTF-8 | a cap that cuts a rune |
| `TestCheckVersion` | `git version 2.53.0.windows.2` → ok; `git version 2.37.1` → error containing `2.40` | a lexical compare (`"2.9" > "2.40"`) |
| `TestMessageRefusesANewlineInATrailer` | value `brief:b-1@2\nPanacea-Land: w-x` → `ErrBadMessage` | trailer injection through a key |
| `TestMessageCopiesTheValueByteForByte` | `brief:b-42@6` appears verbatim once | normalising or parsing a key (P5) |
| `TestMessageIsReadBackByGit` | real git: `git interpret-trailers --parse` on `Message(...)` prints exactly the trailers given | a missing blank line before the trailer block |
| `TestExecRefusesALaneDir` | R4.2, red first: `LaneRoot` = a `t.TempDir()` holding a `CloneOf` clone `L`; table, each on a `Scope Project` call: `Dir: L`; `Dir: L/sub`; `Dir` = `L` in other letter case (Windows only); `-C L`; `--git-dir=L/.git`; `--git-dir L/.git`; `--work-tree L`; `Env` `GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR`, `GIT_OBJECT_DIRECTORY` = a path in `L`; `GIT_ALTERNATE_OBJECT_DIRECTORIES` = `x` + list separator + `L/.git/objects`; `Dir` = `L` spelled `<LaneRoot>/x/../<L's name>` → each `errors.Is(err, ErrLaneDir)`, and `Spy` recorded nothing, so not even H3's config read ran in `L` | a guard that compares uncleaned strings, reads only `Dir`, or checks after start |
| `TestExecAdmitsNewLaneOnlyForScopeNone` | `Dir == NewLane == L`, `Scope None` → runs; the same with `Scope Project` → `ErrCallShape`; `NewLane: L`, `Dir: L2` (another clone) → `ErrLaneDir`; `NewLane` set on a `Scope Project` call in the project → `ErrCallShape` | a NewLane that opens every lane, or any scope |
| `TestSpyRecordsWhatExecStarts` | a `Read` call with `Scope Project` → one or more Spawns, the last with `Dir` = the repo, `Argv` ending in `Call.Args`, `Env` holding `GIT_OPTIONAL_LOCKS=0`; a call refused by `ErrCallShape` → no Spawn; after cleanup, a call adds none | a spy fed from `Call` instead of `cmd`, or one left installed |
| `TestLocalMachineIsLocal` | `Local{Root: r}.ID() == "local"`, `CloneRoot() == r` | a Local that rewrites paths |
| `TestScriptFailsOnAnUnscriptedCall` | an unknown argv fails the calling test | a fake that silently succeeds |
| `TestLocksSerialiseOneProject` | under `-race`: goroutine A holds `Lock(ctx, "p-1")`; B's `Lock(ctx, "p-1")` has not returned 50 ms later; A unlocks → B returns within 1 s; calling A's unlock twice does not release B's hold | a lock keyed per call, or an unlock that frees the next holder |
| `TestLocksLetOtherProjectsRun` | A holds `"p-1"`; `Lock(ctx, "p-2")` returns at once | one global mutex that stalls every project behind a slow push |
| `TestLocksLockHonoursTheContext` | A holds `"p-1"`; B's `Lock` with a context cancelled after 20 ms → `context.Canceled`, and A's later unlock leaves the lock free for C | a waiter that outlives its request, or a cancelled waiter that swallows the next release |
| `TestLocksTryLockNeverWaits` (re-review b06-1 issue 6) | 1000 times: a free `"p-1"` → `TryLock` ok, then unlock; A holds `"p-1"` → `TryLock("p-1")` returns `ok == false` within 1 ms and `TryLock("p-2")` is ok; A unlocks → `TryLock("p-1")` ok | a TryLock built on Lock with a done ctx, which fails at random on a free lock |

#### Host git hardening (HGH)

*Fix batch b01 (review C1, C3). This is the rule's only copy. Every other part cites
`task-02 §HGH` at the call site it hardens and restates none of it.*

**Why.** No sandbox mounts the project (R4.1), so its `.git` is host-private and its config is
the user's and the cockpit's. A lane clone is agent-writable, and host git never runs in one
(R4.2; task 24 runs lane git in the sandbox). So §HGH hardens project calls only. H1 and H2 stay
as defence against the user's own config and a machine's system config.

**H1. The command-scope prefix.** `Exec.Run` puts these overrides in argv, between `git` and
`Args`, on **every** call of every class. Command scope outranks repository config (task 01's
`TestBehaviourLocalConfigHooksPathIgnoresEnvironment` proves the ordering), so no key the agent
writes can undo them. They go in argv, never in `GIT_CONFIG_COUNT`, because `Call.Env` wins over
Exec's environment and task 13's `PushEnv` sets `GIT_CONFIG_COUNT` itself.

| Override | What it neutralises |
|---|---|
| `-c core.hooksPath=<Hardening.HooksDir>` | every hook in `.git/hooks` or in a repository-named `core.hooksPath`: `reference-transaction` on update-ref and fetch, `post-checkout`, and `pre-push` (see H4) |
| `-c core.fsmonitor=false` | a `core.fsmonitor=<program>` that `status` and `diff` would run |
| `-c core.sshCommand=` | a repository-named SSH program. An SSH remote fails closed, because phase 3's remotes are HTTPS (task 13) |
| `-c protocol.allow=never -c protocol.file.allow=always`, plus `-c protocol.https.allow=always` on a `Remote` call only | every transport the phase does not use, by default rather than by name: `ext::<command>`, `ssh`, `git://`, and a lazy fetch that a `Read` or `Local` call would make from a partial-clone promisor remote. A local path counts as `file` |
| `-c core.askPass=` | a repository-named askpass program. An empty value falls through to `GIT_TERMINAL_PROMPT=0`, which fails |
| `-c commit.gpgSign=false -c tag.gpgSign=false` | a repository `gpg.program` run by task 09's and task 10's `commit-tree` |
| `-c submodule.recurse=false -c fetch.recurseSubmodules=false` | a host fetch that recurses into `.git/modules/*`, whose config the agent also writes |
| `-c credential.helper=`; then, on a `Remote` call or a `Scope None` call only, each `Hardening.Credential` entry as `-c <key>=<value>` (H2) | every helper a repository names, including URL-scoped `credential.<url>.helper`. The empty value clears the list read so far, and command scope is read last (checked on git 2.53.0.windows.2, 2026-09-24: a repository `credential.https://example.invalid.helper` answered without the reset and did not answer with it). A `Read` or `Local` call needs no credential, so no helper runs in it at all |
| `--upload-pack=git-upload-pack` after `fetch` and `ls-remote`; `--receive-pack=git-receive-pack` after `push` | `remote.<name>.uploadpack` and `.receivepack`, which run as a local command when the remote is a path |
| `--literal-pathspecs` (a top-level option, in the prefix beside the `-c` pairs) | a path the agent named being read as a glob: `diff -- 'app/[slug]/page.tsx'` also matches `app/s/page.tsx`, so task 16 would file another file's hunks, with their back-links, under this path (re-review b05 issue 9). Checked on git 2.53.0.windows.2, 2026-09-24 (`scratchpad/p3/b05r2-lit-probe.out`): without it that diff lists both files, with it one; `rev-parse` and `for-each-ref` are unaffected. No phase 3 call uses a glob or magic pathspec, so none needs it off |

`Args[0]` is always the subcommand, because `Call.Dir` carries the directory. Exec refuses a
`Call` whose `Args[0]` starts with `-` (`ErrCallShape`), so the prefix stays Exec's alone and no
caller can put its own `-c` ahead of it.

**H2. Network calls take their proxy, TLS and helpers from the machine, never the repository.**
This closes review C3, where the user's token reached whatever the agent wrote.
- `cmd/api` reads the machine's git settings once at startup, from `git config --system` and then
  `--global`: every `credential.*helper` entry in file order (`Hardening.Credential`), and
  `http.proxy`. If neither file names a proxy, it reads `HTTPS_PROXY`, then `https_proxy`, then
  `ALL_PROXY`, and otherwise uses `""`, which means no proxy (`Hardening.Proxy`).
  `MachineHardening` does this, and task 04 calls it in `main.go`. The agent can write neither
  file.
- A `Remote`-class call must carry `Call.RemoteURL`, the literal URL it talks to. Exec refuses
  one without it. Exec then adds `-c http.sslVerify=true -c http.proxy=<Hardening.Proxy>` twice:
  once generic, and once scoped to the exact URL (`-c http.<RemoteURL>.sslVerify=true`,
  `-c http.<RemoteURL>.proxy=…`). git picks the most specific `http.<url>.*` match, whichever
  file it is in, so a generic override alone loses to a repository key such as
  `http.https://github.com.proxy`. At equal specificity, command scope is read last and wins.
- A per-remote `remote.<name>.proxy` beats both overrides: git reads it ahead of every
  `http.proxy` (verified by the b01 re-review on git 2.53.0.windows.2, loopback ports only). No
  `http.*` override can reach it, and fetch and push go by remote name (task 12). So H2 does not
  claim it. H3 refuses `remote.*.proxy`, `remote.*.proxyAuthMethod` and `remote.*.vcs` on every
  Project call. No host call runs in a lane clone at all (R4.2).
- The host names only the project, by its literal path, and remotes; a lane is reached only
  through the runner (task 24). The lane sync (task 07) fetches, in the project, from a bundle
  file the runner wrote into a host-owned directory (ruling R4.4). The origin refresh (task 09)
  goes the other way: the host writes a bundle from the project, and the runner fetches it inside
  the sandbox (R4.5). So `Machine` has no `FetchURL`. A local path or a bundle file needs no
  credential, proxy or TLS.

**H3. The config check: what the agent wrote is read before git acts on it.** A filter, diff or
merge driver cannot be neutralised by name, because `.gitattributes` chooses the name. So before
every call, Exec runs `git config --local --no-includes --list -z` in `Call.Dir` with the H1
prefix and H5's layout pin. Reading config runs nothing. A record is `key\nvalue\0`, or `key\0`
for a key with no value (checked on git 2.53.0.windows.2: `[x] flag` lists as `x.flag\0`). The
parser splits on `\0` only, takes the key up to the first `\n` or the end, and applies the rule to
every key, valued or not. So a value holding a newline can neither forge a key nor hide one.
Section and key names compare lowercased.

`Call.Scope` has no default (`Unset` is `ErrCallShape`). It follows `Dir`: a project path is
`Project`, and a `Dir` under `Hardening.OwnedRoot` is `None`. A `Dir` under `Hardening.LaneRoot`
is `ErrLaneDir` (ruling R4.2), refused before any check, except task 03's `NewLane` calls with
`Scope None` (its steps 4a to 6, in the clone `lanes.Create` is making; after `Store.Add`, no host
call runs there again, R4.5). `clone` runs with `Dir` the project and `Scope Project`, so H3
checks the source under the Project rule. Every other lane git call runs in the sandbox, through
task 24's runner. Every call site names its scope in its own task. What the check refuses
depends on `Call.Scope` and, for `Project`, on `Call.Class`:

| `Call.Scope` | Rule | Refusal |
|---|---|---|
| `Project` (the user's own repository, so a closed list would refuse their real config) | **refusal list**. **On every call** it refuses: `include.*` and `includeIf.*`; the execution-capable keys H1 cannot neutralise by name (`filter.*`, `diff.*.textconv`, `diff.*.command`, `diff.external`, `merge.*.driver`, `core.gitProxy`, `core.alternateRefsCommand`, `extensions.worktreeConfig`); the URL rewrites H2 cannot neutralise (`remote.*.pushurl`, `url.*.insteadOf`, `url.*.pushInsteadOf`); and the per-remote transport keys that beat H2 (`remote.*.proxy`, `remote.*.proxyAuthMethod`, `remote.*.vcs`). **On a `Remote` call only** it also refuses every `http.*` key (they shape that call's TLS, proxy and name resolution, for example `http.sslCAInfo` and `http.curloptResolve`, and H2 forces only two of them) and every `credential.*` key except four. The machine's helper runs in the project on a `Remote` call, and a helper reads its own settings from that repository's config: Git Credential Manager's `credential.trace` and `credential.traceSecrets` would write the token to a file the sandbox can read (from GCM's documentation; not exercised here). The four admitted keys are inert: `credential.helper` and `credential.<url>.helper` (H1's reset clears them) and `credential.useHttpPath` and `credential.<url>.useHttpPath` (they choose only whether the path is part of the query). On a `Read` or `Local` call, `http.*` and `credential.*` are inert, because no helper runs (H1) and https is not allowed (H1's protocol row), so they are admitted there. This repository's own `.git/config` (`credential.usehttppath` and a `credential.https://github.com/lcfrai/panaceaapp.helper`) passes every call. The cockpit's own project config writes are task 12's `branch.<b>.remote` and `.merge` (from `push --set-upstream`), task 11's `branch -m` (it moves `branch.<old>.*`) and task 11's `config --remove-section branch.<b>`. None is on the list, so no exception is needed. A user whose project config holds a refused key moves it to `--global`, and the refusal names the key | `*UntrustedConfig{Scope: Project}` → `conflict`, `reason: project_config_untrusted`, `details.key` |
| `None` | no check. Only for `git version`, for a `Dir` under `Hardening.OwnedRoot`, such as task 13's `GitHelper` repository, and for `Dir == Call.NewLane`, the clone `lanes.Create` is making, before any sandbox mounts it (R4.5). Exec refuses `None` with any other `Dir`. `CopyableHooksPath` (task 03) keeps the one rule the retired clone allowlist had: a relative `core.hooksPath` with no `..` segment | — |

A refused call never starts. `UntrustedConfig` carries the key, never the value, since a value may
be a secret.

*The account a cockpit push uses.* H1 clears a repository's own helper, so a cockpit fetch or push
authenticates as whatever the machine's helpers (system, then global) answer for that URL, not the
account a repo-local `credential.<url>.helper` chooses. On this repository that means the global
helper's account, not the one the repo-local `lcfrai/panaceaapp` helper selects. A user who wants
the repo-local choice moves that key to `--global`: it is URL-scoped, so it still applies to that
repository alone. Plan Q-table row Q8 records this.

*The source side of a local fetch or clone.* H3 checks `Call.Dir` only. The only local sources
are the project (task 03's clone, whose source is host-private, R4.1) and bundle files (task 24);
a bundle fetch runs no upload-pack and no source config (R4.4).

**H4. Control-plane writes run no repository hooks.** H1's `core.hooksPath` applies to the land
path too: task 09's `update-ref` and `commit-tree`, task 10's undo and revert, and task 12's push.
That includes the user's own husky `pre-push` on a cockpit push. The reason is that the cockpit's
plumbing writes run no hooks, so a land behaves the same on every machine. A cockpit push is therefore not the
same as the user running `git push`: a check the user's `pre-push` enforces must run in CI.

**H5. The read-only config bind is hygiene, not a control (R4).** Task 05's `sessionMounts`
still binds a lane clone's `<path>/.git/config` **read-only** over `/home/dev/work/.git/config`.
§HGH claims nothing from it: the host never reads a lane's config (R4.2), so no host call depends
on what that file holds. The cost is that the agent's own `git config` writes in its checkout
fail. Plan Q-table row Q7 records that trade-off.

*The layout pin.* git's discovery reads more of `.git` than
`config`. A `.git/commondir` file moves the common directory, and with it the `config` git reads.
Checked on git 2.53.0.windows.2, 2026-09-24: in a plain `git init` repository, writing
`../../evil` into `.git/commondir` made `git config user.name` read `evil/config`. Setting
`GIT_DIR` and `GIT_COMMON_DIR` to the repository's own `.git` restored the real file. So Exec pins
the layout on every `Project` call whose `<Dir>/.git` is a directory: it sets `GIT_DIR=<Dir>/.git`,
`GIT_COMMON_DIR=<Dir>/.git` and `GIT_WORK_TREE=<Dir>`, after `Call.Env`, and it refuses a
`Call.Env` naming any of the three (`ErrCallShape`; a lane path there is `ErrLaneDir` first). The
pin is not set for `clone`, which creates a repository rather than running in one. H3's check
runs under the same pin, so the check and the call read one file. A project whose `.git` is a
file (a linked worktree) is not pinned.
*Unverified:* whether any other `.git` file changes which config git reads. `commondir` is the one
this review found, from git's documented repository layout. `TestHGHCommondirCannotRedirect`
pins it, and nothing else is claimed.

**The §HGH tests.** Each is written first and shown **red** against an `Exec` with no §HGH: the
marker file appears, or the call runs. Markers are written by a script in `t.TempDir()`, and each
test asserts that the marker path does not exist after the call. They use real git (`gitxtest`),
except the argv tests, which use the helper process. **Each layer is tested alone.** A test of an
H1 or H2 override runs with a `Scope` whose H3 rule admits the key: `Project` for
`core.fsmonitor`, `remote.*.uploadpack` and `.receivepack`, and `None`, with `OwnedRoot` set to
the test's temp directory, for `credential.helper` and `http.*`. Otherwise H3 would refuse first,
and an H1 mutant would survive.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestHGHPrefixOnEveryClass` | helper process: for `Read`, `Local` and `Remote`, argv holds every H1 `-c` pair, between `git` and `Args[0]`; `protocol.https.allow=always` and the `Hardening.Credential` entries appear on `Remote` (and `Scope None`) only | a prefix added for one class only; https or a machine helper on a `Read` call |
| `TestHGHPathspecsAreLiteral` (re-review b05 issue 9) | a repository with `app/[slug]/page.tsx` and `app/s/page.tsx`, both changed in one commit; `diff --name-only HEAD~1 HEAD -- app/[slug]/page.tsx` through Exec → exactly `app/[slug]/page.tsx` | `--literal-pathspecs` dropped from the prefix: a bracketed Next.js path read as a glob |
| `TestHGHFsmonitorNeverRuns` | a clone whose `.git/config` sets `core.fsmonitor=<marker script>`; `status --porcelain` through Exec → no marker | `core.fsmonitor=false` dropped |
| `TestHGHReferenceTransactionHookNeverRuns` | `.git/hooks/reference-transaction` writes a marker; `update-ref` through Exec → no marker, and the ref moved | `core.hooksPath` pointed at the repository's own directory |
| `TestHGHUploadPackIsNeverTheRepositorys` | `remote.origin.uploadpack=<marker script>` with origin a local path; `fetch origin` through Exec → no marker, and the fetch succeeded | the `--upload-pack` insertion dropped |
| `TestHGHReceivePackIsNeverTheRepositorys` | the same with `remote.origin.receivepack` and `push` | `--receive-pack` missing |
| `TestHGHFilterDriverIsRefused` | project config `filter.x.clean=<marker script>`, `.gitattributes` `* filter=x`, and a modified file; `status` with `Scope: Project` → `*UntrustedConfig{Key: "filter.x.clean"}` and no marker | the H3 check skipped for `Read` |
| `TestHGHProjectRefusalList` | table over `Class`: every "every call" key in H3's `Project` row (including `remote.origin.proxy`, `.proxyAuthMethod` and `.vcs`) → refused on `Read`, `Local` and `Remote`; `http.sslCAInfo`, `http.<url>.proxy` and `credential.trace` → refused on `Remote`, admitted on `Read` and `Local`; `core.autocrlf`, `lfs.url`, `remote.origin.url` → admitted everywhere | a `Project` scope that refuses the user's ordinary config, or admits a per-remote proxy |
| `TestHGHProjectAdmitsRealConfig` | b01 re-review R1 and R2, red first against round 1's rule (`credential.*` refused on every Project call): a project holding this repository's shape (`credential.usehttppath=true`, `credential.https://github.com/lcfrai/panaceaapp.helper=<marker script>`) plus `branch.main.vscode-merge-base`, `remote.origin.gh-resolved`, `core.longpaths` and `pull.rebase` → a `Read`, a `Local` and a `Remote` call with `Scope Project` all pass H3, and the helper's marker never appears | a project call refused for ordinary config; a repo-local helper that runs |
| `TestHGHConfigParseIsNULSafe` | `user.name` set to `"a\nfilter.x.clean=y"` → admitted, with no key `filter.x.clean` seen; a real `filter.x.clean` beside it → refused; a valueless `filter.x.required` (listed as `key\0`) under `Project` → refused, naming it | line-splitting the listing; a parser that drops a key with no value |
| `TestHGHUntrustedConfigHidesTheValue` | a refused `credential.helper=s3cr3t` → `"s3cr3t"` in neither `Error()` nor any problem field | echoing the value |
| `TestHGHRemoteNeedsAURL` | `Class: Remote` with `RemoteURL: ""` → `ErrCallShape`, git never starts | a remote call on the repository's `origin` |
| `TestHGHRemoteOverridesBeatAURLScopedKey` | transport level, loopback only. `O` is a bare `httptest.NewTLSServer`, not `HTTPSRemote`, so no config trusts its certificate, and `U` is exactly its URL plus `/r.git`; `P` is a plain `httptest` listener that counts connections. A `Scope None` repository under `OwnedRoot` holds `remote.origin.url=U`, `http.<U>.sslVerify=false` and `http.<U>.proxy=<P's URL>`, keyed at exactly `U` (scheme, host, port and path). **Precondition**, raw git with no Exec: `config --get-urlmatch http.sslVerify U` → `false` and `http.proxy` → P, so the keys match. Then `ls-remote origin` through Exec with `RemoteURL: U` and `Hardening.Proxy: ""` → a certificate error, `P` saw no connection, and `O` served no request | the generic-only override (b01 re-review R4: a key keyed short of `U` never matched, and the old test could not fail) |
| `TestHGHRepoCredentialHelperNeverRuns` | table: repo config `credential.helper=<script that writes its stdin to a marker>`, then the URL-scoped `credential.https://example.invalid.helper=<same>`; `credential fill` for `host=example.invalid` through Exec → no marker, and the machine helper from `Hardening.Credential` answered | the empty `credential.helper=` reset dropped, or a reset that clears only the generic key |
| `TestHGHCallShape` | `Args[0] == "-c"` → `ErrCallShape`; `Scope` left `Unset` → `ErrCallShape`; `Scope: None` with a `Dir` outside `OwnedRoot` that is not its `NewLane` → `ErrCallShape`; `Call.Env` naming `GIT_DIR`, `GIT_COMMON_DIR` or `GIT_WORK_TREE` → `ErrCallShape`; `Exec{}` (no HooksDir) → `ErrCallShape`. Git never starts | a caller's own `-c` ahead of the prefix; an unmarked call that silently gets a rule (b01 re-review R1) |
| `TestHGHCommondirCannotRedirect` | b01 re-review R5, red first without the pin: a clone whose `.git/commondir` names a copy of its `.git` whose `config` sets `user.name=EVIL` and `filter.x.clean=<marker script>`, with `.gitattributes` `* filter=x` → `rev-parse --git-common-dir` through Exec prints `<Dir>/.git`, `config user.name` prints the real name, H3 admits the call, and `status` leaves no marker | a layout pin dropped, or set before `Call.Env` |
| `TestHGHUploadPackSourceRunsNothing` | a git-behaviour pin in task 01's style, not a mutant killer: a source repository whose config sets `uploadpack.packObjectsHook=<marker script>`, and whose `.git/hooks/` holds a marker script under every hook name; a local fetch from it through Exec → no marker | a git upgrade that honours the source's repository config |

Tasks 07, 09, 12 and 13 each add the end-to-end marker test for their own call site (the observer,
the land and origin refresh, the push, and `GitHelper`), citing this section.

#### Steps

- [ ] **Step 1: Preflight.** Task 01's results document exists; read its Divergences section. If
  it records merge-tree's exit code or output format differently from the table above, this
  task's `OKExit` guidance and task 08's parser follow the document.
- [ ] **Step 2:** write the tests above, the helper process first.
- [ ] **Step 2a:** write the 16 §HGH tests (the two clone-allowlist tests went with `Scope Clone`, R4.2). Run them against an `Exec` built without `harden.go`,
  and record in the task report each marker that appeared and each call that ran: red first.
- [ ] **Step 3:** implement `runner.go`, `error.go`, `version.go`, `machine.go`, `trailers.go`,
  `locks.go`, `harden.go`, `internal/spawnseam`, then `gitxtest`. The body of `Run` follows `gitrunner.go:46-78` line for line,
  except for the class switch, `OKExit`, and the §HGH prefix and check that run before `exec`.
- [ ] **Step 3a: Mutants for §HGH**, on a `$TEMP` copy. Drop each H1 override in turn. Drop the
  `--upload-pack` insertion. Check `ErrLaneDir` on `Dir` only, or on an uncleaned path. Split the config listing on
  `\n` instead of `\0`. Drop a valueless record's key. Keep only the generic H2 override. Treat
  `Unset` as `Project`. Drop the layout pin. Apply the Project `Remote`-only refusals on every call.
  Allow https on every class. Each must turn a named §HGH test red (`TestHGHUploadPackSourceRunsNothing`
  is a git-behaviour pin and has no mutant).
- [ ] **Step 4:** amend `gitrunner.go:19-20`'s comment to "the launcher for phase 2's read-only
  enumeration; `internal/gitx` is the other one".
- [ ] **Step 5:** run, then commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/gitx/... -race -count=1        → PASS (39 new tests, 18 of them §HGH, plus task 01's)
go test ./control-plane/internal/project/ -run TestEveryGitCommandIsReadOnly -count=1 → PASS, unchanged
git diff --stat control-plane/go.mod go.work                     → empty
```

#### What this task does not claim

- Nothing calls `gitx` yet except its tests. Task 03 is the first consumer.
- `CheckVersion` is not yet wired; task 04 calls it at startup.
- `Machine` has no second implementation; phase 8 owns it (seam S3-P8.1).
- §HGH hardens `gitx.Exec` only. Phase 2's `project.ExecRunner`, and the scanner that runs git
  in the primary worktree's repository, are not covered here. Whether their read-only commands
  are exposed is phase 2's to check, and is not verified (plan Q-table row Q6).
