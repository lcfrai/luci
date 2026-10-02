package dockerbe

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func validateBootstrapEnvironment(env map[string]string) error {
	for key := range env {
		// A dynamic loader processes these before bash -p or any script line
		// can run. They cannot be accepted by a root bootstrap even though the
		// eventual application process is unprivileged.
		if strings.HasPrefix(key, "LD_") || key == "GCONV_PATH" || key == "GLIBC_TUNABLES" {
			return fmt.Errorf("dockerbe: loader environment %q is not supported by the sandbox bootstrap: %w", key, backend.ErrUnsupported)
		}
	}
	return nil
}

// mapHostIdentity is used only with IDs read by the control plane, never with
// project/session input. The fixed entrypoint changes image-owned account
// metadata and drops all privileges before starting sandboxd. Project mounts
// retain their host ownership and private modes.
func mapHostIdentity(cfg *container.Config, host *container.HostConfig, uid, gid int) {
	if uid <= 0 || gid <= 0 {
		return // never run an agent as host root or a root-group member
	}
	cfg.User = "0:0"
	cfg.Env = append(cfg.Env, "LUCI_HOST_UID="+strconv.Itoa(uid), "LUCI_HOST_GID="+strconv.Itoa(gid))
	// Only the trusted bootstrap receives these capabilities. DAC_OVERRIDE
	// permits its initial chdir into a 0700 host-owned project. setpriv removes
	// this entire bounding set before sandboxd or any project command runs.
	host.CapAdd = []string{"CHOWN", "DAC_OVERRIDE", "SETUID", "SETGID", "SETPCAP"}
}
