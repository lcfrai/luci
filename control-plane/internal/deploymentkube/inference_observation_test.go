package deploymentkube

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ktesting "k8s.io/client-go/testing"
	"strings"
	"testing"
)

func TestInferenceOwnedPodObservationRejectsAdmissionMutationAndForeignLineage(t *testing.T) {
	c, client, _, request := workloadFixture(t)
	service := request.Blueprint.Services[0]
	service.Health = &deploymentplan.ServiceHealth{Readiness: &deploymentplan.Probe{Kind: "tcp", PeriodSeconds: 5, TimeoutSeconds: 1, FailureThreshold: 3}}
	request.Blueprint.Services[0] = service
	plan, err := deploymentplan.Compile(request.Blueprint)
	if err != nil {
		t.Fatal(err)
	}
	request.Identity.PlanDigest = plan.Digest
	if _, err = c.Apply(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	deployment, err := client.AppsV1().Deployments("apps").Get(context.Background(), request.Expected[0].Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	deployment.Status.ObservedGeneration = deployment.Generation
	set := &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "current", Namespace: "apps", UID: "rs-one", Labels: deployment.Spec.Template.Labels, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.Name, UID: deployment.UID, Controller: pointer(true)}}}, Spec: appsv1.ReplicaSetSpec{Template: *deployment.Spec.Template.DeepCopy()}}
	if err = client.Tracker().Add(set); err != nil {
		t.Fatal(err)
	}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "owned", Namespace: "apps", UID: "pod-one", Labels: deployment.Spec.Template.Labels, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: set.Name, UID: set.UID, Controller: pointer(true)}}}, Spec: *set.Spec.Template.Spec.DeepCopy(), Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}, ContainerStatuses: []corev1.ContainerStatus{{Name: service.ID, Ready: true, RestartCount: 2, LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", Message: "PRIVATE-RAW-MESSAGE"}}}}}}
	pod.Spec.NodeName = "scheduler-selected-node"
	pod.Spec.ServiceAccountName = "default"
	pod.Spec.DNSPolicy = corev1.DNSClusterFirst
	pod.Spec.SchedulerName = corev1.DefaultSchedulerName
	pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
	pod.Spec.TerminationGracePeriodSeconds = pointer(int64(30))
	pod.Spec.EnableServiceLinks = pointer(true)
	pod.Spec.PreemptionPolicy = pointer(corev1.PreemptLowerPriority)
	pod.Spec.Priority = pointer(int32(0))
	pod.Spec.SecurityContext = &corev1.PodSecurityContext{}
	pod.Spec.Containers[0].TerminationMessagePath = "/dev/termination-log"
	pod.Spec.Containers[0].TerminationMessagePolicy = corev1.TerminationMessageReadFile
	if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
		t.Fatal("compiler must retain explicit token automount refusal")
	}

	pod.Spec.Tolerations = []corev1.Toleration{{Key: "node.kubernetes.io/not-ready", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute, TolerationSeconds: pointer(int64(300))}}
	if err = client.Tracker().Add(pod); err != nil {
		t.Fatal(err)
	}
	foreign := pod.DeepCopy()
	foreign.Name = "foreign"
	foreign.UID = "foreign-pod"
	foreign.OwnerReferences[0].UID = types.UID("foreign-rs")
	if err = client.Tracker().Add(foreign); err != nil {
		t.Fatal(err)
	}
	target, err := c.target(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	observed := c.observeWorkload(context.Background(), target, request, service, deployment, &workloadObservationBudget{})
	if observed.State != "ready" || observed.ObservedPods != 1 || observed.RestartCount != 2 || !observed.OOMObserved || observed.ModelContentsVerified {
		t.Fatalf("exact owned runtime not observed: %+v", observed)
	}
	for _, code := range observed.Reasons {
		if strings.Contains(code, "PRIVATE") {
			t.Fatal("raw message leaked")
		}
	}
	pod.Spec.Containers[0].Args = []string{"admission-mutated"}
	if err = client.Tracker().Update(corev1.SchemeGroupVersion.WithResource("pods"), pod, "apps"); err != nil {
		t.Fatal(err)
	}
	observed = c.observeWorkload(context.Background(), target, request, service, deployment, &workloadObservationBudget{})
	if observed.State != "unknown" || !observed.Omitted || observed.ReadyPods != 0 {
		t.Fatalf("mutated workload certified: %+v", observed)
	}
}

func TestInferencePodIdentityChecksReviewedProbesReferencesAndResources(t *testing.T) {
	base := corev1.PodSpec{AutomountServiceAccountToken: pointer(false), Containers: []corev1.Container{{Name: "model", Image: "registry/model@sha256:" + strings.Repeat("a", 64), Args: []string{"--model", "/models"}, Env: []corev1.EnvVar{{Name: "TOKEN", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "approved"}, Key: "token"}}}}, ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{}}, PeriodSeconds: 5}}}, Volumes: []corev1.Volume{{Name: "model", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "approved-model", ReadOnly: true}}}}}
	if !podMatchesTemplate(base, base) {
		t.Fatal("identical reviewed spec refused")
	}
	for _, mutate := range []func(*corev1.PodSpec){func(p *corev1.PodSpec) { p.Containers[0].Env[0].ValueFrom.SecretKeyRef.Name = "other" }, func(p *corev1.PodSpec) { p.Containers[0].ReadinessProbe.PeriodSeconds = 20 }, func(p *corev1.PodSpec) { p.Volumes[0].PersistentVolumeClaim.ClaimName = "other" }, func(p *corev1.PodSpec) { p.Containers[0].Command = []string{"other"} }, func(p *corev1.PodSpec) { p.AutomountServiceAccountToken = pointer(true) }} {
		changed := base.DeepCopy()
		mutate(changed)
		if podMatchesTemplate(base, *changed) {
			t.Fatal("unreviewed workload mutation accepted")
		}
	}
}

func TestInferenceObservationPaginationIsUnknownNotZeroHealthy(t *testing.T) {
	c, client, _, request := workloadFixture(t)
	target, err := c.target(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	client.PrependReactor("list", "replicasets", func(ktesting.Action) (bool, runtime.Object, error) {
		return true, &appsv1.ReplicaSetList{ListMeta: metav1.ListMeta{Continue: "more"}}, nil
	})
	observed := c.observeWorkload(context.Background(), target, request, request.Blueprint.Services[0], &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{UID: "original"}, Spec: appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "one"}}}}, &workloadObservationBudget{})
	if observed.State != "unknown" || !observed.Omitted || observed.ReadyPods != 0 {
		t.Fatal("omitted runtime certified")
	}
}
