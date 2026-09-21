### Task 6: The Kata backend, dormant but tested

Phase 2 spends nothing on Azure and touches no cluster. This task still ships the Kata implementation of `backend.Backend`, complete and unit-tested, because that is the only way to know the seam is a seam. An interface with one implementation is a type declaration; an interface with two is a contract. If `katabe` is written in Phase 8 instead, Phase 8 discovers that `Spec` carries fields Kubernetes cannot express, and the "Azure is a swap of one implementation" claim in `docs/PROGRAM.md` turns out to have been untested for six months.

`control-plane/internal/sandbox/manager.go` already creates a Kata pod correctly and is tested against `k8s.io/client-go/kubernetes/fake`. This task **grows** it and rewrites nothing: `Spec` gains `ArgsJSON`, `AttachToken`, `Env` and `SessionID`, and the `Manager` gains `Get` and `List`. The existing runtime class, tolerations, node selector, read-only root with its `home` and `tmp` emptyDirs, and the matched requests and limits are untouched, so `manager_test.go` keeps compiling and keeps passing unchanged.

Four decisions the contract left open, made here so no reader has to re-derive them. **(1) `internal/sandbox` now imports `internal/backend`** for the `KubeLabel*` and `PANACEA_APP_*` constants. The dependency is acyclic — `backend` imports only the standard library and `coder/websocket` — and the alternative is two spellings of the same label key in one repository, which is the exact defect those constants exist to prevent. **(2) `Create` rejects a non-empty `Spec.WorkDir` with `ErrUnsupported`**, on the same reasoning the contract gives for `Mounts`: contract §2.5 fixes the four fields `sandbox.Spec` grows and `WorkDir` is not among them, so the pod cannot honour it, and an agent silently starting in the image's default directory is a wrong-directory bug with no error anywhere. Phase 8 can add `container.WorkingDir` deliberately. **(3) `Create` waits for readiness and does not delete the pod when that wait times out.** A slow schedule is the normal Kata cold start (`docs/PROGRAM.md` open risk 1, the reason warm pools are mandatory), and deleting the pod guarantees the retry is slow too; the reaper in task 13 sweeps what is genuinely orphaned. **(4) `Config.Port` exists and defaults to `backend.ContainerPort`.** The contract writes the endpoint as `net.JoinHostPort(podIP, "8080")` and that stays the behaviour, but with the port hard-coded no test can exercise the readiness wait at all: a fake clientset will report any `PodIP` you like, and nothing can make an `httptest` server listen on port 8080 of `127.0.0.1` without a privileged, flaky, machine-wide port grab.

Two constraints from `docs/PROGRAM.md` are pinned by tests rather than by comments. Explicit CPU and memory are mandatory because the Kata guest VM is sized from them and under-requesting causes a guest OOM — `Resources.KubeCPU()` and `Resources.KubeMemory()` render them and a test asserts all four quantities, requests and limits alike, at the exact values. The requests are the pair Kata sizes the VM from, so they are the pair a limit-only render would silently omit. `Mounts` are refused rather than ignored, because a host path on an AKS node is meaningless.

**Files:**
- `control-plane/internal/sandbox/manager.go` — pod creation unchanged; `Spec` grows args/env/token/session id, `Get`/`List`/`ErrNotFound` added (modified)
- `control-plane/internal/sandbox/manager_growth_test.go` — tests for the grown `Spec` and the two new methods, in a second file so `manager_test.go` is not touched (new)
- `control-plane/internal/backend/katabe/kata.go` — `backend.Backend` over `sandbox.Manager`; dormant until Phase 8, compiled and tested now (new)
- `control-plane/internal/backend/katabe/kata_test.go` — the whole adapter against `fake.NewSimpleClientset()` and an `httptest` sandboxd; no cluster, no network beyond loopback (new)

**Interfaces:**
- Produces:
  - `sandbox.Spec.ArgsJSON`, `.AttachToken`, `.Env`, `.SessionID` — the four new fields
  - `(*sandbox.Manager).Get(ctx, id) (*corev1.Pod, error)`, `(*sandbox.Manager).List(ctx) ([]corev1.Pod, error)`, `sandbox.ErrNotFound`
  - `katabe.Config{Manager, ReadyTimeout, Port}`, `katabe.Backend`, `katabe.New(Config) (*Backend, error)`
  - `var _ backend.Backend = (*katabe.Backend)(nil)` — the compile-time proof the seam holds
- Consumes:
  - Task 4: `backend.Backend`, `backend.Spec` with `ArgsJSON()`/`EnvEntries()`, `backend.ValidateSpec`, `backend.Sandbox`, `backend.Resources.KubeCPU`/`KubeMemory`, `backend.Kind`/`KindKata`, `backend.State*`, `backend.ErrNotFound`/`ErrNotRunning`/`ErrAlreadyExists`/`ErrUnavailable`/`ErrUnsupported`, `backend.EnvCommand`/`EnvArgsJSON`/`EnvToken`, `backend.KubeLabelSandboxID`/`KubeLabelSessionID`/`KubeLabelComponent`, `backend.ContainerPort`, `backend.ReadyTimeout`, `backend.Dial`, `backend.WaitReady`
  - Phase 1: `sandbox.NewManager`, `(*sandbox.Manager).Create`, `(*sandbox.Manager).Delete`
- Consumed by: task 13 only, and only as a `PANACEA_APP_BACKEND=kata` branch that Phase 2 never takes. No task in this phase selects it by default.

- [ ] **Step 1: Write the failing tests for the grown manager**

These go in a second file rather than in `manager_test.go`. The existing file is Phase 1's proof that a Kata pod is shaped correctly and it must stay byte-identical, so that a reviewer can see at a glance that pod creation was not rewritten. Both files are `package sandbox_test`, so the helper names below must not collide with anything in `manager_test.go` — they do not, since that file declares no helpers.

What these pin: the three `PANACEA_APP_*` variables arrive in a fixed order with the right values; an empty `ArgsJSON` becomes `"[]"` and never `""`, because sandboxd reads an empty value as "fall back to the legacy comma-separated form" and that form splits an opening prompt on its commas; extra environment is sorted so two identical `Spec`s produce byte-identical pods; a caller-supplied `PANACEA_APP_*` key cannot displace the attach token; `Get` finds by label and returns `ErrNotFound` otherwise; and neither `Get` nor `List` can see a pod this manager did not create.

Create `control-plane/internal/sandbox/manager_growth_test.go`:

```go
package sandbox_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const growthNamespace = "panacea-app"

func growthSpec() sandbox.Spec {
	return sandbox.Spec{
		ID:          "s-3f2a91c0",
		Image:       "panacea-app/sandbox:dev",
		Command:     "claude",
		CPU:         "2000m",
		Memory:      "4096Mi",
		ArgsJSON:    `["--model","opus","plan the migration, then stop"]`,
		AttachToken: "tok-abcdefgh",
		SessionID:   "s-3f2a91c0",
	}
}

func createPod(t *testing.T, client *fake.Clientset, spec sandbox.Spec) *corev1.Pod {
	t.Helper()
	m := sandbox.NewManager(client, growthNamespace)
	name, err := m.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	pod, err := client.CoreV1().Pods(growthNamespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get %s: %v", name, err)
	}
	return pod
}

func podNames(pods []corev1.Pod) []string {
	out := make([]string, 0, len(pods))
	for _, p := range pods {
		out = append(out, p.Name)
	}
	return out
}

// The downstream contract with sandboxd, fixed by Group A task 3. Order is
// asserted, not membership, because both backends must produce the same
// environment and a set comparison would hide a divergence.
func TestCreateInjectsTheSandboxdContractEnvironment(t *testing.T) {
	pod := createPod(t, fake.NewSimpleClientset(), growthSpec())
	env := pod.Spec.Containers[0].Env
	if len(env) < 3 {
		t.Fatalf("got %d env vars, want at least the three PANACEA_APP_* variables", len(env))
	}
	want := []corev1.EnvVar{
		{Name: "PANACEA_APP_COMMAND", Value: "claude"},
		{Name: "PANACEA_APP_ARGS_JSON", Value: `["--model","opus","plan the migration, then stop"]`},
		{Name: "PANACEA_APP_ATTACH_TOKEN", Value: "tok-abcdefgh"},
	}
	for i, w := range want {
		if env[i].Name != w.Name || env[i].Value != w.Value {
			t.Errorf("env[%d] = %s=%q, want %s=%q", i, env[i].Name, env[i].Value, w.Name, w.Value)
		}
	}
}

// An empty PANACEA_APP_ARGS_JSON is not "no arguments" to sandboxd, it is
// "use the legacy comma-separated form", which splits the prompt above into
// two arguments at its comma.
func TestCreateDefaultsAnEmptyArgsJSONToAnEmptyArray(t *testing.T) {
	spec := growthSpec()
	spec.ArgsJSON = ""
	pod := createPod(t, fake.NewSimpleClientset(), spec)
	for _, e := range pod.Spec.Containers[0].Env {
		if e.Name == "PANACEA_APP_ARGS_JSON" {
			if e.Value != "[]" {
				t.Fatalf("PANACEA_APP_ARGS_JSON = %q, want %q", e.Value, "[]")
			}
			return
		}
	}
	t.Fatal("PANACEA_APP_ARGS_JSON is absent from the container environment")
}
```

Continuing `control-plane/internal/sandbox/manager_growth_test.go`:

```go
func TestCreateAppendsExtraEnvironmentSortedAndDropsReservedKeys(t *testing.T) {
	spec := growthSpec()
	spec.Env = map[string]string{
		"ZULU":                     "z",
		"ALPHA":                    "a",
		"PANACEA_APP_ATTACH_TOKEN": "stolen",
	}
	pod := createPod(t, fake.NewSimpleClientset(), spec)
	env := pod.Spec.Containers[0].Env

	if len(env) != 5 {
		t.Fatalf("got %d env vars %v, want 5: the three contract variables plus ALPHA and ZULU", len(env), env)
	}
	if env[3].Name != "ALPHA" || env[4].Name != "ZULU" {
		t.Errorf("extra env is %s then %s, want ALPHA then ZULU: unsorted map iteration makes two identical Specs produce different pods", env[3].Name, env[4].Name)
	}
	for _, e := range env {
		if e.Name == "PANACEA_APP_ATTACH_TOKEN" && e.Value != "tok-abcdefgh" {
			t.Fatalf("PANACEA_APP_ATTACH_TOKEN = %q: a caller-supplied reserved key replaced the sandbox secret", e.Value)
		}
	}
}

func TestCreateLabelsThePodWithTheSessionAndComponent(t *testing.T) {
	pod := createPod(t, fake.NewSimpleClientset(), growthSpec())
	if got := pod.Labels["panacea-app/session-id"]; got != "s-3f2a91c0" {
		t.Errorf("session-id label = %q, want s-3f2a91c0", got)
	}
	if got := pod.Labels["panacea-app/component"]; got != "sandbox" {
		t.Errorf("component label = %q, want sandbox", got)
	}
	if got := pod.Labels["panacea-app/sandbox-id"]; got != "s-3f2a91c0" {
		t.Errorf("sandbox-id label = %q, want s-3f2a91c0", got)
	}
}

func TestCreateOmitsTheSessionLabelWhenThereIsNoSession(t *testing.T) {
	spec := growthSpec()
	spec.SessionID = ""
	pod := createPod(t, fake.NewSimpleClientset(), spec)
	if _, ok := pod.Labels["panacea-app/session-id"]; ok {
		t.Error("session-id label is present with an empty value; an empty label is noise in every kubectl output and matches nothing useful")
	}
}

// Kata sizes the guest VM from the requests. manager_test.go already proves
// both are present; this proves they are the values the caller asked for and
// that they match, which is what stops a guest OOM.
func TestCreateSetsRequestsEqualToLimitsAtTheGivenQuantities(t *testing.T) {
	c := createPod(t, fake.NewSimpleClientset(), growthSpec()).Spec.Containers[0]
	if got := c.Resources.Requests.Cpu().String(); got != "2000m" {
		t.Errorf("cpu request = %s, want 2000m", got)
	}
	if got := c.Resources.Limits.Cpu().String(); got != "2000m" {
		t.Errorf("cpu limit = %s, want 2000m", got)
	}
	if got := c.Resources.Requests.Memory().String(); got != "4096Mi" {
		t.Errorf("memory request = %s, want 4096Mi", got)
	}
	if got := c.Resources.Limits.Memory().String(); got != "4096Mi" {
		t.Errorf("memory limit = %s, want 4096Mi", got)
	}
}

func TestGetFindsThePodBySandboxID(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, growthNamespace)
	if _, err := m.Create(context.Background(), growthSpec()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	pod, err := m.Get(context.Background(), "s-3f2a91c0")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if pod.Name != "sandbox-s-3f2a91c0" {
		t.Errorf("pod name = %q, want sandbox-s-3f2a91c0", pod.Name)
	}
}

func TestGetReturnsErrNotFoundForAnUnknownID(t *testing.T) {
	m := sandbox.NewManager(fake.NewSimpleClientset(), growthNamespace)
	if _, err := m.Get(context.Background(), "never-created"); !errors.Is(err, sandbox.ErrNotFound) {
		t.Fatalf("Get(unknown) = %v, want an error wrapping sandbox.ErrNotFound", err)
	}
}

// An empty id must not degenerate into "any sandbox". Without the guard the
// selector collapses to the component label and Get returns somebody else's
// session, which the API layer would then happily attach the caller to.
func TestGetOnAnEmptyIDIsErrNotFoundAndNotTheFirstPod(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, growthNamespace)
	if _, err := m.Create(context.Background(), growthSpec()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := m.Get(context.Background(), ""); !errors.Is(err, sandbox.ErrNotFound) {
		t.Fatalf(`Get("") = %v, want ErrNotFound`, err)
	}
}
```

Continuing `control-plane/internal/sandbox/manager_growth_test.go`:

```go
func TestListReturnsEveryManagedPod(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, growthNamespace)
	for _, id := range []string{"s-one", "s-two", "s-three"} {
		spec := growthSpec()
		spec.ID = id
		spec.SessionID = id
		if _, err := m.Create(context.Background(), spec); err != nil {
			t.Fatalf("Create(%s): %v", id, err)
		}
	}
	pods, err := m.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(pods) != 3 {
		t.Fatalf("List returned %d pods %v, want 3", len(pods), podNames(pods))
	}
}

// The namespace is not ours alone. A pod without the component label belongs
// to something else, and reaping or attaching to it would be a cross-tenant
// action in Phase 8.
func TestGetAndListIgnorePodsThisManagerDidNotCreate(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "someone-elses-pod",
			Namespace: growthNamespace,
			Labels:    map[string]string{"panacea-app/sandbox-id": "s-3f2a91c0"},
		},
	})
	m := sandbox.NewManager(client, growthNamespace)
	if _, err := m.Create(context.Background(), growthSpec()); err != nil {
		t.Fatalf("Create: %v", err)
	}

	pods, err := m.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(pods) != 1 || pods[0].Name != "sandbox-s-3f2a91c0" {
		t.Fatalf("List returned %v, want only sandbox-s-3f2a91c0: the component label is missing from the selector", podNames(pods))
	}

	pod, err := m.Get(context.Background(), "s-3f2a91c0")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if pod.Name != "sandbox-s-3f2a91c0" {
		t.Fatalf("Get returned %q, want sandbox-s-3f2a91c0: the sandbox-id label alone matched a foreign pod", pod.Name)
	}
}
```

- [ ] **Step 2: Run the manager tests and watch them fail**

Run: `go test ./control-plane/internal/sandbox/...`

Expected: FAIL to build, with nine distinct complaints from the new file and none from `manager_test.go`. Only the three struct-literal positions are fixed; the rest depend on where the three fenced blocks above land once they are concatenated into one file. `ArgsJSON` is reported twice, once as a literal key and once as an assignment target; `SessionID` three times, once as a literal key and twice as an assignment target:

```
control-plane/internal/sandbox/manager_growth_test.go:23:3: unknown field ArgsJSON in struct literal of type sandbox.Spec
control-plane/internal/sandbox/manager_growth_test.go:24:3: unknown field AttachToken in struct literal of type sandbox.Spec
control-plane/internal/sandbox/manager_growth_test.go:25:3: unknown field SessionID in struct literal of type sandbox.Spec
control-plane/internal/sandbox/manager_growth_test.go:...: spec.ArgsJSON undefined (type sandbox.Spec has no field or method ArgsJSON)
control-plane/internal/sandbox/manager_growth_test.go:...: spec.Env undefined (type sandbox.Spec has no field or method Env)
control-plane/internal/sandbox/manager_growth_test.go:...: spec.SessionID undefined (type sandbox.Spec has no field or method SessionID)
control-plane/internal/sandbox/manager_growth_test.go:...: m.Get undefined (type *sandbox.Manager has no field or method Get)
control-plane/internal/sandbox/manager_growth_test.go:...: undefined: sandbox.ErrNotFound
control-plane/internal/sandbox/manager_growth_test.go:...: m.List undefined (type *sandbox.Manager has no field or method List)
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox [build failed]
```

Count kinds, not lines: `m.Get`, `m.List` and `sandbox.ErrNotFound` are reported once per call site — four, two and two respectively — which with the seven field complaints makes fifteen error lines in all, and `gc` stops after ten and prints `too many errors`, so the tail of the list above is cut off and `m.List` is usually the complaint you never see. That is the right failure as long as every line names one of the four fields or two methods this task adds, and none of them names the pod shape. If you also see errors pointing at `manager_test.go`, stop — you have edited the Phase 1 file, which this task must leave alone.

- [ ] **Step 3: Grow the manager**

Replace `control-plane/internal/sandbox/manager.go` with the file below. Everything from `RuntimeClassName` to the security context is the Phase 1 text; the new material is the four `Spec` fields, the three helpers, `Get`, `List` and `ErrNotFound`. The two literals that had a constant in `internal/backend` — the label keys and the container port — now use it, which is a spelling change and not a behaviour change.

```go
// Package sandbox creates the per-session Kata microVM pods that agent
// CLIs run inside.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// runtimeClass is the RuntimeClass installed by AKS pod sandboxing.
const runtimeClass = "kata-mshv-vm-isolation"

// componentSandbox is the value of the panacea-app/component label on every
// pod this manager creates. Get and List select on it, so a pod that merely
// carries a sandbox-id label is invisible to both.
const componentSandbox = "sandbox"

// envReservedPrefix is the namespace of the three variables the sandboxd
// contract owns. internal/backend.ValidateSpec is the authoritative rejection;
// the check in containerEnv is the last line for a direct Manager caller.
const envReservedPrefix = "PANACEA_APP_"

// emptyArgsJSON is what "no arguments" looks like on the wire. It is not "".
const emptyArgsJSON = "[]"

// ErrNotFound is returned by Get when no pod carries the sandbox-id label.
var ErrNotFound = errors.New("sandbox: no pod for that id")

// Spec describes one sandbox.
type Spec struct {
	ID      string
	Image   string
	Command string
	CPU     string
	Memory  string

	// Added in Phase 2 so a Kata pod carries the same contract as a Docker
	// container. ArgsJSON is already-encoded JSON, not []string, so the manager
	// never needs encoding/json and the two backends cannot encode differently.
	ArgsJSON    string
	AttachToken string
	Env         map[string]string
	SessionID   string
}

// Manager creates and removes sandbox pods in a single namespace.
type Manager struct {
	client    kubernetes.Interface
	namespace string
}

// NewManager returns a Manager bound to namespace.
func NewManager(client kubernetes.Interface, namespace string) *Manager {
	return &Manager{client: client, namespace: namespace}
}

// podNameFor is the one place a sandbox id becomes a Kubernetes object name.
// It is not called podName, because Delete already has a parameter by that
// name and the shadowing reads as a bug even though it compiles.
func podNameFor(id string) string { return "sandbox-" + id }

// podLabels are what Get, List and every kubectl invocation select on.
//
// The session label is omitted rather than set empty when there is no session:
// an empty-valued label is noise in every listing and matches nothing useful.
func podLabels(spec Spec) map[string]string {
	labels := map[string]string{
		backend.KubeLabelSandboxID: spec.ID,
		backend.KubeLabelComponent: componentSandbox,
	}
	if spec.SessionID != "" {
		labels[backend.KubeLabelSessionID] = spec.SessionID
	}
	return labels
}

// containerEnv renders the container's environment: the three PANACEA_APP_*
// variables the sandboxd contract fixes, then every Spec.Env key in sorted
// order, so two identical Specs produce byte-identical pods and a test can
// assert on a slice instead of a set.
//
// The order is deliberately the order backend.Spec.EnvEntries() produces, and
// the katabe test TestThePodEnvironmentIsExactlySpecEnvEntries pins the two
// together, because dockerbe renders EnvEntries() directly and the agent must
// see the same environment on both backends. This function re-derives the
// order rather than calling EnvEntries() because Manager takes sandbox.Spec
// and is called by code that never held a backend.Spec; the test is what stops
// the re-derivation from drifting.
func containerEnv(spec Spec) []corev1.EnvVar {
	argsJSON := spec.ArgsJSON
	if strings.TrimSpace(argsJSON) == "" {
		// sandboxd reads an empty PANACEA_APP_ARGS_JSON as "fall back to the
		// legacy comma-separated form", and that form splits an opening prompt
		// on its commas.
		argsJSON = emptyArgsJSON
	}

	env := []corev1.EnvVar{
		{Name: backend.EnvCommand, Value: spec.Command},
		{Name: backend.EnvArgsJSON, Value: argsJSON},
		{Name: backend.EnvToken, Value: spec.AttachToken},
	}

	keys := make([]string, 0, len(spec.Env))
	for k := range spec.Env {
		// A caller that could set PANACEA_APP_ATTACH_TOKEN here could replace
		// the sandbox's secret with one it chose.
		if strings.HasPrefix(k, envReservedPrefix) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, corev1.EnvVar{Name: k, Value: spec.Env[k]})
	}
	return env
}

// selector matches every pod this manager created, narrowed to one sandbox id
// when id is non-empty.
func selector(id string) string {
	s := backend.KubeLabelComponent + "=" + componentSandbox
	if id != "" {
		s = backend.KubeLabelSandboxID + "=" + id + "," + s
	}
	return s
}
```

Continuing `control-plane/internal/sandbox/manager.go`:

```go
// Create starts a sandbox pod and returns its name.
func (m *Manager) Create(ctx context.Context, spec Spec) (string, error) {
	name := podNameFor(spec.ID)
	rc := runtimeClass
	nonRoot := true
	readOnlyRoot := true
	noEscalation := false
	uid := int64(1001)

	quantities := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(spec.CPU),
		corev1.ResourceMemory: resource.MustParse(spec.Memory),
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: m.namespace,
			Labels:    podLabels(spec),
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: &rc,
			// Kata does not support host networking.
			HostNetwork:   false,
			RestartPolicy: corev1.RestartPolicyNever,
			Tolerations: []corev1.Toleration{{
				Key:      "panacea-app/sandbox",
				Operator: corev1.TolerationOpEqual,
				Value:    "true",
				Effect:   corev1.TaintEffectNoSchedule,
			}},
			NodeSelector: map[string]string{"panacea-app/role": "sandbox"},
			// The root filesystem is read-only, so every path the agent writes to
			// must be an explicit volume. The image's home directory is /home/dev
			// (uid 1001) and tooling writes to /tmp. Without these two mounts the
			// container starts and then fails on the first write.
			Volumes: []corev1.Volume{
				{Name: "home", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
			Containers: []corev1.Container{{
				Name:  "sandbox",
				Image: spec.Image,
				Env:   containerEnv(spec),
				Ports: []corev1.ContainerPort{{Name: "attach", ContainerPort: backend.ContainerPort}},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "home", MountPath: "/home/dev"},
					{Name: "tmp", MountPath: "/tmp"},
				},
				// Requests AND limits are mandatory: the Kata guest VM is
				// sized from them and under-requesting causes guest OOM.
				Resources: corev1.ResourceRequirements{
					Requests: quantities,
					Limits:   quantities,
				},
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot:             &nonRoot,
					RunAsUser:                &uid,
					ReadOnlyRootFilesystem:   &readOnlyRoot,
					AllowPrivilegeEscalation: &noEscalation,
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				},
			}},
		},
	}

	created, err := m.client.CoreV1().Pods(m.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

// Delete removes a sandbox pod.
func (m *Manager) Delete(ctx context.Context, podName string) error {
	return m.client.CoreV1().Pods(m.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

// Get returns the pod for a sandbox id, or ErrNotFound.
//
// It selects by label rather than fetching podName(id) directly, so that a pod
// created by some other tool under the same name is not mistaken for ours.
func (m *Manager) Get(ctx context.Context, id string) (*corev1.Pod, error) {
	if id == "" {
		// Without this the selector collapses to the component label and Get
		// returns an arbitrary sandbox belonging to somebody else.
		return nil, fmt.Errorf("sandbox %q: %w", id, ErrNotFound)
	}
	list, err := m.client.CoreV1().Pods(m.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector(id)})
	if err != nil {
		return nil, fmt.Errorf("list pods for sandbox %q: %w", id, err)
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("sandbox %q: %w", id, ErrNotFound)
	}
	// Copy: returning &list.Items[0] aliases the list's backing array, which
	// the caller does not own and which the next List call may reuse.
	pod := list.Items[0]
	return &pod, nil
}

// List returns every pod this manager created in its namespace.
func (m *Manager) List(ctx context.Context) ([]corev1.Pod, error) {
	list, err := m.client.CoreV1().Pods(m.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector("")})
	if err != nil {
		return nil, fmt.Errorf("list sandbox pods: %w", err)
	}
	return list.Items, nil
}
```

- [ ] **Step 4: Run the manager tests and watch them pass**

Run: `go test ./control-plane/internal/sandbox/... -race -count=1 -v`

Expected: PASS — the three Phase 1 tests (`TestCreateProducesAKataPodWithExplicitResources`, `TestDeleteRemovesThePod`, `TestCreateMountsWritablePathsUnderReadOnlyRoot`) and the eleven new ones, no race reports, final line `ok  github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox`.

Known failure mode: if `TestGetAndListIgnorePodsThisManagerDidNotCreate` reports two pods, the `List` selector is missing the component clause — `fake.NewSimpleClientset` **does** honour label selectors (it filters with `labels.Set(item.Labels)` in `fake_pod.go`), so a two-pod result is a real selector bug and not a fake-client limitation. If instead every test fails with an import cycle, the `internal/backend` import above has been added to a package that `backend` itself imports; `backend` must depend on nothing inside this repository.

- [ ] **Step 5: Write the failing tests for the Kata backend**

Everything here runs on a laptop with no cluster and no Azure account. The Kubernetes half is `fake.NewSimpleClientset()`, exactly as `manager_test.go` already does; the sandbox half is an `httptest` server on loopback that serves `/healthz` and upgrades `/attach`. Pod phases do not advance by themselves in the fake client, which is the point — the test writes the phase, so "Create waits for readiness" is observable rather than assumed.

What these pin, in order of what would hurt most if it broke: `Mounts` and `WorkDir` are refused rather than ignored; the CPU and memory quantities reach the pod as `"2000m"` and `"4096Mi"` in the requests **and** the limits — Kata sizes the guest VM from the requests, so a render that fills in only the limits is a guest OOM on a real cluster; `Create` does not return until the pod answers `/healthz`, and returns `ErrNotRunning` rather than a half-built `Sandbox` when it never does; every pod phase maps to exactly one `State`, including a Running pod whose kubelet has reported no container status at all; the pod's creation timestamp reaches `Sandbox.CreatedAt`; `Attach` carries the bearer token and surfaces task 4's sentinels unchanged; and `Destroy` is idempotent. Two more pin configuration rather than behaviour under load, and both would otherwise be uncovered: an unset `Config.ReadyTimeout` resolves to `backend.ReadyTimeout` and not to zero, and the pod's environment is exactly `Spec.EnvEntries()` — the same slice dockerbe renders — entry for entry in order.

Create `control-plane/internal/backend/katabe/kata_test.go`:

```go
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
```

Continuing `control-plane/internal/backend/katabe/kata_test.go`:

```go
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
	b, err := katabe.New(katabe.Config{Manager: sandbox.NewManager(client, namespace)})
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
```

Continuing `control-plane/internal/backend/katabe/kata_test.go`:

```go
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
}

// Kata sizes the guest VM from the requests, which is why all four quantities
// are asserted and not one of each pair: rendering 2000 millicpu as anything
// but "2000m", or filling in only the limits, is a guest OOM on a real cluster
// and is invisible until then.
func TestCreateRendersKubernetesQuantitiesAndTheArgvContract(t *testing.T) {
	spec := validSpec()
	_, client, _, sb := runningSandbox(t, spec)

	pod, err := client.CoreV1().Pods(namespace).Get(context.Background(), sb.Ref, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get %s: %v", sb.Ref, err)
	}
	c := pod.Spec.Containers[0]
	if got := c.Resources.Requests.Cpu().String(); got != "2000m" {
		t.Errorf("cpu request = %s, want 2000m (backend.Resources.KubeCPU); Kata sizes the guest VM from the requests", got)
	}
	if got := c.Resources.Limits.Cpu().String(); got != "2000m" {
		t.Errorf("cpu limit = %s, want 2000m (backend.Resources.KubeCPU)", got)
	}
	if got := c.Resources.Requests.Memory().String(); got != "4096Mi" {
		t.Errorf("memory request = %s, want 4096Mi (backend.Resources.KubeMemory); an under-requested guest OOMs before the limit is ever reached", got)
	}
	if got := c.Resources.Limits.Memory().String(); got != "4096Mi" {
		t.Errorf("memory limit = %s, want 4096Mi (backend.Resources.KubeMemory)", got)
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
```

Continuing `control-plane/internal/backend/katabe/kata_test.go`:

```go
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
			ID: id, Image: "x:y", Command: "claude", CPU: "500m", Memory: "1Gi", SessionID: id,
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
```

- [ ] **Step 6: Run the Kata tests and watch them fail**

Run: `go test ./control-plane/internal/backend/katabe/...`

Expected: FAIL to build, because the directory holds only a test file:

```
github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/katabe: no non-test Go files in C:\Users\au-dev-dmackie1\Projects\PanaceaApp\control-plane\internal\backend\katabe
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/katabe [build failed]
```

That is the right failure: it proves the test file is the only thing in the package, so nothing below can be mistaken for pre-existing behaviour. If you instead see `undefined: katabe.New`, `kata.go` already exists — this task is being redone, and the honest move is to delete the implementation and watch the compile fail before writing it again.

- [ ] **Step 7: Write the Kata backend**

Create `control-plane/internal/backend/katabe/kata.go`:

```go
// Package katabe runs sandboxes as Kata Containers pods on AKS. It is DORMANT:
// docs/PROGRAM.md phase 8 is the first real Azure spend, and nothing in Phase 2
// selects this backend by default. It compiles and is unit-tested now so that
// turning it on in Phase 8 is a configuration change and not a rewrite.
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
	b := &Backend{mgr: cfg.Manager, readyTimeout: cfg.ReadyTimeout, port: cfg.Port}
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
```

Continuing `control-plane/internal/backend/katabe/kata.go`:

```go
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
// The token is a parameter rather than something read back from the pod, so the
// secret lives in exactly one durable place (the registry) and is not
// recoverable by anything that can merely list pods.
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

// Destroy removes the pod. It is idempotent: the reaper and an explicit DELETE
// race each other, and a second delete must not become a 500 in the cockpit.
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
```

- [ ] **Step 8: Run the Kata tests and watch them pass**

Run: `go test ./control-plane/internal/backend/katabe/... -race -count=1 -v`

Expected: PASS — 20 top-level tests, of which `TestGetMapsEveryPodPhaseToOneState` reports 8 subtests (the 7 table cases plus the empty-`ContainerStatuses` case that follows them), no race reports, final line `ok  github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend/katabe`. Wall time is about three seconds: the only deliberate waits are the 400ms readiness timeout, the 700ms context in `TestNewDefaultsReadyTimeoutToTheBackendConstant`, and the 250ms pod poll. If that last test finishes in milliseconds instead of ~700ms, the `readyTimeout <= 0` default in `New` is missing and the test is telling you so.

No Docker and no cluster are involved. This is the one task in Group B that needs neither, which is the whole claim it is making.

Three known failure modes, with their causes:

- **`TestCreateWaitsForHealthzAndReturnsTheEndpoint` hangs for ten seconds and then fails with `never scheduled (state starting, endpoint "")`.** The pod status written by the helper is not being read back. `fake.NewSimpleClientset`'s tracker replaces the whole object on an `UpdateStatus`, so this means the goroutine's `Get` is looking at the wrong pod name or the wrong namespace. Print `client.Actions()` to see what was actually called; if `UpdateStatus` is genuinely being dropped, swap it for `Update` in both the helper and the goroutine — the fake tracker treats them the same.
- **`TestAttachWithTheWrongTokenIsErrUnauthorized` reports `ErrNotRunning` instead.** `backend.Dial` only maps a 401 when the handshake response is non-nil, so this means the fake sandboxd hijacked the connection before checking the header. The bearer check must run before `websocket.Accept`.
- **The race detector flags `fakeSandboxd.gotBearer` or the pod status.** The channel is buffered and the fake client's tracker is mutex-protected, so a report here means a test goroutine outlived its test — most likely `runningSandbox`'s goroutine still polling after a `t.Fatalf` elsewhere ended the test. Bound it with the 200-iteration counter it already has and do not add a `for {}`.

- [ ] **Step 9: Run the whole Go suite and prove nothing moved**

Run: `go test ./sandboxd/... ./control-plane/... -race -count=1`

Expected: every package `ok` or `[no test files]`. Never `go test ./...` — the workspace root is not a module and that form fails with `directory prefix . does not contain modules listed in go.work`.

Then prove this task added no dependency and moved no toolchain:

Run: `git diff --stat control-plane/go.mod control-plane/go.sum go.work`

Expected: **no output at all.** Every package this task imports — `k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`, `github.com/coder/websocket` — is already a `require` line, and no step ran `go get`. If `go.mod` has changed, something ran `go mod tidy`; check that the `go 1.23` directive is still `go 1.23` before doing anything else, because a bumped directive makes a network-restricted agent attempt a toolchain download and fail.

- [ ] **Step 10: Commit**

```bash
git add control-plane/internal/sandbox/manager.go control-plane/internal/sandbox/manager_growth_test.go control-plane/internal/backend/katabe/kata.go control-plane/internal/backend/katabe/kata_test.go
git commit -m "feat(katabe): the Kata backend, dormant but tested

An interface with one implementation is a type declaration. Writing katabe
now, against a fake clientset and an httptest sandboxd, is what makes the
claim in docs/PROGRAM.md - that Azure is a swap of one implementation rather
than a prerequisite - something that has been tested rather than asserted.
Phase 8 is still the first Azure spend; nothing here selects this backend.

internal/sandbox grows the four Spec fields a sandbox needs to match the
Docker contract, plus Get and List. Pod creation is untouched, so the Phase 1
tests still pass unchanged. Mounts and WorkDir are refused with ErrUnsupported
rather than ignored: a host path on an AKS node is meaningless, and dropping
either silently is a wrong-directory or empty-worktree bug with no error
anywhere.

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```
