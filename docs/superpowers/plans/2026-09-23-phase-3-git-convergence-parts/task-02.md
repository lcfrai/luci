### Task 02: `gitx` — the write runner, its timeout classes, git's own sentence, the `Machine` seam

Phase 2's `project.ExecRunner` is *"the only place in the product that starts a git process"*
(`gitrunner.go:19-20`), and it is right for reads behind a 2 s poll: stdout only, headless, ten
seconds, `GIT_OPTIONAL_LOCKS=0`. Two of those are wrong for phase 3 (survey §1.2): a merge,
`update-ref` or clone must take its locks, and ten seconds is wrong for a clone of a large
repository or a push through a corporate proxy. This task builds the runner every phase 3 package
uses, copying `ExecRunner`'s discipline and changing exactly those two things per call class. It
also declares the one seam phase 8 needs (brief A6) and the trailer writer every land uses.

`project.ExecRunner` is **not** edited, and phase 2's `project.Runner` stays the read path for
enumeration. After this task the product has two process launchers; the comment on
`ExecRunner` that calls itself "the only place" is amended in one line to name this one too.

**Files:**
- `control-plane/internal/gitx/runner.go` — `Class`, `Call`, `Result`, `Runner`, `Exec` (new)
- `control-plane/internal/gitx/error.go` — `*Error`, `Sentence`, `AsError` (new)
- `control-plane/internal/gitx/version.go` — `CheckVersion` (new)
- `control-plane/internal/gitx/machine.go` — `Machine`, `Local` (new)
- `control-plane/internal/gitx/trailers.go` — trailer keys, `Message`, `TrailerFormat` (new)
- `control-plane/internal/gitx/gitxtest/repo.go` — real-git fixtures and a scripted fake (new)
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
	// Env is applied after os.Environ() and after the headless defaults, so a key here wins.
	// A value may be a secret (task 13 passes an http.extraHeader for push): Env is never
	// logged, never formatted into an Error, and never echoed in a Result.
	Env map[string]string
	// OKExit lists non-zero exit codes that are answers rather than failures. merge-tree
	// --write-tree exits 1 on a conflict (task 01's pin), and a conflict is a forecast.
	OKExit []int
}

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
}

var _ Runner = (*Exec)(nil)

func NewExec() *Exec { return &Exec{} }

// Run starts git headless — GIT_TERMINAL_PROMPT=0, GCM_INTERACTIVE=never, GIT_PAGER=cat,
// GIT_EDITOR=false (a call that wants an editor fails rather than hangs), plus
// GIT_OPTIONAL_LOCKS=0 for Read only — captures stdout and stderr separately, and returns an
// *Error for a start failure, a timeout, or an exit code outside {0} ∪ OKExit.
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
// adds a Kata machine behind the same four methods: a pod gets a clone, FetchURL names it,
// and the control plane fetches from it — the same model phase 3 builds (survey §4.4).
type Machine interface {
	// ID is the machine id worktrees and sessions carry: api.LocalMachineID ("local") here.
	ID() string
	// Git runs git on this machine.
	Git() Runner
	// CloneRoot is the absolute directory this machine's lane clones are created under.
	CloneRoot() string
	// FetchURL is how the project repository names a clone on this machine as a fetch
	// source. For Local it is the clone's absolute path, unchanged.
	FetchURL(clonePath string) string
}

// Local is this machine.
type Local struct {
	Runner Runner
	Root   string // absolute; cmd/api validates it (task 04)
}

func (l Local) ID() string                       { return "local" }
func (l Local) Git() Runner                      { return l.Runner }
func (l Local) CloneRoot() string                { return l.Root }
func (l Local) FetchURL(clonePath string) string { return clonePath }
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
type Repo struct {
	T   testing.TB
	Dir string
	Env map[string]string
}

func NewRepo(t testing.TB) *Repo
func (r *Repo) Git(args ...string) string                          // fails the test on error; trimmed stdout
func (r *Repo) Commit(msg string, files map[string]string) string  // write, add, commit; returns the sha
func (r *Repo) Runner() gitx.Runner                                 // an Exec whose calls carry r.Env
func CloneOf(t testing.TB, src *Repo, branch string) *Repo           // git clone --local --branch

// Script is a Runner that answers from a table keyed by the joined argv and records calls.
type Script struct {
	Calls []gitx.Call
	// unexported table
}

func (s *Script) On(argv string, res gitx.Result, err error) *Script
func (s *Script) Run(ctx context.Context, c gitx.Call) (gitx.Result, error) // unknown argv fails the test
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
| `TestLocalMachineFetchURLIsThePath` | `Local{Root: r}.FetchURL(p) == p`, `ID() == "local"` | a Local that rewrites paths |
| `TestScriptFailsOnAnUnscriptedCall` | an unknown argv fails the calling test | a fake that silently succeeds |

#### Host git hardening (HGH)

*Fix batch b01 (review C1, C3). This is the rule's only copy. Every other part cites
`task-02 §HGH` at the call site it hardens and restates none of it.*

**Why.** A lane clone's `.git` sits inside the sandbox's read-write worktree mount
(`sessionMounts`, `api/sessions.go`). So does the project's `.git` whenever a session runs on the
primary worktree. The agent can therefore write any key into either `config`, and any file into
either `hooks/`. The control plane runs the **host's** git in both repositories: task 07's
observer, task 09's land and origin refresh, task 10's undo and revert, task 11's remove question,
task 12's sync and push, and task 13's `credential fill`. Unhardened, each of those calls is code
execution on the Windows host, as the user, outside the microVM. Each is also a path to the
user's token, which breaks A4.

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
| `-c protocol.ext.allow=never` | `ext::<command>` URLs |
| `-c core.askPass=` | a repository-named askpass program. An empty value falls through to `GIT_TERMINAL_PROMPT=0`, which fails |
| `-c commit.gpgSign=false -c tag.gpgSign=false` | a repository `gpg.program` run by task 09's and task 10's `commit-tree` |
| `-c submodule.recurse=false -c fetch.recurseSubmodules=false` | a host fetch that recurses into `.git/modules/*`, whose config the agent also writes |
| `-c credential.helper=`, then `-c credential.helper=<h>` for each machine helper (H2) | every helper a repository names, including URL-scoped `credential.<url>.helper`. The empty value clears the list read so far, and command scope is read last |
| `--upload-pack=git-upload-pack` after `fetch` and `ls-remote`; `--receive-pack=git-receive-pack` after `push` | `remote.<name>.uploadpack` and `.receivepack`, which run as a local command when the remote is a path |

`Args[0]` is always the subcommand, because `Call.Dir` carries the directory. Exec refuses a
`Call` whose `Args[0]` starts with `-` (`ErrCallShape`), so the prefix stays Exec's alone and no
caller can put its own `-c` ahead of it.

#### Steps

- [ ] **Step 1: Preflight.** Task 01's results document exists; read its Divergences section. If
  it records merge-tree's exit code or output format differently from the table above, this
  task's `OKExit` guidance and task 08's parser follow the document.
- [ ] **Step 2:** write the tests above, the helper process first.
- [ ] **Step 3:** implement `runner.go`, `error.go`, `version.go`, `machine.go`, `trailers.go`,
  then `gitxtest`. The body of `Run` follows `gitrunner.go:46-78` line for line except the class
  switch and `OKExit`.
- [ ] **Step 4:** amend `gitrunner.go:19-20`'s comment to "the launcher for phase 2's read-only
  enumeration; `internal/gitx` is the other one".
- [ ] **Step 5:** run, then commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/gitx/... -race -count=1        → PASS (17 new tests plus task 01's)
go test ./control-plane/internal/project/ -run TestEveryGitCommandIsReadOnly -count=1 → PASS, unchanged
git diff --stat control-plane/go.mod go.work                     → empty
```

#### What this task does not claim

- Nothing calls `gitx` yet except its tests. Task 03 is the first consumer.
- `CheckVersion` is not yet wired; task 04 calls it at startup.
- `Machine` has no second implementation; phase 8 owns it (seam S3-P8.1).
