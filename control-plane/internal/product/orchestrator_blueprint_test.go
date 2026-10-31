package product

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
)

func assistedBlueprintFixture() (OrchestratorContext, *OrchestratorBlueprintProposal) {
	source := deploymentplan.Source{ConfigurationRevision: 1, Context: &deploymentplan.ContextReference{BriefRevision: 1, SpecificationRevision: 1, DesignRevision: 1}}
	blueprint := deploymentplan.Blueprint{Version: 1, ProjectID: "p-blueprint", Source: source, Infrastructure: deploymentplan.Infrastructure{ID: "cluster", Kind: "kubernetes"}, Services: []deploymentplan.Service{{ID: "app-1", Kind: "app", Port: 8080, Replicas: 1, Resources: deploymentplan.ResourceBudget{RequestMilliCPU: 100, LimitMilliCPU: 200, RequestMemoryMiB: 64, LimitMemoryMiB: 128}}}}
	captured := &OrchestratorBlueprintContext{ProjectID: blueprint.ProjectID, Source: source, ContextDigest: strings.Repeat("a", 64), SourceReviewDigest: strings.Repeat("c", 64), Available: true}
	return OrchestratorContext{ProjectID: blueprint.ProjectID, DeploymentBlueprint: captured}, &OrchestratorBlueprintProposal{ContextDigest: captured.ContextDigest, SourceReviewDigest: captured.SourceReviewDigest, Blueprint: blueprint}
}
func TestAssistedBlueprintOptionalContextAndUnavailableTopology(t *testing.T) {
	var legacy struct {
		Blueprint *OrchestratorBlueprintContext `json:"deploymentBlueprint,omitempty"`
	}
	raw, _ := json.Marshal(legacy)
	if string(raw) != "{}" {
		t.Fatal("old optional encoding changed")
	}
	snapshot, proposal := assistedBlueprintFixture()
	if !validOrchestratorBlueprintContext(snapshot.DeploymentBlueprint, snapshot.ProjectID) || validateOrchestratorBlueprint(snapshot, proposal) != nil {
		t.Fatal("an honest missing-reference draft was rejected")
	}
	omitted := *snapshot.DeploymentBlueprint
	omitted.SavedRevision = 2
	omitted.SavedDigest = strings.Repeat("b", 64)
	omitted.Available = false
	omitted.UnavailableReason = "topology-size-limit"
	omitted.OmittedBytes = orchestratorBlueprintLimit + 1
	if !validOrchestratorBlueprintContext(&omitted, snapshot.ProjectID) {
		t.Fatal("explicit omission unavailable")
	}
	snapshot.DeploymentBlueprint = &omitted
	if validateOrchestratorBlueprint(snapshot, proposal) == nil {
		t.Fatal("amendment invented omitted topology")
	}
}
func TestAssistedBlueprintRejectsInventedExternalReferences(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*deploymentplan.Blueprint)
	}{
		{"image", func(b *deploymentplan.Blueprint) {
			b.Services[0].Image = "registry.example/app@sha256:" + strings.Repeat("a", 64)
		}},
		{"target", func(b *deploymentplan.Blueprint) { b.Infrastructure.Reference = "not-observed" }},
		{"namespace", func(b *deploymentplan.Blueprint) { b.Infrastructure.Namespace = "not-observed" }},
		{"model PVC", func(b *deploymentplan.Blueprint) {
			b.Services[0].Kind = "inference"
			b.Services[0].Model = &deploymentplan.ModelReference{Name: "model", ClaimName: "not-observed", MountPath: "/models"}
		}},
		{"secret key", func(b *deploymentplan.Blueprint) {
			b.Services[0].Environment = []deploymentplan.EnvironmentReference{{Name: "TOKEN", Kind: "secret", ResourceName: "not-observed", Key: "token"}}
		}},
		{"telemetry", func(b *deploymentplan.Blueprint) {
			b.ExternalResources = []deploymentplan.ExternalResource{{ID: "analytics", Kind: "analytics", Reference: "not-observed"}}
			b.Services[0].Telemetry = &deploymentplan.TelemetryBinding{ResourceID: "analytics", Endpoint: "https://receiver.example"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, proposal := assistedBlueprintFixture()
			tc.mutate(&proposal.Blueprint)
			if validateOrchestratorBlueprint(snapshot, proposal) == nil {
				t.Fatal("model invented an observed external reference")
			}
		})
	}
}
func TestAssistedBlueprintRequiresExactSavedRevisionAndSource(t *testing.T) {
	snapshot, proposal := assistedBlueprintFixture()
	saved := proposal.Blueprint
	raw, _ := json.Marshal(saved)
	snapshot.DeploymentBlueprint.Saved = &saved
	snapshot.DeploymentBlueprint.SavedRevision = 4
	snapshot.DeploymentBlueprint.SavedDigest = strings.Repeat("b", 64)
	proposal.ExpectedRevision = 4
	proposal.ExpectedDigest = snapshot.DeploymentBlueprint.SavedDigest
	if validateOrchestratorBlueprint(snapshot, proposal) != nil {
		t.Fatal("exact saved baseline rejected")
	}
	proposal.ExpectedRevision = 3
	if validateOrchestratorBlueprint(snapshot, proposal) == nil {
		t.Fatal("stale revision accepted")
	}
	proposal.ExpectedRevision = 4
	proposal.ContextDigest = strings.Repeat("c", 64)
	if validateOrchestratorBlueprint(snapshot, proposal) == nil {
		t.Fatal("stale deployment context accepted")
	}
	proposal.ContextDigest = snapshot.DeploymentBlueprint.ContextDigest
	proposal.Blueprint.Source.ConfigurationRevision++
	if validateOrchestratorBlueprint(snapshot, proposal) == nil {
		t.Fatal("source/configuration substituted")
	}
	after, _ := json.Marshal(saved)
	if string(after) != string(raw) {
		t.Fatal("validation changed saved topology")
	}
}

func TestAssistedBlueprintNeverEntersGenericApplyOrCoordinationRecovery(t *testing.T) {
	snapshot, proposal := assistedBlueprintFixture()
	turn := storedOrchestratorTurn{OrchestratorTurn: OrchestratorTurn{Context: snapshot, Answer: &OrchestratorAnswer{Proposal: &OrchestratorProposal{Target: "deployment-blueprint", DeploymentBlueprint: proposal}}}}
	if !manualBlueprintTurn(turn) || !validManualBlueprintTurn(turn) || coordinationChanges(&turn) != nil {
		t.Fatal("manual topology entered automatic coordination")
	}
	turn.ApplyIntent = "applying"
	if validManualBlueprintTurn(turn) {
		t.Fatal("private generic apply marker accepted")
	}
	turn.ApplyIntent = ""
	turn.AppliedRevision = 1
	if validManualBlueprintTurn(turn) {
		t.Fatal("fabricated applied revision accepted")
	}
	turn.AppliedRevision = 0
	turn.ApplyTargetRevision = 1
	if validManualBlueprintTurn(turn) {
		t.Fatal("generic target recovery accepted")
	}
}

func TestAssistedBlueprintCannotDropSavedExternalResources(t *testing.T) {
	snapshot, proposal := assistedBlueprintFixture()
	saved := proposal.Blueprint
	saved.ExternalResources = []deploymentplan.ExternalResource{{ID: "database", Kind: "database", Reference: "existing-data"}}
	snapshot.DeploymentBlueprint.Saved = &saved
	snapshot.DeploymentBlueprint.SavedRevision = 1
	snapshot.DeploymentBlueprint.SavedDigest = strings.Repeat("b", 64)
	proposal.ExpectedRevision = 1
	proposal.ExpectedDigest = snapshot.DeploymentBlueprint.SavedDigest
	if validateOrchestratorBlueprint(snapshot, proposal) == nil {
		t.Fatal("editable subset dropped a saved external resource")
	}
	proposal.Blueprint.ExternalResources = append([]deploymentplan.ExternalResource{}, saved.ExternalResources...)
	if validateOrchestratorBlueprint(snapshot, proposal) != nil {
		t.Fatal("preserved external resource rejected")
	}
}
func TestAssistedBlueprintUnreadableSavedLedgerDoesNotBlockOrdinaryConversation(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "Topology preservation")
	path, _, err := s.readDeploymentBlueprintLedger(p)
	if err != nil {
		t.Fatal(err)
	}
	original := []byte("{\"version\":99,\"unknownSavedShape\":true}\n")
	if err = os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.OrchestratorSnapshot(orchestratorActor(t, "topology-owner"), p.ID)
	if err != nil || snapshot.DeploymentBlueprint == nil || snapshot.DeploymentBlueprint.Available || snapshot.DeploymentBlueprint.UnavailableReason != "saved-topology-unconfirmed" || snapshot.DeploymentBlueprint.Saved != nil {
		t.Fatal("optional saved topology prevented ordinary conversation or became an empty blueprint", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(original) {
		t.Fatal("unsupported private ledger normalized or changed", err)
	}
}
