package main

import (
	"fmt"
	"syscall"
)

// protectEnviron makes sandboxd non-dumpable. The session runs as sandboxd's
// user, and /proc/<pid>/environ keeps the environment sandboxd was started
// with, attach token included, whatever the Go runtime's copy now says; a
// non-dumpable process's /proc files are root's, and a process without
// CAP_SYS_PTRACE cannot read them. The sandbox image grants none.
//
// Only sandboxd is affected: the session's execve makes it dumpable again.
// sandboxd itself then writes no core dump and cannot be ptraced by its
// session, which is the point.
func protectEnviron() error {
	if _, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, syscall.PR_SET_DUMPABLE, 0, 0, 0, 0, 0); errno != 0 {
		return fmt.Errorf("sandboxd: prctl(PR_SET_DUMPABLE, 0): %w", errno)
	}
	return nil
}
