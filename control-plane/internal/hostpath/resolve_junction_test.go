package hostpath

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// mkJunction makes a Windows junction (a reparse point that is NOT a symlink)
// or skips. A junction needs no privilege on Windows, so it is the attack a
// symlink-only walk would miss.
func mkJunction(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("a junction is a Windows reparse point")
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("this host will not make a junction at %s: %v: %s", link, err, out)
	}
}

// X-3 fix round. m-5's guarantee is that NO step of the resolution may be the
// projects directory, because a step there is the agent's to swap. A junction
// is a step the agent can plant too: a dotfile manager junctions ~/gitcfg into
// a dotfiles repository under the projects directory, and the host's git names
// ~/gitcfg/ignore as its excludes file. Both ends are outside the root, so only
// the HOP through the root betrays the attack — and the walk sees that hop only
// if it follows the junction. Before the fix the symlink-only walk skipped the
// junction and answered a real path with no error; the ModeIrregular mask makes
// it follow the junction, land on the root, and refuse. Red without the fix.
func TestAJunctionHopIntoTheProjectsDirectoryIsRefused(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "projects")
	home := filepath.Join(base, "home")
	dotgit := filepath.Join(root, "dotfiles", "git")
	for _, d := range []string{dotgit, home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dotgit, "ignore"), []byte("*.o\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitcfg := filepath.Join(home, "gitcfg")
	mkJunction(t, dotgit, gitcfg) // ~/gitcfg -> <root>/dotfiles/git, a hop INTO the root

	_, err := ResolveOutside(root, filepath.Join(gitcfg, "ignore"))
	if !errors.Is(err, ErrInside) {
		t.Fatalf("ResolveOutside through a junction into the projects dir = %v, want ErrInside: a junction the agent plants must be followed, not skipped", err)
	}
}
