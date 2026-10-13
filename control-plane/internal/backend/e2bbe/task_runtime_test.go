package e2bbe

import (
	"context"
	"errors"
	"io"
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

func TestTaskRuntimeRetiresOnlyExactDurableCapturedOutputAfterReviewFailure(t *testing.T) {
	c, g, process, provider, store, started, _ := preparedCaptureFixture(t)
	runtime := TaskRuntime{Controller: c, Store: store}
	ctx := context.Background()
	observed, err := c.CollectGuestOutput(ctx, started.Request.Identity.OperationID, g, store)
	if err != nil {
		t.Fatal(err)
	}
	tampered := observed
	tampered.Revision++
	if _, err = runtime.RetireCaptured(ctx, tampered); !errors.Is(err, ErrConflict) || provider.deletes != 0 {
		t.Fatal("changed capture identity deleted the guest", err, provider.deletes)
	}
	missing := TaskRuntime{Controller: c, Store: openStore(t, storeConfig(t))}
	if _, err = missing.RetireCaptured(ctx, observed); err == nil || provider.deletes != 0 {
		t.Fatal("undurable output deleted the guest", err, provider.deletes)
	}
	retired, err := runtime.RetireCaptured(ctx, observed)
	if err != nil || retired.State != Absent || retired.ResourcesHeld || provider.deletes != 1 || process.captures != 1 || process.starts != 1 {
		t.Fatal("exact captured VM did not settle without replay", retired, err, provider.deletes, process.captures, process.starts)
	}
	if _, err = store.CopyTransfer(ctx, TransferBinding{Identity: started.Request.Identity,
		SourceDigest: started.Request.SourceDigest, ContextDigest: started.Request.ContextDigest,
		Direction: "output"}, observed.Capture.ArchiveSHA256, io.Discard); err != nil {
		t.Fatal("retirement removed the durable task archive", err)
	}
}
