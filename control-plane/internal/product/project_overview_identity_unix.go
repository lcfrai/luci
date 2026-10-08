//go:build unix

package product

import (
	"fmt"
	"os"
	"syscall"
)

func projectOverviewFileIdentity(file *os.File) (string, error) {
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || stat.Nlink != 1 {
		return "", errOverviewQuestions
	}
	return fmt.Sprintf("unix:%x:%x", uint64(stat.Dev), uint64(stat.Ino)), nil
}
