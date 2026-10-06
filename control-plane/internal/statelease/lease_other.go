//go:build !linux

package statelease

import (
	"errors"
	"os"
)

const FileName = ".luci-state.lock"

func Acquire(dir, releaseVersion string) (*os.File, error) {
	return nil, errors.New("managed host offline recovery leases currently require Linux")
}
