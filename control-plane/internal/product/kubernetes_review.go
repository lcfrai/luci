package product

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"regexp"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/management"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubernetesReviewInput struct {
	AcknowledgePublicRoute bool    `json:"acknowledgePublicRoute,omitempty"`
	TargetID               string  `json:"targetId"`
	BlueprintRevision      int     `json:"blueprintRevision"`
	BlueprintDigest        string  `json:"blueprintDigest"`
	RunID                  string  `json:"runId"`
	CheckID                string  `json:"checkId"`
	MonthlyBudgetUSD       float64 `json:"monthlyBudgetUsd"`
}
type KubernetesReview struct {
	PublicRouteAcknowledged bool                         `json:"publicRouteAcknowledged,omitempty"`
	ID                      string                       `json:"id"`
	CoordinatorOrigin       *coordinatorKubernetesOrigin `json:"coordinatorOrigin,omitempty"`
	Request                 deploymentkube.Request       `json:"request"`
	Target                  KubernetesTargetDescriptor   `json:"target"`
	BlueprintRevision       int                          `json:"blueprintRevision"`
	BlueprintDigest         string                       `json:"blueprintDigest"`
	RunID                   string                       `json:"runId"`
	CheckID                 string                       `json:"checkId"`
	RunInputDigest          string                       `json:"runInputDigest"`
	CheckDigest             string                       `json:"checkDigest"`
	ContextDigest           string                       `json:"contextDigest"`
	MonthlyBudgetUSD        float64                      `json:"monthlyBudgetUsd"`
	CreatedAt               string                       `json:"createdAt"`
	ExpiresAt               string                       `json:"expiresAt"`
	Applied                 bool                         `json:"applied"` // admission attempted, never a successful-deployment claim
}
type kubernetesReviewLedger struct {
	Version      int                `json:"version"`
	ProjectID    string             `json:"projectId"`
	AccountScope string             `json:"accountScope"`
	ProjectScope string             `json:"projectScope"`
	Reviews      []KubernetesReview `json:"reviews"`
}
type KubernetesDeploymentStatus struct {
	Review                    KubernetesReview          `json:"review"`
	Operation                 *deploymentkube.Operation `json:"operation"`
	State                     string                    `json:"state"`
	ApplicationHealthVerified bool                      `json:"applicationHealthVerified"`
	ImageProvenance           string                    `json:"imageProvenance"`
}

var kubeImmutableImage = regexp.MustCompile(`^[^\s@]+@sha256:[a-f0-9]{64}$`)

func kubeActor(id deploymentkube.Identity) management.ExecutionActorRef {
	return management.ExecutionActorRef{UserID: id.ActorID, TeamID: id.TeamID, ProjectID: id.ProjectID}
}

func (s *Server) readKubernetesReviews(projectID string) (string, kubernetesReviewLedger, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return "", kubernetesReviewLedger{}, err
	}
	s.mu.RLock()
	scope, err := s.approvalScopes(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return "", kubernetesReviewLedger{}, err
	}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "kubernetes-reviews-"+projectID)
	empty := kubernetesReviewLedger{Version: 1, ProjectID: projectID, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Reviews: []KubernetesReview{}}
	if err != nil {
		return "", empty, err
	}
	var ledger kubernetesReviewLedger
	err = externalReadJSON(path, 32<<20, &ledger)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || ledger.Version != 1 || ledger.ProjectID != projectID || ledger.AccountScope != empty.AccountScope || ledger.ProjectScope != empty.ProjectScope || ledger.Reviews == nil || len(ledger.Reviews) > 128 {
		return "", empty, ErrKubernetesReview
	}
	seen := map[string]bool{}
	for _, row := range ledger.Reviews {
		created, ce := time.Parse(time.RFC3339Nano, row.CreatedAt)
		expiry, ee := time.Parse(time.RFC3339Nano, row.ExpiresAt)
		plan, pe := deploymentplan.Compile(row.Request.Blueprint)
		if ce != nil || ee != nil || !expiry.Equal(created.Add(10*time.Minute)) || pe != nil || plan.State != "compiled-prototype" || row.ID != row.Request.ApprovalReference || row.ID != row.Request.Identity.OperationID || row.Request.Identity.ProjectID != projectID || !validKubeReceipt(row.ID) || seen[row.ID] || row.Target.ID != row.Request.Identity.TargetID || row.Target.Fingerprint != row.Request.Identity.TargetFingerprint || row.Target.ProjectID != projectID || row.Target.Namespace != plan.Blueprint.Infrastructure.Namespace || row.Request.Identity.PlanDigest != plan.Digest || !hexDigest(row.BlueprintDigest) || !hexDigest(row.ContextDigest) || !hexDigest(row.RunInputDigest) || !hexDigest(row.CheckDigest) || row.BlueprintRevision < 1 || row.MonthlyBudgetUSD <= 0 || math.IsNaN(row.MonthlyBudgetUSD) || math.IsInf(row.MonthlyBudgetUSD, 0) {
			return "", empty, ErrKubernetesReview
		}
		if row.CoordinatorOrigin != nil {
			origin := row.CoordinatorOrigin
			if !prototypeReleaseID.MatchString(origin.ContinuationID) || origin.ActorID != row.Request.Identity.ActorID || origin.ActionID != origin.ContinuationID+"-kubernetes" || !hexDigest(origin.BindingDigest) {
				return "", empty, ErrKubernetesReview
			}
		}
		if (row.Request.Blueprint.PublicRoute != nil) != row.PublicRouteAcknowledged {
			return "", empty, ErrKubernetesReview
		}
		seen[row.ID] = true
		if deploymentkube.ValidateReferenceBinding(row.Request) != nil {
			return "", empty, ErrKubernetesReview
		}
	}
	return path, ledger, nil
}
func validKubeReceipt(id string) bool {
	return len(id) == 41 && id[:9] == "workload-" && validConnectionToken(id, 41, false)
}

// A stable check binding omits only the new local join timestamp and transient
// original-availability projection. Passing commands/source/approvals stay exact.
func kubeCheckDigest(check ProjectRunCheckEvidence) string {
	check.JoinedAt = ""
	check.OriginalAvailable = false
	return reviewDigest(check)
}

func (s *Server) kubernetesSource(ctx context.Context, actor management.ExecutionActorRef, input KubernetesReviewInput) (SavedDeploymentBlueprint, string, string, string, error) {
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return SavedDeploymentBlueprint{}, "", "", "", err
	}
	s.mu.RLock()
	_, ledger, err := s.readDeploymentBlueprintLedger(p)
	view := blueprintView(actor.ProjectID, ledger)
	contextDigest := ""
	if err == nil && view.Saved != nil {
		contextDigest, err = s.foundationContextLocked(p, view.Saved.Blueprint)
	}
	s.mu.RUnlock()
	if err != nil || view.Saved == nil || view.Saved.Revision != input.BlueprintRevision || view.Saved.Digest != input.BlueprintDigest {
		return SavedDeploymentBlueprint{}, "", "", "", ErrKubernetesReview
	}
	saved := *view.Saved
	for _, service := range saved.Blueprint.Services {
		if !kubeImmutableImage.MatchString(service.Image) {
			return SavedDeploymentBlueprint{}, "", "", "", ErrKubernetesReview
		}
	}
	run, err := s.GetProjectSwarmRun(actor.ProjectID, input.RunID)
	if err != nil || run.ProjectID != actor.ProjectID || len(run.Input.Requirements) == 0 {
		return SavedDeploymentBlueprint{}, "", "", "", ErrKubernetesReview
	}
	c := saved.Blueprint.Source.Context
	implementation := run.Input.Implementation
	if c == nil || implementation.ProjectID != actor.ProjectID || c.BriefRevision != implementation.BriefRevision || c.SpecificationRevision != implementation.SpecificationRevision || c.SpecificationDigest != implementation.SpecificationDigest || c.DesignRevision != implementation.DesignPlanRevision || c.DesignDigest != implementation.DesignPlanDigest {
		return SavedDeploymentBlueprint{}, "", "", "", ErrKubernetesReview
	}
	check, err := s.SnapshotProjectRunCheck(ctx, actor.ProjectID, input.CheckID, run.Input.Approvals, run.Input.Requirements)
	if err != nil || check.Commit != saved.Blueprint.Source.Commit || validProjectRunCheck(run.Input, check) != nil {
		return SavedDeploymentBlueprint{}, "", "", "", ErrKubernetesReview
	}
	// Evidence I/O can wait. Recheck the complete captured task/skills/plan
	// context as well as the blueprint's smaller definition/design reference.
	// A passing old check never makes a changed implementation input current.
	latestRun, err := s.GetProjectSwarmRun(actor.ProjectID, input.RunID)
	if err != nil || reviewDigest(latestRun.Input) != reviewDigest(run.Input) {
		return SavedDeploymentBlueprint{}, "", "", "", ErrKubernetesReview
	}
	s.mu.RLock()
	state, stateErr := s.readApprovedJourney(p.Path, p.ID)
	latestContext, contextErr := s.foundationContextLocked(p, saved.Blueprint)
	fresh := projectRunContextFreshness(latestRun, state, stateErr)
	_, latestLedger, ledgerErr := s.readDeploymentBlueprintLedger(p)
	latestView := blueprintView(actor.ProjectID, latestLedger)
	s.mu.RUnlock()
	if contextErr != nil || latestContext != contextDigest || fresh.Status != "current" || ledgerErr != nil || latestView.Saved == nil || latestView.Saved.Revision != saved.Revision || latestView.Saved.Digest != saved.Digest {
		return SavedDeploymentBlueprint{}, "", "", "", ErrKubernetesReview
	}
	return saved, contextDigest, reviewDigest(run.Input), kubeCheckDigest(check), nil
}

func (s *Server) ReviewKubernetesDeployment(ctx context.Context, actor management.ExecutionActorRef, input KubernetesReviewInput) (KubernetesReview, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.kubernetesAuthority(ctx, actor, false, false, 0); err != nil {
		return KubernetesReview{}, err
	}
	if input.MonthlyBudgetUSD <= 0 || math.IsNaN(input.MonthlyBudgetUSD) || math.IsInf(input.MonthlyBudgetUSD, 0) {
		return KubernetesReview{}, ErrKubernetesReview
	}
	saved, contextDigest, runDigest, checkDigest, err := s.kubernetesSource(ctx, actor, input)
	if err != nil {
		return KubernetesReview{}, err
	}
	plan, err := deploymentplan.Compile(saved.Blueprint)
	if err != nil || plan.State != "compiled-prototype" {
		return KubernetesReview{}, ErrKubernetesReview
	}
	if plan.Blueprint.PublicRoute != nil {
		if !input.AcknowledgePublicRoute || s.kubernetesPublicAuthority(ctx, actor) != nil {
			return KubernetesReview{}, ErrKubernetesReview
		}
	} else if input.AcknowledgePublicRoute {
		return KubernetesReview{}, ErrKubernetesReview
	}
	needsReferences := deploymentplan.RuntimeReferenceCount(plan.Blueprint) > 0
	if err := s.kubernetesAuthority(ctx, actor, false, needsReferences, 0); err != nil {
		return KubernetesReview{}, err
	}
	_, targets, err := s.readKubernetesTargets(actor.ProjectID)
	if err != nil {
		return KubernetesReview{}, err
	}
	var selected KubernetesTargetDescriptor
	for _, target := range targets.Targets {
		if target.ID == input.TargetID {
			selected = target.KubernetesTargetDescriptor
		}
	}
	if selected.ID == "" || selected.Namespace != plan.Blueprint.Infrastructure.Namespace || selected.ID != plan.Blueprint.Infrastructure.Reference {
		return KubernetesReview{}, ErrKubernetesReview
	}
	var id string
	var origin *coordinatorKubernetesOrigin
	if captured, internal := ctx.Value(coordinatorKubernetesContextKey{}).(coordinatorKubernetesOrigin); internal {
		origin = &captured
		id, err = s.coordinatorKubernetesRequestedReview(ctx, actor, input, captured)
		if err == nil {
			s.kubernetesReviewMu.Lock()
			_, existing, readErr := s.readKubernetesReviews(actor.ProjectID)
			s.kubernetesReviewMu.Unlock()
			if readErr != nil {
				return KubernetesReview{}, readErr
			}
			for _, prior := range existing.Reviews {
				if prior.ID == id {
					if reviewDigest(prior.CoordinatorOrigin) != reviewDigest(origin) || !sameCoordinatorKubernetesReviewInput(prior, input) {
						return KubernetesReview{}, ErrKubernetesReview
					}
					if err = s.authorizeKubernetesReview(ctx, prior, false, true); err != nil {
						return KubernetesReview{}, err
					}
					return prior, nil
				}
			}
		}
	} else {
		id, err = kubeOpaqueID("workload-")
	}
	if err != nil {
		return KubernetesReview{}, err
	}
	identity := deploymentkube.Identity{ProjectID: actor.ProjectID, ActorID: actor.UserID, TeamID: actor.TeamID, TargetID: selected.ID, TargetFingerprint: selected.Fingerprint, OperationID: id, PlanDigest: plan.Digest}
	target, err := s.resolveKubernetesTarget(ctx, identity)
	if err != nil {
		return KubernetesReview{}, err
	}
	namespace, err := target.Client.CoreV1().Namespaces().Get(ctx, target.Namespace, metav1.GetOptions{})
	if err != nil || namespace == nil || string(namespace.UID) != selected.NamespaceUID || namespace.DeletionTimestamp != nil {
		return KubernetesReview{}, ErrKubernetesReview
	}
	expected := []deploymentkube.ExpectedResource{}
	for _, artifact := range plan.Artifacts {
		if err := s.kubernetesAuthority(ctx, actor, false, needsReferences, 0); err != nil {
			return KubernetesReview{}, err
		}
		var header struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		}
		if json.Unmarshal([]byte(artifact.Content), &header) != nil {
			return KubernetesReview{}, ErrKubernetesReview
		}
		row := deploymentkube.ExpectedResource{Kind: header.Kind, Name: header.Metadata.Name}
		var object metav1.Object
		if row.Kind == "Deployment" {
			object, err = target.Client.AppsV1().Deployments(target.Namespace).Get(ctx, row.Name, metav1.GetOptions{})
		} else if row.Kind == "ConfigMap" {
			object, err = target.Client.CoreV1().ConfigMaps(target.Namespace).Get(ctx, row.Name, metav1.GetOptions{})
		} else if row.Kind == "Ingress" {
			object, err = target.Client.NetworkingV1().Ingresses(target.Namespace).Get(ctx, row.Name, metav1.GetOptions{})
		} else if row.Kind == "Service" {
			object, err = target.Client.CoreV1().Services(target.Namespace).Get(ctx, row.Name, metav1.GetOptions{})
		} else {
			return KubernetesReview{}, ErrKubernetesReview
		}
		if err == nil && object != nil {
			row.UID = string(object.GetUID())
			row.ResourceVersion = object.GetResourceVersion()
			if row.UID == "" || row.ResourceVersion == "" {
				return KubernetesReview{}, ErrKubernetesReview
			}
		} else if !apierrors.IsNotFound(err) {
			return KubernetesReview{}, ErrKubernetesReview
		}
		expected = append(expected, row)
	}
	importedReferences, err := deploymentkube.ReadReferences(ctx, target, plan.Blueprint, func(ctx context.Context) error { return s.kubernetesAuthority(ctx, actor, false, true, 0) })
	if err != nil {
		return KubernetesReview{}, ErrKubernetesReview
	}
	publicBinding, err := deploymentkube.ReadPublicRouteBinding(ctx, target, plan.Blueprint, func(call context.Context) error { return s.kubernetesAuthority(call, actor, false, true, 0) })
	if err != nil {
		return KubernetesReview{}, ErrKubernetesReview
	}
	now := time.Now().UTC()
	review := KubernetesReview{PublicRouteAcknowledged: input.AcknowledgePublicRoute, ID: id, CoordinatorOrigin: origin, Request: deploymentkube.Request{Identity: identity, Blueprint: plan.Blueprint, ApprovalReference: id, Expected: expected}, Target: selected, BlueprintRevision: saved.Revision, BlueprintDigest: saved.Digest, RunID: input.RunID, CheckID: input.CheckID, RunInputDigest: runDigest, CheckDigest: checkDigest, ContextDigest: contextDigest, MonthlyBudgetUSD: input.MonthlyBudgetUSD, CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339Nano)}
	review.Request.References = importedReferences
	review.Request.PublicRouteBinding = publicBinding
	if err := s.authorizeKubernetesReview(ctx, review, false, true); err != nil {
		return KubernetesReview{}, err
	}
	s.kubernetesReviewMu.Lock()
	defer s.kubernetesReviewMu.Unlock()
	path, ledger, err := s.readKubernetesReviews(actor.ProjectID)
	if err != nil {
		return KubernetesReview{}, err
	}
	for _, prior := range ledger.Reviews {
		if prior.ID == id {
			if origin == nil || reviewDigest(prior.CoordinatorOrigin) != reviewDigest(origin) || !sameCoordinatorKubernetesReviewInput(prior, input) {
				return KubernetesReview{}, ErrKubernetesReview
			}
			if err = s.authorizeKubernetesReview(ctx, prior, false, true); err != nil {
				return KubernetesReview{}, err
			}
			return prior, nil
		}
	}
	kept := []KubernetesReview{}
	// Keep all unresolved originals, but a rolling recent window of positively
	// completed observations. Journal records themselves are never deleted.
	completed := 0
	journal := &kubernetesOperationJournal{server: s}
	for index := len(ledger.Reviews) - 1; index >= 0; index-- {
		row := ledger.Reviews[index]
		expiry, _ := time.Parse(time.RFC3339Nano, row.ExpiresAt)
		if !row.Applied && !time.Now().Before(expiry) {
			continue
		}
		if row.Applied {
			operation, readErr := journal.Load(ctx, row.Request.Identity)
			if readErr == nil && (operation.State == "ready" || operation.State == "blocked") {
				completed++
				if completed > 50 {
					continue
				}
			}
		}
		kept = append([]KubernetesReview{row}, kept...)
	}
	if len(kept) >= 128 {
		return KubernetesReview{}, errors.New("The retained deployment history is full. Uncertain operations have been preserved; no deployment was started.")
	}
	ledger.Reviews = append(kept, review)
	if raw, err := json.MarshalIndent(ledger, "", "  "); err != nil || len(raw)+1 > 32<<20 {
		return KubernetesReview{}, ErrKubernetesReview
	}
	if err := s.authorizeKubernetesReview(ctx, review, false, true); err != nil {
		return KubernetesReview{}, err
	}
	if err := durableRunJSON(path, ledger); err != nil {
		return KubernetesReview{}, err
	}
	if err := s.kubernetesAuthority(ctx, actor, false, needsReferences, 0); err != nil {
		return KubernetesReview{}, err
	}
	return review, nil
}

func (s *Server) authorizeKubernetesReview(ctx context.Context, row KubernetesReview, mutation, source bool) error {
	actor := kubeActor(row.Request.Identity)
	if row.Request.Blueprint.PublicRoute != nil {
		if !row.PublicRouteAcknowledged {
			return ErrKubernetesReview
		}
		if (mutation || source) && s.kubernetesPublicAuthority(ctx, actor) != nil {
			return ErrKubernetesReview
		}
	}
	if err := s.validateCoordinatorKubernetesOrigin(ctx, row, mutation); err != nil {
		return err
	}
	needsReferences := deploymentplan.RuntimeReferenceCount(row.Request.Blueprint) > 0
	if err := s.kubernetesAuthority(ctx, actor, mutation, needsReferences, row.MonthlyBudgetUSD); err != nil {
		return err
	}
	if mutation {
		expiry, err := time.Parse(time.RFC3339Nano, row.ExpiresAt)
		if err != nil || !time.Now().Before(expiry) || !row.Applied {
			return ErrKubernetesReview
		}
	}
	target, err := s.resolveKubernetesTarget(ctx, row.Request.Identity)
	if err != nil || target.NamespaceUID != row.Target.NamespaceUID {
		return ErrKubernetesReview
	}
	if source {
		input := KubernetesReviewInput{TargetID: row.Target.ID, BlueprintRevision: row.BlueprintRevision, BlueprintDigest: row.BlueprintDigest, RunID: row.RunID, CheckID: row.CheckID, MonthlyBudgetUSD: row.MonthlyBudgetUSD}
		saved, contextDigest, runDigest, checkDigest, err := s.kubernetesSource(ctx, actor, input)
		if err != nil || reviewDigest(saved.Blueprint) != reviewDigest(row.Request.Blueprint) || saved.PlanDigest != row.Request.Identity.PlanDigest || contextDigest != row.ContextDigest || runDigest != row.RunInputDigest || checkDigest != row.CheckDigest {
			return ErrKubernetesReview
		}
	}
	return s.kubernetesAuthority(ctx, actor, mutation, needsReferences, row.MonthlyBudgetUSD)
}

func kubeStatus(row KubernetesReview, operation *deploymentkube.Operation) KubernetesDeploymentStatus {
	state := "reviewed"
	if row.Applied {
		state = "unknown"
	}
	if operation != nil {
		state = operation.State
	}
	return KubernetesDeploymentStatus{Review: row, Operation: operation, State: state, ImageProvenance: "user-supplied immutable digest; checked-source-to-image provenance not verified"}
}

func (s *Server) KubernetesDeployments(ctx context.Context, actor management.ExecutionActorRef) ([]KubernetesDeploymentStatus, error) {
	if err := s.kubernetesAuthority(ctx, actor, false, false, 0); err != nil {
		return nil, err
	}
	_, ledger, err := s.readKubernetesReviews(actor.ProjectID)
	if err != nil {
		return nil, err
	}
	rows := []KubernetesDeploymentStatus{}
	needsReferences := false
	journal := &kubernetesOperationJournal{server: s}
	for _, row := range ledger.Reviews {
		if kubeActor(row.Request.Identity) != actor {
			continue
		}
		if deploymentplan.RuntimeReferenceCount(row.Request.Blueprint) > 0 {
			needsReferences = true
			if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
				return nil, management.ErrExecutionAuthority
			}
		}
		status := kubeStatus(row, nil)
		operation, err := journal.Load(ctx, row.Request.Identity)
		if err == nil {
			status = kubeStatus(row, &operation)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, ErrKubernetesReview
		}
		rows = append(rows, status)
	}
	if err := s.kubernetesAuthority(ctx, actor, false, needsReferences, 0); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Server) KubernetesDeploymentAction(ctx context.Context, actor management.ExecutionActorRef, id string, apply, costAcknowledged, imageAcknowledged bool, telemetryAcknowledged ...bool) (KubernetesDeploymentStatus, error) {
	if !validKubeReceipt(id) {
		return KubernetesDeploymentStatus{}, ErrKubernetesReview
	}
	if err := s.kubernetesAuthority(ctx, actor, false, false, 0); err != nil {
		return KubernetesDeploymentStatus{}, err
	}
	s.kubernetesReviewMu.Lock()
	path, ledger, err := s.readKubernetesReviews(actor.ProjectID)
	var selected *KubernetesReview
	if err == nil {
		for index := range ledger.Reviews {
			if ledger.Reviews[index].ID == id && kubeActor(ledger.Reviews[index].Request.Identity) == actor {
				selected = &ledger.Reviews[index]
				break
			}
		}
	}
	if err != nil || selected == nil {
		s.kubernetesReviewMu.Unlock()
		return KubernetesDeploymentStatus{}, ErrKubernetesReview
	}
	row := *selected
	first := apply && !row.Applied
	if first {
		if !costAcknowledged || !imageAcknowledged {
			s.kubernetesReviewMu.Unlock()
			return KubernetesDeploymentStatus{}, ErrKubernetesReview
		}
		for _, service := range row.Request.Blueprint.Services {
			if service.Collector != nil && (len(telemetryAcknowledged) != 1 || !telemetryAcknowledged[0]) {
				s.kubernetesReviewMu.Unlock()
				return KubernetesDeploymentStatus{}, ErrKubernetesReview
			}
		}
		row.Applied = true
		if err = s.authorizeKubernetesReview(ctx, row, true, true); err == nil {
			*selected = row
			err = durableRunJSON(path, ledger)
		}
	}
	s.kubernetesReviewMu.Unlock()
	if err != nil {
		return KubernetesDeploymentStatus{}, err
	}
	connector := s.NewKubernetesDeploymentConnector(s.resolveKubernetesTarget, func(ctx context.Context, request deploymentkube.Request, mutation bool) error {
		if reviewDigest(request) != reviewDigest(row.Request) {
			return ErrKubernetesReview
		}
		return s.authorizeKubernetesReview(ctx, row, mutation, mutation)
	})
	var operation deploymentkube.Operation
	if first {
		operation, err = connector.Apply(ctx, row.Request)
	} else {
		operation, err = connector.Observe(ctx, row.Request.Identity)
	}
	// The attempted admission and any provider outcome stay retained even when
	// the client disconnects or this response is withheld. There is no replay.
	if s.kubernetesAuthority(ctx, actor, false, deploymentplan.RuntimeReferenceCount(row.Request.Blueprint) > 0, 0) != nil {
		return KubernetesDeploymentStatus{}, management.ErrExecutionAuthority
	}
	if err != nil {
		if errors.Is(err, deploymentkube.ErrUnknown) {
			return kubeStatus(row, &operation), nil
		}
		return KubernetesDeploymentStatus{}, ErrKubernetesReview
	}
	return kubeStatus(row, &operation), nil
}
