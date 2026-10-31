package product

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// This is a saved delegation to an existing target, not credentials, a new
// cluster request or application acceptance. Checks still run independently.
type CoordinatorKubernetesPlan struct {
	RequiresPublicRoute    bool                              `json:"requiresPublicRoute,omitempty"`
	AcknowledgePublicRoute bool                              `json:"acknowledgePublicRoute,omitempty"`
	Preparation            *CoordinatorRuntimePreparationRef `json:"preparation,omitempty"`
	TargetID               string                            `json:"targetId"`
	TargetFingerprint      string                            `json:"targetFingerprint"`
	NamespaceUID           string                            `json:"namespaceUid"`
	BlueprintRevision      int                               `json:"blueprintRevision"`
	BlueprintDigest        string                            `json:"blueprintDigest"`
	BlueprintContextDigest string                            `json:"blueprintContextDigest"`
	PlanDigest             string                            `json:"planDigest"`
	ServiceImages          []CoordinatorKubernetesImage      `json:"serviceImages"`
	RequiresTelemetry      bool                              `json:"requiresTelemetry,omitempty"`
	MonthlyBudgetUSD       float64                           `json:"monthlyBudgetUsd"`
	AcknowledgeCosts       bool                              `json:"acknowledgeCosts"`
	AcknowledgeImages      bool                              `json:"acknowledgeImages"`
	AcknowledgeTelemetry   bool                              `json:"acknowledgeTelemetry"`
}
type CoordinatorKubernetesImage struct {
	ServiceID string `json:"serviceId"`
	Image     string `json:"image"`
}
type CoordinatorKubernetesResource struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	UID  string `json:"uid"`
}
type CoordinatorKubernetesAction struct {
	ActionID                  string                          `json:"actionId"`
	ReviewID                  string                          `json:"reviewId"`
	BindingDigest             string                          `json:"bindingDigest"`
	ApplyAttempted            bool                            `json:"applyAttempted"`
	Status                    string                          `json:"status"`
	Message                   string                          `json:"message"`
	StartedAt                 string                          `json:"startedAt"`
	UpdatedAt                 string                          `json:"updatedAt"`
	OperationRevision         int                             `json:"operationRevision"`
	Resources                 []CoordinatorKubernetesResource `json:"resources"`
	ApplicationHealthVerified bool                            `json:"applicationHealthVerified"`
}

func (s *Server) prepareCoordinatorKubernetesPlan(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TargetID         string  `json:"targetId"`
		MonthlyBudgetUSD float64 `json:"monthlyBudgetUsd"`
		PreparationID    string  `json:"preparationId,omitempty"`
	}
	if !journeyDecode(w, r, &input) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, err := management.CaptureDeploymentActor(r.Context(), p.ID)
	if err != nil {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	if err = s.kubernetesAuthority(r.Context(), actor, false, true, 0); err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	s.mu.RLock()
	_, ledger, err := s.readDeploymentBlueprintLedger(p)
	view := blueprintView(p.ID, ledger)
	digest := ""
	if err == nil && view.Saved != nil {
		digest, err = s.foundationContextLocked(p, view.Saved.Blueprint)
	}
	s.mu.RUnlock()
	if err != nil || view.Saved == nil {
		coordinatorPolicyProblem(w, ErrBlueprintUnavailable)
		return
	}
	saved := view.Saved
	_, targets, err := s.readKubernetesTargets(p.ID)
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	plan := CoordinatorKubernetesPlan{RequiresPublicRoute: saved.Blueprint.PublicRoute != nil, AcknowledgePublicRoute: true, TargetID: input.TargetID, BlueprintRevision: saved.Revision, BlueprintDigest: saved.Digest, BlueprintContextDigest: digest, PlanDigest: saved.PlanDigest, MonthlyBudgetUSD: input.MonthlyBudgetUSD, ServiceImages: []CoordinatorKubernetesImage{}, AcknowledgeCosts: true, AcknowledgeImages: true, AcknowledgeTelemetry: true}
	for _, target := range targets.Targets {
		if target.ID == input.TargetID && target.ID == saved.Blueprint.Infrastructure.Reference && target.Namespace == saved.Blueprint.Infrastructure.Namespace {
			plan.TargetFingerprint, plan.NamespaceUID = target.Fingerprint, target.NamespaceUID
		}
	}
	if input.PreparationID != "" {
		_, preparation, readErr := s.readRuntimePreparation(actor, input.PreparationID)
		if readErr != nil || preparation.Plan.BlueprintRevision != saved.Revision || preparation.Plan.BlueprintDigest != saved.Digest || preparation.Plan.MonthlyBudgetUSD != input.MonthlyBudgetUSD || s.runtimePreparationAuthority(r.Context(), preparation, false) != nil {
			coordinatorPolicyProblem(w, ErrRuntimePreparation)
			return
		}
		plan.Preparation = &CoordinatorRuntimePreparationRef{ID: preparation.Plan.ID, Digest: preparation.Plan.Digest}
	}
	for _, service := range saved.Blueprint.Services {
		plan.ServiceImages = append(plan.ServiceImages, CoordinatorKubernetesImage{service.ID, service.Image})
		if service.Collector != nil {
			plan.RequiresTelemetry = true
		}
	}
	if err = s.coordinatorKubernetesPlanCurrent(r.Context(), p, &plan); err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	if err = s.kubernetesAuthority(r.Context(), actor, false, true, 0); err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	plan.AcknowledgeCosts, plan.AcknowledgeImages, plan.AcknowledgeTelemetry, plan.AcknowledgePublicRoute = false, false, false, false
	journeyJSON(w, 200, plan)
}

func validateCoordinatorKubernetesPlan(plan *CoordinatorKubernetesPlan) error {
	if plan == nil {
		return nil
	}
	if !coolifyID.MatchString(plan.TargetID) || !hexDigest(plan.TargetFingerprint) || !validConnectionToken(plan.NamespaceUID, 128, false) || plan.BlueprintRevision < 1 || plan.BlueprintRevision >= 1_000_000_000 || !hexDigest(plan.BlueprintDigest) || !hexDigest(plan.BlueprintContextDigest) || !hexDigest(plan.PlanDigest) || !plan.AcknowledgeCosts || !plan.AcknowledgeImages || plan.RequiresTelemetry && !plan.AcknowledgeTelemetry || plan.RequiresPublicRoute && !plan.AcknowledgePublicRoute || plan.MonthlyBudgetUSD <= 0 || math.IsNaN(plan.MonthlyBudgetUSD) || math.IsInf(plan.MonthlyBudgetUSD, 0) || len(plan.ServiceImages) < 1 || len(plan.ServiceImages) > 32 {
		return ErrOrchestratorInvalid
	}
	if plan.Preparation != nil && (!runtimePreparationID(plan.Preparation.ID) || !hexDigest(plan.Preparation.Digest)) {
		return ErrOrchestratorInvalid
	}
	seen := map[string]bool{}
	for _, image := range plan.ServiceImages {
		if !coolifyID.MatchString(image.ServiceID) || seen[image.ServiceID] || len(image.Image) > 1024 || (!kubeImmutableImage.MatchString(image.Image) && !(image.Image == "" && plan.Preparation != nil)) {
			return ErrOrchestratorInvalid
		}
		seen[image.ServiceID] = true
	}
	return nil
}
func validateCoordinatorKubernetesSelection(plan *CoordinatorKubernetesPlan, legacy *CoordinatorDeploymentPlan, checks *CoordinatorCheckPlan, outcome string) error {
	if err := validateCoordinatorKubernetesPlan(plan); err != nil {
		return err
	}
	if plan != nil && (legacy != nil || checks == nil || outcome != "publish") {
		return ErrOrchestratorInvalid
	}
	return nil
}
func coordinatorKubernetesReviewID(row coordinatorContinuationRecord) string {
	return "workload-" + reviewDigest([]string{row.ID, row.Delivery.CheckID, "kubernetes"})[:32]
}
func coordinatorKubernetesBinding(row coordinatorContinuationRecord) string {
	return reviewDigest(struct {
		ContinuationID, CheckID, Commit, SourceDigest, RunID string
		Plan                                                 *CoordinatorKubernetesPlan
	}{row.ID, row.Delivery.CheckID, row.Delivery.Commit, row.Delivery.SourceDigest, row.RunID, row.KubernetesPlan})
}
func validateCoordinatorKubernetesContinuation(row coordinatorContinuationRecord) error {
	if err := validateCoordinatorKubernetesSelection(row.KubernetesPlan, row.DeploymentPlan, row.CheckPlan, row.Run.Outcome); err != nil {
		return err
	}
	if row.Delivery == nil || row.Delivery.Kubernetes == nil {
		return nil
	}
	a := row.Delivery.Kubernetes
	if row.KubernetesPlan == nil || row.Delivery.Status != "verified" || a.ActionID != row.ID+"-kubernetes" || a.ReviewID != coordinatorKubernetesReviewID(row) || a.BindingDigest != coordinatorKubernetesBinding(row) || a.ApplicationHealthVerified || len(a.Resources) > 68 || len(a.Message) > 4000 || !feedbackTimestamp(a.StartedAt) || !feedbackTimestamp(a.UpdatedAt) {
		return ErrOrchestratorInvalid
	}
	switch a.Status {
	case "pending", "running", "ready", "unknown", "blocked":
	default:
		return ErrOrchestratorInvalid
	}
	if a.Status == "ready" && (!a.ApplyAttempted || a.OperationRevision < 1 || len(a.Resources) == 0) {
		return ErrOrchestratorInvalid
	}
	for _, r := range a.Resources {
		if r.Kind != "Deployment" && r.Kind != "Service" && r.Kind != "Ingress" && r.Kind != "ConfigMap" || !validConnectionToken(r.Name, 253, false) || !validConnectionToken(r.UID, 128, false) {
			return ErrOrchestratorInvalid
		}
	}
	return nil
}

// Current source is checked outside any continuation write lock. The existing
// Kubernetes review subsequently binds exact run/check and namespace objects.
func (s *Server) coordinatorKubernetesPlanCurrent(ctx context.Context, p project.Project, plan *CoordinatorKubernetesPlan) error {
	if validateCoordinatorKubernetesPlan(plan) != nil || plan == nil {
		return ErrOrchestratorInvalid
	}
	if plan.Preparation != nil {
		return s.coordinatorRuntimeTemplateCurrent(ctx, p, plan)
	}
	s.mu.RLock()
	_, ledger, err := s.readDeploymentBlueprintLedger(p)
	view := blueprintView(p.ID, ledger)
	digest := ""
	if err == nil && view.Saved != nil {
		digest, err = s.foundationContextLocked(p, view.Saved.Blueprint)
	}
	s.mu.RUnlock()
	if err != nil || view.Saved == nil || view.Saved.Revision != plan.BlueprintRevision || view.Saved.Digest != plan.BlueprintDigest || view.Saved.PlanDigest != plan.PlanDigest || digest != plan.BlueprintContextDigest || view.Saved.Blueprint.Infrastructure.Reference != plan.TargetID || len(view.Saved.Blueprint.Services) != len(plan.ServiceImages) {
		return ErrOrchestratorConflict
	}
	if (view.Saved.Blueprint.PublicRoute != nil) != plan.RequiresPublicRoute {
		return ErrOrchestratorConflict
	}
	if plan.RequiresPublicRoute {
		actor, err := management.CaptureDeploymentActor(ctx, p.ID)
		if err != nil || s.kubernetesPublicAuthority(ctx, actor) != nil {
			return ErrOrchestratorConflict
		}
	}
	needsTelemetry := false
	for i, service := range view.Saved.Blueprint.Services {
		if plan.ServiceImages[i] != (CoordinatorKubernetesImage{service.ID, service.Image}) || service.Collector != nil && !plan.AcknowledgeTelemetry {
			return ErrOrchestratorConflict
		}
		if service.Collector != nil {
			needsTelemetry = true
		}
	}
	if needsTelemetry != plan.RequiresTelemetry {
		return ErrOrchestratorConflict
	}
	_, targets, err := s.readKubernetesTargets(p.ID)
	if err != nil {
		return err
	}
	for _, target := range targets.Targets {
		if target.ID == plan.TargetID && target.Fingerprint == plan.TargetFingerprint && target.NamespaceUID == plan.NamespaceUID {
			return ctx.Err()
		}
	}
	return ErrOrchestratorConflict
}
func (s *Server) reauthorizeCoordinatorKubernetesActor(ctx context.Context, p project.Project, row coordinatorContinuationRecord, mutation bool) (context.Context, error) {
	if row.KubernetesPlan == nil || s.coordinatorDeploymentAuthorize == nil || row.ExecutionActor.Local || row.ExecutionActor.UserID != row.Actor.ID || row.ExecutionActor.ProjectID != p.ID {
		return ctx, errCoordinatorAuthority
	}
	authority, err := s.coordinatorDeploymentAuthorize(ctx, row.ExecutionActor, mutation)
	if err != nil {
		return ctx, err
	}
	current := management.WithExecutionAuthority(ctx, authority)
	if ref, err := management.CaptureDeploymentActor(current, p.ID); err != nil || ref != row.ExecutionActor {
		return ctx, errCoordinatorAuthority
	}
	if err = s.kubernetesAuthority(current, row.ExecutionActor, mutation, true, row.KubernetesPlan.MonthlyBudgetUSD); err != nil {
		return ctx, err
	}
	return current, nil
}
func (s *Server) coordinatorKubernetesAuthority(ctx context.Context, p project.Project, row coordinatorContinuationRecord, mutation bool) (context.Context, error) {
	current, err := s.reauthorizeCoordinatorKubernetesActor(ctx, p, row, mutation)
	if err != nil {
		return ctx, err
	}
	if mutation {
		s.mu.Lock()
		err = s.coordinatorDeliveryAuthorityLocked(current, p, row)
		if err == nil {
			var policy CoordinatorPolicy
			policy, err = s.readCoordinatorPolicy(p)
			if err == nil && policy.DeploymentMode != "existing-target" {
				err = errCoordinatorPolicy
			}
		}
		s.mu.Unlock()
		if err == nil {
			err = s.coordinatorKubernetesPlanCurrent(current, p, row.KubernetesPlan)
		}
	}
	return current, err
}

func (s *Server) continueCoordinatorKubernetes(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (coordinatorContinuationRecord, error) {
	if row.KubernetesPlan == nil || row.Delivery == nil || row.Delivery.Status != "verified" {
		return row, ErrOrchestratorInvalid
	}
	current, err := s.coordinatorKubernetesAuthority(ctx, p, row, row.Delivery.Kubernetes == nil)
	if err != nil {
		return row, err
	}
	if row.KubernetesPlan.Preparation != nil && row.Delivery.Kubernetes == nil {
		ready, prepErr := s.continueCoordinatorRuntimePreparation(current, p, row)
		if prepErr != nil || !ready {
			return row, prepErr
		}
	}
	if row.Delivery.Kubernetes == nil {
		copy := *row.Delivery
		row.Delivery = &copy
		now := coordinatorPreviewNow()
		row.Delivery.Kubernetes = &CoordinatorKubernetesAction{ActionID: row.ID + "-kubernetes", ReviewID: coordinatorKubernetesReviewID(row), BindingDigest: coordinatorKubernetesBinding(row), Status: "pending", StartedAt: now, UpdatedAt: now, Resources: []CoordinatorKubernetesResource{}}
		row, err = s.persistCoordinatorKubernetes(current, p, row, "pending", "The original Kubernetes review identity is retained before observation. No workload mutation is implied.", nil)
		if err != nil {
			return row, err
		}
	}
	a := row.Delivery.Kubernetes
	s.mu.Lock()
	live := s.coordinatorActions[a.ActionID]
	if !live {
		if s.coordinatorActions == nil {
			s.coordinatorActions = map[string]bool{}
		}
		s.coordinatorActions[a.ActionID] = true
	}
	s.mu.Unlock()
	if live {
		return row, nil
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(current), 2*time.Minute)
	stop := context.AfterFunc(s.workspaceContext, cancel)
	wakeAfterOriginal := !a.ApplyAttempted
	go func() {
		defer cancel()
		defer stop()
		defer func() {
			s.mu.Lock()
			delete(s.coordinatorActions, a.ActionID)
			s.mu.Unlock()
			if wakeAfterOriginal {
				s.WakeCoordinator()
			}
		}()
		s.invokeCoordinatorKubernetes(bounded, p, row)
	}()
	return row, nil
}

func (s *Server) invokeCoordinatorKubernetes(ctx context.Context, p project.Project, row coordinatorContinuationRecord) {
	a := row.Delivery.Kubernetes
	current, err := s.coordinatorKubernetesAuthority(ctx, p, row, !a.ApplyAttempted)
	if err != nil {
		_, _ = s.persistCoordinatorKubernetes(ctx, p, row, "blocked", "Current original-actor deployment permission, source or policy no longer admits this step.", nil)
		return
	}
	if !a.ApplyAttempted {
		origin := coordinatorKubernetesOrigin{ContinuationID: row.ID, ActorID: row.Actor.ID, ActionID: a.ActionID, BindingDigest: a.BindingDigest}
		effective, planErr := s.coordinatorRuntimeEffectivePlan(current, row)
		if planErr != nil {
			_, _ = s.persistCoordinatorKubernetes(current, p, row, "blocked", "The original preparation has no current confirmed image/topology result.", nil)
			return
		}
		input := KubernetesReviewInput{AcknowledgePublicRoute: effective.AcknowledgePublicRoute, TargetID: effective.TargetID, BlueprintRevision: effective.BlueprintRevision, BlueprintDigest: effective.BlueprintDigest, RunID: row.RunID, CheckID: row.Delivery.CheckID, MonthlyBudgetUSD: effective.MonthlyBudgetUSD}
		_, err = s.ReviewKubernetesDeployment(context.WithValue(current, coordinatorKubernetesContextKey{}, origin), row.ExecutionActor, input)
		if err != nil {
			_, _ = s.persistCoordinatorKubernetes(current, p, row, "blocked", "The exact saved topology, checked source, namespace or references could not be reviewed. No Apply was issued.", nil)
			return
		}
		// Only this successful CAS may issue Apply. A crash between this marker and
		// the existing Kubernetes admission marker remains unknown, never replayed.
		a.ApplyAttempted = true
		row, err = s.persistCoordinatorKubernetes(current, p, row, "running", "The original Apply intent is durable. A lost outcome is observed by this exact review ID, never repeated.", nil)
		if err != nil {
			return
		}
		status, callErr := s.KubernetesDeploymentAction(current, row.ExecutionActor, a.ReviewID, true, true, true, row.KubernetesPlan.AcknowledgeTelemetry)
		s.finishCoordinatorKubernetes(current, p, row, status, callErr)
		return
	}
	status, callErr := s.KubernetesDeploymentAction(current, row.ExecutionActor, a.ReviewID, false, false, false)
	s.finishCoordinatorKubernetes(current, p, row, status, callErr)
}
func (s *Server) finishCoordinatorKubernetes(ctx context.Context, p project.Project, row coordinatorContinuationRecord, status KubernetesDeploymentStatus, err error) {
	if err != nil {
		_, _ = s.persistCoordinatorKubernetes(ctx, p, row, "unknown", "The original workload outcome could not be confirmed. Retained resources may exist; no Apply is replayed.", nil)
		return
	}
	next := "running"
	message := "The original workload is observed but not yet ready. No new Apply was issued."
	switch status.State {
	case "ready":
		next = "ready"
		message = "The original namespace workloads report deployment readiness. Application behavior, public availability and inference quality remain separate checks."
	case "blocked", "failed":
		next = "blocked"
		message = "The original workload is blocked. Inspect its exact retained operation before authorizing replacement work."
	case "unknown":
		next = "unknown"
		message = "The original operation remains unknown; no create or update is replayed."
	}
	_, _ = s.persistCoordinatorKubernetes(ctx, p, row, next, message, &status)
}
func (s *Server) persistCoordinatorKubernetes(ctx context.Context, p project.Project, row coordinatorContinuationRecord, status, message string, observed *KubernetesDeploymentStatus) (coordinatorContinuationRecord, error) {
	if _, err := s.reauthorizeCoordinatorKubernetesActor(ctx, p, row, false); err != nil {
		return row, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readContinuations(p, row.Actor.ID)
	index := findContinuation(rows, row.ID)
	if err != nil || index < 0 {
		return row, ErrOrchestratorConflict
	}
	prior := rows.Rows[index]
	if prior.Revision != row.Revision || row.Delivery == nil || row.Delivery.Kubernetes == nil {
		return prior, ErrOrchestratorConflict
	}
	delivery := *row.Delivery
	action := *delivery.Kubernetes
	delivery.Kubernetes = &action
	row.Delivery = &delivery
	if prior.Delivery != nil && prior.Delivery.Kubernetes != nil {
		old := prior.Delivery.Kubernetes
		if old.ActionID != action.ActionID || old.ReviewID != action.ReviewID || old.BindingDigest != action.BindingDigest || old.ApplyAttempted && !action.ApplyAttempted {
			return prior, ErrOrchestratorConflict
		}
	}
	action.Status, action.Message, action.UpdatedAt = status, message, coordinatorPreviewNow()
	if observed != nil {
		if observed.Review.ID != action.ReviewID || observed.Review.CoordinatorOrigin == nil || observed.Review.CoordinatorOrigin.BindingDigest != action.BindingDigest || observed.ApplicationHealthVerified {
			return prior, ErrOrchestratorConflict
		}
		if operation := observed.Operation; operation != nil {
			if operation.Request.Identity.OperationID != action.ReviewID {
				return prior, ErrOrchestratorConflict
			}
			action.OperationRevision = operation.Revision
			action.Resources = []CoordinatorKubernetesResource{}
			for _, r := range operation.Resources {
				if r.ObservedUID != "" {
					action.Resources = append(action.Resources, CoordinatorKubernetesResource{r.Kind, r.Name, r.ObservedUID})
				}
			}
		}
	}
	top := "deploying"
	if status == "ready" {
		top = "deployed"
	} else if status == "blocked" {
		top = "deployment-blocked"
	} else if status == "unknown" {
		top = "unknown"
	}
	if prior.Status == top && reviewDigest(prior.Delivery) == reviewDigest(row.Delivery) {
		return prior, nil
	}
	row.Status, row.Message, row.UpdatedAt = top, message, action.UpdatedAt
	row.Revision++
	if err = validateCoordinatorKubernetesContinuation(row); err != nil {
		return prior, err
	}
	rows.Rows[index] = row
	return row, s.saveContinuations(rows)
}

type coordinatorKubernetesContextKey struct{}
type coordinatorKubernetesOrigin struct {
	ContinuationID string `json:"continuationId"`
	ActorID        string `json:"actorId"`
	ActionID       string `json:"actionId"`
	BindingDigest  string `json:"bindingDigest"`
}

func sameCoordinatorKubernetesReviewInput(row KubernetesReview, input KubernetesReviewInput) bool {
	return row.PublicRouteAcknowledged == input.AcknowledgePublicRoute && row.Target.ID == input.TargetID && row.BlueprintRevision == input.BlueprintRevision && row.BlueprintDigest == input.BlueprintDigest && row.RunID == input.RunID && row.CheckID == input.CheckID && row.MonthlyBudgetUSD == input.MonthlyBudgetUSD
}
func (s *Server) coordinatorKubernetesRequestedReview(ctx context.Context, actor management.ExecutionActorRef, input KubernetesReviewInput, origin coordinatorKubernetesOrigin) (string, error) {
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return "", err
	}
	s.mu.RLock()
	rows, err := s.readContinuations(p, origin.ActorID)
	index := findContinuation(rows, origin.ContinuationID)
	s.mu.RUnlock()
	if err != nil || index < 0 {
		return "", ErrOrchestratorConflict
	}
	row := rows.Rows[index]
	effective, planErr := s.coordinatorRuntimeEffectivePlan(ctx, row)
	if planErr != nil {
		return "", planErr
	}
	if row.ExecutionActor != actor || row.Delivery == nil || row.Delivery.Kubernetes == nil || row.KubernetesPlan == nil || row.Delivery.Kubernetes.ActionID != origin.ActionID || row.Delivery.Kubernetes.BindingDigest != origin.BindingDigest || coordinatorKubernetesBinding(row) != origin.BindingDigest || input.RunID != row.RunID || input.CheckID != row.Delivery.CheckID || input.TargetID != effective.TargetID || input.BlueprintRevision != effective.BlueprintRevision || input.BlueprintDigest != effective.BlueprintDigest || input.MonthlyBudgetUSD != row.KubernetesPlan.MonthlyBudgetUSD {
		return "", ErrOrchestratorConflict
	}
	if _, err = s.coordinatorKubernetesAuthority(ctx, p, row, true); err != nil {
		return "", err
	}
	return row.Delivery.Kubernetes.ReviewID, nil
}

// Persisted origin is correlation, not authority. Every underlying existing
// Kubernetes callback reopens the original continuation and fresh grants.
func (s *Server) validateCoordinatorKubernetesOrigin(ctx context.Context, review KubernetesReview, mutation bool) error {
	origin := review.CoordinatorOrigin
	if origin == nil {
		return nil
	}
	p, err := s.resolveProject(review.Request.Identity.ProjectID)
	if err != nil {
		return err
	}
	s.mu.RLock()
	rows, err := s.readContinuations(p, origin.ActorID)
	index := findContinuation(rows, origin.ContinuationID)
	s.mu.RUnlock()
	if err != nil || index < 0 {
		return ErrOrchestratorConflict
	}
	row := rows.Rows[index]
	if row.Delivery == nil || row.Delivery.Kubernetes == nil || row.ExecutionActor != kubeActor(review.Request.Identity) || row.Delivery.Kubernetes.ActionID != origin.ActionID || row.Delivery.Kubernetes.ReviewID != review.ID || row.Delivery.Kubernetes.BindingDigest != origin.BindingDigest || coordinatorKubernetesBinding(row) != origin.BindingDigest || mutation && !row.Delivery.Kubernetes.ApplyAttempted {
		return ErrOrchestratorConflict
	}
	_, err = s.coordinatorKubernetesAuthority(ctx, p, row, mutation)
	return err
}
