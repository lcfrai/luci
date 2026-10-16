//go:build !windows

package api

import (
	"os"
	"path/filepath"
)

func publishAssemblyCatalogObject(from, to string) error {
	// link is an atomic no-replace publication of the exact owned synced inode.
	// Remove our temporary name before admitting single-name immutable reads.
	if err := os.Link(from, to); err != nil {
		return err
	}
	if err := os.Remove(from); err != nil {
		return err
	}
	for path := filepath.Dir(to); ; path = filepath.Dir(path) {
		dir, err := os.Open(path)
		if err != nil {
			return err
		}
		err = dir.Sync()
		closeErr := dir.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if filepath.Dir(path) == path {
			return nil
		}
	}
}
