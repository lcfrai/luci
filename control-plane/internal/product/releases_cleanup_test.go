package product

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestReleaseCleanupCapabilityUsesPrivateRetainedHandle(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "release-cleanup-capability")
	cases := []struct {
		id, target, status, container string
		want                          bool
	}{
		{"failed-retained", "local", "failed", strings.Repeat("a", 64), true},
		{"unknown-retained", "local", "unknown", strings.Repeat("b", 12), true},
		{"failed-no-handle", "local", "failed", "", false},
		{"stopped-retained", "local", "stopped", strings.Repeat("c", 64), false},
		{"failed-unsafe-handle", "local", "failed", "../another-container", false},
		{"failed-short-handle", "local", "failed", "abc123", false},
		{"remote-retained", "remote", "failed", strings.Repeat("d", 64), false},
	}
	for _, test := range cases {
		row := storedRelease{
			Release:     Release{ID: test.id, ProjectID: p.ID, Target: test.target, Status: test.status, CanStop: !test.want},
			ContainerID: test.container, Image: "private-image-" + test.id,
		}
		if err := s.storeRelease(row); err != nil {
			t.Fatal(err)
		}
	}
	// Failed/unknown/stopped rows need no Docker probe to offer cleanup. The
	// capability is reconstructed from private history, not a saved public flag.
	s.runner = func(_ context.Context, executable string, args []string, _ string) (string, error) {
		t.Fatalf("listing retained cleanup handles dispatched %s %v", executable, args)
		return "", nil
	}
	w := journeyRequest(t, mux, http.MethodGet, "/v1/product/projects/"+p.ID+"/releases", nil, http.StatusOK)
	var response struct {
		Releases []map[string]json.RawMessage `json:"releases"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Releases) != len(cases) {
		t.Fatalf("got %d releases, want %d", len(response.Releases), len(cases))
	}
	byID := make(map[string]map[string]json.RawMessage, len(response.Releases))
	for _, row := range response.Releases {
		var id string
		if err := json.Unmarshal(row["id"], &id); err != nil {
			t.Fatal(err)
		}
		byID[id] = row
		for _, field := range []string{"containerId", "image", "ContainerID", "Image"} {
			if _, exists := row[field]; exists {
				t.Fatalf("release %s exposed private field %s", id, field)
			}
		}
	}
	for _, test := range cases {
		t.Run(test.id, func(t *testing.T) {
			row, exists := byID[test.id]
			if !exists {
				t.Fatal("retained release disappeared")
			}
			raw, present := row["canStop"]
			if !present || string(raw) != "true" && string(raw) != "false" {
				t.Fatalf("canStop must be an explicit JSON boolean, got %q", raw)
			}
			if got := string(raw) == "true"; got != test.want {
				t.Fatalf("canStop=%v, want %v", got, test.want)
			}
			if strings.Contains(w.Body.String(), "private-image-"+test.id) || test.container != "" && strings.Contains(w.Body.String(), test.container) {
				t.Fatal("public release JSON exposed a private image or container handle")
			}
		})
	}
}

func TestFailedReleaseCleanupStillRefusesForeignContainer(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "failed-release-ownership")
	row := storedRelease{
		Release:     Release{ID: "failed-owned-handle", ProjectID: p.ID, Target: "local", Status: "failed"},
		ContainerID: strings.Repeat("e", 64), Image: "retained-private-image",
	}
	if err := s.storeRelease(row); err != nil {
		t.Fatal(err)
	}
	if !s.browserRelease(row).CanStop {
		t.Fatal("failed release with a retained handle did not offer cleanup")
	}
	inspections := 0
	s.runner = func(_ context.Context, executable string, args []string, _ string) (string, error) {
		if executable != "docker" || len(args) != 4 || args[0] != "inspect" || args[3] != row.ContainerID {
			t.Fatalf("ownership refusal attempted an unexpected operation: %s %v", executable, args)
		}
		inspections++
		return `{"luci.preview":"true","luci.release-id":"foreign-release","luci.project-id":"foreign-project"}`, nil
	}
	journeyRequest(t, mux, http.MethodPost, "/v1/product/projects/"+p.ID+"/releases/"+row.ID+"/stop", nil, http.StatusConflict)
	if inspections != 1 {
		t.Fatalf("got %d ownership inspections, want 1", inspections)
	}
	retained, err := s.readReleases(p.ID)
	if err != nil || len(retained) != 1 || retained[0] != row {
		t.Fatalf("foreign ownership refusal altered the private release: %#v, %v", retained, err)
	}
}
