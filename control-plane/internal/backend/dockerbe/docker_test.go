package dockerbe_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/dockerbe"
)

// notFoundErr is shaped exactly like the SDK's own objectNotFoundError: a
// NotFound() method, which is what cerrdefs.IsNotFound — and therefore
// client.IsErrNotFound — actually tests for.
type notFoundErr struct{ what string }

func (e notFoundErr) Error() string { return "Error: No such container: " + e.what }
func (e notFoundErr) NotFound()     {}

type createCall struct {
	config     *container.Config
	hostConfig *container.HostConfig
	networking *network.NetworkingConfig
	platform   *ocispec.Platform
	name       string
}

type inspectResult struct {
	resp container.InspectResponse
	err  error
}

// fakeDocker is the whole Docker daemon, for these tests. Results are queued;
// the last queued inspect repeats forever, because Create inspects more than
// once.
type fakeDocker struct {
	mu sync.Mutex

	pingErr error

	creates   []createCall
	createID  string
	createErr error

	starts   []string
	startErr error

	inspects     []inspectResult
	inspectNames []string

	listOpts []container.ListOptions
	list     []container.Summary
	listErr  error

	removes    []string
	removeOpts []container.RemoveOptions
	removeErr  error

	closed bool
}

var _ dockerbe.APIClient = (*fakeDocker)(nil)

func (f *fakeDocker) ContainerCreate(_ context.Context, cfg *container.Config, host *container.HostConfig, net *network.NetworkingConfig, plat *ocispec.Platform, name string) (container.CreateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates = append(f.creates, createCall{config: cfg, hostConfig: host, networking: net, platform: plat, name: name})
	if f.createErr != nil {
		return container.CreateResponse{}, f.createErr
	}
	return container.CreateResponse{ID: f.createID}, nil
}

func (f *fakeDocker) ContainerStart(_ context.Context, id string, _ container.StartOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts = append(f.starts, id)
	return f.startErr
}

func (f *fakeDocker) ContainerInspect(_ context.Context, name string) (container.InspectResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inspectNames = append(f.inspectNames, name)
	if len(f.inspects) == 0 {
		return container.InspectResponse{}, notFoundErr{name}
	}
	r := f.inspects[0]
	if len(f.inspects) > 1 {
		f.inspects = f.inspects[1:]
	}
	return r.resp, r.err
}

func (f *fakeDocker) ContainerList(_ context.Context, opts container.ListOptions) ([]container.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listOpts = append(f.listOpts, opts)
	return f.list, f.listErr
}

func (f *fakeDocker) ContainerRemove(_ context.Context, id string, opts container.RemoveOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removes = append(f.removes, id)
	f.removeOpts = append(f.removeOpts, opts)
	return f.removeErr
}

func (f *fakeDocker) Ping(_ context.Context) (types.Ping, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pingErr != nil {
		return types.Ping{}, f.pingErr
	}
	return types.Ping{APIVersion: "1.51"}, nil
}

func (f *fakeDocker) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeDocker) snapshotRemoves() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.removes...)
}

func (f *fakeDocker) snapshotCreates() []createCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]createCall{}, f.creates...)
}

// healthzAddr starts a real loopback server answering 200 on /healthz and
// returns its host:port. backend.WaitReady polls it for real.
func healthzAddr(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(backend.HealthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

// deadAddr returns a loopback host:port with nothing listening on it.
func deadAddr(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NewServeMux())
	addr := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()
	return addr
}

func portOf(t *testing.T, addr string) string {
	t.Helper()
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		t.Fatalf("addr %q has no port", addr)
	}
	return addr[i+1:]
}

func labelsFor(sandboxID, sessionID string) map[string]string {
	return map[string]string{
		backend.LabelManaged:   "true",
		backend.LabelSandboxID: sandboxID,
		backend.LabelSessionID: sessionID,
		backend.LabelCreatedAt: "2026-09-18T01:02:03Z",
	}
}

// inspectOf builds an InspectResponse. hostPort "" means "no binding yet".
func inspectOf(ref, status, hostIP, hostPort string, labels map[string]string) container.InspectResponse {
	ports := nat.PortMap{}
	if hostPort != "" {
		ports[nat.Port("8080/tcp")] = []nat.PortBinding{{HostIP: hostIP, HostPort: hostPort}}
	}
	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:      ref,
			Created: "2026-09-18T01:02:03.400000000Z",
			State:   &container.State{Status: status, Running: status == "running"},
		},
		Config:          &container.Config{Labels: labels},
		NetworkSettings: &container.NetworkSettings{NetworkSettingsBase: container.NetworkSettingsBase{Ports: ports}},
	}
}

func validSpec() backend.Spec {
	return backend.Spec{
		ID:          "s-3f2a91c0",
		SessionID:   "sess-1",
		Image:       "panacea-app/sandbox:dev",
		Command:     "claude",
		Args:        []string{"--model", "opus", "-p", "refactor auth, then test"},
		AttachToken: "tok-abc",
		Resources:   backend.Resources{CPUMillis: 2000, MemoryMiB: 4096},
		WorkDir:     "/work",
		Mounts:      []backend.Mount{{HostPath: "C:/repo", ContainerPath: "/work"}},
	}
}

func newBackend(t *testing.T, f *fakeDocker) *dockerbe.Backend {
	t.Helper()
	b, err := dockerbe.New(dockerbe.Config{Client: f, ReadyTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}

// newBackendWithLog is newBackend with a capturable logger. List's refusal to
// error a whole sweep means the log line is the only place an off-loopback
// publish surfaces from a sweep, so that line is behaviour and is asserted,
// not decoration.
func newBackendWithLog(t *testing.T, f *fakeDocker) (*dockerbe.Backend, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	b, err := dockerbe.New(dockerbe.Config{Client: f, ReadyTimeout: 5 * time.Second, Log: logger})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b, &buf
}

// A Docker Desktop that is installed but not running must fail here, with a
// sentence the operator can act on, not later with a nil dereference.
func TestNewIsErrUnavailableWhenNoDaemonAnswers(t *testing.T) {
	f := &fakeDocker{pingErr: errors.New("cannot connect to the Docker daemon")}

	_, err := dockerbe.New(dockerbe.Config{Client: f})
	if !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("New with a dead daemon = %v, want an error wrapping ErrUnavailable", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "docker desktop") {
		t.Errorf("New error %q does not tell the operator what to start", err)
	}
}

// Publishing to anything but loopback puts a shell on the LAN. It has to be a
// decision someone makes on purpose, not a default anyone can trip over.
func TestNewRefusesANonLoopbackBindIP(t *testing.T) {
	_, err := dockerbe.New(dockerbe.Config{Client: &fakeDocker{}, BindIP: "0.0.0.0"})
	if !errors.Is(err, backend.ErrNonLoopbackPublish) {
		t.Fatalf("New(BindIP 0.0.0.0) = %v, want an error wrapping ErrNonLoopbackPublish", err)
	}
}

func TestNewAllowsANonLoopbackBindIPOnlyWhenSaidOnPurpose(t *testing.T) {
	b, err := dockerbe.New(dockerbe.Config{Client: &fakeDocker{}, BindIP: "0.0.0.0", AllowNonLoopback: true})
	if err != nil {
		t.Fatalf("New(BindIP 0.0.0.0, AllowNonLoopback) = %v, want nil", err)
	}
	if b.Kind() != backend.KindDocker {
		t.Errorf("Kind() = %q, want %q", b.Kind(), backend.KindDocker)
	}
}

func TestCreateBuildsTheContractedContainerAndReturnsALoopbackEndpoint(t *testing.T) {
	addr := healthzAddr(t)
	f := &fakeDocker{createID: "c0ffeecafe"}
	f.inspects = []inspectResult{
		{err: notFoundErr{"panacea-app-s-3f2a91c0"}},
		{resp: inspectOf("c0ffeecafe", "running", "127.0.0.1", portOf(t, addr), labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	sb, err := b.Create(context.Background(), validSpec())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	creates := f.snapshotCreates()
	if len(creates) != 1 {
		t.Fatalf("ContainerCreate called %d times, want 1", len(creates))
	}
	c := creates[0]
	if c.name != "panacea-app-s-3f2a91c0" {
		t.Errorf("container name = %q, want %q", c.name, "panacea-app-s-3f2a91c0")
	}
	if c.networking != nil || c.platform != nil {
		t.Errorf("networkingConfig=%v platform=%v, want both nil (default bridge, image platform)", c.networking, c.platform)
	}
	if c.config.Tty || c.config.OpenStdin {
		t.Errorf("Tty=%v OpenStdin=%v, want false/false — sandboxd owns the PTY, not Docker", c.config.Tty, c.config.OpenStdin)
	}
	if c.config.WorkingDir != "/work" || c.config.Image != "panacea-app/sandbox:dev" {
		t.Errorf("WorkingDir=%q Image=%q", c.config.WorkingDir, c.config.Image)
	}
	if _, ok := c.config.ExposedPorts[nat.Port("8080/tcp")]; !ok {
		t.Errorf("ExposedPorts = %v, want 8080/tcp", c.config.ExposedPorts)
	}

	wantEnv := []string{
		"PANACEA_APP_COMMAND=claude",
		`PANACEA_APP_ARGS_JSON=["--model","opus","-p","refactor auth, then test"]`,
		"PANACEA_APP_ATTACH_TOKEN=tok-abc",
	}
	if !reflect.DeepEqual(c.config.Env, wantEnv) {
		t.Errorf("Env =\n%q\nwant\n%q", c.config.Env, wantEnv)
	}
	if got := c.config.Labels[backend.LabelManaged]; got != "true" {
		t.Errorf("label %s = %q, want %q — List discovers on this label alone", backend.LabelManaged, got, "true")
	}
	if got := c.config.Labels[backend.LabelSandboxID]; got != "s-3f2a91c0" {
		t.Errorf("label %s = %q, want %q", backend.LabelSandboxID, got, "s-3f2a91c0")
	}
	if got := c.config.Labels[backend.LabelSessionID]; got != "sess-1" {
		t.Errorf("label %s = %q, want %q", backend.LabelSessionID, got, "sess-1")
	}
	if _, err := time.Parse(time.RFC3339, c.config.Labels[backend.LabelCreatedAt]); err != nil {
		t.Errorf("label %s = %q, want RFC3339: %v", backend.LabelCreatedAt, c.config.Labels[backend.LabelCreatedAt], err)
	}
	if len(c.config.Labels) != 4 {
		t.Errorf("labels = %v, want exactly the four contract labels", c.config.Labels)
	}
	for _, v := range c.config.Labels {
		if v == "tok-abc" {
			t.Fatalf("the attach token is a label: docker inspect ends up in bug reports")
		}
	}

	bindings := c.hostConfig.PortBindings[nat.Port("8080/tcp")]
	want := []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: ""}}
	if !reflect.DeepEqual(bindings, want) {
		t.Errorf("PortBindings = %#v, want %#v — an empty HostIP publishes on every interface", bindings, want)
	}
	if c.hostConfig.AutoRemove {
		t.Errorf("AutoRemove = true: a self-removing container deletes the transcript claude --resume reads")
	}
	if c.hostConfig.RestartPolicy.Name != container.RestartPolicyDisabled {
		t.Errorf("RestartPolicy = %q, want %q", c.hostConfig.RestartPolicy.Name, container.RestartPolicyDisabled)
	}
	if c.hostConfig.Resources.NanoCPUs != 2_000_000_000 || c.hostConfig.Resources.Memory != 4096<<20 {
		t.Errorf("Resources = %d nanocpus / %d bytes", c.hostConfig.Resources.NanoCPUs, c.hostConfig.Resources.Memory)
	}
	if !reflect.DeepEqual(c.hostConfig.Binds, []string{"C:/repo:/work"}) {
		t.Errorf("Binds = %q, want %q", c.hostConfig.Binds, []string{"C:/repo:/work"})
	}

	if sb.Kind != backend.KindDocker || sb.Ref != "c0ffeecafe" || sb.ID != "s-3f2a91c0" {
		t.Errorf("Sandbox = %+v", sb)
	}
	if sb.Endpoint != "127.0.0.1:"+portOf(t, addr) {
		t.Errorf("Endpoint = %q, want 127.0.0.1:%s", sb.Endpoint, portOf(t, addr))
	}
	if sb.State != backend.StateRunning {
		t.Errorf("State = %q, want %q", sb.State, backend.StateRunning)
	}
}

// A caller that could set PANACEA_APP_ATTACH_TOKEN could replace the sandbox's
// secret. ValidateSpec is the first statement of Create in both backends.
func TestCreateRejectsAReservedEnvKey(t *testing.T) {
	f := &fakeDocker{createID: "c0ffeecafe"}
	b := newBackend(t, f)
	spec := validSpec()
	spec.Env = map[string]string{"PANACEA_APP_ATTACH_TOKEN": "mine"}

	_, err := b.Create(context.Background(), spec)
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("Create with a reserved env key = %v, want an error wrapping ErrUnsupported", err)
	}
	if len(f.snapshotCreates()) != 0 {
		t.Errorf("ContainerCreate was called despite an invalid spec")
	}
}

func TestCreateIsErrAlreadyExistsWhenTheNameIsTaken(t *testing.T) {
	f := &fakeDocker{createID: "c0ffeecafe"}
	f.inspects = []inspectResult{
		{resp: inspectOf("older", "running", "127.0.0.1", "51000", labelsFor("s-3f2a91c0", "sess-0"))},
	}
	b := newBackend(t, f)

	_, err := b.Create(context.Background(), validSpec())
	if !errors.Is(err, backend.ErrAlreadyExists) {
		t.Fatalf("Create over an existing name = %v, want an error wrapping ErrAlreadyExists", err)
	}
	if len(f.snapshotCreates()) != 0 {
		t.Errorf("ContainerCreate was called over an existing container")
	}
}

// THE security test. A sandbox published on 0.0.0.0 is an unauthenticated shell
// on the corporate LAN. Create must not hand the caller an endpoint for it, and
// must not leave it running.
func TestCreateDestroysTheSandboxWhenDockerPublishesOffLoopback(t *testing.T) {
	f := &fakeDocker{createID: "c0ffeecafe"}
	f.inspects = []inspectResult{
		{err: notFoundErr{"panacea-app-s-3f2a91c0"}},
		{resp: inspectOf("c0ffeecafe", "running", "0.0.0.0", "51000", labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	_, err := b.Create(context.Background(), validSpec())
	if !errors.Is(err, backend.ErrNonLoopbackPublish) {
		t.Fatalf("Create with a 0.0.0.0 binding = %v, want an error wrapping ErrNonLoopbackPublish", err)
	}
	if got := f.snapshotRemoves(); len(got) != 1 || got[0] != "c0ffeecafe" {
		t.Fatalf("removes = %v, want [c0ffeecafe]: a LAN-published sandbox must not survive Create", got)
	}
}

func TestCreateDestroysTheSandboxThatNeverAnswersHealthz(t *testing.T) {
	dead := deadAddr(t)
	f := &fakeDocker{createID: "c0ffeecafe"}
	f.inspects = []inspectResult{
		{err: notFoundErr{"panacea-app-s-3f2a91c0"}},
		{resp: inspectOf("c0ffeecafe", "running", "127.0.0.1", portOf(t, dead), labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b, err := dockerbe.New(dockerbe.Config{Client: f, ReadyTimeout: 400 * time.Millisecond})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = b.Create(context.Background(), validSpec())
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Create of a sandbox that never serves /healthz = %v, want an error wrapping ErrNotRunning", err)
	}
	if got := f.snapshotRemoves(); len(got) != 1 || got[0] != "c0ffeecafe" {
		t.Fatalf("removes = %v, want [c0ffeecafe]: Create is all-or-nothing", got)
	}
}

// ReadyTimeout is one budget across both of Create's waits, not one per wait.
// A daemon that publishes the port slowly and a sandboxd that never answers
// /healthz must together cost ReadyTimeout, because an operator who set it and
// then waited twice it cannot tell which of the two went wrong.
//
// Nine unpublished inspects hold the port back for about nine endpoint polls,
// spending most of the budget before /healthz is even dialled. The bound is
// deliberately loose and one-sided: a fast machine can never fail it, and the
// two-budget bug overshoots it by enough that scheduling noise cannot hide it.
func TestCreateSpendsOneReadyBudgetAcrossBothWaits(t *testing.T) {
	const budget = time.Second
	dead := deadAddr(t)
	f := &fakeDocker{createID: "c0ffeecafe"}
	f.inspects = []inspectResult{{err: notFoundErr{"panacea-app-s-3f2a91c0"}}}
	unpublished := inspectOf("c0ffeecafe", "running", "", "", labelsFor("s-3f2a91c0", "sess-1"))
	for i := 0; i < 9; i++ {
		f.inspects = append(f.inspects, inspectResult{resp: unpublished})
	}
	f.inspects = append(f.inspects, inspectResult{
		resp: inspectOf("c0ffeecafe", "running", "127.0.0.1", portOf(t, dead), labelsFor("s-3f2a91c0", "sess-1")),
	})
	b, err := dockerbe.New(dockerbe.Config{Client: f, ReadyTimeout: budget})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	start := time.Now()
	_, err = b.Create(context.Background(), validSpec())
	elapsed := time.Since(start)

	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Create of a slow-to-publish sandbox that never serves /healthz = %v, want an error wrapping ErrNotRunning", err)
	}
	if elapsed > budget+budget/2 {
		t.Fatalf("Create took %v with ReadyTimeout %v: the port wait and the /healthz wait must share one deadline, not take one each", elapsed, budget)
	}
	if got := f.snapshotRemoves(); len(got) != 1 || got[0] != "c0ffeecafe" {
		t.Fatalf("removes = %v, want [c0ffeecafe]: an expired ready budget still destroys the container", got)
	}
}

func TestCreateRemovesTheContainerWhenStartFails(t *testing.T) {
	f := &fakeDocker{createID: "c0ffeecafe", startErr: errors.New("driver failed programming external connectivity")}
	f.inspects = []inspectResult{{err: notFoundErr{"panacea-app-s-3f2a91c0"}}}
	b := newBackend(t, f)

	_, err := b.Create(context.Background(), validSpec())
	if err == nil {
		t.Fatalf("Create with a failing start = nil, want an error")
	}
	if got := f.snapshotRemoves(); len(got) != 1 || got[0] != "c0ffeecafe" {
		t.Fatalf("removes = %v, want [c0ffeecafe]: a created-but-unstarted container is a leak", got)
	}
}

func TestGetMapsInspectOntoASandbox(t *testing.T) {
	f := &fakeDocker{}
	f.inspects = []inspectResult{
		{resp: inspectOf("c0ffeecafe", "running", "127.0.0.1", "51000", labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	sb, err := b.Get(context.Background(), "s-3f2a91c0")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sb.ID != "s-3f2a91c0" || sb.Ref != "c0ffeecafe" || sb.Endpoint != "127.0.0.1:51000" {
		t.Errorf("Sandbox = %+v", sb)
	}
	if sb.State != backend.StateRunning {
		t.Errorf("State = %q, want %q", sb.State, backend.StateRunning)
	}
	if !sb.CreatedAt.Equal(time.Date(2026, 9, 18, 1, 2, 3, 400000000, time.UTC)) {
		t.Errorf("CreatedAt = %s, want 2026-09-18T01:02:03.4Z", sb.CreatedAt)
	}
	if f.inspectNames[0] != "panacea-app-s-3f2a91c0" {
		t.Errorf("inspected %q, want the prefixed container name", f.inspectNames[0])
	}
}

func TestGetIsErrNotFoundForAnUnknownSandbox(t *testing.T) {
	b := newBackend(t, &fakeDocker{})

	_, err := b.Get(context.Background(), "s-nope")
	if !errors.Is(err, backend.ErrNotFound) {
		t.Fatalf("Get of an unknown sandbox = %v, want an error wrapping ErrNotFound", err)
	}
}

func TestGetRefusesToHandBackAnOffLoopbackEndpoint(t *testing.T) {
	f := &fakeDocker{}
	f.inspects = []inspectResult{
		{resp: inspectOf("c0ffeecafe", "running", "0.0.0.0", "51000", labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	_, err := b.Get(context.Background(), "s-3f2a91c0")
	if !errors.Is(err, backend.ErrNonLoopbackPublish) {
		t.Fatalf("Get of a 0.0.0.0-published sandbox = %v, want an error wrapping ErrNonLoopbackPublish", err)
	}
}

// The daemon publishes the port after the container starts. A running sandbox
// with no binding yet is starting, not broken.
func TestGetOfAnUnpublishedContainerIsStartingWithNoEndpoint(t *testing.T) {
	f := &fakeDocker{}
	f.inspects = []inspectResult{
		{resp: inspectOf("c0ffeecafe", "created", "", "", labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	sb, err := b.Get(context.Background(), "s-3f2a91c0")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sb.State != backend.StateStarting || sb.Endpoint != "" {
		t.Fatalf("Sandbox = %+v, want starting with an empty endpoint", sb)
	}
}

func TestListDiscoversOnTheManagedLabelAndSkipsForeignContainers(t *testing.T) {
	f := &fakeDocker{list: []container.Summary{
		{
			ID:      "c1",
			Created: 1758157323,
			State:   "running",
			Labels:  labelsFor("s-aaaaaaaa", "sess-1"),
			Ports:   []container.Port{{IP: "127.0.0.1", PrivatePort: 8080, PublicPort: 51001, Type: "tcp"}},
		},
		{
			ID:     "c2",
			State:  "running",
			Labels: map[string]string{backend.LabelManaged: "true"}, // no sandbox id
		},
	}}
	b := newBackend(t, f)

	got, err := b.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List returned %d sandboxes, want 1 — a container with no %s label is not ours to manage", len(got), backend.LabelSandboxID)
	}
	if got[0].ID != "s-aaaaaaaa" || got[0].Endpoint != "127.0.0.1:51001" || got[0].Ref != "c1" {
		t.Errorf("Sandbox = %+v", got[0])
	}
	if got[0].NonLoopback {
		t.Errorf("NonLoopback = true for a 127.0.0.1 binding, want false")
	}
	if !got[0].CreatedAt.Equal(time.Unix(1758157323, 0).UTC()) {
		t.Errorf("CreatedAt = %s, want the Unix Created field in UTC", got[0].CreatedAt)
	}
	if len(f.listOpts) != 1 || !f.listOpts[0].All {
		t.Fatalf("ListOptions = %+v, want All:true — the reaper must see exited containers", f.listOpts)
	}
	if want := []string{backend.LabelManaged + "=true"}; !reflect.DeepEqual(f.listOpts[0].Filters.Get("label"), want) {
		t.Errorf("label filter = %v, want %v", f.listOpts[0].Filters.Get("label"), want)
	}
}

func TestListMapsEveryDockerStateOntoABackendState(t *testing.T) {
	cases := map[string]backend.State{
		"running":    backend.StateRunning,
		"created":    backend.StateStarting,
		"restarting": backend.StateStarting,
		"exited":     backend.StateStopped,
		"paused":     backend.StateStopped,
		"dead":       backend.StateStopped,
		"removing":   backend.StateStopped,
	}
	for dockerState, want := range cases {
		f := &fakeDocker{list: []container.Summary{
			{ID: "c1", State: dockerState, Labels: labelsFor("s-aaaaaaaa", "sess-1")},
		}}
		b := newBackend(t, f)

		got, err := b.List(context.Background())
		if err != nil {
			t.Fatalf("%s: List: %v", dockerState, err)
		}
		if len(got) != 1 || got[0].State != want {
			t.Errorf("docker state %q mapped to %v, want %q", dockerState, got, want)
		}
	}
}

// List must not fail a whole sweep over one bad container: the reaper needs to
// see it in order to destroy it, and Get is where the refusal is loud. But it
// must not come back looking normal either. An off-loopback publish is the
// defect this phase exists to prevent, and a sandbox returned with nothing but
// an empty Endpoint is indistinguishable from one the daemon has not published
// yet — so the condition is carried explicitly in Sandbox.NonLoopback.
//
// State stays whatever the daemon reports. Overwriting a running container's
// state with "stopped" to make the reaper collect it would put a fabricated
// value into the one field that records observed reality, and task 11 renders
// that field to the user.
func TestListFlagsAnOffLoopbackBindingWithoutFailingTheSweep(t *testing.T) {
	f := &fakeDocker{list: []container.Summary{
		{
			ID:     "c1",
			State:  "running",
			Labels: labelsFor("s-aaaaaaaa", "sess-1"),
			Ports:  []container.Port{{IP: "0.0.0.0", PrivatePort: 8080, PublicPort: 51001, Type: "tcp"}},
		},
		{
			ID:     "c2",
			State:  "running",
			Labels: labelsFor("s-bbbbbbbb", "sess-2"),
			Ports:  []container.Port{{IP: "127.0.0.1", PrivatePort: 8080, PublicPort: 51002, Type: "tcp"}},
		},
	}}
	b, logBuf := newBackendWithLog(t, f)

	got, err := b.List(context.Background())
	if err != nil {
		t.Fatalf("List = %v, want nil: one bad container must not blind the reaper to the rest", err)
	}
	if len(got) != 2 {
		t.Fatalf("List = %+v, want both sandboxes: the healthy one must survive the bad one", got)
	}

	bad, good := got[0], got[1]
	if bad.ID != "s-aaaaaaaa" || good.ID != "s-bbbbbbbb" {
		t.Fatalf("List = %+v, want the summaries in order", got)
	}
	if !bad.NonLoopback {
		t.Errorf("NonLoopback = false for a 0.0.0.0 binding, want true: the security signal is the only thing that distinguishes this from a container the daemon has not published yet")
	}
	if bad.Endpoint != "" {
		t.Errorf("Endpoint = %q, want empty: nothing may attach to an off-loopback publish", bad.Endpoint)
	}
	if bad.State != backend.StateRunning {
		t.Errorf("State = %v, want %v: the daemon reports this container running and State records observed reality", bad.State, backend.StateRunning)
	}
	if good.NonLoopback || good.Endpoint != "127.0.0.1:51002" {
		t.Errorf("the loopback-published sandbox came back %+v, want an untouched endpoint and NonLoopback false", good)
	}
	line := logBuf.String()
	if !strings.Contains(line, "s-aaaaaaaa") || !strings.Contains(line, "loopback") || !strings.Contains(line, "0.0.0.0") {
		t.Errorf("log = %q, want one warning naming the sandbox, the off-loopback publish and the offending host IP", line)
	}
	if strings.Contains(line, "s-bbbbbbbb") {
		t.Errorf("log = %q, want no warning for the loopback-published sandbox", line)
	}
}

// Task 13's reaper and an explicit DELETE race by design. A reaper that errors
// on "already gone" stops sweeping.
func TestDestroyOfAnAlreadyGoneSandboxIsSuccess(t *testing.T) {
	f := &fakeDocker{removeErr: notFoundErr{"panacea-app-s-3f2a91c0"}}
	b := newBackend(t, f)

	if err := b.Destroy(context.Background(), "s-3f2a91c0"); err != nil {
		t.Fatalf("Destroy of a gone sandbox = %v, want nil", err)
	}
}

func TestDestroyIsIdempotentAcrossRepeatedCalls(t *testing.T) {
	f := &fakeDocker{}
	b := newBackend(t, f)

	for i := 0; i < 3; i++ {
		if err := b.Destroy(context.Background(), "s-3f2a91c0"); err != nil {
			t.Fatalf("Destroy call %d = %v, want nil", i+1, err)
		}
	}
	if got := f.snapshotRemoves(); len(got) != 3 {
		t.Fatalf("removes = %v, want three calls", got)
	}
	if !f.removeOpts[0].Force || !f.removeOpts[0].RemoveVolumes {
		t.Errorf("RemoveOptions = %+v, want Force and RemoveVolumes", f.removeOpts[0])
	}
}

func TestDestroySurfacesARealDaemonFailure(t *testing.T) {
	f := &fakeDocker{removeErr: errors.New("connection refused")}
	b := newBackend(t, f)

	err := b.Destroy(context.Background(), "s-3f2a91c0")
	if !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("Destroy with a dead daemon = %v, want an error wrapping ErrUnavailable", err)
	}
}

func TestAttachIsErrNotRunningWhenTheSandboxHasNoEndpoint(t *testing.T) {
	f := &fakeDocker{}
	f.inspects = []inspectResult{
		{resp: inspectOf("c0ffeecafe", "exited", "", "", labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	_, err := b.Attach(context.Background(), "s-3f2a91c0", "tok-abc")
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Attach to an exited sandbox = %v, want an error wrapping ErrNotRunning", err)
	}
}

func TestAttachOfAnUnknownSandboxIsErrNotFound(t *testing.T) {
	b := newBackend(t, &fakeDocker{})

	_, err := b.Attach(context.Background(), "s-nope", "tok-abc")
	if !errors.Is(err, backend.ErrNotFound) {
		t.Fatalf("Attach to an unknown sandbox = %v, want an error wrapping ErrNotFound", err)
	}
}

// Attach goes through Get, so an off-loopback publish refuses here too. This is
// the pair to List: the sweep flags it, every path that could hand out a
// connection to it refuses.
func TestAttachRefusesAnOffLoopbackSandbox(t *testing.T) {
	f := &fakeDocker{}
	f.inspects = []inspectResult{
		{resp: inspectOf("c0ffeecafe", "running", "0.0.0.0", "51000", labelsFor("s-3f2a91c0", "sess-1"))},
	}
	b := newBackend(t, f)

	_, err := b.Attach(context.Background(), "s-3f2a91c0", "tok-abc")
	if !errors.Is(err, backend.ErrNonLoopbackPublish) {
		t.Fatalf("Attach to a 0.0.0.0-published sandbox = %v, want an error wrapping ErrNonLoopbackPublish", err)
	}
}

func TestCloseClosesTheDockerClient(t *testing.T) {
	f := &fakeDocker{}
	b := newBackend(t, f)

	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !f.closed {
		t.Errorf("Close did not close the Docker client; a leaked npipe handle survives every restart")
	}
}

func TestBackendSatisfiesTheBackendInterface(t *testing.T) {
	var _ backend.Backend = (*dockerbe.Backend)(nil)
}
