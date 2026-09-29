//go:build !windows && !unix

package hostpath

import "errors"

// linkCount has no source on this platform, so OneName refuses every file:
// a count that cannot be read is not a count of one.
func linkCount(string) (uint64, error) {
	return 0, errors.New("this platform reports no link count")
}
