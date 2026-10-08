//go:build !linux && !windows

package dockerbe

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func probeDaemonDisk(context.Context, APIClient, system.Info) (resourcebudget.Disk, error) {
	return resourcebudget.Disk{}, errors.New("daemon filesystem capacity observation is unsupported on this platform")
}
