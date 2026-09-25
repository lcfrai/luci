package main

import (
	"os"
	"path/filepath"
	"strings"
)

// canon is the one form every path is compared in (re-review RR-M1):
// filepath.Abs, an existence check, then longPath (on Windows,
// syscall.GetLongPathName, which expands 8.3 short elements and keeps
// junctions as they are; nothing elsewhere), then filepath.Clean. It is never
// filepath.EvalSymlinks: a worktree's node_modules may be a junction, and
// resolving it would move every node out of its project.
//
// The existence check runs on every platform (carried RR2-M1): a missing
// path, such as a link whose target is gone, is an error and the run exits 2,
// on ubuntu CI exactly as on Windows, never "not a scanned project".
func canon(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	long, err := longPath(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(long), nil
}

// sameCanon compares two canon forms: case-insensitively on Windows.
func sameCanon(a, b string) bool {
	if caseInsensitivePaths {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// insideCanon is "dir is root or under it", for two canon forms: filepath.Rel
// with no leading ".." element (filepath.Rel folds case per element on
// Windows). Never a string prefix: "<root>-fork/x" is outside "<root>".
func insideCanon(root, dir string) bool {
	rel, err := filepath.Rel(root, dir)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// samePath reports whether a and b canon to the same path; false when either
// cannot be canon'd (a source canons first, so its error exits 2).
func samePath(a, b string) bool {
	ca, err := canon(a)
	if err != nil {
		return false
	}
	cb, err := canon(b)
	if err != nil {
		return false
	}
	return sameCanon(ca, cb)
}

// insideRoot reports whether dir is root or under it, both canon'd; false
// when either cannot be canon'd.
func insideRoot(root, dir string) bool {
	cr, err := canon(root)
	if err != nil {
		return false
	}
	cd, err := canon(dir)
	if err != nil {
		return false
	}
	return insideCanon(cr, cd)
}
