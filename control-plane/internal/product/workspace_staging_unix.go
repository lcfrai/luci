//go:build !windows

package product

import "os"

func restrictWorkspaceStagingDir(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0077 != 0 {
		return os.ErrPermission
	}
	return nil
}
