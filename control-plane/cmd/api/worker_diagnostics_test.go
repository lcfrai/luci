package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/diagnostics"
)

func TestWorkerDiagnosticsArePrivateWorkerOnlyAndUnavailableStorageIsNonfatal(t *testing.T) {
	root := t.TempDir()
	if got := openWorkerDiagnostics(Config{StateDir: root}, quietLogger()); got != nil {
		t.Fatal("standalone API acquired a worker recorder")
	}
	if _, err := os.Stat(filepath.Join(root, diagnostics.Directory)); !os.IsNotExist(err) {
		t.Fatal("standalone wiring created diagnostic state")
	}
	r := openWorkerDiagnostics(Config{StateDir: root, Worker: true}, quietLogger())
	if r == nil {
		t.Fatal("worker recorder missing")
	}
	if err := r.Record("prepare private provider state", os.ErrPermission); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, diagnostics.Directory, diagnostics.Filename)); err != nil {
		t.Fatal(err)
	}
	blocked := t.TempDir()
	if err := os.WriteFile(filepath.Join(blocked, diagnostics.Directory), []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := openWorkerDiagnostics(Config{StateDir: blocked, Worker: true}, quietLogger()); got != nil {
		got.Close()
		t.Fatal("invalid storage was adopted")
	}
	data, err := os.ReadFile(filepath.Join(blocked, diagnostics.Directory))
	if err != nil || string(data) != "preserve" {
		t.Fatal("diagnostic failure modified unrelated state", err)
	}
}
