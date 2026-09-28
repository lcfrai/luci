// Package hostpath decides whether a host path is one a sandbox can write.
//
// Phase 3's ruling R4 rule 6 is "the host never trusts anything a sandbox can
// write". Phase 2 mounts host paths into a sandbox that were chosen from
// something an agent can steer — a link the agent owns, swapped between the
// host git config's file and its target (X-1 review m-5), or a directory that
// is really somewhere else (X-1 review O-2). This package holds the three
// primitives those checks share, so no caller invents its own idea of where a
// path really is (§3D):
//
//   - Real: a place, not a name.
//   - Confine: this path's real location must be inside that one.
//   - ResolveOutside: no STEP of this resolution may be that directory.
//
// ResolveOutside's root is the projects directory. That is an
// over-approximation of "a sandbox can write it" in one direction and an
// UNDER-approximation in another: a linked worktree registered outside the
// projects root is mounted read-write too, and a chain through one of those is
// not refused. The X-1 reviewer named that residue when it sent m-5 here, and
// R4 rule 1 closes it in phase 3 by not mounting the project repository at all.
//
// A root of "" is an unconfigured projects directory: there is nothing under
// it, so nothing is refused.
package hostpath

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrInside is returned when a path a sandbox can write is on the way to, or
// is, something the host was about to mount.
var ErrInside = errors.New("hostpath: the path is one a sandbox can write")

// ErrOutside is returned when a path that had to be inside a given directory
// is not. The directory is the caller's: O-2's is the worktree, not the
// projects directory, so the sentence names no particular one and Confine's
// wrapper says which it was.
var ErrOutside = errors.New("hostpath: the path is outside the directory it has to be in")

// Real is where a path actually is: absolute, cleaned, and with every symlink
// and every symlinked directory component followed. Comparing anything else
// compares names rather than places, and a link, a junction, a Windows short
// name or a different letter case is then enough to dress one place as
// another. The path must exist.
func Real(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

// Confine answers with the real location of path and fails unless that
// location is inside root. Its caller is O-2's `<worktree>/.git/worktrees`
// mount: the source has to stay in the worktree it belongs to, and the agent
// writes that worktree, so the host checks the place rather than the name.
//
// root "" confines nothing.
func Confine(root, path string) (string, error) {
	real, err := Real(path)
	if err != nil {
		return "", err
	}
	if root == "" {
		return real, nil
	}
	realRoot, err := Real(root)
	if err != nil {
		return "", fmt.Errorf("%w: %q cannot be resolved: %v", ErrOutside, root, err)
	}
	if !within(realRoot, real) {
		return "", fmt.Errorf("%w: %q is really %q, and it has to be inside %q", ErrOutside, path, real, realRoot)
	}
	return real, nil
}

// within reports whether p is root or lies under it. Both must already be
// real. Windows is compared case-insensitively for the same reason project's
// id derivation lowercases: C:\Users\x and c:\users\x are one directory.
func within(root, p string) bool {
	if root == "" {
		return false
	}
	if runtime.GOOS == "windows" {
		root, p = strings.ToLower(root), strings.ToLower(p)
	}
	return p == root || strings.HasPrefix(p, root+string(filepath.Separator))
}
