//go:build windows

package product

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"time"
)

func lockSwarmPolicy(ctx context.Context, file *os.File) (func(), error) {
	overlapped := &windows.Overlapped{}
	for {
		err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlapped)
		if err == nil {
			return func() { _ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlapped) }, nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
