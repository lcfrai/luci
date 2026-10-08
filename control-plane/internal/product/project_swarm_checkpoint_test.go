package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func checkpointHTTPFixture(t *testing.T) (*Server, *http.ServeMux, project.Project, ProjectRun, string, string) {
	t.Helper()
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "checkpoint-first-attempt")
	op := run.Operations[0]
	path, err := s.swarmPath(p.ID, run.ID, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(path, "alpha.txt"), []byte("retained partial result\nremaining work\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, "failed", "controlled settled worker exit")
	if err != nil {
		t.Fatal(err)
	}
	s.SetProjectSwarmCheckpointGuard(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(_ context.Context, req ProjectSwarmRecoveryRequest) (func(), error) {
		if req.ProjectID != p.ID || req.RunID != run.ID || req.OperationID != op.ID || req.ChildID != op.ChildID {
			t.Fatal("checkpoint guard received foreign attempt", req)
		}
		return func() {}, nil
	})
	// Exercise the production source-admission registration too: the checkpoint
	// POST owns exclusive admission; its GET remains reader protected.
	guarded := http.NewServeMux()
	s.registerProjectSwarm(s.guardProjectSourceRoutes(guarded))
	return s, guarded, p, run, base + "/operations/" + op.ID + "/checkpoint", path
}

func checkpointReviewHTTP(t *testing.T, mux *http.ServeMux, url string) ProjectSwarmCheckpointReview {
	t.Helper()
	var out struct {
		Review ProjectSwarmCheckpointReview `json:"review"`
	}
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", url, nil, 200).Body.Bytes(), &out)
	return out.Review
}
func checkpointBody(review ProjectSwarmCheckpointReview, key string) map[string]any {
	return map[string]any{"expectedRevision": review.ExpectedRevision, "idempotencyKey": key, "reviewDigest": review.ReviewDigest, "childId": review.ChildID}
}

func TestSwarmCheckpointContinuesExactRetainedBytesInNewAttemptWithoutAcceptingTask(t *testing.T) {
	s, mux, p, run, url, oldPath := checkpointHTTPFixture(t)
	beforeMain, beforeOld := auditTree(t, p.Path), auditTree(t, oldPath)
	review := checkpointReviewHTTP(t, mux, url)
	if !review.CanContinue || len(review.Files) != 1 || review.Files[0].Status != "added" || review.Files[0].After != "retained partial result\nremaining work\n" {
		t.Fatal("incomplete checkpoint review", review)
	}
	response := journeyRequest(t, mux, "POST", url, checkpointBody(review, "explicit-reviewed-checkpoint"), 201)
	nextRun := projectRunHTTPResponse(t, response).ProjectRun
	if len(nextRun.Operations) != 2 {
		t.Fatal("did not create one distinct attempt", nextRun)
	}
	op := nextRun.Operations[1]
	if op.ChildID == run.Operations[0].ChildID || op.Checkpoint == nil || op.Allocation != "ready" || op.Status != "reserved" || op.BaseCommit != review.TargetCommit || op.Checkpoint.SeedCommit == op.BaseCommit || op.Implementation.SourceCommit != op.Checkpoint.SeedCommit {
		t.Fatal("checkpoint lost independent initial source", op)
	}
	newPath, err := s.swarmPath(p.ID, run.ID, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(newPath, "alpha.txt"))
	if err != nil || string(data) != review.Files[0].After {
		t.Fatal("new worker did not receive reviewed bytes", err)
	}
	if !reflect.DeepEqual(beforeMain, auditTree(t, p.Path)) || !reflect.DeepEqual(beforeOld, auditTree(t, oldPath)) {
		t.Fatal("continuation mutated primary or original retained attempt")
	}
	if _, err = os.Stat(filepath.Join(newPath, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
		t.Fatal("checkpoint borrows writable old Git metadata", err)
	}
	if swarmTask(&nextRun, "task-a").Integration != nil || swarmTask(&nextRun, "task-c").OperationID != "" {
		t.Fatal("checkpoint released dependent task")
	}
	if _, err = s.CaptureProjectRunTaskContext(context.Background(), p.ID, run.ID, op.ID, op.WorktreeID, newPath, *ImplementationContextSelection(op.Implementation)); err != nil {
		t.Fatal("exact checkpoint cannot pass later capture", err)
	}
	base := strings.TrimSuffix(url, "/operations/"+run.Operations[0].ID+"/checkpoint")
	journeyRequest(t, mux, "POST", base+"/tasks/task-c/reserve", map[string]any{"expectedRevision": nextRun.Revision, "idempotencyKey": "checkpoint-does-not-accept"}, 409)
	// A service/store reopen and lost response must recover the same reserved
	// child without re-reading a now superseded source or making another copy.
	s.projectRuns, err = NewProjectRunStore(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	before := auditTree(t, s.projectRoot())
	duplicate := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", url, checkpointBody(review, "explicit-reviewed-checkpoint"), 200)).ProjectRun
	if len(duplicate.Operations) != 2 || duplicate.Operations[1].ID != op.ID || !reflect.DeepEqual(before, auditTree(t, s.projectRoot())) {
		t.Fatal("lost response replayed checkpoint allocation")
	}
	changed := checkpointBody(review, "explicit-reviewed-checkpoint")
	changed["reviewDigest"] = strings.Repeat("f", 64)
	journeyRequest(t, mux, "POST", url, changed, 409)
	// Adopted changes remain part of the task's later integration delta.
	nextRun, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, duplicate.Revision, op.ID, op.ChildID, "failed", "controlled stopped new attempt")
	if err != nil {
		t.Fatal(err)
	}
	integration, err := s.swarmReview(context.Background(), p, nextRun, "task-a")
	if err != nil || !integration.CanIntegrate || integration.SourceCommit != op.Checkpoint.SeedCommit || !strings.Contains(integration.Patch, "retained partial result") {
		t.Fatal("checkpoint changes disappeared from eventual review", integration, err)
	}
}

func TestSwarmCheckpointReviewRefusesUnsafeOrIncompleteOutput(t *testing.T) {
	for _, scenario := range []string{"binary", "large", "out-of-scope", "protected", "symlink", "hardlink", "assume-unchanged"} {
		t.Run(scenario, func(t *testing.T) {
			s, mux, p, _, url, path := checkpointHTTPFixture(t)
			name := filepath.Join(path, "alpha.txt")
			switch scenario {
			case "binary":
				if err := os.WriteFile(name, []byte{0, 1, 2}, 0600); err != nil {
					t.Fatal(err)
				}
			case "large":
				if err := os.WriteFile(name, []byte(strings.Repeat("a", swarmCheckpointFileLimit+1)), 0600); err != nil {
					t.Fatal(err)
				}
			case "out-of-scope":
				if err := os.WriteFile(filepath.Join(path, "foreign.txt"), []byte("foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			case "protected":
				if err := os.WriteFile(filepath.Join(path, "credentials.json"), []byte("DO-NOT-EXPOSE-CHECKPOINT-SECRET"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink", "hardlink":
				foreign := filepath.Join(t.TempDir(), "outside.txt")
				if err := os.WriteFile(foreign, []byte("DO-NOT-EXPOSE-CHECKPOINT-SECRET"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
				var err error
				if scenario == "symlink" {
					err = os.Symlink(foreign, name)
				} else {
					err = os.Link(foreign, name)
				}
				if err != nil {
					t.Skipf("host cannot make controlled link: %v", err)
				}
			case "assume-unchanged":
				confinedFixtureGit(t, path, "add", "--", "alpha.txt")
				confinedFixtureGit(t, path, "update-index", "--assume-unchanged", "--", "alpha.txt")
			}
			before := auditTree(t, s.stateDir)
			beforeMain := auditTree(t, p.Path)
			w := journeyRequest(t, mux, "GET", url, nil, 200)
			var body struct {
				Review ProjectSwarmCheckpointReview `json:"review"`
			}
			decodeEvidenceResponse(t, w.Body.Bytes(), &body)
			if body.Review.CanContinue || len(body.Review.Files) != 0 || body.Review.ReviewDigest != "" || strings.Contains(w.Body.String(), "DO-NOT-EXPOSE") {
				t.Fatal("unsafe or partial checkpoint advertised", w.Body.String())
			}
			if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeMain, auditTree(t, p.Path)) {
				t.Fatal("read review mutated state/source")
			}
		})
	}
}

func TestSwarmCheckpointFreshReviewAuthorityAndLifecycleBeforeAllocation(t *testing.T) {
	for _, scenario := range []string{"retained-edit", "dirty-primary", "revision", "revoked", "unsettled", "pending-lease"} {
		t.Run(scenario, func(t *testing.T) {
			s, mux, p, run, url, path := checkpointHTTPFixture(t)
			review := checkpointReviewHTTP(t, mux, url)
			if !review.CanContinue {
				t.Fatal(review)
			}
			switch scenario {
			case "retained-edit":
				if err := os.WriteFile(filepath.Join(path, "alpha.txt"), []byte("changed after consent\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "dirty-primary":
				if err := os.WriteFile(filepath.Join(p.Path, "new-source.txt"), []byte("external edit"), 0600); err != nil {
					t.Fatal(err)
				}
			case "revision":
				var err error
				run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(r *ProjectRun) error { r.Reason = "concurrent mutation"; return nil })
				if err != nil {
					t.Fatal(err)
				}
			case "revoked":
				s.projectSwarmCheckpointAuthorize = func(context.Context, ProjectSwarmRecoveryRequest) error {
					return errors.New("current permission revoked")
				}
			case "unsettled":
				s.projectSwarmCheckpointGuard = func(context.Context, ProjectSwarmRecoveryRequest) (func(), error) {
					return nil, errors.New("original launch remains unsettled")
				}
			case "pending-lease":
				s.SetProjectSwarmPendingLaunch(func(context.Context, string, string, string, string) (bool, error) { return true, nil })
			}
			beforeState, beforeProjects := auditTree(t, s.stateDir), auditTree(t, s.projectRoot())
			journeyRequest(t, mux, "POST", url, checkpointBody(review, "refuse-checkpoint-before-copy"), 409)
			if !reflect.DeepEqual(beforeState, auditTree(t, s.stateDir)) || !reflect.DeepEqual(beforeProjects, auditTree(t, s.projectRoot())) {
				t.Fatal("refused checkpoint allocated or wrote files")
			}
		})
	}
}

func TestSwarmCheckpointPrimaryDriftRequiresExactAffectedBaseIdentity(t *testing.T) {
	for _, overlap := range []bool{false, true} {
		t.Run(map[bool]string{false: "disjoint-reviewed-integration", true: "overlapping-reviewed-integration"}[overlap], func(t *testing.T) {
			s, mux, p, run, url, _ := checkpointHTTPFixture(t)
			name := "beta.txt"
			if overlap {
				name = "alpha.txt"
			}
			commit := swarmCommitFixture(t, p.Path, name, "another reviewed task\n")
			var err error
			run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(r *ProjectRun) error {
				swarmTask(r, "task-b").Integration = &ProjectSwarmIntegration{State: "integrated", Commit: commit, SourceCommit: commit, TargetCommit: run.Input.Implementation.SourceCommit, ReviewDigest: strings.Repeat("a", 64), IntegratedAt: time.Now().UTC().Format(time.RFC3339Nano)}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			review := checkpointReviewHTTP(t, mux, url)
			if overlap {
				if review.CanContinue || !strings.Contains(review.Reason, "also changed") {
					t.Fatal("overlapping target allowed", review)
				}
				return
			}
			if !review.CanContinue || review.TargetCommit != commit {
				t.Fatal("disjoint integration lost checkpoint", review)
			}
			next := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", url, checkpointBody(review, "checkpoint-after-disjoint-integration"), 201)).ProjectRun
			op := next.Operations[len(next.Operations)-1]
			path, err := s.swarmPath(p.ID, run.ID, op.ID)
			if err != nil {
				t.Fatal(err)
			}
			if op.BaseCommit != commit || op.Allocation != "ready" {
				t.Fatal("new integration base not preserved", op)
			}
			if b, e := os.ReadFile(filepath.Join(path, "beta.txt")); e != nil || string(b) != "another reviewed task\n" {
				t.Fatal("continuation discarded disjoint integrated work", e)
			}
		})
	}
}

func TestSwarmCheckpointCapabilityReadAudit(t *testing.T) {
	s, mux, p, _, url, path := checkpointHTTPFixture(t)
	s.projectSwarmCheckpointAuthorize = func(context.Context, ProjectSwarmRecoveryRequest) error {
		t.Fatal("GET reauthorized mutation")
		return nil
	}
	s.projectSwarmCheckpointGuard = func(context.Context, ProjectSwarmRecoveryRequest) (func(), error) {
		t.Fatal("GET acquired lifecycle mutation lock")
		return nil, nil
	}
	state, primary, retained := auditTree(t, s.stateDir), auditTree(t, p.Path), auditTree(t, path)
	for _, method := range []string{"GET", "HEAD"} {
		journeyRequest(t, mux, method, url, nil, 200)
	}
	if !reflect.DeepEqual(state, auditTree(t, s.stateDir)) || !reflect.DeepEqual(primary, auditTree(t, p.Path)) || !reflect.DeepEqual(retained, auditTree(t, path)) {
		t.Fatal("checkpoint read mutated private/source/retained bytes")
	}
}

func TestSwarmCheckpointIncludesCommittedAndDirtyOutputAndPreservesTrailingBytes(t *testing.T) {
	s, mux, p, run, url, path := checkpointHTTPFixture(t)
	commit := swarmCommitFixture(t, path, "alpha.txt", "committed portion\n")
	wanted := "committed portion\nunfinished portion  \n\n"
	if err := os.WriteFile(filepath.Join(path, "alpha.txt"), []byte(wanted), 0600); err != nil {
		t.Fatal(err)
	}
	review := checkpointReviewHTTP(t, mux, url)
	if !review.CanContinue || review.SourceCommit != commit || len(review.Files) != 1 || review.Files[0].After != wanted {
		t.Fatal("dirty output was replaced by prior commit or trimmed", review)
	}
	next := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", url, checkpointBody(review, "checkpoint-committed-plus-dirty"), 201)).ProjectRun
	op := next.Operations[1]
	newPath, err := s.swarmPath(p.ID, run.ID, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if content, e := os.ReadFile(filepath.Join(newPath, "alpha.txt")); e != nil || string(content) != wanted {
		t.Fatal("new checkpoint did not preserve exact trailing bytes", e)
	}
}

func TestSwarmCheckpointDeletionIsReviewedAndAppliedWithoutAcceptingTask(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t,
		ProjectTask{ID: "task-a", Title: "Change source", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"check.js"}},
		ProjectTask{ID: "task-b", Title: "Independent task", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"beta.txt"}},
		ProjectTask{ID: "task-c", Title: "Dependent task", Status: "todo", RequirementIDs: []string{"AC-save"}, WritePaths: []string{"gamma.txt"}, DependsOn: []string{"task-a"}})
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "checkpoint-delete-original")
	op := run.Operations[0]
	path, err := s.swarmPath(p.ID, run.ID, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(path, "check.js")); err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, "failed", "controlled stopped worker")
	if err != nil {
		t.Fatal(err)
	}
	s.SetProjectSwarmCheckpointGuard(func(context.Context, ProjectSwarmRecoveryRequest) error { return nil }, func(context.Context, ProjectSwarmRecoveryRequest) (func(), error) { return func() {}, nil })
	url := base + "/operations/" + op.ID + "/checkpoint"
	review := checkpointReviewHTTP(t, mux, url)
	if !review.CanContinue || len(review.Files) != 1 || review.Files[0].Status != "deleted" || review.Files[0].Before == "" || review.Files[0].AfterMode != "" {
		t.Fatal("deleted source was omitted", review)
	}
	next := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", url, checkpointBody(review, "checkpoint-delete-reviewed"), 201)).ProjectRun
	newOp := next.Operations[1]
	newPath, err := s.swarmPath(p.ID, run.ID, newOp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(newPath, "check.js")); !os.IsNotExist(err) {
		t.Fatal("reviewed deletion not applied", err)
	}
	if _, err = os.Stat(filepath.Join(p.Path, "check.js")); err != nil {
		t.Fatal("deletion affected primary before integration", err)
	}
	if newOp.Allocation != "ready" || swarmTask(&next, "task-a").Integration != nil {
		t.Fatal("deletion checkpoint was incomplete or accepted", newOp)
	}
}

func TestSwarmCheckpointInterruptedAllocationRetainsIntentAndNeverReplaysCopy(t *testing.T) {
	s, mux, p, run, url, path := checkpointHTTPFixture(t)
	review := checkpointReviewHTTP(t, mux, url)
	if !review.CanContinue {
		t.Fatal(review)
	}
	beforeOld := auditTree(t, path)
	runner := s.runner
	attempts := 0
	s.runner = func(ctx context.Context, command string, args []string, dir string) (string, error) {
		if command == "git" && dir != path && dir != p.Path {
			for _, arg := range args {
				if arg == "init" {
					attempts++
					return "", errors.New("controlled interruption after durable reservation")
				}
			}
		}
		return runner(ctx, command, args, dir)
	}
	request := checkpointBody(review, "checkpoint-interrupted-allocation")
	next := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", url, request, 201)).ProjectRun
	if len(next.Operations) != 2 || next.Operations[1].Checkpoint == nil || next.Operations[1].Allocation != "unknown" || next.Operations[1].Checkpoint.SeedCommit != "" || next.Operations[1].Implementation != nil || attempts != 1 {
		t.Fatal("interrupted copy lost intent or became dispatchable", next)
	}
	var err error
	s.projectRuns, err = NewProjectRunStore(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	duplicate := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", url, request, 200)).ProjectRun
	if len(duplicate.Operations) != 2 || duplicate.Operations[1].ID != next.Operations[1].ID || attempts != 1 || !reflect.DeepEqual(beforeOld, auditTree(t, path)) {
		t.Fatal("interrupted checkpoint replayed allocation or touched original")
	}
	if original, err := s.GetProjectSwarmRun(p.ID, run.ID); err != nil || original.Operations[0].ID != run.Operations[0].ID {
		t.Fatal("original private receipt lost", err)
	}
}

func TestSwarmCheckpointUnstartedRetirementFencesDelayedAllocationPublishers(t *testing.T) {
	for _, afterSeed := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-seed", true: "after-seed-before-ready"}[afterSeed], func(t *testing.T) {
			s, mux, p, run, url, oldPath := checkpointHTTPFixture(t)
			review := checkpointReviewHTTP(t, mux, url)
			if !review.CanContinue {
				t.Fatal(review)
			}
			runner := s.runner
			s.runner = func(ctx context.Context, command string, args []string, dir string) (string, error) {
				if command == "git" && dir != oldPath && dir != p.Path {
					if afterSeed {
						current, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
						if err == nil && len(current.Operations) == 2 && current.Operations[1].Checkpoint.SeedCommit != "" {
							return "", errors.New("interrupted after durable seed before implementation binding")
						}
					} else {
						for _, arg := range args {
							if arg == "init" {
								return "", errors.New("interrupted before seed")
							}
						}
					}
				}
				return runner(ctx, command, args, dir)
			}
			current := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", url, checkpointBody(review, "explicit-interrupted-checkpoint"), 201)).ProjectRun
			s.runner = runner
			op := current.Operations[1]
			if op.Allocation != "unknown" || op.Implementation != nil || !unstartedSwarmCheckpoint(current, op) || (op.Checkpoint.SeedCommit != "") != afterSeed {
				t.Fatal("wrong pre-dispatch boundary", op)
			}
			s.SetProjectSwarmPendingLaunch(func(context.Context, string, string, string, string) (bool, error) {
				t.Fatal("never-ready checkpoint inferred retirement from a missing ledger")
				return true, nil
			})
			s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
			base := strings.TrimSuffix(url, "/operations/"+run.Operations[0].ID+"/checkpoint")
			resume := make(chan struct{})
			done := make(chan error, 2)
			go func() {
				<-resume
				_, err := s.projectRuns.recordSwarmCheckpointSeed(p.ID, p.Path, run.ID, current.Revision, op.ID, strings.Repeat("2", 40))
				done <- err
				_, err = s.projectRuns.CompleteSwarmAllocation(p.ID, p.Path, run.ID, current.Revision, op.ID, nil, registry.Session{}, nil)
				done <- err
			}()
			retired := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/abandon", map[string]any{"expectedRevision": current.Revision, "operationId": op.ID, "childId": op.ChildID}, 200)).ProjectRun
			close(resume)
			if <-done == nil || <-done == nil {
				t.Fatal("delayed allocation publisher revived cancelled checkpoint")
			}
			privateRetired, readErr := s.GetProjectSwarmRun(p.ID, run.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !privateRetired.Operations[1].UnclaimedRetirement || retired.Operations[1].Allocation != "abandoned" || retired.Operations[1].Status != "cancelled" {
				t.Fatal("explicit cancellation lost private no-launch provenance", retired.Operations[1])
			}
			// Even a stale publisher that reloads the revision cannot overwrite the
			// terminal allocation state: both publication transitions check it.
			if _, err := s.projectRuns.recordSwarmCheckpointSeed(p.ID, p.Path, run.ID, retired.Revision, op.ID, strings.Repeat("3", 40)); err == nil {
				t.Fatal("seed publication overwrote abandoned state")
			}
			if _, err := s.projectRuns.CompleteSwarmAllocation(p.ID, p.Path, run.ID, retired.Revision, op.ID, nil, registry.Session{}, nil); err == nil {
				t.Fatal("ready publication overwrote abandoned state")
			}
			var reopenErr error
			s.projectRuns, reopenErr = NewProjectRunStore(s.stateDir)
			if reopenErr != nil {
				t.Fatal(reopenErr)
			}
			next := swarmReserveHTTP(t, mux, base, retired, "task-a", "explicit-fresh-attempt-after-copy-failure")
			if len(next.Operations) != 3 || next.Operations[2].ID == op.ID || next.Operations[2].Allocation != "ready" {
				t.Fatal("unstarted retirement prevented distinct explicit retry", next)
			}
			if data, err := os.ReadFile(filepath.Join(oldPath, "alpha.txt")); err != nil || string(data) != "retained partial result\nremaining work\n" {
				t.Fatal("copy recovery lost original retained output", err)
			}
		})
	}
}

func TestSwarmCheckpointUnstartedProofRejectsClaimedOrForeignState(t *testing.T) {
	_, _, _, run, _, _ := checkpointHTTPFixture(t)
	source := run.Operations[0]
	op := ProjectRunOperation{ID: "op-checkpoint-next", ChildID: "s-checkpoint-next", TaskID: source.TaskID, Attempt: source.Attempt + 1, Status: "unknown", Allocation: "unknown", BaseCommit: source.BaseCommit, Checkpoint: &ProjectSwarmCheckpoint{SourceOperationID: source.ID, SourceChildID: source.ChildID, SourceCommit: source.BaseCommit, SourceBaseCommit: source.BaseCommit, TargetCommit: source.BaseCommit, ReviewDigest: strings.Repeat("a", 64), ContextDigest: implementationDigest(source.Implementation), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}}
	if !unstartedSwarmCheckpoint(run, op) {
		t.Fatal("valid private pre-ready provenance refused")
	}
	for name, change := range map[string]func(*ProjectRunOperation){
		"legacy no checkpoint": func(o *ProjectRunOperation) { o.Checkpoint = nil },
		"claimed status":       func(o *ProjectRunOperation) { o.Status = "dispatching" },
		"ready allocation":     func(o *ProjectRunOperation) { o.Allocation = "ready" },
		"saved implementation": func(o *ProjectRunOperation) { o.Implementation = source.Implementation },
		"recovery intent":      func(o *ProjectRunOperation) { o.LaunchRecovery = &ProjectSwarmRecoveryIntent{} },
		"recovery proof":       func(o *ProjectRunOperation) { o.LaunchRecoveryProof = &ProjectSwarmRecoveryProof{} },
		"retirement proof":     func(o *ProjectRunOperation) { o.AbsenceRetirement = &ProjectSwarmRecoveryProof{} },
		"integration":          func(o *ProjectRunOperation) { o.Integration = &ProjectSwarmIntegration{} },
		"foreign source child": func(o *ProjectRunOperation) { o.Checkpoint.SourceChildID = "s-foreign" },
		"foreign context":      func(o *ProjectRunOperation) { o.Checkpoint.ContextDigest = strings.Repeat("b", 64) },
		"self source":          func(o *ProjectRunOperation) { o.Checkpoint.SourceOperationID = o.ID },
		"false seed":           func(o *ProjectRunOperation) { o.Checkpoint.SeedCommit = o.BaseCommit },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := op
			checkpoint := *op.Checkpoint
			candidate.Checkpoint = &checkpoint
			change(&candidate)
			if unstartedSwarmCheckpoint(run, candidate) {
				t.Fatal("claimed/foreign state minted never-started proof")
			}
		})
	}
}

func TestSwarmCheckpointGuardedRoutesRefuseBusySourceBeforeNewWorkspace(t *testing.T) {
	s, mux, p, run, url, _ := checkpointHTTPFixture(t)
	review := checkpointReviewHTTP(t, mux, url)
	before := auditTree(t, s.projectRoot())
	guard := s.branchGuard(p.ID)
	guard.Lock()
	func() {
		defer guard.Unlock()
		journeyRequest(t, mux, "GET", url, nil, 409)
		journeyRequest(t, mux, "POST", url, checkpointBody(review, "checkpoint-source-exclusively-busy"), 409)
	}()
	guard.RLock()
	func() {
		defer guard.RUnlock()
		// Source reads can coexist; allocating a checkpoint cannot.
		checkpointReviewHTTP(t, mux, url)
		journeyRequest(t, mux, "POST", url, checkpointBody(review, "checkpoint-source-reader-busy"), 409)
	}()
	current, err := s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || current.Revision != run.Revision || len(current.Operations) != 1 || !reflect.DeepEqual(before, auditTree(t, s.projectRoot())) {
		t.Fatal("busy source mutated run or allocated retained output", err)
	}
	journeyRequest(t, mux, "POST", url, checkpointBody(review, "checkpoint-source-now-free"), 201)
}
