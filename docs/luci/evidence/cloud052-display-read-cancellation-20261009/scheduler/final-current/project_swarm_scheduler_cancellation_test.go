package product

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSwarmSchedulerCancelledResponseKeepsSettledPolicyMutation(t *testing.T) {
	s := sharedPolicyServer(t, t.TempDir(), "owner", "")
	mux := http.NewServeMux()
	s.registerSwarmScheduler(mux)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type requestKey struct{}
	ctx = context.WithValue(ctx, requestKey{}, "original-request")
	calls := 0
	s.swarmScheduler.runtime.Resources = func(received context.Context) any {
		calls++
		if received != ctx || received.Value(requestKey{}) != "original-request" {
			t.Error("display callback lost the original request context")
		}
		cancel() // Policy persistence has settled; only its display reply is lost.
		return map[string]any{"status": "unavailable"}
	}
	policy := defaultSwarmSchedulerPolicy()
	policy.MaxActive = 3
	body, err := json.Marshal(map[string]any{"expectedRevision": 1, "policy": policy, "dispatchPaused": true})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/v1/product/swarm-scheduler", strings.NewReader(string(body))).WithContext(ctx)
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if calls != 1 || w.Body.Len() != 0 {
		t.Fatal("cancelled mutation response continued display work", calls, w.Body.String())
	}
	s.swarmScheduler.mu.Lock()
	stored, err := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if err != nil || stored.PolicyRevision != 2 || !stored.Paused || stored.Policy != policy {
		t.Fatal("display cancellation undid a settled policy mutation", stored, err)
	}
	s.swarmScheduler.runtime.Resources = func(context.Context) any { return map[string]any{"status": "unavailable"} }
	live := httptest.NewRecorder()
	mux.ServeHTTP(live, httptest.NewRequest("GET", "/v1/product/swarm-scheduler", nil))
	var response struct {
		Revision       int                  `json:"revision"`
		DispatchPaused bool                 `json:"dispatchPaused"`
		Policy         SwarmSchedulerPolicy `json:"policy"`
	}
	if live.Code != 200 || json.Unmarshal(live.Body.Bytes(), &response) != nil || response.Revision != 2 || !response.DispatchPaused || response.Policy != policy {
		t.Fatal("live reconciliation lost the settled policy", live.Code, live.Body.String())
	}
	// The consumed revision must still conflict; display cancellation cannot
	// make the same uncertain mutation safe to replay.
	replay := httptest.NewRecorder()
	replayRequest := httptest.NewRequest("PUT", "/v1/product/swarm-scheduler", strings.NewReader(string(body)))
	replayRequest.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(replay, replayRequest)
	if replay.Code != 409 {
		t.Fatal("display cancellation reset mutation revision protection", replay.Code, replay.Body.String())
	}
}
