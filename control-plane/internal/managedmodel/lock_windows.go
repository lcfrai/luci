//go:build windows

package managedmodel

import (
	"context"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"golang.org/x/sys/windows"
	"os"
	"time"
)

func lock(ctx context.Context, path string, exclusive bool) (func(), error) {
	if before, e := os.Lstat(path); e == nil && !before.Mode().IsRegular() {
		return nil, errors.New("managed model barrier is not an ordinary file")
	} else if e != nil && !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	before, e := os.Lstat(path)
	opened, e2 := f.Stat()
	if e != nil || e2 != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("managed model barrier changed")
	}
	if e = hostpath.OneName(path); e != nil {
		f.Close()
		return nil, e
	}
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	over := &windows.Overlapped{}
	for {
		if err = ctx.Err(); err != nil {
			f.Close()
			return nil, err
		}
		err = windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, over)
		if err == nil {
			return func() { _ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, over); _ = f.Close() }, nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
