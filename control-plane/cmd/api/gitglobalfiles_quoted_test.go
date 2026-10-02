package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/api"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

// originEnv points the real git at a global file in a fresh directory and at
// nothing else, as TestTheRealHostGitNamesTheOriginOfAnIncludedGlobalFileSetting
// does, with GIT_CONFIG_GLOBAL spelled by spell. It returns the projects
// directory, the global file, and a host file OUTSIDE the projects directory for
// a setting to name: the named-path check (m-5) passes it, only an origin can
// catch it.
func originEnv(t *testing.T, spell func(string) string) (projects, global, secret string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	projects = filepath.Join(dir, "projects")
	global = filepath.Join(dir, "gitconfig-global")
	secret = filepath.Join(dir, "home", ".ssh", "id_ed25519")
	for _, d := range []string{filepath.Join(projects, "dotfiles", "sub"), filepath.Dir(secret)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(secret, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", spell(global))
	t.Setenv("GIT_CONFIG_COUNT", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("HOME", filepath.Join(dir, "home"))
	return projects, global, secret
}

// gitDo runs the test's own git, once per argument list.
func gitDo(t *testing.T, cmds ...[]string) {
	t.Helper()
	for _, args := range cmds {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

// originsCaught runs the production resolver and reports whether the session's
// own walk refuses any origin it hands over as inside projects.
func originsCaught(t *testing.T, projects string) (api.GitGlobalFiles, bool) {
	t.Helper()
	host, err := newHostGit()
	if err != nil {
		t.Fatal(err)
	}
	got, err := gitGlobalFilesResolver(host)(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	for _, o := range got.Origins {
		if _, err := hostpath.ResolveOutside(projects, o); errors.Is(err, hostpath.ErrInside) {
			return got, true
		}
	}
	return got, false
}

// asIs is GIT_CONFIG_GLOBAL as filepath.Join spells it: backslashes on Windows.
func asIs(p string) string { return p }

// X-3 review 4 I-C, with the real git. I-C's premise: the host's global config
// includes b1, a file in a dotfiles PROJECT, so the agent owns b1's content. It
// adds ONE include line naming b2 in a form git C-quotes in --show-origin unless
// -z is given — an absolute backslash path, a relative backslash separator, a
// non-ASCII name — and b2 sets the key to a host file outside the projects
// directory. A quoted origin resolves to nothing, so the walk never saw b2 and
// the create went on (measured at c62e220f: 201, the key mounted). The origin
// the session is handed must be b2 itself, for either key (MUT-ATTR dropped the
// attributes origin with every committed test green). A relative hop is joined
// to b1's directory the way git joins it: on Windows `sub\b2.inc` is two names,
// elsewhere one name holding a backslash, which git quotes all the same.
func TestAnOriginGitWouldQuoteIsStillTheFileThatSetTheValue(t *testing.T) {
	for _, key := range []string{"core.excludesFile", "core.attributesFile"} {
		for _, tc := range []struct{ name, hop string }{
			{"absolute backslash path", ""},
			{"relative backslash separator", `sub\b2.inc`},
			{"relative non-ASCII name", "évil.inc"},
		} {
			t.Run(key+", "+tc.name, func(t *testing.T) {
				projects, global, secret := originEnv(t, asIs)
				b1 := filepath.Join(projects, "dotfiles", "b1.inc")
				b2 := filepath.Join(projects, "dotfiles", "b2.inc")
				hop := b2 // filepath.Join's spelling: backslashes on Windows
				if tc.hop != "" {
					hop, b2 = tc.hop, filepath.Join(projects, "dotfiles", tc.hop)
				}
				gitDo(t, []string{"config", "--file", b2, key, filepath.ToSlash(secret)},
					[]string{"config", "--file", b1, "include.path", hop},
					[]string{"config", "--file", global, "include.path", filepath.ToSlash(b1)})
				got, caught := originsCaught(t, projects)
				value := got.Excludes
				if key == "core.attributesFile" {
					value = got.Attributes
				}
				if filepath.Clean(value) != filepath.Clean(secret) {
					t.Fatalf("attack not set up: %s = %q, want %q", key, value, secret)
				}
				if !caught {
					t.Fatalf("BYPASS: %q, inside %q, set %s to a host file, and no origin in %#v is refused as inside it", b2, projects, key, got.Origins)
				}
			})
		}
	}
}
