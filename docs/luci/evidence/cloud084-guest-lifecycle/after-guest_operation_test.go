package e2bbe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"
)

type heldGuestBody struct {
	io.Reader
	entered, release chan struct{}
	once             sync.Once
}

func (b *heldGuestBody) Read(p []byte) (int, error) {
	b.once.Do(func() { close(b.entered); <-b.release })
	return b.Reader.Read(p)
}
func (*heldGuestBody) Close() error { return nil }

type bodyGuestTransport struct{ body io.ReadCloser }

func (t bodyGuestTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: t.body, Header: http.Header{}}, nil
}

func TestGuestCloseWaitsForAcceptedResponseBodyAndRefusesLaterOperations(t *testing.T) {
	_, cfg, _ := newGuestFixture(t)
	g := openGuest(t, cfg)
	store := openStore(t, storeConfig(t))
	binding := transferBinding()
	binding.Direction = "output"
	var archive bytes.Buffer
	_, hash, err := WriteTransfer(&archive, binding, []Payload{payload("output/result.txt", "fixture result")})
	if err != nil {
		t.Fatal(err)
	}
	body := &heldGuestBody{Reader: bytes.NewReader(archive.Bytes()), entered: make(chan struct{}), release: make(chan struct{})}
	g.files.Transport = bodyGuestTransport{body}
	var release sync.Once
	unblock := func() { release.Do(func() { close(body.release) }) }
	t.Cleanup(unblock)
	readDone := make(chan error, 1)
	go func() { _, e := g.ReceiveOutput(context.Background(), hash, store); readDone <- e }()
	select {
	case <-body.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("response body did not enter")
	}
	closed := make(chan struct{})
	go func() { g.Close(); close(closed) }()
	select {
	case <-g.ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("guest close did not cancel")
	}
	select {
	case <-closed:
		t.Error("close certified drain while body reader was held")
	default:
	}
	if _, e := g.StageInput(context.Background(), bytes.NewReader(nil)); !errors.Is(e, ErrClosed) {
		t.Error(e)
	}
	unblock()
	select {
	case e := <-readDone:
		if e == nil {
			t.Error("cancelled output was accepted")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("response did not drain")
	}
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("guest close did not settle")
	}
}

func TestGuestAlreadyCancelledRequestStartsNoIO(t *testing.T) {
	f, cfg, archive := newGuestFixture(t)
	g := openGuest(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := g.StageInput(ctx, bytes.NewReader(archive)); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.posts != 0 || f.gets != 0 || f.upgrades != 0 {
		t.Fatal("cancelled operation performed IO")
	}
}

func TestGuestCloseCancelsPendingStartWithoutReplayOrChargeRelease(t *testing.T) {
	c, _, g, proc, provider := launchFixture(t)
	proc.entered, proc.release = make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(proc.release) })
	started := make(chan error, 1)
	go func() {
		_, e := c.StartGuest(context.Background(), proc.r.Request.Identity.OperationID, proc.r.Revision, launchSpec(), g)
		started <- e
	}()
	select {
	case <-proc.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not enter")
	}
	g.Close()
	select {
	case e := <-started:
		if !errors.Is(e, ErrUnknown) {
			t.Error(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after guest close")
	}
	r, err := c.record(proc.r.Request.Identity.OperationID)
	if err != nil || !r.LaunchRequested || r.ProcessPID != 0 || !r.ResourcesHeld {
		t.Fatal(r, err)
	}
	if _, err = c.StartGuest(context.Background(), r.Request.Identity.OperationID, r.Revision, launchSpec(), g); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	proc.mu.Lock()
	starts := proc.starts
	proc.mu.Unlock()
	_, usage := c.Snapshot()
	if starts != 1 || provider.creates != 1 || provider.deletes != 0 || usage.Active != 1 {
		t.Fatal("unknown Start replayed or released", starts, usage)
	}
	after, err := c.Cancel(context.Background(), r.Request.Identity.OperationID, r.Revision)
	requireAbsent(t, after, err)
}
