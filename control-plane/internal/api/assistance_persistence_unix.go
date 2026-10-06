//go:build !windows

package api

import (
	"os"
	"path/filepath"
)

func publishAssistanceReceipt(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	// Existing parents may have been created by prepareAssistRoot during this
	// request. Flush bottom-up through the existing filesystem root; a failed
	// durability boundary prevents dispatch rather than silently weakening it.
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
