//go:build linux

package dockerbe

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"os"
	"strings"
)

func probeDaemonDisk(_ context.Context, api APIClient, info system.Info) (resourcebudget.Disk, error) {
	endpoint, ok := api.(interface{ DaemonHost() string })
	hostname, e := os.Hostname()
	if !ok || e != nil || hostname != info.Name || info.OSType != "linux" || !strings.HasPrefix(endpoint.DaemonHost(), "unix://") || !validDaemonDiskTarget(info.DockerRootDir, info.ID) {
		return resourcebudget.Disk{}, errors.New("daemon filesystem does not have a verified local scope")
	}
	// statfs does not read private files inside Docker's root. Docker-group
	// users normally cannot read engine-id (0600), even on the same host.
	disk, err := resourcebudget.ObserveDisk(resourcebudget.DiskPath{Role: "docker", Path: info.DockerRootDir})
	if err == nil {
		disk.Basis = "local-unix-daemon-api-root-matching-hostname"
	}
	return disk, err
}
