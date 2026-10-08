package management

import "context"

// CheckAuthorizer rereads current identity, grants, service and resource policy.
// It does not substitute an arbitrary allowed AI harness for check authority.
type CheckAuthorizer func(context.Context, ExecutionActorRef) (ExecutionAuthority, error)

func (g *Gateway) AuthorizeChecks(ctx context.Context, ref ExecutionActorRef) (ExecutionAuthority, error) {
	if err := ctx.Err(); err != nil {
		return ExecutionAuthority{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.authorizeWorkLocked(ref, false)
}

func NewFileCheckAuthorizer(scope FileExecutionAuthorityScope) (CheckAuthorizer, error) {
	if _, err := NewFileExecutionAuthorizer(scope); err != nil {
		return nil, err
	}
	return func(ctx context.Context, ref ExecutionActorRef) (ExecutionAuthority, error) {
		if err := ctx.Err(); err != nil {
			return ExecutionAuthority{}, err
		}
		if ref.Local || ref.UserID == "" || (scope.UserID != "" && (ref.UserID != scope.UserID || ref.TeamID != "")) || (scope.TeamID != "" && ref.TeamID != scope.TeamID) {
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
		if row, ok := state.Teams[ref.TeamID]; ok && row.Billing != nil && scope.BillingConfigID == "" {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		if err := ctx.Err(); err != nil {
			return ExecutionAuthority{}, err
		}
		return g.authorizeWorkLocked(ref, false)
	}, nil
}
