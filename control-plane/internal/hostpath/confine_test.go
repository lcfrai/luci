package hostpath

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The case the guard exists to keep working: a dotfile manager links
// ~/.config/git/ignore at a repository that is NOT under the projects root.
// Nothing on the walk is the agent's, so the real file comes back and the
// session mounts it.
func TestAChainThatNeverTouchesTheRootResolvesToTheRealFile(t *testing.T) {
	root, host := layout(t)
	real := filepath.Join(host, "dotfiles", "gitignore")
	writeFile(t, real, []byte("*.log\n"))
	named := filepath.Join(host, "config", "git", "ignore")
	linkOrSkip(t, real, named)

	got, err := ResolveOutside(root, named)
	if err != nil {
		t.Fatalf("ResolveOutside(%q) = %v; want the real file", named, err)
	}
	want, err := Real(real)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("ResolveOutside(%q) = %q; want %q", named, got, want)
	}
}

// An empty root is an unconfigured projects directory: nothing under it is the
// agent's, because there is nothing under it.
func TestAnEmptyRootConfinesNothing(t *testing.T) {
	root, host := layout(t)
	inside := filepath.Join(root, "p", "ignore")
	writeFile(t, inside, []byte("*.log\n"))
	_ = host
	if _, err := ResolveOutside("", inside); err != nil {
		t.Fatalf("ResolveOutside with no root = %v; want the file", err)
	}
	if _, err := ResolveOutside(root, inside); !errors.Is(err, ErrInside) {
		t.Fatalf("ResolveOutside with the root = %v; want ErrInside", err)
	}
}

// X-1 review O-1: `git worktree list` in the agent-writable primary worktree
// named a directory in ANOTHER repository, and a create mounts it read-WRITE.
// Confine is the gate that decides, so it answers on the REAL location: a
// directory that is only lexically under the root is not under it.
func TestConfineRefusesALinkThatLeavesTheRoot(t *testing.T) {
	root, host := layout(t)
	elsewhere := filepath.Join(host, "other-repo")
	if err := os.MkdirAll(elsewhere, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "looks-like-a-worktree")
	linkOrSkip(t, elsewhere, link)

	// What worktree.Verify alone answers, and what the create mounted read-
	// WRITE before X-3: Stat follows the link, sees a directory, and says yes.
	if info, err := os.Stat(link); err != nil || !info.IsDir() {
		t.Fatalf("the attack is not set up: Stat(%q) = %v, %v; want a directory", link, info, err)
	}

	if got, err := Confine(root, link); !errors.Is(err, ErrOutside) {
		t.Fatalf("Confine(%q) = %q, %v; want ErrOutside", link, got, err)
	}
}

// The legitimate case, and it must keep working: a real linked worktree inside
// the projects root.
func TestConfineAcceptsARealDirectoryInsideTheRoot(t *testing.T) {
	root, _ := layout(t)
	wt := filepath.Join(root, "proj", "wt-feature")
	if err := os.MkdirAll(wt, 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := Confine(root, wt)
	if err != nil {
		t.Fatalf("Confine(%q) = %v; want the directory", wt, err)
	}
	want, err := Real(wt)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Confine(%q) = %q; want %q", wt, got, want)
	}
}
