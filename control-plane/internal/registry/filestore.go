package registry

// This file owns persistence: the on-disk document, the rules for reading a bad
// one, and the replace that must not lose a running session. The document holds
// attach tokens, so it is written 0o600. On Windows that mode is advisory —
// os.Chmod moves only the read-only attribute — and the real boundary there is
// that the file lives under the user's own %LOCALAPPDATA%.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

// schemaVersion is the version this build writes and the highest it can read.
const schemaVersion = 1

// ErrSchemaTooNew reports a document written by a newer build. It is fatal on
// purpose: see load.
var ErrSchemaTooNew = errors.New("registry: document was written by a newer version of this app")

// WorktreeFlags is the control plane's own state for one worktree — the part git
// cannot tell it. Today that is one flag; it is a struct rather than a bare bool
// so the next one is an additive change inside schemaVersion 1.
type WorktreeFlags struct {
	// Ready is the user's "this worktree is ready to merge" mark. Nothing derives
	// it; only PATCH .../worktrees/{id} sets it.
	Ready bool `json:"ready"`
}

// WorktreeFlagStore is the narrow view the worktree handler depends on. Store
// stays session-only, as the contract declares it.
type WorktreeFlagStore interface {
	WorktreeFlagsSnapshot() map[string]WorktreeFlags
	SetWorktreeReady(worktreeID string, ready bool) (WorktreeFlags, error)
}

// document is the on-disk shape. One document rather than one file per session:
// sessions number in the tens, their order matters to the rail, and a single
// os.Rename is the only atomic primitive available on both Windows and POSIX.
type document struct {
	SchemaVersion int                      `json:"schemaVersion"`
	SavedAt       time.Time                `json:"savedAt"`
	Sessions      []Session                `json:"sessions"`
	WorktreeFlags map[string]WorktreeFlags `json:"worktreeFlags"`
}

// FileStore is the Phase 2 implementation: an in-memory index that is written
// through to one JSON document on every mutation.
type FileStore struct {
	path string

	mu       sync.RWMutex
	sessions map[string]Session
	order    []string // session ids in creation order; List preserves it

	// flags is the per-worktree state the control plane owns, keyed by worktree
	// id. It lives beside the sessions in the same document and is written by
	// the same atomic replace, so a crash cannot leave the two disagreeing.
	flags map[string]WorktreeFlags
}

// NewFileStore opens the store at path. A missing file is an empty store and no
// error; nothing is written until the first mutation. The contract says "creates
// if absent", which is looser than this: Step 8's missing-file test asserts the
// file is still absent afterwards, because a cockpit that has started no sessions
// should leave nothing behind to explain.
//
// A file that exists but cannot be parsed is renamed to <path>.corrupt-<stamp>
// and an empty store is returned, because refusing to start leaves no cockpit
// and no way to fix it; the rename means nothing is lost. A file from a newer
// schemaVersion is an error — see load.
func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{
		path:     path,
		sessions: make(map[string]Session),
		flags:    make(map[string]WorktreeFlags),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// load fills the in-memory index from the document at s.path. It runs once, from
// NewFileStore, before the store is shared with any goroutine, so it takes no
// lock.
//
// Three outcomes, each chosen rather than fallen into:
//
//   - The file is absent. First run. Not an error (§5.5 step 1), and nothing is
//     created — the caller has not mutated anything yet.
//   - The file is unreadable as a schemaVersion-1 document. It is moved aside to
//     <path>.corrupt-<stamp> and the store starts empty. Refusing to start would
//     leave the user with no cockpit and no way to repair the file; starting
//     empty without the rename would destroy it on the next save. The rename is
//     what makes "start empty" safe.
//   - The file is from the future. Hard error, and the file is left exactly where
//     it is. A newer build has written fields this one would silently drop on its
//     next save, so continuing is the destructive option and stopping is the safe
//     one.
func (s *FileStore) load() error {
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("registry: read %s: %w", s.path, err)
	}

	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return s.quarantine(fmt.Sprintf("unparseable: %v", err))
	}
	if doc.SchemaVersion > schemaVersion {
		return fmt.Errorf("%w: %s is schemaVersion %d, this build understands %d",
			ErrSchemaTooNew, s.path, doc.SchemaVersion, schemaVersion)
	}
	if doc.SchemaVersion < schemaVersion {
		// Version 1 is the first version, so anything lower is an absent field:
		// a truncated write, a hand edit, or a pre-release file. There is no
		// migration to run and never will be, so it takes the corruption path
		// rather than stranding the user behind a fatal error.
		return s.quarantine(fmt.Sprintf("schemaVersion %d, want %d", doc.SchemaVersion, schemaVersion))
	}

	for _, sess := range doc.Sessions {
		if sess.ID == "" {
			continue
		}
		if _, dup := s.sessions[sess.ID]; dup {
			continue
		}
		s.sessions[sess.ID] = sess
		s.order = append(s.order, sess.ID)
	}
	for id, flags := range doc.WorktreeFlags {
		if id == "" {
			continue
		}
		s.flags[id] = flags
	}
	return nil
}

// quarantine moves an unreadable document aside and reports nil, which tells load
// to continue with an empty store. It reports an error only when the move itself
// fails, because continuing then would overwrite the evidence on the first save.
func (s *FileStore) quarantine(reason string) error {
	// The contract spells this suffix RFC3339, and RFC3339 cannot be a Windows
	// filename: it contains colons, ':' opens an alternate data stream, and
	// os.Rename fails with "The filename, directory name, or volume label syntax
	// is incorrect." The stamp uses '-' for the time separators, which keeps it
	// sortable and loses nothing else.
	stamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	aside := s.path + ".corrupt-" + stamp
	for i := 1; ; i++ {
		if _, err := os.Lstat(aside); errors.Is(err, os.ErrNotExist) {
			break
		}
		if i > 9 {
			return fmt.Errorf("registry: %s is unreadable (%s) and every quarantine name for %s is taken",
				s.path, reason, stamp)
		}
		aside = fmt.Sprintf("%s.corrupt-%s.%d", s.path, stamp, i)
	}
	if err := os.Rename(s.path, aside); err != nil {
		return fmt.Errorf("registry: %s is unreadable (%s) and could not be moved aside: %w", s.path, reason, err)
	}
	return nil
}

// The compile-time proof that this type is the Store the rest of the phase
// depends on. It costs nothing at run time and turns a missing or wrongly
// signed method into a build failure here rather than an error in task 11.
var _ Store = (*FileStore)(nil)

// Create adds one session. It is the only way a row enters the store, and it
// takes no mutate function because a row nobody can see yet has nothing to race
// with.
//
// An empty id is ErrInvalidID rather than ErrExists: the map would accept "" as
// a key perfectly well, and task 11 must answer 400 to a caller defect rather
// than 409 to a collision that did not happen.
func (s *FileStore) Create(sess Session) error {
	if sess.ID == "" {
		return ErrInvalidID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, dup := s.sessions[sess.ID]; dup {
		return ErrExists
	}

	// Cloned on the way in: the caller keeps the value it passed, and Args is a
	// slice, so the struct copy alone would leave the caller holding a live
	// pointer into the store's record.
	rec := sess.Clone()

	// The store stamps time, in UTC, because asking every caller to remember is
	// the class of defect Update-only exists to prevent. A caller-supplied
	// CreatedAt is kept — task 13's reaper and the rail both order by it — and
	// only normalised.
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	} else {
		rec.CreatedAt = rec.CreatedAt.UTC()
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = rec.CreatedAt
	} else {
		rec.UpdatedAt = rec.UpdatedAt.UTC()
	}

	// order is what saveLocked writes from, so a Create that updates the map and
	// not the slice persists nothing at all while looking entirely correct in
	// memory until the next restart.
	s.sessions[rec.ID] = rec
	s.order = append(s.order, rec.ID)

	if err := s.saveLocked(); err != nil {
		delete(s.sessions, rec.ID)
		s.order = s.order[:len(s.order)-1]
		return err
	}
	return nil
}

// Get returns one session by id, or ErrNotFound.
//
// The returned value shares nothing with the index: a caller that appends to the
// Args it was handed would otherwise be writing into the store from outside the
// lock, which -race catches only on the unlucky run.
func (s *FileStore) Get(id string) (Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	return sess.Clone(), nil
}

// List returns every session, sorted by CreatedAt ascending with ID as the
// tie-break.
//
// The sort is not a nicety. Go randomises map iteration deliberately, so a List
// that ranged the index would reshuffle the session rail on every two-second
// poll of the cockpit. Sorting here rather than maintaining a sorted structure
// means there is nothing that can fall out of sync with Delete or with a
// hand-edited document.
func (s *FileStore) List() ([]Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, sess.Clone())
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		// Two sessions created inside the same clock tick — which on Windows is
		// about 15 ms wide — would otherwise swap places between polls.
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Update is the only way a session changes, and the reason the Store interface
// has no Put.
//
// mutate runs on a clone of the stored record, under the write lock, and the
// lock is not released between mutate returning and the document being written:
// the whole read-modify-write is one critical section, so the HTTP handler, the
// attach relay writing LastOutputAt and the reaper cannot lose each other's
// writes. mutate must not run against the stored record itself — a mutate that
// fails after changing two fields would leave those two committed.
//
// A failing mutate is returned unwrapped, so errors.Is against the caller's own
// sentinel works, and nothing at all is committed or written.
func (s *FileStore) Update(id string, mutate func(*Session) error) (Session, error) {
	if mutate == nil {
		return Session{}, fmt.Errorf("registry: Update %s: mutate is nil", id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	before, ok := s.sessions[id]
	if !ok {
		return Session{}, ErrNotFound
	}

	working := before.Clone()
	if err := mutate(&working); err != nil {
		return Session{}, err
	}
	if working.ID != before.ID {
		// The id is the map key. Committing this would leave the index keyed by
		// one id and holding a record carrying another, and the next reload
		// would rename the session with nothing to show why.
		return Session{}, fmt.Errorf("registry: Update %s: mutate changed the session id to %q", id, working.ID)
	}

	// The store owns both stamps: CreatedAt is a fact about the past, and
	// UpdatedAt is what the cockpit sorts and ages by, so neither is left to the
	// mutate function to remember.
	working.CreatedAt = before.CreatedAt
	working.UpdatedAt = time.Now().UTC()

	// Clone on commit, and clone again on return: the pointer mutate was handed
	// still exists in the caller's closure, and if either the index or the
	// returned value shared its Args array, a write through it after Update has
	// returned would reach the index without ever reaching the document.
	committed := working.Clone()
	s.sessions[id] = committed

	if err := s.saveLocked(); err != nil {
		s.sessions[id] = before
		return Session{}, err
	}
	return committed.Clone(), nil
}

// Delete removes one session, or reports ErrNotFound.
//
// An unknown id is an error rather than a silent no-op because task 13's reaper
// deletes sessions it believes are gone: without the error, "the reaper is
// deleting the same record every ten seconds" is a condition with no symptom.
func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	before, ok := s.sessions[id]
	if !ok {
		return ErrNotFound
	}

	// A fresh slice rather than append(s.order[:i], s.order[i+1:]...): that
	// shifts in place, which would corrupt the copy this function needs to
	// restore if the save fails. Sessions number in the tens.
	next := make([]string, 0, len(s.order))
	for _, existing := range s.order {
		if existing != id {
			next = append(next, existing)
		}
	}
	prevOrder := s.order
	s.order = next
	delete(s.sessions, id)

	if err := s.saveLocked(); err != nil {
		s.sessions[id] = before
		s.order = prevOrder
		return err
	}
	return nil
}

// saveLocked writes the whole document. THE CALLER HOLDS s.mu FOR WRITING — every
// mutation ends with it, inside the same critical section that
// changed the index, so what lands on disk is exactly what a reader would see in
// memory. It does not take the lock itself; Go's mutex is not reentrant and doing
// so would deadlock Update.
func (s *FileStore) saveLocked() error {
	doc := document{
		SchemaVersion: schemaVersion,
		SavedAt:       time.Now().UTC(),
		Sessions:      make([]Session, 0, len(s.order)),
		WorktreeFlags: make(map[string]WorktreeFlags, len(s.flags)),
	}
	for _, id := range s.order {
		sess, ok := s.sessions[id]
		if !ok {
			continue
		}
		doc.Sessions = append(doc.Sessions, sess)
	}
	for id, flags := range s.flags {
		doc.WorktreeFlags[id] = flags
	}

	// Indented: tens of sessions make the size irrelevant, and the first thing
	// anyone does with a broken registry is open it in an editor.
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: encode %s: %w", s.path, err)
	}
	return writeAtomic(s.path, append(data, '\n'))
}

// WorktreeFlagsSnapshot returns a copy of every flag the registry holds. A copy,
// because the caller ranges over it while other goroutines are mutating the
// store, and handing out the live map is a data race that -race only catches on
// the unlucky run.
func (s *FileStore) WorktreeFlagsSnapshot() map[string]WorktreeFlags {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]WorktreeFlags, len(s.flags))
	for id, flags := range s.flags {
		out[id] = flags
	}
	return out
}

// SetWorktreeReady records the user's ready mark for one worktree. It does not
// verify that the worktree exists: git owns that inventory and the handler has
// already resolved it.
func (s *FileStore) SetWorktreeReady(worktreeID string, ready bool) (WorktreeFlags, error) {
	if worktreeID == "" {
		return WorktreeFlags{}, fmt.Errorf("registry: SetWorktreeReady: empty worktree id")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	before, had := s.flags[worktreeID]
	next := WorktreeFlags{Ready: ready}
	if had && before == next {
		return next, nil // nothing changed; do not rewrite the document
	}

	s.flags[worktreeID] = next
	if err := s.saveLocked(); err != nil {
		// Roll the index back so memory and disk still agree. Without this a
		// full disk leaves the app reporting a flag that no restart will find.
		if had {
			s.flags[worktreeID] = before
		} else {
			delete(s.flags, worktreeID)
		}
		return WorktreeFlags{}, err
	}
	return next, nil
}

// writeAtomic replaces path with data.
//
// os.Rename on Windows is MoveFileEx with MOVEFILE_REPLACE_EXISTING, so it does
// replace an existing file atomically. What it does NOT do is succeed while
// another process has the target open — Windows Defender and search indexers open
// files briefly and opportunistically, and the failure surfaces as
// ERROR_ACCESS_DENIED ("Access is denied.") on a rename that worked a hundred
// times before, on the developer's machine only, never on Linux CI. A bounded
// retry is the whole fix. Writing in place instead would risk a truncated
// registry, which is the one failure that loses running sessions.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("registry: create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".sessions-*.json")
	if err != nil {
		return fmt.Errorf("registry: temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // a no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("registry: write %s: %w", tmpName, err)
	}
	// Sync before the rename, not after: the rename is what publishes the file,
	// and a rename that beats the data to disk is how a power cut produces a
	// zero-length registry.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("registry: sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("registry: close %s: %w", tmpName, err)
	}
	// Chmod the temp file, never the published one: the document must never be
	// world-readable, not even for the instant between rename and chmod.
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("registry: chmod %s: %w", tmpName, err)
	}

	const attempts = 5
	var last error
	for i := 0; i < attempts; i++ {
		if last = os.Rename(tmpName, path); last == nil {
			syncDir(dir)
			return nil
		}
		// Nothing sleeps after the final attempt: there is no sixth rename for
		// it to wait on, and saveLocked runs under the write lock, so that last
		// 100 ms would be 100 ms of Get and List blocked on a save already
		// doomed — while the cockpit polls both every two seconds. A transient
		// holder still gets the same 20+40+60+80 ms window to let go.
		if i < attempts-1 {
			time.Sleep(time.Duration(20*(i+1)) * time.Millisecond)
		}
	}
	return fmt.Errorf("registry: replace %s: %w", path, last)
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
