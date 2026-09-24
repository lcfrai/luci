package statepath

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Mode is who may read what WriteAtomic writes. A value other than the two
// below is treated as Private: an unknown mode fails closed.
type Mode int

const (
	// Private: file 0o600, created directories 0o700. The session registry.
	Private Mode = iota
	// Shared: file 0o644, created directories 0o755. A file another user on
	// the machine must read - CLAUDE.md, which the agent reads through a bind
	// mount as the sandbox's user, not as ours.
	Shared
)

// FilePerm is the permission WriteAtomic gives the file it publishes.
func (m Mode) FilePerm() os.FileMode {
	if m == Shared {
		return 0o644
	}
	return 0o600
}

// DirPerm is the permission WriteAtomic asks for on a directory it creates.
// os.MkdirAll applies the process umask to it, as it does to any directory.
func (m Mode) DirPerm() os.FileMode {
	if m == Shared {
		return 0o755
	}
	return 0o700
}

// WriteAtomic replaces path with data so that a reader sees either the old
// bytes or the new bytes, never a mixture and never an empty file.
//
// It creates missing parent directories with the mode's directory permission,
// writes a temp file in the same directory named "." + stem + "-*" + ext
// (sessions.json -> .sessions-*.json, CLAUDE.md -> .CLAUDE-*.md), syncs it,
// sets the mode's file permission on the TEMP file (never on the published
// one), and renames it over path. The rename is retried five times with
// 20, 40, 60 and 80 ms between attempts and no sleep after the last, because
// on Windows a scanner holding the target open fails MoveFileEx with
// ERROR_ACCESS_DENIED. Errors are wrapped with "statepath: <op> <path>".
// The temp file is removed on every failure path.
//
// os.Rename on Windows is MoveFileEx with MOVEFILE_REPLACE_EXISTING, so it does
// replace an existing file atomically. What it does NOT do is succeed while
// another process has the target open — Windows Defender and search indexers open
// files briefly and opportunistically, and the failure surfaces as
// ERROR_ACCESS_DENIED ("Access is denied.") on a rename that worked a hundred
// times before, on the developer's machine only, never on Linux CI. A bounded
// retry is the whole fix. Writing in place instead would risk a truncated
// file, which for the session registry is the one failure that loses running
// sessions.
func WriteAtomic(path string, data []byte, mode Mode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, mode.DirPerm()); err != nil {
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
	// zero-length file.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("statepath: sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("statepath: close %s: %w", tmpName, err)
	}
	// Chmod the temp file, never the published one: a Private document must
	// never be world-readable, not even for the instant between rename and
	// chmod, and a Shared one must never be published unreadable.
	if err := os.Chmod(tmpName, mode.FilePerm()); err != nil {
		return fmt.Errorf("statepath: chmod %s: %w", tmpName, err)
	}

	const attempts = 5
	var last error
	for i := 0; i < attempts; i++ {
		if last = rename(tmpName, path); last == nil {
			syncDir(dir)
			return nil
		}
		// Nothing sleeps after the final attempt: there is no sixth rename for
		// it to wait on, and the registry's saveLocked calls this under its
		// write lock, so that last 100 ms would be 100 ms of Get and List
		// blocked on a save already doomed — while the cockpit polls both every
		// two seconds. A transient holder still gets the same 20+40+60+80 ms
		// window to let go.
		if i < attempts-1 {
			sleep(time.Duration(20*(i+1)) * time.Millisecond)
		}
	}
	return fmt.Errorf("statepath: replace %s: %w", path, last)
}

// rename and sleep are os.Rename and time.Sleep. They are variables only so a
// test can hold the target for exactly N attempts and read the backoff, which a
// real file handle and a real clock cannot do without a flake. Nothing outside
// this package's tests assigns them.
var (
	rename = os.Rename
	sleep  = time.Sleep
)

// tempPattern names the temp file after its target, so the debris a crash
// leaves is recognisably that file's: sessions.json -> .sessions-*.json,
// CLAUDE.md -> .CLAUDE-*.md. The leading dot keeps it out of a plain listing.
func tempPattern(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return "." + strings.TrimSuffix(base, ext) + "-*" + ext
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
