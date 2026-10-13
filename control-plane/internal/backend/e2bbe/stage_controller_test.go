package e2bbe

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestDurableGuestInputUploadNeverReplaysAfterLostReadback(t *testing.T) {
	f, guestConfig, archive := newGuestFixture(t)
	provider := newProvider(t)
	config, _ := fixtureConfig(t, provider)
	c := openController(t, config)
	r, err := c.Allocate(context.Background(), guestConfig.Record.Request)
	if err != nil {
		t.Fatal(err)
	}
	guestConfig.Record = r
	guestConfig.CurrentRecord = func(ctx context.Context, op string) (Record, error) { return c.record(op) }
	g := openGuest(t, guestConfig)
	g.owner = c
	store := openStore(t, storeConfig(t))
	f.mu.Lock()
	f.lostACK, f.corrupt = true, true
	f.mu.Unlock()
	first, err := c.StageGuestInput(context.Background(), r.Request.Identity.OperationID, g, store, bytes.NewReader(archive))
	if !errors.Is(err, ErrUnknown) || !first.InputUploadRequested || !first.ResourcesHeld {
		t.Fatal("uncertain input was accepted or reservation lost", first, err)
	}
	f.mu.Lock()
	posts := f.posts
	f.corrupt = false
	f.mu.Unlock()
	if posts != 1 {
		t.Fatal("expected one upload", posts)
	}
	confirmed, err := c.StageGuestInput(context.Background(), r.Request.Identity.OperationID, g, store, nil)
	if err != nil || !confirmed.InputUploadRequested || !confirmed.ResourcesHeld {
		t.Fatal("exact readback could not settle upload", confirmed, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.posts != 1 || provider.deletes != 0 || !bytes.Equal(f.input, archive) {
		t.Fatal("input upload replayed, changed or released VM", f.posts, provider.deletes)
	}
}
