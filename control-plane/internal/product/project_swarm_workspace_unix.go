//go:build unix

package product

import (
	"errors"
	"os"
	"syscall"
)

// Inspect the opened handle, not a pathname an active worker can replace.
func swarmWorkspaceSingleLink(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return errors.New("linked file is not exposed")
	}
	return nil
}
