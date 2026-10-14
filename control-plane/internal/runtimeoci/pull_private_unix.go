//go:build !windows

package runtimeoci

import (
	"errors"
	"os"
	"path/filepath"
)

func protectPullPath(name string, directory bool) error {
	// The direct parent of the stage is the already private 0700 execution
	// directory. Inside that boundary the non-root helper (uid 65532) needs
	// read/traverse access to its read-only bind mounts. No secrets are staged.
	if directory {
		return os.Chmod(name, 0755)
	}
	return os.Chmod(name, 0644)
}

func validatePullPrivateParent(name string) error {
	info, err := os.Lstat(name)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return errors.New("runtime pull parent is not private")
	}
	return nil
}

func exposePullReadOnlyTree(root string) error {
	return filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("runtime pull stage contains linked path")
		}
		if entry.IsDir() {
			return os.Chmod(name, 0755)
		}
		if !entry.Type().IsRegular() {
			return errors.New("runtime pull stage contains non-file")
		}
		return os.Chmod(name, 0644)
	})
}

func syncPullDirectory(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	err = f.Sync()
	return errors.Join(err, f.Close())
}
