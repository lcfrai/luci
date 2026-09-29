package hostpath

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrReparsePoint is returned when a component that a sandbox can write is a
// symlink or a junction on the way to something the host was about to mount
// read-write.
var ErrReparsePoint = errors.New("hostpath: a mount component inside a read-write mount is a reparse point")

// ConfineToMounts refuses a reparse point at target, or at any component of the
// path to it, that lies INSIDE any of mounts, and returns target's cleaned
// absolute path.
//
// mounts are every directory some sandbox mounts read-WRITE (X-3 review 4 I-D):
// the projects directory and each project's primary under it, each linked
// worktree a project registers wherever it is, and the conversation stores'
// root. Every entry below one of them is some agent's to swap, so an entry
// there that is really a symlink or a junction lets that agent aim the next
// read-write mount of target at any directory on the host (X-3 review 3 I-B).
// The boundary is not target's own project: a location registered inside
// ANOTHER project's tree, or nested in a linked worktree that lives outside the
// projects directory (the e2e fixture's layout, desktop/e2e/fixture.sh, with
// the .claude/worktrees/ host-side Claude Code makes inside it), sits in a
// mount other sessions write, and a boundary of the primary alone returned it
// unchecked (measured, review 4).
//
// The walk goes the way the operating system goes. A component OUTSIDE every
// mount is the host's own, so a link there is followed — restarting at its
// target — rather than refused or stepped past: a host-made junction INTO a
// mount must still land the walk on the mount by identity, or the agent's
// junction beyond it is never seen. A component INSIDE a mount (the walk has
// passed through one, found by file identity so no spelling can spoof it) is
// refused when it is a reparse point, at the leaf or on the way. A mount that
// does not exist holds nothing, and an agent cannot create one outside the
// mounts it has; a mount that cannot be read otherwise refuses the lot.
//
// The mask is sessionWorktreeAdmin's Lstat (internal/api/sessions.go): a
// junction (os.ModeIrregular; mklink /J needs no privilege on Windows and
// filepath.EvalSymlinks does not resolve it) is refused as well as a symlink,
// because a drvfs symlink an agent writes inside its Linux mount surfaces to the
// host as a symlink (a relative target) or a junction (an absolute one) and
// os.Stat follows it — measured on this host, X-3 review 3 I-B.
func ConfineToMounts(mounts []string, target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	var roots []os.FileInfo
	for _, m := range mounts {
		info, err := os.Stat(m)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("%w: the read-write mount %q cannot be read, so what lies inside it is not known: %v", ErrOutside, m, err)
		}
		roots = append(roots, info)
	}
	cur, rest := volumeRoot(abs), components(abs)
	inside := mountAt(roots, cur)
	for hops := 0; len(rest) > 0; {
		cur, rest = filepath.Join(cur, rest[0]), rest[1:]
		info, err := os.Lstat(cur)
		if err != nil {
			return "", err
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0 {
			if inside == "" && isAny(roots, info) {
				inside = cur
			}
			continue
		}
		if inside != "" {
			return "", fmt.Errorf("%w: %q is inside %q, which a sandbox mounts read-write, so an agent there can aim it at any directory on this host", ErrReparsePoint, cur, inside)
		}
		if hops++; hops > maxLinkHops {
			return "", fmt.Errorf("hostpath: %q passes through more than %d links", target, maxLinkHops)
		}
		link, err := os.Readlink(cur)
		if err != nil {
			return "", err
		}
		// Resolved as ResolveOutside resolves it: from the link's directory, an
		// absolute target from its own volume, a rooted one from this volume.
		switch {
		case filepath.IsAbs(link):
			cur = volumeRoot(link)
		case strings.HasPrefix(link, "/") || strings.HasPrefix(link, `\`):
			cur = volumeRoot(cur)
		default:
			cur = filepath.Dir(cur)
		}
		rest = append(components(link), rest...)
		inside = mountAt(roots, cur)
	}
	return abs, nil
}

// isAny reports whether info is one of roots, by identity.
func isAny(roots []os.FileInfo, info os.FileInfo) bool {
	for _, r := range roots {
		if os.SameFile(r, info) {
			return true
		}
	}
	return false
}

// mountAt is p when p itself is one of roots (a walk that starts, or restarts,
// at a volume root or a link's directory), else "".
func mountAt(roots []os.FileInfo, p string) string {
	if info, err := os.Lstat(p); err == nil && isAny(roots, info) {
		return p
	}
	return ""
}
