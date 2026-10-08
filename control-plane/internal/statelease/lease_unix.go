//go:build linux || darwin

package statelease

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Acquire takes an exclusive, process-held advisory lock without following path
// links. The operator recovery tool uses the same flock protocol. Never unlink
// a lease: doing so could give two writers separate locks for the same state.
func Acquire(dir, releaseVersion string) (*os.File, error) {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || dir == "/" || releaseVersion == "" {
		return nil, errors.New("state lease requires a private absolute state directory and release version")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(dir, "/"), "/") {
		next, openErr := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if errors.Is(openErr, unix.ENOENT) {
			if makeErr := unix.Mkdirat(fd, part, 0700); makeErr != nil && !errors.Is(makeErr, unix.EEXIST) {
				unix.Close(fd)
				return nil, makeErr
			}
			next, openErr = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		}
		unix.Close(fd)
		if openErr != nil {
			return nil, fmt.Errorf("state directory must not contain symbolic links: %w", openErr)
		}
		fd = next
	}
	defer unix.Close(fd)
	var root unix.Stat_t
	if err := unix.Fstat(fd, &root); err != nil {
		return nil, err
	}
	if root.Uid != uint32(os.Geteuid()) || root.Mode&0077 != 0 {
		return nil, errors.New("state directory must belong to the service account with mode 0700")
	}
	var deleted unix.Stat_t
	if err := unix.Fstatat(fd, ".luci-host-deleted", &deleted, unix.AT_SYMLINK_NOFOLLOW); err == nil {
		return nil, errors.New("managed host state was marked for erasure; inspect the operator receipt before commissioning a new empty state directory")
	} else if !errors.Is(err, unix.ENOENT) {
		return nil, err
	}
	var incomplete unix.Stat_t
	if err := unix.Fstatat(fd, ".luci-host-restore-incomplete", &incomplete, unix.AT_SYMLINK_NOFOLLOW); err == nil {
		return nil, errors.New("host restore is incomplete; retain the partial state and recover into an empty directory before starting Luci")
	} else if !errors.Is(err, unix.ENOENT) {
		return nil, err
	}
	lockFD, err := unix.Openat(fd, FileName, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(lockFD), FileName)
	closeOnError := true
	defer func() {
		if closeOnError {
			file.Close()
		}
	}()
	var info unix.Stat_t
	if err := unix.Fstat(lockFD, &info); err != nil {
		return nil, err
	}
	if info.Mode&unix.S_IFMT != unix.S_IFREG || info.Nlink != 1 || info.Uid != uint32(os.Geteuid()) || info.Mode&0077 != 0 {
		return nil, errors.New("state lease must be an owned private regular file")
	}
	if err := unix.Flock(lockFD, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInUse, err)
	}
	data, _ := json.Marshal(struct {
		Version        int    `json:"version"`
		ReleaseVersion string `json:"releaseVersion"`
	}{1, releaseVersion})
	if err := file.Truncate(0); err != nil {
		return nil, err
	}
	if _, err := file.Write(data); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	closeOnError = false
	return file, nil
}
