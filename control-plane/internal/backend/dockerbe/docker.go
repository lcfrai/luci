package dockerbe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
)

const (
	// namePrefix makes `docker ps` legible during an incident, and gives
	// Create a cheap existence check: one inspect by name, not a list.
	namePrefix = "panacea-app-"

	// loopbackIPv4 is the only interface a sandbox port may be published to.
	loopbackIPv4 = "127.0.0.1"

	pingTimeout          = 5 * time.Second
	endpointPollInterval = 100 * time.Millisecond
	cleanupTimeout       = 20 * time.Second
)

// attachPort is derived from backend.ContainerPort rather than written as the
// literal "8080/tcp", so the container port and the published port cannot drift
// apart. nat.Port is a string type, so this is a plain conversion.
var attachPort = nat.Port(strconv.Itoa(backend.ContainerPort) + "/tcp")

// The real SDK client must keep satisfying the slice in api.go. An SDK bump
// that renames a type breaks the build here, not at run time.
var _ APIClient = (*client.Client)(nil)

var _ backend.Backend = (*Backend)(nil)

// Config configures the Docker backend.
type Config struct {
	// Client is the Docker API. Leave nil and New builds one from the
	// environment with API-version negotiation.
	Client APIClient

	// Host overrides DOCKER_HOST. Empty means "use the environment", which on
	// Windows with Docker Desktop is npipe:////./pipe/docker_engine.
	Host string

	// BindIP is the host interface sandbox ports are published to. It defaults
	// to 127.0.0.1 and New rejects anything else unless AllowNonLoopback is
	// set: publishing a sandbox to 0.0.0.0 puts an unauthenticated shell on the
	// LAN, guarded only by a bearer token that is also on that LAN.
	BindIP string

	// AllowNonLoopback exists so the refusal above is a decision someone has to
	// make on purpose. Nothing in Phase 2 sets it.
	AllowNonLoopback bool

	// ReadyTimeout is one budget spanning both halves of Create's wait: for
	// the daemon to publish a host port, and for sandboxd to answer /healthz.
	// Whatever the first half spends, the second half does not get, so Create
	// returns within ReadyTimeout and not within twice it. Zero means
	// backend.ReadyTimeout. It is a field rather than a constant so the
	// ready-failure path is testable in under a second. Nothing sets it from
	// the environment yet: PANACEA_APP_READY_TIMEOUT does not exist, and task
	// 13 is where it has to be added if an operator is ever to raise it.
	ReadyTimeout time.Duration

	// Log receives the one warning List emits for a container published off
	// loopback. Nil means slog.Default(). This is not decoration: List cannot
	// return ErrNonLoopbackPublish without blinding the reaper to the very
	// container that needs destroying, so this line is the only human-readable
	// place the condition surfaces during a sweep — the machine-readable one is
	// backend.Sandbox.NonLoopback. Get and Attach still refuse loudly.
	Log *slog.Logger
}

// Backend implements backend.Backend over Docker.
type Backend struct {
	api          APIClient
	bindIP       string
	readyTimeout time.Duration
	log          *slog.Logger
}

// New returns a Docker backend, or fails with backend.ErrUnavailable and a
// sentence the operator can act on. A Docker Desktop that is installed but not
// running must fail here, not later with a nil dereference.
func New(cfg Config) (*Backend, error) {
	bindIP := cfg.BindIP
	if bindIP == "" {
		bindIP = loopbackIPv4
	}
	if bindIP != loopbackIPv4 && !cfg.AllowNonLoopback {
		return nil, fmt.Errorf("dockerbe: BindIP %q would publish sandbox ports off loopback; set AllowNonLoopback to mean it: %w", bindIP, backend.ErrNonLoopbackPublish)
	}

	readyTimeout := cfg.ReadyTimeout
	if readyTimeout <= 0 {
		readyTimeout = backend.ReadyTimeout
	}

	logger := cfg.Log
	if logger == nil {
		logger = slog.Default()
	}

	api := cfg.Client
	built := false
	if api == nil {
		opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
		if cfg.Host != "" {
			opts = append(opts, client.WithHost(cfg.Host))
		}
		c, err := client.NewClientWithOpts(opts...)
		if err != nil {
			return nil, fmt.Errorf("dockerbe: cannot build a Docker client (check DOCKER_HOST): %v: %w", err, backend.ErrUnavailable)
		}
		api, built = c, true
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if _, err := api.Ping(pingCtx); err != nil {
		if built {
			_ = api.Close() // close only what we built; never the caller's client
		}
		return nil, fmt.Errorf("dockerbe: no Docker daemon answered — start Docker Desktop, or set PANACEA_APP_DOCKER_HOST: %v: %w", err, backend.ErrUnavailable)
	}
	return &Backend{api: api, bindIP: bindIP, readyTimeout: readyTimeout, log: logger}, nil
}

func (b *Backend) Kind() backend.Kind { return backend.KindDocker }

func (b *Backend) Close() error { return b.api.Close() }

// Create is all-or-nothing: every path that fails after ContainerCreate removes
// the container before returning. A half-created sandbox has no session record,
// so nothing but the reaper would ever find it again.
func (b *Backend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
	if err := backend.ValidateSpec(spec); err != nil {
		return backend.Sandbox{}, err
	}
	// A machine mount is a sub-path of a cloud machine's disk, which a local
	// daemon cannot reach. Ignoring the field would start the agent in an empty
	// directory with no error anywhere — the mirror of katabe's mount refusal.
	if len(spec.MachineMounts) > 0 || spec.Machine != "" {
		return backend.Sandbox{}, fmt.Errorf("dockerbe: sandbox %q is on machine %q with %d machine mount(s), which a local Docker daemon has no disk for: %w", spec.ID, spec.Machine, len(spec.MachineMounts), backend.ErrUnsupported)
	}
	name := namePrefix + spec.ID

	// A name collision is how ErrAlreadyExists is detected: inspect the name
	// first rather than decoding the daemon's 409.
	if _, err := b.api.ContainerInspect(ctx, name); err == nil {
		return backend.Sandbox{}, fmt.Errorf("dockerbe: container %s: %w", name, backend.ErrAlreadyExists)
	} else if !client.IsErrNotFound(err) {
		return backend.Sandbox{}, fmt.Errorf("dockerbe: inspect %s: %v: %w", name, err, backend.ErrUnavailable)
	}

	// The environment comes from the seam, never from spec.Args directly:
	// Spec.ArgsJSON renders an empty argv as "[]" where json.Marshal of a nil
	// slice renders "null", which sandboxd rejects with exit 2, and
	// Spec.EnvEntries drops any user key that collides with a reserved
	// PANACEA_APP_* name so it cannot shadow the attach token.
	env := make([]string, 0, 3+len(spec.Env))
	for _, e := range spec.EnvEntries() {
		env = append(env, e.Name+"="+e.Value)
	}
	binds := make([]string, 0, len(spec.Mounts))
	for _, m := range spec.Mounts {
		bind := m.HostPath + ":" + m.ContainerPath
		if m.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}

	cfg := &container.Config{
		Image:        spec.Image,
		Labels:       containerLabels(spec),
		Env:          env,
		ExposedPorts: nat.PortSet{attachPort: struct{}{}},
		WorkingDir:   spec.WorkDir,
		Tty:          false, // sandboxd owns the PTY, not Docker
		OpenStdin:    false,
	}
	hostCfg := &container.HostConfig{
		// HostPort "" asks the daemon for an ephemeral port. HostIP is the
		// whole security property of this package.
		PortBindings:  nat.PortMap{attachPort: {{HostIP: b.bindIP, HostPort: ""}}},
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
		// AutoRemove false is load-bearing: destroy happens on explicit kill,
		// idle TTL or reap, never on disconnect, and a self-removing container
		// would delete the transcript `claude --resume` reads.
		AutoRemove: false,
		Binds:      binds,
		Resources: container.Resources{
			NanoCPUs: spec.Resources.NanoCPUs(),
			Memory:   spec.Resources.MemoryBytes(),
		},
	}

	// networkingConfig nil and NetworkMode unset means the default bridge.
	// Host networking is deliberately not used: Kata cannot support it and the
	// two backends must not diverge in what a sandbox can reach. platform nil
	// lets Docker Desktop pick the image's platform.
	created, err := b.api.ContainerCreate(ctx, cfg, hostCfg, nil, nil, name)
	if err != nil {
		return backend.Sandbox{}, fmt.Errorf("dockerbe: create %s: %v: %w", name, err, backend.ErrUnavailable)
	}
	if err := b.api.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		b.cleanup(ctx, created.ID)
		return backend.Sandbox{}, fmt.Errorf("dockerbe: start %s: %v: %w", name, err, backend.ErrUnavailable)
	}

	// One deadline for both waits. Computing it here rather than inside each
	// wait is what keeps Create's worst case at readyTimeout instead of twice
	// it.
	deadline := time.Now().Add(b.readyTimeout)

	endpoint, err := b.waitForEndpoint(ctx, name, deadline)
	if err != nil {
		b.cleanup(ctx, created.ID)
		return backend.Sandbox{}, err
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		// The port arrived with the budget already spent. Calling WaitReady
		// with a non-positive timeout would poll zero times and report a last
		// error of <nil>; say what actually happened instead.
		b.cleanup(ctx, created.ID)
		return backend.Sandbox{}, fmt.Errorf("dockerbe: sandbox %s published %s with no time left in the %s ready budget to answer %s: %w", spec.ID, endpoint, b.readyTimeout, backend.HealthPath, backend.ErrNotRunning)
	}
	if err := backend.WaitReady(ctx, endpoint, remaining); err != nil {
		b.cleanup(ctx, created.ID)
		return backend.Sandbox{}, fmt.Errorf("dockerbe: sandbox %s never answered %s on %s within the %s ready budget: %w", spec.ID, backend.HealthPath, endpoint, b.readyTimeout, err)
	}

	// The final read is a failure path like any other, and the failure it is
	// most likely to report is ErrNonLoopbackPublish — which would otherwise
	// mean Create tells the caller "this sandbox is on the LAN" and leaves it
	// there. All-or-nothing has no exceptions.
	sb, err := b.Get(ctx, spec.ID)
	if err != nil {
		b.cleanup(ctx, created.ID)
		return backend.Sandbox{}, err
	}
	return sb, nil
}

// containerLabels is every label this package writes. The attach token is
// deliberately absent: a label is readable by anything that can reach the
// Docker socket and is echoed into `docker inspect` output that ends up in bug
// reports.
func containerLabels(spec backend.Spec) map[string]string {
	return map[string]string{
		backend.LabelManaged:   "true",
		backend.LabelSandboxID: spec.ID,
		backend.LabelSessionID: spec.SessionID,
		backend.LabelCreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// cleanup removes a container on a failed Create. It uses a context detached
// from the caller's, because the most common reason Create fails is that the
// caller's context expired, and that is exactly when the container must still
// be removed.
func (b *Backend) cleanup(ctx context.Context, ref string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	_ = b.api.ContainerRemove(cleanupCtx, ref, container.RemoveOptions{Force: true, RemoveVolumes: true})
}

// waitForEndpoint polls inspect until the daemon reports a host binding. The
// daemon publishes the port after the container starts, so a freshly started
// container legitimately has none for a few hundred milliseconds. deadline is
// Create's, shared with the /healthz wait that follows, so this function takes
// it rather than deriving one of its own.
func (b *Backend) waitForEndpoint(ctx context.Context, name string, deadline time.Time) (string, error) {
	for {
		resp, err := b.api.ContainerInspect(ctx, name)
		if err != nil {
			// A caller who gave up is not a broken daemon. Classify that
			// before the ErrUnavailable wrap below, or the same cancellation
			// maps to 503 or to 409 depending on which statement noticed it
			// first — the inspect, or the sleep at the bottom of the loop.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", abandonedWait(name, ctxErr)
			}
			if client.IsErrNotFound(err) {
				return "", fmt.Errorf("dockerbe: container %s vanished while starting: %w", name, backend.ErrNotRunning)
			}
			return "", fmt.Errorf("dockerbe: inspect %s: %v: %w", name, err, backend.ErrUnavailable)
		}
		endpoint, err := b.endpointFromPortMap(portsOf(resp))
		if err != nil {
			return "", err
		}
		if endpoint != "" {
			return endpoint, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("dockerbe: container %s published no %s binding on %s within the %s ready budget: %w", name, attachPort, b.bindIP, b.readyTimeout, backend.ErrNotRunning)
		}
		select {
		case <-ctx.Done():
			return "", abandonedWait(name, ctx.Err())
		case <-time.After(endpointPollInterval):
		}
	}
}

// abandonedWait classifies a caller's cancellation. It is not returned raw:
// task 13 maps errors with errors.Is, and a bare context error matches no
// sentinel and falls through to 500. Both wraps are kept, so a caller can still
// ask which cancellation it was.
func abandonedWait(name string, ctxErr error) error {
	return fmt.Errorf("dockerbe: waiting for container %s to publish %s: %w: %w", name, attachPort, ctxErr, backend.ErrNotRunning)
}

func (b *Backend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	name := namePrefix + id
	resp, err := b.api.ContainerInspect(ctx, name)
	if err != nil {
		if client.IsErrNotFound(err) {
			return backend.Sandbox{}, fmt.Errorf("dockerbe: sandbox %s: %w", id, backend.ErrNotFound)
		}
		return backend.Sandbox{}, fmt.Errorf("dockerbe: inspect %s: %v: %w", name, err, backend.ErrUnavailable)
	}
	if resp.ContainerJSONBase == nil {
		return backend.Sandbox{}, fmt.Errorf("dockerbe: inspect %s returned no container body: %w", name, backend.ErrUnavailable)
	}
	endpoint, err := b.endpointFromPortMap(portsOf(resp))
	if err != nil {
		return backend.Sandbox{}, err
	}
	labels := map[string]string{}
	if resp.Config != nil && resp.Config.Labels != nil {
		labels = resp.Config.Labels
	}
	status := ""
	if resp.State != nil {
		status = resp.State.Status
	}
	return backend.Sandbox{
		ID:        id,
		Kind:      backend.KindDocker,
		Ref:       resp.ID,
		Endpoint:  endpoint,
		State:     stateFrom(status),
		CreatedAt: parseCreated(resp.Created, labels),
		Labels:    labels,
	}, nil
}

// List discovers by label, not by name pattern, and never fails the whole sweep
// over one bad container: task 13's reaper has to see a container in order to
// destroy it.
//
// This is a deliberate amendment to contract §2.4, which says Get/List must
// return backend.ErrNonLoopbackPublish. Get does. List must not, because one
// defective container would then stop every other sandbox from being reaped.
// What List does instead is make the container unmistakable rather than
// invisible: empty Endpoint so nothing can attach, NonLoopback so it cannot be
// read as a container the daemon is merely slow to publish, and one warning
// naming the offending bind address. State is left as the daemon reports it —
// backend.State records what was observed, and task 11 renders it — so the
// security signal is carried in its own field rather than encoded as a status
// the container does not have. Get and Attach on the same id still refuse
// loudly with ErrNonLoopbackPublish.
func (b *Backend) List(ctx context.Context) ([]backend.Sandbox, error) {
	opts := container.ListOptions{
		All:     true, // include exited containers; the reaper needs to see them
		Filters: filters.NewArgs(filters.Arg("label", backend.LabelManaged+"=true")),
	}
	summaries, err := b.api.ContainerList(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("dockerbe: list: %v: %w", err, backend.ErrUnavailable)
	}
	out := make([]backend.Sandbox, 0, len(summaries))
	for _, s := range summaries {
		id := s.Labels[backend.LabelSandboxID]
		if id == "" {
			continue // managed-looking but not ours to manage
		}
		endpoint, epErr := b.endpointFromSummaryPorts(s.Ports)
		nonLoopback := epErr != nil
		if nonLoopback {
			// Not returned: erroring the sweep blinds the reaper. Not hidden
			// either: NonLoopback plus no endpoint means nothing attaches to it
			// and nothing mistakes it for a normal starting container.
			endpoint = ""
			hostIP := ""
			var off offLoopbackError
			if errors.As(epErr, &off) {
				hostIP = off.hostIP
			}
			b.log.Warn("sandbox port is published off loopback; refusing to expose it",
				"sandbox", id, "container", s.ID, "host_ip", hostIP, "err", epErr)
		}
		out = append(out, backend.Sandbox{
			ID:          id,
			Kind:        backend.KindDocker,
			Ref:         s.ID,
			Endpoint:    endpoint,
			NonLoopback: nonLoopback,
			State:       stateFrom(s.State),
			CreatedAt:   createdFromUnix(s.Created, s.Labels),
			Labels:      s.Labels,
		})
	}
	return out, nil
}

func (b *Backend) Attach(ctx context.Context, id, token string) (*websocket.Conn, error) {
	sb, err := b.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sb.State != backend.StateRunning || sb.Endpoint == "" {
		return nil, fmt.Errorf("dockerbe: sandbox %s is %s with endpoint %q: %w", id, sb.State, sb.Endpoint, backend.ErrNotRunning)
	}
	return backend.Dial(ctx, sb.Endpoint, token)
}

// Destroy is idempotent. Task 13's reaper and an explicit DELETE race by
// design, so "already gone" is the outcome both wanted.
func (b *Backend) Destroy(ctx context.Context, id string) error {
	err := b.api.ContainerRemove(ctx, namePrefix+id, container.RemoveOptions{Force: true, RemoveVolumes: true})
	if err == nil || client.IsErrNotFound(err) {
		return nil
	}
	return fmt.Errorf("dockerbe: remove %s: %v: %w", id, err, backend.ErrUnavailable)
}

func portsOf(resp container.InspectResponse) nat.PortMap {
	if resp.NetworkSettings == nil {
		return nil
	}
	return resp.NetworkSettings.Ports
}

// offLoopbackError is the one way this package reports a sandbox port published
// somewhere it must not be. It carries the offending address as a field so a
// sweep can log it as a structured attribute rather than leaving the value to
// be scraped back out of a message, and it wraps backend.ErrNonLoopbackPublish,
// so every errors.Is check at the four refusal sites is unaffected.
type offLoopbackError struct {
	hostIP string
	bindIP string
}

func (e offLoopbackError) Error() string {
	return fmt.Sprintf("dockerbe: %s is published on %q instead of %s", attachPort, e.hostIP, e.bindIP)
}

func (e offLoopbackError) Unwrap() error { return backend.ErrNonLoopbackPublish }

// bindingIsSafe judges one published host address.
//
// The rule is loopback, decided by net.ParseIP rather than by comparing
// strings: that admits ::1, which this daemon has not been seen to produce but
// is no less loopback for it, and it rejects the empty spelling, because
// net.ParseIP("") is nil and Docker reads an empty HostIP as every interface —
// the defect this whole guard exists to catch. A binding on the configured
// bindIP is safe by definition: when that is not a loopback address, someone
// set AllowNonLoopback to mean it.
func (b *Backend) bindingIsSafe(hostIP string) bool {
	if hostIP == b.bindIP {
		return true
	}
	ip := net.ParseIP(hostIP)
	return ip != nil && ip.IsLoopback()
}

// endpointFromPortMap reads the inspect shape.
//
// It judges EVERY binding for the attach port rather than returning on the
// first usable one. The question this guard has to answer is "is every binding
// for this port safe?", not "is there a safe binding?", because it exists to
// establish the ABSENCE of LAN reachability and an existence check cannot
// establish an absence. One acceptable binding does not make a container safe;
// one unacceptable binding makes it unsafe whatever sits alongside it. Docker
// will publish a container port to 127.0.0.1 and 0.0.0.0 at once, and
// first-match-wins hands the caller the loopback one.
//
// An empty endpoint with a nil error means "not published yet", which for a
// starting container is normal.
func (b *Backend) endpointFromPortMap(ports nat.PortMap) (string, error) {
	var endpoint string
	for _, bd := range ports[attachPort] {
		if bd.HostPort == "" {
			continue // exposed, not yet published
		}
		if !b.bindingIsSafe(bd.HostIP) {
			return "", offLoopbackError{hostIP: bd.HostIP, bindIP: b.bindIP}
		}
		if bd.HostIP == b.bindIP {
			endpoint = net.JoinHostPort(b.bindIP, bd.HostPort)
		}
	}
	return endpoint, nil
}

// endpointFromSummaryPorts reads the list shape, which reports the same fact in
// a different struct. It scans every binding for the same reason.
func (b *Backend) endpointFromSummaryPorts(ports []container.Port) (string, error) {
	var endpoint string
	for _, p := range ports {
		if int(p.PrivatePort) != backend.ContainerPort || p.Type != "tcp" || p.PublicPort == 0 {
			continue
		}
		if !b.bindingIsSafe(p.IP) {
			return "", offLoopbackError{hostIP: p.IP, bindIP: b.bindIP}
		}
		if p.IP == b.bindIP {
			endpoint = net.JoinHostPort(b.bindIP, strconv.Itoa(int(p.PublicPort)))
		}
	}
	return endpoint, nil
}

// stateFrom maps container.ContainerState (a string alias whose constants are
// created, running, paused, restarting, removing, exited, dead) onto the four
// states a backend is allowed to know about.
func stateFrom(status string) backend.State {
	switch status {
	case "running":
		return backend.StateRunning
	case "created", "restarting":
		return backend.StateStarting
	default:
		return backend.StateStopped
	}
}

// parseCreated prefers the daemon's own timestamp and falls back to the label
// this package wrote, so a sandbox created by an older build still sorts. This
// is the inspect shape, which carries an RFC3339 string.
func parseCreated(raw string, labels map[string]string) time.Time {
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC()
	}
	return createdFromLabel(labels)
}

// createdFromUnix is the list shape, which carries a Unix second instead. A
// zero there is not 1970: it is the daemon not reporting one, and this is the
// field task 13's reaper ages containers off, so falling through to the epoch
// would make every such sandbox 56 years old and immediately reapable.
func createdFromUnix(sec int64, labels map[string]string) time.Time {
	if sec > 0 {
		return time.Unix(sec, 0).UTC()
	}
	return createdFromLabel(labels)
}

func createdFromLabel(labels map[string]string) time.Time {
	if t, err := time.Parse(time.RFC3339, labels[backend.LabelCreatedAt]); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
