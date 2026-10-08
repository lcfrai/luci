package product

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func settledUnknownHistoryRun(id string) ProjectCheckRun {
	held, zero := false, 0
	return ProjectCheckRun{ID: id, ProjectID: "project-history", Revision: 4, Execution: "docker", Status: "unknown", SourceChanged: true, ResourcesHeld: &held,
		Commands: []CheckCommandResult{{AllocationID: "checkexec-" + id, Status: "passed", ExitCode: &zero, CleanupConfirmed: true, Output: "retained exact output"}}}
}

// Exercise retention itself for ownership shapes that cannot be manufactured
// by the normal successful HTTP fixture. Twenty unresolved rows must never be
// displaced merely to allow another command, including legacy/missing markers.
func TestProjectCheckHistoryPreservesUnresolvedOwnership(t *testing.T) {
	for name, alter := range map[string]func(*ProjectCheckRun){
		"missing-held":        func(row *ProjectCheckRun) { row.ResourcesHeld = nil },
		"held-unknown":        func(row *ProjectCheckRun) { *row.ResourcesHeld = true },
		"held-passed":         func(row *ProjectCheckRun) { row.Status = "passed"; *row.ResourcesHeld = true },
		"missing-allocation":  func(row *ProjectCheckRun) { row.Commands[0].AllocationID = "" },
		"unconfirmed-cleanup": func(row *ProjectCheckRun) { row.Commands[0].CleanupConfirmed = false },
		"missing-revision":    func(row *ProjectCheckRun) { row.Revision = 0 },
		"missing-commands":    func(row *ProjectCheckRun) { row.Commands = nil },
		"running-command": func(row *ProjectCheckRun) {
			row.Commands[0].Status, row.Commands[0].ExitCode, row.Commands[0].CleanupConfirmed = "running", nil, false
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := &Server{stateDir: t.TempDir()}
			for i := 0; i < 20; i++ {
				row := settledUnknownHistoryRun(fmt.Sprintf("old-%d", i))
				alter(&row)
				if err := s.storeCheckRun(row); err != nil {
					t.Fatal("could not retain unresolved history", err)
				}
			}
			path := s.checkHistoryPath("project-history")
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			next := settledUnknownHistoryRun("new-command")
			next.Status, next.Commands[0].Status, next.Commands[0].CleanupConfirmed = "queued", "queued", false
			if err := s.storeCheckRun(next); err == nil {
				t.Fatal("new command displaced unresolved ownership")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("failed admission changed retained ownership history", err)
			}
		})
	}
}

func TestProjectCheckHistoryPrunesSettledUnknownEvidence(t *testing.T) {
	for _, mode := range []string{"source-changed", "exit-unknown", "remaining-skipped"} {
		t.Run(mode, func(t *testing.T) {
			s := &Server{stateDir: t.TempDir()}
			for i := 0; i < 20; i++ {
				row := settledUnknownHistoryRun(fmt.Sprintf("old-%d", i))
				if mode == "exit-unknown" {
					row.SourceChanged, row.Commands[0].Status, row.Commands[0].ExitCode = false, "unknown", nil
				} else if mode == "remaining-skipped" {
					row.Commands = append(row.Commands, CheckCommandResult{AllocationID: "unused-" + row.ID, Status: "skipped"})
				}
				if err := s.storeCheckRun(row); err != nil {
					t.Fatal(err)
				}
			}
			next := settledUnknownHistoryRun("new-command")
			next.Status, next.Commands[0].Status, next.Commands[0].CleanupConfirmed = "queued", "queued", false
			if err := s.storeCheckRun(next); err != nil {
				t.Fatal("settled unknown evidence blocked a new reservation", err)
			}
			data, err := os.ReadFile(s.checkHistoryPath(next.ProjectID))
			var rows []ProjectCheckRun
			if err != nil || json.Unmarshal(data, &rows) != nil || len(rows) != 20 || rows[0].ID != next.ID {
				t.Fatal("bounded history lost newest reservation", err, len(rows))
			}
			for _, row := range rows {
				if row.ID == "old-0" {
					t.Fatal("oldest settled evidence was not evicted")
				}
			}
		})
	}
}
