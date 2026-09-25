//go:build linux || darwin

package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	closeServer(t, off)
	if got := mcpStatus(t, off, "tok"); got != http.StatusNotFound {
		t.Errorf("-mcp-listen off: /mcp-channel answered %d, want 404", got)
	}

	on, err := newServer("/bin/sh", nil, "tok", false, freeLoopbackAddr(t))
	if err != nil {
		t.Fatalf("-mcp-listen on loopback: %v", err)
	}
	closeServer(t, on)
	if got := mcpStatus(t, on, ""); got != http.StatusUnauthorized {
		t.Errorf("/mcp-channel without the token answered %d, want 401", got)
	}
}

// closeServer stops h's session when the test ends: a started server owns a
// PTY child (review t18 M9). Its relay listener, which the process owns for
// its lifetime, goes with the test binary.
func closeServer(t *testing.T, h http.Handler) {
	t.Helper()
	c, ok := h.(io.Closer)
	if !ok {
		t.Fatalf("newServer returned a %T, which cannot be closed", h)
	}
	t.Cleanup(func() { c.Close() })
}

const probeToken = "tok-probe-8c1f2e"

// waitForFile waits until path holds want, as a PTY child writes it.
func waitForFile(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		if b, err := os.ReadFile(path); err == nil && string(b) == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s never held %q: the PTY child did not finish", path, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// The attach token opens /attach and /mcp-channel. A process in the session
// that held it could type into the session and replace the relay's channel
// with its own, so the session's environment does not carry it; every other
// variable still reaches the harness (review t18 M3).
func TestNewServer_sessionEnvLacksTheToken(t *testing.T) {
	t.Setenv(attachTokenEnv, probeToken)
	t.Setenv("PANACEA_APP_PROBE_KEPT", "kept")
	dir := t.TempDir()
	script := "env > " + dir + "/env.txt; printf done > " + dir + "/done"
	srv, err := newServer("/bin/sh", []string{"-c", script}, probeToken, false, "off")
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	closeServer(t, srv)
	waitForFile(t, filepath.Join(dir, "done"), "done")
	b, err := os.ReadFile(filepath.Join(dir, "env.txt"))
	if err != nil {
		t.Fatalf("read the session's environment: %v", err)
	}
	if strings.Contains(string(b), probeToken) || strings.Contains(string(b), attachTokenEnv+"=") {
		t.Errorf("the session's environment carries the attach token:\n%s", b)
	}
	if !strings.Contains(string(b), "PANACEA_APP_PROBE_KEPT=kept") {
		t.Errorf("the session's environment lost sandboxd's other variables:\n%s", b)
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
