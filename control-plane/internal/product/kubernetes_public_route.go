package product

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Public routing alone adds the publishing policy gate. Private workloads and
// retained read-only inspection do not acquire this unrelated requirement.
func (s *Server) kubernetesPublicAuthority(ctx context.Context, actor management.ExecutionActorRef) error {
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return err
	}
	authority, err := s.coordinatorDeploymentAuthorize(ctx, actor, false)
	if err != nil || !authority.Policy.AllowPublicPublishing {
		return management.ErrExecutionAuthority
	}
	fresh := management.WithExecutionAuthority(ctx, authority)
	if current, err := management.CaptureDeploymentActor(fresh, actor.ProjectID); err != nil || current != actor {
		return management.ErrExecutionAuthority
	}
	return ctx.Err()
}
