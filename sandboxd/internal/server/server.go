//go:build linux || darwin

// Package server exposes the sandbox's single PTY over a WebSocket using the
// wire protocol. It never inspects terminal content.
package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/lcfrai/luci/sandboxd/internal/hub"
	"github.com/lcfrai/luci/sandboxd/internal/wire"
)

// maxMessageBytes raises coder/websocket's 32 KiB default read limit. The
// replay frame carries the whole scrollback (up to ring.DefaultCapacity) in one
// message and a paste arrives as one Stdin frame; either exceeds the default and
// would close the connection with StatusMessageTooBig. Must match
// backend.MaxMessageBytes in the control plane.
const maxMessageBytes = 4 << 20
const outputWriteTimeout = 5 * time.Second
const outputOverloadReason = "terminal output consumer too slow; reconnect for recent output only"

// Config describes the process every connection attaches to.
type Config struct {
	Command string
	Args    []string
	Env     []string

	// AttachToken is a shared secret the control plane must present on
	// /attach. On a developer laptop the sandbox port is published to
	// 127.0.0.1, which every local process can reach — the origin check in
	// the control plane does not protect this hop, so this token does. New
	// refuses to start with this empty unless AllowUnauthenticated is also
	// set: a security property must not depend on every caller remembering
	// to supply it.
	AttachToken string

	// AllowUnauthenticated opts out of the AttachToken requirement. Local
	// development and ad hoc testing only — the production image never sets
	// this, so a sandbox container always requires a token unless someone
	// deliberately chose not to.
	AllowUnauthenticated bool

	// Scrollback is the replay buffer size in bytes; 0 uses the hub default.
	Scrollback int
}

// Server binds HTTP requests to the sandbox's hub.
type Server struct {
	cfg       Config
	h         *hub.Hub
	mux       *http.ServeMux
	startedAt int64
	nativeURL string
}

// New starts the PTY and returns a Server serving /attach and /healthz. The
// PTY starts here, once, rather than per connection: a session must outlive
// the client that is looking at it.
//
// New fails closed: with no AttachToken and no explicit AllowUnauthenticated,
// it refuses to start rather than silently serving an unauthenticated
// /attach. A published loopback port is reachable by every process on a
// developer laptop, so absence of a token must be a visible, deliberate
// choice — never a default that a forgotten environment variable falls back
// to.
func New(cfg Config) (*Server, error) {
	if cfg.AttachToken == "" && !cfg.AllowUnauthenticated {
		return nil, errors.New("sandboxd: no attach token configured; " +
			"set AttachToken (LUCI_ATTACH_TOKEN / -attach-token) or " +
			"pass -allow-unauthenticated to run without one")
	}
	startedAt := time.Now().UnixMilli()
	h, err := hub.Start(hub.Config{
		Command:    cfg.Command,
		Args:       cfg.Args,
		Env:        cfg.Env,
		Scrollback: cfg.Scrollback,
	})
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, h: h, mux: http.NewServeMux(), startedAt: startedAt, nativeURL: "http://127.0.0.1:4096"}
	s.mux.HandleFunc("/attach", s.attach)
	if cfg.AttachToken != "" && nativeOpenCodeCommand(cfg.Command, cfg.Args) {
		s.mux.HandleFunc("/native-turn", s.nativeTurn)
	}
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// Close stops the underlying process. In practice only tests reach this: the
// entrypoint's way out is Done, below, and a container that exits.
func (s *Server) Close() error { return s.h.Close() }

// Done closes when the process this sandbox exists to run has ended. The
// entrypoint waits on it and then exits, which is F3's fix: PID 1 returning is
// what stops the container, and a stopped container is the only channel the
// control plane has for "the process ended" (Go contract §5.3 rule 3). Before
// it, a shell that ran `exit` left sandboxd serving an /attach that could only
// answer with an Exit frame and a close, for as long as anything kept asking.
func (s *Server) Done() <-chan struct{} { return s.h.Done() }

// ExitReason describes why the process ended, for the entrypoint's last log
// line. Empty until Done is closed.
func (s *Server) ExitReason() string { return s.h.ExitReason() }

// authorised checks the bearer token. New guarantees AttachToken is non-empty
// here unless AllowUnauthenticated was set, so an empty token at this point
// means auth was deliberately disabled. ConstantTimeCompare avoids leaking
// the token's content through timing; it still returns early on a length
// mismatch, which leaks length, but the token's length is not itself a
// secret.
func (s *Server) authorised(r *http.Request) bool {
	if s.cfg.AttachToken == "" {
		return true
	}
	const scheme = "Bearer "
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, scheme) {
		return false
	}
	got := strings.TrimPrefix(auth, scheme)
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.AttachToken)) == 1
}

// exitFrame decides whether a closed subscriber channel means the process
// actually exited. Subscribe's cancel (a client detaching) and the hub's own
// finish (the process exiting) close that same channel with no distinguishing
// signal, so an Exit frame must only be sent when Done is also closed —
// otherwise a plain detach would be reported to that connection as the
// session having ended, which is untrue.
func exitFrame(h *hub.Hub) (frame []byte, send bool) {
	select {
	case <-h.Done():
		return wire.Encode(wire.Exit, []byte(h.ExitReason())), true
	default:
		return nil, false
	}
}

// wsConn is the minimal surface fanOut needs from a *websocket.Conn.
// Extracting it lets a test drive the real production fan-out loop —
// including the exitFrame decision — against a recording fake, with no live
// network connection and nothing timing-dependent to observe.
type wsConn interface {
	Write(ctx context.Context, typ websocket.MessageType, p []byte) error
	Close(code websocket.StatusCode, reason string) error
}

// fanOut relays hub output to conn until out closes or ctx is done. This is
// the exact decision the exitFrame amendment protects: whether a closed out
// means the process exited (send an Exit frame) or the client merely
// detached (send nothing beyond a normal close).
func fanOut(ctx context.Context, conn wsConn, h *hub.Hub, out <-chan []byte, overflow <-chan struct{}) {
	for {
		// Prefer the overflow signal to queued chunks and even a simultaneous exit.
		// A partial terminal transcript must not be presented as a complete exit.
		select {
		case <-overflow:
			conn.Close(websocket.StatusTryAgainLater, outputOverloadReason)
			return
		default:
		}
		select {
		case <-overflow:
			conn.Close(websocket.StatusTryAgainLater, outputOverloadReason)
			return
		case chunk, ok := <-out:
			if !ok {
				select {
				case <-overflow:
					conn.Close(websocket.StatusTryAgainLater, outputOverloadReason)
					return
				default:
				}
				if frame, send := exitFrame(h); send {
					_ = conn.Write(ctx, websocket.MessageBinary, frame)
				}
				conn.Close(websocket.StatusNormalClosure, "session ended")
				return
			}
			if err := writeOutput(ctx, conn, wire.Encode(wire.Stdout, chunk)); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func writeOutput(ctx context.Context, conn wsConn, frame []byte) error {
	writeCtx, cancel := context.WithTimeout(ctx, outputWriteTimeout)
	defer cancel()
	err := conn.Write(writeCtx, websocket.MessageBinary, frame)
	if err != nil && ctx.Err() == nil {
		conn.Close(websocket.StatusTryAgainLater, outputOverloadReason)
	}
	return err
}

func (s *Server) attach(w http.ResponseWriter, r *http.Request) {
	if !s.authorised(r) {
		slog.Warn("rejected unauthenticated attach", "remote", r.RemoteAddr)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Origin is enforced upstream by the control plane (cmd/api), which is the
	// only component a browser context can reach. This hop is guarded by the
	// bearer token above.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		slog.Error("websocket accept failed", "err", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "closing")
	conn.SetReadLimit(maxMessageBytes)

	ctx := r.Context()
	replay, out, overflow, cancel := s.h.SubscribeWithOverflow()
	defer cancel()

	// Replay first, so a returning client sees what it missed before anything
	// live arrives.
	if len(replay) > 0 {
		if err := writeOutput(ctx, conn, wire.Encode(wire.Stdout, replay)); err != nil {
			return
		}
	}

	go fanOut(ctx, conn, s.h, out, overflow)

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			// The client went away. Detach only — the process keeps running.
			return
		}
		kind, payload, err := wire.Decode(data)
		if err != nil {
			continue
		}
		switch kind {
		case wire.Stdin:
			if _, err := s.h.Write(payload); err != nil {
				return
			}
		case wire.Resize:
			var rp wire.ResizePayload
			if err := rp.UnmarshalBinary(payload); err == nil {
				_ = s.h.Resize(rp.Cols, rp.Rows)
			}
		}
	}
}
