package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Decode the public wire shape, keeping this regression buildable against the
// original source where sessionJSON has no AttachReady field.
func attachReadinessObject(t *testing.T, raw []byte, private registry.Session) map[string]json.RawMessage {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal("session response is not a JSON object")
	}
	for _, key := range []string{"attachToken", "sandboxId", "sandboxRef", "endpoint", "command", "args"} {
		if _, exists := fields[key]; exists {
			t.Fatal("private allocation key reached session response")
		}
	}
	for _, secret := range []string{private.AttachToken, private.SandboxID, private.SandboxRef, private.Endpoint} {
		if secret != "" && bytes.Contains(raw, []byte(secret)) {
			t.Fatal("private allocation value reached session response")
		}
	}
	return fields
}

func requireAttachReadiness(t *testing.T, fields map[string]json.RawMessage, want bool) {
	t.Helper()
	value, exists := fields["attachReady"]
	if !exists {
		t.Fatal("current allocation is missing attachReady")
	}
	var got bool
	if err := json.Unmarshal(value, &got); err != nil || string(value) == "null" || got != want {
		t.Fatal("attachReady does not match the recorded allocation endpoint")
	}
}

func TestSessionAttachReadinessHTTPRestartWaitsForRecordedEndpoint(t *testing.T) {
	be := newAbandonedCreate()
	st := sessionsMustStore(t)
	env := sessionsNewEnvOverWith(t, t.TempDir(), st, st, func(d *Deps) {
		d.Backend = be
		d.ReadyTimeout = 10 * time.Second
	})
	prior := sessionsStarting("s-0000000b")
	prior.Status = registry.StatusExited
	prior.AttachToken = executionGenerationFixture().AttachToken
	prior.Endpoint = "127.0.0.1:49199"
	sessionsSeed(t, st, prior)
	oldGeneration := sessionExecutionGeneration(prior)
	if oldGeneration == "" {
		t.Fatal("fixture needs a known prior generation")
	}

	// Keep health pending independently of Create. Readiness must become true
	// as soon as the endpoint is recorded, while status is still starting.
	healthHold := make(chan struct{})
	previousReady := sessionWaitReady
	sessionWaitReady = func(ctx context.Context, _ string, _ time.Duration) error {
		select {
		case <-healthHold:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	t.Cleanup(func() { sessionWaitReady = previousReady })
	watch := sessionsWatchDone(t)
	server := httptest.NewServer(env.mux)
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 15 * time.Second}
	type reply struct {
		code int
		raw  []byte
		err  error
	}
	posted := make(chan struct{})
	var result reply
	go func() {
		defer close(posted)
		response, err := client.Post(server.URL+"/v1/sessions/"+prior.ID+"/restart", "application/json", nil)
		if err != nil {
			result.err = err
			return
		}
		defer response.Body.Close()
		result.code = response.StatusCode
		result.raw, result.err = io.ReadAll(io.LimitReader(response.Body, 16<<10))
	}()
	var releaseCreate, releaseHealth sync.Once
	t.Cleanup(func() {
		releaseCreate.Do(func() { close(be.hold) })
		releaseHealth.Do(func() { close(healthHold) })
		select {
		case <-posted:
			if result.err == nil && result.code == http.StatusOK {
				select {
				case <-watch:
				case <-time.After(5 * time.Second):
					t.Error("readiness watch did not settle")
				}
			}
		case <-time.After(20 * time.Second):
			t.Error("Restart request did not settle after release")
		}
	})
	select {
	case <-be.entered:
	case <-time.After(15 * time.Second):
		t.Fatal("Restart did not reach the controlled backend Create")
	}
	get := func() (map[string]json.RawMessage, registry.Session) {
		t.Helper()
		response, err := client.Get(server.URL + "/v1/sessions/" + prior.ID)
		if err != nil {
			t.Fatal("session GET failed")
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(response.Body, 16<<10))
		if err != nil || response.StatusCode != http.StatusOK {
			t.Fatal("session GET did not return a complete successful response")
		}
		current, err := st.Get(prior.ID)
		if err != nil {
			t.Fatal(err)
		}
		return attachReadinessObject(t, raw, current), current
	}
	before, pending := get()
	if pending.Status != registry.StatusStarting || pending.Endpoint != "" || pending.SandboxID == prior.SandboxID {
		t.Fatal("Create barrier did not hold the new allocation before its endpoint write")
	}
	var generation string
	if err := json.Unmarshal(before["executionGeneration"], &generation); err != nil || generation == "" || generation == oldGeneration {
		t.Fatal("pending Restart lacks a distinct current generation")
	}
	requireAttachReadiness(t, before, false)
	select {
	case <-posted:
		t.Fatal("Restart acknowledged before Create was released")
	default:
	}
	releaseCreate.Do(func() { close(be.hold) })
	select {
	case <-posted:
	case <-time.After(15 * time.Second):
		t.Fatal("Restart did not acknowledge after Create returned")
	}
	if result.err != nil || result.code != http.StatusOK {
		t.Fatal("Restart did not return a successful complete response")
	}
	after, allocated := get()
	if allocated.Status != registry.StatusStarting || allocated.Endpoint == "" || allocated.SandboxID != pending.SandboxID {
		t.Fatal("successful Create did not publish the same allocation endpoint before health completion")
	}
	ack := attachReadinessObject(t, result.raw, allocated)
	for _, fields := range []map[string]json.RawMessage{after, ack} {
		requireAttachReadiness(t, fields, true)
		var got string
		if err := json.Unmarshal(fields["executionGeneration"], &got); err != nil || got != generation {
			t.Fatal("endpoint publication changed the allocation generation")
		}
		if string(fields["status"]) != `"starting"` {
			t.Fatal("test did not preserve the starting attach contract")
		}
	}
}
