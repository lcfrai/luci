package e2bbe

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPinnedTemplatePublicationCheckpointsBeforeOneBuildAndInspects(t *testing.T) {
	const image = "registry.example/luci/guest@sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	journal := filepath.Join(t.TempDir(), "build-intent.json")
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "operator-test-key" {
			t.Error("build request omitted private operator authority")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /v2/templates/template-one/builds/build-one":
			posts++
			data, err := os.ReadFile(journal)
			if err != nil {
				t.Error("provider effect preceded durable intent", err)
			}
			var saved TemplateBuildIntent
			if json.Unmarshal(data, &saved) != nil || saved.BuildID != "build-one" || saved.FromImage != image || saved.ImageDigest != strings.Repeat("a", 64) {
				t.Error("provider effect had no exact pinned image intent")
			}
			var body struct {
				FromImage string `json:"fromImage"`
				Force     bool   `json:"force"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil || body.FromImage != image || body.Force {
				t.Error("build request changed the immutable image or forced an unrelated rebuild")
			}
			w.WriteHeader(http.StatusAccepted)
		case "GET /templates/template-one/builds/build-one/status":
			if r.URL.Query().Get("limit") != "0" {
				t.Error("status inspected unbounded provider logs")
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"templateID": "template-one", "buildID": "build-one", "status": "ready"})
		default:
			t.Errorf("unexpected provider request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	p := TemplatePublication{Endpoint: server.URL, APIKey: "operator-test-key", RootCAs: roots,
		TemplateID: "template-one", BuildID: "build-one", FromImage: image, JournalPath: journal}
	intent, err := p.StartTemplateBuild(context.Background())
	if err != nil || !intent.Valid() || TemplateImageBinding(intent) == "" {
		t.Fatal("exact build did not start", err)
	}
	if _, err := p.StartTemplateBuild(context.Background()); !errors.Is(err, ErrConflict) || posts != 1 {
		t.Fatal("repeated operator request replayed the provider build", posts, err)
	}
	if status, err := p.InspectTemplateBuild(context.Background()); err != nil || status != "ready" {
		t.Fatal("pinned status was not readable after the exact mutation", status, err)
	}
	p.FromImage = "registry.example/luci/other@sha256:" + strings.Repeat("b", 64)
	if _, err := p.InspectTemplateBuild(context.Background()); !errors.Is(err, ErrConflict) {
		t.Fatal("different image adopted the saved intent", err)
	}
}

func TestPinnedTemplatePublicationKeepsUncertainMutationWithoutRetry(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "uncertain.json")
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	p := TemplatePublication{Endpoint: server.URL, APIKey: "operator-test-key", RootCAs: roots,
		TemplateID: "template-one", BuildID: "build-one", FromImage: "guest@sha256:" + strings.Repeat("a", 64), JournalPath: journal}
	if _, err := p.StartTemplateBuild(context.Background()); !errors.Is(err, ErrUnknown) {
		t.Fatal("uncertain provider response appeared safely retryable", err)
	}
	if _, err := p.StartTemplateBuild(context.Background()); !errors.Is(err, ErrConflict) || posts != 1 {
		t.Fatal("uncertain build POST was replayed", posts, err)
	}
	p.JournalPath = filepath.Join(t.TempDir(), "invalid.json")
	p.FromImage = "guest:mutable-tag"
	if _, err := p.StartTemplateBuild(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Fatal("mutable image tag was accepted", err)
	}
}

func TestPrivateTemplateCreationRetainsExactIDsAndNeverReplays(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "creation.json")
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		if r.Method != http.MethodPost || r.URL.Path != "/v3/templates" || r.Header.Get("X-API-Key") != "operator-test-key" {
			t.Error("unexpected template creation request")
		}
		var intent TemplateCreateIntent
		data, err := os.ReadFile(journal)
		if err != nil || json.Unmarshal(data, &intent) != nil || intent.Name != "luci-guest-one" || intent.CPUCount != 2 || intent.MemoryMiB != 2048 {
			t.Error("private template mutation preceded its exact intent", err)
		}
		var body struct {
			Name      string `json:"name"`
			CPUCount  int64  `json:"cpuCount"`
			MemoryMiB int64  `json:"memoryMB"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Name != intent.Name || body.CPUCount != intent.CPUCount || body.MemoryMiB != intent.MemoryMiB {
			t.Error("provider received a changed template request")
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"templateID": "template-one", "buildID": "build-one", "public": false})
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	p := TemplateCreation{Endpoint: server.URL, APIKey: "operator-test-key", RootCAs: roots,
		Name: "luci-guest-one", CPUCount: 2, MemoryMiB: 2048, JournalPath: journal}
	created, err := p.CreateTemplate(context.Background())
	if err != nil || created.TemplateID != "template-one" || created.BuildID != "build-one" {
		t.Fatal("private template IDs were not retained", created, err)
	}
	if _, err = p.CreateTemplate(context.Background()); !errors.Is(err, ErrConflict) || posts != 1 {
		t.Fatal("duplicate template Create escaped the durable fence", posts, err)
	}
	read, err := p.ReadTemplateCreation()
	if err != nil || read != created {
		t.Fatal("acknowledged creation cannot be read after restart", read, err)
	}
	p.CPUCount = 4
	if _, err = p.ReadTemplateCreation(); !errors.Is(err, ErrUnknown) {
		t.Fatal("changed template parameters adopted an old receipt", err)
	}
}

func TestUnknownPrivateTemplateCreationIsNotRetried(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "creation.json")
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	p := TemplateCreation{Endpoint: server.URL, APIKey: "operator-test-key", RootCAs: roots,
		Name: "luci-guest-one", CPUCount: 2, MemoryMiB: 2048, JournalPath: journal}
	if _, err := p.CreateTemplate(context.Background()); !errors.Is(err, ErrUnknown) {
		t.Fatal("uncertain template creation appeared complete", err)
	}
	if _, err := p.CreateTemplate(context.Background()); !errors.Is(err, ErrConflict) || posts != 1 {
		t.Fatal("uncertain template creation was replayed", posts, err)
	}
	if _, err := p.ReadTemplateCreation(); !errors.Is(err, ErrUnknown) {
		t.Fatal("absent provider acknowledgement was fabricated", err)
	}
}
