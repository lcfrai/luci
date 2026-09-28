package hostpath

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// linkOrSkip makes a symlink or skips. A security claim this host cannot
// exercise must not read as proven, so the skip says which one it was.
func linkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("this platform will not make a symlink at %s: %v", link, err)
	}
}

func writeFile(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// layout is a projects root an agent writes and a host-owned directory beside
// it, both real directories so neither is reached through a link.
func layout(t *testing.T) (root, host string) {
	t.Helper()
	base := t.TempDir()
	root, host = filepath.Join(base, "Projects"), filepath.Join(base, "host")
	for _, d := range []string{root, host} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return root, host
}

// X-1 review m-5, with review-2 m-4's dotfile-manager variant: the host's own
// global config names ~/.config/git/ignore, a dotfile manager has linked that
// into a repository under the projects root, and the agent swaps the file in
// that repository for a link to a key. Every path the final answer names is
// OUTSIDE the root, so a check of the resolved path alone mounts the key. The
// HOP is where the agent got to choose, and the hop is inside the root.
func TestALinkHopInsideTheRootIsRefusedEvenWhenItsTargetIsOutside(t *testing.T) {
	root, host := layout(t)
	secret := filepath.Join(host, "id_rsa")
	writeFile(t, secret, []byte("PRIVATE KEY"))
	hop := filepath.Join(root, "dotfiles", "git", "ignore")
	linkOrSkip(t, secret, hop)
	named := filepath.Join(host, "config", "git", "ignore")
	linkOrSkip(t, hop, named)

	// What sessions.go did before X-3, and what it would do again if the walk
	// were reduced to its answer: the key itself, ready to mount.
	if resolved, err := Real(named); err != nil || resolved != mustReal(t, secret) {
		t.Fatalf("the attack is not set up: Real(%q) = %q, %v; want the key at %q", named, resolved, err, secret)
	}

	got, err := ResolveOutside(root, named)
	if !errors.Is(err, ErrInside) {
		t.Fatalf("ResolveOutside(%q) = %q, %v; want ErrInside", named, got, err)
	}
	if got != "" {
		t.Fatalf("a refusal must hand back no path to mount, got %q", got)
	}
}

func mustReal(t *testing.T, p string) string {
	t.Helper()
	real, err := Real(p)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

// The same attack one level up: it is a DIRECTORY on the way that the agent
// owns, so Lstat of the named path sees an ordinary file and every link the
// walk reads points outside the root.
func TestADirectoryComponentInsideTheRootIsRefused(t *testing.T) {
	root, host := layout(t)
	real := filepath.Join(root, "dotfiles", "git")
	writeFile(t, filepath.Join(real, "ignore"), []byte("*.log\n"))
	linkOrSkip(t, real, filepath.Join(host, "config", "git"))
	named := filepath.Join(host, "config", "git", "ignore")

	got, err := ResolveOutside(root, named)
	if !errors.Is(err, ErrInside) {
		t.Fatalf("ResolveOutside(%q) = %q, %v; want ErrInside", named, got, err)
	}
}
