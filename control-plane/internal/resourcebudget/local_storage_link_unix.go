//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package resourcebudget

import (
	"os"
	"syscall"
)

func localStorageSingleLink(info os.FileInfo) bool {
	value, ok := info.Sys().(*syscall.Stat_t)
	return ok && value.Nlink == 1
}
