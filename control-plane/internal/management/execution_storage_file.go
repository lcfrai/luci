package management

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

// All components are operator state, outside any browser-selected project.
// Refuse symlinks/reparse points before creating or changing permissions.
func executionStorageDirectory(dir string, create bool) error {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return errors.New("execution storage needs an absolute canonical directory")
	}
	var missing []string
	for current := dir; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) && create {
			missing = append(missing, current)
		} else if err != nil {
			return err
		} else if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("execution storage ancestors must be ordinary directories")
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := os.Lstat(missing[i])
		if err != nil || !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("execution storage directory changed during creation")
		}
		if err := privatePath(missing[i]); err != nil {
			return err
		}
	}
	return nil
}

func lockExecutionStorage(ctx context.Context, dir string) (func(), error) {
	if err := executionStorageDirectory(dir, true); err != nil {
		return nil, err
	}
	if err := privatePath(dir); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "local-storage-confirmation.lock")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || hostpath.OneName(path) != nil {
			return nil, errors.New("execution storage lock is not an ordinary private file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	good := false
	defer func() {
		if !good {
			_ = f.Close()
		}
	}()
	opened, err := f.Stat()
	named, statErr := os.Lstat(path)
	if err != nil || statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(opened, named) || hostpath.OneName(path) != nil {
		return nil, errors.New("execution storage lock identity changed")
	}
	if err = privatePath(path); err != nil {
		return nil, err
	}
	unlock, err := lockExecutionStorageFile(ctx, f)
	if err != nil {
		return nil, err
	}
	good = true
	return func() { unlock(); _ = f.Close() }, nil
}

func writeExecutionStorage(path string, data []byte) error {
	if len(data) > 16<<10 {
		return errors.New("execution storage confirmation exceeds private record limit")
	}
	if err := executionStorageDirectory(filepath.Dir(path), false); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".local-storage-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = privatePath(f.Name()); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return publishExecutionStorage(f.Name(), path)
}
