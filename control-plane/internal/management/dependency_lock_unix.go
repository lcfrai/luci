//go:build !windows

package management

import (
	"context"
	"errors"
)

func acquireDependencyHostInstallation(context.Context) (func(), error) {
	return nil, errors.New("managed dependency installation is available on Windows only")
}
