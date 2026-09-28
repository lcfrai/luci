package binding_test

// The Hub stand-in. hubclient.NewFake() and hubapi do not exist in this tree,
// so these tests hold binding against the two things it is allowed to know
// about the Hub: an answer (a project id, or "no"), and a CLASS of failure that
// main.go's adapter derives from hubclient's own error - never a status code,
// never a problem body, and never a copy of the Hub's key pattern.

import (
	"context"
	"errors"
	"sync"
	"time"
)

// hubStub answers ProjectForRepo from a key -> project id map, records every
// key it was asked (verbatim, for TestHubKeyIsRepoKeyVerbatim), and can be
// armed to fail with a class.
type hubStub struct {
	mu       sync.Mutex
	byKey    map[string]string
	keysSeen []string
	err      error
	delay    time.Duration
}

func (h *hubStub) ProjectForRepo(ctx context.Context, repoKey string) (string, bool, error) {
	h.mu.Lock()
	h.keysSeen = append(h.keysSeen, repoKey)
	err, byKey, delay := h.err, h.byKey, h.delay
	h.mu.Unlock()
	if err != nil {
		return "", false, err
	}
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return "", false, ctx.Err()
		}
	}
	if ctx.Err() != nil {
		return "", false, ctx.Err()
	}
	id, ok := byKey[repoKey]
	return id, ok, nil
}

func (h *hubStub) seen() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.keysSeen...)
}

func (h *hubStub) fail(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.err = err
}

// The three stand-in Hub errors. main.go's adapter reads hubclient's own types;
// the tests read these, and classifyHubStub is the seam in both directions.
var (
	errHubDown     = errors.New("hub: connection refused")
	errHubBadRepo  = errors.New("hub: 400 field: repo")
	errHubSignedIn = errors.New("hub: signed out")
)
