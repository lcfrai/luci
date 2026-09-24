package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// X-1 (phase 3 ruling R4 rule 6): the host-side resolvers read the HOST's own
// config and nothing a sandbox can write. A worktree's .git/config is the
// agent's, so a key there that names a host file must not choose what is
// mounted, and a line-ending key there must not choose what the sandbox gets.

// x1Host is a real host git confined to files this test writes: a system file
// with core.autocrlf=true, which is Git for Windows' installer default, and a
// global file naming the user's own excludes file and identity. It returns the
// worktree an agent has written a hostile .git/config into, the secret that
// config points at, and the global excludes file.
func x1Host(t *testing.T) (worktree, secret, globalIgnore string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	secret = filepath.Join(dir, "home", ".ssh", "id_rsa")
	globalIgnore = filepath.Join(dir, "home", "global-ignore")
	system := filepath.Join(dir, "gitconfig-system")
	global := filepath.Join(dir, "gitconfig-global")
	for path, body := range map[string]string{
		secret:       "-----BEGIN OPENSSH PRIVATE KEY-----\n",
		globalIgnore: "*.log\n",
		system:       "[core]\n\tautocrlf = true\n",
		global: "[core]\n\texcludesFile = " + filepath.ToSlash(globalIgnore) + "\n" +
			"[user]\n\tname = Host Person\n\temail = host@example.invalid\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "")
	t.Setenv("GIT_CONFIG_SYSTEM", system)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_COUNT", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("HOME", filepath.Join(dir, "home"))
	// Unset, not empty: git reads an empty GIT_AUTHOR_NAME as an empty name
	// and refuses it. t.Setenv first so the cleanup restores each one.
	for _, v := range []string{"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "EMAIL"} {
		t.Setenv(v, "")
		os.Unsetenv(v)
	}

	worktree = filepath.Join(dir, "projects", "repo")
	if out, err := exec.Command("git", "init", "-q", worktree).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	// What an agent in the sandbox writes: its worktree's .git/config is in
	// the mount, and git config writes it.
	for _, kv := range [][2]string{
		{"core.excludesFile", filepath.ToSlash(secret)},
		{"core.attributesFile", filepath.ToSlash(secret)},
		{"core.autocrlf", "input"},
		{"core.eol", "lf"},
		{"user.name", "Agent Chosen"},
		{"user.email", "agent@example.invalid"},
	} {
		if out, err := exec.Command("git", "-C", worktree, "config", kv[0], kv[1]).CombinedOutput(); err != nil {
			t.Fatalf("git config %s: %v: %s", kv[0], err, out)
		}
	}
	return worktree, secret, globalIgnore
}

// x1Spy is the real git runner with every invocation's directory and argv
// recorded, so a test can say where the host's git ran and with what.
type x1Spy struct {
	mu    sync.Mutex
	calls []identityCall
	real  project.Runner
}

func (s *x1Spy) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	s.mu.Lock()
	s.calls = append(s.calls, identityCall{dir: dir, args: append([]string(nil), args...)})
	s.mu.Unlock()
	return s.real.Run(ctx, dir, args...)
}

// x1NeverInTheWorktree fails for any git the spy saw run in, or pointed at,
// the worktree: its directory, -C, --git-dir or --work-tree, and for a config
// read that is not scoped to --global or --system.
func x1NeverInTheWorktree(t *testing.T, spy *x1Spy, worktree string) {
	t.Helper()
	if len(spy.calls) == 0 {
		t.Fatal("the resolver ran no git at all")
	}
	inside := func(p string) bool {
		rel, err := filepath.Rel(worktree, p)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	for _, c := range spy.calls {
		line := strings.Join(c.args, " ")
		if c.dir == "" || inside(c.dir) {
			t.Errorf("git %q ran in %q, which is the worktree or no directory at all", line, c.dir)
		}
		scoped := false
		for _, a := range c.args {
			if a == "-C" || a == "--git-dir" || a == "--work-tree" ||
				strings.HasPrefix(a, "--git-dir=") || strings.HasPrefix(a, "--work-tree=") {
				t.Errorf("git %q names a repository with %s", line, a)
			}
			if a == "--global" || a == "--system" {
				scoped = true
			}
		}
		if len(c.args) > 0 && c.args[0] == "config" && !scoped {
			t.Errorf("git %q reads config without --global or --system, so a repository's own scope counts", line)
		}
		if out, err := exec.Command("git", "-C", c.dir, "rev-parse", "--git-dir").CombinedOutput(); err == nil {
			t.Errorf("git %q ran in %q, which is inside the repository %s", line, c.dir, strings.TrimSpace(string(out)))
		}
	}
}

// The attack: core.excludesFile in the worktree's .git/config names an SSH
// key, and the next create mounts it. The resolver's answer is what the
// session mounts, so the secret must not be in it; the user's global file
// still is, and the unset attributes key is git's own default.
func TestARepoLocalExcludesFileNamingASecretIsNotMountedButAGlobalOneIs(t *testing.T) {
	worktree, secret, globalIgnore := x1Host(t)
	spy := &x1Spy{}

	got, err := x1GlobalFiles(t, spy, worktree)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	for label, p := range map[string]string{"excludes": got.Excludes, "attributes": got.Attributes} {
		if filepath.Clean(p) == filepath.Clean(secret) {
			t.Errorf("%s = %q: the worktree's own config chose a host file to mount", label, p)
		}
	}
	if filepath.Clean(got.Excludes) != filepath.Clean(globalIgnore) {
		t.Errorf("excludes = %q, want the global one %q", got.Excludes, globalIgnore)
	}
	if want := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "git", "attributes"); filepath.Clean(got.Attributes) != filepath.Clean(want) {
		t.Errorf("attributes = %q, want git's default %q", got.Attributes, want)
	}
	x1NeverInTheWorktree(t, spy, worktree)
}

// The worktree's core.autocrlf=input and core.eol=lf do not reach the
// sandbox: it gets the host's own answer, system core.autocrlf=true here.
func TestARepoLocalAutocrlfDoesNotChangeWhatTheSandboxGets(t *testing.T) {
	worktree, _, _ := x1Host(t)
	spy := &x1Spy{}

	got, err := x1LineEndings(t, spy, worktree)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := (api.GitLineEndings{AutoCRLF: "true", EOL: "crlf"}); got != want {
		t.Errorf("line endings = %#v, want the host's own %#v", got, want)
	}
	x1NeverInTheWorktree(t, spy, worktree)
}

// The identity is the host's too: the worktree's user.name is the agent's.
func TestARepoLocalIdentityIsNotTheOneCommitsAreMadeAs(t *testing.T) {
	worktree, _, _ := x1Host(t)
	spy := &x1Spy{}

	got, err := x1Identity(t, spy, worktree)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := (api.GitIdentity{Name: "Host Person", Email: "host@example.invalid"}); got != want {
		t.Errorf("identity = %#v, want the host's own %#v", got, want)
	}
	x1NeverInTheWorktree(t, spy, worktree)
}

// newHostGit's directory is made in the temp directory, and nothing stops a
// temp directory being inside a repository. GIT_CEILING_DIRECTORIES keeps git
// from finding it: git var reads a found repository's user.name, so without
// the ceiling the enclosing repository's identity would be the answer.
func TestHostGitFindsNoRepositoryEvenWhenTempIsInsideOne(t *testing.T) {
	worktree, _, _ := x1Host(t)
	tmp := filepath.Join(worktree, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(v, tmp)
	}
	spy := &x1Spy{}
	host := x1HostGit(t, spy)
	if rel, err := filepath.Rel(worktree, host.dir); err != nil || strings.HasPrefix(rel, "..") {
		t.Fatalf("hostGit's directory %q is not inside %q, so this test proves nothing", host.dir, worktree)
	}

	got, err := gitIdentityResolver("", "", host)(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := (api.GitIdentity{Name: "Host Person", Email: "host@example.invalid"}); got != want {
		t.Errorf("identity = %#v, want the host's own %#v: git found the enclosing repository", got, want)
	}
	eol, err := gitLineEndingsResolverWith(host, "crlf")(context.Background())
	if want := (api.GitLineEndings{AutoCRLF: "true", EOL: "crlf"}); err != nil || eol != want {
		t.Errorf("line endings = %#v, %v; want the host's own %#v", eol, err, want)
	}
}

// x1HostGit is newHostGit, the hostGit main.go wires, with spy recording
// every git it runs. The worktree is not an argument: no resolver takes one.
func x1HostGit(t *testing.T, spy *x1Spy) hostGit {
	t.Helper()
	host, cleanup, err := newHostGit()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	spy.real = host.run
	host.run = spy
	return host
}

// The resolvers as main.go wires them, at a session create in worktree.
func x1GlobalFiles(t *testing.T, spy *x1Spy, worktree string) (api.GitGlobalFiles, error) {
	return gitGlobalFilesResolver(x1HostGit(t, spy))(context.Background())
}

func x1LineEndings(t *testing.T, spy *x1Spy, worktree string) (api.GitLineEndings, error) {
	return gitLineEndingsResolverWith(x1HostGit(t, spy), "crlf")(context.Background())
}

func x1Identity(t *testing.T, spy *x1Spy, worktree string) (api.GitIdentity, error) {
	return gitIdentityResolver("", "", x1HostGit(t, spy))(context.Background())
}
