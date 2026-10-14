package dockerbe

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

const pullProgressLimit int64 = 32 << 20

var _ backend.RuntimeImagePullRecoveryExecutor = (*Backend)(nil)

func (b *Backend) RetireKnownPullOwnership(ctx context.Context, id backend.RuntimeImageIdentity, manifestSHA, bootstrapID, stage string, helper backend.RuntimePullHelper) error {
	if !validRuntimeIdentity(id, true) || !fenceHex(manifestSHA, 32) || !runtimeImageID(bootstrapID) || !filepath.IsAbs(stage) || helper.NetworkID == "" || helper.HostPort != "" {
		return backend.ErrLaunchFenceOwnership
	}
	bundle := &runtimeoci.PullBundle{ManifestSHA256: manifestSHA}
	bundle.Manifest.Bootstrap.ImageID = bootstrapID
	prepared := backend.PreparedRuntimeImage{Spec: backend.RuntimeImageSpec{Identity: id, PullStagingDirectory: stage}, PullBundle: bundle}
	if helper.ContainerID != "" {
		return b.RetirePullHelper(ctx, prepared, helper)
	}
	if err := b.runtimeImageIdentity(ctx, id); err != nil {
		return err
	}
	client, ok := b.api.(runtimePullNetworkClient)
	if !ok {
		return backend.ErrUnsupported
	}
	if err := inspectOwnedPullNetwork(ctx, client, id, helper.NetworkID); err != nil {
		return err
	}
	if err := client.NetworkRemove(ctx, helper.NetworkID); err != nil {
		return backend.ErrUnavailable
	}
	name, _, _ := pullHelperNames(id)
	if _, err := client.NetworkInspect(ctx, helper.NetworkID, network.InspectOptions{}); !errdefs.IsNotFound(err) {
		return backend.ErrUnavailable
	}
	if _, err := client.NetworkInspect(ctx, name, network.InspectOptions{}); !errdefs.IsNotFound(err) {
		return backend.ErrUnavailable
	}
	return b.runtimeImageIdentity(ctx, id)
}

func (b *Backend) DerivePullRuntimeImage(ctx context.Context, prepared backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error) {
	if prepared.PullBundle == nil || prepared.Bundle != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, backend.ErrUnsupported
	}
	return b.DerivePackagedRuntimeImage(ctx, prepared)
}

func (b *Backend) inspectPulledOriginal(ctx context.Context, prepared backend.PreparedRuntimeImage) error {
	if prepared.PullBundle == nil || b.runtimeImageIdentity(ctx, prepared.Spec.Identity) != nil {
		return backend.ErrLaunchFenceOwnership
	}
	client, ok := b.api.(runtimeImageLoadClient)
	if !ok {
		return backend.ErrUnsupported
	}
	m := prepared.PullBundle.Manifest
	row, err := client.ImageInspect(ctx, m.Image.ID)
	if err != nil {
		return err
	}
	if row.ID != m.Image.ID || !runtimeBaseValid(row) || row.Architecture != "amd64" || row.Config.User != m.Runtime.User || row.Config.WorkingDir != m.Runtime.WorkingDir || !equalPullStrings(row.Config.Entrypoint, m.Runtime.Entrypoint) || len(row.RootFS.Layers) != len(m.Image.Layers) {
		return backend.ErrLaunchFenceOwnership
	}
	for i, layer := range m.Image.Layers {
		if row.RootFS.Layers[i] != layer.DiffID {
			return backend.ErrLaunchFenceOwnership
		}
	}
	for label := range row.Config.Labels {
		if strings.HasPrefix(label, "luci.runtime-image") || label == backend.LabelInstance {
			return backend.ErrLaunchFenceOwnership
		}
	}
	return b.runtimeImageIdentity(ctx, prepared.Spec.Identity)
}

// PullPackagedRuntimeImage starts only the one digest-addressed pull from the
// verified helper. Lost stream/close/readback is unknown and never retried.
func (b *Backend) PullPackagedRuntimeImage(ctx context.Context, prepared backend.PreparedRuntimeImage, helper backend.RuntimePullHelper) (backend.RuntimeImageBuildResult, error) {
	if prepared.PullBundle == nil || helper.NetworkID == "" || helper.ContainerID == "" || helper.HostPort == "" {
		return backend.RuntimeImageBuildResult{Settled: true}, backend.ErrLaunchFenceOwnership
	}
	if err := prepared.PullBundle.VerifyPullPayload(); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	if _, err := prepared.PullBundle.VerifyPullStage(prepared.Spec.PullStagingDirectory); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	if err := b.ProbePullHelper(ctx, prepared, helper); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	ref, err := pullImageReference(helper.HostPort, prepared.PullBundle.Manifest.Image.ManifestDigest)
	if err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	images, ok := b.api.(runtimeImageClient)
	if !ok {
		return backend.RuntimeImageBuildResult{Settled: true}, backend.ErrUnsupported
	}
	response, err := images.ImagePull(ctx, ref, image.PullOptions{Platform: "linux/amd64"})
	if err != nil || response == nil {
		if response != nil {
			_ = response.Close()
		}
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	limited := &io.LimitedReader{R: response, N: pullProgressLimit + 1}
	decoder := json.NewDecoder(limited)
	complete, failed := false, false
	for {
		var msg runtimeStreamMessage
		e := decoder.Decode(&msg)
		if e == io.EOF {
			complete = true
			break
		}
		if e != nil {
			break
		}
		if runtimeMessageError(msg) != "" {
			failed = true
		}
	}
	closeErr := response.Close()
	if !complete || limited.N == 0 || closeErr != nil || ctx.Err() != nil || failed {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	probe, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := b.inspectPulledOriginal(probe, prepared); err != nil {
		return backend.RuntimeImageBuildResult{}, err
	}
	return backend.RuntimeImageBuildResult{ImageID: prepared.PullBundle.Manifest.Image.ID, Settled: true}, nil
}

// RetirePullHelper never removes by a stale tag or a missing-container guess.
// The coordinator must journal the retirement intent and retain the lease
// until both exact owned objects have positive stopped/absence readback.
func (b *Backend) RetirePullHelper(ctx context.Context, prepared backend.PreparedRuntimeImage, helper backend.RuntimePullHelper) error {
	if prepared.PullBundle == nil || b.runtimeImageIdentity(ctx, prepared.Spec.Identity) != nil {
		return backend.ErrLaunchFenceOwnership
	}
	client, ok := b.api.(runtimePullNetworkClient)
	if !ok {
		return backend.ErrUnsupported
	}
	stopper, ok := b.api.(runtimePullStopClient)
	if !ok {
		return backend.ErrUnsupported
	}
	if err := inspectOwnedPullNetwork(ctx, client, prepared.Spec.Identity, helper.NetworkID); err != nil {
		return err
	}
	row, err := inspectOwnedPullContainer(ctx, b, prepared, helper.ContainerID)
	if err != nil {
		return err
	}
	if row.State == nil {
		return backend.ErrLaunchFenceOwnership
	}
	if row.State.Running {
		seconds := 5
		if err := stopper.ContainerStop(ctx, helper.ContainerID, container.StopOptions{Timeout: &seconds}); err != nil {
			return backend.ErrUnavailable
		}
		row, err = inspectOwnedPullContainer(ctx, b, prepared, helper.ContainerID)
		if err != nil || row.State == nil || row.State.Running {
			return backend.ErrUnavailable
		}
	}
	if err := b.api.ContainerRemove(ctx, helper.ContainerID, container.RemoveOptions{Force: false, RemoveVolumes: false}); err != nil {
		return backend.ErrUnavailable
	}
	_, name, _ := pullHelperNames(prepared.Spec.Identity)
	if _, err := b.api.ContainerInspect(ctx, helper.ContainerID); !errdefs.IsNotFound(err) {
		return backend.ErrUnavailable
	}
	if _, err := b.api.ContainerInspect(ctx, name); !errdefs.IsNotFound(err) {
		return backend.ErrUnavailable
	}
	if err := client.NetworkRemove(ctx, helper.NetworkID); err != nil {
		return backend.ErrUnavailable
	}
	networkName, _, _ := pullHelperNames(prepared.Spec.Identity)
	if _, err := client.NetworkInspect(ctx, helper.NetworkID, network.InspectOptions{}); !errdefs.IsNotFound(err) {
		return backend.ErrUnavailable
	}
	if _, err := client.NetworkInspect(ctx, networkName, network.InspectOptions{}); !errdefs.IsNotFound(err) {
		return backend.ErrUnavailable
	}
	return b.runtimeImageIdentity(ctx, prepared.Spec.Identity)
}
