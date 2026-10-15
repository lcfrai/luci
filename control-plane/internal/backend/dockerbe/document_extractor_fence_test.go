package dockerbe

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestDocumentFenceLostACKRequiresNewRealACKAndBlocksLateCreate(t *testing.T) {
	b, f, spec := newDocumentProbe(t)
	prepared, err := b.PrepareDocument(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	f.loseCreateACK = true
	if ref, err := b.CreateDocument(context.Background(), prepared); err == nil || ref != "" || len(f.rows) != 1 || len(f.starts)+len(f.removes) != 0 {
		t.Fatal("lost original Create was adopted or replayed", ref, err)
	}
	request := backend.DocumentExtractionFenceRequest{Identity: prepared.Spec.Identity, IntentID: strings.Repeat("e", 32)}
	f.loseCreateACK = true
	if proof, err := b.FenceDocumentLaunch(context.Background(), request); !errors.Is(err, backend.ErrLaunchFencePending) || proof.FenceRef != "" {
		t.Fatal("lost fence ACK became proof", proof, err)
	}
	var unacknowledged string
	for ref := range f.rows {
		unacknowledged = ref
	}
	// A daemon rollback after that ambiguous response cannot become absence
	// proof, and no inspection of the marker may manufacture its missing ACK.
	delete(f.rows, unacknowledged)
	proof, err := b.FenceDocumentLaunch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if proof.FenceRef == unacknowledged || len(f.starts) != 0 {
		t.Fatal("ambiguous marker was adopted or started")
	}
	if err = b.VerifyDocumentLaunchFence(context.Background(), request, proof); err != nil {
		t.Fatal(err)
	}
	if _, err = b.CreateDocument(context.Background(), prepared); err == nil {
		t.Fatal("late original Create displaced permanent marker")
	}
	if err = b.RetireDocument(context.Background(), prepared.Spec.Identity, proof.FenceRef); err == nil {
		t.Fatal("ordinary document cleanup removed permanent marker")
	}
}

func TestDocumentFenceStrictIdentityAndConfigurationRefuseForeignPurposes(t *testing.T) {
	b, f, spec := newDocumentProbe(t)
	prepared, err := b.PrepareDocument(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	request := backend.DocumentExtractionFenceRequest{Identity: prepared.Spec.Identity, IntentID: strings.Repeat("e", 32)}
	proof, err := b.FenceDocumentLaunch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	row := copyCheckRow(f.rows[proof.FenceRef])
	if !inertDocumentFenceLabels(row) || !validDocumentFenceContainer(request, row) {
		t.Fatal("exact inert document marker refused")
	}
	for _, kind := range []string{"actor", "original", "command", "agent", "check", "extra", "mount", "memory", "running"} {
		t.Run(kind, func(t *testing.T) {
			bad := copyCheckRow(row)
			switch kind {
			case "actor":
				bad.Config.Labels["luci.document-actor"] = strings.Repeat("f", 64)
			case "original":
				bad.Config.Labels["luci.document-original"] = strings.Repeat("f", 64)
			case "command":
				bad.Config.Labels["luci.document-command"] = strings.Repeat("f", 64)
			case "agent":
				bad.Config.Labels[backend.LabelSandboxID] = "foreign"
			case "check":
				bad.Config.Labels[checkLabel] = "1"
			case "extra":
				bad.Config.Labels["foreign"] = "value"
			case "mount":
				bad.HostConfig.Binds = []string{"/foreign:/foreign"}
			case "memory":
				bad.HostConfig.Memory++
			case "running":
				bad.State.Running = true
			}
			if validDocumentFenceContainer(request, bad) {
				t.Fatal("foreign identity/configuration accepted")
			}
		})
	}
	// A correctly shaped agent/check marker is still foreign to this exact
	// document request, even though the shared observer recognizes its purpose.
	agentConfig, agentHost := fenceConfiguration(backend.LaunchFenceRequest{InstanceID: prepared.Spec.Identity.InstanceID, SandboxID: "foreign", SessionID: "foreign", IntentID: strings.Repeat("a", 32)})
	foreign := copyCheckRow(row)
	foreign.Config, foreign.HostConfig, foreign.Name = agentConfig, agentHost, "/"+namePrefix+"foreign"
	if inertDocumentFenceLabels(foreign) || validDocumentFenceContainer(request, foreign) {
		t.Fatal("agent marker adopted as document")
	}
	checkConfig, checkHost := checkFenceConfiguration(backend.CheckFenceRequest{Identity: backend.CheckIdentity{Protocol: backend.CheckExecutionProtocol, AllocationID: "foreign-check", CheckID: "check-one", CommandIndex: 0, EngineID: prepared.Spec.Identity.EngineID, InstanceID: prepared.Spec.Identity.InstanceID, ScopeDigest: prepared.Spec.Identity.ScopeDigest, ProjectID: prepared.Spec.Identity.ProjectID, CommandDigest: strings.Repeat("b", 64)}, IntentID: strings.Repeat("c", 32)})
	foreign = copyCheckRow(row)
	foreign.Config, foreign.HostConfig = checkConfig, checkHost
	if inertDocumentFenceLabels(foreign) || validDocumentFenceContainer(request, foreign) {
		t.Fatal("check marker adopted as document")
	}
}
