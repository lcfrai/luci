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

func acknowledgedTemplateCreate(t *testing.T, dir, templateID, buildID string) string {
	t.Helper()
	path := filepath.Join(dir, "create-intent.json")
	intent := TemplateCreateIntent{RuntimeRevision: RuntimeRevision,
		Name: "luci-guest-0123456789abcdef0123456789abcdef", CPUCount: 2, MemoryMiB: 2048}
	body, _ := json.Marshal(intent)
	if err := checkpointPublicationIntent(path, body); err != nil {
		t.Fatal(err)
	}
	ack, _ := json.Marshal(TemplateCreateReceipt{TemplateCreateIntent: intent, TemplateID: templateID, BuildID: buildID})
	if err := checkpointPublicationIntent(path+".response", ack); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPinnedTemplatePublicationCheckpointsBeforeOneBuildAndInspects(t *testing.T) {
	const image = "registry.example/luci/guest@sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	dir := t.TempDir()
	journal := filepath.Join(dir, "build-intent.json")
	createJournal := acknowledgedTemplateCreate(t, dir, "template-one", "build-one")
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
		TemplateID: "template-one", BuildID: "build-one", FromImage: image, CreateJournalPath: createJournal, JournalPath: journal}
	intent, err := p.StartTemplateBuild(context.Background())
	if err != nil || !intent.Valid() || TemplateImageBinding(intent) == "" {
		t.Fatal("exact build did not start", err)
	}
	if saved, err := p.ReadTemplateBuildIntent(); err != nil || saved != intent {
		t.Fatal("selected template lost its exact private build intent", saved, err)
	}
	if _, err := p.StartTemplateBuild(context.Background()); !errors.Is(err, ErrConflict) || posts != 1 {
		t.Fatal("repeated operator request replayed the provider build", posts, err)
	}
	if status, err := p.InspectTemplateBuild(context.Background()); err != nil || status != "ready" {
		t.Fatal("pinned status was not readable after the exact mutation", status, err)
	}
	p.FromImage = "registry.example/luci/other@sha256:" + strings.Repeat("b", 64)
	if _, err := p.ReadTemplateBuildIntent(); !errors.Is(err, ErrConflict) {
		t.Fatal("changed image adopted the saved build intent", err)
	}
	if _, err := p.InspectTemplateBuild(context.Background()); !errors.Is(err, ErrConflict) {
		t.Fatal("different image adopted the saved intent", err)
	}
}

func TestPinnedTemplatePublicationKeepsUncertainMutationWithoutRetry(t *testing.T) {
	dir := t.TempDir()
	journal := filepath.Join(dir, "uncertain.json")
	createJournal := acknowledgedTemplateCreate(t, dir, "template-one", "build-one")
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	p := TemplatePublication{Endpoint: server.URL, APIKey: "operator-test-key", RootCAs: roots,
		TemplateID: "template-one", BuildID: "build-one", FromImage: "guest@sha256:" + strings.Repeat("a", 64), CreateJournalPath: createJournal, JournalPath: journal}
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

func TestTemplateBuildRequiresExactAcknowledgedPrivateCreate(t *testing.T) {
	dir := t.TempDir()
	createJournal := acknowledgedTemplateCreate(t, dir, "template-one", "build-one")
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		posts++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	p := TemplatePublication{Endpoint: server.URL, APIKey: "operator-test-key", RootCAs: roots,
		TemplateID: "template-two", BuildID: "build-one", FromImage: "guest@sha256:" + strings.Repeat("a", 64),
		CreateJournalPath: createJournal, JournalPath: filepath.Join(dir, "build-intent.json")}
	if _, err := p.StartTemplateBuild(context.Background()); !errors.Is(err, ErrConflict) || posts != 0 {
		t.Fatal("foreign template received a build mutation", err, posts)
	}
	p.TemplateID = "template-one"
	p.BuildID = "build-two"
	if _, err := p.StartTemplateBuild(context.Background()); !errors.Is(err, ErrConflict) || posts != 0 {
		t.Fatal("changed build ID received a build mutation", err, posts)
	}
	p.BuildID = "build-one"
	p.CreateJournalPath = filepath.Join(dir, "missing-create.json")
	if _, err := p.StartTemplateBuild(context.Background()); !errors.Is(err, ErrUnknown) || posts != 0 {
		t.Fatal("missing Create ACK received a build mutation", err, posts)
	}
	p.CreateJournalPath = createJournal
	if err := os.WriteFile(createJournal+".response", []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.StartTemplateBuild(context.Background()); !errors.Is(err, ErrConflict) || posts != 0 {
		t.Fatal("altered Create ACK received a build mutation", err, posts)
	}
	if _, err := os.Stat(p.JournalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("invalid Create proof wrote a build intent", err)
	}
}

func TestPrivateTemplateCreationRetainsExactIDsAndNeverReplays(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "creation.json")
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/v2/templates" {
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}
		posts++
		if r.Method != http.MethodPost || r.URL.Path != "/v3/templates" || r.Header.Get("X-API-Key") != "operator-test-key" {
			t.Error("unexpected template creation request")
		}
		var intent TemplateCreateIntent
		data, err := os.ReadFile(journal)
		if err != nil || json.Unmarshal(data, &intent) != nil || intent.Name != "luci-guest-0123456789abcdef0123456789abcdef" || intent.CPUCount != 2 || intent.MemoryMiB != 2048 {
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
		Name: "luci-guest-0123456789abcdef0123456789abcdef", CPUCount: 2, MemoryMiB: 2048, JournalPath: journal}
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
		if r.Method == http.MethodGet && r.URL.Path == "/v2/templates" {
			if posts == 0 {
				_ = json.NewEncoder(w).Encode([]any{})
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{"templateID": "template-one", "names": []string{"luci-guest-0123456789abcdef0123456789abcdef"}, "public": false,
				"builds": []map[string]any{{"buildID": "build-one", "cpuCount": 2, "memoryMB": 2048}}}})
			return
		}
		posts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	p := TemplateCreation{Endpoint: server.URL, APIKey: "operator-test-key", RootCAs: roots,
		Name: "luci-guest-0123456789abcdef0123456789abcdef", CPUCount: 2, MemoryMiB: 2048, JournalPath: journal}
	if _, err := p.CreateTemplate(context.Background()); !errors.Is(err, ErrUnknown) {
		t.Fatal("uncertain template creation appeared complete", err)
	}
	if _, err := p.CreateTemplate(context.Background()); !errors.Is(err, ErrConflict) || posts != 1 {
		t.Fatal("uncertain template creation was replayed", posts, err)
	}
	if _, err := p.ReadTemplateCreation(); !errors.Is(err, ErrUnknown) {
		t.Fatal("absent provider acknowledgement was fabricated", err)
	}
	if recovered, err := p.ReconcileTemplateCreation(context.Background()); err != nil || recovered.TemplateID != "template-one" || recovered.BuildID != "build-one" || posts != 1 {
		t.Fatal("exact read-only inventory did not settle lost acknowledgement", recovered, posts, err)
	}
	if _, err := p.CreateTemplate(context.Background()); !errors.Is(err, ErrConflict) || posts != 1 {
		t.Fatal("reconciled result caused a second provider Create", posts, err)
	}
}

func TestTemplateCreationReconcileRefusesAmbiguousProviderInventory(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "creation.json")
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if posts == 0 {
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}
		row := func(id string) map[string]any {
			return map[string]any{"templateID": id, "names": []string{"luci-guest-0123456789abcdef0123456789abcdef"}, "public": false,
				"builds": []map[string]any{{"buildID": "build-one", "cpuCount": 2, "memoryMB": 2048}}}
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{row("template-one"), row("template-two")})
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	p := TemplateCreation{Endpoint: server.URL, APIKey: "operator-test-key", RootCAs: roots,
		Name: "luci-guest-0123456789abcdef0123456789abcdef", CPUCount: 2, MemoryMiB: 2048, JournalPath: journal}
	_, _ = p.CreateTemplate(context.Background())
	if _, err := p.ReconcileTemplateCreation(context.Background()); !errors.Is(err, ErrUnknown) || posts != 1 {
		t.Fatal("ambiguous provider identity was adopted or Create replayed", posts, err)
	}
	if _, err := p.ReadTemplateCreation(); !errors.Is(err, ErrUnknown) {
		t.Fatal("ambiguous provider identity became a durable success", err)
	}
}

func TestTemplateCreationRefusesPreexistingNameBeforeIntentOrMutation(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "creation.json")
	posts := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts++
			w.WriteHeader(http.StatusAccepted)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{{"names": []string{"luci-guest-0123456789abcdef0123456789abcdef"}}})
	}))
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	p := TemplateCreation{Endpoint: server.URL, APIKey: "operator-test-key", RootCAs: roots,
		Name: "luci-guest-0123456789abcdef0123456789abcdef", CPUCount: 2, MemoryMiB: 2048, JournalPath: journal}
	if _, err := p.CreateTemplate(context.Background()); !errors.Is(err, ErrConflict) || posts != 0 {
		t.Fatal("preexisting template name was adopted or mutated", posts, err)
	}
	if _, err := os.Lstat(journal); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a rejected name persisted a fake Create intent", err)
	}
}
