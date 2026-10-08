//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package management

import "os"

func executionIdentityFilePrivate(*os.File, os.FileInfo) bool { return false }
