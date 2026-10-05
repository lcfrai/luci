package product

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func cloudTestServer(t *testing.T, runner Runner) (*Server, *http.ServeMux) {
	t.Helper()
	s, err := New(Options{StateDir: filepath.Join(t.TempDir(), "state"), Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	s.Register(mux)
	t.Cleanup(s.Close)
	return s, mux
}
func cloudRequest(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}
func cloudPlanRequest(provider string) CloudRequest {
	return CloudRequest{Provider: provider, Name: "luci-test", Region: "australiaeast", Domain: "app.example.com", Size: "small", MonthlyBudgetUSD: 50, SSHPublicKey: "ssh-ed25519 " + base64.StdEncoding.EncodeToString(make([]byte, 40))}
}
func waitCloudJob(t *testing.T, s *Server, id string) CloudJob {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		j := s.jobsSnapshot(id)
		if j.Status != "creating" {
			return j
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("cloud operation did not settle")
	return CloudJob{}
}

func TestSetupReadOnlyAndRestartPersistence(t *testing.T) {
	s, mux := cloudTestServer(t, func(context.Context, string, []string, string) (string, error) {
		t.Fatal("read-only setup launched a process")
		return "", nil
	})
	for _, path := range []string{"/v1/setup/status", "/v1/setup/cloud/jobs", "/v1/setup/targets", "/v1/setup/ssh/connections"} {
		if w := cloudRequest(mux, "GET", path, ""); w.Code != 200 {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
	if s.Config().Mode != "local" {
		t.Fatal("fresh setup must not select billable cloud execution")
	}
	root := filepath.Join(t.TempDir(), "my-projects")
	body, _ := json.Marshal(Config{ProjectsDir: root, Mode: "local", MonthlyBudgetUSD: 30})
	if w := cloudRequest(mux, "POST", "/v1/setup/config", string(body)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	restarted, err := New(Options{StateDir: s.stateDir, Runner: s.runner})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if restarted.Config().ProjectsDir != root || !restarted.status().Configured {
		t.Fatal("settings did not survive restart")
	}
}

func TestCloudPlansDoNotProvisionAndIncludeCompleteResources(t *testing.T) {
	s, _ := cloudTestServer(t, func(_ context.Context, exe string, args []string, _ string) (string, error) {
		if exe != "aws" || strings.Join(args[:2], " ") != "lightsail get-bundles" {
			t.Fatalf("plan unexpectedly ran %s %v", exe, args)
		}
		return `{"bundles":[{"bundleId":"medium_3_0","price":24,"isActive":true}]}`, nil
	})
	for _, provider := range []string{"aws", "azure"} {
		req := cloudPlanRequest(provider)
		p, err := s.buildCloudPlan(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		var template map[string]any
		if len(p.Files) != 2 || json.Unmarshal([]byte(p.Files[1].Content), &template) != nil {
			t.Fatal("missing complete JSON template")
		}
		if provider == "aws" {
			if !p.Estimate.Verified || p.Estimate.Monthly == nil || *p.Estimate.Monthly != 24 {
				t.Fatal("quote not preserved")
			}
			resources := template["Resources"].(map[string]any)
			if len(resources) != 2 {
				t.Fatal("missing persistent public IP")
			}
		} else if len(template["resources"].([]any)) != 5 {
			t.Fatal("Azure network, disk and VM must be explicit")
		}
		if strings.Contains(p.Files[0].Content, "lcfr.ai/downloads") {
			t.Fatal("bootstrap depends on unpublished Luci release")
		}
		if !strings.Contains(p.Files[0].Content, "sha256sum --check") {
			t.Fatal("installer verification missing")
		}
	}
}

func TestCloudMissingOrInvalidQuoteCannotBypassPriceAcknowledgement(t *testing.T) {
	for _, quote := range []string{
		`{"bundleId":"medium_3_0","isActive":true}`,
		`{"bundleId":"medium_3_0","price":null,"isActive":true}`,
		`{"bundleId":"medium_3_0","price":0,"isActive":true}`,
		`{"bundleId":"medium_3_0","price":-24,"isActive":true}`,
		`{"bundleId":"medium_3_0","price":24,"isActive":false}`,
		`{"bundleId":"large_3_0","price":44,"isActive":true}`,
	} {
		t.Run(quote, func(t *testing.T) {
			calls := 0
			s, mux := cloudTestServer(t, func(_ context.Context, exe string, args []string, _ string) (string, error) {
				calls++
				if exe != "aws" || strings.Join(args[:2], " ") != "lightsail get-bundles" {
					t.Fatalf("unverified quote reached provisioning: %s %v", exe, args)
				}
				return `{"bundles":[` + quote + `]}`, nil
			})
			p, err := s.buildCloudPlan(context.Background(), cloudPlanRequest("aws"))
			if err != nil || p.Estimate.Verified || p.Estimate.Monthly != nil {
				t.Fatalf("missing or invalid quote became a verified price: %+v, %v", p.Estimate, err)
			}
			s.plans[p.ID] = p
			body, _ := json.Marshal(map[string]any{"planId": p.ID, "acknowledgeCosts": true})
			w := cloudRequest(mux, "POST", "/v1/setup/cloud/create", string(body))
			if w.Code != 400 || !strings.Contains(w.Body.String(), "quote_required") || len(s.jobs) != 0 || calls != 1 {
				t.Fatalf("incomplete quote bypassed acknowledgement: %d %s; jobs=%d calls=%d", w.Code, w.Body.String(), len(s.jobs), calls)
			}
		})
	}
}

func TestCloudPresetsDiscloseCapacityWithoutImplyingCompletePrice(t *testing.T) {
	s, _ := cloudTestServer(t, func(context.Context, string, []string, string) (string, error) {
		return "", errors.New("no account quote available")
	})
	for _, tc := range []struct{ provider, size, sku, memory, disk, cpu string }{
		{"aws", "small", "medium_3_0", "4 GB RAM", "80 GB", "2 vCPUs"},
		{"aws", "standard", "large_3_0", "8 GB RAM", "160 GB", "2 vCPUs"},
		{"aws", "full", "xlarge_3_0", "16 GB RAM", "320 GB", "4 vCPUs"},
		{"azure", "small", "Standard_B2als_v2", "4 GiB RAM", "64 GiB", "2 vCPUs"},
		{"azure", "standard", "Standard_B2as_v2", "8 GiB RAM", "64 GiB", "2 vCPUs"},
		{"azure", "full", "Standard_B4als_v2", "8 GiB RAM", "64 GiB", "4 vCPUs"},
	} {
		t.Run(tc.provider+"/"+tc.size, func(t *testing.T) {
			req := cloudPlanRequest(tc.provider)
			req.Size = tc.size
			if err := validateCloud(req); err != nil {
				t.Fatalf("Disclosed preset cannot be selected: %v", err)
			}
			p, err := s.buildCloudPlan(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			resources := strings.Join(p.Resources, " ")
			for _, expected := range []string{tc.sku, tc.cpu, tc.memory, tc.disk} {
				if !strings.Contains(resources, expected) {
					t.Fatalf("review omits selected capacity %q: %s", expected, resources)
				}
			}
			if !strings.Contains(p.Files[1].Content, `"`+tc.sku+`"`) || p.Estimate.Verified || p.Estimate.Monthly != nil {
				t.Fatalf("template and disclosed choice/quote disagree: %+v", p)
			}
			warnings := strings.Join(p.Warnings, " ")
			if !strings.Contains(warnings, "not a provider billing cap") || (tc.size == "small" && !strings.Contains(warnings, "one active build or agent session")) {
				t.Fatalf("review omits cost or capacity limits: %s", warnings)
			}
		})
	}
}

func TestCloudCreationRequiresCostAcknowledgementAndResumesByIdentity(t *testing.T) {
	var mu sync.Mutex
	creates := 0
	runner := func(_ context.Context, exe string, args []string, _ string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		switch strings.Join(args[:2], " ") {
		case "sts get-caller-identity":
			return `{}`, nil
		case "cloudformation list-stacks":
			return `{"StackSummaries":[]}`, nil
		case "cloudformation validate-template":
			return `{}`, nil
		case "cloudformation create-stack":
			creates++
			return `{"StackId":"arn:aws:cloudformation:stack/luci-test/one"}`, nil
		}
		return "", errors.New("unexpected command")
	}
	s, mux := cloudTestServer(t, runner)
	p := CloudPlan{ID: "cp-test", Provider: "aws", Name: "luci-test", Region: "ap-southeast-2", Domain: "app.example.com", CreatedAt: time.Now(), MonthlyBudgetUSD: 50, Files: []PlanFile{{Name: "template.json", Content: "{}"}}}
	s.plans[p.ID] = p
	for _, body := range []string{`{"planId":"cp-test"}`, `{"planId":"cp-test","acknowledgeCosts":true}`} {
		if w := cloudRequest(mux, "POST", "/v1/setup/cloud/create", body); w.Code != 400 {
			t.Fatal("missing acknowledgement accepted", w.Code)
		}
	}
	body := `{"planId":"cp-test","acknowledgeCosts":true,"acknowledgeUnverifiedPrice":true}`
	w := cloudRequest(mux, "POST", "/v1/setup/cloud/create", body)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var j CloudJob
	_ = json.Unmarshal(w.Body.Bytes(), &j)
	j = waitCloudJob(t, s, j.ID)
	if j.Status != "provisioning" || !j.Owned || j.ResourceID == "" {
		t.Fatal(j)
	}
	if w = cloudRequest(mux, "POST", "/v1/setup/cloud/create", body); w.Code != 200 {
		t.Fatal("retry created duplicate")
	}
	mu.Lock()
	if creates != 1 {
		t.Fatal("duplicate create", creates)
	}
	mu.Unlock()
	restarted, err := New(Options{StateDir: s.stateDir, Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	record := restarted.jobsSnapshot(j.ID)
	if !record.Owned || record.ResourceID != j.ResourceID {
		t.Fatal("ownership lost on restart")
	}
}

func TestCloudRefusesExistingGroupsAndUnownedDeletion(t *testing.T) {
	s, mux := cloudTestServer(t, func(_ context.Context, _ string, args []string, _ string) (string, error) {
		switch strings.Join(args[:2], " ") {
		case "account show":
			return `{}`, nil
		case "group exists":
			return "true", nil
		}
		t.Fatalf("modified existing resources: %v", args)
		return "", nil
	})
	p := CloudPlan{ID: "cp-azure", Provider: "azure", Name: "luci-test", Region: "australiaeast", CreatedAt: time.Now(), MonthlyBudgetUSD: 50}
	s.plans[p.ID] = p
	w := cloudRequest(mux, "POST", "/v1/setup/cloud/create", `{"planId":"cp-azure","acknowledgeCosts":true,"acknowledgeUnverifiedPrice":true}`)
	var j CloudJob
	_ = json.Unmarshal(w.Body.Bytes(), &j)
	j = waitCloudJob(t, s, j.ID)
	if j.Status != "failed" || j.Owned {
		t.Fatal("existing group adopted", j)
	}
	w = cloudRequest(mux, "POST", "/v1/setup/cloud/jobs/"+j.ID+"/delete", `{"confirmName":"luci-test"}`)
	if w.Code != 409 {
		t.Fatal("unowned resource deletion accepted")
	}
}

func TestSSHHostTrustAndCredentialRedaction(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/servers" || r.Header.Get("Authorization") != "Bearer top-secret" {
			t.Error("wrong target request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"uuid":"server-one","name":"Local","ip":"127.0.0.1"}]`))
	}))
	defer api.Close()
	s, mux := cloudTestServer(t, func(_ context.Context, exe string, _ []string, _ string) (string, error) {
		if exe != "ssh-keyscan" {
			t.Fatal("unverified SSH was launched")
		}
		return "203.0.113.7 ssh-ed25519 " + base64.StdEncoding.EncodeToString(make([]byte, 40)) + "\n", nil
	})
	req, _ := json.Marshal(map[string]string{"name": "My server", "baseUrl": api.URL, "token": "top-secret"})
	w := cloudRequest(mux, "POST", "/v1/setup/targets", string(req))
	if w.Code != 201 || strings.Contains(w.Body.String(), "top-secret") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = cloudRequest(mux, "GET", "/v1/setup/targets", "")
	if strings.Contains(w.Body.String(), "top-secret") {
		t.Fatal("GET leaked credential")
	}
	w = cloudRequest(mux, "POST", "/v1/setup/ssh/inspect", `{"host":"203.0.113.7"}`)
	var scan SSHScan
	_ = json.Unmarshal(w.Body.Bytes(), &scan)
	if w.Code != 200 || len(scan.Fingerprints) != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = cloudRequest(mux, "POST", "/v1/setup/ssh/connect", `{"scanId":"`+scan.ID+`","acceptedFingerprint":"wrong"}`)
	if w.Code != 409 {
		t.Fatal("unverified host accepted")
	}
	restarted, err := New(Options{StateDir: s.stateDir, Runner: s.runner})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restarted.mu.RLock()
	defer restarted.mu.RUnlock()
	for _, target := range restarted.targets {
		if target.Token != "top-secret" {
			t.Fatal("credential did not persist privately")
		}
	}
}
