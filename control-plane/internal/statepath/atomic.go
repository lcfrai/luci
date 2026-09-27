package statepath

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// WriteAtomic replaces path with data, creating the directory if it is missing
// and publishing the file only once every byte of it is on disk.
//
// It was registry.writeAtomic (Phase 2) until phase 7 task 05 needed the same
// replace for its own small state file, connection.json. Two copies of a replace
// drift the moment one of them gains a retry, and the one failure this guards
// against - a half-written document - is the one that loses running sessions, so
// the lift is here beside Dir rather than a second copy beside the new caller.
//
// os.Rename on Windows is MoveFileEx with MOVEFILE_REPLACE_EXISTING, so it does
// replace an existing file atomically. What it does NOT do is succeed while
// another process has the target open - Windows Defender and search indexers open
// files briefly and opportunistically, and the failure surfaces as
// ERROR_ACCESS_DENIED ("Access is denied.") on a rename that worked a hundred
// times before, on the developer's machine only, never on Linux CI. A bounded
// retry is the whole fix. Writing in place instead would risk a truncated file.
//
// The file is written 0o600. On Windows that mode is advisory - os.Chmod moves
// only the read-only attribute - and the real boundary there is that state lives
// under the user's own %LOCALAPPDATA%.
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("statepath: create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, tempPattern(path))
	if err != nil {
		return fmt.Errorf("statepath: temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // a no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("statepath: write %s: %w", tmpName, err)
	}
	// Sync before the rename, not after: the rename is what publishes the file,
	// and a rename that beats the data to disk is how a power cut produces a
	// zero-length document.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("statepath: sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("statepath: close %s: %w", tmpName, err)
	}
	// Chmod the temp file, never the published one: the document must never be
	// world-readable, not even for the instant between rename and chmod.
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("statepath: chmod %s: %w", tmpName, err)
	}

	const attempts = 5
	var last error
	for i := 0; i < attempts; i++ {
		if last = os.Rename(tmpName, path); last == nil {
			syncDir(dir)
			return nil
		}
		// Nothing sleeps after the final attempt: there is no sixth rename for
		// it to wait on, and a caller may hold a write lock across this call, so
		// that last 100 ms would be 100 ms of readers blocked on a save already
		// doomed. A transient holder still gets the same 20+40+60+80 ms window
		// to let go.
		if i < attempts-1 {
			time.Sleep(time.Duration(20*(i+1)) * time.Millisecond)
		}
	}
	return fmt.Errorf("statepath: replace %s: %w", path, last)
}

// tempPattern is ".<name>-*<ext>" for "<name><ext>", so the debris a crash
// leaves is named after the document it was becoming, sorts beside it, and is
// recognisable to the loader that must ignore it (registry's own
// TestLoadIgnoresALeftoverTempFile seeds ".sessions-4172.json" and depends on
// this shape).
func tempPattern(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return "." + base[:len(base)-len(ext)] + "-*" + ext
}

// syncDir flushes the directory entry so the rename survives a power cut, and is
// best effort on purpose. Windows has no directory fsync - opening a directory
// handle and calling Sync fails with "Incorrect function." - and on POSIX a
// failure here means the data is written but the entry may not be durable yet,
// which is not worth failing a save the caller has already applied in memory.
func syncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
