package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// eolRunner is a project.Runner double that answers each git invocation from
// a script keyed by its arguments, and records every one.
type eolRunner struct {
	mu      sync.Mutex
	calls   []identityCall
	answers map[string]eolAnswer
}

type eolAnswer struct {
	out string
	err error
}

func (r *eolRunner) Run(_ context.Context, dir string, args ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, identityCall{dir: dir, args: append([]string(nil), args...)})
	a, ok := r.answers[strings.Join(args, " ")]
	if !ok {
		return nil, errors.New("eolRunner: no answer scripted for git " + strings.Join(args, " "))
	}
	if a.err != nil {
		return nil, a.err
	}
	return []byte(a.out), nil
}

var _ project.Runner = (*eolRunner)(nil)

const (
	eolAskBool = "config --get --type=bool --default false core.autocrlf"
	eolAskRaw  = "config --get core.autocrlf"
	eolAskEOL  = "config --get --default native core.eol"
)

// git's refusal of a value that is not a boolean, as the runner reports it.
var eolNotABool = errors.New("git config --get --type=bool --default false core.autocrlf in C:\\w: exit status 128: fatal: bad boolean config value 'input' for 'core.autocrlf'")

// Each answer is asked of git in the worktree, and each value reaches the
// sandbox the way the host's git reads it. native is both hosts' in turn, so a
// resolver that ignored its parameter passes one of them at most.
func TestTheLineEndingsAreTheHostGitsOwnAnswerInTheWorktree(t *testing.T) {
	for _, tc := range []struct {
		name    string
		answers map[string]eolAnswer
		native  string
		want    api.GitLineEndings
	}{
		{"git for windows' installer default", map[string]eolAnswer{eolAskBool: {out: "true\n"}, eolAskEOL: {out: "native\n"}},
			"crlf", api.GitLineEndings{AutoCRLF: "true", EOL: "crlf"}},
		{"unset on windows", map[string]eolAnswer{eolAskBool: {out: "false\n"}, eolAskEOL: {out: "native\n"}},
			"crlf", api.GitLineEndings{AutoCRLF: "false", EOL: "crlf"}},
		{"unset on linux", map[string]eolAnswer{eolAskBool: {out: "false\n"}, eolAskEOL: {out: "native\n"}},
			"lf", api.GitLineEndings{AutoCRLF: "false", EOL: "lf"}},
		{"input", map[string]eolAnswer{eolAskBool: {err: eolNotABool}, eolAskRaw: {out: "InPut\n"}, eolAskEOL: {out: "native\n"}},
			"crlf", api.GitLineEndings{AutoCRLF: "input", EOL: "crlf"}},
		{"an explicit eol, in git's case-blind spelling", map[string]eolAnswer{eolAskBool: {out: "false\n"}, eolAskEOL: {out: "LF\n"}},
			"crlf", api.GitLineEndings{AutoCRLF: "false", EOL: "lf"}},
		{"crlf on linux", map[string]eolAnswer{eolAskBool: {out: "false\n"}, eolAskEOL: {out: "crlf\n"}},
			"lf", api.GitLineEndings{AutoCRLF: "false", EOL: "crlf"}},
		// git reads a core.eol it does not know as native, not as an error.
		{"an eol git does not know", map[string]eolAnswer{eolAskBool: {out: "false\n"}, eolAskEOL: {out: "unix\n"}},
			"crlf", api.GitLineEndings{AutoCRLF: "false", EOL: "crlf"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &eolRunner{answers: tc.answers}
			got, err := gitLineEndingsResolver(host, tc.native)(context.Background(), `C:\work\evidence-upload`)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if got != tc.want {
				t.Errorf("line endings = %#v, want %#v", got, tc.want)
			}
			for _, c := range host.calls {
				if c.dir != `C:\work\evidence-upload` {
					t.Errorf("git %q ran in %q, want the worktree", strings.Join(c.args, " "), c.dir)
				}
			}
		})
	}
}

// A core.autocrlf the host's git would die on is an error, carrying git's own
// words, and so is a git that cannot answer at all.
func TestALineEndingSettingTheHostsGitCannotReadIsAnError(t *testing.T) {
	gitDied := errors.New("git config --get core.eol in C:\\w: exit status 128: fatal: bad config line 3 in file .git/config")
	for name, answers := range map[string]map[string]eolAnswer{
		"autocrlf is not a word git knows": {eolAskBool: {err: errors.New("exit status 128: fatal: bad boolean config value 'maybe' for 'core.autocrlf'")}, eolAskRaw: {out: "maybe\n"}, eolAskEOL: {out: "native\n"}},
		"the raw read fails too":           {eolAskBool: {err: eolNotABool}, eolAskRaw: {err: gitDied}, eolAskEOL: {out: "native\n"}},
		"core.eol cannot be read":          {eolAskBool: {out: "true\n"}, eolAskEOL: {err: gitDied}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := gitLineEndingsResolver(&eolRunner{answers: answers}, "crlf")(context.Background(), `C:\w`)
			if err == nil {
				t.Fatalf("resolve returned %#v and no error", got)
			}
			if got != (api.GitLineEndings{}) {
				t.Errorf("line endings = %#v alongside an error, want none", got)
			}
			if !strings.Contains(err.Error(), "exit status 128") {
				t.Errorf("error %q does not carry git's own words", err)
			}
		})
	}
}

// native is CRLF to Git for Windows and LF to git everywhere else. CI runs on
// Linux and the owner on Windows, so each run pins its own half.
func TestNativeIsCRLFOnWindowsAndLFElsewhere(t *testing.T) {
	want := "lf"
	if runtime.GOOS == "windows" {
		want = "crlf"
	}
	if got := hostNativeEOL(); got != want {
		t.Errorf("hostNativeEOL() on %s = %q, want %q", runtime.GOOS, got, want)
	}
}

// Against the real git: the answer includes the SYSTEM scope, which is where
// Git for Windows puts core.autocrlf=true and which no container sees, and a
// repository's own setting beats it, as it does on the host.
func TestTheRealHostGitAnswersFromEveryScope(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	system := filepath.Join(dir, "gitconfig-system")
	if err := os.WriteFile(system, []byte("[core]\n\tautocrlf = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "")
	t.Setenv("GIT_CONFIG_SYSTEM", system)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "gitconfig-global"))
	t.Setenv("GIT_CONFIG_COUNT", "")
	repo := filepath.Join(dir, "repo")
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	resolve := gitLineEndingsResolver(project.NewExecRunner(), "crlf")

	got, err := resolve(context.Background(), repo)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := (api.GitLineEndings{AutoCRLF: "true", EOL: "crlf"}); got != want {
		t.Errorf("with system core.autocrlf=true: %#v, want %#v", got, want)
	}

	for _, args := range [][]string{
		{"-C", repo, "config", "core.autocrlf", "input"},
		{"-C", repo, "config", "core.eol", "lf"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	got, err = resolve(context.Background(), repo)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := (api.GitLineEndings{AutoCRLF: "input", EOL: "lf"}); got != want {
		t.Errorf("with the repository's own settings: %#v, want %#v", got, want)
	}
}
