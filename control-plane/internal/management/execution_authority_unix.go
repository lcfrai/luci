//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package management

import (
	"os"
	"syscall"
)

func executionIdentityFilePrivate(_ *os.File, info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Nlink == 1 && info.Mode().IsRegular() && info.Mode().Perm()&0077 == 0
}
