package product

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Native Git review/integration, committed archives, checks and HTTP health are
// real. Only the admitted container runtime is controlled; this is not Docker,
// provider, public deployment or application behavior acceptance evidence.
func coordinatorPreviewFixture(t *testing.T, outcome string, lostResult bool) (*Server, project.Project, http.Handler, context.Context, CoordinatorContinuation, *atomic.Int32) {
	t.Helper()
	s, p, h, ctx, request, _ := coordinatorContinuationFixture(t)
	if err := os.WriteFile(filepath.Join(p.Path, "Dockerfile"), []byte("FROM scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, h, "POST", base+"/git/commit", reviewedCommitRequest(t, h, base+"/git/commit", "Review local preview build configuration"), 200)
	var policy CoordinatorPolicy
	decodeEvidenceResponse(t, journeyRequest(t, h, "POST", base+"/orchestrator/policy", map[string]any{"expectedRevision": request.ExpectedPolicyRevision, "mode": "review", "allowedTargets": []string{"plan"}, "executionMode": "reviewed-tasks", "deploymentMode": "manual", "previewMode": "checked-source"}, 200).Body.Bytes(), &policy)
	turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Continue the checked application through its local preview", "")
	answer := OrchestratorAnswer{Summary: "Use the saved checked-source preview policy.", Options: []string{}, Impact: []string{}, NextAction: "build"}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	request.OriginTurnID, request.ExpectedContextDigest, request.ExpectedPolicyRevision = turn.ID, view.Context.Digest, policy.Revision
	request.Run.Outcome = outcome
	request.PreviewPlan = &CoordinatorPreviewPlan{Dockerfile: "Dockerfile", Port: 3000, ExpectedConfigurationRevision: 0, AcknowledgePreviewBuild: true}
	s, p, h, ctx, row, _ := coordinatorDeliveryRequestFixture(t, s, p, h, ctx, request)
	starts := &atomic.Int32{}
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("owned fixture health")) }))
	t.Cleanup(health.Close)
	s.SetPreviewRuntime(previewruntime.Runtime{
		CaptureActor: management.CaptureDeploymentActor,
		Start: func(ctx context.Context, req previewruntime.Request, publish func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
			starts.Add(1)
			s.mu.RLock()
			rows, err := s.readContinuations(p, "alice")
			s.mu.RUnlock()
			if err != nil || len(rows.Rows) != 1 || rows.Rows[0].Delivery == nil || rows.Rows[0].Delivery.Preview == nil || rows.Rows[0].Delivery.Preview.ReleaseID != req.ReleaseID || rows.Rows[0].Delivery.Preview.Status != "pending" {
				return previewruntime.Outcome{}, errors.New("preview executed before exact durable reservation")
			}
			if req.SourceDirectory == p.Path || req.SourceDigest != rows.Rows[0].Delivery.SourceDigest || req.Actor.UserID != "alice" || req.Actor.ProjectID != p.ID {
				return previewruntime.Outcome{}, errors.New("preview lost checked source or stored actor")
			}
			for _, name := range []string{"Dockerfile", "result.txt", "LUCI.md"} {
				if _, err := os.Stat(filepath.Join(req.SourceDirectory, name)); err != nil {
					return previewruntime.Outcome{}, err
				}
			}
			if err := req.ValidateSource(ctx); err != nil {
				return previewruntime.Outcome{}, err
			}
			out := previewruntime.Outcome{Identity: req.Identity, Phase: "running", URL: health.URL, ResourcesHeld: true}
			out.BuildAllocationID, out.AllocationID, out.EngineID, out.InstanceID = "build-"+req.ReleaseID, "serve-"+req.ReleaseID, "controlled-engine", "controlled-instance"
			out.ContainerID, out.Image = strings.Repeat("a", 64), "sha256:"+strings.Repeat("b", 64)
			if err := publish(out); err != nil {
				return out, err
			}
			if lostResult {
				return out, errors.New("controlled transport lost after running receipt, before ready publication")
			}
			return out, nil
		},
		Observe: func(ctx context.Context, identity previewruntime.Identity) (previewruntime.Outcome, error) {
			s.mu.RLock()
			defer s.mu.RUnlock()
			rows, err := s.readReleases(p.ID)
			for _, row := range rows {
				if row.ID == identity.ReleaseID && row.Runtime != nil && row.Runtime.Identity == identity {
					return *row.Runtime, err
				}
			}
			return previewruntime.Outcome{}, errors.New("exact owned preview absent")
		},
		Stop: func(context.Context, previewruntime.Identity, func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
			return previewruntime.Outcome{}, errors.New("observation must not stop resources")
		},
	})
	return s, p, h, ctx, row, starts
}

func coordinatorPreviewCheckedFixture(t *testing.T, s *Server, p project.Project, h http.Handler, ctx context.Context, out CoordinatorContinuation) coordinatorContinuationRecord {
	t.Helper()
	coordinatorIntegrateFixture(t, s, p, h, out)
	row, run := coordinatorReserveCheckFixture(t, s, p, ctx, out.ID)
	if err := s.invokeCoordinatorChecks(ctx, p, row, run); err != nil {
		t.Fatal(err)
	}
	check := finishEvidenceRun(t, s, p.ID, row.Delivery.CheckID)
	evidence, err := s.SnapshotProjectRunCheck(ctx, p.ID, check.ID, run.Input.Approvals, run.Input.Requirements)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.projectRuns.JoinCheck(p.ID, p.Path, run.ID, run.Revision, evidence); err != nil {
		t.Fatal(err)
	}
	row, err = s.persistCoordinatorDelivery(ctx, p, row, "verified", "Controlled interruption after exact check join and before preview reservation.")
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func coordinatorReservePreviewFixture(t *testing.T, s *Server, p project.Project, ctx context.Context, row coordinatorContinuationRecord) coordinatorContinuationRecord {
	t.Helper()
	now := coordinatorPreviewNow()
	row.Delivery.Preview = &CoordinatorPreviewAction{ActionID: row.ID + "-preview", ReleaseID: coordinatorPreviewID(row.ID), Status: "pending", StartedAt: now, UpdatedAt: now}
	row, err := s.persistCoordinatorPreview(ctx, p, row, "pending", "Controlled process-loss boundary before preview invocation.", nil)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func waitCoordinatorPreviewFixture(t *testing.T, s *Server, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.RLock()
		live := s.coordinatorActions[id+"-preview"]
		s.mu.RUnlock()
		if !live {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("controlled preview did not finish")
}

func TestCoordinatorPreviewHTTPAdvancesExactCheckedSourceWithoutPublishing(t *testing.T) {
	for _, outcome := range []string{"preview", "publish"} {
		t.Run(outcome, func(t *testing.T) {
			s, p, h, _, out, starts := coordinatorPreviewFixture(t, outcome, false)
			coordinatorIntegrateFixture(t, s, p, h, out)
			out = coordinatorDeliveryAdvance(t, h, p, out, 200)
			finishEvidenceRun(t, s, p.ID, out.Delivery.CheckID)
			out = coordinatorDeliveryAdvance(t, h, p, out, 200)
			if out.Status != "previewing" || out.Delivery.Preview == nil || out.Delivery.Preview.ReleaseID != coordinatorPreviewID(out.ID) {
				t.Fatal(out)
			}
			waitCoordinatorPreviewFixture(t, s, out.ID)
			before := auditTree(t, s.stateDir)
			journeyRequest(t, h, "GET", "/v1/product/projects/"+p.ID+"/orchestrator/continuations/"+out.ID, nil, 200)
			journeyRequest(t, h, "GET", "/v1/product/projects/"+p.ID+"/releases", nil, 200)
			if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
				t.Fatal("GET repaired or executed preview")
			}
			out = coordinatorDeliveryAdvance(t, h, p, out, 200)
			if out.Status != "preview-ready" || out.Delivery.Status != "verified" || out.Delivery.Preview.Evidence == nil || out.Delivery.Preview.Evidence.Outcome != "preview" || starts.Load() != 1 {
				t.Fatal(out, starts.Load())
			}
			run, err := s.projectRuns.Get(p.ID, p.Path, out.RunID)
			if err != nil || run.Acceptance != nil || run.State == "complete" || outcome == "preview" && len(run.Releases) != 1 || outcome == "publish" && len(run.Releases) != 0 {
				t.Fatal("local health claimed wrong intended outcome", run, err)
			}
		})
	}
}

func TestCoordinatorPreviewRestartReconcilesRetainedRunningReceiptWithoutRebuild(t *testing.T) {
	s, p, h, ctx, out, starts := coordinatorPreviewFixture(t, "preview", true)
	row := coordinatorReservePreviewFixture(t, s, p, ctx, coordinatorPreviewCheckedFixture(t, s, p, h, ctx, out))
	if err := s.invokeCoordinatorPreview(ctx, p, row); err == nil {
		t.Fatal("controlled lost response unexpectedly confirmed")
	}
	releases, err := s.readReleases(p.ID)
	if err != nil || len(releases) != 1 || releases[0].Status == "ready" || releases[0].Runtime.Phase != "running" {
		t.Fatal("fixture failed to retain interrupted-ready boundary", releases, err)
	}
	reopened, handler := coordinatorDeliveryReopen(t, s)
	reopened.SetPreviewRuntime(s.PreviewRuntime())
	before := auditTree(t, s.stateDir)
	journeyRequest(t, handler, "GET", "/v1/product/projects/"+p.ID+"/releases", nil, 200)
	if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("retained observation rewrote release")
	}
	out = coordinatorDeliveryAdvance(t, handler, p, row.CoordinatorContinuation, 200)
	if out.Status != "preview-ready" || starts.Load() != 1 || out.Delivery.Preview.ReleaseID != releases[0].ID {
		t.Fatal("restart replayed or lost exact preview", out, starts.Load())
	}
	retained, _ := reopened.readReleases(p.ID)
	if !reflect.DeepEqual(retained, releases) {
		t.Fatal("joining observed preview rewrote retained runtime receipt")
	}
}

func TestCoordinatorPreviewPendingNeverAdoptsAnotherReleaseOrReplays(t *testing.T) {
	s, p, h, ctx, out, starts := coordinatorPreviewFixture(t, "preview", false)
	row := coordinatorReservePreviewFixture(t, s, p, ctx, coordinatorPreviewCheckedFixture(t, s, p, h, ctx, out))
	if err := s.storeRelease(storedRelease{Release: Release{ID: "rel-unrelated", ProjectID: p.ID, Target: "local", Commit: row.Delivery.Commit, Status: "ready"}}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		reopened, handler := coordinatorDeliveryReopen(t, s)
		reopened.SetPreviewRuntime(s.PreviewRuntime())
		out = coordinatorDeliveryAdvance(t, handler, p, row.CoordinatorContinuation, 200)
		if out.Status != "unknown" || out.Delivery.Preview.Evidence != nil || starts.Load() != 0 {
			t.Fatal("missing reservation adopted or replayed", out)
		}
		rows, _ := reopened.readContinuations(p, "alice")
		row = rows.Rows[0]
	}
}

func TestCoordinatorPreviewRejectsDriftBeforeBuildAndBeforeJoin(t *testing.T) {
	for _, boundary := range []string{"before-build", "before-join"} {
		for _, drift := range []string{"source", "configuration", "authority", "policy", "foreign-origin", "running", "foreign-runtime", "cancelled", "unhealthy", "authority-during-observe"} {
			if boundary == "before-build" && (drift == "foreign-origin" || drift == "running" || drift == "foreign-runtime" || drift == "cancelled" || drift == "unhealthy" || drift == "authority-during-observe") {
				continue
			}
			t.Run(boundary+"/"+drift, func(t *testing.T) {
				s, p, h, ctx, out, starts := coordinatorPreviewFixture(t, "preview", false)
				row := coordinatorReservePreviewFixture(t, s, p, ctx, coordinatorPreviewCheckedFixture(t, s, p, h, ctx, out))
				if boundary == "before-join" {
					if err := s.invokeCoordinatorPreview(ctx, p, row); err != nil {
						t.Fatal(err)
					}
				}
				switch drift {
				case "source":
					if err := os.WriteFile(filepath.Join(p.Path, "result.txt"), []byte("changed after check"), 0600); err != nil {
						t.Fatal(err)
					}
				case "configuration":
					path, err := s.privateApplicationStatePath("application-config-" + p.ID)
					if err != nil {
						t.Fatal(err)
					}
					if err = atomicJSON(path, applicationConfiguration{ProjectID: p.ID, Revision: 1, Values: map[string]string{"NAME": "new value"}}); err != nil {
						t.Fatal(err)
					}
				case "authority":
					runtime := s.PreviewRuntime()
					runtime.CaptureActor = func(context.Context, string) (management.ExecutionActorRef, error) {
						return management.ExecutionActorRef{}, errors.New("Deploy permission revoked")
					}
					s.SetPreviewRuntime(runtime)
				case "policy":
					journeyRequest(t, h, "POST", "/v1/product/projects/"+p.ID+"/orchestrator/policy", map[string]any{"expectedRevision": row.PolicyRevision, "mode": "review", "allowedTargets": []string{"plan"}, "executionMode": "reviewed-tasks", "deploymentMode": "manual", "previewMode": "manual"}, 200)
				case "foreign-origin", "cancelled":
					releases, _ := s.readReleases(p.ID)
					if drift == "cancelled" {
						releases[0].CancelRequested = true
					} else {
						releases[0].CoordinatorOriginDigest = strings.Repeat("f", 64)
					}
					if err := s.storeRelease(releases[0]); err != nil {
						t.Fatal(err)
					}
				case "authority-during-observe":
					runtime := s.PreviewRuntime()
					capture, observe := runtime.CaptureActor, runtime.Observe
					var revoked atomic.Bool
					runtime.CaptureActor = func(ctx context.Context, id string) (management.ExecutionActorRef, error) {
						if revoked.Load() {
							return management.ExecutionActorRef{}, errors.New("Deploy revoked during observation")
						}
						return capture(ctx, id)
					}
					runtime.Observe = func(ctx context.Context, id previewruntime.Identity) (previewruntime.Outcome, error) {
						out, err := observe(ctx, id)
						revoked.Store(true)
						return out, err
					}
					s.SetPreviewRuntime(runtime)
				case "unhealthy":
					unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
					t.Cleanup(unhealthy.Close)
					runtime := s.PreviewRuntime()
					observe := runtime.Observe
					runtime.Observe = func(ctx context.Context, id previewruntime.Identity) (previewruntime.Outcome, error) {
						out, err := observe(ctx, id)
						out.URL = unhealthy.URL
						return out, err
					}
					s.SetPreviewRuntime(runtime)
				case "running", "foreign-runtime":
					runtime := s.PreviewRuntime()
					observe := runtime.Observe
					runtime.Observe = func(ctx context.Context, id previewruntime.Identity) (previewruntime.Outcome, error) {
						out, err := observe(ctx, id)
						if drift == "running" {
							out.Phase = "starting"
						} else {
							out.ReleaseID = "rel-foreign"
						}
						return out, err
					}
					s.SetPreviewRuntime(runtime)
				}
				if boundary == "before-build" {
					if err := s.invokeCoordinatorPreview(ctx, p, row); err == nil || starts.Load() != 0 {
						t.Fatal("stale reservation reached build", err, starts.Load())
					}
				} else {
					got, err := s.continueCoordinatorPreview(ctx, p, row)
					if err == nil && got.Status != "unknown" && got.Status != "preview-blocked" || got.Delivery.Preview.Evidence != nil || starts.Load() != 1 {
						t.Fatal("stale or unowned preview joined", got, err)
					}
				}
				run, _ := s.projectRuns.Get(p.ID, p.Path, row.RunID)
				if len(run.Releases) != 0 || run.Acceptance != nil {
					t.Fatal("rejected preview gained release evidence")
				}
			})
		}
	}
}

func TestCoordinatorDeliveryChecksExplicitlyReviewedFailedWorkerSource(t *testing.T) {
	for _, terminal := range []string{"failed", "review-required"} {
		t.Run(terminal, func(t *testing.T) {
			s, p, h, _, out, starts := coordinatorDeliveryFixture(t)
			coordinatorIntegrateFixture(t, s, p, h, out, terminal)
			out = coordinatorDeliveryAdvance(t, h, p, out, 200)
			finishEvidenceRun(t, s, p.ID, out.Delivery.CheckID)
			out = coordinatorDeliveryAdvance(t, h, p, out, 200)
			if out.Status != "checks-verified" || starts.Load() != 1 {
				t.Fatal("authoritative reviewed integration stranded by worker exit", out)
			}
		})
	}
}
