// Package proxy relays WebSocket traffic between a desktop client and a
// sandbox. It never inspects payloads; the frame protocol is opaque here.
package proxy

import (
	"context"
	"errors"

	"github.com/coder/websocket"
)

// Hooks observe the SIZES of relayed messages. They never receive payload
// bytes, and there is no variant that does: docs/PROGRAM.md forbids deriving
// meaning from agent output absolutely, and a hook holding a []byte is an
// invitation. A nil function is skipped.
//
// The one consumer is the attach handler, which turns OnUpstream into
// registry.Session.LastOutputAt and so into the "idle" status. That is the
// entire mechanism: idle means no output crossed this relay for N seconds, and
// nothing here can tell what the output said.
type Hooks struct {
	OnUpstream func(n int) // sandbox → desktop, i.e. the agent produced output
	OnClient   func(n int) // desktop → sandbox, i.e. the user typed
}

// RelayHooked copies messages between client and upstream until either closes
// or ctx is cancelled, reporting each message's size to h. It returns the first
// error observed. It is the only relay: seam S8.7 in docs/SEAMS.md.
//
// A hook fires after a successful read and before the write, so a message the
// agent produced is counted even when the desktop has already gone away. That
// is deliberate: the timestamp records when the agent last did something, not
// when a socket last succeeded.
func RelayHooked(ctx context.Context, client, upstream *websocket.Conn, h Hooks) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, 2)

	copyMessages := func(dst, src *websocket.Conn, observe func(int)) {
		for {
			typ, data, err := src.Read(ctx)
			if err != nil {
				if websocket.CloseStatus(err) == websocket.StatusTryAgainLater {
					// Canceling the sibling Read closes its connection immediately.
					// Forward this overload boundary first, while the destination
					// can still receive a close frame rather than an unexplained EOF.
					_ = dst.Close(websocket.StatusTryAgainLater, "terminal output consumer too slow; reconnect for recent output only")
				}
				errs <- err
				return
			}
			if observe != nil {
				observe(len(data))
			}
			if err := dst.Write(ctx, typ, data); err != nil {
				errs <- err
				return
			}
		}
	}

	go copyMessages(upstream, client, h.OnClient)
	go copyMessages(client, upstream, h.OnUpstream)

	err := <-errs
	cancel()
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
