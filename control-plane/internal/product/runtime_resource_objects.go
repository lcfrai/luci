package product

import (
	"fmt"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func runtimePointer[T any](v T) *T { return &v }
func runtimeResourceMeta(row storedRuntimePreparation, name, digest string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: row.Plan.Resources.Namespace, Labels: map[string]string{"app.kubernetes.io/managed-by": "luci-runtime-preparation", "luci.ai/runtime-project": reviewDigest(row.Plan.ProjectID)[:12]}, Annotations: map[string]string{"luci.ai/runtime-preparation": row.Plan.ID, "luci.ai/runtime-desired": digest}}
}
func runtimeResourceOwned(row storedRuntimePreparation, meta metav1.Object, name, digest, uid string) bool {
	return meta.GetDeletionTimestamp() == nil && runtimeResourceStampMatches(row, meta, name, digest, uid)
}
func runtimeResourceStampMatches(row storedRuntimePreparation, meta metav1.Object, name, digest, uid string) bool {
	return meta.GetName() == name && meta.GetNamespace() == row.Plan.Resources.Namespace && len(meta.GetOwnerReferences()) == 0 && meta.GetLabels()["app.kubernetes.io/managed-by"] == "luci-runtime-preparation" && meta.GetLabels()["luci.ai/runtime-project"] == reviewDigest(row.Plan.ProjectID)[:12] && meta.GetAnnotations()["luci.ai/runtime-preparation"] == row.Plan.ID && meta.GetAnnotations()["luci.ai/runtime-desired"] == digest && (uid == "" || string(meta.GetUID()) == uid)
}
func runtimeVolumeDigest(volume RuntimeVolumePlan) string { return reviewDigest(volume) }
func runtimeDesiredPVC(row storedRuntimePreparation, volume RuntimeVolumePlan) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolumeClaim"}, ObjectMeta: runtimeResourceMeta(row, volume.ClaimName, runtimeVolumeDigest(volume)), Spec: corev1.PersistentVolumeClaimSpec{AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, StorageClassName: runtimePointer(volume.StorageClass), VolumeMode: runtimePointer(corev1.PersistentVolumeFilesystem), Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", volume.SizeGiB))}}}}
}
func runtimeSecretDigest(plan RuntimeSecretPlan) string { return reviewDigest(plan) }
func runtimeDesiredSecret(row storedRuntimePreparation, plan RuntimeSecretPlan) (*corev1.Secret, error) {
	private := runtimePrivateSecretFor(row, plan.Name)
	if private == nil || private.GenerationID != plan.GenerationID {
		return nil, ErrRuntimePreparation
	}
	values := map[string][]byte{}
	for key, value := range private.Data {
		values[key] = append([]byte{}, value...)
	}
	return &corev1.Secret{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}, ObjectMeta: runtimeResourceMeta(row, plan.Name, runtimeSecretDigest(plan)), Immutable: runtimePointer(true), Type: corev1.SecretType(private.Type), Data: values}, nil
}
func runtimeTransferPodDigest(row storedRuntimePreparation, service string, image string) string {
	return reviewDigest(struct{ Preparation, Service, Profile, Image, Claim, Path, NamespaceUID string }{row.Plan.ID, service, runtimeModelSinkProfile(), image, runtimeClaimName(row.Plan.ID, service), row.Plan.Resources.ModelsRelativePaths[service], row.Plan.Resources.NamespaceUID})
}
func runtimeDesiredTransferPod(row storedRuntimePreparation, model RuntimeModelSource, pvcUID, image string) (*corev1.Pod, error) {
	if pvcUID == "" || !kubeImmutableImage.MatchString(image) {
		return nil, ErrRuntimePreparation
	}
	modelState := runtimeModelTransfer(row.Resources, model.ServiceID)
	if modelState == nil {
		return nil, ErrRuntimePreparation
	}
	environment := []corev1.EnvVar{{Name: "LUCI_TRANSFER_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: runtimeTransferSecret(row.Plan.ID, model.ServiceID)}, Key: "transfer.key", Optional: runtimePointer(false)}}}}
	for _, pair := range [][2]string{{"LUCI_TRANSFER_BYTES", fmt.Sprint(model.Bytes)}, {"LUCI_TRANSFER_SHA256", model.SHA256}, {"LUCI_TRANSFER_PATH", row.Plan.Resources.ModelsRelativePaths[model.ServiceID]}, {"LUCI_PREPARATION_ID", row.Plan.ID}, {"LUCI_OPERATION_ID", modelState.OperationID}, {"LUCI_PVC_UID", pvcUID}} {
		environment = append(environment, corev1.EnvVar{Name: pair[0], Value: pair[1]})
	}
	spec := corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, AutomountServiceAccountToken: runtimePointer(false), EnableServiceLinks: runtimePointer(false), ActiveDeadlineSeconds: runtimePointer(int64(1200)), TerminationGracePeriodSeconds: runtimePointer(int64(10)), ServiceAccountName: "default", NodeSelector: map[string]string{"kubernetes.io/os": "linux", "kubernetes.io/arch": "amd64"}, SecurityContext: &corev1.PodSecurityContext{RunAsUser: runtimePointer(int64(1000)), RunAsGroup: runtimePointer(int64(1000)), RunAsNonRoot: runtimePointer(true), FSGroup: runtimePointer(int64(1000)), FSGroupChangePolicy: runtimePointer(corev1.FSGroupChangeOnRootMismatch), SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}, Volumes: []corev1.Volume{{Name: "model", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: runtimeClaimName(row.Plan.ID, model.ServiceID)}}}}, Containers: []corev1.Container{{Name: "model-sink", Image: image, ImagePullPolicy: corev1.PullIfNotPresent, Command: []string{"/luci-model-sink"}, Env: environment, Ports: []corev1.ContainerPort{{Name: "upload", ContainerPort: 9090, Protocol: corev1.ProtocolTCP}}, Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("250m"), corev1.ResourceMemory: resource.MustParse("128Mi")}, Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("256Mi")}}, SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: runtimePointer(false), ReadOnlyRootFilesystem: runtimePointer(true), Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}, VolumeMounts: []corev1.VolumeMount{{Name: "model", MountPath: "/model"}}, ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/readyz", Port: intstr.FromInt32(9090), Scheme: corev1.URISchemeHTTP}}, InitialDelaySeconds: 1, PeriodSeconds: 2, TimeoutSeconds: 1, SuccessThreshold: 1, FailureThreshold: 60}, TerminationMessagePath: "/dev/termination-log", TerminationMessagePolicy: corev1.TerminationMessageReadFile}}}
	// Explicit standard unreachable/not-ready eviction defaults; injected sidecars,
	// volumes, identities, security or other placement changes remain unsupported.
	for _, key := range []string{"node.kubernetes.io/not-ready", "node.kubernetes.io/unreachable"} {
		spec.Tolerations = append(spec.Tolerations, corev1.Toleration{Key: key, Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute, TolerationSeconds: runtimePointer(int64(300))})
	}
	if row.Plan.Resources.ImagePullSecret != "" {
		spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: row.Plan.Resources.ImagePullSecret}}
	}
	return &corev1.Pod{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}, ObjectMeta: runtimeResourceMeta(row, modelState.PodName, runtimeTransferPodDigest(row, model.ServiceID, image)), Spec: spec}, nil
}
func runtimeReceiptMatches(value deploymentkube.ModelSinkReceipt, row storedRuntimePreparation, model RuntimeModelSource, state RuntimeModelTransfer) bool {
	return value.Protocol == 1 && value.PreparationID == row.Plan.ID && value.OperationID == state.OperationID && value.PVCUID == state.PVCUID && value.Bytes == model.Bytes && value.SHA256 == model.SHA256 && value.Path == row.Plan.Resources.ModelsRelativePaths[model.ServiceID] && value.State == "stored-reported"
}
