//go:build unix

package hostpath

import (
	"fmt"
	"os"
	"syscall"
)

// linkCount is the file's st_nlink, following a link to the file it names.
func linkCount(p string) (uint64, error) {
	info, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("the stat of %q carries no link count", p)
	}
	return uint64(st.Nlink), nil
}
