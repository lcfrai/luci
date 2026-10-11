package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

type previewCancelProbe struct {
	s         *Server
	mux       *http.ServeMux
	projectID string
	row       storedRelease
	cancelled chan struct{}
	finish    func()
	done      chan *httptest.ResponseRecorder
	calls     atomic.Int32
}

func heldPreviewCancellation(t *testing.T) *previewCancelProbe {
	t.Helper()
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "cancel-preview")
	if err := os.WriteFile(filepath.Join(p.Path, "Dockerfile"), []byte("FROM scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	reviewDeploymentFixture(t, mux, p.ID)
	commitPreviewFixture(t, p.Path)
	entered := make(chan struct{})
	finish := make(chan struct{})
	var once sync.Once
	probe := &previewCancelProbe{s: s, mux: mux, projectID: p.ID, cancelled: make(chan struct{}), done: make(chan *httptest.ResponseRecorder, 1), finish: func() { once.Do(func() { close(finish) }) }}
	runtime := s.PreviewRuntime()
	runtime.Start = func(ctx context.Context, req previewruntime.Request, publish func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
		probe.calls.Add(1)
		id := req.Identity
		id.BuildAllocationID = "cancel-build"
		id.AllocationID = "cancel-serve"
		id.EngineID = "cancel-engine"
		id.InstanceID = "cancel-instance"
		out := previewruntime.Outcome{Identity: id, Phase: "building", ResourcesHeld: true, Message: "Controlled held build"}
		if err := publish(out); err != nil {
			return out, err
		}
		close(entered)
		select {
		case <-ctx.Done():
			close(probe.cancelled)
		case <-finish:
		}
		<-finish
		out.Phase = "unknown"
		out.Message = "Build acknowledgement remains unknown."
		return out, errors.New("controlled unacknowledged build")
	}
	runtime.Stop = func(context.Context, previewruntime.Identity, func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
		t.Error("cancel reached runtime Stop")
		return previewruntime.Outcome{}, errors.New("forbidden")
	}
	s.SetPreviewRuntime(runtime)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		probe.finish()
		select {
		case <-probe.done:
		case <-time.After(3 * time.Second):
			t.Error("held preview request did not finish")
		}
	})
	go func() {
		r := httptest.NewRequest("POST", "/v1/product/projects/"+p.ID+"/releases", strings.NewReader(`{"target":"local"}`)).WithContext(ctx)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		probe.done <- w
	}()
	runtimeSignal(t, entered)
	s.mu.RLock()
	rows, err := s.readReleases(p.ID)
	s.mu.RUnlock()
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	probe.row = rows[0]
	return probe
}

func TestPreviewCancelLiveRouteRetainsUnknownAndDoesNotReplay(t *testing.T) {
	p := heldPreviewCancellation(t)
	path := "/v1/product/projects/" + p.projectID + "/releases/" + p.row.ID + "/cancel"
	w := journeyRequest(t, p.mux, "POST", path, map[string]any{"expectedRevision": p.row.Revision}, 202)
	var receipt struct {
		Revision        uint64 `json:"revision"`
		CanCancel       bool   `json:"canCancel"`
		CancelRequested bool   `json:"cancelRequested"`
		Status          string `json:"status"`
	}
	if json.Unmarshal(w.Body.Bytes(), &receipt) != nil || !receipt.CancelRequested || receipt.CanCancel || receipt.Revision != p.row.Revision+1 || receipt.Status != "cancelling" {
		t.Fatal("cancel acceptance not exact pending intent", w.Body.String())
	}
	runtimeSignal(t, p.cancelled)
	journeyRequest(t, p.mux, "POST", path, map[string]any{"expectedRevision": p.row.Revision}, 200)
	p.finish()
	var result *httptest.ResponseRecorder
	select {
	case result = <-p.done:
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled request did not persist")
	}
	p.done <- result
	rows, err := p.s.readReleases(p.projectID)
	if err != nil || len(rows) != 1 || rows[0].Status != "unknown" || rows[0].Runtime == nil || !rows[0].Runtime.ResourcesHeld || rows[0].Runtime.CleanupConfirmed || rows[0].Runtime.Identity != p.row.Runtime.Identity {
		t.Fatal("cancellation invented settlement", rows, err)
	}
	reopened, err := New(Options{StateDir: p.s.stateDir, ProjectsDir: p.s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	history, err := reopened.readReleases(p.projectID)
	if err != nil || len(history) != 1 || !history[0].Runtime.ResourcesHeld || p.calls.Load() != 1 {
		t.Fatal("reopen lost or replayed unknown", history, err)
	}
}

func TestPreviewCloseCancelsOwnedBuildAndJoinsFinalPublication(t *testing.T) {
	p := heldPreviewCancellation(t)
	closed := make(chan struct{})
	go func() { p.s.Close(); close(closed) }()
	select {
	case <-closed:
		t.Fatal("Close returned without cancelling its owned preview")
	case <-p.cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not cancel preview")
	}
	p.finish()
	runtimeSignal(t, closed)
	rows, err := p.s.readReleases(p.projectID)
	if err != nil || len(rows) != 1 || rows[0].Status != "unknown" || rows[0].Runtime == nil || !rows[0].Runtime.ResourcesHeld {
		t.Fatal("Close did not join final unknown publication", rows, err)
	}
}
