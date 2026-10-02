//go:build !windows

package product

import "os/exec"

func hideProcess(cmd *exec.Cmd) {}
