//go:build !windows

package dockerinfo

import "os/exec"

func hideWindow(*exec.Cmd) {}
