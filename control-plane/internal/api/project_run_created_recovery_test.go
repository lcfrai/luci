package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type interruptedRunBackend struct {
	backend.Backend
	child   backend.Sandbox
	removed []string
}

func (b *interruptedRunBackend) Get(_ context.Context, id string) (backend.Sandbox, error) {
	if b.child.ID != id {
		return backend.Sandbox{}, backend.ErrNotFound
	}
	return b.child, nil
}
func (b *interruptedRunBackend) List(context.Context) ([]backend.Sandbox, error) {
	if b.child.ID == "" {
		return nil, nil
	}
	return []backend.Sandbox{b.child}, nil
}
func (b *interruptedRunBackend) Destroy(_ context.Context, id string) error {
	b.removed = append(b.removed, id)
	b.child = backend.Sandbox{}
	return nil
}

func TestInterruptedCreatedRunRetainsItsChildAndBlocksUntilExplicitNewAttempt(t *testing.T) {
	env, run, op, req, candidate := reservedProjectRunSession(t, false)
	admission, err := env.api.deps.Product.BeginProjectRunSession(run.ProjectID, run.ID, op.ID, candidate, env.st.Get)
	if err != nil {
		t.Fatal(err)
	}
	candidate.ID, candidate.SandboxID, candidate.Status = op.ChildID, op.ChildID, registry.StatusStarting
	candidate.Backend, candidate.AttachToken = "docker", "private-original-token"
	err = env.st.Create(candidate)
	admission.Release()
	if err != nil {
		t.Fatal(err)
	}
	const instance = "11111111-1111-4111-8111-111111111111"
	be := &interruptedRunBackend{Backend: env.be, child: backend.Sandbox{ID: op.ChildID, Ref: "original-created-container", Kind: backend.KindDocker, State: backend.StateStarting, CreatedOnly: true, Instance: instance, CreatedAt: time.Now().Add(-3 * time.Minute), Labels: map[string]string{backend.LabelManaged: "true", backend.LabelSandboxID: op.ChildID, backend.LabelSessionID: op.ChildID}}}
	env.api.deps.Backend = be
	base := "/v1/product/projects/" + run.ProjectID + "/runs/" + run.ID
	readRun := func() product.ProjectRun {
		var result struct {
			Run product.ProjectRun `json:"run"`
		}
		w := env.do(t, "GET", base, "")
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
			t.Fatal(w.Code, w.Body.String())
		}
		return result.Run
	}
	reconcile := func() product.ProjectRun {
		current := readRun()
		body, _ := json.Marshal(map[string]int{"expectedRevision": current.Revision})
		w := env.do(t, "POST", base+"/reconcile", string(body))
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		return readRun()
	}
	starting := reconcile()
	if !strings.Contains(starting.Reason, "agent execution has not been confirmed") || strings.Contains(starting.Reason, "session is active") {
		t.Fatal("starting child claimed confirmed execution", starting.Reason)
	}
	r := reaper.New(reaper.Config{Store: env.st, Backend: be, Instance: instance})
	for range 2 {
		if err := r.Sweep(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	blocked := reconcile()
	if blocked.State != "blocked" || blocked.Operations[0].Status != "failed" || blocked.Operations[0].ChildID != op.ChildID || blocked.Acceptance != nil {
		t.Fatal("interruption lost exact unverified child", blocked)
	}
	if len(be.removed) != 1 || be.removed[0] != op.ChildID {
		t.Fatal("did not remove original child only", be.removed)
	}
	// Repeating the exact original dispatch recovers the same terminal child;
	// it is never permission to invoke the selected paid provider a second time.
	body, _ := json.Marshal(req)
	w := env.do(t, "POST", "/v1/sessions", string(body))
	var child struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &child) != nil || child.ID != op.ChildID || child.Status != "exited" {
		t.Fatal("original dispatch was replayed", w.Code, w.Body.String())
	}
	if len(env.be.specs) != 0 {
		t.Fatal("recovery allocated a new backend child")
	}
	retained, err := env.st.Get(op.ChildID)
	if err != nil || retained.ProjectRunID != run.ID || retained.ProjectRunOperationID != op.ID || retained.OpeningPrompt != candidate.OpeningPrompt {
		t.Fatal("lost reviewed allocation identity", retained, err)
	}
}
