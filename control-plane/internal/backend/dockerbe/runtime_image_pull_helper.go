package dockerbe

import (
	"errors"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

const pullHelperPort = nat.Port("5000/tcp")

func pullHelperNames(id backend.RuntimeImageIdentity) (networkName, containerName string, err error) {
	if !validRuntimeIdentity(id, true) || len(id.AllocationID) > 100 {
		return "", "", backend.ErrLaunchFenceOwnership
	}
	return "luci-runtime-pull-net-" + id.AllocationID, "luci-runtime-pull-helper-" + id.AllocationID, nil
}

func pullHelperConfiguration(prepared backend.PreparedRuntimeImage, receipt runtimeoci.PullStageReceipt) (*container.Config, *container.HostConfig, network.CreateOptions, error) {
	bundle := prepared.PullBundle
	id := prepared.Spec.Identity
	networkName, _, err := pullHelperNames(id)
	if err != nil || bundle == nil || receipt.Version != 1 || receipt.ManifestSHA256 != bundle.ManifestSHA256 || receipt.StoreSHA256 != bundle.Manifest.RegistryStore.SHA256 || receipt.Repository != bundle.Manifest.RegistryStore.Repository || receipt.ManifestDigest != bundle.Manifest.Image.ManifestDigest || receipt.AllowlistBytes < 1 || receipt.AllowlistBytes > 1<<20 || !fenceHex(receipt.AllowlistSHA256, 32) || !filepath.IsAbs(prepared.Spec.PullStagingDirectory) {
		return nil, nil, network.CreateOptions{}, errors.New("runtime pull staged identity changed")
	}
	labels := runtimeIdentityLabels(id)
	labels["luci.runtime-pull"] = "true"
	labels["luci.runtime-pull-manifest"] = bundle.ManifestSHA256
	config := &container.Config{
		Image:        bundle.Manifest.Bootstrap.ImageID,
		User:         "65532:65532",
		Env:          []string{"GOMEMLIMIT=32MiB", "GOMAXPROCS=1", "LUCI_PULL_ALLOWLIST_SHA256=" + receipt.AllowlistSHA256},
		ExposedPorts: nat.PortSet{pullHelperPort: struct{}{}},
		Labels:       labels,
	}
	pidLimit := int64(64)
	host := &container.HostConfig{
		ReadonlyRootfs:  true,
		CapDrop:         []string{"ALL"},
		SecurityOpt:     []string{"no-new-privileges:true"},
		Resources:       container.Resources{Memory: 256 << 20, MemorySwap: 256 << 20, NanoCPUs: 1_000_000_000, PidsLimit: &pidLimit},
		NetworkMode:     container.NetworkMode(networkName),
		PortBindings:    nat.PortMap{pullHelperPort: []nat.PortBinding{{HostIP: loopbackIPv4, HostPort: ""}}},
		PublishAllPorts: false,
		Mounts:          []mount.Mount{{Type: mount.TypeBind, Source: prepared.Spec.PullStagingDirectory, Target: "/registry-data", ReadOnly: true}},
		Tmpfs:           map[string]string{"/run/registry": "rw,noexec,nosuid,size=16777216,mode=0700,uid=65532,gid=65532"},
		AutoRemove:      false,
	}
	option, err := pullNetworkOptions(prepared)
	if err != nil {
		return nil, nil, network.CreateOptions{}, err
	}
	return config, host, option, nil
}

func pullNetworkOptions(prepared backend.PreparedRuntimeImage) (network.CreateOptions, error) {
	if prepared.PullBundle == nil {
		return network.CreateOptions{}, backend.ErrLaunchFenceOwnership
	}
	if _, _, err := pullHelperNames(prepared.Spec.Identity); err != nil {
		return network.CreateOptions{}, err
	}
	labels := runtimeIdentityLabels(prepared.Spec.Identity)
	labels["luci.runtime-pull"] = "true"
	labels["luci.runtime-pull-manifest"] = prepared.PullBundle.ManifestSHA256
	labels["luci.runtime-pull-port"] = "5000"
	return network.CreateOptions{Driver: "bridge", Scope: "local", Internal: false, Attachable: false, Ingress: false, Labels: labels}, nil
}
