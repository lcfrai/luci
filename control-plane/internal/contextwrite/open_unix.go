//go:build linux || darwin

package contextwrite

import (
	"os"
	"syscall"
)

// openNoFollow opens path for reading without following a final link
// (O_NOFOLLOW) and without waiting on a FIFO swapped in after the check
// (O_NONBLOCK, which a regular file ignores). readChecked still compares the
// opened file with the one it checked.
func openNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
