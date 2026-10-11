package product

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func (s *Server) continueCoordinatorDeployment(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (coordinatorContinuationRecord, error) {
	if row.DeploymentPlan == nil {
		return row, nil
	}
	s.mu.Lock()
	policy, err := s.readCoordinatorPolicy(p)
	if err == nil && policy.DeploymentMode != "existing-target" && row.Delivery.Deployment == nil {
		s.mu.Unlock()
		return row, nil
	}
	if err == nil {
		err = s.coordinatorDeploymentAuthorityLocked(ctx, p, row)
	}
	s.mu.Unlock()
	if err != nil {
		return row, err
	}
	current, err := s.authorizeCoordinatorDeployment(ctx, p, row, row.Delivery.Deployment == nil && row.Delivery.GitPublication == nil)
	if err != nil {
		return row, err
	}
	run, check, err := s.validateCoordinatorDeploymentCandidate(current, p, row)
	if err != nil {
		return row, err
	}
	if row.Delivery.Deployment == nil {
		row, err = s.continueCoordinatorGitPublication(current, p, row)
		if err != nil || !coordinatorGitPublished(row) {
			return row, err
		}
		// Git network observation may block. Recheck the exact current candidate
		// and fresh deployment authority before reserving any provider action.
		current, err = s.coordinatorDeploymentAuthority(current, p, row, true)
		if err != nil {
			return row, err
		}
		delivery := *row.Delivery
		row.Delivery = &delivery
		now := coordinatorPreviewNow()
		row.Delivery.Deployment = &CoordinatorDeploymentAction{ActionID: row.ID + "-deploy", ReleaseID: coordinatorDeploymentID(row.ID), Status: "pending", StartedAt: now, UpdatedAt: now}
		row, err = s.persistCoordinatorDeployment(current, p, row, "pending", "The exact public release, existing target and acknowledged deployment effects are reserved before any provider mutation.", nil)
		if err != nil {
			return row, err
		}
		s.startCoordinatorDeployment(current, p, row)
		return row, nil
	}
	s.mu.RLock()
	releases, err := s.readCloudReleases(p.ID)
	live := s.coordinatorActions[row.Delivery.Deployment.ActionID]
	s.mu.RUnlock()
	if err != nil {
		return row, err
	}
	var exact *CloudRelease
	for _, saved := range releases {
		if saved.ID == row.Delivery.Deployment.ReleaseID {
			copy := saved
			exact = &copy
			break
		}
	}
	if exact == nil {
		if live || row.Delivery.Deployment.Status == "blocked" {
			return row, nil
		}
		return s.persistCoordinatorDeployment(current, p, row, "unknown", "The reserved deployment has no observable release. No provider request is replayed; inspect the interrupted operation before authorizing new work.", nil)
	}
	plan := row.DeploymentPlan
	if exact.CoordinatorOriginDigest != coordinatorDeploymentBinding(row) || exact.ProjectID != p.ID || exact.CloudReleaseRequest != coordinatorDeploymentRequest(plan, check.Commit) || exact.Release.Commit != check.Commit || exact.Target != plan.TargetID || exact.ConfigurationRevision != plan.ExpectedConfigurationRevision || exact.DataDomainRevision != plan.ExpectedDataDomainRevision || exact.DataBindingFingerprint != plan.ExpectedDataBindingFingerprint || exact.TargetFingerprint != plan.TargetFingerprint || exact.ServerUUID != plan.ServerUUID {
		return row, ErrOrchestratorConflict
	}
	switch exact.Status {
	case "cancelled", "stopped", "stopping", "failed":
		return s.persistCoordinatorDeployment(current, p, row, "blocked", "The exact deployment was stopped, cancelled or failed. Inspect its retained provider identifiers and billing before authorizing another attempt.", nil)
	}
	if live {
		return s.persistCoordinatorDeployment(current, p, row, "running", "The existing target is processing the reserved deployment. Admission and a queued provider job do not establish public availability.", nil)
	}
	if exact.DeploymentID == "" || exact.ApplicationID == "" {
		return s.persistCoordinatorDeployment(current, p, row, "unknown", "The provider has not confirmed the exact application and deployment identifiers. Retained resources may still exist; no create or deploy request is replayed.", nil)
	}
	evidence, err := s.SnapshotProjectRunRelease(current, p.ID, exact.ID, plan.TargetID, "publish", run.Input.Approvals, check)
	if err != nil {
		return s.persistCoordinatorDeployment(current, p, row, "unknown", "The exact public release has not established current application ownership, the checked commit and successful HTTPS health. Its receipt is retained without replay.", nil)
	}
	current, err = s.authorizeCoordinatorDeployment(current, p, row, false)
	if err != nil {
		return row, err
	}
	s.mu.Lock()
	err = s.coordinatorDeploymentAuthorityLocked(current, p, row)
	if err == nil {
		currentReleases, readErr := s.readCloudReleases(p.ID)
		err = readErr
		matched := false
		for i, saved := range currentReleases {
			if saved.ID == exact.ID && reviewDigest(saved) == reviewDigest(*exact) {
				matched = true
				// This authorized continuation publishes a freshly verified
				// observation using an exact receipt CAS. GET/Snapshot remain
				// pure, and a later amendment can reuse the owned application.
				if saved.Status != "ready" || saved.URL != evidence.URL {
					saved.Status, saved.URL = "ready", evidence.URL
					saved.Message = "The provider confirmed this exact checked commit and its public HTTPS health."
					currentReleases[i] = saved
					err = atomicJSON(s.cloudReleaseFile(p.ID), currentReleases)
				}
			}
		}
		if err == nil && !matched {
			err = ErrOrchestratorConflict
		}
	}
	if err == nil {
		var currentRun ProjectRun
		currentRun, err = s.projectRuns.Get(p.ID, p.Path, row.RunID)
		if err == nil {
			_, err = coordinatorPreviewCheck(currentRun, row)
		}
		if err == nil {
			_, err = s.projectRuns.JoinRelease(p.ID, p.Path, row.RunID, currentRun.Revision, evidence)
		}
	}
	s.mu.Unlock()
	if err != nil {
		return row, err
	}
	return s.persistCoordinatorDeployment(current, p, row, "ready", "The exact public release passed current provider ownership, checked-commit and HTTPS health observation. This confirms deployment availability; full application behavior acceptance remains separate.", &evidence)
}

func (s *Server) startCoordinatorDeployment(ctx context.Context, p project.Project, row coordinatorContinuationRecord) {
	action := row.Delivery.Deployment.ActionID
	s.mu.Lock()
	if s.coordinatorActions == nil {
		s.coordinatorActions = map[string]bool{}
	}
	s.coordinatorActions[action] = true
	s.mu.Unlock()
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
	stop := context.AfterFunc(s.workspaceContext, cancel)
	go func() {
		defer cancel()
		defer stop()
		err := s.invokeCoordinatorDeployment(bounded, p, row)
		s.mu.Lock()
		delete(s.coordinatorActions, action)
		s.mu.Unlock()
		var refused *coordinatorStepError
		if errors.As(err, &refused) && refused.Status >= 400 && refused.Status < 500 {
			s.mu.RLock()
			releases, readErr := s.readCloudReleases(p.ID)
			s.mu.RUnlock()
			found := false
			for _, release := range releases {
				found = found || release.ID == row.Delivery.Deployment.ReleaseID
			}
			if readErr == nil && !found {
				_, _ = s.persistCoordinatorDeployment(bounded, p, row, "blocked", "The reserved deployment was refused before provider admission. Review current target, source, cost policy and Deploy access; no automatic replay is permitted.", nil)
			}
		}
		s.WakeCoordinator()
	}()
}

func (s *Server) persistCoordinatorDeployment(ctx context.Context, p project.Project, row coordinatorContinuationRecord, status, message string, evidence *ProjectRunReleaseEvidence) (coordinatorContinuationRecord, error) {
	currentContext, err := s.authorizeCoordinatorDeployment(ctx, p, row, false)
	if err != nil {
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
	if err = s.coordinatorDeploymentAuthorityLocked(currentContext, p, current); err != nil {
		return current, err
	}
	if row.Delivery == nil || row.Delivery.Deployment == nil {
		return current, ErrOrchestratorInvalid
	}
	delivery := *row.Delivery
	deployment := *delivery.Deployment
	delivery.Deployment, row.Delivery = &deployment, &delivery
	deployment.Status, deployment.Message = status, message
	if evidence != nil {
		copy := *evidence
		deployment.Evidence = &copy
	}
	top := "unknown"
	switch status {
	case "pending", "running":
		top = "deploying"
	case "ready":
		top = "deployed"
	case "blocked":
		top = "deployment-blocked"
	}
	if current.Status == top && current.Message == message && reviewDigest(current.Delivery) == reviewDigest(row.Delivery) {
		return current, nil
	}
	deployment.UpdatedAt = coordinatorPreviewNow()
	row.Status, row.Message = top, message
	row.Revision++
	row.UpdatedAt = deployment.UpdatedAt
	if err = validateCoordinatorDeployment(row); err != nil {
		return current, err
	}
	rows.Rows[index] = row
	return row, s.saveContinuations(rows)
}
