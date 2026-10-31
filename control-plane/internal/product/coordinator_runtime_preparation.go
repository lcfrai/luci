package product

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type CoordinatorRuntimePreparationRef struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

func (s *Server) coordinatorRuntimeTemplateCurrent(ctx context.Context, p project.Project, plan *CoordinatorKubernetesPlan) error {
	actor, err := management.CaptureDeploymentActor(ctx, p.ID)
	if err != nil {
		return err
	}
	_, preparation, err := s.readRuntimePreparation(actor, plan.Preparation.ID)
	if err != nil || preparation.Plan.Digest != plan.Preparation.Digest || preparation.Plan.BlueprintRevision != plan.BlueprintRevision || preparation.Plan.BlueprintDigest != plan.BlueprintDigest || preparation.Plan.ContextDigest != plan.BlueprintContextDigest || preparation.Plan.MonthlyBudgetUSD != plan.MonthlyBudgetUSD || preparation.Template.Infrastructure.Reference != plan.TargetID || len(preparation.Template.Services) != len(plan.ServiceImages) {
		return ErrRuntimePreparation
	}
	if (preparation.Template.PublicRoute != nil) != plan.RequiresPublicRoute {
		return ErrRuntimePreparation
	}
	if plan.RequiresPublicRoute && (!plan.AcknowledgePublicRoute || s.kubernetesPublicAuthority(ctx, actor) != nil) {
		return ErrRuntimePreparation
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return err
	}
	for i, service := range preparation.Template.Services {
		if plan.ServiceImages[i] != (CoordinatorKubernetesImage{service.ID, service.Image}) {
			return ErrRuntimePreparation
		}
		if service.Image == "" {
			found := false
			for _, b := range preparation.Plan.Builds {
				if b.Selection.ServiceID == service.ID {
					found = true
				}
			}
			if !found {
				return ErrRuntimePreparation
			}
		}
	}
	compiled, compileErr := deploymentplan.Compile(preparation.Template)
	if compileErr != nil || compiled.Digest != plan.PlanDigest {
		return ErrRuntimePreparation
	}
	if preparation.SavedBlueprintRevision == 0 {
		if published, readErr := s.runtimePreparationPublishedBlueprint(ctx, preparation); readErr == nil {
			preparation.SavedBlueprintRevision, preparation.SavedBlueprintDigest = published.Revision, published.Digest
		}
	}
	if preparation.SavedBlueprintRevision > 0 {
		effective, err := s.runtimePreparationEffectiveSelection(preparation, plan)
		if err != nil {
			return err
		}
		return s.coordinatorKubernetesPlanCurrent(ctx, p, effective)
	}
	if err = s.runtimePreparationAuthority(ctx, preparation, false); err != nil {
		return err
	}
	_, targets, err := s.readKubernetesTargets(p.ID)
	if err != nil {
		return err
	}
	for _, target := range targets.Targets {
		if target.ID == plan.TargetID && target.NamespaceUID == plan.NamespaceUID && target.Fingerprint == plan.TargetFingerprint && target.Namespace == preparation.Template.Infrastructure.Namespace {
			return ctx.Err()
		}
	}
	return ErrRuntimePreparation
}
func (s *Server) runtimePreparationEffectiveSelection(preparation storedRuntimePreparation, original *CoordinatorKubernetesPlan) (*CoordinatorKubernetesPlan, error) {
	if original == nil || original.Preparation == nil || preparation.Plan.ID != original.Preparation.ID || preparation.Plan.Digest != original.Preparation.Digest || preparation.SavedBlueprintRevision <= preparation.Plan.BlueprintRevision || !hexDigest(preparation.SavedBlueprintDigest) {
		return nil, ErrRuntimePreparation
	}
	b, err := runtimePreparationDerivedBlueprint(preparation)
	if err != nil {
		return nil, err
	}
	p, err := s.resolveProject(preparation.Plan.ProjectID)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	_, ledger, err := s.readDeploymentBlueprintLedger(p)
	view := blueprintView(p.ID, ledger)
	s.mu.RUnlock()
	if err != nil || view.Saved == nil || view.Saved.Revision != preparation.SavedBlueprintRevision || view.Saved.Digest != preparation.SavedBlueprintDigest || reviewDigest(view.Saved.Blueprint) != reviewDigest(b) {
		return nil, ErrRuntimePreparation
	}
	out := *original
	out.Preparation = nil
	out.BlueprintRevision = view.Saved.Revision
	out.BlueprintDigest = view.Saved.Digest
	out.PlanDigest = view.Saved.PlanDigest
	out.ServiceImages = []CoordinatorKubernetesImage{}
	for _, service := range b.Services {
		out.ServiceImages = append(out.ServiceImages, CoordinatorKubernetesImage{service.ID, service.Image})
	}
	return &out, nil
}
func (s *Server) coordinatorRuntimeEffectivePlan(ctx context.Context, row coordinatorContinuationRecord) (*CoordinatorKubernetesPlan, error) {
	if row.KubernetesPlan == nil {
		return nil, ErrRuntimePreparation
	}
	if row.KubernetesPlan.Preparation == nil {
		return row.KubernetesPlan, nil
	}
	_, p, err := s.readRuntimePreparation(row.ExecutionActor, row.KubernetesPlan.Preparation.ID)
	if err != nil {
		return nil, err
	}
	if err = s.kubernetesAuthority(ctx, row.ExecutionActor, false, true, 0); err != nil {
		return nil, err
	}
	return s.runtimePreparationEffectiveSelection(p, row.KubernetesPlan)
}

// One existing coordinator action at a time. Each distinct service has its own
// original durable attempt. Restart only observes that attempt. A positive
// original result advances to the next planned service, never a blind replay.
func (s *Server) continueCoordinatorRuntimePreparation(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (bool, error) {
	_, prep, err := s.readRuntimePreparation(row.ExecutionActor, row.KubernetesPlan.Preparation.ID)
	if err != nil {
		return false, err
	}
	if prep.Plan.Digest != row.KubernetesPlan.Preparation.Digest {
		return false, ErrRuntimePreparation
	}
	if prep.SavedBlueprintRevision > 0 {
		_, err = s.coordinatorRuntimeEffectivePlan(ctx, row)
		return err == nil, err
	}
	if _, err = s.coordinatorKubernetesAuthority(ctx, p, row, true); err != nil {
		return false, err
	}
	var serviceID string
	fresh := false
	for _, build := range prep.Plan.Builds {
		var attempt *RuntimeBuildAttempt
		for i := range prep.Builds {
			if prep.Builds[i].ServiceID == build.Selection.ServiceID {
				attempt = &prep.Builds[i]
			}
		}
		if attempt != nil && attempt.Runtime != nil && attempt.Runtime.Phase == "published" && attempt.Runtime.ImmutableImage != "" && !attempt.Runtime.ResourcesHeld && attempt.Runtime.CleanupConfirmed {
			continue
		}
		serviceID = build.Selection.ServiceID
		fresh = attempt == nil
		break
	}
	if serviceID == "" && prep.Plan.Resources != nil && (prep.Resources == nil || prep.Resources.State != "ready") {
		return s.continueCoordinatorRuntimeResources(ctx, p, row, prep)
	}
	key := row.ID + "-runtime-preparation"
	s.mu.Lock()
	if s.coordinatorActions == nil {
		s.coordinatorActions = map[string]bool{}
	}
	live := s.coordinatorActions[key]
	if !live {
		s.coordinatorActions[key] = true
	}
	s.mu.Unlock()
	if live {
		return false, nil
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Minute)
	stop := context.AfterFunc(s.workspaceContext, cancel)
	go func() {
		finalized := false
		defer cancel()
		defer stop()
		defer func() {
			s.mu.Lock()
			delete(s.coordinatorActions, key)
			s.mu.Unlock()
			if fresh || finalized {
				s.WakeCoordinator()
			}
		}()
		current, err := s.coordinatorKubernetesAuthority(bounded, p, row, true)
		if err != nil {
			return
		}
		if serviceID == "" {
			got, err := s.FinalizeRuntimePreparation(current, row.ExecutionActor, prep.Plan.ID)
			finalized = err == nil && got.SavedBlueprintRevision > 0
			return
		}
		_, _ = s.BuildRuntimePreparationService(current, row.ExecutionActor, prep.Plan.ID, serviceID, row.RunID, row.Delivery.CheckID)
	}()
	return false, nil
}
