package deploymentplan

import (
	"encoding/json"
	networkingv1 "k8s.io/api/networking/v1"
	"reflect"
	"strings"
	"testing"
)

func TestPublicRouteCompilerUsesExactAppServicePortAndPreservesPrivateArtifacts(t *testing.T) {
	b := exampleBlueprint()
	private, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := json.Marshal(b)
	if err != nil || strings.Contains(string(legacy), "publicRoute") {
		t.Fatal("optional route changed legacy encoding", err)
	}
	b.PublicRoute = &PublicRoute{Profile: PublicRouteProfile, ServiceID: "web", IngressClassName: "traefik", Hostname: "app.example.com", Path: "/app", PathType: "Prefix", TLSSecretName: "site-tls"}
	plan, err := Compile(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Artifacts) != len(private.Artifacts)+1 {
		t.Fatal("route did not add exactly one owned resource")
	}
	var ingress networkingv1.Ingress
	if json.Unmarshal([]byte(plan.Artifacts[len(plan.Artifacts)-1].Content), &ingress) != nil {
		t.Fatal("invalid Ingress JSON")
	}
	path := ingress.Spec.Rules[0].HTTP.Paths[0]
	if ingress.APIVersion != "networking.k8s.io/v1" || *ingress.Spec.IngressClassName != "traefik" || ingress.Spec.Rules[0].Host != "app.example.com" || ingress.Spec.TLS[0].SecretName != "site-tls" || path.Backend.Service.Name != ResourceName(b.ProjectID, "web") || path.Backend.Service.Port.Number != 8080 || path.Path != "/app" || string(*path.PathType) != "Prefix" {
		t.Fatal("compiler confused public TLS443 with app backend or changed reviewed selection", ingress.Spec)
	}
	if ingress.Annotations["traefik.ingress.kubernetes.io/router.entrypoints"] != "websecure" || ingress.Annotations["traefik.ingress.kubernetes.io/router.tls"] != "true" || ingress.Spec.DefaultBackend != nil {
		t.Fatal("unexpected controller instructions")
	}
	b.PublicRoute = nil
	again, err := Compile(b)
	if err != nil || again.Digest != private.Digest || !reflect.DeepEqual(again.Artifacts, private.Artifacts) {
		t.Fatal("removing optional route changed legacy private compiler", err)
	}
}
func TestPublicRouteCompilerRejectsUnsupportedExposureAndCredentialChannels(t *testing.T) {
	for _, change := range []struct {
		name string
		edit func(*Blueprint)
	}{
		{"inference", func(b *Blueprint) { b.PublicRoute.ServiceID = "model" }},
		{"ip", func(b *Blueprint) { b.PublicRoute.Hostname = "127.0.0.1" }},
		{"numeric-tld", func(b *Blueprint) { b.PublicRoute.Hostname = "999.123.456.789" }},
		{"wildcard", func(b *Blueprint) { b.PublicRoute.Hostname = "*.example.com" }},
		{"query", func(b *Blueprint) { b.PublicRoute.Path = "/?token=secret" }},
		{"encoded", func(b *Blueprint) { b.PublicRoute.Path = "/a%2fb" }},
		{"traversal", func(b *Blueprint) { b.PublicRoute.Path = "/a/../b" }},
		{"controller", func(b *Blueprint) { b.PublicRoute.Profile = "arbitrary-annotations" }},
	} {
		t.Run(change.name, func(t *testing.T) {
			b := exampleBlueprint()
			b.PublicRoute = &PublicRoute{Profile: PublicRouteProfile, ServiceID: "web", IngressClassName: "traefik", Hostname: "app.example.com", Path: "/", PathType: "Exact", TLSSecretName: "site-tls"}
			change.edit(&b)
			if _, err := Compile(b); err == nil {
				t.Fatal("unsupported exposure compiled")
			}
		})
	}
}
