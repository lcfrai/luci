package dockerbe

import (
	"os"

	"github.com/docker/docker/api/types/container"
)

// Docker Desktop for Mac preserves permissions on host bind mounts. Match the
// macOS user's numeric owner for private projects and transcripts just as on
// native Linux; the trusted image bootstrap still drops privileges before the
// agent starts. See Docker's Mac permission requirements, "Containers running
// as root within the Linux VM".
func configureHostIdentity(cfg *container.Config, host *container.HostConfig) {
	mapHostIdentity(cfg, host, os.Geteuid(), os.Getegid())
}
