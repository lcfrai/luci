//go:build linux || darwin

package main

import (
	"net/http"

	"github.com/kpmg-au-emu/panacea-app/sandboxd/internal/server"
)

func newServer(command string, argv []string, token string) (http.Handler, error) {
	return server.New(server.Config{Command: command, Args: argv, AttachToken: token})
}
