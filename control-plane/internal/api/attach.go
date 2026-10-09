package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/proxy"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// lastOutputFlushInterval bounds how often a relay writes LastOutputAt through
// to the registry. Contract §5.4: at most once every two seconds. The registry
// is one JSON document rewritten in full on every mutation, and an agent
// emitting a few hundred frames a second would otherwise rewrite it a few
// hundred times a second.
//
// It is also why the flush is throttled rather than per-message at all: task
// 7's FileStore holds its write lock across the fsync and the whole rename
// retry budget, so Get and List block for the duration of a disk write — and
// the cockpit polls both every two seconds.
const lastOutputFlushInterval = 2 * time.Second

// registerAttach mounts the one route this file serves, on the same guarded
// recorder task 10's registerCatalog and task 11's registerSessions use. api.New
// calls it, on the line task 10 reserved.
//
// It takes *routes and not *http.ServeMux (group-C §1A): registration records
// the pattern so contract §10a's audit walks the real route table.
//
// This is the one GET on the surface that writes. It is allowed to be, by
// contract §10a.1, and only for as long as the ordering that section rests on
// holds — see check 5 in handleAttach.
func (a *API) registerAttach(mux *routes) {
	mux.HandleFunc("GET /v1/sessions/{id}/attach", a.handleAttach)
}

// handleAttach upgrades GET /v1/sessions/{id}/attach and relays the sandbox's
// framed stdio to the desktop.
//
// THE ORDER OF THE CHECKS IS PART OF THE CONTRACT (§3.16). All five run BEFORE
// the upgrade, so a refusal is an HTTP status with a JSON body the desktop can
// render, rather than a close code a browser reports as "connection failed".
//
// AND THIS HANDLER DESTROYS NOTHING. Not on a dropped socket, not on a failed
// upstream dial, not on an off-loopback sandbox. A closing laptop lid must not
// kill a running agent. Destruction belongs to DELETE (task 11) and to the
// reaper (task 13), both of which are deliberate and observable.
func (a *API) handleAttach(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// a.deps.Log is never nil: New defaults it to slog.Default().
	log := a.deps.Log.With("route", "attach", "session", id)

	// 1. Origin. Checked here, with the full origin string, rather than by
	//    coder/websocket's OriginPatterns, which matches the host only and
	//    allows a missing Origin unconditionally (accept.go:208-229). Task 9's
	//    middleware checks it too, in front of the guarded mux; this is not
	//    redundant, because a WebSocket upgrade must not depend on a middleware
	//    that a future route table could mount it without.
	origin := r.Header.Get("Origin")
	if !a.deps.Origin.Allow(origin) {
		log.Warn("attach refused: origin not allowed", "origin", origin)
		httpx.WriteProblem(w, http.StatusForbidden, httpx.CodeForbiddenOrigin,
			"This origin may not reach the control plane.",
			map[string]string{"origin": origin})
		return
	}

	// 2. Host. The DNS-rebinding defence: a page on any domain can point a
	//    name at 127.0.0.1, and its requests then carry that name as Host.
	if !httpx.LoopbackHost(r.Host) {
		log.Warn("attach refused: non-loopback Host", "host", r.Host)
		httpx.WriteProblem(w, http.StatusForbidden, httpx.CodeForbiddenHost,
			"The control plane answers only to a loopback Host header.",
			map[string]string{"host": r.Host})
		return
	}

	// 3. The session.
	sess, err := a.deps.Store.Get(id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeSessionNotFound,
				"No session with that id.", map[string]string{"id": id})
			return
		}
		a.internal(w, r, "read the session registry", err)
		return
	}

	// 4. The sandbox, as the record sees it and as the backend sees it.
	if sess.Status == registry.StatusExited || sess.SandboxID == "" {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeSandboxNotRunning,
			"This session has no running sandbox.",
			map[string]string{"id": id, "status": string(sess.Status)})
		return
	}
	if !a.attachSandboxReady(w, r, log, id, sess.SandboxID) {
		return
	}

	// 5. Is this even an upgrade request?
	//
	//    THIS CHECK IS LOAD-BEARING FOR SECURITY, NOT JUST FOR ERGONOMICS. It is
	//    what lets a writing GET exist on this surface at all: contract §10a.1
	//    makes attach an exception to §10a's no-side-effecting-GET rule, and the
	//    exception is conditional on every write coming AFTER this check and
	//    after check 1. Read §10a.1 before moving anything across this line —
	//    a write hoisted above it is reachable by a page that never sees the
	//    response, and the route audit's entry goes on claiming otherwise.
	//    TestAttachWritesNothingUntilTheOriginAndUpgradeChecksHavePassed fails
	//    if that ordering breaks.
	if !isWebSocketUpgrade(r) {
		w.Header().Set("Upgrade", "websocket")
		httpx.WriteProblem(w, http.StatusUpgradeRequired, httpx.CodeUpgradeRequired,
			"This endpoint is a WebSocket upgrade.", nil)
		return
	}

	// 6. Upgrade, then SITE THREE OF THREE of SetReadLimit. See
	//    backend.MaxMessageBytes: the library's default is 32768 and a replay
	//    frame is 262145. Never a literal here.
	client, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Safe: check 1 above already compared the full Origin string, which
		// is stricter than anything this option can express.
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Warn("attach failed: upgrade", "err", err)
		return
	}
	defer client.CloseNow()
	client.SetReadLimit(backend.MaxMessageBytes)

	a.relayAttached(r, log, client, sess)
}

// attachSandboxReady is check 4's second half. It reports whether the relay
// may proceed, and writes the Problem itself when it may not.
func (a *API) attachSandboxReady(w http.ResponseWriter, r *http.Request, log *slog.Logger, id, sandboxID string) bool {
	sb, err := a.deps.Backend.Get(r.Context(), sandboxID)
	switch {
	case err == nil:
	case errors.Is(err, backend.ErrNonLoopbackPublish):
		// 500, loudly, and DO NOT destroy it here. Destroying from a
		// read-only attach would race the reaper, which the group-C
		// contract's ruling 10 already requires to destroy this sandbox and
		// log it distinctly. Contract §2.2's "and the sandbox is destroyed"
		// was written for Create, where the caller owns what it just made.
		//
		// The code is the sixteenth, non_loopback_publish, and task 9 declares
		// it (ruling 7). Mapping this to sandbox_not_running would be a lie:
		// the sandbox IS running, which is exactly the problem.
		log.Error("attach refused: sandbox port is not bound to 127.0.0.1",
			"sandbox", sandboxID, "err", err)
		httpx.WriteProblem(w, http.StatusInternalServerError, httpx.CodeNonLoopbackPublish,
			"The sandbox port is not bound to 127.0.0.1, so the control plane will not attach to it.",
			map[string]string{"id": id})
		return false
	case errors.Is(err, backend.ErrUnavailable):
		httpx.WriteProblem(w, http.StatusServiceUnavailable, httpx.CodeBackendUnavailable,
			"The sandbox backend is not reachable.", nil)
		return false
	case errors.Is(err, backend.ErrNotFound), errors.Is(err, backend.ErrNotRunning):
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeSandboxNotRunning,
			"This session has no running sandbox.", map[string]string{"id": id})
		return false
	default:
		a.internal(w, r, "look the sandbox up in the backend", err)
		return false
	}

	// StateStarting is allowed through deliberately: create returns 201 before
	// the sandbox answers /healthz, and the cockpit attaches immediately.
	if sb.Endpoint == "" || sb.State == backend.StateStopped || sb.State == backend.StateGone {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeSandboxNotRunning,
			"This session has no running sandbox.",
			map[string]string{"id": id, "state": string(sb.State)})
		return false
	}
	return true
}

// relayAttached owns everything after the upgrade: byte transport, attachment
// bookkeeping, and generation-bound process completion. It never destroys an
// allocation or releases its execution reservation.
func (a *API) relayAttached(r *http.Request, log *slog.Logger, client *websocket.Conn, sess registry.Session) {
	// 7. The upstream dial, AFTER the upgrade because §3.16 orders it so. The
	//    reason is sandboxd's replay frame: a dial that succeeds and is then
	//    discarded because the client never upgraded consumes a scrollback
	//    replay nobody ever reads. The cost is that a failure here can only be
	//    a close code, so make it a specific one.
	upstream, err := a.deps.Backend.Attach(r.Context(), sess.SandboxID, sess.AttachToken)
	if err != nil {
		code, reason := attachCloseStatus(err)
		log.Warn("attach failed: upstream dial", "sandbox", sess.SandboxID, "err", err)
		_ = client.Close(code, reason)
		return
	}
	defer upstream.CloseNow()

	a.addAttachCount(log, sess.ID, +1)
	defer a.addAttachCount(log, sess.ID, -1)

	tracker := newOutputTracker(a.deps.Store, sess.ID, lastOutputFlushInterval, time.Now, log)
	defer tracker.flushFinal()

	// 8. OnClient is deliberately nil. The user typing is not output, and
	//    idle is defined on output only (§5.4).
	terminal := a.newAttachedExitObserver(sess)
	// Joining happens after the explicit socket closes below, before final
	// registry bookkeeping. The observer is owned by this attach lifecycle.
	defer terminal.finish()
	relayErr := proxy.RelayHooked(r.Context(), client, upstream, proxy.Hooks{
		OnUpstream: tracker.observe, OnUpstreamExit: terminal.observe,
	})
	if relayErr != nil {
		// A dropped desktop is the normal case, not an incident.
		log.Info("attach relay ended", "err", relayErr)
	}

	if websocket.CloseStatus(relayErr) == websocket.StatusTryAgainLater {
		// Preserve the sandbox's explicit slow-consumer boundary. Turning this
		// into a normal close makes the desktop silently reconnect over lost output.
		_ = client.Close(websocket.StatusTryAgainLater, "terminal output consumer too slow; reconnect for recent output only")
	} else {
		_ = client.Close(websocket.StatusNormalClosure, "")
	}
	_ = upstream.Close(websocket.StatusNormalClosure, "")
	// AND NOTHING IS DESTROYED. See the handler's doc comment.
}

// addAttachCount moves the displayed count of connected clients. It is a
// display value and never an input to status: a session with no client
// attached is still running.
func (a *API) addAttachCount(log *slog.Logger, id string, delta int) {
	_, err := a.deps.Store.Update(id, func(sess *registry.Session) error {
		sess.AttachCount += delta
		if sess.AttachCount < 0 {
			// Below zero means a decrement outlived its increment, which a
			// control-plane restart can produce. Clamp rather than persist a
			// negative the cockpit would render.
			sess.AttachCount = 0
		}
		return nil
	})
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		log.Warn("could not record attachCount", "delta", delta, "err", err)
	}
}

// outputTracker turns "a message crossed the relay" into
// registry.Session.LastOutputAt, at most once every `every`.
//
// It receives sizes, never bytes — see proxy.Hooks. LastOutputAt is the only
// input to the idle derivation in task 13, and idle means exactly "no output
// for N seconds". It says nothing about what the agent is doing or waiting for.
type outputTracker struct {
	store registry.Store
	id    string
	every time.Duration
	now   func() time.Time
	log   *slog.Logger

	mu      sync.Mutex
	last    time.Time // most recent observation
	flushed time.Time // when the last registry write happened
	dirty   bool      // last has moved since the last write
}

func newOutputTracker(store registry.Store, id string, every time.Duration, now func() time.Time, log *slog.Logger) *outputTracker {
	return &outputTracker{store: store, id: id, every: every, now: now, log: log}
}

// observe is proxy.Hooks.OnUpstream. The size argument is ignored on purpose:
// only the fact and the time of output matter, and a tracker that cared how
// much was written would be one step from caring what it said.
//
// The registry write happens inline, on the relay's read goroutine. That
// blocks the relay for one document rewrite, which the throttle bounds to once
// every two seconds; doing it asynchronously would let an older timestamp land
// after a newer one.
func (t *outputTracker) observe(int) {
	t.mu.Lock()
	now := t.now()
	t.last = now
	t.dirty = true
	if !t.flushed.IsZero() && now.Sub(t.flushed) < t.every {
		t.mu.Unlock()
		return
	}
	t.flushed = now
	t.dirty = false
	t.mu.Unlock()

	t.write(now)
}

// flushFinal writes whatever the throttle held back. Contract §5.4's
// "unconditionally" means "not subject to the throttle", not "write even with
// nothing pending": a flush with nothing to say is a no-op, so calling it
// twice costs nothing and the deferred call is always safe.
func (t *outputTracker) flushFinal() {
	t.mu.Lock()
	dirty, stamp := t.dirty, t.last
	if dirty {
		t.dirty = false
		t.flushed = stamp
	}
	t.mu.Unlock()

	if dirty {
		t.write(stamp)
	}
}

func (t *outputTracker) write(stamp time.Time) {
	_, err := t.store.Update(t.id, func(sess *registry.Session) error {
		// Never move the timestamp backwards. Two clients can be attached to
		// one session; the later observation wins.
		if stamp.After(sess.LastOutputAt) {
			sess.LastOutputAt = stamp
		}
		return nil
	})
	// A deleted session during a relay is ordinary — the user pressed delete
	// while watching — so it is not worth a log line.
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		t.log.Warn("could not record lastOutputAt", "err", err)
	}
}

// attachCloseStatus maps a backend failure to a close frame. Reasons are kept
// short deliberately: RFC 6455 caps the close payload at 125 bytes, of which
// two are the code, so a reason over 123 bytes makes Close fail and the socket
// dies with no diagnosis at all.
func attachCloseStatus(err error) (websocket.StatusCode, string) {
	switch {
	case errors.Is(err, backend.ErrUnauthorized):
		return websocket.StatusPolicyViolation, "sandbox rejected the attach token"
	case errors.Is(err, backend.ErrNonLoopbackPublish):
		return websocket.StatusPolicyViolation, "sandbox port is not bound to 127.0.0.1"
	case errors.Is(err, backend.ErrNotFound),
		errors.Is(err, backend.ErrNotRunning),
		errors.Is(err, backend.ErrUnavailable):
		return websocket.StatusTryAgainLater, "sandbox is not accepting connections"
	default:
		return websocket.StatusInternalError, "attach failed"
	}
}

// isWebSocketUpgrade answers check 5 without attempting the upgrade, so the
// refusal is a 426 with a body rather than a failed handshake.
//
// Connection is a comma-separated token list — "keep-alive, Upgrade" is legal
// and common through a proxy — so it cannot be compared whole.
func isWebSocketUpgrade(r *http.Request) bool {
	return attachHeaderHasToken(r.Header, "Connection", "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// attachHeaderHasToken is prefixed for the same reason every other name in
// this task is: four tasks write into package api, and a bare headerHasToken
// is the collision ruling 5 exists to prevent.
func attachHeaderHasToken(h http.Header, key, want string) bool {
	for _, v := range h.Values(key) {
		for _, tok := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(tok), want) {
				return true
			}
		}
	}
	return false
}
