package lanes

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func lane(id, projectID, branch, name string, at time.Time) Lane {
	return Lane{ID: id, ProjectID: projectID, Branch: branch, Name: name,
		Path: filepath.Join("C:\\lanes", projectID, name+"-aaaaaa"), MachineID: "local", CreatedAt: at}
}

func openStore(t *testing.T, dir string) *FileStore {
	t.Helper()
	s, err := OpenFileStore(dir)
	if err != nil {
		t.Fatalf("OpenFileStore: %v", err)
	}
	return s
}

func TestFileStoreRoundTripsAcrossReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "lanes")
	s := openStore(t, dir)
	at := time.Date(2026, 9, 27, 1, 2, 3, 0, time.UTC)
	a, b := lane("w-aaaaaaaa", "p-1", "feat", "ui", at), lane("w-bbbbbbbb", "p-1", "feat", "api", at.Add(time.Second))
	for _, l := range []Lane{a, b} {
		if err := s.Add(l); err != nil {
			t.Fatalf("Add(%s): %v", l.ID, err)
		}
	}
	if err := s.SetQueue("p-1", "feat", []string{b.ID, a.ID}); err != nil {
		t.Fatalf("SetQueue: %v", err)
	}
	r := Rename{ProjectID: "p-1", FromBranchID: "b-x", ToBranchID: "b-y", From: "x", To: "y", At: at}
	if err := s.AddRename(r); err != nil {
		t.Fatalf("AddRename: %v", err)
	}

	re := openStore(t, dir)
	if got, want := re.List(), s.List(); !reflect.DeepEqual(got, want) || len(got) != 2 {
		t.Errorf("List after reopen = %+v, want %+v", got, want)
	}
	if got := re.ByProject("p-1"); len(got) != 2 || got[0].ID != a.ID || got[1].ID != b.ID {
		t.Errorf("ByProject after reopen = %+v, want a then b (CreatedAt order)", got)
	}
	if got, ok := re.ByID(b.ID); !ok || !reflect.DeepEqual(got, b) {
		t.Errorf("ByID(%s) after reopen = %+v, %v; want %+v", b.ID, got, ok, b)
	}
	if got := re.Queue("p-1", "feat"); !reflect.DeepEqual(got, []string{b.ID, a.ID}) {
		t.Errorf("Queue after reopen = %v", got)
	}
	if got := re.Renames("p-1"); len(got) != 1 || !reflect.DeepEqual(got[0], r) {
		t.Errorf("Renames after reopen = %+v, want [%+v]", got, r)
	}
	if got := re.Queue("p-1", "other"); got != nil {
		t.Errorf("Queue of a branch never set = %v, want nil", got)
	}
}

func TestFileStoreRefusesDuplicateIDOrName(t *testing.T) {
	s := openStore(t, t.TempDir())
	at := time.Now().UTC()
	if err := s.Add(lane("w-11111111", "p-1", "feat", "ui", at)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Add(lane("w-11111111", "p-2", "feat", "other", at)); !errors.Is(err, ErrExists) {
		t.Errorf("same ID: err = %v, want ErrExists", err)
	}
	if err := s.Add(lane("w-22222222", "p-1", "main", "ui", at)); !errors.Is(err, ErrExists) {
		t.Errorf("same ProjectID+Name, other ID: err = %v, want ErrExists", err)
	}
	if err := s.Add(lane("w-33333333", "p-2", "feat", "ui", at)); err != nil {
		t.Errorf("same Name in another project: err = %v, want nil", err)
	}
	if n := len(s.List()); n != 2 {
		t.Errorf("List has %d lanes, want 2", n)
	}
}

func TestFileStoreRemoveDropsTheIDFromQueues(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	at := time.Now().UTC()
	for _, l := range []Lane{lane("w-11111111", "p-1", "feat", "a", at), lane("w-22222222", "p-1", "feat", "b", at)} {
		if err := s.Add(l); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := s.SetQueue("p-1", "feat", []string{"w-11111111", "w-22222222"}); err != nil {
		t.Fatalf("SetQueue: %v", err)
	}
	if err := s.Remove("w-11111111"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got := s.Queue("p-1", "feat"); !reflect.DeepEqual(got, []string{"w-22222222"}) {
		t.Errorf("Queue after Remove = %v, want [w-22222222]", got)
	}
	if got := openStore(t, dir).Queue("p-1", "feat"); !reflect.DeepEqual(got, []string{"w-22222222"}) {
		t.Errorf("Queue after Remove and reopen = %v, want [w-22222222]", got)
	}
	if err := s.Remove("w-11111111"); !errors.Is(err, ErrUnknown) {
		t.Errorf("Remove of a removed id: err = %v, want ErrUnknown", err)
	}
}

// TestFileStoreFailedWriteLeavesMemoryUnchanged: the part's "a read-only dir" cannot fail a write on
// Windows, which ignores FILE_ATTRIBUTE_READONLY on a directory. A directory standing where
// lanes.json goes fails WriteAtomic's rename on every OS, which is the same full-disk shape.
func TestFileStoreFailedWriteLeavesMemoryUnchanged(t *testing.T) {
	dir := t.TempDir()
	s := openStore(t, dir)
	at := time.Now().UTC()
	if err := s.Add(lane("w-11111111", "p-1", "feat", "a", at)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	before := s.List()
	doc := filepath.Join(dir, "lanes.json")
	if err := os.Remove(doc); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(doc, "block"), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := s.Add(lane("w-22222222", "p-1", "feat", "b", at)); err == nil {
		t.Fatal("Add over an unwritable index answered nil")
	}
	if got := s.List(); !reflect.DeepEqual(got, before) {
		t.Errorf("List after a failed Add = %+v, want %+v", got, before)
	}
	if err := s.SetQueue("p-1", "feat", []string{"w-11111111"}); err == nil {
		t.Fatal("SetQueue over an unwritable index answered nil")
	}
	if got := s.Queue("p-1", "feat"); got != nil {
		t.Errorf("Queue after a failed SetQueue = %v, want nil", got)
	}
	if err := s.Remove("w-11111111"); err == nil {
		t.Fatal("Remove over an unwritable index answered nil")
	}
	if _, ok := s.ByID("w-11111111"); !ok {
		t.Error("a failed Remove dropped the lane from memory")
	}
}

func TestOpenFileStoreRefusesACorruptFile(t *testing.T) {
	dir := t.TempDir()
	doc := filepath.Join(dir, "lanes.json")
	if err := os.WriteFile(doc, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	s, err := OpenFileStore(dir)
	if err == nil {
		t.Fatalf("OpenFileStore over a corrupt file = %v, nil; want an error", s)
	}
	if !strings.Contains(err.Error(), doc) {
		t.Errorf("error %q does not name %s", err, doc)
	}
}

func TestOpenFileStoreCreatesItsDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "lanes")
	s := openStore(t, dir)
	if n := len(s.List()); n != 0 {
		t.Errorf("a new index lists %d lanes", n)
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		t.Fatalf("OpenFileStore did not create %s: %v", dir, err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, want 0700", fi.Mode().Perm())
	}
}

func TestFileStoreRetargetMovesLanesAndQueues(t *testing.T) {
	s := openStore(t, t.TempDir())
	at := time.Now().UTC()
	if err := s.Add(lane("w-11111111", "p-1", "old", "a", at)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.SetQueue("p-1", "old", []string{"w-11111111"}); err != nil {
		t.Fatalf("SetQueue: %v", err)
	}
	if err := s.Retarget("p-1", "old", "new"); err != nil {
		t.Fatalf("Retarget: %v", err)
	}
	if l, _ := s.ByID("w-11111111"); l.Branch != "new" {
		t.Errorf("lane branch after Retarget = %q, want new", l.Branch)
	}
	if got := s.Queue("p-1", "new"); !reflect.DeepEqual(got, []string{"w-11111111"}) {
		t.Errorf("Queue(new) = %v", got)
	}
	if got := s.Queue("p-1", "old"); got != nil {
		t.Errorf("Queue(old) = %v, want nil", got)
	}
}
