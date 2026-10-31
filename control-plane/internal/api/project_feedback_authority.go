package api

import (
	"context"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Resolve the intersected project/team grant afresh at every boundary. Original
// request claims cannot retain Secrets after revocation during the GH read.
// This connector requires no Execute/Deploy grant and starts no model job.
func (a *API) authorizeProjectFeedback(ctx context.Context, ref management.ExecutionActorRef, credentialed bool) error {
	original, _, ok := management.ActorFromContext(ctx)
	if ctx.Err() != nil || !ok || original.Disabled || original.ID != ref.UserID || ref.Local || a.deps.AuthorizeProjectRead == nil {
		return management.ErrExecutionAuthority
	}
	if team, scoped := management.TeamAccessFromContext(ctx); scoped {
		if team.Team.ID != ref.TeamID || team.ProjectID != ref.ProjectID || team.Member.UserID != ref.UserID {
			return management.ErrExecutionAuthority
		}
	} else if ref.TeamID != "" || management.TeamRuntimeIDFromContext(ctx) != "" {
		return management.ErrExecutionAuthority
	}
	// Run the existing control check first; the last grant read below must own
	// Edit+Secrets truth after that potentially waiting authorization I/O.
	if credentialed && (a.deps.AuthorizeExecutionControl == nil || a.deps.AuthorizeExecutionControl(ctx, ref) != nil) {
		return management.ErrExecutionAuthority
	}
	fresh, err := a.deps.AuthorizeProjectRead(ctx, ref)
	if err != nil || fresh.Actor.ID != ref.UserID || fresh.Actor.Disabled {
		return management.ErrExecutionAuthority
	}
	if ref.TeamID != "" && (fresh.Team == nil || fresh.Team.Team.ID != ref.TeamID || fresh.Team.ProjectID != ref.ProjectID || fresh.Team.Member.UserID != ref.UserID || !fresh.Team.Permissions.Read) {
		return management.ErrExecutionAuthority
	}
	if credentialed {
		if ref.TeamID != "" && (!fresh.Team.Permissions.Edit || !fresh.Team.Permissions.Secrets) {
			return management.ErrExecutionAuthority
		}
		if ref.TeamID == "" && fresh.Actor.Role != "member" && fresh.Actor.Role != "admin" && fresh.Actor.Role != "super-admin" {
			return management.ErrExecutionAuthority
		}
		current := management.WithExecutionAuthority(ctx, fresh)
		captured, captureErr := management.CaptureExecutionControlActor(current, ref.ProjectID)
		if captureErr != nil || captured != ref {
			return management.ErrExecutionAuthority
		}
	}
	return ctx.Err()
}
