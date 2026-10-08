package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// This is controlled backend mechanics, not a real Docker/model witness. The
// real API allocator persists the original engine/protocol and captured context
// before the controlled Create fails with an ambiguous transport result.
type controlledLaunchFencer struct {
	backend.Backend
	mu                               sync.Mutex
	identity                         backend.LaunchFenceIdentity
	identityErr, fenceErr, verifyErr error
	identityCalls, verifyCalls       int
	requests                         []backend.LaunchFenceRequest
	proofs                           map[string]backend.LaunchFenceProof
	beforeFence                      func(backend.LaunchFenceRequest) error
	beforeVerify                     func(backend.LaunchFenceRequest) error
}

func fixtureFenceHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
func (b *controlledLaunchFencer) Kind() backend.Kind { return backend.KindDocker }
func (b *controlledLaunchFencer) LaunchFenceIdentity(context.Context) (backend.LaunchFenceIdentity, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.identityCalls++
	return b.identity, b.identityErr
}
func (b *controlledLaunchFencer) FenceLaunch(ctx context.Context, request backend.LaunchFenceRequest) (backend.LaunchFenceProof, error) {
	if err := ctx.Err(); err != nil {
		return backend.LaunchFenceProof{}, err
	}
	b.mu.Lock()
	b.requests = append(b.requests, request)
	index, hook, failure := len(b.requests), b.beforeFence, b.fenceErr
	b.mu.Unlock()
	if hook != nil {
		if err := hook(request); err != nil {
			return backend.LaunchFenceProof{}, err
		}
	}
	if failure != nil {
		return backend.LaunchFenceProof{}, failure
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stable := request
	stable.ExpectedOriginalRef = ""
	data, _ := json.Marshal(stable)
	proof := backend.LaunchFenceProof{Protocol: backend.LaunchFenceProtocol, EngineID: request.EngineID, InstanceID: request.InstanceID, SandboxID: request.SandboxID, SessionID: request.SessionID, IntentID: request.IntentID, ScopeDigest: request.ScopeDigest, OperationDigest: request.OperationDigest, FenceRef: fixtureFenceHash(fmt.Sprintf("controlled-fence-%s-%d", request.IntentID, index)), ConfigDigest: fixtureFenceHash(string(data)), AcknowledgedAt: now, ObservedAt: now}
	b.mu.Lock()
	b.proofs[request.IntentID] = proof
	b.mu.Unlock()
	return proof, nil
}
func (b *controlledLaunchFencer) VerifyLaunchFence(ctx context.Context, request backend.LaunchFenceRequest, proof backend.LaunchFenceProof) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	b.verifyCalls++
	known, present := b.proofs[request.IntentID]
	hook, failure := b.beforeVerify, b.verifyErr
	b.mu.Unlock()
	if hook != nil {
		if err := hook(request); err != nil {
			return err
		}
	}
	if failure != nil {
		return failure
	}
	if !present || known.FenceRef != proof.FenceRef || known.EngineID != request.EngineID || known.InstanceID != request.InstanceID || known.ScopeDigest != request.ScopeDigest || known.OperationDigest != request.OperationDigest || known.ConfigDigest != proof.ConfigDigest || known.AcknowledgedAt != proof.AcknowledgedAt {
		return backend.ErrLaunchFenceOwnership
	}
	return nil
}

func fencedTaskFixture(t *testing.T) (*swarmSessionFixture, *controlledLaunchFencer, string) {
	t.Helper()
	f := newSwarmSessionFixtureTasks(t, "api")
	a := f.env.api
	a.deps.ReadyTimeout = 5 * time.Second
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	b := &controlledLaunchFencer{Backend: f.env.be, identity: backend.LaunchFenceIdentity{Protocol: backend.LaunchFenceProtocol, EngineID: "controlled-original-engine", InstanceID: "11111111-1111-4111-8111-111111111111"}, proofs: map[string]backend.LaunchFenceProof{}}
	a.deps.Backend = b
	f.env.be.mu.Lock()
	f.env.be.createErr, f.env.be.destroyErr = backend.ErrUnavailable, backend.ErrUnavailable
	f.env.be.mu.Unlock()
	body, err := json.Marshal(f.requests[0])
	if err != nil {
		t.Fatal(err)
	}
	w := f.env.do(t, http.MethodPost, "/v1/sessions", string(body))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatal("controlled ambiguous Create did not reach backend", w.Code, w.Body.String())
	}
	ledger, err := a.execution.readFenceLedger()
	if err != nil || len(ledger.Leases) != 1 || ledger.Leases[0].FenceProtocol != backend.LaunchFenceProtocol || ledger.Leases[0].State != "backend-dispatching" || ledger.Leases[0].FenceEngineID != b.identity.EngineID {
		t.Fatal("real allocator did not retain guarded original identity", ledger, err)
	}
	f.run, err = a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return f, b, ledger.Leases[0].ID
}

func fencedRecoveryRequest(t *testing.T, f *swarmSessionFixture) product.ProjectSwarmFenceRecoveryRequest {
	t.Helper()
	run, err := f.env.api.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil {
		t.Fatal(err)
	}
	op := run.Operations[0]
	queueID, err := f.env.api.deps.Product.SwarmQueueIDForOperation(run.ProjectID, run.ID, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(op.Implementation)
	if err != nil {
		t.Fatal(err)
	}
	req := product.ProjectSwarmFenceRecoveryRequest{ProjectSwarmRecoveryRequest: product.ProjectSwarmRecoveryRequest{ProjectID: run.ProjectID, RunID: run.ID, OperationID: op.ID, ChildID: op.ChildID, QueueID: queueID, UpdatedAt: op.UpdatedAt}, RequestDigest: op.RequestDigest, ImplementationDigest: fixtureFenceHash(string(data))}
	if op.LaunchRecovery != nil {
		copy := *op.LaunchRecovery
		req.Intent = &copy
		req.UpdatedAt = copy.OriginalUpdatedAt
	}
	return req
}

func fencedRecover(t *testing.T, f *swarmSessionFixture, expected int) *httptest.ResponseRecorder {
	t.Helper()
	op := f.ops[0]
	body, _ := json.Marshal(map[string]any{"expectedRevision": f.run.Revision, "operationId": op.ID, "childId": op.ChildID, "stopUncertainLaunch": true})
	w := f.env.do(t, http.MethodPost, f.base()+"/"+f.run.ID+"/swarm/tasks/api/abandon", string(body))
	if w.Code != expected {
		t.Fatalf("stop uncertain launch returned%d want%d: %s", w.Code, expected, w.Body.String())
	}
	run, err := f.env.api.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	f.run = run
	return w
}
