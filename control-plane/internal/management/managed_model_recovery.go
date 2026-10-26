package management

import (
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"io"
	"os"
	"path/filepath"
)

// Existing execution leases retain uncertain work across API/native failure.
// The manager reads their model binding; only the owning API may reconcile or
// remove one after exact backend absence. It never edits another worker's row.
func unresolvedManagedModelLeases(path string) error {
	info, e := os.Lstat(path)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
		return errors.New("managed model participant history unavailable")
	}
	if e = hostpath.OneName(path); e != nil {
		return e
	}
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(info, opened) {
		return errors.New("managed model participant history changed")
	}
	raw, e := io.ReadAll(io.LimitReader(f, 2<<20+1))
	if e != nil || len(raw) > 2<<20 {
		return errors.New("managed model participant history exceeds bound")
	}
	after, e := f.Stat()
	if e != nil || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
		return errors.New("managed model participant history changed")
	}
	var ledger struct {
		Version int `json:"version"`
		Leases  []struct {
			ID                string                `json:"id"`
			Scope             string                `json:"scope"`
			ManagedLocalModel *managedmodel.Binding `json:"managedLocalModel"`
		} `json:"leases"`
	}
	if json.Unmarshal(raw, &ledger) != nil || ledger.Version != 1 || len(ledger.Leases) > 4096 {
		return errors.New("managed model participant history invalid")
	}
	for _, lease := range ledger.Leases {
		if b := lease.ManagedLocalModel; b != nil {
			if b.LeaseID != lease.ID || b.RuntimeScope != lease.Scope {
				return errors.New("managed model participant identity invalid")
			}
			return errors.New("End or reconcile the previous managed-model session in Luci before starting a replacement.")
		}
	}
	return nil
}
func (m *managedModelManager) checkPreviousParticipants() error {
	return unresolvedManagedModelLeases(filepath.Join(m.gateway.opts.ExecutionStateDir, "leases.json"))
}
