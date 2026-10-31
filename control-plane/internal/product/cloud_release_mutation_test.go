package product

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type manualReleaseTransport func(*http.Request) (*http.Response, error)

func (f manualReleaseTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type manualReleaseFixture struct {
	server          *Server
	project         project.Project
	target          DeploymentTarget
	row             CloudRelease
	ctx             context.Context
	client          *CoolifyClient
	mutations       int
	afterRead       func()
	deny            bool
	changedActor    bool
	allocationCalls []bool
	observedBranch  string
}

func newManualReleaseFixture(t *testing.T, allocate bool) *manualReleaseFixture {
	t.Helper()
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "manual-release-guard")
	f := &manualReleaseFixture{server: s, project: p, ctx: homeActor("alice"), observedBranch: "main"}
	f.target = DeploymentTarget{ID: "target-test", ServerUUID: "server-test", BaseURL: "https://provider.invalid", Token: "fixture-token"}
	f.row = CloudRelease{Release: Release{ID: "cloud-rel-guard", ProjectID: p.ID, Commit: strings.Repeat("a", 40), Target: f.target.ID, Status: "ready"}, CloudReleaseRequest: CloudReleaseRequest{TargetID: f.target.ID, Repository: "https://github.com/example/project.git", Branch: "main", Commit: strings.Repeat("a", 40), Domain: "https://app.example.com", Port: 3000, HealthPath: "/health", Dockerfile: "Dockerfile"}, TargetFingerprint: targetFingerprint(f.target), ServerUUID: f.target.ServerUUID, ApplicationID: "app-one", ProviderProjectID: "provider-project"}
	s.targets[f.target.ID] = f.target
	if err := atomicJSON(s.cloudReleaseFile(p.ID), []CloudRelease{f.row}); err != nil {
		t.Fatal(err)
	}
	actor, policy, _ := management.ActorFromContext(f.ctx)
	policy.AllowPublicPublishing = true
	s.SetCoordinatorDeploymentAuthorizer(func(_ context.Context, ref management.ExecutionActorRef, allocation bool) (management.ExecutionAuthority, error) {
		f.allocationCalls = append(f.allocationCalls, allocation)
		if f.deny || ref.UserID != "alice" || ref.ProjectID != p.ID || ref.Local {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		current := actor
		if f.changedActor {
			current.ID = "other-user"
		}
		return management.ExecutionAuthority{Actor: current, Policy: policy}, nil
	})
	base, _ := url.Parse(f.target.BaseURL)
	f.client = &CoolifyClient{base: base, token: f.target.Token, http: &http.Client{Transport: manualReleaseTransport(func(r *http.Request) (*http.Response, error) {
		body := "{}"
		if r.Method == http.MethodGet {
			payload, _ := json.Marshal(map[string]string{"uuid": f.row.ApplicationID, "server_uuid": f.row.ServerUUID, "git_repository": f.row.Repository, "git_branch": f.observedBranch, "git_commit_sha": f.row.Release.Commit, "fqdn": f.row.Domain})
			body = string(payload)
			if f.afterRead != nil {
				f.afterRead()
			}
		} else {
			f.mutations++
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}}
	if err := s.bindManualCloudReleaseMutations(f.ctx, p, f.target, &f.row, allocate, f.client); err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *manualReleaseFixture) mutate() error {
	return f.client.request(f.ctx, http.MethodPatch, "/applications/"+f.row.ApplicationID, nil, map[string]string{"name": "same-owned-app"}, nil)
}
func (f *manualReleaseFixture) changeConfiguration(t *testing.T) {
	t.Helper()
	path, err := f.server.privateApplicationStatePath("application-config-" + f.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := atomicJSON(path, applicationConfiguration{ProjectID: f.project.ID, Revision: 1, Values: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
}

func TestManualCloudMutationRechecksOriginalActorAfterOwnershipRead(t *testing.T) {
	for _, which := range []string{"revoked", "different-actor"} {
		t.Run(which, func(t *testing.T) {
			f := newManualReleaseFixture(t, true)
			f.afterRead = func() {
				if which == "revoked" {
					f.deny = true
				} else {
					f.changedActor = true
				}
			}
			if err := f.mutate(); err == nil || f.mutations != 0 {
				t.Fatal("late authority change reached provider mutation", err, f.mutations)
			}
			rows, err := f.server.readCloudReleases(f.project.ID)
			if err != nil || len(rows) != 1 || rows[0].ApplicationID != f.row.ApplicationID {
				t.Fatal("original ownership was lost", rows, err)
			}
		})
	}
}
func TestManualCloudSecondMutationKeepsOriginalTargetAndConfiguration(t *testing.T) {
	for _, which := range []string{"target", "configuration"} {
		t.Run(which, func(t *testing.T) {
			f := newManualReleaseFixture(t, true)
			if err := f.mutate(); err != nil || f.mutations != 1 {
				t.Fatal(err, f.mutations)
			}
			f.afterRead = func() {
				if which == "target" {
					f.server.mu.Lock()
					next := f.target
					next.Token = "replacement-token"
					f.server.targets[next.ID] = next
					f.server.mu.Unlock()
				} else {
					f.changeConfiguration(t)
				}
			}
			if err := f.mutate(); err == nil || f.mutations != 1 {
				t.Fatal("changed target/config reached second mutation", err, f.mutations)
			}
		})
	}
}
func TestManualCloudStopRetainsControlWithoutAllocationOrCurrentConfiguration(t *testing.T) {
	f := newManualReleaseFixture(t, false)
	f.changeConfiguration(t)
	if err := f.mutate(); err != nil || f.mutations != 1 {
		t.Fatal("retained stop was made dependent on a new-build config", err, f.mutations)
	}
	for _, allocate := range f.allocationCalls {
		if allocate {
			t.Fatal("stop requested new allocation authority")
		}
	}
}
func TestManualCloudBranchUpdateRequiresExactRetainedOwnedPriorSource(t *testing.T) {
	f := newManualReleaseFixture(t, true)
	prior := f.row
	f.row.Branch = "release"
	prior.ID = "cloud-rel-prior"
	if err := atomicJSON(f.server.cloudReleaseFile(f.project.ID), []CloudRelease{f.row, prior}); err != nil {
		t.Fatal(err)
	}
	if err := f.mutate(); err != nil || f.mutations != 1 {
		t.Fatal("known original branch could not be updated explicitly", err, f.mutations)
	}
	f.observedBranch = "foreign-branch"
	if err := f.mutate(); err == nil || f.mutations != 1 {
		t.Fatal("foreign provider source was adopted", err, f.mutations)
	}
}
func TestManualCloudGuardDoesNotReplaceCoordinatorMutationAdmission(t *testing.T) {
	s, p, h, ctx, out, provider := coordinatorDeploymentFixture(t, false)
	row := coordinatorReserveDeploymentFixture(t, s, p, ctx, coordinatorDeploymentPreviewedFixture(t, s, p, h, ctx, out))
	if err := s.invokeCoordinatorDeployment(ctx, p, row); err != nil {
		t.Fatal(err)
	}
	projects, applications, deployments := provider.counts()
	if projects != 1 || applications != 1 || deployments != 1 {
		t.Fatal("coordinator provider path changed", projects, applications, deployments)
	}
	saved, _, err := s.ownedCloudRelease(p.ID, row.Delivery.Deployment.ReleaseID)
	if err != nil || saved.CoordinatorOriginDigest != coordinatorDeploymentBinding(row) {
		t.Fatal("original coordinator receipt was replaced", saved, err)
	}
}
