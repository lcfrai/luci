package management

import (
	"context"
	"sort"
)

// ProjectRecipientLister returns durable identity references, not membership
// snapshots. The worker rechecks each reference before delivering content and
// again before any optional paid model turn.
type ProjectRecipientLister func(context.Context, ExecutionActorRef) ([]ExecutionActorRef, error)
type ProjectReadAuthorizer func(context.Context, ExecutionActorRef) (ExecutionAuthority, error)

func NewFileProjectRecipients(scope FileExecutionAuthorityScope) (ProjectRecipientLister, ProjectReadAuthorizer, error) {
	if _, err := NewFileExecutionAuthorizer(scope); err != nil {
		return nil, nil, err
	}
	load := func(ctx context.Context) (*Gateway, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		state, err := readExecutionIdentity(scope.IdentityPath)
		if err != nil {
			return nil, ErrExecutionAuthority
		}
		return &Gateway{state: state, managedTeamID: scope.ManagedTeamID}, nil
	}
	read := func(ctx context.Context, ref ExecutionActorRef) (ExecutionAuthority, error) {
		g, err := load(ctx)
		if err != nil || ref.Local || ref.UserID == "" || !teamProjectIDPattern.MatchString(ref.ProjectID) || (scope.UserID != "" && (ref.UserID != scope.UserID || ref.TeamID != "")) || (scope.TeamID != "" && ref.TeamID != scope.TeamID) {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		user, ok := g.state.Users[ref.UserID]
		if !ok || user.User.Disabled || user.User.ID != ref.UserID {
			return ExecutionAuthority{}, ErrExecutionAuthority
		}
		out := ExecutionAuthority{Actor: user.User, Policy: g.state.Policy}
		if ref.TeamID != "" {
			access, accessErr := g.resolveTeamAccessLocked(ref.TeamID, ref.UserID, ref.ProjectID)
			if accessErr != nil || !access.Permissions.Read {
				return ExecutionAuthority{}, ErrExecutionAuthority
			}
			out.Actor, out.Team = scopedTeamActor(user.User, access), &access
		}
		return out, nil
	}
	list := func(ctx context.Context, source ExecutionActorRef) ([]ExecutionActorRef, error) {
		if _, err := read(ctx, source); err != nil {
			return nil, err
		}
		if source.TeamID == "" {
			return []ExecutionActorRef{source}, nil
		}
		g, err := load(ctx)
		if err != nil {
			return nil, err
		}
		team, ok := g.state.Teams[source.TeamID]
		if !ok {
			return nil, ErrExecutionAuthority
		}
		out := make([]ExecutionActorRef, 0, len(team.Members))
		for id := range team.Members {
			if access, err := g.resolveTeamAccessLocked(source.TeamID, id, source.ProjectID); err == nil && access.Permissions.Read {
				out = append(out, ExecutionActorRef{UserID: id, TeamID: source.TeamID, ProjectID: source.ProjectID})
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].UserID < out[j].UserID })
		return out, nil
	}
	return list, read, nil
}
