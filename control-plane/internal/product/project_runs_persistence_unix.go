//go:build !windows

package product

import (
	"os"
	"path/filepath"
)

// File bytes are already synced by publishJSON. Persist the renamed entry and
// newly created ancestor entries before granting an execution capability.
func publishRunReceipt(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	for path := filepath.Dir(to); ; path = filepath.Dir(path) {
		if err := syncProjectRunDirectory(path); err != nil {
			return err
		}
		if filepath.Dir(path) == path {
			return nil
		}
	}
}
