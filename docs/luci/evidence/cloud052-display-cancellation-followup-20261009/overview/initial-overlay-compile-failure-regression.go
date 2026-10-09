package product

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestProjectOverviewAlreadyCancelledDoesNotEnterSourceOrFill(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "already-cancelled-display")
	before := auditTree(t, p.Path)
	beforePrivate := auditTree(t, s.stateDir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := httptest.NewRecorder()
	mux.ServeHTTP(out, httptest.NewRequest("GET", "/v1/product/projects/"+p.ID+"/overview", nil).WithContext(ctx))
	// The source registrar checks cancellation before creating an admission
	// guard or resolving the project. These are entry oracles, not syscall
	// tracing or a promise to preempt every filesystem operation.
	if len(s.branchGuards) != 0 || s.projectOverview.fill != nil || s.projectOverview.sequence != 0 || len(s.projectOverview.entries) != 0 {
		t.Fatal("already-cancelled read entered source admission or cache fill")
	}
	if !reflect.DeepEqual(before, auditTree(t, p.Path)) || !reflect.DeepEqual(beforePrivate, auditTree(t, s.stateDir)) {
		t.Fatal("already-cancelled display changed project or private state")
	}
}

type overviewFollowupRegistrar struct {
	Registrar
	peerEntered chan struct{}
}

func (r overviewFollowupRegistrar) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	// Observe arrival after the real source registrar has acquired admission,
	// before delegating to the unchanged production handler. No production hook.
	r.Registrar.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("X-Overview-Fixture") == "peer" {
			close(r.peerEntered)
		}
		handler(w, req)
	})
}

func TestProjectOverviewCancelledInitiatorLeavesLiveSameProjectWaiter(t *testing.T) {
	s, mux := journeyFixture(t)
	entered := make(chan struct{})
	s.registerProjectOverview(overviewFollowupRegistrar{Registrar: s.guardProjectSourceRoutes(mux), peerEntered: entered})
	p := createJourneyProject(t, mux, "independent-display-waiter")
	state := forgedReviewState(p.ID)
	state.Brief.Purpose = "Only the still-live reader should publish this project"
	if err := writeJourney(p.Path, state); err != nil {
		t.Fatal(err)
	}
	s.projectRuns.mu.Lock()
	blocked := true
	defer func() {
		if blocked {
			s.projectRuns.mu.Unlock()
		}
	}()
	cancel, initiatingDone, initiatingResponse := overviewAsync(mux, p)
	defer cancel()
	overviewWait(t, "initiator never entered its fill", func() bool {
		if s.projectOverview.mu.TryLock() {
			s.projectOverview.mu.Unlock()
			return false
		}
		return true
	})
	peerResponse := httptest.NewRecorder()
	peerDone := make(chan struct{})
	peerRequest := httptest.NewRequest("GET", "/v1/product/projects/"+p.ID+"/overview", nil)
	peerRequest.Header.Set("X-Overview-Fixture", "peer")
	go func() { defer close(peerDone); mux.ServeHTTP(peerResponse, peerRequest) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("live same-project request did not acquire source admission")
	}
	cancel()
	s.projectRuns.mu.Unlock()
	blocked = false
	overviewAwait(t, initiatingDone)
	overviewAwait(t, peerDone)
	if initiatingResponse.Body.Len() != 0 {
		t.Error("cancelled initiating request published its response")
	}
	var view ProjectOverview
	if peerResponse.Code != 200 || json.Unmarshal(peerResponse.Body.Bytes(), &view) != nil || view.ProjectID != p.ID || view.Status != "ready" || view.Goal == nil || view.Goal.Text != state.Brief.Purpose {
		t.Fatalf("live same-project waiter did not retain its own current read: %d %s", peerResponse.Code, peerResponse.Body.String())
	}
	if overviewSourceAdmitted(s, p.ID) {
		t.Fatal("settled display requests retained source admission")
	}
	if len(s.projectOverview.entries) != 1 {
		t.Fatal("live request did not retain one compact project entry")
	}
	for _, entry := range s.projectOverview.entries {
		if entry.view.ProjectID != p.ID || entry.view.Goal == nil || entry.view.Goal.Text != state.Brief.Purpose {
			t.Fatal("live peer retained an incorrect cached project projection")
		}
	}
}
