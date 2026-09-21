package backend

import (
	"context"
	"errors"
	"fmt"
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
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("dial %s: %w", endpoint, ErrUnauthorized)
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
	client := &http.Client{Timeout: 2 * time.Second}
	url := "http://" + endpoint + HealthPath

	var last error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			last = fmt.Errorf("healthz returned %d", resp.StatusCode)
		} else {
			last = err
		}

		// A plain time.Sleep here would keep polling a sandbox nobody is
		// waiting for any more, for up to one interval past the cancel.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(readyPollInterval):
		}
	}
	return fmt.Errorf("sandbox %s never became ready (last: %v): %w", endpoint, last, ErrNotRunning)
}
