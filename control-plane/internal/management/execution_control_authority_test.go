package management

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func controlFixture(t *testing.T) (*Gateway, ExecutionActorRef, ExecutionControlAuthorizer) {
	t.Helper()
	g, ref := executionFixture(t)
	row := g.state.Teams[ref.TeamID]
	member := row.Members[ref.UserID]
	member.Grants = TeamGrants{}
	row.Members[ref.UserID] = member
	grant := row.Projects[ref.ProjectID][ref.UserID]
	grant.Grants = TeamGrants{}
	row.Projects[ref.ProjectID][ref.UserID] = grant
	row.Service = TeamService{Mode: "managed", Status: "suspended", Revision: 1, Reason: "service paused", UpdatedBy: "owner", UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	row.Billing = &TeamBilling{ConfigID: strings.Repeat("a", 64), Revision: 1, Status: "canceled", CustomerID: "customer", SubscriptionID: "subscription", PeriodEnd: time.Now().Add(-time.Hour).Unix()}
	g.state.Teams[ref.TeamID] = row
	g.managedTeamID = ref.TeamID
	g.billing = &teamBillingRuntime{configID: row.Billing.ConfigID}
	g.state.Policy.AllowedHarnesses = []string{"shell"}
	saveExecutionFixture(t, g)
	file, err := NewFileExecutionControlAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), TeamID: ref.TeamID, ManagedTeamID: ref.TeamID})
	if err != nil {
		t.Fatal(err)
	}
	return g, ref, file
}

func TestExecutionControlAllowsCurrentEditorWithoutExecutionOrBilling(t *testing.T) {
	g, ref, file := controlFixture(t)
	if _, err := g.AuthorizeExecution(context.Background(), ref, "codex", true); err == nil {
		t.Fatal("fixture unexpectedly allowed execution")
	}
	if err := g.AuthorizeExecutionControl(context.Background(), ref); err != nil {
		t.Fatalf("editor blocked by execution/service/billing settings: %v", err)
	}
	if err := file(context.Background(), ref); err != nil {
		t.Fatalf("worker editor blocked by execution/service/billing settings: %v", err)
	}
	access, err := g.resolveTeamAccess(ref.TeamID, ref.UserID, ref.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithTeamAccess(withActor(context.Background(), g.state.Users[ref.UserID].User, g.state.Policy), access)
	captured, err := CaptureExecutionControlActor(ctx, ref.ProjectID)
	if err != nil || captured != ref {
		t.Fatalf("edit-only control identity unavailable: %+v %v", captured, err)
	}
	if _, err := CaptureExecutionActor(ctx, ref.ProjectID); err == nil {
		t.Fatal("control capture expanded execution access")
	}
}

func TestExecutionControlUsesFreshMembershipAndFixedScope(t *testing.T) {
	for _, kind := range []string{"disabled", "viewer", "project removed", "foreign team", "local", "missing state"} {
		t.Run(kind, func(t *testing.T) {
			g, ref, file := controlFixture(t)
			candidate := ref
			switch kind {
			case "disabled":
				u := g.state.Users[ref.UserID]
				u.User.Disabled = true
				g.state.Users[ref.UserID] = u
			case "viewer":
				row := g.state.Teams[ref.TeamID]
				grant := row.Projects[ref.ProjectID][ref.UserID]
				grant.Role = "viewer"
				row.Projects[ref.ProjectID][ref.UserID] = grant
			case "project removed":
				delete(g.state.Teams[ref.TeamID].Projects[ref.ProjectID], ref.UserID)
			case "foreign team":
				candidate.TeamID = "team-vutsrqponmlkjihgfedcba"
			case "local":
				candidate = ExecutionActorRef{ProjectID: ref.ProjectID, Local: true}
			}
			saveExecutionFixture(t, g)
			if kind == "missing state" {
				if err := os.Remove(filepath.Join(g.dir, "identity.json")); err != nil {
					t.Fatal(err)
				}
			} else if err := g.AuthorizeExecutionControl(context.Background(), candidate); err == nil {
				t.Fatal("gateway reused old editing access")
			}
			if err := file(context.Background(), candidate); err == nil {
				t.Fatal("worker reused old or foreign editing access")
			}
		})
	}
}

func TestExecutionControlCaptureRejectsUnscopedTeamAndForeignMember(t *testing.T) {
	g, ref, _ := controlFixture(t)
	ctx := withTeamRuntime(withActor(context.Background(), g.state.Users[ref.UserID].User, g.state.Policy), ref.TeamID)
	if _, err := CaptureExecutionControlActor(ctx, ref.ProjectID); err == nil {
		t.Fatal("team runtime captured personal authority")
	}
	access, _ := g.resolveTeamAccess(ref.TeamID, ref.UserID, ref.ProjectID)
	access.Member.UserID = "other"
	ctx = WithTeamAccess(ctx, access)
	if _, err := CaptureExecutionControlActor(ctx, ref.ProjectID); err == nil {
		t.Fatal("foreign member captured as current actor")
	}
}

func TestExecutionControlPersonalWorkerCannotRecoverAnotherOwner(t *testing.T) {
	g, _, _ := controlFixture(t)
	file, err := NewFileExecutionControlAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), UserID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	own := ExecutionActorRef{UserID: "owner", ProjectID: "own-project"}
	if err := file(context.Background(), own); err != nil {
		t.Fatal(err)
	}
	own.UserID = "member"
	if err := file(context.Background(), own); err == nil {
		t.Fatal("personal worker recovered foreign account")
	}
}
