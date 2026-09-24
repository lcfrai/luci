package statepath

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWriteAtomicCreatesTheFileAndItsDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "sessions.json")

	if err := WriteAtomic(path, []byte("first\n"), Private); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
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

	if err := WriteAtomic(path, []byte("new\n"), Private); err != nil {
		t.Fatalf("WriteAtomic over an existing file: %v", err)
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

	err = WriteAtomic(path, []byte("new\n"), Private)

	if runtime.GOOS == "windows" {
		// Fail closed, and say which operation failed.
		if err == nil {
			t.Fatal("WriteAtomic succeeded with a reader attached; MoveFileEx cannot replace a file another handle holds open, so either the share flags changed or WriteAtomic stopped using os.Rename")
		}
		if !strings.Contains(err.Error(), "statepath: replace") {
			t.Fatalf("error = %v, want it wrapped as \"statepath: replace <path>\"", err)
		}
		// This is the guarantee the registry lives or dies by: the save was
		// lost, the document was not. A WriteAtomic that fell back to writing
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
		t.Fatalf("WriteAtomic with a reader attached: %v", err)
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
		if err := WriteAtomic(path, []byte("round\n"), Private); err != nil {
			t.Fatalf("WriteAtomic round %d: %v", i, err)
		}
	}

	if n := countTempFiles(t, dir); n != 0 {
		t.Fatalf("%d leftover temp files after three successful writes, want 0", n)
	}
}

func TestWriteAtomicFailureLeavesThePreviousContentIntact(t *testing.T) {
	// A replace that can never succeed: the target path is an existing,
	// non-empty directory. Renaming a file onto it fails on every OS, which is
	// the only portable way to drive WriteAtomic's retry loop to exhaustion.
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "occupant"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed occupant: %v", err)
	}

	err := WriteAtomic(path, []byte("new\n"), Private)
	if err == nil {
		t.Fatal("WriteAtomic onto an unreplaceable target returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "statepath: replace") {
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

func TestSharedModeWritesAWorldReadableFile(t *testing.T) {
	// CLAUDE.md is read by the agent inside the sandbox, through a bind mount,
	// as the container's user, which is not guaranteed to be ours. A lift that
	// kept the registry's hard-coded 0o600 would publish a file that user
	// cannot open, which is the defect this mode exists for.
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not honour POSIX permission bits: os.Chmod only moves the read-only attribute")
	}
	root := t.TempDir()
	path := filepath.Join(root, "wt", ".claude", "CLAUDE.md")

	if err := WriteAtomic(path, []byte("# brief\n"), Shared); err != nil {
		t.Fatalf("WriteAtomic(Shared): %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// Exact, not umask-adjusted: the mode is set by os.Chmod on the temp file,
	// and chmod does not consult the umask.
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Fatalf("file mode = %v, want -rw-r--r--", perm)
	}

	// os.MkdirAll does consult the umask, so "created 0o755" means 0o755 less
	// whatever the umask strips. A sibling made with os.Mkdir(0o755) measures
	// exactly that without a syscall.Umask, which does not exist on Windows.
	ref := filepath.Join(root, "reference")
	if err := os.Mkdir(ref, 0o755); err != nil {
		t.Fatalf("mkdir reference: %v", err)
	}
	refInfo, err := os.Stat(ref)
	if err != nil {
		t.Fatalf("stat reference: %v", err)
	}
	want := refInfo.Mode().Perm()
	if want == 0o700 {
		t.Logf("the umask strips group and other bits, so the directory half cannot tell Shared from Private here")
	}
	for _, dir := range []string{filepath.Join(root, "wt"), filepath.Join(root, "wt", ".claude")} {
		dirInfo, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if perm := dirInfo.Mode().Perm(); perm != want {
			t.Fatalf("created directory %s has mode %v, want %v (0o755 under this umask)", dir, perm, want)
		}
	}
}

func TestModePermissions(t *testing.T) {
	// The table the two modes promise, pinned where Windows runs it too: the
	// permission tests above are skipped there, so without this a swapped
	// constant would pass on the developer's machine.
	cases := []struct {
		mode      Mode
		file, dir os.FileMode
	}{
		{Private, 0o600, 0o700},
		{Shared, 0o644, 0o755},
	}
	for _, c := range cases {
		if got := c.mode.FilePerm(); got != c.file {
			t.Errorf("Mode(%d).FilePerm() = %v, want %v", c.mode, got, c.file)
		}
		if got := c.mode.DirPerm(); got != c.dir {
			t.Errorf("Mode(%d).DirPerm() = %v, want %v", c.mode, got, c.dir)
		}
	}
}

func TestTempNameFollowsTheTarget(t *testing.T) {
	// The temp file sits next to its target, so its name is visible wherever
	// the target is: a fixed .sessions-*.json would appear in every worktree
	// that gets a CLAUDE.md. The temp file only exists while WriteAtomic runs
	// and is removed on every path, so it is observed during the retry window
	// (20+40+60+80 ms) that a directory standing at the target forces.
	cases := []struct {
		name, pattern string
	}{
		{"CLAUDE.md", ".CLAUDE-*.md"},
		{"sessions.json", ".sessions-*.json"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, c.name)
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatalf("mkdir target: %v", err)
			}
			if err := os.WriteFile(filepath.Join(path, "occupant"), []byte("x"), 0o600); err != nil {
				t.Fatalf("seed occupant: %v", err)
			}

			done := make(chan error, 1)
			go func() { done <- WriteAtomic(path, []byte("new\n"), Shared) }()

			seen := map[string]bool{}
			var err error
			deadline := time.After(10 * time.Second)
		poll:
			for {
				entries, readErr := os.ReadDir(dir)
				if readErr != nil {
					t.Fatalf("read dir: %v", readErr)
				}
				for _, e := range entries {
					if e.Name() != c.name {
						seen[e.Name()] = true
					}
				}
				select {
				case err = <-done:
					break poll
				case <-deadline:
					t.Fatal("WriteAtomic did not return within 10s")
				default:
					time.Sleep(time.Millisecond)
				}
			}
			if err == nil {
				t.Fatal("WriteAtomic onto a directory returned nil, want an error")
			}

			if len(seen) == 0 {
				t.Fatalf("never saw a temp file next to %s during the retry window", c.name)
			}
			for name := range seen {
				ok, globErr := filepath.Match(c.pattern, name)
				if globErr != nil {
					t.Fatalf("match %q: %v", c.pattern, globErr)
				}
				if !ok {
					t.Fatalf("temp file %q next to %s, want it to match %q", name, c.name, c.pattern)
				}
			}

			// And it is gone once WriteAtomic has given up.
			entries, readErr := os.ReadDir(dir)
			if readErr != nil {
				t.Fatalf("read dir: %v", readErr)
			}
			for _, e := range entries {
				if e.Name() != c.name {
					t.Fatalf("%q left next to %s after the failed write", e.Name(), c.name)
				}
			}
		})
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
