package sandbox_test

import (
	"context"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreateProducesAKataPodWithExplicitResources(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, "panacea-app")

	name, err := m.Create(context.Background(), sandbox.Spec{
		ID:      "abc123",
		Image:   "example/sandbox:dev",
		Command: "/bin/bash",
		CPU:     "500m",
		Memory:  "1Gi",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	pod, err := client.CoreV1().Pods("panacea-app").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "kata-mshv-vm-isolation" {
		t.Errorf("RuntimeClassName = %v, want kata-mshv-vm-isolation", pod.Spec.RuntimeClassName)
	}
	if pod.Spec.HostNetwork {
		t.Error("HostNetwork must be false: Kata does not support it")
	}

	c := pod.Spec.Containers[0]
	// Kata sizes the guest VM from the sum of requests and limits. Missing
	// values cause guest OOM, so both must be present.
	for _, key := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
		if _, ok := c.Resources.Requests[key]; !ok {
			t.Errorf("missing resource request %q", key)
		}
		if _, ok := c.Resources.Limits[key]; !ok {
			t.Errorf("missing resource limit %q", key)
		}
	}
	if c.SecurityContext == nil || c.SecurityContext.RunAsNonRoot == nil || !*c.SecurityContext.RunAsNonRoot {
		t.Error("container must run as non-root")
	}
	if pod.Labels["panacea-app/sandbox-id"] != "abc123" {
		t.Errorf("sandbox-id label = %q, want abc123", pod.Labels["panacea-app/sandbox-id"])
	}
}

func TestDeleteRemovesThePod(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, "panacea-app")

	name, err := m.Create(context.Background(), sandbox.Spec{
		ID: "gone", Image: "x:y", Command: "/bin/sh", CPU: "100m", Memory: "256Mi",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.Delete(context.Background(), name); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := client.CoreV1().Pods("panacea-app").Get(context.Background(), name, metav1.GetOptions{}); err == nil {
		t.Fatal("pod still present after Delete")
	}
}

func TestCreateMountsWritablePathsUnderReadOnlyRoot(t *testing.T) {
	client := fake.NewSimpleClientset()
	m := sandbox.NewManager(client, "panacea-app")
	name, err := m.Create(context.Background(), sandbox.Spec{
		ID: "vols", Image: "x:y", Command: "/bin/bash", CPU: "500m", Memory: "1Gi",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	pod, err := client.CoreV1().Pods("panacea-app").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	c := pod.Spec.Containers[0]
	if c.SecurityContext == nil || c.SecurityContext.ReadOnlyRootFilesystem == nil || !*c.SecurityContext.ReadOnlyRootFilesystem {
		t.Fatal("root filesystem must stay read-only")
	}
	// Every writable path the image needs must be mounted, or the container
	// fails on its first write at runtime.
	want := map[string]bool{"/home/dev": false, "/tmp": false}
	for _, vm := range c.VolumeMounts {
		if _, ok := want[vm.MountPath]; ok {
			want[vm.MountPath] = true
		}
	}
	for path, mounted := range want {
		if !mounted {
			t.Errorf("no volume mounted at %s; writes there will fail under a read-only root", path)
		}
	}
	if len(pod.Spec.Volumes) < 2 {
		t.Errorf("got %d volumes, want at least 2", len(pod.Spec.Volumes))
	}
}
