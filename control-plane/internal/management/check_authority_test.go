package management

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckAuthorityUsesFreshWorkGrantsWithoutHarnessOrSecrets(t *testing.T) {
	g, ref := executionFixture(t)
	g.state.Policy.AllowedHarnesses = []string{"codex"}
	team := g.state.Teams[ref.TeamID]
	member := team.Members[ref.UserID]
	member.Grants.Secrets = false
	team.Members[ref.UserID] = member
	grant := team.Projects[ref.ProjectID][ref.UserID]
	grant.Grants.Secrets = false
	team.Projects[ref.ProjectID][ref.UserID] = grant
	saveExecutionFixture(t, g)
	file, err := NewFileCheckAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), TeamID: ref.TeamID})
	if err != nil {
		t.Fatal(err)
	}
	for name, authorize := range map[string]CheckAuthorizer{"gateway": g.AuthorizeChecks, "fixed worker": file} {
		t.Run(name, func(t *testing.T) {
			authority, err := authorize(context.Background(), ref)
			if err != nil || authority.Team == nil || !authority.Team.Permissions.Execute || authority.Team.Permissions.Secrets {
				t.Fatal("check improperly required a model or secret grant", err)
			}
		})
	}
	if _, err := g.AuthorizeExecution(context.Background(), ref, "shell", true); err == nil {
		t.Fatal("check authorization relaxed ordinary agent policy")
	}
	g.state.Policy.CheckMemoryMiB = 2048
	saveExecutionFixture(t, g)
	if fresh, err := file(context.Background(), ref); err != nil || fresh.Policy.CheckMemoryMiB != 2048 {
		t.Fatal("check used stale resource policy", err)
	}
}
func TestCheckAuthorityRevocationAndFixedScopeAreFresh(t *testing.T) {
	for _, mode := range []string{"disabled", "team-execute", "project-execute", "viewer", "removed", "suspended"} {
		t.Run(mode, func(t *testing.T) {
			g, ref := executionFixture(t)
			file, err := NewFileCheckAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), TeamID: ref.TeamID})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = file(context.Background(), ref); err != nil {
				t.Fatal(err)
			}
			team := g.state.Teams[ref.TeamID]
			switch mode {
			case "disabled":
				u := g.state.Users[ref.UserID]
				u.User.Disabled = true
				g.state.Users[ref.UserID] = u
			case "team-execute":
				m := team.Members[ref.UserID]
				m.Grants.Execute = false
				team.Members[ref.UserID] = m
			case "project-execute":
				p := team.Projects[ref.ProjectID][ref.UserID]
				p.Grants.Execute = false
				team.Projects[ref.ProjectID][ref.UserID] = p
			case "viewer":
				p := team.Projects[ref.ProjectID][ref.UserID]
				p.Role = "viewer"
				team.Projects[ref.ProjectID][ref.UserID] = p
			case "removed":
				delete(team.Members, ref.UserID)
				delete(team.Projects[ref.ProjectID], ref.UserID)
			case "suspended":
				team.Service = TeamService{Mode: "managed", Status: "suspended", Revision: 1, Reason: "Operator pause", UpdatedBy: "owner", UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			}
			g.state.Teams[ref.TeamID] = team
			saveExecutionFixture(t, g)
			if _, err = g.AuthorizeChecks(context.Background(), ref); err == nil {
				t.Fatal("gateway reused revoked check authority")
			}
			if _, err = file(context.Background(), ref); err == nil {
				t.Fatal("worker reused revoked check authority")
			}
		})
	}
	g, ref := executionFixture(t)
	file, err := NewFileCheckAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), TeamID: ref.TeamID})
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []ExecutionActorRef{{Local: true, ProjectID: ref.ProjectID}, {UserID: ref.UserID, ProjectID: ref.ProjectID}, {UserID: "foreign", TeamID: ref.TeamID, ProjectID: ref.ProjectID}, {UserID: ref.UserID, TeamID: "team-vutsrqponmlkjihgfedcba", ProjectID: ref.ProjectID}} {
		if _, err = file(context.Background(), bad); err == nil {
			t.Fatal("fixed worker accepted foreign/local authority")
		}
	}
}
