package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Draft persistence and routine review are separate opt-ins. A routine review
// can only renew an already approved document through the existing validation
// and exact actor/context gates; it never grants execution or deployment.
type CoordinatorPolicy struct {
	ProjectID       string        `json:"projectId"`
	Revision        int           `json:"revision"`
	Mode            string        `json:"mode"`
	ReviewMode      string        `json:"reviewMode"`
	ExecutionMode   string        `json:"executionMode"`
	IntegrationMode string        `json:"integrationMode"`
	PreviewMode     string        `json:"previewMode"`
	DeploymentMode  string        `json:"deploymentMode"`
	AllowedTargets  []string      `json:"allowedTargets"`
	UpdatedAt       string        `json:"updatedAt,omitempty"`
	Actor           *ContextActor `json:"actor,omitempty"`
}
type storedCoordinatorPolicy struct {
	Version      int               `json:"version"`
	AccountScope string            `json:"accountScope"`
	ProjectScope string            `json:"projectScope"`
	Policy       CoordinatorPolicy `json:"policy"`
}

var errCoordinatorPolicy = errors.New("The coordinator policy changed or does not permit this exact draft. Refresh before continuing.")
var errCoordinatorSourceBusy = errors.New("This project source is changing. Wait for its current source operation, then refresh the continuation.")

var errCoordinatorAuthority = errors.New("Current project edit authority is required for this coordinator action.")

func (s *Server) SetCoordinatorAuthorizer(authorize func(context.Context, string) error) {
	s.coordinatorAuthorize = authorize
}
func (s *Server) coordinatorAuthority(ctx context.Context, id string) error {
	if ctx.Err() != nil || s.coordinatorAuthorize == nil {
		return errCoordinatorAuthority
	}
	if _, ok := authenticatedContextActor(ctx); !ok {
		return errCoordinatorAuthority
	}
	if err := s.coordinatorAuthorize(ctx, id); err != nil {
		return errCoordinatorAuthority
	}
	return nil
}
func coordinatorTargets(values []string) ([]string, error) {
	if len(values) > 3 {
		return nil, errCoordinatorPolicy
	}
	selected := map[string]bool{}
	for _, target := range values {
		if selected[target] || (target != "specification" && target != "design-plan" && target != "plan") {
			return nil, errCoordinatorPolicy
		}
		selected[target] = true
	}
	out := []string{}
	for _, target := range []string{"specification", "design-plan", "plan"} {
		if selected[target] {
			out = append(out, target)
		}
	}
	return out, nil
}
func (s *Server) coordinatorPolicyPath(id string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(id) {
		return "", ErrOrchestratorInvalid
	}
	root, err := guardedProjectPath(s.stateDir, "orchestrator-policy", true)
	if err != nil {
		return "", err
	}
	if create {
		if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
		if runtime.GOOS == "windows" {
			err = restrictPrivateFile(root)
		} else {
			err = os.Chmod(root, 0700)
		}
		if err != nil {
			return "", err
		}
	}
	root, err = guardedProjectPath(s.stateDir, "orchestrator-policy", false)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", ErrOrchestratorInvalid
	}
	path, err := guardedProjectPath(s.stateDir, "orchestrator-policy/"+id+".json", create)
	if err == nil {
		if e := hostpath.OneName(path); e != nil && !errors.Is(e, os.ErrNotExist) {
			return "", e
		}
	}
	return path, err
}
func (s *Server) readCoordinatorPolicy(p project.Project) (CoordinatorPolicy, error) {
	policy := CoordinatorPolicy{ProjectID: p.ID, Revision: 0, Mode: "review", ReviewMode: "manual", ExecutionMode: "manual", IntegrationMode: "manual", PreviewMode: "manual", DeploymentMode: "manual", AllowedTargets: []string{"specification", "design-plan", "plan"}}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return policy, err
	}
	path, err := s.coordinatorPolicyPath(p.ID, false)
	if errors.Is(err, os.ErrNotExist) {
		return policy, nil
	}
	if err != nil {
		return policy, err
	}
	var stored storedCoordinatorPolicy
	if err = externalReadJSON(path, 16<<10, &stored); err != nil {
		return policy, ErrOrchestratorInvalid
	}
	if stored.Policy.ExecutionMode == "" {
		stored.Policy.ExecutionMode = "manual"
	}
	if stored.Policy.ReviewMode == "" {
		stored.Policy.ReviewMode = "manual"
	}
	if stored.Policy.IntegrationMode == "" {
		stored.Policy.IntegrationMode = "manual"
	}
	if stored.Policy.PreviewMode == "" {
		stored.Policy.PreviewMode = "manual"
	}
	if stored.Policy.DeploymentMode == "" {
		stored.Policy.DeploymentMode = "manual"
	}
	targets, err := coordinatorTargets(stored.Policy.AllowedTargets)
	if err != nil || (stored.Policy.ReviewMode != "manual" && stored.Policy.ReviewMode != "approved-amendments") || (stored.Policy.ReviewMode == "approved-amendments" && stored.Policy.Mode != "auto-drafts") || (stored.Policy.ExecutionMode != "manual" && stored.Policy.ExecutionMode != "reviewed-tasks") || (stored.Policy.IntegrationMode != "manual" && stored.Policy.IntegrationMode != "reviewed-scopes") || (stored.Policy.PreviewMode != "manual" && stored.Policy.PreviewMode != "checked-source") || (stored.Policy.DeploymentMode != "manual" && stored.Policy.DeploymentMode != "existing-target") || stored.Version != 1 || stored.AccountScope != scope.AccountScope || stored.ProjectScope != scope.ProjectScope || stored.Policy.ProjectID != p.ID || stored.Policy.Revision < 1 || (stored.Policy.Mode != "review" && stored.Policy.Mode != "auto-drafts") || (stored.Policy.Mode == "auto-drafts" && len(targets) == 0) || strings.Join(targets, ",") != strings.Join(stored.Policy.AllowedTargets, ",") || stored.Policy.Actor == nil || stored.Policy.Actor.ID == "" {
		return policy, ErrOrchestratorInvalid
	}
	return stored.Policy, nil
}
func coordinatorPolicyProblem(w http.ResponseWriter, err error) {
	if errors.Is(err, errCoordinatorSourceBusy) {
		journeyError(w, 409, "branch_busy", err.Error())
		return
	}
	if errors.Is(err, errCoordinatorAuthority) {
		journeyError(w, 403, "coordinator_forbidden", err.Error())
		return
	}
	if errors.Is(err, errCoordinatorPolicy) {
		journeyError(w, 409, "coordinator_policy_changed", err.Error())
		return
	}
	orchestratorProblem(w, err)
}
func (s *Server) getCoordinatorPolicy(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	policy, err := s.readCoordinatorPolicy(p)
	s.mu.RUnlock()
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	if r.Context().Err() != nil {
		return
	}
	journeyJSON(w, 200, policy)
}
func (s *Server) saveCoordinatorPolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision int      `json:"expectedRevision"`
		Mode             string   `json:"mode"`
		ReviewMode       *string  `json:"reviewMode"`
		ExecutionMode    *string  `json:"executionMode"`
		IntegrationMode  *string  `json:"integrationMode"`
		PreviewMode      *string  `json:"previewMode"`
		DeploymentMode   *string  `json:"deploymentMode"`
		AllowedTargets   []string `json:"allowedTargets"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	targets, err := coordinatorTargets(req.AllowedTargets)
	if err != nil || (req.ReviewMode != nil && *req.ReviewMode != "manual" && *req.ReviewMode != "approved-amendments") || (req.ExecutionMode != nil && *req.ExecutionMode != "manual" && *req.ExecutionMode != "reviewed-tasks") || (req.IntegrationMode != nil && *req.IntegrationMode != "manual" && *req.IntegrationMode != "reviewed-scopes") || (req.PreviewMode != nil && *req.PreviewMode != "manual" && *req.PreviewMode != "checked-source") || (req.DeploymentMode != nil && *req.DeploymentMode != "manual" && *req.DeploymentMode != "existing-target") || req.ExpectedRevision < 0 || (req.Mode != "review" && req.Mode != "auto-drafts") || (req.Mode == "auto-drafts" && len(targets) == 0) {
		journeyError(w, 400, "invalid_coordinator_policy", "Choose review-only or automatic draft saving with explicit document targets. This grants no approval or execution permission.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if err = s.coordinatorAuthority(r.Context(), p.ID); err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.coordinatorAuthority(r.Context(), p.ID); err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	current, err := s.readCoordinatorPolicy(p)
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	if current.Revision != req.ExpectedRevision {
		coordinatorPolicyProblem(w, errCoordinatorPolicy)
		return
	}
	reviewMode, executionMode, integrationMode, previewMode, deploymentMode := current.ReviewMode, current.ExecutionMode, current.IntegrationMode, current.PreviewMode, current.DeploymentMode
	if req.ReviewMode != nil {
		reviewMode = *req.ReviewMode
	}
	if reviewMode == "approved-amendments" && req.Mode != "auto-drafts" {
		coordinatorPolicyProblem(w, errCoordinatorPolicy)
		return
	}
	if req.ExecutionMode != nil {
		executionMode = *req.ExecutionMode
	}
	if req.IntegrationMode != nil {
		integrationMode = *req.IntegrationMode
	}
	if req.PreviewMode != nil {
		previewMode = *req.PreviewMode
	}
	if req.DeploymentMode != nil {
		deploymentMode = *req.DeploymentMode
	}
	actor, _ := authenticatedContextActor(r.Context())
	policy := CoordinatorPolicy{ProjectID: p.ID, Revision: current.Revision + 1, Mode: req.Mode, ReviewMode: reviewMode, ExecutionMode: executionMode, IntegrationMode: integrationMode, PreviewMode: previewMode, DeploymentMode: deploymentMode, AllowedTargets: targets, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano), Actor: &actor}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	stored := storedCoordinatorPolicy{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Policy: policy}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil || len(data)+1 > 16<<10 {
		coordinatorPolicyProblem(w, ErrOrchestratorInvalid)
		return
	}
	path, err := s.coordinatorPolicyPath(p.ID, true)
	if err == nil {
		err = durableRunJSON(path, stored)
	}
	if err != nil {
		coordinatorPolicyProblem(w, ErrOrchestratorInvalid)
		return
	}
	journeyJSON(w, 200, policy)
}
func (s *Server) advanceOrchestrator(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision       int    `json:"expectedRevision"`
		ExpectedContextDigest  string `json:"expectedContextDigest"`
		ExpectedPolicyRevision int    `json:"expectedPolicyRevision"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	if err := s.coordinatorAuthority(r.Context(), id); err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	if req.ExpectedPolicyRevision < 1 {
		coordinatorPolicyProblem(w, errCoordinatorPolicy)
		return
	}
	err := s.applyOrchestratorProposalGuarded(r.Context(), id, r.PathValue("turnId"), req.ExpectedRevision, req.ExpectedContextDigest, func(p project.Project, saved orchestratorLedger, turn storedOrchestratorTurn) error {
		if err := s.coordinatorAuthority(r.Context(), id); err != nil {
			return err
		}
		policy, err := s.readCoordinatorPolicy(p)
		if err != nil {
			return err
		}
		if policy.Revision != req.ExpectedPolicyRevision || policy.Mode != "auto-drafts" {
			return errCoordinatorPolicy
		}
		actor, ok := authenticatedContextActor(r.Context())
		if !ok || actor.ID != turn.Actor.ID {
			return errCoordinatorAuthority
		}
		if len(orchestratorQuestions(saved)) > 0 || turn.Answer == nil || turn.Answer.Blocking {
			return ErrOrchestratorQuestion
		}
		if turn.Answer.Proposal == nil {
			return ErrOrchestratorInvalid
		}
		allowed := false
		for _, target := range policy.AllowedTargets {
			allowed = allowed || target == turn.Answer.Proposal.Target
		}
		if !allowed {
			return errCoordinatorPolicy
		}
		return nil
	})
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	view, err := s.OrchestratorView(r.Context(), id, 0)
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	journeyJSON(w, 200, view)
}
