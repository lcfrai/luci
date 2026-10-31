package api

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/proxy"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// NEW observer-only bound, not a task deadline. It never renews, extends,
// retries, starts or stops a worker. Tasks completing later remain unknown to
// this live connector; opted-in task containers have independent durable
// original-allocation observation before cleanup, including after restart.
// CreatedAt is the registry's original immutable timestamp.
const taskTerminalObservationLimit = 30 * time.Minute

func taskTerminalObservationDeadline(created, now time.Time) (time.Time, bool) {
	if created.IsZero() || created.After(now) {
		return time.Time{}, false
	}
	deadline := created.Add(taskTerminalObservationLimit)
	if !now.Before(deadline) {
		return time.Time{}, false
	}
	return deadline, true
}

type taskTerminalFrameReader interface {
	Reader(context.Context) (websocket.MessageType, io.Reader, error)
}

// Conn.Close has its own internal handshake timeout. Do not grant a fresh
// timeout: expiry disposes this owned socket, then joins that close operation.
// This is transport cleanup only; no process/container operation is invoked.
func taskTerminalClose(ctx context.Context, conn *websocket.Conn) error {
	done := make(chan error, 1)
	go func() { done <- conn.Close(websocket.StatusNormalClosure, "") }()
	select {
	case err := <-done:
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		_ = conn.CloseNow()
		<-done
		return ctx.Err()
	}
}

// sandboxd independently broadcasts each subscriber's bounded replay/live
// frames. This reader never sends stdin/Resize and does not consume anyone
// else's stream. Only the typed Exit frame is decoded; output is discarded.
func taskTerminalReadEOF(ctx context.Context, conn taskTerminalFrameReader) (proxy.ProcessExit, bool) {
	var observed proxy.ProcessExit
	seen := false
	for {
		kind, frame, err := conn.Reader(ctx)
		if err != nil {
			return observed, seen && ctx.Err() == nil && websocket.CloseStatus(err) == websocket.StatusNormalClosure
		}
		if kind != websocket.MessageBinary || seen {
			return proxy.ProcessExit{}, false
		}
		var tag [1]byte
		if _, err = io.ReadFull(frame, tag[:]); err != nil {
			return proxy.ProcessExit{}, false
		}
		switch tag[0] {
		case 0: // wire.Stdout: opaque, bounded per frame and drained without storage
			n, readErr := io.Copy(io.Discard, io.LimitReader(frame, backend.MaxMessageBytes+1))
			if readErr != nil || n > backend.MaxMessageBytes {
				return proxy.ProcessExit{}, false
			}
		case 3: // wire.Exit: closed sandboxd Wait status, never a stdout lookalike
			payload, readErr := io.ReadAll(io.LimitReader(frame, 65))
			if readErr != nil || len(payload) > 64 {
				return proxy.ProcessExit{}, false
			}
			observed = proxy.DecodeProcessExit(payload)
			if !observed.Known() {
				return proxy.ProcessExit{}, false
			}
			seen = true
		default:
			return proxy.ProcessExit{}, false
		}
	}
}

func (a *API) observeTaskTerminal(original registry.Session) {
	if !terminalSession(original) || original.CreatedAt.IsZero() || original.AttachToken == "" || original.SandboxRef == "" {
		return
	}
	now := time.Now()
	deadline, valid := taskTerminalObservationDeadline(original.CreatedAt, now)
	if !valid {
		return
	}
	// One monotonic duration covers the entire dial, drain and close; no wait
	// receives a new budget. The original absolute clock is rechecked as well.
	ctx, cancel := context.WithTimeout(context.Background(), deadline.Sub(now))
	defer cancel()
	// Readiness atomically recorded the one-shot intent in its unique original
	// starting->running transition, before launching this reader. No asynchronous
	// registry write races capture/retirement just to claim observation ownership.
	current, err := a.deps.Store.Get(original.ID)
	if err != nil || current.Status != registry.StatusRunning || current.AttachToken != original.AttachToken || !current.TaskTerminalObserverStarted || !original.TaskTerminalObserverStarted || !taskTerminalObservationMatches(current, original) || ctx.Err() != nil || !time.Now().Before(deadline) {
		return
	}
	conn, err := a.deps.Backend.Attach(ctx, original.SandboxID, original.AttachToken)
	if err != nil || conn == nil {
		return
	}
	conn.SetReadLimit(backend.MaxMessageBytes)
	// Normal socket close affects only this subscription, never the process or
	// other subscribers. CloseNow is transport disposal, not a process signal.
	defer conn.CloseNow()
	exit, eof := taskTerminalReadEOF(ctx, conn)
	closeErr := taskTerminalClose(ctx, conn)
	if closeErr != nil || !eof || !exit.Known() || ctx.Err() != nil || !time.Now().Before(deadline) {
		return
	}
	a.markTaskTerminalExitedWithEOF(ctx, original, exit)
}
