package platform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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

func TestRestoredDataKeepsBrowserOriginAndVerifiesReplacementStack(t *testing.T) {
	s, _ := newDataFake(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sessions"), 0700); err != nil {
		t.Fatal(err)
	}
	registry, err := previewgateway.NewRegistry(filepath.Join(root, "sessions"), 8731)
	if err != nil {
		t.Fatal(err)
	}
	s.browserRegistry = registry
	upstream := func(body string) *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rest/v1/health" {
				t.Error("request left the registered data API")
			}
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(server.Close)
		return server
	}
	first, replacement := upstream("original"), upstream("restored")
	port := func(raw string) int {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		value, err := strconv.Atoi(u.Port())
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	original := dataInstallation{ProjectID: "owned", StackID: "stack-original", APIPort: port(first.URL)}
	active := original
	secret := newDataSecret()
	activeSecret := secret
	saveInstallation := func(i dataInstallation, credentials dataSecret) {
		t.Helper()
		if err := s.write("owned-data-installation", i); err != nil {
			t.Fatal(err)
		}
		if err := s.write(i.StackID+"-secret", credentials); err != nil {
			t.Fatal(err)
		}
	}
	saveInstallation(original, secret)
	address, err := s.browserDataURL(original, secret)
	if err != nil {
		t.Fatal(err)
	}
	verified := 0
	gateway, err := previewgateway.New(root, 8731, func(_ context.Context, target previewgateway.Target) error {
		verified++
		if target.ResourceID != original.storageTenant() || target.ProjectID != "owned" || target.Container != s.dataName(active, "api-gw") || target.Upstream != dataURL(active) || target.Labels[labelInstance] != s.instance || target.Labels[labelProject] != "owned" || target.Labels[labelResource] != "data-"+active.StackID+"-api-gw" || target.PublishableKey != activeSecret.Anon {
			return errors.New("replacement data container ownership changed")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		gateway.ServeHTTP(w, httptest.NewRequest("GET", address+"/rest/v1/health", nil))
		return w
	}
	if response := request(); response.Code != 200 || response.Body.String() != "original" {
		t.Fatal("original browser address did not reach its owned stack")
	}
	candidate := dataInstallation{ProjectID: "owned", StackID: "stack-replacement", StorageTenantID: original.storageTenant(), APIPort: port(replacement.URL), Previous: []string{original.StackID}}
	candidateSecret := newDataSecret()
	if err := s.prepareDataStack(candidate, candidateSecret); err != nil {
		t.Fatal(err)
	}
	candidateRoot, err := s.dataRoot(candidate)
	if err != nil {
		t.Fatal(err)
	}
	env, err := os.ReadFile(filepath.Join(candidateRoot, "runtime.env"))
	if err != nil || !strings.Contains(string(env), "SUPABASE_PUBLIC_URL="+address+"\n") || !strings.Contains(string(env), "API_EXTERNAL_URL="+address+"/auth/v1\n") {
		t.Fatal("candidate configuration lost the durable client address")
	}
	if response := request(); response.Code != 200 || response.Body.String() != "original" {
		t.Fatal("candidate preparation redirected the live browser endpoint before promotion")
	}
	active, activeSecret = candidate, candidateSecret
	saveInstallation(active, activeSecret)
	view := s.browserDataStatus(DataStatus{ProjectID: "owned", Status: "ready", URL: dataURL(active), PublishableKey: activeSecret.Anon})
	bindings, err := s.dataBindings("owned")
	if err != nil || view.URL != address || view.EndpointScope != "browser-gateway" || bindings["SUPABASE_PUBLIC_URL"] != address {
		t.Fatal("restoration invalidated existing browser client configuration")
	}
	if response := request(); response.Code != 200 || response.Body.String() != "restored" || verified != 3 {
		t.Fatal("unchanged browser address did not verify and reach the replacement stack")
	}
	// A stale target must still fail verification even though the public data
	// identity remains stable. It cannot silently reach the retained old stack.
	if _, err := registry.Publish(previewgateway.Target{Kind: "data", ProjectID: "owned", ResourceID: original.storageTenant(), Upstream: first.URL, Container: s.dataName(original, "api-gw"), Labels: map[string]string{labelInstance: s.instance, labelProject: "owned", labelResource: "data-" + original.StackID + "-api-gw"}, PublishableKey: secret.Anon}); err != nil {
		t.Fatal(err)
	}
	if response := request(); response.Code != 503 {
		t.Fatal("stale owned target bypassed active-stack verification")
	}
}

func TestAPIOnlyRestorePublishesActiveDataTargetAndReinstatesOriginalOnFailure(t *testing.T) {
	for _, failActivation := range []bool{false, true} {
		name := "healthy-promotion"
		if failActivation {
			name = "failed-activation"
		}
		t.Run(name, func(t *testing.T) {
			s, f := newDataFake(t)
			registry, err := previewgateway.NewRegistry(s.opts.StateDir, 8731)
			if err != nil {
				t.Fatal(err)
			}
			s.browserRegistry = registry
			installData(t, s)
			var original dataInstallation
			if err := s.read("owned-data-installation", &original); err != nil {
				t.Fatal(err)
			}
			w := request(t, s, "POST", "/v1/platform/projects/owned/data", "member", dataActionRequest{Action: "backup"})
			if w.Code != 202 {
				t.Fatal("backup not accepted")
			}
			state := waitData(t, s)
			if state.Operation.Status != "succeeded" || len(state.Backups) != 1 {
				t.Fatal("backup did not finish")
			}
			// The live stack's current credentials may differ from the earlier
			// snapshot. Rollback must use its own keys, not the archive's keys.
			liveSecret := newDataSecret()
			if err := s.write(original.StackID+"-secret", liveSecret); err != nil {
				t.Fatal(err)
			}
			address, err := s.browserDataURL(original, liveSecret)
			if err != nil {
				t.Fatal(err)
			}
			injected := false
			transport := s.opts.HTTPClient.Transport
			if failActivation {
				s.opts.HTTPClient = &http.Client{Transport: dataTransport(func(r *http.Request) (*http.Response, error) {
					var current dataInstallation
					if !injected && r.URL.Path == "/auth/v1/health" && s.read("owned-data-installation", &current) == nil && current.StackID != original.StackID {
						injected = true
						candidateSecret, err := s.dataCredentials(current)
						if err != nil {
							return nil, err
						}
						// A concurrent browser status read can register the candidate
						// between the active write and final health result.
						if _, err := s.browserDataURL(current, candidateSecret); err != nil {
							return nil, err
						}
						return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("unhealthy")), Header: http.Header{}}, nil
					}
					return transport.RoundTrip(r)
				})}
			}
			w = request(t, s, "POST", "/v1/platform/projects/owned/data", "member", dataActionRequest{Action: "restore", BackupID: state.Backups[0].ID, AcknowledgeResources: true, ConfirmRestore: true})
			if w.Code != 202 {
				t.Fatal("restore not accepted")
			}
			state = waitData(t, s) // private receipt only; no DTO/bindings read publishes a target
			var active dataInstallation
			if err := s.read("owned-data-installation", &active); err != nil {
				t.Fatal(err)
			}
			if failActivation {
				if !injected || state.Operation.Status != "failed" || state.Status != "ready" || active.StackID != original.StackID || !strings.Contains(state.Message, "reinstated") {
					t.Fatal("failed activation did not truthfully reinstate the original stack")
				}
			} else if state.Operation.Status != "succeeded" || active.StackID == original.StackID {
				t.Fatal("API-only restore did not promote the healthy stack")
			}
			files, err := os.ReadDir(filepath.Join(s.opts.StateDir, "browser-previews"))
			if err != nil || len(files) != 1 {
				t.Fatal("restore changed the durable origin inventory")
			}
			bytes, err := os.ReadFile(filepath.Join(s.opts.StateDir, "browser-previews", files[0].Name()))
			var registered struct {
				previewgateway.Target
				Key string `json:"key"`
			}
			if err != nil || json.Unmarshal(bytes, &registered) != nil {
				t.Fatal("active browser target unavailable")
			}
			activeSecret, err := s.dataCredentials(active)
			if err != nil || address != "http://"+registered.Key+".localhost:8731" || registered.ResourceID != original.storageTenant() || registered.Container != s.dataName(active, "api-gw") || registered.Upstream != dataURL(active) || registered.Labels[labelResource] != "data-"+active.StackID+"-api-gw" || registered.PublishableKey != activeSecret.Anon {
				t.Fatal("API-only restore left the browser target or credentials pointing at another stack")
			}
			for _, name := range dataServiceNames {
				if !f.containers[s.dataName(active, name)].State.Running {
					t.Fatal("registered active stack is not running")
				}
			}
		})
	}
}
