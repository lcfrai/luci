package main

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func (b *recoveringBackend) ObserveResources(ctx context.Context) resourcebudget.ExecutionSnapshot {
	value, err := b.get()
	if err == nil {
		if observer, ok := value.(resourcebudget.ExecutionObserver); ok {
			return observer.ObserveResources(ctx)
		}
	}
	return resourcebudget.ExecutionSnapshot{Status: "unavailable", Scope: "execution-daemon", Unavailable: []resourcebudget.Issue{{Metric: "execution", Reason: "execution runtime resource observation is unavailable"}}}
}
