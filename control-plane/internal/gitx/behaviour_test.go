package gitx

// Behaviour pins: each test runs the host's real git and asserts one fact a later phase 3
// task rests on (plan task 01). A test that fails here is a finding about git, not a bug in
// this package: the results document, docs/verification/2026-09-25-phase-3-git-behaviours.md,
// records what git did and which task it changes.
//
// Every call runs under a hermetic environment (no system config, an empty global file, a
// temporary HOME and a fixed identity), so the owner's own config can neither pass nor fail a
// pin. The production-shaped pins (review C2) swap GIT_CONFIG_NOSYSTEM for a system file of
// their own that holds what Git for Windows' installer writes, core.autocrlf=true.
//
// Every pin calls t.Parallel(): one git process costs about 2 s of wall-clock on the owner's
// laptop (git --version took 2.5 s on 2026-09-25, almost all of it outside git), and the pins
// start a few hundred.

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// gitEnv is a complete environment for a git child process, as "KEY=value" entries.
type gitEnv []string

// pinIdentity is the fixed identity every pin commits as.
var pinIdentity = []string{
	"GIT_AUTHOR_NAME=Pin Author", "GIT_AUTHOR_EMAIL=pin@example.invalid",
	"GIT_COMMITTER_NAME=Pin Author", "GIT_COMMITTER_EMAIL=pin@example.invalid",
}

// envKey is the name part of an environment entry. Windows' per-drive "=C:=C:\..." entries
// have an empty name and are kept as they are.
func envKey(kv string) string {
	k, _, _ := strings.Cut(kv, "=")
	return strings.ToUpper(k)
}

// inheritedEnv is this process's environment minus everything git reads as configuration:
// every GIT_* variable, HOME, XDG_CONFIG_HOME and EMAIL (git's identity fallback).
func inheritedEnv() gitEnv {
	var env gitEnv
	for _, kv := range os.Environ() {
		k := envKey(kv)
		if strings.HasPrefix(k, "GIT_") || k == "HOME" || k == "XDG_CONFIG_HOME" || k == "EMAIL" {
			continue
		}
		env = append(env, kv)
	}
	return env
}

// hermetic is the default pin environment.
func hermetic(t *testing.T) gitEnv {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH, so the git behaviour pins cannot run; install git and rerun")
	}
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	writeFile(t, filepath.Join(dir, "empty"), "")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	env := inheritedEnv().with(
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+filepath.Join(dir, "empty"),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
	)
	return env.with(pinIdentity...)
}

// production is hermetic with a system config file of the test's own in place of
// GIT_CONFIG_NOSYSTEM (review C2). system is that file's text.
func production(t *testing.T, system string) gitEnv {
	t.Helper()
	path := filepath.Join(t.TempDir(), "system")
	writeFile(t, path, system)
	return hermetic(t).without("GIT_CONFIG_NOSYSTEM").with("GIT_CONFIG_SYSTEM=" + path)
}

// installerSystem is what Git for Windows' installer writes that phase 3 cares about, and
// what this machine's C:/Program Files/Git/etc/gitconfig holds (2026-09-25).
const installerSystem = "[core]\n\tautocrlf = true\n"

// with returns a copy of e with each "KEY=value" entry set, replacing any earlier value.
func (e gitEnv) with(kvs ...string) gitEnv {
	out := append(gitEnv(nil), e...)
	for _, kv := range kvs {
		out = out.without(envKey(kv))
		out = append(out, kv)
	}
	return out
}

// without returns a copy of e with the named variables removed.
func (e gitEnv) without(keys ...string) gitEnv {
	var out gitEnv
	for _, kv := range e {
		drop := false
		for _, k := range keys {
			if envKey(kv) == strings.ToUpper(k) && k != "" {
				drop = true
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}

// run runs git in dir under env and returns what it printed and its exit code. The code is
// ProcessState.ExitCode(), never read through a pipe. A git that cannot be started fails the
// test.
func run(t *testing.T, dir string, env gitEnv, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	return runStdin(t, dir, env, "", args...)
}

func runStdin(t *testing.T, dir string, env gitEnv, stdin string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin = strings.NewReader(stdin)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("git %s: could not run: %v", strings.Join(args, " "), err)
		}
	}
	return out.String(), errb.String(), cmd.ProcessState.ExitCode()
}

// must runs git and fails the test unless it exits 0.
func must(t *testing.T, dir string, env gitEnv, args ...string) string {
	t.Helper()
	out, errs, code := run(t, dir, env, args...)
	if code != 0 {
		t.Fatalf("git %s in %s: exit %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), dir, code, out, errs)
	}
	return out
}

// line is must with its single trailing newline removed.
func line(t *testing.T, dir string, env gitEnv, args ...string) string {
	t.Helper()
	return strings.TrimSuffix(must(t, dir, env, args...), "\n")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var hex40 = regexp.MustCompile(`^[0-9a-f]{40}$`)

// newRepo makes a repository on main in a fresh temporary directory, commits files to it and
// returns its path.
func newRepo(t *testing.T, env gitEnv, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "src")
	must(t, t.TempDir(), env, "init", "-q", "-b", "main", dir)
	commitFiles(t, dir, env, "one", files)
	return dir
}

// commitFiles writes files into dir's working tree, commits them all and returns HEAD.
func commitFiles(t *testing.T, dir string, env gitEnv, msg string, files map[string]string) string {
	t.Helper()
	for name, content := range files {
		writeFile(t, filepath.Join(dir, filepath.FromSlash(name)), content)
	}
	must(t, dir, env, "add", ".")
	must(t, dir, env, "commit", "-q", "--allow-empty", "-m", msg)
	return line(t, dir, env, "rev-parse", "HEAD")
}

// objectsFixture is a repository holding both a pack and a loose object. It returns the
// repository and the two object files' paths relative to it.
func objectsFixture(t *testing.T, env gitEnv) (src, loose, pack string) {
	t.Helper()
	src = newRepo(t, env, map[string]string{"a.txt": "one\n"})
	must(t, src, env, "repack", "-a", "-d", "-q")
	commitFiles(t, src, env, "two", map[string]string{"b.txt": "two\n"})
	blob := line(t, src, env, "rev-parse", "HEAD:b.txt")
	loose = filepath.Join(".git", "objects", blob[:2], blob[2:])
	packs, err := filepath.Glob(filepath.Join(src, ".git", "objects", "pack", "*.pack"))
	if err != nil || len(packs) != 1 {
		t.Fatalf("fixture: want one pack, got %v (%v)", packs, err)
	}
	pack, _ = filepath.Rel(src, packs[0])
	for _, rel := range []string{loose, pack} {
		if _, err := os.Stat(filepath.Join(src, rel)); err != nil {
			t.Fatalf("fixture: %s missing: %v", rel, err)
		}
	}
	return src, loose, pack
}

func sameFile(t *testing.T, a, b string) bool {
	t.Helper()
	fa, err := os.Stat(a)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := os.Stat(b)
	if err != nil {
		t.Fatal(err)
	}
	return os.SameFile(fa, fb)
}

// clone runs `git clone -q <flags...> -- src <tmp>/dst` and returns dst.
func clone(t *testing.T, env gitEnv, src string, flags ...string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "dst")
	args := append(append([]string{"clone", "-q"}, flags...), "--", src, dst)
	must(t, t.TempDir(), env, args...)
	return dst
}

// --- Clones (task 03) -------------------------------------------------------------------

// `--local` shares the project's object files: that is why task 03 does not use it (I8, Q9).
func TestBehaviourLocalCloneHardLinksObjects(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	src, loose, pack := objectsFixture(t, env)
	dst := clone(t, env, src, "--local")
	for _, rel := range []string{loose, pack} {
		if !sameFile(t, filepath.Join(src, rel), filepath.Join(dst, rel)) {
			t.Errorf("%s in a --local clone is its own file; git did not hard-link it", rel)
		}
	}
}

// `--no-hardlinks` copies every object file, so an agent's write to a clone's pack cannot
// reach the project (I8, Q9).
func TestBehaviourNoHardlinksCloneSharesNoObjectFile(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	src, _, _ := objectsFixture(t, env)
	dst := clone(t, env, src, "--no-hardlinks")
	compared := 0
	root := filepath.Join(dst, ".git", "objects")
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dst, p)
		if _, err := os.Stat(filepath.Join(src, rel)); err != nil {
			return nil // a file only the clone has
		}
		compared++
		if sameFile(t, filepath.Join(src, rel), p) {
			t.Errorf("%s in a --no-hardlinks clone is the project's own file", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if compared < 2 {
		t.Fatalf("compared %d object files with the project; want the pack and the loose object at least", compared)
	}
	t.Logf("compared %d object files, none shared", compared)
}

// Neither clone borrows the project's objects through alternates, so a gc in the project
// cannot break a lane.
func TestBehaviourLocalCloneHasNoAlternates(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	src, _, _ := objectsFixture(t, env)
	for _, flag := range []string{"--local", "--no-hardlinks"} {
		dst := clone(t, env, src, flag)
		alt := filepath.Join(dst, ".git", "objects", "info", "alternates")
		if _, err := os.Stat(alt); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("clone %s: %s exists (stat err %v); the clone borrows the project's objects", flag, alt, err)
		}
	}
}
