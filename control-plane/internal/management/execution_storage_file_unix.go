//go:build !windows

package management

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

func lockExecutionStorageFile(ctx context.Context, f *os.File) (func(), error) {
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func publishExecutionStorage(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	// Directory entries along a freshly created state path must also survive
	// restart. Sync bottom-up; do not report uncertain publication as success.
	for dir := filepath.Dir(to); ; dir = filepath.Dir(dir) {
		f, err := os.Open(dir)
		if err != nil {
			return err
		}
		err = f.Sync()
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if filepath.Dir(dir) == dir {
			return nil
		}
	}
}
