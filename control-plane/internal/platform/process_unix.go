//go:build !windows

package platform

import (
	"os"
	"os/exec"
)

func hide(cmd *exec.Cmd) {}
func protect(path string) error {
	v, e := os.Stat(path)
	if e != nil {
		return e
	}
	mode := os.FileMode(0600)
	if v.IsDir() {
		mode = 0700
	}
	return os.Chmod(path, mode)
}
