package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func sessionPreparationRecord() registry.Session {
	s := executionGenerationFixture()
	s.Status = registry.StatusStarting
	s.Endpoint = ""
	return s
}

// Assert the optional wire field, including absence versus explicit false.
// These API values deliberately have no backend: projection owns no probe or
// execution capability, and depends only on its local preparation observation.
func assertSessionPreparationProjection(t *testing.T, a *API, s registry.Session, want string) {
	t.Helper()
	raw, err := json.Marshal(a.sessionJSON(s, sessionBasePath))
	if err != nil {
		t.Fatal(err)
	}
	var view map[string]json.RawMessage
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	got, present := view["attachReady"]
	if want == "" {
		if present {
			t.Fatalf("attachReady = %s; want omitted without a current local preparation", got)
		}
	} else if !present || string(got) != want {
		t.Fatalf("attachReady = %s (present %t), want %s", got, present, want)
	}
	var generation string
	if value, ok := view["executionGeneration"]; ok {
		if err := json.Unmarshal(value, &generation); err != nil {
			t.Fatal(err)
		}
	}
	if generation != sessionExecutionGeneration(s) {
		t.Fatal("preparation projection changed the allocation display identity")
	}
	for _, key := range []string{"endpoint", "attachToken", "sandboxId", "sandboxRef"} {
		if _, ok := view[key]; ok {
			t.Fatalf("private key %s reached the public session", key)
		}
	}
	secrets := []string{s.Endpoint, s.AttachToken, s.SandboxRef}
	// Initial Create intentionally reuses the public session ID as SandboxID;
	// its public appearance is not a disclosure of a separate private value.
	if s.SandboxID != s.ID {
		secrets = append(secrets, s.SandboxID)
	}
	for _, secret := range secrets {
		if secret != "" && bytes.Contains(raw, []byte(secret)) {
			t.Fatal("private allocation value reached the public session")
		}
	}
}

func assertSessionPreparationCount(t *testing.T, a *API, want int) {
	t.Helper()
	a.sessionPreparations.mu.Lock()
	got := len(a.sessionPreparations.entries)
	a.sessionPreparations.mu.Unlock()
	if got != want {
		t.Fatalf("retained preparations = %d, want %d", got, want)
	}
}

func TestSessionPreparationIsOwnedByAPIAndExactGeneration(t *testing.T) {
	owner, recovered := &API{}, &API{}
	s := sessionPreparationRecord()
	assertSessionPreparationProjection(t, owner, s, "")
	release := owner.beginSessionPreparation(s)
	defer release()
	assertSessionPreparationProjection(t, owner, s, "false")
	assertSessionPreparationProjection(t, recovered, s, "")
	assertSessionPreparationCount(t, owner, 1)
	assertSessionPreparationCount(t, recovered, 0)

	otherGeneration := s
	otherGeneration.SandboxID += "-replacement"
	if sessionExecutionGeneration(otherGeneration) == sessionExecutionGeneration(s) {
		t.Fatal("fixture did not change allocation generation")
	}
	assertSessionPreparationProjection(t, owner, otherGeneration, "")
	otherSession := s
	otherSession.ID += "-other"
	assertSessionPreparationProjection(t, owner, otherSession, "")

	// Once the endpoint is recorded, readiness is true even while the creating
	// handler is unwinding; another API can observe that same positive hint.
	ready := s
	ready.Endpoint = "127.0.0.1:43210"
	assertSessionPreparationProjection(t, owner, ready, "true")
	assertSessionPreparationProjection(t, recovered, ready, "true")
	terminal := ready
	terminal.Status = registry.StatusExited
	assertSessionPreparationProjection(t, owner, terminal, "")

	release()
	assertSessionPreparationCount(t, owner, 0)
	assertSessionPreparationProjection(t, owner, s, "")
}

func TestSessionPreparationStaleReleaseCannotClearReplacement(t *testing.T) {
	for _, tc := range []struct {
		name             string
		newGeneration    bool
		releaseBeforeNew bool
	}{
		{name: "new generation", newGeneration: true},
		{name: "same generation superseded entry"},
		{name: "same generation after completed release", releaseBeforeNew: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &API{}
			old := sessionPreparationRecord()
			releaseOld := a.beginSessionPreparation(old)
			defer releaseOld()
			if tc.releaseBeforeNew {
				releaseOld()
				assertSessionPreparationCount(t, a, 0)
			}
			next := old
			if tc.newGeneration {
				next.SandboxID += "-next"
			}
			releaseNext := a.beginSessionPreparation(next)
			defer releaseNext()
			assertSessionPreparationProjection(t, a, next, "false")
			if tc.newGeneration {
				assertSessionPreparationProjection(t, a, old, "")
			}
			releaseOld()
			releaseOld()
			assertSessionPreparationProjection(t, a, next, "false")
			assertSessionPreparationCount(t, a, 1)
			releaseNext()
			releaseNext()
			assertSessionPreparationCount(t, a, 0)
			assertSessionPreparationProjection(t, a, next, "")
		})
	}
}

func TestSessionPreparationMissingIdentityRetainsNoEntry(t *testing.T) {
	for name, change := range map[string]func(*registry.Session){
		"missing session":    func(s *registry.Session) { s.ID = "" },
		"missing allocation": func(s *registry.Session) { s.SandboxID = "" },
		"missing token":      func(s *registry.Session) { s.AttachToken = "" },
		"legacy token":       func(s *registry.Session) { s.AttachToken = "legacy-private" },
		"invalid token":      func(s *registry.Session) { s.AttachToken = strings.Repeat("!", 43) },
	} {
		t.Run(name, func(t *testing.T) {
			a := &API{}
			unknown := sessionPreparationRecord()
			change(&unknown)
			releaseUnknown := a.beginSessionPreparation(unknown)
			defer releaseUnknown()
			assertSessionPreparationCount(t, a, 0)
			assertSessionPreparationProjection(t, a, unknown, "")
			known := sessionPreparationRecord()
			releaseKnown := a.beginSessionPreparation(known)
			defer releaseKnown()
			releaseUnknown()
			releaseUnknown()
			assertSessionPreparationCount(t, a, 1)
			assertSessionPreparationProjection(t, a, known, "false")
			releaseKnown()
			assertSessionPreparationCount(t, a, 0)
		})
	}
}

func TestSessionPreparationFailedAllocationReleasesObservation(t *testing.T) {
	for _, restart := range []bool{false, true} {
		name := "Create"
		if restart {
			name = "Restart"
		}
		t.Run(name, func(t *testing.T) {
			env := sessionsNewEnv(t)
			target, body, previousSandbox := "/v1/sessions", sessionsCreateBody, ""
			if restart {
				before := sessionsSeed(t, env.st, sessionsStarting("s-0000000b"))
				target, body, previousSandbox = "/v1/sessions/"+before.ID+"/restart", "", before.SandboxID
			}
			env.be.createErr = backend.ErrUnavailable
			rec := env.do(t, http.MethodPost, target, body)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("failed allocation status = %d, want 503", rec.Code)
			}
			if spec := env.be.spec(t, 0); spec.ID == "" || spec.ID == previousSandbox {
				t.Fatal("failed allocation did not attempt a fresh backend Create")
			}
			assertSessionPreparationCount(t, env.api, 0)
			rows, err := env.st.List()
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0].Status != registry.StatusExited {
				t.Fatal("failed allocation did not retain one exited session")
			}
			assertSessionPreparationProjection(t, env.api, rows[0], "")
		})
	}
}
