package previewgateway

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strconv"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

func confine(root, path string) (string, error) {
	return hostpath.ConfineToMounts([]string{root}, path)
}
func portString(port int) string { return strconv.Itoa(port) }

// DockerVerifier reads only ownership and bindings, never a container's private
// environment. Rechecking at request time prevents reuse of an old host port.
func DockerVerifier(dockerHost string) Verify {
	return func(ctx context.Context, t Target) error {
		if t.Kind == "bridge" {
			return verifyBridge(ctx, t)
		} // published by the owned worker bridge
		args := []string{}
		if dockerHost != "" {
			args = append(args, "--host", dockerHost)
		}
		args = append(args, "inspect", "--format", `{"running":{{json .State.Running}},"labels":{{json .Config.Labels}},"ports":{{json .NetworkSettings.Ports}}}`, t.Container)
		b, err := exec.CommandContext(ctx, "docker", args...).Output()
		if err != nil {
			return err
		}
		return verifyContainer(t, b)
	}
}
func verifyContainer(t Target, b []byte) error {
	var v struct {
		Running bool
		Labels  map[string]string
		Ports   map[string][]struct{ HostIP, HostPort string }
	}
	if json.Unmarshal(b, &v) != nil || !v.Running {
		return errors.New("owned container is stopped")
	}
	for k, value := range t.Labels {
		if v.Labels[k] != value {
			return errors.New("container ownership changed")
		}
	}
	u, err := validate(t, 0)
	if err != nil {
		return err
	}
	for _, bindings := range v.Ports {
		if len(bindings) == 1 && bindings[0].HostIP == "127.0.0.1" && bindings[0].HostPort == u.Port() {
			return nil
		}
	}
	return errors.New("owned loopback port changed")
}
