//go:build linux || darwin

// Package server exposes the sandbox's single PTY over a WebSocket using the
// wire protocol. It never inspects terminal content.
package server

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/hub"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/wire"
)

// maxMessageBytes raises coder/websocket's 32 KiB default read limit. The
// replay frame carries the whole scrollback (up to ring.DefaultCapacity) in one
// message and a paste arrives as one Stdin frame; either exceeds the default and
// would close the connection with StatusMessageTooBig. Must match
// backend.MaxMessageBytes in the control plane.
const maxMessageBytes = 4 << 20

// Config describes the process every connection attaches to.
type Config struct {
	Command string
	Args    []string
	Env     []string

	// AttachToken is a shared secret the control plane must present on
	// /attach. On a developer laptop the sandbox port is published to
	// 127.0.0.1, which every local process can reach — the origin check in the
	// control plane does not protect this hop, so this token does.
	// An empty token disables the check and is only acceptable in tests.
	AttachToken string

	// Scrollback is the replay buffer size in bytes; 0 uses the hub default.
	Scrollback int
}

// Server binds HTTP requests to the sandbox's hub.
type Server struct {
	cfg Config
	h   *hub.Hub
	mux *http.ServeMux
}

// New starts the PTY and returns a Server serving /attach and /healthz. The
// PTY starts here, once, rather than per connection: a session must outlive
// the client that is looking at it.
func New(cfg Config) (*Server, error) {
	h, err := hub.Start(hub.Config{
		Command:    cfg.Command,
		Args:       cfg.Args,
		Env:        cfg.Env,
		Scrollback: cfg.Scrollback,
	})
	if err != nil {
		return nil, err
	}
	s := &Server{cfg: cfg, h: h, mux: http.NewServeMux()}
	s.mux.HandleFunc("/attach", s.attach)
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// Close stops the underlying process. Only the entrypoint calls this.
func (s *Server) Close() error { return s.h.Close() }

func (s *Server) authorised(r *http.Request) bool {
	if s.cfg.AttachToken == "" {
		return true
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
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
	replay, out, cancel := s.h.Subscribe()
	defer cancel()

	// Replay first, so a returning client sees what it missed before anything
	// live arrives.
	if len(replay) > 0 {
		if err := conn.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Stdout, replay)); err != nil {
			return
		}
	}

	go func() {
		for {
			select {
			case chunk, ok := <-out:
				if !ok {
					if frame, send := exitFrame(s.h); send {
						_ = conn.Write(ctx, websocket.MessageBinary, frame)
					}
					conn.Close(websocket.StatusNormalClosure, "session ended")
					return
				}
				if err := conn.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Stdout, chunk)); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

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
