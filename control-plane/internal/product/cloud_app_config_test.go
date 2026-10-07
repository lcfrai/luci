package product

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type runtimeEnvironmentFixture struct {
	mu                                                            sync.Mutex
	variables                                                     []coolifyRuntimeVariable
	values                                                        map[string]string
	mutations, deletes                                            int
	lostWrite, lostDelete, ignoreBuildFlag, ignoreClientBuildFlag bool
}

func environmentBool(value bool) *bool { return &value }
func (f *runtimeEnvironmentFixture) handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const base = "/api/v1/applications/remote-app/envs"
		if r.URL.Path != base && !strings.HasPrefix(r.URL.Path, base+"/") {
			next.ServeHTTP(w, r)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.values == nil {
			f.values = map[string]string{}
		}
		if r.Header.Get("Authorization") != "Bearer "+testCoolifyToken {
			w.WriteHeader(401)
			return
		}
		if r.Method == "GET" {
			if f.variables == nil {
				f.variables = []coolifyRuntimeVariable{}
			}
			writeJSON(w, 200, f.variables)
			return
		}
		if r.Method == "DELETE" {
			uuid := strings.TrimPrefix(r.URL.Path, base+"/")
			for i, variable := range f.variables {
				if variable.UUID == uuid {
					f.variables = append(f.variables[:i], f.variables[i+1:]...)
					delete(f.values, uuid)
					f.deletes++
					if f.lostDelete {
						f.lostDelete = false
						w.WriteHeader(500)
						return
					}
					writeJSON(w, 200, map[string]string{"message": "deleted"})
					return
				}
			}
			w.WriteHeader(404)
			return
		}
		var input struct {
			Key     string `json:"key"`
			Value   string `json:"value"`
			Preview bool   `json:"is_preview"`
			Literal bool   `json:"is_literal"`
			Runtime bool   `json:"is_runtime"`
			Build   bool   `json:"is_buildtime"`
			Hidden  bool   `json:"is_shown_once"`
			Comment string `json:"comment"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil || !input.Hidden || input.Preview {
			w.WriteHeader(400)
			return
		}
		index := -1
		for i, variable := range f.variables {
			if !variable.IsPreview && variable.Key == input.Key {
				index = i
				break
			}
		}
		if r.Method == "POST" && index >= 0 {
			w.WriteHeader(409)
			return
		}
		if r.Method == "PATCH" && index < 0 {
			w.WriteHeader(404)
			return
		}
		f.mutations++
		build := input.Build || f.ignoreBuildFlag
		if f.ignoreClientBuildFlag && input.Build {
			build = false
		}
		variable := coolifyRuntimeVariable{UUID: fmt.Sprint("env-", f.mutations), Key: input.Key, IsLiteral: input.Literal, IsRuntime: environmentBool(input.Runtime), IsBuildtime: environmentBool(build), Comment: input.Comment}
		if index >= 0 {
			variable.UUID = f.variables[index].UUID
			f.variables[index] = variable
		} else {
			f.variables = append(f.variables, variable)
		}
		f.values[variable.UUID] = input.Value
		if index < 0 {
			foundPreview := false
			for _, previous := range f.variables {
				if previous.Key == input.Key && previous.IsPreview {
					foundPreview = true
				}
			}
			if !foundPreview {
				preview := variable
				preview.UUID += "-preview"
				preview.IsPreview = true
				f.variables = append(f.variables, preview)
				f.values[preview.UUID] = input.Value
			}
		}
		if f.lostWrite {
			f.lostWrite = false
			w.WriteHeader(500)
			_, _ = w.Write([]byte(input.Value))
			return
		}
		writeJSON(w, 201, map[string]string{"uuid": variable.UUID})
	})
}

func TestCloudApplicationConfigurationReconcilesLostResponsesAndPreservesManualVariables(t *testing.T) {
	provider := &cloudProviderFixture{}
	s, _, p, req, _ := cloudReleaseFixture(t, provider)
	f := &runtimeEnvironmentFixture{lostWrite: true, lostDelete: true, variables: []coolifyRuntimeVariable{{UUID: "manual", Key: "MANUAL"}, {UUID: "manual-preview", Key: "MANUAL_PREVIEW", IsPreview: true}}}
	remote := httptest.NewServer(f.handler(provider.handler(t)))
	defer remote.Close()
	client, _ := NewCoolifyClient(remote.URL, testCoolifyToken)
	target, _ := s.deploymentTarget(req.TargetID)
	row := CloudRelease{Release: Release{ProjectID: p.ID, ConfigurationRevision: 1}, ApplicationID: "remote-app", TargetFingerprint: targetFingerprint(target)}
	snapshot := ApplicationConfigurationSnapshot{Revision: 1, Values: map[string]string{"DATABASE_URL": "private-provider-fixture-$literal"}}
	if err := s.reconcileCloudApplicationConfiguration(context.Background(), row, client, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := s.reconcileCloudApplicationConfiguration(context.Background(), row, client, snapshot); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	if f.mutations != 1 || f.deletes != 1 || len(f.variables) != 3 {
		t.Fatal("lost acknowledgement duplicated mutation or manual variables changed", f.mutations, f.deletes, len(f.variables))
	}
	f.mu.Unlock()
	row.ConfigurationRevision = 2
	snapshot = ApplicationConfigurationSnapshot{Revision: 2, Values: map[string]string{}}
	if err := s.reconcileCloudApplicationConfiguration(context.Background(), row, client, snapshot); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	if len(f.variables) != 2 || f.variables[0].UUID != "manual" || f.variables[1].UUID != "manual-preview" {
		t.Fatal("manual provider configuration was removed")
	}
	before := f.mutations
	f.mu.Unlock()
	row.ConfigurationRevision = 3
	if err := s.reconcileCloudApplicationConfiguration(context.Background(), row, client, ApplicationConfigurationSnapshot{Revision: 3, Values: map[string]string{"MANUAL": "must-not-replace"}}); err == nil {
		t.Fatal("manual key was adopted")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.mutations != before {
		t.Fatal("collision mutated provider state")
	}
}

func TestCloudReleaseConfigurationRevisionControlsDeployAndRollback(t *testing.T) {
	provider := &cloudProviderFixture{status: "finished"}
	s, mux, p, req, _ := cloudReleaseFixture(t, provider)
	s.registerApplicationConfiguration(mux)
	f := &runtimeEnvironmentFixture{}
	remote := httptest.NewServer(f.handler(provider.handler(t)))
	defer remote.Close()
	s.mu.Lock()
	target := s.targets[req.TargetID]
	target.BaseURL = remote.URL
	s.targets[req.TargetID] = target
	s.mu.Unlock()
	base := "/v1/product/projects/" + p.ID
	save := func(revision int, value string) {
		journeyRequest(t, mux, "POST", base+"/configuration", map[string]any{"expectedRevision": revision, "updates": []map[string]any{{"name": "API_KEY", "value": value}}}, 200)
	}
	ready := func(row CloudRelease) {
		row.Status = "ready"
		if err := s.storeCloudRelease(row); err != nil {
			t.Fatal(err)
		}
	}
	save(0, "private-release-value-one")
	first := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/releases/cloud", req, 202))
	if first.ConfigurationRevision != 1 {
		t.Fatal("cloud revision was not pinned")
	}
	ready(first)
	save(1, "private-release-value-two")
	second := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/releases/cloud", req, 202))
	if second.ID == first.ID || second.ConfigurationRevision != 2 || second.Release.Commit != first.Release.Commit {
		t.Fatal("changed configuration reused old deployment")
	}
	duplicate := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/releases/cloud", req, 202))
	if duplicate.ID != second.ID {
		t.Fatal("identical configuration duplicated deployment")
	}
	ready(second)
	f.mu.Lock()
	f.ignoreBuildFlag = true
	f.mu.Unlock()
	save(2, "private-release-failure-value")
	failureResponse := journeyRequest(t, mux, "POST", base+"/releases/cloud", req, 502)
	failed := decodeCloudRelease(t, failureResponse)
	if failed.Status != "failed" || strings.Contains(failureResponse.Body.String(), "private-release-") {
		t.Fatal("failed runtime-only guard leaked values or blocked safe retry")
	}
	provider.mu.Lock()
	count := provider.deployments
	provider.mu.Unlock()
	if count != 2 {
		t.Fatal("build-time configuration was deployed")
	}
	f.mu.Lock()
	f.ignoreBuildFlag = false
	f.mu.Unlock()
	save(3, "private-release-fixed-value")
	fixed := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/releases/cloud", req, 202))
	if fixed.ConfigurationRevision != 4 || fixed.ID == failed.ID {
		t.Fatal("fixed configuration did not reserve a fresh attempt")
	}
	ready(fixed)
	rollback := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/releases/"+first.ID+"/rollback", map[string]any{}, 202))
	if rollback.ConfigurationRevision != 4 || rollback.RollbackOf != first.ID || !strings.Contains(rollback.Message, "Application data is not restored") {
		t.Fatal("rollback misrepresented configuration or data")
	}
	ready(rollback)
	save(4, "private-release-current-value")
	newRollback := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/releases/"+first.ID+"/rollback", map[string]any{}, 202))
	if newRollback.ID == rollback.ID || newRollback.ConfigurationRevision != 5 {
		t.Fatal("rollback deduplicated changed runtime configuration")
	}
	history, err := os.ReadFile(s.cloudReleaseFile(p.ID))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(history), "private-release-") {
		t.Fatal("runtime values leaked into release records")
	}
	listed := journeyRequest(t, mux, "GET", base+"/releases", nil, 200)
	if strings.Contains(listed.Body.String(), "private-release-") || !strings.Contains(listed.Body.String(), `"configurationRevision":5`) {
		t.Fatal("release metadata lost revision or exposed values")
	}
}

func cloudDataTestAnon(nonce string) string {
	encode := func(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
	return encode(`{"alg":"HS256","typ":"JWT"}`) + "." + encode(`{"role":"anon","fixture":"`+nonce+`"}`) + "." + encode("fixture-signature")
}

func cloudDataTestSnapshot(p project.Project, req CloudReleaseRequest) CloudDataBindingSnapshot {
	return CloudDataBindingSnapshot{Installed: true, ProjectID: p.ID, StableTenant: "data-stable", TargetID: req.TargetID, TargetFingerprint: targetFingerprint(DeploymentTarget{ID: req.TargetID, ServerUUID: "server-test"}), ServerUUID: "server-test", DomainStatus: "verifiedHTTPS", DomainRevision: 1, ConfigurationDigest: strings.Repeat("a", 64), PublicURL: "https://data.example.com", ApplicationOrigin: req.Domain, PublishableKey: cloudDataTestAnon("first")}
}

func cloudFrameworkCommit(t *testing.T, s *Server, p project.Project, req *CloudReleaseRequest, framework string) {
	t.Helper()
	content := "FROM scratch\n# Selected framework: " + framework + "\n"
	for _, name := range dataClientAliases(framework) {
		content += "ARG " + name + "\n"
	}
	if len(dataClientAliases(framework)) > 0 {
		content += "RUN npm run build\n"
	}
	if err := os.WriteFile(filepath.Join(p.Path, "Dockerfile"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	commitPreviewFixture(t, p.Path)
	commit, err := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	req.Commit, req.DataClientFramework = strings.TrimSpace(commit), framework
}

func TestCloudDataReleaseRequiresCurrentVerifiedBindingAndCommittedBuildArgs(t *testing.T) {
	for _, name := range []string{"callback unavailable", "no domain", "pending", "wrong project", "wrong target", "wrong server", "wrong app origin", "private URL", "service-role key", "missing Dockerfile ARG", "private native credentials", "configuration collision"} {
		t.Run(name, func(t *testing.T) {
			provider := &cloudProviderFixture{}
			s, mux, p, req, _ := cloudReleaseFixture(t, provider)
			s.registerApplicationConfiguration(mux)
			cloudFrameworkCommit(t, s, p, &req, "vite")
			data := cloudDataTestSnapshot(p, req)
			s.cloudDataBindings = func(_ context.Context, input CloudDataBindingRequest) (CloudDataBindingSnapshot, error) {
				if input.ProjectID != p.ID || input.ServerUUID != "server-test" || input.TargetID != req.TargetID || input.Framework != "vite" || input.AppOrigin != req.Domain {
					t.Fatal("cloud binding request was not derived from the owned release")
				}
				return data, nil
			}
			switch name {
			case "callback unavailable":
				s.cloudDataBindings = nil
			case "no domain":
				data = CloudDataBindingSnapshot{Installed: true, DomainStatus: "not-configured"}
			case "pending":
				data.DomainStatus = "pending"
			case "wrong project":
				data.ProjectID = "foreign"
			case "wrong target":
				data.TargetID = "other-target"
			case "wrong server":
				data.ServerUUID = "other-server"
			case "wrong app origin":
				data.ApplicationOrigin = "https://other.example.com"
			case "private URL":
				data.PublicURL = "http://127.0.0.1:8000"
			case "service-role key":
				data.PublishableKey = strings.Replace(cloudDataTestAnon("first"), base64.RawURLEncoding.EncodeToString([]byte(`{"role":"anon","fixture":"first"}`)), base64.RawURLEncoding.EncodeToString([]byte(`{"role":"service_role","fixture":"first"}`)), 1)
			case "missing Dockerfile ARG":
				cloudFrameworkCommit(t, s, p, &req, "none")
				req.DataClientFramework = "vite"
			case "private native credentials", "configuration collision":
				key := "SUPABASE_SERVICE_ROLE_KEY"
				if name == "configuration collision" {
					key = "SUPABASE_URL"
				}
				journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/configuration", map[string]any{"expectedRevision": 0, "updates": []map[string]any{{"name": key, "value": "private-fixture-do-not-send"}}}, 200)
			}
			response := journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/releases/cloud", req, 409)
			if strings.Contains(response.Body.String(), data.PublishableKey) && data.PublishableKey != "" || strings.Contains(response.Body.String(), "private-fixture") {
				t.Fatal("preflight error exposed configuration values")
			}
			provider.mu.Lock()
			defer provider.mu.Unlock()
			if provider.projects != 0 || provider.applications != 0 || provider.deployments != 0 {
				t.Fatal("invalid Data configuration mutated the provider")
			}
		})
	}
}

func TestCloudDataBuildVariablesArePublicScopedOwnedAndRestoreChangesRedeploy(t *testing.T) {
	provider := &cloudProviderFixture{status: "finished"}
	s, mux, p, req, _ := cloudReleaseFixture(t, provider)
	s.registerApplicationConfiguration(mux)
	f := &runtimeEnvironmentFixture{variables: []coolifyRuntimeVariable{{UUID: "manual", Key: "MANUAL"}}}
	remote := httptest.NewServer(f.handler(provider.handler(t)))
	defer remote.Close()
	s.mu.Lock()
	target := s.targets[req.TargetID]
	target.BaseURL = remote.URL
	s.targets[req.TargetID] = target
	s.mu.Unlock()
	cloudFrameworkCommit(t, s, p, &req, "vite")
	data := cloudDataTestSnapshot(p, req)
	s.cloudDataBindings = func(context.Context, CloudDataBindingRequest) (CloudDataBindingSnapshot, error) { return data, nil }
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/configuration", map[string]any{"expectedRevision": 0, "updates": []map[string]any{{"name": "API_KEY", "value": "private-runtime-only"}}}, 200)
	firstResponse := journeyRequest(t, mux, "POST", base+"/releases/cloud", req, 202)
	first := decodeCloudRelease(t, firstResponse)
	if first.DataDomainRevision != 1 || first.DataBindingFingerprint == "" || first.DataBindingStatus != "current" || strings.Contains(firstResponse.Body.String(), data.PublishableKey) || strings.Contains(firstResponse.Body.String(), "private-runtime-only") {
		t.Fatal("release lost Data provenance or exposed variable values")
	}
	if encoded, _ := json.Marshal(data); string(encoded) != "{}" {
		t.Fatal("trusted Data snapshot can serialize its values")
	}
	checkVariables := func(framework, anon string) {
		t.Helper()
		f.mu.Lock()
		defer f.mu.Unlock()
		seen := map[string]bool{}
		for _, variable := range f.variables {
			if variable.UUID == "manual" {
				continue
			}
			if variable.IsPreview || variable.IsBuildtime == nil || variable.IsRuntime == nil || !*variable.IsRuntime {
				t.Fatal("owned variable has invalid runtime/preview scope")
			}
			seen[variable.Key] = true
			wantBuild := containsString(dataClientAliases(framework), variable.Key)
			if *variable.IsBuildtime != wantBuild {
				t.Fatal("private/runtime value entered build scope or client variable missing from build")
			}
			if strings.Contains(variable.Key, "KEY") && variable.Key != "API_KEY" && f.values[variable.UUID] != anon {
				t.Fatal("client key did not match active restored key")
			}
			if strings.Contains(variable.Key, "SERVICE_ROLE") || strings.Contains(variable.Key, "DB_URL") {
				t.Fatal("private native Data credential sent to another target")
			}
		}
		for _, name := range dataClientAliases(framework) {
			if !seen[name] {
				t.Fatal("selected framework alias missing")
			}
		}
		other := "vite"
		if framework == "vite" {
			other = "nextjs"
		}
		for _, name := range dataClientAliases(other) {
			if seen[name] {
				t.Fatal("obsolete client aliases were not removed")
			}
		}
	}
	checkVariables("vite", data.PublishableKey)
	first.Status = "ready"
	if err := s.storeCloudRelease(first); err != nil {
		t.Fatal(err)
	}
	data.DomainRevision++
	data.ConfigurationDigest = strings.Repeat("b", 64)
	data.PublishableKey = cloudDataTestAnon("restored")
	observed := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/releases/"+first.ID+"/refresh", map[string]any{}, 200))
	if observed.DataBindingStatus != "redeploy-required" || !strings.Contains(observed.Message, "deploy again") {
		t.Fatal("restored key change looked ready for the existing baked client")
	}
	second := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/releases/cloud", req, 202))
	if second.ID == first.ID || second.DataBindingFingerprint == first.DataBindingFingerprint || second.DataDomainRevision != 2 {
		t.Fatal("restore/key revision reused stale deployment")
	}
	checkVariables("vite", data.PublishableKey)
	second.Status = "ready"
	if err := s.storeCloudRelease(second); err != nil {
		t.Fatal(err)
	}
	cloudFrameworkCommit(t, s, p, &req, "nextjs")
	third := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/releases/cloud", req, 202))
	checkVariables("nextjs", data.PublishableKey)
	third.Status = "ready"
	if err := s.storeCloudRelease(third); err != nil {
		t.Fatal(err)
	}
	rollback := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/releases/"+first.ID+"/rollback", map[string]any{}, 202))
	if rollback.RollbackOf != first.ID || rollback.DataDomainRevision != 2 || rollback.DataBindingFingerprint == first.DataBindingFingerprint {
		t.Fatal("historical rebuild reused old Data keys/revision")
	}
	checkVariables("vite", data.PublishableKey)
	history, err := os.ReadFile(s.cloudReleaseFile(p.ID))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(history), data.PublishableKey) || strings.Contains(string(history), "private-runtime-only") {
		t.Fatal("Data/application values leaked into durable release history")
	}
}

func TestCloudDataChangeDuringConfigurationAndProviderBuildScopeRefuseDeploy(t *testing.T) {
	for _, scenario := range []string{"mid-config restore", "ignored client build flag", "manual client alias"} {
		t.Run(scenario, func(t *testing.T) {
			provider := &cloudProviderFixture{}
			s, mux, p, req, _ := cloudReleaseFixture(t, provider)
			cloudFrameworkCommit(t, s, p, &req, "vite")
			data := cloudDataTestSnapshot(p, req)
			calls := 0
			s.cloudDataBindings = func(context.Context, CloudDataBindingRequest) (CloudDataBindingSnapshot, error) {
				calls++
				current := data
				if scenario == "mid-config restore" && calls > 1 {
					current.DomainRevision++
					current.PublishableKey = cloudDataTestAnon("new-key")
				}
				return current, nil
			}
			f := &runtimeEnvironmentFixture{ignoreClientBuildFlag: scenario == "ignored client build flag"}
			if scenario == "manual client alias" {
				f.variables = []coolifyRuntimeVariable{{UUID: "manual", Key: "VITE_SUPABASE_URL"}}
			}
			remote := httptest.NewServer(f.handler(provider.handler(t)))
			defer remote.Close()
			s.mu.Lock()
			target := s.targets[req.TargetID]
			target.BaseURL = remote.URL
			s.targets[req.TargetID] = target
			s.mu.Unlock()
			wantStatus := 502
			if scenario == "mid-config restore" {
				wantStatus = 409
			}
			response := journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/releases/cloud", req, wantStatus)
			row := decodeCloudRelease(t, response)
			if row.Status != "failed" || strings.Contains(response.Body.String(), data.PublishableKey) {
				t.Fatal("ambiguous scope/key change leaked values or reported deployment")
			}
			if scenario == "mid-config restore" && row.DataBindingStatus != "redeploy-required" {
				t.Fatal("changed Data binding was reported as current")
			}
			provider.mu.Lock()
			deployments := provider.deployments
			provider.mu.Unlock()
			if deployments != 0 {
				t.Fatal("unconfirmed client configuration started a deployment")
			}
			if scenario == "manual client alias" {
				f.mu.Lock()
				mutations := f.mutations
				f.mu.Unlock()
				if mutations != 0 {
					t.Fatal("manual variable collision changed provider state")
				}
			}
		})
	}
}

func TestCloudDataDockerfileArgsMustReachFrameworkBuildStage(t *testing.T) {
	args := "ARG VITE_SUPABASE_URL\nARG VITE_SUPABASE_ANON_KEY\n"
	for _, fixture := range []struct {
		name, source string
		accepted     bool
	}{
		{"declared before build", "FROM node:24\n" + args + "RUN npm run build\n", true},
		{"later runtime declarations", "FROM node:24 AS build\nRUN npm run build\nFROM node:24\n" + args, false},
		{"global declarations are not stage inputs", args + "FROM node:24\nRUN npm run build\n", false},
		{"unrelated stage declarations", "FROM node:24 AS tools\n" + args + "RUN npm ci\nFROM node:24\nRUN npm run build\n", false},
		{"declarations after build", "FROM node:24\nRUN npm run build\n" + args, false},
		{"inherited build stage", "FROM node:24 AS base\n" + args + "RUN npm ci\nFROM base AS app\nRUN npx vite build\n", true},
		{"continued standard build", "FROM node:24\n" + args + "RUN npm ci && \\\n  npm run build\n", true},
		{"custom wrapper cannot imply environment", "FROM node:24\n" + args + "RUN ./build-custom.sh\n", false},
		{"one input omitted", "FROM node:24\nARG VITE_SUPABASE_URL\nRUN vite build\n", false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			if accepted := requireDataClientDockerfileArgs(fixture.source, "vite") == nil; accepted != fixture.accepted {
				t.Fatal("Dockerfile build-stage scope was incorrectly admitted")
			}
		})
	}
}
