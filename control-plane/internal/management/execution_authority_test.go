package management

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func executionFixture(t *testing.T) (*Gateway, ExecutionActorRef) {
	t.Helper()
	teamID := "team-abcdefghijklmnopqrstuv"
	policy := teamRuntimeTestPolicy()
	owner := User{ID: "owner", Username: "owner", Role: "super-admin"}
	member := User{ID: "member", Username: "member", Role: "member"}
	g := &Gateway{dir: t.TempDir(), state: storedState{Version: 1, OwnerID: owner.ID, Users: map[string]userRecord{owner.ID: {User: owner}, member.ID: {User: member}}, Sessions: map[string]sessionRecord{}, Policy: policy, Teams: map[string]teamRecord{teamID: {Team: Team{ID: teamID, Name: "Team", Revision: 2}, Members: map[string]TeamMember{owner.ID: {UserID: owner.ID, Role: "owner", Grants: TeamGrants{Admin: true, Execute: true, Secrets: true}, Revision: 1}, member.ID: {UserID: member.ID, Role: "editor", Grants: TeamGrants{Execute: true, Secrets: true}, Revision: 1}}, Projects: map[string]map[string]TeamProjectGrant{"project": {member.ID: {UserID: member.ID, Role: "editor", Grants: TeamGrants{Execute: true, Secrets: true}, Revision: 2}}}}}}}
	if e := privatePath(g.dir); e != nil {
		t.Fatal(e)
	}
	saveExecutionFixture(t, g)
	return g, ExecutionActorRef{UserID: member.ID, TeamID: teamID, ProjectID: "project"}
}
func saveExecutionFixture(t *testing.T, g *Gateway) {
	t.Helper()
	if e := g.persistLocked(); e != nil {
		t.Fatal(e)
	}
}
func fileExecutionFixture(t *testing.T, g *Gateway, ref ExecutionActorRef) ExecutionAuthorizer {
	t.Helper()
	a, e := NewFileExecutionAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), TeamID: ref.TeamID})
	if e != nil {
		t.Fatal(e)
	}
	return a
}

func TestExecutionAuthorityRechecksCurrentPermissions(t *testing.T) {
	mutations := map[string]func(*Gateway, ExecutionActorRef){
		"disabled user": func(g *Gateway, r ExecutionActorRef) {
			u := g.state.Users[r.UserID]
			u.User.Disabled = true
			g.state.Users[r.UserID] = u
		},
		"removed member": func(g *Gateway, r ExecutionActorRef) {
			row := g.state.Teams[r.TeamID]
			delete(row.Members, r.UserID)
			delete(row.Projects[r.ProjectID], r.UserID)
		},
		"removed project": func(g *Gateway, r ExecutionActorRef) { delete(g.state.Teams[r.TeamID].Projects[r.ProjectID], r.UserID) },
		"execute revoked": func(g *Gateway, r ExecutionActorRef) {
			row := g.state.Teams[r.TeamID]
			m := row.Members[r.UserID]
			m.Grants.Execute = false
			row.Members[r.UserID] = m
		},
		"secrets revoked": func(g *Gateway, r ExecutionActorRef) {
			row := g.state.Teams[r.TeamID]
			m := row.Members[r.UserID]
			m.Grants.Secrets = false
			row.Members[r.UserID] = m
		},
		"viewer project": func(g *Gateway, r ExecutionActorRef) {
			row := g.state.Teams[r.TeamID]
			p := row.Projects[r.ProjectID][r.UserID]
			p.Role = "viewer"
			row.Projects[r.ProjectID][r.UserID] = p
		},
		"harness disabled": func(g *Gateway, _ ExecutionActorRef) { g.state.Policy.AllowedHarnesses = []string{"codex"} },
		"service suspended": func(g *Gateway, r ExecutionActorRef) {
			row := g.state.Teams[r.TeamID]
			row.Service = TeamService{Mode: "managed", Status: "suspended", Revision: 1, Reason: "Operator pause", UpdatedBy: "owner", UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
			g.state.Teams[r.TeamID] = row
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			g, ref := executionFixture(t)
			file := fileExecutionFixture(t, g, ref)
			if _, e := g.AuthorizeExecution(context.Background(), ref, "shell", true); e != nil {
				t.Fatal(e)
			}
			if _, e := file(context.Background(), ref, "shell", true); e != nil {
				t.Fatal(e)
			}
			mutate(g, ref)
			saveExecutionFixture(t, g)
			if _, e := g.AuthorizeExecution(context.Background(), ref, "shell", true); e == nil {
				t.Fatal("gateway used stale grant")
			}
			if _, e := file(context.Background(), ref, "shell", true); e == nil {
				t.Fatal("worker used stale grant")
			}
		})
	}
}

func TestExecutionAuthorityUsesCurrentPolicyAndMasksStaleContext(t *testing.T) {
	g, ref := executionFixture(t)
	authority, e := g.AuthorizeExecution(context.Background(), ref, "shell", true)
	if e != nil {
		t.Fatal(e)
	}
	if authority.Actor.Role != "member" || authority.Policy.AllowHostCommands {
		t.Fatal("team execution inherited host admin")
	}
	authority.Policy.AllowedHarnesses[0] = "changed"
	if g.state.Policy.AllowedHarnesses[0] == "changed" {
		t.Fatal("authority aliases durable policy")
	}
	g.state.Policy.SessionMemoryMiB = 2048
	fresh, e := g.AuthorizeExecution(context.Background(), ref, "shell", true)
	if e != nil || fresh.Policy.SessionMemoryMiB != 2048 {
		t.Fatal("policy not refreshed")
	}
	stale := WithTeamAccess(withTeamRuntime(withActor(context.Background(), User{ID: "forged", Role: "super-admin"}, g.state.Policy), "foreign"), TeamAccess{Team: Team{ID: "foreign"}})
	personal := ExecutionAuthority{Actor: g.state.Users["owner"].User, Policy: g.state.Policy}
	ctx := WithExecutionAuthority(stale, personal)
	u, _, ok := ActorFromContext(ctx)
	if !ok || u.ID != "owner" {
		t.Fatal("fresh actor missing")
	}
	if _, ok := TeamAccessFromContext(ctx); ok || TeamRuntimeIDFromContext(ctx) != "" {
		t.Fatal("stale team survives fresh authority")
	}
}

func TestFileExecutionAuthorityRefusesForeignScopeAndLocal(t *testing.T) {
	g, ref := executionFixture(t)
	file := fileExecutionFixture(t, g, ref)
	for _, bad := range []ExecutionActorRef{{Local: true, ProjectID: "project"}, {UserID: ref.UserID, ProjectID: "project"}, {UserID: ref.UserID, TeamID: "team-vutsrqponmlkjihgfedcba", ProjectID: "project"}, {UserID: "foreign", TeamID: ref.TeamID, ProjectID: "project"}} {
		if _, e := file(context.Background(), bad, "shell", false); e == nil {
			t.Fatalf("foreign identity accepted: %+v", bad)
		}
	}
	personal, e := NewFileExecutionAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), UserID: "owner"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e := personal(context.Background(), ExecutionActorRef{UserID: "owner", ProjectID: "own-project"}, "shell", false); e != nil {
		t.Fatal(e)
	}
	if _, e := personal(context.Background(), ref, "shell", false); e == nil {
		t.Fatal("personal worker accepted team authority")
	}
}

func TestExecutionAuthorityBillingBindingAndExpiry(t *testing.T) {
	g, ref := executionFixture(t)
	config := strings.Repeat("a", 64)
	g.managedTeamID = ref.TeamID
	g.billing = &teamBillingRuntime{configID: config}
	row := g.state.Teams[ref.TeamID]
	row.Billing = &TeamBilling{ConfigID: config, Revision: 1, Status: "active", CustomerID: "customer", SubscriptionID: "subscription", PeriodEnd: time.Now().Add(time.Hour).Unix()}
	g.state.Teams[ref.TeamID] = row
	saveExecutionFixture(t, g)
	scope := FileExecutionAuthorityScope{IdentityPath: filepath.Join(g.dir, "identity.json"), TeamID: ref.TeamID, ManagedTeamID: ref.TeamID, BillingConfigID: config}
	file, e := NewFileExecutionAuthorizer(scope)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = file(context.Background(), ref, "shell", true); e != nil {
		t.Fatal(e)
	}
	if _, e = fileExecutionFixture(t, g, ref)(context.Background(), ref, "shell", true); e == nil {
		t.Fatal("lost billing configuration downgraded to self-hosted")
	}
	row.Billing.PeriodEnd = time.Now().Add(-time.Second).Unix()
	saveExecutionFixture(t, g)
	if _, e = g.AuthorizeExecution(context.Background(), ref, "shell", true); e == nil {
		t.Fatal("expired billing accepted")
	}
	if _, e = file(context.Background(), ref, "shell", true); e == nil {
		t.Fatal("worker accepted expired billing")
	}
	row.Billing.PeriodEnd = time.Now().Add(time.Hour).Unix()
	row.Billing.ConfigID = strings.Repeat("b", 64)
	saveExecutionFixture(t, g)
	if _, e = file(context.Background(), ref, "shell", true); e == nil {
		t.Fatal("foreign merchant binding accepted")
	}
}

func TestFileExecutionAuthorityRejectsMissingCorruptAndLinkedState(t *testing.T) {
	for _, kind := range []string{"missing", "corrupt", "oversize", "symlink", "hardlink", "public"} {
		t.Run(kind, func(t *testing.T) {
			g, ref := executionFixture(t)
			file := fileExecutionFixture(t, g, ref)
			path := filepath.Join(g.dir, "identity.json")
			switch kind {
			case "missing":
				os.Remove(path)
			case "corrupt":
				os.WriteFile(path, []byte("{}"), 0600)
			case "oversize":
				os.WriteFile(path, []byte(strings.Repeat(" ", (8<<20)+1)), 0600)
			case "symlink":
				target := filepath.Join(g.dir, "copy")
				os.Rename(path, target)
				if e := os.Symlink(target, path); e != nil {
					t.Skipf("symlink unavailable: %v", e)
				}
			case "hardlink":
				if e := os.Link(path, filepath.Join(g.dir, "alias")); e != nil {
					t.Skipf("hardlink unavailable: %v", e)
				}
			case "public":
				if runtime.GOOS == "windows" {
					t.Skip("POSIX mode test; protected Windows DACL checked natively")
				}
				os.Chmod(path, 0644)
			}
			if _, e := file(context.Background(), ref, "shell", true); e == nil {
				t.Fatal("unsafe state accepted")
			}
		})
	}
}

func TestFileExecutionAuthorityRejectsSymlinkDirectory(t *testing.T) {
	g, ref := executionFixture(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if e := os.Symlink(g.dir, alias); e != nil {
		t.Skipf("symlink unavailable: %v", e)
	}
	file, e := NewFileExecutionAuthorizer(FileExecutionAuthorityScope{IdentityPath: filepath.Join(alias, "identity.json"), TeamID: ref.TeamID})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = file(context.Background(), ref, "shell", false); e == nil {
		t.Fatal("linked authority directory accepted")
	}
}

func TestExecutionAuthorityCaptureStoresOnlyIdentity(t *testing.T) {
	g, ref := executionFixture(t)
	a, e := g.AuthorizeExecution(context.Background(), ref, "shell", true)
	if e != nil {
		t.Fatal(e)
	}
	got, e := CaptureExecutionActor(WithExecutionAuthority(context.Background(), a), ref.ProjectID)
	if e != nil || got != ref {
		t.Fatalf("wrong durable reference: %+v %v", got, e)
	}
	data, _ := json.Marshal(got)
	for _, secret := range []string{"role", "policy", "grants", "token"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("queue retained authority snapshot")
		}
	}
}
