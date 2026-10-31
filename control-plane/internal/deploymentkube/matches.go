package deploymentkube

import (
	"bytes"
	"encoding/json"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func pointer[T any](value T) *T { return &value }

// Normalize a closed set of absent/default semantic equivalents. Some defaults
// are materialized on Pods by admission, not on Deployment templates (notably
// the default service account and priority). This is a comparison rule, NOT a
// certification of any particular API server's defaulting/admission behavior.
// Compare the entire resulting spec, not annotations or a permissive subset.
// Admission-injected sidecars,
// placement, identity, security, DNS or runtime settings require another reviewed
// compiler capability; they are not silently certified as the planned workload.
func normalizePod(spec *corev1.PodSpec) {
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = corev1.RestartPolicyAlways
	}
	if spec.DNSPolicy == "" {
		spec.DNSPolicy = corev1.DNSClusterFirst
	}
	if spec.SchedulerName == "" {
		spec.SchedulerName = corev1.DefaultSchedulerName
	}
	if spec.TerminationGracePeriodSeconds == nil {
		spec.TerminationGracePeriodSeconds = pointer(int64(30))
	}
	if spec.EnableServiceLinks == nil {
		spec.EnableServiceLinks = pointer(true)
	}
	if spec.PreemptionPolicy == nil {
		spec.PreemptionPolicy = pointer(corev1.PreemptLowerPriority)
	}
	if spec.ShareProcessNamespace == nil {
		spec.ShareProcessNamespace = pointer(false)
	}
	if spec.SetHostnameAsFQDN == nil {
		spec.SetHostnameAsFQDN = pointer(false)
	}
	if spec.HostUsers == nil {
		spec.HostUsers = pointer(true)
	}
	if spec.Priority == nil {
		spec.Priority = pointer(int32(0))
	}
	if spec.SecurityContext == nil {
		spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	if spec.DNSConfig == nil {
		spec.DNSConfig = &corev1.PodDNSConfig{}
	}
	if spec.Affinity == nil {
		spec.Affinity = &corev1.Affinity{}
	}
	if spec.ServiceAccountName == "" {
		spec.ServiceAccountName = "default"
	}
	if spec.DeprecatedServiceAccount == "" {
		spec.DeprecatedServiceAccount = spec.ServiceAccountName
	}
	for index := range spec.Containers {
		container := &spec.Containers[index]
		if container.TerminationMessagePath == "" {
			container.TerminationMessagePath = "/dev/termination-log"
		}
		if container.TerminationMessagePolicy == "" {
			container.TerminationMessagePolicy = corev1.TerminationMessageReadFile
		}
		for port := range container.Ports {
			if container.Ports[port].Protocol == "" {
				container.Ports[port].Protocol = corev1.ProtocolTCP
			}
		}
	}
}

func normalizeDeployment(spec *appsv1.DeploymentSpec) {
	normalizePod(&spec.Template.Spec)
	if spec.RevisionHistoryLimit == nil {
		spec.RevisionHistoryLimit = pointer(int32(10))
	}
	if spec.ProgressDeadlineSeconds == nil {
		spec.ProgressDeadlineSeconds = pointer(int32(600))
	}
	if spec.Strategy.Type == "" {
		spec.Strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
	}
	if spec.Strategy.Type == appsv1.RollingUpdateDeploymentStrategyType {
		if spec.Strategy.RollingUpdate == nil {
			spec.Strategy.RollingUpdate = &appsv1.RollingUpdateDeployment{}
		}
		if spec.Strategy.RollingUpdate.MaxSurge == nil {
			spec.Strategy.RollingUpdate.MaxSurge = pointer(intstr.FromString("25%"))
		}
		if spec.Strategy.RollingUpdate.MaxUnavailable == nil {
			spec.Strategy.RollingUpdate.MaxUnavailable = pointer(intstr.FromString("25%"))
		}
	}
}

func jsonEqual(a, b any) bool {
	left, e1 := json.Marshal(a)
	right, e2 := json.Marshal(b)
	return e1 == nil && e2 == nil && bytes.Equal(left, right)
}

func deploymentMatches(expected, actual *appsv1.Deployment) bool {
	if expected == nil || actual == nil || len(expected.Spec.Template.Spec.Containers) != 1 || len(actual.Spec.Template.Spec.Containers) != 1 {
		return false
	}
	a, b := expected.DeepCopy().Spec, actual.DeepCopy().Spec
	// Quantity syntax may differ while representing exactly the same requested
	// resources. Compare its typed semantics, then use the same representation.
	if !equality.Semantic.DeepEqual(a.Template.Spec.Containers[0].Resources, b.Template.Spec.Containers[0].Resources) {
		return false
	}
	b.Template.Spec.Containers[0].Resources = a.Template.Spec.Containers[0].Resources
	normalizeDeployment(&a)
	normalizeDeployment(&b)
	return jsonEqual(a, b)
}

func serviceMatches(expected, actual *corev1.Service) bool {
	if expected == nil || actual == nil || actual.Spec.Type != corev1.ServiceTypeClusterIP {
		return false
	}
	a, b := expected.DeepCopy().Spec, actual.DeepCopy().Spec
	// These are assigned network identities, not deployment input or public
	// exposure. All other fields (including externalIPs) must match the plan.
	a.ClusterIP, a.ClusterIPs, a.IPFamilies, a.IPFamilyPolicy = b.ClusterIP, b.ClusterIPs, b.IPFamilies, b.IPFamilyPolicy
	for _, spec := range []*corev1.ServiceSpec{&a, &b} {
		if spec.SessionAffinity == "" {
			spec.SessionAffinity = corev1.ServiceAffinityNone
		}
		if spec.InternalTrafficPolicy == nil {
			spec.InternalTrafficPolicy = pointer(corev1.ServiceInternalTrafficPolicyCluster)
		}
		for index := range spec.Ports {
			if spec.Ports[index].Protocol == "" {
				spec.Ports[index].Protocol = corev1.ProtocolTCP
			}
		}
	}
	return jsonEqual(a, b)
}
