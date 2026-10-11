package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type coordinatorDeploymentProvider struct {
	git                                 *coordinatorGitFixture
	foreignServer                       atomic.Bool
	mu                                  sync.Mutex
	projects, applications, deployments int
	commit                              string
	lostResult                          bool
	afterProject, afterApplication      func()
	deny                                atomic.Bool
	public                              atomic.Bool
	foreign                             atomic.Bool
	health                              atomic.Bool
}

func (f *coordinatorDeploymentProvider) counts() (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.projects, f.applications, f.deployments
}

// Provider HTTP is controlled; native Git, explicit integration, native checks,
// private journals and actual TLS health requests exercise production joins.
// The injected health mapper requires the exact planned public origin and uses
// a verified httptest certificate. It is not live provider/deployment evidence.
func coordinatorDeploymentFixture(t *testing.T, lostResult bool) (*Server, project.Project, http.Handler, context.Context, CoordinatorContinuation, *coordinatorDeploymentProvider) {
	t.Helper()
	f := &coordinatorDeploymentProvider{lostResult: lostResult}
	f.public.Store(true)
	f.health.Store(true)
	s, p, h, ctx, out, _ := coordinatorPreviewFixture(t, "publish", false, func(s *Server, p project.Project, h http.Handler, ctx context.Context, request *coordinatorContinuationRequest) {
		remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if r.Header.Get("Authorization") != "Bearer "+testCoolifyToken {
				t.Error("provider credential missing")
			}
			if r.Method != http.MethodGet {
				s.mu.RLock()
				rows, err := s.readContinuations(p, "alice")
				s.mu.RUnlock()
				if err != nil || len(rows.Rows) != 1 || rows.Rows[0].Delivery == nil || rows.Rows[0].Delivery.Deployment == nil || rows.Rows[0].Delivery.Deployment.ReleaseID != coordinatorDeploymentID(rows.Rows[0].ID) {
					t.Error("provider effect preceded exact durable deployment identity")
					w.WriteHeader(409)
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			switch r.Method + " " + r.URL.Path {
			case "GET /api/v1/teams/current":
				w.Write([]byte(`{"id":1}`))
			case "GET /api/v1/projects":
				w.Write([]byte(`[]`))
			case "POST /api/v1/projects":
				f.projects++
				w.Write([]byte(`{"uuid":"coordinator-project"}`))
				if f.afterProject != nil {
					f.afterProject()
				}
			case "POST /api/v1/applications/public":
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				f.commit, _ = body["git_commit_sha"].(string)
				f.applications++
				if body["instant_deploy"] != false || body["is_auto_deploy_enabled"] != false {
					t.Error("application creation implicitly deployed")
				}
				w.Write([]byte(`{"uuid":"coordinator-app"}`))
				if f.afterApplication != nil {
					f.afterApplication()
				}
			case "POST /api/v1/deploy":
				f.deployments++
				if f.lostResult {
					w.WriteHeader(502)
					w.Write([]byte(`{"message":"controlled lost acknowledgement after provider effect"}`))
					return
				}
				w.Write([]byte(`{"deployments":[{"resource_uuid":"coordinator-app","deployment_uuid":"coordinator-deployment"}]}`))
			case "GET /api/v1/deployments/coordinator-deployment":
				json.NewEncoder(w).Encode(map[string]string{"deployment_uuid": "coordinator-deployment", "status": "finished", "commit": f.commit})
			case "GET /api/v1/applications/coordinator-app":
				commit := f.commit
				if f.foreign.Load() {
					commit = strings.Repeat("f", 40)
				}
				server := "coordinator-server"
				if f.foreignServer.Load() {
					server = "foreign-server"
				}
				json.NewEncoder(w).Encode(map[string]string{"uuid": "coordinator-app", "server_uuid": server, "status": "running", "git_repository": "https://github.com/example/coordinator.git", "git_branch": "main", "git_commit_sha": commit, "fqdn": "https://app.example.com"})
			default:
				t.Errorf("unexpected provider request %s %s", r.Method, r.URL.Path)
				w.WriteHeader(404)
			}
		}))
		t.Cleanup(remote.Close)
		target := DeploymentTarget{ID: "coordinator-target", Name: "Existing fixture host", BaseURL: remote.URL, Token: testCoolifyToken, ServerUUID: "coordinator-server"}
		s.mu.Lock()
		s.targets[target.ID] = target
		s.mu.Unlock()
		if err := atomicJSON(filepath.Join(s.stateDir, "deployment-targets.json"), []storedTarget{{Target: target, Token: target.Token}}); err != nil {
			t.Fatal(err)
		}
		s.SetCoordinatorDeploymentAuthorizer(func(ctx context.Context, ref management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
			if ctx.Err() != nil || ref.UserID != "alice" || ref.ProjectID != p.ID || ref.Local || f.deny.Load() {
				return management.ExecutionAuthority{}, errors.New("current Deploy revoked")
			}
			return management.ExecutionAuthority{Actor: management.User{ID: "alice", Role: "member"}, Policy: management.Policy{AllowPublicPublishing: f.public.Load()}}, nil
		})
		health := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				t.Error("wrong health path")
			}
			if !f.health.Load() {
				w.WriteHeader(503)
				return
			}
			w.WriteHeader(204)
		}))
		t.Cleanup(health.Close)
		s.cloudReleaseHealthObserver = func(ctx context.Context, address string) error {
			if address != "https://app.example.com/health" {
				return errors.New("foreign planned health origin")
			}
			return probeReleaseHealth(ctx, health.URL+"/health", health.Client())
		}
		plan := CoordinatorDeploymentPlan{TargetID: target.ID, Repository: "https://github.com/example/coordinator.git", Branch: "main", Domain: "https://app.example.com", Port: 3000, HealthPath: "/health", Dockerfile: "Dockerfile", Source: "public", DataClientFramework: "none"}
		before := auditTree(t, s.stateDir)
		decodeEvidenceResponse(t, journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/deployment-plan", plan, 200).Body.Bytes(), &plan)
		if plan.AcknowledgeRepositoryPush || plan.AcknowledgePublic || plan.AcknowledgeDeploymentCosts || plan.TargetFingerprint != targetFingerprint(target) || plan.ServerUUID != target.ServerUUID || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
			t.Fatal("preparation granted consent or mutated state", plan)
		}
		a, b, c := f.counts()
		if a+b+c != 0 {
			t.Fatal("plan preparation allocated resources")
		}
		plan.AcknowledgeRepositoryPush, plan.AcknowledgePublic, plan.AcknowledgeDeploymentCosts = true, true, true
		request.DeploymentPlan = &plan
		var policy CoordinatorPolicy
		decodeEvidenceResponse(t, journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/policy", map[string]any{"expectedRevision": request.ExpectedPolicyRevision, "mode": "review", "allowedTargets": []string{"plan"}, "executionMode": "reviewed-tasks", "previewMode": "checked-source", "deploymentMode": "existing-target"}, 200).Body.Bytes(), &policy)
		view, err := s.OrchestratorView(ctx, p.ID, 0)
		if err != nil {
			t.Fatal(err)
		}
		request.ExpectedPolicyRevision, request.ExpectedContextDigest = policy.Revision, view.Context.Digest
	})
	f.git = installCoordinatorGitFixture(t, s, p)
	return s, p, h, ctx, out, f
}

func coordinatorDeploymentPreviewedFixture(t *testing.T, s *Server, p project.Project, h http.Handler, ctx context.Context, out CoordinatorContinuation) coordinatorContinuationRecord {
	t.Helper()
	row := coordinatorReservePreviewFixture(t, s, p, ctx, coordinatorPreviewCheckedFixture(t, s, p, h, ctx, out))
	if err := s.invokeCoordinatorPreview(ctx, p, row); err != nil {
		t.Fatal(err)
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil {
		t.Fatal(err)
	}
	check, err := coordinatorPreviewCheck(run, row)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := s.SnapshotProjectRunRelease(ctx, p.ID, row.Delivery.Preview.ReleaseID, "local", "preview", run.Input.Approvals, check)
	if err != nil {
		t.Fatal(err)
	}
	row, err = s.persistCoordinatorPreview(ctx, p, row, "ready", "Controlled interruption after exact preview observation and before provider reservation.", &evidence)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func coordinatorReserveDeploymentFixture(t *testing.T, s *Server, p project.Project, ctx context.Context, row coordinatorContinuationRecord) coordinatorContinuationRecord {
	t.Helper()
	row, err := s.continueCoordinatorGitPublication(ctx, p, row)
	if err != nil || !coordinatorGitPublished(row) {
		t.Fatal("exact source was not published", row, err)
	}
	now := coordinatorPreviewNow()
	row.Delivery.Deployment = &CoordinatorDeploymentAction{ActionID: row.ID + "-deploy", ReleaseID: coordinatorDeploymentID(row.ID), Status: "pending", StartedAt: now, UpdatedAt: now}
	row, err = s.persistCoordinatorDeployment(ctx, p, row, "pending", "Controlled process-loss boundary before provider admission.", nil)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func waitCoordinatorDeploymentFixture(t *testing.T, s *Server, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.RLock()
		live := s.coordinatorActions[id+"-deploy"]
		s.mu.RUnlock()
		if !live {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("controlled deployment did not finish")
}

func TestCoordinatorDeploymentHTTPContinuesChecksPreviewAndExactPublicRelease(t *testing.T) {
	s, p, h, _, out, f := coordinatorDeploymentFixture(t, false)
	coordinatorIntegrateFixture(t, s, p, h, out)
	out = coordinatorDeliveryAdvance(t, h, p, out, 200)
	finishEvidenceRun(t, s, p.ID, out.Delivery.CheckID)
	out = coordinatorDeliveryAdvance(t, h, p, out, 200)
	waitCoordinatorPreviewFixture(t, s, out.ID)
	out = coordinatorDeliveryAdvance(t, h, p, out, 200)
	if out.Status != "deploying" || out.Delivery.Deployment == nil || out.Delivery.Preview.Evidence == nil {
		t.Fatal(out)
	}
	waitCoordinatorDeploymentFixture(t, s, out.ID)
	before := auditTree(t, s.stateDir)
	journeyRequest(t, h, "GET", "/v1/product/projects/"+p.ID+"/orchestrator/continuations/"+out.ID, nil, 200)
	if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("GET executed or repaired deployment")
	}
	out = coordinatorDeliveryAdvance(t, h, p, out, 200)
	a, b, c := f.counts()
	if out.Status != "deployed" || out.Delivery.Deployment.Evidence == nil || out.Delivery.Deployment.Evidence.Outcome != "publish" || out.Delivery.Deployment.Evidence.URL != "https://app.example.com" || a != 1 || b != 1 || c != 1 {
		t.Fatal("exact provider/TLS lifecycle incomplete", out, a, b, c)
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, out.RunID)
	if err != nil || len(run.Releases) != 1 || run.Releases[0].ReleaseID != out.Delivery.Deployment.ReleaseID || run.State == "complete" || run.Acceptance != nil {
		t.Fatal("deployment claimed full application acceptance", run, err)
	}
}

func TestCoordinatorDeploymentRestartUsesExactReceiptWithoutProviderReplay(t *testing.T) {
	for _, lost := range []bool{false, true} {
		t.Run(map[bool]string{false: "response-lost-after-exact-ID", true: "provider-acknowledgement-lost"}[lost], func(t *testing.T) {
			s, p, h, ctx, out, f := coordinatorDeploymentFixture(t, lost)
			row := coordinatorReserveDeploymentFixture(t, s, p, ctx, coordinatorDeploymentPreviewedFixture(t, s, p, h, ctx, out))
			err := s.invokeCoordinatorDeployment(ctx, p, row)
			if (err != nil) != lost {
				t.Fatal("unexpected controlled provider outcome", err)
			}
			reopened, handler := coordinatorDeliveryReopen(t, s)
			reopened.SetPreviewRuntime(s.PreviewRuntime())
			reopened.SetCoordinatorDeploymentAuthorizer(s.coordinatorDeploymentAuthorize)
			reopened.cloudReleaseHealthObserver = s.cloudReleaseHealthObserver
			out = coordinatorDeliveryAdvance(t, handler, p, row.CoordinatorContinuation, 200)
			a, b, c := f.counts()
			expected := "deployed"
			if lost {
				expected = "unknown"
			}
			if out.Status != expected || a != 1 || b != 1 || c != 1 || lost && out.Delivery.Deployment.Evidence != nil {
				t.Fatal("restart replayed/adopted unconfirmed provider result", out, a, b, c)
			}
		})
	}
}

func TestCoordinatorDeploymentPendingMissingReleaseNeverAdoptsEquivalentRequest(t *testing.T) {
	s, p, h, ctx, out, f := coordinatorDeploymentFixture(t, false)
	row := coordinatorReserveDeploymentFixture(t, s, p, ctx, coordinatorDeploymentPreviewedFixture(t, s, p, h, ctx, out))
	plan := row.DeploymentPlan
	_, _, err := s.reserveCloudRelease(CloudRelease{Release: Release{ID: "cloud-rel-unrelated", ProjectID: p.ID, Target: plan.TargetID, Commit: row.Delivery.Commit, Status: "preparing"}, CloudReleaseRequest: coordinatorDeploymentRequest(plan, row.Delivery.Commit), TargetFingerprint: plan.TargetFingerprint, ServerUUID: plan.ServerUUID})
	if err != nil {
		t.Fatal(err)
	}
	reopened, handler := coordinatorDeliveryReopen(t, s)
	reopened.SetPreviewRuntime(s.PreviewRuntime())
	reopened.SetCoordinatorDeploymentAuthorizer(s.coordinatorDeploymentAuthorize)
	reopened.cloudReleaseHealthObserver = s.cloudReleaseHealthObserver
	out = coordinatorDeliveryAdvance(t, handler, p, row.CoordinatorContinuation, 200)
	a, b, c := f.counts()
	if out.Status != "unknown" || out.Delivery.Deployment.Evidence != nil || a+b+c != 0 {
		t.Fatal("pending absence adopted or replayed another release", out, a, b, c)
	}
}

func TestCoordinatorDeploymentRechecksCurrentAuthorityBeforeEachProviderMutation(t *testing.T) {
	for _, drift := range []string{"deploy-access", "public-policy", "source", "target"} {
		t.Run(drift, func(t *testing.T) {
			s, p, h, ctx, out, f := coordinatorDeploymentFixture(t, false)
			row := coordinatorReserveDeploymentFixture(t, s, p, ctx, coordinatorDeploymentPreviewedFixture(t, s, p, h, ctx, out))
			f.afterProject = func() {
				switch drift {
				case "deploy-access":
					f.deny.Store(true)
				case "public-policy":
					f.public.Store(false)
				case "source":
					if err := os.WriteFile(filepath.Join(p.Path, "result.txt"), []byte("changed after first provider effect"), 0600); err != nil {
						t.Error(err)
					}
				case "target":
					s.mu.Lock()
					target := s.targets[row.DeploymentPlan.TargetID]
					target.ServerUUID = "changed-server"
					s.targets[target.ID] = target
					s.mu.Unlock()
				}
			}
			if err := s.invokeCoordinatorDeployment(ctx, p, row); err == nil {
				t.Fatal("drift admitted all provider effects")
			}
			a, b, c := f.counts()
			if a != 1 || b != 0 || c != 0 {
				t.Fatal("current authority/source was not checked at next mutation", a, b, c)
			}
			releases, err := s.readCloudReleases(p.ID)
			if err != nil || len(releases) != 1 || releases[0].ProviderProjectID != "coordinator-project" || releases[0].Status != "attention" {
				t.Fatal("uncertain provider resource was discarded", releases, err)
			}
		})
	}
}

func TestCoordinatorDeploymentReadbackRejectsForeignApplicationAndFailedHTTPS(t *testing.T) {
	for _, drift := range []string{"foreign-source", "foreign-server", "health", "public-policy"} {
		t.Run(drift, func(t *testing.T) {
			s, p, h, ctx, out, f := coordinatorDeploymentFixture(t, false)
			row := coordinatorReserveDeploymentFixture(t, s, p, ctx, coordinatorDeploymentPreviewedFixture(t, s, p, h, ctx, out))
			if err := s.invokeCoordinatorDeployment(ctx, p, row); err != nil {
				t.Fatal(err)
			}
			switch drift {
			case "foreign-source":
				f.foreign.Store(true)
			case "foreign-server":
				f.foreignServer.Store(true)
			case "health":
				f.health.Store(false)
			case "public-policy":
				f.public.Store(false)
			}
			next, err := s.continueCoordinatorDeployment(ctx, p, row)
			if err == nil && next.Status != "unknown" || next.Delivery.Deployment.Evidence != nil {
				t.Fatal("unverified public result claimed deployment", next, err)
			}
			run, _ := s.projectRuns.Get(p.ID, p.Path, row.RunID)
			if len(run.Releases) != 0 {
				t.Fatal("stale or unhealthy result joined")
			}
			a, b, c := f.counts()
			if a != 1 || b != 1 || c != 1 {
				t.Fatal("readback replayed provider effects", a, b, c)
			}
		})
	}
}

func TestCoordinatorDeploymentDefaultHealthRetainsPublicNetworkGuards(t *testing.T) {
	s, _ := journeyFixture(t)
	if s.cloudReleaseHealthObserver != nil {
		t.Fatal("production default replaces public health verifier")
	}
	if err := s.cloudReleaseHealth(context.Background(), "https://127.0.0.1/health"); err == nil {
		t.Fatal("default delivery observer bypassed public DNS/IP policy")
	}
}
