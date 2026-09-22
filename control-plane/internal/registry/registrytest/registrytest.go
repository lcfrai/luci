// Package registrytest is the behaviour suite every registry.Store
// implementation must pass. FileStore runs it from
// filestore_contract_test.go, and task 11's in-memory double runs the same
// suite, which is the only thing that stops a double drifting from the thing
// it doubles.
//
// IT IMPORTS testing AT ORDINARY BUILD TIME, AND THAT IS DELIBERATE. It is a
// normal package rather than a _test.go file because Go will not let anything
// import an external test package (package registry_test), so a suite written
// there could be called from exactly one file and the "every implementation"
// promise would be a comment. net/http/httptest and testing/fstest have this
// exact shape for this exact reason. The testing import costs nothing outside
// test binaries, because nothing but test code imports this package. Do not
// "fix" it by renaming the file.
//
// A property that belongs to the Store interface belongs HERE, not in an
// implementation's own test file: a property outside this suite is a property
// the double is never held to.
package registrytest

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// NewStoreFunc opens a Store at path, creating it when it does not exist.
// Calling it twice with one path must reopen the same state: several tests
// below prove a property by reloading, which is the only way to tell an
// in-memory edit from a persisted one.
type NewStoreFunc func(path string) (registry.Store, error)

// RunStoreContract holds any Store implementation to the rules in contract
// §2.8. filestore_contract_test.go calls it with NewFileStore, and task 11's
// in-memory double is expected to call it too: an interface with one tested
// implementation is a type declaration rather than a contract.
//
// This package is not a _test.go file, so it imports testing at normal build
// time. That is deliberate and is what net/http/httptest and testing/fstest do:
// a suite only two packages can reach is a suite an external test package
// cannot import at all. Do not "fix" it by renaming the file.
func RunStoreContract(t *testing.T, newStore NewStoreFunc) {
	t.Helper()
	t.Run("CreateGetListDelete", func(t *testing.T) { testCreateGetListDelete(t, newStore) })
	t.Run("CreateRejectsDuplicateAndEmptyID", func(t *testing.T) { testCreateRejects(t, newStore) })
	t.Run("CreateClonesTheSessionItIsGiven", func(t *testing.T) { testCreateClones(t, newStore) })
	t.Run("CreateStampsUpdatedAtAndNormalisesTimesToUTC", func(t *testing.T) { testCreateStamps(t, newStore) })
	t.Run("UpdateAppliesAndStamps", func(t *testing.T) { testUpdateStamps(t, newStore) })
	t.Run("UpdateErrorDiscardsTheChange", func(t *testing.T) { testUpdateErrorDiscardsTheChange(t, newStore) })
	t.Run("UpdateStampsInUTCAndRefusesAnIDChange", func(t *testing.T) { testUpdateRefusesAnIDChange(t, newStore) })
	t.Run("TheSealHolds", func(t *testing.T) { testSeal(t, newStore) })
	t.Run("ListReturnsClones", func(t *testing.T) { testListReturnsClones(t, newStore) })
	t.Run("ListIsOrderedByCreatedAt", func(t *testing.T) { testListOrder(t, newStore) })
	t.Run("DeleteRemovesOnlyItsOwnRecord", func(t *testing.T) { testDeleteRemovesOnlyItsOwnRecord(t, newStore) })
	t.Run("ConcurrentUpdatesAllLand", func(t *testing.T) { testConcurrentUpdates(t, newStore) })
}

// StorePath puts the document inside the test's own temp directory. t.TempDir
// is removed when the test ends, on Windows as well as Linux, so no test here
// leaves a file behind or collides with a sibling.
func StorePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "sessions.json")
}

// MustOpen opens the store or fails the test. StorePath, MustOpen and Seed are
// exported because filestore_contract_test.go and task 11's double write their
// own tests beside the suite and must build the same fixtures the suite does.
func MustOpen(t *testing.T, newStore NewStoreFunc, path string) registry.Store {
	t.Helper()
	s, err := newStore(path)
	if err != nil {
		t.Fatalf("open store at %s: %v", path, err)
	}
	return s
}

// Seed is the fixture every test here starts from: the fields a session cannot
// be without, and nothing a single test would then have to work around.
func Seed(id string, created time.Time) registry.Session {
	return registry.Session{
		ID:        id,
		Title:     "session " + id,
		MachineID: "local",
		Harness:   "claude-code",
		Command:   "claude",
		Args:      []string{"--model", "opus"},
		Status:    registry.StatusStarting,
		Backend:   "docker",
		CreatedAt: created,
	}
}

func testCreateGetListDelete(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)

	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "session s-00000001" {
		t.Fatalf("Title = %q, want %q", got.Title, "session s-00000001")
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("Create left CreatedAt zero; the store stamps it when the caller has not")
	}
	if got.CreatedAt.Location() != time.UTC {
		t.Fatalf("CreatedAt location = %v, want UTC", got.CreatedAt.Location())
	}
	if _, err := s.Get("s-nope"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("Get unknown id: err = %v, want ErrNotFound", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d sessions, want 1", len(list))
	}

	// Reopening proves the write reached the file rather than only the index.
	reloaded := MustOpen(t, newStore, path)
	if _, err := reloaded.Get("s-00000001"); err != nil {
		t.Fatalf("after reload, Get: %v", err)
	}

	if err := s.Delete("s-00000001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get("s-00000001"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("after Delete, Get: err = %v, want ErrNotFound", err)
	}
	if err := s.Delete("s-00000001"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("second Delete: err = %v, want ErrNotFound", err)
	}
	if _, err := MustOpen(t, newStore, path).Get("s-00000001"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("Delete did not reach the file: err = %v, want ErrNotFound", err)
	}
}

func testCreateRejects(t *testing.T, newStore NewStoreFunc) {
	s := MustOpen(t, newStore, StorePath(t))

	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if err := s.Create(Seed("s-00000001", time.Time{})); !errors.Is(err, registry.ErrExists) {
		t.Fatalf("duplicate Create: err = %v, want ErrExists", err)
	}
	if err := s.Create(Seed("", time.Time{})); !errors.Is(err, registry.ErrInvalidID) {
		t.Fatalf("Create with an empty id: err = %v, want ErrInvalidID", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("a rejected Create still landed: List returned %d sessions, want 1", len(list))
	}
}

// testCreateClones closes the half of the seal testSeal cannot see: the caller
// still holds the value it passed, and Args is a slice, so a Create that stores
// the struct as given shares its backing array with whatever the caller does
// next — outside the lock, with nothing serialising it. An in-memory double is
// the likelier offender here than a file store, because it has no marshalling
// step to launder the aliasing away.
func testCreateClones(t *testing.T, newStore NewStoreFunc) {
	s := MustOpen(t, newStore, StorePath(t))

	in := Seed("s-00000001", time.Time{})
	if err := s.Create(in); err != nil {
		t.Fatalf("Create: %v", err)
	}
	in.Args[0] = "--smuggled"

	got, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Args[0] != "--model" {
		t.Fatalf("the stored record shares the caller's Args array: %q, want --model", got.Args[0])
	}
}

// testCreateStamps pins decision 2 beyond what testCreateGetListDelete reaches:
// that a caller-supplied CreatedAt keeps its instant and loses its zone, and
// that UpdatedAt is stamped too. A record nobody has updated must still read as
// updated when it was created.
func testCreateStamps(t *testing.T, newStore NewStoreFunc) {
	s := MustOpen(t, newStore, StorePath(t))

	// A fixed offset rather than a zoneinfo lookup: Windows ships no tzdata, so
	// time.LoadLocation("Australia/Sydney") fails there and nowhere else.
	sydney := time.FixedZone("AEST", 10*60*60)
	created := time.Date(2026, 9, 18, 19, 14, 3, 0, sydney)
	if err := s.Create(Seed("s-00000001", created)); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get("s-00000001")
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

func testUpdateStamps(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)
	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}
	before, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	got, err := s.Update("s-00000001", func(sess *registry.Session) error {
		sess.Status = registry.StatusRunning
		sess.AttachCount++
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Status != registry.StatusRunning || got.AttachCount != 1 {
		t.Fatalf("Update returned %q/%d, want running/1", got.Status, got.AttachCount)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("Update left UpdatedAt zero; the store stamps it after mutate returns")
	}
	// Not-before rather than strictly-after: the Windows wall clock can tick in
	// steps of roughly 15 ms, so two updates inside one tick share a timestamp
	// and a strict comparison would fail on that machine and nowhere else.
	if got.UpdatedAt.Before(before.UpdatedAt) {
		t.Fatalf("UpdatedAt went backwards: %v then %v", before.UpdatedAt, got.UpdatedAt)
	}
	if !got.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("Update moved CreatedAt from %v to %v", before.CreatedAt, got.CreatedAt)
	}

	if _, err := s.Update("s-nope", func(*registry.Session) error { return nil }); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("Update unknown id: err = %v, want ErrNotFound", err)
	}

	after, err := MustOpen(t, newStore, path).Get("s-00000001")
	if err != nil {
		t.Fatalf("after reload, Get: %v", err)
	}
	if after.Status != registry.StatusRunning || after.AttachCount != 1 {
		t.Fatalf("Update did not reach the file: %q/%d, want running/1", after.Status, after.AttachCount)
	}
}

func testUpdateErrorDiscardsTheChange(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)
	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	boom := errors.New("boom")
	if _, err := s.Update("s-00000001", func(sess *registry.Session) error {
		sess.Status = registry.StatusExited
		return boom
	}); !errors.Is(err, boom) {
		t.Fatalf("Update: err = %v, want the mutate error returned unwrapped", err)
	}

	got, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != registry.StatusStarting {
		t.Fatalf("a failed mutate was committed to the index: status = %q, want starting", got.Status)
	}
	after, err := MustOpen(t, newStore, path).Get("s-00000001")
	if err != nil {
		t.Fatalf("after reload, Get: %v", err)
	}
	if after.Status != registry.StatusStarting {
		t.Fatalf("a failed mutate reached the file: status = %q, want starting", after.Status)
	}
}

// testUpdateRefusesAnIDChange. The id is the store's key: a mutate that changes
// it would leave the store keyed by one id and holding a record carrying
// another, and the next reload would rename the session with nothing to show
// why. Refusing is a symptom; restoring the id quietly is not. UpdatedAt's zone
// rides along here because it is the same call.
func testUpdateRefusesAnIDChange(t *testing.T, newStore NewStoreFunc) {
	s := MustOpen(t, newStore, StorePath(t))
	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Update("s-00000001", func(sess *registry.Session) error {
		sess.Status = registry.StatusIdle
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.UpdatedAt.Location() != time.UTC {
		t.Fatalf("UpdatedAt location = %v, want UTC", got.UpdatedAt.Location())
	}

	if _, err := s.Update("s-00000001", func(sess *registry.Session) error {
		sess.ID = "s-00000002"
		return nil
	}); err == nil {
		t.Fatal("Update accepted a mutate that changed the session id")
	}
	if _, err := s.Get("s-00000002"); !errors.Is(err, registry.ErrNotFound) {
		t.Fatalf("Get s-00000002: err = %v, want ErrNotFound — the rename was committed", err)
	}
	if _, err := s.Get("s-00000001"); err != nil {
		t.Fatalf("Get s-00000001 after the refused rename: %v", err)
	}
}

func testSeal(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)
	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Capture the pointer mutate is handed, then write through it after Update
	// has returned. If the store hands out a pointer into its own index, this
	// late write lands in memory and never reaches the file, and the cockpit
	// then shows state the document does not contain.
	var escaped *registry.Session
	if _, err := s.Update("s-00000001", func(sess *registry.Session) error {
		escaped = sess
		sess.Title = "committed"
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	escaped.Title = "smuggled"
	escaped.Args[0] = "--smuggled"

	got, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "committed" {
		t.Fatalf("a write through the escaped pointer reached the index: title = %q, want committed", got.Title)
	}
	if got.Args[0] != "--model" {
		t.Fatalf("the escaped pointer still shares the Args array: %q, want --model", got.Args[0])
	}

	after, err := MustOpen(t, newStore, path).Get("s-00000001")
	if err != nil {
		t.Fatalf("after reload, Get: %v", err)
	}
	if after.Title != "committed" || after.Args[0] != "--model" {
		t.Fatalf("the escaped write survived a reload: %q / %q, want committed / --model", after.Title, after.Args[0])
	}

	// The same seal on the read side: a caller editing what Get returned must
	// not be editing the store.
	got.Title = "edited by the caller"
	got.Args[0] = "--edited"
	again, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if again.Title != "committed" || again.Args[0] != "--model" {
		t.Fatalf("Get returned a view into the index: %q / %q, want committed / --model", again.Title, again.Args[0])
	}
}

// testListReturnsClones is the seal on the read side of List. testSeal pins it
// for Get only, and a List that ranges the index appending the stored values
// hands out live Args arrays to every poll of the rail.
func testListReturnsClones(t *testing.T, newStore NewStoreFunc) {
	s := MustOpen(t, newStore, StorePath(t))
	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d sessions, want 1", len(list))
	}
	list[0].Args[0] = "--smuggled"

	got, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Args[0] != "--model" {
		t.Fatalf("List returned a view into the index: %q, want --model", got.Args[0])
	}
}

func testListOrder(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)

	base := time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)
	// Created out of order, and two share a timestamp so the id tie-break is
	// exercised rather than assumed.
	for _, tc := range []struct {
		id      string
		created time.Time
	}{
		{"s-0000000c", base.Add(2 * time.Hour)},
		{"s-0000000a", base},
		{"s-0000000b", base.Add(time.Hour)},
		{"s-0000000d", base.Add(2 * time.Hour)},
	} {
		if err := s.Create(Seed(tc.id, tc.created)); err != nil {
			t.Fatalf("Create %s: %v", tc.id, err)
		}
	}

	want := []string{"s-0000000a", "s-0000000b", "s-0000000c", "s-0000000d"}
	for _, store := range []registry.Store{s, MustOpen(t, newStore, path)} {
		list, err := store.List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		got := make([]string, 0, len(list))
		for _, sess := range list {
			got = append(got, sess.ID)
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("List order = %v, want %v — in map order the rail reshuffles on every poll", got, want)
		}
	}
}

// testDeleteRemovesOnlyItsOwnRecord. testCreateGetListDelete deletes the only
// session it created, so a Delete that clears the whole store, or one that
// drops the record and leaves its id in whatever structure List orders by,
// passes it. This reloads, because the survivors have to be durable and not
// just present in memory.
func testDeleteRemovesOnlyItsOwnRecord(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)

	base := time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC)
	for i, id := range []string{"s-0000000a", "s-0000000b", "s-0000000c"} {
		if err := s.Create(Seed(id, base.Add(time.Duration(i)*time.Hour))); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}
	if err := s.Delete("s-0000000b"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	list, err := MustOpen(t, newStore, path).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := make([]string, 0, len(list))
	for _, sess := range list {
		got = append(got, sess.ID)
	}
	want := []string{"s-0000000a", "s-0000000c"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("after Delete the store holds %v, want %v", got, want)
	}
}

func testConcurrentUpdates(t *testing.T, newStore NewStoreFunc) {
	path := StorePath(t)
	s := MustOpen(t, newStore, path)
	if err := s.Create(Seed("s-00000001", time.Time{})); err != nil {
		t.Fatalf("Create: %v", err)
	}

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Update("s-00000001", func(sess *registry.Session) error {
				sess.AttachCount++
				return nil
			}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Update: %v", err)
	}

	got, err := s.Get("s-00000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AttachCount != n {
		t.Fatalf("AttachCount = %d, want %d — a read-modify-write was lost, which is the whole reason Update exists", got.AttachCount, n)
	}
	after, err := MustOpen(t, newStore, path).Get("s-00000001")
	if err != nil {
		t.Fatalf("after reload, Get: %v", err)
	}
	if after.AttachCount != n {
		t.Fatalf("after reload AttachCount = %d, want %d", after.AttachCount, n)
	}
}
