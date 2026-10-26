package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/api"
	"io"
	"net/http"
	"sync"
	"time"
)

type managedShutdownSignal struct {
	once     sync.Once
	done     chan struct{}
	deadline time.Time
	now      func() time.Time
}

func newManagedShutdownSignal(now func() time.Time) *managedShutdownSignal {
	return &managedShutdownSignal{done: make(chan struct{}), now: now}
}
func (s *managedShutdownSignal) request() {
	s.once.Do(func() { s.deadline = s.now().Add(35 * time.Second); close(s.done) })
}
func finishManagedShutdown(ctx context.Context, owned, httpStop func(context.Context) error) error {
	var ownerErr error
	if owned != nil {
		ownerErr = owned(ctx)
	}
	return errors.Join(ownerErr, httpStop(ctx))
}

func workerManagedModel(w http.ResponseWriter, r *http.Request, a *api.API) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	raw, e := io.ReadAll(io.LimitReader(r.Body, 1025))
	var q struct {
		Boot       string `json:"boot"`
		Generation string `json:"generation"`
		Stop       bool   `json:"stop"`
	}
	if e != nil || len(raw) > 1024 || json.Unmarshal(raw, &q) != nil || len(q.Boot) != 64 || q.Generation == "" {
		http.Error(w, "invalid owned model request", 400)
		return
	}
	if e = a.ManagedModelDependents(r.Context(), q.Boot, q.Generation, q.Stop); e != nil {
		http.Error(w, "dependent work remains or is unknown", 409)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"settled": true})
}
