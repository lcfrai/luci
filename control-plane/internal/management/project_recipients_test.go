package management

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileProjectRecipientsIncludeCurrentReaderAndDropRevokedMember(t *testing.T) {
	g, ref := executionFixture(t)
	team := g.state.Teams[ref.TeamID]
	grant := team.Projects[ref.ProjectID][ref.UserID]
	grant.Role, grant.Grants.Execute = "viewer", false
	team.Projects[ref.ProjectID][ref.UserID] = grant
	team.Members[ref.UserID] = TeamMember{UserID: ref.UserID, Role: "viewer", Revision: team.Team.Revision}
	g.state.Teams[ref.TeamID] = team
	saveExecutionFixture(t, g)
	list, read, err := NewFileProjectRecipients(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), TeamID: ref.TeamID})
	if err != nil {
		t.Fatal(err)
	}
	owner := ExecutionActorRef{UserID: "owner", TeamID: ref.TeamID, ProjectID: ref.ProjectID}
	refs, err := list(context.Background(), owner)
	if err != nil || len(refs) != 2 || refs[0].UserID != ref.UserID || refs[1].UserID != owner.UserID {
		t.Fatalf("current readers were not enumerated: %+v %v", refs, err)
	}
	if authority, err := read(context.Background(), ref); err != nil || authority.Team == nil || !authority.Team.Permissions.Read || authority.Team.Permissions.Execute {
		t.Fatal("read notification required or granted execution", err)
	}
	delete(team.Members, ref.UserID)
	delete(team.Projects[ref.ProjectID], ref.UserID)
	g.state.Teams[ref.TeamID] = team
	saveExecutionFixture(t, g)
	if _, err := read(context.Background(), ref); err == nil {
		t.Fatal("revoked recipient retained project contents")
	}
	refs, err = list(context.Background(), owner)
	if err != nil || len(refs) != 1 || refs[0] != owner {
		t.Fatalf("revoked recipient remained in current list: %+v %v", refs, err)
	}
}
