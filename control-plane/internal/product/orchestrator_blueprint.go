package product

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const orchestratorBlueprintLimit = 48 << 10

// This optional snapshot is saved planning context, never deployment authority.
// Absence preserves historical context encoding. Omitted topology cannot be
// amended by a model: a person must simplify or review it in Deploy first.
type OrchestratorBlueprintContext struct {
	ProjectID          string                    `json:"projectId"`
	SavedRevision      int                       `json:"savedRevision"`
	SavedDigest        string                    `json:"savedDigest"`
	Source             deploymentplan.Source     `json:"source"`
	ContextDigest      string                    `json:"contextDigest"`
	SourceReviewDigest string                    `json:"sourceReviewDigest"`
	Available          bool                      `json:"available"`
	UnavailableReason  string                    `json:"unavailableReason"`
	OmittedBytes       int                       `json:"omittedBytes"`
	Saved              *deploymentplan.Blueprint `json:"saved,omitempty"`
}

type OrchestratorBlueprintProposal struct {
	ExpectedRevision   int                      `json:"expectedRevision"`
	ExpectedDigest     string                   `json:"expectedDigest"`
	ContextDigest      string                   `json:"contextDigest"`
	SourceReviewDigest string                   `json:"sourceReviewDigest"`
	Blueprint          deploymentplan.Blueprint `json:"blueprint"`
}

func manualBlueprintProposal(proposal *OrchestratorProposal) bool {
	return proposal != nil && proposal.Target == "deployment-blueprint"
}

// Caller holds the normal project read lock. Reuses the existing private
// blueprint ledger and application-config identity; no target/credential reads.
func (s *Server) orchestratorBlueprintContextLocked(ctx context.Context, p project.Project, snapshot OrchestratorContext) (*OrchestratorBlueprintContext, error) {
	if _, ok := authenticatedContextActor(ctx); !ok {
		return nil, nil
	}
	if err := gtmOrchestratorAuthority(ctx, p.ID, false); err != nil {
		return nil, err
	}
	_, ledger, ledgerErr := s.readDeploymentBlueprintLedger(p)
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "application-config-"+p.ID)
	if err != nil {
		return nil, nil
	}
	configuration, err := readApplicationConfiguration(path, p.ID)
	if err != nil {
		return nil, nil
	}
	source := deploymentplan.Source{ConfigurationRevision: configuration.Revision, Context: &deploymentplan.ContextReference{
		BriefRevision: snapshot.Brief.Revision, SpecificationRevision: snapshot.Specification.Revision, SpecificationDigest: snapshot.Specification.Digest, DesignRevision: snapshot.DesignPlan.Revision, DesignDigest: snapshot.DesignPlan.Digest,
	}}
	if snapshot.SourceKnown && !snapshot.SourceDirty {
		source.Commit = snapshot.SourceCommit
	}
	digest, err := s.foundationContextLocked(p, deploymentplan.Blueprint{ProjectID: p.ID, Source: source})
	if err != nil {
		return nil, nil
	}
	value := &OrchestratorBlueprintContext{ProjectID: p.ID, Source: source, ContextDigest: digest, Available: true}
	if ledgerErr == nil && len(ledger.Revisions) > 0 {
		last := ledger.Revisions[len(ledger.Revisions)-1]
		value.SavedRevision, value.SavedDigest = last.Revision, last.Digest
		raw, err := json.Marshal(last.Blueprint)
		if err != nil {
			return nil, ErrOrchestratorInvalid
		}
		if len(raw) > orchestratorBlueprintLimit {
			value.Available = false
			value.OmittedBytes = len(raw)
		} else {
			var captured deploymentplan.Blueprint
			if json.Unmarshal(raw, &captured) != nil {
				return nil, ErrOrchestratorInvalid
			}
			value.Saved = &captured
		}
	}
	if ledgerErr != nil {
		value.Available = false
		value.UnavailableReason = "saved-topology-unconfirmed"
	}
	// One existing bounded capture per snapshot; failure withdraws only topology
	// assistance, never the ordinary conversation or existing saved topology.
	bounded, cancel := context.WithTimeout(context.WithValue(ctx, journeyGitRootContextKey{}, s.config.ProjectsDir), 3*time.Second)
	defer cancel()
	review, reviewErr := s.captureGitReview(bounded, p.Path, "")
	if reviewErr != nil || !hexDigest(review.Digest) {
		if value.Saved != nil {
			raw, _ := json.Marshal(value.Saved)
			value.OmittedBytes = len(raw)
		}
		value.Available = false
		if value.UnavailableReason == "" {
			value.UnavailableReason = "source-unconfirmed"
		}
		value.SourceReviewDigest = ""
		value.Saved = nil
	} else {
		value.SourceReviewDigest = review.Digest
		value.Source.Commit = ""
		if review.Status == "" {
			value.Source.Commit = review.Head
		}
		if !value.Available && value.UnavailableReason == "" {
			value.UnavailableReason = "topology-size-limit"
		}
	}
	if !validOrchestratorBlueprintContext(value, p.ID) {
		if value.Saved != nil {
			raw, _ := json.Marshal(value.Saved)
			value.OmittedBytes = len(raw)
		}
		value.Saved = nil
		value.Available = false
		value.UnavailableReason = "saved-topology-unsupported"
		if !validOrchestratorBlueprintContext(value, p.ID) {
			return nil, nil
		}
	}
	return value, nil
}

func validOrchestratorBlueprintContext(value *OrchestratorBlueprintContext, projectID string) bool {
	if value == nil {
		return true
	}
	if value.ProjectID != projectID || value.SavedRevision < 0 || value.SavedRevision > blueprintRevisionLimit || value.SavedRevision == 0 && value.SavedDigest != "" || value.SavedRevision > 0 && !hexDigest(value.SavedDigest) || !hexDigest(value.ContextDigest) || value.Source.Context == nil || value.Source.ConfigurationRevision < 0 || value.Source.Commit != "" && (len(value.Source.Commit) != 40 || strings.Trim(value.Source.Commit, "0123456789abcdef") != "") {
		return false
	}
	c := value.Source.Context
	if c.BriefRevision < 0 || c.SpecificationRevision < 0 || c.DesignRevision < 0 || c.SpecificationDigest != "" && !hexDigest(c.SpecificationDigest) || c.DesignDigest != "" && !hexDigest(c.DesignDigest) {
		return false
	}
	if !value.Available {
		if value.Saved != nil {
			return false
		}
		if value.UnavailableReason == "saved-topology-unconfirmed" {
			return value.SavedRevision == 0 && value.SavedDigest == "" && value.OmittedBytes == 0 && (value.SourceReviewDigest == "" || hexDigest(value.SourceReviewDigest))
		}
		if value.UnavailableReason == "saved-topology-unsupported" {
			return value.SavedRevision > 0 && value.OmittedBytes > 0 && (value.SourceReviewDigest == "" || hexDigest(value.SourceReviewDigest))
		}
		if value.UnavailableReason == "source-unconfirmed" {
			return value.SourceReviewDigest == "" && value.OmittedBytes >= 0
		}
		return value.UnavailableReason == "topology-size-limit" && hexDigest(value.SourceReviewDigest) && value.SavedRevision > 0 && value.OmittedBytes > orchestratorBlueprintLimit
	}
	if value.UnavailableReason != "" || !hexDigest(value.SourceReviewDigest) {
		return false
	}
	if value.OmittedBytes != 0 || (value.SavedRevision == 0) != (value.Saved == nil) {
		return false
	}
	if value.Saved != nil {
		raw, err := json.Marshal(value.Saved)
		if err != nil || len(raw) > orchestratorBlueprintLimit || value.Saved.ProjectID != projectID || !blueprintStorageSafe(*value.Saved) {
			return false
		}
		if _, err = deploymentplan.Compile(*value.Saved); err != nil {
			return false
		}
	}
	return true
}

// Unknown references remain blank and become compiler decisions. A model may
// reuse exact saved resource names but cannot invent an existing image, target,
// credential/config key, PVC or telemetry binding and present it as observed.
func blueprintReferenceSet(value *deploymentplan.Blueprint) map[string]bool {
	refs := map[string]bool{}
	if value == nil {
		return refs
	}
	add := func(kind string, v any) { refs[kind+":"+reviewDigest(v)] = true }
	if value.Infrastructure.Reference != "" {
		add("target", value.Infrastructure)
	}
	if value.Infrastructure.Namespace != "" {
		add("namespace", value.Infrastructure.Namespace)
	}
	for _, resource := range value.ExternalResources {
		if resource.Reference != "" {
			add("external", resource)
		}
	}
	for _, service := range value.Services {
		raw, _ := json.Marshal(service)
		fields := map[string]json.RawMessage{}
		if json.Unmarshal(raw, &fields) == nil {
			if collector, present := fields["collector"]; present && string(collector) != "null" {
				add("collector", collector)
			}
		}
		if service.Image != "" {
			add("image", service.Image)
		}
		for _, name := range service.ImagePullSecrets {
			add("image-pull-secret", name)
		}
		if service.Model != nil {
			add("model", service.Model)
		}
		if service.Telemetry != nil {
			add("telemetry", service.Telemetry)
		}
		if service.TelemetryStorage != nil {
			add("telemetry-storage", service.TelemetryStorage)
		}
		for _, ref := range service.Environment {
			add("environment", ref)
		}
	}
	return refs
}
func validateOrchestratorBlueprint(snapshot OrchestratorContext, proposal *OrchestratorBlueprintProposal) error {
	captured := snapshot.DeploymentBlueprint
	if captured == nil || !validOrchestratorBlueprintContext(captured, snapshot.ProjectID) || !captured.Available || proposal == nil || proposal.ExpectedRevision != captured.SavedRevision || proposal.ExpectedDigest != captured.SavedDigest || proposal.ContextDigest != captured.ContextDigest || proposal.SourceReviewDigest != captured.SourceReviewDigest || proposal.Blueprint.ProjectID != snapshot.ProjectID || !reflect.DeepEqual(proposal.Blueprint.Source, captured.Source) || !blueprintStorageSafe(proposal.Blueprint) {
		return ErrOrchestratorInvalid
	}
	raw, err := json.Marshal(proposal.Blueprint)
	if err != nil || len(raw) > orchestratorBlueprintLimit {
		return ErrOrchestratorInvalid
	}
	if _, err = deploymentplan.Compile(proposal.Blueprint); err != nil {
		return ErrOrchestratorInvalid
	}
	if len(proposal.Blueprint.Services) > 12 {
		return ErrOrchestratorInvalid
	}
	if captured.Saved != nil {
		for _, old := range captured.Saved.Services {
			found := false
			for _, next := range proposal.Blueprint.Services {
				if next.ID == old.ID && next.Kind == old.Kind {
					found = true
					break
				}
			}
			if !found {
				return ErrOrchestratorInvalid
			}
		}
	}
	if captured.Saved != nil {
		for _, old := range captured.Saved.ExternalResources {
			found := false
			for _, next := range proposal.Blueprint.ExternalResources {
				if reflect.DeepEqual(old, next) {
					found = true
					break
				}
			}
			if !found {
				return ErrOrchestratorInvalid
			}
		}
	}
	known := blueprintReferenceSet(captured.Saved)
	for ref := range blueprintReferenceSet(&proposal.Blueprint) {
		if !known[ref] {
			return ErrOrchestratorInvalid
		}
	}
	return nil
}

func manualBlueprintTurn(turn storedOrchestratorTurn) bool {
	return turn.Answer != nil && manualBlueprintProposal(turn.Answer.Proposal)
}
func validManualBlueprintTurn(turn storedOrchestratorTurn) bool {
	return !manualBlueprintTurn(turn) || turn.ApplyIntent == "" && turn.AppliedRevision == 0 && turn.ApplyTargetRevision == 0 && turn.ApplyTargetDigest == ""
}
func orchestratorBlueprintBytes(value *OrchestratorBlueprintContext) int {
	if value == nil {
		return 0
	}
	raw, _ := json.MarshalIndent(value, "", "  ")
	return len(raw)
}
