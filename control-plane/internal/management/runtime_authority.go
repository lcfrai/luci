package management

import "context"

// RuntimeInstallerAuthorizer is backing-management authority, independent of
// project Execute/Deploy and AI providers. Team workers cannot install a shared
// daemon's runtime through a project grant. Every side effect rechecks this.
type RuntimeInstallerAuthorizer func(context.Context, ExecutionActorRef) (ExecutionAuthority, error)

func CaptureRuntimeInstallerActor(ctx context.Context) (ExecutionActorRef, error) {
	if err := ctx.Err(); err != nil {
		return ExecutionActorRef{}, err
	}
	if _, team := TeamAccessFromContext(ctx); team || TeamRuntimeIDFromContext(ctx) != "" {
		return ExecutionActorRef{}, ErrExecutionAuthority
	}
	actor, _, authenticated := ActorFromContext(ctx)
	if !authenticated {
		return ExecutionActorRef{Local: true}, nil
	}
	if actor.ID == "" || actor.Disabled || (actor.Role != "admin" && actor.Role != "super-admin") {
		return ExecutionActorRef{}, ErrExecutionAuthority
	}
	return ExecutionActorRef{UserID: actor.ID}, nil
}
func (g *Gateway) AuthorizeRuntimeInstaller(ctx context.Context, ref ExecutionActorRef) (ExecutionAuthority, error) {
	if err := ctx.Err(); err != nil {
		return ExecutionAuthority{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return ExecutionAuthority{}, err
	}
	return g.authorizeRuntimeInstallerLocked(ref)
}
func (g *Gateway) authorizeRuntimeInstallerLocked(ref ExecutionActorRef) (ExecutionAuthority, error) {
	if g.closed || g.managedTeamID != "" || ref.Local || ref.UserID == "" || ref.TeamID != "" || ref.ProjectID != "" || validatePolicy(g.state.Policy) != nil {
		return ExecutionAuthority{}, ErrExecutionAuthority
	}
	row, ok := g.state.Users[ref.UserID]
	if !ok || row.User.ID != ref.UserID || row.User.Disabled || (row.User.Role != "admin" && row.User.Role != "super-admin") {
		return ExecutionAuthority{}, ErrExecutionAuthority
	}
	policy := g.state.Policy
	policy.AllowedHarnesses = append([]string(nil), policy.AllowedHarnesses...)
	return ExecutionAuthority{Actor: row.User, Policy: policy}, nil
}
func NewFileRuntimeInstallerAuthorizer(scope FileExecutionAuthorityScope) (RuntimeInstallerAuthorizer, error) {
	if _, err := NewFileExecutionAuthorizer(scope); err != nil {
		return nil, err
	}
	return func(ctx context.Context, ref ExecutionActorRef) (ExecutionAuthority, error) {
		if err := ctx.Err(); err != nil {
			return ExecutionAuthority{}, err
		}
		if scope.TeamID != "" || scope.ManagedTeamID != "" || scope.UserID == "" || ref.Local || ref.UserID != scope.UserID || ref.TeamID != "" || ref.ProjectID != "" {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		state, err := readExecutionIdentity(scope.IdentityPath)
		if err != nil {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		if err = ctx.Err(); err != nil {
			return ExecutionAuthority{}, err
		}
		g := Gateway{state: state}
		return g.authorizeRuntimeInstallerLocked(ref)
	}, nil
}
