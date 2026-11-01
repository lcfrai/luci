package deploymentplan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestManagedHTTPSReusesRetainedEndpointWithoutDuplicateIngress(t *testing.T) {
	b := exampleBlueprint()
	private, e := Compile(b)
	if e != nil {
		t.Fatal(e)
	}
	b.PublicRoute = &PublicRoute{Profile: ManagedPublicRouteProfile, ManagedHTTPSID: "https-" + strings.Repeat("a", 32), ServiceID: "web", Hostname: "app.example.com", Path: "/app", PathType: "Prefix"}
	plan, e := Compile(b)
	if e != nil {
		t.Fatal(e)
	}
	if RuntimeReferenceCount(b) != RuntimeReferenceCount(private.Blueprint) {
		t.Fatal("managed endpoint invented manual Secret/class imports")
	}
	if len(plan.Artifacts) != len(private.Artifacts) {
		t.Fatal("duplicate Ingress/TLS artifact")
	}
	for _, a := range plan.Artifacts {
		var meta struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal([]byte(a.Content), &meta) != nil || meta.Kind == "Ingress" {
			t.Fatal("unexpected public controller artifact")
		}
	}
	b.PublicRoute.TLSSecretName = "manual-tls"
	if _, e = Compile(b); e == nil {
		t.Fatal("mixed manual/managed authority")
	}
	b.PublicRoute = nil
	again, e := Compile(b)
	if e != nil || again.Digest != private.Digest {
		t.Fatal("legacy omission changed")
	}
}
