//go:build linux

package microvmguest

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Protect uses the template's existing unprivileged dev identity. The immutable
// context is a sibling of writable source, under a root-owned parent. A worker
// cannot replace the context through a writable containing directory.
func Protect(p Prepared, uid, gid int) error {
	if e := validatePrepared(p); e != nil {
		return e
	}
	if _, e := os.Lstat(filepath.Join(p.directory, "launch-reserved.json")); !os.IsNotExist(e) {
		return ErrRefused
	}
	if os.Geteuid() != 0 || uid <= 0 || gid <= 0 {
		return ErrRefused
	}
	for _, sub := range []string{"source", "context"} {
		root := filepath.Join(p.directory, sub)
		if e := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, e := entry.Info()
			if e != nil {
				return e
			}
			if entry.Type()&os.ModeSymlink != 0 || (!entry.IsDir() && !info.Mode().IsRegular()) {
				return ErrRefused
			}
			owner, group, mode := 0, 0, os.FileMode(0444)
			if sub == "source" {
				owner, group = uid, gid
				mode = 0644
			}
			if entry.IsDir() {
				if sub == "source" {
					mode = 0755
				} else {
					mode = 0555
				}
			} else if sub == "source" {
				relative, e := filepath.Rel(p.directory, path)
				if e != nil {
					return e
				}
				relative = filepath.ToSlash(relative)
				found := false
				for _, f := range p.manifest.Files {
					if f.Path == relative {
						mode = os.FileMode(f.Mode)
						found = true
						break
					}
				}
				if !found {
					return ErrRefused
				}
			}
			if e = os.Chown(path, owner, group); e != nil {
				return e
			}
			return os.Chmod(path, mode)
		}); e != nil {
			return e
		}
	}
	if e := os.Chown(p.directory, 0, 0); e != nil {
		return e
	}
	return os.Chmod(p.directory, 0755)
}
