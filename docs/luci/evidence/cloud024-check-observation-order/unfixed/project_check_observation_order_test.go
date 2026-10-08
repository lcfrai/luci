package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Exercise production HTTP execution/publication, retaining only the final ACK
// as a controlled fault. A rotating observer can visit a later finished command
// first; its disappearance must not turn an earlier receipt into permission to
// rewrite the later command or the final run. No backend command is replayed.
func TestProjectCheckHTTPFinishedObservationPreservesLaterCommandOutcome(t *testing.T) {
	for _, mode := range []string{"passed", "failed", "prepare-blocked", "cleanup-pending"} {
		t.Run(mode, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t)
			be := newHTTPCheckExecutor(f.env.be)
			wireHTTPCheckFixture(t, f, be, filepath.Join(t.TempDir(), "capacity"), "user-check", 12)
			be.prepareHook = func(_ context.Context, spec backend.CheckSpec) error {
				if spec.Identity.CommandIndex != 1 {
					return nil
				}
				if mode == "prepare-blocked" {
					return backend.ErrUnavailable
				}
				be.mu.Lock()
				defer be.mu.Unlock()
				be.output = "second command exact output\n"
				if mode == "failed" {
					be.exitCode = 7
				}
				return nil
			}
			be.removeHook = func(identity backend.CheckIdentity, _ string) error {
				if mode == "cleanup-pending" && identity.CommandIndex == 1 {
					return errors.New("controlled second command cleanup interruption")
				}
				return nil
			}
			a := f.env.api
			a.deps.Product.SetProjectCheckRuntime(product.ProjectCheckRuntime{
				CaptureActor: a.captureCheckActor, AuthorizeControl: a.authorizeExecutionRetirement,
				Execute: a.executeProjectCheck, Busy: a.projectCheckBusy,
				Recovery: a.projectCheckRecovery, Recover: a.recoverProjectCheck,
				AcknowledgeCompleted: func(context.Context, string, string) error {
					return errors.New("controlled final completion acknowledgment interruption")
				},
			})
			body := `{"execution":"docker","acknowledgeHostExecution":true,"commands":[{"label":"First","executable":"go","args":["test","./first"]},{"label":"Second","executable":"go","args":["test","./second"]}]}`
			w := checkHTTPDo(f, "user-check", "POST", "/v1/product/projects/"+f.owner.ID+"/checks", body)
			var started product.ProjectCheckRun
			if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &started) != nil || len(started.Commands) != 2 {
				t.Fatal("two-command check consent", w.Code, w.Body.String())
			}
			wantStatus := mode
			if mode == "prepare-blocked" {
				wantStatus = "blocked"
			} else if mode == "cleanup-pending" {
				wantStatus = "unknown"
			}
			row := checkHTTPWait(t, f, "user-check", started.ID, func(row product.ProjectCheckRun) bool {
				return row.Status == wantStatus && row.ResourcesHeld != nil && *row.ResourcesHeld == (mode == "cleanup-pending")
			})
			checkHTTPIdle(t, f, row)
			ledger, err := a.execution.readFenceLedger()
			wantLeases := 2
			if mode == "prepare-blocked" {
				wantLeases = 1
			}
			if err != nil || len(ledger.Checks) != wantLeases || ledger.Checks[0].State != "finished" {
				t.Fatal("expected exact retained completion receipts", err, ledger.Checks)
			}
			beforeCalls := [5]int{}
			beforeCalls[0], beforeCalls[1], beforeCalls[2], beforeCalls[3], beforeCalls[4] = be.counts()
			historyPath := filepath.Join(f.options.StateDir, "project-checks", started.ProjectID+".json")
			// Set a valid pagination position, as retained after visiting an
			// earlier page containing live jobs. Use the real observer turn.
			if mode == "passed" || mode == "failed" {
				a.execution.checkObserver.cursor = 1
				if err = a.ObserveChecksOnce(context.Background()); err != nil {
					t.Fatal("reverse completion observation", err)
				}
			} else {
				// A blocked preparation has no later lease. An unsettled cleanup
				// does; observing only its earlier receipt must not release it.
				if err = a.observeCheckCandidate(context.Background(), ledger.Checks[0]); err != nil {
					t.Fatal("earlier completion observation", err)
				}
			}
			after := checkHTTPRead(t, f, "user-check", started.ID)
			beforeCommands, _ := json.Marshal(row.Commands)
			afterCommands, _ := json.Marshal(after.Commands)
			if after.Status != row.Status || after.ResourcesHeld == nil || *after.ResourcesHeld != *row.ResourcesHeld || !bytes.Equal(beforeCommands, afterCommands) {
				t.Errorf("earlier completion rewrote later durable outcome: run %s -> %s; second %s -> %s; held %v -> %v", row.Status, after.Status, row.Commands[1].Status, after.Commands[1].Status, *row.ResourcesHeld, after.ResourcesHeld)
			}
			// Verify private history agrees with the observed row and no later
			// allocation is lost or charged again by this publication-only work.
			if _, err = os.ReadFile(historyPath); err != nil {
				t.Fatal("private history unavailable", err)
			}
			remaining, err := a.execution.readFenceLedger()
			if err != nil {
				t.Fatal(err)
			}
			if mode == "cleanup-pending" {
				if len(remaining.Checks) != 1 || remaining.Checks[0] != ledger.Checks[1] {
					t.Fatal("earlier receipt changed unsettled later capacity")
				}
			} else if len(remaining.Checks) != 0 {
				t.Fatal("finished receipts did not clear", remaining.Checks)
			}
			afterCalls := [5]int{}
			afterCalls[0], afterCalls[1], afterCalls[2], afterCalls[3], afterCalls[4] = be.counts()
			if beforeCalls != afterCalls {
				t.Fatal("completion publication repeated backend work", beforeCalls, afterCalls)
			}
		})
	}
}
