package product

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Actual public handlers serialize controlled retained outcomes. These rows
// exercise the wire contract; they are not evidence that a Docker command ran.
func TestProjectCheckHTTPSharedContract(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	p := createJourneyProject(t, mux, "check-wire")
	base := "/v1/product/projects/" + p.ID + "/checks"
	held := false
	zero := 0
	row := ProjectCheckRun{ID: "check-wire-contract", ProjectID: p.ID, Revision: 3, ResourcesHeld: &held, Execution: "docker", MemoryMiB: 1024, CPUMillis: 1000, SourceDigest: strings.Repeat("a", 64), Commit: strings.Repeat("b", 40), EvidenceStatus: evidenceUnverified, StaleReasons: []string{}, Status: "blocked", Stage: "admission", ReasonCode: "memory_headroom", Message: "Fresh shared memory capacity does not admit this command.", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z", Commands: []CheckCommandResult{
		{CheckCommand: CheckCommand{Label: "Earlier command", Executable: "node", Args: []string{}}, AllocationID: "checkexec-wire-first", Status: "passed", ExitCode: &zero, Output: "earlier controlled output\n", CleanupConfirmed: true, Stage: "cleanup"},
		{CheckCommand: CheckCommand{Label: "Next command", Executable: "go", Args: []string{"test", "./..."}}, AllocationID: "checkexec-wire-second", Status: "blocked", Output: "", Stage: "admission", ReasonCode: "memory_headroom", Message: "Fresh shared memory capacity does not admit this command."},
	}}
	save := func() {
		t.Helper()
		s.mu.Lock()
		err := s.storeCheckRun(row)
		s.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	}
	save()
	recovery := ProjectCheckRecoveryView{ProjectID: p.ID, CheckID: row.ID, Revision: row.Revision, State: "complete", ReasonCode: "check_recovery_not_required", Message: "This check has no retained allocation requiring recovery."}
	s.projectCheckRuntime.Recovery = func(_ context.Context, projectID, checkID string) (ProjectCheckRecoveryView, error) {
		if projectID != p.ID || checkID != row.ID {
			t.Fatal("wrong recovery identity")
		}
		return recovery, nil
	}
	s.projectCheckRuntime.Recover = func(_ context.Context, projectID, checkID string, req ProjectCheckRecoveryRequest) (ProjectCheckRun, error) {
		if projectID != p.ID || checkID != row.ID || req.ExpectedRevision != row.Revision || recovery.RecoveryDigest == nil || req.RecoveryDigest != *recovery.RecoveryDigest || !req.AcknowledgeStop {
			return ProjectCheckRun{}, ErrProjectRunConflict
		}
		row.Revision++
		held = false
		row.ResourcesHeld = &held
		row.Status = "passed"
		row.Stage = "cleanup"
		row.ReasonCode = ""
		row.Message = "The exact check cleanup is confirmed; recorded output remains retained."
		row.Commands[0].CleanupConfirmed = true
		save()
		return snapshotProjectCheck(row), nil
	}
	cases := map[string]json.RawMessage{}
	read := func(name, path string) {
		t.Helper()
		body := journeyRequest(t, mux, "GET", path, nil, 200).Body.Bytes()
		cases[name] = append(json.RawMessage(nil), body...)
	}
	read("blocked", base+"/"+row.ID)
	read("complete", base+"/"+row.ID+"/recovery")
	row.Revision++
	held = true
	row.ResourcesHeld = &held
	row.Status = "unknown"
	row.Stage = "cleanup"
	row.ReasonCode = "check_cleanup_unconfirmed"
	row.Message = "The command result is retained, but exact Docker cleanup is unconfirmed. Its capacity remains held."
	row.Commands = row.Commands[:1]
	row.Commands[0].CleanupConfirmed = false
	row.Commands[0].ReasonCode = row.ReasonCode
	row.Commands[0].Message = row.Message
	save()
	read("unknownCleanup", base+"/"+row.ID)
	method, index, digest := "remove-exact", 0, strings.Repeat("c", 64)
	recovery = ProjectCheckRecoveryView{ProjectID: p.ID, CheckID: row.ID, Revision: row.Revision, State: "available", Method: &method, CommandIndex: &index, RecoveryDigest: &digest, ReasonCode: "check_stop_review_required", Message: "Stop only this exact check allocation and retain its recorded output. No command will be replayed."}
	read("available", base+"/"+row.ID+"/recovery")
	recovery.State = "pending"
	recovery.ReasonCode = "check_recovery_pending"
	recovery.Message = "The prior reviewed check recovery remains incomplete. Review its current state to finish the same recovery."
	read("pending", base+"/"+row.ID+"/recovery")
	body := journeyRequest(t, mux, "POST", base+"/"+row.ID+"/recovery", ProjectCheckRecoveryRequest{ExpectedRevision: row.Revision, RecoveryDigest: digest, AcknowledgeStop: true}, 200).Body.Bytes()
	cases["recovered"] = append(json.RawMessage(nil), body...)
	raw, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	var actual any
	if err = json.Unmarshal(raw, &actual); err != nil {
		t.Fatal(err)
	}
	actual = normalizeTaskHandoffWire(actual, map[string]string{p.ID: "project-fixture"})
	rendered, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if output := os.Getenv("LUCI_CHECK_HTTP_CAPTURE"); output != "" {
		if !filepath.IsAbs(output) {
			t.Fatal("capture path must be absolute")
		}
		if err = os.WriteFile(output, append(rendered, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Capture is deliberately external. The shared fixture must be populated
	// from an observed handler result before the final contract assertion passes.
	expectedPath := "testdata/project-check-http.json"
	if external := os.Getenv("LUCI_CHECK_HTTP_EXPECTED"); external != "" {
		if !filepath.IsAbs(external) {
			t.Fatal("overlay expected path must be absolute")
		}
		expectedPath = external // Go overlays do not intercept runtime os.ReadFile.
	}
	expectedBytes, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatal(err)
	}
	var expected any
	if err = json.Unmarshal(expectedBytes, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("actual check handler contract differs from shared fixture:\n%s", rendered)
	}
}
