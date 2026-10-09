package backend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// DialTimeout bounds the /attach handshake. A sandbox that has answered
// /healthz should upgrade in milliseconds; ten seconds is a generous ceiling
// that still fails fast enough for a UI.
const DialTimeout = 10 * time.Second

// ReadyTimeout is how long a caller waits for a new sandbox to answer
// /healthz. A cold Docker image start on a laptop is seconds; docs/PROGRAM.md
// records Kata cold start as seconds too, which is why warm pools are
// mandatory in Phase 8.
const ReadyTimeout = 60 * time.Second

// readyPollInterval is deliberately short: the difference between a 250ms and
// a 1s poll is the difference between a session that feels instant and one
// that does not.
const readyPollInterval = 250 * time.Millisecond

// Dial opens the sandbox's /attach WebSocket.
//
// endpoint is host:port. token, when non-empty, is sent as
// "Authorization: Bearer <token>"; sandboxd answers 401 without it. The
// returned connection has already had SetReadLimit(MaxMessageBytes) applied —
// this is one of the three sites named in the MaxMessageBytes doc comment, and
// the caller must not have to remember.
func Dial(ctx context.Context, endpoint, token string) (*websocket.Conn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, DialTimeout)
	defer cancel()

	opts := &websocket.DialOptions{HTTPHeader: http.Header{}}
	if token != "" {
		opts.HTTPHeader.Set("Authorization", "Bearer "+token)
	}

	conn, resp, err := websocket.Dial(dialCtx, "ws://"+endpoint+AttachPath, opts)
	if err != nil {
		// Any non-101 handshake response — not just 401 — must map to a
		// sentinel the API layer can switch on with errors.Is: a sandbox
		// mid-startup can answer 404, 500 or 503 before /attach exists at all.
		if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
			if resp.StatusCode == http.StatusUnauthorized {
				return nil, fmt.Errorf("dial %s: %w", endpoint, ErrUnauthorized)
			}
			return nil, fmt.Errorf("dial %s: handshake returned %d: %w", endpoint, resp.StatusCode, ErrNotRunning)
		}
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			return nil, fmt.Errorf("dial %s: %v: %w", endpoint, err, ErrNotRunning)
		}
		return nil, fmt.Errorf("dial %s: %w", endpoint, err)
	}
	conn.SetReadLimit(MaxMessageBytes)
	return conn, nil
}

// WaitReady polls GET http://endpoint/healthz until it answers 200 or the
// timeout expires. /healthz is deliberately unauthenticated, so this needs no
// token.
//
// It exists because a container start returning nil means "the process was
// launched", not "the server is listening". Attaching before that produces a
// connection-refused that looks, in the UI, exactly like a crashed agent.
func WaitReady(ctx context.Context, endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{}
	url := "http://" + endpoint + HealthPath

	var last error
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		// Bound each attempt by whatever is left of timeout, not by a flat 2s:
		// otherwise one slow attempt against a sandbox that accepts the TCP
		// connection and then hangs can run past the caller's own deadline —
		// a 700ms WaitReady call blocking for ~2.7s.
		attemptTimeout := 2 * time.Second
		if remaining < attemptTimeout {
			attemptTimeout = remaining
		}
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			return err
		}
		resp, err := client.Do(req)
		switch {
		case err != nil:
			cancel()
			last = err
		case resp.StatusCode == http.StatusOK:
			resp.Body.Close()
			cancel()
			return nil
		default:
			// Drain and close BEFORE cancelling attemptCtx: cancelling a
			// request's context aborts its in-flight body read (a documented
			// net/http hazard), so draining after cancel would fail
			// intermittently and silently undo the keep-alive fix below on
			// those iterations. Draining lets the connection be reused for
			// the next poll; an undrained body forces a fresh TCP connection
			// per poll, up to ~240 of them across a 60s ReadyTimeout.
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			cancel()
			last = fmt.Errorf("healthz returned %d", resp.StatusCode)
		}

		// A plain time.Sleep here would keep polling a sandbox nobody is
		// waiting for any more, for up to one interval past the cancel. The
		// wait itself is also capped by what is left of the deadline, so a
		// failed attempt right before the deadline does not add a full
		// readyPollInterval on top of it.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(min(readyPollInterval, time.Until(deadline))):
		}
	}
	return fmt.Errorf("sandbox %s never became ready (last: %v): %w", endpoint, last, ErrNotRunning)
}
