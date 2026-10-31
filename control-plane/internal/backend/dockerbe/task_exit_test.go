package dockerbe

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type taskExitInspectFixture struct {
	APIClient
	row       container.InspectResponse
	requested string
	reads     int
}

func (f *taskExitInspectFixture) ContainerInspect(_ context.Context, ref string) (container.InspectResponse, error) {
	f.requested = ref
	f.reads++
	return f.row, nil
}

func TestTaskExitInspectorBindsOriginalContainerAndNeverPromotesGenericZero(t *testing.T) {
	for _, kind := range []string{"zero", "nonzero", "oom", "wrong-cid", "wrong-image", "wrong-created", "wrong-started", "restarted", "missing-capability", "wrong-contract", "running", "unknown-daemon-error"} {
		t.Run(kind, func(t *testing.T) {
			created := time.Date(2026, 10, 31, 1, 0, 0, 0, time.UTC)
			identity := backend.TaskExitIdentity{SessionID: "s-12345678", SandboxID: "s-12345678", Ref: strings.Repeat("a", 64), Image: "sha256:" + strings.Repeat("b", 64), ContractDigest: strings.Repeat("c", 64), CreatedAt: created, StartedAt: created.Add(time.Second)}
			labels := map[string]string{backend.LabelInstance: "instance", backend.LabelManaged: "true", backend.LabelSessionID: identity.SessionID, backend.LabelSandboxID: identity.SandboxID, backend.TaskExitContractLabel: identity.ContractDigest, backend.TaskExitImageLabel: identity.Image, backend.TaskExitProtocolLabel: backend.TaskExitProtocol, backend.TaskNormalZeroCapability: "v1"}
			fixture := &taskExitInspectFixture{row: container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{ID: identity.Ref, Image: identity.Image, Created: created.Format(time.RFC3339Nano), State: &container.State{Status: "exited", StartedAt: identity.StartedAt.Format(time.RFC3339Nano), FinishedAt: created.Add(time.Hour).Format(time.RFC3339Nano)}}, Config: &container.Config{Labels: labels}}}
			switch kind {
			case "nonzero":
				fixture.row.State.ExitCode = 1
			case "oom":
				fixture.row.State.OOMKilled = true
				fixture.row.State.ExitCode = 137
			case "wrong-cid":
				fixture.row.ID = strings.Repeat("d", 64)
			case "wrong-image":
				fixture.row.Image = "sha256:" + strings.Repeat("d", 64)
			case "wrong-created":
				fixture.row.Created = created.Add(-time.Second).Format(time.RFC3339Nano)
			case "wrong-started":
				fixture.row.State.StartedAt = created.Add(2 * time.Second).Format(time.RFC3339Nano)
			case "restarted":
				fixture.row.RestartCount = 1
			case "missing-capability":
				delete(labels, backend.TaskNormalZeroCapability)
			case "wrong-contract":
				labels[backend.TaskExitContractLabel] = strings.Repeat("d", 64)
			case "running":
				fixture.row.State.Status, fixture.row.State.Running, fixture.row.State.Pid = "running", true, 42
			case "unknown-daemon-error":
				fixture.row.State.Error = "unconfirmed runtime state"
			}
			b := &Backend{api: fixture, instance: "instance"}
			fact, err := b.ObserveTaskExit(context.Background(), identity)
			known := kind == "zero" || kind == "nonzero" || kind == "oom"
			if (err == nil) != known || fixture.reads != 1 || fixture.requested != identity.Ref || known && fact.NormalZero != (kind == "zero") || kind == "oom" && !fact.OOMKilled {
				t.Fatal("original terminal identity/state was not enforced", kind, fact, err)
			}
		})
	}
}

func TestShortStructuredTaskPreservesItsOriginalStoppedAllocation(t *testing.T) {
	created := time.Date(2026, 10, 31, 1, 0, 0, 0, time.UTC)
	ref, image, digest := strings.Repeat("a", 64), "sha256:"+strings.Repeat("b", 64), strings.Repeat("c", 64)
	spec := backend.Spec{ID: "s-12345678", SessionID: "s-12345678", Image: image, Command: "/usr/local/bin/node", Args: []string{backend.TaskResultWrapper, digest, "/usr/local/bin/codex", "exec"}}
	fixture := &taskExitInspectFixture{row: container.InspectResponse{ContainerJSONBase: &container.ContainerJSONBase{ID: ref, Image: image, Created: created.Format(time.RFC3339Nano), State: &container.State{Status: "exited", StartedAt: created.Add(time.Second).Format(time.RFC3339Nano), FinishedAt: created.Add(2 * time.Second).Format(time.RFC3339Nano)}}, Config: &container.Config{Labels: map[string]string{backend.LabelInstance: "instance", backend.LabelManaged: "true", backend.LabelSessionID: spec.SessionID, backend.LabelSandboxID: spec.ID, backend.TaskExitContractLabel: digest, backend.TaskExitImageLabel: image, backend.TaskExitProtocolLabel: backend.TaskExitProtocol, backend.TaskNormalZeroCapability: "v1"}}}}
	b := &Backend{api: fixture, instance: "instance"}
	row, ok := b.completedStructuredTask(context.Background(), spec, ref)
	if !ok || row.Ref != ref || row.State != backend.StateStopped || row.CreatedAt.IsZero() || row.StartedAt.IsZero() {
		t.Fatal("short original discarded before its stopped fact could be recorded", row)
	}
	spec.Command = "/bin/bash"
	if _, ok := b.completedStructuredTask(context.Background(), spec, ref); ok {
		t.Fatal("generic session gained short-task recovery")
	}
	spec.Command = "/usr/local/bin/node"
	delete(fixture.row.Config.Labels, backend.TaskNormalZeroCapability)
	if _, ok := b.completedStructuredTask(context.Background(), spec, ref); ok {
		t.Fatal("legacy image container zero became task evidence")
	}
	fixture.row.Config.Labels[backend.TaskNormalZeroCapability] = "v1"
	fixture.row.NetworkSettings = &container.NetworkSettings{NetworkSettingsBase: container.NetworkSettingsBase{Ports: nat.PortMap{attachPort: {{HostIP: "0.0.0.0", HostPort: "49200"}}}}}
	if _, ok := b.completedStructuredTask(context.Background(), spec, ref); ok {
		t.Fatal("short stopped task masked an unsafe published binding")
	}
	fixture.row.NetworkSettings = nil
	fixture.row.Config.Labels[backend.LabelLaunchFence] = ""
	if _, ok := b.completedStructuredTask(context.Background(), spec, ref); ok {
		t.Fatal("short stopped task masked an owned launch-fence marker")
	}
}
