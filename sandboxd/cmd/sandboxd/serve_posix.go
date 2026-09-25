//go:build linux || darwin

package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/mcprelay"
	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/server"
)

// newServer opens the MCP relay's listener, then starts the session. The
// order matters: the harness dials MCP as soon as the PTY starts it, and a
// bound listener holds that connection (mcprelay.HoldTimeout) where an
// unbound port would refuse it. "off" leaves the relay out.
//
// The session never sees the attach token: not in its environment
// (withoutAttachToken), and not in sandboxd's /proc/<pid>/environ
// (protectEnviron). A session process that held it could type into the
// session over /attach and replace the relay's channel with its own.
func newServer(command string, argv []string, token string, allowUnauthenticated bool, mcpListen string) (http.Handler, error) {
	if err := protectEnviron(); err != nil {
		return nil, err
	}
	mcp, err := mcprelay.Listen(mcpListen, slog.Default())
	if err != nil {
		return nil, err
	}
	srv, err := server.New(server.Config{
		Command:              command,
		Args:                 argv,
		Env:                  withoutAttachToken(os.Environ()),
		AttachToken:          token,
		AllowUnauthenticated: allowUnauthenticated,
		MCP:                  mcp,
	})
	if err != nil {
		if mcp != nil {
			mcp.Close()
		}
		return nil, err
	}
	return srv, nil
}
