package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

type swarmFixtureLog struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *swarmFixtureLog) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if left := 64*1024 - b.data.Len(); left > 0 {
		b.data.Write(p[:min(left, len(p))])
	}
	return len(p), nil
}

func logSwarmFixtureFailure(t *testing.T, f *swarmSessionFixture) {
	t.Helper()
	logs := &swarmFixtureLog{}
	f.env.api.deps.Log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		logs.mu.Lock()
		output := logs.data.String()
		logs.mu.Unlock()
		t.Logf("Controlled swarm fixture diagnostic (first 64 KiB):\n%s", output)
	})
}

// This verifies the real API-to-backend allocation contract with a controlled
// backend. Natural native CLI exit is a separate installed-image acceptance gate.
func TestSwarmTaskAllocatorUsesNativeOneShotForManualAndQueuedLaunch(t *testing.T) {
	for _, queued := range []bool{false, true} {
		t.Run(map[bool]string{false: "manual task start", true: "queued task start"}[queued], func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t, "api")
			logSwarmFixtureFailure(t, f)
			sessionsStubReady(t, nil)
			done := sessionsWatchDone(t)
			if queued {
				a := f.env.api
				a.deps.ResourceSnapshot = executionFixtureSnapshot
				a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
				actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
				policy, err := a.deps.Product.SwarmSchedulerPolicy()
				if err != nil {
					t.Fatal(err)
				}
				admission, err := a.admitQueuedSwarm(context.Background(), product.SwarmAdmissionRequest{QueueID: "queue-native-task", ProjectID: f.owner.ID, RunID: f.run.ID, Actor: actor, Policy: policy})
				if err != nil || admission.State != "admitted" {
					t.Fatal(admission, err)
				}
				result, err := a.dispatchQueuedSwarm(context.Background(), product.SwarmDispatchRequest{QueueID: "queue-native-task", ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: f.ops[0].ID, Actor: actor})
				if err != nil || result.State != "running" || result.SessionID != f.ops[0].ChildID {
					t.Fatal(result, err)
				}
			} else {
				body, _ := json.Marshal(f.requests[0])
				got := sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 201)
				if got.ID != f.ops[0].ChildID {
					t.Fatal("task child identity changed")
				}
			}
			sessionsAwaitWatch(t, done)
			spec := f.env.be.spec(t, 0)
			at := slices.Index(spec.Args, "exec")
			if spec.Command != "codex" || at < 0 || slices.Contains(spec.Args, "--no-alt-screen") || slices.Contains(spec.Args[at+1:], "-c") || !slices.Contains(spec.Args, "--dangerously-bypass-approvals-and-sandbox") {
				t.Fatalf("swarm allocated an interactive or conflicting Codex invocation: %s %v", spec.Command, spec.Args)
			}
			if !strings.Contains(strings.Join(spec.Args, "\n"), "developer_instructions=") || !strings.Contains(strings.Join(spec.Args, "\n"), "model_providers.luci-openai") {
				t.Fatal("native task lost captured-context instructions or provider overrides")
			}
			child, err := f.store.Get(f.ops[0].ChildID)
			if err != nil || child.PermissionMode != "bypass" || child.ReviewedContext == nil || child.ConversationID == "" || spec.Env["CODEX_HOME"] != nativeAgentDir {
				t.Fatal("task lost saved consent, captured context or retained native history", child, err)
			}
		})
	}
}

func TestSwarmTaskStartRejectsInteractiveModeWithoutBackendAllocation(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t, "api")
	logSwarmFixtureFailure(t, f)
	for _, mode := range []string{"ask", "plan"} {
		req := f.requests[0]
		req.PermissionMode = mode
		body, _ := json.Marshal(req)
		w := f.env.do(t, "POST", "/v1/sessions", string(body))
		if code := sessionsProblemCode(t, w, 409); code != "task_permission_requires_interactive" {
			t.Fatal("task silently changed interactive consent", code, w.Body.String())
		}
	}
	if controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("unsupported permission allocated a task worker")
	}
}
