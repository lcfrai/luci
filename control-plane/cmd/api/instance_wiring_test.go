package main

import (
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

// newBackend must hand the Docker backend the instance id serve loaded. It is
// the one line of that wiring a unit test can reach without a daemon, because
// dockerbe.New refuses a missing instance (ErrUnsupported) BEFORE it pings:
// pointed at a port nothing listens on, a wired id fails at the ping
// (ErrUnavailable), and a dropped one fails at the validation instead. Nothing
// here reaches a real Docker daemon — the host is explicit and dead.
func TestNewBackendHandsDockerTheInstanceID(t *testing.T) {
	t.Parallel()
	cfg := Config{Backend: string(backend.KindDocker), DockerHost: "tcp://127.0.0.1:1"}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := newBackend(cfg, "11111111-1111-4111-8111-111111111111", log)
	if errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("newBackend = %v: the instance id did not reach dockerbe.Config", err)
	}
	if !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("newBackend against a dead host = %v, want ErrUnavailable from the ping", err)
	}
}
