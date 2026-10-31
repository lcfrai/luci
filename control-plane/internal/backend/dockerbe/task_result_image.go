package dockerbe

import (
	"context"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func (b *Backend) ObserveCodexTaskResultImage(ctx context.Context, reference string) (string, error) {
	return b.observeHarnessResultImage(ctx, reference, "codex")
}
func (b *Backend) ObserveHarnessTaskResultImage(ctx context.Context, reference, harness string) (string, error) {
	if harness != "claude-code" && harness != "opencode" {
		return "", backend.ErrUnsupported
	}
	return b.observeHarnessResultImage(ctx, reference, harness)
}
func (b *Backend) observeHarnessResultImage(ctx context.Context, reference, harness string) (string, error) {
	images, ok := b.api.(imageResourceClient)
	if !ok || reference == "" || len(reference) > 4096 {
		return "", backend.ErrUnsupported
	}
	row, err := images.ImageInspect(ctx, reference)
	if err != nil || ctx.Err() != nil {
		return "", backend.ErrUnavailable
	}
	if !strings.HasPrefix(row.ID, "sha256:") || !fenceHex(strings.TrimPrefix(row.ID, "sha256:"), 32) || row.Os != "linux" || row.Architecture != "amd64" || row.Config == nil || row.Config.Labels[backend.CodexTaskResultCapability] != "v1" || row.Config.Labels[backend.CodexTaskResultSchemaLabel] != backend.CodexTaskResultSchemaDigest || row.Config.Labels[backend.TaskNormalZeroCapability] != "v1" {
		return "", backend.ErrUnsupported
	}
	if harness != "codex" && row.Config.Labels[backend.HarnessTaskResultCapability] != backend.HarnessTaskResultVersion {
		return "", backend.ErrUnsupported
	}
	return row.ID, nil
}

var _ backend.CodexTaskResultImageObserver = (*Backend)(nil)

var _ backend.HarnessTaskResultImageObserver = (*Backend)(nil)
