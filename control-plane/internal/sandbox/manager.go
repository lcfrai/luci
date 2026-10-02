// Package sandbox creates the per-session Kata microVM pods that agent
// CLIs run inside.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// runtimeClass is the RuntimeClass installed by AKS pod sandboxing.
const runtimeClass = "kata-mshv-vm-isolation"

// componentSandbox is the value of the luci/component label on every
// pod this manager creates. Get and List select on it, so a pod that merely
// carries a sandbox-id label is invisible to both.
const componentSandbox = "sandbox"

// envReservedPrefix is the namespace of the three variables the sandboxd
// contract owns. internal/backend.ValidateSpec is the authoritative rejection;
// the check in containerEnv is the last line for a direct Manager caller.
const envReservedPrefix = "LUCI_"

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

	// Instance is the owning control plane's instance id, stamped as
	// backend.KubeLabelInstance. katabe always sets it; it is omitted when
	// empty, like SessionID, because an empty label value matches nothing.
	Instance string
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
	if spec.Instance != "" {
		labels[backend.KubeLabelInstance] = spec.Instance
	}
	return labels
}

// containerEnv renders the container's environment: the three LUCI_*
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
		// sandboxd reads an empty LUCI_ARGS_JSON as "fall back to the
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
		// A caller that could set LUCI_ATTACH_TOKEN here could replace
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
				Key:      "luci/sandbox",
				Operator: corev1.TolerationOpEqual,
				Value:    "true",
				Effect:   corev1.TaintEffectNoSchedule,
			}},
			NodeSelector: map[string]string{"luci/role": "sandbox"},
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

// List returns every pod this manager created in its namespace — every
// control plane's. katabe.List drops another instance's.
func (m *Manager) List(ctx context.Context) ([]corev1.Pod, error) {
	list, err := m.client.CoreV1().Pods(m.namespace).List(ctx, metav1.ListOptions{LabelSelector: selector("")})
	if err != nil {
		return nil, fmt.Errorf("list sandbox pods: %w", err)
	}
	return list.Items, nil
}
