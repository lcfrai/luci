// Package proxy relays WebSocket traffic between a desktop client and a
// sandbox. It never inspects payloads; the frame protocol is opaque here.
package proxy

import (
	"context"
	"errors"

	"github.com/coder/websocket"
)

// Relay copies messages between client and upstream until either closes or
// ctx is cancelled. It returns the first error observed.
func Relay(ctx context.Context, client, upstream *websocket.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errs := make(chan error, 2)

	copyMessages := func(dst, src *websocket.Conn) {
		for {
			typ, data, err := src.Read(ctx)
			if err != nil {
				errs <- err
				return
			}
			if err := dst.Write(ctx, typ, data); err != nil {
				errs <- err
				return
			}
		}
	}

	go copyMessages(upstream, client)
	go copyMessages(client, upstream)

	err := <-errs
	cancel()
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
