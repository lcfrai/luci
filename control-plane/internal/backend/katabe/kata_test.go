package katabe_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/katabe"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const namespace = "panacea-app"

func validSpec() backend.Spec {
	return backend.Spec{
		ID:          "s-3f2a91c0",
		SessionID:   "s-3f2a91c0",
		Image:       "panacea-app/sandbox:dev",
		Command:     "claude",
		Args:        []string{"--model", "opus", "plan the migration, then stop"},
		AttachToken: "tok-abcdefgh",
		Resources:   backend.DefaultResources,
	}
}

// fakeSandboxd is sandboxd's HTTP surface, reduced to the two things katabe
// does with it: poll /healthz and upgrade /attach with a bearer token.
type fakeSandboxd struct {
	srv       *httptest.Server
	port      int
	gotBearer chan string
}

func newFakeSandboxd(t *testing.T, token string) *fakeSandboxd {
	t.Helper()
	fs := &fakeSandboxd{gotBearer: make(chan string, 4)}

	mux := http.NewServeMux()
	mux.HandleFunc(backend.HealthPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(backend.AttachPath, func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		fs.gotBearer <- auth
		if auth != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		conn.SetReadLimit(backend.MaxMessageBytes)
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	})

	fs.srv = httptest.NewServer(mux)
	t.Cleanup(fs.srv.Close)

	addr, err := net.ResolveTCPAddr("tcp", strings.TrimPrefix(fs.srv.URL, "http://"))
	if err != nil {
		t.Fatalf("resolve %s: %v", fs.srv.URL, err)
	}
	fs.port = addr.Port
	return fs
}

func newBackend(t *testing.T, client *fake.Clientset, port int, readyTimeout time.Duration) *katabe.Backend {
	t.Helper()
	b, err := katabe.New(katabe.Config{
		Manager:      sandbox.NewManager(client, namespace),
		ReadyTimeout: readyTimeout,
		Port:         port,
		Instance:     kataInstance,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

// setStatus writes the three facts katabe reads to decide what a sandbox is
// doing. The fake client's object tracker is mutex-protected, so calling this
// from a second goroutine while Create polls is race-free.
func setStatus(t *testing.T, client *fake.Clientset, name string, phase corev1.PodPhase, ip string, ready bool) {
	t.Helper()
	ctx := context.Background()
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get %s: %v", name, err)
	}
	pod.Status.Phase = phase
	pod.Status.PodIP = ip
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "sandbox", Ready: ready}}
	if _, err := client.CoreV1().Pods(namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("UpdateStatus %s: %v", name, err)
	}
}

// runningSandbox drives the whole happy path: a fake sandboxd on loopback and
// a pod that becomes Running at that server's address shortly after Create is
// called. It returns once Create has returned.
func runningSandbox(t *testing.T, spec backend.Spec) (*katabe.Backend, *fake.Clientset, *fakeSandboxd, backend.Sandbox) {
	t.Helper()
	fs := newFakeSandboxd(t, spec.AttachToken)
	client := fake.NewSimpleClientset()
	b := newBackend(t, client, fs.port, 10*time.Second)

	// t.Fatalf is illegal off the test goroutine, so this reports with Errorf
	// and returns rather than calling setStatus.
	go func() {
		name := "sandbox-" + spec.ID
		for i := 0; i < 200; i++ {
			pod, err := client.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			pod.Status.Phase = corev1.PodRunning
			pod.Status.PodIP = "127.0.0.1"
			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "sandbox", Ready: true}}
			if _, err := client.CoreV1().Pods(namespace).UpdateStatus(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
				t.Errorf("UpdateStatus %s: %v", name, err)
			}
			return
		}
		t.Errorf("pod %s never appeared: Create did not create it", name)
	}()

	sb, err := b.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return b, client, fs, sb
}

// The reason this package exists in Phase 2 at all.
func TestBackendSatisfiesTheSeam(t *testing.T) {
	var _ backend.Backend = newBackend(t, fake.NewSimpleClientset(), 0, 0)
	b := newBackend(t, fake.NewSimpleClientset(), 0, 0)
	if b.Kind() != backend.KindKata {
		t.Errorf("Kind() = %q, want kata", b.Kind())
	}
	if err := b.Close(); err != nil {
		t.Errorf("Close() = %v, want nil: katabe owns no connection to release", err)
	}
}

func TestNewWithoutAManagerIsErrUnavailable(t *testing.T) {
	_, err := katabe.New(katabe.Config{})
	if !errors.Is(err, backend.ErrUnavailable) {
		t.Fatalf("New(no manager) = %v, want an error wrapping ErrUnavailable: no kubeconfig is a 503, not a panic at the first Create", err)
	}
}

// Config.ReadyTimeout is an OVERRIDE of backend.ReadyTimeout, so leaving it
// unset is a supported configuration and its default is behaviour, not
// decoration. Every other test here passes an explicit timeout, which would
// leave the default line unexecuted: a zero readyTimeout makes the deadline
// "now", and waitScheduled then returns ErrNotRunning on its first poll with
// no wait at all. This creates a pod that never schedules and ends the call
// from the context at 700ms. With the default applied (60s) the wait is still
// running at 700ms and the context is what stops it; without it the call
// returns ErrNotRunning almost immediately, and both assertions below fire.
func TestNewDefaultsReadyTimeoutToTheBackendConstant(t *testing.T) {
	client := fake.NewSimpleClientset()
	b, err := katabe.New(katabe.Config{Manager: sandbox.NewManager(client, namespace), Instance: kataInstance})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = b.Create(ctx, validSpec())
	elapsed := time.Since(start)

	if elapsed < 500*time.Millisecond {
		t.Fatalf("Create returned after %v with %v, want it still waiting at 500ms: an unset Config.ReadyTimeout must resolve to backend.ReadyTimeout (%v), and a zero one collapses the deadline to now", elapsed, err, backend.ReadyTimeout)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Create(ReadyTimeout unset, ctx expiring at 700ms) = %v, want context.DeadlineExceeded: the caller's context, not a collapsed readiness deadline, must be what ends the wait", err)
	}
}

// A host path on an AKS node is meaningless. Ignoring the field would hand the
// agent an empty worktree with no error anywhere, which is the failure this
// test exists to make impossible.
func TestCreateRejectsMountsAndCreatesNoPod(t *testing.T) {
	client := fake.NewSimpleClientset()
	b := newBackend(t, client, 0, time.Second)

	spec := validSpec()
	spec.Mounts = []backend.Mount{{HostPath: `C:\repos\panacea`, ContainerPath: "/work"}}

	if _, err := b.Create(context.Background(), spec); !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("Create(with mounts) = %v, want an error wrapping ErrUnsupported", err)
	}
	pods, err := client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(pods.Items) != 0 {
		t.Fatalf("Create created %d pod(s) before rejecting the spec; the check must run before the API call", len(pods.Items))
	}
}

// sandbox.Spec carries no working directory, so honouring WorkDir is not
// possible and ignoring it is a wrong-directory bug with no error anywhere.
func TestCreateRejectsAWorkDir(t *testing.T) {
	b := newBackend(t, fake.NewSimpleClientset(), 0, time.Second)
	spec := validSpec()
	spec.WorkDir = "/work/panacea"
	if _, err := b.Create(context.Background(), spec); !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("Create(with WorkDir) = %v, want an error wrapping ErrUnsupported", err)
	}
}

// ValidateSpec is task 4's, and both backends call it first so that "docker
// accepted it and kata did not" cannot happen.
func TestCreateRejectsASpecWithNoAttachToken(t *testing.T) {
	b := newBackend(t, fake.NewSimpleClientset(), 0, time.Second)
	spec := validSpec()
	spec.AttachToken = ""
	if _, err := b.Create(context.Background(), spec); !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("Create(no token) = %v, want an error wrapping ErrUnsupported", err)
	}
}

func TestCreateOfADuplicateIsErrAlreadyExists(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-s-3f2a91c0", Namespace: namespace},
	})
	b := newBackend(t, client, 0, 500*time.Millisecond)
	if _, err := b.Create(context.Background(), validSpec()); !errors.Is(err, backend.ErrAlreadyExists) {
		t.Fatalf("Create(duplicate) = %v, want an error wrapping ErrAlreadyExists", err)
	}
}

func TestCreateWaitsForHealthzAndReturnsTheEndpoint(t *testing.T) {
	spec := validSpec()
	_, _, fs, sb := runningSandbox(t, spec)

	if sb.Kind != backend.KindKata {
		t.Errorf("Kind = %q, want kata", sb.Kind)
	}
	if sb.ID != spec.ID {
		t.Errorf("ID = %q, want %q", sb.ID, spec.ID)
	}
	if sb.Ref != "sandbox-"+spec.ID {
		t.Errorf("Ref = %q, want the pod name sandbox-%s", sb.Ref, spec.ID)
	}
	if sb.State != backend.StateRunning {
		t.Errorf("State = %q, want running: Create returned before the sandbox was up", sb.State)
	}
	want := net.JoinHostPort("127.0.0.1", strconv.Itoa(fs.port))
	if sb.Endpoint != want {
		t.Errorf("Endpoint = %q, want %q", sb.Endpoint, want)
	}
	if sb.Labels[backend.KubeLabelSessionID] != spec.SessionID {
		t.Errorf("session label = %q, want %q", sb.Labels[backend.KubeLabelSessionID], spec.SessionID)
	}
	if sb.Labels[backend.KubeLabelInstance] != kataInstance || sb.Instance != kataInstance {
		t.Errorf("instance label = %q, Instance = %q, want %q: without it this backend's own List never sees the pod again",
			sb.Labels[backend.KubeLabelInstance], sb.Instance, kataInstance)
	}
}

// Kata sizes the guest VM from the requests, which is why all four quantities
// are asserted and not one of each pair: rendering 2000 millicpu as anything
// but the value the caller asked for, or filling in only the limits, is a
// guest OOM on a real cluster and is invisible until then.
//
// Compared with Equal rather than String(): resource.Quantity.String()
// canonicalizes to the smallest exact suffix, so resource.MustParse("2000m")
// prints as "2" and resource.MustParse("4096Mi") prints as "4Gi". That is
// apimachinery's own formatting choice and has nothing to do with whether the
// pod carries the right quantity, so pinning the string would make this test
// a hostage to an upstream rendering decision.
func TestCreateRendersKubernetesQuantitiesAndTheArgvContract(t *testing.T) {
	spec := validSpec()
	_, client, _, sb := runningSandbox(t, spec)

	pod, err := client.CoreV1().Pods(namespace).Get(context.Background(), sb.Ref, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get %s: %v", sb.Ref, err)
	}
	c := pod.Spec.Containers[0]
	wantCPU := resource.MustParse(spec.Resources.KubeCPU())
	wantMem := resource.MustParse(spec.Resources.KubeMemory())
	reqCPU, limCPU := c.Resources.Requests.Cpu(), c.Resources.Limits.Cpu()
	reqMem, limMem := c.Resources.Requests.Memory(), c.Resources.Limits.Memory()

	if !reqCPU.Equal(wantCPU) {
		t.Errorf("cpu request = %s, want %s (backend.Resources.KubeCPU); Kata sizes the guest VM from the requests", reqCPU.String(), wantCPU.String())
	}
	if !limCPU.Equal(wantCPU) {
		t.Errorf("cpu limit = %s, want %s (backend.Resources.KubeCPU)", limCPU.String(), wantCPU.String())
	}
	if !reqMem.Equal(wantMem) {
		t.Errorf("memory request = %s, want %s (backend.Resources.KubeMemory); an under-requested guest OOMs before the limit is ever reached", reqMem.String(), wantMem.String())
	}
	if !limMem.Equal(wantMem) {
		t.Errorf("memory limit = %s, want %s (backend.Resources.KubeMemory)", limMem.String(), wantMem.String())
	}

	// The actual invariant: a requests/limits mismatch would let the scheduler
	// admit the pod at one size while Kata builds the guest VM at another.
	if !reqCPU.Equal(*limCPU) {
		t.Errorf("cpu request %s != cpu limit %s", reqCPU.String(), limCPU.String())
	}
	if !reqMem.Equal(*limMem) {
		t.Errorf("memory request %s != memory limit %s", reqMem.String(), limMem.String())
	}

	env := map[string]string{}
	for _, e := range c.Env {
		env[e.Name] = e.Value
	}
	if got := env[backend.EnvArgsJSON]; got != `["--model","opus","plan the migration, then stop"]` {
		t.Errorf("PANACEA_APP_ARGS_JSON = %q; Spec.ArgsJSON() did not reach the pod", got)
	}
	if env[backend.EnvToken] != spec.AttachToken {
		t.Errorf("PANACEA_APP_ATTACH_TOKEN = %q, want the spec's token", env[backend.EnvToken])
	}
}

// The two backends must hand the agent identical environments, which is why
// task 4 centralises the ordering in Spec.EnvEntries() — its doc comment says
// so in as many words. dockerbe renders that slice directly; sandbox's
// containerEnv re-derives the same order because Manager also serves callers
// that never held a backend.Spec. Re-derivation is where drift starts, so this
// pins the two against each other: one assertion, and "katabe sorts, dockerbe
// does not" becomes a test failure rather than a Phase 8 discovery.
//
// ValidateSpec has already rejected any PANACEA_APP_* key in Env by the time
// containerEnv runs, so containerEnv's reserved-prefix drop — which
// EnvEntries() does not have, because it does not need it — cannot be the
// source of a difference on this path. That drop is pinned separately, for
// direct Manager callers, by manager_growth_test.go.
func TestThePodEnvironmentIsExactlySpecEnvEntries(t *testing.T) {
	spec := validSpec()
	spec.Env = map[string]string{"ZED": "3", "ALPHA": "1", "MID": "2"}

	_, client, _, sb := runningSandbox(t, spec)

	pod, err := client.CoreV1().Pods(namespace).Get(context.Background(), sb.Ref, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get %s: %v", sb.Ref, err)
	}
	got := pod.Spec.Containers[0].Env
	want := spec.EnvEntries()
	if len(got) != len(want) {
		t.Fatalf("pod env has %d entries, want %d (%#v vs %#v): sandbox.containerEnv and backend.Spec.EnvEntries() disagree", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Value != want[i].Value {
			t.Fatalf("pod env[%d] = %s=%q, want %s=%q: sandbox.containerEnv has drifted from backend.Spec.EnvEntries(), so katabe and dockerbe no longer produce the same environment", i, got[i].Name, got[i].Value, want[i].Name, want[i].Value)
		}
	}
}

// A slow schedule is the normal Kata cold start, so the pod is left alone for
// the reaper rather than deleted: deleting it would make the retry slow too.
func TestCreateTimesOutWithErrNotRunningAndLeavesThePod(t *testing.T) {
	client := fake.NewSimpleClientset()
	b := newBackend(t, client, 0, 400*time.Millisecond)

	_, err := b.Create(context.Background(), validSpec())
	if !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Create(pod never schedules) = %v, want an error wrapping ErrNotRunning", err)
	}
	if _, err := client.CoreV1().Pods(namespace).Get(context.Background(), "sandbox-s-3f2a91c0", metav1.GetOptions{}); err != nil {
		t.Fatalf("pod is gone after a readiness timeout (%v); Create must not delete it", err)
	}
}

func TestGetMapsEveryPodPhaseToOneState(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, namespace)
	if _, err := m.Create(context.Background(), sandbox.Spec{
		ID: "s-state", Image: "x:y", Command: "claude", CPU: "500m", Memory: "1Gi",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// A real API server stamps CreationTimestamp and the fake clientset does
	// not, so the test writes one. Without it, dropping CreatedAt from
	// sandboxFromPod would leave every assertion here green and the cockpit
	// would report every sandbox as having started at the epoch. setStatus
	// round-trips the whole object, so the stamp survives each case.
	created := time.Date(2026, 9, 18, 9, 30, 0, 0, time.UTC)
	stamped, err := client.CoreV1().Pods(namespace).Get(context.Background(), "sandbox-s-state", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get sandbox-s-state: %v", err)
	}
	stamped.CreationTimestamp = metav1.NewTime(created)
	if _, err := client.CoreV1().Pods(namespace).Update(context.Background(), stamped, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Update sandbox-s-state: %v", err)
	}

	// Port 0 exercises the backend.ContainerPort default.
	b := newBackend(t, client, 0, time.Second)

	cases := []struct {
		name         string
		phase        corev1.PodPhase
		ip           string
		ready        bool
		wantState    backend.State
		wantEndpoint string
	}{
		{"pending, unscheduled", corev1.PodPending, "", false, backend.StateStarting, ""},
		{"running, no address yet", corev1.PodRunning, "", true, backend.StateStarting, ""},
		{"running, container not ready", corev1.PodRunning, "10.1.2.3", false, backend.StateStarting, "10.1.2.3:8080"},
		{"running and ready", corev1.PodRunning, "10.1.2.3", true, backend.StateRunning, "10.1.2.3:8080"},
		{"succeeded", corev1.PodSucceeded, "10.1.2.3", false, backend.StateStopped, "10.1.2.3:8080"},
		{"failed", corev1.PodFailed, "10.1.2.3", false, backend.StateStopped, "10.1.2.3:8080"},
		// A node that stopped reporting has not proven the sandbox ended.
		{"unknown", corev1.PodUnknown, "10.1.2.3", true, backend.StateStarting, "10.1.2.3:8080"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setStatus(t, client, "sandbox-s-state", tc.phase, tc.ip, tc.ready)
			sb, err := b.Get(context.Background(), "s-state")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if sb.State != tc.wantState {
				t.Errorf("State = %q, want %q", sb.State, tc.wantState)
			}
			if sb.Endpoint != tc.wantEndpoint {
				t.Errorf("Endpoint = %q, want %q", sb.Endpoint, tc.wantEndpoint)
			}
			if !sb.CreatedAt.Equal(created) {
				t.Errorf("CreatedAt = %v, want %v: the pod's creation timestamp is not reaching Sandbox.CreatedAt", sb.CreatedAt, created)
			}
		})
	}

	// setStatus always writes exactly one container status, so no case above
	// reaches podState's empty-ContainerStatuses branch. A Running pod with an
	// address whose kubelet has reported no container at all is not serving
	// /attach yet; without this case the branch can be deleted and Create's
	// readiness wait will accept such a pod as ready.
	t.Run("running, no container status reported", func(t *testing.T) {
		pod, err := client.CoreV1().Pods(namespace).Get(context.Background(), "sandbox-s-state", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Get sandbox-s-state: %v", err)
		}
		pod.Status.Phase = corev1.PodRunning
		pod.Status.PodIP = "10.1.2.3"
		pod.Status.ContainerStatuses = nil
		if _, err := client.CoreV1().Pods(namespace).UpdateStatus(context.Background(), pod, metav1.UpdateOptions{}); err != nil {
			t.Fatalf("UpdateStatus sandbox-s-state: %v", err)
		}

		sb, err := b.Get(context.Background(), "s-state")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if sb.State != backend.StateStarting {
			t.Errorf("State = %q, want starting", sb.State)
		}
		if sb.Endpoint != "10.1.2.3:8080" {
			t.Errorf("Endpoint = %q, want 10.1.2.3:8080", sb.Endpoint)
		}
	})
}

func TestGetOnAnUnknownSandboxIsErrNotFound(t *testing.T) {
	b := newBackend(t, fake.NewSimpleClientset(), 0, time.Second)
	if _, err := b.Get(context.Background(), "s-nothing"); !errors.Is(err, backend.ErrNotFound) {
		t.Fatalf("Get(unknown) = %v, want an error wrapping backend.ErrNotFound: sandbox.ErrNotFound must be translated, not leaked", err)
	}
}

func TestListReturnsEverySandboxAsABackendSandbox(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, namespace)
	for _, id := range []string{"s-one", "s-two"} {
		if _, err := m.Create(context.Background(), sandbox.Spec{
			ID: id, Image: "x:y", Command: "claude", CPU: "500m", Memory: "1Gi", SessionID: id, Instance: kataInstance,
		}); err != nil {
			t.Fatalf("Create(%s): %v", id, err)
		}
	}
	b := newBackend(t, client, 0, time.Second)

	list, err := b.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List returned %d sandboxes, want 2", len(list))
	}
	seen := map[string]bool{}
	for _, sb := range list {
		seen[sb.ID] = true
		if sb.Kind != backend.KindKata {
			t.Errorf("%s: Kind = %q, want kata", sb.ID, sb.Kind)
		}
		if sb.State != backend.StateStarting {
			t.Errorf("%s: State = %q, want starting for a pod with no status yet", sb.ID, sb.State)
		}
	}
	if !seen["s-one"] || !seen["s-two"] {
		t.Errorf("List returned ids %v, want s-one and s-two: the sandbox-id label is not being read back", seen)
	}
}

// kataInstance is the instance id every katabe under test owns.
const kataInstance = "11111111-1111-4111-8111-111111111111"

// dockerbe.List's ownership rule, on the other backend: two control planes
// sharing a namespace see each other's pods through the manager, and another
// instance's never come back. A pod with no instance label does come back,
// with Instance "" (reaper-scope fix round, I1): whether it is ours is the
// reaper's call, by whether this registry holds its sandbox id.
func TestListReturnsThisInstancesAndUnlabelledPodsButNotAnothers(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, namespace)
	for id, instance := range map[string]string{
		"s-ours":   kataInstance,
		"s-theirs": "22222222-2222-4222-8222-222222222222",
		"s-legacy": "",
	} {
		if _, err := m.Create(context.Background(), sandbox.Spec{
			ID: id, Image: "x:y", Command: "claude", CPU: "500m", Memory: "1Gi", Instance: instance,
		}); err != nil {
			t.Fatalf("Create(%s): %v", id, err)
		}
	}
	b := newBackend(t, client, 0, time.Second)

	list, err := b.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := map[string]string{}
	for _, sb := range list {
		got[sb.ID] = sb.Instance
	}
	want := map[string]string{"s-ours": kataInstance, "s-legacy": ""}
	if len(got) != len(want) || got["s-ours"] != kataInstance {
		t.Fatalf("List = %+v, want s-ours owned by %s and the unlabelled s-legacy, never s-theirs", list, kataInstance)
	}
	if inst, ok := got["s-legacy"]; !ok || inst != "" {
		t.Fatalf("List = %+v, want the unlabelled s-legacy with Instance \"\"", list)
	}
}

func TestNewRefusesAMissingInstance(t *testing.T) {
	_, err := katabe.New(katabe.Config{Manager: sandbox.NewManager(fake.NewSimpleClientset(), namespace)})
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("New(no instance) = %v, want an error wrapping ErrUnsupported: a backend with no instance owns every unlabelled pod", err)
	}
}

// The token is passed in rather than read back from the pod, so the secret
// lives in exactly one durable place and nothing that can merely list pods can
// recover it.
func TestAttachSendsTheBearerTokenToThePod(t *testing.T) {
	spec := validSpec()
	b, _, fs, _ := runningSandbox(t, spec)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := b.Attach(ctx, spec.ID, spec.AttachToken)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	select {
	case got := <-fs.gotBearer:
		if got != "Bearer "+spec.AttachToken {
			t.Fatalf("Authorization header = %q, want %q", got, "Bearer "+spec.AttachToken)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the sandbox never saw an /attach request")
	}
}

func TestAttachWithTheWrongTokenIsErrUnauthorized(t *testing.T) {
	spec := validSpec()
	b, _, _, _ := runningSandbox(t, spec)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := b.Attach(ctx, spec.ID, "wrong"); !errors.Is(err, backend.ErrUnauthorized) {
		t.Fatalf("Attach(wrong token) = %v, want an error wrapping ErrUnauthorized", err)
	}
}

func TestAttachToAPodThatIsNotRunningIsErrNotRunning(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, namespace)
	if _, err := m.Create(context.Background(), sandbox.Spec{
		ID: "s-pending", Image: "x:y", Command: "claude", CPU: "500m", Memory: "1Gi",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	b := newBackend(t, client, 0, time.Second)

	if _, err := b.Attach(context.Background(), "s-pending", "tok"); !errors.Is(err, backend.ErrNotRunning) {
		t.Fatalf("Attach(pending pod) = %v, want an error wrapping ErrNotRunning, not a dial to an empty address", err)
	}
}

func TestAttachToAnUnknownSandboxIsErrNotFound(t *testing.T) {
	b := newBackend(t, fake.NewSimpleClientset(), 0, time.Second)
	if _, err := b.Attach(context.Background(), "s-nothing", "tok"); !errors.Is(err, backend.ErrNotFound) {
		t.Fatalf("Attach(unknown) = %v, want an error wrapping ErrNotFound", err)
	}
}

func TestDestroyDeletesThePod(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, namespace)
	if _, err := m.Create(context.Background(), sandbox.Spec{
		ID: "s-doomed", Image: "x:y", Command: "claude", CPU: "500m", Memory: "1Gi",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	b := newBackend(t, client, 0, time.Second)

	if err := b.Destroy(context.Background(), "s-doomed"); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if _, err := b.Get(context.Background(), "s-doomed"); !errors.Is(err, backend.ErrNotFound) {
		t.Fatalf("Get after Destroy = %v, want ErrNotFound", err)
	}
}

// The reaper (task 13) and an explicit DELETE race each other. A second delete
// must be a no-op, or every race becomes a 500 in the cockpit.
func TestDestroyOfAnUnknownSandboxIsNil(t *testing.T) {
	b := newBackend(t, fake.NewSimpleClientset(), 0, time.Second)
	if err := b.Destroy(context.Background(), "s-never-existed"); err != nil {
		t.Fatalf("Destroy(unknown) = %v, want nil: destroy is idempotent", err)
	}
}
