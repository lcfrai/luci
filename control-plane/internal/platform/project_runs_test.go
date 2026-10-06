package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNativeProjectRunAdmissionSavesProvenanceBeforeAllocating(t *testing.T) {
	var runnerCalls atomic.Int32
	s := fixture(t, func(context.Context, string, []string, string, []byte) ([]byte, error) {
		runnerCalls.Add(1)
		return nil, errors.New("must not allocate")
	})
	called := false
	s.opts.RecordDeploymentProvenance = func(_ context.Context, projectID, releaseID, target, commit string) error {
		called = true
		if projectID != "owned" || !strings.HasPrefix(releaseID, "deploy-") || target != "native-docker" || commit != strings.Repeat("a", 40) {
			t.Fatal("native provenance used unowned inputs")
		}
		return errors.New("private provenance unavailable")
	}
	w := request(t, s, "POST", "/v1/platform/projects/owned/deployments", "member", map[string]any{"commit": strings.Repeat("a", 40), "port": 3000, "healthPath": "/health"})
	if w.Code != 409 || !called || runnerCalls.Load() != 0 {
		t.Fatal("failed provenance allocated a native resource", w.Code, w.Body.String(), runnerCalls.Load())
	}
	state, err := s.load("owned")
	if err != nil || len(state.Deployments) != 0 {
		t.Fatal("failed admission saved a deployable child", err)
	}
}

func TestNativeProjectRunObservationRequiresOwnedImagePortAndRealHealth(t *testing.T) {
	var hits atomic.Int32
	var health atomic.Int32
	health.Store(200)
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/health" {
			t.Error("wrong health path")
		}
		w.WriteHeader(int(health.Load()))
	}))
	defer app.Close()
	address, _ := url.Parse(app.URL)
	var inspection []byte
	s := fixture(t, func(_ context.Context, name string, args []string, _ string, _ []byte) ([]byte, error) {
		if name != "docker" || len(args) != 2 || args[0] != "inspect" {
			t.Error("observation attempted resource mutation", name, args)
			return nil, errors.New("unexpected command")
		}
		return inspection, nil
	})
	d := Deployment{ID: "deploy-observe", ProjectID: "owned", Provider: "docker", Commit: strings.Repeat("a", 40), Status: "ready", URL: app.URL, Container: "owned-container", Image: "sha256:owned", Port: 3000, HealthPath: "/health"}
	if err := s.saveDeployment(d); err != nil {
		t.Fatal(err)
	}
	info := containerInfo{ID: d.Container, Image: d.Image}
	info.Config.Labels = map[string]string{labelInstance: s.instance, labelProject: d.ProjectID, labelResource: d.ID}
	info.Config.Image = "luci-platform/" + s.instance + ":" + d.ID
	info.State.Running = true
	info.NetworkSettings.Ports = map[string][]struct{ HostIP, HostPort string }{"3000/tcp": {{HostIP: "127.0.0.1", HostPort: address.Port()}}}
	inspection, _ = json.Marshal([]containerInfo{info})
	observed, err := s.ObserveProjectRunDeployment(context.Background(), d.ProjectID, d.ID)
	if err != nil || observed.Deployment.ID != d.ID || observed.ObservedAt == "" || observed.Limitations == "" || hits.Load() != 1 {
		t.Fatal("owned healthy release unobserved", observed, err)
	}
	for _, test := range []string{"foreign", "image", "port", "stopped"} {
		copy := info
		switch test {
		case "foreign":
			copy.Config.Labels = map[string]string{labelInstance: s.instance, labelProject: "foreign", labelResource: d.ID}
		case "image":
			copy.Image = "sha256:other"
		case "port":
			copy.NetworkSettings.Ports = map[string][]struct{ HostIP, HostPort string }{"3000/tcp": {{HostIP: "0.0.0.0", HostPort: address.Port()}}}
		case "stopped":
			copy.State.Running = false
		}
		inspection, _ = json.Marshal([]containerInfo{copy})
		before := hits.Load()
		if _, err := s.ObserveProjectRunDeployment(context.Background(), d.ProjectID, d.ID); err == nil || hits.Load() != before {
			t.Fatal("unowned release reached health or joined", test, err)
		}
	}
	inspection, _ = json.Marshal([]containerInfo{info})
	health.Store(503)
	if _, err := s.ObserveProjectRunDeployment(context.Background(), d.ProjectID, d.ID); err == nil {
		t.Fatal("ready history row bypassed current health")
	}
	if _, err := s.ObserveProjectRunDeployment(context.Background(), "foreign", d.ID); err == nil {
		t.Fatal("foreign project observed")
	}
}
