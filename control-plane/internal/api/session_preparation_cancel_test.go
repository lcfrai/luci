package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSessionPreparationCallerCancellationRetainsUntilCreateSettles(t *testing.T) {
	be := newAbandonedCreate()
	st := sessionsMustStore(t)
	env := sessionsNewEnvOverWith(t, t.TempDir(), st, st, func(d *Deps) {
		d.Backend = be
		d.ReadyTimeout = 10 * time.Second
	})
	sessionsStubReady(t, nil)
	watch := sessionsWatchDone(t)
	sessionsSeed(t, st, sessionsStarting("s-0000000b"))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	request := httptest.NewRequest(http.MethodPost, sessionsTarget("/v1/sessions/s-0000000b/restart"), nil).WithContext(ctx)
	response := httptest.NewRecorder()
	served := make(chan struct{})
	var release sync.Once
	go func() {
		defer close(served)
		env.mux.ServeHTTP(response, request)
	}()
	t.Cleanup(func() {
		release.Do(func() { close(be.hold) })
		select {
		case <-served:
			if response.Code == http.StatusOK {
				select {
				case <-watch:
				case <-time.After(5 * time.Second):
					t.Error("readiness watch did not settle")
				}
			}
		case <-time.After(20 * time.Second):
			t.Error("canceled caller's Create did not settle after release")
		}
	})
	select {
	case <-be.entered:
	case <-time.After(15 * time.Second):
		t.Fatal("Restart did not enter Create")
	}
	cancel()
	row, err := st.Get("s-0000000b")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(env.api.sessionJSON(row, sessionBasePath))
	if err != nil {
		t.Fatal(err)
	}
	requireAttachReadiness(t, attachReadinessObject(t, raw, row), false)
	release.Do(func() { close(be.hold) })
	select {
	case <-served:
	case <-time.After(15 * time.Second):
		t.Fatal("Restart did not finish after release")
	}
	if response.Code != http.StatusOK || be.contextError() != nil {
		t.Fatal("caller cancellation incorrectly ended the allocation")
	}
	env.api.sessionPreparations.mu.Lock()
	remaining := len(env.api.sessionPreparations.entries)
	env.api.sessionPreparations.mu.Unlock()
	if remaining != 0 {
		t.Fatal("settled allocation leaked preparation tracking after caller cancellation")
	}
}
