package api

import (
	"context"
	"errors"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestProjectFeedbackUsesLatestIntersectedSecretsWithoutExecuteOrDeploy(t *testing.T) {
	e := sessionsNewEnv(t)
	ctx := orchestratorExecutionActor(t)
	ref := management.ExecutionActorRef{UserID: "u-reviewed-task-fixture", TeamID: "team-feedback", ProjectID: "p-8c1d0e42"}
	team := management.TeamAccess{Team: management.Team{ID: ref.TeamID}, Member: management.TeamMember{UserID: ref.UserID}, ProjectID: ref.ProjectID, Permissions: management.TeamPermissions{Read: true, Edit: true, Secrets: true}}
	ctx = management.WithTeamAccess(ctx, team)
	fresh := management.ExecutionAuthority{Actor: management.User{ID: ref.UserID, Role: "member"}, Team: &team}
	order := []string{}
	e.api.deps.AuthorizeExecutionControl = func(context.Context, management.ExecutionActorRef) error {
		order = append(order, "control")
		return nil
	}
	e.api.deps.AuthorizeProjectRead = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		order = append(order, "fresh-read")
		return fresh, nil
	}
	if err := e.api.authorizeProjectFeedback(ctx, ref, true); err != nil {
		t.Fatal("fetch incorrectly required Execute/Deploy", err)
	}
	if len(order) != 2 || order[0] != "control" || order[1] != "fresh-read" {
		t.Fatal("Secrets read did not follow waiting control check")
	}
	// Original request still has Secrets; only the fresh intersected grant changes.
	revoked := team
	revoked.Permissions.Secrets = false
	fresh.Team = &revoked
	if err := e.api.authorizeProjectFeedback(ctx, ref, true); !errors.Is(err, management.ErrExecutionAuthority) {
		t.Fatal("stale request Secrets retained authority")
	}
	if err := e.api.authorizeProjectFeedback(ctx, ref, false); err != nil {
		t.Fatal("saved read incorrectly required Secrets", err)
	}
	revoked.Permissions.Edit = false
	fresh.Team = &revoked
	if err := e.api.authorizeProjectFeedback(ctx, ref, true); err == nil {
		t.Fatal("viewer imported shared content")
	}
	fresh.Actor.ID = "different-person"
	if err := e.api.authorizeProjectFeedback(ctx, ref, false); err == nil {
		t.Fatal("actor substituted")
	}
}
