package product

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type delayedCheckResponse struct {
	*httptest.ResponseRecorder
	headerWritten chan struct{}
	resume        chan struct{}
}

func (w *delayedCheckResponse) WriteHeader(status int) {
	w.ResponseRecorder.WriteHeader(status)
	close(w.headerWritten)
	<-w.resume
}

func TestCheckCancelResponseRetainsItsSnapshotWhileWorkerFinishes(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	p := createJourneyProject(t, mux, "immutable-check")
	base := "/v1/product/projects/" + p.ID + "/checks"
	started := make(chan struct{})
	original := s.runner
	s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
		if name != "docker" {
			return original(ctx, name, args, dir)
		}
		if args[0] == "rm" {
			return "", nil
		}
		close(started)
		<-ctx.Done()
		return "worker output after the cancellation snapshot", ctx.Err()
	}
	body := journeyRequest(t, mux, http.MethodPost, base, map[string]any{
		"commands":  []CheckCommand{{"Wait", "node", []string{"check.js"}}},
		"execution": "docker", "acknowledgeHostExecution": true,
	}, http.StatusAccepted)
	var initial ProjectCheckRun
	if err := json.Unmarshal(body.Body.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("check did not start")
	}
	w := &delayedCheckResponse{ResponseRecorder: httptest.NewRecorder(), headerWritten: make(chan struct{}), resume: make(chan struct{})}
	var release sync.Once
	unblock := func() { release.Do(func() { close(w.resume) }) }
	t.Cleanup(unblock)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		r := httptest.NewRequest(http.MethodPost, base+"/"+initial.ID+"/cancel", strings.NewReader(`{}`))
		mux.ServeHTTP(w, r)
	}()
	select {
	case <-w.headerWritten:
	case <-time.After(10 * time.Second):
		t.Fatal("cancel response did not reach encoding")
	}
	final := waitProjectCheck(t, s, p.ID, initial.ID)
	if final.Status != "cancelled" || final.Commands[0].ExitCode == nil || final.Commands[0].Output == "" {
		t.Fatalf("worker did not finish while the response was delayed: %#v", final)
	}
	unblock()
	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		t.Fatal("cancel response remained blocked")
	}
	var snapshot ProjectCheckRun
	if err := json.Unmarshal(w.Body.Bytes(), &snapshot); err != nil || w.Code != http.StatusAccepted {
		t.Fatalf("cancel response: %d %s %v", w.Code, w.Body.String(), err)
	}
	if snapshot.Status != "running" || snapshot.Commands[0].Status != "running" || snapshot.Commands[0].ExitCode != nil || snapshot.Commands[0].Output != "" {
		t.Fatalf("cancel response changed after its snapshot was taken: %#v", snapshot)
	}
}
