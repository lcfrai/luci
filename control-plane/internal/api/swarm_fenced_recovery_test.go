package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func assertFencedLedger(t *testing.T, f *swarmSessionFixture, leases int, state string) executionLeaseLedger {
	t.Helper()
	ledger, err := f.env.api.execution.readFenceLedger()
	if err != nil || len(ledger.Leases) != leases || len(ledger.Fences) != 1 || ledger.Fences[0].State != state {
		t.Fatal("unexpected durable recovery state", ledger, err)
	}
	return ledger
}

func TestSwarmFencedHTTPRetainsOutputContextAndRetiresOnlyExactLaunch(t *testing.T) {
	f, probe, leaseID := fencedTaskFixture(t)
	op := f.ops[0]
	a := f.env.api
	taskPath, err := a.deps.Worktrees.Path(context.Background(), op.WorktreeID)
	if err != nil {
		t.Fatal(err)
	}
	retained := []byte("uncommitted output survives stop-and-retain\n")
	if err := os.WriteFile(filepath.Join(taskPath, "retained.txt"), retained, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := f.store.Get(op.ChildID)
	if err != nil || before.ReviewedContext == nil {
		t.Fatal("missing captured context", err)
	}
	var contextPath string
	f.env.be.mu.Lock()
	for _, mount := range f.env.be.specs[0].Mounts {
		if mount.ContainerPath == "/home/dev/.luci-reviewed-context" && mount.ReadOnly {
			contextPath = mount.HostPath
		}
	}
	f.env.be.mu.Unlock()
	captured, err := os.ReadFile(filepath.Join(contextPath, "LUCI.md"))
	if err != nil || len(captured) == 0 {
		t.Fatal("missing task bundle", err)
	}
	// A separate project's reserved resources must not be retired by this proof.
	policy, _ := a.deps.Product.SwarmSchedulerPolicy()
	otherProjectID := "controlled-unrelated-project"
	actor := management.ExecutionActorRef{Local: true, ProjectID: otherProjectID}
	if decision, err := a.execution.reserve(context.Background(), "unrelated-reservation", otherProjectID, f.run.Input.Route.Connection.ID, policy.MaxActive, actor, management.ExecutionAuthority{}, policy); err != nil || decision.State != "admitted" {
		t.Fatal(decision, err)
	}
	response := fencedRecover(t, f, http.StatusOK)
	ledger := assertFencedLedger(t, f, 1, "retired")
	if ledger.Leases[0].ID != "unrelated-reservation" || len(ledger.Retirements) != 1 || ledger.Retirements[0].LeaseID != leaseID || ledger.Retirements[0].ChildID != op.ChildID {
		t.Fatal("foreign reservation changed", ledger)
	}
	if len(probe.requests) != 1 || probe.verifyCalls != 1 || controlledAllocationCount(f.env.be) != 1 {
		t.Fatal("recovery launched replacement or missed verification")
	}
	current, err := f.store.Get(op.ChildID)
	if err != nil || current.Status != registry.StatusExited || current.Endpoint != "" || current.AttachToken != "" || !reflect.DeepEqual(current.ReviewedContext, before.ReviewedContext) {
		t.Fatal("retirement changed captured context or left live endpoint", err)
	}
	if data, err := os.ReadFile(filepath.Join(taskPath, "retained.txt")); err != nil || !bytes.Equal(data, retained) {
		t.Fatal("retained work changed", err)
	}
	if data, err := os.ReadFile(filepath.Join(contextPath, "LUCI.md")); err != nil || !bytes.Equal(data, captured) {
		t.Fatal("captured task context changed", err)
	}
	for _, private := range []string{"launchRecovery", "launchRecoveryProof", "absenceRetirement", probe.identity.EngineID, ledger.Fences[0].Intent.ScopeDigest, ledger.Fences[0].Proof.FenceProof.FenceRef} {
		if strings.Contains(response.Body.String(), private) {
			t.Fatal("public recovery leaks private proof field", private)
		}
	}
	var public struct {
		Run product.ProjectRun `json:"run"`
	}
	if json.Unmarshal(response.Body.Bytes(), &public) != nil || public.Run.Operations[0].Recovery == nil || public.Run.Operations[0].Recovery.State != "finalized" {
		t.Fatal("public finalized summary missing")
	}
	if err := a.execution.markBackendSettled(context.Background(), leaseID, op.ChildID, op.ChildID); err == nil {
		t.Fatal("late backend acknowledgement revived retired launch")
	}
	if err := a.execution.markBackendStarting(context.Background(), leaseID, op.ChildID, op.ChildID); err == nil {
		t.Fatal("late allocator crossed permanent retirement")
	}
	f.reopen(t)
	fencedRecover(t, f, http.StatusOK)
	request, _ := json.Marshal(f.requests[0])
	replay := f.env.do(t, http.MethodPost, "/v1/sessions", string(request))
	if replay.Code < 400 || controlledAllocationCount(f.env.be) != 1 || len(probe.requests) != 1 {
		t.Fatal("old task replay allocated new work", replay.Code, replay.Body.String())
	}
}

func TestSwarmFencedHTTPCapabilityReadDoesNotProbeOrMutate(t *testing.T) {
	f, probe, _ := fencedTaskFixture(t)
	before, err := os.ReadFile(f.env.api.execution.path)
	if err != nil {
		t.Fatal(err)
	}
	calls := probe.identityCalls
	w := f.env.do(t, "GET", f.base()+"/"+f.run.ID+"/swarm/operations/"+f.ops[0].ID+"/recovery", "")
	var view struct {
		Availability product.ProjectSwarmRecoveryAvailability `json:"availability"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &view) != nil || !view.Availability.Supported {
		t.Fatal("capability read", w.Code, w.Body.String())
	}
	after, err := os.ReadFile(f.env.api.execution.path)
	if err != nil || !bytes.Equal(before, after) || probe.identityCalls != calls || len(probe.requests) != 0 || probe.verifyCalls != 0 {
		t.Fatal("read-only availability mutated state or probed backend", err)
	}
}

func TestSwarmFencedHTTPPrivateIntentSurvivesUnpublishedProductState(t *testing.T) {
	f, probe, _ := fencedTaskFixture(t)
	req := fencedRecoveryRequest(t, f)
	intent, release, err := f.env.api.acquireSwarmFenceRecovery(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	release() // Process state discarded after private slot publication, before product CAS.
	ledger := assertFencedLedger(t, f, 1, "intent")
	if f.run.Operations[0].LaunchRecovery != nil || ledger.Fences[0].Intent != intent || len(probe.requests) != 0 {
		t.Fatal("fixture did not stop before product publication")
	}
	// A legitimate observer changes the product revision in that interruption.
	store, err := product.NewProjectRunStore(f.options.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	f.run, err = store.ObserveOperation(f.owner.ID, f.owner.Path, f.run.ID, f.run.Revision, f.ops[0].ID, f.ops[0].ChildID, "failed", "controlled failed launch observation")
	if err != nil {
		t.Fatal(err)
	}
	f.reopen(t)
	fencedRecover(t, f, 200)
	ledger = assertFencedLedger(t, f, 0, "retired")
	if ledger.Fences[0].Intent != intent || len(probe.requests) != 1 || f.run.Operations[0].LaunchRecovery == nil || *f.run.Operations[0].LaunchRecovery != intent {
		t.Fatal("retry replaced interrupted private intent")
	}
}

func TestSwarmFencedHTTPPrivateProofReopenVerifiesWithoutSecondFence(t *testing.T) {
	f, probe, _ := fencedTaskFixture(t)
	a := f.env.api
	a.deps.Product.SetProjectSwarmFenceRecovery(a.projectSwarmFenceRecoveryAvailability, a.authorizeSwarmFenceRecovery, a.acquireSwarmFenceRecovery, func(ctx context.Context, req product.ProjectSwarmFenceRecoveryRequest, intent product.ProjectSwarmRecoveryIntent) (product.ProjectSwarmRecoveryProof, error) {
		if _, err := a.proveSwarmFencedLaunch(ctx, req, intent); err != nil {
			return product.ProjectSwarmRecoveryProof{}, err
		}
		return product.ProjectSwarmRecoveryProof{}, errors.New("controlled interruption after private proof, before product proof publication")
	}, a.finalizeSwarmFencedLaunch)
	fencedRecover(t, f, 409)
	ledger := assertFencedLedger(t, f, 1, "proved")
	saved := ledger.Fences[0].Proof.FenceProof
	if f.run.Operations[0].LaunchRecovery == nil || f.run.Operations[0].LaunchRecoveryProof != nil || f.run.Operations[0].AbsenceRetirement != nil || len(probe.requests) != 1 {
		t.Fatal("incorrect interrupted publication boundary")
	}
	time.Sleep(5100 * time.Millisecond) // The returned proof must genuinely outlive the five-second freshness window.
	f.reopen(t)
	fencedRecover(t, f, 200)
	ledger = assertFencedLedger(t, f, 0, "retired")
	now := ledger.Fences[0].Proof.FenceProof
	if len(probe.requests) != 1 || probe.verifyCalls != 2 || now.FenceRef != saved.FenceRef || now.ConfigDigest != saved.ConfigDigest || now.AcknowledgedAt != saved.AcknowledgedAt || now.ObservedAt <= saved.ObservedAt {
		t.Fatal("retry manufactured a second fence acknowledgement")
	}
}

func TestSwarmFencedHTTPRefusesLegacyWrongEngineAndRevokedEditor(t *testing.T) {
	for _, mode := range []string{"legacy", "wrong engine", "revoked before action", "revoked after proof verification"} {
		t.Run(mode, func(t *testing.T) {
			f, probe, _ := fencedTaskFixture(t)
			a := f.env.api
			var revoked atomic.Bool
			switch mode {
			case "legacy":
				if err := a.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
					l.Leases[0].FenceProtocol = 0
					l.Leases[0].FenceEngineID = ""
					l.Leases[0].FenceInstanceID = ""
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			case "wrong engine":
				probe.identity.EngineID = "different-engine"
			case "revoked before action":
				revoked.Store(true)
			case "revoked after proof verification":
				probe.beforeVerify = func(backend.LaunchFenceRequest) error { revoked.Store(true); return nil }
			}
			a.deps.AuthorizeExecutionControl = func(context.Context, management.ExecutionActorRef) error {
				if revoked.Load() {
					return management.ErrExecutionAuthority
				}
				return nil
			}
			fencedRecover(t, f, 409)
			ledger, err := a.execution.readFenceLedger()
			if err != nil || len(ledger.Leases) != 1 || len(ledger.Retirements) != 0 || controlledAllocationCount(f.env.be) != 1 {
				t.Fatal("refused recovery released capacity or replaced worker", err)
			}
			if mode == "revoked after proof verification" {
				op := f.run.Operations[0]
				if len(probe.requests) != 1 || op.AbsenceRetirement == nil || op.AbsenceRetirement.FinalizedAt != "" {
					t.Fatal("revocation lost durable proof or falsely finalized")
				}
				revoked.Store(false)
				probe.beforeVerify = nil
				f.reopen(t)
				fencedRecover(t, f, 200)
				assertFencedLedger(t, f, 0, "retired")
				if len(probe.requests) != 1 {
					t.Fatal("retry fenced twice")
				}
			} else if len(probe.requests) != 0 || f.run.Operations[0].LaunchRecovery != nil {
				t.Fatal("refusal caused recovery effects")
			}
		})
	}
}

func TestSwarmFencedHTTPPendingPrivateLaunchCannotBeReplaced(t *testing.T) {
	for _, mode := range []string{"backend-dispatching", "private-intent", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			f, probe, _ := fencedTaskFixture(t)
			store, err := product.NewProjectRunStore(f.options.StateDir)
			if err != nil {
				t.Fatal(err)
			}
			f.run, err = store.ObserveOperation(f.owner.ID, f.owner.Path, f.run.ID, f.run.Revision, f.ops[0].ID, f.ops[0].ChildID, "failed", "controlled failed launch observation")
			if err != nil {
				t.Fatal(err)
			}
			if mode == "private-intent" {
				_, release, err := f.env.api.acquireSwarmFenceRecovery(context.Background(), fencedRecoveryRequest(t, f))
				if err != nil {
					t.Fatal(err)
				}
				release()
			}
			if mode == "legacy" {
				if err := f.env.api.execution.mutate(context.Background(), func(l *executionLeaseLedger) error {
					l.Leases[0].FenceProtocol = 0
					l.Leases[0].FenceEngineID = ""
					l.Leases[0].FenceInstanceID = ""
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(f.env.api.execution.path)
			if err != nil {
				t.Fatal(err)
			}
			revision := f.run.Revision
			for _, suffix := range []string{"abandon", "reserve", "queue"} {
				request := map[string]any{"expectedRevision": revision}
				if suffix != "abandon" {
					request["idempotencyKey"] = "must-not-replace-uncertain-launch"
				}
				if suffix == "queue" {
					request["priority"] = "normal"
				}
				body, _ := json.Marshal(request)
				w := f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/tasks/api/"+suffix, string(body))
				if w.Code != 409 {
					t.Fatal("pending launch identity was replaceable", suffix, w.Code, w.Body.String())
				}
			}
			current, err := f.env.api.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
			after, e := os.ReadFile(f.env.api.execution.path)
			if err != nil || e != nil || current.Revision != revision || current.Swarm.Tasks[0].OperationID != f.ops[0].ID || !bytes.Equal(before, after) || len(probe.requests) != 0 {
				t.Fatal("refused ordinary action changed recovery identity", err, e)
			}
			if mode != "legacy" {
				fencedRecover(t, f, 200)
				assertFencedLedger(t, f, 0, "retired")
			}
		})
	}
}

func TestSwarmFencedHTTPQueuedUncertainLaunchRetiresQueueAfterProof(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	a := f.env.api
	a.deps.ReadyTimeout = 5 * time.Second
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	probe := &controlledLaunchFencer{Backend: f.env.be, identity: backend.LaunchFenceIdentity{Protocol: 1, EngineID: "controlled-queue-engine", InstanceID: "11111111-1111-4111-8111-111111111111"}, proofs: map[string]backend.LaunchFenceProof{}}
	a.deps.Backend = probe
	f.env.be.createErr, f.env.be.destroyErr = backend.ErrUnavailable, backend.ErrUnavailable
	data, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "idempotencyKey": "controlled-queued-uncertain", "taskIds": []string{"api"}, "priority": "normal"})
	w := f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/queue", string(data))
	var queued struct {
		Items []product.SwarmQueueItem `json:"items"`
	}
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &queued) != nil || len(queued.Items) != 1 {
		t.Fatal("queue consent", w.Code, w.Body.String())
	}
	if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Enqueue also wakes the real background scheduler. Returning from the
	// explicit turn does not join a subsequent observation turn, which may
	// legitimately hold dispatch exclusion and make recovery return busy.
	// Model a quiescent controller explicitly: disable future runtime work,
	// then acquire the scheduler's serialized turn to drain any prior callback.
	// The actual ambiguous Create has completed; no HTTP retries or sleeps
	// manufacture recovery eligibility while an allocator is still running.
	a.deps.Product.SetSwarmSchedulerRuntime(product.SwarmSchedulerRuntime{})
	if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	var err error
	f.run, err = a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil || len(f.run.Operations) != 1 || controlledAllocationCount(f.env.be) != 1 {
		t.Fatal("queued dispatcher did not reach controlled ambiguous Create", err)
	}
	f.ops = f.run.Operations
	ledger, err := a.execution.readFenceLedger()
	if err != nil || len(ledger.Leases) != 1 || ledger.Leases[0].ID != queued.Items[0].ID || ledger.Leases[0].State != "backend-dispatching" {
		t.Fatal("queue lease not retained", ledger, err)
	}
	fencedRecover(t, f, 200)
	ledger = assertFencedLedger(t, f, 0, "retired")
	if ledger.Retirements[0].QueueID != queued.Items[0].ID || ledger.Fences[0].QueueID != queued.Items[0].ID {
		t.Fatal("wrong queue retired")
	}
	w = f.env.do(t, "GET", "/v1/product/projects/"+f.owner.ID+"/swarm-queue", "")
	var result struct {
		Items []product.SwarmQueueItem `json:"items"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || len(result.Items) != 1 || result.Items[0].State != "cancelled" || (result.Items[0].MemoryMiB != 0 || result.Items[0].CPUMillis != 0) {
		t.Fatal("queue capacity was not finalized after exact proof", w.Code, w.Body.String())
	}
	// Resume the real admission/dispatch/release callbacks before asserting
	// that a completed recovery cannot replay its retired queued attempt.
	a.deps.Product.SetSwarmSchedulerRuntime(product.SwarmSchedulerRuntime{CaptureActor: a.captureQueuedActor, Admit: a.admitQueuedSwarm, Dispatch: a.dispatchQueuedSwarm, Release: a.execution.release, Resources: a.queuedResourceView})
	if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if controlledAllocationCount(f.env.be) != 1 {
		t.Fatal("retired queue relaunched")
	}
}

func TestSwarmFencedHTTPSettledFailedAttemptStillAllowsReviewedRetry(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t, "api")
	a := f.env.api
	a.deps.ReadyTimeout = 5 * time.Second
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	body, _ := json.Marshal(f.requests[0])
	w := f.env.do(t, "POST", "/v1/sessions", string(body))
	if w.Code != 201 {
		t.Fatal("successful controlled backend", w.Code, w.Body.String())
	}
	sessionsAwaitWatch(t, done)
	ledger, err := a.execution.readFenceLedger()
	if err != nil || len(ledger.Leases) != 1 || ledger.Leases[0].State != "backend-settled" {
		t.Fatal("successful full backend acknowledgment not durable", err)
	}
	op := f.ops[0]
	if _, err := f.store.Update(op.ChildID, func(row *registry.Session) error {
		row.Status = registry.StatusExited
		row.ExitReason = "controlled completed backend, later task failed"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.run, err = a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil {
		t.Fatal(err)
	}
	store, err := product.NewProjectRunStore(f.options.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	f.run, err = store.ObserveOperation(f.owner.ID, f.owner.Path, f.run.ID, f.run.Revision, op.ID, op.ChildID, "failed", "controlled task failure after acknowledged launch")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = json.Marshal(map[string]any{"expectedRevision": f.run.Revision})
	f.readRun(t, f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/tasks/api/abandon", string(body)), 200)
	body, _ = json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "idempotencyKey": "explicit-review-retry-after-settled-failure"})
	f.readRun(t, f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/tasks/api/reserve", string(body)), 201)
	if len(f.run.Operations) != 2 || f.run.Swarm.Tasks[0].OperationID == op.ID || controlledAllocationCount(f.env.be) != 1 {
		t.Fatal("settled failure retry was blocked or automatically launched")
	}
}

func TestSwarmFencedHTTPUnclaimedAbandonReopenAllowsFreshConsent(t *testing.T) {
	for _, next := range []string{"reserve", "queue"} {
		t.Run(next, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t, "api")
			op := f.ops[0]
			if _, err := os.Stat(f.env.api.execution.path); !os.IsNotExist(err) {
				t.Fatal("fixture unexpectedly has execution ledger", err)
			}
			body, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision})
			w := f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/tasks/api/abandon", string(body))
			f.readRun(t, w, 200)
			if strings.Contains(w.Body.String(), "unclaimedRetirement") {
				t.Fatal("public run leaked private unclaimed proof")
			}
			private, err := f.env.api.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
			if err != nil || !private.Operations[0].UnclaimedRetirement || private.Operations[0].Status != "cancelled" || private.Operations[0].Allocation != "ready" {
				t.Fatal("never-claimed cancellation lost private proof or retained checkout", err)
			}
			f.reopen(t)
			request := map[string]any{"expectedRevision": f.run.Revision, "idempotencyKey": "fresh-consent-after-unclaimed-cancellation"}
			if next == "queue" {
				request["priority"] = "normal"
			}
			body, _ = json.Marshal(request)
			w = f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/tasks/api/"+next, string(body))
			if w.Code != 201 {
				t.Fatal("fresh consent incorrectly requires absent execution ledger", w.Code, w.Body.String())
			}
			if next == "reserve" {
				f.readRun(t, w, 201)
				if len(f.run.Operations) != 2 || f.run.Swarm.Tasks[0].OperationID == op.ID {
					t.Fatal("fresh reservation did not preserve old attempt")
				}
			}
			if controlledAllocationCount(f.env.be) != 0 {
				t.Fatal("consent started backend without dispatch")
			}
		})
	}
}

func TestSwarmFencedHTTPStaleAbandonCannotMintUnclaimedRetirement(t *testing.T) {
	f, _, _ := fencedTaskFixture(t)
	// The actual HTTP allocator has already claimed the reserved operation and
	// attempted Create. The old reservation revision cannot publish no-launch proof.
	body, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision - 1})
	w := f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/tasks/api/abandon", string(body))
	if w.Code != 409 {
		t.Fatal("stale abandonment accepted", w.Code, w.Body.String())
	}
	run, err := f.env.api.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil || run.Operations[0].UnclaimedRetirement || run.Revision != f.run.Revision {
		t.Fatal("claimed operation acquired unclaimed retirement", err)
	}
	if ledger, err := f.env.api.execution.readFenceLedger(); err != nil || len(ledger.Leases) != 1 || ledger.Leases[0].State != "backend-dispatching" {
		t.Fatal("stale request released uncertain claim", err)
	}
}
