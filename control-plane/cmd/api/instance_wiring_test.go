package main

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/instance"
)

// newBackend must hand the Docker backend the instance id serve loaded. This
// pins newBackend's body without any daemon, because dockerbe.New refuses a
// missing instance (ErrUnsupported) BEFORE it pings (serve's own arguments to
// newBackend and reaperConfigFor are pinned in fakedaemon_test.go):
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

// serve loads the instance id before it builds anything, and a restart on the
// same state directory is the same instance. Driven through run() twice on one
// state directory, hermetic (runHermetic): serve mints the id, logs that it
// did, and then fails in newBackend before a reaper, a registry or a listener
// exists. The second run must find the same id and not mint again.
func TestServeMintsTheInstanceIDOnceAndARestartKeepsIt(t *testing.T) {
	t.Parallel()
	state := privateStateDir(t)
	path := filepath.Join(state, "instance", instance.FileName)

	var first bytes.Buffer
	if code := run(env(runHermetic(t, map[string]string{envStateDir: state})), &first); code != 1 {
		t.Fatalf("first run exited %d, want 1 (the hermetic kata refusal); output:\n%s", code, first.String())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("serve did not persist an instance id at %s before building the backend: %v", path, err)
	}
	id := strings.TrimSpace(string(raw))
	if !strings.Contains(first.String(), "minted this state directory's instance id") || !strings.Contains(first.String(), id) {
		t.Errorf("the first start did not log the id it minted (%s); output:\n%s", id, first.String())
	}

	var second bytes.Buffer
	if code := run(env(runHermetic(t, map[string]string{envStateDir: state})), &second); code != 1 {
		t.Fatalf("second run exited %d, want 1; output:\n%s", code, second.String())
	}
	if strings.Contains(second.String(), "minted") {
		t.Errorf("the restart minted a new identity, which disowns every sandbox the first made; output:\n%s", second.String())
	}
	raw, _ = os.ReadFile(path)
	if got := strings.TrimSpace(string(raw)); got != id {
		t.Errorf("instance id changed across a restart: %q, then %q", id, got)
	}
}

// A corrupt id stops the process with a sentence naming the file — before the
// backend exists, so nothing is created under an identity nobody chose.
func TestServeRefusesACorruptInstanceID(t *testing.T) {
	t.Parallel()
	state := privateStateDir(t)
	path := filepath.Join(state, "instance", instance.FileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not an id\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := run(env(runHermetic(t, map[string]string{envStateDir: state})), &out); code != 1 {
		t.Fatalf("run exited %d, want 1; output:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "instance id") || strings.Contains(out.String(), "kubeconfig") {
		t.Errorf("run did not stop at the instance id, before the backend; output:\n%s", out.String())
	}
}
