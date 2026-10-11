package management

import "context"

// DeploymentAuthorizer resolves current deploy grants independently of agent
// execution. allocate=false permits an authorized deployer to stop retained
// work after new allocations have been suspended; it never authorizes a build.
type DeploymentAuthorizer func(context.Context, ExecutionActorRef, bool) (ExecutionAuthority, error)

func CaptureDeploymentActor(ctx context.Context, projectID string) (ExecutionActorRef, error) {
	if err := ctx.Err(); err != nil {
		return ExecutionActorRef{}, err
	}
	ref, err := CaptureExecutionControlActor(ctx, projectID)
	if err != nil {
		return ExecutionActorRef{}, err
	}
	if team, scoped := TeamAccessFromContext(ctx); scoped && !team.Permissions.Deploy {
		return ExecutionActorRef{}, ErrExecutionAuthority
	}
	return ref, nil
}

func (g *Gateway) AuthorizeDeployment(ctx context.Context, ref ExecutionActorRef, allocate bool) (ExecutionAuthority, error) {
	if err := ctx.Err(); err != nil {
		return ExecutionAuthority{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.authorizeDeploymentLocked(ref, allocate)
}

func (g *Gateway) authorizeDeploymentLocked(ref ExecutionActorRef, allocate bool) (ExecutionAuthority, error) {
	if err := g.authorizeExecutionControlLocked(ref); err != nil || validatePolicy(g.state.Policy) != nil {
		return ExecutionAuthority{}, ErrExecutionAuthority
	}
	user := g.state.Users[ref.UserID].User
	policy := g.state.Policy
	policy.AllowedHarnesses = append([]string(nil), policy.AllowedHarnesses...)
	out := ExecutionAuthority{Actor: user, Policy: policy}
	if ref.TeamID == "" {
		return out, nil
	}
	access, err := g.resolveTeamAccessLocked(ref.TeamID, ref.UserID, ref.ProjectID)
	if err != nil || !access.Permissions.Read || !access.Permissions.Edit || !access.Permissions.Deploy || allocate && !g.teamServiceAllowsExecutionLocked(ref.TeamID) {
		return ExecutionAuthority{}, ErrExecutionAuthority
	}
	out.Actor = scopedTeamActor(user, access)
	out.Policy = teamPolicy(policy)
	out.Team = &access
	return out, nil
}

func NewFileDeploymentAuthorizer(scope FileExecutionAuthorityScope) (DeploymentAuthorizer, error) {
	// Reuse the trusted fixed-directory and worker-scope constructor validation.
	if _, err := NewFileExecutionAuthorizer(scope); err != nil {
		return nil, err
	}
	return func(ctx context.Context, ref ExecutionActorRef, allocate bool) (ExecutionAuthority, error) {
		if err := ctx.Err(); err != nil {
			return ExecutionAuthority{}, err
		}
		if ref.Local || ref.UserID == "" || scope.UserID != "" && (ref.UserID != scope.UserID || ref.TeamID != "") || scope.TeamID != "" && ref.TeamID != scope.TeamID {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		state, err := readExecutionIdentity(scope.IdentityPath)
		if err != nil {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		g := Gateway{state: state, managedTeamID: scope.ManagedTeamID}
		if scope.BillingConfigID != "" {
			g.billing = &teamBillingRuntime{configID: scope.BillingConfigID}
		}
		// Retain the existing fixed worker's billing binding fail-closed rule.
		if row, ok := state.Teams[ref.TeamID]; ok && row.Billing != nil && scope.BillingConfigID == "" {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		if err := ctx.Err(); err != nil {
			return ExecutionAuthority{}, err
		}
		return g.authorizeDeploymentLocked(ref, allocate)
	}, nil
}
