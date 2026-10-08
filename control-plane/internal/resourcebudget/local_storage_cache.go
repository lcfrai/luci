package resourcebudget

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalStorageObservationCache is an ephemeral private host observation. The
// gateway produces it; workers never turn a missing cache into a native probe.
// It is not a durable authorization and cannot outlive its metric timestamps.
type LocalStorageObservationCache struct {
	Version              int                `json:"version"`
	EngineID             string             `json:"engineId"`
	DaemonRoot           string             `json:"daemonRoot"`
	ConfirmationRevision uint64             `json:"confirmationRevision"`
	ConfirmationSHA256   string             `json:"confirmationSha256"`
	ObservedAt           time.Time          `json:"observedAt"`
	Status               LocalStorageStatus `json:"status"`
	HostStorage          *HostStorageBound  `json:"hostStorage"`
	DaemonDisk           *Disk              `json:"daemonDisk"`
}

var ErrLocalStorageObservation = errors.New("The gateway's private Windows storage observation is missing, changed or stale. Wait for a fresh host observation before new heavy work.")

func LocalStorageObservationPath(confirmationPath string) string {
	if !filepath.IsAbs(confirmationPath) {
		return ""
	}
	return filepath.Join(filepath.Dir(confirmationPath), "windows-storage-observation.json")
}
func LocalStorageConfirmationHash(r LocalStorageConfirmation) string {
	if !r.Valid() {
		return ""
	}
	data, _ := json.Marshal(r)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ReadLocalStorageObservation(path string) (LocalStorageObservationCache, error) {
	var out LocalStorageObservationCache
	if !filepath.IsAbs(path) {
		return out, ErrLocalStorageObservation
	}
	root, e := OpenLocalStorageDirectory(filepath.Dir(path))
	if e != nil {
		return out, e
	}
	defer root.Close()
	name := filepath.Base(path)
	before, e := root.Lstat(name)
	if e != nil {
		return out, e
	}
	if !before.Mode().IsRegular() || before.Size() > 32<<10 {
		return out, ErrLocalStorageObservation
	}
	f, e := root.Open(name)
	if e != nil {
		return out, ErrLocalStorageObservation
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !os.SameFile(before, info) || !localStorageRecordPrivate(f, info) {
		return out, ErrLocalStorageObservation
	}
	decoder := json.NewDecoder(io.LimitReader(f, (32<<10)+1))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&out) != nil || !errors.Is(decoder.Decode(new(any)), io.EOF) {
		return LocalStorageObservationCache{}, ErrLocalStorageObservation
	}
	after, e := root.Lstat(name)
	if e != nil || !os.SameFile(info, after) || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
		return LocalStorageObservationCache{}, ErrLocalStorageObservation
	}
	return out, nil
}

func ValidateLocalStorageObservation(c LocalStorageObservationCache, engine, root string, record LocalStorageConfirmation, now time.Time) error {
	hash := LocalStorageConfirmationHash(record)
	if hash == "" || c.Version != 1 || c.EngineID != engine || c.DaemonRoot != root || c.ConfirmationRevision != record.Revision || c.ConfirmationSHA256 != hash || c.Status.Revision != record.Revision || c.ObservedAt.IsZero() || c.ObservedAt.After(now) || now.Sub(c.ObservedAt) > 5*time.Second || c.HostStorage == nil || !c.HostStorage.Required {
		return ErrLocalStorageObservation
	}
	if c.DaemonDisk != nil && (c.DaemonDisk.ObservedAt.IsZero() || c.DaemonDisk.ObservedAt.After(now) || now.Sub(c.DaemonDisk.ObservedAt) > 5*time.Second) {
		return ErrLocalStorageObservation
	}
	if c.HostStorage.State == "confirmed" {
		if c.Status.State != "confirmed" || c.Status.ObservedAt.IsZero() || c.Status.ObservedAt.After(now) || now.Sub(c.Status.ObservedAt) > 5*time.Second || ValidateHostStorage(c.HostStorage, true, 0, now, 5*time.Second) != nil {
			return ErrLocalStorageObservation
		}
	}
	return nil
}

func WriteLocalStorageObservation(path string, value LocalStorageObservationCache) error {
	if !filepath.IsAbs(path) {
		return ErrLocalStorageObservation
	}
	data, e := json.Marshal(value)
	if e != nil || len(data) > 32<<10 {
		return ErrLocalStorageObservation
	}
	root, e := OpenLocalStorageDirectory(filepath.Dir(path))
	if e != nil {
		return e
	}
	defer root.Close()
	name := ".storage-observation-" + rand.Text() + ".tmp"
	f, e := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer root.Remove(name)
	if e = protectLocalStorageFile(f); e == nil {
		_, e = f.Write(data)
	}
	closed := f.Close()
	if e != nil {
		return e
	}
	if closed != nil {
		return closed
	}
	// Atomic relative publication replaces the old observation without following
	// names outside the pinned directory. This ephemeral cache does not fsync:
	// after a crash, any lost/corrupt/expired observation refuses new work.
	return root.Rename(name, filepath.Base(path))
}
