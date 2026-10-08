package api

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// Exercise actual HTTP, durable command publication and exact lease retirement
// with controlled backend observations. Pending cleanup must remain visible
// before removal, then become confirmed without changing the command result.
// These are not Docker or installed-browser execution witnesses.
func TestProjectCheckHTTPCleanupConfirmationReplacesPendingMessage(t *testing.T) {
	for _, mode := range []string{"live-passed", "live-failed", "reopened-exited", "reopened-absent"} {
		t.Run(mode, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t)
			be := newHTTPCheckExecutor(f.env.be)
			exitCode, wantStatus := 0, "passed"
			if mode != "live-passed" {
				exitCode, wantStatus = 17, "failed"
			}
			be.exitCode = exitCode
			be.output = "exact retained output for " + mode + "\n"
			reopened := strings.HasPrefix(mode, "reopened-")
			if reopened {
				be.removeErr = backend.ErrUnavailable
			}
			wireHTTPCheckFixture(t, f, be, filepath.Join(t.TempDir(), "capacity"), "user-check", 12)
			var pendingObserved atomic.Bool
			be.removeHook = func(identity backend.CheckIdentity, _ string) error {
				row, err := f.env.api.deps.Product.GetProjectCheckRun(identity.ProjectID, identity.CheckID)
				if err != nil || len(row.Commands) != 1 || row.ResourcesHeld == nil || !*row.ResourcesHeld || row.Commands[0].CleanupConfirmed || !strings.Contains(row.Commands[0].Message, "cleanup is pending") {
					return errors.New("pending exact cleanup was not published before removal")
				}
				pendingObserved.Store(true)
				return nil
			}
			started := checkHTTPStart(t, f, "user-check")
			row := checkHTTPWait(t, f, "user-check", started.ID, func(row product.ProjectCheckRun) bool {
				if reopened {
					return row.Status == "unknown" && row.ReasonCode == "check_cleanup_unconfirmed"
				}
				return row.Status == wantStatus && row.ResourcesHeld != nil && !*row.ResourcesHeld
			})
			checkHTTPIdle(t, f, row)
			if !pendingObserved.Load() {
				t.Fatal("no pre-removal pending-cleanup publication was observed")
			}
			if reopened {
				if row.ResourcesHeld == nil || !*row.ResourcesHeld || row.Commands[0].CleanupConfirmed || !strings.Contains(row.Commands[0].Message, "cleanup is unconfirmed") {
					t.Fatal("failed removal lost its uncertainty or held capacity")
				}
				ledger, err := f.env.api.execution.readFenceLedger()
				if err != nil || len(ledger.Checks) != 1 || ledger.Checks[0].State != "cleaning" {
					t.Fatal("missing retained exact cleanup claim", err)
				}
				f.reopen(t)
				be.mu.Lock()
				be.removeErr = nil
				if mode == "reopened-absent" {
					// The exact acknowledged object disappeared after the original
					// removal response failed; retained exit/output remains valid.
					be.states[ledger.Checks[0].Ref] = "absent"
				}
				be.mu.Unlock()
				if err = f.env.api.ObserveChecksOnce(context.Background()); err != nil {
					t.Fatal("exact reopened cleanup observation", err)
				}
				row = checkHTTPRead(t, f, "user-check", started.ID)
			}
			checkHTTPCompletionAcknowledged(t, f, row)
			command := row.Commands[0]
			if row.Status != wantStatus || row.ResourcesHeld == nil || *row.ResourcesHeld || command.Status != wantStatus || command.ExitCode == nil || *command.ExitCode != exitCode || command.Output != be.output || !command.CleanupConfirmed {
				t.Fatal("confirmed cleanup changed the exact command outcome or retained capacity")
			}
			if command.ReasonCode != "" || command.Message != "The command exit and output were recorded; exact container cleanup is confirmed." {
				t.Fatalf("confirmed cleanup retained contradictory diagnosis: code=%q message=%q", command.ReasonCode, command.Message)
			}
			before := row
			if err := f.env.api.ObserveChecksOnce(context.Background()); err != nil {
				t.Fatal("settled check observation", err)
			}
			if after := checkHTTPRead(t, f, "user-check", row.ID); !reflect.DeepEqual(before, after) {
				t.Fatal("repeated settled observation changed the published outcome")
			}
			_, creates, starts, _, removes := be.counts()
			wantRemoves := 1
			if mode == "reopened-exited" {
				wantRemoves = 2
			}
			if creates != 1 || starts != 1 || removes != wantRemoves {
				t.Fatal("cleanup observation replayed command work", creates, starts, removes)
			}
		})
	}
}

// A historical finished receipt is already bound to its exact message bytes.
// New display copy must not invalidate or silently migrate this journal during
// observation, nor interfere with its idempotent completion acknowledgment.
func TestProjectCheckHTTPFinishedLegacyCleanupMessagePreservesDigest(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	be := newHTTPCheckExecutor(f.env.be)
	wireHTTPCheckFixture(t, f, be, filepath.Join(t.TempDir(), "capacity"), "user-check", 12)
	a := f.env.api
	a.deps.Product.SetProjectCheckRuntime(product.ProjectCheckRuntime{
		CaptureActor: a.captureCheckActor, AuthorizeControl: a.authorizeExecutionRetirement,
		Execute: a.executeProjectCheck, Busy: a.projectCheckBusy,
		Recovery: a.projectCheckRecovery, Recover: a.recoverProjectCheck,
		AcknowledgeCompleted: func(context.Context, string, string) error {
			return errors.New("controlled interruption before completion acknowledgment")
		},
	})
	started := checkHTTPStart(t, f, "user-check")
	row := checkHTTPWait(t, f, "user-check", started.ID, func(row product.ProjectCheckRun) bool {
		return row.Status == "passed" && row.ResourcesHeld != nil && !*row.ResourcesHeld
	})
	checkHTTPIdle(t, f, row)
	_, index, out, err := a.deps.Product.ProjectCheckObservation(row.ProjectID, row.ID, row.Commands[0].AllocationID)
	if err != nil {
		t.Fatal(err)
	}
	// Construct one valid older-version finished receipt in this disposable
	// fixture; production does not rewrite the original installed evidence.
	out.Message = "The command exit and output were recorded; exact container cleanup is pending."
	legacy, err := a.deps.Product.RecordProjectCheckObservation(row, index, out)
	if err != nil {
		t.Fatal("legacy published outcome", err)
	}
	if err = a.execution.mutate(context.Background(), func(ledger *executionLeaseLedger) error {
		if len(ledger.Checks) != 1 || ledger.Checks[0].ID != out.AllocationID || ledger.Checks[0].State != "finished" {
			return errors.New("legacy fixture has no exact finished receipt")
		}
		ledger.Checks[0].ResultDigest = checkOutcomeDigest(out)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// Compare the same public projection on both sides; HTTP adds evidence
	// annotations that are deliberately absent from the private store result.
	legacy = checkHTTPRead(t, f, "user-check", row.ID)
	beforeCalls := [5]int{}
	beforeCalls[0], beforeCalls[1], beforeCalls[2], beforeCalls[3], beforeCalls[4] = be.counts()
	f.reopen(t)
	for i := 0; i < 2; i++ {
		if err = f.env.api.ObserveChecksOnce(context.Background()); err != nil {
			t.Fatal("legacy finished observation", err)
		}
		if after := checkHTTPRead(t, f, "user-check", row.ID); !reflect.DeepEqual(legacy, after) {
			t.Fatal("finished observation rewrote digest-bound historical copy")
		}
	}
	checkHTTPCompletionAcknowledged(t, f, legacy)
	afterCalls := [5]int{}
	afterCalls[0], afterCalls[1], afterCalls[2], afterCalls[3], afterCalls[4] = be.counts()
	if beforeCalls != afterCalls {
		t.Fatal("historical publication replayed backend work", beforeCalls, afterCalls)
	}
}

// The same cleanup boundary also serves outcomes without a recorded exit.
// Retain their cancellation, admission and missing-output meaning after actual
// controlled absence; a settled object alone cannot establish passing evidence.
type cleanupMessageCheckExecutor struct {
	*httpCheckExecutor
	mode string
}

func (b *cleanupMessageCheckExecutor) StartCheck(ctx context.Context, id backend.CheckIdentity, ref string) error {
	if err := b.httpCheckExecutor.StartCheck(ctx, id, ref); err != nil {
		return err
	}
	if b.mode == "cancelled" {
		b.mu.Lock()
		b.states[ref] = "running"
		b.mu.Unlock()
	}
	return nil
}

func (b *cleanupMessageCheckExecutor) CheckLogs(ctx context.Context, id backend.CheckIdentity, ref string, limit int) (backend.CheckOutput, error) {
	if b.mode == "missing-output" {
		return backend.CheckOutput{}, backend.ErrUnavailable
	}
	return b.httpCheckExecutor.CheckLogs(ctx, id, ref, limit)
}

func TestProjectCheckHTTPCleanupConfirmationPreservesOtherDiagnoses(t *testing.T) {
	for _, mode := range []string{"cancelled", "post-create-admission", "missing-output"} {
		t.Run(mode, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t)
			base := newHTTPCheckExecutor(f.env.be)
			wireHTTPCheckFixture(t, f, base, filepath.Join(t.TempDir(), "capacity"), "user-check", 12)
			f.env.api.deps.Backend = &cleanupMessageCheckExecutor{httpCheckExecutor: base, mode: mode}
			f.env.api.deps.AuthorizeExecutionControl = func(_ context.Context, actor management.ExecutionActorRef) error {
				if actor.UserID != "user-check" || actor.ProjectID != f.owner.ID {
					return management.ErrExecutionAuthority
				}
				return nil
			}
			if mode == "post-create-admission" {
				f.env.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
					_, creates, _, _, _ := base.counts()
					if creates > 0 {
						return httpCheckSnapshot(0)
					}
					return httpCheckSnapshot(12)
				}
			}
			started := checkHTTPStart(t, f, "user-check")
			wantStatus, wantCode := "cancelled", "check_cancelled"
			wantStarts, wantRemoves := 1, 1
			switch mode {
			case "cancelled":
				checkHTTPWait(t, f, "user-check", started.ID, func(row product.ProjectCheckRun) bool {
					_, _, starts, _, _ := base.counts()
					return row.Status == "running" && starts == 1
				})
				w := checkHTTPDo(f, "user-check", "POST", "/v1/product/projects/"+f.owner.ID+"/checks/"+started.ID+"/cancel", "{}")
				if w.Code != 202 {
					t.Fatal("current editor cancellation", w.Code, w.Body.String())
				}
			case "post-create-admission":
				wantStatus, wantCode, wantStarts = "blocked", "memory_pressure", 0
			case "missing-output":
				wantStatus, wantCode, wantRemoves = "unknown", "check_output_unavailable", 0
				pending := checkHTTPWait(t, f, "user-check", started.ID, func(row product.ProjectCheckRun) bool {
					return row.Status == "unknown" && row.ReasonCode == "check_output_unavailable"
				})
				checkHTTPIdle(t, f, pending)
				ledger, err := f.env.api.execution.readFenceLedger()
				if err != nil || len(ledger.Checks) != 1 || pending.ResourcesHeld == nil || !*pending.ResourcesHeld {
					t.Fatal("missing-output allocation was prematurely released", err)
				}
				f.reopen(t)
				base.mu.Lock()
				base.states[ledger.Checks[0].Ref] = "absent"
				base.mu.Unlock()
				if err = f.env.api.ObserveChecksOnce(context.Background()); err != nil {
					t.Fatal("absent object with missing output", err)
				}
			}
			row := checkHTTPWait(t, f, "user-check", started.ID, func(row product.ProjectCheckRun) bool {
				return row.Status == wantStatus && row.ResourcesHeld != nil && !*row.ResourcesHeld
			})
			checkHTTPIdle(t, f, row)
			checkHTTPCompletionAcknowledged(t, f, row)
			command := row.Commands[0]
			if command.Status != wantStatus || command.ExitCode != nil || command.ReasonCode != wantCode || !command.CleanupConfirmed || strings.Contains(command.Message, "command exit and output were recorded") {
				t.Fatal("confirmed absence erased uncertainty or manufactured terminal evidence", command)
			}
			_, creates, starts, _, removes := base.counts()
			if creates != 1 || starts != wantStarts || removes != wantRemoves {
				t.Fatal("diagnostic cleanup changed backend effects", creates, starts, removes)
			}
		})
	}
}
