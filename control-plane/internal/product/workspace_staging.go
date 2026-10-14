package product

import (
	"errors"
	"os"
)

func prepareWorkspaceStagingDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("private workspace staging directory unavailable")
	}
	if err := restrictWorkspaceStagingDir(path, info); err != nil {
		return errors.New("private workspace staging directory unavailable")
	}
	return nil
}
