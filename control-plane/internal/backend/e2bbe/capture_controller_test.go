package e2bbe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func preparedCaptureFixture(t *testing.T) (*Controller, *GuestTransport, *processFixture, *providerFixture, *DirectoryStore, Record, CaptureReceipt) {
	t.Helper()
	c, _, g, process, provider, r, id := observedLaunchFixture(t)
	op := r.Request.Identity.OperationID
	var err error
	r, err = c.ObserveLaunchKernel(context.Background(), op, g)
	if err != nil {
		t.Fatal(err)
	}
	process.guest.mu.Lock()
	process.guest.attachFrames = [][]byte{[]byte("\x03exit status 0")}
	process.guest.mu.Unlock()
	r, err = c.ObserveNativeExit(context.Background(), op, g, "fixture-attach")
	if err != nil {
		t.Fatal(err)
	}
	source := `{"version":1,"source":"main.go"}`
	changes := `{"version":1,"changes":[]}`
	binding := TransferBinding{Identity: r.Request.Identity, SourceDigest: r.Request.SourceDigest, ContextDigest: r.Request.ContextDigest, Direction: "output"}
	var archive bytes.Buffer
	_, digest, err := WriteTransfer(&archive, binding, []Payload{payload("output/source/main.go", "package main"), payload("output/source-manifest.json", source), payload("output/changes.json", changes)})
	if err != nil {
		t.Fatal(err)
	}
	receipt := CaptureReceipt{Version: 1, CaptureID: "capture-one", Identity: id, ArchiveSHA256: digest, ArchiveBytes: int64(archive.Len()), CapturedAt: time.Now().UTC(), SourceManifestSHA256: bytesDigest([]byte(source)), ChangeManifestSHA256: bytesDigest([]byte(changes))}
	body, _ := json.Marshal(receipt)
	process.guest.mu.Lock()
	process.guest.output = archive.Bytes()
	process.guest.captureReceipt = body
	process.guest.mu.Unlock()
	store := openStore(t, storeConfig(t))
	return c, g, process, provider, store, r, receipt
}

func TestCollectedGuestOutputIsDurableAndNeverReinvokesCollector(t *testing.T) {
	c, g, process, provider, store, r, receipt := preparedCaptureFixture(t)
	op := r.Request.Identity.OperationID
	got, err := c.CollectGuestOutput(context.Background(), op, g, store)
	if err != nil || got.Capture != receipt || !got.CaptureRequested || !got.ResourcesHeld || got.State != Allocated {
		t.Fatalf("exact output not retained: %+v %v", got, err)
	}
	var archive bytes.Buffer
	binding := TransferBinding{Identity: r.Request.Identity, SourceDigest: r.Request.SourceDigest, ContextDigest: r.Request.ContextDigest, Direction: "output"}
	if _, err = store.CopyTransfer(context.Background(), binding, receipt.ArchiveSHA256, &archive); err != nil || bytesDigest(archive.Bytes()) != receipt.ArchiveSHA256 {
		t.Fatal("output not durably readable", err)
	}
	again, err := c.CollectGuestOutput(context.Background(), op, g, store)
	if err != nil || again.Capture != receipt || process.captures != 1 || process.starts != 1 || provider.deletes != 0 {
		t.Fatal("capture replayed or released the VM", err, process.captures, process.starts, provider.deletes)
	}
}

func TestLostCollectorReceiptRequiresExactReadbackWithoutReplay(t *testing.T) {
	c, g, process, provider, store, r, _ := preparedCaptureFixture(t)
	op := r.Request.Identity.OperationID
	process.guest.mu.Lock()
	retained := append([]byte(nil), process.guest.captureReceipt...)
	process.guest.captureReceipt = nil
	process.guest.mu.Unlock()
	first, err := c.CollectGuestOutput(context.Background(), op, g, store)
	if !errors.Is(err, ErrUnknown) || !first.CaptureRequested || first.Capture != (CaptureReceipt{}) || !first.ResourcesHeld || process.captures != 1 || provider.deletes != 0 {
		t.Fatal("uncertain collector result was accepted or released", first, err)
	}
	process.guest.mu.Lock()
	process.guest.captureReceipt = retained
	process.guest.mu.Unlock()
	second, err := c.CollectGuestOutput(context.Background(), op, g, store)
	if err != nil || second.Capture == (CaptureReceipt{}) || process.captures != 1 || !second.ResourcesHeld {
		t.Fatal("exact readback replayed collector or lost output", second, err)
	}
}

func TestCaptureReceiptCannotClaimDifferentArchiveLength(t *testing.T) {
	c, g, process, provider, store, r, receipt := preparedCaptureFixture(t)
	receipt.ArchiveBytes++
	body, _ := json.Marshal(receipt)
	process.guest.mu.Lock()
	process.guest.captureReceipt = body
	process.guest.mu.Unlock()
	got, err := c.CollectGuestOutput(context.Background(), r.Request.Identity.OperationID, g, store)
	if !errors.Is(err, ErrUnknown) || got.Capture != (CaptureReceipt{}) || !got.ResourcesHeld || process.captures != 1 || provider.deletes != 0 {
		t.Fatal("false archive length was accepted as durable task output", got, err)
	}
}
