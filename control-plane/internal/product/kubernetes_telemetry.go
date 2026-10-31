package product

import (
	"context"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// One explicit read uses the EXISTING original deployment journal/controller.
// It cannot Apply, start a collector, acquire an image, choose a URL or forward
// raw metrics/credentials. Late denial withholds the response; retained samples
// are not rolled back or silently changed into current/lifetime evidence.
func (s *Server) ReadKubernetesTelemetry(ctx context.Context, actor management.ExecutionActorRef, id string) (KubernetesDeploymentStatus, error) {
	ctx = WithKubernetesTargetResolution(ctx)
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if !validKubeReceipt(id) || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return KubernetesDeploymentStatus{}, management.ErrExecutionAuthority
	}
	s.kubernetesReviewMu.Lock()
	_, ledger, err := s.readKubernetesReviews(actor.ProjectID)
	var selected *KubernetesReview
	if err == nil {
		for _, row := range ledger.Reviews {
			if row.ID == id && kubeActor(row.Request.Identity) == actor {
				copy := row
				selected = &copy
				break
			}
		}
	}
	s.kubernetesReviewMu.Unlock()
	if err != nil || selected == nil || !selected.Applied {
		return KubernetesDeploymentStatus{}, ErrKubernetesReview
	}
	row := *selected
	if err = s.authorizeKubernetesReview(ctx, row, false, true); err != nil {
		return KubernetesDeploymentStatus{}, err
	}
	connector := s.NewKubernetesDeploymentConnector(s.resolveKubernetesTarget, func(current context.Context, request deploymentkube.Request, mutation bool) error {
		if mutation || reviewDigest(request) != reviewDigest(row.Request) {
			return ErrKubernetesReview
		}
		return s.authorizeKubernetesTelemetryRead(current, row)
	})
	operation, err := connector.ReadTelemetry(ctx, row.Request.Identity)
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return KubernetesDeploymentStatus{}, management.ErrExecutionAuthority
	}
	if err != nil {
		return KubernetesDeploymentStatus{}, ErrKubernetesReview
	}
	if s.authorizeKubernetesReview(ctx, row, false, true) != nil {
		return KubernetesDeploymentStatus{}, ErrKubernetesReview
	}
	return kubeStatus(row, &operation), nil
}

// Per-object reads recheck current authority and exact private target metadata.
// Remote account/cluster/source resolution stays at the explicit boundaries;
// this callback never mints another credential or spends the whole read budget.
func (s *Server) authorizeKubernetesTelemetryRead(ctx context.Context, row KubernetesReview) error {
	actor := kubeActor(row.Request.Identity)
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return management.ErrExecutionAuthority
	}
	_, ledger, err := s.readKubernetesTargets(actor.ProjectID)
	if err != nil {
		return ErrKubernetesReview
	}
	matched := false
	for _, target := range ledger.Targets {
		if target.KubernetesTargetDescriptor == row.Target && target.ID == row.Request.Identity.TargetID && target.Fingerprint == row.Request.Identity.TargetFingerprint {
			matched = true
			break
		}
	}
	if !matched {
		return ErrKubernetesReview
	}
	return s.kubernetesAuthority(ctx, actor, false, true, 0)
}
