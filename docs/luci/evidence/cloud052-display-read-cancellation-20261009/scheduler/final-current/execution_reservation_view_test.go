package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
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

func reservationTestGate(t *testing.T) *executionAdmission {
	t.Helper()
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	actor := management.ExecutionActorRef{Local: true, ProjectID: "summary-worker"}
	d, err := g.reserve(context.Background(), "summary-one", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
	if err != nil || d.State != "admitted" {
		t.Fatal("reservation setup", d, err)
	}
	return g
}
func reservationJSON(t *testing.T, g *executionAdmission, ctx context.Context) map[string]any {
	t.Helper()
	b, err := json.Marshal(g.reservationObservation(ctx))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func reservationUnknown(t *testing.T, v map[string]any) {
	t.Helper()
	if v["status"] != "unavailable" {
		t.Fatal("expected explicitly unavailable observation", v)
	}
	for _, key := range []string{"count", "memoryMiB", "cpuMillis", "observedAt"} {
		if _, ok := v[key]; ok {
			t.Fatal("unknown observation published numeric demand or stale time", v)
		}
	}
}
func TestExecutionReservationViewUnavailableAndReadOnly(t *testing.T) {
	for _, name := range []string{"missing-root", "missing-ledger", "missing-lock", "invalid-json", "invalid-foreign-owner", "oversized", "lock-directory", "linked-lock", "linked-ledger", "hardlinked-lock", "hardlinked-ledger", "project-owned-ledger", "relative-path"} {
		t.Run(name, func(t *testing.T) {
			g := reservationTestGate(t)
			root := filepath.Dir(g.path)
			lock := filepath.Join(root, "leases.lock")
			original, err := os.ReadFile(g.path)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "missing-root":
				g.path = filepath.Join(t.TempDir(), "absent", "leases.json")
			case "missing-ledger":
				if err := os.Remove(g.path); err != nil {
					t.Fatal(err)
				}
			case "missing-lock":
				if err := os.Remove(lock); err != nil {
					t.Fatal(err)
				}
			case "invalid-json":
				if err := os.WriteFile(g.path, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "invalid-foreign-owner":
				l, err := g.readFenceLedger()
				if err != nil {
					t.Fatal(err)
				}
				l.Leases[0].Scope = strings.Repeat("b", 64)
				l.Leases[0].Actor.ProjectID = "different-project"
				b, _ := json.Marshal(l)
				if err := os.WriteFile(g.path, b, 0600); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				if err := os.WriteFile(g.path, bytes.Repeat([]byte(" "), (2<<20)+1), 0600); err != nil {
					t.Fatal(err)
				}
			case "lock-directory":
				if err := os.Remove(lock); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(lock, 0700); err != nil {
					t.Fatal(err)
				}
			case "linked-lock", "linked-ledger":
				target := lock
				if name == "linked-ledger" {
					target = g.path
				}
				renamed := target + ".retained"
				if err := os.Rename(target, renamed); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(renamed, target); err != nil {
					t.Skip("symlinks unavailable", err)
				}
			case "hardlinked-lock", "hardlinked-ledger":
				target := lock
				if name == "hardlinked-ledger" {
					target = g.path
				}
				if err := os.Link(target, target+".alias"); err != nil {
					t.Skip("hardlinks unavailable", err)
				}
			case "project-owned-ledger":
				g.api.deps.ProjectsDir = filepath.Join(t.TempDir(), "projects")
				dir := filepath.Join(g.api.projectsDir(), "untrusted-ledger")
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				g.path = filepath.Join(dir, "leases.json")
				if err := os.WriteFile(g.path, original, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "leases.lock"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "relative-path":
				g.path = "relative/leases.json"
			}
			// Reading must preserve the exact names, modes, mtimes and bytes it sees;
			// unlike admission, it must not repair or create private state.
			before := reservationTree(t, root)
			reservationUnknown(t, reservationJSON(t, g, context.Background()))
			after := reservationTree(t, root)
			if before != after {
				t.Fatal("unavailable GET modified retained state")
			}
			if name == "missing-root" {
				if _, err := os.Lstat(filepath.Dir(g.path)); !os.IsNotExist(err) {
					t.Fatal("GET created missing private root", err)
				}
			}
		})
	}
	t.Run("ordinary-read-preserves-modes-and-bytes", func(t *testing.T) {
		g := reservationTestGate(t)
		if err := os.Chmod(g.path, 0400); err != nil {
			t.Fatal(err)
		}
		lock := filepath.Join(filepath.Dir(g.path), "leases.lock")
		if err := os.Chmod(lock, 0400); err != nil {
			t.Fatal(err)
		}
		before := reservationTree(t, filepath.Dir(g.path))
		v := reservationJSON(t, g, context.Background())
		if v["status"] != "ready" || v["count"] != float64(1) {
			t.Fatal("read-only files should be observable", v)
		}
		if before != reservationTree(t, filepath.Dir(g.path)) {
			t.Fatal("read repaired file permissions or rewrote state")
		}
	})
}
func reservationTree(t *testing.T, root string) string {
	t.Helper()
	var rows []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		var data []byte
		if info.Mode().IsRegular() {
			data, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		row, _ := json.Marshal([]any{rel, info.Mode().String(), info.ModTime().UnixNano(), string(data)})
		rows = append(rows, string(row))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(rows, "\n")
}
func TestExecutionReservationViewBoundedLockAndCancellation(t *testing.T) {
	g := reservationTestGate(t)
	lock, err := os.Open(filepath.Join(filepath.Dir(g.path), "leases.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	release, err := lockExecutionLedger(context.Background(), lock)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	before := reservationTree(t, filepath.Dir(g.path))
	start := time.Now()
	reservationUnknown(t, reservationJSON(t, g, context.Background()))
	elapsed := time.Since(start)
	if elapsed < 80*time.Millisecond || elapsed > time.Second {
		t.Fatal("lock wait was not bounded", elapsed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start = time.Now()
	reservationUnknown(t, reservationJSON(t, g, ctx))
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("cancelled read waited for lock")
	}
	if before != reservationTree(t, filepath.Dir(g.path)) {
		t.Fatal("failed lock read changed state")
	}
	release()
	release = func() {}
	if v := reservationJSON(t, g, context.Background()); v["status"] != "ready" || v["count"] != float64(1) {
		t.Fatal("reader did not recover after lock release", v)
	}
}
func TestExecutionReservationViewConcurrentPeerMutation(t *testing.T) {
	first := reservationTestGate(t)
	peer := executionTestGate(t, filepath.Dir(first.path))
	actor := management.ExecutionActorRef{Local: true, ProjectID: "other-summary-worker"}
	failed := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 20; i++ {
			d, err := peer.reserve(context.Background(), "peer-row", actor.ProjectID, "api", 2, actor, management.ExecutionAuthority{}, executionFixturePolicy())
			if err != nil {
				failed <- err
				return
			}
			if d.State != "admitted" {
				failed <- fmt.Errorf("peer admission: %s/%s", d.State, d.Code)
				return
			}
			if err := peer.release(context.Background(), "peer-row"); err != nil {
				failed <- err
				return
			}
		}
	}()
	defer func() { <-done }()
	ready := 0
	for i := 0; i < 30; i++ {
		v := reservationJSON(t, first, context.Background())
		if v["status"] == "unavailable" {
			reservationUnknown(t, v)
			continue
		}
		ready++
		count := v["count"].(float64)
		if count != 1 && count != 2 || v["memoryMiB"] != count*1024 || v["cpuMillis"] != count*1000 {
			t.Fatal("torn shared reservation observation", v)
		}
	}
	<-done
	select {
	case err := <-failed:
		t.Fatal(err)
	default:
	}
	if ready == 0 {
		t.Fatal("no successful bounded shared observation")
	}
	v := reservationJSON(t, first, context.Background())
	if v["count"] != float64(1) {
		t.Fatal("peer release removed foreign retained demand", v)
	}
}
func TestExecutionReservationViewDoesNotWidenSchedulerAuthority(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t)
	calls := 0
	f.env.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot { calls++; return executionFixtureSnapshot() }
	for _, kind := range []string{"member", "team-admin"} {
		req := httptest.NewRequest("GET", sessionsTarget("/v1/product/swarm-scheduler"), nil)
		authority := management.ExecutionAuthority{Actor: management.User{ID: "limited-user", Role: "member"}}
		if kind == "team-admin" {
			authority.Actor.Role = "admin"
		}
		ctx := management.WithExecutionAuthority(req.Context(), authority)
		if kind == "team-admin" {
			ctx = management.WithTeamAccess(ctx, management.TeamAccess{Team: management.Team{ID: "team-private"}})
		}
		w := httptest.NewRecorder()
		f.env.api.guarded.ServeHTTP(w, req.WithContext(ctx))
		if w.Code != 403 || !strings.Contains(w.Body.String(), "scheduler_policy_forbidden") || calls != 0 {
			t.Fatal("global resources exposed before authorization", kind, w.Code, calls)
		}
	}
}

func TestExecutionReservationViewFinishedChecksAndNoHostSampler(t *testing.T) {
	g := reservationTestGate(t)
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot { return httpCheckSnapshot(12) }
	actor := management.ExecutionActorRef{UserID: "summary-team-user", TeamID: "summary-team", ProjectID: "summary-check-project"}
	identity := backend.CheckIdentity{Protocol: backend.CheckExecutionProtocol, EngineID: httpCheckEngine, InstanceID: httpCheckInstance, AllocationID: "summary-check-allocation", CheckID: "summary-check", ProjectID: actor.ProjectID, ScopeDigest: g.scope, CommandDigest: strings.Repeat("a", 64)}
	p := backend.PreparedCheck{Spec: backend.CheckSpec{Identity: identity, Resources: backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}}, ImageVirtualSizeBytes: 1 << 30, StorageDriver: "overlay2"}
	d, err := g.reserveCheck(context.Background(), p, actor, checkHTTPAuthority(actor.UserID), executionFixturePolicy())
	if err != nil || d.State != "admitted" {
		t.Fatal("check reservation", d, err)
	}
	if v := reservationJSON(t, g, context.Background()); v["count"] != float64(2) {
		t.Fatal("worker and reserved team check were not aggregated", v)
	}
	// A reserved command cancelled before any Create has a supported production
	// completion transition. The retained finished receipt no longer charges.
	outcome := product.ProjectCheckCommandOutcome{AllocationID: identity.AllocationID, Status: "cancelled", Stage: "admission", CleanupConfirmed: true}
	if err := g.completeCheck(context.Background(), identity, "", outcome); err != nil {
		t.Fatal(err)
	}
	ledger, err := g.readFenceLedger()
	if err != nil || len(ledger.Checks) != 1 || ledger.Checks[0].State != "finished" {
		t.Fatal("finished receipt missing", err)
	}
	g.api.deps.ResourceSnapshot = nil
	data, err := json.Marshal(g.api.queuedResourceView(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Status       string              `json:"status"`
		Reservations reservationHTTPView `json:"reservations"`
	}
	if json.Unmarshal(data, &result) != nil || result.Status != "unavailable" || result.Reservations.Status != "ready" {
		t.Fatal("host unavailability erased verified reservation observation", string(data))
	}
	reservationWant(t, result.Reservations, 1)
	for _, secret := range []string{actor.UserID, actor.TeamID, actor.ProjectID, identity.AllocationID, g.scope, g.path} {
		if bytes.Contains(data, []byte(secret)) {
			t.Fatal("global totals exposed private ownership")
		}
	}
	if err := g.acknowledgeCheckCompletion(context.Background(), identity); err != nil {
		t.Fatal(err)
	}
	if v := reservationJSON(t, g, context.Background()); v["count"] != float64(1) {
		t.Fatal("receipt acknowledgement released another charge", v)
	}
}
