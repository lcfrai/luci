// Package server exposes a PTY over a WebSocket using the wire protocol.
package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/ptysession"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/wire"
)

// Config describes the process a connection should attach to.
type Config struct {
	Command string
	Args    []string
	Env     []string
}

// Server upgrades HTTP requests to a PTY-backed WebSocket.
type Server struct {
	cfg Config
	mux *http.ServeMux
}

// New builds a Server serving /attach.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("/attach", s.attach)
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) attach(w http.ResponseWriter, r *http.Request) {
	// The control plane is the only client and it dials in-cluster, so the
	// origin check is disabled here and enforced upstream.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		slog.Error("websocket accept failed", "err", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "closing")

	sess, err := ptysession.Start(s.cfg.Command, s.cfg.Args, s.cfg.Env)
	if err != nil {
		slog.Error("pty start failed", "err", err)
		_ = conn.Write(r.Context(), websocket.MessageBinary,
			wire.Encode(wire.Exit, []byte("failed to start: "+err.Error())))
		return
	}
	defer sess.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// PTY -> socket
	go func() {
		defer cancel()
		buf := make([]byte, 32*1024)
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				if werr := conn.Write(ctx, websocket.MessageBinary,
					wire.Encode(wire.Stdout, buf[:n])); werr != nil {
					return
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					slog.Debug("pty read ended", "err", err)
				}
				_ = conn.Write(ctx, websocket.MessageBinary, wire.Encode(wire.Exit, []byte("session ended")))
				return
			}
		}
	}()

	// socket -> PTY
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		kind, payload, err := wire.Decode(data)
		if err != nil {
			continue
		}
		switch kind {
		case wire.Stdin:
			if _, err := sess.Write(payload); err != nil {
				return
			}
		case wire.Resize:
			var rp wire.ResizePayload
			if err := rp.UnmarshalBinary(payload); err == nil {
				_ = sess.Resize(rp.Cols, rp.Rows)
			}
		}
	}
}
