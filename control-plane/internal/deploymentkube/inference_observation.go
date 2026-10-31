package deploymentkube

import (
	"context"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const workloadObservationObjectLimit = 128

type workloadObservationBudget struct{ objects int }

func controllerOwner(metadata metav1.Object, kind, uid string) bool {
	found := false
	for _, owner := range metadata.GetOwnerReferences() {
		if owner.Controller == nil || !*owner.Controller {
			continue
		}
		if found || owner.APIVersion != "apps/v1" || owner.Kind != kind || string(owner.UID) != uid {
			return false
		}
		found = true
	}
	return found
}

func currentReplicaSet(expected *appsv1.Deployment, actual *appsv1.ReplicaSet) bool {
	if actual == nil || actual.Namespace != expected.Namespace || actual.Name == "" || actual.DeletionTimestamp != nil || actual.UID == "" || !controllerOwner(actual, "Deployment", string(expected.UID)) {
		return false
	}
	a, b := expected.Spec.Template.DeepCopy(), actual.Spec.Template.DeepCopy()
	delete(b.Labels, "pod-template-hash")
	normalizePod(&a.Spec)
	normalizePod(&b.Spec)
	return jsonEqual(a.ObjectMeta, b.ObjectMeta) && equality.Semantic.DeepEqual(a.Spec, b.Spec)
}

// Compare the complete reviewed workload spec after the existing closed default
// normalization. NodeName is a scheduler result, not a planned node selector.
// The two standard finite node-unreachable/not-ready admission tolerations are
// accepted only when the template omitted them; arbitrary admission mutations
// (including sidecars, credentials, probes, resources or storage) stay unknown.
func podMatchesTemplate(template corev1.PodSpec, pod corev1.PodSpec) bool {
	a, b := template.DeepCopy(), pod.DeepCopy()
	if len(a.Containers) != 1 || len(b.Containers) != 1 {
		return false
	}
	if !equality.Semantic.DeepEqual(a.Containers[0].Resources, b.Containers[0].Resources) {
		return false
	}
	b.Containers[0].Resources = a.Containers[0].Resources
	b.NodeName = a.NodeName
	filtered := make([]corev1.Toleration, 0, len(b.Tolerations))
	allowedDefault := map[string]bool{}
	for _, row := range b.Tolerations {
		planned := false
		for _, want := range a.Tolerations {
			if equality.Semantic.DeepEqual(row, want) {
				planned = true
				break
			}
		}
		standard := !planned && (row.Key == "node.kubernetes.io/not-ready" || row.Key == "node.kubernetes.io/unreachable") && row.Operator == corev1.TolerationOpExists && row.Effect == corev1.TaintEffectNoExecute && row.Value == "" && row.TolerationSeconds != nil && *row.TolerationSeconds == 300
		if standard {
			if allowedDefault[row.Key] {
				return false
			}
			allowedDefault[row.Key] = true
			continue
		}
		filtered = append(filtered, row)
	}
	b.Tolerations = filtered
	normalizePod(a)
	normalizePod(b)
	return jsonEqual(a, b)
}

// Observe only the exact owned current-revision Deployment -> ReplicaSet ->
// Pod controller chain. Labels narrow the query; labels alone never prove
// ownership. Raw conditions/messages/events/logs are never retained or exposed.
func (c *Connector) observeWorkload(ctx context.Context, target Target, request Request, service deploymentplan.Service, deployment *appsv1.Deployment, budget *workloadObservationBudget) WorkloadObservation {
	result := WorkloadObservation{ComponentID: service.ID, DeploymentUID: string(deployment.UID), State: "unknown", ReadinessProbeConfigured: service.Health != nil && service.Health.Readiness != nil, Reasons: []string{}, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if deployment.UID == "" || budget == nil || budget.objects >= workloadObservationObjectLimit || c.authorize(ctx, request, false) != nil {
		result.Omitted = true
		return result
	}
	selector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels).String()
	limit := int64(workloadObservationObjectLimit - budget.objects)
	sets, err := target.Client.AppsV1().ReplicaSets(target.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector, Limit: limit})
	if err != nil || sets == nil || len(sets.Items) > int(limit) || c.authorize(ctx, request, false) != nil {
		result.Omitted = true
		return result
	}
	budget.objects += len(sets.Items)
	result.Omitted = sets.Continue != ""
	owned := map[string]*appsv1.ReplicaSet{}
	for index := range sets.Items {
		if currentReplicaSet(deployment, &sets.Items[index]) {
			owned[string(sets.Items[index].UID)] = &sets.Items[index]
		}
	}
	if len(owned) == 0 {
		if !result.Omitted {
			result.State = "pending"
			result.Reasons = []string{"replica-set-not-observed"}
		}
		return result
	}
	limit = int64(workloadObservationObjectLimit - budget.objects)
	if limit <= 0 {
		result.Omitted = true
		return result
	}
	pods, err := target.Client.CoreV1().Pods(target.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector, Limit: limit})
	if err != nil || pods == nil || len(pods.Items) > int(limit) || c.authorize(ctx, request, false) != nil {
		result.Omitted = true
		return result
	}
	budget.objects += len(pods.Items)
	result.Omitted = result.Omitted || pods.Continue != ""
	reasons := map[string]bool{}
	for index := range pods.Items {
		pod := &pods.Items[index]
		if pod.DeletionTimestamp != nil {
			continue
		}
		if pod.Namespace != target.Namespace || pod.Name == "" || len(pod.UID) > 128 {
			result.Omitted = true
			continue
		}
		var owner *appsv1.ReplicaSet
		for uid, set := range owned {
			if controllerOwner(pod, "ReplicaSet", uid) {
				owner = set
				break
			}
		}
		if owner == nil {
			continue
		}
		result.ObservedPods++
		if pod.UID == "" || len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Name != service.ID || pod.Spec.Containers[0].Image != service.Image || !podMatchesTemplate(owner.Spec.Template.Spec, pod.Spec) {
			result.Omitted = true
			reasons["runtime-identity-unconfirmed"] = true
			continue
		}
		ready := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				ready = true
			}
			if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
				reasons["scheduling-pending"] = true
			}
		}
		if len(pod.Status.ContainerStatuses) != 1 || pod.Status.ContainerStatuses[0].Name != service.ID {
			ready = false
			reasons["container-status-unavailable"] = true
		} else {
			status := pod.Status.ContainerStatuses[0]
			if status.RestartCount < 0 {
				result.Omitted = true
				ready = false
			} else {
				result.RestartCount += int64(status.RestartCount)
			}
			if !status.Ready {
				ready = false
			}
			for _, terminated := range []*corev1.ContainerStateTerminated{status.State.Terminated, status.LastTerminationState.Terminated} {
				if terminated != nil && terminated.Reason == "OOMKilled" {
					result.OOMObserved = true
					reasons["oom-observed"] = true
				}
			}
			if waiting := status.State.Waiting; waiting != nil {
				switch waiting.Reason {
				case "ImagePullBackOff", "ErrImagePull":
					reasons["image-pull-pending"] = true
				case "CrashLoopBackOff":
					reasons["restart-backoff"] = true
				case "CreateContainerConfigError":
					reasons["configuration-unavailable"] = true
				default:
					reasons["container-waiting"] = true
				}
			}
		}
		if ready {
			result.ReadyPods++
		}
	}
	for code := range reasons {
		result.Reasons = append(result.Reasons, code)
	}
	sort.Strings(result.Reasons)
	if result.Omitted || ctx.Err() != nil || c.authorize(ctx, request, false) != nil {
		result.State = "unknown"
		return result
	}
	result.State = "pending"
	if !result.ReadinessProbeConfigured {
		result.State = "unprobed"
	} else if result.ObservedPods == service.Replicas && result.ReadyPods == service.Replicas && deployment.Status.ObservedGeneration >= deployment.Generation {
		result.State = "ready"
	}
	return result
}

func validWorkloadObservation(row WorkloadObservation) bool {
	if row.ComponentID == "" || len(row.ComponentID) > 63 || row.DeploymentUID == "" || len(row.DeploymentUID) > 128 || row.ModelContentsVerified || row.ObservedPods < 0 || row.ObservedPods > 128 || row.ReadyPods < 0 || row.ReadyPods > row.ObservedPods || row.RestartCount < 0 || row.RestartCount > 128*(1<<31) || len(row.Reasons) > 9 {
		return false
	}
	if row.State != "ready" && row.State != "pending" && row.State != "unknown" && row.State != "unprobed" {
		return false
	}
	if row.State == "ready" && (!row.ReadinessProbeConfigured || row.Omitted) {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, row.ObservedAt); err != nil {
		return false
	}
	seen := map[string]bool{}
	for _, code := range row.Reasons {
		if seen[code] {
			return false
		}
		seen[code] = true
		switch code {
		case "replica-set-not-observed", "runtime-identity-unconfirmed", "scheduling-pending", "container-status-unavailable", "oom-observed", "image-pull-pending", "restart-backoff", "configuration-unavailable", "container-waiting":
		default:
			return false
		}
	}
	return true
}
