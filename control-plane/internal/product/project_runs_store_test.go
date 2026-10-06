package product

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func projectRunStoreFixture(t *testing.T) (*ProjectRunStore, string, string, ProjectRunInput) {
	t.Helper()
	stateDir, projectDir := t.TempDir(), t.TempDir()
	store, err := NewProjectRunStore(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	id := project.ProjectID(projectDir)
	digest := strings.Repeat("a", 64)
	approvals := ProjectRunApprovalSelection{SpecificationApprovalID: "approval-" + strings.Repeat("a", 24), SpecificationRevision: 2, SpecificationDigest: digest, DesignApprovalID: "approval-" + strings.Repeat("b", 24), DesignRevision: 3, DesignDigest: digest, BriefDigest: digest, SketchDigest: digest}
	input := ProjectRunInput{
		Implementation: registry.ImplementationContext{ProjectID: id, WorktreeID: project.WorktreeID(projectDir), BriefRevision: 1, BriefDigest: digest, PlanRevision: 4, PlanDigest: digest, SpecificationRevision: 2, SpecificationDigest: digest, DesignPlanRevision: 3, DesignPlanDigest: digest, TaskIDs: []string{"task-1"}, TaskTitles: []string{"Build a useful app"}, RequirementIDs: []string{"requirement-1"}, SourceCommit: strings.Repeat("1", 40), SourceDigest: digest},
		Approvals:      approvals, Route: ProjectRunRoute{Harness: "codex", Model: "test-model", PermissionMode: "ask", Connection: AgentConnectionReceipt{ID: "connection-1", Revision: 1, Kind: "codex-subscription", AuthMode: "subscription"}}, Outcome: "preview", Requirements: []RequirementSnapshot{{ID: "requirement-1", Title: "A useful result", Acceptance: "The app performs its expected behavior"}},
	}
	return store, id, projectDir, input
}

func createStoredRun(t *testing.T, s *ProjectRunStore, id, path string, input ProjectRunInput) ProjectRun {
	t.Helper()
	run, created, err := s.Create(id, path, "create-key-123456789", input, "private-codex-generation")
	if err != nil || !created {
		t.Fatalf("create: created=%v err=%v", created, err)
	}
	return run
}

func TestProjectRunStoreCreateIdempotencyImmutablePrivateAndReadOnly(t *testing.T) {
	s, id, path, input := projectRunStoreFixture(t)
	if rows, err := s.List(id, path); err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
	if _, err := os.Stat(filepath.Join(s.stateDir, "project-runs")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("listing created storage: %v", err)
	}
	run := createStoredRun(t, s, id, path, input)
	input.Implementation.TaskTitles[0] = "caller mutation"
	run.Input.Requirements[0].Acceptance = "return mutation"
	loaded, err := s.Get(id, path, run.ID)
	if err != nil || loaded.Input.Implementation.TaskTitles[0] != "Build a useful app" || loaded.Input.Requirements[0].Acceptance == "return mutation" {
		t.Fatal(loaded, err)
	}
	duplicate, created, err := s.Create(id, path, "create-key-123456789", loaded.Input, "private-codex-generation")
	if err != nil || created || duplicate.ID != run.ID {
		t.Fatalf("duplicate: %s %v %v", duplicate.ID, created, err)
	}
	changed := loaded.Input
	changed.Outcome = "publish"
	if _, _, err = s.Create(id, path, "create-key-123456789", changed, "private-codex-generation"); !errors.Is(err, ErrProjectRunKeyConflict) {
		t.Fatal(err)
	}
	file := filepath.Join(s.stateDir, "project-runs", id, run.ID+".json")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewProjectRunStore(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.List(id, path); err != nil {
		t.Fatal(err)
	}
	loaded, err = reopened.Get(id, path, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(file)
	if err != nil || string(before) != string(after) {
		t.Fatal("reads/reopen changed persisted bytes", err)
	}
	public, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-codex-generation", s.stateDir, path, "accountScope", "projectScope", "createKeyDigest", "operationKeys"} {
		if strings.Contains(string(public), secret) {
			t.Fatalf("private storage leaked %q", secret)
		}
	}
	if generation, err := reopened.PrivateCodexConnectionID(id, path, run.ID); err != nil || generation != "private-codex-generation" {
		t.Fatal(generation, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(file)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal(info, err)
		}
		info, err = os.Stat(filepath.Dir(file))
		if err != nil || info.Mode().Perm() != 0700 {
			t.Fatal(info, err)
		}
	}
}

func TestProjectRunStoreNormalizedSelectionSurvivesSourceChanges(t *testing.T) {
	s, id, path, input := projectRunStoreFixture(t)
	selectionDigest := reviewDigest("server-normalized selected task IDs, revisions, route and outcome")
	run, created, err := s.Create(id, path, "create-key-123456789", input, "private-generation", selectionDigest)
	if err != nil || !created {
		t.Fatal(created, err)
	}
	input.Implementation.SourceCommit = strings.Repeat("2", 40)
	input.Implementation.SourceDigest = strings.Repeat("b", 64)
	duplicate, created, err := s.Create(id, path, "create-key-123456789", input, "private-generation", selectionDigest)
	if err != nil || created || duplicate.ID != run.ID || duplicate.Input.Implementation.SourceCommit != run.Input.Implementation.SourceCommit {
		t.Fatal(duplicate, created, err)
	}
	found, exists, err := s.FindByKey(id, path, "create-key-123456789", selectionDigest)
	if err != nil || !exists || found.ID != run.ID {
		t.Fatal(found, exists, err)
	}
	if _, _, err = s.FindByKey(id, path, "create-key-123456789", reviewDigest("another request")); !errors.Is(err, ErrProjectRunKeyConflict) {
		t.Fatal(err)
	}
}

func TestProjectRunStoreFailedReservationDoesNotGrantDispatch(t *testing.T) {
	s, id, path, input := projectRunStoreFixture(t)
	run := createStoredRun(t, s, id, path, input)
	dir := filepath.Join(s.stateDir, "project-runs", id)
	file := filepath.Join(dir, run.ID+".json")
	// A nonregular file cannot become an authoritative run. The retained
	// original is restored after the simulated storage obstruction.
	retained := filepath.Join(s.stateDir, "retained-run.json")
	if err := os.Rename(file, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0700); err != nil {
		t.Fatal(err)
	}
	if _, _, created, err := s.ReserveOperation(id, path, run.ID, run.Revision, "operation-key-123456789", "session", reviewDigest("coding")); err == nil || created {
		t.Fatal("failed storage granted reservation", created, err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(retained, file); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Get(id, path, run.ID)
	if err != nil || loaded.Revision != run.Revision || len(loaded.Operations) != 0 {
		t.Fatal(loaded, err)
	}
	reserved, _, created, err := s.ReserveOperation(id, path, run.ID, loaded.Revision, "operation-key-123456789", "session", reviewDigest("coding"))
	if err != nil || !created || len(reserved.Operations) != 1 {
		t.Fatal(reserved, created, err)
	}
}

func TestProjectRunStoreReservationClaimCrashAndExplicitRetry(t *testing.T) {
	s, id, path, input := projectRunStoreFixture(t)
	run := createStoredRun(t, s, id, path, input)
	const key = "operation-key-123456789"
	digest := reviewDigest("coding request")
	var created atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, fresh, err := s.ReserveOperation(id, path, run.ID, run.Revision, key, "session", digest)
			if err != nil {
				t.Error(err)
			}
			if fresh {
				created.Add(1)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 {
		t.Fatalf("%d reservations", created.Load())
	}
	reserved, err := s.Get(id, path, run.ID)
	if err != nil || len(reserved.Operations) != 1 {
		t.Fatal(reserved, err)
	}
	op := reserved.Operations[0]
	if op.Status != "reserved" || !strings.HasPrefix(op.ChildID, "s-") {
		t.Fatal(op)
	}
	if _, _, _, err = s.ReserveOperation(id, path, run.ID, reserved.Revision, key, "session", reviewDigest("changed")); !errors.Is(err, ErrProjectRunKeyConflict) {
		t.Fatal(err)
	}
	if _, _, _, err = s.ReserveOperation(id, path, run.ID, reserved.Revision, "new-operation-key-123456", "session", digest); !errors.Is(err, ErrProjectRunPending) {
		t.Fatal(err)
	}
	dispatching, claimedOp, claimed, err := s.ClaimOperation(id, path, run.ID, reserved.Revision, op.ID)
	if err != nil || !claimed || claimedOp.ChildID != op.ChildID {
		t.Fatal(dispatching, claimedOp, claimed, err)
	}
	// Simulate interruption immediately after claim, before registry creation.
	reopened, err := NewProjectRunStore(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	current, err := reopened.Get(id, path, run.ID)
	if err != nil || current.Operations[0].Status != "dispatching" {
		t.Fatal(current, err)
	}
	_, same, claimed, err := reopened.ClaimOperation(id, path, run.ID, current.Revision, op.ID)
	if err != nil || claimed || same.ChildID != op.ChildID {
		t.Fatal(same, claimed, err)
	}
	unknown, err := reopened.ObserveOperation(id, path, run.ID, current.Revision, op.ID, op.ChildID, "unknown", "No matching child was found; inspect before retrying.")
	if err != nil || unknown.State != "blocked" {
		t.Fatal(unknown, err)
	}
	if _, _, _, err = reopened.ReserveOperation(id, path, run.ID, unknown.Revision, "retry-operation-key-123456", "session", digest); !errors.Is(err, ErrProjectRunPending) {
		t.Fatal(err)
	}
	failed, err := reopened.ObserveOperation(id, path, run.ID, unknown.Revision, op.ID, op.ChildID, "failed", "The existing child is confirmed failed.")
	if err != nil {
		t.Fatal(err)
	}
	retry, newOp, createdRetry, err := reopened.ReserveOperation(id, path, run.ID, failed.Revision, "retry-operation-key-123456", "session", digest)
	if err != nil || !createdRetry || newOp.Attempt != 2 || newOp.ChildID == op.ChildID || len(retry.Operations) != 2 {
		t.Fatal(retry, newOp, createdRetry, err)
	}
}

func TestProjectRunStoreStaleWritersAndCancellationCannotDispatch(t *testing.T) {
	s, id, path, input := projectRunStoreFixture(t)
	run := createStoredRun(t, s, id, path, input)
	reserved, op, _, err := s.ReserveOperation(id, path, run.ID, run.Revision, "operation-key-123456789", "session", reviewDigest("coding"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Cancel(id, path, run.ID, run.Revision); !errors.Is(err, ErrProjectRunConflict) {
		t.Fatal(err)
	}
	cancelled, err := s.Cancel(id, path, run.ID, reserved.Revision)
	if err != nil || cancelled.State != "cancelled" || cancelled.Operations[0].Status != "cancelled" {
		t.Fatal(cancelled, err)
	}
	if _, _, claimed, err := s.ClaimOperation(id, path, run.ID, cancelled.Revision, op.ID); !errors.Is(err, ErrProjectRunCancelled) || claimed {
		t.Fatal(claimed, err)
	}
	if _, _, _, err = s.ReserveOperation(id, path, run.ID, cancelled.Revision, "another-key-123456789", "session", reviewDigest("coding")); !errors.Is(err, ErrProjectRunCancelled) {
		t.Fatal(err)
	}
	observed, err := s.Observe(id, path, run.ID, cancelled.Revision, ProjectRunObservation{State: "ready", Reason: "No child started", Valid: true})
	if err != nil || observed.State != "cancelled" {
		t.Fatal(observed, err)
	}
}

func TestProjectRunStoreCopiedRecordsAndUnsafePathsRefused(t *testing.T) {
	s, id, path, input := projectRunStoreFixture(t)
	run := createStoredRun(t, s, id, path, input)
	file := filepath.Join(s.stateDir, "project-runs", id, run.ID+".json")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewProjectRunStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	otherDir, err := other.directory(id, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(otherDir, run.ID+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = other.Get(id, path, run.ID); !errors.Is(err, ErrProjectRunInvalid) {
		t.Fatal("copied account accepted", err)
	}
	newPath := t.TempDir()
	newID := project.ProjectID(newPath)
	newDir, err := s.directory(newID, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(newDir, run.ID+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(newID, newPath, run.ID); !errors.Is(err, ErrProjectRunInvalid) {
		t.Fatal("copied project accepted", err)
	}
	if _, err = s.Get(id, newPath, run.ID); !errors.Is(err, ErrProjectRunInvalid) {
		t.Fatal("foreign path accepted", err)
	}
	if err = os.Remove(file); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "untrusted.json")
	if err = os.WriteFile(outside, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, file); err == nil {
		if _, err = s.Get(id, path, run.ID); err == nil {
			t.Fatal("linked private record accepted")
		}
	} else {
		t.Log("symlink unavailable", err)
	}
}

func TestProjectRunStoreEvidenceCompletionAndHistoricalDrift(t *testing.T) {
	s, id, path, input := projectRunStoreFixture(t)
	run := createStoredRun(t, s, id, path, input)
	if _, err := s.Accept(id, path, run.ID, run.Revision); err == nil {
		t.Fatal("missing evidence completed run")
	}
	check := ProjectRunCheckEvidence{CheckID: "check-1", ProjectID: id, Commit: strings.Repeat("2", 40), SourceDigest: strings.Repeat("b", 64), Approvals: input.Approvals, Requirements: input.Requirements, Execution: "docker", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), OriginalAvailable: true, Commands: []ProjectRunCommandEvidence{{CheckCommand: CheckCommand{Label: "Verify", Executable: "node", Args: []string{"test.js"}}, Status: "passed", ExitCode: 0}}}
	bad := check
	bad.Requirements = nil
	if _, err := s.JoinCheck(id, path, run.ID, run.Revision, bad); err == nil {
		t.Fatal("incomplete coverage accepted")
	}
	checked, err := s.JoinCheck(id, path, run.ID, run.Revision, check)
	if err != nil || checked.State != "awaiting-release" {
		t.Fatal(checked, err)
	}
	check.Commands[0].Args[0] = "caller-modification"
	stored, err := s.Get(id, path, run.ID)
	if err != nil || stored.Checks[0].Commands[0].Args[0] != "test.js" {
		t.Fatal(stored, err)
	}
	release := ProjectRunReleaseEvidence{ReleaseID: "release-1", ProjectID: id, Commit: strings.Repeat("3", 40), SourceDigest: check.SourceDigest, Target: "native-docker", Outcome: "preview", Approvals: input.Approvals, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), URL: "http://127.0.0.1:1234"}
	if _, err = s.JoinRelease(id, path, run.ID, checked.Revision, release); err == nil {
		t.Fatal("different commit release accepted")
	}
	release.Commit = check.Commit
	joined, err := s.JoinRelease(id, path, run.ID, checked.Revision, release)
	if err != nil {
		t.Fatal(err)
	}
	complete, err := s.Accept(id, path, run.ID, joined.Revision)
	if err != nil || complete.State != "complete" || complete.Acceptance == nil {
		t.Fatal(complete, err)
	}
	if complete.Input.Implementation.SourceCommit == check.Commit {
		t.Fatal("checked source replaced immutable starting source")
	}
	drifted, err := s.Observe(id, path, run.ID, complete.Revision, ProjectRunObservation{State: "blocked", Valid: false, Reason: "The release lost current health."})
	if err != nil || drifted.State != "complete" || drifted.Observation.Valid || drifted.Acceptance == nil {
		t.Fatal(drifted, err)
	}
	reopened, err := NewProjectRunStore(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := reopened.Get(id, path, run.ID)
	if err != nil || len(retained.Checks) != 1 || retained.Checks[0].Commands[0].Args[0] != "test.js" {
		t.Fatal(retained, err)
	}
	// No dependency on the separately pruned ordinary check history exists.
	if retained.Acceptance.CheckID != "check-1" {
		t.Fatal(retained.Acceptance)
	}
}

func TestProjectRunStoreNativePreviewCannotCompletePublication(t *testing.T) {
	s, id, path, input := projectRunStoreFixture(t)
	input.Outcome = "publish"
	run := createStoredRun(t, s, id, path, input)
	check := ProjectRunCheckEvidence{CheckID: "check-1", ProjectID: id, Commit: strings.Repeat("2", 40), SourceDigest: strings.Repeat("b", 64), Approvals: input.Approvals, Requirements: input.Requirements, Execution: "docker", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Commands: []ProjectRunCommandEvidence{{CheckCommand: CheckCommand{Label: "Verify", Executable: "node", Args: []string{"test.js"}}, Status: "passed", ExitCode: 0}}}
	checked, err := s.JoinCheck(id, path, run.ID, run.Revision, check)
	if err != nil {
		t.Fatal(err)
	}
	release := ProjectRunReleaseEvidence{ReleaseID: "native-preview", ProjectID: id, Commit: check.Commit, SourceDigest: check.SourceDigest, Target: "native-docker", Outcome: "publish", Approvals: input.Approvals, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), URL: "http://127.0.0.1:1234"}
	if _, err = s.JoinRelease(id, path, run.ID, checked.Revision, release); err == nil {
		t.Fatal("native Docker preview counted as publication")
	}
	retained, err := s.Get(id, path, run.ID)
	if err != nil || retained.State != "awaiting-release" || len(retained.Releases) != 0 {
		t.Fatal(retained, err)
	}
}
