package previewgateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataBridgeGatewayPreservesOnlySameOwnerProjectClientSurface(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(r.Header.Get("Authorization"))) }))
	defer up.Close()
	g, registry := testGateway(t, func(context.Context, Target) error { return nil })
	bridge := Target{Kind: "bridge", ClientKind: "data", ProjectID: "project-1", ResourceID: "data-bridge", Upstream: up.URL, BridgeKey: strings.Repeat("b", 64), PublishableKey: clientJWT("anon")}
	data, err := registry.Publish(bridge)
	if err != nil {
		t.Fatal(err)
	}
	appTarget := Target{Kind: "bridge", ClientKind: "application", ProjectID: "project-1", ResourceID: "app-bridge", Upstream: up.URL, BridgeKey: strings.Repeat("c", 64)}
	app, err := registry.Publish(appTarget)
	if err != nil {
		t.Fatal(err)
	}
	user := clientJWT("authenticated")
	res := request(g, "POST", data, "/rest/v1/notes", map[string]string{"Origin": app, "Authorization": "Bearer " + user, "apikey": bridge.PublishableKey})
	if res.Code != 200 || res.Header().Get("Access-Control-Allow-Origin") != app || res.Body.String() != "Bearer "+user {
		t.Fatal("same owner/project cloud data bridge failed")
	}
	if w := request(g, "GET", data, "/rest/v1/notes", map[string]string{"Authorization": "Bearer " + clientJWT("service_role")}); w.Code != 401 {
		t.Fatal("data bridge admitted service key")
	}
	if w := request(g, "GET", data, "/v1/admin/users", nil); w.Code != 404 {
		t.Fatal("data bridge exposed control route")
	}
	foreign := filepath.Join(g.stateDir, "control-plane/users/another/runtime/state/sessions")
	_ = os.MkdirAll(foreign, 0700)
	foreignRegistry, err := NewRegistry(foreign, 8731)
	if err != nil {
		t.Fatal(err)
	}
	foreignApp, err := foreignRegistry.Publish(appTarget)
	if err != nil {
		t.Fatal(err)
	}
	if w := request(g, "POST", data, "/rest/v1/notes", map[string]string{"Origin": foreignApp}); w.Code != 403 {
		t.Fatal("another owner's identical project crossed cloud data bridge")
	}
	appTarget.ProjectID = "another-project"
	different, err := registry.Publish(appTarget)
	if err != nil {
		t.Fatal(err)
	}
	if w := request(g, "POST", data, "/rest/v1/notes", map[string]string{"Origin": different}); w.Code != 403 {
		t.Fatal("another project crossed cloud data bridge")
	}
}
