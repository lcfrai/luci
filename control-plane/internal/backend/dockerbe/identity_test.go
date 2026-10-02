package dockerbe

import (
	"errors"
	"reflect"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestHostIdentityBootstrapKeepsPrivateMountOwnershipAndCannotSelectRoot(t *testing.T) {
	cfg := &container.Config{Env: []string{"LUCI_COMMAND=/bin/bash"}, WorkingDir: "/home/dev/work"}
	host := &container.HostConfig{Binds: []string{"/private-project:/home/dev/work", "/private-transcript:/home/dev/.claude/projects"}, CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"}}
	mapHostIdentity(cfg, host, 1000, 1002)
	if cfg.User != "0:0" || !reflect.DeepEqual(cfg.Env, []string{"LUCI_COMMAND=/bin/bash", "LUCI_HOST_UID=1000", "LUCI_HOST_GID=1002"}) {
		t.Fatal("bootstrap did not receive the trusted unprivileged host IDs")
	}
	if !reflect.DeepEqual([]string(host.CapAdd), []string{"CHOWN", "DAC_OVERRIDE", "SETUID", "SETGID", "SETPCAP"}) || !reflect.DeepEqual([]string(host.CapDrop), []string{"ALL"}) || !reflect.DeepEqual(host.SecurityOpt, []string{"no-new-privileges:true"}) {
		t.Fatal("bootstrap privilege set expanded")
	}
	if cfg.WorkingDir != "/home/dev/work" || !reflect.DeepEqual(host.Binds, []string{"/private-project:/home/dev/work", "/private-transcript:/home/dev/.claude/projects"}) {
		t.Fatal("identity mapping changed the private bind mounts")
	}
	for _, ids := range [][2]int{{0, 1000}, {1000, 0}, {-1, 1000}, {1000, -1}} {
		image := &container.Config{User: "dev"}
		settings := &container.HostConfig{}
		mapHostIdentity(image, settings, ids[0], ids[1])
		if image.User != "dev" || len(image.Env) != 0 || len(settings.CapAdd) != 0 {
			t.Fatalf("invalid/root host IDs selected privileged execution: %v", ids)
		}
	}
}

func TestSessionCannotOverrideTrustedHostIdentity(t *testing.T) {
	for _, key := range []string{"LUCI_HOST_UID", "LUCI_HOST_GID"} {
		spec := backend.Spec{ID: "s-host-id", Image: "luci/sandbox:dev", Command: "/bin/bash", AttachToken: "fixture", Resources: backend.Resources{CPUMillis: 1000, MemoryMiB: 1024}, Env: map[string]string{key: "0"}}
		if err := backend.ValidateSpec(spec); err == nil {
			t.Fatalf("project-controlled %s reached container bootstrap", key)
		}
		for _, entry := range spec.EnvEntries() {
			if entry.Name == key {
				t.Fatalf("environment backstop retained project-controlled %s", key)
			}
		}
		spec.Env = nil
		if err := backend.ValidateSpec(spec); err != nil {
			t.Fatal("identity rejection fixture was invalid independently of its reserved key", err)
		}
	}
	for _, key := range []string{"LD_PRELOAD", "LD_LIBRARY_PATH", "LD_AUDIT", "GCONV_PATH", "GLIBC_TUNABLES"} {
		if err := validateBootstrapEnvironment(map[string]string{key: "/home/dev/work/evil.so"}); !errors.Is(err, backend.ErrUnsupported) {
			t.Fatalf("loader injection %s was not refused before privileged startup", key)
		}
	}
	if err := validateBootstrapEnvironment(map[string]string{"ANTHROPIC_API_KEY": "fixture", "LANG": "C.UTF-8"}); err != nil {
		t.Fatal("ordinary agent environment was refused", err)
	}
}

func TestMacStyleHostIdentityRetainsThePrivateOwner(t *testing.T) {
	cfg := &container.Config{WorkingDir: "/home/dev/work"}
	settings := &container.HostConfig{CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"}}
	mapHostIdentity(cfg, settings, 501, 20)
	if cfg.User != "0:0" || !reflect.DeepEqual(cfg.Env, []string{"LUCI_HOST_UID=501", "LUCI_HOST_GID=20"}) {
		t.Fatal("private macOS owner was replaced with the image's UID1001")
	}
}
