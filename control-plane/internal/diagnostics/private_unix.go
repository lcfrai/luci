//go:build !windows

package diagnostics

import "os"

// Same private-file convention as management.atomicPrivate. Callers validate
// ownership and links before applying permissions.
func securePrivate(path string, directory bool) error {
	mode := os.FileMode(0600)
	if directory {
		mode = 0700
	}
	return os.Chmod(path, mode)
}
