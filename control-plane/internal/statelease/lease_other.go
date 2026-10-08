//go:build !linux && !darwin && !windows

package statelease

import (
	"errors"
	"os"
)

func Acquire(dir, releaseVersion string) (*os.File, error) {
	return nil, errors.New("state leases require Linux, macOS, or Windows")
}
