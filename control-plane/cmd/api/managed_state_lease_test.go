//go:build linux || darwin || windows

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/statelease"
)

func TestManagedRootAndOrphanWorkerLeaseBeforeAnyStateWrite(t *testing.T) {
	for _, kind := range []string{"initial-bootstrap", "requested-root", "bound-root", "worker"} {
		t.Run(kind, func(t *testing.T) {
			dir := leaseStateDirectory(t)
			t.Setenv("LUCI_EXECUTION_STATE_DIR", filepath.Join(dir, "control-plane", "execution"))
			cfg := Config{Addr: "127.0.0.1:18730", StateDir: dir, InstancePath: filepath.Join(dir, "instance", "instance-id")}
			switch kind {
			case "requested-root":
				cfg.ManagedTeamID = "team-abcdefghijklmnopqrstuv"
			case "bound-root":
				if err := os.Mkdir(filepath.Join(dir, "control-plane"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "control-plane", "managed-team.json"), []byte("existing marker"), 0600); err != nil {
					t.Fatal(err)
				}
			case "worker":
				cfg.Worker = true
			}
			holder, err := statelease.Acquire(dir, "0.5.0")
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Close()
			if err := serve(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil))); !errors.Is(err, statelease.ErrInUse) {
				t.Fatal("server ignored held state lease")
			}
			if _, err := os.Stat(cfg.InstancePath); !os.IsNotExist(err) {
				t.Fatal("server wrote state before acquiring lease")
			}
		})
	}
}

// A second address cannot make an independent writer of the same state. Reuse
// after closing the owned sidecar keeps existing identity and session bytes.
func TestOrdinaryStateLeaseIndependentOfAddressAndBinding(t *testing.T) {
	dir := leaseStateDirectory(t)
	first, err := acquireManagedStateLease(Config{StateDir: dir, Addr: "127.0.0.1:18730"})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(binary, "-test.run=^TestOrdinaryStateLeaseContenderProcess$", "-test.timeout=30s")
	child.Env = append(os.Environ(), "LUCI_TEST_API_LEASE_ROOT="+dir)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("second API process did not refuse before bootstrap: %v\n%s", err, output)
	}
	sentinel := filepath.Join(dir, "existing-state.json")
	saved := []byte(`{"owner":"existing","project":"retained"}`)
	if err := os.WriteFile(sentinel, saved, 0600); err != nil {
		t.Fatal(err)
	}
	for _, cfg := range []Config{
		{StateDir: dir, Addr: "127.0.0.1:28730"},
		{SessionsPath: filepath.Join(dir, "sessions", "sessions.json"), Addr: "127.0.0.1:38730"},
	} {
		if second, err := acquireManagedStateLease(cfg); !errors.Is(err, statelease.ErrInUse) {
			if second != nil {
				second.Close()
			}
			t.Fatalf("same root did not refuse different address: %v", err)
		}
	}
	isolated, err := acquireManagedStateLease(Config{StateDir: leaseStateDirectory(t), Addr: "127.0.0.1:28730"})
	if err != nil {
		t.Fatal(err)
	}
	isolated.Close()
	first.Close()
	reopened, err := acquireManagedStateLease(Config{StateDir: dir, Addr: "127.0.0.1:48730"})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	data, err := os.ReadFile(sentinel)
	if err != nil || !bytes.Equal(data, saved) {
		t.Fatal("existing state was changed during contention/restart")
	}
}

func leaseStateDirectory(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(privateStateDir(t))
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestOrdinaryStateLeaseContenderProcess(t *testing.T) {
	dir := os.Getenv("LUCI_TEST_API_LEASE_ROOT")
	if dir == "" {
		return
	}
	cfg := Config{StateDir: dir, Addr: "127.0.0.1:28730", InstancePath: filepath.Join(dir, "instance", "instance-id")}
	if err := serve(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil))); !errors.Is(err, statelease.ErrInUse) {
		t.Fatalf("expected state-in-use before backend/stores, got %v", err)
	}
	if _, err := os.Stat(cfg.InstancePath); !os.IsNotExist(err) {
		t.Fatal("second API process wrote instance state")
	}
}
