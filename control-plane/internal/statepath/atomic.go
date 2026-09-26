package statepath

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// WriteAtomic replaces path with data (orchestrator ruling R2: lifted from registry's
// writeAtomic, filestore.go, so the registry and the lane index share one replace).
//
// os.Rename on Windows is MoveFileEx with MOVEFILE_REPLACE_EXISTING, so it does
// replace an existing file atomically. What it does NOT do is succeed while
// another process has the target open — Windows Defender and search indexers open
// files briefly and opportunistically, and the failure surfaces as
// ERROR_ACCESS_DENIED ("Access is denied.") on a rename that worked a hundred
// times before, on the developer's machine only, never on Linux CI. A bounded
// retry is the whole fix. Writing in place instead would risk a truncated
// document, which is the one failure that loses running sessions.
//
// The temp file is ".<stem>-*<ext>" beside path (".sessions-*.json" for sessions.json),
// the name registry used before the lift.
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("statepath: create %s: %w", dir, err)
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	tmp, err := os.CreateTemp(dir, "."+strings.TrimSuffix(base, ext)+"-*"+ext)
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
		// it to wait on, and a caller saving under its write lock would hold that
		// lock 100 ms longer for a save already doomed. A transient holder still
		// gets the same 20+40+60+80 ms window to let go.
		if i < attempts-1 {
			time.Sleep(time.Duration(20*(i+1)) * time.Millisecond)
		}
	}
	return fmt.Errorf("statepath: replace %s: %w", path, last)
}

// syncDir flushes the directory entry so the rename survives a power cut, and is
// best effort on purpose. Windows has no directory fsync — opening a directory
// handle and calling Sync fails with "Incorrect function." — and on POSIX a
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
