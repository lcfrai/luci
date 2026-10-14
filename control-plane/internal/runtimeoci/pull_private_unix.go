//go:build !windows

package runtimeoci

import "os"

func protectPullPath(name string, directory bool) error {
	if directory {
		return os.Chmod(name, 0700)
	}
	return os.Chmod(name, 0600)
}
