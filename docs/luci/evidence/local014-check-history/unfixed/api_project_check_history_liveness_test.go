package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// These are actual HTTP/product/ledger completions with a controlled backend,
// not injected lease rows or Docker execution. The fixture changes source during
// each approved run, so successful exits remain truthfully unknown evidence
// even after exact cleanup and completion ACK have released every allocation.
func TestProjectCheckHTTPSettledUnknownHistoryAllowsNextCheck(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	be := newHTTPCheckExecutor(f.env.be)
	wireHTTPCheckFixture(t, f, be, filepath.Join(t.TempDir(), "capacity"), "user-check", 12)
	be.prepareHook = func(_ context.Context, spec backend.CheckSpec) error {
		return os.WriteFile(filepath.Join(spec.SourceDirectory, "changed-during-check.txt"), []byte(spec.Identity.CheckID), 0600)
	}
	var firstID, newestID string
	for attempt := 0; attempt < 21; attempt++ {
		w := checkHTTPDo(f, "user-check", "POST", "/v1/product/projects/"+f.owner.ID+"/checks", checkHTTPBody())
		var started product.ProjectCheckRun
		if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &started) != nil || started.ID == "" {
			t.Fatalf("check %d refused after settled unknown history: HTTP %d %s", attempt+1, w.Code, w.Body.String())
		}
		if attempt == 0 {
			firstID = started.ID
		}
		newestID = started.ID
		row := checkHTTPWait(t, f, "user-check", started.ID, func(row product.ProjectCheckRun) bool {
			return row.Status == "unknown" && row.SourceChanged && row.ResourcesHeld != nil && !*row.ResourcesHeld
		})
		if len(row.Commands) != 1 || row.Commands[0].Status != "passed" || row.Commands[0].ExitCode == nil || *row.Commands[0].ExitCode != 0 || !row.Commands[0].CleanupConfirmed || row.Commands[0].Output != be.output {
			t.Fatal("source change did not preserve actual settled command outcome")
		}
		checkHTTPIdle(t, f, row)
		checkHTTPCompletionAcknowledged(t, f, row)
		ledger, err := f.env.api.execution.readFenceLedger()
		if err != nil || len(ledger.Checks) != 0 {
			t.Fatal("completion ACK left an active or finished lease", err)
		}
		view, err := f.env.api.projectCheckRecovery(context.Background(), row.ProjectID, row.ID)
		if err != nil || view.State != "complete" {
			t.Fatal("settled source-changing check still required recovery", view.State, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(f.options.StateDir, "project-checks", f.owner.ID+".json"))
	var history []product.ProjectCheckRun
	if err != nil || json.Unmarshal(data, &history) != nil || len(history) != 20 {
		t.Fatal("history did not remain bounded at twenty", err, len(history))
	}
	foundNewest := false
	for _, row := range history {
		if row.ID == firstID {
			t.Fatal("oldest settled row was not pruned")
		}
		foundNewest = foundNewest || row.ID == newestID
		if row.Status != "unknown" || !row.SourceChanged || row.ResourcesHeld == nil || *row.ResourcesHeld || !row.Commands[0].CleanupConfirmed {
			t.Fatal("retention changed truthful evidence or resource ownership")
		}
	}
	if !foundNewest {
		t.Fatal("newest run was lost")
	}
	_, creates, starts, _, removes := be.counts()
	if creates != 21 || starts != 21 || removes != 21 {
		t.Fatal("unexpected allocation/replay during bounded history", creates, starts, removes)
	}
}
