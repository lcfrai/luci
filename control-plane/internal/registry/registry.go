// Package registry holds every session this control plane knows about, and is
// the only state in Phase 2 that outlives the process.
//
// CONCURRENCY DISCIPLINE, AND IT IS NOT OPTIONAL: no caller ever performs a
// Get-then-Put. Three goroutines mutate a session concurrently — the HTTP
// handler, the attach relay writing LastOutputAt, and the reaper — and
// Get-then-Put loses whichever write finishes first. Update is the only mutation
// primitive; the mutate function runs while the store holds its write lock, so
// read-modify-write is atomic by construction. Create exists to add a row, and
// takes no mutate function because a row nobody can see yet has nothing to race
// with.
package registry

import (
	"errors"
	"path/filepath"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath"
)

// ErrNotFound is returned for an unknown session id.
var ErrNotFound = errors.New("registry: no such session")

// ErrExists is returned when Create is given an id that is already taken.
var ErrExists = errors.New("registry: session already exists")

// ErrInvalidID is returned when Create is given an empty id. It is separate
// from ErrExists so the REST surface can answer 400 rather than 409: an empty
// id is a caller defect, not a collision.
var ErrInvalidID = errors.New("registry: invalid session id")

// Store holds every session this control plane knows about.
//
// Sessions cross this boundary by value, and deeply: an implementation clones
// on the way in and on the way out, so a caller can never hold a reference into
// the index. The pointer Update hands to mutate is the store's own working copy
// and must not outlive the call — the store commits that copy when mutate
// returns nil and discards it entirely when mutate returns an error.
//
// List returns sessions sorted by CreatedAt ascending with ID as the tie-break.
// Map order would reshuffle the rail on every two-second poll.
//
// Five methods and no more. The worktree ready flag persists in the same
// document and under the same lock, but it is reached through WorktreeFlagStore
// in filestore.go: every consumer of Store cares about sessions, and a widened
// Store would make each of their test doubles implement worktree methods they
// have no opinion about.
type Store interface {
	Create(s Session) error
	Get(id string) (Session, error)
	List() ([]Session, error)
	Update(id string, mutate func(*Session) error) (Session, error)
	Delete(id string) error
}

// DefaultPath is statepath.Dir("sessions") + "/sessions.json".
//
// It resolves at call time rather than in a package variable, because both the
// tests and the desktop set PANACEA_APP_STATE_DIR after this package has been
// initialised, and a cached value would ignore them.
func DefaultPath() string {
	return filepath.Join(statepath.Dir("sessions"), "sessions.json")
}
