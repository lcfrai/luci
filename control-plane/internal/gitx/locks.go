package gitx

import (
	"context"
	"sync"
)

// Locks is the ONE lock scope for the project repository's branches (review I7). Every call that
// moves a product branch, or reads one to publish it, holds its project's lock for the whole
// verb: Land and BringIn (task 09), Undo and Revert (task 10), CreateBranch, RenameBranch and
// DeleteBranch (task 11), Fetch and Push (task 12, and task 13's push through it). A push that
// read a tip a land was moving would publish whichever tip it happened to see. The clone
// refresh's other callers take it too (review I4): RefreshBranch, the refresh route (task 09),
// and the lane syncer's tip-change sweep (task 07), which never waits for it: it calls TryLock,
// and a busy project is retried on the next sweep (re-review b06-1 issue 6).
//
// One per process. Task 04 constructs it in cmd/api and hands the same value to every user
// through GitDeps.Locks, so no task builds its own. The key is the project ID, phase 2's
// "p-" + hash of the catalog path. Every caller takes it from the one catalog entry, which has
// one spelling. Two catalog entries for one repository get two locks; compare-and-swap
// update-ref still refuses the double land, and the loser gets branch_moved (task 09).
//
// Lock order: a Locks lock is taken first. Syncer.SyncLane's per-lane lock (task 07) may be taken
// inside it; nothing takes a Locks lock while holding a lane lock.
type Locks struct {
	mu sync.Mutex
	m  map[string]chan struct{} // capacity 1, one per project ID; a token in it means held
}

func (l *Locks) ch(projectID string) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.m == nil {
		l.m = make(map[string]chan struct{})
	}
	c, ok := l.m[projectID]
	if !ok {
		c = make(chan struct{}, 1)
		l.m[projectID] = c
	}
	return c
}

// release returns an idempotent unlock for a held c: only its first call frees the lock, so a
// second call can never free the next holder's hold.
func release(c chan struct{}) func() {
	var once sync.Once
	return func() { once.Do(func() { <-c }) }
}

// Lock waits for projectID's lock, or for ctx. It returns ctx.Err() when ctx ends first, so a
// request the human abandoned stops waiting. unlock is idempotent. The zero Locks is ready.
func (l *Locks) Lock(ctx context.Context, projectID string) (unlock func(), err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c := l.ch(projectID)
	select {
	case c <- struct{}{}:
		return release(c), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// TryLock takes projectID's lock only if it is free now, and never waits: ok is false when
// another holder has it. It is a select with a default case, not Lock with a done ctx, because
// Go's select picks at random between a free lock and a done ctx (re-review b06-1 issue 6).
// The sweep (task 07) is its one caller. unlock is idempotent; it is nil when ok is false.
func (l *Locks) TryLock(projectID string) (unlock func(), ok bool) {
	c := l.ch(projectID)
	select {
	case c <- struct{}{}:
		return release(c), true
	default:
		return nil, false
	}
}
