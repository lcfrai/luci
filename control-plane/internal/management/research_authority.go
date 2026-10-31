package management

import "context"

// Public research needs current project Edit; credentialed search additionally
// needs Secrets, not Execute/Deploy or a model/provider grant.
type ResearchAuthorizer func(context.Context, ExecutionActorRef, bool) error

func (g *Gateway) authorizeResearchLocked(ref ExecutionActorRef, secrets bool) error {
	if err := g.authorizeExecutionControlLocked(ref); err != nil {
		return err
	}
	if secrets && ref.TeamID != "" {
		access, err := g.resolveTeamAccessLocked(ref.TeamID, ref.UserID, ref.ProjectID)
		if err != nil || !access.Permissions.Secrets {
			return ErrExecutionAuthority
		}
	}
	return nil
}
func (g *Gateway) AuthorizeResearch(ctx context.Context, ref ExecutionActorRef, secrets bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.authorizeResearchLocked(ref, secrets)
}
func NewFileResearchAuthorizer(scope FileExecutionAuthorityScope) (ResearchAuthorizer, error) {
	if _, err := NewFileExecutionControlAuthorizer(scope); err != nil {
		return nil, err
	}
	return func(ctx context.Context, ref ExecutionActorRef, secrets bool) error {
		if ctx.Err() != nil || ref.Local || ref.UserID == "" || scope.UserID != "" && (ref.UserID != scope.UserID || ref.TeamID != "") || scope.TeamID != "" && ref.TeamID != scope.TeamID {
			return ErrExecutionAuthority
		}
		state, err := readExecutionIdentity(scope.IdentityPath)
		if err != nil || ctx.Err() != nil {
			return ErrExecutionAuthority
		}
		g := Gateway{state: state, managedTeamID: scope.ManagedTeamID}
		return g.authorizeResearchLocked(ref, secrets)
	}, nil
}
