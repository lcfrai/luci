package lanes

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// RemoveTree deletes a directory tree that git wrote. It refuses a path that is not strictly
// under root, and a path reached through a component between root and it that is not a real
// directory by Lstat (a link or junction there would carry the delete outside root), then runs
// os.RemoveAll. There is no read-only walk (carried t01-D6): git leaves object and pack files
// read-only on Windows, and os.RemoveAll already deletes them on both toolchains this module
// builds with. go1.26's removeall_at.go deletes through windows.Deleteat, which passes
// FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE; go1.23's removeall_noat.go deletes through
// os.Remove, which clears FILE_ATTRIBUTE_READONLY and retries. TestRemoveTreeDeletesReadOnlyObjects
// is a regression pin for that, and passes with no walk.
func RemoveTree(root, path string) error {
	if root == "" || path == "" {
		return fmt.Errorf("lanes: RemoveTree needs a root and a path")
	}
	r, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("lanes: root %s: %w", root, err)
	}
	p, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("lanes: path %s: %w", path, err)
	}
	rel, err := filepath.Rel(r, p)
	sep := string(filepath.Separator)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+sep) || filepath.IsAbs(rel) {
		return fmt.Errorf("lanes: %s is not under %s; nothing removed", p, r)
	}
	segs := strings.Split(rel, sep)
	cur := r
	for _, seg := range segs[:len(segs)-1] {
		cur = filepath.Join(cur, seg)
		fi, err := os.Lstat(cur)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil // nothing is there to remove
		case err != nil:
			return fmt.Errorf("lanes: %s: %w", cur, err)
		case !realDir(fi):
			return fmt.Errorf("lanes: %s is not a real directory; %s not removed", cur, p)
		}
	}
	if err := os.RemoveAll(p); err != nil {
		return fmt.Errorf("lanes: remove %s: %w", p, err)
	}
	return nil
}
