package product

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

func runtimePreparationDerivedBlueprint(row storedRuntimePreparation) (deploymentplan.Blueprint, error) {
	raw, err := json.Marshal(row.Template)
	if err != nil {
		return deploymentplan.Blueprint{}, err
	}
	var next deploymentplan.Blueprint
	if json.Unmarshal(raw, &next) != nil {
		return next, ErrRuntimePreparation
	}
	commit := ""
	input := ""
	check := ""
	if len(row.Plan.Builds) == 0 && row.Resources != nil {
		commit = row.Resources.Source.Commit
		input = row.Resources.Source.RunInputDigest
		check = row.Resources.Source.CheckDigest
	}
	for _, build := range row.Plan.Builds {
		var original *RuntimeBuildAttempt
		for i := range row.Builds {
			if row.Builds[i].ServiceID == build.Selection.ServiceID {
				original = &row.Builds[i]
			}
		}
		if original == nil || original.Runtime == nil || original.Runtime.Phase != "published" || original.Runtime.ResourcesHeld || !original.Runtime.CleanupConfirmed || !kubeImmutableImage.MatchString(original.Runtime.ImmutableImage) || !strings.HasPrefix(original.Runtime.ImmutableImage, row.Plan.Registry.Repository+"@") {
			return next, ErrRuntimePreparation
		}
		if commit == "" {
			commit, input, check = original.Commit, original.RunInputDigest, original.CheckDigest
		} else if commit != original.Commit || input != original.RunInputDigest || check != original.CheckDigest {
			return next, ErrRuntimePreparation
		}
		found := false
		for i := range next.Services {
			if next.Services[i].ID == original.ServiceID {
				next.Services[i].Image = original.Runtime.ImmutableImage
				found = true
			}
		}
		if !found {
			return next, ErrRuntimePreparation
		}
	}
	if !deliverySHA.MatchString(commit) {
		return next, ErrRuntimePreparation
	}
	next.Source.Commit = commit
	if err = deriveRuntimeResourceReferences(row, &next); err != nil {
		return next, err
	}
	// Compilation retains all unrelated original topology/runtime references.
	// Provisioned references will be sealed by the same preparation before this
	// finalization entry point when a plan requests them.
	plan, err := deploymentplan.Compile(next)
	if err != nil {
		return next, ErrRuntimePreparation
	}
	return plan.Blueprint, nil
}

func (s *Server) FinalizeRuntimePreparation(ctx context.Context, actor management.ExecutionActorRef, id string) (RuntimePreparation, error) {
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	_, row, err := s.readRuntimePreparation(actor, id)
	if err != nil {
		return RuntimePreparation{}, err
	}
	next, err := runtimePreparationDerivedBlueprint(row)
	if err != nil {
		return row.RuntimePreparation, err
	}
	// All original positive daemon/registry receipts must still be readable;
	// Observe only, never a replacement build or tag adoption on recovery.
	for _, b := range row.Builds {
		if b.Runtime == nil || s.PreviewRuntime().Observe == nil {
			return row.RuntimePreparation, ErrRuntimePreparation
		}
		got, err := s.PreviewRuntime().Observe(ctx, b.Runtime.Identity)
		if err != nil || got.Phase != "published" || got.ImmutableImage != b.Runtime.ImmutableImage || got.ResourcesHeld || !got.CleanupConfirmed {
			return row.RuntimePreparation, ErrRuntimePreparation
		}
		fresh, err := s.runtimeBuildSourceForFinalization(ctx, row, b)
		if err != nil || !sameRuntimeBuildSource(fresh, b) {
			return row.RuntimePreparation, ErrRuntimePreparation
		}
	}
	if row.Plan.Resources != nil {
		if row.Resources == nil || row.Resources.State != "ready" {
			return row.RuntimePreparation, ErrRuntimePreparation
		}
		if _, err = s.runtimeBuildSourceForFinalization(ctx, row, row.Resources.Source); err != nil {
			return row.RuntimePreparation, err
		}
		if err = s.observeRuntimeResourcesForFinalization(ctx, row); err != nil {
			return row.RuntimePreparation, err
		}
	}
	if _, err = s.runtimeRegistryCredentials(actor.ProjectID, row.Plan.Registry); err != nil {
		return row.RuntimePreparation, err
	}
	// This existing API is both the exact template CAS and crash-gap replay key.
	// If its successful receipt was lost, the same key+body can confirm only the
	// original latest revision. A different manual save remains a conflict.
	view, err := s.SaveDeploymentBlueprint(ctx, actor.ProjectID, DeploymentBlueprintSave{ExpectedRevision: row.Plan.BlueprintRevision, ExpectedDigest: row.Plan.BlueprintDigest, IdempotencyKey: row.Plan.ID + "-images", Blueprint: next}, func(call context.Context, projectID string, write bool) error {
		if projectID != actor.ProjectID {
			return ErrRuntimePreparation
		}
		return s.kubernetesAuthority(call, actor, false, true, 0)
	})
	if err != nil || view.Saved == nil {
		if err == nil {
			err = ErrRuntimePreparation
		}
		return row.RuntimePreparation, err
	}
	if reviewDigest(view.Saved.Blueprint) != reviewDigest(next) {
		return row.RuntimePreparation, ErrRuntimePreparation
	}
	s.runtimePreparationMu.Lock()
	defer s.runtimePreparationMu.Unlock()
	file, current, err := s.readRuntimePreparation(actor, id)
	if err != nil {
		return row.RuntimePreparation, err
	}
	if current.Revision != row.Revision {
		return current.RuntimePreparation, ErrRuntimePreparation
	}
	if current.SavedBlueprintRevision != 0 && (current.SavedBlueprintRevision != view.Saved.Revision || current.SavedBlueprintDigest != view.Saved.Digest) {
		return current.RuntimePreparation, ErrRuntimePreparation
	}
	current.SavedBlueprintRevision, current.SavedBlueprintDigest = view.Saved.Revision, view.Saved.Digest
	current.Revision++
	if err = runtimePreparationBytes(current); err != nil {
		return row.RuntimePreparation, err
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	if err = durableRunJSON(file, current); err != nil {
		return row.RuntimePreparation, err
	}
	if err = s.indexRuntimePreparation(current); err != nil {
		return current.RuntimePreparation, err
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	return current.RuntimePreparation, nil
}

// After the original image CAS the template revision has intentionally moved;
// this validates original checked work without pretending the old template is
// still latest. SaveDeploymentBlueprint above independently enforces old-to-new CAS.
func (s *Server) runtimeBuildSourceForFinalization(ctx context.Context, row storedRuntimePreparation, want RuntimeBuildAttempt) (RuntimeBuildAttempt, error) {
	if err := s.kubernetesAuthority(ctx, row.Plan.Actor, false, true, 0); err != nil {
		return RuntimeBuildAttempt{}, err
	}
	run, err := s.GetProjectSwarmRun(row.Plan.ProjectID, want.RunID)
	if err != nil || !swarmAllIntegrated(run) || reviewDigest(run.Input) != want.RunInputDigest {
		return RuntimeBuildAttempt{}, ErrRuntimePreparation
	}
	check, err := s.SnapshotProjectRunCheck(ctx, row.Plan.ProjectID, want.CheckID, run.Input.Approvals, run.Input.Requirements)
	if err != nil || validProjectRunCheck(run.Input, check) != nil || kubeCheckDigest(check) != want.CheckDigest || check.Commit != want.Commit || check.SourceDigest != want.SourceDigest {
		return RuntimeBuildAttempt{}, ErrRuntimePreparation
	}
	if err = s.ValidateDeploymentSource(ctx, row.Plan.ProjectID, want.Commit); err != nil {
		return RuntimeBuildAttempt{}, err
	}
	p, err := s.resolveProject(row.Plan.ProjectID)
	if err != nil {
		return RuntimeBuildAttempt{}, err
	}
	s.mu.RLock()
	state, readErr := s.readApprovedJourney(p.Path, p.ID)
	fresh := projectRunContextFreshness(run, state, readErr)
	contextDigest, contextErr := s.foundationContextLocked(p, row.Template)
	s.mu.RUnlock()
	if fresh.Status != "current" || contextErr != nil || contextDigest != row.Plan.ContextDigest {
		return RuntimeBuildAttempt{}, ErrRuntimePreparation
	}
	return want, nil
}

// Exact original idempotency witness, not merely an equal-looking topology.
// This covers successful blueprint publication before the preparation receipt.
func (s *Server) runtimePreparationPublishedBlueprint(ctx context.Context, row storedRuntimePreparation) (*SavedDeploymentBlueprint, error) {
	next, err := runtimePreparationDerivedBlueprint(row)
	if err != nil {
		return nil, err
	}
	actor, ok := authenticatedContextActor(ctx)
	if !ok || actor.ID != row.Plan.Actor.UserID {
		return nil, ErrRuntimePreparation
	}
	request := DeploymentBlueprintSave{ExpectedRevision: row.Plan.BlueprintRevision, ExpectedDigest: row.Plan.BlueprintDigest, IdempotencyKey: row.Plan.ID + "-images", Blueprint: next}
	key, digest := reviewDigest([]string{row.Plan.ProjectID, actor.ID, request.IdempotencyKey}), reviewDigest(request)
	p, err := s.resolveProject(row.Plan.ProjectID)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ledger, err := s.readDeploymentBlueprintLedger(p)
	if err != nil || len(ledger.Revisions) == 0 {
		return nil, ErrRuntimePreparation
	}
	last := ledger.Revisions[len(ledger.Revisions)-1]
	if last.RequestKey != key || last.RequestHash != digest || reviewDigest(last.Blueprint) != reviewDigest(next) {
		return nil, ErrRuntimePreparation
	}
	copy := last.SavedDeploymentBlueprint
	return &copy, nil
}
