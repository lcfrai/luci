package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Workbook parsing and native document generation share one host-owned permit
// across managed API workers. This uses the same exact file identity and OS
// lock protocol as execution admission, but a separate lock keeps planning
// and pressure-response admission responsive while pure computation runs.
func (a *API) admitDistributeWork(ctx context.Context) (func(), error) {
	if a.deps.Product == nil {
		return nil, errors.New("saved execution policy is unavailable")
	}
	policy, err := a.deps.Product.SwarmSchedulerPolicy()
	if err != nil {
		return nil, err
	}
	return a.admitDistributeWorkWith(ctx, policy)
}

func (a *API) admitDistributeWorkWith(ctx context.Context, policy product.SwarmSchedulerPolicy) (func(), error) {
	root, err := a.execution.prepareExecutionDirectory()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "distribute-work.lock")
	if info, e := os.Lstat(path); e == nil {
		if !info.Mode().IsRegular() || hostpath.OneName(path) != nil {
			return nil, errors.New("distribute admission lock changed identity")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = f.Close()
		}
	}()
	opened, e1 := f.Stat()
	named, e2 := os.Lstat(path)
	if e1 != nil || e2 != nil || !opened.Mode().IsRegular() || !os.SameFile(opened, named) || hostpath.OneName(path) != nil {
		return nil, errors.New("distribute admission lock changed identity")
	}
	if _, err = hostpath.ResolveOutside(a.projectsDir(), path); err != nil {
		return nil, err
	}
	if err = codexPrivateFile(path); err != nil {
		return nil, err
	}
	release, err := lockExecutionLedger(ctx, f)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeOnError {
			release()
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if a.deps.ResourceSnapshot == nil {
		return nil, errors.New("fresh host capacity is unavailable")
	}
	snapshot := a.deps.ResourceSnapshot()
	now := time.Now()
	if ctx.Err() != nil || snapshot.Host.Status != "ready" || !snapshot.Host.Fresh(now, 5*time.Second) || snapshot.Host.Memory.EffectiveAvailableBytes == nil || snapshot.Host.Memory.EffectiveTotalBytes == nil {
		return nil, errors.New("fresh host capacity is unavailable")
	}
	// One job's parser/output buffers are bounded in product; charge 256 MiB
	// in addition to the configured host reserve before admitting it.
	needed := policy.MemoryHeadroomBytes(*snapshot.Host.Memory.EffectiveTotalBytes) + (256 << 20)
	if *snapshot.Host.Memory.EffectiveAvailableBytes < needed {
		return nil, errors.New("host memory reserve is unavailable")
	}
	closeOnError = false
	return func() { release(); _ = f.Close() }, nil
}
