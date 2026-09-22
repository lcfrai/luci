package registry_test

import (
	"errors"
	"fmt"
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
func TestFileStoreSatisfiesTheStoreContract(t *testing.T) {
	registrytest.RunStoreContract(t, newFileStore)
}

// TestCreateClonesTheSessionItIsGiven closes the half of the seal the suite
// cannot see: the caller still holds the value it passed, and Args is a slice,
// so a Create that stores the struct as given shares its backing array with
// whatever the caller does next — outside the lock, with nothing serialising it.
func TestCreateClonesTheSessionItIsGiven(t *testing.T) {
	st := registrytest.MustOpen(t, newFileStore, registrytest.StorePath(t))

	in := registrytest.Seed("s-00000001", time.Time{})
	if err := st.Create(in); err != nil {
		t.Fatalf("Create: %v", err)
	}
	in.Args[0] = "--smuggled"

	got, err := st.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Args[0] != "--model" {
		t.Fatalf("the stored record shares the caller's Args array: %q, want --model", got.Args[0])
	}
}

// TestListReturnsClones is the same seal on the read side of List. The suite
// pins it for Get only, and a List that ranges the index appending the map
// values hands out live Args arrays to every poll of the rail.
func TestListReturnsClones(t *testing.T) {
	st := registrytest.MustOpen(t, newFileStore, registrytest.StorePath(t))
	if err := st.Create(registrytest.Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	list, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d sessions, want 1", len(list))
	}
	list[0].Args[0] = "--smuggled"

	got, err := st.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Args[0] != "--model" {
		t.Fatalf("List returned a view into the index: %q, want --model", got.Args[0])
	}
}

// TestCreateStampsUpdatedAtAndNormalisesTimesToUTC pins decision 2. The suite
// checks CreatedAt is stamped and UTC when the caller left it zero; this checks
// the other two cases — a caller-supplied time keeps its instant and loses its
// zone, and UpdatedAt is stamped as well.
func TestCreateStampsUpdatedAtAndNormalisesTimesToUTC(t *testing.T) {
	st := registrytest.MustOpen(t, newFileStore, registrytest.StorePath(t))

	// A fixed offset rather than a zoneinfo lookup: Windows ships no tzdata, so
	// time.LoadLocation("Australia/Sydney") fails there and nowhere else.
	sydney := time.FixedZone("AEST", 10*60*60)
	created := time.Date(2026, 9, 18, 19, 14, 3, 0, sydney)
	if err := st.Create(registrytest.Seed("s-00000001", created)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := st.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.CreatedAt.Equal(created) {
		t.Fatalf("CreatedAt = %s, want the instant the caller supplied, %s", got.CreatedAt, created)
	}
	if got.CreatedAt.Location() != time.UTC {
		t.Fatalf("CreatedAt location = %v, want UTC — two machines' documents should diff", got.CreatedAt.Location())
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("Create left UpdatedAt zero; a record nobody has updated reads as updated when it was created")
	}
	if got.UpdatedAt.Location() != time.UTC {
		t.Fatalf("UpdatedAt location = %v, want UTC", got.UpdatedAt.Location())
	}
}

// TestUpdateStampsInUTCAndRefusesAnIDChange. The id is the map key: a mutate
// that changes it would leave the index keyed by one id and holding a record
// carrying another, and the next reload would rename the session silently.
// Refusing is a symptom; restoring the id quietly is not.
func TestUpdateStampsInUTCAndRefusesAnIDChange(t *testing.T) {
	st := registrytest.MustOpen(t, newFileStore, registrytest.StorePath(t))
	if err := st.Create(registrytest.Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := st.Update("s-00000001", func(sess *registry.Session) error {
		sess.Status = registry.StatusIdle
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.UpdatedAt.Location() != time.UTC {
		t.Fatalf("UpdatedAt location = %v, want UTC", got.UpdatedAt.Location())
	}

	if _, err := st.Update("s-00000001", func(sess *registry.Session) error {
		sess.ID = "s-00000002"
		return nil
	}); err == nil {
		t.Fatal("Update accepted a mutate that changed the session id")
	}
	if _, err := st.Get("s-00000002"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("Get s-00000002: err = %v, want ErrNotFound — the rename was committed", err)
	}
	if _, err := st.Get("s-00000001"); err != nil {
		t.Fatalf("Get s-00000001 after the refused rename: %v", err)
	}
}

// TestDeleteRemovesOnlyItsOwnRecord. The suite deletes the only session it
// created, so a Delete that clears the whole index, or one that drops the row
// from the map and leaves its id in the order slice, passes it. This reloads,
// because the survivors have to be in the document and not just in memory.
func TestDeleteRemovesOnlyItsOwnRecord(t *testing.T) {
	path := registrytest.StorePath(t)
	st := registrytest.MustOpen(t, newFileStore, path)

	base := time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)
	for i, id := range []string{"s-0000000a", "s-0000000b", "s-0000000c"} {
		if err := st.Create(registrytest.Seed(id, base.Add(time.Duration(i)*time.Hour))); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}
	if err := st.Delete("s-0000000b"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	list, err := registrytest.MustOpen(t, newFileStore, path).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := make([]string, 0, len(list))
	for _, sess := range list {
		got = append(got, sess.ID)
	}
	want := []string{"s-0000000a", "s-0000000c"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("after Delete the document holds %v, want %v", got, want)
	}
}

// TestASaveFailureRollsTheIndexBack is the failure mode Step 11 names. A
// mutation that applies in memory and fails on disk leaves the cockpit showing
// state no restart will find, which is the one class of bug this whole file
// exists to prevent.
func TestASaveFailureRollsTheIndexBack(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	path := filepath.Join(dir, "sessions.json")
	st := registrytest.MustOpen(t, newFileStore, path)
	if err := st.Create(registrytest.Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Replace the state directory with a regular file. os.MkdirAll then fails
	// with ENOTDIR before writeAtomic reaches its temp file, on Windows exactly
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

	if err := st.Delete("s-00000001"); err == nil {
		t.Fatal("Delete reported success while the document could not be written")
	}
	if _, err := st.Get("s-00000001"); err != nil {
		t.Fatalf("Get after the failed Delete: %v — the row left memory and never left the file", err)
	}
}
