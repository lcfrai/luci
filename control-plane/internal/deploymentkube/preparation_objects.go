package deploymentkube

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
)

// API defaults are the same closed semantic equivalents used by the existing
// Deployment connector. NodeName is an observed scheduler assignment; this does
// not certify placement, capacity, image contents or model quality.
func PreparationPodSpecMatches(want, observed corev1.PodSpec) bool {
	a, b := want.DeepCopy(), observed.DeepCopy()
	if b.NodeName != "" && len(kubevalidation.IsDNS1123Subdomain(b.NodeName)) != 0 {
		return false
	}
	if a.NodeName == "" {
		a.NodeName = b.NodeName
	}
	normalizePod(a)
	normalizePod(b)
	return equality.Semantic.DeepEqual(a, b)
}
func PreparationPVCSpecMatches(want, observed corev1.PersistentVolumeClaimSpec) bool {
	a, b := want.DeepCopy(), observed.DeepCopy()
	// Dynamic binding may fill the PV name; every other requested storage/access
	// setting stays exact. Current original PVC UID is separately mandatory.
	if a.VolumeName == "" {
		a.VolumeName = b.VolumeName
	}
	return equality.Semantic.DeepEqual(a, b)
}
