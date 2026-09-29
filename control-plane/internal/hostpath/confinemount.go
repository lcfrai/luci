package hostpath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrReparsePoint is returned when a component that a sandbox can write is a
// symlink or a junction on the way to something the host was about to mount
// read-write.
var ErrReparsePoint = errors.New("hostpath: a mount component inside the read-write worktree is a reparse point")

// ConfineToMount refuses a reparse point at target, or at any component of the
// path to it that lies INSIDE base, and returns target's cleaned absolute path.
//
// base is a worktree the host is about to mount read-WRITE. Every component
// below base is the agent's to swap, so a directory entry there that is really a
// symlink or a junction lets the agent aim the read-write mount at any directory
// on the host (X-3 review 3 I-B). registers() proves the project's own
// host-owned registry NAMES this location, but the location's own directory
// entry is not host-owned, and Path hands the unresolved location to the mount.
//
// A component AT or ABOVE base is host-owned (the projects directory and up are
// not in any sandbox's mount), and a target OUTSIDE base entirely is too: a
// linked worktree registered outside the projects directory is the e2e
// fixture's own layout (desktop/e2e/fixture.sh), reached through a link on
// purpose. Neither is checked; the walk only starts refusing once it has passed
// THROUGH base, found by file identity so a link cannot spoof the boundary.
//
// The mask is sessionWorktreeAdmin's Lstat (internal/api/sessions.go): a
// junction (os.ModeIrregular; mklink /J needs no privilege on Windows and
// filepath.EvalSymlinks does not resolve it) is refused as well as a symlink,
// because a drvfs symlink an agent writes inside its Linux mount surfaces to the
// host as a symlink (a relative target) or a junction (an absolute one) and
// os.Stat follows it — measured on this host, X-3 review 3 I-B.
//
// base "" mounts nothing under a projects directory, so nothing is refused.
func ConfineToMount(base, target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if base == "" {
		return abs, nil
	}
	baseInfo, err := os.Stat(base)
	if err != nil {
		return "", fmt.Errorf("%w: the worktree's project %q: %v", ErrOutside, base, err)
	}
	cur, rest := volumeRoot(abs), components(abs)
	inside := false
	for len(rest) > 0 {
		cur, rest = filepath.Join(cur, rest[0]), rest[1:]
		info, err := os.Lstat(cur)
		if err != nil {
			return "", err
		}
		// Only once the walk is inside base is a reparse point the agent's to
		// plant; base itself and its ancestors are host-owned and skipped. The
		// answer (the last component) is checked because inside is set by then.
		if inside && info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return "", fmt.Errorf("%w: %q is inside the read-write worktree %q, so an agent there can aim it at any directory on this host", ErrReparsePoint, cur, base)
		}
		if !inside && os.SameFile(info, baseInfo) {
			inside = true
		}
	}
	return abs, nil
}
