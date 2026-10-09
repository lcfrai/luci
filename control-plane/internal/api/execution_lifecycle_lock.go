package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

var errExecutionLifecycleBusy = errors.New("This exact worker is still starting or being recovered. Wait for its current operation; no recovery was started.")

// The lock is specific to one account and child. It is held through backend
// Create and all owned cleanup/publication, but never holds the global capacity
// lock while a Docker request is in flight. An OS process exit releases it.
func (g *executionAdmission) lockChildLifecycle(ctx context.Context, childID string) (func(), error) {
	if childID == "" || len(childID) > 200 {
		return nil, errors.New("execution child identity is invalid")
	}
	parent, err := g.prepareExecutionDirectory()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(parent, "children")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := codexRegularPath(parent, dir); err != nil {
		return nil, err
	}
	if err := codexPrivateDir(dir); err != nil {
		return nil, err
	}
	digest := sha256.Sum256([]byte(g.scope + "\x00" + childID))
	path := filepath.Join(dir, hex.EncodeToString(digest[:])+".lock")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("execution lifecycle lock is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			_ = file.Close()
		}
	}()
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("execution lifecycle lock identity changed")
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(opened, info) {
		return nil, errors.New("execution lifecycle lock identity changed")
	}
	if err := hostpath.OneName(path); err != nil {
		return nil, err
	}
	if err := codexPrivateFile(path); err != nil {
		return nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	unlock, err := lockExecutionLedger(bounded, file)
	if err != nil {
		if bounded.Err() != nil {
			return nil, errExecutionLifecycleBusy
		}
		return nil, err
	}
	failed = false
	var once sync.Once
	return func() { once.Do(func() { unlock(); _ = file.Close() }) }, nil
}

func validFenceIdentity(identity backend.LaunchFenceIdentity) bool {
	return identity.Protocol == backend.LaunchFenceProtocol && identity.EngineID != "" && len(identity.EngineID) <= 200 && identity.InstanceID != "" && len(identity.InstanceID) <= 200 && !strings.ContainsAny(identity.EngineID+identity.InstanceID, "\x00\r\n")
}

func (g *executionAdmission) acquireLaunchLifecycle(ctx context.Context, leaseID string, row registry.Session) (func(), error) {
	release, err := g.lockChildLifecycle(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			release()
		}
	}()
	task, err := g.api.executionRequiresBackendAbsence(row)
	if err != nil {
		return nil, err
	}
	fencer, supported := g.api.deps.Backend.(backend.LaunchFencer)
	if !task || !supported || g.api.deps.Backend.Kind() != backend.KindDocker {
		failed = false
		return release, nil
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	identity, err := fencer.LaunchFenceIdentity(bounded)
	cancel()
	if errors.Is(err, backend.ErrUnsupported) {
		// Ordinary coding remains supported on other transports; this launch
		// must explicitly retain the legacy, non-fence-aware protocol.
		failed = false
		return release, nil
	}
	if err != nil {
		return nil, err
	}
	if !validFenceIdentity(identity) {
		return nil, errors.New("the original execution engine identity could not be verified")
	}
	err = g.mutate(ctx, func(ledger *executionLeaseLedger) error {
		for _, fence := range ledger.Fences {
			if fence.Scope == g.scope && fence.ChildID == row.ID {
				return backend.ErrLaunchFenced
			}
		}
		for i := range ledger.Leases {
			lease := &ledger.Leases[i]
			if lease.Scope != g.scope || lease.ID != leaseID {
				continue
			}
			if lease.State != "allocating" || lease.LaunchGuard != 1 || lease.SessionID != row.ID || lease.SandboxID != row.SandboxID || lease.ProjectID != row.ProjectID || lease.FenceProtocol != 0 {
				return errors.New("the new guarded launch no longer matches its exact reservation")
			}
			lease.FenceProtocol, lease.FenceEngineID, lease.FenceInstanceID = identity.Protocol, identity.EngineID, identity.InstanceID
			lease.UpdatedAt = time.Now().UTC()
			return nil
		}
		return errors.New("the new guarded launch has no exact capacity reservation")
	})
	if err != nil {
		return nil, err
	}
	failed = false
	return release, nil
}
