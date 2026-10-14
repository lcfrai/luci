package dockerbe

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

func TestPullHelperOnlyOwnedReadOnlyLoopbackConfiguration(t *testing.T) {
	id := backend.RuntimeImageIdentity{Protocol: 1, EngineID: "engine-one", InstanceID: "instance-one", AllocationID: "runtime-one", JobID: "job-one", ScopeDigest: strings.Repeat("a", 64), SourceDigest: strings.Repeat("b", 64)}
	stage := filepath.Join(t.TempDir(), "store")
	bundle := &runtimeoci.PullBundle{ManifestSHA256: strings.Repeat("c", 64)}
	bundle.Manifest.RegistryStore.SHA256 = strings.Repeat("d", 64)
	bundle.Manifest.RegistryStore.Repository = "luci/runtime-installed"
	bundle.Manifest.Image.ManifestDigest = "sha256:" + strings.Repeat("e", 64)
	bundle.Manifest.Bootstrap.ImageID = "sha256:" + strings.Repeat("f", 64)
	p := backend.PreparedRuntimeImage{Spec: backend.RuntimeImageSpec{Identity: id, PullStagingDirectory: stage}, PullBundle: bundle}
	r := runtimeoci.PullStageReceipt{Version: 1, ManifestSHA256: bundle.ManifestSHA256, StoreSHA256: bundle.Manifest.RegistryStore.SHA256, Repository: bundle.Manifest.RegistryStore.Repository, ManifestDigest: bundle.Manifest.Image.ManifestDigest, AllowlistSHA256: strings.Repeat("0", 64), AllowlistBytes: 128}
	cfg, host, network, err := pullHelperConfiguration(p, r)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Image != bundle.Manifest.Bootstrap.ImageID || cfg.User != "65532:65532" || len(cfg.Env) != 3 || host.ReadonlyRootfs != true || host.Memory != 256<<20 || host.MemorySwap != host.Memory || len(host.CapDrop) != 1 || host.CapDrop[0] != "ALL" || host.PublishAllPorts || len(host.Mounts) != 1 || !host.Mounts[0].ReadOnly || host.Mounts[0].Source != stage || len(host.PortBindings) != 1 || host.PortBindings[pullHelperPort][0].HostIP != "127.0.0.1" || network.Driver != "bridge" || network.Internal || network.Attachable {
		t.Fatalf("helper exposure or ownership drift: config=%+v host=%+v network=%+v", cfg, host, network)
	}
	r.AllowlistSHA256 = "not-a-hash"
	if _, _, _, err := pullHelperConfiguration(p, r); err == nil {
		t.Fatal("changed allowlist accepted")
	}
}
