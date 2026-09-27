package connection

import (
	"context"
	"sync/atomic"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// Current is a deploy.Deployer that delegates to whatever client Connect installed, or
// returns deploy.ErrNotConnected from every method when none is.
//
// It exists so every other phase 7 package can hold ONE value for the process's life. The
// alternative - handing out the client of the moment - means a package that took it before
// the user connected holds a dead one for ever, and a package that took it before Disconnect
// keeps calling Panacea with a token the user just revoked.
type Current struct {
	p atomic.Pointer[deploy.Deployer]
}

var _ deploy.Deployer = (*Current)(nil)

func (c *Current) set(d deploy.Deployer) {
	if d == nil {
		c.p.Store(nil)
		return
	}
	c.p.Store(&d)
}

func (c *Current) get() deploy.Deployer {
	if p := c.p.Load(); p != nil {
		return *p
	}
	return nil
}

// Connected reports whether a client is installed. The watcher asks before it polls.
func (c *Current) Connected() bool { return c.get() != nil }

func (c *Current) Whoami(ctx context.Context) (deploy.Account, error) {
	d := c.get()
	if d == nil {
		return deploy.Account{}, deploy.ErrNotConnected
	}
	return d.Whoami(ctx)
}

func (c *Current) Projects(ctx context.Context) ([]deploy.Project, error) {
	d := c.get()
	if d == nil {
		return nil, deploy.ErrNotConnected
	}
	return d.Projects(ctx)
}

func (c *Current) Environments(ctx context.Context, p deploy.ProjectRef) ([]deploy.Environment, error) {
	d := c.get()
	if d == nil {
		return nil, deploy.ErrNotConnected
	}
	return d.Environments(ctx, p)
}

func (c *Current) EnvironmentFor(ctx context.Context, p deploy.ProjectRef, branch string) (string, error) {
	d := c.get()
	if d == nil {
		return "", deploy.ErrNotConnected
	}
	return d.EnvironmentFor(ctx, p, branch)
}

func (c *Current) Latest(ctx context.Context, p deploy.ProjectRef) (deploy.Deployment, bool, error) {
	d := c.get()
	if d == nil {
		return deploy.Deployment{}, false, deploy.ErrNotConnected
	}
	return d.Latest(ctx, p)
}

func (c *Current) Deployments(ctx context.Context, p deploy.ProjectRef) ([]deploy.Deployment, error) {
	d := c.get()
	if d == nil {
		return nil, deploy.ErrNotConnected
	}
	return d.Deployments(ctx, p)
}

func (c *Current) Run(ctx context.Context, p deploy.ProjectRef, run string) (deploy.RunProgress, error) {
	d := c.get()
	if d == nil {
		return deploy.RunProgress{}, deploy.ErrNotConnected
	}
	return d.Run(ctx, p, run)
}

func (c *Current) Scans(ctx context.Context, p deploy.ProjectRef, env, deploymentID string) (deploy.ScanReport, error) {
	d := c.get()
	if d == nil {
		return deploy.ScanReport{}, deploy.ErrNotConnected
	}
	return d.Scans(ctx, p, env, deploymentID)
}

func (c *Current) Pods(ctx context.Context, p deploy.ProjectRef, env string) ([]deploy.Pod, error) {
	d := c.get()
	if d == nil {
		return nil, deploy.ErrNotConnected
	}
	return d.Pods(ctx, p, env)
}

func (c *Current) Events(ctx context.Context, p deploy.ProjectRef, env string) ([]deploy.Event, error) {
	d := c.get()
	if d == nil {
		return nil, deploy.ErrNotConnected
	}
	return d.Events(ctx, p, env)
}

func (c *Current) Diagnosis(ctx context.Context, p deploy.ProjectRef, env string) (deploy.Diagnosis, error) {
	d := c.get()
	if d == nil {
		return deploy.Diagnosis{}, deploy.ErrNotConnected
	}
	return d.Diagnosis(ctx, p, env)
}

func (c *Current) Stream(ctx context.Context, p deploy.ProjectRef, kind deploy.StreamKind, id string) (deploy.Stream, error) {
	d := c.get()
	if d == nil {
		return nil, deploy.ErrNotConnected
	}
	return d.Stream(ctx, p, kind, id)
}

// SameEnvironment is the one method that answers when disconnected. A comparison of two names
// needs no credential, and the alternative - a false for every pair - would tell the lens that
// a branch's own environment is not its own.
func (c *Current) SameEnvironment(a, b string) bool {
	if d := c.get(); d != nil {
		return d.SameEnvironment(a, b)
	}
	return a == b
}

func (c *Current) Ship(ctx context.Context, p deploy.ProjectRef, req deploy.ShipRequest) (deploy.ShipResult, error) {
	d := c.get()
	if d == nil {
		return deploy.ShipResult{}, deploy.ErrNotConnected
	}
	return d.Ship(ctx, p, req)
}

func (c *Current) Cancel(ctx context.Context, p deploy.ProjectRef, run string) (deploy.CancelResult, error) {
	d := c.get()
	if d == nil {
		return deploy.CancelResult{}, deploy.ErrNotConnected
	}
	return d.Cancel(ctx, p, run)
}

func (c *Current) Rollback(ctx context.Context, p deploy.ProjectRef, deploymentID string) (deploy.RollbackResult, error) {
	d := c.get()
	if d == nil {
		return deploy.RollbackResult{}, deploy.ErrNotConnected
	}
	return d.Rollback(ctx, p, deploymentID)
}

func (c *Current) AutoDeploy(ctx context.Context, p deploy.ProjectRef, branch string) (deploy.AutoDeploy, error) {
	d := c.get()
	if d == nil {
		return deploy.AutoDeploy{}, deploy.ErrNotConnected
	}
	return d.AutoDeploy(ctx, p, branch)
}

func (c *Current) SetAutoDeploy(ctx context.Context, p deploy.ProjectRef, branch string, enabled bool) (deploy.AutoDeploy, error) {
	d := c.get()
	if d == nil {
		return deploy.AutoDeploy{}, deploy.ErrNotConnected
	}
	return d.SetAutoDeploy(ctx, p, branch, enabled)
}

func (c *Current) Posture(ctx context.Context, p deploy.ProjectRef, branch string) (deploy.Posture, error) {
	d := c.get()
	if d == nil {
		return deploy.Posture{}, deploy.ErrNotConnected
	}
	return d.Posture(ctx, p, branch)
}

func (c *Current) SetPosture(ctx context.Context, p deploy.ProjectRef, branch string, ch deploy.PostureChange) (deploy.PostureResult, error) {
	d := c.get()
	if d == nil {
		return deploy.PostureResult{}, deploy.ErrNotConnected
	}
	return d.SetPosture(ctx, p, branch, ch)
}

func (c *Current) Access(ctx context.Context, p deploy.ProjectRef) ([]deploy.AccessGrant, error) {
	d := c.get()
	if d == nil {
		return nil, deploy.ErrNotConnected
	}
	return d.Access(ctx, p)
}

func (c *Current) Grant(ctx context.Context, p deploy.ProjectRef, g deploy.GrantRequest) error {
	d := c.get()
	if d == nil {
		return deploy.ErrNotConnected
	}
	return d.Grant(ctx, p, g)
}

func (c *Current) Revoke(ctx context.Context, p deploy.ProjectRef, group string) error {
	d := c.get()
	if d == nil {
		return deploy.ErrNotConnected
	}
	return d.Revoke(ctx, p, group)
}

func (c *Current) SetURLPrefix(ctx context.Context, p deploy.ProjectRef, branch, prefix string) (deploy.URLPrefix, error) {
	d := c.get()
	if d == nil {
		return deploy.URLPrefix{}, deploy.ErrNotConnected
	}
	return d.SetURLPrefix(ctx, p, branch, prefix)
}

func (c *Current) ClearURLPrefix(ctx context.Context, p deploy.ProjectRef, branch string) (deploy.URLPrefixCleared, error) {
	d := c.get()
	if d == nil {
		return deploy.URLPrefixCleared{}, deploy.ErrNotConnected
	}
	return d.ClearURLPrefix(ctx, p, branch)
}
