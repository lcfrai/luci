package dockerbe

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestLinuxIdentityUsesProcessCredentialsInsteadOfEnvironment(t *testing.T) {
	t.Setenv("LUCI_HOST_UID", "0")
	t.Setenv("LUCI_HOST_GID", "0")
	cfg := &container.Config{}
	configureHostIdentity(cfg, &container.HostConfig{})
	if os.Geteuid() <= 0 || os.Getegid() <= 0 {
		if cfg.User != "" || len(cfg.Env) != 0 {
			t.Fatal("a root control plane enabled root agent execution")
		}
		return
	}
	want := "LUCI_HOST_UID=" + strconv.Itoa(os.Geteuid()) + ",LUCI_HOST_GID=" + strconv.Itoa(os.Getegid())
	if strings.Join(cfg.Env, ",") != want {
		t.Fatalf("identity did not come from real host process: %v", cfg.Env)
	}
}
