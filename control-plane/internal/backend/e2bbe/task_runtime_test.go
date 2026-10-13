package e2bbe

import (
	"context"
	"testing"
)

func TestTaskRuntimeReentryKeepsSingleUpstreamStart(t *testing.T) {
	c, _, _, process, provider, started, _ := observedLaunchFixture(t)
	store := openStore(t, storeConfig(t))
	runtime := TaskRuntime{Controller: c, Store: store}
	got, err := runtime.Start(context.Background(), started.Request, nil, launchSpec())
	if err != nil || got.ProcessPID != started.ProcessPID || process.starts != 1 || provider.deletes != 0 {
		t.Fatal("task runtime replayed acknowledged upstream work", got, err, process.starts, provider.deletes)
	}
}

func TestTaskRuntimeWillNotRetireGuestWhoseOriginalInputIsNotDurable(t *testing.T) {
	c, _, process, provider, store, r, _ := preparedCaptureFixture(t)
	runtime := TaskRuntime{Controller: c, Store: store}
	got, _, err := runtime.Finish(context.Background(), r.Request.Identity.OperationID, "fixture-attach", t.TempDir())
	if err == nil || !got.ResourcesHeld || provider.deletes != 0 || process.captures != 1 {
		t.Fatal("undurable input was treated as reviewed task result", got, err, provider.deletes, process.captures)
	}
}
