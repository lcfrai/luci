package platform

import (
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/previewgateway"
)

func TestBrowserDataBindingsKeepPrivateServerURLAndPublicClientURL(t *testing.T) {
	s, _ := newDataFake(t)
	registry, err := previewgateway.NewRegistry(s.opts.StateDir, 8731)
	if err != nil {
		t.Fatal(err)
	}
	s.browserRegistry = registry
	i := dataInstallation{ProjectID: "owned", StackID: "stack-browser", APIPort: 35432}
	secret := newDataSecret()
	if err := s.write("owned-data-installation", i); err != nil {
		t.Fatal(err)
	}
	if err := s.write(i.StackID+"-secret", secret); err != nil {
		t.Fatal(err)
	}
	if err := s.changeData("owned", func(v *DataStatus) { v.Status = "ready" }); err != nil {
		t.Fatal(err)
	}
	bindings, err := s.dataBindings("owned")
	if err != nil {
		t.Fatal(err)
	}
	if bindings["SUPABASE_URL"] != "http://supabase-api:8000" || !strings.Contains(bindings["SUPABASE_PUBLIC_URL"], ".localhost:8731") || bindings["SUPABASE_ANON_KEY"] != secret.Anon || bindings["SUPABASE_SERVICE_ROLE_KEY"] != secret.ServiceRole {
		t.Fatal("server/client data bindings conflated")
	}
	v := s.browserDataStatus(DataStatus{ProjectID: "owned", URL: dataURL(i), PublishableKey: secret.Anon})
	if v.EndpointScope != "browser-gateway" || v.URL != bindings["SUPABASE_PUBLIC_URL"] {
		t.Fatal("client DTO and deployed app use different browser endpoint")
	}
	d := Deployment{ID: "deployment-1", ProjectID: "owned", Provider: "docker", Status: "ready", Container: "owned-app", URL: "http://127.0.0.1:34567"}
	view := s.browserDeployment(d)
	if !strings.Contains(view.URL, ".localhost:8731") || d.URL != "http://127.0.0.1:34567" {
		t.Fatal("DTO mapping changed private health receipt")
	}
}
