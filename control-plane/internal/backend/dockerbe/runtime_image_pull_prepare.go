package dockerbe

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

const (
	runtimePullHelperMiB = uint64(256)
	runtimePullDaemonMiB = uint64(512)
)

var _ backend.RuntimeImagePullExecutor = (*Backend)(nil)

// pullAdmissionMiB charges every verified uncompressed final layer and every
// compressed blob Moby may keep in temporary storage at once. Serial HTTP
// delivery is not treated as proof of serial daemon extraction. The small
// bootstrap ImageLoad has its own separate conservative admission.
func pullAdmissionMiB(m runtimeoci.PullManifest, workerMiB int) (main, bootstrap int, err error) {
	if workerMiB < 128 || workerMiB > 32768 || len(m.Image.Layers) < 1 || len(m.Image.Layers) > 2048 || len(m.Bootstrap.Layers) < 1 || len(m.Bootstrap.Layers) > 8 || m.Bootstrap.Bytes < 1024 || m.Bootstrap.Bytes > runtimeoci.MaxBootstrapBytes {
		return 0, 0, backend.ErrUnsupported
	}
	var compressed, expanded, bootExpanded uint64
	for _, layer := range m.Image.Layers {
		if layer.CompressedBytes < 1 || layer.UncompressedBytes < 1 || uint64(layer.CompressedBytes) > uint64(runtimeoci.MaxPullStoreBytes)-compressed || uint64(layer.UncompressedBytes) > uint64(runtimeoci.MaxPullImageBytes)-expanded {
			return 0, 0, backend.ErrUnsupported
		}
		compressed += uint64(layer.CompressedBytes)
		expanded += uint64(layer.UncompressedBytes)
	}
	for _, layer := range m.Bootstrap.Layers {
		if layer.UncompressedBytes < 1 || uint64(layer.UncompressedBytes) > uint64(runtimeoci.MaxBootstrapExpandedBytes)-bootExpanded {
			return 0, 0, backend.ErrUnsupported
		}
		bootExpanded += uint64(layer.UncompressedBytes)
	}
	const mib = uint64(1 << 20)
	mainBytes := compressed + expanded + (runtimePullHelperMiB+runtimePullDaemonMiB)*mib
	bootstrapBytes := uint64(m.Bootstrap.Bytes) + bootExpanded + runtimePullDaemonMiB*mib
	main = max(workerMiB, int((mainBytes+mib-1)/mib))
	bootstrap = max(workerMiB, int((bootstrapBytes+mib-1)/mib))
	if main > math.MaxInt32 || bootstrap > math.MaxInt32 {
		return 0, 0, backend.ErrUnsupported
	}
	return main, bootstrap, nil
}

// PreparePullRuntimeImage is read-only. It holds verified descriptors and
// selected-engine identity; no Docker image, network or container is created.
// The coordinator must reserve and journal the exact work before any effect.
func (b *Backend) PreparePullRuntimeImage(ctx context.Context, spec backend.RuntimeImageSpec) (backend.PreparedRuntimeImage, error) {
	if !validRuntimeIdentity(spec.Identity, false) || spec.Resources.MemoryMiB <= 0 || int64(spec.Resources.MemoryMiB) > math.MaxInt64>>20 || spec.Resources.CPUMillis <= 0 || !filepath.IsAbs(spec.PullStagingDirectory) {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	// The bridge's published 127.0.0.1 port must belong to this machine. A
	// remote Docker client cannot be made safe by trusting its engine ID alone.
	host, ok := b.api.(interface{ DaemonHost() string })
	if !ok || (!strings.HasPrefix(host.DaemonHost(), "npipe://") && !strings.HasPrefix(host.DaemonHost(), "unix://")) {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	tag, err := runtimeImageTag(spec.ImageTag)
	if err != nil {
		return backend.PreparedRuntimeImage{}, err
	}
	spec.ImageTag = tag
	bundle, err := runtimeoci.OpenPull(spec.SourceDirectory)
	if err != nil {
		return backend.PreparedRuntimeImage{}, fmt.Errorf("installed compressed runtime distribution is unavailable: %w", err)
	}
	closeBundle := true
	defer func() {
		if closeBundle {
			_ = bundle.Close()
		}
	}()
	probe, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	fence, err := b.LaunchFenceIdentity(probe)
	if err != nil {
		return backend.PreparedRuntimeImage{}, err
	}
	if spec.Identity.EngineID != "" && spec.Identity.EngineID != fence.EngineID || spec.Identity.InstanceID != "" && spec.Identity.InstanceID != fence.InstanceID {
		return backend.PreparedRuntimeImage{}, backend.ErrLaunchFenceOwnership
	}
	spec.Identity.EngineID, spec.Identity.InstanceID, spec.Identity.SourceDigest = fence.EngineID, fence.InstanceID, bundle.Manifest.CanonicalInputDigest
	limits, ok := b.api.(resourceLimitClient)
	if !ok {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	info, err := limits.Info(probe)
	if err != nil || probe.Err() != nil {
		return backend.PreparedRuntimeImage{}, backend.ErrUnavailable
	}
	if info.ID != fence.EngineID || info.OSType != "linux" {
		return backend.PreparedRuntimeImage{}, backend.ErrLaunchFenceOwnership
	}
	if !requestedLimitsSupported(info, spec.Resources) || info.Driver != "vfs" && info.Driver != "overlay2" && info.Driver != "overlayfs" {
		return backend.PreparedRuntimeImage{}, backend.ErrResourceLimitsUnsupported
	}
	if _, ok := b.api.(runtimeImageLoadClient); !ok {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	if _, ok := b.api.(runtimeImageClient); !ok {
		return backend.PreparedRuntimeImage{}, backend.ErrUnsupported
	}
	mainMiB, bootstrapMiB, err := pullAdmissionMiB(bundle.Manifest, spec.Resources.MemoryMiB)
	if err != nil {
		return backend.PreparedRuntimeImage{}, err
	}
	closeBundle = false
	return backend.PreparedRuntimeImage{Spec: spec, ContextDigest: bundle.Manifest.CanonicalInputDigest, RequiredDiskBytes: backend.RuntimeImageRequiredDiskBytes, AdmissionMemoryMiB: mainMiB, BootstrapAdmissionMemoryMiB: bootstrapMiB, StorageDriver: info.Driver, PullBundle: bundle}, nil
}
