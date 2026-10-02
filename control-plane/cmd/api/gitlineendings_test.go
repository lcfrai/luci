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

	"github.com/lcfrai/luci/control-plane/internal/api"
	"github.com/lcfrai/luci/control-plane/internal/project"
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

// hostAsk is one host-config question: `git config --get <scope> --includes
// <rest...>`, as hostGit.config asks it.
func hostAsk(scope string, rest ...string) string {
	return strings.Join(append([]string{"config", "--get", scope, "--includes"}, rest...), " ")
}

// hostProbe is hostGit.scope's question for key.
func hostProbe(key string) string { return hostAsk("--global", "--default", hostGitUnset, key) }

// scriptedHost is a hostGit whose git answers from answers, each git in a
// directory hostGit makes for it in the OS temp directory.
func scriptedHost(r *eolRunner, getenv func(string) string) hostGit {
	return hostGit{run: r, root: os.TempDir(), getenv: getenv}
}

// madeByHostGit is whether dir is a directory hostGit.git made for one git
// under a scripted host: a direct child of the OS temp directory, named by
// hostGitPattern.
func madeByHostGit(dir string) bool {
	return filepath.Dir(dir) == filepath.Clean(os.TempDir()) && strings.HasPrefix(filepath.Base(dir), hostGitPattern)
}

// Neither key set in the global scope, so each is read from the system scope.
var (
	eolAskBool = hostAsk("--system", "--type=bool", "--default", "false", "core.autocrlf")
	eolAskRaw  = hostAsk("--system", "core.autocrlf")
	eolAskEOL  = hostAsk("--system", "--default", "native", "core.eol")
	eolUnset   = map[string]eolAnswer{
		hostProbe("core.autocrlf"): {out: hostGitUnset + "\n"},
		hostProbe("core.eol"):      {out: hostGitUnset + "\n"},
	}
)

// eolAnswers is eolUnset with more answers on top.
func eolAnswers(more map[string]eolAnswer) map[string]eolAnswer {
	all := map[string]eolAnswer{}
	for k, v := range eolUnset {
		all[k] = v
	}
	for k, v := range more {
		all[k] = v
	}
	return all
}

// git's refusal of a value that is not a boolean, as the runner reports it.
var eolNotABool = errors.New("git config --get --system --includes --type=bool --default false core.autocrlf in C:\\w: exit status 128: fatal: bad boolean config value 'input' for 'core.autocrlf'")

// Each answer is asked of the host's own config, in hostGit's directory, and
// each value reaches the sandbox the way the host's git reads it. native is
// both hosts' in turn, so a resolver that ignored its parameter passes one of
// them at most.
func TestTheLineEndingsAreTheHostGitsOwnAnswer(t *testing.T) {
	for _, tc := range []struct {
		name    string
		answers map[string]eolAnswer
		native  string
		want    api.GitLineEndings
	}{
		{"git for windows' installer default", eolAnswers(map[string]eolAnswer{eolAskBool: {out: "true\n"}, eolAskEOL: {out: "native\n"}}),
			"crlf", api.GitLineEndings{AutoCRLF: "true", EOL: "crlf"}},
		{"unset on windows", eolAnswers(map[string]eolAnswer{eolAskBool: {out: "false\n"}, eolAskEOL: {out: "native\n"}}),
			"crlf", api.GitLineEndings{AutoCRLF: "false", EOL: "crlf"}},
		{"unset on linux", eolAnswers(map[string]eolAnswer{eolAskBool: {out: "false\n"}, eolAskEOL: {out: "native\n"}}),
			"lf", api.GitLineEndings{AutoCRLF: "false", EOL: "lf"}},
		{"input", eolAnswers(map[string]eolAnswer{eolAskBool: {err: eolNotABool}, eolAskRaw: {out: "InPut\n"}, eolAskEOL: {out: "native\n"}}),
			"crlf", api.GitLineEndings{AutoCRLF: "input", EOL: "crlf"}},
		{"an explicit eol, in git's case-blind spelling", eolAnswers(map[string]eolAnswer{eolAskBool: {out: "false\n"}, eolAskEOL: {out: "LF\n"}}),
			"crlf", api.GitLineEndings{AutoCRLF: "false", EOL: "lf"}},
		{"crlf on linux", eolAnswers(map[string]eolAnswer{eolAskBool: {out: "false\n"}, eolAskEOL: {out: "crlf\n"}}),
			"lf", api.GitLineEndings{AutoCRLF: "false", EOL: "crlf"}},
		// git reads a core.eol it does not know as native, not as an error.
		{"an eol git does not know", eolAnswers(map[string]eolAnswer{eolAskBool: {out: "false\n"}, eolAskEOL: {out: "unix\n"}}),
			"crlf", api.GitLineEndings{AutoCRLF: "false", EOL: "crlf"}},
		// The global scope sets both, so each is read from there: git reads it
		// after the system scope. The system answers are scripted to lose.
		{"set in the global scope, over the system's", map[string]eolAnswer{
			hostProbe("core.autocrlf"): {out: "false\n"},
			hostProbe("core.eol"):      {out: "lf\n"},
			hostAsk("--global", "--type=bool", "--default", "false", "core.autocrlf"): {out: "false\n"},
			hostAsk("--global", "--default", "native", "core.eol"):                    {out: "lf\n"},
			eolAskBool: {out: "true\n"}, eolAskEOL: {out: "crlf\n"},
		}, "crlf", api.GitLineEndings{AutoCRLF: "false", EOL: "lf"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &eolRunner{answers: tc.answers}
			got, err := gitLineEndingsResolverWith(scriptedHost(host, noXDG), tc.native)(context.Background())
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if got != tc.want {
				t.Errorf("line endings = %#v, want %#v", got, tc.want)
			}
			for _, c := range host.calls {
				if !madeByHostGit(c.dir) {
					t.Errorf("git %q ran in %q, want a directory hostGit made for it", strings.Join(c.args, " "), c.dir)
				}
			}
		})
	}
}

// GIT_CONFIG_NOSYSTEM tells the host's git to skip the system file, which
// `git config --system` reads regardless, so the global scope and git's
// default answer instead. The system file's core.autocrlf=true must not count.
func TestWithGitConfigNoSystemTheSystemScopeIsNotRead(t *testing.T) {
	for _, v := range []string{"1", "true", "YES", "on"} {
		host := &eolRunner{answers: map[string]eolAnswer{
			hostProbe("core.autocrlf"): {out: hostGitUnset + "\n"},
			hostProbe("core.eol"):      {out: hostGitUnset + "\n"},
			hostAsk("--global", "--type=bool", "--default", "false", "core.autocrlf"): {out: "false\n"},
			hostAsk("--global", "--default", "native", "core.eol"):                    {out: "native\n"},
			eolAskBool: {out: "true\n"}, eolAskEOL: {out: "lf\n"},
		}}
		env := func(k string) string {
			if k == "GIT_CONFIG_NOSYSTEM" {
				return v
			}
			return ""
		}
		got, err := gitLineEndingsResolverWith(scriptedHost(host, env), "crlf")(context.Background())
		if want := (api.GitLineEndings{AutoCRLF: "false", EOL: "crlf"}); err != nil || got != want {
			t.Errorf("GIT_CONFIG_NOSYSTEM=%s: %#v, %v; want %#v", v, got, err, want)
		}
	}
}

// GIT_CONFIG_NOSYSTEM is a boolean, so a false value, or the empty string, is
// git reading the system file as usual. Git for Windows' installer writes
// core.autocrlf=true there, and a resolver that read any value as "skip it"
// would hand that host's sandbox false: every CRLF file reads as modified.
func TestWithGitConfigNoSystemFalseTheSystemScopeStillCounts(t *testing.T) {
	for _, v := range []string{"", "0", "false", "No", "off"} {
		host := &eolRunner{answers: eolAnswers(map[string]eolAnswer{
			eolAskBool: {out: "true\n"}, eolAskEOL: {out: "lf\n"},
		})}
		env := func(k string) string {
			if k == "GIT_CONFIG_NOSYSTEM" {
				return v
			}
			return ""
		}
		got, err := gitLineEndingsResolverWith(scriptedHost(host, env), "crlf")(context.Background())
		if want := (api.GitLineEndings{AutoCRLF: "true", EOL: "lf"}); err != nil || got != want {
			t.Errorf("GIT_CONFIG_NOSYSTEM=%q: %#v, %v; want the system scope's %#v", v, got, err, want)
		}
	}
}

// A core.autocrlf the host's git would die on is an error, carrying git's own
// words, and so is a git that cannot answer at all.
func TestALineEndingSettingTheHostsGitCannotReadIsAnError(t *testing.T) {
	gitDied := errors.New("git config --get --system --includes core.eol in C:\\w: exit status 128: fatal: bad config line 3 in file C:/Program Files/Git/etc/gitconfig")
	for name, answers := range map[string]map[string]eolAnswer{
		"autocrlf is not a word git knows": eolAnswers(map[string]eolAnswer{eolAskBool: {err: errors.New("exit status 128: fatal: bad boolean config value 'maybe' for 'core.autocrlf'")}, eolAskRaw: {out: "maybe\n"}, eolAskEOL: {out: "native\n"}}),
		"the raw read fails too":           eolAnswers(map[string]eolAnswer{eolAskBool: {err: eolNotABool}, eolAskRaw: {err: gitDied}, eolAskEOL: {out: "native\n"}}),
		"core.eol cannot be read":          eolAnswers(map[string]eolAnswer{eolAskBool: {out: "true\n"}, eolAskEOL: {err: gitDied}}),
		"the global scope cannot be read":  {hostProbe("core.autocrlf"): {err: gitDied}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := gitLineEndingsResolverWith(scriptedHost(&eolRunner{answers: answers}, noXDG), "crlf")(context.Background())
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
	// And the production resolver, the one main.go wires, is on this host.
	host := &eolRunner{answers: eolAnswers(map[string]eolAnswer{eolAskBool: {out: "false\n"}, eolAskEOL: {out: "native\n"}})}
	got, err := gitLineEndingsResolver(scriptedHost(host, noXDG))(context.Background())
	if err != nil || got.EOL != want {
		t.Errorf("gitLineEndingsResolver on %s: %#v, %v; want core.eol native read as %q", runtime.GOOS, got, err, want)
	}
}

// Against the real git: the answer includes the SYSTEM scope, which is where
// Git for Windows puts core.autocrlf=true and which no container sees, and the
// global scope beats it, an included file too, as they do on the host.
func TestTheRealHostGitAnswersFromItsOwnScopes(t *testing.T) {
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
	global := filepath.Join(dir, "gitconfig-global")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_COUNT", "")
	host, err := newHostGit()
	if err != nil {
		t.Fatal(err)
	}
	resolve := gitLineEndingsResolverWith(host, "crlf")

	got, err := resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := (api.GitLineEndings{AutoCRLF: "true", EOL: "crlf"}); got != want {
		t.Errorf("with system core.autocrlf=true: %#v, want %#v", got, want)
	}

	// core.eol arrives through include.path, which naming a scope turns off
	// unless --includes turns it back on.
	included := filepath.Join(dir, "gitconfig-included")
	if err := os.WriteFile(included, []byte("[core]\n\teol = lf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "[core]\n\tautocrlf = input\n[include]\n\tpath = " + filepath.ToSlash(included) + "\n"
	if err := os.WriteFile(global, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := (api.GitLineEndings{AutoCRLF: "input", EOL: "lf"}); got != want {
		t.Errorf("with the global scope's own settings: %#v, want %#v", got, want)
	}
}

// Each key is read from the scope git takes IT from (contract §5.1 step 5),
// not from the scope the other came from. Git for Windows' installer puts
// core.autocrlf=true in the system file, and a user's own core.eol=lf lives
// in the global one; reading core.eol in core.autocrlf's scope would send the
// sandbox crlf (X-1 review m-3, mutant R2).
func TestEachLineEndingKeyIsReadFromItsOwnScope(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	system := filepath.Join(dir, "gitconfig-system")
	global := filepath.Join(dir, "gitconfig-global")
	if err := os.WriteFile(system, []byte("[core]\n\tautocrlf = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(global, []byte("[core]\n\teol = lf\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "")
	t.Setenv("GIT_CONFIG_SYSTEM", system)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_COUNT", "")
	host, err := newHostGit()
	if err != nil {
		t.Fatal(err)
	}
	got, err := gitLineEndingsResolverWith(host, "crlf")(context.Background())
	if want := (api.GitLineEndings{AutoCRLF: "true", EOL: "lf"}); err != nil || got != want {
		t.Errorf("system autocrlf=true, global eol=lf: %#v, %v; want %#v", got, err, want)
	}
}
