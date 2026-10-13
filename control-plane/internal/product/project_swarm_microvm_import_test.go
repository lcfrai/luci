package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

func microVMImportFixture(t *testing.T, runID string, op ProjectRunOperation, content string) (e2bbe.Record, e2bbe.SourceChanges, string) {
	t.Helper()
	stage := t.TempDir()
	if err := os.WriteFile(filepath.Join(stage, "alpha.txt"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(stage, "alpha.txt"), 0644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	req := e2bbe.Request{Identity: e2bbe.Identity{OwnerID: "owner-a", ProjectID: op.Implementation.ProjectID, RunID: runID, TaskID: op.TaskID, AttemptID: op.ID, OperationID: op.ChildID}, SourceDigest: implementationDigest(swarmStartingCommit(op)), ContextDigest: implementationDigest(op.Implementation), BundleDigest: strings.Repeat("b", 64), MaxSeconds: 60, EstimatedCostMicros: 100}
	body, _ := json.Marshal(req)
	requestHash := sha256.Sum256(body)
	identity := e2bbe.CaptureIdentity{Version: 1, Identity: req.Identity, RequestDigest: hex.EncodeToString(requestHash[:]), SandboxID: "vm-one", LaunchDigest: strings.Repeat("a", 64), ProcessPID: 321, InputBundleDigest: req.BundleDigest, SourceDigest: req.SourceDigest, ContextDigest: req.ContextDigest, BootID: "11111111-2222-4333-8444-555555555555", CgroupRelativePath: "luci-tasks/" + op.ChildID, CgroupInode: 123}
	reason := "exit status 0"
	frame := sha256.Sum256(append([]byte{3}, reason...))
	record := e2bbe.Record{Request: req, RequestDigest: identity.RequestDigest, SandboxID: identity.SandboxID, LaunchRequested: true, LaunchDigest: identity.LaunchDigest, ProcessPID: identity.ProcessPID, LaunchKernel: identity, NativeExit: e2bbe.NativeExitReceipt{Version: 1, SandboxID: identity.SandboxID, RequestDigest: identity.RequestDigest, LaunchDigest: identity.LaunchDigest, ProcessPID: identity.ProcessPID, FrameSHA256: hex.EncodeToString(frame[:]), Reason: reason, ObservedAt: time.Now().UTC()}, CaptureRequested: true}
	record.Capture = e2bbe.CaptureReceipt{Version: 1, CaptureID: "capture-one", Identity: identity, ArchiveSHA256: strings.Repeat("c", 64), ArchiveBytes: 4096, CapturedAt: time.Now().UTC(), SourceManifestSHA256: strings.Repeat("d", 64), ChangeManifestSHA256: strings.Repeat("e", 64)}
	after := e2bbe.TransferFile{Path: "alpha.txt", Size: int64(len(content)), SHA256: hex.EncodeToString(sum[:]), Mode: 0644}
	changes := e2bbe.SourceChanges{Version: 1, Policy: e2bbe.SourceSnapshotPolicy, InputBundleSHA256: req.BundleDigest, SourceManifestSHA256: record.Capture.SourceManifestSHA256, Changes: []e2bbe.SourceChange{{Path: "alpha.txt", Kind: "added", After: &after}}}
	encodedChanges, _ := json.Marshal(changes)
	changeHash := sha256.Sum256(encodedChanges)
	record.Capture.ChangeManifestSHA256 = hex.EncodeToString(changeHash[:])
	return record, changes, stage
}

func TestMicroVMCandidateUsesTaskOwnedGitReviewWithoutPrimaryWriteOrReplay(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "microvm-owned-attempt")
	op := run.Operations[0]
	var err error
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, "running", "cloud native process started")
	if err != nil {
		t.Fatal(err)
	}
	op = run.Operations[0]
	record, changes, stage := microVMImportFixture(t, run.ID, op, "cloud candidate\n")
	if _, err := s.RecordProjectSwarmMicroVMExit(context.Background(), p.ID, run.ID, op.ID, record); err == nil {
		t.Fatal("unregistered guard published a cloud native exit")
	}
	s.SetProjectSwarmMicroVMImportGuard(func(_ context.Context, scope ProjectSwarmMicroVMImportScope) (func(), error) {
		if scope.ProjectID != p.ID || scope.OperationID != op.ID || scope.ChildID != op.ChildID || scope.RequestDigest != record.RequestDigest {
			t.Fatal("import guard received different task identity", scope)
		}
		return func() {}, nil
	})
	tampered := record
	tampered.NativeExit.FrameSHA256 = strings.Repeat("0", 64)
	if _, err := s.RecordProjectSwarmMicroVMExit(context.Background(), p.ID, run.ID, op.ID, tampered); err == nil {
		t.Fatal("unverified native exit advanced task review")
	}
	run, err = s.RecordProjectSwarmMicroVMExit(context.Background(), p.ID, run.ID, op.ID, record)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := s.RecordProjectSwarmMicroVMExit(context.Background(), p.ID, run.ID, op.ID, record)
	if err != nil || repeated.Revision != run.Revision {
		t.Fatal("same captured exit replayed a run mutation", err)
	}
	op = run.Operations[0]
	before := auditTree(t, p.Path)
	commit, err := s.ImportProjectSwarmMicroVMCandidate(context.Background(), p.ID, run.ID, op.ID, record, changes, stage)
	if err != nil || !deliverySHA.MatchString(commit) || commit == swarmStartingCommit(op) {
		t.Fatal("candidate did not enter owned Git review", commit, err)
	}
	if got := auditTree(t, p.Path); !reflect.DeepEqual(got, before) {
		t.Fatal("cloud candidate wrote primary source")
	}
	workerPath := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	if data, err := os.ReadFile(filepath.Join(workerPath, "alpha.txt")); err != nil || string(data) != "cloud candidate\n" {
		t.Fatal("task checkout lacks returned source", err)
	}
	review, err := s.swarmReview(context.Background(), p, run, "task-a")
	if err != nil || !review.CanIntegrate || review.SourceCommit != commit || len(review.Paths) != 1 || review.Paths[0] != "alpha.txt" {
		t.Fatal("candidate skipped or failed normal integration review", review, err)
	}
	if _, err = s.ImportProjectSwarmMicroVMCandidate(context.Background(), p.ID, run.ID, op.ID, record, changes, stage); err == nil {
		t.Fatal("the same cloud candidate replayed over the committed task source")
	}
}

func TestMicroVMCandidateRejectsOutOfScopeSourceBeforeAnyGitWrite(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	s.SetProjectSwarmMicroVMImportGuard(func(context.Context, ProjectSwarmMicroVMImportScope) (func(), error) { return func() {}, nil })
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "microvm-scope-refusal")
	op := run.Operations[0]
	var err error
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, "running", "cloud native process started")
	if err != nil {
		t.Fatal(err)
	}
	op = run.Operations[0]
	record, changes, stage := microVMImportFixture(t, run.ID, op, "wrong path\n")
	run, err = s.RecordProjectSwarmMicroVMExit(context.Background(), p.ID, run.ID, op.ID, record)
	if err != nil {
		t.Fatal(err)
	}
	op = run.Operations[0]
	changes.Changes[0].Path, changes.Changes[0].After.Path = "beta.txt", "beta.txt"
	encoded, _ := json.Marshal(changes)
	hash := sha256.Sum256(encoded)
	record.Capture.ChangeManifestSHA256 = hex.EncodeToString(hash[:])
	workerPath := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
	before := auditTree(t, workerPath)
	if _, err = s.ImportProjectSwarmMicroVMCandidate(context.Background(), p.ID, run.ID, op.ID, record, changes, stage); err == nil {
		t.Fatal("cloud output outside reviewed write scope was admitted")
	}
	if !reflect.DeepEqual(before, auditTree(t, workerPath)) {
		t.Fatal("refused cloud candidate changed task source")
	}
}

func TestMicroVMCapturedExitRequiresExactReviewedRun(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	s.SetProjectSwarmMicroVMImportGuard(func(context.Context, ProjectSwarmMicroVMImportScope) (func(), error) { return func() {}, nil })
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "microvm-run-identity")
	op := run.Operations[0]
	var err error
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, "running", "cloud native process started")
	if err != nil {
		t.Fatal(err)
	}
	wrong, _, _ := microVMImportFixture(t, "another-run", run.Operations[0], "plausible output\n")
	if _, err = s.RecordProjectSwarmMicroVMExit(context.Background(), p.ID, run.ID, op.ID, wrong); err == nil {
		t.Fatal("a signed exit for another run advanced this task")
	}
	current, err := s.GetProjectSwarmRun(p.ID, run.ID)
	if err != nil || current.Revision != run.Revision || current.Operations[0].Status != "running" {
		t.Fatal("foreign run output changed local task state", err)
	}
}

func TestMicroVMCapturedReportJoinsTaskReviewWithoutTrustingWorkerChecks(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "microvm-report-123456")
	op := run.Operations[0]
	var err error
	run, err = s.projectRuns.ObserveOperation(p.ID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, "running", "cloud native process started")
	if err != nil {
		t.Fatal(err)
	}
	op = run.Operations[0]
	record, changes, stage := microVMImportFixture(t, run.ID, op, "cloud candidate\n")
	report := []byte(`{"version":1,"summary":"Built alpha","changedFiles":["alpha.txt"],"decisions":[],"reportedChecks":[{"command":"go test ./...","outcome":"passed","detail":"worker claim only"}],"questions":[],"limitations":[]}`)
	sum := sha256.Sum256(report)
	record.Capture.ReportSHA256 = hex.EncodeToString(sum[:])
	s.SetProjectSwarmMicroVMImportGuard(func(_ context.Context, scope ProjectSwarmMicroVMImportScope) (func(), error) {
		if scope.ProjectID != p.ID || scope.RunID != run.ID || scope.OperationID != op.ID || scope.ChildID != op.ChildID || scope.RequestDigest != record.RequestDigest {
			t.Fatal("cloud report escaped its guarded owner", scope)
		}
		return func() {}, nil
	})
	run, err = s.RecordProjectSwarmMicroVMExit(context.Background(), p.ID, run.ID, op.ID, record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RecordProjectSwarmMicroVMHandoff(context.Background(), p.ID, run.ID, op.ID, record, []byte("tampered")); err == nil {
		t.Fatal("report outside the frozen capture was published")
	}
	if _, err = s.ImportProjectSwarmMicroVMCandidate(context.Background(), p.ID, run.ID, op.ID, record, changes, stage); err != nil {
		t.Fatal(err)
	}
	run, err = s.RecordProjectSwarmMicroVMHandoff(context.Background(), p.ID, run.ID, op.ID, record, report)
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.projectRuns.taskHandoffView(run, run.Operations[0])
	if err != nil || view.Status != "captured" || view.Receipt == nil || view.Receipt.CloudReportDigest != record.Capture.ReportSHA256 ||
		view.Receipt.CloudRequestDigest != record.RequestDigest || view.Receipt.Reported == nil || len(view.Receipt.Reported.ReportedChecks) != 1 || len(view.RecordedChecks) != 0 {
		t.Fatal("cloud worker claim was lost or promoted into independent check evidence", view, err)
	}
	repeated, err := s.RecordProjectSwarmMicroVMHandoff(context.Background(), p.ID, run.ID, op.ID, record, report)
	if err != nil || repeated.Revision != run.Revision {
		t.Fatal("same cloud report replayed a run mutation", err)
	}
	other := record
	other.Capture.ArchiveSHA256 = strings.Repeat("1", 64)
	if _, err = s.RecordProjectSwarmMicroVMHandoff(context.Background(), p.ID, run.ID, op.ID, other, report); err == nil {
		t.Fatal("a different cloud capture replaced the retained report")
	}
}
