package dockerbe

import (
	"os"

	"github.com/docker/docker/api/types/container"
)

func configureHostIdentity(cfg *container.Config, host *container.HostConfig) {
	mapHostIdentity(cfg, host, os.Geteuid(), os.Getegid())
}
