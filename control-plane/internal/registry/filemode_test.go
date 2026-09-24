package registry_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest"
)

func TestTheDocumentIsWrittenPrivate(t *testing.T) {
	// statepath.WriteAtomic serves two callers whose readers differ (ruling R2):
	// CLAUDE.md is written Shared, for the sandbox's user, and this document is
	// written Private, because it carries attach tokens. The tests that moved
	// to statepath pin what each mode writes; only this one pins which mode the
	// registry asks for, so a registry that passed Shared would fail here and
	// nowhere else.
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not honour POSIX permission bits: os.Chmod only moves the read-only attribute")
	}
	root := t.TempDir()
	path := filepath.Join(root, "state", "sessions", "sessions.json")

	st, err := registry.NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if err := st.Create(registrytest.Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("document mode = %v, want -rw-------", perm)
	}
	// 0o700 less the umask is 0o700 under any umask that leaves the owner's
	// own bits alone, so the directories are compared exactly.
	for _, dir := range []string{filepath.Join(root, "state"), filepath.Join(root, "state", "sessions")} {
		dirInfo, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if perm := dirInfo.Mode().Perm(); perm != 0o700 {
			t.Fatalf("created directory %s has mode %v, want drwx------", dir, perm)
		}
	}
}
