// Package statelease excludes concurrent writers of a Luci state root and, on
// Linux, keeps offline host snapshots separate from live writers.
package statelease

import "errors"

const FileName = ".luci-state.lock"

// ErrInUse means a live process owns the root. Changing the listening address
// does not provide independent state. Never remove the lock file to recover:
// the operating system releases ownership when the owning process exits.
var ErrInUse = errors.New("state is in use by another Luci process or offline recovery operation; stop the existing process or select a separate LUCI_STATE_DIR")
