package main

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"sync"
)

// liveProjects swaps the scanner after setup without invalidating attached sessions.
type liveProjects struct {
	mu      sync.RWMutex
	scanner *project.Scanner
	cfg     Config
}

func (p *liveProjects) configure(root string) error {
	cfg := p.cfg
	cfg.ProjectsDir = root
	scanner, err := newScanner(cfg, project.NewExecRunner())
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.scanner = scanner
	p.mu.Unlock()
	return nil
}
func (p *liveProjects) get() *project.Scanner { p.mu.RLock(); defer p.mu.RUnlock(); return p.scanner }
func (p *liveProjects) Projects(ctx context.Context) ([]project.Project, error) {
	return p.get().Projects(ctx)
}
func (p *liveProjects) Branches(ctx context.Context, id string) ([]project.Branch, error) {
	return p.get().Branches(ctx, id)
}
func (p *liveProjects) Worktrees(ctx context.Context, id string) ([]project.Worktree, error) {
	return p.get().Worktrees(ctx, id)
}
func (p *liveProjects) Path(ctx context.Context, id string) (string, error) {
	return p.get().Path(ctx, id)
}
