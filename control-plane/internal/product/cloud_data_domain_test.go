package product

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

type dataDomainProviderFixture struct {
	mu                                                        sync.Mutex
	projects                                                  []CoolifyProject
	service                                                   *coolifyService
	projectCreates, creates, updates, starts, restarts, stops int
	lostProject, lostCreate, lostUpdate, lostStart            bool
	noCreate, missingProof, duplicate                         bool
	noProject                                                 bool
	serverID                                                  string
	beforeCreate                                              func()
}

func domainBool(v bool) *bool { return &v }
func (f *dataDomainProviderFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+testCoolifyToken {
		w.WriteHeader(401)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1")
	switch {
	case path == "/servers":
		settings := &CoolifyServerSettings{BuildOnly: domainBool(false), SwarmManager: domainBool(false), SwarmWorker: domainBool(false), Usable: domainBool(true), Reachable: domainBool(true)}
		if f.missingProof {
			settings.SwarmWorker = nil
		}
		writeJSON(w, 200, []CoolifyServer{{UUID: "server-data", IP: "host.docker.internal", Settings: settings}})
	case path == "/projects" && r.Method == "GET":
		writeJSON(w, 200, f.projects)
	case strings.HasPrefix(path, "/projects/") && r.Method == "GET":
		id := strings.TrimPrefix(path, "/projects/")
		for _, p := range f.projects {
			if p.UUID == id {
				writeJSON(w, 200, p)
				return
			}
		}
		w.WriteHeader(404)
	case path == "/projects" && r.Method == "POST":
		var p CoolifyProject
		_ = json.NewDecoder(r.Body).Decode(&p)
		f.projectCreates++
		p.UUID = "provider-project"
		if f.noProject {
			w.WriteHeader(500)
			return
		}
		f.projects = append(f.projects, p)
		if f.lostProject {
			f.lostProject = false
			w.WriteHeader(500)
			return
		}
		writeJSON(w, 201, p)
	case path == "/services" && r.Method == "GET":
		list := []coolifyService{}
		if f.service != nil {
			list = append(list, *f.service)
			if f.duplicate {
				other := *f.service
				other.UUID = "foreign-service"
				list = append(list, other)
			}
		}
		writeJSON(w, 200, list)
	case path == "/services" && r.Method == "POST":
		if f.beforeCreate != nil {
			f.beforeCreate()
		}
		f.creates++
		var input struct {
			Name, Description string
			Compose           string                       `json:"docker_compose_raw"`
			Server            string                       `json:"server_uuid"`
			URLs              []struct{ Name, URL string } `json:"urls"`
			Instant           bool                         `json:"instant_deploy"`
			Force             bool                         `json:"force_domain_override"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil || input.Instant || input.Force || input.Server != "server-data" || len(input.URLs) != 1 {
			w.WriteHeader(400)
			return
		}
		if !f.noCreate {
			b, _ := base64.StdEncoding.DecodeString(input.Compose)
			f.service = &coolifyService{UUID: "provider-service", Name: input.Name, Description: input.Description, Compose: string(b), Status: "stopped:unknown"}
			f.service.Destination.Server.UUID = input.Server
			f.service.Applications = append(f.service.Applications, struct {
				Name string `json:"name"`
				FQDN string `json:"fqdn"`
			}{input.URLs[0].Name, input.URLs[0].URL})
		}
		if f.lostCreate || f.noCreate {
			f.lostCreate = false
			w.WriteHeader(500)
			return
		}
		writeJSON(w, 201, map[string]string{"uuid": "provider-service"})
	case path == "/services/provider-service" && r.Method == "GET":
		if f.service == nil {
			w.WriteHeader(404)
			return
		}
		observed := *f.service
		if f.serverID != "" {
			observed.Destination.Server.UUID = f.serverID
		}
		writeJSON(w, 200, observed)
	case path == "/services/provider-service" && r.Method == "PATCH":
		var input struct {
			Compose string                       `json:"docker_compose_raw"`
			URLs    []struct{ Name, URL string } `json:"urls"`
			Force   bool                         `json:"force_domain_override"`
		}
		if f.service == nil || json.NewDecoder(r.Body).Decode(&input) != nil || input.Force || len(input.URLs) != 1 {
			w.WriteHeader(400)
			return
		}
		f.updates++
		b, _ := base64.StdEncoding.DecodeString(input.Compose)
		f.service.Compose = string(b)
		f.service.Applications[0].FQDN = input.URLs[0].URL
		if f.lostUpdate {
			f.lostUpdate = false
			w.WriteHeader(500)
			return
		}
		writeJSON(w, 200, map[string]string{"uuid": "provider-service"})
	case path == "/services/provider-service/start" || path == "/services/provider-service/restart":
		if strings.HasSuffix(path, "/start") {
			f.starts++
		} else {
			if r.URL.Query().Get("latest") != "false" {
				w.WriteHeader(400)
				return
			}
			f.restarts++
		}
		if f.service != nil {
			f.service.Status = "running:healthy"
		}
		if f.lostStart {
			f.lostStart = false
			w.WriteHeader(500)
			return
		}
		writeJSON(w, 200, map[string]string{"message": "queued"})
	case path == "/services/provider-service/stop":
		if r.URL.Query().Get("docker_cleanup") != "false" {
			w.WriteHeader(400)
			return
		}
		f.stops++
		f.service.Status = "stopped:unknown"
		writeJSON(w, 200, map[string]string{"message": "queued"})
	default:
		w.WriteHeader(404)
	}
}

func dataDomainAnon(role string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"role":"`+role+`"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte("fixture-signature-not-a-real-credential"))
}
func dataDomainFixture(t *testing.T, f *dataDomainProviderFixture) (*Server, PublicDataDomainRequest, PublicDataDomainPrerequisites) {
	t.Helper()
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "public-data")
	remote := httptest.NewServer(f)
	t.Cleanup(remote.Close)
	s.targets["target-data"] = DeploymentTarget{ID: "target-data", Name: "Data", BaseURL: remote.URL, Token: testCoolifyToken, ServerUUID: "server-data"}
	req := PublicDataDomainRequest{ProjectID: p.ID, StableTenant: "tenant-data", TargetID: "target-data", ServerUUID: "server-data", Domain: "https://api.example.com", AppOrigin: "https://app.example.com", AcknowledgePublic: true}
	pre := PublicDataDomainPrerequisites{Data: PublicDataHost{ProjectID: p.ID, StableTenant: req.StableTenant, ApplicationNetwork: "owned-project-network", NetworkID: "verified-network-id", DockerEngineID: "verified-engine-id", PublishableKey: dataDomainAnon("anon")}, VerifyHost: func(context.Context, CoolifyServer) error { return nil }, VerifyHTTPS: func(_ context.Context, b PublicDataDomainBinding) error {
		if len(b.ConfigurationDigest) != 64 || b.ProbePath != "/__luci/data-health" {
			return errors.New("missing running-configuration proof")
		}
		return nil
	}}
	return s, req, pre
}

func TestPublicDataDomainLostCreateReadbackAndRestartPreservesOwnedReceipt(t *testing.T) {
	f := &dataDomainProviderFixture{lostProject: true, lostCreate: true, lostStart: true}
	s, req, pre := dataDomainFixture(t, f)
	f.beforeCreate = func() {
		v, e := s.readDataDomain(req.ProjectID)
		if e != nil || !v.CreateAttempted || v.Status != "uncertain" || v.Owner == "" {
			t.Error("create occurred before durable ownership intent")
		}
	}
	first, err := s.PublishDataDomain(context.Background(), req, pre)
	if err != nil || first.Status != "uncertain" || first.ProviderServiceUUID != "provider-service" {
		t.Fatalf("unexpected lost activation receipt: %+v %v", first, err)
	}
	ready, err := s.RefreshDataDomain(context.Background(), req.ProjectID, pre)
	if err != nil || ready.Status != "verifiedHTTPS" {
		t.Fatalf("readback did not recover: %+v %v", ready, err)
	}
	_, err = s.PublishDataDomain(context.Background(), req, pre)
	if err != nil {
		t.Fatal(err)
	}
	if f.projectCreates != 1 || f.creates != 1 || f.starts != 1 || f.restarts != 0 {
		t.Fatal("duplicate creation/activation", f.projectCreates, f.creates, f.starts, f.restarts)
	}
	file, _ := s.privateApplicationStatePath("data-domain-" + req.ProjectID)
	b, _ := os.ReadFile(file)
	if strings.Contains(string(b), testCoolifyToken) || strings.Contains(string(b), pre.Data.PublishableKey) || strings.Contains(string(b), s.targets[req.TargetID].BaseURL) {
		t.Fatal("secret/connection value entered ownership receipt")
	}
	if _, e := os.Stat(filepath.Join(s.projectRoot(), "data-domain-"+req.ProjectID+".json")); !os.IsNotExist(e) {
		t.Fatal("receipt entered project tree")
	}
	// A new process must reconcile the same attempt, never create another.
	reopened, e := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	reopened.targets = s.targets
	again, e := reopened.RefreshDataDomain(context.Background(), req.ProjectID, pre)
	if e != nil || again.ProviderServiceUUID != ready.ProviderServiceUUID || f.creates != 1 {
		t.Fatal("restart lost ownership", again, e)
	}
}

func TestPublicDataDomainUnconfirmedCreationNeverRepeatsAndRefreshNeverMutates(t *testing.T) {
	f := &dataDomainProviderFixture{noCreate: true}
	s, req, pre := dataDomainFixture(t, f)
	for i := 0; i < 2; i++ {
		row, e := s.PublishDataDomain(context.Background(), req, pre)
		if e != nil || row.Status != "uncertain" {
			t.Fatal(row, e)
		}
	}
	row, e := s.RefreshDataDomain(context.Background(), req.ProjectID, pre)
	if e != nil || row.Status != "uncertain" || f.creates != 1 || f.starts != 0 || f.projectCreates != 1 {
		t.Fatal("ambiguous mutation repeated", row, e, f.creates, f.starts)
	}
}

func TestPublicDataDomainUnconfirmedProjectCreationNeverRepeats(t *testing.T) {
	f := &dataDomainProviderFixture{noProject: true}
	s, req, pre := dataDomainFixture(t, f)
	for i := 0; i < 2; i++ {
		row, e := s.PublishDataDomain(context.Background(), req, pre)
		if e != nil || row.Status != "uncertain" {
			t.Fatal(row, e)
		}
	}
	if f.projectCreates != 1 || f.creates != 0 {
		t.Fatal("ambiguous project creation was repeated or service was allocated", f.projectCreates, f.creates)
	}
}

func TestPublicDataDomainConcurrentPublishesReserveOneIngress(t *testing.T) {
	f := &dataDomainProviderFixture{}
	s, req, pre := dataDomainFixture(t, f)
	var wait sync.WaitGroup
	for i := 0; i < 4; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			v, e := s.PublishDataDomain(context.Background(), req, pre)
			if e != nil || v.Status != "verifiedHTTPS" {
				t.Errorf("publish: %+v %v", v, e)
			}
		}()
	}
	wait.Wait()
	if f.projectCreates != 1 || f.creates != 1 || f.starts != 1 {
		t.Fatal("concurrent requests duplicated owned resource", f.projectCreates, f.creates, f.starts)
	}
}

func TestPublicDataDomainRestoreClientKeyRequiresExplicitOwnedUpdate(t *testing.T) {
	f := &dataDomainProviderFixture{}
	s, req, pre := dataDomainFixture(t, f)
	initial, e := s.PublishDataDomain(context.Background(), req, pre)
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(pre.Data.PublishableKey, ".")
	parts[1] = base64.RawURLEncoding.EncodeToString([]byte(`{"role":"anon","restore":"backup-identity"}`))
	pre.Data.PublishableKey = strings.Join(parts, ".")
	row, e := s.RefreshDataDomain(context.Background(), req.ProjectID, pre)
	if e != nil || row.Status != "attention" || f.updates != 0 {
		t.Fatal("refresh silently changed restored key policy", row, e)
	}
	req.ExpectedRevision = initial.Revision
	updated, e := s.PublishDataDomain(context.Background(), req, pre)
	if e != nil || updated.Status != "verifiedHTTPS" || updated.ProviderServiceUUID != initial.ProviderServiceUUID || updated.Domain != initial.Domain || updated.ConfigurationDigest == initial.ConfigurationDigest || f.updates != 1 || f.creates != 1 {
		t.Fatal("stable restored identity did not reconcile", updated, e)
	}
	pre.Data.PublishableKey = dataDomainAnon("anon")
	req.ExpectedRevision = updated.Revision
	rollback, e := s.PublishDataDomain(context.Background(), req, pre)
	if e != nil || rollback.Status != "verifiedHTTPS" || rollback.ConfigurationDigest != initial.ConfigurationDigest || f.updates != 2 || f.creates != 1 {
		t.Fatal("previous policy was not reinstated on the same route", rollback, e)
	}
}

func TestPublicDataDomainRequiresPositiveHostProofAndPublicAcknowledgement(t *testing.T) {
	for _, name := range []string{"ack", "host", "missing-metadata", "key", "engine", "same-origin", "credentials", "local-domain"} {
		t.Run(name, func(t *testing.T) {
			f := &dataDomainProviderFixture{}
			s, req, pre := dataDomainFixture(t, f)
			switch name {
			case "ack":
				req.AcknowledgePublic = false
			case "host":
				pre.VerifyHost = func(context.Context, CoolifyServer) error { return errors.New("unrelated engine") }
			case "missing-metadata":
				f.missingProof = true
			case "key":
				pre.Data.PublishableKey = dataDomainAnon("service_role")
			case "engine":
				pre.Data.DockerEngineID = ""
			case "same-origin":
				req.AppOrigin = req.Domain
			case "credentials":
				req.Domain = "https://user:secret@api.example.com"
			case "local-domain":
				req.Domain = "https://api.localhost"
			}
			if _, e := s.PublishDataDomain(context.Background(), req, pre); e == nil {
				t.Fatal("invalid prerequisite accepted")
			}
			if f.projectCreates+f.creates+f.starts+f.restarts+f.stops != 0 {
				t.Fatal("mutation happened before proof")
			}
		})
	}
}

func TestPublicDataDomainOwnedUpdateStopAndPublicHealthRecovery(t *testing.T) {
	f := &dataDomainProviderFixture{}
	s, req, pre := dataDomainFixture(t, f)
	ready, e := s.PublishDataDomain(context.Background(), req, pre)
	if e != nil || ready.Status != "verifiedHTTPS" {
		t.Fatal(ready, e)
	}
	req.ExpectedRevision = ready.Revision
	req.AppOrigin = "https://new-app.example.com"
	f.lostUpdate = true
	pre.VerifyHTTPS = func(context.Context, PublicDataDomainBinding) error {
		return errors.New("old ingress digest still serving")
	}
	pending, e := s.PublishDataDomain(context.Background(), req, pre)
	if e != nil || pending.Status != "pending" || pending.Revision != 2 || f.updates != 1 || f.restarts != 1 || pending.ConfigurationDigest == ready.ConfigurationDigest {
		t.Fatal("update readiness fabricated", pending, e)
	}
	pre.VerifyHTTPS = func(context.Context, PublicDataDomainBinding) error { return nil }
	ready, e = s.RefreshDataDomain(context.Background(), req.ProjectID, pre)
	if e != nil || ready.Status != "verifiedHTTPS" || f.updates != 1 || f.restarts != 1 {
		t.Fatal("refresh mutated ingress", ready, e)
	}
	stopped, e := s.StopDataDomain(context.Background(), req.ProjectID, pre)
	if e != nil || stopped.Status != "stopped" || f.stops != 1 {
		t.Fatal(stopped, e)
	}
	_, _ = s.RefreshDataDomain(context.Background(), req.ProjectID, pre)
	if f.stops != 1 || f.starts != 1 || f.creates != 1 {
		t.Fatal("refresh repeated stop or touched native stack")
	}
}

func TestPublicDataDomainRejectsReplacedProviderOwnershipAndChangedHost(t *testing.T) {
	for _, name := range []string{"server", "compose", "uuid", "domain", "engine"} {
		t.Run(name, func(t *testing.T) {
			f := &dataDomainProviderFixture{}
			s, req, pre := dataDomainFixture(t, f)
			first, e := s.PublishDataDomain(context.Background(), req, pre)
			if e != nil || first.Status != "verifiedHTTPS" {
				t.Fatal(first, e)
			}
			switch name {
			case "server":
				f.serverID = "foreign-server"
			case "compose":
				f.service.Compose = "services: {foreign: {image: alpine}}"
			case "uuid":
				f.service.UUID = "foreign-service"
			case "domain":
				f.service.Applications[0].FQDN = "https://foreign.example.com:8080"
			case "engine":
				pre.Data.DockerEngineID = "replacement-engine"
			}
			row, _ := s.RefreshDataDomain(context.Background(), req.ProjectID, pre)
			if row.Status != "attention" {
				t.Fatal("replacement retained public readiness", row)
			}
			_, _ = s.StopDataDomain(context.Background(), req.ProjectID, pre)
			if f.stops != 0 || f.creates != 1 || f.starts != 1 || f.restarts != 0 {
				t.Fatal("foreign resource was mutated")
			}
		})
	}
}

func TestDataIngressCompositionIsPrivatePinnedAndCanonicalAcrossYAML(t *testing.T) {
	v := dataDomainRecord{PublicDataDomainBinding: PublicDataDomainBinding{ProjectID: "p-test", StableTenant: "tenant-test", ApplicationNetwork: "native-test", Domain: "https://api.example.com", AppOrigin: "https://app.example.com", ProbePath: "/__luci/data-health", ConfigurationDigest: strings.Repeat("a", 64)}, Owner: "owner-test"}
	raw, e := dataIngressCompose(v, dataDomainAnon("anon"))
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]any
	if e = json.Unmarshal([]byte(raw), &m); e != nil {
		t.Fatal(e)
	}
	service := m["services"].(map[string]any)["data-ingress"].(map[string]any)
	if service["image"] != dataIngressImage || service["read_only"] != true || service["user"] != "101:101" || service["ports"] != nil || service["volumes"] != nil || service["privileged"] != nil || service["environment"] != nil {
		t.Fatal("ingress gained unsafe deployment inputs")
	}
	yamlBytes, e := yaml.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := dataComposeHash(raw)
	b, _ := dataComposeHash(string(yamlBytes))
	if a != b {
		t.Fatal("provider YAML formatting changed ownership digest")
	}
	content := m["configs"].(map[string]any)["luci-data-client"].(map[string]any)["content"].(string)
	if strings.Contains(content, "SERVICE_ROLE") || strings.Contains(content, "DASHBOARD_PASSWORD") || strings.Contains(raw, "docker.sock") {
		t.Fatal("ingress configuration contains an admin binding")
	}
	var envoy map[string]any
	if json.Unmarshal([]byte(content), &envoy) != nil || envoy["admin"] != nil {
		t.Fatal("public Envoy admin listener was enabled")
	}
}
