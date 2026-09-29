package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
)

// globalAsk is the git question for key when the global scope does not set
// it: the system scope's --show-origin answer, with def as git's default.
func globalAsk(def, key string) string {
	return globalOriginAsk("--system", def, key)
}

// globalOriginAsk is hostGit.valueWithOrigin's question (X-3 review 3 I-C): the
// scoped --get the global-files resolver now reads WITH --show-origin.
func globalOriginAsk(scope, def, key string) string {
	return strings.Join([]string{"config", "--show-origin", "--get", scope, "--includes", "--type=path", "--default", def, key}, " ")
}

// fileOrigin and cmdOrigin build a --show-origin answer, `<origin>\t<value>`: a
// value set in config file f, and one an unset key gets from git's --default,
// whose origin git reports as the command line (no file).
func fileOrigin(f, value string) string { return "file:" + f + "\t" + value + "\n" }
func cmdOrigin(value string) string     { return "command line:\t" + value + "\n" }

// globalAnswers is answers with neither key set in the global scope.
func globalAnswers(answers map[string]eolAnswer) map[string]eolAnswer {
	all := map[string]eolAnswer{
		hostProbe("core.excludesFile"):   {out: hostGitUnset + "\n"},
		hostProbe("core.attributesFile"): {out: hostGitUnset + "\n"},
	}
	for k, v := range answers {
		all[k] = v
	}
	return all
}

// noXDG is a host environment with XDG_CONFIG_HOME unset.
func noXDG(string) string { return "" }

// Each file is asked of the host's own config in hostGit's directory, with
// git's own default for an unset key, and git's answer is the path.
// XDG_CONFIG_HOME moves the default, as it does for git; a relative answer is
// passed on for the session to refuse, and an empty one is no file at all.
func TestTheGlobalGitFilesAreTheHostGitsOwnAnswer(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".config", "git")
	xdg := t.TempDir()
	for _, tc := range []struct {
		name    string
		getenv  func(string) string
		answers map[string]eolAnswer
		want    api.GitGlobalFiles
	}{
		{"both unset, no XDG_CONFIG_HOME", noXDG, globalAnswers(map[string]eolAnswer{
			globalAsk("~/.config/git/ignore", "core.excludesFile"):       {out: cmdOrigin(filepath.Join(home, "ignore"))},
			globalAsk("~/.config/git/attributes", "core.attributesFile"): {out: cmdOrigin(filepath.Join(home, "attributes"))},
		}), api.GitGlobalFiles{Excludes: filepath.Join(home, "ignore"), Attributes: filepath.Join(home, "attributes")}},
		{"XDG_CONFIG_HOME set", func(k string) string {
			if k == "XDG_CONFIG_HOME" {
				return xdg
			}
			return ""
		}, globalAnswers(map[string]eolAnswer{
			globalAsk(xdg+"/git/ignore", "core.excludesFile"):       {out: cmdOrigin(xdg + "/git/ignore")},
			globalAsk(xdg+"/git/attributes", "core.attributesFile"): {out: cmdOrigin(xdg + "/git/attributes")},
		}), api.GitGlobalFiles{Excludes: xdg + "/git/ignore", Attributes: xdg + "/git/attributes"}},
		// A relative value names a file in each repository, not a host file, so
		// its origin is dropped even when git reports one: no provenance to check.
		{"a relative core.excludesFile, an empty core.attributesFile", noXDG, globalAnswers(map[string]eolAnswer{
			globalAsk("~/.config/git/ignore", "core.excludesFile"):       {out: fileOrigin("/host/.gitconfig", "my-ignore")},
			globalAsk("~/.config/git/attributes", "core.attributesFile"): {out: "\n"},
		}), api.GitGlobalFiles{Excludes: "my-ignore", Attributes: ""}},
		// The global scope sets excludes, so it is read from there; the system
		// answer is scripted to lose. Its value is absolute, so its origin file
		// is kept for the session to check (X-3 review 3 I-C).
		{"excludes set in the global scope", noXDG, globalAnswers(map[string]eolAnswer{
			hostProbe("core.excludesFile"):                              {out: "~/my-ignore\n"},
			globalOriginAsk("--global", "~/.config/git/ignore", "core.excludesFile"): {out: fileOrigin("/host/.gitconfig", home+"/my-ignore")},
			globalAsk("~/.config/git/attributes", "core.attributesFile"):             {out: "\n"},
		}), api.GitGlobalFiles{Excludes: home + "/my-ignore", Origins: []string{"/host/.gitconfig"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &eolRunner{answers: tc.answers}
			got, err := gitGlobalFilesResolver(scriptedHost(host, tc.getenv))(context.Background())
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("global files = %#v, want %#v", got, tc.want)
			}
			if len(host.calls) != 4 {
				t.Errorf("git ran %d times, want a probe and a read for each: %#v", len(host.calls), host.calls)
			}
			for _, c := range host.calls {
				if !madeByHostGit(c.dir) {
					t.Errorf("git %q ran in %q, want a directory hostGit made for it", strings.Join(c.args, " "), c.dir)
				}
			}
		})
	}
}

// A git that cannot answer is an error carrying git's own words, and no half
// answer comes with it.
func TestAGlobalGitFileTheHostsGitCannotReadIsAnError(t *testing.T) {
	gitDied := errors.New("git config in C:\\w: exit status 128: fatal: bad config line 3 in file C:/Users/u/.gitconfig")
	for name, answers := range map[string]map[string]eolAnswer{
		"excludes": globalAnswers(map[string]eolAnswer{globalAsk("~/.config/git/ignore", "core.excludesFile"): {err: gitDied}}),
		"attributes": globalAnswers(map[string]eolAnswer{
			globalAsk("~/.config/git/ignore", "core.excludesFile"):       {out: "/x/ignore\n"},
			globalAsk("~/.config/git/attributes", "core.attributesFile"): {err: gitDied},
		}),
		"excludes, in the probe": {hostProbe("core.excludesFile"): {err: gitDied}},
	} {
		t.Run(name, func(t *testing.T) {
			if strings.HasSuffix(name, ", in the probe") {
				name = strings.TrimSuffix(name, ", in the probe")
			}
			got, err := gitGlobalFilesResolver(scriptedHost(&eolRunner{answers: answers}, noXDG))(context.Background())
			if err == nil {
				t.Fatalf("resolve returned %#v and no error", got)
			}
			if !reflect.DeepEqual(got, api.GitGlobalFiles{}) {
				t.Errorf("global files = %#v alongside an error, want none", got)
			}
			if !strings.Contains(err.Error(), "exit status 128") || !strings.Contains(err.Error(), "core."+name) {
				t.Errorf("error %q does not name core.%s and carry git's own words", err, name)
			}
		})
	}
}

// Against the real git: with both keys unset the answer is git's default under
// XDG_CONFIG_HOME, and the global scope's settings beat it, a relative one
// passed on as git gave it and "~/" expanded with git's HOME. The production
// hostGit, the one main.go wires, reads this process's environment, and
// GIT_CONFIG_NOSYSTEM keeps the host's own system file out of the answer.
func TestTheRealHostGitNamesItsGlobalFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	xdg := filepath.Join(dir, "xdg")
	home := filepath.Join(dir, "home")
	global := filepath.Join(dir, "gitconfig-global")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_COUNT", "")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", home)
	repo := filepath.Join(dir, "repo")
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	host, err := newHostGit()
	if err != nil {
		t.Fatal(err)
	}
	resolve := gitGlobalFilesResolver(host)

	same := func(label, got, want string) {
		t.Helper()
		if filepath.Clean(got) != filepath.Clean(want) {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
	got, err := resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	same("unset excludes", got.Excludes, filepath.Join(xdg, "git", "ignore"))
	same("unset attributes", got.Attributes, filepath.Join(xdg, "git", "attributes"))

	for _, args := range [][]string{
		{"config", "--file", global, "core.excludesFile", "my-ignore"},
		{"config", "--file", global, "core.attributesFile", "~/attrs"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	got, err = resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Excludes != "my-ignore" {
		t.Errorf("a relative excludes = %q, want it as git gave it, for the session to refuse", got.Excludes)
	}
	same("a ~/ attributes", got.Attributes, filepath.Join(home, "attrs"))

	// And git does read the default it names: a pattern there ignores a file.
	if err := os.MkdirAll(filepath.Join(xdg, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "git", "ignore"), []byte("secret.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "config", "--file", global, "--unset", "core.excludesFile").CombinedOutput(); err != nil {
		t.Fatalf("git config --unset: %v: %s", err, out)
	}
	// The test's own git, in its own repository, checks the claim.
	if out, err := exec.Command("git", "-C", repo, "check-ignore", "-q", "secret.txt").CombinedOutput(); err != nil {
		t.Fatalf("git check-ignore secret.txt: %v: %s; the default the resolver names is not the one git reads", err, out)
	}
	got, err = resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	same("the default git reads", got.Excludes, filepath.Join(xdg, "git", "ignore"))
}

// X-3 review 3 I-C, with the real git. core.excludesFile is set by a config file
// an [include] pulls in — the dotfiles-repository-that-is-a-project case — and it
// names an absolute host file, so a host file would be mounted. --show-origin
// follows the include to the file that actually set it, not the top-level file,
// and the resolver reports that file as the value's origin so the session can
// refuse a setting whose provenance a sandbox can write. Without the origin
// plumbing Origins is empty here.
func TestTheRealHostGitNamesTheOriginOfAnIncludedGlobalFileSetting(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	global := filepath.Join(dir, "gitconfig-global")
	included := filepath.Join(dir, "projects", "dotfiles", "gitconfig")
	// An absolute host file, so the value is a mount candidate and its origin is
	// kept; the value itself is outside any projects dir, so only the origin is
	// what a provenance check would catch.
	named := filepath.Join(dir, "home", "ignore")
	for _, d := range []string{filepath.Dir(included), filepath.Dir(named)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(named, []byte("*.log\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_COUNT", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("HOME", filepath.Join(dir, "home"))
	for _, args := range [][]string{
		{"config", "--file", included, "core.excludesFile", filepath.ToSlash(named)},
		{"config", "--file", global, "include.path", filepath.ToSlash(included)},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	host, err := newHostGit()
	if err != nil {
		t.Fatal(err)
	}
	got, err := gitGlobalFilesResolver(host)(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if filepath.Clean(got.Excludes) != filepath.Clean(named) {
		t.Fatalf("excludes = %q, want the host file the include set, %q", got.Excludes, named)
	}
	found := false
	for _, o := range got.Origins {
		if filepath.Clean(o) == filepath.Clean(included) {
			found = true
		}
	}
	if !found {
		t.Fatalf("Origins = %#v, want the included file %q that SET core.excludesFile", got.Origins, included)
	}
}
