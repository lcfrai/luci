// Package reaper is the periodic sweep that keeps the registry and the backend
// telling the same story. It is a package rather than a loop inside main for
// two reasons: a reaper inside main cannot be unit-tested, and its rules are
// the ones most likely to destroy a running agent by mistake (contract §1.2
// correction 7).
//
// The first rule is "change nothing". Every destructive rule here is downstream
// of a SUCCESSFUL backend.List; a failed List returns immediately, because
// reading "the daemon is restarting" as "every sandbox is gone" would mark
// every session exited, and exited is terminal (contract §5.6).
//
// ONE SWEEP COSTS ONE registry.Store.List PLUS ONE DISK WRITE PER SESSION IT
// ACTUALLY CHANGES. That matters because FileStore holds its write lock across
// the fsync and the full rename-retry budget — up to about 200 ms when Windows
// refuses a rename (registry/filestore.go:492-505) — while the cockpit polls
// Get and List every two seconds. The lock is taken per Update and released on
// return, never held around a loop, so a concurrent poll waits for at most one
// write and never for the sweep.
//
// A converged system writes NOTHING here: every rule below either aborts its
// Update with errUnchanged, which commits and writes nothing, or is not entered
// at all. It is not free, though, and the difference is worth stating exactly
// in a file that makes a point of this lock: rule 5 calls Update for every
// running or idle session before it can discover there is nothing to change,
// and Update takes the WRITE lock (filestore.go:298) to find that out. With
// PANACEA_APP_IDLE_TIMEOUT set, rule 6 adds one Get per session with a sandbox.
// So the converged cost is one List plus one uncontended lock acquisition per
// session — microseconds, no disk — rather than one read.
package reaper

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// The exit reasons this package writes. They reach the user through
// registry.Session.ExitReason and the session rail, so they are sentences.
const (
	reasonDisappeared = "sandbox disappeared"
	reasonProcessExit = "process exited"
	reasonIdleTimeout = "idle timeout"

	// reasonNonLoopback answers "why did my sandbox vanish". A sandbox whose
	// port was published off 127.0.0.1 is an unauthenticated shell on the LAN
	// (contract §2.4, backend.ErrNonLoopbackPublish); task 5's List keeps it in
	// the slice with NonLoopback set precisely so this sweep can destroy it,
	// and nothing else in the phase will.
	//
	// Contract §9A makes this string load-bearing rather than decorative. An
	// endpoint-less sandbox leaves task 11's readiness watch writing NOTHING —
	// an exit reason on a session that has not exited is a falsehood one field
	// over — so the record sits at bare `starting` until this sweep, and this
	// reason is the only place the user ever learns the cause.
	reasonNonLoopback = "sandbox destroyed: its port was published outside 127.0.0.1"
)

const defaultInterval = 30 * time.Second

// errUnchanged aborts a store.Update without writing. FileStore.Update runs the
// mutate function under its write lock and commits nothing when it returns an
// error (task 7), so this is how a rule that turns out to be a no-op avoids
// rewriting the whole JSON document — and, with it, holding the lock that every
// two-second cockpit poll is waiting on.
var errUnchanged = errors.New("reaper: nothing to change")

// Config is contract §5.3's, field for field, plus two the contract does not
// name. IdleAfter is here rather than on api.Deps because the reaper is what
// derives idle and the API never computes it (Group C ruling 4). CreateGrace is
// decision (f).
type Config struct {
	Store       registry.Store
	Backend     backend.Backend
	IdleAfter   time.Duration
	IdleTimeout time.Duration
	Interval    time.Duration

	// CreateGrace is how long a sandbox or a session record is presumed to be
	// mid-create rather than lost. It guards the two rules that can make an
	// irreversible decision about a create that has not finished:
	//
	//   - Rule 2, the session side (Group C ruling 9). Both POST /v1/sessions
	//     and POST /v1/sessions/{id}/restart write the registry record with
	//     SandboxID set BEFORE they call backend.Create, so for the duration of
	//     that create the id is real and the container is not; marking such a
	//     session exited is terminal and it is starting normally.
	//   - Rule 4, the sandbox side. The same window seen from the other end: a
	//     container that exists before the record naming it has been committed
	//     is, to rule 4, indistinguishable from an orphan.
	//
	// MEASURED FROM UpdatedAt, NOT CreatedAt, and that is the whole of why the
	// rule covers a restart. registry.FileStore pins CreatedAt across Update by
	// design (filestore.go:210) — the reaper and the session rail both order by
	// it — so a session restarted an hour after birth has an hour-old CreatedAt
	// and is outside any grace measured from it, which is to say every real
	// restart. UpdatedAt moves on the write that names the new sandbox, so the
	// window opens exactly when the risk does, for a create and a restart alike.
	//
	// Zero means backend.ReadyTimeout, the budget task 11's readiness watch
	// spends before it sets the status itself. That is the right order of
	// magnitude and NOT a proof of sufficiency: dockerbe starts its ready budget
	// AFTER ContainerCreate and ContainerStart (dockerbe/docker.go:207 vs :219),
	// so a create that spends its whole budget outlives a grace of the same
	// length by the container's start latency. What this covers is the common
	// case — a sandbox that comes up in a second or two — and the residual is
	// the slow-start tail, which is precisely what PANACEA_APP_READY_TIMEOUT
	// exists to let an operator lengthen. Raising it lengthens the grace with
	// it, because they are the same number.
	CreateGrace time.Duration

	Now func() time.Time
	Log *slog.Logger
}

// Reaper sweeps. Sweep is exported so tests drive it directly with a fake clock
// and never sleep.
type Reaper struct {
	store       registry.Store
	be          backend.Backend
	idleAfter   time.Duration
	idleTimeout time.Duration
	interval    time.Duration
	createGrace time.Duration
	now         func() time.Time
	log         *slog.Logger

	mu       sync.RWMutex
	ready    bool
	reported bool
}

func New(cfg Config) *Reaper {
	r := &Reaper{
		store:       cfg.Store,
		be:          cfg.Backend,
		idleAfter:   cfg.IdleAfter,
		idleTimeout: cfg.IdleTimeout,
		interval:    cfg.Interval,
		createGrace: cfg.CreateGrace,
		now:         cfg.Now,
		log:         cfg.Log,
	}
	if r.now == nil {
		r.now = time.Now
	}
	if r.log == nil {
		r.log = slog.Default()
	}
	r.log = r.log.With("component", "reaper")
	// cmd/api clamps the interval to 5s, but New must not panic on a zero it
	// was handed by a test or a future caller: time.NewTicker(0) does.
	if r.interval <= 0 {
		r.interval = defaultInterval
	}
	// A zero grace would make rule 2 terminal the instant task 11 writes the
	// record, which is the defect this field exists to close, so zero means the
	// default rather than "no grace".
	if r.createGrace <= 0 {
		r.createGrace = backend.ReadyTimeout
	}
	return r
}

// BackendReady is what task 10's GET /v1/healthz reports. It is the result of
// the last sweep, not a fresh probe: the health endpoint must not be able to
// hang on a wedged Docker daemon.
func (r *Reaper) BackendReady() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ready
}

// Run sweeps immediately and then every Interval until ctx is done. The
// immediate sweep is contract §5.5 step 5: restart recovery deliberately leaves
// orphans alone, because this is the pass that collects them.
func (r *Reaper) Run(ctx context.Context) {
	r.sweepQuietly(ctx)
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.sweepQuietly(ctx)
		}
	}
}

// sweepQuietly logs a failed sweep at DEBUG. The loud line belongs to the
// ready-state transition instead, which fires once when the daemon goes away
// and once when it comes back: a warning on every failed sweep would put two
// lines a minute in the log for as long as Docker Desktop takes to restart, and
// burying the transition is how an operator stops reading the log.
func (r *Reaper) sweepQuietly(ctx context.Context) {
	if err := r.Sweep(ctx); err != nil {
		r.log.Debug("sweep returned an error", "err", err)
	}
}

// Sweep applies contract §5.3's rules, in order, against one snapshot.
func (r *Reaper) Sweep(ctx context.Context) error {
	// Rule 1: on a failed List, change nothing at all.
	live, err := r.be.List(ctx)
	if err != nil {
		r.markBackendDown(err)
		return fmt.Errorf("reaper: list sandboxes: %w", err)
	}
	// Contract §5.3 step 7, hoisted out of step order on purpose. backendReady
	// answers one question — did the backend answer — and List just did, so
	// every failure below this line is a fact about the registry rather than
	// about Docker. Left at the bottom it would report backendReady=false for a
	// healthy daemon whenever the registry document is unreadable, which sends
	// an operator after the wrong system; the sweep completing is not what the
	// flag means.
	r.markBackendUp()

	sessions, err := r.store.List()
	if err != nil {
		return fmt.Errorf("reaper: list sessions: %w", err)
	}

	bySandbox := make(map[string]backend.Sandbox, len(live))
	for _, sb := range live {
		bySandbox[sb.ID] = sb
	}
	tracked := make(map[string]registry.Session, len(sessions))
	for _, s := range sessions {
		if s.SandboxID != "" {
			tracked[s.SandboxID] = s
		}
	}
	// gone holds what this sweep destroyed. The live snapshot still contains
	// those sandboxes, so without it a later rule would act on something that
	// is not there any more.
	//
	// IT IS DEFENCE IN DEPTH AND NOT A GUARANTEE ANYWHERE, and that is worth
	// saying plainly because the obvious reading is wrong twice over. It does
	// not protect the ExitReason rule 1a writes — markExited refuses to touch a
	// record that is already exited, and that is what makes the reason survive
	// rules 3, 5 and 6 looking at the session again. And it is not what stops
	// rule 4 reaping a destroyed sandbox as an orphan: rule 4 skips NonLoopback
	// outright, which covers the failure path this set cannot see.
	//
	// Keep it anyway. "Do not act on something this sweep destroyed" is the
	// right invariant for a rule added later, and it costs a map.
	gone := make(map[string]bool)
	now := r.now()

	// Rule 1a, task 5's amendment to contract §2.4: a sandbox reported with
	// NonLoopback is published where the LAN can reach it. It is destroyed
	// first, before any rule can decide it is merely running, and it is NOT
	// given the create grace the other two destructive rules get — a sandbox on
	// the LAN is a defect whether or not its session has finished starting.
	for _, sb := range live {
		if !sb.NonLoopback {
			continue
		}
		sess, known := tracked[sb.ID]
		// Ruling 2 asks this log to name the offending address, and
		// backend.Sandbox carries only the boolean. Get is where the address
		// lives: every backend answers a non-loopback id with an error wrapping
		// ErrNonLoopbackPublish that names where it was published, and this is
		// a defect path taken once per exposed sandbox, not a hot one. It has to
		// run BEFORE the Destroy, because afterwards there is nothing to ask.
		where := r.publishedAt(ctx, sb.ID)
		if err := r.be.Destroy(ctx, sb.ID); err != nil {
			r.log.Error("could not destroy a sandbox published outside 127.0.0.1; it is still reachable from the LAN",
				"sandbox", sb.ID, "session", sess.ID, "ref", sb.Ref, "published", where, "err", err)
			continue
		}
		gone[sb.ID] = true
		r.log.Warn("destroyed a sandbox whose port was published outside 127.0.0.1",
			"sandbox", sb.ID, "session", sess.ID, "ref", sb.Ref, "published", where)
		if known {
			r.markExited(sess.ID, reasonNonLoopback)
		}
	}

	// Rule 2: a record whose sandbox is not in the list any more.
	for _, s := range sessions {
		if s.SandboxID == "" || s.Status == registry.StatusExited {
			continue
		}
		// Decision (f) and Group C ruling 9. Task 11 writes SandboxID before it
		// calls backend.Create, so inside the create window this id is real and
		// the container does not exist yet. exited is terminal (contract §5.6),
		// so the one rule that can see a half-built session must be the one
		// that waits. Only starting sessions are spared, and only for
		// CreateGrace: after that a starting session whose sandbox is missing
		// really is lost, and rule 2 collects it on the next sweep.
		//
		// UpdatedAt, not CreatedAt — see Config.CreateGrace. A restart reuses
		// the record, so its CreatedAt is as old as the session and only
		// UpdatedAt moves when the new sandbox id is written.
		if s.Status == registry.StatusStarting && now.Sub(s.UpdatedAt) < r.createGrace {
			continue
		}
		if _, alive := bySandbox[s.SandboxID]; !alive {
			r.markExited(s.ID, reasonDisappeared)
		}
	}

	// Rule 3: the sandbox is there and its process is not.
	for _, s := range sessions {
		if s.SandboxID == "" || s.Status == registry.StatusExited || gone[s.SandboxID] {
			continue
		}
		sb, alive := bySandbox[s.SandboxID]
		if !alive {
			continue
		}
		if sb.State == backend.StateStopped || sb.State == backend.StateGone {
			r.markExited(s.ID, reasonProcessExit)
		}
	}

	// Rule 4: ours, but no record. This is the crash-recovery path, and it is
	// why task 11 writes the registry record BEFORE it creates the container.
	for _, sb := range live {
		if sb.Labels[backend.LabelManaged] != "true" {
			continue
		}
		// Rule 1a owns every NonLoopback sandbox unconditionally, whether its
		// destroy SUCCEEDED or failed, and this line and not the gone set is
		// what says so. gone records successes only, so on the failure path an
		// exposed sandbox that no record claims would otherwise be destroyed a
		// second time here and logged as an orphan — relabelling a live security
		// defect as routine litter, which is the mislabelling ruling 2's
		// distinct messages exist to prevent. Rule 1a retries it next sweep,
		// under its own message.
		//
		// This subsumes gone[sb.ID] for rule 4 — rule 1a is the only thing that
		// writes to gone ahead of here, and everything it writes is NonLoopback
		// — so that term is not also tested here. A guard no input can reach is
		// a guard no test can hold honest.
		if sb.NonLoopback {
			continue
		}
		id := sb.Labels[backend.LabelSandboxID]
		if id != "" {
			if _, known := tracked[id]; known {
				continue
			}
		}
		// Ruling 9's hazard from the sandbox side. POST /v1/sessions writes its
		// record first, so its containers are never orphans — but
		// POST /v1/sessions/{id}/restart mints a fresh sandbox id, calls
		// backend.Create, and writes that id to the record only afterwards, so
		// in between the container is managed, live and matched by nothing.
		// Waiting one sweep costs nothing; destroying costs a running agent. A
		// backend that reports no creation time gets no grace, which is the old
		// behaviour and the safe direction for a container nothing can date.
		if !sb.CreatedAt.IsZero() && now.Sub(sb.CreatedAt) < r.createGrace {
			continue
		}
		why := "no session record"
		if id == "" {
			// Ours and unmappable. Docker labels are immutable, so this cannot
			// be a sandbox that lost its label; it is one created by a build
			// that set LabelManaged and nothing else.
			why = "managed sandbox carries no " + backend.LabelSandboxID + " label"
		}
		gone[sb.ID] = r.destroyOrphan(ctx, sb, why)
	}

	// Rule 5: idle derivation, opt-out via PANACEA_APP_IDLE_AFTER=0.
	if r.idleAfter > 0 {
		for _, s := range sessions {
			if gone[s.SandboxID] {
				continue
			}
			if s.Status == registry.StatusRunning || s.Status == registry.StatusIdle {
				r.deriveIdle(s.ID)
			}
		}
	}

	// Rule 6: idle TTL, opt-IN via PANACEA_APP_IDLE_TIMEOUT. Zero means never,
	// and that is the default on purpose: an agent left thinking overnight must
	// still be there in the morning.
	if r.idleTimeout > 0 {
		for _, s := range sessions {
			if s.SandboxID != "" && !gone[s.SandboxID] {
				r.enforceIdleTTL(ctx, s.ID, gone)
			}
		}
	}

	return nil
}

// publishedAt asks the backend where a sandbox was published, for the one log
// line that has to say so. It returns "" rather than guessing: the address is
// only knowable through the error a backend returns for a non-loopback id
// (dockerbe's offLoopbackError, which names the host and wraps
// ErrNonLoopbackPublish), and an empty attribute is a missing detail where a
// fabricated one would be a lie in a security log.
func (r *Reaper) publishedAt(ctx context.Context, id string) string {
	_, err := r.be.Get(ctx, id)
	if err == nil || !errors.Is(err, backend.ErrNonLoopbackPublish) {
		return ""
	}
	return err.Error()
}

// destroyOrphan reports whether the sandbox is now gone. A failed destroy is a
// warning and a retry next sweep, never an error that stops the loop: the
// reaper and an explicit DELETE race by design, and a reaper that gives up on
// "already gone" stops sweeping everything else too.
func (r *Reaper) destroyOrphan(ctx context.Context, sb backend.Sandbox, why string) bool {
	if err := r.be.Destroy(ctx, sb.ID); err != nil {
		r.log.Warn("could not destroy an orphan sandbox; retrying next sweep",
			"sandbox", sb.ID, "ref", sb.Ref, "why", why, "err", err)
		return false
	}
	r.log.Info("destroyed an orphan sandbox", "sandbox", sb.ID, "ref", sb.Ref, "why", why)
	return true
}

// enforceIdleTTL destroys first and records second. If the destroy fails the
// session stays idle: the process really is still running, and reporting exited
// for a sandbox that is still there is the fabricated status this phase rules
// out.
func (r *Reaper) enforceIdleTTL(ctx context.Context, id string, gone map[string]bool) {
	// Re-read. Rule 5 may have changed this session a moment ago, and the
	// snapshot at the top of the sweep is stale. This is the only Get in the
	// package and it is not a get-then-put: the write below still goes through
	// Update (contract §2.8's concurrency discipline).
	cur, err := r.store.Get(id)
	if err != nil {
		return
	}
	if cur.Status != registry.StatusIdle || cur.SandboxID == "" {
		return
	}
	if r.now().Sub(lastActivity(cur)) <= r.idleTimeout {
		return
	}
	if err := r.be.Destroy(ctx, cur.SandboxID); err != nil {
		r.log.Warn("could not destroy an idle-timed-out sandbox; leaving the session idle",
			"session", id, "sandbox", cur.SandboxID, "err", err)
		return
	}
	gone[cur.SandboxID] = true
	r.log.Info("destroyed a sandbox that exceeded the idle timeout",
		"session", id, "sandbox", cur.SandboxID, "after", r.idleTimeout)
	r.markExited(id, reasonIdleTimeout)
}

func (r *Reaper) deriveIdle(id string) {
	now := r.now()
	_, err := r.store.Update(id, func(s *registry.Session) error {
		silent := now.Sub(lastActivity(*s))
		switch {
		case s.Status == registry.StatusRunning && silent > r.idleAfter:
			s.Status = registry.StatusIdle
		case s.Status == registry.StatusIdle && silent <= r.idleAfter:
			s.Status = registry.StatusRunning
		default:
			return errUnchanged
		}
		return nil
	})
	r.reportUpdate(id, err)
}

func (r *Reaper) markExited(id, reason string) {
	_, err := r.store.Update(id, func(s *registry.Session) error {
		if s.Status == registry.StatusExited {
			return errUnchanged // exited is terminal; the first reason is the true one
		}
		s.Status = registry.StatusExited
		s.ExitReason = reason
		s.AttachCount = 0 // nothing is connected to a process that has ended
		return nil
	})
	r.reportUpdate(id, err)
}

func (r *Reaper) reportUpdate(id string, err error) {
	if err == nil || errors.Is(err, errUnchanged) || errors.Is(err, registry.ErrNotFound) {
		return // ErrNotFound: deleted between the snapshot and now, which is fine
	}
	r.log.Warn("could not update a session", "session", id, "err", err)
}

// lastActivity is when this session was last observably doing something. A
// session that has never produced output has a zero LastOutputAt, and
// now.Sub(zero) is two thousand years: measuring from CreatedAt instead is what
// stops a session going idle on the first sweep after it is created.
func lastActivity(s registry.Session) time.Time {
	if !s.LastOutputAt.IsZero() {
		return s.LastOutputAt
	}
	return s.CreatedAt
}

func (r *Reaper) markBackendDown(err error) {
	r.mu.Lock()
	announce := !r.reported || r.ready
	r.ready, r.reported = false, true
	r.mu.Unlock()
	if announce {
		r.log.Warn("backend is not answering; session statuses are left exactly as they are until it does", "err", err)
	}
}

func (r *Reaper) markBackendUp() {
	r.mu.Lock()
	announce := !r.reported || !r.ready
	r.ready, r.reported = true, true
	r.mu.Unlock()
	if announce {
		r.log.Info("backend is answering")
	}
}
