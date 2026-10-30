package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

var errAzureBlueprint = errors.New("The blueprint, current project context, selected Azure foundation or deploy authority changed. Review a fresh foundation plan.")

// Only the server creates this binding. Client digests are equality checks,
// never actor/grant assertions. Optional absence preserves legacy CloudPlan JSON.
type AzureBlueprintBinding struct {
	Version       int                                   `json:"version"`
	Request       deploymentplan.AzureFoundationRequest `json:"request"`
	Actor         management.ExecutionActorRef          `json:"actor"`
	ContextDigest string                                `json:"contextDigest"`
	Fit           deploymentplan.AzureFoundationFit     `json:"fit"`
	PlanDigest    string                                `json:"planDigest"`
}

func foundationPlanDigest(plan CloudPlan) string {
	if plan.Foundation == nil {
		return ""
	}
	binding := *plan.Foundation
	binding.PlanDigest = ""
	plan.Foundation = &binding
	return reviewDigest(plan)
}

func validFoundationPlan(plan CloudPlan) bool {
	b := plan.Foundation
	if b == nil {
		return true
	}
	if b.Version != 1 || plan.Provider != "azure" || b.Actor.Local || b.Actor.UserID == "" || b.Actor.ProjectID != b.Request.Blueprint.ProjectID || !hexDigest(b.ContextDigest) || !hexDigest(b.PlanDigest) || b.PlanDigest != foundationPlanDigest(plan) {
		return false
	}
	fit, err := deploymentplan.AzureFoundation(b.Request, b.Fit.Tier)
	return err == nil && fit.Fits && reviewDigest(fit) == reviewDigest(b.Fit)
}

// False is still Read+Edit+Deploy authority, not a viewer permission. It allows
// observation/cleanup after new allocations are suspended. New foundations also
// require current provisioning/public-exposure policy and the current budget.
func (s *Server) foundationAuthority(ctx context.Context, actor management.ExecutionActorRef, allocate bool, budget float64) (context.Context, error) {
	if s.coordinatorDeploymentAuthorize == nil || actor.UserID == "" || actor.Local || actor.ProjectID == "" {
		return ctx, management.ErrExecutionAuthority
	}
	authority, err := s.coordinatorDeploymentAuthorize(ctx, actor, allocate)
	if err != nil {
		return ctx, err
	}
	current := management.WithExecutionAuthority(ctx, authority)
	captured, err := management.CaptureDeploymentActor(current, actor.ProjectID)
	if err != nil || captured != actor {
		return ctx, management.ErrExecutionAuthority
	}
	if allocate && (!authority.Policy.AllowCloudProvisioning || !authority.Policy.AllowPublicPublishing || budget <= 0 || budget > authority.Policy.MonthlyBudgetUSD) {
		return ctx, management.ErrExecutionAuthority
	}
	return current, nil
}

// Caller holds s.mu; no projectRoot/resolveProject recursion occurs here.
// Source commit is retained intent only: a foundation does not build that commit.
func (s *Server) foundationContextLocked(p project.Project, blueprint deploymentplan.Blueprint) (string, error) {
	if p.ID != blueprint.ProjectID || blueprint.Source.Context == nil {
		return "", errAzureBlueprint
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return "", err
	}
	spec, design := currentSpecification(state), currentDesignPlan(state)
	expected := blueprint.Source.Context
	if expected.BriefRevision != state.Brief.Revision || expected.SpecificationRevision != spec.Revision || expected.DesignRevision != design.Revision || expected.SpecificationDigest != spec.Digest || expected.DesignDigest != design.Digest {
		return "", errAzureBlueprint
	}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "application-config-"+p.ID)
	if err != nil {
		return "", err
	}
	configuration, err := readApplicationConfiguration(path, p.ID)
	if err != nil || configuration.Revision != blueprint.Source.ConfigurationRevision {
		return "", errAzureBlueprint
	}
	return reviewDigest(struct {
		ProjectID             string
		Brief                 Brief
		Specification         ProjectSpecification
		Design                ProjectDesignPlan
		ConfigurationRevision int
	}{p.ID, state.Brief, spec, design, configuration.Revision}), nil
}

func (s *Server) prepareFoundationBinding(ctx context.Context, request CloudRequest) (*AzureBlueprintBinding, error) {
	if request.Foundation == nil {
		return nil, nil
	}
	if request.Provider != "azure" {
		return nil, errAzureBlueprint
	}
	fit, err := deploymentplan.AzureFoundation(*request.Foundation, request.Size)
	if err != nil || !fit.Fits {
		return nil, errAzureBlueprint
	}
	actor, err := management.CaptureDeploymentActor(ctx, request.Foundation.Blueprint.ProjectID)
	if err != nil {
		return nil, err
	}
	// Plan inspection uses deploy authority without acquiring a new allocation.
	current, err := s.foundationAuthority(ctx, actor, false, 0)
	if err != nil {
		return nil, err
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	digest, err := s.foundationContextLocked(p, request.Foundation.Blueprint)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	if _, err = s.foundationAuthority(current, actor, false, 0); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(request.Foundation)
	if err != nil {
		return nil, err
	}
	var captured deploymentplan.AzureFoundationRequest
	if err = json.Unmarshal(raw, &captured); err != nil {
		return nil, err
	}
	return &AzureBlueprintBinding{Version: 1, Request: captured, Actor: actor, ContextDigest: digest, Fit: fit}, nil
}

// Fresh authority is resolved outside s.mu. The exact context is subsequently
// checked under the normal project lock immediately before job reservation.
func (s *Server) authorizeFoundationPlan(ctx context.Context, plan CloudPlan, allocate bool, requireCaller bool) (project.Project, error) {
	if plan.Foundation == nil {
		return project.Project{}, nil
	}
	if !validFoundationPlan(plan) {
		return project.Project{}, errAzureBlueprint
	}
	b := plan.Foundation
	if requireCaller {
		actor, err := management.CaptureDeploymentActor(ctx, b.Actor.ProjectID)
		if err != nil || actor != b.Actor {
			return project.Project{}, management.ErrExecutionAuthority
		}
	}
	if _, err := s.foundationAuthority(ctx, b.Actor, allocate, plan.MonthlyBudgetUSD); err != nil {
		return project.Project{}, err
	}
	return s.resolveProject(b.Actor.ProjectID)
}

func (s *Server) recheckFoundationContextLocked(p project.Project, plan CloudPlan) error {
	if plan.Foundation == nil {
		return nil
	}
	digest, err := s.foundationContextLocked(p, plan.Foundation.Request.Blueprint)
	if err != nil || digest != plan.Foundation.ContextDigest {
		return errAzureBlueprint
	}
	return nil
}

func (s *Server) admitFoundationProvider(ctx context.Context, job CloudJob) error {
	if job.Plan.Foundation == nil {
		return nil
	}
	// Original actor only; background work never adopts a browser's new identity.
	p, err := s.authorizeFoundationPlan(ctx, job.Plan, true, false)
	if err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	saved, ok := s.jobs[job.ID]
	if !ok || saved.Status != "creating" || saved.PlanID != job.PlanID || saved.Plan.Foundation == nil || saved.Plan.Foundation.PlanDigest != job.Plan.Foundation.PlanDigest {
		return errAzureBlueprint
	}
	return s.recheckFoundationContextLocked(p, job.Plan)
}

func (s *Server) foundationJobReadable(ctx context.Context, job CloudJob) bool {
	if job.Plan.Foundation == nil {
		return true
	}
	_, err := s.authorizeFoundationPlan(ctx, job.Plan, false, true)
	return err == nil
}

func foundationProblem(w http.ResponseWriter, err error) {
	status := http.StatusConflict
	if errors.Is(err, management.ErrExecutionAuthority) {
		status = http.StatusForbidden
	}
	problem(w, status, "azure_blueprint_changed", errAzureBlueprint.Error())
}

// Observation may already have been retained. A revoked response is withheld,
// never described as rollback, and never causes another provider operation.
func (s *Server) writeFoundationJobResponse(w http.ResponseWriter, r *http.Request, job CloudJob, status int, value any) {
	if job.Plan.Foundation != nil && !s.foundationJobReadable(r.Context(), job) {
		problem(w, http.StatusForbidden, "azure_foundation_response_withheld", "Current project access could not be verified, so the response was withheld. Saved operation or observation changes are not rolled back. Read the retained job when access is restored; do not replay it automatically.")
		return
	}
	writeJSON(w, status, value)
}
