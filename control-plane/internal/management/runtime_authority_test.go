package management

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func runtimeAuthorityFixture(t *testing.T) (*Gateway, ExecutionActorRef, RuntimeInstallerAuthorizer) {
	t.Helper()
	g, _ := executionFixture(t)
	ref := ExecutionActorRef{UserID: "runtime-admin"}
	g.state.Users[ref.UserID] = userRecord{User: User{ID: ref.UserID, Username: "runtime-admin", Role: "admin"}}
	saveExecutionFixture(t, g)
	file, e := NewFileRuntimeInstallerAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), UserID: ref.UserID})
	if e != nil {
		t.Fatal(e)
	}
	return g, ref, file
}
func TestRuntimeInstallerAuthorityAcceptsFreshPersonalAdminsOnly(t *testing.T) {
	for _, role := range []string{"admin", "super-admin"} {
		t.Run(role, func(t *testing.T) {
			g, ref, file := runtimeAuthorityFixture(t)
			user := g.state.Users[ref.UserID]
			user.User.Role = role
			g.state.Users[ref.UserID] = user
			saveExecutionFixture(t, g)
			for _, authorize := range []RuntimeInstallerAuthorizer{g.AuthorizeRuntimeInstaller, file} {
				authority, e := authorize(context.Background(), ref)
				if e != nil || authority.Actor.ID != ref.UserID || authority.Actor.Role != role || authority.Team != nil {
					t.Fatal(authority, e)
				}
				captured, e := CaptureRuntimeInstallerActor(WithExecutionAuthority(context.Background(), authority))
				if e != nil || captured != ref {
					t.Fatal(captured, e)
				}
				if len(authority.Policy.AllowedHarnesses) > 0 {
					authority.Policy.AllowedHarnesses[0] = "mutated-copy"
					if g.state.Policy.AllowedHarnesses[0] == "mutated-copy" {
						t.Fatal("returned policy aliases authority store")
					}
				}
			}
			g.state.Policy.CheckMemoryMiB = 2048
			g.state.Policy.CheckCPUMillis = 500
			saveExecutionFixture(t, g)
			authority, e := file(context.Background(), ref)
			if e != nil || authority.Policy.CheckMemoryMiB != 2048 || authority.Policy.CheckCPUMillis != 500 {
				t.Fatal("file authority retained stale limits", authority, e)
			}
		})
	}
}
func TestRuntimeInstallerAuthorityRechecksRoleDisabledRemovalAndPolicy(t *testing.T) {
	for _, mode := range []string{"role withdrawal", "disabled", "removed", "invalid policy"} {
		t.Run(mode, func(t *testing.T) {
			g, ref, file := runtimeAuthorityFixture(t)
			if _, e := file(context.Background(), ref); e != nil {
				t.Fatal(e)
			}
			user := g.state.Users[ref.UserID]
			switch mode {
			case "role withdrawal":
				user.User.Role = "member"
				g.state.Users[ref.UserID] = user
			case "disabled":
				user.User.Disabled = true
				g.state.Users[ref.UserID] = user
			case "removed":
				delete(g.state.Users, ref.UserID)
			case "invalid policy":
				g.state.Policy.CheckMemoryMiB = 0
			}
			saveExecutionFixture(t, g)
			for _, authorize := range []RuntimeInstallerAuthorizer{g.AuthorizeRuntimeInstaller, file} {
				if _, e := authorize(context.Background(), ref); !errors.Is(e, ErrExecutionAuthority) {
					t.Fatal("stale authority granted installer operation", mode, e)
				}
			}
		})
	}
}
func TestRuntimeInstallerAuthorityRejectsTeamProjectForeignAndManagedLocal(t *testing.T) {
	g, ref, file := runtimeAuthorityFixture(t)
	for _, bad := range []ExecutionActorRef{{Local: true}, {UserID: "foreign"}, {UserID: ref.UserID, TeamID: "team-abcdefghijklmnopqrstuv"}, {UserID: ref.UserID, ProjectID: "p-one"}, {Local: true, UserID: ref.UserID}} {
		for _, authorize := range []RuntimeInstallerAuthorizer{g.AuthorizeRuntimeInstaller, file} {
			if _, e := authorize(context.Background(), bad); !errors.Is(e, ErrExecutionAuthority) {
				t.Fatal("installer accepted foreign/team/project/unauthenticated actor", bad, e)
			}
		}
	}
	teamFile, e := NewFileRuntimeInstallerAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), TeamID: "team-abcdefghijklmnopqrstuv"})
	if e != nil {
		t.Fatal("team worker can initialize a permanently refusing capability", e)
	}
	if _, e = teamFile(context.Background(), ref); !errors.Is(e, ErrExecutionAuthority) {
		t.Fatal("team worker installed shared runtime", e)
	}
	g.managedTeamID = "team-abcdefghijklmnopqrstuv"
	if _, e = g.AuthorizeRuntimeInstaller(context.Background(), ref); !errors.Is(e, ErrExecutionAuthority) {
		t.Fatal("managed service accepted personal infrastructure authority", e)
	}
}
func TestRuntimeInstallerCaptureRejectsScopedAndDisabledActors(t *testing.T) {
	for _, user := range []User{{ID: "member", Role: "member"}, {ID: "admin", Role: "admin", Disabled: true}, {Role: "admin"}} {
		if _, e := CaptureRuntimeInstallerActor(WithExecutionAuthority(context.Background(), ExecutionAuthority{Actor: user})); !errors.Is(e, ErrExecutionAuthority) {
			t.Fatal("unprivileged request capture", user, e)
		}
	}
	authority := ExecutionAuthority{Actor: User{ID: "owner", Role: "super-admin"}, Team: &TeamAccess{Team: Team{ID: "team-abcdefghijklmnopqrstuv"}}}
	if _, e := CaptureRuntimeInstallerActor(WithExecutionAuthority(context.Background(), authority)); !errors.Is(e, ErrExecutionAuthority) {
		t.Fatal("team scope laundered owner role", e)
	}
	local, e := CaptureRuntimeInstallerActor(context.Background())
	if e != nil || !local.Local || local.ProjectID != "" {
		t.Fatal("legacy unmanaged capture shape", local, e)
	}
	// Capture alone never grants a managed operation: both fresh authorizers above
	// reject Local, and the API may only use fallback on an unmanaged instance.
}
func TestRuntimeInstallerAuthorityCancellationAndUnavailablePrivateState(t *testing.T) {
	g, ref, file := runtimeAuthorityFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, authorize := range []RuntimeInstallerAuthorizer{g.AuthorizeRuntimeInstaller, file} {
		if _, e := authorize(ctx, ref); !errors.Is(e, context.Canceled) {
			t.Fatal("cancelled authority continued", e)
		}
	}
	if _, e := CaptureRuntimeInstallerActor(ctx); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	// Hold the gateway lock until cancellation, then release. No stale successful
	// authorization is returned when a caller was cancelled while queued.
	g.mu.Lock()
	queuedCtx, queuedCancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	entered := make(chan struct{})
	barrierCtx := &runtimeAuthorityBarrierContext{Context: queuedCtx, checked: entered}
	go func() { _, e := g.AuthorizeRuntimeInstaller(barrierCtx, ref); done <- e }()
	<-entered
	queuedCancel()
	g.mu.Unlock()
	if e := <-done; !errors.Is(e, context.Canceled) {
		t.Fatal("cancelled queued authority succeeded", e)
	}
	if e := os.Rename(filepath.Join(g.dir, "identity.json"), filepath.Join(g.dir, "retained-identity.json")); e != nil {
		t.Fatal(e)
	}
	if _, e := file(context.Background(), ref); !errors.Is(e, ErrExecutionAuthority) {
		t.Fatal("missing private source granted stale access", e)
	}
	g.closed = true
	if _, e := g.AuthorizeRuntimeInstaller(context.Background(), ref); !errors.Is(e, ErrExecutionAuthority) {
		t.Fatal("closed management authority accepted work", e)
	}
}

type runtimeAuthorityBarrierContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *runtimeAuthorityBarrierContext) Err() error {
	v := c.Context.Err()
	c.once.Do(func() { close(c.checked) })
	return v
}
