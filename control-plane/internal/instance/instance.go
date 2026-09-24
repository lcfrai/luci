// Package instance names this control plane. The name is what the
// panacea-app.instance label carries on every sandbox (backend.LabelInstance),
// and it is the whole of how two control planes on one Docker daemon tell
// their sandboxes apart: List and the reaper act only on sandboxes that carry
// this instance's id (task-25-26-reconcile.md item 11).
//
// ONE ID PER STATE DIRECTORY, minted the first time a control plane starts on
// it and read back on every start after. It lives in the state directory
// because that is what a control plane's identity already is: the registry
// that says which sessions exist sits there too, and a restart on the same
// directory must keep ownership of the sandboxes it made before it stopped —
// otherwise its own running sessions become foreign to it and are never
// collected.
//
// It is RANDOM, not a hash of the directory's path. A path hash collides the
// moment two machines use the same default path against one remote daemon
// (two dev boxes both at /home/dev/.local/state/panacea-app), which is the
// defect this label exists to end, and it changes when the same directory is
// reached through a different spelling (a short 8.3 name, a symlink), which
// would disown a running fleet. Minted once and persisted, a UUID has neither
// failure.
package instance

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// FileName is the file the id is kept in, inside the directory cmd/api
// derives from the state directory (statepath.Dir("instance")).
const FileName = "instance-id"

// beforeLink runs between writing the new id to its temporary file and linking
// it into place. It is a no-op outside a test; the race test uses it to make
// another process win the mint at exactly the one moment it can.
var beforeLink = func() {}

// Load returns this state directory's instance id, minting and persisting one
// if there is none yet. minted reports which happened, so the caller can log
// the one start that created an identity.
//
// A file that exists and does not hold a valid id is an ERROR, never replaced.
// The id is ownership: a fresh one would make every sandbox the old one made
// foreign, which means never collected, and its sessions would be reported
// gone while their containers kept running. An operator who means that
// deletes the file.
//
// The mint is crash-safe and race-safe. The id is written to a temporary file
// in the same directory, synced, and then hard-linked to its final name, which
// fails rather than replaces if the name exists: a crash leaves at most a stray
// temporary file and never an empty id file, and a control plane that loses a
// race to another on the same directory reads the winner's id rather than
// overwriting it.
func Load(path string) (id string, minted bool, err error) {
	id, err = read(path)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", false, err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, fmt.Errorf("instance: create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, "."+FileName+"-*")
	if err != nil {
		return "", false, fmt.Errorf("instance: create a temporary file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	id = ids.UUID()
	_, werr := tmp.WriteString(id + "\n")
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		return "", false, fmt.Errorf("instance: write %s: %w", tmpName, err)
	}

	beforeLink()
	if err := os.Link(tmpName, path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			// Another control plane on this directory minted first. Its id is
			// the directory's id now; ours was never visible to anything.
			won, rerr := read(path)
			return won, false, rerr
		}
		return "", false, fmt.Errorf("instance: persist the new instance id at %s: %w", path, err)
	}
	return id, true, nil
}

func read(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		return "", fmt.Errorf("instance: read %s: %w", path, err)
	}
	id := strings.TrimSpace(string(raw))
	if err := backend.ValidateInstance(id); err != nil {
		return "", fmt.Errorf("instance: %s does not hold a valid instance id (%v). It names which sandboxes this control plane owns, so it is never replaced automatically: restore it, or delete it to start a new identity, after which every sandbox the old one made is foreign and has to be removed by hand", path, err)
	}
	return id, nil
}
