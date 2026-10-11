package e2bbe

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
)

// The transport barrier does not finish just because its context is cancelled.
// It models a client operation that still owns cleanup and prevents a timing
// race from looking like proof that controller ownership has been drained.
type heldGuestTransport struct {
	base             http.RoundTripper
	entered, release chan struct{}
	once             sync.Once
}

func (h *heldGuestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method == http.MethodPost && r.URL.Path == "/files" {
		h.once.Do(func() { close(h.entered) })
		<-h.release
	}
	return h.base.RoundTrip(r)
}

func TestControllerCloseRetainsOwnershipUntilGuestUploadDrains(t *testing.T) {
	f, guestConfig, archive := newGuestFixture(t)
	p := newProvider(t)
	p.guestGrant = true
	cfg, _ := fixtureConfig(t, p)
	cfg.RootCAs.AddCert(f.server.Certificate())
	cfg.GuestOrigins = func(string) (string, string, string, error) {
		return f.server.URL, f.server.URL, EnvdVersion, nil
	}
	c := openController(t, cfg)
	r, err := c.Allocate(context.Background(), guestConfig.Record.Request)
	if err != nil {
		t.Fatal(err)
	}
	f.record = r
	g, err := c.Guest(context.Background(), r.Request.Identity.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.Close)
	h := &heldGuestTransport{base: g.files.Transport, entered: make(chan struct{}), release: make(chan struct{})}
	g.files.Transport = h
	var release sync.Once
	unblock := func() { release.Do(func() { close(h.release) }) }
	t.Cleanup(unblock)
	stageDone := make(chan error, 1)
	go func() { _, e := g.StageInput(context.Background(), bytes.NewReader(archive)); stageDone <- e }()
	select {
	case <-h.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not enter controlled barrier")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = c.Close(ctx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("controller reported drained while old upload is held: %v", err)
	}
	if next, e := New(cfg); e == nil {
		t.Error("replacement acquired state ownership while old upload is held")
		_ = next.Close(context.Background())
	}
	unblock()
	select {
	case e := <-stageDone:
		if e == nil {
			t.Error("closed controller accepted upload")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not drain")
	}
	if err = c.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	posts := f.posts
	f.mu.Unlock()
	if posts != 0 {
		t.Errorf("old controller forwarded %d uploads after ownership release", posts)
	}
	next := openController(t, cfg)
	rows, usage := next.Snapshot()
	if len(rows) != 1 || !rows[0].ResourcesHeld || usage.Active != 1 || p.creates != 1 {
		t.Fatal("shutdown replayed or released the exact VM", rows, usage, p.creates)
	}
	settled, err := next.Cancel(context.Background(), r.Request.Identity.OperationID, rows[0].Revision)
	requireAbsent(t, settled, err)
}
