package registry_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry/registrytest"
)

// newFileStore adapts the constructor to registrytest.NewStoreFunc. It returns
// the interface rather than *FileStore on purpose: every assertion the suite
// makes is one task 11's in-memory double must satisfy too, and a suite written
// against the concrete type could not be reused by it.
func newFileStore(path string) (registry.Store, error) { return registry.NewFileStore(path) }

// TestFileStoreSatisfiesTheStoreContract is the call that makes the shared suite
// mean something. Without it registrytest compiles, reports nothing, and the one
// implementation of Store in the phase is untested.
//
// Every property of the Store interface lives in that suite and not in this
// file, so task 11's double is held to the identical set. What is left here is
// the one thing that reaches past the interface.
func TestFileStoreSatisfiesTheStoreContract(t *testing.T) {
	registrytest.RunStoreContract(t, newFileStore)
}

// TestASaveFailureRollsTheIndexBack is the failure mode Step 11 names, and the
// reason it is here rather than in the shared suite: it breaks the filesystem
// under the store, which has no meaning for an in-memory double. A mutation
// that applies in memory and fails on disk leaves the cockpit showing state no
// restart will find, which is the one class of bug this whole file exists to
// prevent.
//
// All four mutating paths are covered: Create, Update, Delete and — through
// WorktreeFlagStore — SetWorktreeReady.
func TestASaveFailureRollsTheIndexBack(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	path := filepath.Join(dir, "sessions.json")
	st := registrytest.MustOpen(t, newFileStore, path)
	if err := st.Create(registrytest.Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// The worktree flags are reached through the narrow interface task 10 takes,
	// which also proves *FileStore satisfies it.
	flags, ok := st.(registry.WorktreeFlagStore)
	if !ok {
		t.Fatalf("%T does not implement registry.WorktreeFlagStore", st)
	}

	// Replace the state directory with a regular file. os.MkdirAll then fails
	// with ENOTDIR before statepath.WriteAtomic reaches its temp file, on Windows exactly
	// as on Linux, which is the portable way to make every save fail — a chmod
	// on the directory would be ignored on Windows.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove %s: %v", dir, err)
	}
	if err := os.WriteFile(dir, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write %s: %v", dir, err)
	}

	if err := st.Create(registrytest.Seed("s-00000002", time.Time{})); err == nil {
		t.Fatal("Create reported success while the document could not be written")
	}
	if _, err := st.Get("s-00000002"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("Get after the failed Create: err = %v, want ErrNotFound", err)
	}

	if _, err := st.Update("s-00000001", func(sess *registry.Session) error {
		sess.Status = registry.StatusExited
		return nil
	}); err == nil {
		t.Fatal("Update reported success while the document could not be written")
	}
	got, err := st.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != registry.StatusStarting {
		t.Fatalf("status = %q after the failed Update, want starting — the index was not rolled back", got.Status)
	}

	if _, err := flags.SetWorktreeReady("w-4a77b1c9", true); err == nil {
		t.Fatal("SetWorktreeReady reported success while the document could not be written")
	}
	if flags.WorktreeFlagsSnapshot()["w-4a77b1c9"].Ready {
		t.Fatal("the flag stayed in the index after its save failed — the app would report a worktree ready that no restart will find")
	}

	if err := st.Delete("s-00000001"); err == nil {
		t.Fatal("Delete reported success while the document could not be written")
	}
	if _, err := st.Get("s-00000001"); err != nil {
		t.Fatalf("Get after the failed Delete: %v — the row left memory and never left the file", err)
	}
}
