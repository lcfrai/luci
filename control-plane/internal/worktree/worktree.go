// Package worktree resolves a worktree id to a directory on disk.
//
// PHASE 3 SEAM — this contract is restated in the Phase 3 plan and, in
// identical words, in Phase 4's contextwrite package:
//  1. Path returns an ABSOLUTE path to an EXISTING directory that is the root
//     of a checked-out worktree. It never creates, moves or removes anything.
//  2. Phase 3 owns creating, listing and removing worktrees. Phase 2 owns only
//     resolution and read-only enumeration.
//  3. Anything that must be on disk before an agent starts — Phase 4 writes
//     CLAUDE.md here — is written BEFORE the sandbox is created, never after.
//     Claude Code reads CLAUDE.md at session start; writing it afterwards means
//     the session inherits nothing, silently, with no error anywhere.
//
// X-3 (2026-09-28) adds a condition to point 1 for ONE implementation, and the
// three sentences above are unchanged so that the phase 3 and phase 4 copies
// still match word for word: *project.Scanner also confines its answer to the
// projects directory, because its answer comes from git run where an agent
// writes (X-1 review O-1, Go contract §2.10). Static is not confined — it is
// the operator's own map, not git's answer.
package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrUnknownID is returned for an id the provider does not know.
var ErrUnknownID = errors.New("worktree: unknown worktree id")

// ErrNotADir is returned when the resolved path is not an existing directory.
var ErrNotADir = errors.New("worktree: path is not an existing directory")

// Provider resolves worktree ids. The method set is deliberately one method:
// Phase 4's contextwrite.WorktreeResolver declares exactly this signature, so
// one type satisfies both with no adapter.
type Provider interface {
	Path(ctx context.Context, worktreeID string) (string, error)
}

// Static is the implementation Phase 2 ships when no projects directory is
// configured, and the one every test uses.
type Static map[string]string

// Path implements Provider. It verifies the directory exists, because a
// Provider that returns a plausible path for a directory that is not there
// pushes the failure into Docker's bind-mount error, which is unreadable.
func (s Static) Path(ctx context.Context, worktreeID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	raw, ok := s[worktreeID]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownID, worktreeID)
	}
	return Verify(raw)
}

// Verify cleans a path, makes it absolute and confirms it is a directory. It
// is exported because project.Scanner resolves ids a completely different way
// and must end with exactly this check — one implementation, so the two
// providers cannot disagree about what "resolvable" means.
func Verify(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("%w: %q: %v", ErrNotADir, path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("%w: %q: %v", ErrNotADir, abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: %q is a file", ErrNotADir, abs)
	}
	return abs, nil
}
