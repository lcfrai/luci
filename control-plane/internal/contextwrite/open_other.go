//go:build !linux && !darwin

package contextwrite

import "os"

// openNoFollow is os.Open here: package os has no O_NOFOLLOW on Windows.
// readChecked's os.SameFile against the Lstat it checked still refuses a
// link swapped in between the check and the open.
func openNoFollow(path string) (*os.File, error) { return os.Open(path) }
