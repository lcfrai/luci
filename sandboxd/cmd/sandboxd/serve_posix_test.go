//go:build linux || darwin

package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func mcpStatus(t *testing.T, h http.Handler, token string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/mcp-channel", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// -mcp-listen reaches the server: "off" leaves /mcp-channel unrouted, a
// loopback address routes it (401 before the token), and anything else is
// refused before the session starts (G4).
func TestNewServer_mcpListen(t *testing.T) {
	if _, err := newServer("/bin/sh", nil, "tok", false, "0.0.0.0:0"); err == nil {
		t.Errorf("-mcp-listen 0.0.0.0:0 was accepted; the relay must bind loopback only")
	}

	off, err := newServer("/bin/sh", nil, "tok", false, "off")
	if err != nil {
		t.Fatalf("-mcp-listen off: %v", err)
	}
	if got := mcpStatus(t, off, "tok"); got != http.StatusNotFound {
		t.Errorf("-mcp-listen off: /mcp-channel answered %d, want 404", got)
	}

	on, err := newServer("/bin/sh", nil, "tok", false, freeLoopbackAddr(t))
	if err != nil {
		t.Fatalf("-mcp-listen on loopback: %v", err)
	}
	if got := mcpStatus(t, on, ""); got != http.StatusUnauthorized {
		t.Errorf("/mcp-channel without the token answered %d, want 401", got)
	}
}

// A session that fails to start must not keep the relay's port: a restarted
// sandboxd could not bind it again.
func TestNewServer_releasesTheRelayOnFailure(t *testing.T) {
	addr := freeLoopbackAddr(t)
	if _, err := newServer("/bin/sh", nil, "", false, addr); err == nil {
		t.Fatalf("a server with no token and no -allow-unauthenticated started")
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("the relay's port %s is still held after the failed start: %v", addr, err)
	}
	ln.Close()
}
