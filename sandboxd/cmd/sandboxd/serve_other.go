//go:build !linux && !darwin

package main

import (
	"errors"
	"net/http"
)

// sandboxd runs inside the Linux sandbox image. This stub exists only so the
// workspace builds on a Windows development machine; creack/pty has no Windows
// support and ConPTY is out of scope for this phase.
func newServer(string, []string, string, bool) (http.Handler, error) {
	return nil, errors.New("sandboxd requires linux or darwin")
}
