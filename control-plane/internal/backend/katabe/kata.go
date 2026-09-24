// Package katabe runs sandboxes as Kata Containers pods on AKS. On a laptop it
// is DORMANT: nothing selects it by default, and PANACEA_APP_BACKEND=kata
// refuses to run outside a cluster (cmd/api, seam S8.2).
//
// Phase 8 runs it INSIDE THE BROKER (task 18), not in the desktop's control
// plane: the broker builds one Backend per request, scoped with Config.Owner to
// the caller's sandbox.OwnerLabelValue, so one user can neither see nor reach
// another's pods. The pods themselves are hardened by internal/sandbox (task
// 14), and a sandbox's mounts arrive as machine mounts (task 15) rather than as
// host paths, which is why Create still refuses Spec.Mounts here.
package katabe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Backend implements backend.Backend. The assertion is the point of the
// package in Phase 2: if the seam ever grows a method Kubernetes cannot serve,
// this file stops compiling on a laptop instead of in Phase 8 on a cluster.
var _ backend.Backend = (*Backend)(nil)

// podPollInterval is how often Create re-reads the pod while it waits for a
// node. It matches the /healthz poll in backend.WaitReady, for the same reason:
// the gap between a 250ms and a 1s poll is the gap between a session that feels
// instant and one that does not.
const podPollInterval = 250 * time.Millisecond

// Config configures the Kata backend.
type Config struct {
	// Manager creates the pods. Required.
	Manager *sandbox.Manager
	// ReadyTimeout overrides backend.ReadyTimeout. Kata cold start is seconds
	// (docs/PROGRAM.md open risk 1), which is why Phase 8 needs warm pools.
	ReadyTimeout time.Duration
	// Port is the sandboxd port on the pod IP. Zero means backend.ContainerPort,
	// which is the only value a real cluster ever uses; it is settable so a test
	// can point the endpoint at an httptest server.
	Port int
	// Owner, when set, scopes this backend to one owner: New wraps Manager in
	// Manager.WithOwner(Owner). The broker builds one Backend per request with
	// the caller's OwnerLabelValue. "" is the unscoped backend cmd/api builds.
	Owner string
}

// Backend implements backend.Backend over Kata pods.
type Backend struct {
	mgr          *sandbox.Manager
	readyTimeout time.Duration
	port         int
}

// New returns a Kata backend. A nil Manager is ErrUnavailable rather than a
// panic at the first Create: "no kubeconfig on this machine" is a 503, and in
// Phase 2 it is the expected state of every developer laptop.
func New(cfg Config) (*Backend, error) {
	if cfg.Manager == nil {
		return nil, fmt.Errorf("katabe: no sandbox manager: %w", backend.ErrUnavailable)
	}
	mgr := cfg.Manager
	if cfg.Owner != "" {
		mgr = mgr.WithOwner(cfg.Owner)
	}
	b := &Backend{mgr: mgr, readyTimeout: cfg.ReadyTimeout, port: cfg.Port}
	if b.readyTimeout <= 0 {
		b.readyTimeout = backend.ReadyTimeout
	}
	if b.port <= 0 {
		b.port = backend.ContainerPort
	}
	return b, nil
}

// Kind identifies this backend in the registry and over the REST surface.
func (b *Backend) Kind() backend.Kind { return backend.KindKata }

// Create starts a sandbox pod and waits until it answers /healthz.
//
// It refuses two parts of the Spec instead of ignoring them. Mounts: a host
// path on an AKS node is meaningless, and silently dropping it would hand the
// agent an empty worktree with no error anywhere. WorkDir: the pod spec carries
// no working directory in Phase 2, and starting the agent in the image default
// instead would be a wrong-directory bug with no error anywhere either.
func (b *Backend) Create(ctx context.Context, spec backend.Spec) (backend.Sandbox, error) {
	if err := backend.ValidateSpec(spec); err != nil {
		return backend.Sandbox{}, err
	}
	if len(spec.Mounts) > 0 {
		return backend.Sandbox{}, fmt.Errorf("katabe: %d mount(s) requested but a host path on an AKS node is meaningless: %w", len(spec.Mounts), backend.ErrUnsupported)
	}
	if spec.WorkDir != "" {
		return backend.Sandbox{}, fmt.Errorf("katabe: spec.WorkDir %q is not expressible in the Phase 2 pod spec: %w", spec.WorkDir, backend.ErrUnsupported)
	}

	if _, err := b.mgr.Create(ctx, sandbox.Spec{
		ID:          spec.ID,
		Image:       spec.Image,
		Command:     spec.Command,
		CPU:         spec.Resources.KubeCPU(),
		Memory:      spec.Resources.KubeMemory(),
		ArgsJSON:    spec.ArgsJSON(),
		AttachToken: spec.AttachToken,
		Env:         spec.Env,
		SessionID:   spec.SessionID,
	}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return backend.Sandbox{}, fmt.Errorf("katabe: sandbox %q: %w", spec.ID, backend.ErrAlreadyExists)
		}
		return backend.Sandbox{}, fmt.Errorf("katabe: create pod for sandbox %q: %v: %w", spec.ID, err, backend.ErrUnavailable)
	}

	// The pod is left in place if either wait fails. A slow schedule is the
	// normal Kata cold start, and deleting the pod makes the retry slow too;
	// the reaper sweeps what is genuinely orphaned.
	deadline := time.Now().Add(b.readyTimeout)
	sb, err := b.waitScheduled(ctx, spec.ID, deadline)
	if err != nil {
		return backend.Sandbox{}, err
	}
	if err := backend.WaitReady(ctx, sb.Endpoint, time.Until(deadline)); err != nil {
		return backend.Sandbox{}, fmt.Errorf("katabe: sandbox %q pod %s: %w", spec.ID, sb.Ref, err)
	}
	return sb, nil
}

// waitScheduled polls the pod until it is Running with an address and a ready
// container, or the deadline passes.
//
// It exists because Manager.Create returning nil means "the API server accepted
// the object", not "the guest VM is up". Dialing before that produces a
// connection-refused that looks, in the cockpit, exactly like a crashed agent.
func (b *Backend) waitScheduled(ctx context.Context, id string, deadline time.Time) (backend.Sandbox, error) {
	for {
		sb, err := b.Get(ctx, id)
		if err != nil {
			return backend.Sandbox{}, err
		}
		switch {
		case sb.State == backend.StateRunning && sb.Endpoint != "":
			return sb, nil
		case sb.State == backend.StateStopped:
			// RestartPolicy is Never, so a stopped pod will not come back.
			return backend.Sandbox{}, fmt.Errorf("katabe: sandbox %q pod %s ended before it was ready: %w", id, sb.Ref, backend.ErrNotRunning)
		case !time.Now().Before(deadline):
			return backend.Sandbox{}, fmt.Errorf("katabe: sandbox %q pod %s never scheduled (state %s, endpoint %q): %w", id, sb.Ref, sb.State, sb.Endpoint, backend.ErrNotRunning)
		}
		select {
		case <-ctx.Done():
			return backend.Sandbox{}, ctx.Err()
		case <-time.After(podPollInterval):
		}
	}
}

// Get returns one sandbox, or ErrNotFound.
func (b *Backend) Get(ctx context.Context, id string) (backend.Sandbox, error) {
	pod, err := b.mgr.Get(ctx, id)
	if err != nil {
		if errors.Is(err, sandbox.ErrNotFound) {
			return backend.Sandbox{}, fmt.Errorf("katabe: sandbox %q: %w", id, backend.ErrNotFound)
		}
		// Anything else is the API server refusing to talk, which is a 503 and
		// not a 404: the sandbox may well be there.
		return backend.Sandbox{}, fmt.Errorf("katabe: get sandbox %q: %v: %w", id, err, backend.ErrUnavailable)
	}
	return b.sandboxFromPod(pod), nil
}

// List returns every sandbox this backend's manager created.
func (b *Backend) List(ctx context.Context) ([]backend.Sandbox, error) {
	pods, err := b.mgr.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("katabe: list sandboxes: %v: %w", err, backend.ErrUnavailable)
	}
	out := make([]backend.Sandbox, 0, len(pods))
	for i := range pods {
		out = append(out, b.sandboxFromPod(&pods[i]))
	}
	return out, nil
}

// sandboxFromPod is the only place a pod becomes a backend.Sandbox, so the
// endpoint and the state cannot be derived two different ways.
func (b *Backend) sandboxFromPod(pod *corev1.Pod) backend.Sandbox {
	labels := make(map[string]string, len(pod.Labels))
	for k, v := range pod.Labels {
		labels[k] = v
	}
	endpoint := ""
	if ip := pod.Status.PodIP; ip != "" {
		endpoint = net.JoinHostPort(ip, strconv.Itoa(b.port))
	}
	return backend.Sandbox{
		ID:        pod.Labels[backend.KubeLabelSandboxID],
		Kind:      backend.KindKata,
		Ref:       pod.Name,
		Endpoint:  endpoint,
		State:     podState(pod),
		CreatedAt: pod.CreationTimestamp.Time,
		Labels:    labels,
	}
}

// podState maps a pod phase onto the four backend states.
//
// StateGone is never produced: a pod that does not exist is ErrNotFound, which
// is what the API layer maps to 404 or 409. PodUnknown maps to StateStarting
// and not StateStopped, because a node that stopped reporting has not proven
// the sandbox ended, and StateStopped is what makes the reaper destroy it.
func podState(pod *corev1.Pod) backend.State {
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		return backend.StateStopped
	case corev1.PodRunning:
		// A Running pod with no address, or with a container that has not
		// reported Ready, is not yet serving /attach.
		if pod.Status.PodIP == "" || len(pod.Status.ContainerStatuses) == 0 {
			return backend.StateStarting
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				return backend.StateStarting
			}
		}
		return backend.StateRunning
	default:
		return backend.StateStarting
	}
}

// Attach opens the sandbox's /attach WebSocket.
//
// The token is a parameter rather than something read back from the cluster.
// In the cluster it lives in the sandbox's own Secret,
// sandbox.AttachSecretName(owner, id), which the pod's PANACEA_APP_ATTACH_TOKEN
// references by secretKeyRef (task 14): the pod spec carries no value, so the
// token is not recoverable by anything that can merely get or list pods.
func (b *Backend) Attach(ctx context.Context, id, token string) (*websocket.Conn, error) {
	sb, err := b.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sb.State != backend.StateRunning || sb.Endpoint == "" {
		return nil, fmt.Errorf("katabe: sandbox %q is %s: %w", id, sb.State, backend.ErrNotRunning)
	}
	return backend.Dial(ctx, sb.Endpoint, token)
}

// Destroy removes the pod and its token Secret (Manager.Delete). It is
// idempotent: the reaper and an explicit DELETE race each other, and a second
// delete must not become a 500 in the cockpit. In an owner view another
// owner's id is not found, so it is nil too, and that pod is left alone.
func (b *Backend) Destroy(ctx context.Context, id string) error {
	pod, err := b.mgr.Get(ctx, id)
	if err != nil {
		if errors.Is(err, sandbox.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("katabe: destroy sandbox %q: %v: %w", id, err, backend.ErrUnavailable)
	}
	if err := b.mgr.Delete(ctx, pod.Name); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("katabe: delete pod %s: %v: %w", pod.Name, err, backend.ErrUnavailable)
	}
	return nil
}

// Close releases nothing. The kubernetes.Interface belongs to whoever built the
// Manager and may be shared, so closing its transport here would break a second
// consumer of the same client.
func (b *Backend) Close() error { return nil }
