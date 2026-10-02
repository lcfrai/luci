package worktree_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/worktree"
)

// Phase 4's contextwrite.WorktreeResolver declares exactly this method set, so
// one type satisfies both with no adapter. If this line stops compiling, the
// seam has drifted.
var _ worktree.Provider = worktree.Static{}

func TestStaticResolvesAKnownIdToItsDirectory(t *testing.T) {
	dir := t.TempDir()
	s := worktree.Static{"w-4a77b1c9": dir}

	got, err := s.Path(context.Background(), "w-4a77b1c9")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if got != dir {
		t.Fatalf("Path = %q, want %q", got, dir)
	}
}

func TestStaticRejectsAnUnknownId(t *testing.T) {
	s := worktree.Static{"w-4a77b1c9": t.TempDir()}

	_, err := s.Path(context.Background(), "w-deadbeef")
	if !errors.Is(err, worktree.ErrUnknownID) {
		t.Fatalf("Path of an unknown id = %v, want an error wrapping ErrUnknownID", err)
	}
}

func TestNilStaticReportsUnknownRatherThanPanicking(t *testing.T) {
	var s worktree.Static

	_, err := s.Path(context.Background(), "w-4a77b1c9")
	if !errors.Is(err, worktree.ErrUnknownID) {
		t.Fatalf("Path on a nil Static = %v, want an error wrapping ErrUnknownID", err)
	}
}

// The reason Path stats. Without it the bad path reaches Docker's bind-mount
// validation and the user sees a daemon error about an invalid mount spec.
func TestStaticRejectsADirectoryThatIsNotThere(t *testing.T) {
	s := worktree.Static{"w-4a77b1c9": filepath.Join(t.TempDir(), "removed")}

	_, err := s.Path(context.Background(), "w-4a77b1c9")
	if !errors.Is(err, worktree.ErrNotADir) {
		t.Fatalf("Path of a missing directory = %v, want an error wrapping ErrNotADir", err)
	}
}

func TestStaticRejectsAPathThatIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "CLAUDE.md")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s := worktree.Static{"w-4a77b1c9": file}

	_, err := s.Path(context.Background(), "w-4a77b1c9")
	if !errors.Is(err, worktree.ErrNotADir) {
		t.Fatalf("Path of a regular file = %v, want an error wrapping ErrNotADir", err)
	}
}

// The contract says ABSOLUTE and it means it: Phase 4 writes CLAUDE.md at this
// path and Docker binds it, and neither resolves a relative path against the
// control plane's working directory in the way a caller would assume.
func TestStaticReturnsACleanedAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	s := worktree.Static{"w-4a77b1c9": filepath.Join(dir, "sub", "..") + string(filepath.Separator) + "."}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	got, err := s.Path(context.Background(), "w-4a77b1c9")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("Path = %q, want an absolute path", got)
	}
	if got != dir {
		t.Fatalf("Path = %q, want the cleaned %q", got, dir)
	}
}

func TestStaticHonoursACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := worktree.Static{"w-4a77b1c9": t.TempDir()}

	_, err := s.Path(ctx, "w-4a77b1c9")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Path with a cancelled context = %v, want context.Canceled", err)
	}
}
