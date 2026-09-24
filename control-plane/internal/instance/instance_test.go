package instance

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

// The property the label rests on: a restart on the same state directory keeps
// the identity, so it keeps ownership of the sandboxes it made before it
// stopped.
func TestLoadMintsOnceAndEveryLaterLoadReadsTheSameID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance", FileName)

	first, minted, err := Load(path)
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if !minted {
		t.Error("the first Load on an empty directory reported minted=false")
	}
	if err := backend.ValidateInstance(first); err != nil {
		t.Fatalf("minted id %q is not stampable as a label on either backend: %v", first, err)
	}
	for i := 0; i < 3; i++ {
		again, minted, err := Load(path)
		if err != nil {
			t.Fatalf("Load %d: %v", i, err)
		}
		if again != first {
			t.Fatalf("Load %d returned %q, want %q: a restart just disowned every sandbox it had made", i, again, first)
		}
		if minted {
			t.Errorf("Load %d reported minted=true for an id that was already on disk", i)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != first+"\n" {
		t.Errorf("file holds %q, want %q", raw, first+"\n")
	}
}

func TestTwoStateDirectoriesAreTwoInstances(t *testing.T) {
	a, _, err := Load(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := Load(filepath.Join(t.TempDir(), FileName))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("two state directories share instance id %q, so their control planes own each other's sandboxes", a)
	}
}

// A hand-edited, truncated or corrupted id is refused, and the file is left
// exactly as it was. Replacing it would disown every sandbox the old id made.
func TestACorruptIDIsRefusedAndNeverReplaced(t *testing.T) {
	for _, body := range []string{"", "\n", "Not An ID", "-leading-hyphen", strings.Repeat("a", 64)} {
		path := filepath.Join(t.TempDir(), FileName)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if id, _, err := Load(path); err == nil {
			t.Errorf("body %q: Load returned %q with no error", body, id)
			continue
		} else if !strings.Contains(err.Error(), path) {
			t.Errorf("body %q: the error does not name the file: %v", body, err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != body {
			t.Errorf("body %q: Load rewrote the file to %q", body, raw)
		}
	}
}

// The file is linked into place, so the temporary it was written to is the
// only thing a mint could leave behind — and it does not.
func TestAMintLeavesNothingButTheIDFile(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Load(filepath.Join(dir, FileName)); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != FileName {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want only %s", names, FileName)
	}
}

// Two control planes started on one directory at once: the one that links
// second must adopt the first's id, not overwrite it. Not parallel — it swaps
// the package's hook.
func TestTheLoserOfAMintRaceAdoptsTheWinnersID(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	const winner = "33333333-3333-4333-8333-333333333333"
	beforeLink = func() {
		if err := os.WriteFile(path, []byte(winner+"\n"), 0o600); err != nil {
			t.Errorf("write the winner: %v", err)
		}
	}
	t.Cleanup(func() { beforeLink = func() {} })

	got, minted, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got != winner {
		t.Errorf("the loser returned %q, want the winner's %q: two processes on one directory now disagree about what they own", got, winner)
	}
	if minted {
		t.Error("the loser reported minted=true")
	}
	raw, _ := os.ReadFile(path)
	if strings.TrimSpace(string(raw)) != winner {
		t.Errorf("the winner's file was overwritten with %q", raw)
	}
}

// Reaper-scope fix round, m1. FAT/exFAT and some SMB shares (Azure Files over
// SMB, for one) have no hard links, and os.Link fails there with an error that
// is not ErrExist. The mint must not stop the control plane on its first start
// for that: it falls back to an exclusive create of the final name, which
// still never replaces an existing id.
func TestLoadMintsOnAFilesystemWithoutHardLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance", FileName)
	link = func(string, string) error {
		return &os.LinkError{Op: "link", Err: errors.New("the file system does not support hard links")}
	}
	t.Cleanup(func() { link = os.Link })

	id, minted, err := Load(path)
	if err != nil {
		t.Fatalf("Load without hard links = %v, want a minted id", err)
	}
	if !minted || backend.ValidateInstance(id) != nil {
		t.Fatalf("Load = (%q, minted %v), want a freshly minted valid id", id, minted)
	}
	again, minted, err := Load(path)
	if err != nil || minted || again != id {
		t.Errorf("reload = (%q, %v, %v), want the same id %q, not minted", again, minted, err, id)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 || entries[0].Name() != FileName {
		t.Errorf("the fallback left %v in the directory, want only %s", entries, FileName)
	}
}

// The fallback keeps the mint's one rule: an id that is already there is never
// replaced. With no hard links and another process's id at the final name,
// the loser adopts it.
func TestTheFallbackMintNeverReplacesAnExistingID(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	const winner = "44444444-4444-4444-8444-444444444444"
	link = func(string, string) error {
		if err := os.WriteFile(path, []byte(winner+"\n"), 0o600); err != nil {
			t.Errorf("write the winner: %v", err)
		}
		return &os.LinkError{Op: "link", Err: errors.New("the file system does not support hard links")}
	}
	t.Cleanup(func() { link = os.Link })

	got, minted, err := Load(path)
	if err != nil || minted || got != winner {
		t.Fatalf("Load = (%q, minted %v, %v), want the winner's %q adopted", got, minted, err, winner)
	}
}

// A fallback write that fails after the exclusive create (a full disk, a
// dropped share) removes the file it made. Left behind, the half-written id
// would make every later start refuse the directory until someone deleted it
// by hand; removed, the next start simply mints.
func TestAFailedFallbackWriteLeavesNothingBehind(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	link = func(string, string) error {
		return &os.LinkError{Op: "link", Err: errors.New("the file system does not support hard links")}
	}
	writeString = func(*os.File, string) (int, error) { return 0, errors.New("no space left on device") }
	t.Cleanup(func() { link, writeString = os.Link, (*os.File).WriteString })

	if _, _, err := Load(path); err == nil {
		t.Fatal("Load with a failing write = nil error, want the write's error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("after a failed write, %s: %v, want it gone so the next start can mint", path, err)
	}
	writeString = (*os.File).WriteString
	if id, minted, err := Load(path); err != nil || !minted || backend.ValidateInstance(id) != nil {
		t.Errorf("the next start = (%q, minted %v, %v), want a fresh mint", id, minted, err)
	}
}
