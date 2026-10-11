package api

import (
	"context"
	"errors"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestCoordinatorBackgroundReauthorizesExactStoredIdentityAndMasksStaleContext(t *testing.T) {
	ref := management.ExecutionActorRef{UserID: "alice", TeamID: "team-a", ProjectID: "p-project"}
	stale := management.WithExecutionAuthority(context.Background(), management.ExecutionAuthority{Actor: management.User{ID: "old-admin", Role: "super-admin"}})
	current := management.ExecutionAuthority{Actor: management.User{ID: "alice", Role: "member"}, Team: &management.TeamAccess{Team: management.Team{ID: "team-a"}, Member: management.TeamMember{UserID: "alice", Role: "member", Revision: 1}, ProjectID: "p-project", Permissions: management.TeamPermissions{Read: true, Edit: true, Execute: true}}}
	calls := 0
	controls := 0
	a := &API{deps: Deps{AuthorizeExecution: func(ctx context.Context, actor management.ExecutionActorRef, harness string, secrets bool) (management.ExecutionAuthority, error) {
		calls++
		if actor != ref || harness != "codex" || !secrets {
			t.Fatal("stored route identity changed")
		}
		return current, nil
	}, AuthorizeExecutionControl: func(_ context.Context, actor management.ExecutionActorRef) error {
		controls++
		if actor != ref {
			t.Fatal("fresh control authority changed")
		}
		return nil
	}}}
	fresh, err := a.reauthorizeCoordinator(stale, ref, "codex", true)
	if err != nil || calls != 1 || controls != 1 {
		t.Fatal("fresh admission", err, calls, controls)
	}
	actor, _, ok := management.ActorFromContext(fresh)
	if !ok || actor.ID != "alice" || actor.Role != "member" {
		t.Fatal("stale browser authority survived")
	}
	access, ok := management.TeamAccessFromContext(fresh)
	if !ok || access.Team.ID != "team-a" || access.ProjectID != ref.ProjectID {
		t.Fatal("current team scope lost")
	}
	for _, kind := range []string{"execute-revoked", "control-revoked", "wrong-actor", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			candidate := *a
			switch kind {
			case "execute-revoked":
				candidate.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
					return management.ExecutionAuthority{}, errors.New("revoked")
				}
			case "control-revoked":
				candidate.deps.AuthorizeExecutionControl = func(context.Context, management.ExecutionActorRef) error { return errors.New("revoked") }
			case "wrong-actor":
				candidate.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
					other := current
					other.Actor.ID = "bob"
					return other, nil
				}
			}
			ctx := stale
			if kind == "cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, err := candidate.reauthorizeCoordinator(ctx, ref, "codex", true); err == nil {
				t.Fatal("invalid current authority accepted")
			}
		})
	}
}
