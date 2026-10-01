package sandbox_test

import (
	"context"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// machineOwner is an OwnerLabelValue-shaped owner: 32 lower-case hex.
const machineOwner = "0123456789abcdef0123456789abcdef"

const machineID = "m-0123abcd"

// machineSpec is a session on a cloud machine: its worktree and its
// conversation store, both sub-paths of the machine's one disk (P8-D6).
func machineSpec() sandbox.Spec {
	return sandbox.Spec{
		ID:      "abc123",
		Image:   "example/sandbox:dev",
		Command: "claude",
		CPU:     "2000m",
		Memory:  "4096Mi",
		WorkDir: "/home/dev/work",
		Machine: machineID,
		MachineMounts: []sandbox.VolumeMount{
			{SubPath: "lanes/p-1/w-2", MountPath: "/home/dev/work"},
			{SubPath: "conversations/p-1/w-2", MountPath: "/home/dev/.claude/projects", ReadOnly: true},
		},
	}
}

// createPod runs Create and returns the pod it stored.
func createPod(t *testing.T, m *sandbox.Manager, client *fake.Clientset, spec sandbox.Spec) *corev1.Pod {
	t.Helper()
	name, err := m.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	pod, err := client.CoreV1().Pods("panacea-app").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get %s: %v", name, err)
	}
	return pod
}

// ownerManager is an owner view over a fresh fake clientset.
func ownerManager(t *testing.T, cfg sandbox.Config) (*sandbox.Manager, *fake.Clientset) {
	t.Helper()
	cfg.Namespace = "panacea-app"
	client := fake.NewSimpleClientset()
	return sandbox.NewManager(client, cfg).WithOwner(machineOwner), client
}

// mountAt returns the container's volumeMount at path, or false.
func mountAt(pod *corev1.Pod, path string) (corev1.VolumeMount, bool) {
	for _, vm := range pod.Spec.Containers[0].VolumeMounts {
		if vm.MountPath == path {
			return vm, true
		}
	}
	return corev1.VolumeMount{}, false
}

// volumeNamed returns the pod volume called name, or false.
func volumeNamed(pod *corev1.Pod, name string) (corev1.Volume, bool) {
	for _, v := range pod.Spec.Volumes {
		if v.Name == name {
			return v, true
		}
	}
	return corev1.Volume{}, false
}

// The claim is the whole disk of one machine belonging to one owner. The wrong
// name is another machine's disk, or another owner's disk whose id happens to
// match — which is why the owner is in the name and why an unscoped Manager,
// which has no owner to put there, refuses the create outright (review m12).
func TestMachineSpecMountsTheMachineClaim(t *testing.T) {
	m, client := ownerManager(t, sandbox.Config{})
	pod := createPod(t, m, client, machineSpec())

	vol, ok := volumeNamed(pod, "machine")
	if !ok {
		t.Fatalf("no volume named \"machine\" in %v", pod.Spec.Volumes)
	}
	if vol.PersistentVolumeClaim == nil {
		t.Fatalf("volume \"machine\" is %+v, want a persistentVolumeClaim", vol.VolumeSource)
	}
	want := "machine-" + machineOwner[:8] + "-" + machineID
	if got := vol.PersistentVolumeClaim.ClaimName; got != want {
		t.Errorf("claimName = %q, want %q", got, want)
	}
	if got := sandbox.MachineClaimName(machineOwner, machineID); got != want {
		t.Errorf("MachineClaimName = %q, want %q", got, want)
	}

	unscoped := sandbox.NewManager(fake.NewSimpleClientset(), sandbox.Config{Namespace: "panacea-app"})
	if _, err := unscoped.Create(context.Background(), machineSpec()); err == nil {
		t.Fatal("unscoped Create(Spec.Machine set) = nil: a machine volume exists only in an owner view, and without an owner the claim name is somebody's guess")
	}
}

// The whole disk mounted at /home/dev/work would make every project on the
// machine visible to every session on it. Each mount is one sub-path.
func TestMachineMountsAreSubPaths(t *testing.T) {
	m, client := ownerManager(t, sandbox.Config{})
	spec := machineSpec()
	pod := createPod(t, m, client, spec)

	for _, want := range spec.MachineMounts {
		vm, ok := mountAt(pod, want.MountPath)
		if !ok {
			t.Errorf("no volumeMount at %s", want.MountPath)
			continue
		}
		if vm.Name != "machine" {
			t.Errorf("mount at %s uses volume %q, want \"machine\"", want.MountPath, vm.Name)
		}
		if vm.SubPath != want.SubPath {
			t.Errorf("mount at %s has subPath %q, want %q", want.MountPath, vm.SubPath, want.SubPath)
		}
		if vm.ReadOnly != want.ReadOnly {
			t.Errorf("mount at %s has readOnly %v, want %v", want.MountPath, vm.ReadOnly, want.ReadOnly)
		}
	}
}

// Without this the agent starts in the image default, /home/dev, and every
// relative path it is given resolves outside the worktree.
func TestWorkDirBecomesTheWorkingDirectory(t *testing.T) {
	m, client := ownerManager(t, sandbox.Config{})
	pod := createPod(t, m, client, machineSpec())
	if got := pod.Spec.Containers[0].WorkingDir; got != "/home/dev/work" {
		t.Fatalf("workingDir = %q, want /home/dev/work", got)
	}
}

// A ReadWriteOnce disk attaches to one node. Preferred affinity would let the
// scheduler place the session elsewhere, where the disk cannot attach and the
// pod sits in ContainerCreating until somebody deletes it. The owner is in the
// selector too: owners share nodes (task 18), so a session must not follow
// another owner's machine pod whose id happens to match.
func TestSessionIsCoLocatedWithItsMachine(t *testing.T) {
	m, client := ownerManager(t, sandbox.Config{MachineAffinity: true})
	pod := createPod(t, m, client, machineSpec())

	if pod.Spec.Affinity == nil || pod.Spec.Affinity.PodAffinity == nil {
		t.Fatalf("pod has no podAffinity: %+v", pod.Spec.Affinity)
	}
	terms := pod.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	if len(terms) != 1 {
		t.Fatalf("podAffinity has %d required term(s), want 1: %+v", len(terms), terms)
	}
	if len(pod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution) != 0 {
		t.Error("podAffinity is preferred, not required: the scheduler may place the session where the RWO disk cannot attach")
	}
	if got := terms[0].TopologyKey; got != "kubernetes.io/hostname" {
		t.Errorf("topologyKey = %q, want kubernetes.io/hostname: the disk attaches to a node, not a zone", got)
	}
	if terms[0].LabelSelector == nil {
		t.Fatal("affinity term has no labelSelector: it would match every pod in the namespace")
	}
	want := map[string]string{
		"panacea-app/component":  "machine",
		"panacea-app/machine-id": machineID,
		"panacea-app/owner":      machineOwner,
	}
	got := terms[0].LabelSelector.MatchLabels
	for k, v := range want {
		if got[k] != v {
			t.Errorf("affinity selector[%q] = %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("affinity selector = %v, want exactly %v", got, want)
	}
	if pod.Labels["panacea-app/machine-id"] != machineID {
		t.Errorf("machine-id label = %q, want %q", pod.Labels["panacea-app/machine-id"], machineID)
	}
}

// The NFS fallback (task 32) is shared, RWX storage: a required node affinity
// there would pile every session of a machine onto one node for no reason.
func TestNoAffinityForSharedStorage(t *testing.T) {
	m, client := ownerManager(t, sandbox.Config{MachineAffinity: false})
	pod := createPod(t, m, client, machineSpec())
	if pod.Spec.Affinity != nil {
		t.Fatalf("pod has affinity %+v with MachineAffinity false", pod.Spec.Affinity)
	}
}

// A local-style spec must not reference a claim that does not exist: the pod
// would never schedule, and the failure would read as a cluster fault.
func TestNoMachineMeansNoMachineVolume(t *testing.T) {
	m, client := ownerManager(t, sandbox.Config{MachineAffinity: true})
	spec := machineSpec()
	spec.Machine = ""
	spec.MachineMounts = nil
	pod := createPod(t, m, client, spec)

	if _, ok := volumeNamed(pod, "machine"); ok {
		t.Error("pod has a \"machine\" volume with no Spec.Machine")
	}
	if pod.Spec.Affinity != nil {
		t.Errorf("pod has affinity %+v with no Spec.Machine", pod.Spec.Affinity)
	}
	if _, ok := pod.Labels["panacea-app/machine-id"]; ok {
		t.Errorf("pod carries machine-id = %q with no Spec.Machine", pod.Labels["panacea-app/machine-id"])
	}
}

// ~/.claude outside projects/ holds /login's credentials. They must not land
// on the machine disk beside the transcripts: /home/dev stays the emptyDir
// that dies with the pod, and only the conversation sub-directory is
// persisted (seam S8-P6.1).
func TestCredentialsStayOffTheDisk(t *testing.T) {
	m, client := ownerManager(t, sandbox.Config{MachineAffinity: true})
	pod := createPod(t, m, client, machineSpec())

	home, ok := mountAt(pod, "/home/dev")
	if !ok {
		t.Fatal("no volumeMount at /home/dev")
	}
	if home.Name != "home" {
		t.Fatalf("/home/dev is volume %q, want the \"home\" emptyDir", home.Name)
	}
	if home.SubPath != "" {
		t.Errorf("/home/dev has subPath %q: it must be the whole emptyDir", home.SubPath)
	}
	vol, ok := volumeNamed(pod, "home")
	if !ok || vol.EmptyDir == nil {
		t.Fatalf("volume \"home\" is %+v, want an emptyDir", vol.VolumeSource)
	}
	if vm, ok := mountAt(pod, "/home/dev/.claude"); ok {
		t.Errorf("a volume is mounted at /home/dev/.claude (%+v): /login's credentials would persist beside the transcripts", vm)
	}
}
