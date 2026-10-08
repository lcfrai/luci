package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// The ledger is published by atomic rename, so a bounded read sees one complete
// old or new receipt. This read never creates a directory/lock or rewrites state
// while serving review. An unacknowledged lease conservatively blocks Git use.
func (g *executionAdmission) confirmNoPendingLaunch(ctx context.Context, row registry.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !filepath.IsAbs(g.path) {
		return errors.New("execution state path is not absolute")
	}
	if err := codexRegularPath(filepath.VolumeName(g.path)+string(filepath.Separator), g.path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if _, err := hostpath.ResolveOutside(g.api.projectsDir(), g.path); err != nil {
		return err
	}
	data, err := codexRead(filepath.Dir(g.path), g.path, 2<<20)
	if err != nil {
		return err
	}
	ledger, err := decodeExecutionLedger(data)
	if err != nil {
		return err
	}
	for _, lease := range ledger.Leases {
		if lease.Scope != g.scope || lease.SessionID != row.ID {
			continue
		}
		if lease.ProjectID != row.ProjectID || lease.SandboxID != row.SandboxID || lease.LaunchGuard != 1 || lease.State != "backend-settled" {
			return errors.New("The task's create/start outcome is still unconfirmed. Its capacity and checkout remain held; absence alone cannot rule out a late worker.")
		}
	}
	return ctx.Err()
}
