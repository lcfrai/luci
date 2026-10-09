// Package proxy relays WebSocket traffic between a desktop client and a
// sandbox. It recognizes the upstream Exit discriminant; agent output and
// the remaining frame payload stay opaque.
package proxy

import (
	"context"
	"errors"

	"github.com/coder/websocket"
)

// Hooks observe message sizes and the upstream terminal Exit discriminant.
// They never receive payload bytes or derive meaning from agent output. The
// attach handler uses OnUpstream for LastOutputAt and OnUpstreamExit to request
// an authoritative session observation. A nil function is skipped. Hooks run
// inline and should return promptly.
type Hooks struct {
	OnUpstream func(n int) // sandbox → desktop, i.e. the agent produced output
	OnClient   func(n int) // desktop → sandbox, i.e. the user typed
	// OnUpstreamExit fires once per relay for a binary upstream Exit frame.
	// It conveys no exit payload or success claim.
	OnUpstreamExit func()
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

	copyMessages := func(dst, src *websocket.Conn, observe func(int), observeExit func()) {
		exitObserved := false
		for {
			typ, data, err := src.Read(ctx)
			if err != nil {
				switch websocket.CloseStatus(err) {
				case websocket.StatusNormalClosure:
					// A normal upstream close must cross the relay before the
					// sibling Read is cancelled: coder/websocket closes its
					// underlying connection when that read context ends. Closing
					// only after RelayHooked returns races this cancellation and
					// exposes an abnormal EOF after an intact terminal Exit frame.
					// Payloads remain opaque and peer-provided reasons stay private.
					_ = dst.Close(websocket.StatusNormalClosure, "")
				case websocket.StatusTryAgainLater:
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
			if !exitObserved && observeExit != nil && typ == websocket.MessageBinary && len(data) > 0 && data[0] == 0x03 {
				exitObserved = true
				observeExit()
			}
			if err := dst.Write(ctx, typ, data); err != nil {
				errs <- err
				return
			}
		}
	}

	go copyMessages(upstream, client, h.OnClient, nil)
	go copyMessages(client, upstream, h.OnUpstream, h.OnUpstreamExit)

	err := <-errs
	cancel()
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
