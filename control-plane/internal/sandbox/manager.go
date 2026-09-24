// Package sandbox creates the per-session Kata microVM pods that agent
// CLIs run inside.
package sandbox

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DefaultRuntimeClass is what AKS Pod Sandboxing is believed to install for
// workload_runtime = KataVmIsolation. UNVERIFIED (survey G-3): the first smoke
// check of phase 8 (task 32) is `kubectl get runtimeclass`, and the value is
// configuration precisely so that a wrong guess is an environment change.
const DefaultRuntimeClass = "kata-vm-isolation"

// AttachSecretKey is the Secret's one data key.
const AttachSecretKey = "token"

// componentSandbox is the value of the panacea-app/component label on every
// pod this manager creates. Get and List select on it, so a pod that merely
// carries a sandbox-id label is invisible to both.
const componentSandbox = "sandbox"

// annotationSafeToEvict tells the cluster autoscaler that consolidating a node
// must not evict this pod: a running agent is somebody's work in progress.
const annotationSafeToEvict = "cluster-autoscaler.kubernetes.io/safe-to-evict"

// agentID is the uid and gid the sandbox image runs as (/home/dev is 1001's).
const agentID = int64(1001)

// ownerNameChars is how much of an owner value goes into an object name.
const ownerNameChars = 8

// cleanupTimeout bounds the Secret delete after a refused pod create.
const cleanupTimeout = 10 * time.Second

// envReservedPrefix is the namespace of the three variables the sandboxd
// contract owns. internal/backend.ValidateSpec is the authoritative rejection;
// the check in containerEnv is the last line for a direct Manager caller.
const envReservedPrefix = "PANACEA_APP_"

// emptyArgsJSON is what "no arguments" looks like on the wire. It is not "".
const emptyArgsJSON = "[]"

// ErrNotFound is returned by Get when no pod carries the sandbox-id label.
var ErrNotFound = errors.New("sandbox: no pod for that id")

// ownerPattern is what WithOwner accepts: OwnerLabelValue's alphabet, at most
// a label value's 63 characters. It goes into a label selector by string
// concatenation, so a comma or an "=" here would widen the selector.
var ownerPattern = regexp.MustCompile(`^[a-z0-9]{1,63}$`)

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

// Config configures a Manager. It replaces the bare namespace argument.
type Config struct {
	// Namespace holds every pod and Secret this manager creates. Required.
	Namespace string
	// RuntimeClass is every pod's runtimeClassName. "" means DefaultRuntimeClass.
	RuntimeClass string
	// PriorityClass is every session pod's priorityClassName. "" sets none — a
	// fake clientset or a kind cluster has no panacea-session class, and a pod
	// naming a class that does not exist is rejected at admission.
	PriorityClass string
}

// Manager creates and removes sandbox pods in one namespace, optionally scoped
// to one owner.
type Manager struct {
	client kubernetes.Interface
	cfg    Config
	owner  string // "" = unscoped (the in-cluster tests, and phase 2's direct callers)
}

// NewManager returns an unscoped Manager.
func NewManager(client kubernetes.Interface, cfg Config) *Manager {
	if cfg.RuntimeClass == "" {
		cfg.RuntimeClass = DefaultRuntimeClass
	}
	return &Manager{client: client, cfg: cfg}
}

// WithOwner returns a view of m that stamps backend.KubeLabelOwner = owner on
// every pod and Secret it creates, and whose Get and List select on it, so one
// owner can neither see nor reach another's sandbox. It also names objects
// "sandbox-<owner[:8]>-<id>" (and "…-attach" for the Secret) instead of
// "sandbox-<id>", so two owners whose laptops happen to mint the same id do not
// collide — and a collision cannot reveal that another owner's id exists. The
// longest pod name is 8 + 8 + 1 + 40 = 57 characters, inside the 63 a pod's
// hostname is cut to; names are DNS subdomains, so the Secret's 64 is fine.
//
// owner must come from OwnerLabelValue. It panics on "" — an owner view with
// no owner is the bug this method exists to prevent — and on anything outside
// ^[a-z0-9]{1,63}$, because the owner is concatenated into a label selector.
// Called on an owner view it re-scopes to the new owner; it never widens.
func (m *Manager) WithOwner(owner string) *Manager {
	if owner == "" {
		panic("sandbox: WithOwner(\"\"): an owner view with no owner is an unscoped manager")
	}
	if !ownerPattern.MatchString(owner) {
		panic(fmt.Sprintf("sandbox: WithOwner(%q): an owner must match %s (use OwnerLabelValue)", owner, ownerPattern))
	}
	v := *m
	v.owner = owner
	return &v
}

// OwnerLabelValue is the owner label for an Entra principal: the first 32 lower-
// case hex characters of sha256(tenantID + "/" + objectID). Label-safe (≤ 63),
// stable across sessions, and not an object id anyone can read back.
func OwnerLabelValue(tenantID, objectID string) string {
	sum := sha256.Sum256([]byte(tenantID + "/" + objectID))
	return hex.EncodeToString(sum[:])[:32]
}

// PodName is the one place a sandbox id becomes a pod name. owner "" (the
// unscoped manager) gives "sandbox-<id>"; an OwnerLabelValue gives
// "sandbox-<owner[:8]>-<id>", the name WithOwner(owner) creates. Exported
// because the broker's rollback (task 18, deleteIfOurs) must name the pod its
// request created (b09 re-review 2).
func PodName(owner, id string) string {
	if owner == "" {
		return "sandbox-" + id
	}
	if len(owner) > ownerNameChars {
		owner = owner[:ownerNameChars]
	}
	return "sandbox-" + owner + "-" + id
}

// AttachSecretName is PodName(owner, id) + "-attach": the Secret holding that
// sandbox's attach token.
func AttachSecretName(owner, id string) string { return attachSecretFor(PodName(owner, id)) }

// attachSecretFor is the Secret name for a pod name. Delete derives it from the
// pod name it is given, and AttachSecretName goes through it, so the two
// cannot drift.
func attachSecretFor(podName string) string { return podName + "-attach" }

// podLabels are what Get, List and every kubectl invocation select on. The
// token Secret carries the same set.
//
// The session label is omitted rather than set empty when there is no session:
// an empty-valued label is noise in every listing and matches nothing useful.
func (m *Manager) podLabels(spec Spec) map[string]string {
	labels := map[string]string{
		backend.KubeLabelSandboxID: spec.ID,
		backend.KubeLabelComponent: componentSandbox,
	}
	if spec.SessionID != "" {
		labels[backend.KubeLabelSessionID] = spec.SessionID
	}
	if m.owner != "" {
		labels[backend.KubeLabelOwner] = m.owner
	}
	return labels
}

// containerEnv renders the container's environment: the three PANACEA_APP_*
// variables the sandboxd contract fixes, then every Spec.Env key in sorted
// order, so two identical Specs produce byte-identical pods and a test can
// assert on a slice instead of a set.
//
// The token is the one entry with no Value: it is a secretKeyRef to the
// sandbox's Secret, so nothing that can merely get or list pods reads it. The
// kubelet resolves the reference before the runtime sees the spec, so the
// agent's process sees the same variable either way.
//
// The order is deliberately the order backend.Spec.EnvEntries() produces, and
// the katabe test TestThePodEnvironmentIsExactlySpecEnvEntries pins the two
// together, because dockerbe renders EnvEntries() directly and the agent must
// see the same environment on both backends. This function re-derives the
// order rather than calling EnvEntries() because Manager takes sandbox.Spec
// and is called by code that never held a backend.Spec; the test is what stops
// the re-derivation from drifting.
func containerEnv(spec Spec, secretName string) []corev1.EnvVar {
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
		{Name: backend.EnvToken, ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  AttachSecretKey,
			},
		}},
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
// when id is non-empty, and to the view's owner in an owner view.
func (m *Manager) selector(id string) string {
	s := backend.KubeLabelComponent + "=" + componentSandbox
	if id != "" {
		s = backend.KubeLabelSandboxID + "=" + id + "," + s
	}
	if m.owner != "" {
		s += "," + backend.KubeLabelOwner + "=" + m.owner
	}
	return s
}

// attachSecret is the sandbox's token Secret, labelled as its pod is.
func (m *Manager) attachSecret(spec Spec, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: m.cfg.Namespace,
			Labels:    m.podLabels(spec),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{AttachSecretKey: []byte(spec.AttachToken)},
	}
}

// pod is the sandbox pod: Kata, non-root, read-only root, no API credentials,
// RuntimeDefault seccomp, and the token by reference only.
func (m *Manager) pod(spec Spec, name, secretName string) *corev1.Pod {
	rc := m.cfg.RuntimeClass
	yes, no := true, false
	uid := agentID
	onRootMismatch := corev1.FSGroupChangeOnRootMismatch
	quantities := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse(spec.CPU),
		corev1.ResourceMemory: resource.MustParse(spec.Memory),
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   m.cfg.Namespace,
			Labels:      m.podLabels(spec),
			Annotations: map[string]string{annotationSafeToEvict: "false"},
		},
		Spec: corev1.PodSpec{
			RuntimeClassName:  &rc,
			PriorityClassName: m.cfg.PriorityClass,
			// The agent never talks to the API server, and a mounted token
			// would let it, as the namespace's default service account.
			AutomountServiceAccountToken: &no,
			// Service links inject every Service in the namespace as env.
			EnableServiceLinks: &no,
			// Kata does not support host networking.
			HostNetwork:   false,
			RestartPolicy: corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &yes,
				RunAsUser:    &uid,
				RunAsGroup:   &uid,
				// Task 15's disk must be writable by the agent's uid.
				FSGroup:             &uid,
				FSGroupChangePolicy: &onRootMismatch,
				SeccompProfile:      &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
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
				Env:   containerEnv(spec, secretName),
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
					RunAsNonRoot:             &yes,
					RunAsUser:                &uid,
					ReadOnlyRootFilesystem:   &yes,
					AllowPrivilegeEscalation: &no,
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
					SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				},
			}},
		},
	}
}

// Create starts a sandbox pod and returns its name.
//
// The order is Secret, then Pod, then the Secret's owner reference to the Pod:
//   - The Secret is the id's claim. If it already exists, Create fails there
//     with the API server's AlreadyExists, unwrapped, and touches nothing: only
//     the request that created the Secret goes on to create the pod, so the
//     token in the Secret names that request (task 18's deleteIfOurs).
//   - A refused Pod create (podCreateRefused) deletes the Secret before the
//     error returns: the pod was not stored, and the token must not outlive it.
//   - Any other Pod create error is ambiguous — the pod may have been stored —
//     so the Secret is kept: it is the only proof of whose pod that is, and
//     Delete removes both.
//   - If the owner-reference update fails the pod is kept and the error is
//     returned with the pod's name; Delete removes both.
func (m *Manager) Create(ctx context.Context, spec Spec) (string, error) {
	name := PodName(m.owner, spec.ID)
	secretName := attachSecretFor(name)
	secrets := m.client.CoreV1().Secrets(m.cfg.Namespace)

	secret, err := secrets.Create(ctx, m.attachSecret(spec, secretName), metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	created, err := m.client.CoreV1().Pods(m.cfg.Namespace).Create(ctx, m.pod(spec, name, secretName), metav1.CreateOptions{})
	if err != nil {
		if podCreateRefused(err) {
			// A fresh, bounded context: the caller's may end at any moment,
			// and the token must still go.
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
			defer cancel()
			if derr := secrets.Delete(cleanup, secretName, metav1.DeleteOptions{}); derr != nil && !apierrors.IsNotFound(derr) {
				return "", errors.Join(err, fmt.Errorf("sandbox %q: delete the attach secret after the pod was refused: %w", spec.ID, derr))
			}
		}
		return "", err
	}

	no := false
	secret.OwnerReferences = []metav1.OwnerReference{{
		APIVersion:         "v1",
		Kind:               "Pod",
		Name:               created.Name,
		UID:                created.UID,
		Controller:         &no,
		BlockOwnerDeletion: &no,
	}}
	if _, err := secrets.Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		return created.Name, fmt.Errorf("sandbox %q: own the attach secret by pod %s (the pod is kept; Delete removes both): %w", spec.ID, created.Name, err)
	}
	return created.Name, nil
}

// podCreateRefused reports whether the API server answered a pod create with a
// refusal, which means the pod was not stored: a status error in the 4xx range
// other than the two timeout reasons. Everything else is ambiguous — a
// transport error or the context ending (no answer), a timeout status, and any
// 5xx, because an etcd failure during the write answers 500 or 503 and may
// still have committed it (b09-2 Minor 4; the part named only the timeouts).
func podCreateRefused(err error) bool {
	var status apierrors.APIStatus
	if !errors.As(err, &status) {
		return false
	}
	if apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) {
		return false
	}
	code := status.Status().Code
	return code >= http.StatusBadRequest && code < http.StatusInternalServerError
}

// Delete removes a sandbox pod and its attach Secret. Either's absence is
// success: the reaper and an explicit DELETE race, and a leftover Secret from
// an ambiguous create has no pod. The pod goes first; if its delete fails the
// Secret is kept, because it is the claim a retry — or the broker's
// deleteIfOurs — needs to prove whose pod that is.
func (m *Manager) Delete(ctx context.Context, podName string) error {
	if err := m.client.CoreV1().Pods(m.cfg.Namespace).Delete(ctx, podName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	secretName := attachSecretFor(podName)
	if err := m.client.CoreV1().Secrets(m.cfg.Namespace).Delete(ctx, secretName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete attach secret %s: %w", secretName, err)
	}
	return nil
}

// Get returns the pod for a sandbox id, or ErrNotFound. In an owner view it
// finds only that owner's pod.
//
// It selects by label rather than fetching PodName(owner, id) directly, so
// that a pod created by some other tool under the same name is not mistaken
// for ours.
func (m *Manager) Get(ctx context.Context, id string) (*corev1.Pod, error) {
	if id == "" {
		// Without this the selector collapses to the component label and Get
		// returns an arbitrary sandbox belonging to somebody else.
		return nil, fmt.Errorf("sandbox %q: %w", id, ErrNotFound)
	}
	list, err := m.client.CoreV1().Pods(m.cfg.Namespace).List(ctx, metav1.ListOptions{LabelSelector: m.selector(id)})
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

// List returns every pod this manager created in its namespace: in an owner
// view, that owner's only.
func (m *Manager) List(ctx context.Context) ([]corev1.Pod, error) {
	list, err := m.client.CoreV1().Pods(m.cfg.Namespace).List(ctx, metav1.ListOptions{LabelSelector: m.selector("")})
	if err != nil {
		return nil, fmt.Errorf("list sandbox pods: %w", err)
	}
	return list.Items, nil
}
