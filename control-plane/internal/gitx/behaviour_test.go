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
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// runtimeWindows gates the pins whose answer is Git for Windows' own (a 260-character path,
// read-only object files).
var runtimeWindows = runtime.GOOS == "windows"

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

// task03LineEndings is task 03 step 4's line-ending half of the clone argv, one flag pair
// per line.
var task03LineEndings = []string{
	"--no-hardlinks",
	"--config", "core.autocrlf=false",
	"--config", "core.eol=lf",
	"--config", "core.checkStat=minimal",
	"--config", "core.trustctime=false",
}

// A plain clone inherits the system core.autocrlf=true and writes no local key, and task
// 03's four keys undo it (review C2).
func TestBehaviourSystemAutocrlfReachesAClone(t *testing.T) {
	t.Parallel()
	src := newRepo(t, hermetic(t), map[string]string{
		"a.txt": "x\ny\n", "b.bat": "x\ny\n", ".gitattributes": "*.bat eol=crlf\n",
	})
	env := production(t, installerSystem)

	plain := clone(t, env, src, "--no-hardlinks")
	if got := readFile(t, filepath.Join(plain, "a.txt")); got != "x\r\ny\r\n" {
		t.Errorf("plain clone a.txt = %q, want %q (the system autocrlf did not reach it)", got, "x\r\ny\r\n")
	}
	if out, errs, code := run(t, plain, env, "config", "--local", "--get", "core.autocrlf"); code != 1 {
		t.Errorf("plain clone: config --local --get core.autocrlf exit %d (%q %q), want 1: unset", code, out, errs)
	}

	fixed := clone(t, env, src, task03LineEndings...)
	if got := readFile(t, filepath.Join(fixed, "a.txt")); got != "x\ny\n" {
		t.Errorf("task 03 clone a.txt = %q, want %q", got, "x\ny\n")
	}
	if got := readFile(t, filepath.Join(fixed, "b.bat")); got != "x\r\ny\r\n" {
		t.Errorf("task 03 clone b.bat = %q, want %q (eol=crlf)", got, "x\r\ny\r\n")
	}
	local := strings.ToLower(must(t, fixed, env, "config", "--local", "--list"))
	for _, kv := range []string{"core.autocrlf=false", "core.eol=lf", "core.checkstat=minimal", "core.trustctime=false"} {
		if !strings.Contains("\n"+local, "\n"+kv+"\n") {
			t.Errorf("task 03 clone's local config lacks %s:\n%s", kv, local)
		}
	}
	if st := must(t, fixed, env, "status", "--porcelain"); st != "" {
		t.Errorf("task 03 clone: status --porcelain = %q, want empty", st)
	}
}

// symlinkRepo holds t.txt and a symlink entry link -> t.txt (mode 120000), which no
// working-tree file backs, so it works for a user who cannot make symlinks.
func symlinkRepo(t *testing.T) string {
	t.Helper()
	env := hermetic(t)
	src := newRepo(t, env, map[string]string{"t.txt": "x\n"})
	oid := strings.TrimSpace(mustStdin(t, src, env, "t.txt", "hash-object", "-w", "--stdin"))
	must(t, src, env, "update-index", "--add", "--cacheinfo", "120000,"+oid+",link")
	must(t, src, env, "commit", "-q", "-m", "link")
	return src
}

func mustStdin(t *testing.T, dir string, env gitEnv, stdin string, args ...string) string {
	t.Helper()
	out, errs, code := runStdin(t, dir, env, stdin, args...)
	if code != 0 {
		t.Fatalf("git %s: exit %d: %s", strings.Join(args, " "), code, errs)
	}
	return out
}

// canSymlink reports whether this user may create a symbolic link (on Windows: Developer
// Mode or SeCreateSymbolicLinkPrivilege).
func canSymlink(t *testing.T) bool {
	dir := t.TempDir()
	return os.Symlink("t.txt", filepath.Join(dir, "probe")) == nil
}

// linkShape reports how a clone checked `link` out: "symlink", or "file:<its bytes>".
func linkShape(t *testing.T, dst string) string {
	t.Helper()
	p := filepath.Join(dst, "link")
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return "symlink"
	}
	return "file:" + readFile(t, p)
}

// What a clone does with a symlink follows the machine's system config, which is why task 03
// writes core.symlinks=false into every clone (review Minor 15).
func TestBehaviourCloneSymlinksFollowTheSystemConfig(t *testing.T) {
	t.Parallel()
	src := symlinkRepo(t)
	withSymlinks := production(t, installerSystem+"\tsymlinks = true\n")
	installer := production(t, installerSystem)
	userCan := canSymlink(t)
	t.Logf("this user can create symlinks: %v", userCan)

	c := clone(t, withSymlinks, src, "--no-hardlinks")
	if out, _, code := run(t, c, withSymlinks, "config", "--local", "--get", "core.symlinks"); code != 1 {
		t.Errorf("system symlinks=true: the clone wrote a local core.symlinks %q (exit %d), want none", out, code)
	}
	got := linkShape(t, c)
	t.Logf("system symlinks=true, plain clone: link is %s", got)
	if userCan && got != "symlink" {
		t.Errorf("system symlinks=true, plain clone: link is %s, want a symlink", got)
	}

	c = clone(t, installer, src, "--no-hardlinks")
	if v := line(t, c, installer, "config", "--local", "--get", "core.symlinks"); v != "false" {
		t.Errorf("system without symlinks: local core.symlinks = %q, want false", v)
	}
	if got := linkShape(t, c); got != "file:t.txt" {
		t.Errorf("system without symlinks: link is %s, want file:t.txt", got)
	}

	for name, env := range map[string]gitEnv{"symlinks=true": withSymlinks, "installer": installer} {
		c = clone(t, env, src, "--no-hardlinks", "--config", "core.symlinks=false")
		if v := line(t, c, env, "config", "--local", "--get", "core.symlinks"); v != "false" {
			t.Errorf("system %s, --config core.symlinks=false: local core.symlinks = %q", name, v)
		}
		if got := linkShape(t, c); got != "file:t.txt" {
			t.Errorf("system %s, --config core.symlinks=false: link is %s, want file:t.txt", name, got)
		}
	}
}

// noFilterEnv is task 03's NoFilterEnv: command-scope empty LFS drivers (review Minor 13).
var noFilterEnv = []string{
	"GIT_CONFIG_COUNT=3",
	"GIT_CONFIG_KEY_0=filter.lfs.smudge", "GIT_CONFIG_VALUE_0=",
	"GIT_CONFIG_KEY_1=filter.lfs.process", "GIT_CONFIG_VALUE_1=",
	"GIT_CONFIG_KEY_2=filter.lfs.required", "GIT_CONFIG_VALUE_2=false",
}

const lfsPointer = "version https://git-lfs.github.com/spec/v1\noid sha256:00\nsize 3\n"

// lfsSystem is a system config defining an LFS-shaped filter whose smudge touches marker and
// prints SMUDGED. The clean is `cat` because without one a later host status exits 128
// (task 03's TestCreateWritesBlobBytes fixture).
func lfsSystem(t *testing.T, marker string) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "smudge.sh")
	writeFile(t, script, "#!/bin/sh\ncat >/dev/null\ntouch '"+filepath.ToSlash(marker)+"'\nprintf SMUDGED\n")
	return installerSystem + "[filter \"lfs\"]\n\tsmudge = sh " + filepath.ToSlash(script) +
		"\n\tclean = cat\n\trequired = true\n"
}

// An empty command-scope smudge and process, with required=false, turn a system LFS driver
// off for one clone, so the checkout writes the blob's own bytes (review Minor 13).
func TestBehaviourEmptyFilterEnvChecksOutTheBlob(t *testing.T) {
	t.Parallel()
	src := newRepo(t, hermetic(t), map[string]string{
		"a.bin": lfsPointer, ".gitattributes": "*.bin filter=lfs -text\n",
	})
	marker := filepath.Join(t.TempDir(), "smudge-ran")
	env := production(t, lfsSystem(t, marker))

	plain := clone(t, env, src, "--no-hardlinks")
	if got := readFile(t, filepath.Join(plain, "a.bin")); got != "SMUDGED" {
		t.Errorf("plain clone a.bin = %q, want the smudge's SMUDGED", got)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("plain clone: the system smudge did not run (%v)", err)
	}
	if err := os.Remove(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}

	quiet := clone(t, env.with(noFilterEnv...), src, "--no-hardlinks")
	if got := readFile(t, filepath.Join(quiet, "a.bin")); got != lfsPointer {
		t.Errorf("NoFilterEnv clone a.bin = %q, want the blob %q", got, lfsPointer)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("NoFilterEnv clone: the system smudge ran (marker stat %v)", err)
	}
	if out, errs, code := run(t, quiet, env, "status", "--porcelain"); code != 0 {
		t.Errorf("a later production-shaped status in the clone exits %d (%q %q), want 0", code, out, errs)
	}

	// git-lfs's own install also sets filter.lfs.process, which outranks smudge. The mutation
	// check found the empty smudge redundant (git skips smudge whenever process is set, and an
	// empty process runs nothing), so this half pins which of NoFilterEnv's keys carry it.
	script := filepath.Join(t.TempDir(), "process.sh")
	writeFile(t, script, "#!/bin/sh\ntouch '"+filepath.ToSlash(marker)+"'\nexit 0\n")
	penv := production(t, lfsSystem(t, marker)+"\tprocess = sh "+filepath.ToSlash(script)+"\n")
	_, errs, code := run(t, t.TempDir(), penv, "clone", "-q", "--no-hardlinks", "--", src, filepath.Join(t.TempDir(), "p"))
	_, statErr := os.Stat(marker)
	t.Logf("plain clone, system filter.lfs.process: exit %d, driver ran %v, stderr verbatim:\n%s", code, statErr == nil, errs)
	if statErr != nil {
		t.Errorf("plain clone with a system filter.lfs.process: the process driver did not run")
	}
	os.Remove(marker)
	for _, c := range []struct {
		name string
		env  gitEnv
		ok   bool
	}{
		{"NoFilterEnv", penv.with(noFilterEnv...), true},
		{"process= and required=false only", penv.with("GIT_CONFIG_COUNT=2",
			"GIT_CONFIG_KEY_0=filter.lfs.process", "GIT_CONFIG_VALUE_0=",
			"GIT_CONFIG_KEY_1=filter.lfs.required", "GIT_CONFIG_VALUE_1=false"), true},
		{"smudge= and process= without required=false", penv.with("GIT_CONFIG_COUNT=2",
			"GIT_CONFIG_KEY_0=filter.lfs.smudge", "GIT_CONFIG_VALUE_0=",
			"GIT_CONFIG_KEY_1=filter.lfs.process", "GIT_CONFIG_VALUE_1="), false},
	} {
		dst := filepath.Join(t.TempDir(), "q")
		_, errs, code := run(t, t.TempDir(), c.env, "clone", "-q", "--no-hardlinks", "--", src, dst)
		_, statErr := os.Stat(marker)
		t.Logf("%s over a system process driver: exit %d, driver ran %v, stderr %q", c.name, code, statErr == nil, errs)
		if statErr == nil {
			t.Errorf("%s: the system process driver ran", c.name)
		}
		if c.ok && (code != 0 || readFile(t, filepath.Join(dst, "a.bin")) != lfsPointer) {
			t.Errorf("%s: exit %d; want 0 and a.bin equal to its blob", c.name, code)
		}
		if !c.ok && code == 0 {
			t.Errorf("%s: exit 0; want the checkout to fail, since a required driver that runs nothing is a failure", c.name)
		}
	}
}

// deepPath is a 5 x 50-character path, so a checkout under a temporary directory passes
// Windows' 260.
var deepPath = strings.Repeat(strings.Repeat("a", 50)+"/", 5) + "f.txt"

// A deep repository fails its checkout under Git for Windows unless core.longpaths is set
// (review Minor 15).
func TestBehaviourLongPathNeedsCoreLongpaths(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	src := newRepo(t, env, map[string]string{"top.txt": "x\n"})
	oid := strings.TrimSpace(mustStdin(t, src, env, "x\n", "hash-object", "-w", "--stdin"))
	must(t, src, env, "update-index", "--add", "--cacheinfo", "100644,"+oid+","+deepPath)
	must(t, src, env, "commit", "-q", "-m", "deep")

	dst := filepath.Join(t.TempDir(), "dst")
	if n := len(dst) + 1 + len(deepPath); n <= 260 {
		t.Fatalf("fixture: the deep file's path is %d characters, not past 260", n)
	}
	t.Logf("deep file path length in the clone: %d (clone root %d)", len(dst)+1+len(deepPath), len(dst))
	_, errs, code := run(t, t.TempDir(), env, "clone", "-q", "--no-hardlinks", "--", src, dst)
	t.Logf("without core.longpaths: exit %d, stderr verbatim:\n%s", code, errs)
	if runtimeWindows {
		if code != 128 || !strings.Contains(errs, "Filename too long") ||
			!strings.Contains(errs, "warning: Clone succeeded, but checkout failed.") {
			t.Errorf("plain clone of a deep tree: exit %d, stderr %q; want 128, Filename too long, checkout failed", code, errs)
		}
	}

	fixed := filepath.Join(t.TempDir(), "dst")
	_, errs, code = run(t, t.TempDir(), env, "clone", "-q", "--no-hardlinks", "--config", "core.longpaths=true", "--", src, fixed)
	if code != 0 {
		t.Fatalf("clone --config core.longpaths=true: exit %d: %s", code, errs)
	}
	if st := must(t, fixed, env, "status", "--porcelain"); st != "" {
		t.Errorf("longpaths clone: status --porcelain = %q, want empty", st)
	}
}

// A clone of one branch tracks that branch of origin, so the agent's `git status` shows its
// ahead/behind against origin/feat/x (tasks 03, 09).
func TestBehaviourCloneBranchSetsUpstream(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	src := newRepo(t, env, map[string]string{"a.txt": "x\n"})
	must(t, src, env, "branch", "feat/x")
	dst := clone(t, env, src, "--no-hardlinks", "--branch", "feat/x")
	if b := line(t, dst, env, "symbolic-ref", "--short", "HEAD"); b != "feat/x" {
		t.Errorf("clone --branch feat/x checked out %q", b)
	}
	if r := line(t, dst, env, "config", "--get", "branch.feat/x.remote"); r != "origin" {
		t.Errorf("branch.feat/x.remote = %q, want origin", r)
	}
	if m := line(t, dst, env, "config", "--get", "branch.feat/x.merge"); m != "refs/heads/feat/x" {
		t.Errorf("branch.feat/x.merge = %q, want refs/heads/feat/x", m)
	}
}

// --- merge-tree (tasks 07, 08, 09, 10, 16) ------------------------------------------------

// twoBranches makes a repository whose main holds base, then commits a on branch A and b on
// branch B, each forked from main. It returns the repository and the two tips.
func twoBranches(t *testing.T, env gitEnv, base, a, b map[string]string) (dir, tipA, tipB string) {
	t.Helper()
	dir = newRepo(t, env, base)
	must(t, dir, env, "switch", "-q", "-c", "A")
	tipA = commitFiles(t, dir, env, "a", a)
	must(t, dir, env, "switch", "-q", "-c", "B", "main")
	tipB = commitFiles(t, dir, env, "b", b)
	must(t, dir, env, "switch", "-q", "main")
	return dir, tipA, tipB
}

// A clean merge-tree -z prints exactly a tree id and one NUL, and exits 0 (review I2).
func TestBehaviourMergeTreeCleanExitsZeroWithATree(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir, a, b := twoBranches(t, env, map[string]string{"a.txt": "a\n", "b.txt": "b\n"},
		map[string]string{"a.txt": "a2\n"}, map[string]string{"b.txt": "b2\n"})
	out, errs, code := run(t, dir, env, "merge-tree", "--write-tree", "--name-only", "-z", a, b)
	if code != 0 {
		t.Fatalf("clean merge-tree: exit %d, stderr %q", code, errs)
	}
	tree, rest, ok := strings.Cut(out, "\x00")
	if !ok || rest != "" || !hex40.MatchString(tree) {
		t.Errorf("clean merge-tree -z stdout = %q, want a 40-hex tree id and one NUL", out)
	}
}

// A conflicted merge-tree exits 1, which is not an error. Under -z it names each conflicted
// path once as raw bytes; without -z git quotes a non-ASCII path, which is why every phase 3
// call passes -z (review I2, re-review b05 nit).
func TestBehaviourMergeTreeConflictExitsOneWithPaths(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	const resume, spaced = "docs/résumé.md", "a b.txt"
	dir, a, b := twoBranches(t, env, map[string]string{resume: "base\n", spaced: "base\n"},
		map[string]string{resume: "A\n", spaced: "A\n"}, map[string]string{resume: "B\n", spaced: "B\n"})

	out, errs, code := run(t, dir, env, "merge-tree", "--write-tree", "--name-only", "-z", a, b)
	t.Logf("merge-tree -z: exit %d, stderr %q, stdout verbatim:\n%q", code, errs, out)
	if code != 1 {
		t.Fatalf("conflicted merge-tree -z: exit %d, want 1", code)
	}
	fields := strings.Split(out, "\x00")
	if !hex40.MatchString(fields[0]) {
		t.Errorf("first -z field %q is not a tree id", fields[0])
	}
	i := 1
	var paths []string
	for ; i < len(fields) && fields[i] != ""; i++ {
		paths = append(paths, fields[i])
	}
	slices.Sort(paths)
	if want := []string{spaced, resume}; !slices.Equal(paths, want) {
		t.Errorf("-z conflicted paths = %q, want each of %q once and unquoted", paths, want)
	}
	if i >= len(fields) || !strings.Contains(strings.Join(fields[i+1:], "\x00"), "CONFLICT") {
		t.Errorf("-z output has no empty field followed by CONFLICT messages: %q", out)
	}

	plain, _, code := run(t, dir, env, "merge-tree", "--write-tree", "--name-only", a, b)
	t.Logf("merge-tree without -z: exit %d, stdout verbatim:\n%s", code, plain)
	if quoted := "\n\"docs/r\\303\\251sum\\303\\251.md\"\n"; code != 1 || !strings.Contains(plain, quoted) {
		t.Errorf("merge-tree without -z: exit %d, stdout lacks the quoted line %q", code, quoted)
	}
}

// --- update-ref (tasks 09, 10, 11) --------------------------------------------------------

// chain makes a repository with n commits on main and returns it and the commits, oldest
// first.
func chain(t *testing.T, env gitEnv, n int) (string, []string) {
	t.Helper()
	dir := newRepo(t, env, map[string]string{"c.txt": "0\n"})
	commits := []string{line(t, dir, env, "rev-parse", "HEAD")}
	for len(commits) < n {
		commits = append(commits, commitFiles(t, dir, env, "next", map[string]string{"c.txt": strings.Repeat("x", len(commits)) + "\n"}))
	}
	return dir, commits
}

// update-ref with an old value that is not the ref's refuses and moves nothing (A5, R13).
func TestBehaviourUpdateRefCompareAndSwapRefuses(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir, c := chain(t, env, 3)
	must(t, dir, env, "branch", "b", c[2])
	_, errs, code := run(t, dir, env, "update-ref", "refs/heads/b", c[1], c[0])
	t.Logf("update-ref b NEW OLD with b at a third commit: exit %d, stderr verbatim:\n%s", code, errs)
	if code == 0 {
		t.Errorf("compare-and-swap update-ref exited 0")
	}
	if got := line(t, dir, env, "rev-parse", "refs/heads/b"); got != c[2] {
		t.Errorf("b moved to %s, want it left at %s", got, c[2])
	}
}

// update-ref with the zero id as old value creates only: it fails when the ref exists (11).
func TestBehaviourUpdateRefCreateOnlyRefusesAnExistingRef(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir, c := chain(t, env, 2)
	must(t, dir, env, "branch", "b", c[0])
	_, errs, code := run(t, dir, env, "update-ref", "refs/heads/b", c[1], strings.Repeat("0", 40))
	t.Logf("create-only update-ref of an existing b: exit %d, stderr verbatim:\n%s", code, errs)
	if code == 0 {
		t.Errorf("create-only update-ref of an existing ref exited 0")
	}
	if got := line(t, dir, env, "rev-parse", "refs/heads/b"); got != c[0] {
		t.Errorf("b moved to %s, want it left at %s", got, c[0])
	}
}

// --- commit-tree (tasks 08, 09, 10) -------------------------------------------------------

// A trailer commit-tree writes reads back through log's trailers placeholder, so a land's
// brief link is recoverable (tasks 09, 10).
func TestBehaviourCommitTreeTrailersReadBack(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir, c := chain(t, env, 1)
	msg := "Land feat/x\n\nPanacea-Brief: brief:b-42@6\n"
	id := line(t, dir, env, "commit-tree", c[0]+"^{tree}", "-p", c[0], "-m", msg)
	out := must(t, dir, env, "log", "-1", "--format=%(trailers:key=Panacea-Brief,valueonly)", id)
	t.Logf("log --format=%%(trailers:key=Panacea-Brief,valueonly) stdout verbatim: %q", out)
	if got := strings.TrimRight(out, "\n"); got != "brief:b-42@6" {
		t.Errorf("trailer read back as %q, want brief:b-42@6", got)
	}
}

// merge-tree with the merge as base, the later tip as ours and the merge's first parent as
// theirs gives the tip minus the merge's change: that is task 10's revert of a land.
func TestBehaviourMergeTreeWithMergeBaseRevertsAMerge(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir := newRepo(t, env, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	must(t, dir, env, "switch", "-q", "-c", "f")
	commitFiles(t, dir, env, "feature", map[string]string{"f.txt": "f\n"})
	must(t, dir, env, "switch", "-q", "main")
	must(t, dir, env, "merge", "-q", "--no-ff", "-m", "land f", "f")
	m := line(t, dir, env, "rev-parse", "HEAD")
	tip := commitFiles(t, dir, env, "later", map[string]string{"b.txt": "b2\n"})

	out, errs, code := run(t, dir, env, "merge-tree", "--write-tree", "--name-only", "-z",
		"--merge-base="+m, tip, m+"^1")
	if code != 0 {
		t.Fatalf("revert merge-tree: exit %d, stdout %q, stderr %q", code, out, errs)
	}
	tree, _, _ := strings.Cut(out, "\x00")
	var want []string
	for _, l := range strings.Split(must(t, dir, env, "ls-tree", "-r", tip), "\n") {
		if l != "" && !strings.HasSuffix(l, "\tf.txt") {
			want = append(want, l)
		}
	}
	got := strings.Split(strings.TrimSuffix(must(t, dir, env, "ls-tree", "-r", tree), "\n"), "\n")
	if !slices.Equal(got, want) {
		t.Errorf("revert tree = %q, want the tip's tree minus f.txt %q", got, want)
	}
}

// commit-tree with a fixed date is a pure function of its inputs, so a repeated forecast
// writes no new commit (decision 6).
func TestBehaviourFixedDateCommitTreeIsDeterministic(t *testing.T) {
	t.Parallel()
	env := hermetic(t).with("GIT_AUTHOR_DATE=@0 +0000", "GIT_COMMITTER_DATE=@0 +0000")
	dir, c := chain(t, env, 2)
	args := []string{"commit-tree", c[1] + "^{tree}", "-p", c[1], "-p", c[0], "-m", "forecast"}
	first := line(t, dir, env, args...)
	second := line(t, dir, env, args...)
	if first != second || !hex40.MatchString(first) {
		t.Errorf("two fixed-date commit-trees returned %s and %s, want one id", first, second)
	}
}

// --- Reads that must stay reads (tasks 05, 07, 08) ----------------------------------------

type fileState struct {
	bytes string
	mtime time.Time
}

func stateOf(t *testing.T, path string) fileState {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fileState{readFile(t, path), fi.ModTime()}
}

func (s fileState) same(o fileState) bool { return s.bytes == o.bytes && s.mtime.Equal(o.mtime) }

// status with GIT_OPTIONAL_LOCKS=0 does not refresh the index, so an observed-claims read
// writes nothing (task 07). The control shows a plain status does rewrite it.
func TestBehaviourStatusWithoutOptionalLocksLeavesTheIndex(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir, _ := chain(t, env, 1)
	index := filepath.Join(dir, ".git", "index")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "c.txt"), later, later); err != nil {
		t.Fatal(err)
	}
	before := stateOf(t, index)
	must(t, dir, env.with("GIT_OPTIONAL_LOCKS=0"), "status", "--porcelain=v2", "-z")
	if after := stateOf(t, index); !after.same(before) {
		t.Errorf("status with GIT_OPTIONAL_LOCKS=0 changed .git/index (mtime %v -> %v)", before.mtime, after.mtime)
	}
	must(t, dir, env, "status", "--porcelain=v2", "-z")
	after := stateOf(t, index)
	t.Logf("control, plain status after the touch: index rewritten = %v", !after.same(before))
	if after.same(before) {
		t.Errorf("control: a plain status left .git/index alone, so this pin proves nothing")
	}
}

// The environment's core.hooksPath wins for git's own use, while --local still reads the
// repository's own value, so the dispatcher can find the repository's hooks (task 05).
func TestBehaviourLocalConfigHooksPathIgnoresEnvironment(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir, _ := chain(t, env, 1)
	must(t, dir, env, "config", "core.hooksPath", ".husky/_")
	withEnv := env.with("GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=/x")
	if got := line(t, dir, withEnv, "config", "--local", "--get", "core.hooksPath"); got != ".husky/_" {
		t.Errorf("config --local --get core.hooksPath under the env = %q, want .husky/_", got)
	}
	if got := line(t, dir, withEnv, "config", "--get", "core.hooksPath"); got != "/x" {
		t.Errorf("config --get core.hooksPath under the env = %q, want the env's /x", got)
	}
}

// writeHook writes an executable hook script into dir/.git/hooks.
func writeHook(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, ".git", "hooks", name)
	writeFile(t, p, "#!/bin/sh\n"+body)
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

// prepare-commit-msg runs under --no-verify, which skips pre-commit and commit-msg only, so
// the session trailer survives an agent's --no-verify (task 05).
func TestBehaviourPrepareCommitMsgRunsUnderNoVerify(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir, _ := chain(t, env, 1)
	writeHook(t, dir, "prepare-commit-msg", "printf '\\nPanacea-Session: s-1\\n' >> \"$1\"\n")
	writeHook(t, dir, "pre-commit", "exit 1\n")
	writeHook(t, dir, "commit-msg", "exit 1\n")
	must(t, dir, env, "commit", "-q", "--no-verify", "--allow-empty", "-m", "x")
	msg := must(t, dir, env, "log", "-1", "--format=%B")
	if !strings.Contains(msg, "Panacea-Session: s-1") {
		t.Errorf("commit --no-verify message = %q, want the prepare-commit-msg line", msg)
	}
}

// rev-list --left-right --count prints left, a tab, right and a newline (task 08's
// ahead/behind). The counts differ, so a swapped parse would show.
func TestBehaviourRevListLeftRightCountFormat(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir, _, b := twoBranches(t, env, map[string]string{"a.txt": "a\n"},
		map[string]string{"a.txt": "a2\n"}, map[string]string{"b.txt": "b\n"})
	b2 := line(t, dir, env, "commit-tree", b+"^{tree}", "-p", b, "-m", "b again")
	must(t, dir, env, "update-ref", "refs/heads/B", b2, b)
	if out := must(t, dir, env, "rev-list", "--left-right", "--count", "A...B"); out != "1\t2\n" {
		t.Errorf("rev-list --left-right --count A...B = %q, want %q", out, "1\t2\n")
	}
}

// A clone does not inherit the source's .git/config identity, so task 03 copies it.
func TestBehaviourGitVarInACloneMissesTheSourcesLocalIdentity(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	src, _ := chain(t, env, 1)
	must(t, src, env, "config", "user.name", "Src Name")
	must(t, src, env, "config", "user.email", "src@example.invalid")
	bare := env.without("GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL")
	if got := line(t, src, bare, "var", "GIT_AUTHOR_IDENT"); !strings.HasPrefix(got, "Src Name <src@example.invalid> ") {
		t.Errorf("control: git var in the source = %q, want its local identity", got)
	}
	dst := clone(t, env, src, "--no-hardlinks")
	out, errs, code := run(t, dst, bare, "var", "GIT_AUTHOR_IDENT")
	t.Logf("git var GIT_AUTHOR_IDENT in the clone: exit %d, stdout %q, stderr %q", code, out, errs)
	if strings.Contains(out, "Src Name") || strings.Contains(out, "src@example.invalid") {
		t.Errorf("the clone's git var printed the source's local identity: %q", out)
	}
}

// fetch --no-write-fetch-head from a bundle file (R4.4: the host fetches lanes from bundles)
// leaves FETCH_HEAD as it was; a plain fetch origin writes it (tasks 07, 08, 12). Every fetch
// passes --no-auto-maintenance, as every phase 3 host fetch does, so no detached maintenance
// outlives the test.
func TestBehaviourNoWriteFetchHeadLeavesFetchHeadAlone(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	upstream, _ := chain(t, env, 1)
	project := clone(t, env, upstream, "--no-hardlinks")
	scratch := clone(t, env, project, "--no-hardlinks")
	must(t, scratch, env, "switch", "-q", "-c", "b")
	lane := commitFiles(t, scratch, env, "lane work", map[string]string{"w.txt": "w\n"})
	bundle := filepath.Join(t.TempDir(), "lane.bundle")
	must(t, scratch, env, "bundle", "create", "-q", bundle, "refs/heads/b")
	fetchHead := filepath.Join(project, ".git", "FETCH_HEAD")
	laneFetch := []string{"fetch", "-q", "--no-auto-maintenance", "--no-write-fetch-head", bundle, "+refs/heads/b:refs/panacea/lanes/w"}

	must(t, project, env, laneFetch...)
	if _, err := os.Stat(fetchHead); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a --no-write-fetch-head bundle fetch into a fresh clone left FETCH_HEAD (stat err %v)", err)
	}
	if got := line(t, project, env, "rev-parse", "refs/panacea/lanes/w"); got != lane {
		t.Errorf("refs/panacea/lanes/w = %s, want the lane's %s", got, lane)
	}
	must(t, project, env, "fetch", "-q", "--no-auto-maintenance", "origin")
	if _, err := os.Stat(fetchHead); err != nil {
		t.Fatalf("control: a plain fetch origin wrote no FETCH_HEAD (%v)", err)
	}
	before := stateOf(t, fetchHead)
	must(t, project, env, laneFetch...)
	if after := stateOf(t, fetchHead); !after.same(before) {
		t.Errorf("a --no-write-fetch-head bundle fetch changed FETCH_HEAD: %q -> %q", before.bytes, after.bytes)
	}
}

// --- Branch names and branch config (task 11) --------------------------------------------

// gitCase is one git call and what it must answer: exit code, and a stdout and stderr that
// must equal (stdout) or contain (stderr) the given text. "" in stderr means empty.
type gitCase struct {
	args           []string
	code           int
	stdout, stderr string
}

func checkCases(t *testing.T, dir string, env gitEnv, cases []gitCase) {
	t.Helper()
	for _, c := range cases {
		out, errs, code := run(t, dir, env, c.args...)
		t.Logf("git %s: exit %d, stdout %q, stderr verbatim %q", strings.Join(c.args, " "), code, out, errs)
		if code != c.code || out != c.stdout ||
			(c.stderr == "" && errs != "") || !strings.Contains(errs, c.stderr) {
			t.Errorf("git %s: exit %d, stdout %q, stderr %q; want exit %d, stdout %q, stderr holding %q",
				strings.Join(c.args, " "), code, out, errs, c.code, c.stdout, c.stderr)
		}
	}
}

// check-ref-format --branch expands @{-1} and refuses -x and HEAD, which the plain form
// accepts, so ValidBranchName needs --branch and the printed-name equality both (Minor 6).
func TestBehaviourCheckRefFormatBranchExpandsShorthand(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir, _ := chain(t, env, 1)
	must(t, dir, env, "switch", "-q", "-c", "feature")
	must(t, dir, env, "switch", "-q", "main")
	checkCases(t, dir, env, []gitCase{
		{args: []string{"check-ref-format", "--branch", "@{-1}"}, code: 0, stdout: "feature\n"},
		{args: []string{"check-ref-format", "refs/heads/@{-1}"}, code: 1},
		{args: []string{"update-ref", "refs/heads/@{-1}", "HEAD", strings.Repeat("0", 40)}, code: 128,
			stderr: "refusing to update ref with bad name"},
		{args: []string{"check-ref-format", "--branch", "-x"}, code: 128, stderr: "fatal: '-x' is not a valid branch name"},
		{args: []string{"check-ref-format", "--branch", "HEAD"}, code: 128, stderr: "fatal: 'HEAD' is not a valid branch name"},
		{args: []string{"check-ref-format", "refs/heads/-x"}, code: 0},
		{args: []string{"check-ref-format", "refs/heads/HEAD"}, code: 0},
		{args: []string{"check-ref-format", "refs/heads/a..b"}, code: 1},
	})
}

// remove-section's exit codes, and branch -D's answer when it cannot rewrite the config:
// DeleteBranch's OKExit 128 and its warn-and-answer rule rest on them (Minor 12).
func TestBehaviourRemoveSectionExitCodes(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir, _ := chain(t, env, 1)
	for _, b := range []string{"x", "z"} {
		must(t, dir, env, "branch", b)
		must(t, dir, env, "config", "branch."+b+".remote", "origin")
	}
	must(t, dir, env, "config", "branch.y.remote", "origin")
	checkCases(t, dir, env, []gitCase{
		{args: []string{"config", "--remove-section", "branch.nope"}, code: 128, stderr: "fatal: no such section: branch.nope"},
		{args: []string{"config", "--remove-section", "branch.z"}, code: 0},
	})
	writeFile(t, filepath.Join(dir, ".git", "config.lock"), "")
	checkCases(t, dir, env, []gitCase{
		{args: []string{"config", "--remove-section", "branch.y"}, code: 255, stderr: "error: could not lock config file .git/config"},
	})
	out, errs, code := run(t, dir, env, "branch", "-D", "x")
	t.Logf("branch -D x with .git/config.lock held: exit %d, stdout %q, stderr verbatim %q", code, out, errs)
	if code != 0 || !strings.Contains(errs, "warning: update of config-file failed") {
		t.Errorf("branch -D x with the config locked: exit %d, stderr %q; want 0 and the config-file warning", code, errs)
	}
	if cfg := readFile(t, filepath.Join(dir, ".git", "config")); !strings.Contains(cfg, "[branch \"x\"]") {
		t.Errorf("branch -D x with the config locked removed branch.x anyway:\n%s", cfg)
	}
	if _, _, code := run(t, dir, env, "rev-parse", "--verify", "-q", "refs/heads/x"); code == 0 {
		t.Errorf("branch -D x exited 0 but refs/heads/x is still there")
	}
}

// branch -m renames the checked-out branch silently and moves HEAD with it, so RenameBranch
// must refuse branch_checked_out itself (Minor 16).
func TestBehaviourBranchRenameMovesACheckoutsHead(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	dir, _ := chain(t, env, 1)
	checkCases(t, dir, env, []gitCase{
		{args: []string{"branch", "-m", "main", "renamed"}, code: 0},
		{args: []string{"symbolic-ref", "HEAD"}, code: 0, stdout: "refs/heads/renamed\n"},
	})
}

// --- Maintenance and removal (tasks 07, 09, 11, 12) ---------------------------------------

// A fetch starts git maintenance --auto unless --no-auto-maintenance is passed, even when the
// repository's own config asks for it, so every host fetch needs the flag (Minor 8).
func TestBehaviourFetchStartsAutoMaintenance(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	src, _ := chain(t, env, 1)
	dst := filepath.Join(t.TempDir(), "dst")
	must(t, t.TempDir(), env, "init", "-q", "-b", "main", dst)
	traced := env.with("GIT_TRACE=1")
	const started = "run_command: git maintenance run --auto --quiet --detach"

	_, errs, code := run(t, dst, traced, "fetch", "--no-tags", "--quiet", src, "+refs/heads/main:refs/x")
	if code != 0 || !strings.Contains(errs, started) {
		t.Errorf("fetch under GIT_TRACE=1: exit %d, trace lacks %q:\n%s", code, started, errs)
	}
	must(t, dst, env, "config", "maintenance.auto", "true")
	_, errs, code = run(t, dst, traced, "fetch", "--no-tags", "--quiet", "--no-auto-maintenance", src, "+refs/heads/main:refs/y")
	// "maintenance run", not "maintenance": the trace echoes the argv's --no-auto-maintenance.
	if code != 0 || strings.Contains(errs, "maintenance run") {
		t.Errorf("fetch --no-auto-maintenance with maintenance.auto=true: exit %d, trace runs maintenance:\n%s", code, errs)
	}
}

// os.RemoveAll on a clone: Git for Windows writes object files read-only. Record whether
// RemoveAll copes, and prove the fix (clear the read-only bit during a walk) if it does not.
func TestBehaviourRemoveAllOnAClone(t *testing.T) {
	t.Parallel()
	env := hermetic(t)
	src, _, pack := objectsFixture(t, env)
	dst := clone(t, env, src, "--no-hardlinks")
	if fi, err := os.Stat(filepath.Join(dst, pack)); err == nil {
		t.Logf("a clone's pack file mode as git wrote it: %v", fi.Mode())
	}
	// Force the case the plan names: a read-only object file (on Windows, os.Chmod 0o444 sets
	// FILE_ATTRIBUTE_READONLY), whether or not this git wrote one.
	if err := os.Chmod(filepath.Join(dst, pack), 0o444); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(dst, pack)); err != nil || fi.Mode().Perm()&0o200 != 0 {
		t.Fatalf("fixture: the pack is not read-only (stat %v, err %v)", fi, err)
	}
	err := os.RemoveAll(dst)
	t.Logf("os.RemoveAll(clone) on %s: err = %v", runtime.GOOS, err)
	if err != nil {
		walkErr := filepath.WalkDir(dst, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			mode := os.FileMode(0o600)
			if d.IsDir() {
				mode = 0o700
			}
			return os.Chmod(p, mode)
		})
		if walkErr != nil {
			t.Fatalf("clearing read-only bits: %v", walkErr)
		}
		if err := os.RemoveAll(dst); err != nil {
			t.Fatalf("os.RemoveAll after clearing read-only bits: %v", err)
		}
		t.Logf("os.RemoveAll succeeded after os.Chmod(p, 0o600) over the tree")
	}
	if _, err := os.Stat(dst); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the clone is still there after RemoveAll (stat err %v)", err)
	}
}
