package e2bbe

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func saveFixtureGrant(t *testing.T, c *Controller, r Record) {
	t.Helper()
	if e := c.saveGrant(r, providerSandbox{EnvdAccessToken: "fixture-envd", TrafficAccessToken: "fixture-traffic", EnvdVersion: EnvdVersion}); e != nil {
		t.Fatal(e)
	}
}
func TestPrivateGuestGrantSurvivesControllerRestartButNeverEntersCheckpoint(t *testing.T) {
	p := newProvider(t)
	p.guestGrant = true
	cfg, sink := fixtureConfig(t, p)
	cfg.GuestOrigins = func(string) (string, string, string, error) { return p.server.URL, p.server.URL, EnvdVersion, nil }
	c := openController(t, cfg)
	r, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	c = openController(t, cfg)
	g, e := c.Guest(context.Background(), r.Request.Identity.OperationID)
	if e != nil {
		t.Fatal(e)
	}
	g.Close()
	b, e := os.ReadFile(filepath.Join(cfg.StateDir, "attempts.json"))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(b), "fixture-envd") || strings.Contains(string(b), "fixture-traffic") {
		t.Fatal("journal contains private grant")
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	for _, row := range sink.rows {
		if strings.Contains(string(row), "fixture-envd") || strings.Contains(string(row), "fixture-traffic") {
			t.Fatal("checkpoint contains private grant")
		}
	}
}

func TestInvalidGrantAfterAcknowledgedCreateIsUnknownAndRetainsExactVMCharge(t *testing.T) {
	p := newProvider(t)
	p.guestGrant = true
	p.invalidGuestGrant = true
	cfg, _ := fixtureConfig(t, p)
	c := openController(t, cfg)
	r, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if !errors.Is(e, ErrUnknown) || r.SandboxID != "vm-1" || !r.ResourcesHeld || r.State != Unknown {
		t.Fatalf("acknowledged VM cannot be a no-effect unsupported refusal: state=%s id=%s held=%v err=%v", r.State, r.SandboxID, r.ResourcesHeld, e)
	}
	if _, e = c.Allocate(context.Background(), r.Request); !errors.Is(e, ErrUnknown) {
		t.Fatal(e)
	}
	_, u := c.Snapshot()
	if u.Active != 1 || p.creates != 1 || p.deletes != 0 {
		t.Fatal("unknown allocation was replayed or released", u, p.creates, p.deletes)
	}
	current, e := c.Reconcile(context.Background(), r.Request.Identity.OperationID)
	if e != nil {
		t.Fatal(e)
	}
	after, e := c.Cancel(context.Background(), r.Request.Identity.OperationID, current.Revision)
	requireAbsent(t, after, e)
}
func TestRestoredMissingGuestGrantDoesNotCreateOrImplicitlyResume(t *testing.T) {
	p := newProvider(t)
	cfg, _ := fixtureConfig(t, p)
	s := openStore(t, storeConfig(t))
	cfg.Checkpoint = s
	cfg.GuestOrigins = func(string) (string, string, string, error) { return p.server.URL, p.server.URL, EnvdVersion, nil }
	c := openController(t, cfg)
	r, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
	if e != nil {
		t.Fatal(e)
	}
	saveFixtureGrant(t, c, r)
	head, e := s.Head(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if e = c.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	cfg.StateDir = filepath.Join(t.TempDir(), "restored")
	restored, e := s.RestoreController(context.Background(), cfg, head)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close(context.Background())
	if _, e = restored.Guest(context.Background(), r.Request.Identity.OperationID); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	if _, e = restored.Reconcile(context.Background(), r.Request.Identity.OperationID); e != nil {
		t.Fatal(e)
	}
	current, _ := restored.record(r.Request.Identity.OperationID)
	after, e := restored.Cancel(context.Background(), r.Request.Identity.OperationID, current.Revision)
	requireAbsent(t, after, e)
	if p.creates != 1 || p.deletes != 1 {
		t.Fatal(p.creates, p.deletes)
	}
}
func TestPrivateGuestGrantRejectsCrossGenerationCorruptionAndOverwrite(t *testing.T) {
	for _, name := range []string{"generation", "operation", "sandbox", "token", "partial", "directory", "overwrite"} {
		t.Run(name, func(t *testing.T) {
			p := newProvider(t)
			cfg, _ := fixtureConfig(t, p)
			c := openController(t, cfg)
			r, e := c.Allocate(context.Background(), taskRequest("alpha", "one"))
			if e != nil {
				t.Fatal(e)
			}
			saveFixtureGrant(t, c, r)
			path := filepath.Join(cfg.StateDir, grantName(r.Request.Identity.OperationID))
			if name == "overwrite" {
				if e = c.saveGrant(r, providerSandbox{EnvdAccessToken: "replacement", TrafficAccessToken: "replacement", EnvdVersion: EnvdVersion}); e == nil {
					t.Fatal("grant overwritten")
				}
				return
			}
			b, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			var v guestGrant
			if e = json.Unmarshal(b, &v); e != nil {
				t.Fatal(e)
			}
			switch name {
			case "generation":
				v.Deployment.HostGeneration = "foreign"
			case "operation":
				v.OperationID = "foreign"
			case "sandbox":
				v.SandboxID = "foreign"
			case "token":
				v.TrafficAccessToken = ""
			}
			b, _ = json.Marshal(v)
			if name == "partial" {
				b = []byte("{")
			}
			if e = os.Remove(path); e != nil {
				t.Fatal(e)
			}
			if name == "directory" {
				e = os.Mkdir(path, 0700)
			} else {
				e = os.WriteFile(path, b, 0600)
			}
			if e != nil {
				t.Fatal(e)
			}
			if _, e = c.readGrant(r); !errors.Is(e, ErrConflict) {
				t.Fatal("unsafe grant read", e)
			}
		})
	}
}
