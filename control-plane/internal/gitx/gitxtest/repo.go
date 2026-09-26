// Package gitxtest builds throwaway repositories for tests that need real git, and a scripted
// Runner for tests that must not.
package gitxtest

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/gitx/internal/spawnseam"
)

// Repo is a repository in t.TempDir() with an isolated config: GIT_CONFIG_NOSYSTEM=1,
// GIT_CONFIG_GLOBAL pointing at an empty file, a fixed identity, init.defaultBranch=main.
// NewRepo is a PROJECT, so its local config also holds keys real projects carry:
// branch.main.vscode-merge-base=origin/main, pull.rebase=true, credential.useHttpPath=true.
// §HGH H3's Project refusal list admits them, so a project check tightened past that list goes
// red in its own task's tests (b01 re-review R1). No host scope names a lane clone to run them under
// (ruling R4.2). CloneOf's repo holds only what git clone writes, since clone copies no local
// config from its source; it stands in for a lane clone, which host git never runs in.
// Runner()'s Hardening has LaneRoot "", so a test of ErrLaneDir sets LaneRoot itself.
type Repo struct {
	T   testing.TB
	Dir string
	Env map[string]string
}

// isolatedEnv is a hermetic git environment: no system config, an empty global file, a
// temporary HOME and a fixed identity.
func isolatedEnv(t testing.TB) map[string]string {
	t.Helper()
	cfg := t.TempDir()
	global := filepath.Join(cfg, "global.gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		t.Fatalf("gitxtest: %v", err)
	}
	return map[string]string{
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL":   global,
		"HOME":                cfg,
		"XDG_CONFIG_HOME":     cfg,
		"GIT_AUTHOR_NAME":     "Panacea Test",
		"GIT_AUTHOR_EMAIL":    "test@panacea.invalid",
		"GIT_COMMITTER_NAME":  "Panacea Test",
		"GIT_COMMITTER_EMAIL": "test@panacea.invalid",
	}
}

// NewRepo is an initialised project repository on branch main with one commit.
func NewRepo(t testing.TB) *Repo {
	t.Helper()
	r := &Repo{T: t, Dir: t.TempDir(), Env: isolatedEnv(t)}
	r.Git("init", "-q", "-b", "main")
	r.Git("config", "branch.main.vscode-merge-base", "origin/main")
	r.Git("config", "pull.rebase", "true")
	r.Git("config", "credential.useHttpPath", "true")
	r.Commit("initial", map[string]string{"README.md": "hello\n"})
	return r
}

// environ is the process environment with the repository-locating keys dropped, then r.Env.
func (r *Repo) environ() []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(k) {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_CONFIG_PARAMETERS":
			continue
		}
		env = append(env, kv)
	}
	keys := make([]string, 0, len(r.Env))
	for k := range r.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, k+"="+r.Env[k])
	}
	return env
}

// Git runs raw git (not gitx.Exec) in r.Dir under r.Env. It fails the test on error and
// returns stdout, trimmed.
func (r *Repo) Git(args ...string) string {
	r.T.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = r.environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		r.T.Fatalf("gitxtest: git %s in %s: %v: %s", strings.Join(args, " "), r.Dir, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

// Commit writes files (slash paths under r.Dir), adds them, commits, and returns the sha.
func (r *Repo) Commit(msg string, files map[string]string) string {
	r.T.Helper()
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		full := filepath.Join(r.Dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			r.T.Fatalf("gitxtest: %v", err)
		}
		if err := os.WriteFile(full, []byte(files[p]), 0o644); err != nil {
			r.T.Fatalf("gitxtest: %v", err)
		}
	}
	r.Git(append([]string{"add", "--"}, paths...)...)
	r.Git("commit", "-q", "--allow-empty", "-m", msg)
	return r.Git("rev-parse", "HEAD")
}

// NewExec is a gitx.Exec whose Hardening.HooksDir is an empty t.TempDir().
func NewExec(t testing.TB) *gitx.Exec {
	t.Helper()
	x, err := gitx.NewExec(gitx.Hardening{HooksDir: t.TempDir()})
	if err != nil {
		t.Fatalf("gitxtest: %v", err)
	}
	return x
}

// envRunner adds a repository's isolated environment under every call's own Env.
type envRunner struct {
	x   gitx.Runner
	env map[string]string
}

func (e envRunner) Run(ctx context.Context, c gitx.Call) (gitx.Result, error) {
	merged := make(map[string]string, len(e.env)+len(c.Env))
	for k, v := range e.env {
		merged[k] = v
	}
	for k, v := range c.Env {
		merged[k] = v
	}
	c.Env = merged
	return e.x.Run(ctx, c)
}

// Runner is an Exec whose calls carry r.Env; Hardening.HooksDir is an empty t.TempDir().
func (r *Repo) Runner() gitx.Runner { return r.RunnerOf(NewExec(r.T)) }

// RunnerOf wraps x so its calls carry r.Env, for a test that configures (or spies on) x itself.
func (r *Repo) RunnerOf(x gitx.Runner) gitx.Runner { return envRunner{x: x, env: r.Env} }

// CloneOf is `git clone --no-hardlinks --branch <branch>` of src (task 03's I8 rule), under a
// fresh isolated config.
func CloneOf(t testing.TB, src *Repo, branch string) *Repo {
	t.Helper()
	r := &Repo{T: t, Dir: filepath.Join(t.TempDir(), "clone"), Env: isolatedEnv(t)}
	parent := &Repo{T: t, Dir: filepath.Dir(r.Dir), Env: r.Env}
	parent.Git("clone", "-q", "--no-hardlinks", "--branch", branch, "--", src.Dir, r.Dir)
	return r
}

// UseSystemConfig makes r production-shaped (review C2): it drops GIT_CONFIG_NOSYSTEM from r.Env
// and sets GIT_CONFIG_SYSTEM to a file in t.TempDir() holding body, so the setting that breaks
// production reaches the test. Task 03's line-ending tests run with WindowsInstallerSystem,
// the system config Git for Windows' installer writes (and this machine holds).
func (r *Repo) UseSystemConfig(body string) {
	r.T.Helper()
	f := filepath.Join(r.T.TempDir(), "system.gitconfig")
	if err := os.WriteFile(f, []byte(body), 0o600); err != nil {
		r.T.Fatalf("gitxtest: %v", err)
	}
	delete(r.Env, "GIT_CONFIG_NOSYSTEM")
	r.Env["GIT_CONFIG_SYSTEM"] = f
}

const WindowsInstallerSystem = "[core]\n\tautocrlf = true\n"

// HTTPSRemote starts an httptest.NewTLSServer with h and returns its URL. It writes the server's
// certificate to a file and sets http.sslCAInfo to it in r's isolated GIT_CONFIG_GLOBAL file, which
// is not repository config, so §HGH admits it. A Remote call allows https only (§HGH H1), so every
// remote a phase 3 test talks to is one of these, never a plain httptest.NewServer.
func HTTPSRemote(t testing.TB, r *Repo, h http.Handler) string {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	ca := filepath.Join(t.TempDir(), "remote-ca.pem")
	block := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(ca, block, 0o600); err != nil {
		t.Fatalf("gitxtest: %v", err)
	}
	r.Git("config", "--file", r.Env["GIT_CONFIG_GLOBAL"], "http.sslCAInfo", filepath.ToSlash(ca))
	return srv.URL
}

// ErrUnscripted is what Script.Run returns for an argv no On line names.
var ErrUnscripted = errors.New("gitxtest: unscripted git call")

// Script is a Runner that answers from a table keyed by the joined argv and records calls.
// Build it with NewScript, which gives it the test an unscripted call fails.
type Script struct {
	Calls []gitx.Call

	mu    sync.Mutex
	t     testing.TB
	table map[string]scripted
}

type scripted struct {
	res gitx.Result
	err error
}

// NewScript is an empty Script that fails t on any call its table does not name.
func NewScript(t testing.TB) *Script { return &Script{t: t, table: map[string]scripted{}} }

// On answers the argv (Call.Args joined by single spaces) with res and err.
func (s *Script) On(argv string, res gitx.Result, err error) *Script {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.table == nil {
		s.table = map[string]scripted{}
	}
	s.table[argv] = scripted{res: res, err: err}
	return s
}

// Run records c and answers from the table. An unknown argv fails the calling test (Errorf, so
// it is safe from any goroutine) and returns ErrUnscripted.
func (s *Script) Run(ctx context.Context, c gitx.Call) (gitx.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls = append(s.Calls, c)
	argv := strings.Join(c.Args, " ")
	a, ok := s.table[argv]
	if !ok {
		if s.t != nil {
			s.t.Errorf("gitxtest: unscripted git call %q in %s", argv, c.Dir)
		}
		return gitx.Result{}, ErrUnscripted
	}
	return a.res, a.err
}

var _ gitx.Runner = (*Script)(nil)

// Spawn is one process a real Exec started: Argv is cmd.Args (the git executable first, then
// §HGH's prefix, then Call.Args), Dir is cmd.Dir, Env is cmd.Env as "KEY=value".
type Spawn = spawnseam.Spawn

// Spy installs a recorder on e's process seam and returns the slice it appends to, under a
// mutex, one Spawn per process e starts; e still starts it. Read the slice after the calls
// return. t.Cleanup removes the recorder. Env can hold a push token (task 13), so gitxtest
// never logs a Spawn. Task 24's TestHostNeverRunsGitInALane is its first caller.
func Spy(t testing.TB, e *gitx.Exec) *[]Spawn {
	t.Helper()
	var mu sync.Mutex
	got := &[]Spawn{}
	spawnseam.Set(e, func(s Spawn) {
		mu.Lock()
		defer mu.Unlock()
		*got = append(*got, s)
	})
	t.Cleanup(func() { spawnseam.Set(e, nil) })
	return got
}
