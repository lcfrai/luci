package product

import (
	"context"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// This receipt records source publication, not provider deployment or user
// behavior acceptance. Raw credential/Git output never enters project history.
type CoordinatorGitAction struct {
	ActionID   string `json:"actionId"`
	Status     string `json:"status"`
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Commit     string `json:"commit"`
	StartedAt  string `json:"startedAt"`
	UpdatedAt  string `json:"updatedAt"`
	Message    string `json:"message"`
}

func validateCoordinatorGitPublication(row coordinatorContinuationRecord) error {
	if row.Delivery == nil || row.Delivery.GitPublication == nil {
		return nil
	}
	a, plan := row.Delivery.GitPublication, row.DeploymentPlan
	if plan == nil || !plan.AcknowledgeRepositoryPush || row.Delivery.Status != "verified" || a.ActionID != row.ID+"-git" || a.Repository != plan.Repository || a.Branch != plan.Branch || a.Commit != row.Delivery.Commit || !deliverySHA.MatchString(a.Commit) || len(a.Message) > 4000 {
		return ErrOrchestratorInvalid
	}
	if address, err := safeDeliveryURL(a.Repository); err != nil || address != a.Repository || !deliveryBranch.MatchString(a.Branch) {
		return ErrOrchestratorInvalid
	}
	if _, err := time.Parse(time.RFC3339Nano, a.StartedAt); err != nil {
		return ErrOrchestratorInvalid
	}
	if _, err := time.Parse(time.RFC3339Nano, a.UpdatedAt); err != nil {
		return ErrOrchestratorInvalid
	}
	switch a.Status {
	case "pending", "unknown", "verified", "blocked":
	default:
		return ErrOrchestratorInvalid
	}
	return nil
}

func coordinatorGitPublished(row coordinatorContinuationRecord) bool {
	return row.Delivery != nil && row.Delivery.GitPublication != nil && row.Delivery.GitPublication.Status == "verified" && validateCoordinatorGitPublication(row) == nil
}

// Caller owns the continuation's action lock and source admission. Only the
// invocation creating the durable reservation may push. Restart/lost ACK paths
// perform an exact read only; they cannot replay even a supposedly safe push.
func (s *Server) continueCoordinatorGitPublication(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (coordinatorContinuationRecord, error) {
	if row.DeploymentPlan == nil || !row.DeploymentPlan.AcknowledgeRepositoryPush {
		return row, errCoordinatorPolicy
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	bounded, err := s.coordinatorDeploymentAuthority(bounded, p, row, true)
	if err != nil {
		return row, err
	}
	if !s.reserveDelivery(p.ID) {
		return row, errCoordinatorSourceBusy
	}
	defer s.releaseDelivery(p.ID)
	plan := row.DeploymentPlan
	// Origin and current branch are reviewed destinations, not repository-provided
	// pushurl, default refspecs, hooks, URL rewrites or a model-supplied remote.
	origin, err := s.rawOrigin(bounded, p.Path)
	if err != nil || origin != plan.Repository {
		return row, ErrOrchestratorConflict
	}
	branch, err := s.journeyGit(bounded, p.Path, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || strings.TrimSpace(branch) != plan.Branch {
		return row, ErrOrchestratorConflict
	}
	if _, err = s.journeyGit(bounded, p.Path, "check-ref-format", "refs/heads/"+plan.Branch); err != nil {
		return row, ErrOrchestratorConflict
	}
	invoke := row.Delivery.GitPublication == nil
	if invoke {
		copy := *row.Delivery
		row.Delivery = &copy
		now := time.Now().UTC().Format(time.RFC3339Nano)
		copy.GitPublication = &CoordinatorGitAction{ActionID: row.ID + "-git", Status: "pending", Repository: plan.Repository, Branch: plan.Branch, Commit: copy.Commit, StartedAt: now, UpdatedAt: now}
		row, err = s.persistCoordinatorGitPublication(bounded, p, row, "pending", "The exact reviewed repository, branch and commit are reserved before source publication.")
		if err != nil {
			return row, err
		}
	} else if err = validateCoordinatorGitPublication(row); err != nil {
		return row, err
	}
	if row.Delivery.GitPublication.Status == "blocked" {
		return row, nil
	}
	// Even a previous verified receipt needs current remote equality before a new
	// deployment is admitted. A moved branch is not repaired by an implicit push.
	if invoke {
		bounded, err = s.coordinatorDeploymentAuthority(bounded, p, row, true)
		if err != nil {
			return row, err
		}
		if _, err = s.deliveryNetworkGit(bounded, p.Path, plan.Repository, "push", "--porcelain", "--no-force", "--no-mirror", "--no-follow-tags", "--recurse-submodules=no", plan.Repository, row.Delivery.Commit+":refs/heads/"+plan.Branch); err != nil {
			// Exit errors/timeouts do not prove rejection. Keep exact pending ownership;
			// a fresh observation can establish success but never retries this effect.
			return s.persistCoordinatorGitPublication(bounded, p, row, "unknown", "The exact source push is unconfirmed. Its outcome will be observed without replaying it; provider deployment has not started.")
		}
	}
	bounded, err = s.coordinatorDeploymentAuthority(bounded, p, row, false)
	if err != nil {
		return row, err
	}
	output, observeErr := s.deliveryNetworkGit(bounded, p.Path, plan.Repository, "ls-remote", "--heads", plan.Repository, "refs/heads/"+plan.Branch)
	if observeErr != nil || strings.TrimSpace(output) != row.Delivery.Commit+"\trefs/heads/"+plan.Branch {
		return s.persistCoordinatorGitPublication(bounded, p, row, "unknown", "The destination does not currently confirm the exact reviewed commit. No source push is replayed and provider deployment remains blocked.")
	}
	return s.persistCoordinatorGitPublication(bounded, p, row, "verified", "The configured remote branch currently names the exact reviewed commit. Provider deployment and application behavior verification remain separate.")
}

func (s *Server) persistCoordinatorGitPublication(ctx context.Context, p project.Project, row coordinatorContinuationRecord, status, message string) (coordinatorContinuationRecord, error) {
	if _, err := s.coordinatorDeploymentAuthority(ctx, p, row, false); err != nil {
		return row, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readContinuations(p, row.Actor.ID)
	index := findContinuation(rows, row.ID)
	if err != nil || index < 0 {
		return row, ErrOrchestratorInvalid
	}
	current := rows.Rows[index]
	if current.Revision != row.Revision {
		return current, ErrOrchestratorConflict
	}
	if err = s.coordinatorDeploymentAuthorityLocked(ctx, p, current); err != nil {
		return current, err
	}
	if row.Delivery == nil || row.Delivery.GitPublication == nil {
		return current, ErrOrchestratorInvalid
	}
	d := *row.Delivery
	a := *d.GitPublication
	d.GitPublication = &a
	row.Delivery = &d
	a.Status, a.Message = status, message
	top := "unknown"
	if status == "pending" {
		top = "continuing"
	}
	if status == "verified" {
		top = "preview-ready"
	}
	if current.Status == top && current.Message == message && reviewDigest(current.Delivery) == reviewDigest(row.Delivery) {
		return current, nil
	}
	a.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	row.Status, row.Message = top, message
	row.UpdatedAt = a.UpdatedAt
	row.Revision++
	if err = validateCoordinatorGitPublication(row); err != nil {
		return current, err
	}
	rows.Rows[index] = row
	return row, s.saveContinuations(rows)
}
