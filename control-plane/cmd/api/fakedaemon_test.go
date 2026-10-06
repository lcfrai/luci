package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/instance"
	"github.com/lcfrai/luci/control-plane/internal/reaper"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// fakeDaemon is the Docker Engine API, as much of it as serve and the reaper
// call: /_ping, GET /containers/json and DELETE /containers/{name}. It is an
// httptest server on 127.0.0.1, so a test that points
// LUCI_DOCKER_HOST at it reaches no real daemon, and it holds no
// container anybody made. Any other request is recorded and answered 404, so a
// test can say what serve asked that it did not expect.
type fakeDaemon struct {
	srv *httptest.Server

	mu      sync.Mutex
	list    []container.Summary
	removed []string
	other   []string

	removedCh chan string
}

func newFakeDaemon(t *testing.T, list []container.Summary) *fakeDaemon {
	t.Helper()
	d := &fakeDaemon{list: list, removedCh: make(chan string, 16)}
	d.srv = httptest.NewServer(http.HandlerFunc(d.serveHTTP))
	t.Cleanup(d.srv.Close)
	return d
}

func (d *fakeDaemon) host() string { return "tcp://" + d.srv.Listener.Addr().String() }

func (d *fakeDaemon) serveHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	switch {
	case strings.HasSuffix(p, "/_ping"):
		w.Header().Set("API-Version", "1.43")
		w.Header().Set("OSType", "linux")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, "OK")
		}
	case r.Method == http.MethodGet && strings.HasSuffix(p, "/containers/json"):
		d.mu.Lock()
		body, err := json.Marshal(d.list)
		d.mu.Unlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	case r.Method == http.MethodDelete && strings.Contains(p, "/containers/"):
		name := p[strings.LastIndex(p, "/")+1:]
		d.mu.Lock()
		d.removed = append(d.removed, name)
		d.mu.Unlock()
		select {
		case d.removedCh <- name:
		default:
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		d.mu.Lock()
		d.other = append(d.other, r.Method+" "+p)
		d.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"fake daemon: no such route"}`)
	}
}

func (d *fakeDaemon) removedNames() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.removed...)
}

// summaryOf is one managed container as the daemon lists it. An empty
// instanceID is a container from before per-instance ownership: LabelManaged
// and a sandbox id, no LabelInstance.
func summaryOf(containerID, sandboxID, instanceID string, ports ...container.Port) container.Summary {
	labels := map[string]string{
		backend.LabelManaged:   "true",
		backend.LabelSandboxID: sandboxID,
		backend.LabelSessionID: "sess-" + sandboxID,
	}
	if instanceID != "" {
		labels[backend.LabelInstance] = instanceID
	}
	return container.Summary{
		ID:      containerID,
		Names:   []string{"/luci-" + sandboxID},
		State:   "running",
		Created: time.Now().Add(-time.Hour).Unix(), // well past any create grace
		Labels:  labels,
		Ports:   ports,
	}
}

// clearDockerEnv empties the variables the Docker client's FromEnv reads
// besides the host, so a developer's DOCKER_TLS_VERIFY or DOCKER_API_VERSION
// cannot turn a request to the fake into TLS or pin a version it does not
// serve. t.Setenv, so these tests are not parallel.
func clearDockerEnv(t *testing.T) {
	for _, k := range []string{"DOCKER_HOST", "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH", "DOCKER_API_VERSION"} {
		t.Setenv(k, "")
	}
}

// writeInstanceID gives a state directory an id before serve starts, so the
// test knows the id serve will load (and serve does not mint).
func writeInstanceID(t *testing.T, state, id string) {
	t.Helper()
	path := filepath.Join(state, "instance", instance.FileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// lockedBuf is a log sink serve's goroutines and the test can share.
type lockedBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func fakeDaemonConfig(t *testing.T, state string, d *fakeDaemon) Config {
	t.Helper()
	cfg, err := loadConfig(env(map[string]string{
		envStateDir:     state,
		envAddr:         "127.0.0.1:0",
		envBackend:      string(backend.KindDocker),
		envDockerHost:   d.host(),
		envReapInterval: "24h", // the startup sweep is the one under test
		envGitName:      "Fake Daemon Test",
		envGitEmail:     "fake-daemon@example.invalid",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	return cfg
}

const (
	fakeOurs   = "33333333-3333-4333-8333-333333333333"
	fakeTheirs = "44444444-4444-4444-8444-444444444444"
)

// Reaper-scope fix round, I2. serve loads one instance id and must hand the
// SAME one to the backend and to the reaper. A helper test pins
// reaperConfigFor's body, and one pins newBackend's; neither pins the two
// arguments serve passes. With "" handed to the reaper, the control plane
// starts, logs one warning, and never again reaps an orphan of its own or
// destroys a LAN-published sandbox of its own (rule 1a) — and every other test
// stays green.
//
// So this runs serve itself, against a fake Docker API, with an own orphan
// and another instance's orphan on the "daemon": the startup sweep must
// destroy the first by name and never touch the second. Hermetic: its own
// state directory, 127.0.0.1:0, and a daemon that is an httptest server.
func TestServeReapsItsOwnOrphanWithTheIDItLoadedAndSparesAnothers(t *testing.T) {
	clearDockerEnv(t)
	state := privateStateDir(t)
	writeInstanceID(t, state, fakeOurs)
	d := newFakeDaemon(t, []container.Summary{
		summaryOf("c-own-orphan", "s-ownorph1", fakeOurs),
		summaryOf("c-their-orphan", "s-theirs01", fakeTheirs),
	})
	cfg := fakeDaemonConfig(t, state, d)
	logs := &lockedBuf{}
	log := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, log) }()

	select {
	case name := <-d.removedCh:
		if name != "luci-s-ownorph1" {
			t.Errorf("the first container serve's reaper removed was %q, want its own orphan luci-s-ownorph1", name)
		}
	case err := <-done:
		t.Fatalf("serve returned (%v) before its reaper destroyed its own orphan; logs:\n%s", err, logs.String())
	case <-time.After(20 * time.Second):
		t.Fatalf("serve's startup sweep destroyed no orphan of its own in 20s: the reaper was not given the id serve loaded; logs:\n%s", logs.String())
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve = %v after cancel, want nil; logs:\n%s", err, logs.String())
		}
	case <-time.After(20 * time.Second):
		t.Fatal("serve did not return within 20s of its context being cancelled")
	}
	if got := d.removedNames(); !reflect.DeepEqual(got, []string{"luci-s-ownorph1"}) {
		t.Errorf("removed %v, want only this instance's orphan: another instance's is its own reaper's", got)
	}
	if !strings.Contains(logs.String(), "instance="+fakeOurs) {
		t.Errorf("the listening line did not name the instance id serve loaded (%s); logs:\n%s", fakeOurs, logs.String())
	}
}

// Reaper-scope fix round, I1, through the real composition: dockerbe over a
// fake daemon, then restart recovery, then the reaper's first sweep. A session
// made by a build from before per-instance ownership has a container with no
// instance label. Its sandbox id is in this registry, which proves the
// container is this state directory's (the id was minted here; the name is
// unique on the daemon). It is still running, so the session must come out of
// the upgrade still running — not `exited` with a reason saying the sandbox
// disappeared, which is what dropping it from List produced.
func TestRestartRecoveryKeepsAPreUpgradeSessionWhoseSandboxIsStillRunning(t *testing.T) {
	clearDockerEnv(t)
	state := privateStateDir(t)
	writeInstanceID(t, state, fakeOurs)
	d := newFakeDaemon(t, []container.Summary{
		summaryOf("c-legacy", "s-legacy01", "", container.Port{IP: "127.0.0.1", PrivatePort: uint16(backend.ContainerPort), PublicPort: 51001, Type: "tcp"}),
		summaryOf("c-stray", "s-stray001", ""), // legacy, and no record here: not ours to reap
	})
	cfg := fakeDaemonConfig(t, state, d)
	logs := &lockedBuf{}
	log := slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := context.Background()

	if err := os.MkdirAll(filepath.Dir(cfg.SessionsPath), 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := registry.NewFileStore(cfg.SessionsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Create(registry.Session{
		ID: "sess-legacy", SandboxID: "s-legacy01", Status: registry.StatusRunning,
		Endpoint: "127.0.0.1:51001", AttachToken: "a-token-the-sandbox-holds",
	}); err != nil {
		t.Fatal(err)
	}

	be, err := newBackend(cfg, fakeOurs, log)
	if err != nil {
		t.Fatalf("newBackend against the fake daemon: %v", err)
	}
	defer func() { _ = be.Close() }()
	recoverSessions(ctx, store, be, log)
	rp := reaper.New(reaperConfigFor(cfg, fakeOurs, store, be, log))
	for i := 0; i < 2; i++ {
		if err := rp.Sweep(ctx); err != nil {
			t.Fatalf("Sweep %d: %v", i, err)
		}
	}

	got, err := store.Get("sess-legacy")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != registry.StatusRunning || got.ExitReason != "" {
		t.Errorf("the pre-upgrade session came out %q (%q), want running: its container is still running on the daemon; logs:\n%s", got.Status, got.ExitReason, logs.String())
	}
	if got.Endpoint != "127.0.0.1:51001" || got.AttachToken == "" {
		t.Errorf("endpoint %q, token set %v: restart recovery must keep what attach needs", got.Endpoint, got.AttachToken != "")
	}
	if removed := d.removedNames(); len(removed) != 0 {
		t.Errorf("removed %v: neither a tracked legacy sandbox nor an untracked one is an orphan to reap", removed)
	}
	for _, line := range strings.Split(logs.String(), "\n") {
		if strings.Contains(line, "sandbox=s-legacy01") && strings.Contains(line, "foreign") {
			t.Errorf("the session's own sandbox was called foreign: %s", line)
		}
	}
	if n := strings.Count(logs.String(), "sandbox=s-stray001"); n != 1 {
		t.Errorf("the untracked legacy sandbox appeared in %d log lines, want exactly the reaper's one; logs:\n%s", n, logs.String())
	}
}
