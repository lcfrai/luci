package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testCoolifyToken = "42|test-secret-do-not-log"
const testCoolifyCommit = "0123456789abcdef0123456789abcdef01234567"

func TestCoolifyConnectionValidation(t *testing.T) {
	for _, address := range []string{
		"http://coolify.example.com", "http://localhost:8000", "http://192.168.1.2:8000",
		"https://user:secret@coolify.example.com", "https://coolify.example.com?token=secret",
		"https://coolify.example.com/#secret", "https://coolify.example.com/other",
		"https://coolify.example.com:65536", "https://bad_host.example.com",
		"https://coolify.example.com/api%2fv1",
	} {
		t.Run(address, func(t *testing.T) {
			client, err := NewCoolifyClient(address, testCoolifyToken)
			if err == nil || client != nil {
				t.Fatal("unsafe address accepted")
			}
			if strings.Contains(err.Error(), address) || strings.Contains(err.Error(), "secret") {
				t.Fatal("address leaked in error")
			}
		})
	}
	for _, address := range []string{"https://coolify.example.com", "https://coolify.example.com/api/v1/", "http://127.0.0.1:18000", "http://[::1]:18000/api/v1"} {
		if _, err := NewCoolifyClient(address, testCoolifyToken); err != nil {
			t.Fatalf("valid address rejected: %v", err)
		}
	}
	for _, token := range []string{"", "secret\nInjected: true", "token with space", "secret\x00", "token♥"} {
		if _, err := NewCoolifyClient("https://coolify.example.com", token); err == nil {
			t.Fatal("invalid token accepted")
		}
	}
}

func TestCoolifyGitReleaseJourney(t *testing.T) {
	var projects []CoolifyProject
	creates, apps, deploys := 0, 0, 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testCoolifyToken {
			t.Error("missing auth")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/teams/current":
			_, _ = w.Write([]byte(`{"id":7,"name":"Personal"}`))
		case "GET /api/v1/servers":
			_, _ = w.Write([]byte(`[{"uuid":"server1","name":"localhost","ip":"127.0.0.1"}]`))
		case "GET /api/v1/projects":
			_ = json.NewEncoder(w).Encode(projects)
		case "POST /api/v1/projects":
			creates++
			projects = []CoolifyProject{{UUID: "project1", Name: "Luci example"}}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"uuid":"project1"}`))
		case "POST /api/v1/applications/public":
			apps++
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["git_repository"] != "https://github.com/example/project.git" || body["git_commit_sha"] != testCoolifyCommit {
				t.Error("release source not pinned")
			}
			if body["instant_deploy"] != false || body["is_auto_deploy_enabled"] != false || body["force_domain_override"] != false {
				t.Error("creation must not deploy or override domain conflicts")
			}
			if body["ports_exposes"] != "3000" || body["build_pack"] != "dockerfile" || body["environment_name"] != "production" {
				t.Error("incorrect application contract")
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"uuid":"app1"}`))
		case "POST /api/v1/deploy":
			deploys++
			if r.URL.Query().Get("uuid") != "app1" || r.URL.Query().Get("force") != "false" {
				t.Error("incorrect deployment target")
			}
			_, _ = w.Write([]byte(`{"deployments":[{"resource_uuid":"app1","deployment_uuid":"deployment1"}]}`))
		case "GET /api/v1/deployments/deployment1":
			_ = json.NewEncoder(w).Encode(map[string]string{"deployment_uuid": "deployment1", "status": "finished", "commit": testCoolifyCommit, "logs": testCoolifyToken})
		default:
			t.Errorf("unexpected API call %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := NewCoolifyClient(server.URL, testCoolifyToken)
	if err != nil {
		t.Fatal(err)
	}
	client.http.Transport = server.Client().Transport
	ctx := context.Background()
	if err := client.Check(ctx); err != nil {
		t.Fatal(err)
	}
	servers, err := client.ListServers(ctx)
	if err != nil || len(servers) != 1 {
		t.Fatalf("server list: %v", err)
	}
	for i := 0; i < 2; i++ {
		p, err := client.EnsureProject(ctx, "Luci example")
		if err != nil || p.UUID != "project1" {
			t.Fatalf("project: %+v %v", p, err)
		}
	}
	app, err := client.CreateGitApplication(ctx, validCoolifyApplication())
	if err != nil || app != "app1" {
		t.Fatalf("application: %s %v", app, err)
	}
	deployment, err := client.Deploy(ctx, app)
	if err != nil || deployment.Status != "queued" {
		t.Fatalf("deploy: %+v %v", deployment, err)
	}
	deployment, err = client.Deployment(ctx, deployment.UUID)
	if err != nil || deployment.Status != "verifying" || deployment.Commit != testCoolifyCommit {
		t.Fatalf("provider success must still require health: %+v %v", deployment, err)
	}
	encoded, _ := json.Marshal(deployment)
	if strings.Contains(string(encoded), testCoolifyToken) {
		t.Fatal("raw logs leaked")
	}
	if creates != 1 || apps != 1 || deploys != 1 {
		t.Fatalf("duplicate mutations: %d/%d/%d", creates, apps, deploys)
	}
}

func validCoolifyApplication() CoolifyGitApplication {
	return CoolifyGitApplication{ProjectUUID: "project1", ServerUUID: "server1", Name: "Example", Repository: "https://github.com/example/project.git", Branch: "main", Commit: testCoolifyCommit, Port: 3000, Domain: "https://app.example.com"}
}

func TestCoolifyRejectsUntransferableSourceBeforeMutation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(500) }))
	defer server.Close()
	client, _ := NewCoolifyClient(server.URL, testCoolifyToken)
	for _, repository := range []string{"C:/local/project", "file:///tmp/project", "git@github.com:example/project.git", "https://token@github.com/example/project", "http://github.com/example/project", "https://127.0.0.1/example/project", "https://localhost/example/project", "https://github.com"} {
		app := validCoolifyApplication()
		app.Repository = repository
		if _, err := client.CreateGitApplication(context.Background(), app); err == nil {
			t.Fatalf("accepted unavailable/unsafe source: %s", repository)
		}
	}
	app := validCoolifyApplication()
	app.Commit = "main"
	if _, err := client.CreateGitApplication(context.Background(), app); err == nil {
		t.Fatal("mutable commit accepted")
	}
	if _, err := client.Deploy(context.Background(), "app1,app2"); err == nil {
		t.Fatal("multi-resource injection accepted")
	}
	if _, err := client.Deployment(context.Background(), "../teams/current"); err == nil {
		t.Fatal("path injection accepted")
	}
	if calls != 0 {
		t.Fatal("invalid source or identifiers caused remote calls")
	}
}

func TestCoolifyNeverFollowsRedirectsOrLeaksErrors(t *testing.T) {
	redirectCalls := 0
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirectCalls++ }))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer source.Close()
	client, _ := NewCoolifyClient(source.URL, testCoolifyToken)
	err := client.Check(context.Background())
	if err == nil || redirectCalls != 0 {
		t.Fatal("redirect followed")
	}
	if strings.Contains(err.Error(), source.URL) || strings.Contains(err.Error(), testCoolifyToken) {
		t.Fatal("connection details leaked")
	}
	for _, status := range []int{401, 403, 409, 422, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(testCoolifyToken + " " + source.URL))
			}))
			defer server.Close()
			c, _ := NewCoolifyClient(server.URL, testCoolifyToken)
			err := c.Check(context.Background())
			var apiErr *CoolifyAPIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != status {
				t.Fatalf("expected structured HTTP error: %v", err)
			}
			if strings.Contains(err.Error(), testCoolifyToken) || strings.Contains(err.Error(), source.URL) {
				t.Fatal("upstream body leaked")
			}
		})
	}
}

func TestCoolifyRejectsAmbiguousOrMalformedResponses(t *testing.T) {
	for _, body := range []string{"not json " + testCoolifyToken, `{"id":0}`, strings.Repeat("x", (4<<20)+1)} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		client, _ := NewCoolifyClient(server.URL, testCoolifyToken)
		err := client.Check(context.Background())
		server.Close()
		if err == nil || strings.Contains(err.Error(), testCoolifyToken) {
			t.Fatal("unsafe malformed-response handling")
		}
	}
	mutations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutations++
		}
		_, _ = w.Write([]byte(`[{"uuid":"one","name":"same"},{"uuid":"two","name":"same"}]`))
	}))
	defer server.Close()
	client, _ := NewCoolifyClient(server.URL, testCoolifyToken)
	if _, err := client.EnsureProject(context.Background(), "same"); err == nil || mutations != 0 {
		t.Fatal("ambiguous project selection caused a mutation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.Check(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
