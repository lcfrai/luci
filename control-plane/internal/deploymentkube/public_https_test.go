package deploymentkube

import (
	"encoding/json"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"strings"
	"testing"
)

func publicHTTPSFixtureSpec() PublicHTTPSResourceSpec {
	return PublicHTTPSResourceSpec{ID: "https-" + strings.Repeat("a", 32), ProjectID: "project-one", Namespace: "apps", ServiceName: "project-app", ServiceUID: "app-original", Hostname: "app.example.com", Path: "/", PathType: "Prefix", ServicePort: 8080, Image: PublicHTTPSImageDefault, StorageClass: "managed-csi", Email: "owner@example.com", SubscriptionID: "11111111-1111-1111-1111-111111111111", ResourceGroup: "dns", ZoneName: "example.com", Replicas: 1}
}
func publicHTTPSFixtureObjects(t *testing.T) []PublicHTTPSObject {
	t.Helper()
	objects, e := PublicHTTPSObjects(publicHTTPSFixtureSpec(), map[string][]byte{"tenant-id": []byte("tenant"), "client-id": []byte("client"), "client-secret": []byte("private-value")})
	if e != nil {
		t.Fatal(e)
	}
	return objects
}
func TestPublicHTTPSProfileOwnsOnlyFiveClosedResources(t *testing.T) {
	objects := publicHTTPSFixtureObjects(t)
	if len(objects) != 5 {
		t.Fatal(len(objects))
	}
	spec := publicHTTPSFixtureSpec()
	for _, row := range objects {
		if row.Name != PublicHTTPSName(spec.ProjectID, spec.ID) || row.Object.GetObjectKind().GroupVersionKind().Kind != row.Kind {
			t.Fatal("missing typed API object", row.Kind)
		}
		meta := row.Object.(metav1.Object)
		meta.SetUID(types.UID("original"))
		originalName := meta.GetName()
		meta.SetName("different-original")
		if PublicHTTPSOwned(spec, meta, "original") {
			t.Fatal("wrong metadata name adopted")
		}
		meta.SetName(originalName)
		if !PublicHTTPSOwned(spec, meta, "original") || PublicHTTPSOwned(spec, meta, "") || PublicHTTPSOwned(spec, meta, "replacement") {
			t.Fatal("missing UID adopted", row.Kind)
		}
	}
	deployment := objects[3].Object.(*appsv1.Deployment)
	pod := deployment.Spec.Template.Spec
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken || *pod.SecurityContext.RunAsUser != 65532 || *pod.SecurityContext.FSGroup != 65532 || pod.ServiceAccountName != "" || len(pod.Containers) != 1 || !(*pod.Containers[0].SecurityContext.ReadOnlyRootFilesystem) || len(pod.Containers[0].Command) != 0 {
		t.Fatal("private non-root official-entrypoint profile changed")
	}
	args := strings.Join(pod.Containers[0].Args, " ")
	if strings.Contains(args, "kubernetes") || !strings.Contains(args, "--api=false") || !strings.Contains(args, "/data/acme.json") || !strings.Contains(args, "dnschallenge.provider=azuredns") {
		t.Fatal(args)
	}
	secret := objects[1].Object.(*corev1.Secret)
	if secret.Immutable == nil || !*secret.Immutable || len(secret.Data) != 3 {
		t.Fatal("private immutable credential")
	}
	if *deployment.Spec.Replicas != 1 || deployment.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType {
		t.Fatal("certificate writer must remain single")
	}
}
func TestPublicHTTPSFileRouterEscapesExactPathAndHasNoDashboard(t *testing.T) {
	objects := publicHTTPSFixtureObjects(t)
	cm := objects[2].Object.(*corev1.ConfigMap)
	var config map[string]any
	if json.Unmarshal([]byte(cm.Data["route.json"]), &config) != nil {
		t.Fatal("invalid file config")
	}
	http := config["http"].(map[string]any)
	router := http["routers"].(map[string]any)["project"].(map[string]any)
	if router["rule"] != `Host("app.example.com") && PathPrefix("/")` {
		t.Fatal(router)
	}
	if strings.Contains(cm.Data["route.json"], "private-value") || strings.Contains(cm.Data["route.json"], "api@internal") {
		t.Fatal("private/controller surface escaped")
	}
	for _, mutate := range []func(*PublicHTTPSResourceSpec){func(s *PublicHTTPSResourceSpec) { s.Image = "traefik:latest" }, func(s *PublicHTTPSResourceSpec) { s.Path = "/safe?other" }, func(s *PublicHTTPSResourceSpec) { s.Replicas = 2 }} {
		s := publicHTTPSFixtureSpec()
		mutate(&s)
		if _, e := PublicHTTPSObjects(s, map[string][]byte{"tenant-id": []byte("t"), "client-id": []byte("c"), "client-secret": []byte("s")}); e == nil {
			t.Fatal("unreviewed profile accepted", s)
		}
	}
}
func TestPublicHTTPSObservedSpecsRejectInjectedWorkAndWrongOriginal(t *testing.T) {
	objects := publicHTTPSFixtureObjects(t)
	want := objects[3].Object.(*appsv1.Deployment)
	got := want.DeepCopy()
	if !PublicHTTPSMatches(want, got) {
		t.Fatal("same desired")
	}
	got.Spec.Template.Spec.Containers = append(got.Spec.Template.Spec.Containers, corev1.Container{Name: "injected", Image: "other"})
	if PublicHTTPSMatches(want, got) {
		t.Fatal("injected sidecar")
	}
	svc := objects[4].Object.(*corev1.Service)
	actual := svc.DeepCopy()
	actual.Spec.ClusterIP = "10.0.0.1"
	actual.Spec.ClusterIPs = []string{"10.0.0.1"}
	actual.Spec.Ports[0].NodePort = 30443
	if !PublicHTTPSMatches(svc, actual) {
		t.Fatal("assigned LB fields should be allowed")
	}
	actual.Spec.ExternalIPs = []string{"8.8.8.8"}
	if PublicHTTPSMatches(svc, actual) {
		t.Fatal("unreviewed external address")
	}
}
