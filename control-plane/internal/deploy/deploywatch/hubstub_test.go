package deploywatch_test

// The Hub stand-in for the watcher's tests. hubclient.NewFake() does not exist
// in this tree, so this answers the two calls deploywatch.Releases declares and
// records every query it was asked, which is how "one call, not one per
// project" and "Limit 200, Kind run" are pinned.

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploywatch"
)

var (
	errHubDown     = errors.New("hub: connection refused")
	errHubSignedIn = errors.New("hub: signed out")
)

func classifyHubStub(err error) binding.HubErrorClass {
	if errors.Is(err, errHubSignedIn) {
		return binding.HubSignedOut
	}
	return binding.HubBlip
}

type hubQuery struct {
	projectID string
	q         deploywatch.ReleaseQuery
}

type hubStub struct {
	mu       sync.Mutex
	byProj   map[string][]deploywatch.Release
	queries  []hubQuery
	joins    [][2]string
	err      error
	block    bool
	joinErr  error
	meID     string
	pageSize int
}

func newHubStub() *hubStub {
	return &hubStub{byProj: map[string][]deploywatch.Release{}, meID: "u-me", pageSize: 200}
}

func (h *hubStub) add(projectID string, r deploywatch.Release) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.byProj[projectID] = append(h.byProj[projectID], r)
}

// confirm turns a reservation into a confirmed release with its run, the way
// the Hub's confirm does.
func (h *hubStub) confirm(projectID, releaseID, run string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.byProj[projectID] {
		if h.byProj[projectID][i].ID == releaseID {
			h.byProj[projectID][i].State = "confirmed"
			h.byProj[projectID][i].PipelineRun = run
		}
	}
}

func (h *hubStub) fail(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.err = err
}

func (h *hubStub) Releases(ctx context.Context, projectID string, q deploywatch.ReleaseQuery) (deploywatch.ReleasePage, error) {
	h.mu.Lock()
	h.queries = append(h.queries, hubQuery{projectID: projectID, q: q})
	err, block, rows, size := h.err, h.block, append([]deploywatch.Release(nil), h.byProj[projectID]...), h.pageSize
	h.mu.Unlock()
	if block {
		<-ctx.Done()
		return deploywatch.ReleasePage{}, errHubDown
	}
	if err != nil {
		return deploywatch.ReleasePage{}, err
	}
	if ctx.Err() != nil {
		return deploywatch.ReleasePage{}, errHubDown
	}
	var items []deploywatch.Release
	for _, r := range rows {
		if q.PipelineRun != "" && r.PipelineRun != q.PipelineRun {
			continue
		}
		if q.State != "" && r.State != q.State {
			continue
		}
		if q.Kind != "" && r.Kind != q.Kind {
			continue
		}
		items = append(items, r)
	}
	limit := q.Page.Limit
	if limit <= 0 {
		limit = 50 // the Hub's own default, which is why every query sets one
	}
	if limit > size {
		limit = size
	}
	page := deploywatch.ReleasePage{Items: items}
	if len(items) > limit {
		next := "cursor"
		page.Items, page.Next = items[:limit], &next
	}
	return page, nil
}

func (h *hubStub) JoinDeployment(_ context.Context, releaseID, deploymentID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.joins = append(h.joins, [2]string{releaseID, deploymentID})
	return h.joinErr
}

func (h *hubStub) MeID(context.Context) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.meID, nil
}

func (h *hubStub) seen() []hubQuery {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]hubQuery(nil), h.queries...)
}

func (h *hubStub) joinsMade() [][2]string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([][2]string(nil), h.joins...)
}

// release is the shorthand every Hub test builds rows with.
func release(id, run, state, kind string, reservedAt time.Time, ttl time.Duration, by string) deploywatch.Release {
	return deploywatch.Release{
		ID:           id,
		Kind:         kind,
		State:        state,
		PipelineRun:  run,
		ReservedByID: by,
		ReservedAt:   reservedAt.UTC().Format(time.RFC3339),
		ExpiresAt:    reservedAt.Add(ttl).UTC().Format(time.RFC3339),
	}
}
