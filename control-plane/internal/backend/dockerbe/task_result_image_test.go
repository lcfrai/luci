package dockerbe

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type terminalImageProbe struct {
	APIClient
	row   image.InspectResponse
	err   error
	calls int
}

func (p *terminalImageProbe) ImageInspect(context.Context, string, ...client.ImageInspectOption) (image.InspectResponse, error) {
	p.calls++
	return p.row, p.err
}

func TestCodexTaskResultImageRequiresExactAdvertisedCapabilityWithoutExecution(t *testing.T) {
	for _, kind := range []string{"valid", "missing-capability", "missing-durable-exit", "wrong-schema", "unknown-id", "wrong-platform", "inspect-error"} {
		t.Run(kind, func(t *testing.T) {
			p := &terminalImageProbe{row: image.InspectResponse{ID: "sha256:" + strings.Repeat("a", 64), Os: "linux", Architecture: "amd64", Config: &container.Config{Labels: map[string]string{backend.CodexTaskResultCapability: "v1", backend.CodexTaskResultSchemaLabel: backend.CodexTaskResultSchemaDigest, backend.TaskNormalZeroCapability: "v1"}}}}
			switch kind {
			case "missing-capability":
				delete(p.row.Config.Labels, backend.CodexTaskResultCapability)
			case "missing-durable-exit":
				delete(p.row.Config.Labels, backend.TaskNormalZeroCapability)
			case "wrong-schema":
				p.row.Config.Labels[backend.CodexTaskResultSchemaLabel] = strings.Repeat("b", 64)
			case "unknown-id":
				p.row.ID = "mutable:tag"
			case "wrong-platform":
				p.row.Os = "windows"
			case "inspect-error":
				p.err = errors.New("unavailable")
			}
			b := &Backend{api: p}
			id, err := b.ObserveCodexTaskResultImage(context.Background(), "luci/sandbox:dev")
			if p.calls != 1 || (err == nil) != (kind == "valid") || kind == "valid" && id != p.row.ID {
				t.Fatal("capability admission mismatch", kind, id, err, p.calls)
			}
		})
	}
	// No client mutation, build, pull, container or CLI method is implemented by
	// this probe. Any such accidental call would fail instead of faking success.
}
