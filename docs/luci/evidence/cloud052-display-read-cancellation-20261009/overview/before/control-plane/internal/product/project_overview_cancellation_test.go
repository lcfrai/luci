package product

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// These fixtures hold existing production locks, rather than pausing a fake
// cache loader. The request traverses the real source-admission registrar.
func overviewWait(t *testing.T, message string, condition func() bool) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for !condition() {
		select {
		case <-deadline:
			t.Fatal(message)
		case <-time.After(time.Millisecond):
		}
	}
}

func overviewAsync(mux http.Handler, p project.Project) (context.CancelFunc, <-chan struct{}, *httptest.ResponseRecorder) {
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/v1/product/projects/"+p.ID+"/overview", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		mux.ServeHTTP(recorder, req)
	}()
	return cancel, done, recorder
}

func overviewSourceAdmitted(s *Server, id string) bool {
	guard := s.branchGuard(id)
	if guard.TryLock() {
		guard.Unlock()
		return false
	}
	return true
}

func overviewAwait(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("overview request did not settle after fixture locks were released")
	}
}

func TestProjectOverviewCanceledWaitingReadReleasesSourceAndNeverFills(t *testing.T) {
	s, mux := overviewFixture(t)
	projects := make([]project.Project, 4)
	for index, name := range []string{"active-fill", "cancelled-one", "cancelled-two", "live-peer"} {
		projects[index] = createJourneyProject(t, mux, name)
	}
	// A owns cache fill admission and blocks in its existing bounded run reader.
	// B/C must be able to leave that wait without abandoning A or its cache fill.
	s.projectRuns.mu.Lock()
	blocked := true
	defer func() {
		if blocked {
			s.projectRuns.mu.Unlock()
		}
	}()
	cancelA, doneA, responseA := overviewAsync(mux, projects[0])
	defer cancelA()
	overviewWait(t, "active request never began its cache fill", func() bool {
		if s.projectOverview.mu.TryLock() {
			s.projectOverview.mu.Unlock()
			return false
		}
		return true
	})
	cancelB, doneB, responseB := overviewAsync(mux, projects[1])
	defer cancelB()
	cancelC, doneC, responseC := overviewAsync(mux, projects[2])
	defer cancelC()
	cancelD, doneD, responseD := overviewAsync(mux, projects[3])
	defer cancelD()
	for _, p := range projects[1:] {
		overviewWait(t, "waiting request never obtained source admission", func() bool { return overviewSourceAdmitted(s, p.ID) })
	}
	cancelB()
	cancelC()
	for _, waiter := range []struct {
		project project.Project
		done    <-chan struct{}
	}{{projects[1], doneB}, {projects[2], doneC}} {
		select {
		case <-waiter.done:
			if overviewSourceAdmitted(s, waiter.project.ID) {
				t.Errorf("cancelled waiter %s retained its source admission", waiter.project.Name)
			}
		case <-time.After(200 * time.Millisecond):
			t.Errorf("cancelled waiter %s did not leave fill admission while unrelated fill remained blocked", waiter.project.Name)
		}
	}
	select {
	case <-doneD:
		t.Error("live peer bypassed the single bounded fill")
	default:
	}
	s.projectRuns.mu.Unlock()
	blocked = false
	for _, done := range []<-chan struct{}{doneA, doneB, doneC, doneD} {
		overviewAwait(t, done)
	}
	for _, index := range []int{1, 2} {
		p := projects[index]
		if _, exists := s.projectOverview.entries[p.ID+"\x00"+p.Path]; exists {
			t.Errorf("cancelled waiter %s populated the cache after its client left", p.Name)
		}
	}
	if responseB.Body.Len() != 0 || responseC.Body.Len() != 0 {
		t.Error("cancelled display read serialized a project response")
	}
	for index, response := range []*httptest.ResponseRecorder{responseA, responseD} {
		var view ProjectOverview
		want := projects[0].ID
		if index == 1 {
			want = projects[3].ID
		}
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &view) != nil || view.ProjectID != want || view.Status != "ready" {
			t.Errorf("live project did not retain its own authorized result: status=%d project=%s", response.Code, view.ProjectID)
		}
	}
	if view := overviewGet(t, mux, projects[1].ID); view.ProjectID != projects[1].ID || view.Status != "ready" {
		t.Fatal("subsequent live request did not recover its own current context")
	}
}

func TestProjectOverviewCanceledDuringRunReadDoesNotPublishCache(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "cancel-during-bounded-stage")
	s.projectRuns.mu.Lock()
	blocked := true
	defer func() {
		if blocked {
			s.projectRuns.mu.Unlock()
		}
	}()
	cancel, done, response := overviewAsync(mux, p)
	defer cancel()
	overviewWait(t, "request never began its cache fill", func() bool {
		if s.projectOverview.mu.TryLock() {
			s.projectOverview.mu.Unlock()
			return false
		}
		return true
	})
	cancel()
	// An already-started ordinary store mutex is not forcibly interrupted. The
	// cancellation checkpoint must prevent later reads/cache publication once
	// that bounded stage can proceed.
	s.projectRuns.mu.Unlock()
	blocked = false
	overviewAwait(t, done)
	if len(s.projectOverview.entries) != 0 || response.Body.Len() != 0 {
		t.Fatal("cancelled in-progress read published a cache entry or response")
	}
	if overviewSourceAdmitted(s, p.ID) {
		t.Fatal("completed cancellation retained source admission")
	}
	if view := overviewGet(t, mux, p.ID); view.ProjectID != p.ID || view.Status != "ready" {
		t.Fatal("subsequent live read did not recover after cancellation")
	}
}
