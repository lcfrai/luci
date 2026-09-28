// Package hostpath decides whether a host path is one a sandbox can write.
//
// Phase 3's ruling R4 rule 6 is "the host never trusts anything a sandbox can
// write". Phase 2 mounts two kinds of host path into a sandbox: a worktree,
// read-WRITE, and a file the host's own git config names, read-only. Both
// choices are made from something an agent can steer — `git worktree list`
// runs in the agent-writable primary worktree (X-1 review O-1), and a link the
// agent owns can be swapped between the config's file and its target (X-1
// review m-5). This package is the ONE place that answers where the projects
// directory begins and ends, so the two callers cannot disagree (§3D).
//
// The over-approximation it uses — "a sandbox can write exactly what is under
// the projects root" — is sound only because Confine holds. Confine is what
// keeps a mounted worktree under the root, and the root is what ResolveOutside
// refuses to walk through. Loosen either and the other stops meaning anything.
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

// ErrOutside is returned when a path that had to be inside the projects
// directory is not.
var ErrOutside = errors.New("hostpath: the path is outside the projects directory")

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
// location is inside root. It is the gate every worktree mount passes: a
// worktree id resolves through `git worktree list`, whose answer the agent
// chooses, so the host checks the place rather than trusting the name (X-1
// review O-1).
//
// root "" confines nothing, because no projects directory is configured and
// the only provider then is the operator's own static map.
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
		return "", fmt.Errorf("%w: the projects directory %q: %v", ErrOutside, root, err)
	}
	if !within(realRoot, real) {
		return "", fmt.Errorf("%w: %q is really %q, and %q is the projects directory", ErrOutside, path, real, realRoot)
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
