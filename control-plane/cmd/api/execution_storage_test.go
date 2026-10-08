package main

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

type storageBackendFixture struct {
	*availableBackend
	prepared string
}

func (b *storageBackendFixture) LocalStorageStatus() resourcebudget.LocalStorageStatus {
	return resourcebudget.LocalStorageStatus{State: "confirmed", Revision: 7}
}
func (b *storageBackendFixture) PrepareLocalStorageConfirmation(_ context.Context, path string) (resourcebudget.LocalStorageConfirmation, error) {
	b.prepared = path
	return resourcebudget.LocalStorageConfirmation{Version: 1, VHDPath: path}, nil
}

func TestExecutionStorageUsesTrustedSharedWorkerDirectory(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "host", "execution")
	cfg := Config{StateDir: filepath.Join(root, "worker-private"), Worker: true}
	if _, err := executionStateDirectory(cfg, ""); err == nil {
		t.Fatal("worker silently used private state as host authority")
	}
	if _, err := executionStateDirectory(cfg, "relative"); err == nil {
		t.Fatal("relative host configuration accepted")
	}
	got, err := executionStateDirectory(cfg, shared)
	if err != nil || got != shared || executionStorageConfirmationPath(got) != filepath.Join(shared, "local-storage-confirmation.json") {
		t.Fatal(got, err)
	}
	cfg.Worker = false
	got, err = executionStateDirectory(cfg, "")
	if err != nil || got != filepath.Join(cfg.StateDir, "control-plane", "execution") {
		t.Fatal(got, err)
	}
}

func TestExecutionStorageRecoveringStatusNeverInitializesBackend(t *testing.T) {
	calls := 0
	ready := &storageBackendFixture{availableBackend: &availableBackend{}}
	b := &recoveringBackend{factory: func() (backend.Backend, error) { calls++; return ready, nil }}
	if status := b.LocalStorageStatus(); calls != 0 || status.State != "unavailable" {
		t.Fatal("GET reconnected Docker", calls, status)
	}
	wantScope := "not-required"
	if runtime.GOOS == "windows" {
		wantScope = "local-docker-desktop"
	}
	if b.LocalStorageStatus().Scope != wantScope {
		t.Fatal("recovering status does not match the public client scope enum")
	}
	result, err := b.PrepareLocalStorageConfirmation(context.Background(), "controlled-vhd")
	if err != nil || calls != 1 || result.VHDPath != "controlled-vhd" || ready.prepared != "controlled-vhd" {
		t.Fatal(result, err, calls)
	}
	if status := b.LocalStorageStatus(); status.Revision != 7 || calls != 1 {
		t.Fatal("cached backend status was not forwarded", status)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.PrepareLocalStorageConfirmation(context.Background(), "later"); err == nil || calls != 1 {
		t.Fatal("closed backend reconnected")
	}
}
