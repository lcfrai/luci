package product

import (
	"context"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	appsv1 "k8s.io/api/apps/v1"
)

func publicHTTPSActionKind(v string) bool {
	switch v {
	case "apply", "observe", "pause", "resume", "retire":
		return true
	}
	return false
}
func publicHTTPSReceipt(row *storedPublicHTTPS, key string) *PublicHTTPSActionReceipt {
	for i := range row.Actions {
		if row.Actions[i].RequestKey == key {
			return &row.Actions[i]
		}
	}
	return nil
}
func publicHTTPSTrimActions(row *storedPublicHTTPS) bool {
	for len(row.Actions) >= 32 {
		removed := false
		for i, r := range row.Actions {
			if r.State == "settled" {
				row.Actions = append(row.Actions[:i], row.Actions[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			return false
		}
	}
	return true
}

// Original receipt lookup precedes revision admission. Repeating an exact key
// can observe its original outcome; it never reacquires a lost effect permit.
func (s *Server) beginPublicHTTPSAction(ctx context.Context, p project.Project, actor management.ExecutionActorRef, id string, input PublicHTTPSActionInput) (storedPublicHTTPS, bool, error) {
	s.publicHTTPSMu.Lock()
	defer s.publicHTTPSMu.Unlock()
	recordPath, row, e := s.readPublicHTTPS(p, id)
	if e != nil || row.Actor != actor {
		return row, false, ErrPublicHTTPS
	}
	digest := reviewDigest(input)
	if original := publicHTTPSReceipt(&row, input.RequestKey); original != nil {
		if original.RequestDigest != digest || original.Action != input.Action || original.StartedRevision != input.ExpectedRevision {
			return row, false, ErrPublicHTTPS
		}
		return row, false, nil
	}
	if row.Revision != input.ExpectedRevision || s.publicHTTPSAuthority(ctx, actor, input.Action == "apply" || input.Action == "resume", row.Plan.MonthlyBudgetUSD) != nil {
		return row, false, ErrPublicHTTPS
	}
	if !publicHTTPSTrimActions(&row) {
		return row, false, ErrPublicHTTPSCapacity
	}
	first := true
	if input.Action == "pause" || input.Action == "resume" {
		for _, prior := range row.Actions {
			if (prior.Action == "pause" || prior.Action == "resume") && prior.State != "settled" {
				return row, false, ErrPublicHTTPS
			}
		}
	}
	switch input.Action {
	case "apply":
		if row.RetireAttempted {
			return row, false, ErrPublicHTTPS
		}
		if row.ApplyAttempted {
			first = publicHTTPSHasUnattempted(row)
		} else {
			expiry, _ := time.Parse(time.RFC3339Nano, row.Plan.ExpiresAt)
			if !time.Now().Before(expiry) {
				return row, false, ErrPublicHTTPS
			}
			if _, e = s.publicHTTPSJob(row); e != nil {
				return row, false, e
			}
			if e = s.publicHTTPSActivateLocked(p, row); e != nil {
				return row, false, e
			}
			row.ApplyAttempted = true
			row.DesiredReplicas = 1
			publicHTTPSUnready(&row, "provisioning", "Original HTTPS creation admitted. Each named resource has its own durable attempted marker; a crash never creates replacements.")
		}
	case "pause":
		item := publicHTTPSResource(&row, "Deployment")
		if !row.ApplyAttempted || row.RetireAttempted || item == nil || item.UID == "" {
			return row, false, ErrPublicHTTPS
		}
		row.DesiredReplicas = 0
		row.PauseAttempted = true
		row.ScaleVersion = ""
		publicHTTPSUnready(&row, "provisioning", "Original controller pause admitted; load balancer, DNS and certificate data are retained and can continue costing money.")
	case "resume":
		item := publicHTTPSResource(&row, "Deployment")
		if !row.ApplyAttempted || row.RetireAttempted || item == nil || item.UID == "" {
			return row, false, ErrPublicHTTPS
		}
		row.DesiredReplicas = 1
		row.ResumeAttempted = true
		row.ScaleVersion = ""
		publicHTTPSUnready(&row, "provisioning", "Original controller resume admitted under current public/cost authority. No controller or certificate-state replacement is created.")
	case "retire":
		if !row.ApplyAttempted {
			publicHTTPSUnready(&row, "retired", "Unused review retired. No resources were admitted.")
			row.PVCDataRetained = false
			row.DNSCredential = PublicHTTPSDNSCredential{}
			first = false
		} else if row.State == "retired" {
			first = false
		} else {
			row.RetireAttempted = true
			publicHTTPSUnready(&row, "retiring", "Original endpoint retirement admitted; only retained positive UIDs and the exact acknowledged conditional DNS record can be removed. Certificate PVC is retained.")
		}
	}
	row.Actions = append(row.Actions, PublicHTTPSActionReceipt{RequestKey: input.RequestKey, RequestDigest: digest, Action: input.Action, State: "attempted", StartedRevision: input.ExpectedRevision})
	row.LastAction, row.LastActionRequestKey = input.Action, input.RequestKey
	row.RequestKey, row.RequestDigest = input.RequestKey, digest
	row.Revision++
	if !validStoredPublicHTTPS(row, p) || publicHTTPSWrite(recordPath, row, 128<<10) != nil {
		return row, false, ErrPublicHTTPS
	}
	return row, first, nil
}
func (s *Server) settlePublicHTTPSAction(p project.Project, row storedPublicHTTPS, key string, settled bool) (storedPublicHTTPS, error) {
	return s.publicHTTPSMutate(p, row.ID, row.Actor, func(r *storedPublicHTTPS) error {
		receipt := publicHTTPSReceipt(r, key)
		if receipt == nil {
			return ErrPublicHTTPS
		}
		if settled {
			receipt.State = "settled"
			receipt.SettledRevision = r.Revision + 1
		} else if receipt.State != "settled" {
			receipt.State = "unknown"
		}
		return nil
	})
}

// Reconcile the retained ORIGINAL keys, even when a later Observe became the
// latest action. A marker/positive UID is evidence of state, not a lost response.
func (s *Server) reconcilePublicHTTPSReceipts(ctx context.Context, p project.Project, row storedPublicHTTPS) (storedPublicHTTPS, error) {
	for _, receipt := range append([]PublicHTTPSActionReceipt(nil), row.Actions...) {
		if receipt.State == "settled" || receipt.Action == "observe" {
			continue
		}
		settled := false
		switch receipt.Action {
		case "apply":
			settled = true
			for _, item := range row.Resources {
				if !item.Attempted || item.UID == "" || item.State == "unknown" || item.State == "planned" {
					settled = false
				}
			}
		case "retire":
			settled = row.State == "retired"
		case "pause", "resume":
			if row.State == "retired" {
				settled = true
				break
			}
			target, e := s.publicHTTPSTarget(ctx, row)
			if e != nil {
				return row, e
			}
			owned := publicHTTPSResource(&row, "Deployment")
			if owned == nil || owned.UID == "" {
				continue
			}
			replicas := int32(0)
			if receipt.Action == "resume" {
				replicas = 1
			}
			var actual appsv1.Deployment
			if s.publicHTTPSRPC(ctx, target, "GET", "Deployment", owned.Name, nil, &actual) != nil || !deploymentkube.PublicHTTPSOwned(publicHTTPSSpec(row), &actual, owned.UID) {
				return row, ErrPublicHTTPS
			}
			marker := reviewDigest(struct {
				ID, Key  string
				Replicas int32
			}{row.ID, receipt.RequestKey, replicas})
			settled = actual.Spec.Replicas != nil && *actual.Spec.Replicas == replicas && actual.Annotations["luci.ai/https-action"] == marker
		}
		if settled {
			var e error
			row, e = s.settlePublicHTTPSAction(p, row, receipt.RequestKey, true)
			if e != nil {
				return row, e
			}
		}
	}
	return row, nil
}
func (s *Server) PublicHTTPSAction(ctx context.Context, actor management.ExecutionActorRef, id string, input PublicHTTPSActionInput) (PublicHTTPSStatus, error) {
	ctx, cancel := context.WithTimeout(publicHTTPSRequestContext(ctx), 90*time.Second)
	defer cancel()
	if !s.publicHTTPSWorkMu.TryLock() {
		return PublicHTTPSStatus{}, ErrPublicHTTPS
	}
	defer s.publicHTTPSWorkMu.Unlock()
	if !publicHTTPSActionKind(input.Action) || !publicHTTPSKey.MatchString(input.RequestKey) || input.ExpectedRevision < 1 || s.publicHTTPSAuthority(ctx, actor, false, 0) != nil {
		return PublicHTTPSStatus{}, management.ErrExecutionAuthority
	}
	p, e := s.resolveProject(actor.ProjectID)
	if e != nil {
		return PublicHTTPSStatus{}, e
	}
	_, existing, e := s.readPublicHTTPS(p, id)
	if e != nil || existing.Actor != actor {
		return PublicHTTPSStatus{}, ErrPublicHTTPS
	}
	if publicHTTPSReceipt(&existing, input.RequestKey) == nil {
		if input.Action == "apply" && !existing.ApplyAttempted {
			if e = s.publicHTTPSGuard(ctx, existing, true, true); e != nil {
				return PublicHTTPSStatus{}, e
			}
		}
		if input.Action == "resume" {
			if e = s.publicHTTPSAuthority(ctx, actor, true, existing.Plan.MonthlyBudgetUSD); e != nil {
				return PublicHTTPSStatus{}, e
			}
			if e = s.publicHTTPSDestinationCurrent(ctx, p, existing); e != nil {
				return PublicHTTPSStatus{}, e
			}
		}
	}
	row, first, e := s.beginPublicHTTPSAction(ctx, p, actor, id, input)
	if e != nil {
		return PublicHTTPSStatus{}, e
	}
	var workErr error
	if first {
		switch input.Action {
		case "apply":
			workErr = s.createPublicHTTPSObjects(ctx, p, row)
		case "pause", "resume":
			workErr = s.scalePublicHTTPS(ctx, p, row, input.RequestKey)
		case "retire":
			if row.ApplyAttempted {
				workErr = s.retirePublicHTTPS(ctx, p, row)
			}
		}
	}
	_, row, e = s.readPublicHTTPS(p, id)
	if e != nil {
		return PublicHTTPSStatus{}, e
	}
	if workErr == nil && row.ApplyAttempted && row.State != "retired" {
		row, workErr = s.observePublicHTTPS(ctx, p, row)
	}
	if workErr != nil {
		row, _ = s.publicHTTPSMutate(p, id, actor, func(r *storedPublicHTTPS) error {
			publicHTTPSUnready(r, "unknown", "The original action or current resource/access could not be confirmed. Infrastructure may remain and incur costs. Read or observe the original; an attempted creation is never repeated.")
			publicHTTPSDelay(r, false)
			return nil
		})
	}
	// A duplicate unknown effect is not pronounced settled merely because a
	// later read returned. Pause/resume and retirement have explicit reconciliation.
	// An inspection receipt describes completion of this read attempt, including
	// a retained unknown result. It never upgrades resource/TLS/application state.
	settled := input.Action == "observe" || workErr == nil
	if !first && input.Action != "observe" {
		prior := publicHTTPSReceipt(&row, input.RequestKey)
		settled = prior != nil && prior.State == "settled"
	}
	if workErr == nil {
		if next, re := s.reconcilePublicHTTPSReceipts(ctx, p, row); re == nil {
			row = next
		}
	}
	if input.Action == "retire" && row.State == "retired" {
		settled = true
	}
	row, e = s.settlePublicHTTPSAction(p, row, input.RequestKey, settled)
	if e != nil {
		return PublicHTTPSStatus{}, e
	}
	if s.publicHTTPSAuthority(ctx, actor, false, 0) != nil {
		return PublicHTTPSStatus{}, management.ErrExecutionAuthority
	}
	if row.State == "retired" {
		s.publicHTTPSMu.Lock()
		e = s.publicHTTPSDeactivateLocked(p, row)
		s.publicHTTPSMu.Unlock()
		if e != nil {
			return PublicHTTPSStatus{}, e
		}
	}
	s.WakeCoordinator()
	return row.PublicHTTPSStatus, nil
}
