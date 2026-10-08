package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

func observationSession(id string) Session {
	row := sampleSession()
	row.ID = id
	row.SandboxID = "sandbox-" + id
	row.SandboxRef = "immutable-" + id
	row.ProjectRunID = "run-" + id
	row.ProjectRunOperationID = "operation-" + id
	row.Status = StatusExited
	return row
}

func observationFixture(t *testing.T, rows []Session) *FileStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sessions.json")
	raw, err := json.Marshal(document{SchemaVersion: schemaVersion, Sessions: rows})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return newStore(t, path)
}

func TestTaskObservationPageBoundsScanAndReturnsOnlyEndedTaskMetadata(t *testing.T) {
	rows := make([]Session, 150)
	for i := range rows {
		rows[i] = observationSession(fmt.Sprintf("row-%03d", i))
		rows[i].Status = StatusRunning
	}
	rows[127].Status = StatusExited
	rows[129].Status = StatusExited
	st := observationFixture(t, rows)
	page, next, err := st.TaskObservationPage(0, 100000, 100000)
	if err != nil || next != 128 || len(page) != 1 {
		t.Fatalf("bounded first page = %v next=%d err=%v", page, next, err)
	}
	want := rows[127]
	if got := page[0]; got != (TaskObservationCandidate{
		ID: want.ID, ProjectID: want.ProjectID, RunID: want.ProjectRunID,
		OperationID: want.ProjectRunOperationID, SandboxID: want.SandboxID,
		SandboxRef: want.SandboxRef, Status: want.Status, UpdatedAt: want.UpdatedAt,
	}) {
		t.Fatalf("candidate generation lost: %#v", got)
	}
	// The page visits exactly the requested positions, including ineligible
	// history, and does not inspect a 129th position to fill its result limit.
	page, next, err = st.TaskObservationPage(next, 2, 8)
	if err != nil || next != 130 || len(page) != 1 || page[0].ID != rows[129].ID {
		t.Fatalf("bounded continuation = %v next=%d err=%v", page, next, err)
	}
	page[0].ID = "changed-caller-copy"
	got, err := st.Get(rows[129].ID)
	if err != nil || got.ID != rows[129].ID {
		t.Fatalf("candidate mutation reached registry: %v %v", got.ID, err)
	}
}

func TestTaskObservationPageCapsResultsWrapsAndSkipsCompletedOrUnboundRows(t *testing.T) {
	rows := make([]Session, 14)
	for i := range rows {
		rows[i] = observationSession(fmt.Sprintf("row-%02d", i))
	}
	rows[0].Status = StatusStarting
	rows[1].Status = StatusIdle
	rows[2].TaskObservationComplete = true
	rows[3].ProjectRunID = ""
	rows[4].ProjectRunOperationID = ""
	st := observationFixture(t, rows)
	page, next, err := st.TaskObservationPage(0, 128, 128)
	if err != nil || len(page) != 8 || next != 13 {
		t.Fatalf("capped page len=%d next=%d err=%v", len(page), next, err)
	}
	for i, row := range page {
		if row.ID != rows[i+5].ID {
			t.Fatalf("ineligible or out-of-order row: %v", page)
		}
	}
	page, next, err = st.TaskObservationPage(next, 2, 8)
	if err != nil || len(page) != 1 || next != 1 || page[0].ID != rows[13].ID {
		t.Fatalf("wrap = %v next=%d err=%v", page, next, err)
	}
	// A small store cannot repeat its eligible rows to fill a requested page.
	st = observationFixture(t, []Session{rows[5]})
	page, next, err = st.TaskObservationPage(99, 128, 8)
	if err != nil || len(page) != 1 || next != 0 {
		t.Fatalf("normalized cursor duplicated tiny history: %v %d %v", page, next, err)
	}
	for _, input := range [][3]int{{-1, 128, 8}, {0, 0, 8}, {0, 128, 0}, {0, -1, 8}, {0, 128, -1}} {
		if _, _, err := st.TaskObservationPage(input[0], input[1], input[2]); err == nil {
			t.Fatalf("accepted invalid page arguments %v", input)
		}
	}
	empty := newStore(t, filepath.Join(t.TempDir(), "absent.json"))
	if page, next, err := empty.TaskObservationPage(7, 128, 8); err != nil || len(page) != 0 || next != 0 {
		t.Fatalf("empty page = %v %d %v", page, next, err)
	}
}

func TestTaskObservationWatermarkSurvivesRestartAndOldRowsBackfill(t *testing.T) {
	row := observationSession("old-record")
	st := observationFixture(t, []Session{row})
	raw, err := os.ReadFile(st.path)
	if err != nil || strings.Contains(string(raw), "taskObservationComplete") {
		t.Fatalf("old omitted watermark fixture invalid: %v", err)
	}
	page, _, err := st.TaskObservationPage(0, 128, 8)
	if err != nil || len(page) != 1 {
		t.Fatalf("old task was not available for backfill: %v %v", page, err)
	}
	marked, err := st.Update(row.ID, func(current *Session) error {
		current.TaskObservationComplete = true
		return nil
	})
	if err != nil || !marked.TaskObservationComplete {
		t.Fatalf("watermark update: %v %v", marked.TaskObservationComplete, err)
	}
	reopened := newStore(t, st.path)
	got, err := reopened.Get(row.ID)
	if err != nil || !got.TaskObservationComplete {
		t.Fatalf("watermark did not persist: %v %v", got.TaskObservationComplete, err)
	}
	page, _, err = reopened.TaskObservationPage(0, 128, 8)
	if err != nil || len(page) != 0 {
		t.Fatalf("completed watermark was observed again: %v %v", page, err)
	}
	// It remains a private observation fact; it does not change the failure.
	if got.Status != row.Status || got.ExitReason != row.ExitReason {
		t.Fatal("watermark changed the task outcome")
	}
}

func TestProjectRunSessionsPreservesExactConflictUnionAndDetachedRecords(t *testing.T) {
	matching := observationSession("z-matching")
	foreign := observationSession("a-foreign")
	foreign.ProjectID = "another-project"
	foreign.ProjectRunID = "another-run"
	foreign.ProjectRunOperationID = matching.ProjectRunOperationID
	unrelated := observationSession("unrelated")
	unrelated.ProjectRunID = matching.ProjectRunID // different nonempty operation is excluded
	st := observationFixture(t, []Session{matching, unrelated, foreign})
	rows, err := st.ProjectRunSessions(matching.ProjectRunID, matching.ProjectRunOperationID)
	if err != nil || len(rows) != 2 || rows[0].ID != foreign.ID || rows[1].ID != matching.ID {
		t.Fatalf("foreign binding hidden or candidate order changed: %v %v", sessionIDs(rows), err)
	}
	rows[0].Args[0] = "changed"
	got, _ := st.Get(foreign.ID)
	if got.Args[0] == "changed" {
		t.Fatal("indexed lookup returned an aliased record")
	}
	legacy := observationSession("legacy")
	legacy.ProjectRunID, legacy.ProjectRunOperationID = matching.ProjectRunID, ""
	if err := st.Create(legacy); err != nil {
		t.Fatal(err)
	}
	if rows, err := st.ProjectRunSessions(matching.ProjectRunID, matching.ProjectRunOperationID); err == nil || rows != nil {
		t.Fatalf("three claimants were not refused: %v %v", sessionIDs(rows), err)
	}
	if err := st.Delete(foreign.ID); err != nil {
		t.Fatal(err)
	}
	rows, err = st.ProjectRunSessions(matching.ProjectRunID, matching.ProjectRunOperationID)
	if err != nil || len(rows) != 2 || rows[0].ID != legacy.ID || rows[1].ID != matching.ID {
		t.Fatalf("legacy candidate missing: %v %v", sessionIDs(rows), err)
	}
	for _, input := range [][2]string{{"", "op"}, {"run", ""}, {"", ""}} {
		if _, err := st.ProjectRunSessions(input[0], input[1]); err == nil {
			t.Fatalf("accepted empty indexed lookup identity %v", input)
		}
	}
}

func TestProjectRunSessionsIndexesFollowCommittedRebindingAndLoad(t *testing.T) {
	row := observationSession("moves")
	st := observationFixture(t, []Session{row, row, {ID: ""}})
	if len(st.order) != 1 {
		t.Fatal("fixture duplicate/invalid rows not filtered")
	}
	assertLookup := func(run, op string, want []string) {
		t.Helper()
		got, err := st.ProjectRunSessions(run, op)
		if err != nil || !slices.Equal(sessionIDs(got), want) {
			t.Fatalf("lookup %s/%s = %v %v, want %v", run, op, sessionIDs(got), err, want)
		}
	}
	assertLookup(row.ProjectRunID, row.ProjectRunOperationID, []string{row.ID})
	if _, err := st.Update(row.ID, func(current *Session) error {
		current.ProjectRunID, current.ProjectRunOperationID = "new-run", "new-op"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	assertLookup(row.ProjectRunID, row.ProjectRunOperationID, nil)
	assertLookup("new-run", "new-op", []string{row.ID})
	if _, err := st.Update(row.ID, func(current *Session) error {
		current.ProjectRunOperationID = ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	assertLookup("other-run", "new-op", nil)
	assertLookup("new-run", "different-operation", []string{row.ID})
	st = newStore(t, st.path)
	assertLookup("new-run", "different-operation", []string{row.ID})
	if err := st.Delete(row.ID); err != nil {
		t.Fatal(err)
	}
	assertLookup("new-run", "different-operation", nil)
	if len(st.operationSessions) != 0 || len(st.legacyRunSessions) != 0 {
		t.Fatal("empty index buckets were retained after deletion")
	}
}

func TestTaskObservationIndexesRollbackAllFailedPublications(t *testing.T) {
	for _, action := range []string{"create", "update", "delete", "mutate-error", "changed-id"} {
		t.Run(action, func(t *testing.T) {
			row := observationSession("original")
			st := observationFixture(t, []Session{row})
			originalPath := st.path
			before, err := os.ReadFile(originalPath)
			if err != nil {
				t.Fatal(err)
			}
			// A nonempty directory makes atomic publication fail on Unix and
			// Windows without depending on root's permission behavior.
			blocked := filepath.Join(t.TempDir(), "directory-target")
			if err := os.Mkdir(blocked, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(blocked, "keep"), []byte("unchanged"), 0o600); err != nil {
				t.Fatal(err)
			}
			st.path = blocked
			switch action {
			case "create":
				err = st.Create(observationSession("uncommitted"))
			case "update", "mutate-error", "changed-id":
				_, err = st.Update(row.ID, func(current *Session) error {
					current.ProjectRunID, current.ProjectRunOperationID = "rebound-run", ""
					current.TaskObservationComplete = true
					if action == "mutate-error" {
						return errors.New("mutation refused")
					}
					if action == "changed-id" {
						current.ID = "changed-id"
					}
					return nil
				})
			case "delete":
				err = st.Delete(row.ID)
			}
			if err == nil {
				t.Fatal("expected publication/mutation failure")
			}
			st.path = originalPath
			fresh := newStore(t, originalPath)
			if !reflect.DeepEqual(st.order, fresh.order) || !reflect.DeepEqual(st.sessions, fresh.sessions) ||
				!reflect.DeepEqual(st.operationSessions, fresh.operationSessions) || !reflect.DeepEqual(st.legacyRunSessions, fresh.legacyRunSessions) {
				t.Fatal("failed publication changed in-memory observation indexes or rows")
			}
			got, next, err := st.TaskObservationPage(0, 128, 8)
			if err != nil || len(got) != 1 || got[0].ID != row.ID || next != 0 {
				t.Fatalf("failed publication changed observation page: %v %d %v", got, next, err)
			}
			after, err := os.ReadFile(originalPath)
			if err != nil || string(before) != string(after) {
				t.Fatalf("original document changed on failed publication: %v", err)
			}
		})
	}
}

func TestTaskObservationReadersRemainConsistentWithConcurrentUpdates(t *testing.T) {
	row := observationSession("concurrent")
	st := observationFixture(t, []Session{row})
	var workers sync.WaitGroup
	for i := 0; i < 3; i++ {
		workers.Go(func() {
			for n := 0; n < 24; n++ {
				page, _, err := st.TaskObservationPage(0, 128, 8)
				if err != nil || len(page) > 1 {
					t.Errorf("concurrent page: %v %v", page, err)
					return
				}
				rows, err := st.ProjectRunSessions(row.ProjectRunID, row.ProjectRunOperationID)
				if err != nil || len(rows) != 1 || rows[0].ID != row.ID {
					t.Errorf("concurrent indexed lookup: %v %v", sessionIDs(rows), err)
					return
				}
			}
		})
	}
	for i := 0; i < 12; i++ {
		if _, err := st.Update(row.ID, func(current *Session) error {
			current.TaskObservationComplete = !current.TaskObservationComplete
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	workers.Wait()
}

func TestTaskObservationWatermarkResetsOnlyForChangedTaskGeneration(t *testing.T) {
	changes := map[string]func(*Session){
		"project":               func(s *Session) { s.ProjectID += "new" },
		"worktree":              func(s *Session) { s.WorktreeID += "new" },
		"run":                   func(s *Session) { s.ProjectRunID += "new" },
		"operation":             func(s *Session) { s.ProjectRunOperationID += "new" },
		"sandbox":               func(s *Session) { s.SandboxID += "new" },
		"immutable-ref":         func(s *Session) { s.SandboxRef += "new" },
		"status":                func(s *Session) { s.Status = StatusRunning },
		"exit-reason":           func(s *Session) { s.ExitReason += "new" },
		"start-failure":         func(s *Session) { s.StartFailure = NewSessionStartFailure("admission", "dispatch_paused") },
		"harness":               func(s *Session) { s.Harness += "new" },
		"model":                 func(s *Session) { s.Model += "new" },
		"effort":                func(s *Session) { s.Effort += "new" },
		"permission":            func(s *Session) { s.PermissionMode += "new" },
		"auth":                  func(s *Session) { s.AuthMode += "new" },
		"connection":            func(s *Session) { s.AgentConnectionID += "new" },
		"connection-kind":       func(s *Session) { s.AgentConnectionKind += "new" },
		"connection-protocol":   func(s *Session) { s.AgentConnectionProtocol += "new" },
		"connection-revision":   func(s *Session) { s.AgentConnectionRevision++ },
		"private-connection":    func(s *Session) { s.CodexConnectionID += "new" },
		"implementation":        func(s *Session) { s.ImplementationContext.SourceDigest += "new" },
		"nested-implementation": func(s *Session) { s.ImplementationContext.TaskIDs[0] += "new" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			row := observationSession("already-observed")
			row.TaskObservationComplete = true
			row.ImplementationContext = &ImplementationContext{TaskIDs: []string{"task-original"}, SourceDigest: "original"}
			st := observationFixture(t, []Session{row})
			changed, err := st.Update(row.ID, func(current *Session) error {
				change(current)
				current.TaskObservationComplete = true // stale carried marker cannot win
				return nil
			})
			if err != nil || changed.TaskObservationComplete {
				t.Fatalf("new generation retained old watermark: %v %v", changed.TaskObservationComplete, err)
			}
			if name == "status" {
				page, _, err := st.TaskObservationPage(0, 128, 8)
				if err != nil || len(page) != 0 {
					t.Fatal("restarted running row entered ended task page", err)
				}
				if _, err = st.Update(row.ID, func(current *Session) error { current.Status = StatusExited; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			st = newStore(t, st.path)
			page, _, err := st.TaskObservationPage(0, 128, 8)
			if err != nil || len(page) != 1 || page[0].ID != row.ID {
				t.Fatalf("new ended generation did not reappear after reopen: %v %v", page, err)
			}
		})
	}
	t.Run("accounting-and-display", func(t *testing.T) {
		row := observationSession("observed")
		row.TaskObservationComplete = true
		row.ImplementationContext = &ImplementationContext{TaskIDs: []string{"task-original"}}
		st := observationFixture(t, []Session{row})
		changed, err := st.Update(row.ID, func(current *Session) error {
			current.Title = "renamed by user"
			current.AttachCount++
			current.LastOutputAt = current.LastOutputAt.Add(1)
			current.UpdatedAt = current.UpdatedAt.Add(1)
			current.ImplementationContext = row.ImplementationContext.Clone()
			return nil
		})
		if err != nil || !changed.TaskObservationComplete {
			t.Fatalf("accounting unnecessarily requeued output: %v %v", changed.TaskObservationComplete, err)
		}
		page, _, err := newStore(t, st.path).TaskObservationPage(0, 128, 8)
		if err != nil || len(page) != 0 {
			t.Fatal("unchanged task was requeued after reopen", err)
		}
	})
	t.Run("failed-publication-keeps-old-watermark", func(t *testing.T) {
		row := observationSession("observed")
		row.TaskObservationComplete = true
		st := observationFixture(t, []Session{row})
		path := st.path
		st.path = filepath.Join(t.TempDir(), "blocking-file", "sessions.json")
		if err := os.WriteFile(filepath.Dir(st.path), []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := st.Update(row.ID, func(current *Session) error { current.SandboxRef += "new"; return nil }); err == nil {
			t.Fatal("expected failed publication")
		}
		st.path = path
		for _, store := range []*FileStore{st, newStore(t, path)} {
			got, err := store.Get(row.ID)
			if err != nil || !got.TaskObservationComplete || got.SandboxRef != row.SandboxRef {
				t.Fatalf("failed publication changed old generation: %#v %v", got, err)
			}
			page, _, err := store.TaskObservationPage(0, 128, 8)
			if err != nil || len(page) != 0 {
				t.Fatal("failed new generation was requeued", err)
			}
		}
	})
}

func sessionIDs(rows []Session) []string {
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return ids
}
