package api

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Controlled Create/fence failures exercise the real HTTP review, durable
// receipt, observer and recovery paths. These do not establish Docker or
// installed-browser behavior. No original installed fixture is modified.
func TestProjectCheckHTTPUnconfirmedCreateObservationPreservesReview(t *testing.T) {
	for _, mode := range []string{"creating", "fencing"} {
		t.Run(mode, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t)
			base := newHTTPCheckExecutor(f.env.be)
			base.createErr = backend.ErrUnavailable
			wireHTTPCheckFixture(t, f, base, filepath.Join(t.TempDir(), "capacity"), "user-check", 12)
			be := &httpCheckFenceExecutor{httpCheckExecutor: base}
			f.env.api.deps.Backend = be
			var denied atomic.Bool
			f.env.api.deps.AuthorizeExecutionControl = func(_ context.Context, actor management.ExecutionActorRef) error {
				if denied.Load() || actor.UserID != "user-check" || actor.ProjectID != f.owner.ID {
					return management.ErrExecutionAuthority
				}
				return nil
			}
			started := checkHTTPStart(t, f, "user-check")
			row := checkHTTPWait(t, f, "user-check", started.ID, func(row product.ProjectCheckRun) bool {
				return row.Status == "unknown" && row.ReasonCode == "check_create_unconfirmed"
			})
			checkHTTPIdle(t, f, row)
			row = checkHTTPRead(t, f, "user-check", row.ID)
			if mode == "fencing" {
				prior := checkHTTPRecovery(t, f, "user-check", row.ID)
				be.beforeFence = func(backend.CheckFenceRequest) error { return backend.ErrUnavailable }
				checkHTTPRecover(t, f, "user-check", row.ID, prior, 503)
				// A real lease/recovery transition must still invalidate review.
				checkHTTPRecover(t, f, "user-check", row.ID, prior, 409)
				be.beforeFence = nil
				if err := f.env.api.ObserveChecksOnce(context.Background()); err != nil {
					t.Fatal("interrupted fence observation", err)
				}
				row = checkHTTPRead(t, f, "user-check", row.ID)
			}
			view := checkHTTPRecovery(t, f, "user-check", row.ID)
			wantState := "available"
			if mode == "fencing" {
				wantState = "pending"
			}
			if view.State != wantState || view.RecoveryDigest == nil || view.Method == nil || *view.Method != "fence-launch" {
				t.Fatal("exact retained create has no review", view)
			}
			before, err := f.env.api.execution.readFenceLedger()
			if err != nil || len(before.Checks) != 1 || before.Checks[0].State != mode {
				t.Fatal("missing retained create lease", err)
			}
			for tick := 0; tick < 4; tick++ {
				if tick == 2 {
					f.reopen(t)
				}
				if err = f.env.api.ObserveChecksOnce(context.Background()); err != nil {
					t.Fatal("unchanged create observation", err)
				}
				after := checkHTTPRead(t, f, "user-check", row.ID)
				if !reflect.DeepEqual(row, after) {
					t.Errorf("no-op create observation changed published receipt: tick=%d revision=%d->%d message=%q->%q", tick, row.Revision, after.Revision, row.Commands[0].Message, after.Commands[0].Message)
				}
				if current := checkHTTPRecovery(t, f, "user-check", row.ID); !reflect.DeepEqual(view, current) {
					t.Errorf("no-op create observation invalidated exact review: tick=%d revision=%d->%d", tick, view.Revision, current.Revision)
				}
				retained, e := f.env.api.execution.readFenceLedger()
				if e != nil || !reflect.DeepEqual(before, retained) {
					t.Fatal("no-op observation changed retained ownership", e)
				}
				demand := executionCapacityDemand{}
				lease := retained.Checks[0]
				if e = addCheckCapacity(httpCheckSnapshot(12), retained.Checks, "", "", &demand); e != nil || demand.PendingMemory != uint64(lease.Resources.MemoryMiB)<<20 || demand.AllMemory != demand.PendingMemory || demand.CPU != int64(lease.Resources.CPUMillis) || demand.PendingDiskBytes != lease.CreateDiskBytes {
					t.Fatal("unconfirmed creation lost conservative capacity", e, demand)
				}
			}
			// A stable review is no substitute for current editor authority.
			denied.Store(true)
			checkHTTPRecover(t, f, "user-check", row.ID, view, 403)
			denied.Store(false)
			be.fenceMu.Lock()
			fences := be.fences
			be.fenceMu.Unlock()
			_, creates, starts, logs, removes := base.counts()
			if creates != 1 || starts+logs+removes+fences != 0 {
				t.Fatal("observation or revoked recovery repeated backend work", creates, starts, logs, removes, fences)
			}
			if mode == "fencing" {
				// Meaningful output still changes revision and invalidates the old
				// review; observer no-ops must retain those exact new bytes.
				captured, index, out, e := f.env.api.deps.Product.ProjectCheckObservation(row.ProjectID, row.ID, row.Commands[0].AllocationID)
				if e != nil {
					t.Fatal(e)
				}
				out.Output, out.Truncated = "retained partial output\n", true
				if _, e = f.env.api.deps.Product.RecordProjectCheckObservation(captured, index, out); e != nil {
					t.Fatal("publish changed output", e)
				}
				if _, e = f.env.api.deps.Product.RecordProjectCheckObservation(captured, index, out); !errors.Is(e, product.ErrProjectRunConflict) {
					t.Fatal("changed output accepted a stale observation snapshot", e)
				}
				checkHTTPRecover(t, f, "user-check", row.ID, view, 409)
				row = checkHTTPRead(t, f, "user-check", row.ID)
				current := checkHTTPRecovery(t, f, "user-check", row.ID)
				if current.Revision <= view.Revision || current.RecoveryDigest == nil || *current.RecoveryDigest == *view.RecoveryDigest {
					t.Fatal("changed output retained stale review")
				}
				view = current
				if e = f.env.api.ObserveChecksOnce(context.Background()); e != nil {
					t.Fatal(e)
				}
				if after := checkHTTPRead(t, f, "user-check", row.ID); !reflect.DeepEqual(row, after) {
					t.Fatal("observation changed newly retained output")
				}
			}
			// In the creating case this is the exact review taken before any
			// observer tick or reopen; it must still authorize only this attempt.
			recovered := checkHTTPRecover(t, f, "user-check", row.ID, view, 200)
			if recovered.Status != "cancelled" || recovered.ResourcesHeld == nil || *recovered.ResourcesHeld || !recovered.Commands[0].CleanupConfirmed || recovered.Commands[0].ExitCode != nil || recovered.Commands[0].Output != row.Commands[0].Output || recovered.Commands[0].Truncated != row.Commands[0].Truncated {
				t.Fatal("exact recovery invented or lost a command result")
			}
			be.fenceMu.Lock()
			fences = be.fences
			be.fenceMu.Unlock()
			_, creates, starts, logs, removes = base.counts()
			if creates != 1 || starts+logs+removes != 0 || fences != 1 {
				t.Fatal("reviewed recovery repeated command work", creates, starts, logs, removes, fences)
			}
		})
	}
}

func TestProjectCheckHTTPInterruptedCreateObservationStillPublishesUnknown(t *testing.T) {
	for _, mode := range []string{"command-publication", "run-publication"} {
		t.Run(mode, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t)
			base := newHTTPCheckExecutor(f.env.be)
			base.createErr = backend.ErrUnavailable
			wireHTTPCheckFixture(t, f, base, filepath.Join(t.TempDir(), "capacity"), "user-check", 12)
			a := f.env.api
			a.deps.AuthorizeExecutionControl = func(context.Context, management.ExecutionActorRef) error { return nil }
			// Interrupt either the unknown command write or the following run
			// publication. The observer must reconcile both kinds of progress.
			a.deps.Product.SetProjectCheckRuntime(product.ProjectCheckRuntime{
				CaptureActor: a.captureCheckActor, AuthorizeControl: a.authorizeExecutionRetirement,
				Busy: a.projectCheckBusy, Recovery: a.projectCheckRecovery, Recover: a.recoverProjectCheck,
				Execute: func(ctx context.Context, req product.ProjectCheckCommandRequest, persist func(product.ProjectCheckCommandOutcome) error) (product.ProjectCheckCommandOutcome, error) {
					out, err := a.executeProjectCheck(ctx, req, func(out product.ProjectCheckCommandOutcome) error {
						if mode == "command-publication" && out.ReasonCode == "check_create_unconfirmed" {
							return errors.New("controlled unknown-create publication interruption")
						}
						return persist(out)
					})
					if err == nil {
						err = errors.New("controlled run publication interruption")
					}
					return out, err
				},
			})
			started := checkHTTPStart(t, f, "user-check")
			row := checkHTTPWait(t, f, "user-check", started.ID, func(row product.ProjectCheckRun) bool {
				return row.Status == "unknown" && row.ReasonCode == "check_execution_unconfirmed"
			})
			checkHTTPIdle(t, f, row)
			row = checkHTTPRead(t, f, "user-check", row.ID)
			wantStatus, wantStage := "running", "admission"
			if mode == "run-publication" {
				wantStatus, wantStage = "unknown", "create"
			}
			if row.Commands[0].Status != wantStatus || row.Commands[0].Stage != wantStage {
				t.Fatal("controlled failure did not retain interrupted progress")
			}
			prior := checkHTTPRecovery(t, f, "user-check", row.ID)
			f.reopen(t)
			if err := f.env.api.ObserveChecksOnce(context.Background()); err != nil {
				t.Fatal("interrupted create observation", err)
			}
			after := checkHTTPRead(t, f, "user-check", row.ID)
			command := after.Commands[0]
			if after.Revision <= row.Revision || after.Status != "unknown" || after.Stage != "create" || after.ReasonCode != "check_create_unconfirmed" || command.Status != "unknown" || command.Stage != "create" || command.ReasonCode != "check_create_unconfirmed" || command.ExitCode != nil || command.CleanupConfirmed || after.ResourcesHeld == nil || !*after.ResourcesHeld || after.Message != command.Message {
				t.Fatal("interrupted create was not published as retained unknown execution")
			}
			if mode == "run-publication" && !reflect.DeepEqual(row.Commands, after.Commands) {
				t.Fatal("run reconciliation rewrote already confirmed command diagnosis")
			}
			checkHTTPRecover(t, f, "user-check", row.ID, prior, 409)
			current := checkHTTPRecovery(t, f, "user-check", row.ID)
			if current.RecoveryDigest == nil || prior.RecoveryDigest == nil || *current.RecoveryDigest == *prior.RecoveryDigest {
				t.Fatal("meaningful interrupted-state change did not invalidate review")
			}
			if err := f.env.api.ObserveChecksOnce(context.Background()); err != nil {
				t.Fatal("repeated interrupted create observation", err)
			}
			if settled := checkHTTPRead(t, f, "user-check", row.ID); !reflect.DeepEqual(after, settled) {
				t.Fatal("normalized unknown create did not become stable")
			}
			_, creates, starts, logs, removes := base.counts()
			if creates != 1 || starts+logs+removes != 0 {
				t.Fatal("interrupted create observation replayed command work", creates, starts, logs, removes)
			}
		})
	}
}
