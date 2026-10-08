//go:build !unix && !windows

package product

import (
	"errors"
	"os"
)

func swarmWorkspaceSingleLink(file *os.File) error {
	return errors.New("this platform cannot verify ordinary retained file ownership")
}
