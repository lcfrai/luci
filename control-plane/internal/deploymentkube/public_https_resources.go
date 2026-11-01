package deploymentkube

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	kubevalidation "k8s.io/apimachinery/pkg/util/validation"
)

const PublicHTTPSProfile = "traefik-native-acme-azuredns-file-v1"
const PublicHTTPSImageDefault = "ghcr.io/traefik/traefik@sha256:6d3e069ee484ab1da5adfdda555a75eb9b1545ef09096a0cc7c3043101986a20"

// Default source: official traefik package metadata, v3.7.16. No image is
// fetched by compiling/generating a plan; this is not platform/runtime proof.
var publicHTTPSImage = regexp.MustCompile(`^(ghcr\.io/traefik/traefik|docker\.io/library/traefik)@sha256:[a-f0-9]{64}$`)

type PublicHTTPSResourceSpec struct {
	ID             string `json:"id"`
	ProjectID      string `json:"projectId"`
	Namespace      string `json:"namespace"`
	ServiceName    string `json:"serviceName"`
	ServiceUID     string `json:"serviceUid"`
	Hostname       string `json:"hostname"`
	Path           string `json:"path"`
	PathType       string `json:"pathType"`
	ServicePort    int    `json:"servicePort"`
	Image          string `json:"image"`
	StorageClass   string `json:"storageClass"`
	Email          string `json:"email"`
	SubscriptionID string `json:"subscriptionId"`
	ResourceGroup  string `json:"resourceGroup"`
	ZoneName       string `json:"zoneName"`
	Replicas       int32  `json:"replicas"`
}
type PublicHTTPSObject struct {
	Kind, Name string
	Object     runtime.Object
}

func PublicHTTPSName(projectID, id string) string {
	return deploymentplan.ResourceName(projectID, "https-"+digest(id)[:12])
}
func PublicHTTPSObjectDigest(spec PublicHTTPSResourceSpec) string {
	spec.Replicas = 1
	return digest(spec)
}
func PublicHTTPSLabels(spec PublicHTTPSResourceSpec) map[string]string {
	return map[string]string{"app.kubernetes.io/managed-by": "luci-public-https", "luci.ai/project": projectLabel(spec.ProjectID), "luci.ai/https": digest(spec.ID)[:24]}
}
func PublicHTTPSOwned(spec PublicHTTPSResourceSpec, meta metav1.Object, uid string) bool {
	if meta == nil || uid == "" || string(meta.GetUID()) != uid || meta.GetName() != PublicHTTPSName(spec.ProjectID, spec.ID) || meta.GetNamespace() != spec.Namespace || meta.GetDeletionTimestamp() != nil || len(meta.GetOwnerReferences()) != 0 || meta.GetAnnotations()["luci.ai/https-spec"] != PublicHTTPSObjectDigest(spec) {
		return false
	}
	for k, v := range PublicHTTPSLabels(spec) {
		if meta.GetLabels()[k] != v {
			return false
		}
	}
	return true
}
func PublicHTTPSObjects(spec PublicHTTPSResourceSpec, credential map[string][]byte) ([]PublicHTTPSObject, error) {
	if !token.MatchString(spec.ID) || !token.MatchString(spec.ProjectID) || len(kubevalidation.IsDNS1123Label(spec.Namespace)) != 0 || !token.MatchString(spec.ServiceName) || spec.ServiceUID == "" || !deploymentplan.ValidPublicHostname(spec.Hostname) || spec.ServicePort < 1 || spec.ServicePort > 65535 || spec.Replicas < 0 || spec.Replicas > 1 || !publicHTTPSImage.MatchString(spec.Image) || !(strings.HasPrefix(spec.Image, "ghcr.io/traefik/traefik@sha256:") || strings.HasPrefix(spec.Image, "docker.io/library/traefik@sha256:")) || len(kubevalidation.IsDNS1123Subdomain(spec.StorageClass)) != 0 || spec.Email == "" || !deploymentplan.ValidPublicHostname(spec.ZoneName) {
		return nil, ErrRefused
	}
	if spec.Path == "" || spec.Path[0] != '/' || len(spec.Path) > 256 || (spec.PathType != "Exact" && spec.PathType != "Prefix") || strings.ContainsAny(spec.Path, "\\%?#\r\n") {
		return nil, ErrRefused
	}
	for _, key := range []string{"tenant-id", "client-id", "client-secret"} {
		if len(credential[key]) == 0 || len(credential[key]) > 4096 {
			return nil, ErrRefused
		}
	}
	if len(credential) != 3 {
		return nil, ErrRefused
	}
	name := PublicHTTPSName(spec.ProjectID, spec.ID)
	labels := PublicHTTPSLabels(spec)
	meta := func() metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: name, Namespace: spec.Namespace, Labels: labels, Annotations: map[string]string{"luci.ai/https-spec": PublicHTTPSObjectDigest(spec)}}
	}
	rule := "Host(" + strconv.Quote(spec.Hostname) + ") && "
	if spec.PathType == "Exact" {
		rule += "Path(" + strconv.Quote(spec.Path) + ")"
	} else if spec.Path == "/" {
		rule += "PathPrefix(" + strconv.Quote("/") + ")"
	} else {
		rule += "(Path(" + strconv.Quote(spec.Path) + ") || PathPrefix(" + strconv.Quote(strings.TrimSuffix(spec.Path, "/")+"/") + "))"
	}
	config := map[string]any{"http": map[string]any{"routers": map[string]any{"project": map[string]any{"rule": rule, "entryPoints": []string{"websecure"}, "service": "project", "tls": map[string]string{"certResolver": "luci"}}}, "services": map[string]any{"project": map[string]any{"loadBalancer": map[string]any{"servers": []map[string]string{{"url": fmt.Sprintf("http://%s.%s.svc:%d", spec.ServiceName, spec.Namespace, spec.ServicePort)}}}}}}, "tls": map[string]any{"options": map[string]any{"default": map[string]any{"minVersion": "VersionTLS12", "sniStrict": true}}}}
	raw, e := json.Marshal(config)
	if e != nil || len(raw) > 16<<10 {
		return nil, ErrRefused
	}
	cm := &corev1.ConfigMap{ObjectMeta: meta(), Immutable: pointer(true), Data: map[string]string{"route.json": string(raw)}}
	secret := &corev1.Secret{ObjectMeta: meta(), Immutable: pointer(true), Type: corev1.SecretTypeOpaque, Data: credential}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: meta(), Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: pointer(spec.StorageClass), AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeMode: pointer(corev1.PersistentVolumeFilesystem), Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}}}
	env := []corev1.EnvVar{{Name: "AZURE_AUTH_METHOD", Value: "env"}, {Name: "AZURE_ENVIRONMENT", Value: "public"}, {Name: "AZURE_PRIVATE_ZONE", Value: "false"}, {Name: "AZURE_TENANT_ID_FILE", Value: "/dns/tenant-id"}, {Name: "AZURE_CLIENT_ID_FILE", Value: "/dns/client-id"}, {Name: "AZURE_CLIENT_SECRET_FILE", Value: "/dns/client-secret"}, {Name: "AZURE_SUBSCRIPTION_ID", Value: spec.SubscriptionID}, {Name: "AZURE_RESOURCE_GROUP", Value: spec.ResourceGroup}, {Name: "AZURE_ZONE_NAME", Value: spec.ZoneName}, {Name: "AZURE_TTL", Value: "60"}, {Name: "AZURE_PROPAGATION_TIMEOUT", Value: "120"}, {Name: "LEGO_DISABLE_CNAME_SUPPORT", Value: "true"}, {Name: "HOME", Value: "/data"}}
	args := []string{"--global.checknewversion=false", "--global.sendanonymoususage=false", "--api=false", "--log.level=ERROR", "--entrypoints.websecure.address=:8443", "--providers.file.filename=/config/route.json", "--providers.file.watch=false", "--certificatesresolvers.luci.acme.email=" + spec.Email, "--certificatesresolvers.luci.acme.storage=/data/acme.json", "--certificatesresolvers.luci.acme.caserver=https://acme-v02.api.letsencrypt.org/directory", "--certificatesresolvers.luci.acme.dnschallenge.provider=azuredns", "--certificatesresolvers.luci.acme.clienttimeout=120s", "--certificatesresolvers.luci.acme.clientresponseheadertimeout=30s"}
	pod := corev1.PodSpec{AutomountServiceAccountToken: pointer(false), EnableServiceLinks: pointer(false), NodeSelector: map[string]string{"kubernetes.io/os": "linux", "kubernetes.io/arch": "amd64"}, SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: pointer(true), RunAsUser: pointer(int64(65532)), RunAsGroup: pointer(int64(65532)), FSGroup: pointer(int64(65532)), FSGroupChangePolicy: pointer(corev1.FSGroupChangeOnRootMismatch), SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}}, Containers: []corev1.Container{{Name: "https", Image: spec.Image, ImagePullPolicy: corev1.PullIfNotPresent, Args: args, Env: env, Ports: []corev1.ContainerPort{{Name: "websecure", ContainerPort: 8443, Protocol: corev1.ProtocolTCP}}, SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: pointer(false), ReadOnlyRootFilesystem: pointer(true), Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}, Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("128Mi")}, Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("512Mi")}}, VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/config", ReadOnly: true}, {Name: "dns", MountPath: "/dns", ReadOnly: true}, {Name: "certificates", MountPath: "/data"}, {Name: "tmp", MountPath: "/tmp"}}, ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(8443)}}, InitialDelaySeconds: 2, PeriodSeconds: 10, TimeoutSeconds: 2}}}, Volumes: []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: name}, DefaultMode: pointer(int32(0444))}}}, {Name: "dns", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: name, DefaultMode: pointer(int32(0440))}}}, {Name: "certificates", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name}}}, {Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: pointer(resource.MustParse("16Mi"))}}}}}
	deployment := &appsv1.Deployment{ObjectMeta: meta(), Spec: appsv1.DeploymentSpec{Replicas: pointer(spec.Replicas), RevisionHistoryLimit: pointer(int32(1)), Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}, Selector: &metav1.LabelSelector{MatchLabels: labels}, Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels, Annotations: meta().Annotations}, Spec: pod}}}
	service := &corev1.Service{ObjectMeta: meta(), Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, Selector: labels, AllocateLoadBalancerNodePorts: pointer(true), ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster, IPFamilies: []corev1.IPFamily{corev1.IPv4Protocol}, IPFamilyPolicy: pointer(corev1.IPFamilyPolicySingleStack), Ports: []corev1.ServicePort{{Name: "websecure", Port: 443, TargetPort: intstr.FromInt32(8443), Protocol: corev1.ProtocolTCP}}}}
	pvc.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolumeClaim"}
	secret.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}
	cm.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"}
	deployment.TypeMeta = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"}
	service.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Service"}
	return []PublicHTTPSObject{{"PersistentVolumeClaim", name, pvc}, {"Secret", name, secret}, {"ConfigMap", name, cm}, {"Deployment", name, deployment}, {"Service", name, service}}, nil
}
func PublicHTTPSMatches(expected, actual runtime.Object) bool {
	switch a := expected.(type) {
	case *appsv1.Deployment:
		b, ok := actual.(*appsv1.Deployment)
		return ok && deploymentMatches(a, b)
	case *corev1.PersistentVolumeClaim:
		b, ok := actual.(*corev1.PersistentVolumeClaim)
		return ok && PreparationPVCSpecMatches(a.Spec, b.Spec)
	case *corev1.Secret:
		b, ok := actual.(*corev1.Secret)
		return ok && b.Immutable != nil && *b.Immutable && b.Type == a.Type && reflect.DeepEqual(a.Data, b.Data)
	case *corev1.ConfigMap:
		b, ok := actual.(*corev1.ConfigMap)
		return ok && b.Immutable != nil && *b.Immutable && reflect.DeepEqual(a.Data, b.Data) && len(b.BinaryData) == 0
	case *corev1.Service:
		b, ok := actual.(*corev1.Service)
		if !ok || b.Spec.Type != corev1.ServiceTypeLoadBalancer || len(b.Spec.Ports) != 1 || b.Spec.Ports[0].NodePort < 1 || b.Spec.Ports[0].NodePort > 65535 {
			return false
		}
		x, y := a.DeepCopy().Spec, b.DeepCopy().Spec
		x.ClusterIP, x.ClusterIPs = y.ClusterIP, y.ClusterIPs
		x.Ports[0].NodePort = y.Ports[0].NodePort
		for _, v := range []*corev1.ServiceSpec{&x, &y} {
			if v.SessionAffinity == "" {
				v.SessionAffinity = corev1.ServiceAffinityNone
			}
			if v.InternalTrafficPolicy == nil {
				v.InternalTrafficPolicy = pointer(corev1.ServiceInternalTrafficPolicyCluster)
			}
		}
		return equality.Semantic.DeepEqual(x, y)
	}
	return false
}

// Probe ownership/UID checks and the caller's fresh authorization surround each
// use. Namespace checks cannot make an API Create atomic with namespace reuse.
func PublicHTTPSNamespace(ctx context.Context, target Target) error {
	if target.Client == nil || target.NamespaceUID == "" {
		return ErrRefused
	}
	n, e := target.Client.CoreV1().Namespaces().Get(ctx, target.Namespace, metav1.GetOptions{})
	if e != nil || n == nil || string(n.UID) != target.NamespaceUID || n.DeletionTimestamp != nil {
		return ErrRefused
	}
	return ctx.Err()
}
