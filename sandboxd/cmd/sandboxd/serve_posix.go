//go:build linux || darwin

package main

import (
	"github.com/lcfrai/luci/sandboxd/internal/server"
)

func newServer(command string, argv []string, token string, allowUnauthenticated bool) (sandboxServer, error) {
	return server.New(server.Config{
		Command:              command,
		Args:                 argv,
		AttachToken:          token,
		AllowUnauthenticated: allowUnauthenticated,
	})
}
