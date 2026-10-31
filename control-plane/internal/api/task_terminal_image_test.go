package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Existing session tests use a controlled capable-image advertisement, never an
// installed-image/model claim. Immutable helper image references stay exact.
func (b *sessionsBackend) ObserveCodexTaskResultImage(_ context.Context, reference string) (string, error) {
	if strings.HasPrefix(reference, "sha256:") {
		return reference, nil
	}
	return "sha256:" + strings.Repeat("a", 64), nil
}
func (b *handoffLifecycleBackend) ObserveCodexTaskResultImage(ctx context.Context, reference string) (string, error) {
	observer, ok := b.Backend.(backend.CodexTaskResultImageObserver)
	if !ok {
		return "", backend.ErrUnsupported
	}
	return observer.ObserveCodexTaskResultImage(ctx, reference)
}

type refusedTerminalImage struct{ backend.Backend }

func (b *refusedTerminalImage) ObserveCodexTaskResultImage(context.Context, string) (string, error) {
	return "", backend.ErrUnsupported
}

func TestTaskTerminalLegacyImageRefusesBeforeBackendCreate(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t, "api")
	f.env.api.deps.Backend = &refusedTerminalImage{Backend: f.env.api.deps.Backend}
	body, _ := json.Marshal(f.requests[0])
	response := f.env.do(t, "POST", "/v1/sessions", string(body))
	if response.Code < 400 || controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("unknown capability reached allocation", response.Code, response.Body.String())
	}
}

type changedTerminalImage struct {
	backend.Backend
	id    string
	calls int
}

func (b *changedTerminalImage) ObserveCodexTaskResultImage(context.Context, string) (string, error) {
	b.calls++
	return b.id, nil
}
func TestTaskTerminalFinalImageReadbackRefusesDifferentImmutableIdentity(t *testing.T) {
	_, row, _, _ := terminalContractFixture()
	image := "sha256:" + strings.Repeat("a", 64)
	probe := &changedTerminalImage{id: "sha256:" + strings.Repeat("b", 64)}
	a := &API{deps: Deps{Backend: probe}}
	row.Command = "/usr/local/bin/node"
	row.Args = []string{taskTerminalWrapper, strings.Repeat("c", 64), "/usr/local/bin/codex"}
	if err := a.validateTaskTerminalImage(context.Background(), row, image); err == nil || probe.calls != 1 {
		t.Fatal("changed image admitted", err)
	}
	probe.id = "unknown"
	if _, err := a.taskTerminalImage(context.Background(), image); err == nil {
		t.Fatal("unknown image identity admitted")
	}
}

func TestTaskTerminalControlledBackendDoesNotInventRuntimeEOF(t *testing.T) {
	b := sessionsNewBackend()
	b.specs = []backend.Spec{{ID: "s-12345678", Command: "/usr/local/bin/node", Args: []string{taskTerminalWrapper, "contract"}}}
	conn, err := b.Attach(context.Background(), "s-12345678", "fixture-token")
	if conn != nil || err != backend.ErrUnsupported {
		t.Fatal("controlled backend invented terminal stream/EOF", err)
	}
}

func TestTaskTerminalCapabilityGatePreservesUnwrappedSessions(t *testing.T) {
	for _, row := range []registry.Session{
		{Harness: "codex", Command: "codex", Args: []string{"--model", "default"}},
		{Harness: "codex", Command: "codex", Args: []string{"exec", "--", "legacy task"}},
		{Harness: "claude-code", Command: "claude", Args: []string{"--print", "task"}},
		{Harness: "opencode", Command: "opencode", Args: []string{"run", "--", "task"}},
		{Command: "/usr/local/bin/node", Args: []string{"/usr/local/lib/luci/unrelated.cjs"}},
	} {
		probe := &changedTerminalImage{id: "unknown"}
		a := &API{deps: Deps{Backend: probe}}
		if err := a.validateTaskTerminalImage(context.Background(), row, "legacy-image"); err != nil || probe.calls != 0 {
			t.Fatalf("ordinary %s session unexpectedly required structured image capability: %v (calls %d)", row.Harness, err, probe.calls)
		}
	}
}
