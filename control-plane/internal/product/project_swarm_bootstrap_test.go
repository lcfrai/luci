package product

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWindowsSwarmBootstrapFeasibilityBoundaries(t *testing.T) {
	// The longest fixed bootstrap directory has17 ASCII units. These bases
	// reach exactly248 units with separator and NUL, including a surrogate pair.
	boundary := "C:/" + strings.Repeat("a", 226)
	unicodeBoundary := "C:/" + strings.Repeat("a", 224) + "😀"
	for _, tc := range []struct {
		name string
		path string
		wantError bool
	}{
		{"ordinary drive", `C:\projects\.luci-swarms\p\run-a\op-a`, false},
		{"ordinary UNC", `\\server\share\projects\task`, false},
		{"directory boundary", boundary, false},
		{"directory overflow", boundary + "a", true},
		{"surrogate boundary", unicodeBoundary, false},
		{"surrogate overflow", unicodeBoundary + "a", true},
		{"relative", "projects/task", true},
		{"drive relative", "C:projects/task", true},
		{"parent traversal", "C:/projects/../task", true},
		{"dot component", "C:/projects/./task", true},
		{"verbatim", `\\?\C:\projects\task`, true},
		{"device", `\\.\C:\projects\task`, true},
		{"incomplete UNC", `\\server`, true},
		{"NUL", "C:/projects/task\x00", true},
		{"invalid UTF8", "C:/projects/" + string([]byte{0xff}), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := windowsSwarmBootstrapFeasibility(tc.path)
			if (err != nil) != tc.wantError || err != nil && !errors.Is(err, errSwarmBootstrapPath) {
				t.Fatalf("unexpected path-policy result: %v", err)
			}
		})
	}
	if !windowsBootstrapUnitsFit(strings.Repeat("a", 242), ".git/config.lock", 260) || windowsBootstrapUnitsFit(strings.Repeat("a", 243), ".git/config.lock", 260) {
		t.Fatal("file budget must count the separator and terminating NUL")
	}
}

type bootstrapHandoffInventory struct {
	Exists bool
	Entries map[string]string
}

func bootstrapHandoffSnapshot(t *testing.T, store *ProjectRunStore, projectID string) bootstrapHandoffInventory {
	t.Helper()
	dir := filepath.Join(store.stateDir, "task-handoffs", projectID)
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		return bootstrapHandoffInventory{}
	} else if err != nil {
		t.Fatal(err)
	}
	// auditTree includes directory names, file names, modes and hashes of all
	// bytes. A final run-row comparison alone would miss orphan handoff writes.
	return bootstrapHandoffInventory{Exists: true, Entries: auditTree(t, dir)}
}

func TestSwarmReservationBootstrapRefusalHasNoDurableAttempt(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing-handoff=%v", existing), func(t *testing.T) {
			store, id, projectPath, run := swarmStoreFixture(t, []ProjectSwarmTask{storedSwarmTask("a", "src/a"), storedSwarmTask("b", "src/b")}, 2)
			var retained ProjectRunOperation
			if existing {
				run, retained = reserveStoredSwarmTask(t, store, projectPath, run, "b")
			}
			scope, err := store.projectScope(id, projectPath)
			if err != nil {
				t.Fatal(err)
			}
			beforeRow, err := store.read(id, scope, run.ID)
			if err != nil {
				t.Fatal(err)
			}
			beforeState, beforeProject := auditTree(t, store.stateDir), auditTree(t, projectPath)
			beforeHandoffs := bootstrapHandoffSnapshot(t, store, id)
			if beforeHandoffs.Exists != existing {
				t.Fatal("fixture did not establish the intended handoff inventory")
			}
			calls := 0
			for _, key := range []string{"path-refused-attempt-123456", "path-refused-attempt-123456", "path-refused-fresh-key-123456"} {
				var pathErr error
				_, _, created, reserveErr := store.ReserveSwarmTask(id, projectPath, run.ID, run.Revision, key, "a", run.Input.Implementation.SourceCommit, func(operationID string) string {
					calls++
					if !strings.HasPrefix(operationID, "op-") {
						t.Fatal("callback did not receive the real new operation identity")
					}
					pathErr = windowsSwarmBootstrapFeasibility("C:/" + strings.Repeat("long-projects/", 30) + operationID)
					return ""
				})
				if created || !errors.Is(reserveErr, ErrProjectRunInvalid) || !errors.Is(pathErr, errSwarmBootstrapPath) {
					t.Fatal("refused path was reserved or never reached the boundary", created, reserveErr, pathErr)
				}
				row, readErr := store.read(id, scope, run.ID)
				if readErr != nil || !reflect.DeepEqual(beforeRow, row) || !reflect.DeepEqual(beforeState, auditTree(t, store.stateDir)) || !reflect.DeepEqual(beforeProject, auditTree(t, projectPath)) || !reflect.DeepEqual(beforeHandoffs, bootstrapHandoffSnapshot(t, store, id)) {
					t.Fatal("refusal changed revision, task/operation/key history, directory or handoff object bytes", readErr)
				}
			}
			if calls != 3 {
				t.Fatal("a refusal accidentally published an idempotency key", calls)
			}
			if existing {
				// Historical duplicate lookup precedes the new-work callback, even
				// with an old revision. It must neither allocate nor rewrite evidence.
				duplicateRun, duplicate, created, duplicateErr := store.ReserveSwarmTask(id, projectPath, run.ID, 1, "reserve-task-b-123456789", "b", retained.BaseCommit, func(string) string {
					t.Fatal("historical duplicate entered new bootstrap admission")
					return ""
				})
				if duplicateErr != nil || created || duplicate.ID != retained.ID || duplicateRun.Revision != run.Revision || !reflect.DeepEqual(beforeState, auditTree(t, store.stateDir)) {
					t.Fatal("historical readback changed", duplicateErr, created)
				}
			}
		})
	}
}

func TestSwarmCheckpointBootstrapRefusalPreservesOriginal(t *testing.T) {
	store, id, projectPath, run := swarmStoreFixture(t, []ProjectSwarmTask{storedSwarmTask("a", "src/a")}, 1)
	run, prior := reserveStoredSwarmTask(t, store, projectPath, run, "a")
	run = allocateStoredSwarmTask(t, store, projectPath, run, prior)
	var err error
	run, err = store.ObserveOperation(id, projectPath, run.ID, run.Revision, prior.ID, prior.ChildID, "failed", "controlled stopped-attempt fixture")
	if err != nil {
		t.Fatal(err)
	}
	prior = *swarmOperation(&run, prior.ID)
	checkpoint := ProjectSwarmCheckpoint{SourceOperationID: prior.ID, SourceChildID: prior.ChildID, SourceCommit: prior.BaseCommit, SourceBaseCommit: prior.BaseCommit, TargetCommit: prior.BaseCommit, ReviewDigest: strings.Repeat("a", 64), ContextDigest: implementationDigest(prior.Implementation), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	beforeState, beforeProject := auditTree(t, store.stateDir), auditTree(t, projectPath)
	beforeHandoffs := bootstrapHandoffSnapshot(t, store, id)
	for i := range 2 {
		calls := 0
		_, _, created, reserveErr := store.reserveSwarmTaskGated(id, projectPath, run.ID, run.Revision, fmt.Sprintf("refused-checkpoint-%d-123456", i), "a", prior.BaseCommit, func(string) string { calls++; return "" }, &checkpoint, nil)
		current, readErr := store.Get(id, projectPath, run.ID)
		if calls != 1 || created || !errors.Is(reserveErr, ErrProjectRunInvalid) || readErr != nil || !reflect.DeepEqual(current, run) || !reflect.DeepEqual(beforeState, auditTree(t, store.stateDir)) || !reflect.DeepEqual(beforeProject, auditTree(t, projectPath)) || !reflect.DeepEqual(beforeHandoffs, bootstrapHandoffSnapshot(t, store, id)) {
			t.Fatal("refused checkpoint changed original attempt, repair/dependency evidence or history", calls, created, reserveErr, readErr)
		}
	}
}
