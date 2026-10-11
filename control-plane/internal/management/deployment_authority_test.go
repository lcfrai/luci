package management

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func deploymentAuthorityFixture(t *testing.T) (*Gateway, ExecutionActorRef, DeploymentAuthorizer) {
	t.Helper()
	g, ref := executionFixture(t)
	row := g.state.Teams[ref.TeamID]
	member := row.Members[ref.UserID]
	member.Grants = TeamGrants{Deploy: true}
	row.Members[ref.UserID] = member
	project := row.Projects[ref.ProjectID][ref.UserID]
	project.Grants = TeamGrants{Deploy: true}
	row.Projects[ref.ProjectID][ref.UserID] = project
	g.state.Teams[ref.TeamID] = row
	saveExecutionFixture(t, g)
	file, err := NewFileDeploymentAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), TeamID: ref.TeamID})
	if err != nil {
		t.Fatal(err)
	}
	return g, ref, file
}

func TestDeploymentAuthorityUsesDeployWithoutExecuteHarnessOrSecrets(t *testing.T) {
	g, ref, file := deploymentAuthorityFixture(t)
	for name, authorize := range map[string]DeploymentAuthorizer{"gateway": g.AuthorizeDeployment, "fixed worker": file} {
		t.Run(name, func(t *testing.T) {
			for _, allocate := range []bool{true, false} {
				authority, err := authorize(context.Background(), ref, allocate)
				if err != nil || authority.Team == nil || !authority.Team.Permissions.Deploy || authority.Team.Permissions.Execute || authority.Team.Permissions.Secrets {
					t.Fatal("deployment substituted agent or secret grants", allocate, authority, err)
				}
				captured, err := CaptureDeploymentActor(WithExecutionAuthority(context.Background(), authority), ref.ProjectID)
				if err != nil || captured != ref {
					t.Fatal("request capture disagrees with fresh deployment authority", captured, err)
				}
			}
		})
	}
	if _, err := g.AuthorizeChecks(context.Background(), ref); err == nil {
		t.Fatal("deploy-only grant gained check execution authority")
	}
	g.state.Policy.CheckMemoryMiB = 2048
	saveExecutionFixture(t, g)
	if fresh, err := file(context.Background(), ref, true); err != nil || fresh.Policy.CheckMemoryMiB != 2048 {
		t.Fatal("deployment reused old resource policy", err)
	}
}

func TestDeploymentAuthorityReauthorizesAndAllowsSuspendedCleanupOnly(t *testing.T) {
	for _, mode := range []string{"disabled", "team-deploy", "project-deploy", "viewer", "removed", "suspended"} {
		t.Run(mode, func(t *testing.T) {
			g, ref, file := deploymentAuthorityFixture(t)
			if _, err := file(context.Background(), ref, true); err != nil {
				t.Fatal(err)
			}
			team := g.state.Teams[ref.TeamID]
			switch mode {
			case "disabled":
				user := g.state.Users[ref.UserID]
				user.User.Disabled = true
				g.state.Users[ref.UserID] = user
			case "team-deploy":
				member := team.Members[ref.UserID]
				member.Grants.Deploy = false
				team.Members[ref.UserID] = member
			case "project-deploy":
				project := team.Projects[ref.ProjectID][ref.UserID]
				project.Grants.Deploy = false
				team.Projects[ref.ProjectID][ref.UserID] = project
			case "viewer":
				project := team.Projects[ref.ProjectID][ref.UserID]
				project.Role = "viewer"
				team.Projects[ref.ProjectID][ref.UserID] = project
			case "removed":
				delete(team.Members, ref.UserID)
				delete(team.Projects[ref.ProjectID], ref.UserID)
			case "suspended":
				team.Service = TeamService{Mode: "managed", Status: "suspended", Revision: 1, Reason: "Operator pause", UpdatedBy: "owner", UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			}
			g.state.Teams[ref.TeamID] = team
			saveExecutionFixture(t, g)
			for _, authorize := range []DeploymentAuthorizer{g.AuthorizeDeployment, file} {
				if _, err := authorize(context.Background(), ref, true); err == nil {
					t.Fatal("revoked authority admitted a new deployment")
				}
				_, err := authorize(context.Background(), ref, false)
				if (err == nil) != (mode == "suspended") {
					t.Fatal("cleanup grants confused with service admission", mode, err)
				}
			}
		})
	}
}

func TestDeploymentAuthorityFixedScopeAndCanceledRequestsFailClosed(t *testing.T) {
	g, ref, file := deploymentAuthorityFixture(t)
	for _, bad := range []ExecutionActorRef{{Local: true, ProjectID: ref.ProjectID}, {UserID: ref.UserID, ProjectID: ref.ProjectID}, {UserID: "foreign", TeamID: ref.TeamID, ProjectID: ref.ProjectID}, {UserID: ref.UserID, TeamID: "team-vutsrqponmlkjihgfedcba", ProjectID: ref.ProjectID}} {
		for _, allocate := range []bool{true, false} {
			if _, err := file(context.Background(), bad, allocate); err == nil {
				t.Fatal("fixed worker accepted foreign/local deployment authority", bad)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, authorize := range []DeploymentAuthorizer{g.AuthorizeDeployment, file} {
		if _, err := authorize(ctx, ref, true); err != context.Canceled {
			t.Fatal("canceled request continued authorization", err)
		}
	}
}
