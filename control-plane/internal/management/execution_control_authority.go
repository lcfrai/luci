package management

import "context"

// Recovery changes private task bookkeeping, not execution admission. An
// editor must still be able to retire proven never-started work after model
// permission, service access or a subscription has been withdrawn.
type ExecutionControlAuthorizer func(context.Context, ExecutionActorRef) error

func CaptureExecutionControlActor(ctx context.Context, projectID string) (ExecutionActorRef, error) {
	if !teamProjectIDPattern.MatchString(projectID) {
		return ExecutionActorRef{}, ErrExecutionAuthority
	}
	actor, _, authenticated := ActorFromContext(ctx)
	team, scoped := TeamAccessFromContext(ctx)
	if scoped {
		if !authenticated || actor.ID == "" || actor.Disabled || team.Team.ID == "" || team.Member.UserID != actor.ID || team.ProjectID != projectID || !team.Permissions.Read || !team.Permissions.Edit {
			return ExecutionActorRef{}, ErrExecutionAuthority
		}
		return ExecutionActorRef{UserID: actor.ID, TeamID: team.Team.ID, ProjectID: projectID}, nil
	}
	if TeamRuntimeIDFromContext(ctx) != "" {
		return ExecutionActorRef{}, ErrExecutionAuthority
	}
	if authenticated {
		if actor.ID == "" || actor.Disabled {
			return ExecutionActorRef{}, ErrExecutionAuthority
		}
		return ExecutionActorRef{UserID: actor.ID, ProjectID: projectID}, nil
	}
	return ExecutionActorRef{ProjectID: projectID, Local: true}, nil
}

func (g *Gateway) AuthorizeExecutionControl(ctx context.Context, ref ExecutionActorRef) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.authorizeExecutionControlLocked(ref)
}

func (g *Gateway) authorizeExecutionControlLocked(ref ExecutionActorRef) error {
	if g.closed || ref.Local || ref.UserID == "" || !teamProjectIDPattern.MatchString(ref.ProjectID) {
		return ErrExecutionAuthority
	}
	user, ok := g.state.Users[ref.UserID]
	if !ok || user.User.Disabled || user.User.ID != ref.UserID || (user.User.Role != "member" && user.User.Role != "admin" && user.User.Role != "super-admin") {
		return ErrExecutionAuthority
	}
	if ref.TeamID == "" {
		if g.managedTeamID != "" {
			return ErrExecutionAuthority
		}
		return nil
	}
	access, err := g.resolveTeamAccessLocked(ref.TeamID, ref.UserID, ref.ProjectID)
	if err != nil || !access.Permissions.Read || !access.Permissions.Edit {
		return ErrExecutionAuthority
	}
	return nil
}

func NewFileExecutionControlAuthorizer(scope FileExecutionAuthorityScope) (ExecutionControlAuthorizer, error) {
	// Reuse exactly the execution constructor's trusted fixed-scope validation.
	// Constructing its closure performs no state read or admission decision.
	if _, err := NewFileExecutionAuthorizer(scope); err != nil {
		return nil, err
	}
	return func(ctx context.Context, ref ExecutionActorRef) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ref.Local || ref.UserID == "" || (scope.UserID != "" && (ref.UserID != scope.UserID || ref.TeamID != "")) || (scope.TeamID != "" && ref.TeamID != scope.TeamID) {
			return ErrExecutionAuthority
		}
		state, err := readExecutionIdentity(scope.IdentityPath)
		if err != nil {
			return ErrExecutionAuthority
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		g := Gateway{state: state, managedTeamID: scope.ManagedTeamID}
		return g.authorizeExecutionControlLocked(ref)
	}, nil
}
