// Package sandbox creates the per-session Kata microVM pods that agent
// CLIs run inside.
package sandbox

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// runtimeClass is the RuntimeClass installed by AKS pod sandboxing.
const runtimeClass = "kata-mshv-vm-isolation"

// Spec describes one sandbox.
type Spec struct {
	ID      string
	Image   string
	Command string
	CPU     string
	Memory  string
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

// Create starts a sandbox pod and returns its name.
func (m *Manager) Create(ctx context.Context, spec Spec) (string, error) {
	name := "sandbox-" + spec.ID
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
			Labels: map[string]string{
				"panacea-app/sandbox-id": spec.ID,
				"panacea-app/component":  "sandbox",
			},
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
				Env: []corev1.EnvVar{
					{Name: "PANACEA_APP_COMMAND", Value: spec.Command},
				},
				Ports: []corev1.ContainerPort{{Name: "attach", ContainerPort: 8080}},
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
