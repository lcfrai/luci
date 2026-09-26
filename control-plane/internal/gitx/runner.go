package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/internal/spawnseam"
)

// Class decides a call's timeout and whether it may take git's optional locks.
type Class int

const (
	// Read never takes an optional lock (GIT_OPTIONAL_LOCKS=0), so a status or a diff run
	// beside an agent's own git never races it for index.lock. ReadTimeout bounds it.
	Read Class = iota
	// LocalWrite writes a repository on this machine: clone, fetch between two local
	// repositories, merge-tree, commit-tree, update-ref, config. It takes its locks. The plan
	// named it Local, which is also the Machine type's name (machine.go); one package cannot
	// hold both, so the class is LocalWrite (task 02 departure 1).
	LocalWrite
	// Remote talks to a git host over the network: fetch origin, push.
	Remote
)

const (
	ReadTimeout = 10 * time.Second
	// LocalTimeout is also task 24's bundle create and bundle fetch in the project: equal to
	// the runner's BundleTimeout (review r4-2 I-2).
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
	// The `clone` itself is not marked: it is Scope Project with Dir the project and NewLane "",
	// and its destination is a positional, which ErrLaneDir does not parse. Task 24's spy test
	// names that one call as its second exception (review r4-1 I-2).
	NewLane string
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
	// Hardening is §HGH's prefix. An Exec whose HooksDir is "" runs nothing (ErrCallShape).
	Hardening Hardening
	// spawn is the process seam: nil in production. When set, Run hands it the child's final
	// argv, Dir and environment just before cmd.Start, for every process it starts (the H3
	// config read included) and for none it refuses. Only gitxtest.Spy sets it, through
	// internal/spawnseam, which no package outside gitx can import (Go's internal rule).
	spawn func(spawnseam.Spawn)
}

var _ Runner = (*Exec)(nil)

func init() {
	spawnseam.Set = func(e any, f func(spawnseam.Spawn)) {
		e.(*Exec).spawn = f
	}
}

// NewExec checks that h.HooksDir is an existing, empty directory, then returns the Exec.
func NewExec(h Hardening) (*Exec, error) {
	if h.HooksDir == "" {
		return nil, fmt.Errorf("%w: Hardening.HooksDir is empty", ErrCallShape)
	}
	fi, err := os.Stat(h.HooksDir)
	if err != nil {
		return nil, fmt.Errorf("gitx: hooks directory: %w", err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("gitx: hooks directory %s is not a directory", h.HooksDir)
	}
	ents, err := os.ReadDir(h.HooksDir)
	if err != nil {
		return nil, fmt.Errorf("gitx: hooks directory: %w", err)
	}
	if len(ents) != 0 {
		return nil, fmt.Errorf("gitx: hooks directory %s is not empty", h.HooksDir)
	}
	return &Exec{Hardening: h}, nil
}

func (x *Exec) timeout(c Class) time.Duration {
	if d := x.Timeouts[c]; d > 0 {
		return d
	}
	switch c {
	case LocalWrite:
		return LocalTimeout
	case Remote:
		return RemoteTimeout
	default:
		return ReadTimeout
	}
}

func (x *Exec) git() string {
	if x.Git == "" {
		return "git"
	}
	return x.Git
}

// Run refuses a lane call with ErrLaneDir and a bad shape with ErrCallShape, then applies §HGH
// (the H1 and H2 prefix, then the H3 config check for c.Scope, which refuses with
// *UntrustedConfig before git starts), then starts git headless — GIT_TERMINAL_PROMPT=0,
// GCM_INTERACTIVE=never, GIT_PAGER=cat, GIT_EDITOR=false (a call that wants an editor fails
// rather than hangs), plus GIT_OPTIONAL_LOCKS=0 for Read only — captures stdout and stderr
// separately, and returns an *Error for a start failure, a timeout, or an exit code outside
// {0} ∪ OKExit.
func (x *Exec) Run(ctx context.Context, c Call) (Result, error) {
	if err := x.guardLane(c); err != nil {
		return Result{}, err
	}
	if err := x.checkShape(c); err != nil {
		return Result{}, err
	}
	var pin []string
	if c.Scope == Project {
		pin = layoutPin(c.Dir)
		if err := x.checkConfig(ctx, c, pin); err != nil {
			return Result{}, err
		}
		if c.Args[0] == "clone" { // clone creates a repository rather than running in one (H5)
			pin = nil
		}
	}
	argv := append(x.prefix(c.Class, c.Scope, c.RemoteURL), packPins(c.Args)...)
	return x.start(ctx, c, argv, environ(c.Class, c.Env, pin))
}

// statusDLLInitFailed is Windows' STATUS_DLL_INIT_FAILED: the loader gave up before git's main
// ran, and ExitCode() hands it back as 3221225794 (carried t01-D14). git never exits with it.
const statusDLLInitFailed = 0xC0000142

// errLoader marks a git that Windows could not start; no git code ran, so one retry is safe.
var errLoader = errors.New("windows could not start git (0xC0000142, STATUS_DLL_INIT_FAILED)")

// waitDelay bounds Wait after a kill, when a grandchild still holds git's stdout or stderr.
const waitDelay = 5 * time.Second

// start runs argv once, and once more only when no git code ran the first time (t01-D14). A
// git that started and then failed is never retried: it cannot be told from a real failure.
func (x *Exec) start(ctx context.Context, c Call, argv, env []string) (Result, error) {
	res, err := x.once(ctx, c, argv, env)
	if ge, ok := AsError(err); ok && ge.NotStarted && errors.Is(ge.Err, errLoader) {
		res, err = x.once(ctx, c, argv, env)
	}
	return res, err
}

func (x *Exec) once(ctx context.Context, c Call, argv, env []string) (Result, error) {
	cctx, cancel := context.WithTimeout(ctx, x.timeout(c.Class))
	defer cancel()
	cmd := exec.CommandContext(cctx, x.git(), argv...)
	cmd.Dir = c.Dir
	cmd.Env = env
	cmd.WaitDelay = waitDelay
	if c.Stdin != nil {
		cmd.Stdin = bytes.NewReader(c.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if f := x.spawn; f != nil {
		f(spawnseam.Spawn{Argv: append([]string(nil), cmd.Args...), Dir: cmd.Dir, Env: append([]string(nil), cmd.Env...)})
	}
	err := cmd.Run()
	if err == nil {
		return Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, nil
	}
	fail := func(code int, cause error) *Error {
		return &Error{Args: append([]string(nil), c.Args...), Dir: c.Dir, ExitCode: code,
			Stderr: capTail(strings.TrimSpace(stderr.String()), MaxSentence), Err: cause}
	}
	// The per-call context is read before the exit code (carried t01-D16): Windows kills with
	// TerminateProcess(h, 1), so a timeout would otherwise read as exit 1, which OKExit lists
	// (merge-tree, merge-base --is-ancestor) treat as an answer.
	if cerr := cctx.Err(); cerr != nil {
		e := fail(-1, cerr)
		e.TimedOut = errors.Is(cerr, context.DeadlineExceeded)
		return Result{}, e
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		e := fail(-1, err)
		e.NotStarted = cmd.Process == nil
		return Result{}, e
	}
	code := ee.ExitCode()
	if uint32(code) == statusDLLInitFailed {
		e := fail(-1, fmt.Errorf("%w: %w", errLoader, err))
		e.NotStarted = true
		return Result{}, e
	}
	for _, ok := range c.OKExit {
		if code == ok {
			return Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: code}, nil
		}
	}
	return Result{}, fail(code, err)
}

// inheritedDrop are the process-environment keys a child never inherits: the ones that choose
// which repository git runs in (Exec pins or refuses them per call, §HGH H5), compared without
// case because Windows' environment is case-insensitive.
var inheritedDrop = []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE",
	"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_OPTIONAL_LOCKS"}

func envKeyIn(k string, keys []string) bool {
	for _, want := range keys {
		if strings.EqualFold(k, want) {
			return true
		}
	}
	return false
}

// environ is os.Environ(), then the headless defaults, then the call's Env (sorted, so a key
// there wins), then H5's layout pin. Go's exec keeps the last of a repeated key.
func environ(class Class, callEnv map[string]string, pin []string) []string {
	base := os.Environ()
	env := make([]string, 0, len(base)+len(callEnv)+len(pin)+5)
	for _, kv := range base {
		if k, _, _ := strings.Cut(kv, "="); k != "" && envKeyIn(k, inheritedDrop) {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never", "GIT_PAGER=cat", "GIT_EDITOR=false")
	if class == Read {
		env = append(env, "GIT_OPTIONAL_LOCKS=0")
	}
	keys := make([]string, 0, len(callEnv))
	for k := range callEnv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, k+"="+callEnv[k])
	}
	return append(env, pin...)
}
