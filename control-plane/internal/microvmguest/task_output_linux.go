//go:build linux

package microvmguest

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// PrepareTaskOutput supplies the same dedicated handoff path used by local
// upstream sessions. The actual output directory is under the root-owned
// prepared attempt, outside writable source and immutable reviewed context.
// The home link is only a convenience for the unprivileged harness; capture
// reads the confined attempt directory, never follows that link.
func PrepareTaskOutput(p Prepared, homeLink string, uid, gid int) error {
	if err := validatePrepared(p); err != nil || os.Geteuid() != 0 || uid <= 0 || gid <= 0 ||
		!filepath.IsAbs(homeLink) || filepath.Base(homeLink) != ".luci-task-output" {
		return ErrRefused
	}
	parent := filepath.Dir(homeLink)
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrRefused
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != uid || info.Mode().Perm()&0022 != 0 {
		return ErrRefused
	}
	if _, err = os.Lstat(homeLink); !errors.Is(err, os.ErrNotExist) {
		return ErrRefused
	}
	dir := filepath.Join(p.directory, "task-output")
	if err = os.Mkdir(dir, 0700); err != nil {
		return ErrRefused
	}
	if err = os.Chown(dir, uid, gid); err != nil {
		return err
	}
	if err = os.Symlink(dir, homeLink); err != nil {
		return err
	}
	return nil
}
