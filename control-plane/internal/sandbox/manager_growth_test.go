package sandbox_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
//
// Compared with Equal rather than String(): resource.Quantity.String()
// canonicalizes to the smallest exact suffix, so resource.MustParse("2000m")
// prints as "2" and resource.MustParse("4096Mi") prints as "4Gi". The
// underlying quantity — what Kata actually sizes the guest VM from — is
// unaffected; only the text representation changes.
func TestCreateSetsRequestsEqualToLimitsAtTheGivenQuantities(t *testing.T) {
	c := createPod(t, fake.NewSimpleClientset(), growthSpec()).Spec.Containers[0]
	wantCPU := resource.MustParse("2000m")
	wantMem := resource.MustParse("4096Mi")
	reqCPU, limCPU := c.Resources.Requests.Cpu(), c.Resources.Limits.Cpu()
	reqMem, limMem := c.Resources.Requests.Memory(), c.Resources.Limits.Memory()

	if !reqCPU.Equal(wantCPU) {
		t.Errorf("cpu request = %s, want %s", reqCPU.String(), wantCPU.String())
	}
	if !limCPU.Equal(wantCPU) {
		t.Errorf("cpu limit = %s, want %s", limCPU.String(), wantCPU.String())
	}
	if !reqMem.Equal(wantMem) {
		t.Errorf("memory request = %s, want %s", reqMem.String(), wantMem.String())
	}
	if !limMem.Equal(wantMem) {
		t.Errorf("memory limit = %s, want %s", limMem.String(), wantMem.String())
	}

	// The actual invariant docs/PROGRAM.md cares about: a requests/limits
	// mismatch is what would let the scheduler admit the pod at one size and
	// Kata build the guest VM at another.
	if !reqCPU.Equal(*limCPU) {
		t.Errorf("cpu request %s != cpu limit %s", reqCPU.String(), limCPU.String())
	}
	if !reqMem.Equal(*limMem) {
		t.Errorf("memory request %s != memory limit %s", reqMem.String(), limMem.String())
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
