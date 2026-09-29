package main

import (
	"os"
	"path/filepath"
	"testing"
)

// X-3 review 4's controls for I-C, with the real git: routes the origin check
// covers with no quoting in play, and the one route where the quoting is the
// host's own spelling. "chain" is two in-project hops, "relative" b1 naming b2
// by a relative name, and "linked" the global file itself a symlink to an
// in-project file that sets the value. "linked" keeps GIT_CONFIG_GLOBAL as
// filepath.Join spells it, backslashes on Windows, which git C-quotes without -z
// even though no agent chose it; at c62e220f that alone made the check inert.
func TestAnInProjectOriginIsCaughtHoweverGitReachedIt(t *testing.T) {
	for _, mode := range []string{"chain", "relative", "linked", "linked-slash"} {
		t.Run(mode, func(t *testing.T) {
			spell := asIs
			if mode == "linked-slash" {
				spell = filepath.ToSlash
			}
			projects, global, secret := originEnv(t, spell)
			b1 := filepath.Join(projects, "dotfiles", "b1.inc")
			b2 := filepath.Join(projects, "dotfiles", "b2.inc")
			set := []string{"config", "--file", b2, "core.excludesFile", filepath.ToSlash(secret)}
			switch mode {
			case "chain":
				gitDo(t, set, []string{"config", "--file", b1, "include.path", filepath.ToSlash(b2)},
					[]string{"config", "--file", global, "include.path", filepath.ToSlash(b1)})
			case "relative":
				gitDo(t, set, []string{"config", "--file", b1, "include.path", "b2.inc"},
					[]string{"config", "--file", global, "include.path", filepath.ToSlash(b1)})
			default:
				gitDo(t, set)
				// Linked after b2 exists: git reads GIT_CONFIG_GLOBAL on every run.
				if err := os.Symlink(b2, global); err != nil {
					t.Skipf("no symlink here: %v", err)
				}
			}
			got, caught := originsCaught(t, projects)
			if filepath.Clean(got.Excludes) != filepath.Clean(secret) {
				t.Fatalf("not set up: excludes = %q, want %q", got.Excludes, secret)
			}
			if !caught {
				t.Fatalf("NOT CAUGHT (%s): Origins %#v", mode, got.Origins)
			}
		})
	}
}
