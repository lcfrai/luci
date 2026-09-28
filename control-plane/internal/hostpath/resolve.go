package hostpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxLinkHops bounds one resolution's links. Linux's own limit is 40, and a
// cycle has to end in an error rather than a hang.
const maxLinkHops = 40

// ResolveOutside resolves path the way the operating system does — component
// by component, following every symlink and restarting at each target the way
// POSIX does — and refuses as soon as ANY step of that walk IS the projects
// directory. It answers the real location otherwise.
//
// Why every step and not the answer. X-1 review m-5: the host's own global git
// config names an excludes file, and the host mounts it into every sandbox. A
// dotfile manager links ~/.config/git/ignore into a dotfiles repository; if
// that repository is under the projects root, an agent running in it swaps the
// file for a link to ~/.ssh/id_rsa. Both ends of that chain are outside the
// root, so a check of the named path or of the resolved path lets the key
// through. The hop in the middle is where the agent chose. The same holds one
// level up, for a DIRECTORY on the way, which is why the walk descends rather
// than calling EvalSymlinks and comparing once.
//
// Each step is compared to the root by FILE IDENTITY, not by name. A name
// comparison is bypassable here and was measured to be: on Windows a temp
// directory comes back as C:\Users\AU-DEV~1\… and its real name is
// C:\Users\au-dev-dmackie1\…, so a link an agent writes in 8.3 short form, or
// in another letter case, has a different prefix and the same target. Checking
// each step also makes ancestry unnecessary: a descendant of the root is only
// reachable through the root, and a link that jumps into the middle of it
// restarts the walk at the volume, so the root is a step of every such walk.
//
// A path that does not exist returns an error wrapping fs.ErrNotExist, which
// is the caller's cue that the host's git reads nothing there either.
func ResolveOutside(root, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var rootInfo os.FileInfo
	if root != "" {
		if rootInfo, err = os.Stat(root); err != nil {
			return "", fmt.Errorf("%w: the projects directory %q: %v", ErrOutside, root, err)
		}
	}

	cur, rest := volumeRoot(abs), components(abs)
	for hops := 0; len(rest) > 0; {
		cur, rest = filepath.Join(cur, rest[0]), rest[1:]
		if isRoot(rootInfo, cur) {
			return "", fmt.Errorf("%w: reaching %q passes through the projects directory %q, at %q", ErrInside, path, root, cur)
		}
		info, err := os.Lstat(cur)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		if hops++; hops > maxLinkHops {
			return "", fmt.Errorf("hostpath: %q passes through more than %d links", path, maxLinkHops)
		}
		target, err := os.Readlink(cur)
		if err != nil {
			return "", err
		}
		// A target is resolved from the directory holding the link, an
		// absolute one from its own volume, and a rooted Windows one from the
		// volume the walk is already on.
		switch {
		case filepath.IsAbs(target):
			cur = volumeRoot(target)
		case strings.HasPrefix(target, "/") || strings.HasPrefix(target, `\`):
			cur = volumeRoot(cur)
		default:
			cur = filepath.Dir(cur)
		}
		rest = append(components(target), rest...)
	}
	// cur holds no unresolved link now, so this only normalises it: the case
	// the filesystem stores, and long names in place of 8.3 short ones.
	return Real(cur)
}

// isRoot reports whether p is the projects directory itself, by identity.
func isRoot(root os.FileInfo, p string) bool {
	if root == nil {
		return false
	}
	info, err := os.Lstat(p)
	return err == nil && os.SameFile(info, root)
}

// volumeRoot is the point a walk of p starts from: "/" on Unix, `C:\` or
// `\\host\share\` on Windows.
func volumeRoot(p string) string {
	return filepath.VolumeName(p) + string(filepath.Separator)
}

// components splits p into the names between its separators, with the volume
// and every empty run dropped.
func components(p string) []string {
	var out []string
	for _, c := range strings.Split(filepath.ToSlash(p[len(filepath.VolumeName(p)):]), "/") {
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}
