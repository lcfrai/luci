package product

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// A verified check is retained in the run, so pruning ordinary check history
// cannot substitute another result or strand an already linked candidate.
func (s *Server) continueCoordinatorPreview(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (coordinatorContinuationRecord, error) {
	if row.PreviewPlan == nil {
		return row, nil
	}
	s.mu.Lock()
	policy, err := s.readCoordinatorPolicy(p)
	if err == nil && !coordinatorPreviewPolicyMatches(policy.PreviewMode, row.PreviewPlan) && row.Delivery.Preview == nil {
		s.mu.Unlock()
		return row, nil
	}
	if err == nil {
		err = s.coordinatorPreviewAuthorityLocked(ctx, p, row)
	}
	s.mu.Unlock()
	if err != nil {
		return row, err
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil {
		return row, err
	}
	check, err := coordinatorPreviewCheck(run, row)
	if err != nil {
		return row, err
	}
	if err = s.validateProjectRunApprovals(p.ID, run.Input.Approvals); err != nil {
		return row, err
	}
	if err = s.validateProjectRunCandidate(ctx, p.ID, check); err != nil {
		return row, err
	}
	runtime := s.PreviewRuntime()
	if runtime.CaptureActor == nil || runtime.Start == nil || runtime.Observe == nil || runtime.Stop == nil {
		return row, errCoordinatorAuthority
	}
	if _, err = runtime.CaptureActor(ctx, p.ID); err != nil {
		return row, errCoordinatorAuthority
	}
	if row.Delivery.Preview == nil {
		delivery := *row.Delivery
		row.Delivery = &delivery
		now := coordinatorPreviewNow()
		row.Delivery.Preview = &CoordinatorPreviewAction{ActionID: row.ID + "-preview", ReleaseID: coordinatorPreviewID(row.ID), Status: "pending", StartedAt: now, UpdatedAt: now}
		row, err = s.persistCoordinatorPreview(ctx, p, row, "pending", "The exact local preview identity, checked source and configuration are reserved before building.", nil)
		if err != nil {
			return row, err
		}
		// Only the call which durably created this identity may invoke it. A
		// later controller observes the identity even if no release was saved.
		s.startCoordinatorPreview(ctx, p, row)
		return row, nil
	}
	s.mu.RLock()
	releases, err := s.readReleases(p.ID)
	live := s.coordinatorActions[row.Delivery.Preview.ActionID] || s.activeLocalReleases[row.Delivery.Preview.ReleaseID]
	s.mu.RUnlock()
	if err != nil {
		return row, err
	}
	var exact *storedRelease
	for _, saved := range releases {
		if saved.ID == row.Delivery.Preview.ReleaseID {
			copy := saved
			exact = &copy
			break
		}
	}
	if exact == nil {
		if live {
			return row, nil
		}
		if row.Delivery.Preview.Status == "blocked" {
			return row, nil
		}
		return s.persistCoordinatorPreview(ctx, p, row, "unknown", "The reserved preview has no observable release. Its build will not be replayed; inspect the interrupted operation before authorizing new work.", nil)
	}
	if exact.CoordinatorOriginDigest != coordinatorPreviewBinding(row) || !previewReleaseKindMatches(*exact, row.PreviewPlan.Kind) || exact.ProjectID != p.ID || exact.Target != "local" || exact.Commit != check.Commit || exact.ConfigurationRevision != row.PreviewPlan.ExpectedConfigurationRevision || exact.Runtime == nil || exact.Runtime.Identity.ProjectID != p.ID || exact.Runtime.Identity.ReleaseID != exact.ID || exact.Runtime.Identity.Target != "local" || exact.Runtime.Identity.Commit != check.Commit || exact.Runtime.Identity.SourceDigest != check.SourceDigest {
		return row, ErrOrchestratorConflict
	}
	if exact.CancelRequested || exact.Status == "cancelled" || exact.Status == "stopped" || exact.Status == "failed" {
		return s.persistCoordinatorPreview(ctx, p, row, "blocked", "The exact local preview was cancelled, stopped or failed. Its release and any retained allocation remain available for inspection.", nil)
	}
	if live {
		return s.persistCoordinatorPreview(ctx, p, row, "running", "The reserved local preview is building. Its admission and running container do not yet establish fresh owned HTTP health.", nil)
	}
	evidence, err := s.SnapshotProjectRunRelease(ctx, p.ID, exact.ID, "local", "preview", run.Input.Approvals, check)
	if err != nil {
		return s.persistCoordinatorPreview(ctx, p, row, "unknown", "The exact local preview has not established current owned HTTP health. Its receipt is retained and no build is replayed.", nil)
	}
	if _, err = runtime.CaptureActor(ctx, p.ID); err != nil {
		return row, errCoordinatorAuthority
	}
	s.mu.Lock()
	err = s.coordinatorPreviewAuthorityLocked(ctx, p, row)
	if err == nil {
		currentReleases, readErr := s.readReleases(p.ID)
		err = readErr
		matched := false
		for _, current := range currentReleases {
			if current.ID == exact.ID && current.Revision == exact.Revision && current.CoordinatorOriginDigest == exact.CoordinatorOriginDigest && previewReleaseKindMatches(current, row.PreviewPlan.Kind) && !current.CancelRequested && current.Runtime != nil && current.Runtime.Identity == exact.Runtime.Identity {
				matched = true
			}
		}
		if err == nil && !matched {
			err = ErrOrchestratorConflict
		}
	}
	if err == nil {
		var current ProjectRun
		current, err = s.projectRuns.Get(p.ID, p.Path, row.RunID)
		if err == nil {
			_, err = coordinatorPreviewCheck(current, row)
			// Publishing requires a public release. Keep this local observation
			// as an intermediate receipt without changing the intended outcome.
			if err == nil && current.Input.Outcome == "preview" {
				_, err = s.projectRuns.JoinRelease(p.ID, p.Path, row.RunID, current.Revision, evidence)
			}
		}
	}
	s.mu.Unlock()
	if err != nil {
		return row, err
	}
	row, err = s.persistCoordinatorPreview(ctx, p, row, "ready", "The exact checked local preview passed fresh ownership and HTTP health observation. Public deployment and full behavior acceptance remain separate.", &evidence)
	if err != nil {
		return row, err
	}
	return s.continueCoordinatorDeployment(ctx, p, row)
}

func (s *Server) startCoordinatorPreview(ctx context.Context, p project.Project, row coordinatorContinuationRecord) {
	action := row.Delivery.Preview.ActionID
	s.mu.Lock()
	if s.coordinatorActions == nil {
		s.coordinatorActions = map[string]bool{}
	}
	s.coordinatorActions[action] = true
	s.mu.Unlock()
	// Browser disconnects do not revoke a durable delegation. Existing runtime
	// authorizers and source/policy callbacks still revalidate each new effect.
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
	stop := context.AfterFunc(s.workspaceContext, cancel)
	go func() {
		defer cancel()
		defer stop()
		err := s.invokeCoordinatorPreview(bounded, p, row)
		s.mu.Lock()
		delete(s.coordinatorActions, action)
		s.mu.Unlock()
		var refused *coordinatorStepError
		if errors.As(err, &refused) && refused.Status >= 400 && refused.Status < 500 {
			// A known pre-admission refusal is useful evidence, but never erase
			// an admitted release whose runtime may still own resources.
			s.mu.RLock()
			releases, readErr := s.readReleases(p.ID)
			s.mu.RUnlock()
			found := false
			for _, release := range releases {
				found = found || release.ID == row.Delivery.Preview.ReleaseID
			}
			if readErr == nil && !found {
				_, _ = s.persistCoordinatorPreview(bounded, p, row, "blocked", "The reserved local preview was refused before admission. Review its source, authority and saved configuration; no automatic replay is permitted.", nil)
			}
		}
		s.WakeCoordinator()
	}()
}

func (s *Server) persistCoordinatorPreview(ctx context.Context, p project.Project, row coordinatorContinuationRecord, status, message string, evidence *ProjectRunReleaseEvidence) (coordinatorContinuationRecord, error) {
	runtime := s.PreviewRuntime()
	if runtime.CaptureActor == nil {
		return row, errCoordinatorAuthority
	}
	if _, err := runtime.CaptureActor(ctx, p.ID); err != nil {
		return row, errCoordinatorAuthority
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
	if err = s.coordinatorPreviewAuthorityLocked(ctx, p, current); err != nil {
		return current, err
	}
	if row.Delivery == nil || row.Delivery.Status != "verified" || row.Delivery.Preview == nil {
		return current, ErrOrchestratorInvalid
	}
	delivery := *row.Delivery
	preview := *delivery.Preview
	delivery.Preview, row.Delivery = &preview, &delivery
	preview.Status, preview.Message = status, message
	if evidence != nil {
		copy := *evidence
		preview.Evidence = &copy
	}
	top := "unknown"
	switch status {
	case "pending", "running":
		top = "previewing"
	case "ready":
		top = "preview-ready"
	case "blocked":
		top = "preview-blocked"
	}
	if current.Status == top && current.Message == message && reviewDigest(current.Delivery) == reviewDigest(row.Delivery) {
		return current, nil
	}
	preview.UpdatedAt = coordinatorPreviewNow()
	row.Status, row.Message = top, message
	row.Revision++
	row.UpdatedAt = preview.UpdatedAt
	if err = validateCoordinatorPreview(row); err != nil {
		return current, err
	}
	rows.Rows[index] = row
	return row, s.saveContinuations(rows)
}
