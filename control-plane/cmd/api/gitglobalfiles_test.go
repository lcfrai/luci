package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// globalAsk is the one git question for key, with def as git's default.
func globalAsk(def, key string) string {
	return "config --get --type=path --default " + def + " " + key
}

// noXDG is a host environment with XDG_CONFIG_HOME unset.
func noXDG(string) string { return "" }

// Each file is asked of git in the worktree, with git's own default for an
// unset key, and git's answer is the path. XDG_CONFIG_HOME moves the default,
// as it does for git; a relative answer is read against the worktree, and an
// empty one is no file at all.
func TestTheGlobalGitFilesAreTheHostGitsOwnAnswerInTheWorktree(t *testing.T) {
	worktree := t.TempDir()
	home := filepath.Join(t.TempDir(), ".config", "git")
	xdg := t.TempDir()
	for _, tc := range []struct {
		name    string
		getenv  func(string) string
		answers map[string]eolAnswer
		want    api.GitGlobalFiles
	}{
		{"both unset, no XDG_CONFIG_HOME", noXDG, map[string]eolAnswer{
			globalAsk("~/.config/git/ignore", "core.excludesFile"):       {out: filepath.Join(home, "ignore") + "\n"},
			globalAsk("~/.config/git/attributes", "core.attributesFile"): {out: filepath.Join(home, "attributes") + "\n"},
		}, api.GitGlobalFiles{Excludes: filepath.Join(home, "ignore"), Attributes: filepath.Join(home, "attributes")}},
		{"XDG_CONFIG_HOME set", func(k string) string {
			if k == "XDG_CONFIG_HOME" {
				return xdg
			}
			return ""
		}, map[string]eolAnswer{
			globalAsk(xdg+"/git/ignore", "core.excludesFile"):       {out: xdg + "/git/ignore\n"},
			globalAsk(xdg+"/git/attributes", "core.attributesFile"): {out: xdg + "/git/attributes\n"},
		}, api.GitGlobalFiles{Excludes: xdg + "/git/ignore", Attributes: xdg + "/git/attributes"}},
		{"a relative core.excludesFile, an empty core.attributesFile", noXDG, map[string]eolAnswer{
			globalAsk("~/.config/git/ignore", "core.excludesFile"):       {out: "my-ignore\n"},
			globalAsk("~/.config/git/attributes", "core.attributesFile"): {out: "\n"},
		}, api.GitGlobalFiles{Excludes: filepath.Join(worktree, "my-ignore"), Attributes: ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &eolRunner{answers: tc.answers}
			got, err := gitGlobalFilesResolverWith(host, tc.getenv)(context.Background(), worktree)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if got != tc.want {
				t.Errorf("global files = %#v, want %#v", got, tc.want)
			}
			if len(host.calls) != 2 {
				t.Errorf("git ran %d times, want twice: %#v", len(host.calls), host.calls)
			}
			for _, c := range host.calls {
				if c.dir != worktree {
					t.Errorf("git %q ran in %q, want the worktree", strings.Join(c.args, " "), c.dir)
				}
			}
		})
	}
}

// A git that cannot answer is an error carrying git's own words, and no half
// answer comes with it.
func TestAGlobalGitFileTheHostsGitCannotReadIsAnError(t *testing.T) {
	gitDied := errors.New("git config in C:\\w: exit status 128: fatal: bad config line 3 in file .git/config")
	for name, answers := range map[string]map[string]eolAnswer{
		"excludes": {globalAsk("~/.config/git/ignore", "core.excludesFile"): {err: gitDied}},
		"attributes": {
			globalAsk("~/.config/git/ignore", "core.excludesFile"):       {out: "/x/ignore\n"},
			globalAsk("~/.config/git/attributes", "core.attributesFile"): {err: gitDied},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := gitGlobalFilesResolverWith(&eolRunner{answers: answers}, noXDG)(context.Background(), t.TempDir())
			if err == nil {
				t.Fatalf("resolve returned %#v and no error", got)
			}
			if got != (api.GitGlobalFiles{}) {
				t.Errorf("global files = %#v alongside an error, want none", got)
			}
			if !strings.Contains(err.Error(), "exit status 128") || !strings.Contains(err.Error(), "core."+name) {
				t.Errorf("error %q does not name core.%s and carry git's own words", err, name)
			}
		})
	}
}

// Against the real git: with both keys unset the answer is git's default under
// XDG_CONFIG_HOME, and a repository's own settings beat it, a relative one read
// against the worktree and "~/" expanded with git's HOME. The production
// resolver, the one main.go wires, reads this process's environment.
func TestTheRealHostGitNamesItsGlobalFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	xdg := filepath.Join(dir, "xdg")
	home := filepath.Join(dir, "home")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "gitconfig-global"))
	t.Setenv("GIT_CONFIG_COUNT", "")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", home)
	repo := filepath.Join(dir, "repo")
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	resolve := gitGlobalFilesResolver(project.NewExecRunner())

	same := func(label, got, want string) {
		t.Helper()
		if filepath.Clean(got) != filepath.Clean(want) {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
	got, err := resolve(context.Background(), repo)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	same("unset excludes", got.Excludes, filepath.Join(xdg, "git", "ignore"))
	same("unset attributes", got.Attributes, filepath.Join(xdg, "git", "attributes"))

	for _, args := range [][]string{
		{"-C", repo, "config", "core.excludesFile", "my-ignore"},
		{"-C", repo, "config", "core.attributesFile", "~/attrs"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	got, err = resolve(context.Background(), repo)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	same("a relative excludes", got.Excludes, filepath.Join(repo, "my-ignore"))
	same("a ~/ attributes", got.Attributes, filepath.Join(home, "attrs"))

	// And git does read the default it names: a pattern there ignores a file.
	if err := os.MkdirAll(filepath.Join(xdg, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xdg, "git", "ignore"), []byte("secret.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", repo, "config", "--unset", "core.excludesFile").CombinedOutput(); err != nil {
		t.Fatalf("git config --unset: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "check-ignore", "-q", "secret.txt").CombinedOutput(); err != nil {
		t.Fatalf("git check-ignore secret.txt: %v: %s; the default the resolver names is not the one git reads", err, out)
	}
	got, err = resolve(context.Background(), repo)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	same("the default git reads", got.Excludes, filepath.Join(xdg, "git", "ignore"))
}
