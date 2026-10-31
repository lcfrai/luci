package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/statelease"
)

// This is the existing controller's durable private admission state, not a
// second work queue. It contains no provider tokens, endpoints or attach keys.
type cloudPlacementStore struct {
	root       *os.Root
	lease      *os.File
	dir, scope string
	opening    bool
}

func openCloudPlacementStore(dir, scope string) (*cloudPlacementStore, error) {
	if !filepath.IsAbs(dir) || strings.ContainsRune(dir, 0) {
		return nil, e2bbe.ErrUnsupported
	}
	lease, err := statelease.Acquire(dir, "cloud-placement-v1")
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		lease.Close()
		return nil, err
	}
	store := &cloudPlacementStore{root: root, lease: lease, dir: dir, scope: scope, opening: true}
	journal, err := store.read()
	if err != nil {
		store.Close()
		return nil, err
	}
	if journal.CreatedAt.IsZero() {
		journal.CreatedAt = time.Now().UTC()
		if err = store.write(journal); err != nil {
			store.Close()
			return nil, err
		}
	}
	marker, markerErr := root.OpenFile("initialized", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if markerErr == nil {
		_, markerErr = marker.Write([]byte("cloud-placement-v1\n"))
		if markerErr == nil {
			markerErr = marker.Sync()
		}
		markerErr = errors.Join(markerErr, marker.Close())
	} else if errors.Is(markerErr, os.ErrExist) {
		markerErr = nil
	}
	if markerErr != nil {
		store.Close()
		return nil, markerErr
	}
	store.opening = false
	return store, nil
}
func (s *cloudPlacementStore) Close() error {
	if s == nil {
		return nil
	}
	return errors.Join(s.root.Close(), s.lease.Close())
}
func (s *cloudPlacementStore) read() (cloudPlacementJournal, error) {
	empty := cloudPlacementJournal{Version: 1, Scope: s.scope, Rows: map[string]cloudPlacement{}}
	before, err := s.root.Lstat("placements.json")
	if errors.Is(err, os.ErrNotExist) {
		if !s.opening {
			return empty, e2bbe.ErrUnknown
		}
		if _, markerErr := s.root.Lstat("initialized"); !errors.Is(markerErr, os.ErrNotExist) {
			return empty, e2bbe.ErrUnknown
		}
		return empty, nil
	}
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() > 8<<20 || runtime.GOOS != "windows" && before.Mode().Perm()&0077 != 0 {
		return empty, e2bbe.ErrUnknown
	}
	file, err := s.root.Open("placements.json")
	if err != nil {
		return empty, err
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		file.Close()
		return empty, e2bbe.ErrUnknown
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, (8<<20)+1))
	after, statErr := file.Stat()
	closeErr := file.Close()
	latest, pathErr := s.root.Lstat("placements.json")
	if readErr != nil || statErr != nil || closeErr != nil || pathErr != nil || len(raw) > 8<<20 || int64(len(raw)) != before.Size() || !os.SameFile(before, after) || !os.SameFile(before, latest) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return empty, e2bbe.ErrUnknown
	}
	var journal cloudPlacementJournal
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&journal) != nil || decoder.Decode(new(any)) != io.EOF || journal.Version != 1 || journal.Scope != s.scope || journal.CreatedAt.IsZero() || journal.CreatedAt.After(time.Now().Add(time.Minute)) || journal.Rows == nil || len(journal.Rows) > 8192 {
		return empty, e2bbe.ErrUnknown
	}
	originals := map[string]bool{}
	for id, row := range journal.Rows {
		identity := row.Request.Identity
		if id == "" || id != identity.OperationID || row.QueueID == "" || row.Actor.Local || row.Actor.UserID == "" || row.Actor.ProjectID != identity.ProjectID || row.Actor.UserID != row.Request.BillingUserID || row.Actor.TeamID != row.Request.BillingTeamID || row.RequestDigest != cloudPlacementDigest(row.Request) || len(row.RunInputDigest) != 64 || !cloudHostID(row.HostID) || len(row.HostFingerprint) != 64 || row.Template.ID == "" || row.Template.MemoryMiB < 128 || row.Template.CPUMillis < 1000 || row.CreatedAt.IsZero() || row.CreatedAt.After(time.Now().Add(time.Minute)) || row.NoProviderEffect && !row.Retired || row.Request.EstimatedCostMicros < 1 || row.Request.EstimatedCostMicros > 1<<50 || row.Request.MaxSeconds < 1 || row.Request.MaxSeconds > 86400 {
			return empty, e2bbe.ErrUnknown
		}
		// Multiple child IDs cannot turn one task attempt into multiple placements.
		key := identity.ProjectID + "\x00" + identity.TaskID + "\x00" + identity.AttemptID
		if originals[key] {
			return empty, e2bbe.ErrUnknown
		}
		originals[key] = true
	}
	return journal, nil
}
func (s *cloudPlacementStore) write(journal cloudPlacementJournal) error {
	raw, err := json.Marshal(journal)
	if err != nil || len(raw) > 8<<20 {
		return e2bbe.ErrCapacity
	}
	const next = ".placements-next"
	if info, err := s.root.Lstat(next); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 8<<20 {
			return e2bbe.ErrUnknown
		}
		if err = s.root.Remove(next); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := s.root.OpenFile(next, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	n, err := file.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = file.Sync()
	}
	closed := file.Close()
	if err != nil || closed != nil {
		return errors.Join(err, closed)
	}
	if err = s.root.Rename(next, "placements.json"); err != nil {
		return err
	}
	directory, err := os.Open(s.dir)
	if err != nil {
		return e2bbe.ErrUnknown
	}
	synced := directory.Sync()
	closed = directory.Close()
	if closed != nil || synced != nil && runtime.GOOS != "windows" {
		return e2bbe.ErrUnknown
	}
	return nil
}
