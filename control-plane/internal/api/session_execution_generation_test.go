package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func executionGenerationFixture() registry.Session {
	s := sessionsFullRecord()
	s.AttachToken = base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xa7}, 32))
	s.SandboxID = "private-allocation-generation-a"
	return s
}

func projectedExecutionGeneration(t *testing.T, s registry.Session) (string, string) {
	t.Helper()
	raw, err := json.Marshal(toJSON(s, sessionBasePath))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	var generation string
	if value, ok := decoded["executionGeneration"]; ok {
		if err := json.Unmarshal(value, &generation); err != nil {
			t.Fatal(err)
		}
	}
	return generation, string(raw)
}

func TestSessionExecutionGenerationIsOpaqueStableAndAllocationBound(t *testing.T) {
	original := executionGenerationFixture()
	generation, raw := projectedExecutionGeneration(t, original)
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(generation) {
		t.Fatalf("missing opaque generation: %q", generation)
	}
	for _, secret := range []string{original.AttachToken, original.SandboxID, original.SandboxRef, original.Endpoint} {
		if bytes.Contains([]byte(raw), []byte(secret)) {
			t.Fatal("private allocation input reached session JSON")
		}
	}
	for _, key := range []string{`"attachToken"`, `"sandboxId"`, `"sandboxRef"`, `"endpoint"`} {
		if bytes.Contains([]byte(raw), []byte(key)) {
			t.Fatalf("private key %s reached session JSON", key)
		}
	}
	for name, change := range map[string]func(*registry.Session){
		"status":               func(s *registry.Session) { s.Status = registry.StatusExited; s.ExitReason = "exit status 17" },
		"output":               func(s *registry.Session) { s.LastOutputAt = s.LastOutputAt.Add(time.Second) },
		"poll-updates":         func(s *registry.Session) { s.UpdatedAt = s.UpdatedAt.Add(time.Second); s.AttachCount++ },
		"private-endpoint-ref": func(s *registry.Session) { s.Endpoint = "127.0.0.1:43210"; s.SandboxRef = "private-later-ref" },
	} {
		t.Run(name, func(t *testing.T) {
			s := original
			change(&s)
			got, _ := projectedExecutionGeneration(t, s)
			if got != generation {
				t.Fatal("ordinary update changed allocation display identity")
			}
		})
	}
	for name, change := range map[string]func(*registry.Session){
		"allocation": func(s *registry.Session) { s.SandboxID = "private-allocation-generation-b" },
		"restart-token": func(s *registry.Session) {
			s.AttachToken = base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xb8}, 32))
		},
		"session-alias": func(s *registry.Session) { s.ID = "different-session" },
	} {
		t.Run(name, func(t *testing.T) {
			s := original
			change(&s)
			got, _ := projectedExecutionGeneration(t, s)
			if got == "" || got == generation {
				t.Fatal("different allocation/session aliased generation")
			}
		})
	}
	// Delimited JSON input prevents concatenation aliases between session and allocation.
	left, right := original, original
	left.ID, left.SandboxID = "ab", "c"
	right.ID, right.SandboxID = "a", "bc"
	a, _ := projectedExecutionGeneration(t, left)
	b, _ := projectedExecutionGeneration(t, right)
	if a == b {
		t.Fatal("ambiguous session/allocation input")
	}
}

func TestSessionExecutionGenerationOmitsUnavailableLegacyOrErasedSecrets(t *testing.T) {
	for name, change := range map[string]func(*registry.Session){
		"missing-token":      func(s *registry.Session) { s.AttachToken = "" },
		"legacy-short-token": func(s *registry.Session) { s.AttachToken = "legacy-private" },
		"invalid-token":      func(s *registry.Session) { s.AttachToken = string(bytes.Repeat([]byte{'!'}, 43)) },
		"missing-allocation": func(s *registry.Session) { s.SandboxID = "" },
		"missing-session":    func(s *registry.Session) { s.ID = "" },
		"cleaned-terminal":   func(s *registry.Session) { s.Status = registry.StatusExited; s.AttachToken = ""; s.Endpoint = "" },
	} {
		t.Run(name, func(t *testing.T) {
			s := executionGenerationFixture()
			change(&s)
			generation, raw := projectedExecutionGeneration(t, s)
			if generation != "" || bytes.Contains([]byte(raw), []byte(`"executionGeneration"`)) {
				t.Fatal("unknown generation must be omitted")
			}
		})
	}
}
