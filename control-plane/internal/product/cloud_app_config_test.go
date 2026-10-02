package product

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

type runtimeEnvironmentFixture struct {
	mu                                     sync.Mutex
	variables                              []coolifyRuntimeVariable
	values                                 map[string]string
	mutations, deletes                     int
	lostWrite, lostDelete, ignoreBuildFlag bool
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
