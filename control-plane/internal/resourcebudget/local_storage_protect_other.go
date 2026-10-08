//go:build !windows

package resourcebudget

import "os"

func protectLocalStorageFile(f *os.File) error { return f.Chmod(0600) }
