//go:build unix

package api

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// sessionRemoveIn deletes name in root's subdirectory dir, and nowhere else:
// what goes is the file at root/dir/name when dir is opened, whatever dir has
// become since the caller looked at it (why: sessionForgetTranscripts).
//
// On a unix, dir is opened refusing a link at its own name (O_NOFOLLOW), and
// the file is unlinked relative to that descriptor, so a link swapped in for
// dir after the open is never walked through, and one swapped in before it
// fails the open. root itself is resolved as it stands: it is the store, the
// mount's root, and no sandbox can replace it.
//
// As os.Remove does, an empty directory at the name is removed too.
func sessionRemoveIn(root, dir, name string) error {
	p := filepath.Join(root, dir, name)
	fd, err := unix.Open(filepath.Join(root, dir), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return &os.PathError{Op: "remove", Path: p, Err: err}
	}
	defer unix.Close(fd)
	err = unix.Unlinkat(fd, name, 0)
	if err == nil {
		return nil
	}
	e1 := unix.Unlinkat(fd, name, unix.AT_REMOVEDIR)
	if e1 == nil {
		return nil
	}
	if e1 != unix.ENOTDIR {
		err = e1
	}
	return &os.PathError{Op: "remove", Path: p, Err: err}
}
