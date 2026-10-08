package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

type reservationHTTPView struct {
	Status     string    `json:"status"`
	ObservedAt time.Time `json:"observedAt"`
	Count      *int      `json:"count"`
	MemoryMiB  *int      `json:"memoryMiB"`
	CPUMillis  *int      `json:"cpuMillis"`
}

func reservationHTTPRead(t *testing.T, f *swarmSessionFixture) reservationHTTPView {
	t.Helper()
	w := f.env.do(t, "GET", "/v1/product/swarm-scheduler", "")
	var out struct {
		DispatchPaused bool `json:"dispatchPaused"`
		Resources      struct {
			Reservations reservationHTTPView `json:"reservations"`
		} `json:"resources"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil || !out.DispatchPaused {
		t.Fatal("paused owner capacity read failed", w.Code, w.Body.String())
	}
	return out.Resources.Reservations
}
func reservationWant(t *testing.T, v reservationHTTPView, count int) {
	t.Helper()
	if v.Count == nil || v.MemoryMiB == nil || v.CPUMillis == nil || *v.Count != count || *v.MemoryMiB != count*1024 || *v.CPUMillis != count*1000 {
		t.Fatalf("shared reservation totals = %+v; want %d allocations / %d MiB / %d mCPU", v, count, count*1024, count*1000)
	}
}

// Real HTTP checks and production admission/release with a controlled backend;
// no Docker, gateway deployment, model or command process is involved.
func TestExecutionReservationViewPausedOwnerObservesTeamCheck(t *testing.T) {
	owner, team := newSwarmSessionFixtureTasks(t), newSwarmSessionFixtureTasks(t)
	shared := filepath.Join(t.TempDir(), "capacity")
	be := newHTTPCheckExecutor(team.env.be)
	gate := make(chan struct{})
	var once sync.Once
	be.createGate = gate
	wireHTTPCheckFixture(t, owner, newHTTPCheckExecutor(owner.env.be), shared, "owner-user", 12)
	wireHTTPCheckFixture(t, team, be, shared, "team-user", 12)
	if owner.env.api.execution.scope == team.env.api.execution.scope {
		t.Fatal("test needs two independent API scopes")
	}
	actor := management.ExecutionActorRef{Local: true, ProjectID: owner.owner.ID}
	d, err := owner.env.api.execution.reserve(context.Background(), "unrelated-owner", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if err != nil || d.State != "admitted" {
		t.Fatal("owner reservation", d, err)
	}
	reservationWant(t, reservationHTTPRead(t, owner), 1)
	access := management.TeamAccess{Team: management.Team{ID: "team-controlled-summary"}, ProjectID: team.owner.ID, Permissions: management.TeamPermissions{Read: true, Edit: true, Execute: true}}
	team.env.api.deps.AuthorizeChecks = func(_ context.Context, ref management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		if ref.UserID != "team-user" || ref.TeamID != access.Team.ID || ref.ProjectID != team.owner.ID || ref.Local {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		a := checkHTTPAuthority("team-user")
		a.Team = &access
		return a, nil
	}
	req := httptest.NewRequest("POST", sessionsTarget("/v1/product/projects/"+team.owner.ID+"/checks"), strings.NewReader(checkHTTPBody()))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(management.WithTeamAccess(management.WithExecutionAuthority(req.Context(), checkHTTPAuthority("team-user")), access))
	w := httptest.NewRecorder()
	team.env.api.guarded.ServeHTTP(w, req)
	var started product.ProjectCheckRun
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &started) != nil || started.ID == "" {
		t.Fatal("team check consent", w.Code, w.Body.String())
	}
	defer func() {
		once.Do(func() { close(gate) })
		done := checkHTTPWait(t, team, "team-user", started.ID, func(r product.ProjectCheckRun) bool {
			return r.Status == "passed" && r.ResourcesHeld != nil && !*r.ResourcesHeld
		})
		checkHTTPIdle(t, team, done)
		checkHTTPCompletionAcknowledged(t, team, done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, creates, _, _, _ := be.counts()
		if creates == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("team check did not reach creating")
		}
		time.Sleep(5 * time.Millisecond)
	}
	ledger, err := team.env.api.execution.readFenceLedger()
	if err != nil || len(ledger.Checks) != 1 || ledger.Checks[0].Actor.TeamID != access.Team.ID || ledger.Checks[0].State != "creating" {
		t.Fatal("team did not persist exact creating charge", err)
	}
	before, err := os.ReadFile(owner.env.api.execution.path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		v := reservationHTTPRead(t, owner)
		reservationWant(t, v, 2)
		if v.Status != "ready" || time.Since(v.ObservedAt) > time.Second {
			t.Fatal("summary lacks ready/fresh observation", v)
		}
	}
	after, err := os.ReadFile(owner.env.api.execution.path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("GET rewrote the shared ledger", err)
	}
	_, creates, starts, _, removes := be.counts()
	if creates != 1 || starts != 0 || removes != 0 {
		t.Fatal("GET changed controlled check execution", creates, starts, removes)
	}
	once.Do(func() { close(gate) })
	done := checkHTTPWait(t, team, "team-user", started.ID, func(r product.ProjectCheckRun) bool {
		return r.Status == "passed" && r.ResourcesHeld != nil && !*r.ResourcesHeld
	})
	checkHTTPIdle(t, team, done)
	checkHTTPCompletionAcknowledged(t, team, done)
	reservationWant(t, reservationHTTPRead(t, owner), 1)
	// A fresh API has no per-instance mutation history but must read the same
	// unrelated retained charge. Team completion must not release that charge.
	owner.env.api.execution = newExecutionAdmission(owner.env.api)
	reservationWant(t, reservationHTTPRead(t, owner), 1)
	if err := owner.env.api.execution.release(context.Background(), "unrelated-owner"); err != nil {
		t.Fatal(err)
	}
	reservationWant(t, reservationHTTPRead(t, owner), 0)
}
