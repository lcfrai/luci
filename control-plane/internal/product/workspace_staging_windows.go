package product

import "os"

// Go's Windows FileMode bits do not represent the ACL. Apply the same
// protected, current-user-only DACL used for private product files.
func restrictWorkspaceStagingDir(path string, _ os.FileInfo) error {
	return restrictPrivateFile(path)
}
