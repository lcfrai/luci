package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func TestLocal044ManagedUsersShareOneHostPressureDecision(t *testing.T) {
	g, public, ownerToken, _ := fixture(t, nil)
	request(t, public, "GET", hostPressurePath, ownerToken, nil, 404)
	request(t, public, "POST", "/api"+hostPressurePath, ownerToken, nil, 404)
	users := []User{addUser(t, public, ownerToken, "alice", "member"), addUser(t, public, ownerToken, "bob", "member")}
	var stopped [2]atomic.Int32
	for i, user := range users {
		index := i
		candidate := HostPressureCandidate{SessionID: "s-person-" + user.ID, SandboxID: "exact-" + user.ID, SandboxRef: "owned-ref-" + user.ID, LeaseID: "lease-" + user.ID, CreatedAt: time.Now().Add(time.Duration(i) * time.Minute)}
		secret := randomToken()
		worker := WorkerHandler(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Luci-Host-Pressure-Control") != secret || r.URL.Path != hostPressurePath {
				http.Error(w, "wrong internal route", 403)
				return
			}
			switch r.Method {
			case http.MethodGet:
				_ = json.NewEncoder(w).Encode(HostPressureObservation{ReserveBytes: 1 << 30, Candidate: &candidate})
			case http.MethodPost:
				var exact HostPressureCandidate
				if json.NewDecoder(r.Body).Decode(&exact) != nil || exact.SessionID != candidate.SessionID || exact.SandboxID != candidate.SandboxID || exact.SandboxRef != candidate.SandboxRef || exact.LeaseID != candidate.LeaseID || !exact.CreatedAt.Equal(candidate.CreatedAt) {
					http.Error(w, "another worker was selected", 409)
					return
				}
				stopped[index].Add(1)
				_ = json.NewEncoder(w).Encode(map[string]bool{"acted": true})
			}
		}))
		server := httptest.NewServer(worker)
		t.Cleanup(server.Close)
		rt := &userRuntime{user: user, policy: g.state.Policy, secret: secret, address: server.URL, status: RuntimeStatus{Status: "running"}, ready: make(chan struct{}), done: make(chan struct{})}
		g.mu.Lock()
		g.runtimes[user.ID] = rt
		g.mu.Unlock()
	}
	t.Cleanup(func() {
		g.mu.Lock()
		for _, user := range users {
			delete(g.runtimes, user.ID)
		}
		g.mu.Unlock()
	})
	g.opts.HostPressureInitialObservation = func(context.Context, uint64) (HostPressureObservation, error) {
		return HostPressureObservation{ReserveBytes: 1 << 30}, nil
	}
	g.opts.HostPressureLog = func(err error) { t.Logf("pressure guard: %v", err) }
	g.opts.HostPressureInitialStop = func(context.Context, HostPressureCandidate) (bool, error) {
		t.Fatal("owner's empty registry was chosen over managed work")
		return false, nil
	}
	total, available := uint64(8<<30), uint64(900<<20)
	sample := func() resourcebudget.HostSnapshot {
		return resourcebudget.HostSnapshot{ObservedAt: time.Now().UTC(), Memory: resourcebudget.Memory{EffectiveTotalBytes: &total, EffectiveAvailableBytes: &available}}
	}
	g.pressureStep(context.Background(), sample())
	if stopped[0].Load()+stopped[1].Load() != 0 {
		t.Fatal("transient host pressure stopped work")
	}
	g.pressureStep(context.Background(), sample())
	for range 9 {
		g.pressureStep(context.Background(), sample())
	}
	if stopped[0].Load() != 0 || stopped[1].Load() != 1 {
		t.Fatal("managed registry scope did not select exactly one newest owned session", stopped[0].Load(), stopped[1].Load())
	}
	available = 2 << 30
	for range 3 {
		g.pressureStep(context.Background(), sample())
	}
	available = 900 << 20
	g.pressureStep(context.Background(), sample())
	if stopped[0].Load()+stopped[1].Load() != 1 {
		t.Fatal("three healthy samples rearmed a second stop")
	}
}

func TestLocal044LostManagedStopAcknowledgmentDoesNotCascade(t *testing.T) {
	g, _, _, _ := fixture(t, nil)
	user := User{ID: "person-one", Role: "member"}
	secret := randomToken()
	var calls atomic.Int32
	server := httptest.NewServer(WorkerHandler(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(HostPressureObservation{ReserveBytes: 1 << 30, Candidate: &HostPressureCandidate{SessionID: "s-owned", SandboxID: "exact", SandboxRef: "ref", LeaseID: "lease", CreatedAt: time.Now().Add(-time.Minute)}})
			return
		}
		calls.Add(1)
		http.Error(w, "stop response lost after effect", 503)
	})))
	defer server.Close()
	g.mu.Lock()
	g.runtimes[user.ID] = &userRuntime{user: user, policy: g.state.Policy, secret: secret, address: server.URL, status: RuntimeStatus{Status: "running"}, ready: make(chan struct{}), done: make(chan struct{})}
	g.mu.Unlock()
	t.Cleanup(func() { g.mu.Lock(); delete(g.runtimes, user.ID); g.mu.Unlock() })
	g.opts.HostPressureInitialObservation = func(context.Context, uint64) (HostPressureObservation, error) {
		return HostPressureObservation{ReserveBytes: 1 << 30}, nil
	}
	total, available := uint64(8<<30), uint64(900<<20)
	for range 10 {
		g.pressureStep(context.Background(), resourcebudget.HostSnapshot{ObservedAt: time.Now().UTC(), Memory: resourcebudget.Memory{EffectiveTotalBytes: &total, EffectiveAvailableBytes: &available}})
	}
	if calls.Load() != 1 {
		t.Fatal("uncertain managed stop retried or cascaded", calls.Load())
	}
	var recorded hostPressureEpisode
	if data, err := os.ReadFile(g.pressureEpisodePath()); err != nil || json.Unmarshal(data, &recorded) != nil || recorded.Cause != "host_memory_pressure" {
		t.Fatal("the private episode lost its stable machine cause", err)
	}
	g.hostPressure = hostPressureState{} // Recreate the process-local guard state.
	if err := g.loadPressureEpisode(); err != nil || !g.hostPressure.acted {
		t.Fatal("restart lost the durable one-stop fence", err)
	}
	for range 3 {
		g.pressureStep(context.Background(), resourcebudget.HostSnapshot{ObservedAt: time.Now().UTC(), Memory: resourcebudget.Memory{EffectiveTotalBytes: &total, EffectiveAvailableBytes: &available}})
	}
	if calls.Load() != 1 {
		t.Fatal("restart replayed an uncertain managed stop", calls.Load())
	}
}
