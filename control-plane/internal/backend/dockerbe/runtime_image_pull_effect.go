package dockerbe

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

type runtimePullNetworkClient interface {
	NetworkCreate(context.Context, string, network.CreateOptions) (network.CreateResponse, error)
	NetworkInspect(context.Context, string, network.InspectOptions) (network.Inspect, error)
	NetworkRemove(context.Context, string) error
}

type runtimePullDaemonProbe interface {
	DistributionInspect(context.Context, string, string) (registry.DistributionInspect, error)
}

type runtimePullStopClient interface {
	ContainerStop(context.Context, string, container.StopOptions) error
}

func inspectOwnedPullNetwork(ctx context.Context, client runtimePullNetworkClient, id backend.RuntimeImageIdentity, networkID string) error {
	name, _, err := pullHelperNames(id)
	if err != nil || !fenceHex(networkID, 32) {
		return backend.ErrLaunchFenceOwnership
	}
	row, err := client.NetworkInspect(ctx, networkID, network.InspectOptions{})
	if err != nil {
		return err
	}
	if row.ID != networkID || row.Name != name || row.Driver != "bridge" || row.Scope != "local" || row.Internal || row.Attachable || row.Ingress || row.Labels["luci.runtime-pull"] != "true" || row.Labels[runtimeImagePrefix+"allocation"] != id.AllocationID || row.Labels[backend.LabelInstance] != id.InstanceID {
		return backend.ErrLaunchFenceOwnership
	}
	return nil
}

func (b *Backend) CreatePullNetwork(ctx context.Context, prepared backend.PreparedRuntimeImage) (string, error) {
	if prepared.PullBundle == nil || b.runtimeImageIdentity(ctx, prepared.Spec.Identity) != nil {
		return "", backend.ErrLaunchFenceOwnership
	}
	client, ok := b.api.(runtimePullNetworkClient)
	if !ok {
		return "", backend.ErrUnsupported
	}
	id := prepared.Spec.Identity
	name, _, err := pullHelperNames(id)
	if err != nil {
		return "", err
	}
	if _, err := client.NetworkInspect(ctx, name, network.InspectOptions{}); err == nil {
		return "", backend.ErrLaunchFenceOwnership
	} else if !errdefs.IsNotFound(err) {
		return "", backend.ErrUnavailable
	}
	options, err := pullNetworkOptions(prepared)
	if err != nil {
		return "", err
	}
	created, err := client.NetworkCreate(ctx, name, options)
	if err != nil || created.ID == "" || ctx.Err() != nil {
		return "", backend.ErrUnavailable
	}
	if err := inspectOwnedPullNetwork(ctx, client, id, created.ID); err != nil {
		return "", err
	}
	return created.ID, nil
}

func inspectOwnedPullContainer(ctx context.Context, b *Backend, prepared backend.PreparedRuntimeImage, containerID string) (container.InspectResponse, error) {
	id := prepared.Spec.Identity
	_, name, err := pullHelperNames(id)
	if err != nil || !fenceHex(containerID, 32) {
		return container.InspectResponse{}, backend.ErrLaunchFenceOwnership
	}
	row, err := b.api.ContainerInspect(ctx, containerID)
	if err != nil {
		return row, err
	}
	if row.ID != containerID || row.Name != "/"+name || row.Image != prepared.PullBundle.Manifest.Bootstrap.ImageID || row.Config == nil || row.HostConfig == nil || row.Config.Labels["luci.runtime-pull"] != "true" || row.Config.Labels["luci.runtime-pull-manifest"] != prepared.PullBundle.ManifestSHA256 || row.Config.Labels[runtimeImagePrefix+"allocation"] != id.AllocationID || row.Config.Labels[backend.LabelInstance] != id.InstanceID || row.HostConfig.Memory != 256<<20 || row.HostConfig.MemorySwap != 256<<20 || row.HostConfig.NanoCPUs != 1_000_000_000 || !row.HostConfig.ReadonlyRootfs || len(row.HostConfig.Mounts) != 1 || !row.HostConfig.Mounts[0].ReadOnly || row.HostConfig.Mounts[0].Source != prepared.Spec.PullStagingDirectory {
		return container.InspectResponse{}, backend.ErrLaunchFenceOwnership
	}
	return row, nil
}

func (b *Backend) CreatePullHelper(ctx context.Context, prepared backend.PreparedRuntimeImage, receipt runtimeoci.PullStageReceipt, networkID string) (string, error) {
	if prepared.PullBundle == nil || b.runtimeImageIdentity(ctx, prepared.Spec.Identity) != nil {
		return "", backend.ErrLaunchFenceOwnership
	}
	client, ok := b.api.(runtimePullNetworkClient)
	if !ok {
		return "", backend.ErrUnsupported
	}
	if err := inspectOwnedPullNetwork(ctx, client, prepared.Spec.Identity, networkID); err != nil {
		return "", err
	}
	readback, err := prepared.PullBundle.VerifyPullStage(prepared.Spec.PullStagingDirectory)
	if err != nil || readback != receipt {
		return "", backend.ErrLaunchFenceOwnership
	}
	if err := b.inspectPullBootstrap(ctx, prepared); err != nil {
		return "", err
	}
	config, host, _, err := pullHelperConfiguration(prepared, receipt)
	if err != nil {
		return "", err
	}
	_, name, _ := pullHelperNames(prepared.Spec.Identity)
	if _, err := b.api.ContainerInspect(ctx, name); err == nil {
		return "", backend.ErrLaunchFenceOwnership
	} else if !errdefs.IsNotFound(err) {
		return "", backend.ErrUnavailable
	}
	created, err := b.api.ContainerCreate(ctx, config, host, nil, nil, name)
	if err != nil || created.ID == "" || ctx.Err() != nil {
		return "", backend.ErrUnavailable
	}
	row, err := inspectOwnedPullContainer(ctx, b, prepared, created.ID)
	if err != nil || row.State == nil || row.State.Status != "created" {
		return "", backend.ErrLaunchFenceOwnership
	}
	return created.ID, nil
}

func (b *Backend) StartPullHelper(ctx context.Context, prepared backend.PreparedRuntimeImage, helper backend.RuntimePullHelper) (string, error) {
	if prepared.PullBundle == nil || helper.HostPort != "" || b.runtimeImageIdentity(ctx, prepared.Spec.Identity) != nil {
		return "", backend.ErrLaunchFenceOwnership
	}
	client, ok := b.api.(runtimePullNetworkClient)
	if !ok {
		return "", backend.ErrUnsupported
	}
	if err := inspectOwnedPullNetwork(ctx, client, prepared.Spec.Identity, helper.NetworkID); err != nil {
		return "", err
	}
	row, err := inspectOwnedPullContainer(ctx, b, prepared, helper.ContainerID)
	if err != nil || row.State == nil || row.State.Status != "created" {
		return "", backend.ErrLaunchFenceOwnership
	}
	if err := b.api.ContainerStart(ctx, helper.ContainerID, container.StartOptions{}); err != nil || ctx.Err() != nil {
		return "", backend.ErrUnavailable
	}
	row, err = inspectOwnedPullContainer(ctx, b, prepared, helper.ContainerID)
	if err != nil || row.State == nil || !row.State.Running || row.NetworkSettings == nil {
		return "", backend.ErrLaunchFenceOwnership
	}
	ports := row.NetworkSettings.Ports[pullHelperPort]
	if len(ports) != 1 || ports[0].HostIP != loopbackIPv4 {
		return "", backend.ErrNonLoopbackPublish
	}
	port, err := strconv.Atoi(ports[0].HostPort)
	if err != nil || port < 1024 || port > 65535 {
		return "", backend.ErrLaunchFenceOwnership
	}
	return ports[0].HostPort, nil
}

func pullImageReference(port string, manifest string) (string, error) {
	n, err := strconv.Atoi(port)
	if err != nil || n < 1024 || n > 65535 || !runtimeImageID(manifest) {
		return "", backend.ErrUnsupported
	}
	return "127.0.0.1:" + port + "/luci/runtime-installed@" + manifest, nil
}

// ProbePullHelper performs two independent read-only checks: the host HTTP
// path and the selected daemon's DistributionInspect to the exact digest.
func (b *Backend) ProbePullHelper(ctx context.Context, prepared backend.PreparedRuntimeImage, helper backend.RuntimePullHelper) error {
	if prepared.PullBundle == nil || b.runtimeImageIdentity(ctx, prepared.Spec.Identity) != nil {
		return backend.ErrLaunchFenceOwnership
	}
	client, ok := b.api.(runtimePullDaemonProbe)
	if !ok {
		return backend.ErrUnsupported
	}
	ref, err := pullImageReference(helper.HostPort, prepared.PullBundle.Manifest.Image.ManifestDigest)
	if err != nil {
		return err
	}
	row, err := inspectOwnedPullContainer(ctx, b, prepared, helper.ContainerID)
	if err != nil || row.State == nil || !row.State.Running {
		return backend.ErrLaunchFenceOwnership
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	httpClient := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, "http://"+strings.SplitN(ref, "/", 2)[0]+"/v2/luci/runtime-installed/manifests/"+prepared.PullBundle.Manifest.Image.ManifestDigest, nil)
	if err != nil {
		return err
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return backend.ErrUnavailable
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Docker-Content-Digest") != prepared.PullBundle.Manifest.Image.ManifestDigest {
		return backend.ErrUnavailable
	}
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	remote, err := client.DistributionInspect(probe, ref, "")
	if err != nil || remote.Descriptor.Digest.String() != prepared.PullBundle.Manifest.Image.ManifestDigest {
		return backend.ErrUnavailable
	}
	return b.runtimeImageIdentity(ctx, prepared.Spec.Identity)
}
