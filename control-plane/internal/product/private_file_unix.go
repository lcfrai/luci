//go:build !windows

package product

import "os"

func restrictPrivateFile(path string) error { return os.Chmod(path, 0600) }
