package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func livePreviewReceipt(t *testing.T) (*Server, *http.ServeMux, storedRelease, context.Context) {
	t.Helper()
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "live-cancel-receipt")
	row := retainedPreviewFixture(t, s, p.ID)
	ctx, cancel := context.WithCancel(context.Background())
	op, err := s.beginLocalPreview(&row, cancel)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); s.finishLocalPreview(row.ID, op) })
	return s, mux, row, ctx
}

func cancelReceiptRequest(t *testing.T, mux http.Handler, row storedRelease, revision uint64, status int) *httptest.ResponseRecorder {
	t.Helper()
	return journeyRequest(t, mux, "POST", "/v1/product/projects/"+row.ProjectID+"/releases/"+row.ID+"/cancel", map[string]any{"expectedRevision": revision}, status)
}

func TestPreviewCancellationRequiresCurrentAuthorityRevisionAndOwnedIdentity(t *testing.T) {
	for _, mode := range []string{"missing-revision", "stale-revision", "revoked", "changed-allocation", "changed-source", "not-live", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			s, mux, row, ctx := livePreviewReceipt(t)
			want, revision := 409, row.Revision
			switch mode {
			case "missing-revision":
				revision = 0
				want = 400
			case "stale-revision":
				revision++
			case "revoked":
				runtime := s.PreviewRuntime()
				runtime.CaptureActor = func(context.Context, string) (management.ExecutionActorRef, error) {
					return management.ExecutionActorRef{}, errors.New("current Deploy grant revoked")
				}
				s.SetPreviewRuntime(runtime)
				want = 403
			case "changed-allocation":
				copyOutcome := *row.Runtime
				// This field was empty at begin. A permissive Continues match
				// would accept its forgery; only a published transition may bind it.
				copyOutcome.AllocationID = "foreign-build"
				next := row
				next.Runtime = &copyOutcome
				if err := s.storeRelease(next); err != nil {
					t.Fatal(err)
				}
			case "changed-source":
				copyOutcome := *row.Runtime
				copyOutcome.SourceDigest = strings.Repeat("c", 64)
				next := row
				next.Runtime = &copyOutcome
				if err := s.storeRelease(next); err != nil {
					t.Fatal(err)
				}
			case "not-live":
				s.mu.Lock()
				row.operation.sealed = true
				s.mu.Unlock()
			case "legacy":
				next := row
				next.Revision = 0
				if err := s.storeRelease(next); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.ReadFile(s.releaseFile(row.ProjectID))
			if err != nil {
				t.Fatal(err)
			}
			cancelReceiptRequest(t, mux, row, revision, want)
			after, err := os.ReadFile(s.releaseFile(row.ProjectID))
			if err != nil {
				t.Fatal(err)
			}
			if ctx.Err() != nil || !bytes.Equal(before, after) {
				t.Fatal("refused cancellation signalled live work or changed receipt")
			}
		})
	}
}

func TestPreviewCancellationMergesOnlyExactIntentAndKeepsLatestAllocation(t *testing.T) {
	for _, phase := range []string{"built", "unknown", "running"} {
		t.Run(phase, func(t *testing.T) {
			s, mux, row, ctx := livePreviewReceipt(t)
			cancelReceiptRequest(t, mux, row, row.Revision, 202)
			if ctx.Err() == nil {
				t.Fatal("owned context was not cancelled")
			}
			out := *row.Runtime
			out.Phase = phase
			if phase == "built" {
				out.ResourcesHeld = false
				out.CleanupConfirmed = true
				out.Image = "sha256:" + strings.Repeat("d", 64)
			}
			if phase == "running" {
				out.AllocationID = "acknowledged-serve"
				out.ContainerID = strings.Repeat("e", 64)
				out.Image = "sha256:" + strings.Repeat("d", 64)
			}
			if err := s.publishPreviewOutcome(&row, out, nil); err != nil {
				t.Fatal("cancel intent broke producer CAS", err)
			}
			next := row
			next.Status = "failed"
			if phase == "running" {
				next.Status = "ready"
				next.URL = "http://127.0.0.1:3000"
			}
			if err := s.storeAdmittedRelease(row, &next); err != nil {
				t.Fatal(err)
			}
			want := "unknown"
			if phase == "built" {
				want = "cancelled"
			}
			if next.Status != want || !next.CancelRequested || next.Revision != 4 || next.Runtime.Identity != out.Identity || next.Runtime.ResourcesHeld != out.ResourcesHeld || next.URL != "" {
				t.Fatalf("late result erased intent/identity or claimed cleanup: %+v", next)
			}
			foreign := next
			foreign.Revision++
			foreign.Message = "a newer unrelated outcome"
			if err := s.storeRelease(foreign); err != nil {
				t.Fatal(err)
			}
			late := next
			late.Message = "stale producer"
			if err := s.storeAdmittedRelease(next, &late); err == nil {
				t.Fatal("cancel merge accepted unrelated newer receipt")
			}
		})
	}
}

func TestPreviewCancellationNewPublishedIdentityReplacesInitialBinding(t *testing.T) {
	s, mux, row, ctx := livePreviewReceipt(t)
	out := *row.Runtime
	out.AllocationID = "new-serving-allocation"
	out.ContainerID = strings.Repeat("d", 64)
	out.Image = "sha256:" + strings.Repeat("e", 64)
	out.Phase = "running"
	if err := s.publishPreviewOutcome(&row, out, nil); err != nil {
		t.Fatal(err)
	}
	if !s.browserRelease(row).CanCancel {
		t.Fatal("live latest exact allocation lost cancellation")
	}
	cancelReceiptRequest(t, mux, row, row.Revision, 202)
	if ctx.Err() == nil {
		t.Fatal("latest owned allocation did not cancel")
	}
}

func TestPreviewCancellationWinsBeforeFinalReadyReceiptWithoutClaimingCreated(t *testing.T) {
	s, mux, row, _ := livePreviewReceipt(t)
	out := *row.Runtime
	out.AllocationID = "acknowledged-health-serve"
	out.ContainerID = strings.Repeat("d", 64)
	out.Image = "sha256:" + strings.Repeat("e", 64)
	out.Phase = "running"
	if err := s.publishPreviewOutcome(&row, out, nil); err != nil {
		t.Fatal(err)
	}
	// Health has succeeded; cancel wins immediately before the final ready CAS.
	cancelReceiptRequest(t, mux, row, row.Revision, 202)
	w := httptest.NewRecorder()
	s.completeLocalPreview(w, &row, "http://127.0.0.1:3000")
	if w.Code != 422 || row.Status != "unknown" || !row.CancelRequested || row.URL != "" || !row.Runtime.ResourcesHeld || row.Runtime.ContainerID != out.ContainerID || !s.browserRelease(row).CanStop {
		t.Fatal("late cancel claimed ready/cleanup or lost exact Stop", w.Code, w.Body.String())
	}
}

func TestPreviewCancellationRouteWorksWithExclusiveSourceGuardHeld(t *testing.T) {
	s, mux, row, ctx := livePreviewReceipt(t)
	guard := s.branchGuard(row.ProjectID)
	guard.Lock()
	defer guard.Unlock()
	// The actual registrar must not seek shared source admission for cancel.
	cancelReceiptRequest(t, mux, row, row.Revision, 202)
	if ctx.Err() == nil {
		t.Fatal("private control did not reach live context")
	}
}

func TestPreviewCancellationRevokedRequestCannotSignalAfterMutexWait(t *testing.T) {
	s, mux, row, live := livePreviewReceipt(t)
	authorized := make(chan struct{})
	runtime := s.PreviewRuntime()
	runtime.CaptureActor = func(context.Context, string) (management.ExecutionActorRef, error) {
		close(authorized)
		return management.ExecutionActorRef{Local: true}, nil
	}
	s.SetPreviewRuntime(runtime)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body, _ := json.Marshal(map[string]any{"expectedRevision": row.Revision})
	r := httptest.NewRequest("POST", "/v1/product/projects/"+row.ProjectID+"/releases/"+row.ID+"/cancel", bytes.NewReader(body)).WithContext(ctx)
	w := httptest.NewRecorder()
	done := make(chan struct{})
	// requestProject precedes CaptureActor and takes s.mu, so capture first,
	// then hold the mutex from the actor callback until revocation arrives.
	proceed := make(chan struct{})
	runtime.CaptureActor = func(context.Context, string) (management.ExecutionActorRef, error) {
		s.mu.Lock()
		close(authorized)
		<-proceed
		return management.ExecutionActorRef{Local: true}, nil
	}
	s.SetPreviewRuntime(runtime)
	go func() { mux.ServeHTTP(w, r); close(done) }()
	runtimeSignal(t, authorized)
	cancel()
	close(proceed)
	s.mu.Unlock()
	runtimeSignal(t, done)
	if w.Code != 403 || live.Err() != nil {
		t.Fatal("revoked waiting control still cancelled work", w.Code)
	}
	rows, _ := s.readReleases(row.ProjectID)
	if rows[0].Revision != row.Revision || rows[0].CancelRequested {
		t.Fatal("revoked request changed intent")
	}
}

func TestPreviewCancellationFreshGrantAfterMutexWaitWithoutContextRevocation(t *testing.T) {
	s, mux, row, live := livePreviewReceipt(t)
	authorized, proceed, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	var revoked atomic.Bool
	runtime := s.PreviewRuntime()
	runtime.CaptureActor = func(context.Context, string) (management.ExecutionActorRef, error) {
		if calls.Add(1) == 1 {
			s.mu.Lock()
			close(authorized)
			<-proceed
			return management.ExecutionActorRef{Local: true}, nil
		}
		if revoked.Load() {
			return management.ExecutionActorRef{}, errors.New("current file-backed Deploy grant revoked")
		}
		return management.ExecutionActorRef{Local: true}, nil
	}
	s.SetPreviewRuntime(runtime)
	body, _ := json.Marshal(map[string]any{"expectedRevision": row.Revision})
	r := httptest.NewRequest("POST", "/v1/product/projects/"+row.ProjectID+"/releases/"+row.ID+"/cancel", bytes.NewReader(body))
	w := httptest.NewRecorder()
	go func() { mux.ServeHTTP(w, r); close(done) }()
	runtimeSignal(t, authorized)
	// Gateway-to-worker disconnect propagation can lag the authoritative grant
	// update. Keep the request context live and require a new authority read.
	revoked.Store(true)
	close(proceed)
	s.mu.Unlock()
	runtimeSignal(t, done)
	if r.Context().Err() != nil || w.Code != 403 || live.Err() != nil || calls.Load() != 2 {
		t.Fatal("stale successful authority cancelled live preview", w.Code, calls.Load())
	}
	rows, _ := s.readReleases(row.ProjectID)
	if rows[0].Revision != row.Revision || rows[0].CancelRequested {
		t.Fatal("revoked control changed durable receipt")
	}
}

func TestPreviewCloseDeadlineSealsLatePublicationAndDoesNotReactivateOnReopen(t *testing.T) {
	s, mux, row, live := livePreviewReceipt(t)
	join, cancel := context.WithCancel(context.Background())
	cancel()
	s.closeLocalPreviews(join)
	if live.Err() == nil {
		t.Fatal("bounded close did not cancel owned context")
	}
	before, err := os.ReadFile(s.releaseFile(row.ProjectID))
	if err != nil {
		t.Fatal(err)
	}
	out := *row.Runtime
	out.Phase = "finished"
	out.ResourcesHeld = false
	out.CleanupConfirmed = true
	if err := s.publishPreviewOutcome(&row, out, nil); err == nil {
		t.Fatal("sealed late publication accepted")
	}
	after, _ := os.ReadFile(s.releaseFile(row.ProjectID))
	if !bytes.Equal(before, after) {
		t.Fatal("late publication rewrote retained uncertainty")
	}
	cancelReceiptRequest(t, mux, row, row.Revision, 409)
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	rows, err := reopened.readReleases(row.ProjectID)
	if err != nil || len(rows) != 1 || rows[0].Status != "unknown" || !rows[0].Runtime.ResourcesHeld || reopened.browserRelease(rows[0]).CanCancel {
		t.Fatal("reopen invented live control or cleanup", rows, err)
	}
	var calls atomic.Int32
	next := row
	next.ID = "another-release"
	next.Runtime = &previewruntime.Outcome{Identity: previewruntime.Identity{ProjectID: row.ProjectID, ReleaseID: next.ID}}
	if _, err := s.beginLocalPreview(&next, func() { calls.Add(1) }); err == nil || calls.Load() != 0 {
		t.Fatal("closed server admitted new operation")
	}
}
