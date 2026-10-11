package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// This regression deliberately references no preparation-specific API member or
// new sessionJSON field. The same public-route test can exercise the original,
// unsafe persisted-endpoint-only, and corrected receiver-local projections.
func TestSessionPreparationRecoveryAllowsBackendResolvedAttach(t *testing.T) {
	token := executionGenerationFixture().AttachToken
	f := attachNewFixture(t, nil, func(s *registry.Session) {
		s.Status = registry.StatusStarting
		s.Endpoint = ""
		s.AttachToken = token
	})
	stored, err := f.store.Get(f.id)
	if err != nil {
		t.Fatal(err)
	}
	generation := sessionExecutionGeneration(stored)
	if len(generation) != 64 {
		t.Fatal("recovery fixture has no known allocation generation")
	}
	srv := httptest.NewServer(f.api)
	t.Cleanup(srv.Close)
	client := &http.Client{Timeout: 10 * time.Second}
	assertRecoveredProjection := func() {
		t.Helper()
		res, err := client.Get(srv.URL + "/v1/sessions/" + f.id)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Fatalf("recovered session GET status = %d, want 200", res.StatusCode)
		}
		var view map[string]json.RawMessage
		if err := json.NewDecoder(res.Body).Decode(&view); err != nil {
			t.Fatal(err)
		}
		if _, present := view["attachReady"]; present {
			t.Fatal("recovered API inferred readiness from an absent persisted endpoint")
		}
		var projectedGeneration string
		if err := json.Unmarshal(view["executionGeneration"], &projectedGeneration); err != nil || projectedGeneration != generation {
			t.Fatal("recovered API lost the known allocation generation")
		}
		for _, key := range []string{"endpoint", "attachToken", "sandboxId", "sandboxRef"} {
			if _, present := view[key]; present {
				t.Fatalf("private key %s reached recovered session JSON", key)
			}
		}
	}
	// The persisted row is present when this API starts, with no current local
	// Create observation. Its missing endpoint must preserve attach recovery.
	assertRecoveredProjection()

	// attachNewFixture's original upstream intentionally uses a legacy short
	// token. This authenticated echo endpoint accepts the canonical test token,
	// so the actual Backend.Attach path and a known generation are both used.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/attach" || r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.CloseNow()
		c.SetReadLimit(backend.MaxMessageBytes)
		for {
			kind, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if err := c.Write(r.Context(), kind, data); err != nil {
				return
			}
		}
	}))
	t.Cleanup(upstream.Close)
	f.be.sb.Endpoint = strings.TrimPrefix(upstream.URL, "http://")
	f.be.sb.State = backend.StateStarting

	// Readiness omission cannot bypass the existing local transport guards.
	// Management project permissions are exercised by their own router tests.
	writesBefore := f.store.writes.Load()
	for _, tc := range []struct {
		name, origin, host string
		status             int
		code               string
	}{
		{name: "origin", origin: "http://evil.example", status: http.StatusForbidden, code: httpx.CodeForbiddenOrigin},
		{name: "host", host: "attacker.example:8730", status: http.StatusForbidden, code: httpx.CodeForbiddenHost},
		{name: "upgrade", status: http.StatusUpgradeRequired, code: httpx.CodeUpgradeRequired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, problem := f.plainGET(t, "/v1/sessions/"+f.id+"/attach", tc.origin, tc.host)
			if status != tc.status || problem.Error != tc.code {
				t.Fatalf("attach guard = %d/%s, want %d/%s", status, problem.Error, tc.status, tc.code)
			}
		})
	}
	if f.store.writes.Load() != writesBefore || f.be.attaches.Load() != 0 {
		t.Fatal("rejected attach wrote state or dialed the backend")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/sessions/"+f.id+"/attach", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	c.SetReadLimit(backend.MaxMessageBytes)
	want := []byte{0x01, 'r', 'e', 'c', 'o', 'v', 'e', 'r'}
	if err := c.Write(ctx, websocket.MessageBinary, want); err != nil {
		t.Fatal(err)
	}
	kind, got, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if kind != websocket.MessageBinary || !bytes.Equal(got, want) || f.be.attaches.Load() != 1 {
		t.Fatal("backend-resolved attach did not complete the authenticated binary relay")
	}
	after, err := f.store.Get(f.id)
	if err != nil {
		t.Fatal(err)
	}
	if after.Endpoint != "" || sessionExecutionGeneration(after) != generation {
		t.Fatal("attach manufactured a persisted endpoint or changed allocation identity")
	}
	assertRecoveredProjection()
	if f.be.destroyCount() != 0 {
		t.Fatal("attach observation destroyed an allocation")
	}
}
