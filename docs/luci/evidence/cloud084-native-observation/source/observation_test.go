package e2bbe

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func observedLaunchFixture(t *testing.T) (*Controller, Config, *GuestTransport, *processFixture, *providerFixture, Record, CaptureIdentity) {
	t.Helper()
	c, cfg, g, p, provider := launchFixture(t)
	r, err := c.StartGuest(context.Background(), p.r.Request.Identity.OperationID, p.r.Revision, launchSpec(), g)
	if err != nil {
		t.Fatal(err)
	}
	id := CaptureIdentity{Version: 1, Identity: r.Request.Identity, RequestDigest: r.RequestDigest, SandboxID: r.SandboxID, LaunchDigest: r.LaunchDigest, ProcessPID: r.ProcessPID, InputBundleDigest: r.Request.BundleDigest, SourceDigest: r.Request.SourceDigest, ContextDigest: r.Request.ContextDigest, BootID: "10000000-0000-0000-0000-000000000001", CgroupRelativePath: "luci-tasks/" + r.Request.Identity.OperationID, CgroupInode: 37}
	b, _ := json.Marshal(id)
	p.guest.mu.Lock()
	p.guest.launchKernel = append(b, '\n')
	p.guest.mu.Unlock()
	return c, cfg, g, p, provider, r, id
}

func TestLaunchKernelAndNativeExitPersistIndependentlyWithoutReleaseOrReplay(t *testing.T) {
	c, cfg, g, p, provider, r, id := observedLaunchFixture(t)
	op := r.Request.Identity.OperationID
	var err error
	r, err = c.ObserveLaunchKernel(context.Background(), op, g)
	if err != nil || r.LaunchKernel != id || !r.ResourcesHeld || r.NativeExit != (NativeExitReceipt{}) {
		t.Fatal(r, err)
	}
	p.guest.mu.Lock()
	p.guest.attachFrames = [][]byte{[]byte("\x00model text exit status 19"), []byte("\x03exit status 0")}
	p.guest.mu.Unlock()
	r, err = c.ObserveNativeExit(context.Background(), op, g, "fixture-attach")
	if err != nil || r.NativeExit.Reason != "exit status 0" || r.LaunchKernel != id || r.State != Allocated || !r.ResourcesHeld {
		t.Fatal(r, err)
	}
	original, err := os.ReadFile(filepath.Join(cfg.StateDir, "attempts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = c.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	c = openController(t, cfg)
	g, err = c.Guest(context.Background(), op)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	got, err := c.ObserveNativeExit(context.Background(), op, g, "fixture-attach")
	after, _ := os.ReadFile(filepath.Join(cfg.StateDir, "attempts.json"))
	if err != nil || got != r || string(original) != string(after) {
		t.Fatal("unchanged native observation mutated history", err)
	}
	p.guest.mu.Lock()
	upgrades := p.guest.upgrades
	p.guest.mu.Unlock()
	if p.starts != 1 || provider.creates != 1 || provider.deletes != 0 || upgrades != 1 {
		t.Fatal("observation replayed or released allocation", p.starts, provider.creates, provider.deletes, upgrades)
	}
}

func TestNativeExitRequiresProtectedLaunchAndRefusesOutputOrDisconnectedStream(t *testing.T) {
	for _, name := range []string{"missing-kernel", "stdout-only", "unknown-frame", "malformed-exit", "wrong-token"} {
		t.Run(name, func(t *testing.T) {
			c, _, g, p, provider, r, _ := observedLaunchFixture(t)
			op := r.Request.Identity.OperationID
			if name != "missing-kernel" {
				var err error
				r, err = c.ObserveLaunchKernel(context.Background(), op, g)
				if err != nil {
					t.Fatal(err)
				}
			}
			frames := [][]byte{[]byte("\x00exit status 0")}
			token := "fixture-attach"
			switch name {
			case "unknown-frame":
				frames = [][]byte{[]byte("\x02exit status 0")}
			case "malformed-exit":
				frames = [][]byte{[]byte("\x03model says exit status 0")}
			case "wrong-token":
				token = "wrong-token"
			}
			p.guest.mu.Lock()
			p.guest.attachFrames = frames
			p.guest.mu.Unlock()
			got, err := c.ObserveNativeExit(context.Background(), op, g, token)
			if err == nil || got.NativeExit != (NativeExitReceipt{}) || !got.ResourcesHeld || got.Revision != r.Revision || provider.deletes != 0 || p.starts != 1 {
				t.Fatal(got, err)
			}
		})
	}
}

func TestProtectedLaunchReadRejectsChangedIdentityAndJSONWithoutMutation(t *testing.T) {
	for _, name := range []string{"PID", "boot-shape", "foreign-operation", "unknown-field", "trailing-value", "oversize"} {
		t.Run(name, func(t *testing.T) {
			c, _, g, p, provider, r, id := observedLaunchFixture(t)
			switch name {
			case "PID":
				id.ProcessPID++
			case "boot-shape":
				id.BootID = "unknown"
			case "foreign-operation":
				id.Identity.OperationID = "other"
				id.CgroupRelativePath = "luci-tasks/other"
			}
			b, _ := json.Marshal(id)
			switch name {
			case "unknown-field":
				b = append(b[:len(b)-1], []byte(`,"quiescent":true}`)...)
			case "trailing-value":
				b = append(b, []byte(`{}`)...)
			case "oversize":
				b = make([]byte, 4097)
			}
			p.guest.mu.Lock()
			p.guest.launchKernel = b
			p.guest.mu.Unlock()
			got, err := c.ObserveLaunchKernel(context.Background(), r.Request.Identity.OperationID, g)
			if err == nil || got.LaunchKernel != (CaptureIdentity{}) || !got.ResourcesHeld || got.Revision != r.Revision || provider.deletes != 0 {
				t.Fatal(got, err)
			}
		})
	}
}

func TestNativeObserverReviewedCancellationWakesAndDeletesOnlyExactVM(t *testing.T) {
	c, _, g, p, provider, r, _ := observedLaunchFixture(t)
	op := r.Request.Identity.OperationID
	r, err := c.ObserveLaunchKernel(context.Background(), op, g)
	if err != nil {
		t.Fatal(err)
	}
	p.guest.mu.Lock()
	p.guest.attachFrames = [][]byte{}
	p.guest.attachHeld = true
	p.guest.attachEntered = make(chan struct{})
	entered := p.guest.attachEntered
	p.guest.mu.Unlock()
	done := make(chan error, 1)
	go func() { _, e := c.ObserveNativeExit(context.Background(), op, g, "fixture-attach"); done <- e }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("observer did not attach")
	}
	asked, err := c.Cancel(context.Background(), op, r.Revision)
	if err != nil || !asked.CancelRequested || !asked.ResourcesHeld {
		t.Fatal(asked, err)
	}
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled observer remained blocked")
	}
	got, _ := c.record(op)
	requireAbsent(t, got, err)
	if got.NativeExit != (NativeExitReceipt{}) || got.LaunchKernel != r.LaunchKernel || provider.creates != 1 || provider.deletes != 1 || p.starts != 1 {
		t.Fatal("cancel manufactured native success or replayed", got)
	}
}

func TestObservationCheckpointFailureRemainsUnconfirmedUntilAcknowledged(t *testing.T) {
	c, cfg, g, p, provider, r, _ := observedLaunchFixture(t)
	op := r.Request.Identity.OperationID
	sink := cfg.Checkpoint.(*checkpointFixture)
	sink.mu.Lock()
	sink.fail = true
	sink.mu.Unlock()
	got, err := c.ObserveLaunchKernel(context.Background(), op, g)
	if !errors.Is(err, ErrUnknown) || got.LaunchKernel == (CaptureIdentity{}) || !got.ResourcesHeld {
		t.Fatal(got, err)
	}
	got, err = c.ObserveLaunchKernel(context.Background(), op, g)
	if !errors.Is(err, ErrUnknown) {
		t.Fatal("unacknowledged checkpoint became successful on reread", got, err)
	}
	sink.mu.Lock()
	sink.fail = false
	sink.mu.Unlock()
	r, err = c.ObserveLaunchKernel(context.Background(), op, g)
	if err != nil {
		t.Fatal(err)
	}
	p.guest.mu.Lock()
	p.guest.attachFrames = [][]byte{[]byte("\x03exit status 17")}
	p.guest.mu.Unlock()
	sink.mu.Lock()
	sink.fail = true
	sink.mu.Unlock()
	got, err = c.ObserveNativeExit(context.Background(), op, g, "fixture-attach")
	if !errors.Is(err, ErrUnknown) || got.NativeExit.Reason != "exit status 17" || !got.ResourcesHeld {
		t.Fatal(got, err)
	}
	if _, err = c.ObserveNativeExit(context.Background(), op, g, "fixture-attach"); !errors.Is(err, ErrUnknown) {
		t.Fatal("unacknowledged native checkpoint upgraded", err)
	}
	sink.mu.Lock()
	sink.fail = false
	sink.mu.Unlock()
	got, err = c.ObserveNativeExit(context.Background(), op, g, "fixture-attach")
	if err != nil || got.NativeExit.Reason != "exit status 17" || !got.ResourcesHeld || provider.deletes != 0 || p.starts != 1 {
		t.Fatal(got, err)
	}
}

func TestObservationCurrentAuthorityAndJournalBindingsRemainRequired(t *testing.T) {
	c, cfg, g, p, _, r, id := observedLaunchFixture(t)
	var denied atomic.Bool
	authorize := func(context.Context, Request, string) error {
		if denied.Load() {
			return backend.ErrUnauthorized
		}
		return nil
	}
	c.config.Authorize = authorize
	g.config.Authorize = authorize
	denied.Store(true)
	if _, err := c.ObserveLaunchKernel(context.Background(), r.Request.Identity.OperationID, g); !errors.Is(err, backend.ErrUnauthorized) {
		t.Fatal(err)
	}
	denied.Store(false)
	r, err := c.ObserveLaunchKernel(context.Background(), r.Request.Identity.OperationID, g)
	if err != nil {
		t.Fatal(err)
	}
	denied.Store(true)
	if _, err = c.ObserveNativeExit(context.Background(), r.Request.Identity.OperationID, g, "fixture-attach"); !errors.Is(err, backend.ErrUnauthorized) {
		t.Fatal(err)
	}
	p.guest.mu.Lock()
	upgrades := p.guest.upgrades
	p.guest.mu.Unlock()
	if upgrades != 0 {
		t.Fatal("revoked observer opened stream")
	}
	b, err := os.ReadFile(filepath.Join(cfg.StateDir, "attempts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var j journal
	if json.Unmarshal(b, &j) != nil {
		t.Fatal("journal fixture")
	}
	row := j.Records[r.Request.Identity.OperationID]
	row.LaunchKernel = id
	row.LaunchKernel.ProcessPID++
	j.Records[r.Request.Identity.OperationID] = row
	b, _ = json.Marshal(j)
	if _, err = validateJournal(b, cfg.OwnerID, cfg.Deployment.Scope); err == nil {
		t.Fatal("changed protected launch accepted on restore")
	}
}
