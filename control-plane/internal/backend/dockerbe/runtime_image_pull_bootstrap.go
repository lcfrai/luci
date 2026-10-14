package dockerbe

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/errdefs"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func (b *Backend) inspectPullBootstrap(ctx context.Context, prepared backend.PreparedRuntimeImage) error {
	if prepared.PullBundle == nil || b.runtimeImageIdentity(ctx, prepared.Spec.Identity) != nil {
		return backend.ErrLaunchFenceOwnership
	}
	images, ok := b.api.(runtimeImageLoadClient)
	if !ok {
		return backend.ErrUnsupported
	}
	m := prepared.PullBundle.Manifest.Bootstrap
	row, err := images.ImageInspect(ctx, m.ImageID)
	if err != nil {
		return err
	}
	if row.ID != m.ImageID || row.Os != "linux" || row.Architecture != "amd64" || row.Config == nil || row.Config.User != m.User || row.Config.WorkingDir != m.WorkingDir || !equalPullStrings(row.Config.Entrypoint, m.Entrypoint) || len(row.RootFS.Layers) != len(m.Layers) {
		return backend.ErrLaunchFenceOwnership
	}
	for i, layer := range m.Layers {
		if row.RootFS.Layers[i] != layer.DiffID {
			return backend.ErrLaunchFenceOwnership
		}
	}
	return b.runtimeImageIdentity(ctx, prepared.Spec.Identity)
}

func equalPullStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// LoadPullBootstrap is its own journaled daemon effect. A lost ImageLoad
// transport is not settled by image absence or by a later successful inspect.
func (b *Backend) LoadPullBootstrap(ctx context.Context, prepared backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error) {
	bundle := prepared.PullBundle
	if bundle == nil || prepared.ContextDigest != bundle.Manifest.CanonicalInputDigest || prepared.Spec.Identity.SourceDigest != prepared.ContextDigest {
		return backend.RuntimeImageBuildResult{Settled: true}, backend.ErrLaunchFenceOwnership
	}
	if err := b.runtimeImageIdentity(ctx, prepared.Spec.Identity); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	if err := bundle.VerifyPullPayload(); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	if err := b.inspectPullBootstrap(ctx, prepared); err == nil {
		return backend.RuntimeImageBuildResult{ImageID: bundle.Manifest.Bootstrap.ImageID, Settled: true}, nil
	} else if !errdefs.IsNotFound(err) {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	images, ok := b.api.(runtimeImageLoadClient)
	if !ok {
		return backend.RuntimeImageBuildResult{Settled: true}, backend.ErrUnsupported
	}
	if _, err := bundle.Bootstrap.Seek(0, io.SeekStart); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	response, err := images.ImageLoad(ctx, io.LimitReader(bundle.Bootstrap, bundle.Manifest.Bootstrap.Bytes))
	if err != nil || response.Body == nil {
		if response.Body != nil {
			_ = response.Body.Close()
		}
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	reader := &io.LimitedReader{R: response.Body, N: runtimeImageStreamLimit + 1}
	decoder := json.NewDecoder(reader)
	complete, failure := false, false
	for {
		var msg runtimeStreamMessage
		decodeErr := decoder.Decode(&msg)
		if decodeErr == io.EOF {
			complete = true
			break
		}
		if decodeErr != nil {
			break
		}
		if runtimeMessageError(msg) != "" {
			failure = true
		}
	}
	closeErr := response.Body.Close()
	if !complete || reader.N == 0 || closeErr != nil || ctx.Err() != nil {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	if failure {
		return backend.RuntimeImageBuildResult{Settled: true}, backend.ErrUnavailable
	}
	probe, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := b.inspectPullBootstrap(probe, prepared); err != nil {
		return backend.RuntimeImageBuildResult{}, err
	}
	return backend.RuntimeImageBuildResult{ImageID: strings.ToLower(bundle.Manifest.Bootstrap.ImageID), Settled: true}, nil
}
