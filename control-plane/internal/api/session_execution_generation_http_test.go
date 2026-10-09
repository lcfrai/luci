package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestSessionExecutionGenerationChangesOnHTTPRestartButSurvivesPolling(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	seed := sessionsStarting("s-0000000b")
	seed.AttachToken = executionGenerationFixture().AttachToken
	before := sessionsSeed(t, env.st, seed)
	wantBefore, _ := projectedExecutionGeneration(t, before)
	if wantBefore == "" {
		t.Fatal("fresh fixture has no generation")
	}
	initial := sessionsDecodeOne(t, env.do(t, http.MethodGet, "/v1/sessions/s-0000000b", ""), http.StatusOK)
	if initial.ExecutionGeneration != wantBefore {
		t.Fatal("GET lost initial allocation fingerprint")
	}
	restarted := sessionsDecodeOne(t, env.do(t, http.MethodPost, "/v1/sessions/s-0000000b/restart", ""), http.StatusOK)
	if restarted.ID != initial.ID || restarted.ExecutionGeneration == "" || restarted.ExecutionGeneration == initial.ExecutionGeneration {
		t.Fatal("same-ID restart did not publish a distinct allocation fingerprint")
	}
	sessionsAwaitWatch(t, done)
	stored, err := env.st.Get(initial.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantAfter, _ := projectedExecutionGeneration(t, stored)
	response := env.do(t, http.MethodGet, "/v1/sessions/s-0000000b", "")
	polled := sessionsDecodeOne(t, response, http.StatusOK)
	if polled.ExecutionGeneration != wantAfter || polled.ExecutionGeneration != restarted.ExecutionGeneration {
		t.Fatal("ordinary readiness update/poll changed allocation fingerprint")
	}
	for _, secret := range []string{stored.AttachToken, stored.SandboxID, stored.SandboxRef, stored.Endpoint} {
		if secret != "" && strings.Contains(response.Body.String(), secret) {
			t.Fatal("GET leaked private execution identity")
		}
	}
}
