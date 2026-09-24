package sandbox_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/sandbox"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Phase 8 task 14: the pod has to be fit to share a cluster with other users'
// pods. Every test here uses the fake clientset and growthSpec().

func hardenedManager(cfg sandbox.Config) (*fake.Clientset, *sandbox.Manager) {
	if cfg.Namespace == "" {
		cfg.Namespace = growthNamespace
	}
	client := fake.NewSimpleClientset()
	return client, sandbox.NewManager(client, cfg)
}

// createWith creates growthSpec() through m and returns the stored pod.
func createWith(t *testing.T, client *fake.Clientset, m *sandbox.Manager, spec sandbox.Spec) *corev1.Pod {
	t.Helper()
	name, err := m.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	pod, err := client.CoreV1().Pods(growthNamespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get pod %s: %v", name, err)
	}
	return pod
}

func hardenedPod(t *testing.T, cfg sandbox.Config) *corev1.Pod {
	t.Helper()
	client, m := hardenedManager(cfg)
	return createWith(t, client, m, growthSpec())
}

func getSecret(t *testing.T, client *fake.Clientset, name string) *corev1.Secret {
	t.Helper()
	s, err := client.CoreV1().Secrets(growthNamespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get secret %s: %v", name, err)
	}
	return s
}

func secretCount(t *testing.T, client *fake.Clientset) int {
	t.Helper()
	list, err := client.CoreV1().Secrets(growthNamespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("List secrets: %v", err)
	}
	return len(list.Items)
}

func TestCreateUsesTheConfiguredRuntimeClass(t *testing.T) {
	pod := hardenedPod(t, sandbox.Config{RuntimeClass: "rc-x"})
	if rc := pod.Spec.RuntimeClassName; rc == nil || *rc != "rc-x" {
		t.Fatalf("runtimeClassName = %v, want rc-x: the constant is still in Create", rc)
	}
}

// An empty runtimeClassName is not "no preference": it schedules a runc pod,
// and the toleration lets it onto a Kata node without the microVM.
func TestCreateDefaultsTheRuntimeClass(t *testing.T) {
	pod := hardenedPod(t, sandbox.Config{})
	if rc := pod.Spec.RuntimeClassName; rc == nil || *rc != sandbox.DefaultRuntimeClass {
		t.Fatalf("runtimeClassName = %v, want %q", rc, sandbox.DefaultRuntimeClass)
	}
	if sandbox.DefaultRuntimeClass != "kata-vm-isolation" {
		t.Fatalf("DefaultRuntimeClass = %q, want kata-vm-isolation (P8-D15)", sandbox.DefaultRuntimeClass)
	}
}

func TestPodMountsNoServiceAccountToken(t *testing.T) {
	pod := hardenedPod(t, sandbox.Config{})
	if a := pod.Spec.AutomountServiceAccountToken; a == nil || *a {
		t.Fatalf("automountServiceAccountToken = %v, want false: the agent would hold API server credentials", a)
	}
}

func TestPodHasNoServiceLinks(t *testing.T) {
	pod := hardenedPod(t, sandbox.Config{})
	if l := pod.Spec.EnableServiceLinks; l == nil || *l {
		t.Fatalf("enableServiceLinks = %v, want false: every Service's address is injected as env", l)
	}
}

// RuntimeDefault on both levels is also what restricted Pod Security demands
// (task 11): without it the namespace rejects the pod outright.
func TestPodAndContainerUseRuntimeDefaultSeccomp(t *testing.T) {
	pod := hardenedPod(t, sandbox.Config{})
	psc := pod.Spec.SecurityContext
	if psc == nil || psc.SeccompProfile == nil || psc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("pod seccompProfile = %+v, want RuntimeDefault", psc)
	}
	csc := pod.Spec.Containers[0].SecurityContext
	if csc == nil || csc.SeccompProfile == nil || csc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("container seccompProfile = %+v, want RuntimeDefault", csc)
	}
}

// Task 15's disk is mounted into this pod; without fsGroup the agent's uid
// cannot write it.
func TestPodSecurityContextOwnsItsVolumesAsTheAgentUser(t *testing.T) {
	psc := hardenedPod(t, sandbox.Config{}).Spec.SecurityContext
	if psc == nil {
		t.Fatal("the pod has no securityContext")
	}
	for _, c := range []struct {
		name string
		got  *int64
	}{{"runAsUser", psc.RunAsUser}, {"runAsGroup", psc.RunAsGroup}, {"fsGroup", psc.FSGroup}} {
		if c.got == nil || *c.got != 1001 {
			t.Errorf("pod %s = %v, want 1001", c.name, c.got)
		}
	}
	if psc.RunAsNonRoot == nil || !*psc.RunAsNonRoot {
		t.Errorf("pod runAsNonRoot = %v, want true", psc.RunAsNonRoot)
	}
	if p := psc.FSGroupChangePolicy; p == nil || *p != corev1.FSGroupChangeOnRootMismatch {
		t.Errorf("fsGroupChangePolicy = %v, want OnRootMismatch", p)
	}
}

// katabe's Attach says the token "is not recoverable by anything that can
// merely list pods". A literal env value is exactly that.
func TestAttachTokenIsInASecretNotThePodSpec(t *testing.T) {
	client, m := hardenedManager(sandbox.Config{})
	spec := growthSpec()
	pod := createWith(t, client, m, spec)

	var tokenEnv *corev1.EnvVar
	for i, c := range pod.Spec.Containers {
		for j, e := range c.Env {
			if e.Value == spec.AttachToken {
				t.Errorf("container %d env %s carries the token as a literal value", i, e.Name)
			}
			if e.Name == "PANACEA_APP_ATTACH_TOKEN" {
				tokenEnv = &pod.Spec.Containers[i].Env[j]
			}
		}
	}
	if tokenEnv == nil {
		t.Fatal("PANACEA_APP_ATTACH_TOKEN is absent from the pod")
	}
	if tokenEnv.Value != "" {
		t.Errorf("PANACEA_APP_ATTACH_TOKEN value = %q, want empty", tokenEnv.Value)
	}
	ref := tokenEnv.ValueFrom
	if ref == nil || ref.SecretKeyRef == nil {
		t.Fatalf("PANACEA_APP_ATTACH_TOKEN valueFrom = %+v, want a secretKeyRef", ref)
	}
	want := "sandbox-" + spec.ID + "-attach"
	if ref.SecretKeyRef.Name != want || ref.SecretKeyRef.Key != sandbox.AttachSecretKey || sandbox.AttachSecretKey != "token" {
		t.Errorf("secretKeyRef = %s/%s, want %s/token", ref.SecretKeyRef.Name, ref.SecretKeyRef.Key, want)
	}
	if s := getSecret(t, client, want); string(s.Data["token"]) != spec.AttachToken {
		t.Errorf("secret %s data.token = %q, want the attach token", want, s.Data["token"])
	}
}

// The pod's UID is stamped by a reactor because the fake tracker assigns none,
// and a real API server rejects an owner reference with an empty uid.
func TestTheTokenSecretIsOwnedByItsPod(t *testing.T) {
	client, m := hardenedManager(sandbox.Config{})
	client.PrependReactor("create", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		a.(k8stesting.CreateAction).GetObject().(*corev1.Pod).UID = types.UID("uid-pod-1")
		return false, nil, nil
	})
	spec := growthSpec()
	createWith(t, client, m, spec)

	refs := getSecret(t, client, sandbox.AttachSecretName("", spec.ID)).OwnerReferences
	if len(refs) != 1 {
		t.Fatalf("owner references = %+v, want exactly one: the pod", refs)
	}
	r := refs[0]
	if r.Kind != "Pod" || r.APIVersion != "v1" || r.Name != "sandbox-"+spec.ID || r.UID != "uid-pod-1" {
		t.Errorf("owner reference = %+v, want v1 Pod sandbox-%s uid-pod-1", r, spec.ID)
	}
	if r.Controller == nil || *r.Controller || r.BlockOwnerDeletion == nil || *r.BlockOwnerDeletion {
		t.Errorf("controller = %v, blockOwnerDeletion = %v, want both explicitly false", r.Controller, r.BlockOwnerDeletion)
	}
}

func failPodCreate(client *fake.Clientset, err error) {
	client.PrependReactor("create", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
}

var podsResource = schema.GroupResource{Resource: "pods"}

// The API server answered 4xx, so the pod was not stored: the token must not
// outlive the request.
func TestCreateDeletesTheSecretWhenThePodIsRefused(t *testing.T) {
	for _, refusal := range []error{
		apierrors.NewForbidden(podsResource, "sandbox-s-3f2a91c0", errors.New("exceeded quota")),
		apierrors.NewInvalid(schema.GroupKind{Kind: "Pod"}, "sandbox-s-3f2a91c0", nil),
		apierrors.NewAlreadyExists(podsResource, "sandbox-s-3f2a91c0"),
		apierrors.NewTooManyRequests("slow down", 1),
	} {
		client, m := hardenedManager(sandbox.Config{})
		failPodCreate(client, refusal)
		if _, err := m.Create(context.Background(), growthSpec()); err == nil {
			t.Fatalf("Create with the pod refused (%v) returned nil", refusal)
		}
		if n := secretCount(t, client); n != 0 {
			t.Errorf("pod refused with %q: %d secret(s) left behind, want 0", apierrors.ReasonForError(refusal), n)
		}
	}
}

// The pod may have been stored. Deleting the Secret would leave a stored pod
// whose owner the broker's rollback could never prove (b09 re-review 2). A 5xx
// is here too, not only the two timeout reasons: an etcd failure during the
// write answers 500 and may still have committed it (b09-2 Minor 4).
func TestCreateKeepsTheSecretWhenThePodCreateIsAmbiguous(t *testing.T) {
	for _, ambiguous := range []error{
		context.DeadlineExceeded,
		fmt.Errorf("post pods: %w", errors.New("connection reset by peer")),
		apierrors.NewTimeoutError("the request timed out", 1),
		apierrors.NewServerTimeout(podsResource, "create", 1),
		apierrors.NewInternalError(errors.New("etcdserver: request timed out")),
		apierrors.NewServiceUnavailable("etcd leader changed"),
	} {
		client, m := hardenedManager(sandbox.Config{})
		failPodCreate(client, ambiguous)
		spec := growthSpec()
		if _, err := m.Create(context.Background(), spec); err == nil {
			t.Fatalf("Create with an ambiguous pod create (%v) returned nil", ambiguous)
		}
		s, err := client.CoreV1().Secrets(growthNamespace).Get(context.Background(), sandbox.AttachSecretName("", spec.ID), metav1.GetOptions{})
		if err != nil {
			t.Errorf("ambiguous pod create (%v): the secret is gone (%v), want it kept", ambiguous, err)
			continue
		}
		if string(s.Data[sandbox.AttachSecretKey]) != spec.AttachToken {
			t.Errorf("ambiguous pod create (%v): secret holds %q, want the token", ambiguous, s.Data[sandbox.AttachSecretKey])
		}
	}
}

// The Secret is the id's claim: only the request that created it may go on.
func TestCreateLeavesAnExistingSecretAlone(t *testing.T) {
	spec := growthSpec()
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: sandbox.AttachSecretName("", spec.ID), Namespace: growthNamespace},
		Data:       map[string][]byte{sandbox.AttachSecretKey: []byte("tok-A")},
	})
	m := sandbox.NewManager(client, sandbox.Config{Namespace: growthNamespace})
	spec.AttachToken = "tok-B"
	_, err := m.Create(context.Background(), spec)
	if !apierrors.IsAlreadyExists(err) {
		t.Fatalf("Create over an existing claim = %v, want AlreadyExists", err)
	}
	pods, _ := client.CoreV1().Pods(growthNamespace).List(context.Background(), metav1.ListOptions{})
	if len(pods.Items) != 0 {
		t.Errorf("a pod was created behind another request's claim: %v", podNames(pods.Items))
	}
	if got := string(getSecret(t, client, sandbox.AttachSecretName("", spec.ID)).Data[sandbox.AttachSecretKey]); got != "tok-A" {
		t.Errorf("the existing claim now holds %q, want tok-A untouched", got)
	}
}

// The owner reference is written after the pod exists. If that write fails the
// pod is kept, the error returned, and Delete still removes both.
func TestAFailedOwnerReferenceKeepsThePodAndDeleteRemovesBoth(t *testing.T) {
	client, m := hardenedManager(sandbox.Config{})
	client.PrependReactor("update", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewServiceUnavailable("try later")
	})
	spec := growthSpec()
	name, err := m.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("Create with the owner-reference update failing returned nil")
	}
	if name != sandbox.PodName("", spec.ID) {
		t.Errorf("Create returned name %q with its error, want %q so the caller can Delete it", name, sandbox.PodName("", spec.ID))
	}
	if _, err := client.CoreV1().Pods(growthNamespace).Get(context.Background(), sandbox.PodName("", spec.ID), metav1.GetOptions{}); err != nil {
		t.Fatalf("the pod is gone after a failed owner-reference update (%v), want it kept", err)
	}
	if err := m.Delete(context.Background(), sandbox.PodName("", spec.ID)); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if n := secretCount(t, client); n != 0 {
		t.Errorf("%d secret(s) left after Delete, want 0", n)
	}
}

// The broker's rollback (task 18) names the pod with PodName; if that differs
// from what the owner view created, the rollback deletes nothing.
func TestPodNameIsWhatTheOwnerViewCreates(t *testing.T) {
	if got := sandbox.PodName("", "s-1"); got != "sandbox-s-1" {
		t.Errorf(`PodName("", "s-1") = %q, want sandbox-s-1`, got)
	}
	if got := sandbox.AttachSecretName("", "s-1"); got != "sandbox-s-1-attach" {
		t.Errorf(`AttachSecretName("", "s-1") = %q, want sandbox-s-1-attach`, got)
	}
	o := sandbox.OwnerLabelValue("tenant-1", "object-1")
	if want := "sandbox-" + o[:8] + "-s-1"; sandbox.PodName(o, "s-1") != want {
		t.Errorf("PodName(owner, s-1) = %q, want %q", sandbox.PodName(o, "s-1"), want)
	}
	client, m := hardenedManager(sandbox.Config{})
	spec := growthSpec()
	spec.ID = "s-1"
	name, err := m.WithOwner(o).Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("owner-view Create: %v", err)
	}
	if name != sandbox.PodName(o, "s-1") {
		t.Errorf("the owner view created pod %q, want PodName(owner, s-1) = %q", name, sandbox.PodName(o, "s-1"))
	}
	getSecret(t, client, sandbox.AttachSecretName(o, "s-1"))
	if sandbox.AttachSecretName(o, "s-1") != sandbox.PodName(o, "s-1")+"-attach" {
		t.Errorf("AttachSecretName(owner, s-1) = %q, want the pod name plus -attach", sandbox.AttachSecretName(o, "s-1"))
	}
	// The longest name: 8 + 8 + 1 + 40 characters, inside a hostname's 63.
	long := strings.Repeat("a", 40)
	if n := len(sandbox.PodName(o, long)); n != 57 {
		t.Errorf("the longest owner-view pod name is %d characters, want 57", n)
	}
}

// The reaper and an explicit DELETE race; the second must not be a 500.
func TestDeleteRemovesPodAndSecretAndIsIdempotent(t *testing.T) {
	client, m := hardenedManager(sandbox.Config{})
	spec := growthSpec()
	createWith(t, client, m, spec)
	name := sandbox.PodName("", spec.ID)
	if err := m.Delete(context.Background(), name); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := client.CoreV1().Pods(growthNamespace).Get(context.Background(), name, metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("pod after Delete: %v, want NotFound", err)
	}
	if n := secretCount(t, client); n != 0 {
		t.Errorf("%d secret(s) after Delete, want 0: the token outlives its sandbox", n)
	}
	if err := m.Delete(context.Background(), name); err != nil {
		t.Fatalf("second Delete = %v, want nil", err)
	}
}

// A pod-delete failure keeps the Secret: it is the claim a retry, or the
// broker's deleteIfOurs, needs to prove the pod is its own.
func TestDeleteKeepsTheSecretWhenThePodDeleteFails(t *testing.T) {
	client, m := hardenedManager(sandbox.Config{})
	spec := growthSpec()
	createWith(t, client, m, spec)
	client.PrependReactor("delete", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewServiceUnavailable("try later")
	})
	if err := m.Delete(context.Background(), sandbox.PodName("", spec.ID)); err == nil {
		t.Fatal("Delete with the pod delete failing returned nil")
	}
	if n := secretCount(t, client); n != 1 {
		t.Errorf("%d secret(s) after a failed pod delete, want the claim kept", n)
	}
}

func TestOwnerViewStampsTheLabel(t *testing.T) {
	client, m := hardenedManager(sandbox.Config{})
	pod := createWith(t, client, m.WithOwner("o1"), growthSpec())
	if got := pod.Labels["panacea-app/owner"]; got != "o1" {
		t.Errorf("pod owner label = %q, want o1", got)
	}
	s := getSecret(t, client, sandbox.AttachSecretName("o1", growthSpec().ID))
	if got := s.Labels["panacea-app/owner"]; got != "o1" {
		t.Errorf("secret owner label = %q, want o1", got)
	}
	for k, v := range pod.Labels {
		if s.Labels[k] != v {
			t.Errorf("secret label %s = %q, want the pod's %q", k, s.Labels[k], v)
		}
	}
	unscoped := hardenedPod(t, sandbox.Config{})
	if _, ok := unscoped.Labels["panacea-app/owner"]; ok {
		t.Error("the unscoped manager stamped an owner label")
	}
}

// OQ-1: one owner can neither see nor reach another's sandbox.
func TestOwnerViewsCannotSeeEachOther(t *testing.T) {
	client, m := hardenedManager(sandbox.Config{})
	spec := growthSpec()
	createWith(t, client, m.WithOwner("o1"), spec)

	o2 := m.WithOwner("o2")
	if _, err := o2.Get(context.Background(), spec.ID); !errors.Is(err, sandbox.ErrNotFound) {
		t.Errorf("o2.Get(o1's id) = %v, want ErrNotFound", err)
	}
	if pods, err := o2.List(context.Background()); err != nil || len(pods) != 0 {
		t.Errorf("o2.List = %v, %v, want empty", podNames(pods), err)
	}
	if pod, err := m.WithOwner("o1").Get(context.Background(), spec.ID); err != nil || pod.Labels["panacea-app/owner"] != "o1" {
		t.Errorf("o1.Get(own id) = %v, %v, want o1's pod", pod, err)
	}
	if pods, err := m.List(context.Background()); err != nil || len(pods) != 1 {
		t.Errorf("unscoped List = %v, %v, want o1's pod", podNames(pods), err)
	}
}

// With sandbox-<id> names the second owner's create would fail with a
// conflict that proves the first owner's id exists.
func TestTwoOwnersMayUseTheSameID(t *testing.T) {
	client, m := hardenedManager(sandbox.Config{})
	spec := growthSpec()
	spec.ID = "s-1234abcd"
	for _, o := range []string{"o1", "o2"} {
		s := spec
		s.AttachToken = "tok-" + o
		if _, err := m.WithOwner(o).Create(context.Background(), s); err != nil {
			t.Fatalf("%s Create(%s): %v", o, spec.ID, err)
		}
	}
	all, err := m.List(context.Background())
	if err != nil || len(all) != 2 {
		t.Fatalf("unscoped List = %v, %v, want two pods", podNames(all), err)
	}
	for _, o := range []string{"o1", "o2"} {
		pod, err := m.WithOwner(o).Get(context.Background(), spec.ID)
		if err != nil {
			t.Fatalf("%s Get: %v", o, err)
		}
		if pod.Labels["panacea-app/owner"] != o || pod.Name != sandbox.PodName(o, spec.ID) {
			t.Errorf("%s Get returned %s (owner %q), want its own pod", o, pod.Name, pod.Labels["panacea-app/owner"])
		}
		sec := getSecret(t, client, sandbox.AttachSecretName(o, spec.ID))
		if string(sec.Data[sandbox.AttachSecretKey]) != "tok-"+o {
			t.Errorf("%s's secret holds %q, want tok-%s", o, sec.Data[sandbox.AttachSecretKey], o)
		}
	}
}

func TestWithOwnerRefusesAnEmptyOwner(t *testing.T) {
	_, m := hardenedManager(sandbox.Config{})
	for _, bad := range []string{"", "o1,panacea-app/component=sandbox", "O1", "o 1"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("WithOwner(%q) did not panic: a scoped view with no usable owner", bad)
				}
			}()
			m.WithOwner(bad)
		}()
	}
}

func TestOwnerLabelValueIsStableAndLabelSafe(t *testing.T) {
	a := sandbox.OwnerLabelValue("tid-1", "oid-1")
	if b := sandbox.OwnerLabelValue("tid-1", "oid-1"); a != b {
		t.Errorf("OwnerLabelValue changed between calls: %q then %q", a, b)
	}
	if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(a) {
		t.Errorf("OwnerLabelValue = %q, want 32 lower-case hex characters", a)
	}
	if a == sandbox.OwnerLabelValue("tid-1", "oid-2") || a == sandbox.OwnerLabelValue("tid-2", "oid-1") {
		t.Error("a different principal got the same owner value")
	}
	if strings.Contains(a, "oid-1") || strings.Contains(a, "tid-1") {
		t.Errorf("OwnerLabelValue %q embeds the raw id", a)
	}
	// The derivation task 17's whoami must reproduce: sha256(tid + "/" + oid),
	// the first 32 lower-case hex characters.
	sum := sha256.Sum256([]byte("tid-1/oid-1"))
	if want := hex.EncodeToString(sum[:])[:32]; a != want {
		t.Errorf("OwnerLabelValue(tid-1, oid-1) = %s, want %s", a, want)
	}
	// Moving the "/" between tenant and object must not collide.
	if sandbox.OwnerLabelValue("ab", "c") == sandbox.OwnerLabelValue("a", "bc") {
		t.Error(`("ab","c") and ("a","bc") share an owner: the separator is missing`)
	}
}

func TestSessionPodIsNotSafeToEvict(t *testing.T) {
	pod := hardenedPod(t, sandbox.Config{})
	if got := pod.Annotations["cluster-autoscaler.kubernetes.io/safe-to-evict"]; got != "false" {
		t.Fatalf("safe-to-evict = %q, want \"false\"", got)
	}
}

// A pod naming a class the cluster lacks is rejected at admission, so none is
// set unless configured.
func TestPriorityClassOnlyWhenConfigured(t *testing.T) {
	if got := hardenedPod(t, sandbox.Config{}).Spec.PriorityClassName; got != "" {
		t.Errorf("priorityClassName = %q with none configured, want empty", got)
	}
	if got := hardenedPod(t, sandbox.Config{PriorityClass: "panacea-session"}).Spec.PriorityClassName; got != "panacea-session" {
		t.Errorf("priorityClassName = %q, want panacea-session", got)
	}
}
