package management

import (
	"context"
	"path/filepath"
	"testing"
)

func TestResearchUsesFreshEditAndSecretsWithoutExecutionOrDeployment(t *testing.T) {
	g, ref, _ := controlFixture(t)
	file, err := NewFileResearchAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), TeamID: ref.TeamID, ManagedTeamID: ref.TeamID})
	if err != nil {
		t.Fatal(err)
	}
	if err = g.AuthorizeResearch(context.Background(), ref, false); err != nil {
		t.Fatal("public read should need only current Edit", err)
	}
	if err = file(context.Background(), ref, true); err == nil {
		t.Fatal("search read project secret without Secrets")
	}
	row := g.state.Teams[ref.TeamID]
	member := row.Members[ref.UserID]
	member.Grants.Secrets = true
	row.Members[ref.UserID] = member
	grant := row.Projects[ref.ProjectID][ref.UserID]
	grant.Grants.Secrets = true
	row.Projects[ref.ProjectID][ref.UserID] = grant
	g.state.Teams[ref.TeamID] = row
	saveExecutionFixture(t, g)
	if err = file(context.Background(), ref, true); err != nil {
		t.Fatal("search wrongly required Execute/Deploy/service admission", err)
	}
	grant.Grants.Secrets = false
	row.Projects[ref.ProjectID][ref.UserID] = grant
	g.state.Teams[ref.TeamID] = row
	saveExecutionFixture(t, g)
	if err = file(context.Background(), ref, true); err == nil {
		t.Fatal("retained context overrode current revoked project Secrets")
	}
	if err = file(context.Background(), ref, false); err != nil {
		t.Fatal("public-only collection lost independent current Edit", err)
	}
}
