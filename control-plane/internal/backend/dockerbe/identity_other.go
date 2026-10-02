//go:build !linux && !darwin

package dockerbe

import "github.com/docker/docker/api/types/container"

// Windows access is mediated by Docker Desktop and host ACLs, not a POSIX
// numeric user ID. Keep the image's unprivileged account on this platform.
func configureHostIdentity(*container.Config, *container.HostConfig) {}
