package registry

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteAtomicCreatesTheFileAndItsDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "sessions.json")

	if err := writeAtomic(path, []byte("first\n")); err != nil {
		t.Fatalf("writeAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "first\n" {
		t.Fatalf("content = %q, want %q", got, "first\n")
	}

	// The document carries attach tokens, so the mode matters. On Windows the
	// mode bits are advisory and os.Chmod only moves the read-only attribute,
	// so the assertion is skipped there rather than weakened everywhere.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("mode = %v, want -rw-------", perm)
		}
	}
}

func TestWriteAtomicReplacesAFileThatAlreadyExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	if err := os.WriteFile(path, []byte("old and much longer than the new one\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := writeAtomic(path, []byte("new\n")); err != nil {
		t.Fatalf("writeAtomic over an existing file: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "new\n" {
		t.Fatalf("content = %q, want %q", got, "new\n")
	}
}

func TestWriteAtomicWhileAReaderHoldsTheTargetOpen(t *testing.T) {
	// The two platforms genuinely differ here, so the test says so rather than
	// asserting one of them everywhere. Go's os.Open reaches CreateFile through
	// syscall.Open, which hard-codes FILE_SHARE_READ|FILE_SHARE_WRITE and NOT
	// FILE_SHARE_DELETE (src/syscall/syscall_windows.go), so on Windows any open
	// handle — this one, an editor's, a scanner's — makes MoveFileEx return
	// ERROR_ACCESS_DENIED, and a handle held for the whole window outlives the
	// bounded retry. On POSIX a rename onto an open file is ordinary and the
	// reader simply keeps the old inode.
	//
	// The control plane never trips this on itself: load reads with os.ReadFile,
	// which opens, reads and closes before returning, so no reader is alive
	// across its own write. An external holder can still block a save, and the
	// property both branches pin is the one that matters when it does — the
	// target is never left half-written.
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	reader, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer reader.Close()

	err = writeAtomic(path, []byte("new\n"))

	if runtime.GOOS == "windows" {
		// Fail closed, and say which operation failed.
		if err == nil {
			t.Fatal("writeAtomic succeeded with a reader attached; MoveFileEx cannot replace a file another handle holds open, so either the share flags changed or writeAtomic stopped using os.Rename")
		}
		if !strings.Contains(err.Error(), "registry: replace") {
			t.Fatalf("error = %v, want it wrapped as \"registry: replace <path>\"", err)
		}
		// This is the guarantee the registry lives or dies by: the save was
		// lost, the document was not. A writeAtomic that fell back to writing
		// in place to dodge the error would trade this clean failure for a
		// truncated file.
		got, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read back after the failed replace: %v", readErr)
		}
		if string(got) != "old\n" {
			t.Fatalf("content = %q after a failed replace, want the previous content %q", got, "old\n")
		}
		if n := countTempFiles(t, dir); n != 0 {
			t.Fatalf("%d leftover temp files after a failed write, want 0", n)
		}
		return
	}

	if err != nil {
		t.Fatalf("writeAtomic with a reader attached: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "new\n" {
		t.Fatalf("content = %q, want %q", got, "new\n")
	}
	if n := countTempFiles(t, dir); n != 0 {
		t.Fatalf("%d leftover temp files after a successful write, want 0", n)
	}
}

func TestWriteAtomicLeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")

	for i := 0; i < 3; i++ {
		if err := writeAtomic(path, []byte("round\n")); err != nil {
			t.Fatalf("writeAtomic round %d: %v", i, err)
		}
	}

	if n := countTempFiles(t, dir); n != 0 {
		t.Fatalf("%d leftover temp files after three successful writes, want 0", n)
	}
}

func TestWriteAtomicFailureLeavesThePreviousContentIntact(t *testing.T) {
	// A replace that can never succeed: the target path is an existing,
	// non-empty directory. Renaming a file onto it fails on every OS, which is
	// the only portable way to drive writeAtomic's retry loop to exhaustion.
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "occupant"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed occupant: %v", err)
	}

	err := writeAtomic(path, []byte("new\n"))
	if err == nil {
		t.Fatal("writeAtomic onto an unreplaceable target returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "registry: replace") {
		t.Fatalf("error = %v, want it to name the operation and the path", err)
	}

	// Nothing was clobbered and nothing was left behind.
	if _, statErr := os.Stat(filepath.Join(path, "occupant")); statErr != nil {
		t.Fatalf("the pre-existing content was disturbed: %v", statErr)
	}
	if n := countTempFiles(t, dir); n != 0 {
		t.Fatalf("%d leftover temp files after a failed write, want 0", n)
	}
}

func countTempFiles(t *testing.T, dir string) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".sessions-*.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	return len(matches)
}
