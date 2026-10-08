package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Controlled fence acknowledgments exercise coordinator durability. Native
// Docker ownership/fence behavior is covered by the adapter's distinct tests.
type httpCheckFenceExecutor struct {
	*httpCheckExecutor
	fenceMu               sync.Mutex
	fences, verifications int
	proof                 backend.CheckFenceProof
	verifyHook            func() error
	beforeFence           func(backend.CheckFenceRequest) error
}

func (b *httpCheckFenceExecutor) FenceCheckLaunch(ctx context.Context, req backend.CheckFenceRequest) (backend.CheckFenceProof, error) {
	if err := ctx.Err(); err != nil {
		return backend.CheckFenceProof{}, err
	}
	b.fenceMu.Lock()
	defer b.fenceMu.Unlock()
	if b.beforeFence != nil {
		if err := b.beforeFence(req); err != nil {
			return backend.CheckFenceProof{}, err
		}
	}
	b.fences++
	if b.proof != (backend.CheckFenceProof{}) {
		return backend.CheckFenceProof{}, errors.New("fence replay")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	b.proof = backend.CheckFenceProof{Identity: req.Identity, IntentID: req.IntentID, FenceRef: fixtureFenceHash("check-fence-" + req.IntentID), ConfigDigest: fixtureFenceHash("check-fence-config-" + req.IntentID), AcknowledgedAt: now, ObservedAt: now}
	return b.proof, nil
}
func (b *httpCheckFenceExecutor) VerifyCheckLaunchFence(ctx context.Context, req backend.CheckFenceRequest, proof backend.CheckFenceProof) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.fenceMu.Lock()
	defer b.fenceMu.Unlock()
	b.verifications++
	if proof != b.proof || req.Identity != proof.Identity || req.IntentID != proof.IntentID {
		return errors.New("foreign or changed fence")
	}
	if b.verifyHook != nil {
		return b.verifyHook()
	}
	return nil
}
func checkHTTPRecovery(t *testing.T, f *swarmSessionFixture, user, id string) product.ProjectCheckRecoveryView {
	t.Helper()
	w := checkHTTPDo(f, user, "GET", "/v1/product/projects/"+f.owner.ID+"/checks/"+id+"/recovery", "")
	var view product.ProjectCheckRecoveryView
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &view) != nil || view.CheckID != id || view.ProjectID != f.owner.ID {
		t.Fatal("exact recovery read", w.Code, w.Body.String())
	}
	return view
}
func checkHTTPRecover(t *testing.T, f *swarmSessionFixture, user, id string, view product.ProjectCheckRecoveryView, status int) product.ProjectCheckRun {
	t.Helper()
	if view.RecoveryDigest == nil {
		t.Fatal("no exact reviewed recovery digest", view)
	}
	body, _ := json.Marshal(product.ProjectCheckRecoveryRequest{ExpectedRevision: view.Revision, RecoveryDigest: *view.RecoveryDigest, AcknowledgeStop: true})
	w := checkHTTPDo(f, user, "POST", "/v1/product/projects/"+f.owner.ID+"/checks/"+id+"/recovery", string(body))
	if w.Code != status {
		t.Fatal("recovery result", w.Code, w.Body.String())
	}
	if status != 200 {
		return product.ProjectCheckRun{}
	}
	var row product.ProjectCheckRun
	if json.Unmarshal(w.Body.Bytes(), &row) != nil {
		t.Fatal("recovery response is not a check")
	}
	return row
}
func TestProjectCheckHTTPUnacknowledgedCreateRequiresReviewedFenceAndFreshEditor(t *testing.T) {
	for _, mode := range []string{"complete", "proof-retry", "revoke-after-proof"} {
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
			before, err := f.env.api.execution.readFenceLedger()
			if err != nil || len(before.Checks) != 1 || before.Checks[0].State != "creating" || before.Checks[0].Ref != "" {
				t.Fatal("lost Create acknowledgment was released", err)
			}
			if err = f.env.api.ObserveChecksOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			retained, err := f.env.api.execution.readFenceLedger()
			if err != nil || len(retained.Checks) != 1 || retained.Checks[0] != before.Checks[0] {
				t.Fatal("observation treated absence as a lost-ACK proof", err)
			}
			view := checkHTTPRecovery(t, f, "user-check", row.ID)
			if view.State != "available" || view.Method == nil || *view.Method != "fence-launch" {
				t.Fatal("wrong explicit recovery", view)
			}
			// Execution may now be revoked while the current editor can still stop the
			// exact prior attempt. No provider/harness is selected by this control path.
			f.env.api.deps.AuthorizeChecks = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
				return management.ExecutionAuthority{}, management.ErrExecutionAuthority
			}
			be.beforeFence = func(req backend.CheckFenceRequest) error {
				l, e := f.env.api.execution.readFenceLedger()
				if e != nil || len(l.CheckFences) != 1 || l.CheckFences[0].IntentID != req.IntentID || len(l.Checks) != 1 || l.Checks[0].State != "fencing" {
					return errors.New("missing durable slot before fence")
				}
				r, e := f.env.api.deps.Product.GetProjectCheckRun(f.owner.ID, row.ID)
				if e != nil || r.Stage != "recovery" || r.ResourcesHeld == nil || !*r.ResourcesHeld {
					return errors.New("missing public recovery receipt before fence")
				}
				return nil
			}
			switch mode {
			case "proof-retry":
				be.verifyHook = func() error { return backend.ErrUnavailable }
			case "revoke-after-proof":
				be.verifyHook = func() error { denied.Store(true); return nil }
			}
			if mode == "complete" {
				row = checkHTTPRecover(t, f, "user-check", row.ID, view, 200)
			} else {
				checkHTTPRecover(t, f, "user-check", row.ID, view, 503)
				l, e := f.env.api.execution.readFenceLedger()
				if e != nil || len(l.Checks) != 1 || l.Checks[0].State != "fencing" || len(l.CheckFences) != 1 || l.CheckFences[0].State != "proved" {
					t.Fatal("partial proof lost retained ownership", e)
				}
				f.reopen(t)
				denied.Store(false)
				be.fenceMu.Lock()
				be.verifyHook = nil
				be.fenceMu.Unlock()
				view = checkHTTPRecovery(t, f, "user-check", row.ID)
				if view.State != "pending" {
					t.Fatal("partial recovery not visible", view)
				}
				row = checkHTTPRecover(t, f, "user-check", row.ID, view, 200)
			}
			if row.Status != "cancelled" || row.ResourcesHeld == nil || *row.ResourcesHeld || !row.Commands[0].CleanupConfirmed || row.Commands[0].ExitCode != nil {
				t.Fatal("retirement invented a command outcome", row.Status)
			}
			l, e := f.env.api.execution.readFenceLedger()
			if e != nil || len(l.Checks) != 0 || len(l.CheckFences) != 1 || l.CheckFences[0].State != "retired" || l.CheckFences[0].Identity != before.Checks[0].Identity {
				t.Fatal("retirement lost exact permanent proof", e)
			}
			if err = f.env.api.ObserveChecksOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			_, creates, starts, logs, removes := base.counts()
			be.fenceMu.Lock()
			fences := be.fences
			be.fenceMu.Unlock()
			if creates != 1 || starts+logs+removes != 0 || fences != 1 {
				t.Fatal("recovery replayed a command/fence or generic cleanup", creates, starts, logs, removes, fences)
			}
		})
	}
}

// The adapter deliberately waits on the caller's deadline, so this is a
// context-propagation/fairness test, not a wall-clock performance claim.
type deadlineCheckExecutor struct {
	*httpCheckExecutor
	slowAllocation string
	slow           atomic.Bool
	peerRemoves    atomic.Int32
}

func (b *deadlineCheckExecutor) RemoveCheck(ctx context.Context, id backend.CheckIdentity, ref string, force bool) error {
	if b.slow.Load() && id.AllocationID == b.slowAllocation {
		<-ctx.Done()
		return ctx.Err()
	}
	if b.slow.Load() {
		b.peerRemoves.Add(1)
	}
	return b.httpCheckExecutor.RemoveCheck(ctx, id, ref, force)
}
func TestProjectCheckHTTPObservationBudgetPreservesPeerProgressAfterReopen(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	peer := f.anotherProject(t, "check-observer-peer")
	base := newHTTPCheckExecutor(f.env.be)
	base.removeErr = backend.ErrUnavailable
	wireHTTPCheckFixture(t, f, base, filepath.Join(t.TempDir(), "capacity"), "user-check", 12)
	f.env.api.deps.AuthorizeChecks = func(_ context.Context, actor management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		if actor.UserID != "user-check" || (actor.ProjectID != f.owner.ID && actor.ProjectID != peer.owner.ID) {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return checkHTTPAuthority("user-check"), nil
	}
	be := &deadlineCheckExecutor{httpCheckExecutor: base}
	f.env.api.deps.Backend = be
	first := checkHTTPStart(t, f, "user-check")
	first = checkHTTPWait(t, f, "user-check", first.ID, func(row product.ProjectCheckRun) bool {
		return row.Status == "unknown" && row.ReasonCode == "check_cleanup_unconfirmed"
	})
	checkHTTPIdle(t, f, first)
	second := checkHTTPStart(t, peer, "user-check")
	second = checkHTTPWait(t, peer, "user-check", second.ID, func(row product.ProjectCheckRun) bool {
		return row.Status == "unknown" && row.ReasonCode == "check_cleanup_unconfirmed"
	})
	checkHTTPIdle(t, peer, second)
	be.slowAllocation = first.Commands[0].AllocationID
	base.mu.Lock()
	base.removeErr = nil
	base.mu.Unlock()
	be.slow.Store(true)
	boundedTurn := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- f.env.api.ObserveChecksOnce(ctx) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("observer cleanup ignored its shared context budget")
		}
	}
	boundedTurn()
	if be.peerRemoves.Load() != 0 {
		t.Fatal("timed out turn unexpectedly continued backend work")
	}
	f.reopen(t) // Durable leases survive; the small pending-page cache is rebuilt.
	boundedTurn()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := f.env.api.ObserveChecksOnce(ctx); err != nil {
		t.Fatal("unattempted peer did not progress", err)
	}
	if be.peerRemoves.Load() != 1 {
		t.Fatal("slow first cleanup starved an unrelated project", be.peerRemoves.Load())
	}
	l, err := f.env.api.execution.readFenceLedger()
	if err != nil || len(l.Checks) != 1 || l.Checks[0].ID != first.Commands[0].AllocationID || l.Checks[0].State != "cleaning" {
		t.Fatal("slow attempt was falsely released or peer remained held", err)
	}
	latest := checkHTTPRead(t, peer, "user-check", second.ID)
	if latest.Status != "passed" || latest.ResourcesHeld == nil || *latest.ResourcesHeld {
		t.Fatal("peer output did not reach durable completion")
	}
	_, creates, starts, _, _ := base.counts()
	if creates != 2 || starts != 2 {
		t.Fatal("observation/reopen replayed checks", creates, starts)
	}
}

func TestProjectCheckHTTPFinishedReceiptSurvivesFinalPublicationBoundary(t *testing.T) {
	for _, mode := range []string{"controller-after-command", "completion-ack"} {
		t.Run(mode, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t)
			base := newHTTPCheckExecutor(f.env.be)
			wireHTTPCheckFixture(t, f, base, filepath.Join(t.TempDir(), "capacity"), "user-check", 12)
			a := f.env.api
			execute := a.executeProjectCheck
			if mode == "controller-after-command" {
				execute = func(ctx context.Context, req product.ProjectCheckCommandRequest, persist func(product.ProjectCheckCommandOutcome) error) (product.ProjectCheckCommandOutcome, error) {
					out, err := a.executeProjectCheck(ctx, req, persist)
					if err == nil {
						err = errors.New("controlled interruption after durable command outcome")
					}
					return out, err
				}
			}
			a.deps.Product.SetProjectCheckRuntime(product.ProjectCheckRuntime{CaptureActor: a.captureCheckActor, AuthorizeControl: a.authorizeExecutionRetirement, Execute: execute, Busy: a.projectCheckBusy, Recovery: a.projectCheckRecovery, Recover: a.recoverProjectCheck, AcknowledgeCompleted: func(context.Context, string, string) error {
				return errors.New("controlled completion acknowledgment interruption")
			}})
			started := checkHTTPStart(t, f, "user-check")
			row := checkHTTPWait(t, f, "user-check", started.ID, func(row product.ProjectCheckRun) bool {
				return (row.Status == "passed" || row.Status == "unknown") && row.ResourcesHeld != nil && !*row.ResourcesHeld && row.Commands[0].CleanupConfirmed
			})
			checkHTTPIdle(t, f, row)
			l, err := a.execution.readFenceLedger()
			if err != nil || len(l.Checks) != 1 || l.Checks[0].State != "finished" || l.Checks[0].ResultDigest == "" {
				t.Fatal("completion work receipt was lost before final acknowledgment", err)
			}
			if row.Commands[0].Output != base.output || row.Commands[0].ExitCode == nil || *row.Commands[0].ExitCode != 0 {
				t.Fatal("completion interruption lost actual outcome")
			}
			f.reopen(t)
			if err = f.env.api.ObserveChecksOnce(context.Background()); err != nil {
				t.Fatal("finished publication retry", err)
			}
			row = checkHTTPRead(t, f, "user-check", row.ID)
			if row.Status != "passed" || row.ResourcesHeld == nil || *row.ResourcesHeld || !row.Commands[0].CleanupConfirmed || row.Commands[0].Output != base.output {
				t.Fatal("final outcome did not survive reopen", row.Status)
			}
			l, err = f.env.api.execution.readFenceLedger()
			if err != nil || len(l.Checks) != 0 {
				t.Fatal("finished publication receipt did not clear", err)
			}
			_, creates, starts, _, removes := base.counts()
			if creates != 1 || starts != 1 || removes != 1 {
				t.Fatal("publication retry repeated backend effects", creates, starts, removes)
			}
		})
	}
}

// Non-Linux local controllers do not hold the Linux whole-state writer lease.
// An independent controller can therefore finish a reserved attempt while its
// original HTTP controller is still waiting for that attempt's lifecycle lock.
// Hold the recovery side of that lock explicitly to make the scheduling exact;
// settlement and publication use the same production methods as observation.
func TestProjectCheckHTTPRetiredReservationCannotBeRecreatedByDelayedController(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	base := newHTTPCheckExecutor(f.env.be)
	shared := filepath.Join(t.TempDir(), "capacity")
	wireHTTPCheckFixture(t, f, base, shared, "user-check", 12)
	if err := os.MkdirAll(shared, 0700); err != nil {
		t.Fatal(err)
	}
	peerProduct, err := product.New(f.options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(peerProduct.Close)
	peer := &API{deps: f.env.api.deps}
	peer.deps.Product = peerProduct
	peer.execution = newExecutionAdmission(peer)
	prepared := make(chan struct{})
	resume := make(chan struct{})
	base.prepareHook = func(ctx context.Context, _ backend.CheckSpec) error {
		close(prepared)
		select {
		case <-resume:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	started := checkHTTPStart(t, f, "user-check")
	select {
	case <-prepared:
	case <-time.After(time.Second):
		t.Fatal("launch did not reach image preparation")
	}
	unlock, err := peer.execution.lockChildLifecycle(context.Background(), started.Commands[0].AllocationID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	close(resume)
	var lease checkExecutionLease
	deadline := time.Now().Add(750 * time.Millisecond)
	for time.Now().Before(deadline) {
		ledger, readErr := peer.execution.readFenceLedger()
		if readErr == nil && len(ledger.Checks) == 1 {
			lease = ledger.Checks[0]
			break
		}
		time.Sleep(time.Millisecond)
	}
	if lease.State != "reserved" || lease.ID != started.Commands[0].AllocationID {
		t.Fatal("original controller did not publish its pending reservation")
	}
	row, index, out, err := peerProduct.ProjectCheckObservation(started.ProjectID, started.ID, lease.ID)
	if err != nil {
		t.Fatal(err)
	}
	out.Status, out.Stage, out.ReasonCode, out.Message = "blocked", "admission", "check_controller_interrupted", "The service restarted before this command started. No command will be replayed."
	out.ResourcesHeld = true
	persist := func(value product.ProjectCheckCommandOutcome) error {
		next, e := peerProduct.RecordProjectCheckObservation(row, index, value)
		if e == nil {
			row = next
		}
		return e
	}
	if _, err = peer.finishUnstartedCheck(context.Background(), lease.Identity, out, persist, false); err != nil {
		t.Fatal("independent reserved-attempt settlement", err)
	}
	ledger, err := peer.execution.readFenceLedger()
	if err != nil || len(ledger.Checks) != 0 {
		t.Fatal("settled reservation was not acknowledged", err)
	}
	historyPath := filepath.Join(f.options.StateDir, "project-checks", started.ProjectID+".json")
	settledHistory, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	checkHTTPIdle(t, f, started)
	_, creates, starts, _, _ := base.counts()
	if creates != 0 || starts != 0 {
		t.Errorf("delayed controller recreated retired reservation: Create=%d Start=%d", creates, starts)
	}
	afterHistory, err := os.ReadFile(historyPath)
	if err != nil || !bytes.Equal(afterHistory, settledHistory) {
		t.Error("delayed controller overwrote the settled public revision or command outcome", err)
	}
	ledger, err = peer.execution.readFenceLedger()
	if err != nil || len(ledger.Checks) != 0 {
		t.Fatal("retired check gained a new capacity claim", err)
	}
}

func TestProjectCheckReservedClaimRequiresExactPendingOwnership(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	base := newHTTPCheckExecutor(f.env.be)
	wireHTTPCheckFixture(t, f, base, filepath.Join(t.TempDir(), "capacity"), "user-check", 12)
	g := f.env.api.execution
	identity := backend.CheckIdentity{Protocol: backend.CheckExecutionProtocol, EngineID: httpCheckEngine, InstanceID: httpCheckInstance, AllocationID: "checkexec-claim-guard", CheckID: "check-claim-guard", ScopeDigest: g.scope, ProjectID: f.owner.ID, CommandDigest: fixtureFenceHash("check-command")}
	actor := management.ExecutionActorRef{UserID: "user-check", ProjectID: f.owner.ID}
	resources := backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	now := time.Now().UTC()
	original := checkExecutionLease{ID: identity.AllocationID, Scope: g.scope, Actor: actor, Identity: identity, Resources: resources, CreateDiskBytes: 1 << 30, State: "reserved", CreatedAt: now, UpdatedAt: now}
	for _, mode := range []string{"exact", "absent", "created", "actor", "memory", "cpu", "engine", "instance", "command", "project", "scope"} {
		t.Run(mode, func(t *testing.T) {
			lease := original
			switch mode {
			case "created":
				lease.State, lease.Ref = "created", fixtureFenceHash("acknowledged-container")
			case "actor":
				lease.Actor.UserID = "different-user"
			case "memory":
				lease.Resources.MemoryMiB *= 2
			case "cpu":
				lease.Resources.CPUMillis *= 2
			case "engine":
				lease.Identity.EngineID = "different-engine"
			case "instance":
				lease.Identity.InstanceID = "22222222-2222-4222-8222-222222222222"
			case "command":
				lease.Identity.CommandDigest = fixtureFenceHash("different-command")
			case "project":
				lease.Actor.ProjectID, lease.Identity.ProjectID = "different-project", "different-project"
			case "scope":
				lease.Scope, lease.Identity.ScopeDigest = fixtureFenceHash("different-scope"), fixtureFenceHash("different-scope")
			}
			if err := g.mutate(context.Background(), func(l *executionLeaseLedger) error {
				l.Checks = []checkExecutionLease{lease}
				if mode == "absent" {
					l.Checks = nil
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			unlock, err := g.lockChildLifecycle(context.Background(), identity.AllocationID)
			if err != nil {
				t.Fatal(err)
			}
			defer unlock()
			err = g.requireReservedCheck(identity, actor, resources)
			if mode == "exact" {
				if err != nil {
					t.Fatal("current reserved claim was rejected", err)
				}
			} else if !errors.Is(err, product.ErrProjectCheckSuperseded) {
				t.Fatal("changed reservation did not stop its stale controller", err)
			}
		})
	}
}
