//go:build !windows && !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris

package resourcebudget

import "os"

func localStorageSingleLink(os.FileInfo) bool { return false }
