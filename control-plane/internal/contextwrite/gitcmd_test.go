package contextwrite_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/contextwrite"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// recordingRunner is a Runner that records every argv and answers out/err.
type recordingRunner struct {
	mu   sync.Mutex
	out  map[string]string // by subcommand
	err  error
	argv [][]string
	dirs []string
}

func (r *recordingRunner) Run(_ context.Context, dir string, args ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.argv = append(r.argv, append([]string(nil), args...))
	r.dirs = append(r.dirs, dir)
	if r.err != nil {
		return nil, r.err
	}
	return []byte(r.out[subcommand(args)]), nil
}

// subcommand is the first argument that is neither an option nor an
// option's value: "-c" takes the next argument.
func subcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-c":
			i++
		case strings.HasPrefix(args[i], "-"):
		default:
			return args[i]
		}
	}
	return ""
}

// Review I-1: every command carries -c core.fsmonitor=false, first, and the
// argv is exactly the contract's. The subcommands are the three read-only
// ones; none writes.
func TestEveryGitQuestionDisablesTheRepositorysFsmonitor(t *testing.T) {
	r := &recordingRunner{out: map[string]string{
		"ls-files":     "",
		"rev-parse":    ".git/info/exclude\n",
		"check-ignore": ".git/info/exclude:7:/CLAUDE.md\tCLAUDE.md\n",
	}}
	g := contextwrite.NewGit(r)
	ctx := context.Background()
	if _, err := g.Tracked(ctx, "/w", contextwrite.FileName); err != nil {
		t.Fatal(err)
	}
	if _, err := g.ExcludePath(ctx, "/w"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Ignored(ctx, "/w", contextwrite.FileName); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-c core.fsmonitor=false --icase-pathspecs ls-files -- CLAUDE.md",
		"-c core.fsmonitor=false rev-parse --git-path info/exclude",
		"-c core.fsmonitor=false check-ignore -v -- CLAUDE.md",
	}
	if len(r.argv) != len(want) {
		t.Fatalf("argv = %q, want %q", r.argv, want)
	}
	for i, a := range r.argv {
		if got := strings.Join(a, " "); got != want[i] {
			t.Errorf("command %d = %q, want %q", i, got, want[i])
		}
		if r.dirs[i] != "/w" {
			t.Errorf("command %d ran in %q, want the worktree as Run's dir", i, r.dirs[i])
		}
		switch sub := subcommand(a); sub {
		case "ls-files", "rev-parse", "check-ignore":
		default:
			t.Errorf("command %d runs %q, which is not one of the three read-only questions", i, sub)
		}
	}
}

func TestGitQuestionsReadTheirOutput(t *testing.T) {
	ctx := context.Background()
	for _, row := range []struct {
		lsFiles string
		want    bool
	}{{"", false}, {"\n", false}, {"CLAUDE.md\n", true}, {"claude.md\n", true}} {
		g := contextwrite.NewGit(&recordingRunner{out: map[string]string{"ls-files": row.lsFiles}})
		if got, err := g.Tracked(ctx, "/w", contextwrite.FileName); err != nil || got != row.want {
			t.Errorf("ls-files printing %q: Tracked = %v, %v; want %v, nil", row.lsFiles, got, err, row.want)
		}
	}
	for _, out := range []string{"", "\n", "a\nb\n"} {
		g := contextwrite.NewGit(&recordingRunner{out: map[string]string{"rev-parse": out}})
		if p, err := g.ExcludePath(ctx, "/w"); err == nil {
			t.Errorf("rev-parse printing %q: ExcludePath = %q, want an error", out, p)
		}
	}
	g := contextwrite.NewGit(&recordingRunner{out: map[string]string{"rev-parse": "C:/r/.git/info/exclude\r\n"}})
	if p, err := g.ExcludePath(ctx, "/w"); err != nil || p != "C:/r/.git/info/exclude" {
		t.Errorf("ExcludePath = %q, %v", p, err)
	}
}

// check-ignore -v's line, read (review m1).
func TestCheckIgnoreOutputIsRead(t *testing.T) {
	ctx := context.Background()
	for _, row := range []struct {
		rel, out string
		want     bool
		wantErr  bool
	}{
		{"CLAUDE.md", ".git/info/exclude:7:/CLAUDE.md\tCLAUDE.md\n", true, false},
		{".claude/CLAUDE.md", ".git/info/exclude:8:/.claude/CLAUDE.md\t.claude/CLAUDE.md\n", true, false},
		{"CLAUDE.md", ".gitignore:1:!CLAUDE.md\tCLAUDE.md\n", false, false},
		{"CLAUDE.md", ".gitignore:1:!*.md\tCLAUDE.md\n", false, false},
		{"CLAUDE.md", "C:/Users/p/.config/git/ignore:3:*.md\tCLAUDE.md\n", true, false},
		{"CLAUDE.md", "C:/Users/p/.config/git/ignore:3:!*.md\tCLAUDE.md\r\n", false, false},
		{"CLAUDE.md", ".gitignore:12:a:1:!b\tCLAUDE.md\n", true, false}, // the pattern holds a colon
		{".claude/CLAUDE.md", ".claude/.gitignore:2:!CLAUDE.md\t.claude/CLAUDE.md\n", false, false},
		{"CLAUDE.md", "", false, true},
		{"CLAUDE.md", "::\tCLAUDE.md\n", false, true},
		{"CLAUDE.md", ".gitignore:1:\tCLAUDE.md\n", false, true},
		{"CLAUDE.md", ".gitignore:1:*.md\tOTHER.md\n", false, true},
		{"CLAUDE.md", ".gitignore:1:*.md\tCLAUDE.md\n.gitignore:2:x\tCLAUDE.md\n", false, true},
		{"CLAUDE.md", ".gitignore:x:*.md\tCLAUDE.md\n", false, true},
	} {
		g := contextwrite.NewGit(&recordingRunner{out: map[string]string{"check-ignore": row.out}})
		got, err := g.Ignored(ctx, "/w", row.rel)
		if row.wantErr {
			if err == nil {
				t.Errorf("%q: Ignored = %v, nil; want an error", row.out, got)
			}
			continue
		}
		if err != nil || got != row.want {
			t.Errorf("%q: Ignored = %v, %v; want %v, nil", row.out, got, err, row.want)
		}
	}
}

func TestGitErrorsAreNotARepoOrReturned(t *testing.T) {
	ctx := context.Background()
	for _, row := range []struct {
		err      error
		notARepo bool
	}{
		{errors.New("git ls-files in /w: exit status 128: fatal: not a git repository (or any of the parent directories): .git"), true},
		{errors.New("git ls-files in /w: exit status 128: fatal: index file corrupt"), false},
	} {
		g := contextwrite.NewGit(&recordingRunner{err: row.err})
		tr, err1 := g.Tracked(ctx, "/w", contextwrite.FileName)
		_, err2 := g.ExcludePath(ctx, "/w")
		ig, err3 := g.Ignored(ctx, "/w", contextwrite.FileName)
		for i, err := range []error{err1, err2, err3} {
			if err == nil {
				t.Fatalf("question %d: nil error from a failing runner (Tracked %v, Ignored %v)", i, tr, ig)
			}
			if errors.Is(err, contextwrite.ErrNotARepo) != row.notARepo {
				t.Errorf("question %d: %v; ErrNotARepo = %v, want %v", i, err, !row.notARepo, row.notARepo)
			}
		}
	}
}

// ── over real git ───────────────────────────────────────────────────────────

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
}

// isolateGit neutralises the developer's own git config for this test, for
// the fixture and for the Runner under test alike (both read os.Environ).
func isolateGit(t *testing.T) {
	t.Helper()
	requireGit(t)
	cfg := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(cfg, "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(cfg, "gitconfig-system"))
	t.Setenv("GIT_AUTHOR_NAME", "test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}

// fixture runs one git command line in dir, to BUILD a test repository. The
// package under test runs no such command (TestEveryGitQuestion...).
func fixture(t *testing.T, dir, cmdline string) string {
	t.Helper()
	cmd := exec.Command("git", strings.Fields(cmdline)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", cmdline, err, out)
	}
	return string(out)
}

// newRepo is a real repository in a fresh directory, with one commit.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	fixture(t, dir, "init -q")
	writeFile(t, filepath.Join(dir, "README.md"), "x\n")
	fixture(t, dir, "add -- README.md")
	fixture(t, dir, "commit -q -m initial")
	return dir
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

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	writeFile(t, path, string(b)+content)
}

func TestGitQuestionsOverARealRepository(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	g := contextwrite.NewGit(project.NewExecRunner())

	dir := newRepo(t)
	if got, err := g.Tracked(ctx, dir, contextwrite.FileName); err != nil || got {
		t.Fatalf("Tracked(CLAUDE.md), none anywhere = %v, %v; want false, nil", got, err)
	}
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "mine\n")
	if got, err := g.Tracked(ctx, dir, contextwrite.FileName); err != nil || got {
		t.Fatalf("Tracked(CLAUDE.md), untracked on disk = %v, %v; want false, nil", got, err)
	}

	lower := newRepo(t)
	writeFile(t, filepath.Join(lower, "claude.md"), "lower case\n")
	fixture(t, lower, "add -- claude.md")
	fixture(t, lower, "commit -q -m lower")
	if got, err := g.Tracked(ctx, lower, contextwrite.FileName); err != nil || !got {
		t.Fatalf("Tracked(CLAUDE.md) with claude.md committed = %v, %v; want true (re-review R-I1)", got, err)
	}
	if got, err := g.Tracked(ctx, lower, contextwrite.AltFileName); err != nil || got {
		t.Fatalf("Tracked(.claude/CLAUDE.md) = %v, %v; want false, nil", got, err)
	}

	p, err := g.ExcludePath(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	if !strings.HasSuffix(filepath.ToSlash(p), ".git/info/exclude") {
		t.Fatalf("ExcludePath = %q, want the clone's .git/info/exclude", p)
	}

	// Ignored, asked after the line (review m1).
	appendFile(t, p, "/CLAUDE.md\n")
	if got, err := g.Ignored(ctx, dir, contextwrite.FileName); err != nil || !got {
		t.Fatalf("Ignored(CLAUDE.md) with /CLAUDE.md excluded = %v, %v; want true, nil", got, err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := g.Ignored(ctx, sub, contextwrite.FileName); err == nil {
		t.Fatalf("Ignored from a subdirectory = %v, nil; want an error: the anchored line matches nothing there", got)
	}
	writeFile(t, filepath.Join(dir, ".gitignore"), "!CLAUDE.md\n")
	if got, err := g.Ignored(ctx, dir, contextwrite.FileName); err != nil || got {
		t.Fatalf("Ignored(CLAUDE.md) under a .gitignore's !CLAUDE.md = %v, %v; want false, nil", got, err)
	}

	plain := filepath.Join(t.TempDir(), "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(plain))
	_, err1 := g.Tracked(ctx, plain, contextwrite.FileName)
	_, err2 := g.ExcludePath(ctx, plain)
	_, err3 := g.Ignored(ctx, plain, contextwrite.FileName)
	for i, err := range []error{err1, err2, err3} {
		if !errors.Is(err, contextwrite.ErrNotARepo) {
			t.Errorf("question %d outside every repository: %v, want ErrNotARepo", i, err)
		}
	}
}

// Review I-1, over real git: a repository-local core.fsmonitor, which the
// agent can write, is never started by the three questions. The control
// proves this git runs the hook at all, so the assertion cannot pass
// vacuously.
func TestNoGitQuestionStartsTheRepositorysFsmonitor(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	dir := newRepo(t)
	marker := filepath.Join(t.TempDir(), "fsmonitor-ran")
	hook := filepath.Join(t.TempDir(), "fsmonitor.sh")
	writeFile(t, hook, "#!/bin/sh\necho ran >> '"+filepath.ToSlash(marker)+"'\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "config", "core.fsmonitor", filepath.ToSlash(hook))
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}
	r := project.NewExecRunner()
	if _, err := r.Run(ctx, dir, "ls-files", "--", "CLAUDE.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Skipf("this git did not start a repository-local core.fsmonitor for a plain ls-files (%v); the hook cannot be observed here", err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}

	g := contextwrite.NewGit(r)
	for _, rel := range []string{contextwrite.FileName, contextwrite.AltFileName} {
		if _, err := g.Tracked(ctx, dir, rel); err != nil {
			t.Fatal(err)
		}
	}
	p, err := g.ExcludePath(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	appendFile(t, p, "/CLAUDE.md\n")
	if _, err := g.Ignored(ctx, dir, contextwrite.FileName); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a Git question started the repository's core.fsmonitor on the host (review I-1)")
	}
}

// 3-D6 over real git and NewGit: after a start, git status lists nothing -
// not the brief's file, and not a temp file a crash between WriteAtomic's
// CreateTemp and its rename would leave, holding the brief (review m2). A
// .gitignore that re-includes CLAUDE.md, which outranks info/exclude, refuses
// the start with nothing written (review m1).
func TestRealGitNeverListsTheBrief(t *testing.T) {
	isolateGit(t)
	ctx := context.Background()
	start := func(dir string) (string, error) {
		src := &fakeSource{}
		src.set(inh(doc(idA, 6, "the brief")), nil)
		m := contextwrite.New(src, contextwrite.NewGit(project.NewExecRunner()), nil)
		res, err := m.Materialise(ctx, api.MaterialiseRequest{WorktreeID: "wt-1", WorktreePath: dir, ProjectID: "pr-1", BranchID: "br-1"})
		return res.ContextRef, err
	}
	for _, row := range []struct {
		name          string
		claudeTracked bool
		target        string
		debris        string
	}{
		{"CLAUDE.md", false, "CLAUDE.md", ".CLAUDE-4242.md"},
		{".claude/CLAUDE.md", true, ".claude/CLAUDE.md", ".claude/.CLAUDE-77.md"},
	} {
		t.Run(row.name, func(t *testing.T) {
			dir := newRepo(t)
			if row.claudeTracked {
				writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# Repository text\n")
				fixture(t, dir, "add -- CLAUDE.md")
				fixture(t, dir, "commit -q -m claude")
			}
			key, err := start(dir)
			if err != nil {
				t.Fatal(err)
			}
			if key != doc(idA, 6, "").Key {
				t.Fatalf("key = %q", key)
			}
			b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(row.target)))
			if err != nil || !strings.Contains(string(b), claudemd.BeginPrefix) {
				t.Fatalf("%s = %q, %v; want the block", row.target, b, err)
			}
			writeFile(t, filepath.Join(dir, filepath.FromSlash(row.debris)), string(b))
			if s := fixture(t, dir, "status --porcelain --untracked-files=all"); s != "" {
				t.Fatalf("git status lists %q: an agent's git add -A would commit the brief (3-D6)", s)
			}
		})
	}
	t.Run("a .gitignore re-includes CLAUDE.md", func(t *testing.T) {
		dir := newRepo(t)
		writeFile(t, filepath.Join(dir, ".gitignore"), "!CLAUDE.md\n")
		fixture(t, dir, "add -- .gitignore")
		fixture(t, dir, "commit -q -m ignore")
		_, err := start(dir)
		wantReason(t, err, api.BriefReasonWriteFailed)
		if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("CLAUDE.md exists (%v): the brief landed in a file git lists", err)
		}
	})
}
