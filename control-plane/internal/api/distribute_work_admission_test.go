package api

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func TestDistributeWorkAdmissionSerializesManagedWorkersAndRespectsHostReserve(t *testing.T) {
	shared := filepath.Join(t.TempDir(), "execution")
	left, right := sessionsNewEnv(t).api, sessionsNewEnv(t).api
	for _, a := range []*API{left, right} {
		a.deps.ExecutionStateDir = shared
		a.deps.ResourceSnapshot = executionFixtureSnapshot
		a.execution = newExecutionAdmission(a)
	}
	release, err := left.admitDistributeWorkWith(context.Background(), executionFixturePolicy())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	if second, e := right.admitDistributeWorkWith(ctx, executionFixturePolicy()); e == nil {
		second()
		release()
		t.Fatal("second managed worker entered the same host-heavy operation")
	}
	release()
	second, err := right.admitDistributeWorkWith(context.Background(), executionFixturePolicy())
	if err != nil {
		t.Fatal("settled permit did not release", err)
	}
	second()
	left.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		snap := executionFixtureSnapshot()
		available := uint64(500) << 20
		snap.Host.Memory.EffectiveAvailableBytes = &available
		return snap
	}
	if rejected, e := left.admitDistributeWorkWith(context.Background(), executionFixturePolicy()); e == nil {
		rejected()
		t.Fatal("host below policy reserve admitted parsing/export")
	}
	left.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		snap := executionFixtureSnapshot()
		snap.Host.Status = "unavailable"
		return snap
	}
	if rejected, e := left.admitDistributeWorkWith(context.Background(), executionFixturePolicy()); e == nil {
		rejected()
		t.Fatal("unavailable host observation admitted parsing/export")
	}
}
