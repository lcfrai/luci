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

func TestApplicationPrototypeReleasesRequireLiveOwnedHealthAndSource(t *testing.T) {
	for _, reason := range []string{"healthy", "stopped", "not-running", "wrong-image", "foreign-label", "wrong-port", "unhealthy", "foreign-record", "bad-commit", "duplicate", "busy", "canceled", "changed-during-health"} {
		t.Run(reason, func(t *testing.T) {
			calls := 0
			var healthCalls atomic.Int32
			var s *Service
			health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				healthCalls.Add(1)
				if reason == "changed-during-health" {
					if err := s.update("owned", func(state *ProjectState) { state.Deployments[0].Status = "stopped" }); err != nil {
						t.Error(err)
					}
				}
				if r.URL.Path != "/health" {
					t.Error("health path changed")
				}
				if reason == "unhealthy" {
					w.WriteHeader(503)
				} else {
					w.WriteHeader(200)
				}
			}))
			defer health.Close()
			address, _ := url.Parse(health.URL)
			s = fixture(t, nil)
			d := Deployment{ID: "deploy-owned", ProjectID: "owned", Provider: "docker", Status: "ready", Commit: strings.Repeat("a", 40), Container: "owned-app", Image: "sha256:" + strings.Repeat("b", 64), Port: 3000, HealthPath: "/health", URL: "http://127.0.0.1:12345", ConfigurationRevision: 7}
			info := containerInfo{Image: d.Image}
			info.State.Running = true
			info.Config.Labels = map[string]string{labelInstance: s.instance, labelProject: d.ProjectID, labelResource: d.ID}
			info.NetworkSettings.Ports = map[string][]struct{ HostIP, HostPort string }{"3000/tcp": {{"127.0.0.1", address.Port()}}}
			switch reason {
			case "stopped":
				d.Status = "stopped"
			case "not-running":
				info.State.Running = false
			case "wrong-image":
				info.Image = "sha256:changed"
			case "foreign-label":
				info.Config.Labels[labelProject] = "foreign"
			case "wrong-port":
				info.NetworkSettings.Ports["3000/tcp"][0].HostIP = "0.0.0.0"
			case "foreign-record":
				d.ProjectID = "foreign"
			case "bad-commit":
				d.Commit = "invalid"
			case "busy":
				s.busy["owned"] = true
			}
			rows := []Deployment{d}
			if reason == "duplicate" {
				rows = append(rows, d)
			}
			if err := s.update("owned", func(state *ProjectState) { state.Deployments = rows }); err != nil {
				t.Fatal(err)
			}
			s.opts.Runner = func(ctx context.Context, exe string, args []string, dir string, input []byte) ([]byte, error) {
				calls++
				if exe != "docker" || len(args) != 2 || args[0] != "inspect" || args[1] != "owned-app" {
					t.Error("prototype attempted a resource mutation")
					return nil, errors.New("unexpected operation")
				}
				return json.Marshal([]containerInfo{info})
			}
			ctx := context.Background()
			if reason == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			result, err := s.ApplicationPrototypeReleases(ctx, "owned")
			if reason == "foreign-record" || reason == "bad-commit" || reason == "duplicate" || reason == "canceled" {
				if err == nil {
					t.Fatal("invalid native provenance was accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if reason == "healthy" {
				if len(result) != 1 || result[0].Commit != d.Commit || result[0].ID != d.ID || result[0].ProjectID != "owned" || result[0].ConfigurationRevision != 7 {
					t.Fatal("verified native source missing")
				}
				data, _ := json.Marshal(result)
				for _, private := range []string{d.Container, d.Image, address.Port(), d.URL} {
					if strings.Contains(string(data), private) {
						t.Fatal("native prototype metadata exposed a private resource")
					}
				}
				if calls != 1 || healthCalls.Load() != 1 {
					t.Fatal("live ownership/health were not verified")
				}
			} else if len(result) != 0 {
				t.Fatal("unavailable native application remained selectable")
			}
			if (reason == "busy" || reason == "stopped") && calls != 0 {
				t.Fatal("inactive deployment was probed")
			}
		})
	}
}
