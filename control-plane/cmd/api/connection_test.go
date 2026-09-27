package main

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/connection"
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// TestNewConnectionRefusesAStateDirThatIsAFile is the fatal case serve relies
// on. The alternative - carrying on with a Manager that cannot write - answers
// every connect attempt with a write error naming a path, which is not
// something the person pasting a key can act on.
func TestNewConnectionRefusesAStateDirThatIsAFile(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "deploy")
	if err := os.WriteFile(file, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := newConnection(Config{DeployStateDir: file}, quietLog())
	if err == nil {
		t.Fatal("newConnection accepted a state directory that is a file")
	}
	if !strings.Contains(err.Error(), file) {
		t.Errorf("error %q does not name the path", err)
	}
}

// TestNewConnectionMakesNoNetworkCall: startup never waits on Panacea, and it
// writes nothing until a key has been proved. serve runs the first Check in a
// goroutine once the listener is up.
func TestNewConnectionMakesNoNetworkCall(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "deploy")
	m, err := newConnection(Config{DeployStateDir: dir, PanaceaURL: "https://dev.panacea.kpmg"}, quietLog())
	if err != nil {
		t.Fatalf("newConnection: %v", err)
	}
	got := m.Status()
	if got.State != connection.NotConnected {
		t.Errorf("state = %q, want not_connected with no key stored", got.State)
	}
	if got.Origin != "https://dev.panacea.kpmg" || !got.OriginLocked {
		t.Errorf("origin %q locked %v; want the administrator's address, locked", got.Origin, got.OriginLocked)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("the state directory was not created: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("the state directory holds %d files; a start that stored nothing must write nothing", len(entries))
	}
}

// TestMemorySecretsRoundTrip pins the one behaviour connection.Config asks of a
// credential store beyond storing: a missing row is errSecretNotFound and not
// some other error, because "you have not connected" and "the store is broken"
// send a user to different places.
func TestMemorySecretsRoundTrip(t *testing.T) {
	t.Parallel()
	s := newMemorySecrets()
	if _, err := s.Get("PanaceaApp/panacea/dev.panacea.kpmg"); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("Get of a missing row = %v, want errSecretNotFound", err)
	}
	if err := s.Put("PanaceaApp/panacea/dev.panacea.kpmg", "dmackie1", []byte("k")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	blob, err := s.Get("PanaceaApp/panacea/dev.panacea.kpmg")
	if err != nil || string(blob) != "k" {
		t.Fatalf("Get = %q, %v; want the stored blob", blob, err)
	}
	if err := s.Delete("PanaceaApp/panacea/dev.panacea.kpmg"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("PanaceaApp/panacea/dev.panacea.kpmg"); !errors.Is(err, errSecretNotFound) {
		t.Errorf("Get after Delete = %v, want errSecretNotFound", err)
	}
}
